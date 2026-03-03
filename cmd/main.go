package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"rtcm-relay/internal/config"
	"rtcm-relay/internal/sniffer"
	"syscall"
)

func main() {
	configPath  := flag.String("config", "config.yaml", "Path to config file")
	ifaceOverride := flag.String("interface", "", "Override network interface (e.g. eth0, ens3). Ghi de gia tri trong config.yaml")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Flag -interface ghi de config
	if *ifaceOverride != "" {
		cfg.Source.Interface = *ifaceOverride
	}

	log.Printf("[DEBUG] Configuration loaded:")
	log.Printf("[DEBUG]   Source Interface: %s", cfg.Source.Interface)
	log.Printf("[DEBUG]   Source Port: %d", cfg.Source.Port)
	log.Printf("[DEBUG]   Destination Host: %s", cfg.Destination.Host)
	log.Printf("[DEBUG]   Destination Port: %d", cfg.Destination.Port)
	log.Printf("[DEBUG]   Log Level: %s", cfg.Logging.Level)

	sniffer, err := sniffer.NewSniffer(
		cfg.Source.Interface,
		cfg.Source.Port,
		cfg.Destination.Host,
		cfg.Destination.Port,
	)
	if err != nil {
		log.Fatalf("Failed to create sniffer: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("[DEBUG] Shutting down...")
		sniffer.Close()
		os.Exit(0)
	}()

	log.Println("[DEBUG] RTCM Relay starting...")
	sniffer.Start()
}
