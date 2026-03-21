package handlers

import (
	"context"
	"fmt"
	"strings"
)

type TokenVerifier interface {
	Verify(ctx context.Context, authHeader string) (string, error)
}

type StubTokenVerifier struct {
	allowedTokens map[string]string
}

func NewStubTokenVerifier() *StubTokenVerifier {
	return &StubTokenVerifier{
		allowedTokens: map[string]string{
			"test-token-user-1": "00000000-0000-0000-0000-000000000001",
			"test-token-user-2": "00000000-0000-0000-0000-000000000002",
			"test-token-admin":  "00000000-0000-0000-0000-000000000099",
		},
	}
}

func (v *StubTokenVerifier) Verify(ctx context.Context, authHeader string) (string, error) {
	_ = ctx
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", fmt.Errorf("token invalid")
	}

	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		return "", fmt.Errorf("token invalid")
	}

	userID, ok := v.allowedTokens[token]
	if !ok {
		return "", fmt.Errorf("token invalid")
	}

	return userID, nil
}
