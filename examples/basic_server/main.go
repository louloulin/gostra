// Package main demonstrates a basic API server setup using Gostra
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/louloulin/gostra/pkg"
	"github.com/louloulin/gostra/pkg/api"
)

func main() {
	// 创建API服务器选项
	options := &api.ServerOptions{
		Host:         "localhost",
		Port:         8080,
		ReadTimeout:  time.Second * 15,
		WriteTimeout: time.Second * 15,
		CorsEnabled:  true,
		CorsOrigins:  []string{"*"},
	}

	// 创建Gostra实例
	gostraInstance := pkg.NewGostra(pkg.DefaultOptions())

	// 创建API服务器
	server := api.NewServer(gostraInstance, options)

	// 添加自定义中间件
	server.AddMiddleware(loggingMiddleware)

	// 启动服务器（在goroutine中运行）
	go func() {
		fmt.Println("Starting API server on", options.Host, "port", options.Port)
		if err := server.Start(); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// 等待终止信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// 优雅地关闭服务器
	fmt.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := server.Stop(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	fmt.Println("Server shutdown complete")
}

// 自定义中间件示例
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("Middleware: %s %s took %v", r.Method, r.URL.Path, time.Since(start))
	})
}
