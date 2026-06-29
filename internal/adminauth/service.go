package adminauth

import (
	"aegis/internal/id"
	"aegis/internal/logs"
	"fmt"
	"sync"
	"time"
)

// SessionTTL is the default session lifetime.
const SessionTTL = 24 * time.Hour

// Service provides admin authentication operations.
type Service struct {
	userRepo    *AdminUserRepository
	sessionRepo *AdminSessionRepository
}

// NewService creates a new admin auth service.
func NewService(userRepo *AdminUserRepository, sessionRepo *AdminSessionRepository) *Service {
	return &Service{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
	}
}

// LoginResult contains the result of a successful login.
type LoginResult struct {
	User         *AdminUser `json:"user"`
	SessionToken string     `json:"session_token"`
	ExpiresAt    time.Time  `json:"expires_at"`
}

// Rate limiting defaults (personal tool — conservative settings).
const (
	maxLoginAttempts  = 5
	loginWindow       = 1 * time.Minute
	loginLockDuration = 60 * time.Second
)

// loginRate tracks login attempts per IP.
type loginRate struct {
	attempts   int
	firstSeen  time.Time
	lockedUntil time.Time
}

var loginRates sync.Map // map[string]*loginRate

func checkLoginRate(ip string) error {
	now := time.Now()
	val, _ := loginRates.LoadOrStore(ip, &loginRate{firstSeen: now})
	lr := val.(*loginRate)

	// Check if locked out
	if now.Before(lr.lockedUntil) {
		return fmt.Errorf("too many login attempts; try again in %d seconds", int(time.Until(lr.lockedUntil).Seconds()))
	}

	// Reset window if expired
	if now.Sub(lr.firstSeen) > loginWindow {
		lr.attempts = 0
		lr.firstSeen = now
	}

	lr.attempts++
	if lr.attempts > maxLoginAttempts {
		lr.lockedUntil = now.Add(loginLockDuration)
		return fmt.Errorf("too many login attempts; locked for %v", loginLockDuration)
	}
	return nil
}

// Login authenticates an admin user and creates a session.
func (s *Service) Login(username, password, ip, userAgent string) (*LoginResult, error) {
	// Rate limit check
	if err := checkLoginRate(ip); err != nil {
		logAuditEvent("admin", "", "login_rate_limited", ip, userAgent, "admin_user", username, "failed", "RATE_LIMITED")
		return nil, err
	}

	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		logAuditEvent("admin", "", "login_failed", ip, userAgent, "admin_user", username, "failed", "DB_ERROR")
		return nil, fmt.Errorf("login failed")
	}
	if user == nil {
		logAuditEvent("admin", "", "login_failed", ip, userAgent, "admin_user", username, "failed", "USER_NOT_FOUND")
		return nil, fmt.Errorf("invalid username or password")
	}

	if !user.CheckPassword(password) {
		logAuditEvent("admin", user.ID, "login_failed", ip, userAgent, "admin_user", username, "failed", "BAD_PASSWORD")
		return nil, fmt.Errorf("invalid username or password")
	}

	// Successful login clears rate limit state
	loginRates.Delete(ip)

	// Generate session token
	sessionToken := generateToken(32)
	session := NewAdminSession(user.ID, sessionToken, SessionTTL)
	if err := s.sessionRepo.Create(session); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	logAuditEvent("admin", user.ID, "login_success", ip, userAgent, "admin_session", session.ID, "success", "")

	return &LoginResult{
		User:         user,
		SessionToken: sessionToken,
		ExpiresAt:    session.ExpiresAt,
	}, nil
}

// Logout revokes the current session.
func (s *Service) Logout(sessionHash, ip, userAgent string) error {
	session, err := s.sessionRepo.FindBySessionHash(sessionHash)
	if err != nil || session == nil {
		return fmt.Errorf("session not found")
	}
	if err := s.sessionRepo.Revoke(sessionHash); err != nil {
		return err
	}
	logAuditEvent("admin", session.UserID, "logout", ip, userAgent, "admin_session", session.ID, "success", "")
	return nil
}

// GetMe returns the user for a valid session.
func (s *Service) GetMe(sessionHash string) (*AdminUser, error) {
	session, err := s.sessionRepo.FindBySessionHash(sessionHash)
	if err != nil || session == nil {
		return nil, fmt.Errorf("session not found")
	}
	if session.IsExpired() {
		return nil, fmt.Errorf("session expired")
	}
	if session.IsRevoked() {
		return nil, fmt.Errorf("session revoked")
	}
	// Update last seen
	_ = s.sessionRepo.TouchLastSeen(sessionHash)
	return s.userRepo.FindByID(session.UserID)
}

// ValidateSession checks if a session is valid and returns the user.
func (s *Service) ValidateSession(sessionHash string) (*AdminUser, error) {
	return s.GetMe(sessionHash)
}

// RevokeSession revokes a session by hash.
func (s *Service) RevokeSession(sessionHash string) error {
	return s.sessionRepo.Revoke(sessionHash)
}

// EnsureAdmin creates the default admin user if no admin exists.
func (s *Service) EnsureAdmin(username, password string) (*AdminUser, error) {
	count, err := s.userRepo.Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, fmt.Errorf("admin user already exists")
	}
	user, err := NewAdminUser(username, password)
	if err != nil {
		return nil, err
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}
	return user, nil
}

// AuditLogger is an optional interface for audit logging.

// logAudit writes an audit log entry if a logger is configured.
// This is a no-op until wired in main.go.
var auditLogger logs.AuditLogger

// SetAuditLogger configures the audit logger for admin operations.
func SetAuditLogger(l logs.AuditLogger) {
	auditLogger = l
}

func logAuditEvent(actorType, actorID, eventType, ip, userAgent, targetType, targetID, result, errorCode string) {
	if auditLogger != nil {
		auditLogger.LogAudit(actorType, actorID, eventType, ip, userAgent, targetType, targetID, result, errorCode)
	}
}

// generateToken creates a cryptographically random hex token.
// Delegates to id.GenerateRandomHex — the project's canonical random-hex generator.
func generateToken(bytes int) string {
	return id.GenerateRandomHex(bytes)
}
