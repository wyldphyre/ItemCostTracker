package model

import (
	"math"
	"testing"
	"time"
)

func date(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func approxEqual(a, b, tol float64) bool {
	return math.Abs(a-b) < tol
}

// Values verified directly from the spreadsheet.
func TestYearFrac_SpreadsheetValues(t *testing.T) {
	tests := []struct {
		name  string
		start time.Time
		end   time.Time
		want  float64
	}{
		{
			// MacBook Pro: purchase 2009-10-20, final activity 2014-11-14 → 5.0666...
			name:  "MacBook Pro",
			start: date(2009, 10, 20),
			end:   date(2014, 11, 14),
			want:  5.0666,
		},
		{
			// PS3: purchase 2009-11-21, final activity 2015-12-05 → 6.0388...
			name:  "PS3",
			start: date(2009, 11, 21),
			end:   date(2015, 12, 5),
			want:  6.0388,
		},
		{
			// Drobo: purchase 2010-02-22, final activity 2017-09-04 → 7.5333...
			name:  "Drobo",
			start: date(2010, 2, 22),
			end:   date(2017, 9, 4),
			want:  7.5333,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := YearFrac(tt.start, tt.end)
			if !approxEqual(got, tt.want, 0.001) {
				t.Errorf("YearFrac(%v, %v) = %.4f, want %.4f", tt.start, tt.end, got, tt.want)
			}
		})
	}
}

func TestYearFrac_SameDay(t *testing.T) {
	d := date(2023, 6, 15)
	got := YearFrac(d, d)
	if got != 0 {
		t.Errorf("YearFrac same day = %v, want 0", got)
	}
}

func TestYearFrac_EndOfMonth(t *testing.T) {
	// Jan 31 to Feb 28: basis 0 treats Jan 31 as day 30. Feb 28 is NOT rounded
	// up, because the end-of-February collapse only applies when the START date
	// is also the last day of February. days = 30*1 + (28-30) = 28 → 28/360.
	start := date(2023, 1, 31)
	end := date(2023, 2, 28)
	got := YearFrac(start, end)
	want := 28.0 / 360.0
	if !approxEqual(got, want, 0.0001) {
		t.Errorf("YearFrac end-of-month = %.6f, want %.6f", got, want)
	}
}

// The four documented Excel basis-0 day adjustments, each exercised directly.
func TestYearFrac_ExcelBasis0Rules(t *testing.T) {
	tests := []struct {
		name       string
		start, end time.Time
		want       float64
	}{
		// Rule 1: both dates are the last day of February → d2 = 30, d1 = 30.
		{"both last day of Feb", date(2022, 2, 28), date(2023, 2, 28), 360.0 / 360},
		{"both last day of Feb, leap end", date(2023, 2, 28), date(2024, 2, 29), 360.0 / 360},
		// Rule 2 alone: start is last day of Feb, end is an ordinary day.
		{"start last day of Feb", date(2023, 2, 28), date(2023, 6, 15), 105.0 / 360},
		// Rule 3: d2 == 31 with d1 of 30 or 31 → d2 = 30.
		{"Jan 31 to Mar 31", date(2023, 1, 31), date(2023, 3, 31), 60.0 / 360},
		{"Apr 30 to Dec 31", date(2023, 4, 30), date(2023, 12, 31), 240.0 / 360},
		// Rule 3 does NOT fire when d1 is an ordinary day.
		{"Jan 15 to Mar 31", date(2023, 1, 15), date(2023, 3, 31), 76.0 / 360},
		// Rule 4 alone: start is the 31st, end is an ordinary day.
		{"Jan 31 to Feb 28", date(2023, 1, 31), date(2023, 2, 28), 28.0 / 360},
		{"Jan 31 to Feb 29 leap", date(2024, 1, 31), date(2024, 2, 29), 29.0 / 360},
		{"Mar 31 to Feb 28 next year", date(2022, 3, 31), date(2023, 2, 28), 328.0 / 360},
		// An end date that merely ends a 30-day month is already day 30.
		{"Jan 31 to Apr 30", date(2023, 1, 31), date(2023, 4, 30), 90.0 / 360},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := YearFrac(tt.start, tt.end)
			if !approxEqual(got, tt.want, 1e-9) {
				t.Errorf("YearFrac(%s, %s) = %.6f, want %.6f (off by %.1f days)",
					tt.start.Format("2006-01-02"), tt.end.Format("2006-01-02"),
					got, tt.want, (got-tt.want)*360)
			}
		})
	}
}

func TestCompute_ActiveItem(t *testing.T) {
	item := &Item{
		PurchaseDate:  date(2020, 1, 1),
		PurchasePrice: 1000,
	}
	c := item.Compute()
	if !c.IsActive {
		t.Error("expected IsActive = true when FinalActivityDate is nil")
	}
	if c.Cost != 1000 {
		t.Errorf("Cost = %.2f, want 1000", c.Cost)
	}
	if c.DaysActive <= 0 {
		t.Error("expected DaysActive > 0 for active item")
	}
}

func TestCompute_AdditionalCosts(t *testing.T) {
	end := date(2022, 1, 1)
	item := &Item{
		PurchaseDate:      date(2020, 1, 1),
		PurchasePrice:     1000,
		FinalActivityDate: &end,
		ResaleValue:       100,
		AdditionalCosts: []AdditionalCost{
			{Description: "Insurance", Amount: 200},
			{Description: "Trade-in credit", Amount: -50},
		},
	}
	c := item.Compute()

	wantTotal := 150.0 // 200 + (-50)
	if !approxEqual(c.AdditionalCostsTotal, wantTotal, 0.001) {
		t.Errorf("AdditionalCostsTotal = %.2f, want %.2f", c.AdditionalCostsTotal, wantTotal)
	}

	// Cost = 1000 + 150 - 100 = 1050
	wantCost := 1050.0
	if !approxEqual(c.Cost, wantCost, 0.001) {
		t.Errorf("Cost = %.2f, want %.2f", c.Cost, wantCost)
	}
}

func TestCompute_ProjectedCost(t *testing.T) {
	// MacBook Pro: purchase 2009-10-20, purchase price 2703.64, resale 200, additional 0
	// Projected years = 5
	// Cost = 2503.64
	// Projected end = 2014-10-20
	// YearFrac(2009-10-20, 2014-10-20) = 5.0 (exactly)
	// Projected cost/year = 2503.64 / 5 = 500.728
	end := date(2014, 11, 14)
	item := &Item{
		PurchaseDate:      date(2009, 10, 20),
		PurchasePrice:     2703.64,
		FinalActivityDate: &end,
		ResaleValue:       200,
		ProjectedYears:    5,
	}
	c := item.Compute()
	// Projected end = 2014-10-20, YearFrac = 5.0 exactly
	if !approxEqual(c.ProjectedCostPerYear, 500.728, 0.01) {
		t.Errorf("ProjectedCostPerYear = %.4f, want ~500.728", c.ProjectedCostPerYear)
	}
}
