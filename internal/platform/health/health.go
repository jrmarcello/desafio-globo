package health

import (
	context "context"
	"database/sql"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

type Checker struct {
	db    *sql.DB
	redis *redis.Client
}

func NewChecker(db *sql.DB, redis *redis.Client) *Checker {
	return &Checker{db: db, redis: redis}
}

func (c *Checker) ReadyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if c.db != nil {
			if err := c.db.PingContext(ctx); err != nil {
				http.Error(w, "database unavailable", http.StatusServiceUnavailable)
				return
			}
		}

		if c.redis != nil {
			if err := c.redis.Ping(ctx).Err(); err != nil {
				http.Error(w, "redis unavailable", http.StatusServiceUnavailable)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}
