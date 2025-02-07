package main

import (
	"fmt"

	"github.com/negrel/aegis/internal/services/ads"
	"github.com/negrel/aegis/internal/services/cds"
	"github.com/negrel/aegis/internal/services/lds"
	"github.com/negrel/aegis/internal/xnet"
	"github.com/negrel/sgo"
)

// Start ADS gRPC server.
func StartAds(n sgo.Nursery) (*ads.Service, uint16, error) {
	// Create xDS services.
	ads := ads.ProvideService(
		lds.ProvideService(),
		cds.ProvideService(),
	)
	lis, adsPort, err := xnet.RandomListener("tcp")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to setup TCP listener: %w", err)
	}

	n.Go(func() error {
		err := ads.Serve(lis)
		if err != nil {
			panic(err)
		}
		return nil
	})
	n.Go(func() error {
		<-n.Done()
		ads.GracefulStop()
		return nil
	})

	return ads, adsPort, nil
}
