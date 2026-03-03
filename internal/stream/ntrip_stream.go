package stream

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"rtcm-relay/internal/forwarder"
	"rtcm-relay/internal/parser"

	"github.com/google/gopacket"
	"github.com/google/gopacket/tcpassembly"
)

// BaseStationStream xu ly chieu base station -> server (dst port = 12101).
// Parse SOURCE/GET header de lay mount, sau do forward tat ca RTCM data len caster.
type BaseStationStream struct {
	factory      *StreamFactory
	headerBuf    []byte
	headerParsed bool
	mount        string
	writer       *io.PipeWriter
}

func (s *BaseStationStream) Reassembled(r []tcpassembly.Reassembly) {
	for _, reassembly := range r {
		data := reassembly.Bytes
		if len(data) == 0 {
			continue
		}

		if !s.headerParsed {
			s.headerBuf = append(s.headerBuf, data...)
			headerEnd := bytes.Index(s.headerBuf, []byte("\r\n\r\n"))
			if headerEnd == -1 {
				continue
			}

			s.headerParsed = true

			req, err := parser.ParseNTRIPRequest(s.headerBuf)
			if err != nil || req == nil || req.MountPoint == "" {
				log.Printf("[WARN] BaseStationStream: no mount in header")
				return
			}

			s.mount = req.MountPoint
			log.Printf("[INFO] Base station connected, mount=%s", s.mount)

			reader, writer := io.Pipe()
			s.writer = writer

			fwd := forwarder.NewForwarder(
				s.factory.DestHost, s.factory.DestPort,
				s.mount,
				s.factory.DestUser, s.factory.DestPass,
				s.factory.NTRIPVersion,
				func() { log.Printf("[DEBUG] Forwarder closed, mount=%s", s.mount) },
			)
			go fwd.StartForwarding(reader)

			remaining := s.headerBuf[headerEnd+4:]
			s.headerBuf = nil
			if len(remaining) > 0 {
				writer.Write(remaining)
			}
		} else if s.writer != nil {
			if _, err := s.writer.Write(data); err != nil {
				log.Printf("[DEBUG] Write RTCM error mount=%s: %v", s.mount, err)
			}
		}
	}
}

func (s *BaseStationStream) ReassemblyComplete() {
	if s.writer != nil {
		s.writer.Close()
	}
	if s.mount != "" {
		log.Printf("[INFO] Base station disconnected, mount=%s", s.mount)
	}
}

// IgnoredStream bo qua chieu server -> base station (ICY 200 OK, khong co RTCM).
type IgnoredStream struct{}

func (s *IgnoredStream) Reassembled(_ []tcpassembly.Reassembly) {}
func (s *IgnoredStream) ReassemblyComplete()                    {}

// StreamFactory tao stream tuong ung cho tung TCP connection.
type StreamFactory struct {
	DestHost     string
	DestPort     int
	DestUser     string
	DestPass     string
	NTRIPVersion int
	SrcPort      int
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
	listenPort := fmt.Sprintf("%d", f.SrcPort)

	if dstPort == listenPort {
		// Base station -> server: co RTCM data
		log.Printf("[DEBUG] Base station stream: %s:%s -> %s:%s", srcIP, srcPort, dstIP, dstPort)
		return &BaseStationStream{factory: f}
	}

	// Server -> base station: chi co ICY 200 OK, bo qua
	_ = srcIP
	_ = dstIP
	_ = srcPort
	_ = dstPort
	return &IgnoredStream{}
}
