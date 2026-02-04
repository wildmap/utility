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

	"golang.org/x/sys/windows"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

var (
	acp = windows.GetACP()
)

func (s *script) execSysScript() (code int, err error) {
	ext := filepath.Ext(s.path)
	var (
		command        string
		args           []string
		wrapperPath    string
		wrapperContent string
	)
	switch ext {
	case ".ps1", ".ps":
		command = "powershell.exe"
		args = []string{"-NoLogo", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File"}
		wrapperPath = filepath.Join(os.TempDir(), s.randomFilename("wrapper", ".ps1"))
		wrapperContent = fmt.Sprintf(`$ErrorActionPreference='Continue'
try {
    & "%s" @args
    if ($LASTEXITCODE -eq $null) {
        $exitCode = 0
    } else {
        $exitCode = $LASTEXITCODE
    }
} catch {
    $exitCode = 255
    Write-Error "[$($_.Exception.GetType().FullName)] $($_.Exception.Message)"
} finally {
    if (-not (Test-Path "$env:XEXEC_EXIT_CODE_FILE")) {
        $exitCode | Out-File -FilePath "$env:XEXEC_EXIT_CODE_FILE" -Encoding ASCII
    }
    exit $exitCode
}`, s.path)
	default:
		command = "cmd.exe"
		args = []string{"/C"}
		wrapperPath = filepath.Join(os.TempDir(), s.randomFilename("wrapper", ".bat"))
		wrapperContent = fmt.Sprintf(`@echo off
call "%s" %%*
set exitcode=%%ERRORLEVEL%%
if not exist "%%XEXEC_EXIT_CODE_FILE%%" (
    echo %%exitcode%% > "%%XEXEC_EXIT_CODE_FILE%%"
)
exit /b %%exitcode%%`, s.path)
	}
	defer func() {
		_ = os.Remove(wrapperPath)
	}()
	if err = os.WriteFile(wrapperPath, []byte(wrapperContent), os.ModePerm); err != nil {
		return 255, err
	}
	// 将包装脚本路径和用户参数合并传递
	args = append(args, wrapperPath)
	args = append(args, s.args...)
	return s.exec(command, args...)
}

func (s *script) beforeExec() {
	s.cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
	s.cmd.Cancel = func() error {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Errorf("[SYSTEM] cmd cancel panic: %v\n%s", r, string(debug.Stack()))
			}
			_ = os.WriteFile(s.exitCodePath, []byte("137"), os.ModePerm)
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
	// 当系统代码页为936(GBK)或检测到GBK编码时，转换为UTF-8
	if acp == 936 || s.isGBK(line) {
		return s.gbkToUtf8(line)
	}
	return line
}

func (s *script) gbkToUtf8(line string) string {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Errorf("[SYSTEM] gbkToUtf8 panic: %v\n%s", r, string(debug.Stack()))
		}
	}()

	reader := transform.NewReader(strings.NewReader(line), simplifiedchinese.GBK.NewDecoder())
	b, err := io.ReadAll(reader)
	if err != nil {
		s.logger.Errorf("[SYSTEM] gbkToUtf8 error: %v", err)
		return line
	}
	return string(b)
}

func (s *script) isGBK(data string) bool {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Errorf("[SYSTEM] isGBK panic: %v\n%s", r, string(debug.Stack()))
		}
	}()

	length := len(data)
	hasMultiByte := false // 标记是否包含多字节字符

	for i := 0; i < length; {
		b := data[i]

		// ASCII字符，直接跳过
		if b <= 0x7f {
			i++
			continue
		}

		// 边界检查
		if i+1 >= length {
			return false
		}

		// GBK 双字节检测
		// 第一字节范围: 0x81-0xFE
		// 第二字节范围: 0x40-0xFE (排除0x7F)
		b2 := data[i+1]
		if b >= 0x81 && b <= 0xfe && b2 >= 0x40 && b2 <= 0xfe && b2 != 0x7f {
			hasMultiByte = true
			i += 2
			continue
		}

		// 不符合GBK编码规则
		return false
	}

	// 只有当包含多字节字符时才返回true，避免纯ASCII被误判为GBK
	return hasMultiByte
}
