# Steam Inventory Telegram Bot

Telegram бот для просмотра инвентаря Steam профилей.

## Возможности

- 🔍 Сканирование инвентаря Steam профилей
- 💰 Получение цен с торговой площадки
- 📊 Статистика по инвентарю
- 🎮 Поддержка всех игр Steam

## Команды

- `/start` - Начать работу с ботом
- `/help` - Список команд
- `/scan <steam_id>` - Сканировать инвентарь
- `/price <item_name>` - Найти цену предмета

## Установка

1. Клонируйте репозиторий
2. Установите зависимости: `go mod download`
3. Создайте бота через @BotFather
4. Установите переменную окружения `BOT_TOKEN`
5. Запустите: `go run telegram_bot.go steam_api.go`

## Railway Deploy

[![Deploy on Railway](https://railway.app/button.svg)](https://railway.app/template/your-template-id)
