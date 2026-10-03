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
    func textoMente(_ m: Mente) -> String {
        var l: [String] = []
        l.append("MENTE \(m.rol) \(m.s.count) \(m.aciertos) \(m.intentos) \(m.ciclos) \(m.dichos) \(m.ideas) \(m.cristales) \(m.reshAprendidas)")
        var val = "VAL"
        for a in 0 ..< numAcciones { val += " \(m.valor[a]) \(m.veces[a])" }
        l.append(val)
        l.append("OBJ " + m.obj.map { String($0) }.joined(separator: " "))
        for x in m.s { l.append(lineaSemion(x)) }
        return l.joined(separator: "\n")
    }

    /// Las frases recordadas, como texto.
    func textoFrases() -> String {
        var l: [String] = ["FRASES \(frases.frases.count)"]
        for f in frases.frases { l.append("F \(f.puntos) \(f.usos) \(f.texto)") }
        return l.joined(separator: "\n")
    }

    /// Texto con todo lo aprendido.
    func exporta() -> String {
        var l: [String] = ["NYX 1 \(tick)"]
        for m in mentes { l.append(textoMente(m)) }
        l.append(textoFrases())
        l.append("FIN")
        return l.joined(separator: "\n") + "\n"
    }

    /// Rearma el consejo con el texto de cada mente y el de las frases.
    static func desdePartes(mentes: [String], frases: String) -> Consejo? {
        var t = "NYX 1 0\n" + mentes.joined(separator: "\n") + "\n"
        if !frases.isEmpty { t += frases + "\n" }
        return desde(t + "FIN\n")
    }

    private func lineaSemion(_ x: Semion) -> String {
        var t = "S \(x.et) \(x.A) \(x.fase) \(x.frec) \(x.carga) \(x.fusion ? 1 : 0) \(x.sabe ? 1 : 0) \(x.usos) \(x.dialogo)"
        for v in x.f { t += " \(v)" }
        t += " \(x.v.count)"
        for a in x.v { t += " \(a.j) \(a.w) \(a.th) \(a.sec)" }
        return t
    }

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
        let p = linea.split(separator: " ").map(String.init)
        guard p.count >= 19, p[0] == "S" else { return nil }
        var f: [Float] = []
        for k in 0 ..< 8 { f.append(Float(p[10 + k]) ?? 0) }
        guard let nv = Int(p[18]), nv >= 0, nv <= maxAcoples, p.count == 19 + 4 * nv else { return nil }
        var v: [Acople] = []
        for k in 0 ..< nv {
            let b = 19 + 4 * k
            guard let j = Int(p[b]), j >= 0, j < n else { return nil }
            v.append(Acople(j: j, w: Float(p[b + 1]) ?? 0, th: Float(p[b + 2]) ?? 0, sec: Float(p[b + 3]) ?? 0))
        }
        return Semion(et: p[1], f: f, A: Float(p[2]) ?? 0.5, fase: Float(p[3]) ?? 0, frec: Float(p[4]) ?? 1,
                      carga: Int(p[5]) ?? 0, fusion: p[6] == "1", sabe: p[7] == "1",
                      usos: Int(p[8]) ?? 0, dialogo: Int(p[9]) ?? 0, v: v)
    }
}
