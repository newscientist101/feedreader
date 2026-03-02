// Package safenet provides SSRF protection for outbound HTTP requests.
// It blocks requests to private/loopback/link-local IPs and cloud metadata endpoints.
package safenet

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

// privateRanges are CIDR ranges that must not be reached by outbound requests.
var privateRanges []*net.IPNet

func init() {
	for _, cidr := range []string{
		"0.0.0.0/8",          // Current network
		"10.0.0.0/8",         // Private (RFC 1918)
		"100.64.0.0/10",      // Carrier-grade NAT
		"127.0.0.0/8",        // Loopback
		"169.254.0.0/16",     // Link-local / cloud metadata
		"172.16.0.0/12",      // Private (RFC 1918)
		"192.0.0.0/24",       // IETF protocol assignments
		"192.0.2.0/24",       // TEST-NET-1
		"192.88.99.0/24",     // 6to4 relay
		"192.168.0.0/16",     // Private (RFC 1918)
		"198.18.0.0/15",      // Benchmarking
		"198.51.100.0/24",    // TEST-NET-2
		"203.0.113.0/24",     // TEST-NET-3
		"224.0.0.0/4",        // Multicast
		"240.0.0.0/4",        // Reserved
		"255.255.255.255/32", // Broadcast
		// IPv6
		"::1/128",   // Loopback
		"fc00::/7",  // Unique local
		"fe80::/10", // Link-local
		"ff00::/8",  // Multicast
		// Note: IPv4-mapped IPv6 (::ffff:x.x.x.x) is handled by
		// normalizing to IPv4 before checking.
	} {
		_, network, _ := net.ParseCIDR(cidr)
		privateRanges = append(privateRanges, network)
	}
}

// isPrivateIP reports whether ip falls in a blocked range.
func isPrivateIP(ip net.IP) bool {
	// Normalize IPv4-mapped IPv6 addresses (e.g. ::ffff:127.0.0.1 → 127.0.0.1)
	// so the IPv4 private ranges match correctly.
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}

// blockedHosts are hostnames that resolve to metadata services.
var blockedHosts = []string{
	"metadata.google.internal",
	"metadata.google",
}

// ValidateURL checks that a URL is safe to fetch: must be http(s), must not
// target a private IP or known metadata hostname.
func ValidateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	switch u.Scheme {
	case "http", "https":
		// ok
	default:
		return fmt.Errorf("scheme %q not allowed (must be http or https)", u.Scheme)
	}

	hostname := u.Hostname()
	if hostname == "" {
		return fmt.Errorf("empty hostname")
	}

	// Block known metadata hostnames.
	lower := strings.ToLower(hostname)
	if slices.Contains(blockedHosts, lower) {
		return fmt.Errorf("hostname %q is blocked", hostname)
	}

	// If it's a literal IP, check immediately.
	if ip := net.ParseIP(hostname); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("IP address %s is in a blocked range", ip)
		}
	}

	return nil
}

// MaxRedirects is the default redirect limit for safe HTTP clients.
const MaxRedirects = 10

// safeDialer wraps a net.Dialer and checks resolved IPs before connecting.
type safeDialer struct {
	resolver *net.Resolver
	dialer   *net.Dialer
}

func (d *safeDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	ips, err := d.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}

	for _, ipAddr := range ips {
		if isPrivateIP(ipAddr.IP) {
			return nil, fmt.Errorf("resolved IP %s for host %q is in a blocked range", ipAddr.IP, host)
		}
	}

	// Connect to the first resolved address.
	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses found for host %q", host)
	}
	return d.dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}

// WrapTransport returns a copy of base (or http.DefaultTransport if nil) with
// a DialContext that blocks connections to private IPs. The returned transport
// protects against DNS rebinding because IP checks happen at dial time, after
// resolution.
func WrapTransport(base *http.Transport) *http.Transport {
	var cloned *http.Transport
	if base != nil {
		cloned = base.Clone()
	} else {
		cloned = http.DefaultTransport.(*http.Transport).Clone()
	}

	sd := &safeDialer{
		resolver: net.DefaultResolver,
		dialer: &net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		},
	}
	cloned.DialContext = sd.DialContext
	return cloned
}

// redirectChecker returns a CheckRedirect func that validates each hop's URL
// and limits the number of redirects.
func redirectChecker(limit int) func(req *http.Request, via []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= limit {
			return fmt.Errorf("stopped after %d redirects", limit)
		}
		if err := ValidateURL(req.URL.String()); err != nil {
			return fmt.Errorf("redirect target blocked: %w", err)
		}
		return nil
	}
}

// NewSafeClient creates an http.Client with SSRF protections:
//   - Blocks connections to private/loopback/link-local IPs
//   - Validates redirect targets
//   - Limits redirect hops
//
// The provided transport is used as the base (its DialContext is overridden).
// Pass nil to use defaults.
func NewSafeClient(timeout time.Duration, base *http.Transport) *http.Client {
	return &http.Client{
		Timeout:       timeout,
		Transport:     WrapTransport(base),
		CheckRedirect: redirectChecker(MaxRedirects),
	}
}
