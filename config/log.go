package config

import (
	"github.com/go-kit/log"
	"os"
	"time"
)

var TimestampSecondsUTC log.Valuer = func() interface{} {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func LoggerWithPrefixes(target string, keyvals ...interface{}) log.Logger {
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	logger = log.With(logger, "TIME", TimestampSecondsUTC)
	logger = log.With(logger, "TARGET", target)

	return log.With(logger, keyvals...)
}

func DefaultDebugLogger(target string) log.Logger {
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	logger = log.With(logger, "TIME", TimestampSecondsUTC)
	logger = log.With(logger, "DEBUG", target)

	return logger
}
