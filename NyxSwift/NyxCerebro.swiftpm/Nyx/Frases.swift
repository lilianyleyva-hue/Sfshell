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
    var frases: [FraseMem] = []
    var indiceTexto: [String: Int] = [:]
    let maximo = 1500

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
            return frases.count - 1
        }
        var peor = 0
        for k in 1 ..< frases.count where valor(k) < valor(peor) { peor = k }
        indiceTexto[frases[peor].texto] = nil
        frases[peor] = f
        indiceTexto[limpio] = peor
        return peor
    }

    private func valor(_ k: Int) -> Float {
        return frases[k].puntos + 0.1 * Float(frases[k].usos)
    }

    /// Raíz de una palabra: "brilla" y "brillan" son la misma idea.
    static func raiz(_ t: String) -> String {
        return t.count > 5 ? String(t.prefix(5)) : t
    }

    /// En cuántas frases sale cada raíz (para saber cuáles son raras).
    private func frecuencias() -> [String: Int] {
        var df: [String: Int] = [:]
        for f in frases {
            for t in Set(f.toks.map { MemoriaFrases.raiz($0) }) { df[t, default: 0] += 1 }
        }
        return df
    }

    /// La frase que esta mente diría sobre 'centro'. ctx: lo que se preguntó.
    /// Vale más la frase que reúne MÁS palabras de la pregunta (las raras
    /// pesan más) y la que está más activada en la mente; la idea ganadora suma.
    /// tema: de qué se venía hablando (si la pregunta lo continúa, la frase
    /// tiene que seguir en ese tema: "háblame del mar" → "¿y de qué color es?").
    func elige(mente m: Mente, centro: Int, ctx: [Int], tema: [String] = [], esEco: (String) -> Bool) -> (Int, Float)? {
        let raicesTema: Set<String> = Set(tema.map { MemoriaFrases.raiz($0) })
        let et = m.s[centro].et
        let df = frecuencias()
        let n = Float(max(1, frases.count))
        var pesoCtx: [String: Float] = [:]
        for id in ctx where id != centro && !Palabras.vacia(m.s[id].et) {
            let r = MemoriaFrases.raiz(m.s[id].et)
            pesoCtx[r] = log(1 + n / Float(df[r] ?? 1)) * m.pesoPista(id)
        }
        let act = m.activaDesde(ctx)
        var mejor: Int? = nil
        var mp: Float = -1e9
        for k in 0 ..< frases.count {
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
