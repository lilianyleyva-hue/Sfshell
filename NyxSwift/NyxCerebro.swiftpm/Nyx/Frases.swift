import Foundation

// Frases.swift — memoria de frases: lo que leyeron, entero.
// Para responder, la mente que habla busca las frases que contienen su idea
// ganadora y elige la que mejor encaja con la pregunta y con lo que tiene
// activo. Tu opinión (bien/mal) también cuenta.

struct FraseMem {
    var texto: String
    var toks: [String]
    var puntos: Float
    var usos: Int
}

final class MemoriaFrases {
    private(set) var frases: [FraseMem] = []
    var indiceTexto: [String: Int] = [:]
    /// Nyx 2: memoria grande (antes 1500) con índice, como el índice de un
    /// libro: raíz de palabra → frases donde sale. Buscar ya no recorre todo.
    let maximo = 40000
    private var porRaiz: [String: [Int]] = [:]

    private func indexa(_ k: Int) {
        for r in Set(frases[k].toks.map { MemoriaFrases.raiz($0) }) { porRaiz[r, default: []].append(k) }
    }

    private func desindexa(_ k: Int) {
        for r in Set(frases[k].toks.map { MemoriaFrases.raiz($0) }) {
            porRaiz[r]?.removeAll { $0 == k }
            if porRaiz[r]?.isEmpty == true { porRaiz[r] = nil }
        }
    }

    /// En cuántas frases sale una raíz.
    func cuantas(_ raiz: String) -> Int { porRaiz[raiz]?.count ?? 0 }

    /// Las frases que contienen alguna de estas raíces.
    func candidatas(_ raices: Set<String>) -> [Int] {
        var vistas = Set<Int>()
        var out: [Int] = []
        for r in raices {
            for k in porRaiz[r] ?? [] where !vistas.contains(k) {
                vistas.insert(k)
                out.append(k)
            }
        }
        return out
    }

    func usa(_ k: Int) {
        if k < frases.count { frases[k].usos += 1 }
    }

    /// Guarda una frase (si no la tenía). Devuelve su índice.
    @discardableResult
    func pon(_ texto: String, puntos: Float = 0, usos: Int = 1) -> Int? {
        let toks = Palabras.tokens(texto)
        if toks.count < 3 { return nil }        // "hola" no es una frase que recordar
        let limpio = toks.joined(separator: " ")
        if let k = indiceTexto[limpio] {
            frases[k].usos += 1
            return k
        }
        let f = FraseMem(texto: limpio, toks: toks, puntos: puntos, usos: usos)
        if frases.count < maximo {
            frases.append(f)
            indiceTexto[limpio] = frases.count - 1
            indexa(frases.count - 1)
            return frases.count - 1
        }
        var peor = 0
        for k in 1 ..< frases.count where valor(k) < valor(peor) { peor = k }
        indiceTexto[frases[peor].texto] = nil
        desindexa(peor)
        frases[peor] = f
        indiceTexto[limpio] = peor
        indexa(peor)
        return peor
    }

    private func valor(_ k: Int) -> Float {
        return frases[k].puntos + 0.1 * Float(frases[k].usos)
    }

    /// Raíz de una palabra: "brilla" y "brillan" son la misma idea.
    static func raiz(_ t: String) -> String {
        return t.count > 5 ? String(t.prefix(5)) : t
    }

    /// La frase que esta mente diría sobre 'centro'. ctx: lo que se preguntó.
    /// Vale más la frase que reúne MÁS palabras de la pregunta (las raras
    /// pesan más) y la que está más activada en la mente; la idea ganadora suma.
    /// tema: de qué se venía hablando (si la pregunta lo continúa, la frase
    /// tiene que seguir en ese tema: "háblame del mar" → "¿y de qué color es?").
    func elige(mente m: Mente, centro: Int, ctx: [Int], tema: [String] = [], esEco: (String) -> Bool) -> (Int, Float)? {
        let raicesTema: Set<String> = Set(tema.map { MemoriaFrases.raiz($0) })
        let et = m.s[centro].et
        let n = Float(max(1, frases.count))
        var pesoCtx: [String: Float] = [:]
        for id in ctx where id != centro && !Palabras.vacia(m.s[id].et) {
            let r = MemoriaFrases.raiz(m.s[id].et)
            pesoCtx[r] = log(1 + n / Float(max(1, cuantas(r)))) * m.pesoPista(id)
        }
        // solo las frases que tienen algo que ver (por el índice)
        var buscar = Set(pesoCtx.keys)
        buscar.insert(MemoriaFrases.raiz(et))
        for r in raicesTema { buscar.insert(r) }
        let act = m.activaDesde(ctx)
        var mejor: Int? = nil
        var mp: Float = -1e9
        for k in candidatas(buscar) {
            let f = frases[k]
            let tieneCentro = f.toks.contains(et)
            var enCtx: Float = 0
            var activada: Float = 0
            var cuentan = 0
            for t in f.toks where !Palabras.vacia(t) {
                enCtx += pesoCtx[MemoriaFrases.raiz(t)] ?? 0
                if let id = m.busca(t) {
                    activada += max(0, act[id] ?? 0)
                    cuentan += 1
                }
            }
            let enTema = !raicesTema.isEmpty && f.toks.contains { raicesTema.contains(MemoriaFrases.raiz($0)) }
            if !tieneCentro && enCtx == 0 && !enTema { continue }
            var p: Float = 0.6 * enCtx + f.puntos + Azar.rango(0, m.ruido + 0.05) + (tieneCentro ? 0.5 : 0)
            if !raicesTema.isEmpty { p += enTema ? 2.0 : -1.0 }
            if cuentan > 0 { p += 0.8 * activada / Float(cuentan) }
            if esEco(f.texto) { p -= 2 }       // no repetir lo que se acaba de decir
            if p > mp { mp = p; mejor = k }
        }
        guard let k = mejor else { return nil }
        return (k, mp)
    }

    /// La frase en Resh, con lo que esta mente sabe decir.
    func enResh(_ k: Int, mente m: Mente) -> String {
        var out: [String] = []
        for t in frases[k].toks {
            if let id = m.busca(t), m.s[id].sabe, let r = Resh.deEspanol(t) { out.append(r) } else { out.append(t) }
        }
        return out.joined(separator: " ")
    }

    func opina(_ k: Int?, bueno: Bool) {
        guard let k = k, k < frases.count else { return }
        frases[k].puntos = limita(frases[k].puntos + (bueno ? 0.5 : -1), -5, 5)
    }
}
