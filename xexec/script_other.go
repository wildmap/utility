//go:build !windows

package xexec

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/wildmap/utility/xlog"
)

func (s *script) execScript(path string) (code int, err error) {
	_ = os.Chmod(path, os.ModePerm)
	wrapperPath := filepath.Join(os.TempDir(), fmt.Sprintf(".wrapper-%d.sh", time.Now().UnixNano()))
	defer func() {
		_ = os.Remove(wrapperPath)
	}()
	wrapperContent := fmt.Sprintf(`
trap 'exitcode=$?; echo $exitcode > "%s"; exit $exitcode' EXIT INT TERM ERR;
%s
`, s.codeFilePath, path)
	if err = os.WriteFile(wrapperPath, []byte(wrapperContent), os.ModePerm); err != nil {
		return 255, err
	}
	code, err = s.exec("sh", "-c", wrapperPath)
	return code, err
}

func (s *script) beforeExec() {
	s.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGKILL,
	}
	s.cmd.Cancel = func() error {
		defer func() {
			if r := recover(); r != nil {
				xlog.Errorln("cmd cancel panic: %v\n%s", r, string(debug.Stack()))
			}
			_ = os.WriteFile(s.codeFilePath, []byte("137"), os.ModePerm)
		}()
		if s.cmd.Process == nil {
			return fmt.Errorf("process not started")
		}
		var errs error
		if err := s.cmd.Process.Kill(); err != nil {
			errs = errors.Join(errs, err)
		}
		if err := syscall.Kill(-s.cmd.Process.Pid, syscall.SIGKILL); err != nil {
			errs = errors.Join(errs, err)
		}
		if errs != nil {
			return fmt.Errorf("error cancelling pid=%d, errors=%v", s.cmd.Process.Pid, errs)
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
