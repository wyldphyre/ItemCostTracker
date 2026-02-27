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
	// Jan 31 to Feb 28: basis 0 treats Jan 31 as day 30 → 28 days / 360 = ~0.0778
	start := date(2023, 1, 31)
	end := date(2023, 2, 28)
	got := YearFrac(start, end)
	// Basis 0: d1=30, d2=30 (end of Feb when d1==30), days = 30*1+30-30 = 30 → 30/360
	want := 30.0 / 360.0
	if !approxEqual(got, want, 0.0001) {
		t.Errorf("YearFrac end-of-month = %.6f, want %.6f", got, want)
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
