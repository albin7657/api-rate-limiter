package ratelimiter

import (
	"sync"
	"time"
)

// Embedded struct
type Metadata struct {
	ClientID string
}

// Client struct (Struct + Embedded Struct)
type Client struct {
	Metadata
	RequestCount int
	WindowStart  time.Time
	RequestLog   []time.Time // Slice
}

// RateLimiter struct
type RateLimiter struct {
	Clients map[string]*Client // Map
	Mutex   sync.Mutex
}

// Constructor using make()
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		Clients: make(map[string]*Client),
	}
}

func (rl *RateLimiter) Allow(clientID string, maxRequests int, window time.Duration) bool {
	rl.Mutex.Lock()
	defer rl.Mutex.Unlock()

	client, exists := rl.Clients[clientID]

	if !exists {
		rl.Clients[clientID] = &Client{
			Metadata:     Metadata{ClientID: clientID},
			RequestCount: 1,
			WindowStart:  time.Now(),
			RequestLog:   make([]time.Time, 0), // make() slice
		}
		rl.Clients[clientID].RequestLog = append(
			rl.Clients[clientID].RequestLog, time.Now(),
		)
		return true
	}

	// Reset window
	if time.Since(client.WindowStart) > window {
		client.RequestCount = 0
		client.WindowStart = time.Now()
		client.RequestLog = client.RequestLog[:0] // delete slice data
	}

	// Enforce limit
	if client.RequestCount >= maxRequests {
		return false
	}

	client.RequestCount++
	client.RequestLog = append(client.RequestLog, time.Now())
	return true
}
