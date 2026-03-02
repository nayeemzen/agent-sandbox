package templates

import "testing"

func TestResolveExplicit(t *testing.T) {
	got, err := Resolve(ResolveInput{
		Explicit: "t1",
		Default:  "t2",
		Names:    []string{"t1", "t2"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "t1" {
		t.Fatalf("expected t1, got %q", got)
	}
}

func TestResolveExplicitMissing(t *testing.T) {
	_, err := Resolve(ResolveInput{
		Explicit: "t1",
		Default:  "t2",
		Names:    []string{"t2"},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveDefault(t *testing.T) {
	got, err := Resolve(ResolveInput{
		Default: "t2",
		Names:   []string{"t1", "t2"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "t2" {
		t.Fatalf("expected t2, got %q", got)
	}
}

func TestResolveSingleTemplate(t *testing.T) {
	got, err := Resolve(ResolveInput{
		Names: []string{"only"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "only" {
		t.Fatalf("expected only, got %q", got)
	}
}

func TestResolveNoTemplates(t *testing.T) {
	_, err := Resolve(ResolveInput{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveMultipleNoDefault(t *testing.T) {
	_, err := Resolve(ResolveInput{Names: []string{"a", "b"}})
	if err == nil {
		t.Fatalf("expected error")
	}
}
