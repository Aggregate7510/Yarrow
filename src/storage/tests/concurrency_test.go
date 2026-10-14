package tests

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nkanaev/yarr/src/storage"
	"github.com/nkanaev/yarr/src/storage/model"
)

// Feed refreshes write items while favicon lookups update feeds from other
// goroutines. On a file database none of those writes may fail with
// "database is locked".
func TestConcurrentWritesOnFileDatabase(t *testing.T) {
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)
	db, err := storage.New(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var feeds []*model.Feed
	for i := range 8 {
		feeds = append(feeds, db.CreateFeed(model.CreateFeedParams{Title: "f", FeedLink: fmt.Sprintf("https://e/%d", i)}))
	}
	var wg sync.WaitGroup
	errs := make(chan string, 1000)
	for i, f := range feeds {
		wg.Add(2)
		go func(i int, f *model.Feed) {
			defer wg.Done()
			for round := range 20 {
				items := make([]model.Item, 25)
				for j := range items {
					items[j] = model.Item{GUID: fmt.Sprintf("%d-%d-%d", i, round, j), FeedId: f.Id, Title: "t", Date: time.Now()}
				}
				if !db.CreateItems(items) {
					errs <- "CreateItems failed"
				}
			}
		}(i, f)
		go func(f *model.Feed) {
			defer wg.Done()
			for range 20 {
				icon := model.Icon([]byte{1, 2, 3})
				if _, err := db.UpdateFeed(f.Id, model.UpdateFeedParams{Icon: model.SetNullable(&icon)}); err != nil {
					errs <- err.Error()
				}
			}
		}(f)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	if n := db.CountItems(); n != 8*20*25 {
		t.Fatalf("expected %d items, got %d", 8*20*25, n)
	}
}
