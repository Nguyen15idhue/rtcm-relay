package sniffer

import (
	"fmt"
	"log"
	"rtcm-relay/internal/stream"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/tcpassembly"
)

type Sniffer struct {
	handle     *pcap.Handle
	assembler  *tcpassembly.Assembler
	streamPool *tcpassembly.StreamPool
	factory    *stream.StreamFactory
}

func NewSniffer(interfaceName string, port int, destHost string, destPort int, destUser string, destPass string, ntripVersion int) (*Sniffer, error) {
	handle, err := pcap.OpenLive(interfaceName, 1600, true, pcap.BlockForever)
	if err != nil {
		return nil, fmt.Errorf("failed to open pcap handle: %w", err)
	}

	filter := fmt.Sprintf("tcp port %d", port)
	if err := handle.SetBPFFilter(filter); err != nil {
		return nil, fmt.Errorf("failed to set BPF filter: %w", err)
	}

	log.Printf("[DEBUG] Sniffer started on interface %s, filtering port %d", interfaceName, port)

	factory := stream.NewStreamFactory(destHost, destPort, destUser, destPass, ntripVersion, port)
	streamPool := tcpassembly.NewStreamPool(factory)
	assembler := tcpassembly.NewAssembler(streamPool)

	return &Sniffer{
		handle:     handle,
		assembler:  assembler,
		streamPool: streamPool,
		factory:    factory,
	}, nil
}

func (s *Sniffer) Start() {
	packetSource := gopacket.NewPacketSource(s.handle, layers.LayerTypeEthernet)

	log.Println("[DEBUG] Starting packet processing...")

	for packet := range packetSource.Packets() {
		if packet == nil {
			continue
		}

		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		tcpLayer := packet.Layer(layers.LayerTypeTCP)

		if ipLayer == nil || tcpLayer == nil {
			continue
		}

		ip, _ := ipLayer.(*layers.IPv4)
		tcp, _ := tcpLayer.(*layers.TCP)

		if tcp == nil {
			continue
		}

		netFlow, _ := gopacket.FlowFromEndpoints(
			layers.NewIPEndpoint(ip.SrcIP),
			layers.NewIPEndpoint(ip.DstIP),
		)

		s.assembler.Assemble(netFlow, tcp)
	}
}

func (s *Sniffer) Close() {
	log.Println("[DEBUG] Closing sniffer...")
	if s.handle != nil {
		s.handle.Close()
	}
}
