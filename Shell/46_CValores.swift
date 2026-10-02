import Foundation

// ============================================================
// MARK: - C, C++ y Java: valores, memoria y tipos
// ============================================================
// La memoria se reparte en bloques (CMem): cada variable, arreglo,
// malloc o new es un bloque, y un puntero es (bloque, posición).
// Así p+1, *p, p[i], &x y p->campo funcionan como en C, y salirse
// de un arreglo da un error claro en vez de corromper memoria.

enum CNum: UInt8 {
    case bool, char, uchar, short, ushort, jchar, int, uint, long, ulong

    var conSigno: Bool {
        switch self {
        case .uchar, .ushort, .uint, .ulong, .jchar, .bool: return false
        default: return true
        }
    }

    var rango: Int {
        switch self {
        case .bool: return 0
        case .char, .uchar: return 1
        case .short, .ushort, .jchar: return 2
        case .int, .uint: return 3
        case .long, .ulong: return 4
        }
    }

    var bytes: Int {
        switch self {
        case .bool, .char, .uchar: return 1
        case .short, .ushort, .jchar: return 2
        case .int, .uint: return 4
        case .long, .ulong: return 8
        }
    }

    /// Promoción aritmética de dos tipos enteros (como en C).
    static func comun(_ a: CNum, _ b: CNum) -> CNum {
        let x: CNum = a.rango < 3 ? .int : a
        let y: CNum = b.rango < 3 ? .int : b
        if x.rango == 4 || y.rango == 4 {
            if x == .ulong || y == .ulong { return .ulong }
            return .long
        }
        if x == .uint || y == .uint { return .uint }
        return .int
    }
}

/// Recorta un entero al ancho de su tipo (desbordamiento como en C/Java).
@inline(__always)
func cAjusta(_ v: Int64, _ k: CNum) -> Int64 {
    switch k {
    case .int: return Int64(Int32(truncatingIfNeeded: v))
    case .long, .ulong: return v
    case .uint: return Int64(UInt32(truncatingIfNeeded: v))
    case .char: return Int64(Int8(truncatingIfNeeded: v))
    case .uchar: return Int64(UInt8(truncatingIfNeeded: v))
    case .short: return Int64(Int16(truncatingIfNeeded: v))
    case .ushort, .jchar: return Int64(UInt16(truncatingIfNeeded: v))
    case .bool: return v != 0 ? 1 : 0
    }
}

indirect enum CTipo {
    case vacio
    case auto
    case num(CNum)
    case real(Bool)
    case ptr(CTipo)
    case arr(CTipo, Int)
    case clase(String)
    case cad
    case cont(String, [CTipo])
    case ref(CTipo)
    case fn
    case canal

    var sinRef: CTipo {
        if case .ref(let t) = self { return t }
        return self
    }

    var esRef: Bool {
        if case .ref = self { return true }
        return false
    }

    var nombre: String {
        switch self {
        case .vacio: return "void"
        case .auto: return "auto"
        case .num(let k): return CTipo.nombreNum(k)
        case .real(let f): return f ? "float" : "double"
        case .ptr(let t): return t.nombre + "*"
        case .arr(let t, let n): return t.nombre + (n >= 0 ? "[\(n)]" : "[]")
        case .clase(let n): return n
        case .cad: return "string"
        case .cont(let n, let a): return n + "<" + a.map { $0.nombre }.joined(separator: ", ") + ">"
        case .ref(let t): return t.nombre + "&"
        case .fn: return "función"
        case .canal: return "stream"
        }
    }

    static func nombreNum(_ k: CNum) -> String {
        switch k {
        case .bool: return "bool"
        case .char: return "char"
        case .uchar: return "unsigned char"
        case .short: return "short"
        case .ushort: return "unsigned short"
        case .jchar: return "char"
        case .int: return "int"
        case .uint: return "unsigned int"
        case .long: return "long"
        case .ulong: return "unsigned long"
        }
    }

    /// Elemento de un puntero o arreglo.
    var elemento: CTipo? {
        switch self {
        case .ptr(let t): return t
        case .arr(let t, _): return t
        case .ref(let t): return t.elemento
        default: return nil
        }
    }
}

final class CMem {
    var a: [CV]
    var libre = false
    /// bytes pedidos con malloc cuando aún no se sabe el tipo
    var crudo = 0
    var elem: CTipo?
    /// arreglo declarado (int a[10]): se copia entero dentro de un struct
    var esArreglo = false
    /// calloc: empieza en ceros
    var relleno = false

    init(_ a: [CV]) { self.a = a }
    init(n: Int, _ v: CV) { a = [CV](repeating: v, count: max(0, n)) }
}

struct CPtr {
    let mem: CMem?
    let i: Int
    static let nulo = CPtr(mem: nil, i: 0)
}

final class CObj {
    let clase: CClase
    let c: CMem
    /// marco donde se creó (clases anónimas de Java: ven sus variables)
    var exterior: CMarco?
    init(_ clase: CClase, _ c: CMem) {
        self.clase = clase
        self.c = c
    }
}

final class CLista {
    var a: [CV]
    /// vector, deque, list, stack, queue, pq, ArrayList, LinkedList…
    var k: String
    var cmp: CCierre?
    var max = false
    var elem: CTipo?
    init(_ k: String, _ a: [CV] = []) {
        self.k = k
        self.a = a
    }
}

enum CKey: Hashable, Comparable {
    case n(Int64)
    case d(Double)
    case s([UInt16])
    case t([CKey])
    case o(ObjectIdentifier)

    var orden: Int {
        switch self {
        case .n: return 0
        case .d: return 1
        case .s: return 2
        case .t: return 3
        case .o: return 4
        }
    }

    static func < (a: CKey, b: CKey) -> Bool {
        switch (a, b) {
        case (.n(let x), .n(let y)): return x < y
        case (.d(let x), .d(let y)): return x < y
        case (.n(let x), .d(let y)): return Double(x) < y
        case (.d(let x), .n(let y)): return x < Double(y)
        case (.s(let x), .s(let y)): return x.lexicographicallyPrecedes(y)
        case (.t(let x), .t(let y)): return x.lexicographicallyPrecedes(y)
        case (.o(let x), .o(let y)): return x < y
        default: return a.orden < b.orden
        }
    }
}

final class CMapa {
    /// map, set, unordered_map, HashMap, TreeMap, HashSet…
    var k: String
    var ordenado: Bool
    var esSet: Bool
    var multi = false
    var cmp: CCierre?
    var tipoValor: CTipo?
    var claves: [CKey] = []
    var vals: [CKey: (CV, CV)] = [:]
    var cuentas: [CKey: Int] = [:]

    init(_ k: String, ordenado: Bool, esSet: Bool) {
        self.k = k
        self.ordenado = ordenado
        self.esSet = esSet
    }

    var count: Int { multi ? cuentas.values.reduce(0, +) : claves.count }

    func posicion(_ c: CKey) -> Int {
        // primera clave >= c (búsqueda binaria; solo si está ordenado)
        var lo = 0
        var hi = claves.count
        while lo < hi {
            let m = (lo + hi) / 2
            if claves[m] < c { lo = m + 1 } else { hi = m }
        }
        return lo
    }

    func pon(_ c: CKey, _ clave: CV, _ v: CV) {
        if vals[c] == nil {
            if ordenado { claves.insert(c, at: posicion(c)) } else { claves.append(c) }
        }
        vals[c] = (clave, v)
        if multi { cuentas[c, default: 0] += 1 }
    }

    func quita(_ c: CKey) -> Bool {
        guard vals[c] != nil else { return false }
        if multi, let n = cuentas[c], n > 1 { cuentas[c] = n - 1; return true }
        vals[c] = nil
        cuentas[c] = nil
        if ordenado {
            let p = posicion(c)
            if p < claves.count && claves[p] == c { claves.remove(at: p) }
        } else if let p = claves.firstIndex(of: c) {
            claves.remove(at: p)
        }
        return true
    }
}

final class CIterBase {
    let lista: CLista?
    let mapa: CMapa?
    let cad: CRefCaja?
    let mem: CMem?
    let reves: Bool
    init(lista: CLista? = nil, mapa: CMapa? = nil, cad: CRefCaja? = nil, mem: CMem? = nil, reves: Bool = false) {
        self.lista = lista
        self.mapa = mapa
        self.cad = cad
        self.mem = mem
        self.reves = reves
    }
}

final class CRefCaja {
    let l: CLugar
    init(_ l: CLugar) { self.l = l }
}

indirect enum CLugar {
    case celda(CMem, Int)
    case lista(CLista, Int)
    case mapa(CMapa, CV)
    case car(CLugar, Int)
}

/// Función con lo que captura: lambda, puntero a función o método enlazado.
final class CCierre {
    let fn: CFuncion?
    let marco: CMarco?
    let este: CObj?
    let nat: ((CMaq, [CV]) throws -> CV)?
    let nombre: String
    init(fn: CFuncion?, marco: CMarco?, este: CObj?, nombre: String = "", nat: ((CMaq, [CV]) throws -> CV)? = nil) {
        self.fn = fn
        self.marco = marco
        self.este = este
        self.nat = nat
        self.nombre = nombre.isEmpty ? (fn?.nombre ?? "lambda") : nombre
    }
}

final class CMarco {
    let s: CMem
    let padre: CMarco?
    let este: CObj?
    var devuelveRef = false
    init(_ n: Int, padre: CMarco?, este: CObj?) {
        s = CMem(n: n, .vacio)
        self.padre = padre
        self.este = este
    }
}

/// Flujo de entrada/salida: cout, cin, archivos, stringstream, FILE*.
final class CCanal {
    enum Clase { case salida, error, entrada, archivoSalida, archivoEntrada, cadena }
    let clase: Clase
    var buf: [UInt8] = []
    var pos = 0
    var url: URL?
    var abierto = true
    var fallo = false
    var anexar = false
    // estado de formato de C++
    var precision = 6
    var fijo = false
    var cientifico = false
    var ancho = 0
    var relleno: UInt8 = 32
    var izquierda = false
    var boolalpha = false
    var base = 10
    var mostrarPunto = false
    var mayusculas = false
    var ent: CEntrada?
    var nombre = ""

    init(_ clase: Clase) { self.clase = clase }
}

enum CV {
    case n(Int64, CNum)
    case d(Double, Bool)
    case p(CPtr)
    case o(CObj)
    case s([UInt16])
    case l(CLista)
    case m(CMapa)
    case f(CCierre)
    case it(CIterBase, Int)
    case canal(CCanal)
    case ref(CRefCaja)
    case nulo
    case vacio

    static let cero = CV.n(0, .int)
    static let verdadero = CV.n(1, .bool)
    static let falso = CV.n(0, .bool)
    static let ptrNulo = CV.p(CPtr.nulo)

    static func ent(_ v: Int) -> CV { .n(Int64(Int32(truncatingIfNeeded: v)), .int) }
    static func largo(_ v: Int) -> CV { .n(Int64(v), .long) }
    static func bool(_ b: Bool) -> CV { b ? verdadero : falso }

    var esNulo: Bool {
        switch self {
        case .nulo: return true
        case .p(let p): return p.mem == nil
        default: return false
        }
    }

    var tipoNombre: String {
        switch self {
        case .n(_, let k): return CTipo.nombreNum(k)
        case .d(_, let f): return f ? "float" : "double"
        case .p: return "puntero"
        case .o(let o): return o.clase.nombre
        case .s: return "string"
        case .l(let l): return l.k
        case .m(let m): return m.k
        case .f: return "función"
        case .it: return "iterador"
        case .canal: return "stream"
        case .ref: return "referencia"
        case .nulo: return "null"
        case .vacio: return "void"
        }
    }
}

// MARK: clases y funciones

struct CParam {
    let nombre: String
    let tipo: CTipo
    let defecto: CExpr?
}

final class CFuncion {
    let nombre: String
    var params: [CParam]
    var ret: CTipo
    var cuerpo: CSent?
    var nSlots = 0
    weak var clase: CClase?
    var estatico = false
    var virtual = false
    var abstracto = false
    var variadica = false
    var constante = false
    /// constructor de C++: x(a), Base(b)…
    var inits: [(String, [CExpr], Int)] = []
    /// constructor de Java: this(...) o super(...) al principio
    let linea: Int
    var nat: ((CMaq, [CV]) throws -> CV)?
    var resuelta = false

    init(_ nombre: String, params: [CParam], ret: CTipo, linea: Int) {
        self.nombre = nombre
        self.params = params
        self.ret = ret
        self.linea = linea
    }

    var minArgs: Int { params.filter { $0.defecto == nil }.count }
}

struct CCampo {
    let nombre: String
    let tipo: CTipo
    let ini: CExpr?
    let estatico: Bool
    let linea: Int
}

final class CClase {
    let nombre: String
    var nombrePadre: String?
    var padre: CClase?
    var interfaces: [String] = []
    var campos: [CCampo] = []
    var indice: [String: Int] = [:]
    var tipos: [CTipo] = []
    var metodos: [String: [CFuncion]] = [:]
    var ctors: [CFuncion] = []
    var dtor: CFuncion?
    var estaticos = CMem([])
    var estIndice: [String: Int] = [:]
    var estTipos: [CTipo] = []
    var iniEstatico: [CSent] = []
    var iniInstancia: [CSent] = []
    var esInterfaz = false
    var esAbstracta = false
    var esEnum = false
    var esUnion = false
    var enumValores: [CObj] = []
    var enumArgs: [(String, [CExpr], Int)] = []
    var listo = false
    var preparada = false
    var nativa = false
    let linea: Int
    let id: Int

    init(_ nombre: String, linea: Int, id: Int) {
        self.nombre = nombre
        self.linea = linea
        self.id = id
    }

    var nCampos: Int { tipos.count }

    func esSubclase(de n: String) -> Bool {
        var c: CClase? = self
        while let x = c {
            if x.nombre == n || x.interfaces.contains(n) { return true }
            c = x.padre
        }
        return false
    }

    /// Busca un método por nombre y número de argumentos, subiendo por la herencia.
    func metodo(_ n: String, _ nargs: Int) -> CFuncion? {
        var c: CClase? = self
        while let x = c {
            if let lista = x.metodos[n] {
                if let f = lista.first(where: { nargs >= $0.minArgs && (nargs <= $0.params.count || $0.variadica) && !$0.abstracto }) { return f }
            }
            c = x.padre
        }
        return nil
    }

    func tieneMetodo(_ n: String) -> Bool {
        var c: CClase? = self
        while let x = c {
            if x.metodos[n] != nil { return true }
            c = x.padre
        }
        return false
    }

    func campoEstatico(_ n: String) -> (CClase, Int)? {
        var c: CClase? = self
        while let x = c {
            if let i = x.estIndice[n] { return (x, i) }
            c = x.padre
        }
        return nil
    }
}

// MARK: conversiones básicas

extension CV {
    @inline(__always)
    var comoEntero: Int64? {
        switch self {
        case .n(let v, _): return v
        case .d(let d, _): return CV.truncaD(d)
        case .ref: return nil
        default: return nil
        }
    }

    static func truncaD(_ d: Double) -> Int64 {
        if d.isNaN { return 0 }
        if d >= 9.2e18 { return Int64.max }
        if d <= -9.2e18 { return Int64.min }
        return Int64(d)
    }

    var comoReal: Double? {
        switch self {
        case .n(let v, let k): return k == .ulong ? Double(UInt64(bitPattern: v)) : Double(v)
        case .d(let d, _): return d
        default: return nil
        }
    }
}

extension Array where Element == UInt16 {
    var texto: String { String(decoding: self, as: UTF16.self) }
}

// Los valores se usan siempre desde el hilo del programa que corre;
// las constantes estáticas (CV.cero…) no tienen estado que cambie.
extension CV: @unchecked Sendable {}
extension CPtr: @unchecked Sendable {}
