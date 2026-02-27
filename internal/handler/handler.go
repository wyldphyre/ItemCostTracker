package handler

import (
	"html/template"
	"io/fs"
	"log"
	"net/http"

	"itemcosttracker/internal/model"
	"itemcosttracker/internal/store"
)

// Handler holds shared dependencies for HTTP handlers.
type Handler struct {
	store     *store.Store
	templates *template.Template
}

// New creates a Handler and parses all templates from the given filesystem.
func New(s *store.Store, templateFS fs.FS) (*Handler, error) {
	tmpl, err := template.New("").
		Funcs(templateFuncs()).
		ParseFS(templateFS,
			"templates/base.html",
			"templates/index.html",
			"templates/partials/item_row.html",
			"templates/partials/item_form.html",
			"templates/partials/cost_row.html",
			"templates/partials/confirm_delete.html",
			"templates/partials/import_form.html",
		)
	if err != nil {
		return nil, err
	}
	return &Handler{store: s, templates: tmpl}, nil
}

// Register wires all routes to the mux.
func (h *Handler) Register(mux *http.ServeMux, staticFS fs.FS) {
	mux.HandleFunc("GET /{$}", h.listItems)
	mux.HandleFunc("GET /items/new", h.newItemForm)
	mux.HandleFunc("POST /items", h.createItem)
	mux.HandleFunc("GET /items/{id}/edit", h.editItemForm)
	mux.HandleFunc("PUT /items/{id}", h.updateItem)
	mux.HandleFunc("DELETE /items/{id}", h.deleteItem)
	mux.HandleFunc("GET /items/{id}/confirm-delete", h.confirmDelete)
	mux.HandleFunc("GET /items/{id}/cancel-delete", h.cancelDelete)
	mux.HandleFunc("POST /items/cost-row", h.addCostRow)
	mux.HandleFunc("GET /export/json", h.exportJSON)
	mux.HandleFunc("GET /export/csv", h.exportCSV)
	mux.HandleFunc("GET /import", h.importForm)
	mux.HandleFunc("POST /import", h.importData)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))
}

// render executes a named template with the given data.
func (h *Handler) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template error (%s): %v", name, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// renderTableBody re-renders the full table body (used after any mutation).
func (h *Handler) renderTableBody(w http.ResponseWriter) {
	items := h.store.All()
	withCalc := make([]model.ItemWithCalc, len(items))
	for i, item := range items {
		withCalc[i] = model.ItemWithCalc{Item: item, Calc: item.Compute()}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	for _, item := range withCalc {
		if err := h.templates.ExecuteTemplate(w, "item_row", item); err != nil {
			log.Printf("template error (item_row): %v", err)
			return
		}
	}
}
