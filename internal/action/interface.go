package action

import (
	"context"

	"github.com/mbaitelman/leash/internal/resource"
)

// Action performs an operation on a matched resource.
// Mutating actions must be no-ops when dryRun is true.
type Action interface {
	Type() string
	Execute(ctx context.Context, r resource.Resource, dryRun bool) error
}

// PassAction is optionally implemented by actions that can also operate on
// resources that pass (do not match) a policy's filters.
type PassAction interface {
	ExecuteOnPass(ctx context.Context, r resource.Resource, dryRun bool) error
}

// Factory constructs an Action from the raw YAML spec map.
type Factory func(spec map[string]any) (Action, error)
