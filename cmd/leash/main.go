package main

import (
	"github.com/mbaitelman/leash/internal/cli"

	// Blank imports trigger init() registrations for all resource, filter, and action types.
	_ "github.com/mbaitelman/leash/internal/action"
	_ "github.com/mbaitelman/leash/internal/filter"
	_ "github.com/mbaitelman/leash/internal/resource"
)

func main() {
	cli.Execute()
}
