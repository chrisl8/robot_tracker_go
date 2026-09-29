//go:build gocv

package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

func getLocalIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "unknown"
	}

	preferredRanges := []string{"192.168.", "10."}
	skipPrefixes := []string{"docker0", "br-", "veth", "vmnet", "virbr"}

	var preferredIP string
	var fallbackIP string

	for _, iface := range interfaces {
		skip := false
		for _, prefix := range skipPrefixes {
			if strings.HasPrefix(iface.Name, prefix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
				ipStr := ipNet.IP.String()
				if ipNet.IP.IsLoopback() {
					continue
				}

				isPrivate := false
				for _, pref := range preferredRanges {
					if strings.HasPrefix(ipStr, pref) {
						isPrivate = true
						break
					}
				}

				if isPrivate {
					if strings.HasPrefix(ipStr, "192.168.") {
						return ipStr
					}
					if preferredIP == "" {
						preferredIP = ipStr
					}
				} else if fallbackIP == "" {
					fallbackIP = ipStr
				}
			}
		}
	}

	if preferredIP != "" {
		return preferredIP
	}
	if fallbackIP != "" {
		return fallbackIP
	}
	return "unknown"
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

func getWebUIURLs(port string) string {
	localIP := getLocalIP()
	hostname := getHostname()
	return fmt.Sprintf("\n\n---------------------------------\n--     Web UI started at:\n--     http://localhost:%s\n--     http://%s:%s\n--     http://%s:%s\n---------------------------------\n\n", port, localIP, port, hostname, port)
}
