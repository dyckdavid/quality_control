import SwiftUI

struct StatusBadge: View {
    var text: String

    var body: some View {
        Text(displayName(text))
            .font(.caption.weight(.semibold))
            .padding(.horizontal, 8)
            .padding(.vertical, 3)
            .background(color.opacity(0.16), in: Capsule())
            .foregroundStyle(color)
    }

    private var color: Color {
        switch text {
        case "pass", "accepted", "active", "closed":
            .green
        case "fail", "rejected", "critical", "void", "cancelled":
            .red
        case "conditional", "on_hold", "major", "under_review", "verification":
            .orange
        case "in_progress", "in_inspection", "dispositioned":
            .blue
        default:
            .secondary
        }
    }
}

struct MetricCard: View {
    var title: String
    var value: String
    var systemImage: String

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Label(title, systemImage: systemImage)
                .font(.subheadline)
                .foregroundStyle(.secondary)
            Text(value)
                .font(.title.weight(.semibold))
                .minimumScaleFactor(0.6)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding()
        .background(.fill.tertiary, in: RoundedRectangle(cornerRadius: 16))
    }
}

struct ErrorAlert: ViewModifier {
    @Binding var message: String?

    func body(content: Content) -> some View {
        VStack(spacing: 0) {
            if let message {
                HStack(alignment: .top, spacing: 10) {
                    Image(systemName: "exclamationmark.triangle.fill")
                        .foregroundStyle(.orange)
                    Text(message)
                        .font(.subheadline)
                        .frame(maxWidth: .infinity, alignment: .leading)
                    Button("Dismiss") { self.message = nil }
                        .font(.subheadline.weight(.semibold))
                }
                .padding(12)
                .background(.orange.opacity(0.14))
                .accessibilityElement(children: .combine)
            }
            content
        }
    }
}

extension View {
    func errorAlert(_ message: Binding<String?>) -> some View {
        modifier(ErrorAlert(message: message))
    }
}

@Observable
final class RemoteData<Value> {
    var value: Value?
    var error: String?
    var loading = false

    func load(_ work: () async throws -> Value) async {
        if value == nil { loading = true }
        defer { loading = false }
        do {
            let next = try await work()
            guard !Task.isCancelled else { return }
            value = next
            error = nil
        } catch is CancellationError {
            return
        } catch let error as URLError where error.code == .cancelled {
            return
        } catch {
            guard !Task.isCancelled else { return }
            self.error = error.localizedDescription
        }
    }
}

struct LabeledField: View {
    var title: String
    @Binding var text: String
    var axis: Axis = .horizontal

    var body: some View {
        TextField(title, text: $text, axis: axis)
    }
}

struct ActiveToggle: View {
    @Binding var active: Bool
    var body: some View {
        Toggle("Active", isOn: $active)
    }
}

struct SaveToolbar: View {
    var title: String = "Save"
    var disabled: Bool
    var action: () -> Void

    var body: some View {
        Button(title, action: action)
            .disabled(disabled)
    }
}

extension AppSection {
    @ViewBuilder
    func destination() -> some View {
        switch self {
        case .dashboard: DashboardView()
        case .lots: LotsView()
        case .inspections: InspectionsView()
        case .nonconformances: NonconformancesView()
        case .correctiveActions: CorrectiveActionsView()
        case .measurements: MeasurementsView()
        case .products: ProductsView()
        case .plans: PlansView()
        case .sites: SitesView()
        case .stations: StationsView()
        case .suppliers: SuppliersView()
        case .gauges: GaugesView()
        case .defects: DefectCodesView()
        case .users: UsersView()
        case .audit: AuditView()
        case .account: AccountView()
        case .tools: ToolsView()
        }
    }
}
