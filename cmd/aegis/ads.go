package main

import (
	"fmt"

	"github.com/negrel/aegis/internal/xds/ads"
	"github.com/negrel/aegis/internal/xds/cds"
	"github.com/negrel/aegis/internal/xds/lds"
	"github.com/negrel/aegis/internal/xnet"
	"github.com/negrel/conc"
)

// Start ADS gRPC server.
func StartAds(n conc.Nursery) (*ads.Service, uint16, error) {
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
