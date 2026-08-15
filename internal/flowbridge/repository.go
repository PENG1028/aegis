package flowbridge

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrReferenced is returned when deleting an instance that routes reference.
var ErrReferenced = errors.New("flowbridge instance is referenced by routes")

// Repository provides database access for flowbridge instances.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a flowbridge instance repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

const instanceCols = `id, name, machine_ip, data_plane_port, control_address, enabled,
	last_health_status, last_health_latency_ms, last_health_message, last_checked_at,
	space_id, owner_type, owner_id, created_by_token_id, created_at, updated_at`

func scanInstance(scanner interface{ Scan(...interface{}) error }) (*Instance, error) {
	var inst Instance
	var enabled, createdAt, updatedAt, lastCheckedAt string
	err := scanner.Scan(
		&inst.ID, &inst.Name, &inst.MachineIP, &inst.DataPlanePort, &inst.ControlAddress, &enabled,
		&inst.LastHealthStatus, &inst.LastHealthLatency, &inst.LastHealthMessage, &lastCheckedAt,
		&inst.SpaceID, &inst.OwnerType, &inst.OwnerID, &inst.CreatedByTokenID, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	inst.Enabled = enabled == "1"
	inst.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	inst.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	inst.LastCheckedAt, _ = time.Parse(time.RFC3339, lastCheckedAt)
	return &inst, nil
}

func scanInstances(rows *sql.Rows) ([]Instance, error) {
	var out []Instance
	for rows.Next() {
		inst, err := scanInstance(rows)
		if err != nil {
			return nil, fmt.Errorf("scan instance: %w", err)
		}
		out = append(out, *inst)
	}
	return out, rows.Err()
}

// Create inserts a new instance.
func (r *Repository) Create(inst *Instance) error {
	_, err := r.db.Exec(
		`INSERT INTO flowbridge_instances (id, name, machine_ip, data_plane_port, control_address, enabled,
			last_health_status, last_health_latency_ms, last_health_message, last_checked_at,
			space_id, owner_type, owner_id, created_by_token_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		inst.ID, inst.Name, inst.MachineIP, inst.DataPlanePort, inst.ControlAddress, boolInt(inst.Enabled),
		inst.LastHealthStatus, inst.LastHealthLatency, inst.LastHealthMessage, fmtTime(inst.LastCheckedAt),
		inst.SpaceID, inst.OwnerType, inst.OwnerID, inst.CreatedByTokenID,
		fmtTime(inst.CreatedAt), fmtTime(inst.UpdatedAt),
	)
	return err
}

// FindAll returns all instances ordered by creation time.
func (r *Repository) FindAll() ([]Instance, error) {
	rows, err := r.db.Query(`SELECT ` + instanceCols + ` FROM flowbridge_instances ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := scanInstances(rows)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []Instance{}
	}
	return out, nil
}

// FindByID returns an instance by ID, or nil.
func (r *Repository) FindByID(id string) (*Instance, error) {
	row := r.db.QueryRow(`SELECT `+instanceCols+` FROM flowbridge_instances WHERE id = ?`, id)
	inst, err := scanInstance(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return inst, err
}

// FindByIDs returns instances matching the given IDs.
func (r *Repository) FindByIDs(ids []string) (map[string]*Instance, error) {
	out := make(map[string]*Instance, len(ids))
	for _, id := range ids {
		inst, err := r.FindByID(id)
		if err != nil {
			return nil, err
		}
		if inst != nil {
			out[id] = inst
		}
	}
	return out, nil
}

// Update persists the mutable fields of an instance.
func (r *Repository) Update(inst *Instance) error {
	_, err := r.db.Exec(
		`UPDATE flowbridge_instances SET name=?, machine_ip=?, data_plane_port=?, control_address=?, enabled=?,
			last_health_status=?, last_health_latency_ms=?, last_health_message=?, last_checked_at=?,
			space_id=?, owner_type=?, owner_id=?, created_by_token_id=?, updated_at=? WHERE id=?`,
		inst.Name, inst.MachineIP, inst.DataPlanePort, inst.ControlAddress, boolInt(inst.Enabled),
		inst.LastHealthStatus, inst.LastHealthLatency, inst.LastHealthMessage, fmtTime(inst.LastCheckedAt),
		inst.SpaceID, inst.OwnerType, inst.OwnerID, inst.CreatedByTokenID,
		fmtTime(inst.UpdatedAt), inst.ID,
	)
	return err
}

// UpdateHealth persists only the health fields. The periodic checker must not
// write back a stale full-row snapshot — that would revert concurrent admin
// edits (e.g. re-enabling a just-disabled instance).
func (r *Repository) UpdateHealth(id, status string, latencyMS int64, message string, checkedAt time.Time) error {
	_, err := r.db.Exec(
		`UPDATE flowbridge_instances SET last_health_status=?, last_health_latency_ms=?,
			last_health_message=?, last_checked_at=? WHERE id=?`,
		status, latencyMS, message, fmtTime(checkedAt), id,
	)
	return err
}

// Delete removes an instance. Referenced instances are rejected by the
// FLOWBRIDGE_REFERENCED trigger.
func (r *Repository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM flowbridge_instances WHERE id = ?`, id)
	if err != nil && strings.Contains(err.Error(), "FLOWBRIDGE_REFERENCED") {
		return ErrReferenced
	}
	return err
}

// RoutesReferencing returns the number of routes pointing at an instance.
func (r *Repository) RoutesReferencing(id string) (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM routes WHERE flowbridge_id = ?`, id).Scan(&count)
	return count, err
}

func boolInt(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
