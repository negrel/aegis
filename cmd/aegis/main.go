package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	"github.com/negrel/aegis/internal/services/cds"
	"github.com/negrel/aegis/internal/services/lds"
	"github.com/negrel/aegis/internal/xnet"
	"github.com/negrel/sgo"
	"github.com/spf13/pflag"
)

func main() {
	help := pflag.BoolP("help", "h", false, "Print this help and exit")
	debug := pflag.Bool("debug", false, "Enable debug logs")
	port := pflag.Uint16P("port", "p", 8080, "Listening port")
	domain := pflag.StringP("domain", "d", "*", "Service domain name")

	pflag.Parse()

	if *help {
		printUsage()
		return
	}

	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}

	// Setup logger.
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	err := aegisMain(logger, Args{
		domain:    *domain,
		port:      *port,
		remaining: pflag.Args(),
	})
	if err != nil {
		logger.Error("unexpected error occured", slog.Any("error", err))
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "aegis 0.1.0")
	fmt.Fprintln(os.Stderr, "Alexandre Negrel <alexandre@negrel.dev>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "aegis stands for API Edge Gateway Integration System. It is designed")
	fmt.Fprintln(os.Stderr, "to protects and forwards traffic to your services.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "USAGE:")
	fmt.Fprintln(os.Stderr, "  aegis [OPTIONS] SERVICE...")
	fmt.Fprintln(os.Stderr, "  aegis 'deno run -A ./main.ts --port=$PORT'")
	fmt.Fprintln(os.Stderr, "  aegis 'deno run ./main.ts' 'python main.py'")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Options:")
	pflag.PrintDefaults()
}

type Args struct {
	domain    string
	port      uint16
	remaining []string
}

func aegisMain(logger *slog.Logger, args Args) error {
	if args.port == 0 {
		return fmt.Errorf("please specify a valid port")
	}
	if args.domain == "" {
		return fmt.Errorf("please specify a valid domain")
	}
	if len(args.remaining) != 1 {
		return fmt.Errorf("please specify a single command")
	}
	service := args.remaining[0]

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	cancelf := func(format string, args ...any) error {
		cancel()
		return fmt.Errorf(format, args...)
	}

	return sgo.Block(func(n sgo.Nursery) error {
		// Cancel nursery on signal.
		n.Go(func() error {
			<-signals
			logger.Info("signal received, stopping services...")
			cancel()
			return nil
		})

		// Start ADS gRPC server.
		ads, adsPort, err := StartAds(n)
		if err != nil {
			return cancelf("failed to start ADS gRPC server: %w", err)
		}

		// Start envoy.
		err = StartEnvoy(n, logger, adsPort, 9901)
		if err != nil {
			return cancelf("failed to start envoy: %w", err)
		}

		// Start service.
		servicePort, err := StartService(
			n,
			logger.With(slog.Any("service", service)),
			service,
		)
		if err != nil {
			return cancelf("failed to start service process: %w", err)
		}

		// Create cluster.
		serviceCluster := &cds.Cluster{
			Name:           "service",
			ConnectTimeout: time.Second,
			LbPolicy:       cluster.Cluster_ROUND_ROBIN,
			Endpoints: []xnet.SocketAddr{
				xnet.IPSocketAddr{
					Host: netip.MustParseAddr("127.0.0.1"),
					Port: servicePort,
				},
			},
			TcpKeepAlive: nil,
		}
		ads.CDS.SetCluster(serviceCluster)

		// Create listener.
		ads.LDS.SetListener(&lds.Listener{
			Name: "entrypoint",
			Address: xnet.IPSocketAddr{
				Host: netip.MustParseAddr("0.0.0.0"),
				Port: args.port,
			},
			FilterChains: [][]lds.Filter{{
				lds.HttpProxyFilter{
					HttpFilters: []lds.HttpFilter{lds.HttpRouter{}},
					RouteConfig: lds.RouteConfig{
						Name: "service",
						VirtualHosts: []lds.VirtualHost{
							{
								Name:    "service",
								Domains: []string{"*"},
								Cluster: serviceCluster,
							},
						},
					},
				},
			}},
		})

		// Create snapshot of initial configuration.
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err = ads.Snapshot(ctx)
		if err != nil {
			return cancelf("failed to create xDS snapshot: %w", err)
		}

		return nil
	}, sgo.WithContext(ctx))
}
