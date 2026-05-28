## 2024-05-28 - Initial Reconnaissance
## 2024-05-28 - Transient Thread Topology Diagnostics
**Learning:** Thread network topology snapshots from `ot-ctl` diagnostics are transient and frequently desync from fully converged live routing paths during propagation.
**Action:** Always maintain and document fallback rendering mechanisms (like BFS pathing) when dealing with diagnostics to prevent visual breakage or false failure reporting when intermediate hop data is momentarily absent.
