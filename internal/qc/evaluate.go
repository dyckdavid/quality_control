// Package qc holds the inspection decision rules.
// Limits are inclusive. A missing limit on a variable check does not constrain that side.
package qc

import (
	"fmt"
	"math"

	"github.com/google/uuid"
)

const (
	Pass        = "pass"
	Fail        = "fail"
	Conditional = "conditional"
)

// Sample is one recorded reading after in_spec has been decided.
type Sample struct {
	InSpec   bool
	Critical bool
}

// Defect is one defect row already snapshotted with a severity.
type Defect struct {
	Severity string
	Quantity int
}

// Decision is the inspection outcome and the NCR severity to use when it fails.
type Decision struct {
	Result          string
	Severity        string
	OutOfSpec       int
	CriticalMisses  int
	MinorDefects    int
	MajorDefects    int
	CriticalDefects int
}

// InSpec decides a single sample. Attribute checks use pass/fail.
// Variable checks compare the measured value with inclusive limits.
func InSpec(charType string, measured *float64, attribute *string, lower, upper *float64) (bool, error) {
	switch charType {
	case "attribute":
		if attribute == nil || (*attribute != "pass" && *attribute != "fail") {
			return false, fmt.Errorf("attribute result must be pass or fail")
		}
		if measured != nil {
			return false, fmt.Errorf("attribute checks do not take a measured value")
		}
		return *attribute == "pass", nil
	case "variable":
		if attribute != nil {
			return false, fmt.Errorf("variable checks do not take an attribute result")
		}
		if measured == nil || math.IsNaN(*measured) || math.IsInf(*measured, 0) {
			return false, fmt.Errorf("measured value is required")
		}
		if lower != nil && *measured < *lower {
			return false, nil
		}
		if upper != nil && *measured > *upper {
			return false, nil
		}
		return true, nil
	default:
		return false, fmt.Errorf("characteristic type is not valid")
	}
}

// Evaluate rolls samples and defects into pass, fail, or conditional.
// Any out-of-spec sample, major or critical defect, or rejected quantity fails
// the inspection. Minor defects alone are conditional. rejectedQty is the
// quantity the inspector marked rejected.
func Evaluate(results []Sample, defects []Defect, rejectedQty float64) (Decision, error) {
	if len(results) == 0 {
		return Decision{}, fmt.Errorf("at least one result is required")
	}
	if math.IsNaN(rejectedQty) || math.IsInf(rejectedQty, 0) || rejectedQty < 0 {
		return Decision{}, fmt.Errorf("rejected quantity is not valid")
	}
	d := Decision{Result: Pass, Severity: ""}
	for _, r := range results {
		if !r.InSpec {
			d.OutOfSpec++
			if r.Critical {
				d.CriticalMisses++
			}
		}
	}
	for _, def := range defects {
		if def.Quantity <= 0 {
			continue
		}
		switch def.Severity {
		case "minor":
			d.MinorDefects += def.Quantity
		case "critical":
			d.CriticalDefects += def.Quantity
		default:
			d.MajorDefects += def.Quantity
		}
	}
	switch {
	case d.OutOfSpec > 0 || d.MajorDefects > 0 || d.CriticalDefects > 0 || rejectedQty > 0:
		d.Result = Fail
	case d.MinorDefects > 0:
		d.Result = Conditional
	default:
		d.Result = Pass
	}
	switch {
	case d.CriticalMisses > 0 || d.CriticalDefects > 0:
		d.Severity = "critical"
	case d.Result == Fail:
		d.Severity = "major"
	case d.Result == Conditional:
		d.Severity = "minor"
	}
	return d, nil
}

// Need is a characteristic that must be covered before an inspection can close.
type Need struct {
	ID   uuid.UUID
	Code string
	Type string
}

// Reading is a sample that was actually stored.
type Reading struct {
	CharacteristicID uuid.UUID
	SampleIndex      int
}

// MissingCoverage returns characteristic codes that do not yet have the
// required samples. Variable checks need every index from 1 through
// sampleSize. Attribute checks need sample 1.
func MissingCoverage(needs []Need, readings []Reading, sampleSize int) []string {
	if sampleSize < 1 {
		sampleSize = 1
	}
	have := map[uuid.UUID]map[int]bool{}
	for _, r := range readings {
		if have[r.CharacteristicID] == nil {
			have[r.CharacteristicID] = map[int]bool{}
		}
		have[r.CharacteristicID][r.SampleIndex] = true
	}
	var missing []string
	for _, n := range needs {
		samples := have[n.ID]
		if n.Type == "attribute" {
			if !samples[1] {
				missing = append(missing, n.Code)
			}
			continue
		}
		for i := 1; i <= sampleSize; i++ {
			if !samples[i] {
				missing = append(missing, n.Code)
				break
			}
		}
	}
	if missing == nil {
		missing = []string{}
	}
	return missing
}
