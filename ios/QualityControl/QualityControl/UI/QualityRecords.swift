import SwiftUI

struct NonconformancesView: View {
    @Environment(AppModel.self) private var model
    @State private var data = RemoteData<[Nonconformance]>()
    @State private var creating = false

    var body: some View {
        Group {
            if let rows = data.value {
                List(rows) { ncr in
                    NavigationLink(value: ncr.id) {
                        VStack(alignment: .leading, spacing: 4) {
                            Text(ncr.ncrNumber).font(.headline)
                            Text(ncr.title).font(.subheadline)
                        }
                        StatusBadge(text: ncr.status)
                    }
                }
                .overlay { if rows.isEmpty { ContentUnavailableView("No nonconformances", systemImage: "checkmark.seal") } }
            } else if data.loading {
                ProgressView("Loading nonconformances")
            } else {
                ContentUnavailableView("Could not load nonconformances", systemImage: "wifi.exclamationmark", description: Text(data.error ?? "Pull to refresh."))
            }
        }
        .navigationTitle("Nonconformances")
        .navigationDestination(for: UUID.self) { NCRDetailView(id: $0) }
        .toolbar {
            if model.user?.allows(.inspector) == true {
                ToolbarItem(placement: .primaryAction) {
                    Button("New NCR", systemImage: "plus") { creating = true }
                }
            }
        }
        .refreshable { await reload() }
        .task { await reload() }
        .sheet(isPresented: $creating, onDismiss: { Task { await reload() } }) {
            NavigationStack { NCRForm() }
        }
        .errorAlert($data.error)
    }

    private func reload() async {
        await data.load { try await model.client.list("/api/v1/nonconformances") }
    }
}

struct NCRForm: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @State private var title = ""
    @State private var description = ""
    @State private var severity = "major"
    @State private var quantity = "0"
    @State private var productId: UUID?
    @State private var products: [Product] = []
    @State private var errorMessage: String?
    @State private var saving = false

    var body: some View {
        Form {
            TextField("Title", text: $title)
            TextField("Description", text: $description, axis: .vertical)
            Picker("Product", selection: $productId) {
                Text("Select").tag(Optional<UUID>.none)
                ForEach(products) { Text("\($0.partNumber) · \($0.name)").tag(Optional($0.id)) }
            }
            Picker("Severity", selection: $severity) {
                ForEach(model.enums.severities, id: \.self) { Text(displayName($0)).tag($0) }
            }
            TextField("Quantity affected", text: $quantity).keyboardType(.decimalPad)
        }
        .navigationTitle("New nonconformance")
        .toolbar {
            ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
            ToolbarItem(placement: .confirmationAction) {
                Button("Save") { Task { await save() } }
                    .disabled(saving || title.isEmpty || description.isEmpty || productId == nil)
            }
        }
        .task { products = (try? await model.client.list("/api/v1/products", query: Query.compact(["active": "true"]))) ?? [] }
        .errorAlert($errorMessage)
    }

    private func save() async {
        guard let productId, let quantity = parseNumber(quantity) else { return }
        saving = true
        defer { saving = false }
        do {
            let _: Nonconformance = try await model.client.create("/api/v1/nonconformances", body: [
                "title": title,
                "description": description,
                "severity": severity,
                "product_id": productId.uuidString,
                "quantity_affected": quantity
            ])
            dismiss()
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}

struct NCRDetailView: View {
    @Environment(AppModel.self) private var model
    var id: UUID
    @State private var detail: NCRDetail?
    @State private var disposition = "scrap"
    @State private var notes = ""
    @State private var errorMessage: String?
    @State private var confirmVoid = false

    var body: some View {
        Group {
            if let detail {
                Form {
                    LabeledContent("Number", value: detail.ncr.ncrNumber)
                    Text(detail.ncr.title).font(.headline)
                    Text(detail.ncr.description)
                    LabeledContent("Severity", value: displayName(detail.ncr.severity))
                    LabeledContent("Quantity", value: formatNumber(detail.ncr.quantityAffected))
                    StatusBadge(text: detail.ncr.status)
                    StatusBadge(text: detail.ncr.disposition)
                    if !detail.ncr.dispositionNotes.isEmpty { Text(detail.ncr.dispositionNotes) }
                    if model.user?.allows(.supervisor) == true, detail.ncr.status == "open" || detail.ncr.status == "under_review" {
                        Section("Disposition") {
                            Picker("Decision", selection: $disposition) {
                                ForEach(model.enums.dispositions.filter { $0 != "pending" }, id: \.self) { Text(displayName($0)).tag($0) }
                            }
                            TextField("Notes", text: $notes, axis: .vertical)
                            Button("Record disposition") { Task { await dispose() } }
                            Button("Void", role: .destructive) { confirmVoid = true }
                            .confirmationDialog("Void this nonconformance?", isPresented: $confirmVoid, titleVisibility: .visible) {
                                Button("Void", role: .destructive) { Task { await voidNCR() } }
                                Button("Keep it", role: .cancel) {}
                            } message: {
                                Text("A voided nonconformance stays in the record and cannot be dispositioned.")
                            }
                        }
                    }
                    if model.user?.allows(.supervisor) == true, detail.ncr.status == "dispositioned" {
                        Button("Close nonconformance") { Task { await close() } }
                    }
                    if model.user?.allows(.supervisor) == true {
                        NavigationLink("Open corrective action") {
                            CAPAForm(nonconformanceId: detail.ncr.id)
                        }
                    }
                    Section("Corrective actions") {
                        if detail.correctiveActions.isEmpty {
                            Text("None yet").foregroundStyle(.secondary)
                        }
                        ForEach(detail.correctiveActions) { action in
                            VStack(alignment: .leading) {
                                Text(action.caNumber).font(.headline)
                                Text(action.title)
                                StatusBadge(text: action.status)
                            }
                        }
                    }
                    AttachmentsSection(entityType: "nonconformance", entityId: detail.ncr.id, canUpload: model.user?.allows(.operator) == true, canDelete: model.user?.allows(.supervisor) == true)
                }
            } else {
                ProgressView()
            }
        }
        .navigationTitle(detail?.ncr.ncrNumber ?? "Nonconformance")
        .task { await reload() }
        .errorAlert($errorMessage)
    }

    private func reload() async {
        do { detail = try await model.client.one("/api/v1/nonconformances/\(id.uuidString)") }
        catch { errorMessage = error.localizedDescription }
    }

    private func dispose() async {
        do {
            let updated: Nonconformance = try await model.client.action("/api/v1/nonconformances/\(id.uuidString)/disposition", body: ["disposition": disposition, "notes": notes])
            detail?.ncr = updated
        } catch { errorMessage = error.localizedDescription }
    }

    private func close() async {
        do {
            let updated: Nonconformance = try await model.client.action("/api/v1/nonconformances/\(id.uuidString)/close")
            detail?.ncr = updated
        } catch { errorMessage = error.localizedDescription }
    }

    private func voidNCR() async {
        do {
            let updated: Nonconformance = try await model.client.action("/api/v1/nonconformances/\(id.uuidString)/void", body: ["notes": notes])
            detail?.ncr = updated
        } catch { errorMessage = error.localizedDescription }
    }
}

struct CorrectiveActionsView: View {
    @Environment(AppModel.self) private var model
    @State private var data = RemoteData<[CorrectiveAction]>()
    @State private var creating = false

    var body: some View {
        Group {
            if let rows = data.value {
                List(rows) { action in
                    NavigationLink(value: action.id) {
                        VStack(alignment: .leading) {
                            Text(action.caNumber).font(.headline)
                            Text(action.title).font(.subheadline)
                        }
                        StatusBadge(text: action.status)
                    }
                }
                .overlay { if rows.isEmpty { ContentUnavailableView("No corrective actions", systemImage: "wrench.and.screwdriver") } }
            } else if data.loading {
                ProgressView("Loading actions")
            } else {
                ContentUnavailableView("Could not load corrective actions", systemImage: "wifi.exclamationmark", description: Text(data.error ?? "Pull to refresh."))
            }
        }
        .navigationTitle("Corrective actions")
        .navigationDestination(for: UUID.self) { CAPADetailView(id: $0) }
        .toolbar {
            if model.user?.allows(.supervisor) == true {
                ToolbarItem(placement: .primaryAction) {
                    Button("New action", systemImage: "plus") { creating = true }
                }
            }
        }
        .refreshable { await reload() }
        .task { await reload() }
        .sheet(isPresented: $creating, onDismiss: { Task { await reload() } }) {
            NavigationStack { CAPAForm(nonconformanceId: nil) }
        }
        .errorAlert($data.error)
    }

    private func reload() async {
        await data.load { try await model.client.list("/api/v1/corrective-actions") }
    }
}

struct CAPAForm: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    var nonconformanceId: UUID?
    @State private var title = ""
    @State private var rootCause = ""
    @State private var actionPlan = ""
    @State private var dueDate = ""
    @State private var errorMessage: String?
    @State private var saving = false

    var body: some View {
        Form {
            TextField("Title", text: $title)
            TextField("Root cause", text: $rootCause, axis: .vertical)
            TextField("Action plan", text: $actionPlan, axis: .vertical)
            TextField("Due date YYYY-MM-DD", text: $dueDate)
        }
        .navigationTitle("New action")
        .toolbar {
            ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
            ToolbarItem(placement: .confirmationAction) {
                Button("Save") { Task { await save() } }.disabled(saving || title.isEmpty)
            }
        }
        .errorAlert($errorMessage)
    }

    private func save() async {
        saving = true
        defer { saving = false }
        var body: [String: Any] = ["title": title, "root_cause": rootCause, "action_plan": actionPlan, "due_date": dueDate]
        if let nonconformanceId { body["nonconformance_id"] = nonconformanceId.uuidString }
        do {
            let _: CorrectiveAction = try await model.client.create("/api/v1/corrective-actions", body: body)
            dismiss()
        } catch { errorMessage = error.localizedDescription }
    }
}

struct CAPADetailView: View {
    @Environment(AppModel.self) private var model
    var id: UUID
    @State private var action: CorrectiveAction?
    @State private var notes = ""
    @State private var errorMessage: String?

    var body: some View {
        Group {
            if let action {
                Form {
                    LabeledContent("Number", value: action.caNumber)
                    Text(action.title).font(.headline)
                    LabeledContent("Root cause", value: action.rootCause.isEmpty ? "—" : action.rootCause)
                    LabeledContent("Plan", value: action.actionPlan.isEmpty ? "—" : action.actionPlan)
                    LabeledContent("Due", value: action.dueDate.isEmpty ? "—" : action.dueDate)
                    StatusBadge(text: action.status)
                    if !action.verificationNotes.isEmpty { Text(action.verificationNotes) }
                    if !action.effectivenessNotes.isEmpty { Text(action.effectivenessNotes) }
                    if model.user?.allows(.supervisor) == true {
                        TextField("Notes", text: $notes, axis: .vertical)
                        if action.status == "open" {
                            Button("Start") { Task { await move("start") } }
                        }
                        if action.status == "in_progress" {
                            Button("Send to verification") { Task { await move("verify") } }
                            Button("Cancel", role: .destructive) { Task { await move("cancel") } }
                        }
                        if action.status == "verification" {
                            Button("Send back") { Task { await move("reopen") } }
                        }
                    }
                    if model.user?.allows(.qualityManager) == true, action.status == "verification" {
                        Button("Close action") { Task { await move("close") } }
                    }
                    AttachmentsSection(entityType: "corrective_action", entityId: action.id, canUpload: model.user?.allows(.operator) == true, canDelete: model.user?.allows(.supervisor) == true)
                }
            } else { ProgressView() }
        }
        .navigationTitle(action?.caNumber ?? "Action")
        .task { await reload() }
        .errorAlert($errorMessage)
    }

    private func reload() async {
        do { action = try await model.client.one("/api/v1/corrective-actions/\(id.uuidString)") }
        catch { errorMessage = error.localizedDescription }
    }

    private func move(_ step: String) async {
        do {
            action = try await model.client.action("/api/v1/corrective-actions/\(id.uuidString)/\(step)", body: ["notes": notes])
            notes = ""
        } catch { errorMessage = error.localizedDescription }
    }
}
