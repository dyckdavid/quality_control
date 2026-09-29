import SwiftUI

@main
struct QualityControlApp: App {
    @State private var model = AppModel()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(model)
                .tint(Color(red: 0.11, green: 0.38, blue: 0.40))
        }
    }
}
