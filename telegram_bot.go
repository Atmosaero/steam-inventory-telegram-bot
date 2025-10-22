package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// InventoryItem представляет предмет в инвентаре для GUI
type InventoryItem struct {
	Name       string  `json:"name"`
	MarketName string  `json:"market_name"`
	Type       string  `json:"type"`
	Price      string  `json:"price"`
	PriceValue float64 `json:"price_value"`
	AssetID    string  `json:"asset_id"`
}

type TelegramBot struct {
	bot         *tgbotapi.BotAPI
	cache       *Cache
	rateLimiter *RateLimiter
}

func NewTelegramBot(token string) (*TelegramBot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	bot.Debug = false
	log.Printf("Авторизован как %s", bot.Self.UserName)

	// Создаем кэш на 30 минут и rate limiter на 3 секунды
	cache := NewCache(30 * time.Minute)
	rateLimiter := NewRateLimiter(3 * time.Second)

	return &TelegramBot{
		bot:         bot,
		cache:       cache,
		rateLimiter: rateLimiter,
	}, nil
}

func (tb *TelegramBot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := tb.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			tb.handleMessage(update.Message)
		} else if update.CallbackQuery != nil {
			tb.handleCallback(update.CallbackQuery)
		}
	}
}

func (tb *TelegramBot) handleMessage(message *tgbotapi.Message) {
	chatID := message.Chat.ID
	text := message.Text

	// Игнорируем старые сообщения
	if time.Since(message.Time()) > 5*time.Minute {
		return
	}

	switch {
	case text == "/start":
		tb.sendWelcomeMessage(chatID)
	case text == "/help":
		tb.sendHelpMessage(chatID)
	case strings.HasPrefix(text, "/scan"):
		tb.handleScanCommand(chatID, text)
	case strings.HasPrefix(text, "/price"):
		tb.handlePriceCommand(chatID, text)
	default:
		// Если сообщение похоже на Steam ID или ссылку
		if tb.isSteamInput(text) {
			tb.handleSteamInput(chatID, text)
		} else {
			tb.sendMessage(chatID, "Не понимаю команду. Используйте /help для справки.")
		}
	}
}

func (tb *TelegramBot) handleCallback(callback *tgbotapi.CallbackQuery) {
	chatID := callback.Message.Chat.ID
	data := callback.Data

	// Отвечаем на callback
	callbackConfig := tgbotapi.NewCallback(callback.ID, "")
	tb.bot.Request(callbackConfig)

	switch {
	case strings.HasPrefix(data, "scan_"):
		parts := strings.Split(data, "_")
		if len(parts) >= 3 {
			steamID := parts[1]
			appID := parts[2]
			tb.scanInventory(chatID, steamID, appID)
		}
	case data == "help":
		tb.sendHelpMessage(chatID)
	}
}

func (tb *TelegramBot) sendWelcomeMessage(chatID int64) {
	text := `🎮 *Steam Inventory Bot*

Привет! Я помогу тебе проверить инвентарь Steam профиля.

*Доступные команды:*
/scan - Сканировать инвентарь
/price - Проверить цену предмета
/help - Справка

*Как использовать:*
1. Отправь Steam ID или ссылку на профиль
2. Выбери игру из списка
3. Получи статистику инвентаря

*Примеры Steam ID:*
• 76561198111717059
• https://steamcommunity.com/id/username
• https://steamcommunity.com/profiles/76561198111717059`

	tb.sendMessage(chatID, text)
}

func (tb *TelegramBot) sendHelpMessage(chatID int64) {
	text := `📋 *Справка по командам*

*/scan* - Сканировать инвентарь
Использование: /scan <steam_id> [app_id]
Пример: /scan 76561198111717059 730

*/price* - Проверить цену предмета
Использование: /price <market_hash_name>
Пример: /price "AK-47 | Redline (Field-Tested)"

*Поддерживаемые игры:*
• CS:GO (730)
• Dota 2 (570)
• TF2 (440)
• Rust (252490)

*Форматы Steam ID:*
• Steam64 ID: 76561198111717059
• Ссылка профиля: https://steamcommunity.com/profiles/76561198111717059
• Пользовательская ссылка: https://steamcommunity.com/id/username`

	tb.sendMessage(chatID, text)
}

func (tb *TelegramBot) handleScanCommand(chatID int64, text string) {
	parts := strings.Fields(text)
	if len(parts) < 2 {
		tb.sendMessage(chatID, "Использование: /scan <steam_id> [app_id]")
		return
	}

	steamID := parts[1]
	appID := "730" // CS:GO по умолчанию
	if len(parts) > 2 {
		appID = parts[2]
	}

	tb.scanInventory(chatID, steamID, appID)
}

func (tb *TelegramBot) handlePriceCommand(chatID int64, text string) {
	parts := strings.Fields(text)
	if len(parts) < 2 {
		tb.sendMessage(chatID, "Использование: /price <market_hash_name>")
		return
	}

	marketName := strings.Join(parts[1:], " ")
	appID := "730" // CS:GO по умолчанию

	tb.sendMessage(chatID, "🔍 Проверяю цену...")

	price := getMarketPrice(appID, marketName, false)
	if price == "" {
		tb.sendMessage(chatID, "❌ Не удалось получить цену для: "+marketName)
		return
	}

	response := fmt.Sprintf("💰 *%s*\nЦена: %s", marketName, price)
	tb.sendMessage(chatID, response)
}

func (tb *TelegramBot) handleSteamInput(chatID int64, text string) {
	// Разрешаем Steam ID
	resolvedID := resolveSteamID(text)
	if resolvedID == "" {
		tb.sendMessage(chatID, "❌ Не удалось распознать Steam ID")
		return
	}

	// Показываем меню выбора игры
	tb.sendGameSelection(chatID, resolvedID)
}

func (tb *TelegramBot) sendGameSelection(chatID int64, steamID string) {
	text := "🎮 Выберите игру для сканирования:"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("CS:GO", fmt.Sprintf("scan_%s_730", steamID)),
			tgbotapi.NewInlineKeyboardButtonData("Dota 2", fmt.Sprintf("scan_%s_570", steamID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("TF2", fmt.Sprintf("scan_%s_440", steamID)),
			tgbotapi.NewInlineKeyboardButtonData("Rust", fmt.Sprintf("scan_%s_252490", steamID)),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	msg.ParseMode = "Markdown"

	tb.bot.Send(msg)
}

func (tb *TelegramBot) scanInventory(chatID int64, steamID, appID string) {
	// Создаем ключ для кэша
	cacheKey := fmt.Sprintf("%s_%s", steamID, appID)

	// Проверяем кэш
	if cachedData, exists := tb.cache.Get(cacheKey); exists {
		tb.sendMessage(chatID, "⚡ Использую кэшированные данные...")
		
		// Формируем отчет из кэшированных данных
		var totalValue, minPrice, maxPrice float64
		var minItem, maxItem string
		
		for i, item := range cachedData {
			if i == 0 {
				minPrice = item.PriceValue
				maxPrice = item.PriceValue
				minItem = item.Name
				maxItem = item.Name
			}
			
			totalValue += item.PriceValue
			
			if item.PriceValue < minPrice {
				minPrice = item.PriceValue
				minItem = item.Name
			}
			if item.PriceValue > maxPrice {
				maxPrice = item.PriceValue
				maxItem = item.Name
			}
		}
		
		gameName := getGameName(appID)
		response := fmt.Sprintf(`📊 *Статистика инвентаря %s* (из кэша)

🎮 Игра: %s
📦 Всего предметов: %d
💵 Общая стоимость: %.2f ₽

📈 *Ценовая статистика:*
• Минимальная: %.2f ₽ (%s)
• Максимальная: %.2f ₽ (%s)`,
			steamID, gameName, len(cachedData), totalValue,
			minPrice, minItem, maxPrice, maxItem)
		
		tb.sendMessage(chatID, response)
		
		// Показываем топ-5 самых дорогих предметов
		if len(cachedData) > 0 {
			tb.sendTopItems(chatID, cachedData)
		}
		return
	}

	tb.sendMessage(chatID, "🔍 Сканирую инвентарь...")

	startTime := time.Now()

	// Ждем разрешения от rate limiter
	tb.rateLimiter.Wait()

	// Разрешаем Steam ID
	resolvedID := resolveSteamID(steamID)
	if resolvedID == "" {
		tb.sendMessage(chatID, "❌ Не удалось разрешить Steam ID")
		return
	}

	// Добавляем таймаут для сканирования
	done := make(chan bool, 1)
	var assets []Asset
	var descriptions []Description
	var totalCount int

	go func() {
		contextID := "2"
		assets, descriptions, totalCount = fetchAllInventory(resolvedID, appID, contextID, false)
		done <- true
	}()

	// Ждем завершения или таймаута (2 минуты)
	select {
	case <-done:
		// Сканирование завершено
	case <-time.After(2 * time.Minute):
		tb.sendMessage(chatID, "⏰ Таймаут сканирования. Инвентарь слишком большой или недоступен.")
		return
	}

	if totalCount == 0 {
		tb.sendMessage(chatID, "❌ Инвентарь пуст или недоступен")
		return
	}

	tb.sendMessage(chatID, fmt.Sprintf("📦 Найдено %d предметов. Обрабатываю цены...", totalCount))

	// Ограничиваем количество предметов для обработки цен (максимум 50)
	maxItems := 50
	if len(assets) > maxItems {
		tb.sendMessage(chatID, fmt.Sprintf("⚠️ Инвентарь большой (%d предметов). Обрабатываю только первые %d для ускорения.", len(assets), maxItems))
		assets = assets[:maxItems]
	}

	// Обрабатываем предметы
	items := processInventoryItems(assets, descriptions, appID, false)

	if len(items) == 0 {
		tb.sendMessage(chatID, "❌ Нет продаваемых предметов в инвентаре")
		return
	}

	// Вычисляем статистику
	var totalValue, minPrice, maxPrice float64
	var minItem, maxItem string

	for i, item := range items {
		if i == 0 {
			minPrice = item.PriceValue
			maxPrice = item.PriceValue
			minItem = item.Name
			maxItem = item.Name
		}

		totalValue += item.PriceValue

		if item.PriceValue < minPrice {
			minPrice = item.PriceValue
			minItem = item.Name
		}
		if item.PriceValue > maxPrice {
			maxPrice = item.PriceValue
			maxItem = item.Name
		}
	}

	duration := time.Since(startTime)

	// Формируем ответ
	gameName := getGameName(appID)
	response := fmt.Sprintf(`📊 *Статистика инвентаря %s*

🎮 Игра: %s
📦 Всего предметов: %d
💰 Продаваемых: %d
💵 Общая стоимость: %.2f ₽

📈 *Ценовая статистика:*
• Минимальная: %.2f ₽ (%s)
• Максимальная: %.2f ₽ (%s)

⏱ Время сканирования: %v`,
		steamID, gameName, totalCount, len(items), totalValue,
		minPrice, minItem, maxPrice, maxItem, duration)

	// Сохраняем в кэш
	tb.cache.Set(cacheKey, items)

	tb.sendMessage(chatID, response)

	// Показываем топ-5 самых дорогих предметов
	if len(items) > 0 {
		tb.sendTopItems(chatID, items)
	}
}

func (tb *TelegramBot) sendTopItems(chatID int64, items []InventoryItem) {
	// Сортируем по цене (убывание)
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].PriceValue < items[j].PriceValue {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	text := "🏆 *Топ-5 самых дорогих предметов:*\n\n"

	topCount := 5
	if len(items) < topCount {
		topCount = len(items)
	}

	for i := 0; i < topCount; i++ {
		item := items[i]
		text += fmt.Sprintf("%d. *%s*\n   💰 %.2f ₽\n\n", i+1, item.Name, item.PriceValue)
	}

	tb.sendMessage(chatID, text)
}

func (tb *TelegramBot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"

	_, err := tb.bot.Send(msg)
	if err != nil {
		log.Printf("Ошибка отправки сообщения: %v", err)
	}
}

func (tb *TelegramBot) isSteamInput(text string) bool {
	// Проверяем, похоже ли на Steam ID или ссылку
	return strings.Contains(text, "steamcommunity.com") ||
		strings.HasPrefix(text, "7656119") ||
		len(text) > 10 && strings.Contains(text, "/")
}

func getGameName(appID string) string {
	switch appID {
	case "730":
		return "CS:GO"
	case "570":
		return "Dota 2"
	case "440":
		return "TF2"
	case "252490":
		return "Rust"
	default:
		return "Неизвестная игра"
	}
}

func processInventoryItems(assets []Asset, descriptions []Description, appID string, debug bool) []InventoryItem {
	descMap := make(map[string]Description)
	for _, desc := range descriptions {
		key := desc.ClassID + "_" + desc.InstanceID
		descMap[key] = desc
	}

	priceCache := make(map[string]string)
	var items []InventoryItem
	processedCount := 0

	for _, asset := range assets {
		key := asset.ClassID + "_" + asset.InstanceID
		desc, found := descMap[key]

		if !found || desc.Marketable != 1 || desc.MarketHashName == "" {
			continue
		}

		price, cached := priceCache[desc.MarketHashName]
		if !cached {
			price = getMarketPrice(appID, desc.MarketHashName, debug)
			priceCache[desc.MarketHashName] = price
			time.Sleep(3 * time.Second) // Уменьшили задержку
		}

		if price == "" {
			continue
		}

		priceValue := parsePrice(price)
		item := InventoryItem{
			Name:       desc.Name,
			MarketName: desc.MarketName,
			Type:       desc.Type,
			Price:      price,
			PriceValue: priceValue,
			AssetID:    asset.AssetID,
		}
		items = append(items, item)
		processedCount++

		// Обновляем прогресс каждые 3 предмета
		if processedCount%3 == 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return items
}

func main() {
	// Получаем токен бота из переменной окружения
	botToken := "8232781922:AAH5k5RX9jhG-9oDa7BW5-9DVpK8kiGv3Mg" // Замените на ваш токен

	bot, err := NewTelegramBot(botToken)
	if err != nil {
		log.Fatal("Ошибка создания бота:", err)
	}

	log.Println("Бот запущен...")
	bot.Start()
}
