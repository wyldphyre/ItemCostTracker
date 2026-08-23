package handler

import (
	"bytes"
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
func New(s *store.Store, templateFS fs.FS, version string) (*Handler, error) {
	tmpl, err := template.New("").
		Funcs(templateFuncs(version)).
		ParseFS(templateFS,
			"templates/base.html",
			"templates/index.html",
			"templates/partials/item_row.html",
			"templates/partials/empty_row.html",
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

// render executes a named template with the given data. The output is buffered
// so a mid-template failure produces a clean 500 rather than a half-written body.
func (h *Handler) render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("template error (%s): %v", name, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(buf.Bytes())
}

// renderTableBody re-renders the full table body (used after any mutation).
// With no items it emits the empty-state row, so deleting the last item leaves
// the same markup a fresh page load would produce.
func (h *Handler) renderTableBody(w http.ResponseWriter) {
	items := h.store.All()

	var buf bytes.Buffer
	if len(items) == 0 {
		if err := h.templates.ExecuteTemplate(&buf, "empty_row", nil); err != nil {
			log.Printf("template error (empty_row): %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	for _, item := range items {
		withCalc := model.ItemWithCalc{Item: item, Calc: item.Compute()}
		if err := h.templates.ExecuteTemplate(&buf, "item_row", withCalc); err != nil {
			log.Printf("template error (item_row): %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(buf.Bytes())
}
