package main

import (
	"context"
	"encoding/binary"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/ajvpot/one-million-checkboxes-go/pkg/network"
	"github.com/ajvpot/one-million-checkboxes-go/pkg/state"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	persistencePeriod = 5 * time.Minute // Period to persist state to file
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	manager = network.NewWebSocketManager()

	masterConn      *websocket.Conn
	masterConnMutex sync.Mutex

	// Prometheus metrics
	totalConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "total_connections",
		Help: "Total number of active WebSocket connections",
	})
	totalMessages = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "total_messages",
		Help: "Total number of messages received",
	})
)

func init() {
	// Register Prometheus metrics
	prometheus.MustRegister(totalConnections)
	prometheus.MustRegister(totalMessages)
}

var mode *string
var masterServerURL *string

func main() {
	mode = flag.String("mode", "master", "Mode to run the server: master or relayer")
	masterServerURL = flag.String("masterServerURL", "ws://localhost:8080/ws", "Address of the master server")
	port := flag.String("port", ":8080", "Port to run the server on")
	flag.Parse()

	if *mode == "master" {
		state.LoadStateFromFile()
		go persistStatePeriodically()
	}

	http.HandleFunc("/ws", handleWebSocket)
	http.Handle("/metrics", promhttp.Handler())

	server := &http.Server{Addr: *port}

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	go func() {
		<-stop
		log.Println("Shutting down server...")

		if masterConn != nil {
			masterConn.Close()
		}

		if *mode == "master" {
			state.SaveStateToFile()
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Fatalf("Server forced to shutdown: %v", err)
		}

		log.Println("Server gracefully stopped")
		os.Exit(0)
	}()

	if *mode == "relayer" {
		go connectToMaster()
	}

	log.Printf("%s server started on %s", *mode, *port)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("ListenAndServe(): %v", err)
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := network.UpgradeConnection(w, r, upgrader)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	// Track connection
	manager.AddConnection(conn)
	totalConnections.Inc()

	defer func() {
		// Remove connection on exit
		manager.RemoveConnection(conn)
		totalConnections.Dec()
	}()

	// Send the full state to the new connection if in master mode
	if *mode == "master" {
		if err := sendFullState(conn); err != nil {
			log.Println("Error sending full state:", err)
			return
		}
	}

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}

		totalMessages.Inc()

		if len(message) < 5 {
			log.Println("Invalid message length")
			continue
		}

		action := message[0]
		index := binary.BigEndian.Uint32(message[1:5])

		if index >= state.NumCheckboxes {
			log.Println("Invalid checkbox index")
			continue
		}

		switch action {
		case 0: // Uncheck
			state.UpdateCheckbox(index, false)
		case 1: // Check
			state.UpdateCheckbox(index, true)
		default:
			log.Println("Invalid action")
		}

		// Broadcast the update to all connected clients
		manager.BroadcastMessage(message)

		// Relay the message to the master server if in relayer mode
		if *mode == "relayer" {
			relayToMaster(message)
		}
	}
}

func sendFullState(conn *websocket.Conn) error {
	stateData := make([]byte, (state.NumCheckboxes+31)/32*4)
	for i := 0; i < state.NumCheckboxes; i++ {
		if state.GetCheckboxState(uint32(i)) {
			stateData[(i/32)*4] |= 1 << (31 - (i % 32))
		}
	}
	return conn.WriteMessage(websocket.BinaryMessage, stateData)
}

func connectToMaster() {
	for {
		conn, _, err := websocket.DefaultDialer.Dial(*masterServerURL, nil)
		if err != nil {
			log.Fatal("Dial error:", err)
		}

		masterConnMutex.Lock()
		masterConn = conn
		masterConnMutex.Unlock()

		defer func() {
			masterConnMutex.Lock()
			if masterConn != nil {
				masterConn.Close()
				masterConn = nil
			}
			masterConnMutex.Unlock()
		}()

		// Handle initial state dump from master
		_, initialState, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error reading initial state from master:", err)
			continue
		}
		updateLocalState(initialState)

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("Read error from master:", err)
				break
			}

			// Update the local state based on the message from the master server
			if len(message) >= 5 {
				action := message[0]
				index := binary.BigEndian.Uint32(message[1:5])

				if index < state.NumCheckboxes {
					switch action {
					case 0: // Uncheck
						state.UpdateCheckbox(index, false)
					case 1: // Check
						state.UpdateCheckbox(index, true)
					}
				}
			}

			// Relay the message to all connected clients
			manager.BroadcastMessage(message)
		}
	}
}

func updateLocalState(initialState []byte) {
	for i := 0; i < state.NumCheckboxes; i++ {
		if initialState[(i/32)*4]&(1<<(31-(i%32))) != 0 {
			state.UpdateCheckbox(uint32(i), true)
		} else {
			state.UpdateCheckbox(uint32(i), false)
		}
	}
	log.Print("loaded state from master")
}

func relayToMaster(message []byte) {
	masterConnMutex.Lock()
	defer masterConnMutex.Unlock()

	if masterConn == nil {
		log.Println("Master connection is not available")
		return
	}

	err := masterConn.WriteMessage(websocket.BinaryMessage, message)
	if err != nil {
		log.Println("Write error to master:", err)
		masterConn.Close()
		masterConn = nil
	}
}

func persistStatePeriodically() {
	for {
		time.Sleep(persistencePeriod)
		state.SaveStateToFile()
	}
}
