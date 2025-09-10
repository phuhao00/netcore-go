//go:build ignore
// +build ignore

// Package main protobuf代码生成工具
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	log.Println("Generating protobuf code...")
	
	// 获取当前目录
	currentDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}
	
	// proto文件目录
	protoDir := filepath.Join(currentDir, "proto")
	outputDir := filepath.Join(currentDir, "proto")
	
	// 确保输出目录存在
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}
	
	// 生成的proto文件列表
	protoFiles := []string{
		"user.proto",
		"calculator.proto",
	}
	
	// 生成每个proto文件
	for _, protoFile := range protoFiles {
		protoPath := filepath.Join(protoDir, protoFile)
		
		// 检查proto文件是否存在
		if _, err := os.Stat(protoPath); os.IsNotExist(err) {
			log.Printf("Proto file not found: %s, skipping...", protoPath)
			continue
		}
		
		log.Printf("Generating code for %s...", protoFile)
		
		// 构建protoc命令
		cmd := exec.Command("protoc",
			"--go_out="+outputDir,
			"--go_opt=paths=source_relative",
			"--go-grpc_out="+outputDir,
			"--go-grpc_opt=paths=source_relative",
			"--proto_path="+protoDir,
			protoFile,
		)
		
		// 设置工作目录
		cmd.Dir = currentDir
		
		// 执行命令
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("Failed to generate code for %s: %v", protoFile, err)
			log.Printf("Command output: %s", string(output))
			continue
		}
		
		log.Printf("Successfully generated code for %s", protoFile)
	}
	
	log.Println("\n=== Code Generation Summary ===")
	log.Println("Generated files:")
	
	// 列出生成的文件
	generatedFiles := []string{
		"proto/user.pb.go",
		"proto/user_grpc.pb.go",
		"proto/calculator.pb.go",
		"proto/calculator_grpc.pb.go",
	}
	
	for _, file := range generatedFiles {
		filePath := filepath.Join(currentDir, file)
		if _, err := os.Stat(filePath); err == nil {
			log.Printf("  ✓ %s", file)
		} else {
			log.Printf("  ✗ %s (not found)", file)
		}
	}
	
	log.Println("\n=== Usage Instructions ===")
	log.Println("1. Make sure you have protoc installed:")
	log.Println("   - Download from: https://github.com/protocolbuffers/protobuf/releases")
	log.Println("   - Or install via package manager")
	log.Println("")
	log.Println("2. Install Go protobuf plugins:")
	log.Println("   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest")
	log.Println("   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest")
	log.Println("")
	log.Println("3. Run this generator:")
	log.Println("   go run generate.go")
	log.Println("")
	log.Println("4. Or use the Makefile:")
	log.Println("   make generate")
	
	fmt.Println("\nProtobuf code generation completed!")
}