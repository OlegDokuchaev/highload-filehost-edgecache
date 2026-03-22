package unit

import (
	"context"
	"fmt"
	"strings"
)

// StubTokenVerifier нужен как заглушка для локальных тестов.
type StubTokenVerifier struct {
	AllowedTokens map[string]string
}

func NewStubTokenVerifier() *StubTokenVerifier {
	return &StubTokenVerifier{
		AllowedTokens: map[string]string{
			"test-token-user-1": "00000000-0000-0000-0000-000000000001",
			"test-token-user-2": "00000000-0000-0000-0000-000000000002",
		},
	}
}

func (v *StubTokenVerifier) Verify(ctx context.Context, authHeader string) (string, error) {
	_ = ctx
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", fmt.Errorf("token invalid")
	}

	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	userID, ok := v.AllowedTokens[token]
	if !ok {
		return "", fmt.Errorf("token invalid")
	}
	return userID, nil
}
