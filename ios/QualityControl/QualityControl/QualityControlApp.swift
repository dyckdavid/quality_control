import SwiftUI

@main
struct QualityControlApp: App {
    @State private var model = AppModel()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(model)
                .tint(QCTheme.brandColor)
                .preferredColorScheme(nil)
        }
    }
}
