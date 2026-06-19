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

Schedule precedence: --schedule flag > LEASH_SCHEDULE env var
Log level precedence: --log-level flag > LOG_LEVEL env var > info

Examples:
  leash serve
  leash serve --port 9090 --runs-dir ./runs
  leash serve --policy ./policies/ --runs-dir ./runs
  leash serve --schedule "0 * * * *"
  LEASH_SCHEDULE="*/30 * * * *" leash serve`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetString("port")
		runsDir, _ := cmd.Flags().GetString("runs-dir")

		// Schedule precedence: --schedule flag > LEASH_SCHEDULE env var
		schedule, _ := cmd.Flags().GetString("schedule")
		if schedule == "" {
			schedule = os.Getenv("LEASH_SCHEDULE")
		}

		srv := server.New(port, runsDir, policyPaths, dryRun, schedule)

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
		if schedule != "" {
			fmt.Printf("Schedule  →  %s\n", schedule)
		}
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
	serveCmd.Flags().String("schedule", "", `Cron expression for automatic runs, e.g. "0 * * * *" (hourly). Overrides LEASH_SCHEDULE env var.`)
}
