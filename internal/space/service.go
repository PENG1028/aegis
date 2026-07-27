package space

import (
	"context"
	"fmt"
	"time"

	"aegis/internal/logs"
)

// AppService provides space management operations.
type AppService struct {
	repo   *Repository
	logSvc logs.Logger
}

// NewAppService creates a new space application service.
func NewAppService(repo *Repository, logSvc logs.Logger) *AppService {
	return &AppService{repo: repo, logSvc: logSvc}
}

// CreateSpace creates a new space.
func (s *AppService) CreateSpace(ctx context.Context, name string) (*Space, error) {
	sp := NewSpace(name)
	if err := s.repo.Create(sp); err != nil {
		return nil, fmt.Errorf("create space: %w", err)
	}
	s.logSvc.Log(ctx, "space.create", "space", sp.ID, "success",
		fmt.Sprintf("created space %s", name), "system")
	return sp, nil
}

// ListSpaces returns all spaces.
func (s *AppService) ListSpaces(ctx context.Context) ([]*Space, error) {
	spaces, err := s.repo.FindAll()
	if err != nil {
		return nil, fmt.Errorf("list spaces: %w", err)
	}
	if spaces == nil {
		spaces = []*Space{}
	}
	return spaces, nil
}

// GetSpace returns a space by internal ID.
func (s *AppService) GetSpace(ctx context.Context, id string) (*Space, error) {
	sp, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("get space: %w", err)
	}
	return sp, nil
}

// GetSpaceBySpaceID returns a space by its logical space_id.
func (s *AppService) GetSpaceBySpaceID(ctx context.Context, spaceID string) (*Space, error) {
	sp, err := s.repo.FindBySpaceID(spaceID)
	if err != nil {
		return nil, fmt.Errorf("get space by space_id: %w", err)
	}
	return sp, nil
}

// UpdateSpace updates a space's name and quotas.
func (s *AppService) UpdateSpace(ctx context.Context, id, name string, quotas Quota) (*Space, error) {
	sp, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("find space: %w", err)
	}
	if sp == nil {
		return nil, fmt.Errorf("space not found: %s", id)
	}
	if name != "" {
		sp.Name = name
	}
	sp.Quotas = quotas
	sp.UpdatedAt = time.Now()
	if err := s.repo.Update(sp); err != nil {
		return nil, fmt.Errorf("update space: %w", err)
	}
	s.logSvc.Log(ctx, "space.update", "space", sp.ID, "success",
		fmt.Sprintf("updated space %s", sp.Name), "system")
	return sp, nil
}

// DisableSpace disables a space.
func (s *AppService) DisableSpace(ctx context.Context, id string) (*Space, error) {
	sp, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("find space: %w", err)
	}
	if sp == nil {
		return nil, fmt.Errorf("space not found: %s", id)
	}
	sp.Status = "disabled"
	sp.UpdatedAt = time.Now()
	if err := s.repo.Update(sp); err != nil {
		return nil, fmt.Errorf("disable space: %w", err)
	}
	s.logSvc.Log(ctx, "space.disable", "space", sp.ID, "success",
		fmt.Sprintf("disabled space %s", sp.Name), "system")
	return sp, nil
}

// DeleteSpace removes a space entirely.
func (s *AppService) DeleteSpace(ctx context.Context, id string) error {
	sp, err := s.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("find space: %w", err)
	}
	if sp == nil {
		return fmt.Errorf("space not found: %s", id)
	}
	if err := s.repo.Delete(sp.ID); err != nil {
		return fmt.Errorf("delete space: %w", err)
	}
	s.logSvc.Log(ctx, "space.delete", "space", sp.ID, "success",
		fmt.Sprintf("deleted space %s", sp.Name), "system")
	return nil
}
