package xlog

import (
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/DeRuina/timberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// levelController 全局日志级别原子控制器，支持运行时动态调整日志级别。
	// 使用 AtomicLevel 而非静态 Level，允许在不重启服务的情况下通过 SetLevel 切换级别。
	levelController = zap.NewAtomicLevelAt(zap.DebugLevel)
)

// initDefaultLogger 懒初始化默认日志器，在首次使用时自动调用。
//
// 当业务代码未主动调用 SetupLogger 时，使用此函数初始化一个输出到 stdout 的基础日志器，
// 方便开发小工具和单元测试时无需显式配置日志即可正常输出。
func initDefaultLogger() {
	SetupLogger("")
}

// CloseLogger 在进程退出前调用，将缓冲区中未写入的日志强制刷写到磁盘。
//
// Zap 为了性能会缓冲部分日志，不调用 Sync 可能导致最后几条日志丢失。
// 建议在 main 函数中通过 defer CloseLogger() 确保此函数被调用。
func CloseLogger() {
	_ = rootLogger.Sync()
}

// header 构建日志前缀标识，从环境变量中读取实例标识信息。
//
// 读取顺序：GAME_ID → WORLD_ID → INSTANCE_TYPE → INSTANCE_ID，
// 过滤掉空值和 "0" 值后拼接为空格分隔的字符串，附加到每条日志的时间字段后。
// 适合多实例部署场景下快速定位日志来源。
func header() string {
	var strs = []string{
		os.Getenv("GAME_ID"),
		os.Getenv("WORLD_ID"),
		os.Getenv("INSTANCE_TYPE"),
		os.Getenv("INSTANCE_ID"),
	}
	strs = slices.DeleteFunc(strs, func(s string) bool {
		s = strings.TrimSpace(s)
		return s == "" || s == "0"
	})
	return strings.Join(strs, " ")
}

// SetupLogger 初始化全局日志器。
//
// 日志格式（Console 格式）：时间 [实例标识] 级别 [文件:行号] 消息
//
// logfile 参数行为：
//   - 空字符串：日志输出到 stdout
//   - 非空路径：日志输出到指定文件（通过 timberjack 自动按天轮转）
//
// 配置要点：
//   - AddCallerSkip(2)：跳过封装层调用帧，确保显示业务代码的真实调用位置
//   - AddStacktrace(ErrorLevel)：Error 及以上级别自动附加调用栈，便于问题定位
//   - 时间格式含毫秒（.999），兼顾可读性和精度
func SetupLogger(logfile string) {
	head := header()
	config := zapcore.EncoderConfig{
		CallerKey:     "line",
		LevelKey:      "level",
		MessageKey:    "message",
		TimeKey:       "time",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeTime: func(t time.Time, encoder zapcore.PrimitiveArrayEncoder) {
			str := t.Format("2006-01-02 15:04:05.999")
			encoder.AppendString(str)
			if head != "" {
				encoder.AppendString(head) // 在时间字段后附加实例标识
			}
		},
		EncodeLevel: func(level zapcore.Level, encoder zapcore.PrimitiveArrayEncoder) {
			encoder.AppendString(strings.ToTitle(level.String()))
		},
		EncodeCaller: func(caller zapcore.EntryCaller, encoder zapcore.PrimitiveArrayEncoder) {
			encoder.AppendString("[" + caller.TrimmedPath() + "]")
		},
		EncodeDuration:   zapcore.SecondsDurationEncoder,
		EncodeName:       zapcore.FullNameEncoder,
		ConsoleSeparator: " ",
	}
	encoder := zapcore.NewConsoleEncoder(config)

	core := zapcore.NewCore(encoder, os.Stdout, levelController)
	if logfile != "" {
		lumberWriterSync := zapcore.AddSync(fileWriter(logfile))
		core = zapcore.NewCore(encoder, lumberWriterSync, levelController)
	}
	// AddCallerSkip(2) 跳过 xlog 包自身的封装调用，使调用位置指向业务代码
	_zLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(2), zap.AddStacktrace(zapcore.ErrorLevel))

	rootLogger = newzLogger(_zLogger)
}

// SetLevel 动态调整全局日志输出级别，无需重启服务即时生效。
//
// 适用场景：生产环境临时开启 Debug 日志排查问题，排查完毕后恢复 Info 级别。
func SetLevel(l zapcore.Level) {
	levelController.SetLevel(l)
}

// fileWriter 创建基于 timberjack 的按天轮转日志文件写入器。
//
// 轮转策略：
//   - MaxSize：单文件最大 50MB
//   - MaxBackups：最多保留 7 个备份文件
//   - MaxAge：文件最多保留 7 天
//   - RotationInterval：每 24 小时轮转一次（基于时间，非仅基于大小）
func fileWriter(path string) io.Writer {
	out := &timberjack.Logger{
		Filename:         path,
		MaxBackups:       7,
		MaxSize:          50, // MB
		MaxAge:           7,  // 天
		Compression:      "none",
		LocalTime:        true,
		RotationInterval: 24 * time.Hour,
		BackupTimeFormat: "2006-01-02-15-04-05",
	}
	return out
}
