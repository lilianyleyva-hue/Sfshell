#if canImport(SwiftUI)
import Foundation

// NyxEnTerminal.swift — la orden «nyx» de la terminal habla con Nyx Uno
// (solo Nyx, una sola mente: ninguno de los consejos).

extension NyxModelo: CerebroTerminal {
    @MainActor
    func terminalPregunta(_ texto: String) -> (respuesta: String, pensamiento: [String]) {
        if cargandoMemoria { return ("espera un momento: estoy recordando todo lo que sé…", []) }
        let p = uno.escucha(texto)
        refrescaUno()
        guarda()
        return (p.es, p.pensamiento)
    }

    @MainActor
    func terminalImagina(_ texto: String) -> String {
        if cargandoMemoria { return "espera un momento: estoy recordando…" }
        return uno.imagina(texto).es
    }

    @MainActor
    func terminalAprende(_ texto: String) -> String {
        if cargandoMemoria { return "espera un momento: estoy recordando…" }
        let p = uno.escucha(texto)
        guarda()
        return p.es
    }

    @MainActor
    func terminalLee(_ url: URL, nombre: String) {
        adjunta(archivos: [(url, nombre)])
    }

    @MainActor
    func terminalEstado() -> String {
        let ideas = consejo.mentes.map { $0.s.count }.reduce(0, +)
        return "\(ideas) ideas · \(consejo.frases.frases.count) frases · \(logico.hechos.count) hechos · \(estadoMemoria)"
    }

    @MainActor
    func terminalGuardaMemoria() async -> Data? {
        return await preparaArchivo()
    }

    @MainActor
    func terminalCargaMemoria(_ url: URL) {
        cargaArchivo(url)
    }
}
#endif
