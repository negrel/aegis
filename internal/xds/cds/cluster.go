package cds

import (
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/negrel/aegis/internal/xnet"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// A cluster is a group of logically similar upstream hosts that Envoy connects
// to. Envoy discovers the members of a cluster via service discovery. It
// optionally determines the health of cluster members via active health
// checking. The cluster member that Envoy routes a request to is determined by
// the load balancing policy.
type Cluster struct {
	Name           string
	ConnectTimeout time.Duration
	LbPolicy       cluster.Cluster_LbPolicy
	Endpoints      []xnet.SocketAddr
	TcpKeepAlive   *TcpKeepAlive
}

func (c *Cluster) ToResource() types.Resource {
	resource := &cluster.Cluster{
		Name:           c.Name,
		ConnectTimeout: durationpb.New(c.ConnectTimeout),
		LbPolicy:       c.LbPolicy,
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: c.Name,
			Endpoints: []*endpoint.LocalityLbEndpoints{{
				LbEndpoints: []*endpoint.LbEndpoint{},
			}},
		},
		UpstreamConnectionOptions: &cluster.UpstreamConnectionOptions{
			TcpKeepalive: c.TcpKeepAlive.ToCoreTcpKeepAlive(),
		},
	}

	for _, addr := range c.Endpoints {
		host, port := addr.HostPort()

		resource.LoadAssignment.Endpoints[0].LbEndpoints = append(resource.LoadAssignment.Endpoints[0].LbEndpoints, &endpoint.LbEndpoint{
			HostIdentifier: &endpoint.LbEndpoint_Endpoint{
				Endpoint: &endpoint.Endpoint{
					Address: &core.Address{
						Address: &core.Address_SocketAddress{
							SocketAddress: &core.SocketAddress{
								Address:       host,
								PortSpecifier: &core.SocketAddress_PortValue{PortValue: uint32(port)},
								Ipv4Compat:    true,
							},
						},
					},
				},
			},
		})
	}

	return resource
}

// Cluster TcpKeepAlive options.
// See https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/core/v3/address.proto#envoy-v3-api-msg-config-core-v3-tcpkeepalive
type TcpKeepAlive struct {
	Probes   uint32
	Time     uint32
	Interval uint32
}

var TcpKeepAliveDefault = TcpKeepAlive{
	Probes:   9,
	Time:     7200, // 2h
	Interval: 75,   // 75s
}

func (tka *TcpKeepAlive) ToCoreTcpKeepAlive() *core.TcpKeepalive {
	if tka == nil {
		return nil
	}

	return &core.TcpKeepalive{
		KeepaliveProbes:   wrapperspb.UInt32(tka.Probes),
		KeepaliveTime:     wrapperspb.UInt32(tka.Time),
		KeepaliveInterval: wrapperspb.UInt32(tka.Interval),
	}
}
