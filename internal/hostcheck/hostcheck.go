package hostcheck

import (
	"context"
	"fmt"
	"net"
	"strings"
)

// Resolver performs DNS lookups. *net.Resolver satisfies this interface.
type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

var privateRanges []*net.IPNet

func init() {
	cidrs := []string{
		"0.0.0.0/8",      // "this" network
		"10.0.0.0/8",     // RFC 1918
		"100.64.0.0/10",  // CGNAT
		"127.0.0.0/8",    // loopback
		"169.254.0.0/16", // link-local
		"172.16.0.0/12",  // RFC 1918
		"192.168.0.0/16", // RFC 1918
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local
		"fe80::/10",      // IPv6 link-local
	}
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("hostcheck: invalid CIDR %q: %v", cidr, err))
		}
		privateRanges = append(privateRanges, network)
	}
}

// IsAllowed reports whether hostname is permitted by allowlist.
// An empty allowlist permits every hostname.
func IsAllowed(hostname string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return true
	}
	for _, entry := range allowlist {
		if strings.EqualFold(hostname, entry) {
			return true
		}
	}
	return false
}

// IsPrivateIP reports whether ip falls within any private/reserved range.
func IsPrivateIP(ip net.IP) bool {
	// Normalise to 4-byte form for IPv4 so net.IPNet.Contains works correctly.
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	for _, network := range privateRanges {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// CheckHost validates hostname against allowlist and ensures it resolves only
// to public IP addresses. It returns the first resolved IP on success.
func CheckHost(ctx context.Context, hostname string, allowlist []string, resolver Resolver) (net.IP, error) {
	// Step 1: allowlist check.
	if !IsAllowed(hostname, allowlist) {
		return nil, fmt.Errorf("host %q is not in allowed_hosts", hostname)
	}

	// Step 2: raw IP literal — skip DNS entirely.
	if ip := net.ParseIP(hostname); ip != nil {
		if IsPrivateIP(ip) {
			return nil, fmt.Errorf("address %s is a private/reserved IP", hostname)
		}
		return ip, nil
	}

	// Step 3: DNS resolution.
	if resolver == nil {
		return nil, fmt.Errorf("no DNS resolver configured for host %q", hostname)
	}
	addrs, err := resolver.LookupIPAddr(ctx, hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %q: %w", hostname, err)
	}

	// Step 4: reject if any resolved address is private.
	for _, addr := range addrs {
		if IsPrivateIP(addr.IP) {
			return nil, fmt.Errorf("host %q resolves to private address %s", hostname, addr.IP)
		}
	}

	// Step 5: return first address.
	if len(addrs) == 0 {
		return nil, fmt.Errorf("host %q resolved to no addresses", hostname)
	}
	return addrs[0].IP, nil
}
