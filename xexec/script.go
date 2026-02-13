package xexec

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	// digitRe 用于匹配数字的正则表达式
	digitRe = regexp.MustCompile(`\d+`)
)

const (
	// maxCapacity 最大缓冲区大小 100MB
	maxCapacity = 100 * 1024 * 1024
)

// script 脚本执行器结构
type script struct {
	cmd          *exec.Cmd          // 命令对象
	ctx          context.Context    // 上下文
	cancel       context.CancelFunc // 取消函数
	env          []string           // 环境变量列表
	args         []string           // 脚本参数
	dir          string             // 工作目录
	path         string             // 脚本路径
	stdin        io.Reader          // 标准输入
	exitCodePath string             // 退出码文件路径
	saveEnvPath  string             // 保存环境变量文件路径
	logger       ILogger            // 日志记录器
	secrets      []string           // 敏感词列表
	masker       *strings.Replacer  // 用于脱敏的替换器
}

// ExecScriptContent 执行脚本内容
// 将内容写入临时文件后执行
func ExecScriptContent(ctx context.Context, kind, content string, options ...ScriptOption) (code int, env map[string]any, err error) {
	scriptPath := filepath.Join(os.TempDir(), fmt.Sprintf(".script_%d.%s", time.Now().UnixNano(), kind))
	defer func() {
		_ = os.Remove(scriptPath)
	}()
	err = os.WriteFile(scriptPath, []byte(content), os.ModePerm)
	if err != nil {
		return 255, nil, err
	}
	return ExecScript(ctx, scriptPath, options...)
}

// ExecScript 执行脚本文件
func ExecScript(ctx context.Context, path string, options ...ScriptOption) (code int, env map[string]any, err error) {
	if ctx == nil {
		ctx = context.Background()
	}

	s := &script{
		path:   path,
		logger: &defaultLogger{}, // 使用默认日志实现
	}
	s.exitCodePath = filepath.Join(os.TempDir(), s.randomFilename("exitCode", ".txt"))
	s.saveEnvPath = filepath.Join(os.TempDir(), s.randomFilename("saveEnv", ".txt"))
	s.abs()
	_ = os.Chmod(path, os.ModePerm)
	_, err = os.Stat(path)
	if err != nil {
		return 255, nil, fmt.Errorf("stat script: %w", err)
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	defer func() {
		s.cancel()
		_ = os.Remove(s.exitCodePath)
		_ = os.Remove(s.saveEnvPath)
	}()
	for _, option := range options {
		option(s)
	}

	// 生成替换对：["secret1", "***", "secret2", "***"]
	replacements := make([]string, 0, len(s.secrets)*2)
	for _, secret := range s.secrets {
		replacements = append(replacements, secret, "***")
	}
	if len(replacements) > 0 {
		s.masker = strings.NewReplacer(replacements...)
	}

	defer func() {
		if s.ctx.Err() != nil {
			err = s.ctx.Err()
		}
	}()
	code, err = s.execScript()
	// 从文件读取的退出码更准确(因为包装脚本会捕获真实退出码)
	code1, err1 := s.parseExitCodeFromFile()
	if err1 == nil {
		code = code1
	}
	env = s.parseSaveEnvFile()
	return code, env, err
}

func (s *script) abs() {
	if filepath.IsAbs(s.path) {
		s.path = filepath.Clean(s.path)
	}
	s.path = filepath.Join(s.dir, s.path)
}

func (s *script) execScript() (code int, err error) {
	ext := filepath.Ext(s.path)
	switch ext {
	case ".py", ".py3":
		return s.execPythonScript()
	default:
		return s.execSysScript()
	}
}

func (s *script) randomFilename(prefix, ext string) string {
	return fmt.Sprintf("%s-%d%s", prefix, time.Now().UnixNano(), ext)
}

func (s *script) execPythonScript() (code int, err error) {
	// 使用 Python 包装脚本以正确透传标准输入和参数
	wrapperContent := fmt.Sprintf(`# -*- coding: utf-8 -*-
import sys
import os
import runpy

exit_code = 0
script_path = r'%s'

# 设置 sys.argv，第一个参数是脚本路径，后续是传入的参数
sys.argv = [script_path] + sys.argv[1:]

try:
    runpy.run_path(script_path, run_name='__main__')
except SystemExit as e:
    exit_code = e.code if e.code is not None else 0
except Exception:
    import traceback
    traceback.print_exc()
    exit_code = 255
finally:
    code_file = os.environ.get('XEXEC_EXIT_CODE_FILE')
    if code_file and not os.path.exists(code_file):
        try:
            with open(code_file, 'w') as f:
                f.write(str(exit_code if isinstance(exit_code, int) else 1))
        except:
            pass
    sys.exit(exit_code if isinstance(exit_code, int) else 1)
`, s.path)
	wrapperPath := filepath.Join(os.TempDir(), s.randomFilename("wrapper", ".py"))
	defer func() {
		_ = os.Remove(wrapperPath)
	}()
	if err = os.WriteFile(wrapperPath, []byte(wrapperContent), os.ModePerm); err != nil {
		return 255, err
	}
	// 将包装脚本路径和用户参数合并传递给 python
	args := append([]string{"-u", wrapperPath}, s.args...)
	return s.exec("python", args...)
}

func (s *script) exec(command string, args ...string) (code int, err error) {
	s.cmd = exec.CommandContext(s.ctx, command, args...)
	s.cmd.Env = s.mergeEnv()
	if s.dir != "" {
		s.cmd.Dir = s.dir
	}
	s.beforeExec()
	if s.stdin != nil {
		s.cmd.Stdin = s.stdin
	}
	stdout, err := s.cmd.StdoutPipe()
	if err != nil {
		return 255, err
	}
	stderr, err := s.cmd.StderrPipe()
	if err != nil {
		return 255, err
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		s.consoleOutput("STDOUT", stdout)
	}()
	go func() {
		defer wg.Done()
		s.consoleOutput("STDERR", stderr)
	}()
	err = s.cmd.Run()
	wg.Wait()

	// 僵尸进程收割会触发no child processes
	if err != nil && strings.Contains(err.Error(), "waitid: no child processes") {
		err = nil
	}

	if err != nil {
		code = 255
	}
	if s.cmd.ProcessState != nil {
		code = s.cmd.ProcessState.ExitCode()
	}
	return code, err
}

// parseExitCodeFromFile 按行读取文件，获取到退出码就结束
func (s *script) parseExitCodeFromFile() (int, error) {
	data, err := os.ReadFile(s.exitCodePath)
	if err != nil {
		return 255, fmt.Errorf("read code file error: %w", err)
	}

	matches := digitRe.FindString(string(bytes.TrimSpace(data)))
	if matches == "" {
		return 255, fmt.Errorf("no valid exit code found")
	}

	return strconv.Atoi(matches)
}

// mergeEnv 合并系统环境变量和自定义环境变量，去除重复
// 自定义环境变量 s.env 优先级更高，会覆盖系统环境变量中的同名键
func (s *script) mergeEnv() []string {
	// 创建 map 来存储环境变量，键是变量名，值是完整的 "KEY=VALUE" 字符串
	envMap := make(map[string]string)

	// 先添加系统环境变量
	for _, env := range syscall.Environ() {
		key := s.getEnvKey(env)
		if key != "" {
			envMap[key] = env
		}
	}

	// 用自定义环境变量覆盖（优先级更高）
	for _, env := range s.env {
		key := s.getEnvKey(env)
		if key != "" {
			envMap[key] = env
		}
	}

	// 转换回 []string
	result := make([]string, 0, len(envMap))
	for _, env := range envMap {
		result = append(result, env)
	}

	result = append(
		result,
		// 增加自定义退出码环境变量
		fmt.Sprintf("XEXEC_EXIT_CODE_FILE=%s", s.exitCodePath),
		fmt.Sprintf("XEXEC_SAVE_ENV_FILE=%s", s.saveEnvPath),
	)

	return result
}

// getEnvKey 从 "KEY=VALUE" 格式的字符串中提取键名
func (s *script) getEnvKey(env string) string {
	idx := strings.IndexByte(env, '=')
	if idx == -1 {
		return ""
	}
	return env[:idx]
}

func (s *script) parseSaveEnvFile() map[string]any {
	var saveEnv = make(map[string]any)
	file, err := os.Open(s.saveEnvPath)
	if err != nil {
		return saveEnv
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // 设置最大 token 为 10MB

	firstLine := true
	for scanner.Scan() {
		line := scanner.Text()

		// 1. BOM 处理
		if firstLine {
			firstLine = false
			if len(line) >= 3 && line[0] == 0xEF && line[1] == 0xBB && line[2] == 0xBF {
				s.logger.Debugf("[SYSTEM] UTF-8 BOM detected and removed")
				line = line[3:]
			}
		}

		// 2. 优化点：跳过空行，避免不必要的报错
		if strings.TrimSpace(line) == "" {
			continue
		}

		singleLineEnv := strings.Index(line, "=")
		multiLineEnv := strings.Index(line, "<<")

		// 逻辑分析：
		// 情况 A: 只有 = (singleLine != -1, multiLine == -1) -> 单行
		// 情况 B: 都有，且 = 在前 (singleLine < multiLine) -> 单行 (例如: KEY=VALUE<<EOF 被视为值的一部分)
		// 情况 C: << 在前 (multiLine != -1 且 (singleLine == -1 || multiLine < singleLine)) -> 多行
		// 注意：原代码逻辑在 multiLine < singleLine 时会误入单行逻辑，这里微调判断顺序

		if multiLineEnv != -1 && (singleLineEnv == -1 || multiLineEnv < singleLineEnv) {
			// === 多行处理逻辑 ===
			key := strings.TrimSpace(line[:multiLineEnv])
			delimiter := strings.TrimSpace(line[multiLineEnv+2:]) // 获取分界符

			if key == "" || delimiter == "" {
				s.logger.Errorf("[SYSTEM] invalid multiline format: key or delimiter is empty in line '%s'", line)
				continue
			}

			var valueBuilder strings.Builder
			delimiterFound := false

			for scanner.Scan() {
				content := scanner.Text()
				if strings.TrimSpace(content) == delimiter {
					delimiterFound = true
					break
				}
				if valueBuilder.Len() > 0 {
					valueBuilder.WriteString("\n")
				}
				valueBuilder.WriteString(content)
			}

			if !delimiterFound {
				s.logger.Errorf("[SYSTEM] EOF reached before finding delimiter '%s'", delimiter)
				return saveEnv
			}

			// 存储结果, 禁止覆盖系统变量
			if _, exists := os.LookupEnv(key); exists {
				// 发现冲突，记录警告并跳过覆盖
				s.logger.Warnf("[SYSTEM] ignored attempt to overwrite system variable: %s", key)
				continue
			}
			saveEnv[key] = valueBuilder.String()
			s.logger.Debugf("[SYSTEM] parsed env: %s", key)

		} else if singleLineEnv != -1 {
			// === 单行处理逻辑 ===
			key := strings.TrimSpace(line[:singleLineEnv])
			value := line[singleLineEnv+1:]

			if key == "" {
				s.logger.Errorf("[SYSTEM] invalid format: empty key in line '%s'", line)
				continue
			}

			// 存储结果, 禁止覆盖系统变量
			if _, exists := os.LookupEnv(key); exists {
				// 发现冲突，记录警告并跳过覆盖
				s.logger.Warnf("[SYSTEM] ignored attempt to overwrite system variable: %s", key)
				continue
			}
			saveEnv[key] = value
			s.logger.Debugf("[SYSTEM] parsed env: %s", key)

		} else {
			// === 格式错误 ===
			s.logger.Errorf("[SYSTEM] invalid format '%v', expected 'KEY=VALUE' or 'KEY<<EOF'", line)
			continue
		}
	}

	if err = scanner.Err(); err != nil {
		s.logger.Errorf("[SYSTEM] error reading file: %v", err)
		return saveEnv
	}

	// 自查确认
	s.logger.Infof("[SYSTEM] successfully parsed %d environment variables.", len(saveEnv))
	return saveEnv
}
