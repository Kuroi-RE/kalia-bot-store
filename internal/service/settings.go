package service

import (
	"context"
	"errors"
	"strings"

	"github.com/kalia/store/internal/model"
	"github.com/kalia/store/internal/repository"
	"github.com/kalia/store/pkg/apperr"
)

// SettingsService manages runtime key/value configuration.
type SettingsService struct {
	settings *repository.SettingRepository
}

// NewSettingsService builds a settings service.
func NewSettingsService(settings *repository.SettingRepository) *SettingsService {
	return &SettingsService{settings: settings}
}

// List returns all settings.
func (s *SettingsService) List(ctx context.Context) ([]model.Setting, error) {
	items, err := s.settings.List(ctx)
	if err != nil {
		return nil, apperr.Internal("could not list settings").Wrap(err)
	}
	return items, nil
}

// Get returns a single setting.
func (s *SettingsService) Get(ctx context.Context, key string) (*model.Setting, error) {
	setting, err := s.settings.Get(ctx, strings.TrimSpace(key))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("setting not found")
		}
		return nil, apperr.Internal("lookup failed").Wrap(err)
	}
	return setting, nil
}

// Set upserts a setting value.
func (s *SettingsService) Set(ctx context.Context, key, value string) (*model.Setting, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, apperr.BadRequest("key is required")
	}
	setting, err := s.settings.Upsert(ctx, key, value)
	if err != nil {
		return nil, apperr.Internal("could not update setting").Wrap(err)
	}
	return setting, nil
}
