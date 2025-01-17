package handlers

import (
    "bytes"
    "encoding/json"
    "io"
    "log"
    "net/http"
    "strings"
    "github.com/PuerkitoBio/goquery"
    "unicode"
)

type HealthCheckResponse struct {
    Message string `json:"message"`
}

func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
    response := HealthCheckResponse{Message: "Api is running"}
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

type RecipeStep struct {
    Number      int    `json:"step_number"`
    Instruction string `json:"instruction"`
}

type RecipeResponse struct {
    Title       string       `json:"title"`
    Ingredients []string     `json:"ingredients"`
    Steps       []RecipeStep `json:"steps"`
    Error       string       `json:"error,omitempty"`
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

    // Add NYT Cooking specific headers
    req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
    req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
    req.Header.Set("Accept-Language", "en-US,en;q=0.5")
    req.Header.Set("Referer", "https://cooking.nytimes.com/")
    req.Header.Set("Origin", "https://cooking.nytimes.com")

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
	var title string
    seenIngredients := make(map[string]bool)
    
    excludeTexts := []string{
        "Deselect All",
        "Select All",
        "Ingredients",
        "For the",
        "Special equipment",
        "Yield",
        "Nutritional Information",
        "Preparation",
    }

    selectors := []string{
        // NYT Cooking specific selectors
        ".recipe-ingredients li",
        ".ingredients li",
        ".recipe__ingredient",
        ".ingredient-name",
        ".ingredient",
        // Previous selectors
        ".o-Ingredients__a-Ingredient",
        ".recipe-ingredients-item",
        ".ingredient-item",
        ".wprm-recipe-ingredient",
        ".wprm-recipe-ingredients li",
        ".tasty-recipes-ingredients li",
        ".ingredients-list li",
        "[itemprop='recipeIngredient']",
        ".Recipe__ingredients li",
        ".Recipe__ingredientItems li",
        ".Ingredients__ingredient",
        "[data-ingredient]",
        ".recipe-ingredient",
        ".ingredient-list > li",
        "[data-testid='ingredient-item']",
        ".ingredient-text",
    }

    // Try to get the title from different selectors in order of preference
    title = strings.TrimSpace(doc.Find("h1").First().Text())
    
    // If h1 is empty, try article title
    if title == "" {
        title = strings.TrimSpace(doc.Find("article.recipe h1").First().Text())
    }
    
    // If still empty, try recipe schema
    if title == "" {
        title = strings.TrimSpace(doc.Find("[itemtype='http://schema.org/Recipe'] h1").First().Text())
    }
    
    // Fallback to title tag if nothing else worked
    if title == "" {
        title = strings.TrimSpace(doc.Find("title").Text())
        // Apply the suffix removal logic only if we had to use the title tag
        suffixes := []string{
            " Recipe |",
            " | Food Network Kitchen",
            // ... rest of your existing suffixes ...
        }
        
        for _, suffix := range suffixes {
            if idx := strings.Index(title, suffix); idx != -1 {
                title = strings.TrimSpace(title[:idx])
            }
        }
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

    // Add these selectors after your ingredients selectors
    stepSelectors := []string{
        // Common recipe step selectors
        ".recipe-instructions li",
        ".recipe-directions__list li",
        ".recipe__instructions li",
        ".wprm-recipe-instruction",
        ".tasty-recipes-instructions li",
        ".recipe-method li",
        ".instructions li",
        "[itemprop='recipeInstructions'] li",
        ".Recipe__instructions li",
        ".recipe-steps li",
        ".preparation-steps li",
        "[data-testid='instruction-step']",
        ".instruction-step",
    }

    var steps []RecipeStep
    seenSteps := make(map[string]bool)

    // Extract recipe steps
    for _, selector := range stepSelectors {
        doc.Find(selector).Each(func(i int, s *goquery.Selection) {
            step := strings.TrimSpace(s.Text())
            
            // Clean up the step text
            step = strings.TrimPrefix(step, "Step")
            step = strings.TrimPrefix(step, ".")
            step = strings.TrimSpace(step)
            
            // Remove any numbering at the start (like "1.", "2.", etc.)
            step = strings.TrimLeftFunc(step, func(r rune) bool {
                return r == '.' || unicode.IsNumber(r) || unicode.IsSpace(r)
            })
            
            // Only add if it's not empty and not seen before
            if step != "" && !seenSteps[step] {
                steps = append(steps, RecipeStep{
                    Number:      len(steps) + 1,
                    Instruction: step,
                })
                seenSteps[step] = true
            }
        })

        // If we found steps with this selector, stop looking
        if len(steps) > 0 {
            break
        }
    }

    if len(ingredients) == 0 {
        log.Printf("No ingredients found for URL: %s", url)
        json.NewEncoder(w).Encode(RecipeResponse{
            Error: "No ingredients found on the webpage",
        })
        return
    }

    // Check response status code
    if resp.StatusCode == 403 {
        log.Printf("Access forbidden for URL: %s", url)
        json.NewEncoder(w).Encode(RecipeResponse{
            Error: "This website requires a subscription or login to access recipes",
        })
        return
    }

    if resp.StatusCode != 200 {
        log.Printf("Unexpected status code %d for URL: %s", resp.StatusCode, url)
        json.NewEncoder(w).Encode(RecipeResponse{
            Error: "Unable to access the recipe webpage",
        })
        return
    }

    // Pretty print the JSON response
    w.Header().Set("Content-Type", "application/json")
    encoder := json.NewEncoder(w)
    encoder.SetIndent("", "    ")
    encoder.Encode(RecipeResponse{
		Title:       title,
        Ingredients: ingredients,
        Steps:       steps,
    })
}
