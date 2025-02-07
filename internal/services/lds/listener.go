package lds

import (
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	tcpproxy "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/negrel/aegis/internal/pbutils"
	"github.com/negrel/aegis/internal/services/cds"
	"github.com/negrel/aegis/internal/xnet"
)

// Listener is a named network location (e.g., port, unix domain socket, etc.)
// that can be connected to by downstream clients. Envoy exposes one or more
// listeners that downstream hosts connect to.
type Listener struct {
	Name         string
	Address      xnet.SocketAddr
	FilterChains [][]Filter
}

// Filter define a listener filter.
// See https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/listeners/listener_filters.
type Filter interface {
	ToFilter() *listener.Filter
}

func (l *Listener) ToResource() types.Resource {
	host, port := l.Address.HostPort()
	resource := &listener.Listener{
		Name: l.Name,
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Address:       host,
					PortSpecifier: &core.SocketAddress_PortValue{PortValue: uint32(port)},
				},
			},
		},
		FilterChains: []*listener.FilterChain{},
	}

	for _, chain := range l.FilterChains {
		var filters []*listener.Filter
		for _, f := range chain {
			filters = append(filters, f.ToFilter())
		}

		if filters != nil {
			resource.FilterChains = append(resource.FilterChains,
				&listener.FilterChain{
					Filters: filters,
				},
			)
		}
	}

	return resource
}

type TcpProxyFilter struct {
	Cluster *cds.Cluster
}

// ToFilter implements Filter.
func (tpf TcpProxyFilter) ToFilter() *listener.Filter {
	return &listener.Filter{
		Name: "envoy.filters.network.tcp_proxy",
		ConfigType: &listener.Filter_TypedConfig{
			TypedConfig: pbutils.MustMarshalAny(&tcpproxy.TcpProxy{
				StatPrefix: "tcp-proxy",
				ClusterSpecifier: &tcpproxy.TcpProxy_Cluster{
					Cluster: tpf.Cluster.Name,
				},
			}),
		},
	}
}
