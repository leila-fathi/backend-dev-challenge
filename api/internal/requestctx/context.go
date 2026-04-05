package requestctx

import (
	"context"
)

type contextKey string

const (
	tokenClaimsKey contextKey = "token_claims"
)

func WithTokenClaims(ctx context.Context, claims any) context.Context {
	return context.WithValue(ctx, tokenClaimsKey, claims)
}

func TokenClaimsFromContext(ctx context.Context) (any, bool) {
	claims := ctx.Value(tokenClaimsKey)
	if claims == nil {
		return nil, false
	}
	return claims, true
}

func TypedTokenClaimsFromContext[T any](ctx context.Context) (*T, bool) {
	raw := ctx.Value(tokenClaimsKey)
	if raw == nil {
		return nil, false
	}
	claims, ok := raw.(*T)
	return claims, ok
}
