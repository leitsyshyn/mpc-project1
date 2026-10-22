package bench

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestSummarize(t *testing.T) {
	ms := func(ns ...int) []time.Duration {
		var ds []time.Duration
		for _, n := range ns {
			ds = append(ds, time.Duration(n)*time.Millisecond)
		}
		return ds
	}
	tests := []struct {
		name             string
		in               []time.Duration
		median, min, max time.Duration
	}{
		{"odd", ms(5, 1, 3), 3 * time.Millisecond, 1 * time.Millisecond, 5 * time.Millisecond},
		{"even", ms(4, 1, 3, 2), 2500 * time.Microsecond, 1 * time.Millisecond, 4 * time.Millisecond},
		{"single", ms(7), 7 * time.Millisecond, 7 * time.Millisecond, 7 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := slices.Clone(tt.in)
			got := Summarize(tt.in)
			if got.Median != tt.median || got.Min != tt.min || got.Max != tt.max {
				t.Errorf("Summarize(%v) = %+v, want median %v min %v max %v", tt.in, got, tt.median, tt.min, tt.max)
			}
			if !slices.Equal(tt.in, orig) {
				t.Errorf("Summarize reordered its input: %v", tt.in)
			}
		})
	}
}

func TestMeasureExcludesWarmup(t *testing.T) {
	calls := 0
	times := Measure(1, 5, func() {
		calls++
		if calls == 1 {
			time.Sleep(20 * time.Millisecond) // a slow first run must not be reported
		}
	})
	if calls != 6 {
		t.Errorf("run called %d times, want 6 (1 warmup + 5 measured)", calls)
	}
	if len(times) != 5 {
		t.Fatalf("got %d samples, want 5", len(times))
	}
	for _, d := range times {
		if d >= 20*time.Millisecond {
			t.Errorf("sample %v includes the warmup run", d)
		}
	}
}

func TestAppendCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "results.csv")
	header := []string{"version", "seconds"}
	for _, row := range [][]string{{"v0", "1.5"}, {"v2", "2.5"}} {
		if err := AppendCSV(path, header, [][]string{row}); err != nil {
			t.Fatal(err)
		}
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "version,seconds\nv0,1.5\nv2,2.5\n"; string(got) != want {
		t.Errorf("csv =\n%s\nwant\n%s", got, want)
	}
}

func TestRows(t *testing.T) {
	got := Rows("invindex", "v3", 8, 64, []time.Duration{1500 * time.Millisecond, 2 * time.Second})
	want := [][]string{
		{"invindex", "v3", "8", "64", "1", "1.500000"},
		{"invindex", "v3", "8", "64", "2", "2.000000"},
	}
	if !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("Rows = %q, want %q", got, want)
	}
	// K is left empty for versions that have no K.
	if got := Rows("invindex", "v0", 1, 0, []time.Duration{time.Second}); got[0][3] != "" {
		t.Errorf("k column = %q, want empty", got[0][3])
	}
}
