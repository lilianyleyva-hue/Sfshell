import Foundation

// NyxUno.swift — una sola IA hecha con lo mejor de todos los consejos.
//
// No son tres consejos hablando: es UNA mente con una sola memoria y varias
// formas de pensar, que se reparten el trabajo:
//
//   · razonamiento exacto  (del consejo lógico): hechos, reglas, pruebas, cuentas;
//   · asociación           (del consejo nuevo): las 18 mentes resonantes, sus frases,
//                           el Resh y lo que ven y oyen;
//   · estrategias          (de las misiones): las 18 mentes con lo que aprendieron
//                           sobre cómo resolver cada tipo de reto;
//   · recuerdo             : lo que ya resolvió y le confirmaste.
//
// Del consejo antiguo toma sus ideas, no su motor (era el que menos acertaba):
// el crítico que inhibe la respuesta que contradice lo que se sabe, debatir
// más rondas cuando la decisión está reñida, y calibrar cuánto se fía de cada
// parte según sus aciertos.
//
// Y lo que se le añadió para mejorarla:
//   · metacognición: aprende en qué clase de pregunta acierta cada forma de pensar;
//   · verificación: comprueba la respuesta por otro camino (sustituye la x, etc.);
//   · pensar más cuando duda: si la decisión está reñida o insegura, deduce más
//     y vuelve a decidir (hasta 3 rondas);
//   · honestidad: si su confianza es baja dice "no estoy segura" o "no lo sé"
//     y pide que se lo enseñes;
//   · aprende de tus correcciones ("no, es 12");
//   · se entrena sola con retos y consolida lo que sabe (como dormir).

enum Clase: Int, CaseIterable {
    case cuenta, serie, ecuacion, problema, siNo, definicion, causa, comparacion, intruso, abierta

    var nombre: String {
        switch self {
        case .cuenta: return "cuenta"
        case .serie: return "serie"
        case .ecuacion: return "ecuación"
        case .problema: return "problema"
        case .siNo: return "sí o no"
        case .definicion: return "qué es"
        case .causa: return "causas"
        case .comparacion: return "comparar"
        case .intruso: return "cuál sobra"
        case .abierta: return "pregunta abierta"
        }
    }

    /// La misión equivalente (para las clases que son retos).
    var tipoMision: TipoMision? {
        switch self {
        case .cuenta: return .cuenta
        case .serie: return .serie
        case .ecuacion: return .ecuacion
        case .problema: return .problema
        case .comparacion: return .orden
        case .intruso: return .intruso
        default: return nil
        }
    }

    static func de(_ t: TipoMision) -> Clase {
        switch t {
        case .cuenta: return .cuenta
        case .serie: return .serie
        case .ecuacion: return .ecuacion
        case .problema: return .problema
        case .silogismo: return .siNo
        case .orden: return .comparacion
        case .intruso: return .intruso
        }
    }
}

enum Sistema: Int, CaseIterable {
    case logico, asociativo, estrategias, recuerdo, busqueda

    var nombre: String {
        switch self {
        case .logico: return "razonamiento"
        case .asociativo: return "asociación"
        case .estrategias: return "estrategias"
        case .recuerdo: return "recuerdo"
        case .busqueda: return "búsqueda"
        }
    }

    var icono: String {
        switch self {
        case .logico: return "⚖️"
        case .asociativo: return "✨"
        case .estrategias: return "🎯"
        case .recuerdo: return "📌"
        case .busqueda: return "🔎"
        }
    }
}

struct Candidato {
    let sistema: Sistema
    let respuesta: String       // corta: "144", "sí", "gala" o una frase
    let frase: String           // lo que diría
    var confianza: Float        // 0…1, lo que el propio sistema cree
    var pasos: [String] = []
    var nota = ""               // "verificado", "el crítico lo descartó…"
    var puntos: Float = 0       // después de pesar cuánto se fía de él
}

struct Pensamiento {
    var pregunta = ""
    var clase = Clase.abierta
    var candidatos: [Candidato] = []
    var elegido: Candidato? = nil
    var confianza: Float = 0
    var rondas = 1
    var segunda: Float = 0      // lo que pesa la segunda mejor respuesta
    var es = ""
    var resh = ""
    var aprendio = ""           // si era algo que le enseñaste
}

final class NyxUno {
    var consejo: Consejo
    var logica: ConsejoLogico
    var saber: SaberMisiones
    /// [clase][sistema]: aciertos y fallos (empieza con lo que se espera de cada uno).
    private(set) var aciertos: [[Float]] = []
    private(set) var fallos: [[Float]] = []
    /// Preguntas ya resueltas y confirmadas.
    var recuerdos: [String: String] = [:]
    private(set) var ultimo: Pensamiento? = nil
    /// Lo que no supo y quiere que le enseñes.
    private(set) var pendiente: String? = nil
    var entrenadas = 0
    var entrenadasBien = 0

    init(consejo: Consejo, logica: ConsejoLogico, saber: SaberMisiones) {
        self.consejo = consejo
        self.logica = logica
        self.saber = saber
        reinicia()
    }

    /// Lo que espera de cada forma de pensar antes de tener experiencia.
    func reinicia() {
        aciertos = []
        fallos = []
        for c in Clase.allCases {
            var a: [Float] = []
            var f: [Float] = []
            for s in Sistema.allCases {
                let p = NyxUno.previo(c, s)
                a.append(p.0)
                f.append(p.1)
            }
            aciertos.append(a)
            fallos.append(f)
        }
        recuerdos = [:]
        entrenadas = 0
        entrenadasBien = 0
    }

    private static func previo(_ c: Clase, _ s: Sistema) -> (Float, Float) {
        switch s {
        case .recuerdo: return (9, 1)
        case .logico: return c == .abierta ? (1, 3) : (8, 1)
        case .asociativo: return c == .abierta ? (6, 2) : ([.definicion, .causa, .siNo].contains(c) ? (2, 2) : (1, 4))
        case .estrategias: return c.tipoMision != nil || c == .siNo ? (2, 2) : (0.5, 4)
        case .busqueda: return c == .abierta ? (5, 2) : ([.definicion, .causa].contains(c) ? (3, 2) : (0.5, 4))
        }
    }

    /// Cuánto se fía de un sistema en una clase de pregunta (0…1).
    func fiabilidad(_ c: Clase, _ s: Sistema) -> Float {
        let a = aciertos[c.rawValue][s.rawValue]
        let f = fallos[c.rawValue][s.rawValue]
        return a / (a + f)
    }

    private func anota(_ c: Clase, _ s: Sistema, bien: Bool) {
        if bien { aciertos[c.rawValue][s.rawValue] += 1 } else { fallos[c.rawValue][s.rawValue] += 1 }
    }

    // MARK: - entender qué le preguntan

    private static let interrogativas: Set<String> = ["qué", "que", "quién", "quien", "cómo", "como", "dónde", "donde", "cuándo", "cuál", "cual", "por", "para", "cuánto", "cuántos", "cuántas", "de"]

    func clasifica(_ texto: String) -> Clase {
        let t = texto.lowercased()
        let toks = Palabras.tokens(t, max: 60)
        let numeros = Resuelve.numerosDe(t)
        if t.contains("="), toks.contains(where: { $0.contains("x") }) { return .ecuacion }
        if numeros.count >= 4, t.filter({ $0 == "," }).count >= 3 { return .serie }
        if NyxUno.cuentaDe(t) != nil { return .cuenta }
        if t.contains("cuál no es") || t.contains("cual no es") { return .intruso }
        if t.contains("quién es más") || t.contains("quien es más") || t.contains("quién es mayor") { return .comparacion }
        if !numeros.isEmpty, t.contains("cuánt") { return .problema }
        if t.contains("todos los") || t.contains("ningún") { return .siNo }
        guard let p = toks.first else { return .abierta }
        if ["es", "son", "tiene", "tienen", "puede", "pueden"].contains(p) { return .siNo }
        if toks.count >= 2, p == "qué" || p == "que" {
            if ["es", "son", "tiene", "tienen"].contains(toks[1]) { return .definicion }
            if ["causa", "causan", "provoca", "pasa"].contains(toks[1]) { return .causa }
        }
        if p == "por", toks.count > 1, toks[1] == "qué" { return .causa }
        return .abierta
    }

    /// "hola, ¿cuánto es 12 por 12?" → "12 por 12"
    static func cuentaDe(_ t: String) -> String? {
        let bajo = t.lowercased()
        for clave in ["cuánto es", "cuanto es", "calcula", "cuánto da", "cuanto da"] {
            if let r = bajo.range(of: clave) {
                let resto = String(bajo[r.upperBound...])
                if Aritmetica.opera(resto), Aritmetica.resultado(resto) != nil { return resto }
            }
        }
        if Aritmetica.opera(bajo), !bajo.contains(","), Aritmetica.resultado(bajo) != nil { return bajo }
        return nil
    }

    private func esPregunta(_ t: String) -> Bool {
        if t.contains("?") { return true }
        guard let p = Palabras.tokens(t).first else { return false }
        return NyxUno.interrogativas.contains(p) || ["es", "son", "tiene", "puede", "hay"].contains(p)
    }

    // MARK: - pensar

    /// Lo que le dices. Si es una pregunta, piensa y contesta; si es una
    /// afirmación, la aprende; si es una corrección, aprende de su error.
    func escucha(_ texto: String) -> Pensamiento {
        var t = Resh.traduceConservando(texto.trimmingCharacters(in: .whitespacesAndNewlines))
        let bajo = t.lowercased().trimmingCharacters(in: .whitespaces)
        for p in ["misión:", "mision:", "reto:", "misión", "reto"] where bajo.hasPrefix(p) {
            t = String(t.dropFirst(p.count)).trimmingCharacters(in: .whitespaces)
            break
        }
        if let c = correccion(bajo) { return corrige(c) }
        // solo un número ("67"): no es algo que aprender ni una cuenta
        if !t.contains(where: { $0.isLetter }), !Aritmetica.opera(t), clasifica(t) == .abierta {
            var p = Pensamiento()
            p.pregunta = t
            p.es = "\(t.trimmingCharacters(in: .whitespaces)) es un número. ¿Qué hago con él? Por ejemplo: «\(t) por 3», «\(t) al cuadrado» o «misión: \(t), \(t) más 3…»"
            if let n = Grande(t.trimmingCharacters(in: .whitespaces)), n.cifras < 40 {
                p.es = "\(n.texto) es un número. ¿Qué hago con él? Por ejemplo: «\(n.texto) por 3» o «\(n.texto) al cuadrado»"
            }
            p.resh = Resh.traduceTexto(p.es)
            p.confianza = 1
            return p
        }
        let retos: [Clase] = [.cuenta, .serie, .ecuacion, .problema, .intruso, .comparacion]
        if !esPregunta(t) && !retos.contains(clasifica(t)), let p = aprende(t) { return p }
        let p = piensa(t)
        ultimo = p
        pendiente = p.elegido == nil || p.confianza < 0.4 ? t : nil
        return p
    }

    /// "no, es 12" / "no: la respuesta es gala" / "mal" → lo correcto (o "" si solo dice que está mal)
    private func correccion(_ t: String) -> String? {
        guard ultimo != nil else { return nil }
        if ["mal", "no", "incorrecto", "está mal", "eso está mal"].contains(t) { return "" }
        for p in ["no, ", "no: ", "no. ", "incorrecto, ", "mal, "] where t.hasPrefix(p) {
            return Respuestas.norma(String(t.dropFirst(p.count)))
        }
        return nil
    }

    /// Piensa una pregunta: cada forma de pensar propone, el crítico y la
    /// verificación filtran, y si la cosa está reñida piensa otra ronda.
    func piensa(_ texto: String, mision: Mision? = nil) -> Pensamiento {
        var p = Pensamiento()
        p.pregunta = texto
        p.clase = mision.map { Clase.de($0.tipo) } ?? clasifica(texto)
        let m = mision ?? misionDe(texto, p.clase)
        var veces = 1
        let usaAsociacion = m == nil && [.abierta, .definicion, .causa, .siNo].contains(p.clase)
        var asociativa = usaAsociacion ? proponeAsociativo(texto) : []
        if usaAsociacion, let b = proponeBusqueda(texto) { asociativa.append(b) }
        while true {
            p.candidatos = proponen(texto, p.clase, m, asociativa: asociativa)
            for i in 0 ..< p.candidatos.count { critica(&p.candidatos[i], m, p.clase, texto) }
            decide(&p)
            let reñida = p.candidatos.count > 1 && margen(p) < 0.12
            if veces >= 3 || (p.confianza >= 0.55 && !reñida) { break }
            // pensar más: deduce más cosas y la asociación piensa varias veces
            veces += 1
            for _ in 0 ..< 36 { logica.razona() }
        }
        p.rondas = veces
        redacta(&p)
        return p
    }

    private func misionDe(_ t: String, _ c: Clase) -> Mision? {
        guard c.tipoMision != nil || (c == .siNo && (t.lowercased().contains("todos") || t.lowercased().contains("ningún"))) else { return nil }
        var m = Misiones.propia(t)
        if c == .cuenta, let e = NyxUno.cuentaDe(t) { m = Mision(tipo: .cuenta, enunciado: "¿cuánto es \(e)?", respuesta: nil) }
        return m
    }

    // cada forma de pensar propone su respuesta

    private func proponen(_ t: String, _ c: Clase, _ m: Mision?, asociativa: [Candidato]) -> [Candidato] {
        var out: [Candidato] = []
        if let r = recuerdos[Respuestas.norma(t)] {
            out.append(Candidato(sistema: .recuerdo, respuesta: r, frase: r, confianza: 0.95, pasos: ["ya lo había resuelto"]))
        }
        if let l = proponeLogico(t, c, m) { out.append(l) }
        if let m = m, let e = proponeEstrategias(m) { out.append(e) }
        out.append(contentsOf: asociativa)
        return out
    }

    private func proponeLogico(_ t: String, _ c: Clase, _ m: Mision?) -> Candidato? {
        if let m = m {
            guard let r = Resuelve.logico(m, logica) else { return nil }
            return Candidato(sistema: .logico, respuesta: Respuestas.norma(r.0), frase: r.0, confianza: 0.9, pasos: [r.1])
        }
        if c == .abierta { return nil }
        let r = logica.procesa(t)
        if r.veredicto == "no sé" || r.veredicto.isEmpty || r.veredicto == "aprendido" { return nil }
        if r.veredicto == "no saqué ningún hecho de eso" { return nil }
        return Candidato(sistema: .logico, respuesta: Respuestas.norma(r.veredicto), frase: r.veredicto,
                         confianza: r.explicacion.isEmpty ? 0.6 : 0.92, pasos: r.explicacion)
    }

    private func proponeEstrategias(_ m: Mision) -> Candidato? {
        let r = saber.intenta(m, consejo)
        guard let x = r.respuesta else { return nil }
        let votos = r.intentos.filter { $0.respuesta.map { Respuestas.norma($0) } == x }
        var cuenta: [Estrategia: Int] = [:]
        for i in votos { cuenta[i.estrategia, default: 0] += 1 }
        let como = cuenta.sorted { $0.value > $1.value }.map { "\($0.value) por \($0.key.nombre)" }.joined(separator: ", ")
        var c = Candidato(sistema: .estrategias, respuesta: x, frase: x, confianza: Float(votos.count) / 18,
                          pasos: ["\(votos.count) de 18 mentes: \(como)"])
        c.nota = ""
        return c
    }

    /// La asociación: las 18 mentes resonantes votan (su acuerdo es la confianza).
    /// Se le pregunta una sola vez: si se le repite la pregunta, a propósito
    /// evita repetir lo que acaba de decir.
    private func proponeAsociativo(_ t: String) -> [Candidato] {
        let r = consejo.delibera(t)
        if r.frase.isEmpty || r.vocero < 0 { return [] }
        let quien = consejo.mentes[r.vocero].nombre
        let acuerdo = Int((r.acuerdo * 100).rounded())
        return [Candidato(sistema: .asociativo, respuesta: Respuestas.norma(r.frase), frase: r.frase,
                          confianza: min(0.95, 0.35 + 0.6 * r.acuerdo) * (r.recordada ? 1 : 0.7),
                          pasos: ["ganó la idea «\(r.ganador)» con \(acuerdo)% de acuerdo · habló \(quien) · " + (r.recordada ? "lo recordó" : "frase propia")])]
    }

    /// Búsqueda: la frase recordada que mejor cubre las palabras raras de la pregunta.
    private func proponeBusqueda(_ t: String) -> Candidato? {
        let pregunta = Palabras.tokens(t).filter { !Palabras.vacia($0) && !Palabras.ligeras.contains($0) && !NyxUno.interrogativas.contains($0) }
        let q = Set(pregunta.map { MemoriaFrases.raiz($0) })
        if q.isEmpty { return nil }
        let memoria = consejo.frases
        let frases = memoria.frases
        let n = Float(max(1, frases.count))
        var idf: [String: Float] = [:]
        var total: Float = 0
        for r in q {
            let v = log(1 + n / Float(max(1, memoria.cuantas(r))))
            idf[r] = v
            total += v
        }
        // ¿pregunta qué es algo? la frase que lo DEFINE («X es …») vale mucho más
        let palabra: String? = clasifica(t) == .definicion ? pregunta.last : nil
        let definida: String? = palabra.map { MemoriaFrases.raiz(ConsejoLogico.singular($0)) }
        var mejor: (Int, Float)? = nil
        for k in memoria.candidatas(q) {
            let raices = Set(frases[k].toks.map { MemoriaFrases.raiz($0) })
            var cubre: Float = 0
            for r in q where raices.contains(r) { cubre += idf[r] ?? 0 }
            if cubre == 0 { continue }
            // a igualdad: la que habla DE eso (lo nombra al principio), la más corta y la más valorada
            let alPrincipio = frases[k].toks.prefix(3).contains { q.contains(MemoriaFrases.raiz($0)) }
            var p = cubre + (alPrincipio ? 0.3 : 0) - 0.01 * Float(frases[k].toks.count) + 0.05 * frases[k].puntos
            if let d = definida, let primera = frases[k].toks.first(where: { !Palabras.vacia($0) }),
               MemoriaFrases.raiz(ConsejoLogico.singular(primera)) == d, frases[k].toks.contains("es") {
                p += total            // «casa es …»: es su definición
                if primera == palabra { p += total }      // y si es la palabra exacta («negras», no «negra»), más
            }
            if mejor == nil || p > mejor!.1 { mejor = (k, p) }
        }
        guard let m = mejor else { return nil }
        let f = frases[m.0]
        var cubre: Float = 0
        let raices = Set(f.toks.map { MemoriaFrases.raiz($0) })
        for r in q where raices.contains(r) { cubre += idf[r] ?? 0 }
        let cobertura = cubre / max(0.001, total)
        return Candidato(sistema: .busqueda, respuesta: Respuestas.norma(f.texto), frase: f.texto,
                         confianza: 0.25 + 0.65 * cobertura, pasos: ["la frase que recuerdo que más cubre la pregunta (\(Int(cobertura * 100))%)"])
    }

    // el crítico (del consejo antiguo) y la verificación

    private func critica(_ c: inout Candidato, _ m: Mision?, _ clase: Clase, _ pregunta: String) {
        // ¿habla de lo que se preguntó?
        if c.sistema == .asociativo || c.sistema == .busqueda {
            let q = Set(Palabras.tokens(pregunta).filter { !Palabras.vacia($0) && !Palabras.ligeras.contains($0) && !NyxUno.interrogativas.contains($0) }.map { MemoriaFrases.raiz($0) })
            let r = Set(Palabras.tokens(c.frase).map { MemoriaFrases.raiz($0) })
            if !q.isEmpty && q.isDisjoint(with: r) {
                c.confianza *= 0.25
                c.nota = "no habla de lo que preguntaste"
            }
        }
        // ¿contradice lo que sabe la lógica?
        if c.sistema == .asociativo, let h = logica.hechoDe(Palabras.tokens(c.frase.lowercased(), max: 30)) {
            let r = logica.prueba(h)
            if r.veredicto == "no" {
                c.confianza *= 0.15
                c.nota = "el crítico la descartó: contradice " + (r.explicacion.last ?? "lo que sé")
            } else if r.veredicto == "sí" {
                c.confianza = min(0.98, c.confianza + 0.15)
                c.nota = "la lógica la confirma"
            }
        }
        // ¿se comprueba por otro camino?
        guard let m = m, let v = Double(c.respuesta) else { return }
        var ok: Bool? = nil
        switch m.tipo {
        case .ecuacion:
            if let a = Resuelve.conX(m.lados.0, v), let b = Resuelve.conX(m.lados.1, v) { ok = abs(a - b) < 1e-6 }
        case .serie:
            if let s = Resuelve.serie(m.numeros) { ok = abs(s.0 - v) < 1e-6 }
        case .cuenta:
            if let e = NyxUno.cuentaDe(m.enunciado), let x = Aritmetica.resultado(e) { ok = Respuestas.igual(x, c.respuesta) }
        default: break
        }
        if ok == true { c.confianza = min(0.99, c.confianza + 0.1); c.nota = "verificado por otro camino" }
        if ok == false { c.confianza *= 0.3; c.nota = "no pasó la verificación" }
    }

    /// Junta las respuestas iguales y elige la que más pesa.
    private func decide(_ p: inout Pensamiento) {
        var grupos: [String: [Int]] = [:]
        for i in 0 ..< p.candidatos.count {
            let c = p.candidatos[i]
            p.candidatos[i].puntos = fiabilidad(p.clase, c.sistema) * c.confianza
            let k = clave(c)
            grupos[k, default: []].append(i)
        }
        var mejor: (String, Float)? = nil
        p.segunda = 0
        for (k, idx) in grupos {
            var noFalla: Float = 1
            for i in idx { noFalla *= 1 - p.candidatos[i].puntos }
            let total = 1 - noFalla           // si varios coinciden, se refuerzan
            if mejor == nil || total > mejor!.1 {
                if let m = mejor { p.segunda = max(p.segunda, m.1) }
                mejor = (k, total)
            } else {
                p.segunda = max(p.segunda, total)
            }
        }
        guard let g = mejor, let idx = grupos[g.0] else {
            p.elegido = nil
            p.confianza = 0
            return
        }
        let i = idx.max { p.candidatos[$0].puntos < p.candidatos[$1].puntos }!
        p.elegido = p.candidatos[i]
        p.confianza = g.1
    }

    private func clave(_ c: Candidato) -> String {
        if let g = Grande(Respuestas.norma(c.respuesta)) { return g.texto }
        if let v = Double(c.respuesta) { return Aritmetica.bonito(v) }
        return ConsejoLogico.singular(Respuestas.norma(c.respuesta))
    }

    private func margen(_ p: Pensamiento) -> Float {
        return p.confianza - p.segunda
    }

    private func redacta(_ p: inout Pensamiento) {
        guard let e = p.elegido, p.confianza >= 0.2 else {
            p.elegido = nil
            p.es = "no lo sé. ¿me lo enseñas?"
            p.resh = Resh.traduceTexto("no lo sé") + " ye?"
            return
        }
        var frase = e.frase
        if p.clase == .siNo, e.sistema == .logico, !e.pasos.isEmpty {
            let pasos = e.pasos.prefix(3).map { $0.replacingOccurrences(of: "así que ", with: "") }
            frase = e.frase + ", porque " + pasos.joined(separator: " ⇒ ")
        } else if p.clase.tipoMision != nil, e.sistema != .asociativo {
            frase = "es " + e.frase
        }
        if p.confianza < 0.4 {
            frase = "no estoy segura… quizás: " + frase
        }
        p.es = frase
        p.resh = Resh.traduceTexto(frase)
    }

    // MARK: - aprender de ti

    /// Una afirmación: la aprenden la lógica y la asociación (una sola memoria).
    private func aprende(_ t: String) -> Pensamiento? {
        var p = Pensamiento()
        p.pregunta = t
        let nuevos = logica.lee(t, quien: "tú")
        consejo.lee(t)
        if let q = pendiente {
            recuerdos[Respuestas.norma(q)] = Respuestas.norma(t)
            pendiente = nil
        }
        p.aprendio = nuevos.isEmpty ? "lo recuerdo" : "aprendí: " + nuevos.joined(separator: " · ")
        p.es = nuevos.isEmpty ? "lo guardo en la memoria" : p.aprendio
        p.resh = Resh.traduceTexto(p.es)
        p.confianza = 1
        ultimo = nil
        return p
    }

    /// Le dices que se equivocó (y quizás cuál era la respuesta).
    private func corrige(_ verdad: String) -> Pensamiento {
        var p = Pensamiento()
        guard let u = ultimo else { return p }
        p.pregunta = u.pregunta
        p.clase = u.clase
        if verdad.isEmpty {
            // solo "está mal": el que contestó falla
            if let e = u.elegido { anota(u.clase, e.sistema, bien: false) }
            if u.elegido?.sistema == .asociativo { _ = consejo.opina(bueno: false) }
            p.es = "vale, me equivoqué. ¿cuál es la respuesta? (escribe «no, es …»)"
        } else {
            for c in u.candidatos { anota(u.clase, c.sistema, bien: Respuestas.igual(c.respuesta, verdad)) }
            recuerdos[Respuestas.norma(u.pregunta)] = verdad
            // si era de sí o no, lo apunta como hecho
            if u.clase == .siNo, let h = hechoDe(u.pregunta), verdad == "sí" || verdad == "no" {
                let dicho = verdad == "sí" ? h : Hecho(a: h.a, rel: h.rel.contraria ?? h.rel, b: h.b)
                logica.lee(dicho.texto, quien: "tú")
                consejo.lee(dicho.texto)
            } else if u.clase == .abierta || u.clase == .definicion || u.clase == .causa {
                consejo.lee(verdad)
                logica.lee(verdad, quien: "tú")
            }
            p.es = "gracias: era «\(verdad)». Ya lo sé y sé de quién fiarme la próxima vez"
        }
        p.resh = Resh.traduceTexto(p.es)
        p.confianza = 1
        ultimo = nil
        return p
    }

    private func hechoDe(_ pregunta: String) -> Hecho? {
        var toks = Palabras.tokens(pregunta.lowercased(), max: 30)
        if let v = toks.first, ["es", "son", "tiene", "tienen", "puede", "pueden"].contains(v), toks.count >= 3 {
            toks = ConsejoLogico.reordena(v, Array(toks.dropFirst()))
        }
        return logica.hechoDe(toks)
    }

    /// 👍 / 👎 a la última respuesta.
    func opina(_ bueno: Bool) {
        guard let u = ultimo, let e = u.elegido else { return }
        if bueno {
            for c in u.candidatos { anota(u.clase, c.sistema, bien: clave(c) == clave(e)) }
            recuerdos[Respuestas.norma(u.pregunta)] = e.respuesta
        } else {
            for c in u.candidatos where clave(c) == clave(e) { anota(u.clase, c.sistema, bien: false) }
        }
        if e.sistema == .asociativo { _ = consejo.opina(bueno: bueno) }
        ultimo = nil
    }

    // MARK: - mejorar sola

    /// Se pone retos con respuesta conocida, los piensa con todo su cerebro y
    /// aprende de qué forma de pensar fiarse (y las 18 mentes, qué estrategia usar).
    @discardableResult
    func entrena(_ n: Int) -> (bien: Int, total: Int) {
        var bien = 0
        for k in 0 ..< n {
            let tipo = TipoMision.allCases[k % TipoMision.allCases.count]
            let m = Misiones.nueva(tipo, nivel: saber.nivel(tipo))
            guard let verdad = m.respuesta else { continue }
            let p = piensa(m.enunciado, mision: m)
            for c in p.candidatos where c.sistema != .recuerdo {
                anota(p.clase, c.sistema, bien: Respuestas.igual(c.respuesta, verdad))
            }
            // las 18 mentes aprenden su estrategia
            let r = saber.intenta(m, consejo)
            for i in r.intentos {
                let premio: Float = i.respuesta == nil ? -0.2 : (Respuestas.igual(i.respuesta!, verdad) ? 1 : 0)
                saber.aprende(i.rol, m.tipo, i.estrategia, premio)
            }
            let ok = p.elegido.map { Respuestas.igual($0.respuesta, verdad) } ?? false
            if ok { bien += 1; saber.ganadas[tipo.rawValue] += 1 }
            saber.jugadasTipo[tipo.rawValue] += 1
            if r.respuesta.map({ Respuestas.igual($0, verdad) }) ?? false { saber.aciertosNuevo[tipo.rawValue] += 1 }
        }
        entrenadas += n
        entrenadasBien += bien
        return (bien, n)
    }

    /// Consolidar (como dormir): la lógica deduce todo lo que puede de lo que
    /// sabe la asociación, y lo deducido pasa a la asociación.
    @discardableResult
    func consolida() -> (hechos: Int, pasadas: Int) {
        let antes = Set(logica.hechos.keys)
        logica.encola(consejo.frases.frases.suffix(200).map { $0.texto })
        for _ in 0 ..< 18 * 15 { logica.razona() }
        var pasadas = 0
        for (h, ap) in logica.hechos where !antes.contains(h) && !ap.hipotesis && pasadas < 40 {
            consejo.lee(h.texto)
            pasadas += 1
        }
        return (logica.hechos.count - antes.count, pasadas)
    }

    /// Algo que sabe por asociación pero no entiende con la lógica: quiere preguntártelo.
    func curiosidad() -> String? {
        var conocidas: Set<String> = []
        for h in logica.hechos.keys { conocidas.insert(h.a) }
        let m = consejo.mentes[Azar.ent(numRoles)]
        var ids = Array(0 ..< m.s.count).filter { !Palabras.vacia(m.s[$0].et) && m.s[$0].et.count > 3 && !m.s[$0].fusion }
        ids.sort { m.s[$0].A > m.s[$1].A }
        for i in ids.prefix(30) {
            let w = ConsejoLogico.singular(m.s[i].et)
            if !conocidas.contains(w) && !Palabras.ligeras.contains(w) { return "¿qué es \(w)?" }
        }
        return nil
    }

    // MARK: - guardar

    func exporta() -> String {
        var l: [String] = ["entrenadas \(entrenadas) \(entrenadasBien)"]
        for c in Clase.allCases {
            let a = aciertos[c.rawValue].map { fmt($0) }.joined(separator: " ")
            let f = fallos[c.rawValue].map { fmt($0) }.joined(separator: " ")
            l.append("f \(c.rawValue) \(a) | \(f)")
        }
        for (q, r) in recuerdos.prefix(500) { l.append("r \(q)\t\(r)") }
        return l.joined(separator: "\n")
    }

    func importa(_ texto: String) {
        for linea in texto.components(separatedBy: "\n") {
            if linea.hasPrefix("r ") {
                let p = linea.dropFirst(2).components(separatedBy: "\t")
                if p.count == 2 { recuerdos[p[0]] = p[1] }
                continue
            }
            let p = linea.split(separator: " ").map { String($0) }
            if p.first == "entrenadas", p.count == 3 {
                entrenadas = Int(p[1]) ?? 0
                entrenadasBien = Int(p[2]) ?? 0
            }
            guard p.first == "f", p.count > 2, let c = Int(p[1]), c < aciertos.count, let bar = p.firstIndex(of: "|") else { continue }
            let a = p[2 ..< bar].compactMap { Float($0) }
            let f = p[(bar + 1)...].compactMap { Float($0) }
            if a.count == Sistema.allCases.count { aciertos[c] = a }
            if f.count == Sistema.allCases.count { fallos[c] = f }
        }
    }
}
