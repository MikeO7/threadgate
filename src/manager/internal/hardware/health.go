package hardware

import (
	"os"
	"strings"
)

// HostAudit captures host-level routing and TUN readiness for border routing.
type HostAudit struct {
	IPv6ForwardingAll     bool     `json:"ipv6ForwardingAll"`
	IPv6ForwardingDefault bool     `json:"ipv6ForwardingDefault"`
	IPv6AcceptRaAll       bool     `json:"ipv6AcceptRaAll"`
	IPv6AcceptRaDefault   bool     `json:"ipv6AcceptRaDefault"`
	TunDeviceExists       bool     `json:"tunDeviceExists"`
	Warnings              []string `json:"warnings"`
}

// AuditHost checks host-level routing configurations and virtual interface capabilities.
func AuditHost() HostAudit {
	var audit HostAudit

	audit.IPv6ForwardingAll = checkSysctl("/proc/sys/net/ipv6/conf/all/forwarding", "1")
	audit.IPv6ForwardingDefault = checkSysctl("/proc/sys/net/ipv6/conf/default/forwarding", "1")

	if !audit.IPv6ForwardingAll || !audit.IPv6ForwardingDefault {
		audit.Warnings = append(audit.Warnings, "Host IPv6 packet forwarding is not enabled. Run: sysctl -w net.ipv6.conf.all.forwarding=1 net.ipv6.conf.default.forwarding=1")
	}

	audit.IPv6AcceptRaAll = checkSysctl("/proc/sys/net/ipv6/conf/all/accept_ra", "2")
	audit.IPv6AcceptRaDefault = checkSysctl("/proc/sys/net/ipv6/conf/default/accept_ra", "2")

	if !audit.IPv6AcceptRaAll || !audit.IPv6AcceptRaDefault {
		audit.Warnings = append(audit.Warnings, "Host Accept Router Advertisements (accept_ra) is not set to 2. Run: sysctl -w net.ipv6.conf.all.accept_ra=2 net.ipv6.conf.default.accept_ra=2")
	}

	_, err := os.Stat("/dev/net/tun")
	audit.TunDeviceExists = (err == nil)
	if !audit.TunDeviceExists {
		audit.Warnings = append(audit.Warnings, "Virtual TUN adapter device /dev/net/tun is missing in the container. Make sure --cap-add=NET_ADMIN or --privileged is enabled and devices include /dev/net/tun.")
	}

	return audit
}

func checkSysctl(path, expected string) bool {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is a fixed sysctl location from AuditHost
	if err != nil {
		return false
	}
	val := strings.TrimSpace(string(data))
	return val == expected
}
