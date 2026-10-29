package toolbox

// orDefaultInt 把 0 视为「未设置」，用于端口等允许省略的参数。
func orDefaultInt(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}
