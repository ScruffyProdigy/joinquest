package auth

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/internal/store"
)

// CreateGuestSession creates a guest user and returns a session JWT.
func (s *Service) CreateGuestSession(ctx context.Context) (*store.User, string, error) {
	user, err := s.store.CreateGuestUser(ctx)
	if err != nil {
		return nil, "", err
	}
	if err := s.store.UpdateUserLastLogin(ctx, user.ID); err != nil {
		return nil, "", err
	}
	sessionToken, err := s.signer.SignUserToken(user.ID, s.sessionTTL)
	if err != nil {
		return nil, "", err
	}
	return user, sessionToken, nil
}

// RequireNonGuestUser loads the authenticated user and rejects guest accounts.
func (s *Service) RequireNonGuestUser(ctx context.Context) (*store.User, error) {
	user, err := s.GetAuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrAuthenticationRequired
	}
	if user.IsGuest {
		return nil, ErrGuestAccountNotAllowed
	}
	return user, nil
}

// RequestLinkEmail sends a verification email to attach an address to the current account.
func (s *Service) RequestLinkEmail(ctx context.Context, userID uuid.UUID, emailAddr string) error {
	normalized, err := normalizeEmail(emailAddr)
	if err != nil {
		return err
	}

	since := time.Now().Add(-magicLinkRateWindow)
	count, err := s.store.CountRecentMagicLinksByEmail(ctx, normalized, since)
	if err != nil {
		return err
	}
	if count >= maxMagicLinksPerEmailPerHour {
		return ErrMagicLinkRateLimit
	}

	code, err := generateLoginCode()
	if err != nil {
		return err
	}

	token := uuid.NewString()
	link, err := s.store.CreateMagicLink(ctx, store.CreateMagicLinkParams{
		UserID:    &userID,
		Email:     normalized,
		TokenHash: hashMagicLinkToken(token),
		CodeHash:  hashLoginCode(code),
		ExpiresAt: time.Now().Add(s.magicLinkTTL),
	})
	if err != nil {
		return err
	}

	if err := s.deliverLinkEmail(ctx, normalized, token, code); err != nil {
		if delErr := s.store.DeleteMagicLink(ctx, link.ID); delErr != nil {
			log.Printf("auth: rollback link email %s after delivery failure: %v", link.ID, delErr)
		}
		return err
	}
	return nil
}

// PreviewLinkEmail reports whether verifying this email would merge another account in.
func (s *Service) PreviewLinkEmail(ctx context.Context, userID uuid.UUID, emailAddr string) (*EmailLinkPreview, error) {
	normalized, err := normalizeEmail(emailAddr)
	if err != nil {
		return nil, err
	}

	preview := &EmailLinkPreview{Email: normalized}
	existingOwnerID, err := s.store.ResolveUserIDByEmail(ctx, normalized)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return preview, nil
		}
		return nil, err
	}
	if existingOwnerID == userID {
		return preview, nil
	}

	source, err := s.store.GetUserByID(ctx, existingOwnerID)
	if err != nil {
		return nil, err
	}

	preview.WillMergeAccounts = true
	preview.MergeSourceDisplayName = source.ChosenDisplayName()
	return preview, nil
}

// CompleteLinkEmailWithCode verifies a code and attaches the email to link.user_id.
func (s *Service) CompleteLinkEmailWithCode(ctx context.Context, emailAddr, code string, confirmMerge bool) (*store.User, string, error) {
	normalized, err := normalizeEmail(emailAddr)
	if err != nil {
		return nil, "", err
	}

	now := time.Now()
	link, err := s.store.GetLatestValidMagicLinkByEmail(ctx, normalized, now)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, "", ErrInvalidLoginCode
		}
		return nil, "", err
	}
	if link.UserID == nil {
		return nil, "", ErrInvalidLoginCode
	}

	if !verifyLoginCode(strings.TrimSpace(code), link.CodeHash) {
		if incErr := s.store.IncrementMagicLinkFailedAttempts(ctx, link.ID); incErr != nil {
			return nil, "", incErr
		}
		if link.FailedAttempts+1 >= store.MaxLoginCodeAttempts {
			return nil, "", ErrTooManyLoginAttempts
		}
		return nil, "", ErrInvalidLoginCode
	}

	needsMerge, err := s.linkNeedsMergeConfirmation(ctx, *link.UserID, link.Email)
	if err != nil {
		return nil, "", err
	}
	if needsMerge && !confirmMerge {
		return nil, "", ErrMergeConfirmationRequired
	}

	link, err = s.store.ConsumeMagicLinkByID(ctx, link.ID, now)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, "", ErrInvalidLoginCode
		}
		return nil, "", err
	}
	return s.finishEmailLink(ctx, *link.UserID, link.Email, confirmMerge)
}

func (s *Service) linkNeedsMergeConfirmation(ctx context.Context, targetUserID uuid.UUID, emailAddr string) (bool, error) {
	normalized := strings.ToLower(strings.TrimSpace(emailAddr))
	existingOwnerID, err := s.store.ResolveUserIDByEmail(ctx, normalized)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return existingOwnerID != targetUserID, nil
}

// CompleteLinkEmailWithToken verifies a magic-link token and attaches the email to link.user_id.
func (s *Service) CompleteLinkEmailWithToken(ctx context.Context, token string, confirmMerge bool) (*store.User, string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, "", ErrInvalidMagicLink
	}
	now := time.Now()
	tokenHash := hashMagicLinkToken(token)
	link, err := s.store.GetValidMagicLinkByTokenHash(ctx, tokenHash, now)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, "", ErrInvalidMagicLink
		}
		return nil, "", err
	}
	if link.UserID == nil {
		return nil, "", ErrInvalidMagicLink
	}

	needsMerge, err := s.linkNeedsMergeConfirmation(ctx, *link.UserID, link.Email)
	if err != nil {
		return nil, "", err
	}
	if needsMerge && !confirmMerge {
		return nil, "", ErrMergeConfirmationRequired
	}

	link, err = s.store.ConsumeMagicLinkByTokenHash(ctx, tokenHash, now)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, "", ErrInvalidMagicLink
		}
		return nil, "", err
	}
	return s.finishEmailLink(ctx, *link.UserID, link.Email, confirmMerge)
}

func (s *Service) finishEmailLink(ctx context.Context, targetUserID uuid.UUID, emailAddr string, confirmMerge bool) (*store.User, string, error) {
	normalized := strings.ToLower(strings.TrimSpace(emailAddr))
	existingOwnerID, err := s.store.ResolveUserIDByEmail(ctx, normalized)
	if err == nil && existingOwnerID != targetUserID {
		if !confirmMerge {
			return nil, "", ErrMergeConfirmationRequired
		}
		if err := s.store.MergeUserInto(ctx, existingOwnerID, targetUserID); err != nil {
			return nil, "", err
		}
	} else if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, "", err
	}

	if _, err := s.store.AddVerifiedUserEmail(ctx, targetUserID, normalized, true); err != nil {
		if errors.Is(err, store.ErrEmailAlreadyLinked) {
			return nil, "", ErrEmailAlreadyLinked
		}
		return nil, "", err
	}

	user, err := s.store.GetUserByID(ctx, targetUserID)
	if err != nil {
		return nil, "", err
	}
	if err := s.store.UpdateUserLastLogin(ctx, user.ID); err != nil {
		return nil, "", err
	}
	sessionToken, err := s.signer.SignUserToken(user.ID, s.sessionTTL)
	if err != nil {
		return nil, "", err
	}
	return user, sessionToken, nil
}

// RemoveLinkedEmail removes a linked email from the account.
func (s *Service) RemoveLinkedEmail(ctx context.Context, userID, emailID uuid.UUID) (*store.User, error) {
	if err := s.store.RemoveUserEmail(ctx, userID, emailID); err != nil {
		if errors.Is(err, store.ErrLastSignInMethod) {
			return nil, ErrLastSignInMethod
		}
		if errors.Is(err, store.ErrEmailNotLinked) {
			return nil, ErrEmailNotLinked
		}
		return nil, err
	}
	return s.store.GetUserByID(ctx, userID)
}

// SetPrimaryEmail marks an email as primary for the account.
func (s *Service) SetPrimaryEmail(ctx context.Context, userID, emailID uuid.UUID) (*store.User, error) {
	if _, err := s.store.SetPrimaryUserEmail(ctx, userID, emailID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrEmailNotLinked
		}
		return nil, err
	}
	return s.store.GetUserByID(ctx, userID)
}
