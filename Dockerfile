# Используем официальный Go образ
FROM golang:1.21-alpine AS builder

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем go.mod и go.sum
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем исходный код
COPY . .

# Компилируем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Финальный образ
FROM alpine:latest

# Устанавливаем ca-certificates для HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Копируем скомпилированное приложение
COPY --from=builder /app/main .

# Запускаем приложение
CMD ["./main"]
