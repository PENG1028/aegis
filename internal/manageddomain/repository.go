package manageddomain

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository provides database access for managed domains.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new managed domain repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// Create inserts a new managed domain.
func (r *Repository) Create(md *ManagedDomain) error {
	_, err := r.DB.Exec(
		`INSERT INTO managed_domains
		 (id, domain, service_id, owner_ref, target_type, target_ref,
		  verification_type, verification_name, verification_value,
		  status, tls_status, last_check_message, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		md.ID, md.Domain, md.ServiceID, md.OwnerRef, md.TargetType, md.TargetRef,
		md.VerificationType, md.VerificationName, md.VerificationValue,
		md.Status, md.TLSStatus, md.LastCheckMessage,
		md.CreatedAt.Format(time.RFC3339),
		md.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert managed_domain: %w", err)
	}
	return nil
}

// FindByID returns a managed domain by ID.
func (r *Repository) FindByID(id string) (*ManagedDomain, error) {
	var md ManagedDomain
	var createdAt, updatedAt string
	var targetType, targetRef, lastCheckMessage sql.NullString
	err := r.DB.QueryRow(
		`SELECT id, domain, service_id, owner_ref, target_type, target_ref,
		        verification_type, verification_name, verification_value,
		        status, tls_status, last_check_message, created_at, updated_at
		 FROM managed_domains WHERE id = ?`, id,
	).Scan(&md.ID, &md.Domain, &md.ServiceID, &md.OwnerRef, &targetType, &targetRef,
		&md.VerificationType, &md.VerificationName, &md.VerificationValue,
		&md.Status, &md.TLSStatus, &lastCheckMessage, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query managed_domain by id: %w", err)
	}
	md.TargetType = targetType.String
	md.TargetRef = targetRef.String
	md.LastCheckMessage = lastCheckMessage.String
	md.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	md.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &md, nil
}

// FindByDomain returns a managed domain by domain name.
func (r *Repository) FindByDomain(domain string) (*ManagedDomain, error) {
	var md ManagedDomain
	var createdAt, updatedAt string
	var targetType, targetRef, lastCheckMessage sql.NullString
	err := r.DB.QueryRow(
		`SELECT id, domain, service_id, owner_ref, target_type, target_ref,
		        verification_type, verification_name, verification_value,
		        status, tls_status, last_check_message, created_at, updated_at
		 FROM managed_domains WHERE domain = ?`, domain,
	).Scan(&md.ID, &md.Domain, &md.ServiceID, &md.OwnerRef, &targetType, &targetRef,
		&md.VerificationType, &md.VerificationName, &md.VerificationValue,
		&md.Status, &md.TLSStatus, &lastCheckMessage, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query managed_domain by domain: %w", err)
	}
	md.TargetType = targetType.String
	md.TargetRef = targetRef.String
	md.LastCheckMessage = lastCheckMessage.String
	md.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	md.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &md, nil
}

// FindAll returns all managed domains.
func (r *Repository) FindAll() ([]ManagedDomain, error) {
	rows, err := r.DB.Query(
		`SELECT id, domain, service_id, owner_ref, target_type, target_ref,
		        verification_type, verification_name, verification_value,
		        status, tls_status, last_check_message, created_at, updated_at
		 FROM managed_domains ORDER BY domain`)
	if err != nil {
		return nil, fmt.Errorf("query managed_domains: %w", err)
	}
	defer rows.Close()
	return scanManagedDomains(rows)
}

// FindActive returns all active managed domains (status = 'active').
func (r *Repository) FindActive() ([]ManagedDomain, error) {
	rows, err := r.DB.Query(
		`SELECT id, domain, service_id, owner_ref, target_type, target_ref,
		        verification_type, verification_name, verification_value,
		        status, tls_status, last_check_message, created_at, updated_at
		 FROM managed_domains WHERE status = 'active' ORDER BY domain`)
	if err != nil {
		return nil, fmt.Errorf("query active managed_domains: %w", err)
	}
	defer rows.Close()
	return scanManagedDomains(rows)
}

// Update updates a managed domain.
func (r *Repository) Update(md *ManagedDomain) error {
	_, err := r.DB.Exec(
		`UPDATE managed_domains SET
		 domain=?, service_id=?, owner_ref=?, target_type=?, target_ref=?,
		 verification_type=?, verification_name=?, verification_value=?,
		 status=?, tls_status=?, last_check_message=?, updated_at=?
		 WHERE id=?`,
		md.Domain, md.ServiceID, md.OwnerRef, md.TargetType, md.TargetRef,
		md.VerificationType, md.VerificationName, md.VerificationValue,
		md.Status, md.TLSStatus, md.LastCheckMessage,
		md.UpdatedAt.Format(time.RFC3339), md.ID,
	)
	if err != nil {
		return fmt.Errorf("update managed_domain: %w", err)
	}
	return nil
}

// Delete removes a managed domain by ID.
func (r *Repository) Delete(id string) error {
	_, err := r.DB.Exec(`DELETE FROM managed_domains WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete managed_domain: %w", err)
	}
	return nil
}

func scanManagedDomains(rows *sql.Rows) ([]ManagedDomain, error) {
	var domains []ManagedDomain
	for rows.Next() {
		var md ManagedDomain
		var createdAt, updatedAt string
		var targetType, targetRef, lastCheckMessage sql.NullString
		if err := rows.Scan(
			&md.ID, &md.Domain, &md.ServiceID, &md.OwnerRef, &targetType, &targetRef,
			&md.VerificationType, &md.VerificationName, &md.VerificationValue,
			&md.Status, &md.TLSStatus, &lastCheckMessage, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan managed_domain: %w", err)
		}
		md.TargetType = targetType.String
		md.TargetRef = targetRef.String
		md.LastCheckMessage = lastCheckMessage.String
		md.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		md.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		domains = append(domains, md)
	}
	return domains, rows.Err()
}
