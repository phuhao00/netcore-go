// HTTP/3 Server Example
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/phuhao00/netcore-go/pkg/http3"
)

func main() {
	// 创建HTTP/3配置
	config := http3.DefaultHTTP3Config()
	config.Host = "localhost"
	config.Port = 8443
	config.CertFile = "server.crt"
	config.KeyFile = "server.key"

	// 创建HTTP/3服务器
	server := http3.NewHTTP3Server(config)

	// 设置HTTP处理器
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "Hello from HTTP/3 server!\nTime: %s\n", time.Now().Format(time.RFC3339))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","protocol":"HTTP/3","time":"%s"}`, time.Now().Format(time.RFC3339))
	})

	server.SetHandler(mux)

	// 启动服务器
	log.Printf("Starting HTTP/3 server on https://%s:%d", config.Host, config.Port)
	log.Println("Note: You need to generate TLS certificates (server.crt and server.key) to run this example")
	log.Println("You can generate self-signed certificates with:")
	log.Println("  openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt -days 365 -nodes")

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start HTTP/3 server: %v", err)
	}

	// 保持服务器运行
	select {}
}
