package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/ingress"
	"github.com/matiasinsaurralde/nina/pkg/logger"
	"github.com/matiasinsaurralde/nina/pkg/store"
)

func main() {
	// Parse command line flags
	var (
		configPath = flag.String("config", "", "Path to configuration file")
		logLevel   = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		logFormat  = flag.String("log-format", "text", "Log format (text, json)")
		verbose    = flag.Bool("verbose", false, "Enable verbose logging")
	)
	flag.Parse()

	// Set log level based on verbose flag
	if *verbose {
		*logLevel = "debug"
	}

	// Initialize logger
	log := logger.New(logger.Level(*logLevel), *logFormat)
	log.Info("Starting Nina Ingress")

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatal("Failed to load configuration", "error", err)
	}

	log.Info("Configuration loaded", "config_path", *configPath)

	// Initialize store
	st, err := store.NewStore(cfg, log)
	if err != nil {
		log.Fatal("Failed to initialize store", "error", err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			log.Error("Failed to close store", "error", err)
		}
	}()

	// Initialize ingress
	ing := ingress.NewIngress(cfg, log, st)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Info("Received shutdown signal", "signal", sig)
		cancel()
	}()

	// Start the ingress
	log.Info("Starting ingress", "addr", cfg.GetIngressAddr())
	if err := ing.Start(ctx); err != nil {
		log.Fatal("Ingress failed", "error", err)
	}

	log.Info("Ingress stopped")
}
