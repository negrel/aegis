package ads

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"

	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	server "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/negrel/aegis/internal/xds/cds"
	"github.com/negrel/aegis/internal/xds/lds"
	"google.golang.org/grpc"
)

// Service define an Envoy Aggregated xDS Service.
type Service struct {
	mu         sync.Mutex
	version    atomic.Uint64
	cache      cache.SnapshotCache
	grpcServer *grpc.Server

	LDS lds.Service
	CDS cds.Service
}

// ProvideService is a wire provider for Envoy ADS server service.
func ProvideService(
	lds lds.Service,
	cds cds.Service,
) *Service {
	grpcSrv := grpc.NewServer()

	cache := cache.NewSnapshotCache(true, cache.IDHash{}, nil)
	srv := server.NewServer(context.Background(), cache, nil)

	// Register services
	discovery.RegisterAggregatedDiscoveryServiceServer(grpcSrv, srv)

	return &Service{
		grpcServer: grpcSrv,
		cache:      cache,
		LDS:        lds,
		CDS:        cds,
	}
}

// Serve starts gRPC server.
func (s *Service) Serve(lis net.Listener) error {
	return s.grpcServer.Serve(lis)
}

// GracefulStop stops the gRPC server gracefully. It stops the server from
// accepting new connections and RPCs and blocks until all the pending RPCs are
// finished.
func (s *Service) GracefulStop() {
	s.grpcServer.GracefulStop()
}

// Snapshot implements Service.
func (s *Service) Snapshot(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	versionNum := s.version.Add(1)
	version := strconv.FormatUint(versionNum, 10)

	listeners := s.LDS.Resources()
	clusters := s.CDS.Resources()
	snapshot, err := cache.NewSnapshot(version,
		map[resource.Type][]types.Resource{
			resource.ClusterType:  clusters,
			resource.ListenerType: listeners,
		},
	)
	if err != nil {
		return err
	}

	err = s.cache.SetSnapshot(ctx, "expo-envoy", snapshot)
	if err != nil {
		return fmt.Errorf("failed to set snapshot: %v", err)
	}

	return nil
}
