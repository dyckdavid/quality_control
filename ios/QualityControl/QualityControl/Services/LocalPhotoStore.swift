import Foundation

#if canImport(UIKit)
import UIKit
#endif

nonisolated struct LocalPhoto: Identifiable, Hashable, Codable {
    var id: UUID
    var entityType: String
    var entityId: UUID
    var fileName: String
    var createdAt: Date
    var uploaded: Bool
}

/// Stores inspection/entity photos on disk when the API upload is unavailable.
@MainActor
final class LocalPhotoStore {
    static let shared = LocalPhotoStore()

    private let root: URL
    private let indexURL: URL
    private let encoder = JSONEncoder()
    private let decoder = JSONDecoder()

    private init() {
        let base = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first
            ?? FileManager.default.temporaryDirectory
        root = base.appendingPathComponent("QCPhotos", isDirectory: true)
        indexURL = root.appendingPathComponent("index.json")
        try? FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        encoder.dateEncodingStrategy = .iso8601
        decoder.dateDecodingStrategy = .iso8601
    }

    func photos(entityType: String, entityId: UUID) -> [LocalPhoto] {
        loadIndex().filter { $0.entityType == entityType && $0.entityId == entityId }
            .sorted { $0.createdAt > $1.createdAt }
    }

    #if canImport(UIKit)
    func save(image: UIImage, entityType: String, entityId: UUID) throws -> LocalPhoto {
        guard let data = image.jpegData(compressionQuality: 0.82) else {
            throw APIFailure(status: 0, message: "Could not encode that photo.")
        }
        return try save(data: data, entityType: entityType, entityId: entityId)
    }
    #endif

    func save(data: Data, entityType: String, entityId: UUID) throws -> LocalPhoto {
        let id = UUID()
        let fileName = "\(id.uuidString).jpg"
        let url = root.appendingPathComponent(fileName)
        try data.write(to: url, options: .atomic)
        let photo = LocalPhoto(
            id: id,
            entityType: entityType,
            entityId: entityId,
            fileName: fileName,
            createdAt: Date(),
            uploaded: false
        )
        var index = loadIndex()
        index.append(photo)
        saveIndex(index)
        return photo
    }

    #if canImport(UIKit)
    func image(for photo: LocalPhoto) -> UIImage? {
        guard let data = data(for: photo) else { return nil }
        return UIImage(data: data)
    }
    #endif

    func data(for photo: LocalPhoto) -> Data? {
        try? Data(contentsOf: root.appendingPathComponent(photo.fileName))
    }

    func markUploaded(_ photo: LocalPhoto) {
        var index = loadIndex()
        if let idx = index.firstIndex(where: { $0.id == photo.id }) {
            index[idx].uploaded = true
            saveIndex(index)
        }
    }

    func delete(_ photo: LocalPhoto) {
        try? FileManager.default.removeItem(at: root.appendingPathComponent(photo.fileName))
        var index = loadIndex()
        index.removeAll { $0.id == photo.id }
        saveIndex(index)
    }

    func pendingUploads() -> [LocalPhoto] {
        loadIndex().filter { !$0.uploaded }
    }

    private func loadIndex() -> [LocalPhoto] {
        guard let data = try? Data(contentsOf: indexURL),
              let items = try? decoder.decode([LocalPhoto].self, from: data) else {
            return []
        }
        return items
    }

    private func saveIndex(_ items: [LocalPhoto]) {
        guard let data = try? encoder.encode(items) else { return }
        try? data.write(to: indexURL, options: .atomic)
    }
}
