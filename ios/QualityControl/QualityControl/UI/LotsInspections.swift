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
    @State private var creating = false

    var body: some View {
        Group {
            if let lots = data.value {
                List(lots) { lot in
                    NavigationLink(value: WorkRoute.lot(lot.id)) {
                        VStack(alignment: .leading, spacing: 4) {
                            Text(lot.lotNumber).font(.headline)
                            Text("\(formatNumber(lot.quantity)) \(lot.unit) · \(displayName(lot.source))")
                                .font(.subheadline)
                                .foregroundStyle(.secondary)
                        }
                        .badge(Text(displayName(lot.status)))
                    }
                }
                .overlay {
                    if lots.isEmpty { ContentUnavailableView("No lots", systemImage: "shippingbox", description: Text("Open a lot to start an inspection.")) }
                }
            } else if data.loading {
                ProgressView("Loading lots")
            } else {
                ContentUnavailableView("Could not load lots", systemImage: "wifi.exclamationmark", description: Text(data.error ?? "Pull to refresh."))
            }
        }
        .navigationTitle("Lots")
        .navigationDestination(for: WorkRoute.self) { route in
            switch route {
            case .lot(let id): LotDetailView(id: id)
            case .inspection(let id): InspectionDetailView(id: id)
            }
        }
        .searchable(text: $search, prompt: "Lot number")
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Menu("Status") {
                    Button("All") { status = ""; Task { await reload() } }
                    ForEach(model.enums.lotStatuses, id: \.self) { item in
                        Button(displayName(item)) { status = item; Task { await reload() } }
                    }
                }
            }
            if model.user?.allows(.operator) == true {
                ToolbarItem(placement: .primaryAction) {
                    Button("New lot", systemImage: "plus") { creating = true }
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
        await data.load {
            try await model.client.list("/api/v1/lots", query: Query.compact(["q": search, "status": status]))
        }
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

    var body: some View {
        Form {
            TextField("Lot number", text: $lotNumber)
            Picker("Product", selection: $productId) {
                Text("Select").tag(Optional<UUID>.none)
                ForEach(products) { product in
                    Text("\(product.partNumber) · \(product.name)").tag(Optional(product.id))
                }
            }
            Picker("Site", selection: $siteId) {
                Text("Select").tag(Optional<UUID>.none)
                ForEach(sites) { site in
                    Text("\(site.code) · \(site.name)").tag(Optional(site.id))
                }
            }
            Picker("Supplier", selection: $supplierId) {
                Text("None").tag(Optional<UUID>.none)
                ForEach(suppliers) { supplier in
                    Text(supplier.name).tag(Optional(supplier.id))
                }
            }
            Picker("Source", selection: $source) {
                ForEach(model.enums.lotSources, id: \.self) { Text(displayName($0)).tag($0) }
            }
            TextField("Quantity", text: $quantity).keyboardType(.decimalPad)
            TextField("Unit", text: $unit)
            TextField("Supplier lot", text: $supplierLot)
            TextField("Notes", text: $notes, axis: .vertical)
        }
        .navigationTitle("New lot")
        .toolbar {
            ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
            ToolbarItem(placement: .confirmationAction) {
                Button("Save") { Task { await save() } }
                    .disabled(saving || lotNumber.isEmpty || productId == nil || siteId == nil || parseNumber(quantity) == nil)
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
        guard let productId, let siteId, let quantity = parseNumber(quantity) else { return }
        saving = true
        defer { saving = false }
        var body: [String: Any] = [
            "lot_number": lotNumber,
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
            let _: Lot = try await model.client.create("/api/v1/lots", body: body)
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

    var body: some View {
        Group {
            if let lot {
                Form {
                    LabeledContent("Number", value: lot.lotNumber)
                    LabeledContent("Quantity", value: "\(formatNumber(lot.quantity)) \(lot.unit)")
                    LabeledContent("Source", value: displayName(lot.source))
                    LabeledContent("Supplier lot", value: lot.supplierLot.isEmpty ? "—" : lot.supplierLot)
                    LabeledContent("Received", value: lot.receivedAt.formatted(date: .abbreviated, time: .shortened))
                    if !lot.notes.isEmpty { Text(lot.notes) }
                    StatusBadge(text: lot.status)
                    if model.user?.allows(.supervisor) == true {
                        Picker("Set status", selection: $status) {
                            Text("Choose").tag("")
                            ForEach(model.enums.lotStatuses.filter { $0 != "in_inspection" }, id: \.self) { item in
                                Text(displayName(item)).tag(item)
                            }
                        }
                        Button("Apply status") { Task { await setStatus() } }
                            .disabled(status.isEmpty || status == lot.status)
                    }
                    if model.user?.allows(.inspector) == true, lot.status != "closed" {
                        Button("Start inspection") { Task { await startInspection() } }
                    }
                    AttachmentsSection(entityType: "lot", entityId: lot.id, canUpload: model.user?.allows(.operator) == true, canDelete: model.user?.allows(.supervisor) == true)
                }
            } else {
                ProgressView()
            }
        }
        .navigationTitle(lot?.lotNumber ?? "Lot")
        .task { await reload() }
        .errorAlert($errorMessage)
    }

    private func reload() async {
        do {
            lot = try await model.client.one("/api/v1/lots/\(id.uuidString)")
            status = lot?.status ?? ""
        } catch {
            errorMessage = error.localizedDescription
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
            lot = try await model.client.one("/api/v1/lots/\(id.uuidString)")
            _ = created
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}

struct InspectionsView: View {
    @Environment(AppModel.self) private var model
    @State private var data = RemoteData<[Inspection]>()
    @State private var status = ""

    var body: some View {
        Group {
            if let rows = data.value {
                List(rows) { inspection in
                    NavigationLink(value: inspection.id) {
                        VStack(alignment: .leading, spacing: 4) {
                            Text(inspection.inspectionNumber).font(.headline)
                            Text(inspection.startedAt.formatted(date: .abbreviated, time: .shortened))
                                .font(.subheadline)
                                .foregroundStyle(.secondary)
                        }
                        HStack {
                            StatusBadge(text: inspection.status)
                            StatusBadge(text: inspection.result)
                        }
                    }
                }
                .overlay {
                    if rows.isEmpty { ContentUnavailableView("No inspections", systemImage: "checklist") }
                }
            } else if data.loading {
                ProgressView("Loading inspections")
            } else {
                ContentUnavailableView("Could not load inspections", systemImage: "wifi.exclamationmark", description: Text(data.error ?? "Pull to refresh."))
            }
        }
        .navigationTitle("Inspections")
        .navigationDestination(for: UUID.self) { InspectionDetailView(id: $0) }
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Menu("Status") {
                    Button("All") { status = ""; Task { await reload() } }
                    ForEach(model.enums.inspectionStatuses, id: \.self) { item in
                        Button(displayName(item)) { status = item; Task { await reload() } }
                    }
                }
            }
        }
        .refreshable { await reload() }
        .task { await reload() }
        .errorAlert($data.error)
    }

    private func reload() async {
        await data.load {
            try await model.client.list("/api/v1/inspections", query: Query.compact(["status": status]))
        }
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

    var body: some View {
        Group {
            if let detail {
                Form {
                    LabeledContent("Number", value: detail.inspection.inspectionNumber)
                    LabeledContent("Sample size", value: "\(detail.inspection.sampleSize)")
                    HStack {
                        StatusBadge(text: detail.inspection.status)
                        StatusBadge(text: detail.inspection.result)
                    }
                    if !detail.missingCharacteristics.isEmpty {
                        Text("Still needed: \(detail.missingCharacteristics.joined(separator: ", "))")
                            .foregroundStyle(.orange)
                    }
                    Section("Results") {
                        if detail.results.isEmpty {
                            Text("No results yet").foregroundStyle(.secondary)
                        }
                        ForEach(detail.results) { result in
                            VStack(alignment: .leading) {
                                Text("\(result.characteristicCode) · sample \(result.sampleIndex)")
                                Text(resultLine(result))
                                    .font(.subheadline)
                                    .foregroundStyle(.secondary)
                                if result.calibrationWarning {
                                    Text("Gauge calibration is overdue")
                                        .font(.caption)
                                        .foregroundStyle(.orange)
                                }
                            }
                            .swipeActions {
                                if detail.inspection.status == "in_progress", model.user?.allows(.operator) == true {
                                    Button("Remove", role: .destructive) {
                                        Task { await removeResult(result) }
                                    }
                                }
                            }
                        }
                    }
                    if detail.inspection.status == "in_progress", let plan, model.user?.allows(.operator) == true {
                        Section("Record sample \(sampleIndex)") {
                            Stepper("Sample \(sampleIndex)", value: $sampleIndex, in: 1...max(detail.inspection.sampleSize, 1))
                            ForEach(plan.characteristics) { characteristic in
                                if characteristic.characteristicType == "variable" {
                                    TextField(characteristicLabel(characteristic), text: binding(for: characteristic.id))
                                        .keyboardType(.decimalPad)
                                } else {
                                    Picker(characteristic.name, selection: attributeBinding(characteristic.id)) {
                                        Text("—").tag("")
                                        Text("Pass").tag("pass")
                                        Text("Fail").tag("fail")
                                    }
                                }
                            }
                            Button("Save sample") { Task { await record() } }
                        }
                    }
                    Section("Defects") {
                        ForEach(detail.defects) { defect in
                            VStack(alignment: .leading) {
                                Text("\(defect.code) · \(defect.quantity)")
                                Text(displayName(defect.severity)).font(.caption).foregroundStyle(.secondary)
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
                                .disabled(defectId == nil)
                        }
                    }
                    if detail.inspection.status == "in_progress", model.user?.allows(.inspector) == true {
                        Section("Complete") {
                            TextField("Quantity inspected", text: $inspected).keyboardType(.decimalPad)
                            TextField("Quantity accepted", text: $accepted).keyboardType(.decimalPad)
                            TextField("Quantity rejected", text: $rejected).keyboardType(.decimalPad)
                            TextField("Notes", text: $completeNotes, axis: .vertical)
                            Toggle("Open a nonconformance if this fails", isOn: $openNCR)
                            Button("Complete inspection") { Task { await complete() } }
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
                    AttachmentsSection(
                        entityType: "inspection",
                        entityId: detail.inspection.id,
                        canUpload: model.user?.allows(.operator) == true,
                        canDelete: model.user?.allows(.supervisor) == true
                    )
                }
            } else {
                ProgressView()
            }
        }
        .navigationTitle(detail?.inspection.inspectionNumber ?? "Inspection")
        .task { await reload() }
        .errorAlert($errorMessage)
        .alert("Inspection closed", isPresented: Binding(get: { outcome != nil }, set: { if !$0 { outcome = nil } })) {
            Button("OK", role: .cancel) {}
        } message: {
            Text(outcome ?? "")
        }
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
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func record() async {
        guard let plan else { return }
        var results: [[String: Any]] = []
        for characteristic in plan.characteristics {
            if characteristic.characteristicType == "variable" {
                guard let value = parseNumber(measurements[characteristic.id, default: ""]) else { continue }
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
            errorMessage = "Enter at least one measurement or pass/fail."
            return
        }
        do {
            let _: [InspectionResult] = try await model.client.create("/api/v1/inspections/\(id.uuidString)/results", body: ["results": results])
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
