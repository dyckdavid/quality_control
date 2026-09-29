package domain

import "testing"

func TestLotTransitions(t *testing.T) {
	if !CanManualLotTransition(LotOpen, LotOnHold) {
		t.Fatal("open to hold")
	}
	if CanManualLotTransition(LotInInspection, LotAccepted) {
		t.Fatal("inspection status is not a manual move")
	}
	if !CanNCRTransition(NCROpen, NCRDispositioned) {
		t.Fatal("open ncr can be dispositioned")
	}
	if CanNCRTransition(NCRClosed, NCROpen) {
		t.Fatal("closed ncr stays closed")
	}
	if !CanCAPATransition(CAPAVerification, CAPAClosed) || !CanCAPATransition(CAPAVerification, CAPAInProgress) {
		t.Fatal("verification can close or return")
	}
}
