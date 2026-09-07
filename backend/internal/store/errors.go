package store

import "errors"

var (
	ErrNotFound       = errors.New("store: not found")
	ErrAlreadyExists  = errors.New("store: already exists")
	ErrAlreadyMatched = errors.New("store: already matched for this game")
	ErrActiveGame     = errors.New("store: active game in progress")
)
