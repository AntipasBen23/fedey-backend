package publishing

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseWindows(value string) []Window {
	var windows []Window
	for _, raw := range strings.Split(value, ",") {
		parts := strings.Split(strings.TrimSpace(raw), ":")
		if len(parts) != 2 {
			continue
		}

		hour, errHour := strconv.Atoi(parts[0])
		minute, errMinute := strconv.Atoi(parts[1])
		if errHour != nil || errMinute != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
			continue
		}

		windows = append(windows, Window{
			Hour:   hour,
			Minute: minute,
			Label:  fmt.Sprintf("%02d:%02d", hour, minute),
		})
	}

	return windows
}
