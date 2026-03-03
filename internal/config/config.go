package config

import (
	"fmt"
	"net"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Source      SourceConfig      `yaml:"source"`
	Destination DestinationConfig `yaml:"destination"`
	Logging     LoggingConfig     `yaml:"logging"`
}

type SourceConfig struct {
	Interface string `yaml:"interface"`
	Port      int    `yaml:"port"`
}

type DestinationConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	User         string `yaml:"user"`
	Pass         string `yaml:"pass"`
	NTRIPVersion int    `yaml:"ntrip_version"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Neu interface la "auto" hoac rong, tu phat hien
	if cfg.Source.Interface == "auto" || cfg.Source.Interface == "" {
		iface, err := AutoDetectInterface()
		if err != nil {
			return nil, fmt.Errorf("auto-detect interface failed: %w", err)
		}
		cfg.Source.Interface = iface
	}

	return &cfg, nil
}

// AutoDetectInterface tra ve ten interface mang dau tien co IP va dang hoat dong (khong phai loopback).
func AutoDetectInterface() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("khong lay duoc danh sach interface: %w", err)
	}

	for _, iface := range ifaces {
		// Bo qua loopback va interface dang tat
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil || len(addrs) == 0 {
			continue
		}
		// Uu tien interface co IPv4
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil && ip.To4() != nil && !ip.IsLoopback() {
				return iface.Name, nil
			}
		}
	}

	// Fallback
	return "eth0", fmt.Errorf("khong tim thay interface hop le, dung fallback eth0")
}
