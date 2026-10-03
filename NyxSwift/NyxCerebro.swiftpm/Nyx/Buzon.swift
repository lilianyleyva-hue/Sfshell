import Foundation

// Buzon.swift — cómo se pasan conocimiento las mentes.
//
// Cuando una mente aprende algo que vale la pena (una palabra Resh, un
// concepto cristalizado, una frase que te gustó), lo ENVÍA. Las demás lo
// recogen de su buzón cuando les toca y lo absorben.
// En la app el buzón es SwiftData (AlmacenSwiftData.swift): cada envío es
// un registro @Model que queda guardado. Aquí está también un buzón en
// memoria, para cuando no hay SwiftData.

enum TipoEnvio: String {
    case palabra    // "árbol=yuz": cómo se dice en Resh
    case concepto   // "camino⊕tiene⊕historia": una idea cristalizada
    case frase      // una frase entera que vale la pena

    var icono: String {
        switch self {
        case .palabra: return "🔤"
        case .concepto: return "💎"
        case .frase: return "💬"
        }
    }
}

struct Envio {
    let de: Int
    let para: Int          // -1: para todas
    let tipo: TipoEnvio
    let contenido: String
}

protocol Buzon: AnyObject {
    /// Deja un envío.
    func envia(_ e: Envio)
    /// Lo que le llegó a 'rol' y aún no recogió (y lo marca como recogido).
    func recoge(para rol: Int, maximo: Int) -> [Envio]
}

/// Buzón en memoria (sin SwiftData).
final class BuzonMemoria: Buzon {
    private var envios: [(envio: Envio, recogido: Int)] = []

    func envia(_ e: Envio) {
        envios.append((e, 0))
        if envios.count > 500 { envios.removeFirst(envios.count - 500) }
    }

    func recoge(para rol: Int, maximo: Int) -> [Envio] {
        var out: [Envio] = []
        let bit = 1 << rol
        for k in 0 ..< envios.count where out.count < maximo {
            let e = envios[k].envio
            let paraMi = e.para == rol || (e.para == -1 && e.de != rol)
            if paraMi && envios[k].recogido & bit == 0 {
                envios[k].recogido |= bit
                out.append(e)
            }
        }
        return out
    }
}

extension Mente {
    /// Absorbe lo que otra mente le envió. Devuelve una descripción.
    func absorbe(_ e: Envio) -> String? {
        switch e.tipo {
        case .palabra:
            let p = e.contenido.split(separator: "=").map(String.init)
            guard p.count == 2 else { return nil }
            let id = busca(p[0]) ?? ingesta(p[0], dialogo: true).first
            guard let i = id, !s[i].sabe else { return nil }
            s[i].sabe = true
            reshAprendidas += 1
            return "aprendió a decir «\(p[0])» = \(p[1])"
        case .concepto:
            return adoptaConcepto(e.contenido)
        case .frase:
            let ids = ingesta(e.contenido, dialogo: true)
            if ids.isEmpty { return nil }
            for id in ids where !Palabras.vacia(s[id].et) { s[id].A = min(1, s[id].A + 0.1) }
            return "leyó: «\(e.contenido)»"
        }
    }

    /// Hace suyo un concepto que otra cristalizó ("a⊕b⊕c").
    private func adoptaConcepto(_ et: String) -> String? {
        if busca(et) != nil { return nil }
        let partes = et.split(separator: "⊕").map(String.init)
        guard partes.count >= 2 else { return nil }
        let ids = ingesta(partes.joined(separator: " "), dialogo: true)
        guard ids.count >= 2, let id = nuevo(et, fusion: true) else { return nil }
        s[id].A = 0.85
        s[id].dialogo = 1
        for x in ids {
            fija(x, id, w: 0.7, th: 0)
            fija(id, x, w: 0.7, th: 0)
        }
        return "adoptó el concepto «\(et)»"
    }
}
