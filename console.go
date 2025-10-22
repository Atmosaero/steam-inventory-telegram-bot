package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

type SteamInventoryResponse struct {
	Assets       []Asset       `json:"assets"`
	Descriptions []Description `json:"descriptions"`
	TotalCount   int           `json:"total_inventory_count"`
	Success      int           `json:"success"`
	Error        string        `json:"error"`
	MoreItems    int           `json:"more_items"`
	LastAssetID  string        `json:"last_assetid"`
}

type Asset struct {
	AppID      int    `json:"appid"`
	ContextID  string `json:"contextid"`
	AssetID    string `json:"assetid"`
	ClassID    string `json:"classid"`
	InstanceID string `json:"instanceid"`
	Amount     string `json:"amount"`
}

type Description struct {
	AppID                     int    `json:"appid"`
	ClassID                   string `json:"classid"`
	InstanceID                string `json:"instanceid"`
	IconURL                   string `json:"icon_url"`
	IconURLLarge              string `json:"icon_url_large"`
	Name                      string `json:"name"`
	MarketName                string `json:"market_name"`
	MarketHashName            string `json:"market_hash_name"`
	Type                      string `json:"type"`
	Tradable                  int    `json:"tradable"`
	Marketable                int    `json:"marketable"`
	Commodity                 int    `json:"commodity"`
	MarketTradableRestriction int    `json:"market_tradable_restriction"`
}

type MarketPriceResponse struct {
	Success     bool   `json:"success"`
	LowestPrice string `json:"lowest_price"`
	MedianPrice string `json:"median_price"`
	Volume      string `json:"volume"`
}

func fetchAllInventory(steamID, appID, contextID string, debug bool) ([]Asset, []Description, int) {
	allAssets := []Asset{}
	allDescriptions := []Description{}
	descMap := make(map[string]Description)
	totalCount := 0
	startAssetID := ""
	page := 0

	for {
		page++
		apiURL := fmt.Sprintf("https://steamcommunity.com/inventory/%s/%s/%s", steamID, appID, contextID)
		if startAssetID != "" {
			apiURL += "?start_assetid=" + startAssetID
		}

		if debug {
			fmt.Printf("[DEBUG] Fetching page %d (start_assetid=%s)\n", page, startAssetID)
		}

		client := &http.Client{
			Timeout: 30 * time.Second,
		}
		resp, err := client.Get(apiURL)
		if err != nil {
			if debug {
				fmt.Printf("[DEBUG] HTTP error: %v\n", err)
			}
			break
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			if debug {
				fmt.Printf("[DEBUG] Status code error: %d %s\n", resp.StatusCode, resp.Status)
			}
			break
		}

		var inventory SteamInventoryResponse
		if err := json.NewDecoder(resp.Body).Decode(&inventory); err != nil {
			resp.Body.Close()
			if debug {
				fmt.Printf("[DEBUG] JSON decode error: %v\n", err)
			}
			break
		}
		resp.Body.Close()

		if inventory.Success != 1 {
			if debug {
				fmt.Printf("[DEBUG] API error: %s\n", inventory.Error)
			}
			break
		}

		allAssets = append(allAssets, inventory.Assets...)

		for _, desc := range inventory.Descriptions {
			key := desc.ClassID + "_" + desc.InstanceID
			if _, exists := descMap[key]; !exists {
				descMap[key] = desc
			}
		}

		totalCount = inventory.TotalCount

		if debug {
			fmt.Printf("[DEBUG] Page %d: Assets=%d, Descriptions=%d, MoreItems=%d\n",
				page, len(inventory.Assets), len(inventory.Descriptions), inventory.MoreItems)
		}

		if inventory.MoreItems != 1 || inventory.LastAssetID == "" {
			break
		}

		startAssetID = inventory.LastAssetID
		time.Sleep(1 * time.Second)
	}

	for _, desc := range descMap {
		allDescriptions = append(allDescriptions, desc)
	}

	if debug {
		fmt.Printf("[DEBUG] Total fetched: Assets=%d, Descriptions=%d\n",
			len(allAssets), len(allDescriptions))
	}

	return allAssets, allDescriptions, totalCount
}

func main() {
	startTime := time.Now()

	// Добавляем debug режим
	debug := false
	args := []string{}
	for _, arg := range os.Args {
		if arg == "debug" {
			debug = true
		} else {
			args = append(args, arg)
		}
	}

	var input, appID string

	if len(args) >= 2 {
		input = args[1]
		if len(args) >= 3 {
			appID = args[2]
		}
	} else {
		fmt.Println("=== Steam Inventory Viewer ===")
		fmt.Println()
		fmt.Print("Enter Steam ID or profile URL: ")
		fmt.Scanln(&input)

		fmt.Println()
		fmt.Println("App IDs: 730 (CS2), 570 (Dota 2), 753 (Steam items)")
		fmt.Print("Enter App ID (press Enter for all games): ")
		fmt.Scanln(&appID)
	}

	if input == "" {
		log.Fatal("Steam ID cannot be empty")
	}

	steamID := resolveSteamID(input)
	if steamID == "" {
		log.Fatalf("Failed to resolve Steam ID from: %s", input)
	}

	if debug {
		fmt.Printf("[DEBUG] Resolved Steam ID: %s\n", steamID)
	}

	if appID == "" {
		fmt.Println("\nScanning all games in inventory...")
		scanAllGames(steamID, debug)
		fmt.Println("\nPress Enter to exit...")
		fmt.Scanln()
		return
	}

	contextID := "2"
	if appID == "753" {
		contextID = "6"
	}

	fmt.Printf("\nFetching inventory for Steam ID: %s (App ID: %s, Context: %s)\n", steamID, appID, contextID)

	assets, descriptions, totalCount := fetchAllInventory(steamID, appID, contextID, debug)

	fmt.Printf("\n--- Inventory (Total: %d items) ---\n", totalCount)

	descMap := make(map[string]Description)
	for _, desc := range descriptions {
		key := desc.ClassID + "_" + desc.InstanceID
		descMap[key] = desc
	}

	if debug {
		fmt.Printf("[DEBUG] Final Assets=%d, Descriptions=%d\n", len(assets), len(descriptions))
	}

	priceCache := make(map[string]string)
	marketableItems := 0
	totalValue := 0.0
	minPrice := 0.0
	maxPrice := 0.0
	minPriceItem := ""
	maxPriceItem := ""
	skippedNoDesc := 0
	skippedNotMarketable := 0
	skippedNoHash := 0

	fmt.Println("\n--- Marketable Items Only ---")

	for _, asset := range assets {
		key := asset.ClassID + "_" + asset.InstanceID
		desc, found := descMap[key]

		if !found {
			skippedNoDesc++
			continue
		}

		if desc.Marketable != 1 {
			skippedNotMarketable++
			continue
		}

		if desc.MarketHashName == "" {
			skippedNoHash++
			continue
		}

		price, cached := priceCache[desc.MarketHashName]
		if !cached {
			price = getMarketPrice(appID, desc.MarketHashName, debug)
			priceCache[desc.MarketHashName] = price
			time.Sleep(5 * time.Second)
		}

		if price == "" {
			continue
		}

		priceValue := parsePrice(price)
		totalValue += priceValue

		if marketableItems == 0 {
			minPrice = priceValue
			maxPrice = priceValue
			minPriceItem = desc.Name
			maxPriceItem = desc.Name
		} else {
			if priceValue < minPrice {
				minPrice = priceValue
				minPriceItem = desc.Name
			}
			if priceValue > maxPrice {
				maxPrice = priceValue
				maxPriceItem = desc.Name
			}
		}

		marketableItems++
		fmt.Printf("\n[%d] %s\n", marketableItems, desc.Name)
		if desc.MarketName != "" {
			fmt.Printf("    Market Name: %s\n", desc.MarketName)
		}
		fmt.Printf("    Type: %s\n", desc.Type)
		fmt.Printf("    Market Price: %s\n", price)
		fmt.Printf("    Asset ID: %s\n", asset.AssetID)
	}

	if debug {
		fmt.Printf("[DEBUG] Skipped - No desc: %d, Not marketable: %d, No hash: %d\n",
			skippedNoDesc, skippedNotMarketable, skippedNoHash)
	}

	fmt.Printf("\n--- Inventory Statistics ---\n")
	fmt.Printf("Total marketable items: %d (from %d total)\n", marketableItems, totalCount)
	if marketableItems > 0 {
		fmt.Printf("Lowest price: %.2f руб. (%s)\n", minPrice, minPriceItem)
		fmt.Printf("Highest price: %.2f руб. (%s)\n", maxPrice, maxPriceItem)
		fmt.Printf("Total inventory value: %.2f руб.\n", totalValue)
	}

	duration := time.Since(startTime)
	fmt.Printf("Scan completed in: %v\n", duration.Round(time.Second))

	if len(args) < 2 {
		fmt.Println("\nPress Enter to exit...")
		fmt.Scanln()
	}
}

func scanAllGames(steamID string, debug bool) {
	startTime := time.Now()

	games := []struct {
		appID   string
		name    string
		context string
	}{
		{"730", "Counter-Strike 2", "2"},
		{"570", "Dota 2", "2"},
		{"753", "Steam Items", "6"},
		{"440", "Team Fortress 2", "2"},
		{"578080", "PUBG", "2"},
		{"252490", "Rust", "2"},
	}

	totalValue := 0.0
	totalItems := 0

	for _, game := range games {
		assets, descriptions, totalCount := fetchAllInventory(steamID, game.appID, game.context, debug)

		if totalCount == 0 {
			if debug {
				fmt.Printf("[DEBUG] %s: No items\n", game.name)
			}
			continue
		}

		fmt.Printf("\n--- %s (%d items) ---\n", game.name, totalCount)

		descMap := make(map[string]Description)
		for _, desc := range descriptions {
			key := desc.ClassID + "_" + desc.InstanceID
			descMap[key] = desc
		}

		if debug {
			fmt.Printf("[DEBUG] %s: Assets=%d, Descriptions=%d\n", game.name, len(assets), len(descriptions))
		}

		priceCache := make(map[string]string)
		marketableCount := 0
		gameValue := 0.0
		skippedNoDesc := 0
		skippedNotMarketable := 0
		skippedNoHash := 0

		for _, asset := range assets {
			key := asset.ClassID + "_" + asset.InstanceID
			desc, found := descMap[key]

			if !found {
				skippedNoDesc++
				continue
			}

			if desc.Marketable != 1 {
				skippedNotMarketable++
				continue
			}

			if desc.MarketHashName == "" {
				skippedNoHash++
				continue
			}

			price, cached := priceCache[desc.MarketHashName]
			if !cached {
				price = getMarketPrice(game.appID, desc.MarketHashName, debug)
				priceCache[desc.MarketHashName] = price
				time.Sleep(5 * time.Second)
			}

			if price != "" {
				priceValue := parsePrice(price)
				gameValue += priceValue
				marketableCount++
			}
		}

		if debug {
			fmt.Printf("[DEBUG] %s: Skipped - No desc: %d, Not marketable: %d, No hash: %d\n",
				game.name, skippedNoDesc, skippedNotMarketable, skippedNoHash)
		}

		if marketableCount > 0 {
			fmt.Printf("Marketable items: %d\n", marketableCount)
			fmt.Printf("Total value: %.2f руб.\n", gameValue)
			totalValue += gameValue
			totalItems += marketableCount
		}
	}

	fmt.Printf("\n=== TOTAL ACROSS ALL GAMES ===\n")
	fmt.Printf("Total marketable items: %d\n", totalItems)
	fmt.Printf("Total inventory value: %.2f руб.\n", totalValue)

	duration := time.Since(startTime)
	fmt.Printf("Scan completed in: %v\n", duration.Round(time.Second))
}

func getMarketPrice(appID string, marketHashName string, debug bool) string {
	encodedName := url.QueryEscape(marketHashName)
	marketURL := fmt.Sprintf("https://steamcommunity.com/market/priceoverview/?appid=%s&currency=5&market_hash_name=%s", appID, encodedName)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", marketURL, nil)
	if err != nil {
		return ""
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		if debug {
			fmt.Printf("[DEBUG] Market API error: %v\n", err)
		}
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		if debug {
			fmt.Printf("[DEBUG] Market API status: %d %s\n", resp.StatusCode, resp.Status)
		}
		return ""
	}

	var priceResp MarketPriceResponse
	if err := json.NewDecoder(resp.Body).Decode(&priceResp); err != nil {
		return ""
	}

	if !priceResp.Success {
		return ""
	}

	if priceResp.LowestPrice != "" {
		price := strings.TrimSpace(priceResp.LowestPrice)
		return fmt.Sprintf("%s (lowest)", price)
	}

	return ""
}

func parsePrice(priceStr string) float64 {
	priceStr = strings.TrimSpace(priceStr)
	priceStr = strings.Replace(priceStr, " (lowest)", "", 1)
	priceStr = strings.Replace(priceStr, "руб.", "", 1)
	priceStr = strings.Replace(priceStr, ",", ".", 1)
	priceStr = strings.TrimSpace(priceStr)

	var value float64
	fmt.Sscanf(priceStr, "%f", &value)
	return value
}

func resolveSteamID(input string) string {
	input = strings.TrimSpace(input)

	if regexp.MustCompile(`^\d+$`).MatchString(input) {
		return input
	}

	re := regexp.MustCompile(`profiles/(\d+)`)
	if matches := re.FindStringSubmatch(input); len(matches) >= 2 {
		return matches[1]
	}

	vanityRe := regexp.MustCompile(`id/([^/]+)`)
	if matches := vanityRe.FindStringSubmatch(input); len(matches) >= 2 {
		vanityName := matches[1]
		return getSteamIDFromVanity(vanityName)
	}

	return getSteamIDFromVanity(input)
}

func getSteamIDFromVanity(vanityName string) string {
	vanityName = strings.TrimSpace(vanityName)
	xmlURL := fmt.Sprintf("https://steamcommunity.com/id/%s/?xml=1", vanityName)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", xmlURL, nil)
	if err != nil {
		return ""
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ""
	}

	bodyText := readBody(resp)
	re := regexp.MustCompile(`<steamID64>(\d+)</steamID64>`)
	matches := re.FindStringSubmatch(bodyText)

	if len(matches) < 2 {
		return ""
	}

	return matches[1]
}

func readBody(resp *http.Response) string {
	body := make([]byte, 10000)
	n, _ := resp.Body.Read(body)
	return string(body[:n])
}
