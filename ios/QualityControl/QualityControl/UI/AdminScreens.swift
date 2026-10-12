import SwiftUI

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
