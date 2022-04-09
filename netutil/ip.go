/*
	netutil包实现一些网络工具, 例如IP地址的判断处理等
*/
package netutil

import (
	"net"
)

// 是否本机IP
func IsLocalHostIP(ipstr string) bool {
	ip := net.ParseIP(ipstr)
	if ip == nil {
		return false
	}
	ipAddrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, ipAddr := range ipAddrs {
		// 检查ip地址判断是否回环地址
		if ipNet, ok := ipAddr.(*net.IPNet); ok {
			if ipNet.IP.Equal(ip) {
				return true
			}
		}
	}
	return false
}

// 获取本地ip地址
func GetLocalAddr() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unKnown"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if ipnet.IP.IsLoopback() {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil {
				continue
			}
			return ipnet.IP.String()
		}
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return "unKnown"
}
