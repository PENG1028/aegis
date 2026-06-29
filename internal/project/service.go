package project

import (
	"context"
	"fmt"
	"time"

	"aegis/internal/id"
	"aegis/internal/logs"
)

// AppService defines the project application service interface.
type AppService struct {
	repo    *Repository
	logSvc  logs.Logger
}

// NewAppService creates a new project application service.
func NewAppService(repo *Repository, logSvc logs.Logger) *AppService {
	return &AppService{repo: repo, logSvc: logSvc}
}

// CreateProject creates a new project.
func (s *AppService) CreateProject(ctx context.Context, input CreateProjectInput) (*Project, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("project name is required")
	}

	// Check for duplicate name
	existing, err := s.repo.FindByName(input.Name)
	if err != nil {
		return nil, fmt.Errorf("check duplicate project name: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("project with name %q already exists", input.Name)
	}

	now := time.Now()
	p := &Project{
		ID:          id.New("proj"),
		Name:        input.Name,
		Description: input.Description,
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.Create(p); err != nil {
		s.logSvc.Log(ctx, "project.create", "project", p.ID, "failed", err.Error(), "cli")
		return nil, fmt.Errorf("create project: %w", err)
	}

	s.logSvc.Log(ctx, "project.create", "project", p.ID, "success",
		fmt.Sprintf("created project %q", p.Name), "cli")
	return p, nil
}

// ListProjects returns all projects.
func (s *AppService) ListProjects(ctx context.Context) ([]Project, error) {
	projects, err := s.repo.FindAll()
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	if projects == nil {
		projects = []Project{}
	}
	return projects, nil
}

// GetProject finds a project by ID or name.
func (s *AppService) GetProject(ctx context.Context, idOrName string) (*Project, error) {
	p, err := s.repo.FindByID(idOrName)
	if err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	if p != nil {
		return p, nil
	}

	p, err = s.repo.FindByName(idOrName)
	if err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	if p == nil {
		return nil, fmt.Errorf("project %q not found", idOrName)
	}
	return p, nil
}

// ArchiveProject archives a project.
func (s *AppService) ArchiveProject(ctx context.Context, idOrName string) error {
	p, err := s.GetProject(ctx, idOrName)
	if err != nil {
		return err
	}

	if p.Status == "archived" {
		return fmt.Errorf("project %q is already archived", p.Name)
	}

	p.Status = "archived"
	p.UpdatedAt = time.Now()

	if err := s.repo.Update(p); err != nil {
		s.logSvc.Log(ctx, "project.archive", "project", p.ID, "failed", err.Error(), "cli")
		return fmt.Errorf("archive project: %w", err)
	}

	s.logSvc.Log(ctx, "project.archive", "project", p.ID, "success",
		fmt.Sprintf("archived project %q", p.Name), "cli")
	return nil
}
