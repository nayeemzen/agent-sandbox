package incus

import (
	"context"
	"fmt"
	"os/exec"

	incusclient "github.com/lxc/incus/v6/client"

	"github.com/nayeemzen/agent-sandbox/internal/doctor"
)

type DoctorOptions struct {
	// LocalMode controls whether we run checks that only make sense when the
	// Incus daemon is local to the machine where sandbox is running.
	LocalMode bool
}

func RunDoctor(ctx context.Context, s incusclient.InstanceServer, opts DoctorOptions) []doctor.CheckResult {
	results := []doctor.CheckResult{}

	server, _, err := s.GetServer()
	if err != nil {
		return []doctor.CheckResult{
			{
				ID:          "incus.api",
				Status:      doctor.Fail,
				Summary:     "connected but failed to query server info",
				Details:     err.Error(),
				Remediation: "Verify Incus daemon health and permissions.",
			},
		}
	}

	results = append(results, doctor.CheckResult{
		ID:      "incus.api",
		Status:  doctor.Pass,
		Summary: "reachable",
		Details: fmt.Sprintf("server=%s api_extensions=%d", server.Environment.ServerName, len(server.APIExtensions)),
	})

	pools, err := s.GetStoragePools()
	if err != nil {
		results = append(results, doctor.CheckResult{
			ID:          "incus.storage",
			Status:      doctor.Fail,
			Summary:     "failed to list storage pools",
			Details:     err.Error(),
			Remediation: "Create or select a storage pool (incus admin init).",
		})
	} else if len(pools) == 0 {
		results = append(results, doctor.CheckResult{
			ID:          "incus.storage",
			Status:      doctor.Fail,
			Summary:     "no storage pools found",
			Remediation: "Run incus admin init to create a default storage pool.",
		})
	} else {
		results = append(results, doctor.CheckResult{
			ID:      "incus.storage",
			Status:  doctor.Pass,
			Summary: "storage pools present",
			Details: fmt.Sprintf("count=%d", len(pools)),
		})
	}

	networks, err := s.GetNetworks()
	if err != nil {
		results = append(results, doctor.CheckResult{
			ID:          "incus.network",
			Status:      doctor.Fail,
			Summary:     "failed to list networks",
			Details:     err.Error(),
			Remediation: "Ensure Incus has a managed bridge network (incus admin init).",
		})
	} else {
		managedBridges := 0
		for _, n := range networks {
			if n.Managed && n.Type == "bridge" {
				managedBridges++
			}
		}

		if managedBridges == 0 {
			results = append(results, doctor.CheckResult{
				ID:          "incus.network",
				Status:      doctor.Fail,
				Summary:     "no managed bridge networks found",
				Remediation: "Create a managed bridge network (incus admin init).",
			})
		} else {
			results = append(results, doctor.CheckResult{
				ID:      "incus.network",
				Status:  doctor.Pass,
				Summary: "managed bridge network present",
				Details: fmt.Sprintf("count=%d", managedBridges),
			})
		}
	}

	if metrics, err := s.GetMetrics(); err != nil {
		results = append(results, doctor.CheckResult{
			ID:          "incus.metrics",
			Status:      doctor.Warn,
			Summary:     "metrics endpoint unavailable",
			Details:     err.Error(),
			Remediation: "Enable metrics in Incus if you want sandbox monitor to work.",
		})
	} else if metrics == "" {
		results = append(results, doctor.CheckResult{
			ID:          "incus.metrics",
			Status:      doctor.Warn,
			Summary:     "metrics endpoint returned empty output",
			Remediation: "Verify metrics are enabled and the daemon is returning OpenMetrics text.",
		})
	} else {
		results = append(results, doctor.CheckResult{
			ID:      "incus.metrics",
			Status:  doctor.Pass,
			Summary: "metrics endpoint reachable",
		})
	}

	if opts.LocalMode {
		if _, err := exec.LookPath("skopeo"); err != nil {
			results = append(results, doctor.CheckResult{
				ID:          "oci.skopeo",
				Status:      doctor.Warn,
				Summary:     "skopeo not found on host",
				Details:     err.Error(),
				Remediation: "Install skopeo if you plan to create templates from OCI images.",
			})
		} else {
			results = append(results, doctor.CheckResult{
				ID:      "oci.skopeo",
				Status:  doctor.Pass,
				Summary: "skopeo found on host",
			})
		}
	}

	return results
}
