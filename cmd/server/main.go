package main

import (
	"fmt"
	"inventory/internals/app"
	"inventory/internals/config"
	"inventory/internals/database"
	"inventory/internals/routes"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// ipLimiter holds a rate limiter and the last time it was seen.
type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiterStore manages per-IP limiters with automatic cleanup.
type RateLimiterStore struct {
	mu       sync.Mutex
	limiters map[string]*ipLimiter
	r        rate.Limit // requests per second
	b        int        // burst size
}

func NewRateLimiterStore(r rate.Limit, b int) *RateLimiterStore {
	store := &RateLimiterStore{
		limiters: make(map[string]*ipLimiter),
		r:        r,
		b:        b,
	}
	// Background goroutine: clean up stale IPs every 5 minutes
	go store.cleanupLoop()
	return store
}

func (s *RateLimiterStore) getLimiter(ip string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.limiters[ip]
	if !exists {
		limiter := rate.NewLimiter(s.r, s.b)
		s.limiters[ip] = &ipLimiter{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}
	entry.lastSeen = time.Now()
	return entry.limiter
}

func (s *RateLimiterStore) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		for ip, entry := range s.limiters {
			if time.Since(entry.lastSeen) > 10*time.Minute {
				delete(s.limiters, ip)
			}
		}
		s.mu.Unlock()
	}
}

// RateLimitMiddleware returns a Gin middleware using the store.
func RateLimitMiddleware(store *RateLimiterStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiter := store.getLimiter(ip)

		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "Too many requests",
				"retry_after": "Please wait before making another request.",
			})
			return
		}
		c.Next()
	}
}

func main() {
	start := time.Now()

	if err := config.InitConfig(); err != nil {
		log.Printf("%v", err)
	}

	if err := database.InitializeDatabase(); err != nil {
		log.Printf("db connection error : %v", err)
	}

	application, err := app.New()
	if err != nil {
		log.Fatalf("failed to initalize app : %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	limiterStore := NewRateLimiterStore(rate.Limit(5), 10)
	router.Use(RateLimitMiddleware(limiterStore))

	router.HEAD("/", func(c *gin.Context) {
		fmt.Println("calling head ip : ", c.ClientIP(), c.RemoteIP())
		c.Status(200)
	})

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "https://gharmai-shikshya-tms.vercel.app", "https://gharmai-shikshya-tms-git-main-abhishekh-nepali.vercel.app", "https://gharmai-shikshya-li0cr1qzs-abhishekh-nepali.vercel.app"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "Cookie"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	routes.SetUpRoutes(router, application)
	log.Println("🌐 Starting HTTP server on :8080...")
	log.Println("📡 Server endpoints are now available")
	log.Println("App ready in", time.Since(start).Seconds(), "seconds.")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("❌ Server failed to start: %v", err)
	}

}
