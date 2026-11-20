package main

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// UI/log units use decimal bytes. Downloader protocol conversions remain in adapters.
func formatBytes(v int64, rate bool) string {
	if v == 0 {
		if rate {
			return "0 KB/s"
		}
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	power := int(math.Floor(math.Log10(math.Abs(float64(v))) / 3))
	if rate && power < 1 {
		power = 1
	}
	if power < 0 {
		power = 0
	}
	if power > 4 {
		power = 4
	}
	number := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", float64(v)/math.Pow(1000, float64(power))), "0"), ".")
	suffix := ""
	if rate {
		suffix = "/s"
	}
	return number + " " + units[power] + suffix
}

var legacyRate = regexp.MustCompile(`\b([0-9]+(?:\.[0-9]+)?)\s*(GiB/s|MiB/s|KiB/s|GB/s|MB/s|KB/s|B/s|Mbps)\b`)

func normalizedRateText(message string) string {
	return legacyRate.ReplaceAllStringFunc(message, func(match string) string {
		m := legacyRate.FindStringSubmatch(match)
		value, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return match
		}
		scale := map[string]float64{"B/s": 1, "KB/s": 1000, "MB/s": 1e6, "GB/s": 1e9, "KiB/s": 1024, "MiB/s": 1048576, "GiB/s": 1073741824, "Mbps": 125000}[m[2]]
		if value*scale > math.MaxInt64 {
			return match
		}
		return formatBytes(int64(math.Round(value*scale)), true)
	})
}
