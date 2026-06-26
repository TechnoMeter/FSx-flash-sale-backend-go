package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRaceCondition(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in CI")
	}

	// Reset stock to 100 (the default)
	resetURL := "http://localhost:8080/reset?key=reset2026"
	resp, err := http.Get(resetURL)
	if err != nil {
		t.Fatalf("failed to reset stock: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reset failed with status %d", resp.StatusCode)
	}

	time.Sleep(100 * time.Millisecond)

	var wg sync.WaitGroup
	successes := 0
	tooMany := 0
	mu := sync.Mutex{}

	url := "http://localhost:8080/reserve"
	totalRequests := 105 // 100 stock + 5 extra to force sold-out

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			payload := map[string]interface{}{
				"product_id": 1,
				"user_id":    fmt.Sprintf("test-user-%d", idx),
			}
			data, _ := json.Marshal(payload)

			req, _ := http.NewRequest("POST", url, bytes.NewBuffer(data))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Test-Mode", "true") // bypass rate limiter

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Error(err)
				return
			}
			defer resp.Body.Close()

			mu.Lock()
			if resp.StatusCode == http.StatusOK {
				successes++
			} else if resp.StatusCode == http.StatusTooManyRequests {
				tooMany++
			} else {
				t.Logf("unexpected status: %d", resp.StatusCode)
			}
			mu.Unlock()
		}(i)
		time.Sleep(5 * time.Millisecond) // stagger
	}

	wg.Wait()

	assert.Equal(t, 100, successes, "should have 100 successful reservations")
	assert.Equal(t, 5, tooMany, "should have 5 sold-out responses")
}