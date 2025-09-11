# NetCore-Go 部署指南

本指南详细介绍了如何在不同环境和云平台上部署 NetCore-Go 应用程序。

## 目录

- [部署准备](#部署准备)
- [本地部署](#本地部署)
- [Docker 部署](#docker-部署)
- [Kubernetes 部署](#kubernetes-部署)
- [云平台部署](#云平台部署)
  - [AWS 部署](#aws-部署)
  - [Google Cloud 部署](#google-cloud-部署)
  - [Azure 部署](#azure-部署)
  - [阿里云部署](#阿里云部署)
- [CI/CD 集成](#cicd-集成)
- [监控和日志](#监控和日志)
- [安全配置](#安全配置)
- [性能优化](#性能优化)
- [故障恢复](#故障恢复)

## 部署准备

### 环境要求

- **Go 版本**：>= 1.21
- **操作系统**：Linux, macOS, Windows
- **内存**：最小 512MB，推荐 2GB+
- **CPU**：最小 1 核，推荐 2 核+
- **存储**：最小 1GB 可用空间

### 依赖服务

根据应用功能，可能需要以下服务：

- **服务发现**：Consul, etcd, 或 Kubernetes
- **数据库**：PostgreSQL, MySQL, MongoDB
- **缓存**：Redis, Memcached
- **消息队列**：RabbitMQ, Apache Kafka
- **负载均衡器**：Nginx, HAProxy, 云负载均衡器

### 配置检查清单

```bash
# 检查 Go 环境
go version

# 验证应用构建
go build -o myapp ./cmd/server

# 测试配置文件
./myapp --config-check

# 运行测试
go test ./...
```

## 本地部署

### 直接运行

```bash
# 1. 克隆项目
git clone https://github.com/your-org/your-netcore-app.git
cd your-netcore-app

# 2. 安装依赖
go mod download

# 3. 构建应用
go build -o myapp ./cmd/server

# 4. 配置环境变量
export DATABASE_URL="postgres://user:pass@localhost/dbname"
export REDIS_URL="redis://localhost:6379"
export LOG_LEVEL="info"

# 5. 运行应用
./myapp
```

### 使用 systemd 服务

创建服务文件 `/etc/systemd/system/myapp.service`：

```ini
[Unit]
Description=NetCore-Go Application
After=network.target

[Service]
Type=simple
User=appuser
Group=appuser
WorkingDirectory=/opt/myapp
ExecStart=/opt/myapp/myapp
Restart=always
RestartSec=5
EnvironmentFile=/opt/myapp/.env

# 安全设置
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/myapp/logs

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
# 重新加载 systemd
sudo systemctl daemon-reload

# 启动服务
sudo systemctl start myapp

# 设置开机自启
sudo systemctl enable myapp

# 查看状态
sudo systemctl status myapp
```

## Docker 部署

### 单阶段构建

```dockerfile
FROM golang:1.21-alpine AS builder

# 安装必要工具
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /app

# 复制 go mod 文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o myapp ./cmd/server

# 运行阶段
FROM alpine:latest

# 安装 ca-certificates
RUN apk --no-cache add ca-certificates

# 创建非 root 用户
RUN adduser -D -s /bin/sh appuser

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/myapp .
COPY --from=builder /app/configs ./configs

# 设置权限
RUN chown -R appuser:appuser /app
USER appuser

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 启动应用
CMD ["./myapp"]
```

### 多阶段优化构建

```dockerfile
# 构建阶段
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata make

WORKDIR /app

# 缓存依赖
COPY go.mod go.sum ./
RUN go mod download

# 构建
COPY . .
RUN make build-linux

# 最终镜像
FROM scratch

# 复制 CA 证书
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# 复制时区数据
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# 复制应用
COPY --from=builder /app/bin/myapp /myapp

# 暴露端口
EXPOSE 8080

# 启动应用
ENTRYPOINT ["/myapp"]
```

### Docker Compose 部署

```yaml
version: '3.8'

services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://user:pass@db:5432/myapp
      - REDIS_URL=redis://redis:6379
      - LOG_LEVEL=info
    depends_on:
      - db
      - redis
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  db:
    image: postgres:15-alpine
    environment:
      - POSTGRES_DB=myapp
      - POSTGRES_USER=user
      - POSTGRES_PASSWORD=pass
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data
    restart: unless-stopped

  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf
      - ./ssl:/etc/nginx/ssl
    depends_on:
      - app
    restart: unless-stopped

volumes:
  postgres_data:
  redis_data:
```

## Kubernetes 部署

### 基础部署配置

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
  labels:
    app: myapp
spec:
  replicas: 3
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
      - name: myapp
        image: myapp:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: myapp-secrets
              key: database-url
        - name: REDIS_URL
          valueFrom:
            configMapKeyRef:
              name: myapp-config
              key: redis-url
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        volumeMounts:
        - name: config
          mountPath: /app/configs
          readOnly: true
      volumes:
      - name: config
        configMap:
          name: myapp-config
---
apiVersion: v1
kind: Service
metadata:
  name: myapp-service
spec:
  selector:
    app: myapp
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8080
  type: ClusterIP
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: myapp-config
data:
  redis-url: "redis://redis-service:6379"
  log-level: "info"
  config.yaml: |
    server:
      host: "0.0.0.0"
      port: 8080
      timeout: 30s
    discovery:
      type: "kubernetes"
      namespace: "default"
---
apiVersion: v1
kind: Secret
metadata:
  name: myapp-secrets
type: Opaque
data:
  database-url: cG9zdGdyZXM6Ly91c2VyOnBhc3NAbG9jYWxob3N0L2RibmFtZQ== # base64 encoded
```

### Ingress 配置

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: myapp-ingress
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/rate-limit: "100"
    nginx.ingress.kubernetes.io/rate-limit-window: "1m"
spec:
  tls:
  - hosts:
    - api.example.com
    secretName: myapp-tls
  rules:
  - host: api.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: myapp-service
            port:
              number: 80
```

### HPA 自动扩缩容

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: myapp-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: myapp
  minReplicas: 2
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

## 云平台部署

### AWS 部署

#### ECS 部署

```json
{
  "family": "myapp",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "256",
  "memory": "512",
  "executionRoleArn": "arn:aws:iam::account:role/ecsTaskExecutionRole",
  "taskRoleArn": "arn:aws:iam::account:role/ecsTaskRole",
  "containerDefinitions": [
    {
      "name": "myapp",
      "image": "your-account.dkr.ecr.region.amazonaws.com/myapp:latest",
      "portMappings": [
        {
          "containerPort": 8080,
          "protocol": "tcp"
        }
      ],
      "environment": [
        {
          "name": "LOG_LEVEL",
          "value": "info"
        }
      ],
      "secrets": [
        {
          "name": "DATABASE_URL",
          "valueFrom": "arn:aws:secretsmanager:region:account:secret:myapp/database-url"
        }
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/myapp",
          "awslogs-region": "us-west-2",
          "awslogs-stream-prefix": "ecs"
        }
      },
      "healthCheck": {
        "command": ["CMD-SHELL", "curl -f http://localhost:8080/health || exit 1"],
        "interval": 30,
        "timeout": 5,
        "retries": 3,
        "startPeriod": 60
      }
    }
  ]
}
```

#### EKS 部署

```bash
# 创建 EKS 集群
eksctl create cluster --name myapp-cluster --region us-west-2 --nodes 3

# 配置 kubectl
aws eks update-kubeconfig --region us-west-2 --name myapp-cluster

# 部署应用
kubectl apply -f k8s/

# 安装 AWS Load Balancer Controller
kubectl apply -k "github.com/aws/eks-charts/stable/aws-load-balancer-controller//crds?ref=master"
```

#### Lambda 部署

```go
// main.go for Lambda
package main

import (
    "context"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/awslabs/aws-lambda-go-api-proxy/gin"
    "github.com/gin-gonic/gin"
)

var ginLambda *ginadapter.GinLambda

func init() {
    r := gin.Default()
    // 设置路由
    setupRoutes(r)
    ginLambda = ginadapter.New(r)
}

func Handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    return ginLambda.ProxyWithContext(ctx, req)
}

func main() {
    lambda.Start(Handler)
}
```

### Google Cloud 部署

#### Cloud Run 部署

```yaml
# service.yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: myapp
  annotations:
    run.googleapis.com/ingress: all
spec:
  template:
    metadata:
      annotations:
        autoscaling.knative.dev/maxScale: "10"
        run.googleapis.com/cpu-throttling: "false"
        run.googleapis.com/memory: "512Mi"
        run.googleapis.com/cpu: "1000m"
    spec:
      containerConcurrency: 100
      containers:
      - image: gcr.io/project-id/myapp:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: myapp-secrets
              key: database-url
        resources:
          limits:
            memory: "512Mi"
            cpu: "1000m"
```

```bash
# 部署到 Cloud Run
gcloud run deploy myapp \
  --image gcr.io/project-id/myapp:latest \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated
```

#### GKE 部署

```bash
# 创建 GKE 集群
gcloud container clusters create myapp-cluster \
  --zone us-central1-a \
  --num-nodes 3 \
  --enable-autoscaling \
  --min-nodes 1 \
  --max-nodes 10

# 获取凭据
gcloud container clusters get-credentials myapp-cluster --zone us-central1-a

# 部署应用
kubectl apply -f k8s/
```

### Azure 部署

#### Container Instances 部署

```bash
# 创建资源组
az group create --name myapp-rg --location eastus

# 部署容器实例
az container create \
  --resource-group myapp-rg \
  --name myapp \
  --image myregistry.azurecr.io/myapp:latest \
  --cpu 1 \
  --memory 1 \
  --ports 8080 \
  --environment-variables LOG_LEVEL=info \
  --secure-environment-variables DATABASE_URL=$DATABASE_URL
```

#### AKS 部署

```bash
# 创建 AKS 集群
az aks create \
  --resource-group myapp-rg \
  --name myapp-cluster \
  --node-count 3 \
  --enable-addons monitoring \
  --generate-ssh-keys

# 获取凭据
az aks get-credentials --resource-group myapp-rg --name myapp-cluster

# 部署应用
kubectl apply -f k8s/
```

### 阿里云部署

#### ECS 部署

```bash
# 使用阿里云 CLI 创建 ECS 实例
aliyun ecs CreateInstance \
  --ImageId ubuntu_20_04_x64_20G_alibase_20210420.vhd \
  --InstanceType ecs.t5-lc1m1.small \
  --SecurityGroupId sg-xxxxxxxxx \
  --VSwitchId vsw-xxxxxxxxx
```

#### ACK 部署

```bash
# 创建 ACK 集群（通过控制台或 Terraform）
# 配置 kubectl
kubectl config use-context ack-cluster

# 部署应用
kubectl apply -f k8s/
```

## CI/CD 集成

### GitHub Actions

```yaml
# .github/workflows/deploy.yml
name: Deploy to Production

on:
  push:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - uses: actions/setup-go@v3
      with:
        go-version: 1.21
    - run: go test ./...
    - run: go vet ./...

  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - name: Build Docker image
      run: |
        docker build -t myapp:${{ github.sha }} .
        docker tag myapp:${{ github.sha }} myapp:latest
    
    - name: Push to registry
      run: |
        echo ${{ secrets.DOCKER_PASSWORD }} | docker login -u ${{ secrets.DOCKER_USERNAME }} --password-stdin
        docker push myapp:${{ github.sha }}
        docker push myapp:latest

  deploy:
    needs: build
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - name: Deploy to Kubernetes
      run: |
        echo ${{ secrets.KUBECONFIG }} | base64 -d > kubeconfig
        export KUBECONFIG=kubeconfig
        sed -i 's/IMAGE_TAG/${{ github.sha }}/g' k8s/deployment.yaml
        kubectl apply -f k8s/
```

### GitLab CI/CD

```yaml
# .gitlab-ci.yml
stages:
  - test
  - build
  - deploy

variables:
  DOCKER_DRIVER: overlay2
  DOCKER_TLS_CERTDIR: "/certs"

test:
  stage: test
  image: golang:1.21
  script:
    - go test ./...
    - go vet ./...

build:
  stage: build
  image: docker:latest
  services:
    - docker:dind
  script:
    - docker build -t $CI_REGISTRY_IMAGE:$CI_COMMIT_SHA .
    - docker login -u $CI_REGISTRY_USER -p $CI_REGISTRY_PASSWORD $CI_REGISTRY
    - docker push $CI_REGISTRY_IMAGE:$CI_COMMIT_SHA

deploy:
  stage: deploy
  image: bitnami/kubectl:latest
  script:
    - kubectl config use-context $KUBE_CONTEXT
    - sed -i "s/IMAGE_TAG/$CI_COMMIT_SHA/g" k8s/deployment.yaml
    - kubectl apply -f k8s/
  only:
    - main
```

### Jenkins Pipeline

```groovy
pipeline {
    agent any
    
    environment {
        DOCKER_REGISTRY = 'your-registry.com'
        IMAGE_NAME = 'myapp'
        KUBECONFIG = credentials('kubeconfig')
    }
    
    stages {
        stage('Test') {
            steps {
                sh 'go test ./...'
                sh 'go vet ./...'
            }
        }
        
        stage('Build') {
            steps {
                script {
                    def image = docker.build("${DOCKER_REGISTRY}/${IMAGE_NAME}:${BUILD_NUMBER}")
                    docker.withRegistry("https://${DOCKER_REGISTRY}", 'docker-registry-credentials') {
                        image.push()
                        image.push('latest')
                    }
                }
            }
        }
        
        stage('Deploy') {
            steps {
                sh """
                    sed -i 's/IMAGE_TAG/${BUILD_NUMBER}/g' k8s/deployment.yaml
                    kubectl apply -f k8s/
                """
            }
        }
    }
    
    post {
        always {
            cleanWs()
        }
    }
}
```

## 监控和日志

### Prometheus 监控

```yaml
# prometheus-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
    scrape_configs:
    - job_name: 'myapp'
      static_configs:
      - targets: ['myapp-service:80']
      metrics_path: /metrics
      scrape_interval: 5s
```

### Grafana 仪表板

```json
{
  "dashboard": {
    "title": "NetCore-Go Application",
    "panels": [
      {
        "title": "Request Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(http_requests_total[5m])",
            "legendFormat": "{{method}} {{path}}"
          }
        ]
      },
      {
        "title": "Response Time",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))",
            "legendFormat": "95th percentile"
          }
        ]
      }
    ]
  }
}
```

### ELK Stack 日志

```yaml
# filebeat.yml
filebeat.inputs:
- type: log
  enabled: true
  paths:
    - /var/log/myapp/*.log
  fields:
    service: myapp
    environment: production
  fields_under_root: true

output.elasticsearch:
  hosts: ["elasticsearch:9200"]
  index: "myapp-%{+yyyy.MM.dd}"

logging.level: info
```

## 安全配置

### TLS/SSL 配置

```go
// TLS 服务器配置
tlsConfig := &tls.Config{
    MinVersion: tls.VersionTLS12,
    CipherSuites: []uint16{
        tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
        tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
        tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
        tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
    },
    PreferServerCipherSuites: true,
    CurvePreferences: []tls.CurveID{
        tls.CurveP256,
        tls.X25519,
    },
}

server := &http.Server{
    Addr:      ":8443",
    TLSConfig: tlsConfig,
    Handler:   router,
}

log.Fatal(server.ListenAndServeTLS("cert.pem", "key.pem"))
```

### 网络策略

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: myapp-netpol
spec:
  podSelector:
    matchLabels:
      app: myapp
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: nginx
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: database
    ports:
    - protocol: TCP
      port: 5432
```

### 安全扫描

```bash
# 使用 Trivy 扫描容器镜像
trivy image myapp:latest

# 使用 gosec 扫描 Go 代码
gosec ./...

# 使用 nancy 检查依赖漏洞
nancy sleuth
```

## 性能优化

### 应用层优化

```go
// 连接池配置
db, err := sql.Open("postgres", dsn)
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)

// HTTP 客户端优化
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10,
    IdleConnTimeout:     90 * time.Second,
    DisableCompression:  false,
}

client := &http.Client{
    Transport: transport,
    Timeout:   30 * time.Second,
}
```

### 缓存策略

```go
// Redis 缓存配置
rdb := redis.NewClient(&redis.Options{
    Addr:         "localhost:6379",
    PoolSize:     10,
    MinIdleConns: 5,
    MaxRetries:   3,
})

// 缓存中间件
func cacheMiddleware(cache *redis.Client, ttl time.Duration) gin.HandlerFunc {
    return func(c *gin.Context) {
        key := c.Request.URL.Path
        
        // 尝试从缓存获取
        if cached, err := cache.Get(c, key).Result(); err == nil {
            c.Data(200, "application/json", []byte(cached))
            return
        }
        
        // 继续处理请求
        c.Next()
        
        // 缓存响应
        if c.Writer.Status() == 200 {
            response := c.Writer.(*responseWriter).body.String()
            cache.Set(c, key, response, ttl)
        }
    }
}
```

## 故障恢复

### 备份策略

```bash
#!/bin/bash
# backup.sh

# 数据库备份
pg_dump -h localhost -U user -d myapp > backup_$(date +%Y%m%d_%H%M%S).sql

# 配置文件备份
tar -czf config_backup_$(date +%Y%m%d_%H%M%S).tar.gz /etc/myapp/

# 上传到云存储
aws s3 cp backup_*.sql s3://myapp-backups/database/
aws s3 cp config_backup_*.tar.gz s3://myapp-backups/config/
```

### 灾难恢复

```yaml
# disaster-recovery.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: dr-procedures
data:
  recovery.sh: |
    #!/bin/bash
    
    # 1. 恢复数据库
    kubectl exec -it postgres-pod -- psql -U user -d myapp < backup.sql
    
    # 2. 重新部署应用
    kubectl rollout restart deployment/myapp
    
    # 3. 验证服务状态
    kubectl get pods -l app=myapp
    kubectl logs -l app=myapp --tail=100
    
    # 4. 运行健康检查
    curl -f http://myapp-service/health
```

### 回滚策略

```bash
# Kubernetes 回滚
kubectl rollout undo deployment/myapp

# 查看回滚历史
kubectl rollout history deployment/myapp

# 回滚到特定版本
kubectl rollout undo deployment/myapp --to-revision=2
```

## 最佳实践

### 部署检查清单

- [ ] 代码审查和测试通过
- [ ] 安全扫描无高危漏洞
- [ ] 性能测试满足要求
- [ ] 配置文件正确
- [ ] 环境变量设置
- [ ] 数据库迁移完成
- [ ] 监控和告警配置
- [ ] 备份策略就位
- [ ] 回滚计划准备
- [ ] 文档更新

### 生产环境配置

```yaml
# 生产环境 Kubernetes 配置
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp-prod
spec:
  replicas: 5
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 1
  template:
    spec:
      containers:
      - name: myapp
        image: myapp:stable
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        env:
        - name: ENVIRONMENT
          value: "production"
        - name: LOG_LEVEL
          value: "warn"
        securityContext:
          runAsNonRoot: true
          runAsUser: 1000
          readOnlyRootFilesystem: true
          allowPrivilegeEscalation: false
```

---

本部署指南涵盖了 NetCore-Go 应用程序在各种环境中的部署方法。根据具体需求选择合适的部署策略，并确保遵循安全和性能最佳实践。