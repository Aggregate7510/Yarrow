package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nkanaev/yarr/src/migrate"
)

func runMigrate(args []string) int {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	from := fs.String("from", "", "`path` to the existing Yarr database (opened read-only)")
	to := fs.String("to", opt("YARR_DB", ""), "`path` of the database Yarrow will use")
	backupDir := fs.String("backup-dir", "", "`directory` for the backup copy (default: next to -to)")
	overwrite := fs.Bool("overwrite", false, "replace an existing target (it is renamed aside, never deleted)")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s migrate -from OLD.db -to NEW.db [-backup-dir DIR] [-overwrite]\n\n", os.Args[0])
		fmt.Fprintln(fs.Output(), "Copies an existing Yarr SQLite database into place for Yarrow, keeping")
		fmt.Fprintln(fs.Output(), "feeds, folders, read/unread state, starred (saved) articles, article")
		fmt.Fprintln(fs.Output(), "history and settings. The source file is never modified.")
		fmt.Fprintln(fs.Output())
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *from == "" || *to == "" {
		fs.Usage()
		return 2
	}
	res, err := migrate.Run(migrate.Options{
		Source: *from, Target: *to, BackupDir: *backupDir, Overwrite: *overwrite, Out: os.Stdout,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "migration failed:", err)
		fmt.Fprintln(os.Stderr, "nothing was changed in the source database.")
		return 1
	}
	fmt.Println()
	fmt.Println("Migration complete and verified.")
	fmt.Println("  backup of original:", res.Backup)
	fmt.Println("  new database:      ", res.Target)
	return 0
}

func runBackup(args []string) int {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	db := fs.String("db", opt("YARR_DB", ""), "`path` of the live database")
	to := fs.String("to", "", "`path` of the backup file to create")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *db == "" || *to == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s backup [-db LIVE.db] -to BACKUP.db\n", os.Args[0])
		return 2
	}
	counts, err := migrate.Backup(*db, *to)
	if err != nil {
		fmt.Fprintln(os.Stderr, "backup failed:", err)
		return 1
	}
	fmt.Printf("backup written to %s (%s)\n", *to, counts)
	return 0
}

// runHealthcheck probes the local server. It exists so container images do
// not need curl or wget for their HEALTHCHECK.
func runHealthcheck(args []string) int {
	fs := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
	addr := fs.String("addr", opt("YARR_ADDR", "127.0.0.1:7070"), "server address")
	base := fs.String("base", opt("YARR_BASE", ""), "base path")
	tlsOn := fs.Bool("tls", opt("YARR_CERTFILE", "") != "", "use https")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	path := "/" + strings.Trim(*base, "/")
	if path != "/" {
		path += "/"
	}
	path += "healthz"

	transport := &http.Transport{}
	host := *addr
	if sock, ok := strings.CutPrefix(host, "unix:"); ok {
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", sock)
		}
		host = "localhost"
	} else if h, port, err := net.SplitHostPort(host); err == nil {
		if h == "" || h == "0.0.0.0" || h == "::" || h == "[::]" {
			h = "127.0.0.1"
		}
		host = net.JoinHostPort(h, port)
	}
	scheme := "http"
	if *tlsOn {
		scheme = "https"
		// loopback self-probe of our own certificate
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	client := &http.Client{Timeout: 4 * time.Second, Transport: transport}
	res, err := client.Get(scheme + "://" + host + path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "unhealthy:", err)
		return 1
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "unhealthy: status", res.StatusCode)
		return 1
	}
	fmt.Println("healthy")
	return 0
}
