package incus

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	incusclient "github.com/lxc/incus/v6/client"
)

const PortDevicePrefix = "sandbox-port-"

type PublishedPort struct {
	Device         string
	Protocol       string
	ListenAddress  string
	HostPort       int
	ConnectAddress string
	GuestPort      int
}

type PublishPortInput struct {
	Protocol       string // tcp or udp
	ListenAddress  string
	HostPort       int
	ConnectAddress string
	GuestPort      int
}

func ListPublishedPorts(s incusclient.InstanceServer, sandbox string) ([]PublishedPort, error) {
	inst, _, err := s.GetInstance(sandbox)
	if err != nil {
		return nil, err
	}

	out := []PublishedPort{}
	for name, dev := range inst.Devices {
		if !strings.HasPrefix(name, PortDevicePrefix) {
			continue
		}
		if strings.TrimSpace(dev["type"]) != "proxy" {
			continue
		}

		listenProto, listenAddr, listenPort, err := parseProxyEndpoint(dev["listen"])
		if err != nil {
			continue
		}
		connectProto, connectAddr, connectPort, err := parseProxyEndpoint(dev["connect"])
		if err != nil {
			continue
		}
		if listenProto != connectProto {
			continue
		}
		out = append(out, PublishedPort{
			Device:         name,
			Protocol:       listenProto,
			ListenAddress:  listenAddr,
			HostPort:       listenPort,
			ConnectAddress: connectAddr,
			GuestPort:      connectPort,
		})
	}
	return out, nil
}

func PublishPort(ctx context.Context, s incusclient.InstanceServer, sandbox string, in PublishPortInput) (PublishedPort, bool, error) {
	proto := strings.ToLower(strings.TrimSpace(in.Protocol))
	if proto == "" {
		proto = "tcp"
	}
	if proto != "tcp" && proto != "udp" {
		return PublishedPort{}, false, fmt.Errorf("unsupported protocol %q", in.Protocol)
	}

	listenAddr := strings.TrimSpace(in.ListenAddress)
	if listenAddr == "" {
		listenAddr = "0.0.0.0"
	}
	connectAddr := strings.TrimSpace(in.ConnectAddress)
	if connectAddr == "" {
		connectAddr = "127.0.0.1"
	}
	if in.HostPort <= 0 || in.HostPort > 65535 {
		return PublishedPort{}, false, fmt.Errorf("invalid host port %d", in.HostPort)
	}
	if in.GuestPort <= 0 || in.GuestPort > 65535 {
		return PublishedPort{}, false, fmt.Errorf("invalid guest port %d", in.GuestPort)
	}

	inst, etag, err := s.GetInstance(sandbox)
	if err != nil {
		return PublishedPort{}, false, err
	}
	if strings.TrimSpace(inst.Config["user.sandbox.managed"]) != "true" {
		return PublishedPort{}, false, fmt.Errorf("%q is not a sandbox-managed instance", sandbox)
	}

	deviceName := portDeviceName(proto, in.HostPort, in.GuestPort)
	listen := fmt.Sprintf("%s:%s:%d", proto, listenAddr, in.HostPort)
	connect := fmt.Sprintf("%s:%s:%d", proto, connectAddr, in.GuestPort)

	if inst.Devices == nil {
		inst.Devices = map[string]map[string]string{}
	}

	// Detect conflicts: another managed port device already using the same listen address/port.
	for devName, dev := range inst.Devices {
		if strings.TrimSpace(dev["type"]) != "proxy" {
			continue
		}

		lp, la, lport, err := parseProxyEndpoint(dev["listen"])
		if err != nil {
			continue
		}
		if lp == proto && lport == in.HostPort && la == listenAddr {
			if devName == deviceName {
				// Exact same device name exists; check idempotency.
				if strings.TrimSpace(dev["connect"]) == connect {
					return PublishedPort{
						Device:         deviceName,
						Protocol:       proto,
						ListenAddress:  listenAddr,
						HostPort:       in.HostPort,
						ConnectAddress: connectAddr,
						GuestPort:      in.GuestPort,
					}, false, nil
				}
				return PublishedPort{}, false, fmt.Errorf("port %s:%s:%d already published with a different target", proto, listenAddr, in.HostPort)
			}

			return PublishedPort{}, false, fmt.Errorf("port %s:%s:%d already published by device %q", proto, listenAddr, in.HostPort, devName)
		}
	}

	put := inst.Writable()
	if put.Devices == nil {
		put.Devices = map[string]map[string]string{}
	}
	put.Devices[deviceName] = map[string]string{
		"type":    "proxy",
		"listen":  listen,
		"connect": connect,
	}

	op, err := s.UpdateInstance(sandbox, put, etag)
	if err != nil {
		return PublishedPort{}, false, err
	}
	if err := op.WaitContext(ctx); err != nil {
		return PublishedPort{}, false, err
	}

	return PublishedPort{
		Device:         deviceName,
		Protocol:       proto,
		ListenAddress:  listenAddr,
		HostPort:       in.HostPort,
		ConnectAddress: connectAddr,
		GuestPort:      in.GuestPort,
	}, true, nil
}

func UnpublishPort(ctx context.Context, s incusclient.InstanceServer, sandbox string, device string) error {
	device = strings.TrimSpace(device)
	if device == "" {
		return fmt.Errorf("device name is required")
	}

	inst, etag, err := s.GetInstance(sandbox)
	if err != nil {
		return err
	}
	if strings.TrimSpace(inst.Config["user.sandbox.managed"]) != "true" {
		return fmt.Errorf("%q is not a sandbox-managed instance", sandbox)
	}

	if inst.Devices == nil {
		return nil
	}
	if _, ok := inst.Devices[device]; !ok {
		return nil
	}

	put := inst.Writable()
	if put.Devices == nil {
		put.Devices = map[string]map[string]string{}
	}
	delete(put.Devices, device)

	op, err := s.UpdateInstance(sandbox, put, etag)
	if err != nil {
		return err
	}
	return op.WaitContext(ctx)
}

func portDeviceName(proto string, hostPort int, guestPort int) string {
	proto = strings.ToLower(strings.TrimSpace(proto))
	if proto == "" {
		proto = "tcp"
	}
	return fmt.Sprintf("%s%s-%d-%d", PortDevicePrefix, proto, hostPort, guestPort)
}

func parseProxyEndpoint(v string) (proto string, host string, port int, err error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", "", 0, fmt.Errorf("empty endpoint")
	}

	i := strings.IndexByte(v, ':')
	if i < 0 {
		return "", "", 0, fmt.Errorf("invalid endpoint %q", v)
	}
	proto = strings.ToLower(strings.TrimSpace(v[:i]))
	rest := strings.TrimSpace(v[i+1:])
	if proto == "" || rest == "" {
		return "", "", 0, fmt.Errorf("invalid endpoint %q", v)
	}

	host, port, err = splitHostPortLoose(rest)
	if err != nil {
		return "", "", 0, err
	}
	return proto, host, port, nil
}

func splitHostPortLoose(rest string) (host string, port int, err error) {
	// Prefer net.SplitHostPort for bracketed IPv6 and normal host:port forms.
	if strings.HasPrefix(rest, "[") {
		h, p, err := net.SplitHostPort(rest)
		if err != nil {
			return "", 0, err
		}
		port, err = strconv.Atoi(p)
		if err != nil {
			return "", 0, err
		}
		return h, port, nil
	}

	// net.SplitHostPort requires brackets for IPv6; this fallback handles our own
	// generated forms like 0.0.0.0:1234.
	idx := strings.LastIndexByte(rest, ':')
	if idx < 0 {
		return "", 0, fmt.Errorf("invalid endpoint %q", rest)
	}
	host = strings.TrimSpace(rest[:idx])
	p := strings.TrimSpace(rest[idx+1:])
	if host == "" || p == "" {
		return "", 0, fmt.Errorf("invalid endpoint %q", rest)
	}
	port, err = strconv.Atoi(p)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}
