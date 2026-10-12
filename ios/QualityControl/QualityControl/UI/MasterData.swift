import SwiftUI

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
                            Text("\(plan.name) · \(plan.revision)").font(QCTheme.headlineFont)
                            Text(displayName(plan.samplingType)).font(.subheadline).foregroundStyle(QCTheme.mutedInk)
                        }
                        StatusBadge(text: plan.status)
                    }
                    .accessibilityLabel("Plan \(plan.name) revision \(plan.revision), \(displayName(plan.status))")
                }
                .qcListChrome()
                .overlay {
                    if plans.isEmpty {
                        EmptyStateView(title: "No plans", systemImage: "list.bullet.rectangle", description: "Create an inspection plan for a product.")
                    }
                }
            } else if data.loading {
                LoadingStateView(message: "Loading plans")
            } else {
                ErrorStateView(title: "Could not load plans", message: data.error ?? "Pull to refresh.", retry: { Task { await reload() } })
            }
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
    @State private var editingCheck: Characteristic?
    @State private var editingPlan = false

    var body: some View {
        Group {
            if let detail {
                Form {
                    LabeledContent("Name", value: detail.plan.name)
                    LabeledContent("Revision", value: detail.plan.revision)
                    LabeledContent("Sampling", value: displayName(detail.plan.samplingType))
                    StatusBadge(text: detail.plan.status)
                    Section {
                        ForEach(detail.characteristics) { characteristic in
                            Button {
                                if canEditChecks(detail) { editingCheck = characteristic }
                            } label: {
                                VStack(alignment: .leading) {
                                    Text("\(characteristic.sequence). \(characteristic.code) · \(characteristic.name)")
                                        .foregroundStyle(QCTheme.ink)
                                    Text(characteristic.characteristicType == "variable" ? limitText(characteristic) : "Pass or fail")
                                        .font(.caption)
                                        .foregroundStyle(.secondary)
                                    if characteristic.critical { Text("Critical").font(.caption).foregroundStyle(QCTheme.danger) }
                                }
                            }
                            .swipeActions {
                                if detail.plan.status == "draft", model.user?.allows(.qualityManager) == true {
                                    Button("Remove", role: .destructive) { Task { await removeCheck(characteristic) } }
                                }
                            }
                        }
                    } header: {
                        Text("Checks")
                    } footer: {
                        if canEditChecks(detail) {
                            Text(detail.plan.status == "draft" ? "Tap a check to edit it, swipe to remove it." : "As admin you can fix a check's name or limits on an active plan.")
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
        .navigationTitle(detail.map { "\($0.plan.name) rev \($0.plan.revision)" } ?? "Plan")
        .toolbar {
            if let detail, model.user?.allows(.qualityManager) == true,
               detail.plan.status == "draft" || model.user?.allows(.admin) == true {
                ToolbarItem(placement: .primaryAction) {
                    Button("Edit plan", systemImage: "pencil") { editingPlan = true }
                }
            }
        }
        .task { await reload() }
        .refreshable { await reload() }
        .sheet(isPresented: $adding, onDismiss: { Task { await reload() } }) {
            NavigationStack { CharacteristicForm(planId: id, nextSequence: (detail?.characteristics.map(\.sequence).max() ?? 0) + 1) }
        }
        .sheet(item: $editingCheck, onDismiss: { Task { await reload() } }) { check in
            NavigationStack { CharacteristicForm(planId: id, nextSequence: check.sequence, existing: check, locked: detail?.plan.status != "draft") }
        }
        .sheet(isPresented: $editingPlan, onDismiss: { Task { await reload() } }) {
            NavigationStack {
                RecordEditorView(kind: .plans, id: id)
                    .toolbar { ToolbarItem(placement: .cancellationAction) { Button("Done") { editingPlan = false } } }
            }
        }
        .errorAlert($errorMessage)
    }

    private func canEditChecks(_ detail: PlanDetail) -> Bool {
        guard model.user?.allows(.qualityManager) == true else { return false }
        return detail.plan.status == "draft" || (detail.plan.status == "active" && model.user?.allows(.admin) == true)
    }

    private func removeCheck(_ check: Characteristic) async {
        do {
            try await model.client.delete("/api/v1/characteristics/\(check.id.uuidString)")
            await reload()
        } catch { errorMessage = error.localizedDescription }
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
    /// The check being edited; nil adds a new one.
    var existing: Characteristic?
    /// On an active plan the kind of check can't change.
    var locked = false
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
                .textInputAutocapitalization(.characters)
            TextField("Name", text: $name)
            Picker("Type", selection: $kind) {
                ForEach(model.enums.characteristicTypes, id: \.self) { Text(displayName($0)).tag($0) }
            }
            .disabled(locked)
            if kind == "variable" {
                TextField("Nominal", text: $nominal).keyboardType(.decimalPad)
                TextField("Lower", text: $lower).keyboardType(.decimalPad)
                TextField("Upper", text: $upper).keyboardType(.decimalPad)
                TextField("Unit", text: $unit)
            }
            Toggle("Critical", isOn: $critical)
        }
        .navigationTitle(existing == nil ? "New check" : "Edit check")
        .toolbar {
            ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
            ToolbarItem(placement: .confirmationAction) { Button("Save") { Task { await save() } }.disabled(code.isEmpty || name.isEmpty) }
        }
        .onAppear(perform: fill)
        .errorAlert($errorMessage)
    }

    private func fill() {
        guard let existing, code.isEmpty else { return }
        code = existing.code
        name = existing.name
        kind = existing.characteristicType
        nominal = existing.nominal.map(plainNumber) ?? ""
        lower = existing.lowerLimit.map(plainNumber) ?? ""
        upper = existing.upperLimit.map(plainNumber) ?? ""
        unit = existing.unit
        critical = existing.critical
    }

    private func save() async {
        var body: [String: Any] = [
            "sequence": nextSequence, "code": code, "name": name, "characteristic_type": kind, "unit": unit, "critical": critical
        ]
        for (key, text) in [("nominal", nominal), ("lower_limit", lower), ("upper_limit", upper)] {
            if let value = parseNumber(text) {
                body[key] = value
            } else if existing != nil {
                body["clear_" + (key == "nominal" ? "nominal" : key)] = true
            }
        }
        do {
            if let existing {
                let _: Characteristic = try await model.client.update("/api/v1/characteristics/\(existing.id.uuidString)", body: body)
            } else {
                let _: Characteristic = try await model.client.create("/api/v1/plans/\(planId.uuidString)/characteristics", body: body)
            }
            dismiss()
        } catch { errorMessage = error.localizedDescription }
    }
}

extension View {
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
