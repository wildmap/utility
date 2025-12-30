package utility

// Flag 标记
type Flag uint64

// Set 设置状态
func (flag *Flag) Set(f Flag) {
	*flag |= f
}

// Clean 清除状态
func (flag *Flag) Clean(f Flag) {
	*flag = flag.Exclude(f)
}

// Include 判断是否在指定状态
func (flag *Flag) Include(exp Flag) bool {
	return (*flag & exp) == exp
}

// IncludeAny 判断是否在指定状态中的其中一个
func (flag *Flag) IncludeAny(exp Flag) bool {
	return (*flag & exp) != 0
}

// Exclude 排除某些状态
func (flag *Flag) Exclude(s Flag) Flag {
	return *flag &^ s
}

// Equal 判断俩个是否相等
func (flag *Flag) Equal(s Flag) bool {
	return *flag == s
}
