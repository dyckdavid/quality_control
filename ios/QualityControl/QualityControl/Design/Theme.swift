import SwiftUI

#if canImport(UIKit)
import UIKit
#endif

/// Plant-floor design tokens: slate + teal + amber. Adaptive for light and dark mode.
enum QCTheme {
    static let brandColor = Color(red: 0.07, green: 0.42, blue: 0.45)
    static let brandDeep = Color(red: 0.04, green: 0.22, blue: 0.28)
    static let brandSoft = Color(red: 0.16, green: 0.55, blue: 0.58)
    static let amber = Color(red: 0.86, green: 0.52, blue: 0.14)
    static let success = Color(red: 0.16, green: 0.58, blue: 0.36)
    static let danger = Color(red: 0.76, green: 0.22, blue: 0.24)
    static let info = Color(red: 0.18, green: 0.42, blue: 0.72)

    static var canvas: Color {
        #if canImport(UIKit)
        Color(uiColor: UIColor { traits in
            traits.userInterfaceStyle == .dark
                ? UIColor(red: 0.07, green: 0.09, blue: 0.11, alpha: 1)
                : UIColor(red: 0.94, green: 0.96, blue: 0.97, alpha: 1)
        })
        #else
        Color(red: 0.94, green: 0.96, blue: 0.97)
        #endif
    }

    static var elevated: Color {
        #if canImport(UIKit)
        Color(uiColor: UIColor { traits in
            traits.userInterfaceStyle == .dark
                ? UIColor(red: 0.12, green: 0.15, blue: 0.18, alpha: 1)
                : UIColor.white
        })
        #else
        Color.white
        #endif
    }

    static var ink: Color {
        #if canImport(UIKit)
        Color(uiColor: UIColor { traits in
            traits.userInterfaceStyle == .dark
                ? UIColor(red: 0.93, green: 0.95, blue: 0.96, alpha: 1)
                : UIColor(red: 0.10, green: 0.14, blue: 0.16, alpha: 1)
        })
        #else
        Color(red: 0.10, green: 0.14, blue: 0.16)
        #endif
    }

    static var mutedInk: Color {
        #if canImport(UIKit)
        Color(uiColor: UIColor { traits in
            traits.userInterfaceStyle == .dark
                ? UIColor(red: 0.62, green: 0.68, blue: 0.72, alpha: 1)
                : UIColor(red: 0.35, green: 0.42, blue: 0.45, alpha: 1)
        })
        #else
        Color(red: 0.35, green: 0.42, blue: 0.45)
        #endif
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
        case "minor", "draft":
            mutedInk
        default:
            mutedInk
        }
    }
}

struct QCBackground: View {
    var body: some View {
        ZStack {
            QCTheme.canvas
            LinearGradient(
                colors: [
                    QCTheme.brandColor.opacity(0.14),
                    Color.clear,
                    QCTheme.brandDeep.opacity(0.08)
                ],
                startPoint: .topLeading,
                endPoint: .bottomTrailing
            )
            GeometryReader { geo in
                Path { path in
                    let step: CGFloat = 28
                    var x: CGFloat = 0
                    while x < geo.size.width {
                        path.move(to: CGPoint(x: x, y: 0))
                        path.addLine(to: CGPoint(x: x, y: geo.size.height))
                        x += step
                    }
                    var y: CGFloat = 0
                    while y < geo.size.height {
                        path.move(to: CGPoint(x: 0, y: y))
                        path.addLine(to: CGPoint(x: geo.size.width, y: y))
                        y += step
                    }
                }
                .stroke(QCTheme.brandColor.opacity(0.04), lineWidth: 1)
            }
        }
        .ignoresSafeArea()
    }
}
