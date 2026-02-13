package xexec

import (
	"io"
)

// ScriptOption 脚本选项函数类型
type ScriptOption func(*script)

// WithScriptWorkdir 设置脚本工作目录
func WithScriptWorkdir(path string) ScriptOption {
	return func(s *script) {
		s.dir = path
	}
}

// WithScriptEnv 设置脚本环境变量
func WithScriptEnv(env ...string) ScriptOption {
	return func(s *script) {
		s.env = env
	}
}

// WithScriptArgs 设置脚本参数
func WithScriptArgs(args ...string) ScriptOption {
	return func(s *script) {
		s.args = args
	}
}

// WithScriptStdin 设置脚本标准输入
func WithScriptStdin(stdin io.Reader) ScriptOption {
	return func(s *script) {
		s.stdin = stdin
	}
}

// WithScriptLogger 设置自定义日志记录器
func WithScriptLogger(logger ILogger) ScriptOption {
	return func(s *script) {
		s.logger = logger
	}
}

// WithScriptSecrets 设置需要脱敏的敏感词
func WithScriptSecrets(secrets ...string) ScriptOption {
	return func(s *script) {
		// 过滤掉空字符串，避免替换所有字符
		var validSecrets []string
		for _, sec := range secrets {
			if len(sec) > 0 {
				validSecrets = append(validSecrets, sec)
			}
		}
		s.secrets = append(s.secrets, validSecrets...)
	}
}
