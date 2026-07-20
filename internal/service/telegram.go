package service

import (
	"context"
	"errors"
	"strings"

	"github.com/kalia/store/internal/model"
	"github.com/kalia/store/internal/repository"
	"github.com/kalia/store/pkg/apperr"
)

// TelegramService manages dynamic bot menus and static responses.
type TelegramService struct {
	menus     *repository.MenuRepository
	responses *repository.ResponseRepository
}

// NewTelegramService builds a telegram content service.
func NewTelegramService(menus *repository.MenuRepository, responses *repository.ResponseRepository) *TelegramService {
	return &TelegramService{menus: menus, responses: responses}
}

// normalizeCommand trims and ensures a single leading slash, lowercased.
func normalizeCommand(cmd string) (string, error) {
	c := strings.TrimSpace(strings.ToLower(cmd))
	if c == "" {
		return "", apperr.BadRequest("command is required")
	}
	if !strings.HasPrefix(c, "/") {
		c = "/" + c
	}
	if strings.ContainsAny(c[1:], " \t\n/") {
		return "", apperr.BadRequest("command must be a single token, e.g. /order")
	}
	return c, nil
}

// ---- Menus ----

// MenuInput carries menu create/update fields.
type MenuInput struct {
	Command   string
	Title     string
	ReplyText string
	IsEnabled bool
	SortOrder int
}

// CreateMenu creates a menu.
func (s *TelegramService) CreateMenu(ctx context.Context, in MenuInput) (*model.TelegramMenu, error) {
	cmd, err := normalizeCommand(in.Command)
	if err != nil {
		return nil, err
	}
	m, err := s.menus.Create(ctx, &model.TelegramMenu{
		Command:   cmd,
		Title:     in.Title,
		ReplyText: in.ReplyText,
		IsEnabled: in.IsEnabled,
		SortOrder: in.SortOrder,
	})
	if err != nil {
		if repository.IsUniqueViolation(err) {
			return nil, apperr.Conflict("a menu with this command already exists")
		}
		return nil, apperr.Internal("could not create menu").Wrap(err)
	}
	return m, nil
}

// ListMenus returns all menus (admin) or enabled only (bot).
func (s *TelegramService) ListMenus(ctx context.Context, enabledOnly bool) ([]model.TelegramMenu, error) {
	items, err := s.menus.List(ctx, enabledOnly)
	if err != nil {
		return nil, apperr.Internal("could not list menus").Wrap(err)
	}
	return items, nil
}

// UpdateMenu updates a menu.
func (s *TelegramService) UpdateMenu(ctx context.Context, id int64, in MenuInput) (*model.TelegramMenu, error) {
	existing, err := s.getMenu(ctx, id)
	if err != nil {
		return nil, err
	}
	cmd, err := normalizeCommand(in.Command)
	if err != nil {
		return nil, err
	}
	existing.Command = cmd
	existing.Title = in.Title
	existing.ReplyText = in.ReplyText
	existing.SortOrder = in.SortOrder
	m, err := s.menus.Update(ctx, existing)
	if err != nil {
		if repository.IsUniqueViolation(err) {
			return nil, apperr.Conflict("a menu with this command already exists")
		}
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("menu not found")
		}
		return nil, apperr.Internal("could not update menu").Wrap(err)
	}
	return m, nil
}

// SetMenuEnabled toggles a menu.
func (s *TelegramService) SetMenuEnabled(ctx context.Context, id int64, enabled bool) (*model.TelegramMenu, error) {
	m, err := s.menus.SetEnabled(ctx, id, enabled)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("menu not found")
		}
		return nil, apperr.Internal("could not update menu").Wrap(err)
	}
	return m, nil
}

// DeleteMenu removes a menu.
func (s *TelegramService) DeleteMenu(ctx context.Context, id int64) error {
	if err := s.menus.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return apperr.NotFound("menu not found")
		}
		return apperr.Internal("could not delete menu").Wrap(err)
	}
	return nil
}

func (s *TelegramService) getMenu(ctx context.Context, id int64) (*model.TelegramMenu, error) {
	m, err := s.menus.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("menu not found")
		}
		return nil, apperr.Internal("lookup failed").Wrap(err)
	}
	return m, nil
}

// ---- Responses ----

// ResponseInput carries response create/update fields.
type ResponseInput struct {
	Command   string
	ReplyText string
	IsEnabled bool
}

// CreateResponse creates a static response.
func (s *TelegramService) CreateResponse(ctx context.Context, in ResponseInput) (*model.TelegramResponse, error) {
	cmd, err := normalizeCommand(in.Command)
	if err != nil {
		return nil, err
	}
	r, err := s.responses.Create(ctx, &model.TelegramResponse{
		Command:   cmd,
		ReplyText: in.ReplyText,
		IsEnabled: in.IsEnabled,
	})
	if err != nil {
		if repository.IsUniqueViolation(err) {
			return nil, apperr.Conflict("a response with this command already exists")
		}
		return nil, apperr.Internal("could not create response").Wrap(err)
	}
	return r, nil
}

// ListResponses returns all responses (admin) or enabled only.
func (s *TelegramService) ListResponses(ctx context.Context, enabledOnly bool) ([]model.TelegramResponse, error) {
	items, err := s.responses.List(ctx, enabledOnly)
	if err != nil {
		return nil, apperr.Internal("could not list responses").Wrap(err)
	}
	return items, nil
}

// ResolveResponse returns the enabled reply for a command (bot use).
func (s *TelegramService) ResolveResponse(ctx context.Context, command string) (*model.TelegramResponse, error) {
	cmd, err := normalizeCommand(command)
	if err != nil {
		return nil, err
	}
	r, err := s.responses.GetByCommand(ctx, cmd)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("no response for command")
		}
		return nil, apperr.Internal("lookup failed").Wrap(err)
	}
	return r, nil
}

// UpdateResponse updates a response.
func (s *TelegramService) UpdateResponse(ctx context.Context, id int64, in ResponseInput) (*model.TelegramResponse, error) {
	existing, err := s.getResponse(ctx, id)
	if err != nil {
		return nil, err
	}
	cmd, err := normalizeCommand(in.Command)
	if err != nil {
		return nil, err
	}
	existing.Command = cmd
	existing.ReplyText = in.ReplyText
	r, err := s.responses.Update(ctx, existing)
	if err != nil {
		if repository.IsUniqueViolation(err) {
			return nil, apperr.Conflict("a response with this command already exists")
		}
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("response not found")
		}
		return nil, apperr.Internal("could not update response").Wrap(err)
	}
	return r, nil
}

// SetResponseEnabled toggles a response.
func (s *TelegramService) SetResponseEnabled(ctx context.Context, id int64, enabled bool) (*model.TelegramResponse, error) {
	r, err := s.responses.SetEnabled(ctx, id, enabled)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("response not found")
		}
		return nil, apperr.Internal("could not update response").Wrap(err)
	}
	return r, nil
}

// DeleteResponse removes a response.
func (s *TelegramService) DeleteResponse(ctx context.Context, id int64) error {
	if err := s.responses.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return apperr.NotFound("response not found")
		}
		return apperr.Internal("could not delete response").Wrap(err)
	}
	return nil
}

func (s *TelegramService) getResponse(ctx context.Context, id int64) (*model.TelegramResponse, error) {
	r, err := s.responses.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("response not found")
		}
		return nil, apperr.Internal("lookup failed").Wrap(err)
	}
	return r, nil
}
