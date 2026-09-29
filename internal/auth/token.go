package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AccessClaims is the signed identity. Role is informational; the server
// reloads the account on every request so a disablement takes effect immediately.
type AccessClaims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Manager signs access tokens and creates refresh-token secrets.
type Manager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewManager returns a signer. secret must already be long enough.
func NewManager(secret string, accessTTL, refreshTTL time.Duration) *Manager {
	return &Manager{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

// AccessTTL is the lifetime advertised to clients.
func (m *Manager) AccessTTL() time.Duration { return m.accessTTL }

// RefreshTTL is how long a refresh token stays valid.
func (m *Manager) RefreshTTL() time.Duration { return m.refreshTTL }

// SignAccess returns a compact JWT.
func (m *Manager) SignAccess(userID uuid.UUID, username string, now time.Time) (string, error) {
	claims := AccessClaims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(m.secret)
}

// ParseAccess validates a JWT and returns the user id and username.
func (m *Manager) ParseAccess(token string, now time.Time) (uuid.UUID, string, error) {
	var claims AccessClaims
	parsed, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithTimeFunc(func() time.Time { return now }), jwt.WithLeeway(30*time.Second))
	if err != nil || !parsed.Valid {
		return uuid.Nil, "", fmt.Errorf("invalid token")
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("invalid token")
	}
	return id, claims.Username, nil
}

// NewRefreshToken returns the client secret and the hash to store.
func NewRefreshToken() (secret, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	secret = base64.RawURLEncoding.EncodeToString(buf)
	hash = HashRefresh(secret)
	return secret, hash, nil
}

// HashRefresh is the SHA-256 hex digest stored for a refresh token.
func HashRefresh(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
