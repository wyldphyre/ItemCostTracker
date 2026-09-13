package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"itemcosttracker/internal/model"
	"itemcosttracker/internal/store"
)

func formRequest(method string, form url.Values) *http.Request {
	r := httptest.NewRequest(method, "/items", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func cost(description string, amount float64) model.AdditionalCost {
	return model.AdditionalCost{Description: description, Amount: amount}
}

func baseForm() url.Values {
	return url.Values{
		"name":           {"Widget"},
		"purchase_date":  {"2021-01-01"},
		"purchase_price": {"100"},
	}
}

func TestParseItemForm_CostRows(t *testing.T) {
	tests := []struct {
		name  string
		descs []string
		amts  []string
		want  []model.AdditionalCost
	}{
		{
			// The rows left after the user removed the middle of three rows and
			// added another: previously only the first survived.
			name:  "row removed then another added",
			descs: []string{"A", "C", "D"},
			amts:  []string{"10", "30", "40"},
			want:  []model.AdditionalCost{cost("A", 10), cost("C", 30), cost("D", 40)},
		},
		{
			// A blank first row used to end parsing, dropping everything after it.
			name:  "blank row before a filled one",
			descs: []string{"", "Case"},
			amts:  []string{"", "25"},
			want:  []model.AdditionalCost{cost("Case", 25)},
		},
		{
			name:  "description only, amount only, negative credit",
			descs: []string{"Note", "", "Trade-in"},
			amts:  []string{"", "5", "-50"},
			want:  []model.AdditionalCost{cost("Note", 0), cost("", 5), cost("Trade-in", -50)},
		},
		{
			name: "no rows",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := baseForm()
			form["cost_description"] = tt.descs
			form["cost_amount"] = tt.amts
			item, err := parseItemForm(formRequest(http.MethodPost, form))
			if err != nil {
				t.Fatal(err)
			}
			if len(item.AdditionalCosts) != len(tt.want) {
				t.Fatalf("costs = %+v, want %+v", item.AdditionalCosts, tt.want)
			}
			for i := range tt.want {
				if item.AdditionalCosts[i] != tt.want[i] {
					t.Errorf("cost %d = %+v, want %+v", i, item.AdditionalCosts[i], tt.want[i])
				}
			}
		})
	}
}

// PUT bodies must be parsed the same way as POST (the edit form uses PUT).
func TestParseItemForm_CostRowsOnPut(t *testing.T) {
	form := baseForm()
	form["cost_description"] = []string{"", "Kept"}
	form["cost_amount"] = []string{"", "7"}
	item, err := parseItemForm(formRequest(http.MethodPut, form))
	if err != nil {
		t.Fatal(err)
	}
	if len(item.AdditionalCosts) != 1 || item.AdditionalCosts[0].Description != "Kept" {
		t.Errorf("costs = %+v, want [{Kept 7}]", item.AdditionalCosts)
	}
}

func TestParseItemForm_Rejects(t *testing.T) {
	tests := map[string]func(url.Values){
		"NaN price":            func(f url.Values) { f.Set("purchase_price", "NaN") },
		"Inf resale":           func(f url.Values) { f.Set("resale_value", "Inf") },
		"-Infinity years":      func(f url.Values) { f.Set("projected_years", "-Infinity") },
		"overflow uses":        func(f url.Values) { f.Set("estimated_use_count", "1e999") },
		"text price":           func(f url.Values) { f.Set("purchase_price", "abc") },
		"NaN cost amount":      func(f url.Values) { f["cost_description"] = []string{"x"}; f["cost_amount"] = []string{"nan"} },
		"mismatched cost rows": func(f url.Values) { f["cost_description"] = []string{"a", "b"}; f["cost_amount"] = []string{"1"} },
		"blank name":           func(f url.Values) { f.Set("name", "   ") },
		"bad usage period":     func(f url.Values) { f.Set("usage_period", "Weekly") },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			form := baseForm()
			mutate(form)
			if _, err := parseItemForm(formRequest(http.MethodPost, form)); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

// ── Handler-level tests ───────────────────────────────────────────────

func newTestHandler(t *testing.T) (*Handler, *store.Store, *http.ServeMux) {
	t.Helper()
	s, err := store.New(filepath.Join(t.TempDir(), "items.json"))
	if err != nil {
		t.Fatal(err)
	}
	h, err := New(s, os.DirFS("../.."), "test")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	h.Register(mux, os.DirFS("../../static"))
	return h, s, mux
}

func serve(mux *http.ServeMux, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

func TestCancelDeleteReturnsOnlyActionButtons(t *testing.T) {
	_, s, mux := newTestHandler(t)
	if err := s.Create(model.Item{ID: "abc", Name: "Widget"}); err != nil {
		t.Fatal(err)
	}
	w := serve(mux, httptest.NewRequest(http.MethodGet, "/items/abc/cancel-delete", nil))
	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	// The response is swapped into #actions-abc, so it must not contain the
	// row, its cells, or another element carrying the actions id.
	for _, forbidden := range []string{"<tr", "<td", `id="actions-abc"`, `id="item-row-abc"`, "col-name"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("response contains %q:\n%s", forbidden, body)
		}
	}
	for _, required := range []string{`hx-get="/items/abc/edit"`, `hx-get="/items/abc/confirm-delete"`} {
		if !strings.Contains(body, required) {
			t.Errorf("response missing %q:\n%s", required, body)
		}
	}
}

func TestDeleteStatusCodes(t *testing.T) {
	_, s, mux := newTestHandler(t)

	w := serve(mux, httptest.NewRequest(http.MethodDelete, "/items/missing", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("delete missing item: status = %d, want 404", w.Code)
	}

	if err := s.Create(model.Item{ID: "abc", Name: "Widget"}); err != nil {
		t.Fatal(err)
	}
	// Make writes fail by replacing the data directory with a file.
	dir := filepath.Dir(s.Path())
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, nil, 0644); err != nil {
		t.Fatal(err)
	}

	w = serve(mux, httptest.NewRequest(http.MethodDelete, "/items/abc", nil))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("delete with failing disk: status = %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), dir) {
		t.Errorf("error response leaks the server path: %q", w.Body.String())
	}
	if _, ok := s.Get("abc"); !ok {
		t.Error("item was removed from memory even though the delete failed")
	}
}

func TestEmptyCostRow(t *testing.T) {
	_, _, mux := newTestHandler(t)
	w := serve(mux, httptest.NewRequest(http.MethodGet, "/items/cost-row", nil))
	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(body, `name="cost_description"`) || !strings.Contains(body, `name="cost_amount"`) {
		t.Errorf("status %d, body:\n%s", w.Code, body)
	}
	if strings.Contains(body, `value="0"`) {
		t.Errorf("blank cost row should not prefill 0:\n%s", body)
	}
}
