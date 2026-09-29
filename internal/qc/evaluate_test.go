package qc

import (
	"math"
	"testing"

	"github.com/google/uuid"
)

func TestInSpecLimitsAreInclusive(t *testing.T) {
	lo, hi := 9.9, 10.1
	ok, err := InSpec("variable", &lo, nil, &lo, &hi)
	if err != nil || !ok {
		t.Fatalf("lower bound: ok=%v err=%v", ok, err)
	}
	ok, err = InSpec("variable", &hi, nil, &lo, &hi)
	if err != nil || !ok {
		t.Fatalf("upper bound: ok=%v err=%v", ok, err)
	}
	below := 9.89
	ok, err = InSpec("variable", &below, nil, &lo, &hi)
	if err != nil || ok {
		t.Fatalf("below: ok=%v err=%v", ok, err)
	}
	pass := "pass"
	ok, err = InSpec("attribute", nil, &pass, nil, nil)
	if err != nil || !ok {
		t.Fatalf("attribute pass: ok=%v err=%v", ok, err)
	}
	if _, err := InSpec("variable", nil, nil, nil, nil); err == nil {
		t.Fatal("expected missing measurement to fail")
	}
	nan := math.NaN()
	if _, err := InSpec("variable", &nan, nil, nil, nil); err == nil {
		t.Fatal("expected NaN to fail")
	}
}

func TestEvaluate(t *testing.T) {
	pass, err := Evaluate([]Sample{{InSpec: true}}, nil, 0)
	if err != nil || pass.Result != Pass {
		t.Fatalf("pass: %+v %v", pass, err)
	}
	fail, err := Evaluate([]Sample{{InSpec: false, Critical: true}}, nil, 0)
	if err != nil || fail.Result != Fail || fail.Severity != "critical" {
		t.Fatalf("critical miss: %+v %v", fail, err)
	}
	cond, err := Evaluate([]Sample{{InSpec: true}}, []Defect{{Severity: "minor", Quantity: 2}}, 0)
	if err != nil || cond.Result != Conditional || cond.Severity != "minor" {
		t.Fatalf("conditional: %+v %v", cond, err)
	}
	major, err := Evaluate([]Sample{{InSpec: true}}, []Defect{{Severity: "major", Quantity: 1}}, 0)
	if err != nil || major.Result != Fail || major.Severity != "major" {
		t.Fatalf("major: %+v %v", major, err)
	}
	rejected, err := Evaluate([]Sample{{InSpec: true}}, nil, 1)
	if err != nil || rejected.Result != Fail {
		t.Fatalf("rejected qty: %+v %v", rejected, err)
	}
	if _, err := Evaluate(nil, nil, 0); err == nil {
		t.Fatal("expected empty results to fail")
	}
}

func TestMissingCoverage(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()
	needs := []Need{{ID: id1, Code: "LEN", Type: "variable"}, {ID: id2, Code: "VIS", Type: "attribute"}}
	got := MissingCoverage(needs, []Reading{{CharacteristicID: id1, SampleIndex: 1}}, 2)
	if len(got) != 2 || got[0] != "LEN" || got[1] != "VIS" {
		t.Fatalf("missing: %#v", got)
	}
	got = MissingCoverage(needs, []Reading{
		{CharacteristicID: id1, SampleIndex: 1},
		{CharacteristicID: id1, SampleIndex: 2},
		{CharacteristicID: id2, SampleIndex: 1},
	}, 2)
	if len(got) != 0 {
		t.Fatalf("expected covered, got %#v", got)
	}
}
