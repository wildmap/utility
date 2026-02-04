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
	// levelController 日志输出基本控制器
	levelController = zap.NewAtomicLevelAt(zap.DebugLevel)
)

// initDefaultLogger 在没有外部调用Setup进行日志库设置的情况下，进行默认的日志库配置；
// 以便开发单独的小应用的使用时候；
func initDefaultLogger() {
	SetupLogger("")
}

// CloseLogger 系统运行结束时，将日志落盘；
func CloseLogger() {
	_ = rootLogger.Sync()
}

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

func SetupLogger(logfile string) {
	head := header()
	// 将日志输出到屏幕
	config := zapcore.EncoderConfig{
		CallerKey:     "line", // 打印文件名和行数
		LevelKey:      "level",
		MessageKey:    "message",
		TimeKey:       "time",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeTime: func(t time.Time, encoder zapcore.PrimitiveArrayEncoder) {
			str := t.Format("2006-01-02 15:04:05.999")
			encoder.AppendString(str)
			if head != "" {
				encoder.AppendString(head)
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
	// 将日志输出到滚动切割文件中
	if logfile != "" {
		lumberWriterSync := zapcore.AddSync(fileWriter(logfile))
		core = zapcore.NewCore(encoder, lumberWriterSync, levelController)
	}
	// 生产根logger，设置输出调度点(上跳2行），输出Error级别的堆栈信息，
	_zLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(2), zap.AddStacktrace(zapcore.ErrorLevel)) // 选择输出调用点,对于ErrorLevel输出调用堆栈；

	rootLogger = newzLogger(_zLogger)
}

func SetLevel(l zapcore.Level) {
	levelController.SetLevel(l)
}

func fileWriter(path string) io.Writer {
	out := &timberjack.Logger{
		Filename:         path,                  // 日志文件路径
		MaxBackups:       7,                     // 最多保留7个备份
		MaxSize:          50,                    // 日志文件最大M
		MaxAge:           7,                     // 最大保存天数
		Compression:      "none",                // 压缩方式, none, gzip, zstd
		LocalTime:        true,                  // 是否使用本地时间
		RotationInterval: 24 * time.Hour,        // 日志轮转时间间隔
		BackupTimeFormat: "2006-01-02-15-04-05", // 日志轮转时间格式
	}
	return out
}
