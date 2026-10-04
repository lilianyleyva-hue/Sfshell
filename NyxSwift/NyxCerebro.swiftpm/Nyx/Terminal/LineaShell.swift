import Foundation

// LineaShell.swift — entiende una línea de órdenes:
//   echo "hola $USER" | grep -i HOLA > salida.txt && cat salida.txt ; ls *.py
// Palabras con comillas '…' (literal) y "…" (con $VAR), \, $VAR ${VAR} $? $(orden),
// ~ al principio, tuberías |, redirecciones < > >> 2> 2>> 2>&1 &>, y && || ; (o &).

enum LineaShell {
    struct Fallo: Error {
        let mensaje: String
    }

    /// Un trozo de una palabra.
    enum Trozo {
        case literal(String, protegido: Bool)      // protegido: venía entre comillas (sin comodines)
        case variable(String)
        case orden(String)                          // $(…)
        case casa                                   // ~ al principio
    }

    struct Palabra {
        var trozos: [Trozo] = []
    }

    struct Redireccion {
        let archivo: Palabra
        let añadir: Bool
    }

    struct Orden {
        var palabras: [Palabra] = []
        var entrada: Palabra? = nil
        var salida: Redireccion? = nil
        var errores: Redireccion? = nil
        var errorASalida = false
    }

    enum Enlace { case primera, y, o, luego }

    struct Enlazada {
        let enlace: Enlace
        let tuberia: [Orden]
    }

    /// Las piezas de la línea: palabras y operadores.
    enum Pieza {
        case palabra(Palabra)
        case op(String)
    }

    // MARK: piezas

    static func piezas(_ s: String) throws -> [Pieza] {
        let cs = Array(s)
        var out: [Pieza] = []
        var i = 0
        var actual = Palabra()
        var hay = false
        var literal = ""
        func cierraLiteral(_ protegido: Bool = false) {
            if !literal.isEmpty { actual.trozos.append(.literal(literal, protegido: protegido)); literal = "" }
        }
        func cierraPalabra() {
            cierraLiteral()
            if hay { out.append(.palabra(actual)) }
            actual = Palabra()
            hay = false
        }
        while i < cs.count {
            let c = cs[i]
            if c == "#" && !hay {                    // comentario
                break
            }
            if c == " " || c == "\t" || c == "\n" {
                cierraPalabra()
                i += 1
                continue
            }
            if let (op, largo) = operador(cs, i) {
                cierraPalabra()
                out.append(.op(op))
                i += largo
                continue
            }
            hay = true
            switch c {
            case "\\":
                if i + 1 < cs.count { literal.append(cs[i + 1]); i += 2 } else { i += 1 }
                cierraLiteral(true)
            case "'":
                cierraLiteral()
                guard let fin = cs[(i + 1)...].firstIndex(of: "'") else { throw Fallo(mensaje: "falta cerrar la comilla '") }
                actual.trozos.append(.literal(String(cs[(i + 1) ..< fin]), protegido: true))
                i = fin + 1
            case "\"":
                cierraLiteral()
                i = try dobles(cs, i + 1, &actual)
            case "$":
                cierraLiteral()
                i = try dolar(cs, i, &actual)
            case "~" where literal.isEmpty && actual.trozos.isEmpty && (i + 1 == cs.count || cs[i + 1] == "/" || cs[i + 1] == " "):
                actual.trozos.append(.casa)
                i += 1
            default:
                literal.append(c)
                i += 1
            }
        }
        cierraPalabra()
        return out
    }

    private static func operador(_ cs: [Character], _ i: Int) -> (String, Int)? {
        let tres = i + 3 <= cs.count ? String(cs[i ..< i + 3]) : ""
        let dos = i + 2 <= cs.count ? String(cs[i ..< i + 2]) : ""
        if tres == "2>&" && i + 4 <= cs.count && cs[i + 3] == "1" { return ("2>&1", 4) }
        if tres == "2>>" { return ("2>>", 3) }
        for o in ["&&", "||", ">>", "&>", "2>"] where dos == o { return (o, 2) }
        if "|;<>&".contains(cs[i]) { return (String(cs[i]), 1) }
        return nil
    }

    /// Dentro de "…": $VAR y $(…) se expanden; \" \$ \\ se escapan.
    private static func dobles(_ cs: [Character], _ desde: Int, _ p: inout Palabra) throws -> Int {
        var i = desde
        var lit = ""
        while i < cs.count {
            let c = cs[i]
            if c == "\"" {
                p.trozos.append(.literal(lit, protegido: true))
                return i + 1
            }
            if c == "\\", i + 1 < cs.count, "\"$\\`".contains(cs[i + 1]) {
                lit.append(cs[i + 1])
                i += 2
                continue
            }
            if c == "$", i + 1 < cs.count, cs[i + 1].isLetter || "{(?_$0".contains(cs[i + 1]) {
                if !lit.isEmpty { p.trozos.append(.literal(lit, protegido: true)); lit = "" }
                i = try dolar(cs, i, &p)
                continue
            }
            lit.append(c)
            i += 1
        }
        throw Fallo(mensaje: "falta cerrar la comilla \"")
    }

    /// $VAR ${VAR} $? $(orden)
    private static func dolar(_ cs: [Character], _ i: Int, _ p: inout Palabra) throws -> Int {
        guard i + 1 < cs.count else { p.trozos.append(.literal("$", protegido: true)); return i + 1 }
        let n = cs[i + 1]
        if n == "(" {
            var nivel = 1
            var j = i + 2
            while j < cs.count && nivel > 0 {
                if cs[j] == "(" { nivel += 1 } else if cs[j] == ")" { nivel -= 1 }
                j += 1
            }
            if nivel > 0 { throw Fallo(mensaje: "falta cerrar $(") }
            p.trozos.append(.orden(String(cs[(i + 2) ..< (j - 1)])))
            return j
        }
        if n == "{" {
            guard let fin = cs[(i + 2)...].firstIndex(of: "}") else { throw Fallo(mensaje: "falta cerrar ${") }
            p.trozos.append(.variable(String(cs[(i + 2) ..< fin])))
            return fin + 1
        }
        if "?$0#".contains(n) {
            p.trozos.append(.variable(String(n)))
            return i + 2
        }
        var j = i + 1
        while j < cs.count && (cs[j].isLetter || cs[j].isNumber || cs[j] == "_") { j += 1 }
        if j == i + 1 {
            p.trozos.append(.literal("$", protegido: true))
            return i + 1
        }
        p.trozos.append(.variable(String(cs[(i + 1) ..< j])))
        return j
    }

    /// Solo las palabras (para expandir un trozo de texto suelto).
    static func palabras(_ s: String) throws -> [Palabra] {
        return try piezas(s).compactMap { if case .palabra(let p) = $0 { return p } else { return nil } }
    }

    // MARK: órdenes

    static func analiza(_ s: String) throws -> [Enlazada] {
        let ps = try piezas(s)
        var out: [Enlazada] = []
        var tuberia: [Orden] = []
        var actual = Orden()
        var enlace = Enlace.primera
        var i = 0
        func cierraOrden() throws {
            if actual.palabras.isEmpty && actual.salida == nil && actual.entrada == nil {
                throw Fallo(mensaje: "error de sintaxis: falta una orden")
            }
            tuberia.append(actual)
            actual = Orden()
        }
        func cierraTuberia(_ siguiente: Enlace) throws {
            try cierraOrden()
            out.append(Enlazada(enlace: enlace, tuberia: tuberia))
            tuberia = []
            enlace = siguiente
        }
        while i < ps.count {
            switch ps[i] {
            case .palabra(let p):
                actual.palabras.append(p)
                i += 1
            case .op(let o):
                i += 1
                switch o {
                case "|":
                    try cierraOrden()
                case "&&":
                    try cierraTuberia(.y)
                case "||":
                    try cierraTuberia(.o)
                case ";", "&":
                    if actual.palabras.isEmpty && tuberia.isEmpty { continue }
                    try cierraTuberia(.luego)
                case "2>&1":
                    actual.errorASalida = true
                default:
                    guard i < ps.count, case .palabra(let f) = ps[i] else { throw Fallo(mensaje: "error de sintaxis cerca de «\(o)»") }
                    i += 1
                    switch o {
                    case "<": actual.entrada = f
                    case ">": actual.salida = Redireccion(archivo: f, añadir: false)
                    case ">>": actual.salida = Redireccion(archivo: f, añadir: true)
                    case "2>": actual.errores = Redireccion(archivo: f, añadir: false)
                    case "2>>": actual.errores = Redireccion(archivo: f, añadir: true)
                    case "&>":
                        actual.salida = Redireccion(archivo: f, añadir: false)
                        actual.errorASalida = true
                    default: break
                    }
                }
            }
        }
        if !actual.palabras.isEmpty || !tuberia.isEmpty { try cierraTuberia(.luego) }
        return out
    }
}
