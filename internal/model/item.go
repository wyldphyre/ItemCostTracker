package model

import "time"

// AdditionalCost is a labelled cost entry. Amount can be negative (e.g. trade-in credit).
type AdditionalCost struct {
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
}

// Item is the persisted domain object. Calculated fields are derived on read.
type Item struct {
	ID                string           `json:"id"`
	Name              string           `json:"name"`
	PurchaseDate      time.Time        `json:"purchase_date"`
	PurchasePrice     float64          `json:"purchase_price"`
	FinalActivityDate *time.Time       `json:"final_activity_date,omitempty"` // nil = still active
	ResaleValue       float64          `json:"resale_value"`
	AdditionalCosts   []AdditionalCost `json:"additional_costs"`
	ProjectedYears    float64          `json:"projected_years"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

// Calculated holds all derived fields, computed fresh on each request. Never stored.
type Calculated struct {
	AdditionalCostsTotal float64
	Cost                 float64
	EffectiveEndDate     time.Time
	IsActive             bool
	DaysActive           float64
	MonthsActive         float64
	YearsActive          float64
	CostPerDay           float64
	CostPerMonth         float64
	CostPerYear             float64
	ProjectedCostPerYear    float64
	ProjectedYearsReached   bool // true when YearsActive >= ProjectedYears
}

// Compute derives all calculated fields from the item's raw data.
func (item *Item) Compute() Calculated {
	var c Calculated

	for _, ac := range item.AdditionalCosts {
		c.AdditionalCostsTotal += ac.Amount
	}

	c.Cost = item.PurchasePrice + c.AdditionalCostsTotal - item.ResaleValue

	if item.FinalActivityDate != nil {
		c.EffectiveEndDate = *item.FinalActivityDate
		c.IsActive = false
	} else {
		c.EffectiveEndDate = time.Now().UTC().Truncate(24 * time.Hour)
		c.IsActive = true
	}

	c.DaysActive = c.EffectiveEndDate.Sub(item.PurchaseDate).Hours() / 24
	c.MonthsActive = c.DaysActive / 30
	c.YearsActive = YearFrac(item.PurchaseDate, c.EffectiveEndDate)

	if c.DaysActive > 0 {
		c.CostPerDay = c.Cost / c.DaysActive
	}
	if c.MonthsActive > 0 {
		c.CostPerMonth = c.Cost / c.MonthsActive
	}
	if c.YearsActive > 0 {
		c.CostPerYear = c.Cost / c.YearsActive
	}

	if item.ProjectedYears > 0 {
		projEndDate := addFractionalYears(item.PurchaseDate, item.ProjectedYears)
		projYears := YearFrac(item.PurchaseDate, projEndDate)
		if projYears > 0 {
			c.ProjectedCostPerYear = c.Cost / projYears
		}
		c.ProjectedYearsReached = c.YearsActive >= item.ProjectedYears
	}

	return c
}

// ItemWithCalc bundles an item with its computed values for template rendering.
type ItemWithCalc struct {
	Item
	Calc Calculated
}
