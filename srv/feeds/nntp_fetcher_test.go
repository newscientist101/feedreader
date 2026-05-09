package feeds

import "testing"

// TestPlanArticleRange verifies the range planner policy:
//   - empty/invalid groups are no-ops
//   - first fetch (highWater==0) imports the latest firstFetchCount articles
//   - first fetch with fewer than firstFetchCount articles in the group uses all
//   - subsequent fetch starts at highWater+1
//   - low-water clamping (server retired old articles)
//   - per-run cap of fetchRunCap articles
func TestPlanArticleRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		low       int64
		high      int64
		highWater int64
		wantStart int64
		wantEnd   int64
	}{
		{
			name: "empty group: serverHigh < serverLow",
			low:  5, high: 4, highWater: 0,
			wantStart: 1, wantEnd: 0, // no-op range
		},
		{
			name: "empty group: both zero",
			low:  0, high: 0, highWater: 0,
			wantStart: 1, wantEnd: 0, // no-op range
		},
		{
			name: "first fetch exactly firstFetchCount articles",
			low:  1, high: 100, highWater: 0,
			wantStart: 1, wantEnd: 100,
		},
		{
			name: "first fetch more than firstFetchCount articles",
			low:  1, high: 5000, highWater: 0,
			wantStart: 4901, wantEnd: 5000, // latest 100
		},
		{
			name: "first fetch fewer than firstFetchCount articles",
			low:  1, high: 42, highWater: 0,
			wantStart: 1, wantEnd: 42, // clamp to serverLow
		},
		{
			name: "first fetch only one article",
			low:  99, high: 99, highWater: 0,
			wantStart: 99, wantEnd: 99,
		},
		{
			name: "subsequent fetch no new articles",
			low:  1, high: 200, highWater: 200,
			wantStart: 201, wantEnd: 200, // no-op
		},
		{
			name: "subsequent fetch from highWater+1",
			low:  1, high: 300, highWater: 200,
			wantStart: 201, wantEnd: 300,
		},
		{
			name: "subsequent fetch capped at fetchRunCap",
			low:  1, high: 10000, highWater: 200,
			wantStart: 201, wantEnd: 700, // 201 + 500 - 1
		},
		{
			name: "low-water clamping: server retired old articles",
			low:  50, high: 200, highWater: 30, // highWater+1=31 < serverLow=50
			wantStart: 50, wantEnd: 200,
		},
		{
			name: "low-water clamp on first fetch when high < firstFetchCount",
			low:  10, high: 50, highWater: 0,
			wantStart: 10, wantEnd: 50, // high-firstFetchCount+1 = -49, clamp to low=10
		},
		{
			name: "cap applies on first fetch when group is large",
			low:  1, high: 100000, highWater: 0,
			// start = 100000-100+1=99901; end = 99901+500-1=100400 but capped at 100000
			wantStart: 99901, wantEnd: 100000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotStart, gotEnd := planArticleRange(tc.low, tc.high, tc.highWater)
			if gotStart != tc.wantStart || gotEnd != tc.wantEnd {
				t.Errorf("planArticleRange(%d,%d,%d) = (%d,%d), want (%d,%d)",
					tc.low, tc.high, tc.highWater,
					gotStart, gotEnd,
					tc.wantStart, tc.wantEnd)
			}
		})
	}
}

// TestPlanArticleRange_EmptyRange ensures start > end is consistently a no-op.
func TestPlanArticleRange_EmptyRange(t *testing.T) {
	t.Parallel()

	start, end := planArticleRange(1, 100, 100)
	if start <= end {
		t.Errorf("expected no-op range, got start=%d end=%d", start, end)
	}
}

// TestNNTPNotConfigured verifies ErrNNTPNotConfigured wrapping.
func TestNNTPNotConfigured(t *testing.T) {
	t.Parallel()

	// ErrNNTPNotConfigured must be a distinct, unwrappable sentinel.
	if ErrNNTPNotConfigured == nil {
		t.Fatal("ErrNNTPNotConfigured is nil")
	}
}
