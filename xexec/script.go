package xexec

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wildmap/utility/xlog"
)

const (
	// 最大缓冲区大小 100MB
	maxCapacity = 100 * 1024 * 1024
)

type script struct {
	cmd          *exec.Cmd
	ctx          context.Context
	cancel       context.CancelFunc
	env          []string
	dir          string
	codeFilePath string
}

type ScriptOption func(*script)

func WithScriptWorkdir(path string) ScriptOption {
	return func(s *script) {
		s.dir = path
	}
}

func WithScriptEnv(env []string) ScriptOption {
	return func(s *script) {
		s.env = env
	}
}

func ExecScriptContent(ctx context.Context, kind, content string, options ...ScriptOption) (code int, err error) {
	scriptPath := filepath.Join(os.TempDir(), fmt.Sprintf(".script_%d.%s", time.Now().UnixNano(), kind))
	defer func() {
		_ = os.Remove(scriptPath)
	}()
	err = os.WriteFile(scriptPath, []byte(content), os.ModePerm)
	if err != nil {
		return 255, err
	}
	return ExecScript(ctx, scriptPath, options...)
}

func ExecScript(ctx context.Context, path string, options ...ScriptOption) (code int, err error) {
	_, err = os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 255, errors.New("script not found")
		}
		return 255, fmt.Errorf("stat script: %w", err)
	}

	if ctx == nil {
		ctx = context.Background()
	}

	s := &script{
		codeFilePath: filepath.Join(os.TempDir(), fmt.Sprintf(".%d-exitcode.txt", time.Now().UnixNano())),
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	defer func() {
		s.cancel()
		_ = os.Remove(s.codeFilePath)
	}()
	for _, option := range options {
		option(s)
	}
	code, err = s.execScript(path)
	// 优先从文件中获取
	code1, err1 := s.parseExitCodeFromFile()
	if err1 == nil {
		return code1, err
	}
	return code, err
}

func (s *script) exec(command string, args ...string) (code int, err error) {
	s.cmd = exec.CommandContext(s.ctx, command, args...)
	s.cmd.Env = append(syscall.Environ(), s.env...)
	if s.dir != "" {
		s.cmd.Dir = s.dir
	}
	s.beforeExec()
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

	if err != nil {
		code = 255
	}
	if s.cmd.ProcessState != nil {
		code = s.cmd.ProcessState.ExitCode()
	}
	return code, err
}

func (s *script) consoleOutput(title string, reader io.ReadCloser) {
	defer func() {
		if r := recover(); r != nil {
			xlog.Errorf("consoleOutput panic:%v\n%s", r, string(debug.Stack()))
		}
		xlog.Infof("stop %s console print", title)
	}()

	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxCapacity)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = s.transform(line)
		xlog.Infof("[%s] %s", title, line)
	}
}

// parseExitCodeFromFile 按行读取文件，获取到退出码就结束
func (s *script) parseExitCodeFromFile() (int, error) {
	file, err := os.Open(s.codeFilePath)
	if err != nil {
		return 255, fmt.Errorf("open code file error: %v", err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	scanner := bufio.NewScanner(file)
	// 逐行读取
	for scanner.Scan() {
		line := scanner.Bytes()

		// 去除首尾空白
		trimmed := bytes.TrimSpace(line)

		// 跳过空行
		if len(trimmed) == 0 {
			continue
		}

		// 尝试转换为整数
		if exitCode, err := strconv.Atoi(string(trimmed)); err == nil {
			return exitCode, nil
		}

		// 如果转换失败，尝试只提取数字
		digitOnly := bytes.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, trimmed)

		if len(digitOnly) > 0 {
			if exitCode, err := strconv.Atoi(string(digitOnly)); err == nil {
				return exitCode, nil
			}
		}
	}

	// 检查扫描错误
	if err = scanner.Err(); err != nil {
		return 255, fmt.Errorf("scan file error: %v", err)
	}

	// 文件为空或没有有效数字
	return 255, fmt.Errorf("no valid exit code found in file")
}
