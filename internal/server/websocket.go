package server

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Simple WebSocket implementation without external dependencies

const (
	wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
)

type wsConn struct {
	conn   net.Conn
	closed bool
	mu     sync.Mutex
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade to WebSocket
	conn, err := upgradeWebSocket(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ws := &wsConn{conn: conn}

	// Track active client
	s.mu.Lock()
	s.activeClients++
	s.lastActivity = time.Now()
	s.mu.Unlock()

	defer func() {
		ws.close()
		s.mu.Lock()
		s.activeClients--
		s.mu.Unlock()
	}()

	// Send initial connected message
	ws.sendText(`{"type":"connected"}`)

	// Read messages (for ping/pong and activity)
	reader := bufio.NewReader(conn)
	for {
		msg, err := readWSMessage(reader)
		if err != nil {
			break
		}

		s.recordActivity()

		// Handle ping message
		if strings.Contains(string(msg), `"type":"ping"`) {
			ws.sendText(`{"type":"pong"}`)
		}

		// Handle tab visibility
		if strings.Contains(string(msg), `"tab_active":true`) {
			s.setTabActive(true)
		} else if strings.Contains(string(msg), `"tab_active":false`) {
			s.setTabActive(false)
		}
	}
}

func upgradeWebSocket(w http.ResponseWriter, r *http.Request) (net.Conn, error) {
	if r.Header.Get("Upgrade") != "websocket" {
		return nil, fmt.Errorf("not a websocket request")
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, fmt.Errorf("missing Sec-WebSocket-Key")
	}

	// Calculate accept key
	h := sha1.New()
	h.Write([]byte(key + wsGUID))
	acceptKey := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Hijack connection
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("hijacking not supported")
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}

	// Send upgrade response
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey + "\r\n\r\n"

	bufrw.WriteString(response)
	bufrw.Flush()

	return conn, nil
}

func (ws *wsConn) sendText(msg string) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.closed {
		return fmt.Errorf("connection closed")
	}

	data := []byte(msg)
	frame := make([]byte, 0, 2+len(data))

	// Text frame, FIN bit set
	frame = append(frame, 0x81)

	// Length
	if len(data) < 126 {
		frame = append(frame, byte(len(data)))
	} else if len(data) < 65536 {
		frame = append(frame, 126)
		frame = append(frame, byte(len(data)>>8), byte(len(data)))
	} else {
		frame = append(frame, 127)
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(len(data)))
		frame = append(frame, b...)
	}

	frame = append(frame, data...)

	_, err := ws.conn.Write(frame)
	return err
}

func (ws *wsConn) close() {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if !ws.closed {
		ws.closed = true
		ws.conn.Close()
	}
}

func readWSMessage(r *bufio.Reader) ([]byte, error) {
	// Read first two bytes
	header := make([]byte, 2)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	// Check if it's a close frame
	opcode := header[0] & 0x0F
	if opcode == 0x08 {
		return nil, fmt.Errorf("close frame received")
	}

	// Get payload length
	payloadLen := int(header[1] & 0x7F)
	masked := (header[1] & 0x80) != 0

	if payloadLen == 126 {
		lenBytes := make([]byte, 2)
		if _, err := io.ReadFull(r, lenBytes); err != nil {
			return nil, err
		}
		payloadLen = int(binary.BigEndian.Uint16(lenBytes))
	} else if payloadLen == 127 {
		lenBytes := make([]byte, 8)
		if _, err := io.ReadFull(r, lenBytes); err != nil {
			return nil, err
		}
		payloadLen = int(binary.BigEndian.Uint64(lenBytes))
	}

	// Read mask key if present
	var maskKey []byte
	if masked {
		maskKey = make([]byte, 4)
		if _, err := io.ReadFull(r, maskKey); err != nil {
			return nil, err
		}
	}

	// Read payload
	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}

	// Unmask if needed
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}

	return payload, nil
}
