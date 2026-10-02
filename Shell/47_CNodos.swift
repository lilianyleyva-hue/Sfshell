import Foundation

// ============================================================
// MARK: - C, C++ y Java: expresiones
// ============================================================
// Cada nodo sabe evaluarse (ev), dar su lugar en memoria si se le
// puede asignar (lugar) y resolver sus nombres antes de ejecutar
// (res): así las variables locales son posiciones fijas del marco
// y no se buscan por nombre en cada paso.

enum COp: UInt8 {
    case suma, resta, mult, div, mod, shl, shr, ushr, y, o, xor, menor, mayor, menorIg, mayorIg, igual, distinto

    static func de(_ s: String) -> COp? {
        switch s {
        case "+": return .suma
        case "-": return .resta
        case "*": return .mult
        case "/": return .div
        case "%": return .mod
        case "<<": return .shl
        case ">>": return .shr
        case ">>>": return .ushr
        case "&": return .y
        case "|": return .o
        case "^": return .xor
        case "<": return .menor
        case ">": return .mayor
        case "<=": return .menorIg
        case ">=": return .mayorIg
        case "==": return .igual
        case "!=": return .distinto
        default: return nil
        }
    }

    var texto: String {
        switch self {
        case .suma: return "+"
        case .resta: return "-"
        case .mult: return "*"
        case .div: return "/"
        case .mod: return "%"
        case .shl: return "<<"
        case .shr: return ">>"
        case .ushr: return ">>>"
        case .y: return "&"
        case .o: return "|"
        case .xor: return "^"
        case .menor: return "<"
        case .mayor: return ">"
        case .menorIg: return "<="
        case .mayorIg: return ">="
        case .igual: return "=="
        case .distinto: return "!="
        }
    }

    var esComparacion: Bool { rawValue >= COp.menor.rawValue }
}

class CExpr {
    let linea: Int
    var tipo: CTipo?
    init(_ linea: Int) { self.linea = linea }

    func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        throw CFallo("expresión sin evaluar", linea)
    }
    func lugar(_ m: CMaq, _ f: CMarco) throws -> CLugar {
        throw CFallo("no se puede asignar a esta expresión", linea)
    }
    var asignable: Bool { false }
    func res(_ r: CResol) throws {}
}

// MARK: literales y nombres

final class CLit: CExpr {
    let v: CV
    init(_ v: CV, _ linea: Int, tipo: CTipo? = nil) {
        self.v = v
        super.init(linea)
        self.tipo = tipo
    }
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV { v }
}

/// "texto": en C es un arreglo de char; en C++/Java, una cadena.
final class CLitCad: CExpr {
    let s: String
    var mem: CMem?
    var valor: CV?
    init(_ s: String, _ linea: Int) {
        self.s = s
        super.init(linea)
    }
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        if let v = valor { return v }
        if m.dialecto == .java {
            let v = CV.s(Array(s.utf16))
            valor = v
            return v
        }
        // C y C++: const char* a memoria propia del literal
        let bytes = Array(s.utf8)
        var celdas: [CV] = bytes.map { CV.n(Int64(Int8(bitPattern: $0)), .char) }
        celdas.append(.n(0, .char))
        let mm = CMem(celdas)
        mm.elem = .num(.char)
        let v = CV.p(CPtr(mem: mm, i: 0))
        valor = v
        return v
    }
    override func res(_ r: CResol) throws {
        tipo = r.m.dialecto == .java ? .cad : .ptr(.num(.char))
    }
}

enum CModo {
    case sinResolver
    case local(Int, Int)
    case global(Int)
    case campo(Int)
    case estatico(CClase, Int)
    case funcion(String)
    case metodo(String)
    case clase(CClase)
    case constante(CV)
    case nativo(String)
}

final class CNombre: CExpr {
    let nombre: String
    var modo: CModo = .sinResolver
    /// max<string>(…): tipos explícitos de una plantilla
    var plantilla: [CTipo] = []
    init(_ nombre: String, _ linea: Int) {
        self.nombre = nombre
        super.init(linea)
    }

    override var asignable: Bool {
        switch modo {
        case .local, .global, .campo, .estatico: return true
        default: return false
        }
    }

    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        var v: CV
        switch modo {
        case .local(let d, let s):
            if d == 0 { v = f.s.a[s] } else { v = CNombre.marco(f, d).s.a[s] }
        case .global(let i):
            v = m.globales.a[i]
        case .campo(let i):
            guard let o = f.este else { throw CFallo("'\(nombre)' necesita un objeto (this)", linea) }
            v = o.c.a[i]
        case .estatico(let c, let i):
            try m.preparaEstaticos(c)
            v = c.estaticos.a[i]
        case .constante(let c):
            return c
        case .funcion(let n):
            return try m.valorFuncion(n, linea)
        case .metodo(let n):
            guard let c = f.este?.clase, let fn = c.metodo(n, -1) ?? c.metodos[n]?.first else {
                throw CFallo("'\(nombre)' no es un valor", linea)
            }
            return .f(CCierre(fn: fn, marco: nil, este: f.este))
        case .clase(let c):
            throw CFallo("'\(c.nombre)' es un tipo, no un valor", linea)
        case .nativo(let n):
            return try m.valorNativo(n, linea)
        case .sinResolver:
            throw CFallo("'\(nombre)' sin resolver", linea)
        }
        if case .ref(let r) = v { v = try m.lee(r.l) }
        if case .vacio = v { throw CFallo("variable '\(nombre)' usada antes de tener valor", linea) }
        return v
    }

    static func marco(_ f: CMarco, _ d: Int) -> CMarco {
        var x = f
        var k = d
        while k > 0, let p = x.padre { x = p; k -= 1 }
        return x
    }

    override func lugar(_ m: CMaq, _ f: CMarco) throws -> CLugar {
        let l: CLugar
        switch modo {
        case .local(let d, let s):
            l = .celda(d == 0 ? f.s : CNombre.marco(f, d).s, s)
        case .global(let i):
            l = .celda(m.globales, i)
        case .campo(let i):
            guard let o = f.este else { throw CFallo("'\(nombre)' necesita un objeto (this)", linea) }
            l = .celda(o.c, i)
        case .estatico(let c, let i):
            try m.preparaEstaticos(c)
            l = .celda(c.estaticos, i)
        default:
            throw CFallo("no se puede asignar a '\(nombre)'", linea)
        }
        // si la variable es una referencia (int& x), el lugar es el de destino
        if case .celda(let mem, let i) = l, i < mem.a.count, case .ref(let r) = mem.a[i] { return r.l }
        return l
    }

    override func res(_ r: CResol) throws {
        try r.resuelveNombre(self)
    }
}

final class CEste: CExpr {
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        guard let o = f.este else { throw CFallo("'this' fuera de un método", linea) }
        // en C++ this es un puntero; en Java, el objeto
        return .o(o)
    }
    override func res(_ r: CResol) throws {
        if let c = r.claseActual { tipo = r.m.dialecto == .java ? .clase(c.nombre) : .ptr(.clase(c.nombre)) }
    }
}

// MARK: operadores

final class CBin: CExpr {
    let op: COp
    let a: CExpr
    let b: CExpr
    init(_ op: COp, _ a: CExpr, _ b: CExpr, _ linea: Int) {
        self.op = op
        self.a = a
        self.b = b
        super.init(linea)
    }

    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        let x = try a.ev(m, f)
        // cin >> x, cout << x
        if case .canal(let c) = x, op == .shl || op == .shr {
            return try m.flujo(c, op, b, f, linea)
        }
        let y = try b.ev(m, f)
        // camino rápido: dos int
        if case .n(let p, .int) = x, case .n(let q, .int) = y {
            switch op {
            case .suma: return .n(Int64(Int32(truncatingIfNeeded: p &+ q)), .int)
            case .resta: return .n(Int64(Int32(truncatingIfNeeded: p &- q)), .int)
            case .mult: return .n(Int64(Int32(truncatingIfNeeded: p &* q)), .int)
            case .menor: return m.verdad(p < q)
            case .mayor: return m.verdad(p > q)
            case .menorIg: return m.verdad(p <= q)
            case .mayorIg: return m.verdad(p >= q)
            case .igual: return m.verdad(p == q)
            case .distinto: return m.verdad(p != q)
            default: break
            }
        }
        return try m.binaria(op, x, y, linea)
    }

    override func res(_ r: CResol) throws {
        try a.res(r)
        try b.res(r)
        tipo = r.tipoBinario(op, a.tipo, b.tipo)
    }
}

final class CLogico: CExpr {
    let esY: Bool
    let a: CExpr
    let b: CExpr
    init(_ esY: Bool, _ a: CExpr, _ b: CExpr, _ linea: Int) {
        self.esY = esY
        self.a = a
        self.b = b
        super.init(linea)
    }
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        let x = m.aBool(try a.ev(m, f))
        if esY && !x { return m.verdad(false) }
        if !esY && x { return m.verdad(true) }
        return m.verdad(m.aBool(try b.ev(m, f)))
    }
    override func res(_ r: CResol) throws {
        try a.res(r)
        try b.res(r)
        tipo = r.m.tipoVerdad
    }
}

final class CUn: CExpr {
    let op: String
    let a: CExpr
    init(_ op: String, _ a: CExpr, _ linea: Int) {
        self.op = op
        self.a = a
        super.init(linea)
    }
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        let x = try a.ev(m, f)
        return try m.unaria(op, x, linea)
    }
    override func res(_ r: CResol) throws {
        try a.res(r)
        if op == "!" { tipo = r.m.tipoVerdad; return }
        if let t = a.tipo?.sinRef, case .num(let k) = t { tipo = .num(CNum.comun(k, .int)) } else { tipo = a.tipo?.sinRef }
    }
}

final class CIncDec: CExpr {
    let pre: Bool
    let delta: Int64
    let a: CExpr
    init(pre: Bool, delta: Int64, _ a: CExpr, _ linea: Int) {
        self.pre = pre
        self.delta = delta
        self.a = a
        super.init(linea)
    }
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        let l = try a.lugar(m, f)
        let v = try m.lee(l)
        let nuevo: CV
        switch v {
        case .n(let x, let k): nuevo = .n(cAjusta(x &+ delta, k), k)
        case .d(let d, let fl): nuevo = .d(d + Double(delta), fl)
        case .p(let p): nuevo = .p(CPtr(mem: p.mem, i: p.i + Int(delta)))
        case .it(let b, let i): nuevo = .it(b, i + Int(delta))
        case .o:
            let nombre = delta > 0 ? "operator++" : "operator--"
            return try m.llamaOperador(nombre, v, [], linea) ?? v
        default:
            throw CFallo("no se puede usar ++/-- con \(v.tipoNombre)", linea)
        }
        try m.escribe(l, nuevo)
        return pre ? nuevo : v
    }
    override func res(_ r: CResol) throws {
        try a.res(r)
        guard a.asignable || !(a is CNombre) else { throw CFallo("++/-- necesita una variable", linea) }
        tipo = a.tipo?.sinRef
    }
}

final class CAsig: CExpr {
    let op: COp?
    let a: CExpr
    let b: CExpr
    var tipoDestino: CTipo?
    init(_ op: COp?, _ a: CExpr, _ b: CExpr, _ linea: Int) {
        self.op = op
        self.a = a
        self.b = b
        super.init(linea)
    }
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        // camino rápido: variable local = valor
        if op == nil, let n = a as? CNombre, case .local(0, let s) = n.modo {
            var v = try b.ev(m, f)
            if case .ref(let r) = f.s.a[s] {
                if let t = tipoDestino { v = m.convierte(v, t) }
                try m.escribe(r.l, m.copia(v))
                return v
            }
            if let t = tipoDestino { v = m.convierte(v, t) }
            v = m.copia(v)
            f.s.a[s] = v
            return v
        }
        let l = try a.lugar(m, f)
        var v: CV
        if let op = op {
            let viejo = try m.lee(l)
            if case .o = viejo, let r = try m.llamaOperador("operator" + op.texto + "=", viejo, [try b.ev(m, f)], linea) {
                return r
            }
            v = try m.binaria(op, viejo, try b.ev(m, f), linea)
            // x += 1.5 con x int: el resultado vuelve al tipo de x
            if tipoDestino == nil {
                if case .n(_, let k) = viejo { v = m.convierte(v, .num(k)) }
                else if case .d(_, let fl) = viejo { v = m.convierte(v, .real(fl)) }
            }
        } else {
            v = try b.ev(m, f)
            if case .o(let o) = (try? m.lee(l)) ?? .vacio, m.dialecto == .cpp,
               o.clase.metodos["operator="] != nil, let r = try m.llamaOperador("operator=", .o(o), [v], linea) {
                return r
            }
        }
        if let t = tipoDestino { v = m.convierte(v, t) }
        v = m.copia(v)
        try m.escribe(l, v)
        return v
    }
    override func res(_ r: CResol) throws {
        try a.res(r)
        try b.res(r)
        if let n = a as? CNombre, !n.asignable {
            throw CFallo("no se puede asignar a '\(n.nombre)'", linea)
        }
        tipoDestino = r.tipoConversion(a.tipo)
        tipo = a.tipo?.sinRef
    }
}

final class CTernario: CExpr {
    let c: CExpr
    let a: CExpr
    let b: CExpr
    init(_ c: CExpr, _ a: CExpr, _ b: CExpr, _ linea: Int) {
        self.c = c
        self.a = a
        self.b = b
        super.init(linea)
    }
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        let v = m.aBool(try c.ev(m, f)) ? try a.ev(m, f) : try b.ev(m, f)
        // a ? 1 : 2.5 → double en los dos casos
        if let t = tipo, case .real = t { return m.convierte(v, t) }
        return v
    }
    override func lugar(_ m: CMaq, _ f: CMarco) throws -> CLugar {
        m.aBool(try c.ev(m, f)) ? try a.lugar(m, f) : try b.lugar(m, f)
    }
    override var asignable: Bool { a.asignable && b.asignable }
    override func res(_ r: CResol) throws {
        try c.res(r)
        try a.res(r)
        try b.res(r)
        if let x = a.tipo?.sinRef, let y = b.tipo?.sinRef {
            if case .real = x { tipo = x; return }
            if case .real = y { tipo = y; return }
        }
        tipo = a.tipo ?? b.tipo
    }
}

final class CComa: CExpr {
    let a: CExpr
    let b: CExpr
    init(_ a: CExpr, _ b: CExpr, _ linea: Int) {
        self.a = a
        self.b = b
        super.init(linea)
    }
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        _ = try a.ev(m, f)
        return try b.ev(m, f)
    }
    override func res(_ r: CResol) throws {
        try a.res(r)
        try b.res(r)
        tipo = b.tipo
    }
}

// MARK: punteros, arreglos y miembros

final class CDeref: CExpr {
    let a: CExpr
    init(_ a: CExpr, _ linea: Int) {
        self.a = a
        super.init(linea)
    }
    override var asignable: Bool { true }
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        let v = try a.ev(m, f)
        if case .it(let b, let i) = v { return try m.leeIterador(b, i, linea) }
        if case .f = v { return v }
        if case .o(let o) = v, m.dialecto == .cpp {
            if let r = try m.llamaOperador("operator*", .o(o), [], linea) { return r }
            return v
        }
        return try m.lee(try lugarDe(v, m))
    }

    override func lugar(_ m: CMaq, _ f: CMarco) throws -> CLugar {
        try lugarDe(try a.ev(m, f), m)
    }

    func lugarDe(_ v: CV, _ m: CMaq) throws -> CLugar {
        switch v {
        case .p(let p):
            guard let mem = p.mem else { throw CFallo("se usó un puntero NULL (*p con p = NULL)", linea) }
            try m.prepara(mem, tipo, linea)
            guard p.i >= 0 && p.i < mem.a.count else {
                throw CFallo("puntero fuera del bloque de memoria (posición \(p.i), tamaño \(mem.a.count))", linea)
            }
            if mem.libre { throw CFallo("se usó memoria ya liberada con free/delete", linea) }
            return .celda(mem, p.i)
        case .it(let b, let i):
            return try m.lugarIterador(b, i, linea)
        default:
            throw CFallo("'*' necesita un puntero, no \(v.tipoNombre)", linea)
        }
    }
    override func res(_ r: CResol) throws {
        try a.res(r)
        tipo = a.tipo?.sinRef.elemento
    }
}

final class CDir: CExpr {
    let a: CExpr
    init(_ a: CExpr, _ linea: Int) {
        self.a = a
        super.init(linea)
    }
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        // &funcion: el puntero a función es la propia función
        if let n = a as? CNombre, case .funcion = n.modo { return try n.ev(m, f) }
        let l = try a.lugar(m, f)
        switch l {
        case .celda(let mem, let i):
            return .p(CPtr(mem: mem, i: i))
        default:
            // &v[i] de un vector: se copia a una celda propia
            let v = try m.lee(l)
            let mem = CMem([v])
            return .p(CPtr(mem: mem, i: 0))
        }
    }
    override func res(_ r: CResol) throws {
        try a.res(r)
        if let t = a.tipo { tipo = .ptr(t.sinRef) }
    }
}

final class CIndice: CExpr {
    let a: CExpr
    let i: CExpr
    init(_ a: CExpr, _ i: CExpr, _ linea: Int) {
        self.a = a
        self.i = i
        super.init(linea)
    }
    override var asignable: Bool { true }

    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        let base = try a.ev(m, f)
        switch base {
        case .p(let p):
            let k = try m.entero(try i.ev(m, f), linea)
            guard let mem = p.mem else { throw m.excepcion("NullPointerException", "índice sobre un puntero NULL", linea) }
            let j = p.i + Int(k)
            if j < 0 || j >= mem.a.count {
                if mem.crudo > 0 { try m.prepara(mem, tipo, linea); return try ev(m, f) }
                throw m.fueraDeRango(j, mem.a.count, linea)
            }
            let v = mem.a[j]
            if case .vacio = v { return m.valorInicial(tipo ?? .num(.int)) }
            return v
        case .s(let u):
            let k = Int(try m.entero(try i.ev(m, f), linea))
            guard k >= 0 && k <= u.count else { throw m.fueraDeRango(k, u.count, linea) }
            if k == u.count { return .n(0, .char) }
            return m.caracter(u[k])
        case .l(let l):
            let k = Int(try m.entero(try i.ev(m, f), linea))
            guard k >= 0 && k < l.a.count else { throw m.fueraDeRango(k, l.a.count, linea) }
            return l.a[k]
        case .m(let mp):
            return try m.lee(.mapa(mp, try i.ev(m, f)))
        case .o(let o):
            let idx = try i.ev(m, f)
            if let r = try m.llamaOperador("operator[]", .o(o), [idx], linea) {
                if case .ref(let rr) = r { return try m.lee(rr.l) }
                return r
            }
            throw CFallo("'[]' no está definido para \(o.clase.nombre) (falta operator[])", linea)
        case .nulo:
            throw m.excepcion("NullPointerException", "índice sobre un arreglo null", linea)
        default:
            throw CFallo("'[]' no se puede usar con \(base.tipoNombre)", linea)
        }
    }

    override func lugar(_ m: CMaq, _ f: CMarco) throws -> CLugar {
        let base: CV
        var lugarBase: CLugar?
        if a.asignable {
            let l = try a.lugar(m, f)
            lugarBase = l
            base = try m.lee(l)
        } else {
            base = try a.ev(m, f)
        }
        let idx = try i.ev(m, f)
        switch base {
        case .p(let p):
            guard let mem = p.mem else { throw m.excepcion("NullPointerException", "índice sobre un puntero NULL", linea) }
            let j = p.i + Int(try m.entero(idx, linea))
            if (j < 0 || j >= mem.a.count) && mem.crudo > 0 { try m.prepara(mem, tipo, linea) }
            guard j >= 0 && j < mem.a.count else { throw m.fueraDeRango(j, mem.a.count, linea) }
            return .celda(mem, j)
        case .l(let l):
            let j = Int(try m.entero(idx, linea))
            guard j >= 0 && j < l.a.count else { throw m.fueraDeRango(j, l.a.count, linea) }
            return .lista(l, j)
        case .m(let mp):
            return .mapa(mp, idx)
        case .s(let u):
            let j = Int(try m.entero(idx, linea))
            guard j >= 0 && j < u.count, let lb = lugarBase else { throw m.fueraDeRango(j, u.count, linea) }
            return .car(lb, j)
        case .o(let o):
            if let r = try m.llamaOperador("operator[]", .o(o), [idx], linea) {
                if case .ref(let rr) = r { return rr.l }
                return .celda(CMem([r]), 0)
            }
            throw CFallo("'[]' no está definido para \(o.clase.nombre) (falta operator[])", linea)
        case .nulo:
            throw m.excepcion("NullPointerException", "índice sobre un arreglo null", linea)
        default:
            throw CFallo("'[]' no se puede usar con \(base.tipoNombre)", linea)
        }
    }

    override func res(_ r: CResol) throws {
        try a.res(r)
        try i.res(r)
        if let t = a.tipo?.sinRef {
            switch t {
            case .ptr(let e), .arr(let e, _): tipo = e
            case .cad: tipo = .num(r.m.dialecto == .java ? .jchar : .char)
            case .cont(let n, let args):
                if n == "map" || n == "unordered_map" { tipo = args.count > 1 ? args[1] : nil }
                else { tipo = args.first }
            default: tipo = nil
            }
        }
    }
}

final class CMiembro: CExpr {
    let a: CExpr
    let nombre: String
    let flecha: Bool
    var cacheClase = -1
    var cacheIdx = 0
    /// Clase.campo estático (Java) o Clase::x (C++), resuelto al compilar
    var estatico: (CClase, Int)?
    var nativo: String?
    var claseEst: String?
    var estaticoConst: CV?

    init(_ a: CExpr, _ nombre: String, flecha: Bool, _ linea: Int) {
        self.a = a
        self.nombre = nombre
        self.flecha = flecha
        super.init(linea)
    }

    override var asignable: Bool { nativo == nil }

    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        if let (c, i) = estatico {
            try m.preparaEstaticos(c)
            return c.estaticos.a[i]
        }
        if let c = estaticoConst { return c }
        if let n = nativo { return try m.valorNativo(n, linea) }
        let base = try a.ev(m, f)
        if case .o(let o) = base, let i = indiceCampo(o) {
            var v = o.c.a[i]
            if case .ref(let r) = v { v = try m.lee(r.l) }
            return v
        }
        return try m.miembro(base, nombre, flecha, self, linea)
    }

    @inline(__always)
    func indiceCampo(_ o: CObj) -> Int? {
        if o.clase.id == cacheClase { return cacheIdx }
        guard let i = o.clase.indice[nombre] else { return nil }
        cacheClase = o.clase.id
        cacheIdx = i
        return i
    }

    override func lugar(_ m: CMaq, _ f: CMarco) throws -> CLugar {
        if let (c, i) = estatico {
            try m.preparaEstaticos(c)
            return .celda(c.estaticos, i)
        }
        var base = try a.ev(m, f)
        if flecha, case .p(let p) = base {
            guard let mem = p.mem else { throw CFallo("p->\(nombre) con p = NULL", linea) }
            try m.prepara(mem, a.tipo?.sinRef.elemento, linea)
            guard p.i >= 0 && p.i < mem.a.count else { throw CFallo("p->\(nombre) fuera de la memoria reservada", linea) }
            if mem.libre { throw CFallo("p->\(nombre) usa memoria ya liberada", linea) }
            base = mem.a[p.i]
            if case .o = base {} else {
                // memoria de malloc sin tipo todavía: se crea la estructura
                base = try m.objetoEnCelda(mem, p.i, a.tipo?.sinRef.elemento, nombre, linea)
            }
        }
        if case .it(let b, let i) = base, let mp = b.mapa {
            guard i >= 0 && i < mp.claves.count else { throw CFallo("iterador fuera del mapa", linea) }
            if nombre == "second" { return .mapa(mp, mp.vals[mp.claves[i]]!.0) }
        }
        if case .it(let b, let i) = base {
            base = try m.leeIterador(b, i, linea)
        }
        guard case .o(let o) = base else {
            if case .nulo = base { throw m.excepcion("NullPointerException", "acceso a '\(nombre)' de un objeto null", linea) }
            throw CFallo("'\(nombre)' no es un campo de \(base.tipoNombre)", linea)
        }
        guard let i = indiceCampo(o) else {
            throw CFallo("\(o.clase.nombre) no tiene el campo '\(nombre)'", linea)
        }
        if case .ref(let r) = o.c.a[i] { return r.l }
        return .celda(o.c, i)
    }

    override func res(_ r: CResol) throws {
        try r.resuelveMiembro(self)
    }
}

// MARK: llamadas

final class CLlamada: CExpr {
    let f: CExpr
    let args: [CExpr]
    /// resuelto: función global, método del propio objeto o nativa
    var destino: CDestino = .dinamico
    var plantilla: [CTipo] = []
    var claseCtx: CClase?

    init(_ f: CExpr, _ args: [CExpr], _ linea: Int) {
        self.f = f
        self.args = args
        super.init(linea)
    }

    override func ev(_ m: CMaq, _ marco: CMarco) throws -> CV {
        try m.revisaPaso()
        return try m.llamada(self, marco)
    }

    override var asignable: Bool { true }
    override func lugar(_ m: CMaq, _ marco: CMarco) throws -> CLugar {
        // v.at(i) = x, v.front() = x, mapa.get… : se devuelve una referencia
        let v = try m.llamada(self, marco, quiereLugar: true)
        if case .ref(let r) = v { return r.l }
        if case .p(let p) = v, let mem = p.mem, m.dialecto == .cpp, tipo?.esRef == true { return .celda(mem, p.i) }
        let mem = CMem([v])
        return .celda(mem, 0)
    }

    override func res(_ r: CResol) throws {
        try r.resuelveLlamada(self)
    }
}

enum CDestino {
    case dinamico
    case global([CFuncion])
    case propio(String)
    case estatico(CClase, String)
    case nativo(String)
    case superMetodo(String)
    case ctor(CClase)
    case superCtor
    case esteCtor
}

final class CCast: CExpr {
    let t: CTipo
    let a: CExpr
    init(_ t: CTipo, _ a: CExpr, _ linea: Int) {
        self.t = t
        self.a = a
        super.init(linea)
    }
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        let v = try a.ev(m, f)
        if case .clase(let n) = t, case .o(let o) = v, m.dialecto == .java, !o.clase.esSubclase(de: n), m.clases[n] != nil {
            throw m.excepcion("ClassCastException", "class \(o.clase.nombre) cannot be cast to class \(n)", linea)
        }
        if case .cad = t, m.dialecto == .cpp { return try m.aCadena(v, linea) }
        return m.convierte(v, t)
    }
    override func res(_ r: CResol) throws {
        try a.res(r)
        tipo = t
    }
}

final class CSizeof: CExpr {
    let t: CTipo?
    let a: CExpr?
    init(tipo: CTipo?, expr: CExpr?, _ linea: Int) {
        self.t = tipo
        self.a = expr
        super.init(linea)
    }
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        if let t = t { return .n(Int64(m.tamano(t)), .ulong) }
        if let e = a {
            if let tt = e.tipo { return .n(Int64(m.tamano(tt)), .ulong) }
            let v = try e.ev(m, f)
            return .n(Int64(m.tamanoValor(v)), .ulong)
        }
        return .n(0, .ulong)
    }
    override func res(_ r: CResol) throws {
        try a?.res(r)
        tipo = .num(.ulong)
    }
}

/// {1, 2, 3}: su forma final depende de dónde se use.
final class CListaIni: CExpr {
    let elems: [CExpr]
    /// .campo = valor (inicializadores designados de C)
    let nombres: [String?]
    init(_ elems: [CExpr], nombres: [String?], _ linea: Int) {
        self.elems = elems
        self.nombres = nombres
        super.init(linea)
    }
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        try m.construyeLista(self, tipo, f)
    }
    override func res(_ r: CResol) throws {
        for e in elems { try e.res(r) }
    }
}

final class CNuevo: CExpr {
    let t: CTipo
    let args: [CExpr]
    let dims: [CExpr]
    let ini: CListaIni?
    let anonima: CClase?
    /// cuántos [] lleva (new int[3][] → 2)
    var niveles = 0
    init(_ t: CTipo, args: [CExpr], dims: [CExpr], ini: CListaIni?, anonima: CClase?, _ linea: Int) {
        self.t = t
        self.args = args
        self.dims = dims
        self.ini = ini
        self.anonima = anonima
        super.init(linea)
    }
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        try m.nuevo(self, f)
    }
    override func res(_ r: CResol) throws {
        for e in args { try e.res(r) }
        for e in dims { try e.res(r) }
        if let i = ini {
            try i.res(r)
            var tt = t
            for _ in 0..<max(1, max(niveles, dims.count)) { tt = .arr(tt, -1) }
            i.tipo = dims.isEmpty && r.m.dialecto == .cpp ? .arr(t, -1) : tt
        }
        if let c = anonima { try r.resuelveClaseAnonima(c) }
        if r.m.dialecto == .java {
            if dims.isEmpty && ini == nil { tipo = t } else {
                var tt = t
                let n = max(max(dims.count, niveles), 1)
                for _ in 0..<n { tt = .arr(tt, -1) }
                tipo = tt
            }
        } else {
            tipo = .ptr(t)
        }
    }
}

final class CBorrar: CExpr {
    let a: CExpr
    init(_ a: CExpr, _ linea: Int) {
        self.a = a
        super.init(linea)
    }
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        let v = try a.ev(m, f)
        try m.libera(v, linea)
        return .vacio
    }
    override func res(_ r: CResol) throws { try a.res(r) }
}

/// Lambda de C++ ([&](int x){…}) o de Java (x -> x * 2).
final class CLambda: CExpr {
    let fn: CFuncion
    init(_ fn: CFuncion, _ linea: Int) {
        self.fn = fn
        super.init(linea)
    }
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        .f(CCierre(fn: fn, marco: f, este: f.este))
    }
    override func res(_ r: CResol) throws {
        try r.resuelveLambda(fn)
        tipo = .fn
    }
}

/// Clase::metodo de Java (System.out::println, Integer::compare, String::length…)
final class CRefMetodo: CExpr {
    let base: CExpr?
    let clase: String?
    let nombre: String
    init(base: CExpr?, clase: String?, nombre: String, _ linea: Int) {
        self.base = base
        self.clase = clase
        self.nombre = nombre
        super.init(linea)
    }
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        try m.refMetodo(self, f)
    }
    override func res(_ r: CResol) throws {
        try base?.res(r)
        tipo = .fn
    }
}

final class CInstancia: CExpr {
    let a: CExpr
    let t: CTipo
    let enlace: String?
    var slot = -1
    var nombreTipo = ""
    init(_ a: CExpr, _ t: CTipo, enlace: String?, _ linea: Int) {
        self.a = a
        self.t = t
        self.enlace = enlace
        super.init(linea)
    }
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        let v = try a.ev(m, f)
        let ok = m.esInstanciaJava(v, nombreTipo, t)
        if ok && slot >= 0 { f.s.a[slot] = v }
        return .n(ok ? 1 : 0, .bool)
    }
    override func res(_ r: CResol) throws {
        try a.res(r)
        if let n = enlace { slot = try r.declara(n, t, linea) }
        tipo = .num(.bool)
    }
}

/// Expresión switch de Java: switch (x) { case 1 -> "uno"; default -> "otro"; }
final class CSwitchExpr: CExpr {
    let e: CExpr
    let casos: [([CExpr], CSent)]
    let defecto: CSent?
    init(_ e: CExpr, _ casos: [([CExpr], CSent)], _ defecto: CSent?, _ linea: Int) {
        self.e = e
        self.casos = casos
        self.defecto = defecto
        super.init(linea)
    }
    override func ev(_ m: CMaq, _ f: CMarco) throws -> CV {
        let v = try e.ev(m, f)
        for (vals, s) in casos {
            for x in vals where m.igualesCaso(v, try x.ev(m, f)) {
                return try m.valorDeRama(s, f, linea)
            }
        }
        if let d = defecto { return try m.valorDeRama(d, f, linea) }
        throw CFallo("switch sin caso para el valor", linea)
    }
    override func res(_ r: CResol) throws {
        try e.res(r)
        for (vals, s) in casos {
            for x in vals { try r.resuelveCaso(x, e.tipo) }
            try s.res(r)
        }
        try defecto?.res(r)
    }
}
