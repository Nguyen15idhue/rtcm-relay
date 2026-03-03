package stream

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"rtcm-relay/internal/forwarder"
	"rtcm-relay/internal/parser"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/tcpassembly"
)

// RequestStream: chi doc GET request de lay mount point, KHONG forward gi ca.
// Chieu: client -> server (dst port = 12101)
type RequestStream struct {
	headerBuf    []byte
	headerParsed bool
	key          string // "clientIP:clientPort"
	factory      *StreamFactory
}

func (s *RequestStream) Reassembled(r []tcpassembly.Reassembly) {
	if s.headerParsed {
		return
	}
	for _, reassembly := range r {
		if len(reassembly.Bytes) == 0 {
			continue
		}
		s.headerBuf = append(s.headerBuf, reassembly.Bytes...)
		headerEnd := bytes.Index(s.headerBuf, []byte("\r\n\r\n"))
		if headerEnd == -1 {
			continue
		}
		req, err := parser.ParseNTRIPRequest(s.headerBuf)
		if err == nil && req != nil && req.MountPoint != "" {
			log.Printf("[INFO] Mount detected: %s (client: %s)", req.MountPoint, s.key)
			s.factory.mounts.Store(s.key, req.MountPoint)
		}
		s.headerParsed = true
		break
	}
}

func (s *RequestStream) ReassemblyComplete() {
	// Giu lai mount mot luc de DataStream kip lay, xoa sau
}

// DataStream: forward RTCM data tu server -> client len caster.
// Strip ICY/HTTP header neu co, forward raw RTCM binary.
type DataStream struct {
	key         string // "clientIP:clientPort"
	factory     *StreamFactory
	writer      *io.PipeWriter
	writerMu    sync.Mutex
	started     bool
	startOnce   sync.Once
	headerBuf   []byte
	headerDone  bool
}

const maxHeaderBuf = 2048 // neu sau 2KB van chua gap \r\n\r\n, coi nhu khong co header

func (s *DataStream) Reassembled(r []tcpassembly.Reassembly) {
	for _, reassembly := range r {
		if len(reassembly.Bytes) == 0 {
			continue
		}

		// Lan dau co data: launch goroutine doi mount roi bat dau forward
		s.startOnce.Do(func() {
			go s.startForwardWhenReady()
		})

		if !s.headerDone {
			s.headerBuf = append(s.headerBuf, reassembly.Bytes...)

			headerEnd := bytes.Index(s.headerBuf, []byte("\r\n\r\n"))
			if headerEnd != -1 {
				// Tim thay header, bo qua phan header, giu lai phan con lai
				s.headerDone = true
				remaining := s.headerBuf[headerEnd+4:]
				s.headerBuf = nil
				if len(remaining) > 0 {
					s.writeToForwarder(remaining)
				}
			} else if len(s.headerBuf) >= maxHeaderBuf {
				// Qua 2KB van chua gap header -> coi nhu khong co HTTP header, forward tat ca
				s.headerDone = true
				buf := s.headerBuf
				s.headerBuf = nil
				s.writeToForwarder(buf)
			}
			// Else: van dang buffer, cho them data
		} else {
			s.writeToForwarder(reassembly.Bytes)
		}
	}
}

func (s *DataStream) writeToForwarder(data []byte) {
	s.writerMu.Lock()
	w := s.writer
	s.writerMu.Unlock()
	if w != nil {
		if _, err := w.Write(data); err != nil {
			log.Printf("[DEBUG] Pipe write error key=%s: %v", s.key, err)
		}
	}
}

func (s *DataStream) startForwardWhenReady() {
	// Doi toi da 3 giay de RequestStream luu mount
	var mount string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if mountVal, ok := s.factory.mounts.Load(s.key); ok {
			mount = mountVal.(string)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if mount == "" {
		log.Printf("[WARN] No mount for client %s after 3s, skip", s.key)
		return
	}
	log.Printf("[INFO] Starting RTCM forward: mount=%s client=%s", mount, s.key)

	reader, writer := io.Pipe()

	fwd := forwarder.NewForwarder(
		s.factory.DestHost, s.factory.DestPort,
		mount,
		s.factory.DestUser, s.factory.DestPass,
		s.factory.NTRIPVersion,
		func() { log.Printf("[DEBUG] Forwarder closed, mount=%s", mount) },
	)
	go fwd.StartForwarding(reader)

	s.writerMu.Lock()
	s.writer = writer
	s.writerMu.Unlock()
}

func (s *DataStream) ReassemblyComplete() {
	s.writerMu.Lock()
	w := s.writer
	s.writerMu.Unlock()
	if w != nil {
		w.Close()
	}
	s.factory.mounts.Delete(s.key)
}

// StreamFactory tao stream cho tung TCP connection.
type StreamFactory struct {
	DestHost     string
	DestPort     int
	DestUser     string
	DestPass     string
	NTRIPVersion int
	SrcPort      int
	mounts       sync.Map // "clientIP:clientPort" -> mountPoint
}

func NewStreamFactory(host string, port int, user string, pass string, ntripVersion int, srcPort int) *StreamFactory {
	return &StreamFactory{
		DestHost:     host,
		DestPort:     port,
		DestUser:     user,
		DestPass:     pass,
		NTRIPVersion: ntripVersion,
		SrcPort:      srcPort,
	}
}

func (f *StreamFactory) New(netFlow, tcpFlow gopacket.Flow) tcpassembly.Stream {
	srcIP := netFlow.Src().String()
	dstIP := netFlow.Dst().String()
	srcPort := tcpFlow.Src().String()
	dstPort := tcpFlow.Dst().String()

	srcPortStr := fmt.Sprintf("%d", f.SrcPort)

	if dstPort == srcPortStr {
		// Client -> Server: parse GET de lay mount
		key := fmt.Sprintf("%s:%s", srcIP, srcPort)
		log.Printf("[DEBUG] Request stream: %s:%s -> %s:%s", srcIP, srcPort, dstIP, dstPort)
		return &RequestStream{key: key, factory: f}
	} else if srcPort == srcPortStr {
		// Server -> Client: RTCM data (sau ICY 200 OK)
		// key dung clientIP:clientPort de match voi RequestStream
		key := fmt.Sprintf("%s:%s", dstIP, dstPort)
		log.Printf("[DEBUG] Data stream: %s:%s -> %s:%s (mount key: %s)", srcIP, srcPort, dstIP, dstPort, key)
		return &DataStream{key: key, factory: f}
	}

	// Traffic khac (khong lien quan)
	return &RequestStream{key: "", factory: f}
}
