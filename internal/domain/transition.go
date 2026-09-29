package domain

// CanManualLotTransition is the set of lot moves a person can request.
// Entering and leaving in_inspection is reserved for inspection start,
// complete, and cancel.
func CanManualLotTransition(from, to string) bool {
	if from == to {
		return false
	}
	switch from {
	case LotOpen:
		return to == LotOnHold || to == LotClosed
	case LotOnHold:
		return to == LotOpen || to == LotAccepted || to == LotRejected || to == LotClosed
	case LotAccepted, LotRejected:
		return to == LotOnHold || to == LotClosed
	case LotClosed:
		return to == LotOpen
	default:
		return false
	}
}

// CanNCRTransition is the allowed NCR status graph.
func CanNCRTransition(from, to string) bool {
	switch from {
	case NCROpen:
		return to == NCRUnderReview || to == NCRDispositioned || to == NCRVoid
	case NCRUnderReview:
		return to == NCRDispositioned || to == NCRVoid
	case NCRDispositioned:
		return to == NCRClosed
	default:
		return false
	}
}

// CanCAPATransition is the allowed corrective-action status graph.
func CanCAPATransition(from, to string) bool {
	switch from {
	case CAPAOpen:
		return to == CAPAInProgress || to == CAPACancelled
	case CAPAInProgress:
		return to == CAPAVerification || to == CAPACancelled
	case CAPAVerification:
		return to == CAPAClosed || to == CAPAInProgress
	default:
		return false
	}
}
