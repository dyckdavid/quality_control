import SwiftUI
import UIKit
import UniformTypeIdentifiers

struct AttachmentsSection: View {
    @Environment(AppModel.self) private var model
    var entityType: String
    var entityId: UUID
    var canUpload: Bool
    var canDelete: Bool

    @State private var files: [Attachment] = []
    @State private var errorMessage: String?
    @State private var importing = false
    @State private var preview: PreviewFile?
    @State private var pendingDelete: Attachment?

    var body: some View {
        Section("Files") {
            if files.isEmpty {
                Text("No files")
                    .foregroundStyle(.secondary)
            }
            ForEach(files) { file in
                Button {
                    Task { await open(file) }
                } label: {
                    VStack(alignment: .leading) {
                        Text(file.fileName)
                        Text(file.contentType)
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
                .swipeActions {
                    if canDelete {
                        Button("Delete", role: .destructive) {
                            pendingDelete = file
                        }
                    }
                }
            }
            if canUpload {
                Button("Attach a file") { importing = true }
            }
        }
        .task { await reload() }
        .fileImporter(isPresented: $importing, allowedContentTypes: [.jpeg, .png, .pdf, .plainText, .webP]) { result in
            Task { await upload(result) }
        }
        .sheet(item: $preview) { file in
            NavigationStack {
                VStack {
                    if let image = UIImage(data: file.data) {
                        Image(uiImage: image)
                            .resizable()
                            .scaledToFit()
                    } else {
                        ShareLink(item: file.url) {
                            Label("Share \(file.name)", systemImage: "square.and.arrow.up")
                        }
                    }
                }
                .padding()
                .navigationTitle(file.name)
                .navigationBarTitleDisplayMode(.inline)
                .toolbar {
                    ToolbarItem(placement: .cancellationAction) {
                        Button("Close") { preview = nil }
                    }
                }
            }
        }
        .confirmationDialog("Remove this file?", isPresented: Binding(
            get: { pendingDelete != nil },
            set: { if !$0 { pendingDelete = nil } }
        ), titleVisibility: .visible) {
            Button("Remove", role: .destructive) {
                if let pendingDelete { Task { await remove(pendingDelete) } }
            }
            Button("Keep it", role: .cancel) {}
        } message: {
            Text(pendingDelete?.fileName ?? "")
        }
        .errorAlert($errorMessage)
    }

    private func reload() async {
        do {
            files = try await model.client.one("/api/v1/attachments", query: [
                "entity_type": entityType,
                "entity_id": entityId.uuidString
            ])
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func upload(_ result: Result<URL, Error>) async {
        do {
            let url = try result.get()
            guard url.startAccessingSecurityScopedResource() else {
                errorMessage = "Cannot read that file."
                return
            }
            defer { url.stopAccessingSecurityScopedResource() }
            let data = try Data(contentsOf: url)
            if data.isEmpty {
                errorMessage = "That file is empty."
                return
            }
            if data.count > 25 * 1024 * 1024 {
                errorMessage = "That file is larger than 25 MB."
                return
            }
            let mime = mimeType(for: url)
            guard ["image/jpeg", "image/png", "image/webp", "application/pdf", "text/plain"].contains(mime) else {
                errorMessage = "Use a JPEG, PNG, WebP, PDF, or plain text file."
                return
            }
            _ = try await model.client.upload(entityType: entityType, entityId: entityId, fileName: url.lastPathComponent, mimeType: mime, data: data)
            await reload()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func open(_ file: Attachment) async {
        do {
            let data = try await model.client.download(id: file.id)
            let url = FileManager.default.temporaryDirectory.appendingPathComponent(file.fileName)
            try data.write(to: url)
            preview = PreviewFile(name: file.fileName, data: data, url: url)
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func remove(_ file: Attachment) async {
        do {
            try await model.client.delete("/api/v1/attachments/\(file.id.uuidString)")
            await reload()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func mimeType(for url: URL) -> String {
        switch url.pathExtension.lowercased() {
        case "jpg", "jpeg": "image/jpeg"
        case "png": "image/png"
        case "webp": "image/webp"
        case "pdf": "application/pdf"
        case "txt": "text/plain"
        default: "application/octet-stream"
        }
    }
}

struct PreviewFile: Identifiable {
    var id: String { name }
    var name: String
    var data: Data
    var url: URL
}

extension UTType {
    static var webP: UTType { UTType(filenameExtension: "webp") ?? .data }
}
