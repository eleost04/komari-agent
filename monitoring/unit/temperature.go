package monitoring

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type TemperatureInfo struct {
	CPU     *float64 `json:"cpu,omitempty"`
	Battery *float64 `json:"battery,omitempty"`
}

// Temperature 采集 CPU/SoC 与电池温度（摄氏度）。
// 优先使用 hwmon，其次回退到 thermal_zone；无任何数据时返回 nil。
func Temperature() *TemperatureInfo {
	info := &TemperatureInfo{
		CPU:     cpuTemperature(),
		Battery: batteryTemperature(),
	}
	if info.CPU == nil && info.Battery == nil {
		return nil
	}
	return info
}

func readFloat(path string) (float64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// milliToC 将毫摄氏度转换为摄氏度，过滤明显无效值。
func milliToC(v float64) (float64, bool) {
	if v <= 0 || v > 150000 {
		return 0, false
	}
	return v / 1000, true
}

func isExcludedType(t string) bool {
	for _, ex := range []string{
		"battery", "batt", "charger", "vbat", "ibat", "bcl", "gpu", "ddr",
		"wifi", "modem", "mmw", "camera", "usb", "flash", "quiet", "pa_",
		"sdr", "isense", "nsp", "video", "lvl", "pm8", "pmr7", "pm6", "socd",
	} {
		if strings.Contains(t, ex) {
			return true
		}
	}
	return false
}

func isCPUTempType(t string) bool {
	t = strings.ToLower(t)
	if isExcludedType(t) {
		return false
	}
	for _, in := range []string{
		"cpu", "soc", "pkg", "package", "core", "cluster", "x86",
		"k10", "zen", "therm", "apm", "tsens", "acpi", "tsensor",
	} {
		if strings.Contains(t, in) {
			return true
		}
	}
	return false
}

func isCPUHwmonName(name string) bool {
	name = strings.ToLower(name)
	if isExcludedType(name) {
		return false
	}
	for _, in := range []string{
		"coretemp", "k10temp", "zenpower", "cpu", "soc", "pkg",
		"core", "x86", "acpi", "k8temp", "it87", "nct",
	} {
		if strings.Contains(name, in) {
			return true
		}
	}
	return false
}

func cpuTemperature() *float64 {
	if t := hwmonTemperature(); t != nil {
		return t
	}
	return thermalZoneTemperature()
}

func hwmonTemperature() *float64 {
	dirs, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	var best float64
	found := false
	for _, dir := range dirs {
		nameBytes, err := os.ReadFile(filepath.Join(dir, "name"))
		if err != nil {
			continue
		}
		if !isCPUHwmonName(strings.TrimSpace(string(nameBytes))) {
			continue
		}
		inputs, _ := filepath.Glob(filepath.Join(dir, "temp*_input"))
		for _, in := range inputs {
			raw, ok := readFloat(in)
			if !ok {
				continue
			}
			c, ok := milliToC(raw)
			if !ok {
				continue
			}
			if !found || c > best {
				best = c
				found = true
			}
		}
	}
	if !found {
		return nil
	}
	return &best
}

// thermalZoneTemperature 返回所有 CPU/SoC 类型热区中的最高温度。
func thermalZoneTemperature() *float64 {
	dirs, _ := filepath.Glob("/sys/class/thermal/thermal_zone*")
	var best float64
	found := false
	for _, dir := range dirs {
		typeBytes, err := os.ReadFile(filepath.Join(dir, "type"))
		if err != nil {
			continue
		}
		if !isCPUTempType(strings.TrimSpace(string(typeBytes))) {
			continue
		}
		raw, ok := readFloat(filepath.Join(dir, "temp"))
		if !ok {
			continue
		}
		c, ok := milliToC(raw)
		if !ok {
			continue
		}
		if !found || c > best {
			best = c
			found = true
		}
	}
	if !found {
		return nil
	}
	return &best
}

func thermalZoneByType(name string) *float64 {
	dirs, _ := filepath.Glob("/sys/class/thermal/thermal_zone*")
	for _, dir := range dirs {
		typeBytes, err := os.ReadFile(filepath.Join(dir, "type"))
		if err != nil {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(string(typeBytes)), name) {
			continue
		}
		raw, ok := readFloat(filepath.Join(dir, "temp"))
		if !ok {
			continue
		}
		if c, ok := milliToC(raw); ok {
			return &c
		}
	}
	return nil
}

// batteryTemperature 读取电池温度。优先 power_supply/temp，其次 thermal_zone 中的 battery。
func batteryTemperature() *float64 {
	supplies, _ := filepath.Glob("/sys/class/power_supply/*/temp")
	for _, p := range supplies {
		raw, ok := readFloat(p)
		if !ok {
			continue
		}
		// Android/Linux 电源子系统温度通常为 0.1°C；部分设备为毫摄氏度。
		switch {
		case raw >= 1000:
			if c, ok := milliToC(raw); ok {
				return &c
			}
		case raw >= 50:
			c := raw / 10
			return &c
		}
	}
	return thermalZoneByType("battery")
}
