//go:build !windows

package xexec

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"syscall"
)

func (s *script) execSysScript() (code int, err error) {
	_ = os.Chmod(s.path, os.ModePerm)
	wrapperContent := fmt.Sprintf(`#!/bin/sh
_save_exit_code() {
	local code=$?
	[ ! -f "${XEXEC_EXIT_CODE_FILE}" ] && echo $code > "${XEXEC_EXIT_CODE_FILE}"
	exit $code
}
trap _save_exit_code EXIT INT TERM
"%s" "$@"
`, s.path)

	wrapperPath := filepath.Join(os.TempDir(), s.randomFilename("wrapper", ".sh"))
	defer func() {
		_ = os.Remove(wrapperPath)
	}()

	if err = os.WriteFile(wrapperPath, []byte(wrapperContent), os.ModePerm); err != nil {
		return 255, fmt.Errorf("failed to create wrapper script: %w", err)
	}

	// 将包装脚本路径和用户参数合并传递给 sh
	args := append([]string{wrapperPath}, s.args...)
	return s.exec("sh", args...)
}

func (s *script) beforeExec() {
	// 设置进程组，便于杀死整个进程树
	s.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    os.Getpid(),
	}

	s.cmd.Cancel = func() error {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Errorf("[SYSTEM] cmd cancel panic: %v\n%s", r, string(debug.Stack()))
			}
			// 写入退出码137 (SIGKILL)
			_ = os.WriteFile(s.exitCodePath, []byte("137"), os.ModePerm)
		}()

		if s.cmd.Process == nil {
			return fmt.Errorf("process not started")
		}

		var errs error
		pid := s.cmd.Process.Pid

		// 尝试杀死进程本身
		if err := s.cmd.Process.Kill(); err != nil {
			errs = errors.Join(errs, fmt.Errorf("kill process: %w", err))
		}

		// 尝试杀死整个进程组
		if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
			errs = errors.Join(errs, fmt.Errorf("kill process group: %w", err))
		}

		if errs != nil {
			return fmt.Errorf("error cancelling pid=%d: %w", pid, errs)
		}
		return nil
	}
}

func (s *script) utf8ToGb2312(line string) string {
	return line
}

func (s *script) transform(line string) string {
	return line
}
