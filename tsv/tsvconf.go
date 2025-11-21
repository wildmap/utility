package tsv

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
)

/* tsv配置表格式
第1行表头, 列格式: {用途}_{字段名}
第2-9行, 其他用途的数据
第10行开始, 数据行
*/

// Conf 表示单个tsv配置表
type Conf[T any] struct {
	mu      sync.Mutex // 锁
	source  string     // 文件路径
	records []T        // 所有记录行

}

// New 根据struct原型创建TSvConf
func New[T any](dir string) (*Conf[T], error) {
	t := reflect.TypeOf(*new(T))
	if t.Kind() != reflect.Struct && t.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("st must be a struct or struct ptr")
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	tsv := &Conf[T]{
		source:  filepath.Join(dir, fmt.Sprintf("%s.tsv", t.Name())),
		records: make([]T, 0),
	}

	return tsv, tsv.Reload()
}

// Reload 从文件中重新读取并填充配置表数据
func (tsv *Conf[T]) Reload() error {
	tsv.mu.Lock()
	defer tsv.mu.Unlock()

	file, err := os.Open(tsv.source)
	if err != nil {
		return fmt.Errorf("open file error: %w", err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	scanner := bufio.NewScanner(file)
	var (
		rowindex int
		names    []string
		records  []T
	)

	for scanner.Scan() {
		rowindex++
		row := strings.Split(strings.TrimSpace(scanner.Text()), "\t")

		// 处理表头
		if rowindex == 1 {
			for _, field := range row {
				_, after, found := strings.Cut(field, "_")
				if !found {
					return fmt.Errorf("invalid tsv file: %s", tsv.source)
				}
				names = append(names, after)
			}
			if len(names) == 0 {
				return fmt.Errorf("invalid tsv file: %s", tsv.source)
			}
			continue
		}
		if rowindex <= 9 {
			continue
		}
		// 处理数据
		item, err := tsv.parseColumns(names, row)
		if err != nil {
			return fmt.Errorf("parse row %d error %w", rowindex, err)
		}
		records = append(records, item)
	}

	if err = scanner.Err(); err != nil {
		return fmt.Errorf("scan file error %w", err)
	}
	tsv.records = records
	return nil
}

// 解析列
func (tsv *Conf[T]) parseColumns(names, row []string) (res T, err error) {
	var item = make(map[string]any)
	for idx, name := range names {
		var filed any
		err = json.Unmarshal([]byte(row[idx]), &filed)
		if err != nil {
			// 直接使用字符串?
			filed = row[idx]
		}
		item[name] = filed
	}
	data, err := json.Marshal(item)
	if err != nil {
		return res, err
	}
	err = json.Unmarshal(data, &res)
	if err != nil {
		return res, err
	}
	return
}

// NumRecord 返回tsv行数
func (tsv *Conf[T]) NumRecord() int {
	tsv.mu.Lock()
	defer tsv.mu.Unlock()

	return len(tsv.records)
}

// Get 通过Index Key读取单行记录
func (tsv *Conf[T]) Get(idx int) (T, error) {
	tsv.mu.Lock()
	defer tsv.mu.Unlock()

	if idx < 0 || idx >= len(tsv.records) {
		var zero T
		return zero, fmt.Errorf("not found")
	}
	return tsv.records[idx], nil
}

// Select 筛选符合条件的记录，只返回第一个匹配的数据
func (tsv *Conf[T]) Select(filter func(line T) bool) (T, error) {
	tsv.mu.Lock()
	defer tsv.mu.Unlock()

	for _, line := range tsv.records {
		if filter(line) {
			return line, nil
		}
	}
	var zero T
	return zero, fmt.Errorf("not found")
}

// Filter 筛选符合条件的记录，返回所有匹配的数据
func (tsv *Conf[T]) Filter(filter func(line T) bool) []T {
	tsv.mu.Lock()
	defer tsv.mu.Unlock()

	var results []T
	for _, line := range tsv.records {
		if filter(line) {
			results = append(results, line)
		}
	}
	return results
}
