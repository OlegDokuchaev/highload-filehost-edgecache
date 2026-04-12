package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

type StubTokenVerifier struct {
	AllowedTokens map[string]string // token -> userID
	AllowedLogins map[string]string // token -> normalizedLogin
}

// NewStubTokenVerifier создаёт экземпляр Verifier с предопределёнными тестовыми данными.
func NewStubTokenVerifier() *StubTokenVerifier {
	return &StubTokenVerifier{
		AllowedTokens: map[string]string{
			"test-token-user-1": "00000000-0000-0000-0000-000000000001",
			"test-token-user-2": "00000000-0000-0000-0000-000000000002",
		},
		AllowedLogins: map[string]string{
			"test-token-user-1": "user1",
			"test-token-user-2": "user2",
		},
	}
}

// Verify проверяет заголовок Authorization и возвращает userID, если токен валиден.
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

// VerifyResponse — структура ответа ручки /auth/verify.
type VerifyResponse struct {
	Active          bool    `json:"active"`
	UserID          *string `json:"user_id,omitempty"`          // используется указатель для null в JSON
	NormalizedLogin *string `json:"normalized_login,omitempty"` // используется указатель для null в JSON
}

func main() {
	verifier := NewStubTokenVerifier()

	http.HandleFunc("/auth/verify", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendJSONResponse(w, VerifyResponse{Active: false})
			return
		}

		userID, err := verifier.Verify(r.Context(), "Bearer "+req.Token)
		if err != nil {
			sendJSONResponse(w, VerifyResponse{Active: false})
			return
		}

		login, ok := verifier.AllowedLogins[req.Token]
		if !ok {
			// Fallback: если логин не задан, используем userID
			login = userID
		}

		sendJSONResponse(w, VerifyResponse{
			Active:          true,
			UserID:          &userID,
			NormalizedLogin: &login,
		})
	})

	log.Println("Сервер запущен на :8081")
	// log.Fatal(http.ListenAndServe(":8081", nil))
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal("server failed", "error", err)
	}
}

// sendJSONResponse отправляет JSON‑ответ с кодом 200 OK.
func sendJSONResponse(w http.ResponseWriter, resp VerifyResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
