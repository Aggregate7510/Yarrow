package migrate

import (
	"bytes"
	"crypto/sha256"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nkanaev/yarr/src/storage"
	"github.com/nkanaev/yarr/src/storage/model"
)

func makeYarrDB(t *testing.T, path string) {
	t.Helper()
	db, err := storage.New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tech := db.CreateFolder("Technology")
	db.CreateFolder("Empty")
	f1 := db.CreateFeed(model.CreateFeedParams{Title: "A", FeedLink: "https://a.example/feed", FolderID: &tech.Id})
	f2 := db.CreateFeed(model.CreateFeedParams{Title: "B", FeedLink: "https://b.example/feed"})
	now := time.Now()
	db.CreateItems([]model.Item{
		{GUID: "1", FeedId: f1.Id, Title: "one", Date: now, Status: model.UNREAD},
		{GUID: "2", FeedId: f1.Id, Title: "two", Date: now, Status: model.READ},
		{GUID: "3", FeedId: f2.Id, Title: "three", Date: now, Status: model.STARRED},
		{GUID: "4", FeedId: f2.Id, Title: "four", Date: now, Status: model.UNREAD},
	})
	db.UpdateSettings(model.UpdateSettingsParams{ThemeName: new("night"), RefreshRate: new(int64(30))})
}

func fileHash(t *testing.T, path string) [32]byte {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(b)
}

func TestMigratePreservesEverything(t *testing.T) {
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	dir := t.TempDir()
	src := filepath.Join(dir, "yarr", "storage.db")
	os.MkdirAll(filepath.Dir(src), 0o755)
	makeYarrDB(t, src)
	srcHashBefore := fileHash(t, src)

	target := filepath.Join(dir, "data", "storage.db")
	var out bytes.Buffer
	res, err := Run(Options{Source: src, Target: target, Out: &out})
	if err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}

	if fileHash(t, src) != srcHashBefore {
		t.Fatal("source database was modified")
	}
	if _, err := os.Stat(res.Backup); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	want := Counts{Folders: 2, Feeds: 2, Items: 4, Unread: 2, Read: 1, Starred: 1}
	got := res.After
	if got.Folders != want.Folders || got.Feeds != want.Feeds || got.Items != want.Items ||
		got.Unread != want.Unread || got.Read != want.Read || got.Starred != want.Starred {
		t.Fatalf("unexpected counts: %s", got)
	}

	db, err := storage.New(target)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := db.GetSettings()
	if s.ThemeName != "night" || s.RefreshRate != 30 {
		t.Fatalf("settings not preserved: %+v", s)
	}
	folders := db.ListFolders()
	if len(folders) != 2 {
		t.Fatalf("folders: %+v", folders)
	}
	feeds := db.ListFeeds()
	var inFolder int
	for _, f := range feeds {
		if f.FolderId != nil {
			inFolder++
		}
	}
	if inFolder != 1 {
		t.Fatalf("folder assignment lost: %+v", feeds)
	}
}

func TestMigrateRefusesExistingTargetAndCanOverwrite(t *testing.T) {
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	dir := t.TempDir()
	src := filepath.Join(dir, "old.db")
	makeYarrDB(t, src)
	target := filepath.Join(dir, "new.db")

	if _, err := Run(Options{Source: src, Target: target}); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(Options{Source: src, Target: target}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected refusal, got %v", err)
	}
	res, err := Run(Options{Source: src, Target: target, Overwrite: true, Now: func() time.Time { return time.Now().Add(time.Hour) }})
	if err != nil {
		t.Fatal(err)
	}
	if res.MovedAside == "" {
		t.Fatal("existing target should have been moved aside")
	}
	if _, err := os.Stat(res.MovedAside); err != nil {
		t.Fatalf("moved-aside file missing: %v", err)
	}
}

func TestMigrateRejectsBadInput(t *testing.T) {
	dir := t.TempDir()
	if _, err := Run(Options{Source: filepath.Join(dir, "missing.db"), Target: filepath.Join(dir, "t.db")}); err == nil {
		t.Fatal("missing source should fail")
	}
	same := filepath.Join(dir, "same.db")
	os.WriteFile(same, nil, 0o644)
	if _, err := Run(Options{Source: same, Target: same}); err == nil {
		t.Fatal("same source and target should fail")
	}
	junk := filepath.Join(dir, "junk.db")
	os.WriteFile(junk, []byte("this is not sqlite"), 0o644)
	if _, err := Run(Options{Source: junk, Target: filepath.Join(dir, "t2.db")}); err == nil {
		t.Fatal("non-sqlite source should fail")
	}
	if _, err := os.Stat(filepath.Join(dir, "t2.db")); err == nil {
		t.Fatal("no target should be created on failure")
	}
}
