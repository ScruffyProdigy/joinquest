package graph

import (
	"strings"

	"github.com/scruffyprodigy/playhub/graph/model"
	"github.com/scruffyprodigy/playhub/internal/auth"
	"github.com/scruffyprodigy/playhub/internal/avatars"
	"github.com/scruffyprodigy/playhub/internal/store"
)

func toGraphQLAvatarSource(source *string) *model.AvatarSource {
	out := model.AvatarSourceNone
	if source == nil {
		return &out
	}
	switch strings.ToLower(strings.TrimSpace(*source)) {
	case avatars.SourceStarter:
		out = model.AvatarSourceStarter
	case avatars.SourceSigil:
		out = model.AvatarSourceSigil
	case "spirit_animal":
		out = model.AvatarSourceSpiritAnimal
	default:
		out = model.AvatarSourceNone
	}
	return &out
}

func userAvatarURL(user *store.User) *string {
	if user == nil {
		return nil
	}
	return avatars.ResolveURL(auth.LobbyPublicURL(), user.AvatarURL, user.AvatarKey)
}
