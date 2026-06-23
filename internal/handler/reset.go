package handler

import (
    "fmt"
    "net/http"
    "https://github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/db"
)

func ResetStock(rdb *db.RedisDB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Secret key to prevent abuse (optional but good)
        key := r.URL.Query().Get("key")
        if key != "reset2026" {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }

        err := rdb.Client.Set(r.Context(), "inventory:product:1", 100, 0).Err()
        if err != nil {
            http.Error(w, "reset failed", http.StatusInternalServerError)
            return
        }

        w.WriteHeader(http.StatusOK)
        fmt.Fprintln(w, "Stock reset to 100")
    }
}