<p align="center">
  <img src="src/manager/internal/api/logo.svg" alt="ThreadGate Logo" width="120" height="120" />
</p>

# ThreadGate


> [!WARNING]
> **Under Heavy Development**: This project is undergoing rapid, active development and is not yet suitable for production environments. System stability, configuration options, and APIs are subject to frequent and major breaking changes without notice. Use at your own discretion.

ThreadGate is a modern, self-healing OpenThread Border Router (OTBR) container optimized for Home Assistant and standalone Docker environments. It wraps Google's certified C++ `ot-br-posix` architecture with a robust **Golang orchestrator** (`threadgate`) that replaces standard entrypoint shell scripts, automates hardware USB mapping, and manages self-healing service loops.

---

## Key Features

- **Golang Supervisor Engine**: Acts as PID 1 to gracefully manage child processes (`dbus-daemon`, `avahi-daemon`, `otbr-agent`) and route POSIX OS signals perfectly.
- **Hardware Auto-Discovery**: USB signature scanner automatically detects common smart home coordinators (such as Home Assistant Connect ZBT-1, Sonoff, and Nordic dongles) and binds the exact spinel protocol URLs dynamically.
- **Automatic Self-Healing**: Actively monitors C++ sub-processes. If the `otbr-agent` encounters a critical fault, the manager cleanly recovers and restarts it.
- **Modern REST API & high-Fidelity UI**: Exposes the Home Assistant-expected REST API (Port `8081`) while hosting a premium, responsive HSL dark-themed system diagnostics dashboard. Automatically syncs with Home Assistant to resolve friendly, human-readable names for Thread nodes!
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
Create a directory on the host and run the container via Docker Compose. Use specific Linux capabilities instead of full `privileged` mode:

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
      - OTBR_HASS_URL=              # Optional: e.g., http://192.168.1.100:8123 (to fetch friendly node names)
      - OTBR_HASS_TOKEN=            # Optional: Long-Lived Access Token from Home Assistant
```

Simply run:
```bash
docker compose up -d
```

Access the diagnostic dashboard by visiting `http://localhost:8081` in your browser!

### 3. Local Home Assistant + ThreadGate (Docker, no USB dongle)

Spin up Home Assistant and ThreadGate on a shared Docker network, onboard HA, seed mock devices, pair the containers, and run end-to-end smoke tests:

```bash
make integration
```

First run takes a minute (onboarding + golden fixture). Later runs reuse `testdata/ha-fixture.tar.gz` and start much faster.

| Command | What it does |
|---------|----------------|
| `make integration` | Start stack, configure HA; **manual pairing** (dashboard approve), then verify |
| `make integration-auto` | Same, but **auto-approves** pairing (test other features without UI) |
| `make integration-fresh` | Wipe `testdata/`, manual pairing, tests |
| `make integration-fresh-auto` | Wipe `testdata/`, auto pairing, tests |
| `make integration-down` | Stop containers (keep config for quick `make integration`) |
| `make integration-reset` | Stop and delete local HA/ThreadGate test state |
| `make integration-test` | Re-run smoke tests only |

- Home Assistant: http://127.0.0.1:8123
- ThreadGate (mock OTBR): http://127.0.0.1:8081
- Credentials: `testdata/ha-credentials.env` (default user `admin`)

ThreadGate runs with `OTBR_MOCK_MODE=true` so no physical Thread radio is required.

**Pairing (manual, default):** When setup reaches pairing, open **http://127.0.0.1:8081** and click **Approve Connection**. The script waits up to 10 minutes.

**Pairing (auto):** `make integration-auto`, `make integration-fresh-auto`, `./scripts/hass-dev.sh up-auto`, or `PAIR_AUTO_APPROVE=1 ./scripts/hass-dev.sh up`.

With a golden fixture, `hass-dev.sh` starts ThreadGate first, applies the HA config snapshot while Home Assistant is **stopped**, then starts HA once (avoids hanging on a mid-boot `docker stop`). If HA stays down, run `docker compose -f docker-compose.integration.yml up -d homeassistant` and check `docker logs threadgate-ha`.

**Pairing popup:** The banner on http://127.0.0.1:8081 appears only after something calls `POST /api/pair/initiate` (not automatically from HA’s OTBR setup). Use `./scripts/hass-dev.sh up` (manual, default), or `./scripts/hass-dev.sh pair-initiate` while the stack is running. Auto-approve paths (`up-auto`, `make integration-auto`) skip the popup by design.

### 4. Home Assistant Device Name Synchronization (Optional)
To display user-friendly device names (like `"Living Room Motion Sensor"` or `"Kitchen Thermostat"`) in the system topology map instead of raw hex values (like `0xc001`), ThreadGate can query your Home Assistant device registry.

1. Go to your **Home Assistant Profile** (click your username in the bottom left).
2. Scroll to the bottom and click **Create Token** under *Long-Lived Access Tokens*.
3. Add `OTBR_HASS_URL` and `OTBR_HASS_TOKEN` to your `environment` block in the compose file as shown above.

> [!NOTE]
> **Try it in Mock Mode**: If you want to test or preview friendly name rendering in the UI without real hardware or Home Assistant credentials, you can set `OTBR_MOCK_MODE=true` in your environment. ThreadGate will automatically populate a set of simulated devices with realistic friendly names (such as `"Living Room Multi-Sensor"`, `"Kitchen Smart Plug"`, and `"Office Desk Lamp"`) so you can explore the fully-mapped topology!

---

## Home Assistant setup (production)

ThreadGate exposes an **OTBR-compatible REST API** on port `8081`. Home Assistant uses its built-in **Open Thread Border Router** integration (not a separate “ThreadGate” integration name).

1. **Deploy ThreadGate** with `docker-compose.yml` on the host (see host sysctls in section 1). Do **not** set `OTBR_MOCK_MODE` for real hardware.
2. In Home Assistant: **Settings → Devices & services → Add integration → Open Thread Border Router** → URL `http://<HOST_IP>:8081` (use the machine’s LAN IP; `network_mode: host` does not provide a `threadgate` hostname).
3. Confirm the OTBR entry shows **Loaded** (not “Failed to set up”). If it failed after an upgrade, reload under **Settings → Devices & services**, or run `hassdev repair-otbr` (see integration scripts below).
4. **Pairing (optional, for friendly names):** Call `POST http://<HOST_IP>:8081/api/pair/initiate` or use your setup flow, then **Approve** on the ThreadGate dashboard. Saves `/data/hass_config.json`. Use an HA URL reachable **from inside** the ThreadGate container (often `http://127.0.0.1:8123` when HA runs on the same host with host networking).
5. Let Home Assistant **create a new Thread network** (do not reuse OpenThread web UI default credentials). Restrict port **8081** to the HA host (see Security below).

### Production checklist (real Thread radio)

| Step | Action |
|------|--------|
| Host | `net.ipv6.conf.all/default.forwarding=1`, `accept_ra=2` |
| Radio | RCP Thread firmware on USB coordinator (not Zigbee-only firmware) |
| Compose | `docker compose up -d` with **mock mode off**, dongle on `/dev/ttyUSB0` (or set `OTBR_RADIO_URL`) |
| Dashboard | **Radio Connected**, leader/router state (not `ThreadGate-Mock`) |
| HA OTBR | Integration **Loaded**; no “Insecure Thread network” repair |
| Security | Firewall: allow `8081/tcp` only from Home Assistant |
| Matter | Commission one Thread/Matter device end-to-end |

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
