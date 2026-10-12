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
    case settings
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
        case .settings: "Settings"
        case .account: "Account"
        }
    }

    var symbol: String {
        switch self {
        case .dashboard: "square.grid.2x2.fill"
        case .lots: "shippingbox.fill"
        case .inspections: "checklist"
        case .nonconformances: "exclamationmark.triangle.fill"
        case .correctiveActions: "wrench.and.screwdriver.fill"
        case .measurements: "chart.xyaxis.line"
        case .products: "cube.fill"
        case .plans: "list.bullet.rectangle.fill"
        case .sites: "building.2.fill"
        case .stations: "wrench.adjustable.fill"
        case .suppliers: "truck.box.fill"
        case .gauges: "ruler.fill"
        case .defects: "tag.fill"
        case .users: "person.2.fill"
        case .audit: "clock.arrow.circlepath"
        case .tools: "hammer.fill"
        case .settings: "gearshape.fill"
        case .account: "person.crop.circle.fill"
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
    @State private var columnVisibility: NavigationSplitViewVisibility = .automatic

    var body: some View {
        Group {
            if model.isSignedIn {
                NavigationSplitView(columnVisibility: $columnVisibility) {
                    List(selection: $section) {
                        if let user = model.user {
                            Section {
                                HStack(spacing: 12) {
                                    ZStack {
                                        Circle()
                                            .fill(QCTheme.brandColor.opacity(0.18))
                                            .frame(width: 44, height: 44)
                                        Image(systemName: "person.fill")
                                            .foregroundStyle(QCTheme.brandColor)
                                    }
                                    VStack(alignment: .leading, spacing: 2) {
                                        Text(user.fullName)
                                            .font(QCTheme.headlineFont)
                                            .foregroundStyle(QCTheme.ink)
                                        Text(user.roleValue.title)
                                            .font(QCTheme.captionFont)
                                            .foregroundStyle(QCTheme.mutedInk)
                                    }
                                }
                                .padding(.vertical, 4)
                                .accessibilityElement(children: .combine)
                                .accessibilityLabel("Signed in as \(user.fullName), \(user.roleValue.title)")
                            }
                        }
                        Section("Floor") {
                            sidebar(.dashboard, .lots, .inspections, .nonconformances, .correctiveActions, .measurements)
                        }
                        Section("Master data") {
                            sidebar(.products, .plans, .sites, .stations, .suppliers, .gauges, .defects)
                        }
                        Section("System") {
                            sidebar(.users, .audit, .tools, .settings, .account)
                        }
                    }
                    .listStyle(.sidebar)
                    .scrollContentBackground(.hidden)
                    .background(QCTheme.canvas)
                    .navigationTitle("Quality")
                    #if os(iOS)
                    .navigationBarTitleDisplayMode(.large)
                    #endif
                } detail: {
                    NavigationStack {
                        if let section {
                            section.destination()
                        } else {
                            EmptyStateView(
                                title: "Select a section",
                                systemImage: "sidebar.left",
                                description: "Choose a workspace from the sidebar."
                            )
                        }
                    }
                }
                .navigationSplitViewStyle(.balanced)
                .safeAreaInset(edge: .top) {
                    VStack(spacing: 0) {
                        OfflineBanner()
                        if let notice = model.notice {
                            HStack {
                                Image(systemName: "info.circle.fill")
                                    .foregroundStyle(QCTheme.info)
                                Text(notice)
                                    .font(.subheadline)
                                    .frame(maxWidth: .infinity, alignment: .leading)
                                Button("Dismiss") { model.notice = nil }
                            }
                            .padding(12)
                            .background(.thinMaterial)
                            .accessibilityElement(children: .combine)
                        }
                    }
                }
                .task {
                    await model.loadEnums()
                    await model.refreshProfile()
                    await model.syncPending()
                }
                .onChange(of: model.user?.role) {
                    if let section, let user = model.user, !section.isVisible(to: user) {
                        self.section = .dashboard
                    }
                }
                .onChange(of: model.offline.isOnline) { _, online in
                    if online {
                        Task { await model.syncPending() }
                    }
                }
            } else {
                LoginView()
            }
        }
        .animation(.easeInOut(duration: 0.25), value: model.isSignedIn)
    }

    @ViewBuilder
    private func sidebar(_ sections: AppSection...) -> some View {
        if let user = model.user {
            ForEach(sections.filter { $0.isVisible(to: user) }) { item in
                Label(item.title, systemImage: item.symbol)
                    .tag(item)
                    .accessibilityLabel(item.title)
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
    @State private var appear = false

    var body: some View {
        @Bindable var model = model
        ZStack {
            QCBackground()
            ScrollView {
                VStack(spacing: 28) {
                    VStack(spacing: 10) {
                        Image(systemName: "checkmark.seal.fill")
                            .font(.system(size: 52))
                            .foregroundStyle(QCTheme.brandColor)
                            .symbolEffect(.bounce, value: appear)
                            .accessibilityHidden(true)
                        Text("Quality Control")
                            .font(QCTheme.displayFont)
                            .foregroundStyle(QCTheme.ink)
                        Text("Floor inspections, NCRs, and CAPA in one place.")
                            .font(.subheadline)
                            .foregroundStyle(QCTheme.mutedInk)
                            .multilineTextAlignment(.center)
                    }
                    .padding(.top, 36)
                    .opacity(appear ? 1 : 0)
                    .offset(y: appear ? 0 : 12)

                    VStack(spacing: 16) {
                        VStack(alignment: .leading, spacing: 8) {
                            Text("API server")
                                .font(QCTheme.captionFont)
                                .foregroundStyle(QCTheme.mutedInk)
                            TextField("http://127.0.0.1:8080", text: $model.serverAddress)
                                .textInputAutocapitalization(.never)
                                .autocorrectionDisabled()
                                .keyboardType(.URL)
                                .padding(12)
                                .background(QCTheme.elevated, in: RoundedRectangle(cornerRadius: 12, style: .continuous))
                                .accessibilityLabel("Server URL")
                        }
                        VStack(alignment: .leading, spacing: 8) {
                            Text("Account")
                                .font(QCTheme.captionFont)
                                .foregroundStyle(QCTheme.mutedInk)
                            TextField("Username", text: $username)
                                .textInputAutocapitalization(.never)
                                .autocorrectionDisabled()
                                .padding(12)
                                .background(QCTheme.elevated, in: RoundedRectangle(cornerRadius: 12, style: .continuous))
                                .accessibilityLabel("Username")
                            SecureField("Password", text: $password)
                                .padding(12)
                                .background(QCTheme.elevated, in: RoundedRectangle(cornerRadius: 12, style: .continuous))
                                .accessibilityLabel("Password")
                        }

                        if let connectionNote {
                            Text(connectionNote)
                                .font(.footnote)
                                .foregroundStyle(connectionNote.hasPrefix("The API answered") ? QCTheme.success : QCTheme.amber)
                                .frame(maxWidth: .infinity, alignment: .leading)
                                .accessibilityLabel(connectionNote)
                        }

                        Button {
                            Task { await checkConnection() }
                        } label: {
                            HStack {
                                Text("Check connection")
                                if checking { ProgressView() }
                            }
                            .frame(maxWidth: .infinity)
                        }
                        .buttonStyle(.bordered)
                        .disabled(checking || working || model.serverAddress.isEmpty)
                        .accessibilityLabel("Check connection to API server")

                        Button {
                            Task { await signIn() }
                        } label: {
                            HStack {
                                Text("Sign in")
                                    .fontWeight(.semibold)
                                if working { ProgressView().tint(.white) }
                            }
                            .frame(maxWidth: .infinity)
                            .padding(.vertical, 4)
                        }
                        .buttonStyle(.borderedProminent)
                        .tint(QCTheme.brandColor)
                        .disabled(working || checking || username.isEmpty || password.isEmpty || model.serverAddress.isEmpty)
                        .accessibilityLabel("Sign in")

                        Text("Simulator: http://127.0.0.1:8080. A TestFlight iPhone must use this Mac’s Wi-Fi address and port 8080.")
                            .font(.caption)
                            .foregroundStyle(QCTheme.mutedInk)
                    }
                    .padding(20)
                    .background(QCTheme.elevated.opacity(0.92), in: RoundedRectangle(cornerRadius: 22, style: .continuous))
                    .overlay(
                        RoundedRectangle(cornerRadius: 22, style: .continuous)
                            .stroke(QCTheme.brandColor.opacity(0.12), lineWidth: 1)
                    )
                    .padding(.horizontal, 20)
                    .frame(maxWidth: 520)
                }
                .frame(maxWidth: .infinity)
                .padding(.bottom, 40)
            }
        }
        .errorAlert($errorMessage)
        .onAppear {
            if username.isEmpty { username = model.lastUsername }
            if let notice = model.notice { errorMessage = notice }
            withAnimation(.spring(response: 0.55, dampingFraction: 0.86)) {
                appear = true
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
    @State private var stats: ClientDashboardStats?
    @State private var serverDashboard: Dashboard?
    @State private var loading = false
    @State private var errorMessage: String?
    @State private var fromCache = false
    @State private var appear = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                header
                if let stats {
                    LazyVGrid(columns: [GridItem(.adaptive(minimum: 150), spacing: 12)], spacing: 12) {
                        MetricCard(title: "Open inspections", value: "\(stats.openInspections)", systemImage: "checklist", tint: QCTheme.info)
                        MetricCard(title: "Lots on hold", value: "\(stats.lotsOnHold)", systemImage: "pause.circle.fill", tint: QCTheme.amber)
                        MetricCard(title: "Open lots", value: "\(stats.openLots)", systemImage: "shippingbox.fill", tint: QCTheme.brandSoft)
                        MetricCard(title: "Open NCRs", value: "\(stats.openNonconformances)", systemImage: "exclamationmark.triangle.fill", tint: QCTheme.danger)
                        MetricCard(title: "Open CAPA", value: "\(stats.openCorrectiveActions)", systemImage: "wrench.and.screwdriver.fill", tint: QCTheme.brandColor)
                        MetricCard(title: "Overdue CAPA", value: "\(stats.overdueCorrectiveActions)", systemImage: "calendar.badge.exclamationmark", tint: QCTheme.amber)
                        MetricCard(title: "Pass rate, 30d", value: rate(stats.passRateLast30Days), systemImage: "percent", tint: QCTheme.success)
                    }
                    .opacity(appear ? 1 : 0)
                    .offset(y: appear ? 0 : 8)

                    VStack(alignment: .leading, spacing: 12) {
                        Text("Last 30 days")
                            .font(QCTheme.titleFont)
                            .foregroundStyle(QCTheme.ink)
                        trendRow("Inspections", "\(stats.inspectionsLast30Days)", QCTheme.brandColor)
                        trendRow("Pass", "\(stats.passLast30Days)", QCTheme.success)
                        trendRow("Conditional", "\(stats.conditionalLast30Days)", QCTheme.amber)
                        trendRow("Fail", "\(stats.failLast30Days)", QCTheme.danger)
                        ForEach(stats.defectsBySeverity, id: \.severity) { item in
                            trendRow("\(displayName(item.severity)) NCRs open", "\(item.count)", QCTheme.statusColor(item.severity))
                        }
                    }
                    .padding(16)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(QCTheme.elevated, in: RoundedRectangle(cornerRadius: 18, style: .continuous))
                    .accessibilityElement(children: .contain)
                } else if loading {
                    LoadingStateView(message: "Loading plant metrics")
                        .frame(minHeight: 240)
                } else {
                    ErrorStateView(
                        title: "Dashboard unavailable",
                        message: errorMessage ?? "Pull to refresh.",
                        retry: { Task { await reload() } }
                    )
                    .frame(minHeight: 240)
                }
            }
            .padding()
        }
        .background(QCBackground())
        .navigationTitle("Dashboard")
        .refreshable { await reload() }
        .task {
            await reload()
            withAnimation(.easeOut(duration: 0.35)) { appear = true }
        }
        .errorAlert($errorMessage)
        .toolbar {
            if fromCache {
                ToolbarItem(placement: .status) {
                    Label("Cached", systemImage: "externaldrive")
                        .font(.caption)
                        .foregroundStyle(QCTheme.mutedInk)
                }
            }
        }
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Plant pulse")
                .font(QCTheme.displayFont)
                .foregroundStyle(QCTheme.ink)
            Text("Metrics computed on device from lots, inspections, NCRs, and CAPA.")
                .font(.subheadline)
                .foregroundStyle(QCTheme.mutedInk)
        }
        .accessibilityElement(children: .combine)
    }

    private func trendRow(_ title: String, _ value: String, _ tint: Color) -> some View {
        HStack {
            Circle()
                .fill(tint)
                .frame(width: 8, height: 8)
            Text(title)
                .foregroundStyle(QCTheme.ink)
            Spacer()
            Text(value)
                .font(QCTheme.headlineFont)
                .foregroundStyle(QCTheme.ink)
                .contentTransition(.numericText())
        }
        .accessibilityElement(children: .combine)
        .accessibilityLabel("\(title): \(value)")
    }

    private func reload() async {
        loading = stats == nil
        defer { loading = false }
        fromCache = false

        async let lotsTask: [Lot] = (try? await model.client.list("/api/v1/lots", query: ["limit": "100"])) ?? []
        async let inspectionsTask: [Inspection] = (try? await model.client.list("/api/v1/inspections", query: ["limit": "100"])) ?? []
        async let ncrsTask: [Nonconformance] = (try? await model.client.list("/api/v1/nonconformances", query: ["limit": "100"])) ?? []
        async let capaTask: [CorrectiveAction] = (try? await model.client.list("/api/v1/corrective-actions", query: ["limit": "100"])) ?? []
        async let dashTask: Dashboard? = try? await model.client.one("/api/v1/dashboard")

        let lots = await lotsTask
        let inspections = await inspectionsTask
        let ncrs = await ncrsTask
        let capas = await capaTask
        let dash = await dashTask

        if lots.isEmpty && inspections.isEmpty && ncrs.isEmpty && capas.isEmpty && dash == nil {
            if let cached: ClientDashboardStats = model.offline.cached(ClientDashboardStats.self, forKey: CacheKey.dashboard) {
                stats = cached
                fromCache = true
                errorMessage = model.offline.isOnline ? nil : "Showing cached dashboard."
                return
            }
            errorMessage = "Could not load plant data."
            return
        }

        serverDashboard = dash
        let client = ClientDashboardStats.compute(
            lots: lots,
            inspections: inspections,
            nonconformances: ncrs,
            correctiveActions: capas
        )
        let merged = ClientDashboardStats.merge(client: client, server: dash)
        stats = merged
        model.offline.cache(merged, forKey: CacheKey.dashboard)
        model.offline.cache(lots, forKey: CacheKey.lots)
        model.offline.cache(inspections, forKey: CacheKey.inspections)
        model.offline.cache(ncrs, forKey: CacheKey.nonconformances)
        model.offline.cache(capas, forKey: CacheKey.correctiveActions)
        errorMessage = nil
        await model.syncPending()
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
                Section("Profile") {
                    LabeledContent("Name", value: user.fullName)
                    LabeledContent("Username", value: user.username)
                    LabeledContent("Email", value: user.email)
                    LabeledContent("Role", value: user.roleValue.title)
                }
            }
            Section("Change password") {
                SecureField("Current password", text: $currentPassword)
                    .accessibilityLabel("Current password")
                SecureField("New password", text: $newPassword)
                    .accessibilityLabel("New password")
                Button("Update password") {
                    Task { await changePassword() }
                }
                .disabled(currentPassword.isEmpty || newPassword.count < 10)
            }
            Section {
                Button("Sign out", role: .destructive) { confirmSignOut = true }
                    .accessibilityLabel("Sign out")
            }
            .confirmationDialog("Sign out of this device?", isPresented: $confirmSignOut, titleVisibility: .visible) {
                Button("Sign out", role: .destructive) { Task { await model.logout() } }
                Button("Stay signed in", role: .cancel) {}
            }
        }
        .qcListChrome()
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

struct SettingsView: View {
    @Environment(AppModel.self) private var model
    @State private var draftServer = ""
    @State private var note: String?
    @State private var errorMessage: String?
    @State private var clearing = false

    var body: some View {
        @Bindable var model = model
        Form {
            Section {
                TextField("Server URL", text: $draftServer)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .keyboardType(.URL)
                    .accessibilityLabel("Server URL")
                Button("Save server URL") {
                    model.serverAddress = draftServer
                    do {
                        try model.applyServerAddress()
                        note = "Server updated to \(model.serverAddress)."
                    } catch {
                        errorMessage = error.localizedDescription
                    }
                }
                .disabled(draftServer.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                Button("Test connection") {
                    Task {
                        model.serverAddress = draftServer
                        note = await model.probe()
                    }
                }
                if let note {
                    Text(note)
                        .font(.footnote)
                        .foregroundStyle(note.contains("answered") || note.hasPrefix("Server updated") ? QCTheme.success : QCTheme.amber)
                }
            } header: {
                Text("API server")
            } footer: {
                Text("Use http://127.0.0.1:8080 in the simulator, or your Mac’s LAN address on a physical device. Port 5433 is PostgreSQL, not this app.")
            }

            Section("Offline") {
                LabeledContent("Network", value: model.offline.isOnline ? "Online" : "Offline")
                LabeledContent("Queued writes", value: "\(model.offline.pendingCount)")
                if let flushError = model.offline.lastFlushError {
                    Text(flushError)
                        .font(.footnote)
                        .foregroundStyle(QCTheme.amber)
                }
                Button("Sync now") {
                    Task { await model.syncPending() }
                }
                .disabled(!model.offline.isOnline)
                Button("Clear local cache", role: .destructive) {
                    clearing = true
                    model.offline.clearCache()
                    clearing = false
                    note = "Local list cache cleared."
                }
                .disabled(clearing)
            }

            Section("Appearance") {
                LabeledContent("Theme", value: "System (light & dark)")
                LabeledContent("Accent", value: "Teal")
            }
        }
        .qcListChrome()
        .navigationTitle("Settings")
        .onAppear { draftServer = model.serverAddress }
        .errorAlert($errorMessage)
    }
}
