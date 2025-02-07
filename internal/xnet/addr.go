package xnet

import (
	"context"
	"errors"
	"net"
	"net/netip"
)

// Addr define a socket address (TCP/UDP).
type SocketAddr interface {
	HostPort() (host string, port uint16)
}

type IPSocketAddr struct {
	Host netip.Addr
	Port uint16
}

// HostPort implements SocketAddr.
func (ipsa IPSocketAddr) HostPort() (host string, port uint16) {
	return ipsa.Host.String(), ipsa.Port
}

type hostSocketAddr struct {
	host string
	port uint16
}

// HostPort implements SocketAddr.
func (h hostSocketAddr) HostPort() (host string, port uint16) {
	return h.host, h.port
}

var (
	ErrNoAddresses = errors.New("no associated addresses")
)

// HostSocketAddr returns a new SocketAddr that returns the given host and port
// after validating that the host exists.
func HostSocketAddr(ctx context.Context, host string, port uint16) (SocketAddr, error) {
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, ErrNoAddresses
	}

	return hostSocketAddr{host, port}, nil
}
