package config

import (
	"bytes"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
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

	for k, v := range entry.Data {
		b.WriteString(" ")
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

func InitLogger() {
	defaultLoggerOnce.Do(func() {
		defaultLogger = &logrus.Logger{
			Out:   os.Stderr,
			Level: logrus.DebugLevel,
			Formatter: &logrus.TextFormatter{
				FullTimestamp:   true,
				TimestampFormat: "2006-01-02 15:04:05",
				ForceColors:     true,
			},
		}
	})

	blockchainLoggerOnce.Do(func() {
		blockchainLogger = &logrus.Logger{
			Out:   os.Stderr,
			Level: logrus.DebugLevel,
			Formatter: &CustomFormatter{
				Prefix:      "[BLOCKCHAIN]",
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
