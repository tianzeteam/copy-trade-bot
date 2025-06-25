package utils

import (
	"os"
	"path/filepath"
	"runtime"
)

func GetRootDir() string {
	// 使用os.Executable()获取当前运行的可执行文件路径
	execPath, err := os.Executable()
	if err != nil {
		// 如果获取失败，回退到使用runtime.Caller
		_, b, _, _ := runtime.Caller(0)
		return filepath.Dir(b)
	}
	// 返回可执行文件所在目录
	return filepath.Dir(execPath)
}
