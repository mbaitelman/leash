package cli

import (
	"fmt"
	"os"

	"github.com/mbaitelman/leash/internal/config"
	"github.com/mbaitelman/leash/internal/engine"
	"github.com/mbaitelman/leash/internal/output"
	"github.com/mbaitelman/leash/internal/policy"
	"github.com/spf13/cobra"
)

var outputFile string

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute policies against Datadog",
	Long: `Load and execute YAML policies against live Datadog resources.
Outputs a JSON findings report by default.

Examples:
  leash run --policy ./policies/
  leash run --policy ./policies/monitor-naming.yaml --dry-run=false
  leash run --policy ./policies/ --output-file findings.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		policies, err := policy.LoadPaths(policyPaths)
		if err != nil {
			return fmt.Errorf("loading policies: %w", err)
		}
		if len(policies) == 0 {
			return fmt.Errorf("no policies found in %v", policyPaths)
		}

		client, ctx, err := config.BuildClient()
		if err != nil {
			return err
		}

		eng := engine.New(ctx, client)
		report, err := eng.Run(policies, dryRun)
		if err != nil {
			return err
		}

		w := os.Stdout
		if outputFile != "" {
			f, err := os.Create(outputFile)
			if err != nil {
				return fmt.Errorf("creating output file: %w", err)
			}
			defer f.Close()
			w = f
		}

		if outputFormat == "text" {
			return output.WriteText(w, report)
		}
		return output.WriteJSON(w, report)
	},
}

func init() {
	runCmd.Flags().StringVar(&outputFile, "output-file", "", "Write findings to file instead of stdout")
}
