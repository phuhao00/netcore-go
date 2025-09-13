# NetCore-Go Testing Framework Example

这个示例展示了如何使用NetCore-Go测试框架进行单元测试、集成测试和端到端测试。

## 功能特性

### 🧪 测试套件类型
- **单元测试套件** - 用于测试单个函数或组件
- **集成测试套件** - 用于测试多个组件之间的交互
- **端到端测试套件** - 用于测试完整的用户场景

### 🔧 HTTP测试支持
- **GET请求** - 获取资源
- **POST请求** - 创建资源，支持JSON请求体
- **PUT请求** - 更新资源，支持JSON请求体
- **DELETE请求** - 删除资源
- **WAIT操作** - 在测试步骤之间添加延迟

### 📊 模拟组件
- **模拟数据库** - 内存中的键值存储
- **模拟缓存** - 支持TTL的缓存系统
- **测试服务器** - 用于集成测试的HTTP服务器

## 快速开始

### 运行示例

```bash
# 编译示例
go build ./examples/testing

# 运行示例
./testing
```

### 基本用法

#### 1. 单元测试

```go
// 创建单元测试套件
unitSuite := testing.NewUnitTestSuite("My Unit Tests")

// 设置测试环境
if err := unitSuite.SetUp(); err != nil {
    log.Fatal(err)
}
defer unitSuite.TearDown()

// 添加清理函数
unitSuite.AddCleanup(func() error {
    // 清理资源
    return nil
})
```

#### 2. 集成测试

```go
// 创建集成测试套件
integrationSuite := testing.NewIntegrationTestSuite("My Integration Tests")

// 设置测试环境
if err := integrationSuite.SetUp(); err != nil {
    log.Fatal(err)
}
defer integrationSuite.TearDown()

// 添加模拟数据库
mockDB := testing.NewMockDatabase()
mockDB.Set("user:1", map[string]interface{}{
    "id":   1,
    "name": "John Doe",
})
integrationSuite.AddMockDatabase("users", mockDB)

// 添加模拟缓存
mockCache := testing.NewMockCache()
mockCache.Set("session:123", "user:1", 30*time.Minute)
integrationSuite.AddMockCache("sessions", mockCache)
```

#### 3. 端到端测试

```go
// 创建E2E测试套件
e2eSuite := testing.NewE2ETestSuite("My E2E Tests", "http://localhost:8080")

// 设置测试环境
if err := e2eSuite.SetUp(); err != nil {
    log.Fatal(err)
}
defer e2eSuite.TearDown()

// 创建测试场景
scenario := testing.TestScenario{
    Name:        "User CRUD Operations",
    Description: "Test complete user management workflow",
    Steps: []testing.TestStep{
        {
            Name:   "Create User",
            Action: "POST",
            Parameters: map[string]interface{}{
                "path": "/api/users",
                "body": map[string]interface{}{
                    "name":  "Alice",
                    "email": "alice@example.com",
                },
                "headers": map[string]string{
                    "Content-Type": "application/json",
                },
            },
            Validation: func(result interface{}) error {
                if resp, ok := result.(map[string]interface{}); ok {
                    if statusCode, ok := resp["status_code"].(int); ok {
                        if statusCode != 201 {
                            return fmt.Errorf("expected 201, got %d", statusCode)
                        }
                    }
                }
                return nil
            },
        },
        {
            Name:   "Get User",
            Action: "GET",
            Parameters: map[string]interface{}{
                "path": "/api/users/1",
            },
        },
        {
            Name:   "Update User",
            Action: "PUT",
            Parameters: map[string]interface{}{
                "path": "/api/users/1",
                "body": map[string]interface{}{
                    "name":  "Alice Smith",
                    "email": "alice.smith@example.com",
                },
            },
        },
        {
            Name:   "Delete User",
            Action: "DELETE",
            Parameters: map[string]interface{}{
                "path": "/api/users/1",
            },
        },
    },
}

// 添加并运行场景
e2eSuite.AddScenario(scenario)
if err := e2eSuite.RunScenario(scenario); err != nil {
    log.Fatal(err)
}
```

## HTTP测试步骤参数

### GET请求
```go
{
    "Action": "GET",
    "Parameters": {
        "path": "/api/resource",
        "headers": {
            "Authorization": "Bearer token"
        }
    }
}
```

### POST/PUT请求
```go
{
    "Action": "POST", // 或 "PUT"
    "Parameters": {
        "path": "/api/resource",
        "body": {
            "key": "value"
        },
        "headers": {
            "Content-Type": "application/json",
            "Authorization": "Bearer token"
        }
    }
}
```

### DELETE请求
```go
{
    "Action": "DELETE",
    "Parameters": {
        "path": "/api/resource/1",
        "headers": {
            "Authorization": "Bearer token"
        }
    }
}
```

### WAIT操作
```go
{
    "Action": "WAIT",
    "Parameters": {
        "duration": time.Second * 2
    }
}
```

## 响应验证

每个测试步骤都可以包含验证函数：

```go
Validation: func(result interface{}) error {
    if resp, ok := result.(map[string]interface{}); ok {
        // 检查状态码
        if statusCode, ok := resp["status_code"].(int); ok {
            if statusCode != 200 {
                return fmt.Errorf("expected 200, got %d", statusCode)
            }
        }
        
        // 检查响应体
        if body, ok := resp["body"].(string); ok {
            if !strings.Contains(body, "expected_content") {
                return fmt.Errorf("response body does not contain expected content")
            }
        }
    }
    return nil
}
```

## 模拟组件使用

### 模拟数据库
```go
mockDB := testing.NewMockDatabase()

// 设置数据
mockDB.Set("key", "value")

// 获取数据
value, exists := mockDB.Get("key")

// 删除数据
mockDB.Delete("key")

// 清空数据
mockDB.Clear()
```

### 模拟缓存
```go
mockCache := testing.NewMockCache()

// 设置缓存（带TTL）
mockCache.Set("key", "value", 5*time.Minute)

// 获取缓存
value, exists := mockCache.Get("key")

// 删除缓存
mockCache.Delete("key")

// 清空缓存
mockCache.Clear()
```

## 最佳实践

1. **测试隔离** - 每个测试应该独立运行，不依赖其他测试的状态
2. **清理资源** - 使用AddCleanup函数确保测试后清理资源
3. **验证响应** - 为每个HTTP请求添加适当的验证函数
4. **使用模拟** - 在集成测试中使用模拟数据库和缓存
5. **场景设计** - E2E测试应该模拟真实的用户工作流程

## 注意事项

- E2E测试需要真实的API服务器运行在指定的URL
- 模拟组件仅用于测试，不应在生产环境中使用
- 测试步骤按顺序执行，前面的步骤失败会导致整个场景失败
- HTTP请求体会自动序列化为JSON格式

## 扩展功能

测试框架还支持：
- 负载测试
- 性能基准测试
- 测试报告生成（JSON、XML、HTML、控制台）
- 自定义验证函数
- 测试数据生成

更多详细信息请参考NetCore-Go文档。