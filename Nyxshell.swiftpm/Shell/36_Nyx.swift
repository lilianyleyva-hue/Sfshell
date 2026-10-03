import Foundation

// ============================================================
// MARK: - Nyx en la terminal: las 18 mentes resonantes
// ============================================================
// nyx                     estado del consejo
// nyx <pregunta>          las 18 deliberan y dan un veredicto
// nyx habla <mensaje>     tu mensaje entra al éter; responden libres
// nyx enseña <es> = <resh>[,<resh>]   enseña una palabra a las 18
// nyx resh <texto>        dilo en Resh con lo que ya saben
// nyx mentes              precisión y tamaño de cada mente
// nyx despierta | duerme  pensamiento continuo encendido / apagado
// nyx guarda              guarda su memoria ya
// nyx log                 lo último que pasó en el consejo
// Simulacro: Nyx/pregunta
// Memoria: /var/nyx/memoria.json
//
// Huella le cuenta a Nyx lo que ve pasar en la terminal: así las
// mentes atan sus palabras a hechos reales.

/// El consejo es UNO solo para la terminal y para la ventana.
final class NyxNucleo: @unchecked Sendable {
    static let uno = NyxNucleo()
    static let ruta = "/var/nyx/memoria.json"

    let consejo = ConsejoResonante()
    private(set) var listo = false
    private var memoria: URL?
    private var tareaConsejo: Task<Void, Never>?
    private var cambiosSinGuardar = 0

    /// Especializa las 18 y recupera lo que aprendieron. La memoria se
    /// carga ANTES de especializar, para que el objetivo de cada mente
    /// reutilice sus semiones recordados en vez de duplicarlos.
    func preparar(memoria url: URL?) async -> String {
        if memoria == nil { memoria = url }
        guard !listo else { return "" }
        listo = true
        var nota = ""
        if let u = memoria, let d = FileManager.default.contents(atPath: u.path) {
            let n = await consejo.importar(d)
            if n > 0 { nota = "nyx: las \(n) mentes recuerdan lo que aprendieron\n" }
        }
        await consejo.especializar()
        return nota
    }

    func despertar() async {
        await consejo.arrancarCognicionEterna()
        if tareaConsejo == nil {
            tareaConsejo = Task.detached(priority: .background) { [consejo] in
                await consejo.runForever()
            }
        }
    }

    func dormir() async {
        tareaConsejo?.cancel()
        tareaConsejo = nil
        await consejo.dormir()
        _ = await guardar()
    }

    @discardableResult
    func guardar() async -> Bool {
        guard listo, let u = memoria, let d = await consejo.exportar() else { return false }
        try? FileManager.default.createDirectory(at: u.deletingLastPathComponent(), withIntermediateDirectories: true)
        cambiosSinGuardar = 0
        return (try? d.write(to: u)) != nil
    }

    /// Guarda cada pocas lecciones: guardar 18 mentes no es instantáneo.
    func anotaCambio() async {
        cambiosSinGuardar += 1
        if cambiosSinGuardar >= 4 { await guardar() }
    }

    /// Huella llama aquí con lo que vio. Si Nyx no está preparada, no hace nada.
    func percibir(_ hechos: [String]) {
        guard listo, !hechos.isEmpty else { return }
        Task { [consejo] in
            for h in hechos.prefix(8) { await consejo.percibir(h) }
        }
    }
}

extension Shell {

    static func nyx() -> [String: Spec] {
        var c: [String: Spec] = [:]

        c["nyx"] = Spec(help: "nyx <pregunta> — las 18 mentes deliberan · nyx habla|enseña|resh|mentes|despierta|duerme|guarda|log") { ctx in
            let nucleo = NyxNucleo.uno
            let consejo = nucleo.consejo
            let url = try? ctx.env.resolve(NyxNucleo.ruta)
            var out = await nucleo.preparar(memoria: url)
            let a = ctx.args
            let orden = a.first?.lowercased() ?? ""
            let resto = a.dropFirst().joined(separator: " ")

            switch orden {
            case "":
                let estado = await consejo.estadoMentes()
                let semiones = estado.map(\.semiones).reduce(0, +)
                let palabras = estado.map(\.palabras).reduce(0, +)
                let despiertas = await consejo.despiertas
                out += "nyx: \(estado.count) mentes · \(semiones) semiones · \(palabras) palabras aprendidas · "
                out += despiertas ? "despiertas\n" : "dormidas (solo piensan cuando les hablas)\n"
                out += "  nyx <pregunta> · nyx habla <msg> · nyx enseña árbol = kash\n"
                return out

            case "habla":
                guard !resto.isEmpty else { throw ShErr("nyx habla: ¿qué les dices?") }
                await consejo.conversar(resto)
                // lo que respondieron después de TU mensaje (el chat se recorta
                // solo, así que se busca tu mensaje en vez de contar posiciones)
                let chat = await consejo.chatReciente(150)
                let desde = chat.lastIndex(where: { $0.esHumano && $0.texto == resto }).map { $0 + 1 } ?? chat.count
                let nuevos = chat[desde...].filter { !$0.esHumano }
                if nuevos.isEmpty { return out + "nyx: …(silencio)\n" }
                for m in nuevos {
                    out += (m.destacada ? "★ " : "  ") + "\(m.autor): \(m.texto)\n"
                }
                await nucleo.anotaCambio()
                return out

            case "enseña", "ensena":
                let partes = resto.components(separatedBy: "=")
                guard partes.count == 2 else { throw ShErr("uso: nyx enseña árbol = kash[, otra]") }
                let es = partes[0].trimmingCharacters(in: .whitespaces)
                let resh = partes[1].split(separator: ",").map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
                guard !es.isEmpty, !resh.isEmpty else { throw ShErr("uso: nyx enseña árbol = kash[, otra]") }
                await consejo.enseñarATodos(español: es, significa: resh)
                await nucleo.guardar()
                return out + "nyx: las 18 aprendieron «\(es)» = \(resh.joined(separator: ", "))\n"

            case "resh":
                guard !resto.isEmpty else { throw ShErr("nyx resh: ¿qué texto?") }
                return out + (await consejo.traducirAresh(resto)) + "\n"

            case "mentes":
                for e in await consejo.estadoMentes() {
                    let p = Int((e.precision * 100).rounded())
                    out += e.rol.rawValue.padding(toLength: 12, withPad: " ", startingAt: 0)
                    out += " precisión \(p)% · \(e.semiones) semiones · \(e.palabras) palabras\n"
                }
                return out

            case "despierta":
                await nucleo.despertar()
                return out + "nyx: las 18 piensan sin parar (gasta batería; 'nyx duerme' para pararlas)\n"

            case "duerme":
                await nucleo.dormir()
                return out + "nyx: duermen. Lo aprendido queda guardado\n"

            case "guarda":
                let ok = await nucleo.guardar()
                return out + (ok ? "nyx: memoria guardada en \(NyxNucleo.ruta)\n" : "nyx: no pude guardar\n")

            case "log":
                return out + (await consejo.recentLog(20)).joined(separator: "\n") + "\n"

            default:
                // una pregunta: deliberación completa del consejo
                let pregunta = a.joined(separator: " ")
                let r = await consejo.deliberar(sobre: pregunta)
                out += "nyx: \(r.veredicto)\n"
                for v in r.votos.prefix(5) {
                    out += "  \(v.rol.rawValue): \(v.atractor)"
                    out += "  (C=\(String(format: "%.2f", v.coherencia)) E=\(String(format: "%.2f", v.incrustacion)))\n"
                }
                ctx.env.vars["NYX"] = r.veredicto
                await nucleo.anotaCambio()
                return out
            }
        }

        return c
    }
}
