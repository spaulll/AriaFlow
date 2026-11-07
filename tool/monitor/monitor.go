package monitor

import (
	"ariaflow/internal/config"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"os"
	"strings"
	"time"
)

/*
about system information
*/

func GetCpuPercent() float64 {
	percent, err := cpu.Percent(time.Second, false)
	if err != nil || len(percent) == 0 {
		return 0
	}
	return percent[0]
}

func GetMemPercent() float64 {
	memInfo, err := mem.VirtualMemory()
	if err != nil || memInfo == nil {
		return 0
	}
	return memInfo.UsedPercent
}

func GetDiskPercent(path string) float64 {
	//parts, _ := disk.Partitions(true)
	diskInfo, err := disk.Usage(path)
	if err != nil || diskInfo == nil {
		// mount unavailable (e.g. USB/NAS hiccup) - report 0 instead of
		// nil-pointering the live progress view
		return 0
	}
	return diskInfo.UsedPercent
}

// IsLocal checks if the uri is local
func IsLocal(uri string) bool {
	_, err := os.Stat(config.GetDownloadFolder())
	if err != nil {
		return false
	}
	return strings.Contains(uri, "127.0.0.1") || strings.Contains(uri, "localhost")
}
