import Foundation
import Testing
@testable import QualityControl

struct QualityControlTests {

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
