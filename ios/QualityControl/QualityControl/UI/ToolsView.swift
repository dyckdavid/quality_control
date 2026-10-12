import SwiftUI
import UIKit

struct ToolsView: View {
    @Environment(AppModel.self) private var model
    @State private var report = ""
    @State private var checking = false
    @State private var copied = false

    var body: some View {
        Form {
            Section("This install") {
                LabeledContent("Version", value: Self.version)
                LabeledContent("Build", value: Self.build)
                LabeledContent("System", value: Self.system)
                LabeledContent("Role", value: model.user?.roleValue.title ?? "Signed out")
                LabeledContent("Server", value: model.serverAddress)
            }
            Section("Connection") {
                Button {
                    Task { await check() }
                } label: {
                    HStack {
                        Text("Check the API")
                        Spacer()
                        if checking { ProgressView() }
                    }
                }
                .disabled(checking)
                if !report.isEmpty {
                    Text(report)
                        .font(.footnote)
                }
                Button("Copy diagnostics") { copy() }
                    .disabled(report.isEmpty)
            }
            Section("What each role can try") {
                LabeledContent("Viewer", value: "Read every list")
                LabeledContent("Operator", value: "Lots, results, files")
                LabeledContent("Inspector", value: "Inspections and NCRs")
                LabeledContent("Supervisor", value: "Disposition, actions, audit")
                LabeledContent("Quality manager", value: "Plans, products, close actions")
                LabeledContent("Admin", value: "Users")
            }
            Section("TestFlight") {
                Text("127.0.0.1 is the phone itself, so a TestFlight device cannot use it. Set the server to http:// and this Mac’s Wi-Fi address, then port 8080. The phone and the Mac have to be on the same network, and Docker has to be running.")
                Text("Sign in as admin with password change-me-now, or as viewer1, operator1, inspector1, supervisor1, or quality1 with password Role-test-1.")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            }
        }
        .qcListChrome()
        .navigationTitle("Tester tools")
        .alert("Copied", isPresented: $copied) {
            Button("OK", role: .cancel) {}
        } message: {
            Text("Paste the diagnostics into your test notes.")
        }
    }

    private func check() async {
        checking = true
        defer { checking = false }
        report = await model.probe()
    }

    private func copy() {
        let text = """
        Quality Control \(Self.version) (\(Self.build))
        \(Self.system)
        Role: \(model.user?.roleValue.title ?? "signed out")
        Server: \(model.serverAddress)
        \(report)
        """
        UIPasteboard.general.string = text
        copied = true
    }

    private static var version: String {
        Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "1.0"
    }

    private static var build: String {
        Bundle.main.object(forInfoDictionaryKey: "CFBundleVersion") as? String ?? "1"
    }

    private static var system: String {
        "\(UIDevice.current.systemName) \(UIDevice.current.systemVersion)"
    }
}
