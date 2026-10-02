import Foundation

// ============================================================
// MARK: - C, C++ y Java: operadores, comparación y texto
// ============================================================

extension CMaq {

    func binaria(_ op: COp, _ a: CV, _ b: CV, _ linea: Int) throws -> CV {
        var x = a
        var y = b
        if case .ref(let r) = x { x = try lee(r.l) }
        if case .ref(let r) = y { y = try lee(r.l) }
        switch (x, y) {
        case (.n(let p, let kp), .n(let q, let kq)):
            return try binEnteros(op, p, kp, q, kq, linea)
        case (.n, .d), (.d, .n), (.d, .d):
            return try binReales(op, x, y, linea)
        default:
            return try binOtros(op, x, y, linea)
        }
    }

    func binEnteros(_ op: COp, _ p: Int64, _ kp: CNum, _ q: Int64, _ kq: CNum, _ linea: Int) throws -> CV {
        if op == .shl || op == .shr || op == .ushr {
            let k = CNum.comun(kp, .int)
            let bits: Int64 = k.rango == 4 ? 63 : 31
            let s = q & bits
            switch op {
            case .shl: return .n(cAjusta(p << s, k), k)
            case .shr:
                if k == .ulong { return .n(Int64(bitPattern: UInt64(bitPattern: p) >> UInt64(s)), k) }
                return .n(cAjusta(p, k) >> s, k)
            default:
                if k.rango == 4 { return .n(Int64(bitPattern: UInt64(bitPattern: p) >> UInt64(s)), k) }
                return .n(cAjusta(Int64(UInt32(truncatingIfNeeded: p) >> UInt32(s)), k), k)
            }
        }
        // booleanos de Java/C++ con & | ^
        if kp == .bool && kq == .bool && (op == .y || op == .o || op == .xor) {
            let r: Bool
            switch op {
            case .y: r = p != 0 && q != 0
            case .o: r = p != 0 || q != 0
            default: r = (p != 0) != (q != 0)
            }
            return .n(r ? 1 : 0, .bool)
        }
        let k = CNum.comun(kp, kq)
        if op.esComparacion {
            if !k.conSigno {
                let x = UInt64(bitPattern: cAjusta(p, k))
                let y = UInt64(bitPattern: cAjusta(q, k))
                return verdad(CMaq.compara(op, x < y, x == y))
            }
            return verdad(CMaq.compara(op, p < q, p == q))
        }
        switch op {
        case .suma: return .n(cAjusta(p &+ q, k), k)
        case .resta: return .n(cAjusta(p &- q, k), k)
        case .mult: return .n(cAjusta(p &* q, k), k)
        case .div, .mod:
            if q == 0 {
                if dialecto == .java { throw excepcion("ArithmeticException", "/ by zero", linea) }
                throw CFallo("división entre cero", linea)
            }
            if !k.conSigno {
                let x = UInt64(bitPattern: cAjusta(p, k))
                let y = UInt64(bitPattern: cAjusta(q, k))
                let r = op == .div ? x / y : x % y
                return .n(cAjusta(Int64(bitPattern: r), k), k)
            }
            let (r, desborda) = op == .div ? p.dividedReportingOverflow(by: q) : p.remainderReportingOverflow(dividingBy: q)
            return .n(cAjusta(desborda ? (op == .div ? p : 0) : r, k), k)
        case .y: return .n(cAjusta(p & q, k), k)
        case .o: return .n(cAjusta(p | q, k), k)
        case .xor: return .n(cAjusta(p ^ q, k), k)
        default: return .n(0, k)
        }
    }

    static func compara(_ op: COp, _ menor: Bool, _ igual: Bool) -> Bool {
        switch op {
        case .menor: return menor
        case .mayor: return !menor && !igual
        case .menorIg: return menor || igual
        case .mayorIg: return !menor
        case .igual: return igual
        default: return !igual
        }
    }

    func binReales(_ op: COp, _ x: CV, _ y: CV, _ linea: Int) throws -> CV {
        let a = try real(x, linea)
        let b = try real(y, linea)
        var esFloat = true
        if case .d(_, false) = x { esFloat = false }
        if case .d(_, false) = y { esFloat = false }
        if op.esComparacion {
            if op == .igual { return verdad(a == b) }
            if op == .distinto { return verdad(a != b) }
            return verdad(CMaq.compara(op, a < b, a == b))
        }
        var r: Double
        switch op {
        case .suma: r = a + b
        case .resta: r = a - b
        case .mult: r = a * b
        case .div: r = a / b
        case .mod: r = fmod(a, b)
        default: throw CFallo("'\(op.texto)' no se puede usar con números con decimales", linea)
        }
        if esFloat { r = Double(Float(r)) }
        return .d(r, esFloat)
    }

    func binOtros(_ op: COp, _ x: CV, _ y: CV, _ linea: Int) throws -> CV {
        switch (x, y) {
        case (.p(let p), .n(let q, _)):
            if op == .suma { return .p(CPtr(mem: p.mem, i: p.i + Int(q))) }
            if op == .resta { return .p(CPtr(mem: p.mem, i: p.i - Int(q))) }
            if op == .igual || op == .distinto { return verdad((p.mem == nil && q == 0) == (op == .igual)) }
        case (.n(let q, _), .p(let p)):
            if op == .suma { return .p(CPtr(mem: p.mem, i: p.i + Int(q))) }
            if op == .igual || op == .distinto { return verdad((p.mem == nil && q == 0) == (op == .igual)) }
        case (.p(let p), .p(let q)):
            if op == .resta { return .n(Int64(p.i - q.i), .long) }
            let mismo = p.mem === q.mem
            if op == .igual { return verdad(mismo && (p.i == q.i || p.mem == nil)) }
            if op == .distinto { return verdad(!(mismo && (p.i == q.i || p.mem == nil))) }
            if op.esComparacion { return verdad(CMaq.compara(op, p.i < q.i, p.i == q.i)) }
        case (.it(let b1, let i), .n(let q, _)):
            if op == .suma { return .it(b1, i + Int(q)) }
            if op == .resta { return .it(b1, i - Int(q)) }
        case (.n(let q, _), .it(let b1, let i)):
            if op == .suma { return .it(b1, i + Int(q)) }
        case (.it(_, let i), .it(_, let j)):
            if op == .resta { return .n(Int64(i - j), .long) }
            if op.esComparacion { return verdad(CMaq.compara(op, i < j, i == j)) }
        default:
            break
        }
        if case .s = x { return try binCadena(op, x, y, linea) }
        if case .s = y { return try binCadena(op, x, y, linea) }
        if op == .igual || op == .distinto {
            if x.esNulo || y.esNulo {
                let r = x.esNulo && y.esNulo
                return verdad(op == .igual ? r : !r)
            }
        }
        if case .o = x, let r = try llamaOperadorBinario(op, x, y, linea) { return r }
        if case .o = y, let r = try llamaOperadorBinario(op, x, y, linea) { return r }
        if op == .igual || op == .distinto {
            let r = try igualesIdentidad(x, y, linea)
            return verdad(op == .igual ? r : !r)
        }
        if op.esComparacion, dialecto == .cpp {
            let c = try compara(x, y, linea)
            return verdad(CMaq.compara(op, c < 0, c == 0))
        }
        throw CFallo("'\(op.texto)' no se puede usar entre \(x.tipoNombre) y \(y.tipoNombre)", linea)
    }

    func binCadena(_ op: COp, _ x: CV, _ y: CV, _ linea: Int) throws -> CV {
        if op == .suma {
            if dialecto == .java {
                return .s(unidades(try texto(x, linea)) + unidades(try texto(y, linea)))
            }
            return .s(try unidadesDe(x, linea) + (try unidadesDe(y, linea)))
        }
        if op.esComparacion {
            let a = try unidadesDe(x, linea)
            let b = try unidadesDe(y, linea)
            if case .nulo = x, dialecto == .java { return verdad(op == .distinto) }
            if case .nulo = y, dialecto == .java { return verdad(op == .distinto) }
            return verdad(CMaq.compara(op, a.lexicographicallyPrecedes(b), a == b))
        }
        throw CFallo("'\(op.texto)' no se puede usar con cadenas", linea)
    }

    /// Unidades de texto de un valor que se suma a un string de C++.
    func unidadesDe(_ v: CV, _ linea: Int) throws -> [UInt16] {
        switch v {
        case .s(let u): return u
        case .p: return unidades(try cadenaC(v, linea))
        case .n(let x, let k):
            if k == .char || k == .uchar || k == .jchar { return [UInt16(truncatingIfNeeded: k == .jchar ? x : Int64(UInt8(truncatingIfNeeded: x)))] }
            if dialecto == .cpp { throw CFallo("no se puede sumar un número a un string (usa to_string)", linea) }
            return unidades(try texto(v, linea))
        case .nulo: return unidades("null")
        default: return unidades(try texto(v, linea))
        }
    }

    func unaria(_ op: String, _ v: CV, _ linea: Int) throws -> CV {
        var x = v
        if case .ref(let r) = x { x = try lee(r.l) }
        switch op {
        case "!":
            if case .o = x, let r = try llamaOperador("operator!", x, [], linea) { return r }
            return verdad(!aBool(x))
        case "-":
            switch x {
            case .n(let a, let k):
                let r = CNum.comun(k, .int)
                return .n(cAjusta(0 &- a, r), r)
            case .d(let d, let f): return .d(-d, f)
            case .o:
                if let r = try llamaOperador("operator-", x, [], linea) { return r }
            default: break
            }
        case "+":
            if case .n(let a, let k) = x { let r = CNum.comun(k, .int); return .n(cAjusta(a, r), r) }
            return x
        case "~":
            if case .n(let a, let k) = x {
                if k == .bool { return verdad(a == 0) }
                let r = CNum.comun(k, .int)
                return .n(cAjusta(~a, r), r)
            }
        default:
            break
        }
        throw CFallo("'\(op)' no se puede usar con \(x.tipoNombre)", linea)
    }

    // MARK: comparación (sort, map, max…)

    func compara(_ a: CV, _ b: CV, _ linea: Int) throws -> Int {
        var x = a
        var y = b
        if case .ref(let r) = x { x = try lee(r.l) }
        if case .ref(let r) = y { y = try lee(r.l) }
        switch (x, y) {
        case (.n(let p, let kp), .n(let q, let kq)):
            let k = CNum.comun(kp, kq)
            if !k.conSigno {
                let u = UInt64(bitPattern: p)
                let w = UInt64(bitPattern: q)
                return u < w ? -1 : (u == w ? 0 : 1)
            }
            return p < q ? -1 : (p == q ? 0 : 1)
        case (.n, .d), (.d, .n), (.d, .d):
            let p = try real(x, linea)
            let q = try real(y, linea)
            return p < q ? -1 : (p == q ? 0 : 1)
        case (.s(let p), .s(let q)):
            if p == q { return 0 }
            if dialecto == .java {
                // compareTo de Java: diferencia del primer carácter distinto
                let n = min(p.count, q.count)
                for k in 0..<n where p[k] != q[k] { return Int(p[k]) - Int(q[k]) }
                return p.count - q.count
            }
            return p.lexicographicallyPrecedes(q) ? -1 : 1
        case (.p, .p), (.s, .p), (.p, .s):
            let p = unidades(try cadenaC(x, linea))
            let q = unidades(try cadenaC(y, linea))
            return p == q ? 0 : (p.lexicographicallyPrecedes(q) ? -1 : 1)
        case (.o(let o1), .o(let o2)):
            if o1.clase.nombre == "pair" || o1.clase.nombre == "$tupla" {
                for k in 0..<min(o1.c.a.count, o2.c.a.count) {
                    let c = try compara(o1.c.a[k], o2.c.a[k], linea)
                    if c != 0 { return c }
                }
                return 0
            }
            if dialecto == .java {
                if let fn = o1.clase.metodo("compareTo", 1) {
                    return Int(try entero(try llama(fn, [y], este: o1, padre: o1.exterior, linea: linea), linea))
                }
                if o1.clase.esEnum, let i = o1.clase.indice["$ordinal"] {
                    return Int(try entero(o1.c.a[i], linea) - (try entero(o2.c.a[i], linea)))
                }
                throw excepcion("ClassCastException", "class \(o1.clase.nombre) cannot be cast to class java.lang.Comparable", linea)
            }
            if let r = try llamaOperadorBinario(.menor, x, y, linea) {
                if aBool(r) { return -1 }
                if let r2 = try llamaOperadorBinario(.menor, y, x, linea), aBool(r2) { return 1 }
                return 0
            }
            throw CFallo("no se pueden comparar dos \(o1.clase.nombre) (falta operator<)", linea)
        case (.l(let l1), .l(let l2)):
            for k in 0..<min(l1.a.count, l2.a.count) {
                let c = try compara(l1.a[k], l2.a[k], linea)
                if c != 0 { return c }
            }
            return l1.a.count - l2.a.count
        case (.nulo, .nulo):
            return 0
        default:
            if x.esNulo || y.esNulo { throw excepcion("NullPointerException", "comparación con null", linea) }
            throw CFallo("no se pueden comparar \(x.tipoNombre) y \(y.tipoNombre)", linea)
        }
    }

    /// equals de Java / == de C++ en contenedores (contains, indexOf, find…).
    func iguales(_ a: CV, _ b: CV, _ linea: Int) throws -> Bool {
        var x = a
        var y = b
        if case .ref(let r) = x { x = try lee(r.l) }
        if case .ref(let r) = y { y = try lee(r.l) }
        switch (x, y) {
        case (.n, .n), (.n, .d), (.d, .n), (.d, .d), (.s, .s):
            return try compara(x, y, linea) == 0
        case (.o(let o), _):
            if dialecto == .java, let fn = o.clase.metodo("equals", 1) {
                return aBool(try llama(fn, [y], este: o, padre: o.exterior, linea: linea))
            }
            if case .o(let o2) = y {
                if o === o2 { return true }
                if o.clase.nombre == "pair" || o.clase.nombre == "$tupla" { return try compara(x, y, linea) == 0 }
                if dialecto == .cpp, let r = try llamaOperadorBinario(.igual, x, y, linea) { return aBool(r) }
                if dialecto != .java && o.clase === o2.clase {
                    for k in 0..<o.c.a.count where !(try iguales(o.c.a[k], o2.c.a[k], linea)) { return false }
                    return true
                }
            }
            return false
        case (.l(let l1), .l(let l2)):
            if l1 === l2 { return true }
            guard l1.a.count == l2.a.count else { return false }
            for k in 0..<l1.a.count where !(try iguales(l1.a[k], l2.a[k], linea)) { return false }
            return true
        case (.m(let m1), .m(let m2)):
            if m1 === m2 { return true }
            guard m1.claves.count == m2.claves.count else { return false }
            for k in m1.claves {
                guard let v1 = m1.vals[k], let v2 = m2.vals[k] else { return false }
                if !(try iguales(v1.1, v2.1, linea)) { return false }
            }
            return true
        default:
            return try igualesIdentidad(x, y, linea)
        }
    }

    func igualesIdentidad(_ x: CV, _ y: CV, _ linea: Int) throws -> Bool {
        switch (x, y) {
        case (.o(let a), .o(let b)): return a === b
        case (.l(let a), .l(let b)): return dialecto == .cpp ? try iguales(x, y, linea) : a === b
        case (.m(let a), .m(let b)): return dialecto == .cpp ? try iguales(x, y, linea) : a === b
        case (.nulo, .nulo): return true
        case (.f(let a), .f(let b)): return a === b
        case (.canal(let a), .canal(let b)): return a === b
        case (.it(let a, let i), .it(let b, let j)): return i == j && (a === b || a.lista === b.lista && a.lista != nil || a.mapa === b.mapa && a.mapa != nil || a.mem === b.mem && a.mem != nil || a.cad != nil && b.cad != nil)
        case (.n, .n), (.d, .d), (.n, .d), (.d, .n): return try compara(x, y, linea) == 0
        default: return false
        }
    }

    func igualesCaso(_ v: CV, _ c: CV) -> Bool {
        switch (v, c) {
        case (.n(let a, _), .n(let b, _)): return a == b
        case (.s(let a), .s(let b)): return a == b
        case (.o(let a), .o(let b)): return a === b
        case (.d(let a, _), .d(let b, _)): return a == b
        default: return false
        }
    }

    // MARK: texto de un valor (println, cout, concatenación)

    func texto(_ v: CV, _ linea: Int) throws -> String {
        switch v {
        case .n(let x, let k):
            switch k {
            case .bool: return dialecto == .java ? (x != 0 ? "true" : "false") : (x != 0 ? "1" : "0")
            case .char, .uchar:
                if dialecto == .java { return String(x) }
                return String(decoding: [UInt8(truncatingIfNeeded: x)], as: UTF8.self)
            case .jchar: return String(decoding: [UInt16(truncatingIfNeeded: x)], as: UTF16.self)
            case .ulong: return String(UInt64(bitPattern: x))
            default: return String(x)
            }
        case .d(let d, let f):
            if dialecto == .java { return CMaq.javaReal(d, f) }
            return CMaq.formatoG(d, 6)
        case .s(let u):
            return texto(u)
        case .p(let p):
            guard let mem = p.mem else { return dialecto == .java ? "null" : "0" }
            if case .num(.char)? = mem.elem { return try cadenaC(v, linea) }
            if p.i < mem.a.count, case .n(_, .char) = mem.a[p.i] { return try cadenaC(v, linea) }
            if dialecto == .java {
                let tipo = mem.elem.map { CMaq.firmaJava($0) } ?? "Ljava.lang.Object;"
                return "[" + tipo + "@" + String(UInt(bitPattern: ObjectIdentifier(mem).hashValue) & 0xFFFFFFF, radix: 16)
            }
            return "0x" + String(UInt(bitPattern: ObjectIdentifier(mem).hashValue) & 0xFFFFFFFFFF, radix: 16)
        case .nulo:
            return "null"
        case .o(let o):
            return try textoObjeto(o, linea)
        case .l(let l):
            switch l.k {
            case "StringBuilder", "File", "AtomicInteger":
                return try texto(l.a.first ?? .vacio, linea)
            case "Optional":
                return l.a.isEmpty ? "Optional.empty" : "Optional[" + (try texto(l.a[0], linea)) + "]"
            case "Random", "Thread", "Stream":
                return "java.util." + l.k + "@" + String(UInt(bitPattern: ObjectIdentifier(l).hashValue) & 0xFFFFFF, radix: 16)
            default:
                break
            }
            var partes: [String] = []
            for x in l.a { partes.append(try texto(x, linea)) }
            return "[" + partes.joined(separator: ", ") + "]"
        case .m(let mp):
            var partes: [String] = []
            for k in mp.claves {
                guard let (kv, vv) = mp.vals[k] else { continue }
                if mp.esSet { partes.append(try texto(kv, linea)) }
                else { partes.append(try texto(kv, linea) + "=" + (try texto(vv, linea))) }
            }
            return mp.esSet ? "[" + partes.joined(separator: ", ") + "]" : "{" + partes.joined(separator: ", ") + "}"
        case .f(let c):
            return dialecto == .java ? "Main$$Lambda@" + String(UInt(bitPattern: ObjectIdentifier(c).hashValue) & 0xFFFFFF, radix: 16) : "1"
        case .it: return "iterador"
        case .canal: return "stream"
        case .ref(let r): return try texto(try lee(r.l), linea)
        case .vacio: return ""
        }
    }

    static func firmaJava(_ t: CTipo) -> String {
        switch t {
        case .num(.int): return "I"
        case .num(.long): return "J"
        case .num(.jchar): return "C"
        case .num(.bool): return "Z"
        case .num(.char): return "B"
        case .num(.short): return "S"
        case .real(true): return "F"
        case .real(false): return "D"
        case .cad: return "Ljava.lang.String;"
        case .arr(let e, _): return "[" + firmaJava(e)
        case .clase(let n): return "L" + n + ";"
        default: return "Ljava.lang.Object;"
        }
    }

    func textoObjeto(_ o: CObj, _ linea: Int) throws -> String {
        if let fn = o.clase.metodo("toString", 0) {
            return try texto(try llama(fn, [], este: o, padre: o.exterior, linea: linea), linea)
        }
        if o.clase.esEnum, let i = o.clase.indice["$nombre"] { return try texto(o.c.a[i], linea) }
        if o.clase.nombre == "pair" {
            let a = try texto(o.c.a[0], linea)
            let b = try texto(o.c.a[1], linea)
            return dialecto == .java ? a + "=" + b : "(" + a + ", " + b + ")"
        }
        if o.clase.nombre == "$tupla" {
            return "(" + (try o.c.a.map { try texto($0, linea) }).joined(separator: ", ") + ")"
        }
        if dialecto == .java {
            // record: Punto[x=1, y=2]
            if o.clase.ctors.count == 1, o.clase.metodos.count >= o.clase.campos.count, !o.clase.campos.isEmpty,
               o.clase.campos.allSatisfy({ o.clase.metodos[$0.nombre] != nil }) {
                var partes: [String] = []
                for c in o.clase.campos where !c.estatico {
                    if let i = o.clase.indice[c.nombre] { partes.append(c.nombre + "=" + (try texto(o.c.a[i], linea))) }
                }
                return o.clase.nombre + "[" + partes.joined(separator: ", ") + "]"
            }
            return o.clase.nombre + "@" + String(hashObjeto(o), radix: 16)
        }
        return o.clase.nombre
    }

    func hashObjeto(_ o: CObj) -> Int {
        Int(truncatingIfNeeded: UInt(bitPattern: ObjectIdentifier(o).hashValue) % 0x7FFFFFFF)
    }

    /// Double.toString de Java: 3.0, 0.1, 1.0E10, 1.234E-5
    static func javaReal(_ d: Double, _ esFloat: Bool) -> String {
        if d.isNaN { return "NaN" }
        if d.isInfinite { return d < 0 ? "-Infinity" : "Infinity" }
        if d == 0 { return d.sign == .minus ? "-0.0" : "0.0" }
        let base = esFloat ? "\(Float(d))" : "\(d)"
        var (digitos, exp, negativo) = CMaq.digitosDe(base)
        let a = abs(d)
        if a >= 1e-3 && a < 1e7 {
            // forma normal
            var entera = ""
            var frac = ""
            if exp >= 0 {
                let n = exp + 1
                while digitos.count < n { digitos += "0" }
                entera = String(digitos.prefix(n))
                frac = String(digitos.dropFirst(n))
            } else {
                entera = "0"
                frac = String(repeating: "0", count: -exp - 1) + digitos
            }
            if frac.isEmpty { frac = "0" }
            return (negativo ? "-" : "") + entera + "." + frac
        }
        let primero = String(digitos.prefix(1))
        var resto = String(digitos.dropFirst())
        if resto.isEmpty { resto = "0" }
        return (negativo ? "-" : "") + primero + "." + resto + "E" + String(exp)
    }

    /// "1.25e-07" o "1234.5" → ("125", -7) / ("12345", 3)
    static func digitosDe(_ s: String) -> (String, Int, Bool) {
        var t = s
        var neg = false
        if t.hasPrefix("-") { neg = true; t.removeFirst() }
        var exp = 0
        if let e = t.firstIndex(where: { $0 == "e" || $0 == "E" }) {
            exp = Int(t[t.index(after: e)...]) ?? 0
            t = String(t[..<e])
        }
        var entera = t
        var frac = ""
        if let p = t.firstIndex(of: ".") {
            entera = String(t[..<p])
            frac = String(t[t.index(after: p)...])
        }
        var digitos = entera + frac
        var e10 = exp + entera.count - 1
        while digitos.hasPrefix("0") && digitos.count > 1 { digitos.removeFirst(); e10 -= 1 }
        while digitos.hasSuffix("0") && digitos.count > 1 { digitos.removeLast() }
        return (digitos, e10, neg)
    }

    /// %g de C con precisión p (lo que muestra cout por defecto)
    static func formatoG(_ d: Double, _ p: Int) -> String {
        if d.isNaN { return d.sign == .minus ? "-nan" : "nan" }
        if d.isInfinite { return d < 0 ? "-inf" : "inf" }
        return String(format: "%.\(max(1, p))g", d)
    }

    func aCadena(_ v: CV, _ linea: Int) throws -> CV {
        switch v {
        case .s: return v
        case .p: return cadena(try cadenaC(v, linea))
        case .n(_, let k) where k == .char || k == .uchar:
            return .s(try unidadesDe(v, linea))
        default:
            return cadena(try texto(v, linea))
        }
    }
}
