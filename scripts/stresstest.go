// stress_test.go — concurrent load test for /v1/agent/chat
//
// Usage: go run scripts/stress_test.go -c 10 -n 50 -url http://localhost:18080
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type result struct {
	status   int
	latency  time.Duration
	err      error
	tokens   int
	response string
}

func main() {
	concurrency := flag.Int("c", 10, "concurrent workers")
	total := flag.Int("n", 50, "total requests")
	baseURL := flag.String("url", "http://localhost:18080", "base URL")
	apiKey := flag.String("key", "", "API key (required)")
	model := flag.String("model", "MiniMax-M2.7", "model name")
	prompt := flag.String("prompt", "What is 2+2? Reply with just the number.", "test prompt")
	stream := flag.Bool("stream", false, "use SSE streaming")
	flag.Parse()

	if *apiKey == "" {
		fmt.Fprintln(os.Stderr, "ERROR: -key is required")
		os.Exit(1)
	}

	endpoint := *baseURL + "/v1/agent/chat"
	fmt.Printf("═══════════════════════════════════════════════════════════\n")
	fmt.Printf("  Hermes Agent Stress Test\n")
	fmt.Printf("═══════════════════════════════════════════════════════════\n")
	fmt.Printf("  Target:      %s\n", endpoint)
	fmt.Printf("  Model:       %s\n", *model)
	fmt.Printf("  Concurrency: %d\n", *concurrency)
	fmt.Printf("  Total:       %d requests\n", *total)
	fmt.Printf("  Stream:      %v\n", *stream)
	fmt.Printf("  Prompt:      %q\n", *prompt)
	fmt.Printf("═══════════════════════════════════════════════════════════\n\n")

	body, _ := json.Marshal(map[string]interface{}{
		"messages": []map[string]string{{"role": "user", "content": *prompt}},
		"model":    *model,
		"stream":   *stream,
	})

	results := make([]result, *total)
	var wg sync.WaitGroup
	var completed int64
	sem := make(chan struct{}, *concurrency)

	client := &http.Client{Timeout: 120 * time.Second}
	start := time.Now()

	for i := 0; i < *total; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			reqStart := time.Now()
			req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+*apiKey)

			resp, err := client.Do(req)
			if err != nil {
				results[idx] = result{err: err, latency: time.Since(reqStart)}
				n := atomic.AddInt64(&completed, 1)
				fmt.Printf("\r  Progress: %d/%d", n, *total)
				return
			}
			defer resp.Body.Close()
			respBody, _ := io.ReadAll(resp.Body)
			latency := time.Since(reqStart)

			r := result{status: resp.StatusCode, latency: latency}
			if resp.StatusCode == 200 && !*stream {
				var chatResp struct {
					Usage struct {
						TotalTokens int `json:"total_tokens"`
					} `json:"usage"`
					Choices []struct {
						Message struct {
							Content string `json:"content"`
						} `json:"message"`
					} `json:"choices"`
				}
				if json.Unmarshal(respBody, &chatResp) == nil {
					r.tokens = chatResp.Usage.TotalTokens
					if len(chatResp.Choices) > 0 {
						r.response = chatResp.Choices[0].Message.Content
					}
				}
			} else if resp.StatusCode != 200 {
				r.response = string(respBody)
			}
			results[idx] = r

			n := atomic.AddInt64(&completed, 1)
			fmt.Printf("\r  Progress: %d/%d", n, *total)
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)
	fmt.Printf("\r  Progress: %d/%d — Done!\n\n", *total, *total)

	// Analyze results
	var (
		success    int
		fail       int
		errCount   int
		latencies  []time.Duration
		totalTok   int
		statusDist = map[int]int{}
	)

	for _, r := range results {
		if r.err != nil {
			errCount++
			continue
		}
		statusDist[r.status]++
		if r.status == 200 {
			success++
			latencies = append(latencies, r.latency)
			totalTok += r.tokens
		} else {
			fail++
		}
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	fmt.Printf("═══════════════════════════════════════════════════════════\n")
	fmt.Printf("  RESULTS\n")
	fmt.Printf("═══════════════════════════════════════════════════════════\n")
	fmt.Printf("  Duration:    %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("  Throughput:  %.2f req/s\n", float64(*total)/elapsed.Seconds())
	fmt.Printf("  Success:     %d / %d (%.1f%%)\n", success, *total, float64(success)/float64(*total)*100)
	fmt.Printf("  Failures:    %d (HTTP errors: %d, network errors: %d)\n", fail+errCount, fail, errCount)
	fmt.Printf("  Total tokens: %d\n", totalTok)

	if len(latencies) > 0 {
		fmt.Printf("\n  Latency Distribution:\n")
		fmt.Printf("    Min:    %s\n", latencies[0].Round(time.Millisecond))
		fmt.Printf("    P50:    %s\n", percentile(latencies, 0.50).Round(time.Millisecond))
		fmt.Printf("    P90:    %s\n", percentile(latencies, 0.90).Round(time.Millisecond))
		fmt.Printf("    P95:    %s\n", percentile(latencies, 0.95).Round(time.Millisecond))
		fmt.Printf("    P99:    %s\n", percentile(latencies, 0.99).Round(time.Millisecond))
		fmt.Printf("    Max:    %s\n", latencies[len(latencies)-1].Round(time.Millisecond))
	}

	if len(statusDist) > 0 {
		fmt.Printf("\n  Status Codes:\n")
		for code, count := range statusDist {
			fmt.Printf("    %d: %d\n", code, count)
		}
	}

	// Print first error if any
	if fail > 0 || errCount > 0 {
		fmt.Printf("\n  First error sample:\n")
		for _, r := range results {
			if r.err != nil {
				fmt.Printf("    Network: %v\n", r.err)
				break
			}
			if r.status != 200 {
				fmt.Printf("    HTTP %d: %.200s\n", r.status, r.response)
				break
			}
		}
	}

	fmt.Printf("═══════════════════════════════════════════════════════════\n")
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}
