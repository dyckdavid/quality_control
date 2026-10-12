import Foundation

@Observable
final class AppModel {
    var serverAddress: String
    var user: User?
    var enums = APIEnums()
    var notice: String?
    var lastUsername: String
    /// The open tab, so any screen can jump to Scan or a list.
    var tab: AppSection = .dashboard
    private(set) var client: APIClient
    let offline = OfflineStore()
    private let defaults = UserDefaults.standard

    init() {
        let saved = defaults.string(forKey: Keys.server) ?? "http://127.0.0.1:8080"
        serverAddress = saved
        lastUsername = defaults.string(forKey: Keys.username) ?? "admin"
        client = APIClient(baseURL: URL(string: saved) ?? URL(string: "http://127.0.0.1:8080")!)
        if let data = defaults.data(forKey: Keys.session),
           let session = try? SessionCoding.decoder.decode(StoredSession.self, from: data) {
            user = session.user
            client.accessToken = session.accessToken
            client.refreshToken = session.refreshToken
            if let url = URL(string: session.serverAddress) {
                serverAddress = session.serverAddress
                client.baseURL = url
            }
        }
        client.onSessionExpired = { [weak self] in
            self?.notice = "Your session ended. Sign in again."
            self?.clearSession()
        }
        client.onTokensRefreshed = { [weak self] in
            self?.persist()
        }
    }

    func applyServerAddress() throws {
        let url = try validatedURL()
        client.baseURL = url
        serverAddress = url.absoluteString
        defaults.set(serverAddress, forKey: Keys.server)
    }

    func syncPending() async {
        await offline.flush(using: client)
        await uploadPendingPhotos()
    }

    private func uploadPendingPhotos() async {
        guard offline.isOnline, user?.allows(.operator) == true else { return }
        for photo in LocalPhotoStore.shared.pendingUploads() {
            guard let data = LocalPhotoStore.shared.data(for: photo) else { continue }
            do {
                _ = try await client.upload(
                    entityType: photo.entityType,
                    entityId: photo.entityId,
                    fileName: photo.fileName,
                    mimeType: "image/jpeg",
                    data: data
                )
                LocalPhotoStore.shared.markUploaded(photo)
            } catch {
                break
            }
        }
    }

    var isSignedIn: Bool { user != nil }

    func login(username: String, password: String) async throws {
        let url = try validatedURL()
        client.baseURL = url
        serverAddress = url.absoluteString
        _ = try await client.health()
        let name = username.trimmingCharacters(in: .whitespacesAndNewlines)
        let payload = try await client.login(username: name, password: password)
        lastUsername = name
        defaults.set(name, forKey: Keys.username)
        notice = nil
        apply(payload)
        await loadEnums()
    }

    func probe() async -> String {
        do {
            let url = try validatedURL()
            client.baseURL = url
            serverAddress = url.absoluteString
            let health = try await client.health()
            return "The API answered (\(health.status)) at \(serverAddress)."
        } catch {
            return error.localizedDescription
        }
    }

    private func validatedURL() throws -> URL {
        let address = normalize(serverAddress)
        guard let url = URL(string: address), url.scheme == "http" || url.scheme == "https", url.host != nil else {
            throw APIFailure(status: 0, message: "Enter the API address http://127.0.0.1:8080. Port 5433 is the database, not this app.")
        }
        if url.port == 5432 || url.port == 5433 {
            throw APIFailure(status: 0, message: "Port \(url.port ?? 0) is PostgreSQL. The app server is http://127.0.0.1:8080. Sign in as admin, not qc.")
        }
        return url
    }

    func logout() async {
        await client.logout()
        clearSession()
    }

    func loadEnums() async {
        if let loaded: APIEnums = try? await client.one("/api/v1/enums") {
            enums = loaded
        }
    }

    func refreshProfile() async {
        if let me: User = try? await client.one("/api/v1/me") {
            user = me
            persist()
        }
    }

    private func apply(_ payload: SessionPayload) {
        user = payload.user
        client.accessToken = payload.accessToken
        client.refreshToken = payload.refreshToken
        persist()
    }

    private func persist() {
        guard let user, let access = client.accessToken, let refresh = client.refreshToken else { return }
        let session = StoredSession(serverAddress: serverAddress, accessToken: access, refreshToken: refresh, user: user)
        if let data = try? SessionCoding.encoder.encode(session) {
            defaults.set(data, forKey: Keys.session)
        }
        defaults.set(serverAddress, forKey: Keys.server)
    }

    private func clearSession() {
        user = nil
        client.accessToken = nil
        client.refreshToken = nil
        defaults.removeObject(forKey: Keys.session)
    }

    private func normalize(_ value: String) -> String {
        var text = value.trimmingCharacters(in: .whitespacesAndNewlines)
        while text.hasSuffix("/") {
            text.removeLast()
        }
        if !text.contains("://") {
            text = "http://" + text
        }
        return text
    }
}

private struct StoredSession: Codable {
    var serverAddress: String
    var accessToken: String
    var refreshToken: String
    var user: User
}

private enum Keys {
    static let server = "qc.server"
    static let session = "qc.session"
    static let username = "qc.username"
}

private enum SessionCoding {
    static let encoder: JSONEncoder = {
        let encoder = JSONEncoder()
        encoder.dateEncodingStrategy = .iso8601
        return encoder
    }()

    static let decoder: JSONDecoder = {
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return decoder
    }()
}

extension APIClient {
    func one<T: Decodable>(_ path: String, query: [String: String] = [:]) async throws -> T {
        let envelope: Envelope<T> = try await get(path, query: query)
        return envelope.data
    }

    func list<T: Decodable>(_ path: String, query: [String: String] = [:]) async throws -> [T] {
        let page: Page<T> = try await get(path, query: query)
        return page.data
    }

    func create<T: Decodable>(_ path: String, body: [String: Any]) async throws -> T {
        let envelope: Envelope<T> = try await post(path, body: body)
        return envelope.data
    }

    func update<T: Decodable>(_ path: String, body: [String: Any]) async throws -> T {
        let envelope: Envelope<T> = try await patch(path, body: body)
        return envelope.data
    }

    func action<T: Decodable>(_ path: String, body: [String: Any] = [:]) async throws -> T {
        let envelope: Envelope<T> = try await post(path, body: body)
        return envelope.data
    }
}

enum Query {
    static func compact(_ pairs: [String: String?]) -> [String: String] {
        var query: [String: String] = ["limit": "100"]
        for (key, value) in pairs {
            if let value, !value.isEmpty {
                query[key] = value
            }
        }
        return query
    }
}

func displayName(_ raw: String) -> String {
    raw.replacingOccurrences(of: "_", with: " ")
}

func lotSourceName(_ raw: String) -> String {
    switch raw {
    case "incoming": "Bought in"
    case "in_process": "Built here"
    case "final": "Final"
    case "customer_return": "Customer return"
    default: displayName(raw)
    }
}

func parseNumber(_ text: String) -> Double? {
    let cleaned = text.trimmingCharacters(in: .whitespacesAndNewlines).replacingOccurrences(of: ",", with: ".")
    return Double(cleaned)
}

func formatNumber(_ value: Double) -> String {
    value.formatted(.number.precision(.fractionLength(0...4)))
}

/// A number for an edit field: no grouping and a dot for decimals, so `parseNumber` reads it back unchanged.
func plainNumber(_ value: Double) -> String {
    value.formatted(.number.grouping(.never).precision(.fractionLength(0...6)).locale(Locale(identifier: "en_US_POSIX")))
}
