package server

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nkanaev/yarr/src/storage"
	"github.com/nkanaev/yarr/src/storage/model"
)

type testEnv struct {
	db      storage.Storage
	handler http.Handler
	folder  *model.Folder
	feedA   *model.Feed
	feedB   *model.Feed
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer("127.0.0.1:0")
	srv.Storage = NewLocalStorage(db)
	env := &testEnv{db: db, handler: srv.Handler()}
	env.folder = db.CreateFolder("Technology")
	env.feedA = db.CreateFeed(model.CreateFeedParams{Title: "A", FeedLink: "https://a.example/feed", Link: "https://a.example/", FolderID: &env.folder.Id})
	env.feedB = db.CreateFeed(model.CreateFeedParams{Title: "B", FeedLink: "https://b.example/feed", Link: "https://b.example/"})
	return env
}

func (e *testEnv) do(t *testing.T, method, url, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, url, r)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func TestCardsView(t *testing.T) {
	env := newTestEnv(t)
	now := time.Now()
	env.db.CreateItems([]model.Item{
		{GUID: "1", FeedId: env.feedA.Id, Title: "With image", Link: "/posts/1", Date: now,
			Content: `<p>First paragraph <script>evil()</script>of text.</p><img src="/hero.jpg">`},
		{GUID: "2", FeedId: env.feedB.Id, Title: "", Link: "https://b.example/2", Date: now.Add(-time.Hour),
			Content: `<p>Untitled body text becomes the title.</p>`},
		{GUID: "3", FeedId: env.feedB.Id, Title: "Podcast", Link: "https://b.example/3", Date: now.Add(-2 * time.Hour),
			MediaLinks: model.MediaLinks{{URL: "https://b.example/ep.mp3", Type: "audio", Description: "Episode notes"}}},
	})

	rec := env.do(t, "GET", "/api/items?view=cards", "", nil)
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var resp struct {
		List    []CardItem `json:"list"`
		HasMore bool       `json:"has_more"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.List) != 3 {
		t.Fatalf("expected 3 cards, got %d", len(resp.List))
	}
	if strings.Contains(rec.Body.String(), `"content"`) {
		t.Error("card view must not include full content")
	}
	c := resp.List[0]
	if c.Snippet != "First paragraph of text." {
		t.Errorf("snippet: %q", c.Snippet)
	}
	if c.Image != "https://a.example/hero.jpg" {
		t.Errorf("image: %q", c.Image)
	}
	if c.Link != "https://a.example/posts/1" {
		t.Errorf("relative link not resolved: %q", c.Link)
	}
	if resp.List[1].Title != "Untitled body text becomes the title." || resp.List[1].Snippet != "" {
		t.Errorf("untitled item: %+v", resp.List[1])
	}
	if !resp.List[2].HasAudio || resp.List[2].Snippet != "Episode notes" {
		t.Errorf("podcast item: %+v", resp.List[2])
	}
}

func TestCardsPaginationNoDuplicates(t *testing.T) {
	env := newTestEnv(t)
	now := time.Now()
	var items []model.Item
	for i := range 45 {
		feed := env.feedA.Id
		if i%2 == 0 {
			feed = env.feedB.Id
		}
		// several items share a timestamp to exercise the (date, id) tiebreak
		items = append(items, model.Item{GUID: string(rune('a' + i)), FeedId: feed, Title: "t", Date: now.Add(-time.Duration(i/3) * time.Minute)})
	}
	env.db.CreateItems(items)

	seen := map[int64]bool{}
	after := ""
	pages := 0
	for {
		rec := env.do(t, "GET", "/api/items?view=cards&limit=10"+after, "", nil)
		var resp struct {
			List    []CardItem `json:"list"`
			HasMore bool       `json:"has_more"`
		}
		json.Unmarshal(rec.Body.Bytes(), &resp)
		for _, c := range resp.List {
			if seen[c.Id] {
				t.Fatalf("duplicate item %d on page %d", c.Id, pages)
			}
			seen[c.Id] = true
		}
		pages++
		if !resp.HasMore {
			break
		}
		after = "&after=" + itoa(resp.List[len(resp.List)-1].Id)
	}
	if len(seen) != 45 || pages != 5 {
		t.Fatalf("saw %d items in %d pages", len(seen), pages)
	}
}

func itoa(i int64) string {
	b, _ := json.Marshal(i)
	return string(b)
}

func TestLimitIsClamped(t *testing.T) {
	if parsePageSize("100000") != maxPageSize || parsePageSize("-3") != defaultPageSize || parsePageSize("x") != defaultPageSize {
		t.Fatal("page size not clamped")
	}
}

func TestInvalidStatusRejected(t *testing.T) {
	env := newTestEnv(t)
	if rec := env.do(t, "GET", "/api/items?status=bogus", "", nil); rec.Code != 400 {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestMarkAboveRead(t *testing.T) {
	env := newTestEnv(t)
	now := time.Now()
	env.db.CreateItems([]model.Item{
		{GUID: "new", FeedId: env.feedA.Id, Title: "new", Date: now},
		{GUID: "mid", FeedId: env.feedA.Id, Title: "mid", Date: now.Add(-time.Hour)},
		{GUID: "old", FeedId: env.feedA.Id, Title: "old", Date: now.Add(-2 * time.Hour)},
	})
	var mid int64
	for _, it := range env.db.ListItems(model.ItemFilter{}, 10, true, false) {
		if it.GUID == "mid" {
			mid = it.Id
		}
	}
	rec := env.do(t, "PUT", "/api/items?folder_id="+itoa(env.folder.Id)+"&above_id="+itoa(mid), "", nil)
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	status := map[string]model.ItemStatus{}
	for _, it := range env.db.ListItems(model.ItemFilter{}, 10, true, false) {
		status[it.GUID] = it.Status
	}
	if status["new"] != model.READ || status["mid"] != model.UNREAD || status["old"] != model.UNREAD {
		t.Fatalf("unexpected statuses: %v", status)
	}
}

func TestSettingsValidation(t *testing.T) {
	env := newTestEnv(t)
	if rec := env.do(t, "PUT", "/api/settings", `{"nav_mode":"sideways"}`, nil); rec.Code != 400 {
		t.Fatalf("expected 400 for invalid nav_mode, got %d", rec.Code)
	}
	if rec := env.do(t, "PUT", "/api/settings", `{"nav_mode":"expanded","theme_name":"night"}`, nil); rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if s := env.db.GetSettings(); s.NavMode != "expanded" || s.ThemeName != "night" {
		t.Fatalf("settings not saved: %+v", s)
	}
}

func TestCrossOriginRequestsBlocked(t *testing.T) {
	env := newTestEnv(t)
	// a malicious page POSTing a "simple" text/plain request
	evil := map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example", "Content-Type": "text/plain"}
	if rec := env.do(t, "POST", "/api/folders", `{"title":"pwned"}`, evil); rec.Code != http.StatusForbidden {
		t.Fatalf("cross-site POST should be forbidden, got %d", rec.Code)
	}
	if rec := env.do(t, "POST", "/api/feeds/refresh", "", evil); rec.Code != http.StatusForbidden {
		t.Fatalf("cross-site refresh should be forbidden, got %d", rec.Code)
	}
	same := map[string]string{"Sec-Fetch-Site": "same-origin", "Content-Type": "application/json"}
	if rec := env.do(t, "POST", "/api/folders", `{"title":"ok"}`, same); rec.Code != http.StatusCreated {
		t.Fatalf("same-origin POST should work, got %d", rec.Code)
	}
	// non-browser clients (no fetch metadata) are allowed
	if rec := env.do(t, "POST", "/api/folders", `{"title":"cli"}`, nil); rec.Code != http.StatusCreated {
		t.Fatalf("header-less POST should work, got %d", rec.Code)
	}
	// cross-site GETs (e.g. a link) still work
	if rec := env.do(t, "GET", "/api/folders", "", evil); rec.Code != 200 {
		t.Fatalf("cross-site GET should work, got %d", rec.Code)
	}
}

func TestSecurityHeadersAndHealth(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, "GET", "/healthz", "", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"ok"`) {
		t.Fatalf("health: %d %s", rec.Code, rec.Body.String())
	}
	for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "Referrer-Policy", "X-Frame-Options"} {
		if rec.Header().Get(h) == "" {
			t.Errorf("missing header %s", h)
		}
	}
	if csp := rec.Header().Get("Content-Security-Policy"); strings.Contains(csp, "unsafe") {
		t.Errorf("CSP must not allow unsafe-*: %s", csp)
	}
}

func TestIndexHasNoInlineScript(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, "GET", "/", "", nil)
	body := rec.Body.String()
	if !strings.Contains(body, `id="app-config" type="application/json"`) {
		t.Fatal("config block missing")
	}
	// the only <script> without src must be the JSON data block
	if strings.Count(body, "<script") != 2 || strings.Count(body, `<script src=`) != 1 {
		t.Fatalf("unexpected script tags:\n%s", body)
	}
}

func TestManifestAndServiceWorker(t *testing.T) {
	srv := NewServer("127.0.0.1:0")
	srv.BasePath = "/rss"
	h := srv.Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/rss/manifest.json", nil))
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["start_url"] != "/rss/" || m["scope"] != "/rss/" || m["display"] != "standalone" {
		t.Fatalf("manifest: %v", m)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/rss/sw.js", nil))
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("service worker: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
}

func TestStaticETag(t *testing.T) {
	h := NewServer("127.0.0.1:0").Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/static/bundle.js", nil))
	etag := rec.Header().Get("ETag")
	if rec.Code != 200 || etag == "" {
		t.Fatalf("status %d etag %q", rec.Code, etag)
	}
	req := httptest.NewRequest("GET", "/static/bundle.js", nil)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("expected 304, got %d", rec.Code)
	}
}

func TestAuthCookieFlags(t *testing.T) {
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)
	db, _ := storage.New(":memory:")
	srv := NewServer("127.0.0.1:0")
	srv.Storage = NewLocalStorage(db)
	srv.Auth = NewLocalAuthProvider("me", "secret", "")
	h := srv.Handler()

	login := func(proto string) *http.Cookie {
		req := httptest.NewRequest("POST", "/login", strings.NewReader("username=me&password=secret"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if proto != "" {
			req.Header.Set("X-Forwarded-Proto", proto)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("login failed: %d", rec.Code)
		}
		return rec.Result().Cookies()[0]
	}
	plain := login("")
	if !plain.HttpOnly || plain.Secure {
		t.Errorf("plain http cookie: httponly=%v secure=%v", plain.HttpOnly, plain.Secure)
	}
	proxied := login("https")
	if !proxied.HttpOnly || !proxied.Secure {
		t.Errorf("https cookie: httponly=%v secure=%v", proxied.HttpOnly, proxied.Secure)
	}

	// API is protected, health is not
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/items", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("api without cookie: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != 200 {
		t.Errorf("healthz should be public: %d", rec.Code)
	}
	req := httptest.NewRequest("GET", "/api/items", nil)
	req.AddCookie(plain)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("api with cookie: %d", rec.Code)
	}
}
