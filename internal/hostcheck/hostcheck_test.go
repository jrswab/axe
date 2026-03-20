package hostcheck_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/hostcheck"
)

// fakeResolver is a test double for the Resolver interface.
type fakeResolver struct {
	addrs []net.IPAddr
	err   error
}

func (f *fakeResolver) LookupIPAddr(_ context.Context, _ string) ([]net.IPAddr, error) {
	return f.addrs, f.err
}

// panicResolver panics if called — used to assert DNS is never consulted.
type panicResolver struct{}

func (p *panicResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	panic(fmt.Sprintf("LookupIPAddr should not have been called for %q", host))
}

// ---------------------------------------------------------------------------
// IsAllowed
// ---------------------------------------------------------------------------

func TestIsAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		hostname  string
		allowlist []string
		want      bool
	}{
		{
			name:      "empty allowlist allows any hostname",
			hostname:  "anything.com",
			allowlist: []string{},
			want:      true,
		},
		{
			name:      "exact match returns true",
			hostname:  "api.example.com",
			allowlist: []string{"api.example.com"},
			want:      true,
		},
		{
			name:      "non-match returns false",
			hostname:  "other.com",
			allowlist: []string{"api.example.com"},
			want:      false,
		},
		{
			name:      "case-insensitive match",
			hostname:  "API.Example.COM",
			allowlist: []string{"api.example.com"},
			want:      true,
		},
		{
			name:      "subdomain does not match parent entry",
			hostname:  "sub.api.example.com",
			allowlist: []string{"api.example.com"},
			want:      false,
		},
		{
			name:      "parent domain does not match child entry",
			hostname:  "example.com",
			allowlist: []string{"api.example.com"},
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := hostcheck.IsAllowed(tc.hostname, tc.allowlist)
			if got != tc.want {
				t.Errorf("IsAllowed(%q, %v) = %v; want %v", tc.hostname, tc.allowlist, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// IsPrivateIP
// ---------------------------------------------------------------------------

func TestIsPrivateIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"loopback IPv4", "127.0.0.1", true},
		{"loopback IPv6", "::1", true},
		{"RFC 1918 10/8", "10.0.0.1", true},
		{"RFC 1918 172.16/12 start", "172.16.0.1", true},
		{"RFC 1918 192.168/16", "192.168.1.1", true},
		{"link-local IPv4", "169.254.1.1", true},
		{"link-local IPv6", "fe80::1", true},
		{"unique local IPv6 fd00", "fd00::1", true},
		{"unique local IPv6 fc00", "fc00::1", true},
		{"this network", "0.0.0.1", true},
		{"CGNAT", "100.64.0.1", true},
		{"public IPv4", "8.8.8.8", false},
		{"public IPv6", "2001:4860:4860::8888", false},
		{"just below RFC 1918 172.16/12", "172.15.255.255", false},
		{"RFC 1918 172.16/12 boundary start", "172.16.0.0", true},
		{"RFC 1918 172.16/12 boundary end", "172.31.255.255", true},
		{"just above RFC 1918 172.16/12", "172.32.0.0", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("invalid test IP: %q", tc.ip)
			}
			got := hostcheck.IsPrivateIP(ip)
			if got != tc.want {
				t.Errorf("IsPrivateIP(%s) = %v; want %v", tc.ip, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CheckHost
// ---------------------------------------------------------------------------

func TestCheckHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		hostname  string
		allowlist []string
		resolver  hostcheck.Resolver
		wantIP    net.IP
		wantErr   string // substring expected in error, empty = no error
	}{
		{
			name:      "allowlist non-empty, hostname not in list",
			hostname:  "evil.com",
			allowlist: []string{"api.example.com"},
			resolver:  &fakeResolver{},
			wantErr:   "not in allowed_hosts",
		},
		{
			name:      "allowlist non-empty, hostname in list, resolves to public IP",
			hostname:  "api.example.com",
			allowlist: []string{"api.example.com"},
			resolver: &fakeResolver{
				addrs: []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}},
			},
			wantIP:  net.ParseIP("8.8.8.8"),
			wantErr: "",
		},
		{
			name:      "allowlist non-empty, hostname in list, resolves to private IP",
			hostname:  "api.example.com",
			allowlist: []string{"api.example.com"},
			resolver: &fakeResolver{
				addrs: []net.IPAddr{{IP: net.ParseIP("10.0.0.1")}},
			},
			wantErr: "private address",
		},
		{
			name:      "empty allowlist, resolves to public IP",
			hostname:  "open.example.com",
			allowlist: []string{},
			resolver: &fakeResolver{
				addrs: []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}},
			},
			wantIP:  net.ParseIP("8.8.8.8"),
			wantErr: "",
		},
		{
			name:      "empty allowlist, resolves to private IP",
			hostname:  "internal.example.com",
			allowlist: []string{},
			resolver: &fakeResolver{
				addrs: []net.IPAddr{{IP: net.ParseIP("10.0.0.1")}},
			},
			wantErr: "private address",
		},
		{
			name:      "raw private IP literal bypasses DNS",
			hostname:  "192.168.1.1",
			allowlist: []string{},
			resolver:  &panicResolver{},
			wantErr:   "private",
		},
		{
			name:      "raw public IP literal with empty allowlist",
			hostname:  "8.8.8.8",
			allowlist: []string{},
			resolver:  &panicResolver{},
			wantIP:    net.ParseIP("8.8.8.8"),
			wantErr:   "",
		},
		{
			name:      "raw public IP with non-empty allowlist is rejected",
			hostname:  "8.8.8.8",
			allowlist: []string{"example.com"},
			resolver:  &panicResolver{},
			wantErr:   "not in allowed_hosts",
		},
		{
			name:      "mixed IPs (public + private) are blocked",
			hostname:  "mixed.example.com",
			allowlist: []string{},
			resolver: &fakeResolver{
				addrs: []net.IPAddr{
					{IP: net.ParseIP("8.8.8.8")},
					{IP: net.ParseIP("10.0.0.1")},
				},
			},
			wantErr: "private address",
		},
		{
			name:      "nil resolver returns error",
			hostname:  "example.com",
			allowlist: []string{},
			resolver:  nil,
			wantErr:   "no DNS resolver configured",
		},
		{
			name:      "DNS resolution error is returned with hostname",
			hostname:  "bad.example.com",
			allowlist: []string{},
			resolver:  &fakeResolver{err: fmt.Errorf("dns timeout")},
			wantErr:   "failed to resolve",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotIP, err := hostcheck.CheckHost(context.Background(), tc.hostname, tc.allowlist, tc.resolver)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q; got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %q; want substring %q", err.Error(), tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.wantIP != nil && !tc.wantIP.Equal(gotIP) {
				t.Errorf("IP = %v; want %v", gotIP, tc.wantIP)
			}
		})
	}
}
