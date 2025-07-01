package middlewares

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	limiters  map[string]*rate.Limiter
	blocked   map[string]time.Time
	mu        sync.Mutex
	requests  int
	per       time.Duration
	blockTime time.Duration
}

func NewRateLimiter(rps int, per, blockTime time.Duration) *RateLimiter {
	return &RateLimiter{
		limiters:  make(map[string]*rate.Limiter),
		blocked:   make(map[string]time.Time),
		requests:  rps,
		per:       per,
		blockTime: blockTime,
	}
}

func (r *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ip, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		r.mu.Lock()

		if blockedUntil, found := r.blocked[ip]; found {
			if time.Now().Before(blockedUntil) {
				r.mu.Unlock()

				http.Error(w, "Too many requests, you are temporarily blocked.", http.StatusTooManyRequests)
				return
			}

			delete(r.blocked, ip)
		}

		limiter, exists := r.limiters[ip]
		if !exists {
			limiter = rate.NewLimiter(rate.Every(r.per), r.requests)
			r.limiters[ip] = limiter
		}

		r.mu.Unlock()

		if !limiter.Allow() {

			r.mu.Lock()
			defer r.mu.Unlock()

			r.blocked[ip] = time.Now().Add(r.blockTime)
			http.Error(w, "Too many requests, you are blocked temporarily.", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, req)
	})
}
