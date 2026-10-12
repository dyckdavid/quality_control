import SwiftUI

/// The shop floor home: scan first, then what needs doing, then the numbers.
struct DashboardView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.horizontalSizeClass) private var sizeClass
    @State private var stats: ClientDashboardStats?
    @State private var waiting: [Lot] = []
    @State private var alerts: [Nonconformance] = []
    @State private var loading = false
    @State private var errorMessage: String?
    @State private var fromCache = false
    @State private var creatingLot = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                greeting
                scanCard
                quickActions
                if sizeClass == .regular {
                    HStack(alignment: .top, spacing: 16) {
                        waitingCard
                        alertsCard
                    }
                } else {
                    waitingCard
                    alertsCard
                }
                metrics
            }
            .padding()
            .frame(maxWidth: 1100)
            .frame(maxWidth: .infinity)
        }
        .background(QCBackground())
        .navigationTitle("Home")
        #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
        #endif
        .navigationDestination(for: WorkRoute.self) { route in
            switch route {
            case .lot(let id): LotDetailView(id: id)
            case .inspection(let id): InspectionDetailView(id: id)
            }
        }
        .navigationDestination(for: Nonconformance.self) { NCRDetailView(id: $0.id) }
        .refreshable { await reload() }
        .task { await reload() }
        .errorAlert($errorMessage)
        .sheet(isPresented: $creatingLot, onDismiss: { Task { await reload() } }) {
            NavigationStack { LotForm() }
        }
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

    private var greeting: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(Date.now.formatted(.dateTime.weekday(.wide).month().day()))
                .font(QCTheme.captionFont)
                .foregroundStyle(QCTheme.mutedInk)
                .textCase(.uppercase)
            Text(hello)
                .font(QCTheme.displayFont)
                .foregroundStyle(QCTheme.ink)
        }
        .accessibilityElement(children: .combine)
    }

    private var hello: String {
        let hour = Calendar.current.component(.hour, from: .now)
        let part = hour < 12 ? "Good morning" : hour < 17 ? "Good afternoon" : "Good evening"
        let first = model.user?.fullName.split(separator: " ").first.map(String.init) ?? ""
        return first.isEmpty ? part : "\(part), \(first)"
    }

    private var scanCard: some View {
        Button {
            model.tab = .scan
        } label: {
            HStack(spacing: 18) {
                Image(systemName: "barcode.viewfinder")
                    .font(.system(size: 40, weight: .semibold))
                    .frame(width: 72, height: 72)
                    .background(.white.opacity(0.16), in: RoundedRectangle(cornerRadius: 18, style: .continuous))
                VStack(alignment: .leading, spacing: 4) {
                    Text("Scan a tag")
                        .font(.system(.title2, design: .rounded).weight(.bold))
                    Text("Lot tag, gauge label, part barcode, or alert number. Goes straight to the record.")
                        .font(.subheadline)
                        .opacity(0.88)
                        .multilineTextAlignment(.leading)
                }
                Spacer(minLength: 0)
                Image(systemName: "chevron.right")
                    .font(.headline)
                    .opacity(0.7)
            }
            .foregroundStyle(.white)
            .padding(20)
            .background(QCTheme.brandGradient, in: RoundedRectangle(cornerRadius: 24, style: .continuous))
            .shadow(color: QCTheme.brandColor.opacity(0.28), radius: 14, y: 8)
        }
        .buttonStyle(.plain)
        .accessibilityLabel("Scan a barcode or QR code")
    }

    private var quickActions: some View {
        HStack(spacing: 12) {
            if model.user?.allows(.operator) == true {
                QuickAction(title: "New lot", systemImage: "plus.square.fill", tint: QCTheme.copper) { creatingLot = true }
            }
            QuickAction(title: "Lots", systemImage: "shippingbox.fill", tint: QCTheme.brandColor) { model.tab = .lots }
            QuickAction(title: "Alerts", systemImage: "exclamationmark.triangle.fill", tint: QCTheme.danger) { model.tab = .nonconformances }
        }
    }

    private var waitingCard: some View {
        QCCard {
            VStack(alignment: .leading, spacing: 12) {
                cardTitle("Waiting for inspection", count: waiting.count, tint: QCTheme.info)
                if waiting.isEmpty {
                    Text(loading ? "Loading…" : "Nothing waiting. Nice.")
                        .foregroundStyle(QCTheme.mutedInk)
                }
                ForEach(waiting.prefix(6)) { lot in
                    NavigationLink(value: WorkRoute.lot(lot.id)) {
                        HStack {
                            VStack(alignment: .leading, spacing: 2) {
                                Text(lot.lotNumber).font(QCTheme.headlineFont).foregroundStyle(QCTheme.ink)
                                Text("\(formatNumber(lot.quantity)) \(lot.unit) · \(lot.receivedAt.formatted(.relative(presentation: .named)))")
                                    .font(.caption)
                                    .foregroundStyle(QCTheme.mutedInk)
                            }
                            Spacer()
                            StatusBadge(text: lot.status)
                            Image(systemName: "chevron.right").font(.caption).foregroundStyle(QCTheme.mutedInk)
                        }
                    }
                    .buttonStyle(.plain)
                    if lot.id != waiting.prefix(6).last?.id { Divider() }
                }
            }
        }
    }

    private var alertsCard: some View {
        QCCard {
            VStack(alignment: .leading, spacing: 12) {
                cardTitle("Open alerts", count: alerts.count, tint: QCTheme.danger)
                if alerts.isEmpty {
                    Text(loading ? "Loading…" : "No open alerts.")
                        .foregroundStyle(QCTheme.mutedInk)
                }
                ForEach(alerts.prefix(6)) { ncr in
                    NavigationLink(value: ncr) {
                        HStack {
                            Circle().fill(QCTheme.statusColor(ncr.severity)).frame(width: 10, height: 10)
                            VStack(alignment: .leading, spacing: 2) {
                                Text(ncr.ncrNumber).font(QCTheme.headlineFont).foregroundStyle(QCTheme.ink)
                                Text(ncr.title).font(.caption).foregroundStyle(QCTheme.mutedInk).lineLimit(1)
                            }
                            Spacer()
                            StatusBadge(text: ncr.status)
                            Image(systemName: "chevron.right").font(.caption).foregroundStyle(QCTheme.mutedInk)
                        }
                    }
                    .buttonStyle(.plain)
                    if ncr.id != alerts.prefix(6).last?.id { Divider() }
                }
            }
        }
    }

    @ViewBuilder
    private var metrics: some View {
        if let stats {
            VStack(alignment: .leading, spacing: 12) {
                Text("Shop numbers")
                    .font(QCTheme.titleFont)
                    .foregroundStyle(QCTheme.ink)
                LazyVGrid(columns: [GridItem(.adaptive(minimum: 150), spacing: 12)], spacing: 12) {
                    MetricCard(title: "Pass rate, 30 days", value: rate(stats.passRateLast30Days), systemImage: "checkmark.seal.fill", tint: QCTheme.success)
                    MetricCard(title: "Inspections, 30 days", value: "\(stats.inspectionsLast30Days)", systemImage: "checklist", tint: QCTheme.brandColor)
                    MetricCard(title: "Open inspections", value: "\(stats.openInspections)", systemImage: "hourglass", tint: QCTheme.info)
                    MetricCard(title: "Lots on hold", value: "\(stats.lotsOnHold)", systemImage: "pause.circle.fill", tint: QCTheme.amber)
                    MetricCard(title: "Open corrective actions", value: "\(stats.openCorrectiveActions)", systemImage: "wrench.and.screwdriver.fill", tint: QCTheme.copper)
                    MetricCard(title: "Overdue actions", value: "\(stats.overdueCorrectiveActions)", systemImage: "calendar.badge.exclamationmark", tint: QCTheme.danger)
                }
                QCCard {
                    VStack(spacing: 10) {
                        trendRow("Passed", stats.passLast30Days, QCTheme.success)
                        trendRow("Passed with minor defects", stats.conditionalLast30Days, QCTheme.amber)
                        trendRow("Failed", stats.failLast30Days, QCTheme.danger)
                    }
                }
            }
        } else if !loading, errorMessage != nil {
            ErrorStateView(title: "Numbers unavailable", message: errorMessage ?? "", retry: { Task { await reload() } })
        }
    }

    private func cardTitle(_ title: String, count: Int, tint: Color) -> some View {
        HStack {
            Text(title).font(QCTheme.headlineFont).foregroundStyle(QCTheme.ink)
            Spacer()
            Text("\(count)")
                .font(.subheadline.weight(.bold))
                .foregroundStyle(tint)
                .padding(.horizontal, 10)
                .padding(.vertical, 3)
                .background(tint.opacity(0.14), in: Capsule())
                .contentTransition(.numericText())
        }
    }

    private func trendRow(_ title: String, _ value: Int, _ tint: Color) -> some View {
        let total = max(stats?.inspectionsLast30Days ?? 0, 1)
        return VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text(title).foregroundStyle(QCTheme.ink)
                Spacer()
                Text("\(value)").font(QCTheme.headlineFont).foregroundStyle(QCTheme.ink)
            }
            GeometryReader { geo in
                Capsule().fill(QCTheme.line)
                    .overlay(alignment: .leading) {
                        Capsule().fill(tint).frame(width: geo.size.width * CGFloat(value) / CGFloat(total))
                    }
            }
            .frame(height: 6)
        }
        .accessibilityElement(children: .combine)
        .accessibilityLabel("\(title): \(value)")
    }

    private func reload() async {
        loading = true
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
                errorMessage = model.offline.isOnline ? nil : "Showing saved numbers while offline."
                return
            }
            errorMessage = "Could not load shop data."
            return
        }

        waiting = lots.filter { $0.status == "open" || $0.status == "in_inspection" }.sorted { $0.receivedAt > $1.receivedAt }
        alerts = ncrs.filter { $0.status == "open" || $0.status == "under_review" }
        let merged = ClientDashboardStats.merge(
            client: ClientDashboardStats.compute(lots: lots, inspections: inspections, nonconformances: ncrs, correctiveActions: capas),
            server: dash
        )
        withAnimation(.easeOut(duration: 0.3)) { stats = merged }
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

private struct QuickAction: View {
    var title: String
    var systemImage: String
    var tint: Color
    var action: () -> Void

    var body: some View {
        Button(action: action) {
            VStack(spacing: 8) {
                Image(systemName: systemImage)
                    .font(.title2.weight(.semibold))
                    .foregroundStyle(tint)
                Text(title)
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(QCTheme.ink)
            }
            .frame(maxWidth: .infinity)
            .padding(.vertical, 16)
            .background(QCTheme.elevated, in: RoundedRectangle(cornerRadius: 18, style: .continuous))
            .overlay(RoundedRectangle(cornerRadius: 18, style: .continuous).stroke(QCTheme.line))
        }
        .buttonStyle(.plain)
    }
}
