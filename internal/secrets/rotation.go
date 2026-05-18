package secrets

import (
	"context"
	"regexp"
	"sync"
	"time"
)

type PatternSource interface {
	LoadPatterns(ctx context.Context) ([]PatternEntry, error)
}

type PatternWatcher struct {
	scanner  *LeakScanner
	source   PatternSource
	interval time.Duration
	mu       sync.Mutex
	cancel   context.CancelFunc
	done     chan struct{}
}

func NewPatternWatcher(scanner *LeakScanner, source PatternSource, interval time.Duration) *PatternWatcher {
	return &PatternWatcher{
		scanner:  scanner,
		source:   source,
		interval: interval,
	}
}

func (w *PatternWatcher) Start(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cancel != nil {
		return
	}

	ctx, w.cancel = context.WithCancel(ctx)
	w.done = make(chan struct{})

	go w.run(ctx)
}

func (w *PatternWatcher) Stop() {
	w.mu.Lock()
	if w.cancel != nil {
		w.cancel()
		w.cancel = nil
	}
	done := w.done
	w.mu.Unlock()

	if done != nil {
		<-done
	}
}

func (w *PatternWatcher) run(ctx context.Context) {
	defer close(w.done)

	w.reload(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.reload(ctx)
		}
	}
}

func (w *PatternWatcher) reload(ctx context.Context) {
	patterns, err := w.source.LoadPatterns(ctx)
	if err != nil {
		return
	}

	w.scanner.mu.Lock()
	defer w.scanner.mu.Unlock()

	existing := make(map[string]bool)
	for _, p := range w.scanner.patterns {
		existing[p.Name] = true
	}

	for _, p := range patterns {
		if !existing[p.Name] {
			w.scanner.patterns = append(w.scanner.patterns, p)
			w.scanner.regexVersion++
		}
	}
}

func (w *PatternWatcher) ForceReload(ctx context.Context) error {
	patterns, err := w.source.LoadPatterns(ctx)
	if err != nil {
		return err
	}

	w.scanner.mu.Lock()
	defer w.scanner.mu.Unlock()

	w.scanner.patterns = w.scanner.patterns[:0]
	w.scanner.loadBuiltinPatterns()
	for _, p := range patterns {
		w.scanner.patterns = append(w.scanner.patterns, p)
	}
	w.scanner.regexVersion++
	return nil
}

type StaticPatternSource struct {
	mu       sync.RWMutex
	patterns []PatternEntry
}

func NewStaticPatternSource() *StaticPatternSource {
	return &StaticPatternSource{}
}

func (s *StaticPatternSource) Add(name string, pattern *regexp.Regexp, severity Severity) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.patterns = append(s.patterns, PatternEntry{Name: name, Pattern: pattern, Severity: severity})
}

func (s *StaticPatternSource) LoadPatterns(_ context.Context) ([]PatternEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]PatternEntry, len(s.patterns))
	copy(out, s.patterns)
	return out, nil
}
