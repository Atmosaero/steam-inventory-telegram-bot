package main

import (
	"sync"
	"time"
)

// Кэш для хранения результатов сканирования
type Cache struct {
	items map[string]CachedInventory
	mutex sync.RWMutex
	ttl   time.Duration
}

type CachedInventory struct {
	Data      []InventoryItem
	ExpiresAt time.Time
	CreatedAt time.Time
}

// Создаем новый кэш
func NewCache(ttl time.Duration) *Cache {
	cache := &Cache{
		items: make(map[string]CachedInventory),
		ttl:   ttl,
	}

	// Запускаем очистку устаревших данных
	go cache.cleanup()

	return cache
}

// Получаем данные из кэша
func (c *Cache) Get(key string) ([]InventoryItem, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	// Проверяем, не истек ли срок
	if time.Now().After(item.ExpiresAt) {
		return nil, false
	}

	return item.Data, true
}

// Сохраняем данные в кэш
func (c *Cache) Set(key string, data []InventoryItem) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items[key] = CachedInventory{
		Data:      data,
		ExpiresAt: time.Now().Add(c.ttl),
		CreatedAt: time.Now(),
	}
}

// Очистка устаревших данных
func (c *Cache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mutex.Lock()
		now := time.Now()
		for key, item := range c.items {
			if now.After(item.ExpiresAt) {
				delete(c.items, key)
			}
		}
		c.mutex.Unlock()
	}
}

// Rate limiter для контроля запросов
type RateLimiter struct {
	requests chan struct{}
	ticker   *time.Ticker
}

// Создаем rate limiter
func NewRateLimiter(interval time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(chan struct{}, 1),
		ticker:   time.NewTicker(interval),
	}

	// Запускаем разрешение запросов
	go rl.allowRequests()

	return rl
}

// Разрешаем запросы с интервалом
func (rl *RateLimiter) allowRequests() {
	for range rl.ticker.C {
		select {
		case rl.requests <- struct{}{}:
		default:
		}
	}
}

// Ждем разрешения на запрос
func (rl *RateLimiter) Wait() {
	<-rl.requests
}

// Останавливаем rate limiter
func (rl *RateLimiter) Stop() {
	rl.ticker.Stop()
}
