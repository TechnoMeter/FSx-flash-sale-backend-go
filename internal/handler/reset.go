package handler

import (
    "encoding/json"
    "net/http"

    "github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/db"
)

func ResetStock(rdb *db.RedisDB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        key := r.URL.Query().Get("key")
        if key != "reset2026" {
            http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
            return
        }

        err := rdb.Client.Set(r.Context(), "inventory:product:1", 100, 0).Err()
        if err != nil {
            http.Error(w, `{"error":"reset failed"}`, http.StatusInternalServerError)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "Stock reset to 100"})
    }
}