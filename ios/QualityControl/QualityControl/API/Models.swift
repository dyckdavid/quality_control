import Foundation

nonisolated enum Role: String, CaseIterable, Identifiable {
    case viewer
    case inspector = "inspector"
    case `operator` = "operator"
    case supervisor
    case qualityManager = "quality_manager"
    case admin

    var id: String { rawValue }

    var title: String {
        switch self {
        case .viewer: "Viewer"
        case .operator: "Operator"
        case .inspector: "Inspector"
        case .supervisor: "Supervisor"
        case .qualityManager: "Quality manager"
        case .admin: "Admin"
        }
    }

    var rank: Int {
        switch self {
        case .viewer: 0
        case .operator: 1
        case .inspector: 2
        case .supervisor: 3
        case .qualityManager: 4
        case .admin: 5
        }
    }

    static func named(_ raw: String) -> Role? {
        Role(rawValue: raw)
    }
}

nonisolated struct User: Codable, Identifiable, Hashable {
    var id: UUID
    var username: String
    var email: String
    var fullName: String
    var role: String
    var active: Bool
    var lastLoginAt: Date?
    var createdAt: Date
    var updatedAt: Date

    var roleValue: Role { Role.named(role) ?? .viewer }

    func allows(_ minimum: Role) -> Bool {
        roleValue.rank >= minimum.rank
    }
}

nonisolated struct SessionPayload: Decodable {
    var accessToken: String
    var refreshToken: String
    var tokenType: String
    var expiresIn: Int
    var user: User
}

nonisolated struct APIEnums: Decodable {
    var roles: [String] = Role.allCases.map(\.rawValue)
    var lotSources: [String] = ["incoming", "in_process", "final", "customer_return"]
    var lotStatuses: [String] = ["open", "in_inspection", "accepted", "rejected", "on_hold", "closed"]
    var planStatuses: [String] = ["draft", "active", "obsolete"]
    var samplingTypes: [String] = ["hundred_percent", "fixed", "aql"]
    var characteristicTypes: [String] = ["variable", "attribute"]
    var inspectionStatuses: [String] = ["in_progress", "completed", "cancelled"]
    var inspectionResults: [String] = ["pending", "pass", "fail", "conditional"]
    var severities: [String] = ["minor", "major", "critical"]
    var ncrStatuses: [String] = ["open", "under_review", "dispositioned", "closed", "void"]
    var dispositions: [String] = ["pending", "use_as_is", "rework", "repair", "scrap", "return_to_supplier", "sort"]
    var capaStatuses: [String] = ["open", "in_progress", "verification", "closed", "cancelled"]
    var attachmentEntities: [String] = ["lot", "inspection", "nonconformance", "corrective_action"]

    init() {}

    private enum CodingKeys: String, CodingKey {
        case roles, lotSources, lotStatuses, planStatuses, samplingTypes, characteristicTypes
        case inspectionStatuses, inspectionResults, severities, ncrStatuses, dispositions, capaStatuses, attachmentEntities
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.init()
        roles = try container.decodeIfPresent([String].self, forKey: .roles) ?? roles
        lotSources = try container.decodeIfPresent([String].self, forKey: .lotSources) ?? lotSources
        lotStatuses = try container.decodeIfPresent([String].self, forKey: .lotStatuses) ?? lotStatuses
        planStatuses = try container.decodeIfPresent([String].self, forKey: .planStatuses) ?? planStatuses
        samplingTypes = try container.decodeIfPresent([String].self, forKey: .samplingTypes) ?? samplingTypes
        characteristicTypes = try container.decodeIfPresent([String].self, forKey: .characteristicTypes) ?? characteristicTypes
        inspectionStatuses = try container.decodeIfPresent([String].self, forKey: .inspectionStatuses) ?? inspectionStatuses
        inspectionResults = try container.decodeIfPresent([String].self, forKey: .inspectionResults) ?? inspectionResults
        severities = try container.decodeIfPresent([String].self, forKey: .severities) ?? severities
        ncrStatuses = try container.decodeIfPresent([String].self, forKey: .ncrStatuses) ?? ncrStatuses
        dispositions = try container.decodeIfPresent([String].self, forKey: .dispositions) ?? dispositions
        capaStatuses = try container.decodeIfPresent([String].self, forKey: .capaStatuses) ?? capaStatuses
        attachmentEntities = try container.decodeIfPresent([String].self, forKey: .attachmentEntities) ?? attachmentEntities
    }
}

nonisolated struct Site: Codable, Identifiable, Hashable {
    var id: UUID
    var code: String
    var name: String
    var address: String
    var active: Bool
    var createdAt: Date
    var updatedAt: Date
}

nonisolated struct Station: Codable, Identifiable, Hashable {
    var id: UUID
    var siteId: UUID
    var code: String
    var name: String
    var description: String
    var active: Bool
    var createdAt: Date
    var updatedAt: Date
}

nonisolated struct Supplier: Codable, Identifiable, Hashable {
    var id: UUID
    var code: String
    var name: String
    var contactName: String
    var email: String
    var phone: String
    var active: Bool
    var createdAt: Date
    var updatedAt: Date
}

nonisolated struct Gauge: Codable, Identifiable, Hashable {
    var id: UUID
    var code: String
    var name: String
    var serialNumber: String
    var calibrationDue: String
    var active: Bool
    var createdAt: Date
    var updatedAt: Date
}

nonisolated struct Product: Codable, Identifiable, Hashable {
    var id: UUID
    var partNumber: String
    var name: String
    var description: String
    var unit: String
    var active: Bool
    var createdBy: UUID
    var createdAt: Date
    var updatedAt: Date
}

nonisolated struct DefectCode: Codable, Identifiable, Hashable {
    var id: UUID
    var code: String
    var name: String
    var severity: String
    var category: String
    var active: Bool
    var createdAt: Date
    var updatedAt: Date
}

nonisolated struct Characteristic: Codable, Identifiable, Hashable {
    var id: UUID
    var planId: UUID
    var sequence: Int
    var code: String
    var name: String
    var characteristicType: String
    var method: String
    var nominal: Double?
    var lowerLimit: Double?
    var upperLimit: Double?
    var unit: String
    var critical: Bool
    var createdAt: Date?
    var updatedAt: Date?
}

nonisolated struct InspectionPlan: Codable, Identifiable, Hashable {
    var id: UUID
    var productId: UUID
    var revision: String
    var name: String
    var status: String
    var samplingType: String
    var aql: String
    var sampleSize: Int?
    var notes: String
    var createdBy: UUID
    var approvedBy: UUID?
    var approvedAt: Date?
    var createdAt: Date
    var updatedAt: Date
}

nonisolated struct PlanDetail: Decodable, Identifiable {
    var plan: InspectionPlan
    var characteristics: [Characteristic]
    var id: UUID { plan.id }

    init(from decoder: Decoder) throws {
        plan = try InspectionPlan(from: decoder)
        let container = try decoder.container(keyedBy: CodingKeys.self)
        characteristics = try container.decodeIfPresent([Characteristic].self, forKey: .characteristics) ?? []
    }

    enum CodingKeys: String, CodingKey {
        case characteristics
    }
}

nonisolated struct Lot: Codable, Identifiable, Hashable {
    var id: UUID
    var lotNumber: String
    var productId: UUID
    var siteId: UUID
    var supplierId: UUID?
    var quantity: Double
    var unit: String
    var source: String
    var supplierLot: String
    var status: String
    var receivedAt: Date
    var notes: String
    var createdBy: UUID
    var createdAt: Date
    var updatedAt: Date
}

nonisolated struct Inspection: Codable, Identifiable, Hashable {
    var id: UUID
    var inspectionNumber: String
    var lotId: UUID
    var planId: UUID
    var stationId: UUID?
    var inspectorId: UUID
    var status: String
    var result: String
    var sampleSize: Int
    var quantityInspected: Double
    var quantityAccepted: Double
    var quantityRejected: Double
    var notes: String
    var startedAt: Date
    var completedAt: Date?
    var createdBy: UUID
    var createdAt: Date
    var updatedAt: Date
}

nonisolated struct InspectionResult: Codable, Identifiable, Hashable {
    var id: UUID
    var inspectionId: UUID
    var characteristicId: UUID
    var sampleIndex: Int
    var measuredValue: Double?
    var attributeResult: String?
    var inSpec: Bool
    var calibrationWarning: Bool
    var critical: Bool
    var nominal: Double?
    var lowerLimit: Double?
    var upperLimit: Double?
    var unit: String
    var characteristicCode: String
    var characteristicName: String
    var characteristicType: String
    var gaugeId: UUID?
    var notes: String
    var recordedBy: UUID
    var recordedAt: Date
    var updatedAt: Date
}

nonisolated struct InspectionDefect: Codable, Identifiable, Hashable {
    var id: UUID
    var inspectionId: UUID
    var defectCodeId: UUID
    var code: String
    var name: String
    var severity: String
    var quantity: Int
    var notes: String
    var recordedBy: UUID
    var createdAt: Date
}

nonisolated struct InspectionDetail: Decodable, Identifiable {
    var inspection: Inspection
    var results: [InspectionResult]
    var defects: [InspectionDefect]
    var missingCharacteristics: [String]
    var id: UUID { inspection.id }

    init(from decoder: Decoder) throws {
        inspection = try Inspection(from: decoder)
        let container = try decoder.container(keyedBy: CodingKeys.self)
        results = try container.decodeIfPresent([InspectionResult].self, forKey: .results) ?? []
        defects = try container.decodeIfPresent([InspectionDefect].self, forKey: .defects) ?? []
        missingCharacteristics = try container.decodeIfPresent([String].self, forKey: .missingCharacteristics) ?? []
    }

    enum CodingKeys: String, CodingKey {
        case results, defects, missingCharacteristics
    }
}

nonisolated struct CompletedInspection: Decodable {
    var inspection: Inspection
    var nonconformance: Nonconformance?

    init(from decoder: Decoder) throws {
        inspection = try Inspection(from: decoder)
        let container = try decoder.container(keyedBy: CodingKeys.self)
        nonconformance = try container.decodeIfPresent(Nonconformance.self, forKey: .nonconformance)
    }

    enum CodingKeys: String, CodingKey {
        case nonconformance
    }
}

nonisolated struct Nonconformance: Codable, Identifiable, Hashable {
    var id: UUID
    var ncrNumber: String
    var inspectionId: UUID?
    var lotId: UUID?
    var productId: UUID
    var title: String
    var description: String
    var severity: String
    var status: String
    var disposition: String
    var quantityAffected: Double
    var assignedTo: UUID?
    var openedBy: UUID
    var disposedBy: UUID?
    var closedBy: UUID?
    var dispositionNotes: String
    var openedAt: Date
    var disposedAt: Date?
    var closedAt: Date?
    var createdAt: Date
    var updatedAt: Date
}

nonisolated struct NCRDetail: Decodable, Identifiable {
    var ncr: Nonconformance
    var correctiveActions: [CorrectiveAction]
    var id: UUID { ncr.id }

    init(from decoder: Decoder) throws {
        ncr = try Nonconformance(from: decoder)
        let container = try decoder.container(keyedBy: CodingKeys.self)
        correctiveActions = try container.decodeIfPresent([CorrectiveAction].self, forKey: .correctiveActions) ?? []
    }

    enum CodingKeys: String, CodingKey {
        case correctiveActions
    }
}

nonisolated struct CorrectiveAction: Codable, Identifiable, Hashable {
    var id: UUID
    var caNumber: String
    var nonconformanceId: UUID?
    var title: String
    var rootCause: String
    var actionPlan: String
    var status: String
    var ownerId: UUID?
    var dueDate: String
    var verificationNotes: String
    var effectivenessNotes: String
    var openedBy: UUID
    var verifiedBy: UUID?
    var closedBy: UUID?
    var openedAt: Date
    var verifiedAt: Date?
    var closedAt: Date?
    var createdAt: Date
    var updatedAt: Date
}

nonisolated struct Attachment: Codable, Identifiable, Hashable {
    var id: UUID
    var entityType: String
    var entityId: UUID
    var fileName: String
    var contentType: String
    var sizeBytes: Int64
    var sha256: String
    var uploadedBy: UUID
    var createdAt: Date
}

nonisolated struct AuditEntry: Decodable, Identifiable, Hashable {
    var id: Int64
    var actorId: UUID?
    var action: String
    var entityType: String
    var entityId: UUID?
    var summary: String
    var ip: String
    var requestId: String
    var createdAt: Date
}

nonisolated struct Measurement: Decodable, Identifiable, Hashable {
    var id: String { "\(inspectionNumber)-\(characteristicCode)-\(sampleIndex)-\(recordedAt.timeIntervalSince1970)" }
    var recordedAt: Date
    var lotNumber: String
    var inspectionNumber: String
    var characteristicCode: String
    var characteristicName: String
    var sampleIndex: Int
    var measuredValue: Double
    var inSpec: Bool
    var unit: String
    var nominal: Double?
    var lowerLimit: Double?
    var upperLimit: Double?
    var critical: Bool
}

nonisolated struct SeverityCount: Decodable, Hashable {
    var severity: String
    var count: Int
}

nonisolated struct Dashboard: Decodable {
    var openInspections: Int
    var lotsOnHold: Int
    var openNonconformances: Int
    var openCorrectiveActions: Int
    var overdueCorrectiveActions: Int
    var inspectionsLast30Days: Int
    var passLast30Days: Int
    var failLast30Days: Int
    var conditionalLast30Days: Int
    var passRateLast30Days: Double?
    var defectsBySeverity: [SeverityCount]
}
