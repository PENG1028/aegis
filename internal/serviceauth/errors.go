package serviceauth

import "errors"

// Sentinel errors returned by the serviceauth package.
var (
	// ErrNotInCluster is returned when a registration request originates
	// from an IP that is not recognised as part of the trusted cluster.
	ErrNotInCluster = errors.New("serviceauth: not in cluster")

	// ErrServiceNotFound is returned when a named service has not registered.
	ErrServiceNotFound = errors.New("serviceauth: service not found")

	// ErrServiceBlocked is returned when the caller or target service is blocked.
	ErrServiceBlocked = errors.New("serviceauth: service is blocked")

	// ErrAPIBlocked is returned when the specific API is blocked.
	ErrAPIBlocked = errors.New("serviceauth: api is blocked")

	// ErrTicketInvalid is returned when a ticket fails HMAC verification.
	ErrTicketInvalid = errors.New("serviceauth: invalid ticket")

	// ErrTicketExpired is returned when a ticket's expiry time has passed.
	ErrTicketExpired = errors.New("serviceauth: ticket expired")

	// ErrMissingTicket is returned when no X-Service-Ticket header is present.
	ErrMissingTicket = errors.New("serviceauth: missing service ticket")

	// ErrInvalidInput is returned when a registration request contains
	// invalid field values (empty names, reserved characters, excessive length).
	ErrInvalidInput = errors.New("serviceauth: invalid input")
)
