import SwiftUI

struct ProductsView: View {
    @Environment(AppModel.self) private var model
    @State private var data = RemoteData<[Product]>()
    @State private var creating = false
    @State private var search = ""

    var body: some View {
        simpleCatalog(title: "Products", rows: data.value, loading: data.loading, problem: data.error, empty: "No products") { product in
            VStack(alignment: .leading) {
                Text(product.partNumber).font(.headline)
                Text(product.name).foregroundStyle(.secondary)
            }
            if !product.active { Text("Inactive").font(.caption) }
        }
        .searchable(text: $search)
        .toolbar { createButton("New product", allowed: .qualityManager) { creating = true } }
        .refreshable { await reload() }
        .task(id: search) { await reload() }
        .sheet(isPresented: $creating, onDismiss: { Task { await reload() } }) { NavigationStack { ProductForm() } }
        .errorAlert($data.error)
    }

    private func reload() async {
        await data.load { try await model.client.list("/api/v1/products", query: Query.compact(["q": search])) }
    }
}

struct ProductForm: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @State private var partNumber = ""
    @State private var name = ""
    @State private var description = ""
    @State private var unit = "ea"
    @State private var errorMessage: String?

    var body: some View {
        Form {
            TextField("Part number", text: $partNumber)
            TextField("Name", text: $name)
            TextField("Description", text: $description, axis: .vertical)
            TextField("Unit", text: $unit)
        }
        .navigationTitle("New product")
        .toolbar {
            ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
            ToolbarItem(placement: .confirmationAction) {
                Button("Save") { Task { await save() } }.disabled(partNumber.isEmpty || name.isEmpty)
            }
        }
        .errorAlert($errorMessage)
    }

    private func save() async {
        do {
            let _: Product = try await model.client.create("/api/v1/products", body: [
                "part_number": partNumber, "name": name, "description": description, "unit": unit
            ])
            dismiss()
        } catch { errorMessage = error.localizedDescription }
    }
}

struct PlansView: View {
    @Environment(AppModel.self) private var model
    @State private var data = RemoteData<[InspectionPlan]>()
    @State private var creating = false

    var body: some View {
        Group {
            if let plans = data.value {
                List(plans) { plan in
                    NavigationLink(value: plan.id) {
                        VStack(alignment: .leading) {
                            Text("\(plan.name) · \(plan.revision)").font(.headline)
                            Text(displayName(plan.samplingType)).font(.subheadline).foregroundStyle(.secondary)
                        }
                        StatusBadge(text: plan.status)
                    }
                }
            } else if data.loading { ProgressView() }
        }
        .navigationTitle("Plans")
        .navigationDestination(for: UUID.self) { PlanDetailView(id: $0) }
        .toolbar { createButton("New plan", allowed: .qualityManager) { creating = true } }
        .refreshable { await reload() }
        .task { await reload() }
        .sheet(isPresented: $creating, onDismiss: { Task { await reload() } }) { NavigationStack { PlanForm() } }
        .errorAlert($data.error)
    }

    private func reload() async {
        await data.load { try await model.client.list("/api/v1/plans") }
    }
}

struct PlanForm: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @State private var productId: UUID?
    @State private var products: [Product] = []
    @State private var revision = "A"
    @State private var name = ""
    @State private var sampling = "fixed"
    @State private var sampleSize = "1"
    @State private var code = "LEN"
    @State private var characteristicName = "Length"
    @State private var kind = "variable"
    @State private var nominal = ""
    @State private var lower = ""
    @State private var upper = ""
    @State private var unit = "mm"
    @State private var critical = false
    @State private var errorMessage: String?

    var body: some View {
        Form {
            Picker("Product", selection: $productId) {
                Text("Select").tag(Optional<UUID>.none)
                ForEach(products) { Text("\($0.partNumber) · \($0.name)").tag(Optional($0.id)) }
            }
            TextField("Revision", text: $revision)
            TextField("Plan name", text: $name)
            Picker("Sampling", selection: $sampling) {
                ForEach(model.enums.samplingTypes, id: \.self) { Text(displayName($0)).tag($0) }
            }
            TextField("Sample size", text: $sampleSize).keyboardType(.numberPad)
            Section("First characteristic") {
                TextField("Code", text: $code)
                TextField("Name", text: $characteristicName)
                Picker("Type", selection: $kind) {
                    ForEach(model.enums.characteristicTypes, id: \.self) { Text(displayName($0)).tag($0) }
                }
                if kind == "variable" {
                    TextField("Nominal", text: $nominal).keyboardType(.decimalPad)
                    TextField("Lower limit", text: $lower).keyboardType(.decimalPad)
                    TextField("Upper limit", text: $upper).keyboardType(.decimalPad)
                    TextField("Unit", text: $unit)
                }
                Toggle("Critical", isOn: $critical)
            }
        }
        .navigationTitle("New plan")
        .toolbar {
            ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
            ToolbarItem(placement: .confirmationAction) { Button("Save") { Task { await save() } }.disabled(productId == nil || name.isEmpty) }
        }
        .task { products = (try? await model.client.list("/api/v1/products", query: Query.compact(["active": "true"]))) ?? [] }
        .errorAlert($errorMessage)
    }

    private func save() async {
        guard let productId else { return }
        var characteristic: [String: Any] = [
            "sequence": 1,
            "code": code,
            "name": characteristicName,
            "characteristic_type": kind,
            "critical": critical,
            "unit": unit
        ]
        if let nominal = parseNumber(nominal) { characteristic["nominal"] = nominal }
        if let lower = parseNumber(lower) { characteristic["lower_limit"] = lower }
        if let upper = parseNumber(upper) { characteristic["upper_limit"] = upper }
        var body: [String: Any] = [
            "product_id": productId.uuidString,
            "revision": revision,
            "name": name,
            "sampling_type": sampling,
            "characteristics": [characteristic]
        ]
        if let size = Int(sampleSize), size > 0 { body["sample_size"] = size }
        do {
            let _: PlanDetail = try await model.client.create("/api/v1/plans", body: body)
            dismiss()
        } catch { errorMessage = error.localizedDescription }
    }
}

struct PlanDetailView: View {
    @Environment(AppModel.self) private var model
    var id: UUID
    @State private var detail: PlanDetail?
    @State private var revision = ""
    @State private var errorMessage: String?
    @State private var adding = false
    @State private var confirmDelete = false

    var body: some View {
        Group {
            if let detail {
                Form {
                    LabeledContent("Name", value: detail.plan.name)
                    LabeledContent("Revision", value: detail.plan.revision)
                    LabeledContent("Sampling", value: displayName(detail.plan.samplingType))
                    StatusBadge(text: detail.plan.status)
                    Section("Characteristics") {
                        ForEach(detail.characteristics) { characteristic in
                            VStack(alignment: .leading) {
                                Text("\(characteristic.sequence). \(characteristic.code) · \(characteristic.name)")
                                Text(characteristic.characteristicType == "variable" ? limitText(characteristic) : "Pass or fail")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                                if characteristic.critical { Text("Critical").font(.caption).foregroundStyle(.red) }
                            }
                        }
                    }
                    if detail.plan.status == "draft", model.user?.allows(.qualityManager) == true {
                        Button("Add characteristic") { adding = true }
                        Button("Activate plan") { Task { await run("activate") } }
                        Button("Delete draft", role: .destructive) { confirmDelete = true }
                        .confirmationDialog("Delete this draft plan?", isPresented: $confirmDelete, titleVisibility: .visible) {
                            Button("Delete draft", role: .destructive) { Task { await deleteDraft() } }
                            Button("Keep it", role: .cancel) {}
                        }
                    }
                    if detail.plan.status == "active", model.user?.allows(.qualityManager) == true {
                        TextField("New revision", text: $revision)
                        Button("Revise") { Task { await revise() } }.disabled(revision.isEmpty)
                        Button("Mark obsolete", role: .destructive) { Task { await run("obsolete") } }
                    }
                }
            } else { ProgressView() }
        }
        .navigationTitle(detail?.plan.revision ?? "Plan")
        .task { await reload() }
        .sheet(isPresented: $adding, onDismiss: { Task { await reload() } }) {
            NavigationStack { CharacteristicForm(planId: id, nextSequence: (detail?.characteristics.count ?? 0) + 1) }
        }
        .errorAlert($errorMessage)
    }

    private func limitText(_ characteristic: Characteristic) -> String {
        let lower = characteristic.lowerLimit.map(formatNumber) ?? "—"
        let upper = characteristic.upperLimit.map(formatNumber) ?? "—"
        return "\(lower) to \(upper) \(characteristic.unit)"
    }

    private func reload() async {
        do { detail = try await model.client.one("/api/v1/plans/\(id.uuidString)") }
        catch { errorMessage = error.localizedDescription }
    }

    private func run(_ action: String) async {
        do {
            detail = try await model.client.action("/api/v1/plans/\(id.uuidString)/\(action)")
        } catch { errorMessage = error.localizedDescription }
    }

    private func revise() async {
        do {
            let created: PlanDetail = try await model.client.create("/api/v1/plans/\(id.uuidString)/revise", body: ["revision": revision])
            detail = created
        } catch { errorMessage = error.localizedDescription }
    }

    private func deleteDraft() async {
        do {
            try await model.client.delete("/api/v1/plans/\(id.uuidString)")
            detail = nil
            errorMessage = "Draft deleted. Go back to the list."
        } catch { errorMessage = error.localizedDescription }
    }
}

struct CharacteristicForm: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    var planId: UUID
    var nextSequence: Int
    @State private var code = ""
    @State private var name = ""
    @State private var kind = "attribute"
    @State private var nominal = ""
    @State private var lower = ""
    @State private var upper = ""
    @State private var unit = ""
    @State private var critical = false
    @State private var errorMessage: String?

    var body: some View {
        Form {
            TextField("Code", text: $code)
            TextField("Name", text: $name)
            Picker("Type", selection: $kind) {
                ForEach(model.enums.characteristicTypes, id: \.self) { Text(displayName($0)).tag($0) }
            }
            if kind == "variable" {
                TextField("Nominal", text: $nominal).keyboardType(.decimalPad)
                TextField("Lower", text: $lower).keyboardType(.decimalPad)
                TextField("Upper", text: $upper).keyboardType(.decimalPad)
                TextField("Unit", text: $unit)
            }
            Toggle("Critical", isOn: $critical)
        }
        .navigationTitle("Characteristic")
        .toolbar {
            ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
            ToolbarItem(placement: .confirmationAction) { Button("Save") { Task { await save() } }.disabled(code.isEmpty || name.isEmpty) }
        }
        .errorAlert($errorMessage)
    }

    private func save() async {
        var body: [String: Any] = [
            "sequence": nextSequence, "code": code, "name": name, "characteristic_type": kind, "unit": unit, "critical": critical
        ]
        if let nominal = parseNumber(nominal) { body["nominal"] = nominal }
        if let lower = parseNumber(lower) { body["lower_limit"] = lower }
        if let upper = parseNumber(upper) { body["upper_limit"] = upper }
        do {
            let _: Characteristic = try await model.client.create("/api/v1/plans/\(planId.uuidString)/characteristics", body: body)
            dismiss()
        } catch { errorMessage = error.localizedDescription }
    }
}

struct SitesView: View {
    @Environment(AppModel.self) private var model
    @State private var data = RemoteData<[Site]>()
    @State private var creating = false
    var body: some View {
        simpleCatalog(title: "Sites", rows: data.value, loading: data.loading, problem: data.error, empty: "No sites") { site in
            VStack(alignment: .leading) { Text(site.code).font(.headline); Text(site.name).foregroundStyle(.secondary) }
        }
        .toolbar { createButton("New site", allowed: .supervisor) { creating = true } }
        .refreshable { await reload() }
        .task { await reload() }
        .sheet(isPresented: $creating, onDismiss: { Task { await reload() } }) { NavigationStack { NamedCodeForm(title: "New site", path: "/api/v1/sites", includeAddress: true) } }
        .errorAlert($data.error)
    }
    private func reload() async { await data.load { try await model.client.list("/api/v1/sites") } }
}

struct StationsView: View {
    @Environment(AppModel.self) private var model
    @State private var data = RemoteData<[Station]>()
    @State private var creating = false
    var body: some View {
        simpleCatalog(title: "Stations", rows: data.value, loading: data.loading, problem: data.error, empty: "No stations") { station in
            VStack(alignment: .leading) { Text(station.code).font(.headline); Text(station.name).foregroundStyle(.secondary) }
        }
        .toolbar { createButton("New station", allowed: .supervisor) { creating = true } }
        .refreshable { await reload() }
        .task { await reload() }
        .sheet(isPresented: $creating, onDismiss: { Task { await reload() } }) { NavigationStack { StationForm() } }
        .errorAlert($data.error)
    }
    private func reload() async { await data.load { try await model.client.list("/api/v1/stations") } }
}

struct StationForm: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @State private var sites: [Site] = []
    @State private var siteId: UUID?
    @State private var code = ""
    @State private var name = ""
    @State private var description = ""
    @State private var errorMessage: String?
    var body: some View {
        Form {
            Picker("Site", selection: $siteId) {
                Text("Select").tag(Optional<UUID>.none)
                ForEach(sites) { Text($0.name).tag(Optional($0.id)) }
            }
            TextField("Code", text: $code)
            TextField("Name", text: $name)
            TextField("Description", text: $description)
        }
        .navigationTitle("New station")
        .toolbar {
            ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
            ToolbarItem(placement: .confirmationAction) { Button("Save") { Task { await save() } }.disabled(siteId == nil || code.isEmpty || name.isEmpty) }
        }
        .task { sites = (try? await model.client.list("/api/v1/sites", query: Query.compact(["active": "true"]))) ?? [] }
        .errorAlert($errorMessage)
    }
    private func save() async {
        guard let siteId else { return }
        do {
            let _: Station = try await model.client.create("/api/v1/stations", body: ["site_id": siteId.uuidString, "code": code, "name": name, "description": description])
            dismiss()
        } catch { errorMessage = error.localizedDescription }
    }
}

struct SuppliersView: View {
    @Environment(AppModel.self) private var model
    @State private var data = RemoteData<[Supplier]>()
    @State private var creating = false
    var body: some View {
        simpleCatalog(title: "Suppliers", rows: data.value, loading: data.loading, problem: data.error, empty: "No suppliers") { supplier in
            VStack(alignment: .leading) { Text(supplier.code).font(.headline); Text(supplier.name).foregroundStyle(.secondary) }
        }
        .toolbar { createButton("New supplier", allowed: .qualityManager) { creating = true } }
        .refreshable { await reload() }
        .task { await reload() }
        .sheet(isPresented: $creating, onDismiss: { Task { await reload() } }) { NavigationStack { SupplierForm() } }
        .errorAlert($data.error)
    }
    private func reload() async { await data.load { try await model.client.list("/api/v1/suppliers") } }
}

struct SupplierForm: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @State private var code = ""
    @State private var name = ""
    @State private var contact = ""
    @State private var email = ""
    @State private var phone = ""
    @State private var errorMessage: String?
    var body: some View {
        Form {
            TextField("Code", text: $code)
            TextField("Name", text: $name)
            TextField("Contact", text: $contact)
            TextField("Email", text: $email).textInputAutocapitalization(.never)
            TextField("Phone", text: $phone)
        }
        .navigationTitle("New supplier")
        .toolbar {
            ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
            ToolbarItem(placement: .confirmationAction) { Button("Save") { Task { await save() } }.disabled(code.isEmpty || name.isEmpty) }
        }
        .errorAlert($errorMessage)
    }
    private func save() async {
        do {
            let _: Supplier = try await model.client.create("/api/v1/suppliers", body: ["code": code, "name": name, "contact_name": contact, "email": email, "phone": phone])
            dismiss()
        } catch { errorMessage = error.localizedDescription }
    }
}

struct GaugesView: View {
    @Environment(AppModel.self) private var model
    @State private var data = RemoteData<[Gauge]>()
    @State private var creating = false
    var body: some View {
        simpleCatalog(title: "Gauges", rows: data.value, loading: data.loading, problem: data.error, empty: "No gauges") { gauge in
            VStack(alignment: .leading) {
                Text(gauge.code).font(.headline)
                Text(gauge.calibrationDue.isEmpty ? gauge.name : "\(gauge.name) · due \(gauge.calibrationDue)").foregroundStyle(.secondary)
            }
        }
        .toolbar { createButton("New gauge", allowed: .qualityManager) { creating = true } }
        .refreshable { await reload() }
        .task { await reload() }
        .sheet(isPresented: $creating, onDismiss: { Task { await reload() } }) { NavigationStack { GaugeForm() } }
        .errorAlert($data.error)
    }
    private func reload() async { await data.load { try await model.client.list("/api/v1/gauges") } }
}

struct GaugeForm: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @State private var code = ""
    @State private var name = ""
    @State private var serial = ""
    @State private var due = ""
    @State private var errorMessage: String?
    var body: some View {
        Form {
            TextField("Code", text: $code)
            TextField("Name", text: $name)
            TextField("Serial number", text: $serial)
            TextField("Calibration due YYYY-MM-DD", text: $due)
        }
        .navigationTitle("New gauge")
        .toolbar {
            ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
            ToolbarItem(placement: .confirmationAction) { Button("Save") { Task { await save() } }.disabled(code.isEmpty || name.isEmpty) }
        }
        .errorAlert($errorMessage)
    }
    private func save() async {
        do {
            let _: Gauge = try await model.client.create("/api/v1/gauges", body: ["code": code, "name": name, "serial_number": serial, "calibration_due": due])
            dismiss()
        } catch { errorMessage = error.localizedDescription }
    }
}

struct DefectCodesView: View {
    @Environment(AppModel.self) private var model
    @State private var data = RemoteData<[DefectCode]>()
    @State private var creating = false
    var body: some View {
        simpleCatalog(title: "Defect codes", rows: data.value, loading: data.loading, problem: data.error, empty: "No defect codes") { defect in
            VStack(alignment: .leading) {
                Text(defect.code).font(.headline)
                Text("\(defect.name) · \(displayName(defect.severity))").foregroundStyle(.secondary)
            }
        }
        .toolbar { createButton("New defect", allowed: .qualityManager) { creating = true } }
        .refreshable { await reload() }
        .task { await reload() }
        .sheet(isPresented: $creating, onDismiss: { Task { await reload() } }) { NavigationStack { DefectForm() } }
        .errorAlert($data.error)
    }
    private func reload() async { await data.load { try await model.client.list("/api/v1/defect-codes") } }
}

struct DefectForm: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @State private var code = ""
    @State private var name = ""
    @State private var severity = "minor"
    @State private var category = ""
    @State private var errorMessage: String?
    var body: some View {
        Form {
            TextField("Code", text: $code)
            TextField("Name", text: $name)
            Picker("Severity", selection: $severity) {
                ForEach(model.enums.severities, id: \.self) { Text(displayName($0)).tag($0) }
            }
            TextField("Category", text: $category)
        }
        .navigationTitle("New defect code")
        .toolbar {
            ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
            ToolbarItem(placement: .confirmationAction) { Button("Save") { Task { await save() } }.disabled(code.isEmpty || name.isEmpty) }
        }
        .errorAlert($errorMessage)
    }
    private func save() async {
        do {
            let _: DefectCode = try await model.client.create("/api/v1/defect-codes", body: ["code": code, "name": name, "severity": severity, "category": category])
            dismiss()
        } catch { errorMessage = error.localizedDescription }
    }
}

struct NamedCodeForm: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    var title: String
    var path: String
    var includeAddress = false
    @State private var code = ""
    @State private var name = ""
    @State private var address = ""
    @State private var errorMessage: String?
    var body: some View {
        Form {
            TextField("Code", text: $code)
            TextField("Name", text: $name)
            if includeAddress { TextField("Address", text: $address) }
        }
        .navigationTitle(title)
        .toolbar {
            ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
            ToolbarItem(placement: .confirmationAction) { Button("Save") { Task { await save() } }.disabled(code.isEmpty || name.isEmpty) }
        }
        .errorAlert($errorMessage)
    }
    private func save() async {
        do {
            let _: Site = try await model.client.create(path, body: ["code": code, "name": name, "address": address])
            dismiss()
        } catch { errorMessage = error.localizedDescription }
    }
}

private struct CatalogList<Item: Identifiable, Row: View>: View {
    var title: String
    var rows: [Item]?
    var empty: String
    var loading: Bool = false
    var problem: String?
    @ViewBuilder var row: (Item) -> Row
    var body: some View {
        Group {
            if let rows {
                List(rows) { item in row(item) }
                    .overlay {
                        if rows.isEmpty {
                            ContentUnavailableView(empty, systemImage: "tray", description: Text("Pull to refresh after you add one."))
                        }
                    }
            } else if loading {
                ProgressView("Loading \(title.lowercased())")
            } else {
                ContentUnavailableView("Could not load \(title.lowercased())", systemImage: "wifi.exclamationmark", description: Text(problem ?? "Pull to refresh."))
            }
        }
        .navigationTitle(title)
    }
}

extension View {
    func simpleCatalog<Item: Identifiable, Row: View>(title: String, rows: [Item]?, loading: Bool = false, problem: String? = nil, empty: String, @ViewBuilder row: @escaping (Item) -> Row) -> some View {
        CatalogList(title: title, rows: rows, empty: empty, loading: loading, problem: problem, row: row)
    }

    @ViewBuilder
    func createButton(_ title: String, allowed: Role, action: @escaping () -> Void) -> some View {
        CreateButton(title: title, allowed: allowed, action: action)
    }
}

private struct CreateButton: View {
    @Environment(AppModel.self) private var model
    var title: String
    var allowed: Role
    var action: () -> Void
    var body: some View {
        if model.user?.allows(allowed) == true {
            Button(title, systemImage: "plus", action: action)
        }
    }
}
