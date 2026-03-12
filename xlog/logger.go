package xlog

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 编译期接口合规检查：确保 zLogger 实现了 ILogger 接口。
var _ ILogger = &zLogger{}

// ILogger 定义完整的日志记录接口，覆盖 Debug/Info/Warn/Error/Panic/Fatal/DPanic 七个级别。
//
// 每个级别提供五种调用风格：
//   - 普通风格（Debug）：接受任意参数，空格分隔
//   - 换行风格（Debugln）：同普通风格，末尾追加换行
//   - 格式化风格（Debugf）：fmt.Sprintf 格式
//   - 键值对风格（Debugw）：msg + key/value 对，适合结构化日志
//   - 字段风格（Debugx）：msg + zapcore.Field，性能最优，零内存分配
//
// Warning 系列是 Warn 系列的别名，兼容 gRPC 等期望 Warning 方法的接口。
type ILogger interface {
	Print(...any)
	Println(...any)
	Printf(string, ...any)
	Printw(string, ...any)
	Printx(string, ...zapcore.Field)

	Debug(...any)
	Debugln(...any)
	Debugf(string, ...any)
	Debugw(string, ...any)
	Debugx(string, ...zapcore.Field)

	Info(...any)
	Infoln(...any)
	Infof(string, ...any)
	Infow(string, ...any)
	Infox(string, ...zapcore.Field)

	Warn(...any)
	Warnln(...any)
	Warnf(string, ...any)
	Warnw(string, ...any)
	Warnx(string, ...zapcore.Field)

	Warning(...any)
	Warningln(...any)
	Warningf(string, ...any)
	Warningw(string, ...any)
	Warningx(string, ...zapcore.Field)

	Error(...any)
	Errorln(...any)
	Errorf(string, ...any)
	Errorw(string, ...any)
	Errorx(string, ...zapcore.Field)

	Panic(...any)
	Panicln(...any)
	Panicf(string, ...any)
	Panicw(string, ...any)
	Panicx(string, ...zapcore.Field)

	Fatal(...any)
	Fatalln(...any)
	Fatalf(string, ...any)
	Fatalw(string, ...any)
	Fatalx(string, ...zapcore.Field)

	DPanic(...any)
	DPanicln(...any)
	DPanicf(string, ...any)
	DPanicw(string, ...any)
	DPanicx(string, ...zapcore.Field)

	Enabled(level zapcore.Level) bool
	GetSubLogger() ILogger
	GetSubLoggerWithFields(fields ...zap.Field) ILogger
	GetSubLoggerWithKeyValue(map[string]string) ILogger
	GetSubLoggerWithOption(...zap.Option) ILogger
}

// zLogger 封装 Uber Zap 的高性能日志器，实现 ILogger 接口。
//
// 同时持有 *zap.Logger（用于 Debugx/Infox 等字段风格，零反射最高性能）
// 和 *zap.SugaredLogger（用于 Debug/Debugf/Debugw 等便捷风格）。
type zLogger struct {
	logger  *zap.Logger
	slogger *zap.SugaredLogger
}

// newzLogger 创建 zLogger 实例，共享同一底层 zap.Logger。
func newzLogger(logger *zap.Logger) *zLogger {
	aLogger := &zLogger{
		logger:  logger,
		slogger: logger.Sugar(),
	}
	return aLogger
}

// Print 以 Info 级别输出日志（Print 系列映射到 Info 级别）。
func (z *zLogger) Print(args ...any) {
	z.slogger.Info(args...)
}

// Println 以 Info 级别输出日志（末尾追加换行）。
func (z *zLogger) Println(args ...any) {
	z.slogger.Infoln(args...)
}

// Printf 以 Info 级别输出格式化日志。
func (z *zLogger) Printf(template string, args ...any) {
	z.slogger.Infof(template, args...)
}

// Printw 以 Info 级别输出带键值对的结构化日志。
func (z *zLogger) Printw(msg string, keysAndValues ...any) {
	z.slogger.Infow(msg, keysAndValues...)
}

// Printx 以 Info 级别输出带 zap.Field 的高性能结构化日志。
func (z *zLogger) Printx(msg string, fields ...zapcore.Field) {
	z.logger.Info(msg, fields...)
}

// Debug 输出 Debug 级别日志。
func (z *zLogger) Debug(args ...any) {
	z.slogger.Debug(args...)
}

// Debugln 输出 Debug 级别日志（末尾追加换行）。
func (z *zLogger) Debugln(args ...any) {
	z.slogger.Debugln(args...)
}

// Debugf 输出格式化的 Debug 级别日志。
func (z *zLogger) Debugf(template string, args ...any) {
	z.slogger.Debugf(template, args...)
}

// Debugw 输出带键值对的 Debug 级别结构化日志。
func (z *zLogger) Debugw(msg string, keysAndValues ...any) {
	z.slogger.Debugw(msg, keysAndValues...)
}

// Debugx 以 zapcore.Field 方式输出高性能 Debug 级别日志（零分配）。
func (z *zLogger) Debugx(msg string, fields ...zapcore.Field) {
	z.logger.Debug(msg, fields...)
}

// Info 输出 Info 级别日志。
func (z *zLogger) Info(args ...any) {
	z.slogger.Info(args...)
}

// Infoln 输出 Info 级别日志（末尾追加换行）。
func (z *zLogger) Infoln(args ...any) {
	z.slogger.Infoln(args...)
}

// Infof 输出格式化的 Info 级别日志。
func (z *zLogger) Infof(template string, args ...any) {
	z.slogger.Infof(template, args...)
}

// Infow 输出带键值对的 Info 级别结构化日志。
func (z *zLogger) Infow(msg string, keysAndValues ...any) {
	z.slogger.Infow(msg, keysAndValues...)
}

// Infox 以 zapcore.Field 方式输出高性能 Info 级别日志（零分配）。
func (z *zLogger) Infox(msg string, fields ...zapcore.Field) {
	z.logger.Info(msg, fields...)
}

// Warn 输出 Warn 级别日志。
func (z *zLogger) Warn(args ...any) {
	z.slogger.Warn(args...)
}

// Warnln 输出 Warn 级别日志（末尾追加换行）。
func (z *zLogger) Warnln(args ...any) {
	z.slogger.Warnln(args...)
}

// Warnf 输出格式化的 Warn 级别日志。
func (z *zLogger) Warnf(template string, args ...any) {
	z.slogger.Warnf(template, args...)
}

// Warnw 输出带键值对的 Warn 级别结构化日志。
func (z *zLogger) Warnw(msg string, keysAndValues ...any) {
	z.slogger.Warnw(msg, keysAndValues...)
}

// Warnx 以 zapcore.Field 方式输出高性能 Warn 级别日志（零分配）。
func (z *zLogger) Warnx(msg string, fields ...zapcore.Field) {
	z.logger.Warn(msg, fields...)
}

// Warning 是 Warn 的别名，兼容 gRPC 等期望 Warning 方法的第三方接口。
func (z *zLogger) Warning(args ...any) {
	z.slogger.Warn(args...)
}

// Warningln 是 Warnln 的别名。
func (z *zLogger) Warningln(args ...any) {
	z.slogger.Warnln(args...)
}

// Warningf 是 Warnf 的别名。
func (z *zLogger) Warningf(template string, args ...any) {
	z.slogger.Warnf(template, args...)
}

// Warningw 是 Warnw 的别名。
func (z *zLogger) Warningw(msg string, keysAndValues ...any) {
	z.slogger.Warnw(msg, keysAndValues...)
}

// Warningx 是 Warnx 的别名。
func (z *zLogger) Warningx(msg string, fields ...zapcore.Field) {
	z.logger.Warn(msg, fields...)
}

// Error 输出 Error 级别日志，并自动附加调用栈信息。
func (z *zLogger) Error(args ...any) {
	z.slogger.Error(args...)
}

// Errorln 输出 Error 级别日志（末尾追加换行）。
func (z *zLogger) Errorln(args ...any) {
	z.slogger.Errorln(args...)
}

// Errorf 输出格式化的 Error 级别日志。
func (z *zLogger) Errorf(template string, args ...any) {
	z.slogger.Errorf(template, args...)
}

// Errorw 输出带键值对的 Error 级别结构化日志。
func (z *zLogger) Errorw(msg string, keysAndValues ...any) {
	z.slogger.Errorw(msg, keysAndValues...)
}

// Errorx 以 zapcore.Field 方式输出高性能 Error 级别日志（零分配）。
func (z *zLogger) Errorx(msg string, fields ...zapcore.Field) {
	z.logger.Error(msg, fields...)
}

// DPanic 在 Development 模式下 panic，生产模式下仅记录日志。
func (z *zLogger) DPanic(args ...any) {
	z.slogger.DPanic(args...)
}

// DPanicln 同 DPanic，末尾追加换行。
func (z *zLogger) DPanicln(args ...any) {
	z.slogger.DPanicln(args...)
}

// DPanicf 同 DPanic，支持格式化字符串。
func (z *zLogger) DPanicf(template string, args ...any) {
	z.slogger.DPanicf(template, args...)
}

// DPanicw 同 DPanic，支持键值对结构化输出。
func (z *zLogger) DPanicw(msg string, keysAndValues ...any) {
	z.slogger.DPanicw(msg, keysAndValues...)
}

// DPanicx 同 DPanic，以 zapcore.Field 方式输出高性能日志。
func (z *zLogger) DPanicx(msg string, fields ...zapcore.Field) {
	z.logger.DPanic(msg, fields...)
}

// Panic 输出 Panic 级别日志，并调用 panic() 触发运行时恐慌。
func (z *zLogger) Panic(args ...any) {
	z.slogger.Panic(args...)
}

// Panicln 同 Panic，末尾追加换行。
func (z *zLogger) Panicln(args ...any) {
	z.slogger.Panicln(args...)
}

// Panicf 同 Panic，支持格式化字符串。
func (z *zLogger) Panicf(template string, args ...any) {
	z.slogger.Panicf(template, args...)
}

// Panicw 同 Panic，支持键值对结构化输出。
func (z *zLogger) Panicw(msg string, keysAndValues ...any) {
	z.slogger.Panicw(msg, keysAndValues...)
}

// Panicx 同 Panic，以 zapcore.Field 方式输出高性能日志。
func (z *zLogger) Panicx(msg string, fields ...zapcore.Field) {
	z.logger.Panic(msg, fields...)
}

// Fatal 输出 Fatal 级别日志，并调用 os.Exit(1) 终止进程。
func (z *zLogger) Fatal(args ...any) {
	z.slogger.Fatal(args...)
}

// Fatalln 同 Fatal，末尾追加换行。
func (z *zLogger) Fatalln(args ...any) {
	z.slogger.Fatalln(args...)
}

// Fatalf 同 Fatal，支持格式化字符串。
func (z *zLogger) Fatalf(template string, args ...any) {
	z.slogger.Fatalf(template, args...)
}

// Fatalw 同 Fatal，支持键值对结构化输出。
func (z *zLogger) Fatalw(msg string, keysAndValues ...any) {
	z.slogger.Fatalw(msg, keysAndValues...)
}

// Fatalx 同 Fatal，以 zapcore.Field 方式输出高性能日志。
func (z *zLogger) Fatalx(msg string, fields ...zapcore.Field) {
	z.logger.Fatal(msg, fields...)
}

// Sync 将日志缓冲区内容刷写到输出端（文件或标准输出），程序退出前应调用此方法。
func (z *zLogger) Sync() error {
	return z.logger.Sync()
}

// Enabled 检查指定日志级别是否启用，用于避免构造日志参数的无效开销。
func (z *zLogger) Enabled(level zapcore.Level) bool {
	return z.logger.Core().Enabled(level)
}

// GetSubLogger 创建继承当前配置的子 Logger，子 Logger 的修改不影响父 Logger。
func (z *zLogger) GetSubLogger() ILogger {
	_zLogger := z.logger.WithOptions()
	tmp := newzLogger(_zLogger)
	return tmp
}

// GetSubLoggerWithFields 创建附加了固定字段的子 Logger，适合为特定业务模块添加标识字段。
func (z *zLogger) GetSubLoggerWithFields(fields ...zap.Field) ILogger {
	_zLogger := z.logger.With(fields...)
	tmp := newzLogger(_zLogger)
	return tmp
}

// GetSubLoggerWithKeyValue 通过键值对 map 创建附加了固定字段的子 Logger。
//
// 适合在初始化时通过配置文件指定日志字段（如服务名、机器 ID）的场景。
func (z *zLogger) GetSubLoggerWithKeyValue(keyAndValues map[string]string) ILogger {
	fields := make([]zap.Field, 0)
	for key, value := range keyAndValues {
		fields = append(fields, zap.String(key, value))
	}
	_zLogger := z.logger.With(fields...)
	tmp := newzLogger(_zLogger)
	return tmp
}

// GetSubLoggerWithOption 通过 zap.Option 创建附加了自定义选项的子 Logger。
func (z *zLogger) GetSubLoggerWithOption(opts ...zap.Option) ILogger {
	_zLogger := z.logger.WithOptions(opts...)
	tmp := newzLogger(_zLogger)
	return tmp
}
