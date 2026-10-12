package report

import (
	"bytes"
	"testing"
)

func TestWriteCSVAndPDF(t *testing.T) {
	csvBytes, err := WriteCSV([]string{"a", "b"}, [][]string{{"1", "2"}, {"3", "hi,there"}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(csvBytes, []byte("a,b")) {
		t.Fatalf("csv missing header: %s", csvBytes)
	}
	doc := NewPDF("Test report")
	doc.AddHeading("Section")
	doc.AddKV("Count", "3")
	doc.AddLine(`line with (parens) and \slash`)
	pdf := doc.Bytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) {
		t.Fatalf("bad pdf prefix: %q", pdf[:min(20, len(pdf))])
	}
	if !bytes.Contains(pdf, []byte("%%EOF")) {
		t.Fatal("missing EOF")
	}
}
