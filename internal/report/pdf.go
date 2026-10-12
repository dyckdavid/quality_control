package report

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

// PDFDoc is a minimal single-page (or multi-page) text PDF built without external deps.
type PDFDoc struct {
	title string
	lines []string
}

// NewPDF starts a document.
func NewPDF(title string) *PDFDoc {
	return &PDFDoc{title: title}
}

// AddHeading appends a section heading.
func (d *PDFDoc) AddHeading(text string) {
	d.lines = append(d.lines, "", strings.ToUpper(text), strings.Repeat("-", min(60, len(text)+8)))
}

// AddLine appends a body line.
func (d *PDFDoc) AddLine(text string) {
	for _, part := range wrapLine(text, 90) {
		d.lines = append(d.lines, part)
	}
}

// AddKV appends "label: value".
func (d *PDFDoc) AddKV(label, value string) {
	d.AddLine(label + ": " + value)
}

// Bytes renders the PDF.
func (d *PDFDoc) Bytes() []byte {
	var content bytes.Buffer
	content.WriteString("BT\n/F1 11 Tf\n14 TL\n50 780 Td\n")
	header := sanitizePDFText(fmt.Sprintf("%s  |  generated %s UTC", d.title, time.Now().UTC().Format(time.RFC3339)))
	content.WriteString("(" + header + ") Tj\nT*\nT*\n")
	for _, line := range d.lines {
		content.WriteString("(" + sanitizePDFText(line) + ") Tj\nT*\n")
	}
	content.WriteString("ET\n")
	stream := content.Bytes()

	var buf bytes.Buffer
	write := func(s string) { buf.WriteString(s) }
	offsets := make([]int, 0, 8)
	write("%PDF-1.4\n")
	obj := func(n int, body string) {
		offsets = append(offsets, buf.Len())
		write(fmt.Sprintf("%d 0 obj\n%s\nendobj\n", n, body))
	}
	obj(1, "<< /Type /Catalog /Pages 2 0 R >>")
	obj(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	obj(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>")
	obj(4, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	obj(5, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	xrefPos := buf.Len()
	write(fmt.Sprintf("xref\n0 %d\n", len(offsets)+1))
	write("0000000000 65535 f \n")
	for _, off := range offsets {
		write(fmt.Sprintf("%010d 00000 n \n", off))
	}
	write(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xrefPos))
	return buf.Bytes()
}

func sanitizePDFText(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "(", "\\(")
	s = strings.ReplaceAll(s, ")", "\\)")
	s = strings.Map(func(r rune) rune {
		if r < 32 || r > 126 {
			return '?'
		}
		return r
	}, s)
	return s
}

func wrapLine(s string, width int) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return []string{""}
	}
	var out []string
	for len(s) > width {
		out = append(out, s[:width])
		s = s[width:]
	}
	out = append(out, s)
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
