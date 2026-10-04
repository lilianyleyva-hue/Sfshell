#if canImport(SwiftUI)
import SwiftUI

// NyxApp.swift — aquí empieza la app.

@main
struct NyxApp: App {
    @StateObject private var nyx = NyxModelo()
    @Environment(\.scenePhase) private var fase

    var body: some Scene {
        WindowGroup {
            VistaPrincipal(nyx: nyx)
        }
        .onChange(of: fase) { (f: ScenePhase) in
            if f != .active { nyx.guardaYa() }     // al salir, guarda lo aprendido (ya)
        }
    }
}
#endif
