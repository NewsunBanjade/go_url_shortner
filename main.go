package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
	"github.com/teris-io/shortid"
)

var (
	redisClient *redis.Client
	ctx         = context.Background()
)

func init() {
	redisAddr := "localhost:6379" // ENV TO USE LATER
	redisClient = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "admin@123", //Not Used ENV Later to Put ENV
		DB:       0,
	})
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	log.Printf("Successfully connected to Redis at %s", redisAddr)
}

func generateShortURL() (string, error) {
	id, err := shortid.Generate()
	if err != nil {
		return "", fmt.Errorf("failed to generated short id %w", err)
	}
	return id, nil
}

func storeURL(shortURL, longURL string) error {
	exp := 24 * time.Hour
	err := redisClient.Set(ctx, shortURL, longURL, exp).Err()

	if err != nil {
		return fmt.Errorf("failed to store in redis,%v", err)
	}
	log.Printf("Stored short URL '%s' for long URL '%s' with expiration %s", shortURL, longURL, exp)

	return nil
}

func getLongURL(shortURL string) (string, error) {
	longUrl, err := redisClient.Get(ctx, shortURL).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("short url not found %w", err)
	}
	if err != nil {
		return "", fmt.Errorf("error while getting url from redis, %w", err)

	}
	return longUrl, nil

}

type UrlReq struct {
	URL string `json:"url"`
}

func shortnerHandler(w http.ResponseWriter, r *http.Request) {

	var urlReq UrlReq

	err := json.NewDecoder(r.Body).Decode(&urlReq)
	if err != nil {
		fmt.Fprintf(w, "error while gettin %v", err)
		return
	}
	if urlReq.URL == "" {
		fmt.Fprintf(w, "url is missing ")
		return
	}

	//ShortURL generation
	shortURL, err := generateShortURL()
	if err != nil {
		fmt.Fprintf(w, "Sorry, unable to generate short url ")
		return
	}

	fullShortURL := fmt.Sprintf("http://localhost:3000/%s", shortURL)
	err = storeURL(shortURL, urlReq.URL)
	if err != nil {
		fmt.Fprintf(w, "Sorry, unable to generate short url %+v ", err)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "Shortened URL: %s\n", fullShortURL)

}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	shortURL := chi.URLParam(r, "url")
	if shortURL == "" {
		http.Error(w, "no shorturl in url", http.StatusBadRequest)
		return
	}
	longURL, err := getLongURL(shortURL)
	if err != nil {
		fmt.Fprintf(w, "error %+v ", err)
		return
	}
	http.Redirect(w, r, ensureHTTPSScheme(longURL), http.StatusMovedPermanently)

}
func ensureHTTPSScheme(url string) string {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "http://" + url
	}
	return url
}

func main() {
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Get("/{url}", redirectHandler)
	router.Post("/short", shortnerHandler)
	http.ListenAndServe(":3000", router)
}
