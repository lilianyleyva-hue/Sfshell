import Foundation

// Comprension.swift — entre ellas se hablan en Resh.
//
// Cuando una mente habla, dice en Resh las palabras que sabe (las que no,
// en español). Quien escucha solo ENTIENDE las palabras Resh que sabe: las
// demás se le quedan como dudas y luego pregunta ("ye kivr?") hasta que
// alguna le contesta ("kivr ka azul"). Así aprender Resh importa de verdad.
// Tú sí sabes todo el Resh: lo que escribas en Resh lo entienden.

extension Resh {
    /// resh -> español (la primera palabra que usa esa forma).
    static let inverso: [String: String] = {
        var d: [String: String] = [:]
        for (es, r) in particulas { d[r] = es }
        for (es, r) in lexico where d[r] == nil { d[r] = es }
        return d
    }()

    /// Partículas de la gramática Resh (mi, ko, ye, va…).
    static let formasParticula: Set<String> = Set(particulas.values)

    static func aEspanol(_ r: String) -> String? { inverso[r] }

    /// ¿Es una palabra Resh (y no una palabra española)?
    static func esForma(_ t: String) -> Bool {
        return inverso[t] != nil && lexico[t] == nil && particulas[t] == nil
    }

    /// Español → Resh palabra por palabra (las que no tienen Resh se quedan).
    static func traduceTexto(_ es: String) -> String {
        return Palabras.tokens(es, max: 60).map { deEspanol($0) ?? $0 }.joined(separator: " ")
    }

    /// Lo que escribe el humano: sus palabras Resh se traducen (el humano las sabe todas).
    static func traduceHumano(_ texto: String) -> String {
        var out: [String] = []
        for t in Palabras.tokens(texto, max: 40) {
            if formasParticula.contains(t) && esForma(t) { continue }
            if esForma(t), let es = inverso[t] { out.append(es) } else { out.append(t) }
        }
        return out.joined(separator: " ")
    }
}

extension Mente {
    /// Oye una frase en Resh: entiende solo las palabras que sabe.
    func comprende(_ resh: String) -> (entendido: String, desconocidas: [String]) {
        var es: [String] = []
        var no: [String] = []
        for t in Palabras.tokens(resh, max: 40) {
            if Resh.formasParticula.contains(t) && Resh.esForma(t) { continue }   // gramática: se entiende
            if Resh.esForma(t), let palabra = Resh.aEspanol(t) {
                if let id = busca(palabra), s[id].sabe {
                    es.append(palabra)
                } else {
                    no.append(t)
                }
            } else {
                es.append(t)      // quien habló no lo sabía en Resh: lo dijo en español
            }
        }
        for t in no where !reshOido.contains(t) { reshOido.append(t) }
        if reshOido.count > 20 { reshOido.removeFirst(reshOido.count - 20) }
        return (es.joined(separator: " "), no)
    }

    /// Aprende qué significa una palabra Resh.
    func aprendeResh(_ r: String, significa es: String) {
        let id = busca(es) ?? ingesta(es, dialogo: true).first
        if let i = id, !s[i].sabe {
            s[i].sabe = true
            reshAprendidas += 1
        }
        reshOido.removeAll { $0 == r }
    }
}
