package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
//go:embed static/*
var staticFiles embed.FS

// GetStaticFS 返回嵌入的静态文件系统
func GetStaticFS() http.FileSystem {
	return http.FS(staticFiles)
}

// GetIndexFileSystem 返回用于提供index.html的文件系统
func GetIndexFileSystem() http.FileSystem {
	// 从嵌入的文件系统中提取 static 子目录
	subFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	return http.FS(subFS)
}
