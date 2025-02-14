package lds

import (
	accesslog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	accesslogfile "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/file/v3"
	httprouter "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	httpman "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	tcpproxy "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/negrel/aegis/internal/pbutils"
	"github.com/negrel/aegis/internal/xds/cds"
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

// HttpProxyFilter is a listener filter to process HTTP streams.
type HttpProxyFilter struct {
	HttpFilters []HttpFilter
	RouteConfig RouteConfig
}

func (hpf HttpProxyFilter) ToFilter() *listener.Filter {
	filters := make([]*httpman.HttpFilter, len(hpf.HttpFilters))
	for i, f := range hpf.HttpFilters {
		filters[i] = f.ToHttpFilter()
	}

	return &listener.Filter{
		Name: "envoy.filters.network.http_connection_manager",
		ConfigType: &listener.Filter_TypedConfig{
			TypedConfig: pbutils.MustMarshalAny(&httpman.HttpConnectionManager{
				StatPrefix: "http-conn-man",
				AccessLog: []*accesslog.AccessLog{
					{
						Name: "envoy.access_loggers.stdout",
						ConfigType: &accesslog.AccessLog_TypedConfig{
							TypedConfig: pbutils.MustMarshalAny(&accesslogfile.FileAccessLog{
								Path:            "/dev/stdout",
								AccessLogFormat: &accesslogfile.FileAccessLog_Format{},
							}),
						},
					},
				},
				HttpFilters: filters,
				RouteSpecifier: &httpman.HttpConnectionManager_RouteConfig{
					RouteConfig: hpf.RouteConfig.toRouteConfig(),
				},
			}),
		},
	}
}

// HttpFilter define a filter processing HTTP streams.
type HttpFilter interface {
	ToHttpFilter() *httpman.HttpFilter
}

// HttpRouter define an HTTP router filter.
// See https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/http/router/v3/router.proto#envoy-v3-api-msg-extensions-filters-http-router-v3-router
type HttpRouter struct{}

func (hr HttpRouter) ToHttpFilter() *httpman.HttpFilter {
	return &httpman.HttpFilter{
		Name: "envoy.filters.http.router",
		ConfigType: &httpman.HttpFilter_TypedConfig{
			TypedConfig: pbutils.MustMarshalAny(&httprouter.Router{})},
		IsOptional: false,
		Disabled:   false,
	}
}

// RouteConfig define HTTP route configurations.
// See https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/route/v3/route.proto#envoy-v3-api-msg-config-route-v3-routeconfiguration
type RouteConfig struct {
	Name         string
	VirtualHosts []VirtualHost
}

func (rc RouteConfig) toRouteConfig() *route.RouteConfiguration {
	vhosts := make([]*route.VirtualHost, len(rc.VirtualHosts))
	for i, f := range rc.VirtualHosts {
		vhosts[i] = f.toVirtualHost()
	}

	return &route.RouteConfiguration{
		Name:         rc.Name,
		VirtualHosts: vhosts,
	}
}

// VirtualHost define virtual HTTP host.
// See https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/route/v3/route_components.proto#envoy-v3-api-msg-config-route-v3-virtualhost
type VirtualHost struct {
	Name    string
	Domains []string
	Cluster *cds.Cluster
}

func (vh VirtualHost) toVirtualHost() *route.VirtualHost {
	return &route.VirtualHost{
		Name:    vh.Name,
		Domains: vh.Domains,
		Routes: []*route.Route{
			{
				Name: "route",
				Match: &route.RouteMatch{
					PathSpecifier: &route.RouteMatch_Prefix{Prefix: "/"},
				},
				Action: &route.Route_Route{
					Route: &route.RouteAction{
						ClusterSpecifier: &route.RouteAction_Cluster{Cluster: vh.Cluster.Name},
					},
				},
			},
		},
		RequireTls: route.VirtualHost_NONE,
	}
}
