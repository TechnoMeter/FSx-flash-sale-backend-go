package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRaceCondition(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in CI")
	}

	// Read reset key from environment
	resetKey := os.Getenv("RESET_KEY")
	if resetKey == "" {
		resetKey = "reset2026"
	}
	resetURL := fmt.Sprintf("http://localhost:8080/reset?key=%s", resetKey)
	resp, err := http.Get(resetURL)
	if err != nil {
		t.Fatalf("failed to reset stock: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reset failed with status %d", resp.StatusCode)
	}
	time.Sleep(100 * time.Millisecond)

	// Verify stock is 100
	stockResp, err := http.Get("http://localhost:8080/stock")
	if err != nil {
		t.Fatalf("failed to get stock: %v", err)
	}
	defer stockResp.Body.Close()
	var stockData map[string]int64
	if err := json.NewDecoder(stockResp.Body).Decode(&stockData); err != nil {
		t.Fatalf("failed to decode stock: %v", err)
	}
	assert.Equal(t, int64(100), stockData["stock"], "stock should be 100 after reset")

	var wg sync.WaitGroup
	successes := 0
	tooMany := 0
	mu := sync.Mutex{}

	url := "http://localhost:8080/reserve"
	totalRequests := 105
	delay := 150 * time.Millisecond

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
		time.Sleep(delay)
	}

	wg.Wait()

	assert.Equal(t, 100, successes, "should have 100 successful reservations")
	assert.Equal(t, 5, tooMany, "should have 5 sold-out responses")

	// Final check: Redis stock should be 0
	stockResp2, err := http.Get("http://localhost:8080/stock")
	if err != nil {
		t.Fatalf("failed to get final stock: %v", err)
	}
	defer stockResp2.Body.Close()
	var stockData2 map[string]int64
	if err := json.NewDecoder(stockResp2.Body).Decode(&stockData2); err != nil {
		t.Fatalf("failed to decode final stock: %v", err)
	}
	assert.Equal(t, int64(0), stockData2["stock"], "final stock should be 0")
}