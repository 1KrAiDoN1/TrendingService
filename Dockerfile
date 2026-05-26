FROM golang:1.26.1-alpine AS build

WORKDIR /src

# Копируем файлы зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем бинарник
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/trending ./cmd/server/main.go

# Финальный образ
FROM alpine:latest

# Копируем бинарник
COPY --from=build /out/trending /trending

EXPOSE 8080

ENTRYPOINT ["/trending"]