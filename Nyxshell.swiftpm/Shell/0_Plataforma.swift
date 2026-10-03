import Foundation
#if (canImport(UIKit) && APPLE_COMPLETO)
import UIKit
#endif

// ============================================================
// MARK: - Plataforma: lo poco que depende del sistema
// ============================================================
// Todo lo que toca UIKit vive aquí. En iPad usa el sistema; en
// Linux/Debian (o en pruebas) usa equivalentes de texto, así la
// shell entera compila y funciona en cualquier sitio.

enum Plataforma {
    /// Portapapeles de texto cuando no hay UIKit (Linux/Debian).
    private final class Caja: @unchecked Sendable {
        let l = NSLock()
        var texto = ""
    }
    private static let caja = Caja()

    static func copiar(_ t: String) async {
        #if (canImport(UIKit) && APPLE_COMPLETO)
        await MainActor.run { UIPasteboard.general.string = t }
        #else
        caja.l.conCandado { caja.texto = t }
        #endif
    }

    static func pegar() async -> String {
        #if (canImport(UIKit) && APPLE_COMPLETO)
        return await MainActor.run { UIPasteboard.general.string ?? "" }
        #else
        return caja.l.conCandado { caja.texto }
        #endif
    }

    static func dispositivo() async -> (nombre: String, modelo: String, sistema: String, version: String, pantalla: String) {
        #if (canImport(UIKit) && APPLE_COMPLETO)
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

extension NSLock {
    /// Como withLock (que solo existe desde iOS 16), pero en cualquier versión.
    @discardableResult
    func conCandado<R>(_ cuerpo: () throws -> R) rethrows -> R {
        lock()
        defer { unlock() }
        return try cuerpo()
    }
}

#if os(Linux) && canImport(FoundationNetworking)
import FoundationNetworking

// En Linux con Swift 5.x, URLSession no trae las versiones async:
// estas hacen lo mismo con los callbacks de siempre.
extension URLSession {
    func data(for req: URLRequest) async throws -> (Data, URLResponse) {
        try await withCheckedThrowingContinuation { c in
            dataTask(with: req) { d, r, e in
                if let e {
                    c.resume(throwing: e)
                } else {
                    let vacia = URLResponse(url: req.url ?? URL(fileURLWithPath: "/"), mimeType: nil,
                                            expectedContentLength: 0, textEncodingName: nil)
                    c.resume(returning: (d ?? Data(), r ?? vacia))
                }
            }.resume()
        }
    }

    func data(from url: URL) async throws -> (Data, URLResponse) {
        try await data(for: URLRequest(url: url))
    }
}
#endif
