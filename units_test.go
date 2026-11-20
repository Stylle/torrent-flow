package main

import "testing"

func TestUnifiedDecimalUnits(t *testing.T) {
	for _, v := range []struct {
		bytes int64
		rate  bool
		text  string
	}{{1000000, false, "1 MB"}, {1048576, false, "1.049 MB"}, {50000, true, "50 KB/s"}, {6250000, true, "6.25 MB/s"}, {0, true, "0 KB/s"}, {1, true, "0.001 KB/s"}} {
		if formatBytes(v.bytes, v.rate) != v.text {
			t.Fatal("unit mismatch")
		}
	}
	if normalizedRateText("limit: 1048576 B/s; 1 MiB/s; 8 Mbps") != "limit: 1.049 MB/s; 1.049 MB/s; 1 MB/s" {
		t.Fatal("legacy log units not normalized")
	}
}
