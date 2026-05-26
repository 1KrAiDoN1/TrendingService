# 🚀 Быстрый старт Trend Service

## За 5 минут до первого запроса

### 1. Запуск всех сервисов

```bash
# Клонируем репозиторий
git clone <repository-url>
cd trend_service

# (Опционально) Настройте конфигурацию
cp .env.example .env
# Отредактируйте .env если нужно изменить параметры

# Запускаем все сервисы (Kafka, Redis, Trend Service, Prometheus)
docker-compose up -d

# Проверяем статус
docker-compose ps
```

**Примечание:** Все переменные окружения имеют значения по умолчанию, поэтому `.env` файл опционален для Docker Compose.

**Ожидаемый вывод:**
```
NAME                COMMAND                  SERVICE             STATUS
trend_service-kafka-1       "/etc/confluent/dock…"   kafka               running
trend_service-prometheus-1  "/bin/prometheus --c…"   prometheus          running
trend_service-redis-1       "docker-entrypoint.s…"   redis               running
trend_service-trending-1    "/trending"              trending            running
trend_service-zookeeper-1   "/etc/confluent/dock…"   zookeeper           running
```

### 2. Проверка работоспособности

```bash
# Health check
curl http://localhost:8080/healthz
# Ответ: ok

# Получить топ (пока пустой)
curl http://localhost:8080/top | jq
```

### 3. Генерация тестовых данных

```bash
# Генерируем события в Kafka (1000 RPS, 60 секунд)
make produce

# Или с высокой нагрузкой (5000 RPS, 30 секунд)
make produce-high
```

### 4. Проверка топа

```bash
# Подождите 5-10 секунд и запросите топ
curl http://localhost:8080/top?n=5 | jq
```

**Пример ответа:**
```json
{
  "window_seconds": 300,
  "generated_at": 1735228800123456789,
  "items": [
    {
      "query": "iphone 15 pro",
      "count": 523
    },
    {
      "query": "samsung galaxy s24",
      "count": 412
    },
    {
      "query": "macbook air m3",
      "count": 387
    }
  ]
}
```

### 5. Работа со стоп-листом

```bash
# Добавить слова в стоп-лист
curl -X POST http://localhost:8080/stoplist \
  -H "Content-Type: application/json" \
  -d '{"add": ["spam", "test"]}'

# Проверить стоп-лист
curl http://localhost:8080/stoplist | jq

# Удалить слово
curl -X POST http://localhost:8080/stoplist \
  -H "Content-Type: application/json" \
  -d '{"remove": ["test"]}'
```

### 6. Просмотр метрик в Prometheus

```bash
# Открыть Prometheus в браузере
make prometheus
# Или вручную: http://localhost:9090
```

**Полезные запросы:**
- RPS: `rate(trending_top_requests_total[1m])`
- P99 Latency: `histogram_quantile(0.99, rate(trending_top_latency_seconds_bucket[5m]))`

### 7. Нагрузочное тестирование

```bash
# Установить hey (если еще не установлен)
make install-tools

# Запустить полный набор тестов
make load

# Или простой тест
make load-simple
```

---

## Полезные команды

```bash
# Просмотр логов
docker-compose logs -f trending

# Перезапуск сервиса
docker-compose restart trending

# Остановка всех сервисов
docker-compose down

# Остановка с удалением данных
docker-compose down -v

# Тестовые запросы к API
make api-test
```

---

## Структура проекта

```
trend_service/
├── cmd/server/          # Точка входа
├── internal/
│   ├── app/             # Инициализация приложения
│   ├── aggregator/      # Агрегация топа
│   ├── consumer/        # Kafka консьюмер
│   ├── cache/           # Redis клиент
│   ├── stoplist/        # Стоп-лист
│   ├── http-server/     # HTTP API (Gin)
│   ├── config/          # Конфигурация
│   └── metrics/         # Prometheus метрики
├── pkg/
│   ├── contract/        # Контракт данных Kafka
│   └── lib/logger/      # Логирование (zap)
├── scripts/
│   ├── produce_events.go  # Генератор событий
│   └── load_test.sh       # Нагрузочное тестирование
├── docker-compose.yaml
├── Dockerfile
├── Makefile
└── README.md
```

---

## Troubleshooting

### Сервис не запускается

```bash
# Проверить логи
docker-compose logs trending

# Проверить порты
lsof -i :8080
lsof -i :9092
lsof -i :6379
```

### Kafka не доступна

```bash
# Перезапустить Kafka
docker-compose restart kafka zookeeper

# Подождать 30 секунд для инициализации
```

### Redis не доступен

```bash
# Проверить Redis
docker-compose logs redis

# Перезапустить
docker-compose restart redis
```

### Метрики не отображаются

```bash
# Проверить targets в Prometheus
open http://localhost:9090/targets

# Убедиться, что trend-service в статусе UP
```

---

## Следующие шаги

1. **Изучите README.md** - подробная документация
2. **Изучите PROMETHEUS_GUIDE.md** - работа с метриками
3. **Запустите нагрузочные тесты** - `make load`
4. **Настройте стоп-лист** - добавьте нежелательные слова
5. **Интегрируйте с вашим приложением** - используйте API

---

## Полезные ссылки

- **API**: http://localhost:8080
- **Prometheus**: http://localhost:9090
- **Health Check**: http://localhost:8080/healthz
- **Metrics**: http://localhost:8080/metrics

---

## Поддержка

Для вопросов и предложений создавайте Issue в репозитории.
