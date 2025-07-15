package utils

import (
	"os"
)

func GetRootDir() string {
	// 使用os.Getwd()获取当前工作目录, 这在`go run`时更可靠
	wd, err := os.Getwd()
	if err != nil {
		// 如果获取失败，可以记录一个错误或者panic
		// 为了简单起见，我们回退到一个相对路径，但在实际应用中应更稳健地处理
		return "."
	}
	return wd
}
