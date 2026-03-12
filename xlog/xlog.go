package xlog

// xlog 包提供全局单例日志器的便捷访问接口。
//
// 所有函数均代理到全局 rootLogger 实例，若 rootLogger 尚未通过 SetupLogger 初始化，
// 则在首次调用时自动执行 initDefaultLogger 完成懒初始化。
// 这种设计使得在不调用 SetupLogger 的情况下也能正常使用日志，
// 适合单元测试和小工具的快速开发场景。
//
// 用法示例：
//
//	xlog.SetupLogger("app.log")         // 程序启动时初始化
//	defer xlog.CloseLogger()             // 程序退出时刷写缓冲
//	xlog.Infof("server started on %s", addr)

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	rootLogger *zLogger
)

// Debug 输出 Debug 级别日志。
func Debug(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Debug(args...)
}

// Debugln 输出 Debug 级别日志（末尾追加换行）。
func Debugln(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Debugln(args...)
}

// Debugf 输出格式化的 Debug 级别日志。
func Debugf(format string, args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Debugf(format, args...)
}

// Debugw 输出带键值对的 Debug 级别结构化日志。
func Debugw(msg string, keysAndValues ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Debugw(msg, keysAndValues...)
}

// Debugx 以 zapcore.Field 方式输出高性能 Debug 级别日志（零分配）。
func Debugx(msg string, fields ...zapcore.Field) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Debugx(msg, fields...)
}

// Info 输出 Info 级别日志。
func Info(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Info(args...)
}

// Infoln 输出 Info 级别日志（末尾追加换行）。
func Infoln(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Infoln(args...)
}

// Infof 输出格式化的 Info 级别日志。
func Infof(format string, args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Infof(format, args...)
}

// Infow 输出带键值对的 Info 级别结构化日志。
func Infow(msg string, keysAndValues ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Infow(msg, keysAndValues...)
}

// Infox 以 zapcore.Field 方式输出高性能 Info 级别日志（零分配）。
func Infox(msg string, fields ...zapcore.Field) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Infox(msg, fields...)
}

// Warn 输出 Warn 级别日志。
func Warn(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warn(args...)
}

// Warnln 输出 Warn 级别日志（末尾追加换行）。
func Warnln(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warnln(args...)
}

// Warnf 输出格式化的 Warn 级别日志。
func Warnf(format string, args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warnf(format, args...)
}

// Warnw 输出带键值对的 Warn 级别结构化日志。
func Warnw(msg string, keysAndValues ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warnw(msg, keysAndValues...)
}

// Warnx 以 zapcore.Field 方式输出高性能 Warn 级别日志（零分配）。
func Warnx(msg string, fields ...zapcore.Field) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warnx(msg, fields...)
}

// Warning 是 Warn 的别名，兼容 gRPC 等期望 Warning 方法的第三方接口。
func Warning(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warning(args...)
}

// Warningln 是 Warnln 的别名。
func Warningln(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warningln(args...)
}

// Warningf 是 Warnf 的别名。
func Warningf(format string, args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warningf(format, args...)
}

// Warningw 是 Warnw 的别名。
func Warningw(msg string, keysAndValues ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warningw(msg, keysAndValues...)
}

// Warningx 是 Warnx 的别名。
func Warningx(msg string, fields ...zapcore.Field) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Warningx(msg, fields...)
}

// Error 输出 Error 级别日志，自动附加调用栈信息。
func Error(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Error(args...)
}

// Errorln 输出 Error 级别日志（末尾追加换行）。
func Errorln(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Errorln(args...)
}

// Errorf 输出格式化的 Error 级别日志。
func Errorf(format string, args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Errorf(format, args...)
}

// Errorw 输出带键值对的 Error 级别结构化日志。
func Errorw(msg string, keysAndValues ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Errorw(msg, keysAndValues...)
}

// Errorx 以 zapcore.Field 方式输出高性能 Error 级别日志（零分配）。
func Errorx(msg string, fields ...zapcore.Field) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Errorx(msg, fields...)
}

// DPanic 在 Development 模式下 panic，生产模式下仅记录日志。
func DPanic(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Panic(args...)
}

// DPanicln 同 DPanic，末尾追加换行。
func DPanicln(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.DPanicln(args...)
}

// DPanicf 同 DPanic，支持格式化字符串。
func DPanicf(format string, args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Panicf(format, args...)
}

// DPanicw 同 DPanic，支持键值对结构化输出。
func DPanicw(msg string, keysAndValues ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Panicw(msg, keysAndValues...)
}

// DPanicx 同 DPanic，以 zapcore.Field 方式输出高性能日志。
func DPanicx(msg string, fields ...zapcore.Field) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Panicx(msg, fields...)
}

// Panic 输出 Panic 级别日志，并调用 panic() 触发运行时恐慌。
func Panic(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Panic(args...)
}

// Panicln 同 Panic，末尾追加换行。
func Panicln(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Panicln(args...)
}

// Panicf 同 Panic，支持格式化字符串。
func Panicf(format string, args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Panicf(format, args...)
}

// Panicw 同 Panic，支持键值对结构化输出。
func Panicw(msg string, keysAndValues ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Panicw(msg, keysAndValues...)
}

// Panicx 同 Panic，以 zapcore.Field 方式输出高性能日志。
func Panicx(msg string, fields ...zapcore.Field) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Panicx(msg, fields...)
}

// Fatal 输出 Fatal 级别日志，并调用 os.Exit(1) 终止进程。
func Fatal(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Fatal(args...)
}

// Fatalln 同 Fatal，末尾追加换行。
func Fatalln(args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Fatalln(args...)
}

// Fatalf 同 Fatal，支持格式化字符串。
func Fatalf(format string, args ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Fatalf(format, args...)
}

// Fatalw 同 Fatal，支持键值对结构化输出。
func Fatalw(msg string, keysAndValues ...any) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Fatalw(msg, keysAndValues...)
}

// Fatalx 同 Fatal，以 zapcore.Field 方式输出高性能日志。
func Fatalx(msg string, fields ...zapcore.Field) {
	if rootLogger == nil {
		initDefaultLogger()
	}
	rootLogger.Fatalx(msg, fields...)
}

// GetSubLogger 获取继承当前配置的子 Logger。
func GetSubLogger() ILogger {
	if rootLogger == nil {
		initDefaultLogger()
	}
	return rootLogger.GetSubLogger()
}

// GetSubLoggerWithFields 获取附加了固定字段的子 Logger。
func GetSubLoggerWithFields(fields ...zap.Field) ILogger {
	if rootLogger == nil {
		initDefaultLogger()
	}
	return rootLogger.GetSubLoggerWithFields(fields...)
}

// GetSubLoggerWithKeyValue 通过键值对 map 获取附加了固定字段的子 Logger。
func GetSubLoggerWithKeyValue(keysAndValues map[string]string) ILogger {
	if rootLogger == nil {
		initDefaultLogger()
	}
	return rootLogger.GetSubLoggerWithKeyValue(keysAndValues)
}

// GetSubLoggerWithOption 通过 zap.Option 获取自定义配置的子 Logger。
func GetSubLoggerWithOption(opts ...zap.Option) ILogger {
	if rootLogger == nil {
		initDefaultLogger()
	}
	return rootLogger.GetSubLoggerWithOption(opts...)
}
