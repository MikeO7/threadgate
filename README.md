# ThreadGate

> [!WARNING]
> **Under Heavy Development**: This project is undergoing rapid, active development and is not yet suitable for production environments. System stability, configuration options, and APIs are subject to frequent and major breaking changes without notice. Use at your own discretion.

ThreadGate is a modern, self-healing OpenThread Border Router (OTBR) container optimized for Home Assistant and standalone Docker environments. It wraps Google's certified C++ `ot-br-posix` architecture with a robust **Golang orchestrator** (`threadgate`) that replaces standard entrypoint shell scripts, automates hardware USB mapping, and manages self-healing service loops.

---

## Key Features

- **Golang Supervisor Engine**: Acts as PID 1 to gracefully manage child processes (`dbus-daemon`, `avahi-daemon`, `otbr-agent`) and route POSIX OS signals perfectly.
- **Hardware Auto-Discovery**: USB signature scanner automatically detects common smart home coordinators (such as Home Assistant Connect ZBT-1, Sonoff, and Nordic dongles) and binds the exact spinel protocol URLs dynamically.
- **Automatic Self-Healing**: Actively monitors C++ sub-processes. If the `otbr-agent` encounters a critical fault, the manager cleanly recovers and restarts it.
- **Modern REST API & high-Fidelity UI**: Exposes the Home Assistant-expected REST API (Port `8081`) while hosting a premium, responsive HSL dark-themed system diagnostics dashboard.
- **Production-Ready & Minimal**: Built using a multi-stage Dockerfile to eliminate build toolchains from the final runtime image.

---

## Quick Start

### 1. Requirements & Host Configuration
Thread Border Routers require low-level kernel interface routing. Run these commands on your host system:
```bash
# Enable IPv6 Forwarding
sudo sysctl -w net.ipv6.conf.all.forwarding=1
sudo sysctl -w net.ipv6.conf.default.forwarding=1

# Accept Router Advertisements
sudo sysctl -w net.ipv6.conf.all.accept_ra=2
sudo sysctl -w net.ipv6.conf.default.accept_ra=2
```

### 2. Deploy Container
Create a directory on the host and run the container via Docker Compose. The recommended configuration drops full root privileges (`privileged: true`) in favor of specific Linux kernel capabilities:

```yaml
services:
  threadgate:
    container_name: threadgate
    image: ghcr.io/mikeo7/threadgate:latest
    network_mode: host
    restart: unless-stopped

    # Secure Hardening: use specific capabilities instead of full privileged mode
    privileged: false
    cap_add:
      - NET_ADMIN   # Required to configure network interfaces and routing tables
      - SYS_ADMIN   # Required for tun/tap device management (wpan0 creation)
    devices:
      - /dev/net/tun:/dev/net/tun  # Grants access to create virtual interfaces

    volumes:
      - ./data:/data
      - /dev:/dev                   # Required for USB hardware auto-discovery
      - /lib/modules:/lib/modules   # Required for interface loading

    environment:
      - OTBR_RADIO_URL=             # Leave empty for USB auto-discovery
      - OTBR_BAUDRATE=460800        # Optimized default baudrate for Silicon Labs chips
      - OTBR_PORT=8081              # REST API and Dashboard port
      - OTBR_AUTO_DISCOVER=true
```

Simply run:
```bash
docker compose up -d
```

Access the diagnostic dashboard by visiting `http://localhost:8081` in your browser!

---

## Security Posture & Recommendations

### REST API & Dashboard Authentication
To maintain 100% out-of-the-box compatibility with the **Home Assistant OpenThread Border Router integration**, `threadgate`'s REST API and diagnostic web dashboard do **not** enforce built-in username/password authentication. The Home Assistant integration expects a direct, unauthenticated OpenThread REST endpoint to read and configure active network datasets.

### Recommendations for Hardening

1. **Drop Privileged Mode (Highly Recommended)**:
   Avoid running the container with `privileged: true`. Instead, use the `cap_add` block shown in the compose template above (`NET_ADMIN` and `SYS_ADMIN`) alongside mounting `/dev/net/tun`. This grants the container exactly enough kernel access to configure the Thread virtual routing table (`wpan0`) and serial ports without giving it root control over the host OS.

2. **Network Segregation**:
   Run `threadgate` on a secure, trusted local home network or a dedicated **IoT VLAN**. Since the REST API on port `8081` allows fetching and writing active Thread credentials, ensure that untrusted guest devices on your LAN do not have access to this port.

3. **Host-Level Firewall Rules**:
   Since `network_mode: host` is required for Thread router advertisements and mDNS mesh discovery, port `8081` is exposed to all interfaces by default. If you wish to restrict access to only trusted nodes (like a dedicated Home Assistant host), implement host-level firewall rules using `iptables` or `ufw`:
   ```bash
   # Example: Block everyone from port 8081, but allow trusted Home Assistant IP
   sudo ufw deny 8081/tcp
   sudo ufw allow from <HOME_ASSISTANT_IP> to any port 8081 proto tcp
   ```
