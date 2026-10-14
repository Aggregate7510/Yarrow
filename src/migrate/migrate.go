// Package migrate imports an existing Yarr SQLite database into Yarrow.
//
// Yarrow uses Yarr's schema unchanged, so "migration" is: take a consistent
// backup of the source, copy it into place, let the normal schema migrations
// run on the copy (only needed when the source came from an older Yarr), and
// verify that nothing was lost. The source database is opened read-only and
// is never modified.
package migrate

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nkanaev/yarr/src/storage"
	"github.com/nkanaev/yarr/src/storage/sqlite"
)

type Options struct {
	Source    string // existing Yarr database
	Target    string // database Yarrow will use
	BackupDir string // where the untouched backup copy is written (default: next to Target)
	Overwrite bool   // replace an existing Target (it is moved aside first, never deleted)
	Out       io.Writer
	Now       func() time.Time
}

// Counts captures what is compared before and after migrating.
type Counts struct {
	SchemaVersion int64
	Folders       int64
	Feeds         int64
	Items         int64
	Unread        int64
	Read          int64
	Starred       int64
	Settings      int64
}

func (c Counts) String() string {
	return fmt.Sprintf("folders=%d feeds=%d items=%d (unread=%d read=%d starred=%d) settings=%d schema=v%d",
		c.Folders, c.Feeds, c.Items, c.Unread, c.Read, c.Starred, c.Settings, c.SchemaVersion)
}

type Result struct {
	Backup string
	Target string
	Before Counts
	After  Counts
	// MovedAside is set when an existing Target was renamed out of the way.
	MovedAside string
}

func openReadOnly(path string) (*sql.DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	// mode=ro: never write to the source; the driver refuses any write.
	db, err := sql.Open("sqlite3", "file:"+abs+"?mode=ro&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func readCounts(db *sql.DB) (Counts, error) {
	var c Counts
	if err := db.QueryRow("pragma user_version").Scan(&c.SchemaVersion); err != nil {
		return c, err
	}
	queries := []struct {
		dst *int64
		q   string
	}{
		{&c.Folders, "select count(*) from folders"},
		{&c.Feeds, "select count(*) from feeds"},
		{&c.Items, "select count(*) from items"},
		{&c.Unread, "select count(*) from items where status = 0"},
		{&c.Read, "select count(*) from items where status = 1"},
		{&c.Starred, "select count(*) from items where status = 2"},
		{&c.Settings, "select count(*) from settings"},
	}
	for _, x := range queries {
		if err := db.QueryRow(x.q).Scan(x.dst); err != nil {
			return c, fmt.Errorf("%s: %w", x.q, err)
		}
	}
	return c, nil
}

func checkIntegrity(db *sql.DB) error {
	var res string
	if err := db.QueryRow("pragma quick_check").Scan(&res); err != nil {
		return err
	}
	if res != "ok" {
		return fmt.Errorf("integrity check failed: %s", res)
	}
	return nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Run performs the migration. It is safe to repeat: without Overwrite it
// refuses to touch an existing target, and with Overwrite the old target is
// renamed (not deleted) before the new one is put in place.
func Run(opts Options) (*Result, error) {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	logf := func(format string, args ...any) { fmt.Fprintf(opts.Out, format+"\n", args...) }

	if opts.Source == "" || opts.Target == "" {
		return nil, errors.New("both source and target paths are required")
	}
	if strings.HasPrefix(opts.Source, "postgres") {
		return nil, errors.New("only SQLite sources are supported; use OPML export/import for PostgreSQL")
	}
	srcAbs, _ := filepath.Abs(opts.Source)
	dstAbs, _ := filepath.Abs(opts.Target)
	if srcAbs == dstAbs {
		return nil, errors.New("source and target must be different files (the source is never modified)")
	}
	if !exists(opts.Source) {
		return nil, fmt.Errorf("source database %s does not exist", opts.Source)
	}
	if exists(opts.Target) && !opts.Overwrite {
		return nil, fmt.Errorf("target %s already exists; pass -overwrite to replace it (the existing file will be renamed, not deleted)", opts.Target)
	}

	stamp := opts.Now().UTC().Format("20060102-150405")
	backupDir := opts.BackupDir
	if backupDir == "" {
		backupDir = filepath.Dir(dstAbs)
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return nil, err
	}
	backup := filepath.Join(backupDir, "yarr-backup-"+stamp+".db")
	tmp := dstAbs + ".migrating-" + stamp

	// 1. inspect the source, read-only
	src, err := openReadOnly(opts.Source)
	if err != nil {
		return nil, fmt.Errorf("open source read-only: %w (stop Yarr first so its write-ahead log is checkpointed)", err)
	}
	defer src.Close()
	if err := checkIntegrity(src); err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	before, err := readCounts(src)
	if err != nil {
		return nil, fmt.Errorf("source does not look like a Yarr database: %w", err)
	}
	if before.SchemaVersion > sqlite.LatestSchemaVersion() {
		return nil, fmt.Errorf("source schema v%d is newer than this build supports (v%d); upgrade Yarrow first",
			before.SchemaVersion, sqlite.LatestSchemaVersion())
	}
	logf("source:  %s", before)

	// 2. consistent backup (includes any un-checkpointed WAL content)
	if _, err := src.Exec("vacuum into ?", backup); err != nil {
		return nil, fmt.Errorf("backup failed: %w", err)
	}
	logf("backup:  %s", backup)

	// 3. working copy from the backup; schema migrations run on the copy
	if err := copyFile(backup, tmp); err != nil {
		return nil, err
	}
	cleanup := func() {
		for _, suffix := range []string{"", "-wal", "-shm"} {
			os.Remove(tmp + suffix)
		}
	}
	store, err := storage.New(tmp)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("upgrading copy: %w", err)
	}
	store.Close()

	// 4. verify
	check, err := sql.Open("sqlite3", "file:"+tmp+"?_busy_timeout=5000")
	if err != nil {
		cleanup()
		return nil, err
	}
	if err := checkIntegrity(check); err != nil {
		check.Close()
		cleanup()
		return nil, fmt.Errorf("migrated copy: %w", err)
	}
	after, err := readCounts(check)
	if _, cerr := check.Exec("pragma wal_checkpoint(truncate)"); cerr != nil && err == nil {
		err = cerr
	}
	check.Close()
	if err != nil {
		cleanup()
		return nil, err
	}
	logf("result:  %s", after)
	if before.Folders != after.Folders || before.Feeds != after.Feeds || before.Items != after.Items ||
		before.Unread != after.Unread || before.Read != after.Read || before.Starred != after.Starred ||
		after.Settings < before.Settings {
		cleanup()
		return nil, fmt.Errorf("verification failed: counts differ (before: %s, after: %s)", before, after)
	}

	// 5. move into place
	res := &Result{Backup: backup, Target: dstAbs, Before: before, After: after}
	if exists(dstAbs) {
		aside := dstAbs + ".replaced-" + stamp
		if err := os.Rename(dstAbs, aside); err != nil {
			cleanup()
			return nil, err
		}
		for _, suffix := range []string{"-wal", "-shm"} {
			if exists(dstAbs + suffix) {
				os.Rename(dstAbs+suffix, aside+suffix)
			}
		}
		res.MovedAside = aside
		logf("moved existing target aside: %s", aside)
	}
	if err := os.Rename(tmp, dstAbs); err != nil {
		cleanup()
		return nil, err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		os.Remove(tmp + suffix)
	}
	logf("migrated: %s", dstAbs)
	return res, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// Backup writes a consistent, compacted copy of a live database to dst using
// SQLite's VACUUM INTO. It is safe while Yarrow is running: the source is
// opened read-only and the snapshot is transactionally consistent.
func Backup(src, dst string) (Counts, error) {
	if exists(dst) {
		return Counts{}, fmt.Errorf("%s already exists", dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return Counts{}, err
	}
	db, err := openReadOnly(src)
	if err != nil {
		return Counts{}, err
	}
	defer db.Close()
	if _, err := db.Exec("vacuum into ?", dst); err != nil {
		return Counts{}, err
	}
	check, err := openReadOnly(dst)
	if err != nil {
		return Counts{}, err
	}
	defer check.Close()
	if err := checkIntegrity(check); err != nil {
		return Counts{}, err
	}
	return readCounts(check)
}
