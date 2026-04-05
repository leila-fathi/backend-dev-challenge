package publicapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrRepositoryNotConfigured = errors.New("repository not configured")
	ErrNotFound                = errors.New("not found")
	ErrInvalidProjectName      = errors.New("invalid project name")
)

const maxProjectNameLen = 120

type Repository struct {
	db *pgxpool.Pool
}

type UserRow struct {
	ID             uuid.UUID
	Email          string
	DisplayName    string
	DefaultGroupID uuid.UUID
}

type MembershipRow struct {
	GroupID   uuid.UUID
	GroupName string
	Role      string
}

type ProjectRow struct {
	ID        uuid.UUID
	GroupID   uuid.UUID
	Name      string
	CreatedBy uuid.UUID
	CreatedAt time.Time
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetUserByID(ctx context.Context, userID uuid.UUID) (*UserRow, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}

	var u UserRow
	err := r.db.QueryRow(ctx, `
		SELECT id, email, display_name, default_group_id
		FROM public.users
		WHERE id = $1
	`, userID).Scan(&u.ID, &u.Email, &u.DisplayName, &u.DefaultGroupID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("get user by id: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("get user by id query failed: %w", err)
	}

	return &u, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*UserRow, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return nil, fmt.Errorf("get user by email: %w", ErrNotFound)
	}

	var u UserRow
	err := r.db.QueryRow(ctx, `
		SELECT id, email, display_name, default_group_id
		FROM public.users
		WHERE email = $1
	`, trimmed).Scan(&u.ID, &u.Email, &u.DisplayName, &u.DefaultGroupID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("get user by email: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("get user by email query failed: %w", err)
	}

	return &u, nil
}

func (r *Repository) GetUserByZitadelSubject(ctx context.Context, subject string) (*UserRow, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(subject)
	if trimmed == "" {
		return nil, fmt.Errorf("get user by zitadel subject: %w", ErrNotFound)
	}

	var u UserRow
	err := r.db.QueryRow(ctx, `
		SELECT id, email, display_name, default_group_id
		FROM public.users
		WHERE zitadel_subject = $1
	`, trimmed).Scan(&u.ID, &u.Email, &u.DisplayName, &u.DefaultGroupID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("get user by zitadel subject: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("get user by zitadel subject query failed: %w", err)
	}

	return &u, nil
}

func (r *Repository) ListMemberships(ctx context.Context, userID uuid.UUID) ([]MembershipRow, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT gm.group_id, g.name, gm.role
		FROM public.group_memberships gm
		JOIN public.groups g ON g.id = gm.group_id
		WHERE gm.user_id = $1
		ORDER BY g.name ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list memberships query failed: %w", err)
	}
	defer rows.Close()

	out := make([]MembershipRow, 0)
	for rows.Next() {
		var m MembershipRow
		if err := rows.Scan(&m.GroupID, &m.GroupName, &m.Role); err != nil {
			return nil, fmt.Errorf("list memberships scan failed: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list memberships iteration failed: %w", err)
	}

	return out, nil
}

func (r *Repository) UserHasMembership(ctx context.Context, userID, groupID uuid.UUID) (bool, error) {
	if err := r.ensureDB(); err != nil {
		return false, err
	}

	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM public.group_memberships
			WHERE user_id = $1 AND group_id = $2
		)
	`, userID, groupID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check user membership failed: %w", err)
	}

	return exists, nil
}

func (r *Repository) ListProjectsByGroup(ctx context.Context, groupID uuid.UUID) ([]ProjectRow, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, group_id, name, created_by, created_at
		FROM public.projects
		WHERE group_id = $1
		ORDER BY created_at DESC
	`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list projects query failed: %w", err)
	}
	defer rows.Close()

	out := make([]ProjectRow, 0)
	for rows.Next() {
		var p ProjectRow
		if err := rows.Scan(&p.ID, &p.GroupID, &p.Name, &p.CreatedBy, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("list projects scan failed: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list projects iteration failed: %w", err)
	}

	return out, nil
}

func (r *Repository) CreateProject(ctx context.Context, groupID, createdBy uuid.UUID, name string) (*ProjectRow, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, fmt.Errorf("create project validation failed: %w", ErrInvalidProjectName)
	}

	var p ProjectRow
	err := r.db.QueryRow(ctx, `
		INSERT INTO public.projects (group_id, name, created_by)
		VALUES ($1, $2, $3)
		RETURNING id, group_id, name, created_by, created_at
	`, groupID, trimmed, createdBy).Scan(
		&p.ID, &p.GroupID, &p.Name, &p.CreatedBy, &p.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create project failed: %w", err)
	}

	return &p, nil
}

func (r *Repository) ensureDB() error {
	if r == nil || r.db == nil {
		return ErrRepositoryNotConfigured
	}
	return nil
}
