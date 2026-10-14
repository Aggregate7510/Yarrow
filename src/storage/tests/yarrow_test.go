package tests

import (
	"reflect"
	"slices"
	"testing"

	"github.com/nkanaev/yarr/src/storage"
	"github.com/nkanaev/yarr/src/storage/model"
)

func itemIDByGUID(t *testing.T, db storage.Storage, guid string) int64 {
	t.Helper()
	for _, it := range db.ListItems(model.ItemFilter{}, 100, true, false) {
		if it.GUID == guid {
			return it.Id
		}
	}
	t.Fatalf("item %s not found", guid)
	return 0
}

func unreadGuids(db storage.Storage) []string {
	unread := model.UNREAD
	g := getItemGuids(db.ListItems(model.ItemFilter{Status: &unread}, 100, false, false))
	slices.Sort(g)
	return g
}

func TestMarkItemsReadAboveNewestFirst(t *testing.T) {
	dbtest(t, func(t *testing.T, db storage.Storage) {
		testItemsSetup(db)
		id := itemIDByGUID(t, db, "item121")
		db.MarkItemsRead(model.MarkFilter{AboveID: &id, NewestFirst: true})
		// item011 (newer) becomes read; item121 itself and older item111 stay unread
		if have, want := unreadGuids(db), []string{"item111", "item121"}; !reflect.DeepEqual(have, want) {
			t.Fatalf("want %v have %v", want, have)
		}
		// starred items must never be downgraded to read
		starred := model.STARRED
		if n := len(db.ListItems(model.ItemFilter{Status: &starred}, 100, false, false)); n != 3 {
			t.Fatalf("starred items changed: %d", n)
		}
	})
}

func TestMarkItemsReadAboveOldestFirst(t *testing.T) {
	dbtest(t, func(t *testing.T, db storage.Storage) {
		testItemsSetup(db)
		id := itemIDByGUID(t, db, "item121")
		db.MarkItemsRead(model.MarkFilter{AboveID: &id, NewestFirst: false})
		if have, want := unreadGuids(db), []string{"item011", "item121"}; !reflect.DeepEqual(have, want) {
			t.Fatalf("want %v have %v", want, have)
		}
	})
}

func TestMarkItemsReadAboveInFolder(t *testing.T) {
	dbtest(t, func(t *testing.T, db storage.Storage) {
		scope := testItemsSetup(db)
		id := itemIDByGUID(t, db, "item111")
		db.MarkItemsRead(model.MarkFilter{FolderID: &scope.folder1.Id, AboveID: &id, NewestFirst: true})
		// item121 (folder1, newer) read; item011 is in no folder and must stay unread
		if have, want := unreadGuids(db), []string{"item011", "item111"}; !reflect.DeepEqual(have, want) {
			t.Fatalf("want %v have %v", want, have)
		}
	})
}

func TestFolderTimelinePagination(t *testing.T) {
	dbtest(t, func(t *testing.T, db storage.Storage) {
		scope := testItemsSetup(db)
		// page through folder1 two at a time, newest first, and make sure the
		// concatenation equals one big query with no duplicates
		all := getItemGuids(db.ListItems(model.ItemFilter{FolderID: &scope.folder1.Id}, 100, true, false))
		var paged []string
		var after *int64
		for range 10 {
			page := db.ListItems(model.ItemFilter{FolderID: &scope.folder1.Id, After: after}, 2, true, false)
			if len(page) == 0 {
				break
			}
			paged = append(paged, getItemGuids(page)...)
			last := page[len(page)-1].Id
			after = &last
		}
		if !reflect.DeepEqual(all, paged) {
			t.Fatalf("pagination mismatch: all=%v paged=%v", all, paged)
		}
		want := []string{"item122", "item121", "item113", "item112", "item111"}
		if !reflect.DeepEqual(all, want) {
			t.Fatalf("want %v have %v", want, all)
		}
	})
}

func TestNavModeSetting(t *testing.T) {
	dbtest(t, func(t *testing.T, db storage.Storage) {
		if got := db.GetSettings().NavMode; got != "folders" {
			t.Fatalf("default nav_mode should be folders, got %q", got)
		}
		if !db.UpdateSettings(model.UpdateSettingsParams{NavMode: new("expanded")}) {
			t.Fatal("update failed")
		}
		if got := db.GetSettings().NavMode; got != "expanded" {
			t.Fatalf("got %q", got)
		}
	})
}

func TestSettingsValidate(t *testing.T) {
	bad := []model.UpdateSettingsParams{
		{NavMode: new("sideways")},
		{ThemeName: new("hotdog")},
		{RefreshRate: new(int64(-1))},
		{Filter: new("everything")},
		{ThemeSize: new(100.0)},
	}
	for i, p := range bad {
		if p.Validate() == nil {
			t.Errorf("case %d should fail validation", i)
		}
	}
	good := model.UpdateSettingsParams{NavMode: new("all"), ThemeName: new("night"), RefreshRate: new(int64(30))}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
}
