# ThreadGate Reference Specification
This document serves as the permanent reference specification and single source of truth for building **ThreadGate**, a high-fidelity, secure, and self-healing OpenThread Border Router (OTBR) container managed by a custom Golang orchestrator.

---

## 1. Core Container & Host Network Requirements
An OTBR requires full interaction with the host system's Linux network stack to route IPv6 packets and perform multicast group registrations.

### Kernel & Firewall Requirements
- **IP Forwarding**: IPv6 forwarding must be enabled on the host system to route packets between the Thread interface (`wpan0`) and standard LAN networks:
  ```bash
  sysctl -w net.ipv6.conf.all.forwarding=1
  sysctl -w net.ipv6.conf.default.forwarding=1
  ```
- **Router Advertisements**: The host must accept router advertisements:
  ```bash
  sysctl -w net.ipv6.conf.all.accept_ra=2
  sysctl -w net.ipv6.conf.default.accept_ra=2
  ```
- **Firewall Support**: Ensure `ip6table_filter` and related modules are loaded on the host to avoid interface initialization failures.

### Docker Privileges & Networking
- **Host Network Mode**: `network_mode: host` is **mandatory**. Bridged networks prevent mDNS multicast discovery, making Home Assistant auto-discovery fail.
- **Capabilities & Tun Device**:
  - The container must be run with `--cap-add=NET_ADMIN` or in `--privileged` mode to create the virtual network interface `wpan0` via `/dev/net/tun`.
  - The tun device `/dev/net/tun` must be passed into the container.

---

## 2. USB RCP Radio hardware Profiles
The Go Orchestrator (`threadgate`) will monitor serial interfaces and auto-bind compatible hardware. Common USB Thread coordinators to support:

| Manufacturer / Model | USB Vendor ID (VID) | USB Product ID (PID) | Standard Driver | Recommended Baudrate |
| :--- | :--- | :--- | :--- | :--- |
| **Home Assistant Connect ZBT-1** | `10c4` | `ea60` (CP2102) | `cp21x` | `460800` |
| **Sonoff ZBT-1 / ZBDongle-E** | `10c4` | `ea60` (CP2102) | `cp21x` | `460800` |
| **Nordic nRF52840 Dongle** | `1915` | `528f` | `cdc_acm` | `115200` |
| **Silicon Labs SLWSTK6020B** | `1366` | `0101` (SEGGER) | `cdc_acm` | `460800` |

### Radio URL Format
The URL scheme passed to `otbr-agent` is structured as:
```
spinel+hdlc+uart://<device_path>?uart-baudrate=<baudrate>
```
*Example*: `spinel+hdlc+uart:///dev/ttyUSB0?uart-baudrate=460800`

---

## 3. D-Bus Introspection Details
`otbr-agent` exposes its management APIs via D-Bus to allow host-level services to configure the Thread network.

- **D-Bus Name**: `io.openthread.BorderRouter`
- **Object Path**: `/io/openthread/BorderRouter/wpan0` (or similar interface name)
- **Interface**: `io.openthread.BorderRouter`

### Key Methods to Monitor / Query:
1. **`Scan()`**: Triggers an IEEE 802.15.4 active scan for nearby Thread networks.
2. **`Attach()`**: Binds to a Thread network using a specified Operational Dataset.
3. **`Detach()`**: Disconnects from the current Thread network.
4. **`FactoryReset()`**: Clears Thread state, keys, and dataset.
5. **`GetProperties()`**: Retrieves details like connection status, active datasets, TX power, and network name.

---

## 4. REST API & OpenAPI Specifications (Port 8081)
To integrate with Home Assistant natively, the Go Orchestrator must expose (or proxy) the official REST API on **Port 8081**.

### Key API Endpoints Expected by Home Assistant:

#### 1. Node Info
- **URL**: `GET /api/node`
- **Response Format**: `JSON`
- **Sample Payload**:
  ```json
  {
    "State": "leader",
    "Rloc16": 49152,
    "ExtAddress": "0225134efd0c72c1",
    "NetworkName": "Thread-HASS",
    "ExtPanId": "deadbeef00facade"
  }
  ```

#### 2. Active Operational Dataset
- **URL**: `GET /node/dataset/active` or `GET /api/node/dataset/active`
- **Description**: Returns the active dataset containing credentials, master keys, and PAN IDs.
- **Response Format**: Hex-encoded string representing the binary Thread TLVs, or structured JSON.

#### 3. Pending Operational Dataset
- **URL**: `PUT /node/dataset/pending` or `PUT /api/node/dataset/pending`
- **Description**: Applies new pending network parameters to the Border Router.

#### 4. Diagnostics Collection
- **URL**: `GET /api/diagnostics`
- **Description**: Returns the raw diagnostics properties of the Thread network for telemetry, including routing tables, RSSI, and error counters.

---

## 5. Persistence Requirements
All configuration files, operational datasets, and network key databases are stored in the state directory.
- **Default Directory**: `/data`
- **Key Files to Persist**:
  - Thread dataset backups: `/data/otbr-agent.state` (keeps Thread networks intact across reboots/container upgrades).
  - Web UI customizations & system configuration files.
