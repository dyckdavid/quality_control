import CoreImage.CIFilterBuiltins
import SwiftUI
import UIKit

enum QRCode {
    /// A crisp QR code for `text`, or nil when it can't be encoded.
    static func image(_ text: String, size: CGFloat = 600) -> UIImage? {
        let filter = CIFilter.qrCodeGenerator()
        filter.message = Data(text.utf8)
        filter.correctionLevel = "M"
        guard let output = filter.outputImage, output.extent.width > 0 else { return nil }
        let scale = size / output.extent.width
        let scaled = output.transformed(by: CGAffineTransform(scaleX: scale, y: scale))
        guard let cg = CIContext().createCGImage(scaled, from: scaled.extent) else { return nil }
        return UIImage(cgImage: cg)
    }
}

/// A 2 by 1 inch shop label: the QR code plus the code in big type, so it reads by eye too.
struct CodeLabel: View {
    var code: String
    var title: String
    var subtitle: String

    var body: some View {
        HStack(spacing: 14) {
            if let qr = QRCode.image(code) {
                Image(uiImage: qr)
                    .interpolation(.none)
                    .resizable()
                    .frame(width: 120, height: 120)
            }
            VStack(alignment: .leading, spacing: 4) {
                Text(code)
                    .font(.system(size: 26, weight: .heavy, design: .monospaced))
                    .minimumScaleFactor(0.4)
                    .lineLimit(1)
                if !title.isEmpty {
                    Text(title)
                        .font(.system(size: 15, weight: .semibold))
                        .lineLimit(2)
                }
                if !subtitle.isEmpty {
                    Text(subtitle)
                        .font(.system(size: 12))
                        .lineLimit(1)
                }
            }
            .foregroundStyle(.black)
            Spacer(minLength: 0)
        }
        .padding(12)
        .frame(width: 320, height: 150)
        .background(.white)
    }
}

/// Preview of a record's label with share and print.
struct LabelSection: View {
    var code: String
    var title: String
    var subtitle: String
    @State private var image: UIImage?

    var body: some View {
        Section {
            CodeLabel(code: code, title: title, subtitle: subtitle)
                .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
                .overlay(RoundedRectangle(cornerRadius: 10, style: .continuous).stroke(QCTheme.line))
                .frame(maxWidth: .infinity)
                .accessibilityLabel("Label for \(code)")
            if let image {
                HStack {
                    ShareLink(item: Image(uiImage: image), preview: SharePreview(code, image: Image(uiImage: image))) {
                        Label("Share", systemImage: "square.and.arrow.up")
                    }
                    Spacer()
                    Button {
                        print(image)
                    } label: {
                        Label("Print", systemImage: "printer")
                    }
                }
                .buttonStyle(.borderless)
            }
        } header: {
            Text("Label")
        } footer: {
            Text("Stick it on the part or gauge. Scanning it opens this record.")
        }
        .task(id: code + title + subtitle) { image = render() }
    }

    private func render() -> UIImage? {
        let renderer = ImageRenderer(content: CodeLabel(code: code, title: title, subtitle: subtitle))
        renderer.scale = 3
        return renderer.uiImage
    }

    private func print(_ image: UIImage) {
        let info = UIPrintInfo(dictionary: nil)
        info.outputType = .photo
        info.jobName = "Label \(code)"
        let controller = UIPrintInteractionController.shared
        controller.printInfo = info
        controller.printingItem = image
        controller.present(animated: true)
    }
}
