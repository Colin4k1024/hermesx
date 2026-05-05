package metering

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const (
	defaultBufferSize    = 100
	defaultFlushInterval = 5 * time.Second
	defaultChannelCap    = 1000
)

// RecorderOption configures a UsageRecorder.
type RecorderOption func(*recorderConfig)

type recorderConfig struct {
	bufferSize    int
	flushInterval time.Duration
	channelCap    int
}

// WithBufferSize sets the flush threshold for the recorder.
func WithBufferSize(n int) RecorderOption {
	return func(c *recorderConfig) { c.bufferSize = n }
}

// WithFlushInterval sets the timer-based flush interval.
func WithFlushInterval(d time.Duration) RecorderOption {
	return func(c *recorderConfig) { c.flushInterval = d }
}

// WithChannelCap sets the channel capacity.
func WithChannelCap(n int) RecorderOption {
	return func(c *recorderConfig) { c.channelCap = n }
}

// UsageRecorder asynchronously batches and persists usage records.
type UsageRecorder struct {
	ch     chan UsageRecord
	store  UsageStore
	done   chan struct{}
	once   sync.Once
	config recorderConfig
}

// NewUsageRecorder creates a new recorder that batches writes to store.
func NewUsageRecorder(store UsageStore, opts ...RecorderOption) *UsageRecorder {
	cfg := recorderConfig{
		bufferSize:    defaultBufferSize,
		flushInterval: defaultFlushInterval,
		channelCap:    defaultChannelCap,
	}
	for _, o := range opts {
		o(&cfg)
	}
	return &UsageRecorder{
		ch:     make(chan UsageRecord, cfg.channelCap),
		store:  store,
		done:   make(chan struct{}),
		config: cfg,
	}
}

// Record enqueues a usage record for async persistence.
// Non-blocking: drops the record and logs a warning if the channel is full.
func (r *UsageRecorder) Record(record UsageRecord) {
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	select {
	case r.ch <- record:
	default:
		slog.Warn("usage recorder channel full, dropping record",
			"tenant_id", record.TenantID,
			"session_id", record.SessionID)
	}
}

// Start begins the background flush goroutine. It blocks until ctx is cancelled
// or Stop is called.
func (r *UsageRecorder) Start(ctx context.Context) {
	ticker := time.NewTicker(r.config.flushInterval)
	defer ticker.Stop()

	buf := make([]UsageRecord, 0, r.config.bufferSize)

	flush := func() {
		if len(buf) == 0 {
			return
		}
		batch := make([]UsageRecord, len(buf))
		copy(batch, buf)
		buf = buf[:0]

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := r.store.BatchInsert(ctx, batch); err != nil {
			slog.Error("usage recorder flush failed", "error", err, "count", len(batch))
		}
	}

	for {
		select {
		case rec, ok := <-r.ch:
			if !ok {
				// Channel closed — drain complete.
				flush()
				return
			}
			buf = append(buf, rec)
			if len(buf) >= r.config.bufferSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-ctx.Done():
			// Context cancelled — drain remaining from channel then flush.
			r.drainAndFlush(buf)
			return
		}
	}
}

// Stop gracefully shuts down the recorder, draining buffered records.
func (r *UsageRecorder) Stop() {
	r.once.Do(func() {
		close(r.ch)
		close(r.done)
	})
}

// drainAndFlush reads all remaining records from the channel and flushes them.
func (r *UsageRecorder) drainAndFlush(buf []UsageRecord) {
	for {
		select {
		case rec, ok := <-r.ch:
			if !ok {
				goto done
			}
			buf = append(buf, rec)
		default:
			goto done
		}
	}
done:
	if len(buf) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := r.store.BatchInsert(ctx, buf); err != nil {
		slog.Error("usage recorder final flush failed", "error", err, "count", len(buf))
	}
}
