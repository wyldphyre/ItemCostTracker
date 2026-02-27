package handler

import (
	"fmt"
	"html/template"
	"strings"
	"time"
)

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"formatDate": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("2006-01-02")
		},
		"formatDatePtr": func(t *time.Time) string {
			if t == nil {
				return ""
			}
			return t.Format("2006-01-02")
		},
		"formatDateInput": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("2006-01-02")
		},
		"formatDateInputPtr": func(t *time.Time) string {
			if t == nil {
				return ""
			}
			return t.Format("2006-01-02")
		},
		"formatCurrency": func(f float64) string {
			return fmt.Sprintf("$%.2f", f)
		},
		// formatCurrencyOpt shows "-" for zero values (optional fields)
		"formatCurrencyOpt": func(f float64) string {
			if f == 0 {
				return "-"
			}
			return fmt.Sprintf("$%.2f", f)
		},
		"formatDecimal": func(precision int, f float64) string {
			return fmt.Sprintf("%.*f", precision, f)
		},
		// dict creates a map for passing multiple values to sub-templates
		"dict": func(values ...any) (map[string]any, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("dict requires even number of arguments")
			}
			m := make(map[string]any, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				m[key] = values[i+1]
			}
			return m, nil
		},
		"lower": strings.ToLower,
	}
}
