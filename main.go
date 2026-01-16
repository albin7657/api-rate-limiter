package main

import (
	"fmt"
	"net/http"

	"api-rate-limiter/middleware"
	ratelimiter "api-rate-limiter/rate-limiter"
)

func main() {

	rl := ratelimiter.NewRateLimiter()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "API request successful")
	})

	// Apply CORS → Rate Limiter → Handler
	finalHandler := middleware.CORSMiddleware(
		middleware.RateLimitMiddleware(rl, handler),
	)

	http.Handle("/", finalHandler)

	fmt.Println("Server running at http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
