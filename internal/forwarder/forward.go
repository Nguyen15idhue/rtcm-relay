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
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		log.Printf("[DEBUG] Failed to connect to %s: %v", addr, err)
		return err
	}

	var request string
	if f.ntripVersion == 1 {
		// NTRIP 1.0 SOURCE protocol: day data LEN caster
		// Format: SOURCE password /mountpoint HTTP/1.0
		request = fmt.Sprintf("SOURCE %s /%s HTTP/1.0\r\nSource-Agent: NTRIP GoCaster/1.0\r\n\r\n",
			f.pass, f.mount)
	} else {
		// NTRIP 2.0 POST protocol
		var authHeader string
		if f.pass != "" {
			authStr := f.user + ":" + f.pass
			encoded := base64.StdEncoding.EncodeToString([]byte(authStr))
			authHeader = fmt.Sprintf("Authorization: Basic %s\r\n", encoded)
		}
		request = fmt.Sprintf(
			"POST /%s HTTP/1.1\r\nHost: %s\r\nNtrip-Version: Ntrip/2.0\r\n"+
				"User-Agent: NTRIP GoCaster/2.0\r\n%sContent-Type: gnss/data\r\n"+
				"Transfer-Encoding: chunked\r\n\r\n",
			f.mount, f.host, authHeader)
	}

	_, err = conn.Write([]byte(request))
	if err != nil {
		log.Printf("[DEBUG] Failed to send SOURCE request: %v", err)
		conn.Close()
		return err
	}

	// Doc response tu caster: "ICY 200 OK" hoac "20 OK" (NTRIP 1.0)
	respBuf := make([]byte, 256)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	n, err := conn.Read(respBuf)
	conn.SetReadDeadline(time.Time{}) // reset deadline
	if err != nil {
		log.Printf("[DEBUG] Failed to read caster response: %v", err)
		conn.Close()
		return fmt.Errorf("no response from caster: %w", err)
	}
	resp := string(respBuf[:n])
	log.Printf("[DEBUG] Caster response for mount %s: %q", f.mount, resp)

	// Kiem tra phan hoi hop le
	if !isOKResponse(resp) {
		conn.Close()
		return fmt.Errorf("caster rejected mount %s: %q", f.mount, resp)
	}

	f.conn = conn
	f.connected = true
	log.Printf("[INFO] SOURCE connected: %s/%s (NTRIP v%d)", addr, f.mount, f.ntripVersion)
	return nil
}

// isOKResponse kiem tra caster co chap nhan SOURCE request khong.
func isOKResponse(resp string) bool {
	for _, ok := range []string{"ICY 200 OK", "20 OK", "HTTP/1.1 200", "HTTP/1.0 200"} {
		if len(resp) >= len(ok) && resp[:len(ok)] == ok {
			return true
		}
	}
	return false
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
