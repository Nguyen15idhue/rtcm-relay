package forwarder

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type Forwarder struct {
	host         string
	port         int
	mount        string
	user         string
	pass         string
	ntripVersion int
	conn         net.Conn
	mu           sync.Mutex
	connected    bool
	onClose      func()
}

func NewForwarder(host string, port int, mount string, user string, pass string, ntripVersion int, onClose func()) *Forwarder {
	if ntripVersion == 0 {
		ntripVersion = 1
	}
	return &Forwarder{
		host:         host,
		port:         port,
		mount:        mount,
		user:         user,
		pass:         pass,
		ntripVersion: ntripVersion,
		onClose:      onClose,
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

	var request string
	if f.ntripVersion == 1 {
		// NTRIP 1.0: dung HTTP/1.0, Authorization: Basic
		authStr := f.user + ":" + f.pass
		encoded := base64.StdEncoding.EncodeToString([]byte(authStr))
		request = fmt.Sprintf("GET /%s HTTP/1.0\r\nUser-Agent: NTRIP GoCaster/1.0\r\n", f.mount)
		if f.pass != "" {
			request += fmt.Sprintf("Authorization: Basic %s\r\n", encoded)
		}
		request += "\r\n"
	} else {
		// NTRIP 2.0
		request = fmt.Sprintf("GET /%s HTTP/1.1\r\nHost: %s\r\nNtrip-Version: Ntrip/2.0\r\nUser-Agent: NTRIP GoCaster/2.0\r\n", f.mount, f.host)
		if f.pass != "" {
			authStr := f.user + ":" + f.pass
			encoded := base64.StdEncoding.EncodeToString([]byte(authStr))
			request += fmt.Sprintf("Authorization: Basic %s\r\n", encoded)
		}
		request += "\r\n"
	}

	_, err = conn.Write([]byte(request))
	if err != nil {
		log.Printf("[DEBUG] Failed to send NTRIP request: %v", err)
		conn.Close()
		return err
	}

	f.conn = conn
	f.connected = true
	log.Printf("[DEBUG] Connected to %s, mount: %s (NTRIP v%d)", addr, f.mount, f.ntripVersion)
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

func (f *Forwarder) SetMount(mount string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mount = mount
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
