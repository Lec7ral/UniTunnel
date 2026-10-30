//go:build windows

package tun

import (
	"strings"
	"time"

	"unitunnel/pkg/appstate"
)

const routesStateFile = "routes_state.json"

// routeSpec is one route exactly as UniTunnel added it.
//
// Cleanup must never be a blanket `route delete 10.0.0.0 mask 255.0.0.0`: on a
// corporate machine that also removes a route the user (or another VPN client)
// created, silently breaking their LAN. Only routes recorded here are deleted.
type routeSpec struct {
	Destination string `json:"destination"`
	Mask        string `json:"mask"`
	Gateway     string `json:"gateway"`
}

type routesState struct {
	CapturedAt string      `json:"captured_at"`
	Routes     []routeSpec `json:"routes"`
}

// add registers a route that Windows accepted and persists the state
// immediately, so a crash right after `route add` still leaves a record that
// the next startup can clean up.
func (s *routesState) add(spec routeSpec) error {
	for _, existing := range s.Routes {
		if existing == spec {
			return nil
		}
	}
	s.Routes = append(s.Routes, spec)
	return s.persist()
}

func (s *routesState) persist() error {
	if s.CapturedAt == "" {
		s.CapturedAt = time.Now().Format(time.RFC3339)
	}
	if len(s.Routes) == 0 {
		return appstate.Remove(routesStateFile)
	}
	return appstate.WriteJSON(routesStateFile, s)
}

// deleteAll removes every recorded route. Deleting a route that is already
// gone is a no-op at the OS level, so this is safe to call repeatedly.
func (s *routesState) deleteAll() {
	for _, r := range s.Routes {
		_ = runHidden("route", "delete", r.Destination, "mask", r.Mask, r.Gateway)
	}
}

// addRouteAndRecord adds a route and only records it when Windows accepted it,
// so Close can never delete a route UniTunnel did not create.
func addRouteAndRecord(state *routesState, dest, mask, gateway, metric string) error {
	if err := runHidden("route", "add", dest, "mask", mask, gateway, "metric", metric); err != nil {
		return err
	}
	_ = state.add(routeSpec{Destination: dest, Mask: mask, Gateway: gateway})
	return nil
}

// CleanStaleRoutes deletes only the routes registered by a previous UniTunnel
// session and then removes the state file. If there is no state file (clean
// machine, or the app never connected) nothing is touched.
//
// proxyHost/tunIP are kept for the existing diagnostics call sites; route
// ownership now comes from the state file instead of guessing by subnet.
func CleanStaleRoutes(proxyHost, tunIP string) {
	_ = proxyHost
	_ = tunIP

	var state routesState
	if err := appstate.ReadJSON(routesStateFile, &state); err != nil {
		return
	}
	state.deleteAll()
	_ = appstate.Remove(routesStateFile)
}

// pendingRouteState reports whether a previous session left routes behind.
func pendingRouteState() bool {
	exists, err := appstate.Exists(routesStateFile)
	return err == nil && exists
}

// hasRoute checks the IPv4 routing table for an exact destination/mask pair.
// When gateway is empty any next hop matches.
func hasRoute(dest, mask, gateway string) bool {
	out, err := outputHidden("route", "print", "-4")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[0] != dest || fields[1] != mask {
			continue
		}
		if gateway == "" || fields[2] == gateway {
			return true
		}
	}
	return false
}
