package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/db"
)

func ResetStock(rdb *db.RedisDB, pg *db.PostgresDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		expected := os.Getenv("RESET_KEY")
		if expected == "" {
			http.Error(w, `{"error":"reset key not configured"}`, http.StatusInternalServerError)
			return
		}
		if key != expected {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		// 1. Delete all orders for product 1 (clear the slate)
		ctx := r.Context()
		_, err := pg.Pool.Exec(ctx, "DELETE FROM orders WHERE product_id = 1")
		if err != nil {
			http.Error(w, `{"error":"failed to clear orders"}`, http.StatusInternalServerError)
			return
		}

		// 2. Reset Redis stock to 100
		err = rdb.Client.Set(ctx, "inventory:product:1", 100, 0).Err()
		if err != nil {
			http.Error(w, `{"error":"reset failed"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "Full reset complete (orders cleared, stock=100)"})
	}
}