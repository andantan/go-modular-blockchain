package config

import (
	"bytes"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"sort"
	"strings"
	"sync"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
)

type CustomFormatter struct {
	logrus.TextFormatter
	Prefix      string
	PrefixColor string
}

func (f *CustomFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer

	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	if f.Prefix != "" {
		if f.PrefixColor != "" {
			b.WriteString(f.PrefixColor)
		}
		b.WriteString(f.Prefix)
		if f.PrefixColor != "" {
			b.WriteString(ColorReset)
		}
		b.WriteString(" ")
	}

	var levelColor string

	switch entry.Level {
	case logrus.DebugLevel:
		levelColor = ColorPurple
	case logrus.InfoLevel:
		levelColor = ColorBlue
	case logrus.WarnLevel:
		levelColor = ColorYellow
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		levelColor = ColorRed
	default:
		levelColor = ColorWhite
	}

	timestamp := entry.Time.Format(f.TimestampFormat)
	b.WriteString(timestamp)
	b.WriteString(" ")

	levelText := "[" + strings.ToUpper(entry.Level.String()) + "]"
	b.WriteString(levelColor)
	b.WriteString(levelText)
	b.WriteString(ColorReset)

	_, _ = fmt.Fprintf(b, " %s ", entry.Message)

	if len(entry.Data) > 0 {
		b.WriteString(" ")
	}

	keys := make([]string, 0, len(entry.Data))
	for k := range entry.Data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for i, k := range keys {
		v := entry.Data[k]

		if i > 0 {
			b.WriteString(" ")
		}

		b.WriteString(ColorGreen)
		b.WriteString(fmt.Sprintf("%s", k))
		b.WriteString(ColorReset)
		b.WriteString("=")

		if v == nil {
			b.WriteString("<nil>")
		} else {
			b.WriteString(fmt.Sprintf("%v", v))
		}
	}
	b.WriteByte('\n')

	return b.Bytes(), nil
}

var defaultLogger *logrus.Logger
var defaultLoggerOnce sync.Once

var blockchainLogger *logrus.Logger
var blockchainLoggerOnce sync.Once

var networkLogger *logrus.Logger
var networkLoggerOnce sync.Once

var mempoolLogger *logrus.Logger
var mempoolLoggerOnce sync.Once

var serverLogger *logrus.Logger
var serverLoggerOnce sync.Once

func InitLogger(ID string) {
	prefixId := fmt.Sprintf("[ID=%s] ", ID)

	defaultLoggerOnce.Do(func() {
		defaultLogger = &logrus.Logger{
			Out:   os.Stderr,
			Level: logrus.DebugLevel,
			Formatter: &CustomFormatter{
				Prefix:      prefixId + "[DEFAULT]",
				PrefixColor: ColorCyan,
				TextFormatter: logrus.TextFormatter{
					FullTimestamp:   true,
					TimestampFormat: "2006-01-02 15:04:05",
					ForceColors:     true,
				},
			},
		}
	})

	blockchainLoggerOnce.Do(func() {
		blockchainLogger = &logrus.Logger{
			Out:   os.Stderr,
			Level: logrus.DebugLevel,
			Formatter: &CustomFormatter{
				Prefix:      prefixId + "[BLOCKCHAIN]",
				PrefixColor: ColorCyan,
				TextFormatter: logrus.TextFormatter{
					FullTimestamp:   true,
					TimestampFormat: "2006-01-02 15:04:05",
					ForceColors:     true,
				},
			},
		}
	})

	networkLoggerOnce.Do(func() {
		networkLogger = &logrus.Logger{
			Out:   os.Stderr,
			Level: logrus.DebugLevel,
			Formatter: &CustomFormatter{
				Prefix:      prefixId + "[NETWORK]",
				PrefixColor: ColorPurple,
				TextFormatter: logrus.TextFormatter{
					FullTimestamp:   true,
					TimestampFormat: "2006-01-02 15:04:05",
					ForceColors:     true,
				},
			},
		}
	})

	mempoolLoggerOnce.Do(func() {
		mempoolLogger = &logrus.Logger{
			Out:   os.Stderr,
			Level: logrus.DebugLevel,
			Formatter: &CustomFormatter{
				Prefix:      prefixId + "[MEMPOOL]",
				PrefixColor: ColorCyan,
				TextFormatter: logrus.TextFormatter{
					FullTimestamp:   true,
					TimestampFormat: "2006-01-02 15:04:05",
					ForceColors:     true,
				},
			},
		}
	})

	serverLoggerOnce.Do(func() {
		serverLogger = &logrus.Logger{
			Out:   os.Stderr,
			Level: logrus.DebugLevel,
			Formatter: &CustomFormatter{
				Prefix:      prefixId + "[SERVER]",
				PrefixColor: ColorCyan,
				TextFormatter: logrus.TextFormatter{
					FullTimestamp:   true,
					TimestampFormat: "2006-01-02 15:04:05",
					ForceColors:     true,
				},
			},
		}
	})
}

func GetDefaultLogger() *logrus.Logger {
	return defaultLogger
}

func GetBlockchainLogger() *logrus.Logger {
	return blockchainLogger
}

func GetNetworkLogger() *logrus.Logger {
	return networkLogger
}

func GetMempoolLogger() *logrus.Logger {
	return mempoolLogger
}

func GetServerLogger() *logrus.Logger {
	return serverLogger
}
