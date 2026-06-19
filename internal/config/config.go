package config

import (
	"context"
	"fmt"
	"os"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
)

// BuildClient constructs the Datadog API client and an authenticated context
// from environment variables: DD_API_KEY, DD_APP_KEY, DD_SITE (optional).
func BuildClient() (*datadog.APIClient, context.Context, error) {
	apiKey := os.Getenv("DD_API_KEY")
	appKey := os.Getenv("DD_APP_KEY")

	if apiKey == "" {
		return nil, nil, fmt.Errorf("DD_API_KEY environment variable is required")
	}
	if appKey == "" {
		return nil, nil, fmt.Errorf("DD_APP_KEY environment variable is required")
	}

	ctx := datadog.NewDefaultContext(context.Background())
	configuration := datadog.NewConfiguration()

	if site := os.Getenv("DD_SITE"); site != "" {
		configuration.Host = "api." + site
	}

	client := datadog.NewAPIClient(configuration)
	return client, ctx, nil
}
