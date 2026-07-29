package provider

import (
	"fmt"
	"net"
	"time"
)

// PortProbe reports whether something is accepting TCP connections on a local
// port. Injected rather than called directly so detection stays testable and so
// no code path pays for socket I/O unless it asked for evidence.
type PortProbe func(port int) bool

// DialPortProbe is the production PortProbe. A refused connection means nothing
// is listening; any other outcome (accepted, or timed out with a filtered
// socket) counts as occupied, because the question being asked is "is this port
// free for the target mode to bind", and a filtered port is not free.
func DialPortProbe(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 700*time.Millisecond)
	if err == nil {
		conn.Close()
		return true
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return true
	}
	return false
}

// PortMismatch is one disagreement between what a mode expects to own and what
// is actually listening.
type PortMismatch struct {
	ProviderID string `json:"provider_id"`
	Port       int    `json:"port"`
	Expected   bool   `json:"expected_listening"`
	Actual     bool   `json:"actual_listening"`
	Detail     string `json:"detail"`
}

// ModeEvidence carries the config-side check that DetectRuntimeMode cannot do.
//
// DetectRuntimeMode decides purely from process liveness: a provider that is
// installed, running, and error-free counts as participating, and the mode with
// the most participating providers wins. It never asks whether the ports that
// mode implies are the ports actually bound. So a haproxy left running by
// systemd after a switch back to legacy — or started by hand with an unrelated
// config — makes detection report edge_mux, and every subsequent plan is built
// for a mode the host is not really in.
//
// SwitchMode re-runs detection at the end, so the switch itself is guarded. The
// gap is afterwards: DetectDrift compares route sets only and never checks mode
// identity, so nothing notices.
//
// Consistent is false when evidence contradicts the liveness verdict. Nothing
// acts on it automatically — restarting a gateway to force agreement is far more
// dangerous than reporting the conflict and letting an operator decide.
type ModeEvidence struct {
	Mode       string         `json:"mode"`
	Consistent bool           `json:"consistent"`
	Probed     bool           `json:"probed"`
	Mismatches []PortMismatch `json:"mismatches,omitempty"`
}

// Summary renders a one-line operator-facing description, empty when consistent.
func (e ModeEvidence) Summary() string {
	if e.Consistent || len(e.Mismatches) == 0 {
		return ""
	}
	first := e.Mismatches[0]
	extra := ""
	if len(e.Mismatches) > 1 {
		extra = fmt.Sprintf("（另有 %d 处）", len(e.Mismatches)-1)
	}
	return fmt.Sprintf("检测为 %s 模式，但 %s%s", e.Mode, first.Detail, extra)
}

// DetectRuntimeModeWithEvidence returns the same verdict as DetectRuntimeMode,
// plus a port-level cross-check of that verdict.
//
// Deliberately separate from DetectRuntimeMode: that function has ten callers
// including the planner and the TLS lifecycle service, several on hot paths, and
// it must stay a pure function over states. This one performs I/O and is meant
// for the places a human reads the answer or a decision is gated on it.
//
// A nil probe yields Probed=false and Consistent=true — no evidence is not
// counter-evidence.
func DetectRuntimeModeWithEvidence(states []ProviderState, probe PortProbe) (RuntimeMode, ModeEvidence) {
	mode := DetectRuntimeMode(states)
	ev := ModeEvidence{Mode: mode.ID, Consistent: true}
	if probe == nil {
		return mode, ev
	}
	ev.Probed = true

	// Ports the detected mode claims must be bound.
	for _, pa := range mode.Providers {
		for atom, slots := range pa.Bindings {
			for _, slot := range slots {
				if slot.Port <= 0 {
					continue
				}
				if !probe(slot.Port) {
					ev.Consistent = false
					ev.Mismatches = append(ev.Mismatches, PortMismatch{
						ProviderID: pa.ProviderID, Port: slot.Port,
						Expected: true, Actual: false,
						Detail: fmt.Sprintf("%s 应占用的 :%d(%s) 上没有监听", pa.ProviderID, slot.Port, atom),
					})
				}
			}
		}
	}

	// Ports that only some other implemented mode claims. Something listening
	// there while the detected mode does not want it is the systemd-leftover
	// signature.
	for _, other := range AllRuntimeModes() {
		if !other.Implemented || other.ID == mode.ID {
			continue
		}
		for _, pa := range other.Providers {
			for atom, slots := range pa.Bindings {
				for _, slot := range slots {
					if slot.Port <= 0 || modeClaimsPort(mode, slot.Port) {
						continue
					}
					if probe(slot.Port) {
						ev.Consistent = false
						ev.Mismatches = append(ev.Mismatches, PortMismatch{
							ProviderID: pa.ProviderID, Port: slot.Port,
							Expected: false, Actual: true,
							Detail: fmt.Sprintf(":%d 上有监听，但当前模式不需要它（%s 模式的 %s/%s）",
								slot.Port, other.ID, pa.ProviderID, atom),
						})
					}
				}
			}
		}
	}
	return mode, ev
}

// modeClaimsPort reports whether any provider in the mode binds this port.
func modeClaimsPort(m RuntimeMode, port int) bool {
	for _, pa := range m.Providers {
		for _, slots := range pa.Bindings {
			for _, slot := range slots {
				if slot.Port == port {
					return true
				}
			}
		}
	}
	return false
}
