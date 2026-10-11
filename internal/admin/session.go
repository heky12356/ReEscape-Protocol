package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	adminSessionCookie = "admin_session"
	adminSessionTTL    = 8 * time.Hour
)

type adminSession struct {
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastSeen   time.Time
	SecretHash string
}

type sessionStore struct {
	mu       sync.Mutex
	sessions map[string]adminSession
}

func newSessionStore() *sessionStore {
	return &sessionStore{sessions: make(map[string]adminSession)}
}

func (s *sessionStore) create(now time.Time, secret string) (token string, expiresAt time.Time, err error) {
	if s == nil {
		return "", time.Time{}, fmt.Errorf("session store is nil")
	}
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", time.Time{}, fmt.Errorf("generate session token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(randomBytes)
	expiresAt = now.Add(adminSessionTTL)
	s.mu.Lock()
	s.pruneLocked(now)
	s.sessions[sessionDigest(token)] = adminSession{CreatedAt: now, ExpiresAt: expiresAt, LastSeen: now, SecretHash: sessionDigest(secret)}
	s.mu.Unlock()
	return token, expiresAt, nil
}

func (s *sessionStore) valid(token string, now time.Time, secret string) bool {
	if s == nil || strings.TrimSpace(token) == "" {
		return false
	}
	digest := sessionDigest(token)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(now)
	session, ok := s.sessions[digest]
	if !ok || !now.Before(session.ExpiresAt) || session.SecretHash != sessionDigest(secret) {
		return false
	}
	session.LastSeen = now
	s.sessions[digest] = session
	return true
}

func (s *sessionStore) revoke(token string) {
	if s == nil || strings.TrimSpace(token) == "" {
		return
	}
	s.mu.Lock()
	delete(s.sessions, sessionDigest(token))
	s.mu.Unlock()
}

func (s *sessionStore) pruneLocked(now time.Time) {
	for digest, session := range s.sessions {
		if !now.Before(session.ExpiresAt) {
			delete(s.sessions, digest)
		}
	}
}

func sessionDigest(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func constantTimeSecretEqual(provided, expected string) bool {
	provided = strings.TrimSpace(provided)
	expected = strings.TrimSpace(expected)
	if provided == "" || expected == "" || len(provided) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}
