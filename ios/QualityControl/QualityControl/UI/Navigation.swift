import SwiftUI

enum AppSection: String, CaseIterable, Identifiable {
    case dashboard
    case scan
    case lots
    case inspections
    case nonconformances
    case correctiveActions
    case measurements
    case products
    case plans
    case checklists
    case sites
    case stations
    case suppliers
    case gauges
    case defects
    case users
    case webhooks
    case audit
    case tools
    case settings
    case account
    case more

    var id: String { rawValue }

    var title: String {
        switch self {
        case .dashboard: "Home"
        case .scan: "Scan"
        case .lots: "Lots"
        case .inspections: "Inspections"
        case .nonconformances: "Alerts"
        case .correctiveActions: "Corrective actions"
        case .measurements: "Measurements"
        case .products: "Products"
        case .plans: "Plans"
        case .checklists: "Checklists"
        case .sites: "Sites"
        case .stations: "Stations"
        case .suppliers: "Suppliers"
        case .gauges: "Gauges"
        case .defects: "Defect codes"
        case .users: "Users"
        case .webhooks: "Webhooks"
        case .audit: "Audit log"
        case .tools: "Tester tools"
        case .settings: "Settings"
        case .account: "Account"
        case .more: "More"
        }
    }

    var symbol: String {
        switch self {
        case .dashboard: "house.fill"
        case .scan: "barcode.viewfinder"
        case .lots: "shippingbox.fill"
        case .inspections: "checklist"
        case .nonconformances: "exclamationmark.triangle.fill"
        case .correctiveActions: "wrench.and.screwdriver.fill"
        case .measurements: "chart.xyaxis.line"
        case .products: "cube.fill"
        case .plans: "list.bullet.rectangle.fill"
        case .checklists: "checklist.checked"
        case .sites: "building.2.fill"
        case .stations: "wrench.adjustable.fill"
        case .suppliers: "truck.box.fill"
        case .gauges: "ruler.fill"
        case .defects: "tag.fill"
        case .users: "person.2.fill"
        case .webhooks: "antenna.radiowaves.left.and.right"
        case .audit: "clock.arrow.circlepath"
        case .tools: "hammer.fill"
        case .settings: "gearshape.fill"
        case .account: "person.crop.circle.fill"
        case .more: "ellipsis.circle.fill"
        }
    }

    func isVisible(to user: User) -> Bool {
        switch self {
        case .users: user.allows(.admin)
        case .audit, .webhooks: user.allows(.supervisor)
        default: true
        }
    }

    static let floor: [AppSection] = [.inspections, .correctiveActions, .measurements]
    static let setup: [AppSection] = [.products, .plans, .checklists, .sites, .stations, .suppliers, .gauges, .defects]
    static let system: [AppSection] = [.users, .webhooks, .audit, .tools, .settings, .account]

    @ViewBuilder
    func destination() -> some View {
        switch self {
        case .dashboard: DashboardView()
        case .scan: ScanView()
        case .lots: LotsView()
        case .inspections: InspectionsView()
        case .nonconformances: NonconformancesView()
        case .correctiveActions: CorrectiveActionsView()
        case .measurements: MeasurementsView()
        case .products: RecordListView(kind: .products)
        case .plans: PlansView()
        case .checklists: RecordListView(kind: .checklists)
        case .sites: RecordListView(kind: .sites)
        case .stations: RecordListView(kind: .stations)
        case .suppliers: RecordListView(kind: .suppliers)
        case .gauges: RecordListView(kind: .gauges)
        case .defects: RecordListView(kind: .defectCodes)
        case .users: RecordListView(kind: .users)
        case .webhooks: RecordListView(kind: .webhooks)
        case .audit: AuditView()
        case .account: AccountView()
        case .settings: SettingsView()
        case .tools: ToolsView()
        case .more: MoreView()
        }
    }
}

/// Tab bar on iPhone, sidebar on iPad. Shop setup and system pages live in the sidebar, or under More on iPhone.
struct RootView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.horizontalSizeClass) private var sizeClass

    private static let tabBar: [AppSection] = [.dashboard, .scan, .lots, .nonconformances, .more]

    var body: some View {
        @Bindable var model = model
        Group {
            if let user = model.user {
                TabView(selection: $model.tab) {
                    Tab(AppSection.dashboard.title, systemImage: AppSection.dashboard.symbol, value: AppSection.dashboard) {
                        SectionStack(section: .dashboard)
                    }
                    Tab(AppSection.scan.title, systemImage: AppSection.scan.symbol, value: AppSection.scan) {
                        ScanView().withBanners()
                    }
                    Tab(AppSection.lots.title, systemImage: AppSection.lots.symbol, value: AppSection.lots) {
                        SectionStack(section: .lots)
                    }
                    Tab(AppSection.nonconformances.title, systemImage: AppSection.nonconformances.symbol, value: AppSection.nonconformances) {
                        SectionStack(section: .nonconformances)
                    }
                    // Hidden tabs still count toward UIKit's five-tab limit, and its overflow screen
                    // wraps More in a second navigation bar, so compact width only gets the five above.
                    if sizeClass != .compact {
                        sidebarSections(user)
                    }
                    Tab(AppSection.more.title, systemImage: AppSection.more.symbol, value: AppSection.more) {
                        SectionStack(section: .more)
                    }
                    .defaultVisibility(.hidden, for: .sidebar)
                }
                .tabViewStyle(.sidebarAdaptable)
                .tint(QCTheme.brandColor)
                .task {
                    await model.loadEnums()
                    await model.refreshProfile()
                    await model.syncPending()
                }
                .onChange(of: sizeClass, initial: true) {
                    if sizeClass == .compact, !Self.tabBar.contains(model.tab) {
                        model.tab = .more
                    }
                }
                .onChange(of: model.user?.role) {
                    if let user = model.user, !model.tab.isVisible(to: user) {
                        model.tab = .dashboard
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

    @TabContentBuilder<AppSection>
    private func sidebarSections(_ user: User) -> some TabContent<AppSection> {
        TabSection("Floor") {
            ForEach(AppSection.floor) { section in
                Tab(section.title, systemImage: section.symbol, value: section) {
                    SectionStack(section: section)
                }
            }
        }
        .defaultVisibility(.hidden, for: .tabBar)
        TabSection("Shop setup") {
            ForEach(AppSection.setup) { section in
                Tab(section.title, systemImage: section.symbol, value: section) {
                    SectionStack(section: section)
                }
            }
        }
        .defaultVisibility(.hidden, for: .tabBar)
        TabSection("System") {
            ForEach(AppSection.system.filter { $0.isVisible(to: user) }) { section in
                Tab(section.title, systemImage: section.symbol, value: section) {
                    SectionStack(section: section)
                }
            }
        }
        .defaultVisibility(.hidden, for: .tabBar)
    }
}

private struct SectionStack: View {
    var section: AppSection

    var body: some View {
        NavigationStack {
            section.destination()
        }
        .withBanners()
    }
}

private struct Banners: ViewModifier {
    @Environment(AppModel.self) private var model

    func body(content: Content) -> some View {
        content.safeAreaInset(edge: .top, spacing: 0) {
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
    }
}

extension View {
    func withBanners() -> some View { modifier(Banners()) }
}

/// The iPhone list of every page that doesn't fit in the tab bar.
struct MoreView: View {
    @Environment(AppModel.self) private var model

    var body: some View {
        List {
            if let user = model.user {
                Section {
                    HStack(spacing: 14) {
                        Text(initials(user.fullName))
                            .font(.headline.weight(.bold))
                            .foregroundStyle(.white)
                            .frame(width: 46, height: 46)
                            .background(QCTheme.brandGradient, in: Circle())
                        VStack(alignment: .leading, spacing: 2) {
                            Text(user.fullName).font(QCTheme.headlineFont)
                            Text(user.roleValue.title).font(.subheadline).foregroundStyle(QCTheme.mutedInk)
                        }
                    }
                    .padding(.vertical, 4)
                    .accessibilityElement(children: .combine)
                }
                group("Floor", AppSection.floor, user)
                group("Shop setup", AppSection.setup, user)
                group("System", AppSection.system, user)
            }
        }
        .qcListChrome()
        .navigationTitle("More")
        .navigationDestination(for: AppSection.self) { $0.destination() }
    }

    private func group(_ title: String, _ sections: [AppSection], _ user: User) -> some View {
        Section(title) {
            ForEach(sections.filter { $0.isVisible(to: user) }) { section in
                NavigationLink(value: section) {
                    Label {
                        Text(section.title)
                    } icon: {
                        Image(systemName: section.symbol)
                            .foregroundStyle(.white)
                            .font(.footnote.weight(.semibold))
                            .frame(width: 28, height: 28)
                            .background(QCTheme.brandColor, in: RoundedRectangle(cornerRadius: 7, style: .continuous))
                    }
                }
            }
        }
    }
}

func initials(_ name: String) -> String {
    let parts = name.split(separator: " ").prefix(2)
    let letters = parts.compactMap(\.first).map(String.init).joined()
    return letters.isEmpty ? "?" : letters.uppercased()
}
