package simpchache

// 提供一种安全的方式来操作和访问字节数据，
// 通过深拷贝机制防止外部代码意外修改原始数据
// 保证数据的完整性和不可变性。


// 安全访问，不被外部修改
type ByteView struct {
	b []byte
}

// Len
func (v ByteView) Len() int {
	return len(v.b)
}

// 返回字节数据的一个副本（深拷贝）
func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

// String 
func (v ByteView) String() string {
	return string(v.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}