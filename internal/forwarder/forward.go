package forwarder

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type Forwarder struct {
	host      string
	port      int
	mount     string
	conn      net.Conn
	mu        sync.Mutex
	connected bool
	onClose   func()
}

func NewForwarder(host string, port int, mount string, onClose func()) *Forwarder {
	return &Forwarder{
		host:    host,
		port:    port,
		mount:   mount,
		onClose: onClose,
	}
}

func (f *Forwarder) Connect() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.connected && f.conn != nil {
		return nil
	}

	addr := fmt.Sprintf("%s:%d", f.host, f.port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Printf("[DEBUG] Failed to connect to %s: %v", addr, err)
		return err
	}

	request := fmt.Sprintf("GET /%s HTTP/1.1\r\nHost: %s\r\nNtrip-Version: Ntrip/2.0\r\n\r\n",
		f.mount, f.host)

	_, err = conn.Write([]byte(request))
	if err != nil {
		log.Printf("[DEBUG] Failed to send NTRIP request: %v", err)
		conn.Close()
		return err
	}

	f.conn = conn
	f.connected = true
	log.Printf("[DEBUG] Connected to %s, mount: %s", addr, f.mount)
	return nil
}

func (f *Forwarder) Forward(data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.connected || f.conn == nil {
		if err := f.Connect(); err != nil {
			return err
		}
	}

	_, err := f.conn.Write(data)
	if err != nil {
		f.connected = false
		f.conn = nil
		if f.onClose != nil {
			f.onClose()
		}
		return err
	}
	return nil
}

func (f *Forwarder) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.conn != nil {
		f.conn.Close()
		f.conn = nil
		f.connected = false
	}
}

func (f *Forwarder) IsConnected() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.connected
}

func PipeData(reader io.Reader, writer io.Writer, onClose func()) {
	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			_, writeErr := writer.Write(buf[:n])
			if writeErr != nil {
				log.Printf("[DEBUG] Write error: %v", writeErr)
				break
			}
		}
		if err != nil {
			break
		}
	}
	if onClose != nil {
		onClose()
	}
}

func (f *Forwarder) StartForwarding(reader io.Reader) {
	for {
		err := f.Connect()
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		PipeData(reader, f.conn, func() {
			f.mu.Lock()
			f.connected = false
			f.conn = nil
			f.mu.Unlock()
			log.Printf("[DEBUG] Connection closed, will reconnect")
		})

		time.Sleep(1 * time.Second)
	}
}
