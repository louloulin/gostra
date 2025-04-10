package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/louloulin/gostra/pkg"
	"github.com/louloulin/gostra/pkg/agent"
)

// Server 表示API服务器
type Server struct {
	router         *mux.Router
	gostra         *pkg.Gostra
	server         *http.Server
	options        *ServerOptions
	pluginRegistry *PluginRegistry
	middlewares    []func(http.Handler) http.Handler // 中间件列表
}

// ServerOptions 包含服务器配置选项
type ServerOptions struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	CorsEnabled  bool
	CorsOrigins  []string
}

// DefaultServerOptions 返回默认服务器选项
func DefaultServerOptions() *ServerOptions {
	return &ServerOptions{
		Host:         "localhost",
		Port:         8080,
		ReadTimeout:  time.Second * 15,
		WriteTimeout: time.Second * 15,
		CorsEnabled:  false,
		CorsOrigins:  []string{"*"},
	}
}

// NewServer 创建一个新的API服务器
func NewServer(gostra *pkg.Gostra, options *ServerOptions) *Server {
	if options == nil {
		options = DefaultServerOptions()
	}

	return &Server{
		router:         mux.NewRouter(),
		gostra:         gostra,
		options:        options,
		pluginRegistry: NewPluginRegistry(),
		middlewares:    []func(http.Handler) http.Handler{},
	}
}

// AddMiddleware adds a middleware function to the server's middleware stack
func (s *Server) AddMiddleware(middleware func(http.Handler) http.Handler) {
	s.middlewares = append(s.middlewares, middleware)
}

// RegisterPlugin registers a new API plugin
func (s *Server) RegisterPlugin(plugin *Plugin) error {
	return s.pluginRegistry.Register(plugin)
}

// Start 启动API服务器
func (s *Server) Start() error {
	// 设置路由
	s.setupRoutes()

	// 注册插件路由
	s.setupPluginRoutes()

	// 创建HTTP服务器
	addr := fmt.Sprintf("%s:%d", s.options.Host, s.options.Port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  s.options.ReadTimeout,
		WriteTimeout: s.options.WriteTimeout,
	}

	// 应用所有中间件
	s.applyMiddlewares()

	// 启动服务器
	log.Printf("Starting API server on %s", addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// applyMiddlewares applies all registered middlewares in order
func (s *Server) applyMiddlewares() {
	// 先应用系统内置中间件
	if s.options.CorsEnabled {
		s.router.Use(s.corsMiddleware)
	}
	s.router.Use(s.loggingMiddleware)

	// 再应用自定义中间件
	for _, middleware := range s.middlewares {
		s.router.Use(middleware)
	}
}

// Stop 停止API服务器
func (s *Server) Stop(ctx context.Context) error {
	log.Println("Stopping API server...")

	// 关闭所有插件
	s.pluginRegistry.Shutdown()

	return s.server.Shutdown(ctx)
}

// setupPluginRoutes registers routes from all plugins
func (s *Server) setupPluginRoutes() {
	// Create a plugin subrouter
	pluginRouter := s.router.PathPrefix("/plugins").Subrouter()

	// Add plugin info endpoint
	pluginRouter.HandleFunc("", s.listPluginsHandler).Methods("GET")

	// Register each plugin's routes
	for _, plugin := range s.pluginRegistry.List() {
		if plugin.RegisterRoutes != nil {
			// Create a subrouter for each plugin
			pluginSubrouter := pluginRouter.PathPrefix("/" + plugin.Name).Subrouter()

			// Apply plugin middleware if provided
			if plugin.Middleware != nil {
				pluginSubrouter.Use(plugin.Middleware)
			}

			// Let the plugin register its routes
			plugin.RegisterRoutes(pluginSubrouter)

			log.Printf("Registered routes for plugin: %s (v%s)", plugin.Name, plugin.Version)
		}
	}
}

// listPluginsHandler returns a list of all registered plugins
func (s *Server) listPluginsHandler(w http.ResponseWriter, r *http.Request) {
	plugins := s.pluginRegistry.List()

	// Convert to a simpler structure for JSON response
	type pluginInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Version     string `json:"version"`
	}

	info := make([]pluginInfo, 0, len(plugins))
	for _, p := range plugins {
		info = append(info, pluginInfo{
			Name:        p.Name,
			Description: p.Description,
			Version:     p.Version,
		})
	}

	sendSuccess(w, "Plugins retrieved successfully", info)
}

// setupRoutes 设置API路由
func (s *Server) setupRoutes() {
	// 健康检查
	s.router.HandleFunc("/health", s.healthHandler).Methods("GET")

	// API版本前缀
	api := s.router.PathPrefix("/api/v1").Subrouter()

	// Agent相关API
	agents := api.PathPrefix("/agents").Subrouter()
	agents.HandleFunc("", s.listAgentsHandler).Methods("GET")
	agents.HandleFunc("/{name}", s.getAgentHandler).Methods("GET")
	agents.HandleFunc("/{name}/run", s.runAgentHandler).Methods("POST")
	agents.HandleFunc("/{name}/stream", s.streamAgentHandler).Methods("POST")

	// Thread相关API
	threads := api.PathPrefix("/threads").Subrouter()
	threads.HandleFunc("", s.createThreadHandler).Methods("POST")
	threads.HandleFunc("", s.listThreadsHandler).Methods("GET")
	threads.HandleFunc("/{threadID}", s.getThreadHandler).Methods("GET")
	threads.HandleFunc("/{threadID}", s.updateThreadHandler).Methods("PUT")
	threads.HandleFunc("/{threadID}", s.deleteThreadHandler).Methods("DELETE")
	threads.HandleFunc("/{threadID}/messages", s.listMessagesHandler).Methods("GET")
	threads.HandleFunc("/{threadID}/messages", s.addMessageHandler).Methods("POST")
	threads.HandleFunc("/{threadID}/messages", s.deleteMessagesHandler).Methods("DELETE")
}

// 中间件
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 设置CORS头
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// 处理预检请求
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.RequestURI, time.Since(start))
	})
}

// 响应结构
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// 工具函数：发送JSON响应
func sendJSON(w http.ResponseWriter, status int, resp interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// 工具函数：发送成功响应
func sendSuccess(w http.ResponseWriter, message string, data interface{}) {
	resp := Response{
		Success: true,
		Message: message,
		Data:    data,
	}
	sendJSON(w, http.StatusOK, resp)
}

// 工具函数：发送错误响应
func sendError(w http.ResponseWriter, status int, message string) {
	resp := Response{
		Success: false,
		Error:   message,
	}
	sendJSON(w, status, resp)
}

// 路由处理函数
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	sendSuccess(w, "API server is healthy", map[string]string{
		"status": "UP",
		"time":   time.Now().String(),
	})
}

func (s *Server) listAgentsHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: 实现列出所有Agent
	sendSuccess(w, "Agents retrieved successfully", []string{})
}

func (s *Server) getAgentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	agent, err := s.gostra.GetAgent(name)
	if err != nil {
		sendError(w, http.StatusNotFound, "Agent not found: "+name)
		return
	}

	sendSuccess(w, "Agent retrieved successfully", agent)
}

func (s *Server) runAgentHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: 实现运行Agent
	sendError(w, http.StatusNotImplemented, "Not implemented yet")
}

func (s *Server) createThreadHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: 实现创建内存线程
	sendError(w, http.StatusNotImplemented, "Not implemented yet")
}

func (s *Server) listThreadsHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: 实现列出内存线程
	sendError(w, http.StatusNotImplemented, "Not implemented yet")
}

func (s *Server) getThreadHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: 实现获取内存线程
	sendError(w, http.StatusNotImplemented, "Not implemented yet")
}

func (s *Server) updateThreadHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: 实现更新内存线程
	sendError(w, http.StatusNotImplemented, "Not implemented yet")
}

func (s *Server) deleteThreadHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: 实现删除内存线程
	sendError(w, http.StatusNotImplemented, "Not implemented yet")
}

func (s *Server) listMessagesHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: 实现列出线程消息
	sendError(w, http.StatusNotImplemented, "Not implemented yet")
}

func (s *Server) addMessageHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: 实现添加线程消息
	sendError(w, http.StatusNotImplemented, "Not implemented yet")
}

func (s *Server) deleteMessagesHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: 实现删除线程消息
	sendError(w, http.StatusNotImplemented, "Not implemented yet")
}

// 流式请求结构
type StreamRequest struct {
	Messages []agent.Message      `json:"messages"`
	Options  *agent.StreamOptions `json:"options,omitempty"`
}

// streamAgentHandler 处理Agent的流式生成请求
func (s *Server) streamAgentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	// 获取Agent
	agentInstance, err := s.gostra.GetAgent(name)
	if err != nil {
		sendError(w, http.StatusNotFound, "Agent not found: "+name)
		return
	}

	// 解析请求体
	var req StreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// 设置SSE响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// 检测客户端断开连接
	go func() {
		<-ctx.Done()
		log.Println("Client disconnected from SSE stream")
	}()

	// 如果没有设置选项，创建默认选项
	if req.Options == nil {
		req.Options = &agent.StreamOptions{
			MaxSteps:    10,
			Temperature: 0.7,
		}
	}

	// 设置上下文
	req.Options.AbortSignal = ctx

	// 调用Agent的流式生成方法
	streamResp, err := agentInstance.Stream(req.Messages, req.Options)
	if err != nil {
		// 发送错误事件
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		w.(http.Flusher).Flush()
		return
	}

	// 读取文本流
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case text, ok := <-streamResp.TextStream:
				if !ok {
					// 文本流关闭
					return
				}
				// 发送文本事件
				fmt.Fprintf(w, "event: text\ndata: %s\n\n", text)
				w.(http.Flusher).Flush()
			}
		}
	}()

	// 读取消息流
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-streamResp.MessageChan:
				if !ok {
					// 消息流关闭
					return
				}
				// 序列化消息并发送
				msgJSON, _ := json.Marshal(msg)
				fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(msgJSON))
				w.(http.Flusher).Flush()
			}
		}
	}()

	// 读取完成信息
	select {
	case <-ctx.Done():
		return
	case finish, ok := <-streamResp.FinishChan:
		if !ok {
			// 流已关闭
			return
		}
		// 序列化完成信息并发送
		finishJSON, _ := json.Marshal(finish)
		fmt.Fprintf(w, "event: finish\ndata: %s\n\n", string(finishJSON))
		w.(http.Flusher).Flush()
	}

	// 发送关闭事件
	fmt.Fprintf(w, "event: close\ndata: stream closed\n\n")
	w.(http.Flusher).Flush()
}
