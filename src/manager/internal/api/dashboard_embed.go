package api

import (
	_ "embed"
)

//go:embed dashboard.html
var dashboardHTML string

//go:embed dashboard.css
var dashboardCSS string

//go:embed dashboard_topology.js
var dashboardTopologyJS string
