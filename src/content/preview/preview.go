// Package preview derives the small amount of data an article card needs —
// a plain-text snippet and a lead image — from an item's stored HTML.
//
// Nothing is generated or summarised: the snippet is the beginning of the
// article's own text, cleaned of markup, scripts and boilerplate.
package preview

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nkanaev/yarr/src/content/htmlutil"
	"github.com/nkanaev/yarr/src/content/sanitizer"
	"github.com/nkanaev/yarr/src/storage/model"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// DefaultSnippetLength is roughly 3–5 lines of card text on a phone.
const DefaultSnippetLength = 320

// Elements whose text never belongs in a preview.
var skipText = map[atom.Atom]bool{
	atom.Script: true, atom.Style: true, atom.Noscript: true, atom.Template: true,
	atom.Iframe: true, atom.Svg: true, atom.Math: true, atom.Object: true,
	atom.Head: true, atom.Title: true, atom.Button: true, atom.Select: true,
	atom.Figcaption: true, atom.Form: true, atom.Nav: true, atom.Aside: true,
}

// Elements that introduce a visual break between text runs.
var blockLevel = map[atom.Atom]bool{
	atom.P: true, atom.Div: true, atom.Br: true, atom.Li: true, atom.Ul: true,
	atom.Ol: true, atom.H1: true, atom.H2: true, atom.H3: true, atom.H4: true,
	atom.H5: true, atom.H6: true, atom.Blockquote: true, atom.Pre: true,
	atom.Tr: true, atom.Td: true, atom.Th: true, atom.Section: true,
	atom.Article: true, atom.Header: true, atom.Footer: true, atom.Figure: true,
	atom.Hr: true, atom.Dd: true, atom.Dt: true,
}

var sentenceLike = map[atom.Atom]bool{
	atom.H1: true, atom.H2: true, atom.H3: true, atom.H4: true, atom.H5: true,
	atom.H6: true, atom.Li: true, atom.Dt: true, atom.Dd: true,
}

var spaceRun = regexp.MustCompile(`\s+`)

func parseFragment(content string) []*html.Node {
	ctx := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(content), ctx)
	if err != nil {
		return nil
	}
	return nodes
}

// Text returns the readable text of an HTML fragment with block elements
// separated by spaces and all whitespace collapsed.
func Text(content string) string {
	if !strings.ContainsAny(content, "<&") {
		return strings.TrimSpace(spaceRun.ReplaceAllString(content, " "))
	}
	var b strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			b.WriteString(n.Data)
			return
		case html.ElementNode:
			if skipText[n.DataAtom] {
				return
			}
			if blockLevel[n.DataAtom] {
				b.WriteByte(' ')
			}
		case html.CommentNode, html.DoctypeNode:
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode && blockLevel[n.DataAtom] {
			// headings and list items read as run-on text once flattened;
			// end them like sentences when they don't already
			if sentenceLike[n.DataAtom] {
				trimmed := strings.TrimRightFunc(b.String(), unicode.IsSpace)
				if r, _ := utf8.DecodeLastRuneInString(trimmed); unicode.IsLetter(r) || unicode.IsDigit(r) {
					b.Reset()
					b.WriteString(trimmed)
					b.WriteByte('.')
				}
			}
			b.WriteByte(' ')
		}
	}
	for _, n := range parseFragment(content) {
		walk(n)
	}
	return strings.TrimSpace(spaceRun.ReplaceAllString(b.String(), " "))
}

// Truncate shortens text to at most max runes, cutting at a word boundary
// and appending an ellipsis when anything was removed.
func Truncate(text string, max int) string {
	if max <= 0 || utf8.RuneCountInString(text) <= max {
		return text
	}
	runes := []rune(text)
	cut := max
	for i := max; i > max*2/3; i-- {
		if unicode.IsSpace(runes[i]) {
			cut = i
			break
		}
	}
	out := strings.TrimRightFunc(string(runes[:cut]), func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) && r != ')' && r != '"' && r != '”'
	})
	return out + "…"
}

// Snippet returns a clean preview of the article body. If the body begins
// by repeating the headline, the repetition is dropped.
func Snippet(title, content string, max int) string {
	text := Text(content)
	if title != "" {
		t := strings.TrimSpace(title)
		if len(t) > 0 && len(text) >= len(t) && strings.EqualFold(text[:len(t)], t) {
			text = strings.TrimLeftFunc(text[len(t):], func(r rune) bool {
				return unicode.IsSpace(r) || unicode.IsPunct(r) || r == '|'
			})
		}
	}
	return Truncate(text, max)
}

func isHTTPURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func dim(n *html.Node, key string) int {
	v := strings.TrimSuffix(strings.TrimSpace(htmlutil.Attr(n, key)), "px")
	if v == "" {
		return -1
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return -1
	}
	return i
}

// likely decorative: emoji sprites, avatars, share buttons, feed badges.
var decorative = regexp.MustCompile(`(?i)(/emoji/|s\.w\.org/images/core/emoji|gravatar\.com/avatar|/avatars?/|share[-_]?button|/badge|/icons?/|\.svg(\?|$)|/spacer\.gif|/blank\.gif)`)

func usableImage(src string) bool {
	return isHTTPURL(src) && !sanitizer.IsBlockedResource(src) && !decorative.MatchString(src)
}

// Image picks the card image for an item: an explicit media image from the
// feed (media:content, enclosure, thumbnail) first, then the first reasonably
// sized <img> in the content. Relative URLs are resolved against the item's
// link. Returns "" when there is no suitable image.
func Image(item model.Item) string {
	for _, m := range item.MediaLinks {
		if m.Type == "image" && usableImage(m.URL) {
			return m.URL
		}
	}
	if !strings.Contains(item.Content, "<img") && !strings.Contains(item.Content, "<IMG") {
		return ""
	}
	var found string
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if found != "" {
			return
		}
		if n.Type == html.ElementNode && n.DataAtom == atom.Img {
			w, h := dim(n, "width"), dim(n, "height")
			tooSmall := (w >= 0 && w < 80) || (h >= 0 && h < 60)
			src := strings.TrimSpace(htmlutil.Attr(n, "src"))
			if src == "" || strings.HasPrefix(src, "data:") {
				src = firstSrcset(htmlutil.Attr(n, "srcset"))
			}
			if src != "" && !tooSmall {
				abs := src
				if item.Link != "" {
					abs = htmlutil.AbsoluteUrl(src, item.Link)
				}
				if usableImage(abs) {
					found = abs
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range parseFragment(item.Content) {
		walk(n)
	}
	return found
}

func firstSrcset(srcset string) string {
	srcset = strings.TrimSpace(srcset)
	if srcset == "" {
		return ""
	}
	first := strings.SplitN(srcset, ",", 2)[0]
	fields := strings.Fields(first)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
