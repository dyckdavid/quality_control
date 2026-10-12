import Foundation
import Testing
@testable import QualityControl

struct QualityControlTests {

    @Test func scannedLabelsResolveToTheirCode() {
        #expect(ScanCode.clean("  CHUTE-101 \n") == "CHUTE-101")
        #expect(ScanCode.clean("https://shop.example/qc/lots/GATE-12") == "GATE-12")
        #expect(ScanCode.clean("https://shop.example/scan?code=TAPE-25") == "TAPE-25")
    }

    @Test func editorSendsOnlyChangedFieldsWithTypes() throws {
        let row: JSONObject = ["part_number": "GATE-12", "name": "Gate", "unit": "ea", "description": "", "active": true]
        var values = RecordForm.values(for: .products, row: row, editing: true)
        let original = values
        values["name"] = "Corral gate 12 ft"
        values["active"] = "false"
        let body = try RecordForm.body(for: .products, values: values, original: original)
        #expect(body.count == 2)
        #expect(body["name"] as? String == "Corral gate 12 ft")
        #expect(body["active"] as? Bool == false)
    }

    @Test func editorKeepsLargeNumbersExact() throws {
        let row: JSONObject = ["lot_number": "L1", "quantity": 1200, "unit": "ea", "supplier_lot": "", "notes": ""]
        var values = RecordForm.values(for: .lots, row: row, editing: true)
        #expect(values["quantity"] == "1200")
        let original = values
        values["quantity"] = "1250.5"
        let body = try RecordForm.body(for: .lots, values: values, original: original)
        #expect(body["quantity"] as? Double == 1250.5)
    }

    @Test func leavingFixedSamplingClearsTheSize() throws {
        let row: JSONObject = ["name": "Final", "sampling_type": "fixed", "sample_size": 5, "notes": ""]
        var values = RecordForm.values(for: .plans, row: row, editing: true)
        let original = values
        values["sampling_type"] = "hundred_percent"
        let body = try RecordForm.body(for: .plans, values: values, original: original)
        #expect(body["clear_sample_size"] as? Bool == true)
        #expect(body["sample_size"] == nil)
    }

    @Test func requiredFieldsCannotBeBlanked() {
        let row: JSONObject = ["code": "SHOP", "name": "Shop", "address": "", "active": true]
        var values = RecordForm.values(for: .sites, row: row, editing: true)
        let original = values
        values["code"] = "  "
        #expect(throws: APIFailure.self) { try RecordForm.body(for: .sites, values: values, original: original) }
    }

    @Test func clientDashboardComputesOpenWorkAndPassRate() throws {
        let now = Date(timeIntervalSince1970: 1_700_000_000)
        let recent = now.addingTimeInterval(-86_400)
        let older = now.addingTimeInterval(-40 * 86_400)

        let lots = [
            makeLot(status: "on_hold"),
            makeLot(status: "open"),
            makeLot(status: "accepted")
        ]
        let inspections = [
            makeInspection(status: "in_progress", result: "pending", startedAt: recent),
            makeInspection(status: "completed", result: "pass", startedAt: recent),
            makeInspection(status: "completed", result: "fail", startedAt: recent),
            makeInspection(status: "completed", result: "pass", startedAt: older)
        ]
        let ncrs = [
            makeNCR(severity: "critical", status: "open"),
            makeNCR(severity: "major", status: "under_review"),
            makeNCR(severity: "minor", status: "closed")
        ]
        let capas = [
            makeCAPA(status: "open", dueDate: "2020-01-01"),
            makeCAPA(status: "closed", dueDate: "2020-01-01")
        ]

        let stats = ClientDashboardStats.compute(
            lots: lots,
            inspections: inspections,
            nonconformances: ncrs,
            correctiveActions: capas,
            now: now
        )

        #expect(stats.openInspections == 1)
        #expect(stats.lotsOnHold == 1)
        #expect(stats.openLots == 1)
        #expect(stats.openNonconformances == 2)
        #expect(stats.openCorrectiveActions == 1)
        #expect(stats.overdueCorrectiveActions == 1)
        #expect(stats.inspectionsLast30Days == 3)
        #expect(stats.passLast30Days == 1)
        #expect(stats.failLast30Days == 1)
        #expect(stats.criticalNCRs == 1)
        #expect(stats.majorNCRs == 1)
        #expect(stats.passRateLast30Days == 0.5)
    }

    @Test func displayNameReplacesUnderscores() {
        #expect(displayName("in_progress") == "in progress")
        #expect(parseNumber("12,5") == 12.5)
        #expect(parseNumber("abc") == nil)
    }

    @Test func mergePrefersClientWhenPopulated() {
        var client = ClientDashboardStats()
        client.openInspections = 4
        let server = Dashboard(
            openInspections: 9,
            lotsOnHold: 2,
            openNonconformances: 3,
            openCorrectiveActions: 1,
            overdueCorrectiveActions: 0,
            inspectionsLast30Days: 10,
            passLast30Days: 8,
            failLast30Days: 1,
            conditionalLast30Days: 1,
            passRateLast30Days: 0.8,
            defectsBySeverity: []
        )
        let merged = ClientDashboardStats.merge(client: client, server: server)
        #expect(merged.openInspections == 4)
        #expect(merged.lotsOnHold == 2)
    }
}

private func makeLot(status: String) -> Lot {
    Lot(
        id: UUID(),
        lotNumber: "L-1",
        productId: UUID(),
        siteId: UUID(),
        supplierId: nil,
        quantity: 10,
        unit: "ea",
        source: "incoming",
        supplierLot: "",
        status: status,
        receivedAt: Date(),
        notes: "",
        createdBy: UUID(),
        createdAt: Date(),
        updatedAt: Date()
    )
}

private func makeInspection(status: String, result: String, startedAt: Date) -> Inspection {
    Inspection(
        id: UUID(),
        inspectionNumber: "I-1",
        lotId: UUID(),
        planId: UUID(),
        stationId: nil,
        inspectorId: UUID(),
        status: status,
        result: result,
        sampleSize: 1,
        quantityInspected: 0,
        quantityAccepted: 0,
        quantityRejected: 0,
        notes: "",
        startedAt: startedAt,
        completedAt: nil,
        createdBy: UUID(),
        createdAt: startedAt,
        updatedAt: startedAt
    )
}

private func makeNCR(severity: String, status: String) -> Nonconformance {
    Nonconformance(
        id: UUID(),
        ncrNumber: "N-1",
        inspectionId: nil,
        lotId: nil,
        productId: UUID(),
        title: "Title",
        description: "Desc",
        severity: severity,
        status: status,
        disposition: "pending",
        quantityAffected: 1,
        assignedTo: nil,
        openedBy: UUID(),
        disposedBy: nil,
        closedBy: nil,
        dispositionNotes: "",
        openedAt: Date(),
        disposedAt: nil,
        closedAt: nil,
        createdAt: Date(),
        updatedAt: Date()
    )
}

private func makeCAPA(status: String, dueDate: String) -> CorrectiveAction {
    CorrectiveAction(
        id: UUID(),
        caNumber: "C-1",
        nonconformanceId: nil,
        title: "Action",
        rootCause: "",
        actionPlan: "",
        status: status,
        ownerId: nil,
        dueDate: dueDate,
        verificationNotes: "",
        effectivenessNotes: "",
        openedBy: UUID(),
        verifiedBy: nil,
        closedBy: nil,
        openedAt: Date(),
        verifiedAt: nil,
        closedAt: nil,
        createdAt: Date(),
        updatedAt: Date()
    )
}
