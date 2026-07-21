package resource

// flattenInto writes value v into props under key, recursing into nested
// maps with dot-joined keys. Scalars and slices are stored as-is; slices of
// scalars work with the value filter's contains/in operators.
func flattenInto(props map[string]any, key string, v any) {
	if m, ok := v.(map[string]any); ok {
		for k, child := range m {
			flattenInto(props, key+"."+k, child)
		}
		return
	}
	props[key] = v
}
