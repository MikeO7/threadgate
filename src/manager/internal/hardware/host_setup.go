package hardware

// SetupStep is one actionable item in the production setup checklist.
type SetupStep struct {
	ID          string
	Title       string
	Description string
	Done        bool
	Commands    []string
	Note        string
}

const (
	hostStepIPv6Forwarding = "host-ipv6-forwarding"
	hostStepIPv6AcceptRA   = "host-ipv6-accept-ra"
	hostStepTunDevice      = "host-tun-device"
)

// HostNetworkingSteps returns host-level setup steps derived from a fresh audit.
func HostNetworkingSteps(audit HostAudit) []SetupStep {
	var steps []SetupStep
	if !audit.IPv6ForwardingAll || !audit.IPv6ForwardingDefault {
		steps = append(steps, SetupStep{
			ID:          hostStepIPv6Forwarding,
			Title:       "Enable IPv6 forwarding on the Linux host",
			Description: "Thread border routing requires the host kernel to forward IPv6 packets. Run these commands on the machine running Docker (SSH into the server — not inside the container).",
			Done:        false,
			Commands: []string{
				"sudo sysctl -w net.ipv6.conf.all.forwarding=1",
				"sudo sysctl -w net.ipv6.conf.default.forwarding=1",
			},
			Note: "To persist after reboot, save to /etc/sysctl.d/99-threadgate.conf and run: sudo sysctl --system",
		})
	}
	if !audit.IPv6AcceptRaAll || !audit.IPv6AcceptRaDefault {
		steps = append(steps, SetupStep{
			ID:          hostStepIPv6AcceptRA,
			Title:       "Allow router advertisements while forwarding",
			Description: "When IPv6 forwarding is enabled, the host must still accept router advertisements (accept_ra=2) or upstream connectivity can break.",
			Done:        false,
			Commands: []string{
				"sudo sysctl -w net.ipv6.conf.all.accept_ra=2",
				"sudo sysctl -w net.ipv6.conf.default.accept_ra=2",
			},
			Note: "Add the same lines to /etc/sysctl.d/99-threadgate.conf for persistence, then: sudo sysctl --system",
		})
	}
	if !audit.TunDeviceExists {
		steps = append(steps, SetupStep{
			ID:          hostStepTunDevice,
			Title:       "Grant the container access to /dev/net/tun",
			Description: "The border router needs a TUN device. Ensure Docker Compose includes NET_ADMIN and the /dev/net/tun device mapping.",
			Done:        false,
			Commands:    []string{},
			Note:        "In docker-compose.yml: cap_add: [NET_ADMIN, SYS_ADMIN] and devices: [/dev/net/tun:/dev/net/tun]",
		})
	}
	return steps
}

// PersistSysctlSnippet returns lines to add to /etc/sysctl.d/ for missing host settings.
func PersistSysctlSnippet(audit HostAudit) []string {
	var lines []string
	if !audit.IPv6ForwardingAll || !audit.IPv6ForwardingDefault {
		lines = append(lines,
			"net.ipv6.conf.all.forwarding=1",
			"net.ipv6.conf.default.forwarding=1",
		)
	}
	if !audit.IPv6AcceptRaAll || !audit.IPv6AcceptRaDefault {
		lines = append(lines,
			"net.ipv6.conf.all.accept_ra=2",
			"net.ipv6.conf.default.accept_ra=2",
		)
	}
	return lines
}
