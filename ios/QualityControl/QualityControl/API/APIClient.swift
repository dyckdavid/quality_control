import Foundation

final class APIClient {
    var baseURL: URL
    var accessToken: String?
    var refreshToken: String?
    var onSessionExpired: (() -> Void)?
    var onTokensRefreshed: (() -> Void)?

    private let session: URLSession
    private var refreshing = false

    init(baseURL: URL) {
        self.baseURL = baseURL
        let configuration = URLSessionConfiguration.default
        configuration.timeoutIntervalForRequest = 30
        configuration.waitsForConnectivity = true
        session = URLSession(configuration: configuration)
    }

    func health() async throws -> HealthStatus {
        try await send(method: "GET", path: "/healthz", body: nil, authorized: false, retry: false, as: HealthStatus.self)
    }

    func login(username: String, password: String) async throws -> SessionPayload {
        let payload = try await send(
            method: "POST",
            path: "/api/v1/auth/login",
            body: ["username": username, "password": password],
            authorized: false,
            retry: false,
            as: Envelope<SessionPayload>.self
        )
        return payload.data
    }

    func refresh() async throws -> SessionPayload {
        guard let refreshToken else {
            throw APIFailure(status: 401, message: "Sign in again.")
        }
        let payload = try await send(
            method: "POST",
            path: "/api/v1/auth/refresh",
            body: ["refresh_token": refreshToken],
            authorized: false,
            retry: false,
            as: Envelope<SessionPayload>.self
        )
        return payload.data
    }

    func logout() async {
        guard let refreshToken else { return }
        _ = try? await send(
            method: "POST",
            path: "/api/v1/auth/logout",
            body: ["refresh_token": refreshToken],
            authorized: false,
            retry: false,
            as: EmptyResponse.self
        )
    }

    func get<T: Decodable>(_ path: String, query: [String: String] = [:]) async throws -> T {
        try await send(method: "GET", path: path, query: query, body: nil, as: T.self)
    }

    func post<T: Decodable>(_ path: String, body: [String: Any] = [:]) async throws -> T {
        try await send(method: "POST", path: path, body: body, as: T.self)
    }

    func patch<T: Decodable>(_ path: String, body: [String: Any]) async throws -> T {
        try await send(method: "PATCH", path: path, body: body, as: T.self)
    }

    func delete(_ path: String) async throws {
        _ = try await send(method: "DELETE", path: path, body: nil, as: EmptyResponse.self)
    }

    func execute(method: String = "POST", path: String, body: [String: Any]? = nil) async throws {
        _ = try await send(method: method, path: path, body: body, as: EmptyResponse.self)
    }

    func upload(entityType: String, entityId: UUID, fileName: String, mimeType: String, data: Data) async throws -> Attachment {
        let boundary = "qc-\(UUID().uuidString)"
        var payload = Data()
        func append(_ string: String) {
            payload.append(Data(string.utf8))
        }
        append("--\(boundary)\r\nContent-Disposition: form-data; name=\"entity_type\"\r\n\r\n\(entityType)\r\n")
        append("--\(boundary)\r\nContent-Disposition: form-data; name=\"entity_id\"\r\n\r\n\(entityId.uuidString)\r\n")
        append("--\(boundary)\r\nContent-Disposition: form-data; name=\"file\"; filename=\"\(fileName)\"\r\nContent-Type: \(mimeType)\r\n\r\n")
        payload.append(data)
        append("\r\n--\(boundary)--\r\n")
        let envelope: Envelope<Attachment> = try await send(
            method: "POST",
            path: "/api/v1/attachments",
            raw: payload,
            contentType: "multipart/form-data; boundary=\(boundary)",
            as: Envelope<Attachment>.self
        )
        return envelope.data
    }

    func download(id: UUID) async throws -> Data {
        let (data, _) = try await raw(method: "GET", path: "/api/v1/attachments/\(id.uuidString)", body: nil, contentType: nil, authorized: true, retry: true)
        return data
    }

    private func send<T: Decodable>(
        method: String,
        path: String,
        query: [String: String] = [:],
        body: [String: Any]?,
        authorized: Bool = true,
        retry: Bool = true,
        as type: T.Type
    ) async throws -> T {
        let rawBody: Data?
        if let body {
            rawBody = try JSONSerialization.data(withJSONObject: body)
        } else {
            rawBody = nil
        }
        let contentType = body == nil ? nil : "application/json"
        return try await send(method: method, path: path, query: query, raw: rawBody, contentType: contentType, authorized: authorized, retry: retry, as: type)
    }

    private func send<T: Decodable>(
        method: String,
        path: String,
        query: [String: String] = [:],
        raw body: Data?,
        contentType: String?,
        authorized: Bool = true,
        retry: Bool = true,
        as type: T.Type
    ) async throws -> T {
        let (data, response) = try await raw(method: method, path: path, query: query, body: body, contentType: contentType, authorized: authorized, retry: retry)
        if T.self == EmptyResponse.self {
            return EmptyResponse() as! T
        }
        if data.isEmpty {
            throw APIFailure(status: response.statusCode, message: "The server returned an empty response.")
        }
        do {
            return try APICoding.decoder().decode(T.self, from: data)
        } catch {
            throw APIFailure(status: response.statusCode, message: "The server response was not one this app understands. Check that the address is the API, not the database.")
        }
    }

    private func raw(
        method: String,
        path: String,
        query: [String: String] = [:],
        body: Data?,
        contentType: String?,
        authorized: Bool,
        retry: Bool
    ) async throws -> (Data, HTTPURLResponse) {
        var components = URLComponents(url: baseURL, resolvingAgainstBaseURL: false)
        let basePath = components?.path ?? ""
        components?.path = basePath + path
        if !query.isEmpty {
            components?.queryItems = query.map { URLQueryItem(name: $0.key, value: $0.value) }
        }
        guard let url = components?.url else {
            throw APIFailure(status: 0, message: "The server address is not valid.")
        }
        var request = URLRequest(url: url)
        request.httpMethod = method
        request.httpBody = body
        if let contentType {
            request.setValue(contentType, forHTTPHeaderField: "Content-Type")
        }
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        if authorized, let accessToken {
            request.setValue("Bearer \(accessToken)", forHTTPHeaderField: "Authorization")
        }

        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await session.data(for: request)
        } catch is CancellationError {
            throw CancellationError()
        } catch let error as URLError where error.code == .cancelled {
            throw error
        } catch {
            throw APIFailure(status: 0, message: "Cannot reach the server at \(baseURL.absoluteString). Check that Docker is running and the address is the API on port 8080.")
        }
        guard let http = response as? HTTPURLResponse else {
            throw APIFailure(status: 0, message: "The server did not answer.")
        }
        if http.statusCode == 401, retry, authorized, refreshToken != nil, !refreshing {
            do {
                try await renew()
            } catch {
                onSessionExpired?()
                throw APIFailure(status: 401, message: "Your session ended. Sign in again.")
            }
            return try await raw(method: method, path: path, query: query, body: body, contentType: contentType, authorized: authorized, retry: false)
        }
        guard (200..<300).contains(http.statusCode) else {
            if http.statusCode == 401, authorized {
                onSessionExpired?()
            }
            throw APIFailure(status: http.statusCode, message: APIFailure.describe(status: http.statusCode, data: data))
        }
        return (data, http)
    }

    private func renew() async throws {
        refreshing = true
        defer { refreshing = false }
        let payload = try await refresh()
        accessToken = payload.accessToken
        refreshToken = payload.refreshToken
        onTokensRefreshed?()
    }
}

nonisolated struct EmptyResponse: Decodable {
    init() {}
    init(from decoder: Decoder) throws {}
}
