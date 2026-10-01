import Foundation
#if canImport(UIKit)
import UIKit
#endif

// ============================================================
// MARK: - Plataforma: lo poco que depende del sistema
// ============================================================
// Todo lo que toca UIKit vive aquí. En iPad usa el sistema; en
// Linux/Debian (o en pruebas) usa equivalentes de texto, así la
// shell entera compila y funciona en cualquier sitio.

enum Plataforma {
    #if !canImport(UIKit)
    nonisolated(unsafe) private static var portapapeles = ""
    #endif

    static func copiar(_ t: String) async {
        #if canImport(UIKit)
        await MainActor.run { UIPasteboard.general.string = t }
        #else
        portapapeles = t
        #endif
    }

    static func pegar() async -> String {
        #if canImport(UIKit)
        return await MainActor.run { UIPasteboard.general.string ?? "" }
        #else
        return portapapeles
        #endif
    }

    static func dispositivo() async -> (nombre: String, modelo: String, sistema: String, version: String, pantalla: String) {
        #if canImport(UIKit)
        return await MainActor.run {
            let d = UIDevice.current
            let b = UIScreen.main.bounds
            return (d.name, d.model, d.systemName, d.systemVersion, "\(Int(b.width))x\(Int(b.height))")
        }
        #else
        let p = ProcessInfo.processInfo
        return (p.hostName, "computadora", "Linux", p.operatingSystemVersionString, "texto")
        #endif
    }

    /// Huella de 64 bits (cuando no hay CryptoKit).
    static func fnv64(_ d: Data) -> String {
        var h: UInt64 = 0xcbf29ce484222325
        for b in d { h ^= UInt64(b); h = h &* 0x100000001b3 }
        return String(format: "%016llx", h)
    }
}
