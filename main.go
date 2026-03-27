package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"api-rate-limiter/config"
	"api-rate-limiter/middleware"
	ratelimiter "api-rate-limiter/rate-limiter"
)

func main() {

	rl := ratelimiter.NewRateLimiter()
	defer rl.Shutdown()
	blockedStore := ratelimiter.NewBlockedRequestStore()
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

	// bcrypt demo - register test client
	err := rl.RegisterClient("client1", "mypassword")
	if err != nil {
		fmt.Println("Registration error:", err)
	}

	// Register multiple clients
	clients := []string{"client1", "client2", "client3"}

	for _, c := range clients {
		err := rl.RegisterClient(c, "mypassword")
		if err != nil {
			fmt.Println("Registration error:", err)
		}
	}

	// API handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := struct {
			Message string `json:"message"`
			Status  string `json:"status"`
		}{
			Message: "API request successful",
			Status:  "ok",
		}
		json.NewEncoder(w).Encode(response)
	})

	// Replay queued blocked requests for current client
	serveBlockedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := parseBearerToken(r.Header.Get("Authorization"))
		if token == "" {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
			return
		}

		clientID, err := ratelimiter.ValidateJWT(token, config.JWTSecret)
		if err != nil {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
			return
		}

		batchSize := 1
		if rawCount := r.URL.Query().Get("count"); rawCount != "" {
			parsed, parseErr := strconv.Atoi(rawCount)
			if parseErr == nil && parsed > 0 {
				batchSize = parsed
			}
		}
		if batchSize > config.MaxServeBlockedBatch {
			batchSize = config.MaxServeBlockedBatch
		}

		pending := blockedStore.PendingCount(clientID)
		if pending == 0 {
			respondJSON(w, http.StatusOK, map[string]any{
				"clientID":     clientID,
				"served":       []map[string]any{},
				"served_count": 0,
				"pending":      0,
			})
			return
		}

		desiredToServe := batchSize
		if desiredToServe > pending {
			desiredToServe = pending
		}

		granted, retryAfterSeconds, reserveErr := rl.ReserveServeSlots(clientID, config.MaxRequests, config.WindowDuration, desiredToServe)
		if reserveErr != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": reserveErr.Error()})
			return
		}

		if granted == 0 {
			respondJSON(w, http.StatusTooManyRequests, map[string]any{
				"error":               "Rate limit window active. Wait for reset before serving blocked requests.",
				"retry_after_seconds": retryAfterSeconds,
				"pending":             pending,
			})
			return
		}

		served := make([]map[string]any, 0, granted)

		batch := blockedStore.PopBatch(clientID, granted)
		for _, req := range batch {
			served = append(served, map[string]any{
				"queued_request_id": req.ID,
				"path":              req.Path,
				"method":            req.Method,
				"queued_at":         req.QueuedAt,
				"status":            "served",
				"message":           "queued request served successfully",
			})
		}

		respondJSON(w, http.StatusOK, map[string]any{
			"clientID":     clientID,
			"served":       served,
			"served_count": len(served),
			"pending":      blockedStore.PendingCount(clientID),
		})
	})

	listBlockedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := parseBearerToken(r.Header.Get("Authorization"))
		if token == "" {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
			return
		}

		clientID, err := ratelimiter.ValidateJWT(token, config.JWTSecret)
		if err != nil {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
			return
		}

		respondJSON(w, http.StatusOK, map[string]any{
			"clientID": clientID,
			"pending":  blockedStore.PendingCount(clientID),
			"requests": blockedStore.List(clientID),
		})
	})

	// Authentication handler
	authHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		// Parse request body for authentication (JSON unmarshalling)
		var authRequest struct {
			ClientID string `json:"clientID"`
			Password string `json:"password"`
		}

		err := json.NewDecoder(r.Body).Decode(&authRequest)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request format",
			})
			return
		}

		// Authenticate using bcrypt
		err = rl.Authenticate(authRequest.ClientID, authRequest.Password)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid credentials",
			})
			return
		}

		token, err := ratelimiter.GenerateJWT(authRequest.ClientID, config.JWTSecret, config.JWTExpiry)
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to issue token"})
			return
		}

		fmt.Println("Authentication successful for client:", authRequest.ClientID)
		tokenResponse := struct {
			Token     string `json:"token"`
			ClientID  string `json:"clientID"`
			Message   string `json:"message"`
			ExpiresAt string `json:"expiresAt"`
		}{
			Token:     token,
			ClientID:  authRequest.ClientID,
			Message:   "Authentication successful",
			ExpiresAt: config.JWTExpiry.String(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(tokenResponse)
	})

	// Apply middleware
	finalHandler := middleware.CORSMiddleware(
		middleware.RateLimitMiddleware(rl, func(token string) (string, error) {
			return ratelimiter.ValidateJWT(token, config.JWTSecret)
		}, blockedStore, handler),
	)

	authFinalHandler := middleware.CORSMiddleware(authHandler)

	// Register routes
	http.Handle("/", finalHandler)
	http.Handle("/api/blocked", middleware.CORSMiddleware(listBlockedHandler))
	http.Handle("/api/blocked/serve", middleware.CORSMiddleware(serveBlockedHandler))
	http.Handle("/auth/login", authFinalHandler)

	listener, addr, err := listenOnAvailableAddress()
	if err != nil {
		fmt.Println("Server failed to start:", err)
		return
	}

	fmt.Printf("Server running at http://localhost%s\n", addr)
	fmt.Println("Rate limiter service with authentication enabled")

	if err := http.Serve(listener, nil); err != nil {
		fmt.Println("Server stopped:", err)
	}
}

func listenOnAvailableAddress() (net.Listener, string, error) {
	preferredPort := os.Getenv("PORT")
	if preferredPort == "" {
		preferredPort = "8080"
	}

	firstAddr := ":" + preferredPort
	if ln, err := net.Listen("tcp", firstAddr); err == nil {
		return ln, firstAddr, nil
	}

	for p := 8081; p <= 8090; p++ {
		addr := fmt.Sprintf(":%d", p)
		if ln, err := net.Listen("tcp", addr); err == nil {
			fmt.Printf("Preferred port %s busy. Falling back to %s\n", preferredPort, addr)
			return ln, addr, nil
		}
	}

	return nil, "", fmt.Errorf("no available ports in range 8080-8090")
}

func parseBearerToken(header string) string {
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

func respondJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
