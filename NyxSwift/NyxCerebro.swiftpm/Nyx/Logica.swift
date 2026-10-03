import Foundation

// Logica.swift — el consejo lógico: 18 agentes que razonan con hechos.
//
// No resuenan ni asocian: convierten frases en HECHOS ("perro es mamífero",
// "si llueve entonces suelo mojado", "elefante mayor que perro"), deducen
// hechos nuevos con reglas de lógica y contestan sí / no / no sé con la
// cadena de pasos que lo demuestra.

enum Rel: String, CaseIterable {
    case es, noEs, tiene, noTiene, causa, parte, mayor, puede, noPuede

    var texto: String {
        switch self {
        case .es: return "es"
        case .noEs: return "no es"
        case .tiene: return "tiene"
        case .noTiene: return "no tiene"
        case .causa: return "causa"
        case .parte: return "es parte de"
        case .mayor: return "es mayor que"
        case .puede: return "puede"
        case .noPuede: return "no puede"
        }
    }

    var contraria: Rel? {
        switch self {
        case .es: return .noEs
        case .noEs: return .es
        case .tiene: return .noTiene
        case .noTiene: return .tiene
        case .puede: return .noPuede
        case .noPuede: return .puede
        default: return nil
        }
    }
}

struct Hecho: Hashable {
    let a: String
    let rel: Rel
    let b: String

    var texto: String { "\(a) \(rel.texto) \(b)" }
}

struct Apoyo {
    var confianza: Float
    var quien: String          // "leído", "tú" o el agente que lo dedujo
    var porque: [String]       // los pasos
    var hipotesis = false
}

struct Regla: Hashable {
    let si: String
    let entonces: String
}

struct RespuestaLogica {
    var veredicto: String = "no sé"       // "sí", "no", "no sé", o un valor
    var explicacion: [String] = []
    var aportes: [(agente: String, texto: String)] = []
}

let agentesLogicos: [String] = [
    "lector", "silogismo", "herencia", "transitividad", "causal", "modus ponens",
    "negación", "contradicción", "inducción", "abducción", "contraejemplo", "revisor",
    "verificador", "aritmética", "comparador", "categorías", "cuantificador", "explicador",
]

final class ConsejoLogico {
    private(set) var hechos: [Hecho: Apoyo] = [:]
    private(set) var reglas: [Regla: Apoyo] = [:]
    private(set) var props: [String: Apoyo] = [:]       // proposiciones verdaderas ("llueve")
    private(set) var contradicciones: [String] = []
    var contadores: [Int] = Array(repeating: 0, count: 18)
    var alEvento: ((Int, String) -> Void)?
    private var turno = 0
    private var porLeer: [String] = []

    init(primer: Bool) {
        if primer {
            for f in ConsejoLogico.cartilla { lee(f, quien: "leído") }
        }
    }

    private func evento(_ agente: Int, _ t: String) {
        contadores[agente] += 1
        alEvento?(agente, t)
    }

    // MARK: lenguaje → hechos

    static let articulos: Set<String> = ["el", "la", "los", "las", "un", "una", "unos", "unas", "lo", "al", "del"]

    /// Singular sencillo: perros → perro, animales → animal, peces → pez.
    static func singular(_ w: String) -> String {
        if w.count <= 3 || !w.hasSuffix("s") { return w }
        if w.hasSuffix("ces") { return String(w.dropLast(3)) + "z" }
        let sin = String(w.dropLast())
        if sin.hasSuffix("e") {
            let antes = sin.dropLast().last
            if let c = antes, "rlndj".contains(c) { return String(sin.dropLast()) }
        }
        return sin
    }

    /// "los perros grandes" → "perro grande".
    static func frase(_ toks: [String]) -> String {
        let limpias = toks.filter { !articulos.contains($0) && !["todo", "todos", "toda", "todas", "cada", "ningún", "ninguno", "ninguna", "algún", "alguno", "alguna"].contains($0) }
        return limpias.map { singular($0) }.joined(separator: " ")
    }

    /// Lee una frase y guarda los hechos o reglas que contiene.
    @discardableResult
    func lee(_ texto: String, quien: String) -> [String] {
        let t = texto.lowercased()
        var nuevos: [String] = []
        if let r = reglaDe(t) {
            if reglas[r] == nil {
                reglas[r] = Apoyo(confianza: 1, quien: quien, porque: [texto])
                nuevos.append("si \(r.si) entonces \(r.entonces)")
            }
            return nuevos
        }
        let toks = Palabras.tokens(t, max: 30)
        if let h = hechoDe(toks) {
            // "la música tiene ritmo y forma" → dos hechos: ritmo, forma
            let partes = h.b.components(separatedBy: " y ").flatMap { $0.components(separatedBy: " e ") }
            for b in partes where !b.isEmpty {
                let uno = Hecho(a: h.a, rel: h.rel, b: b.trimmingCharacters(in: .whitespaces))
                if pon(uno, Apoyo(confianza: 1, quien: quien, porque: [texto])) { nuevos.append(uno.texto) }
            }
        } else if toks.count <= 3, !toks.isEmpty, !t.contains("?") {
            let p = ConsejoLogico.frase(toks)
            if !p.isEmpty && props[p] == nil {
                props[p] = Apoyo(confianza: 1, quien: quien, porque: [texto])
                nuevos.append(p)
            }
        }
        return nuevos
    }

    /// "si llueve entonces el suelo se moja" / "si llueve, el suelo se moja" / "cuando llueve, …"
    private func reglaDe(_ t: String) -> Regla? {
        let toks = Palabras.tokens(t, max: 30)
        guard let primera = toks.first, primera == "si" || primera == "cuando" else { return nil }
        var resto = t
        if let r = resto.range(of: primera) { resto = String(resto[r.upperBound...]) }
        let partes: [String]
        if resto.contains(" entonces ") {
            partes = resto.components(separatedBy: " entonces ")
        } else {
            partes = resto.components(separatedBy: ",")
        }
        guard partes.count >= 2 else { return nil }
        let a = ConsejoLogico.frase(Palabras.tokens(partes[0]))
        let b = ConsejoLogico.frase(Palabras.tokens(partes[1...].joined(separator: " ")))
        if a.isEmpty || b.isEmpty { return nil }
        return Regla(si: a, entonces: b)
    }

    /// Pone el verbo de una pregunta en su sitio.
    static func reordena(_ verbo: String, _ resto: [String]) -> [String] {
        var marcas = articulos
        for m in ["más", "mayor", "menor", "parte"] { marcas.insert(m) }
        if let f = resto.first, articulos.contains(f) {
            // el sujeto va primero: "el perro | un animal"
            var j = 1
            while j < resto.count && !marcas.contains(resto[j]) { j += 1 }
            return Array(resto[0 ..< j]) + [verbo] + Array(resto[j...])
        }
        if let k = resto.firstIndex(where: { articulos.contains($0) }), k > 0 {
            // el objeto va primero: "pelo | el gato"
            return Array(resto[k...]) + [verbo] + Array(resto[..<k])
        }
        return [resto[0], verbo] + Array(resto.dropFirst())
    }

    /// Busca el verbo y arma el hecho.
    func hechoDe(_ toks: [String]) -> Hecho? {
        if toks.isEmpty { return nil }
        let ninguno = ["ningún", "ninguno", "ninguna"].contains(toks[0])
        for (i, w) in toks.enumerated() where i > 0 {
            let no = toks[i - 1] == "no"
            let sujeto = Array(toks[0 ..< (no ? i - 1 : i)])
            let resto = Array(toks[(i + 1)...])
            if sujeto.isEmpty || resto.isEmpty { continue }
            let a = ConsejoLogico.frase(sujeto)
            if a.isEmpty { continue }
            switch w {
            case "es", "son":
                if let h = comparacion(a, resto) { return h }
                if resto.count >= 2 && resto[0] == "parte" && resto[1] == "de" {
                    return Hecho(a: a, rel: .parte, b: ConsejoLogico.frase(Array(resto.dropFirst(2))))
                }
                let b = ConsejoLogico.frase(resto)
                if b.isEmpty { continue }
                return Hecho(a: a, rel: (no || ninguno) ? .noEs : .es, b: b)
            case "tiene", "tienen":
                return Hecho(a: a, rel: (no || ninguno) ? .noTiene : .tiene, b: ConsejoLogico.frase(resto))
            case "causa", "causan", "provoca", "provocan", "produce", "producen":
                return Hecho(a: a, rel: .causa, b: ConsejoLogico.frase(resto))
            case "puede", "pueden":
                return Hecho(a: a, rel: (no || ninguno) ? .noPuede : .puede, b: ConsejoLogico.frase(resto))
            default:
                continue
            }
        }
        return nil
    }

    /// "mayor que", "más grande que", "menor que", "más pequeño que"
    private func comparacion(_ a: String, _ resto: [String]) -> Hecho? {
        guard let que = resto.firstIndex(of: "que"), que + 1 < resto.count else { return nil }
        let antes = Set(resto[..<que])
        let b = ConsejoLogico.frase(Array(resto[(que + 1)...]))
        if b.isEmpty { return nil }
        let menor = antes.contains("menor") || antes.contains("pequeño") || antes.contains("pequeña") || antes.contains("chico")
        let mayor = antes.contains("mayor") || antes.contains("más") || antes.contains("grande")
        if menor { return Hecho(a: b, rel: .mayor, b: a) }
        if mayor { return Hecho(a: a, rel: .mayor, b: b) }
        return nil
    }

    @discardableResult
    private func pon(_ h: Hecho, _ ap: Apoyo) -> Bool {
        if let viejo = hechos[h], viejo.confianza >= ap.confianza { return false }
        hechos[h] = ap
        return true
    }

    func apoyo(_ h: Hecho) -> Apoyo? { hechos[h] }

    // MARK: búsquedas

    private func salientes(_ a: String, _ rel: Rel) -> [String] {
        var out: [String] = []
        for (h, _) in hechos where h.a == a && h.rel == rel { out.append(h.b) }
        return out
    }

    /// Camino a →rel→ … → b (para es, mayor, causa, parte). nil si no hay.
    func camino(_ a: String, _ rel: Rel, _ b: String, hasta: Int = 7) -> [String]? {
        var visto: Set<String> = [a]
        var cola: [(String, [String])] = [(a, [a])]
        var i = 0
        while i < cola.count {
            let (x, ruta) = cola[i]
            i += 1
            if ruta.count > hasta { continue }
            for y in salientes(x, rel) where !visto.contains(y) {
                let r2 = ruta + [y]
                if y == b { return r2 }
                visto.insert(y)
                cola.append((y, r2))
            }
        }
        return nil
    }

    /// Todo lo que 'a' es (sus categorías, de cerca a lejos).
    func ancestros(_ a: String) -> [String] {
        var out: [String] = []
        var cola: [String] = [a]
        var visto: Set<String> = [a]
        var i = 0
        while i < cola.count {
            for y in salientes(cola[i], .es) where !visto.contains(y) {
                visto.insert(y)
                out.append(y)
                cola.append(y)
            }
            i += 1
        }
        return out
    }

    // MARK: preguntas

    /// Contesta una pregunta (o aprende, si es una afirmación).
    func procesa(_ texto: String) -> RespuestaLogica {
        let t = texto.lowercased().trimmingCharacters(in: .whitespaces)
        let toks = Palabras.tokens(t, max: 30)
        var r = RespuestaLogica()
        if let valor = Aritmetica.calcula(t) {
            r.veredicto = Aritmetica.bonito(valor)
            r.aportes.append(("aritmética", "\(Aritmetica.expresion(t)) = \(r.veredicto)"))
            evento(13, "calculó \(Aritmetica.expresion(t)) = \(r.veredicto)")
            return r
        }
        let esPregunta = t.contains("?") || ["qué", "que", "quién", "quien", "es", "son", "por", "tiene", "tienen", "cuál", "puede", "pueden", "hay"].contains(toks.first ?? "")
        if !esPregunta {
            let nuevos = lee(t, quien: "tú")
            r.veredicto = nuevos.isEmpty ? "no saqué ningún hecho de eso" : "aprendido"
            r.aportes.append(("lector", nuevos.isEmpty ? "no entendí la forma (prueba: «el perro es un mamífero»)" : nuevos.joined(separator: " · ")))
            if !nuevos.isEmpty { evento(0, "leyó: " + nuevos.joined(separator: " · ")) }
            for _ in 0 ..< 6 { razona() }        // y deducen de inmediato
            return r
        }
        return pregunta(t, toks)
    }

    private func pregunta(_ t: String, _ toks: [String]) -> RespuestaLogica {
        var q = toks
        if q.first == "por" && q.count > 1 && q[1] == "qué" { q = Array(q.dropFirst(2)) }
        if q.first == "hay" && t.contains("contradic") { return informeContradicciones() }
        if (q.first == "qué" || q.first == "que"), q.count >= 3 {
            switch q[1] {
            case "es", "son": return queEs(ConsejoLogico.frase(Array(q.dropFirst(2))))
            case "tiene", "tienen": return queTiene(ConsejoLogico.frase(Array(q.dropFirst(2))))
            case "causa", "causan", "provoca", "provocan": return queCausa(ConsejoLogico.frase(Array(q.dropFirst(2))))
            case "pasa": return quePasaSi(ConsejoLogico.frase(Array(q.dropFirst(q.count > 2 && q[2] == "si" ? 3 : 2))))
            default: break
            }
        }
        if (q.first == "quién" || q.first == "quien" || q.first == "cuál"), let o = q.firstIndex(of: "o"), o > 2 {
            let comparativas: Set<String> = ["mayor", "más", "grande", "menor", "pequeño", "es"]
            let a = ConsejoLogico.frase(Array(q[1 ..< o]).filter { !comparativas.contains($0) })
            let b = ConsejoLogico.frase(Array(q[(o + 1)...]))
            return quienMayor(a, b)
        }
        // "¿es el perro un animal?" → "el perro es un animal"; "¿tiene pelo el gato?" → "el gato tiene pelo"
        var orden = q
        if let v = orden.first, ["es", "son", "tiene", "tienen", "puede", "pueden"].contains(v), orden.count >= 3 {
            orden = ConsejoLogico.reordena(v, Array(orden.dropFirst()))
        }
        if let h = hechoDe(orden) { return prueba(h) }
        var r = RespuestaLogica()
        r.aportes.append(("lector", "no sé leer esa pregunta (prueba: «¿es el perro un animal?», «¿qué es la ballena?», «¿qué pasa si llueve?»)"))
        return r
    }

    /// ¿Se cumple el hecho? sí / no / no sé, con los pasos.
    func prueba(_ h: Hecho) -> RespuestaLogica {
        var r = RespuestaLogica()
        switch h.rel {
        case .es, .noEs:
            let pos = demuestraEs(h.a, h.b)
            let neg = demuestraNoEs(h.a, h.b)
            if let p = pos, let n = neg {
                r.veredicto = "contradicción"
                r.explicacion = p + ["pero también:"] + n
                r.aportes.append(("contradicción", "hay pruebas de que sí y de que no"))
            } else if let p = pos {
                r.veredicto = h.rel == .es ? "sí" : "no"
                r.explicacion = p
                r.aportes.append(("silogismo", p.joined(separator: " ⇒ ")))
            } else if let n = neg {
                r.veredicto = h.rel == .es ? "no" : "sí"
                r.explicacion = n
                r.aportes.append(("negación", n.joined(separator: " ⇒ ")))
            }
        case .tiene, .noTiene:
            if let p = demuestraTiene(h.a, h.b) {
                r.veredicto = h.rel == .tiene ? "sí" : "no"
                r.explicacion = p
                r.aportes.append(("herencia", p.joined(separator: " ⇒ ")))
            } else if let n = demuestraNoTiene(h.a, h.b) {
                r.veredicto = h.rel == .tiene ? "no" : "sí"
                r.explicacion = n
                r.aportes.append(("negación", n.joined(separator: " ⇒ ")))
            }
        case .mayor, .causa, .parte:
            if let c = camino(h.a, h.rel, h.b) {
                r.veredicto = "sí"
                r.explicacion = pasos(c, h.rel)
                r.aportes.append((h.rel == .mayor ? "transitividad" : (h.rel == .causa ? "causal" : "categorías"), r.explicacion.joined(separator: " ⇒ ")))
            } else if let c = camino(h.b, h.rel, h.a) {
                r.veredicto = "no"
                r.explicacion = pasos(c, h.rel) + ["así que al revés no"]
                r.aportes.append(("comparador", r.explicacion.joined(separator: " ⇒ ")))
            }
        case .puede, .noPuede:
            if let p = demuestraPuede(h.a, h.b) {
                r.veredicto = h.rel == .puede ? "sí" : "no"
                r.explicacion = p
                r.aportes.append(("herencia", p.joined(separator: " ⇒ ")))
            } else if let n = demuestraNoPuede(h.a, h.b) {
                r.veredicto = h.rel == .puede ? "no" : "sí"
                r.explicacion = n
                r.aportes.append(("negación", n.joined(separator: " ⇒ ")))
            }
        }
        r.explicacion = expande(r.explicacion)
        r.aportes.append(("verificador", r.veredicto == "no sé" ? "no hay hechos que lo prueben ni lo nieguen" : "comprobado: \(r.veredicto)"))
        if !r.explicacion.isEmpty { r.aportes.append(("explicador", r.explicacion.joined(separator: " · "))) }
        evento(12, "\(h.texto)? → \(r.veredicto)")
        return r
    }

    /// Si un paso fue deducido, se explica de dónde salió: "perro es animal (perro es mamífero + mamífero es animal)".
    private func expande(_ pasos: [String]) -> [String] {
        var porTexto: [String: Apoyo] = [:]
        for (h, ap) in hechos where ap.quien != "leído" && ap.quien != "tú" { porTexto[h.texto] = ap }
        return pasos.map { p in
            guard let ap = porTexto[p], !ap.porque.isEmpty else { return p }
            return p + " (" + ap.porque.joined(separator: " + ") + ")"
        }
    }

    private func pasos(_ ruta: [String], _ rel: Rel) -> [String] {
        var out: [String] = []
        if ruta.count < 2 { return out }
        for i in 0 ..< ruta.count - 1 { out.append("\(ruta[i]) \(rel.texto) \(ruta[i + 1])") }
        return out
    }

    func demuestraEs(_ a: String, _ b: String) -> [String]? {
        if a == b { return ["\(a) es \(a)"] }
        guard let c = camino(a, .es, b) else { return nil }
        return pasos(c, .es)
    }

    /// a no es b si a (o algo que a es) no es b, o b no es algo que a es.
    func demuestraNoEs(_ a: String, _ b: String) -> [String]? {
        let lineaA = [a] + ancestros(a)
        for x in lineaA {
            if hechos[Hecho(a: x, rel: .noEs, b: b)] != nil {
                let subida = x == a ? [] : (demuestraEs(a, x) ?? [])
                return subida + ["\(x) no es \(b)"]
            }
            if hechos[Hecho(a: b, rel: .noEs, b: x)] != nil {
                let subida = x == a ? [] : (demuestraEs(a, x) ?? [])
                return subida + ["\(b) no es \(x)", "así que \(a) no es \(b)"]
            }
        }
        return nil
    }

    /// a tiene p si a (o algo que a es) tiene p — salvo que lo más cercano diga que no.
    func demuestraTiene(_ a: String, _ p: String) -> [String]? {
        for x in [a] + ancestros(a) {
            if hechos[Hecho(a: x, rel: .noTiene, b: p)] != nil { return nil }
            if hechos[Hecho(a: x, rel: .tiene, b: p)] != nil {
                let subida = x == a ? [] : (demuestraEs(a, x) ?? [])
                return subida + ["\(x) tiene \(p)"]
            }
        }
        return nil
    }

    /// Lo más cercano que lo niega ("el pingüino no puede volar" gana a "los pájaros pueden volar").
    func demuestraNoPuede(_ a: String, _ p: String) -> [String]? {
        return negacionCercana(a, p, si: .puede, no: .noPuede)
    }

    func demuestraNoTiene(_ a: String, _ p: String) -> [String]? {
        return negacionCercana(a, p, si: .tiene, no: .noTiene)
    }

    private func negacionCercana(_ a: String, _ p: String, si: Rel, no: Rel) -> [String]? {
        for x in [a] + ancestros(a) {
            if hechos[Hecho(a: x, rel: no, b: p)] != nil {
                let subida = x == a ? [] : (demuestraEs(a, x) ?? [])
                return subida + ["\(x) \(no.texto) \(p)"]
            }
            if hechos[Hecho(a: x, rel: si, b: p)] != nil { return nil }
        }
        return nil
    }

    func demuestraPuede(_ a: String, _ p: String) -> [String]? {
        for x in [a] + ancestros(a) {
            if hechos[Hecho(a: x, rel: .noPuede, b: p)] != nil { return nil }
            if hechos[Hecho(a: x, rel: .puede, b: p)] != nil {
                let subida = x == a ? [] : (demuestraEs(a, x) ?? [])
                return subida + ["\(x) puede \(p)"]
            }
        }
        return nil
    }

    private func queEs(_ a: String) -> RespuestaLogica {
        var r = RespuestaLogica()
        let anc = ancestros(a)
        if anc.isEmpty { return r }
        r.veredicto = "\(a) es " + anc.joined(separator: ", ")
        r.aportes.append(("categorías", r.veredicto))
        return r
    }

    private func queTiene(_ a: String) -> RespuestaLogica {
        var r = RespuestaLogica()
        var cosas: [String] = []
        for x in [a] + ancestros(a) {
            for p in salientes(x, .tiene) where !cosas.contains(p) && demuestraTiene(a, p) != nil { cosas.append(p) }
        }
        if cosas.isEmpty { return r }
        r.veredicto = "\(a) tiene " + cosas.joined(separator: ", ")
        r.aportes.append(("herencia", r.veredicto))
        return r
    }

    private func queCausa(_ b: String) -> RespuestaLogica {
        var r = RespuestaLogica()
        var causas: [String] = []
        for (h, _) in hechos where h.rel == .causa && h.b == b { causas.append(h.a) }
        for (g, _) in reglas where g.entonces == b { causas.append(g.si) }
        if causas.isEmpty { return r }
        r.veredicto = causas.joined(separator: ", ")
        r.aportes.append(("abducción", "\(b) puede venir de: " + r.veredicto))
        return r
    }

    /// Encadena reglas y causas a partir de una proposición.
    private func quePasaSi(_ a: String) -> RespuestaLogica {
        var r = RespuestaLogica()
        var verdades: [String] = [a]
        var i = 0
        while i < verdades.count && verdades.count < 12 {
            let x = verdades[i]
            for (g, _) in reglas where g.si == x && !verdades.contains(g.entonces) {
                verdades.append(g.entonces)
                r.explicacion.append("si \(g.si) entonces \(g.entonces)")
            }
            for (h, _) in hechos where h.rel == .causa && h.a == x && !verdades.contains(h.b) {
                verdades.append(h.b)
                r.explicacion.append("\(h.a) causa \(h.b)")
            }
            i += 1
        }
        if verdades.count > 1 {
            r.veredicto = verdades.dropFirst().joined(separator: ", ")
            r.aportes.append(("modus ponens", r.explicacion.joined(separator: " ⇒ ")))
        }
        return r
    }

    private func quienMayor(_ a: String, _ b: String) -> RespuestaLogica {
        var r = RespuestaLogica()
        if let c = camino(a, .mayor, b) {
            r.veredicto = a
            r.explicacion = pasos(c, .mayor)
        } else if let c = camino(b, .mayor, a) {
            r.veredicto = b
            r.explicacion = pasos(c, .mayor)
        }
        if !r.explicacion.isEmpty { r.aportes.append(("comparador", r.explicacion.joined(separator: " ⇒ "))) }
        return r
    }

    private func informeContradicciones() -> RespuestaLogica {
        var r = RespuestaLogica()
        buscaContradicciones()
        r.veredicto = contradicciones.isEmpty ? "no hay contradicciones" : "\(contradicciones.count) contradicciones"
        r.explicacion = contradicciones
        r.aportes.append(("contradicción", r.veredicto))
        return r
    }

    // MARK: razonar por su cuenta (los 18 agentes)

    /// Un paso de razonamiento: le toca a un agente.
    func razona() {
        turno += 1
        let a = turno % 18
        switch a {
        case 0: leePendiente()
        case 1: silogismo()
        case 2: herencia()
        case 3: transitividad(.mayor, agente: 3)
        case 4: transitividad(.causa, agente: 4)
        case 5: modusPonens()
        case 6: negacion()
        case 7: buscaContradicciones()
        case 8: induccion()
        case 9: abduccion()
        case 10: contraejemplo()
        case 11: revisor()
        case 12: verificador()
        case 13: break                        // aritmética: trabaja cuando le preguntas
        case 14: comparador()
        case 15: categorias()
        case 16: cuantificador()
        default: explicador()
        }
    }

    /// Frases que el lector tiene pendientes (por ejemplo, lo que sabe el consejo nuevo).
    func encola(_ frases: [String]) {
        porLeer.append(contentsOf: frases)
    }

    private func leePendiente() {
        var leidas = 0
        while leidas < 5, !porLeer.isEmpty {
            let f = porLeer.removeFirst()
            leidas += 1
            let n = lee(f, quien: "leído")
            if !n.isEmpty { evento(0, "leyó «\(f)» → " + n.joined(separator: " · ")) }
        }
    }

    private func deduce(_ h: Hecho, _ agente: Int, _ porque: [String], conf: Float, hipotesis: Bool = false) {
        if h.a == h.b { return }
        if let c = h.rel.contraria, hechos[Hecho(a: h.a, rel: c, b: h.b)] != nil, !hipotesis { return }
        var ap = Apoyo(confianza: conf, quien: agentesLogicos[agente], porque: porque)
        ap.hipotesis = hipotesis
        if pon(h, ap) {
            evento(agente, (hipotesis ? "supone " : "deduce ") + h.texto + "   (" + porque.joined(separator: " + ") + ")")
        }
    }

    private func conf(_ h: Hecho) -> Float { hechos[h]?.confianza ?? 0 }

    /// X es A, A es B ⇒ X es B
    private func silogismo() {
        let es = hechos.filter { $0.key.rel == .es && !$0.value.hipotesis }.map { $0.key }
        for h1 in es.shuffled().prefix(30) {
            for b in salientes(h1.b, .es) {
                let nuevo = Hecho(a: h1.a, rel: .es, b: b)
                if hechos[nuevo] == nil {
                    deduce(nuevo, 1, [h1.texto, "\(h1.b) es \(b)"], conf: min(conf(h1), conf(Hecho(a: h1.b, rel: .es, b: b))) * 0.95)
                    return
                }
            }
        }
    }

    /// X es A, A tiene P ⇒ X tiene P (si nada lo impide)
    private func herencia() {
        let es = hechos.filter { $0.key.rel == .es }.map { $0.key }
        for h1 in es.shuffled().prefix(30) {
            for rel in [Rel.tiene, Rel.puede] {
                for p in salientes(h1.b, rel) {
                    let nuevo = Hecho(a: h1.a, rel: rel, b: p)
                    if hechos[nuevo] == nil {
                        deduce(nuevo, 2, [h1.texto, "\(h1.b) \(rel.texto) \(p)"], conf: conf(h1) * 0.9)
                        return
                    }
                }
            }
        }
    }

    /// A r B, B r C ⇒ A r C (mayor, causa)
    private func transitividad(_ rel: Rel, agente: Int) {
        let hs = hechos.filter { $0.key.rel == rel }.map { $0.key }
        for h1 in hs.shuffled().prefix(30) {
            for c in salientes(h1.b, rel) where c != h1.a {
                let nuevo = Hecho(a: h1.a, rel: rel, b: c)
                if hechos[nuevo] == nil {
                    deduce(nuevo, agente, [h1.texto, "\(h1.b) \(rel.texto) \(c)"], conf: conf(h1) * 0.9)
                    return
                }
            }
        }
    }

    /// si A entonces B, A ⇒ B
    private func modusPonens() {
        for (g, _) in reglas where props[g.si] != nil && props[g.entonces] == nil {
            props[g.entonces] = Apoyo(confianza: 0.9, quien: "modus ponens", porque: ["si \(g.si) entonces \(g.entonces)", g.si])
            evento(5, "deduce «\(g.entonces)»   (si \(g.si) entonces \(g.entonces) + \(g.si))")
            return
        }
    }

    /// X es A, A no es B ⇒ X no es B
    private func negacion() {
        let no = hechos.filter { $0.key.rel == .noEs }.map { $0.key }
        for h1 in no.shuffled().prefix(20) {
            for (h, _) in hechos where h.rel == .es && h.b == h1.a {
                let nuevo = Hecho(a: h.a, rel: .noEs, b: h1.b)
                if hechos[nuevo] == nil && hechos[Hecho(a: h.a, rel: .es, b: h1.b)] == nil {
                    deduce(nuevo, 6, [h.texto, h1.texto], conf: conf(h) * 0.9)
                    return
                }
            }
        }
    }

    func buscaContradicciones() {
        var lista: [String] = []
        for (h, _) in hechos {
            guard let c = h.rel.contraria, h.rel == .es || h.rel == .tiene || h.rel == .puede else { continue }
            if hechos[Hecho(a: h.a, rel: c, b: h.b)] != nil { lista.append("\(h.texto) ↔ \(h.a) \(c.texto) \(h.b)") }
        }
        for (h, _) in hechos where h.rel == .mayor && hechos[Hecho(a: h.b, rel: .mayor, b: h.a)] != nil && h.a < h.b {
            lista.append("\(h.texto) ↔ \(h.b) es mayor que \(h.a)")
        }
        if lista.count > contradicciones.count, let nueva = lista.first(where: { !contradicciones.contains($0) }) {
            evento(7, "¡contradicción! " + nueva)
        }
        contradicciones = lista
    }

    /// Si varios de una categoría tienen algo y ninguno lo niega ⇒ "probablemente todos".
    private func induccion() {
        var miembros: [String: [String]] = [:]
        for (h, _) in hechos where h.rel == .es { miembros[h.b, default: []].append(h.a) }
        for (cat, lista) in miembros.shuffled() where lista.count >= 2 {
            var cuenta: [String: Int] = [:]
            for x in lista { for p in salientes(x, .tiene) { cuenta[p, default: 0] += 1 } }
            for (p, n) in cuenta where n >= 2 && n * 3 >= lista.count * 2 {
                let nuevo = Hecho(a: cat, rel: .tiene, b: p)
                if hechos[nuevo] == nil && hechos[Hecho(a: cat, rel: .noTiene, b: p)] == nil {
                    deduce(nuevo, 8, ["\(n) de \(lista.count) \(cat) tienen \(p)"], conf: 0.6, hipotesis: true)
                    return
                }
            }
        }
    }

    /// B es verdad y "si A entonces B" ⇒ quizás A.
    private func abduccion() {
        for (g, _) in reglas where props[g.entonces] != nil && props[g.si] == nil {
            evento(9, "quizás \(g.si)   (porque \(g.entonces), y si \(g.si) entonces \(g.entonces))")
            return
        }
    }

    /// Busca un caso que refute una generalización inducida.
    private func contraejemplo() {
        for (h, ap) in hechos where ap.hipotesis && h.rel == .tiene {
            for (m, _) in hechos where m.rel == .es && m.b == h.a {
                if hechos[Hecho(a: m.a, rel: .noTiene, b: h.b)] != nil {
                    hechos[h] = nil
                    evento(10, "refuta «\(h.texto)»: \(m.a) es \(h.a) y no tiene \(h.b)")
                    return
                }
            }
        }
    }

    /// Ante una contradicción, se queda con lo más confiable.
    private func revisor() {
        for (h, ap) in hechos {
            guard let c = h.rel.contraria else { continue }
            let otro = Hecho(a: h.a, rel: c, b: h.b)
            if let ap2 = hechos[otro], ap2.confianza < ap.confianza {
                hechos[otro] = nil
                evento(11, "descarta «\(otro.texto)» (confianza \(fmt(ap2.confianza))) y se queda con «\(h.texto)» (\(fmt(ap.confianza)))")
                return
            }
        }
    }

    /// Comprueba que lo deducido todavía se sostiene.
    private func verificador() {
        for (h, ap) in hechos.shuffled().prefix(20) where ap.quien == "silogismo" && h.rel == .es {
            if camino(h.a, .es, h.b, hasta: 8) == nil || demuestraEsSin(h) == nil {
                hechos[h] = nil
                evento(12, "retira «\(h.texto)»: ya no hay cadena que lo pruebe")
                return
            }
        }
    }

    /// Prueba a→b sin usar el hecho directo (para verificar deducciones).
    private func demuestraEsSin(_ h: Hecho) -> [String]? {
        let guardado = hechos[h]
        hechos[h] = nil
        let r = demuestraEs(h.a, h.b)
        hechos[h] = guardado
        return r
    }

    private func comparador() {
        var cuenta: [String: Int] = [:]
        for (h, _) in hechos where h.rel == .mayor { cuenta[h.a, default: 0] += 1 }
        if let top = cuenta.max(by: { $0.value < $1.value }), top.value >= 2 {
            evento(14, "\(top.key) es mayor que \(top.value) cosas")
        }
    }

    private func categorias() {
        var miembros: [String: Int] = [:]
        for (h, _) in hechos where h.rel == .es { miembros[h.b, default: 0] += 1 }
        if let top = miembros.max(by: { $0.value < $1.value }), top.value >= 3 {
            evento(15, "«\(top.key)» agrupa \(top.value) cosas")
        }
    }

    private func cuantificador() {
        let hip = hechos.filter { $0.value.hipotesis }.count
        if hip > 0 { evento(16, "\(hip) generalizaciones son solo probables (no 'todos' seguro)") }
    }

    private func explicador() {
        let deducidos = hechos.filter { $0.value.quien != "leído" && $0.value.quien != "tú" }
        guard let (h, ap) = deducidos.randomElement() else { return }
        evento(17, "«\(h.texto)» porque " + ap.porque.joined(separator: " y "))
    }

    // MARK: memoria

    func exporta() -> String {
        var l: [String] = []
        for (h, ap) in hechos {
            l.append(["H", h.a, h.rel.rawValue, h.b, "\(ap.confianza)", ap.quien, ap.hipotesis ? "1" : "0", ap.porque.joined(separator: " ¦ ")].joined(separator: "\t"))
        }
        for (g, ap) in reglas { l.append(["R", g.si, g.entonces, ap.quien].joined(separator: "\t")) }
        for (p, ap) in props { l.append(["P", p, ap.quien].joined(separator: "\t")) }
        return l.joined(separator: "\n")
    }

    static func desde(_ texto: String) -> ConsejoLogico {
        let c = ConsejoLogico(primer: false)
        for linea in texto.split(separator: "\n") {
            let p = linea.components(separatedBy: "\t")
            if p.count >= 8, p[0] == "H", let rel = Rel(rawValue: p[2]) {
                var ap = Apoyo(confianza: Float(p[4]) ?? 1, quien: p[5], porque: p[7].components(separatedBy: " ¦ "))
                ap.hipotesis = p[6] == "1"
                c.hechos[Hecho(a: p[1], rel: rel, b: p[3])] = ap
            } else if p.count >= 4, p[0] == "R" {
                c.reglas[Regla(si: p[1], entonces: p[2])] = Apoyo(confianza: 1, quien: p[3], porque: [])
            } else if p.count >= 3, p[0] == "P" {
                c.props[p[1]] = Apoyo(confianza: 1, quien: p[2], porque: [])
            }
        }
        return c
    }

    /// Lo que saben al nacer: unas cuantas verdades con forma lógica.
    static let cartilla: [String] = [
        "el perro es un mamífero", "el gato es un mamífero", "la ballena es un mamífero",
        "los mamíferos son animales", "los peces son animales", "los pájaros son animales",
        "el tiburón es un pez", "el águila es un pájaro", "el pingüino es un pájaro",
        "ningún pez es un mamífero", "los animales son seres vivos", "las plantas son seres vivos",
        "el árbol es una planta", "los mamíferos tienen pelo", "los pájaros tienen plumas",
        "los peces tienen escamas", "la ballena no tiene pelo", "los pájaros pueden volar",
        "el pingüino no puede volar", "los seres vivos necesitan agua",
        "el elefante es un mamífero", "el elefante es más grande que el caballo",
        "el caballo es más grande que el perro", "el perro es más grande que el gato",
        "la lluvia causa charcos", "las nubes causan lluvia", "el fuego causa calor",
        "si llueve entonces el suelo está mojado", "si el suelo está mojado entonces resbala",
        "el sol es una estrella", "las estrellas dan luz",
    ]
}

/// Cuentas: "cuánto es 3 + 4 * 2", "(2 + 3) * 4", "10 / 4".
enum Aritmetica {
    static func expresion(_ t: String) -> String {
        var s = t.lowercased()
        for (a, b) in [("cuánto es", ""), ("cuanto es", ""), ("calcula", ""), ("?", ""), ("¿", ""),
                       (" más ", " + "), (" menos ", " - "), (" por ", " * "), (" entre ", " / "), ("x", "*")] {
            s = s.replacingOccurrences(of: a, with: b)
        }
        return s.trimmingCharacters(in: .whitespaces)
    }

    static func calcula(_ t: String) -> Double? {
        let e = expresion(t)
        if e.isEmpty || !e.contains(where: { $0.isNumber }) { return nil }
        let permitidos = Set("0123456789.+-*/() ")
        if !e.allSatisfy({ permitidos.contains($0) }) { return nil }
        var p = Parser(Array(e.filter { $0 != " " }))
        guard let v = p.suma(), p.i == p.c.count else { return nil }
        return v
    }

    static func bonito(_ v: Double) -> String {
        if v == v.rounded() && abs(v) < 1e15 { return String(Int(v)) }
        return String(format: "%.4g", v)
    }

    struct Parser {
        let c: [Character]
        var i = 0
        init(_ c: [Character]) { self.c = c }

        mutating func suma() -> Double? {
            guard var v = producto() else { return nil }
            while i < c.count, c[i] == "+" || c[i] == "-" {
                let op = c[i]
                i += 1
                guard let w = producto() else { return nil }
                v = op == "+" ? v + w : v - w
            }
            return v
        }

        mutating func producto() -> Double? {
            guard var v = factor() else { return nil }
            while i < c.count, c[i] == "*" || c[i] == "/" {
                let op = c[i]
                i += 1
                guard let w = factor() else { return nil }
                if op == "/" && w == 0 { return nil }
                v = op == "*" ? v * w : v / w
            }
            return v
        }

        mutating func factor() -> Double? {
            if i < c.count, c[i] == "-" {
                i += 1
                return factor().map { -$0 }
            }
            if i < c.count, c[i] == "(" {
                i += 1
                let v = suma()
                if i < c.count, c[i] == ")" { i += 1 } else { return nil }
                return v
            }
            var s = ""
            while i < c.count, c[i].isNumber || c[i] == "." {
                s.append(c[i])
                i += 1
            }
            return Double(s)
        }
    }
}
