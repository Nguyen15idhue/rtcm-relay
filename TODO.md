# RTCM Stream Relay - Task List

## Project: RTCM Stream Relay (Go + Pcap)

### Mục tiêu
- Sniff RTCM stream từ port 12101 (trước khi vào CGBAS Pro)
- Parse NTRIP protocol để lấy mount name
- Forward đến GeoNtripCaster (rtktk.online:1509)

---

## Task List

| # | Task | Status | Ghi chú |
|---|------|--------|---------|
| 1 | Tạo cấu trúc thư mục project | ✅ Done | |
| 2 | Tạo go.mod và config.yaml | ✅ Done | |
| 3 | Implement config loader | ✅ Done | |
| 4 | Implement NTRIP parser | ✅ Done | |
| 5 | Implement forwarder | ✅ Done | |
| 6 | Implement TCP stream handler | ✅ Done | |
| 7 | Implement pcap sniffer | ✅ Done | |
| 8 | Tạo main.go | ✅ Done | |
| 9 | Build và verify | ✅ Done | Binary: rtcm-relay.exe |

---

## Thông tin hệ thống

| Thông số | Giá trị |
|----------|---------|
| Source Port | 12101 |
| Destination | rtktk.online:1509 |
| Interface | eth0 |
| Auth | Không |
| Forward | Tất cả mount |

## Cấu trúc Project

```
rtcm-relay/
├── go.mod
├── go.sum
├── config.yaml
├── rtcm-relay.exe        # Binary đã build
├── cmd/main.go
├── TODO.md
└── internal/
    ├── config/config.go
    ├── sniffer/sniffer.go
    ├── stream/ntrip_stream.go
    ├── parser/ntrip.go
    └── forwarder/forward.go
```

## Cách chạy trên VPS

```bash
# Cài đặt libpcap
apt update
apt install libpcap-dev

# Copy binary và config lên VPS
scp rtcm-relay.exe user@vps:/path/
scp config.yaml user@vps:/path/

# Chạy (cần root)
sudo ./rtcm-relay.exe -config config.yaml
```

## Log

- 2026-02-28: Khởi tạo project ✅
- 2026-02-28: Hoàn thành tất cả tasks ✅
- 2026-02-28: Build thành công ✅
