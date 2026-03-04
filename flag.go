package utility

// Flag 基于 uint64 的高性能位标记管理器。
//
// 利用位运算在单个 uint64 值中同时管理最多 64 个布尔标志，
// 所有操作均为 O(1) 时间复杂度，内存占用仅 8 字节。
// 相比 64 个独立 bool 字段节省 56 字节，且缓存友好性更佳。
//
// 使用场景：
//   - 游戏角色状态管理（眩晕、无敌、沉默、隐身等）
//   - 权限系统的权限位组合管理
//   - 配置选项的功能开关组合
//   - 网络协议头部的标志位字段
//
// 标志位定义惯例：
//
//	const (
//	    FlagRead   Flag = 1 << 0  // 0x01
//	    FlagWrite  Flag = 1 << 1  // 0x02
//	    FlagExec   Flag = 1 << 2  // 0x04
//	)
//
// 注意事项：
//   - 标志位值应使用 1 << n 的形式定义（n: 0~63）
//   - 避免使用 0 作为标志位，0 在位运算中无任何效果
//   - 多个标志位可通过 | 运算符组合使用
type Flag uint64

// Set 设置一个或多个标志位（将对应位置 1）。
//
// 使用位或赋值（|=）实现，仅影响目标标志位，不改变其他位的状态。
// 重复设置已置位的标志是幂等操作，不会产生副作用。
func (flag *Flag) Set(f Flag) {
	*flag |= f
}

// Clean 清除一个或多个标志位（将对应位置 0）。
//
// 内部委托给 Exclude 实现，使用位清除运算（&^），
// 仅清除目标标志位，不影响其他位的状态。
func (flag *Flag) Clean(f Flag) {
	*flag = flag.Exclude(f)
}

// Include 判断是否同时包含所有指定标志位（AND 语义）。
//
// 通过位与运算 (flag & exp) == exp 实现全量匹配，
// exp 中的每一个标志位都必须在 flag 中置位才返回 true。
//
// 典型用途：验证用户是否同时拥有一组权限。
func (flag *Flag) Include(exp Flag) bool {
	return (*flag & exp) == exp
}

// IncludeAny 判断是否包含至少一个指定标志位（OR 语义）。
//
// 通过位与运算 (flag & exp) != 0 实现任意匹配，
// exp 中只要有一个标志位在 flag 中置位即返回 true。
//
// 典型用途：验证用户是否拥有候选权限集合中的任意一个。
func (flag *Flag) IncludeAny(exp Flag) bool {
	return (*flag & exp) != 0
}

// Exclude 返回清除指定标志位后的新 Flag 值，不修改原值。
//
// 使用位清除运算（&^，Go 特有的 AND NOT 运算符）生成新值，
// 原 Flag 值保持不变，适合在链式操作或临时计算中使用。
// 如需就地修改，请使用 [Flag.Clean] 方法。
func (flag *Flag) Exclude(s Flag) Flag {
	return *flag &^ s
}

// Equal 判断两个 Flag 值是否完全相同（逐位比较）。
//
// 进行精确的全量匹配，所有 64 位都必须相同才返回 true。
// 与 Include 的包含关系检查不同，此方法不允许额外位的存在。
func (flag *Flag) Equal(s Flag) bool {
	return *flag == s
}
