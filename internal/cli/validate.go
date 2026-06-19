package cli

import (
	"fmt"

	"github.com/mbaitelman/leash/internal/action"
	"github.com/mbaitelman/leash/internal/filter"
	"github.com/mbaitelman/leash/internal/policy"
	"github.com/mbaitelman/leash/internal/resource"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate policy files without executing",
	Long: `Parse and validate YAML policy files. Checks:
  - Required fields present (name, resource)
  - Resource type is registered
  - Filter types are valid
  - Action types are valid

Exits with code 0 on success, non-zero if any policy is invalid.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		policies, err := policy.LoadPaths(policyPaths)
		if err != nil {
			return err
		}

		var errs []error
		for _, pol := range policies {
			if verr := validatePolicy(pol); verr != nil {
				errs = append(errs, fmt.Errorf("policy %q: %w", pol.Name, verr))
			}
		}

		if len(errs) > 0 {
			for _, e := range errs {
				fmt.Printf("ERROR: %s\n", e)
			}
			return fmt.Errorf("%d policy validation error(s)", len(errs))
		}

		fmt.Printf("OK: %d policies valid\n", len(policies))
		return nil
	},
}

func validatePolicy(pol policy.Policy) error {
	if _, err := resource.Get(pol.Resource); err != nil {
		return err
	}

	for _, spec := range pol.Filters {
		if _, err := filter.Build(spec); err != nil {
			return fmt.Errorf("filter: %w", err)
		}
	}

	for _, spec := range pol.Actions {
		m := map[string]any(spec)
		actionType, ok := m["type"].(string)
		if !ok || actionType == "" {
			return fmt.Errorf("action spec missing 'type' field")
		}
		factory, err := action.Get(actionType)
		if err != nil {
			return err
		}
		if _, err := factory(m); err != nil {
			return fmt.Errorf("action %q: %w", actionType, err)
		}
	}

	return nil
}
