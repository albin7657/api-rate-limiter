package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"

	"api-rate-limiter/config"
	ratelimiter "api-rate-limiter/rate-limiter"
)

func RateLimitMiddleware(rl *ratelimiter.RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		clientID := r.Header.Get("X-Client-ID")
		if clientID == "" {
			clientID = r.RemoteAddr
		}

		allowed, err := rl.Allow(clientID, config.MaxRequests, config.WindowDuration)

		if err != nil {
			fmt.Println("Error:", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Anonymous struct
		response := struct {
			ClientID string `json:"client_id"`
			Allowed  bool   `json:"allowed"`
			Message  string `json:"message"`
		}{}

		if !allowed {
			fmt.Println("Rate limit exceeded for:", clientID)
			response.ClientID = clientID
			response.Allowed = false
			response.Message = "Rate limit exceeded"
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(response)
			return
		}

		fmt.Println("Request allowed for:", clientID)
		response.ClientID = clientID
		response.Allowed = true
		response.Message = "Request allowed"
		json.NewEncoder(w).Encode(response)

		next.ServeHTTP(w, r)
	})
}
