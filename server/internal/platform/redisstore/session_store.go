package redisstore

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/dsingh80/the-block/server/internal/domain"
)

// DefaultSessionTTL is the sliding session lifetime production code should use;
// tests use a much shorter one via NewSessionStore's explicit ttl parameter so
// expiry can be observed without a real 30-minute wait.
const DefaultSessionTTL = 30 * time.Minute

// SessionStore implements sessions.Store against Redis. There is deliberately no
// cookie-side Max-Age to match (guidelines/06-backend-architecture.md) -- Redis's
// own EXPIRE is the single source of truth for whether a session is still valid.
type SessionStore struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewSessionStore(rdb *redis.Client, ttl time.Duration) *SessionStore {
	return &SessionStore{rdb: rdb, ttl: ttl}
}

func (s *SessionStore) Touch(ctx context.Context, token string) (domain.Session, bool, error) {
	if token != "" {
		session, found, err := s.refreshExisting(ctx, token)
		if err != nil {
			return domain.Session{}, false, err
		}
		if found {
			return session, false, nil
		}
		// Unknown or expired: fall through and issue a brand-new token rather
		// than trusting the caller's value -- a forged or stale cookie must
		// never let a client pick its own session identity.
	}
	return s.create(ctx)
}

func (s *SessionStore) refreshExisting(ctx context.Context, token string) (domain.Session, bool, error) {
	key := SessionKey(token)

	vals, err := s.rdb.HMGet(ctx, key, "created_at_ms").Result()
	if err != nil {
		return domain.Session{}, false, fmt.Errorf("redisstore: read session %s: %w", token, err)
	}
	createdAtMs, ok := vals[0].(string)
	if !ok || createdAtMs == "" {
		return domain.Session{}, false, nil // expired or never existed -- both read as "not found"
	}

	now := time.Now().UTC()
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, key, "last_seen_at_ms", now.UnixMilli())
	pipe.Expire(ctx, key, s.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return domain.Session{}, false, fmt.Errorf("redisstore: refresh session %s: %w", token, err)
	}

	created, err := parseUnixMillisString(createdAtMs)
	if err != nil {
		return domain.Session{}, false, fmt.Errorf("redisstore: parse created_at for session %s: %w", token, err)
	}

	return domain.Session{Token: token, CreatedAt: created, LastSeenAt: now}, true, nil
}

func (s *SessionStore) create(ctx context.Context) (domain.Session, bool, error) {
	token, err := generateSessionToken()
	if err != nil {
		return domain.Session{}, false, fmt.Errorf("redisstore: generate session token: %w", err)
	}

	now := time.Now().UTC()
	key := SessionKey(token)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, key, "created_at_ms", now.UnixMilli(), "last_seen_at_ms", now.UnixMilli())
	pipe.Expire(ctx, key, s.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return domain.Session{}, false, fmt.Errorf("redisstore: create session: %w", err)
	}

	return domain.Session{Token: token, CreatedAt: now, LastSeenAt: now}, true, nil
}

// generateSessionToken is a 32-byte crypto/rand value, base64url-encoded -- an
// opaque lookup key, not a JWT, nothing worth decoding (guidelines/06-backend-architecture.md).
func generateSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func parseUnixMillisString(s string) (time.Time, error) {
	var ms int64
	if _, err := fmt.Sscanf(s, "%d", &ms); err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(ms).UTC(), nil
}
