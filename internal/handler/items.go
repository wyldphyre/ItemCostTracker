package handler

import (
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"itemcosttracker/internal/model"
	"itemcosttracker/internal/store"
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

// parseAmount parses an optional numeric form value. A blank value is zero;
// anything non-numeric is an error rather than a silent zero. NaN and ±Inf are
// rejected too: strconv accepts "NaN" and "Inf", but neither can be stored as
// JSON, so letting one through would make every save fail.
func parseAmount(field, raw string) (float64, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("invalid %s: %q is not a number", field, v)
	}
	return f, nil
}

// formFloat parses an optional numeric form field by name.
func formFloat(r *http.Request, field string) (float64, error) {
	return parseAmount(field, r.FormValue(field))
}

// storeError reports a failed store operation. A missing item is a 404; any
// other failure is logged in full but answered with a generic message, so
// server paths and internals are not echoed to the browser.
func storeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "That item no longer exists. Reload the page.", http.StatusNotFound)
		return
	}
	log.Printf("store error (%s %s): %v", r.Method, r.URL.Path, err)
	http.Error(w, "Could not save changes. Nothing was changed — please try again.", http.StatusInternalServerError)
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
		storeError(w, r, err)
		return
	}
	h.renderTableBody(w)
}

// editItemForm returns the edit form pre-filled with the item's data.
func (h *Handler) editItemForm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, ok := h.store.Get(id)
	if !ok {
		storeError(w, r, store.ErrNotFound)
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
		storeError(w, r, store.ErrNotFound)
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
		storeError(w, r, err)
		return
	}
	h.renderTableBody(w)
}

// deleteItem deletes an item and returns the refreshed table body.
func (h *Handler) deleteItem(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Delete(r.PathValue("id")); err != nil {
		storeError(w, r, err)
		return
	}
	h.renderTableBody(w)
}

// confirmDelete returns an inline confirmation fragment.
func (h *Handler) confirmDelete(w http.ResponseWriter, r *http.Request) {
	item, ok := h.store.Get(r.PathValue("id"))
	if !ok {
		storeError(w, r, store.ErrNotFound)
		return
	}
	h.render(w, "confirm_delete", map[string]any{"Item": item})
}

// cancelDelete restores the Edit/Delete buttons. It is swapped into the
// row's #actions-<id> div, so it must return only the buttons, not the row.
func (h *Handler) cancelDelete(w http.ResponseWriter, r *http.Request) {
	item, ok := h.store.Get(r.PathValue("id"))
	if !ok {
		storeError(w, r, store.ErrNotFound)
		return
	}
	h.render(w, "item_actions", item)
}

// addCostRow returns a blank additional-cost input row for HTMX append.
func (h *Handler) addCostRow(w http.ResponseWriter, r *http.Request) {
	h.render(w, "cost_row", model.AdditionalCost{})
}

// parseItemForm extracts an Item from an HTTP form submission.
//
// Additional costs arrive as repeated cost_description / cost_amount fields,
// one pair per row, in the order the rows appear in the form. Pairing by
// position (rather than by a numeric index in the field name) means removing a
// row, adding rows after a removal, or leaving a row blank can never cause
// other rows to be skipped or overwritten.
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
		return model.Item{}, fmt.Errorf("item name is required")
	}

	descriptions := r.PostForm["cost_description"]
	amounts := r.PostForm["cost_amount"]
	if len(descriptions) != len(amounts) {
		return model.Item{}, fmt.Errorf("additional costs are malformed: %d descriptions but %d amounts",
			len(descriptions), len(amounts))
	}

	var additionalCosts []model.AdditionalCost
	for i := range descriptions {
		desc := strings.TrimSpace(descriptions[i])
		amt, err := parseAmount("additional cost amount", amounts[i])
		if err != nil {
			return model.Item{}, err
		}
		// A completely blank row is ignored; it never ends the list.
		if desc == "" && amt == 0 {
			continue
		}
		additionalCosts = append(additionalCosts, model.AdditionalCost{
			Description: desc,
			Amount:      amt,
		})
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
