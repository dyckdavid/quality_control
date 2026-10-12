import SwiftUI

enum WorkRoute: Hashable {
    case lot(UUID)
    case inspection(UUID)
}

struct LotsView: View {
    @Environment(AppModel.self) private var model
    @State private var data = RemoteData<[Lot]>()
    @State private var search = ""
    @State private var status = ""
    @State private var sort: ListSort = .newest
    @State private var creating = false

    private var filtered: [Lot] {
        let rows = data.value ?? []
        let searched: [Lot]
        if search.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            searched = rows
        } else {
            let q = search.lowercased()
            searched = rows.filter {
                $0.lotNumber.lowercased().contains(q)
                    || $0.supplierLot.lowercased().contains(q)
                    || $0.notes.lowercased().contains(q)
                    || $0.status.lowercased().contains(q)
            }
        }
        return sortLots(searched, by: sort)
    }

    var body: some View {
        Group {
            if data.value != nil {
                List(filtered) { lot in
                    NavigationLink(value: WorkRoute.lot(lot.id)) {
                        HStack(alignment: .top, spacing: 12) {
                            Image(systemName: "shippingbox.fill")
                                .foregroundStyle(QCTheme.brandColor)
                                .frame(width: 28)
                                .accessibilityHidden(true)
                            VStack(alignment: .leading, spacing: 4) {
                                Text(lot.lotNumber)
                                    .font(QCTheme.headlineFont)
                                Text("\(formatNumber(lot.quantity)) \(lot.unit) · \(displayName(lot.source))")
                                    .font(.subheadline)
                                    .foregroundStyle(QCTheme.mutedInk)
                                Text(lot.receivedAt.formatted(date: .abbreviated, time: .omitted))
                                    .font(.caption)
                                    .foregroundStyle(QCTheme.mutedInk)
                            }
                            Spacer(minLength: 0)
                            StatusBadge(text: lot.status)
                        }
                        .padding(.vertical, 2)
                    }
                    .accessibilityLabel("Lot \(lot.lotNumber), \(displayName(lot.status))")
                }
                .qcListChrome()
                .overlay {
                    if filtered.isEmpty {
                        EmptyStateView(
                            title: "No lots",
                            systemImage: "shippingbox",
                            description: search.isEmpty && status.isEmpty
                                ? "Open a lot to start an inspection."
                                : "Nothing matches these filters."
                        )
                    }
                }
            } else if data.loading {
                LoadingStateView(message: "Loading lots")
            } else {
                ErrorStateView(
                    title: "Could not load lots",
                    message: data.error ?? "Pull to refresh.",
                    retry: { Task { await reload() } }
                )
            }
        }
        .navigationTitle("Lots")
        .navigationDestination(for: WorkRoute.self) { route in
            switch route {
            case .lot(let id): LotDetailView(id: id)
            case .inspection(let id): InspectionDetailView(id: id)
            }
        }
        .searchable(text: $search, prompt: "Lot number or notes")
        .safeAreaInset(edge: .top) {
            ScrollView(.horizontal, showsIndicators: false) {
                HStack(spacing: 8) {
                    FilterChip(title: "All", selected: status.isEmpty) {
                        status = ""
                        Task { await reload() }
                    }
                    ForEach(model.enums.lotStatuses, id: \.self) { item in
                        FilterChip(title: displayName(item), selected: status == item) {
                            status = item
                            Task { await reload() }
                        }
                    }
                }
                .padding(.horizontal)
                .padding(.vertical, 8)
            }
            .background(QCTheme.canvas.opacity(0.95))
        }
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                SortMenu(selection: $sort, options: ListSort.allCases)
            }
            if model.user?.allows(.operator) == true {
                ToolbarItem(placement: .primaryAction) {
                    Button("New lot", systemImage: "plus") { creating = true }
                        .accessibilityLabel("Create new lot")
                }
            }
        }
        .refreshable { await reload() }
        .task(id: search) { await reload() }
        .sheet(isPresented: $creating, onDismiss: { Task { await reload() } }) {
            NavigationStack { LotForm() }
        }
        .errorAlert($data.error)
    }

    private func reload() async {
        await data.load(
            cache: { model.offline.cached([Lot].self, forKey: CacheKey.lots) },
            remote: {
                try await model.client.list("/api/v1/lots", query: Query.compact(["q": search, "status": status]))
            },
            store: { model.offline.cache($0, forKey: CacheKey.lots) }
        )
    }
}

private func sortLots(_ lots: [Lot], by sort: ListSort) -> [Lot] {
    switch sort {
    case .newest: lots.sorted { $0.receivedAt > $1.receivedAt }
    case .oldest: lots.sorted { $0.receivedAt < $1.receivedAt }
    case .nameAZ: lots.sorted { $0.lotNumber.localizedCaseInsensitiveCompare($1.lotNumber) == .orderedAscending }
    case .nameZA: lots.sorted { $0.lotNumber.localizedCaseInsensitiveCompare($1.lotNumber) == .orderedDescending }
    case .status: lots.sorted { $0.status.localizedCaseInsensitiveCompare($1.status) == .orderedAscending }
    }
}

struct LotForm: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @State private var lotNumber = ""
    @State private var quantity = ""
    @State private var unit = ""
    @State private var source = "incoming"
    @State private var supplierLot = ""
    @State private var notes = ""
    @State private var productId: UUID?
    @State private var siteId: UUID?
    @State private var supplierId: UUID?
    @State private var products: [Product] = []
    @State private var sites: [Site] = []
    @State private var suppliers: [Supplier] = []
    @State private var errorMessage: String?
    @State private var saving = false
    @State private var attempted = false

    private var quantityValue: Double? { parseNumber(quantity) }
    private var isValid: Bool {
        !lotNumber.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            && productId != nil
            && siteId != nil
            && quantityValue != nil
            && (quantityValue ?? -1) > 0
    }

    var body: some View {
        Form {
            Section {
                TextField("Lot number", text: $lotNumber)
                    .accessibilityLabel("Lot number")
                if attempted && lotNumber.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                    Text("Lot number is required.")
                        .font(.caption)
                        .foregroundStyle(QCTheme.danger)
                }
                Picker("Product", selection: $productId) {
                    Text("Select").tag(Optional<UUID>.none)
                    ForEach(products) { product in
                        Text("\(product.partNumber) · \(product.name)").tag(Optional(product.id))
                    }
                }
                .accessibilityLabel("Product")
                Picker("Site", selection: $siteId) {
                    Text("Select").tag(Optional<UUID>.none)
                    ForEach(sites) { site in
                        Text("\(site.code) · \(site.name)").tag(Optional(site.id))
                    }
                }
                .accessibilityLabel("Site")
                Picker("Supplier", selection: $supplierId) {
                    Text("None").tag(Optional<UUID>.none)
                    ForEach(suppliers) { supplier in
                        Text(supplier.name).tag(Optional(supplier.id))
                    }
                }
                Picker("Source", selection: $source) {
                    ForEach(model.enums.lotSources, id: \.self) { Text(displayName($0)).tag($0) }
                }
            }
            Section("Quantity") {
                TextField("Quantity", text: $quantity)
                    .keyboardType(.decimalPad)
                    .accessibilityLabel("Quantity")
                TextField("Unit", text: $unit)
                    .accessibilityLabel("Unit")
                if attempted, quantityValue == nil || (quantityValue ?? 0) <= 0 {
                    Text("Enter a quantity greater than zero.")
                        .font(.caption)
                        .foregroundStyle(QCTheme.danger)
                }
            }
            Section("Notes") {
                TextField("Supplier lot", text: $supplierLot)
                TextField("Notes", text: $notes, axis: .vertical)
            }
        }
        .navigationTitle("New lot")
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
        .task { await loadChoices() }
        .errorAlert($errorMessage)
    }

    private func loadChoices() async {
        products = (try? await model.client.list("/api/v1/products", query: Query.compact(["active": "true"]))) ?? []
        sites = (try? await model.client.list("/api/v1/sites", query: Query.compact(["active": "true"]))) ?? []
        suppliers = (try? await model.client.list("/api/v1/suppliers", query: Query.compact(["active": "true"]))) ?? []
    }

    private func save() async {
        guard isValid, let productId, let siteId, let quantity = quantityValue else { return }
        saving = true
        defer { saving = false }
        var body: [String: Any] = [
            "lot_number": lotNumber.trimmingCharacters(in: .whitespacesAndNewlines),
            "product_id": productId.uuidString,
            "site_id": siteId.uuidString,
            "quantity": quantity,
            "source": source,
            "supplier_lot": supplierLot,
            "notes": notes
        ]
        if !unit.isEmpty { body["unit"] = unit }
        if let supplierId { body["supplier_id"] = supplierId.uuidString }
        do {
            if model.offline.isOnline {
                let _: Lot = try await model.client.create("/api/v1/lots", body: body)
            } else {
                model.offline.enqueue(method: "POST", path: "/api/v1/lots", body: body)
            }
            dismiss()
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}

struct LotDetailView: View {
    @Environment(AppModel.self) private var model
    var id: UUID
    @State private var lot: Lot?
    @State private var errorMessage: String?
    @State private var status = ""
    @State private var startedInspection: UUID?

    var body: some View {
        Group {
            if let lot {
                Form {
                    Section("Lot") {
                        LabeledContent("Number", value: lot.lotNumber)
                        LabeledContent("Quantity", value: "\(formatNumber(lot.quantity)) \(lot.unit)")
                        LabeledContent("Source", value: displayName(lot.source))
                        LabeledContent("Supplier lot", value: lot.supplierLot.isEmpty ? "—" : lot.supplierLot)
                        LabeledContent("Received", value: lot.receivedAt.formatted(date: .abbreviated, time: .shortened))
                        StatusBadge(text: lot.status)
                        if !lot.notes.isEmpty { Text(lot.notes) }
                    }
                    if model.user?.allows(.supervisor) == true {
                        Section("Status") {
                            Picker("Set status", selection: $status) {
                                Text("Choose").tag("")
                                ForEach(model.enums.lotStatuses.filter { $0 != "in_inspection" }, id: \.self) { item in
                                    Text(displayName(item)).tag(item)
                                }
                            }
                            .accessibilityLabel("Set lot status")
                            Button("Apply status") { Task { await setStatus() } }
                                .disabled(status.isEmpty || status == lot.status)
                        }
                    }
                    if model.user?.allows(.inspector) == true, lot.status != "closed" {
                        Section("Inspection") {
                            Button {
                                Task { await startInspection() }
                            } label: {
                                Label("Start inspection", systemImage: "checklist")
                            }
                            .accessibilityLabel("Start inspection for this lot")
                            if let startedInspection {
                                NavigationLink("Open latest inspection") {
                                    InspectionDetailView(id: startedInspection)
                                }
                            }
                        }
                    }
                    PhotoCaptureSection(
                        entityType: "lot",
                        entityId: lot.id,
                        canCapture: model.user?.allows(.operator) == true
                    )
                    AttachmentsSection(
                        entityType: "lot",
                        entityId: lot.id,
                        canUpload: model.user?.allows(.operator) == true,
                        canDelete: model.user?.allows(.supervisor) == true
                    )
                }
                .qcListChrome()
            } else if errorMessage == nil {
                LoadingStateView(message: "Loading lot")
            } else {
                ErrorStateView(title: "Lot unavailable", message: errorMessage ?? "", retry: { Task { await reload() } })
            }
        }
        .navigationTitle(lot?.lotNumber ?? "Lot")
        .task { await reload() }
        .refreshable { await reload() }
        .errorAlert($errorMessage)
    }

    private func reload() async {
        do {
            lot = try await model.client.one("/api/v1/lots/\(id.uuidString)")
            status = lot?.status ?? ""
            errorMessage = nil
        } catch {
            if lot == nil { errorMessage = error.localizedDescription }
        }
    }

    private func setStatus() async {
        do {
            lot = try await model.client.action("/api/v1/lots/\(id.uuidString)/status", body: ["status": status])
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func startInspection() async {
        do {
            let created: InspectionDetail = try await model.client.create("/api/v1/inspections", body: ["lot_id": id.uuidString])
            startedInspection = created.inspection.id
            lot = try await model.client.one("/api/v1/lots/\(id.uuidString)")
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}

struct InspectionsView: View {
    @Environment(AppModel.self) private var model
    @State private var data = RemoteData<[Inspection]>()
    @State private var status = ""
    @State private var resultFilter = ""
    @State private var search = ""
    @State private var sort: ListSort = .newest

    private var filtered: [Inspection] {
        var rows = data.value ?? []
        if !search.isEmpty {
            let q = search.lowercased()
            rows = rows.filter {
                $0.inspectionNumber.lowercased().contains(q)
                    || $0.status.lowercased().contains(q)
                    || $0.result.lowercased().contains(q)
                    || $0.notes.lowercased().contains(q)
            }
        }
        if !resultFilter.isEmpty {
            rows = rows.filter { $0.result == resultFilter }
        }
        switch sort {
        case .newest: return rows.sorted { $0.startedAt > $1.startedAt }
        case .oldest: return rows.sorted { $0.startedAt < $1.startedAt }
        case .nameAZ: return rows.sorted { $0.inspectionNumber.localizedCaseInsensitiveCompare($1.inspectionNumber) == .orderedAscending }
        case .nameZA: return rows.sorted { $0.inspectionNumber.localizedCaseInsensitiveCompare($1.inspectionNumber) == .orderedDescending }
        case .status: return rows.sorted { $0.status.localizedCaseInsensitiveCompare($1.status) == .orderedAscending }
        }
    }

    var body: some View {
        Group {
            if data.value != nil {
                List(filtered) { inspection in
                    NavigationLink(value: inspection.id) {
                        HStack(alignment: .top, spacing: 12) {
                            Image(systemName: "checklist")
                                .foregroundStyle(QCTheme.brandColor)
                                .frame(width: 28)
                                .accessibilityHidden(true)
                            VStack(alignment: .leading, spacing: 4) {
                                Text(inspection.inspectionNumber)
                                    .font(QCTheme.headlineFont)
                                Text(inspection.startedAt.formatted(date: .abbreviated, time: .shortened))
                                    .font(.subheadline)
                                    .foregroundStyle(QCTheme.mutedInk)
                            }
                            Spacer(minLength: 0)
                            VStack(alignment: .trailing, spacing: 4) {
                                StatusBadge(text: inspection.status)
                                StatusBadge(text: inspection.result)
                            }
                        }
                    }
                    .accessibilityLabel("Inspection \(inspection.inspectionNumber), \(displayName(inspection.status)), \(displayName(inspection.result))")
                }
                .qcListChrome()
                .overlay {
                    if filtered.isEmpty {
                        EmptyStateView(title: "No inspections", systemImage: "checklist", description: "Start one from a lot, or clear filters.")
                    }
                }
            } else if data.loading {
                LoadingStateView(message: "Loading inspections")
            } else {
                ErrorStateView(
                    title: "Could not load inspections",
                    message: data.error ?? "Pull to refresh.",
                    retry: { Task { await reload() } }
                )
            }
        }
        .navigationTitle("Inspections")
        .navigationDestination(for: UUID.self) { InspectionDetailView(id: $0) }
        .searchable(text: $search, prompt: "Inspection number")
        .safeAreaInset(edge: .top) {
            VStack(alignment: .leading, spacing: 8) {
                ScrollView(.horizontal, showsIndicators: false) {
                    HStack(spacing: 8) {
                        FilterChip(title: "All statuses", selected: status.isEmpty) {
                            status = ""
                            Task { await reload() }
                        }
                        ForEach(model.enums.inspectionStatuses, id: \.self) { item in
                            FilterChip(title: displayName(item), selected: status == item) {
                                status = item
                                Task { await reload() }
                            }
                        }
                    }
                    .padding(.horizontal)
                }
                ScrollView(.horizontal, showsIndicators: false) {
                    HStack(spacing: 8) {
                        FilterChip(title: "All results", selected: resultFilter.isEmpty) {
                            resultFilter = ""
                        }
                        ForEach(model.enums.inspectionResults, id: \.self) { item in
                            FilterChip(title: displayName(item), selected: resultFilter == item) {
                                resultFilter = item
                            }
                        }
                    }
                    .padding(.horizontal)
                }
            }
            .padding(.vertical, 8)
            .background(QCTheme.canvas.opacity(0.95))
        }
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                SortMenu(selection: $sort, options: ListSort.allCases)
            }
        }
        .refreshable { await reload() }
        .task { await reload() }
        .errorAlert($data.error)
    }

    private func reload() async {
        await data.load(
            cache: { model.offline.cached([Inspection].self, forKey: CacheKey.inspections) },
            remote: {
                try await model.client.list("/api/v1/inspections", query: Query.compact(["status": status]))
            },
            store: { model.offline.cache($0, forKey: CacheKey.inspections) }
        )
    }
}

struct InspectionDetailView: View {
    @Environment(AppModel.self) private var model
    var id: UUID
    @State private var detail: InspectionDetail?
    @State private var plan: PlanDetail?
    @State private var sampleIndex = 1
    @State private var measurements: [UUID: String] = [:]
    @State private var attributes: [UUID: String] = [:]
    @State private var defectId: UUID?
    @State private var defectQty = "1"
    @State private var defectNotes = ""
    @State private var defects: [DefectCode] = []
    @State private var inspected = ""
    @State private var accepted = ""
    @State private var rejected = ""
    @State private var completeNotes = ""
    @State private var openNCR = true
    @State private var errorMessage: String?
    @State private var outcome: String?
    @State private var confirmCancel = false
    @State private var validationMessage: String?

    var body: some View {
        Group {
            if let detail {
                Form {
                    Section("Summary") {
                        LabeledContent("Number", value: detail.inspection.inspectionNumber)
                        LabeledContent("Sample size", value: "\(detail.inspection.sampleSize)")
                        HStack {
                            StatusBadge(text: detail.inspection.status)
                            StatusBadge(text: detail.inspection.result)
                        }
                        .accessibilityElement(children: .combine)
                        if !detail.missingCharacteristics.isEmpty {
                            Text("Still needed: \(detail.missingCharacteristics.joined(separator: ", "))")
                                .foregroundStyle(QCTheme.amber)
                                .accessibilityLabel("Missing characteristics: \(detail.missingCharacteristics.joined(separator: ", "))")
                        }
                    }
                    Section("Results") {
                        if detail.results.isEmpty {
                            Text("No results yet").foregroundStyle(QCTheme.mutedInk)
                        }
                        ForEach(detail.results) { result in
                            VStack(alignment: .leading, spacing: 4) {
                                Text("\(result.characteristicCode) · sample \(result.sampleIndex)")
                                    .font(QCTheme.headlineFont)
                                Text(resultLine(result))
                                    .font(.subheadline)
                                    .foregroundStyle(QCTheme.mutedInk)
                                if result.calibrationWarning {
                                    Label("Gauge calibration is overdue", systemImage: "exclamationmark.circle")
                                        .font(.caption)
                                        .foregroundStyle(QCTheme.amber)
                                }
                            }
                            .swipeActions {
                                if detail.inspection.status == "in_progress", model.user?.allows(.operator) == true {
                                    Button("Remove", role: .destructive) {
                                        Task { await removeResult(result) }
                                    }
                                }
                            }
                            .accessibilityLabel("\(result.characteristicCode), sample \(result.sampleIndex), \(resultLine(result))")
                        }
                    }
                    if detail.inspection.status == "in_progress", let plan, model.user?.allows(.operator) == true {
                        Section("Record sample \(sampleIndex)") {
                            Stepper("Sample \(sampleIndex)", value: $sampleIndex, in: 1...max(detail.inspection.sampleSize, 1))
                                .accessibilityLabel("Sample index \(sampleIndex)")
                            ForEach(plan.characteristics) { characteristic in
                                if characteristic.characteristicType == "variable" {
                                    TextField(characteristicLabel(characteristic), text: binding(for: characteristic.id))
                                        .keyboardType(.decimalPad)
                                        .accessibilityLabel(characteristicLabel(characteristic))
                                } else {
                                    Picker(characteristic.name, selection: attributeBinding(characteristic.id)) {
                                        Text("—").tag("")
                                        Text("Pass").tag("pass")
                                        Text("Fail").tag("fail")
                                    }
                                    .accessibilityLabel(characteristic.name)
                                }
                            }
                            if let validationMessage {
                                Text(validationMessage)
                                    .font(.caption)
                                    .foregroundStyle(QCTheme.danger)
                            }
                            Button {
                                Task { await record() }
                            } label: {
                                Label("Save sample", systemImage: "square.and.arrow.down")
                            }
                            .accessibilityLabel("Save sample measurements")
                        }
                    }
                    Section("Defects") {
                        ForEach(detail.defects) { defect in
                            VStack(alignment: .leading) {
                                Text("\(defect.code) · \(defect.quantity)")
                                    .font(QCTheme.headlineFont)
                                Text(displayName(defect.severity))
                                    .font(.caption)
                                    .foregroundStyle(QCTheme.mutedInk)
                            }
                            .swipeActions {
                                if detail.inspection.status == "in_progress", model.user?.allows(.inspector) == true {
                                    Button("Remove", role: .destructive) { Task { await removeDefect(defect) } }
                                }
                            }
                        }
                        if detail.inspection.status == "in_progress", model.user?.allows(.inspector) == true {
                            Picker("Defect", selection: $defectId) {
                                Text("Select").tag(Optional<UUID>.none)
                                ForEach(defects) { code in
                                    Text("\(code.code) · \(code.name)").tag(Optional(code.id))
                                }
                            }
                            TextField("Quantity", text: $defectQty).keyboardType(.numberPad)
                            TextField("Notes", text: $defectNotes)
                            Button("Add defect") { Task { await addDefect() } }
                                .disabled(defectId == nil || Int(defectQty).map { $0 <= 0 } ?? true)
                        }
                    }
                    if detail.inspection.status == "in_progress", model.user?.allows(.inspector) == true {
                        Section("Complete") {
                            TextField("Quantity inspected", text: $inspected)
                                .keyboardType(.decimalPad)
                                .accessibilityLabel("Quantity inspected")
                            TextField("Quantity accepted", text: $accepted)
                                .keyboardType(.decimalPad)
                                .accessibilityLabel("Quantity accepted")
                            TextField("Quantity rejected", text: $rejected)
                                .keyboardType(.decimalPad)
                                .accessibilityLabel("Quantity rejected")
                            TextField("Notes", text: $completeNotes, axis: .vertical)
                            Toggle("Open a nonconformance if this fails", isOn: $openNCR)
                            if let completeError = completeValidationError {
                                Text(completeError)
                                    .font(.caption)
                                    .foregroundStyle(QCTheme.danger)
                            }
                            Button("Complete inspection") { Task { await complete() } }
                                .disabled(completeValidationError != nil && !(inspected.isEmpty && accepted.isEmpty && rejected.isEmpty))
                            Button("Cancel inspection", role: .destructive) { confirmCancel = true }
                            .confirmationDialog("Cancel this inspection?", isPresented: $confirmCancel, titleVisibility: .visible) {
                                Button("Cancel inspection", role: .destructive) { Task { await cancel() } }
                                Button("Keep it", role: .cancel) {}
                            } message: {
                                Text("The lot status is recalculated from the inspections that remain.")
                            }
                        }
                    }
                    if !detail.inspection.notes.isEmpty {
                        Section("Notes") { Text(detail.inspection.notes) }
                    }
                    PhotoCaptureSection(
                        entityType: "inspection",
                        entityId: detail.inspection.id,
                        canCapture: model.user?.allows(.operator) == true
                    )
                    AttachmentsSection(
                        entityType: "inspection",
                        entityId: detail.inspection.id,
                        canUpload: model.user?.allows(.operator) == true,
                        canDelete: model.user?.allows(.supervisor) == true
                    )
                }
                .qcListChrome()
            } else if errorMessage == nil {
                LoadingStateView(message: "Loading inspection")
            } else {
                ErrorStateView(title: "Inspection unavailable", message: errorMessage ?? "", retry: { Task { await reload() } })
            }
        }
        .navigationTitle(detail?.inspection.inspectionNumber ?? "Inspection")
        .task { await reload() }
        .refreshable { await reload() }
        .errorAlert($errorMessage)
        .alert("Inspection closed", isPresented: Binding(get: { outcome != nil }, set: { if !$0 { outcome = nil } })) {
            Button("OK", role: .cancel) {}
        } message: {
            Text(outcome ?? "")
        }
    }

    private var completeValidationError: String? {
        let fields = [inspected, accepted, rejected]
        if fields.allSatisfy({ $0.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty }) {
            return nil
        }
        guard let inspectedValue = parseNumber(inspected),
              let acceptedValue = parseNumber(accepted),
              let rejectedValue = parseNumber(rejected) else {
            return "Enter inspected, accepted, and rejected quantities as numbers."
        }
        if inspectedValue < 0 || acceptedValue < 0 || rejectedValue < 0 {
            return "Quantities cannot be negative."
        }
        if abs((acceptedValue + rejectedValue) - inspectedValue) > 0.0001 {
            return "Accepted + rejected should equal inspected."
        }
        return nil
    }

    private func binding(for id: UUID) -> Binding<String> {
        Binding(get: { measurements[id, default: ""] }, set: { measurements[id] = $0 })
    }

    private func attributeBinding(_ id: UUID) -> Binding<String> {
        Binding(get: { attributes[id, default: ""] }, set: { attributes[id] = $0 })
    }

    private func characteristicLabel(_ characteristic: Characteristic) -> String {
        var text = characteristic.name
        if let lower = characteristic.lowerLimit, let upper = characteristic.upperLimit {
            text += " (\(formatNumber(lower))–\(formatNumber(upper)) \(characteristic.unit))"
        }
        return text
    }

    private func resultLine(_ result: InspectionResult) -> String {
        if let value = result.measuredValue {
            return "\(formatNumber(value)) \(result.unit) · \(result.inSpec ? "in spec" : "out of spec")"
        }
        return "\(result.attributeResult ?? "—") · \(result.inSpec ? "in spec" : "out of spec")"
    }

    private func reload() async {
        do {
            let loaded: InspectionDetail = try await model.client.one("/api/v1/inspections/\(id.uuidString)")
            detail = loaded
            plan = try? await model.client.one("/api/v1/plans/\(loaded.inspection.planId.uuidString)")
            defects = (try? await model.client.list("/api/v1/defect-codes", query: Query.compact(["active": "true"]))) ?? []
            errorMessage = nil
        } catch {
            if detail == nil { errorMessage = error.localizedDescription }
        }
    }

    private func record() async {
        guard let plan else { return }
        validationMessage = nil
        var results: [[String: Any]] = []
        for characteristic in plan.characteristics {
            if characteristic.characteristicType == "variable" {
                let raw = measurements[characteristic.id, default: ""]
                if raw.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty { continue }
                guard let value = parseNumber(raw) else {
                    validationMessage = "“\(characteristic.name)” must be a number."
                    return
                }
                results.append([
                    "characteristic_id": characteristic.id.uuidString,
                    "sample_index": sampleIndex,
                    "measured_value": value
                ])
            } else if let choice = attributes[characteristic.id], choice == "pass" || choice == "fail" {
                results.append([
                    "characteristic_id": characteristic.id.uuidString,
                    "sample_index": sampleIndex,
                    "attribute_result": choice
                ])
            }
        }
        guard !results.isEmpty else {
            validationMessage = "Enter at least one measurement or pass/fail."
            return
        }
        do {
            let _: [InspectionResult] = try await model.client.create("/api/v1/inspections/\(id.uuidString)/results", body: ["results": results])
            measurements = [:]
            attributes = [:]
            await reload()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func removeResult(_ result: InspectionResult) async {
        do {
            try await model.client.delete("/api/v1/inspection-results/\(result.id.uuidString)")
            await reload()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func addDefect() async {
        guard let defectId, let quantity = Int(defectQty), quantity > 0 else { return }
        do {
            let _: InspectionDefect = try await model.client.create("/api/v1/inspections/\(id.uuidString)/defects", body: [
                "defect_code_id": defectId.uuidString,
                "quantity": quantity,
                "notes": defectNotes
            ])
            defectNotes = ""
            await reload()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func removeDefect(_ defect: InspectionDefect) async {
        do {
            try await model.client.delete("/api/v1/inspection-defects/\(defect.id.uuidString)")
            await reload()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func complete() async {
        if let completeValidationError, !(inspected.isEmpty && accepted.isEmpty && rejected.isEmpty) {
            errorMessage = completeValidationError
            return
        }
        var body: [String: Any] = ["notes": completeNotes, "open_ncr": openNCR]
        if let inspected = parseNumber(inspected), let accepted = parseNumber(accepted), let rejected = parseNumber(rejected) {
            body["quantity_inspected"] = inspected
            body["quantity_accepted"] = accepted
            body["quantity_rejected"] = rejected
        }
        do {
            let finished: CompletedInspection = try await model.client.action("/api/v1/inspections/\(id.uuidString)/complete", body: body)
            outcome = "Result: \(displayName(finished.inspection.result))" + (finished.nonconformance.map { ". Opened \($0.ncrNumber)." } ?? ".")
            await reload()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func cancel() async {
        do {
            let _: Inspection = try await model.client.action("/api/v1/inspections/\(id.uuidString)/cancel")
            await reload()
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}
