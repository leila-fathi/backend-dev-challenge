package publicapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"hiring-challenge-backend/api/internal/session"
	"hiring-challenge-backend/api/internal/zitadel"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var _ ServerInterface = (*APIHandler)(nil)

const maxJSONBodyBytes = 1024 * 1024 // 1 MiB

type handlerRepository interface {
	GetUserByID(ctx context.Context, userID uuid.UUID) (*UserRow, error)
	GetUserByEmail(ctx context.Context, email string) (*UserRow, error)
	GetUserByZitadelSubject(ctx context.Context, subject string) (*UserRow, error)
	ListMemberships(ctx context.Context, userID uuid.UUID) ([]MembershipRow, error)
	UserHasMembership(ctx context.Context, userID, groupID uuid.UUID) (bool, error)
	ListProjectsByGroup(ctx context.Context, groupID uuid.UUID) ([]ProjectRow, error)
	CreateProject(ctx context.Context, groupID, createdBy uuid.UUID, name string) (*ProjectRow, error)
}

type handlerSessionStore interface {
	Create(ctx context.Context, userID, groupID uuid.UUID) (string, *session.Record, error)
	SwitchActiveGroup(ctx context.Context, sessionID, userID, nextGroupID uuid.UUID) error
}

type APIHandler struct {
	repo        handlerRepository
	sessions    handlerSessionStore
	zitadelAuth zitadel.Verifier
}

func NewHandler(repo *Repository, sessions *session.Store, zitadelAuth zitadel.Verifier) *APIHandler {
	if repo == nil {
		panic("publicapi handler requires non-nil repository")
	}
	if sessions == nil {
		panic("publicapi handler requires non-nil session store")
	}
	if zitadelAuth == nil {
		panic("publicapi handler requires non-nil zitadel authenticator")
	}
	return &APIHandler{
		repo:        repo,
		sessions:    sessions,
		zitadelAuth: zitadelAuth,
	}
}

func (h *APIHandler) Login(w http.ResponseWriter, r *http.Request) {
	rawZitadelToken := extractBearerToken(r.Header.Get("Authorization"))
	if rawZitadelToken == "" {
		writeUnauthorized(w, "missing bearer token")
		return
	}

	identity, err := h.zitadelAuth.VerifyAccessToken(r.Context(), rawZitadelToken)
	if err != nil {
		if errors.Is(err, zitadel.ErrInvalidAccessToken) {
			writeUnauthorized(w, "invalid access token")
			return
		}
		log.Printf("zitadel token verify failed: err=%v", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "internal_error",
			Message: "internal server error",
		})
		return
	}

	h.loginByIdentity(w, r, identity)
}

func (h *APIHandler) loginByIdentity(w http.ResponseWriter, r *http.Request, identity *zitadel.Identity) {
	if identity == nil {
		writeUnauthorized(w, "invalid user identity")
		return
	}

	email := strings.TrimSpace(identity.Email)
	subject := strings.TrimSpace(identity.Subject)
	if email == "" && subject == "" {
		writeUnauthorized(w, "invalid user identity")
		return
	}

	var (
		user     *UserRow
		foundBy  string
		emailErr error
		subjErr  error
	)

	if email != "" {
		user, emailErr = h.repo.GetUserByEmail(r.Context(), email)
		if emailErr == nil {
			foundBy = "email"
		} else if !errors.Is(emailErr, ErrNotFound) {
			log.Printf("login user lookup by email failed: email=%s err=%v", email, emailErr)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Code:    "internal_error",
				Message: "internal server error",
			})
			return
		}
	}

	if user == nil && subject != "" {
		user, subjErr = h.repo.GetUserByZitadelSubject(r.Context(), subject)
		if subjErr == nil {
			foundBy = "subject"
		} else if !errors.Is(subjErr, ErrNotFound) {
			log.Printf("login user lookup by subject failed: subject=%s err=%v", subject, subjErr)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Code:    "internal_error",
				Message: "internal server error",
			})
			return
		}
	}

	if user == nil {
		writeJSON(w, http.StatusForbidden, ErrorResponse{
			Code:    "forbidden",
			Message: "user is not provisioned in local database",
		})
		return
	}

	// If we matched via subject and token has no email, keep response email sourced from local user record.
	if email == "" {
		email = user.Email
	}
	if foundBy == "" {
		foundBy = "unknown"
	}
	log.Printf("login identity matched local user: user_id=%s match_by=%s", user.ID, foundBy)

	ok, err := h.repo.UserHasMembership(r.Context(), user.ID, user.DefaultGroupID)
	if err != nil {
		log.Printf("login membership check failed: user_id=%s group_id=%s err=%v", user.ID, user.DefaultGroupID, err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "internal_error",
			Message: "internal server error",
		})
		return
	}
	if !ok {
		writeJSON(w, http.StatusForbidden, ErrorResponse{
			Code:    "forbidden",
			Message: "user has no membership in default group",
		})
		return
	}

	rawToken, rec, err := h.sessions.Create(r.Context(), user.ID, user.DefaultGroupID)
	if err != nil {
		log.Printf("login session create failed: user_id=%s err=%v", user.ID, err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "internal_error",
			Message: "internal server error",
		})
		return
	}

	writeJSON(w, http.StatusOK, LoginPayload{
		SessionToken: rawToken,
		AuthContext: AuthContext{
			UserId:        user.ID,
			Email:         openapi_types.Email(user.Email),
			DisplayName:   user.DisplayName,
			ActiveGroupId: rec.ActiveGroupID,
		},
	})
}

func (h *APIHandler) SwitchGroup(w http.ResponseWriter, r *http.Request) {
	sc, ok := SessionFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, "missing session context")
		return
	}

	var req SwitchGroupRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Code:    "bad_request",
			Message: err.Error(),
		})
		return
	}
	if req.GroupId == uuid.Nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Code:    "bad_request",
			Message: "groupId is required",
		})
		return
	}

	if err := h.sessions.SwitchActiveGroup(r.Context(), sc.SessionID, sc.UserID, req.GroupId); err != nil {
		switch {
		case errors.Is(err, session.ErrInvalidSession):
			writeUnauthorized(w, "invalid session")
		case errors.Is(err, session.ErrGroupForbidden):
			writeJSON(w, http.StatusForbidden, ErrorResponse{
				Code:    "forbidden",
				Message: "group membership required",
			})
		default:
			log.Printf("switch group failed: session_id=%s user_id=%s target_group=%s err=%v",
				sc.SessionID, sc.UserID, req.GroupId, err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Code:    "internal_error",
				Message: "failed to switch group",
			})
		}
		return
	}

	user, err := h.repo.GetUserByID(r.Context(), sc.UserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeUnauthorized(w, "invalid session")
			return
		}
		log.Printf("switch group user lookup failed: user_id=%s err=%v", sc.UserID, err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "internal_error",
			Message: "failed to build auth context",
		})
		return
	}

	writeJSON(w, http.StatusOK, SwitchGroupPayload{
		AuthContext: AuthContext{
			UserId:        user.ID,
			Email:         openapi_types.Email(user.Email),
			DisplayName:   user.DisplayName,
			ActiveGroupId: req.GroupId,
		},
	})
}

func (h *APIHandler) GetMemberships(w http.ResponseWriter, r *http.Request) {
	sc, ok := SessionFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, "missing session context")
		return
	}

	rows, err := h.repo.ListMemberships(r.Context(), sc.UserID)
	if err != nil {
		log.Printf("get memberships failed: user_id=%s err=%v", sc.UserID, err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "internal_error",
			Message: "failed to list memberships",
		})
		return
	}

	resp := MembershipsResponse{Memberships: make([]Membership, 0, len(rows))}
	for _, m := range rows {
		role := MembershipRole(m.Role)
		if role != Owner && role != Member {
			role = Member
		}
		resp.Memberships = append(resp.Memberships, Membership{
			GroupId:   m.GroupID,
			GroupName: m.GroupName,
			Role:      role,
			Active:    m.GroupID == sc.ActiveGroupID,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *APIHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	sc, ok := SessionFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, "missing session context")
		return
	}

	rows, err := h.repo.ListProjectsByGroup(r.Context(), sc.ActiveGroupID)
	if err != nil {
		log.Printf("list projects failed: group_id=%s err=%v", sc.ActiveGroupID, err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "internal_error",
			Message: "failed to list projects",
		})
		return
	}

	resp := ProjectsResponse{Projects: make([]Project, 0, len(rows))}
	for _, p := range rows {
		resp.Projects = append(resp.Projects, Project{
			Id:        p.ID,
			GroupId:   p.GroupID,
			Name:      p.Name,
			CreatedBy: p.CreatedBy,
			CreatedAt: p.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *APIHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	sc, ok := SessionFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, "missing session context")
		return
	}

	var req CreateProjectRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Code:    "bad_request",
			Message: err.Error(),
		})
		return
	}

	project, err := h.repo.CreateProject(r.Context(), sc.ActiveGroupID, sc.UserID, req.Name)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidProjectName):
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Code: "bad_request", Message: "invalid project name"})
		default:
			log.Printf("create project failed: user_id=%s group_id=%s err=%v", sc.UserID, sc.ActiveGroupID, err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Code:    "internal_error",
				Message: "failed to create project",
			})
		}
		return
	}

	writeJSON(w, http.StatusCreated, CreateProjectPayload{
		Project: Project{
			Id:        project.ID,
			GroupId:   project.GroupID,
			Name:      project.Name,
			CreatedBy: project.CreatedBy,
			CreatedAt: project.CreatedAt,
		},
	})
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	ct := strings.TrimSpace(r.Header.Get("Content-Type"))
	if ct != "" && !strings.HasPrefix(strings.ToLower(ct), "application/json") {
		return errors.New("content-type must be application/json")
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("request body is required")
		}
		return errors.New("invalid json body")
	}

	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("request body must contain a single JSON object")
	}

	return nil
}
