package main

import (
	"fmt"
	"time"
)

type OutletSchedule struct {
	Enabled     bool   `json:"enabled"`
	NightBudget int64  `json:"nightBudget"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Days        []int  `json:"days"`
}

func (s OutletSchedule) validate() error {
	if !s.Enabled {
		return nil
	}
	if s.NightBudget < 1024 || s.NightBudget > 1<<40 {
		return fmt.Errorf("夜间预算无效 / Invalid night budget")
	}
	if _, err := time.Parse("15:04", s.Start); err != nil {
		return fmt.Errorf("夜间开始时间无效 / Invalid start time")
	}
	if _, err := time.Parse("15:04", s.End); err != nil || s.Start == s.End {
		return fmt.Errorf("开始结束时间必须不同 / Start and end must differ")
	}
	if len(s.Days) == 0 || len(s.Days) > 7 {
		return fmt.Errorf("请选择启用星期 / Select scheduled weekdays")
	}
	seen := map[int]bool{}
	for _, day := range s.Days {
		if day < 0 || day > 6 || seen[day] {
			return fmt.Errorf("invalid weekdays")
		}
		seen[day] = true
	}
	return nil
}

// Weekdays identify the starting day. Monday23:00–08:00 ends Tuesday morning.
func (p Pool) effectiveBudget(now time.Time) (int64, bool) {
	s := p.Schedule
	if !s.Enabled {
		return p.Budget, false
	}
	start, e1 := time.Parse("15:04", s.Start)
	end, e2 := time.Parse("15:04", s.End)
	if e1 != nil || e2 != nil || s.Start == s.End {
		return p.Budget, false
	}
	minute := now.Hour()*60 + now.Minute()
	a := start.Hour()*60 + start.Minute()
	b := end.Hour()*60 + end.Minute()
	day := int(now.Weekday())
	active := false
	if a < b {
		active = minute >= a && minute < b
	} else if minute >= a {
		active = true
	} else if minute < b {
		active = true
		day = int(now.AddDate(0, 0, -1).Weekday())
	}
	if active {
		for _, selected := range s.Days {
			if selected == day {
				return s.NightBudget, true
			}
		}
	}
	return p.Budget, false
}
