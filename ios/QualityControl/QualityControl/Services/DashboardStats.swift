import Foundation

/// Client-side QC metrics derived from list payloads (and optionally merged with `/dashboard`).
nonisolated struct ClientDashboardStats: Equatable, Codable {
    var openInspections: Int = 0
    var lotsOnHold: Int = 0
    var openLots: Int = 0
    var openNonconformances: Int = 0
    var openCorrectiveActions: Int = 0
    var overdueCorrectiveActions: Int = 0
    var inspectionsLast30Days: Int = 0
    var passLast30Days: Int = 0
    var failLast30Days: Int = 0
    var conditionalLast30Days: Int = 0
    var criticalNCRs: Int = 0
    var majorNCRs: Int = 0
    var minorNCRs: Int = 0

    var passRateLast30Days: Double? {
        let decided = passLast30Days + failLast30Days + conditionalLast30Days
        guard decided > 0 else { return nil }
        return Double(passLast30Days) / Double(decided)
    }

    var defectsBySeverity: [SeverityCount] {
        [
            SeverityCount(severity: "critical", count: criticalNCRs),
            SeverityCount(severity: "major", count: majorNCRs),
            SeverityCount(severity: "minor", count: minorNCRs)
        ].filter { $0.count > 0 }
    }

    static func compute(
        lots: [Lot],
        inspections: [Inspection],
        nonconformances: [Nonconformance],
        correctiveActions: [CorrectiveAction],
        now: Date = Date(),
        calendar: Calendar = .current
    ) -> ClientDashboardStats {
        let cutoff = calendar.date(byAdding: .day, value: -30, to: now) ?? now
        var stats = ClientDashboardStats()
        stats.openInspections = inspections.filter { $0.status == "in_progress" }.count
        stats.lotsOnHold = lots.filter { $0.status == "on_hold" }.count
        stats.openLots = lots.filter { $0.status == "open" || $0.status == "in_inspection" }.count
        stats.openNonconformances = nonconformances.filter {
            $0.status == "open" || $0.status == "under_review" || $0.status == "dispositioned"
        }.count
        stats.openCorrectiveActions = correctiveActions.filter {
            $0.status == "open" || $0.status == "in_progress" || $0.status == "verification"
        }.count
        stats.overdueCorrectiveActions = correctiveActions.filter { action in
            guard !action.dueDate.isEmpty,
                  action.status != "closed",
                  action.status != "cancelled",
                  let due = parseDay(action.dueDate) else { return false }
            return due < calendar.startOfDay(for: now)
        }.count

        let recent = inspections.filter { $0.startedAt >= cutoff }
        stats.inspectionsLast30Days = recent.count
        for inspection in recent where inspection.status == "completed" {
            switch inspection.result {
            case "pass": stats.passLast30Days += 1
            case "fail": stats.failLast30Days += 1
            case "conditional": stats.conditionalLast30Days += 1
            default: break
            }
        }

        for ncr in nonconformances where ncr.status != "void" && ncr.status != "closed" {
            switch ncr.severity {
            case "critical": stats.criticalNCRs += 1
            case "major": stats.majorNCRs += 1
            case "minor": stats.minorNCRs += 1
            default: break
            }
        }
        return stats
    }

    /// Prefer live list math; fill gaps from the server dashboard when lists are thin.
    static func merge(client: ClientDashboardStats, server: Dashboard?) -> ClientDashboardStats {
        guard let server else { return client }
        var merged = client
        if client.openInspections == 0 { merged.openInspections = server.openInspections }
        if client.lotsOnHold == 0 { merged.lotsOnHold = server.lotsOnHold }
        if client.openNonconformances == 0 { merged.openNonconformances = server.openNonconformances }
        if client.openCorrectiveActions == 0 { merged.openCorrectiveActions = server.openCorrectiveActions }
        if client.overdueCorrectiveActions == 0 { merged.overdueCorrectiveActions = server.overdueCorrectiveActions }
        if client.inspectionsLast30Days == 0 {
            merged.inspectionsLast30Days = server.inspectionsLast30Days
            merged.passLast30Days = server.passLast30Days
            merged.failLast30Days = server.failLast30Days
            merged.conditionalLast30Days = server.conditionalLast30Days
        }
        return merged
    }

    private static func parseDay(_ value: String) -> Date? {
        let formatter = DateFormatter()
        formatter.calendar = Calendar(identifier: .gregorian)
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = TimeZone(secondsFromGMT: 0)
        formatter.dateFormat = "yyyy-MM-dd"
        return formatter.date(from: value)
    }
}
