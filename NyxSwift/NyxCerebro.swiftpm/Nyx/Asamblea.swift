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
    let consejo: Int        // -1 tú · 0 nuevo · 1 antiguo · 2 lógico
    let quien: String
    let resh: String
    let es: String

    var icono: String {
        switch consejo {
        case 0: return "✨ nuevo"
        case 1: return "🏛 antiguo"
        case 2: return "⚖️ lógico"
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

    private func pon(_ consejo: Int, _ quien: String, _ resh: String, _ es: String) -> LineaAsamblea {
        contador += 1
        let l = LineaAsamblea(id: contador, consejo: consejo, quien: quien, resh: resh, es: es)
        lineas.append(l)
        if lineas.count > 300 { lineas.removeFirst(lineas.count - 300) }
        if consejo != 1 { porOirAntiguo.append(resh) }
        return l
    }

    /// Tú propones un tema: los tres lo oyen y empieza el consejo nuevo.
    func proponTema(_ texto: String, logico: ConsejoLogico) -> [LineaAsamblea] {
        let es = Resh.traduceHumano(texto)
        let resh = Resh.traduceTexto(es)
        turno = 0
        preguntaLogica = nil
        _ = logico.lee(es, quien: "tú")
        return [pon(-1, "tú", resh, es)]
    }

    /// Un turno: habla el consejo al que le toca (nuevo → antiguo → lógico).
    @MainActor
    func turno(nuevo: Consejo, puente: Puente, logico: ConsejoLogico) async -> [LineaAsamblea] {
        let toca = turno % 3
        turno += 1
        switch toca {
        case 0: return hablaNuevo(nuevo, logico)
        case 1: return await hablaAntiguo(puente, nuevo, logico)
        default: return hablaLogico(logico, nuevo)
        }
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
        guard let ultima = lineas.last else {
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
        let oido = Array(lineas.suffix(3).filter { $0.consejo != 2 })
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
        for l in oido.reversed() {
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
