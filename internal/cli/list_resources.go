package cli

import (
	"fmt"

	"github.com/mbaitelman/leash/internal/resource"
	"github.com/spf13/cobra"
)

var listResourcesCmd = &cobra.Command{
	Use:   "list-resources",
	Short: "List all registered resource types",
	RunE: func(cmd *cobra.Command, args []string) error {
		types := resource.ListTypes()
		if len(types) == 0 {
			fmt.Println("No resource types registered.")
			return nil
		}
		fmt.Println("Available resource types:")
		for _, t := range types {
			fmt.Printf("  %s\n", t)
		}
		return nil
	},
}
