package templates

import (
	"fmt"
	"slices"
)

type ResolveInput struct {
	Explicit string
	Default  string
	Names    []string
}

func Resolve(in ResolveInput) (string, error) {
	if in.Explicit != "" {
		if !slices.Contains(in.Names, in.Explicit) {
			return "", fmt.Errorf("template %q not found", in.Explicit)
		}
		return in.Explicit, nil
	}

	if in.Default != "" && slices.Contains(in.Names, in.Default) {
		return in.Default, nil
	}

	if len(in.Names) == 1 {
		return in.Names[0], nil
	}

	if len(in.Names) == 0 {
		return "", fmt.Errorf("no templates available")
	}

	return "", fmt.Errorf("multiple templates available; set a default or specify --template")
}
