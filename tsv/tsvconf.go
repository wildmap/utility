package tsv

import (
	"bufio"
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
)

/* tsv配置表格式
第1行表头, 字段名
第2行, 字段中文名
第3行, 字段注释
第4行开始, 数据行
*/

// Conf 表示单个tsv配置表
type Conf[K cmp.Ordered, V any] struct {
	mu      sync.Mutex // 锁
	dir     string     // 文件路径
	name    string     // 表名/文件名
	records map[K]V    // 所有记录行
}

// New 根据struct原型创建Tsv
func New[K cmp.Ordered, V any](dir string) (*Conf[K, V], error) {
	var zero V
	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("st must be a struct or struct ptr")
	}

	tsv := &Conf[K, V]{
		dir:     dir,
		name:    t.Name(),
		records: make(map[K]V),
	}

	return tsv, tsv.Reload()
}

func (tsv *Conf[K, V]) Name() string {
	return tsv.name
}

// Reload 从文件中重新读取并填充配置表数据
func (tsv *Conf[K, V]) Reload() error {
	file, err := os.Open(filepath.Join(tsv.dir, tsv.name+".tsv"))
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	// 先解析到临时map，确保原子性
	newRecords := make(map[K]V)
	err = tsv.parseTsvFile(file, newRecords)
	if err != nil {
		return err
	}

	// 解析成功后原子替换
	tsv.mu.Lock()
	tsv.records = newRecords
	tsv.mu.Unlock()

	return nil
}

func (tsv *Conf[K, V]) parseTsvFile(file *os.File, records map[K]V) error {
	scanner := bufio.NewScanner(file)
	var (
		rowindex int
		fields   []string
	)

	for scanner.Scan() {
		rowindex++
		row := strings.Split(strings.TrimSpace(scanner.Text()), "\t")

		// 处理表头
		if rowindex == 1 {
			for _, field := range row {
				fields = append(fields, field)
			}
			if len(fields) == 0 {
				return fmt.Errorf("invalid tsv file: %s", tsv.name)
			}
			continue
		}
		if rowindex <= 3 {
			continue
		}
		// 处理数据
		err := tsv.parseColumns(fields, row, records)
		if err != nil {
			return fmt.Errorf("parse row %d error %w", rowindex, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan file error %w", err)
	}
	return nil
}

// 解析列
func (tsv *Conf[K, V]) parseColumns(fields, values []string, records map[K]V) (err error) {
	if len(fields) != len(values) {
		for i := len(values); i < len(fields); i++ {
			values = append(values, "NULL")
		}
	}
	id, err := tsv.parseId(fields, values, records)
	if err != nil {
		return err
	}

	var item = make(map[string]any)
	for idx, field := range fields {
		str := values[idx]
		// NULL值不添加到map中，让JSON解码器使用结构体的零值
		if strings.ToLower(str) == "null" {
			continue
		}

		var value any
		err = json.Unmarshal([]byte(str), &value)
		if err != nil {
			// 直接使用字符串
			value = str
		}
		item[field] = value
	}

	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	var res V
	err = json.Unmarshal(data, &res)
	if err != nil {
		return err
	}
	records[id] = res

	return
}

func (tsv *Conf[K, V]) parseId(names, row []string, records map[K]V) (K, error) {
	// 找到 id 列的索引
	var id K
	var idIndex = -1
	for idx, name := range names {
		if strings.ToLower(name) == "id" {
			idIndex = idx
			break
		}
	}
	if idIndex < 0 {
		return id, fmt.Errorf("tsv %s missing id field", tsv.name)
	}

	idStr := row[idIndex]
	err := json.Unmarshal([]byte(idStr), &id)
	if err != nil {
		return id, fmt.Errorf("tsv %s id parse error: %w", tsv.name, err)
	}

	// 检查重复
	if _, exists := records[id]; exists {
		return id, fmt.Errorf("tsv %s id %v: already exists", tsv.name, id)
	}
	return id, nil
}

// NumRecord 返回tsv行数
func (tsv *Conf[K, V]) NumRecord() int {
	tsv.mu.Lock()
	defer tsv.mu.Unlock()

	return len(tsv.records)
}

// Get 通过Index Key读取单行记录
func (tsv *Conf[K, V]) Get(id K) V {
	tsv.mu.Lock()
	defer tsv.mu.Unlock()

	return tsv.records[id]
}

func (tsv *Conf[K, V]) GetAll() []V {
	tsv.mu.Lock()
	defer tsv.mu.Unlock()

	var results []V
	for _, line := range tsv.records {
		results = append(results, line)
	}
	return results
}

// Select 筛选符合条件的记录，只返回第一个匹配的数据
func (tsv *Conf[K, V]) Select(filter func(line V) bool) (V, error) {
	tsv.mu.Lock()
	defer tsv.mu.Unlock()

	for _, line := range tsv.records {
		if filter(line) {
			return line, nil
		}
	}
	var zero V
	return zero, fmt.Errorf("not found")
}

// Filter 筛选符合条件的记录，返回所有匹配的数据
func (tsv *Conf[K, V]) Filter(filter func(line V) bool) []V {
	tsv.mu.Lock()
	defer tsv.mu.Unlock()

	var results []V
	for _, line := range tsv.records {
		if filter(line) {
			results = append(results, line)
		}
	}
	return results
}
