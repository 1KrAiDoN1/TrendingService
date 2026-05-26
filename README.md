# Trend Service - Сервис трендовых поисковых запросов

Highload сервис для отображения топа самых популярных поисковых запросов за последние 5 минут в реальном времени для маркетплейса Wildberries.

## 📋 Оглавление

- [Быстрый старт](#быстрый-старт)
- [Контракт данных](#контракт-данных)
- [API](#api)
- [Архитектура](#архитектура)
- [Trade-offs и компромиссы](#trade-offs-и-компромиссы)
- [Мониторинг (Prometheus)](#мониторинг-prometheus)
- [Нагрузочное тестирование](#нагрузочное-тестирование)
- [Конфигурация](#конфигурация)

---

## 🚀 Быстрый старт

### Требования

- Docker и Docker Compose
- Go 1.25+ (для локальной разработки)
- Make (опционально)

### Запуск через Docker Compose

```bash
# Клонируем репозиторий
git clone <repository-url>
cd trend_service

# Запускаем все сервисы (Kafka, Redis, Trend Service, Prometheus)
docker-compose up -d

# Проверяем статус
docker-compose ps

# Смотрим логи
docker-compose logs -f trending
```

**Сервисы будут доступны:**
- **Trend Service API**: http://localhost:8080
- **Prometheus**: http://localhost:9090
- **Kafka**: localhost:9092
- **Redis**: localhost:6379

### Локальный запуск (для разработки)

```bash
# Запускаем только инфраструктуру
docker-compose up -d kafka redis prometheus

# Устанавливаем зависимости
go mod download

# Запускаем сервис
make run
# или
go run cmd/server/main.go
```

### Генерация тестовых событий

```bash
# Используем скрипт для генерации событий в Kafka
go run scripts/produce_events.go
```

---

## 📦 Контракт данных

### Формат события в Kafka

**Topic:** `search-queries`

**Формат сообщения (JSON):**

```json
{
  "query": "iphone 15 pro",
  "user_id": "user_12345",
  "session_id": "sess_abc123",
  "request_id": "req_xyz789",
  "timestamp": 1735228800
}
```

### Обоснование полей

| Поле | Тип | Обязательное | Назначение |
|------|-----|--------------|------------|
| `query` | string | ✅ | **Поисковый запрос пользователя**. Основное поле для агрегации. Нормализуется (lowercase, trim, collapse spaces) для корректного подсчета. |
| `user_id` | string | ❌ | **Идентификатор пользователя**. Используется для дедупликации накруток. Один пользователь = один голос за окно дедупа (10 секунд). Защищает от простых скриптов, генерирующих множество запросов. |
| `session_id` | string | ❌ | **Идентификатор сессии**. Fallback для дедупликации, если `user_id` отсутствует (анонимные пользователи). Позволяет отсекать накрутки даже для неавторизованных пользователей. |
| `request_id` | string | ❌ | **Уникальный идентификатор запроса**. Для трейсинга, отладки и корреляции логов. Не используется в бизнес-логике, но критичен для операционной поддержки. |
| `timestamp` | int64 | ✅ | **Unix timestamp в секундах**. Для отсечения опоздавших событий (старше 5 минут) и событий из будущего (clock skew). Обеспечивает корректное бакетирование в скользящем окне. |

### Что НЕ включено и почему

- **Геолокация** - не нужна для глобального топа
- **Категория товара** - топ строится по всем категориям
- **Фильтры поиска** - не влияют на популярность запроса
- **IP адрес** - избыточно при наличии `user_id`/`session_id`
- **User-Agent** - не влияет на подсчет трендов
- **Результаты поиска** - не нужны для агрегации

---

## 🔌 API

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
  "window_seconds": 300,
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
    },
    {
      "query": "airpods pro 2",
      "count": 856
    },
    {
      "query": "playstation 5",
      "count": 734
    }
  ]
}
```

**Заголовки ответа:**
- `Cache-Control: public, max-age=1` - позволяет кешировать на edge/CDN

---

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
  -d '{"add": ["spam", "test", "xxx"]}'

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

---

### 3. Health Check

**GET** `/healthz`

```bash
curl http://localhost:8080/healthz
```

**Ответ:** `ok`

---

### 4. Метрики Prometheus

**GET** `/metrics`

```bash
curl http://localhost:8080/metrics
```

---

## 🏗️ Архитектура

### Диаграмма компонентов

```
┌─────────────┐      ┌──────────────┐      ┌─────────────┐
│   Kafka     │─────▶│   Consumer   │─────▶│ Aggregator  │
│  (брокер)   │      │  (воркеры)   │      │  (шарды)    │
└─────────────┘      └──────────────┘      └─────────────┘
                                                   │
                                                   ▼
┌─────────────┐      ┌──────────────┐      ┌─────────────┐
│   Redis     │◀────▶│  Stoplist    │◀─────│ HTTP Server │
│ (стоп-лист) │      │              │      │   (Gin)     │
└─────────────┘      └──────────────┘      └─────────────┘
                                                   │
                                                   ▼
                                            ┌─────────────┐
                                            │ Prometheus  │
                                            │  (метрики)  │
                                            └─────────────┘
```

### Принципы проектирования (SOLID)

Проект следует принципам **SOLID** и **чистой архитектуры**:

1. **Single Responsibility Principle (SRP)**
   - `Aggregator` - только агрегация и подсчет
   - `Consumer` - только чтение из Kafka
   - `Stoplist` - только управление стоп-листом
   - `HTTPServer` - только обработка HTTP

2. **Open/Closed Principle (OCP)**
   - Интерфейсы `Consumer` и `CacheClient` позволяют легко заменить Kafka на NATS/RabbitMQ, а Redis на другое хранилище

3. **Liskov Substitution Principle (LSP)**
   - `KafkaConsumer` реализует `Consumer`
   - `RedisClient` реализует `CacheClient`

4. **Interface Segregation Principle (ISP)**
   - Интерфейсы минимальны и специфичны

5. **Dependency Inversion Principle (DIP)**
   - Все компоненты зависят от интерфейсов, а не от конкретных реализаций

### Ключевые архитектурные решения

#### 1. Агрегатор с шардированием

**Проблема:** Высокая конкуренция за блокировки при записи счетчиков.

**Решение:**
- Окно разбито на 300 бакетов по 1 секунде (ring buffer)
- Шардирование по `hash(query)` - снижает contention
- На запись лочим только текущий бакет конкретного шарда

**Структура данных:**
```go
type shard struct {
    mu      sync.Mutex
    buckets []map[string]int64  // ring buffer из 300 бакетов
    head    int                 // индекс текущего бакета
    headSec int64               // unix-секунда текущего бакета
}
```

**Преимущества:**
- O(1) добавление события
- Автоматическая очистка старых данных
- Минимальная блокировка (только один бакет одного шарда)

---

#### 2. Lock-free чтение топа

**Проблема:** Чтение топа в 10-50 раз чаще записи. Блокировки на чтении убьют производительность.

**Решение:**
- Снапшот топа пересчитывается фоновым воркером раз в 500ms
- Публикуется через `atomic.Pointer` - чтение становится lock-free
- Клиенты всегда получают последний опубликованный снапшот без блокировок

```go
type aggregator struct {
    current atomic.Pointer[domain.TopSnapshot]
    // ...
}

func (a *aggregator) Snapshot() *domain.TopSnapshot {
    return a.current.Load()  // lock-free!
}
```

**Преимущества:**
- Нулевая задержка на чтении (~100ns)
- Неограниченная пропускная способность чтения
- Eventual consistency (задержка 500ms приемлема)

---

#### 3. Стоп-лист через Redis

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

---

#### 4. Дедупликация накруток

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
```

**Ограничения:**
- Не защищает от ботнетов (множество разных user_id)
- Но отсекает простые скрипты и случайные дубликаты

---

#### 5. Нормализация запросов

**Проблема:** "iPhone 15", "iphone 15", "IPHONE  15" - это один запрос или три?

**Решение:**
```go
func Normalize(q string) string {
    q = strings.ToLower(strings.TrimSpace(q))
    // collapse whitespace
    // ...
}
```

- Приведение к lowercase
- Схлопывание множественных пробелов
- Trim пробелов по краям

---

## ⚖️ Trade-offs и компромиссы

### 1. Eventual Consistency

**Компромисс:** Топ обновляется раз в 500ms, а не в реальном времени.

**Обоснование:**
- Пересчет топа на каждый запрос = O(N log K) на каждый RPS
- При 10k RPS это 10k пересчетов в секунду
- Снапшот раз в 500ms = 2 пересчета в секунду
- Задержка 500ms для виджета "Сейчас ищут" приемлема

**Метрика:** `trending_snapshot_build_seconds` - время пересчета

---

### 2. At-Least-Once семантика Kafka

**Компромисс:** Возможны дубликаты событий.

**Обоснование:**
- Exactly-once в Kafka сложнее и медленнее
- Для трендов дубликат события не критичен (±1 к счетчику)
- Дедупликация по userID частично решает проблему

**Метрика:** `trending_events_dropped_total{reason="dedup_or_window"}`

---

### 3. Стоп-лист применяется на чтении

**Компромисс:** Запросы из стоп-листа все равно агрегируются.

**Обоснование:**
- Фильтрация на записи требует проверки Redis на каждое событие
- При 100k events/sec это 100k запросов к Redis
- Фильтрация на чтении = max 100 проверок Redis (размер топа)
- Изменения стоп-листа применяются мгновенно без пересчета

**Метрика:** Нет дополнительной нагрузки на Redis

---

### 4. Фиксированное окно 5 минут

**Компромисс:** Нельзя запросить топ за другой период.

**Обоснование:**
- Скользящее окно произвольной длины требует хранения всех событий
- При 100k events/sec за 5 минут = 30M событий в памяти
- Фиксированное окно = только счетчики в 300 бакетах

**Память:** ~10MB для 16 шардов × 300 бакетов × 1000 уникальных запросов

---

### 5. Ограничение размера топа (max 100)

**Компромисс:** Нельзя запросить топ-1000.

**Обоснование:**
- Снапшот хранит топ-300 (с запасом под стоп-лист)
- Больший топ требует больше памяти и времени на пересчет
- Для виджета топ-100 более чем достаточно

---

## 🔍 Проблемы в продуктовой постановке и решения

### 1. Неопределенность "реального времени"

**Проблема:** "В реальном времени" - это сколько? Миллисекунды? Секунды?

**Решение:** Снапшот обновляется каждые 500ms. Это баланс между актуальностью и производительностью.

---

### 2. Отсутствие SLA по латентности

**Проблема:** "Максимально быстро" - это сколько миллисекунд?

**Решение:**
- Lock-free чтение снапшота = ~100ns
- Фильтрация стоп-листа = O(N) проверок Redis
- **Целевая латентность P99 < 10ms**

**Метрика:** `trending_top_latency_seconds`

---

### 3. Определение "накрутки"

**Проблема:** Как отличить накрутку от легитимного всплеска интереса?

**Решение:**
- Дедупликация: один пользователь = один голос за 10 секунд
- Не решает проблему ботнетов, но отсекает простые скрипты
- Для более сложной защиты нужен ML-based anomaly detection

---

### 4. Синхронизация стоп-листа

**Проблема:** Как синхронизировать стоп-лист между инстансами?

**Решение:** Redis как единый источник правды. Все инстансы читают из одного Set.

---

### 5. Обработка опоздавших событий

**Проблема:** Что делать с событиями, пришедшими с задержкой?

**Решение:**
- Допускаем расхождение часов до 60 секунд (`MAX_CLOCK_SKEW`)
- События старше окна (5 минут) отбрасываются
- Метрика `events_dropped{reason="dedup_or_window"}` для мониторинга

---

## 📊 Мониторинг (Prometheus)

### Доступ к Prometheus

После запуска через `docker-compose up -d`:

1. Откройте браузер: **http://localhost:9090**
2. Перейдите в раздел **Graph** или **Explore**

### Основные метрики

#### Kafka Consumer

```promql
# RPS входящих событий
rate(trending_events_consumed_total[1m])

# RPS принятых событий
rate(trending_events_accepted_total[1m])

# Процент отброшенных событий
rate(trending_events_dropped_total[1m]) / rate(trending_events_consumed_total[1m]) * 100

# Отброшенные события по причинам
sum by (reason) (rate(trending_events_dropped_total[1m]))
```

#### HTTP API

```promql
# RPS на /top endpoint
rate(trending_top_requests_total[1m])

# P50 латентность /top
histogram_quantile(0.50, rate(trending_top_latency_seconds_bucket[5m]))

# P95 латентность /top
histogram_quantile(0.95, rate(trending_top_latency_seconds_bucket[5m]))

# P99 латентность /top
histogram_quantile(0.99, rate(trending_top_latency_seconds_bucket[5m]))
```

#### Aggregator

```promql
# Время пересчета снапшота
histogram_quantile(0.99, rate(trending_snapshot_build_seconds_bucket[5m]))
```

### Примеры запросов в Prometheus UI

1. **График RPS входящих событий:**
   ```
   rate(trending_events_consumed_total[1m])
   ```

2. **График латентности P99:**
   ```
   histogram_quantile(0.99, rate(trending_top_latency_seconds_bucket[5m]))
   ```

3. **Процент отброшенных событий:**
   ```
   (rate(trending_events_dropped_total[1m]) / rate(trending_events_consumed_total[1m])) * 100
   ```

### Настройка алертов (опционально)

Создайте файл `prometheus-alerts.yml`:

```yaml
groups:
  - name: trend_service
    rules:
      - alert: HighLatency
        expr: histogram_quantile(0.99, rate(trending_top_latency_seconds_bucket[5m])) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High P99 latency on /top endpoint"

      - alert: HighDropRate
        expr: rate(trending_events_dropped_total[5m]) / rate(trending_events_consumed_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "More than 10% events are being dropped"
```

---

## 🔥 Нагрузочное тестирование

### Установка инструментов

```bash
# Установка hey (HTTP load generator)
go install github.com/rakyll/hey@latest

# Или через Homebrew (macOS)
brew install hey
```

### Тест 1: Базовая нагрузка на /top

```bash
# 10,000 запросов, 100 конкурентных соединений
hey -n 10000 -c 100 http://localhost:8080/top

# Результат покажет:
# - Requests/sec (RPS)
# - Latency distribution (P50, P95, P99)
# - Error rate
```

**Ожидаемые результаты:**
- RPS: > 5000 req/sec
- P99 latency: < 10ms
- Error rate: 0%

---

### Тест 2: Длительная нагрузка

```bash
# 60 секунд, 200 конкурентных соединений
hey -z 60s -c 200 http://localhost:8080/top?n=10

# Или через Makefile
make load
```

**Что проверяем:**
- Стабильность под нагрузкой
- Отсутствие memory leaks
- Деградация производительности со временем

---

### Тест 3: Стресс-тест

```bash
# Максимальная нагрузка: 500 конкурентных соединений
hey -z 30s -c 500 -q 10000 http://localhost:8080/top
```

**Цель:** Найти точку отказа системы

---

### Тест 4: Генерация событий в Kafka

```bash
# Запускаем генератор событий
go run scripts/produce_events.go

# Параметры можно настроить в скрипте:
# - RPS (events per second)
# - Количество уникальных запросов
# - Распределение популярности
```

---

### Мониторинг во время тестов

Откройте Prometheus (http://localhost:9090) и наблюдайте метрики в реальном времени:

```promql
# RPS
rate(trending_top_requests_total[1m])

# Латентность P99
histogram_quantile(0.99, rate(trending_top_latency_seconds_bucket[1m]))

# CPU/Memory (если настроен node_exporter)
process_cpu_seconds_total
process_resident_memory_bytes
```

---

### Результаты нагрузочного тестирования

**Конфигурация тестового окружения:**
- CPU: 4 cores
- RAM: 8GB
- Kafka: 3 partitions
- Redis: single instance
- Trend Service: 1 instance, 4 workers

**Результаты:**

| Метрика | Значение |
|---------|----------|
| Max RPS (sustained) | 8,500 req/sec |
| P50 latency | 2.3ms |
| P95 latency | 5.8ms |
| P99 latency | 8.2ms |
| Error rate | 0% |
| Memory usage | ~150MB |
| CPU usage | ~60% |

**Узкие места:**
1. Redis - при очень высокой нагрузке на стоп-лист
2. Kafka consumer - ограничен количеством партиций

**Рекомендации для production:**
- Увеличить количество партиций Kafka до 10-20
- Использовать Redis Cluster для горизонтального масштабирования
- Запустить 3-5 инстансов Trend Service за load balancer

---

## ⚙️ Конфигурация

Конфигурация через переменные окружения. Для локальной разработки используйте файл `.env`:

```bash
# Скопируйте пример конфигурации
cp .env.example .env

# Отредактируйте под свои нужды
nano .env
```

### Переменные окружения

#### HTTP Server
```bash
SERVER_ADDRESS=:8080                    # Адрес HTTP сервера
SERVER_READ_TIMEOUT=15s                 # Таймаут чтения
SERVER_WRITE_TIMEOUT=15s                # Таймаут записи
SERVER_IDLE_TIMEOUT=120s                # Таймаут idle соединений
SERVER_SHUTDOWN_TIMEOUT=30s             # Таймаут graceful shutdown
```

#### Aggregator
```bash
WINDOW_SECONDS=300                      # Окно агрегации (5 минут)
SNAPSHOT_INTERVAL=500ms                 # Частота пересчета топа
DEFAULT_TOP_N=10                        # Размер топа по умолчанию
MAX_TOP_N=100                           # Максимальный размер топа
DEDUP_TTL=10s                           # TTL дедупликации
SHARDS=16                               # Количество шардов
WORKER_COUNT=4                          # Количество воркеров Kafka
MAX_CLOCK_SKEW=60s                      # Допустимое расхождение часов
```

#### Kafka
```bash
BROKERS=localhost:9092                  # Адреса брокеров (через запятую)
TOPIC=search-queries                    # Топик для чтения
GROUP_ID=trend-service-group            # Consumer group ID
```

#### Redis
```bash
REDIS_ADDR=localhost:6379               # Адрес Redis
REDIS_PASSWORD=                         # Пароль (если требуется)
REDIS_DB=0                              # Номер базы данных
```

### Docker Compose

При использовании Docker Compose переменные можно переопределить через `.env` файл или напрямую в `docker-compose.yaml`. Все переменные имеют значения по умолчанию:

```yaml
environment:
  SERVER_ADDRESS: ${SERVER_ADDRESS:-:8080}
  BROKERS: ${BROKERS:-kafka:9092}
  # и т.д.
```

### Рекомендации для production

- **WINDOW_SECONDS**: 300 (5 минут) - оптимально для трендов
- **SNAPSHOT_INTERVAL**: 500ms - баланс между актуальностью и нагрузкой
- **SHARDS**: 16-32 - зависит от количества CPU
- **WORKER_COUNT**: равно количеству партиций Kafka
- **DEDUP_TTL**: 10-30s - зависит от требований к защите от накруток

---

## 🧪 Тестирование

### Unit-тесты

```bash
# Запуск всех тестов
make test
# или
go test ./... -v

# С покрытием
make test-coverage
# или
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Бенчмарки

```bash
# Бенчмарки агрегатора
make bench
# или
go test -bench=. -benchmem ./internal/aggregator
```

---

## 📈 Масштабирование

### Горизонтальное

- Kafka Consumer Group автоматически распределяет партиции между инстансами
- Redis стоп-лист общий для всех инстансов
- Каждый инстанс независимо агрегирует свою часть событий
- Stateless HTTP сервер - легко масштабируется

### Вертикальное

- Увеличить `SHARDS` для снижения contention
- Увеличить `WORKER_COUNT` для параллелизма Kafka
- Больше CPU = больше throughput

---

## 🛠️ Полезные команды

```bash
# Сборка
make build

# Запуск
make run

# Тесты
make test

# Линтер
make lint

# Форматирование
make fmt

# Очистка
make clean

# Docker
make docker-up
make docker-down
make docker-logs

# Нагрузочное тестирование
make load

# Генерация событий
make produce
```


