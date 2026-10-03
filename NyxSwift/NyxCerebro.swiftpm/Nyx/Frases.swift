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

    /// La frase que esta mente diría sobre 'centro'. ctx: lo que se preguntó.
    func elige(mente m: Mente, centro: Int, ctx: [Int], esEco: (String) -> Bool) -> (Int, Float)? {
        let et = m.s[centro].et
        let ctxEt: Set<String> = Set(ctx.filter { $0 != centro }.map { m.s[$0].et })
        var mejor: Int? = nil
        var mp: Float = -1e9
        for k in 0 ..< frases.count {
            let f = frases[k]
            if !f.toks.contains(et) { continue }
            var enCtx = 0
            var act: Float = 0
            var conocidas = 0
            for t in f.toks where !Palabras.vacia(t) {
                if ctxEt.contains(t) { enCtx += 1 }
                if let id = m.busca(t) {
                    act += m.s[id].A
                    conocidas += 1
                }
            }
            var p: Float = Float(enCtx) + f.puntos + Azar.rango(0, m.ruido + 0.05)
            if conocidas > 0 { p += 0.5 * act / Float(conocidas) }
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
