# Telegram Bot для Steam Inventory

## Настройка бота

### 1. Создание бота в Telegram

1. Найдите @BotFather в Telegram
2. Отправьте команду `/newbot`
3. Введите имя бота (например: "Steam Inventory Bot")
4. Введите username бота (например: "steam_inventory_bot")
5. Скопируйте полученный токен

### 2. Настройка токена

Замените `YOUR_BOT_TOKEN_HERE` в файле `telegram_bot.go` на ваш токен:

```go
botToken := "1234567890:ABCdefGHIjklMNOpqrsTUVwxyz" // Ваш токен
```

### 3. Компиляция и запуск

```bash
# Компиляция
go build -o steam-telegram-bot.exe telegram_bot.go steam_api.go

# Запуск
./steam-telegram-bot.exe
```

## Команды бота

### Основные команды:
- `/start` - Приветствие и инструкции
- `/help` - Справка по командам
- `/scan <steam_id> [app_id]` - Сканировать инвентарь
- `/price <market_hash_name>` - Проверить цену предмета

### Примеры использования:

1. **Сканирование инвентаря:**
   ```
   /scan 76561198111717059 730
   ```

2. **Проверка цены:**
   ```
   /price "AK-47 | Redline (Field-Tested)"
   ```

3. **Отправка Steam ID или ссылки:**
   ```
   76561198111717059
   https://steamcommunity.com/profiles/76561198111717059
   https://steamcommunity.com/id/username
   ```

## Поддерживаемые игры:
- CS:GO (730)
- Dota 2 (570) 
- TF2 (440)
- Rust (252490)

## Возможности:
- ✅ Автоматическое разрешение Steam ID
- ✅ Интерактивное меню выбора игры
- ✅ Статистика инвентаря с ценами
- ✅ Топ-5 самых дорогих предметов
- ✅ Проверка цен отдельных предметов
- ✅ Красивое форматирование вывода
