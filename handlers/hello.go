package handlers

import (
    "bytes"
    "encoding/json"
    "io"
    "log"
    "net/http"
    "strings"
    "github.com/PuerkitoBio/goquery"
)

type RecipeResponse struct {
    Ingredients []string `json:"ingredients"`
    Error      string   `json:"error,omitempty"`
}

type HealthCheckResponse struct {
    Message string `json:"message"`
}

func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
    response := HealthCheckResponse{Message: "Api is running"}
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func GetRecipeIngredientsHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    url := r.URL.Query().Get("url")
    if url == "" {
        json.NewEncoder(w).Encode(RecipeResponse{
            Error: "URL parameter is required",
        })
        return
    }

    // Create a new request with headers
    client := &http.Client{}
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        log.Printf("Error creating request: %v", err)
        return
    }

    // Add browser-like headers
    req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
    req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
    req.Header.Set("Accept-Language", "en-US,en;q=0.5")
    req.Header.Set("Connection", "keep-alive")
    req.Header.Set("Cache-Control", "no-cache")
    req.Header.Set("Pragma", "no-cache")
    req.Header.Set("Sec-Fetch-Dest", "document")
    req.Header.Set("Sec-Fetch-Mode", "navigate")
    req.Header.Set("Sec-Fetch-Site", "none")
    req.Header.Set("Sec-Fetch-User", "?1")
    req.Header.Set("Upgrade-Insecure-Requests", "1")

    // Make the request
    resp, err := client.Do(req)
    if err != nil {
        log.Printf("Error fetching URL: %v", err)
        json.NewEncoder(w).Encode(RecipeResponse{
            Error: "Failed to fetch webpage",
        })
        return
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        log.Printf("Error reading body: %v", err)
        return
    }

    doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
    if err != nil {
        log.Printf("Error parsing HTML: %v", err)
        json.NewEncoder(w).Encode(RecipeResponse{
            Error: "Failed to parse webpage",
        })
        return
    }

    var ingredients []string
    seenIngredients := make(map[string]bool)
    
    excludeTexts := []string{
        "Deselect All",
        "Select All",
        "Ingredients",
        "For the",
        "Special equipment",
    }

    selectors := []string{
        // Food Network specific selectors
        ".o-Ingredients__a-Ingredient",
        ".recipe-ingredients li",
        ".recipe-ingredients-item",
        ".ingredients li",
        ".ingredient-item",
        // Previous selectors
        ".wprm-recipe-ingredient",
        ".wprm-recipe-ingredients li",
        ".tasty-recipes-ingredients li",
        ".ingredients-list li",
        "[itemprop='recipeIngredient']",
        ".recipe-ingredients li",
        ".ingredient-list li",
        ".Recipe__ingredients li",
        ".Recipe__ingredientItems li",
        ".Ingredients__ingredient",
        // Generic selectors
        "[data-ingredient]",
        ".ingredient",
        // Additional generic selectors
        ".recipe-ingredient",
        ".ingredient-list > li",
        "[data-testid='ingredient-item']",
        ".ingredient-text",
    }

    for _, selector := range selectors {
        doc.Find(selector).Each(func(i int, s *goquery.Selection) {
            // Clean up the ingredient text
            ingredient := strings.TrimSpace(s.Text())
            ingredient = strings.TrimPrefix(ingredient, "▢")
            ingredient = strings.TrimSpace(ingredient)
            
            // Check if ingredient should be excluded
            shouldExclude := false
            for _, excludeText := range excludeTexts {
                if strings.EqualFold(ingredient, excludeText) {
                    shouldExclude = true
                    break
                }
            }
            
            // Only add if it's not empty, not excluded, and not seen before
            if ingredient != "" && !shouldExclude && !seenIngredients[ingredient] {
                ingredients = append(ingredients, ingredient)
                seenIngredients[ingredient] = true
            }
        })
    }

    if len(ingredients) == 0 {
        log.Printf("No ingredients found for URL: %s", url)
        json.NewEncoder(w).Encode(RecipeResponse{
            Error: "No ingredients found on the webpage",
        })
        return
    }

    // Pretty print the JSON response
    w.Header().Set("Content-Type", "application/json")
    encoder := json.NewEncoder(w)
    encoder.SetIndent("", "    ")
    encoder.Encode(RecipeResponse{
        Ingredients: ingredients,
    })
}
