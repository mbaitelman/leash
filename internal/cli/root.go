package cli

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var (
	policyPaths  []string
	dryRun       bool
	outputFormat string
	logLevel     string
)

var rootCmd = &cobra.Command{
	Use:   "leash",
	Short: "Leash — Datadog governance framework",
	Long: `Leash evaluates declarative YAML policies against Datadog resources
and reports compliance findings.

Required environment variables:
  DD_API_KEY   Datadog API key
  DD_APP_KEY   Datadog Application key
  DD_SITE      Datadog site (default: datadoghq.com)`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		level := slog.LevelInfo
		switch logLevel {
		case "debug":
			level = slog.LevelDebug
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
		return nil
	},
}

// Execute is the CLI entry point.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringArrayVarP(&policyPaths, "policy", "p", []string{"./policies"}, "Policy files or directories")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", true, "Simulate actions without mutating (default: true)")
	rootCmd.PersistentFlags().StringVar(&outputFormat, "output-format", "json", "Output format: json or text")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level: debug, info, warn, error")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(schemaCmd)
	rootCmd.AddCommand(listResourcesCmd)
	rootCmd.AddCommand(serveCmd)
}
