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
Create a directory on the host and run the container via Docker Compose:
```yaml
services:
  threadgate:
    container_name: threadgate
    image: ghcr.io/mikeo7/threadgate:latest
    network_mode: host
    restart: unless-stopped
    privileged: true
    devices:
      - /dev/ttyUSB0:/dev/ttyUSB0
    volumes:
      - ./data:/data
      - /dev:/dev
      - /lib/modules:/lib/modules
    environment:
      - OTBR_RADIO_URL=                  # Set empty for USB auto-discovery
      - OTBR_BAUDRATE=460800             # Optimized for smart home networks
      - OTBR_PORT=8081                   # REST API / Dashboard port
      - OTBR_AUTO_DISCOVER=true
```

Simply run:
```bash
docker compose up -d
```

Access the gorgeous diagnostic dashboard by visiting `http://localhost:8081` in your browser!
