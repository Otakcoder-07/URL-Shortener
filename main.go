package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

type URLShortener struct {
	urls map[string]string
}

type RateLimiter struct {
	mu            sync.Mutex
	requests      int
	windowSize    time.Duration
	maxRequests   int
	lastTimestamp time.Time
}

func NewRateLimiter(windowSize time.Duration, maxRequests int) *RateLimiter {
	return &RateLimiter{
		windowSize:  windowSize,
		maxRequests: maxRequests,
	}
}

func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Reset the request count if the window has elapsed
	if now.Sub(rl.lastTimestamp) >= rl.windowSize {
		rl.requests = 0
		rl.lastTimestamp = now
	}

	// Check if the number of requests exceeds the limit
	if rl.requests >= rl.maxRequests {
		return false
	}

	// Increment the request count and allow the request
	rl.requests++
	return true
}

func (us *URLShortener) HandleShorten(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	originalURL := strings.TrimSpace(r.FormValue("url"))
	if originalURL == "" {
		http.Error(w, "URL parameter is missing", http.StatusBadRequest)
		return
	}

	shortKey := generateShortKey()
	us.urls[shortKey] = originalURL

	shortenedURL := fmt.Sprintf("http://localhost:8080/short/%s", shortKey)

	w.Header().Set("Content-Type", "text/html")
	responseHTML := fmt.Sprintf(`
        <html>
        <head>
            <title>URL Shortener</title>
        </head>
        <body>
            <h2>Shortened URL</h2>
            <input type="text" value="%s" readonly>
        </body>
        </html>
    `, shortenedURL)
	fmt.Fprintf(w, responseHTML)
}

func (us *URLShortener) HandleRedirect(w http.ResponseWriter, r *http.Request, limiter *RateLimiter) {
	// Apply rate limiting
	if !limiter.Allow() {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	shortKey := r.URL.Path[len("/short/"):]
	if shortKey == "" {
		http.Error(w, "Shortened key is missing", http.StatusBadRequest)
		return
	}

	originalURL, found := us.urls[shortKey]
	if !found {
		http.Error(w, "Shortened key not found", http.StatusNotFound)
		return
	}

	http.Redirect(w, r, originalURL, http.StatusMovedPermanently)
}

func generateShortKey() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const keyLength = 6

	key := make([]byte, keyLength)
	for i := range key {
		key[i] = charset[rand.Intn(len(charset))]
	}
	return string(key)
}

func main() {
	shortener := &URLShortener{
		urls: make(map[string]string),
	}

	// Initialize rate limiter with desired window size and maximum requests
	limiter := NewRateLimiter(time.Second, 10) // Allow 10 redirects per second

	// Handle requests
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := `
            <html>
            <head>
                <title>URL Shortener</title>
            </head>
            <body>
                <h2>Enter URL to Shorten</h2>
                <form method="post" action="/shorten">
                    <input type="text" name="url" placeholder="Enter a URL">
                    <input type="submit" value="Shorten">
                </form>
            </body>
            </html>
        `
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, html)
	})

	http.HandleFunc("/shorten", shortener.HandleShorten)
	// Pass rate limiter to HandleRedirect function
	http.HandleFunc("/short/", func(w http.ResponseWriter, r *http.Request) {
		shortener.HandleRedirect(w, r, limiter)
	})

	fmt.Println("URL Shortener is running on :8080")
	http.ListenAndServe(":8080", nil)
}
