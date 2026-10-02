import Foundation

// ============================================================
// MARK: - C, C++ y Java: la máquina
// ============================================================
// Estado de un programa en marcha: memoria global, clases, entrada
// y salida, y las reglas de cada lenguaje (desbordamiento de int,
// división entera, conversiones, punteros…).

/// Entrada de texto: lo que llegue por tubería o lo que se teclee.
final class CEntrada {
    var b: [UInt8]
    var i = 0
    var pide: (() -> String?)?
    var fin = false

    init(_ texto: String, pide: (() -> String?)? = nil) {
        b = Array(texto.utf8)
        self.pide = pide
    }

    /// Asegura que haya al menos un byte; false si se acabó la entrada.
    func hay() -> Bool {
        while i >= b.count {
            guard !fin, let p = pide, let linea = p() else { fin = true; return false }
            if i > 4096 { b.removeFirst(i); i = 0 }
            b += Array(linea.utf8)
            if !linea.hasSuffix("\n") { b.append(10) }
        }
        return true
    }

    func mira() -> Int { hay() ? Int(b[i]) : -1 }

    func lee() -> Int {
        guard hay() else { return -1 }
        let c = b[i]
        i += 1
        return Int(c)
    }

    func devuelve() { if i > 0 { i -= 1 } }

    func saltaBlancos() {
        while hay(), b[i] == 32 || b[i] == 10 || b[i] == 9 || b[i] == 13 { i += 1 }
    }

    func palabra() -> [UInt8]? {
        saltaBlancos()
        guard hay() else { return nil }
        var out: [UInt8] = []
        while hay(), b[i] != 32 && b[i] != 10 && b[i] != 9 && b[i] != 13 {
            out.append(b[i])
            i += 1
        }
        return out
    }

    func linea() -> [UInt8]? {
        guard hay() else { return nil }
        var out: [UInt8] = []
        while hay() {
            let c = b[i]
            i += 1
            if c == 10 { return out }
            if c != 13 { out.append(c) }
        }
        return out
    }

    func entero() -> Int64? {
        saltaBlancos()
        guard hay() else { return nil }
        var s: [UInt8] = []
        if b[i] == 45 || b[i] == 43 { s.append(b[i]); i += 1 }
        while hay(), b[i] >= 48 && b[i] <= 57 { s.append(b[i]); i += 1 }
        guard s.contains(where: { $0 >= 48 && $0 <= 57 }) else {
            if !s.isEmpty { i -= s.count }
            return nil
        }
        let txt = String(decoding: s, as: UTF8.self)
        return Int64(txt) ?? (s.first == 45 ? Int64.min : Int64.max)
    }

    func real() -> Double? {
        saltaBlancos()
        guard hay() else { return nil }
        var s: [UInt8] = []
        let validos: Set<UInt8> = Set("0123456789+-.eE".utf8)
        while hay(), validos.contains(b[i]) {
            let c = b[i]
            if (c == 43 || c == 45) && !s.isEmpty && s.last != 101 && s.last != 69 { break }
            s.append(c)
            i += 1
        }
        // inf / nan
        if s.isEmpty, hay(), b[i] == 105 || b[i] == 110 || b[i] == 73 || b[i] == 78 {
            if let w = palabra() { return Double(String(decoding: w, as: UTF8.self).lowercased()) }
        }
        return Double(String(decoding: s, as: UTF8.self))
    }
}

final class CMaq: @unchecked Sendable {
    let dialecto: CDialecto
    let prog: CPrograma
    let globales = CMem([])
    var globalIndice: [String: Int] = [:]
    var globalTipos: [CTipo] = []
    var iniCampos: [Int: CFuncion] = [:]
    var iniEstaticos: [Int: CFuncion] = [:]

    // entrada y salida
    var salida: [UInt8] = []
    var totalSalida = 0
    var volcar: ((String) -> Void)?
    var entrada: CEntrada
    let cout = CCanal(.salida)
    let cerr = CCanal(.error)
    let cin = CCanal(.entrada)
    var leerArchivo: (String) -> [UInt8]? = { _ in nil }
    var escribirArchivo: (String, [UInt8], Bool) -> Bool = { _, _, _ in false }
    var argv: [String] = []
    var nombrePrograma = "a.out"

    // control
    var pasos = 0
    var cancelado: () -> Bool = { false }
    var limite: Date?
    var profundidad = 0
    let maxProfundidad = 6000
    var ultimaExcepcion: CV?
    var semilla: UInt64 = 1
    let inicio = Date()
    var javaRandom: UInt64 = 0
    var hashSiguiente = 0x1b6d3586
    var rand = CRandGlibc(1)
    var estadoStrtok: CPtr?
    var abiertos: [CCanal] = []
    var lienzo: CLienzo?
    var ventana: CVentana?

    init(dialecto: CDialecto, prog: CPrograma, entrada: CEntrada) {
        self.dialecto = dialecto
        self.prog = prog
        self.entrada = entrada
        cin.ent = entrada
        javaRandom = (UInt64(Date().timeIntervalSince1970 * 1000) ^ 0x5DEECE66D) & ((1 << 48) - 1)
    }

    var clases: [String: CClase] { prog.clases }

    var tipoVerdad: CTipo { dialecto == .c ? .num(.int) : .num(.bool) }

    @inline(__always)
    func verdad(_ b: Bool) -> CV {
        dialecto == .c ? .n(b ? 1 : 0, .int) : .n(b ? 1 : 0, .bool)
    }

    // MARK: control de ejecución

    @inline(__always)
    func revisaPaso() throws {
        pasos &+= 1
        if pasos & 0x3FFF == 0 { try revisaLimites() }
    }

    func revisaLimites() throws {
        if cancelado() { throw CFallo("interrumpido", 0) }
        if let l = limite, Date() > l { throw CFallo("tiempo agotado: el programa tardaba demasiado (¿un bucle infinito?)", 0) }
    }

    // MARK: salida

    func emite(_ bytes: [UInt8]) throws {
        salida += bytes
        totalSalida += bytes.count
        if totalSalida > 8_000_000 { throw CFallo("el programa escribió demasiado (más de 8 MB): se detuvo", 0) }
        if volcar != nil && (bytes.contains(10) || salida.count > 4096) { vuelca() }
    }

    func emite(_ s: String) throws { try emite(Array(s.utf8)) }

    func vuelca() {
        guard let v = volcar, !salida.isEmpty else { return }
        // no se corta un carácter UTF-8 a la mitad
        var corte = salida.count
        var k = 0
        while k < 3 && corte > 0 && (salida[corte - 1] & 0xC0) == 0x80 { corte -= 1; k += 1 }
        if corte > 0 && salida[corte - 1] >= 0xC0 { corte -= 1 } else { corte = salida.count }
        let trozo = Array(salida[0..<corte])
        salida.removeFirst(corte)
        v(String(decoding: trozo, as: UTF8.self))
    }

    func textoSalida() -> String {
        String(decoding: salida, as: UTF8.self)
    }

    // MARK: valores

    func aBool(_ v: CV) -> Bool {
        switch v {
        case .n(let x, _): return x != 0
        case .d(let d, _): return d != 0
        case .p(let p): return p.mem != nil
        case .nulo, .vacio: return false
        case .canal(let c): return !c.fallo
        case .ref(let r): return aBool((try? lee(r.l)) ?? .vacio)
        default: return true
        }
    }

    func entero(_ v: CV, _ linea: Int) throws -> Int64 {
        switch v {
        case .n(let x, _): return x
        case .d(let d, _): return CV.truncaD(d)
        case .ref(let r): return try entero(try lee(r.l), linea)
        case .nulo: throw excepcion("NullPointerException", "se usó null como número", linea)
        case .it(_, let i): return Int64(i)
        default: throw CFallo("se esperaba un número y llegó \(v.tipoNombre)", linea)
        }
    }

    func real(_ v: CV, _ linea: Int) throws -> Double {
        switch v {
        case .n(let x, let k): return k == .ulong ? Double(UInt64(bitPattern: x)) : Double(x)
        case .d(let d, _): return d
        case .ref(let r): return try real(try lee(r.l), linea)
        default: throw CFallo("se esperaba un número y llegó \(v.tipoNombre)", linea)
        }
    }

    func caracter(_ u: UInt16) -> CV {
        dialecto == .java ? .n(Int64(u), .jchar) : .n(Int64(Int8(truncatingIfNeeded: u)), .char)
    }

    func cadena(_ s: String) -> CV {
        dialecto == .java ? .s(Array(s.utf16)) : .s(Array(s.utf8).map { UInt16($0) })
    }

    func unidades(_ s: String) -> [UInt16] {
        dialecto == .java ? Array(s.utf16) : Array(s.utf8).map { UInt16($0) }
    }

    func texto(_ u: [UInt16]) -> String {
        if dialecto == .java { return String(decoding: u, as: UTF16.self) }
        return String(decoding: u.map { UInt8(truncatingIfNeeded: $0) }, as: UTF8.self)
    }

    /// Bytes de una cadena de C (char*) hasta el '\0'.
    func bytesC(_ p: CPtr, _ linea: Int) throws -> [UInt8] {
        guard let mem = p.mem else { throw CFallo("cadena NULL", linea) }
        try prepara(mem, .num(.char), linea)
        var out: [UInt8] = []
        var k = p.i
        while k < mem.a.count {
            guard case .n(let c, _) = mem.a[k] else {
                if case .vacio = mem.a[k] { break }
                throw CFallo("la memoria no contiene texto", linea)
            }
            if c == 0 { return out }
            out.append(UInt8(truncatingIfNeeded: c))
            k += 1
        }
        if k >= mem.a.count && p.i < mem.a.count {
            // sin '\0': en C real esto lee basura; aquí se para al final del bloque
            return out
        }
        return out
    }

    func cadenaC(_ v: CV, _ linea: Int) throws -> String {
        switch v {
        case .p(let p): return String(decoding: try bytesC(p, linea), as: UTF8.self)
        case .s(let u): return texto(u)
        case .ref(let r): return try cadenaC(try lee(r.l), linea)
        default: throw CFallo("se esperaba una cadena (char*) y llegó \(v.tipoNombre)", linea)
        }
    }

    /// Copia bytes a una zona de char con '\0' al final.
    func escribeC(_ bytes: [UInt8], en p: CPtr, _ linea: Int, terminar: Bool = true) throws {
        guard let mem = p.mem else { throw CFallo("destino NULL", linea) }
        try prepara(mem, .num(.char), linea)
        let total = bytes.count + (terminar ? 1 : 0)
        if p.i + total > mem.a.count {
            throw CFallo("desbordamiento: se escriben \(total) bytes en un espacio de \(mem.a.count - p.i) (amplía el arreglo)", linea)
        }
        for (k, b) in bytes.enumerated() { mem.a[p.i + k] = .n(Int64(Int8(bitPattern: b)), .char) }
        if terminar { mem.a[p.i + bytes.count] = .n(0, .char) }
    }

    func nuevaCadenaC(_ s: String) -> CV {
        var celdas: [CV] = Array(s.utf8).map { CV.n(Int64(Int8(bitPattern: $0)), .char) }
        celdas.append(.n(0, .char))
        let mem = CMem(celdas)
        mem.elem = .num(.char)
        return .p(CPtr(mem: mem, i: 0))
    }

    // MARK: lugares

    func lee(_ l: CLugar) throws -> CV {
        switch l {
        case .celda(let mem, let i):
            guard i >= 0 && i < mem.a.count else { throw fueraDeRango(i, mem.a.count, 0) }
            let v = mem.a[i]
            if case .ref(let r) = v { return try lee(r.l) }
            return v
        case .lista(let lista, let i):
            guard i >= 0 && i < lista.a.count else { throw fueraDeRango(i, lista.a.count, 0) }
            return lista.a[i]
        case .mapa(let mp, let k):
            let c = try clave(k, 0)
            if let (_, v) = mp.vals[c] { return v }
            // mapa[k] en C++ crea la entrada con el valor por defecto
            let v = valorPorDefectoMapa(mp)
            mp.pon(c, k, v)
            return v
        case .car(let base, let i):
            let v = try lee(base)
            guard case .s(let u) = v, i >= 0 && i < u.count else { throw fueraDeRango(i, 0, 0) }
            return caracter(u[i])
        }
    }

    func escribe(_ l: CLugar, _ v: CV) throws {
        switch l {
        case .celda(let mem, let i):
            guard i >= 0 && i < mem.a.count else { throw fueraDeRango(i, mem.a.count, 0) }
            if mem.libre { throw CFallo("se escribió en memoria ya liberada", 0) }
            mem.a[i] = v
        case .lista(let lista, let i):
            guard i >= 0 && i < lista.a.count else { throw fueraDeRango(i, lista.a.count, 0) }
            lista.a[i] = v
        case .mapa(let mp, let k):
            mp.pon(try clave(k, 0), k, v)
        case .car(let base, let i):
            let actual = try lee(base)
            guard case .s(var u) = actual, i >= 0 && i < u.count else { throw fueraDeRango(i, 0, 0) }
            let c = try entero(v, 0)
            u[i] = UInt16(truncatingIfNeeded: dialecto == .java ? c : Int64(UInt8(truncatingIfNeeded: c)))
            try escribe(base, .s(u))
        }
    }

    func valorPorDefectoMapa(_ mp: CMapa) -> CV {
        if let t = mp.tipoValor { return (try? valorInicialT(t)) ?? .n(0, .int) }
        return .n(0, .int)
    }

    func clave(_ v: CV, _ linea: Int) throws -> CKey {
        switch v {
        case .n(let x, _): return .n(x)
        case .d(let d, _): return .d(d)
        case .s(let u): return .s(u)
        case .p(let p):
            if let mem = p.mem, mem.elem.map({ if case .num(.char) = $0 { return true } else { return false } }) ?? false {
                return .s(unidades(try cadenaC(v, linea)))
            }
            if let mem = p.mem { return .t([.o(ObjectIdentifier(mem)), .n(Int64(p.i))]) }
            return .n(0)
        case .o(let o):
            if o.clase.nombre == "pair" || o.clase.nombre == "$tupla" {
                return .t(try o.c.a.map { try clave($0, linea) })
            }
            if dialecto == .java, o.clase.tieneMetodo("hashCode"), o.clase.tieneMetodo("equals") {
                // con equals propio: la clave son los valores de sus campos
                return .t(try o.c.a.map { try clave($0, linea) })
            }
            if dialecto == .cpp, o.clase.tieneMetodo("operator<") || o.clase.tieneMetodo("operator==") {
                return .t(try o.c.a.map { try clave($0, linea) })
            }
            if dialecto == .java, o.clase.esEnum, let i = o.clase.indice["$ordinal"], case .n(let x, _) = o.c.a[i] {
                return .n(x)
            }
            return .o(ObjectIdentifier(o))
        case .l(let l): return .t(try l.a.map { try clave($0, linea) })
        case .ref(let r): return try clave(try lee(r.l), linea)
        case .nulo: return .o(ObjectIdentifier(globales))
        default: throw CFallo("no se puede usar \(v.tipoNombre) como clave", linea)
        }
    }

    // MARK: errores

    func fueraDeRango(_ i: Int, _ n: Int, _ linea: Int) -> Error {
        if dialecto == .java {
            return excepcion("ArrayIndexOutOfBoundsException", "Index \(i) out of bounds for length \(n)", linea)
        }
        return CFallo("índice fuera del arreglo: posición \(i), tamaño \(n) (en C esto corrompe la memoria)", linea)
    }

    /// Excepción del lenguaje: en Java (y en C++ para las de la biblioteca)
    /// se puede atrapar con catch; en C termina el programa.
    func excepcion(_ clase: String, _ msg: String, _ linea: Int) -> Error {
        if dialecto != .c, let c = clases[clase], let o = try? nuevoObjeto(c, [cadena(msg)], linea) {
            let v = CV.o(o)
            ultimaExcepcion = v
            return CLanzado(v: v, linea: linea)
        }
        return CFallo(dialecto == .java ? "\(clase): \(msg)" : msg, linea)
    }

    func excepcionDeFallo(_ e: CFallo) -> CV? {
        guard dialecto == .java, let c = clases["RuntimeException"], let o = try? nuevoObjeto(c, [cadena(e.msg)], e.linea) else { return nil }
        return .o(o)
    }

    func atrapa(_ c: CCaptura, _ v: CV) -> Bool {
        if c.tipos.isEmpty { return true }
        for t in c.tipos where esInstancia(v, t) { return true }
        return false
    }

    func esInstancia(_ v: CV, _ t: CTipo) -> Bool {
        let tt = t.sinRef
        switch (v, tt) {
        case (.o(let o), .clase(let n)):
            return o.clase.esSubclase(de: n) || n == "Object" || n == "Throwable" && o.clase.esSubclase(de: "Exception")
        case (.o, .auto): return true
        case (.o(let o), .ptr(.clase(let n))): return o.clase.esSubclase(de: n)
        case (.n(_, let k), .num(let q)): return k == q || (k.rango >= 3 && q.rango >= 3) || dialecto == .cpp
        case (.n, .auto), (.d, .auto), (.s, .auto), (.l, .auto), (.m, .auto): return true
        case (.d, .real): return true
        case (.s, .cad): return true
        case (.p, .ptr), (.p, .cad): return true
        case (.s, .ptr): return true
        case (.l(let l), .cont(let n, _)): return CMaq.familia(l.k, n)
        case (.m(let mp), .cont(let n, _)): return CMaq.familia(mp.k, n)
        case (.l, .clase(let n)), (.m, .clase(let n)), (.s, .clase(let n)), (.n, .clase(let n)), (.d, .clase(let n)):
            return n == "Object"
        default: return false
        }
    }

    /// instanceof de Java, con los tipos envoltorio (Integer, Double…)
    func esInstanciaJava(_ v: CV, _ n: String, _ t: CTipo) -> Bool {
        if v.esNulo { return false }
        switch (n, v) {
        case ("Integer", .n(_, let k)): return k == .int || k == .short
        case ("Long", .n(_, let k)): return k == .long
        case ("Short", .n(_, let k)): return k == .short
        case ("Byte", .n(_, let k)): return k == .char
        case ("Character", .n(_, let k)): return k == .jchar
        case ("Boolean", .n(_, let k)): return k == .bool
        case ("Double", .d(_, let f)): return !f
        case ("Float", .d(_, let f)): return f
        case ("Number", .n(_, let k)): return k != .bool && k != .jchar
        case ("Number", .d): return true
        case ("String", .s), ("CharSequence", .s), ("Comparable", .s): return true
        case ("Object", _): return true
        case ("Integer", _), ("Long", _), ("Short", _), ("Byte", _), ("Character", _), ("Boolean", _),
             ("Double", _), ("Float", _), ("Number", _), ("String", _): return false
        default: return esInstancia(v, t)
        }
    }

    static func familia(_ real: String, _ pedido: String) -> Bool {
        if real == pedido { return true }
        let listas: Set<String> = ["ArrayList", "LinkedList", "List", "Collection", "Vector", "Iterable"]
        let mapas: Set<String> = ["HashMap", "TreeMap", "LinkedHashMap", "Map"]
        let conjuntos: Set<String> = ["HashSet", "TreeSet", "LinkedHashSet", "Set", "Collection"]
        if listas.contains(real) && listas.contains(pedido) { return true }
        if mapas.contains(real) && mapas.contains(pedido) { return true }
        if conjuntos.contains(real) && conjuntos.contains(pedido) { return true }
        return false
    }

    // MARK: conversiones

    func convierte(_ v: CV, _ t: CTipo) -> CV {
        switch t {
        case .num(let k):
            switch v {
            case .n(let x, let q): return q == k ? v : .n(cAjusta(x, k), k)
            case .d(let d, _): return k == .bool ? .n(d != 0 ? 1 : 0, .bool) : .n(cAjusta(CV.truncaD(d), k), k)
            case .p(let p): return k == .bool ? .n(p.mem != nil ? 1 : 0, .bool) : .n(Int64(p.i), k)
            case .nulo: return dialecto == .java ? v : .n(0, k)
            case .canal(let c): return .n(c.fallo ? 0 : 1, k)
            case .ref(let r): return convierte((try? lee(r.l)) ?? .vacio, t)
            case .it(_, let i): return .n(Int64(i), k)
            default: return v
            }
        case .real(let f):
            switch v {
            case .n(let x, let q):
                let d = q == .ulong ? Double(UInt64(bitPattern: x)) : Double(x)
                return .d(f ? Double(Float(d)) : d, f)
            case .d(let d, let g):
                if g == f { return v }
                return .d(f ? Double(Float(d)) : d, f)
            case .ref(let r): return convierte((try? lee(r.l)) ?? .vacio, t)
            default: return v
            }
        case .ptr(let e):
            switch v {
            case .n(0, _), .nulo: return .ptrNulo
            case .p(let p):
                if let mem = p.mem, mem.crudo > 0 { try? materializa(mem, e) }
                return v
            default: return v
            }
        case .cad:
            if dialecto == .cpp, case .p = v, let s = try? cadenaC(v, 0) { return cadena(s) }
            return v
        case .ref(let x):
            return convierte(v, x)
        default:
            return v
        }
    }

    /// C y C++ copian structs, vectores y strings al asignar; Java no.
    func copia(_ v: CV) -> CV {
        guard dialecto != .java else { return v }
        switch v {
        case .o(let o):
            if o.clase.nombre == "$excepcionNativa" { return v }
            let nuevo = CMem(o.c.a.map { copiaCampo($0) })
            let x = CObj(o.clase, nuevo)
            x.exterior = o.exterior
            return .o(x)
        case .l(let l):
            let x = CLista(l.k, l.a.map { copia($0) })
            x.cmp = l.cmp
            x.max = l.max
            return .l(x)
        case .m(let mp):
            let x = CMapa(mp.k, ordenado: mp.ordenado, esSet: mp.esSet)
            x.multi = mp.multi
            x.claves = mp.claves
            x.cuentas = mp.cuentas
            x.tipoValor = mp.tipoValor
            for (k, par) in mp.vals { x.vals[k] = (par.0, copia(par.1)) }
            return .m(x)
        default:
            return v
        }
    }

    /// Un arreglo dentro de un struct se copia entero.
    func copiaCampo(_ v: CV) -> CV {
        if case .p(let p) = v, let mem = p.mem, mem.esArreglo {
            let x = CMem(mem.a.map { copiaCampo($0) })
            x.elem = mem.elem
            x.esArreglo = true
            return .p(CPtr(mem: x, i: p.i))
        }
        return copia(v)
    }

    func valorInicial(_ t: CTipo) -> CV {
        (try? valorInicialT(t)) ?? .n(0, .int)
    }

    /// Valor de una variable recién declarada sin inicializar.
    func valorInicialT(_ t: CTipo) throws -> CV {
        switch t {
        case .num(let k): return .n(0, k)
        case .real(let f): return .d(0, f)
        case .ptr, .fn: return dialecto == .java ? .nulo : .ptrNulo
        case .cad: return dialecto == .java ? .nulo : .s([])
        case .arr(let e, let n):
            if dialecto == .java { return .nulo }
            return try nuevoArreglo(e, max(n, 0))
        case .clase(let n):
            if dialecto == .java { return .nulo }
            guard let c = clases[n] else { return .n(0, .int) }
            return .o(try nuevoObjeto(c, [], 0))
        case .cont(let n, let args):
            if dialecto == .java { return .nulo }
            return try nuevoContenedor(n, args)
        case .canal: return .canal(CCanal(.cadena))
        case .auto: return dialecto == .java ? .nulo : .n(0, .int)
        default: return .vacio
        }
    }

    func nuevoArreglo(_ e: CTipo, _ n: Int) throws -> CV {
        var celdas: [CV] = []
        celdas.reserveCapacity(n)
        for _ in 0..<n { celdas.append(try valorInicialT(e)) }
        let mem = CMem(celdas)
        mem.elem = e
        mem.esArreglo = true
        return .p(CPtr(mem: mem, i: 0))
    }

    func nuevoContenedor(_ n: String, _ args: [CTipo]) throws -> CV {
        switch n {
        case "map", "unordered_map", "multimap", "HashMap", "TreeMap", "LinkedHashMap", "Map":
            let ordenado = n == "map" || n == "multimap" || n == "TreeMap"
            let mp = CMapa(n, ordenado: ordenado, esSet: false)
            mp.multi = n == "multimap"
            if args.count > 1 { mp.tipoValor = args[1] }
            return .m(mp)
        case "set", "unordered_set", "multiset", "HashSet", "TreeSet", "LinkedHashSet", "Set":
            let ordenado = n == "set" || n == "multiset" || n == "TreeSet"
            let mp = CMapa(n, ordenado: ordenado, esSet: true)
            mp.multi = n == "multiset"
            return .m(mp)
        case "pair":
            let a = args.count > 0 ? try valorInicialT(args[0]) : .n(0, .int)
            let b = args.count > 1 ? try valorInicialT(args[1]) : .n(0, .int)
            return par(a, b)
        case "stringstream", "istringstream", "ostringstream":
            let c = CCanal(.cadena)
            c.ent = CEntrada("")
            return .canal(c)
        case "array":
            return .l(CLista("vector"))
        default:
            let l = CLista(n)
            l.elem = args.first
            if n == "priority_queue" || n == "PriorityQueue" {
                l.max = n == "priority_queue"
                if args.count >= 3 { try comparadorDeTipo(args[2], l) }
            }
            return .l(l)
        }
    }

    func par(_ a: CV, _ b: CV) -> CV {
        let c = clasePar()
        return .o(CObj(c, CMem([a, b])))
    }

    func clasePar() -> CClase {
        if let c = prog.clases["pair"] { return c }
        let c = prog.clase("pair", 0)
        c.indice = ["first": 0, "second": 1]
        c.tipos = [.auto, .auto]
        c.campos = [CCampo(nombre: "first", tipo: .auto, ini: nil, estatico: false, linea: 0),
                    CCampo(nombre: "second", tipo: .auto, ini: nil, estatico: false, linea: 0)]
        c.preparada = true
        c.listo = true
        c.nativa = true
        return c
    }

    // MARK: memoria de malloc

    /// Da tipo a la memoria de malloc cuando se usa por primera vez.
    func prepara(_ mem: CMem, _ t: CTipo?, _ linea: Int) throws {
        guard mem.crudo > 0 else { return }
        try materializa(mem, t ?? .num(.char))
    }

    func materializa(_ mem: CMem, _ t: CTipo) throws {
        guard mem.crudo > 0 else { return }
        let tam = max(1, tamano(t))
        let n = max(1, (mem.crudo + tam - 1) / tam)
        let relleno = mem.relleno
        mem.crudo = 0
        mem.elem = t
        var celdas: [CV] = []
        celdas.reserveCapacity(n)
        for _ in 0..<n {
            if relleno, case .num(let k) = t { celdas.append(.n(0, k)) }
            else { celdas.append(try valorInicialT(t)) }
        }
        mem.a = celdas
    }

    func objetoEnCelda(_ mem: CMem, _ i: Int, _ t: CTipo?, _ campo: String, _ linea: Int) throws -> CV {
        var clase: CClase?
        if case .clase(let n)? = t?.sinRef { clase = clases[n] }
        if clase == nil {
            let candidatas = prog.ordenClases.filter { $0.indice[campo] != nil }
            if candidatas.count == 1 { clase = candidatas[0] }
        }
        guard let c = clase else { throw CFallo("no sé qué struct hay en esta memoria (usa un puntero con tipo)", linea) }
        let o = try nuevoObjeto(c, [], linea)
        mem.a[i] = .o(o)
        return .o(o)
    }

    func libera(_ v: CV, _ linea: Int) throws {
        switch v {
        case .p(let p):
            guard let mem = p.mem else { return }
            if mem.libre { throw CFallo("doble free: esta memoria ya se había liberado", linea) }
            for celda in mem.a { if case .o(let o) = celda { try destruye(o, linea) } }
            mem.libre = true
        case .o(let o):
            try destruye(o, linea)
        default:
            return
        }
    }

    // MARK: tamaños (sizeof)

    func tamano(_ t: CTipo) -> Int { CMaq.tamanoEstatico(t, prog) }

    static func tamanoEstatico(_ t: CTipo, _ prog: CPrograma) -> Int {
        switch t {
        case .num(let k): return k.bytes
        case .real(let f): return f ? 4 : 8
        case .ptr, .fn: return 8
        case .arr(let e, let n): return max(n, 0) * tamanoEstatico(e, prog)
        case .cad: return 32
        case .cont: return 24
        case .ref(let x): return tamanoEstatico(x, prog)
        case .clase(let n):
            guard let c = prog.clases[n] else { return 8 }
            var off = 0
            var alin = 1
            let lista = c.tipos.isEmpty ? c.campos.filter { !$0.estatico }.map { $0.tipo } : c.tipos
            for ft in lista {
                let s = tamanoEstatico(ft, prog)
                let a = min(8, max(1, alineacion(ft, prog)))
                alin = max(alin, a)
                if c.esUnion { off = max(off, s); continue }
                off = (off + a - 1) / a * a + s
            }
            return max(1, (off + alin - 1) / alin * alin)
        default: return 8
        }
    }

    static func alineacion(_ t: CTipo, _ prog: CPrograma) -> Int {
        switch t {
        case .arr(let e, _): return alineacion(e, prog)
        case .clase(let n):
            guard let c = prog.clases[n] else { return 8 }
            return c.tipos.map { alineacion($0, prog) }.max() ?? 1
        default: return tamanoEstatico(t, prog)
        }
    }

    func tamanoValor(_ v: CV) -> Int {
        switch v {
        case .n(_, let k): return k.bytes
        case .d(_, let f): return f ? 4 : 8
        case .o(let o): return tamano(.clase(o.clase.nombre))
        case .s: return 32
        default: return 8
        }
    }

    // MARK: globales y clases

    func declaraGlobal(_ n: String, _ t: CTipo, _ linea: Int) throws -> Int {
        if let i = globalIndice[n] { return i }
        let i = globales.a.count
        globales.a.append(.vacio)
        globalTipos.append(t)
        globalIndice[n] = i
        return i
    }

    func claseExterior(_ c: CClase) -> CClase? { prog.exterior[c.nombre] }
}
