        (function() {
            var boot = window.__topology || {};
            var canvas = document.getElementById("topologyCanvas");
            var ctx = canvas.getContext("2d");
            var tooltip = document.getElementById("tooltip");

            // ---- Theme-aware palette (canvas can't read CSS vars directly) ----
            function readVar(name, fallback) {
                var v = getComputedStyle(document.documentElement).getPropertyValue(name);
                return (v && v.trim()) || fallback;
            }
            function hexToRgb(hex) {
                hex = (hex || "").trim().replace("#", "");
                if (hex.length === 3) {
                    hex = hex.split("").map(function(c) { return c + c; }).join("");
                }
                var n = parseInt(hex, 16);
                return { r: (n >> 16) & 255, g: (n >> 8) & 255, b: n & 255 };
            }
            function withAlpha(hex, a) {
                var c = hexToRgb(hex);
                return "rgba(" + c.r + "," + c.g + "," + c.b + "," + a + ")";
            }
            var palette = {};
            function refreshPalette() {
                palette = {
                    bg: readVar("--map-bg", "#0a0b0f"),
                    grid: readVar("--map-grid", "rgba(255,255,255,0.04)"),
                    gridAccent: readVar("--map-grid-accent", "rgba(129,140,248,0.12)"),
                    gateway: readVar("--map-gateway", "#6366f1"),
                    success: readVar("--map-success", "#10b981"),
                    warning: readVar("--map-warning", "#f59e0b"),
                    danger: readVar("--map-danger", "#f43f5e"),
                    child: readVar("--map-child", "#22d3ee"),
                    route: readVar("--map-route", "#a78bfa"),
                    nodeCore: readVar("--map-node-core", "#ffffff"),
                    labelBg: readVar("--map-label-bg", "rgba(10,11,15,0.9)"),
                    labelBorder: readVar("--map-label-border", "rgba(255,255,255,0.1)"),
                    labelText: readVar("--map-label-text", "#cbd5e1"),
                    particle: readVar("--map-particle", "#c7d2fe")
                };
            }
            refreshPalette();
            window.__refreshMapPalette = refreshPalette;
            try {
                window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", refreshPalette);
            } catch (e) {}

            function lqiColor(lqi) {
                return lqi === 3 ? palette.success : (lqi === 2 ? palette.warning : palette.danger);
            }

            function mapLink(l) {
                return {
                    From: l.from || l.FromRloc16 || "",
                    To: l.to || l.ToRloc16 || "",
                    Kind: l.kind || l.Kind || "",
                    PathCost: l.pathCost != null ? l.pathCost : (l.PathCost || 0)
                };
            }

            var rawNeighbors = (boot.neighbors || []).map(function(n) {
                return {
                    ExtAddr: n.extAddr || n.ExtAddr || "",
                    Rloc16: n.rloc16 || n.Rloc16 || "",
                    LQI: n.lqi != null ? n.lqi : (n.LQI || 3),
                    PathCost: n.pathCost != null ? n.pathCost : (n.PathCost || 0),
                    NextHopRloc: n.nextHopRloc || n.NextHopRloc || "",
                    Role: n.role || n.Role || ""
                };
            });

            var meshLinks = (boot.meshLinks || []).map(mapLink);


            function normalizeRloc(value) {
                value = (value || "").toLowerCase();
                return value.indexOf("0x") === 0 ? value.slice(2) : value;
            }

            // High-performance key-value object map lookup for O(1) node discovery
            var nodesByRloc = {};
            var nodesByRlocBuilt = false;

            function findNodeByRloc(rloc) {
                if (!nodesByRlocBuilt) {
                    nodesByRloc = {};
                    for (var i = 0; i < nodes.length; i++) {
                        var key = normalizeRloc(nodes[i].rloc);
                        // Preserve original behavior: return the first matching node
                        if (nodesByRloc[key] === undefined) {
                            nodesByRloc[key] = nodes[i];
                        }
                    }
                    nodesByRlocBuilt = true;
                }
                return nodesByRloc[normalizeRloc(rloc)] || null;
            }

            function buildRoutingTree() {
                var tree = boot.routingTree || {};
                var parentOf = {};
                var childrenOf = tree.childrenOf || {};
                var rawParentOf = tree.parentOf || {};
                Object.keys(rawParentOf).forEach(function(childKey) {
                    var entry = rawParentOf[childKey];
                    parentOf[childKey] = {
                        parent: entry.parent,
                        link: mapLink(entry.link || {})
                    };
                });
                return {
                    parentOf: parentOf,
                    childrenOf: childrenOf,
                    gatewayKey: tree.gatewayKey || normalizeRloc(boot.rloc16 || "")
                };
            }

            function measureRoutingSubtree(nodeKey, childrenOf, memo) {
                if (memo[nodeKey] !== undefined) {
                    return memo[nodeKey];
                }
                var total = 1;
                (childrenOf[nodeKey] || []).forEach(function(childKey) {
                    total += measureRoutingSubtree(childKey, childrenOf, memo);
                });
                memo[nodeKey] = total;
                return total;
            }

            function drawRouteTreeLinks(cx, cy, hoveredNode, routingTree) {
                var pathKeys = null;
                if (hoveredNode && !hoveredNode.isCenter) {
                    var routePath = getTrafficPath(hoveredNode);
                    pathKeys = {};
                    for (var p = 0; p < routePath.length - 1; p++) {
                        var a = normalizeRloc(routePath[p].rloc);
                        var b = normalizeRloc(routePath[p + 1].rloc);
                        pathKeys[a + "|" + b] = true;
                        pathKeys[b + "|" + a] = true;
                    }
                }

                Object.keys(routingTree.parentOf).forEach(function(childKey) {
                    var entry = routingTree.parentOf[childKey];
                    var link = entry.link;
                    var fromNode = findNodeByRloc(link.From);
                    var toNode = findNodeByRloc(link.To);
                    if (!fromNode || !toNode) {
                        return;
                    }

                    var fromR = normalizeRloc(link.From);
                    var toR = normalizeRloc(link.To);
                    var isOnPath = pathKeys && (pathKeys[fromR + "|" + toR] || pathKeys[toR + "|" + fromR]);
                    var isConnected = hoveredNode && !hoveredNode.isCenter &&
                        (fromNode === hoveredNode || toNode === hoveredNode);
                    var isHighlighted = isOnPath || isConnected;
                    var dimmed = hoveredNode && !hoveredNode.isCenter && !isHighlighted;

                    var x1 = cx + fromNode.x;
                    var y1 = cy + fromNode.y;
                    var x2 = cx + toNode.x;
                    var y2 = cy + toNode.y;

                    ctx.save();
                    ctx.globalAlpha = dimmed ? 0.12 : (isHighlighted ? 1 : 0.72);
                    ctx.lineCap = "round";
                    ctx.beginPath();
                    ctx.moveTo(x1, y1);
                    ctx.lineTo(x2, y2);

                    if (link.Kind === "direct") {
                        ctx.strokeStyle = withAlpha(palette.success, isHighlighted ? 0.95 : 0.55);
                        ctx.lineWidth = isHighlighted ? 2.4 : 1.8;
                        ctx.setLineDash([]);
                    } else if (link.Kind === "child") {
                        ctx.strokeStyle = withAlpha(palette.child, isHighlighted ? 0.95 : 0.5);
                        ctx.lineWidth = isHighlighted ? 2.2 : 1.5;
                        ctx.setLineDash([3, 5]);
                    } else {
                        ctx.strokeStyle = withAlpha(palette.route, isHighlighted ? 0.95 : 0.45);
                        ctx.lineWidth = isHighlighted ? 2 : 1.3;
                        ctx.setLineDash([]);
                    }
                    ctx.stroke();
                    ctx.setLineDash([]);
                    ctx.restore();
                });
            }

            function drawMeshLinks(cx, cy, hoveredNode) {
                var pathKeys = null;
                if (hoveredNode && !hoveredNode.isCenter) {
                    var routePath = getTrafficPath(hoveredNode);
                    pathKeys = {};
                    for (var p = 0; p < routePath.length - 1; p++) {
                        var a = normalizeRloc(routePath[p].rloc);
                        var b = normalizeRloc(routePath[p + 1].rloc);
                        pathKeys[a + "|" + b] = true;
                        pathKeys[b + "|" + a] = true;
                    }
                }

                meshLinks.forEach(function(link) {
                    var fromNode = findNodeByRloc(link.From);
                    var toNode = findNodeByRloc(link.To);
                    if (!fromNode || !toNode) {
                        return;
                    }

                    var x1 = cx + fromNode.x;
                    var y1 = cy + fromNode.y;
                    var x2 = cx + toNode.x;
                    var y2 = cy + toNode.y;
                    var fromKey = normalizeRloc(link.From);
                    var toKey = normalizeRloc(link.To);
                    var isOnPath = pathKeys && (pathKeys[fromKey + "|" + toKey] || pathKeys[toKey + "|" + fromKey]);
                    var isConnected = hoveredNode && !hoveredNode.isCenter &&
                        (fromNode === hoveredNode || toNode === hoveredNode);
                    var isHighlighted = isOnPath || isConnected;
                    var dimmed = hoveredNode && !hoveredNode.isCenter && !isHighlighted;

                    ctx.save();
                    ctx.globalAlpha = dimmed ? 0.12 : 1;
                    ctx.lineCap = "round";

                    ctx.beginPath();
                    if (link.Kind === "route") {
                        var mx = (x1 + x2) / 2;
                        var my = (y1 + y2) / 2;
                        var dx = x2 - x1;
                        var dy = y2 - y1;
                        var len = Math.hypot(dx, dy) || 1;
                        var bow = Math.min(28, len * 0.14);
                        var cx1 = mx - (dy / len) * bow;
                        var cy1 = my + (dx / len) * bow;
                        ctx.moveTo(x1, y1);
                        ctx.quadraticCurveTo(cx1, cy1, x2, y2);
                    } else {
                        ctx.moveTo(x1, y1);
                        ctx.lineTo(x2, y2);
                    }

                    if (link.Kind === "direct") {
                        var lqi = toNode.lqi || 3;
                        ctx.strokeStyle = withAlpha(lqiColor(lqi), isHighlighted ? 0.9 : 0.55);
                        ctx.lineWidth = lqi === 3 ? (isHighlighted ? 2.6 : 2.2) : (lqi === 2 ? (isHighlighted ? 2.2 : 1.8) : (isHighlighted ? 2 : 1.5));
                        ctx.setLineDash([]);
                    } else if (link.Kind === "child") {
                        ctx.strokeStyle = withAlpha(palette.child, isHighlighted ? 0.85 : 0.5);
                        ctx.lineWidth = isHighlighted ? 2 : 1.6;
                        ctx.setLineDash([2, 4]);
                    } else {
                        ctx.strokeStyle = withAlpha(palette.route, isHighlighted ? 0.95 : 0.42);
                        ctx.lineWidth = isHighlighted ? 2 : 1.4;
                        ctx.setLineDash([5, 5]);
                    }
                    ctx.stroke();
                    ctx.setLineDash([]);

                    if (link.Kind === "route" && isHighlighted) {
                        var labelX = (x1 + x2) / 2;
                        var labelY = (y1 + y2) / 2;
                        ctx.font = "600 9px 'JetBrains Mono', monospace";
                        ctx.fillStyle = withAlpha(palette.route, 0.9);
                        ctx.textAlign = "center";
                        ctx.fillText("via", labelX, labelY - 4);
                    }

                    ctx.restore();
                });
            }

            var serverTrafficPaths = boot.trafficPaths || {};
            var routingTree = null;

            // Setup central node (OTBR)
            var nodes = [{
                x: 0,
                y: 0,
                label: "Border Router",
                mac: boot.extAddress || "",
                rloc: boot.rloc16 || "",
                isCenter: true,
                baseRadius: 18,
                pulsePhase: 0
            }];

            var particles = [];
            var layout = null;
            var mapViewMode = "route";
            var showTraffic = true;

            function getTrafficPath(targetNode) {
                if (targetNode.isCenter) {
                    return [targetNode];
                }
                var key = normalizeRloc(targetNode.rloc);
                var rlocPath = serverTrafficPaths[key];
                if (!rlocPath || rlocPath.length === 0) {
                    return nodes[0] ? [nodes[0], targetNode] : [targetNode];
                }
                var path = [];
                for (var i = 0; i < rlocPath.length; i++) {
                    var node = findNodeByRloc(rlocPath[i]);
                    if (node) {
                        path.push(node);
                    }
                }
                if (path.length >= 2) {
                    return path;
                }
                return nodes[0] ? [nodes[0], targetNode] : [targetNode];
            }

            function getPathPosition(path, progress, inbound, cx, cy) {
                if (!path || path.length === 0) {
                    return { x: cx, y: cy };
                }
                if (path.length === 1) {
                    return { x: cx + path[0].x, y: cy + path[0].y };
                }

                var segments = path.length - 1;
                var travel = progress * segments;
                var segIdx = Math.min(Math.floor(travel), segments - 1);
                var segT = travel - segIdx;
                var fromNode;
                var toNode;

                if (inbound) {
                    fromNode = path[segIdx + 1];
                    toNode = path[segIdx];
                } else {
                    fromNode = path[segIdx];
                    toNode = path[segIdx + 1];
                }

                return {
                    x: cx + fromNode.x + (toNode.x - fromNode.x) * segT,
                    y: cy + fromNode.y + (toNode.y - fromNode.y) * segT
                };
            }

            function drawTrafficSegment(path, progress, inbound, cx, cy) {
                if (!path || path.length < 2) {
                    return;
                }

                var segments = path.length - 1;
                var travel = progress * segments;
                var segIdx = Math.min(Math.floor(travel), segments - 1);
                var fromNode = inbound ? path[segIdx + 1] : path[segIdx];
                var toNode = inbound ? path[segIdx] : path[segIdx + 1];

                ctx.beginPath();
                ctx.moveTo(cx + fromNode.x, cy + fromNode.y);
                ctx.lineTo(cx + toNode.x, cy + toNode.y);
                ctx.strokeStyle = withAlpha(palette.gateway, 0.22);
                ctx.lineWidth = 2.5;
                ctx.setLineDash([]);
                ctx.stroke();
            }

            function setTrafficEnabled(enabled) {
                showTraffic = enabled;
                var btn = document.getElementById("btn-traffic");
                btn.classList.toggle("active", enabled);
                btn.classList.toggle("traffic-off", !enabled);
                btn.textContent = enabled ? "Traffic On" : "Traffic Off";
                if (!enabled) {
                    particles.length = 0;
                }
            }

            function countNodesByPathCost(maxPathCost) {
                var ringCounts = {};
                for (var cost = 0; cost <= maxPathCost; cost++) {
                    ringCounts[cost] = 0;
                }
                rawNeighbors.forEach(function(n) {
                    var cost = n.PathCost || 0;
                    if (cost > maxPathCost) {
                        cost = maxPathCost;
                    }
                    ringCounts[cost] = (ringCounts[cost] || 0) + 1;
                });
                return ringCounts;
            }

            function computeRouteRingRadii(maxOrbit, maxPathCost, ringCounts, nodeRadius) {
                var radii = [];
                var innerMin = nodeRadius * 4.5;
                var outerMax = maxOrbit * 0.96;

                for (var cost = 0; cost <= maxPathCost; cost++) {
                    var count = Math.max(ringCounts[cost] || 1, 1);
                    var minForSpacing = (count * nodeRadius * 2.75) / (2 * Math.PI);
                    var linear = innerMin + ((cost + 1) / (maxPathCost + 1)) * (outerMax - innerMin);
                    radii[cost] = Math.max(linear, minForSpacing);
                }

                for (var i = 1; i <= maxPathCost; i++) {
                    radii[i] = Math.max(radii[i], radii[i - 1] + nodeRadius * 3.5);
                }
                radii[maxPathCost] = Math.max(radii[maxPathCost], outerMax);

                return radii;
            }

            function ellipseRadius(node, layout) {
                var sx = layout.orbitScaleX || 1;
                var sy = layout.orbitScaleY || 1;
                return Math.hypot(node.x / sx, node.y / sy);
            }

            function setEllipsePosition(node, angle, radius, layout) {
                var sx = layout.orbitScaleX || 1;
                var sy = layout.orbitScaleY || 1;
                node.x = Math.cos(angle) * radius * sx;
                node.y = Math.sin(angle) * radius * sy;
                node.targetRadius = radius;
            }

            function constrainToEllipse(node, layout) {
                var sx = layout.orbitScaleX || 1;
                var sy = layout.orbitScaleY || 1;
                var dist = ellipseRadius(node, layout);
                var limit = layout.maxOrbit;

                if (node.targetRadius && dist > 0) {
                    var angle = Math.atan2(node.y / sy, node.x / sx);
                    var nextDist = dist + (node.targetRadius - dist) * 0.06;
                    node.x = Math.cos(angle) * nextDist * sx;
                    node.y = Math.sin(angle) * nextDist * sy;
                }

                dist = ellipseRadius(node, layout);
                if (dist > limit && dist > 0) {
                    var clampAngle = Math.atan2(node.y / sy, node.x / sx);
                    node.x = Math.cos(clampAngle) * limit * sx;
                    node.y = Math.sin(clampAngle) * limit * sy;
                }
            }

            function drawOrbitRing(cx, cy, radius, layout) {
                var sx = layout.orbitScaleX || 1;
                var sy = layout.orbitScaleY || 1;
                ctx.save();
                ctx.translate(cx, cy);
                ctx.scale(sx, sy);
                ctx.beginPath();
                ctx.arc(0, 0, radius, 0, Math.PI * 2);
                ctx.stroke();
                ctx.restore();
            }

            function computeMeshLayout(rect) {
                var neighborCount = rawNeighbors.length;
                var centerRadius = 18;
                var nodeRadius = neighborCount > 20 ? 7 : (neighborCount > 10 ? 9 : 11);
                var edgePadding = mapViewMode === "route" ? 8 : 16;
                var labelPadding = mapViewMode === "route" ? 12 : 8;

                var maxOrbitX = rect.width / 2 - edgePadding - nodeRadius - labelPadding;
                var maxOrbitY = rect.height / 2 - edgePadding - nodeRadius - labelPadding;
                var maxOrbit = Math.max(80, Math.min(maxOrbitX, maxOrbitY));
                var orbitScaleX = mapViewMode === "route" ? maxOrbitX / maxOrbit : 1;
                var orbitScaleY = mapViewMode === "route" ? maxOrbitY / maxOrbit : 1;
                var minArcSpacing = nodeRadius * 2.2 + 4;
                var minOrbitForSpacing = neighborCount > 0
                    ? (neighborCount * minArcSpacing) / (Math.PI * 2)
                    : 60;
                var orbitRadius = Math.min(maxOrbit, Math.max(minOrbitForSpacing, maxOrbit));

                var maxPathCost = 1;
                rawNeighbors.forEach(function(n) {
                    var cost = n.PathCost || 0;
                    if (cost > maxPathCost) {
                        maxPathCost = cost;
                    }
                });

                var ringCount = Math.max(maxPathCost + 1, 2);
                var ringStep = maxOrbit / ringCount;
                var ringRadii = [];

                if (mapViewMode === "route") {
                    var ringCounts = countNodesByPathCost(maxPathCost);
                    ringRadii = computeRouteRingRadii(maxOrbit, maxPathCost, ringCounts, nodeRadius);
                    ringStep = ringRadii.length > 1
                        ? (ringRadii[ringRadii.length - 1] - ringRadii[0]) / Math.max(ringRadii.length - 1, 1)
                        : maxOrbit;
                } else {
                    for (var ring = 0; ring <= maxPathCost; ring++) {
                        ringRadii[ring] = ((ring + 1) / ringCount) * maxOrbit * 0.98;
                    }
                }

                return {
                    centerRadius: centerRadius,
                    nodeRadius: nodeRadius,
                    maxOrbit: maxOrbit,
                    orbitScaleX: orbitScaleX,
                    orbitScaleY: orbitScaleY,
                    orbitRadius: orbitRadius,
                    ringStep: ringStep,
                    ringRadii: ringRadii,
                    maxPathCost: maxPathCost,
                    linkIdealDirect: ringStep * 0.92,
                    linkIdealChild: ringStep * 0.82,
                    linkIdealRoute: ringStep * 0.96,
                    minSeparation: Math.max(nodeRadius * 2.6 + 6, ringStep * 0.28),
                    showAllLabels: neighborCount <= 12
                };
            }

            function syncNodeMetadata(currentLayout) {
                rawNeighbors.forEach(function(n, index) {
                    var node = nodes[index + 1];
                    if (!node) {
                        return;
                    }
                    node.label = n.Rloc16.indexOf("0x") === 0 ? n.Rloc16 : "0x" + n.Rloc16;
                    node.mac = n.ExtAddr;
                    node.rloc = node.label;
                    node.lqi = n.LQI;
                    node.pathCost = n.PathCost || 0;
                    node.nextHopRloc = n.NextHopRloc || "";
                    node.role = n.Role || "";
                    node.baseRadius = currentLayout.nodeRadius;
                });
            }

            function placeNodesOnOrbit(currentLayout) {
                if (nodes.length <= 1) {
                    return;
                }

                nodes[0].baseRadius = currentLayout.centerRadius;
                syncNodeMetadata(currentLayout);

                rawNeighbors.forEach(function(n, index) {
                    var node = nodes[index + 1];
                    if (!node || node.pinned) {
                        if (node) {
                            clampNodeToBounds(node, currentLayout);
                        }
                        return;
                    }

                    var angle = rawNeighbors.length > 0
                        ? (index / rawNeighbors.length) * Math.PI * 2 - Math.PI / 2
                        : 0;
                    if (mapViewMode === "route") {
                        setEllipsePosition(node, angle, currentLayout.orbitRadius, currentLayout);
                    } else {
                        node.x = Math.cos(angle) * currentLayout.orbitRadius;
                        node.y = Math.sin(angle) * currentLayout.orbitRadius;
                    }
                    node.angle = angle;
                });
            }

            function applyStarPhysics(currentLayout) {
                nodes.forEach(function(node) {
                    if (node.isCenter || node === draggedNode || node.pinned) {
                        return;
                    }

                    var dx = node.x - nodes[0].x;
                    var dy = node.y - nodes[0].y;
                    var dist = Math.hypot(dx, dy) || 1;
                    var force = (currentLayout.orbitRadius - dist) * 0.025;
                    node.x += (dx / dist) * force;
                    node.y += (dy / dist) * force;

                    nodes.forEach(function(other) {
                        if (other === node || other.isCenter) {
                            return;
                        }
                        var odx = node.x - other.x;
                        var ody = node.y - other.y;
                        var odist = Math.hypot(odx, ody) || 1;
                        if (odist < currentLayout.minSeparation) {
                            var repForce = (currentLayout.minSeparation - odist) * 0.04;
                            node.x += (odx / odist) * repForce;
                            node.y += (ody / odist) * repForce;
                        }
                    });

                    clampNodeToBounds(node, currentLayout);
                });
            }

            function drawStarLinks(cx, cy) {
                var center = nodes[0];
                nodes.forEach(function(node) {
                    if (node.isCenter) {
                        return;
                    }

                    ctx.beginPath();
                    ctx.lineCap = "round";
                    ctx.moveTo(cx + center.x, cy + center.y);
                    ctx.lineTo(cx + node.x, cy + node.y);

                    if (node.lqi === 3) {
                        ctx.strokeStyle = withAlpha(palette.success, 0.45);
                        ctx.lineWidth = 2;
                        ctx.setLineDash([]);
                    } else if (node.lqi === 2) {
                        ctx.strokeStyle = withAlpha(palette.warning, 0.5);
                        ctx.lineWidth = 1.5;
                        ctx.setLineDash([4, 4]);
                    } else {
                        ctx.strokeStyle = withAlpha(palette.danger, 0.5);
                        ctx.lineWidth = 1.2;
                        ctx.setLineDash([2, 5]);
                    }
                    ctx.stroke();
                    ctx.setLineDash([]);
                });
            }

            function placeNodesByRelationships(currentLayout) {
                if (nodes.length <= 1) {
                    return;
                }

                nodes[0].baseRadius = currentLayout.centerRadius;
                syncNodeMetadata(currentLayout);
                routingTree = buildRoutingTree();

                var subtreeSizes = {};
                measureRoutingSubtree(routingTree.gatewayKey, routingTree.childrenOf, subtreeSizes);

                function layoutBranch(nodeKey, startAngle, endAngle) {
                    var node = nodeKey === routingTree.gatewayKey
                        ? nodes[0]
                        : findNodeByRloc(nodeKey);
                    if (!node) {
                        return;
                    }

                    if (!node.isCenter && !node.pinned) {
                        var pathCost = Math.max(0, Math.min(node.pathCost || 0, currentLayout.maxPathCost));
                        var radius = currentLayout.ringRadii[pathCost] || currentLayout.maxOrbit * 0.96;
                        var angle = (startAngle + endAngle) / 2;
                        setEllipsePosition(node, angle, radius, currentLayout);
                        node.placedAngle = angle;
                    }

                    var children = routingTree.childrenOf[nodeKey] || [];
                    if (children.length === 0) {
                        return;
                    }

                    var totalWeight = 0;
                    children.forEach(function(childKey) {
                        totalWeight += subtreeSizes[childKey] || 1;
                    });

                    var cursor = startAngle;
                    children.forEach(function(childKey) {
                        var weight = (subtreeSizes[childKey] || 1) / totalWeight;
                        var slice = (endAngle - startAngle) * weight;
                        layoutBranch(childKey, cursor, cursor + slice);
                        cursor += slice;
                    });
                }

                layoutBranch(routingTree.gatewayKey, -Math.PI / 2, Math.PI * 1.5);
            }

            function clampNodeToBounds(node, currentLayout) {
                if (node.isCenter) {
                    node.x = 0;
                    node.y = 0;
                    return;
                }

                if (mapViewMode === "route") {
                    var dist = ellipseRadius(node, currentLayout);
                    var limit = currentLayout.maxOrbit;
                    if (dist > limit && dist > 0) {
                        var sx = currentLayout.orbitScaleX || 1;
                        var sy = currentLayout.orbitScaleY || 1;
                        node.x = (node.x / dist) * limit * sx;
                        node.y = (node.y / dist) * limit * sy;
                    }
                    return;
                }

                var maxRadius = currentLayout.maxOrbit;
                var dist = Math.hypot(node.x, node.y);
                if (dist > maxRadius && dist > 0) {
                    node.x = (node.x / dist) * maxRadius;
                    node.y = (node.y / dist) * maxRadius;
                }
            }

            rawNeighbors.forEach(function(n, index) {
                var rlocLabel = n.Rloc16.indexOf("0x") === 0 ? n.Rloc16 : "0x" + n.Rloc16;
                nodes.push({
                    x: 0,
                    y: 0,
                    label: rlocLabel,
                    mac: n.ExtAddr,
                    rloc: rlocLabel,
                    lqi: n.LQI,
                    pathCost: n.PathCost || 0,
                    nextHopRloc: n.NextHopRloc || "",
                    isCenter: false,
                    pinned: false,
                    baseRadius: 8,
                    angle: 0,
                    pulsePhase: index * 45
                });
            });

            routingTree = buildRoutingTree();

            function refreshLayout() {
                var rect = canvas.getBoundingClientRect();
                layout = computeMeshLayout(rect);
                if (mapViewMode === "star") {
                    placeNodesOnOrbit(layout);
                } else {
                    placeNodesByRelationships(layout);
                }
            }

            function setMapView(mode) {
                if (mode !== "star" && mode !== "route") {
                    return;
                }
                mapViewMode = mode;
                document.getElementById("btn-view-star").classList.toggle("active", mode === "star");
                document.getElementById("btn-view-route").classList.toggle("active", mode === "route");
                document.getElementById("legend-star").classList.toggle("hidden", mode !== "star");
                document.getElementById("legend-route").classList.toggle("hidden", mode !== "route");
                nodes.forEach(function(node) {
                    if (!node.isCenter) {
                        node.pinned = false;
                        node.targetRadius = null;
                    }
                });
                refreshLayout();
            }

            function scaleCanvas() {
                var rect = canvas.getBoundingClientRect();
                var dpr = window.devicePixelRatio || 1;
                canvas.width = rect.width * dpr;
                canvas.height = rect.height * dpr;
                ctx.setTransform(1, 0, 0, 1, 0, 0);
                ctx.scale(dpr, dpr);
                refreshLayout();
            }

            scaleCanvas();
            window.addEventListener('resize', scaleCanvas);
            document.getElementById("btn-view-star").addEventListener("click", function() {
                setMapView("star");
            });
            document.getElementById("btn-view-route").addEventListener("click", function() {
                setMapView("route");
            });
            document.getElementById("btn-traffic").addEventListener("click", function() {
                setTrafficEnabled(!showTraffic);
            });

            var mouseX = -999;
            var mouseY = -999;
            var draggedNode = null;
            var dragOffsetX = 0;
            var dragOffsetY = 0;

            canvas.addEventListener('mousedown', function(e) {
                var rect = canvas.getBoundingClientRect();
                var mx = e.clientX - rect.left;
                var my = e.clientY - rect.top;
                var cx = rect.width / 2;
                var cy = rect.height / 2;

                for (var i = 0; i < nodes.length; i++) {
                    var node = nodes[i];
                    var screenX = cx + node.x;
                    var screenY = cy + node.y;
                    var dist = Math.hypot(mx - screenX, my - screenY);
                    if (dist < (node.baseRadius + 15)) {
                        draggedNode = node;
                        dragOffsetX = mx - screenX;
                        dragOffsetY = my - screenY;
                        canvas.style.cursor = "grabbing";
                        break;
                    }
                }
            });

            canvas.addEventListener('mousemove', function(e) {
                var rect = canvas.getBoundingClientRect();
                var mx = e.clientX - rect.left;
                var my = e.clientY - rect.top;
                var cx = rect.width / 2;
                var cy = rect.height / 2;

                mouseX = mx;
                mouseY = my;

                if (draggedNode) {
                    draggedNode.x = mx - cx - dragOffsetX;
                    draggedNode.y = my - cy - dragOffsetY;
                    clampNodeToBounds(draggedNode, layout);
                } else {
                    // Hover pointer feedback
                    var hovering = false;
                    for (var i = 0; i < nodes.length; i++) {
                        var node = nodes[i];
                        if (Math.hypot(mx - (cx + node.x), my - (cy + node.y)) < (node.baseRadius + 8)) {
                            hovering = true;
                            break;
                        }
                    }
                    canvas.style.cursor = hovering ? "grab" : "default";
                }
            });

            canvas.addEventListener('mouseleave', function() {
                mouseX = -999;
                mouseY = -999;
                tooltip.style.display = "none";
            });

            window.addEventListener('mouseup', function() {
                if (draggedNode && !draggedNode.isCenter) {
                    draggedNode.pinned = true;
                }
                draggedNode = null;
                if (canvas.style.cursor === "grabbing") {
                    canvas.style.cursor = "grab";
                }
            });

            canvas.addEventListener('dblclick', function() {
                nodes.forEach(function(node) {
                    if (!node.isCenter) {
                        node.pinned = false;
                    }
                });
                refreshLayout();
            });

            function draw() {
                var rect = canvas.getBoundingClientRect();
                var cx = rect.width / 2;
                var cy = rect.height / 2;

                if (!layout) {
                    layout = computeMeshLayout(rect);
                }

                // Solid background clear to prevent node dragging trails
                ctx.fillStyle = palette.bg;
                ctx.fillRect(0, 0, rect.width, rect.height);

                // Draw faint hop guides
                ctx.strokeStyle = palette.gridAccent;
                ctx.lineWidth = 1;
                ctx.setLineDash([4, 14]);
                if (mapViewMode === "route" && layout.ringRadii && layout.ringRadii.length) {
                    layout.ringRadii.forEach(function(r, idx) {
                        drawOrbitRing(cx, cy, r, layout);
                        ctx.save();
                        ctx.font = "600 9px 'JetBrains Mono', monospace";
                        ctx.fillStyle = withAlpha(palette.labelText, 0.5);
                        ctx.textAlign = "left";
                        var labelX = cx + r * (layout.orbitScaleX || 1) + 8;
                        ctx.fillText("hop " + idx, labelX, cy - 6);
                        ctx.restore();
                    });
                } else {
                    var ringStep = layout.ringStep || (layout.maxOrbit / 4);
                    for (var r = ringStep; r <= layout.maxOrbit; r += ringStep) {
                        if (mapViewMode === "route") {
                            drawOrbitRing(cx, cy, r, layout);
                        } else {
                            ctx.beginPath();
                            ctx.arc(cx, cy, r, 0, Math.PI * 2);
                            ctx.stroke();
                        }
                    }
                }
                ctx.setLineDash([]);

                var hoveredNode = null;
                if (mouseX > -500) {
                    nodes.forEach(function(node) {
                        var screenX = cx + node.x;
                        var screenY = cy + node.y;
                        if (Math.hypot(mouseX - screenX, mouseY - screenY) < (node.baseRadius + 8)) {
                            hoveredNode = node;
                        }
                    });
                }

                if (mapViewMode === "star") {
                    applyStarPhysics(layout);
                    drawStarLinks(cx, cy);
                } else if (routingTree) {
                    drawRouteTreeLinks(cx, cy, hoveredNode, routingTree);
                }

                // Transmit traffic flow particles along routing hops
                if (showTraffic && Math.random() < 0.04 && nodes.length > 1) {
                    var targetNode = nodes[Math.floor(Math.random() * (nodes.length - 1)) + 1];
                    var path = getTrafficPath(targetNode);
                    if (path.length >= 2) {
                        var hopCount = path.length - 1;
                        particles.push({
                            target: targetNode,
                            path: path,
                            hopCount: hopCount,
                            progress: 0,
                            speed: (0.007 + Math.random() * 0.008) / hopCount,
                            inbound: Math.random() < 0.5,
                            color: lqiColor(targetNode.lqi),
                            trail: []
                        });
                    }
                }

                for (var i = particles.length - 1; i >= 0; i--) {
                    var p = particles[i];
                    if (!showTraffic) {
                        particles.splice(i, 1);
                        continue;
                    }

                    p.progress += p.speed;

                    if (p.progress >= 1) {
                        var direction = p.inbound ? "Received" : "Transmitted";
                        var byteCount = Math.floor(Math.random() * 128) + 16;
                        addLog("TRAFFIC", `${direction} ${byteCount} bytes via ${p.hopCount} hop(s) ${p.inbound ? 'from' : 'to'} node: <span style="color: var(--violet);">${p.target.rloc}</span>`, "var(--cyan)");
                        particles.splice(i, 1);
                        continue;
                    }

                    drawTrafficSegment(p.path, p.progress, p.inbound, cx, cy);
                    var pos = getPathPosition(p.path, p.progress, p.inbound, cx, cy);
                    var px = pos.x;
                    var py = pos.y;

                    // Save trail
                    p.trail.push({x: px, y: py});
                    if (p.trail.length > 6) p.trail.shift();

                    // Render trail
                    p.trail.forEach(function(t, tIdx) {
                        ctx.beginPath();
                        ctx.arc(t.x, t.y, 1.5 + (tIdx * 0.25), 0, Math.PI * 2);
                        ctx.fillStyle = p.color;
                        ctx.globalAlpha = (tIdx + 1) / p.trail.length * 0.5;
                        ctx.fill();
                    });
                    ctx.globalAlpha = 1.0;

                    // Lead particle
                    ctx.beginPath();
                    ctx.arc(px, py, 3.5, 0, Math.PI * 2);
                    ctx.fillStyle = palette.nodeCore;
                    ctx.shadowBlur = 10;
                    ctx.shadowColor = p.color;
                    ctx.fill();
                    ctx.shadowBlur = 0;
                }

                // Render Nodes
                nodes.forEach(function(node) {
                    node.pulsePhase += node.isCenter ? 0.025 : 0.018;

                    var screenX = cx + node.x;
                    var screenY = cy + node.y;
                    var isHovered = node === hoveredNode;
                    var coreColor = node.isCenter ? palette.gateway : lqiColor(node.lqi);

                    // Glowing backdrop aura
                    var pulseSize = 1.35 + (Math.sin(node.pulsePhase) * 0.15);
                    ctx.beginPath();
                    ctx.arc(screenX, screenY, node.baseRadius * pulseSize, 0, Math.PI * 2);
                    ctx.fillStyle = withAlpha(coreColor, 0.10);
                    ctx.fill();

                    // Active border highlight
                    ctx.beginPath();
                    ctx.arc(screenX, screenY, node.baseRadius + (isHovered ? 4 : 2), 0, Math.PI * 2);
                    ctx.strokeStyle = isHovered ? palette.child : withAlpha(palette.labelText, 0.22);
                    ctx.lineWidth = 1.5;
                    ctx.stroke();

                    // Solid node core
                    ctx.beginPath();
                    ctx.arc(screenX, screenY, node.baseRadius, 0, Math.PI * 2);
                    ctx.fillStyle = coreColor;
                    ctx.fill();

                    // Inner bright core
                    ctx.beginPath();
                    ctx.arc(screenX, screenY, node.baseRadius * 0.45, 0, Math.PI * 2);
                    ctx.fillStyle = palette.nodeCore;
                    ctx.fill();

                    if (node.isCenter || isHovered || layout.showAllLabels) {
                        // Technical telemetry pills below nodes
                        ctx.font = "600 9px 'JetBrains Mono', monospace";
                        ctx.textAlign = "center";
                        ctx.textBaseline = "top";
                        var labelText = node.isCenter ? "OTBR (GATEWAY)" : "NODE: " + node.label;

                        var textWidth = ctx.measureText(labelText).width;
                        var boxW = textWidth + 12;
                        var boxH = 15;
                        var boxX = screenX - boxW / 2;
                        var boxY = screenY + node.baseRadius + 8;

                        ctx.fillStyle = palette.labelBg;
                        ctx.beginPath();
                        if (ctx.roundRect) {
                            ctx.roundRect(boxX, boxY, boxW, boxH, 5);
                        } else {
                            ctx.rect(boxX, boxY, boxW, boxH);
                        }
                        ctx.fill();

                        ctx.strokeStyle = isHovered ? withAlpha(palette.child, 0.5) : palette.labelBorder;
                        ctx.lineWidth = 1;
                        ctx.stroke();

                        ctx.fillStyle = isHovered ? palette.child : palette.labelText;
                        ctx.fillText(labelText, screenX, boxY + 3);
                    }
                });

                // Render tooltip
                if (hoveredNode) {
                    tooltip.style.display = "block";
                    tooltip.style.left = (rect.left + cx + hoveredNode.x + 15) + "px";
                    tooltip.style.top = (rect.top + cy + hoveredNode.y - 45) + "px";

                    if (hoveredNode.isCenter) {
                        tooltip.innerHTML = `
                            <strong style="color:var(--accent);">Thread Border Router (Gateway)</strong><br/>
                            <span style="color:var(--text-muted);">Extended MAC:</span> <span style="font-family:var(--font-mono);">${hoveredNode.mac}</span><br/>
                            <span style="color:var(--text-muted);">RLOC16:</span> <span style="font-family:var(--font-mono); color:var(--violet);">0x${hoveredNode.rloc}</span>
                        `;
                    } else {
                        var lqiLabel = hoveredNode.lqi === 3 ? "<span style='color:var(--success)'>EXCELLENT</span>" :
                                     hoveredNode.lqi === 2 ? "<span style='color:var(--warning)'>FAIR</span>" :
                                     "<span style='color:var(--danger)'>POOR</span>";
                        tooltip.innerHTML = `
                            <strong style="color:var(--accent);">Neighbor Node (Router)</strong><br/>
                            <span style="color:var(--text-muted);">Extended MAC:</span> <span style="font-family:var(--font-mono);">${hoveredNode.mac}</span><br/>
                            <span style="color:var(--text-muted);">RLOC16:</span> <span style="font-family:var(--font-mono); color:var(--violet);">${hoveredNode.rloc}</span><br/>
                            <span style="color:var(--text-muted);">Route Cost:</span> <span style="font-family:var(--font-mono);">${hoveredNode.pathCost || 0} hop(s) from OTBR</span><br/>
                            ${hoveredNode.nextHopRloc ? `<span style="color:var(--text-muted);">Next Hop:</span> <span style="font-family:var(--font-mono);">${hoveredNode.nextHopRloc}</span><br/>` : ""}
                            ${hoveredNode.role ? `<span style="color:var(--text-muted);">Role:</span> <span style="font-family:var(--font-mono);">${hoveredNode.role}</span><br/>` : ""}
                            <span style="color:var(--text-muted);">Link Strength:</span> ${lqiLabel} (LQI ${hoveredNode.lqi})
                        `;
                    }
                } else {
                    tooltip.style.display = "none";
                }

                requestAnimationFrame(draw);
            }

            requestAnimationFrame(draw);
        })();
