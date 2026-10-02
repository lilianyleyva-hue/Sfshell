import Foundation

// ============================================================
// MARK: - C, C++ y Java: análisis sintáctico
// ============================================================

final class CPrograma {
    var globales: [CSent] = []
    var funciones: [String: [CFuncion]] = [:]
    var clases: [String: CClase] = [:]
    var ordenClases: [CClase] = []
    var enumConst: [String: CV] = [:]
    var enumAmbito: [String: [String: CV]] = [:]
    var cabeceras: Set<String> = []
    var siguienteId = 1
    /// clase anidada → la clase que la contiene
    var exterior: [String: CClase] = [:]

    func clase(_ n: String, _ linea: Int) -> CClase {
        if let c = clases[n] { return c }
        let c = CClase(n, linea: linea, id: siguienteId)
        siguienteId += 1
        clases[n] = c
        ordenClases.append(c)
        return c
    }

    func agregaFuncion(_ f: CFuncion) {
        var lista = funciones[f.nombre] ?? []
        // la definición reemplaza al prototipo con los mismos parámetros
        if let i = lista.firstIndex(where: { $0.params.count == f.params.count && $0.cuerpo == nil }) {
            lista[i] = f
        } else if f.cuerpo == nil, lista.contains(where: { $0.params.count == f.params.count }) {
            return
        } else {
            lista.append(f)
        }
        funciones[f.nombre] = lista
    }
}

final class CParser {
    var t: [CTok]
    var i = 0
    let d: CDialecto
    let prog: CPrograma
    var tipos: Set<String> = []
    var typedefs: [String: CTipo] = [:]
    var plantilla: Set<String> = []
    var constantes: [String: Int64] = [:]
    var claseActual: CClase?
    var anonimas = 0

    static let primC: Set<String> = ["void", "char", "short", "int", "long", "float", "double", "signed", "unsigned",
                                      "_Bool", "bool", "size_t", "ssize_t", "FILE", "int8_t", "int16_t", "int32_t", "int64_t",
                                      "uint8_t", "uint16_t", "uint32_t", "uint64_t", "ptrdiff_t", "time_t", "clock_t", "wchar_t",
                                      "intptr_t", "uintptr_t"]
    static let califC: Set<String> = ["const", "volatile", "static", "extern", "register", "inline", "constexpr",
                                       "mutable", "typename", "struct", "union", "enum", "class", "auto", "thread_local",
                                       "__inline", "restrict", "__restrict", "consteval", "constinit"]
    static let tiposCpp: Set<String> = ["string", "vector", "map", "set", "unordered_map", "unordered_set", "multiset",
                                         "multimap", "pair", "stack", "queue", "deque", "priority_queue", "list", "array",
                                         "stringstream", "istringstream", "ostringstream", "ifstream", "ofstream", "fstream",
                                         "function", "ostream", "istream", "tuple", "less", "greater", "numeric_limits",
                                         "initializer_list", "wstring", "string_view", "optional", "bitset", "iterator", "nullptr_t"]
    static let primJava: Set<String> = ["byte", "short", "int", "long", "float", "double", "char", "boolean", "void", "var"]
    static let tiposJava: Set<String> = ["String", "Object", "Integer", "Long", "Double", "Float", "Character", "Boolean",
                                          "Byte", "Short", "StringBuilder", "StringBuffer", "Scanner", "ArrayList", "List",
                                          "LinkedList", "Map", "HashMap", "TreeMap", "LinkedHashMap", "Set", "HashSet",
                                          "TreeSet", "LinkedHashSet", "Queue", "Deque", "ArrayDeque", "PriorityQueue", "Stack",
                                          "Iterator", "Random", "BufferedReader", "InputStreamReader", "Comparator",
                                          "Runnable", "Function", "BiFunction", "Supplier", "Consumer", "BiConsumer", "Predicate",
                                          "UnaryOperator", "BinaryOperator", "Optional", "Collection", "Iterable", "Number",
                                          "Comparable", "CharSequence", "Entry", "Vector", "Math", "Collections", "Arrays",
                                          "System", "Thread", "File", "FileWriter", "PrintWriter", "FileReader", "IntStream",
                                          "Stream", "Collectors", "Objects", "BigInteger", "LocalDate", "IntFunction",
                                          "ToIntFunction", "IntBinaryOperator", "IntUnaryOperator", "IntPredicate", "Character"]
    static let modJava: Set<String> = ["public", "private", "protected", "static", "final", "abstract", "synchronized",
                                        "native", "transient", "volatile", "strictfp", "default", "sealed", "non-sealed"]

    init(_ t: [CTok], dialecto: CDialecto, prog: CPrograma) {
        self.t = t
        self.d = dialecto
        self.prog = prog
        for n in prog.clases.keys { tipos.insert(n) }
        preescanea()
    }

    // MARK: utilidades

    @inline(__always) func mira(_ k: Int = 0) -> CTok {
        let j = i + k
        return j < t.count ? t[j] : t[t.count - 1]
    }
    @inline(__always) func es(_ s: String) -> Bool { mira().es(s) }
    @inline(__always) func esId(_ s: String) -> Bool { mira().esId(s) }
    var linea: Int { mira().linea }

    @discardableResult
    func avanza() -> CTok {
        let x = mira()
        if i < t.count - 1 { i += 1 }
        return x
    }

    func acepta(_ s: String) -> Bool {
        if es(s) { avanza(); return true }
        return false
    }
    func aceptaId(_ s: String) -> Bool {
        if esId(s) { avanza(); return true }
        return false
    }

    func espera(_ s: String) throws {
        // '>>' cuando se espera '>' (vector<vector<int>>)
        if s == ">" && (es(">>") || es(">>>")) {
            let resto = String(mira().t.dropFirst())
            t[i].t = resto
            return
        }
        if s == ">" && (es(">=") || es(">>=")) {
            t[i].t = String(mira().t.dropFirst())
            return
        }
        guard es(s) else { throw CFallo(errorEsperado(s), linea) }
        avanza()
    }

    func errorEsperado(_ s: String) -> String {
        let x = mira()
        let visto = x.k == .fin ? "el final del archivo" : "'\(x.k == .cad ? "\"" + x.t + "\"" : x.t)'"
        if s == ";" { return "falta ';' antes de \(visto)" }
        return "se esperaba '\(s)' y aparece \(visto)"
    }

    func ident() throws -> String {
        let x = mira()
        guard x.k == .id else {
            throw CFallo("se esperaba un nombre y aparece '\(x.t)'", x.linea)
        }
        avanza()
        return x.t
    }

    /// Salta un bloque entre paréntesis/llaves/corchetes equilibrados.
    func saltaEquilibrado() {
        var prof = 0
        repeat {
            let x = avanza()
            if x.es("(") || x.es("{") || x.es("[") { prof += 1 }
            if x.es(")") || x.es("}") || x.es("]") { prof -= 1 }
            if x.k == .fin { return }
        } while prof > 0
    }

    // MARK: preescaneo de nombres de tipo

    func preescanea() {
        if d == .java { tipos.formUnion(CParser.tiposJava) } else { tipos.formUnion(CParser.tiposCpp) }
        var k = 0
        while k < t.count - 1 {
            let x = t[k]
            if x.k == .id && ["struct", "class", "union", "enum", "interface", "record"].contains(x.t) {
                var j = k + 1
                if t[j].esId("class") || t[j].esId("struct") { j += 1 }
                while t[j].es("[") || t[j].es("@") { j += 1 }
                if t[j].k == .id { tipos.insert(t[j].t) }
            }
            if x.esId("typedef") {
                var j = k + 1
                var prof = 0
                var ultimo = ""
                while j < t.count - 1 {
                    let y = t[j]
                    if y.es("{") || y.es("(") { prof += 1 }
                    if y.es("}") || y.es(")") { prof -= 1 }
                    if y.es(";") && prof <= 0 { break }
                    if y.k == .id && (prof == 0 || (j > 0 && t[j - 1].es("*"))) { ultimo = y.t }
                    j += 1
                }
                if !ultimo.isEmpty { tipos.insert(ultimo) }
            }
            if x.esId("using") && t[k + 1].k == .id && t[k + 2].es("=") { tipos.insert(t[k + 1].t) }
            if x.esId("template") || (d == .java && x.es("<") && k > 0 && (t[k - 1].k == .id && (tipos.contains(t[k - 1].t) || CParser.modJava.contains(t[k - 1].t)))) {
                var j = k + 1
                if t[j].es("<") { j += 1 }
                while j < t.count - 1 && !t[j].es(">") && !t[j].es(">>") {
                    if (t[j].esId("typename") || t[j].esId("class")) && t[j + 1].k == .id { tipos.insert(t[j + 1].t) }
                    if d == .java && t[j].k == .id && t[j].t.count <= 2 && t[j].t.first?.isUppercase == true { tipos.insert(t[j].t) }
                    j += 1
                }
            }
            k += 1
        }
    }

    // MARK: programa

    func programa() throws {
        while mira().k != .fin {
            if d == .java { try declaracionJava() } else { try declaracionTope() }
        }
    }

    // MARK: tipos

    func esTipoInicio(_ k: Int = 0) -> Bool {
        let x = mira(k)
        guard x.k == .id else { return false }
        if d == .java {
            if CParser.primJava.contains(x.t) || x.t == "final" { return true }
            return tipos.contains(x.t) || plantilla.contains(x.t)
        }
        if CParser.primC.contains(x.t) || CParser.califC.contains(x.t) { return x.t != "class" || claseActual == nil }
        if x.t == "std" && mira(k + 1).es("::") { return esTipoInicio(k + 2) }
        return tipos.contains(x.t) || typedefs[x.t] != nil || plantilla.contains(x.t)
    }

    /// ¿Empieza aquí una declaración de variable (y no una expresión)?
    func empiezaDecl() -> Bool {
        guard esTipoInicio() else { return false }
        let x = mira()
        if d == .java {
            if CParser.primJava.contains(x.t) || x.t == "final" { return true }
            // Clase nombre / Clase<…> nombre / Clase[] nombre
            let y = mira(1)
            if y.k == .id { return true }
            if y.es("<") { return pareceGenericoYNombre(1) }
            if y.es("[") && mira(2).es("]") { return true }
            if y.es(".") && mira(2).k == .id && tipos.contains(mira(2).t) {
                if mira(3).k == .id { return true }
                if mira(3).es("<") { return pareceGenericoYNombre(3) }
            }
            return false
        }
        if CParser.primC.contains(x.t) || CParser.califC.contains(x.t) { return true }
        var k = 0
        if x.t == "std" { k = 2 }
        let y = mira(k + 1)
        if y.es("(") || y.es(".") || y.es("->") || y.es("=") || y.es("[") || y.es(";") || y.es(")") { return false }
        if y.es("::") { return mira(k + 2).k == .id && esTipoAnidado(k) }
        if y.es("<") { return pareceGenericoYNombre(k + 1) }
        return y.k == .id || y.es("*") || y.es("&") || y.es("&&")
    }

    func esTipoAnidado(_ k: Int) -> Bool {
        // vector<int>::iterator it / Clase::Tipo x
        let n = mira(k + 2)
        return n.esId("iterator") || n.esId("const_iterator") || n.esId("size_type") || tipos.contains(n.t)
    }

    /// Clase<...> seguido de un nombre (o *, &, ::iterator)
    func pareceGenericoYNombre(_ desde: Int) -> Bool {
        var k = desde
        var prof = 0
        while true {
            let x = mira(k)
            if x.k == .fin || x.es(";") || x.es("{") || x.es("(") && prof == 0 { return false }
            if x.es("<") { prof += 1 }
            if x.es(">") { prof -= 1 }
            if x.es(">>") { prof -= 2 }
            if x.es(">>>") { prof -= 3 }
            k += 1
            if prof <= 0 { break }
        }
        let y = mira(k)
        if y.es("::") { return true }
        if y.es("[") && mira(k + 1).es("]") { return true }
        return y.k == .id || y.es("*") || y.es("&") || y.es("&&") || y.es("...")
    }

    func tipoBase() throws -> CTipo {
        if d == .java { return try tipoJava() }
        var sinSigno = false
        var conSigno = false
        var largos = 0
        var corto = false
        var base: String?
        var resultado: CTipo?
        while mira().k == .id {
            let x = mira().t
            if ["const", "volatile", "static", "extern", "register", "inline", "constexpr", "mutable", "typename",
                "thread_local", "__inline", "restrict", "__restrict", "virtual", "explicit", "friend", "consteval", "constinit"].contains(x) {
                avanza(); continue
            }
            if x == "unsigned" { sinSigno = true; avanza(); continue }
            if x == "signed" { conSigno = true; avanza(); continue }
            if x == "long" { largos += 1; avanza(); continue }
            if x == "short" { corto = true; avanza(); continue }
            if x == "std" && mira(1).es("::") { avanza(); avanza(); continue }
            if base != nil || resultado != nil { break }
            // long long ll: después de long/unsigned/short solo siguen tipos básicos
            if (sinSigno || conSigno || largos > 0 || corto) && !CParser.primC.contains(x) { break }
            if x == "struct" || x == "class" || x == "union" || x == "enum" {
                avanza()
                if esId("class") || esId("struct") { avanza() }
                if mira().k == .id && !mira(1).es("{") && !mira(1).es(":") {
                    let n = avanza().t
                    if typedefs[n] != nil { resultado = typedefs[n] } else { resultado = x == "enum" ? .num(.int) : .clase(n) }
                    if x == "enum" { resultado = .num(.int) }
                    continue
                }
                // struct { … } o struct X { … } en una declaración: la define aquí
                if x == "enum" { resultado = try defineEnum(); continue }
                let c = try defineClase(x == "class", esUnion: x == "union")
                resultado = .clase(c.nombre)
                continue
            }
            if CParser.primC.contains(x) || x == "auto" {
                base = x; avanza(); continue
            }
            if let td = typedefs[x] { avanza(); resultado = td; continue }
            if plantilla.contains(x) { avanza(); resultado = .auto; continue }
            if tipos.contains(x) {
                avanza()
                resultado = try tipoNombrado(x)
                continue
            }
            break
        }
        // ::iterator, ::size_type…
        while es("::") && mira(1).k == .id {
            avanza()
            let n = avanza().t
            if n == "iterator" || n == "const_iterator" || n == "reverse_iterator" { resultado = .auto }
            else if n == "size_type" { resultado = .num(.ulong) }
            else if let c = prog.clases[n] { resultado = .clase(c.nombre) }
            else { resultado = .auto }
        }
        while esId("const") || esId("volatile") { avanza() }
        if let r = resultado { return r }
        return try tipoPrimitivo(base, sinSigno: sinSigno, conSigno: conSigno, largos: largos, corto: corto)
    }

    func tipoPrimitivo(_ base: String?, sinSigno: Bool, conSigno: Bool, largos: Int, corto: Bool) throws -> CTipo {
        if base == nil && !sinSigno && !conSigno && largos == 0 && !corto {
            throw CFallo("se esperaba un tipo y aparece '\(mira().t)'", linea)
        }
        switch base ?? "int" {
        case "void": return .vacio
        case "auto": return .auto
        case "bool", "_Bool": return .num(.bool)
        case "char", "wchar_t": return .num(sinSigno ? .uchar : .char)
        case "float": return .real(true)
        case "double": return .real(false)
        case "size_t", "uintptr_t": return .num(.ulong)
        case "ssize_t", "ptrdiff_t", "time_t", "clock_t", "intptr_t": return .num(.long)
        case "int8_t": return .num(.char)
        case "uint8_t": return .num(.uchar)
        case "int16_t": return .num(.short)
        case "uint16_t": return .num(.ushort)
        case "int32_t": return .num(.int)
        case "uint32_t": return .num(.uint)
        case "int64_t": return .num(.long)
        case "uint64_t": return .num(.ulong)
        case "FILE": return .canal
        default:
            if corto { return .num(sinSigno ? .ushort : .short) }
            if largos > 0 { return .num(sinSigno ? .ulong : .long) }
            return .num(sinSigno ? .uint : .int)
        }
    }

    /// Tipo con nombre (clase o contenedor), con sus <argumentos>.
    func tipoNombrado(_ n: String) throws -> CTipo {
        var args: [CTipo] = []
        if es("<") {
            args = try argumentosPlantilla()
        }
        switch n {
        case "string", "wstring", "string_view", "String": return .cad
        case "ostream", "istream":
            return .canal
        case "stringstream", "istringstream", "ostringstream", "ifstream", "ofstream", "fstream":
            return .cont(n, [])
        case "function": return .fn
        case "Integer", "Long", "Double", "Float", "Character", "Boolean", "Byte", "Short", "Object", "Number", "Comparable", "CharSequence":
            return .auto
        default:
            if prog.clases[n] != nil && !CParser.tiposCpp.contains(n) { return .clase(n) }
            if d == .cpp && CParser.tiposCpp.contains(n) { return .cont(n, args) }
            if d == .java && CParser.tiposJava.contains(n) && prog.clases[n] == nil { return .cont(n, args) }
            return .clase(n)
        }
    }

    func argumentosPlantilla() throws -> [CTipo] {
        try espera("<")
        var args: [CTipo] = []
        while !es(">") && !es(">>") && !es(">>>") && !es(">=") && !es(">>=") {
            if mira().k == .ent { args.append(.num(.int)); avanza() }
            else if es("?") {
                avanza()
                if aceptaId("extends") || aceptaId("super") { args.append(try tipoCompleto()) } else { args.append(.auto) }
            } else {
                args.append(try tipoCompleto())
            }
            // function<int(int,int)>
            if es("(") { saltaEquilibrado() }
            if !acepta(",") { break }
        }
        try espera(">")
        return args
    }

    /// Tipo con sus * y & (para plantillas, casts y sizeof).
    func tipoCompleto() throws -> CTipo {
        var tt = try tipoBase()
        while true {
            if acepta("*") { tt = .ptr(tt); continue }
            if acepta("&") || acepta("&&") { tt = .ref(tt); continue }
            if esId("const") { avanza(); continue }
            if d == .java && es("[") && mira(1).es("]") { avanza(); avanza(); tt = .arr(tt, -1); continue }
            break
        }
        return tt
    }

    func tipoJava() throws -> CTipo {
        while esId("final") || es("@") {
            if es("@") { avanza(); avanza(); if es("(") { saltaEquilibrado() } } else { avanza() }
        }
        let x = try ident()
        var tt: CTipo
        switch x {
        case "byte": tt = .num(.char)
        case "short": tt = .num(.short)
        case "int": tt = .num(.int)
        case "long": tt = .num(.long)
        case "float": tt = .real(true)
        case "double": tt = .real(false)
        case "char": tt = .num(.jchar)
        case "boolean": tt = .num(.bool)
        case "void": tt = .vacio
        case "var": tt = .auto
        default:
            var nombre = x
            // Map.Entry<…>, java.util.List…
            while es(".") && mira(1).k == .id {
                avanza()
                nombre = avanza().t
            }
            if plantilla.contains(nombre) {
                tt = .auto
                if es("<") { _ = try argumentosPlantilla() }
            } else {
                tt = try tipoNombrado(nombre)
            }
        }
        while es("[") && mira(1).es("]") {
            avanza(); avanza()
            tt = .arr(tt, -1)
        }
        return tt
    }

    // MARK: declaraciones de C y C++

    func declaracionTope() throws {
        if acepta(";") { return }
        if esId("using") { try usando(); return }
        if esId("namespace") {
            avanza()
            if mira().k == .id { avanza() }
            if acepta("=") { while !acepta(";") { avanza() }; return }
            try espera("{")
            while !es("}") && mira().k != .fin { try declaracionTope() }
            try espera("}")
            return
        }
        if esId("extern") && mira(1).k == .cad {
            avanza(); avanza()
            if acepta("{") {
                while !es("}") && mira().k != .fin { try declaracionTope() }
                try espera("}")
            }
            return
        }
        if esId("template") { try cabeceraPlantilla(); return try declaracionTope() }
        if esId("typedef") { try typedefDecl(); return }
        if esId("static_assert") { while !acepta(";") { avanza() }; return }
        // Clase::Clase(…) / Clase::~Clase()
        if mira().k == .id, prog.clases[mira().t] != nil || tipos.contains(mira().t), mira(1).es("::"),
           mira(2).t == mira().t || mira(2).es("~") {
            try metodoFuera(nil)
            return
        }
        // struct X { … }; class X : Base { … };  enum …
        if (esId("struct") || esId("class") || esId("union")) && (mira(2).es("{") || mira(2).es(":") || mira(2).es(";") || mira(2).esId("final")) && mira(1).k == .id {
            let kw = avanza().t
            if mira(1).es(";") { _ = prog.clase(avanza().t, linea); avanza(); return }
            let c = try defineClase(kw == "class", esUnion: kw == "union")
            if acepta(";") { return }
            try variablesTras(.clase(c.nombre), global: true)
            return
        }
        if (esId("struct") || esId("class")) && mira(1).es("{") {
            avanza()
            let c = try defineClase(false, esUnion: false)
            if acepta(";") { return }
            try variablesTras(.clase(c.nombre), global: true)
            return
        }
        if esId("enum") && (mira(1).es("{") || mira(2).es("{") || mira(3).es("{") || mira(2).es(":")) {
            avanza()
            _ = try defineEnum()
            if acepta(";") { return }
            try variablesTras(.num(.int), global: true)
            return
        }
        let ln = linea
        let base = try tipoBase()
        if acepta(";") { return }
        try declaradoresTope(base, ln)
    }

    func usando() throws {
        avanza()
        if aceptaId("namespace") {
            while !acepta(";") { avanza() }
            return
        }
        if mira().k == .id && mira(1).es("=") {
            let n = avanza().t
            avanza()
            let tt = try tipoCompleto()
            typedefs[n] = tt
            tipos.insert(n)
            try espera(";")
            return
        }
        while !acepta(";") { avanza() }
    }

    func cabeceraPlantilla() throws {
        avanza()
        try espera("<")
        var prof = 1
        while prof > 0 && mira().k != .fin {
            let x = avanza()
            if x.es("<") { prof += 1 }
            if x.es(">") { prof -= 1 }
            if x.es(">>") { prof -= 2 }
            if (x.esId("typename") || x.esId("class")) && mira().k == .id {
                plantilla.insert(mira().t)
                tipos.insert(mira().t)
            }
        }
    }

    func typedefDecl() throws {
        avanza()
        let ln = linea
        let base = try tipoBase()
        repeat {
            var tt = base
            while acepta("*") { tt = .ptr(tt) }
            while acepta("&") { tt = .ref(tt) }
            // typedef int (*Op)(int, int);
            if acepta("(") {
                while acepta("*") || acepta("&") {}
                let n = try ident()
                try espera(")")
                if es("(") { saltaEquilibrado() }
                typedefs[n] = .fn
                tipos.insert(n)
                continue
            }
            let n = try ident()
            if es("(") { saltaEquilibrado(); typedefs[n] = .fn; tipos.insert(n); continue }
            var dims: [Int] = []
            while acepta("[") {
                let e = try expresion()
                dims.append(Int(constanteDe(e) ?? -1))
                try espera("]")
            }
            for k in dims.reversed() { tt = .arr(tt, k) }
            typedefs[n] = tt
            tipos.insert(n)
            // typedef struct {…} Punto: la estructura anónima toma el nombre
            if case .clase(let cn) = base, cn.hasPrefix("$anon"), let c = prog.clases[cn] {
                prog.clases[n] = c
                typedefs[n] = .clase(cn)
            }
        } while acepta(",")
        _ = ln
        try espera(";")
    }

    /// Después del tipo: funciones o variables globales.
    func declaradoresTope(_ base: CTipo, _ ln: Int) throws {
        var tt = base
        while acepta("*") || es("&") || es("&&") {
            if acepta("&") || acepta("&&") { tt = .ref(tt); continue }
            tt = .ptr(tt)
            while esId("const") { avanza() }
        }
        // int (*f)(int) = …;
        if es("(") && mira(1).es("*") {
            try variablesTras(base, global: true)
            return
        }
        // tipo Clase::metodo(…)
        if mira().k == .id && mira(1).es("::") && !mira().esId("std") {
            try metodoFuera(tt)
            return
        }
        let nombre = try nombreFuncion()
        if es("(") {
            let fn = try funcion(nombre, tt, ln, clase: nil)
            prog.agregaFuncion(fn)
            return
        }
        try restoVariables(base, primerTipo: tt, nombre: nombre, global: true, ln)
    }

    func nombreFuncion() throws -> String {
        if esId("operator") {
            avanza()
            var op = ""
            if acepta("(") { try espera(")"); op = "()" }
            else if acepta("[") { try espera("]"); op = "[]" }
            else if mira().k == .id { op = " " + avanza().t }
            else {
                op = avanza().t
                if (op == "<" || op == ">") && es(op) && mira().linea == mira(-1).linea { op += avanza().t }
            }
            return "operator" + op
        }
        return try ident()
    }

    /// Clase::metodo(…) { … } escrito fuera de la clase.
    func metodoFuera(_ ret: CTipo?) throws {
        let ln = linea
        let cn = try ident()
        try espera("::")
        let c = prog.clase(cn, ln)
        let anterior = claseActual
        claseActual = c
        defer { claseActual = anterior }
        var esDtor = false
        if acepta("~") { esDtor = true }
        let n = try nombreFuncion()
        // int Clase::estatico = 0;
        if !es("(") {
            var ini: CExpr?
            if acepta("=") { ini = try asignacion() }
            else if es("{") { ini = try listaIni() }
            try espera(";")
            if let i = c.campos.firstIndex(where: { $0.nombre == n && $0.estatico }) {
                let viejo = c.campos[i]
                c.campos[i] = CCampo(nombre: n, tipo: viejo.tipo, ini: ini, estatico: true, linea: ln)
            } else {
                c.campos.append(CCampo(nombre: n, tipo: ret ?? .auto, ini: ini, estatico: true, linea: ln))
            }
            return
        }
        let esCtor = ret == nil && !esDtor && n == cn
        let fn = try funcion(esDtor ? "~" + n : n, ret ?? .vacio, ln, clase: c, esCtor: esCtor)
        if esDtor { c.dtor = fn; return }
        if esCtor {
            if let k = c.ctors.firstIndex(where: { $0.cuerpo == nil && $0.params.count == fn.params.count }) {
                fn.virtual = c.ctors[k].virtual
                c.ctors[k] = fn
            } else {
                c.ctors.append(fn)
            }
            return
        }
        var lista = c.metodos[n] ?? []
        if let k = lista.firstIndex(where: { $0.cuerpo == nil && $0.params.count == fn.params.count }) {
            fn.virtual = lista[k].virtual
            fn.estatico = lista[k].estatico
            lista[k] = fn
        } else {
            lista.append(fn)
        }
        c.metodos[n] = lista
    }

    func parametros() throws -> ([CParam], Bool) {
        try espera("(")
        var ps: [CParam] = []
        var variadica = false
        if esId("void") && mira(1).es(")") { avanza() }
        while !es(")") {
            if acepta("...") { variadica = true; break }
            let ln = linea
            var tt = try tipoBase()
            while true {
                if acepta("*") { tt = .ptr(tt); continue }
                if acepta("&") || acepta("&&") { tt = .ref(tt); continue }
                if esId("const") { avanza(); continue }
                break
            }
            if d == .java && acepta("...") { tt = .arr(tt, -1); variadica = true }
            var nombre = ""
            if es("(") && mira(1).es("*") {
                // int (*f)(int)
                avanza(); avanza()
                nombre = mira().k == .id ? avanza().t : ""
                try espera(")")
                saltaEquilibrado()
                tt = .fn
            } else if mira().k == .id {
                nombre = avanza().t
            }
            var primero = true
            while acepta("[") {
                if !es("]") { _ = try expresion() }
                try espera("]")
                tt = primero && d != .java ? .ptr(tt) : .arr(tt, -1)
                primero = false
            }
            if d == .java { while es("[") && mira(1).es("]") { avanza(); avanza(); tt = .arr(tt, -1) } }
            var def: CExpr?
            if acepta("=") { def = try asignacion() }
            ps.append(CParam(nombre: nombre.isEmpty ? "$p\(ps.count)" : nombre, tipo: tt, defecto: def))
            _ = ln
            if !acepta(",") { break }
        }
        try espera(")")
        return (ps, variadica)
    }

    func funcion(_ nombre: String, _ ret: CTipo, _ ln: Int, clase: CClase?, esCtor: Bool = false) throws -> CFuncion {
        let (ps, variadica) = try parametros()
        let fn = CFuncion(nombre, params: ps, ret: ret, linea: ln)
        fn.variadica = variadica
        fn.clase = clase
        // calificadores tras los parámetros
        while true {
            if esId("const") { avanza(); fn.constante = true; continue }
            if esId("override") || esId("final") || esId("noexcept") || esId("volatile") { avanza(); if es("(") { saltaEquilibrado() }; continue }
            if acepta("&") || acepta("&&") { continue }
            if es("->") {
                // auto f() -> int
                avanza()
                fn.ret = try tipoCompleto()
                continue
            }
            if esId("throws") {
                avanza()
                while mira().k == .id || es(".") || es(",") { avanza() }
                continue
            }
            break
        }
        if acepta("=") {
            if acepta("0") || (mira().k == .ent && mira().v == 0) { if mira().k == .ent { avanza() }; fn.abstracto = true; fn.virtual = true }
            else { avanza() }
            try espera(";")
            return fn
        }
        if acepta(";") { return fn }
        if esCtor && acepta(":") {
            repeat {
                let iln = linea
                var n = try ident()
                while acepta("::") { n = try ident() }
                if es("<") { _ = try argumentosPlantilla() }
                var args: [CExpr] = []
                if es("{") {
                    let l = try listaIni()
                    args = l.elems
                    if args.count == 1 || true { args = [l] }
                } else {
                    args = try argumentos()
                }
                fn.inits.append((n, args, iln))
            } while acepta(",")
        }
        fn.cuerpo = try bloque()
        return fn
    }

    func argumentos() throws -> [CExpr] {
        try espera("(")
        var a: [CExpr] = []
        while !es(")") {
            a.append(try asignacion())
            if !acepta(",") { break }
        }
        try espera(")")
        return a
    }

    /// Variables después de struct {…} o enum {…}
    func variablesTras(_ base: CTipo, global: Bool) throws {
        let ln = linea
        var tt = base
        while acepta("*") { tt = .ptr(tt) }
        if es("(") && mira(1).es("*") {
            avanza(); avanza()
            let n = try ident()
            try espera(")")
            saltaEquilibrado()
            try restoVariables(base, primerTipo: .fn, nombre: n, global: global, ln)
            return
        }
        let n = try ident()
        try restoVariables(base, primerTipo: tt, nombre: n, global: global, ln)
    }

    /// int a = 1, *b, c[10]; (ya leído el primer nombre)
    func restoVariables(_ base: CTipo, primerTipo: CTipo, nombre: String, global: Bool, _ ln: Int) throws {
        var vars: [CDeclVar] = []
        vars.append(try unDeclarador(nombre, primerTipo, ln))
        while acepta(",") {
            var tt = base
            while true {
                if acepta("*") { tt = .ptr(tt); continue }
                if acepta("&") { tt = .ref(tt); continue }
                if esId("const") { avanza(); continue }
                break
            }
            if es("(") && mira(1).es("*") {
                avanza(); avanza()
                let n = try ident()
                try espera(")")
                saltaEquilibrado()
                vars.append(try unDeclarador(n, .fn, linea))
                continue
            }
            let n = try ident()
            vars.append(try unDeclarador(n, tt, linea))
        }
        try espera(";")
        for v in vars { v.global = global }
        let decl = CDecl(vars, ln)
        if global {
            prog.globales.append(decl)
            for v in vars {
                if let ini = v.ini, let k = constanteDe(ini), case .num = v.tipo { constantes[v.nombre] = k }
            }
        } else {
            pendientes.append(decl)
        }
    }

    var pendientes: [CSent] = []

    func unDeclarador(_ nombre: String, _ tipo: CTipo, _ ln: Int) throws -> CDeclVar {
        var dims: [CExpr?] = []
        var tt = tipo
        while acepta("[") {
            if acepta("]") { dims.append(nil); continue }
            dims.append(try expresion())
            try espera("]")
        }
        if d == .java { while es("[") && mira(1).es("]") { avanza(); avanza(); tt = .arr(tt, -1) } }
        // tipo estático de un arreglo: int a[3][4] → arr(arr(int,4),3)
        for e in dims.reversed() {
            let n = e.flatMap { constanteDe($0) }.map { Int($0) } ?? -1
            tt = .arr(tt, n)
        }
        let v = CDeclVar(nombre, tt, ini: nil, dims: dims, ln)
        if acepta("=") {
            v.ini = es("{") ? try listaIni() : try asignacion()
        } else if es("(") && d == .cpp {
            v.ctorArgs = try argumentos()
        } else if es("{") && d == .cpp {
            let l = try listaIni()
            v.ini = l
        }
        // int a[] = {1,2,3}: el tamaño sale de la lista
        if let l = v.ini as? CListaIni, case .arr(let e, -1) = tt, dims.count == 1 {
            v.tipo = .arr(e, l.elems.count)
        }
        if let l = v.ini as? CLitCad, case .arr(let e, -1) = tt, dims.count == 1 {
            v.tipo = .arr(e, l.s.utf8.count + 1)
        }
        return v
    }

    // MARK: clases de C++ (y structs de C)

    func defineClase(_ esClase: Bool, esUnion: Bool) throws -> CClase {
        let ln = linea
        var nombre: String
        if mira().k == .id && !es("{") {
            nombre = avanza().t
        } else {
            anonimas += 1
            nombre = "$anon\(anonimas)"
        }
        tipos.insert(nombre)
        let c = prog.clase(nombre, ln)
        if let ex = claseActual { prog.exterior[nombre] = ex }
        c.esUnion = esUnion
        let plantillaPrevia = plantilla
        if aceptaId("final") {}
        if acepta(":") {
            repeat {
                while esId("public") || esId("private") || esId("protected") || esId("virtual") { avanza() }
                var b = try ident()
                while acepta("::") { b = try ident() }
                if es("<") { _ = try argumentosPlantilla() }
                if c.nombrePadre == nil { c.nombrePadre = b } else { c.interfaces.append(b) }
            } while acepta(",")
        }
        try espera("{")
        let anterior = claseActual
        claseActual = c
        defer { claseActual = anterior }
        while !es("}") && mira().k != .fin {
            try miembroCpp(c, esClase: esClase)
        }
        try espera("}")
        plantilla = plantillaPrevia
        return c
    }

    func miembroCpp(_ c: CClase, esClase: Bool) throws {
        if acepta(";") { return }
        if (esId("public") || esId("private") || esId("protected")) && mira(1).es(":") { avanza(); avanza(); return }
        if esId("using") { try usando(); return }
        if esId("typedef") { try typedefDecl(); return }
        if esId("static_assert") { while !acepta(";") { avanza() }; return }
        if esId("template") { try cabeceraPlantilla(); return try miembroCpp(c, esClase: esClase) }
        if esId("friend") {
            avanza()
            if esId("class") || esId("struct") { while !acepta(";") { avanza() }; return }
            let ln = linea
            let ret = try tipoCompleto()
            let n = try nombreFuncion()
            let anterior = claseActual
            claseActual = nil
            let fn = try funcion(n, ret, ln, clase: nil)
            claseActual = anterior
            prog.agregaFuncion(fn)
            return
        }
        if (esId("struct") || esId("class") || esId("union")) && (mira(2).es("{") || mira(2).es(":") || mira(1).es("{")) {
            let kw = avanza().t
            let anid = try defineClase(kw == "class", esUnion: kw == "union")
            if acepta(";") { return }
            try camposDe(c, .clase(anid.nombre), estatico: false, linea)
            return
        }
        if esId("enum") && (mira(1).es("{") || mira(2).es("{") || mira(3).es("{")) {
            avanza()
            _ = try defineEnum()
            if acepta(";") { return }
            try camposDe(c, .num(.int), estatico: false, linea)
            return
        }
        var virtual = false
        var estatico = false
        while esId("virtual") || esId("static") || esId("inline") || esId("explicit") || esId("constexpr") || esId("mutable") {
            if esId("virtual") { virtual = true }
            if esId("static") { estatico = true }
            avanza()
        }
        let ln = linea
        // destructor
        if acepta("~") {
            _ = try ident()
            let fn = try funcion("~" + c.nombre, .vacio, ln, clase: c)
            c.dtor = fn
            return
        }
        // constructor: Nombre(…)
        if esId(c.nombre) && mira(1).es("(") {
            avanza()
            let fn = try funcion(c.nombre, .vacio, ln, clase: c, esCtor: true)
            c.ctors.append(fn)
            return
        }
        // operator tipo() (conversiones)
        if esId("operator") && !mira(1).es("(") && mira(1).k == .id {
            avanza()
            let tt = try tipoCompleto()
            let fn = try funcion("operator " + tt.nombre, tt, ln, clase: c)
            c.metodos[fn.nombre, default: []].append(fn)
            return
        }
        let base = try tipoBase()
        var tt = base
        while true {
            if acepta("*") { tt = .ptr(tt); continue }
            if acepta("&") || acepta("&&") { tt = .ref(tt); continue }
            if esId("const") { avanza(); continue }
            break
        }
        if es("(") && mira(1).es("*") {
            try camposDe(c, base, estatico: estatico, ln, primero: nil)
            return
        }
        let n = try nombreFuncion()
        if es("(") {
            let fn = try funcion(n, tt, ln, clase: c)
            fn.virtual = fn.virtual || virtual
            fn.estatico = estatico
            c.metodos[n, default: []].append(fn)
            return
        }
        try camposDe(c, base, estatico: estatico, ln, primero: (n, tt))
    }

    func camposDe(_ c: CClase, _ base: CTipo, estatico: Bool, _ ln: Int, primero: (String, CTipo)? = nil) throws {
        var actual = primero
        while true {
            var nombre: String
            var tt: CTipo
            if let p = actual {
                nombre = p.0
                tt = p.1
                actual = nil
            } else {
                tt = base
                while acepta("*") { tt = .ptr(tt) }
                while acepta("&") { tt = .ref(tt) }
                if es("(") && mira(1).es("*") {
                    avanza(); avanza()
                    nombre = try ident()
                    try espera(")")
                    saltaEquilibrado()
                    tt = .fn
                } else {
                    nombre = try ident()
                }
            }
            let dv = try unDeclarador(nombre, tt, ln)
            // int bits : 3 (campos de bits): se ignora el ancho
            if acepta(":") { _ = try condicional() }
            var ini = dv.ini
            if ini == nil, let args = dv.ctorArgs { ini = CListaIni(args, nombres: args.map { _ in nil }, ln) }
            c.campos.append(CCampo(nombre: nombre, tipo: dv.tipo, ini: ini, estatico: estatico, linea: ln))
            if !acepta(",") { break }
        }
        try espera(";")
    }

    func defineEnum() throws -> CTipo {
        var nombre: String?
        var conAmbito = false
        if esId("class") || esId("struct") { avanza(); conAmbito = true }
        if mira().k == .id { nombre = avanza().t; tipos.insert(nombre!) }
        if acepta(":") { _ = try tipoBase() }
        if acepta(";") { return .num(.int) }
        try espera("{")
        var v: Int64 = 0
        var ambito: [String: CV] = [:]
        while !es("}") {
            let n = try ident()
            if acepta("=") {
                let e = try condicional()
                guard let k = constanteDe(e) else { throw CFallo("el valor de '\(n)' tiene que ser una constante", linea) }
                v = k
            }
            ambito[n] = .n(v, .int)
            if !conAmbito { prog.enumConst[n] = .n(v, .int); constantes[n] = v }
            v += 1
            if !acepta(",") { break }
        }
        try espera("}")
        if let n = nombre {
            prog.enumAmbito[n] = ambito
            typedefs[n] = .num(.int)
        }
        return .num(.int)
    }

    /// Valor de una expresión constante (tamaños de arreglos, enum).
    func constanteDe(_ e: CExpr) -> Int64? {
        if let l = e as? CLit, case .n(let x, _) = l.v { return x }
        if let n = e as? CNombre {
            if let k = constantes[n.nombre] { return k }
            if case .n(let x, _)? = prog.enumConst[n.nombre] { return x }
            return nil
        }
        if let u = e as? CUn, let x = constanteDe(u.a) {
            if u.op == "-" { return 0 &- x }
            if u.op == "~" { return ~x }
            if u.op == "+" { return x }
            if u.op == "!" { return x == 0 ? 1 : 0 }
        }
        if let b = e as? CBin, let x = constanteDe(b.a), let y = constanteDe(b.b) {
            if (b.op == .div || b.op == .mod) && y == 0 { return nil }
            return CEvalPre.aplica(b.op.texto, x, y)
        }
        if let s = e as? CSizeof, let tt = s.t { return Int64(CMaq.tamanoEstatico(tt, prog)) }
        if let c = e as? CCast { return constanteDe(c.a) }
        return nil
    }

    // MARK: Java

    func modificadores() -> (estatico: Bool, abstracto: Bool, defecto: Bool) {
        var est = false
        var abs = false
        var def = false
        while true {
            if es("@") {
                if mira(1).esId("interface") { return (est, abs, def) }
                avanza(); avanza()
                while es(".") { avanza(); avanza() }
                if es("(") { saltaEquilibrado() }
                continue
            }
            let x = mira()
            guard x.k == .id, CParser.modJava.contains(x.t) else { break }
            if x.t == "static" { est = true }
            if x.t == "abstract" { abs = true }
            if x.t == "default" { def = true }
            avanza()
        }
        return (est, abs, def)
    }

    func declaracionJava() throws {
        if acepta(";") { return }
        if esId("package") || esId("import") {
            while !acepta(";") && mira().k != .fin { avanza() }
            return
        }
        let mods = modificadores()
        if es("@") && mira(1).esId("interface") { avanza(); avanza(); _ = try ident(); saltaEquilibrado(); return }
        if esId("class") || esId("interface") || esId("enum") || esId("record") {
            _ = try claseJava(abstracta: mods.abstracto)
            return
        }
        throw CFallo("en Java todo va dentro de una clase (aparece '\(mira().t)')", linea)
    }

    func claseJava(abstracta: Bool, nombreFijo: String? = nil) throws -> CClase {
        let kw = avanza().t
        let ln = linea
        let nombre = try nombreFijo ?? ident()
        tipos.insert(nombre)
        let c = prog.clase(nombre, ln)
        if let ex = claseActual { prog.exterior[nombre] = ex }
        c.esInterfaz = kw == "interface"
        c.esAbstracta = abstracta || c.esInterfaz
        c.esEnum = kw == "enum"
        let plantillaPrevia = plantilla
        if es("<") {
            avanza()
            var prof = 1
            while prof > 0 && mira().k != .fin {
                let x = avanza()
                if x.es("<") { prof += 1 }
                if x.es(">") { prof -= 1 }
                if x.es(">>") { prof -= 2 }
                if x.k == .id && (mira().es(",") || mira().es(">") || mira().es(">>") || mira().esId("extends")) && prof == 1 {
                    plantilla.insert(x.t)
                }
            }
        }
        var componentes: [CParam] = []
        if kw == "record" {
            (componentes, _) = try parametros()
        }
        if aceptaId("extends") {
            repeat {
                let b = try tipoJava()
                if case .clase(let bn) = b {
                    if c.esInterfaz { c.interfaces.append(bn) } else { c.nombrePadre = bn }
                } else if case .cont(let bn, _) = b {
                    c.interfaces.append(bn)
                }
            } while acepta(",")
        }
        if aceptaId("implements") {
            repeat {
                let b = try tipoJava()
                if case .clase(let bn) = b { c.interfaces.append(bn) }
                if case .cont(let bn, _) = b { c.interfaces.append(bn) }
            } while acepta(",")
        }
        if aceptaId("permits") { while !es("{") { avanza() } }
        if c.nombrePadre == nil && !c.esInterfaz && nombre != "Object" {
            c.nombrePadre = c.esEnum ? nil : nil
        }
        try espera("{")
        let anterior = claseActual
        claseActual = c
        defer { claseActual = anterior; plantilla = plantillaPrevia }
        if kw == "record" { registro(c, componentes, ln) }
        if c.esEnum { try constantesEnum(c) }
        while !es("}") && mira().k != .fin {
            try miembroJava(c)
        }
        try espera("}")
        return c
    }

    func registro(_ c: CClase, _ comps: [CParam], _ ln: Int) {
        for p in comps {
            c.campos.append(CCampo(nombre: p.nombre, tipo: p.tipo, ini: nil, estatico: false, linea: ln))
            let acc = CFuncion(p.nombre, params: [], ret: p.tipo, linea: ln)
            acc.cuerpo = CBloque([CVuelve(CNombre(p.nombre, ln), ln)], ln)
            acc.clase = c
            c.metodos[p.nombre, default: []].append(acc)
        }
        let ctor = CFuncion(c.nombre, params: comps, ret: .vacio, linea: ln)
        ctor.clase = c
        var cuerpo: [CSent] = []
        for p in comps {
            cuerpo.append(CExprSent(CAsig(nil, CMiembro(CEste(ln), p.nombre, flecha: false, ln), CNombre(p.nombre, ln), ln), ln))
        }
        ctor.cuerpo = CBloque(cuerpo, ln)
        c.ctors.append(ctor)
        c.esEnum = false
        registros.insert(c.nombre)
    }

    var registros: Set<String> = []

    func constantesEnum(_ c: CClase) throws {
        while mira().k == .id && !es(";") {
            let ln = linea
            let n = try ident()
            var args: [CExpr] = []
            if es("(") { args = try argumentos() }
            if es("{") {
                // constante con cuerpo propio: se toman sus métodos en la clase
                saltaEquilibrado()
            }
            c.enumArgs.append((n, args, ln))
            if !acepta(",") { break }
        }
        _ = acepta(";")
    }

    func miembroJava(_ c: CClase) throws {
        if acepta(";") { return }
        if es("{") {
            c.iniInstancia.append(try bloque())
            return
        }
        if esId("static") && mira(1).es("{") {
            avanza()
            c.iniEstatico.append(try bloque())
            return
        }
        let mods = modificadores()
        if esId("class") || esId("interface") || esId("enum") || esId("record") {
            _ = try claseJava(abstracta: mods.abstracto)
            return
        }
        let plantillaPrevia = plantilla
        defer { plantilla = plantillaPrevia }
        if es("<") {
            // método genérico: <T> void f(T x)
            avanza()
            var prof = 1
            while prof > 0 && mira().k != .fin {
                let x = avanza()
                if x.es("<") { prof += 1 }
                if x.es(">") { prof -= 1 }
                if x.es(">>") { prof -= 2 }
                if x.k == .id && prof == 1 && !x.esId("extends") && !x.esId("super") { plantilla.insert(x.t); tipos.insert(x.t) }
            }
        }
        let ln = linea
        // constructor
        if esId(c.nombre) && mira(1).es("(") {
            avanza()
            let fn = try funcion(c.nombre, .vacio, ln, clase: c)
            c.ctors.append(fn)
            return
        }
        // constructor compacto de un record: Nombre { … }
        if esId(c.nombre) && mira(1).es("{") && registros.contains(c.nombre) {
            avanza()
            let cuerpo = try bloque()
            if let ctor = c.ctors.first, let b = ctor.cuerpo as? CBloque { b.s.insert(cuerpo, at: 0) }
            return
        }
        let tt = try tipoJava()
        let n = try ident()
        if es("(") {
            let fn = try funcion(n, tt, ln, clase: c)
            fn.estatico = mods.estatico
            if fn.cuerpo == nil && (c.esInterfaz || mods.abstracto) { fn.abstracto = true }
            fn.virtual = true
            c.metodos[n, default: []].append(fn)
            return
        }
        // campos: en una interfaz son constantes estáticas
        let estatico = mods.estatico || c.esInterfaz
        var dv = try unDeclarador(n, tt, ln)
        c.campos.append(CCampo(nombre: n, tipo: dv.tipo, ini: dv.ini, estatico: estatico, linea: ln))
        while acepta(",") {
            let n2 = try ident()
            dv = try unDeclarador(n2, tt, linea)
            c.campos.append(CCampo(nombre: n2, tipo: dv.tipo, ini: dv.ini, estatico: estatico, linea: ln))
        }
        try espera(";")
    }

    // MARK: sentencias

    func bloque() throws -> CBloque {
        let ln = linea
        try espera("{")
        var s: [CSent] = []
        while !es("}") {
            if mira().k == .fin { throw CFallo("falta '}' para cerrar el bloque que empieza en la línea \(ln)", ln) }
            s.append(try sentencia())
        }
        try espera("}")
        return CBloque(s, ln)
    }

    func sentencia() throws -> CSent {
        let ln = linea
        let x = mira()
        if x.es("{") { return try bloque() }
        if x.es(";") { avanza(); return CVacia(ln) }
        if x.k == .id {
            switch x.t {
            case "if": return try siSent()
            case "while":
                avanza()
                try espera("(")
                let c = try condicionDecl()
                try espera(")")
                return CMientras(c, try sentencia(), ln)
            case "do":
                avanza()
                let cuerpo = try sentencia()
                guard aceptaId("while") else { throw CFallo("falta 'while' después de do { … }", linea) }
                try espera("(")
                let c = try expresion()
                try espera(")")
                try espera(";")
                return CHacer(cuerpo, c, ln)
            case "for": return try paraSent()
            case "switch" where !mira(1).es("."): return try switchSent()
            case "break":
                avanza()
                var et: String?
                if mira().k == .id { et = avanza().t }
                try espera(";")
                return CRompe(et, ln)
            case "continue":
                avanza()
                var et: String?
                if mira().k == .id { et = avanza().t }
                try espera(";")
                return CSigue(et, ln)
            case "return":
                avanza()
                if acepta(";") { return CVuelve(nil, ln) }
                let e = es("{") ? try listaIni() : try expresion()
                try espera(";")
                return CVuelve(e, ln)
            case "yield" where d == .java && !mira(1).es("=") && !mira(1).es("("):
                avanza()
                let e = try expresion()
                try espera(";")
                return CRinde(e, ln)
            case "try": return try intentaSent()
            case "throw":
                avanza()
                if acepta(";") { return CLanza(nil, ln) }
                let e = try expresion()
                try espera(";")
                return CLanza(e, ln)
            case "goto":
                throw CFallo("goto no está soportado (usa break, continue o funciones)", ln)
            case "typedef":
                try typedefDecl()
                return CVacia(ln)
            case "using" where d == .cpp:
                try usando()
                return CVacia(ln)
            case "static_assert":
                while !acepta(";") { avanza() }
                return CVacia(ln)
            case "struct", "class", "union", "enum":
                if d == .java {
                    if x.t != "struct" && x.t != "union" { _ = try claseJava(abstracta: false); return CVacia(ln) }
                } else if mira(1).es("{") || mira(2).es("{") || mira(2).es(":") {
                    return try declLocalTipo()
                }
            case "interface", "record":
                if d == .java { _ = try claseJava(abstracta: false); return CVacia(ln) }
            case "synchronized" where d == .java && mira(1).es("("):
                avanza()
                saltaEquilibrado()
                return try bloque()
            default:
                break
            }
            // etiqueta:  (Java y C)
            if mira(1).es(":") && !mira(2).es(":") && x.t != "default" && x.t != "case" {
                avanza(); avanza()
                return CEtiqueta(x.t, try sentencia(), ln)
            }
        }
        if empiezaDecl() {
            let dcl = try declLocal()
            try espera(";")
            return dcl
        }
        let e = try expresion()
        try espera(";")
        return CExprSent(e, ln)
    }

    func declLocalTipo() throws -> CSent {
        let kw = avanza().t
        if kw == "enum" {
            _ = try defineEnum()
            if acepta(";") { return CVacia(linea) }
            pendientes = []
            try variablesTras(.num(.int), global: false)
            return pendientes.first ?? CVacia(linea)
        }
        let c = try defineClase(kw == "class", esUnion: kw == "union")
        if acepta(";") { return CVacia(linea) }
        pendientes = []
        try variablesTras(.clase(c.nombre), global: false)
        return pendientes.first ?? CVacia(linea)
    }

    /// Declaración local (sin el ';').
    func declLocal() throws -> CDecl {
        let ln = linea
        var estatico = false
        if esId("static") { estatico = true }
        let base = try tipoBase()
        var vars: [CDeclVar] = []
        repeat {
            var tt = base
            while true {
                if acepta("*") { tt = .ptr(tt); continue }
                if acepta("&") || acepta("&&") { tt = .ref(tt); continue }
                if esId("const") { avanza(); continue }
                break
            }
            // auto [a, b] = par;
            if acepta("[") {
                var nombres: [String] = []
                repeat { nombres.append(try ident()) } while acepta(",")
                try espera("]")
                let v = CDeclVar("$enlace", .auto, ini: nil, dims: [], linea)
                v.enlaces = nombres
                if acepta("=") { v.ini = try asignacion() } else if es("{") { v.ini = try listaIni() }
                vars.append(v)
                continue
            }
            if es("(") && mira(1).es("*") {
                avanza(); avanza()
                let n = try ident()
                try espera(")")
                saltaEquilibrado()
                let v = try unDeclarador(n, .fn, linea)
                vars.append(v)
                continue
            }
            let n = try ident()
            let v = try unDeclarador(n, tt, linea)
            v.estatico = estatico
            vars.append(v)
        } while acepta(",")
        return CDecl(vars, ln)
    }

    func condicionDecl() throws -> CExpr {
        // while (int x = f()) / if (auto it = m.find(k); it != m.end())
        return try expresion()
    }

    func siSent() throws -> CSent {
        let ln = linea
        avanza()
        _ = aceptaId("constexpr")
        try espera("(")
        // if (init; cond) de C++17
        var ini: CSent?
        if empiezaDecl() && d == .cpp {
            let dcl = try declLocal()
            if acepta(";") { ini = dcl } else {
                try espera(")")
                // if (int x = f()) { … }
                let v = dcl.vars[0]
                let cond = CNombre(v.nombre, ln)
                let a = try sentencia()
                var b: CSent?
                if aceptaId("else") { b = try sentencia() }
                return CBloque([dcl, CSi(cond, a, b, ln)], ln)
            }
        }
        let c = try expresion()
        try espera(")")
        let a = try sentencia()
        var b: CSent?
        if aceptaId("else") { b = try sentencia() }
        let si = CSi(c, a, b, ln)
        if let i = ini { return CBloque([i, si], ln) }
        return si
    }

    func paraSent() throws -> CSent {
        let ln = linea
        avanza()
        try espera("(")
        // for (tipo x : coleccion)
        if empiezaDecl() {
            let guardado = i
            let base = try tipoBase()
            var esRef = false
            while acepta("&") || acepta("&&") || acepta("*") || esId("const") {
                if mira(-1).es("&") || mira(-1).es("&&") { esRef = true }
                if esId("const") { avanza() }
            }
            var nombres: [String] = []
            if acepta("[") {
                repeat { nombres.append(try ident()) } while acepta(",")
                try espera("]")
            } else if mira().k == .id {
                nombres.append(avanza().t)
            }
            if !nombres.isEmpty && acepta(":") {
                let col = try expresion()
                try espera(")")
                let cuerpo = try sentencia()
                return CParaCada(nombres, base, esRef: esRef, col, cuerpo, ln)
            }
            i = guardado
        }
        var ini: CSent?
        if !es(";") {
            if empiezaDecl() { ini = try declLocal() } else { ini = CExprSent(try expresion(), ln) }
        }
        try espera(";")
        var c: CExpr?
        if !es(";") { c = try expresion() }
        try espera(";")
        var paso: CExpr?
        if !es(")") { paso = try expresion() }
        try espera(")")
        let cuerpo = try sentencia()
        return CPara(ini, c, paso, cuerpo, ln)
    }

    func switchSent() throws -> CSent {
        let ln = linea
        avanza()
        try espera("(")
        let e = try expresion()
        try espera(")")
        try espera("{")
        var cuerpo: [CSent] = []
        var casos: [CCaso] = []
        while !es("}") {
            if mira().k == .fin { throw CFallo("falta '}' del switch", ln) }
            if aceptaId("case") {
                var valores: [CExpr] = []
                repeat {
                    valores.append(try condicional())
                    // case 1 ... 5 (extensión de gcc)
                    if acepta("...") { _ = try condicional() }
                } while acepta(",")
                for v in valores { casos.append(CCaso(v, cuerpo.count)) }
                if acepta("->") {
                    // case X -> sentencia;  (sin caída al siguiente)
                    let s = try sentencia()
                    cuerpo.append(s)
                    cuerpo.append(CRompe(nil, ln))
                    continue
                }
                try espera(":")
                continue
            }
            if aceptaId("default") {
                casos.append(CCaso(nil, cuerpo.count))
                if acepta("->") {
                    cuerpo.append(try sentencia())
                    cuerpo.append(CRompe(nil, ln))
                    continue
                }
                try espera(":")
                continue
            }
            cuerpo.append(try sentencia())
        }
        try espera("}")
        return CSwitch(e, cuerpo, casos, ln)
    }

    func intentaSent() throws -> CSent {
        let ln = linea
        avanza()
        var recursos: [CDecl] = []
        if acepta("(") {
            // try (Scanner sc = new Scanner(System.in)) { … }
            while !es(")") {
                recursos.append(try declLocal())
                if !acepta(";") { break }
            }
            try espera(")")
        }
        let cuerpo = try bloque()
        var capturas: [CCaptura] = []
        while aceptaId("catch") {
            try espera("(")
            var tiposC: [CTipo] = []
            var nombre: String?
            if acepta("...") {
                // catch (...)
            } else {
                repeat {
                    var tt = try tipoBase()
                    while acepta("&") || acepta("*") || esId("const") { if esId("const") { avanza() }; tt = tt.sinRef }
                    tiposC.append(tt)
                } while acepta("|")
                if mira().k == .id { nombre = avanza().t }
            }
            try espera(")")
            capturas.append(CCaptura(tiposC, nombre, try bloque()))
        }
        var final: CSent?
        if aceptaId("finally") { final = try bloque() }
        if capturas.isEmpty && final == nil && recursos.isEmpty { throw CFallo("try sin catch ni finally", ln) }
        return CIntenta(cuerpo, capturas, final, recursos: recursos, ln)
    }

    // MARK: expresiones

    func expresion() throws -> CExpr {
        var e = try asignacion()
        while es(",") && d != .java {
            let ln = linea
            avanza()
            e = CComa(e, try asignacion(), ln)
        }
        return e
    }

    static let asignaciones: [String: COp?] = ["=": nil, "+=": .suma, "-=": .resta, "*=": .mult, "/=": .div,
                                               "%=": .mod, "&=": .y, "|=": .o, "^=": .xor, "<<=": .shl,
                                               ">>=": .shr, ">>>=": .ushr]

    func asignacion() throws -> CExpr {
        if let l = try lambdaJava() { return l }
        let ln = linea
        if esId("throw") && d == .cpp {
            avanza()
            let e = try asignacion()
            let fn = CFuncion("$throw", params: [], ret: .auto, linea: ln)
            fn.cuerpo = CLanza(e, ln)
            return CLlamada(CLambda(fn, ln), [], ln)
        }
        let izq = try condicional()
        let x = mira()
        if x.k == .op, let op = CParser.asignaciones[x.t] {
            avanza()
            let der = es("{") ? try listaIni() : try asignacion()
            return CAsig(op, izq, der, ln)
        }
        return izq
    }

    func condicional() throws -> CExpr {
        let c = try binaria(0)
        if es("?") {
            let ln = linea
            avanza()
            let a = try asignacion()
            try espera(":")
            let b = try asignacion()
            return CTernario(c, a, b, ln)
        }
        return c
    }

    static let niveles: [[String]] = [["||"], ["&&"], ["|"], ["^"], ["&"], ["==", "!="],
                                      ["<", ">", "<=", ">="], ["<<", ">>", ">>>"], ["+", "-"], ["*", "/", "%"]]

    func binaria(_ n: Int) throws -> CExpr {
        if n >= CParser.niveles.count { return try unaria() }
        var a = try binaria(n + 1)
        while true {
            let x = mira()
            if n == 6 && d == .java && x.esId("instanceof") {
                avanza()
                _ = aceptaId("final")
                var nombreTipo = mira().t
                while mira(1).es(".") && mira(2).k == .id { avanza(); avanza(); nombreTipo = mira().t }
                let tt = try tipoJava()
                var enlace: String?
                if mira().k == .id && !CParser.modJava.contains(mira().t) { enlace = avanza().t }
                let ins = CInstancia(a, tt, enlace: enlace, x.linea)
                ins.nombreTipo = nombreTipo
                a = ins
                continue
            }
            guard x.k == .op, CParser.niveles[n].contains(x.t) else { break }
            avanza()
            let b = try binaria(n + 1)
            if x.t == "&&" { a = CLogico(true, a, b, x.linea) }
            else if x.t == "||" { a = CLogico(false, a, b, x.linea) }
            else if let op = COp.de(x.t) { a = CBin(op, a, b, x.linea) }
        }
        return a
    }

    func unaria() throws -> CExpr {
        let x = mira()
        let ln = x.linea
        if x.k == .op {
            switch x.t {
            case "++", "--":
                avanza()
                return CIncDec(pre: true, delta: x.t == "++" ? 1 : -1, try unaria(), ln)
            case "-", "+", "!", "~":
                avanza()
                let a = try unaria()
                // -5 como literal (para los case)
                if x.t == "-", let l = a as? CLit {
                    if case .n(let v, let k) = l.v { return CLit(.n(cAjusta(0 &- v, k), k), ln) }
                    if case .d(let v, let f) = l.v { return CLit(.d(-v, f), ln) }
                }
                return CUn(x.t, a, ln)
            case "*" where d != .java:
                avanza()
                return CDeref(try unaria(), ln)
            case "&" where d != .java:
                avanza()
                return CDir(try unaria(), ln)
            case "&&" where d != .java:
                avanza()
                return CDir(CDir(try unaria(), ln), ln)
            case "(":
                if let c = try intentaCast() { return c }
            default:
                break
            }
        }
        if x.k == .id {
            switch x.t {
            case "sizeof" where d != .java:
                avanza()
                if es("(") && esTipoInicio(1) && !mira(2).es("(") {
                    avanza()
                    let tt = try tipoCompleto()
                    var t2 = tt
                    while acepta("[") {
                        let e = try expresion()
                        t2 = .arr(t2, Int(constanteDe(e) ?? 0))
                        try espera("]")
                    }
                    try espera(")")
                    return CSizeof(tipo: t2, expr: nil, ln)
                }
                if acepta("...") { saltaEquilibrado(); return CLit(.n(0, .ulong), ln) }
                return CSizeof(tipo: nil, expr: try unaria(), ln)
            case "new":
                return try postfija(try nuevo())
            case "delete" where d == .cpp:
                avanza()
                if acepta("[") { try espera("]") }
                return CBorrar(try unaria(), ln)
            default:
                break
            }
        }
        return try postfija(try primaria())
    }

    func intentaCast() throws -> CExpr? {
        // (tipo) expresión
        guard es("("), esTipoInicio(1) else { return nil }
        let guardado = i
        avanza()
        let x = mira()
        // (Clase) en Java solo si lo que sigue no es un operador
        if d != .java, !CParser.primC.contains(x.t), !CParser.califC.contains(x.t), x.t != "std" {
            let y = mira(1)
            if y.es("(") || y.es(".") || y.es("->") || y.es("[") || y.es("::") && !esTipoAnidado(0) { i = guardado; return nil }
        }
        if d == .java, !CParser.primJava.contains(x.t) {
            let y = mira(1)
            if y.es("(") || y.es(".") || y.es("[") && !mira(2).es("]") || y.es("::") || y.es("->") { i = guardado; return nil }
        }
        guard let tt = try? tipoCompleto(), es(")") else { i = guardado; return nil }
        avanza()
        let sig = mira()
        // (a) + b con 'a' variable: no es cast. Con tipo primitivo, siempre lo es.
        if d == .java, case .clase = tt, sig.k == .op, ["+", "-", ")", ";", ",", "*", "/"].contains(sig.t) { i = guardado; return nil }
        if sig.es(")") || sig.es(";") || sig.es(",") || (sig.k == .op && ["*", "/", "%", "==", "<", ">", "?", ":", "]", "}", "=", "|", "^"].contains(sig.t) && d != .java && !sig.es("*")) {
            i = guardado
            return nil
        }
        // (int[]) en Java, (int){…} literal compuesto de C
        if es("{") {
            let l = try listaIni()
            l.tipo = tt
            return CCast(tt, l, x.linea)
        }
        return CCast(tt, try unaria(), x.linea)
    }

    func nuevo() throws -> CExpr {
        let ln = linea
        avanza()
        if es("(") { saltaEquilibrado() }   // new (lugar) T
        var tt: CTipo
        if d == .java {
            let n = try ident()
            switch n {
            case "int": tt = .num(.int)
            case "long": tt = .num(.long)
            case "double": tt = .real(false)
            case "float": tt = .real(true)
            case "char": tt = .num(.jchar)
            case "boolean": tt = .num(.bool)
            case "byte": tt = .num(.char)
            case "short": tt = .num(.short)
            default:
                var nombre = n
                while es(".") && mira(1).k == .id { avanza(); nombre = avanza().t }
                if es("<") {
                    if mira(1).es(">") { avanza(); avanza(); tt = try tipoNombrado(nombre) }
                    else { tt = try tipoNombrado(nombre) }
                } else {
                    tt = try tipoNombrado(nombre)
                }
            }
        } else {
            tt = try tipoBase()
            while acepta("*") { tt = .ptr(tt) }
        }
        var dims: [CExpr] = []
        var ini: CListaIni?
        if es("[") {
            var niveles = 0
            while acepta("[") {
                niveles += 1
                if acepta("]") { continue }
                dims.append(try expresion())
                try espera("]")
            }
            if es("{") { ini = try listaIni() }
            let nv = CNuevo(tt, args: [], dims: dims, ini: ini, anonima: nil, ln)
            nv.niveles = niveles
            return nv
        }
        var args: [CExpr] = []
        if es("(") { args = try argumentos() }
        else if es("{") && d == .cpp { ini = try listaIni(); args = ini!.elems; ini = nil }
        var anonima: CClase?
        if d == .java && es("{") {
            anonimas += 1
            let nombre = "$Anonima\(anonimas)"
            let c = prog.clase(nombre, ln)
            switch tt {
            case .clase(let b): if prog.clases[b]?.esInterfaz == true { c.interfaces.append(b) } else { c.nombrePadre = b }
            case .cont(let b, _): c.interfaces.append(b)
            default: break
            }
            try espera("{")
            let anterior = claseActual
            claseActual = c
            while !es("}") && mira().k != .fin { try miembroJava(c) }
            claseActual = anterior
            try espera("}")
            anonima = c
            tt = .clase(nombre)
        }
        return CNuevo(tt, args: args, dims: [], ini: nil, anonima: anonima, ln)
    }

    func listaIni() throws -> CListaIni {
        let ln = linea
        try espera("{")
        var elems: [CExpr] = []
        var nombres: [String?] = []
        while !es("}") {
            if es(".") && mira(1).k == .id && mira(2).es("=") {
                avanza()
                nombres.append(avanza().t)
                avanza()
            } else if es("[") && d == .c {
                // [3] = x (designado de arreglo): se toma en orden
                saltaEquilibrado()
                try espera("=")
                nombres.append(nil)
            } else {
                nombres.append(nil)
            }
            elems.append(es("{") ? try listaIni() : try asignacion())
            if !acepta(",") { break }
        }
        try espera("}")
        return CListaIni(elems, nombres: nombres, ln)
    }

    func lambdaJava() throws -> CExpr? {
        guard d == .java else { return nil }
        let ln = linea
        // x -> …
        if mira().k == .id && mira(1).es("->") {
            let n = avanza().t
            avanza()
            return try cuerpoLambda([CParam(nombre: n, tipo: .auto, defecto: nil)], ln)
        }
        // (a, b) -> … / (int a, int b) -> …
        guard es("(") else { return nil }
        var k = 1
        var prof = 1
        while prof > 0 {
            let x = mira(k)
            if x.k == .fin { return nil }
            if x.es("(") { prof += 1 }
            if x.es(")") { prof -= 1 }
            k += 1
        }
        guard mira(k).es("->") else { return nil }
        avanza()
        var ps: [CParam] = []
        while !es(")") {
            var n = try ident()
            var tt: CTipo = .auto
            if mira().k == .id || es("[") || es("<") {
                i -= 1
                tt = try tipoJava()
                n = try ident()
            }
            ps.append(CParam(nombre: n, tipo: tt, defecto: nil))
            if !acepta(",") { break }
        }
        try espera(")")
        try espera("->")
        return try cuerpoLambda(ps, ln)
    }

    func cuerpoLambda(_ ps: [CParam], _ ln: Int) throws -> CExpr {
        let fn = CFuncion("lambda", params: ps, ret: .auto, linea: ln)
        if es("{") {
            fn.cuerpo = try bloque()
        } else {
            let e = try asignacion()
            fn.cuerpo = CVuelve(e, ln)
        }
        return CLambda(fn, ln)
    }

    func lambdaCpp() throws -> CExpr {
        let ln = linea
        try espera("[")
        var prof = 1
        while prof > 0 && mira().k != .fin {
            let x = avanza()
            if x.es("[") { prof += 1 }
            if x.es("]") { prof -= 1 }
        }
        var ps: [CParam] = []
        if es("(") { (ps, _) = try parametros() }
        while esId("mutable") || esId("constexpr") || esId("noexcept") { avanza() }
        var ret: CTipo = .auto
        if acepta("->") { ret = try tipoCompleto() }
        let fn = CFuncion("lambda", params: ps, ret: ret, linea: ln)
        fn.cuerpo = try bloque()
        return CLambda(fn, ln)
    }

    func primaria() throws -> CExpr {
        let x = mira()
        let ln = x.linea
        switch x.k {
        case .ent:
            avanza()
            var k: CNum = .int
            if x.largo { k = x.sinSigno ? .ulong : .long }
            else if x.sinSigno { k = x.v > Int64(UInt32.max) ? .ulong : .uint }
            else if x.v > Int64(Int32.max) || x.v < Int64(Int32.min) { k = .long }
            return CLit(.n(x.v, k), ln, tipo: .num(k))
        case .real:
            avanza()
            return CLit(.d(x.flotante ? Double(Float(x.dv)) : x.dv, x.flotante), ln, tipo: .real(x.flotante))
        case .car:
            avanza()
            let k: CNum = d == .java ? .jchar : .char
            return CLit(.n(d == .java ? x.v : cAjusta(x.v, .char), k), ln, tipo: .num(k))
        case .cad:
            var s = x.t
            avanza()
            while mira().k == .cad { s += avanza().t }
            return CLitCad(s, ln)
        case .fin:
            throw CFallo("el programa termina a medias (falta algo al final)", ln)
        default:
            break
        }
        if x.es("(") {
            avanza()
            let e = try expresion()
            try espera(")")
            return e
        }
        if x.es("{") { return try listaIni() }
        if x.es("[") && d == .cpp { return try lambdaCpp() }
        if x.es("::") { avanza(); return try primaria() }
        guard x.k == .id else { throw CFallo("expresión no válida: aparece '\(x.t)'", ln) }
        switch x.t {
        case "true": avanza(); return CLit(.n(1, .bool), ln, tipo: .num(.bool))
        case "false": avanza(); return CLit(.n(0, .bool), ln, tipo: .num(.bool))
        case "NULL", "nullptr": avanza(); return CLit(.ptrNulo, ln, tipo: .ptr(.vacio))
        case "null" where d == .java: avanza(); return CLit(.nulo, ln, tipo: .auto)
        case "this":
            avanza()
            if d == .java && es("(") { return CLlamada(CNombre("$this", ln), try argumentos(), ln) }
            return CEste(ln)
        case "super" where d == .java:
            avanza()
            if es("(") { return CLlamada(CNombre("$super", ln), try argumentos(), ln) }
            try espera(".")
            let n = try ident()
            if es("(") { return CLlamada(CMiembro(CNombre("$super", ln), n, flecha: false, ln), try argumentos(), ln) }
            return CMiembro(CEste(ln), n, flecha: false, ln)
        case "switch" where d == .java:
            return try switchExpr()
        case "static_cast", "reinterpret_cast", "const_cast", "dynamic_cast":
            avanza()
            try espera("<")
            let tt = try tipoCompleto()
            try espera(">")
            try espera("(")
            let e = try expresion()
            try espera(")")
            return CCast(tt, e, ln)
        default:
            break
        }
        // std::algo → algo
        if x.t == "std" && mira(1).es("::") {
            avanza(); avanza()
            return try primaria()
        }
        // Tipo(args) o Tipo{…}: construir un temporal (C++): string(3,'a'), vector<int>(n), Punto{1,2}
        if d == .cpp, esTipoInicio(), !mira(1).es("::") || esTipoAnidado(0) {
            let n = x.t
            if CParser.primC.contains(n) && mira(1).es("(") {
                // int(x), double(y): cast funcional
                let tt = try tipoBase()
                let args = try argumentos()
                return CCast(tt, args.first ?? CLit(.cero, ln), ln)
            }
            if !CParser.primC.contains(n) && !CParser.califC.contains(n) {
                let guardado = i
                avanza()
                var tt: CTipo
                if es("<") {
                    if let targs = try? argumentosPlantilla() { tt = n == "string" ? .cad : (prog.clases[n] != nil ? .clase(n) : .cont(n, targs)) }
                    else { i = guardado; return try nombreSimple() }
                } else {
                    tt = (try? tipoNombrado(n)) ?? .clase(n)
                }
                if es("::") { i = guardado; return try nombreSimple() }
                if es("(") {
                    let args = try argumentos()
                    return CLlamada(CNombre("$construye", ln), args, ln).conTipo(tt)
                }
                if es("{") {
                    let l = try listaIni()
                    l.tipo = tt
                    return CCast(tt, l, ln)
                }
                i = guardado
            }
        }
        return try nombreSimple()
    }

    func nombreSimple() throws -> CExpr {
        let ln = linea
        let n = try ident()
        var e: CExpr = CNombre(n, ln)
        if n == "numeric_limits" && es("<") {
            let args = try argumentosPlantilla()
            try espera("::")
            let q = try ident()
            if es("(") { _ = try argumentos() }
            return CLit(CMaq.limite(args.first ?? .num(.int), q), ln)
        }
        // plantilla en una llamada: max<int>(a, b), numeric_limits<int>::max()
        if es("<") && d == .cpp && (esTipoInicio(1) || mira(1).k == .ent) {
            let guardado = i
            if let targs = try? argumentosPlantilla(), es("(") || es("::") || es("{") {
                (e as? CNombre)?.plantilla = targs
            } else {
                i = guardado
            }
        }
        // Clase::miembro (C++)
        while es("::") && d == .cpp {
            avanza()
            if acepta("~") { _ = try ident(); continue }
            let m = try nombreFuncion()
            e = CMiembro(e, m, flecha: false, ln)
            if es("<") && d == .cpp {
                let guardado = i
                if (try? argumentosPlantilla()) == nil || !es("(") { i = guardado }
            }
        }
        return e
    }

    func switchExpr() throws -> CExpr {
        let ln = linea
        avanza()
        try espera("(")
        let e = try expresion()
        try espera(")")
        try espera("{")
        var casos: [([CExpr], CSent)] = []
        var defecto: CSent?
        while !es("}") {
            var valores: [CExpr] = []
            var esDef = false
            if aceptaId("default") { esDef = true } else {
                guard aceptaId("case") else { throw CFallo("se esperaba 'case' en el switch", linea) }
                repeat { valores.append(try condicional()) } while acepta(",")
            }
            let rama: CSent
            if acepta("->") {
                if es("{") { rama = try bloque() }
                else if esId("throw") { rama = try sentencia() }
                else {
                    let v = try expresion()
                    try espera(";")
                    rama = CRinde(v, ln)
                }
            } else {
                try espera(":")
                var s: [CSent] = []
                while !esId("case") && !esId("default") && !es("}") { s.append(try sentencia()) }
                rama = CBloque(s, ln)
            }
            if esDef { defecto = rama } else { casos.append((valores, rama)) }
        }
        try espera("}")
        return CSwitchExpr(e, casos, defecto, ln)
    }

    func postfija(_ base: CExpr) throws -> CExpr {
        var e = base
        while true {
            let x = mira()
            let ln = x.linea
            if x.es("[") {
                avanza()
                let idx = try expresion()
                try espera("]")
                e = CIndice(e, idx, ln)
                continue
            }
            if x.es("(") {
                let args = try argumentos()
                let ll = CLlamada(e, args, ln)
                if let nb = e as? CNombre, !nb.plantilla.isEmpty { ll.plantilla = nb.plantilla }
                e = ll
                continue
            }
            if x.es(".") {
                avanza()
                if esId("template") { avanza() }
                if es("<") && d == .java { _ = try argumentosPlantilla() }
                if aceptaId("class") { e = CLit(.s(Array("class".utf16)), ln); continue }
                let n = try nombreFuncion()
                e = CMiembro(e, n, flecha: false, ln)
                if es("<") && d == .cpp {
                    let guardado = i
                    if (try? argumentosPlantilla()) == nil || !es("(") { i = guardado }
                }
                continue
            }
            if x.es("->") && d != .java {
                avanza()
                let n = try nombreFuncion()
                e = CMiembro(e, n, flecha: true, ln)
                continue
            }
            if x.es("::") && d == .java {
                avanza()
                let n = esId("new") ? avanza().t : try ident()
                if let nm = e as? CNombre, tipos.contains(nm.nombre) || prog.clases[nm.nombre] != nil {
                    e = CRefMetodo(base: nil, clase: nm.nombre, nombre: n, ln)
                } else {
                    e = CRefMetodo(base: e, clase: nil, nombre: n, ln)
                }
                continue
            }
            if x.es("++") || x.es("--") {
                avanza()
                e = CIncDec(pre: false, delta: x.t == "++" ? 1 : -1, e, ln)
                continue
            }
            break
        }
        return e
    }
}

extension CLlamada {
    func conTipo(_ t: CTipo) -> CLlamada {
        plantilla = [t]
        return self
    }
}
