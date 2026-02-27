package parser

import (
	"bytes"
	"strings"
)

type NTRIPRequest struct {
	Method     string
	MountPoint string
	Version    string
	Headers    map[string]string
	RTCMData   []byte
}

func ParseNTRIPRequest(data []byte) (*NTRIPRequest, error) {
	headerEnd := bytes.Index(data, []byte("\r\n\r\n"))
	if headerEnd == -1 {
		return nil, nil
	}

	headerStr := string(data[:headerEnd])
	lines := strings.Split(headerStr, "\r\n")

	if len(lines) < 1 {
		return nil, nil
	}

	req := &NTRIPRequest{
		Headers: make(map[string]string),
	}

	firstLine := strings.Split(lines[0], " ")
	if len(firstLine) >= 2 {
		req.Method = firstLine[0]
		path := firstLine[1]
		if strings.HasPrefix(path, "/") {
			req.MountPoint = path[1:]
		} else {
			req.MountPoint = path
		}
	}

	for i := 1; i < len(lines); i++ {
		parts := strings.SplitN(lines[i], ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			req.Headers[key] = value
		}
	}

	if headerEnd+4 < len(data) {
		req.RTCMData = data[headerEnd+4:]
	}

	return req, nil
}

func IsNTRIPRequest(data []byte) bool {
	headerEnd := bytes.Index(data, []byte("\r\n\r\n"))
	if headerEnd == -1 {
		return false
	}
	firstLine := string(data[:headerEnd])
	return strings.HasPrefix(firstLine, "GET ") ||
		strings.HasPrefix(firstLine, "POST ") ||
		strings.HasPrefix(firstLine, "SOURCE ")
}
