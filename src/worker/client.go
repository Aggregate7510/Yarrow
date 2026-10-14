package worker

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// MaxResponseBytes caps how much of any remote response is read. Feeds or
// pages larger than this are rejected rather than exhausting memory on small
// hosts such as a Raspberry Pi.
const MaxResponseBytes = 16 << 20 // 16 MiB

var errTooLarge = fmt.Errorf("response exceeds %d bytes", MaxResponseBytes)

type Client struct {
	httpClient *http.Client
	userAgent  string
}

func (c *Client) get(url string) (*http.Response, error) {
	return c.getConditional(url, "", "")
}

func (c *Client) getConditional(url, lastModified, etag string) (*http.Response, error) {
	if err := ValidateFetchURL(url); err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	res.Body = &limitedBody{rc: res.Body, remaining: MaxResponseBytes}
	return res, nil
}

// limitedBody returns an error (instead of a silent EOF) once more than
// `remaining` bytes have been read, so truncated documents are never parsed.
type limitedBody struct {
	rc        io.ReadCloser
	remaining int64
}

func (l *limitedBody) Read(p []byte) (int, error) {
	if l.remaining <= 0 {
		// probe for one more byte to distinguish "exactly at limit" from "over"
		var one [1]byte
		n, err := l.rc.Read(one[:])
		if n > 0 {
			return 0, errTooLarge
		}
		return 0, err
	}
	if int64(len(p)) > l.remaining {
		p = p[:l.remaining]
	}
	n, err := l.rc.Read(p)
	l.remaining -= int64(n)
	return n, err
}

func (l *limitedBody) Close() error { return l.rc.Close() }

var client *Client

func SetVersion(num string) {
	client.userAgent = "Yarrow/" + num
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	// Scheme/host sanity on every hop; the IP itself is checked at dial time.
	return ValidateFetchURL(req.URL.String())
}

func newHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           guardedDialContext,
		DisableKeepAlives:     true,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
	}
	return &http.Client{
		Timeout:       30 * time.Second,
		Transport:     transport,
		CheckRedirect: checkRedirect,
	}
}

func init() {
	client = &Client{
		httpClient: newHTTPClient(),
		userAgent:  "Yarrow/1.0",
	}
}
