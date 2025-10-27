package middleware

import (
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

type RateLimiter struct {
	visitors map[string]*Visitor
	mu       sync.RWMutex
	limit    int
	window   time.Duration
}

type Visitor struct {
	lastSeen time.Time
	count    int
}

func NewRateLimiter() *RateLimiter {
	limit := 10
	if envLimit := os.Getenv("RATE_LIMIT"); envLimit != "" {
		if l, err := strconv.Atoi(envLimit); err == nil {
			limit = l
		}
	}

	rl := &RateLimiter{
		visitors: make(map[string]*Visitor),
		limit:    limit,
		window:   time.Minute,
	}

	// Cleanup old visitors every 5 minutes
	go rl.cleanup()

	return rl
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		for key, v := range rl.visitors {
			if time.Since(v.lastSeen) > rl.window {
				delete(rl.visitors, key)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Allow(identifier string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, exists := rl.visitors[identifier]

	if !exists {
		rl.visitors[identifier] = &Visitor{
			lastSeen: now,
			count:    1,
		}
		return true
	}

	if now.Sub(v.lastSeen) > rl.window {
		v.count = 1
		v.lastSeen = now
		return true
	}

	if v.count >= rl.limit {
		return false
	}

	v.count++
	v.lastSeen = now
	return true
}

func (rl *RateLimiter) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Use user_id from JWT if available, otherwise use IP
		identifier := c.IP()
		if userID := c.Locals("user_id"); userID != nil {
			identifier = strconv.Itoa(userID.(int))
		}

		if !rl.Allow(identifier) {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "rate limit exceeded, please try again later",
			})
		}

		return c.Next()
	}
}
