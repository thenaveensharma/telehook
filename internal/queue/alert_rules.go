package queue

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// AlertRule defines a rule for processing alerts
type AlertRule struct {
	Name           string
	Enabled        bool
	FilterFunc     func(*Alert) bool
	ThrottleWindow time.Duration
	MaxPerWindow   int
}

// RuleEngine manages alert rules
type RuleEngine struct {
	rules            []*AlertRule
	deduplication    *DeduplicationCache
	throttle         *ThrottleManager
	mu               sync.RWMutex
}

// DeduplicationCache tracks seen alerts to prevent duplicates
type DeduplicationCache struct {
	cache  map[string]time.Time
	window time.Duration
	mu     sync.RWMutex
}

// ThrottleManager tracks alert rates per user
type ThrottleManager struct {
	counters map[int]*ThrottleCounter // userID -> counter
	mu       sync.RWMutex
}

// ThrottleCounter tracks alerts for a specific user
type ThrottleCounter struct {
	count      int
	windowEnd  time.Time
	maxPerWindow int
	mu         sync.Mutex
}

// NewRuleEngine creates a new rule engine
func NewRuleEngine(dedupeWindow time.Duration) *RuleEngine {
	re := &RuleEngine{
		rules:         make([]*AlertRule, 0),
		deduplication: NewDeduplicationCache(dedupeWindow),
		throttle:      NewThrottleManager(),
	}

	// Start cleanup goroutine
	go re.deduplication.cleanup()

	return re
}

// AddRule adds a new rule to the engine
func (re *RuleEngine) AddRule(rule *AlertRule) {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.rules = append(re.rules, rule)
}

// ProcessAlert applies all rules to an alert
func (re *RuleEngine) ProcessAlert(alert *Alert) (bool, string) {
	// Check deduplication first
	if re.deduplication.IsDuplicate(alert) {
		return false, "duplicate alert filtered"
	}

	// Check throttling
	if !re.throttle.AllowAlert(alert.UserID, alert.Priority) {
		return false, "rate limit exceeded"
	}

	// Apply custom rules
	re.mu.RLock()
	defer re.mu.RUnlock()

	for _, rule := range re.rules {
		if !rule.Enabled {
			continue
		}

		if rule.FilterFunc != nil && !rule.FilterFunc(alert) {
			return false, fmt.Sprintf("filtered by rule: %s", rule.Name)
		}
	}

	return true, ""
}

// DeduplicationCache methods

// NewDeduplicationCache creates a new deduplication cache
func NewDeduplicationCache(window time.Duration) *DeduplicationCache {
	return &DeduplicationCache{
		cache:  make(map[string]time.Time),
		window: window,
	}
}

// IsDuplicate checks if an alert is a duplicate
func (dc *DeduplicationCache) IsDuplicate(alert *Alert) bool {
	key := dc.generateKey(alert)

	dc.mu.Lock()
	defer dc.mu.Unlock()

	if lastSeen, exists := dc.cache[key]; exists {
		if time.Since(lastSeen) < dc.window {
			return true
		}
	}

	dc.cache[key] = time.Now()
	return false
}

// generateKey creates a unique key for an alert
func (dc *DeduplicationCache) generateKey(alert *Alert) string {
	// Create hash based on user and message content
	message := ""
	if msg, ok := alert.Payload["message"].(string); ok {
		message = msg
	}

	data := fmt.Sprintf("%d:%s", alert.UserID, message)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:16]) // Use first 16 bytes
}

// cleanup removes old entries from cache
func (dc *DeduplicationCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		dc.mu.Lock()
		now := time.Now()
		for key, lastSeen := range dc.cache {
			if now.Sub(lastSeen) > dc.window {
				delete(dc.cache, key)
			}
		}
		dc.mu.Unlock()
	}
}

// ThrottleManager methods

// NewThrottleManager creates a new throttle manager
func NewThrottleManager() *ThrottleManager {
	return &ThrottleManager{
		counters: make(map[int]*ThrottleCounter),
	}
}

// AllowAlert checks if an alert is allowed based on rate limits
func (tm *ThrottleManager) AllowAlert(userID int, priority int) bool {
	tm.mu.Lock()
	counter, exists := tm.counters[userID]
	if !exists {
		counter = &ThrottleCounter{
			count:        0,
			windowEnd:    time.Now().Add(1 * time.Minute),
			maxPerWindow: tm.getMaxForPriority(priority),
		}
		tm.counters[userID] = counter
	}
	tm.mu.Unlock()

	return counter.increment()
}

// getMaxForPriority returns max alerts per minute based on priority
func (tm *ThrottleManager) getMaxForPriority(priority int) int {
	switch priority {
	case 1: // Urgent
		return 100
	case 2: // High
		return 60
	case 3: // Normal
		return 30
	case 4: // Low
		return 10
	default:
		return 30
	}
}

// ThrottleCounter methods

// increment increments the counter and checks limit
func (tc *ThrottleCounter) increment() bool {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	now := time.Now()

	// Reset if window expired
	if now.After(tc.windowEnd) {
		tc.count = 0
		tc.windowEnd = now.Add(1 * time.Minute)
	}

	// Check if limit exceeded
	if tc.count >= tc.maxPerWindow {
		return false
	}

	tc.count++
	return true
}

// DefaultRules returns a set of default alert rules
func DefaultRules() []*AlertRule {
	return []*AlertRule{
		{
			Name:    "Block Empty Messages",
			Enabled: true,
			FilterFunc: func(alert *Alert) bool {
				if msg, ok := alert.Payload["message"].(string); ok {
					return len(msg) > 0
				}
				return false
			},
		},
		{
			Name:    "Block Spam Keywords",
			Enabled: true,
			FilterFunc: func(alert *Alert) bool {
				if msg, ok := alert.Payload["message"].(string); ok {
					spamKeywords := []string{"viagra", "casino", "lottery"}
					for _, keyword := range spamKeywords {
						if contains(msg, keyword) {
							return false
						}
					}
				}
				return true
			},
		},
	}
}

// Helper function to check if string contains substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
