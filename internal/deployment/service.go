package deployment

import (
	"context"
	"fmt"
	"time"

	"aegis/internal/id"
)

// DeploymentLogger is an optional logging interface for the deployment service.
// Implemented by logs.AppService.
type DeploymentLogger interface {
	Log(ctx context.Context, action, targetType, targetID, result, message, actor string)
}

// Service provides deployment version tracking operations.
type Service struct {
	repo     *Repository
	instRepo *InstanceRepository
	logger   DeploymentLogger // v1.7R: optional operation logging
}

// NewService creates a new deployment service.
func NewService(repo *Repository, instRepo *InstanceRepository) *Service {
	return &Service{repo: repo, instRepo: instRepo}
}

// SetLogger sets an optional logger for operation tracking (v1.7R).
func (s *Service) SetLogger(l DeploymentLogger) {
	s.logger = l
}

// CreateDeployment creates a new deployment with per-node instance tracking.
func (s *Service) CreateDeployment(ctx context.Context, version, serviceID string, targetNodes []string, strategy string) (*Deployment, error) {
	if version == "" {
		version = fmt.Sprintf("v%s", time.Now().Format("20060102_150405"))
	}
	if strategy == "" {
		strategy = StrategyAll
	}
	now := time.Now()
	d := &Deployment{
		ID:              id.New("dep"),
		Version:         version,
		ServiceID:       serviceID,
		TargetNodes:     targetNodes,
		RolloutStrategy: strategy,
		Status:          StatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.repo.Create(d); err != nil {
		return nil, err
	}
	// Create per-node instances
	for _, nodeID := range targetNodes {
		inst := &DeploymentInstance{
			ID:           id.New("depi"),
			DeploymentID: d.ID,
			NodeID:       nodeID,
			Status:       StatusPending,
			CreatedAt:    now,
		}
		s.instRepo.Create(inst)
	}

	// v1.7R: Log deployment creation
	if s.logger != nil {
		s.logger.Log(ctx, "deployment.create", "deployment", d.ID, "success",
			fmt.Sprintf("deployment %s created for service %s on %d nodes (strategy=%s)",
				d.Version, serviceID, len(targetNodes), strategy), "admin")
	}

	return d, nil
}

// GetDeployment returns a deployment by ID with instances.
func (s *Service) GetDeployment(ctx context.Context, id string) (*Deployment, []DeploymentInstance, error) {
	d, err := s.repo.FindByID(id)
	if err != nil || d == nil {
		return nil, nil, fmt.Errorf("deployment %s not found", id)
	}
	instances, _ := s.instRepo.FindByDeploymentID(id)
	if instances == nil {
		instances = []DeploymentInstance{}
	}
	return d, instances, nil
}

// ListDeployments returns all deployments.
func (s *Service) ListDeployments(ctx context.Context) ([]Deployment, error) {
	ds, err := s.repo.FindAll()
	if err != nil { return nil, err }
	if ds == nil { ds = []Deployment{} }
	return ds, nil
}

// RollbackDeployment marks a deployment as rolled back.
func (s *Service) RollbackDeployment(ctx context.Context, id string) error {
	d, err := s.repo.FindByID(id)
	if err != nil || d == nil {
		return fmt.Errorf("deployment %s not found", id)
	}
	d.Status = StatusRolledBack
	d.UpdatedAt = time.Now()
	if err := s.repo.Update(d); err != nil {
		return err
	}
	// Mark all instances as rolled back
	instances, _ := s.instRepo.FindByDeploymentID(id)
	for _, inst := range instances {
		inst.Status = StatusRolledBack
		s.instRepo.Update(&inst)
	}

	// v1.7R: Log deployment rollback
	if s.logger != nil {
		s.logger.Log(ctx, "deployment.rollback", "deployment", id, "success",
			fmt.Sprintf("deployment %s (version %s) rolled back — %d instances marked rolled_back",
				id, d.Version, len(instances)), "admin")
	}

	return nil
}

// GetInstanceStatus returns per-node deployment status.
func (s *Service) GetInstanceStatus(ctx context.Context, deploymentID string) ([]DeploymentInstance, error) {
	instances, err := s.instRepo.FindByDeploymentID(deploymentID)
	if err != nil { return nil, err }
	if instances == nil { instances = []DeploymentInstance{} }
	return instances, nil
}

// DiffVersions returns a list of version differences between two deployments.
func DiffVersions(from, to string) string {
	return fmt.Sprintf("diff from %s to %s", from, to)
}
