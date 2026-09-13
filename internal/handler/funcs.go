package handler

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"time"
)

func templateFuncs(version string) template.FuncMap {
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
		// formatNumberInput renders a float for an <input type="number" value=...>.
		// Zero becomes empty so the field's placeholder shows on a blank form, and
		// 'f' formatting avoids the scientific notation %v would produce for large
		// or small magnitudes (e.g. 1000000 rendering as "1e+06").
		"formatNumberInput": func(f float64) string {
			if f == 0 {
				return ""
			}
			return strconv.FormatFloat(f, 'f', -1, 64)
		},
		"lower":   strings.ToLower,
		"version": func() string { return version },
	}
}
