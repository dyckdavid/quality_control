import SwiftUI

typealias JSONObject = [String: Any]

enum FieldKind: Equatable {
    case text, long, number, whole, date, toggle, secret, events, email
    case choice([String])
    case product, supplier, site, user
}

struct FieldSpec: Identifiable {
    var key: String
    var label: String
    var kind: FieldKind = .text
    var onCreate = true
    var onEdit = true
    var required = false
    /// Show a scan button so the value can come from a barcode or QR label.
    var scannable = false
    var id: String { key }
}

/// Every table the app can list, create, edit, or delete through one generic screen.
enum RecordKind: String, CaseIterable, Identifiable, Hashable {
    case lots, products, sites, stations, suppliers, gauges
    case defectCodes = "defect-codes"
    case checklists, webhooks, users, plans, nonconformances
    case correctiveActions = "corrective-actions"

    var id: String { rawValue }
    var path: String { "/api/v1/\(rawValue)" }

    var title: String {
        switch self {
        case .lots: "Lots"
        case .products: "Products"
        case .sites: "Sites"
        case .stations: "Stations"
        case .suppliers: "Suppliers"
        case .gauges: "Gauges"
        case .defectCodes: "Defect codes"
        case .checklists: "Checklists"
        case .webhooks: "Webhooks"
        case .users: "Users"
        case .plans: "Plans"
        case .nonconformances: "Alerts"
        case .correctiveActions: "Corrective actions"
        }
    }

    var singular: String {
        switch self {
        case .lots: "lot"
        case .products: "product"
        case .sites: "site"
        case .stations: "station"
        case .suppliers: "supplier"
        case .gauges: "gauge"
        case .defectCodes: "defect code"
        case .checklists: "checklist"
        case .webhooks: "webhook"
        case .users: "user"
        case .plans: "plan"
        case .nonconformances: "alert"
        case .correctiveActions: "corrective action"
        }
    }

    var symbol: String {
        switch self {
        case .lots: "shippingbox.fill"
        case .products: "cube.fill"
        case .sites: "building.2.fill"
        case .stations: "wrench.adjustable.fill"
        case .suppliers: "truck.box.fill"
        case .gauges: "ruler.fill"
        case .defectCodes: "tag.fill"
        case .checklists: "checklist.checked"
        case .webhooks: "antenna.radiowaves.left.and.right"
        case .users: "person.2.fill"
        case .plans: "list.bullet.rectangle.fill"
        case .nonconformances: "exclamationmark.triangle.fill"
        case .correctiveActions: "wrench.and.screwdriver.fill"
        }
    }

    var titleKey: String {
        switch self {
        case .lots: "lot_number"
        case .products: "part_number"
        case .webhooks, .plans: "name"
        case .users: "full_name"
        case .nonconformances: "ncr_number"
        case .correctiveActions: "ca_number"
        default: "code"
        }
    }

    var subtitleKey: String {
        switch self {
        case .webhooks: "url"
        case .users: "username"
        case .lots: "notes"
        case .nonconformances, .correctiveActions: "title"
        case .plans: "revision"
        default: "name"
        }
    }

    /// Lowest role that may save changes; the server enforces the same rule.
    var editRole: Role {
        switch self {
        case .lots: .operator
        case .nonconformances: .inspector
        case .sites, .stations, .webhooks, .correctiveActions: .supervisor
        case .users: .admin
        default: .qualityManager
        }
    }

    /// Role that may create one here, or nil when another screen creates it.
    var createRole: Role? {
        switch self {
        case .lots, .plans, .nonconformances, .correctiveActions: nil
        default: editRole
        }
    }

    var deletable: Bool { self != .plans }

    /// The type name the lookup endpoint uses for scanned codes.
    static func scanned(_ entity: String) -> RecordKind? {
        switch entity {
        case "gauge": .gauges
        case "product": .products
        case "station": .stations
        case "supplier": .suppliers
        case "defect_code": .defectCodes
        case "site": .sites
        default: nil
        }
    }

    var fields: [FieldSpec] {
        let severities = ["minor", "major", "critical"]
        switch self {
        case .lots:
            return [
                FieldSpec(key: "lot_number", label: "Lot number", required: true, scannable: true),
                FieldSpec(key: "quantity", label: "Quantity", kind: .number, required: true),
                FieldSpec(key: "unit", label: "Unit"),
                FieldSpec(key: "supplier_id", label: "Supplier", kind: .supplier),
                FieldSpec(key: "supplier_lot", label: "Supplier lot", scannable: true),
                FieldSpec(key: "notes", label: "Notes", kind: .long),
            ]
        case .products:
            return [
                FieldSpec(key: "part_number", label: "Part number", required: true, scannable: true),
                FieldSpec(key: "name", label: "Name", required: true),
                FieldSpec(key: "unit", label: "Unit"),
                FieldSpec(key: "description", label: "Description", kind: .long),
                FieldSpec(key: "active", label: "Active", kind: .toggle, onCreate: false),
            ]
        case .sites:
            return [
                FieldSpec(key: "code", label: "Code", required: true),
                FieldSpec(key: "name", label: "Name", required: true),
                FieldSpec(key: "address", label: "Address", kind: .long),
                FieldSpec(key: "active", label: "Active", kind: .toggle, onCreate: false),
            ]
        case .stations:
            return [
                FieldSpec(key: "site_id", label: "Site", kind: .site, onEdit: false, required: true),
                FieldSpec(key: "code", label: "Code", required: true, scannable: true),
                FieldSpec(key: "name", label: "Name", required: true),
                FieldSpec(key: "description", label: "Description", kind: .long),
                FieldSpec(key: "active", label: "Active", kind: .toggle, onCreate: false),
            ]
        case .suppliers:
            return [
                FieldSpec(key: "code", label: "Code", required: true),
                FieldSpec(key: "name", label: "Name", required: true),
                FieldSpec(key: "contact_name", label: "Contact"),
                FieldSpec(key: "email", label: "Email", kind: .email),
                FieldSpec(key: "phone", label: "Phone"),
                FieldSpec(key: "active", label: "Active", kind: .toggle, onCreate: false),
            ]
        case .gauges:
            return [
                FieldSpec(key: "code", label: "Code", required: true, scannable: true),
                FieldSpec(key: "name", label: "Name", required: true),
                FieldSpec(key: "serial_number", label: "Serial number", scannable: true),
                FieldSpec(key: "calibration_due", label: "Calibration due", kind: .date),
                FieldSpec(key: "active", label: "Active", kind: .toggle, onCreate: false),
            ]
        case .defectCodes:
            return [
                FieldSpec(key: "code", label: "Code", required: true),
                FieldSpec(key: "name", label: "Name", required: true),
                FieldSpec(key: "severity", label: "Severity", kind: .choice(severities), required: true),
                FieldSpec(key: "category", label: "Category"),
                FieldSpec(key: "active", label: "Active", kind: .toggle, onCreate: false),
            ]
        case .checklists:
            return [
                FieldSpec(key: "code", label: "Code", required: true),
                FieldSpec(key: "name", label: "Name", required: true),
                FieldSpec(key: "product_id", label: "Product", kind: .product),
                FieldSpec(key: "description", label: "Description", kind: .long),
                FieldSpec(key: "active", label: "Active", kind: .toggle, onCreate: false),
            ]
        case .webhooks:
            return [
                FieldSpec(key: "name", label: "Name", required: true),
                FieldSpec(key: "url", label: "URL", required: true),
                FieldSpec(key: "events", label: "Events, comma separated", kind: .events, required: true),
                FieldSpec(key: "secret", label: "Secret", kind: .secret),
                FieldSpec(key: "description", label: "Description", kind: .long),
                FieldSpec(key: "active", label: "Active", kind: .toggle, onCreate: false),
            ]
        case .users:
            return [
                FieldSpec(key: "username", label: "Username", onEdit: false, required: true),
                FieldSpec(key: "full_name", label: "Full name", required: true),
                FieldSpec(key: "email", label: "Email", kind: .email, required: true),
                FieldSpec(key: "password", label: "Password, 10 or more characters", kind: .secret, onEdit: false, required: true),
                FieldSpec(key: "role", label: "Role", kind: .choice(Role.allCases.map(\.rawValue)), required: true),
                FieldSpec(key: "active", label: "Active", kind: .toggle, onCreate: false),
            ]
        case .plans:
            return [
                FieldSpec(key: "name", label: "Name", required: true),
                FieldSpec(key: "sampling_type", label: "Sampling", kind: .choice(["hundred_percent", "fixed", "aql"])),
                FieldSpec(key: "sample_size", label: "Sample size (fixed count only)", kind: .whole),
                FieldSpec(key: "aql", label: "AQL level (AQL only)"),
                FieldSpec(key: "notes", label: "Notes", kind: .long),
            ]
        case .nonconformances:
            return [
                FieldSpec(key: "title", label: "Title", required: true),
                FieldSpec(key: "description", label: "Description", kind: .long),
                FieldSpec(key: "severity", label: "Severity", kind: .choice(severities)),
                FieldSpec(key: "quantity_affected", label: "Quantity affected", kind: .number),
                FieldSpec(key: "assigned_to", label: "Assigned to", kind: .user),
            ]
        case .correctiveActions:
            return [
                FieldSpec(key: "title", label: "Title", required: true),
                FieldSpec(key: "root_cause", label: "Root cause", kind: .long),
                FieldSpec(key: "action_plan", label: "Action plan", kind: .long),
                FieldSpec(key: "owner_id", label: "Owner", kind: .user),
                FieldSpec(key: "due_date", label: "Due date", kind: .date),
            ]
        }
    }
}

struct RecordRoute: Hashable {
    var kind: RecordKind
    var id: UUID
}

/// The text the form shows for one JSON value.
func jsonText(_ value: Any?) -> String {
    switch value {
    case let s as String:
        return s
    case let n as NSNumber:
        if CFGetTypeID(n) == CFBooleanGetTypeID() { return n.boolValue ? "true" : "false" }
        return plainNumber(n.doubleValue)
    case let items as [Any]:
        return items.compactMap { $0 as? String }.joined(separator: ", ")
    default:
        return ""
    }
}

enum RecordForm {
    static func values(for kind: RecordKind, row: JSONObject?, editing: Bool) -> [String: String] {
        var out: [String: String] = [:]
        for field in kind.fields where editing ? field.onEdit : field.onCreate {
            guard let row else {
                out[field.key] = defaultValue(field)
                continue
            }
            switch field.kind {
            case .secret: out[field.key] = ""
            case .date: out[field.key] = String(jsonText(row[field.key]).prefix(10))
            default: out[field.key] = jsonText(row[field.key])
            }
        }
        return out
    }

    private static func defaultValue(_ field: FieldSpec) -> String {
        switch field.key {
        case "unit": "ea"
        case "severity": "major"
        case "role": "inspector"
        default: ""
        }
    }

    /// The request body: every filled field on create, only changed fields on edit.
    static func body(for kind: RecordKind, values: [String: String], original: [String: String]?) throws -> JSONObject {
        var body: JSONObject = [:]
        let editing = original != nil
        for field in kind.fields where editing ? field.onEdit : field.onCreate {
            let raw = values[field.key] ?? ""
            if let original, original[field.key] == raw { continue }
            let text = raw.trimmingCharacters(in: .whitespacesAndNewlines)
            if text.isEmpty, field.required, field.kind != .toggle {
                throw APIFailure(status: 0, message: "\(field.label) can't be empty.")
            }
            switch field.kind {
            case .toggle:
                body[field.key] = text == "true"
            case .number:
                if text.isEmpty { continue }
                guard let n = parseNumber(text) else { throw APIFailure(status: 0, message: "\(field.label) must be a number.") }
                body[field.key] = n
            case .whole:
                if text.isEmpty {
                    if editing { body["clear_sample_size"] = true }
                    continue
                }
                guard let n = Int(text), n > 0 else { throw APIFailure(status: 0, message: "\(field.label) must be a whole number above zero.") }
                body[field.key] = n
            case .events:
                body[field.key] = text.split(separator: ",").map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
            case .secret, .product, .supplier, .site, .user:
                if text.isEmpty { continue }
                body[field.key] = text
            default:
                if text.isEmpty && !editing { continue }
                body[field.key] = text
            }
        }
        if kind == .plans, let sampling = values["sampling_type"], sampling != "fixed" {
            body.removeValue(forKey: "sample_size")
            let hadSize = !(original?["sample_size"] ?? "").isEmpty
            if hadSize { body["clear_sample_size"] = true } else { body.removeValue(forKey: "clear_sample_size") }
        }
        return body
    }
}

// MARK: - List

struct RecordListView: View {
    @Environment(AppModel.self) private var model
    var kind: RecordKind
    @State private var rows: [JSONObject]?
    @State private var errorMessage: String?
    @State private var search = ""
    @State private var creating = false
    @State private var showInactive = true

    private var filtered: [JSONObject] {
        let all = rows ?? []
        let visible = showInactive ? all : all.filter { jsonText($0["active"]) != "false" }
        let q = search.trimmingCharacters(in: .whitespaces).lowercased()
        guard !q.isEmpty else { return visible }
        return visible.filter {
            jsonText($0[kind.titleKey]).lowercased().contains(q) || jsonText($0[kind.subtitleKey]).lowercased().contains(q)
        }
    }

    var body: some View {
        Group {
            if rows != nil {
                List {
                    ForEach(Array(filtered.enumerated()), id: \.offset) { _, row in
                        if let id = UUID(uuidString: jsonText(row["id"])) {
                            NavigationLink(value: RecordRoute(kind: kind, id: id)) {
                                RecordRow(kind: kind, row: row)
                            }
                        }
                    }
                }
                .qcListChrome()
                .overlay {
                    if filtered.isEmpty {
                        EmptyStateView(
                            title: search.isEmpty ? "No \(kind.title.lowercased()) yet" : "Nothing matches",
                            systemImage: kind.symbol,
                            description: search.isEmpty ? "Tap + to add the first one." : "Try another word or scan the label."
                        )
                    }
                }
            } else if errorMessage == nil {
                LoadingStateView(message: "Loading \(kind.title.lowercased())")
            } else {
                ErrorStateView(title: "Could not load \(kind.title.lowercased())", message: errorMessage ?? "", retry: { Task { await reload() } })
            }
        }
        .navigationTitle(kind.title)
        .searchable(text: $search, prompt: "Search \(kind.title.lowercased())")
        .navigationDestination(for: RecordRoute.self) { RecordEditorView(kind: $0.kind, id: $0.id) }
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Menu {
                    Toggle("Show turned off", isOn: $showInactive)
                } label: {
                    Label("Filter", systemImage: "line.3.horizontal.decrease.circle")
                }
            }
            if let role = kind.createRole, model.user?.allows(role) == true {
                ToolbarItem(placement: .primaryAction) {
                    Button("New \(kind.singular)", systemImage: "plus") { creating = true }
                }
            }
        }
        .refreshable { await reload() }
        .task { await reload() }
        .sheet(isPresented: $creating, onDismiss: { Task { await reload() } }) {
            NavigationStack { RecordEditorView(kind: kind, id: nil) }
        }
    }

    private func reload() async {
        do {
            rows = try await model.client.json(kind.path, query: ["limit": "200"]) as? [JSONObject] ?? []
            errorMessage = nil
        } catch {
            if rows == nil { errorMessage = error.localizedDescription }
        }
    }
}

private struct RecordRow: View {
    var kind: RecordKind
    var row: JSONObject

    var body: some View {
        HStack(spacing: 12) {
            Image(systemName: kind.symbol)
                .font(.body.weight(.semibold))
                .foregroundStyle(QCTheme.brandColor)
                .frame(width: 34, height: 34)
                .background(QCTheme.brandColor.opacity(0.12), in: RoundedRectangle(cornerRadius: 9, style: .continuous))
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 2) {
                Text(jsonText(row[kind.titleKey]))
                    .font(QCTheme.headlineFont)
                    .foregroundStyle(QCTheme.ink)
                let sub = subtitle
                if !sub.isEmpty {
                    Text(sub)
                        .font(.subheadline)
                        .foregroundStyle(QCTheme.mutedInk)
                        .lineLimit(1)
                }
            }
            Spacer(minLength: 0)
            if jsonText(row["active"]) == "false" {
                StatusBadge(text: "inactive")
            } else if let status = row["status"] as? String {
                StatusBadge(text: status)
            } else if let role = row["role"] as? String {
                Text(Role.named(role)?.title ?? role)
                    .font(QCTheme.captionFont)
                    .foregroundStyle(QCTheme.mutedInk)
            }
        }
        .padding(.vertical, 2)
        .accessibilityElement(children: .combine)
    }

    private var subtitle: String {
        var parts = [jsonText(row[kind.subtitleKey])]
        if kind == .gauges, let due = row["calibration_due"] as? String, !due.isEmpty {
            parts.append("due \(due.prefix(10))")
        }
        if kind == .defectCodes { parts.append(displayName(jsonText(row["severity"]))) }
        return parts.filter { !$0.isEmpty }.joined(separator: " · ")
    }
}

// MARK: - Editor

struct RecordEditorView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    var kind: RecordKind
    /// nil creates a new record.
    var id: UUID?

    @State private var row: JSONObject?
    @State private var values: [String: String] = [:]
    @State private var original: [String: String] = [:]
    @State private var loaded = false
    @State private var saving = false
    @State private var errorMessage: String?
    @State private var savedNote: String?
    @State private var confirmDelete = false
    @State private var newPassword = ""
    @State private var choices = PickerChoices()

    private var editing: Bool { id != nil }
    private var mayEdit: Bool { model.user?.allows(editing ? kind.editRole : (kind.createRole ?? .admin)) == true }
    private var changed: Bool { editing ? values != original : true }
    private var visibleFields: [FieldSpec] { kind.fields.filter { editing ? $0.onEdit : $0.onCreate } }

    var body: some View {
        Form {
            if editing && !loaded {
                Section { HStack { Spacer(); ProgressView(); Spacer() } }
            } else {
                if editing, let row {
                    RecordHeader(kind: kind, row: row)
                }
                Section {
                    ForEach(visibleFields) { field in
                        FieldEditor(field: field, value: binding(field.key), choices: choices)
                    }
                } header: {
                    Text(editing ? "Details" : "New \(kind.singular)")
                } footer: {
                    if !mayEdit {
                        Text("Your role can view this but not change it.")
                    } else if kind == .plans, let status = row?["status"] as? String, status != "draft" {
                        Text(model.user?.allows(.admin) == true
                             ? "This plan is \(displayName(status)). As admin you can still fix it; changes are written to the audit log."
                             : "Only a draft plan can be edited. Make a new revision instead.")
                    }
                }
                .disabled(!mayEdit)

                if let savedNote {
                    Section { Label(savedNote, systemImage: "checkmark.circle.fill").foregroundStyle(QCTheme.success) }
                }

                if editing, let row, let id {
                    extras(row: row, id: id)
                }
            }
        }
        .qcListChrome()
        .navigationTitle(editing ? (row.map { jsonText($0[kind.titleKey]) } ?? kind.title) : "New \(kind.singular)")
        #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
        #endif
        .toolbar {
            if !editing {
                ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
            }
            if mayEdit {
                ToolbarItem(placement: .confirmationAction) {
                    Button(editing ? "Save" : "Create") { Task { await save() } }
                        .fontWeight(.semibold)
                        .disabled(saving || !changed)
                }
            }
        }
        .task { await load() }
        .errorAlert($errorMessage)
        .sensoryFeedback(.success, trigger: savedNote)
    }

    @ViewBuilder
    private func extras(row: JSONObject, id: UUID) -> some View {
        switch kind {
        case .gauges, .products, .stations:
            LabelSection(code: jsonText(row[kind.titleKey]), title: jsonText(row["name"]), subtitle: kind == .gauges ? jsonText(row["serial_number"]) : kind.singular.capitalized)
        case .checklists:
            ChecklistItemsSection(checklistID: id, canEdit: model.user?.allows(.qualityManager) == true)
        default:
            EmptyView()
        }
        if kind == .users, model.user?.allows(.admin) == true {
            Section {
                SecureField("New password", text: $newPassword)
                Button("Set password") { Task { await setPassword(id) } }
                    .disabled(newPassword.count < 10 || saving)
            } header: {
                Text("Password")
            } footer: {
                Text("At least 10 characters. Their other sessions are signed out.")
            }
        }
        if kind.deletable, model.user?.allows(.admin) == true {
            Section {
                Button("Delete \(kind.singular)", role: .destructive) { confirmDelete = true }
                    .confirmationDialog("Delete this \(kind.singular) for good?", isPresented: $confirmDelete, titleVisibility: .visible) {
                        Button("Delete", role: .destructive) { Task { await delete(id) } }
                        Button("Keep it", role: .cancel) {}
                    } message: {
                        Text("If anything still uses it, the server refuses and says what. Turning it off keeps the history instead.")
                    }
            } footer: {
                Text("Admin only.")
            }
        }
    }

    private func binding(_ key: String) -> Binding<String> {
        Binding(get: { values[key, default: ""] }, set: { values[key] = $0 })
    }

    private func load() async {
        var options = PickerChoices()
        await options.load(for: kind, client: model.client)
        choices = options
        guard let id else {
            values = RecordForm.values(for: kind, row: nil, editing: false)
            loaded = true
            return
        }
        do {
            let object = try await model.client.json("\(kind.path)/\(id.uuidString)") as? JSONObject
            apply(object)
        } catch {
            errorMessage = error.localizedDescription
        }
        loaded = true
    }

    private func apply(_ object: JSONObject?) {
        // Plans and checklists come back with their children; the header is the same object.
        let header = (object?["plan"] as? JSONObject) ?? object
        row = header
        values = RecordForm.values(for: kind, row: header, editing: true)
        original = values
    }

    private func save() async {
        saving = true
        defer { saving = false }
        savedNote = nil
        do {
            let body = try RecordForm.body(for: kind, values: values, original: editing ? original : nil)
            if let id {
                let updated = try await model.client.json(method: "PATCH", "\(kind.path)/\(id.uuidString)", body: body) as? JSONObject
                apply(updated)
                savedNote = "Saved."
            } else {
                _ = try await model.client.json(method: "POST", kind.path, body: body)
                dismiss()
            }
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func setPassword(_ id: UUID) async {
        saving = true
        defer { saving = false }
        do {
            try await model.client.execute(path: "/api/v1/users/\(id.uuidString)/password", body: ["password": newPassword])
            newPassword = ""
            savedNote = "Password set."
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func delete(_ id: UUID) async {
        do {
            try await model.client.delete("\(kind.path)/\(id.uuidString)")
            dismiss()
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}

private struct RecordHeader: View {
    var kind: RecordKind
    var row: JSONObject

    var body: some View {
        Section {
            HStack(spacing: 14) {
                Image(systemName: kind.symbol)
                    .font(.title2.weight(.semibold))
                    .foregroundStyle(.white)
                    .frame(width: 52, height: 52)
                    .background(QCTheme.brandGradient, in: RoundedRectangle(cornerRadius: 14, style: .continuous))
                VStack(alignment: .leading, spacing: 4) {
                    Text(jsonText(row[kind.titleKey]))
                        .font(QCTheme.titleFont)
                        .foregroundStyle(QCTheme.ink)
                    Text(jsonText(row[kind.subtitleKey]))
                        .font(.subheadline)
                        .foregroundStyle(QCTheme.mutedInk)
                        .lineLimit(2)
                }
                Spacer(minLength: 0)
                if let status = row["status"] as? String {
                    StatusBadge(text: status)
                } else if jsonText(row["active"]) == "false" {
                    StatusBadge(text: "inactive")
                }
            }
            .padding(.vertical, 4)
            .accessibilityElement(children: .combine)
        }
    }
}

/// Options for the pickers on a form, loaded once per screen.
struct PickerChoices {
    var products: [(String, String)] = []
    var suppliers: [(String, String)] = []
    var sites: [(String, String)] = []
    var users: [(String, String)] = []

    mutating func load(for kind: RecordKind, client: APIClient) async {
        let kinds = Set(kind.fields.map(\.kind))
        func options(_ path: String, _ label: (JSONObject) -> String) async -> [(String, String)] {
            let rows = (try? await client.json(path, query: ["limit": "200"]) as? [JSONObject]) ?? []
            return rows.map { (jsonText($0["id"]), label($0)) }
        }
        if kinds.contains(.product) {
            products = await options("/api/v1/products") { "\(jsonText($0["part_number"])) · \(jsonText($0["name"]))" }
        }
        if kinds.contains(.supplier) {
            suppliers = await options("/api/v1/suppliers") { "\(jsonText($0["code"])) · \(jsonText($0["name"]))" }
        }
        if kinds.contains(.site) {
            sites = await options("/api/v1/sites") { "\(jsonText($0["code"])) · \(jsonText($0["name"]))" }
        }
        if kinds.contains(.user) {
            users = await options("/api/v1/users") { jsonText($0["full_name"]) }
        }
    }
}

extension FieldKind: Hashable {
    func hash(into hasher: inout Hasher) {
        switch self {
        case .choice: hasher.combine("choice")
        default: hasher.combine(String(describing: self))
        }
    }
}

private struct FieldEditor: View {
    var field: FieldSpec
    @Binding var value: String
    var choices: PickerChoices

    var body: some View {
        switch field.kind {
        case .toggle:
            Toggle(field.label, isOn: Binding(get: { value == "true" }, set: { value = $0 ? "true" : "false" }))
        case .long:
            TextField(field.label, text: $value, axis: .vertical)
                .lineLimit(2...6)
        case .number:
            TextField(field.label, text: $value)
                .keyboardType(.decimalPad)
        case .whole:
            TextField(field.label, text: $value)
                .keyboardType(.numberPad)
        case .secret:
            SecureField(field.label + (field.onCreate && !field.onEdit ? "" : " (blank keeps it)"), text: $value)
        case .email:
            TextField(field.label, text: $value)
                .keyboardType(.emailAddress)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
        case .date:
            DateField(label: field.label, value: $value)
        case .events:
            TextField(field.label, text: $value, axis: .vertical)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
        case .choice(let options):
            Picker(field.label, selection: $value) {
                if !options.contains(value) { Text("Choose").tag(value) }
                ForEach(options, id: \.self) { option in
                    Text(Role.named(option)?.title ?? displayName(option).capitalized).tag(option)
                }
            }
        case .product:
            idPicker(choices.products, none: "Any product")
        case .supplier:
            idPicker(choices.suppliers, none: "No supplier")
        case .site:
            idPicker(choices.sites, none: "Choose a site")
        case .user:
            idPicker(choices.users, none: "Nobody")
        case .text:
            HStack {
                TextField(field.label, text: $value)
                    .textInputAutocapitalization(field.key.contains("name") || field.key == "title" ? .sentences : .characters)
                    .autocorrectionDisabled()
                if field.scannable {
                    ScanFieldButton(text: $value)
                }
            }
        }
    }

    private func idPicker(_ options: [(String, String)], none: String) -> some View {
        Picker(field.label, selection: $value) {
            Text(none).tag("")
            if !value.isEmpty && !options.contains(where: { $0.0 == value }) {
                Text("Current").tag(value)
            }
            ForEach(options, id: \.0) { option in
                Text(option.1).tag(option.0)
            }
        }
    }
}

/// A date that may be blank, stored as YYYY-MM-DD.
private struct DateField: View {
    var label: String
    @Binding var value: String

    private static let format: DateFormatter = {
        let f = DateFormatter()
        f.calendar = Calendar(identifier: .gregorian)
        f.locale = Locale(identifier: "en_US_POSIX")
        f.dateFormat = "yyyy-MM-dd"
        return f
    }()

    var body: some View {
        if let date = Self.format.date(from: value) {
            HStack {
                DatePicker(label, selection: Binding(get: { date }, set: { value = Self.format.string(from: $0) }), displayedComponents: .date)
                Button("Clear", systemImage: "xmark.circle.fill") { value = "" }
                    .labelStyle(.iconOnly)
                    .foregroundStyle(QCTheme.mutedInk)
                    .buttonStyle(.plain)
            }
        } else {
            Button {
                value = Self.format.string(from: Calendar.current.date(byAdding: .year, value: 1, to: .now) ?? .now)
            } label: {
                LabeledContent(label, value: "Not set")
            }
        }
    }
}

// MARK: - Toolbar menu for detail screens

/// Adds "Edit details" and, for admins, "Delete" to a detail screen's toolbar.
private struct RecordMenu: ViewModifier {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    var path: String
    var noun: String
    var editKind: RecordKind?
    var onChanged: () async -> Void
    @State private var editing = false
    @State private var confirmDelete = false
    @State private var errorMessage: String?

    private var canEdit: Bool { editKind.map { model.user?.allows($0.editRole) == true } ?? false }
    private var canDelete: Bool { model.user?.allows(.admin) == true }

    func body(content: Content) -> some View {
        content
            .toolbar {
                if canEdit || canDelete {
                    ToolbarItem(placement: .primaryAction) {
                        Menu {
                            if canEdit {
                                Button("Edit details", systemImage: "pencil") { editing = true }
                            }
                            if canDelete {
                                Button("Delete \(noun)", systemImage: "trash", role: .destructive) { confirmDelete = true }
                            }
                        } label: {
                            Label("More", systemImage: "ellipsis.circle")
                        }
                    }
                }
            }
            .confirmationDialog("Delete this \(noun)?", isPresented: $confirmDelete, titleVisibility: .visible) {
                Button("Delete", role: .destructive) { Task { await delete() } }
                Button("Keep it", role: .cancel) {}
            } message: {
                Text("This removes it and everything recorded on it. The server refuses if other records still point at it. Admin only.")
            }
            .sheet(isPresented: $editing, onDismiss: { Task { await onChanged() } }) {
                if let editKind, let id = UUID(uuidString: String(path.split(separator: "/").last ?? "")) {
                    NavigationStack {
                        RecordEditorView(kind: editKind, id: id)
                            .toolbar { ToolbarItem(placement: .cancellationAction) { Button("Done") { editing = false } } }
                    }
                }
            }
            .errorAlert($errorMessage)
    }

    private func delete() async {
        do {
            try await model.client.delete(path)
            dismiss()
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}

extension View {
    func recordMenu(path: String, noun: String, editKind: RecordKind? = nil, onChanged: @escaping () async -> Void = {}) -> some View {
        modifier(RecordMenu(path: path, noun: noun, editKind: editKind, onChanged: onChanged))
    }
}

// MARK: - Checklist steps

private struct ChecklistItemsSection: View {
    @Environment(AppModel.self) private var model
    var checklistID: UUID
    var canEdit: Bool
    @State private var items: [JSONObject] = []
    @State private var prompt = ""
    @State private var itemType = "pass_fail"
    @State private var required = true
    @State private var errorMessage: String?

    private let types = [("pass_fail", "Pass or fail"), ("yes_no", "Yes or no"), ("number", "Number"), ("text", "Note")]

    var body: some View {
        Section("Steps") {
            if items.isEmpty {
                Text("No steps yet.").foregroundStyle(QCTheme.mutedInk)
            }
            ForEach(Array(items.enumerated()), id: \.offset) { _, item in
                VStack(alignment: .leading, spacing: 2) {
                    Text("\(jsonText(item["sequence"])). \(jsonText(item["prompt"]))")
                    Text(typeName(jsonText(item["item_type"])) + (jsonText(item["required"]) == "true" ? " · required" : ""))
                        .font(.caption)
                        .foregroundStyle(QCTheme.mutedInk)
                }
                .swipeActions {
                    if canEdit {
                        Button("Remove", role: .destructive) { Task { await remove(item) } }
                    }
                }
            }
            if let errorMessage {
                Text(errorMessage).font(.caption).foregroundStyle(QCTheme.danger)
            }
        }
        .task { await reload() }
        if canEdit {
            Section("Add a step") {
                TextField("What to check", text: $prompt, axis: .vertical)
                Picker("Answer", selection: $itemType) {
                    ForEach(types, id: \.0) { Text($0.1).tag($0.0) }
                }
                Toggle("Required", isOn: $required)
                Button("Add step") { Task { await add() } }
                    .disabled(prompt.trimmingCharacters(in: .whitespaces).isEmpty)
            }
        }
    }

    private func typeName(_ raw: String) -> String {
        types.first { $0.0 == raw }?.1 ?? displayName(raw)
    }

    private func reload() async {
        let detail = try? await model.client.json("/api/v1/checklists/\(checklistID.uuidString)") as? JSONObject
        items = detail?["items"] as? [JSONObject] ?? []
    }

    private func add() async {
        let next = (items.compactMap { ($0["sequence"] as? NSNumber)?.intValue }.max() ?? 0) + 1
        do {
            _ = try await model.client.json(method: "POST", "/api/v1/checklists/\(checklistID.uuidString)/items", body: [
                "sequence": next, "prompt": prompt.trimmingCharacters(in: .whitespaces), "item_type": itemType, "required": required,
            ])
            prompt = ""
            errorMessage = nil
            await reload()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func remove(_ item: JSONObject) async {
        do {
            try await model.client.delete("/api/v1/checklist-items/\(jsonText(item["id"]))")
            await reload()
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}
