import Foundation

// Asamblea.swift — los tres consejos hablan entre sí.
//
//  ✨ el consejo nuevo (18 mentes resonantes, SwiftData)
//  🏛 el consejo antiguo (el original que hablaba Resh)
//  ⚖️ el consejo lógico (18 agentes que razonan con hechos)
//
// Hablan por turnos, siempre en Resh, y cada uno contesta a lo que oyó:
//  · el nuevo entiende solo el Resh que sabe y contesta con la mente que más entendió;
//  · el antiguo oye a los dos y habla con su mente de más peso;
//  · el lógico saca hechos de lo que oye: dice si es verdad o mentira (con
//    la prueba), apunta lo nuevo, cuenta lo que dedujo o pregunta "¿qué es X?".
//    Lo que deduce se lo enseña al consejo nuevo; sus preguntas las contesta el nuevo.

struct LineaAsamblea: Identifiable {
    let id: Int
    let consejo: Int        // -1 tú · 0 nuevo · 1 antiguo · 2 lógico · 3 misión
    let quien: String
    let resh: String
    let es: String
    var deMision = false    // lo dijo enfrentando una misión (no es charla)

    var icono: String {
        switch consejo {
        case 0: return "✨ nuevo"
        case 1: return "🏛 antiguo"
        case 2: return "⚖️ lógico"
        case 3: return "🎯 misión"
        default: return "🙂 tú"
        }
    }
}

final class Asamblea {
    private(set) var lineas: [LineaAsamblea] = []
    private var contador = 0
    private var turno = 0
    /// Lo que el lógico preguntó y aún nadie contestó ("qué es mar").
    private var preguntaLogica: String? = nil
    /// Lo que el antiguo no ha oído todavía.
    private var porOirAntiguo: [String] = []
    /// Palabras por las que el lógico ya preguntó (para no repetir).
    private var yaPreguntadas: Set<String> = []
    /// Lo que el lógico dedujo: el nuevo lo aprende después de contestar.
    private var leccionNuevo: String? = nil
    // hablar a su ritmo
    /// Cuánto acaban de hablar (sube al hablar, baja con el tiempo).
    private var cansancio: [Float] = [0, 0, 0]
    /// Turnos seguidos sin nada nuevo que decir.
    private var aburrimiento = 0
    /// Misiones que se propusieron ellos solos desde la última vez que les escribiste.
    private var misionesSolos = 0
    private var lineaUltimaMision = -100
    // misiones
    var saber = SaberMisiones()
    private(set) var mision: Mision? = nil
    private var pasoMision = 0
    private var respuestasMision: [Int: String] = [:]
    private var intentosNuevo: [IntentoMente] = []
    /// Nadie supo la respuesta de tu misión: esperan a que se la digas.
    private(set) var esperandoRespuesta = false
    var estado = ""

    private func pon(_ consejo: Int, _ quien: String, _ resh: String, _ es: String) -> LineaAsamblea {
        contador += 1
        var l = LineaAsamblea(id: contador, consejo: consejo, quien: quien, resh: resh, es: es)
        l.deMision = consejo == 3 || (mision != nil && consejo >= 0)
        lineas.append(l)
        if lineas.count > 300 { lineas.removeFirst(lineas.count - 300) }
        if consejo != 1 && !l.deMision { porOirAntiguo.append(resh) }
        return l
    }

    /// Lo olvidan todo (la conversación, las misiones y lo aprendido de ellas).
    func olvida() {
        lineas = []
        saber = SaberMisiones()
        mision = nil
        esperandoRespuesta = false
        porOirAntiguo = []
        preguntaLogica = nil
        aburrimiento = 0
        estado = ""
    }

    /// Tú escribes en el chat (en español o en Resh). "misión: …" les pone un reto.
    func escribe(_ texto: String, logico: ConsejoLogico) {
        let t = texto.trimmingCharacters(in: .whitespacesAndNewlines)
        if t.isEmpty { return }
        aburrimiento = 0
        misionesSolos = 0
        if esperandoRespuesta, var m = mision {
            _ = pon(-1, "tú", Resh.traduceTexto(t), t)
            m.respuesta = Respuestas.norma(t)
            mision = m
            esperandoRespuesta = false
            pasoMision = 4
            return
        }
        let bajo = t.lowercased()
        for p in ["misión", "mision", "reto", "acertijo", "puzzle"] where bajo.hasPrefix(p) {
            var resto = String(t.dropFirst(p.count)).trimmingCharacters(in: .whitespaces)
            if resto.hasPrefix(":") { resto = String(resto.dropFirst()).trimmingCharacters(in: .whitespaces) }
            if resto.isEmpty { empiezaMision(Misiones.nueva(tipoAlAzar(), nivel: 1), de: "tú"); return }
            _ = pon(-1, "tú", Resh.traduceTexto(resto), resto)
            empiezaMision(Misiones.propia(resto), de: "tú")
            return
        }
        let es = Resh.traduceHumano(t) + (t.contains("?") ? "?" : "")
        _ = pon(-1, "tú", Resh.traduceTexto(es) + (t.contains("?") ? "?" : ""), es)
        preguntaLogica = nil
        if !es.contains("?") { _ = logico.lee(es, quien: "tú") }
    }

    /// Les pones una misión (nil: de un tipo al azar).
    func proponMision(_ tipo: TipoMision?) {
        aburrimiento = 0
        let t = tipo ?? tipoAlAzar()
        empiezaMision(Misiones.nueva(t, nivel: saber.nivel(t)), de: "tú")
    }

    private func tipoAlAzar() -> TipoMision {
        return TipoMision.allCases[Azar.ent(TipoMision.allCases.count)]
    }

    private func empiezaMision(_ m: Mision, de quien: String) {
        mision = m
        pasoMision = 1
        respuestasMision = [:]
        intentosNuevo = []
        esperandoRespuesta = false
        lineaUltimaMision = contador
        let nivel = m.respuesta == nil ? "tuya" : "nivel \(m.nivel)"
        _ = pon(3, m.tipo.icono + " " + m.tipo.nombre + " · " + nivel, Resh.traduceTexto(m.enunciado), m.enunciado)
    }

    /// Lo siguiente que pasa en la asamblea. Hablan cuando tienen ganas, al
    /// ritmo que quieren; devuelven cuánto esperar y si quieren seguir hablando.
    @MainActor
    func siguiente(nuevo: Consejo, puente: Puente, logico: ConsejoLogico) async -> (pausa: Double, sigue: Bool) {
        if mision != nil { return await pasoDeMision(nuevo, puente, logico) }
        for i in 0 ..< 3 { cansancio[i] *= 0.55 }
        let ganas = [ganasNuevo(), ganasAntiguo(), ganasLogico(logico)]
        var quien = 0
        for i in 1 ..< 3 where ganas[i] > ganas[quien] { quien = i }
        var dichas: [LineaAsamblea] = []
        if ganas[quien] >= 0.45 {
            let antes = Set(lineas.suffix(10).map { Respuestas.norma($0.es) })
            switch quien {
            case 0: dichas = hablaNuevo(nuevo, logico)
            case 1: dichas = await hablaAntiguo(puente, nuevo, logico)
            default: dichas = hablaLogico(logico, nuevo)
            }
            cansancio[quien] += 0.6
            let novedad = dichas.contains { !antes.contains(Respuestas.norma($0.es)) && contenido($0.resh) > 0 }
            aburrimiento = novedad ? 0 : aburrimiento + 1
        } else {
            aburrimiento += 1
        }
        estado = dichas.isEmpty ? "pensando…" : "hablando"
        if aburrimiento >= 3 {
            aburrimiento = 0
            if misionesSolos < 2 && contador - lineaUltimaMision > 12 {
                // aburridos: el lógico propone un reto a los demás
                misionesSolos += 1
                let t = tipoMasDificil()
                _ = pon(2, "explicador", Resh.traduceTexto("propongo una misión"), "propongo una misión: \(t.nombre)")
                empiezaMision(Misiones.nueva(t, nivel: saber.nivel(t)), de: "lógico")
                return (1.5, true)
            }
            _ = pon(3, "silencio", "…", "(se callan: ya no tienen nada nuevo que decir)")
            estado = "se callaron"
            return (0, false)
        }
        guard let ultima = dichas.last else { return (0.9, true) }
        return (ritmo(quien, ultima, ganas[quien]), true)
    }

    /// Cuánto tardan en volver a hablar: el antiguo es lento, las frases
    /// largas toman su tiempo y con muchas ganas se habla más rápido.
    private func ritmo(_ quien: Int, _ l: LineaAsamblea, _ ganas: Float) -> Double {
        let base: [Double] = [0.9, 1.6, 1.1]
        let palabras = Double(Palabras.tokens(l.es).count)
        let r = base[quien] * (0.6 + palabras / 12) * (1.3 - 0.5 * Double(min(1, ganas)))
        return min(4, max(0.4, r))
    }

    private func ganasNuevo() -> Float {
        var g: Float = 0.45 + Azar.rango(0, 0.3) - cansancio[0]
        guard let u = lineas.last else { return g + 0.5 }
        if u.consejo != 0 { g += 0.3 }
        if u.consejo == -1 { g += 0.5 }
        if preguntaLogica != nil { g += 0.6 }
        return g
    }

    private func ganasAntiguo() -> Float {
        var g: Float = 0.35 + Azar.rango(0, 0.3) - cansancio[1]
        g += min(0.4, 0.1 * Float(porOirAntiguo.count))
        if lineas.last?.consejo == -1 { g += 0.3 }
        return g
    }

    private func ganasLogico(_ logico: ConsejoLogico) -> Float {
        var g: Float = 0.3 + Azar.rango(0, 0.3) - cansancio[2]
        for l in lineas.suffix(2) where l.consejo != 2 && !l.deMision {
            if l.es.contains("?") || logico.hechoDe(Palabras.tokens(l.es.lowercased(), max: 30)) != nil { g += 0.5; break }
        }
        if lineas.last?.consejo == -1 { g += 0.3 }
        return g
    }

    /// El tipo de misión en que el consejo nuevo falla más.
    private func tipoMasDificil() -> TipoMision {
        var peor = tipoAlAzar()
        var tasa: Float = 2
        for t in TipoMision.allCases.shuffled() {
            let j = saber.jugadasTipo[t.rawValue]
            let r = j == 0 ? 0.5 : Float(saber.aciertosNuevo[t.rawValue]) / Float(j)
            if r < tasa { tasa = r; peor = t }
        }
        return peor
    }

    // MARK: 🎯 misiones

    @MainActor
    private func pasoDeMision(_ nuevo: Consejo, _ puente: Puente, _ logico: ConsejoLogico) async -> (pausa: Double, sigue: Bool) {
        guard let m = mision else { return (0.5, true) }
        if esperandoRespuesta {
            estado = "esperan tu respuesta"
            return (1, false)
        }
        let paso = pasoMision
        pasoMision += 1
        switch paso {
        case 1: misionNuevo(m, nuevo); return (1.4, true)
        case 2: await misionAntiguo(m, puente); return (1.4, true)
        case 3: misionLogico(m, logico); return (1.6, true)
        default: return await juzga(m, nuevo, puente, logico)
        }
    }

    private func misionNuevo(_ m: Mision, _ nuevo: Consejo) {
        estado = "el consejo nuevo piensa la misión"
        let r = saber.intenta(m, nuevo)
        intentosNuevo = r.intentos
        var cuenta: [Estrategia: Int] = [:]
        for i in r.intentos { cuenta[i.estrategia, default: 0] += 1 }
        let como = cuenta.sorted { $0.value > $1.value }.map { "\($0.value) \($0.key.nombre)" }.joined(separator: " · ")
        guard let x = r.respuesta, r.vocero >= 0 else {
            _ = pon(0, "consejo", Resh.traduceTexto("no sabemos"), "no sabemos   (\(como))")
            return
        }
        respuestasMision[0] = x
        let votos = r.intentos.filter { $0.respuesta.map { Respuestas.norma($0) } == x }.count
        _ = pon(0, nuevo.mentes[r.vocero].nombre, Resh.traduceTexto("creo que es") + " " + x,
                "creo que es \(x)   (\(votos) de 18 · \(como))")
    }

    @MainActor
    private func misionAntiguo(_ m: Mision, _ puente: Puente) async {
        estado = "el consejo antiguo delibera"
        if !puente.listo { await puente.prepara(memoria: nil) }
        let v = await puente.viejo.deliberar(sobre: m.enunciado)
        let x = Respuestas.norma(v.veredicto)
        let vacia = x.count < 2 || Palabras.vacia(x) || Resh.formasParticula.contains(x)
        if x.isEmpty || x == "sin consenso" || vacia {
            _ = pon(1, "consejo", "mi ko ne", "no lo sé")
            return
        }
        respuestasMision[1] = x
        _ = pon(1, "consejo", v.veredicto, "creo que es \(x)")
    }

    private func misionLogico(_ m: Mision, _ logico: ConsejoLogico) {
        estado = "el consejo lógico razona"
        guard let r = Resuelve.logico(m, logico) else {
            _ = pon(2, "verificador", Resh.traduceTexto("no sé"), "no sé resolverlo")
            return
        }
        respuestasMision[2] = Respuestas.norma(r.0)
        let agente = [13, 13, 13, 13, 1, 14, 15][m.tipo.rawValue]
        _ = pon(2, agentesLogicos[agente], Resh.traduceTexto("es") + " " + r.0, "es \(r.0)   (\(r.1))")
    }

    @MainActor
    private func juzga(_ m: Mision, _ nuevo: Consejo, _ puente: Puente, _ logico: ConsejoLogico) async -> (pausa: Double, sigue: Bool) {
        var porLogico = false
        var verdad = m.respuesta
        if verdad == nil, let l = respuestasMision[2] {
            verdad = l
            porLogico = true
        }
        guard let v = verdad else {
            esperandoRespuesta = true
            pasoMision = 4
            _ = pon(3, "misión", "ye?", "nadie lo sabe seguro: escríbeme la respuesta y lo aprenden")
            estado = "esperan tu respuesta"
            return (1, false)
        }
        let nombres = ["✨ nuevo", "🏛 antiguo", "⚖️ lógico"]
        var partes: [String] = []
        var alguno = false
        for c in 0 ..< 3 {
            let bien = respuestasMision[c].map { Respuestas.igual($0, v) } ?? false
            if bien { saber.puntos[c] += 1; alguno = true }
            partes.append((bien ? "✅ " : "❌ ") + nombres[c])
        }
        let t = m.tipo.rawValue
        saber.jugadas += 1
        saber.jugadasTipo[t] += 1
        if alguno { saber.ganadas[t] += 1 }
        if respuestasMision[0].map({ Respuestas.igual($0, v) }) ?? false { saber.aciertosNuevo[t] += 1 }
        // cada mente nueva aprende si su estrategia funcionó
        var aprendieron = 0
        for i in intentosNuevo {
            let r: Float = i.respuesta == nil ? -0.2 : (Respuestas.igual(i.respuesta!, v) ? 1 : 0)
            saber.aprende(i.rol, m.tipo, i.estrategia, r)
            if r > 0 { aprendieron += 1 }
        }
        saber.recuerdos[Respuestas.norma(m.enunciado)] = Respuestas.norma(v)
        if m.tipo == .intruso && !m.categoria.isEmpty {
            let leccion = "\(v) no es \(m.categoria)"
            nuevo.lee(leccion)
            logico.lee(leccion, quien: "misión")
        }
        await puente.viejo.percibir("\(m.enunciado) \(v)")
        let fuente = porLogico ? " (según el lógico)" : ""
        _ = pon(3, "resultado", Resh.traduceTexto("es") + " " + v,
                "la respuesta es \(v)\(fuente) · " + partes.joined(separator: " ") + " · \(aprendieron) mentes nuevas acertaron con su estrategia")
        mision = nil
        intentosNuevo = []
        respuestasMision = [:]
        estado = ""
        return (2, true)
    }

    // MARK: ✨ el consejo nuevo

    private func hablaNuevo(_ nuevo: Consejo, _ logico: ConsejoLogico) -> [LineaAsamblea] {
        defer {
            if let x = leccionNuevo { nuevo.lee(x) }
            leccionNuevo = nil
        }
        // el lógico preguntó algo: contesta el consejo entero
        if let q = preguntaLogica {
            preguntaLogica = nil
            return contestaNuevo(nuevo, logico, q)
        }
        guard let ultima = lineas.last(where: { !$0.deMision }) else {
            // nadie ha dicho nada: el nuevo saca un tema de lo que sabe
            let tema = Roles.infancia[Azar.ent(Roles.infancia.count)]
            return contestaNuevo(nuevo, logico, tema)
        }
        if ultima.consejo == -1 { return contestaNuevo(nuevo, logico, ultima.es) }
        let autor = ultima.consejo == 2 ? "lógico" : "antiguo"
        if let r = nuevo.respondeAExterno(ultima.resh, autor: autor), !repite(r.es, ultima.es) {
            let l = pon(0, nuevo.mentes[r.rol].nombre, r.resh, r.es)
            _ = logico.lee(r.es, quien: "consejo nuevo")
            return [l]
        }
        return contestaNuevo(nuevo, logico, ultima.es)
    }

    /// ¿Dice lo mismo que lo que oyó?
    private func repite(_ a: String, _ b: String) -> Bool {
        let x = Set(Palabras.tokens(a).filter { !Palabras.vacia($0) })
        let y = Set(Palabras.tokens(b).filter { !Palabras.vacia($0) })
        return !x.isEmpty && x.isSubset(of: y)
    }

    private func contestaNuevo(_ nuevo: Consejo, _ logico: ConsejoLogico, _ pregunta: String) -> [LineaAsamblea] {
        let r = nuevo.delibera(pregunta)
        if r.frase.isEmpty || r.vocero < 0 { return [] }
        if let u = lineas.last, repite(r.frase, u.es) {
            // lo que oyó ya lo sabía: habla de otra cosa que tenga que ver
            let otra = nuevo.delibera("y qué más sabes de " + pregunta)
            if otra.vocero >= 0 && !otra.frase.isEmpty && !repite(otra.frase, u.es) {
                _ = logico.lee(otra.frase, quien: "consejo nuevo")
                return [pon(0, nuevo.mentes[otra.vocero].nombre, otra.resh, otra.frase)]
            }
        }
        _ = logico.lee(r.frase, quien: "consejo nuevo")
        return [pon(0, nuevo.mentes[r.vocero].nombre, r.resh, r.frase)]
    }

    // MARK: 🏛 el consejo antiguo

    @MainActor
    private func hablaAntiguo(_ puente: Puente, _ nuevo: Consejo, _ logico: ConsejoLogico) async -> [LineaAsamblea] {
        if !puente.listo { await puente.prepara(memoria: nil) }
        let oido = porOirAntiguo
        porOirAntiguo = []
        for t in oido.suffix(3) { await puente.oyeElViejo(t) }
        let emitidos = await puente.viejo.rondaDeDialogo()
        // de lo que dijeron sus 18, lo que más dice (y a igualdad, lo de más peso)
        var texto = ""
        var quien = ""
        var mejor = (n: 0, peso: -1.0)
        for e in emitidos {
            let n = contenido(e.texto)
            if n > mejor.n || (n == mejor.n && n > 0 && e.peso > mejor.peso) {
                mejor = (n, e.peso)
                texto = e.texto
                quien = e.rol.rawValue
            }
        }
        if texto.isEmpty, let duda = palabraQueNoSabeElViejo() {
            // no tiene nada que decir: pregunta por una palabra Resh que oyó
            texto = "ye \(duda)?"
            quien = "pregunta"
        }
        if texto.isEmpty { return [] }
        let es = quien == "pregunta" ? "¿qué significa \(texto.dropFirst(3).dropLast())?" : Resh.traduceHumano(texto)
        var out = [pon(1, quien, texto, es)]
        // si pregunta qué significa una palabra, una mente nueva se la enseña
        if let leccion = puente.ensenaAlViejo(texto, nuevo) {
            await puente.viejo.enseñarATodos(español: leccion.es, significa: [leccion.resh])
            out.append(pon(0, leccion.quien, "\(leccion.resh) ka \(leccion.es)", "\(leccion.resh) significa \(leccion.es)"))
        } else {
            _ = logico.lee(es, quien: "consejo antiguo")
        }
        return out
    }

    /// Una palabra Resh de lo último que oyó (para preguntar qué significa).
    private func palabraQueNoSabeElViejo() -> String? {
        for l in lineas.suffix(3).reversed() where l.consejo != 1 {
            let formas = Palabras.tokens(l.resh).filter { Resh.esForma($0) && !Resh.formasParticula.contains($0) }
            if !formas.isEmpty { return formas[Azar.ent(formas.count)] }
        }
        return nil
    }

    /// Cuántas palabras con contenido dice (sin partículas Resh).
    private func contenido(_ resh: String) -> Int {
        let es = Palabras.tokens(Resh.traduceHumano(resh))
        return es.filter { !Palabras.vacia($0) && !Resh.formasParticula.contains($0) }.count
    }

    // MARK: ⚖️ el consejo lógico

    private func hablaLogico(_ logico: ConsejoLogico, _ nuevo: Consejo) -> [LineaAsamblea] {
        let oido = Array(lineas.suffix(3).filter { $0.consejo != 2 && !$0.deMision })
        // 1. ¿alguien afirmó algo que se puede comprobar?
        for l in oido.reversed() {
            if let d = juzga(l, logico) { return [diceLogico((d.es, d.decir), agente: d.agente)] }
        }
        // 2. ¿dedujo algo nuevo? se lo cuenta a todos (y el nuevo lo aprende)
        if let d = deduceAlgo(logico) {
            leccionNuevo = d.texto
            return [diceLogico((d.texto + "   (" + d.porque + ")", d.texto), agente: d.agente)]
        }
        // 3. pregunta por algo que oyó y no sabe qué es
        if let x = algoQueNoSabe(oido, logico) {
            yaPreguntadas.insert(x)
            preguntaLogica = "qué es " + x
            return [diceLogico(("¿qué es \(x)?", "qué es \(x)"), agente: 9)]
        }
        return []
    }

    /// (lo que se muestra en español, lo que se dice en Resh)
    private func diceLogico(_ dicho: (es: String, decir: String), agente: Int) -> LineaAsamblea {
        let resh = Resh.traduceTexto(dicho.decir) + (dicho.es.hasSuffix("?") ? "?" : "")
        return pon(2, agentesLogicos[agente], resh, dicho.es)
    }

    /// Comprueba una afirmación: "no: …" si es falsa, "sí: …" si la demuestra,
    /// "apunto: …" si es nueva. nil si no hay nada que decir.
    private func juzga(_ l: LineaAsamblea, _ logico: ConsejoLogico) -> (es: String, decir: String, agente: Int)? {
        if let v = cuentaDentro(l.es) {
            return ("\(v.0) = \(v.1)", v.1, 13)
        }
        if l.es.contains("?") {
            let r = logico.procesa(l.es)
            if r.veredicto == "no sé" || r.veredicto.isEmpty { return nil }
            let porque = r.explicacion.prefix(3).joined(separator: " ⇒ ")
            return (r.veredicto + (porque.isEmpty ? "" : "   (" + porque + ")"), r.veredicto, 17)
        }
        guard let h = logico.hechoDe(Palabras.tokens(l.es.lowercased(), max: 30)) else { return nil }
        let r = logico.prueba(h)
        let porque = r.explicacion.prefix(3).joined(separator: " ⇒ ")
        if r.veredicto == "no" {
            return ("no: \(h.texto) es falso   (\(porque))", "no \(h.texto)", 10)
        }
        if r.veredicto == "contradicción" {
            return ("contradicción con \(h.texto)", "no \(h.texto)", 7)
        }
        if r.veredicto == "sí" && r.explicacion.count > 1 {
            return ("sí: \(h.texto)   (\(porque))", "sí \(h.texto)", 12)
        }
        let nuevos = logico.lee(l.es, quien: l.quien)
        if nuevos.isEmpty { return nil }
        return ("apunto: " + nuevos.joined(separator: " · "), nuevos.joined(separator: " y "), 0)
    }

    /// Una cuenta dentro de una frase: "hola, ¿cuánto es 12 por 12?" → (12 * 12, 144).
    private func cuentaDentro(_ t: String) -> (String, String)? {
        let bajo = t.lowercased()
        for clave in ["cuánto es", "cuanto es", "calcula", "cuánto da", "cuanto da"] {
            guard let r = bajo.range(of: clave) else { continue }
            let resto = String(bajo[r.upperBound...])
            if Aritmetica.opera(resto), let v = Aritmetica.resultado(resto) { return (Aritmetica.expresion(resto), v) }
        }
        return nil
    }

    /// Diez pasos de razonamiento; devuelve un hecho que antes no sabía.
    private func deduceAlgo(_ logico: ConsejoLogico) -> (texto: String, porque: String, agente: Int)? {
        let antes = Set(logico.hechos.keys)
        for _ in 0 ..< 18 { logico.razona() }
        var oidas: Set<String> = []
        for l in lineas.suffix(6) {
            for t in Palabras.tokens(l.es.lowercased(), max: 30) { oidas.insert(ConsejoLogico.singular(t)) }
        }
        var elegido: (texto: String, porque: String, agente: Int)? = nil
        for (h, ap) in logico.hechos where !antes.contains(h) && !ap.hipotesis {
            let agente = agentesLogicos.firstIndex(of: ap.quien) ?? 1
            let d = (h.texto, ap.porque.joined(separator: " + "), agente)
            if oidas.contains(h.a) || oidas.contains(h.b) { return d }
            if elegido == nil { elegido = d }
        }
        return elegido
    }

    /// Una palabra con contenido que oyó y de la que no sabe nada.
    private func algoQueNoSabe(_ oido: [LineaAsamblea], _ logico: ConsejoLogico) -> String? {
        var conocidas: Set<String> = []
        for h in logico.hechos.keys { conocidas.insert(h.a) }
        for l in oido.reversed() where !l.es.contains("significa") && !l.deMision {
            for t in Palabras.tokens(l.es.lowercased(), max: 30) where t.count > 3 && !Palabras.vacia(t) {
                let x = ConsejoLogico.singular(t)
                if conocidas.contains(x) || yaPreguntadas.contains(x) || Resh.esForma(t) { continue }
                if Palabras.ligeras.contains(t) { continue }
                return x
            }
        }
        return nil
    }
}
