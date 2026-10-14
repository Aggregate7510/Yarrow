package worker

// Network guard for every outbound request made by the feed fetcher,
// favicon finder and the "read here" page extractor.
//
// The check runs at *connect* time on the IP address the socket is about to
// use (net.Dialer.Control), not on the URL string. That means it also covers:
//
//   - hostnames that resolve to internal addresses,
//   - DNS rebinding (the address that was resolved is the one that is checked),
//   - HTTP redirects to internal hosts (every hop dials through the guard),
//   - IPv4-mapped IPv6 and other alternate spellings of internal addresses.
//
// Operators who intentionally subscribe to feeds on their LAN (for example an
// RSS-Bridge instance) can allow specific networks with YARR_ALLOWED_NETWORKS.

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ErrBlockedAddress is returned when a request would connect to a
// non-public address that has not been explicitly allowed.
var ErrBlockedAddress = errors.New("blocked: destination is not a public internet address")

var blockedPrefixes = mustPrefixes(
	"0.0.0.0/8",          // "this" network
	"10.0.0.0/8",         // RFC1918
	"100.64.0.0/10",      // CGNAT (also Tailscale's address range)
	"127.0.0.0/8",        // loopback
	"169.254.0.0/16",     // link-local, incl. cloud metadata 169.254.169.254
	"172.16.0.0/12",      // RFC1918
	"192.0.0.0/24",       // IETF protocol assignments
	"192.0.2.0/24",       // TEST-NET-1
	"192.88.99.0/24",     // 6to4 relay anycast
	"192.168.0.0/16",     // RFC1918
	"198.18.0.0/15",      // benchmarking
	"198.51.100.0/24",    // TEST-NET-2
	"203.0.113.0/24",     // TEST-NET-3
	"224.0.0.0/4",        // multicast
	"240.0.0.0/4",        // reserved + broadcast
	"::/128",             // unspecified
	"::1/128",            // loopback
	"64:ff9b::/96",       // NAT64 (could embed an internal IPv4)
	"64:ff9b:1::/48",     // local-use NAT64
	"100::/64",           // discard-only
	"2001::/32",          // Teredo (embeds IPv4)
	"2001:db8::/32",      // documentation
	"2002::/16",          // 6to4 (embeds IPv4)
	"fc00::/7",           // unique local (incl. fd00:ec2::254 metadata)
	"fe80::/10",          // link-local
	"ff00::/8",           // multicast
)

func mustPrefixes(cidrs ...string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(cidrs))
	for _, c := range cidrs {
		out = append(out, netip.MustParsePrefix(c))
	}
	return out
}

type netGuard struct {
	mu      sync.RWMutex
	allowed []netip.Prefix
	// proxyHosts are host:port values of configured HTTP(S) proxies. Connections
	// to the proxy itself are permitted (the proxy then fetches the public URL).
	proxyHosts map[string]bool
}

var guard = &netGuard{proxyHosts: map[string]bool{}}

// SetAllowedNetworks configures CIDR ranges (comma or space separated) that
// are reachable despite being non-public, e.g. "192.168.1.0/24". A bare IP is
// treated as a single-host range.
func SetAllowedNetworks(spec string) error {
	var prefixes []netip.Prefix
	for _, part := range strings.FieldsFunc(spec, func(r rune) bool { return r == ',' || r == ' ' }) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(part, "/") {
			addr, err := netip.ParseAddr(part)
			if err != nil {
				return fmt.Errorf("invalid allowed network %q: %w", part, err)
			}
			prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
			continue
		}
		p, err := netip.ParsePrefix(part)
		if err != nil {
			return fmt.Errorf("invalid allowed network %q: %w", part, err)
		}
		prefixes = append(prefixes, p.Masked())
	}
	guard.mu.Lock()
	guard.allowed = prefixes
	guard.mu.Unlock()
	return nil
}

func init() {
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if v := os.Getenv(key); v != "" {
			if u, err := url.Parse(v); err == nil && u.Host != "" {
				host := u.Host
				if u.Port() == "" {
					if u.Scheme == "https" {
						host += ":443"
					} else {
						host += ":80"
					}
				}
				guard.proxyHosts[strings.ToLower(host)] = true
			}
		}
	}
}

// IsPublicAddr reports whether ip may be contacted by the fetcher.
func IsPublicAddr(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() {
		return false
	}
	guard.mu.RLock()
	allowed := guard.allowed
	guard.mu.RUnlock()
	for _, p := range allowed {
		if p.Contains(ip) {
			return true
		}
	}
	for _, p := range blockedPrefixes {
		if p.Contains(ip) {
			return false
		}
	}
	return ip.IsGlobalUnicast()
}

func guardControl(network, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("unexpected dial address %q", address)
	}
	if !IsPublicAddr(ip) {
		return fmt.Errorf("%w (%s)", ErrBlockedAddress, ip)
	}
	return nil
}

var (
	guardedDialer = &net.Dialer{Timeout: 10 * time.Second, Control: guardControl}
	plainDialer   = &net.Dialer{Timeout: 10 * time.Second}
)

// guardedDialContext dials through the guard unless the destination is the
// configured outbound proxy.
func guardedDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if guard.proxyHosts[strings.ToLower(addr)] {
		return plainDialer.DialContext(ctx, network, addr)
	}
	return guardedDialer.DialContext(ctx, network, addr)
}

// ValidateFetchURL performs cheap syntactic checks before a request is made.
// The authoritative check is the dial-time guard above.
func ValidateFetchURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported url scheme %q", u.Scheme)
	}
	if u.Hostname() == "" {
		return errors.New("url has no host")
	}
	if u.User != nil {
		return errors.New("urls with embedded credentials are not allowed")
	}
	return nil
}
