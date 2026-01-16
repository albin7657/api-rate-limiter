package ratelimiter

import (
	"sync"
	"time"
)

// Custom type for storing client data
type Client struct {
	RequestCount int
	WindowStart  time.Time
}

// RateLimiter structure
type RateLimiter struct {
	Clients map[string]*Client
	Mutex   sync.Mutex
}

// Constructor function
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		Clients: make(map[string]*Client),
	}
}

// Allow checks if request can be processed
func (rl *RateLimiter) Allow(clientID string, maxRequests int, window time.Duration) bool {
	rl.Mutex.Lock()
	defer rl.Mutex.Unlock()

	client, exists := rl.Clients[clientID]

	// New client → zero values applied
	if !exists {
		rl.Clients[clientID] = &Client{
			RequestCount: 1,
			WindowStart:  time.Now(),
		}
		return true
	}

	// Reset window if time expired
	if time.Since(client.WindowStart) > window {
		client.RequestCount = 1
		client.WindowStart = time.Now()
		return true
	}

	// Enforce rate limit
	if client.RequestCount >= maxRequests {
		return false
	}

	client.RequestCount++
	return true
}
