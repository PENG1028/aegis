package manageddomain

import "time"

// ManagedDomain is a domain managed by Aegis for external access.
// It is separate from Route and requires DNS verification before becoming active.
type ManagedDomain struct {
	ID                 string    `json:"id"`
	Domain             string    `json:"domain"`
	ServiceID          string    `json:"service_id"`
	OwnerRef           string    `json:"owner_ref"`
	TargetType         string    `json:"target_type"` // auth_page | service_page | hosting | custom
	TargetRef          string    `json:"target_ref"`
	VerificationType   string    `json:"verification_type"` // dns_txt | cname
	VerificationName   string    `json:"verification_name"`
	VerificationValue  string    `json:"verification_value"`
	Status             string    `json:"status"` // pending_verification | verified | active | failed | disabled
	TLSStatus          string    `json:"tls_status"` // pending | issued | failed | disabled
	LastCheckMessage   string    `json:"last_check_message"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// CreateManagedDomainInput is the input for creating a managed domain.
type CreateManagedDomainInput struct {
	Domain    string
	ServiceID string
	OwnerRef  string
	TargetType string
	TargetRef  string
}
