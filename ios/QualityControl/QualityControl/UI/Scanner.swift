import AVFoundation
import SwiftUI
import Vision
import VisionKit

/// Live barcode and QR reader. Reports each new code once.
struct CodeScanner: UIViewControllerRepresentable {
    var paused = false
    var onCode: (String) -> Void

    static var isReady: Bool {
        DataScannerViewController.isSupported && DataScannerViewController.isAvailable
    }

    func makeUIViewController(context: Context) -> DataScannerViewController {
        let scanner = DataScannerViewController(
            recognizedDataTypes: [.barcode(symbologies: [.qr, .code128, .code39, .code93, .ean13, .ean8, .upce, .dataMatrix, .pdf417, .aztec, .itf14, .i2of5])],
            qualityLevel: .balanced,
            recognizesMultipleItems: false,
            isHighFrameRateTrackingEnabled: false,
            isPinchToZoomEnabled: true,
            isGuidanceEnabled: false,
            isHighlightingEnabled: true
        )
        scanner.delegate = context.coordinator
        return scanner
    }

    func updateUIViewController(_ scanner: DataScannerViewController, context: Context) {
        context.coordinator.onCode = onCode
        if paused {
            if scanner.isScanning { scanner.stopScanning() }
        } else if !scanner.isScanning {
            try? scanner.startScanning()
        }
    }

    static func dismantleUIViewController(_ scanner: DataScannerViewController, coordinator: Coordinator) {
        scanner.stopScanning()
    }

    func makeCoordinator() -> Coordinator { Coordinator(onCode: onCode) }

    final class Coordinator: NSObject, DataScannerViewControllerDelegate {
        var onCode: (String) -> Void
        private var last: (code: String, at: Date)?

        init(onCode: @escaping (String) -> Void) { self.onCode = onCode }

        func dataScanner(_ dataScanner: DataScannerViewController, didAdd addedItems: [RecognizedItem], allItems: [RecognizedItem]) {
            for item in addedItems { report(item) }
        }

        func dataScanner(_ dataScanner: DataScannerViewController, didTapOn item: RecognizedItem) {
            last = nil
            report(item)
        }

        private func report(_ item: RecognizedItem) {
            guard case .barcode(let barcode) = item, let raw = barcode.payloadStringValue else { return }
            let code = ScanCode.clean(raw)
            guard !code.isEmpty else { return }
            if let last, last.code == code, Date.now.timeIntervalSince(last.at) < 2.5 { return }
            last = (code, .now)
            onCode(code)
        }
    }
}

enum ScanCode {
    /// The code a label carries. A URL label resolves to its last path part or its `code` query value.
    static func clean(_ raw: String) -> String {
        let text = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        if let url = URL(string: text), url.scheme != nil, url.host != nil {
            if let code = URLComponents(url: url, resolvingAgainstBaseURL: false)?.queryItems?.first(where: { $0.name == "code" })?.value {
                return code
            }
            if !url.lastPathComponent.isEmpty, url.lastPathComponent != "/" {
                return url.lastPathComponent
            }
        }
        return String(text.prefix(120))
    }
}

/// Asks for the camera once, then reports whether live scanning works on this device.
@Observable
final class CameraGate {
    var ready = false
    var denied = false

    func check() async {
        guard DataScannerViewController.isSupported else {
            ready = false
            return
        }
        switch AVCaptureDevice.authorizationStatus(for: .video) {
        case .notDetermined:
            let granted = await AVCaptureDevice.requestAccess(for: .video)
            denied = !granted
        case .denied, .restricted:
            denied = true
        default:
            break
        }
        ready = CodeScanner.isReady
    }
}

/// One match for a scanned code.
struct ScanHit: Identifiable, Hashable {
    var entityType: String
    var id: UUID
    var number: String
    var title: String
    var status: String

    var symbol: String {
        switch entityType {
        case "lot": "shippingbox.fill"
        case "inspection": "checklist"
        case "nonconformance": "exclamationmark.triangle.fill"
        case "corrective_action": "wrench.and.screwdriver.fill"
        default: RecordKind.scanned(entityType)?.symbol ?? "questionmark.square"
        }
    }

    var kindName: String {
        switch entityType {
        case "nonconformance": "Alert"
        case "corrective_action": "Corrective action"
        case "defect_code": "Defect code"
        default: entityType.capitalized
        }
    }
}

extension AppModel {
    func lookup(_ code: String) async throws -> [ScanHit] {
        let rows = try await client.json("/api/v1/lookup", query: ["code": code]) as? [JSONObject] ?? []
        return rows.compactMap { row in
            guard let id = UUID(uuidString: jsonText(row["id"])) else { return nil }
            return ScanHit(
                entityType: jsonText(row["entity_type"]),
                id: id,
                number: jsonText(row["number"]),
                title: jsonText(row["title"]),
                status: jsonText(row["status"])
            )
        }
    }
}

@ViewBuilder
func scanDestination(_ hit: ScanHit) -> some View {
    switch hit.entityType {
    case "lot": LotDetailView(id: hit.id)
    case "inspection": InspectionDetailView(id: hit.id)
    case "nonconformance": NCRDetailView(id: hit.id)
    case "corrective_action": CAPADetailView(id: hit.id)
    default:
        if let kind = RecordKind.scanned(hit.entityType) {
            RecordEditorView(kind: kind, id: hit.id)
        } else {
            EmptyStateView(title: "Can't open this", systemImage: "questionmark.square", description: hit.number)
        }
    }
}

/// The Scan tab: point at a lot tag, gauge label, or part barcode and go straight to the record.
struct ScanView: View {
    @Environment(AppModel.self) private var model
    @State private var camera = CameraGate()
    @State private var path: [ScanHit] = []
    @State private var typed = ""
    @State private var looking = false
    @State private var lastCode: String?
    @State private var hits: [ScanHit] = []
    @State private var message: String?
    @State private var foundCount = 0
    @State private var missCount = 0
    @AppStorage("qc.recentScans") private var recentRaw = ""
    @FocusState private var typing: Bool

    private var recent: [String] { recentRaw.split(separator: "\n").map(String.init) }

    var body: some View {
        NavigationStack(path: $path) {
            VStack(spacing: 0) {
                viewfinder
                panel
            }
            .background(QCTheme.canvas)
            .navigationTitle("Scan")
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            #endif
            .navigationDestination(for: ScanHit.self) { scanDestination($0) }
            .task { await camera.check() }
            .sensoryFeedback(.success, trigger: foundCount)
            .sensoryFeedback(.warning, trigger: missCount)
        }
    }

    private var viewfinder: some View {
        ZStack {
            Color.black
            if camera.ready {
                CodeScanner(paused: !path.isEmpty || looking || typing) { code in
                    Task { await resolve(code) }
                }
                .ignoresSafeArea(edges: .horizontal)
                RoundedRectangle(cornerRadius: 22, style: .continuous)
                    .strokeBorder(.white.opacity(0.85), style: StrokeStyle(lineWidth: 3, dash: [26, 14]))
                    .frame(width: 250, height: 170)
                    .shadow(color: .black.opacity(0.4), radius: 8)
                    .allowsHitTesting(false)
            } else {
                VStack(spacing: 12) {
                    Image(systemName: camera.denied ? "camera.badge.ellipsis" : "barcode.viewfinder")
                        .font(.system(size: 54, weight: .semibold))
                        .foregroundStyle(.white.opacity(0.9))
                    Text(camera.denied ? "Camera access is off" : "Live scanning needs a camera")
                        .font(QCTheme.headlineFont)
                        .foregroundStyle(.white)
                    Text(camera.denied
                         ? "Turn on Camera for Quality Control in Settings, or type the code below."
                         : "This device can't scan live. Type the code below, or use a Bluetooth barcode scanner in the box.")
                        .font(.subheadline)
                        .foregroundStyle(.white.opacity(0.75))
                        .multilineTextAlignment(.center)
                        .padding(.horizontal, 32)
                    if camera.denied, let url = URL(string: UIApplication.openSettingsURLString) {
                        Link("Open Settings", destination: url)
                            .buttonStyle(.borderedProminent)
                            .tint(QCTheme.brandColor)
                    }
                }
            }
            VStack {
                Spacer()
                Text(looking ? "Looking up \(lastCode ?? "")…" : "Lot tags, gauge labels, part barcodes, alert and inspection numbers")
                    .font(.footnote.weight(.medium))
                    .foregroundStyle(.white)
                    .padding(.horizontal, 14)
                    .padding(.vertical, 8)
                    .background(.black.opacity(0.55), in: Capsule())
                    .padding(.bottom, 14)
            }
            .allowsHitTesting(false)
        }
        .frame(maxHeight: .infinity)
        .clipped()
    }

    private var panel: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(spacing: 10) {
                Image(systemName: "keyboard")
                    .foregroundStyle(QCTheme.mutedInk)
                TextField("Type or scan a code", text: $typed)
                    .textInputAutocapitalization(.characters)
                    .autocorrectionDisabled()
                    .submitLabel(.search)
                    .focused($typing)
                    .onSubmit { Task { await resolve(typed) } }
                    .accessibilityLabel("Code")
                Button("Find") { Task { await resolve(typed) } }
                    .buttonStyle(.borderedProminent)
                    .tint(QCTheme.brandColor)
                    .disabled(typed.trimmingCharacters(in: .whitespaces).isEmpty || looking)
            }
            .padding(12)
            .background(QCTheme.elevated, in: RoundedRectangle(cornerRadius: 14, style: .continuous))
            .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).stroke(QCTheme.line))

            if let message {
                Label(message, systemImage: "exclamationmark.magnifyingglass")
                    .font(.subheadline)
                    .foregroundStyle(QCTheme.amber)
            }

            if hits.count > 1 {
                Text("\(lastCode ?? "") matches more than one record")
                    .font(QCTheme.captionFont)
                    .foregroundStyle(QCTheme.mutedInk)
                ForEach(hits) { hit in
                    Button { path.append(hit) } label: { HitRow(hit: hit) }
                        .buttonStyle(.plain)
                }
            } else if !recent.isEmpty {
                Text("Recent")
                    .font(QCTheme.captionFont)
                    .foregroundStyle(QCTheme.mutedInk)
                ScrollView(.horizontal, showsIndicators: false) {
                    HStack(spacing: 8) {
                        ForEach(recent, id: \.self) { code in
                            FilterChip(title: code, selected: false) { Task { await resolve(code) } }
                        }
                    }
                }
            }
        }
        .padding(16)
        .frame(maxWidth: 700)
        .frame(maxWidth: .infinity)
        .background(QCTheme.canvas)
    }

    private func resolve(_ raw: String) async {
        let code = ScanCode.clean(raw)
        guard !code.isEmpty, !looking, path.isEmpty else { return }
        typing = false
        looking = true
        lastCode = code
        message = nil
        defer { looking = false }
        do {
            let found = try await model.lookup(code)
            hits = found
            remember(code)
            switch found.count {
            case 0:
                missCount += 1
                message = "Nothing in the system matches \(code)."
            case 1:
                foundCount += 1
                typed = ""
                path.append(found[0])
            default:
                foundCount += 1
                typed = ""
            }
        } catch {
            missCount += 1
            message = error.localizedDescription
        }
    }

    private func remember(_ code: String) {
        var list = recent.filter { $0 != code }
        list.insert(code, at: 0)
        recentRaw = list.prefix(8).joined(separator: "\n")
    }
}

private struct HitRow: View {
    var hit: ScanHit

    var body: some View {
        HStack(spacing: 12) {
            Image(systemName: hit.symbol)
                .foregroundStyle(QCTheme.brandColor)
                .frame(width: 34, height: 34)
                .background(QCTheme.brandColor.opacity(0.12), in: RoundedRectangle(cornerRadius: 9, style: .continuous))
            VStack(alignment: .leading, spacing: 2) {
                Text(hit.number).font(QCTheme.headlineFont).foregroundStyle(QCTheme.ink)
                Text([hit.kindName, hit.title].filter { !$0.isEmpty }.joined(separator: " · "))
                    .font(.subheadline)
                    .foregroundStyle(QCTheme.mutedInk)
                    .lineLimit(1)
            }
            Spacer(minLength: 0)
            if !hit.status.isEmpty { StatusBadge(text: hit.status) }
            Image(systemName: "chevron.right").foregroundStyle(QCTheme.mutedInk)
        }
        .padding(12)
        .background(QCTheme.elevated, in: RoundedRectangle(cornerRadius: 14, style: .continuous))
    }
}

/// A small barcode button next to a text field that fills it from a scan.
struct ScanFieldButton: View {
    @Binding var text: String
    @State private var scanning = false

    var body: some View {
        Button {
            scanning = true
        } label: {
            Image(systemName: "barcode.viewfinder")
                .font(.title3)
                .foregroundStyle(QCTheme.brandColor)
        }
        .buttonStyle(.borderless)
        .accessibilityLabel("Scan into this field")
        .sheet(isPresented: $scanning) {
            CaptureSheet { code in
                text = code
                scanning = false
            }
            .presentationDetents([.medium, .large])
        }
    }
}

private struct CaptureSheet: View {
    var onCode: (String) -> Void
    @Environment(\.dismiss) private var dismiss
    @State private var camera = CameraGate()
    @State private var typed = ""
    @State private var captured = 0

    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                ZStack {
                    Color.black
                    if camera.ready {
                        CodeScanner { code in
                            captured += 1
                            onCode(code)
                        }
                    } else {
                        Text(camera.denied ? "Camera access is off. Type the code instead." : "Live scanning isn't available here. Type the code instead.")
                            .foregroundStyle(.white.opacity(0.8))
                            .multilineTextAlignment(.center)
                            .padding()
                    }
                }
                HStack {
                    TextField("Type the code", text: $typed)
                        .textInputAutocapitalization(.characters)
                        .autocorrectionDisabled()
                        .onSubmit { onCode(ScanCode.clean(typed)) }
                    Button("Use") { onCode(ScanCode.clean(typed)) }
                        .buttonStyle(.borderedProminent)
                        .tint(QCTheme.brandColor)
                        .disabled(typed.trimmingCharacters(in: .whitespaces).isEmpty)
                }
                .padding()
            }
            .navigationTitle("Scan a code")
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            #endif
            .toolbar { ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } } }
            .task { await camera.check() }
            .sensoryFeedback(.success, trigger: captured)
        }
    }
}
