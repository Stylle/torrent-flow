package main

import (
	"context"
	"testing"
	"time"
)

func TestNightBudgetWeekdayAndMidnight(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	p := Pool{Budget: 1000000, Schedule: OutletSchedule{Enabled: true, NightBudget: 2000000, Start: "23:00", End: "08:00", Days: []int{1}}}
	for _, v := range []struct {
		date  string
		night bool
	}{{"2026-11-23 22:59", false}, {"2026-11-23 23:00", true}, {"2026-11-24 07:59", true}, {"2026-11-24 08:00", false}, {"2026-11-24 23:00", false}, {"2026-11-23 07:00", false}} {
		at, _ := time.ParseInLocation("2006-01-02 15:04", v.date, loc)
		budget, night := p.effectiveBudget(at)
		if night != v.night || (night && budget != 2000000) || (!night && budget != 1000000) {
			t.Fatal("schedule boundary failed", v.date)
		}
	}
	p.Schedule.Enabled = false
	at, _ := time.ParseInLocation("2006-01-02 15:04", "2026-11-23 23:30", loc)
	if budget, night := p.effectiveBudget(at); night || budget != 1000000 {
		t.Fatal("disabled schedule changed base budget")
	}
}
func TestCappedSiteReallocatesToRemainingWeights(t *testing.T) {
	shares, over := allocateSiteShares(1000000, []Allocation{{"a", 1, 0, 100000}, {"b", 2, 0, 0}, {"c", 2, 0, 0}}, map[string]int64{"a": 50000, "b": 50000, "c": 100000})
	if over || shares["a"] != 100000 || shares["b"] != 450000 || shares["c"] != 450000 {
		t.Fatal("capped site's unused budget was not redistributed")
	}
}
func TestScheduleValidation(t *testing.T) {
	valid := OutletSchedule{Enabled: true, NightBudget: 6250000, Start: "23:00", End: "08:00", Days: []int{0, 1, 2, 3, 4, 5, 6}}
	if valid.validate() != nil {
		t.Fatal("valid schedule rejected")
	}
	valid.Days = nil
	if valid.validate() == nil {
		t.Fatal("empty weekday selection accepted")
	}
	valid.Enabled = false
	if valid.validate() != nil {
		t.Fatal("disabled schedule required configuration")
	}
}

func TestEngineUsesScheduledOutletBudget(t *testing.T) {
	s := testStore(t)
	cl, mock := mockClient(t)
	cl.Pool = "pool"
	c := defaultConfig()
	now := time.Now().UTC()
	c.Timezone = "UTC"
	c.Clients = []Client{cl}
	c.Pools = []Pool{{ID: "pool", Name: "Fixture", Budget: 1000000, Enabled: true, Schedule: OutletSchedule{Enabled: true, NightBudget: 2000000, Start: now.Add(-time.Hour).Format("15:04"), End: now.Add(time.Hour).Format("15:04"), Days: []int{0, 1, 2, 3, 4, 5, 6}}}}
	c.Sites = []Site{{ID: "site", Name: "Fixture", Domains: []string{"a.example", "b.example"}, Weight: 1}}
	c.Overrides["fixture:abc"] = Override{Site: "site", Weight: 1}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	s.saveConfig(c)
	s.db.Exec("INSERT OR REPLACE INTO settings VALUES('guardian',?)", time.Now().Unix())
	e := newEngine(s)
	e.tick(context.Background())
	if mock.limit != 2000000 {
		t.Fatalf("scheduled budget not applied: %d", mock.limit)
	}
	c.Pools[0].Schedule.Enabled = false
	s.saveConfig(c)
	e.tick(context.Background())
	if mock.limit != 1000000 {
		t.Fatalf("base budget not restored: %d", mock.limit)
	}
}
