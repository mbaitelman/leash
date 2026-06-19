package policy

// PolicyFile is the top-level YAML document structure.
type PolicyFile struct {
	Policies []Policy `yaml:"policies"`
}

// Policy is one named governance policy.
type Policy struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description,omitempty"`
	Resource    string       `yaml:"resource"`
	Filters     []FilterSpec `yaml:"filters,omitempty"`
	Actions     []ActionSpec `yaml:"actions,omitempty"`
}

// FilterSpec is an untyped YAML node. The "type" key selects the filter.
// Boolean meta-filters use "and", "or", or "not" keys instead of "type".
type FilterSpec map[string]any

// ActionSpec is an untyped YAML node. The "type" key selects the action.
type ActionSpec map[string]any
