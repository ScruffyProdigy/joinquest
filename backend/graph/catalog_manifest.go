package graph

import (
	"context"
	"errors"
	"fmt"

	"github.com/scruffyprodigy/joinquest/internal/gameclient"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func (r *Resolver) manifestFetcher() *gameclient.ManifestFetcher {
	if r.ManifestFetcher != nil {
		return r.ManifestFetcher
	}
	return gameclient.NewManifestFetcher()
}

func (r *Resolver) syncGameManifest(ctx context.Context, game *store.Game) (*store.ApplyManifestResult, error) {
	if game.APIBaseURL == nil {
		return nil, fmt.Errorf("game has no apiBaseUrl configured")
	}
	priorETag := ""
	if game.ManifestETag != nil {
		priorETag = *game.ManifestETag
	}

	manifest, err := r.manifestFetcher().Fetch(ctx, *game.APIBaseURL, priorETag)
	if err != nil {
		if errors.Is(err, gameclient.ErrManifestNotModified) {
			return &store.ApplyManifestResult{Game: game, Changed: false}, nil
		}
		return nil, err
	}

	st, err := r.requireStore()
	if err != nil {
		return nil, err
	}
	return st.ApplyGameManifest(ctx, game.ID, manifest)
}
