package resource

import (
	"reflect"
	"testing"
)

func TestFlattenInto(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want map[string]any
	}{
		{
			name: "nested maps flatten with dot keys",
			in: map[string]any{
				"evt": map[string]any{"name": "Dashboard", "outcome": "success"},
				"usr": map[string]any{"email": "a@b.com"},
			},
			want: map[string]any{
				"attributes.evt.name":    "Dashboard",
				"attributes.evt.outcome": "success",
				"attributes.usr.email":   "a@b.com",
			},
		},
		{
			name: "scalars stored as-is",
			in:   map[string]any{"action": "deleted", "count": 3, "ok": true},
			want: map[string]any{
				"attributes.action": "deleted",
				"attributes.count":  3,
				"attributes.ok":     true,
			},
		},
		{
			name: "slices preserved intact",
			in:   map[string]any{"roles": []any{"admin", "editor"}},
			want: map[string]any{"attributes.roles": []any{"admin", "editor"}},
		},
		{
			name: "empty map produces nothing",
			in:   map[string]any{},
			want: map[string]any{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			props := map[string]any{}
			flattenInto(props, "attributes", tc.in)
			if !reflect.DeepEqual(props, tc.want) {
				t.Errorf("got %#v, want %#v", props, tc.want)
			}
		})
	}
}
