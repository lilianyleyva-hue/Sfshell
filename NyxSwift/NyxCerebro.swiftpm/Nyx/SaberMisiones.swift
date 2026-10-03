import Foundation

// SaberMisiones.swift — lo que el consejo nuevo aprende de las misiones:
// cada mente lleva la cuenta de qué estrategia le funciona en cada tipo de
// misión (aprendizaje por refuerzo), y el marcador de los tres consejos.

final class SaberMisiones {
    private let nt = TipoMision.allCases.count
    private let ne = Estrategia.allCases.count
    private(set) var valor: [[Float]]       // [rol * tipos + tipo][estrategia]
    private(set) var veces: [[Int]]
    var puntos = [0, 0, 0]                  // ✨ nuevo · 🏛 antiguo · ⚖️ lógico
    var jugadas = 0
    var ganadas: [Int]                      // por tipo: misiones que alguien acertó
    var jugadasTipo: [Int]
    var aciertosNuevo: [Int]                // por tipo
    /// Misiones ya resueltas: enunciado → respuesta (para "recordar").
    var recuerdos: [String: String] = [:]

    init() {
        valor = Array(repeating: Array(repeating: 0, count: Estrategia.allCases.count), count: numRoles * TipoMision.allCases.count)
        veces = Array(repeating: Array(repeating: 0, count: Estrategia.allCases.count), count: numRoles * TipoMision.allCases.count)
        ganadas = Array(repeating: 0, count: TipoMision.allCases.count)
        jugadasTipo = ganadas
        aciertosNuevo = ganadas
    }

    func nivel(_ t: TipoMision) -> Int { 1 + min(4, ganadas[t.rawValue] / 3) }

    /// Cómo es cada mente de nacimiento (antes de aprender nada).
    static func sesgo(_ rol: Int, _ e: Estrategia) -> Float {
        let n = Roles.todos[rol].nombre
        switch e {
        case .calcula: return ["logica", "codigo", "sintaxis"].contains(n) ? 0.3 : (["semantica", "etica"].contains(n) ? 0.1 : 0)
        case .prueba: return ["critico", "esceptico", "percepcion"].contains(n) ? 0.3 : 0
        case .rapido: return ["sintesis", "abstraccion", "analogia"].contains(n) ? 0.25 : 0.05
        case .estima: return ["intuicion", "empatia"].contains(n) ? 0.3 : 0
        case .recuerda: return ["memoria", "contexto", "narrativa"].contains(n) ? 0.3 : 0
        case .adivina: return ["creativo", "curiosidad"].contains(n) ? 0.25 : 0
        }
    }

    func cuanto(_ rol: Int, _ t: TipoMision, _ e: Estrategia) -> Float {
        return valor[rol * nt + t.rawValue][e.rawValue]
    }

    /// La mente elige estrategia: casi siempre la que mejor le ha ido; a veces explora.
    func elige(_ rol: Int, _ t: TipoMision) -> Estrategia {
        if Azar.f01() < 0.12 { return Estrategia.allCases[Azar.ent(ne)] }
        let k = rol * nt + t.rawValue
        var mejor = Estrategia.calcula
        var mp: Float = -1e9
        for e in Estrategia.allCases {
            let p = valor[k][e.rawValue] + SaberMisiones.sesgo(rol, e) + 0.2 / Float(1 + veces[k][e.rawValue]) + Azar.rango(0, 0.1)
            if p > mp { mp = p; mejor = e }
        }
        return mejor
    }

    func aprende(_ rol: Int, _ t: TipoMision, _ e: Estrategia, _ recompensa: Float) {
        let k = rol * nt + t.rawValue
        valor[k][e.rawValue] += 0.3 * (recompensa - valor[k][e.rawValue])
        veces[k][e.rawValue] += 1
    }

    /// La estrategia favorita del consejo para un tipo de misión.
    func favorita(_ t: TipoMision) -> Estrategia? {
        var votos = Array(repeating: 0, count: ne)
        var alguna = false
        for rol in 0 ..< numRoles {
            let k = rol * nt + t.rawValue
            if veces[k].reduce(0, +) == 0 { continue }
            alguna = true
            var mejor = 0
            for e in 1 ..< ne where valor[k][e] > valor[k][mejor] { mejor = e }
            votos[mejor] += 1
        }
        if !alguna { return nil }
        var m = 0
        for e in 1 ..< ne where votos[e] > votos[m] { m = e }
        return Estrategia(rawValue: m)
    }

    func resumen() -> String {
        var partes: [String] = []
        for t in TipoMision.allCases where jugadasTipo[t.rawValue] > 0 {
            let f = favorita(t).map { " (prefieren " + $0.nombre + ")" } ?? ""
            partes.append("\(t.icono) \(aciertosNuevo[t.rawValue])/\(jugadasTipo[t.rawValue]) nivel \(nivel(t))\(f)")
        }
        return partes.joined(separator: " · ")
    }

    // MARK: guardar

    private static func lista(_ xs: [Int]) -> String {
        return xs.map { String($0) }.joined(separator: " ")
    }

    func exporta() -> String {
        var l: [String] = []
        l.append("puntos " + SaberMisiones.lista(puntos) + " \(jugadas)")
        l.append("ganadas " + SaberMisiones.lista(ganadas))
        l.append("jugadas " + SaberMisiones.lista(jugadasTipo))
        l.append("aciertos " + SaberMisiones.lista(aciertosNuevo))
        for k in 0 ..< valor.count {
            let vs: String = valor[k].map { fmt($0) }.joined(separator: " ")
            let ns: String = veces[k].map { String($0) }.joined(separator: " ")
            l.append("v \(k) \(vs) | \(ns)")
        }
        for (e, r) in recuerdos.prefix(300) { l.append("r \(e)\t\(r)") }
        return l.joined(separator: "\n")
    }

    static func desde(_ texto: String) -> SaberMisiones {
        let s = SaberMisiones()
        for linea in texto.components(separatedBy: "\n") {
            if linea.hasPrefix("r ") {
                let p = linea.dropFirst(2).components(separatedBy: "\t")
                if p.count == 2 { s.recuerdos[p[0]] = p[1] }
                continue
            }
            let p = linea.split(separator: " ").map { String($0) }
            guard let cab = p.first else { continue }
            let nums = p.dropFirst().compactMap { Int($0) }
            switch cab {
            case "puntos" where nums.count >= 4:
                s.puntos = Array(nums.prefix(3))
                s.jugadas = nums[3]
            case "ganadas" where nums.count == s.ganadas.count: s.ganadas = nums
            case "jugadas" where nums.count == s.jugadasTipo.count: s.jugadasTipo = nums
            case "aciertos" where nums.count == s.aciertosNuevo.count: s.aciertosNuevo = nums
            case "v":
                guard p.count > 2, let k = Int(p[1]), k < s.valor.count, let bar = p.firstIndex(of: "|") else { continue }
                let vs = p[2 ..< bar].compactMap { Float($0) }
                let ns = p[(bar + 1)...].compactMap { Int($0) }
                if vs.count == s.ne { s.valor[k] = vs }
                if ns.count == s.ne { s.veces[k] = ns }
            default: break
            }
        }
        return s
    }
}

// MARK: - el consejo nuevo enfrenta una misión

struct IntentoMente {
    let rol: Int
    let estrategia: Estrategia
    let respuesta: String?
}

extension SaberMisiones {
    /// Las 18 lo intentan, cada una con su estrategia, y votan.
    func intenta(_ m: Mision, _ nuevo: Consejo) -> (respuesta: String?, vocero: Int, intentos: [IntentoMente]) {
        var intentos: [IntentoMente] = []
        var urna: [String: Float] = [:]
        for rol in 0 ..< numRoles {
            let e = elige(rol, m.tipo)
            let r = aplica(e, m, nuevo)
            intentos.append(IntentoMente(rol: rol, estrategia: e, respuesta: r))
            if let x = r {
                let peso = 1 + max(0, cuanto(rol, m.tipo, e)) * 2 + nuevo.mentes[rol].precision
                urna[Respuestas.norma(x), default: 0] += peso
            }
        }
        guard let gana = urna.max(by: { $0.value < $1.value })?.key else { return (nil, -1, intentos) }
        var vocero = -1
        var mejor: Float = -1e9
        for i in intentos where i.respuesta.map({ Respuestas.norma($0) }) == gana {
            let v = cuanto(i.rol, m.tipo, i.estrategia)
            if v > mejor { mejor = v; vocero = i.rol }
        }
        return (gana, vocero, intentos)
    }

    /// Lo que contesta una mente con una estrategia (nil: no sabe).
    func aplica(_ e: Estrategia, _ m: Mision, _ nuevo: Consejo) -> String? {
        if e == .recuerda { return recuerdos[Respuestas.norma(m.enunciado)] }
        switch m.tipo {
        case .cuenta: return cuenta(e, m)
        case .serie: return serie(e, m)
        case .ecuacion: return ecuacion(e, m)
        case .problema: return problema(e, m)
        case .silogismo: return silogismo(e, m)
        case .orden: return orden(e, m)
        case .intruso: return intruso(e, m, nuevo)
        }
    }

    private func adivinaCerca(_ v: Double?) -> String? {
        guard let x = v else { return nil }
        return Respuestas.numero((x + Double(Azar.ent(7) - 3)).rounded())
    }

    private func cuenta(_ e: Estrategia, _ m: Mision) -> String? {
        switch e {
        case .calcula: return Aritmetica.resultado(m.enunciado)
        case .rapido: return Respuestas.numero(Aritmetica.izquierdaDerecha(m.enunciado))
        case .estima:
            // redondea cada número a la decena y calcula
            var t = Aritmetica.expresion(m.enunciado)
            for n in Resuelve.numerosDe(t).sorted(by: >) where n >= 10 {
                t = t.replacingOccurrences(of: Aritmetica.bonito(n), with: Aritmetica.bonito((n / 10).rounded() * 10))
            }
            return Respuestas.numero(Aritmetica.evalua(t))
        case .adivina: return adivinaCerca(Aritmetica.izquierdaDerecha(m.enunciado))
        default: return nil
        }
    }

    private func serie(_ e: Estrategia, _ m: Mision) -> String? {
        let n = m.numeros
        guard n.count >= 2, let u = n.last else { return nil }
        let d = u - n[n.count - 2]
        switch e {
        case .calcula: return Resuelve.serie(n).flatMap { Respuestas.numero($0.0) }
        case .prueba:
            // solo prueba sumar siempre lo mismo o multiplicar siempre por lo mismo
            guard let r = Resuelve.serie(n), r.1.hasPrefix("siempre") else { return nil }
            return Respuestas.numero(r.0)
        case .rapido: return Respuestas.numero(u + d)
        case .estima: return Respuestas.numero(u * 2)
        case .adivina: return adivinaCerca(u + d)
        default: return nil
        }
    }

    private func ecuacion(_ e: Estrategia, _ m: Mision) -> String? {
        let l = m.lados
        guard let a0 = Resuelve.conX(l.0, 0), let b0 = Resuelve.conX(l.1, 0), let a1 = Resuelve.conX(l.0, 1) else { return nil }
        switch e {
        case .calcula: return Resuelve.ecuacion(l).flatMap { Respuestas.numero($0.0) }
        case .prueba: return Respuestas.numero(Resuelve.pruebaEcuacion(l))
        case .rapido: return Respuestas.numero(b0 - a0)                // olvida lo que multiplica a la x
        case .estima: return a1 - a0 == 0 ? nil : Respuestas.numero((b0 / (a1 - a0)).rounded())
        case .adivina: return adivinaCerca(b0 - a0)
        default: return nil
        }
    }

    private func problema(_ e: Estrategia, _ m: Mision) -> String? {
        let nums = Resuelve.numerosDe(m.enunciado)
        if nums.isEmpty { return nil }
        switch e {
        case .calcula: return Resuelve.problema(m.enunciado).flatMap { Respuestas.numero($0.0) }
        case .rapido: return Respuestas.numero(nums.reduce(0, +))     // suma todo lo que ve
        case .estima: return Respuestas.numero(nums.first)
        case .adivina: return adivinaCerca(nums.first)
        default: return nil
        }
    }

    private func silogismo(_ e: Estrategia, _ m: Mision) -> String? {
        switch e {
        case .calcula: return Resuelve.silogismo(m)?.0
        case .prueba: return Resuelve.silogismo(m, hasta: 3)?.0      // mira solo unos pasos
        case .rapido: return "sí"
        case .estima: return m.enunciado.lowercased().contains("ningún") ? "no" : "sí"
        case .adivina: return Azar.f01() < 0.5 ? "sí" : "no"
        default: return nil
        }
    }

    private func orden(_ e: Estrategia, _ m: Mision) -> String? {
        guard m.opciones.count == 2 else { return nil }
        switch e {
        case .calcula: return Resuelve.orden(m)?.0
        case .rapido: return m.opciones[0]
        case .estima:
            // el que más veces sale como "el más alto que"
            let cuenta = m.opciones.map { o in m.datos.filter { $0.lowercased().hasPrefix(o) }.count }
            if cuenta[0] == cuenta[1] { return nil }
            return cuenta[0] > cuenta[1] ? m.opciones[0] : m.opciones[1]
        case .adivina: return m.opciones[Azar.ent(2)]
        default: return nil
        }
    }

    private func intruso(_ e: Estrategia, _ m: Mision, _ nuevo: Consejo) -> String? {
        guard !m.opciones.isEmpty else { return nil }
        switch e {
        case .calcula:
            // por asociación: el que menos sale junto a los demás en lo que recuerdan
            var peor: String? = nil
            var minimo = Int.max
            var empate = false
            for o in m.opciones {
                let otros = m.opciones.filter { $0 != o } + [m.categoria]
                var n = 0
                for f in nuevo.frases.frases where f.toks.contains(o) {
                    if otros.contains(where: { f.toks.contains($0) }) { n += 1 }
                }
                if n < minimo { minimo = n; peor = o; empate = false } else if n == minimo { empate = true }
            }
            return empate ? nil : peor
        case .rapido: return m.opciones.last
        case .estima:
            // el que menos conoce
            let veces = m.opciones.map { o in nuevo.frases.frases.filter { $0.toks.contains(o) }.count }
            guard let mn = veces.min(), veces.filter({ $0 == mn }).count == 1, let i = veces.firstIndex(of: mn) else { return nil }
            return m.opciones[i]
        case .adivina: return m.opciones[Azar.ent(m.opciones.count)]
        default: return nil
        }
    }
}
