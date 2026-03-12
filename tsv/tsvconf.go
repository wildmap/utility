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

// TSV 配置表格式规范：
//
//	第 1 行：表头（字段名，与结构体 json tag 对应）
//	第 2 行：字段中文说明（仅文档用途，解析时跳过）
//	第 3 行：字段注释（仅文档用途，解析时跳过）
//	第 4 行起：数据行

// Conf 表示单个 TSV 配置表，泛型参数 K 为主键类型，V 为行数据结构体类型。
//
// 线程安全：通过 sync.Mutex 保护 records 并发读写，
// Reload 操作先将新数据解析到临时 map，解析成功后原子替换，
// 保证热重载期间不会出现读到半更新状态的数据。
type Conf[K cmp.Ordered, V any] struct {
	mu      sync.Mutex // 保护 records 的并发读写
	dir     string     // TSV 文件所在目录
	name    string     // 表名（与文件名一致，不含扩展名）
	records map[K]V    // ID → 行数据的索引
}

// New 根据泛型参数 V 推导结构体名称并自动查找对应 TSV 文件。
//
// 文件路径规则：{dir}/{V的类型名}.tsv，例如 V 为 *ItemConf，则加载 {dir}/ItemConf.tsv。
// 创建时立即执行一次 Reload，若文件不存在或格式错误则返回 error。
// V 必须为结构体类型或结构体指针类型，否则返回错误。
func New[K cmp.Ordered, V any](dir string) (*Conf[K, V], error) {
	var zero V
	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Pointer {
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

// Name 返回配置表名称。
func (tsv *Conf[K, V]) Name() string {
	return tsv.name
}

// Reload 重新从文件加载配置数据，支持热更新。
//
// 采用"解析到临时 map → 成功后原子替换"的两阶段更新策略，
// 保证重载失败时原有数据不被破坏，业务逻辑不受影响。
func (tsv *Conf[K, V]) Reload() error {
	file, err := os.Open(filepath.Join(tsv.dir, tsv.name+".tsv"))
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	newRecords := make(map[K]V)
	err = tsv.parseTsvFile(file, newRecords)
	if err != nil {
		return err
	}

	// 解析全部成功后原子替换，防止读到中间态
	tsv.mu.Lock()
	tsv.records = newRecords
	tsv.mu.Unlock()

	return nil
}

// parseTsvFile 逐行解析 TSV 文件，将数据行填充到 records 中。
//
// 跳过规则：第 1 行（表头）、第 2 行（中文说明）、第 3 行（注释）共 3 行元数据。
// 第 4 行起为数据行，每行经过字段对齐和类型解析后映射到 V 类型的实例。
func (tsv *Conf[K, V]) parseTsvFile(file *os.File, records map[K]V) error {
	scanner := bufio.NewScanner(file)
	var (
		rowindex int
		fields   []string
	)

	for scanner.Scan() {
		rowindex++
		row := strings.Split(strings.TrimSpace(scanner.Text()), "\t")

		if rowindex == 1 {
			// 第 1 行：解析字段名列表，作为后续列的 JSON key
			for _, field := range row {
				fields = append(fields, field)
			}
			if len(fields) == 0 {
				return fmt.Errorf("invalid tsv file: %s", tsv.name)
			}
			continue
		}
		if rowindex <= 3 {
			continue // 跳过第 2、3 行元数据行
		}
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

// parseColumns 将单行数据解析为 V 类型的实例并存入 records。
//
// 解析流程：
//  1. 字段对齐：列数不足时用 "NULL" 填充，防止越界
//  2. 提取并验证主键 ID
//  3. 逐列尝试 JSON 解码，失败时保留原始字符串
//  4. 将完整列数据转为 JSON，再反序列化为 V 类型
//
// 使用 JSON 中转的好处：借助 encoding/json 的类型转换能力，
// 无需为每种字段类型手写解析逻辑，自动支持嵌套 JSON 对象（如 Attributes 字段）。
func (tsv *Conf[K, V]) parseColumns(fields, values []string, records map[K]V) (err error) {
	// 字段数多于列数时补 NULL，保证逐列访问不越界
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
		// NULL 值不写入 map，使结构体字段保持零值，符合语义预期
		if strings.ToLower(str) == "null" {
			continue
		}

		var value any
		err = json.Unmarshal([]byte(str), &value)
		if err != nil {
			// JSON 解析失败（如普通字符串），直接使用原始字符串值
			value = str
		}
		item[field] = value
	}

	// 通过 JSON 中转将 map[string]any 转换为目标结构体类型
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

// parseId 从行数据中定位并解析 ID 字段（字段名不区分大小写匹配 "id"）。
//
// 同时检查 ID 是否重复，TSV 文件中 ID 字段必须全局唯一。
func (tsv *Conf[K, V]) parseId(names, row []string, records map[K]V) (K, error) {
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

	if _, exists := records[id]; exists {
		return id, fmt.Errorf("tsv %s id %v: already exists", tsv.name, id)
	}
	return id, nil
}

// NumRecord 返回当前加载的数据行数（线程安全）。
func (tsv *Conf[K, V]) NumRecord() int {
	tsv.mu.Lock()
	defer tsv.mu.Unlock()

	return len(tsv.records)
}

// Get 通过 ID 查询单行数据，ID 不存在时返回 V 的零值（线程安全）。
func (tsv *Conf[K, V]) Get(id K) V {
	tsv.mu.Lock()
	defer tsv.mu.Unlock()

	return tsv.records[id]
}

// GetAll 返回所有数据行的切片（线程安全）。
//
// 返回顺序不保证与 TSV 文件行顺序一致（map 迭代顺序随机）。
func (tsv *Conf[K, V]) GetAll() []V {
	tsv.mu.Lock()
	defer tsv.mu.Unlock()

	var results []V
	for _, line := range tsv.records {
		results = append(results, line)
	}
	return results
}

// Select 查找第一条满足过滤条件的记录，找到返回记录和 nil error，
// 未找到返回零值和 "not found" error（线程安全）。
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

// Filter 返回所有满足过滤条件的记录切片（线程安全）。
//
// 返回的切片可能为空（nil），调用方应处理空切片场景。
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
