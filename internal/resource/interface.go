package resource

import (
	"context"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
)

// Resource represents a single Datadog entity fetched from the API.
type Resource interface {
	// Type returns the resource type identifier, e.g. "datadog.monitor"
	Type() string
	// ID returns the unique string identifier for this resource
	ID() string
	// Properties returns a flat map of field paths to values used by filters.
	// Keys use dot-notation for nested fields: "options.notify_no_data"
	Properties() map[string]any
	// Raw returns the original SDK struct for use by actions
	Raw() any
}

// Provider fetches all resources of a given type from the Datadog API.
type Provider interface {
	ResourceType() string
	List(ctx context.Context, client *datadog.APIClient) ([]Resource, error)
}

// Taggable is implemented by resources that support adding tags via the API.
type Taggable interface {
	AddTags(ctx context.Context, tags []string) error
}

// TagRemovable is implemented by resources that support removing tags via the API.
type TagRemovable interface {
	RemoveTags(ctx context.Context, tags []string) error
}

// Deletable is implemented by resources that support deletion via the API.
type Deletable interface {
	Delete(ctx context.Context) error
}
