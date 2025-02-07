package xnet

import (
	"net"
	"strconv"
)

// RandomListener allocates a random port for the provided network.
func RandomListener(network string) (net.Listener, uint16, error) {
	// Create listener on random port.
	lis, err := net.Listen(network, ":0")
	if err != nil {
		return nil, 0, err
	}

	// Extract port to configure envoy.
	_, tcpPortStr, err := net.SplitHostPort(lis.Addr().String())
	if err != nil {
		panic(err)
	}
	tcpPort, err := strconv.Atoi(tcpPortStr)
	if err != nil {
		panic(err)
	}

	return lis, uint16(tcpPort), err
}
