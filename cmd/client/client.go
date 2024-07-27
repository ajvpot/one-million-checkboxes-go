package main

import (
	"encoding/binary"
	"flag"
	"log"
	"math/rand"
	"time"

	"github.com/gorilla/websocket"
)

const (
	numCheckboxes  = 1000000                // Number of checkboxes
	updateInterval = 100 * time.Millisecond // Interval to update checkbox state
)

func main() {
	// Define the CLI flag for the WebSocket server URL
	serverURL := flag.String("server", "ws://localhost:8080/ws", "WebSocket server URL")
	flag.Parse()

	// Connect to the WebSocket server
	conn, _, err := websocket.DefaultDialer.Dial(*serverURL, nil)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())

	// Update checkbox state at least once per second
	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Generate a random checkbox index and state
			index := rand.Uint32() % numCheckboxes
			state := rand.Intn(2) == 1

			// Create the message to send to the server
			message := make([]byte, 5)
			if state {
				message[0] = 1 // Check
			} else {
				message[0] = 0 // Uncheck
			}
			binary.BigEndian.PutUint32(message[1:], index)

			// Send the message to the server
			err := conn.WriteMessage(websocket.BinaryMessage, message)
			if err != nil {
				log.Printf("Failed to send message: %v", err)
				return
			}

			log.Printf("Updated checkbox %d to %v", index, state)
		}
	}
}
