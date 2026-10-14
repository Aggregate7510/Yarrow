package preview

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/nkanaev/yarr/src/storage/model"
)

func TestTextStripsMarkupAndScripts(t *testing.T) {
	in := `<p>Hello <b>world</b>.</p><script>alert('x')</script><style>p{}</style><p>Second&nbsp;para &amp; more</p><figure><img src="a.jpg"><figcaption>Caption</figcaption></figure>`
	got := Text(in)
	want := "Hello world. Second para & more"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestTextBlockBoundaries(t *testing.T) {
	got := Text(`<h2>Title</h2><p>One</p><ul><li>a</li><li>b</li></ul>line<br>break`)
	if got != "Title One a b line break" {
		t.Fatalf("got %q", got)
	}
}

func TestTextPlain(t *testing.T) {
	if got := Text("  plain\n\n text "); got != "plain text" {
		t.Fatalf("got %q", got)
	}
}

func TestTruncate(t *testing.T) {
	s := strings.Repeat("word ", 100)
	got := Truncate(s, 50)
	if utf8.RuneCountInString(got) > 51 {
		t.Fatalf("too long: %d", utf8.RuneCountInString(got))
	}
	if !strings.HasSuffix(got, "word…") {
		t.Fatalf("should cut at word boundary: %q", got)
	}
	if Truncate("short", 50) != "short" {
		t.Fatal("short text must be unchanged")
	}
	// multibyte safety
	jp := strings.Repeat("日本語", 50)
	if !utf8.ValidString(Truncate(jp, 20)) {
		t.Fatal("invalid utf8 after truncation")
	}
}

func TestSnippetDropsRepeatedTitle(t *testing.T) {
	got := Snippet("Big News", "<h1>Big News</h1><p>Body text here.</p>", 100)
	if got != "Body text here." {
		t.Fatalf("got %q", got)
	}
	got = Snippet("Other", "<p>Body</p>", 100)
	if got != "Body" {
		t.Fatalf("got %q", got)
	}
}

func TestImagePrefersMediaLinks(t *testing.T) {
	item := model.Item{
		Link:    "https://example.com/post",
		Content: `<img src="https://example.com/inline.jpg">`,
		MediaLinks: model.MediaLinks{
			{URL: "https://cdn.example.com/podcast.mp3", Type: "audio"},
			{URL: "https://cdn.example.com/hero.jpg", Type: "image"},
		},
	}
	if got := Image(item); got != "https://cdn.example.com/hero.jpg" {
		t.Fatalf("got %q", got)
	}
}

func TestImageFromContent(t *testing.T) {
	item := model.Item{
		Link: "https://example.com/blog/post",
		Content: `<p><img src="https://s.w.org/images/core/emoji/14.0.0/72x72/1f600.png">
			<img src="https://stats.wordpress.com/b.gif?x=1">
			<img src="/pixel.gif" width="1" height="1">
			<img src="data:image/png;base64,AAAA" srcset="/img/small.jpg 480w, /img/big.jpg 1024w">
			<img src="/img/second.jpg"></p>`,
	}
	if got := Image(item); got != "https://example.com/img/small.jpg" {
		t.Fatalf("got %q", got)
	}
}

func TestImageNone(t *testing.T) {
	if got := Image(model.Item{Content: "<p>no images</p>"}); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := Image(model.Item{Content: `<img src="javascript:alert(1)">`}); got != "" {
		t.Fatalf("unsafe scheme accepted: %q", got)
	}
	bad := model.Item{MediaLinks: model.MediaLinks{{URL: "javascript:alert(1)", Type: "image"}}}
	if got := Image(bad); got != "" {
		t.Fatalf("unsafe media url accepted: %q", got)
	}
}
