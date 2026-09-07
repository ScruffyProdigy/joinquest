package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/auth"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func buildAccount(ctx context.Context, st *store.Store, user *store.User) (*model.Account, error) {
	emails, err := st.ListUserEmails(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	identities, err := st.ListUserIdentities(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	count, err := st.CountUserSignInMethods(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	graphUser := ToGraphQLUser(user)
	graphUser.Emails = ToGraphQLUserEmails(emails)
	graphUser.Identities = ToGraphQLUserIdentities(identities)
	return &model.Account{
		User:              graphUser,
		Emails:            graphUser.Emails,
		Identities:        graphUser.Identities,
		SignInMethodCount: count,
	}, nil
}

func ToGraphQLUserEmails(items []store.UserEmail) []*model.UserEmail {
	result := make([]*model.UserEmail, len(items))
	for i := range items {
		result[i] = &model.UserEmail{
			ID:         items[i].ID.String(),
			Email:      items[i].Email,
			IsPrimary:  items[i].IsPrimary,
			VerifiedAt: items[i].VerifiedAt,
		}
	}
	return result
}

func ToGraphQLEmailLinkPreview(preview *auth.EmailLinkPreview) *model.EmailLinkPreview {
	if preview == nil {
		return nil
	}
	result := &model.EmailLinkPreview{
		Email:             preview.Email,
		WillMergeAccounts: preview.WillMergeAccounts,
	}
	if preview.MergeSourceDisplayName != "" {
		name := preview.MergeSourceDisplayName
		result.MergeSourceDisplayName = &name
	}
	return result
}

func ToGraphQLUserIdentities(items []store.UserIdentity) []*model.UserIdentity {
	result := make([]*model.UserIdentity, len(items))
	for i := range items {
		provider, err := toGraphQLAuthProvider(items[i].Provider)
		if err != nil {
			continue
		}
		result[i] = &model.UserIdentity{
			ID:         items[i].ID.String(),
			Provider:   provider,
			Email:      items[i].Email,
			VerifiedAt: items[i].VerifiedAt,
		}
	}
	return result
}

func toGraphQLAuthProvider(provider string) (model.AuthProvider, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "google":
		return model.AuthProviderGoogle, nil
	case "discord":
		return model.AuthProviderDiscord, nil
	case "apple":
		return model.AuthProviderApple, nil
	case "facebook":
		return model.AuthProviderFacebook, nil
	default:
		return "", fmt.Errorf("unknown auth provider %q", provider)
	}
}

func loadUserEmails(ctx context.Context, st *store.Store, userID string) ([]*model.UserEmail, error) {
	id, err := parseUUID(userID, "user id")
	if err != nil {
		return nil, err
	}
	emails, err := st.ListUserEmails(ctx, id)
	if err != nil {
		return nil, err
	}
	return ToGraphQLUserEmails(emails), nil
}

func loadUserIdentities(ctx context.Context, st *store.Store, userID string) ([]*model.UserIdentity, error) {
	id, err := parseUUID(userID, "user id")
	if err != nil {
		return nil, err
	}
	identities, err := st.ListUserIdentities(ctx, id)
	if err != nil {
		return nil, err
	}
	return ToGraphQLUserIdentities(identities), nil
}
