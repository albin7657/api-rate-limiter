package ratelimiter

import (
	"errors"
	"sync"
	"time"
)

// Embedded struct
type Metadata struct {
	ClientID string
}

// Client struct
type Client struct {
	Metadata
	RequestCount int
	WindowStart  time.Time
	RequestLog   []time.Time
	PasswordHash string
}

// RateLimiterService interface unit-3
type RateLimiterService interface {
	Allow(clientID string, maxRequests int, window time.Duration) (bool, error)
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

func (rl *RateLimiter) Allow(clientID string, maxRequests int, window time.Duration) (bool, error) {

	if clientID == "" {
		return false, errors.New("client ID cannot be empty")
	}

	rl.Mutex.Lock()
	defer rl.Mutex.Unlock()

	client, exists := rl.Clients[clientID]

	if !exists {
		rl.Clients[clientID] = &Client{
			Metadata:     Metadata{ClientID: clientID},
			RequestCount: 1,
			WindowStart:  time.Now(),
			RequestLog:   make([]time.Time, 0),
		}
		return true, nil
	}

	if time.Since(client.WindowStart) > window {
		resetClient(client)
	}

	if client.RequestCount >= maxRequests {
		return false, nil
	}

	client.RequestCount++
	client.RequestLog = append(client.RequestLog, time.Now())

	return true, nil
}

// Helper function
func resetClient(client *Client) {
	client.RequestCount = 0
	client.WindowStart = time.Now()
	client.RequestLog = client.RequestLog[:0]
}
