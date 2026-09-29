import Foundation

nonisolated enum APICoding {
    static func decoder() -> JSONDecoder {
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        decoder.dateDecodingStrategy = .custom { decoder in
            let container = try decoder.singleValueContainer()
            let value = try container.decode(String.self)
            if let date = parseDate(value) {
                return date
            }
            throw DecodingError.dataCorruptedError(in: container, debugDescription: "Unrecognized date \(value)")
        }
        return decoder
    }

    static func parseDate(_ value: String) -> Date? {
        let fractional = ISO8601DateFormatter()
        fractional.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        if let date = fractional.date(from: value) {
            return date
        }
        let plain = ISO8601DateFormatter()
        plain.formatOptions = [.withInternetDateTime]
        return plain.date(from: value)
    }
}

nonisolated struct APIFailure: LocalizedError {
    var status: Int
    var message: String

    var errorDescription: String? { message }

    static func describe(status: Int, data: Data) -> String {
        if let server = try? APICoding.decoder().decode(ServerErrorBody.self, from: data), !server.error.message.isEmpty {
            return server.error.message
        }
        let snippet = String(data: data.prefix(120), encoding: .utf8) ?? ""
        if snippet.contains("<html") || snippet.contains("<!DOCTYPE") {
            return "That address is not the quality-control API. On the simulator use http://127.0.0.1:8080."
        }
        switch status {
        case 401: return "Your session ended. Sign in again."
        case 403: return "Your role cannot do that."
        case 404: return "That record is no longer on the server."
        case 409: return "That name or number is already in use."
        case 413: return "That file is too large. The limit is 25 MB."
        case 422: return "The server rejected those values. Check the required fields."
        case 429: return "Too many sign-in attempts. Wait a few minutes."
        case 500...599: return "The server had a problem. Try again. If it keeps happening, restart Docker."
        default: return "Request failed (\(status))."
        }
    }
}

nonisolated struct PageMeta: Decodable {
    var limit: Int
    var offset: Int
    var total: Int
}

nonisolated struct Page<T: Decodable>: Decodable {
    var data: [T]
    var page: PageMeta
}

nonisolated struct Envelope<T: Decodable>: Decodable {
    var data: T
}

nonisolated struct HealthStatus: Decodable {
    var status: String
}

nonisolated struct ServerErrorBody: Decodable {
    nonisolated struct Detail: Decodable {
        var code: String
        var message: String
    }
    var error: Detail
}
