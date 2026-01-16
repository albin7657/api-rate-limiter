package middleware

import (
	"fmt"
	"net/http"

	"api-rate-limiter/config"
	"api-rate-limiter/rate-limiter"
)

func RateLimitMiddleware(rl *ratelimiter.RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Short declaration operator
		clientID := r.RemoteAddr

		allowed := rl.Allow(clientID, config.MaxRequests, config.WindowDuration)

		if !allowed {
			fmt.Println("Rate limit exceeded for:", clientID)
			http.Error(w, "Too many requests. Please try again later.", http.StatusTooManyRequests)
			return
		}

		fmt.Println("Request allowed for:", clientID)
		next.ServeHTTP(w, r)
	})
}
