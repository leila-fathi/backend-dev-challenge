package resolver

import (
	"hiring-challenge-backend/api/internal/auth"
	"hiring-challenge-backend/api/internal/hasura"
)

type Resolver struct {
	TokenManager *auth.Manager
	HasuraClient *hasura.Client
}
