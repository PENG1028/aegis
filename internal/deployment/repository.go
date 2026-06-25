package deployment

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Repository provides database access for deployments.
type Repository struct{ DB *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{DB: db} }

func (r *Repository) Create(d *Deployment) error {
	nodesJSON, _ := json.Marshal(d.TargetNodes)
	_, err := r.DB.Exec(
		`INSERT INTO deployments (id, version, service_id, target_nodes, rollout_strategy, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.Version, d.ServiceID, string(nodesJSON), d.RolloutStrategy, d.Status,
		d.CreatedAt.Format(time.RFC3339), d.UpdatedAt.Format(time.RFC3339))
	if err != nil { return fmt.Errorf("insert deployment: %w", err) }
	return nil
}

func (r *Repository) FindAll() ([]Deployment, error) {
	rows, err := r.DB.Query(`SELECT id, version, service_id, target_nodes, rollout_strategy, status, created_at, updated_at FROM deployments ORDER BY created_at DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	return scanDeployments(rows)
}

func (r *Repository) FindByID(id string) (*Deployment, error) {
	var d Deployment
	var ca, ua, nodesJSON string
	err := r.DB.QueryRow(`SELECT id, version, service_id, target_nodes, rollout_strategy, status, created_at, updated_at FROM deployments WHERE id=?`, id).
		Scan(&d.ID, &d.Version, &d.ServiceID, &nodesJSON, &d.RolloutStrategy, &d.Status, &ca, &ua)
	if err != nil {
		if err == sql.ErrNoRows { return nil, nil }
		return nil, err
	}
	json.Unmarshal([]byte(nodesJSON), &d.TargetNodes)
	d.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	d.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
	return &d, nil
}

func (r *Repository) FindByServiceID(serviceID string) ([]Deployment, error) {
	rows, err := r.DB.Query(`SELECT id, version, service_id, target_nodes, rollout_strategy, status, created_at, updated_at FROM deployments WHERE service_id=? ORDER BY created_at DESC`, serviceID)
	if err != nil { return nil, err }
	defer rows.Close()
	return scanDeployments(rows)
}

func (r *Repository) Update(d *Deployment) error {
	nodesJSON, _ := json.Marshal(d.TargetNodes)
	_, err := r.DB.Exec(`UPDATE deployments SET version=?, service_id=?, target_nodes=?, rollout_strategy=?, status=?, updated_at=? WHERE id=?`,
		d.Version, d.ServiceID, string(nodesJSON), d.RolloutStrategy, d.Status, d.UpdatedAt.Format(time.RFC3339), d.ID)
	return err
}

func scanDeployments(rows *sql.Rows) ([]Deployment, error) {
	var ds []Deployment
	for rows.Next() {
		var d Deployment
		var ca, ua, nodesJSON string
		if err := rows.Scan(&d.ID, &d.Version, &d.ServiceID, &nodesJSON, &d.RolloutStrategy, &d.Status, &ca, &ua); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(nodesJSON), &d.TargetNodes)
		d.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		d.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
		ds = append(ds, d)
	}
	return ds, rows.Err()
}

// InstanceRepository provides database access for deployment instances.
type InstanceRepository struct{ DB *sql.DB }

func NewInstanceRepository(db *sql.DB) *InstanceRepository { return &InstanceRepository{DB: db} }

func (r *InstanceRepository) Create(inst *DeploymentInstance) error {
	appliedAt := ""
	if !inst.AppliedAt.IsZero() { appliedAt = inst.AppliedAt.Format(time.RFC3339) }
	_, err := r.DB.Exec(
		`INSERT INTO deployment_instances (id, deployment_id, node_id, status, last_applied_version, applied_at, error_message, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		inst.ID, inst.DeploymentID, inst.NodeID, inst.Status, inst.LastAppliedVersion, appliedAt, inst.ErrorMessage,
		inst.CreatedAt.Format(time.RFC3339))
	if err != nil { return fmt.Errorf("insert deployment_instance: %w", err) }
	return nil
}

func (r *InstanceRepository) FindByDeploymentID(depID string) ([]DeploymentInstance, error) {
	rows, err := r.DB.Query(`SELECT id, deployment_id, node_id, status, last_applied_version, applied_at, error_message, created_at FROM deployment_instances WHERE deployment_id=? ORDER BY node_id`, depID)
	if err != nil { return nil, err }
	defer rows.Close()
	return scanInstances(rows)
}

func (r *InstanceRepository) FindByNodeID(nodeID string) ([]DeploymentInstance, error) {
	rows, err := r.DB.Query(`SELECT id, deployment_id, node_id, status, last_applied_version, applied_at, error_message, created_at FROM deployment_instances WHERE node_id=? ORDER BY created_at DESC`, nodeID)
	if err != nil { return nil, err }
	defer rows.Close()
	return scanInstances(rows)
}

func (r *InstanceRepository) Update(inst *DeploymentInstance) error {
	appliedAt := ""
	if !inst.AppliedAt.IsZero() { appliedAt = inst.AppliedAt.Format(time.RFC3339) }
	_, err := r.DB.Exec(`UPDATE deployment_instances SET status=?, last_applied_version=?, applied_at=?, error_message=? WHERE id=?`,
		inst.Status, inst.LastAppliedVersion, appliedAt, inst.ErrorMessage, inst.ID)
	return err
}

func scanInstances(rows *sql.Rows) ([]DeploymentInstance, error) {
	var is []DeploymentInstance
	for rows.Next() {
		var inst DeploymentInstance
		var ca, aa string
		if err := rows.Scan(&inst.ID, &inst.DeploymentID, &inst.NodeID, &inst.Status, &inst.LastAppliedVersion, &aa, &inst.ErrorMessage, &ca); err != nil {
			return nil, err
		}
		inst.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		if aa != "" { inst.AppliedAt, _ = time.Parse(time.RFC3339, aa) }
		is = append(is, inst)
	}
	return is, rows.Err()
}
