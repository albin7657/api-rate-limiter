package main

import (
	"fmt"
	"net/http"

	"api-rate-limiter/middleware"
	"api-rate-limiter/rate-limiter"
)

func main() {

	// Create rate limiter instance
	rl := ratelimiter.NewRateLimiter()

	// API handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "API request successful")
	})

	// Apply rate limiting middleware
	http.Handle("/", middleware.RateLimitMiddleware(rl, handler))

	fmt.Println("Server running at http://localhost:8080")

	// Start server
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Server error:", err)
	}
}
