package xexec

import (
	"fmt"
	"os"
	"regexp"
)

var (
	// customPlaceholder 自定义占位符的正则表达式,匹配 %VAR% 风格
	customPlaceholder = regexp.MustCompile(`%([A-Za-z0-9_-]+)%`)
)

// Inject 替换内容中的占位符变量
// 支持两种占位符风格:
// 1. 自定义风格: %VAR%
// 2. Shell风格: $VAR 或 ${VAR}
// 参数: content - 包含占位符的内容字符串, envs - 环境变量映射表
// 返回: 替换后的字符串
func Inject(content string, envs map[string]interface{}) string {
	// 提取重复的查找逻辑
	lookup := func(name string) string {
		if val, ok := envs[name]; ok {
			return fmt.Sprintf("%v", val)
		}
		if val, exists := os.LookupEnv(name); exists {
			return val
		}
		return ""
	}

	// 首先,替换所有 %VAR% 占位符
	content = customPlaceholder.ReplaceAllStringFunc(content, func(match string) string {
		submatches := customPlaceholder.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		return lookup(submatches[1])
	})

	// 然后,使用os.Expand替换Shell风格的占位符
	return os.Expand(content, lookup)
}
