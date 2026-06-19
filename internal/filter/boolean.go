package filter

import "github.com/mbaitelman/leash/internal/resource"

type andFilter struct{ children []Filter }
type orFilter struct{ children []Filter }
type notFilter struct{ child Filter }

func (f *andFilter) Match(r resource.Resource) (bool, error) {
	for _, child := range f.children {
		ok, err := child.Match(r)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func (f *orFilter) Match(r resource.Resource) (bool, error) {
	for _, child := range f.children {
		ok, err := child.Match(r)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func (f *notFilter) Match(r resource.Resource) (bool, error) {
	ok, err := f.child.Match(r)
	if err != nil {
		return false, err
	}
	return !ok, nil
}
