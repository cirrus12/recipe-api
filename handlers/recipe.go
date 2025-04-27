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
    "compress/gzip"
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

    // Create a new request with modern browser headers
    client := &http.Client{}
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        log.Printf("Error creating request: %v", err)
        return
    }

    // Set modern browser headers for all requests
    req.Header = http.Header{
        "User-Agent":                {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36"},
        "Accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"},
        "Accept-Language":           {"en-US,en;q=0.9"},
        "Accept-Encoding":           {"gzip, deflate"},
        "Connection":                {"keep-alive"},
        "Upgrade-Insecure-Requests": {"1"},
        "Sec-Fetch-Dest":           {"document"},
        "Sec-Fetch-Mode":           {"navigate"},
        "Sec-Fetch-Site":           {"none"},
        "Sec-Fetch-User":           {"?1"},
        "Cache-Control":             {"max-age=0"},
        "sec-ch-ua":                {`"Not A(Brand";v="99", "Google Chrome";v="121", "Chromium";v="121"`},
        "sec-ch-ua-mobile":         {"?0"},
        "sec-ch-ua-platform":       {`"Windows"`},
    }

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

    // Handle compressed responses
    var reader io.Reader = resp.Body
    if resp.Header.Get("Content-Encoding") == "gzip" {
        reader, err = gzip.NewReader(resp.Body)
        if err != nil {
            log.Printf("Error creating gzip reader: %v", err)
            json.NewEncoder(w).Encode(RecipeResponse{
                Error: "Failed to read compressed webpage",
            })
            return
        }
        defer reader.(*gzip.Reader).Close()
    }

    body, err := io.ReadAll(reader)
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

    // Debug logging for full HTML
    fullHtml, _ := doc.Html()
    log.Printf("Full HTML length: %d", len(fullHtml))
    log.Printf("First 500 chars of HTML: %s", fullHtml[:min(len(fullHtml), 500)])

    // Check if we got a valid HTML response
    if len(fullHtml) < 100 {
        log.Printf("Received suspiciously short HTML response. Length: %d", len(fullHtml))
        json.NewEncoder(w).Encode(RecipeResponse{
            Error: "Unable to properly load the recipe page. The website might be blocking automated access.",
        })
        return
    }

    // Try different recipe container selectors
    recipeContainers := []string{
        ".wprm-recipe-container",
        ".recipe-container",
        ".recipe-content",
        "#recipe",
        "[itemtype='http://schema.org/Recipe']",
    }

    for _, container := range recipeContainers {
        containerHtml, _ := doc.Find(container).Html()
        log.Printf("Container '%s' HTML length: %d", container, len(containerHtml))
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

    // Add more specific selectors
    selectors := []string{
        // Love & Lemons specific selectors
        ".wprm-recipe-ingredient",  // Simplified selector
        ".wprm-recipe-ingredients .wprm-recipe-ingredient-group", // Parent container
        ".wprm-recipe-ingredients-container .wprm-recipe-ingredient",
        ".wprm-recipe-ingredient-group .wprm-recipe-ingredient",
        ".wprm-recipe-ingredients-container li",
        ".wprm-recipe-ingredients li",
        ".tasty-recipes-ingredients li",
        // Previous selectors
        ".mntl-structured-ingredients__list-item",
        ".ingredients-item-name",
        ".recipe-ingredients__item-name",
        ".ingredients-item",
        "[data-ingredient-name]",
        ".recipe-ingredients li",
        ".ingredients li",
        ".recipe__ingredient",
        ".ingredient-name",
        ".ingredient",
        ".o-Ingredients__a-Ingredient",
        ".recipe-ingredients-item",
        ".ingredient-item",
        ".wprm-recipe-ingredient",
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

    // Debug logging for HTML structure
    htmlContent, _ := doc.Find(".wprm-recipe-ingredients").Html()
    log.Printf("HTML Content: %s", htmlContent)

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

    // Try to get ingredients with debug logging
    for _, selector := range selectors {
        matches := doc.Find(selector)
        log.Printf("Trying selector '%s': found %d matches", selector, matches.Length())
        matches.Each(func(i int, s *goquery.Selection) {
            ingredient := strings.TrimSpace(s.Text())
            ingredient = strings.TrimPrefix(ingredient, "▢")
            ingredient = strings.TrimSpace(ingredient)
            
            if ingredient != "" {
                log.Printf("Found ingredient: %s", ingredient)
            }
            
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

    // Add more specific step selectors
    stepSelectors := []string{
        // Love & Lemons specific step selectors
        ".wprm-recipe-instructions-container .wprm-recipe-instruction-text",
        ".wprm-recipe-instruction-group .wprm-recipe-instruction-text",
        ".wprm-recipe-instructions-container li",
        ".wprm-recipe-instructions li",
        ".tasty-recipes-instructions li",
        // Previous step selectors
        ".recipe__steps-content p",
        ".mntl-sc-block-html",
        ".recipe-instructions__step",
        ".instructions-section p",
        ".recipe-instructions li",
        ".recipe-directions__list li",
        ".recipe__instructions li",
        ".wprm-recipe-instruction",
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

    // Modify the Allrecipes specific handling
    if strings.Contains(url, "allrecipes.com") {
        // Try multiple ingredient selectors
        ingredientSelectors := []string{
            ".mntl-structured-ingredients__list-item",
            ".ingredients-item-name",
            ".ingredient-list li",
            "[data-ingredient-name]",
            ".recipe-ingredients__list-item",
        }

        for _, selector := range ingredientSelectors {
            doc.Find(selector).Each(func(i int, s *goquery.Selection) {
                ingredient := strings.TrimSpace(s.Text())
                if ingredient != "" && !seenIngredients[ingredient] {
                    ingredients = append(ingredients, ingredient)
                    seenIngredients[ingredient] = true
                }
            })
        }

        // Try multiple step selectors
        stepSelectors := []string{
            ".recipe__steps-content .mntl-sc-block-group--LI",
            ".recipe-instructions__list-item",
            ".instructions-section p",
            ".recipe-directions__item",
            ".recipe__instructions-step",
        }

        for _, selector := range stepSelectors {
            doc.Find(selector).Each(func(i int, s *goquery.Selection) {
                step := strings.TrimSpace(s.Text())
                if step != "" && !seenSteps[step] {
                    steps = append(steps, RecipeStep{
                        Number:      len(steps) + 1,
                        Instruction: step,
                    })
                    seenSteps[step] = true
                }
            })
        }
    }

    // Add debug logging before checking ingredients length
    log.Printf("Found %d ingredients", len(ingredients))
    log.Printf("Found %d steps", len(steps))

    if len(ingredients) == 0 {
        log.Printf("No ingredients found for URL: %s", url)
        json.NewEncoder(w).Encode(RecipeResponse{
            Error: "No ingredients were found on this webpage. Please ensure you're using a URL that leads directly to a recipe page with a list of ingredients and cooking instructions.",
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
