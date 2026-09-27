package httpapi

import (
	"fmt"
	"net"
)

// DefaultAddr is the listen address when ambit start is given no --addr flag.
const DefaultAddr = "127.0.0.1:8080"

// ValidateAddr accepts only a loopback IP and a port. Hostnames are rejected.
// 0.0.0.0 and every other non-loopback host are rejected before listen.
func ValidateAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("listen address %q: %w", addr, err)
	}
	if port == "" {
		return fmt.Errorf("listen address %q has no port", addr)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("listen address %q is not a loopback address", addr)
	}
	return nil
}
