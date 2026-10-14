package server

// Yarrow additions to the Yarr HTTP server: card-oriented item listing,
// health check, PWA endpoints and static asset validators. Kept in a
// separate file to make merging upstream Yarr changes easier.

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/nkanaev/yarr/src/assets"
	"github.com/nkanaev/yarr/src/content/htmlutil"
	"github.com/nkanaev/yarr/src/content/preview"
	"github.com/nkanaev/yarr/src/storage/model"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// CardItem is the compact representation used by the timeline: everything a
// card shows, without the (potentially large) article body.
type CardItem struct {
	Id       int64            `json:"id"`
	FeedId   int64            `json:"feed_id"`
	Title    string           `json:"title"`
	Link     string           `json:"link"`
	Date     time.Time        `json:"date"`
	Status   model.ItemStatus `json:"status"`
	Snippet  string           `json:"snippet"`
	Image    string           `json:"image,omitempty"`
	HasAudio bool             `json:"has_audio,omitempty"`
	HasVideo bool             `json:"has_video,omitempty"`
}

func toCard(item model.Item, feedLink string) CardItem {
	link := item.Link
	if link != "" && !htmlutil.IsAPossibleLink(link) && feedLink != "" {
		link = htmlutil.AbsoluteUrl(link, feedLink)
	}
	item.Link = link
	card := CardItem{
		Id:     item.Id,
		FeedId: item.FeedId,
		Title:  item.Title,
		Link:   link,
		Date:   item.Date,
		Status: item.Status,
		Image:  preview.Image(item),
	}
	card.Snippet = preview.Snippet(item.Title, item.Content, preview.DefaultSnippetLength)
	if card.Snippet == "" {
		// media-only items (podcasts, photo feeds) often carry a description
		// on the enclosure instead of in the body
		for _, m := range item.MediaLinks {
			if m.Description != "" {
				card.Snippet = preview.Snippet(item.Title, m.Description, preview.DefaultSnippetLength)
				break
			}
		}
	}
	if card.Title == "" {
		card.Title = preview.Truncate(card.Snippet, 140)
		card.Snippet = ""
	}
	for _, m := range item.MediaLinks {
		switch m.Type {
		case "audio":
			card.HasAudio = true
		case "video":
			card.HasVideo = true
		}
	}
	return card
}

func parsePageSize(raw string) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultPageSize
	}
	if n > maxPageSize {
		return maxPageSize
	}
	return n
}

// handleHealth is an unauthenticated liveness/readiness probe used by Docker.
// It touches the database so a broken volume reports unhealthy.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if s.Storage == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "no storage"})
		return
	}
	db := s.db(r)
	if db == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "no storage"})
		return
	}
	if err := db.Ping(); err != nil {
		log.Printf("health check failed: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "database error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
	base := s.BasePath + "/"
	writeJSON(w, http.StatusOK, map[string]any{
		"name":             "Yarrow",
		"short_name":       "Yarrow",
		"description":      "A calm, self-hosted RSS reader",
		"display":          "standalone",
		"start_url":        base,
		"scope":            base,
		"id":               base,
		"background_color": "#f6f3ee",
		"theme_color":      "#f6f3ee",
		"icons": []map[string]any{
			{"src": base + "static/icons/icon-192.png", "sizes": "192x192", "type": "image/png"},
			{"src": base + "static/icons/icon-512.png", "sizes": "512x512", "type": "image/png"},
			{"src": base + "static/icons/icon-maskable-512.png", "sizes": "512x512", "type": "image/png", "purpose": "maskable"},
		},
	})
}

// handleServiceWorker serves the service worker from the application root so
// its scope covers the whole app (a worker under /static/ could only control
// /static/).
func (s *Server) handleServiceWorker(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(assets.StaticFS(), "sw.js")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	// the browser must always see the newest worker so updates roll out
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(data)
}

// staticHandler serves embedded assets with a content-hash ETag so clients
// revalidate cheaply instead of re-downloading the bundle.
func staticHandler() http.Handler {
	static := assets.StaticFS()
	files := http.FileServer(http.FS(static))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if f, err := static.Open(name); err == nil {
			if st, err := f.Stat(); err == nil && !st.IsDir() {
				h := sha256.New()
				if _, err := io.Copy(h, f); err == nil {
					w.Header().Set("ETag", `"`+hex.EncodeToString(h.Sum(nil))[:20]+`"`)
				}
			}
			f.Close()
		}
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})
}
