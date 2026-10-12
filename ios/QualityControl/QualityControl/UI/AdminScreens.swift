import SwiftUI

struct UsersView: View {
    @Environment(AppModel.self) private var model
    @State private var data = RemoteData<[User]>()
    @State private var creating = false

    var body: some View {
        Group {
            if let users = data.value {
                List(users) { user in
                    NavigationLink(value: user.id) {
                        VStack(alignment: .leading) {
                            Text(user.fullName).font(.headline)
                            Text("\(user.username) · \(user.roleValue.title)").font(.subheadline).foregroundStyle(.secondary)
                        }
                        if !user.active { Text("Inactive").font(.caption) }
                    }
                }
            } else if data.loading { ProgressView() }
        }
        .qcListChrome()
        .navigationTitle("Users")
        .navigationDestination(for: UUID.self) { UserDetailView(id: $0) }
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Button("New user", systemImage: "plus") { creating = true }
            }
        }
        .refreshable { await reload() }
        .task { await reload() }
        .sheet(isPresented: $creating, onDismiss: { Task { await reload() } }) { NavigationStack { UserForm() } }
        .errorAlert($data.error)
    }

    private func reload() async {
        await data.load { try await model.client.list("/api/v1/users") }
    }
}

struct UserForm: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @State private var username = ""
    @State private var email = ""
    @State private var fullName = ""
    @State private var password = ""
    @State private var role = "inspector"
    @State private var errorMessage: String?

    var body: some View {
        Form {
            TextField("Username", text: $username).textInputAutocapitalization(.never)
            TextField("Full name", text: $fullName)
            TextField("Email", text: $email).textInputAutocapitalization(.never)
            SecureField("Password", text: $password)
            Picker("Role", selection: $role) {
                ForEach(model.enums.roles, id: \.self) { Text(Role.named($0)?.title ?? $0).tag($0) }
            }
        }
        .navigationTitle("New user")
        .toolbar {
            ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
            ToolbarItem(placement: .confirmationAction) {
                Button("Save") { Task { await save() } }
                    .disabled(username.count < 3 || fullName.isEmpty || password.count < 10 || !email.contains("@"))
            }
        }
        .errorAlert($errorMessage)
    }

    private func save() async {
        do {
            let _: User = try await model.client.create("/api/v1/users", body: [
                "username": username, "email": email, "full_name": fullName, "password": password, "role": role
            ])
            dismiss()
        } catch { errorMessage = error.localizedDescription }
    }
}

struct UserDetailView: View {
    @Environment(AppModel.self) private var model
    var id: UUID
    @State private var person: User?
    @State private var role = "viewer"
    @State private var password = ""
    @State private var errorMessage: String?

    var body: some View {
        Form {
            if let person {
                LabeledContent("Username", value: person.username)
                LabeledContent("Email", value: person.email)
                LabeledContent("Name", value: person.fullName)
                Picker("Role", selection: $role) {
                    ForEach(model.enums.roles, id: \.self) { Text(Role.named($0)?.title ?? $0).tag($0) }
                }
                Button("Save role") { Task { await setActive(person.active) } }
                Toggle("Active", isOn: Binding(get: { person.active }, set: { newValue in
                    Task { await setActive(newValue) }
                }))
                .disabled(person.id == model.user?.id)
                SecureField("New password", text: $password)
                Button("Reset password") { Task { await resetPassword() } }
                    .disabled(password.count < 10)
            } else { ProgressView() }
        }
        .navigationTitle(person?.username ?? "User")
        .task { await reload() }
        .errorAlert($errorMessage)
    }

    private func reload() async {
        do {
            person = try await model.client.one("/api/v1/users/\(id.uuidString)")
            role = person?.role ?? role
        } catch { errorMessage = error.localizedDescription }
    }

    private func setActive(_ active: Bool) async {
        do {
            person = try await model.client.update("/api/v1/users/\(id.uuidString)", body: ["role": role, "active": active])
        } catch { errorMessage = error.localizedDescription }
    }

    private func resetPassword() async {
        do {
            try await model.client.execute(path: "/api/v1/users/\(id.uuidString)/password", body: ["password": password])
            password = ""
        } catch { errorMessage = error.localizedDescription }
    }
}

struct AuditView: View {
    @Environment(AppModel.self) private var model
    @State private var data = RemoteData<[AuditEntry]>()

    var body: some View {
        Group {
            if let rows = data.value {
                List(rows) { entry in
                    VStack(alignment: .leading, spacing: 4) {
                        Text(entry.summary.isEmpty ? entry.action : entry.summary)
                        Text("\(entry.action) · \(entry.createdAt.formatted(date: .abbreviated, time: .shortened))")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
            } else if data.loading { ProgressView() }
        }
        .qcListChrome()
        .navigationTitle("Audit log")
        .refreshable { await reload() }
        .task { await reload() }
        .errorAlert($data.error)
    }

    private func reload() async {
        await data.load { try await model.client.list("/api/v1/audit") }
    }
}

struct MeasurementsView: View {
    @Environment(AppModel.self) private var model
    @State private var products: [Product] = []
    @State private var productId: UUID?
    @State private var code = ""
    @State private var rows: [Measurement] = []
    @State private var errorMessage: String?
    @State private var loading = false

    var body: some View {
        VStack {
            Form {
                Picker("Product", selection: $productId) {
                    Text("Select").tag(Optional<UUID>.none)
                    ForEach(products) { Text("\($0.partNumber)").tag(Optional($0.id)) }
                }
                TextField("Characteristic code", text: $code)
                Button("Load measurements") { Task { await load() } }
                    .disabled(productId == nil && code.trimmingCharacters(in: .whitespaces).isEmpty)
            }
            .frame(maxHeight: 220)
            if loading { ProgressView() }
            List(rows) { point in
                VStack(alignment: .leading) {
                    Text("\(point.characteristicCode) · \(formatNumber(point.measuredValue)) \(point.unit)")
                        .font(.headline)
                    Text("\(point.lotNumber) · \(point.inspectionNumber) · sample \(point.sampleIndex)")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                StatusBadge(text: point.inSpec ? "pass" : "fail")
            }
        }
        .qcListChrome()
        .navigationTitle("Measurements")
        .task { products = (try? await model.client.list("/api/v1/products")) ?? [] }
        .errorAlert($errorMessage)
    }

    private func load() async {
        loading = true
        defer { loading = false }
        var query: [String: String] = ["limit": "500"]
        if let productId { query["product_id"] = productId.uuidString }
        if !code.trimmingCharacters(in: .whitespaces).isEmpty { query["characteristic_code"] = code }
        do {
            rows = try await model.client.list("/api/v1/measurements", query: query)
        } catch { errorMessage = error.localizedDescription }
    }
}
