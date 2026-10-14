package worker

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestIsPublicAddr(t *testing.T) {
	t.Cleanup(func() { SetAllowedNetworks("") })
	SetAllowedNetworks("")

	blocked := []string{
		"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.10", "169.254.169.254",
		"100.100.100.100", "0.0.0.0", "224.0.0.1", "255.255.255.255",
		"::1", "::", "fe80::1", "fd00:ec2::254", "fc00::1", "::ffff:127.0.0.1",
		"::ffff:169.254.169.254", "64:ff9b::a00:1", "2002:a00:1::",
	}
	for _, s := range blocked {
		if IsPublicAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s should be blocked", s)
		}
	}
	allowed := []string{"1.1.1.1", "93.184.216.34", "2606:4700:4700::1111", "::ffff:8.8.8.8"}
	for _, s := range allowed {
		if !IsPublicAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s should be allowed", s)
		}
	}
}

func TestAllowedNetworks(t *testing.T) {
	t.Cleanup(func() { SetAllowedNetworks("") })
	if err := SetAllowedNetworks("192.168.1.0/24, 10.0.0.5"); err != nil {
		t.Fatal(err)
	}
	if !IsPublicAddr(netip.MustParseAddr("192.168.1.20")) {
		t.Error("192.168.1.20 should be allowed")
	}
	if !IsPublicAddr(netip.MustParseAddr("10.0.0.5")) {
		t.Error("10.0.0.5 should be allowed")
	}
	if IsPublicAddr(netip.MustParseAddr("10.0.0.6")) {
		t.Error("10.0.0.6 should stay blocked")
	}
	if IsPublicAddr(netip.MustParseAddr("169.254.169.254")) {
		t.Error("metadata address must stay blocked")
	}
	if err := SetAllowedNetworks("not-a-cidr"); err == nil {
		t.Error("expected error for invalid network")
	}
}

func TestClientBlocksLoopback(t *testing.T) {
	SetAllowedNetworks("")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("secret"))
	}))
	defer srv.Close()

	c := &Client{httpClient: newHTTPClient(), userAgent: "test"}
	for _, u := range []string{srv.URL, strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)} {
		_, err := c.get(u)
		if err == nil || !errors.Is(err, ErrBlockedAddress) {
			t.Errorf("%s: expected blocked error, got %v", u, err)
		}
	}
}

func TestClientBlocksRedirectToInternal(t *testing.T) {
	// The "public" server is allowed explicitly; the redirect target is a
	// second loopback server on a different port that is NOT allowed. Since
	// both are on 127.0.0.1 we emulate by allowing only the first port through
	// a custom check: allow 127.0.0.1 for the first hop then disallow.
	t.Cleanup(func() { SetAllowedNetworks("") })
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("internal"))
	}))
	defer internal.Close()
	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// once the first hop is served, revoke the allowance to emulate the
		// redirect target being an internal address
		SetAllowedNetworks("")
		http.Redirect(w, r, internal.URL, http.StatusFound)
	}))
	defer public.Close()

	SetAllowedNetworks("127.0.0.1")
	c := &Client{httpClient: newHTTPClient(), userAgent: "test"}
	_, err := c.get(public.URL)
	if err == nil || !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("expected redirect to internal address to be blocked, got %v", err)
	}
}

func TestClientRejectsSchemes(t *testing.T) {
	c := &Client{httpClient: newHTTPClient(), userAgent: "test"}
	for _, u := range []string{"file:///etc/passwd", "gopher://x", "http://user:pass@example.com/", "http:///nohost"} {
		if _, err := c.get(u); err == nil {
			t.Errorf("%s: expected error", u)
		}
	}
}

func TestLimitedBody(t *testing.T) {
	body := &limitedBody{rc: io.NopCloser(strings.NewReader("0123456789")), remaining: 5}
	_, err := io.ReadAll(body)
	if !errors.Is(err, errTooLarge) {
		t.Fatalf("expected errTooLarge, got %v", err)
	}
	exact := &limitedBody{rc: io.NopCloser(strings.NewReader("01234")), remaining: 5}
	b, err := io.ReadAll(exact)
	if err != nil || string(b) != "01234" {
		t.Fatalf("exact-size body should read fine: %q %v", b, err)
	}
}
