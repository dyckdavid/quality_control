package report

import (
	"strings"
	"testing"
)

func TestSafeCell(t *testing.T) {
	cases := map[string]string{
		"":                   "",
		"Weld miss":          "Weld miss",
		"=HYPERLINK(\"x\")":  "'=HYPERLINK(\"x\")",
		"+1+cmd":             "'+1+cmd",
		"@SUM(A1)":           "'@SUM(A1)",
		"-2.5":               "-2.5",
		"-cmd|' /C calc'!A0": "'-cmd|' /C calc'!A0",
	}
	for in, want := range cases {
		if got := SafeCell(in); got != want {
			t.Errorf("SafeCell(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWriteCSVEscapesFormulas(t *testing.T) {
	out, err := WriteCSV([]string{"title"}, [][]string{{"=1+1"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "'=1+1") {
		t.Fatalf("formula was not escaped: %q", out)
	}
}
