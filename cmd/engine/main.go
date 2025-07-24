// Package main provides the Engine server entry point for the Nina application.
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/engine"
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
		noColor    = flag.Bool("no-color", false, "Disable color output")
	)
	flag.Parse()

	// Set log level based on verbose flag
	if *verbose {
		*logLevel = "debug"
	}

	// Initialize logger
	log := logger.New(logger.Level(*logLevel), *logFormat)
	if !*noColor {
		log.ForceColor() // Force color output for better visibility
	}

	// Debug terminal info
	log.DebugTerminalInfo()

	log.Info("Starting Nina Engine")

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

	// Initialize Engine server
	server := engine.NewEngine(cfg, log, st)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Info("Received shutdown signal", "signal", sig)
		cancel()
	}()

	// Start the server
	log.Info("Starting server", "addr", cfg.GetServerAddr())
	if err := server.Start(ctx); err != nil {
		log.Error("Server failed", "error", err)
		cancel()
		os.Exit(1)
	}

	log.Info("Server stopped")
}
