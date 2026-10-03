import Foundation

// Grande.swift — números enteros de cualquier tamaño, exactos.
// Con decimales normales, 7436…(100 cifras) + 5658… sale "8.002e+112" y se
// pierden las cifras. Con esto la suma, la resta, la multiplicación, la
// división exacta y las potencias dan el número entero, cifra por cifra.

struct Grande: Equatable {
    var negativo = false
    var d: [Int] = []          // trozos de 9 cifras, el menos importante primero; cero = []

    static let base = 1_000_000_000

    init() {}

    init(_ v: Int) {
        negativo = v < 0
        var x = v.magnitude
        while x > 0 {
            d.append(Int(x % UInt(Grande.base)))
            x /= UInt(Grande.base)
        }
    }

    init?(_ s: String) {
        var t = Substring(s)
        if t.first == "-" { negativo = true; t = t.dropFirst() }
        if t.isEmpty || !t.allSatisfy({ $0.isASCII && $0.isNumber }) { return nil }
        var fin = t.endIndex
        while fin > t.startIndex {
            let ini = t.index(fin, offsetBy: -9, limitedBy: t.startIndex) ?? t.startIndex
            d.append(Int(t[ini ..< fin]) ?? 0)
            fin = ini
        }
        limpia()
    }

    var esCero: Bool { d.isEmpty }

    var texto: String {
        if d.isEmpty { return "0" }
        var s = String(d[d.count - 1])
        for i in stride(from: d.count - 2, through: 0, by: -1) {
            let trozo = String(d[i])
            s += String(repeating: "0", count: 9 - trozo.count) + trozo
        }
        return (negativo ? "-" : "") + s
    }

    var cifras: Int { texto.count }

    private mutating func limpia() {
        while let u = d.last, u == 0 { d.removeLast() }
        if d.isEmpty { negativo = false }
    }

    // comparar y operar sin signo

    private static func compara(_ a: [Int], _ b: [Int]) -> Int {
        if a.count != b.count { return a.count < b.count ? -1 : 1 }
        for i in stride(from: a.count - 1, through: 0, by: -1) where a[i] != b[i] {
            return a[i] < b[i] ? -1 : 1
        }
        return 0
    }

    private static func suma(_ a: [Int], _ b: [Int]) -> [Int] {
        var r: [Int] = []
        var lleva = 0
        for i in 0 ..< max(a.count, b.count) {
            let s = (i < a.count ? a[i] : 0) + (i < b.count ? b[i] : 0) + lleva
            r.append(s % base)
            lleva = s / base
        }
        if lleva > 0 { r.append(lleva) }
        return r
    }

    /// a - b con a ≥ b
    private static func resta(_ a: [Int], _ b: [Int]) -> [Int] {
        var r: [Int] = []
        var debe = 0
        for i in 0 ..< a.count {
            var s = a[i] - debe - (i < b.count ? b[i] : 0)
            debe = 0
            if s < 0 { s += base; debe = 1 }
            r.append(s)
        }
        while let u = r.last, u == 0 { r.removeLast() }
        return r
    }

    private static func multiplica(_ a: [Int], _ b: [Int]) -> [Int] {
        if a.isEmpty || b.isEmpty { return [] }
        var r = Array(repeating: 0, count: a.count + b.count)
        for i in 0 ..< a.count {
            var lleva = 0
            for j in 0 ..< b.count {
                let s = r[i + j] + a[i] * b[j] + lleva
                r[i + j] = s % base
                lleva = s / base
            }
            var k = i + b.count
            while lleva > 0 {
                let s = r[k] + lleva
                r[k] = s % base
                lleva = s / base
                k += 1
            }
        }
        while let u = r.last, u == 0 { r.removeLast() }
        return r
    }

    private static func porPequeño(_ a: [Int], _ m: Int) -> [Int] {
        return multiplica(a, m == 0 ? [] : [m])
    }

    /// División larga: (cociente, resto) sin signo.
    private static func divide(_ a: [Int], _ b: [Int]) -> ([Int], [Int]) {
        if compara(a, b) < 0 { return ([], a) }
        var q = Array(repeating: 0, count: a.count)
        var resto: [Int] = []
        for i in stride(from: a.count - 1, through: 0, by: -1) {
            resto.insert(a[i], at: 0)
            while let u = resto.last, u == 0 { resto.removeLast() }
            // la cifra del cociente: búsqueda binaria entre 0 y base-1
            var bajo = 0, alto = base - 1
            while bajo < alto {
                let medio = (bajo + alto + 1) / 2
                if compara(porPequeño(b, medio), resto) <= 0 { bajo = medio } else { alto = medio - 1 }
            }
            q[i] = bajo
            if bajo > 0 { resto = resta(resto, porPequeño(b, bajo)) }
        }
        while let u = q.last, u == 0 { q.removeLast() }
        return (q, resto)
    }

    // con signo

    static func + (x: Grande, y: Grande) -> Grande {
        var r = Grande()
        if x.negativo == y.negativo {
            r.d = suma(x.d, y.d)
            r.negativo = x.negativo
        } else if compara(x.d, y.d) >= 0 {
            r.d = resta(x.d, y.d)
            r.negativo = x.negativo
        } else {
            r.d = resta(y.d, x.d)
            r.negativo = y.negativo
        }
        r.limpia()
        return r
    }

    static prefix func - (x: Grande) -> Grande {
        var r = x
        if !r.esCero { r.negativo.toggle() }
        return r
    }

    static func - (x: Grande, y: Grande) -> Grande { x + (-y) }

    static func * (x: Grande, y: Grande) -> Grande {
        var r = Grande()
        r.d = multiplica(x.d, y.d)
        r.negativo = x.negativo != y.negativo
        r.limpia()
        return r
    }

    /// Solo si la división es exacta (si no, nil).
    static func divideExacto(_ x: Grande, _ y: Grande) -> Grande? {
        if y.esCero { return nil }
        let (q, resto) = divide(x.d, y.d)
        if !resto.isEmpty { return nil }
        var r = Grande()
        r.d = q
        r.negativo = x.negativo != y.negativo
        r.limpia()
        return r
    }

    func elevado(_ e: Int) -> Grande? {
        if e < 0 || e > 5000 { return nil }
        var r = Grande(1)
        var b = self
        var k = e
        while k > 0 {
            if k & 1 == 1 { r = r * b }
            k >>= 1
            if k > 0 { b = b * b }
            if r.d.count > 1200 || b.d.count > 1200 { return nil }      // más de ~10 000 cifras
        }
        return r
    }

    /// Para comparar con números normales.
    var doble: Double { Double(texto) ?? .nan }
}

extension Aritmetica {
    /// Calcula con enteros exactos (+ − × ÷ exacta, potencias ^, paréntesis).
    /// nil si hay decimales o la división no es exacta.
    static func calculaExacto(_ t: String) -> Grande? {
        let e = expresion(t)
        if e.isEmpty || !e.contains(where: { $0.isNumber }) || e.contains(".") { return nil }
        let permitidos = Set("0123456789+-*/^() ")
        if !e.allSatisfy({ permitidos.contains($0) }) { return nil }
        var p = ParserGrande(Array(e.filter { $0 != " " }))
        guard let v = p.suma(), p.i == p.c.count else { return nil }
        return v
    }

    /// El resultado como texto: exacto si se puede; si no, con decimales.
    static func resultado(_ t: String) -> String? {
        if let g = calculaExacto(t) { return g.texto }
        if let v = calcula(t) { return bonito(v) }
        return nil
    }

    /// ¿Hay alguna operación? ("67" solo no es una cuenta)
    static func opera(_ t: String) -> Bool {
        return expresion(t).contains { "+-*/^".contains($0) }
    }

    struct ParserGrande {
        let c: [Character]
        var i = 0
        init(_ c: [Character]) { self.c = c }

        mutating func suma() -> Grande? {
            guard var v = producto() else { return nil }
            while i < c.count, c[i] == "+" || c[i] == "-" {
                let op = c[i]
                i += 1
                guard let w = producto() else { return nil }
                v = op == "+" ? v + w : v - w
            }
            return v
        }

        mutating func producto() -> Grande? {
            guard var v = potencia() else { return nil }
            while i < c.count, c[i] == "*" || c[i] == "/" {
                let op = c[i]
                i += 1
                guard let w = potencia() else { return nil }
                if op == "*" {
                    v = v * w
                } else {
                    guard let q = Grande.divideExacto(v, w) else { return nil }
                    v = q
                }
            }
            return v
        }

        mutating func potencia() -> Grande? {
            guard let b = factor() else { return nil }
            if i < c.count, c[i] == "^" {
                i += 1
                guard let e = potencia(), let n = Int(e.texto) else { return nil }
                return b.elevado(n)
            }
            return b
        }

        mutating func factor() -> Grande? {
            if i < c.count, c[i] == "-" {
                i += 1
                return factor().map { -$0 }
            }
            if i < c.count, c[i] == "(" {
                i += 1
                let v = suma()
                if i < c.count, c[i] == ")" { i += 1 } else { return nil }
                return v
            }
            var s = ""
            while i < c.count, c[i].isNumber {
                s.append(c[i])
                i += 1
            }
            return Grande(s)
        }
    }
}
