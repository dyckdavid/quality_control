import SwiftUI

struct StatusBadge: View {
    var text: String

    var body: some View {
        Text(displayName(text))
            .font(QCTheme.captionFont)
            .padding(.horizontal, 8)
            .padding(.vertical, 4)
            .background(QCTheme.statusColor(text).opacity(0.16), in: Capsule())
            .foregroundStyle(QCTheme.statusColor(text))
            .accessibilityLabel("Status \(displayName(text))")
    }
}

struct MetricCard: View {
    var title: String
    var value: String
    var systemImage: String
    var tint: Color = QCTheme.brandColor

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                Image(systemName: systemImage)
                    .font(.title3.weight(.semibold))
                    .foregroundStyle(tint)
                    .symbolRenderingMode(.hierarchical)
                Spacer(minLength: 0)
            }
            Text(value)
                .font(.system(size: 28, weight: .bold, design: .rounded))
                .foregroundStyle(QCTheme.ink)
                .minimumScaleFactor(0.6)
                .contentTransition(.numericText())
            Text(title)
                .font(QCTheme.captionFont)
                .foregroundStyle(QCTheme.mutedInk)
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(QCTheme.elevated, in: RoundedRectangle(cornerRadius: 18, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .stroke(QCTheme.brandColor.opacity(0.08), lineWidth: 1)
        )
        .accessibilityElement(children: .combine)
        .accessibilityLabel("\(title): \(value)")
    }
}

struct FilterChip: View {
    var title: String
    var selected: Bool
    var action: () -> Void

    var body: some View {
        Button(action: action) {
            Text(title)
                .font(QCTheme.captionFont)
                .padding(.horizontal, 12)
                .padding(.vertical, 7)
                .background(selected ? QCTheme.brandColor : QCTheme.elevated, in: Capsule())
                .foregroundStyle(selected ? Color.white : QCTheme.ink)
                .overlay(
                    Capsule()
                        .stroke(QCTheme.brandColor.opacity(selected ? 0 : 0.18), lineWidth: 1)
                )
        }
        .buttonStyle(.plain)
        .accessibilityLabel(title)
        .accessibilityAddTraits(selected ? .isSelected : [])
    }
}

struct SortMenu<Option: Hashable & Identifiable>: View where Option: CustomStringConvertible {
    @Binding var selection: Option
    var options: [Option]

    var body: some View {
        Menu {
            ForEach(options) { option in
                Button {
                    selection = option
                } label: {
                    if selection == option {
                        Label(option.description, systemImage: "checkmark")
                    } else {
                        Text(option.description)
                    }
                }
            }
        } label: {
            Label("Sort", systemImage: "arrow.up.arrow.down")
        }
        .accessibilityLabel("Sort list")
    }
}

struct LoadingStateView: View {
    var message: String = "Loading"

    var body: some View {
        VStack(spacing: 14) {
            ProgressView()
                .controlSize(.large)
                .tint(QCTheme.brandColor)
            Text(message)
                .font(QCTheme.headlineFont)
                .foregroundStyle(QCTheme.mutedInk)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .accessibilityElement(children: .combine)
        .accessibilityLabel(message)
    }
}

struct EmptyStateView: View {
    var title: String
    var systemImage: String
    var description: String

    var body: some View {
        ContentUnavailableView {
            Label(title, systemImage: systemImage)
                .symbolRenderingMode(.hierarchical)
                .foregroundStyle(QCTheme.brandColor)
        } description: {
            Text(description)
                .foregroundStyle(QCTheme.mutedInk)
        }
        .accessibilityElement(children: .combine)
    }
}

struct ErrorStateView: View {
    var title: String
    var message: String
    var retry: (() -> Void)?

    var body: some View {
        ContentUnavailableView {
            Label(title, systemImage: "wifi.exclamationmark")
                .foregroundStyle(QCTheme.amber)
        } description: {
            Text(message)
        } actions: {
            if let retry {
                Button("Try again", action: retry)
                    .buttonStyle(.borderedProminent)
                    .tint(QCTheme.brandColor)
            }
        }
        .accessibilityElement(children: .combine)
    }
}

struct ErrorAlert: ViewModifier {
    @Binding var message: String?

    func body(content: Content) -> some View {
        VStack(spacing: 0) {
            if let message {
                HStack(alignment: .top, spacing: 10) {
                    Image(systemName: "exclamationmark.triangle.fill")
                        .foregroundStyle(QCTheme.amber)
                        .accessibilityHidden(true)
                    Text(message)
                        .font(.subheadline)
                        .frame(maxWidth: .infinity, alignment: .leading)
                    Button("Dismiss") { self.message = nil }
                        .font(.subheadline.weight(.semibold))
                }
                .padding(12)
                .background(QCTheme.amber.opacity(0.14))
                .accessibilityElement(children: .combine)
                .accessibilityLabel("Error: \(message)")
            }
            content
        }
    }
}

extension View {
    func errorAlert(_ message: Binding<String?>) -> some View {
        modifier(ErrorAlert(message: message))
    }

    func qcListChrome() -> some View {
        self
            .scrollContentBackground(.hidden)
            .background(QCTheme.canvas)
    }
}

@Observable
final class RemoteData<Value> {
    var value: Value?
    var error: String?
    var loading = false
    var fromCache = false

    func load(_ work: () async throws -> Value) async {
        if value == nil { loading = true }
        defer { loading = false }
        do {
            let next = try await work()
            guard !Task.isCancelled else { return }
            value = next
            error = nil
            fromCache = false
        } catch is CancellationError {
            return
        } catch let error as URLError where error.code == .cancelled {
            return
        } catch {
            guard !Task.isCancelled else { return }
            self.error = error.localizedDescription
        }
    }

    func load(
        cache: @escaping () -> Value?,
        remote: () async throws -> Value,
        store: @escaping (Value) -> Void
    ) async {
        if value == nil, let cached = cache() {
            value = cached
            fromCache = true
        }
        if value == nil { loading = true }
        defer { loading = false }
        do {
            let next = try await remote()
            guard !Task.isCancelled else { return }
            value = next
            error = nil
            fromCache = false
            store(next)
        } catch is CancellationError {
            return
        } catch let error as URLError where error.code == .cancelled {
            return
        } catch {
            guard !Task.isCancelled else { return }
            if value == nil {
                self.error = error.localizedDescription
            } else {
                self.error = "Showing cached data. \(error.localizedDescription)"
            }
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

struct OfflineBanner: View {
    @Environment(AppModel.self) private var model

    var body: some View {
        if !model.offline.isOnline || model.offline.pendingCount > 0 {
            HStack(spacing: 8) {
                Image(systemName: model.offline.isOnline ? "arrow.triangle.2.circlepath" : "wifi.slash")
                if model.offline.isOnline {
                    Text("\(model.offline.pendingCount) change\(model.offline.pendingCount == 1 ? "" : "s") waiting to sync")
                } else {
                    Text("Offline — using cached records")
                }
                Spacer(minLength: 0)
                if model.offline.isOnline, model.offline.pendingCount > 0 {
                    Button("Sync") {
                        Task { await model.offline.flush(using: model.client) }
                    }
                    .font(.subheadline.weight(.semibold))
                }
            }
            .font(.subheadline)
            .padding(.horizontal, 12)
            .padding(.vertical, 8)
            .background(QCTheme.amber.opacity(0.18))
            .accessibilityElement(children: .combine)
        }
    }
}

enum ListSort: String, CaseIterable, Identifiable, CustomStringConvertible {
    case newest
    case oldest
    case nameAZ
    case nameZA
    case status

    var id: String { rawValue }

    var description: String {
        switch self {
        case .newest: "Newest first"
        case .oldest: "Oldest first"
        case .nameAZ: "Name A–Z"
        case .nameZA: "Name Z–A"
        case .status: "Status"
        }
    }
}