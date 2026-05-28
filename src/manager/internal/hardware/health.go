// Package hardware implements automatic discovery and detection of USB Thread radio coordinator hardware.
//
//nolint:gosec,noctx,nestif // G301/G306 permissions are system requirements, nestif has direct container device validation
package hardware

import (
	"log"
	"os"
	"os/exec"
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

func tryConfigureSysctl(path, value string) {
	if checkSysctl(path, value) {
		log.Printf("[Hardware] Host %s is already set to %s\n", path, value)
		return
	}
	log.Printf("[Hardware] Attempting to auto-configure %s to %s...\n", path, value)
	if err := writeSysctl(path, value); err != nil {
		log.Printf("[Hardware] Warning: failed to auto-configure %s to %s: %v\n", path, value, err)
	} else {
		log.Printf("[Hardware] Successfully auto-configured %s to %s\n", path, value)
	}
}

func tryConfigureTunDevice() {
	if _, err := os.Stat("/dev/net/tun"); os.IsNotExist(err) {
		log.Println("[Hardware] Virtual TUN device node /dev/net/tun is missing. Attempting auto-creation...")
		if err := os.MkdirAll("/dev/net", 0755); err != nil {
			log.Printf("[Hardware] Warning: failed to create directory /dev/net: %v\n", err)
		} else {
			cmd := exec.Command("mknod", "/dev/net/tun", "c", "10", "200")
			if err := cmd.Run(); err != nil {
				log.Printf("[Hardware] Warning: failed to create /dev/net/tun via mknod: %v\n", err)
			} else {
				log.Println("[Hardware] Successfully created virtual TUN device node /dev/net/tun")
			}
		}
	} else {
		log.Println("[Hardware] Virtual TUN device node /dev/net/tun exists and is accessible")
	}
}

// SelfHealHost attempts to configure the host for optimal routing and TUN readiness.
func SelfHealHost() {
	log.Println("[Hardware] Running self-healing steps for host networking configuration...")

	// 1. Try to enable IPv6 forwarding if disabled
	tryConfigureSysctl("/proc/sys/net/ipv6/conf/all/forwarding", "1")
	tryConfigureSysctl("/proc/sys/net/ipv6/conf/default/forwarding", "1")

	// 2. Try to set accept_ra to 2 if not set
	tryConfigureSysctl("/proc/sys/net/ipv6/conf/all/accept_ra", "2")
	tryConfigureSysctl("/proc/sys/net/ipv6/conf/default/accept_ra", "2")

	// 3. Try to auto-create /dev/net/tun if missing
	tryConfigureTunDevice()
}

// AuditHost checks host-level routing configurations and virtual interface capabilities.
func AuditHost() HostAudit {
	SelfHealHost()
	var audit HostAudit
	audit.IPv6ForwardingAll = checkSysctl("/proc/sys/net/ipv6/conf/all/forwarding", "1")
	audit.IPv6ForwardingDefault = checkSysctl("/proc/sys/net/ipv6/conf/default/forwarding", "1")

	if !audit.IPv6ForwardingAll || !audit.IPv6ForwardingDefault {
		audit.Warnings = append(audit.Warnings, "\n"+
			"================================================================================\n"+
			"[DIAGNOSTIC REPORT] HOST IPV6 FORWARDING DISABLED\n"+
			"================================================================================\n"+
			"Issue: Host IPv6 packet forwarding is disabled.\n"+
			"Root Cause: The host kernel is not routing IPv6 packets, which prevents Thread border routing.\n"+
			"How to Fix: Run the following command on your host machine:\n"+
			"  sysctl -w net.ipv6.conf.all.forwarding=1 net.ipv6.conf.default.forwarding=1\n"+
			"================================================================================")
	}

	audit.IPv6AcceptRaAll = checkSysctl("/proc/sys/net/ipv6/conf/all/accept_ra", "2")
	audit.IPv6AcceptRaDefault = checkSysctl("/proc/sys/net/ipv6/conf/default/accept_ra", "2")

	if !audit.IPv6AcceptRaAll || !audit.IPv6AcceptRaDefault {
		audit.Warnings = append(audit.Warnings, "\n"+
			"================================================================================\n"+
			"[DIAGNOSTIC REPORT] HOST IPV6 ACCEPT_RA NOT SET TO 2\n"+
			"================================================================================\n"+
			"Issue: Host Accept Router Advertisements (accept_ra) is not configured to 2.\n"+
			"Root Cause: The host is not configured to accept Router Advertisements when forwarding is enabled.\n"+
			"How to Fix: Run the following command on your host machine:\n"+
			"  sysctl -w net.ipv6.conf.all.accept_ra=2 net.ipv6.conf.default.accept_ra=2\n"+
			"================================================================================")
	}

	_, err := os.Stat("/dev/net/tun")
	audit.TunDeviceExists = (err == nil)
	if !audit.TunDeviceExists {
		audit.Warnings = append(audit.Warnings, "\n"+
			"================================================================================\n"+
			"[DIAGNOSTIC REPORT] MISSING VIRTUAL TUN DEVICE\n"+
			"================================================================================\n"+
			"Issue: Virtual TUN adapter device /dev/net/tun is missing in the container.\n"+
			"Root Cause: The container is running without proper Linux capabilities or device permissions.\n"+
			"How to Fix: Start the container with NET_ADMIN capability and mount the TUN device:\n"+
			"  Docker Run:   --cap-add=NET_ADMIN --device /dev/net/tun\n"+
			"  Compose file:\n"+
			"    cap_add:\n"+
			"      - NET_ADMIN\n"+
			"    devices:\n"+
			"      - /dev/net/tun:/dev/net/tun\n"+
			"================================================================================")
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

func writeSysctl(path, value string) error {
	return os.WriteFile(path, []byte(value+"\n"), 0644)
}
