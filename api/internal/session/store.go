package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrStoreNotConfigured = errors.New("session store not configured")
	ErrInvalidSession     = errors.New("invalid or expired session")
	ErrGroupForbidden     = errors.New("group switch forbidden")
)

type Record struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	ActiveGroupID uuid.UUID
	ExpiresAt     time.Time
	RevokedAt     *time.Time
}

type Store struct {
	db  *pgxpool.Pool
	ttl time.Duration
}

func NewStore(db *pgxpool.Pool, ttl time.Duration) *Store {
	return &Store{db: db, ttl: ttl}
}

func (s *Store) Create(ctx context.Context, userID, groupID uuid.UUID) (string, *Record, error) {
	if err := s.ensureReady(); err != nil {
		return "", nil, err
	}

	rawToken, err := generateRawToken()
	if err != nil {
		return "", nil, fmt.Errorf("create session: generate token: %w", err)
	}

	tokenHash := hashToken(rawToken)
	expiresAt := time.Now().UTC().Add(s.ttl)

	rec := &Record{}
	err = s.db.QueryRow(ctx, `
		INSERT INTO public.api_sessions (token_hash, user_id, active_group_id, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, active_group_id, expires_at, revoked_at
	`, tokenHash, userID, groupID, expiresAt).Scan(
		&rec.ID, &rec.UserID, &rec.ActiveGroupID, &rec.ExpiresAt, &rec.RevokedAt,
	)
	if err != nil {
		return "", nil, fmt.Errorf("create session insert failed: %w", err)
	}

	return rawToken, rec, nil
}

func (s *Store) GetByRawToken(ctx context.Context, rawToken string) (*Record, error) {
	if err := s.ensureReady(); err != nil {
		return nil, err
	}

	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, ErrInvalidSession
	}

	tokenHash := hashToken(rawToken)

	rec := &Record{}
	err := s.db.QueryRow(ctx, `
		WITH updated AS (
			UPDATE public.api_sessions
			SET last_seen_at = NOW()
			WHERE token_hash = $1
			  AND revoked_at IS NULL
			  AND expires_at > NOW()
			RETURNING id, user_id, active_group_id, expires_at, revoked_at
		)
		SELECT id, user_id, active_group_id, expires_at, revoked_at
		FROM updated
	`, tokenHash).Scan(
		&rec.ID, &rec.UserID, &rec.ActiveGroupID, &rec.ExpiresAt, &rec.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidSession
		}
		return nil, fmt.Errorf("get session by token failed: %w", err)
	}

	return rec, nil
}

func (s *Store) SwitchActiveGroup(ctx context.Context, sessionID, userID, nextGroupID uuid.UUID) error {
	if err := s.ensureReady(); err != nil {
		return err
	}

	tag, err := s.db.Exec(ctx, `
		UPDATE public.api_sessions s
		SET active_group_id = $1, last_seen_at = NOW()
		WHERE s.id = $2
		  AND s.user_id = $3
		  AND s.revoked_at IS NULL
		  AND s.expires_at > NOW()
		  AND EXISTS (
			  SELECT 1
			  FROM public.group_memberships gm
			  WHERE gm.user_id = $3
			    AND gm.group_id = $1
		  )
	`, nextGroupID, sessionID, userID)
	if err != nil {
		return fmt.Errorf("switch active group update failed: %w", err)
	}
	if tag.RowsAffected() > 0 {
		return nil
	}

	// Distinguish invalid/expired session from forbidden target group.
	var sessionValid bool
	err = s.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM public.api_sessions s
			WHERE s.id = $1
			  AND s.user_id = $2
			  AND s.revoked_at IS NULL
			  AND s.expires_at > NOW()
		)
	`, sessionID, userID).Scan(&sessionValid)
	if err != nil {
		return fmt.Errorf("switch active group session check failed: %w", err)
	}
	if !sessionValid {
		return ErrInvalidSession
	}

	return ErrGroupForbidden
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func generateRawToken() (string, error) {
	b := make([]byte, 32) //256 bit token
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Store) ensureReady() error {
	if s == nil || s.db == nil {
		return ErrStoreNotConfigured
	}
	if s.ttl <= 0 {
		return fmt.Errorf("invalid session ttl: %w", ErrStoreNotConfigured)
	}
	return nil
}
