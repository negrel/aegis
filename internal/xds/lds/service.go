package lds

import (
	"sync"

	"github.com/elliotchance/orderedmap/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
)

const ResourceType = types.Listener

// Service define an Envoy LDS (Listener Discovery Service) server service.
type Service interface {
	SetListener(*Listener) bool
	RemoveListener(name string) bool
	Resources() []types.Resource
}

// ProvideService define a wire provider for Envoy LDS server service.
func ProvideService() Service {
	return &service{
		listeners: orderedmap.NewOrderedMap[string, *Listener](),
	}
}

type service struct {
	mu        sync.Mutex
	listeners *orderedmap.OrderedMap[string, *Listener]
}

// RemoveListener implements Service.
func (s *service) RemoveListener(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.listeners.Delete(name)
}

// Resources implements Service.
func (s *service) Resources() []types.Resource {
	s.mu.Lock()
	defer s.mu.Unlock()

	var resources []types.Resource
	for _, r := range s.listeners.AllFromFront() {
		resources = append(resources, r.ToResource())
	}

	return resources
}

// SetListener implements Service.
func (s *service) SetListener(l *Listener) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.listeners.Set(l.Name, l)
}
