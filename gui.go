package main

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type InventoryItem struct {
	Name       string
	MarketName string
	Type       string
	Price      string
	PriceValue float64
	AssetID    string
}

type InventoryData struct {
	Items           []InventoryItem
	TotalItems      int
	MarketableItems int
	TotalValue      float64
	MinPrice        float64
	MaxPrice        float64
	MinPriceItem    string
	MaxPriceItem    string
	Duration        time.Duration
}

func main() {
	createGUI()
}

func createGUI() {
	myApp := app.New()
	myWindow := myApp.NewWindow("Steam Inventory Viewer")
	myWindow.Resize(fyne.NewSize(800, 600))

	// Создаем элементы интерфейса
	steamIDEntry := widget.NewEntry()
	steamIDEntry.SetPlaceHolder("Steam ID или URL профиля")

	gameSelect := widget.NewSelect([]string{
		"Все игры",
		"Counter-Strike 2 (730)",
		"Dota 2 (570)",
		"Steam Items (753)",
		"Team Fortress 2 (440)",
		"PUBG (578080)",
		"Rust (252490)",
	}, nil)
	gameSelect.SetSelected("Все игры")

	debugCheck := widget.NewCheck("Debug режим", nil)

	scanButton := widget.NewButton("Сканировать инвентарь", nil)
	exportButton := widget.NewButton("Экспорт в CSV", nil)

	// Создаем таблицу для результатов
	resultsTable := widget.NewTable(
		func() (int, int) { return 0, 5 },
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Wrapping = fyne.TextWrapWord
			return label
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			label.SetText("")
		},
	)

	// Настраиваем размеры колонок
	resultsTable.SetColumnWidth(0, 200) // Название
	resultsTable.SetColumnWidth(1, 150) // Тип
	resultsTable.SetColumnWidth(2, 100) // Цена
	resultsTable.SetColumnWidth(3, 100) // Стоимость
	resultsTable.SetColumnWidth(4, 120) // Asset ID

	// Создаем лейблы для статистики
	statusLabel := widget.NewLabel("Готов к сканированию")
	progressBar := widget.NewProgressBar()
	progressBar.Hide()

	// Создаем контейнеры
	inputContainer := container.NewVBox(
		widget.NewLabel("Steam ID или URL:"),
		steamIDEntry,
		widget.NewLabel("Игра:"),
		gameSelect,
		debugCheck,
		scanButton,
		exportButton,
	)

	resultsContainer := container.NewBorder(
		widget.NewLabel("Результаты сканирования:"),
		statusLabel,
		nil,
		nil,
		resultsTable,
	)

	mainContainer := container.NewHSplit(inputContainer, resultsContainer)
	mainContainer.SetOffset(0.3)

	// Переменная для хранения данных
	var currentData *InventoryData

	// Обработчик кнопки сканирования
	scanButton.OnTapped = func() {
		steamID := strings.TrimSpace(steamIDEntry.Text)
		if steamID == "" {
			dialog.ShowError(fmt.Errorf("введите Steam ID или URL"), myWindow)
			return
		}

		// Определяем App ID
		var appID string
		switch gameSelect.Selected {
		case "Counter-Strike 2 (730)":
			appID = "730"
		case "Dota 2 (570)":
			appID = "570"
		case "Steam Items (753)":
			appID = "753"
		case "Team Fortress 2 (440)":
			appID = "440"
		case "PUBG (578080)":
			appID = "578080"
		case "Rust (252490)":
			appID = "252490"
		default:
			appID = ""
		}

		// Запускаем сканирование в отдельной горутине
		go func() {
			fyne.DoAndWait(func() {
				statusLabel.SetText("Сканирование...")
				progressBar.Show()
				progressBar.SetValue(0)
				scanButton.Disable()
				exportButton.Disable()
			})

			startTime := time.Now()
			data := scanInventoryGUIWithProgress(steamID, appID, debugCheck.Checked, resultsTable, statusLabel, progressBar)
			duration := time.Since(startTime)
			data.Duration = duration

			// Финальное обновление
			fyne.DoAndWait(func() {
				updateStatusLabel(statusLabel, data)
				currentData = data
				progressBar.SetValue(1.0)
				progressBar.Hide()
				scanButton.Enable()
				exportButton.Enable()
			})
		}()
	}

	// Обработчик кнопки экспорта
	exportButton.OnTapped = func() {
		if currentData == nil {
			dialog.ShowError(fmt.Errorf("нет данных для экспорта"), myWindow)
			return
		}

		dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil {
				dialog.ShowError(err, myWindow)
				return
			}
			if writer == nil {
				return
			}
			defer writer.Close()

			exportToCSV(writer, currentData)
			dialog.ShowInformation("Экспорт", "Данные успешно экспортированы в CSV", myWindow)
		}, myWindow)
	}

	// Создаем финальный контейнер
	content := container.NewVBox(
		widget.NewLabel("Steam Inventory Viewer"),
		mainContainer,
		progressBar,
	)

	myWindow.SetContent(content)
	myWindow.ShowAndRun()
}

func scanInventoryGUIWithProgress(steamID, appID string, debug bool, table *widget.Table, statusLabel *widget.Label, progressBar *widget.ProgressBar) *InventoryData {
	// Разрешаем Steam ID
	resolvedID := resolveSteamID(steamID)
	if resolvedID == "" {
		return &InventoryData{}
	}

	data := &InventoryData{
		Items: []InventoryItem{},
	}

	if appID == "" {
		// Сканируем все игры
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

		for _, game := range games {
			assets, descriptions, totalCount := fetchAllInventory(resolvedID, game.appID, game.context, debug)
			if totalCount == 0 {
				continue
			}

			gameItems := processInventoryItems(assets, descriptions, game.appID, debug)
			data.Items = append(data.Items, gameItems...)
			data.TotalItems += totalCount
		}
	} else {
		// Сканируем конкретную игру
		contextID := "2"
		if appID == "753" {
			contextID = "6"
		}

		assets, descriptions, totalCount := fetchAllInventory(resolvedID, appID, contextID, debug)
		data.TotalItems = totalCount
		data.Items = processInventoryItems(assets, descriptions, appID, debug)
	}

	// Вычисляем статистику
	calculateStatistics(data)

	// Обновляем интерфейс в реальном времени
	fyne.DoAndWait(func() {
		updateResultsTable(table, data)
		progress := float64(len(data.Items)) / float64(data.TotalItems)
		if progress > 1.0 {
			progress = 1.0
		}
		progressBar.SetValue(progress)
		statusLabel.SetText(fmt.Sprintf("Обработано %d из %d предметов...", len(data.Items), data.TotalItems))
	})

	return data
}

func processInventoryItems(assets []Asset, descriptions []Description, appID string, debug bool) []InventoryItem {
	descMap := make(map[string]Description)
	for _, desc := range descriptions {
		key := desc.ClassID + "_" + desc.InstanceID
		descMap[key] = desc
	}

	priceCache := make(map[string]string)
	var items []InventoryItem

	for i, asset := range assets {
		key := asset.ClassID + "_" + asset.InstanceID
		desc, found := descMap[key]

		if !found || desc.Marketable != 1 || desc.MarketHashName == "" {
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
		item := InventoryItem{
			Name:       desc.Name,
			MarketName: desc.MarketName,
			Type:       desc.Type,
			Price:      price,
			PriceValue: priceValue,
			AssetID:    asset.AssetID,
		}
		items = append(items, item)

		// Обновляем прогресс каждые 5 предметов
		if i%5 == 0 {
			time.Sleep(100 * time.Millisecond) // Небольшая пауза для обновления UI
		}
	}

	return items
}

func calculateStatistics(data *InventoryData) {
	if len(data.Items) == 0 {
		return
	}

	data.MarketableItems = len(data.Items)
	data.MinPrice = data.Items[0].PriceValue
	data.MaxPrice = data.Items[0].PriceValue
	data.MinPriceItem = data.Items[0].Name
	data.MaxPriceItem = data.Items[0].Name

	for _, item := range data.Items {
		data.TotalValue += item.PriceValue
		if item.PriceValue < data.MinPrice {
			data.MinPrice = item.PriceValue
			data.MinPriceItem = item.Name
		}
		if item.PriceValue > data.MaxPrice {
			data.MaxPrice = item.PriceValue
			data.MaxPriceItem = item.Name
		}
	}
}

func updateResultsTable(table *widget.Table, data *InventoryData) {
	table.Length = func() (int, int) {
		return len(data.Items), 5
	}

	table.CreateCell = func() fyne.CanvasObject {
		label := widget.NewLabel("")
		label.Wrapping = fyne.TextWrapWord
		return label
	}

	table.UpdateCell = func(id widget.TableCellID, obj fyne.CanvasObject) {
		label := obj.(*widget.Label)
		if id.Row >= len(data.Items) {
			label.SetText("")
			return
		}

		item := data.Items[id.Row]
		var text string
		switch id.Col {
		case 0:
			text = truncateString(item.Name, 30)
		case 1:
			text = truncateString(item.Type, 20)
		case 2:
			text = item.Price
		case 3:
			text = fmt.Sprintf("%.2f руб.", item.PriceValue)
		case 4:
			text = truncateString(item.AssetID, 15)
		}
		label.SetText(text)
	}

	table.Refresh()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func updateStatusLabel(label *widget.Label, data *InventoryData) {
	status := fmt.Sprintf("Найдено %d продаваемых предметов из %d общих. Стоимость: %.2f руб. Время: %v",
		data.MarketableItems, data.TotalItems, data.TotalValue, data.Duration.Round(time.Second))

	if data.MarketableItems > 0 {
		status += fmt.Sprintf(" | Мин: %.2f руб. (%s) | Макс: %.2f руб. (%s)",
			data.MinPrice, data.MinPriceItem, data.MaxPrice, data.MaxPriceItem)
	}

	label.SetText(status)
}

func exportToCSV(writer fyne.URIWriteCloser, data *InventoryData) {
	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()

	// Заголовки
	csvWriter.Write([]string{"Название", "Тип", "Цена", "Стоимость (руб.)", "Asset ID"})

	// Данные
	for _, item := range data.Items {
		csvWriter.Write([]string{
			item.Name,
			item.Type,
			item.Price,
			strconv.FormatFloat(item.PriceValue, 'f', 2, 64),
			item.AssetID,
		})
	}
}
