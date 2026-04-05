package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type UserIdentity struct {
	UserID      uuid.UUID
	GroupID     uuid.UUID
	Email       string
	DisplayName string
}

type TokenClaims struct {
	Type         string
	UserID       uuid.UUID
	GroupID      uuid.UUID
	DefaultRole  string
	AllowedRoles []string
	Email        string
	DisplayName  string
	ExpiresAt    time.Time
}

type Manager struct {
	signingKey      []byte
	claimsNamespace string
	accessTokenTTL  time.Duration
}

func NewManager(signingKey, claimsNamespace string, accessTokenTTL time.Duration) *Manager {
	return &Manager{
		signingKey:      []byte(signingKey),
		claimsNamespace: claimsNamespace,
		accessTokenTTL:  accessTokenTTL,
	}
}

func (m *Manager) IssueAccessToken(identity UserIdentity) (string, time.Time, error) {
	expiresAt := time.Now().Add(m.accessTokenTTL)
	customClaims := map[string]any{
		"t":                      "user",
		"x-hasura-user-id":       identity.UserID.String(),
		"x-hasura-group-id":      identity.GroupID.String(),
		"x-hasura-default-role":  "user",
		"x-hasura-allowed-roles": []string{"user"},
		"u":                      identity.Email,
		"display_name":           identity.DisplayName,
	}

	claims := jwt.MapClaims{
		"sub":             identity.UserID.String(),
		"iat":             time.Now().Unix(),
		"exp":             expiresAt.Unix(),
		m.claimsNamespace: customClaims,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.signingKey)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

func (m *Manager) ParseAccessToken(rawToken string) (*TokenClaims, error) {
	parsedToken, err := jwt.Parse(rawToken, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}
		return m.signingKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}

	mapClaims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok || !parsedToken.Valid {
		return nil, errors.New("invalid token claims")
	}

	namespaceClaimsRaw, ok := mapClaims[m.claimsNamespace]
	if !ok {
		return nil, errors.New("missing claims namespace")
	}

	namespaceClaims, ok := namespaceClaimsRaw.(map[string]any)
	if !ok {
		return nil, errors.New("invalid namespaced claims")
	}

	userID, err := uuid.Parse(getString(namespaceClaims, "x-hasura-user-id"))
	if err != nil {
		return nil, err
	}
	groupID, err := uuid.Parse(getString(namespaceClaims, "x-hasura-group-id"))
	if err != nil {
		return nil, err
	}

	allowedRoles := make([]string, 0)
	if typedRoles, ok := namespaceClaims["x-hasura-allowed-roles"].([]string); ok {
		allowedRoles = append(allowedRoles, typedRoles...)
	} else if rawRoles, ok := namespaceClaims["x-hasura-allowed-roles"].([]any); ok {
		for _, role := range rawRoles {
			roleStr, ok := role.(string)
			if ok {
				allowedRoles = append(allowedRoles, roleStr)
			}
		}
	}

	expUnix, err := mapClaims.GetExpirationTime()
	if err != nil {
		return nil, err
	}

	return &TokenClaims{
		Type:         getString(namespaceClaims, "t"),
		UserID:       userID,
		GroupID:      groupID,
		DefaultRole:  getString(namespaceClaims, "x-hasura-default-role"),
		AllowedRoles: allowedRoles,
		Email:        getString(namespaceClaims, "u"),
		DisplayName:  getString(namespaceClaims, "display_name"),
		ExpiresAt:    expUnix.Time,
	}, nil
}

func ExtractBearerToken(value string) string {
	token := strings.TrimSpace(value)
	if token == "" {
		return ""
	}
	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimPrefix(token, "bearer ")
	return strings.TrimSpace(token)
}

func getString(values map[string]any, key string) string {
	raw, exists := values[key]
	if !exists {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return value
}
