package utility

// Flag 位标记管理器
//
// 基于 uint64 类型实现的高性能位标记系统，支持同时管理64个布尔标志。
// 使用位运算实现，性能优异，内存占用仅8字节。
//
// 设计特点：
//   - 内存高效：64个布尔标志仅占用8字节（相比64个bool节省56字节）
//   - 性能优异：所有操作都是O(1)时间复杂度的位运算
//   - 类型安全：通过 Flag 类型包装，避免直接操作整数
//
// 使用场景：
//   - 游戏中的角色状态管理（眩晕、无敌、沉默等）
//   - 权限系统的权限位管理
//   - 配置选项的开关组合
//   - 网络协议的标志位字段
//
// 示例用法：
//
//	const (
//	    FlagRead   Flag = 1 << 0  // 0x01
//	    FlagWrite  Flag = 1 << 1  // 0x02
//	    FlagExec   Flag = 1 << 2  // 0x04
//	)
//
//	var permissions Flag
//	permissions.Set(FlagRead | FlagWrite)  // 设置读写权限
//	if permissions.Include(FlagWrite) {     // 检查写权限
//	    // 执行写操作
//	}
//
// 注意事项：
//   - 标志位定义建议使用 1 << n 的形式（n: 0-63）
//   - 避免使用0作为标志位，因为它在位运算中不起作用
//   - 标志位可以通过 | 运算符组合使用
type Flag uint64

// Set 设置指定的一个或多个标志位
//
// 操作：将指定标志位置为1，不影响其他位
// 位运算：flag = flag | f
//
// 参数：
//
//	f - 要设置的标志位，可以是单个标志或多个标志的组合（使用 | 连接）
//
// 示例：
//
//	var flags Flag
//	flags.Set(Flag1)              // 设置单个标志
//	flags.Set(Flag1 | Flag2)      // 同时设置多个标志
//	flags.Set(Flag1)
//	flags.Set(Flag2)              // 分步设置，效果同上
//
// 注意：
//   - 重复设置同一标志位是安全的，不会产生副作用
//   - 此方法会直接修改原 Flag 值
func (flag *Flag) Set(f Flag) {
	*flag |= f
}

// Clean 清除指定的一个或多个标志位
//
// 操作：将指定标志位置为0，不影响其他位
// 位运算：flag = flag &^ f
//
// 参数：
//
//	f - 要清除的标志位，可以是单个标志或多个标志的组合
//
// 示例：
//
//	flags.Clean(Flag1)            // 清除单个标志
//	flags.Clean(Flag1 | Flag2)    // 同时清除多个标志
//
// 注意：
//   - 清除未设置的标志位是安全的，不会产生副作用
//   - 此方法会直接修改原 Flag 值
func (flag *Flag) Clean(f Flag) {
	*flag = flag.Exclude(f)
}

// Include 判断是否包含所有指定的标志位（AND 逻辑）
//
// 检查逻辑：exp 中的每一个标志位都必须在 flag 中被设置
// 位运算：(flag & exp) == exp
//
// 参数：
//
//	exp - 期望包含的标志位组合
//
// 返回值：
//
//	true  - flag 包含 exp 中的所有标志位
//	false - flag 缺少 exp 中的至少一个标志位
//
// 示例：
//
//	flags := Flag1 | Flag2 | Flag3
//	flags.Include(Flag1)               // true，包含 Flag1
//	flags.Include(Flag1 | Flag2)       // true，包含 Flag1 和 Flag2
//	flags.Include(Flag1 | Flag4)       // false，缺少 Flag4
//	flags.Include(0)                   // true，空集是任何集合的子集
//
// 应用场景：
//   - 权限检查：验证用户是否同时拥有多个权限
//   - 状态检查：验证对象是否同时处于多个状态
func (flag *Flag) Include(exp Flag) bool {
	return (*flag & exp) == exp
}

// IncludeAny 判断是否包含任意一个指定的标志位（OR 逻辑）
//
// 检查逻辑：exp 中只要有任意一个标志位在 flag 中被设置即可
// 位运算：(flag & exp) != 0
//
// 参数：
//
//	exp - 候选标志位组合
//
// 返回值：
//
//	true  - flag 至少包含 exp 中的一个标志位
//	false - flag 不包含 exp 中的任何标志位
//
// 示例：
//
//	flags := Flag1 | Flag2
//	flags.IncludeAny(Flag1)            // true，包含 Flag1
//	flags.IncludeAny(Flag3 | Flag4)    // false，都不包含
//	flags.IncludeAny(Flag1 | Flag3)    // true，包含 Flag1
//	flags.IncludeAny(0)                // false，空集合
//
// 应用场景：
//   - 权限检查：验证用户是否拥有多个权限中的至少一个
//   - 状态检查：验证对象是否处于多个状态中的任意一个
func (flag *Flag) IncludeAny(exp Flag) bool {
	return (*flag & exp) != 0
}

// Exclude 返回移除指定标志位后的新值（不修改原值）
//
// 操作：生成一个新的 Flag，其中不包含指定的标志位
// 位运算：result = flag &^ s
//
// 参数：
//
//	s - 要排除的标志位
//
// 返回值：
//
//	排除指定标志位后的新 Flag 值
//
// 示例：
//
//	flags := Flag1 | Flag2 | Flag3
//	newFlags := flags.Exclude(Flag2)   // newFlags = Flag1 | Flag3
//	// flags 保持不变，仍为 Flag1 | Flag2 | Flag3
//
// 注意：
//   - 此方法不会修改原 Flag 值，返回新值
//   - 排除未设置的标志位是安全的，返回值与原值相同
//   - 如需直接修改原值，请使用 Clean 方法
func (flag *Flag) Exclude(s Flag) Flag {
	return *flag &^ s
}

// Equal 判断两个 Flag 是否完全相等
//
// 比较逻辑：逐位比较，所有64位都必须相同
//
// 参数：
//
//	s - 要比较的 Flag 值
//
// 返回值：
//
//	true  - 两个 Flag 的所有标志位都相同
//	false - 至少有一个标志位不同
//
// 示例：
//
//	flags1 := Flag1 | Flag2
//	flags2 := Flag1 | Flag2
//	flags3 := Flag1 | Flag3
//	flags1.Equal(flags2)  // true，完全相同
//	flags1.Equal(flags3)  // false，Flag2 和 Flag3 不同
//
// 注意：
//   - 此方法进行的是精确比较，不是包含关系检查
//   - 如需检查包含关系，请使用 Include 或 IncludeAny 方法
func (flag *Flag) Equal(s Flag) bool {
	return *flag == s
}
