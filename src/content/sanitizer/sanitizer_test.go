package sanitizer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const baseURL = "http://example.org/"

func readFixture(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("unable to read fixture %q: %v", path, err)
	}
	return strings.TrimSuffix(string(data), "\n")
}

func TestSanitizeTestData(t *testing.T) {
	files, err := filepath.Glob("testdata/*.in.html")
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".in.html")
		t.Run(name, func(t *testing.T) {
			raw := readFixture(t, file)
			want := readFixture(t, filepath.Join("testdata", name+".out.html"))
			have := Sanitize(baseURL, raw)

			if want != have {
				t.Errorf("Wrong output:\nwant: %s\nhave: %s", want, have)
			}
		})
	}
}

func TestSanitizeRemovesTrackingPixels(t *testing.T) {
	input := `<p>hi<img src="https://t.example.com/p.gif" width="1" height="1"><img src="https://example.com/a.jpg" width="600" height="400"></p>`
	got := Sanitize("https://example.com", input)
	if strings.Contains(got, "p.gif") {
		t.Errorf("tracking pixel not removed: %s", got)
	}
	if !strings.Contains(got, "a.jpg") {
		t.Errorf("real image removed: %s", got)
	}
}
