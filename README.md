# Trend Service - Сервис трендовых поисковых запросов

Highload сервис для отображения топа самых популярных поисковых запросов за последние 5 минут в реальном времени.

## Оглавление

- [Архитектура](#архитектура)
- [Контракт данных](#контракт-данных)
- [Быстрый старт](#быстрый-старт)
- [API](#api)
- [Конфигурация](#конфигурация)
- [Архитектурные решения](#архитектурные-решения)
- [Trade-offs и компромиссы](#trade-offs-и-компромиссы)
- [Метрики](#метрики)

## Архитектура

### Компоненты системы

```
┌─────────────┐      ┌──────────────┐      ┌─────────────┐
│   Kafka     │─────▶│   Consumer   │─────▶│ Aggregator  │
│  (брокер)   │      │  (воркеры)   │      │  (шарды)    │
└─────────────┘      └──────────────┘      └─────────────┘
                                                   │
                                                   ▼
┌─────────────┐      ┌──────────────┐      ┌─────────────┐
│   Redis     │◀────▶│  Stoplist    │◀─────│ HTTP Server │
│ (стоп-лист) │      │              │      │   (API)     │
└─────────────┘      └──────────────┘      └─────────────┘
                                                   │
                                                   ▼
                                            ┌─────────────┐
                                            │ Prometheus  │
                                            │  (метрики)  │
                                            └─────────────┘
```

### Принципы проектирования

Проект следует принципам **SOLID** и **чистой архитектуры**:

1. **Single Responsibility Principle (SRP)**: Каждый компонент отвечает за одну задачу
   - `Aggregator` - агрегация и подсчет запросов
   - `Consumer` - чтение из Kafka
   - `Stoplist` - управление стоп-листом
   - `HTTPServer` - обработка HTTP запросов

2. **Open/Closed Principle (OCP)**: Система открыта для расширения, закрыта для модификации
   - Интерфейсы `Consumer` и `CacheClient` позволяют легко заменить Kafka на NATS/RabbitMQ, а Redis на другое хранилище

3. **Liskov Substitution Principle (LSP)**: Любая реализация интерфейса может быть заменена
   - `KafkaConsumer` реализует `Consumer`
   - `RedisClient` реализует `CacheClient`

4. **Interface Segregation Principle (ISP)**: Интерфейсы специфичны и минимальны
   - `Consumer` содержит только методы для чтения сообщений
   - `CacheClient` содержит только необходимые операции с кешем

5. **Dependency Inversion Principle (DIP)**: Зависимости направлены на абстракции
   - Все компоненты зависят от интерфейсов, а не от конкретных реализаций

### Структура проекта

```
.
├── cmd/
│   └── server/              # Точка входа приложения
├── internal/
│   ├── app/                 # Сборка и инициализация приложения
│   ├── domain/              # Доменные сущности (Event, TopEntry)
│   ├── broker/
│   │   └── consumer/        # Интерфейс и реализации консьюмеров
│   │       └── kafka/       # Kafka консьюмер
│   ├── repository/
│   │   └── cache/           # Интерфейс и реализации кеша
│   │       └── redis/       # Redis клиент
│   ├── usecase/
│   │   ├── aggregator/      # Агрегация и подсчет топа
│   │   │   └── adapter/     # Реализация агрегатора (с дедупликацией)
│   │   ├── stoplist/        # Управление стоп-листом
│   │   └── mocks/           # Mock объекты для тестирования
│   ├── http-server/         # HTTP API сервер
│   │   ├── handler/         # Обработчики HTTP запросов
│   │   └── routes/          # Определение маршрутов
│   ├── config/              # Конфигурация приложения
│   └── metrics/             # Prometheus метрики
├── pkg/
│   ├── contract/            # Контракт данных из Kafka
│   └── lib/
│       └── logger/
│           └── zaplogger/   # Логирование (zap)
├── scripts/                 # Утилиты и вспомогательные скрипты
├── docker-compose.yaml      # Docker Compose для локального запуска
├── Dockerfile               # Docker image конфигурация
├── Makefile                 # Команды для разработки и тестирования
├── prometheus.yml           # Конфигурация Prometheus
└── go.mod                   # Go модули
```

## Контракт данных

### Формат события в Kafka

```json
{
  "query": "iphone 15 pro",
  "user_id": "user_12345",
  "session_id": "sess_abc123",
  "request_id": "req_xyz789",
  "timestamp": 1735228800
}
```



**Что НЕ включено и почему:**
- Геолокация, категория, фильтры - не нужны для глобального топа
- IP адрес - избыточно при наличии `user_id`/`session_id`
- User-Agent - не влияет на подсчет трендов

## Быстрый старт

### Требования

- Docker и Docker Compose
- Go 1.25+ (для локальной разработки)

### Запуск через Docker Compose

```bash
# Клонируем репозиторий
git clone <repository-url>
cd trend_service

# Запускаем все сервисы
docker-compose up --build

# Проверяем статус
docker-compose ps

# Смотрим логи
docker-compose logs -f trending
```

Сервисы будут доступны:
- **Trend Service API**: http://localhost:8080
- **Prometheus**: http://localhost:9090
- **Kafka**: localhost:9092
- **Redis**: localhost:6379



### Генерация тестовых событий

```bash
# Используем скрипт для генерации событий
go run scripts/produce_events.go
```

## API

### 1. Получение топа запросов

**GET** `/top?n=10`

Возвращает топ-N самых популярных поисковых запросов за последние 5 минут.

**Параметры:**
- `n` (optional, default=10, max=100) - количество запросов в топе

**Пример запроса:**
```bash
curl http://localhost:8080/top?n=5
```

**Пример ответа:**
```json
{
  
  "generated_at": 1735228800123456789,
  "items": [
    {
      "query": "iphone 15 pro",
      "count": 1523
    },
    {
      "query": "samsung galaxy s24",
      "count": 1245
    },
    {
      "query": "macbook air m3",
      "count": 987
    }
  ],
  "window_seconds": 300
}
```

**Заголовки ответа:**
- `Cache-Control: public, max-age=1` - позволяет кешировать на edge/CDN

### 2. Управление стоп-листом

#### Получение стоп-листа

**GET** `/stoplist`

```bash
curl http://localhost:8080/stoplist
```

**Ответ:**
```json
{
  "items": ["spam", "test", "xxx"]
}
```

#### Добавление/удаление слов

**POST** `/stoplist`

```bash
# Добавить слова в стоп-лист
curl -X POST http://localhost:8080/stoplist \
  -H "Content-Type: application/json" \
  -d '{"add": ["spam", "test"]}'

# Удалить слова из стоп-листа
curl -X POST http://localhost:8080/stoplist \
  -H "Content-Type: application/json" \
  -d '{"remove": ["test"]}'

# Добавить и удалить одновременно
curl -X POST http://localhost:8080/stoplist \
  -H "Content-Type: application/json" \
  -d '{"add": ["new_word"], "remove": ["old_word"]}'
```

**Ответ:** `204 No Content`

### 3. Health Check

**GET** `/healthz`

```bash
curl http://localhost:8080/healthz
```

**Ответ:** `ok`

### 4. Метрики Prometheus

**GET** `/metrics`

```bash
curl http://localhost:8080/metrics
```

## Конфигурация

Конфигурация через переменные окружения (файл `.env`):

```bash
# HTTP Server
SERVER_ADDRESS=:8080
SERVER_READ_TIMEOUT=15s
SERVER_WRITE_TIMEOUT=15s
SERVER_IDLE_TIMEOUT=120s
SERVER_SHUTDOWN_TIMEOUT=30s

# Aggregator
WINDOW_SECONDS=300              # Окно агрегации (5 минут)
SNAPSHOT_INTERVAL=500ms         # Частота пересчета топа
DEFAULT_TOP_N=10                # Размер топа по умолчанию
MAX_TOP_N=100                   # Максимальный размер топа
DEDUP_TTL=10s                   # TTL дедупликации
SHARDS=16                       # Количество шардов
WORKER_COUNT=4                  # Количество воркеров Kafka
MAX_CLOCK_SKEW=60s              # Допустимое расхождение часов

# Kafka
BROKERS=localhost:9092
TOPIC=search-queries
GROUP_ID=trend-service-group

# Redis
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
```

## Архитектурные решения

### 1. Агрегатор с шардированием

**Проблема:** Высокая конкуренция за блокировки при записи счетчиков.

**Решение:** 
- Окно разбито на N бакетов по 1 секунде
- Шардирование по `hash(query)` - снижает contention
- На запись лочим только текущий бакет конкретного шарда

**Структура данных:**
```go
type shard struct {
    mu      sync.Mutex
    buckets []map[string]int64  // ring buffer
    head    int                 // индекс текущего бакета
    headSec int64               // unix-секунда текущего бакета
}
```

### 2. Lock-free чтение топа

**Проблема:** Чтение топа в 10-50 раз чаще записи. Блокировки на чтении убьют производительность.

**Решение:**
- Снапшот топа пересчитывается фоновым воркером раз в 500ms
- Публикуется через `atomic.Pointer` - чтение становится lock-free
- Клиенты всегда получают последний опубликованный снапшот без блокировок

```go
type Aggregator struct {
    current atomic.Pointer[Snapshot]
    // ...
}

func (a *Aggregator) Snapshot() *Snapshot {
    return a.current.Load()  // lock-free!
}
```

### 3. Стоп-лист через Redis

**Проблема:** Стоп-лист должен работать "на лету" и синхронизироваться между инстансами.

**Решение:**
- Redis Set для хранения стоп-листа
- Проверка через `SISMEMBER` - O(1)
- Изменения видны всем инстансам мгновенно
- Персистентность через AOF

**Альтернативы:**
- ❌ In-memory map - не синхронизируется между инстансами
- ❌ Database - слишком медленно для highload
- ✅ Redis - быстро, персистентно, распределенно

### 4. Дедупликация накруток

**Проблема:** Конкуренты/парсеры генерируют аномальные всплески одних и тех же запросов.

**Решение:**
- Один пользователь = один голос за окно дедупа (10 секунд)
- LRU cache с TTL для хранения `(userID, query)` пар
- Шардирование кеша для снижения contention

```go
type dedupCache struct {
    shards []*dedupShard
    ttl    time.Duration
}

func (d *dedupCache) shouldCount(userID, query string, nowUnix int64) bool {
    key := userID + ":" + query
    // Проверяем, голосовал ли пользователь за этот запрос недавно
}
```

### 5. Нормализация запросов

**Проблема:** "iPhone 15", "iphone 15", "IPHONE  15" - это один запрос или три?

**Решение:**
- Приведение к lowercase
- Схлопывание множественных пробелов
- Trim пробелов по краям

```go
func Normalize(q string) string {
    q = strings.ToLower(strings.TrimSpace(q))
    // collapse whitespace
    // ...
}
```

### 6. Graceful Shutdown

**Проблема:** При остановке сервиса нужно корректно завершить все операции.

**Решение:**
- Контекст с отменой по сигналу (SIGINT/SIGTERM)
- Последовательное закрытие компонентов:
  1. HTTP сервер (дожидаемся завершения активных запросов)
  2. Kafka консьюмер (коммитим оффсеты)
  3. Redis клиент (закрываем соединения)
  4. Логгер (синхронизируем буферы)

## Trade-offs и компромиссы

### 1. Eventual Consistency

**Компромисс:** Топ обновляется раз в 500ms, а не в реальном времени.

**Обоснование:**
- Пересчет топа на каждый запрос = O(N log K) на каждый RPS
- При 10k RPS это 10k пересчетов в секунду
- Снапшот раз в 500ms = 2 пересчета в секунду
- Задержка 500ms для виджета "Сейчас ищут" приемлема

### 2. At-Least-Once семантика Kafka

**Компромисс:** Возможны дубликаты событий.

**Обоснование:**
- Exactly-once в Kafka сложнее и медленнее
- Для трендов дубликат события не критичен (±1 к счетчику)
- Дедупликация по userID частично решает проблему

### 3. Стоп-лист применяется на чтении

**Компромисс:** Запросы из стоп-листа все равно агрегируются.

**Обоснование:**
- Фильтрация на записи требует проверки Redis на каждое событие
- При 100k events/sec это 100k запросов к Redis
- Фильтрация на чтении = 1 проверка на элемент топа (max 100)
- Изменения стоп-листа применяются мгновенно без пересчета

### 4. Фиксированное окно 5 минут

**Компромисс:** Нельзя запросить топ за другой период.

**Обоснование:**
- Скользящее окно произвольной длины требует хранения всех событий
- При 100k events/sec за 5 минут = 30M событий в памяти
- Фиксированное окно = только счетчики в 300 бакетах

### 5. Ограничение размера топа (max 100)

**Компромисс:** Нельзя запросить топ-1000.

**Обоснование:**
- Снапшот хранит топ-300 (с запасом под стоп-лист)
- Больший топ требует больше памяти и времени на пересчет
- Для виджета топ-100 более чем достаточно

## Метрики

Сервис экспортирует метрики в формате Prometheus на `/metrics`:

### Kafka Consumer

- `trending_events_consumed_total` - всего событий прочитано
- `trending_events_accepted_total` - событий принято в агрегатор
- `trending_events_dropped_total{reason}` - событий отброшено (по причинам)

### HTTP API

- `trending_top_requests_total` - запросов к `/top`
- `trending_top_latency_seconds` - латентность `/top` (histogram)

### Aggregator

- `trending_snapshot_build_seconds` - время пересчета снапшота (histogram)

### Пример запросов в Prometheus

```promql
# RPS на /top
rate(trending_top_requests_total[1m])

# P99 латентность /top
histogram_quantile(0.99, rate(trending_top_latency_seconds_bucket[5m]))

# Процент отброшенных событий
rate(trending_events_dropped_total[1m]) / rate(trending_events_consumed_total[1m])
```

## Проблемы в продуктовой постановке и решения

### 1. Неопределенность "реального времени"

**Проблема:** "В реальном времени" - это сколько? Миллисекунды? Секунды?

**Решение:** Снапшот обновляется каждые 500ms. Это баланс между актуальностью и производительностью.

### 2. Отсутствие SLA по латентности

**Проблема:** "Максимально быстро" - это сколько миллисекунд?

**Решение:** 
- Lock-free чтение снапшота = ~100ns
- Фильтрация стоп-листа = O(N) проверок Redis
- Целевая латентность P99 < 10ms

### 3. Определение "накрутки"

**Проблема:** Как отличить накрутку от легитимного всплеска интереса?

**Решение:**
- Дедупликация: один пользователь = один голос за 10 секунд
- Не решает проблему ботнетов, но отсекает простые скрипты

### 4. Синхронизация стоп-листа

**Проблема:** Как синхронизировать стоп-лист между инстансами?

**Решение:** Redis как единый источник правды. Все инстансы читают из одного Set.

### 5. Обработка опоздавших событий

**Проблема:** Что делать с событиями, пришедшими с задержкой?

**Решение:**
- Допускаем расхождение часов до 60 секунд (`MAX_CLOCK_SKEW`)
- События старше окна (5 минут) отбрасываются
- Метрика `events_dropped{reason="dedup_or_window"}` для мониторинга

## Тестирование

### Unit-тесты

```bash
# Запуск всех тестов
go test ./...

# С покрытием
go test -cover ./...

# Конкретный пакет
go test ./internal/aggregator/...
```

### Нагрузочное тестирование

```bash
# Установка hey
go install github.com/rakyll/hey@latest

# Тест /top endpoint
hey -z 60s -n 100000 -c 1000 'http://localhost:8080/top?n=10' 

Summary:
  Total:        60.0277 secs
  Slowest:      0.6268 secs
  Fastest:      0.0004 secs
  Average:      0.0600 secs
  Requests/sec: 29042.0256
  
  Total data:   118546236 bytes
  Size/request: 118 bytes

Response time histogram:
  0.000 [1]     |
  0.063 [906050]        |■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  0.126 [81377] |■■■■
  0.188 [9604]  |
  0.251 [1304]  |
  0.314 [520]   |
  0.376 [364]   |
  0.439 [330]   |
  0.502 [221]   |
  0.564 [101]   |
  0.627 [128]   |


Latency distribution:
  10%% in 0.0132 secs
  25%% in 0.0202 secs
  50%% in 0.0296 secs
  75%% in 0.0428 secs
  90%% in 0.0616 secs
  95%% in 0.0779 secs
  99%% in 0.1362 secs

Details (average, fastest, slowest):
  DNS+dialup:   0.0000 secs, 0.0000 secs, 0.1386 secs
  DNS-lookup:   0.0000 secs, 0.0000 secs, 0.0064 secs
  req write:    0.0000 secs, 0.0000 secs, 0.0090 secs
  resp wait:    0.0598 secs, 0.0003 secs, 0.5248 secs
  resp read:    0.0000 secs, 0.0000 secs, 0.0190 secs

Status code distribution:
  [200] 1000000 responses

```

```bash
go run scripts/produce_events.go -brokers=localhost:9092 -topic=search-queries -rps=15000 -duration=30
2026/05/28 23:24:08 Starting event producer: brokers=localhost:9092, topic=search-queries, rps=15000
2026/05/28 23:24:08 Producing events at 15000 RPS...
2026/05/28 23:24:10 Sent: 25000 events, Elapsed: 2s, Actual RPS: 14715.29
2026/05/28 23:24:11 Sent: 50000 events, Elapsed: 3s, Actual RPS: 14849.00
2026/05/28 23:24:13 Sent: 75000 events, Elapsed: 5s, Actual RPS: 14544.94
2026/05/28 23:24:15 Sent: 100000 events, Elapsed: 7s, Actual RPS: 14621.74
2026/05/28 23:24:16 Sent: 125000 events, Elapsed: 9s, Actual RPS: 14655.34
2026/05/28 23:24:18 Sent: 150000 events, Elapsed: 10s, Actual RPS: 14688.66
2026/05/28 23:24:20 Sent: 175000 events, Elapsed: 12s, Actual RPS: 14583.40
2026/05/28 23:24:22 Sent: 200000 events, Elapsed: 14s, Actual RPS: 14605.02
2026/05/28 23:24:23 Sent: 225000 events, Elapsed: 15s, Actual RPS: 14608.90
2026/05/28 23:24:25 Sent: 250000 events, Elapsed: 17s, Actual RPS: 14580.47
2026/05/28 23:24:27 Sent: 275000 events, Elapsed: 19s, Actual RPS: 14617.49
2026/05/28 23:24:28 Sent: 300000 events, Elapsed: 21s, Actual RPS: 14633.63
2026/05/28 23:24:30 Sent: 325000 events, Elapsed: 22s, Actual RPS: 14657.66
2026/05/28 23:24:32 Sent: 350000 events, Elapsed: 24s, Actual RPS: 14681.24
2026/05/28 23:24:33 Sent: 375000 events, Elapsed: 26s, Actual RPS: 14701.69
2026/05/28 23:24:35 Sent: 400000 events, Elapsed: 27s, Actual RPS: 14719.96
2026/05/28 23:24:37 Sent: 425000 events, Elapsed: 29s, Actual RPS: 14702.54
2026/05/28 23:24:38 Duration reached. Total events sent: 441368
2026/05/28 23:24:38 Producer finished. Total events: 441368, Duration: 30s
```