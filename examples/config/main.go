package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/phuhao00/netcore-go/pkg/config"
)

// AppConfig 应用配置结构体
type AppConfig struct {
	Server   ServerConfig   `json:"server" yaml:"server"`
	Database DatabaseConfig `json:"database" yaml:"database"`
	Redis    RedisConfig    `json:"redis" yaml:"redis"`
	Log      LogConfig      `json:"log" yaml:"log"`
	Debug    bool           `json:"debug" yaml:"debug"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host         string        `json:"host" yaml:"host"`
	Port         int           `json:"port" yaml:"port"`
	ReadTimeout  time.Duration `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`
	MaxConns     int           `json:"max_conns" yaml:"max_conns"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver   string `json:"driver" yaml:"driver"`
	Host     string `json:"host" yaml:"host"`
	Port     int    `json:"port" yaml:"port"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Database string `json:"database" yaml:"database"`
	SSLMode  string `json:"ssl_mode" yaml:"ssl_mode"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string `json:"host" yaml:"host"`
	Port     int    `json:"port" yaml:"port"`
	Password string `json:"password" yaml:"password"`
	DB       int    `json:"db" yaml:"db"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string `json:"level" yaml:"level"`
	Format string `json:"format" yaml:"format"`
	Output string `json:"output" yaml:"output"`
}

// ConfigWatcher 配置变更监听器
type ConfigWatcher struct {
	name string
}

// OnConfigChange 配置变更回调
func (w *ConfigWatcher) OnConfigChange(key string, oldValue, newValue interface{}) {
	fmt.Printf("[%s] Config changed: %s = %v -> %v\n", w.name, key, oldValue, newValue)
}

func main() {
	fmt.Println("=== NetCore-Go 配置管理系统示例 ===")

	// 示例1: 基本配置加载
	fmt.Println("\n1. 基本配置加载示例")
	basicConfigExample()

	// 示例2: 配置构建器
	fmt.Println("\n2. 配置构建器示例")
	configBuilderExample()

	// 示例3: 环境变量配置
	fmt.Println("\n3. 环境变量配置示例")
	envConfigExample()

	// 示例4: 配置档案管理
	fmt.Println("\n4. 配置档案管理示例")
	profileConfigExample()

	// 示例5: 配置监听器
	fmt.Println("\n5. 配置监听器示例")
	configWatcherExample()

	// 示例6: 结构体映射
	fmt.Println("\n6. 结构体映射示例")
	structMappingExample()

	// 示例7: 配置合并
	fmt.Println("\n7. 配置合并示例")
	configMergeExample()
}

// basicConfigExample 基本配置加载示例
func basicConfigExample() {
	// 创建示例配置文件
	createExampleConfigFiles()

	// 加载JSON配置
	jsonConfig := config.NewConfig(&config.ConfigOptions{
		Format:   config.FormatJSON,
		FilePath: "example.json",
	})

	if err := jsonConfig.LoadFromFile("example.json"); err != nil {
		log.Printf("加载JSON配置失败: %v", err)
	} else {
		fmt.Printf("JSON配置 - 服务器端口: %d\n", jsonConfig.GetInt("server.port"))
		fmt.Printf("JSON配置 - 调试模式: %v\n", jsonConfig.GetBool("debug"))
	}

	// 加载YAML配置
	yamlConfig := config.NewConfig(&config.ConfigOptions{
		Format:   config.FormatYAML,
		FilePath: "example.yaml",
	})

	if err := yamlConfig.LoadFromFile("example.yaml"); err != nil {
		log.Printf("加载YAML配置失败: %v", err)
	} else {
		fmt.Printf("YAML配置 - 数据库主机: %s\n", yamlConfig.GetString("database.host"))
		fmt.Printf("YAML配置 - 读取超时: %v\n", yamlConfig.GetDuration("server.read_timeout"))
	}
}

// configBuilderExample 配置构建器示例
func configBuilderExample() {
	defaults := map[string]interface{}{
		"server.host":         "localhost",
		"server.port":         8080,
		"server.read_timeout": "30s",
		"debug":               true,
	}

	config, err := config.NewBuilder().
		WithDefaults(defaults).
		WithFile("example.yaml").
		WithEnv("APP").
		Build()

	if err != nil {
		log.Printf("构建配置失败: %v", err)
		return
	}

	fmt.Printf("构建器配置 - 服务器地址: %s:%d\n",
		config.GetString("server.host"),
		config.GetInt("server.port"))
	fmt.Printf("构建器配置 - 所有配置: %+v\n", config.GetAll())
}

// envConfigExample 环境变量配置示例
func envConfigExample() {
	// 设置一些环境变量
	os.Setenv("APP_SERVER_HOST", "0.0.0.0")
	os.Setenv("APP_SERVER_PORT", "9090")
	os.Setenv("APP_DEBUG", "false")
	os.Setenv("APP_DATABASE_HOST", "db.example.com")

	envConfig := config.NewConfig(&config.ConfigOptions{
		Format: config.FormatENV,
	})

	if err := envConfig.LoadFromEnv("APP"); err != nil {
		log.Printf("加载环境变量配置失败: %v", err)
		return
	}

	fmt.Printf("环境变量配置 - 服务器主机: %s\n", envConfig.GetString("server.host"))
	fmt.Printf("环境变量配置 - 服务器端口: %d\n", envConfig.GetInt("server.port"))
	fmt.Printf("环境变量配置 - 调试模式: %v\n", envConfig.GetBool("debug"))
	fmt.Printf("环境变量配置 - 数据库主机: %s\n", envConfig.GetString("database.host"))
}

// profileConfigExample 配置档案管理示例
func profileConfigExample() {
	pm := config.GetProfileManager()

	// 设置环境变量来模拟不同环境
	os.Setenv("APP_ENV", "development")

	// 自动检测并加载配置档案
	config, err := pm.AutoDetectProfile()
	if err != nil {
		log.Printf("自动加载配置档案失败: %v", err)
		return
	}

	fmt.Printf("当前活跃档案: %s\n", pm.GetActiveProfile())
	fmt.Printf("档案配置 - 调试模式: %v\n", config.GetBool("debug"))
	fmt.Printf("档案配置 - 日志级别: %s\n", config.GetString("log.level"))
	fmt.Printf("档案配置 - 服务器端口: %d\n", config.GetInt("server.port"))

	// 切换到生产环境
	prodConfig, err := pm.LoadProfile("production")
	if err != nil {
		log.Printf("加载生产环境配置失败: %v", err)
	} else {
		fmt.Printf("\n切换到生产环境:")
		fmt.Printf("生产环境 - 调试模式: %v\n", prodConfig.GetBool("debug"))
		fmt.Printf("生产环境 - 日志级别: %s\n", prodConfig.GetString("log.level"))
		fmt.Printf("生产环境 - 服务器端口: %d\n", prodConfig.GetInt("server.port"))
	}
}

// configWatcherExample 配置监听器示例
func configWatcherExample() {
	config := config.NewConfig(nil)

	// 添加监听器
	watcher := &ConfigWatcher{name: "示例监听器"}
	config.AddWatcher(watcher)

	// 设置一些配置值
	config.Set("server.port", 8080)
	config.Set("debug", true)
	config.Set("database.host", "localhost")

	// 修改配置值
	config.Set("server.port", 9090)
	config.Set("debug", false)
}

// structMappingExample 结构体映射示例
func structMappingExample() {
	// 创建配置
	config := config.NewConfig(nil)

	// 设置配置值
	config.Set("server.host", "localhost")
	config.Set("server.port", 8080)
	config.Set("server.read_timeout", "30s")
	config.Set("server.write_timeout", "30s")
	config.Set("server.max_conns", 1000)

	config.Set("database.driver", "postgres")
	config.Set("database.host", "localhost")
	config.Set("database.port", 5432)
	config.Set("database.username", "user")
	config.Set("database.password", "password")
	config.Set("database.database", "mydb")
	config.Set("database.ssl_mode", "disable")

	config.Set("redis.host", "localhost")
	config.Set("redis.port", 6379)
	config.Set("redis.password", "")
	config.Set("redis.db", 0)

	config.Set("log.level", "info")
	config.Set("log.format", "json")
	config.Set("log.output", "stdout")

	config.Set("debug", true)

	// 映射到结构体
	var appConfig AppConfig
	if err := config.Unmarshal(&appConfig); err != nil {
		log.Printf("映射到结构体失败: %v", err)
		return
	}

	fmt.Printf("应用配置: %+v\n", appConfig)
	fmt.Printf("服务器配置: %+v\n", appConfig.Server)
	fmt.Printf("数据库配置: %+v\n", appConfig.Database)

	// 映射单个键到结构体
	var serverConfig ServerConfig
	if err := config.UnmarshalKey("server", &serverConfig); err != nil {
		log.Printf("映射服务器配置失败: %v", err)
	} else {
		fmt.Printf("服务器配置（单独映射）: %+v\n", serverConfig)
	}
}

// configMergeExample 配置合并示例
func configMergeExample() {
	// 创建基础配置
	baseConfig := config.NewConfig(nil)
	baseConfig.Set("server.host", "localhost")
	baseConfig.Set("server.port", 8080)
	baseConfig.Set("database.host", "localhost")
	baseConfig.Set("database.port", 5432)

	// 创建覆盖配置
	overrideConfig := config.NewConfig(nil)
	overrideConfig.Set("server.port", 9090)
	overrideConfig.Set("database.password", "secret")
	overrideConfig.Set("redis.host", "redis.example.com")

	fmt.Printf("合并前基础配置: %+v\n", baseConfig.GetAll())
	fmt.Printf("覆盖配置: %+v\n", overrideConfig.GetAll())

	// 合并配置
	baseConfig.Merge(overrideConfig)

	fmt.Printf("合并后配置: %+v\n", baseConfig.GetAll())

	// 克隆配置
	clonedConfig := baseConfig.Clone()
	clonedConfig.Set("new.key", "new.value")

	fmt.Printf("原始配置: %+v\n", baseConfig.GetAll())
	fmt.Printf("克隆配置: %+v\n", clonedConfig.GetAll())
}

// createExampleConfigFiles 创建示例配置文件
func createExampleConfigFiles() {
	// 创建JSON配置文件
	jsonContent := `{
  "server": {
    "host": "localhost",
    "port": 8080,
    "read_timeout": "30s",
    "write_timeout": "30s",
    "max_conns": 1000
  },
  "database": {
    "driver": "postgres",
    "host": "localhost",
    "port": 5432,
    "username": "user",
    "password": "password",
    "database": "mydb",
    "ssl_mode": "disable"
  },
  "debug": true
}`

	if err := os.WriteFile("example.json", []byte(jsonContent), 0644); err != nil {
		log.Printf("创建JSON配置文件失败: %v", err)
	}

	// 创建YAML配置文件
	yamlContent := `server:
  host: localhost
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  max_conns: 1000

database:
  driver: postgres
  host: localhost
  port: 5432
  username: user
  password: password
  database: mydb
  ssl_mode: disable

redis:
  host: localhost
  port: 6379
  password: ""
  db: 0

log:
  level: info
  format: json
  output: stdout

debug: true
`

	if err := os.WriteFile("example.yaml", []byte(yamlContent), 0644); err != nil {
		log.Printf("创建YAML配置文件失败: %v", err)
	}

	// 创建TOML配置文件
	tomlContent := `debug = true

[server]
host = "localhost"
port = 8080
read_timeout = "30s"
write_timeout = "30s"
max_conns = 1000

[database]
driver = "postgres"
host = "localhost"
port = 5432
username = "user"
password = "password"
database = "mydb"
ssl_mode = "disable"

[redis]
host = "localhost"
port = 6379
password = ""
db = 0

[log]
level = "info"
format = "json"
output = "stdout"
`

	if err := os.WriteFile("example.toml", []byte(tomlContent), 0644); err != nil {
		log.Printf("创建TOML配置文件失败: %v", err)
	}
}
