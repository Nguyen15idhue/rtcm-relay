package stream

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"rtcm-relay/internal/forwarder"
	"rtcm-relay/internal/parser"
	"sync"

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

// DataStream: doc ICY 200 OK (bo qua), sau do forward toan bo RTCM data len caster.
// Chieu: server -> client (src port = 12101)
type DataStream struct {
	headerBuf    []byte
	headerParsed bool
	key          string // "clientIP:clientPort"
	factory      *StreamFactory
	writer       *io.PipeWriter
}

func (s *DataStream) Reassembled(r []tcpassembly.Reassembly) {
	for _, reassembly := range r {
		if len(reassembly.Bytes) == 0 {
			continue
		}
		if !s.headerParsed {
			s.headerBuf = append(s.headerBuf, reassembly.Bytes...)
			headerEnd := bytes.Index(s.headerBuf, []byte("\r\n\r\n"))
			if headerEnd == -1 {
				continue
			}
			s.headerParsed = true

			// Lay mount point tu RequestStream da luu
			mountVal, ok := s.factory.mounts.Load(s.key)
			if !ok {
				log.Printf("[DEBUG] No mount for client %s, skip", s.key)
				return
			}
			mount := mountVal.(string)
			log.Printf("[INFO] Starting RTCM forward: mount=%s client=%s", mount, s.key)

			reader, writer := io.Pipe()
			s.writer = writer

			fwd := forwarder.NewForwarder(
				s.factory.DestHost, s.factory.DestPort,
				mount,
				s.factory.DestUser, s.factory.DestPass,
				s.factory.NTRIPVersion,
				func() { log.Printf("[DEBUG] Forwarder closed, mount=%s", mount) },
			)
			go fwd.StartForwarding(reader)

			// RTCM data bat dau ngay sau ICY header
			remaining := s.headerBuf[headerEnd+4:]
			if len(remaining) > 0 {
				writer.Write(remaining)
			}
			s.headerBuf = nil
		} else {
			if s.writer != nil {
				_, err := s.writer.Write(reassembly.Bytes)
				if err != nil {
					log.Printf("[DEBUG] Pipe write error: %v", err)
				}
			}
		}
	}
}

func (s *DataStream) ReassemblyComplete() {
	if s.writer != nil {
		s.writer.Close()
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
