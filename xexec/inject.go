package xexec

import (
	"fmt"
	"os"
	"regexp"
)

var (
	// variablePlaceholder 统一匹配自定义风格(%VAR%)和Shell风格($VAR或${VAR})
	// Group 1: Custom %VAR%
	// Group 2: Shell ${VAR}
	// Group 3: Shell $VAR
	// 均限制变量名必须以字母或下划线开头
	variablePlaceholder = regexp.MustCompile(`%([A-Za-z_][A-Za-z0-9_-]*)%|\$\{([A-Za-z_][A-Za-z0-9_-]*)\}|\$([A-Za-z_][A-Za-z0-9_-]*)`)
)

// Inject 替换内容中的占位符变量
// 支持三种占位符风格:
// 替换变量占位符 (%VAR%, $VAR, ${VAR})
func Inject(content string, envs map[string]any) string {
	if envs == nil {
		envs = make(map[string]any)
	}

	// 提取重复的查找逻辑
	lookup := func(name string) (string, bool) {
		if val, ok := envs[name]; ok {
			return fmt.Sprintf("%v", val), true
		}
		if val, exists := os.LookupEnv(name); exists {
			return val, true
		}
		return "", false
	}

	// 替换变量占位符 (%VAR%, $VAR, ${VAR})
	content = variablePlaceholder.ReplaceAllStringFunc(content, func(match string) string {
		submatches := variablePlaceholder.FindStringSubmatch(match)
		// 快速提取变量名：遍历子匹配，找到第一个非空的组
		var name string
		for i := 1; i < len(submatches); i++ {
			if submatches[i] != "" {
				name = submatches[i]
				break
			}
		}

		if name == "" {
			return match
		}

		if val, found := lookup(name); found {
			return val
		}

		return match
	})

	return content
}
