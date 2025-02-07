package xds

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"sync/atomic"

	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	server "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/negrel/aegis/internal/services/cds"
	"github.com/negrel/aegis/internal/services/lds"
	"google.golang.org/grpc"
)

// Service define an Envoy xDS service.
type Service interface {
	Snapshot(ctx context.Context) error
}

// ProvideService is a wire provider for Envoy xDS server service.
func ProvideService(grpcSrv *grpc.Server, lds lds.Service, cds cds.Service) Service {
	cache := cache.NewSnapshotCache(true, cache.IDHash{}, nil)
	srv := server.NewServer(context.Background(), cache, nil)

	// Register services
	discovery.RegisterAggregatedDiscoveryServiceServer(grpcSrv, srv)

	return &service{
		cache: cache,
		lds:   lds,
		cds:   cds,
	}
}

type service struct {
	mu      sync.Mutex
	version atomic.Uint64
	cache   cache.SnapshotCache
	lds     lds.Service
	cds     cds.Service
}

// Snapshot implements Service.
func (s *service) Snapshot(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	versionNum := s.version.Add(1)
	version := strconv.FormatUint(versionNum, 10)

	listeners := s.lds.Resources()
	clusters := s.cds.Resources()
	snapshot, err := cache.NewSnapshot(version,
		map[resource.Type][]types.Resource{
			resource.ClusterType:  clusters,
			resource.ListenerType: listeners,
		},
	)
	if err != nil {
		return err
	}

	err = s.cache.SetSnapshot(context.Background(), "expo-envoy", snapshot)
	if err != nil {
		return fmt.Errorf("failed to set snapshot: %v", err)
	}

	log.Println("set snapshot done", clusters, listeners)

	return nil
}
