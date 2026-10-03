import Foundation
#if canImport(SwiftData)
import SwiftData

// AlmacenSwiftData.swift — el conocimiento de las mentes, guardado con SwiftData.
//
//  · SaberMente: lo que sabe cada mente (sus semiones, acoples y gustos).
//    Una fila por mente, más una para las frases que recuerdan.
//  · Transferencia: lo que una mente le pasa a las demás (una palabra Resh,
//    un concepto, una frase). Las otras lo recogen de aquí cuando les toca.

@Model
final class SaberMente {
    var rol: Int              // 0…17 las mentes · 100: frases recordadas
    var texto: String
    var actualizado: Date

    init(rol: Int, texto: String) {
        self.rol = rol
        self.texto = texto
        self.actualizado = Date()
    }
}

@Model
final class Transferencia {
    var de: Int
    var para: Int             // -1: para todas
    var tipo: String          // palabra · concepto · frase
    var contenido: String
    var fecha: Date
    var recogidos: Int        // un bit por cada mente que ya lo recogió

    init(de: Int, para: Int, tipo: String, contenido: String) {
        self.de = de
        self.para = para
        self.tipo = tipo
        self.contenido = contenido
        self.fecha = Date()
        self.recogidos = 0
    }

    var cuantasRecogieron: Int { recogidos.nonzeroBitCount }
}

/// El buzón de las mentes hecho con SwiftData: cada envío queda guardado.
final class BuzonSwiftData: Buzon {
    let contexto: ModelContext
    /// Avisa a la pantalla cuando cambia algo.
    var alCambiar: (() -> Void)?

    init(contexto: ModelContext) {
        self.contexto = contexto
    }

    func envia(_ e: Envio) {
        let t = Transferencia(de: e.de, para: e.para, tipo: e.tipo.rawValue, contenido: e.contenido)
        t.recogidos = e.recogidos
        contexto.insert(t)
        try? contexto.save()
        alCambiar?()
    }

    /// Las últimas transferencias (las más nuevas primero).
    func recientes(_ n: Int) -> [Transferencia] {
        var d = FetchDescriptor<Transferencia>(sortBy: [SortDescriptor(\Transferencia.fecha, order: .reverse)])
        d.fetchLimit = n
        return (try? contexto.fetch(d)) ?? []
    }

    func recoge(para rol: Int, maximo: Int) -> [Envio] {
        let bit = 1 << rol
        var out: [Envio] = []
        for t in recientes(300).reversed() where out.count < maximo {
            let paraMi = t.para == rol || (t.para == -1 && t.de != rol)
            if !paraMi || t.recogidos & bit != 0 { continue }
            t.recogidos |= bit
            if let tipo = TipoEnvio(rawValue: t.tipo) {
                out.append(Envio(de: t.de, para: t.para, tipo: tipo, contenido: t.contenido))
            }
        }
        if !out.isEmpty {
            try? contexto.save()
            alCambiar?()
        }
        return out
    }

    /// Borra lo viejo para que no crezca sin fin.
    func limpia(dejando n: Int) {
        let todas = recientes(5000)
        if todas.count <= n { return }
        for t in todas[n...] { contexto.delete(t) }
        try? contexto.save()
    }
}

/// Guarda y carga el conocimiento de las 18 en SwiftData.
enum AlmacenSwiftData {
    static func guarda(_ c: Consejo, en contexto: ModelContext) {
        let filas = (try? contexto.fetch(FetchDescriptor<SaberMente>())) ?? []
        var porRol: [Int: SaberMente] = [:]
        for f in filas { porRol[f.rol] = f }
        for m in c.mentes {
            pon(c.textoMente(m), rol: m.rol, en: contexto, porRol: porRol)
        }
        // (el consejo antiguo se guarda aparte: guardaAntiguo)
        pon(c.textoFrases(), rol: 100, en: contexto, porRol: porRol)
        try? contexto.save()
    }

    private static func pon(_ texto: String, rol: Int, en contexto: ModelContext, porRol: [Int: SaberMente]) {
        if let f = porRol[rol] {
            f.texto = texto
            f.actualizado = Date()
        } else {
            contexto.insert(SaberMente(rol: rol, texto: texto))
        }
    }

    /// La memoria del consejo lógico (rol 300): sus hechos, reglas y verdades.
    static func guardaLogico(_ texto: String, en contexto: ModelContext) {
        let filas = (try? contexto.fetch(FetchDescriptor<SaberMente>())) ?? []
        if let f = filas.first(where: { $0.rol == 300 }) {
            f.texto = texto
            f.actualizado = Date()
        } else {
            contexto.insert(SaberMente(rol: 300, texto: texto))
        }
        try? contexto.save()
    }

    static func cargaLogico(de contexto: ModelContext) -> String? {
        let filas = (try? contexto.fetch(FetchDescriptor<SaberMente>())) ?? []
        return filas.first(where: { $0.rol == 300 })?.texto
    }

    /// Un texto cualquiera con su número (400: lo aprendido en las misiones).
    static func guardaTexto(_ texto: String, rol: Int, en contexto: ModelContext) {
        let filas = (try? contexto.fetch(FetchDescriptor<SaberMente>())) ?? []
        if let f = filas.first(where: { $0.rol == rol }) {
            f.texto = texto
            f.actualizado = Date()
        } else {
            contexto.insert(SaberMente(rol: rol, texto: texto))
        }
        try? contexto.save()
    }

    static func cargaTexto(rol: Int, de contexto: ModelContext) -> String? {
        let filas = (try? contexto.fetch(FetchDescriptor<SaberMente>())) ?? []
        return filas.first(where: { $0.rol == rol })?.texto
    }

    /// La memoria del consejo antiguo (rol 200).
    static func guardaAntiguo(_ datos: Data, en contexto: ModelContext) {
        let texto = String(decoding: datos, as: UTF8.self)
        let filas = (try? contexto.fetch(FetchDescriptor<SaberMente>())) ?? []
        if let f = filas.first(where: { $0.rol == 200 }) {
            f.texto = texto
            f.actualizado = Date()
        } else {
            contexto.insert(SaberMente(rol: 200, texto: texto))
        }
        try? contexto.save()
    }

    static func cargaAntiguo(de contexto: ModelContext) -> Data? {
        let filas = (try? contexto.fetch(FetchDescriptor<SaberMente>())) ?? []
        guard let f = filas.first(where: { $0.rol == 200 }) else { return nil }
        return Data(f.texto.utf8)
    }

    static func carga(de contexto: ModelContext) -> Consejo? {
        guard let filas = try? contexto.fetch(FetchDescriptor<SaberMente>()), !filas.isEmpty else { return nil }
        var mentes: [String] = []
        for r in 0 ..< numRoles {
            guard let f = filas.first(where: { $0.rol == r }) else { return nil }
            mentes.append(f.texto)
        }
        let frases = filas.first(where: { $0.rol == 100 })?.texto ?? ""
        return Consejo.desdePartes(mentes: mentes, frases: frases)
    }
}
#endif
