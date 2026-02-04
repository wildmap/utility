package xexec

import (
	"bytes"
	"io"
	"runtime/debug"
	"time"
)

// 实现了异步读取 + 100ms 超时强制刷新
func (s *script) consoleOutput(title string, reader io.ReadCloser) {
	dataChan := make(chan []byte, 16)

	go func() {
		defer close(dataChan)
		chunk := make([]byte, 64*1024) // 64KB
		for {
			n, err := reader.Read(chunk)
			if n > 0 {
				// 必须拷贝，防止数据竞争
				tmp := make([]byte, n)
				copy(tmp, chunk[:n])
				dataChan <- tmp
			}
			if err != nil {
				return
			}
		}
	}()

	defer func() {
		if r := recover(); r != nil {
			s.logger.Errorf("[SYSTEM] %s panic:%v\n%s", title, r, string(debug.Stack()))
		}
		s.logger.Infof("[SYSTEM] stop %s console print", title)
	}()

	var buf bytes.Buffer
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	flushLine := func(lineBytes []byte) {
		line := string(lineBytes)
		// 1. 先进行编码转换 (GBK -> UTF8)
		line = s.transform(line)
		// 2. === 新增：进行日志脱敏 ===
		if s.masker != nil {
			line = s.masker.Replace(line)
		}
		// 3. 输出
		s.logger.Infof("[%s] %s", title, line)
	}

	for {
		select {
		case data, ok := <-dataChan:
			if !ok {
				if buf.Len() > 0 {
					flushLine(buf.Bytes())
				}
				return
			}

			buf.Write(data)

			for {
				idx := bytes.IndexByte(buf.Bytes(), '\n')
				if idx == -1 {
					// 没有换行符，等待
					break
				}
				lineBytes := buf.Next(idx)
				// 手动去除尾部的 \r (Carriage Return)
				if len(lineBytes) > 0 && lineBytes[len(lineBytes)-1] == '\r' {
					lineBytes = lineBytes[:len(lineBytes)-1]
				}
				flushLine(lineBytes)
				_, _ = buf.ReadByte()
			}

		case <-ticker.C:
			// 超时强制刷新：如果缓冲区非空，强制输出
			// 这种机制防止了程序输出不带换行符时日志卡住的问题
			if buf.Len() > 0 {
				flushLine(buf.Bytes())
				buf.Reset()
			}

		case <-s.ctx.Done():
			return
		}
	}
}
