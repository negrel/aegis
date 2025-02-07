package cds

import (
	"sync"

	"github.com/elliotchance/orderedmap/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
)

const ResourceType = types.Cluster

// Service define an Envoy CDS (Cluster Discovery Service) server service.
type Service interface {
	SetCluster(*Cluster) bool
	RemoveCluster(name string) bool
	Resources() []types.Resource
}

// ProvideService define a wire provider for Envoy CDS server service.
func ProvideService() Service {
	return &service{
		clusters: orderedmap.NewOrderedMap[string, *Cluster](),
	}
}

type service struct {
	mu       sync.Mutex
	clusters *orderedmap.OrderedMap[string, *Cluster]
}

// SetCluster implements Service.
func (s *service) SetCluster(c *Cluster) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.clusters.Set(c.Name, c)
}

// RemoveCluster implements Service.
func (s *service) RemoveCluster(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.clusters.Delete(name)
}

// Resources implements Service.
func (s *service) Resources() []types.Resource {
	s.mu.Lock()
	defer s.mu.Unlock()

	var resources []types.Resource

	for _, c := range s.clusters.AllFromFront() {
		resources = append(resources, c.ToResource())
	}

	return resources
}
