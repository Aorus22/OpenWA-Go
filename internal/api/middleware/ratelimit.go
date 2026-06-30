package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter provides simple in-memory rate limiting.
type RateLimiter struct {
	mu       sync.Mutex
	requests map[string]*rateEntry
	ttl      time.Duration
	max      int
}

type rateEntry struct {
	count    int
	resetAt  time.Time
}

func NewRateLimiter(ttl time.Duration, max int) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*rateEntry),
		ttl:      ttl,
		max:      max,
	}
	// Periodic cleanup
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			rl.mu.Lock()
			now := time.Now()
			for k, v := range rl.requests {
				if now.After(v.resetAt) {
					delete(rl.requests, k)
				}
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

func (rl *RateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		if apiKeyID, exists := c.Get("apiKeyID"); exists {
			key = apiKeyID.(string)
		}

		rl.mu.Lock()
		entry, exists := rl.requests[key]
		now := time.Now()

		if !exists || now.After(entry.resetAt) {
			rl.requests[key] = &rateEntry{
				count:   1,
				resetAt: now.Add(rl.ttl),
			}
			rl.mu.Unlock()
			c.Next()
			return
		}

		entry.count++
		if entry.count > rl.max {
			rl.mu.Unlock()
			c.Header("Retry-After", fmt.Sprintf("%.0f", entry.resetAt.Sub(now).Seconds()))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
			})
			return
		}
		rl.mu.Unlock()
		c.Next()
	}
}
