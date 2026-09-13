package main

import "testing"

func TestCellFloatIgnoresNonNumericCells(t *testing.T) {
	cells := map[int]cell{
		0: {T: "", V: "44197"},     // plain number
		1: {T: "n", V: "12.5"},     // explicit number
		2: {T: "str", V: "3"},      // formula result
		3: {T: "s", V: "7"},        // shared-string index, not the number 7
		4: {T: "b", V: "1"},        // boolean
		5: {T: "e", V: "#DIV/0!"},  // error
		6: {T: "inlineStr", V: ""}, // inline string
	}
	want := map[int]float64{0: 44197, 1: 12.5, 2: 3, 3: 0, 4: 0, 5: 0, 6: 0}
	for col, w := range want {
		if got := cellFloat(cells, col); got != w {
			t.Errorf("col %d (t=%q): cellFloat = %v, want %v", col, cells[col].T, got, w)
		}
	}
}

// A text value in the final-date column must leave the item active rather
// than dating its end to 1900.
func TestTextFinalDateIsNotADate(t *testing.T) {
	cells := map[int]cell{colFinalDate: {T: "s", V: "7"}}
	if got := cellDatePtr(cells, colFinalDate); got != nil {
		t.Errorf("cellDatePtr on a text cell = %v, want nil", got)
	}
}
