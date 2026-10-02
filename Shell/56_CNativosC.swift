import Foundation

// ============================================================
// MARK: - C: biblioteca estándar (stdio, stdlib, string, math…)
// ============================================================

/// rand() con el mismo algoritmo de glibc: srand(42) da los mismos
/// números que en Linux.
struct CRandGlibc {
    var r: [Int32] = []
    var i = 0

    init(_ semilla: UInt32) { siembra(semilla) }

    mutating func siembra(_ s: UInt32) {
        var v = [Int32](repeating: 0, count: 34)
        v[0] = Int32(bitPattern: s == 0 ? 1 : s)
        for k in 1..<31 {
            let hi = Int64(v[k - 1]) / 127773
            let lo = Int64(v[k - 1]) % 127773
            var w = 16807 * lo - 2836 * hi
            if w < 0 { w += 2147483647 }
            v[k] = Int32(truncatingIfNeeded: w)
        }
        for k in 31..<34 { v[k] = v[k - 31] }
        r = v
        i = 0
        for _ in 34..<344 { _ = siguienteCrudo() }
    }

    mutating func siguienteCrudo() -> Int32 {
        let n = r.count
        let a = UInt32(bitPattern: r[n - 31])
        let b = UInt32(bitPattern: r[n - 3])
        let x = Int32(bitPattern: a &+ b)
        r.append(x)
        if r.count > 128 { r.removeFirst(r.count - 34) }
        return x
    }

    mutating func siguiente() -> Int64 {
        Int64(UInt32(bitPattern: siguienteCrudo()) >> 1)
    }
}

extension CMaq {

    static let nativasConLugar: Set<String> = ["swap", "getline", "iter_swap"]
    static let metodosConLugar: Set<String> = ["get", "swap", "getline"]

    static let funcionesC: Set<String> = [
        "printf", "puts", "putchar", "getchar", "scanf", "sprintf", "snprintf", "fprintf", "fputs", "fputc", "putc",
        "fgets", "fgetc", "getc", "fscanf", "sscanf", "fopen", "fclose", "feof", "fflush", "gets", "perror", "remove",
        "rename", "rewind", "ungetc", "vprintf", "setbuf", "setvbuf",
        "malloc", "calloc", "realloc", "free", "exit", "abort", "atoi", "atol", "atoll", "atof", "strtol", "strtoll",
        "strtoul", "strtoull", "strtod", "strtof", "rand", "srand", "abs", "labs", "llabs", "qsort", "bsearch", "system", "getenv",
        "strlen", "strcpy", "strncpy", "strcat", "strncat", "strcmp", "strncmp", "strchr", "strrchr", "strstr", "strdup",
        "memset", "memcpy", "memmove", "memcmp", "strtok", "strcasecmp", "strncasecmp", "strrev", "strspn", "strcspn", "strpbrk",
        "isalpha", "isdigit", "isalnum", "isspace", "isupper", "islower", "ispunct", "isxdigit", "isprint", "iscntrl", "isgraph",
        "toupper", "tolower",
        "sqrt", "pow", "fabs", "floor", "ceil", "round", "trunc", "sin", "cos", "tan", "asin", "acos", "atan", "atan2",
        "exp", "log", "log10", "log2", "fmod", "hypot", "cbrt", "fmax", "fmin", "sinh", "cosh", "tanh", "lround", "llround",
        "isnan", "isinf", "sqrtf", "powf", "fabsf", "floorf", "ceilf", "roundf", "expf", "logf", "sinf", "cosf", "exp2",
        "time", "clock", "difftime", "sleep", "usleep", "getch", "getche", "clrscr", "gotoxy", "assert", "kbhit", "va_start", "va_end"
    ]

    func esNativoFuncion(_ n: String) -> Bool {
        switch dialecto {
        case .c: return CMaq.funcionesC.contains(n) || CMaq.funcionesGraficas.contains(n)
        case .cpp: return CMaq.funcionesCpp.contains(n) || CMaq.funcionesC.contains(n) || CMaq.funcionesGraficas.contains(n)
        case .java: return CMaq.funcionesJava.contains(n)
        }
    }

    func esNativoValor(_ n: String) -> Bool {
        switch dialecto {
        case .java: return false
        case .c: return ["__stdin", "__stdout", "__stderr"].contains(n) || CMaq.coloresGraficos[n] != nil
        case .cpp: return ["cout", "cin", "cerr", "clog", "endl", "__stdin", "__stdout", "__stderr", "fixed", "scientific",
                           "left", "right", "boolalpha", "noboolalpha", "showpoint", "hex", "dec", "oct", "ends", "flush",
                           "ws", "uppercase", "nouppercase"].contains(n) || CMaq.coloresGraficos[n] != nil
        }
    }

    func esClaseNativa(_ n: String) -> Bool {
        if dialecto == .java { return CMaq.clasesJava.contains(n) }
        return ["string", "ios", "ios_base", "chrono", "this_thread"].contains(n)
    }

    func tipoNativo(_ n: String) -> CTipo? {
        switch n {
        case "strlen", "size": return .num(.ulong)
        case "sqrt", "pow", "fabs", "floor", "ceil", "sin", "cos", "tan", "exp", "log", "log10", "atof", "fmod", "hypot",
             "Math.sqrt", "Math.pow", "Math.random", "Math.floor", "Math.ceil", "Math.sin", "Math.cos", "Math.log",
             "Math.exp", "Math.hypot", "Math.cbrt", "Double.parseDouble", "stod", "Math.PI", "Math.E", "Math.log10", "Math.atan2":
            return .real(false)
        case "atoi", "rand", "Integer.parseInt", "stoi", "Integer.MAX_VALUE", "Integer.MIN_VALUE", "Character.getNumericValue":
            return .num(.int)
        case "Long.parseLong", "System.currentTimeMillis", "System.nanoTime", "Long.MAX_VALUE", "Long.MIN_VALUE", "stoll", "atoll":
            return .num(.long)
        case "String.valueOf", "String.format", "to_string", "Integer.toString", "String.join", "Integer.toBinaryString":
            return dialecto == .c ? nil : .cad
        default:
            return nil
        }
    }

    func valorNativo(_ n: String, _ linea: Int) throws -> CV {
        switch n {
        case "cout", "__stdout": return .canal(cout)
        case "cerr", "clog", "__stderr": return .canal(cerr)
        case "cin", "__stdin": return .canal(cin)
        case "string.npos": return .n(-1, .ulong)
        default: break
        }
        if let c = CMaq.coloresGraficos[n] { return .n(Int64(c), .int) }
        if dialecto == .cpp { return manipulador(n, nil) }
        return try valorJava(n, linea)
    }

    func manipulador(_ n: String, _ arg: Int?) -> CV {
        let nombre = "manip:" + n + (arg.map { ":" + String($0) } ?? "")
        return .f(CCierre(fn: nil, marco: nil, este: nil, nombre: nombre, nat: { _, _ in .vacio }))
    }

    // MARK: despacho

    func nativa(_ n: String, _ a: [CV], _ l: [CLugar?], _ linea: Int) throws -> CV {
        if dialecto == .java { return try nativaJava(n, a, linea) }
        if dialecto == .cpp, let r = try nativaCpp(n, a, l, linea) { return r }
        if let r = try nativaStdio(n, a, linea) { return r }
        if let r = try nativaStdlib(n, a, linea) { return r }
        if let r = try nativaCadenas(n, a, linea) { return r }
        if let r = try nativaMate(n, a, linea) { return r }
        if let r = try nativaCtype(n, a, linea) { return r }
        if let r = try nativaGrafica(n, a, linea) { return r }
        throw CFallo("la función '\(n)' no está disponible", linea)
    }

    func arg(_ a: [CV], _ k: Int, _ n: String, _ linea: Int) throws -> CV {
        guard k < a.count else { throw CFallo("faltan argumentos en '\(n)'", linea) }
        if case .ref(let r) = a[k] { return try lee(r.l) }
        return a[k]
    }

    func argEnt(_ a: [CV], _ k: Int, _ n: String, _ linea: Int) throws -> Int64 {
        try entero(try arg(a, k, n, linea), linea)
    }

    func argReal(_ a: [CV], _ k: Int, _ n: String, _ linea: Int) throws -> Double {
        try real(try arg(a, k, n, linea), linea)
    }

    func argPtr(_ a: [CV], _ k: Int, _ n: String, _ linea: Int) throws -> CPtr {
        let v = try arg(a, k, n, linea)
        switch v {
        case .p(let p): return p
        case .n(0, _), .nulo: return CPtr.nulo
        default: throw CFallo("'\(n)' necesita un puntero en el argumento \(k + 1), no \(v.tipoNombre)", linea)
        }
    }

    // MARK: stdio

    func nativaStdio(_ n: String, _ a: [CV], _ linea: Int) throws -> CV? {
        switch n {
        case "printf", "vprintf":
            let f = try formatoBytes(try arg(a, 0, n, linea), linea)
            let out = try formatea(f, a, 1, linea)
            try emite(out)
            return .n(Int64(out.count), .int)
        case "puts":
            try emite(Array(try cadenaC(try arg(a, 0, n, linea), linea).utf8) + [10])
            return .n(1, .int)
        case "putchar":
            let c = try argEnt(a, 0, n, linea)
            try emite([UInt8(truncatingIfNeeded: c)])
            return .n(c, .int)
        case "getchar", "getch", "getche":
            vuelca()
            return .n(Int64(entrada.lee()), .int)
        case "gets":
            let p = try argPtr(a, 0, n, linea)
            vuelca()
            guard let l = entrada.linea() else { return .ptrNulo }
            try escribeC(l, en: p, linea)
            return .p(p)
        case "scanf":
            vuelca()
            let f = try formatoBytes(try arg(a, 0, n, linea), linea)
            return .n(Int64(try escanea(f, a, 1, entrada, linea)), .int)
        case "sscanf":
            let src = try cadenaC(try arg(a, 0, n, linea), linea)
            let f = try formatoBytes(try arg(a, 1, n, linea), linea)
            return .n(Int64(try escanea(f, a, 2, CEntrada(src), linea)), .int)
        case "sprintf", "snprintf":
            let p = try argPtr(a, 0, n, linea)
            let desde = n == "snprintf" ? 2 : 1
            let f = try formatoBytes(try arg(a, desde, n, linea), linea)
            var out = try formatea(f, a, desde + 1, linea)
            let total = out.count
            if n == "snprintf" {
                let max = Int(try argEnt(a, 1, n, linea))
                if max <= 0 { return .n(Int64(total), .int) }
                if out.count > max - 1 { out = Array(out.prefix(max - 1)) }
            }
            try escribeC(out, en: p, linea)
            return .n(Int64(total), .int)
        case "fprintf":
            let c = try canalDe(try arg(a, 0, n, linea), linea)
            let f = try formatoBytes(try arg(a, 1, n, linea), linea)
            let out = try formatea(f, a, 2, linea)
            try escribeCanal(c, out)
            return .n(Int64(out.count), .int)
        case "fputs":
            let c = try canalDe(try arg(a, 1, n, linea), linea)
            try escribeCanal(c, Array(try cadenaC(try arg(a, 0, n, linea), linea).utf8))
            return .n(1, .int)
        case "fputc", "putc":
            let c = try canalDe(try arg(a, 1, n, linea), linea)
            try escribeCanal(c, [UInt8(truncatingIfNeeded: try argEnt(a, 0, n, linea))])
            return .n(0, .int)
        case "fgets":
            let p = try argPtr(a, 0, n, linea)
            let max = Int(try argEnt(a, 1, n, linea))
            let c = try canalDe(try arg(a, 2, n, linea), linea)
            guard let ent = entradaDe(c) else { return .ptrNulo }
            var bytes: [UInt8] = []
            while bytes.count < max - 1 {
                let b = ent.lee()
                if b < 0 { break }
                bytes.append(UInt8(b))
                if b == 10 { break }
            }
            if bytes.isEmpty { c.fallo = true; return .ptrNulo }
            try escribeC(bytes, en: p, linea)
            return .p(p)
        case "fgetc", "getc":
            let c = try canalDe(try arg(a, 0, n, linea), linea)
            guard let ent = entradaDe(c) else { return .n(-1, .int) }
            let b = ent.lee()
            if b < 0 { c.fallo = true }
            return .n(Int64(b), .int)
        case "ungetc":
            let c = try canalDe(try arg(a, 1, n, linea), linea)
            entradaDe(c)?.devuelve()
            return .n(try argEnt(a, 0, n, linea), .int)
        case "fscanf":
            let c = try canalDe(try arg(a, 0, n, linea), linea)
            guard let ent = entradaDe(c) else { return .n(-1, .int) }
            let f = try formatoBytes(try arg(a, 1, n, linea), linea)
            let r = try escanea(f, a, 2, ent, linea)
            if r < 0 { c.fallo = true }
            return .n(Int64(r), .int)
        case "fopen":
            let nombre = try cadenaC(try arg(a, 0, n, linea), linea)
            let modo = try cadenaC(try arg(a, 1, n, linea), linea)
            return try abreArchivo(nombre, modo, linea)
        case "fclose":
            let c = try canalDe(try arg(a, 0, n, linea), linea)
            try cierraCanal(c)
            return .n(0, .int)
        case "feof":
            let c = try canalDe(try arg(a, 0, n, linea), linea)
            guard let ent = entradaDe(c) else { return .n(1, .int) }
            return .n(ent.mira() < 0 || c.fallo ? 1 : 0, .int)
        case "fflush":
            vuelca()
            return .n(0, .int)
        case "rewind":
            let c = try canalDe(try arg(a, 0, n, linea), linea)
            c.ent?.i = 0
            c.fallo = false
            return .vacio
        case "perror":
            let s = a.isEmpty ? "" : try cadenaC(try arg(a, 0, n, linea), linea)
            try emite(s + ": error\n")
            return .vacio
        case "remove":
            return .n(-1, .int)
        case "rename", "setbuf", "setvbuf":
            return .n(0, .int)
        default:
            return nil
        }
    }

    func formatoBytes(_ v: CV, _ linea: Int) throws -> [UInt8] {
        switch v {
        case .p(let p): return try bytesC(p, linea)
        case .s: return Array(try cadenaC(v, linea).utf8)
        default: throw CFallo("el formato de printf/scanf tiene que ser un texto entre comillas", linea)
        }
    }

    // MARK: archivos

    func canalDe(_ v: CV, _ linea: Int) throws -> CCanal {
        switch v {
        case .canal(let c): return c
        case .p(let p):
            if let mem = p.mem, p.i < mem.a.count, case .canal(let c) = mem.a[p.i] { return c }
            throw CFallo("archivo NULL (¿falló fopen?)", linea)
        case .nulo, .n(0, _): throw CFallo("archivo NULL (¿falló fopen?)", linea)
        default: throw CFallo("se esperaba un FILE* y llegó \(v.tipoNombre)", linea)
        }
    }

    func entradaDe(_ c: CCanal) -> CEntrada? {
        if c.clase == .entrada { vuelca(); return entrada }
        return c.ent
    }

    func escribeCanal(_ c: CCanal, _ bytes: [UInt8]) throws {
        switch c.clase {
        case .salida, .error: try emite(bytes)
        case .archivoSalida, .cadena: c.buf += bytes
        default: throw CFallo("no se puede escribir en un archivo abierto para leer", 0)
        }
    }

    func abreArchivo(_ nombre: String, _ modo: String, _ linea: Int) throws -> CV {
        if modo.hasPrefix("r") {
            guard let bytes = leerArchivo(nombre) else { return .ptrNulo }
            let c = CCanal(.archivoEntrada)
            c.ent = CEntrada(String(decoding: bytes, as: UTF8.self))
            c.nombre = nombre
            return .canal(c)
        }
        let c = CCanal(.archivoSalida)
        c.nombre = nombre
        c.anexar = modo.hasPrefix("a")
        if !escribirArchivo(nombre, [], c.anexar) { return .ptrNulo }
        abiertos.append(c)
        return .canal(c)
    }

    func cierraCanal(_ c: CCanal) throws {
        guard c.abierto else { return }
        c.abierto = false
        if c.clase == .archivoSalida {
            _ = escribirArchivo(c.nombre, c.buf, c.anexar)
            c.anexar = true
            c.buf = []
        }
        abiertos.removeAll { $0 === c }
    }

    func cierraTodo() {
        for c in abiertos { try? cierraCanal(c) }
    }

    // MARK: printf

    func formatea(_ fmt: [UInt8], _ a: [CV], _ desde: Int, _ linea: Int) throws -> [UInt8] {
        var out: [UInt8] = []
        var i = 0
        var k = desde
        func siguiente() throws -> CV {
            defer { k += 1 }
            guard k < a.count else { return .n(0, .int) }
            if case .ref(let r) = a[k] { return try lee(r.l) }
            return a[k]
        }
        while i < fmt.count {
            let c = fmt[i]
            if c != 37 { out.append(c); i += 1; continue }
            i += 1
            guard i < fmt.count else { break }
            if fmt[i] == 37 { out.append(37); i += 1; continue }
            var flags = ""
            while i < fmt.count, let ch = CMaq.banderas[fmt[i]] { flags.append(ch); i += 1 }
            var ancho = ""
            if i < fmt.count && fmt[i] == 42 {
                let w = try entero(try siguiente(), linea)
                if w < 0 { flags.append("-") }
                ancho = String(abs(w))
                i += 1
            }
            while i < fmt.count && fmt[i] >= 48 && fmt[i] <= 57 { ancho.append(Character(UnicodeScalar(fmt[i]))); i += 1 }
            var prec: String?
            if i < fmt.count && fmt[i] == 46 {
                i += 1
                var p = ""
                if i < fmt.count && fmt[i] == 42 { p = String(max(0, try entero(try siguiente(), linea))); i += 1 }
                while i < fmt.count && fmt[i] >= 48 && fmt[i] <= 57 { p.append(Character(UnicodeScalar(fmt[i]))); i += 1 }
                prec = p.isEmpty ? "0" : p
            }
            var largo = 0
            while i < fmt.count, [104, 108, 76, 122, 106, 116, 113].contains(fmt[i]) {
                if fmt[i] == 108 || fmt[i] == 76 || fmt[i] == 113 || fmt[i] == 106 || fmt[i] == 122 { largo += 1 }
                i += 1
            }
            guard i < fmt.count else { break }
            let conv = fmt[i]
            i += 1
            let spec = "%" + flags + ancho + (prec.map { "." + $0 } ?? "")
            out += try unFormato(conv, spec, flags, ancho, prec, largo, try siguienteSiHace(conv, siguiente), linea)
        }
        return out
    }

    static let banderas: [UInt8: Character] = [45: "-", 43: "+", 32: " ", 35: "#", 48: "0"]

    func siguienteSiHace(_ conv: UInt8, _ sig: () throws -> CV) throws -> CV {
        if conv == 110 { return .vacio }
        return try sig()
    }

    func unFormato(_ conv: UInt8, _ spec: String, _ flags: String, _ ancho: String, _ prec: String?, _ largo: Int, _ v: CV, _ linea: Int) throws -> [UInt8] {
        switch conv {
        case 100, 105:
            let x = try entero(v, linea)
            return Array(String(format: spec + "lld", x).utf8)
        case 117, 120, 88, 111:
            var x: UInt64
            if case .n(let raw, let k) = v {
                x = (k.rango <= 3 && largo == 0) ? UInt64(UInt32(truncatingIfNeeded: raw)) : UInt64(bitPattern: raw)
            } else {
                x = UInt64(bitPattern: try entero(v, linea))
            }
            if largo == 0 && x > UInt64(UInt32.max) { x = UInt64(UInt32(truncatingIfNeeded: x)) }
            let letra = conv == 117 ? "u" : String(UnicodeScalar(conv))
            return Array(String(format: spec + "ll" + letra, x).utf8)
        case 102, 70, 101, 69, 103, 71, 97, 65:
            let d = try real(v, linea)
            return Array(String(format: spec + String(UnicodeScalar(conv)), d).utf8)
        case 99:
            let ch = UInt8(truncatingIfNeeded: try entero(v, linea))
            return CMaq.rellena([ch], ancho, izquierda: flags.contains("-"))
        case 115:
            var bytes: [UInt8]
            switch v {
            case .p(let p): bytes = p.mem == nil ? Array("(null)".utf8) : try bytesC(p, linea)
            case .s: bytes = Array(try cadenaC(v, linea).utf8)
            default: bytes = Array(try texto(v, linea).utf8)
            }
            if let p = prec, let n = Int(p), n < bytes.count { bytes = Array(bytes.prefix(n)) }
            return CMaq.rellena(bytes, ancho, izquierda: flags.contains("-"))
        case 112:
            guard case .p(let p) = v, let mem = p.mem else { return Array("(nil)".utf8) }
            let dir = (UInt(bitPattern: ObjectIdentifier(mem).hashValue) & 0xFFFFFFFFFF) + UInt(p.i * 4)
            return CMaq.rellena(Array(("0x" + String(dir, radix: 16)).utf8), ancho, izquierda: flags.contains("-"))
        case 110:
            return []
        default:
            return Array("%".utf8) + [conv]
        }
    }

    static func rellena(_ b: [UInt8], _ ancho: String, izquierda: Bool) -> [UInt8] {
        guard let w = Int(ancho), w > b.count else { return b }
        let pad = [UInt8](repeating: 32, count: w - b.count)
        return izquierda ? b + pad : pad + b
    }

    // MARK: scanf

    func escanea(_ fmt: [UInt8], _ a: [CV], _ desde: Int, _ ent: CEntrada, _ linea: Int) throws -> Int {
        var asignados = 0
        var i = 0
        var k = desde
        while i < fmt.count {
            let c = fmt[i]
            if c == 32 || c == 10 || c == 9 {
                ent.saltaBlancos()
                i += 1
                continue
            }
            if c != 37 {
                if ent.mira() == Int(c) { _ = ent.lee(); i += 1; continue }
                if ent.mira() < 0 && asignados == 0 { return -1 }
                return asignados
            }
            i += 1
            guard i < fmt.count else { break }
            if fmt[i] == 37 {
                ent.saltaBlancos()
                if ent.mira() == 37 { _ = ent.lee() }
                i += 1
                continue
            }
            var suprime = false
            if fmt[i] == 42 { suprime = true; i += 1 }
            var ancho = 0
            while i < fmt.count && fmt[i] >= 48 && fmt[i] <= 57 { ancho = ancho * 10 + Int(fmt[i]) - 48; i += 1 }
            var largo = 0
            var corto = false
            while i < fmt.count, [104, 108, 76, 122, 106, 116].contains(fmt[i]) {
                if fmt[i] == 104 { corto = true } else { largo += 1 }
                i += 1
            }
            guard i < fmt.count else { break }
            let conv = fmt[i]
            i += 1
            var conjunto: Set<UInt8>?
            var negado = false
            if conv == 91 {
                var cj: Set<UInt8> = []
                if i < fmt.count && fmt[i] == 94 { negado = true; i += 1 }
                if i < fmt.count && fmt[i] == 93 { cj.insert(93); i += 1 }
                while i < fmt.count && fmt[i] != 93 {
                    if i + 2 < fmt.count && fmt[i + 1] == 45 && fmt[i + 2] != 93 {
                        for x in fmt[i]...fmt[i + 2] { cj.insert(x) }
                        i += 3
                    } else { cj.insert(fmt[i]); i += 1 }
                }
                i += 1
                conjunto = cj
            }
            let leido = try leeConversion(conv, ancho, largo, corto, conjunto, negado, ent, linea)
            guard let valor = leido else {
                if asignados == 0 && ent.mira() < 0 { return -1 }
                return asignados
            }
            if suprime { continue }
            guard k < a.count else { return asignados }
            try guardaEscaneo(a[k], valor, linea)
            k += 1
            asignados += 1
        }
        return asignados
    }

    enum CLeido { case num(CV), texto([UInt8]) }

    func leeConversion(_ conv: UInt8, _ ancho: Int, _ largo: Int, _ corto: Bool, _ cj: Set<UInt8>?, _ negado: Bool, _ ent: CEntrada, _ linea: Int) throws -> CLeido? {
        switch conv {
        case 100, 105, 117:
            guard let v = ent.entero() else { return nil }
            let k: CNum = largo > 0 ? (conv == 117 ? .ulong : .long) : (corto ? .short : (conv == 117 ? .uint : .int))
            return .num(.n(cAjusta(v, k), k))
        case 120, 88:
            guard let w = ent.palabra(), let v = Int64(String(decoding: w, as: UTF8.self).replacingOccurrences(of: "0x", with: ""), radix: 16) else { return nil }
            return .num(.n(v, largo > 0 ? .long : .int))
        case 102, 101, 103, 69, 71:
            guard let d = ent.real() else { return nil }
            return .num(.d(largo > 0 ? d : Double(Float(d)), largo == 0))
        case 115:
            ent.saltaBlancos()
            var out: [UInt8] = []
            while ent.hay() {
                let b = UInt8(ent.mira())
                if b == 32 || b == 10 || b == 9 || b == 13 { break }
                if ancho > 0 && out.count >= ancho { break }
                out.append(b)
                ent.i += 1
            }
            return out.isEmpty ? nil : .texto(out)
        case 99:
            let n = max(1, ancho)
            var out: [UInt8] = []
            while out.count < n {
                let b = ent.lee()
                if b < 0 { break }
                out.append(UInt8(b))
            }
            if out.isEmpty { return nil }
            if ancho <= 1 { return .num(.n(Int64(Int8(bitPattern: out[0])), .char)) }
            return .texto(out)
        case 91:
            var out: [UInt8] = []
            let cjto = cj ?? []
            while ent.hay() {
                let b = UInt8(ent.mira())
                let dentro = cjto.contains(b)
                if dentro == negado { break }
                if ancho > 0 && out.count >= ancho { break }
                out.append(b)
                ent.i += 1
            }
            return out.isEmpty ? nil : .texto(out)
        default:
            return nil
        }
    }

    func guardaEscaneo(_ destino: CV, _ v: CLeido, _ linea: Int) throws {
        var l: CLugar
        switch destino {
        case .p(let p):
            guard let mem = p.mem else { throw CFallo("scanf: puntero NULL (¿olvidaste el &?)", linea) }
            l = .celda(mem, p.i)
            if case .texto(let bytes) = v {
                try escribeC(bytes, en: p, linea)
                return
            }
        case .ref(let r): l = r.l
        default:
            throw CFallo("scanf necesita la dirección de la variable: escribe &\(destino.tipoNombre == "int" ? "x" : "variable")", linea)
        }
        guard case .num(let x) = v else { return }
        let actual = try lee(l)
        switch actual {
        case .n(_, let k): try escribe(l, convierte(x, .num(k)))
        case .d(_, let f): try escribe(l, convierte(x, .real(f)))
        default: try escribe(l, x)
        }
    }

    // MARK: stdlib

    func nativaStdlib(_ n: String, _ a: [CV], _ linea: Int) throws -> CV? {
        switch n {
        case "malloc":
            let bytes = Int(try argEnt(a, 0, n, linea))
            if bytes < 0 || bytes > 400_000_000 { return .ptrNulo }
            let mem = CMem([])
            mem.crudo = max(bytes, 1)
            return .p(CPtr(mem: mem, i: 0))
        case "calloc":
            let bytes = Int(try argEnt(a, 0, n, linea) * (try argEnt(a, 1, n, linea)))
            if bytes < 0 || bytes > 400_000_000 { return .ptrNulo }
            let mem = CMem([])
            mem.crudo = max(bytes, 1)
            mem.relleno = true
            return .p(CPtr(mem: mem, i: 0))
        case "realloc":
            let p = try argPtr(a, 0, n, linea)
            let bytes = Int(try argEnt(a, 1, n, linea))
            guard let mem = p.mem else {
                let x = CMem([])
                x.crudo = max(bytes, 1)
                return .p(CPtr(mem: x, i: 0))
            }
            if mem.crudo > 0 { mem.crudo = max(bytes, 1); return .p(p) }
            let tam = max(1, tamano(mem.elem ?? .num(.char)))
            let nuevo = max(1, (bytes + tam - 1) / tam)
            if nuevo < mem.a.count { mem.a.removeLast(mem.a.count - nuevo) }
            while mem.a.count < nuevo { mem.a.append(try valorInicialT(mem.elem ?? .num(.char))) }
            return .p(p)
        case "free":
            let p = try argPtr(a, 0, n, linea)
            if let mem = p.mem {
                if mem.libre { throw CFallo("doble free: esta memoria ya se había liberado", linea) }
                mem.libre = true
            }
            return .vacio
        case "exit":
            throw CSalida(codigo: Int(try argEnt(a, 0, n, linea)))
        case "abort":
            throw CFallo("abort(): el programa se abortó", linea)
        case "atoi", "atol", "atoll":
            let s = try cadenaC(try arg(a, 0, n, linea), linea)
            let v = CMaq.prefijoEntero(s, 10).0
            return n == "atoi" ? .n(cAjusta(v, .int), .int) : .n(v, .long)
        case "atof":
            return .d(CMaq.prefijoReal(try cadenaC(try arg(a, 0, n, linea), linea)).0, false)
        case "strtol", "strtoll", "strtoul", "strtoull":
            let p = try argPtr(a, 0, n, linea)
            let s = String(decoding: try bytesC(p, linea), as: UTF8.self)
            let base = a.count > 2 ? Int(try argEnt(a, 2, n, linea)) : 10
            let (v, usados) = CMaq.prefijoEntero(s, base == 0 ? 10 : base)
            try finStrto(a, 1, p, usados, linea)
            return .n(v, n.hasSuffix("ul") || n.hasSuffix("ull") ? .ulong : .long)
        case "strtod", "strtof":
            let p = try argPtr(a, 0, n, linea)
            let s = String(decoding: try bytesC(p, linea), as: UTF8.self)
            let (v, usados) = CMaq.prefijoReal(s)
            try finStrto(a, 1, p, usados, linea)
            return .d(v, n == "strtof")
        case "rand":
            return .n(rand.siguiente(), .int)
        case "srand":
            rand.siembra(UInt32(truncatingIfNeeded: try argEnt(a, 0, n, linea)))
            return .vacio
        case "abs":
            let v = try arg(a, 0, n, linea)
            if case .d(let d, let f) = v, dialecto == .cpp { return .d(Swift.abs(d), f) }
            let x = try entero(v, linea)
            return .n(cAjusta(x < 0 ? 0 &- x : x, .int), .int)
        case "labs", "llabs":
            let x = try argEnt(a, 0, n, linea)
            return .n(x < 0 ? 0 &- x : x, .long)
        case "qsort":
            try qsort(a, linea)
            return .vacio
        case "bsearch":
            return try bsearch(a, linea)
        case "system":
            return .n(-1, .int)
        case "getenv":
            return .ptrNulo
        case "time":
            let t = Int64(Date().timeIntervalSince1970)
            if !a.isEmpty, case .p(let p) = try arg(a, 0, n, linea), let mem = p.mem { mem.a[p.i] = .n(t, .long) }
            return .n(t, .long)
        case "clock":
            return .n(Int64(Date().timeIntervalSince(inicio) * 1_000_000), .long)
        case "difftime":
            return .d(try argReal(a, 0, n, linea) - (try argReal(a, 1, n, linea)), false)
        case "sleep", "usleep":
            vuelca()
            var seg = try argReal(a, 0, n, linea)
            if n == "usleep" { seg /= 1_000_000 }
            try duerme(seg)
            return .n(0, .int)
        case "assert":
            if !aBool(try arg(a, 0, n, linea)) { throw CFallo("assert falló (la condición es falsa)", linea) }
            return .vacio
        case "clrscr", "gotoxy", "va_start", "va_end":
            return .vacio
        case "kbhit":
            return .n(0, .int)
        default:
            return nil
        }
    }

    func duerme(_ segundos: Double) throws {
        var resta = min(segundos, 30)
        while resta > 0 {
            if cancelado() { throw CFallo("interrumpido", 0) }
            let t = min(resta, 0.1)
            Thread.sleep(forTimeInterval: t)
            resta -= t
        }
    }

    func finStrto(_ a: [CV], _ k: Int, _ p: CPtr, _ usados: Int, _ linea: Int) throws {
        guard k < a.count, case .p(let dest) = try arg(a, k, "strto", linea), let mem = dest.mem else { return }
        mem.a[dest.i] = .p(CPtr(mem: p.mem, i: p.i + usados))
    }

    static func prefijoEntero(_ s: String, _ base: Int) -> (Int64, Int) {
        let b = Array(s.utf8)
        var i = 0
        while i < b.count && (b[i] == 32 || b[i] == 9 || b[i] == 10) { i += 1 }
        var neg = false
        if i < b.count && (b[i] == 45 || b[i] == 43) { neg = b[i] == 45; i += 1 }
        if base == 16 && i + 1 < b.count && b[i] == 48 && (b[i + 1] == 120 || b[i + 1] == 88) { i += 2 }
        var v: Int64 = 0
        let ini = i
        while i < b.count {
            let d = CLexer.esHex(b[i]) ? CLexer.valorHex(b[i]) : 99
            if d >= base { break }
            v = v &* Int64(base) &+ Int64(d)
            i += 1
        }
        if i == ini { return (0, 0) }
        return (neg ? 0 &- v : v, i)
    }

    static func prefijoReal(_ s: String) -> (Double, Int) {
        let b = Array(s.utf8)
        var i = 0
        while i < b.count && (b[i] == 32 || b[i] == 9 || b[i] == 10) { i += 1 }
        var mejor: (Double, Int) = (0, 0)
        var j = i
        while j < b.count && j - i < 40 {
            j += 1
            if let d = Double(String(decoding: b[i..<j], as: UTF8.self)) { mejor = (d, j) }
        }
        return mejor
    }

    /// Ordena con un comparador que puede fallar (lanzar).
    func ordena(_ a: inout [CV], _ menor: (CV, CV) throws -> Bool) throws {
        if a.count < 2 { return }
        var aux = a
        try mezcla(&a, &aux, 0, a.count, menor)
    }

    private func mezcla(_ a: inout [CV], _ aux: inout [CV], _ lo: Int, _ hi: Int, _ menor: (CV, CV) throws -> Bool) throws {
        if hi - lo <= 12 {
            // inserción para tramos pequeños
            var i = lo + 1
            while i < hi {
                let x = a[i]
                var j = i - 1
                while j >= lo, try menor(x, a[j]) { a[j + 1] = a[j]; j -= 1 }
                a[j + 1] = x
                i += 1
            }
            return
        }
        let mid = (lo + hi) / 2
        try mezcla(&a, &aux, lo, mid, menor)
        try mezcla(&a, &aux, mid, hi, menor)
        if !(try menor(a[mid], a[mid - 1])) { return }
        for k in lo..<hi { aux[k] = a[k] }
        var i = lo
        var j = mid
        var k = lo
        while i < mid && j < hi {
            if try menor(aux[j], aux[i]) { a[k] = aux[j]; j += 1 } else { a[k] = aux[i]; i += 1 }
            k += 1
        }
        while i < mid { a[k] = aux[i]; i += 1; k += 1 }
        while j < hi { a[k] = aux[j]; j += 1; k += 1 }
    }

    func qsort(_ a: [CV], _ linea: Int) throws {
        let p = try argPtr(a, 0, "qsort", linea)
        let n = Int(try argEnt(a, 1, "qsort", linea))
        let cmp = try arg(a, 3, "qsort", linea)
        guard let mem = p.mem else { return }
        try prepara(mem, nil, linea)
        guard n > 0 && p.i + n <= mem.a.count else { return }
        var trozo = Array(mem.a[p.i..<(p.i + n)])
        try ordena(&trozo) { x, y in
            let px = CV.p(CPtr(mem: CMem([x]), i: 0))
            let py = CV.p(CPtr(mem: CMem([y]), i: 0))
            return try self.entero(try self.invoca(cmp, [px, py], linea), linea) < 0
        }
        for k in 0..<n { mem.a[p.i + k] = trozo[k] }
    }

    func bsearch(_ a: [CV], _ linea: Int) throws -> CV {
        let clave = try arg(a, 0, "bsearch", linea)
        let p = try argPtr(a, 1, "bsearch", linea)
        let n = Int(try argEnt(a, 2, "bsearch", linea))
        let cmp = try arg(a, 4, "bsearch", linea)
        guard let mem = p.mem else { return .ptrNulo }
        var lo = 0
        var hi = n - 1
        while lo <= hi {
            let mid = (lo + hi) / 2
            let pm = CV.p(CPtr(mem: mem, i: p.i + mid))
            let c = try entero(try invoca(cmp, [clave, pm], linea), linea)
            if c == 0 { return pm }
            if c < 0 { hi = mid - 1 } else { lo = mid + 1 }
        }
        return .ptrNulo
    }

    // MARK: string.h

    func nativaCadenas(_ n: String, _ a: [CV], _ linea: Int) throws -> CV? {
        switch n {
        case "strlen":
            return .n(Int64(try bytesC(try argPtr(a, 0, n, linea), linea).count), .ulong)
        case "strcpy":
            let d = try argPtr(a, 0, n, linea)
            try escribeC(try bytesDe(try arg(a, 1, n, linea), linea), en: d, linea)
            return .p(d)
        case "strncpy":
            let d = try argPtr(a, 0, n, linea)
            let k = Int(try argEnt(a, 2, n, linea))
            var b = Array(try bytesDe(try arg(a, 1, n, linea), linea).prefix(k))
            while b.count < k { b.append(0) }
            try escribeC(b, en: d, linea, terminar: false)
            return .p(d)
        case "strcat", "strncat":
            let d = try argPtr(a, 0, n, linea)
            var extra = try bytesDe(try arg(a, 1, n, linea), linea)
            if n == "strncat" { extra = Array(extra.prefix(Int(try argEnt(a, 2, n, linea)))) }
            let actual = try bytesC(d, linea)
            try escribeC(extra, en: CPtr(mem: d.mem, i: d.i + actual.count), linea)
            return .p(d)
        case "strcmp", "strncmp", "strcasecmp", "strncasecmp":
            var x = try bytesDe(try arg(a, 0, n, linea), linea)
            var y = try bytesDe(try arg(a, 1, n, linea), linea)
            if n.hasPrefix("strn") || n == "strncasecmp" {
                let k = Int(try argEnt(a, 2, n, linea))
                x = Array(x.prefix(k))
                y = Array(y.prefix(k))
            }
            if n.contains("case") { x = x.map { CMaq.minus($0) }; y = y.map { CMaq.minus($0) } }
            return .n(Int64(CMaq.comparaBytes(x, y)), .int)
        case "strchr", "strrchr":
            let p = try argPtr(a, 0, n, linea)
            let c = UInt8(truncatingIfNeeded: try argEnt(a, 1, n, linea))
            let b = try bytesC(p, linea)
            if c == 0 { return .p(CPtr(mem: p.mem, i: p.i + b.count)) }
            let pos = n == "strchr" ? b.firstIndex(of: c) : b.lastIndex(of: c)
            guard let k = pos else { return .ptrNulo }
            return .p(CPtr(mem: p.mem, i: p.i + k))
        case "strstr":
            let p = try argPtr(a, 0, n, linea)
            let b = try bytesC(p, linea)
            let aguja = try bytesDe(try arg(a, 1, n, linea), linea)
            guard let k = CMaq.busca(aguja, en: b, desde: 0) else { return .ptrNulo }
            return .p(CPtr(mem: p.mem, i: p.i + k))
        case "strpbrk":
            let p = try argPtr(a, 0, n, linea)
            let b = try bytesC(p, linea)
            let set = Set(try bytesDe(try arg(a, 1, n, linea), linea))
            guard let k = b.firstIndex(where: { set.contains($0) }) else { return .ptrNulo }
            return .p(CPtr(mem: p.mem, i: p.i + k))
        case "strspn", "strcspn":
            let b = try bytesDe(try arg(a, 0, n, linea), linea)
            let set = Set(try bytesDe(try arg(a, 1, n, linea), linea))
            let dentro = n == "strspn"
            return .n(Int64(b.prefix { set.contains($0) == dentro }.count), .ulong)
        case "strdup":
            return nuevaCadenaC(try cadenaC(try arg(a, 0, n, linea), linea))
        case "strrev":
            let p = try argPtr(a, 0, n, linea)
            try escribeC(try bytesC(p, linea).reversed(), en: p, linea)
            return .p(p)
        case "strtok":
            return try strtok(a, linea)
        case "memset":
            try memset(a, linea)
            return try arg(a, 0, n, linea)
        case "memcpy", "memmove":
            try memcpy(a, linea)
            return try arg(a, 0, n, linea)
        case "memcmp":
            let p = try argPtr(a, 0, n, linea)
            let q = try argPtr(a, 1, n, linea)
            guard let m1 = p.mem, let m2 = q.mem else { return .n(0, .int) }
            let tam = max(1, tamano(m1.elem ?? .num(.char)))
            let k = Int(try argEnt(a, 2, n, linea)) / tam
            for j in 0..<k where p.i + j < m1.a.count && q.i + j < m2.a.count {
                let c = try compara(m1.a[p.i + j], m2.a[q.i + j], linea)
                if c != 0 { return .n(Int64(c), .int) }
            }
            return .n(0, .int)
        default:
            return nil
        }
    }

    func bytesDe(_ v: CV, _ linea: Int) throws -> [UInt8] {
        switch v {
        case .p(let p): return try bytesC(p, linea)
        default: return Array(try cadenaC(v, linea).utf8)
        }
    }

    static func minus(_ c: UInt8) -> UInt8 { (c >= 65 && c <= 90) ? c + 32 : c }
    static func mayus(_ c: UInt8) -> UInt8 { (c >= 97 && c <= 122) ? c - 32 : c }

    static func comparaBytes(_ x: [UInt8], _ y: [UInt8]) -> Int {
        let n = min(x.count, y.count)
        for k in 0..<n where x[k] != y[k] { return Int(x[k]) - Int(y[k]) }
        if x.count == y.count { return 0 }
        return x.count < y.count ? -Int(y[n]) : Int(x[n])
    }

    static func busca<T: Equatable>(_ aguja: [T], en pajar: [T], desde: Int) -> Int? {
        if aguja.isEmpty { return min(desde, pajar.count) }
        guard pajar.count >= aguja.count, desde <= pajar.count - aguja.count else { return nil }
        var i = max(0, desde)
        while i <= pajar.count - aguja.count {
            if pajar[i] == aguja[0] && Array(pajar[i..<(i + aguja.count)]) == aguja { return i }
            i += 1
        }
        return nil
    }

    func strtok(_ a: [CV], _ linea: Int) throws -> CV {
        let primero = try arg(a, 0, "strtok", linea)
        let delims = Set(try bytesDe(try arg(a, 1, "strtok", linea), linea))
        var p: CPtr
        if case .p(let x) = primero, x.mem != nil { p = x }
        else if let s = estadoStrtok { p = s }
        else { return .ptrNulo }
        guard let mem = p.mem else { return .ptrNulo }
        var i = p.i
        func byte(_ k: Int) -> UInt8 {
            guard k < mem.a.count, case .n(let c, _) = mem.a[k] else { return 0 }
            return UInt8(truncatingIfNeeded: c)
        }
        while byte(i) != 0 && delims.contains(byte(i)) { i += 1 }
        if byte(i) == 0 { estadoStrtok = nil; return .ptrNulo }
        let inicio = i
        while byte(i) != 0 && !delims.contains(byte(i)) { i += 1 }
        if byte(i) != 0 {
            mem.a[i] = .n(0, .char)
            estadoStrtok = CPtr(mem: mem, i: i + 1)
        } else {
            estadoStrtok = nil
        }
        return .p(CPtr(mem: mem, i: inicio))
    }

    func memset(_ a: [CV], _ linea: Int) throws {
        let p = try argPtr(a, 0, "memset", linea)
        let b = UInt8(truncatingIfNeeded: try argEnt(a, 1, "memset", linea))
        let bytes = Int(try argEnt(a, 2, "memset", linea))
        guard let mem = p.mem else { throw CFallo("memset sobre NULL", linea) }
        try prepara(mem, nil, linea)
        let t = mem.elem ?? (mem.a.first.map { v -> CTipo in
            if case .n(_, let k) = v { return .num(k) }
            if case .d(_, let f) = v { return .real(f) }
            return .num(.char)
        } ?? .num(.char))
        try rellenaMem(mem, p.i, bytes, b, t, linea)
    }

    func rellenaMem(_ mem: CMem, _ desde: Int, _ bytes: Int, _ b: UInt8, _ t: CTipo, _ linea: Int) throws {
        switch t {
        case .num(let k):
            var v: Int64 = 0
            for j in 0..<k.bytes { v |= Int64(b) << (8 * Int64(j)) }
            let n = bytes / k.bytes
            let x = CV.n(cAjusta(v, k), k)
            for j in 0..<n where desde + j < mem.a.count { mem.a[desde + j] = x }
        case .real(let f):
            let n = bytes / (f ? 4 : 8)
            for j in 0..<n where desde + j < mem.a.count { mem.a[desde + j] = .d(b == 0 ? 0 : Double.nan, f) }
        case .arr(let e, let cuantos):
            let tam = max(1, tamano(t))
            let n = bytes / tam
            for j in 0..<n where desde + j < mem.a.count {
                if case .p(let sub) = mem.a[desde + j], let sm = sub.mem { try rellenaMem(sm, sub.i, cuantos * tamano(e), b, e, linea) }
            }
        case .clase:
            let tam = max(1, tamano(t))
            let n = bytes / tam
            for j in 0..<n where desde + j < mem.a.count {
                if case .o(let o) = mem.a[desde + j] {
                    for (k, ft) in o.clase.tipos.enumerated() { if case .num(let q) = ft { o.c.a[k] = .n(b == 0 ? 0 : cAjusta(Int64(b), q), q) } }
                }
            }
        default:
            return
        }
    }

    func memcpy(_ a: [CV], _ linea: Int) throws {
        let d = try argPtr(a, 0, "memcpy", linea)
        let s = try argPtr(a, 1, "memcpy", linea)
        let bytes = Int(try argEnt(a, 2, "memcpy", linea))
        guard let md = d.mem, let ms = s.mem else { throw CFallo("memcpy con NULL", linea) }
        try prepara(ms, md.elem, linea)
        try prepara(md, ms.elem, linea)
        let tam = max(1, tamano(ms.elem ?? md.elem ?? .num(.char)))
        let n = bytes / tam
        guard s.i + n <= ms.a.count, d.i + n <= md.a.count else {
            throw CFallo("memcpy fuera de la memoria (\(n) elementos)", linea)
        }
        let trozo = Array(ms.a[s.i..<(s.i + n)]).map { copiaCampo($0) }
        for k in 0..<n { md.a[d.i + k] = trozo[k] }
    }

    // MARK: math.h

    func nativaMate(_ n: String, _ a: [CV], _ linea: Int) throws -> CV? {
        let esF = n.hasSuffix("f") && n.count > 3 && n != "modf"
        let base = esF ? String(n.dropLast()) : n
        func r1(_ f: (Double) -> Double) throws -> CV {
            let x = f(try argReal(a, 0, n, linea))
            return .d(esF ? Double(Float(x)) : x, esF)
        }
        func r2(_ f: (Double, Double) -> Double) throws -> CV {
            let x = f(try argReal(a, 0, n, linea), try argReal(a, 1, n, linea))
            return .d(esF ? Double(Float(x)) : x, esF)
        }
        switch base {
        case "sqrt": return try r1 { $0.squareRoot() }
        case "pow": return try r2 { mPow($0, $1) }
        case "fabs": return try r1 { Swift.abs($0) }
        case "floor": return try r1 { mFloor($0) }
        case "ceil": return try r1 { mCeil($0) }
        case "round": return try r1 { mRound($0) }
        case "trunc": return try r1 { mTrunc($0) }
        case "sin": return try r1 { mSin($0) }
        case "cos": return try r1 { mCos($0) }
        case "tan": return try r1 { mTan($0) }
        case "asin": return try r1 { mAsin($0) }
        case "acos": return try r1 { mAcos($0) }
        case "atan": return try r1 { mAtan($0) }
        case "atan2": return try r2 { mAtan2($0, $1) }
        case "sinh": return try r1 { mSinh($0) }
        case "cosh": return try r1 { mCosh($0) }
        case "tanh": return try r1 { mTanh($0) }
        case "exp": return try r1 { mExp($0) }
        case "exp2": return try r1 { mExp2($0) }
        case "log": return try r1 { logNatural($0) }
        case "log10": return try r1 { mLog10($0) }
        case "log2": return try r1 { mLog2($0) }
        case "fmod": return try r2 { mFmod($0, $1) }
        case "hypot": return try r2 { mHypot($0, $1) }
        case "cbrt": return try r1 { mCbrt($0) }
        case "fmax": return try r2 { Swift.max($0, $1) }
        case "fmin": return try r2 { Swift.min($0, $1) }
        case "lround", "llround": return .n(Int64(mRound(try argReal(a, 0, n, linea))), .long)
        case "isnan": return .n(try argReal(a, 0, n, linea).isNaN ? 1 : 0, .int)
        case "isinf": return .n(try argReal(a, 0, n, linea).isInfinite ? 1 : 0, .int)
        default: return nil
        }
    }

    // MARK: ctype.h

    func nativaCtype(_ n: String, _ a: [CV], _ linea: Int) throws -> CV? {
        guard n.hasPrefix("is") || n == "toupper" || n == "tolower" else { return nil }
        let x = try argEnt(a, 0, n, linea)
        let c = x >= 0 && x < 256 ? UInt8(x) : 0
        let r: Bool
        switch n {
        case "isalpha": r = (c >= 65 && c <= 90) || (c >= 97 && c <= 122)
        case "isdigit": r = c >= 48 && c <= 57
        case "isalnum": r = (c >= 65 && c <= 90) || (c >= 97 && c <= 122) || (c >= 48 && c <= 57)
        case "isspace": r = c == 32 || (c >= 9 && c <= 13)
        case "isupper": r = c >= 65 && c <= 90
        case "islower": r = c >= 97 && c <= 122
        case "ispunct": r = (c >= 33 && c <= 47) || (c >= 58 && c <= 64) || (c >= 91 && c <= 96) || (c >= 123 && c <= 126)
        case "isxdigit": r = CLexer.esHex(c)
        case "isprint": r = c >= 32 && c < 127
        case "isgraph": r = c > 32 && c < 127
        case "iscntrl": r = c < 32 || c == 127
        case "toupper":
            let k: CNum = dialecto == .cpp ? .int : .int
            return .n(x >= 97 && x <= 122 ? x - 32 : x, k)
        case "tolower":
            return .n(x >= 65 && x <= 90 ? x + 32 : x, .int)
        default: return nil
        }
        return .n(r ? (n == "isalpha" ? 1024 : 1) : 0, .int).normalizado(dialecto)
    }
}

extension CV {
    /// isalpha devuelve "algo distinto de cero": en C++ se muestra 1 si se imprime como bool
    func normalizado(_ d: CDialecto) -> CV {
        if case .n(let x, let k) = self, x != 0 { return .n(d == .c ? x : 1, k) }
        return self
    }
}
