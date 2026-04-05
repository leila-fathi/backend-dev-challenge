package publicapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type getMembershipsRepoStub struct {
	handlerRepository

	listMembershipsFn func(ctx context.Context, userID uuid.UUID) ([]MembershipRow, error)

	called     bool
	lastUserID uuid.UUID
}

func (s *getMembershipsRepoStub) ListMemberships(ctx context.Context, userID uuid.UUID) ([]MembershipRow, error) {
	s.called = true
	s.lastUserID = userID

	if s.listMembershipsFn == nil {
		panic("listMembershipsFn is nil")
	}
	return s.listMembershipsFn(ctx, userID)
}

func withMembershipsSession(req *http.Request, userID, activeGroupID uuid.UUID) *http.Request {
	return req.WithContext(WithSession(req.Context(), SessionContext{
		SessionID:     uuid.New(),
		UserID:        userID,
		ActiveGroupID: activeGroupID,
	}))
}

func TestAPIHandler_GetMemberships(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	groupA := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	groupB := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	groupC := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	activeGroupID := groupB

	tests := []struct {
		name           string
		withSession    bool
		repoFn         func(ctx context.Context, userID uuid.UUID) ([]MembershipRow, error)
		wantStatus     int
		wantCode       string
		wantMessage    string
		wantRepoCalled bool
		assertBody     func(t *testing.T, body []byte)
	}{
		{
			name:        "given_valid_session_when_memberships_exist_then_returns_200_and_active_flag",
			withSession: true,
			repoFn: func(ctx context.Context, uid uuid.UUID) ([]MembershipRow, error) {
				return []MembershipRow{
					{GroupID: groupA, GroupName: "Group A", Role: "owner"},
					{GroupID: groupB, GroupName: "Group B", Role: "member"},
					{GroupID: groupC, GroupName: "Group C", Role: "viewer"}, // unknown role should fallback to member
				}, nil
			},
			wantStatus:     http.StatusOK,
			wantRepoCalled: true,
			assertBody: func(t *testing.T, body []byte) {
				var resp MembershipsResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				require.Len(t, resp.Memberships, 3)

				assert.Equal(t, groupA.String(), resp.Memberships[0].GroupId.String())
				assert.Equal(t, "Group A", resp.Memberships[0].GroupName)
				assert.Equal(t, Owner, resp.Memberships[0].Role)
				assert.False(t, resp.Memberships[0].Active)

				assert.Equal(t, groupB.String(), resp.Memberships[1].GroupId.String())
				assert.Equal(t, "Group B", resp.Memberships[1].GroupName)
				assert.Equal(t, Member, resp.Memberships[1].Role)
				assert.True(t, resp.Memberships[1].Active)

				assert.Equal(t, groupC.String(), resp.Memberships[2].GroupId.String())
				assert.Equal(t, "Group C", resp.Memberships[2].GroupName)
				assert.Equal(t, Member, resp.Memberships[2].Role) // fallback from "viewer"
				assert.False(t, resp.Memberships[2].Active)
			},
		},
		{
			name:        "given_valid_session_when_no_memberships_then_returns_200_empty_list",
			withSession: true,
			repoFn: func(ctx context.Context, uid uuid.UUID) ([]MembershipRow, error) {
				return []MembershipRow{}, nil
			},
			wantStatus:     http.StatusOK,
			wantRepoCalled: true,
			assertBody: func(t *testing.T, body []byte) {

				var resp MembershipsResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Len(t, resp.Memberships, 0)
			},
		},
		{
			name:           "given_missing_session_when_get_memberships_then_returns_401",
			withSession:    false,
			wantStatus:     http.StatusUnauthorized,
			wantCode:       "unauthorized",
			wantMessage:    "missing session context",
			wantRepoCalled: false,
		},
		{
			name:        "given_repository_error_when_get_memberships_then_returns_500",
			withSession: true,
			repoFn: func(ctx context.Context, uid uuid.UUID) ([]MembershipRow, error) {
				return nil, errors.New("db down")
			},
			wantStatus:     http.StatusInternalServerError,
			wantCode:       "internal_error",
			wantMessage:    "failed to list memberships",
			wantRepoCalled: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &getMembershipsRepoStub{}
			if tc.repoFn != nil {
				repo.listMembershipsFn = tc.repoFn
			} else {
				repo.listMembershipsFn = func(context.Context, uuid.UUID) ([]MembershipRow, error) {
					t.Fatalf("ListMemberships should not be called in case %q", tc.name)
					return nil, nil
				}
			}

			h := &APIHandler{repo: repo}

			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/memberships", nil)
			if tc.withSession {
				req = withMembershipsSession(req, userID, activeGroupID)
			}

			rr := httptest.NewRecorder()
			h.GetMemberships(rr, req)

			require.Equal(t, tc.wantStatus, rr.Code)
			assert.Equal(t, tc.wantRepoCalled, repo.called)

			if tc.wantRepoCalled {
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
