package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"ai-monitor/internal/config"
	"ai-monitor/internal/logger"
	"ai-monitor/internal/lifecycle"
)

var version = "1.0.0"

func main() {
	configPath := flag.String("config", "", "Path to config.yaml")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("AI Monitor v%s\n", version)
		return
	}

	fmt.Println("========================================")
	fmt.Println("  AI Monitor v" + version)
	fmt.Println("  Windows Desktop Monitor for DeepSeek")
	fmt.Println("========================================")

	// Load configuration
	cfg, err := config.Load(*configPath)

	// Initialize file logger
	if err := logger.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: logger init: %v\n", err)
	}
	logger.Info("AI Monitor v%s starting", version)

	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: load config: %v\n", err)
		os.Exit(1)
	}

	// Validate API key
	if cfg.DeepSeek.APIKey == "" {
		fmt.Fprintf(os.Stderr, "WARNING: DeepSeek API key is not configured.\n")
		fmt.Fprintf(os.Stderr, "  Edit config.yaml and set deepseek.api_key\n")
		fmt.Fprintf(os.Stderr, "  Or place config.yaml in %%APPDATA%%/AI-Monitor/config.yaml\n")
	}

	// Create application
	app, err := lifecycle.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: create app: %v\n", err)
		os.Exit(1)
	}

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\n[main] signal received, shutting down...")
		app.Shutdown()
		os.Exit(0)
	}()

	// Run blocks until tray exits
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: run: %v\n", err)
		os.Exit(1)
	}
}
