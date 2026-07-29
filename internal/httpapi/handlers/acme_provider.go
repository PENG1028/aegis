package handlers

import (
	"context"

	"aegis/internal/acme"
)

// ACMEProvider is the ACME surface the HTTP handlers actually use.
//
// WHY an interface rather than *acme.Client: the concrete client talks to a real
// ACME directory, so no test could reach any handler path that requires issuance
// or renewal to succeed. The consequential branch behind that wall is the 202 on
// renewal — certificate replaced on disk, provider reload failed, so the gateway
// is still serving the old pair. An operator reading 200 there would believe TLS
// was updated when it was not, and nothing downstream would contradict them.
//
// Declared in the consuming package, per Go convention: the handlers decide what
// they need, and *acme.Client satisfies it without knowing this type exists.
//
// Kept to exactly the methods the handlers call. HasEmail is deliberately absent —
// it exists on the client but no handler uses it, and a method on an interface is
// a promise every future implementation has to keep.
type ACMEProvider interface {
	// Available reports whether ACME is configured enough to attempt an order.
	Available() bool

	// Obtain issues a new certificate for domains and stores it.
	Obtain(ctx context.Context, domains []string) (*acme.ObtainResult, error)

	// RenewCertificate re-issues certID and returns the new certificate's ID.
	// Signature matches certstore.ACMERenewer so the client satisfies both.
	RenewCertificate(ctx context.Context, certID string, domains []string) (string, error)

	// HTTPChallengeResponse returns the key authorization for an HTTP-01 token.
	HTTPChallengeResponse(token string) (string, bool)

	// UpdateEmail re-registers the ACME account under a new contact address.
	UpdateEmail(ctx context.Context, email string) error
}
