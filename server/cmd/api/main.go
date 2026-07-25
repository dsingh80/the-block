// Command api is the long-running HTTP+WS server (guidelines/06-backend-architecture.md).
// Plain HTTP internally -- Caddy is the only thing that terminates TLS
// (guidelines/06-backend-architecture.md, "Deployment & HTTPS").
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dsingh80/the-block/server/internal/platform/config"
	"github.com/dsingh80/the-block/server/internal/platform/inmemory"
	"github.com/dsingh80/the-block/server/internal/platform/logging"
	"github.com/dsingh80/the-block/server/internal/platform/pgstore"
	"github.com/dsingh80/the-block/server/internal/platform/redisstore"
	transporthttp "github.com/dsingh80/the-block/server/internal/transport/http"
	"github.com/dsingh80/the-block/server/internal/usecase/health"
)

// bidRateLimitPerWindow/bidRateLimitWindow bound bid/buy-now attempts per
// session (guidelines/06-backend-architecture.md, "SOC2 principles mapping") --
// generous enough not to block a real user double-checking a bid, tight enough
// to blunt a scripted griefing attempt. Not config-driven: this is a policy
// knob with one sane default, not something an operator needs to tune per
// deployment yet.
const (
	bidRateLimitPerWindow = 20
	bidRateLimitWindow    = time.Minute
	streamTailerInterval  = 500 * time.Millisecond
	shutdownGracePeriod   = 10 * time.Second
)

func main() {
	slog.SetDefault(logging.New(os.Stdout))
	cfg := config.Load()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect to postgres failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb := redisstore.NewClient(cfg.RedisAddr)
	defer rdb.Close()

	listingReader := pgstore.NewListingReader(pool)
	bidReader := pgstore.NewBidReader(pool)
	// bidStore satisfies both bidding.Store (PlaceBid/BuyNow) and
	// bidding.ViewerLookup (BidListingIDs) -- the same Redis-backed type reads
	// what its own accept path writes (guidelines/06-backend-architecture.md,
	// "Computing viewer without joins").
	bidStore := redisstore.NewBidStore(rdb, redisstore.DefaultIdempotencyTTL)
	sessionStore := redisstore.NewSessionStore(rdb, redisstore.DefaultSessionTTL)
	rateLimiter := redisstore.NewRateLimiter(rdb, bidRateLimitPerWindow, bidRateLimitWindow)
	broadcaster := inmemory.NewBroadcaster()

	redisPinger := health.PingerFunc(func(ctx context.Context) error { return rdb.Ping(ctx).Err() })
	// *pgxpool.Pool already has Ping(context.Context) error -- satisfies
	// health.Pinger directly, no adapter needed.

	tailer := redisstore.NewStreamTailer(rdb, pool, broadcaster)
	tailerCtx, stopTailer := context.WithCancel(ctx)
	defer stopTailer()
	go func() {
		if err := tailer.Run(tailerCtx, streamTailerInterval); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("stream tailer stopped unexpectedly", "error", err)
		}
	}()

	router := transporthttp.NewRouter(
		listingReader, bidReader, bidStore, bidStore, rateLimiter, sessionStore,
		redisPinger, pool, broadcaster, []string{cfg.PublicOrigin},
	)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: router}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("api server listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		slog.Error("server failed", "error", err)
		os.Exit(1)
	case <-stop:
		slog.Info("shutting down")
	}

	stopTailer()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
}
