// NetCore-Go CLI Deployment Commands
// Enhanced deployment capabilities
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// DeploymentConfig 部署配置
type DeploymentConfig struct {
	Target      string            `json:"target"`      // docker, k8s, serverless, cloud
	Environment string            `json:"environment"` // dev, staging, prod
	Registry    string            `json:"registry"`    // Docker registry
	Namespace   string            `json:"namespace"`   // Kubernetes namespace
	Region      string            `json:"region"`      // Cloud region
	Variables   map[string]string `json:"variables"`   // Environment variables
	Secrets     map[string]string `json:"secrets"`     // Secrets
	Resources   ResourceLimits    `json:"resources"`   // Resource limits
	Scaling     ScalingConfig     `json:"scaling"`     // Scaling configuration
}

// ResourceLimits 资源限制
type ResourceLimits struct {
	CPURequest    string `json:"cpu_request"`
	CPULimit      string `json:"cpu_limit"`
	MemoryRequest string `json:"memory_request"`
	MemoryLimit   string `json:"memory_limit"`
}

// ScalingConfig 扩缩容配置
type ScalingConfig struct {
	MinReplicas int32 `json:"min_replicas"`
	MaxReplicas int32 `json:"max_replicas"`
	TargetCPU   int32 `json:"target_cpu"`
}

// deploy 命令 - 部署应用
var deployCmd = &cobra.Command{
	Use:   "deploy [target]",
	Short: "Deploy application to various targets",
	Long: `Deploy your NetCore-Go application to various deployment targets.

Supported targets:
  docker      - Build and run Docker container
  k8s         - Deploy to Kubernetes cluster
  compose     - Deploy using Docker Compose
  serverless  - Deploy to serverless platforms
  cloud       - Deploy to cloud providers

Examples:
  netcore-cli deploy docker
  netcore-cli deploy k8s --namespace=production
  netcore-cli deploy compose --env=staging
  netcore-cli deploy serverless --provider=aws
  netcore-cli deploy cloud --provider=gcp --region=us-central1`,
	Args: cobra.ExactArgs(1),
	RunE: runDeploy,
}

func init() {
	// 通用部署标志
	deployCmd.Flags().StringP("environment", "e", "dev", "deployment environment")
	deployCmd.Flags().StringP("config", "c", "", "deployment config file")
	deployCmd.Flags().Bool("dry-run", false, "show what would be deployed")
	deployCmd.Flags().Bool("force", false, "force deployment")
	deployCmd.Flags().Bool("wait", true, "wait for deployment to complete")
	deployCmd.Flags().Duration("timeout", 10*time.Minute, "deployment timeout")
	
	// Docker标志
	deployCmd.Flags().String("registry", "", "Docker registry")
	deployCmd.Flags().String("tag", "latest", "Docker image tag")
	deployCmd.Flags().Bool("push", false, "push image to registry")
	deployCmd.Flags().Bool("no-cache", false, "build without cache")
	
	// Kubernetes标志
	deployCmd.Flags().String("namespace", "default", "Kubernetes namespace")
	deployCmd.Flags().String("context", "", "Kubernetes context")
	deployCmd.Flags().String("kubeconfig", "", "path to kubeconfig file")
	deployCmd.Flags().Int32("replicas", 3, "number of replicas")
	
	// 云平台标志
	deployCmd.Flags().String("provider", "", "cloud provider (aws, gcp, azure)")
	deployCmd.Flags().String("region", "", "cloud region")
	deployCmd.Flags().String("project", "", "cloud project ID")
	
	// 资源标志
	deployCmd.Flags().String("cpu-request", "100m", "CPU request")
	deployCmd.Flags().String("cpu-limit", "500m", "CPU limit")
	deployCmd.Flags().String("memory-request", "128Mi", "memory request")
	deployCmd.Flags().String("memory-limit", "512Mi", "memory limit")
	
	// 扩缩容标志
	deployCmd.Flags().Int32("min-replicas", 1, "minimum replicas")
	deployCmd.Flags().Int32("max-replicas", 10, "maximum replicas")
	deployCmd.Flags().Int32("target-cpu", 70, "target CPU utilization")
}

func runDeploy(cmd *cobra.Command, args []string) error {
	target := args[0]
	
	// 获取标志值
	environment, _ := cmd.Flags().GetString("environment")
	configFile, _ := cmd.Flags().GetString("config")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	force, _ := cmd.Flags().GetBool("force")
	wait, _ := cmd.Flags().GetBool("wait")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	
	// 创建部署配置
	deployConfig := &DeploymentConfig{
		Target:      target,
		Environment: environment,
		Variables:   make(map[string]string),
		Secrets:     make(map[string]string),
	}
	
	// 设置部署配置
	if err := setDeploymentConfig(cmd, deployConfig); err != nil {
		return fmt.Errorf("failed to set deployment config: %w", err)
	}
	
	// 加载配置文件
	if configFile != "" {
		if err := loadDeploymentConfig(configFile, deployConfig); err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
	}
	
	// 验证部署前提条件
	if !force {
		if err := validateDeploymentPrerequisites(target, deployConfig); err != nil {
			return fmt.Errorf("deployment prerequisites not met: %w", err)
		}
	}
	
	// 执行部署
	switch target {
	case "docker":
		return deployDocker(deployConfig, dryRun, wait, timeout)
	case "k8s", "kubernetes":
		return deployKubernetes(deployConfig, dryRun, wait, timeout)
	case "compose":
		return deployCompose(deployConfig, dryRun, wait, timeout)
	case "serverless":
		return deployServerless(deployConfig, dryRun, wait, timeout)
	case "cloud":
		return deployCloud(deployConfig, dryRun, wait, timeout)
	default:
		return fmt.Errorf("unsupported deployment target: %s", target)
	}
}

// setDeploymentConfig 设置部署配置
func setDeploymentConfig(cmd *cobra.Command, config *DeploymentConfig) error {
	if registry, _ := cmd.Flags().GetString("registry"); registry != "" {
		config.Registry = registry
	}
	
	if namespace, _ := cmd.Flags().GetString("namespace"); namespace != "" {
		config.Namespace = namespace
	}
	
	if region, _ := cmd.Flags().GetString("region"); region != "" {
		config.Region = region
	}
	
	// 设置资源限制
	cpuRequest, _ := cmd.Flags().GetString("cpu-request")
	cpuLimit, _ := cmd.Flags().GetString("cpu-limit")
	memoryRequest, _ := cmd.Flags().GetString("memory-request")
	memoryLimit, _ := cmd.Flags().GetString("memory-limit")
	
	config.Resources = ResourceLimits{
		CPURequest:    cpuRequest,
		CPULimit:      cpuLimit,
		MemoryRequest: memoryRequest,
		MemoryLimit:   memoryLimit,
	}
	
	// 设置扩缩容配置
	minReplicas, _ := cmd.Flags().GetInt32("min-replicas")
	maxReplicas, _ := cmd.Flags().GetInt32("max-replicas")
	targetCPU, _ := cmd.Flags().GetInt32("target-cpu")
	
	config.Scaling = ScalingConfig{
		MinReplicas: minReplicas,
		MaxReplicas: maxReplicas,
		TargetCPU:   targetCPU,
	}
	
	return nil
}

// loadDeploymentConfig 加载部署配置文件
func loadDeploymentConfig(configFile string, config *DeploymentConfig) error {
	fmt.Printf("Loading deployment config from: %s\n", configFile)
	
	// 检查配置文件是否存在
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("deployment config file not found: %s", configFile)
	}
	
	// 读取配置文件
	configData, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	
	// 根据文件扩展名选择解析方式
	ext := filepath.Ext(configFile)
	switch ext {
	case ".json":
		if err := json.Unmarshal(configData, config); err != nil {
			return fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(configData, config); err != nil {
			return fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".toml":
		if err := toml.Unmarshal(configData, config); err != nil {
			return fmt.Errorf("failed to parse TOML config: %w", err)
		}
	default:
		// 默认尝试JSON格式
		if err := json.Unmarshal(configData, config); err != nil {
			return fmt.Errorf("failed to parse config file (unknown format): %w", err)
		}
	}
	
	// 验证必要字段
	if config.Environment == "" {
		config.Environment = "production"
	}
	
	if config.Namespace == "" {
		config.Namespace = "default"
	}
	
	if config.Variables == nil {
		config.Variables = make(map[string]string)
	}
	
	fmt.Printf("✅ Loaded deployment config for environment: %s\n", config.Environment)
	return nil
}

// validateDeploymentPrerequisites 验证部署前提条件
func validateDeploymentPrerequisites(target string, config *DeploymentConfig) error {
	switch target {
	case "docker":
		return validateDockerPrerequisites()
	case "k8s", "kubernetes":
		return validateKubernetesPrerequisites(config)
	case "compose":
		return validateComposePrerequisites()
	case "serverless":
		return validateServerlessPrerequisites(config)
	case "cloud":
		return validateCloudPrerequisites(config)
	default:
		return nil
	}
}

// validateDockerPrerequisites 验证Docker前提条件
func validateDockerPrerequisites() error {
	// 检查Docker是否安装
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker is not installed or not in PATH")
	}
	
	// 检查Docker是否运行
	cmd := exec.Command("docker", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker is not running")
	}
	
	return nil
}

// validateKubernetesPrerequisites 验证Kubernetes前提条件
func validateKubernetesPrerequisites(config *DeploymentConfig) error {
	// 检查kubectl是否安装
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl is not installed or not in PATH")
	}
	
	// 检查集群连接
	cmd := exec.Command("kubectl", "cluster-info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cannot connect to Kubernetes cluster")
	}
	
	return nil
}

// validateComposePrerequisites 验证Docker Compose前提条件
func validateComposePrerequisites() error {
	// 检查docker-compose是否安装
	if _, err := exec.LookPath("docker-compose"); err != nil {
		// 尝试docker compose (新版本)
		if _, err := exec.LookPath("docker"); err != nil {
			return fmt.Errorf("docker-compose is not installed")
		}
		cmd := exec.Command("docker", "compose", "version")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("docker compose is not available")
		}
	}
	
	return nil
}

// validateServerlessPrerequisites 验证Serverless前提条件
func validateServerlessPrerequisites(config *DeploymentConfig) error {
	provider := config.Variables["provider"]
	if provider == "" {
		provider = "aws" // 默认AWS
	}
	
	switch provider {
	case "aws":
		// 检查AWS CLI
		if _, err := exec.LookPath("aws"); err != nil {
			return fmt.Errorf("AWS CLI is not installed or not in PATH")
		}
		// 检查SAM CLI
		if _, err := exec.LookPath("sam"); err != nil {
			fmt.Println("⚠️  SAM CLI not found, using AWS CLI for deployment")
		}
	case "gcp":
		// 检查gcloud CLI
		if _, err := exec.LookPath("gcloud"); err != nil {
			return fmt.Errorf("gcloud CLI is not installed or not in PATH")
		}
	case "azure":
		// 检查Azure CLI
		if _, err := exec.LookPath("az"); err != nil {
			return fmt.Errorf("Azure CLI is not installed or not in PATH")
		}
	default:
		return fmt.Errorf("unsupported serverless provider: %s", provider)
	}
	
	return nil
}

// validateCloudPrerequisites 验证云平台前提条件
func validateCloudPrerequisites(config *DeploymentConfig) error {
	provider := config.Variables["provider"]
	if provider == "" {
		provider = "aws" // 默认AWS
	}
	
	switch provider {
	case "aws":
		// 检查AWS CLI
		if _, err := exec.LookPath("aws"); err != nil {
			return fmt.Errorf("AWS CLI is not installed or not in PATH")
		}
		// 检查AWS凭证
		cmd := exec.Command("aws", "sts", "get-caller-identity")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("AWS credentials not configured")
		}
	case "gcp":
		// 检查gcloud CLI
		if _, err := exec.LookPath("gcloud"); err != nil {
			return fmt.Errorf("gcloud CLI is not installed or not in PATH")
		}
		// 检查GCP认证
		cmd := exec.Command("gcloud", "auth", "list", "--filter=status:ACTIVE", "--format=value(account)")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("GCP authentication not configured")
		}
	case "azure":
		// 检查Azure CLI
		if _, err := exec.LookPath("az"); err != nil {
			return fmt.Errorf("Azure CLI is not installed or not in PATH")
		}
		// 检查Azure登录状态
		cmd := exec.Command("az", "account", "show")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("Azure authentication not configured")
		}
	default:
		return fmt.Errorf("unsupported cloud provider: %s", provider)
	}
	
	return nil
}

// deployDocker 部署到Docker
func deployDocker(config *DeploymentConfig, dryRun, wait bool, timeout time.Duration) error {
	fmt.Printf("🐳 Deploying to Docker (environment: %s)\n", config.Environment)
	
	if dryRun {
		fmt.Println("Dry run mode - showing what would be executed:")
		fmt.Println("1. docker build -t app:latest .")
		fmt.Println("2. docker run -d -p 8080:8080 app:latest")
		return nil
	}
	
	// 构建Docker镜像
	fmt.Println("📦 Building Docker image...")
	buildCmd := exec.Command("docker", "build", "-t", "app:latest", ".")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build Docker image: %w", err)
	}
	
	// 停止现有容器
	fmt.Println("🛑 Stopping existing containers...")
	stopCmd := exec.Command("docker", "stop", "netcore-app")
	stopCmd.Run() // 忽略错误，容器可能不存在
	
	removeCmd := exec.Command("docker", "rm", "netcore-app")
	removeCmd.Run() // 忽略错误
	
	// 运行新容器
	fmt.Println("🚀 Starting new container...")
	runArgs := []string{"run", "-d", "--name", "netcore-app", "-p", "8080:8080"}
	
	// 添加环境变量
	for key, value := range config.Variables {
		runArgs = append(runArgs, "-e", fmt.Sprintf("%s=%s", key, value))
	}
	
	runArgs = append(runArgs, "app:latest")
	
	runCmd := exec.Command("docker", runArgs...)
	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("failed to run Docker container: %w", err)
	}
	
	fmt.Println("✅ Docker deployment completed successfully")
	fmt.Println("🌐 Application available at: http://localhost:8080")
	
	return nil
}

// deployKubernetes 部署到Kubernetes
func deployKubernetes(config *DeploymentConfig, dryRun, wait bool, timeout time.Duration) error {
	fmt.Printf("☸️  Deploying to Kubernetes (namespace: %s, environment: %s)\n", config.Namespace, config.Environment)
	
	if dryRun {
		fmt.Println("Dry run mode - showing what would be executed:")
		fmt.Printf("1. kubectl apply -f deployments/k8s/ -n %s\n", config.Namespace)
		fmt.Printf("2. kubectl rollout status deployment/app -n %s\n", config.Namespace)
		return nil
	}
	
	// 创建命名空间（如果不存在）
	if config.Namespace != "default" {
		fmt.Printf("📁 Creating namespace: %s\n", config.Namespace)
		nsCmd := exec.Command("kubectl", "create", "namespace", config.Namespace, "--dry-run=client", "-o", "yaml")
		nsCmd.Run() // 忽略错误，命名空间可能已存在
	}
	
	// 应用Kubernetes清单
	fmt.Println("📋 Applying Kubernetes manifests...")
	applyCmd := exec.Command("kubectl", "apply", "-f", "deployments/k8s/", "-n", config.Namespace)
	applyCmd.Stdout = os.Stdout
	applyCmd.Stderr = os.Stderr
	
	if err := applyCmd.Run(); err != nil {
		return fmt.Errorf("failed to apply Kubernetes manifests: %w", err)
	}
	
	// 等待部署完成
	if wait {
		fmt.Println("⏳ Waiting for deployment to complete...")
		rolloutCmd := exec.Command("kubectl", "rollout", "status", "deployment/app", "-n", config.Namespace, fmt.Sprintf("--timeout=%s", timeout))
		rolloutCmd.Stdout = os.Stdout
		rolloutCmd.Stderr = os.Stderr
		
		if err := rolloutCmd.Run(); err != nil {
			return fmt.Errorf("deployment rollout failed: %w", err)
		}
	}
	
	fmt.Println("✅ Kubernetes deployment completed successfully")
	
	// 显示服务信息
	showKubernetesServiceInfo(config.Namespace)
	
	return nil
}

// deployCompose 使用Docker Compose部署
func deployCompose(config *DeploymentConfig, dryRun, wait bool, timeout time.Duration) error {
	fmt.Printf("🐙 Deploying with Docker Compose (environment: %s)\n", config.Environment)
	
	if dryRun {
		fmt.Println("Dry run mode - showing what would be executed:")
		fmt.Println("1. docker-compose down")
		fmt.Println("2. docker-compose up -d --build")
		return nil
	}
	
	// 停止现有服务
	fmt.Println("🛑 Stopping existing services...")
	downCmd := exec.Command("docker-compose", "down")
	downCmd.Run() // 忽略错误
	
	// 启动服务
	fmt.Println("🚀 Starting services...")
	upCmd := exec.Command("docker-compose", "up", "-d", "--build")
	upCmd.Stdout = os.Stdout
	upCmd.Stderr = os.Stderr
	
	// 设置环境变量
	for key, value := range config.Variables {
		upCmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", key, value))
	}
	
	if err := upCmd.Run(); err != nil {
		return fmt.Errorf("failed to start services with docker-compose: %w", err)
	}
	
	fmt.Println("✅ Docker Compose deployment completed successfully")
	
	return nil
}

// deployServerless 部署到Serverless平台
func deployServerless(config *DeploymentConfig, dryRun, wait bool, timeout time.Duration) error {
	fmt.Printf("⚡ Deploying to Serverless (environment: %s)\n", config.Environment)
	
	if dryRun {
		fmt.Println("Dry run mode - serverless deployment would be executed")
		return nil
	}
	
	provider := config.Variables["provider"]
	if provider == "" {
		provider = "aws"
	}
	
	switch provider {
	case "aws":
		return deployAWSServerless(config, dryRun, wait, timeout)
	case "gcp":
		return deployGCPServerless(config, dryRun, wait, timeout)
	case "azure":
		return deployAzureServerless(config, dryRun, wait, timeout)
	default:
		return fmt.Errorf("unsupported serverless provider: %s", provider)
	}
}

// deployCloud 部署到云平台
func deployCloud(config *DeploymentConfig, dryRun, wait bool, timeout time.Duration) error {
	fmt.Printf("☁️  Deploying to Cloud (environment: %s)\n", config.Environment)
	
	if dryRun {
		fmt.Println("Dry run mode - cloud deployment would be executed")
		return nil
	}
	
	provider := config.Variables["provider"]
	if provider == "" {
		provider = "aws"
	}
	
	switch provider {
	case "aws":
		return deployAWSCloud(config, dryRun, wait, timeout)
	case "gcp":
		return deployGCPCloud(config, dryRun, wait, timeout)
	case "azure":
		return deployAzureCloud(config, dryRun, wait, timeout)
	default:
		return fmt.Errorf("unsupported cloud provider: %s", provider)
	}
}

// showKubernetesServiceInfo 显示Kubernetes服务信息
func showKubernetesServiceInfo(namespace string) {
	fmt.Println("\n📊 Service Information:")
	
	// 获取服务信息
	svcCmd := exec.Command("kubectl", "get", "svc", "-n", namespace)
	svcCmd.Stdout = os.Stdout
	svcCmd.Stderr = os.Stderr
	svcCmd.Run()
	
	// 获取Pod信息
	fmt.Println("\n📦 Pod Information:")
	podCmd := exec.Command("kubectl", "get", "pods", "-n", namespace)
	podCmd.Stdout = os.Stdout
	podCmd.Stderr = os.Stderr
	podCmd.Run()
}

// status 命令 - 查看部署状态
var statusCmd = &cobra.Command{
	Use:   "status [target]",
	Short: "Check deployment status",
	Long:  `Check the status of deployed applications across different targets.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().String("namespace", "default", "Kubernetes namespace")
	statusCmd.Flags().Bool("watch", false, "watch status changes")
	deployCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	target := "all"
	if len(args) > 0 {
		target = args[0]
	}
	
	namespace, _ := cmd.Flags().GetString("namespace")
	watch, _ := cmd.Flags().GetBool("watch")
	
	switch target {
	case "docker":
		return showDockerStatus()
	case "k8s", "kubernetes":
		return showKubernetesStatus(namespace, watch)
	case "compose":
		return showComposeStatus()
	case "all":
		return showAllStatus(namespace)
	default:
		return fmt.Errorf("unsupported status target: %s", target)
	}
}

// showDockerStatus 显示Docker状态
func showDockerStatus() error {
	fmt.Println("🐳 Docker Status:")
	cmd := exec.Command("docker", "ps", "--filter", "name=netcore")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// showKubernetesStatus 显示Kubernetes状态
func showKubernetesStatus(namespace string, watch bool) error {
	fmt.Printf("☸️  Kubernetes Status (namespace: %s):\n", namespace)
	
	args := []string{"get", "all", "-n", namespace}
	if watch {
		args = append(args, "--watch")
	}
	
	cmd := exec.Command("kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// showComposeStatus 显示Docker Compose状态
func showComposeStatus() error {
	fmt.Println("🐙 Docker Compose Status:")
	cmd := exec.Command("docker-compose", "ps")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// showAllStatus 显示所有状态
func showAllStatus(namespace string) error {
	fmt.Println("📊 All Deployment Status:\n")
	
	// Docker状态
	if err := showDockerStatus(); err == nil {
		fmt.Println()
	}
	
	// Kubernetes状态
	if err := showKubernetesStatus(namespace, false); err == nil {
		fmt.Println()
	}
	
	// Docker Compose状态
	if err := showComposeStatus(); err == nil {
		fmt.Println()
	}
	
	return nil
}

// logs 命令 - 查看部署日志
var logsCmd = &cobra.Command{
	Use:   "logs [target]",
	Short: "View deployment logs",
	Long:  `View logs from deployed applications.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runLogs,
}

func init() {
	logsCmd.Flags().String("namespace", "default", "Kubernetes namespace")
	logsCmd.Flags().Bool("follow", false, "follow log output")
	logsCmd.Flags().String("tail", "100", "number of lines to show")
	deployCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	target := "k8s"
	if len(args) > 0 {
		target = args[0]
	}
	
	namespace, _ := cmd.Flags().GetString("namespace")
	follow, _ := cmd.Flags().GetBool("follow")
	tail, _ := cmd.Flags().GetString("tail")
	
	switch target {
	case "docker":
		return showDockerLogs(follow, tail)
	case "k8s", "kubernetes":
		return showKubernetesLogs(namespace, follow, tail)
	case "compose":
		return showComposeLogs(follow, tail)
	default:
		return fmt.Errorf("unsupported logs target: %s", target)
	}
}

// showDockerLogs 显示Docker日志
func showDockerLogs(follow bool, tail string) error {
	args := []string{"logs", "--tail", tail}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, "netcore-app")
	
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// showKubernetesLogs 显示Kubernetes日志
func showKubernetesLogs(namespace string, follow bool, tail string) error {
	args := []string{"logs", "-l", "app=netcore-app", "-n", namespace, "--tail", tail}
	if follow {
		args = append(args, "-f")
	}
	
	cmd := exec.Command("kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// showComposeLogs 显示Docker Compose日志
func showComposeLogs(follow bool, tail string) error {
	args := []string{"logs", "--tail", tail}
	if follow {
		args = append(args, "-f")
	}
	
	cmd := exec.Command("docker-compose", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// rollback 命令 - 回滚部署
var rollbackCmd = &cobra.Command{
	Use:   "rollback [target]",
	Short: "Rollback deployment",
	Long:  `Rollback to previous deployment version.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runRollback,
}

func init() {
	rollbackCmd.Flags().String("namespace", "default", "Kubernetes namespace")
	rollbackCmd.Flags().String("revision", "", "specific revision to rollback to")
	deployCmd.AddCommand(rollbackCmd)
}

func runRollback(cmd *cobra.Command, args []string) error {
	target := "k8s"
	if len(args) > 0 {
		target = args[0]
	}
	
	namespace, _ := cmd.Flags().GetString("namespace")
	revision, _ := cmd.Flags().GetString("revision")
	
	// 确认回滚操作
	fmt.Printf("⚠️  Are you sure you want to rollback %s deployment? (y/N): ", target)
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	
	if response != "y" && response != "yes" {
		fmt.Println("Rollback cancelled")
		return nil
	}
	
	switch target {
	case "k8s", "kubernetes":
		return rollbackKubernetes(namespace, revision)
	default:
		return fmt.Errorf("rollback not supported for target: %s", target)
	}
}

// rollbackKubernetes 回滚Kubernetes部署
func rollbackKubernetes(namespace, revision string) error {
	fmt.Printf("🔄 Rolling back Kubernetes deployment in namespace: %s\n", namespace)
	
	args := []string{"rollout", "undo", "deployment/app", "-n", namespace}
	if revision != "" {
		args = append(args, fmt.Sprintf("--to-revision=%s", revision))
	}
	
	cmd := exec.Command("kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}
	
	fmt.Println("✅ Rollback completed successfully")
	return nil
}

// AWS Serverless部署函数
func deployAWSServerless(config *DeploymentConfig, dryRun, wait bool, timeout time.Duration) error {
	fmt.Println("🚀 Deploying to AWS Lambda...")
	
	if dryRun {
		fmt.Println("Dry run mode - AWS Lambda deployment would be executed")
		return nil
	}
	
	// 检查是否有SAM模板
	if _, err := os.Stat("template.yaml"); err == nil {
		// 使用SAM部署
		cmd := exec.Command("sam", "deploy", "--guided")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	
	// 使用AWS CLI部署
	fmt.Println("📦 Building and deploying Lambda function...")
	cmd := exec.Command("aws", "lambda", "update-function-code", "--function-name", "netcore-app", "--zip-file", "fileb://function.zip")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func deployGCPServerless(config *DeploymentConfig, dryRun, wait bool, timeout time.Duration) error {
	fmt.Println("🚀 Deploying to Google Cloud Functions...")
	
	if dryRun {
		fmt.Println("Dry run mode - GCP Cloud Functions deployment would be executed")
		return nil
	}
	
	cmd := exec.Command("gcloud", "functions", "deploy", "netcore-app", "--runtime", "go121", "--trigger-http", "--allow-unauthenticated")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func deployAzureServerless(config *DeploymentConfig, dryRun, wait bool, timeout time.Duration) error {
	fmt.Println("🚀 Deploying to Azure Functions...")
	
	if dryRun {
		fmt.Println("Dry run mode - Azure Functions deployment would be executed")
		return nil
	}
	
	cmd := exec.Command("az", "functionapp", "deployment", "source", "config-zip", "--resource-group", "netcore-rg", "--name", "netcore-app", "--src", "function.zip")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// AWS Cloud部署函数
func deployAWSCloud(config *DeploymentConfig, dryRun, wait bool, timeout time.Duration) error {
	fmt.Println("☁️  Deploying to AWS ECS/EKS...")
	
	if dryRun {
		fmt.Println("Dry run mode - AWS cloud deployment would be executed")
		return nil
	}
	
	// 检查是否有EKS集群配置
	if config.Variables["cluster_name"] != "" {
		// 部署到EKS
		cmd := exec.Command("aws", "eks", "update-kubeconfig", "--name", config.Variables["cluster_name"])
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to update kubeconfig: %w", err)
		}
		return deployKubernetes(config, dryRun, wait, timeout)
	}
	
	// 部署到ECS
	fmt.Println("📦 Deploying to AWS ECS...")
	cmd := exec.Command("aws", "ecs", "update-service", "--cluster", "netcore-cluster", "--service", "netcore-service", "--force-new-deployment")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func deployGCPCloud(config *DeploymentConfig, dryRun, wait bool, timeout time.Duration) error {
	fmt.Println("☁️  Deploying to Google Cloud Run...")
	
	if dryRun {
		fmt.Println("Dry run mode - GCP cloud deployment would be executed")
		return nil
	}
	
	cmd := exec.Command("gcloud", "run", "deploy", "netcore-app", "--image", "gcr.io/PROJECT_ID/netcore-app", "--platform", "managed", "--region", "us-central1", "--allow-unauthenticated")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func deployAzureCloud(config *DeploymentConfig, dryRun, wait bool, timeout time.Duration) error {
	fmt.Println("☁️  Deploying to Azure Container Instances...")
	
	if dryRun {
		fmt.Println("Dry run mode - Azure cloud deployment would be executed")
		return nil
	}
	
	cmd := exec.Command("az", "container", "create", "--resource-group", "netcore-rg", "--name", "netcore-app", "--image", "netcore-app:latest", "--dns-name-label", "netcore-app", "--ports", "8080")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}