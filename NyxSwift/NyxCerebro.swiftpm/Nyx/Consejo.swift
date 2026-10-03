import Foundation

// Consejo.swift — las 18 juntas: imaginación con aprendizaje, deliberación,
// conversaciones y tu opinión. (Versión Swift de consejo.c de NyxC.)

enum TipoEvento {
    case piensa, dice, aprende, idea, resh, suena, nota, envia, recibe

    var icono: String {
        switch self {
        case .piensa: return "💭"
        case .dice: return "▶"
        case .aprende: return "📚"
        case .idea: return "✨"
        case .resh: return "📖"
        case .suena: return "🌙"
        case .nota: return "·"
        case .envia: return "📤"
        case .recibe: return "📥"
        }
    }
}

struct Evento: Identifiable {
    let id: Int
    let rol: Int
    let tipo: TipoEvento
    let texto: String
}

struct Voto {
    let rol: Int
    let atractor: String
    let C: Float
    let E: Float
    let peso: Float
    let acuerdo: Bool
}

struct Respuesta {
    var frase = ""
    var resh = ""
    var ganador = ""
    var vocero = -1
    var acuerdo: Float = 0
    var votos: [Voto] = []
    var recordada = false
}

final class Consejo {
    var mentes: [Mente] = []
    let frases = MemoriaFrases()
    var tick = 0
    var ultimo = -1
    /// Por donde se pasan conocimiento (en la app: SwiftData).
    var buzon: Buzon = BuzonMemoria()
    private var ecos: [String] = []
    private var contadorEventos = 0
    /// Lo que pasa dentro (para la pantalla "en vivo").
    var alEvento: ((Evento) -> Void)?

    // para 'bien' y 'mal'
    private var fbValido = false
    private var fbMentes: [Int] = []
    private var fbGan: [Int: Int] = [:]
    private var fbEst: [Int: [Int]] = [:]
    private var fbFrase: [Int] = []
    private var fbVocero = -1
    private var fbFr: Int? = nil

    init(infancia: Bool) {
        for r in 0 ..< numRoles { mentes.append(Mente(rol: r)) }
        if infancia {
            for t in Roles.infancia { lee(t) }
        }
    }

    func evento(_ rol: Int, _ tipo: TipoEvento, _ texto: String) {
        contadorEventos += 1
        alEvento?(Evento(id: contadorEventos, rol: rol, tipo: tipo, texto: texto))
    }

    private func esEco(_ t: String) -> Bool { ecos.contains(t) }

    private func ponEco(_ t: String) {
        ecos.append(t)
        if ecos.count > 10 { ecos.removeFirst() }
    }

    /// Todas leen un texto (enseñar). Devuelve cuántas palabras nuevas hubo.
    @discardableResult
    func lee(_ texto: String) -> Int {
        let antes = mentes[0].s.count
        for m in mentes { m.ingesta(texto, dialogo: false) }
        frases.pon(texto)
        return max(0, mentes[0].s.count - antes)
    }

    // MARK: pasarse conocimiento

    /// Una mente comparte algo que aprendió.
    func comparte(_ de: Int, _ tipo: TipoEnvio, _ contenido: String, para: Int = -1) {
        buzon.envia(Envio(de: de, para: para, tipo: tipo, contenido: contenido))
        let destino = para < 0 ? "todas" : mentes[para].nombre
        evento(de, .envia, "\(tipo.icono) envió a \(destino): \(contenido)")
    }

    /// Recoge su correo y lo absorbe.
    private func recogeCorreo(_ rol: Int) {
        let m = mentes[rol]
        for e in buzon.recoge(para: rol, maximo: 4) {
            if let que = m.absorbe(e) {
                evento(rol, .recibe, "\(e.tipo.icono) de \(mentes[e.de].nombre): \(que)")
            }
        }
    }

    private func anunciaCristal(_ rol: Int, _ cr: Int?) {
        guard let cr = cr else { return }
        let et = mentes[rol].s[cr].et
        evento(rol, .idea, "cristalizó un concepto: «\(et)»")
        comparte(rol, .concepto, et)
    }

    // MARK: componer y difundir

    /// Una frase recordada (si encaja) o una propia. No crea semiones: los
    /// índices de quien llama siguen valiendo.
    private func compone(_ m: Mente, centro: Int, ctx: [Int], probRecuerdo: Float)
        -> (ids: [Int], es: String, resh: String, fr: Int?) {
        if Azar.f01() < probRecuerdo,
           let (k, p) = frases.elige(mente: m, centro: centro, ctx: ctx, esEco: esEco), p > -0.5 {
            var ids: [Int] = []
            for t in frases.frases[k].toks {
                if let id = m.busca(t) { ids.append(id) }
            }
            frases.frases[k].usos += 1
            return (ids, frases.frases[k].texto, frases.enResh(k, mente: m), k)
        }
        let ids = m.frase(centro, ctx: ctx)
        let t = m.texto(ids)
        return (ids, t.es, t.resh, nil)
    }

    private func probRecuerdo(_ rol: Int) -> Float {
        switch rol {
        case 6: return 0.8            // memoria
        case 13: return 0.6           // narrativa
        case 4, 9: return 0.15        // creativo, intuicion: prefieren inventar
        default: return 0.35
        }
    }

    /// Las demás oyen lo que dijo el emisor; pueden aprender sus palabras Resh.
    @discardableResult
    private func difunde(_ emisor: Int, _ ids: [Int], texto es: String) -> Float {
        let e = mentes[emisor]
        var suma: Float = 0
        var aprendieron = 0
        var palabra = ""
        let sabeEmisor: Set<String> = Set(ids.filter { e.s[$0].sabe }.map { e.s[$0].et })
        for o in mentes where o.rol != emisor {
            let oidos = o.recibe(es, de: e.nombre)
            var res: Float = 0
            for id in oidos {
                res += o.coherencia(id)
                let et = o.s[id].et
                if !o.s[id].sabe && sabeEmisor.contains(et) && Resh.deEspanol(et) != nil && Azar.f01() < 0.35 {
                    o.s[id].sabe = true
                    o.reshAprendidas += 1
                    if !Palabras.vacia(et) { aprendieron += 1; palabra = et }
                }
            }
            if !oidos.isEmpty { suma += res / Float(oidos.count) }
        }
        if aprendieron > 0, let r = Resh.deEspanol(palabra) {
            let n = aprendieron == 1 ? "1 mente aprendió" : "\(aprendieron) mentes aprendieron"
            evento(emisor, .resh, "\(n) a decir «\(palabra)» = \(r)")
        }
        return suma / Float(max(1, numRoles - 1))
    }

    /// Habla una mente: forma una frase desde 'centro', la dice, las demás la oyen.
    private func habla(_ rol: Int, centro: Int, prefijo: String) -> Float {
        let m = mentes[rol]
        let c = compone(m, centro: centro, ctx: m.foco, probRecuerdo: probRecuerdo(rol))
        if m.yaDijo(c.es) || esEco(c.es) { return -0.6 }
        ponEco(c.es)
        evento(rol, .dice, "\(prefijo)\(c.resh)   «\(c.es)»")
        m.anota("dije: " + c.es)
        m.dichos += 1
        let res = difunde(rol, c.ids, texto: c.es)
        m.cansa(c.ids)
        return 1.5 * tanh(res)
    }

    // MARK: vida libre (imaginación)

    private func puntaje(_ m: Mente, _ a: Int) -> Float {
        var p: Float = m.info.gusto[a] + m.valor[a] + 0.3 / (1 + Float(m.veces[a]))
        if a == m.ultima { p -= 0.15 * Float(m.racha) }      // aburrimiento
        return p + Azar.rango(-m.ruido * 2, m.ruido * 2)
    }

    private func aprende(_ rol: Int, _ a: Int, _ recompensa: Float) {
        let m = mentes[rol]
        let antes = m.valor[a]
        m.valor[a] += 0.25 * (recompensa - m.valor[a])
        m.veces[a] += 1
        let delta = m.valor[a] - antes
        evento(rol, .aprende, "'\(Accion(rawValue: a)?.nombre ?? "")' ahora vale \(fmt(m.valor[a])) (\(delta >= 0 ? "+" : "")\(fmt(delta)))")
    }

    /// Un paso de vida libre: una mente imagina qué quiere hacer, lo hace,
    /// mide cómo le fue y aprende.
    func paso() {
        tick += 1
        var rol = Azar.ent(numRoles)
        if rol == ultimo { rol = (rol + 1 + Azar.ent(numRoles - 1)) % numRoles }
        ultimo = rol
        let m = mentes[rol]
        recogeCorreo(rol)
        for _ in 0 ..< 3 { m.piensa() }
        let tema = m.tema()
        let duda = m.duda()
        var posibles: [Int] = Array(0 ..< numAcciones)
        if tema == nil { posibles.removeAll { $0 == Accion.hablar.rawValue } }
        if m.epis.count < 2 { posibles.removeAll { $0 == Accion.recordar.rawValue } }
        var orden: [(Int, Float)] = posibles.map { ($0, puntaje(m, $0)) }
        orden.sort { $0.1 > $1.1 }
        var a = orden[0].0
        let total = m.veces.reduce(0, +)
        let explora = max(0.08, 0.35 - 0.01 * Float(total))
        if Azar.f01() < explora { a = posibles[Azar.ent(posibles.count)] }
        if a == m.ultima { m.racha += 1 } else { m.ultima = a; m.racha = 1 }
        anunciaDeseo(rol, a, tema: tema, duda: duda, orden: orden)
        let rec: Float
        switch Accion(rawValue: a) ?? .escuchar {
        case .hablar: rec = actHablar(rol, tema)
        case .preguntar: rec = actPreguntar(rol, duda: duda, tema: tema)
        case .imaginar: rec = actImaginar(rol)
        case .sonar: rec = actSonar(rol)
        case .recordar: rec = actRecordar(rol)
        case .escuchar: rec = actEscuchar(rol)
        }
        aprende(rol, a, rec)
    }

    private func anunciaDeseo(_ rol: Int, _ a: Int, tema: Int?, duda: Int?, orden: [(Int, Float)]) {
        let m = mentes[rol]
        let tt = tema.map { m.s[$0].et } ?? "algo"
        let otras = orden.filter { $0.0 != a }.prefix(2).map { "\(Accion(rawValue: $0.0)?.nombre ?? "") \(fmt($0.1))" }
        let alt = otras.isEmpty ? "" : " · también pensó: " + otras.joined(separator: ", ")
        let deseo: String
        switch Accion(rawValue: a) ?? .escuchar {
        case .hablar: deseo = "quiero hablar de «\(tt)»"
        case .preguntar: deseo = duda.map { "quiero preguntar cómo se dice «\(m.s[$0].et)»" } ?? "quiero preguntar algo"
        case .imaginar: deseo = "quiero imaginar algo nuevo"
        case .sonar: deseo = "quiero soñar y ordenar lo que viví"
        case .recordar: deseo = "quiero contar lo que recuerdo"
        case .escuchar: deseo = "quiero escuchar"
        }
        evento(rol, .piensa, deseo + alt)
    }

    private func actHablar(_ rol: Int, _ tema: Int?) -> Float {
        guard let t = tema else { return -0.2 }
        let m = mentes[rol]
        var pre = "mi ko "
        if m.coherencia(t) < 0.2 { pre = "ye " } else if (rol == 5 || rol == 12) && Azar.ent(10) < 5 { pre = "ne " }
        return habla(rol, centro: t, prefijo: pre)
    }

    private func actPreguntar(_ rol: Int, duda: Int?, tema: Int?) -> Float {
        let m = mentes[rol]
        if let d = duda {
            let et = m.s[d].et
            evento(rol, .dice, "ye \(et)?   «¿cómo se dice \(et)?»")
            var quien: Int? = nil
            var mejorP: Float = -1
            for o in mentes where o.rol != rol {
                if let id = o.busca(et), o.s[id].sabe {
                    let p = o.precision + Azar.rango(0, 0.3)
                    if p > mejorP { mejorP = p; quien = o.rol }
                }
            }
            guard let q = quien, let r = Resh.deEspanol(et) else {
                evento(rol, .nota, "nadie supo contestarle")
                return -0.3
            }
            evento(q, .dice, "\(et) ka \(r)   «\(et) se dice \(r)»")
            m.s[d].sabe = true
            m.reshAprendidas += 1
            evento(rol, .resh, "aprendió «\(et)» = \(r)")
            comparte(rol, .palabra, "\(et)=\(r)")      // y se lo pasa a las demás
            return 1.5
        }
        guard let t = tema else { return -0.2 }
        var r = Azar.ent(numRoles)
        if r == rol { r = (r + 1) % numRoles }
        let et = m.s[t].et
        evento(rol, .dice, "ye \(et)?   «¿qué piensas de \(et), \(mentes[r].nombre)?»")
        let ids = mentes[r].recibe(et, de: m.nombre)
        guard !ids.isEmpty, let v = mentes[r].veredicto(ids) else { return 0.1 }
        let res = habla(r, centro: v.ganador, prefijo: "")
        return res > 0 ? 1.0 : 0.1
    }

    private func actImaginar(_ rol: Int) -> Float {
        let m = mentes[rol]
        guard let (id, a, b) = m.imagina() else { return -0.3 }
        for _ in 0 ..< 10 {
            m.decae(id, ruido: m.ruido)
            m.propaga(id)
            m.propaga(a)
            m.propaga(b)
        }
        let C = m.coherencia(id)
        evento(rol, .idea, "imaginó «\(m.s[id].et)» (de \(m.s[a].et) y \(m.s[b].et)) · coherencia \(fmt(C))")
        m.atiende(id)
        return limita(C * 2, -1, 1.5)
    }

    private func actSonar(_ rol: Int) -> Float {
        let m = mentes[rol]
        var antes: Float = 0
        for f in m.foco { antes += m.coherencia(f) }
        for _ in 0 ..< 12 { m.piensa() }
        var ids: [Int] = []
        var intentos = 0
        while ids.count < 8 && intentos < m.s.count && !m.s.isEmpty {
            intentos += 1
            let x = Azar.ent(m.s.count)
            if m.s[x].dialogo > 0 && !m.s[x].fusion && !Palabras.vacia(m.s[x].et) { ids.append(x) }
        }
        var cristal: Int? = nil
        for id in ids {
            m.s[id].A = min(1, m.s[id].A + 0.08)
            if cristal == nil { cristal = m.transitiva(id) }
        }
        var despues: Float = 0
        for f in m.foco { despues += m.coherencia(f) }
        if let cr = cristal {
            evento(rol, .idea, "soñando cristalizó un concepto: «\(m.s[cr].et)»")
            comparte(rol, .concepto, m.s[cr].et)
        } else {
            evento(rol, .suena, "soñó con \(ids.count) recuerdos · coherencia \(despues - antes >= 0 ? "+" : "")\(fmt(despues - antes))")
        }
        return limita((despues - antes) * 0.5 + (cristal != nil ? 1 : 0.1), -1, 1.5)
    }

    private func actRecordar(_ rol: Int) -> Float {
        let m = mentes[rol]
        var ids: [Int] = []
        for e in m.epis.reversed() where !ids.contains(e) && !m.s[e].fusion && ids.count < 3 { ids.append(e) }
        if ids.count < 2 { return -0.2 }
        ids.reverse()
        let t = m.texto(ids)
        evento(rol, .dice, "mi sor ra: \(t.resh)   «recuerdo: \(t.es)»")
        let res = difunde(rol, ids, texto: t.es)
        return 1.2 * tanh(res)
    }

    private func actEscuchar(_ rol: Int) -> Float {
        let m = mentes[rol]
        let r = Azar.ent(numRoles)
        let o = mentes[r]
        if r != rol, let ult = o.ventana.last {
            let contenido: String
            if let rango = ult.range(of: ": ") { contenido = String(ult[rango.upperBound...]) } else { contenido = ult }
            m.recibe(contenido, de: o.nombre)
            evento(rol, .nota, "escuchó a \(o.nombre)")
        }
        return 0.2
    }

    // MARK: preguntas del humano

    /// Pregunta a las 18: deliberan, votan y responde la que más convence.
    func delibera(_ pregunta: String) -> Respuesta {
        var out = Respuesta()
        fbValido = true
        fbMentes = []
        fbGan = [:]
        fbEst = [:]
        fbFr = nil
        var gan: [Int: Mente.Veredicto] = [:]
        var peso: [Int: Float] = [:]
        for m in mentes {
            let ids = m.recibe(pregunta, de: "humano")
            fbEst[m.rol] = ids
            guard !ids.isEmpty, let v = m.veredicto(ids) else { continue }
            gan[m.rol] = v
            fbGan[m.rol] = v.ganador
            peso[m.rol] = (0.3 + m.precision) * (0.3 + max(0, v.C)) * (0.5 + v.E)
            anunciaCristal(m.rol, v.cristal)
        }
        var suma: [String: Float] = [:]
        for (r, v) in gan { suma[mentes[r].s[v.ganador].et, default: 0] += peso[r] ?? 0 }
        guard let ganadora = suma.max(by: { $0.value < $1.value })?.key else {
            out.frase = "(silencio: no entendimos nada)"
            fbValido = false
            return out
        }
        out.ganador = ganadora
        var mejor: Float = -1
        var acuerdo = 0
        for m in mentes {
            let v = gan[m.rol]
            let at = v.map { m.s[$0.ganador].et } ?? "-"
            let ok = at == ganadora
            out.votos.append(Voto(rol: m.rol, atractor: at, C: v?.C ?? 0, E: v?.E ?? 0, peso: peso[m.rol] ?? 0, acuerdo: ok))
            if v != nil {
                m.intentos += 1
                if ok { m.aciertos += 1 }
                if m.intentos > 200 { m.intentos = 100; m.aciertos /= 2 }
            }
            if ok {
                acuerdo += 1
                fbMentes.append(m.rol)
                if (peso[m.rol] ?? 0) > mejor { mejor = peso[m.rol] ?? 0; out.vocero = m.rol }
            }
        }
        out.acuerdo = Float(acuerdo) / Float(numRoles)
        out.votos.sort { $0.peso > $1.peso }
        guard out.vocero >= 0, let gv = gan[out.vocero] else { return out }
        let v = mentes[out.vocero]
        let c = compone(v, centro: gv.ganador, ctx: fbEst[out.vocero] ?? [], probRecuerdo: 0.9)
        fbFrase = c.ids
        fbFr = c.fr
        fbVocero = out.vocero
        out.recordada = c.fr != nil
        ponEco(c.es)
        if out.acuerdo < 0.25 {
            out.frase = "¿\(c.es)?"
            out.resh = "ye \(c.resh)?"
        } else {
            out.frase = c.es
            out.resh = "se ko \(c.resh)"
        }
        difunde(out.vocero, c.ids, texto: c.es)
        return out
    }

    /// Hablar con una sola mente.
    func hablaCon(_ rol: Int, _ texto: String) -> (es: String, resh: String, C: Float, recordada: Bool) {
        let m = mentes[rol]
        fbValido = false
        fbFr = nil
        let ids = m.recibe(texto, de: "humano")
        if ids.isEmpty { return ("(no entendí)", "", 0, false) }
        var ultimo: (ids: [Int], es: String, resh: String, fr: Int?)? = nil
        var vUlt: Mente.Veredicto? = nil
        for _ in 0 ..< 4 {           // si iba a repetir lo que se acaba de decir, piensa otra cosa
            guard let v = m.veredicto(ids) else { break }
            vUlt = v
            let c = compone(m, centro: v.ganador, ctx: ids, probRecuerdo: max(0.8, probRecuerdo(rol)))
            ultimo = c
            if !esEco(c.es) { break }
        }
        guard let c = ultimo, let v = vUlt else { return ("(no entendí)", "", 0, false) }
        ponEco(c.es)
        fbValido = true
        fbMentes = [rol]
        fbGan = [rol: v.ganador]
        fbEst = [rol: ids]
        fbFrase = c.ids
        fbFr = c.fr
        fbVocero = rol
        m.cansa(c.ids)
        anunciaCristal(rol, v.cristal)
        return (c.es, c.resh, v.C, c.fr != nil)
    }

    /// Dos mentes conversan: cada una responde a lo último que dijo la otra.
    func conversa(_ a: Int, _ b: Int, turnos: Int) {
        var ultimoDicho: String = mentes[a].tema().map { mentes[a].s[$0].et } ?? mentes[a].info.objetivo
        evento(a, .nota, "\(mentes[a].nombre) y \(mentes[b].nombre) conversan sobre «\(ultimoDicho)»")
        for i in 0 ..< turnos {
            let quien = i % 2 == 0 ? a : b
            tick += 1
            let r = hablaCon(quien, ultimoDicho)
            let pre = i == 0 ? "" : (r.C < 0.2 ? "ye " : "se ko ")
            evento(quien, .dice, "\(pre)\(r.resh)   «\(r.es)»")
            if !fbFrase.isEmpty { difunde(quien, fbFrase, texto: r.es) }
            ultimoDicho = r.es
        }
        fbValido = false
    }

    /// Tu opinión sobre la última respuesta reestructura lo aprendido.
    @discardableResult
    func opina(bueno: Bool) -> Bool {
        if !fbValido { return false }
        for r in fbMentes {
            guard let g = fbGan[r] else { continue }
            let m = mentes[r]
            for e in fbEst[r] ?? [] where e != g && !Palabras.vacia(m.s[e].et) {
                if bueno {
                    m.refuerza(e, g, dw: 0.3, th: 0, dsec: 0)
                    m.refuerza(g, e, dw: 0.2, th: 0, dsec: 0)
                } else {
                    m.fija(e, g, w: 0.7, th: NYX_PI)
                    m.fija(g, e, w: 0.7, th: NYX_PI)
                }
            }
            m.intentos += 1
            if bueno { m.aciertos += 1 }
            m.valor[Accion.hablar.rawValue] += bueno ? 0.05 : -0.05
        }
        frases.opina(fbFr, bueno: bueno)
        if bueno, fbVocero >= 0 {
            let v = mentes[fbVocero]
            let dicha = v.texto(fbFrase).es
            if !dicha.isEmpty { comparte(fbVocero, .frase, dicha) }   // lo que te gustó se comparte
            for x in 0 ..< max(0, fbFrase.count - 1) {
                v.refuerza(fbFrase[x], fbFrase[x + 1], dw: 0.1, th: 0.3, dsec: 0.2)
            }
        }
        fbValido = false
        return true
    }
}

func fmt(_ x: Float) -> String {
    return String(format: "%.2f", Double(x))
}
