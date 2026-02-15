package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/michaelbrown/forge/internal/config"
	"github.com/michaelbrown/forge/internal/server"
	"github.com/michaelbrown/forge/internal/storage/sqlite"
	"github.com/michaelbrown/forge/internal/tools"
)

var portFlag int

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Forge web server",
	Long: `Start the Forge HTTP server with REST API and WebSocket support.

The web UI is available at the root URL. API endpoints are under /api.

Examples:
  forge serve
  forge serve --port 9090`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().IntVar(&portFlag, "port", 0, "Port to listen on (overrides config)")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Open storage
	store, err := sqlite.Open(cfg.Storage.DBPath)
	if err != nil {
		return fmt.Errorf("opening storage: %w", err)
	}
	defer store.Close()

	// Create tool registry
	registry := tools.NewRegistry()
	defer registry.Close()

	for name, toolCfg := range cfg.Tools {
		if err := registry.Register(name, toolCfg); err != nil {
			log.Printf("Warning: failed to start tool server %s: %v", name, err)
		}
	}

	if registry.HasTools() {
		log.Println("Tools: MCP servers loaded")
	} else {
		log.Println("Tools: builtin shell_exec")
	}

	// Determine port
	port := cfg.Server.Port
	if portFlag > 0 {
		port = portFlag
	}

	// Create and start server
	srv := server.New(cfg, store, registry)

	// Graceful shutdown on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		srv.Shutdown(context.Background())
	}()

	return srv.Start(port)
}
