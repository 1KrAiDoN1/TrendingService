# 🚀 Руководство по развертыванию

## Локальная разработка

### Вариант 1: Docker Compose (рекомендуется)

```bash
# Запуск с дефолтными настройками
docker-compose up -d

# Запуск с кастомными настройками
# Создайте .env файл
cp .env.example .env
# Отредактируйте параметры
nano .env
# Запустите
docker-compose up -d
```

### Вариант 2: Локальный запуск

```bash
# Запустите инфраструктуру
docker-compose up -d kafka redis prometheus

# Создайте .env файл для локальной разработки
cp .env.example .env

# Запустите сервис
go run cmd/server/main.go
```

---

## Production развертывание

### Kubernetes

#### 1. ConfigMap для конфигурации

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: trend-service-config
data:
  SERVER_ADDRESS: ":8080"
  WINDOW_SECONDS: "300"
  SNAPSHOT_INTERVAL: "500ms"
  DEFAULT_TOP_N: "10"
  MAX_TOP_N: "100"
  DEDUP_TTL: "10s"
  SHARDS: "32"
  WORKER_COUNT: "8"
  MAX_CLOCK_SKEW: "60s"
  BROKERS: "kafka-0.kafka-headless:9092,kafka-1.kafka-headless:9092,kafka-2.kafka-headless:9092"
  TOPIC: "search-queries"
  GROUP_ID: "trend-service-group"
  REDIS_ADDR: "redis-master:6379"
  REDIS_DB: "0"
```

#### 2. Secret для чувствительных данных

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: trend-service-secret
type: Opaque
stringData:
  REDIS_PASSWORD: "your-redis-password"
```

#### 3. Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: trend-service
  labels:
    app: trend-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: trend-service
  template:
    metadata:
      labels:
        app: trend-service
    spec:
      containers:
      - name: trend-service
        image: your-registry/trend-service:latest
        ports:
        - containerPort: 8080
          name: http
        envFrom:
        - configMapRef:
            name: trend-service-config
        - secretRef:
            name: trend-service-secret
        resources:
          requests:
            memory: "256Mi"
            cpu: "500m"
          limits:
            memory: "512Mi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

#### 4. Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: trend-service
  labels:
    app: trend-service
spec:
  type: ClusterIP
  ports:
  - port: 8080
    targetPort: 8080
    protocol: TCP
    name: http
  selector:
    app: trend-service
```

#### 5. HorizontalPodAutoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: trend-service-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: trend-service
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

#### 6. ServiceMonitor (для Prometheus Operator)

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: trend-service
  labels:
    app: trend-service
spec:
  selector:
    matchLabels:
      app: trend-service
  endpoints:
  - port: http
    path: /metrics
    interval: 15s
```

---

## Docker Swarm

### 1. Stack файл

```yaml
version: '3.8'

services:
  trend-service:
    image: your-registry/trend-service:latest
    deploy:
      replicas: 3
      update_config:
        parallelism: 1
        delay: 10s
      restart_policy:
        condition: on-failure
      resources:
        limits:
          cpus: '1'
          memory: 512M
        reservations:
          cpus: '0.5'
          memory: 256M
    environment:
      SERVER_ADDRESS: ":8080"
      BROKERS: "kafka:9092"
      REDIS_ADDR: "redis:6379"
      SHARDS: "32"
      WORKER_COUNT: "8"
    ports:
      - "8080:8080"
    networks:
      - trend-network

networks:
  trend-network:
    driver: overlay
```

### 2. Развертывание

```bash
docker stack deploy -c docker-stack.yml trend
```

---

## Переменные окружения

### Обязательные

Нет обязательных переменных - все имеют значения по умолчанию.

### Рекомендуемые для production

```bash
# Увеличьте количество шардов для лучшей производительности
SHARDS=32

# Количество воркеров = количество партиций Kafka
WORKER_COUNT=8

# Адреса Kafka брокеров (через запятую)
BROKERS=kafka-0:9092,kafka-1:9092,kafka-2:9092

# Redis с паролем
REDIS_ADDR=redis-master:6379
REDIS_PASSWORD=your-secure-password
```

---

## Мониторинг

### Prometheus

Добавьте scrape config:

```yaml
scrape_configs:
  - job_name: 'trend-service'
    static_configs:
      - targets: ['trend-service:8080']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

### Grafana Dashboard

Импортируйте дашборд из `grafana-dashboard.json` (если создан) или создайте свой с метриками:

- `rate(trending_top_requests_total[1m])`
- `histogram_quantile(0.99, rate(trending_top_latency_seconds_bucket[5m]))`
- `rate(trending_events_consumed_total[1m])`
- `rate(trending_events_dropped_total[1m]) / rate(trending_events_consumed_total[1m])`

---

## Масштабирование

### Горизонтальное

1. **Увеличьте количество партиций Kafka** (рекомендуется 10-20)
2. **Запустите несколько инстансов** сервиса
3. **Kafka Consumer Group** автоматически распределит партиции

```bash
# Kubernetes
kubectl scale deployment trend-service --replicas=5

# Docker Swarm
docker service scale trend_trend-service=5
```

### Вертикальное

Увеличьте ресурсы:

```yaml
resources:
  limits:
    cpus: '2'
    memory: 1Gi
  reservations:
    cpus: '1'
    memory: 512Mi
```

И параметры:

```bash
SHARDS=64
WORKER_COUNT=16
```

---

## Безопасность

### 1. TLS для Kafka

```bash
# Добавьте сертификаты в ConfigMap/Secret
# Обновите код для использования TLS
```

### 2. Redis с аутентификацией

```bash
REDIS_PASSWORD=your-secure-password
```

### 3. Network Policies (Kubernetes)

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: trend-service-netpol
spec:
  podSelector:
    matchLabels:
      app: trend-service
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: kafka
    ports:
    - protocol: TCP
      port: 9092
  - to:
    - podSelector:
        matchLabels:
          app: redis
    ports:
    - protocol: TCP
      port: 6379
```

---

## Backup и Recovery

### Redis (стоп-лист)

```bash
# Backup
redis-cli --rdb /backup/dump.rdb

# Restore
cp /backup/dump.rdb /var/lib/redis/dump.rdb
redis-cli shutdown
# Redis автоматически загрузит dump.rdb при старте
```

### Kafka (события)

Kafka хранит события с retention policy. Для долгосрочного хранения используйте:

- Kafka Connect + S3/GCS
- Kafka Streams для архивации

---

## Troubleshooting

### Высокая латентность

1. Проверьте метрики Redis
2. Увеличьте количество шардов
3. Добавьте больше инстансов

### Высокий процент отброшенных событий

1. Проверьте синхронизацию часов
2. Увеличьте `MAX_CLOCK_SKEW`
3. Проверьте задержки в Kafka

### Out of Memory

1. Уменьшите `WINDOW_SECONDS`
2. Уменьшите `MAX_TOP_N`
3. Увеличьте memory limits

---

## CI/CD

### GitHub Actions пример

```yaml
name: Build and Deploy

on:
  push:
    branches: [ main ]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v2
    
    - name: Build Docker image
      run: docker build -t your-registry/trend-service:${{ github.sha }} .
    
    - name: Push to registry
      run: docker push your-registry/trend-service:${{ github.sha }}
    
    - name: Deploy to Kubernetes
      run: |
        kubectl set image deployment/trend-service \
          trend-service=your-registry/trend-service:${{ github.sha }}
```

---

## Checklist перед production

- [ ] Настроены переменные окружения
- [ ] Увеличено количество партиций Kafka (10-20)
- [ ] Настроен Redis Cluster или Sentinel
- [ ] Настроен мониторинг (Prometheus + Grafana)
- [ ] Настроены алерты
- [ ] Проведено нагрузочное тестирование
- [ ] Настроен backup Redis
- [ ] Настроены resource limits
- [ ] Настроен HPA (Kubernetes)
- [ ] Настроены health checks
- [ ] Настроены network policies
- [ ] Документация обновлена

---

## Поддержка

Для вопросов создавайте Issue в репозитории.
