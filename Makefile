BINARY     = rtcm-relay
BINARY_WIN = rtcm-relay.exe
CMD        = ./cmd/main.go
VERSION    = $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS    = -ldflags "-X main.version=$(VERSION)"

.PHONY: all linux windows clean run deps

## Default: build cho Linux (chay lenh nay TREN VPS Linux)
## NOTE: gopacket/pcap yeu cau CGO + libpcap, KHONG the cross-compile tu Windows
all: linux

## Build binary cho Linux - PHAI chay tren Linux (VPS)
linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY) $(CMD)
	@echo "Built: $(BINARY) (linux/amd64)"

## Build tren Windows (cho may dev)
windows:
	go build $(LDFLAGS) -o $(BINARY_WIN) $(CMD)
	@echo "Built: $(BINARY_WIN)"

## Tai dependencies
deps:
	go mod tidy
	go mod download

## Chay truc tiep (khong build)
run:
	go run $(CMD) -config config.yaml

## Xoa binaries
clean:
	rm -f $(BINARY) $(BINARY_WIN)

## Deploy: SSH vao VPS, git pull, build lai, restart service
## Su dung: make deploy VPS_HOST=1.2.3.4 VPS_USER=root
deploy:
	@if [ -z "$(VPS_HOST)" ]; then echo "Thieu VPS_HOST. Dung: make deploy VPS_HOST=1.2.3.4 VPS_USER=root"; exit 1; fi
	ssh $(VPS_USER)@$(VPS_HOST) "cd /opt/rtcm-relay && git pull && make linux && systemctl restart rtcm-relay"
	@echo "Deployed to $(VPS_HOST)"
