package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Print the JSON Schema for policy YAML files",
	RunE: func(cmd *cobra.Command, args []string) error {
		schema := buildPolicySchema()
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(schema); err != nil {
			return fmt.Errorf("encoding schema: %w", err)
		}
		return nil
	},
}

func buildPolicySchema() map[string]any {
	filterOps := []string{
		"eq", "ne", "gt", "lt", "gte", "lte",
		"contains", "not-contains",
		"regex", "not-regex",
		"in", "not-in",
		"present", "absent",
	}

	valueFilter := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type":  map[string]any{"type": "string", "const": "value"},
			"key":   map[string]any{"type": "string", "description": "Dot-notation field path"},
			"op":    map[string]any{"type": "string", "enum": filterOps},
			"value": map[string]any{"description": "Comparison value (omit for present/absent)"},
		},
		"required": []string{"type", "key"},
	}

	tagFilter := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type":  map[string]any{"type": "string", "const": "tag"},
			"key":   map[string]any{"type": "string", "description": "Tag key"},
			"value": map[string]any{"type": "string", "description": "Tag value (optional)"},
			"op":    map[string]any{"type": "string", "enum": []string{"present", "absent", "eq"}},
		},
		"required": []string{"type", "key"},
	}

	ageFilter := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type":  map[string]any{"type": "string", "const": "age"},
			"key":   map[string]any{"type": "string"},
			"op":    map[string]any{"type": "string", "enum": []string{"older-than", "newer-than"}},
			"value": map[string]any{"type": "string", "description": "Duration string, e.g. '30d', '24h'"},
		},
		"required": []string{"type", "key", "op", "value"},
	}

	filterSchema := map[string]any{
		"oneOf": []any{valueFilter, tagFilter, ageFilter},
	}

	policySchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":        map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"resource": map[string]any{
				"type": "string",
				"enum": []string{
					"datadog.monitor",
					"datadog.slo",
					"datadog.synthetic",
					"datadog.dashboard",
					"datadog.user",
				},
			},
			"filters": map[string]any{
				"type":  "array",
				"items": filterSchema,
			},
			"actions": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"type": map[string]any{
							"type": "string",
							"enum": []string{"report", "notify", "tag", "delete"},
						},
					},
					"required": []string{"type"},
				},
			},
		},
		"required": []string{"name", "resource"},
	}

	return map[string]any{
		"$schema":     "https://json-schema.org/draft/2020-12/schema",
		"title":       "Leash Policy Schema",
		"description": "Schema for Leash governance policy YAML files",
		"type":        "object",
		"properties": map[string]any{
			"policies": map[string]any{
				"type":  "array",
				"items": policySchema,
			},
		},
		"required": []string{"policies"},
	}
}
