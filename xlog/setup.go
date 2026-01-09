package xlog

import (
	"io"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/wildmap/utility/xtime"
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
	var str string
	if gameID := os.Getenv("GAME_ID"); gameID != "" {
		str += gameID
	}
	if worldID := os.Getenv("WORLD_ID"); worldID != "" {
		str += " " + worldID
	}
	if kind := os.Getenv("INSTANCE_TYPE"); kind != "" {
		str += " " + kind
	}
	if id := os.Getenv("INSTANCE_ID"); id != "" {
		str += " " + id
	}
	return str
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
			str := xtime.ToUTC(t).Format("2006-01-02 15:04:05.999")
			if head != "" {
				str += " " + head
			}
			encoder.AppendString(str)
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
	out := &lumberjack.Logger{
		Filename:   path,  // 日志文件路径
		MaxBackups: 15,    // 最多保留15个备份
		MaxSize:    100,   // 日志文件最大100M
		MaxAge:     28,    // 最多保留28天
		Compress:   false, // 压缩
		LocalTime:  true,  // 本地时间
	}
	return out
}
