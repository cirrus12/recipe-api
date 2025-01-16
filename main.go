package main

import (
    "encoding/json"
    "log"
    "net/http"
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
    http.HandleFunc("/hello", hello)
    log.Printf("Server starting on port 8080...")
    log.Fatal(http.ListenAndServe(":8080", nil))
} 