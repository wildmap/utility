//go:build windows

package xexec

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"github.com/wildmap/utility/xlog"
)

var acp = windows.GetACP()

func (s *script) execScript(path string) (code int, err error) {
	switch filepath.Ext(path) {
	case ".powershell", ".pwsh", ".ps1", ".ps":
		//command := fmt.Sprintf("$ErrorActionPreference='Continue';%s;exit $LASTEXITCODE", path)
		wrapperPath := filepath.Join(os.TempDir(), fmt.Sprintf("wrapper-%d.ps1", time.Now().UnixNano()))
		defer func() {
			_ = os.Remove(wrapperPath)
		}()
		wrapperContent := fmt.Sprintf(`$ErrorActionPreference='Continue';
try {
    & %s
    $exitCode = $LASTEXITCODE
} catch {
    $exitCode = 1
} finally {
    $exitCode | Out-File -FilePath '%s' -Encoding ASCII
    exit $exitCode
}`, path, s.codeFilePath)
		if err = os.WriteFile(wrapperPath, []byte(wrapperContent), os.ModePerm); err != nil {
			return 255, err
		}
		return s.exec("powershell", "-NoLogo", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", wrapperPath)
	default:
		wrapperPath := filepath.Join(os.TempDir(), fmt.Sprintf("wrapper-%d.bat", time.Now().UnixNano()))
		defer func() {
			_ = os.Remove(wrapperPath)
		}()

		wrapperContent := fmt.Sprintf(`@echo off
call %s
echo %%ERRORLEVEL%% > "%s"
exit /b %%ERRORLEVEL%%`, path, s.codeFilePath)
		if err = os.WriteFile(wrapperPath, []byte(wrapperContent), os.ModePerm); err != nil {
			return 255, err
		}
		return s.exec("cmd", "/C", wrapperPath)
	}
}

func (s *script) beforeExec() {
	s.cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
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
		kill := exec.Command("TASKKILL.exe", "/T", "/F", "/PID", strconv.Itoa(s.cmd.Process.Pid))
		if err := kill.Run(); err != nil {
			errs = errors.Join(errs, err)
		}
		if errs != nil {
			return fmt.Errorf("error cancelling pid=%d, errors=%v", s.cmd.Process.Pid, errs)
		}
		return nil
	}
}

func (s *script) transform(line string) string {
	if s.isGBK(line) || acp == 936 {
		line = s.gbkToUtf8(line)
	}
	return line
}

func (s *script) gbkToUtf8(line string) string {
	defer func() {
		if r := recover(); r != nil {
			xlog.Errorf("gbkToUtf8 panic:%v\n%s", r, string(debug.Stack()))
		}
	}()
	reader := transform.NewReader(strings.NewReader(line), simplifiedchinese.GBK.NewDecoder())
	b, err := io.ReadAll(reader)
	if err != nil {
		xlog.Errorln(err)
		return line
	}
	return string(b)
}

func (s *script) isGBK(data string) bool {
	defer func() {
		if r := recover(); r != nil {
			xlog.Errorf("isGBK panic:%v\n%s", r, string(debug.Stack()))
		}
	}()
	length := len(data)
	var i = 0
	for i < length {
		if data[i] <= 0x7f {
			// 编码0~127,只有一个字节的编码，兼容ASCII码
			i++
			continue
		} else {
			// 大于127的使用双字节编码，落在gbk编码范围内的字符
			if data[i] >= 0x81 &&
				data[i] <= 0xfe &&
				data[i+1] >= 0x40 &&
				data[i+1] <= 0xfe &&
				data[i+1] != 0xf7 {
				i += 2
				continue
			} else {
				return false
			}
		}
	}
	return true
}
