package queue

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/thenaveensharma/telehook/internal/models"
)

// Alert represents a queued alert message
type Alert struct {
	ID          string
	UserID      int
	Username    string
	Payload     map[string]interface{}
	Priority    int // 1=urgent, 2=high, 3=normal, 4=low
	Retries     int
	MaxRetries  int
	CreatedAt   time.Time
	ScheduledAt time.Time
	// Multi-channel routing fields
	BotToken    string // User's bot token for this alert
	ChannelID   string // Target channel ID
	DBChannelID int    // Database channel ID for logging
}

// AlertQueue manages the queue of alerts to be sent
type AlertQueue struct {
	queue         chan *Alert
	workers       int
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	processor     AlertProcessor
	retryQueue    chan *Alert
	batchQueue    chan []*Alert
	batchSize     int
	batchInterval time.Duration
	stats         *QueueStats
	mu            sync.RWMutex
}

// QueueStats tracks queue statistics
type QueueStats struct {
	Processed   int64
	Failed      int64
	Retried     int64
	Batched     int64
	CurrentSize int
	mu          sync.RWMutex
}

// AlertProcessor is the interface for processing alerts
type AlertProcessor interface {
	ProcessAlert(ctx context.Context, alert *Alert) error
	ProcessBatch(ctx context.Context, alerts []*Alert) error
}

// NewAlertQueue creates a new alert queue
func NewAlertQueue(workers int, queueSize int, processor AlertProcessor) *AlertQueue {
	ctx, cancel := context.WithCancel(context.Background())

	aq := &AlertQueue{
		queue:         make(chan *Alert, queueSize),
		workers:       workers,
		ctx:           ctx,
		cancel:        cancel,
		processor:     processor,
		retryQueue:    make(chan *Alert, queueSize/2),
		batchQueue:    make(chan []*Alert, 100),
		batchSize:     10,
		batchInterval: 5 * time.Second,
		stats:         &QueueStats{},
	}

	return aq
}

// Start initializes the worker pool
func (aq *AlertQueue) Start() {
	log.Printf("Starting alert queue with %d workers", aq.workers)

	// Start regular workers
	for i := 0; i < aq.workers; i++ {
		aq.wg.Add(1)
		go aq.worker(i)
	}

	// Start retry worker
	aq.wg.Add(1)
	go aq.retryWorker()

	// Start batch processor
	aq.wg.Add(1)
	go aq.batchProcessor()

	log.Println("Alert queue started successfully")
}

// Stop gracefully shuts down the queue
func (aq *AlertQueue) Stop() {
	log.Println("Stopping alert queue...")
	aq.cancel()
	close(aq.queue)
	aq.wg.Wait()
	log.Println("Alert queue stopped")
}

// Enqueue adds an alert to the queue
func (aq *AlertQueue) Enqueue(alert *Alert) error {
	// Set defaults
	if alert.CreatedAt.IsZero() {
		alert.CreatedAt = time.Now()
	}
	if alert.ScheduledAt.IsZero() {
		alert.ScheduledAt = time.Now()
	}
	if alert.MaxRetries == 0 {
		alert.MaxRetries = 3
	}
	if alert.Priority == 0 {
		alert.Priority = 3 // Default to normal priority
	}

	select {
	case aq.queue <- alert:
		aq.updateCurrentSize(1)
		return nil
	case <-aq.ctx.Done():
		return fmt.Errorf("queue is shutting down")
	default:
		return fmt.Errorf("queue is full")
	}
}

// EnqueueBatch adds multiple alerts for batch processing
func (aq *AlertQueue) EnqueueBatch(alerts []*Alert) error {
	select {
	case aq.batchQueue <- alerts:
		return nil
	case <-aq.ctx.Done():
		return fmt.Errorf("queue is shutting down")
	default:
		return fmt.Errorf("batch queue is full")
	}
}

// worker processes alerts from the queue
func (aq *AlertQueue) worker(id int) {
	defer aq.wg.Done()

	log.Printf("Worker %d started", id)

	for {
		select {
		case alert, ok := <-aq.queue:
			if !ok {
				log.Printf("Worker %d stopping", id)
				return
			}

			aq.updateCurrentSize(-1)
			aq.processAlert(alert, id)

		case <-aq.ctx.Done():
			log.Printf("Worker %d received shutdown signal", id)
			return
		}
	}
}

// processAlert handles individual alert processing
func (aq *AlertQueue) processAlert(alert *Alert, workerID int) {
	// Wait until scheduled time
	if time.Now().Before(alert.ScheduledAt) {
		time.Sleep(time.Until(alert.ScheduledAt))
	}

	// Process the alert
	err := aq.processor.ProcessAlert(aq.ctx, alert)
	if err != nil {
		log.Printf("Worker %d: Failed to process alert %s: %v", workerID, alert.ID, err)
		aq.stats.IncrementFailed()

		// Retry if possible
		if alert.Retries < alert.MaxRetries {
			aq.scheduleRetry(alert)
		} else {
			log.Printf("Alert %s exceeded max retries (%d)", alert.ID, alert.MaxRetries)
		}
	} else {
		aq.stats.IncrementProcessed()
	}
}

// scheduleRetry schedules an alert for retry with exponential backoff
func (aq *AlertQueue) scheduleRetry(alert *Alert) {
	alert.Retries++
	aq.stats.IncrementRetried()

	// Exponential backoff: 2^retries seconds
	backoffSeconds := 1 << alert.Retries // 2, 4, 8, 16...
	alert.ScheduledAt = time.Now().Add(time.Duration(backoffSeconds) * time.Second)

	log.Printf("Scheduling retry %d/%d for alert %s in %d seconds",
		alert.Retries, alert.MaxRetries, alert.ID, backoffSeconds)

	select {
	case aq.retryQueue <- alert:
	case <-aq.ctx.Done():
		return
	default:
		log.Printf("Retry queue full, dropping alert %s", alert.ID)
	}
}

// retryWorker handles retries
func (aq *AlertQueue) retryWorker() {
	defer aq.wg.Done()

	log.Println("Retry worker started")

	for {
		select {
		case alert, ok := <-aq.retryQueue:
			if !ok {
				log.Println("Retry worker stopping")
				return
			}

			// Re-enqueue the alert
			if err := aq.Enqueue(alert); err != nil {
				log.Printf("Failed to re-enqueue alert %s: %v", alert.ID, err)
			}

		case <-aq.ctx.Done():
			log.Println("Retry worker received shutdown signal")
			return
		}
	}
}

// batchProcessor handles batch processing
func (aq *AlertQueue) batchProcessor() {
	defer aq.wg.Done()

	log.Println("Batch processor started")

	ticker := time.NewTicker(aq.batchInterval)
	defer ticker.Stop()

	var currentBatch []*Alert

	for {
		select {
		case alerts, ok := <-aq.batchQueue:
			if !ok {
				// Process remaining batch before stopping
				if len(currentBatch) > 0 {
					aq.processBatch(currentBatch)
				}
				log.Println("Batch processor stopping")
				return
			}

			currentBatch = append(currentBatch, alerts...)

			// Process if batch size reached
			if len(currentBatch) >= aq.batchSize {
				aq.processBatch(currentBatch)
				currentBatch = nil
			}

		case <-ticker.C:
			// Process batch on timer
			if len(currentBatch) > 0 {
				aq.processBatch(currentBatch)
				currentBatch = nil
			}

		case <-aq.ctx.Done():
			if len(currentBatch) > 0 {
				aq.processBatch(currentBatch)
			}
			log.Println("Batch processor received shutdown signal")
			return
		}
	}
}

// processBatch processes a batch of alerts
func (aq *AlertQueue) processBatch(alerts []*Alert) {
	log.Printf("Processing batch of %d alerts", len(alerts))

	err := aq.processor.ProcessBatch(aq.ctx, alerts)
	if err != nil {
		log.Printf("Batch processing failed: %v", err)
		aq.stats.IncrementFailed()

		// Fall back to individual processing
		for _, alert := range alerts {
			if err := aq.Enqueue(alert); err != nil {
				log.Printf("Failed to re-enqueue alert from batch: %v", err)
			}
		}
	} else {
		aq.stats.AddBatched(int64(len(alerts)))
		aq.stats.AddProcessed(int64(len(alerts)))
	}
}

// GetStats returns current queue statistics
func (aq *AlertQueue) GetStats() models.QueueStats {
	aq.stats.mu.RLock()
	defer aq.stats.mu.RUnlock()

	return models.QueueStats{
		Processed:   aq.stats.Processed,
		Failed:      aq.stats.Failed,
		Retried:     aq.stats.Retried,
		Batched:     aq.stats.Batched,
		CurrentSize: aq.stats.CurrentSize,
	}
}

// updateCurrentSize updates the current queue size
func (aq *AlertQueue) updateCurrentSize(delta int) {
	aq.stats.mu.Lock()
	defer aq.stats.mu.Unlock()
	aq.stats.CurrentSize += delta
	if aq.stats.CurrentSize < 0 {
		aq.stats.CurrentSize = 0
	}
}

// Stats methods
func (qs *QueueStats) IncrementProcessed() {
	qs.mu.Lock()
	defer qs.mu.Unlock()
	qs.Processed++
}

func (qs *QueueStats) IncrementFailed() {
	qs.mu.Lock()
	defer qs.mu.Unlock()
	qs.Failed++
}

func (qs *QueueStats) IncrementRetried() {
	qs.mu.Lock()
	defer qs.mu.Unlock()
	qs.Retried++
}

func (qs *QueueStats) AddBatched(count int64) {
	qs.mu.Lock()
	defer qs.mu.Unlock()
	qs.Batched += count
}

func (qs *QueueStats) AddProcessed(count int64) {
	qs.mu.Lock()
	defer qs.mu.Unlock()
	qs.Processed += count
}
