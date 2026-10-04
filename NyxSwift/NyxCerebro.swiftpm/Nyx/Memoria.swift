import Foundation

// Memoria.swift — guardar y cargar lo aprendido (un archivo de texto en
// Documentos). Si el archivo está dañado, no se pierde nada: se sigue con
// lo que había.

extension Consejo {
    static var rutaMemoria: URL {
        let docs = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first
            ?? URL(fileURLWithPath: NSTemporaryDirectory())
        return docs.appendingPathComponent("nyx_memoria.txt")
    }

    /// Lo que sabe una mente, como texto (va a SwiftData o al archivo).
    func textoMente(_ m: Mente) -> String { RetratoMente(m).texto() }

    /// Las frases recordadas, como texto.
    func textoFrases() -> String { RetratoCerebro.textoFrases(frases.frases) }

    /// Texto con todo lo aprendido.
    func exporta() -> String { foto().todo() }

    /// Una «foto» del cerebro: se saca al instante (los arrays se copian sin
    /// copiar de verdad) y se convierte en texto en segundo plano, sin parar la app.
    func foto() -> RetratoCerebro {
        return RetratoCerebro(tick: tick, mentes: mentes.map { RetratoMente($0) }, frases: frases.frases)
    }

    /// Rearma el consejo con el texto de cada mente y el de las frases.
    static func desdePartes(mentes: [String], frases: String) -> Consejo? {
        var t = "NYX 1 0\n" + mentes.joined(separator: "\n") + "\n"
        if !frases.isEmpty { t += frases + "\n" }
        return desde(t + "FIN\n")
    }

}

/// La foto de una mente (todo valores: se puede pasar a otro hilo).
struct RetratoMente: @unchecked Sendable {
    let rol: Int
    let s: [Semion]
    let obj: [Int]
    let cabecera: String
    let val: String

    init(_ m: Mente) {
        rol = m.rol
        s = m.s
        obj = m.obj
        cabecera = "MENTE \(m.rol) \(m.s.count) \(m.aciertos) \(m.intentos) \(m.ciclos) \(m.dichos) \(m.ideas) \(m.cristales) \(m.reshAprendidas)"
        var v = "VAL"
        for a in 0 ..< numAcciones { v += " \(m.valor[a]) \(m.veces[a])" }
        val = v
    }

    func texto() -> String {
        var l: [String] = [cabecera, val, "OBJ " + obj.map { String($0) }.joined(separator: " ")]
        l.reserveCapacity(s.count + 3)
        for x in s { l.append(RetratoMente.lineaSemion(x)) }
        return l.joined(separator: "\n")
    }

    /// Nyx 2: los números van con 3 decimales, escritos como enteros (×1000):
    /// la línea empieza por "T". Las antiguas ("S") se siguen pudiendo leer.
    static func lineaSemion(_ x: Semion) -> String {
        func m(_ v: Float) -> String { String(Int((v * 1000).rounded())) }
        var t = "T \(x.et) \(m(x.A)) \(m(x.fase)) \(m(x.frec)) \(x.carga) \(x.fusion ? 1 : 0) \(x.sabe ? 1 : 0) \(x.usos) \(x.dialogo)"
        for v in x.f { t += " " + m(v) }
        t += " \(x.v.count)"
        for a in x.v { t += " \(a.j) " + m(a.w) + " " + m(a.th) + " " + m(a.sec) }
        return t
    }}

/// La foto de todo el cerebro.
struct RetratoCerebro: @unchecked Sendable {
    let tick: Int
    let mentes: [RetratoMente]
    let frases: [FraseMem]

    static func textoFrases(_ fs: [FraseMem]) -> String {
        var l: [String] = ["FRASES \(fs.count)"]
        l.reserveCapacity(fs.count + 1)
        for f in fs { l.append("F \(f.puntos) \(f.usos) \(f.texto)") }
        return l.joined(separator: "\n")
    }

    func textosMentes() -> [String] { mentes.map { $0.texto() } }
    func textoFrases() -> String { RetratoCerebro.textoFrases(frases) }

    func todo() -> String {
        var l: [String] = ["NYX 1 \(tick)"]
        l.append(contentsOf: textosMentes())
        l.append(textoFrases())
        l.append("FIN")
        return l.joined(separator: "\n") + "\n"
    }
}

extension Consejo {


    @discardableResult
    func guarda() -> Bool {
        do {
            try exporta().write(to: Consejo.rutaMemoria, atomically: true, encoding: .utf8)
            return true
        } catch {
            return false
        }
    }

    /// Carga la memoria guardada. Devuelve nil si no hay o está dañada.
    static func carga() -> Consejo? {
        guard let texto = try? String(contentsOf: rutaMemoria, encoding: .utf8) else { return nil }
        return Consejo.desde(texto)
    }

    static func desde(_ texto: String) -> Consejo? {
        let lineas = texto.split(separator: "\n", omittingEmptySubsequences: false).map(String.init)
        var i = 0
        guard i < lineas.count, lineas[i].hasPrefix("NYX ") else { return nil }
        let cab = lineas[i].split(separator: " ")
        let c = Consejo(infancia: false)
        c.tick = cab.count > 2 ? Int(cab[2]) ?? 0 : 0
        i += 1
        for r in 0 ..< numRoles {
            guard let siguiente = cargaMente(c.mentes[r], lineas, desde: i) else { return nil }
            i = siguiente
        }
        if i < lineas.count, lineas[i].hasPrefix("FRASES ") {
            let n = Int(lineas[i].dropFirst(7)) ?? 0
            i += 1
            for _ in 0 ..< n where i < lineas.count {
                let p = lineas[i].split(separator: " ", maxSplits: 3).map(String.init)
                if p.count == 4, p[0] == "F" {
                    c.frases.pon(p[3], puntos: Float(p[1]) ?? 0, usos: Int(p[2]) ?? 1)
                }
                i += 1
            }
        } else {
            for t in Roles.infancia { c.frases.pon(t) }
        }
        return c
    }

    private static func cargaMente(_ m: Mente, _ l: [String], desde: Int) -> Int? {
        var i = desde
        guard i + 2 < l.count else { return nil }
        let cab = l[i].split(separator: " ").map(String.init)
        guard cab.count == 10, cab[0] == "MENTE", Int(cab[1]) == m.rol, let n = Int(cab[2]), n >= 0, n <= maxSemiones else { return nil }
        m.aciertos = Int(cab[3]) ?? 0
        m.intentos = Int(cab[4]) ?? 0
        m.ciclos = Int(cab[5]) ?? 0
        m.dichos = Int(cab[6]) ?? 0
        m.ideas = Int(cab[7]) ?? 0
        m.cristales = Int(cab[8]) ?? 0
        m.reshAprendidas = Int(cab[9]) ?? 0
        let val = l[i + 1].split(separator: " ").map(String.init)
        guard val.count == 1 + 2 * numAcciones, val[0] == "VAL" else { return nil }
        for a in 0 ..< numAcciones {
            m.valor[a] = Float(val[1 + 2 * a]) ?? 0
            m.veces[a] = Int(val[2 + 2 * a]) ?? 0
        }
        let obj = l[i + 2].split(separator: " ").dropFirst().compactMap { Int($0) }
        i += 3
        var sems: [Semion] = []
        sems.reserveCapacity(n)
        for _ in 0 ..< n {
            guard i < l.count, let x = leeSemion(l[i], n: n) else { return nil }
            sems.append(x)
            i += 1
        }
        m.s = sems
        m.obj = obj.filter { $0 < n }
        m.foco = []
        m.epis = []
        m.caminos = []
        m.recientes = []
        m.ventana = []
        m.ultima = -1
        m.racha = 0
        m.rehaceIndice()
        return i
    }

    private static func leeSemion(_ linea: String, n: Int) -> Semion? {
        // trozos sin copiar (Substring): cargar miles de ideas es mucho más rápido
        let p = linea.split(separator: " ")
        guard p.count >= 19, p[0] == "S" || p[0] == "T" else { return nil }
        let enteros = p[0] == "T"          // "T": enteros ×1000 (más rápido de leer)
        func num(_ s: Substring, _ d: Float) -> Float {
            if enteros { return Int(s).map { Float($0) * 0.001 } ?? d }
            return Float(s) ?? d
        }
        var f: [Float] = []
        f.reserveCapacity(8)
        for k in 0 ..< 8 { f.append(num(p[10 + k], 0)) }
        guard let nv = Int(p[18]), nv >= 0, nv <= maxAcoples, p.count == 19 + 4 * nv else { return nil }
        var v: [Acople] = []
        v.reserveCapacity(nv)
        for k in 0 ..< nv {
            let b = 19 + 4 * k
            guard let j = Int(p[b]), j >= 0, j < n else { return nil }
            v.append(Acople(j: j, w: num(p[b + 1], 0), th: num(p[b + 2], 0), sec: num(p[b + 3], 0)))
        }
        return Semion(et: String(p[1]), f: f, A: num(p[2], 0.5), fase: num(p[3], 0), frec: num(p[4], 1),
                      carga: Int(p[5]) ?? 0, fusion: p[6] == "1", sabe: p[7] == "1",
                      usos: Int(p[8]) ?? 0, dialogo: Int(p[9]) ?? 0, v: v)
    }

}

/// Los textos del cerebro ya preparados (para pasarlos entre hilos).
struct TextosCerebro: Sendable {
    let mentes: [String]
    let frases: String
}

/// Lo que se carga en segundo plano.
final class CajaCarga: @unchecked Sendable {
    var consejo: Consejo? = nil
    var logico: ConsejoLogico? = nil
    var saber: SaberMisiones? = nil
    var origen = ""
}
