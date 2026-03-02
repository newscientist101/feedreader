package safenet

import (
	"net"
	"net/http"
	"testing"
	"time"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		// Allowed
		{"public HTTPS", "https://example.com/feed.xml", false},
		{"public HTTP", "http://example.com/feed.xml", false},
		{"public IP", "http://93.184.216.34/feed", false},

		// Blocked schemes
		{"ftp scheme", "ftp://example.com/", true},
		{"file scheme", "file:///etc/passwd", true},
		{"gopher scheme", "gopher://example.com/", true},
		{"no scheme", "example.com/feed", true},

		// Blocked IPs
		{"loopback", "http://127.0.0.1/", true},
		{"loopback alt", "http://127.0.0.2/", true},
		{"private 10.x", "http://10.0.0.1/", true},
		{"private 172.x", "http://172.16.0.1/", true},
		{"private 192.168.x", "http://192.168.1.1/", true},
		{"link-local", "http://169.254.169.254/", true},
		{"metadata IP", "http://169.254.169.254/latest/meta-data/", true},
		{"zero IP", "http://0.0.0.0/", true},
		{"broadcast", "http://255.255.255.255/", true},
		{"IPv6 loopback", "http://[::1]/", true},

		// Blocked hostnames
		{"metadata google", "http://metadata.google.internal/", true},

		// Edge cases
		{"empty hostname", "http:///path", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip      string
		private bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.0.1", true},
		{"169.254.169.254", true},
		{"8.8.8.8", false},
		{"93.184.216.34", false},
		{"::1", true},
		{"fc00::1", true},
		{"2607:f8b0:4004:800::200e", false}, // Google public IPv6
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP %q", tt.ip)
			}
			got := isPrivateIP(ip)
			if got != tt.private {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, got, tt.private)
			}
		})
	}
}

func TestRedirectChecker(t *testing.T) {
	check := redirectChecker(3)

	// Exceeding redirect limit
	via := make([]*http.Request, 3)
	req, _ := http.NewRequest("GET", "https://example.com/", http.NoBody)
	err := check(req, via)
	if err == nil {
		t.Error("expected error when redirect limit exceeded")
	}

	// Under limit with valid URL
	via2 := make([]*http.Request, 1)
	err = check(req, via2)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Under limit but blocked redirect target
	badReq, _ := http.NewRequest("GET", "http://127.0.0.1/", http.NoBody)
	err = check(badReq, via2)
	if err == nil {
		t.Error("expected error for redirect to loopback")
	}
}

func TestNewSafeClient(t *testing.T) {
	client := NewSafeClient(5*time.Second, nil)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", client.Timeout)
	}
	if client.CheckRedirect == nil {
		t.Error("expected CheckRedirect to be set")
	}
}
