import Foundation

// ============================================================
// MARK: - C, C++ y Java: léxico y preprocesador
// ============================================================
// Los tres lenguajes comparten léxico. El preprocesador (#include,
// #define, #if…) solo se usa en C y C++.
//
// No se genera código máquina: iOS no deja que una app cree y
// ejecute código nativo nuevo. El programa se analiza entero (como
// un compilador: errores con línea) y lo ejecuta un intérprete.

enum CDialecto: String { case c, cpp, java }

enum CTK: UInt8 { case id, ent, real, car, cad, op, dir, fin }

struct CTok {
    var k: CTK
    var t: String
    var linea: Int
    var v: Int64 = 0
    var dv: Double = 0
    var sinSigno = false
    var largo = false
    var flotante = false

    init(_ k: CTK, _ t: String, _ linea: Int) {
        self.k = k
        self.t = t
        self.linea = linea
    }

    func es(_ s: String) -> Bool { k == .op && t == s }
    func esId(_ s: String) -> Bool { k == .id && t == s }
}

/// Error de compilación o de ejecución, con la línea del programa.
struct CFallo: Error {
    let msg: String
    let linea: Int
    init(_ msg: String, _ linea: Int) {
        self.msg = msg
        self.linea = linea
    }
}

// MARK: léxico

final class CLexer {
    let b: [UInt8]
    var i = 0
    var linea: Int
    let dialecto: CDialecto
    var inicioDeLinea = true

    static let ops3: [String] = [">>>=", "<<=", ">>=", ">>>", "...", "->*"]
    static let ops2: [String] = ["::", "->", "++", "--", "<<", ">>", "<=", ">=", "==", "!=", "&&", "||",
                                 "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^="]

    init(_ src: String, linea: Int = 1, dialecto: CDialecto) {
        b = Array(src.utf8)
        self.linea = linea
        self.dialecto = dialecto
    }

    func mira(_ d: Int = 0) -> UInt8 {
        let j = i + d
        return j < b.count ? b[j] : 0
    }

    static func esLetra(_ c: UInt8) -> Bool {
        (c >= 97 && c <= 122) || (c >= 65 && c <= 90) || c == 95 || c == 36 || c >= 128
    }
    static func esDigito(_ c: UInt8) -> Bool { c >= 48 && c <= 57 }

    func todos() throws -> [CTok] {
        var out: [CTok] = []
        while true {
            let t = try siguiente()
            out.append(t)
            if t.k == .fin { break }
        }
        return out
    }

    /// Salta espacios y comentarios. Devuelve false si llega al final.
    func saltaBlancos() throws {
        while i < b.count {
            let c = b[i]
            if c == 10 { linea += 1; i += 1; inicioDeLinea = true; continue }
            if c == 32 || c == 9 || c == 13 || c == 12 || c == 11 { i += 1; continue }
            if c == 92 && mira(1) == 10 { i += 2; linea += 1; continue }
            if c == 47 && mira(1) == 47 {
                while i < b.count && b[i] != 10 { i += 1 }
                continue
            }
            if c == 47 && mira(1) == 42 {
                let empieza = linea
                i += 2
                while i < b.count && !(b[i] == 42 && mira(1) == 47) {
                    if b[i] == 10 { linea += 1 }
                    i += 1
                }
                if i >= b.count { throw CFallo("comentario /* sin cerrar", empieza) }
                i += 2
                continue
            }
            break
        }
    }

    func siguiente() throws -> CTok {
        try saltaBlancos()
        guard i < b.count else { return CTok(.fin, "", linea) }
        let c = b[i]
        if c == 35 && inicioDeLinea && dialecto != .java { return directiva() }
        inicioDeLinea = false
        if CLexer.esLetra(c) { return identificador() }
        if CLexer.esDigito(c) || (c == 46 && CLexer.esDigito(mira(1))) { return try numero() }
        if c == 34 { return try cadena() }
        if c == 39 { return try caracter() }
        // prefijos de cadena: L"…", u8"…" — se tratan igual
        return try operador()
    }

    func directiva() -> CTok {
        let l = linea
        var bytes: [UInt8] = []
        i += 1
        while i < b.count {
            let c = b[i]
            if c == 92 && mira(1) == 10 { i += 2; linea += 1; bytes.append(32); continue }
            if c == 92 && mira(1) == 13 && mira(2) == 10 { i += 3; linea += 1; bytes.append(32); continue }
            if c == 10 { break }
            // comentario /* */ dentro de la directiva
            if c == 47 && mira(1) == 42 {
                i += 2
                while i < b.count && !(b[i] == 42 && mira(1) == 47) {
                    if b[i] == 10 { linea += 1 }
                    i += 1
                }
                i += 2
                bytes.append(32)
                continue
            }
            if c == 47 && mira(1) == 47 {
                while i < b.count && b[i] != 10 { i += 1 }
                break
            }
            bytes.append(c)
            i += 1
        }
        let texto = String(decoding: bytes, as: UTF8.self)
        return CTok(.dir, texto.trimmingCharacters(in: .whitespaces), l)
    }

    func identificador() -> CTok {
        let ini = i
        while i < b.count && (CLexer.esLetra(b[i]) || CLexer.esDigito(b[i])) { i += 1 }
        let s = String(decoding: b[ini..<i], as: UTF8.self)
        // prefijos de literales: L"x", u8"x", u"x", U"x", R"(x)" (crudo)
        if i < b.count && (b[i] == 34 || b[i] == 39) && ["L", "u8", "u", "U"].contains(s) {
            if b[i] == 34, let t = try? cadena() { return t }
            if b[i] == 39, let t = try? caracter() { return t }
        }
        if s == "R" && i < b.count && b[i] == 34 && dialecto != .java { return cadenaCruda() }
        return CTok(.id, s, linea)
    }

    func cadenaCruda() -> CTok {
        // R"delim( … )delim"
        let l = linea
        i += 1
        var delim: [UInt8] = []
        while i < b.count && b[i] != 40 { delim.append(b[i]); i += 1 }
        i += 1
        let cierre: [UInt8] = [41] + delim + [34]
        var bytes: [UInt8] = []
        while i < b.count {
            if b[i] == 41 && i + cierre.count <= b.count && Array(b[i..<(i + cierre.count)]) == cierre {
                i += cierre.count
                break
            }
            if b[i] == 10 { linea += 1 }
            bytes.append(b[i])
            i += 1
        }
        return CTok(.cad, String(decoding: bytes, as: UTF8.self), l)
    }

    func numero() throws -> CTok {
        let ini = i
        let l = linea
        var esReal = false
        var base = 10
        var digitos: [UInt8] = []
        if b[i] == 48 && (mira(1) == 120 || mira(1) == 88) {
            base = 16; i += 2
            while i < b.count && (CLexer.esHex(b[i]) || b[i] == 95 || b[i] == 39) {
                if b[i] != 95 && b[i] != 39 { digitos.append(b[i]) }
                i += 1
            }
        } else if b[i] == 48 && (mira(1) == 98 || mira(1) == 66) {
            base = 2; i += 2
            while i < b.count && (b[i] == 48 || b[i] == 49 || b[i] == 95 || b[i] == 39) {
                if b[i] != 95 && b[i] != 39 { digitos.append(b[i]) }
                i += 1
            }
        } else {
            while i < b.count {
                let c = b[i]
                if CLexer.esDigito(c) { digitos.append(c); i += 1; continue }
                if c == 95 || c == 39 { i += 1; continue }
                if c == 46 && !esReal && CLexer.esDigito(mira(1)) { esReal = true; digitos.append(c); i += 1; continue }
                if c == 46 && !esReal { esReal = true; digitos.append(c); i += 1; continue }
                if (c == 101 || c == 69) && (CLexer.esDigito(mira(1)) || ((mira(1) == 43 || mira(1) == 45) && CLexer.esDigito(mira(2)))) {
                    esReal = true
                    digitos.append(c); i += 1
                    if b[i] == 43 || b[i] == 45 { digitos.append(b[i]); i += 1 }
                    continue
                }
                break
            }
            if !esReal && digitos.count > 1 && digitos[0] == 48 { base = 8 }
        }
        var t = CTok(.ent, String(decoding: b[ini..<i], as: UTF8.self), l)
        // sufijos
        while i < b.count {
            let c = b[i]
            if c == 117 || c == 85 { t.sinSigno = true; i += 1; continue }
            if c == 108 || c == 76 { t.largo = true; i += 1; continue }
            if c == 102 || c == 70 { if base == 10 { esReal = true; t.flotante = true }; i += 1; continue }
            if c == 100 || c == 68 { if base == 10 && dialecto == .java { esReal = true }; i += 1; continue }
            break
        }
        let texto = String(decoding: digitos, as: UTF8.self)
        if esReal {
            t.k = .real
            guard let d = Double(texto) else { throw CFallo("número no válido: \(t.t)", l) }
            t.dv = d
            return t
        }
        var v: UInt64 = 0
        for d in digitos {
            let x = UInt64(CLexer.valorHex(d))
            let (m, o1) = v.multipliedReportingOverflow(by: UInt64(base))
            let (s, o2) = m.addingReportingOverflow(x)
            if o1 || o2 { throw CFallo("número demasiado grande: \(t.t)", l) }
            v = s
        }
        t.v = Int64(bitPattern: v)
        if v > UInt64(Int32.max) && !t.sinSigno && base == 10 && dialecto != .java { t.largo = true }
        if v > UInt64(UInt32.max) { t.largo = true }
        if dialecto == .java && !t.largo && v > UInt64(UInt32.max) {
            throw CFallo("entero demasiado grande para int: \(t.t) (añade L)", l)
        }
        if dialecto == .java && !t.largo && v > UInt64(Int32.max) {
            // 0xFFFFFFFF en Java es -1
            t.v = Int64(Int32(truncatingIfNeeded: v))
        }
        return t
    }

    static func esHex(_ c: UInt8) -> Bool {
        esDigito(c) || (c >= 97 && c <= 102) || (c >= 65 && c <= 70)
    }
    static func valorHex(_ c: UInt8) -> Int {
        if c >= 48 && c <= 57 { return Int(c) - 48 }
        if c >= 97 && c <= 102 { return Int(c) - 87 }
        if c >= 65 && c <= 70 { return Int(c) - 55 }
        return 0
    }

    /// Lee un carácter con escapes; devuelve sus bytes UTF-8.
    func escape() throws -> [UInt8] {
        let c = b[i]
        if c != 92 { i += 1; return [c] }
        i += 1
        guard i < b.count else { throw CFallo("escape incompleto", linea) }
        let e = b[i]
        i += 1
        switch e {
        case 110: return [10]
        case 116: return [9]
        case 114: return [13]
        case 48 where !CLexer.esDigito(mira()): return [0]
        case 97: return [7]
        case 98: return [8]
        case 102: return [12]
        case 118: return [11]
        case 101: return [27]
        case 92: return [92]
        case 39: return [39]
        case 34: return [34]
        case 63: return [63]
        case 120:
            var v = 0
            while i < b.count && CLexer.esHex(b[i]) { v = v * 16 + CLexer.valorHex(b[i]); i += 1 }
            return [UInt8(truncatingIfNeeded: v)]
        case 117, 85:
            var v = 0
            var n = 0
            let maxN = e == 117 ? 4 : 8
            while i < b.count && n < maxN && CLexer.esHex(b[i]) { v = v * 16 + CLexer.valorHex(b[i]); i += 1; n += 1 }
            let u = Unicode.Scalar(UInt32(v)) ?? "?"
            return Array(String(Character(u)).utf8)
        default:
            if e >= 48 && e <= 55 {
                var v = Int(e) - 48
                var n = 1
                while n < 3 && i < b.count && b[i] >= 48 && b[i] <= 55 { v = v * 8 + Int(b[i]) - 48; i += 1; n += 1 }
                return [UInt8(truncatingIfNeeded: v)]
            }
            return [e]
        }
    }

    func cadena() throws -> CTok {
        let l = linea
        i += 1
        var bytes: [UInt8] = []
        // bloque de texto de Java: """ … """
        if dialecto == .java && mira() == 34 && mira(1) == 34 {
            i += 2
            while i < b.count && b[i] != 10 { i += 1 }
            i += 1
            linea += 1
            while i < b.count && !(b[i] == 34 && mira(1) == 34 && mira(2) == 34) {
                if b[i] == 10 { linea += 1 }
                bytes.append(contentsOf: try escape())
            }
            i += 3
            return CTok(.cad, CLexer.quitaSangria(String(decoding: bytes, as: UTF8.self)), l)
        }
        while true {
            guard i < b.count, b[i] != 10 else { throw CFallo("cadena sin cerrar (falta \")", l) }
            if b[i] == 34 { i += 1; break }
            bytes.append(contentsOf: try escape())
        }
        return CTok(.cad, String(decoding: bytes, as: UTF8.self), l)
    }

    static func quitaSangria(_ s: String) -> String {
        let lineas = s.components(separatedBy: "\n")
        var minimo = Int.max
        for l in lineas where !l.trimmingCharacters(in: .whitespaces).isEmpty {
            let n = l.prefix { $0 == " " || $0 == "\t" }.count
            minimo = min(minimo, n)
        }
        if minimo == Int.max { minimo = 0 }
        return lineas.map { String($0.dropFirst(min(minimo, $0.count))) }.joined(separator: "\n")
    }

    func caracter() throws -> CTok {
        let l = linea
        i += 1
        guard i < b.count, b[i] != 39 else { throw CFallo("carácter vacío ''", l) }
        let bytes = try escape()
        // un carácter multibyte (ñ, á): se toma el escalar Unicode
        var extra: [UInt8] = []
        while i < b.count && b[i] != 39 && b[i] != 10 && extra.count < 4 { extra.append(b[i]); i += 1 }
        guard i < b.count, b[i] == 39 else { throw CFallo("falta ' al final del carácter", l) }
        i += 1
        let todo = bytes + extra
        var t = CTok(.car, "", l)
        if todo.count == 1 {
            t.v = Int64(todo[0])
        } else {
            let s = String(decoding: todo, as: UTF8.self)
            t.v = Int64(s.unicodeScalars.first?.value ?? 63)
        }
        return t
    }

    func operador() throws -> CTok {
        let l = linea
        for o in CLexer.ops3 {
            if dialecto != .java && o.hasPrefix(">>>") { continue }
            if coincide(o) { i += o.utf8.count; return CTok(.op, o, l) }
        }
        for o in CLexer.ops2 where coincide(o) {
            i += 2
            return CTok(.op, o, l)
        }
        let c = b[i]
        i += 1
        let s = String(UnicodeScalar(c))
        if "+-*/%=<>!&|^~?:;,.()[]{}@#".contains(s) { return CTok(.op, s, l) }
        throw CFallo("carácter inesperado '\(s)'", l)
    }

    func coincide(_ o: String) -> Bool {
        var j = i
        for c in o.utf8 {
            guard j < b.count, b[j] == c else { return false }
            j += 1
        }
        return true
    }
}

// MARK: preprocesador

struct CMacro {
    let params: [String]?
    let variadica: Bool
    let cuerpo: [CTok]
}

final class CPreproc {
    var macros: [String: CMacro] = [:]
    var cabeceras: Set<String> = []
    let dialecto: CDialecto
    let leer: (String) -> String?
    var nivel = 0

    init(dialecto: CDialecto, leer: @escaping (String) -> String?) {
        self.dialecto = dialecto
        self.leer = leer
        definePredefinidas()
    }

    func definePredefinidas() {
        let lista: [String] = [
            "EOF -1", "INT_MAX 2147483647", "INT_MIN (-2147483647-1)", "UINT_MAX 4294967295u",
            "LONG_MAX 9223372036854775807L", "LONG_MIN (-9223372036854775807L-1)",
            "LLONG_MAX 9223372036854775807LL", "LLONG_MIN (-9223372036854775807LL-1)",
            "ULLONG_MAX 18446744073709551615ull", "SHRT_MAX 32767", "SHRT_MIN (-32768)",
            "CHAR_MAX 127", "CHAR_MIN (-128)", "UCHAR_MAX 255", "CHAR_BIT 8", "RAND_MAX 2147483647",
            "EXIT_SUCCESS 0", "EXIT_FAILURE 1", "M_PI 3.14159265358979323846", "M_E 2.7182818284590452354",
            "M_SQRT2 1.41421356237309504880", "INFINITY (1.0/0.0)", "NAN (0.0/0.0)",
            "DBL_MAX 1.7976931348623157e308", "DBL_MIN 2.2250738585072014e-308", "FLT_MAX 3.40282347e38f",
            "DBL_EPSILON 2.220446049250313e-16", "SIZE_MAX 18446744073709551615ull",
            "INT32_MAX 2147483647", "INT32_MIN (-2147483647-1)", "INT64_MAX 9223372036854775807LL",
            "INT64_MIN (-9223372036854775807LL-1)", "UINT32_MAX 4294967295u", "UINT64_MAX 18446744073709551615ull",
            "CLOCKS_PER_SEC 1000000", "__STDC__ 1", "__SWIFTSHELL__ 1", "SEEK_SET 0", "SEEK_CUR 1", "SEEK_END 2",
            "stdin __stdin", "stdout __stdout", "stderr __stderr"
        ]
        for l in lista {
            let partes = l.split(separator: " ", maxSplits: 1).map(String.init)
            let toks = (try? CLexer(partes[1], dialecto: .cpp).todos()) ?? []
            macros[partes[0]] = CMacro(params: nil, variadica: false, cuerpo: toks.filter { $0.k != .fin })
        }
        if dialecto == .cpp { macros["__cplusplus"] = CMacro(params: nil, variadica: false, cuerpo: [entero(201703, 0)]) }
    }

    func entero(_ v: Int64, _ linea: Int) -> CTok {
        var t = CTok(.ent, String(v), linea)
        t.v = v
        return t
    }

    func procesa(_ src: String, archivo: String) throws -> [CTok] {
        nivel += 1
        defer { nivel -= 1 }
        if nivel > 30 { throw CFallo("#include anidado demasiadas veces (¿se incluye a sí mismo?)", 0) }
        let toks = try CLexer(src, dialecto: dialecto).todos()
        if dialecto == .java { return toks }
        var salida: [CTok] = []
        var pila: [(activo: Bool, tomado: Bool)] = []
        var pendiente: [CTok] = []
        for t in toks {
            if t.k == .dir {
                if !pendiente.isEmpty { salida += try expande(pendiente, []); pendiente = [] }
                try directiva(t, &pila, &salida, archivo)
                continue
            }
            if t.k == .fin { break }
            if !pila.allSatisfy({ $0.activo }) { continue }
            pendiente.append(t)
        }
        if !pendiente.isEmpty { salida += try expande(pendiente, []) }
        if !pila.isEmpty { throw CFallo("falta #endif", toks.last?.linea ?? 0) }
        if nivel == 1 { salida.append(CTok(.fin, "", toks.last?.linea ?? 0)) }
        return salida
    }

    func directiva(_ t: CTok, _ pila: inout [(activo: Bool, tomado: Bool)], _ salida: inout [CTok], _ archivo: String) throws {
        let texto = t.t
        let nombre = String(texto.prefix { $0.isLetter })
        let resto = String(texto.dropFirst(nombre.count)).trimmingCharacters(in: .whitespaces)
        let activo = pila.allSatisfy { $0.activo }
        switch nombre {
        case "ifdef", "ifndef":
            let def = macros[resto.trimmingCharacters(in: .whitespaces)] != nil
            let v = nombre == "ifdef" ? def : !def
            pila.append((activo && v, v))
        case "if":
            let v = activo ? try evaluaCondicion(resto, t.linea) : false
            pila.append((activo && v, v))
        case "elif":
            guard let ult = pila.popLast() else { throw CFallo("#elif sin #if", t.linea) }
            let padre = pila.allSatisfy { $0.activo }
            if ult.tomado { pila.append((false, true)) }
            else {
                let v = padre ? try evaluaCondicion(resto, t.linea) : false
                pila.append((padre && v, v))
            }
        case "else":
            guard let ult = pila.popLast() else { throw CFallo("#else sin #if", t.linea) }
            let padre = pila.allSatisfy { $0.activo }
            pila.append((padre && !ult.tomado, true))
        case "endif":
            guard pila.popLast() != nil else { throw CFallo("#endif sin #if", t.linea) }
        default:
            guard activo else { return }
            try directivaActiva(nombre, resto, t.linea, &salida, archivo)
        }
    }

    func directivaActiva(_ nombre: String, _ resto: String, _ linea: Int, _ salida: inout [CTok], _ archivo: String) throws {
        switch nombre {
        case "include":
            if resto.hasPrefix("<") {
                let h = resto.dropFirst().prefix { $0 != ">" }
                cabeceras.insert(String(h))
                return
            }
            guard resto.hasPrefix("\"") else { throw CFallo("#include mal escrito: \(resto)", linea) }
            let h = String(resto.dropFirst().prefix { $0 != "\"" })
            if CPreproc.cabecerasConocidas.contains(h) { cabeceras.insert(h); return }
            guard let src = leer(h) else { throw CFallo("no encuentro el archivo '\(h)' (#include \"\(h)\")", linea) }
            let toks = try procesa(src, archivo: h)
            salida += toks.filter { $0.k != .fin }
        case "define":
            try define(resto, linea)
        case "undef":
            macros[resto] = nil
        case "pragma", "line", "":
            return
        case "error":
            throw CFallo("#error \(resto)", linea)
        case "warning":
            return
        default:
            throw CFallo("directiva desconocida #\(nombre)", linea)
        }
    }

    static let cabecerasConocidas: Set<String> = ["graphics.h", "ui.h", "conio.h"]

    func define(_ texto: String, _ linea: Int) throws {
        let lx = CLexer(texto, linea: linea, dialecto: dialecto)
        let toks = try lx.todos().filter { $0.k != .fin }
        guard let n = toks.first, n.k == .id else { throw CFallo("#define sin nombre", linea) }
        // ¿macro con parámetros? el '(' tiene que ir pegado al nombre
        let tras = texto.dropFirst(n.t.count)
        if tras.hasPrefix("(") {
            var params: [String] = []
            var variadica = false
            var j = 2
            while j < toks.count && !toks[j].es(")") {
                if toks[j].k == .id { params.append(toks[j].t) }
                if toks[j].es("...") { variadica = true; params.append("__VA_ARGS__") }
                j += 1
            }
            let cuerpo = j + 1 < toks.count ? Array(toks[(j + 1)...]) : []
            macros[n.t] = CMacro(params: params, variadica: variadica, cuerpo: cuerpo)
        } else {
            macros[n.t] = CMacro(params: nil, variadica: false, cuerpo: Array(toks.dropFirst()))
        }
    }

    // MARK: expansión de macros

    func expande(_ toks: [CTok], _ prohibidas: Set<String>) throws -> [CTok] {
        var out: [CTok] = []
        out.reserveCapacity(toks.count)
        var i = 0
        while i < toks.count {
            let t = toks[i]
            guard t.k == .id, let m = macros[t.t], !prohibidas.contains(t.t) else {
                out.append(t)
                i += 1
                continue
            }
            var nuevas = prohibidas
            nuevas.insert(t.t)
            guard let params = m.params else {
                let cuerpo = m.cuerpo.map { x -> CTok in var y = x; y.linea = t.linea; return y }
                out += try expande(cuerpo, nuevas)
                i += 1
                continue
            }
            // macro con parámetros: necesita '(' detrás
            guard i + 1 < toks.count, toks[i + 1].es("(") else {
                out.append(t)
                i += 1
                continue
            }
            let (args, fin) = try argumentos(toks, i + 2, t.linea)
            i = fin
            let sust = try sustituye(m, params, args, t.linea)
            out += try expande(sust, nuevas)
        }
        return out
    }

    func argumentos(_ toks: [CTok], _ desde: Int, _ linea: Int) throws -> ([[CTok]], Int) {
        var args: [[CTok]] = [[]]
        var prof = 0
        var j = desde
        while j < toks.count {
            let x = toks[j]
            if x.es("(") { prof += 1 }
            if x.es(")") {
                if prof == 0 { return (args.count == 1 && args[0].isEmpty ? [] : args, j + 1) }
                prof -= 1
            }
            if x.es(",") && prof == 0 { args.append([]); j += 1; continue }
            args[args.count - 1].append(x)
            j += 1
        }
        throw CFallo("faltan ')' en la llamada a la macro", linea)
    }

    func sustituye(_ m: CMacro, _ params: [String], _ args: [[CTok]], _ linea: Int) throws -> [CTok] {
        var valores: [String: [CTok]] = [:]
        for (k, p) in params.enumerated() {
            if p == "__VA_ARGS__" {
                var resto: [CTok] = []
                for (n, a) in args.enumerated() where n >= k {
                    if n > k { resto.append(CTok(.op, ",", linea)) }
                    resto += a
                }
                valores[p] = resto
            } else {
                valores[p] = k < args.count ? args[k] : []
            }
        }
        var out: [CTok] = []
        var j = 0
        let c = m.cuerpo
        while j < c.count {
            let x = c[j]
            if x.es("#"), j + 1 < c.count, let v = valores[c[j + 1].t] {
                let s = v.map { $0.k == .cad ? "\"\($0.t)\"" : $0.t }.joined(separator: " ")
                out.append(CTok(.cad, s, linea))
                j += 2
                continue
            }
            if x.es("#"), j + 1 < c.count, c[j + 1].es("#"), !out.isEmpty, j + 2 < c.count {
                // a ## b: pega dos tokens
                let izq = out.removeLast()
                let derTok = c[j + 2]
                let der = valores[derTok.t]?.first ?? derTok
                let pegado = izq.t + der.t
                let nuevos = (try? CLexer(pegado, linea: linea, dialecto: dialecto).todos()) ?? []
                out += nuevos.filter { $0.k != .fin }
                j += 3
                continue
            }
            if x.k == .id, let v = valores[x.t] {
                out += try expande(v, [])
            } else {
                var y = x
                y.linea = linea
                out.append(y)
            }
            j += 1
        }
        return out
    }

    // MARK: #if

    func evaluaCondicion(_ texto: String, _ linea: Int) throws -> Bool {
        let toks = try CLexer(texto, linea: linea, dialecto: dialecto).todos().filter { $0.k != .fin }
        var limpios: [CTok] = []
        var j = 0
        while j < toks.count {
            let t = toks[j]
            if t.esId("defined") {
                var nombre = ""
                if j + 1 < toks.count && toks[j + 1].es("(") {
                    nombre = j + 2 < toks.count ? toks[j + 2].t : ""
                    j += 4
                } else {
                    nombre = j + 1 < toks.count ? toks[j + 1].t : ""
                    j += 2
                }
                limpios.append(entero(macros[nombre] != nil ? 1 : 0, linea))
                continue
            }
            limpios.append(t)
            j += 1
        }
        let exp = try expande(limpios, []).map { t -> CTok in
            if t.k == .id { return t.t == "true" ? entero(1, linea) : entero(0, linea) }
            return t
        }
        let ev = CEvalPre(exp)
        return try ev.ternario() != 0
    }
}

/// Evalúa expresiones enteras de #if.
final class CEvalPre {
    let t: [CTok]
    var i = 0
    init(_ t: [CTok]) { self.t = t }

    func mira() -> String { i < t.count && t[i].k == .op ? t[i].t : "" }

    func ternario() throws -> Int64 {
        let c = try binario(0)
        if mira() == "?" {
            i += 1
            let a = try ternario()
            if mira() == ":" { i += 1 }
            let b = try ternario()
            return c != 0 ? a : b
        }
        return c
    }

    static let niveles: [[String]] = [["||"], ["&&"], ["|"], ["^"], ["&"], ["==", "!="],
                                      ["<", ">", "<=", ">="], ["<<", ">>"], ["+", "-"], ["*", "/", "%"]]

    func binario(_ n: Int) throws -> Int64 {
        if n >= CEvalPre.niveles.count { return try unario() }
        var a = try binario(n + 1)
        while CEvalPre.niveles[n].contains(mira()) {
            let op = mira()
            i += 1
            let b = try binario(n + 1)
            a = CEvalPre.aplica(op, a, b)
        }
        return a
    }

    static func aplica(_ op: String, _ a: Int64, _ b: Int64) -> Int64 {
        switch op {
        case "||": return (a != 0 || b != 0) ? 1 : 0
        case "&&": return (a != 0 && b != 0) ? 1 : 0
        case "|": return a | b
        case "^": return a ^ b
        case "&": return a & b
        case "==": return a == b ? 1 : 0
        case "!=": return a != b ? 1 : 0
        case "<": return a < b ? 1 : 0
        case ">": return a > b ? 1 : 0
        case "<=": return a <= b ? 1 : 0
        case ">=": return a >= b ? 1 : 0
        case "<<": return a << (b & 63)
        case ">>": return a >> (b & 63)
        case "+": return a &+ b
        case "-": return a &- b
        case "*": return a &* b
        case "/": return b == 0 ? 0 : a / b
        default: return b == 0 ? 0 : a % b
        }
    }

    func unario() throws -> Int64 {
        let o = mira()
        if o == "!" { i += 1; return try unario() == 0 ? 1 : 0 }
        if o == "-" { i += 1; return 0 &- (try unario()) }
        if o == "+" { i += 1; return try unario() }
        if o == "~" { i += 1; return ~(try unario()) }
        if o == "(" {
            i += 1
            let v = try ternario()
            if mira() == ")" { i += 1 }
            return v
        }
        guard i < t.count else { return 0 }
        let x = t[i]
        i += 1
        if x.k == .ent || x.k == .car { return x.v }
        return 0
    }
}
