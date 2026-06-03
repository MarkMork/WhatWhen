package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	port := getenv("PORT", "8080")
	dataFile := getenv("DATA_FILE", "/data/whatwhen.json")

	store, err := NewStore(dataFile)
	if err != nil {
		log.Fatalf("failed to open store at %s: %v", dataFile, err)
	}

	mux := http.NewServeMux()
	registerRoutes(mux, store)

	addr := ":" + port
	log.Printf("WhatWhen listening on %s (data file: %s)", addr, dataFile)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
