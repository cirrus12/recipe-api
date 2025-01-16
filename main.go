package main

import (
    "log"
    "net/http"
    "os"
    "fmt"
    "api/handlers"
)

func main() {
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    // Routes
    http.HandleFunc("/hello", handlers.HelloHandler)

    log.Printf("Server starting on port %s...", port)
    log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
} 