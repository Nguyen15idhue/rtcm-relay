package stream

import (
	"io"
	"log"
	"rtcm-relay/internal/forwarder"
	"rtcm-relay/internal/parser"

	"github.com/google/gopacket"
	"github.com/google/gopacket/tcpassembly"
)

type NTRIPStream struct {
	assembledData []byte
	mountPoint    string
	fwd           *forwarder.Forwarder
	firstData     bool
	reader        *io.PipeReader
	writer        *io.PipeWriter
}

func NewNTRIPStream(fwd *forwarder.Forwarder) *NTRIPStream {
	reader, writer := io.Pipe()
	return &NTRIPStream{
		fwd:       fwd,
		firstData: true,
		reader:    reader,
		writer:    writer,
	}
}

func (s *NTRIPStream) onData(data []byte) {
	if s.firstData {
		req, err := parser.ParseNTRIPRequest(data)
		if err != nil {
			log.Printf("[DEBUG] Failed to parse NTRIP request: %v", err)
			return
		}
		if req != nil && req.MountPoint != "" {
			s.mountPoint = req.MountPoint
			log.Printf("[DEBUG] Detected mount point: %s", s.mountPoint)

			if s.fwd != nil {
				go s.fwd.StartForwarding(s.reader)
			}
		}
		s.firstData = false
	}

	if s.mountPoint != "" && len(data) > 0 {
		_, err := s.writer.Write(data)
		if err != nil {
			log.Printf("[DEBUG] Write to pipe error: %v", err)
		}
	}
}

func (s *NTRIPStream) ReassembledBPF() []string {
	return []string{}
}

func (s *NTRIPStream) Reassembled(r []tcpassembly.Reassembly) {
	for _, reassembly := range r {
		if len(reassembly.Bytes) > 0 {
			s.onData(reassembly.Bytes)
		}
	}
}

func (s *NTRIPStream) ReassemblyComplete() {}

func (s *NTRIPStream) Write(b []byte) (n int, err error) {
	s.onData(b)
	return len(b), nil
}

func (s *NTRIPStream) Close() {
	if s.writer != nil {
		s.writer.Close()
	}
	log.Printf("[DEBUG] Stream closed for mount: %s", s.mountPoint)
}

type StreamFactory struct {
	DestHost string
	DestPort int
}

func NewStreamFactory(host string, port int) *StreamFactory {
	return &StreamFactory{
		DestHost: host,
		DestPort: port,
	}
}

func (f *StreamFactory) New(netFlow, tcpFlow gopacket.Flow) tcpassembly.Stream {
	log.Printf("[DEBUG] New TCP stream")

	fwd := forwarder.NewForwarder(f.DestHost, f.DestPort, "", func() {
		log.Printf("[DEBUG] Forwarder closed")
	})

	return NewNTRIPStream(fwd)
}
