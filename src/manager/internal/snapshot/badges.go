// Package snapshot builds dashboard topology views and status badges from live data.
package snapshot

import (
	"fmt"
	"strings"

	"github.com/MikeO7/threadgate/src/manager/internal/hass"
	"github.com/MikeO7/threadgate/src/manager/internal/radio"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
	"github.com/MikeO7/threadgate/src/manager/internal/topology"
)

// StatusBadge describes a header pill label, color variant, and hover tooltip.
type StatusBadge struct {
	Label   string
	Variant string
	Title   string
	Tooltip string
}

const (
	badgeVariantSuccess  = "success"
	badgeVariantWarning  = "warning"
	badgeVariantDanger   = "danger"
	badgeVariantMuted    = "muted"
	badgeVariantCyan     = "cyan"
	badgeVariantSecondary = "secondary"

	radioBadgeHelp  = "USB Thread dongle and border router (otbr-agent) health."
	threadBadgeHelp = "Thread mesh network role and partition status."
	hassBadgeHelp   = "Home Assistant integration for friendly device names on the topology map."

	threadStateLeader   = "leader"
	threadStateRouter   = "router"
	threadStateRouterP  = "router+"
	threadStateChild    = "child"
	threadStateDetached = "detached"
	threadStateDisabled = "disabled"
	threadStateOffline  = "offline"
)

func newStatusBadge(label, variant, help, title string) StatusBadge {
	tooltip := help
	if title != "" {
		tooltip = help + " " + title
	}
	return StatusBadge{Label: label, Variant: variant, Title: title, Tooltip: tooltip}
}

// RadioBadge derives the header radio status from runtime health and mock mode.
func RadioBadge(status runtime.Status, mockMode bool) StatusBadge {
	report := radio.ReadinessReport{
		RadioPath:  status.RadioPath,
		ProbeError: status.ProbeError,
		Version:    status.ProbedVersion,
		AgentState: status.Agent.State,
		AgentError: status.Agent.LastError,
	}
	label, variant, title := radio.BadgeFromReport(report, mockMode)
	return newStatusBadge(label, variant, radioBadgeHelp, title)
}

// ThreadBadge derives the header Thread mesh status from the topology snapshot.
func ThreadBadge(snap topology.Snapshot) StatusBadge {
	state := strings.ToLower(strings.TrimSpace(snap.State))
	title := threadBadgeTitle(snap, state)

	switch state {
	case threadStateLeader:
		return threadLeaderBadge(snap, title)
	case threadStateRouter, threadStateRouterP:
		return newStatusBadge("Thread Router", badgeVariantSuccess, threadBadgeHelp, title)
	case threadStateChild:
		return newStatusBadge("Thread Child", badgeVariantSuccess, threadBadgeHelp, title)
	case threadStateDetached:
		return newStatusBadge("Thread Detached", badgeVariantWarning, threadBadgeHelp, title)
	case threadStateDisabled, threadStateOffline:
		return newStatusBadge("Thread Offline", badgeVariantDanger, threadBadgeHelp, title)
	case "":
		return threadUnknownBadge(snap)
	default:
		return newStatusBadge("Thread "+displayThreadState(state), badgeVariantMuted, threadBadgeHelp, title)
	}
}

func threadLeaderBadge(snap topology.Snapshot, title string) StatusBadge {
	if len(snap.Neighbors) == 0 {
		return newStatusBadge("Idle Leader", badgeVariantWarning, threadBadgeHelp, title)
	}
	return newStatusBadge("Thread Leader", badgeVariantSuccess, threadBadgeHelp, title)
}

func threadUnknownBadge(snap topology.Snapshot) StatusBadge {
	if len(snap.Warnings) > 0 {
		return newStatusBadge("Thread Unknown", badgeVariantMuted, threadBadgeHelp, snap.Warnings[0])
	}
	return newStatusBadge("Thread Starting", badgeVariantMuted, threadBadgeHelp, "Waiting for Thread network state from the border router.")
}

func threadBadgeTitle(snap topology.Snapshot, state string) string {
	parts := make([]string, 0, 4)
	if snap.NetworkName != "" {
		parts = append(parts, fmt.Sprintf("Network: %s", snap.NetworkName))
	}
	if snap.LeaderData.PartitionID != 0 {
		parts = append(parts, fmt.Sprintf("Partition ID: %d", snap.LeaderData.PartitionID))
	}
	if state == threadStateDetached {
		parts = append(parts, "Not attached to an active Thread partition.")
	}
	if state == threadStateDisabled || state == threadStateOffline {
		parts = append(parts, "Thread interface is down or not started.")
	}
	neighborCount := len(snap.Neighbors)
	switch {
	case neighborCount > 0:
		parts = append(parts, fmt.Sprintf("%d device(s) visible in the mesh.", neighborCount))
	case state == threadStateLeader:
		parts = append(parts, "No child devices connected yet.")
	}
	if len(parts) == 0 {
		return "Thread mesh status from the border router."
	}
	return strings.Join(parts, " ")
}

func displayThreadState(state string) string {
	if state == "" {
		return ""
	}
	return strings.ToUpper(state[:1]) + strings.ToLower(state[1:])
}

// HassBadge derives the header Home Assistant status pill.
func HassBadge(hassStatus, hassError string) StatusBadge {
	switch hassStatus {
	case hass.StatusPending:
		return newStatusBadge("Approve HASS Link", badgeVariantWarning, hassBadgeHelp, "Home Assistant is waiting for you to approve the connection banner above.")
	case hass.StatusMock:
		return newStatusBadge("HASS Mock Data", badgeVariantMuted, hassBadgeHelp, "Showing simulated device names until you approve a real Home Assistant link.")
	case hass.StatusConnected:
		return newStatusBadge("HASS Connected", badgeVariantCyan, hassBadgeHelp, "Home Assistant name sync is active.")
	case hass.StatusFailed:
		title := "Home Assistant connection failed."
		if hassError != "" {
			title += " " + hassError
		}
		return newStatusBadge("HASS Failed", badgeVariantDanger, hassBadgeHelp, title+" Click to troubleshoot.")
	default:
		return newStatusBadge("HASS Not Configured", badgeVariantMuted, hassBadgeHelp, "Home Assistant integration is not configured. Click to set up.")
	}
}
