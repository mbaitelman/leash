package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/mbaitelman/leash/internal/server"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Leash web UI",
	Long: `Start a local web server with the Leash UI.

The UI lets you browse policy runs, inspect findings, and edit policy files.
Policies are read from and written directly to disk.

Log level precedence: --log-level flag > LOG_LEVEL env var > info

Examples:
  leash serve
  leash serve --port 9090 --runs-dir ./runs
  leash serve --policy ./policies/ --runs-dir ./runs
  LOG_LEVEL=debug leash serve`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetString("port")
		runsDir, _ := cmd.Flags().GetString("runs-dir")

		srv := server.New(port, runsDir, policyPaths, dryRun)

		// Determine initial log level.
		// Precedence: --log-level flag (if explicitly set) > LOG_LEVEL env > "info".
		levelStr := "info"
		if env := os.Getenv("LOG_LEVEL"); env != "" {
			levelStr = strings.ToLower(env)
		}
		if cmd.Flags().Changed("log-level") {
			levelStr = logLevel
		}
		level := slog.LevelInfo
		switch levelStr {
		case "debug":
			level = slog.LevelDebug
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}

		slog.SetDefault(slog.New(srv.SlogHandler(level)))

		fmt.Printf("Leash UI  →  http://localhost:%s\n", port)
		if level == slog.LevelDebug {
			fmt.Printf("Log level →  DEBUG\n")
		}

		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		return srv.Start(ctx)
	},
}

func init() {
	serveCmd.Flags().String("port", "8080", "Port to listen on")
	serveCmd.Flags().String("runs-dir", "./runs", "Directory for storing run results")
}
