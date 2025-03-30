package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocket配置
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有跨域请求
	},
}

// 客户端连接管理
type Client struct {
	conn *websocket.Conn
	send chan []byte
}

var (
	clients   = make(map[*Client]bool)
	clientsMu sync.Mutex
)

// 定义数据结构存储上报信息
type ReportedData struct {
	Events      []Event           `json:"events"`
	RobotStatus map[uint32]Status `json:"status"`
	mu          sync.RWMutex
}

type Event struct {
	RobotID   uint32          `json:"robot_id"`
	RobotName string          `json:"robot_name"`
	Message   json.RawMessage `json:"message"`
	Timestamp time.Time       `json:"timestamp"`
}

type Status struct {
	RobotID   uint32            `json:"robot_id"`
	RobotName string            `json:"robot_name"`
	Data      map[string]string `json:"data"`
	Timestamp time.Time         `json:"timestamp"`
}

var storage = &ReportedData{
	Events:      make([]Event, 0),
	RobotStatus: make(map[uint32]Status),
}

func (rd *ReportedData) AddEvent(event Event) {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	rd.Events = append(rd.Events, event)
	if len(rd.Events) > 200 {
		rd.Events = rd.Events[len(rd.Events)-200:] // 保留最新的200条
	}
}

func main() {
	// 机器人上报接口
	http.HandleFunc("/robot/event", handleRobotEvent)
	http.HandleFunc("/robot/data", handleRobotData)

	// WebSocket接口
	http.HandleFunc("/ws", handleWebSocket)

	// 用户查看接口（保留）
	http.HandleFunc("/user/view", handleUserView)

	// 获取端口号，默认为80
	port := ":80"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = ":" + envPort
	} else {
		flag.StringVar(&port, "port", port, "Port to run the server on")
		flag.Parse()
	}

	// 启动服务器
	if err := http.ListenAndServe(port, nil); err != nil {
		panic(err)
	}
}

// WebSocket处理
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &Client{
		conn: conn,
		send: make(chan []byte, 256),
	}

	// 注册客户端
	clientsMu.Lock()
	clients[client] = true
	clientsMu.Unlock()

	// 发送初始数据
	storage.mu.RLock()
	initialData, _ := json.Marshal(storage)
	storage.mu.RUnlock()
	client.send <- initialData

	// 启动读写协程
	go client.writePump()
	go client.readPump()
}

// 客户端写循环
func (c *Client) writePump() {

	for msg := range c.send {
		err := c.conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			return
		}
	}
}

// 客户端读循环（处理关闭）
func (c *Client) readPump() {
	defer c.close()
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// 关闭客户端连接
func (c *Client) close() {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	c.conn.Close()
	delete(clients, c)
	close(c.send)
}

func broadcastEvent(event Event) {
	storage.mu.RLock()
	data, _ := json.Marshal(event)
	log.Println("Broadcasting event:", string(data))
	storage.mu.RUnlock()

	clientsMu.Lock()
	defer clientsMu.Unlock()

	for client := range clients {
		select {
		case client.send <- data:
		default:
			client.close()
		}
	}
}

func broadcastStatus(status Status) {
	storage.mu.RLock()
	data, _ := json.Marshal(status)
	log.Println("Broadcasting status:", string(data))
	storage.mu.RUnlock()

	clientsMu.Lock()
	defer clientsMu.Unlock()

	for client := range clients {
		select {
		case client.send <- data:
		default:
			client.close()
		}
	}
}

// 处理紧急事件上报
func handleRobotEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		RobotID   uint32          `json:"robot_id"`
		RobotName string          `json:"robot_name"`
		Message   json.RawMessage `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	event := Event{
		RobotID:   request.RobotID,
		RobotName: request.RobotName,
		Message:   request.Message,
		Timestamp: time.Now().UTC(),
	}

	storage.AddEvent(Event{
		RobotID:   request.RobotID,
		RobotName: request.RobotName,
		Message:   request.Message,
		Timestamp: time.Now().UTC(),
	})

	go broadcastEvent(event) // 触发广播
	w.WriteHeader(http.StatusCreated)
}

// 处理数据记录上报
func handleRobotData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request struct {
		RobotID    uint32 `json:"robot_id"`
		RobotName  string `json:"robot_name"`
		StatusType string `json:"status_type"`
		// 这里假设数据是一个字符串，实际应用中可能是更复杂的结构
		Data string `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}
	status := Status{
		RobotID:   request.RobotID,
		RobotName: request.RobotName,
		Data:      make(map[string]string),
		Timestamp: time.Now().UTC(),
	}

	storage.mu.Lock()
	if _, exists := storage.RobotStatus[request.RobotID]; !exists {
		storage.RobotStatus[request.RobotID] = Status{
			RobotID:   request.RobotID,
			RobotName: request.RobotName,
			Data:      make(map[string]string),
			Timestamp: time.Now().UTC(),
		}
	}
	storage.RobotStatus[request.RobotID].Data[request.StatusType] = request.Data
	storage.mu.Unlock()
	status.Data[request.StatusType] = request.Data

	go broadcastStatus(status) // 触发广播
	w.WriteHeader(http.StatusCreated)
}

// 处理用户查看请求
func handleUserView(w http.ResponseWriter, r *http.Request) {
	// 设置CORS头允许跨域访问
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	storage.mu.RLock()
	defer storage.mu.RUnlock()

	// 返回所有存储的数据
	if err := json.NewEncoder(w).Encode(storage); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
