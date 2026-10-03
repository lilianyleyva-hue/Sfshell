import Foundation

// Mente.swift — una mente: semiones, resonancia de fase, veredicto y frases.
// (Versión Swift de mente.c de NyxC, ya probada en C.)
//
// Cada palabra o idea es un "semión" con amplitud A y fase. Pensar es dejar
// que las fases se sincronicen (Kuramoto) hasta que gana un atractor.
// Cada acople guarda la asociación (w), el desfase preferido (th: 0 acuerdo,
// π oposición) y la secuencia (sec: cuánto "esta va seguida de aquella").

let maxSemiones = 900
let maxAcoples = 16
let NYX_PI: Float = 3.14159265
let NYX_2PI: Float = 6.28318531

struct Acople {
    var j: Int
    var w: Float
    var th: Float
    var sec: Float
}

struct Semion {
    var et: String
    var f: [Float]
    var A: Float
    var fase: Float
    var frec: Float
    var carga: Int
    var fusion: Bool
    var sabe: Bool
    var usos: Int
    var dialogo: Int
    var v: [Acople]
}

struct Camino {
    var a: Int
    var b: Int
    var c: Int
    var k: Int
}

final class Mente {
    let rol: Int
    var s: [Semion] = []
    var indice: [String: Int] = [:]
    let ruido: Float
    let rigidez: Float
    let A0: Float
    var obj: [Int] = []
    var foco: [Int] = []
    var epis: [Int] = []
    var caminos: [Camino] = []
    var recientes: [Int] = []
    var reshOido: [String] = []      // palabras Resh que oyó y no entendió
    var aciertos = 0
    var intentos = 0
    var valor: [Float] = [0, 0, 0, 0, 0, 0]
    var veces: [Int] = [0, 0, 0, 0, 0, 0]
    var ultima = -1
    var racha = 0
    var ventana: [String] = []
    var ciclos = 0
    var dichos = 0
    var ideas = 0
    var cristales = 0
    var reshAprendidas = 0

    var info: RolInfo { Roles.todos[rol] }
    var nombre: String { Roles.todos[rol].nombre }

    init(rol: Int) {
        self.rol = rol
        let r = Roles.todos[rol]
        ruido = r.ruido
        rigidez = r.rigidez
        A0 = r.A0
        let ids = ingesta(r.objetivo, dialogo: false)
        for id in ids.prefix(8) {
            obj.append(id)
            s[id].A = 0.95
            if Resh.deEspanol(s[id].et) != nil { s[id].sabe = true }
        }
    }

    var precision: Float {
        return intentos > 0 ? Float(aciertos) / Float(intentos) : 0.5
    }

    func anota(_ texto: String) {
        ventana.append(texto)
        if ventana.count > 16 { ventana.removeFirst(ventana.count - 16) }
    }

    func busca(_ et: String) -> Int? { indice[et] }

    // MARK: semiones y acoples

    func nuevo(_ et: String, fusion: Bool) -> Int? {
        if s.count >= maxSemiones { return nil }
        var sabe = false
        if !fusion, Resh.deEspanol(et) != nil {
            sabe = Resh.esParticula(et) || Azar.f01() < info.sabeResh
        }
        let sem = Semion(et: et, f: Palabras.firma(et), A: 0.6, fase: Azar.rango(0, NYX_2PI),
                         frec: Azar.rango(0.8, 1.2), carga: fusion ? 1 : 0, fusion: fusion, sabe: sabe,
                         usos: 0, dialogo: 0, v: [])
        s.append(sem)
        indice[et] = s.count - 1
        return s.count - 1
    }

    func posAcople(_ a: Int, _ b: Int) -> Int? {
        let v = s[a].v
        for k in 0 ..< v.count where v[k].j == b { return k }
        return nil
    }

    /// Crea el acople a->b (si no cabe, reemplaza el más débil). Devuelve su posición.
    func acopleNuevo(_ a: Int, _ b: Int, w: Float, th: Float) -> Int? {
        if a == b { return nil }
        if s[a].v.count < maxAcoples {
            s[a].v.append(Acople(j: b, w: w, th: th, sec: 0))
            return s[a].v.count - 1
        }
        var peor = 0
        for k in 1 ..< s[a].v.count where s[a].v[k].w + s[a].v[k].sec < s[a].v[peor].w + s[a].v[peor].sec {
            peor = k
        }
        if s[a].v[peor].w + s[a].v[peor].sec > w { return nil }
        s[a].v[peor] = Acople(j: b, w: w, th: th, sec: 0)
        return peor
    }

    func fija(_ a: Int, _ b: Int, w: Float, th: Float) {
        if let k = posAcople(a, b) ?? acopleNuevo(a, b, w: w, th: th) {
            s[a].v[k].w = w
            s[a].v[k].th = th
        }
    }

    /// Refuerzo con saturación: repetir acerca a 1 sin pasarse.
    func refuerza(_ a: Int, _ b: Int, dw: Float, th: Float, dsec: Float) {
        if a == b { return }
        if let k = posAcople(a, b) {
            s[a].v[k].w += dw * (1 - s[a].v[k].w)
            s[a].v[k].sec += dsec * (1 - s[a].v[k].sec)
        } else if let k = acopleNuevo(a, b, w: dw, th: th) {
            s[a].v[k].sec = dsec
        }
    }

    func coherencia(_ i: Int) -> Float {
        let si = s[i]
        var c: Float = 0
        for ac in si.v {
            let o = s[ac.j]
            c += ac.w * si.A * o.A * cos(si.fase - o.fase - ac.th)
        }
        return c
    }

    private func envuelve(_ f: Float) -> Float {
        var x = f.truncatingRemainder(dividingBy: NYX_2PI)
        if x < 0 { x += NYX_2PI }
        return x
    }

    func decae(_ i: Int, ruido r: Float) {
        let A = s[i].A
        let dA: Float = -rigidez * (A * A - A0 * A0) * A * 0.02
        s[i].A = limita(A + dA, 0.05, 1)
        s[i].fase = envuelve(s[i].fase + s[i].frec * 0.02 + Azar.rango(-r, r))
    }

    func propaga(_ i: Int) {
        let si = s[i]
        var d: Float = 0
        for ac in si.v {
            let o = s[ac.j]
            if o.carga != si.carga && ac.w <= 0.9 { continue }
            d += ac.w * o.A * sin(o.fase - si.fase - ac.th)
        }
        s[i].fase = envuelve(si.fase + 0.05 * d)
    }

    /// Lo que el estímulo despierta: él, sus vecinos y los vecinos de esos.
    func region(_ est: [Int], tope: Int) -> [Int] {
        var marca = Set<Int>()
        var out: [Int] = []
        for e in est where !marca.contains(e) && out.count < tope {
            marca.insert(e)
            out.append(e)
        }
        let primeros = out
        var segundos: [Int] = []
        for x in primeros {
            for ac in s[x].v where !marca.contains(ac.j) && out.count < tope {
                marca.insert(ac.j)
                out.append(ac.j)
                segundos.append(ac.j)
            }
        }
        for x in segundos {
            for ac in s[x].v.prefix(12) where !marca.contains(ac.j) && out.count < tope {
                marca.insert(ac.j)
                out.append(ac.j)
            }
        }
        return out
    }

    func atiende(_ id: Int) {
        foco.removeAll { $0 == id }
        foco.append(id)
        if foco.count > 7 { foco.removeFirst(foco.count - 7) }
        s[id].A = min(1, s[id].A + 0.12)
    }

    func episodio(_ id: Int) {
        epis.append(id)
        if epis.count > 24 { epis.removeFirst(epis.count - 24) }
    }

    func esObjetivo(_ id: Int) -> Bool { obj.contains(id) }

    // MARK: poda

    /// Libera ~12% cuando la mente se llena. Primero caen las ideas propias
    /// que nadie retomó; luego lo menos coherente y usado.
    func poda() {
        let quitar = s.count / 8
        var punt: [(Int, Float)] = []
        punt.reserveCapacity(s.count)
        for i in 0 ..< s.count {
            punt.append((i, puntajePoda(i)))
        }
        punt.sort { $0.1 < $1.1 }
        var fuera = Set<Int>()
        for p in punt.prefix(quitar) { fuera.insert(p.0) }
        var nuevo = [Int](repeating: -1, count: s.count)
        var compacto: [Semion] = []
        compacto.reserveCapacity(s.count - fuera.count)
        for i in 0 ..< s.count where !fuera.contains(i) {
            nuevo[i] = compacto.count
            compacto.append(s[i])
        }
        for a in 0 ..< compacto.count {
            var vs: [Acople] = []
            for ac in compacto[a].v where nuevo[ac.j] >= 0 {
                var x = ac
                x.j = nuevo[ac.j]
                vs.append(x)
            }
            compacto[a].v = vs
        }
        s = compacto
        obj = obj.compactMap { nuevo[$0] >= 0 ? nuevo[$0] : nil }
        foco = foco.compactMap { nuevo[$0] >= 0 ? nuevo[$0] : nil }
        epis = epis.compactMap { nuevo[$0] >= 0 ? nuevo[$0] : nil }
        recientes = recientes.compactMap { nuevo[$0] >= 0 ? nuevo[$0] : nil }
        var cams: [Camino] = []
        for c in caminos where nuevo[c.a] >= 0 && nuevo[c.b] >= 0 && nuevo[c.c] >= 0 {
            cams.append(Camino(a: nuevo[c.a], b: nuevo[c.b], c: nuevo[c.c], k: c.k))
        }
        caminos = cams
        rehaceIndice()
    }

    private func puntajePoda(_ i: Int) -> Float {
        let x = s[i]
        var p: Float = x.A + 0.08 * Float(x.usos) + 0.3 * Float(x.dialogo) + 0.2 * coherencia(i)
        p += 0.03 * Float(x.v.count) + (x.sabe ? 0.4 : 0)
        if x.fusion && x.dialogo == 0 { p -= 5 }
        if esObjetivo(i) { p += 1000 }
        return p
    }

    func rehaceIndice() {
        indice = [:]
        for (i, x) in s.enumerated() { indice[x.et] = i }
    }

    // MARK: ingesta

    /// Texto -> semiones, con asociación (ventana de 3) y secuencia.
    /// gramatica: cuánto se aprende el ORDEN (las frases de otras mentes
    /// enseñan poca gramática, para que el idioma no se degrade).
    @discardableResult
    func ingesta(_ texto: String, dialogo: Bool, gramatica: Float = 1) -> [Int] {
        let toks = Palabras.tokens(texto)
        if s.count > maxSemiones - 32 { poda() }
        var ids: [Int] = []
        for t in toks {
            guard let id = busca(t) ?? nuevo(t, fusion: false) else { continue }
            s[id].A = min(1, s[id].A + 0.15)
            s[id].usos += 1
            if dialogo { s[id].dialogo += 1 }
            ids.append(id)
        }
        for i in 0 ..< ids.count {
            for d in 1 ... 3 where i + d < ids.count && ids[i] != ids[i + d] {
                let fd = Float(d)
                refuerza(ids[i], ids[i + d], dw: 0.35 / fd, th: 0.3, dsec: d == 1 ? 0.4 * gramatica : 0)
                refuerza(ids[i + d], ids[i], dw: 0.2 / fd, th: -0.3, dsec: 0)
            }
        }
        return ids
    }

    /// Lo que espera oír según su foco.
    func predice(_ n: Int) -> [Int] {
        var peso: [Int: Float] = [:]
        for f in foco {
            for ac in s[f].v {
                peso[ac.j, default: 0] += ac.w * s[ac.j].A
            }
        }
        let orden = peso.sorted { $0.value > $1.value }
        return orden.prefix(n).map { $0.key }
    }

    /// Recibir lo que otro dijo. Lo sorprendente se aprende más.
    @discardableResult
    func recibe(_ texto: String, de quien: String) -> [Int] {
        let pred = predice(3)
        let gram: Float = quien == "humano" ? 1 : 0.15
        let ids = ingesta(texto, dialogo: true, gramatica: gram)
        var aciertosPred = 0
        for id in ids where pred.contains(id) { aciertosPred += 1 }
        let sorpresa: Float = pred.isEmpty ? 0.5 : 1 - Float(aciertosPred) / Float(max(1, ids.count))
        for id in ids {
            s[id].A = min(1, s[id].A + 0.25 * sorpresa + 0.05)
            if s[id].A > 0.55 && !Palabras.vacia(s[id].et) { atiende(id) }
        }
        anota("◀ \(quien): \(texto)")
        return ids
    }

    // MARK: razonar

    /// El siguiente vecino más coherente no visitado.
    func salto(_ id: Int, _ vis: [Int]) -> Int? {
        var mejor: Int? = nil
        var mc: Float = -1e9
        for ac in s[id].v {
            let j = ac.j
            if s[j].fusion || Palabras.vacia(s[j].et) || vis.contains(j) { continue }
            let c = coherencia(j)
            if c > mc { mc = c; mejor = j }
        }
        return mejor
    }

    /// Un camino A->B->C recorrido 3 veces se vuelve concepto.
    func cristaliza(_ a: Int, _ b: Int, _ c: Int) -> Int? {
        if s.count >= maxSemiones - 1 { return nil }
        let et = String(s[a].et.prefix(8)) + "⊕" + String(s[b].et.prefix(8)) + "⊕" + String(s[c].et.prefix(8))
        if busca(et) != nil { return nil }
        guard let id = nuevo(et, fusion: true) else { return nil }
        s[id].A = 0.9
        s[id].dialogo = 2
        var f: [Float] = []
        for k in 0 ..< 8 { f.append((s[a].f[k] + s[b].f[k] + s[c].f[k]) / 3) }
        s[id].f = f
        for x in [a, b, c] {
            fija(x, id, w: 0.8, th: 0)
            fija(id, x, w: 0.8, th: 0)
        }
        cristales += 1
        return id
    }

    /// Inferencia transitiva: refuerza A->B->C; el atajo perdura.
    func transitiva(_ id: Int) -> Int? {
        guard let s1 = salto(id, [id]) else { return nil }
        let w1 = posAcople(id, s1).map { s[id].v[$0].w } ?? 0.3
        fija(id, s1, w: min(1, w1 + 0.1), th: 0.2)
        guard let s2 = salto(s1, [id, s1]) else { return nil }
        let w2 = posAcople(s1, s2).map { s[s1].v[$0].w } ?? 0.3
        fija(s1, s2, w: min(1, w2 + 0.1), th: 0.2)
        for k in 0 ..< caminos.count where caminos[k].a == id && caminos[k].b == s1 && caminos[k].c == s2 {
            caminos[k].k += 1
            if caminos[k].k == 3 { return cristaliza(id, s1, s2) }
            return nil
        }
        caminos.append(Camino(a: id, b: s1, c: s2, k: 1))
        if caminos.count > 48 { caminos.removeFirst() }
        return nil
    }

    func incrustacion(_ id: Int) -> Float {
        let v = s[id].v
        if v.isEmpty { return 0 }
        var n = 0
        for ac in v where s[ac.j].dialogo > 0 { n += 1 }
        return Float(n) / Float(v.count)
    }

    struct Veredicto {
        let ganador: Int
        let C: Float
        let E: Float
        let cristal: Int?
    }

    /// Peso de una palabra como pista: las raras dicen más que las comunes
    /// ("lluvia" pesa más que "hace").
    func pesoPista(_ id: Int) -> Float {
        let base: Float = 1 / (1 + 0.35 * log(1 + Float(s[id].usos)))
        return Palabras.ligeras.contains(s[id].et) ? base * 0.3 : base
    }

    /// Activación que se propaga desde el estímulo, hasta 3 saltos. Cada
    /// palabra empieza con su peso de pista; un acople inhibido (θ≈π) resta.
    /// Así la pregunta evoca también lo que está a 2 o 3 pasos.
    func activa(_ est: [Int: Float], saltos: Int = 3) -> [Int: Float] {
        var act: [Int: Float] = est
        var frente: [Int: Float] = est
        for _ in 0 ..< saltos {
            var sig: [Int: Float] = [:]
            for (i, a) in frente {
                let v = s[i].v
                if v.isEmpty { continue }
                let reparto: Float = 0.6 / Float(v.count).squareRoot()
                for ac in v {
                    sig[ac.j, default: 0] += a * reparto * ac.w * cos(ac.th)
                }
            }
            let mejores = sig.sorted { abs($0.value) > abs($1.value) }.prefix(150)
            frente = [:]
            for (k, v) in mejores {
                frente[k] = v
                act[k, default: 0] += v
            }
        }
        return act
    }

    /// Activación desde una lista de ids (cada uno con su peso de pista).
    func activaDesde(_ ids: [Int], previo: [Int] = []) -> [Int: Float] {
        var est: [Int: Float] = [:]
        for id in previo where !Palabras.vacia(s[id].et) { est[id, default: 0] += 0.4 * pesoPista(id) }
        for id in ids where !Palabras.vacia(s[id].et) { est[id, default: 0] += pesoPista(id) }
        return activa(est)
    }

    /// El estímulo despierta una región, se relaja con recocido y gana lo
    /// que la pregunta EVOCA (no su eco). Solo cuentan sus palabras con contenido.
    /// previo: lo que se venía hablando (da contexto a la pregunta).
    func veredicto(_ est0: [Int], previo: [Int] = []) -> Veredicto? {
        var est = est0.filter { !Palabras.vacia(s[$0].et) }
        if est.isEmpty { est = est0 }
        if est.isEmpty { return nil }
        let act = activaDesde(est, previo: previo)
        var reg = region(est, tope: 200)
        let marcados = Set(reg)
        for (k, v) in act.sorted(by: { $0.value > $1.value }).prefix(60) where v > 0 && !marcados.contains(k) {
            reg.append(k)
        }
        for it in 0 ..< 30 {
            let r: Float = ruido * (3 - 2 * Float(it) / 30)
            for i in reg { decae(i, ruido: r) }
            for i in reg { propaga(i) }
        }
        var gan: Int? = nil
        var mejor: Float = -1e9
        for id in reg {
            if est.contains(id) && reg.count > est.count { continue }
            let rel = relevancia(id, est, act[id] ?? 0)
            if rel > mejor { mejor = rel; gan = id }
        }
        if gan == nil {
            gan = est.first { !Palabras.vacia(s[$0].et) } ?? est[0]
        }
        guard let g = gan else { return nil }
        let cr = transitiva(g)
        episodio(g)
        atiende(g)
        recientes.append(g)
        if recientes.count > 6 { recientes.removeFirst() }
        return Veredicto(ganador: g, C: coherencia(g), E: incrustacion(g), cristal: cr)
    }

    /// Manda lo que la pregunta evoca (enlace); la coherencia desempata.
    /// Un acople inhibido (θ≈π) resta: así 'mal' cambia la respuesta.
    private func relevancia(_ id: Int, _ est: [Int], _ activacion: Float) -> Float {
        let x = s[id]
        if x.fusion || Palabras.vacia(x.et) { return -1e9 }
        var enlace: Float = 0
        for e in est {
            let p = pesoPista(e)
            if let k = posAcople(e, id) { enlace += p * s[e].v[k].w * cos(s[e].v[k].th) }
            if let k = posAcople(id, e) { enlace += p * x.v[k].w * cos(x.v[k].th) }
        }
        var rel: Float = enlace + 1.5 * activacion + 0.35 * tanh(coherencia(id)) + (esObjetivo(id) ? 0.1 : 0)
        rel /= 1 + log(1 + Float(x.usos)) * 0.3
        if recientes.contains(id) { rel -= abs(rel) + 0.2 }   // fatiga
        return rel
    }

    // MARK: frases propias

    /// Lo que va antes y después del centro según la memoria de secuencia.
    /// ctx: lo que se preguntó; la frase prefiere ir por ahí.
    func frase(_ centro: Int, ctx: [Int], max maxN: Int = 24) -> [Int] {
        var pre: [Int] = []
        var actual = centro
        while pre.count < 2 {
            guard let p = antecesor(actual, centro: centro, ya: pre, ctx: ctx) else { break }
            pre.append(p)
            actual = p
        }
        while let ult = pre.last, Palabras.conector(s[ult].et) { pre.removeLast() }
        var out: [Int] = pre.reversed()
        out.append(centro)
        let objetivo = 4 + Azar.ent(5)
        actual = centro
        while out.count < maxN && out.count < objetivo {
            guard let sig = sucesor(actual, ya: out, ctx: ctx) else { break }
            out.append(sig)
            actual = sig
        }
        while out.count > 2, let ult = out.last, ult != centro, Palabras.vacia(s[ult].et) { out.removeLast() }
        if out.count < 2, let fuerte = s[centro].v.filter({ !s[$0.j].fusion }).max(by: { $0.w < $1.w }) {
            out.append(fuerte.j)
        }
        return out
    }

    private func antecesor(_ actual: Int, centro: Int, ya: [Int], ctx: [Int]) -> Int? {
        var mejor: Int? = nil
        var mp: Float = 0.04
        for i in 0 ..< s.count {
            if i == actual || i == centro || s[i].fusion || ya.contains(i) { continue }
            guard let k = posAcople(i, actual), s[i].v[k].sec > 0 else { continue }
            var p: Float = s[i].v[k].sec * (0.5 + s[i].A) + Azar.rango(0, ruido * 2)
            if ctx.contains(i) { p += 0.6 }
            if p > mp { mp = p; mejor = i }
        }
        return mejor
    }

    private func sucesor(_ actual: Int, ya: [Int], ctx: [Int]) -> Int? {
        var mejor: Int? = nil
        var mp: Float = 0.02
        for ac in s[actual].v {
            if ac.sec <= 0 || s[ac.j].fusion || ya.contains(ac.j) { continue }
            var p: Float = ac.sec * (0.5 + s[ac.j].A) + Azar.rango(0, ruido * 3)
            if ctx.contains(ac.j) { p += 0.6 }
            if p > mp { mp = p; mejor = ac.j }
        }
        return mejor
    }

    /// Texto en español y en Resh (con lo que esta mente sabe decir).
    func texto(_ ids: [Int]) -> (es: String, resh: String) {
        var es: [String] = []
        var rs: [String] = []
        for id in ids {
            let et = s[id].et
            es.append(et)
            if s[id].sabe, let r = Resh.deEspanol(et) { rs.append(r) } else { rs.append(et) }
        }
        return (es.joined(separator: " "), rs.joined(separator: " "))
    }

    /// Después de decir algo, esas ideas se cansan: el tema puede cambiar.
    func cansa(_ ids: [Int]) {
        for id in ids { s[id].A = max(0.1, s[id].A * 0.7) }
    }

    // MARK: imaginar y pensar

    /// Idea propia: fusiona dos ideas activas parecidas.
    func imagina() -> (id: Int, a: Int, b: Int)? {
        if s.count > maxSemiones - 4 { poda() }
        var act: [Int] = []
        var intentos = 0
        while intentos < 400 && act.count < 64 && !s.isEmpty {
            intentos += 1
            let x = Azar.ent(s.count)
            if s[x].A > 0.45 && !s[x].fusion && !Palabras.vacia(s[x].et) && !act.contains(x) { act.append(x) }
        }
        if act.count < 2 { return nil }
        let a = act[Azar.ent(act.count)]
        var b = -1
        var mejor: Float = 1e9
        for x in act where x != a {
            var d: Float = 0
            for k in 0 ..< 8 {
                let e = s[a].f[k] - s[x].f[k]
                d += e * e
            }
            for ac in s[a].v where posAcople(x, ac.j) != nil { d -= 0.05 }
            d += Azar.rango(0, ruido)
            if d < mejor { mejor = d; b = x }
        }
        if b < 0 { return nil }
        let et = String(s[a].et.prefix(10)) + "⊕" + String(s[b].et.prefix(10))
        guard let id = busca(et) ?? nuevo(et, fusion: true) else { return nil }
        var f: [Float] = []
        for k in 0 ..< 8 { f.append((s[a].f[k] + s[b].f[k]) / 2) }
        s[id].f = f
        s[id].fase = (s[a].fase + s[b].fase) / 2
        s[id].A = min(1, (s[a].A + s[b].A) / 2 + 0.1)
        fija(a, id, w: 0.5, th: 0.1)
        fija(b, id, w: 0.5, th: -0.1)
        fija(id, a, w: 0.5, th: 0.1)
        fija(id, b, w: 0.5, th: -0.1)
        refuerza(a, b, dw: 0.2, th: 0, dsec: 0)
        ideas += 1
        return (id, a, b)
    }

    /// Un ciclo de pensamiento local: relaja lo que está en el foco.
    func piensa() {
        let reg = region(foco, tope: 120)
        for i in reg { decae(i, ruido: ruido) }
        for i in reg { propaga(i) }
        for o in obj { s[o].A = min(1, s[o].A + 0.01) }
        ciclos += 1
    }

    var cuentaResh: Int { s.reduce(0) { $0 + ($1.sabe ? 1 : 0) } }

    var mejorAccion: Accion {
        var mejor = 0
        for a in 1 ..< numAcciones where info.gusto[a] + valor[a] > info.gusto[mejor] + valor[mejor] { mejor = a }
        return Accion(rawValue: mejor) ?? .hablar
    }

    /// Tema de ahora: lo más activo de su foco (o un objetivo).
    func tema() -> Int? {
        var mejor: Int? = nil
        var mA: Float = -1
        for f in foco {
            if s[f].fusion || Palabras.vacia(s[f].et) { continue }
            let a = s[f].A + Azar.rango(0, 0.3)
            if a > mA { mA = a; mejor = f }
        }
        if mejor == nil && !obj.isEmpty { mejor = obj[Azar.ent(obj.count)] }
        return mejor
    }

    /// Palabra que quiere aprender a decir en Resh.
    func duda() -> Int? {
        var mejor: Int? = nil
        var mA: Float = 0.3
        for i in 0 ..< s.count {
            let x = s[i]
            if x.sabe || x.fusion || Palabras.vacia(x.et) || Resh.deEspanol(x.et) == nil { continue }
            let p = x.A + 0.05 * Float(min(10, x.usos))
            if p > mA { mA = x.A; mejor = i }
        }
        return mejor
    }

    func yaDijo(_ es: String) -> Bool {
        return ventana.contains("dije: " + es)
    }
}
