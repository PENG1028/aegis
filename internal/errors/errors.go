package errors

import "fmt"

// ErrorCode is a machine-readable error identifier.
type ErrorCode string

const (
	CodeBadRequest             ErrorCode = "BAD_REQUEST"
	CodeUnauthorized           ErrorCode = "UNAUTHORIZED"
	CodeForbidden              ErrorCode = "FORBIDDEN"
	CodeResourceNotFound       ErrorCode = "RESOURCE_NOT_FOUND"
	CodeConflict               ErrorCode = "CONFLICT"
	CodeValidationFailed       ErrorCode = "VALIDATION_FAILED"
	CodeApplyFailed            ErrorCode = "APPLY_FAILED"
	CodeProxyValidateFailed    ErrorCode = "PROXY_VALIDATE_FAILED"
	CodeProxyReloadFailed      ErrorCode = "PROXY_RELOAD_FAILED"
	CodeNoAvailableEndpoint    ErrorCode = "NO_AVAILABLE_ENDPOINT"
	CodeDNSVerifyFailed        ErrorCode = "DNS_VERIFY_FAILED"
	CodeInternalError          ErrorCode = "INTERNAL_ERROR"
	CodeServiceDisabled        ErrorCode = "SERVICE_DISABLED"
	CodeManagedDomainNotActive ErrorCode = "MANAGED_DOMAIN_NOT_ACTIVE"
	CodeRouteSkipped           ErrorCode = "ROUTE_SKIPPED"
	CodeEndpointUnreachable    ErrorCode = "ENDPOINT_UNREACHABLE"
	CodeStateTransitionInvalid ErrorCode = "STATE_TRANSITION_INVALID"
)

// APIError is a structured error for HTTP API responses.
type APIError struct {
	Code    ErrorCode              `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// New creates a new APIError.
func New(code ErrorCode, message string) *APIError {
	return &APIError{Code: code, Message: message}
}

// WithDetails adds details to the error.
func (e *APIError) WithDetails(key string, val interface{}) *APIError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = val
	return e
}

// ToResponse converts to the standard error response format.
func (e *APIError) ToResponse() map[string]interface{} {
	resp := map[string]interface{}{
		"code":    e.Code,
		"message": e.Message,
	}
	if e.Details != nil && len(e.Details) > 0 {
		resp["details"] = e.Details
	}
	return map[string]interface{}{"error": resp}
}

// Helper constructors

func BadRequest(msg string) *APIError {
	return New(CodeBadRequest, msg)
}

func NotFound(msg string) *APIError {
	return New(CodeResourceNotFound, msg)
}

func Conflict(msg string) *APIError {
	return New(CodeConflict, msg)
}

func Internal(msg string) *APIError {
	return New(CodeInternalError, msg)
}

func Forbidden(msg string) *APIError {
	return New(CodeForbidden, msg)
}

func Unauthorized(msg string) *APIError {
	return New(CodeUnauthorized, msg)
}

func ValidationFailed(msg string) *APIError {
	return New(CodeValidationFailed, msg)
}

func ApplyFailed(msg string) *APIError {
	return New(CodeApplyFailed, msg)
}

func NoAvailableEndpoint(msg string) *APIError {
	return New(CodeNoAvailableEndpoint, msg)
}

func StateTransitionInvalid(msg string) *APIError {
	return New(CodeStateTransitionInvalid, msg)
}
