package manageddomain

import "time"

// ManagedDomain is a domain managed by Aegis for external access.
// It is separate from Route and requires DNS verification before becoming active.
type ManagedDomain struct {
	ID                string `json:"id"`
	Domain            string `json:"domain"`
	ServiceID         string `json:"service_id"`
	OwnerRef          string `json:"owner_ref"`
	TargetType        string `json:"target_type"` // auth_page | service_page | hosting | custom
	TargetRef         string `json:"target_ref"`
	VerificationType  string `json:"verification_type"` // dns_txt | cname
	VerificationName  string `json:"verification_name"`
	VerificationValue string `json:"verification_value"`
	Status            string `json:"status"` // pending_verification | verified | active | failed | disabled

	// TLSStatus is a reserved placeholder, NOT live certificate state.
	//
	// It is set to TLSStatusNotRequested at creation and never advances: nothing
	// in this package — or anywhere else — issues certificates for a managed
	// domain. Managed domains carry DNS ownership verification only; certificate
	// lifecycle lives in internal/certstore and is bound per Route, and Route has
	// no reference to ManagedDomain. The two subsystems are independent today.
	//
	// Do not read this field to decide whether a domain has TLS. To wire it, an
	// issuance path must first exist (see docs/design/capability-onboarding.md);
	// then advance it alongside that path and drop this comment.
	TLSStatus        string    `json:"tls_status"`
	LastCheckMessage string    `json:"last_check_message"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TLSStatusNotRequested is the only value ManagedDomain.TLSStatus ever holds.
//
// Named for what is true rather than "pending", which implied an issuance
// request was in flight and would eventually resolve. No such request exists.
const TLSStatusNotRequested = "not_requested"

// CreateManagedDomainInput is the input for creating a managed domain.
type CreateManagedDomainInput struct {
	Domain     string
	ServiceID  string
	OwnerRef   string
	TargetType string
	TargetRef  string
}
