import SwiftUI
import PhotosUI
import UniformTypeIdentifiers
import CoreTransferable

#if canImport(UIKit)
import UIKit
#endif

#if os(iOS)
struct CameraPicker: UIViewControllerRepresentable {
    @Binding var image: UIImage?
    @Environment(\.dismiss) private var dismiss

    func makeUIViewController(context: Context) -> UIImagePickerController {
        let picker = UIImagePickerController()
        picker.sourceType = UIImagePickerController.isSourceTypeAvailable(.camera) ? .camera : .photoLibrary
        picker.delegate = context.coordinator
        picker.allowsEditing = false
        return picker
    }

    func updateUIViewController(_ uiViewController: UIImagePickerController, context: Context) {}

    func makeCoordinator() -> Coordinator {
        Coordinator(parent: self)
    }

    final class Coordinator: NSObject, UINavigationControllerDelegate, UIImagePickerControllerDelegate {
        let parent: CameraPicker

        init(parent: CameraPicker) {
            self.parent = parent
        }

        func imagePickerControllerDidCancel(_ picker: UIImagePickerController) {
            parent.dismiss()
        }

        func imagePickerController(
            _ picker: UIImagePickerController,
            didFinishPickingMediaWithInfo info: [UIImagePickerController.InfoKey: Any]
        ) {
            parent.image = info[.originalImage] as? UIImage
            parent.dismiss()
        }
    }
}
#endif

struct PhotoCaptureSection: View {
    @Environment(AppModel.self) private var model
    var entityType: String
    var entityId: UUID
    var canCapture: Bool

    @State private var locals: [LocalPhoto] = []
    @State private var showCamera = false
    @State private var libraryItem: PhotosPickerItem?
    @State private var errorMessage: String?
    @State private var previewData: Data?
    @State private var uploading = false

    var body: some View {
        Section("Photos") {
            if locals.isEmpty {
                Text("No photos yet")
                    .foregroundStyle(QCTheme.mutedInk)
                    .accessibilityLabel("No photos yet")
            }
            ScrollView(.horizontal, showsIndicators: false) {
                HStack(spacing: 10) {
                    ForEach(locals) { photo in
                        if let data = LocalPhotoStore.shared.data(for: photo) {
                            Button {
                                previewData = data
                            } label: {
                                photoThumbnail(data: data, uploaded: photo.uploaded)
                            }
                            .accessibilityLabel("Photo captured \(photo.createdAt.formatted(date: .abbreviated, time: .shortened))")
                            .contextMenu {
                                if !photo.uploaded, model.user?.allows(.operator) == true {
                                    Button("Upload to server") {
                                        Task { await upload(photo) }
                                    }
                                }
                                Button("Delete", role: .destructive) {
                                    LocalPhotoStore.shared.delete(photo)
                                    reload()
                                }
                            }
                        }
                    }
                }
                .padding(.vertical, 4)
            }
            if canCapture {
                HStack {
                    #if os(iOS)
                    Button {
                        showCamera = true
                    } label: {
                        Label("Camera", systemImage: "camera.fill")
                    }
                    .accessibilityLabel("Capture photo with camera")
                    .disabled(uploading)
                    #endif

                    PhotosPicker(selection: $libraryItem, matching: .images) {
                        Label("Library", systemImage: "photo.on.rectangle")
                    }
                    .accessibilityLabel("Choose photo from library")
                    .disabled(uploading)
                }
            }
        }
        .task { reload() }
        .onChange(of: libraryItem) { _, item in
            guard let item else { return }
            Task {
                if let loaded = try? await item.loadTransferable(type: QCImageData.self) {
                    await persist(data: loaded.data)
                }
                libraryItem = nil
            }
        }
        #if os(iOS)
        .fullScreenCover(isPresented: $showCamera) {
            CameraImageBridge { data in
                Task { await persist(data: data) }
            }
        }
        #endif
        .sheet(isPresented: Binding(
            get: { previewData != nil },
            set: { if !$0 { previewData = nil } }
        )) {
            NavigationStack {
                Group {
                    if let previewData {
                        #if canImport(UIKit)
                        if let image = UIImage(data: previewData) {
                            Image(uiImage: image)
                                .resizable()
                                .scaledToFit()
                                .padding()
                        } else {
                            Text("Cannot preview this photo.")
                                .padding()
                        }
                        #else
                        Text("Photo preview")
                            .padding()
                        #endif
                    }
                }
                .navigationTitle("Photo")
                #if os(iOS)
                .navigationBarTitleDisplayMode(.inline)
                #endif
                .toolbar {
                    ToolbarItem(placement: .cancellationAction) {
                        Button("Close") { self.previewData = nil }
                    }
                }
            }
        }
        .errorAlert($errorMessage)
    }

    @ViewBuilder
    private func photoThumbnail(data: Data, uploaded: Bool) -> some View {
        #if canImport(UIKit)
        if let image = UIImage(data: data) {
            Image(uiImage: image)
                .resizable()
                .scaledToFill()
                .frame(width: 88, height: 88)
                .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
                .overlay(alignment: .topTrailing) {
                    if !uploaded {
                        Image(systemName: "icloud.slash")
                            .font(.caption2.weight(.bold))
                            .padding(4)
                            .background(.ultraThinMaterial, in: Circle())
                            .padding(4)
                    }
                }
        } else {
            RoundedRectangle(cornerRadius: 12, style: .continuous)
                .fill(QCTheme.elevated)
                .frame(width: 88, height: 88)
        }
        #else
        RoundedRectangle(cornerRadius: 12, style: .continuous)
            .fill(QCTheme.elevated)
            .frame(width: 88, height: 88)
            .overlay {
                Image(systemName: "photo")
            }
        #endif
    }

    private func reload() {
        locals = LocalPhotoStore.shared.photos(entityType: entityType, entityId: entityId)
    }

    private func persist(data: Data) async {
        uploading = true
        defer { uploading = false }
        #if canImport(UIKit)
        guard let image = UIImage(data: data) else {
            errorMessage = "Could not read that photo."
            return
        }
        do {
            let photo = try LocalPhotoStore.shared.save(image: image, entityType: entityType, entityId: entityId)
            reload()
            await upload(photo)
        } catch {
            errorMessage = error.localizedDescription
        }
        #else
        errorMessage = "Photo capture is not available on this platform."
        #endif
    }

    private func upload(_ photo: LocalPhoto) async {
        guard let data = LocalPhotoStore.shared.data(for: photo) else { return }
        do {
            _ = try await model.client.upload(
                entityType: entityType,
                entityId: entityId,
                fileName: photo.fileName,
                mimeType: "image/jpeg",
                data: data
            )
            LocalPhotoStore.shared.markUploaded(photo)
            reload()
        } catch {
            errorMessage = "Saved on this device. Upload when the server is reachable."
        }
    }
}

nonisolated struct QCImageData: Transferable {
    let data: Data

    static var transferRepresentation: some TransferRepresentation {
        DataRepresentation(importedContentType: .jpeg) { data in
            QCImageData(data: data)
        }
        DataRepresentation(importedContentType: .png) { data in
            QCImageData(data: data)
        }
        DataRepresentation(importedContentType: .heic) { data in
            QCImageData(data: data)
        }
    }
}

#if os(iOS)
private struct CameraImageBridge: View {
    var onCapture: (Data) -> Void
    @State private var image: UIImage?

    var body: some View {
        CameraPicker(image: $image)
            .ignoresSafeArea()
            .onChange(of: image) { _, value in
                guard let value, let data = value.jpegData(compressionQuality: 0.82) else { return }
                onCapture(data)
            }
    }
}
#endif
