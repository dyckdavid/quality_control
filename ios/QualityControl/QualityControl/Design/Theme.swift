import SwiftUI

#if canImport(UIKit)
import UIKit
#endif

/// Shop design tokens, matched to the desktop app: teal for go, copper for the shop's own accent.
enum QCTheme {
    static let brandColor = adaptive(light: (15, 110, 98), dark: (45, 168, 148))
    static let brandDeep = adaptive(light: (10, 70, 63), dark: (18, 48, 44))
    static let brandSoft = adaptive(light: (226, 242, 238), dark: (18, 48, 44))
    static let copper = adaptive(light: (180, 62, 28), dark: (242, 125, 78))
    static let copperSoft = adaptive(light: (252, 236, 228), dark: (58, 32, 24))
    static let amber = adaptive(light: (161, 98, 7), dark: (232, 176, 74))
    static let success = adaptive(light: (21, 128, 61), dark: (74, 196, 120))
    static let danger = adaptive(light: (185, 28, 28), dark: (248, 113, 113))
    static let info = adaptive(light: (37, 84, 158), dark: (126, 168, 232))

    static let canvas = adaptive(light: (244, 245, 247), dark: (14, 16, 20))
    static let elevated = adaptive(light: (255, 255, 255), dark: (24, 27, 34))
    static let line = adaptive(light: (226, 228, 233), dark: (46, 51, 61))
    static let ink = adaptive(light: (17, 19, 24), dark: (244, 245, 247))
    static let mutedInk = adaptive(light: (92, 99, 112), dark: (154, 160, 171))

    static var brandGradient: LinearGradient {
        LinearGradient(colors: [brandColor, brandDeep], startPoint: .topLeading, endPoint: .bottomTrailing)
    }

    static var copperGradient: LinearGradient {
        LinearGradient(colors: [copper, copper.opacity(0.75)], startPoint: .topLeading, endPoint: .bottomTrailing)
    }

    static let displayFont: Font = .system(.largeTitle, design: .rounded).weight(.bold)
    static let titleFont: Font = .system(.title2, design: .rounded).weight(.semibold)
    static let headlineFont: Font = .system(.headline, design: .rounded)
    static let bodyFont: Font = .system(.body, design: .default)
    static let captionFont: Font = .system(.caption, design: .rounded).weight(.medium)

    static func statusColor(_ raw: String) -> Color {
        switch raw {
        case "pass", "accepted", "active", "closed":
            success
        case "fail", "rejected", "critical", "void", "cancelled":
            danger
        case "conditional", "on_hold", "major", "under_review", "verification", "overdue":
            amber
        case "in_progress", "in_inspection", "dispositioned", "open":
            info
        default:
            mutedInk
        }
    }

    private static func adaptive(light: (Double, Double, Double), dark: (Double, Double, Double)) -> Color {
        #if canImport(UIKit)
        Color(uiColor: UIColor { traits in
            let c = traits.userInterfaceStyle == .dark ? dark : light
            return UIColor(red: c.0 / 255, green: c.1 / 255, blue: c.2 / 255, alpha: 1)
        })
        #else
        Color(red: light.0 / 255, green: light.1 / 255, blue: light.2 / 255)
        #endif
    }
}

struct QCBackground: View {
    var body: some View {
        ZStack(alignment: .top) {
            QCTheme.canvas
            LinearGradient(
                colors: [QCTheme.brandColor.opacity(0.10), .clear],
                startPoint: .top,
                endPoint: .center
            )
        }
        .ignoresSafeArea()
    }
}

/// A rounded card on the canvas.
struct QCCard<Content: View>: View {
    var padding: CGFloat = 16
    @ViewBuilder var content: Content

    var body: some View {
        content
            .padding(padding)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(QCTheme.elevated, in: RoundedRectangle(cornerRadius: 18, style: .continuous))
            .overlay(
                RoundedRectangle(cornerRadius: 18, style: .continuous)
                    .stroke(QCTheme.line, lineWidth: 1)
            )
    }
}
