import SwiftUI

struct NonconformancesView: View {
    @Environment(AppModel.self) private var model
    @State private var data = RemoteData<[Nonconformance]>()
    @State private var creating = false
    @State private var search = ""
    @State private var status = ""
    @State private var severity = ""
    @State private var sort: ListSort = .newest

    private var filtered: [Nonconformance] {
        var rows = data.value ?? []
        if !search.isEmpty {
            let q = search.lowercased()
            rows = rows.filter {
                $0.ncrNumber.lowercased().contains(q)
                    || $0.title.lowercased().contains(q)
                    || $0.description.lowercased().contains(q)
            }
        }
        if !status.isEmpty { rows = rows.filter { $0.status == status } }
        if !severity.isEmpty { rows = rows.filter { $0.severity == severity } }
        switch sort {
        case .newest: return rows.sorted { $0.openedAt > $1.openedAt }
        case .oldest: return rows.sorted { $0.openedAt < $1.openedAt }
        case .nameAZ: return rows.sorted { $0.ncrNumber.localizedCaseInsensitiveCompare($1.ncrNumber) == .orderedAscending }
        case .nameZA: return rows.sorted { $0.ncrNumber.localizedCaseInsensitiveCompare($1.ncrNumber) == .orderedDescending }
        case .status: return rows.sorted { $0.status.localizedCaseInsensitiveCompare($1.status) == .orderedAscending }
        }
    }

    var body: some View {
        Group {
            if data.value != nil {
                List(filtered) { ncr in
                    NavigationLink(value: ncr.id) {
                        HStack(alignment: .top, spacing: 12) {
                            Image(systemName: "exclamationmark.triangle.fill")
                                .foregroundStyle(QCTheme.statusColor(ncr.severity))
                                .frame(width: 28)
                                .accessibilityHidden(true)
                            VStack(alignment: .leading, spacing: 4) {
                                Text(ncr.ncrNumber).font(QCTheme.headlineFont)
                                Text(ncr.title).font(.subheadline).foregroundStyle(QCTheme.mutedInk)
                                Text(displayName(ncr.severity))
                                    .font(.caption)
                                    .foregroundStyle(QCTheme.statusColor(ncr.severity))
                            }
                            Spacer(minLength: 0)
                            StatusBadge(text: ncr.status)
                        }
                    }
                    .accessibilityLabel("Nonconformance \(ncr.ncrNumber), \(ncr.title), \(displayName(ncr.status))")
                }
                .qcListChrome()
                .overlay {
                    if filtered.isEmpty {
                        EmptyStateView(title: "No nonconformances", systemImage: "checkmark.seal", description: "Nothing matches these filters.")
                    }
                }
            } else if data.loading {
                LoadingStateView(message: "Loading nonconformances")
            } else {
                ErrorStateView(
                    title: "Could not load nonconformances",
                    message: data.error ?? "Pull to refresh.",
                    retry: { Task { await reload() } }
                )
            }
        }
        .navigationTitle("Alerts")
        .navigationDestination(for: UUID.self) { NCRDetailView(id: $0) }
        .searchable(text: $search, prompt: "NCR number or title")
        .safeAreaBar(edge: .top) {
            VStack(spacing: 8) {
                chipRow(title: "Status", values: model.enums.ncrStatuses, selection: $status)
                chipRow(title: "Severity", values: model.enums.severities, selection: $severity)
            }
            .padding(.vertical, 8)
            .fixedSize(horizontal: false, vertical: true)
        }
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                SortMenu(selection: $sort, options: ListSort.allCases)
            }
            if model.user?.allows(.inspector) == true {
                ToolbarItem(placement: .primaryAction) {
                    Button("New NCR", systemImage: "plus") { creating = true }
                        .accessibilityLabel("Create nonconformance")
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

    private func chipRow(title: String, values: [String], selection: Binding<String>) -> some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 8) {
                FilterChip(title: "All \(title.lowercased())", selected: selection.wrappedValue.isEmpty) {
                    selection.wrappedValue = ""
                }
                ForEach(values, id: \.self) { item in
                    FilterChip(title: displayName(item), selected: selection.wrappedValue == item) {
                        selection.wrappedValue = item
                    }
                }
            }
            .padding(.horizontal)
        }
        .accessibilityLabel("\(title) filters")
    }

    private func reload() async {
        await data.load(
            cache: { model.offline.cached([Nonconformance].self, forKey: CacheKey.nonconformances) },
            remote: { try await model.client.list("/api/v1/nonconformances", query: Query.compact(["status": status])) },
            store: { model.offline.cache($0, forKey: CacheKey.nonconformances) }
        )
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
    @State private var attempted = false

    private var isValid: Bool {
        !title.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            && !description.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            && productId != nil
            && parseNumber(quantity) != nil
    }

    var body: some View {
        Form {
            TextField("Title", text: $title)
                .accessibilityLabel("Title")
            TextField("Description", text: $description, axis: .vertical)
                .accessibilityLabel("Description")
            Picker("Product", selection: $productId) {
                Text("Select").tag(Optional<UUID>.none)
                ForEach(products) { Text("\($0.partNumber) · \($0.name)").tag(Optional($0.id)) }
            }
            Picker("Severity", selection: $severity) {
                ForEach(model.enums.severities, id: \.self) { Text(displayName($0)).tag($0) }
            }
            TextField("Quantity affected", text: $quantity).keyboardType(.decimalPad)
            if attempted && !isValid {
                Text("Title, description, product, and quantity are required.")
                    .font(.caption)
                    .foregroundStyle(QCTheme.danger)
            }
        }
        .navigationTitle("New nonconformance")
        .toolbar {
            ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
            ToolbarItem(placement: .confirmationAction) {
                Button("Save") {
                    attempted = true
                    Task { await save() }
                }
                .disabled(saving)
            }
        }
        .task { products = (try? await model.client.list("/api/v1/products", query: Query.compact(["active": "true"]))) ?? [] }
        .errorAlert($errorMessage)
    }

    private func save() async {
        guard isValid, let productId, let quantity = parseNumber(quantity) else { return }
        saving = true
        defer { saving = false }
        let body: [String: Any] = [
            "title": title,
            "description": description,
            "severity": severity,
            "product_id": productId.uuidString,
            "quantity_affected": quantity
        ]
        do {
            if model.offline.isOnline {
                let _: Nonconformance = try await model.client.create("/api/v1/nonconformances", body: body)
            } else {
                model.offline.enqueue(method: "POST", path: "/api/v1/nonconformances", body: body)
            }
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
                    Section {
                        LabeledContent("Number", value: detail.ncr.ncrNumber)
                        Text(detail.ncr.title).font(QCTheme.headlineFont)
                        Text(detail.ncr.description)
                        LabeledContent("Severity", value: displayName(detail.ncr.severity))
                        LabeledContent("Quantity", value: formatNumber(detail.ncr.quantityAffected))
                        HStack {
                            StatusBadge(text: detail.ncr.status)
                            StatusBadge(text: detail.ncr.disposition)
                        }
                        if !detail.ncr.dispositionNotes.isEmpty { Text(detail.ncr.dispositionNotes) }
                    }
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
                            Text("None yet").foregroundStyle(QCTheme.mutedInk)
                        }
                        ForEach(detail.correctiveActions) { action in
                            VStack(alignment: .leading) {
                                Text(action.caNumber).font(QCTheme.headlineFont)
                                Text(action.title)
                                StatusBadge(text: action.status)
                            }
                        }
                    }
                    PhotoCaptureSection(
                        entityType: "nonconformance",
                        entityId: detail.ncr.id,
                        canCapture: model.user?.allows(.operator) == true
                    )
                    AttachmentsSection(entityType: "nonconformance", entityId: detail.ncr.id, canUpload: model.user?.allows(.operator) == true, canDelete: model.user?.allows(.supervisor) == true)
                }
                .qcListChrome()
            } else if errorMessage == nil {
                LoadingStateView()
            } else {
                ErrorStateView(title: "NCR unavailable", message: errorMessage ?? "", retry: { Task { await reload() } })
            }
        }
        .navigationTitle(detail?.ncr.ncrNumber ?? "Alert")
        .recordMenu(path: "/api/v1/nonconformances/\(id.uuidString)", noun: "alert", editKind: .nonconformances) { await reload() }
        .task { await reload() }
        .refreshable { await reload() }
        .errorAlert($errorMessage)
    }

    private func reload() async {
        do { detail = try await model.client.one("/api/v1/nonconformances/\(id.uuidString)"); errorMessage = nil }
        catch { if detail == nil { errorMessage = error.localizedDescription } }
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
    @State private var search = ""
    @State private var status = ""
    @State private var sort: ListSort = .newest

    private var filtered: [CorrectiveAction] {
        var rows = data.value ?? []
        if !search.isEmpty {
            let q = search.lowercased()
            rows = rows.filter {
                $0.caNumber.lowercased().contains(q)
                    || $0.title.lowercased().contains(q)
                    || $0.rootCause.lowercased().contains(q)
            }
        }
        if !status.isEmpty { rows = rows.filter { $0.status == status } }
        switch sort {
        case .newest: return rows.sorted { $0.openedAt > $1.openedAt }
        case .oldest: return rows.sorted { $0.openedAt < $1.openedAt }
        case .nameAZ: return rows.sorted { $0.caNumber.localizedCaseInsensitiveCompare($1.caNumber) == .orderedAscending }
        case .nameZA: return rows.sorted { $0.caNumber.localizedCaseInsensitiveCompare($1.caNumber) == .orderedDescending }
        case .status: return rows.sorted { $0.status.localizedCaseInsensitiveCompare($1.status) == .orderedAscending }
        }
    }

    var body: some View {
        Group {
            if data.value != nil {
                List(filtered) { action in
                    NavigationLink(value: action.id) {
                        HStack(alignment: .top, spacing: 12) {
                            Image(systemName: "wrench.and.screwdriver.fill")
                                .foregroundStyle(QCTheme.brandColor)
                                .frame(width: 28)
                                .accessibilityHidden(true)
                            VStack(alignment: .leading, spacing: 4) {
                                Text(action.caNumber).font(QCTheme.headlineFont)
                                Text(action.title).font(.subheadline).foregroundStyle(QCTheme.mutedInk)
                                if !action.dueDate.isEmpty {
                                    Text("Due \(action.dueDate)")
                                        .font(.caption)
                                        .foregroundStyle(QCTheme.mutedInk)
                                }
                            }
                            Spacer(minLength: 0)
                            StatusBadge(text: action.status)
                        }
                    }
                    .accessibilityLabel("Corrective action \(action.caNumber), \(action.title), \(displayName(action.status))")
                }
                .qcListChrome()
                .overlay {
                    if filtered.isEmpty {
                        EmptyStateView(title: "No corrective actions", systemImage: "wrench.and.screwdriver", description: "Nothing matches these filters.")
                    }
                }
            } else if data.loading {
                LoadingStateView(message: "Loading actions")
            } else {
                ErrorStateView(
                    title: "Could not load corrective actions",
                    message: data.error ?? "Pull to refresh.",
                    retry: { Task { await reload() } }
                )
            }
        }
        .navigationTitle("Corrective actions")
        .navigationDestination(for: UUID.self) { CAPADetailView(id: $0) }
        .searchable(text: $search, prompt: "CA number or title")
        .safeAreaBar(edge: .top) {
            VStack(spacing: 0) {
                ScrollView(.horizontal, showsIndicators: false) {
                    HStack(spacing: 8) {
                        FilterChip(title: "All", selected: status.isEmpty) { status = "" }
                        ForEach(model.enums.capaStatuses, id: \.self) { item in
                            FilterChip(title: displayName(item), selected: status == item) { status = item }
                        }
                    }
                    .padding(.horizontal)
                }
            }
            .padding(.vertical, 8)
            .fixedSize(horizontal: false, vertical: true)
        }
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                SortMenu(selection: $sort, options: ListSort.allCases)
            }
            if model.user?.allows(.supervisor) == true {
                ToolbarItem(placement: .primaryAction) {
                    Button("New action", systemImage: "plus") { creating = true }
                        .accessibilityLabel("Create corrective action")
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
        await data.load(
            cache: { model.offline.cached([CorrectiveAction].self, forKey: CacheKey.correctiveActions) },
            remote: { try await model.client.list("/api/v1/corrective-actions", query: Query.compact(["status": status])) },
            store: { model.offline.cache($0, forKey: CacheKey.correctiveActions) }
        )
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
    @State private var attempted = false

    private var dueDateValid: Bool {
        guard !dueDate.isEmpty else { return true }
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.dateFormat = "yyyy-MM-dd"
        return formatter.date(from: dueDate) != nil
    }

    var body: some View {
        Form {
            TextField("Title", text: $title)
                .accessibilityLabel("Title")
            TextField("Root cause", text: $rootCause, axis: .vertical)
            TextField("Action plan", text: $actionPlan, axis: .vertical)
            TextField("Due date YYYY-MM-DD", text: $dueDate)
                .accessibilityLabel("Due date")
                .keyboardType(.numbersAndPunctuation)
            if attempted && title.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                Text("Title is required.")
                    .font(.caption)
                    .foregroundStyle(QCTheme.danger)
            }
            if attempted && !dueDateValid {
                Text("Use due date format YYYY-MM-DD.")
                    .font(.caption)
                    .foregroundStyle(QCTheme.danger)
            }
        }
        .navigationTitle("New action")
        .toolbar {
            ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
            ToolbarItem(placement: .confirmationAction) {
                Button("Save") {
                    attempted = true
                    Task { await save() }
                }
                .disabled(saving)
            }
        }
        .errorAlert($errorMessage)
    }

    private func save() async {
        guard !title.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty, dueDateValid else { return }
        saving = true
        defer { saving = false }
        var body: [String: Any] = ["title": title, "root_cause": rootCause, "action_plan": actionPlan, "due_date": dueDate]
        if let nonconformanceId { body["nonconformance_id"] = nonconformanceId.uuidString }
        do {
            if model.offline.isOnline {
                let _: CorrectiveAction = try await model.client.create("/api/v1/corrective-actions", body: body)
            } else {
                model.offline.enqueue(method: "POST", path: "/api/v1/corrective-actions", body: body)
            }
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
                    Section {
                        LabeledContent("Number", value: action.caNumber)
                        Text(action.title).font(QCTheme.headlineFont)
                        LabeledContent("Root cause", value: action.rootCause.isEmpty ? "—" : action.rootCause)
                        LabeledContent("Plan", value: action.actionPlan.isEmpty ? "—" : action.actionPlan)
                        LabeledContent("Due", value: action.dueDate.isEmpty ? "—" : action.dueDate)
                        StatusBadge(text: action.status)
                        if !action.verificationNotes.isEmpty { Text(action.verificationNotes) }
                        if !action.effectivenessNotes.isEmpty { Text(action.effectivenessNotes) }
                    }
                    if model.user?.allows(.supervisor) == true {
                        Section("Workflow") {
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
                    }
                    if model.user?.allows(.qualityManager) == true, action.status == "verification" {
                        Button("Close action") { Task { await move("close") } }
                    }
                    PhotoCaptureSection(
                        entityType: "corrective_action",
                        entityId: action.id,
                        canCapture: model.user?.allows(.operator) == true
                    )
                    AttachmentsSection(entityType: "corrective_action", entityId: action.id, canUpload: model.user?.allows(.operator) == true, canDelete: model.user?.allows(.supervisor) == true)
                }
                .qcListChrome()
            } else if errorMessage == nil {
                LoadingStateView()
            } else {
                ErrorStateView(title: "Action unavailable", message: errorMessage ?? "", retry: { Task { await reload() } })
            }
        }
        .navigationTitle(action?.caNumber ?? "Action")
        .recordMenu(path: "/api/v1/corrective-actions/\(id.uuidString)", noun: "corrective action", editKind: .correctiveActions) { await reload() }
        .task { await reload() }
        .refreshable { await reload() }
        .errorAlert($errorMessage)
    }

    private func reload() async {
        do { action = try await model.client.one("/api/v1/corrective-actions/\(id.uuidString)"); errorMessage = nil }
        catch { if action == nil { errorMessage = error.localizedDescription } }
    }

    private func move(_ step: String) async {
        do {
            action = try await model.client.action("/api/v1/corrective-actions/\(id.uuidString)/\(step)", body: ["notes": notes])
            notes = ""
        } catch { errorMessage = error.localizedDescription }
    }
}
