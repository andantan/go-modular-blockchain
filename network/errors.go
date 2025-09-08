package network

import (
	"errors"
	"io"
	"net"
	"strings"
)

func IsUnrecoverableTCPError(err error) bool {
	if err == nil {
		return false
	}

	if err == io.EOF || errors.Is(err, net.ErrClosed) {
		return true
	}

	var netErr net.Error
	ok := errors.As(err, &netErr)
	if ok && netErr.Timeout() {
		return true
	}

	errStr := err.Error()

	if strings.Contains(errStr, "connection reset by peer") || // Linux/macOS
		strings.Contains(errStr, "forcibly closed") || // Windows
		strings.Contains(errStr, "broken pipe") { // Linux/macOS
		return true
	}

	if strings.Contains(errStr, "connection refused") {
		return true
	}

	if strings.Contains(errStr, "no such host") {
		return true
	}

	return false
}
