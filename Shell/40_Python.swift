import Foundation

// ============================================================
// MARK: - Python (subconjunto) — intérprete escrito en Swift
// ============================================================
// No usa JavaScript ni nada de fuera: funciona aunque el motor JS
// esté apagado, en el iPad, en las shells de las IAs y en Linux.
//
//   python archivo.py [args]     python -c "print(1+1)"
//   python                       modo interactivo ('exit()' o 'salir')
//   ./archivo.py · run archivo.py · /usr/bin/x.py se llama 'x'
//
// Tiene: int, float, str, bool, None, list, tuple, dict, set;
// if/elif/else, while, for, break/continue, def (defaults, *args,
// **kwargs), lambda, return, global, comprensiones, f-strings,
// slicing, try/except/finally, raise, with open(...), assert, del,
// class sencillas (métodos, __init__, herencia simple).
// Módulos: math, random, time, json, os, sys.
// No tiene: async, generadores (yield), decoradores, pip.

// ---------- valores ----------

final class PyLista { var v: [PyValor]; init(_ v: [PyValor]) { self.v = v } }

final class PyDicc {
    var claves: [PyValor] = []
    var mapa: [String: PyValor] = [:]
    init() {}
    func get(_ k: PyValor) -> PyValor? { mapa[k.clave] }
    func set(_ k: PyValor, _ v: PyValor) {
        if mapa[k.clave] == nil { claves.append(k) }
        mapa[k.clave] = v
    }
    @discardableResult
    func quita(_ k: PyValor) -> PyValor? {
        guard let v = mapa.removeValue(forKey: k.clave) else { return nil }
        claves.removeAll { $0.clave == k.clave }
        return v
    }
}

final class PyFuncion {
    let nombre: String
    let params: [PyParam]
    let cuerpo: [PySent]
    let cierre: PyAmbito
    let expresion: PyExp?          // lambda
    init(nombre: String, params: [PyParam], cuerpo: [PySent], cierre: PyAmbito, expresion: PyExp? = nil) {
        self.nombre = nombre; self.params = params; self.cuerpo = cuerpo; self.cierre = cierre; self.expresion = expresion
    }
}

final class PyClase {
    let nombre: String
    let base: PyClase?
    var atributos: [String: PyValor]
    init(nombre: String, base: PyClase?, atributos: [String: PyValor]) {
        self.nombre = nombre; self.base = base; self.atributos = atributos
    }
    func busca(_ n: String) -> PyValor? { atributos[n] ?? base?.busca(n) }
    func esSubclase(de c: PyClase) -> Bool { self === c || (base?.esSubclase(de: c) ?? false) }
}

final class PyObjeto {
    let clase: PyClase
    var atributos: [String: PyValor] = [:]
    init(_ c: PyClase) { clase = c }
}

final class PyArchivo {
    let url: URL
    let modo: String
    var contenido: String
    var pos: String.Index
    var cerrado = false
    init(url: URL, modo: String, contenido: String) {
        self.url = url; self.modo = modo; self.contenido = contenido; pos = contenido.startIndex
    }
}

typealias PyNativa = (_ args: [PyValor], _ kw: [String: PyValor]) throws -> PyValor

final class PyBuiltin {
    let nombre: String
    let f: PyNativa
    init(_ n: String, _ f: @escaping PyNativa) { nombre = n; self.f = f }
}

/// Cada intérprete corre en un solo hilo a la vez (enHiloPython), así que
/// sus valores pueden viajar dentro de los errores sin peligro.
indirect enum PyValor: @unchecked Sendable {
    case none
    case bool(Bool)
    case int(Int)
    case float(Double)
    case str(String)
    case lista(PyLista)
    case tupla([PyValor])
    case dicc(PyDicc)
    case conjunto(PyDicc)
    case funcion(PyFuncion)
    case builtin(PyBuiltin)
    case metodo(PyValor, PyFuncion)          // función ligada a un objeto
    case clase(PyClase)
    case objeto(PyObjeto)
    case modulo(String, PyDicc)
    case archivo(PyArchivo)

    var tipo: String {
        switch self {
        case .none: return "NoneType"
        case .bool: return "bool"
        case .int: return "int"
        case .float: return "float"
        case .str: return "str"
        case .lista: return "list"
        case .tupla: return "tuple"
        case .dicc: return "dict"
        case .conjunto: return "set"
        case .funcion, .metodo: return "function"
        case .builtin: return "builtin_function_or_method"
        case .clase: return "type"
        case .objeto(let o): return o.clase.nombre
        case .modulo: return "module"
        case .archivo: return "TextIOWrapper"
        }
    }

    /// Clave para diccionarios y conjuntos.
    var clave: String {
        switch self {
        case .none: return "N"
        case .bool(let b): return b ? "i:1" : "i:0"
        case .int(let i): return "i:\(i)"
        case .float(let d): return d == d.rounded() && abs(d) < 1e18 ? "i:\(Int(d))" : "f:\(d)"
        case .str(let s): return "s:" + s
        case .tupla(let t): return "t:(" + t.map(\.clave).joined(separator: ",") + ")"
        case .objeto(let o): return "o:\(ObjectIdentifier(o).hashValue)"
        case .clase(let c): return "c:\(ObjectIdentifier(c).hashValue)"
        default: return "x:\(tipo)"
        }
    }

    var verdad: Bool {
        switch self {
        case .none: return false
        case .bool(let b): return b
        case .int(let i): return i != 0
        case .float(let d): return d != 0
        case .str(let s): return !s.isEmpty
        case .lista(let l): return !l.v.isEmpty
        case .tupla(let t): return !t.isEmpty
        case .dicc(let d), .conjunto(let d): return !d.claves.isEmpty
        default: return true
        }
    }
}

struct PyError: Error {
    let tipo: String
    let mensaje: String
    var linea: Int = 0
    var valor: PyValor? = nil
    init(_ tipo: String, _ mensaje: String) { self.tipo = tipo; self.mensaje = mensaje }
}

// ---------- árbol ----------

struct PyParam {
    let nombre: String
    let defecto: PyExp?
    let estrella: Bool        // *args
    let dobleEstrella: Bool   // **kwargs
}

indirect enum PyFParte {
    case texto(String)
    case exp(PyExp, String?, String?)   // expresión, conversión (!r), formato (:.2f)
}

struct PyGen { let objetivo: PyExp; let iter: PyExp; let conds: [PyExp] }

indirect enum PyExp {
    case lit(PyValor)
    case fstr([PyFParte])
    case nombre(String)
    case lista([PyExp])
    case tupla([PyExp])
    case dicc([(PyExp, PyExp)])
    case conjunto([PyExp])
    case bin(String, PyExp, PyExp)
    case un(String, PyExp)
    case y(PyExp, PyExp)
    case o(PyExp, PyExp)
    case compara([String], [PyExp])
    case llamada(PyExp, [PyExp], [(String, PyExp)])
    case estrella(PyExp)                 // *x dentro de una llamada
    case dobleEstrella(PyExp)            // **x dentro de una llamada
    case sub(PyExp, PyExp)
    case rebanada(PyExp, PyExp?, PyExp?, PyExp?)
    case atributo(PyExp, String)
    case ternaria(PyExp, PyExp, PyExp)   // cond, si, no
    case lambda([PyParam], PyExp)
    case compLista(PyExp, [PyGen])
    case compConj(PyExp, [PyGen])
    case compDicc(PyExp, PyExp, [PyGen])
}

indirect enum PySentencia {
    case exp(PyExp)
    case asigna([PyExp], PyExp)
    case aumenta(PyExp, String, PyExp)
    case si([(PyExp, [PySent])], [PySent]?)
    case mientras(PyExp, [PySent], [PySent]?)
    case para(PyExp, PyExp, [PySent], [PySent]?)
    case def(String, [PyParam], [PySent])
    case clase(String, PyExp?, [PySent])
    case retorna(PyExp?)
    case rompe, sigue, pasa
    case global([String])
    case importa([(String, String?)])
    case desde(String, [(String, String?)])
    case intenta([PySent], [(PyExp?, String?, [PySent])], [PySent]?, [PySent]?)
    case lanza(PyExp?)
    case con(PyExp, String?, [PySent])
    case borra([PyExp])
    case afirma(PyExp, PyExp?)
}

struct PySent { let linea: Int; let s: PySentencia }

// ---------- léxico ----------

struct PyTok {
    enum T { case nombre, numero, cadena, fcadena, op, nl, indent, dedent, fin }
    let t: T
    let texto: String
    let linea: Int
}

enum PyLexico {
    static let ops3 = ["**=", "//=", ">>=", "<<=", "..."]
    static let ops2 = ["**", "//", "==", "!=", "<=", ">=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "->", "<<", ">>", ":="]

    static func tokens(_ fuente: String) throws -> [PyTok] {
        let c = Array(fuente.replacingOccurrences(of: "\r\n", with: "\n"))
        var i = 0
        var linea = 1
        var out: [PyTok] = []
        var sangrias = [0]
        var profundidad = 0
        var inicio = true

        func error(_ m: String) -> PyError { var e = PyError("SyntaxError", m); e.linea = linea; return e }

        while i < c.count {
            if inicio && profundidad == 0 {
                // sangría de una línea lógica nueva
                var ancho = 0
                var j = i
                while j < c.count, c[j] == " " || c[j] == "\t" { ancho += c[j] == "\t" ? 4 : 1; j += 1 }
                if j >= c.count { i = j; break }
                if c[j] == "\n" { i = j + 1; linea += 1; continue }          // línea vacía
                if c[j] == "#" {                                               // solo comentario
                    while j < c.count, c[j] != "\n" { j += 1 }
                    i = j; continue
                }
                if ancho > sangrias.last! {
                    sangrias.append(ancho)
                    out.append(PyTok(t: .indent, texto: "", linea: linea))
                } else {
                    while ancho < sangrias.last! {
                        sangrias.removeLast()
                        out.append(PyTok(t: .dedent, texto: "", linea: linea))
                    }
                    if ancho != sangrias.last! { throw error("la sangría no coincide con ningún bloque") }
                }
                i = j
                inicio = false
            }
            let ch = c[i]
            if ch == "\n" {
                if profundidad == 0 {
                    if out.last?.t != .nl { out.append(PyTok(t: .nl, texto: "", linea: linea)) }
                    inicio = true
                }
                linea += 1; i += 1; continue
            }
            if ch == " " || ch == "\t" { i += 1; continue }
            if ch == "#" { while i < c.count, c[i] != "\n" { i += 1 }; continue }
            if ch == "\\", i + 1 < c.count, c[i + 1] == "\n" { i += 2; linea += 1; continue }

            // números
            if ch.isNumber || (ch == "." && i + 1 < c.count && c[i + 1].isNumber) {
                var j = i
                var s = ""
                if ch == "0", j + 1 < c.count, "xXbBoO".contains(c[j + 1]) {
                    s = String(c[j ... j + 1]); j += 2
                    while j < c.count, c[j].isHexDigit || c[j] == "_" { if c[j] != "_" { s.append(c[j]) }; j += 1 }
                } else {
                    while j < c.count, c[j].isNumber || c[j] == "." || c[j] == "_" ||
                            ((c[j] == "e" || c[j] == "E") && j + 1 < c.count && (c[j + 1].isNumber || c[j + 1] == "-" || c[j + 1] == "+")) ||
                            ((c[j] == "-" || c[j] == "+") && j > i && (c[j - 1] == "e" || c[j - 1] == "E")) {
                        if c[j] != "_" { s.append(c[j]) }
                        j += 1
                    }
                }
                out.append(PyTok(t: .numero, texto: s, linea: linea))
                i = j; continue
            }

            // cadenas (con prefijos r, f, b, u)
            var k = i
            var prefijo = ""
            while k < c.count, "rRfFbBuU".contains(c[k]), prefijo.count < 2 { prefijo.append(c[k]); k += 1 }
            if k < c.count, c[k] == "\"" || c[k] == "'", prefijo.isEmpty || !(i > 0 && (c[i - 1].isLetter || c[i - 1] == "_")) {
                let q = c[k]
                let triple = k + 2 < c.count && c[k + 1] == q && c[k + 2] == q
                var j = k + (triple ? 3 : 1)
                var s = ""
                let crudo = prefijo.lowercased().contains("r")
                var cerrada = false
                while j < c.count {
                    if triple, c[j] == q, j + 2 < c.count, c[j + 1] == q, c[j + 2] == q { j += 3; cerrada = true; break }
                    if !triple, c[j] == q { j += 1; cerrada = true; break }
                    if !triple, c[j] == "\n" { break }
                    if c[j] == "\n" { linea += 1 }
                    if c[j] == "\\", j + 1 < c.count, !crudo {
                        let n = c[j + 1]
                        switch n {
                        case "n": s.append("\n")
                        case "t": s.append("\t")
                        case "r": s.append("\r")
                        case "0": s.append("\0")
                        case "\\": s.append("\\")
                        case "'": s.append("'")
                        case "\"": s.append("\"")
                        case "\n": linea += 1
                        default: s.append("\\"); s.append(n)
                        }
                        j += 2; continue
                    }
                    s.append(c[j]); j += 1
                }
                guard cerrada else { throw error("cadena sin cerrar") }
                out.append(PyTok(t: prefijo.lowercased().contains("f") ? .fcadena : .cadena, texto: s, linea: linea))
                i = j; continue
            }

            // nombres
            if ch.isLetter || ch == "_" {
                var j = i
                while j < c.count, c[j].isLetter || c[j].isNumber || c[j] == "_" { j += 1 }
                out.append(PyTok(t: .nombre, texto: String(c[i ..< j]), linea: linea))
                i = j; continue
            }

            // operadores
            let resto3 = i + 3 <= c.count ? String(c[i ..< i + 3]) : ""
            let resto2 = i + 2 <= c.count ? String(c[i ..< i + 2]) : ""
            var op = String(ch)
            if ops3.contains(resto3) { op = resto3 } else if ops2.contains(resto2) { op = resto2 }
            if "([{".contains(op) { profundidad += 1 }
            if ")]}".contains(op) { profundidad = max(0, profundidad - 1) }
            guard "+-*/%=<>!&|^~()[]{},:.;@".contains(op.first!) else { throw error("carácter no válido: \(op)") }
            out.append(PyTok(t: .op, texto: op, linea: linea))
            i += op.count
        }
        if out.last?.t != .nl { out.append(PyTok(t: .nl, texto: "", linea: linea)) }
        while sangrias.count > 1 { sangrias.removeLast(); out.append(PyTok(t: .dedent, texto: "", linea: linea)) }
        out.append(PyTok(t: .fin, texto: "", linea: linea))
        return out
    }
}

// ---------- análisis ----------

final class PyAnalizador {
    private var t: [PyTok]
    private var p = 0

    init(_ fuente: String) throws { t = try PyLexico.tokens(fuente) }

    private var actual: PyTok { t[p] }
    private func es(_ op: String) -> Bool { (actual.t == .op || actual.t == .nombre) && actual.texto == op }
    private func avanza() -> PyTok { let x = t[p]; if p < t.count - 1 { p += 1 }; return x }
    private func error(_ m: String) -> PyError { var e = PyError("SyntaxError", m); e.linea = actual.linea; return e }
    private func espera(_ op: String) throws {
        guard es(op) else { throw error("se esperaba '\(op)' y hay '\(actual.texto.isEmpty ? "fin de línea" : actual.texto)'") }
        _ = avanza()
    }
    private func acepta(_ op: String) -> Bool { if es(op) { _ = avanza(); return true }; return false }

    func programa() throws -> [PySent] {
        var s: [PySent] = []
        while actual.t != .fin {
            if actual.t == .nl { _ = avanza(); continue }
            s += try sentencia()
        }
        return s
    }

    private func bloque() throws -> [PySent] {
        try espera(":")
        if actual.t != .nl {                       // if x: y  (en la misma línea)
            return try simples()
        }
        _ = avanza()
        guard actual.t == .indent else { throw error("se esperaba un bloque con sangría") }
        _ = avanza()
        var s: [PySent] = []
        while actual.t != .dedent && actual.t != .fin {
            if actual.t == .nl { _ = avanza(); continue }
            s += try sentencia()
        }
        if actual.t == .dedent { _ = avanza() }
        return s
    }

    private func sentencia() throws -> [PySent] {
        let l = actual.linea
        if actual.t == .nombre {
            switch actual.texto {
            case "if":
                _ = avanza()
                var ramas = [(try expresion(), try bloque())]
                var sino: [PySent]? = nil
                while es("elif") { _ = avanza(); ramas.append((try expresion(), try bloque())) }
                if es("else") { _ = avanza(); sino = try bloque() }
                return [PySent(linea: l, s: .si(ramas, sino))]
            case "while":
                _ = avanza()
                let c = try expresion()
                let b = try bloque()
                var sino: [PySent]? = nil
                if es("else") { _ = avanza(); sino = try bloque() }
                return [PySent(linea: l, s: .mientras(c, b, sino))]
            case "for":
                _ = avanza()
                let obj = try listaObjetivos()
                try espera("in")
                let it = try listaExp()
                let b = try bloque()
                var sino: [PySent]? = nil
                if es("else") { _ = avanza(); sino = try bloque() }
                return [PySent(linea: l, s: .para(obj, it, b, sino))]
            case "def":
                _ = avanza()
                let n = try nombre()
                try espera("(")
                let ps = try parametros(cierre: ")")
                try espera(")")
                if acepta("->") { _ = try expresion() }
                return [PySent(linea: l, s: .def(n, ps, try bloque()))]
            case "class":
                _ = avanza()
                let n = try nombre()
                var base: PyExp? = nil
                if acepta("(") {
                    if !es(")") { base = try expresion() }
                    try espera(")")
                }
                return [PySent(linea: l, s: .clase(n, base, try bloque()))]
            case "try":
                _ = avanza()
                let cuerpo = try bloque()
                var manejadores: [(PyExp?, String?, [PySent])] = []
                var sino: [PySent]? = nil
                var fin: [PySent]? = nil
                while es("except") {
                    _ = avanza()
                    var tipo: PyExp? = nil
                    var como: String? = nil
                    if !es(":") {
                        tipo = try expresion()
                        if acepta("as") { como = try nombre() }
                    }
                    manejadores.append((tipo, como, try bloque()))
                }
                if es("else") { _ = avanza(); sino = try bloque() }
                if es("finally") { _ = avanza(); fin = try bloque() }
                return [PySent(linea: l, s: .intenta(cuerpo, manejadores, sino, fin))]
            case "with":
                _ = avanza()
                let e = try expresion()
                var como: String? = nil
                if acepta("as") { como = try nombre() }
                return [PySent(linea: l, s: .con(e, como, try bloque()))]
            default: break
            }
        }
        if es("@") { throw error("los decoradores (@) no están soportados") }
        return try simples()
    }

    /// Una o varias sentencias simples separadas por ';' hasta el fin de línea.
    private func simples() throws -> [PySent] {
        var s: [PySent] = [try simple()]
        while acepta(";") {
            if actual.t == .nl { break }
            s.append(try simple())
        }
        guard actual.t == .nl || actual.t == .fin || actual.t == .dedent else {
            throw error("sobra '\(actual.texto)' al final de la línea")
        }
        if actual.t == .nl { _ = avanza() }
        return s
    }

    private func simple() throws -> PySent {
        let l = actual.linea
        if actual.t == .nombre {
            switch actual.texto {
            case "pass": _ = avanza(); return PySent(linea: l, s: .pasa)
            case "break": _ = avanza(); return PySent(linea: l, s: .rompe)
            case "continue": _ = avanza(); return PySent(linea: l, s: .sigue)
            case "return":
                _ = avanza()
                if actual.t == .nl || es(";") { return PySent(linea: l, s: .retorna(nil)) }
                return PySent(linea: l, s: .retorna(try listaExp()))
            case "global", "nonlocal":
                _ = avanza()
                var ns = [try nombre()]
                while acepta(",") { ns.append(try nombre()) }
                return PySent(linea: l, s: .global(ns))
            case "import":
                _ = avanza()
                var ms: [(String, String?)] = []
                repeat {
                    var n = try nombre()
                    while acepta(".") { n += "." + (try nombre()) }
                    ms.append((n, acepta("as") ? try nombre() : nil))
                } while acepta(",")
                return PySent(linea: l, s: .importa(ms))
            case "from":
                _ = avanza()
                var m = try nombre()
                while acepta(".") { m += "." + (try nombre()) }
                try espera("import")
                var ns: [(String, String?)] = []
                let paren = acepta("(")
                repeat {
                    if es("*") { _ = avanza(); ns.append(("*", nil)); continue }
                    let n = try nombre()
                    ns.append((n, acepta("as") ? try nombre() : nil))
                } while acepta(",")
                if paren { try espera(")") }
                return PySent(linea: l, s: .desde(m, ns))
            case "raise":
                _ = avanza()
                if actual.t == .nl { return PySent(linea: l, s: .lanza(nil)) }
                let e = try expresion()
                if acepta("from") { _ = try expresion() }
                return PySent(linea: l, s: .lanza(e))
            case "del":
                _ = avanza()
                var es_ = [try expresion()]
                while acepta(",") { es_.append(try expresion()) }
                return PySent(linea: l, s: .borra(es_))
            case "assert":
                _ = avanza()
                let c = try expresion()
                return PySent(linea: l, s: .afirma(c, acepta(",") ? try expresion() : nil))
            default: break
            }
        }
        let e = try listaExp(estrellas: true)
        let aumentados = ["+=", "-=", "*=", "/=", "//=", "%=", "**=", "&=", "|=", "^=", ">>=", "<<="]
        if actual.t == .op, aumentados.contains(actual.texto) {
            let op = String(avanza().texto.dropLast())
            return PySent(linea: l, s: .aumenta(e, op, try listaExp()))
        }
        if acepta(":") {                         // anotación de tipo: x: int = 3
            _ = try expresion()
            if !acepta("=") { return PySent(linea: l, s: .pasa) }
            return PySent(linea: l, s: .asigna([e], try listaExp()))
        }
        if es("=") {
            var objetivos = [e]
            var valor = e
            while acepta("=") {
                valor = try listaExp(estrellas: true)
                objetivos.append(valor)
            }
            objetivos.removeLast()
            return PySent(linea: l, s: .asigna(objetivos, valor))
        }
        return PySent(linea: l, s: .exp(e))
    }

    private func nombre() throws -> String {
        guard actual.t == .nombre else { throw error("se esperaba un nombre y hay '\(actual.texto)'") }
        return avanza().texto
    }

    private func parametros(cierre: String) throws -> [PyParam] {
        var ps: [PyParam] = []
        while !es(cierre) {
            if acepta("**") { ps.append(PyParam(nombre: try nombre(), defecto: nil, estrella: false, dobleEstrella: true)) }
            else if acepta("*") {
                if es(",") { _ = avanza(); continue }      // * suelto: solo nombrados después
                ps.append(PyParam(nombre: try nombre(), defecto: nil, estrella: true, dobleEstrella: false))
            } else if es("/") { _ = avanza() }
            else {
                let n = try nombre()
                if cierre == ")", acepta(":") { _ = try expresion() }   // anotación
                ps.append(PyParam(nombre: n, defecto: acepta("=") ? try expresion() : nil, estrella: false, dobleEstrella: false))
            }
            if !acepta(",") { break }
        }
        return ps
    }

    /// Objetivos de un for: a  ·  a, b  ·  (a, b)
    private func listaObjetivos() throws -> PyExp {
        // bitOr y no or(): el 'in' del for no es el operador 'in'
        var es_ = [acepta("*") ? PyExp.estrella(try bitOr()) : try bitOr()]
        var tupla = false
        while acepta(",") { tupla = true; if es("in") { break }; es_.append(acepta("*") ? .estrella(try bitOr()) : try bitOr()) }
        return tupla ? .tupla(es_) : es_[0]
    }

    /// a, b, c → tupla (sin paréntesis)
    private func listaExp(estrellas: Bool = false) throws -> PyExp {
        func una() throws -> PyExp {
            if estrellas, acepta("*") { return .estrella(try or()) }
            return try expresion()
        }
        let primera = try una()
        guard es(",") else { return primera }
        var es_ = [primera]
        while acepta(",") {
            if actual.t == .nl || es("=") || es(")") || actual.t == .fin { break }
            es_.append(try una())
        }
        return .tupla(es_)
    }

    func expresion() throws -> PyExp {
        if es("lambda") {
            _ = avanza()
            let ps = try parametros(cierre: ":")
            try espera(":")
            return .lambda(ps, try expresion())
        }
        let e = try or()
        if es("if") {
            _ = avanza()
            let c = try or()
            try espera("else")
            return .ternaria(c, e, try expresion())
        }
        return e
    }

    private func or() throws -> PyExp {
        var e = try and()
        while acepta("or") { e = .o(e, try and()) }
        return e
    }

    private func and() throws -> PyExp {
        var e = try not()
        while acepta("and") { e = .y(e, try not()) }
        return e
    }

    private func not() throws -> PyExp {
        if acepta("not") { return .un("not", try not()) }
        return try comparacion()
    }

    private func comparacion() throws -> PyExp {
        let primera = try bitOr()
        var ops: [String] = []
        var es_ = [primera]
        while true {
            if actual.t == .op, ["<", ">", "==", "!=", "<=", ">="].contains(actual.texto) { ops.append(avanza().texto) }
            else if es("in") { _ = avanza(); ops.append("in") }
            else if es("not"), p + 1 < t.count, t[p + 1].texto == "in" { _ = avanza(); _ = avanza(); ops.append("not in") }
            else if es("is") { _ = avanza(); ops.append(acepta("not") ? "is not" : "is") }
            else { break }
            es_.append(try bitOr())
        }
        return ops.isEmpty ? primera : .compara(ops, es_)
    }

    private func binaria(_ ops: [String], _ siguiente: () throws -> PyExp) throws -> PyExp {
        var e = try siguiente()
        while actual.t == .op, ops.contains(actual.texto) {
            let op = avanza().texto
            e = .bin(op, e, try siguiente())
        }
        return e
    }

    private func bitOr() throws -> PyExp { try binaria(["|"]) { try bitXor() } }
    private func bitXor() throws -> PyExp { try binaria(["^"]) { try bitAnd() } }
    private func bitAnd() throws -> PyExp { try binaria(["&"]) { try desplaza() } }
    private func desplaza() throws -> PyExp { try binaria(["<<", ">>"]) { try suma() } }
    private func suma() throws -> PyExp { try binaria(["+", "-"]) { try producto() } }
    private func producto() throws -> PyExp { try binaria(["*", "/", "//", "%", "@"]) { try unaria() } }

    private func unaria() throws -> PyExp {
        if actual.t == .op, ["-", "+", "~"].contains(actual.texto) {
            let op = avanza().texto
            return .un(op, try unaria())
        }
        return try potencia()
    }

    private func potencia() throws -> PyExp {
        let b = try primaria()
        if acepta("**") { return .bin("**", b, try unaria()) }
        return b
    }

    private func primaria() throws -> PyExp {
        var e = try atomo()
        while true {
            if acepta("(") {
                var args: [PyExp] = []
                var kw: [(String, PyExp)] = []
                while !es(")") {
                    if acepta("**") { args.append(.dobleEstrella(try expresion())) }
                    else if acepta("*") { args.append(.estrella(try expresion())) }
                    else if actual.t == .nombre, p + 1 < t.count, t[p + 1].texto == "=", t[p + 1].t == .op {
                        let n = avanza().texto; _ = avanza()
                        kw.append((n, try expresion()))
                    } else {
                        let a = try expresion()
                        if es("for") { args.append(.compLista(a, try generadores())) }   // f(x for x in l)
                        else { args.append(a) }
                    }
                    if !acepta(",") { break }
                }
                try espera(")")
                e = .llamada(e, args, kw)
            } else if acepta("[") {
                func trozo() throws -> PyExp? { (es(":") || es("]")) ? nil : try expresion() }
                let a = try trozo()
                if acepta(":") {
                    let b = try trozo()
                    var c: PyExp? = nil
                    if acepta(":") { c = try trozo() }
                    try espera("]")
                    e = .rebanada(e, a, b, c)
                } else {
                    var idx = a!
                    if es(",") {
                        var es_ = [idx]
                        while acepta(",") { if es("]") { break }; es_.append(try expresion()) }
                        idx = .tupla(es_)
                    }
                    try espera("]")
                    e = .sub(e, idx)
                }
            } else if acepta(".") {
                e = .atributo(e, try nombre())
            } else {
                return e
            }
        }
    }

    private func generadores() throws -> [PyGen] {
        var gs: [PyGen] = []
        while acepta("for") {
            let obj = try listaObjetivos()
            try espera("in")
            let it = try or()
            var conds: [PyExp] = []
            while acepta("if") { conds.append(try or()) }
            gs.append(PyGen(objetivo: obj, iter: it, conds: conds))
        }
        return gs
    }

    private func atomo() throws -> PyExp {
        let k = actual
        switch k.t {
        case .numero:
            _ = avanza()
            let s = k.texto.lowercased()
            if s.hasPrefix("0x"), let v = Int(s.dropFirst(2), radix: 16) { return .lit(.int(v)) }
            if s.hasPrefix("0b"), let v = Int(s.dropFirst(2), radix: 2) { return .lit(.int(v)) }
            if s.hasPrefix("0o"), let v = Int(s.dropFirst(2), radix: 8) { return .lit(.int(v)) }
            if let v = Int(s) { return .lit(.int(v)) }
            if let d = Double(s) { return .lit(.float(d)) }
            throw error("número no válido: \(k.texto)")
        case .cadena, .fcadena:
            // cadenas seguidas se juntan: "a" "b"
            var partes: [PyFParte] = []
            var hayF = false
            while actual.t == .cadena || actual.t == .fcadena {
                let x = avanza()
                if x.t == .fcadena { hayF = true; partes += try fPartes(x.texto, linea: x.linea) }
                else { partes.append(.texto(x.texto)) }
            }
            if !hayF {
                return .lit(.str(partes.map { if case .texto(let s) = $0 { return s }; return "" }.joined()))
            }
            return .fstr(partes)
        case .nombre:
            switch k.texto {
            case "True": _ = avanza(); return .lit(.bool(true))
            case "False": _ = avanza(); return .lit(.bool(false))
            case "None": _ = avanza(); return .lit(.none)
            default:
                let reservadas: Set<String> = ["if", "else", "elif", "while", "for", "in", "def", "return", "and", "or",
                                               "not", "class", "try", "except", "finally", "import", "from", "pass",
                                               "break", "continue", "global", "raise", "with", "as", "del", "is", "lambda"]
                if reservadas.contains(k.texto) { throw error("'\(k.texto)' no puede ir aquí") }
                _ = avanza(); return .nombre(k.texto)
            }
        case .op:
            if acepta("(") {
                if acepta(")") { return .tupla([]) }
                let e = try expresion()
                if es("for") { let g = try generadores(); try espera(")"); return .compLista(e, g) }
                if es(",") {
                    var es_ = [e]
                    while acepta(",") { if es(")") { break }; es_.append(try expresion()) }
                    try espera(")")
                    return .tupla(es_)
                }
                try espera(")")
                return e
            }
            if acepta("[") {
                if acepta("]") { return .lista([]) }
                let primera = es("*") ? PyExp.estrella({ _ = avanza(); return try! or() }()) : try expresion()
                if es("for") { let g = try generadores(); try espera("]"); return .compLista(primera, g) }
                var es_ = [primera]
                while acepta(",") { if es("]") { break }; es_.append(acepta("*") ? .estrella(try or()) : try expresion()) }
                try espera("]")
                return .lista(es_)
            }
            if acepta("{") {
                if acepta("}") { return .dicc([]) }
                let k1 = try expresion()
                if acepta(":") {
                    let v1 = try expresion()
                    if es("for") { let g = try generadores(); try espera("}"); return .compDicc(k1, v1, g) }
                    var pares = [(k1, v1)]
                    while acepta(",") {
                        if es("}") { break }
                        let k = try expresion(); try espera(":")
                        pares.append((k, try expresion()))
                    }
                    try espera("}")
                    return .dicc(pares)
                }
                if es("for") { let g = try generadores(); try espera("}"); return .compConj(k1, g) }
                var es_ = [k1]
                while acepta(",") { if es("}") { break }; es_.append(try expresion()) }
                try espera("}")
                return .conjunto(es_)
            }
            throw error("no se esperaba '\(k.texto)'")
        case .nl: throw error("la línea está incompleta")
        case .indent: throw error("sangría inesperada")
        default: throw error("no se esperaba '\(k.texto)'")
        }
    }

    /// Partes de un f-string: texto y {expresión!r:formato}
    private func fPartes(_ s: String, linea: Int) throws -> [PyFParte] {
        var partes: [PyFParte] = []
        let c = Array(s)
        var i = 0
        var texto = ""
        while i < c.count {
            if c[i] == "{" {
                if i + 1 < c.count, c[i + 1] == "{" { texto.append("{"); i += 2; continue }
                if !texto.isEmpty { partes.append(.texto(texto)); texto = "" }
                var j = i + 1
                var nivel = 0
                var dentro = ""
                while j < c.count {
                    if c[j] == "{" || c[j] == "[" || c[j] == "(" { nivel += 1 }
                    if c[j] == "]" || c[j] == ")" { nivel -= 1 }
                    if c[j] == "}" { if nivel == 0 { break }; nivel -= 1 }
                    dentro.append(c[j]); j += 1
                }
                guard j < c.count else { var e = PyError("SyntaxError", "f-string sin cerrar '}'"); e.linea = linea; throw e }
                var formato: String? = nil
                var conv: String? = nil
                // separa :formato (fuera de corchetes/paréntesis) y !r
                var nv = 0
                var corte: String.Index? = nil
                for idx in dentro.indices {
                    let ch = dentro[idx]
                    if "([{".contains(ch) { nv += 1 } else if ")]}".contains(ch) { nv -= 1 }
                    else if ch == ":", nv == 0 { corte = idx; break }
                }
                var exp = dentro
                if let cidx = corte {
                    exp = String(dentro[dentro.startIndex ..< cidx])
                    formato = String(dentro[dentro.index(after: cidx)...])
                }
                if exp.hasSuffix("!r") || exp.hasSuffix("!s") { conv = String(exp.suffix(1)); exp.removeLast(2) }
                var autoEtiqueta = ""
                if exp.hasSuffix("="), !exp.hasSuffix("==") { exp.removeLast(); autoEtiqueta = exp + "=" }
                if !autoEtiqueta.isEmpty { partes.append(.texto(autoEtiqueta)); if conv == nil, formato == nil { conv = "r" } }
                let a = try PyAnalizador(exp)
                partes.append(.exp(try a.expresion(), conv, formato))
                i = j + 1
            } else if c[i] == "}", i + 1 < c.count, c[i + 1] == "}" {
                texto.append("}"); i += 2
            } else {
                texto.append(c[i]); i += 1
            }
        }
        if !texto.isEmpty { partes.append(.texto(texto)) }
        return partes
    }
}

// ---------- ámbitos y control ----------

final class PyAmbito {
    var vars: [String: PyValor] = [:]
    let padre: PyAmbito?
    var globales: Set<String> = []
    init(padre: PyAmbito?) { self.padre = padre }

    func busca(_ n: String) -> PyValor? {
        if let v = vars[n] { return v }
        return padre?.busca(n)
    }

    var raiz: PyAmbito { padre?.raiz ?? self }
}

enum PyFlujo: Error {
    case rompe, sigue
    case retorna(PyValor)
}

// ---------- intérprete ----------

final class PyInterprete: @unchecked Sendable {
    let globales = PyAmbito(padre: nil)
    var salida = ""
    weak var env: ShellEnv?
    var argv: [String] = []
    /// input(): primero lo que llegó por tubería, después el teclado (consola)
    var lineasEntrada: [String] = []
    var pideLinea: (() -> String?)?
    var volcar: ((String) -> Void)?
    private var pasos = 0
    private var profundidad = 0
    static let maxPasos = 5_000_000
    static let maxProfundidad = 400
    private var builtins: [String: PyValor] = [:]

    init(env: ShellEnv?) {
        self.env = env
        instalaBuiltins()
    }

    /// Ejecuta código. Devuelve lo impreso, y el error (estilo Python) si lo hubo.
    func ejecuta(_ fuente: String, archivo: String = "<stdin>") -> (salida: String, error: String?) {
        salida = ""
        pasos = 0
        do {
            let prog = try PyAnalizador(fuente).programa()
            try bloque(prog, globales)
            return (salida, nil)
        } catch let e as PyError {
            return (salida, "Traceback (most recent call last):\n  File \"\(archivo)\", line \(e.linea)\n\(e.tipo): \(e.mensaje)")
        } catch PyFlujo.retorna {
            return (salida, "SyntaxError: 'return' fuera de una función")
        } catch {
            return (salida, "SyntaxError: 'break' o 'continue' fuera de un bucle")
        }
    }

    /// Para el modo interactivo: si es una sola expresión, muestra su valor.
    func evalua(_ fuente: String) -> (salida: String, error: String?) {
        salida = ""
        pasos = 0
        do {
            let prog = try PyAnalizador(fuente).programa()
            if prog.count == 1, case .exp(let e) = prog[0].s {
                let v = try evalua(e, globales)
                if case .none = v {} else { salida += repr(v) + "\n" }
                return (salida, nil)
            }
            try bloque(prog, globales)
            return (salida, nil)
        } catch let e as PyError {
            return (salida, "\(e.tipo): \(e.mensaje)" + (e.linea > 1 ? " (línea \(e.linea))" : ""))
        } catch {
            return (salida, "SyntaxError: sentencia fuera de lugar")
        }
    }

    // ----- sentencias -----

    func bloque(_ s: [PySent], _ a: PyAmbito) throws {
        for x in s { try sentencia(x, a) }
    }

    private func sentencia(_ st: PySent, _ a: PyAmbito) throws {
        pasos += 1
        if pasos > PyInterprete.maxPasos { var e = PyError("RuntimeError", "demasiados pasos (¿un bucle infinito?)"); e.linea = st.linea; throw e }
        do {
            switch st.s {
            case .exp(let e): _ = try evalua(e, a)
            case .asigna(let objetivos, let valor):
                let v = try evalua(valor, a)
                for o in objetivos { try asigna(o, v, a) }
            case .aumenta(let o, let op, let e):
                let actual = try evalua(o, a)
                let derecho = try evalua(e, a)
                if op == "+", case .lista(let l) = actual {           // lista += iterable modifica en su sitio
                    l.v += try itera(derecho)
                } else {
                    try asigna(o, try binaria(op, actual, derecho), a)
                }
            case .si(let ramas, let sino):
                for (c, b) in ramas where try evalua(c, a).verdad { try bloque(b, a); return }
                if let s = sino { try bloque(s, a) }
            case .mientras(let c, let b, let sino):
                var roto = false
                while try evalua(c, a).verdad {
                    do { try bloque(b, a) } catch PyFlujo.rompe { roto = true; break } catch PyFlujo.sigue { continue }
                }
                if !roto, let s = sino { try bloque(s, a) }
            case .para(let obj, let it, let b, let sino):
                var roto = false
                for v in try itera(try evalua(it, a)) {
                    try asigna(obj, v, a)
                    do { try bloque(b, a) } catch PyFlujo.rompe { roto = true; break } catch PyFlujo.sigue { continue }
                }
                if !roto, let s = sino { try bloque(s, a) }
            case .def(let n, let ps, let cuerpo):
                try asigna(.nombre(n), .funcion(PyFuncion(nombre: n, params: ps, cuerpo: cuerpo, cierre: a)), a)
            case .clase(let n, let baseExp, let cuerpo):
                var base: PyClase? = nil
                if let b = baseExp {
                    guard case .clase(let c) = try evalua(b, a) else { throw PyError("TypeError", "solo se puede heredar de una clase") }
                    base = c
                }
                let ambito = PyAmbito(padre: a)
                try bloque(cuerpo, ambito)
                try asigna(.nombre(n), .clase(PyClase(nombre: n, base: base, atributos: ambito.vars)), a)
            case .retorna(let e):
                throw PyFlujo.retorna(try e.map { try evalua($0, a) } ?? .none)
            case .rompe: throw PyFlujo.rompe
            case .sigue: throw PyFlujo.sigue
            case .pasa: break
            case .global(let ns): a.globales.formUnion(ns)
            case .importa(let ms):
                for (m, como) in ms {
                    let mod = try modulo(m)
                    let nombre = como ?? String(m.split(separator: ".").first ?? Substring(m))
                    try asigna(.nombre(nombre), como == nil && m.contains(".") ? try modulo(nombre) : mod, a)
                }
            case .desde(let m, let ns):
                guard case .modulo(_, let d) = try modulo(m) else { break }
                for (n, como) in ns {
                    if n == "*" { for k in d.claves { if case .str(let s) = k { a.vars[s] = d.get(k) } }; continue }
                    guard let v = d.get(.str(n)) else { throw PyError("ImportError", "no se puede importar '\(n)' de '\(m)'") }
                    a.vars[como ?? n] = v
                }
            case .intenta(let cuerpo, let manejadores, let sino, let fin):
                defer { if let f = fin { try? bloque(f, a) } }
                do {
                    try bloque(cuerpo, a)
                    if let s = sino { try bloque(s, a) }
                } catch let e as PyError {
                    var atrapado = false
                    for (tipo, como, b) in manejadores {
                        if try coincide(e, tipo, a) {
                            if let n = como { a.vars[n] = e.valor ?? excepcion(e.tipo, e.mensaje) }
                            try bloque(b, a)
                            atrapado = true
                            break
                        }
                    }
                    if !atrapado { throw e }
                }
            case .lanza(let e):
                guard let e else { throw PyError("RuntimeError", "raise sin excepción activa") }
                let v = try evalua(e, a)
                switch v {
                case .clase(let c): throw PyError(c.nombre, "")
                case .objeto(let o):
                    var err = PyError(o.clase.nombre, str(o.atributos["args"].flatMap { if case .tupla(let t) = $0 { return t.first }; return nil } ?? .str("")))
                    err.valor = v
                    throw err
                case .builtin(let b): throw PyError(b.nombre, "")
                default: throw PyError("TypeError", "solo se pueden lanzar excepciones")
                }
            case .con(let e, let como, let cuerpo):
                let v = try evalua(e, a)
                if let n = como { a.vars[n] = v }
                defer { if case .archivo(let f) = v { cierra(f) } }
                try bloque(cuerpo, a)
            case .borra(let es_):
                for e in es_ {
                    switch e {
                    case .nombre(let n): a.vars.removeValue(forKey: n)
                    case .sub(let o, let i):
                        let ob = try evalua(o, a), idx = try evalua(i, a)
                        switch ob {
                        case .lista(let l): l.v.remove(at: try indice(idx, l.v.count))
                        case .dicc(let d):
                            guard d.quita(idx) != nil else { throw PyError("KeyError", repr(idx)) }
                        default: throw PyError("TypeError", "no se puede borrar de \(ob.tipo)")
                        }
                    default: throw PyError("SyntaxError", "no se puede borrar eso")
                    }
                }
            case .afirma(let c, let m):
                if !(try evalua(c, a).verdad) { throw PyError("AssertionError", try m.map { str(try evalua($0, a)) } ?? "") }
            }
        } catch var e as PyError {
            if e.linea == 0 { e.linea = st.linea }
            throw e
        }
    }

    private func coincide(_ e: PyError, _ tipo: PyExp?, _ a: PyAmbito) throws -> Bool {
        guard let tipo else { return true }
        let t = try evalua(tipo, a)
        func uno(_ v: PyValor) -> Bool {
            switch v {
            case .builtin(let b): return b.nombre == "Exception" || b.nombre == e.tipo || (b.nombre == "ArithmeticError" && e.tipo == "ZeroDivisionError") || (b.nombre == "LookupError" && (e.tipo == "KeyError" || e.tipo == "IndexError"))
            case .clase(let c):
                if c.nombre == e.tipo { return true }
                if case .objeto(let o)? = e.valor { return o.clase.esSubclase(de: c) }
                if e.tipo == "SystemExit" { return false }
                if case .clase(let propia)? = builtins[e.tipo] { return propia.esSubclase(de: c) }
                return c.nombre == "Exception"
            default: return false
            }
        }
        if case .tupla(let ts) = t { return ts.contains(where: uno) }
        return uno(t)
    }

    private func excepcion(_ tipo: String, _ m: String) -> PyValor {
        let c = PyClase(nombre: tipo, base: nil, atributos: [:])
        let o = PyObjeto(c)
        o.atributos["args"] = .tupla([.str(m)])
        return .objeto(o)
    }

    private func asigna(_ o: PyExp, _ v: PyValor, _ a: PyAmbito) throws {
        switch o {
        case .nombre(let n):
            if a.globales.contains(n) { a.raiz.vars[n] = v } else { a.vars[n] = v }
        case .tupla(let os), .lista(let os):
            let vs = try itera(v)
            if let ie = os.firstIndex(where: { if case .estrella = $0 { return true }; return false }) {
                let despues = os.count - ie - 1
                guard vs.count >= os.count - 1 else { throw PyError("ValueError", "faltan valores para desempaquetar") }
                for i in 0 ..< ie { try asigna(os[i], vs[i], a) }
                if case .estrella(let x) = os[ie] { try asigna(x, .lista(PyLista(Array(vs[ie ..< vs.count - despues]))), a) }
                for i in 0 ..< despues { try asigna(os[ie + 1 + i], vs[vs.count - despues + i], a) }
                return
            }
            guard vs.count == os.count else {
                throw PyError("ValueError", vs.count > os.count ? "demasiados valores para desempaquetar (se esperaban \(os.count))" : "faltan valores para desempaquetar (se esperaban \(os.count), hay \(vs.count))")
            }
            for (x, y) in zip(os, vs) { try asigna(x, y, a) }
        case .sub(let oe, let ie):
            let ob = try evalua(oe, a), idx = try evalua(ie, a)
            switch ob {
            case .lista(let l): l.v[try indice(idx, l.v.count)] = v
            case .dicc(let d): d.set(idx, v)
            default: throw PyError("TypeError", "'\(ob.tipo)' no admite asignar con []")
            }
        case .rebanada(let oe, let ini, let fin, _):
            guard case .lista(let l) = try evalua(oe, a) else { throw PyError("TypeError", "solo listas admiten asignar rebanadas") }
            let n = l.v.count
            let i = try ini.map { try limite(try evalua($0, a), n) } ?? 0
            let f = try fin.map { try limite(try evalua($0, a), n) } ?? n
            l.v.replaceSubrange(i ..< max(i, f), with: try itera(v))
        case .atributo(let oe, let n):
            switch try evalua(oe, a) {
            case .objeto(let ob): ob.atributos[n] = v
            case .clase(let c): c.atributos[n] = v
            default: throw PyError("AttributeError", "no se puede asignar el atributo '\(n)'")
            }
        default:
            throw PyError("SyntaxError", "no se puede asignar a esa expresión")
        }
    }

    // ----- expresiones -----

    func evalua(_ e: PyExp, _ a: PyAmbito) throws -> PyValor {
        switch e {
        case .lit(let v): return v
        case .nombre(let n):
            if let v = a.busca(n) ?? builtins[n] { return v }
            throw PyError("NameError", "name '\(n)' is not defined")
        case .fstr(let partes):
            var s = ""
            for p in partes {
                switch p {
                case .texto(let t): s += t
                case .exp(let x, let conv, let fmt):
                    let v = try evalua(x, a)
                    if let f = fmt { s += try formatea(v, f) } else { s += conv == "r" ? repr(v) : str(v) }
                }
            }
            return .str(s)
        case .lista(let es_): return .lista(PyLista(try expande(es_, a)))
        case .tupla(let es_): return .tupla(try expande(es_, a))
        case .conjunto(let es_):
            let d = PyDicc()
            for v in try expande(es_, a) { d.set(v, .none) }
            return .conjunto(d)
        case .dicc(let pares):
            let d = PyDicc()
            for (k, v) in pares { d.set(try evalua(k, a), try evalua(v, a)) }
            return .dicc(d)
        case .bin(let op, let x, let y): return try binaria(op, try evalua(x, a), try evalua(y, a))
        case .un(let op, let x):
            let v = try evalua(x, a)
            switch (op, v) {
            case ("not", _): return .bool(!v.verdad)
            case ("-", .int(let i)): return .int(-i)
            case ("-", .float(let d)): return .float(-d)
            case ("-", .bool(let b)): return .int(b ? -1 : 0)
            case ("+", .int), ("+", .float): return v
            case ("~", .int(let i)): return .int(~i)
            default: throw PyError("TypeError", "operando no válido para \(op): '\(v.tipo)'")
            }
        case .y(let x, let y):
            let v = try evalua(x, a)
            return v.verdad ? try evalua(y, a) : v
        case .o(let x, let y):
            let v = try evalua(x, a)
            return v.verdad ? v : try evalua(y, a)
        case .compara(let ops, let es_):
            var izq = try evalua(es_[0], a)
            for (i, op) in ops.enumerated() {
                let der = try evalua(es_[i + 1], a)
                if !(try compara(op, izq, der)) { return .bool(false) }
                izq = der
            }
            return .bool(true)
        case .ternaria(let c, let si, let no):
            return try evalua(c, a).verdad ? try evalua(si, a) : try evalua(no, a)
        case .lambda(let ps, let cuerpo):
            return .funcion(PyFuncion(nombre: "<lambda>", params: ps, cuerpo: [], cierre: a, expresion: cuerpo))
        case .llamada(let f, let args, let kw):
            // método: obj.m(...)
            var posicionales: [PyValor] = []
            var nombrados: [String: PyValor] = [:]
            for x in args {
                switch x {
                case .estrella(let y): posicionales += try itera(try evalua(y, a))
                case .dobleEstrella(let y):
                    guard case .dicc(let d) = try evalua(y, a) else { throw PyError("TypeError", "** necesita un dict") }
                    for k in d.claves { nombrados[str(k)] = d.get(k) }
                default: posicionales.append(try evalua(x, a))
                }
            }
            for (k, v) in kw { nombrados[k] = try evalua(v, a) }
            if case .atributo(let oe, let n) = f {
                let ob = try evalua(oe, a)
                if let r = try metodo(ob, n, posicionales, nombrados) { return r }
            }
            return try llama(try evalua(f, a), posicionales, nombrados)
        case .estrella, .dobleEstrella:
            throw PyError("SyntaxError", "* solo puede ir dentro de una llamada o lista")
        case .sub(let oe, let ie):
            return try subindice(try evalua(oe, a), try evalua(ie, a))
        case .rebanada(let oe, let ini, let fin, let paso):
            let v = try evalua(oe, a)
            return try rebanada(v, try ini.map { try evalua($0, a) }, try fin.map { try evalua($0, a) }, try paso.map { try evalua($0, a) })
        case .atributo(let oe, let n):
            return try atributo(try evalua(oe, a), n)
        case .compLista(let x, let gs):
            var r: [PyValor] = []
            try comprension(gs, 0, PyAmbito(padre: a)) { r.append(try self.evalua(x, $0)) }
            return .lista(PyLista(r))
        case .compConj(let x, let gs):
            let d = PyDicc()
            try comprension(gs, 0, PyAmbito(padre: a)) { d.set(try self.evalua(x, $0), .none) }
            return .conjunto(d)
        case .compDicc(let k, let v, let gs):
            let d = PyDicc()
            try comprension(gs, 0, PyAmbito(padre: a)) { d.set(try self.evalua(k, $0), try self.evalua(v, $0)) }
            return .dicc(d)
        }
    }

    private func expande(_ es_: [PyExp], _ a: PyAmbito) throws -> [PyValor] {
        var r: [PyValor] = []
        for e in es_ {
            if case .estrella(let x) = e { r += try itera(try evalua(x, a)) } else { r.append(try evalua(e, a)) }
        }
        return r
    }

    private func comprension(_ gs: [PyGen], _ i: Int, _ a: PyAmbito, _ f: (PyAmbito) throws -> Void) throws {
        if i == gs.count { try f(a); return }
        let g = gs[i]
        for v in try itera(try evalua(g.iter, a)) {
            pasos += 1
            if pasos > PyInterprete.maxPasos { throw PyError("RuntimeError", "demasiados pasos") }
            try asigna(g.objetivo, v, a)
            if try g.conds.allSatisfy({ try evalua($0, a).verdad }) { try comprension(gs, i + 1, a, f) }
        }
    }

    // ----- llamadas -----

    func llama(_ f: PyValor, _ args: [PyValor], _ kw: [String: PyValor] = [:]) throws -> PyValor {
        switch f {
        case .builtin(let b): return try b.f(args, kw)
        case .metodo(let yo, let fn): return try llamaFuncion(fn, [yo] + args, kw)
        case .funcion(let fn): return try llamaFuncion(fn, args, kw)
        case .clase(let c):
            if c.nombre.hasSuffix("Error") || c.nombre.hasSuffix("Exception"), c.busca("__init__") == nil {
                let o = PyObjeto(c)
                o.atributos["args"] = .tupla(args)
                return .objeto(o)
            }
            let o = PyObjeto(c)
            if case .funcion(let ini)? = c.busca("__init__") { _ = try llamaFuncion(ini, [.objeto(o)] + args, kw) }
            return .objeto(o)
        case .objeto(let o):
            if case .funcion(let fn)? = o.clase.busca("__call__") { return try llamaFuncion(fn, [f] + args, kw) }
            throw PyError("TypeError", "'\(o.clase.nombre)' object is not callable")
        default:
            throw PyError("TypeError", "'\(f.tipo)' object is not callable")
        }
    }

    private func llamaFuncion(_ fn: PyFuncion, _ args: [PyValor], _ kw: [String: PyValor]) throws -> PyValor {
        profundidad += 1
        defer { profundidad -= 1 }
        if profundidad > PyInterprete.maxProfundidad {
            throw PyError("RecursionError", "maximum recursion depth exceeded")
        }
        let a = PyAmbito(padre: fn.cierre)
        var i = 0
        var kwRestantes = kw
        for p in fn.params {
            if p.estrella {
                a.vars[p.nombre] = .tupla(i < args.count ? Array(args[i...]) : [])
                i = args.count
            } else if p.dobleEstrella {
                let d = PyDicc()
                for (k, v) in kwRestantes.sorted(by: { $0.key < $1.key }) { d.set(.str(k), v) }
                a.vars[p.nombre] = .dicc(d)
                kwRestantes = [:]
            } else if i < args.count {
                a.vars[p.nombre] = args[i]; i += 1
            } else if let v = kwRestantes.removeValue(forKey: p.nombre) {
                a.vars[p.nombre] = v
            } else if let d = p.defecto {
                a.vars[p.nombre] = try evalua(d, fn.cierre)
            } else {
                throw PyError("TypeError", "\(fn.nombre)() falta el argumento '\(p.nombre)'")
            }
        }
        if i < args.count { throw PyError("TypeError", "\(fn.nombre)() recibe \(fn.params.count) argumentos pero se dieron \(args.count)") }
        if let k = kwRestantes.keys.first { throw PyError("TypeError", "\(fn.nombre)() no tiene el argumento '\(k)'") }
        if let e = fn.expresion { return try evalua(e, a) }
        do {
            try bloque(fn.cuerpo, a)
        } catch PyFlujo.retorna(let v) {
            return v
        }
        return .none
    }

    // ----- operadores -----

    private func num(_ v: PyValor) -> Double? {
        switch v {
        case .int(let i): return Double(i)
        case .float(let d): return d
        case .bool(let b): return b ? 1 : 0
        default: return nil
        }
    }

    private func entero(_ v: PyValor) -> Int? {
        switch v {
        case .int(let i): return i
        case .bool(let b): return b ? 1 : 0
        default: return nil
        }
    }

    func binaria(_ op: String, _ x: PyValor, _ y: PyValor) throws -> PyValor {
        // enteros
        if let a = entero(x), let b = entero(y) {
            switch op {
            case "+": let r = a.addingReportingOverflow(b); return r.overflow ? .float(Double(a) + Double(b)) : .int(r.partialValue)
            case "-": let r = a.subtractingReportingOverflow(b); return r.overflow ? .float(Double(a) - Double(b)) : .int(r.partialValue)
            case "*": let r = a.multipliedReportingOverflow(by: b); return r.overflow ? .float(Double(a) * Double(b)) : .int(r.partialValue)
            case "/":
                guard b != 0 else { throw PyError("ZeroDivisionError", "division by zero") }
                return .float(Double(a) / Double(b))
            case "//":
                guard b != 0 else { throw PyError("ZeroDivisionError", "integer division or modulo by zero") }
                let q = a / b
                return .int((a % b != 0 && (a < 0) != (b < 0)) ? q - 1 : q)
            case "%":
                guard b != 0 else { throw PyError("ZeroDivisionError", "integer modulo by zero") }
                let r = a % b
                return .int(r != 0 && (r < 0) != (b < 0) ? r + b : r)
            case "**":
                if b < 0 { return .float(pow(Double(a), Double(b))) }
                var r = 1, base = a, e = b
                while e > 0 {
                    if e & 1 == 1 {
                        let m = r.multipliedReportingOverflow(by: base)
                        if m.overflow { return .float(pow(Double(a), Double(b))) }
                        r = m.partialValue
                    }
                    e >>= 1
                    if e > 0 {
                        let m = base.multipliedReportingOverflow(by: base)
                        if m.overflow { return .float(pow(Double(a), Double(b))) }
                        base = m.partialValue
                    }
                }
                return .int(r)
            case "&": return .int(a & b)
            case "|": return .int(a | b)
            case "^": return .int(a ^ b)
            case "<<": return .int(a << b)
            case ">>": return .int(a >> b)
            default: break
            }
        }
        if let a = num(x), let b = num(y) {
            switch op {
            case "+": return .float(a + b)
            case "-": return .float(a - b)
            case "*": return .float(a * b)
            case "/":
                guard b != 0 else { throw PyError("ZeroDivisionError", "float division by zero") }
                return .float(a / b)
            case "//":
                guard b != 0 else { throw PyError("ZeroDivisionError", "float floor division by zero") }
                return .float((a / b).rounded(.down))
            case "%":
                guard b != 0 else { throw PyError("ZeroDivisionError", "float modulo") }
                let r = a.truncatingRemainder(dividingBy: b)
                return .float(r != 0 && (r < 0) != (b < 0) ? r + b : r)
            case "**": return .float(pow(a, b))
            default: break
            }
        }
        switch (op, x, y) {
        case ("+", .str(let a), .str(let b)): return .str(a + b)
        case ("+", .lista(let a), .lista(let b)): return .lista(PyLista(a.v + b.v))
        case ("+", .tupla(let a), .tupla(let b)): return .tupla(a + b)
        case ("*", .str(let s), _), ("*", _, .str(let s)):
            guard let n = entero(op == "*" && entero(y) != nil ? y : x) else { break }
            return .str(n > 0 ? String(repeating: s, count: n) : "")
        case ("*", .lista(let l), _), ("*", _, .lista(let l)):
            guard let n = entero(entero(y) != nil ? y : x) else { break }
            var r: [PyValor] = []
            for _ in 0 ..< max(0, n) { r += l.v }
            return .lista(PyLista(r))
        case ("%", .str(let s), _):
            return .str(try formatoPorcentaje(s, y))
        case ("|", .dicc(let a), .dicc(let b)):
            let d = PyDicc()
            for k in a.claves { d.set(k, a.get(k)!) }
            for k in b.claves { d.set(k, b.get(k)!) }
            return .dicc(d)
        case ("|", .conjunto(let a), .conjunto(let b)):
            let d = PyDicc(); for k in a.claves + b.claves { d.set(k, .none) }; return .conjunto(d)
        case ("&", .conjunto(let a), .conjunto(let b)):
            let d = PyDicc(); for k in a.claves where b.get(k) != nil { d.set(k, .none) }; return .conjunto(d)
        case ("-", .conjunto(let a), .conjunto(let b)):
            let d = PyDicc(); for k in a.claves where b.get(k) == nil { d.set(k, .none) }; return .conjunto(d)
        default: break
        }
        throw PyError("TypeError", "unsupported operand type(s) for \(op): '\(x.tipo)' and '\(y.tipo)'")
    }

    func iguales(_ x: PyValor, _ y: PyValor) -> Bool {
        if let a = num(x), let b = num(y) { return a == b }
        switch (x, y) {
        case (.none, .none): return true
        case (.str(let a), .str(let b)): return a == b
        case (.lista(let a), .lista(let b)): return a.v.count == b.v.count && zip(a.v, b.v).allSatisfy { iguales($0, $1) }
        case (.tupla(let a), .tupla(let b)): return a.count == b.count && zip(a, b).allSatisfy { iguales($0, $1) }
        case (.dicc(let a), .dicc(let b)):
            return a.claves.count == b.claves.count && a.claves.allSatisfy { k in b.get(k).map { iguales(a.get(k)!, $0) } ?? false }
        case (.conjunto(let a), .conjunto(let b)): return a.claves.count == b.claves.count && a.claves.allSatisfy { b.get($0) != nil }
        case (.objeto(let a), .objeto(let b)):
            if case .funcion(let f)? = a.clase.busca("__eq__"), let r = try? llamaFuncion(f, [x, y], [:]) { return r.verdad }
            return a === b
        case (.clase(let a), .clase(let b)): return a === b
        case (.builtin(let a), .builtin(let b)): return a === b
        case (.funcion(let a), .funcion(let b)): return a === b
        default: return false
        }
    }

    func menor(_ x: PyValor, _ y: PyValor) throws -> Bool {
        if let a = num(x), let b = num(y) { return a < b }
        switch (x, y) {
        case (.str(let a), .str(let b)): return a.unicodeScalars.lexicographicallyPrecedes(b.unicodeScalars)
        case (.lista(let a), .lista(let b)): return try secuenciaMenor(a.v, b.v)
        case (.tupla(let a), .tupla(let b)): return try secuenciaMenor(a, b)
        case (.objeto(let o), _):
            if case .funcion(let f)? = o.clase.busca("__lt__") { return try llamaFuncion(f, [x, y], [:]).verdad }
        default: break
        }
        throw PyError("TypeError", "'<' not supported between instances of '\(x.tipo)' and '\(y.tipo)'")
    }

    private func secuenciaMenor(_ a: [PyValor], _ b: [PyValor]) throws -> Bool {
        for (x, y) in zip(a, b) where !iguales(x, y) { return try menor(x, y) }
        return a.count < b.count
    }

    private func compara(_ op: String, _ x: PyValor, _ y: PyValor) throws -> Bool {
        switch op {
        case "==": return iguales(x, y)
        case "!=": return !iguales(x, y)
        case "<": return try menor(x, y)
        case ">": return try menor(y, x)
        case "<=": return try !menor(y, x)
        case ">=": return try !menor(x, y)
        case "in": return try contiene(y, x)
        case "not in": return try !contiene(y, x)
        case "is":
            switch (x, y) {
            case (.none, .none): return true
            case (.bool(let a), .bool(let b)): return a == b
            case (.lista(let a), .lista(let b)): return a === b
            case (.dicc(let a), .dicc(let b)): return a === b
            case (.objeto(let a), .objeto(let b)): return a === b
            default: return false
            }
        case "is not": return try !compara("is", x, y)
        default: throw PyError("SyntaxError", "comparación desconocida \(op)")
        }
    }

    private func contiene(_ c: PyValor, _ x: PyValor) throws -> Bool {
        switch c {
        case .str(let s):
            guard case .str(let t) = x else { throw PyError("TypeError", "'in <string>' requires string as left operand") }
            return t.isEmpty || s.contains(t)
        case .lista(let l): return l.v.contains { iguales($0, x) }
        case .tupla(let t): return t.contains { iguales($0, x) }
        case .dicc(let d), .conjunto(let d): return d.get(x) != nil
        default: return try itera(c).contains { iguales($0, x) }
        }
    }

    // ----- índices y rebanadas -----

    private func indice(_ i: PyValor, _ n: Int) throws -> Int {
        guard let k = entero(i) else { throw PyError("TypeError", "los índices deben ser enteros, no '\(i.tipo)'") }
        let r = k < 0 ? n + k : k
        guard r >= 0, r < n else { throw PyError("IndexError", "index out of range") }
        return r
    }

    private func limite(_ i: PyValor, _ n: Int) throws -> Int {
        guard let k = entero(i) else { throw PyError("TypeError", "los índices deben ser enteros") }
        return min(n, max(0, k < 0 ? n + k : k))
    }

    private func subindice(_ v: PyValor, _ i: PyValor) throws -> PyValor {
        switch v {
        case .lista(let l): return l.v[try indice(i, l.v.count)]
        case .tupla(let t): return t[try indice(i, t.count)]
        case .str(let s):
            let c = Array(s)
            return .str(String(c[try indice(i, c.count)]))
        case .dicc(let d):
            guard let r = d.get(i) else { throw PyError("KeyError", repr(i)) }
            return r
        case .objeto(let o):
            if case .funcion(let f)? = o.clase.busca("__getitem__") { return try llamaFuncion(f, [v, i], [:]) }
        default: break
        }
        throw PyError("TypeError", "'\(v.tipo)' object is not subscriptable")
    }

    private func rebanada(_ v: PyValor, _ ini: PyValor?, _ fin: PyValor?, _ paso: PyValor?) throws -> PyValor {
        let elems: [PyValor]
        switch v {
        case .lista(let l): elems = l.v
        case .tupla(let t): elems = t
        case .str(let s): elems = s.map { .str(String($0)) }
        default: throw PyError("TypeError", "'\(v.tipo)' no admite rebanadas")
        }
        let n = elems.count
        let p = try paso.flatMap { if case .none = $0 { return nil }; return $0 }.map { (x: PyValor) -> Int in
            guard let k = entero(x), k != 0 else { throw PyError("ValueError", "el paso no puede ser 0") }
            return k
        } ?? 1
        func lim(_ x: PyValor?, _ def: Int) throws -> Int {
            guard let x, entero(x) != nil else { return def }
            let k = entero(x)!
            if p > 0 { return min(n, max(0, k < 0 ? n + k : k)) }
            return min(n - 1, max(-1, k < 0 ? n + k : k))
        }
        var r: [PyValor] = []
        if p > 0 {
            var i = try lim(ini, 0)
            let f = try lim(fin, n)
            while i < f { r.append(elems[i]); i += p }
        } else {
            var i = try lim(ini, n - 1)
            let f = try lim(fin, -1)
            while i > f { r.append(elems[i]); i += p }
        }
        switch v {
        case .str: return .str(r.map { if case .str(let s) = $0 { return s }; return "" }.joined())
        case .tupla: return .tupla(r)
        default: return .lista(PyLista(r))
        }
    }

    func itera(_ v: PyValor) throws -> [PyValor] {
        switch v {
        case .lista(let l): return l.v
        case .tupla(let t): return t
        case .str(let s): return s.map { .str(String($0)) }
        case .dicc(let d), .conjunto(let d): return d.claves
        case .archivo(let f): return f.contenido.split(separator: "\n", omittingEmptySubsequences: false).dropLast(f.contenido.hasSuffix("\n") ? 1 : 0).map { .str(String($0) + "\n") }
        case .objeto(let o):
            if case .funcion(let fn)? = o.clase.busca("__iter__") { return try itera(try llamaFuncion(fn, [v], [:])) }
        default: break
        }
        throw PyError("TypeError", "'\(v.tipo)' object is not iterable")
    }

    // ----- texto -----

    func str(_ v: PyValor) -> String {
        switch v {
        case .str(let s): return s
        case .objeto(let o):
            if case .funcion(let f)? = o.clase.busca("__str__"), let r = try? llamaFuncion(f, [v], [:]), case .str(let s) = r { return s }
            if o.clase.nombre.hasSuffix("Error") || o.clase.nombre.hasSuffix("Exception"), case .tupla(let t)? = o.atributos["args"] {
                return t.count == 1 ? str(t[0]) : repr(.tupla(t))
            }
            return repr(v)
        default: return repr(v)
        }
    }

    func repr(_ v: PyValor) -> String {
        switch v {
        case .none: return "None"
        case .bool(let b): return b ? "True" : "False"
        case .int(let i): return String(i)
        case .float(let d): return PyInterprete.flotante(d)
        case .str(let s):
            let comilla = s.contains("'") && !s.contains("\"") ? "\"" : "'"
            var e = s.replacingOccurrences(of: "\\", with: "\\\\").replacingOccurrences(of: "\n", with: "\\n").replacingOccurrences(of: "\t", with: "\\t")
            if comilla == "'" { e = e.replacingOccurrences(of: "'", with: "\\'") }
            return comilla + e + comilla
        case .lista(let l): return "[" + l.v.map(repr).joined(separator: ", ") + "]"
        case .tupla(let t): return "(" + t.map(repr).joined(separator: ", ") + (t.count == 1 ? ",)" : ")")
        case .dicc(let d): return "{" + d.claves.map { repr($0) + ": " + repr(d.get($0)!) }.joined(separator: ", ") + "}"
        case .conjunto(let d): return d.claves.isEmpty ? "set()" : "{" + d.claves.map(repr).joined(separator: ", ") + "}"
        case .funcion(let f): return "<function \(f.nombre)>"
        case .metodo(_, let f): return "<bound method \(f.nombre)>"
        case .builtin(let b): return "<built-in function \(b.nombre)>"
        case .clase(let c): return "<class '\(c.nombre)'>"
        case .objeto(let o):
            if case .funcion(let f)? = o.clase.busca("__repr__"), let r = try? llamaFuncion(f, [v], [:]), case .str(let s) = r { return s }
            return "<\(o.clase.nombre) object>"
        case .modulo(let n, _): return "<module '\(n)'>"
        case .archivo(let f): return "<_io.TextIOWrapper name='\(f.url.lastPathComponent)' mode='\(f.modo)'>"
        }
    }

    static func flotante(_ d: Double) -> String {
        if d.isNaN { return "nan" }
        if d.isInfinite { return d > 0 ? "inf" : "-inf" }
        if d == d.rounded(), abs(d) < 1e16 { return String(format: "%.1f", d) }
        var s = "\(d)"
        if s.contains("e") {           // 1e-05 como Python
            let p = s.split(separator: "e")
            var exp = String(p[1])
            let signo = exp.hasPrefix("-") ? "-" : "+"
            exp = exp.trimmingCharacters(in: CharacterSet(charactersIn: "+-"))
            if exp.count < 2 { exp = "0" + exp }
            s = "\(p[0])e\(signo)\(exp)"
            if s.hasPrefix("1.0e") || p[0].hasSuffix(".0") { s = s.replacingOccurrences(of: ".0e", with: "e") }
        }
        return s
    }

    /// Formato de f-strings y format(): .2f  >10  <5  ^7  05d  ,  %  x  b
    func formatea(_ v: PyValor, _ spec: String) throws -> String {
        if spec.isEmpty { return str(v) }
        var s = Array(spec)
        var relleno: Character = " "
        var alinea: Character? = nil
        if s.count >= 2, "<>^".contains(s[1]) { relleno = s[0]; alinea = s[1]; s.removeFirst(2) }
        else if let f = s.first, "<>^".contains(f) { alinea = f; s.removeFirst() }
        var signo = false
        if s.first == "+" { signo = true; s.removeFirst() }
        if s.first == "0", alinea == nil { relleno = "0"; alinea = "="; s.removeFirst() }
        var ancho = 0
        while let c = s.first, c.isNumber { ancho = ancho * 10 + Int(String(c))!; s.removeFirst() }
        var miles = false
        if s.first == "," || s.first == "_" { miles = true; s.removeFirst() }
        var precision: Int? = nil
        if s.first == "." {
            s.removeFirst()
            var p = 0
            while let c = s.first, c.isNumber { p = p * 10 + Int(String(c))!; s.removeFirst() }
            precision = p
        }
        let tipo = s.first
        var cuerpo: String
        switch tipo {
        case "f", "F":
            guard let d = num(v) else { throw PyError("ValueError", "formato 'f' para '\(v.tipo)'") }
            cuerpo = String(format: "%.\(precision ?? 6)f", d)
        case "e":
            guard let d = num(v) else { throw PyError("ValueError", "formato 'e' para '\(v.tipo)'") }
            cuerpo = String(format: "%.\(precision ?? 6)e", d)
        case "%":
            guard let d = num(v) else { throw PyError("ValueError", "formato '%' para '\(v.tipo)'") }
            cuerpo = String(format: "%.\(precision ?? 6)f", d * 100) + "%"
        case "d":
            guard let i = entero(v) else { throw PyError("ValueError", "formato 'd' para '\(v.tipo)'") }
            cuerpo = String(i)
        case "x": cuerpo = String(entero(v) ?? 0, radix: 16)
        case "X": cuerpo = String(entero(v) ?? 0, radix: 16).uppercased()
        case "b": cuerpo = String(entero(v) ?? 0, radix: 2)
        case "o": cuerpo = String(entero(v) ?? 0, radix: 8)
        case "s": cuerpo = str(v)
        default:
            if let p = precision, let d = num(v), entero(v) == nil { cuerpo = String(format: "%.\(p)g", d) }
            else if let p = precision, case .str(let t) = v { cuerpo = String(t.prefix(p)) }
            else { cuerpo = str(v) }
        }
        if miles, let punto = cuerpo.firstIndex(where: { $0 == "." }) ?? Optional(cuerpo.endIndex) {
            let ent = String(cuerpo[cuerpo.startIndex ..< punto])
            let neg = ent.hasPrefix("-")
            let digitos = Array(neg ? String(ent.dropFirst()) : ent)
            var agrupado = ""
            for (i, ch) in digitos.enumerated() {
                if i > 0, (digitos.count - i) % 3 == 0 { agrupado.append(",") }
                agrupado.append(ch)
            }
            cuerpo = (neg ? "-" : "") + agrupado + String(cuerpo[punto...])
        }
        if signo, num(v).map({ $0 >= 0 }) == true { cuerpo = "+" + cuerpo }
        if cuerpo.count < ancho {
            let falta = ancho - cuerpo.count
            let numerico = num(v) != nil && entero(v) != nil || tipo == "f" || tipo == "d"
            switch alinea ?? (numerico ? ">" : "<") {
            case "<": cuerpo += String(repeating: relleno, count: falta)
            case "^": cuerpo = String(repeating: relleno, count: falta / 2) + cuerpo + String(repeating: relleno, count: falta - falta / 2)
            case "=":
                if cuerpo.hasPrefix("-") { cuerpo = "-" + String(repeating: relleno, count: falta) + cuerpo.dropFirst() }
                else { cuerpo = String(repeating: relleno, count: falta) + cuerpo }
            default: cuerpo = String(repeating: relleno, count: falta) + cuerpo
            }
        }
        return cuerpo
    }

    private func formatoPorcentaje(_ s: String, _ y: PyValor) throws -> String {
        var vals: [PyValor]
        if case .tupla(let t) = y { vals = t } else { vals = [y] }
        var out = ""
        let c = Array(s)
        var i = 0
        while i < c.count {
            if c[i] == "%", i + 1 < c.count {
                if c[i + 1] == "%" { out.append("%"); i += 2; continue }
                var j = i + 1
                var spec = ""
                while j < c.count, !"sdfriexX".contains(c[j]) { spec.append(c[j]); j += 1 }
                guard j < c.count, !vals.isEmpty else { throw PyError("TypeError", "faltan argumentos para el formato") }
                let v = vals.removeFirst()
                let t = c[j]
                switch t {
                case "s": out += try formatea(.str(str(v)), spec)
                case "r": out += repr(v)
                case "i": out += try formatea(v, spec + "d")
                default: out += try formatea(v, spec + String(t))
                }
                i = j + 1
            } else { out.append(c[i]); i += 1 }
        }
        return out
    }

    // ----- atributos y métodos -----

    private func atributo(_ v: PyValor, _ n: String) throws -> PyValor {
        switch v {
        case .modulo(let m, let d):
            guard let r = d.get(.str(n)) else { throw PyError("AttributeError", "module '\(m)' has no attribute '\(n)'") }
            return r
        case .objeto(let o):
            if let r = o.atributos[n] { return r }
            if let r = o.clase.busca(n) {
                if case .funcion(let f) = r { return .metodo(v, f) }
                return r
            }
            throw PyError("AttributeError", "'\(o.clase.nombre)' object has no attribute '\(n)'")
        case .clase(let c):
            guard let r = c.busca(n) else { throw PyError("AttributeError", "type object '\(c.nombre)' has no attribute '\(n)'") }
            return r
        case .str(let s) where n == "__len__": return .int(s.count)
        default:
            // métodos de tipos básicos como valor: l.append
            let yo = self
            return .builtin(PyBuiltin(n) { args, kw in
                guard let r = try yo.metodo(v, n, args, kw) else {
                    throw PyError("AttributeError", "'\(v.tipo)' object has no attribute '\(n)'")
                }
                return r
            })
        }
    }

    /// Métodos de str, list, dict, set y archivos. nil = no es de estos.
    private func metodo(_ v: PyValor, _ n: String, _ a: [PyValor], _ kw: [String: PyValor]) throws -> PyValor? {
        func arg(_ i: Int) throws -> PyValor {
            guard i < a.count else { throw PyError("TypeError", "\(n)() necesita más argumentos") }
            return a[i]
        }
        func texto(_ i: Int) throws -> String {
            guard case .str(let s) = try arg(i) else { throw PyError("TypeError", "\(n)() necesita texto") }
            return s
        }
        switch v {
        case .str(let s):
            switch n {
            case "upper": return .str(s.uppercased())
            case "lower": return .str(s.lowercased())
            case "title": return .str(s.capitalized)
            case "capitalize": return .str(s.prefix(1).uppercased() + s.dropFirst().lowercased())
            case "swapcase": return .str(String(s.map { $0.isUppercase ? Character($0.lowercased()) : Character($0.uppercased()) }))
            case "strip", "lstrip", "rstrip":
                let cs = a.isEmpty ? CharacterSet.whitespacesAndNewlines : CharacterSet(charactersIn: try texto(0))
                var r = Substring(s)
                if n != "rstrip" { while let f = r.unicodeScalars.first, cs.contains(f) { r = r.dropFirst() } }
                if n != "lstrip" { while let l = r.unicodeScalars.last, cs.contains(l) { r = r.dropLast() } }
                return .str(String(r))
            case "split", "rsplit":
                let max = a.count > 1 ? (entero(a[1]) ?? -1) : (entero(kw["maxsplit"] ?? .int(-1)) ?? -1)
                if a.isEmpty || { if case .none = a[0] { return true }; return false }() {
                    var partes: [String] = s.split(whereSeparator: { $0.isWhitespace }).map(String.init)
                    if max >= 0, partes.count > max + 1 {
                        let resto: String = partes.dropFirst(max).joined(separator: " ")
                        partes = Array(partes.prefix(max))
                        partes.append(resto)
                    }
                    return .lista(PyLista(partes.map { PyValor.str($0) }))
                }
                let sep = try texto(0)
                guard !sep.isEmpty else { throw PyError("ValueError", "empty separator") }
                var partes = s.components(separatedBy: sep)
                if max >= 0, partes.count > max + 1 {
                    // (en pasos: como una sola expresión tardaba segundos en compilar)
                    var nuevas: [String] = []
                    if n == "split" {
                        nuevas = Array(partes.prefix(max))
                        let resto: String = partes.dropFirst(max).joined(separator: sep)
                        nuevas.append(resto)
                    } else {
                        let resto: String = partes.dropLast(max).joined(separator: sep)
                        nuevas = [resto]
                        nuevas.append(contentsOf: partes.suffix(max))
                    }
                    partes = nuevas
                }
                return .lista(PyLista(partes.map { .str($0) }))
            case "splitlines":
                var ls: [String] = s.components(separatedBy: "\n")
                if s.hasSuffix("\n") { ls.removeLast() }
                return .lista(PyLista(ls.map { PyValor.str($0) }))
            case "join": return .str(try itera(try arg(0)).map { x -> String in
                guard case .str(let t) = x else { throw PyError("TypeError", "join() necesita textos, hay '\(x.tipo)'") }
                return t
            }.joined(separator: s))
            case "replace": return .str(s.replacingOccurrences(of: try texto(0), with: try texto(1)))
            case "startswith":
                if case .tupla(let t) = try arg(0) { return .bool(t.contains { if case .str(let x) = $0 { return s.hasPrefix(x) }; return false }) }
                return .bool(s.hasPrefix(try texto(0)))
            case "endswith":
                if case .tupla(let t) = try arg(0) { return .bool(t.contains { if case .str(let x) = $0 { return s.hasSuffix(x) }; return false }) }
                return .bool(s.hasSuffix(try texto(0)))
            case "find", "index":
                let t = try texto(0)
                if let r = s.range(of: t) { return .int(s.distance(from: s.startIndex, to: r.lowerBound)) }
                if n == "index" { throw PyError("ValueError", "substring not found") }
                return .int(-1)
            case "rfind":
                let t = try texto(0)
                if let r = s.range(of: t, options: .backwards) { return .int(s.distance(from: s.startIndex, to: r.lowerBound)) }
                return .int(-1)
            case "count":
                let t = try texto(0)
                return .int(t.isEmpty ? s.count + 1 : s.components(separatedBy: t).count - 1)
            case "isdigit", "isnumeric", "isdecimal": return .bool(!s.isEmpty && s.allSatisfy { $0.isNumber })
            case "isalpha": return .bool(!s.isEmpty && s.allSatisfy { $0.isLetter })
            case "isalnum": return .bool(!s.isEmpty && s.allSatisfy { $0.isLetter || $0.isNumber })
            case "isspace": return .bool(!s.isEmpty && s.allSatisfy { $0.isWhitespace })
            case "isupper": return .bool(s.contains { $0.isLetter } && s == s.uppercased())
            case "islower": return .bool(s.contains { $0.isLetter } && s == s.lowercased())
            case "zfill":
                let w = entero(try arg(0)) ?? 0
                let neg = s.hasPrefix("-")
                let cuerpo = neg ? String(s.dropFirst()) : s
                return .str((neg ? "-" : "") + String(repeating: "0", count: max(0, w - s.count)) + cuerpo)
            case "center", "ljust", "rjust":
                let w = entero(try arg(0)) ?? 0
                let f = a.count > 1 ? (try texto(1)) : " "
                let falta = max(0, w - s.count)
                switch n {
                case "ljust": return .str(s + String(repeating: f, count: falta))
                case "rjust": return .str(String(repeating: f, count: falta) + s)
                default: return .str(String(repeating: f, count: falta / 2) + s + String(repeating: f, count: falta - falta / 2))
                }
            case "format":
                var auto = 0
                var out = ""
                let c = Array(s)
                var i = 0
                while i < c.count {
                    if c[i] == "{" {
                        if i + 1 < c.count, c[i + 1] == "{" { out.append("{"); i += 2; continue }
                        var j = i + 1
                        var dentro = ""
                        while j < c.count, c[j] != "}" { dentro.append(c[j]); j += 1 }
                        let partes = dentro.split(separator: ":", maxSplits: 1).map(String.init)
                        let campo = partes.first ?? ""
                        let spec = partes.count > 1 ? partes[1] : ""
                        let val: PyValor
                        if campo.isEmpty { val = try arg(auto); auto += 1 }
                        else if let k = Int(campo) { val = try arg(k) }
                        else if let x = kw[campo] { val = x }
                        else { throw PyError("KeyError", "'\(campo)'") }
                        out += try formatea(val, spec)
                        i = j + 1
                    } else if c[i] == "}", i + 1 < c.count, c[i + 1] == "}" { out.append("}"); i += 2 }
                    else { out.append(c[i]); i += 1 }
                }
                return .str(out)
            case "encode": return .str(s)
            default: return nil
            }

        case .lista(let l):
            switch n {
            case "append": l.v.append(try arg(0)); return PyValor.none
            case "extend": l.v += try itera(try arg(0)); return PyValor.none
            case "insert":
                let i = entero(try arg(0)) ?? 0
                let pos = i < 0 ? max(0, l.v.count + i) : min(i, l.v.count)
                l.v.insert(try arg(1), at: pos); return PyValor.none
            case "pop":
                guard !l.v.isEmpty else { throw PyError("IndexError", "pop from empty list") }
                if a.isEmpty { return l.v.removeLast() }
                return l.v.remove(at: try indice(a[0], l.v.count))
            case "remove":
                guard let i = l.v.firstIndex(where: { iguales($0, a.first ?? PyValor.none) }) else { throw PyError("ValueError", "list.remove(x): x not in list") }
                l.v.remove(at: i); return PyValor.none
            case "index":
                guard let i = l.v.firstIndex(where: { iguales($0, a.first ?? PyValor.none) }) else { throw PyError("ValueError", "\(repr(a.first ?? PyValor.none)) is not in list") }
                return .int(i)
            case "count": return .int(l.v.filter { iguales($0, a.first ?? PyValor.none) }.count)
            case "reverse": l.v.reverse(); return PyValor.none
            case "clear": l.v.removeAll(); return PyValor.none
            case "copy": return .lista(PyLista(l.v))
            case "sort":
                l.v = try ordena(l.v, kw["key"], kw["reverse"]?.verdad ?? false)
                return PyValor.none
            default: return nil
            }

        case .tupla(let t):
            switch n {
            case "index":
                guard let i = t.firstIndex(where: { iguales($0, a.first ?? PyValor.none) }) else { throw PyError("ValueError", "tuple.index(x): x not in tuple") }
                return .int(i)
            case "count": return .int(t.filter { iguales($0, a.first ?? PyValor.none) }.count)
            default: return nil
            }

        case .dicc(let d):
            switch n {
            case "keys": return .lista(PyLista(d.claves))
            case "values": return .lista(PyLista(d.claves.map { d.get($0)! }))
            case "items": return .lista(PyLista(d.claves.map { .tupla([$0, d.get($0)!]) }))
            case "get": return d.get(try arg(0)) ?? (a.count > 1 ? a[1] : PyValor.none)
            case "pop":
                if let v = d.quita(try arg(0)) { return v }
                if a.count > 1 { return a[1] }
                throw PyError("KeyError", repr(a[0]))
            case "setdefault":
                if let v = d.get(try arg(0)) { return v }
                let v = a.count > 1 ? a[1] : .none
                d.set(a[0], v); return v
            case "update":
                if let o = a.first {
                    if case .dicc(let e) = o { for k in e.claves { d.set(k, e.get(k)!) } }
                    else { for par in try itera(o) { let kv = try itera(par); if kv.count == 2 { d.set(kv[0], kv[1]) } } }
                }
                for (k, v) in kw { d.set(.str(k), v) }
                return PyValor.none
            case "copy":
                let e = PyDicc(); for k in d.claves { e.set(k, d.get(k)!) }; return .dicc(e)
            case "clear": d.claves.removeAll(); d.mapa.removeAll(); return PyValor.none
            default: return nil
            }

        case .conjunto(let d):
            switch n {
            case "add": d.set(try arg(0), .none); return PyValor.none
            case "remove":
                guard d.quita(try arg(0)) != nil else { throw PyError("KeyError", repr(a[0])) }
                return PyValor.none
            case "discard": d.quita(try arg(0)); return PyValor.none
            case "union": return try binaria("|", v, .conjunto(conjunto(try itera(try arg(0)))))
            case "intersection": return try binaria("&", v, .conjunto(conjunto(try itera(try arg(0)))))
            case "difference": return try binaria("-", v, .conjunto(conjunto(try itera(try arg(0)))))
            default: return nil
            }

        case .archivo(let f):
            switch n {
            case "read":
                let r = String(f.contenido[f.pos...]); f.pos = f.contenido.endIndex; return .str(r)
            case "readline":
                guard f.pos < f.contenido.endIndex else { return .str("") }
                let fin = f.contenido[f.pos...].firstIndex(of: "\n").map { f.contenido.index(after: $0) } ?? f.contenido.endIndex
                let r = String(f.contenido[f.pos ..< fin]); f.pos = fin; return .str(r)
            case "readlines": return .lista(PyLista(try itera(v)))
            case "write":
                guard f.modo.contains("w") || f.modo.contains("a") else { throw PyError("io.UnsupportedOperation", "not writable") }
                let t = str(try arg(0)); f.contenido += t; return .int(t.count)
            case "close": cierra(f); return PyValor.none
            default: return nil
            }

        default:
            return nil
        }
    }

    private func conjunto(_ vs: [PyValor]) -> PyDicc { let d = PyDicc(); for v in vs { d.set(v, .none) }; return d }

    func ordena(_ vs: [PyValor], _ clave: PyValor?, _ inverso: Bool) throws -> [PyValor] {
        var pares: [(PyValor, PyValor)] = []
        for v in vs {
            if let k = clave, !({ if case .none = k { return true }; return false }()) { pares.append((try llama(k, [v]), v)) }
            else { pares.append((v, v)) }
        }
        var fallo: Error? = nil
        // orden estable, como Python
        let ordenados = pares.enumerated().sorted { x, y in
            do {
                if try menor(x.element.0, y.element.0) { return !inverso }
                if try menor(y.element.0, x.element.0) { return inverso }
            } catch { fallo = error }
            return x.offset < y.offset
        }
        if let f = fallo { throw f }
        return ordenados.map { $0.element.1 }
    }

    private func cierra(_ f: PyArchivo) {
        guard !f.cerrado else { return }
        f.cerrado = true
        if f.modo.contains("w") || f.modo.contains("a") {
            try? FileManager.default.createDirectory(at: f.url.deletingLastPathComponent(), withIntermediateDirectories: true)
            try? f.contenido.write(to: f.url, atomically: true, encoding: .utf8)
        }
    }

    // ----- módulos -----

    private var cacheModulos: [String: PyValor] = [:]

    private func modulo(_ nombre: String) throws -> PyValor {
        if let m = cacheModulos[nombre] { return m }
        let d = PyDicc()
        func f(_ n: String, _ fn: @escaping PyNativa) { d.set(.str(n), .builtin(PyBuiltin(n, fn))) }
        func numero(_ a: [PyValor], _ i: Int) throws -> Double {
            guard i < a.count, let x = num(a[i]) else { throw PyError("TypeError", "se esperaba un número") }
            return x
        }
        switch nombre {
        case "math":
            d.set(.str("pi"), .float(Double.pi)); d.set(.str("e"), .float(M_E))
            d.set(.str("inf"), .float(.infinity)); d.set(.str("nan"), .float(.nan)); d.set(.str("tau"), .float(2 * Double.pi))
            let unarias: [(String, (Double) -> Double)] = [("sqrt", { $0.squareRoot() }), ("sin", sin), ("cos", cos), ("tan", tan),
                ("asin", asin), ("acos", acos), ("atan", atan), ("exp", exp), ("fabs", fabs), ("log10", log10), ("log2", log2),
                ("degrees", { $0 * 180 / .pi }), ("radians", { $0 * .pi / 180 })]
            for (n, g) in unarias {
                f(n) { a, _ in
                    let x = try numero(a, 0)
                    if n == "sqrt", x < 0 { throw PyError("ValueError", "math domain error") }
                    return .float(g(x))
                }
            }
            f("log") { a, _ in
                let x = try numero(a, 0)
                guard x > 0 else { throw PyError("ValueError", "math domain error") }
                return .float(a.count > 1 ? logNatural(x) / logNatural(try numero(a, 1)) : logNatural(x))
            }
            f("pow") { a, _ in .float(pow(try numero(a, 0), try numero(a, 1))) }
            f("atan2") { a, _ in .float(atan2(try numero(a, 0), try numero(a, 1))) }
            f("hypot") { a, _ in .float(hypot(try numero(a, 0), try numero(a, 1))) }
            f("floor") { a, _ in .int(Int((try numero(a, 0)).rounded(.down))) }
            f("ceil") { a, _ in .int(Int((try numero(a, 0)).rounded(.up))) }
            f("trunc") { a, _ in .int(Int(try numero(a, 0))) }
            f("isclose") { a, _ in .bool(abs(try numero(a, 0) - (try numero(a, 1))) <= 1e-9 * max(abs(try numero(a, 0)), abs(try numero(a, 1)))) }
            f("factorial") { a, _ in
                guard let n = self.entero(a.first ?? .none), n >= 0 else { throw PyError("ValueError", "factorial() not defined for negative values") }
                var r = 1
                for i in stride(from: 2, through: n, by: 1) {
                    let m = r.multipliedReportingOverflow(by: i)
                    if m.overflow { throw PyError("OverflowError", "factorial demasiado grande para este intérprete") }
                    r = m.partialValue
                }
                return .int(r)
            }
            f("gcd") { a, _ in
                var x = abs(self.entero(a.first ?? .none) ?? 0), y = abs(self.entero(a.count > 1 ? a[1] : .none) ?? 0)
                while y != 0 { (x, y) = (y, x % y) }
                return .int(x)
            }
            f("isqrt") { a, _ in .int(Int((try numero(a, 0)).squareRoot())) }
        case "random":
            f("random") { _, _ in .float(Double.random(in: 0 ..< 1)) }
            f("randint") { a, _ in
                guard let x = self.entero(a.first ?? .none), let y = self.entero(a.count > 1 ? a[1] : .none), x <= y else { throw PyError("ValueError", "randint(a, b) necesita a <= b") }
                return .int(Int.random(in: x ... y))
            }
            f("uniform") { a, _ in .float(Double.random(in: try numero(a, 0) ... max(try numero(a, 0), try numero(a, 1)))) }
            f("choice") { a, _ in
                let l = try self.itera(a.first ?? .none)
                guard let x = l.randomElement() else { throw PyError("IndexError", "Cannot choose from an empty sequence") }
                return x
            }
            f("shuffle") { a, _ in
                guard case .lista(let l)? = a.first else { throw PyError("TypeError", "shuffle() necesita una lista") }
                l.v.shuffle(); return .none
            }
            f("sample") { a, _ in
                let l = try self.itera(a.first ?? .none)
                let k = self.entero(a.count > 1 ? a[1] : .none) ?? 0
                guard k <= l.count else { throw PyError("ValueError", "Sample larger than population") }
                return .lista(PyLista(Array(l.shuffled().prefix(k))))
            }
            f("seed") { _, _ in .none }
        case "time":
            f("time") { _, _ in .float(Date().timeIntervalSince1970) }
            f("sleep") { a, _ in Thread.sleep(forTimeInterval: min(5, max(0, try numero(a, 0)))); return .none }
            f("perf_counter") { _, _ in .float(ProcessInfo.processInfo.systemUptime) }
            f("strftime") { a, _ in
                let fmt = self.str(a.first ?? .str(""))
                let df = DateFormatter()
                df.dateFormat = fmt.replacingOccurrences(of: "%Y", with: "yyyy").replacingOccurrences(of: "%m", with: "MM")
                    .replacingOccurrences(of: "%d", with: "dd").replacingOccurrences(of: "%H", with: "HH")
                    .replacingOccurrences(of: "%M", with: "mm").replacingOccurrences(of: "%S", with: "ss")
                return .str(df.string(from: Date()))
            }
        case "json":
            f("dumps") { a, kw in .str(try self.aJSON(a.first ?? .none, sangria: self.entero(kw["indent"] ?? .none))) }
            f("loads") { a, _ in
                guard case .str(let s)? = a.first, let dt = s.data(using: .utf8),
                      let o = try? JSONSerialization.jsonObject(with: dt, options: [.fragmentsAllowed]) else {
                    throw PyError("json.JSONDecodeError", "JSON no válido")
                }
                return self.desdeJSON(o)
            }
        case "os", "os.path":
            f("getcwd") { _, _ in .str(self.env.map { $0.vpath($0.cwd) } ?? "/") }
            f("listdir") { a, _ in
                let p = a.isEmpty ? "." : self.str(a[0])
                guard let u = try? self.env?.resolve(p), let items = try? FileManager.default.contentsOfDirectory(atPath: u.path) else {
                    throw PyError("FileNotFoundError", "No such file or directory: '\(p)'")
                }
                return .lista(PyLista(items.sorted().map { .str($0) }))
            }
            f("exists") { a, _ in
                guard let u = try? self.env?.resolve(self.str(a.first ?? .str(""))) else { return .bool(false) }
                return .bool(FileManager.default.fileExists(atPath: u.path))
            }
            f("isdir") { a, _ in
                guard let e = self.env, let u = try? e.resolve(self.str(a.first ?? .str(""))) else { return .bool(false) }
                return .bool(e.isDir(u))
            }
            f("isfile") { a, _ in
                guard let e = self.env, let u = try? e.resolve(self.str(a.first ?? .str(""))) else { return .bool(false) }
                return .bool(e.exists(u) && !e.isDir(u))
            }
            f("join") { a, _ in .str(a.map { self.str($0) }.reduce("") { $0.isEmpty ? $1 : ($1.hasPrefix("/") ? $1 : ($0.hasSuffix("/") ? $0 + $1 : $0 + "/" + $1)) }) }
            f("basename") { a, _ in .str((self.str(a.first ?? .str("")) as NSString).lastPathComponent) }
            f("dirname") { a, _ in .str((self.str(a.first ?? .str("")) as NSString).deletingLastPathComponent) }
            f("splitext") { a, _ in
                let s = self.str(a.first ?? .str(""))
                let ext = (s as NSString).pathExtension
                return .tupla([.str(ext.isEmpty ? s : String(s.dropLast(ext.count + 1))), .str(ext.isEmpty ? "" : "." + ext)])
            }
            f("mkdir") { a, _ in
                guard let u = try? self.env?.resolve(self.str(a.first ?? .str(""))) else { throw PyError("OSError", "ruta no válida") }
                try? FileManager.default.createDirectory(at: u, withIntermediateDirectories: true); return .none
            }
            f("makedirs") { a, _ in
                guard let u = try? self.env?.resolve(self.str(a.first ?? .str(""))) else { throw PyError("OSError", "ruta no válida") }
                try? FileManager.default.createDirectory(at: u, withIntermediateDirectories: true); return .none
            }
            f("remove") { a, _ in
                guard let u = try? self.env?.resolve(self.str(a.first ?? .str(""))), (try? FileManager.default.removeItem(at: u)) != nil else {
                    throw PyError("FileNotFoundError", "No such file or directory")
                }
                return .none
            }
            f("getenv") { a, _ in
                let k = self.str(a.first ?? .str(""))
                return self.env?.vars[k].map { .str($0) } ?? (a.count > 1 ? a[1] : .none)
            }
            d.set(.str("sep"), .str("/"))
            if nombre == "os" {
                cacheModulos["os"] = .modulo("os", d)
                d.set(.str("path"), try modulo("os.path"))
                let env = PyDicc()
                for (k, v) in (self.env?.vars ?? [:]).sorted(by: { $0.key < $1.key }) { env.set(.str(k), .str(v)) }
                d.set(.str("environ"), .dicc(env))
            }
        case "sys":
            d.set(.str("argv"), .lista(PyLista(argv.map { .str($0) })))
            d.set(.str("version"), .str("3.x (SwiftShell, subconjunto)"))
            d.set(.str("platform"), .str("ios"))
            d.set(.str("maxsize"), .int(Int.max))
            f("exit") { a, _ in throw PyError("SystemExit", a.first.map { self.str($0) } ?? "0") }
        default:
            throw PyError("ModuleNotFoundError", "No module named '\(nombre)' (hay: math, random, time, json, os, sys)")
        }
        let m = PyValor.modulo(nombre, d)
        cacheModulos[nombre] = m
        return m
    }

    private func aJSON(_ v: PyValor, sangria: Int?) throws -> String {
        func conv(_ v: PyValor) throws -> Any {
            switch v {
            case .none: return NSNull()
            case .bool(let b): return b
            case .int(let i): return i
            case .float(let d): return d
            case .str(let s): return s
            case .lista(let l): return try l.v.map(conv)
            case .tupla(let t): return try t.map(conv)
            case .dicc(let d):
                var o: [String: Any] = [:]
                for k in d.claves { o[str(k)] = try conv(d.get(k)!) }
                return o
            default: throw PyError("TypeError", "Object of type \(v.tipo) is not JSON serializable")
            }
        }
        // orden de claves de Python: el de inserción
        func escribe(_ v: PyValor, _ nivel: Int) throws -> String {
            let sp = sangria.map { String(repeating: " ", count: $0 * (nivel + 1)) }
            let cierre = sangria.map { "\n" + String(repeating: " ", count: $0 * nivel) } ?? ""
            let sep = sangria == nil ? ", " : ","
            switch v {
            case .lista(let l):
                if l.v.isEmpty { return "[]" }
                return "[" + (try l.v.map { (sp.map { "\n" + $0 } ?? "") + (try escribe($0, nivel + 1)) }.joined(separator: sep)) + cierre + "]"
            case .tupla(let t): return try escribe(.lista(PyLista(t)), nivel)
            case .dicc(let d):
                if d.claves.isEmpty { return "{}" }
                return "{" + (try d.claves.map { k in (sp.map { "\n" + $0 } ?? "") + (try escribe(.str(str(k)), 0)) + ": " + (try escribe(d.get(k)!, nivel + 1)) }.joined(separator: sep)) + cierre + "}"
            case .float(let x): return PyInterprete.flotante(x)
            default:
                let dt = try JSONSerialization.data(withJSONObject: try conv(v), options: [.fragmentsAllowed])
                return String(data: dt, encoding: .utf8) ?? "null"
            }
        }
        return try escribe(v, 0)
    }

    private func desdeJSON(_ o: Any) -> PyValor {
        switch o {
        case is NSNull: return .none
        case let s as String: return .str(s)
        case let n as NSNumber:
            #if canImport(Darwin)
            if CFGetTypeID(n) == CFBooleanGetTypeID() { return .bool(n.boolValue) }
            #else
            if String(cString: n.objCType) == "c" { return .bool(n.boolValue) }
            #endif
            let d = n.doubleValue
            return d == d.rounded() && !"\(n)".contains(".") && abs(d) < 9e18 ? .int(n.intValue) : .float(d)
        case let a as [Any]: return .lista(PyLista(a.map(desdeJSON)))
        case let m as [String: Any]:
            let d = PyDicc()
            for k in m.keys.sorted() { d.set(.str(k), desdeJSON(m[k]!)) }
            return .dicc(d)
        default: return .none
        }
    }

    // ----- funciones integradas -----

    private func instalaBuiltins() {
        func b(_ n: String, _ f: @escaping PyNativa) { builtins[n] = .builtin(PyBuiltin(n, f)) }
        b("print") { a, kw in
            let sep = kw["sep"].map { self.str($0) } ?? " "
            let fin = kw["end"].map { self.str($0) } ?? "\n"
            let t = a.map { self.str($0) }.joined(separator: sep) + fin
            if case .archivo(let f)? = kw["file"] { f.contenido += t } else { self.salida += t }
            if self.salida.count > 2_000_000 { throw PyError("RuntimeError", "demasiada salida") }
            return .none
        }
        b("len") { a, _ in
            switch a.first ?? .none {
            case .str(let s): return .int(s.count)
            case .lista(let l): return .int(l.v.count)
            case .tupla(let t): return .int(t.count)
            case .dicc(let d), .conjunto(let d): return .int(d.claves.count)
            case .objeto(let o):
                if case .funcion(let f)? = o.clase.busca("__len__") { return try self.llamaFuncion(f, [a[0]], [:]) }
                fallthrough
            default: throw PyError("TypeError", "object of type '\((a.first ?? .none).tipo)' has no len()")
            }
        }
        b("range") { a, _ in
            let n = a.map { self.entero($0) }
            guard !n.contains(where: { $0 == nil }), !n.isEmpty else { throw PyError("TypeError", "range() necesita enteros") }
            let (ini, fin, paso) = n.count == 1 ? (0, n[0]!, 1) : (n[0]!, n[1]!, n.count > 2 ? n[2]! : 1)
            guard paso != 0 else { throw PyError("ValueError", "range() arg 3 must not be zero") }
            let cuantos = paso > 0 ? max(0, (fin - ini + paso - 1) / paso) : max(0, (ini - fin - paso - 1) / -paso)
            guard cuantos <= 10_000_000 else { throw PyError("MemoryError", "range demasiado grande (máx 10 millones)") }
            return .lista(PyLista(stride(from: ini, to: fin, by: paso).map { .int($0) }))
        }
        b("str") { a, _ in .str(a.first.map { self.str($0) } ?? "") }
        b("repr") { a, _ in .str(self.repr(a.first ?? .none)) }
        b("int") { a, _ in
            guard let v = a.first else { return .int(0) }
            switch v {
            case .int: return v
            case .bool(let x): return .int(x ? 1 : 0)
            case .float(let d):
                guard d.isFinite else { throw PyError("ValueError", "cannot convert float \(PyInterprete.flotante(d)) to integer") }
                return .int(Int(d))
            case .str(let s):
                let t = s.trimmingCharacters(in: .whitespaces).replacingOccurrences(of: "_", with: "")
                let base = a.count > 1 ? (self.entero(a[1]) ?? 10) : 10
                guard let i = Int(t, radix: base) else { throw PyError("ValueError", "invalid literal for int() with base \(base): \(self.repr(v))") }
                return .int(i)
            default: throw PyError("TypeError", "int() no acepta '\(v.tipo)'")
            }
        }
        b("float") { a, _ in
            guard let v = a.first else { return .float(0) }
            if let d = self.num(v) { return .float(d) }
            if case .str(let s) = v {
                let t = s.trimmingCharacters(in: .whitespaces).lowercased()
                if t == "inf" || t == "infinity" { return .float(.infinity) }
                if t == "-inf" { return .float(-.infinity) }
                if t == "nan" { return .float(.nan) }
                if let d = Double(t) { return .float(d) }
            }
            throw PyError("ValueError", "could not convert string to float: \(self.repr(v))")
        }
        b("bool") { a, _ in .bool(a.first?.verdad ?? false) }
        b("list") { a, _ in .lista(PyLista(a.isEmpty ? [] : try self.itera(a[0]))) }
        b("tuple") { a, _ in .tupla(a.isEmpty ? [] : try self.itera(a[0])) }
        b("set") { a, _ in .conjunto(self.conjunto(a.isEmpty ? [] : try self.itera(a[0]))) }
        b("dict") { a, kw in
            let d = PyDicc()
            if let o = a.first {
                if case .dicc(let e) = o { for k in e.claves { d.set(k, e.get(k)!) } }
                else { for par in try self.itera(o) { let kv = try self.itera(par); if kv.count == 2 { d.set(kv[0], kv[1]) } } }
            }
            for (k, v) in kw.sorted(by: { $0.key < $1.key }) { d.set(.str(k), v) }
            return .dicc(d)
        }
        b("type") { a, _ in
            guard let v = a.first else { throw PyError("TypeError", "type() necesita 1 argumento") }
            if case .objeto(let o) = v { return .clase(o.clase) }
            return self.builtins[v.tipo] ?? .str("<class '\(v.tipo)'>")
        }
        b("isinstance") { a, _ in
            guard a.count == 2 else { throw PyError("TypeError", "isinstance() necesita 2 argumentos") }
            func es(_ t: PyValor) -> Bool {
                switch t {
                case .builtin(let bi):
                    if bi.nombre == "float", case .int = a[0] { return false }
                    if bi.nombre == "int", case .bool = a[0] { return true }
                    return a[0].tipo == bi.nombre
                case .clase(let c): if case .objeto(let o) = a[0] { return o.clase.esSubclase(de: c) }; return false
                case .tupla(let ts): return ts.contains(where: es)
                default: return false
                }
            }
            return .bool(es(a[1]))
        }
        b("abs") { a, _ in
            switch a.first ?? .none {
            case .int(let i): return .int(abs(i))
            case .float(let d): return .float(abs(d))
            case .bool(let x): return .int(x ? 1 : 0)
            default: throw PyError("TypeError", "bad operand type for abs()")
            }
        }
        for n in ["min", "max"] {
            b(n) { a, kw in
                var vs = a.count == 1 ? try self.itera(a[0]) : a
                if vs.isEmpty {
                    if let d = kw["default"] { return d }
                    throw PyError("ValueError", "\(n)() arg is an empty sequence")
                }
                vs = try self.ordena(vs, kw["key"], false)
                return n == "min" ? vs.first! : vs.last!
            }
        }
        b("sum") { a, _ in
            var r: PyValor = a.count > 1 ? a[1] : .int(0)
            for v in try self.itera(a.first ?? .lista(PyLista([]))) { r = try self.binaria("+", r, v) }
            return r
        }
        b("sorted") { a, kw in .lista(PyLista(try self.ordena(try self.itera(a.first ?? .none), kw["key"], kw["reverse"]?.verdad ?? false))) }
        b("reversed") { a, _ in .lista(PyLista(try self.itera(a.first ?? .none).reversed())) }
        b("enumerate") { a, kw in
            let ini = self.entero(a.count > 1 ? a[1] : (kw["start"] ?? .int(0))) ?? 0
            return .lista(PyLista(try self.itera(a.first ?? .none).enumerated().map { .tupla([.int($0.offset + ini), $0.element]) }))
        }
        b("zip") { a, _ in
            let ls = try a.map { try self.itera($0) }
            let n = ls.map(\.count).min() ?? 0
            return .lista(PyLista((0 ..< n).map { i in .tupla(ls.map { $0[i] }) }))
        }
        b("map") { a, _ in
            guard a.count >= 2 else { throw PyError("TypeError", "map() necesita función e iterable") }
            let ls = try a.dropFirst().map { try self.itera($0) }
            let n = ls.map(\.count).min() ?? 0
            return .lista(PyLista(try (0 ..< n).map { i in try self.llama(a[0], ls.map { $0[i] }) }))
        }
        b("filter") { a, _ in
            guard a.count == 2 else { throw PyError("TypeError", "filter() necesita función e iterable") }
            return .lista(PyLista(try self.itera(a[1]).filter { v in
                if case .none = a[0] { return v.verdad }
                return try self.llama(a[0], [v]).verdad
            }))
        }
        b("any") { a, _ in .bool(try self.itera(a.first ?? .none).contains { $0.verdad }) }
        b("all") { a, _ in .bool(try self.itera(a.first ?? .none).allSatisfy { $0.verdad }) }
        b("round") { a, _ in
            guard let d = self.num(a.first ?? .none) else { throw PyError("TypeError", "round() necesita un número") }
            if a.count > 1, let n = self.entero(a[1]) {
                let f = pow(10.0, Double(n))
                return .float((d * f).rounded(.toNearestOrEven) / f)
            }
            return .int(Int(d.rounded(.toNearestOrEven)))
        }
        b("divmod") { a, _ in
            guard a.count == 2 else { throw PyError("TypeError", "divmod() necesita 2 argumentos") }
            return .tupla([try self.binaria("//", a[0], a[1]), try self.binaria("%", a[0], a[1])])
        }
        b("pow") { a, _ in
            guard a.count >= 2 else { throw PyError("TypeError", "pow() necesita 2 argumentos") }
            let r = try self.binaria("**", a[0], a[1])
            if a.count > 2 { return try self.binaria("%", r, a[2]) }
            return r
        }
        b("ord") { a, _ in
            guard case .str(let s)? = a.first, s.count == 1, let u = s.unicodeScalars.first else { throw PyError("TypeError", "ord() necesita un carácter") }
            return .int(Int(u.value))
        }
        b("chr") { a, _ in
            guard let i = self.entero(a.first ?? .none), let u = Unicode.Scalar(UInt32(max(0, i))) else { throw PyError("ValueError", "chr() fuera de rango") }
            return .str(String(Character(u)))
        }
        b("hex") { a, _ in let i = self.entero(a.first ?? .none) ?? 0; return .str((i < 0 ? "-0x" : "0x") + String(abs(i), radix: 16)) }
        b("bin") { a, _ in let i = self.entero(a.first ?? .none) ?? 0; return .str((i < 0 ? "-0b" : "0b") + String(abs(i), radix: 2)) }
        b("oct") { a, _ in let i = self.entero(a.first ?? .none) ?? 0; return .str((i < 0 ? "-0o" : "0o") + String(abs(i), radix: 8)) }
        b("format") { a, _ in .str(try self.formatea(a.first ?? .none, a.count > 1 ? self.str(a[1]) : "")) }
        b("hash") { a, _ in .int((a.first ?? .none).clave.hashValue) }
        b("id") { a, _ in .int((a.first ?? .none).clave.hashValue & 0xffffff) }
        b("callable") { a, _ in
            switch a.first ?? .none {
            case .funcion, .builtin, .metodo, .clase: return .bool(true)
            default: return .bool(false)
            }
        }
        b("hasattr") { a, _ in
            guard a.count == 2, case .str(let n) = a[1] else { return .bool(false) }
            return .bool((try? self.atributo(a[0], n)) != nil)
        }
        b("getattr") { a, _ in
            guard a.count >= 2, case .str(let n) = a[1] else { throw PyError("TypeError", "getattr(obj, 'nombre')") }
            if let v = try? self.atributo(a[0], n) { return v }
            if a.count > 2 { return a[2] }
            throw PyError("AttributeError", "no existe el atributo '\(n)'")
        }
        b("setattr") { a, _ in
            guard a.count == 3, case .str(let n) = a[1], case .objeto(let o) = a[0] else { throw PyError("TypeError", "setattr(obj, 'nombre', valor)") }
            o.atributos[n] = a[2]; return .none
        }
        b("input") { a, _ in
            if let p = a.first { self.salida += self.str(p) }
            if !self.lineasEntrada.isEmpty { return .str(self.lineasEntrada.removeFirst()) }
            if let v = self.volcar, !self.salida.isEmpty { v(self.salida); self.salida = "" }
            guard let pide = self.pideLinea, let l = pide() else {
                throw PyError("EOFError", "EOF when reading a line")
            }
            return .str(l)
        }
        b("open") { a, kw in
            guard let p = a.first.map({ self.str($0) }) else { throw PyError("TypeError", "open() necesita el nombre del archivo") }
            let modo = a.count > 1 ? self.str(a[1]) : (kw["mode"].map { self.str($0) } ?? "r")
            guard let env = self.env, let u = try? env.resolve(p) else { throw PyError("PermissionError", "ruta fuera de tu espacio: '\(p)'") }
            var contenido = ""
            if modo.contains("r") || modo.contains("a") {
                if let t = try? String(contentsOf: u, encoding: .utf8) { contenido = t }
                else if modo.contains("r") { throw PyError("FileNotFoundError", "[Errno 2] No such file or directory: '\(p)'") }
            }
            let f = PyArchivo(url: u, modo: modo, contenido: contenido)
            if modo.contains("a") { f.pos = f.contenido.endIndex }
            return .archivo(f)
        }
        b("exit") { _, _ in throw PyError("SystemExit", "0") }
        b("quit") { _, _ in throw PyError("SystemExit", "0") }
        // jerarquía de excepciones, como en Python
        let excepcion = PyClase(nombre: "Exception", base: nil, atributos: [:])
        builtins["Exception"] = .clase(excepcion)
        builtins["BaseException"] = .clase(excepcion)
        func clase(_ n: String, _ base: PyClase) -> PyClase {
            let c = PyClase(nombre: n, base: base, atributos: [:]); builtins[n] = .clase(c); return c
        }
        let lookup = clase("LookupError", excepcion)
        let aritm = clase("ArithmeticError", excepcion)
        let oserr = clase("OSError", excepcion)
        let runtime = clase("RuntimeError", excepcion)
        _ = clase("KeyError", lookup); _ = clase("IndexError", lookup)
        _ = clase("ZeroDivisionError", aritm); _ = clase("OverflowError", aritm)
        _ = clase("FileNotFoundError", oserr); _ = clase("PermissionError", oserr)
        _ = clase("NotImplementedError", runtime); _ = clase("RecursionError", runtime)
        for e in ["ValueError", "TypeError", "NameError", "AttributeError", "StopIteration", "AssertionError",
                  "ImportError", "ModuleNotFoundError", "EOFError", "SyntaxError", "MemoryError"] {
            _ = clase(e, excepcion)
        }
    }
}

/// Logaritmo natural, fuera de cualquier clase para que 'log' sea siempre
/// la función matemática (sin depender de cómo cada plataforma la exporta).
func logNatural(_ x: Double) -> Double { log(x) }

// ---------- comandos ----------

extension Shell {

    /// Corre Python en un hilo con pila grande: la recursión de Python
    /// gasta mucha pila de Swift y los hilos de Task tienen poca.
    static func enHiloPython(_ py: PyInterprete, _ codigo: String, archivo: String, interactivo: Bool) async -> (String, String?) {
        await withCheckedContinuation { (c: CheckedContinuation<PyResultado, Never>) in
            let t = Thread {
                let r = interactivo ? py.evalua(codigo) : py.ejecuta(codigo, archivo: archivo)
                c.resume(returning: PyResultado(salida: r.salida, error: r.error))
            }
            t.stackSize = 64 << 20
            t.start()
        }.par
    }

    static func python() -> [String: Spec] {
        var c: [String: Spec] = [:]

        c["python"] = Spec(help: "python [archivo.py | -c 'código'] [args] — Python (subconjunto, sin internet ni pip)") { ctx in
            let a = ctx.args
            if a.first == "--version" || a.first == "-V" { return "Python 3 (SwiftShell, subconjunto)\n" }
            let py = PyInterprete(env: ctx.env)
            var codigo: String
            var archivo = "<stdin>"
            if a.first == "-c" {
                guard a.count >= 2 else { throw ShErr("python -c: falta el código") }
                codigo = a[1]
                py.argv = ["-c"] + Array(a.dropFirst(2))
            } else if let p = a.first {
                codigo = try ctx.input([p])
                archivo = p
                py.argv = a
                var l = ctx.stdin.components(separatedBy: "\n")
                if l.last == "" { l.removeLast() }
                py.lineasEntrada = l
                if let c = ctx.consola {
                    py.volcar = { c.escribir($0) }
                    if ctx.tecladoVivo { py.pideLinea = { c.leeLinea() } }
                }
            } else if !ctx.stdin.isEmpty {
                codigo = ctx.stdin
            } else {
                // modo interactivo
                ctx.sh.py = PyInterprete(env: ctx.env)
                ctx.sh.mode = .python
                ctx.sh.buffer = []
                return "Python 3 (SwiftShell) — escribe código; un bloque termina con una línea vacía.\nexit() o salir para volver.\n"
            }
            if codigo.hasPrefix("#!") { codigo = codigo.components(separatedBy: "\n").dropFirst().joined(separator: "\n") }
            ctx.consola?.empieza()
            let (salida, error) = await Shell.enHiloPython(py, codigo, archivo: archivo, interactivo: false)
            ctx.consola?.termina()
            if let e = error {
                if e.hasSuffix("SystemExit: 0") { return salida }
                throw ShErr(salida + e)
            }
            return salida
        }
        c["python3"] = c["python"]

        c["pip"] = Spec(help: "pip — no hay paquetes que instalar") { _ in
            "pip: este Python no instala paquetes (iOS no deja). Módulos incluidos: math, random, time, json, os, sys.\n"
        }
        c["pip3"] = c["pip"]
        return c
    }

    /// Una línea en el modo interactivo de Python.
    func lineaPython(_ line: String) async -> String {
        let t = line.trimmingCharacters(in: .whitespaces)
        if buffer.isEmpty, ["exit()", "quit()", "exit", "salir", "quit"].contains(t) {
            mode = .shell; buffer = []; py = nil
            return "de vuelta al shell\n"
        }
        buffer.append(line)
        // un bloque (línea que termina en ':' o con sangría) sigue hasta una línea vacía
        let abierto = buffer.count > 1 || t.hasSuffix(":") || t.hasSuffix("\\")
        if abierto && !t.isEmpty { return "" }
        let codigo = buffer.joined(separator: "\n")
        buffer = []
        guard !codigo.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return "" }
        let interprete = py ?? PyInterprete(env: env)
        py = interprete
        let (salida, error) = await Shell.enHiloPython(interprete, codigo, archivo: "<stdin>", interactivo: true)
        if let e = error {
            if e.contains("SystemExit") { mode = .shell; py = nil; return salida + "de vuelta al shell\n" }
            return salida + Shell.errMark + e + "\n"
        }
        return salida
    }
}

struct PyResultado: Sendable {
    let salida: String
    let error: String?
    var par: (String, String?) { (salida, error) }
}
