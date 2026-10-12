import SwiftUI

struct LoginView: View {
    @Environment(AppModel.self) private var model
    @State private var username = ""
    @State private var password = ""
    @State private var errorMessage: String?
    @State private var connectionNote: String?
    @State private var working = false
    @State private var checking = false
    @State private var appear = false

    var body: some View {
        @Bindable var model = model
        ZStack {
            QCBackground()
            ScrollView {
                VStack(spacing: 28) {
                    VStack(spacing: 10) {
                        Image(systemName: "checkmark.seal.fill")
                            .font(.system(size: 52))
                            .foregroundStyle(QCTheme.brandColor)
                            .symbolEffect(.bounce, value: appear)
                            .accessibilityHidden(true)
                        Text("Quality Control")
                            .font(QCTheme.displayFont)
                            .foregroundStyle(QCTheme.ink)
                        Text("Floor inspections, NCRs, and CAPA in one place.")
                            .font(.subheadline)
                            .foregroundStyle(QCTheme.mutedInk)
                            .multilineTextAlignment(.center)
                    }
                    .padding(.top, 36)
                    .opacity(appear ? 1 : 0)
                    .offset(y: appear ? 0 : 12)

                    VStack(spacing: 16) {
                        VStack(alignment: .leading, spacing: 8) {
                            Text("API server")
                                .font(QCTheme.captionFont)
                                .foregroundStyle(QCTheme.mutedInk)
                            TextField("http://127.0.0.1:8080", text: $model.serverAddress)
                                .textInputAutocapitalization(.never)
                                .autocorrectionDisabled()
                                .keyboardType(.URL)
                                .padding(12)
                                .background(QCTheme.elevated, in: RoundedRectangle(cornerRadius: 12, style: .continuous))
                                .accessibilityLabel("Server URL")
                        }
                        VStack(alignment: .leading, spacing: 8) {
                            Text("Account")
                                .font(QCTheme.captionFont)
                                .foregroundStyle(QCTheme.mutedInk)
                            TextField("Username", text: $username)
                                .textInputAutocapitalization(.never)
                                .autocorrectionDisabled()
                                .padding(12)
                                .background(QCTheme.elevated, in: RoundedRectangle(cornerRadius: 12, style: .continuous))
                                .accessibilityLabel("Username")
                            SecureField("Password", text: $password)
                                .padding(12)
                                .background(QCTheme.elevated, in: RoundedRectangle(cornerRadius: 12, style: .continuous))
                                .accessibilityLabel("Password")
                        }

                        if let connectionNote {
                            Text(connectionNote)
                                .font(.footnote)
                                .foregroundStyle(connectionNote.hasPrefix("The API answered") ? QCTheme.success : QCTheme.amber)
                                .frame(maxWidth: .infinity, alignment: .leading)
                                .accessibilityLabel(connectionNote)
                        }

                        Button {
                            Task { await checkConnection() }
                        } label: {
                            HStack {
                                Text("Check connection")
                                if checking { ProgressView() }
                            }
                            .frame(maxWidth: .infinity)
                        }
                        .buttonStyle(.bordered)
                        .disabled(checking || working || model.serverAddress.isEmpty)
                        .accessibilityLabel("Check connection to API server")

                        Button {
                            Task { await signIn() }
                        } label: {
                            HStack {
                                Text("Sign in")
                                    .fontWeight(.semibold)
                                if working { ProgressView().tint(.white) }
                            }
                            .frame(maxWidth: .infinity)
                            .padding(.vertical, 4)
                        }
                        .buttonStyle(.borderedProminent)
                        .tint(QCTheme.brandColor)
                        .disabled(working || checking || username.isEmpty || password.isEmpty || model.serverAddress.isEmpty)
                        .accessibilityLabel("Sign in")

                        Text("Simulator: http://127.0.0.1:8080. A TestFlight iPhone must use this Mac’s Wi-Fi address and port 8080.")
                            .font(.caption)
                            .foregroundStyle(QCTheme.mutedInk)
                    }
                    .padding(20)
                    .background(QCTheme.elevated.opacity(0.92), in: RoundedRectangle(cornerRadius: 22, style: .continuous))
                    .overlay(
                        RoundedRectangle(cornerRadius: 22, style: .continuous)
                            .stroke(QCTheme.brandColor.opacity(0.12), lineWidth: 1)
                    )
                    .padding(.horizontal, 20)
                    .frame(maxWidth: 520)
                }
                .frame(maxWidth: .infinity)
                .padding(.bottom, 40)
            }
        }
        .errorAlert($errorMessage)
        .onAppear {
            if username.isEmpty { username = model.lastUsername }
            if let notice = model.notice { errorMessage = notice }
            withAnimation(.spring(response: 0.55, dampingFraction: 0.86)) {
                appear = true
            }
        }
    }

    private func checkConnection() async {
        checking = true
        defer { checking = false }
        connectionNote = await model.probe()
    }

    private func signIn() async {
        working = true
        defer { working = false }
        do {
            try await model.login(username: username, password: password)
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}

struct AccountView: View {
    @Environment(AppModel.self) private var model
    @State private var currentPassword = ""
    @State private var newPassword = ""
    @State private var errorMessage: String?
    @State private var saved = false
    @State private var confirmSignOut = false

    var body: some View {
        Form {
            if let user = model.user {
                Section("Profile") {
                    LabeledContent("Name", value: user.fullName)
                    LabeledContent("Username", value: user.username)
                    LabeledContent("Email", value: user.email)
                    LabeledContent("Role", value: user.roleValue.title)
                }
            }
            Section("Change password") {
                SecureField("Current password", text: $currentPassword)
                    .accessibilityLabel("Current password")
                SecureField("New password", text: $newPassword)
                    .accessibilityLabel("New password")
                Button("Update password") {
                    Task { await changePassword() }
                }
                .disabled(currentPassword.isEmpty || newPassword.count < 10)
            }
            Section {
                Button("Sign out", role: .destructive) { confirmSignOut = true }
                    .accessibilityLabel("Sign out")
            }
            .confirmationDialog("Sign out of this device?", isPresented: $confirmSignOut, titleVisibility: .visible) {
                Button("Sign out", role: .destructive) { Task { await model.logout() } }
                Button("Stay signed in", role: .cancel) {}
            }
        }
        .qcListChrome()
        .navigationTitle("Account")
        .errorAlert($errorMessage)
        .alert("Password updated", isPresented: $saved) {
            Button("OK", role: .cancel) {}
        }
    }

    private func changePassword() async {
        do {
            try await model.client.execute(path: "/api/v1/me/password", body: [
                "current_password": currentPassword,
                "new_password": newPassword
            ])
            currentPassword = ""
            newPassword = ""
            saved = true
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}

struct SettingsView: View {
    @Environment(AppModel.self) private var model
    @State private var draftServer = ""
    @State private var note: String?
    @State private var errorMessage: String?
    @State private var clearing = false

    var body: some View {
        @Bindable var model = model
        Form {
            Section {
                TextField("Server URL", text: $draftServer)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .keyboardType(.URL)
                    .accessibilityLabel("Server URL")
                Button("Save server URL") {
                    model.serverAddress = draftServer
                    do {
                        try model.applyServerAddress()
                        note = "Server updated to \(model.serverAddress)."
                    } catch {
                        errorMessage = error.localizedDescription
                    }
                }
                .disabled(draftServer.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                Button("Test connection") {
                    Task {
                        model.serverAddress = draftServer
                        note = await model.probe()
                    }
                }
                if let note {
                    Text(note)
                        .font(.footnote)
                        .foregroundStyle(note.contains("answered") || note.hasPrefix("Server updated") ? QCTheme.success : QCTheme.amber)
                }
            } header: {
                Text("API server")
            } footer: {
                Text("Use http://127.0.0.1:8080 in the simulator, or your Mac’s LAN address on a physical device. Port 5433 is PostgreSQL, not this app.")
            }

            Section("Offline") {
                LabeledContent("Network", value: model.offline.isOnline ? "Online" : "Offline")
                LabeledContent("Queued writes", value: "\(model.offline.pendingCount)")
                if let flushError = model.offline.lastFlushError {
                    Text(flushError)
                        .font(.footnote)
                        .foregroundStyle(QCTheme.amber)
                }
                Button("Sync now") {
                    Task { await model.syncPending() }
                }
                .disabled(!model.offline.isOnline)
                Button("Clear local cache", role: .destructive) {
                    clearing = true
                    model.offline.clearCache()
                    clearing = false
                    note = "Local list cache cleared."
                }
                .disabled(clearing)
            }

            Section("Appearance") {
                LabeledContent("Theme", value: "System (light & dark)")
                LabeledContent("Accent", value: "Teal")
            }
        }
        .qcListChrome()
        .navigationTitle("Settings")
        .onAppear { draftServer = model.serverAddress }
        .errorAlert($errorMessage)
    }
}
