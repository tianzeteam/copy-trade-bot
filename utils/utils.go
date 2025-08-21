package utils

import (
	"net"
	"os"
	"strings"
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

// GetLocalIPAddresses 获取本地所有非回环IP地址
func GetLocalIPAddresses() []string {
	var ips []string

	interfaces, err := net.Interfaces()
	if err != nil {
		return ips
	}

	for _, iface := range interfaces {
		// 跳过禁用的接口
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		
		// 跳过回环接口
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// 跳过IPv6地址
			if ip == nil || ip.To4() == nil {
				continue
			}

			ipStr := ip.String()
			// 跳过本地链路地址
			if strings.HasPrefix(ipStr, "169.254.") {
				continue
			}

			ips = append(ips, ipStr)
		}
	}

	return ips
}
