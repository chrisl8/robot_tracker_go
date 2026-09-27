//go:build darwin || linux

package main

import (
	"encoding/binary"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// systemLoadAverage returns the 1-minute load average of the whole machine,
// which shows CPU contention from other processes.
func systemLoadAverage() (float64, bool) {
	switch runtime.GOOS {
	case "linux":
		data, err := os.ReadFile("/proc/loadavg")
		if err != nil {
			return 0, false
		}
		fields := strings.Fields(string(data))
		if len(fields) == 0 {
			return 0, false
		}
		v, err := strconv.ParseFloat(fields[0], 64)
		return v, err == nil
	case "darwin":
		// struct loadavg { uint32 ldavg[3]; int64 fscale; } (with padding).
		raw, err := syscall.Sysctl("vm.loadavg")
		if err != nil {
			return 0, false
		}
		b := []byte(raw)
		// syscall.Sysctl drops one trailing NUL, which can be part of fscale.
		for len(b) < 24 {
			b = append(b, 0)
		}
		fscale := binary.LittleEndian.Uint64(b[16:24])
		if fscale == 0 {
			return 0, false
		}
		return float64(binary.LittleEndian.Uint32(b[0:4])) / float64(fscale), true
	}
	return 0, false
}

// processCPUSeconds is this process's cumulative user+system CPU time.
func processCPUSeconds() float64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return float64(ru.Utime.Sec) + float64(ru.Utime.Usec)/1e6 +
		float64(ru.Stime.Sec) + float64(ru.Stime.Usec)/1e6
}

// topCPUProcesses names the busiest other processes right now, so a slowdown
// caused by something else on the machine identifies itself in the log.
func topCPUProcesses(n int) []string {
	args := []string{"-Ao", "pid,%cpu,comm", "-r"}
	if runtime.GOOS == "linux" {
		args = []string{"-Ao", "pid,%cpu,comm", "--sort=-%cpu"}
	}
	out, err := exec.Command("ps", args...).Output()
	if err != nil {
		return nil
	}
	return parseTopCPU(string(out), os.Getpid(), n)
}
