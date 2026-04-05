package publicapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"hiring-challenge-backend/api/internal/session"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type switchGroupRepoStub struct {
	handlerRepository

	getUserByIDFn func(ctx context.Context, userID uuid.UUID) (*UserRow, error)

	getUserCalled bool
	lastUserID    uuid.UUID
}

func (s *switchGroupRepoStub) GetUserByID(ctx context.Context, userID uuid.UUID) (*UserRow, error) {
	s.getUserCalled = true
	s.lastUserID = userID
	if s.getUserByIDFn == nil {
		panic("getUserByIDFn is nil")
	}
	return s.getUserByIDFn(ctx, userID)
}

type switchGroupSessionStub struct {
	handlerSessionStore

	switchFn func(ctx context.Context, sessionID, userID, nextGroupID uuid.UUID) error

	called        bool
	lastSessionID uuid.UUID
	lastUserID    uuid.UUID
	lastGroupID   uuid.UUID
}

func (s *switchGroupSessionStub) SwitchActiveGroup(ctx context.Context, sessionID, userID, nextGroupID uuid.UUID) error {
	s.called = true
	s.lastSessionID = sessionID
	s.lastUserID = userID
	s.lastGroupID = nextGroupID

	if s.switchFn == nil {
		panic("switchFn is nil")
	}
	return s.switchFn(ctx, sessionID, userID, nextGroupID)
}

func withSwitchGroupSession(req *http.Request, sessionID, userID, groupID uuid.UUID) *http.Request {
	return req.WithContext(WithSession(req.Context(), SessionContext{
		SessionID:     sessionID,
		UserID:        userID,
		ActiveGroupID: groupID,
	}))
}

func TestAPIHandler_SwitchGroup(t *testing.T) {
	t.Parallel()

	sessionID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	currentGroupID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	targetGroupID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	tests := []struct {
		name             string
		withSession      bool
		body             string
		contentType      string
		switchFn         func(ctx context.Context, sid, uid, gid uuid.UUID) error
		getUserByIDFn    func(ctx context.Context, uid uuid.UUID) (*UserRow, error)
		wantStatus       int
		wantCode         string
		wantMessage      string
		wantSwitchCalled bool
		wantGetUserCall  bool
		assertBody       func(t *testing.T, body []byte)
	}{
		{
			name:        "given_valid_request_when_switch_group_then_returns_200",
			withSession: true,
			contentType: "application/json",
			body:        `{"groupId":"44444444-4444-4444-4444-444444444444"}`,
			switchFn: func(ctx context.Context, sid, uid, gid uuid.UUID) error {
				return nil
			},
			getUserByIDFn: func(ctx context.Context, uid uuid.UUID) (*UserRow, error) {
				return &UserRow{
					ID:          userID,
					Email:       "owner@example.com",
					DisplayName: "Owner User",
				}, nil
			},
			wantStatus:       http.StatusOK,
			wantSwitchCalled: true,
			wantGetUserCall:  true,
			assertBody: func(t *testing.T, body []byte) {
				var resp SwitchGroupPayload
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, userID.String(), resp.AuthContext.UserId.String())
				assert.Equal(t, "owner@example.com", string(resp.AuthContext.Email))
				assert.Equal(t, "Owner User", resp.AuthContext.DisplayName)
				assert.Equal(t, targetGroupID.String(), resp.AuthContext.ActiveGroupId.String())
			},
		},
		{
			name:             "given_missing_session_when_switch_group_then_returns_401",
			withSession:      false,
			contentType:      "application/json",
			body:             `{"groupId":"44444444-4444-4444-4444-444444444444"}`,
			wantStatus:       http.StatusUnauthorized,
			wantCode:         "unauthorized",
			wantMessage:      "missing session context",
			wantSwitchCalled: false,
			wantGetUserCall:  false,
		},
		{
			name:             "given_invalid_json_when_switch_group_then_returns_400",
			withSession:      true,
			contentType:      "application/json",
			body:             `{`,
			wantStatus:       http.StatusBadRequest,
			wantCode:         "bad_request",
			wantMessage:      "invalid json body",
			wantSwitchCalled: false,
			wantGetUserCall:  false,
		},
		{
			name:             "given_wrong_content_type_when_switch_group_then_returns_400",
			withSession:      true,
			contentType:      "text/plain",
			body:             `{"groupId":"44444444-4444-4444-4444-444444444444"}`,
			wantStatus:       http.StatusBadRequest,
			wantCode:         "bad_request",
			wantMessage:      "content-type must be application/json",
			wantSwitchCalled: false,
			wantGetUserCall:  false,
		},
		{
			name:             "given_missing_group_id_when_switch_group_then_returns_400",
			withSession:      true,
			contentType:      "application/json",
			body:             `{}`,
			wantStatus:       http.StatusBadRequest,
			wantCode:         "bad_request",
			wantMessage:      "groupId is required",
			wantSwitchCalled: false,
			wantGetUserCall:  false,
		},
		{
			name:        "given_invalid_session_error_from_store_when_switch_group_then_returns_401",
			withSession: true,
			contentType: "application/json",
			body:        `{"groupId":"44444444-4444-4444-4444-444444444444"}`,
			switchFn: func(ctx context.Context, sid, uid, gid uuid.UUID) error {
				return session.ErrInvalidSession
			},
			wantStatus:       http.StatusUnauthorized,
			wantCode:         "unauthorized",
			wantMessage:      "invalid session",
			wantSwitchCalled: true,
			wantGetUserCall:  false,
		},
		{
			name:        "given_group_forbidden_error_from_store_when_switch_group_then_returns_403",
			withSession: true,
			contentType: "application/json",
			body:        `{"groupId":"44444444-4444-4444-4444-444444444444"}`,
			switchFn: func(ctx context.Context, sid, uid, gid uuid.UUID) error {
				return session.ErrGroupForbidden
			},
			wantStatus:       http.StatusForbidden,
			wantCode:         "forbidden",
			wantMessage:      "group membership required",
			wantSwitchCalled: true,
			wantGetUserCall:  false,
		},
		{
			name:        "given_unexpected_store_error_when_switch_group_then_returns_500",
			withSession: true,
			contentType: "application/json",
			body:        `{"groupId":"44444444-4444-4444-4444-444444444444"}`,
			switchFn: func(ctx context.Context, sid, uid, gid uuid.UUID) error {
				return errors.New("db down")
			},
			wantStatus:       http.StatusInternalServerError,
			wantCode:         "internal_error",
			wantMessage:      "failed to switch group",
			wantSwitchCalled: true,
			wantGetUserCall:  false,
		},
		{
			name:        "given_user_not_found_after_successful_switch_when_switch_group_then_returns_401",
			withSession: true,
			contentType: "application/json",
			body:        `{"groupId":"44444444-4444-4444-4444-444444444444"}`,
			switchFn: func(ctx context.Context, sid, uid, gid uuid.UUID) error {
				return nil
			},
			getUserByIDFn: func(ctx context.Context, uid uuid.UUID) (*UserRow, error) {
				return nil, ErrNotFound
			},
			wantStatus:       http.StatusUnauthorized,
			wantCode:         "unauthorized",
			wantMessage:      "invalid session",
			wantSwitchCalled: true,
			wantGetUserCall:  true,
		},
		{
			name:        "given_user_lookup_error_after_successful_switch_when_switch_group_then_returns_500",
			withSession: true,
			contentType: "application/json",
			body:        `{"groupId":"44444444-4444-4444-4444-444444444444"}`,
			switchFn: func(ctx context.Context, sid, uid, gid uuid.UUID) error {
				return nil
			},
			getUserByIDFn: func(ctx context.Context, uid uuid.UUID) (*UserRow, error) {
				return nil, errors.New("db down")
			},
			wantStatus:       http.StatusInternalServerError,
			wantCode:         "internal_error",
			wantMessage:      "failed to build auth context",
			wantSwitchCalled: true,
			wantGetUserCall:  true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sessions := &switchGroupSessionStub{}
			if tc.switchFn != nil {
				sessions.switchFn = tc.switchFn
			} else {
				sessions.switchFn = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
					t.Fatalf("SwitchActiveGroup should not be called in case %q", tc.name)
					return nil
				}
			}

			repo := &switchGroupRepoStub{}
			if tc.getUserByIDFn != nil {
				repo.getUserByIDFn = tc.getUserByIDFn
			} else {
				repo.getUserByIDFn = func(context.Context, uuid.UUID) (*UserRow, error) {
					t.Fatalf("GetUserByID should not be called in case %q", tc.name)
					return nil, nil
				}
			}

			h := &APIHandler{
				repo:     repo,
				sessions: sessions,
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/switch-group", bytes.NewBufferString(tc.body))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			if tc.withSession {
				req = withSwitchGroupSession(req, sessionID, userID, currentGroupID)
			}

			rr := httptest.NewRecorder()
			h.SwitchGroup(rr, req)

			require.Equal(t, tc.wantStatus, rr.Code)
			assert.Equal(t, tc.wantSwitchCalled, sessions.called)
			assert.Equal(t, tc.wantGetUserCall, repo.getUserCalled)

			if tc.wantSwitchCalled {
				assert.Equal(t, sessionID, sessions.lastSessionID)
				assert.Equal(t, userID, sessions.lastUserID)
				assert.Equal(t, targetGroupID, sessions.lastGroupID)
			}
			if tc.wantGetUserCall {
				assert.Equal(t, userID, repo.lastUserID)
			}

			if tc.assertBody != nil {
				tc.assertBody(t, rr.Body.Bytes())
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
