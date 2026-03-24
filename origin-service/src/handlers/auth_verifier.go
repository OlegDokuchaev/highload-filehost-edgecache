package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

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


// RemoteTokenVerifier проверяет JWT-токен через HTTP-запрос к внешнему сервису.
type RemoteTokenVerifier struct {
	client *http.Client
	url    string // полный URL, включая ручку /auth/verify
}

// NewRemoteTokenVerifier создаёт новый экземпляр RemoteTokenVerifier.
// url — например, "http://auth-service/auth/verify".
func NewRemoteTokenVerifier(url string) *RemoteTokenVerifier {
	return &RemoteTokenVerifier{
		client: &http.Client{
			Timeout: 5 * time.Second, // разумный таймаут
		},
		url: url,
	}
}

// Verify реализует интерфейс проверки токена.
// authHeader должен содержать строку "Bearer <token>".
// Возвращает user_id, если токен активен, иначе ошибку.
func (v *RemoteTokenVerifier) Verify(ctx context.Context, authHeader string) (string, error) {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", fmt.Errorf("token invalid: missing Bearer prefix")
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))

	reqBody := map[string]string{"token": token}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("auth service call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth service returned HTTP %d", resp.StatusCode)
	}

	var respBody struct {
		Active          bool   `json:"active"`
		UserID          string `json:"user_id"`
		NormalizedLogin string `json:"normalized_login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if !respBody.Active {
		return "", fmt.Errorf("token inactive")
	}

	return respBody.UserID, nil
}