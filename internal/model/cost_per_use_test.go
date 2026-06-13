package model

import (
	"testing"
	"time"
)

// fixedEnd makes an item with a known active window by setting a final activity date.
func usageItem(count float64, period string, end time.Time) Item {
	return Item{
		PurchaseDate:      date(2024, 1, 1),
		PurchasePrice:     100,
		FinalActivityDate: &end,
		EstimatedUseCount: count,
		UsagePeriod:       period,
	}
}

func TestCompute_CostPerUse(t *testing.T) {
	// 10 weeks of activity (70 days): 2024-01-01 .. 2024-03-11.
	tenWeeks := date(2024, 3, 11)

	tests := []struct {
		name          string
		item          Item
		wantHasUsage  bool
		wantUses      float64
		wantCostPerUse float64
	}{
		{
			name:          "weekly rate",
			item:          usageItem(2, "weekly", tenWeeks),
			wantHasUsage:  true,
			wantUses:      20, // 2/week * 10 weeks
			wantCostPerUse: 5, // cost 100 / 20
		},
		{
			name:          "daily rate",
			item:          usageItem(1, "daily", tenWeeks),
			wantHasUsage:  true,
			wantUses:      70, // 1/day * 70 days
			wantCostPerUse: 100.0 / 70.0,
		},
		{
			name:         "no period means no usage data",
			item:         usageItem(2, "", tenWeeks),
			wantHasUsage: false,
		},
		{
			name:         "no count means no usage data",
			item:         usageItem(0, "weekly", tenWeeks),
			wantHasUsage: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.item.Compute()
			if c.HasUsageData != tt.wantHasUsage {
				t.Fatalf("HasUsageData = %v, want %v", c.HasUsageData, tt.wantHasUsage)
			}
			if !tt.wantHasUsage {
				return
			}
			if !approxEqual(c.EstimatedUses, tt.wantUses, 0.01) {
				t.Errorf("EstimatedUses = %v, want %v", c.EstimatedUses, tt.wantUses)
			}
			if !approxEqual(c.CostPerUse, tt.wantCostPerUse, 0.01) {
				t.Errorf("CostPerUse = %v, want %v", c.CostPerUse, tt.wantCostPerUse)
			}
		})
	}
}
