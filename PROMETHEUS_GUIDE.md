# Руководство по работе с Prometheus

## Доступ к Prometheus

После запуска сервисов через `docker-compose up -d`, Prometheus будет доступен по адресу:

**http://localhost:9090**

## Интерфейс Prometheus

### 1. Graph (Графики)

Основной интерфейс для выполнения запросов и построения графиков.

**Как использовать:**
1. Откройте http://localhost:9090/graph
2. В поле "Expression" введите PromQL запрос
3. Нажмите "Execute"
4. Переключитесь на вкладку "Graph" для визуализации

### 2. Targets (Цели)

Просмотр статуса всех целей мониторинга.

**Как проверить:**
1. Откройте http://localhost:9090/targets
2. Убедитесь, что `trend-service` в статусе **UP**
3. Если статус **DOWN**, проверьте логи: `docker-compose logs trending`

### 3. Alerts (Алерты)

Просмотр активных алертов (если настроены).

## Основные метрики Trend Service

### Kafka Consumer

#### 1. RPS входящих событий

```promql
rate(trending_events_consumed_total[1m])
```

**Что показывает:** Количество событий, прочитанных из Kafka в секунду.

**Нормальное значение:** Зависит от нагрузки, обычно 100-10000 events/sec.

---

#### 2. RPS принятых событий

```promql
rate(trending_events_accepted_total[1m])
```

**Что показывает:** Количество событий, принятых агрегатором (прошедших дедупликацию и фильтры).

---

#### 3. Процент отброшенных событий

```promql
(rate(trending_events_dropped_total[1m]) / rate(trending_events_consumed_total[1m])) * 100
```

**Что показывает:** Процент событий, отброшенных из-за дедупликации или выхода за окно.

**Нормальное значение:** 5-15%

**Тревога:** > 30% (возможно, проблемы с часами или слишком много дубликатов)

---

#### 4. Отброшенные события по причинам

```promql
sum by (reason) (rate(trending_events_dropped_total[1m]))
```

**Что показывает:** Распределение причин отбрасывания событий:
- `bad_json` - невалидный JSON
- `empty_query` - пустой запрос
- `dedup_or_window` - дедупликация или выход за окно

---

### HTTP API

#### 5. RPS на /top endpoint

```promql
rate(trending_top_requests_total[1m])
```

**Что показывает:** Количество запросов к `/top` в секунду.

**Нормальное значение:** Зависит от нагрузки, может быть 100-50000 req/sec.

---

#### 6. Латентность P50 (медиана)

```promql
histogram_quantile(0.50, rate(trending_top_latency_seconds_bucket[5m]))
```

**Что показывает:** 50% запросов обрабатываются быстрее этого значения.

**Нормальное значение:** < 5ms

---

#### 7. Латентность P95

```promql
histogram_quantile(0.95, rate(trending_top_latency_seconds_bucket[5m]))
```

**Что показывает:** 95% запросов обрабатываются быстрее этого значения.

**Нормальное значение:** < 10ms

---

#### 8. Латентность P99

```promql
histogram_quantile(0.99, rate(trending_top_latency_seconds_bucket[5m]))
```

**Что показывает:** 99% запросов обрабатываются быстрее этого значения.

**Нормальное значение:** < 15ms

**Тревога:** > 50ms

---

### Aggregator

#### 9. Время пересчета снапшота

```promql
histogram_quantile(0.99, rate(trending_snapshot_build_seconds_bucket[5m]))
```

**Что показывает:** Время, затрачиваемое на пересчет топа.

**Нормальное значение:** < 100ms

**Тревога:** > 500ms (может привести к задержкам)

---

## Примеры дашбордов

### Дашборд 1: Обзор системы

```promql
# RPS входящих событий
rate(trending_events_consumed_total[1m])

# RPS запросов к API
rate(trending_top_requests_total[1m])

# P99 латентность
histogram_quantile(0.99, rate(trending_top_latency_seconds_bucket[5m]))

# Процент отброшенных событий
(rate(trending_events_dropped_total[1m]) / rate(trending_events_consumed_total[1m])) * 100
```

### Дашборд 2: Производительность

```promql
# Латентность (все перцентили)
histogram_quantile(0.50, rate(trending_top_latency_seconds_bucket[5m]))
histogram_quantile(0.95, rate(trending_top_latency_seconds_bucket[5m]))
histogram_quantile(0.99, rate(trending_top_latency_seconds_bucket[5m]))

# Время пересчета снапшота
histogram_quantile(0.99, rate(trending_snapshot_build_seconds_bucket[5m]))
```

### Дашборд 3: Качество данных

```promql
# Отброшенные события по причинам
sum by (reason) (rate(trending_events_dropped_total[1m]))

# Соотношение принятых к отброшенным
rate(trending_events_accepted_total[1m]) / rate(trending_events_consumed_total[1m])
```

---

## Создание графиков

### Пример: График RPS

1. Откройте http://localhost:9090/graph
2. Введите запрос:
   ```promql
   rate(trending_top_requests_total[1m])
   ```
3. Нажмите "Execute"
4. Переключитесь на вкладку "Graph"
5. Настройте временной диапазон (например, "Last 1 hour")

### Пример: Сравнение латентности

1. Введите несколько запросов:
   ```promql
   histogram_quantile(0.50, rate(trending_top_latency_seconds_bucket[5m]))
   histogram_quantile(0.95, rate(trending_top_latency_seconds_bucket[5m]))
   histogram_quantile(0.99, rate(trending_top_latency_seconds_bucket[5m]))
   ```
2. Каждый запрос добавляется кнопкой "+ Add Query"
3. Все графики отобразятся на одном графике

---

## Экспорт данных

### Экспорт в JSON

```bash
# Получить метрики за последний час
curl 'http://localhost:9090/api/v1/query_range?query=rate(trending_top_requests_total[1m])&start=2024-01-01T00:00:00Z&end=2024-01-01T01:00:00Z&step=15s' | jq
```

### Экспорт в CSV

Используйте кнопку "Download CSV" в интерфейсе Prometheus после выполнения запроса.

---

## Интеграция с Grafana (опционально)

Для более красивых дашбордов можно использовать Grafana:

1. Добавьте в `docker-compose.yaml`:
   ```yaml
   grafana:
     image: grafana/grafana:latest
     ports:
       - "3000:3000"
     environment:
       - GF_SECURITY_ADMIN_PASSWORD=admin
     volumes:
       - grafana-data:/var/lib/grafana
   ```

2. Откройте http://localhost:3000
3. Добавьте Prometheus как Data Source:
   - URL: http://prometheus:9090
4. Импортируйте готовый дашборд или создайте свой

---

## Troubleshooting

### Метрики не отображаются

**Проблема:** Prometheus не видит метрики.

**Решение:**
1. Проверьте, что сервис запущен: `docker-compose ps`
2. Проверьте targets: http://localhost:9090/targets
3. Проверьте логи: `docker-compose logs trending`

### Высокая латентность

**Проблема:** P99 > 50ms

**Возможные причины:**
1. Высокая нагрузка на Redis (стоп-лист)
2. Большой размер топа (> 100)
3. Недостаточно ресурсов (CPU/Memory)

**Решение:**
1. Увеличить количество инстансов
2. Оптимизировать стоп-лист
3. Добавить кеширование на уровне CDN

### Высокий процент отброшенных событий

**Проблема:** > 30% событий отбрасывается

**Возможные причины:**
1. Проблемы с синхронизацией часов
2. Слишком много дубликатов от одного пользователя
3. События приходят с большой задержкой

**Решение:**
1. Проверить `MAX_CLOCK_SKEW`
2. Увеличить `DEDUP_TTL`
3. Проверить задержки в Kafka

---

## Полезные ссылки

- [Prometheus Query Language (PromQL)](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Prometheus Functions](https://prometheus.io/docs/prometheus/latest/querying/functions/)
- [Histogram and Summary](https://prometheus.io/docs/practices/histograms/)
