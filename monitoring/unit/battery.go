package monitoring

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type BatteryInfo struct {
	Level    int    `json:"level"`
	Charging bool   `json:"charging"`
	Status   string `json:"status"`
}

// Battery 自动扫描电源子系统并获取电池信息
func Battery() *BatteryInfo {
	basePath := "/sys/class/power_supply"
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		deviceDir := filepath.Join(basePath, entry.Name())
		capPath := filepath.Join(deviceDir, "capacity")
		statusPath := filepath.Join(deviceDir, "status")

		// 检查是否存在 capacity 文件
		capData, err := os.ReadFile(capPath)
		if err != nil {
			continue
		}

		capStr := strings.TrimSpace(string(capData))
		level, err := strconv.Atoi(capStr)
		if err != nil || level < 0 || level > 100 {
			continue
		}

		// 读取状态，默认为 Discharging
		status := "Discharging"
		if statusData, err := os.ReadFile(statusPath); err == nil {
			status = strings.TrimSpace(string(statusData))
		}

		charging := strings.EqualFold(status, "Charging") || strings.EqualFold(status, "Full")

		return &BatteryInfo{
			Level:    level,
			Charging: charging,
			Status:   status,
		}
	}

	return nil
}
