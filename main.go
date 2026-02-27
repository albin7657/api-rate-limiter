package main

import (
	"fmt"
	"net/http"

	"api-rate-limiter/middleware"
	ratelimiter "api-rate-limiter/rate-limiter"
)

func main() {

	rl := ratelimiter.NewRateLimiter()
	// Pointer demonstration
	value := 5

	ratelimiter.IncrementByValue(value)
	fmt.Println("After call by value:", value)

	ratelimiter.IncrementByPointer(&value)
	fmt.Println("After call by pointer:", value)

	// JSON demonstration
	client := ratelimiter.Client{RequestCount: 3}
	jsonData, _ := ratelimiter.ToJSON(client)
	fmt.Println("JSON:", string(jsonData))

	parsedClient, _ := ratelimiter.FromJSON(jsonData)
	fmt.Println("Parsed RequestCount:", parsedClient.RequestCount)

	// bcrypt demo
	err := rl.RegisterClient("client1", "mypassword")
	if err != nil {
		fmt.Println("Registration error:", err)
	}

	err = rl.Authenticate("client1", "mypassword")
	if err != nil {
		fmt.Println("Authentication failed")
	} else {
		fmt.Println("Authentication successful")
	}

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
