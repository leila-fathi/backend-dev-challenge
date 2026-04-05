package publicapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type listProjectsRepoStub struct {
	handlerRepository

	listProjectsFn func(ctx context.Context, groupID uuid.UUID) ([]ProjectRow, error)

	called      bool
	lastGroupID uuid.UUID
}

func (s *listProjectsRepoStub) ListProjectsByGroup(ctx context.Context, groupID uuid.UUID) ([]ProjectRow, error) {
	s.called = true
	s.lastGroupID = groupID

	if s.listProjectsFn == nil {
		panic("listProjectsFn is nil")
	}
	return s.listProjectsFn(ctx, groupID)
}

func withListProjectsSession(req *http.Request, userID, groupID uuid.UUID) *http.Request {
	return req.WithContext(WithSession(req.Context(), SessionContext{
		SessionID:     uuid.New(),
		UserID:        userID,
		ActiveGroupID: groupID,
	}))
}

func TestAPIHandler_ListProjects(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	activeGroupID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	createdAt1 := time.Date(2026, time.April, 5, 10, 0, 0, 0, time.UTC)
	createdAt2 := time.Date(2026, time.April, 5, 10, 5, 0, 0, time.UTC)

	expectedProjects := []ProjectRow{
		{
			ID:        uuid.MustParse("33333333-3333-3333-3333-333333333333"),
			GroupID:   activeGroupID,
			Name:      "Project 1",
			CreatedBy: userID,
			CreatedAt: createdAt1,
		},
		{
			ID:        uuid.MustParse("44444444-4444-4444-4444-444444444444"),
			GroupID:   activeGroupID,
			Name:      "Project 2",
			CreatedBy: userID,
			CreatedAt: createdAt2,
		},
	}

	tests := []struct {
		name           string
		withSession    bool
		repoFn         func(ctx context.Context, groupID uuid.UUID) ([]ProjectRow, error)
		wantStatus     int
		wantCode       string
		wantMessage    string
		wantProjects   []ProjectRow
		wantRepoCalled bool
	}{
		{
			name:        "given_valid_session_and_projects_when_list_projects_then_returns_200",
			withSession: true,
			repoFn: func(ctx context.Context, groupID uuid.UUID) ([]ProjectRow, error) {
				return expectedProjects, nil
			},
			wantStatus:     http.StatusOK,
			wantProjects:   expectedProjects,
			wantRepoCalled: true,
		},
		{
			name:        "given_valid_session_and_no_projects_when_list_projects_then_returns_200_with_empty_list",
			withSession: true,
			repoFn: func(ctx context.Context, groupID uuid.UUID) ([]ProjectRow, error) {
				return []ProjectRow{}, nil
			},
			wantStatus:     http.StatusOK,
			wantProjects:   []ProjectRow{},
			wantRepoCalled: true,
		},
		{
			name:           "given_missing_session_when_list_projects_then_returns_401",
			withSession:    false,
			wantStatus:     http.StatusUnauthorized,
			wantCode:       "unauthorized",
			wantMessage:    "missing session context",
			wantRepoCalled: false,
		},
		{
			name:        "given_repository_error_when_list_projects_then_returns_500",
			withSession: true,
			repoFn: func(ctx context.Context, groupID uuid.UUID) ([]ProjectRow, error) {
				return nil, errors.New("db down")
			},
			wantStatus:     http.StatusInternalServerError,
			wantCode:       "internal_error",
			wantMessage:    "failed to list projects",
			wantRepoCalled: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &listProjectsRepoStub{}
			if tc.repoFn != nil {
				repo.listProjectsFn = tc.repoFn
			} else {
				repo.listProjectsFn = func(context.Context, uuid.UUID) ([]ProjectRow, error) {
					t.Fatalf("ListProjectsByGroup should not be called for case %q", tc.name)
					return nil, nil
				}
			}

			h := &APIHandler{repo: repo}

			req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
			if tc.withSession {
				req = withListProjectsSession(req, userID, activeGroupID)
			}

			rr := httptest.NewRecorder()
			h.ListProjects(rr, req)

			require.Equal(t, tc.wantStatus, rr.Code)
			assert.Equal(t, tc.wantRepoCalled, repo.called)

			if tc.wantRepoCalled {
				assert.Equal(t, activeGroupID, repo.lastGroupID)
			}

			if tc.wantStatus == http.StatusOK {
				var resp ProjectsResponse
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
				require.Len(t, resp.Projects, len(tc.wantProjects))

				for i, want := range tc.wantProjects {
					got := resp.Projects[i]
					assert.Equal(t, want.ID.String(), got.Id.String())
					assert.Equal(t, want.GroupID.String(), got.GroupId.String())
					assert.Equal(t, want.Name, got.Name)
					assert.Equal(t, want.CreatedBy.String(), got.CreatedBy.String())
					assert.True(t, got.CreatedAt.Equal(want.CreatedAt))
				}
				return
			}

			var errResp ErrorResponse
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &errResp))
			assert.Equal(t, tc.wantCode, errResp.Code)
			assert.Equal(t, tc.wantMessage, errResp.Message)

			if tc.wantStatus == http.StatusUnauthorized {
				assert.Equal(t, "Bearer", rr.Header().Get("WWW-Authenticate"))
			}
		})
	}
}
