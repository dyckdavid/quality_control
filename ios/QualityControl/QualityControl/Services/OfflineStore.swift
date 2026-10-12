import Foundation
import Network

/// Disk cache for list payloads plus a small offline write queue.
@Observable
@MainActor
final class OfflineStore {
    private(set) var pendingCount = 0
    private(set) var isOnline = true
    private(set) var lastFlushError: String?

    private let monitor = NWPathMonitor()
    private let monitorQueue = DispatchQueue(label: "qc.offline.monitor")
    private let root: URL
    private let encoder = JSONEncoder()
    private let decoder = JSONDecoder()

    init() {
        let base = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first
            ?? FileManager.default.temporaryDirectory
        root = base.appendingPathComponent("QCOffline", isDirectory: true)
        try? FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        encoder.dateEncodingStrategy = .iso8601
        decoder.dateDecodingStrategy = .iso8601
        pendingCount = loadQueue().count
        startMonitor()
    }

    func cache<T: Encodable>(_ value: T, forKey key: String) {
        let url = cacheURL(key)
        guard let data = try? encoder.encode(value) else { return }
        try? data.write(to: url, options: .atomic)
    }

    func cached<T: Decodable>(_ type: T.Type, forKey key: String) -> T? {
        let url = cacheURL(key)
        guard let data = try? Data(contentsOf: url) else { return nil }
        return try? decoder.decode(type, from: data)
    }

    func enqueue(method: String, path: String, body: [String: Any]?) {
        var queue = loadQueue()
        let bodyData: Data?
        if let body {
            bodyData = try? JSONSerialization.data(withJSONObject: body)
        } else {
            bodyData = nil
        }
        queue.append(PendingMutation(
            id: UUID(),
            method: method,
            path: path,
            bodyJSON: bodyData,
            createdAt: Date()
        ))
        saveQueue(queue)
        pendingCount = queue.count
    }

    func flush(using client: APIClient) async {
        guard isOnline else { return }
        var queue = loadQueue()
        guard !queue.isEmpty else {
            lastFlushError = nil
            return
        }
        var remaining: [PendingMutation] = []
        for item in queue {
            do {
                var object: [String: Any]?
                if let bodyJSON = item.bodyJSON {
                    object = try JSONSerialization.jsonObject(with: bodyJSON) as? [String: Any]
                }
                try await client.execute(method: item.method, path: item.path, body: object)
            } catch {
                remaining.append(item)
                lastFlushError = error.localizedDescription
                break
            }
        }
        if remaining.count < queue.count {
            lastFlushError = remaining.isEmpty ? nil : lastFlushError
        }
        saveQueue(remaining)
        pendingCount = remaining.count
    }

    func clearCache() {
        let files = (try? FileManager.default.contentsOfDirectory(at: root, includingPropertiesForKeys: nil)) ?? []
        for file in files where file.lastPathComponent != "queue.json" {
            try? FileManager.default.removeItem(at: file)
        }
    }

    private func startMonitor() {
        monitor.pathUpdateHandler = { [weak self] path in
            Task { @MainActor in
                guard let self else { return }
                self.isOnline = path.status == .satisfied
            }
        }
        monitor.start(queue: monitorQueue)
    }

    private func cacheURL(_ key: String) -> URL {
        let safe = key.replacingOccurrences(of: "/", with: "_")
        return root.appendingPathComponent("\(safe).json")
    }

    private func queueURL() -> URL {
        root.appendingPathComponent("queue.json")
    }

    private func loadQueue() -> [PendingMutation] {
        guard let data = try? Data(contentsOf: queueURL()),
              let items = try? decoder.decode([PendingMutation].self, from: data) else {
            return []
        }
        return items
    }

    private func saveQueue(_ items: [PendingMutation]) {
        guard let data = try? encoder.encode(items) else { return }
        try? data.write(to: queueURL(), options: .atomic)
    }
}

nonisolated struct PendingMutation: Codable, Identifiable {
    var id: UUID
    var method: String
    var path: String
    var bodyJSON: Data?
    var createdAt: Date
}

enum CacheKey {
    static let dashboard = "dashboard"
    static let lots = "lots"
    static let inspections = "inspections"
    static let nonconformances = "nonconformances"
    static let correctiveActions = "corrective_actions"
}
