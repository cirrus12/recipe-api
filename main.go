package main

import (
    "encoding/json"
    "log"
    "net/http"
    "os"
    "fmt"
)

type Response struct {
    Message string `json:"message"`
}

func hello(w http.ResponseWriter, r *http.Request) {
    response := Response{Message: "Hello World"}
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func main() {
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080" // default port if not specified
    }

    http.HandleFunc("/hello", hello)
    log.Printf("Server starting on port %s...", port)
    log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
} 