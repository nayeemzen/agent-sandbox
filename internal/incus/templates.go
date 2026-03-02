package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	incusclient "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

const TemplateAliasPrefix = "sandbox/"

type Template struct {
	Name        string
	Alias       string
	Fingerprint string
	Source      string
	UploadedAt  time.Time
}

func ListTemplates(s incusclient.InstanceServer) ([]Template, error) {
	images, err := s.GetImages()
	if err != nil {
		return nil, err
	}

	out := []Template{}
	for _, img := range images {
		for _, a := range img.Aliases {
			if !strings.HasPrefix(a.Name, TemplateAliasPrefix) {
				continue
			}

			name := strings.TrimPrefix(a.Name, TemplateAliasPrefix)
			out = append(out, Template{
				Name:        name,
				Alias:       a.Name,
				Fingerprint: img.Fingerprint,
				Source:      img.Properties["user.sandbox.source"],
				UploadedAt:  img.UploadedAt,
			})
		}
	}

	return out, nil
}

func TemplateAlias(name string) string {
	return TemplateAliasPrefix + name
}

func TemplateExists(s incusclient.InstanceServer, name string) (bool, error) {
	alias := TemplateAlias(name)
	_, _, err := s.GetImageAlias(alias)
	if err == nil {
		return true, nil
	}

	// Incus client doesn't expose a typed "not found" error; best-effort.
	if strings.Contains(err.Error(), "not found") {
		return false, nil
	}

	return false, err
}

type CreateTemplateOptions struct {
	ProvisionVersion string
}

func CreateTemplate(ctx context.Context, s incusclient.InstanceServer, name string, source string, opts CreateTemplateOptions) (Template, error) {
	alias := TemplateAlias(name)

	if _, _, err := s.GetImageAlias(alias); err == nil {
		return Template{}, fmt.Errorf("template %q already exists", name)
	}

	seed := fmt.Sprintf("sandbox-seed-%s-%s", name, randSuffix(6))
	instanceSource, err := parseInstanceSource(source)
	if err != nil {
		return Template{}, err
	}

	createReq := api.InstancesPost{
		Name:  seed,
		Type:  api.InstanceTypeContainer,
		Start: true,
		Source: api.InstanceSource{
			Type:        "image",
			Alias:       instanceSource.Alias,
			Protocol:    instanceSource.Protocol,
			Server:      instanceSource.Server,
			Fingerprint: instanceSource.Fingerprint,
		},
	}

	op, err := s.CreateInstance(createReq)
	if err != nil {
		return Template{}, err
	}
	if err := op.WaitContext(ctx); err != nil {
		return Template{}, err
	}

	// Best-effort cleanup if any later step fails.
	cleanupSeed := func() {
		if op, err := s.DeleteInstance(seed); err == nil {
			_ = op.WaitContext(ctx)
		}
	}

	// Minimal provisioning contract for v1 templates.
	if err := execInInstance(ctx, s, seed, []string{"sh", "-lc", "mkdir -p /var/log/sandbox /run/sandbox && chmod 0755 /var/log/sandbox /run/sandbox"}); err != nil {
		cleanupSeed()
		return Template{}, err
	}

	snap := "template"
	if op, err := s.CreateInstanceSnapshot(seed, api.InstanceSnapshotsPost{Name: snap, Stateful: false}); err != nil {
		cleanupSeed()
		return Template{}, err
	} else if err := op.WaitContext(ctx); err != nil {
		cleanupSeed()
		return Template{}, err
	}

	props := map[string]string{
		"user.sandbox.template": name,
		"user.sandbox.source":   source,
	}
	if opts.ProvisionVersion != "" {
		props["user.sandbox.provision_version"] = opts.ProvisionVersion
	}

	imgReq := api.ImagesPost{
		ImagePut: api.ImagePut{
			Properties: props,
		},
		Source: &api.ImagesPostSource{
			Type: "snapshot",
			Name: seed + "/" + snap,
		},
		Aliases: []api.ImageAlias{
			{Name: alias, Description: "sandbox template"},
		},
	}

	imgOp, err := s.CreateImage(imgReq, nil)
	if err != nil {
		cleanupSeed()
		return Template{}, err
	}
	if err := imgOp.WaitContext(ctx); err != nil {
		cleanupSeed()
		return Template{}, err
	}

	cleanupSeed()

	// Retrieve alias to get the fingerprint.
	aliasEntry, _, err := s.GetImageAlias(alias)
	if err != nil {
		return Template{}, nil
	}

	return Template{
		Name:        name,
		Alias:       alias,
		Fingerprint: aliasEntry.Target,
		Source:      source,
	}, nil
}

func DeleteTemplate(ctx context.Context, s incusclient.InstanceServer, name string, force bool) error {
	// Block removal if in use (unless forced).
	if !force {
		inUse, err := templateInUse(s, name)
		if err != nil {
			return err
		}
		if inUse {
			return fmt.Errorf("template %q is in use by existing sandboxes (use --force to override)", name)
		}
	}

	alias := TemplateAlias(name)
	entry, _, err := s.GetImageAlias(alias)
	if err != nil {
		// Treat not-found as success.
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return err
	}

	if err := s.DeleteImageAlias(alias); err != nil && !strings.Contains(err.Error(), "not found") {
		return err
	}

	if op, err := s.DeleteImage(entry.Target); err == nil {
		_ = op.WaitContext(ctx)
	}

	return nil
}

func templateInUse(s incusclient.InstanceServer, name string) (bool, error) {
	instances, err := s.GetInstancesFull(api.InstanceTypeContainer)
	if err != nil {
		return false, err
	}

	for _, inst := range instances {
		if inst.Config["user.sandbox.template"] == name && inst.Config["user.sandbox.managed"] == "true" {
			return true, nil
		}
	}

	return false, nil
}

type instanceSourceRef struct {
	Alias       string
	Protocol    string
	Server      string
	Fingerprint string
}

func parseInstanceSource(ref string) (instanceSourceRef, error) {
	// v1 supported forms:
	// - images:<alias>  (simplestreams from images.linuxcontainers.org)
	// - local:<alias>   (local Incus image alias)
	if strings.HasPrefix(ref, "images:") {
		alias := strings.TrimPrefix(ref, "images:")
		if alias == "" {
			return instanceSourceRef{}, fmt.Errorf("invalid source %q", ref)
		}

		return instanceSourceRef{
			Alias:    alias,
			Protocol: "simplestreams",
			Server:   "https://images.linuxcontainers.org",
		}, nil
	}

	if strings.HasPrefix(ref, "local:") {
		alias := strings.TrimPrefix(ref, "local:")
		if alias == "" {
			return instanceSourceRef{}, fmt.Errorf("invalid source %q", ref)
		}

		return instanceSourceRef{
			Alias: alias,
		}, nil
	}

	return instanceSourceRef{}, fmt.Errorf("unsupported source %q (supported: images:<alias>, local:<alias>)", ref)
}

func randSuffix(nbytes int) string {
	b := make([]byte, nbytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
