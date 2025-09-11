# NetCore-Go 迁移指南

本指南帮助开发者从其他框架迁移到 NetCore-Go，提供详细的迁移步骤、代码对比和最佳实践。

## 目录

- [迁移概述](#迁移概述)
- [从 Gin 迁移](#从-gin-迁移)
- [从 Echo 迁移](#从-echo-迁移)
- [从 Fiber 迁移](#从-fiber-迁移)
- [从 Spring Boot 迁移](#从-spring-boot-迁移)
- [从 Express.js 迁移](#从-expressjs-迁移)
- [从 Django 迁移](#从-django-迁移)
- [数据库迁移](#数据库迁移)
- [配置迁移](#配置迁移)
- [中间件迁移](#中间件迁移)
- [测试迁移](#测试迁移)
- [部署迁移](#部署迁移)
- [常见问题](#常见问题)

## 迁移概述

### 为什么选择 NetCore-Go

- **统一架构**：内置服务发现、负载均衡、健康检查
- **高性能**：优化的网络处理和并发模型
- **生产就绪**：完整的监控、日志、错误处理
- **云原生**：原生支持 Kubernetes 和容器化部署
- **开发效率**：丰富的 CLI 工具和代码生成

### 迁移策略

1. **渐进式迁移**：逐步替换现有服务
2. **并行运行**：新旧系统同时运行，逐步切换流量
3. **功能对等**：确保迁移后功能完全对等
4. **性能验证**：迁移后进行性能测试和验证

### 迁移检查清单

- [ ] 分析现有架构和依赖
- [ ] 制定迁移计划和时间表
- [ ] 设置开发和测试环境
- [ ] 迁移核心业务逻辑
- [ ] 迁移数据访问层
- [ ] 迁移 API 接口
- [ ] 迁移中间件和插件
- [ ] 迁移配置和环境变量
- [ ] 迁移测试用例
- [ ] 性能测试和优化
- [ ] 部署和监控设置
- [ ] 生产环境切换

## 从 Gin 迁移

### 基本服务器设置

**Gin 代码：**
```go
package main

import (
    "github.com/gin-gonic/gin"
    "net/http"
)

func main() {
    r := gin.Default()
    
    r.GET("/ping", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{
            "message": "pong",
        })
    })
    
    r.Run(":8080")
}
```

**NetCore-Go 代码：**
```go
package main

import (
    "context"
    "net/http"
    
    "github.com/netcore-go/pkg/core"
    "github.com/netcore-go/pkg/server"
)

func main() {
    // 创建服务器配置
    config := &server.Config{
        Host: "0.0.0.0",
        Port: 8080,
        Name: "my-service",
    }
    
    // 创建服务器实例
    srv := server.New(config)
    
    // 注册路由
    srv.GET("/ping", func(ctx *server.Context) error {
        return ctx.JSON(http.StatusOK, map[string]string{
            "message": "pong",
        })
    })
    
    // 启动服务器
    if err := srv.Start(context.Background()); err != nil {
        panic(err)
    }
}
```

### 路由和处理器迁移

**Gin 路由组：**
```go
api := r.Group("/api/v1")
{
    api.GET("/users", getUsers)
    api.POST("/users", createUser)
    api.GET("/users/:id", getUser)
    api.PUT("/users/:id", updateUser)
    api.DELETE("/users/:id", deleteUser)
}
```

**NetCore-Go 路由组：**
```go
api := srv.Group("/api/v1")
{
    api.GET("/users", getUsers)
    api.POST("/users", createUser)
    api.GET("/users/:id", getUser)
    api.PUT("/users/:id", updateUser)
    api.DELETE("/users/:id", deleteUser)
}
```

### 中间件迁移

**Gin 中间件：**
```go
func Logger() gin.HandlerFunc {
    return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
        return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\
",
            param.ClientIP,
            param.TimeStamp.Format(time.RFC1123),
            param.Method,
            param.Path,
            param.Request.Proto,
            param.StatusCode,
            param.Latency,
            param.Request.UserAgent(),
            param.ErrorMessage,
        )
    })
}

r.Use(Logger())
```

**NetCore-Go 中间件：**
```go
func Logger() server.MiddlewareFunc {
    return func(next server.HandlerFunc) server.HandlerFunc {
        return func(ctx *server.Context) error {
            start := time.Now()
            
            err := next(ctx)
            
            latency := time.Since(start)
            
            core.GetGlobalLogger().WithFields(map[string]interface{}{
                "method":     ctx.Request().Method,
                "path":       ctx.Request().URL.Path,
                "status":     ctx.Response().Status,
                "latency":    latency,
                "client_ip":  ctx.ClientIP(),
                "user_agent": ctx.Request().UserAgent(),
            }).Info("Request processed")
            
            return err
        }
    }
}

srv.Use(Logger())
```

### 参数绑定迁移

**Gin 参数绑定：**
```go
type User struct {
    Name  string `json:"name" binding:"required"`
    Email string `json:"email" binding:"required,email"`
    Age   int    `json:"age" binding:"gte=0,lte=130"`
}

func createUser(c *gin.Context) {
    var user User
    if err := c.ShouldBindJSON(&user); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // 处理用户创建逻辑
    c.JSON(http.StatusCreated, user)
}
```

**NetCore-Go 参数绑定：**
```go
type User struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"gte=0,lte=130"`
}

func createUser(ctx *server.Context) error {
    var user User
    if err := ctx.Bind(&user); err != nil {
        return ctx.JSON(http.StatusBadRequest, map[string]string{
            "error": err.Error(),
        })
    }
    
    // 验证数据
    if err := ctx.Validate(&user); err != nil {
        return ctx.JSON(http.StatusBadRequest, map[string]string{
            "error": err.Error(),
        })
    }
    
    // 处理用户创建逻辑
    return ctx.JSON(http.StatusCreated, user)
}
```

## 从 Echo 迁移

### 基本服务器设置

**Echo 代码：**
```go
package main

import (
    "net/http"
    
    "github.com/labstack/echo/v4"
    "github.com/labstack/echo/v4/middleware"
)

func main() {
    e := echo.New()
    
    e.Use(middleware.Logger())
    e.Use(middleware.Recover())
    
    e.GET("/", func(c echo.Context) error {
        return c.String(http.StatusOK, "Hello, World!")
    })
    
    e.Logger.Fatal(e.Start(":1323"))
}
```

**NetCore-Go 代码：**
```go
package main

import (
    "context"
    "net/http"
    
    "github.com/netcore-go/pkg/core"
    "github.com/netcore-go/pkg/server"
)

func main() {
    config := &server.Config{
        Host: "0.0.0.0",
        Port: 1323,
        Name: "echo-migration",
    }
    
    srv := server.New(config)
    
    // 内置日志和恢复中间件
    srv.Use(server.Logger())
    srv.Use(server.Recover())
    
    srv.GET("/", func(ctx *server.Context) error {
        return ctx.String(http.StatusOK, "Hello, World!")
    })
    
    if err := srv.Start(context.Background()); err != nil {
        core.GetGlobalLogger().Fatal("Server failed to start: " + err.Error())
    }
}
```

### 自定义中间件迁移

**Echo 中间件：**
```go
func ServerHeader(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        c.Response().Header().Set(echo.HeaderServer, "Echo/3.0")
        return next(c)
    }
}

e.Use(ServerHeader)
```

**NetCore-Go 中间件：**
```go
func ServerHeader() server.MiddlewareFunc {
    return func(next server.HandlerFunc) server.HandlerFunc {
        return func(ctx *server.Context) error {
            ctx.Response().Header().Set("Server", "NetCore-Go/1.0")
            return next(ctx)
        }
    }
}

srv.Use(ServerHeader())
```

## 从 Fiber 迁移

### 基本应用设置

**Fiber 代码：**
```go
package main

import (
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
    app := fiber.New()
    
    app.Use(logger.New())
    
    app.Get("/", func(c *fiber.Ctx) error {
        return c.SendString("Hello, World!")
    })
    
    app.Listen(":3000")
}
```

**NetCore-Go 代码：**
```go
package main

import (
    "context"
    "net/http"
    
    "github.com/netcore-go/pkg/server"
)

func main() {
    config := &server.Config{
        Host: "0.0.0.0",
        Port: 3000,
        Name: "fiber-migration",
    }
    
    srv := server.New(config)
    srv.Use(server.Logger())
    
    srv.GET("/", func(ctx *server.Context) error {
        return ctx.String(http.StatusOK, "Hello, World!")
    })
    
    srv.Start(context.Background())
}
```

### 路由参数迁移

**Fiber 路由参数：**
```go
app.Get("/user/:name", func(c *fiber.Ctx) error {
    name := c.Params("name")
    return c.SendString("Hello, " + name)
})

app.Get("/user/:name/books/:title", func(c *fiber.Ctx) error {
    name := c.Params("name")
    title := c.Params("title")
    return c.SendString(name + " is reading " + title)
})
```

**NetCore-Go 路由参数：**
```go
srv.GET("/user/:name", func(ctx *server.Context) error {
    name := ctx.Param("name")
    return ctx.String(http.StatusOK, "Hello, "+name)
})

srv.GET("/user/:name/books/:title", func(ctx *server.Context) error {
    name := ctx.Param("name")
    title := ctx.Param("title")
    return ctx.String(http.StatusOK, name+" is reading "+title)
})
```

## 从 Spring Boot 迁移

### 控制器迁移

**Spring Boot 控制器：**
```java
@RestController
@RequestMapping("/api/users")
public class UserController {
    
    @Autowired
    private UserService userService;
    
    @GetMapping
    public ResponseEntity<List<User>> getAllUsers() {
        List<User> users = userService.findAll();
        return ResponseEntity.ok(users);
    }
    
    @PostMapping
    public ResponseEntity<User> createUser(@RequestBody @Valid User user) {
        User createdUser = userService.save(user);
        return ResponseEntity.status(HttpStatus.CREATED).body(createdUser);
    }
    
    @GetMapping("/{id}")
    public ResponseEntity<User> getUser(@PathVariable Long id) {
        User user = userService.findById(id);
        if (user == null) {
            return ResponseEntity.notFound().build();
        }
        return ResponseEntity.ok(user);
    }
}
```

**NetCore-Go 控制器：**
```go
type UserController struct {
    userService *UserService
}

func NewUserController(userService *UserService) *UserController {
    return &UserController{
        userService: userService,
    }
}

func (uc *UserController) RegisterRoutes(srv *server.Server) {
    api := srv.Group("/api/users")
    {
        api.GET("", uc.getAllUsers)
        api.POST("", uc.createUser)
        api.GET("/:id", uc.getUser)
    }
}

func (uc *UserController) getAllUsers(ctx *server.Context) error {
    users, err := uc.userService.FindAll()
    if err != nil {
        return ctx.JSON(http.StatusInternalServerError, map[string]string{
            "error": err.Error(),
        })
    }
    return ctx.JSON(http.StatusOK, users)
}

func (uc *UserController) createUser(ctx *server.Context) error {
    var user User
    if err := ctx.Bind(&user); err != nil {
        return ctx.JSON(http.StatusBadRequest, map[string]string{
            "error": err.Error(),
        })
    }
    
    if err := ctx.Validate(&user); err != nil {
        return ctx.JSON(http.StatusBadRequest, map[string]string{
            "error": err.Error(),
        })
    }
    
    createdUser, err := uc.userService.Save(&user)
    if err != nil {
        return ctx.JSON(http.StatusInternalServerError, map[string]string{
            "error": err.Error(),
        })
    }
    
    return ctx.JSON(http.StatusCreated, createdUser)
}

func (uc *UserController) getUser(ctx *server.Context) error {
    id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
    if err != nil {
        return ctx.JSON(http.StatusBadRequest, map[string]string{
            "error": "Invalid user ID",
        })
    }
    
    user, err := uc.userService.FindByID(id)
    if err != nil {
        return ctx.JSON(http.StatusNotFound, map[string]string{
            "error": "User not found",
        })
    }
    
    return ctx.JSON(http.StatusOK, user)
}
```

### 依赖注入迁移

**Spring Boot 依赖注入：**
```java
@Service
public class UserService {
    
    @Autowired
    private UserRepository userRepository;
    
    public List<User> findAll() {
        return userRepository.findAll();
    }
    
    public User save(User user) {
        return userRepository.save(user);
    }
}
```

**NetCore-Go 依赖注入：**
```go
// 使用构造函数注入
type UserService struct {
    userRepository UserRepository
    logger         *core.Logger
}

func NewUserService(userRepository UserRepository, logger *core.Logger) *UserService {
    return &UserService{
        userRepository: userRepository,
        logger:         logger,
    }
}

func (us *UserService) FindAll() ([]*User, error) {
    users, err := us.userRepository.FindAll()
    if err != nil {
        us.logger.Error("Failed to find all users: " + err.Error())
        return nil, err
    }
    return users, nil
}

func (us *UserService) Save(user *User) (*User, error) {
    savedUser, err := us.userRepository.Save(user)
    if err != nil {
        us.logger.Error("Failed to save user: " + err.Error())
        return nil, err
    }
    return savedUser, nil
}

// 在 main 函数中手动装配依赖
func main() {
    // 创建依赖
    logger := core.NewLogger(core.DefaultLoggerConfig())
    db := setupDatabase()
    userRepository := NewUserRepository(db)
    userService := NewUserService(userRepository, logger)
    userController := NewUserController(userService)
    
    // 创建服务器
    srv := server.New(&server.Config{
        Host: "0.0.0.0",
        Port: 8080,
        Name: "spring-migration",
    })
    
    // 注册路由
    userController.RegisterRoutes(srv)
    
    // 启动服务器
    srv.Start(context.Background())
}
```

## 从 Express.js 迁移

### 基本应用设置

**Express.js 代码：**
```javascript
const express = require('express');
const app = express();
const port = 3000;

app.use(express.json());

app.get('/', (req, res) => {
  res.send('Hello World!');
});

app.listen(port, () => {
  console.log(`Server running at http://localhost:${port}`);
});
```

**NetCore-Go 代码：**
```go
package main

import (
    "context"
    "net/http"
    
    "github.com/netcore-go/pkg/core"
    "github.com/netcore-go/pkg/server"
)

func main() {
    config := &server.Config{
        Host: "0.0.0.0",
        Port: 3000,
        Name: "express-migration",
    }
    
    srv := server.New(config)
    
    // 内置 JSON 解析
    srv.Use(server.JSONParser())
    
    srv.GET("/", func(ctx *server.Context) error {
        return ctx.String(http.StatusOK, "Hello World!")
    })
    
    core.GetGlobalLogger().Info("Server running at http://localhost:3000")
    srv.Start(context.Background())
}
```

### 中间件迁移

**Express.js 中间件：**
```javascript
const cors = require('cors');
const helmet = require('helmet');
const morgan = require('morgan');

app.use(cors());
app.use(helmet());
app.use(morgan('combined'));

// 自定义中间件
app.use((req, res, next) => {
  req.timestamp = Date.now();
  next();
});
```

**NetCore-Go 中间件：**
```go
// 使用内置中间件
srv.Use(server.CORS())
srv.Use(server.Security())
srv.Use(server.Logger())

// 自定义中间件
func TimestampMiddleware() server.MiddlewareFunc {
    return func(next server.HandlerFunc) server.HandlerFunc {
        return func(ctx *server.Context) error {
            ctx.Set("timestamp", time.Now().Unix())
            return next(ctx)
        }
    }
}

srv.Use(TimestampMiddleware())
```

### 路由迁移

**Express.js 路由：**
```javascript
const userRouter = express.Router();

userRouter.get('/', (req, res) => {
  res.json({ users: [] });
});

userRouter.post('/', (req, res) => {
  const user = req.body;
  // 处理用户创建
  res.status(201).json(user);
});

userRouter.get('/:id', (req, res) => {
  const id = req.params.id;
  // 获取用户
  res.json({ id, name: 'John Doe' });
});

app.use('/api/users', userRouter);
```

**NetCore-Go 路由：**
```go
func setupUserRoutes(srv *server.Server) {
    userGroup := srv.Group("/api/users")
    {
        userGroup.GET("/", func(ctx *server.Context) error {
            return ctx.JSON(http.StatusOK, map[string]interface{}{
                "users": []interface{}{},
            })
        })
        
        userGroup.POST("/", func(ctx *server.Context) error {
            var user map[string]interface{}
            if err := ctx.Bind(&user); err != nil {
                return ctx.JSON(http.StatusBadRequest, map[string]string{
                    "error": err.Error(),
                })
            }
            
            // 处理用户创建
            return ctx.JSON(http.StatusCreated, user)
        })
        
        userGroup.GET("/:id", func(ctx *server.Context) error {
            id := ctx.Param("id")
            // 获取用户
            return ctx.JSON(http.StatusOK, map[string]interface{}{
                "id":   id,
                "name": "John Doe",
            })
        })
    }
}
```

## 从 Django 迁移

### 视图迁移

**Django 视图：**
```python
from django.http import JsonResponse
from django.views.decorators.csrf import csrf_exempt
from django.views.decorators.http import require_http_methods
import json

@csrf_exempt
@require_http_methods(["GET", "POST"])
def user_list(request):
    if request.method == 'GET':
        users = User.objects.all()
        return JsonResponse({
            'users': [{'id': u.id, 'name': u.name} for u in users]
        })
    
    elif request.method == 'POST':
        data = json.loads(request.body)
        user = User.objects.create(
            name=data['name'],
            email=data['email']
        )
        return JsonResponse({
            'id': user.id,
            'name': user.name,
            'email': user.email
        }, status=201)
```

**NetCore-Go 处理器：**
```go
type UserHandler struct {
    userService *UserService
}

func (uh *UserHandler) UserList(ctx *server.Context) error {
    switch ctx.Request().Method {
    case http.MethodGet:
        users, err := uh.userService.GetAll()
        if err != nil {
            return ctx.JSON(http.StatusInternalServerError, map[string]string{
                "error": err.Error(),
            })
        }
        
        userList := make([]map[string]interface{}, len(users))
        for i, u := range users {
            userList[i] = map[string]interface{}{
                "id":   u.ID,
                "name": u.Name,
            }
        }
        
        return ctx.JSON(http.StatusOK, map[string]interface{}{
            "users": userList,
        })
        
    case http.MethodPost:
        var userData struct {
            Name  string `json:"name" validate:"required"`
            Email string `json:"email" validate:"required,email"`
        }
        
        if err := ctx.Bind(&userData); err != nil {
            return ctx.JSON(http.StatusBadRequest, map[string]string{
                "error": err.Error(),
            })
        }
        
        if err := ctx.Validate(&userData); err != nil {
            return ctx.JSON(http.StatusBadRequest, map[string]string{
                "error": err.Error(),
            })
        }
        
        user, err := uh.userService.Create(userData.Name, userData.Email)
        if err != nil {
            return ctx.JSON(http.StatusInternalServerError, map[string]string{
                "error": err.Error(),
            })
        }
        
        return ctx.JSON(http.StatusCreated, map[string]interface{}{
            "id":    user.ID,
            "name":  user.Name,
            "email": user.Email,
        })
        
    default:
        return ctx.JSON(http.StatusMethodNotAllowed, map[string]string{
            "error": "Method not allowed",
        })
    }
}
```

### URL 配置迁移

**Django URLs：**
```python
from django.urls import path, include
from . import views

urlpatterns = [
    path('api/users/', views.user_list, name='user_list'),
    path('api/users/<int:user_id>/', views.user_detail, name='user_detail'),
    path('api/auth/', include('authentication.urls')),
]
```

**NetCore-Go 路由：**
```go
func setupRoutes(srv *server.Server) {
    userHandler := NewUserHandler(userService)
    
    api := srv.Group("/api")
    {
        // 用户路由
        users := api.Group("/users")
        {
            users.GET("/", userHandler.UserList)
            users.POST("/", userHandler.UserList)
            users.GET("/:user_id", userHandler.UserDetail)
        }
        
        // 认证路由
        auth := api.Group("/auth")
        {
            setupAuthRoutes(auth)
        }
    }
}
```

## 数据库迁移

### ORM 迁移

**从 GORM 迁移：**
```go
// 原 GORM 代码
type User struct {
    gorm.Model
    Name  string `gorm:"size:255;not null"`
    Email string `gorm:"uniqueIndex;size:255;not null"`
}

func GetUser(db *gorm.DB, id uint) (*User, error) {
    var user User
    result := db.First(&user, id)
    if result.Error != nil {
        return nil, result.Error
    }
    return &user, nil
}
```

**NetCore-Go 数据访问：**
```go
// 使用标准 database/sql 或集成的 ORM
type User struct {
    ID        int64     `json:"id" db:"id"`
    Name      string    `json:"name" db:"name" validate:"required,max=255"`
    Email     string    `json:"email" db:"email" validate:"required,email,max=255"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
    UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type UserRepository struct {
    db *sql.DB
}

func (ur *UserRepository) GetByID(id int64) (*User, error) {
    query := `
        SELECT id, name, email, created_at, updated_at 
        FROM users 
        WHERE id = $1
    `
    
    var user User
    err := ur.db.QueryRow(query, id).Scan(
        &user.ID,
        &user.Name,
        &user.Email,
        &user.CreatedAt,
        &user.UpdatedAt,
    )
    
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, ErrUserNotFound
        }
        return nil, err
    }
    
    return &user, nil
}
```

### 数据库连接迁移

**原数据库连接：**
```go
// GORM 连接
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})

// 或者原生连接
db, err := sql.Open("postgres", dsn)
```

**NetCore-Go 数据库连接：**
```go
// 使用 NetCore-Go 的数据库配置
dbConfig := &database.Config{
    Driver:   "postgres",
    Host:     "localhost",
    Port:     5432,
    Database: "myapp",
    Username: "user",
    Password: "password",
    
    // 连接池配置
    MaxOpenConns:    25,
    MaxIdleConns:    5,
    ConnMaxLifetime: 5 * time.Minute,
}

db, err := database.Connect(dbConfig)
if err != nil {
    panic(err)
}
```

## 配置迁移

### 环境变量和配置文件

**原配置方式：**
```go
// 使用 viper
viper.SetConfigName("config")
viper.SetConfigType("yaml")
viper.AddConfigPath(".")
viper.AutomaticEnv()

if err := viper.ReadInConfig(); err != nil {
    panic(err)
}

port := viper.GetInt("server.port")
dbURL := viper.GetString("database.url")
```

**NetCore-Go 配置：**
```go
// 使用 NetCore-Go 配置管理
type AppConfig struct {
    Server   ServerConfig   `yaml:"server"`
    Database DatabaseConfig `yaml:"database"`
    Redis    RedisConfig    `yaml:"redis"`
}

type ServerConfig struct {
    Host string `yaml:"host" env:"SERVER_HOST" default:"0.0.0.0"`
    Port int    `yaml:"port" env:"SERVER_PORT" default:"8080"`
}

type DatabaseConfig struct {
    URL      string `yaml:"url" env:"DATABASE_URL"`
    MaxConns int    `yaml:"max_conns" env:"DB_MAX_CONNS" default:"25"`
}

func loadConfig() (*AppConfig, error) {
    config := &AppConfig{}
    
    // 加载配置文件
    if err := core.LoadConfig("config.yaml", config); err != nil {
        return nil, err
    }
    
    // 验证配置
    if err := core.ValidateConfig(config); err != nil {
        return nil, err
    }
    
    return config, nil
}
```

### 配置文件格式迁移

**原 YAML 配置：**
```yaml
server:
  port: 8080
  host: localhost

database:
  host: localhost
  port: 5432
  name: myapp
  user: postgres
  password: secret

redis:
  addr: localhost:6379
  password: ""
  db: 0
```

**NetCore-Go YAML 配置：**
```yaml
server:
  host: 0.0.0.0
  port: 8080
  name: myapp
  timeout: 30s
  
database:
  driver: postgres
  host: localhost
  port: 5432
  database: myapp
  username: postgres
  password: secret
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 5m
  
redis:
  addr: localhost:6379
  password: ""
  db: 0
  pool_size: 10
  min_idle_conns: 5
  
discovery:
  type: consul
  address: localhost:8500
  
logging:
  level: info
  format: json
  output: stdout
```

## 中间件迁移

### 认证中间件

**原认证中间件：**
```go
func JWTMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        if token == "" {
            c.JSON(401, gin.H{"error": "Missing token"})
            c.Abort()
            return
        }
        
        // 验证 token
        claims, err := validateJWT(token)
        if err != nil {
            c.JSON(401, gin.H{"error": "Invalid token"})
            c.Abort()
            return
        }
        
        c.Set("user_id", claims.UserID)
        c.Next()
    }
}
```

**NetCore-Go 认证中间件：**
```go
func JWTMiddleware(jwtSecret string) server.MiddlewareFunc {
    return func(next server.HandlerFunc) server.HandlerFunc {
        return func(ctx *server.Context) error {
            token := ctx.Request().Header.Get("Authorization")
            if token == "" {
                return ctx.JSON(http.StatusUnauthorized, map[string]string{
                    "error": "Missing token",
                })
            }
            
            // 移除 "Bearer " 前缀
            if strings.HasPrefix(token, "Bearer ") {
                token = token[7:]
            }
            
            // 验证 token
            claims, err := validateJWT(token, jwtSecret)
            if err != nil {
                return ctx.JSON(http.StatusUnauthorized, map[string]string{
                    "error": "Invalid token",
                })
            }
            
            // 设置用户信息到上下文
            ctx.Set("user_id", claims.UserID)
            ctx.Set("user_role", claims.Role)
            
            return next(ctx)
        }
    }
}

// 使用中间件
srv.Use(JWTMiddleware(jwtSecret))
```

### 限流中间件

**原限流中间件：**
```go
func RateLimitMiddleware(limit int, window time.Duration) gin.HandlerFunc {
    limiter := rate.NewLimiter(rate.Every(window/time.Duration(limit)), limit)
    
    return func(c *gin.Context) {
        if !limiter.Allow() {
            c.JSON(429, gin.H{"error": "Rate limit exceeded"})
            c.Abort()
            return
        }
        c.Next()
    }
}
```

**NetCore-Go 限流中间件：**
```go
func RateLimitMiddleware(rps int, burst int) server.MiddlewareFunc {
    limiter := rate.NewLimiter(rate.Limit(rps), burst)
    
    return func(next server.HandlerFunc) server.HandlerFunc {
        return func(ctx *server.Context) error {
            if !limiter.Allow() {
                return ctx.JSON(http.StatusTooManyRequests, map[string]string{
                    "error": "Rate limit exceeded",
                })
            }
            return next(ctx)
        }
    }
}

// 或使用内置的限流中间件
srv.Use(server.RateLimit(&server.RateLimitConfig{
    RPS:    100,
    Burst:  200,
    Window: time.Minute,
}))
```

## 测试迁移

### 单元测试迁移

**原测试代码：**
```go
func TestGetUser(t *testing.T) {
    gin.SetMode(gin.TestMode)
    
    r := gin.Default()
    r.GET("/users/:id", getUserHandler)
    
    req, _ := http.NewRequest("GET", "/users/1", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    
    assert.Equal(t, 200, w.Code)
    
    var response map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &response)
    assert.Equal(t, "John Doe", response["name"])
}
```

**NetCore-Go 测试代码：**
```go
func TestGetUser(t *testing.T) {
    // 创建测试服务器
    config := &server.Config{
        Host: "localhost",
        Port: 0, // 随机端口
        Name: "test-server",
    }
    
    srv := server.New(config)
    srv.GET("/users/:id", getUserHandler)
    
    // 使用测试框架
    testSuite := testing.NewTestSuite(srv)
    
    // 发送测试请求
    resp, err := testSuite.GET("/users/1").Expect()
    assert.NoError(t, err)
    assert.Equal(t, http.StatusOK, resp.StatusCode)
    
    var response map[string]interface{}
    err = json.Unmarshal(resp.Body, &response)
    assert.NoError(t, err)
    assert.Equal(t, "John Doe", response["name"])
}
```

### 集成测试迁移

**NetCore-Go 集成测试：**
```go
func TestUserAPI(t *testing.T) {
    // 设置测试环境
    testDB := setupTestDatabase()
    defer testDB.Close()
    
    userRepo := NewUserRepository(testDB)
    userService := NewUserService(userRepo)
    userHandler := NewUserHandler(userService)
    
    // 创建测试服务器
    srv := server.New(&server.Config{
        Host: "localhost",
        Port: 0,
        Name: "integration-test",
    })
    
    userHandler.RegisterRoutes(srv)
    
    // 启动测试服务器
    testSuite := testing.NewIntegrationTestSuite(srv)
    testSuite.SetUp()
    defer testSuite.TearDown()
    
    // 测试用户创建
    t.Run("CreateUser", func(t *testing.T) {
        userData := map[string]interface{}{
            "name":  "John Doe",
            "email": "john@example.com",
        }
        
        resp, err := testSuite.POST("/api/users").JSON(userData).Expect()
        assert.NoError(t, err)
        assert.Equal(t, http.StatusCreated, resp.StatusCode)
        
        var user User
        err = json.Unmarshal(resp.Body, &user)
        assert.NoError(t, err)
        assert.Equal(t, "John Doe", user.Name)
        assert.Equal(t, "john@example.com", user.Email)
    })
    
    // 测试用户获取
    t.Run("GetUser", func(t *testing.T) {
        resp, err := testSuite.GET("/api/users/1").Expect()
        assert.NoError(t, err)
        assert.Equal(t, http.StatusOK, resp.StatusCode)
    })
}
```

## 部署迁移

### Docker 迁移

**原 Dockerfile：**
```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o main .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
CMD ["./main"]
```

**NetCore-Go Dockerfile：**
```dockerfile
FROM golang:1.21-alpine AS builder

# 安装必要工具
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o main ./cmd/server

# 运行阶段
FROM scratch

# 复制必要文件
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /app/main /main
COPY --from=builder /app/configs /configs

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["./main", "--health-check"]

# 启动应用
ENTRYPOINT ["/main"]
```

### Kubernetes 部署迁移

**NetCore-Go Kubernetes 配置：**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: netcore-app
  labels:
    app: netcore-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: netcore-app
  template:
    metadata:
      labels:
        app: netcore-app
    spec:
      containers:
      - name: netcore-app
        image: netcore-app:latest
        ports:
        - containerPort: 8080
        env:
        - name: SERVER_HOST
          value: "0.0.0.0"
        - name: SERVER_PORT
          value: "8080"
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: app-secrets
              key: database-url
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
        # NetCore-Go 特有的服务发现配置
        env:
        - name: DISCOVERY_TYPE
          value: "kubernetes"
        - name: DISCOVERY_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
---
apiVersion: v1
kind: Service
metadata:
  name: netcore-app-service
  labels:
    app: netcore-app
spec:
  selector:
    app: netcore-app
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8080
  type: ClusterIP
```

## 常见问题

### Q: 如何处理现有的数据库迁移？

**A:** NetCore-Go 支持多种数据库迁移方式：

```go
// 1. 使用内置迁移工具
migrator := database.NewMigrator(db)
migrator.AddMigration("001_create_users_table.sql")
migrator.AddMigration("002_add_email_index.sql")

if err := migrator.Migrate(); err != nil {
    panic(err)
}

// 2. 或者使用 CLI 工具
// netcore-cli db migrate --up
// netcore-cli db migrate --down
// netcore-cli db migrate --status
```

### Q: 如何保持 API 兼容性？

**A:** 在迁移过程中保持 API 兼容性：

```go
// 1. 使用版本化 API
v1 := srv.Group("/api/v1")
v2 := srv.Group("/api/v2")

// 2. 渐进式迁移
func migrateEndpoint(oldHandler, newHandler server.HandlerFunc) server.HandlerFunc {
    return func(ctx *server.Context) error {
        // 根据条件选择处理器
        if ctx.Query("use_new") == "true" {
            return newHandler(ctx)
        }
        return oldHandler(ctx)
    }
}

// 3. 使用特性开关
if featureFlags.IsEnabled("new_user_api") {
    srv.GET("/users", newUserHandler)
} else {
    srv.GET("/users", oldUserHandler)
}
```

### Q: 如何处理大量的现有中间件？

**A:** 创建适配器来复用现有中间件：

```go
// Gin 中间件适配器
func AdaptGinMiddleware(ginMiddleware gin.HandlerFunc) server.MiddlewareFunc {
    return func(next server.HandlerFunc) server.HandlerFunc {
        return func(ctx *server.Context) error {
            // 创建 Gin 兼容的上下文
            ginCtx := &gin.Context{
                Request: ctx.Request(),
                Writer:  ctx.Response(),
            }
            
            // 调用原 Gin 中间件
            ginMiddleware(ginCtx)
            
            // 检查是否被中止
            if ginCtx.IsAborted() {
                return nil
            }
            
            return next(ctx)
        }
    }
}

// 使用适配器
srv.Use(AdaptGinMiddleware(existingGinMiddleware))
```

### Q: 如何进行性能对比？

**A:** 使用基准测试对比性能：

```go
// 原框架基准测试
func BenchmarkOldFramework(b *testing.B) {
    // 设置原框架
    oldServer := setupOldServer()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        req := httptest.NewRequest("GET", "/api/users", nil)
        w := httptest.NewRecorder()
        oldServer.ServeHTTP(w, req)
    }
}

// NetCore-Go 基准测试
func BenchmarkNetCoreGo(b *testing.B) {
    // 设置 NetCore-Go
    srv := setupNetCoreGoServer()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        req := httptest.NewRequest("GET", "/api/users", nil)
        w := httptest.NewRecorder()
        srv.ServeHTTP(w, req)
    }
}

// 运行对比测试
// go test -bench=. -benchmem
```

### Q: 如何处理配置迁移？

**A:** 使用配置转换工具：

```go
// 配置转换器
type ConfigMigrator struct {
    oldConfig map[string]interface{}
    newConfig *NetCoreConfig
}

func (cm *ConfigMigrator) Migrate() error {
    // 转换服务器配置
    if port, ok := cm.oldConfig["port"].(int); ok {
        cm.newConfig.Server.Port = port
    }
    
    // 转换数据库配置
    if dbConfig, ok := cm.oldConfig["database"].(map[string]interface{}); ok {
        cm.newConfig.Database.Host = dbConfig["host"].(string)
        cm.newConfig.Database.Port = dbConfig["port"].(int)
        // ...
    }
    
    return nil
}

// 使用 CLI 工具进行配置迁移
// netcore-cli migrate config --from gin --input config.yaml --output netcore-config.yaml
```

---

本迁移指南提供了从主流框架迁移到 NetCore-Go 的详细步骤和代码示例。根据您的具体情况选择合适的迁移策略，并确保在迁移过程中进行充分的测试和验证。