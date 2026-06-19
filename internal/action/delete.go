package action

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mbaitelman/leash/internal/resource"
)

func init() {
	Register("delete", newDeleteAction)
}

type deleteAction struct {
	confirmed bool
}

func newDeleteAction(spec map[string]any) (Action, error) {
	confirmed, _ := spec["confirm"].(bool)
	return &deleteAction{confirmed: confirmed}, nil
}

func (a *deleteAction) Type() string { return "delete" }

func (a *deleteAction) Execute(ctx context.Context, r resource.Resource, dryRun bool) error {
	deletable, ok := r.(resource.Deletable)
	if !ok {
		return fmt.Errorf("delete action: resource type %q does not support deletion", r.Type())
	}

	if !a.confirmed {
		return fmt.Errorf("delete action: 'confirm: true' must be set explicitly in the policy")
	}

	if dryRun {
		slog.Info("delete (dry-run)", "resource_id", r.ID(), "resource_type", r.Type())
		return nil
	}

	slog.Warn("deleting resource", "resource_id", r.ID(), "resource_type", r.Type())
	return deletable.Delete(ctx)
}
