package resource

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
)

// Payload shape taken from Datadog's own recorded API fixtures
// (datadog-api-client-go tests/scenarios/cassettes, v2 Users API).
const userJSON = `{
	"type": "users",
	"id": "3ad549bf-eba0-11e9-a77a-0705486660d0",
	"attributes": {
		"name": "Jane Doe",
		"handle": "jane@example.com",
		"email": "jane@example.com",
		"title": null,
		"verified": true,
		"service_account": false,
		"disabled": false,
		"mfa_enabled": false,
		"allowed_login_methods": ["saml", "google_oidc"],
		"status": "Active"
	}
}`

func TestAllowedLoginMethodsFromAdditionalProperties(t *testing.T) {
	var user datadogV2.User
	if err := json.Unmarshal([]byte(userJSON), &user); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	attrs := user.GetAttributes()
	raw, ok := attrs.AdditionalProperties["allowed_login_methods"]
	if !ok {
		t.Fatalf("allowed_login_methods not in AdditionalProperties; got keys: %v", attrs.AdditionalProperties)
	}
	t.Logf("raw type: %T, value: %v", raw, raw)

	r := &userResource{inner: user}
	got, ok := r.Properties()["allowed_login_methods"]
	if !ok {
		t.Fatal("Properties() missing allowed_login_methods")
	}
	want := []string{"saml", "google_oidc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v (%T), want %#v", got, got, want)
	}
}

func TestAllowedLoginMethodsEmptyArray(t *testing.T) {
	var user datadogV2.User
	payload := `{"type":"users","id":"x","attributes":{"email":"a@b.c","allowed_login_methods":[]}}`
	if err := json.Unmarshal([]byte(payload), &user); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	r := &userResource{inner: user}
	got, ok := r.Properties()["allowed_login_methods"]
	if !ok {
		t.Fatal("Properties() missing allowed_login_methods for empty array")
	}
	if methods, isSlice := got.([]string); !isSlice || len(methods) != 0 {
		t.Fatalf("got %#v (%T), want empty []string", got, got)
	}
}
