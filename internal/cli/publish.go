package cli

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	incusclient "github.com/lxc/incus/v6/client"
	"github.com/spf13/cobra"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
)

type publishPortJSON struct {
	Sandbox        string `json:"sandbox"`
	Device         string `json:"device"`
	Protocol       string `json:"protocol"`
	ListenAddress  string `json:"listen_address"`
	HostPort       int    `json:"host_port"`
	ConnectAddress string `json:"connect_address"`
	GuestPort      int    `json:"guest_port"`
	Created        bool   `json:"created"`
}

func newPublishCmd(opts *GlobalOptions) *cobra.Command {
	var listenAddr string
	var connectAddr string
	var proto string

	cmd := &cobra.Command{
		Use:           "publish [sandbox] [port...]",
		Aliases:       []string{"expose"},
		Short:         "Publish sandbox ports on the host (Docker-style -p)",
		Args:          cobra.MaximumNArgs(32),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			s, err := connectIncus(ctx, opts)
			if err != nil {
				return err
			}

			var sandbox string
			var portSpecs []string
			switch len(args) {
			case 0:
				sandbox, err = chooseSandboxArg(s, nil, "sandbox", "Select sandbox to publish ports for", nil)
				if err != nil {
					return err
				}
				spec, err := promptRequiredValue("port", "Port mapping (HOST:GUEST, GUEST, or :GUEST)", "8080:80", "Provide a port mapping like 8080:80 or :80")
				if err != nil {
					return err
				}
				portSpecs = []string{spec}
			case 1:
				sandbox = strings.TrimSpace(args[0])
				spec, err := promptRequiredValue("port", "Port mapping (HOST:GUEST, GUEST, or :GUEST)", "8080:80", "Provide a port mapping like 8080:80 or :80")
				if err != nil {
					return err
				}
				portSpecs = []string{spec}
			default:
				sandbox = strings.TrimSpace(args[0])
				portSpecs = args[1:]
			}

			if sandbox == "" {
				return newCLIError("missing required argument: sandbox", "Usage: sandbox publish <sandbox> <port...>")
			}
			if len(portSpecs) == 0 {
				return newCLIError("missing required argument: port", "Usage: sandbox publish <sandbox> <port...>")
			}

			results := []publishPortJSON{}
			for _, spec := range portSpecs {
				spec = strings.TrimSpace(spec)
				if spec == "" {
					continue
				}

				mapping, err := parsePortSpec(spec)
				if err != nil {
					return err
				}

				hostPort := mapping.HostPort
				guestPort := mapping.GuestPort

				var published incus.PublishedPort
				var created bool
				err = publishWithRetries(ctx, s, sandbox, listenAddr, connectAddr, proto, hostPort, guestPort, mapping.RandomHostPort, func(pp incus.PublishedPort, c bool) {
					published = pp
					created = c
				})
				if err != nil {
					return err
				}

				results = append(results, publishPortJSON{
					Sandbox:        sandbox,
					Device:         published.Device,
					Protocol:       published.Protocol,
					ListenAddress:  published.ListenAddress,
					HostPort:       published.HostPort,
					ConnectAddress: published.ConnectAddress,
					GuestPort:      published.GuestPort,
					Created:        created,
				})
			}

			if opts.JSON {
				return writeJSON(cmd.OutOrStdout(), results)
			}

			for _, r := range results {
				action := "published"
				if !r.Created {
					action = "already published"
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s:%s:%d -> %s:%d (device=%s)\n", action, r.Protocol, r.ListenAddress, r.HostPort, r.ConnectAddress, r.GuestPort, r.Device)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&listenAddr, "listen", "0.0.0.0", "Host listen address (0.0.0.0 for LAN+Tailscale, 127.0.0.1 for local-only)")
	cmd.Flags().StringVar(&connectAddr, "connect", "127.0.0.1", "Sandbox connect address (default 127.0.0.1 supports localhost-bound services)")
	cmd.Flags().StringVar(&proto, "proto", "tcp", "Protocol to publish (tcp or udp)")

	return cmd
}

type portSpec struct {
	HostPort       int
	GuestPort      int
	RandomHostPort bool
}

func parsePortSpec(spec string) (portSpec, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return portSpec{}, fmt.Errorf("invalid port mapping %q", spec)
	}

	if strings.Contains(spec, ":") {
		parts := strings.SplitN(spec, ":", 2)
		hostStr := strings.TrimSpace(parts[0])
		guestStr := strings.TrimSpace(parts[1])
		if guestStr == "" {
			return portSpec{}, fmt.Errorf("invalid port mapping %q (missing guest port)", spec)
		}
		guestPort, err := strconv.Atoi(guestStr)
		if err != nil || guestPort <= 0 || guestPort > 65535 {
			return portSpec{}, fmt.Errorf("invalid guest port %q", guestStr)
		}

		if hostStr == "" || hostStr == "0" {
			return portSpec{GuestPort: guestPort, RandomHostPort: true}, nil
		}

		hostPort, err := strconv.Atoi(hostStr)
		if err != nil || hostPort <= 0 || hostPort > 65535 {
			return portSpec{}, fmt.Errorf("invalid host port %q", hostStr)
		}
		return portSpec{HostPort: hostPort, GuestPort: guestPort}, nil
	}

	p, err := strconv.Atoi(spec)
	if err != nil || p <= 0 || p > 65535 {
		return portSpec{}, fmt.Errorf("invalid port %q", spec)
	}
	return portSpec{HostPort: p, GuestPort: p}, nil
}

func publishWithRetries(
	ctx context.Context,
	s incusclient.InstanceServer,
	sandbox string,
	listenAddr string,
	connectAddr string,
	proto string,
	hostPort int,
	guestPort int,
	randomHost bool,
	onSuccess func(incus.PublishedPort, bool),
) error {
	const maxAttempts = 25
	listenAddr = strings.TrimSpace(listenAddr)
	if listenAddr == "" {
		listenAddr = "0.0.0.0"
	}
	connectAddr = strings.TrimSpace(connectAddr)
	if connectAddr == "" {
		connectAddr = "127.0.0.1"
	}
	proto = strings.ToLower(strings.TrimSpace(proto))
	if proto == "" {
		proto = "tcp"
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		chosenHostPort := hostPort
		if randomHost {
			p, err := pickFreeTCPPort(listenAddr)
			if err != nil {
				return err
			}
			chosenHostPort = p
		}

		pp, created, err := incus.PublishPort(ctx, s, sandbox, incus.PublishPortInput{
			Protocol:       proto,
			ListenAddress:  listenAddr,
			HostPort:       chosenHostPort,
			ConnectAddress: connectAddr,
			GuestPort:      guestPort,
		})
		if err == nil {
			onSuccess(pp, created)
			return nil
		}

		if randomHost && isAddressInUse(err) {
			continue
		}
		return err
	}

	return fmt.Errorf("failed to publish a free host port after %d attempts", maxAttempts)
}

func pickFreeTCPPort(listenAddr string) (int, error) {
	// Bind briefly to reserve a free port on the requested interface.
	addr := net.JoinHostPort(listenAddr, "0")
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// Fallback to loopback if the requested address isn't locally bindable.
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return 0, err
		}
	}
	defer ln.Close()

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected listener addr type %T", ln.Addr())
	}
	return tcpAddr.Port, nil
}

func isAddressInUse(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "address already in use") || strings.Contains(msg, "already in use")
}
