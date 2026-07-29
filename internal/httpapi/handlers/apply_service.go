package handlers

import (
	"context"

	"aegis/internal/apply"
	"aegis/internal/certstore"
)

// ApplyService is the apply surface the HTTP handlers use.
//
// WHY an interface rather than *apply.AppService: the concrete service drives real
// config rendering, service control, and gateway reloads, so every handler path
// behind a successful apply was unreachable in a test. The consequential branch
// behind that wall is the 202 on certificate renewal — certificate replaced on
// disk, provider reload failed, so the gateway still serves the old pair. An
// operator reading 200 there would believe TLS was updated when it was not.
//
// Declared in the consuming package, per Go convention. *apply.AppService
// satisfies it without knowing this type exists.
//
// Kept to exactly what the handlers call. RenewCertificateAndApply is here not
// because a handler calls it directly but because h.Apply is handed to
// certstore.CertRenewalChecker as its RenewalCoordinator; the compile-time
// assertion below pins that relationship so it cannot silently break.
type ApplyService interface {
	// DryRun renders configuration without applying it.
	DryRun(ctx context.Context) (*apply.ApplyPlan, error)

	// TryApply applies if the apply lock is free, otherwise marks pending.
	TryApply(ctx context.Context) (*apply.ApplyPlan, error)

	// Apply applies, waiting for the apply lock.
	Apply(ctx context.Context) (*apply.ApplyPlan, error)

	// ForceApply applies unconditionally. Used to install the ACME HTTP-01
	// challenge route before an order is placed.
	ForceApply(ctx context.Context) (*apply.ApplyPlan, error)

	// SwitchMode moves the runtime to targetModeID, rolling back on failure.
	SwitchMode(ctx context.Context, targetModeID string) error

	// History returns past apply versions, newest first.
	History(ctx context.Context) ([]apply.ApplyVersion, error)

	// GetCurrentConfig returns the config currently on disk.
	GetCurrentConfig() (string, error)

	// RenewCertificateAndApply holds the apply lock across PEM replacement and
	// provider reload so config validation never observes a mismatched pair.
	RenewCertificateAndApply(ctx context.Context, mutate func() error) error
}

// Compile-time proof that the real service satisfies both this interface and the
// certstore coordinator contract it is passed as. If either drifts, the build
// fails here rather than at wiring time.
var (
	_ ApplyService                 = (*apply.AppService)(nil)
	_ certstore.RenewalCoordinator = (ApplyService)(nil)
)
