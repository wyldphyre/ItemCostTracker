package handler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"itemcosttracker/internal/model"
)

// ImportPayload is the JSON format accepted by the import endpoint.
// It matches the internal store format so exports can be re-imported directly.
type ImportPayload struct {
	Items []model.Item `json:"items"`
}

// exportJSON serves all items as a downloadable JSON file.
func (h *Handler) exportJSON(w http.ResponseWriter, r *http.Request) {
	items := h.store.All()
	data, err := json.MarshalIndent(ImportPayload{Items: items}, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("itemcosttracker-%s.json", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Write(data)
}

// exportCSV serves all items (with calculated fields) as a downloadable CSV file.
func (h *Handler) exportCSV(w http.ResponseWriter, r *http.Request) {
	items := h.store.All()

	filename := fmt.Sprintf("itemcosttracker-%s.csv", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	cw := csv.NewWriter(w)
	cw.Write([]string{
		"Name",
		"Purchase Date",
		"Purchase Price",
		"Final Activity Date",
		"Resale Value",
		"Additional Costs (JSON)",
		"Projected Years",
		"Estimated Use Count",
		"Usage Period",
		"Cost",
		"Days Active",
		"Cost/Day",
		"Months Active",
		"Cost/Month",
		"Years Active",
		"Cost/Year",
		"Projected Cost/Year",
		"Est. Uses",
		"Cost/Use",
	})

	for _, item := range items {
		c := item.Compute()

		finalDate := ""
		if item.FinalActivityDate != nil {
			finalDate = item.FinalActivityDate.Format("2006-01-02")
		}

		acJSON := "[]"
		if len(item.AdditionalCosts) > 0 {
			b, _ := json.Marshal(item.AdditionalCosts)
			acJSON = string(b)
		}

		// Usage columns stay blank rather than showing 0 when the item has no
		// usage data, matching how the table renders them as "-".
		estUses, costPerUse := "", ""
		if c.HasUsageData {
			estUses = fmt.Sprintf("%.1f", c.EstimatedUses)
			costPerUse = fmt.Sprintf("%.4f", c.CostPerUse)
		}

		cw.Write([]string{
			item.Name,
			item.PurchaseDate.Format("2006-01-02"),
			fmt.Sprintf("%.2f", item.PurchasePrice),
			finalDate,
			fmt.Sprintf("%.2f", item.ResaleValue),
			acJSON,
			fmt.Sprintf("%.4f", item.ProjectedYears),
			fmt.Sprintf("%.4f", item.EstimatedUseCount),
			item.UsagePeriod,
			fmt.Sprintf("%.2f", c.Cost),
			fmt.Sprintf("%.1f", c.DaysActive),
			fmt.Sprintf("%.4f", c.CostPerDay),
			fmt.Sprintf("%.1f", c.MonthsActive),
			fmt.Sprintf("%.4f", c.CostPerMonth),
			fmt.Sprintf("%.4f", c.YearsActive),
			fmt.Sprintf("%.4f", c.CostPerYear),
			fmt.Sprintf("%.4f", c.ProjectedCostPerYear),
			estUses,
			costPerUse,
		})
	}
	cw.Flush()
}

// importForm returns the import modal form fragment.
func (h *Handler) importForm(w http.ResponseWriter, r *http.Request) {
	h.render(w, "import_form", nil)
}

// importData handles JSON file upload and merges or replaces items.
func (h *Handler) importData(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB limit
		h.renderImportError(w, "Could not parse upload: "+err.Error())
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		h.renderImportError(w, "No file provided.")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 10<<20))
	if err != nil {
		h.renderImportError(w, "Could not read file: "+err.Error())
		return
	}

	var payload ImportPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		h.renderImportError(w, "Invalid JSON: "+err.Error())
		return
	}

	if len(payload.Items) == 0 {
		h.renderImportError(w, "No items found in the file.")
		return
	}

	// Ensure every item has an ID and timestamps
	now := time.Now().UTC()
	for i := range payload.Items {
		if payload.Items[i].ID == "" {
			payload.Items[i].ID = model.NewUUID()
		}
		if payload.Items[i].CreatedAt.IsZero() {
			payload.Items[i].CreatedAt = now
		}
		if payload.Items[i].UpdatedAt.IsZero() {
			payload.Items[i].UpdatedAt = now
		}
	}

	mode := r.FormValue("mode")
	var added, updated int

	switch mode {
	case "replace":
		if err := h.store.ReplaceAll(payload.Items); err != nil {
			h.renderImportError(w, "Import failed: "+err.Error())
			return
		}
		added = len(payload.Items)
	default: // "merge"
		var mergeErr error
		added, updated, mergeErr = h.store.Merge(payload.Items)
		if mergeErr != nil {
			h.renderImportError(w, "Import failed: "+mergeErr.Error())
			return
		}
	}

	// Close the modal and reload the page via HTMX redirect
	log.Printf("Import complete: %d added, %d updated", added, updated)
	w.Header().Set("HX-Location", "/")
	w.WriteHeader(http.StatusOK)
}

// renderImportError renders an error message back into the import form.
func (h *Handler) renderImportError(w http.ResponseWriter, msg string) {
	h.render(w, "import_form", map[string]any{"Error": msg})
}
