package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"api-rate-limiter/config"
	ratelimiter "api-rate-limiter/rate-limiter"
)

type TokenValidator func(token string) (string, error)

func RateLimitMiddleware(rl *ratelimiter.RateLimiter, validateToken TokenValidator, blockedStore *ratelimiter.BlockedRequestStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			respondWithError(w, "Authentication required. Please provide valid Bearer token.", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			respondWithError(w, "Invalid authentication token.", http.StatusUnauthorized)
			return
		}

		clientID, err := validateToken(strings.TrimSpace(token))
		if err != nil {
			respondWithError(w, "Invalid or expired token.", http.StatusUnauthorized)
			return
		}

		allowed, err := rl.Allow(clientID, config.MaxRequests, config.WindowDuration)

		if err != nil {
			fmt.Println("Error:", err)
			respondWithError(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Anonymous struct - JSON marshalling
		response := struct {
			ClientID string `json:"client_id"`
			Allowed  bool   `json:"allowed"`
			Message  string `json:"message"`
		}{}

		if !allowed {
			queued := blockedStore.EnqueueFromHTTP(clientID, r)
			pending := blockedStore.PendingCount(clientID)

			response.ClientID = clientID
			response.Allowed = false
			response.Message = "Rate limit exceeded. Request queued for replay."

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"client_id":           response.ClientID,
				"allowed":             response.Allowed,
				"message":             response.Message,
				"queued_request_id":   queued.ID,
				"queued_request_path": queued.Path,
				"pending":             pending,
			})
			return
		}

		r.Header.Set("X-Authenticated-Client-ID", clientID)
		next.ServeHTTP(w, r)
	})
}

// Helper function to respond with error (JSON marshalling)
func respondWithError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := struct {
		Error   string `json:"error"`
		Status  int    `json:"status"`
		Message string `json:"message"`
	}{
		Error:   http.StatusText(statusCode),
		Status:  statusCode,
		Message: message,
	}

	json.NewEncoder(w).Encode(errorResponse)
}
