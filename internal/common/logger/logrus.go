package logger

import (
	"fmt"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// Create a new logrus logger
func NewLogger(level string) *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(toLogrusLevel(level))
	logger.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: time.RFC3339Nano,
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "time",
			logrus.FieldKeyLevel: "severity",
			logrus.FieldKeyMsg:   "message",
		},
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			s := strings.Split(f.Function, ".")
			funcName := s[len(s)-1]
			return funcName, fmt.Sprintf("%s:%d", path.Base(f.File), f.Line)
		},
	})
	return logger
}

// Convert the log level to logrus level
func toLogrusLevel(level string) logrus.Level {
	switch level {
	case "debug":
		return logrus.DebugLevel
	case "info":
		return logrus.InfoLevel
	case "warn":
		return logrus.WarnLevel
	case "error":
		return logrus.ErrorLevel
	case "fatal":
		return logrus.FatalLevel
	case "panic":
		return logrus.PanicLevel
	default:
		return logrus.InfoLevel
	}
}
