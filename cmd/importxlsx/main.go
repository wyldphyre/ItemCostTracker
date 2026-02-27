// importxlsx converts the "Item Cost per Time Period" Excel spreadsheet to
// the ItemCostTracker JSON import format.
//
// Usage:
//
//	go run ./cmd/importxlsx <path/to/spreadsheet.xlsx> [output.json]
//
// If output.json is omitted, JSON is written to stdout.
// To import directly: go run ./cmd/importxlsx spreadsheet.xlsx data/items.json
package main

import (
	"archive/zip"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"itemcosttracker/internal/model"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: importxlsx <spreadsheet.xlsx> [output.json]\n")
		os.Exit(1)
	}

	xlsxPath := os.Args[1]
	outPath := ""
	if len(os.Args) >= 3 {
		outPath = os.Args[2]
	}

	items, err := parseXLSX(xlsxPath)
	if err != nil {
		log.Fatalf("Error parsing spreadsheet: %v", err)
	}

	log.Printf("Parsed %d items from %s", len(items), xlsxPath)

	payload := struct {
		Items []model.Item `json:"items"`
	}{Items: items}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		log.Fatalf("Error encoding JSON: %v", err)
	}

	if outPath == "" {
		os.Stdout.Write(data)
		fmt.Println()
	} else {
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			log.Fatalf("Error writing output: %v", err)
		}
		log.Printf("Written to %s", outPath)
	}
}

// ── xlsx parsing ─────────────────────────────────────────────────────────────

func parseXLSX(path string) ([]model.Item, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("opening xlsx: %w", err)
	}
	defer r.Close()

	sharedStrings, err := readSharedStrings(r)
	if err != nil {
		return nil, fmt.Errorf("reading shared strings: %w", err)
	}

	rows, err := readSheet(r)
	if err != nil {
		return nil, fmt.Errorf("reading sheet: %w", err)
	}

	return convertRows(rows, sharedStrings)
}

// ── Shared strings ────────────────────────────────────────────────────────────

type ssXML struct {
	SI []ssItem `xml:"si"`
}

type ssItem struct {
	T  string   `xml:"t"`
	Rs []ssRich `xml:"r"`
}

type ssRich struct {
	T string `xml:"t"`
}

func (s ssItem) text() string {
	if s.T != "" {
		return s.T
	}
	var b strings.Builder
	for _, r := range s.Rs {
		b.WriteString(r.T)
	}
	return b.String()
}

func readSharedStrings(r *zip.ReadCloser) ([]string, error) {
	f := findFile(r, "xl/sharedStrings.xml")
	if f == nil {
		return nil, nil
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	var ss ssXML
	if err := xml.Unmarshal(data, &ss); err != nil {
		return nil, err
	}

	result := make([]string, len(ss.SI))
	for i, si := range ss.SI {
		result[i] = si.text()
	}
	return result, nil
}

// ── Sheet parsing ─────────────────────────────────────────────────────────────

type sheetXML struct {
	SheetData sheetData `xml:"sheetData"`
}

type sheetData struct {
	Rows []row `xml:"row"`
}

type row struct {
	R    int    `xml:"r,attr"`
	Cells []cell `xml:"c"`
}

type cell struct {
	R string `xml:"r,attr"` // e.g. "A2"
	T string `xml:"t,attr"` // "s" = shared string, "" = number
	V string `xml:"v"`
}

func readSheet(r *zip.ReadCloser) ([]row, error) {
	f := findFile(r, "xl/worksheets/sheet1.xml")
	if f == nil {
		return nil, fmt.Errorf("sheet1.xml not found")
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	var sheet sheetXML
	if err := xml.Unmarshal(data, &sheet); err != nil {
		return nil, err
	}

	return sheet.SheetData.Rows, nil
}

func findFile(r *zip.ReadCloser, name string) *zip.File {
	for _, f := range r.File {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// ── Column mapping ────────────────────────────────────────────────────────────

// Column indices (0-based) for the spreadsheet
const (
	colItem           = 0  // A
	colPurchaseDate   = 1  // B
	colPurchasePrice  = 2  // C
	colFinalDate      = 3  // D
	colResaleValue    = 4  // E
	colAdditionalCost = 5  // F
	colProjectedYears = 13 // N
)

// colIndex converts a column letter (e.g. "A", "B", "AA") to a 0-based index.
func colIndex(col string) int {
	result := 0
	for _, c := range col {
		result = result*26 + int(c-'A'+1)
	}
	return result - 1
}

// colFromRef extracts the column letters from a cell reference like "A2" → "A".
func colFromRef(ref string) string {
	for i, c := range ref {
		if c >= '0' && c <= '9' {
			return ref[:i]
		}
	}
	return ref
}

// ── Row conversion ────────────────────────────────────────────────────────────

func convertRows(rows []row, sharedStrings []string) ([]model.Item, error) {
	now := time.Now().UTC()
	var items []model.Item

	for _, r := range rows {
		if r.R == 1 {
			continue // skip header row
		}

		// Build a map of colIndex → cell for this row
		cells := make(map[int]cell, len(r.Cells))
		for _, c := range r.Cells {
			col := colFromRef(c.R)
			cells[colIndex(col)] = c
		}

		name := cellString(cells, colItem, sharedStrings)
		if name == "" {
			continue // skip empty rows
		}

		purchaseDate, err := cellDate(cells, colPurchaseDate)
		if err != nil || purchaseDate.IsZero() {
			log.Printf("Row %d: skipping %q — invalid purchase date", r.R, name)
			continue
		}

		purchasePrice := cellFloat(cells, colPurchasePrice)
		finalDate := cellDatePtr(cells, colFinalDate)
		resaleValue := cellFloat(cells, colResaleValue)
		additionalCostAmt := cellFloat(cells, colAdditionalCost)
		projectedYears := cellFloat(cells, colProjectedYears)

		var additionalCosts []model.AdditionalCost
		if additionalCostAmt != 0 {
			additionalCosts = []model.AdditionalCost{
				{Description: "Additional", Amount: additionalCostAmt},
			}
		}

		items = append(items, model.Item{
			ID:                model.NewUUID(),
			Name:              name,
			PurchaseDate:      purchaseDate,
			PurchasePrice:     purchasePrice,
			FinalActivityDate: finalDate,
			ResaleValue:       resaleValue,
			AdditionalCosts:   additionalCosts,
			ProjectedYears:    projectedYears,
			CreatedAt:         now,
			UpdatedAt:         now,
		})
	}

	return items, nil
}

// ── Cell value helpers ────────────────────────────────────────────────────────

func cellString(cells map[int]cell, col int, shared []string) string {
	c, ok := cells[col]
	if !ok || c.V == "" {
		return ""
	}
	if c.T == "s" {
		idx, err := strconv.Atoi(c.V)
		if err != nil || idx >= len(shared) {
			return ""
		}
		return shared[idx]
	}
	return c.V
}

func cellFloat(cells map[int]cell, col int) float64 {
	c, ok := cells[col]
	if !ok || c.V == "" {
		return 0
	}
	f, err := strconv.ParseFloat(c.V, 64)
	if err != nil {
		return 0
	}
	return f
}

func cellDate(cells map[int]cell, col int) (time.Time, error) {
	f := cellFloat(cells, col)
	if f == 0 {
		return time.Time{}, nil
	}
	return excelDateToTime(f), nil
}

func cellDatePtr(cells map[int]cell, col int) *time.Time {
	f := cellFloat(cells, col)
	if f == 0 {
		return nil
	}
	t := excelDateToTime(f)
	return &t
}

// excelDateToTime converts an Excel serial date (Windows epoch) to time.Time.
// Excel serial 1 = Jan 1, 1900. There is a bug where 1900 is treated as a leap year,
// so serial 60 = Feb 29, 1900 (invalid). The formula date(1899,12,30) + N days corrects this.
func excelDateToTime(serial float64) time.Time {
	epoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	days := int(math.Round(serial))
	return epoch.AddDate(0, 0, days)
}
