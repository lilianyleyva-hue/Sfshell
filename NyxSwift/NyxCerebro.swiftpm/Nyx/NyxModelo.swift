#if canImport(SwiftUI)
import SwiftUI
#if canImport(SwiftData)
import SwiftData
#endif

// NyxModelo.swift — une el cerebro con la pantalla y con SwiftData.

struct MensajeChat: Identifiable {
    let id: Int
    let deHumano: Bool
    let rol: Int          // quién habló (-1: tú)
    let texto: String
    let resh: String
    let detalle: String
}

struct FilaTransferencia: Identifiable {
    let id: Int
    let de: Int
    let para: Int
    let tipo: String
    let contenido: String
    let recogieron: Int
}

struct Gusto: Identifiable {
    let id: Int
    let nombre: String
    let valor: Double
    let veces: Int
}

struct FilaMente: Identifiable {
    let id: Int
    let nombre: String
    let precision: Double
    let ideas: Int
    let resh: Int
    let dichos: Int
    let gusta: String
}

@MainActor
func colorDe(_ rol: Int) -> Color {
    if rol < 0 { return .white }
    return Color(hue: Double(rol) / 18.0, saturation: 0.55, brightness: 0.95)
}

final class NyxModelo: ObservableObject {
    @Published var chat: [MensajeChat] = []
    @Published var eventos: [Evento] = []
    @Published var vivo = false
    @Published var pausa: Double = 0.6
    @Published var transferencias: [FilaTransferencia] = []
    @Published var version = 0
    @Published var aviso = ""
    var conSwiftData = false
    var consejo: Consejo
    var puedeOpinar = false
    private var contador = 0
    private var tarea: Task<Void, Never>?
    #if canImport(SwiftData)
    private var contenedor: ModelContainer?
    private var contexto: ModelContext?
    private var buzonSD: BuzonSwiftData?
    #endif

    init() {
        var cargado: Consejo? = nil
        var origen = ""
        #if canImport(SwiftData)
        if let cont = try? ModelContainer(for: SaberMente.self, Transferencia.self) {
            let ctx = ModelContext(cont)
            contenedor = cont
            contexto = ctx
            conSwiftData = true
            cargado = AlmacenSwiftData.carga(de: ctx)
            if cargado != nil { origen = "recordaron todo (SwiftData)" }
        }
        #endif
        if cargado == nil, let c = Consejo.carga() {
            cargado = c
            origen = "recordaron todo (archivo)"
        }
        if let c = cargado {
            consejo = c
        } else {
            consejo = Consejo(infancia: true)
            origen = "nacieron y leyeron su infancia"
        }
        aviso = origen + (conSwiftData ? " · conocimiento en SwiftData" : "")
        conecta()
        refrescaTransferencias()
    }

    private func conecta() {
        consejo.alEvento = { [weak self] e in self?.agrega(e) }
        #if canImport(SwiftData)
        if let ctx = contexto {
            let b = BuzonSwiftData(contexto: ctx)
            b.alCambiar = { [weak self] in self?.refrescaTransferencias() }
            buzonSD = b
            consejo.buzon = b
        }
        #endif
    }

    private func agrega(_ e: Evento) {
        eventos.append(e)
        if eventos.count > 400 { eventos.removeFirst(eventos.count - 400) }
    }

    // MARK: hablar

    func pregunta(_ texto: String, a destino: Int) {
        let t = texto.trimmingCharacters(in: .whitespacesAndNewlines)
        if t.isEmpty { return }
        contador += 1
        chat.append(MensajeChat(id: contador, deHumano: true, rol: -1, texto: t, resh: "", detalle: ""))
        contador += 1
        if destino < 0 {
            let r = consejo.delibera(t)
            let acuerdo = Int((r.acuerdo * 100).rounded())
            let quien = r.vocero >= 0 ? consejo.mentes[r.vocero].nombre : "nadie"
            let det = "ganó «\(r.ganador)» · acuerdo \(acuerdo)% · habló \(quien) · " + (r.recordada ? "lo recordó" : "frase propia")
            chat.append(MensajeChat(id: contador, deHumano: false, rol: r.vocero, texto: r.frase, resh: r.resh, detalle: det))
        } else {
            let r = consejo.hablaCon(destino, t)
            let det = "C=\(fmt(r.C)) · " + (r.recordada ? "lo recordó" : "frase propia")
            chat.append(MensajeChat(id: contador, deHumano: false, rol: destino, texto: r.es, resh: r.resh, detalle: det))
        }
        puedeOpinar = true
        version += 1
    }

    func opina(_ bueno: Bool) {
        guard puedeOpinar else { return }
        if consejo.opina(bueno: bueno) {
            contador += 1
            let t = bueno ? "👍 lo reforzaron (y lo compartieron con las demás)" : "👎 lo inhibieron: la próxima vez gana otra respuesta"
            chat.append(MensajeChat(id: contador, deHumano: false, rol: -2, texto: t, resh: "", detalle: ""))
        }
        puedeOpinar = false
        guarda()
    }

    // MARK: vida libre

    func alternaVivo() {
        vivo.toggle()
        if vivo { arranca() } else { tarea?.cancel(); guarda() }
    }

    private func arranca() {
        tarea?.cancel()
        tarea = Task { @MainActor [weak self] in
            while let s = self, s.vivo, !Task.isCancelled {
                s.unPaso()
                let ns = UInt64(max(0.05, s.pausa) * 1_000_000_000)
                try? await Task.sleep(nanoseconds: ns)
            }
        }
    }

    func unPaso() {
        consejo.paso()
        version += 1
        if consejo.tick % 30 == 0 { guarda() }
    }

    func conversa(_ a: Int, _ b: Int) {
        if a == b { return }
        consejo.conversa(a, b, turnos: 8)
        version += 1
    }

    // MARK: enseñar y memoria

    func ensena(_ texto: String) -> Int {
        var lineas = 0
        for l in texto.split(whereSeparator: { $0 == "\n" || $0 == "." }) {
            let t = l.trimmingCharacters(in: .whitespaces)
            if t.isEmpty { continue }
            consejo.lee(t)
            lineas += 1
        }
        version += 1
        guarda()
        return lineas
    }

    func guarda() {
        #if canImport(SwiftData)
        if let ctx = contexto {
            AlmacenSwiftData.guarda(consejo, en: ctx)
            buzonSD?.limpia(dejando: 400)
            return
        }
        #endif
        consejo.guarda()
    }

    func olvidaTodo() {
        vivo = false
        tarea?.cancel()
        consejo = Consejo(infancia: true)
        #if canImport(SwiftData)
        if let ctx = contexto {
            try? ctx.delete(model: Transferencia.self)
            try? ctx.delete(model: SaberMente.self)
            try? ctx.save()
        }
        #endif
        conecta()
        chat = []
        eventos = []
        guarda()
        refrescaTransferencias()
        version += 1
    }

    func refrescaTransferencias() {
        #if canImport(SwiftData)
        if let b = buzonSD {
            var filas: [FilaTransferencia] = []
            var n = 0
            for t in b.recientes(150) {
                n += 1
                filas.append(FilaTransferencia(id: n, de: t.de, para: t.para, tipo: t.tipo,
                                               contenido: t.contenido, recogieron: t.cuantasRecogieron))
            }
            transferencias = filas
        }
        #endif
    }

    // MARK: datos para la pantalla

    func filas() -> [FilaMente] {
        var out: [FilaMente] = []
        for m in consejo.mentes {
            out.append(FilaMente(id: m.rol, nombre: m.nombre, precision: Double(m.precision), ideas: m.s.count,
                                 resh: m.cuentaResh, dichos: m.dichos, gusta: m.mejorAccion.nombre))
        }
        return out
    }

    /// Gusto total por cada acción (rol + lo aprendido).
    func gustos(_ rol: Int) -> [Gusto] {
        let m = consejo.mentes[rol]
        var out: [Gusto] = []
        for a in Accion.allCases {
            let v = Double(m.info.gusto[a.rawValue] + m.valor[a.rawValue])
            out.append(Gusto(id: a.rawValue, nombre: a.nombre, valor: v, veces: m.veces[a.rawValue]))
        }
        return out
    }

    /// Lo más activo en su mente.
    func activo(_ rol: Int) -> [String] {
        let m = consejo.mentes[rol]
        var ids: [Int] = []
        for i in 0 ..< m.s.count where !Palabras.vacia(m.s[i].et) && !(m.s[i].fusion && m.s[i].dialogo == 0) {
            ids.append(i)
        }
        ids.sort { m.s[$0].A > m.s[$1].A }
        return ids.prefix(10).map { i -> String in
            let x = m.s[i]
            let r = x.sabe ? (Resh.deEspanol(x.et).map { " · resh " + $0 } ?? "") : ""
            return "\(x.et)  A=\(fmt(x.A))\(r)"
        }
    }

    func foco(_ rol: Int) -> String {
        let m = consejo.mentes[rol]
        return m.foco.reversed().map { m.s[$0].et }.joined(separator: " · ")
    }

    func recuerdos(_ rol: Int) -> [String] {
        return Array(consejo.mentes[rol].ventana.suffix(6).reversed())
    }
}
#endif
