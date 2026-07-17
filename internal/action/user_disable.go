package action

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mbaitelman/leash/internal/resource"
)

func init() {
	Register("user.disable", newUserDisableAction)
}

type userDisableAction struct {
	confirmed bool
}

func newUserDisableAction(spec map[string]any) (Action, error) {
	confirmed, _ := spec["confirm"].(bool)
	return &userDisableAction{confirmed: confirmed}, nil
}

func (a *userDisableAction) Type() string { return "user.disable" }

func (a *userDisableAction) Execute(ctx context.Context, r resource.Resource, dryRun bool) error {
	if r.Type() != "datadog.user" {
		return fmt.Errorf("user.disable action: not valid for resource type %q", r.Type())
	}

	disabler, ok := r.(interface{ Disable(context.Context) error })
	if !ok {
		return fmt.Errorf("user.disable action: resource type %q does not support disabling", r.Type())
	}

	if !a.confirmed {
		return fmt.Errorf("user.disable action: 'confirm: true' must be set explicitly in the policy")
	}

	if dryRun {
		slog.Info("user.disable (dry-run)", "resource_id", r.ID(), "resource_type", r.Type())
		return nil
	}

	slog.Warn("disabling user", "resource_id", r.ID(), "resource_type", r.Type())
	return disabler.Disable(ctx)
}
