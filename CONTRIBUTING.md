# 贡献指南

感谢您对 NetCore-Go 项目的关注和贡献！本文档将帮助您了解如何参与项目开发。

## 🤝 如何贡献

### 贡献方式

我们欢迎以下形式的贡献：

- **Bug 报告**: 发现问题请提交 Issue
- **功能建议**: 提出新功能或改进建议
- **代码贡献**: 修复 Bug 或实现新功能
- **文档改进**: 完善文档、示例和教程
- **测试用例**: 添加或改进测试覆盖率
- **性能优化**: 提升性能和资源使用效率

### 贡献流程

1. **Fork 项目**: 在 GitHub 上 Fork 本项目
2. **创建分支**: 基于 `main` 分支创建功能分支
3. **开发代码**: 按照代码规范进行开发
4. **测试验证**: 确保所有测试通过
5. **提交 PR**: 创建 Pull Request 并描述变更
6. **代码审查**: 等待维护者审查和反馈
7. **合并代码**: 审查通过后合并到主分支

## 🛠️ 开发环境设置

### 系统要求

- Go 1.19 或更高版本
- Git 版本控制
- 支持的操作系统: Linux, macOS, Windows

### 环境配置

```bash
# 1. 克隆您 Fork 的仓库
git clone https://github.com/YOUR_USERNAME/netcore-go.git
cd netcore-go

# 2. 添加上游仓库
git remote add upstream https://github.com/netcore-go/netcore.git

# 3. 安装依赖
go mod download

# 4. 验证环境
go version
go test ./...
```

### 开发工具推荐

- **IDE**: VS Code, GoLand, Vim/Neovim
- **插件**: Go 语言支持插件
- **工具**: gofmt, golint, go vet, golangci-lint

## 📝 代码规范

### Go 代码规范

遵循 [Effective Go](https://golang.org/doc/effective_go.html) 和 [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) 的指导原则。

#### 命名规范

```go
// ✅ 好的命名
type UserService struct {
    userRepo UserRepository
    logger   Logger
}

func (s *UserService) GetUserByID(ctx context.Context, userID string) (*User, error) {
    // 实现
}

// ❌ 不好的命名
type usrSvc struct {
    repo interface{}
    l    interface{}
}

func (s *usrSvc) get(id string) interface{} {
    // 实现
}
```

#### 错误处理

```go
// ✅ 正确的错误处理
func ProcessData(data []byte) (*Result, error) {
    if len(data) == 0 {
        return nil, errors.New("data cannot be empty")
    }
    
    result, err := parseData(data)
    if err != nil {
        return nil, fmt.Errorf("failed to parse data: %w", err)
    }
    
    return result, nil
}

// ❌ 错误的错误处理
func ProcessData(data []byte) *Result {
    result, _ := parseData(data) // 忽略错误
    return result
}
```

#### 接口设计

```go
// ✅ 小而专注的接口
type Reader interface {
    Read([]byte) (int, error)
}

type Writer interface {
    Write([]byte) (int, error)
}

// ✅ 组合接口
type ReadWriter interface {
    Reader
    Writer
}

// ❌ 过大的接口
type DataProcessor interface {
    Read([]byte) (int, error)
    Write([]byte) (int, error)
    Process([]byte) ([]byte, error)
    Validate([]byte) bool
    Transform([]byte) ([]byte, error)
    // ... 更多方法
}
```

### 代码格式化

使用标准工具格式化代码：

```bash
# 格式化代码
go fmt ./...

# 检查代码质量
go vet ./...

# 使用 golangci-lint (推荐)
golangci-lint run
```

### 注释规范

```go
// Package server provides high-performance TCP and UDP server implementations.
// It supports connection pooling, middleware, and various protocols.
package server

// Config represents the server configuration options.
type Config struct {
    // Network specifies the network type ("tcp", "tcp4", "tcp6", "udp", "udp4", "udp6")
    Network string `json:"network" yaml:"network"`
    
    // Address is the server listening address (e.g., ":8080", "localhost:8080")
    Address string `json:"address" yaml:"address"`
    
    // MaxConnections limits the maximum number of concurrent connections
    MaxConnections int `json:"max_connections" yaml:"max_connections"`
}

// NewServer creates a new server instance with the given configuration.
// It returns an error if the configuration is invalid.
func NewServer(config *Config) (*Server, error) {
    if config == nil {
        return nil, errors.New("config cannot be nil")
    }
    
    // 实现
    return &Server{}, nil
}
```

## 🧪 测试规范

### 测试结构

```go
func TestUserService_GetUserByID(t *testing.T) {
    tests := []struct {
        name    string
        userID  string
        want    *User
        wantErr bool
    }{
        {
            name:   "valid user ID",
            userID: "user123",
            want:   &User{ID: "user123", Name: "John Doe"},
            wantErr: false,
        },
        {
            name:    "empty user ID",
            userID:  "",
            want:    nil,
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            service := NewUserService()
            got, err := service.GetUserByID(context.Background(), tt.userID)
            
            if (err != nil) != tt.wantErr {
                t.Errorf("GetUserByID() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("GetUserByID() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### 基准测试

```go
func BenchmarkServer_HandleConnection(b *testing.B) {
    server := NewServer(&Config{
        Network: "tcp",
        Address: ":0",
    })
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            // 基准测试逻辑
        }
    })
}
```

### 测试覆盖率

```bash
# 生成覆盖率报告
go test -coverprofile=coverage.out ./...

# 查看覆盖率
go tool cover -func=coverage.out

# 生成 HTML 报告
go tool cover -html=coverage.out -o coverage.html
```

## 📋 提交规范

### 提交消息格式

使用 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

#### 提交类型

- `feat`: 新功能
- `fix`: Bug 修复
- `docs`: 文档更新
- `style`: 代码格式化（不影响功能）
- `refactor`: 代码重构
- `perf`: 性能优化
- `test`: 测试相关
- `chore`: 构建过程或辅助工具的变动

#### 示例

```bash
# 新功能
feat(server): add connection pooling support

# Bug 修复
fix(websocket): handle connection close properly

# 文档更新
docs: update installation guide

# 性能优化
perf(rpc): optimize message serialization
```

### 分支命名

- `feature/功能名称`: 新功能开发
- `fix/问题描述`: Bug 修复
- `docs/文档类型`: 文档更新
- `refactor/重构内容`: 代码重构

示例：
```bash
git checkout -b feature/websocket-compression
git checkout -b fix/memory-leak-in-connection-pool
git checkout -b docs/api-reference
```

## 🔍 Pull Request 指南

### PR 标题

使用清晰、描述性的标题：

```
✅ 好的标题
feat(server): add graceful shutdown support
fix(websocket): resolve memory leak in message handler
docs: add comprehensive API documentation

❌ 不好的标题
Update code
Fix bug
Add feature
```

### PR 描述模板

```markdown
## 变更类型
- [ ] Bug 修复
- [ ] 新功能
- [ ] 代码重构
- [ ] 性能优化
- [ ] 文档更新
- [ ] 测试改进

## 变更描述
简要描述本次变更的内容和目的。

## 相关 Issue
关闭 #123

## 测试
- [ ] 添加了新的测试用例
- [ ] 所有现有测试通过
- [ ] 手动测试通过

## 检查清单
- [ ] 代码遵循项目规范
- [ ] 添加了必要的注释
- [ ] 更新了相关文档
- [ ] 测试覆盖率满足要求

## 截图/日志
如果适用，请添加截图或日志输出。
```

### 代码审查

#### 审查要点

1. **功能正确性**: 代码是否实现了预期功能
2. **代码质量**: 是否遵循最佳实践和项目规范
3. **性能影响**: 是否有性能问题或改进空间
4. **安全性**: 是否存在安全漏洞或风险
5. **测试覆盖**: 是否有足够的测试覆盖
6. **文档完整**: 是否更新了相关文档

#### 审查反馈

```markdown
# 审查反馈示例

## 总体评价
整体实现思路正确，代码质量良好。有几个小问题需要修改。

## 具体建议

### 1. 错误处理改进
**文件**: `pkg/server/server.go:45`
**问题**: 错误信息不够详细
**建议**: 
```go
// 当前
return nil, err

// 建议
return nil, fmt.Errorf("failed to start server on %s: %w", s.address, err)
```

### 2. 性能优化
**文件**: `pkg/connection/pool.go:78`
**问题**: 频繁的内存分配
**建议**: 使用对象池减少 GC 压力

## 批准条件
- [x] 修复错误处理
- [ ] 优化内存使用
- [ ] 添加单元测试
```

## 🐛 Bug 报告

### Issue 模板

```markdown
## Bug 描述
简要描述遇到的问题。

## 复现步骤
1. 执行 '...'
2. 点击 '....'
3. 滚动到 '....'
4. 看到错误

## 期望行为
描述您期望发生的情况。

## 实际行为
描述实际发生的情况。

## 环境信息
- OS: [e.g. Ubuntu 20.04]
- Go 版本: [e.g. 1.19.5]
- NetCore-Go 版本: [e.g. v1.0.0]

## 附加信息
添加任何其他有助于解决问题的信息，如日志、截图等。
```

## 💡 功能建议

### 建议模板

```markdown
## 功能描述
简要描述建议的功能。

## 问题背景
描述当前存在的问题或不足。

## 解决方案
详细描述建议的解决方案。

## 替代方案
描述考虑过的其他解决方案。

## 附加信息
添加任何其他相关信息。
```

## 📚 文档贡献

### 文档类型

- **API 文档**: 函数、方法、结构体的详细说明
- **使用指南**: 如何使用特定功能的教程
- **示例代码**: 实际使用场景的代码示例
- **最佳实践**: 推荐的使用方式和注意事项

### 文档规范

```markdown
# 标题使用 H1

## 二级标题使用 H2

### 三级标题使用 H3

#### 代码示例

```go
// 代码示例要有注释
func ExampleFunction() {
    // 实现逻辑
}
```

#### 注意事项

> **注意**: 重要信息使用引用块

#### 链接

- [内部链接](./other-doc.md)
- [外部链接](https://example.com)
```

## 🏆 贡献者认可

### 贡献者列表

我们会在以下地方认可贡献者：

- README.md 贡献者部分
- CONTRIBUTORS.md 文件
- 发布说明中的致谢
- 项目网站（如果有）

### 贡献统计

- **代码贡献**: 提交的代码行数和质量
- **文档贡献**: 编写和改进的文档
- **问题解决**: 帮助解决的 Issue 数量
- **代码审查**: 参与的代码审查次数

## 📞 联系方式

如果您有任何问题或建议，可以通过以下方式联系我们：

- **GitHub Issues**: 项目相关问题和建议
- **GitHub Discussions**: 一般讨论和交流
- **邮箱**: netcore-go@example.com

## 📄 许可证

通过贡献代码，您同意您的贡献将在与项目相同的 MIT 许可证下发布。

---

再次感谢您对 NetCore-Go 项目的贡献！🎉