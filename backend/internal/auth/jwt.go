package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	defaultJWTKID  = "lobby-dev"
	devJWTKeyFile  = ".dev-jwt-key.pem"
	devJWTKeyPerms = 0o600
)

// Signer creates and verifies session JWTs.
type Signer struct {
	kid        string
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	publicX    string

	rotationMu   sync.RWMutex
	rotationKeys map[string]ed25519.PublicKey
}

// LoadSignerFromEnv loads signing keys from environment or generates dev keys.
func LoadSignerFromEnv() (*Signer, error) {
	kid := os.Getenv("JWKS_KID")
	if kid == "" {
		kid = defaultJWTKID
	}

	if pemData := strings.TrimSpace(os.Getenv("JWT_PRIVATE_KEY_PEM")); pemData != "" {
		return newSignerFromPEM(kid, pemData)
	}

	return loadOrCreateDevSigner(kid)
}

func devJWTKeyPath() string {
	if p := strings.TrimSpace(os.Getenv("JWT_DEV_KEY_FILE")); p != "" {
		return p
	}
	return devJWTKeyFile
}

// loadOrCreateDevSigner reuses a PEM file in the backend working directory so
// local restarts keep the same kid/JWKS and game servers can verify seat tokens.
func loadOrCreateDevSigner(kid string) (*Signer, error) {
	path := devJWTKeyPath()
	if data, err := os.ReadFile(path); err == nil {
		pemData := strings.TrimSpace(string(data))
		if pemData != "" {
			signer, err := newSignerFromPEM(kid, pemData)
			if err != nil {
				return nil, fmt.Errorf("auth: dev key file %s: %w", path, err)
			}
			fmt.Fprintf(os.Stderr, "auth: loaded development JWT signing key from %s (kid=%s)\n", path, kid)
			return signer, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("auth: read dev key file %s: %w", path, err)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("auth: generate dev signing key: %w", err)
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("auth: marshal dev signing key: %w", err)
	}
	pemData := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}))
	if err := os.WriteFile(path, []byte(pemData), devJWTKeyPerms); err != nil {
		fmt.Fprintf(os.Stderr, "auth: warning: could not persist dev JWT key to %s: %v\n", path, err)
	} else {
		fmt.Fprintf(os.Stderr, "auth: generated and persisted development JWT signing key to %s (kid=%s)\n", path, kid)
	}

	return &Signer{
		kid:        kid,
		privateKey: priv,
		publicKey:  pub,
		publicX:    base64.RawURLEncoding.EncodeToString(pub),
	}, nil
}

func newSignerFromPEM(kid, pemData string) (*Signer, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, errors.New("auth: invalid JWT private key PEM")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("auth: parse JWT private key: %w", err)
	}

	privateKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("auth: JWT private key must be Ed25519")
	}

	publicKey := privateKey.Public().(ed25519.PublicKey)
	return &Signer{
		kid:        kid,
		privateKey: privateKey,
		publicKey:  publicKey,
		publicX:    base64.RawURLEncoding.EncodeToString(publicKey),
	}, nil
}

// PublicJWK returns the public JWKS payload for this signer's primary key.
func (s *Signer) PublicJWK() map[string]string {
	return publicJWK(s.kid, s.publicX)
}

func publicJWK(kid, publicX string) map[string]string {
	return map[string]string{
		"kty": "OKP",
		"crv": "Ed25519",
		"use": "sig",
		"alg": "EdDSA",
		"kid": kid,
		"x":   publicX,
	}
}

// JWKSHandler serves the public JWKS document, including any temporary
// rotation keys added via AddRotationKey.
func (s *Signer) JWKSHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		s.rotationMu.RLock()
		keys := make([]any, 0, 1+len(s.rotationKeys))
		keys = append(keys, s.PublicJWK())
		for kid, pub := range s.rotationKeys {
			keys = append(keys, publicJWK(kid, base64.RawURLEncoding.EncodeToString(pub)))
		}
		s.rotationMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	}
}

const maxRotationKeys = 4

// AddRotationKey generates a temporary ed25519 keypair and publishes its
// public half in this signer's JWKS document until the returned cleanup
// func is called. Integration checks use this to verify a game can validate
// tokens signed under more than one active key, as happens during a real
// signing-key rotation.
func (s *Signer) AddRotationKey() (kid string, priv ed25519.PrivateKey, cleanup func(), err error) {
	s.rotationMu.RLock()
	atCap := len(s.rotationKeys) >= maxRotationKeys
	s.rotationMu.RUnlock()
	if atCap {
		return "", nil, nil, fmt.Errorf("auth: too many concurrent rotation keys (max %d)", maxRotationKeys)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", nil, nil, fmt.Errorf("auth: generate rotation key: %w", err)
	}
	kid = s.kid + "-rotate-" + uuid.NewString()

	s.rotationMu.Lock()
	if s.rotationKeys == nil {
		s.rotationKeys = make(map[string]ed25519.PublicKey)
	}
	if len(s.rotationKeys) >= maxRotationKeys {
		s.rotationMu.Unlock()
		return "", nil, nil, fmt.Errorf("auth: too many concurrent rotation keys (max %d)", maxRotationKeys)
	}
	s.rotationKeys[kid] = pub
	s.rotationMu.Unlock()

	cleanup = func() {
		s.rotationMu.Lock()
		delete(s.rotationKeys, kid)
		s.rotationMu.Unlock()
	}
	return kid, priv, cleanup, nil
}

const sessionTokenType = "session"

func sessionTokenAudience() string {
	return LobbyIssuer()
}

// SignUserToken issues a session JWT for the given user ID.
func (s *Signer) SignUserToken(userID uuid.UUID, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": LobbyIssuer(),
		"aud": sessionTokenAudience(),
		"typ": sessionTokenType,
		"sub": userID.String(),
		"jti": uuid.NewString(),
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"exp": now.Add(ttl).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = s.kid
	return token.SignedString(s.privateKey)
}

// UserSession is a verified session token plus the window it was issued for.
// Callers use the window to slide the session before it expires (JQ-86): both
// SESSION_TTL and the cookie MaxAge are absolute from creation, so without a
// renewal an active guest eventually expires out of the only identity they have.
type UserSession struct {
	UserID    uuid.UUID
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// Lifetime is how long the token was minted for.
func (u UserSession) Lifetime() time.Duration {
	return u.ExpiresAt.Sub(u.IssuedAt)
}

// PastHalfLife reports whether more than half of the token's life has elapsed.
func (u UserSession) PastHalfLife() bool {
	lifetime := u.Lifetime()
	if lifetime <= 0 {
		return false
	}
	return time.Until(u.ExpiresAt) < lifetime/2
}

// VerifyUserToken validates a session JWT and returns the user ID.
func (s *Signer) VerifyUserToken(tokenString string) (uuid.UUID, error) {
	session, err := s.VerifyUserSession(tokenString)
	if err != nil {
		return uuid.Nil, err
	}
	return session.UserID, nil
}

// VerifyUserSession validates a session JWT and returns the user ID together with
// the token's issue and expiry times.
func (s *Signer) VerifyUserSession(tokenString string) (UserSession, error) {
	issuer := LobbyIssuer()
	audience := sessionTokenAudience()
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodEdDSA {
			return nil, fmt.Errorf("auth: unexpected signing method %v", token.Header["alg"])
		}
		return s.publicKey, nil
	},
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
	)
	if err != nil {
		return UserSession{}, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return UserSession{}, errors.New("auth: invalid token claims")
	}
	if typ, _ := claims["typ"].(string); typ != sessionTokenType {
		return UserSession{}, errors.New("auth: token is not a session token")
	}

	sub, err := claims.GetSubject()
	if err != nil || sub == "" {
		return UserSession{}, errors.New("auth: token missing subject")
	}

	userID, err := uuid.Parse(sub)
	if err != nil {
		return UserSession{}, fmt.Errorf("auth: invalid subject: %w", err)
	}

	session := UserSession{UserID: userID}
	if issuedAt, err := claims.GetIssuedAt(); err == nil && issuedAt != nil {
		session.IssuedAt = issuedAt.Time
	}
	if expiresAt, err := claims.GetExpirationTime(); err == nil && expiresAt != nil {
		session.ExpiresAt = expiresAt.Time
	}

	return session, nil
}
