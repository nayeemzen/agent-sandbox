package templates

import (
	"errors"
	"fmt"
	"slices"
)

var (
	ErrNoTemplates       = errors.New("no templates available")
	ErrMultipleTemplates = errors.New("multiple templates available")
	ErrTemplateNotFound  = errors.New("template not found")
)

type ResolveInput struct {
	Explicit string
	Default  string
	Names    []string
}

func Resolve(in ResolveInput) (string, error) {
	if in.Explicit != "" {
		if !slices.Contains(in.Names, in.Explicit) {
			return "", fmt.Errorf("%w: %q", ErrTemplateNotFound, in.Explicit)
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
		return "", ErrNoTemplates
	}

	return "", ErrMultipleTemplates
}
