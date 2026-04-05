package publicapi

import (
	"bytes"
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

type createProjectRepoStub struct {
	handlerRepository

	createProjectFn func(ctx context.Context, groupID, createdBy uuid.UUID, name string) (*ProjectRow, error)

	called      bool
	lastGroupID uuid.UUID
	lastUserID  uuid.UUID
	lastName    string
}

func (s *createProjectRepoStub) CreateProject(ctx context.Context, groupID, createdBy uuid.UUID, name string) (*ProjectRow, error) {
	s.called = true
	s.lastGroupID = groupID
	s.lastUserID = createdBy
	s.lastName = name

	if s.createProjectFn == nil {
		panic("createProjectFn is nil")
	}
	return s.createProjectFn(ctx, groupID, createdBy, name)
}

func TestAPIHandler_CreateProject(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	groupID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	projectID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	createdAt := time.Date(2026, time.April, 5, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		withSession    bool
		contentType    string
		body           string
		repoFn         func(ctx context.Context, groupID, createdBy uuid.UUID, name string) (*ProjectRow, error)
		wantStatus     int
		wantCode       string
		wantMessage    string
		wantRepoCalled bool
	}{
		{
			name:        "given_valid_request_when_create_project_then_returns_201",
			withSession: true,
			contentType: "application/json",
			body:        `{"name":"Launch Plan"}`,
			repoFn: func(ctx context.Context, gid, uid uuid.UUID, name string) (*ProjectRow, error) {
				return &ProjectRow{
					ID:        projectID,
					GroupID:   gid,
					Name:      name,
					CreatedBy: uid,
					CreatedAt: createdAt,
				}, nil
			},
			wantStatus:     http.StatusCreated,
			wantRepoCalled: true,
		},
		{
			name:           "given_missing_session_when_create_project_then_returns_401",
			withSession:    false,
			contentType:    "application/json",
			body:           `{"name":"Launch Plan"}`,
			wantStatus:     http.StatusUnauthorized,
			wantCode:       "unauthorized",
			wantMessage:    "missing session context",
			wantRepoCalled: false,
		},
		{
			name:           "given_invalid_json_when_create_project_then_returns_400",
			withSession:    true,
			contentType:    "application/json",
			body:           `{`,
			wantStatus:     http.StatusBadRequest,
			wantCode:       "bad_request",
			wantMessage:    "invalid json body",
			wantRepoCalled: false,
		},
		{
			name:           "given_empty_body_when_create_project_then_returns_400",
			withSession:    true,
			contentType:    "application/json",
			body:           ``,
			wantStatus:     http.StatusBadRequest,
			wantCode:       "bad_request",
			wantMessage:    "request body is required",
			wantRepoCalled: false,
		},
		{
			name:           "given_wrong_content_type_when_create_project_then_returns_400",
			withSession:    true,
			contentType:    "text/plain",
			body:           `{"name":"Launch Plan"}`,
			wantStatus:     http.StatusBadRequest,
			wantCode:       "bad_request",
			wantMessage:    "content-type must be application/json",
			wantRepoCalled: false,
		},
		{
			name:           "given_multiple_json_objects_when_create_project_then_returns_400",
			withSession:    true,
			contentType:    "application/json",
			body:           `{"name":"Launch Plan"}{"name":"Another"}`,
			wantStatus:     http.StatusBadRequest,
			wantCode:       "bad_request",
			wantMessage:    "request body must contain a single JSON object",
			wantRepoCalled: false,
		},
		{
			name:        "given_invalid_project_name_error_when_create_project_then_returns_400",
			withSession: true,
			contentType: "application/json",
			body:        `{"name":"Launch Plan"}`,
			repoFn: func(ctx context.Context, gid, uid uuid.UUID, name string) (*ProjectRow, error) {
				return nil, ErrInvalidProjectName
			},
			wantStatus:     http.StatusBadRequest,
			wantCode:       "bad_request",
			wantMessage:    "invalid project name",
			wantRepoCalled: true,
		},
		{
			name:        "given_unexpected_error_when_create_project_then_returns_500",
			withSession: true,
			contentType: "application/json",
			body:        `{"name":"Launch Plan"}`,
			repoFn: func(ctx context.Context, gid, uid uuid.UUID, name string) (*ProjectRow, error) {
				return nil, errors.New("db down")
			},
			wantStatus:     http.StatusInternalServerError,
			wantCode:       "internal_error",
			wantMessage:    "failed to create project",
			wantRepoCalled: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &createProjectRepoStub{
				createProjectFn: func(ctx context.Context, gid, uid uuid.UUID, name string) (*ProjectRow, error) {
					if !tc.wantRepoCalled {
						t.Fatalf("CreateProject must not be called for case %q", tc.name)
					}
					if tc.repoFn == nil {
						t.Fatalf("repoFn is required for case %q", tc.name)
					}
					return tc.repoFn(ctx, gid, uid, name)
				},
			}

			h := &APIHandler{repo: repo}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(tc.body))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			if tc.withSession {
				req = req.WithContext(WithSession(req.Context(), SessionContext{
					SessionID:     uuid.New(),
					UserID:        userID,
					ActiveGroupID: groupID,
				}))
			}

			rr := httptest.NewRecorder()
			h.CreateProject(rr, req)

			require.Equal(t, tc.wantStatus, rr.Code)
			assert.Equal(t, tc.wantRepoCalled, repo.called)

			if tc.wantRepoCalled {
				assert.Equal(t, groupID, repo.lastGroupID)
				assert.Equal(t, userID, repo.lastUserID)
				assert.Equal(t, "Launch Plan", repo.lastName)
			}

			if tc.wantStatus == http.StatusCreated {
				var payload CreateProjectPayload
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))

				assert.Equal(t, projectID.String(), payload.Project.Id.String())
				assert.Equal(t, groupID.String(), payload.Project.GroupId.String())
				assert.Equal(t, userID.String(), payload.Project.CreatedBy.String())
				assert.Equal(t, "Launch Plan", payload.Project.Name)
				assert.True(t, payload.Project.CreatedAt.Equal(createdAt))
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
