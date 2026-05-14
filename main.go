package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	defaultAddr    = ":8080"
	defaultTimeout = 30 * time.Second
	version        = "dev"
)

func main() {
	var (
		addr        string
		readTimeout time.Duration
		proxyURL    string
		showVersion bool
	)

	flag.StringVar(&addr, "addr", envOrDefault("LADDER_ADDR", defaultAddr), "address to listen on")
	flag.DurationVar(&readTimeout, "timeout", defaultTimeout, "request timeout duration")
	flag.StringVar(&proxyURL, "proxy", envOrDefault("LADDER_PROXY", ""), "upstream proxy URL (optional)")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Printf("ladder version %s\n", version)
		os.Exit(0)
	}

	log.Printf("starting ladder %s on %s", version, addr)

	handler, err := newHandler(handlerConfig{
		proxyURL: proxyURL,
		timeout:  readTimeout,
	})
	if err != nil {
		log.Fatalf("failed to create handler: %v", err)
	}

	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  readTimeout,
		WriteTimeout: readTimeout + 5*time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in a goroutine so we can handle shutdown signals
	serverErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if err != nil {
			log.Fatalf("server error: %v", err)
		}
	case sig := <-quit:
		log.Printf("received signal %s, shutting down", sig)
	}

	// Graceful shutdown with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}

	log.Println("ladder stopped")
}

// envOrDefault returns the value of the environment variable named by key,
// or fallback if the variable is not set or empty.
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
