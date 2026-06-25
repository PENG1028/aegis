package action

import "fmt"

// Standard error codes for v1.6 Action API.
const (
	ErrCodeDomainAlreadyOwned   = "DOMAIN_ALREADY_OWNED"
	ErrCodeScopeDenied          = "SCOPE_DENIED"
	ErrCodeTargetNotAllowed     = "TARGET_NOT_ALLOWED"
	ErrCodeProviderMissing      = "PROVIDER_MISSING"
	ErrCodeConfigValidateFailed = "CONFIG_VALIDATE_FAILED"
	ErrCodeApplyLocked          = "APPLY_LOCKED"
	ErrCodeReloadFailed         = "RELOAD_FAILED"
	ErrCodeRuntimeVerifyFailed  = "RUNTIME_VERIFY_FAILED"
	ErrCodeListenerConflict     = "LISTENER_CONFLICT"
	ErrCodeResourceNotFound     = "RESOURCE_NOT_FOUND"
	ErrCodeResourceNotOwned     = "RESOURCE_NOT_OWNED"
	ErrCodeQuotaExceeded        = "QUOTA_EXCEEDED"
)

// ActionError is a structured error with code, message, and optional details.
type ActionError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Error implements the error interface.
func (e *ActionError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewError creates a new ActionError.
func NewError(code, message string) *ActionError {
	return &ActionError{Code: code, Message: message}
}

// NewErrorWithDetails creates a new ActionError with details.
func NewErrorWithDetails(code, message, details string) *ActionError {
	return &ActionError{Code: code, Message: message, Details: details}
}

// Pre-defined error constructors.
func ErrDomainAlreadyOwned(domain, spaceID string) *ActionError {
	return &ActionError{
		Code:    ErrCodeDomainAlreadyOwned,
		Message: fmt.Sprintf("domain %s is already owned by another space", domain),
		Details: fmt.Sprintf("owner_space=%s", spaceID),
	}
}

func ErrScopeDenied(message string) *ActionError {
	return &ActionError{Code: ErrCodeScopeDenied, Message: message}
}

func ErrResourceNotFound(resource, id string) *ActionError {
	return &ActionError{
		Code:    ErrCodeResourceNotFound,
		Message: fmt.Sprintf("%s not found: %s", resource, id),
	}
}

func ErrResourceNotOwned(resource, id, spaceID string) *ActionError {
	return &ActionError{
		Code:    ErrCodeResourceNotOwned,
		Message: fmt.Sprintf("%s %s does not belong to space %s", resource, id, spaceID),
	}
}

func ErrApplyLocked() *ActionError {
	return &ActionError{
		Code:    ErrCodeApplyLocked,
		Message: "another apply is in progress, please retry",
	}
}

func ErrTargetNotAllowed(target string) *ActionError {
	return &ActionError{
		Code:    ErrCodeTargetNotAllowed,
		Message: fmt.Sprintf("target not allowed: %s", target),
	}
}

// IsActionError checks if an error is an ActionError and optionally matches a code.
func IsActionError(err error, code string) bool {
	ae, ok := err.(*ActionError)
	if !ok {
		return false
	}
	if code == "" {
		return true
	}
	return ae.Code == code
}
