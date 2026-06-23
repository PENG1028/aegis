package listener

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository provides database access for listeners.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new listener repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// Create inserts a new listener.
func (r *Repository) Create(l *Listener) error {
	_, err := r.DB.Exec(
		`INSERT INTO listeners (id, provider, protocol, bind_ip, port, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		l.ID, l.Provider, l.Protocol, l.BindIP, l.Port, l.Status,
		l.CreatedAt.Format(time.RFC3339),
		l.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert listener: %w", err)
	}
	return nil
}

// FindAll returns all listeners.
func (r *Repository) FindAll() ([]Listener, error) {
	rows, err := r.DB.Query(
		`SELECT id, provider, protocol, bind_ip, port, status, created_at, updated_at
		 FROM listeners ORDER BY port`)
	if err != nil {
		return nil, fmt.Errorf("query listeners: %w", err)
	}
	defer rows.Close()
	return scanListeners(rows)
}

// FindByBind checks for an existing listener on a bind_ip+port combination.
// Returns the conflicting listener or nil.
func (r *Repository) FindByBind(bindIP string, port int) (*Listener, error) {
	// Check for exact match or 0.0.0.0 wildcard overlap
	rows, err := r.DB.Query(
		`SELECT id, provider, protocol, bind_ip, port, status, created_at, updated_at
		 FROM listeners WHERE port = ? AND (bind_ip = ? OR bind_ip = '0.0.0.0' OR ? = '0.0.0.0')`,
		port, bindIP, bindIP)
	if err != nil {
		return nil, fmt.Errorf("query listener by bind: %w", err)
	}
	defer rows.Close()

	listeners, err := scanListeners(rows)
	if err != nil {
		return nil, err
	}
	if len(listeners) > 0 {
		return &listeners[0], nil
	}
	return nil, nil
}

// FindByProvider returns all listeners for a provider.
func (r *Repository) FindByProvider(provider string) ([]Listener, error) {
	rows, err := r.DB.Query(
		`SELECT id, provider, protocol, bind_ip, port, status, created_at, updated_at
		 FROM listeners WHERE provider = ? ORDER BY port`, provider)
	if err != nil {
		return nil, fmt.Errorf("query listeners by provider: %w", err)
	}
	defer rows.Close()
	return scanListeners(rows)
}

// Delete removes a listener.
func (r *Repository) Delete(id string) error {
	_, err := r.DB.Exec(`DELETE FROM listeners WHERE id = ?`, id)
	return err
}

func scanListeners(rows *sql.Rows) ([]Listener, error) {
	var listeners []Listener
	for rows.Next() {
		var l Listener
		var createdAt, updatedAt string
		if err := rows.Scan(&l.ID, &l.Provider, &l.Protocol, &l.BindIP, &l.Port, &l.Status, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan listener: %w", err)
		}
		l.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		l.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		listeners = append(listeners, l)
	}
	return listeners, rows.Err()
}
