import SwiftUI

enum AppSection: String, CaseIterable, Identifiable {
    case dashboard
    case lots
    case inspections
    case nonconformances
    case correctiveActions
    case measurements
    case products
    case plans
    case sites
    case stations
    case suppliers
    case gauges
    case defects
    case users
    case audit
    case tools
    case account

    var id: String { rawValue }

    var title: String {
        switch self {
        case .dashboard: "Dashboard"
        case .lots: "Lots"
        case .inspections: "Inspections"
        case .nonconformances: "Nonconformances"
        case .correctiveActions: "Corrective actions"
        case .measurements: "Measurements"
        case .products: "Products"
        case .plans: "Plans"
        case .sites: "Sites"
        case .stations: "Stations"
        case .suppliers: "Suppliers"
        case .gauges: "Gauges"
        case .defects: "Defect codes"
        case .users: "Users"
        case .audit: "Audit log"
        case .tools: "Tester tools"
        case .account: "Account"
        }
    }

    var symbol: String {
        switch self {
        case .dashboard: "square.grid.2x2"
        case .lots: "shippingbox"
        case .inspections: "checklist"
        case .nonconformances: "exclamationmark.triangle"
        case .correctiveActions: "wrench.and.screwdriver"
        case .measurements: "chart.xyaxis.line"
        case .products: "cube"
        case .plans: "list.bullet.rectangle"
        case .sites: "building.2"
        case .stations: "wrench.adjustable"
        case .suppliers: "truck.box"
        case .gauges: "ruler"
        case .defects: "tag"
        case .users: "person.2"
        case .audit: "clock.arrow.circlepath"
        case .tools: "wrench.and.screwdriver.fill"
        case .account: "person.crop.circle"
        }
    }

    func isVisible(to user: User) -> Bool {
        switch self {
        case .users: user.allows(.admin)
        case .audit: user.allows(.supervisor)
        default: true
        }
    }
}

struct RootView: View {
    @Environment(AppModel.self) private var model
    @State private var section: AppSection? = .dashboard

    var body: some View {
        if model.isSignedIn {
            NavigationSplitView {
                List(selection: $section) {
                    if let user = model.user {
                        Section {
                            Label(user.fullName, systemImage: "person.fill")
                            Text(user.roleValue.title)
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                    }
                    Section("Floor") {
                        sidebar(.dashboard, .lots, .inspections, .nonconformances, .correctiveActions, .measurements)
                    }
                    Section("Master data") {
                        sidebar(.products, .plans, .sites, .stations, .suppliers, .gauges, .defects)
                    }
                    Section {
                        sidebar(.users, .audit, .tools, .account)
                    }
                }
                .navigationTitle("Quality")
            } detail: {
                NavigationStack {
                    if let section {
                        section.destination()
                    } else {
                        ContentUnavailableView("Select a section", systemImage: "sidebar.left")
                    }
                }
            }
            .navigationSplitViewStyle(.balanced)
            .safeAreaInset(edge: .top) {
                if let notice = model.notice {
                    HStack {
                        Image(systemName: "info.circle.fill")
                        Text(notice)
                            .font(.subheadline)
                            .frame(maxWidth: .infinity, alignment: .leading)
                        Button("Dismiss") { model.notice = nil }
                    }
                    .padding(12)
                    .background(.thinMaterial)
                }
            }
            .task {
                await model.loadEnums()
                await model.refreshProfile()
            }
            .onChange(of: model.user?.role) {
                if let section, let user = model.user, !section.isVisible(to: user) {
                    self.section = .dashboard
                }
            }
        } else {
            LoginView()
        }
    }

    @ViewBuilder
    private func sidebar(_ sections: AppSection...) -> some View {
        if let user = model.user {
            ForEach(sections.filter { $0.isVisible(to: user) }) { item in
                Label(item.title, systemImage: item.symbol)
                    .tag(item)
            }
        }
    }
}

struct LoginView: View {
    @Environment(AppModel.self) private var model
    @State private var username = ""
    @State private var password = ""
    @State private var errorMessage: String?
    @State private var connectionNote: String?
    @State private var working = false
    @State private var checking = false

    var body: some View {
        @Bindable var model = model
        NavigationStack {
            Form {
                Section("API server") {
                    TextField("http://127.0.0.1:8080", text: $model.serverAddress)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .keyboardType(.URL)
                }
                Section("Account") {
                    TextField("Username", text: $username)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                    SecureField("Password", text: $password)
                }
                Section {
                    Button {
                        Task { await checkConnection() }
                    } label: {
                        HStack {
                            Text("Check connection")
                            Spacer()
                            if checking { ProgressView() }
                        }
                    }
                    .disabled(checking || working || model.serverAddress.isEmpty)
                    if let connectionNote {
                        Text(connectionNote)
                            .font(.footnote)
                            .foregroundStyle(connectionNote.hasPrefix("The API answered") ? Color.green : Color.orange)
                    }
                    Button {
                        Task { await signIn() }
                    } label: {
                        HStack {
                            Text("Sign in")
                            Spacer()
                            if working { ProgressView() }
                        }
                    }
                    .disabled(working || checking || username.isEmpty || password.isEmpty || model.serverAddress.isEmpty)
                } footer: {
                    Text("Simulator: http://127.0.0.1:8080. A TestFlight iPhone must use this Mac’s Wi-Fi address, such as http://192.168.1.20:8080. Sign in as admin / change-me-now. Port 5433 is the database.")
                }
            }
            .frame(maxWidth: 560)
            .frame(maxWidth: .infinity)
            .navigationTitle("Quality Control")
            .errorAlert($errorMessage)
            .onAppear {
                if username.isEmpty { username = model.lastUsername }
                if let notice = model.notice { errorMessage = notice }
            }
        }
    }

    private func checkConnection() async {
        checking = true
        defer { checking = false }
        connectionNote = await model.probe()
    }

    private func signIn() async {
        working = true
        defer { working = false }
        do {
            try await model.login(username: username, password: password)
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}

struct DashboardView: View {
    @Environment(AppModel.self) private var model
    @State private var data = RemoteData<Dashboard>()

    var body: some View {
        ScrollView {
            if let dashboard = data.value {
                LazyVGrid(columns: [GridItem(.adaptive(minimum: 160), spacing: 12)], spacing: 12) {
                    MetricCard(title: "Open inspections", value: "\(dashboard.openInspections)", systemImage: "checklist")
                    MetricCard(title: "Lots on hold", value: "\(dashboard.lotsOnHold)", systemImage: "pause.circle")
                    MetricCard(title: "Open NCRs", value: "\(dashboard.openNonconformances)", systemImage: "exclamationmark.triangle")
                    MetricCard(title: "Open actions", value: "\(dashboard.openCorrectiveActions)", systemImage: "wrench")
                    MetricCard(title: "Overdue actions", value: "\(dashboard.overdueCorrectiveActions)", systemImage: "calendar.badge.exclamationmark")
                    MetricCard(title: "Pass rate, 30 days", value: rate(dashboard.passRateLast30Days), systemImage: "percent")
                }
                .padding()
                VStack(alignment: .leading, spacing: 8) {
                    Text("Last 30 days")
                        .font(.headline)
                    LabeledContent("Inspections", value: "\(dashboard.inspectionsLast30Days)")
                    LabeledContent("Pass", value: "\(dashboard.passLast30Days)")
                    LabeledContent("Conditional", value: "\(dashboard.conditionalLast30Days)")
                    LabeledContent("Fail", value: "\(dashboard.failLast30Days)")
                    ForEach(dashboard.defectsBySeverity, id: \.severity) { item in
                        LabeledContent(displayName(item.severity) + " defects", value: "\(item.count)")
                    }
                }
                .padding()
                .frame(maxWidth: .infinity, alignment: .leading)
            } else if data.loading {
                ProgressView("Loading the plant")
                    .padding()
            } else {
                ContentUnavailableView("Dashboard unavailable", systemImage: "wifi.exclamationmark", description: Text(data.error ?? "Pull to refresh."))
                    .padding()
            }
        }
        .navigationTitle("Dashboard")
        .refreshable { await reload() }
        .task { await reload() }
        .errorAlert($data.error)
    }

    private func reload() async {
        await data.load { try await model.client.one("/api/v1/dashboard") }
    }

    private func rate(_ value: Double?) -> String {
        guard let value else { return "—" }
        return (value * 100).formatted(.number.precision(.fractionLength(1))) + "%"
    }
}

struct AccountView: View {
    @Environment(AppModel.self) private var model
    @State private var currentPassword = ""
    @State private var newPassword = ""
    @State private var errorMessage: String?
    @State private var saved = false
    @State private var confirmSignOut = false

    var body: some View {
        Form {
            if let user = model.user {
                LabeledContent("Name", value: user.fullName)
                LabeledContent("Username", value: user.username)
                LabeledContent("Email", value: user.email)
                LabeledContent("Role", value: user.roleValue.title)
                LabeledContent("Server", value: model.serverAddress)
            }
            Section("Change password") {
                SecureField("Current password", text: $currentPassword)
                SecureField("New password", text: $newPassword)
                Button("Update password") {
                    Task { await changePassword() }
                }
                .disabled(currentPassword.isEmpty || newPassword.count < 10)
            }
            Section {
                Button("Sign out", role: .destructive) { confirmSignOut = true }
            }
            .confirmationDialog("Sign out of this device?", isPresented: $confirmSignOut, titleVisibility: .visible) {
                Button("Sign out", role: .destructive) { Task { await model.logout() } }
                Button("Stay signed in", role: .cancel) {}
            }
        }
        .navigationTitle("Account")
        .errorAlert($errorMessage)
        .alert("Password updated", isPresented: $saved) {
            Button("OK", role: .cancel) {}
        }
    }

    private func changePassword() async {
        do {
            try await model.client.execute(path: "/api/v1/me/password", body: [
                "current_password": currentPassword,
                "new_password": newPassword
            ])
            currentPassword = ""
            newPassword = ""
            saved = true
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}
