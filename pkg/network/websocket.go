package network

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type WebSocketManager struct {
	Connections map[*websocket.Conn]bool
	Mutex       sync.Mutex
}

func NewWebSocketManager() *WebSocketManager {
	return &WebSocketManager{
		Connections: make(map[*websocket.Conn]bool),
	}
}

func (manager *WebSocketManager) AddConnection(conn *websocket.Conn) {
	manager.Mutex.Lock()
	defer manager.Mutex.Unlock()
	manager.Connections[conn] = true
}

func (manager *WebSocketManager) RemoveConnection(conn *websocket.Conn) {
	manager.Mutex.Lock()
	defer manager.Mutex.Unlock()
	delete(manager.Connections, conn)
}

func (manager *WebSocketManager) BroadcastMessage(message []byte) {
	manager.Mutex.Lock()
	defer manager.Mutex.Unlock()

	for conn := range manager.Connections {
		err := conn.WriteMessage(websocket.BinaryMessage, message)
		if err != nil {
			log.Println("Write error:", err)
			conn.Close()
			delete(manager.Connections, conn)
		}
	}
}

func UpgradeConnection(w http.ResponseWriter, r *http.Request, upgrader websocket.Upgrader) (*websocket.Conn, error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
