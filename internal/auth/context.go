package auth

import (
	"context"

	"habitual/internal/model"
)

type contextKey string

const userContextKey contextKey = "auth.user"

func ContextWithUser(ctx context.Context, user *model.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) *model.User {
	user, _ := ctx.Value(userContextKey).(*model.User)
	return user
}
