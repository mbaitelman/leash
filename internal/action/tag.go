package action

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mbaitelman/leash/internal/resource"
)

func init() {
	Register("tag", newTagAction)
}

type tagAction struct {
	tags []string
}

func newTagAction(spec map[string]any) (Action, error) {
	raw, ok := spec["tags"]
	if !ok {
		return nil, fmt.Errorf("tag action requires 'tags'")
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("tag action 'tags' must be a list")
	}
	tags := make([]string, 0, len(items))
	for _, item := range items {
		tags = append(tags, fmt.Sprintf("%v", item))
	}
	return &tagAction{tags: tags}, nil
}

func (a *tagAction) Type() string { return "tag" }

func (a *tagAction) Execute(ctx context.Context, r resource.Resource, dryRun bool) error {
	taggable, ok := r.(resource.Taggable)
	if !ok {
		return fmt.Errorf("tag action: resource type %q does not support tagging", r.Type())
	}

	// Skip tags already present on the resource.
	existing := toStringSet(r.Properties()["tags"])
	var missing []string
	for _, t := range a.tags {
		if !existing[t] {
			missing = append(missing, t)
		}
	}
	if len(missing) == 0 {
		slog.Info("tag (skipped, already present)", "resource_id", r.ID(), "tags", a.tags)
		return nil
	}

	if dryRun {
		slog.Info("tag (dry-run)", "resource_id", r.ID(), "tags", missing)
		return nil
	}

	return taggable.AddTags(ctx, missing)
}

func toStringSet(v any) map[string]bool {
	set := map[string]bool{}
	switch tags := v.(type) {
	case []string:
		for _, t := range tags {
			set[t] = true
		}
	case []any:
		for _, t := range tags {
			set[fmt.Sprintf("%v", t)] = true
		}
	}
	return set
}
