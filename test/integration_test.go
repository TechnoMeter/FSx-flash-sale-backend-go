package test

import (
    "bytes"
    "encoding/json"
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
    // Assumes local server is running on :8080
    // We'll send 15 concurrent requests with initial stock=10
    // Expect exactly 10 success and 5 sold-out responses.
    var wg sync.WaitGroup
    successes := 0
    tooMany := 0
    mu := sync.Mutex{}

    // Reserve endpoint
    url := "http://localhost:8080/reserve"
    payload := map[string]interface{}{
        "product_id": 1,
        "user_id":    "test-user",
    }
    data, _ := json.Marshal(payload)

    for i := 0; i < 15; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
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
            }
            mu.Unlock()
        }()
        time.Sleep(10 * time.Millisecond) // small stagger to simulate real concurrency
    }

    wg.Wait()

    assert.Equal(t, 10, successes, "should have 10 successful reservations")
    assert.Equal(t, 5, tooMany, "should have 5 sold-out responses")

    // Verify Redis stock is 0
    // (you'd need to query Redis directly)
}