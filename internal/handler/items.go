package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"itemcosttracker/internal/model"
)

// validUsagePeriods are the only values Compute knows how to turn into a
// number of periods. Anything else would silently yield zero estimated uses.
var validUsagePeriods = map[string]bool{
	"":        true,
	"daily":   true,
	"weekly":  true,
	"monthly": true,
	"yearly":  true,
}

// formFloat parses an optional numeric form field. A blank field is zero;
// anything non-numeric is an error rather than a silent zero.
func formFloat(r *http.Request, field string) (float64, error) {
	v := strings.TrimSpace(r.FormValue(field))
	if v == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %q is not a number", field, v)
	}
	return f, nil
}

// listItems renders the full page with all items.
func (h *Handler) listItems(w http.ResponseWriter, r *http.Request) {
	items := h.store.All()
	withCalc := make([]model.ItemWithCalc, len(items))
	for i, item := range items {
		withCalc[i] = model.ItemWithCalc{Item: item, Calc: item.Compute()}
	}
	h.render(w, "index.html", map[string]any{"Items": withCalc})
}

// newItemForm returns the add-item form fragment for HTMX.
func (h *Handler) newItemForm(w http.ResponseWriter, r *http.Request) {
	h.render(w, "item_form", map[string]any{
		"Item":       model.Item{AdditionalCosts: []model.AdditionalCost{}},
		"FormAction": "/items",
		"Method":     "post",
	})
}

// createItem creates a new item and returns the refreshed table body.
func (h *Handler) createItem(w http.ResponseWriter, r *http.Request) {
	item, err := parseItemForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	now := time.Now().UTC()
	item.ID = model.NewUUID()
	item.CreatedAt = now
	item.UpdatedAt = now

	if err := h.store.Create(item); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.renderTableBody(w)
}

// editItemForm returns the edit form pre-filled with the item's data.
func (h *Handler) editItemForm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, ok := h.store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	h.render(w, "item_form", map[string]any{
		"Item":       item,
		"FormAction": "/items/" + id,
		"Method":     "put",
	})
}

// updateItem updates an item and returns the refreshed table body.
func (h *Handler) updateItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, ok := h.store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	item, err := parseItemForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item.ID = id
	item.CreatedAt = existing.CreatedAt
	item.UpdatedAt = time.Now().UTC()

	if err := h.store.Update(item); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.renderTableBody(w)
}

// deleteItem deletes an item and returns the refreshed table body.
func (h *Handler) deleteItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.Delete(id); err != nil {
		http.NotFound(w, r)
		return
	}
	h.renderTableBody(w)
}

// confirmDelete returns an inline confirmation fragment.
func (h *Handler) confirmDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, ok := h.store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	h.render(w, "confirm_delete", map[string]any{"Item": item})
}

// cancelDelete restores the action buttons (cancels the delete confirmation).
func (h *Handler) cancelDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, ok := h.store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	calc := item.Compute()
	h.render(w, "item_row", model.ItemWithCalc{Item: item, Calc: calc})
}

// addCostRow returns a blank additional-cost input row for HTMX append.
func (h *Handler) addCostRow(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	index, _ := strconv.Atoi(r.FormValue("index"))
	h.render(w, "cost_row", map[string]any{
		"Index": index,
		"Cost":  nil,
	})
}

// parseItemForm extracts an Item from an HTTP form submission.
// Additional costs use array-style naming:
//
//	additional_costs[0][description], additional_costs[0][amount], ...
func parseItemForm(r *http.Request) (model.Item, error) {
	if err := r.ParseForm(); err != nil {
		return model.Item{}, err
	}

	purchaseDate, err := time.Parse("2006-01-02", r.FormValue("purchase_date"))
	if err != nil {
		return model.Item{}, fmt.Errorf("invalid purchase_date: %w", err)
	}

	var finalDate *time.Time
	if fd := r.FormValue("final_activity_date"); fd != "" {
		t, err := time.Parse("2006-01-02", fd)
		if err != nil {
			return model.Item{}, fmt.Errorf("invalid final_activity_date: %w", err)
		}
		finalDate = &t
	}

	purchasePrice, err := formFloat(r, "purchase_price")
	if err != nil {
		return model.Item{}, err
	}
	resaleValue, err := formFloat(r, "resale_value")
	if err != nil {
		return model.Item{}, err
	}
	projectedYears, err := formFloat(r, "projected_years")
	if err != nil {
		return model.Item{}, err
	}
	estimatedUseCount, err := formFloat(r, "estimated_use_count")
	if err != nil {
		return model.Item{}, err
	}

	usagePeriod := r.FormValue("usage_period")
	if !validUsagePeriods[usagePeriod] {
		return model.Item{}, fmt.Errorf("invalid usage_period: %q", usagePeriod)
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		return model.Item{}, fmt.Errorf("name is required")
	}

	// Collect additional costs from array-style form fields
	var additionalCosts []model.AdditionalCost
	for i := 0; i < 100; i++ {
		desc := r.FormValue(fmt.Sprintf("additional_costs[%d][description]", i))
		amtStr := r.FormValue(fmt.Sprintf("additional_costs[%d][amount]", i))
		if desc == "" && amtStr == "" {
			break
		}
		amt, err := formFloat(r, fmt.Sprintf("additional_costs[%d][amount]", i))
		if err != nil {
			return model.Item{}, err
		}
		if desc != "" || amt != 0 {
			additionalCosts = append(additionalCosts, model.AdditionalCost{
				Description: desc,
				Amount:      amt,
			})
		}
	}

	return model.Item{
		Name:              name,
		PurchaseDate:      purchaseDate,
		PurchasePrice:     purchasePrice,
		FinalActivityDate: finalDate,
		ResaleValue:       resaleValue,
		AdditionalCosts:   additionalCosts,
		ProjectedYears:    projectedYears,
		EstimatedUseCount: estimatedUseCount,
		UsagePeriod:       usagePeriod,
	}, nil
}
