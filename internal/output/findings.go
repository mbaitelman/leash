package output

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"
)

// FindingsReport is the top-level output structure — the stable contract for any future UI consumer.
type FindingsReport struct {
	RunID       string         `json:"run_id"`
	GeneratedAt time.Time      `json:"generated_at"`
	DryRun      bool           `json:"dry_run"`
	Policies    []PolicyResult `json:"policies"`
}

// PolicyResult holds the result of evaluating one policy.
type PolicyResult struct {
	PolicyName   string          `json:"policy_name"`
	Resource     string          `json:"resource"`
	MatchCount   int             `json:"match_count"`
	Matches      []ResourceMatch `json:"matches"`
	PassCount    int             `json:"pass_count"`
	Passing      []ResourceMatch `json:"passing"`
	ActionsTaken []ActionRecord  `json:"actions_taken"`
}

// ResourceMatch is one resource that passed all filters.
type ResourceMatch struct {
	ID         string         `json:"id"`
	Properties map[string]any `json:"properties"`
}

// ActionRecord records what an action did (or would have done) to a resource.
type ActionRecord struct {
	ResourceID string `json:"resource_id"`
	ActionType string `json:"action_type"`
	DryRun     bool   `json:"dry_run"`
	Success    bool   `json:"success"`
	Message    string `json:"message,omitempty"`
	Error      string `json:"error,omitempty"`
}

// WriteJSON writes the report as indented JSON.
func WriteJSON(w io.Writer, report *FindingsReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// WriteText writes a human-readable summary table.
func WriteText(w io.Writer, report *FindingsReport) error {
	fmt.Fprintf(w, "Leash Run  %s\n", report.RunID)
	fmt.Fprintf(w, "Generated: %s\n", report.GeneratedAt.Format(time.RFC3339))
	if report.DryRun {
		fmt.Fprintln(w, "Mode:      DRY RUN (no mutations)")
	} else {
		fmt.Fprintln(w, "Mode:      LIVE")
	}
	fmt.Fprintln(w)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "POLICY\tRESOURCE\tMATCHES")
	for _, p := range report.Policies {
		fmt.Fprintf(tw, "%s\t%s\t%d\n", p.PolicyName, p.Resource, p.MatchCount)
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	for _, p := range report.Policies {
		if p.MatchCount == 0 {
			continue
		}
		fmt.Fprintf(w, "\n--- %s ---\n", p.PolicyName)
		for _, m := range p.Matches {
			fmt.Fprintf(w, "  %s\n", m.ID)
		}
	}
	return nil
}
