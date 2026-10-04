import Foundation

// Imaginacion.swift — Nyx imagina: junta lo que sabe de verdad de varias
// cosas para inventar algo nuevo («imagina un pez que vuela», «¿qué pasaría
// si no hubiera sol?»). Lo imaginado se dice como imaginado: no se guarda
// como hecho, y la lógica avisa cuando en la realidad no es posible.

extension NyxUno {
    static let disparadores = ["imagina", "imaginemos", "imaginate", "imagínate", "inventa", "sueña",
                               "qué pasaría", "que pasaria", "qué pasaria", "que pasaría",
                               "cómo sería", "como seria", "cómo seria", "como sería", "y si "]

    static func esImaginar(_ t: String) -> Bool {
        let b = t.lowercased().replacingOccurrences(of: "¿", with: "").trimmingCharacters(in: .whitespaces)
        return disparadores.contains { b.hasPrefix($0) }
    }

    /// La definición que recuerda de una palabra («motor es la máquina…»), si la tiene.
    func definicion(_ w: String) -> String? {
        let memoria = consejo.frases
        let sw = ConsejoLogico.singular(w)
        var mejor: String? = nil
        var mejorN = Int.max
        for k in memoria.candidatas([MemoriaFrases.raiz(w)]) {
            let f = memoria.frases[k]
            guard let primera = f.toks.first(where: { !Palabras.vacia($0) }), f.toks.contains("es") else { continue }
            if primera == w || ConsejoLogico.singular(primera) == sw {
                let n = (primera == w ? 0 : 100) + f.toks.count
                if n < mejorN { mejorN = n; mejor = f.texto }
            }
        }
        return mejor
    }

    /// El verbo en infinitivo («vuela» → volar), si es un verbo que conoce.
    func infinitivo(_ w: String) -> String? {
        if w.count > 3 && (w.hasSuffix("ar") || w.hasSuffix("er") || w.hasSuffix("ir")) { return w }
        guard let d = definicion(w) else { return nil }
        let toks = Palabras.tokens(d)
        if let i = toks.firstIndex(of: "verbo"), i + 1 < toks.count { return toks[i + 1] }
        return nil
    }

    private func rasgosTexto(_ x: String, _ n: Int, solo: Set<Rel>? = nil) -> [String] {
        var out: [String] = []
        let gramatica = ["forma", "plural", "femenino", "masculino", "participio", "gerundio", "verbo", "adjetivo", "adverbio", "pronombre", "preposición"]
        let todos = logica.rasgos(x, maximo: 16).filter { r in !gramatica.contains { r.b.hasPrefix($0) || r.b.contains(" verbo") } }
        let orden = todos.filter { $0.rel != .es } + todos.filter { $0.rel == .es }
        for r in orden where solo?.contains(r.rel) ?? true {
            out.append(r.rel.texto + " " + r.b)
            if out.count >= n { break }
        }
        return out
    }

    private func une(_ xs: [String]) -> String {
        if xs.count <= 1 { return xs.first ?? "" }
        return xs.dropLast().joined(separator: ", ") + " y " + xs.last!
    }

    /// Imagina a partir de lo que le pides.
    func imagina(_ texto: String) -> Pensamiento {
        var p = Pensamiento()
        p.pregunta = texto
        p.clase = .imagina
        var t = texto.lowercased().replacingOccurrences(of: "¿", with: "").replacingOccurrences(of: "?", with: "")
        var hipotesis = false
        for d in NyxUno.disparadores where t.hasPrefix(d) {
            hipotesis = d.contains("pasar") || d == "y si "
            t = String(t.dropFirst(d.count)).trimmingCharacters(in: .whitespaces)
            break
        }
        if t.hasPrefix("si ") { t = String(t.dropFirst(3)); hipotesis = true }
        p.pensamiento.append("🧩 Entiendo: me pides imaginar «\(t)».")
        let partes = hipotesis ? imaginaSi(t, &p) : imaginaCosa(t, &p)
        var frase = partes.joined(separator: ". ")
        if frase.isEmpty {
            frase = "no sé lo bastante de «\(t)» para imaginarlo. Cuéntame algo y lo intento"
            p.pensamiento.append("❓ No recuerdo nada de eso con lo que imaginar.")
        }
        p.pensamiento.append("🌈 Imagino (no es un hecho: es imaginación).")
        p.elegido = Candidato(sistema: .imaginacion, respuesta: frase, frase: frase, confianza: 0.5, pasos: [])
        p.confianza = 0.5
        p.es = "🌈 " + frase
        p.resh = Resh.traduceTexto(frase)
        imaginadas.append(frase)
        if imaginadas.count > 50 { imaginadas.removeFirst() }
        return p
    }

    /// «un pez que vuela»: mezcla lo que sabe de cada cosa.
    private func imaginaCosa(_ t: String, _ p: inout Pensamiento) -> [String] {
        let toks = Palabras.tokens(t).filter { !Palabras.vacia($0) && $0.count > 2 && !["como", "fuera", "hubiera", "tuviera"].contains($0) }
        var cosas: [String] = []
        var verbos: [String] = []
        for w in toks {
            if let v = infinitivo(w), v != w || logica.rasgos(w, maximo: 1).isEmpty {
                verbos.append(v)           // «vuela» → volar
            } else {
                cosas.append(ConsejoLogico.singular(w))
            }
        }
        guard let principal = cosas.first else { return [] }
        var out: [String] = []
        out.append("Imagino \(t)")
        let propios = rasgosTexto(principal, 3)
        if !propios.isEmpty {
            out.append("Como \(principal), \(une(propios))")
            p.pensamiento.append("📖 Recuerdo que \(principal) " + une(propios) + ".")
        } else if let d = definicion(principal) {
            out.append("Sé que " + d)
            p.pensamiento.append("📖 Recuerdo: " + d + ".")
        }
        for v in verbos.prefix(2) {
            let quien = logica.quienes(v, .puede).filter { $0 != principal }
            if let c = quien.first {
                let prestado = rasgosTexto(c, 2, solo: [.tiene])
                let como = prestado.isEmpty ? "" : ": " + une(prestado)
                out.append("Para \(v) tomaría algo de \(c)\(como)")
                p.pensamiento.append("🔗 Busco quién puede \(v): \(une(Array(quien.prefix(3)))).")
            } else if let d = definicion(v) {
                // nadie que yo sepa puede hacerlo: uso lo que significa
                let que = d.components(separatedBy: " es ").dropFirst().joined(separator: " es ")
                if !que.isEmpty {
                    out.append("Podría \(v), es decir, \(que)")
                    p.pensamiento.append("📖 Recuerdo qué es \(v): \(que).")
                }
            }
            if logica.prueba(Hecho(a: principal, rel: .puede, b: v)).veredicto == "no" {
                out.append("En la realidad \(principal) no puede \(v): por eso es imaginación")
                p.pensamiento.append("⚖️ Compruebo: en la realidad \(principal) no puede \(v).")
            }
        }
        for y in cosas.dropFirst().prefix(2) where y != principal {
            let suyos = rasgosTexto(y, 2)
            if !suyos.isEmpty { out.append("Y como \(y), \(une(suyos))") }
            if logica.prueba(Hecho(a: principal, rel: .es, b: y)).veredicto == "no" {
                out.append("En la realidad \(principal) no es \(y): por eso es imaginación")
                p.pensamiento.append("⚖️ Compruebo: en la realidad \(principal) no es \(y).")
            }
        }
        return out.count > 1 ? out : []
    }

    /// «¿qué pasaría si llueve?»: encadena causas y reglas, y lo que asocia.
    private func imaginaSi(_ t: String, _ p: inout Pensamiento) -> [String] {
        var out: [String] = []
        let r = logica.procesa("qué pasa si \(t)")
        if r.veredicto != "no sé" && !r.veredicto.isEmpty {
            out.append("Si \(t), pasaría esto: \(r.veredicto)")
            p.pensamiento.append("⚖️ Encadeno lo que sé: " + r.explicacion.joined(separator: " ⇒ ") + ".")
        }
        let toks = Palabras.tokens(t).filter { !Palabras.vacia($0) && $0.count > 2 }
        for w in toks.prefix(2) {
            if let d = definicion(w) {
                out.append("Sé que " + d)
                p.pensamiento.append("📖 Recuerdo: " + d + ".")
            }
        }
        let a = consejo.delibera(t)
        if Palabras.tokens(a.frase).count >= 4 && Respuestas.norma(a.frase) != Respuestas.norma(t) {
            out.append("Me imagino que " + a.frase)
            p.pensamiento.append("✨ La asociación imagina: " + a.frase + ".")
        }
        return out
    }

    /// Imaginar sola: junta dos cosas que conoce y se pregunta cómo sería.
    func sueña() -> Pensamiento {
        var sujetos: [String] = []
        for h in logica.hechos.keys where h.rel == .es && h.a.count > 3 && !h.a.contains(" ") { sujetos.append(h.a) }
        // cosas concretas: con partes o con algo que pueden hacer
        sujetos = Array(Set(sujetos)).filter { x in logica.rasgos(x, maximo: 8).contains { $0.rel == .tiene || $0.rel == .puede } }
        guard sujetos.count >= 2 else {
            var p = Pensamiento()
            p.es = "todavía sé muy pocas cosas para imaginar. Enséñame algo"
            p.resh = Resh.traduceTexto(p.es)
            return p
        }
        let a = sujetos[Azar.ent(sujetos.count)]
        var b = a
        var intentos = 0
        // mejor dos cosas de clases distintas (más imaginativo)
        while intentos < 30 && (b == a || logica.ancestros(a).first == logica.ancestros(b).first) {
            b = sujetos[Azar.ent(sujetos.count)]
            intentos += 1
        }
        if b == a { b = sujetos[(sujetos.firstIndex(of: a)! + 1) % sujetos.count] }
        return imagina("imagina un \(a) que fuera como un \(b)")
    }
}
