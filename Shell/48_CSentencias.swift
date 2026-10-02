import Foundation

// ============================================================
// MARK: - C, C++ y Java: sentencias
// ============================================================

enum CSalto {
    case nada
    case rompe(String?)
    case sigue(String?)
    case vuelve(CV)
}

/// Excepción lanzada con throw (o por el propio lenguaje en Java).
struct CLanzado: Error {
    let v: CV
    let linea: Int
}

/// exit(n) / System.exit(n)
struct CSalida: Error {
    let codigo: Int
}

class CSent {
    let linea: Int
    init(_ linea: Int) { self.linea = linea }
    func exec(_ m: CMaq, _ f: CMarco) throws -> CSalto { .nada }
    func res(_ r: CResol) throws {}
}

final class CVacia: CSent {}

final class CBloque: CSent {
    var s: [CSent]
    let nuevoAmbito: Bool
    /// objetos de C++ declarados aquí que tienen destructor (se destruyen al salir)
    var destruir: [Int] = []
    init(_ s: [CSent], _ linea: Int, nuevoAmbito: Bool = true) {
        self.s = s
        self.nuevoAmbito = nuevoAmbito
        super.init(linea)
    }
    override func exec(_ m: CMaq, _ f: CMarco) throws -> CSalto {
        if !destruir.isEmpty { return try execConDestructores(m, f) }
        for x in s {
            let r = try x.exec(m, f)
            if case .nada = r { continue }
            return r
        }
        return .nada
    }

    func execConDestructores(_ m: CMaq, _ f: CMarco) throws -> CSalto {
        var resultado: CSalto = .nada
        do {
            for x in s {
                let r = try x.exec(m, f)
                if case .nada = r { continue }
                resultado = r
                break
            }
        } catch {
            try m.destruyeLocales(destruir, f, linea)
            throw error
        }
        try m.destruyeLocales(destruir, f, linea)
        return resultado
    }

    override func res(_ r: CResol) throws {
        if nuevoAmbito { r.abre() }
        r.destructores.append([])
        for x in s { try x.res(r) }
        destruir = r.destructores.removeLast()
        if nuevoAmbito { r.cierra() }
    }
}

final class CExprSent: CSent {
    let e: CExpr
    init(_ e: CExpr, _ linea: Int) {
        self.e = e
        super.init(linea)
    }
    override func exec(_ m: CMaq, _ f: CMarco) throws -> CSalto {
        _ = try e.ev(m, f)
        return .nada
    }
    override func res(_ r: CResol) throws { try e.res(r) }
}

/// Una variable declarada: int x = 5, a[10], *p, v(3, 0)…
final class CDeclVar {
    let nombre: String
    var tipo: CTipo
    var ini: CExpr?
    var ctorArgs: [CExpr]?
    /// tamaños de int a[n][m] (nil = [] sin tamaño)
    var dims: [CExpr?]
    var slot = -1
    var global = false
    var estatico = false
    var iniciada = false
    let linea: Int
    /// auto [a, b] = … (C++17)
    var enlaces: [String] = []
    var slotsEnlace: [Int] = []

    init(_ nombre: String, _ tipo: CTipo, ini: CExpr?, dims: [CExpr?], _ linea: Int) {
        self.nombre = nombre
        self.tipo = tipo
        self.ini = ini
        self.dims = dims
        self.linea = linea
    }
}

final class CDecl: CSent {
    let vars: [CDeclVar]
    init(_ vars: [CDeclVar], _ linea: Int) {
        self.vars = vars
        super.init(linea)
    }
    override func exec(_ m: CMaq, _ f: CMarco) throws -> CSalto {
        for d in vars {
            if d.estatico {
                if d.iniciada { continue }
                d.iniciada = true
            }
            let v = try m.valorDeclarado(d, f)
            if !d.enlaces.isEmpty {
                try m.enlaza(d, v, f)
                continue
            }
            if d.global { m.globales.a[d.slot] = v } else { f.s.a[d.slot] = v }
        }
        return .nada
    }
    override func res(_ r: CResol) throws {
        for d in vars { try r.resuelveDecl(d) }
    }
}

final class CSi: CSent {
    let c: CExpr
    let a: CSent
    let b: CSent?
    init(_ c: CExpr, _ a: CSent, _ b: CSent?, _ linea: Int) {
        self.c = c
        self.a = a
        self.b = b
        super.init(linea)
    }
    override func exec(_ m: CMaq, _ f: CMarco) throws -> CSalto {
        if m.aBool(try c.ev(m, f)) { return try a.exec(m, f) }
        if let b = b { return try b.exec(m, f) }
        return .nada
    }
    override func res(_ r: CResol) throws {
        r.abre()
        try c.res(r)
        r.abre()
        try a.res(r)
        r.cierra()
        r.abre()
        try b?.res(r)
        r.cierra()
        r.cierra()
    }
}

final class CMientras: CSent {
    let c: CExpr
    let cuerpo: CSent
    var etiqueta: String?
    init(_ c: CExpr, _ cuerpo: CSent, _ linea: Int) {
        self.c = c
        self.cuerpo = cuerpo
        super.init(linea)
    }
    override func exec(_ m: CMaq, _ f: CMarco) throws -> CSalto {
        while m.aBool(try c.ev(m, f)) {
            try m.revisaPaso()
            let r = try cuerpo.exec(m, f)
            switch r {
            case .nada: continue
            case .rompe(let e): if e == nil || e == etiqueta { return .nada }; return r
            case .sigue(let e): if e == nil || e == etiqueta { continue }; return r
            case .vuelve: return r
            }
        }
        return .nada
    }
    override func res(_ r: CResol) throws {
        r.abre()
        try c.res(r)
        try cuerpo.res(r)
        r.cierra()
    }
}

final class CHacer: CSent {
    let c: CExpr
    let cuerpo: CSent
    var etiqueta: String?
    init(_ cuerpo: CSent, _ c: CExpr, _ linea: Int) {
        self.c = c
        self.cuerpo = cuerpo
        super.init(linea)
    }
    override func exec(_ m: CMaq, _ f: CMarco) throws -> CSalto {
        repeat {
            try m.revisaPaso()
            let r = try cuerpo.exec(m, f)
            switch r {
            case .nada: break
            case .rompe(let e): if e == nil || e == etiqueta { return .nada }; return r
            case .sigue(let e): if e == nil || e == etiqueta { break }; return r
            case .vuelve: return r
            }
        } while m.aBool(try c.ev(m, f))
        return .nada
    }
    override func res(_ r: CResol) throws {
        r.abre()
        try cuerpo.res(r)
        r.cierra()
        try c.res(r)
    }
}

final class CPara: CSent {
    let ini: CSent?
    let c: CExpr?
    let paso: CExpr?
    let cuerpo: CSent
    var etiqueta: String?
    init(_ ini: CSent?, _ c: CExpr?, _ paso: CExpr?, _ cuerpo: CSent, _ linea: Int) {
        self.ini = ini
        self.c = c
        self.paso = paso
        self.cuerpo = cuerpo
        super.init(linea)
    }
    override func exec(_ m: CMaq, _ f: CMarco) throws -> CSalto {
        if let i = ini { _ = try i.exec(m, f) }
        while true {
            if let c = c, !m.aBool(try c.ev(m, f)) { break }
            try m.revisaPaso()
            let r = try cuerpo.exec(m, f)
            switch r {
            case .nada: break
            case .rompe(let e): if e == nil || e == etiqueta { return .nada }; return r
            case .sigue(let e): if e == nil || e == etiqueta { break }; return r
            case .vuelve: return r
            }
            if let p = paso { _ = try p.ev(m, f) }
        }
        return .nada
    }
    override func res(_ r: CResol) throws {
        r.abre()
        try ini?.res(r)
        try c?.res(r)
        try paso?.res(r)
        r.abre()
        try cuerpo.res(r)
        r.cierra()
        r.cierra()
    }
}

/// for (auto x : v) de C++ y for (int x : arr) de Java.
final class CParaCada: CSent {
    let nombres: [String]
    let tipoVar: CTipo
    let esRef: Bool
    let col: CExpr
    let cuerpo: CSent
    var slots: [Int] = []
    var etiqueta: String?
    init(_ nombres: [String], _ tipoVar: CTipo, esRef: Bool, _ col: CExpr, _ cuerpo: CSent, _ linea: Int) {
        self.nombres = nombres
        self.tipoVar = tipoVar
        self.esRef = esRef
        self.col = col
        self.cuerpo = cuerpo
        super.init(linea)
    }
    override func exec(_ m: CMaq, _ f: CMarco) throws -> CSalto {
        var base: CLugar?
        let v: CV
        if col.asignable {
            let l = try col.lugar(m, f)
            base = l
            v = try m.lee(l)
        } else {
            v = try col.ev(m, f)
        }
        let lugares = try m.elementos(v, base, linea)
        for l in lugares {
            try m.revisaPaso()
            try m.asignaElemento(self, l, f)
            let r = try cuerpo.exec(m, f)
            switch r {
            case .nada: break
            case .rompe(let e): if e == nil || e == etiqueta { return .nada }; return r
            case .sigue(let e): if e == nil || e == etiqueta { break }; return r
            case .vuelve: return r
            }
        }
        return .nada
    }
    override func res(_ r: CResol) throws {
        r.abre()
        try col.res(r)
        let te = r.tipoElemento(col.tipo)
        let t: CTipo
        if case .auto = tipoVar { t = te ?? .auto } else { t = tipoVar }
        slots = []
        if nombres.count == 1 {
            slots.append(try r.declara(nombres[0], esRef ? .ref(t) : t, linea))
        } else {
            for n in nombres { slots.append(try r.declara(n, .auto, linea)) }
        }
        r.abre()
        try cuerpo.res(r)
        r.cierra()
        r.cierra()
    }
}

final class CCaso {
    let valor: CExpr?
    let indice: Int
    init(_ valor: CExpr?, _ indice: Int) {
        self.valor = valor
        self.indice = indice
    }
}

final class CSwitch: CSent {
    let e: CExpr
    let cuerpo: [CSent]
    let casos: [CCaso]
    var etiqueta: String?
    var tabla: [Int64: Int]?
    init(_ e: CExpr, _ cuerpo: [CSent], _ casos: [CCaso], _ linea: Int) {
        self.e = e
        self.cuerpo = cuerpo
        self.casos = casos
        super.init(linea)
    }
    override func exec(_ m: CMaq, _ f: CMarco) throws -> CSalto {
        let v = try e.ev(m, f)
        var inicio = -1
        if let t = tabla, case .n(let x, _) = v {
            inicio = t[x] ?? -1
        } else {
            for c in casos {
                guard let ve = c.valor else { continue }
                if m.igualesCaso(v, try ve.ev(m, f)) { inicio = c.indice; break }
            }
        }
        if inicio < 0, let d = casos.first(where: { $0.valor == nil }) { inicio = d.indice }
        if inicio < 0 { return .nada }
        var k = inicio
        while k < cuerpo.count {
            let r = try cuerpo[k].exec(m, f)
            switch r {
            case .nada: break
            case .rompe(let et): if et == nil || et == etiqueta { return .nada }; return r
            default: return r
            }
            k += 1
        }
        return .nada
    }
    override func res(_ r: CResol) throws {
        try e.res(r)
        r.abre()
        for c in casos { if let v = c.valor { try r.resuelveCaso(v, e.tipo) } }
        for s in cuerpo { try s.res(r) }
        r.cierra()
        // casos con constantes enteras: tabla directa
        var t: [Int64: Int] = [:]
        var todos = true
        for c in casos {
            guard let v = c.valor else { continue }
            if let l = v as? CLit, case .n(let x, _) = l.v { if t[x] == nil { t[x] = c.indice } }
            else if let n = v as? CNombre, case .constante(let cv) = n.modo, case .n(let x, _) = cv { if t[x] == nil { t[x] = c.indice } }
            else if let u = v as? CUn, u.op == "-", let l = u.a as? CLit, case .n(let x, _) = l.v { t[0 &- x] = c.indice }
            else { todos = false }
        }
        tabla = todos ? t : nil
    }
}

final class CRompe: CSent {
    let etiqueta: String?
    init(_ etiqueta: String?, _ linea: Int) {
        self.etiqueta = etiqueta
        super.init(linea)
    }
    override func exec(_ m: CMaq, _ f: CMarco) throws -> CSalto { .rompe(etiqueta) }
}

final class CSigue: CSent {
    let etiqueta: String?
    init(_ etiqueta: String?, _ linea: Int) {
        self.etiqueta = etiqueta
        super.init(linea)
    }
    override func exec(_ m: CMaq, _ f: CMarco) throws -> CSalto { .sigue(etiqueta) }
}

final class CVuelve: CSent {
    let e: CExpr?
    init(_ e: CExpr?, _ linea: Int) {
        self.e = e
        super.init(linea)
    }
    override func exec(_ m: CMaq, _ f: CMarco) throws -> CSalto {
        guard let e = e else { return .vuelve(.vacio) }
        if f.devuelveRef, e.asignable {
            return .vuelve(.ref(CRefCaja(try e.lugar(m, f))))
        }
        let v = try e.ev(m, f)
        // return local; → el objeto sale entero (sin destruirlo ni copiarlo)
        if m.dialecto == .cpp, case .o = v, let n = e as? CNombre, case .local(0, let s) = n.modo { f.s.a[s] = .vacio }
        return .vuelve(v)
    }
    override func res(_ r: CResol) throws { try e?.res(r) }
}

final class CEtiqueta: CSent {
    let nombre: String
    let s: CSent
    init(_ nombre: String, _ s: CSent, _ linea: Int) {
        self.nombre = nombre
        self.s = s
        super.init(linea)
        if let x = s as? CPara { x.etiqueta = nombre }
        if let x = s as? CMientras { x.etiqueta = nombre }
        if let x = s as? CHacer { x.etiqueta = nombre }
        if let x = s as? CParaCada { x.etiqueta = nombre }
        if let x = s as? CSwitch { x.etiqueta = nombre }
    }
    override func exec(_ m: CMaq, _ f: CMarco) throws -> CSalto {
        let r = try s.exec(m, f)
        if case .rompe(let e) = r, e == nombre { return .nada }
        return r
    }
    override func res(_ r: CResol) throws { try s.res(r) }
}

final class CCaptura {
    /// tipos que atrapa (vacío = catch (...))
    let tipos: [CTipo]
    let nombre: String?
    let cuerpo: CSent
    var slot = -1
    init(_ tipos: [CTipo], _ nombre: String?, _ cuerpo: CSent) {
        self.tipos = tipos
        self.nombre = nombre
        self.cuerpo = cuerpo
    }
}

final class CIntenta: CSent {
    let cuerpo: CSent
    let capturas: [CCaptura]
    let final: CSent?
    let recursos: [CDecl]
    init(_ cuerpo: CSent, _ capturas: [CCaptura], _ final: CSent?, recursos: [CDecl], _ linea: Int) {
        self.cuerpo = cuerpo
        self.capturas = capturas
        self.final = final
        self.recursos = recursos
        super.init(linea)
    }
    override func exec(_ m: CMaq, _ f: CMarco) throws -> CSalto {
        var resultado: CSalto = .nada
        var pendiente: Error?
        do {
            for d in recursos { _ = try d.exec(m, f) }
            resultado = try cuerpo.exec(m, f)
        } catch let e as CLanzado {
            if let c = capturas.first(where: { m.atrapa($0, e.v) }) {
                if c.slot >= 0 { f.s.a[c.slot] = e.v }
                do { resultado = try c.cuerpo.exec(m, f) } catch { pendiente = error }
            } else {
                pendiente = e
            }
        } catch let e as CFallo {
            // errores del propio Java (división entre cero, índice…) se pueden atrapar
            if m.dialecto == .java, let v = m.excepcionDeFallo(e),
               let c = capturas.first(where: { m.atrapa($0, v) }) {
                if c.slot >= 0 { f.s.a[c.slot] = v }
                do { resultado = try c.cuerpo.exec(m, f) } catch { pendiente = error }
            } else {
                pendiente = e
            }
        }
        if let fin = final {
            let r = try fin.exec(m, f)
            if case .nada = r {} else { return r }
        }
        if let p = pendiente { throw p }
        return resultado
    }
    override func res(_ r: CResol) throws {
        r.abre()
        for d in recursos { try d.res(r) }
        try cuerpo.res(r)
        r.cierra()
        for c in capturas {
            r.abre()
            if let n = c.nombre { c.slot = try r.declara(n, c.tipos.first ?? .auto, cuerpo.linea) }
            try c.cuerpo.res(r)
            r.cierra()
        }
        try final?.res(r)
    }
}

final class CLanza: CSent {
    let e: CExpr?
    init(_ e: CExpr?, _ linea: Int) {
        self.e = e
        super.init(linea)
    }
    override func exec(_ m: CMaq, _ f: CMarco) throws -> CSalto {
        guard let e = e else {
            if let v = m.ultimaExcepcion { throw CLanzado(v: v, linea: linea) }
            throw CFallo("throw; sin excepción activa", linea)
        }
        let v = try e.ev(m, f)
        if case .nulo = v { throw m.excepcion("NullPointerException", "throw null", linea) }
        m.ultimaExcepcion = v
        throw CLanzado(v: v, linea: linea)
    }
    override func res(_ r: CResol) throws { try e?.res(r) }
}

/// yield de los switch de Java (case X -> { … yield valor; })
final class CRinde: CSent {
    let e: CExpr
    init(_ e: CExpr, _ linea: Int) {
        self.e = e
        super.init(linea)
    }
    override func exec(_ m: CMaq, _ f: CMarco) throws -> CSalto { .vuelve(try e.ev(m, f)) }
    override func res(_ r: CResol) throws { try e.res(r) }
}
