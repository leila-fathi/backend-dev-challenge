package publicapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"hiring-challenge-backend/api/internal/session"
	"hiring-challenge-backend/api/internal/zitadel"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type loginRepoStub struct {
	handlerRepository

	getUserByEmailFn        func(ctx context.Context, email string) (*UserRow, error)
	getUserBySubjectFn      func(ctx context.Context, subject string) (*UserRow, error)
	userHasMembershipFn     func(ctx context.Context, userID, groupID uuid.UUID) (bool, error)
	getUserByEmailCalled    bool
	getUserBySubjectCalled  bool
	userHasMembershipCalled bool
	lastEmail               string
	lastSubject             string
	lastMembershipUserID    uuid.UUID
	lastMembershipGroupID   uuid.UUID
}

func (s *loginRepoStub) GetUserByEmail(ctx context.Context, email string) (*UserRow, error) {
	s.getUserByEmailCalled = true
	s.lastEmail = email
	if s.getUserByEmailFn == nil {
		panic("getUserByEmailFn is nil")
	}
	return s.getUserByEmailFn(ctx, email)
}

func (s *loginRepoStub) GetUserByZitadelSubject(ctx context.Context, subject string) (*UserRow, error) {
	s.getUserBySubjectCalled = true
	s.lastSubject = subject
	if s.getUserBySubjectFn == nil {
		panic("getUserBySubjectFn is nil")
	}
	return s.getUserBySubjectFn(ctx, subject)
}

func (s *loginRepoStub) UserHasMembership(ctx context.Context, userID, groupID uuid.UUID) (bool, error) {
	s.userHasMembershipCalled = true
	s.lastMembershipUserID = userID
	s.lastMembershipGroupID = groupID
	if s.userHasMembershipFn == nil {
		panic("userHasMembershipFn is nil")
	}
	return s.userHasMembershipFn(ctx, userID, groupID)
}

type loginSessionStub struct {
	handlerSessionStore

	createFn      func(ctx context.Context, userID, groupID uuid.UUID) (string, *session.Record, error)
	createCalled  bool
	lastCreateUID uuid.UUID
	lastCreateGID uuid.UUID
}

func (s *loginSessionStub) Create(ctx context.Context, userID, groupID uuid.UUID) (string, *session.Record, error) {
	s.createCalled = true
	s.lastCreateUID = userID
	s.lastCreateGID = groupID
	if s.createFn == nil {
		panic("createFn is nil")
	}
	return s.createFn(ctx, userID, groupID)
}

type loginVerifierStub struct {
	verifyFn     func(ctx context.Context, rawToken string) (*zitadel.Identity, error)
	verifyCalled bool
	lastRawToken string
}

func (s *loginVerifierStub) VerifyAccessToken(ctx context.Context, rawToken string) (*zitadel.Identity, error) {
	s.verifyCalled = true
	s.lastRawToken = rawToken
	if s.verifyFn == nil {
		panic("verifyFn is nil")
	}
	return s.verifyFn(ctx, rawToken)
}

func TestAPIHandler_Login(t *testing.T) {
	t.Parallel()

	const (
		rawAccessToken  = "valid-zitadel-access-token"
		validAuthHeader = "Bearer valid-zitadel-access-token"
	)

	tests := []struct {
		name        string
		setAuth     bool
		authHeader  string
		override    func(t *testing.T, v *loginVerifierStub, r *loginRepoStub, s *loginSessionStub, user *UserRow, sessionID uuid.UUID)
		wantStatus  int
		wantCode    string
		wantMessage string
		check       func(t *testing.T, rr *httptest.ResponseRecorder, v *loginVerifierStub, r *loginRepoStub, s *loginSessionStub, user *UserRow, sessionID uuid.UUID)
	}{
		{
			name:        "given_missing_bearer_when_login_then_401",
			setAuth:     false,
			wantStatus:  http.StatusUnauthorized,
			wantCode:    "unauthorized",
			wantMessage: "missing bearer token",
			check: func(t *testing.T, _ *httptest.ResponseRecorder, v *loginVerifierStub, r *loginRepoStub, s *loginSessionStub, _ *UserRow, _ uuid.UUID) {
				assert.False(t, v.verifyCalled)
				assert.False(t, r.getUserByEmailCalled)
				assert.False(t, r.getUserBySubjectCalled)
				assert.False(t, r.userHasMembershipCalled)
				assert.False(t, s.createCalled)
			},
		},
		{
			name:       "given_invalid_access_token_when_login_then_401",
			setAuth:    true,
			authHeader: validAuthHeader,
			override: func(t *testing.T, v *loginVerifierStub, _ *loginRepoStub, _ *loginSessionStub, _ *UserRow, _ uuid.UUID) {
				v.verifyFn = func(context.Context, string) (*zitadel.Identity, error) {
					return nil, zitadel.ErrInvalidAccessToken
				}
			},
			wantStatus:  http.StatusUnauthorized,
			wantCode:    "unauthorized",
			wantMessage: "invalid access token",
			check: func(t *testing.T, _ *httptest.ResponseRecorder, v *loginVerifierStub, r *loginRepoStub, s *loginSessionStub, _ *UserRow, _ uuid.UUID) {
				assert.True(t, v.verifyCalled)
				assert.Equal(t, rawAccessToken, v.lastRawToken)
				assert.False(t, r.getUserByEmailCalled)
				assert.False(t, s.createCalled)
			},
		},
		{
			name:       "given_verifier_internal_error_when_login_then_500",
			setAuth:    true,
			authHeader: validAuthHeader,
			override: func(t *testing.T, v *loginVerifierStub, _ *loginRepoStub, _ *loginSessionStub, _ *UserRow, _ uuid.UUID) {
				v.verifyFn = func(context.Context, string) (*zitadel.Identity, error) {
					return nil, errors.New("zitadel unavailable")
				}
			},
			wantStatus:  http.StatusInternalServerError,
			wantCode:    "internal_error",
			wantMessage: "internal server error",
		},
		{
			name:       "given_nil_identity_when_login_then_401",
			setAuth:    true,
			authHeader: validAuthHeader,
			override: func(t *testing.T, v *loginVerifierStub, _ *loginRepoStub, _ *loginSessionStub, _ *UserRow, _ uuid.UUID) {
				v.verifyFn = func(context.Context, string) (*zitadel.Identity, error) {
					return nil, nil
				}
			},
			wantStatus:  http.StatusUnauthorized,
			wantCode:    "unauthorized",
			wantMessage: "invalid user identity",
		},
		{
			name:       "given_empty_identity_when_login_then_401",
			setAuth:    true,
			authHeader: validAuthHeader,
			override: func(t *testing.T, v *loginVerifierStub, _ *loginRepoStub, _ *loginSessionStub, _ *UserRow, _ uuid.UUID) {
				v.verifyFn = func(context.Context, string) (*zitadel.Identity, error) {
					return &zitadel.Identity{}, nil
				}
			},
			wantStatus:  http.StatusUnauthorized,
			wantCode:    "unauthorized",
			wantMessage: "invalid user identity",
		},
		{
			name:       "given_email_lookup_error_when_login_then_500",
			setAuth:    true,
			authHeader: validAuthHeader,
			override: func(t *testing.T, _ *loginVerifierStub, r *loginRepoStub, _ *loginSessionStub, _ *UserRow, _ uuid.UUID) {
				r.getUserByEmailFn = func(context.Context, string) (*UserRow, error) {
					return nil, errors.New("db down")
				}
			},
			wantStatus:  http.StatusInternalServerError,
			wantCode:    "internal_error",
			wantMessage: "internal server error",
		},
		{
			name:       "given_user_not_provisioned_when_login_then_403",
			setAuth:    true,
			authHeader: validAuthHeader,
			override: func(t *testing.T, _ *loginVerifierStub, r *loginRepoStub, _ *loginSessionStub, _ *UserRow, _ uuid.UUID) {
				r.getUserByEmailFn = func(context.Context, string) (*UserRow, error) {
					return nil, ErrNotFound
				}
				r.getUserBySubjectFn = func(context.Context, string) (*UserRow, error) {
					return nil, ErrNotFound
				}
			},
			wantStatus:  http.StatusForbidden,
			wantCode:    "forbidden",
			wantMessage: "user is not provisioned in local database",
		},
		{
			name:       "given_membership_check_error_when_login_then_500",
			setAuth:    true,
			authHeader: validAuthHeader,
			override: func(t *testing.T, _ *loginVerifierStub, r *loginRepoStub, _ *loginSessionStub, _ *UserRow, _ uuid.UUID) {
				r.userHasMembershipFn = func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
					return false, errors.New("membership query failed")
				}
			},
			wantStatus:  http.StatusInternalServerError,
			wantCode:    "internal_error",
			wantMessage: "internal server error",
		},
		{
			name:       "given_missing_default_group_membership_when_login_then_403",
			setAuth:    true,
			authHeader: validAuthHeader,
			override: func(t *testing.T, _ *loginVerifierStub, r *loginRepoStub, _ *loginSessionStub, _ *UserRow, _ uuid.UUID) {
				r.userHasMembershipFn = func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
					return false, nil
				}
			},
			wantStatus:  http.StatusForbidden,
			wantCode:    "forbidden",
			wantMessage: "user has no membership in default group",
		},
		{
			name:       "given_session_create_error_when_login_then_500",
			setAuth:    true,
			authHeader: validAuthHeader,
			override: func(t *testing.T, _ *loginVerifierStub, _ *loginRepoStub, s *loginSessionStub, _ *UserRow, _ uuid.UUID) {
				s.createFn = func(context.Context, uuid.UUID, uuid.UUID) (string, *session.Record, error) {
					return "", nil, errors.New("insert failed")
				}
			},
			wantStatus:  http.StatusInternalServerError,
			wantCode:    "internal_error",
			wantMessage: "internal server error",
		},
		{
			name:       "given_valid_identity_by_email_when_login_then_200",
			setAuth:    true,
			authHeader: validAuthHeader,
			wantStatus: http.StatusOK,
			check: func(t *testing.T, rr *httptest.ResponseRecorder, v *loginVerifierStub, r *loginRepoStub, s *loginSessionStub, user *UserRow, sessionID uuid.UUID) {
				assert.True(t, v.verifyCalled)
				assert.Equal(t, rawAccessToken, v.lastRawToken)
				assert.True(t, r.getUserByEmailCalled)
				assert.False(t, r.getUserBySubjectCalled)
				assert.True(t, r.userHasMembershipCalled)
				assert.True(t, s.createCalled)
				assert.Equal(t, user.ID, s.lastCreateUID)
				assert.Equal(t, user.DefaultGroupID, s.lastCreateGID)

				var payload LoginPayload
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))
				assert.Equal(t, "session-token", payload.SessionToken)
				assert.Equal(t, user.ID.String(), payload.AuthContext.UserId.String())
				assert.Equal(t, user.Email, string(payload.AuthContext.Email))
				assert.Equal(t, user.DisplayName, payload.AuthContext.DisplayName)
				assert.Equal(t, user.DefaultGroupID.String(), payload.AuthContext.ActiveGroupId.String())
			},
		},
		{
			name:       "given_identity_without_email_but_subject_match_when_login_then_200_and_email_from_local_user",
			setAuth:    true,
			authHeader: validAuthHeader,
			override: func(t *testing.T, v *loginVerifierStub, r *loginRepoStub, _ *loginSessionStub, user *UserRow, _ uuid.UUID) {
				v.verifyFn = func(context.Context, string) (*zitadel.Identity, error) {
					return &zitadel.Identity{
						Email:   "",
						Subject: "subject-only",
					}, nil
				}
				r.getUserByEmailFn = func(context.Context, string) (*UserRow, error) {
					t.Fatalf("GetUserByEmail should not be called when token email is empty")
					return nil, nil
				}
				r.getUserBySubjectFn = func(context.Context, string) (*UserRow, error) {
					return user, nil
				}
			},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, rr *httptest.ResponseRecorder, _ *loginVerifierStub, r *loginRepoStub, s *loginSessionStub, user *UserRow, _ uuid.UUID) {
				assert.False(t, r.getUserByEmailCalled)
				assert.True(t, r.getUserBySubjectCalled)
				assert.True(t, s.createCalled)

				var payload LoginPayload
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))
				assert.Equal(t, user.Email, string(payload.AuthContext.Email))
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			user := &UserRow{
				ID:             uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
				Email:          "owner@example.com",
				DisplayName:    "Owner User",
				DefaultGroupID: uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
			}
			sessionID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")

			verifier := &loginVerifierStub{
				verifyFn: func(context.Context, string) (*zitadel.Identity, error) {
					return &zitadel.Identity{
						Email:   user.Email,
						Subject: "subject-1",
					}, nil
				},
			}

			repo := &loginRepoStub{
				getUserByEmailFn: func(context.Context, string) (*UserRow, error) {
					return user, nil
				},
				getUserBySubjectFn: func(context.Context, string) (*UserRow, error) {
					return nil, ErrNotFound
				},
				userHasMembershipFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
					return true, nil
				},
			}

			sessions := &loginSessionStub{
				createFn: func(_ context.Context, uid, gid uuid.UUID) (string, *session.Record, error) {
					return "session-token", &session.Record{
						ID:            sessionID,
						UserID:        uid,
						ActiveGroupID: gid,
					}, nil
				},
			}

			if tc.override != nil {
				tc.override(t, verifier, repo, sessions, user, sessionID)
			}

			h := &APIHandler{
				repo:        repo,
				sessions:    sessions,
				zitadelAuth: verifier,
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
			if tc.setAuth {
				req.Header.Set("Authorization", tc.authHeader)
			}

			rr := httptest.NewRecorder()
			h.Login(rr, req)

			require.Equal(t, tc.wantStatus, rr.Code)

			if tc.wantStatus == http.StatusOK {
				if tc.check != nil {
					tc.check(t, rr, verifier, repo, sessions, user, sessionID)
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

			if tc.check != nil {
				tc.check(t, rr, verifier, repo, sessions, user, sessionID)
			}
		})
	}
}
