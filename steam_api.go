package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
	priceStr = strings.Replace(priceStr, " ", "", -1)
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
