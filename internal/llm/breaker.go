package llm

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sony/gobreaker/v2"
)

var (
	breakerState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "hermesx_breaker_state",
		Help: "Circuit breaker state: 0=closed, 1=half-open, 2=open",
	}, []string{"model"})

	breakerRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "hermesx_breaker_requests_total",
		Help: "Total requests through circuit breaker by result",
	}, []string{"model", "result"})

	metricsOnce sync.Once
)

func init() {
	metricsOnce.Do(func() {
		prometheus.MustRegister(breakerState, breakerRequests)
	})
}

// ResilientTransport wraps a Transport with per-model circuit breaker protection.
type ResilientTransport struct {
	inner   Transport
	model   string
	breaker *gobreaker.CircuitBreaker[*ChatResponse]
}

// NewResilientTransport wraps a transport with circuit breaker.
func NewResilientTransport(inner Transport, model string) *ResilientTransport {
	settings := gobreaker.Settings{
		Name:        "llm-" + model,
		MaxRequests: 3,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5 ||
				(counts.Requests > 10 && counts.TotalFailures*2 > counts.Requests)
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			slog.Warn("circuit_breaker_state_change", "breaker", name, "from", from.String(), "to", to.String())
			breakerState.WithLabelValues(model).Set(float64(to))
		},
	}

	rt := &ResilientTransport{
		inner:   inner,
		model:   model,
		breaker: gobreaker.NewCircuitBreaker[*ChatResponse](settings),
	}
	breakerState.WithLabelValues(model).Set(0)
	return rt
}

func (r *ResilientTransport) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	resp, err := r.breaker.Execute(func() (*ChatResponse, error) {
		return r.inner.Chat(ctx, req)
	})
	if err != nil {
		breakerRequests.WithLabelValues(r.model, "failure").Inc()
		return nil, fmt.Errorf("circuit breaker [%s]: %w", r.breaker.Name(), err)
	}
	breakerRequests.WithLabelValues(r.model, "success").Inc()
	return resp, nil
}

func (r *ResilientTransport) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamDelta, <-chan error) {
	state := r.breaker.State()
	if state == gobreaker.StateOpen {
		breakerRequests.WithLabelValues(r.model, "failure").Inc()
		eCh := make(chan error, 1)
		eCh <- fmt.Errorf("circuit breaker open for %s", r.breaker.Name())
		close(eCh)
		dCh := make(chan StreamDelta)
		close(dCh)
		return dCh, eCh
	}

	deltaCh, errCh := r.inner.ChatStream(ctx, req)

	// Wrap error channel to track stream outcome for breaker failure counting.
	wrappedErrCh := make(chan error, 1)
	go func() {
		defer close(wrappedErrCh)
		select {
		case err, ok := <-errCh:
			if !ok {
				return
			}
			if err != nil {
				breakerRequests.WithLabelValues(r.model, "failure").Inc()
				_, _ = r.breaker.Execute(func() (*ChatResponse, error) {
					return nil, err
				})
			} else {
				breakerRequests.WithLabelValues(r.model, "success").Inc()
			}
			select {
			case wrappedErrCh <- err:
			case <-ctx.Done():
			}
		case <-ctx.Done():
		}
	}()

	return deltaCh, wrappedErrCh
}

func (r *ResilientTransport) Name() string { return r.inner.Name() + "+breaker" }

// BreakerState returns the current breaker state for health checks.
func (r *ResilientTransport) BreakerState() string {
	return r.breaker.State().String()
}

// BreakerName returns the breaker identifier.
func (r *ResilientTransport) BreakerName() string {
	return r.breaker.Name()
}

var _ Transport = (*ResilientTransport)(nil)
