package resource

import (
	"context"
	"fmt"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
)

func init() {
	Register(&userProvider{})
}

// ---- Provider ----

type userProvider struct{}

func (p *userProvider) ResourceType() string { return "datadog.user" }

func (p *userProvider) List(ctx context.Context, client *datadog.APIClient) ([]Resource, error) {
	api := datadogV2.NewUsersApi(client)
	items, cancel := api.ListUsersWithPagination(ctx, *datadogV2.NewListUsersOptionalParameters().WithPageSize(500))
	defer cancel()

	var resources []Resource
	for item := range items {
		if item.Error != nil {
			return nil, fmt.Errorf("listing users: %w", item.Error)
		}
		resources = append(resources, &userResource{inner: item.Item, client: client})
	}
	return resources, nil
}

// ---- Resource ----

type userResource struct {
	inner  datadogV2.User
	client *datadog.APIClient
}

func (r *userResource) Type() string { return "datadog.user" }
func (r *userResource) ID() string   { return r.inner.GetId() }
func (r *userResource) Raw() any     { return r.inner }

func (r *userResource) Properties() map[string]any {
	props := map[string]any{}
	if r.inner.Id != nil {
		props["id"] = *r.inner.Id
	}
	attrs := r.inner.GetAttributes()
	if attrs.Email != nil {
		props["email"] = *attrs.Email
	}
	if v := attrs.Name.Get(); v != nil {
		props["name"] = *v
	}
	if v := attrs.Title.Get(); v != nil {
		props["title"] = *v
	}
	if attrs.Disabled != nil {
		props["disabled"] = *attrs.Disabled
	}
	if attrs.CreatedAt != nil {
		props["created"] = *attrs.CreatedAt
	}
	if attrs.ModifiedAt != nil {
		props["modified"] = *attrs.ModifiedAt
	}
	if attrs.Status != nil {
		props["status"] = *attrs.Status
	}
	if attrs.ServiceAccount != nil {
		props["service_account"] = *attrs.ServiceAccount
	}
	return props
}

// Disable disables the user (Datadog does not hard-delete users).
func (r *userResource) Delete(ctx context.Context) error {
	api := datadogV2.NewUsersApi(r.client)
	_, err := api.DisableUser(ctx, r.inner.GetId())
	return err
}
