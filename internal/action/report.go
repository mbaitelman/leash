package action

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mbaitelman/leash/internal/resource"
)

func init() {
	Register("report", newReportAction)
}

type reportAction struct{}

func newReportAction(_ map[string]any) (Action, error) {
	return &reportAction{}, nil
}

func (a *reportAction) Type() string { return "report" }

// Execute logs the matched resource. The engine collects full findings separately;
// this action provides per-resource visibility during execution.
func (a *reportAction) Execute(_ context.Context, r resource.Resource, dryRun bool) error {
	slog.Info("match",
		"resource_type", r.Type(),
		"resource_id", r.ID(),
		"dry_run", dryRun,
	)
	fmt.Printf("[match] %s  id=%s\n", r.Type(), r.ID())
	return nil
}
