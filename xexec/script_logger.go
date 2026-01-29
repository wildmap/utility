package xexec

import (
	"fmt"
	"log/slog"
)

// Logger 日志接口,允许自定义日志实现
type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// defaultLogger 默认日志实现,使用标准log包的slog
type defaultLogger struct{}

func (l *defaultLogger) Debugf(format string, args ...interface{}) {
	if len(args) == 0 {
		slog.Debug(format)
	} else {
		slog.Debug(fmt.Sprintf(format, args...))
	}
}

func (l *defaultLogger) Infof(format string, args ...interface{}) {
	if len(args) == 0 {
		slog.Info(format)
	} else {
		slog.Info(fmt.Sprintf(format, args...))
	}
}

func (l *defaultLogger) Warnf(format string, args ...interface{}) {
	if len(args) == 0 {
		slog.Warn(format)
	} else {
		slog.Warn(fmt.Sprintf(format, args...))
	}
}

func (l *defaultLogger) Errorf(format string, args ...interface{}) {
	if len(args) == 0 {
		slog.Error(format)
	} else {
		slog.Error(fmt.Sprintf(format, args...))
	}
}
