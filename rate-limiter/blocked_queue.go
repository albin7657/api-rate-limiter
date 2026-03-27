package ratelimiter

import (
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type BlockedRequest struct {
	ID       uint64            `json:"id"`
	ClientID string            `json:"clientID"`
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Body     string            `json:"body,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	QueuedAt time.Time         `json:"queuedAt"`
}

type BlockedRequestStore struct {
	mu       sync.Mutex
	requests map[string][]BlockedRequest
	nextID   atomic.Uint64
}

func NewBlockedRequestStore() *BlockedRequestStore {
	return &BlockedRequestStore{requests: make(map[string][]BlockedRequest)}
}

func (s *BlockedRequestStore) EnqueueFromHTTP(clientID string, r *http.Request) BlockedRequest {
	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	bodyBytes, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))

	blocked := BlockedRequest{
		ID:       s.nextID.Add(1),
		ClientID: clientID,
		Method:   r.Method,
		Path:     r.URL.Path,
		Body:     string(bodyBytes),
		Headers:  headers,
		QueuedAt: time.Now(),
	}

	s.mu.Lock()
	s.requests[clientID] = append(s.requests[clientID], blocked)
	s.mu.Unlock()

	return blocked
}

func (s *BlockedRequestStore) List(clientID string) []BlockedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	q := s.requests[clientID]
	out := make([]BlockedRequest, len(q))
	copy(out, q)
	return out
}

func (s *BlockedRequestStore) PendingCount(clientID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests[clientID])
}

func (s *BlockedRequestStore) PopBatch(clientID string, count int) []BlockedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	q := s.requests[clientID]
	if len(q) == 0 {
		return nil
	}
	if count > len(q) {
		count = len(q)
	}

	batch := make([]BlockedRequest, count)
	copy(batch, q[:count])

	rest := q[count:]
	if len(rest) == 0 {
		delete(s.requests, clientID)
	} else {
		s.requests[clientID] = rest
	}

	return batch
}
