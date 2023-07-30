package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
)

// WebSocket server handler struct
type WebSocketServer struct {
	targetURL       string
	targetPort      string
	serverPort      string
	useTLS          bool
	privateKeyPath  string
	certificatePath string
}

func (ws *WebSocketServer) handleClient(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade client connection:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	log.Println("Client connected")

	// Connect to the target WebSocket server
	targetURL := fmt.Sprintf("w%s://%s:%s/", ws.getWSSTag(), ws.targetURL, ws.targetPort)
	targetConn, _, err := websocket.DefaultDialer.Dial(targetURL, nil)
	if err != nil {
		log.Println("Failed to connect to target server:", err, targetURL)
		return
	}
	defer targetConn.Close()

	log.Println("Connected to target server")

	// Proxy messages between client and target server
	go ws.proxyMessages(clientConn, targetConn)
	ws.proxyMessages(targetConn, clientConn)

	log.Println("Client disconnected")
}

func (ws *WebSocketServer) proxyMessages(srcConn, dstConn *websocket.Conn) {
	for {
		msgType, msg, err := srcConn.ReadMessage()
		if err != nil {
			break
		}
		err = dstConn.WriteMessage(msgType, msg)
		if err != nil {
			break
		}
	}
}

func (ws *WebSocketServer) getWSSTag() string {
	if ws.useTLS {
		return "s"
	}
	return ""
}

func main() {
	targetURL := os.Getenv("TARGET_URL")
	targetPort := os.Getenv("TARGET_PORT")
	serverPort := os.Getenv("SERVER_PORT")
	privateKeyPath := os.Getenv("PRIVATE_KEY_PATH")
	certificatePath := os.Getenv("CERTIFICATE_PATH")

	useTLS := privateKeyPath != "" && certificatePath != ""

	if targetURL == "" || targetPort == "" || serverPort == "" || (useTLS && (privateKeyPath == "" || certificatePath == "")) {
		fmt.Println("Usage: TARGET_URL=your-target-server-url TARGET_PORT=your-target-server-port SERVER_PORT=9002 [PRIVATE_KEY_PATH=private-key.pem CERTIFICATE_PATH=certificate.pem] go run main.go")
		os.Exit(1)
	}

	ws := &WebSocketServer{
		targetURL:       targetURL,
		targetPort:      targetPort,
		serverPort:      serverPort,
		useTLS:          useTLS,
		privateKeyPath:  privateKeyPath,
		certificatePath: certificatePath,
	}

	http.HandleFunc("/", ws.handleClient)

	// Use a custom http.Server with TLS configuration if TLS is enabled
	if ws.useTLS {
		tlsConfig := &tls.Config{}
		cert, err := tls.LoadX509KeyPair(ws.certificatePath, ws.privateKeyPath)
		if err != nil {
			log.Fatal("Failed to load certificates:", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}

		server := &http.Server{
			Addr:      ":" + ws.serverPort,
			TLSConfig: tlsConfig,
		}

		log.Println("WebSocket proxy server (WSS) listening on port", ws.serverPort)
		log.Fatal(server.ListenAndServeTLS("", ""))
	} else {
		log.Println("WebSocket proxy server (WS) listening on port", ws.serverPort)
		log.Fatal(http.ListenAndServe(":"+ws.serverPort, nil))
	}
}
