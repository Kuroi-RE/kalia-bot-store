package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/kalia/store/internal/model"
	"github.com/kalia/store/internal/repository"
	"github.com/kalia/store/pkg/apperr"
	"github.com/kalia/store/pkg/crypto"
	"github.com/kalia/store/pkg/token"
)

// AuthService handles admin authentication and bootstrap.
type AuthService struct {
	admins *repository.AdminRepository
	tokens *token.Manager
	log    *slog.Logger
}

// NewAuthService builds an auth service.
func NewAuthService(admins *repository.AdminRepository, tokens *token.Manager, log *slog.Logger) *AuthService {
	return &AuthService{admins: admins, tokens: tokens, log: log}
}

// LoginResult carries an issued token.
type LoginResult struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Admin     *model.Admin `json:"admin"`
}

// Login verifies credentials and returns a signed JWT.
func (s *AuthService) Login(ctx context.Context, username, password string) (*LoginResult, error) {
	admin, err := s.admins.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// Uniform error to avoid user enumeration.
			return nil, apperr.Unauthorized("invalid credentials")
		}
		return nil, apperr.Internal("login failed").Wrap(err)
	}

	if !crypto.CheckPassword(admin.PasswordHash, password) {
		return nil, apperr.Unauthorized("invalid credentials")
	}

	signed, expiresAt, err := s.tokens.Issue(admin.ID, admin.Username)
	if err != nil {
		return nil, apperr.Internal("could not issue token").Wrap(err)
	}
	return &LoginResult{Token: signed, ExpiresAt: expiresAt, Admin: admin}, nil
}

// Me returns the admin for the given id.
func (s *AuthService) Me(ctx context.Context, adminID int64) (*model.Admin, error) {
	admin, err := s.admins.GetByID(ctx, adminID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("admin not found")
		}
		return nil, apperr.Internal("lookup failed").Wrap(err)
	}
	return admin, nil
}

// Bootstrap ensures at least one admin exists, seeding one from the provided
// credentials when the table is empty. Safe to call on every startup.
func (s *AuthService) Bootstrap(ctx context.Context, username, password string) error {
	if username == "" || password == "" {
		s.log.Warn("admin bootstrap skipped: ADMIN_USERNAME/ADMIN_PASSWORD not set")
		return nil
	}
	count, err := s.admins.Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return err
	}
	if _, err := s.admins.Create(ctx, username, hash); err != nil {
		return err
	}
	s.log.Info("seeded initial admin account", slog.String("username", username))
	return nil
}
