package xlog

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	rootLogger *zLogger
)

// Debug 输出"Debug"级别日志信息；
func Debug(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Debug(args...)
}

func Debugln(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Debugln(args...)
}

// Debugf 输出格式化的"Debug"级别日志信息；
func Debugf(format string, args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Debugf(format, args...)
}

// Debugw 输出定制化的"Debug"级别日志信息；
func Debugw(msg string, keysAndValues ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Debugw(msg, keysAndValues...)
}

// Debugx 以zapfield方式，极速输出定制化的"Debug"级别日志信息；
func Debugx(msg string, fields ...zapcore.Field) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Debugx(msg, fields...)
}

// Info 输出"Info"级别日志信息；
func Info(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Info(args...)
}

func Infoln(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Infoln(args...)
}

// Infof 输出格式化的"Info"级别日志信息；
func Infof(format string, args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Infof(format, args...)
}

// InfoW 输出定制化的"Info"级别日志信息；
func Infow(msg string, keysAndValues ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Infow(msg, keysAndValues...)
}

// Infox 以zapfield方式，极速输出定制化的"Info"级别日志信息；
func Infox(msg string, fields ...zapcore.Field) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Infox(msg, fields...)
}

// Warn 输出"Warn"级别日志信息；
func Warn(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warn(args...)
}

func Warnln(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warnln(args...)
}

// Warnf 输出格式化的"Warn"级别日志信息；
func Warnf(format string, args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warnf(format, args...)
}

// Warnw 输出定制化的"Warn"级别日志信息；
func Warnw(msg string, keysAndValues ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warnw(msg, keysAndValues...)
}

// Warnx 以zapfield方式，极速输出定制化的"Warn"级别日志信息；
func Warnx(msg string, fields ...zapcore.Field) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warnx(msg, fields...)
}

// Warning 输出"Warn"级别日志信息；
func Warning(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warning(args...)
}

func Warningln(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warningln(args...)
}

// Warningf 输出格式化的"Warn"级别日志信息；
func Warningf(format string, args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warningf(format, args...)
}

// Warningw 输出定制化的"Warn"级别日志信息；
func Warningw(msg string, keysAndValues ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warningw(msg, keysAndValues...)
}

// Warningx 以zapfield方式，极速输出定制化的"Warn"级别日志信息；
func Warningx(msg string, fields ...zapcore.Field) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warningx(msg, fields...)
}

// Error 输出"Error"级别日志信息；
func Error(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Error(args...)
}

func Errorln(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Errorln(args...)
}

// Errorf 输出格式化的"Error"级别日志信息；
func Errorf(format string, args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Errorf(format, args...)
}

// Errorw 输出定制化的"Error"级别日志信息；
func Errorw(msg string, keysAndValues ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Errorw(msg, keysAndValues...)
}

// Errorx 以zapfield方式，极速输出定制化的"Error"级别日志信息；
func Errorx(msg string, fields ...zapcore.Field) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Errorx(msg, fields...)
}

// DPanic 输出"DPanic"级别日志信息,但不引发程序Panic
func DPanic(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Panic(args...)
}

func DPanicln(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.DPanicln(args...)
}

// DPanicf 输出格式化的"DPanic"级别日志信息，但不引发程序Panic
func DPanicf(format string, args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Panicf(format, args...)
}

// DPanicw 输出定制化的"Panic"级别日志信息,但不引发程序Panic()
func DPanicw(msg string, keysAndValues ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Panicw(msg, keysAndValues...)
}

// DPanicx 以zapfield方式，极速输出定制化的"Panic"级别日志信息,但不引发程序Panic()
func DPanicx(msg string, fields ...zapcore.Field) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Panicx(msg, fields...)
}

// Panic 输出"Panic"级别日志信息，并引发程序Panic；
func Panic(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Panic(args...)
}

func Panicln(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Panicln(args...)
}

// Panicf 输出格式化的"Panic"级别日志信息，并引发程序Panic；
func Panicf(format string, args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Panicf(format, args...)
}

// Panicw 输出定制化的"Panic"级别日志信息，并引发程序Panic；
func Panicw(msg string, keysAndValues ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Panicw(msg, keysAndValues...)
}

// Panicx 以zapfield方式，极速输出定制化的"Panic"级别日志信息，并引发程序Panic；
func Panicx(msg string, fields ...zapcore.Field) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Panicx(msg, fields...)
}

// Fatal 输出"Fatal"级别日志信息，并使程序退出（os.Exit(1)；
func Fatal(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Fatal(args...)
}

func Fatalln(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Fatalln(args...)
}

// Fatalf 输出格式化的"Fatal"级别日志信息，并使程序退出（os.Exit(1)；
func Fatalf(format string, args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Fatalf(format, args...)
}

// Fatalw 输出定制化的"Fatal"级别日志信息，并使程序退出（os.Exit(1)；
func Fatalw(msg string, keysAndValues ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Fatalw(msg, keysAndValues...)
}

// Fatalx 以zapfield方式，极速输出定制化的"Fatal"级别日志信息，并使程序退出（os.Exit(1)；
func Fatalx(msg string, fields ...zapcore.Field) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Fatalx(msg, fields...)
}

// GetSubLogger 获取一个子logger
func GetSubLogger() ILogger {
	if rootLogger == nil {
		initDefaultLogger()
	}
	return rootLogger.GetSubLogger()
}

func GetSubLoggerWithFields(fields ...zap.Field) ILogger {
	if rootLogger == nil {
		initDefaultLogger()
	}
	return rootLogger.GetSubLoggerWithFields(fields...)
}

// GetSubLoggerWithKeyValue 使用指定的key/value,获取一个带有输出key/value内容的子logger
func GetSubLoggerWithKeyValue(keysAndValues map[string]string) ILogger {
	if rootLogger == nil {
		initDefaultLogger()
	}
	return rootLogger.GetSubLoggerWithKeyValue(keysAndValues)
}

func GetSubLoggerWithOption(opts ...zap.Option) ILogger {
	if rootLogger == nil {
		initDefaultLogger()
	}
	return rootLogger.GetSubLoggerWithOption(opts...)
}
