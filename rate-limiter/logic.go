package ratelimiter

import (
	"errors"
	"sync"
	"sync/atomic"
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
	LastSeen     time.Time
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
	Mutex   sync.RWMutex

	allowedCount atomic.Uint64
	blockedCount atomic.Uint64

	stopCleanup chan struct{}
	cleanupDone chan struct{}
}

type RateLimiterStats struct {
	AllowedRequests uint64
	BlockedRequests uint64
	ActiveClients   int
}

// Constructor using make()
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		Clients:     make(map[string]*Client),
		stopCleanup: make(chan struct{}),
		cleanupDone: make(chan struct{}),
	}

	go rl.startCleanupWorker(30*time.Second, 5*time.Minute)

	return rl
}

func (rl *RateLimiter) startCleanupWorker(interval, idleThreshold time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer close(rl.cleanupDone)

	for {
		select {
		case <-ticker.C:
			rl.cleanupInactiveClients(idleThreshold)
		case <-rl.stopCleanup:
			return
		}
	}
}

func (rl *RateLimiter) cleanupInactiveClients(idleThreshold time.Duration) {
	now := time.Now()

	rl.Mutex.Lock()
	defer rl.Mutex.Unlock()

	for clientID, client := range rl.Clients {
		if now.Sub(client.LastSeen) > idleThreshold {
			delete(rl.Clients, clientID)
		}
	}
}

func (rl *RateLimiter) Shutdown() {
	select {
	case <-rl.cleanupDone:
		return
	default:
		close(rl.stopCleanup)
		<-rl.cleanupDone
	}
}

func (rl *RateLimiter) Stats() RateLimiterStats {
	rl.Mutex.RLock()
	activeClients := len(rl.Clients)
	rl.Mutex.RUnlock()

	return RateLimiterStats{
		AllowedRequests: rl.allowedCount.Load(),
		BlockedRequests: rl.blockedCount.Load(),
		ActiveClients:   activeClients,
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
			LastSeen:     time.Now(),
			RequestLog:   make([]time.Time, 0),
		}
		rl.allowedCount.Add(1)
		return true, nil
	}

	if time.Since(client.WindowStart) > window {
		resetClient(client)
	}

	if client.RequestCount >= maxRequests {
		rl.blockedCount.Add(1)
		return false, nil
	}

	client.RequestCount++
	client.LastSeen = time.Now()
	client.RequestLog = append(client.RequestLog, time.Now())
	rl.allowedCount.Add(1)

	return true, nil
}

func (rl *RateLimiter) ReserveServeSlots(clientID string, maxRequests int, window time.Duration, desired int) (int, int, error) {
	if clientID == "" {
		return 0, 0, errors.New("client ID cannot be empty")
	}
	if maxRequests <= 0 {
		return 0, 0, errors.New("max requests must be greater than zero")
	}
	if desired <= 0 {
		return 0, 0, nil
	}

	rl.Mutex.Lock()
	defer rl.Mutex.Unlock()

	now := time.Now()
	client, exists := rl.Clients[clientID]
	if !exists {
		client = &Client{
			Metadata:     Metadata{ClientID: clientID},
			RequestCount: 0,
			WindowStart:  now,
			LastSeen:     now,
			RequestLog:   make([]time.Time, 0),
		}
		rl.Clients[clientID] = client
	}

	if now.Sub(client.WindowStart) > window {
		resetClient(client)
	}

	remaining := maxRequests - client.RequestCount
	if remaining <= 0 {
		retryAfter := window - now.Sub(client.WindowStart)
		if retryAfter < 0 {
			retryAfter = 0
		}

		retryAfterSeconds := int(retryAfter / time.Second)
		if retryAfter%time.Second != 0 {
			retryAfterSeconds++
		}
		return 0, retryAfterSeconds, nil
	}

	granted := desired
	if granted > remaining {
		granted = remaining
	}

	for i := 0; i < granted; i++ {
		client.RequestLog = append(client.RequestLog, now)
	}
	client.RequestCount += granted
	client.LastSeen = now
	rl.allowedCount.Add(uint64(granted))

	return granted, 0, nil
}

// Helper function
func resetClient(client *Client) {
	client.RequestCount = 0
	client.WindowStart = time.Now()
	client.LastSeen = time.Now()
	client.RequestLog = client.RequestLog[:0]
}
