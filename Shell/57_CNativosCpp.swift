import Foundation

// ============================================================
// MARK: - C++: iostream, string, contenedores y algoritmos
// ============================================================

/// Un tramo [lo, hi) de un vector, arreglo o string (para sort, find…).
struct CRango {
    var lista: CLista?
    var mem: CMem?
    var cad: CRefCaja?
    var lo: Int
    var hi: Int
    var reves = false
}

extension CMaq {

    static let funcionesCpp: Set<String> = [
        "max", "min", "swap", "sort", "stable_sort", "reverse", "fill", "fill_n", "iota", "accumulate", "count", "count_if",
        "find", "find_if", "max_element", "min_element", "unique", "lower_bound", "upper_bound", "binary_search",
        "next_permutation", "prev_permutation", "distance", "transform", "for_each", "all_of", "any_of", "none_of",
        "remove", "remove_if", "replace", "rotate", "copy", "begin", "end", "next", "prev", "to_string", "stoi", "stol",
        "stoll", "stoul", "stoull", "stod", "stof", "getline", "make_pair", "make_tuple", "__gcd", "gcd", "lcm", "setw",
        "setprecision", "setfill", "iter_swap", "partial_sum", "equal", "minmax", "clamp", "reverse_copy", "shuffle",
        "random_shuffle", "nth_element", "partial_sort", "is_sorted", "abs", "tie", "advance", "inner_product",
        "chrono.milliseconds", "chrono.seconds", "this_thread.sleep_for", "ios.sync_with_stdio", "ios_base.sync_with_stdio",
        "$get0", "$get1", "$get2", "$get3", "move", "exit", "quick_exit", "sizeof"
    ]

    static func limite(_ t: CTipo, _ q: String) -> CV {
        switch t {
        case .num(let k):
            var maxV: Int64
            var minV: Int64
            switch k {
            case .bool: maxV = 1; minV = 0
            case .char: maxV = 127; minV = -128
            case .uchar: maxV = 255; minV = 0
            case .short: maxV = 32767; minV = -32768
            case .ushort, .jchar: maxV = 65535; minV = 0
            case .int: maxV = Int64(Int32.max); minV = Int64(Int32.min)
            case .uint: maxV = Int64(UInt32.max); minV = 0
            case .long: maxV = Int64.max; minV = Int64.min
            case .ulong: maxV = -1; minV = 0
            }
            return .n(q == "max" ? maxV : minV, k)
        case .real(let f):
            switch q {
            case "max": return .d(f ? Double(Float.greatestFiniteMagnitude) : Double.greatestFiniteMagnitude, f)
            case "min": return .d(f ? Double(Float.leastNormalMagnitude) : Double.leastNormalMagnitude, f)
            case "lowest": return .d(f ? -Double(Float.greatestFiniteMagnitude) : -Double.greatestFiniteMagnitude, f)
            case "epsilon": return .d(f ? Double(Float.ulpOfOne) : Double.ulpOfOne, f)
            case "infinity": return .d(Double.infinity, f)
            default: return .d(Double.nan, f)
            }
        default:
            return .n(0, .int)
        }
    }

    // MARK: iostream

    func flujo(_ c: CCanal, _ op: COp, _ e: CExpr, _ f: CMarco, _ linea: Int) throws -> CV {
        if op == .shl {
            let v = try e.ev(self, f)
            try salidaFlujo(c, v, linea)
            return .canal(c)
        }
        guard e.asignable else { throw CFallo("'>>' necesita una variable a la derecha", linea) }
        let l = try e.lugar(self, f)
        try entradaFlujo(c, l, linea)
        return .canal(c)
    }

    func salidaFlujo(_ c: CCanal, _ v: CV, _ linea: Int) throws {
        var x = v
        if case .ref(let r) = x { x = try lee(r.l) }
        if case .f(let cl) = x, cl.nombre.hasPrefix("manip:") {
            try aplicaManipulador(c, cl.nombre, linea)
            return
        }
        if case .o = x {
            if let fs = prog.funciones["operator<<"] {
                let cand = fs.filter { $0.params.count == 2 }
                if !cand.isEmpty {
                    let fn = try elige(cand, 2, [.canal(c), x], "operator<<", linea)
                    _ = try llama(fn, [.canal(c), x], este: nil, padre: nil, linea: linea)
                    return
                }
            }
            if case .o(let o) = x, o.clase.nombre != "pair" && o.clase.nombre != "$tupla" {
                throw CFallo("no se puede imprimir un \(o.clase.nombre) con << (escribe operator<<)", linea)
            }
        }
        var bytes = Array(try textoFlujo(c, x, linea).utf8)
        if c.ancho > bytes.count {
            let pad = [UInt8](repeating: c.relleno, count: c.ancho - bytes.count)
            bytes = c.izquierda ? bytes + pad : pad + bytes
        }
        c.ancho = 0
        try escribeCanal(c, bytes)
    }

    func textoFlujo(_ c: CCanal, _ v: CV, _ linea: Int) throws -> String {
        switch v {
        case .d(let d, _):
            if d.isNaN { return d.sign == .minus ? "-nan" : "nan" }
            if d.isInfinite { return d < 0 ? "-inf" : "inf" }
            if c.fijo { return String(format: "%.\(c.precision)f", d) }
            if c.cientifico { return String(format: c.mayusculas ? "%.\(c.precision)E" : "%.\(c.precision)e", d) }
            return String(format: (c.mostrarPunto ? "%#." : "%.") + "\(max(1, c.precision))g", d)
        case .n(let x, let k):
            if k == .bool { return c.boolalpha ? (x != 0 ? "true" : "false") : (x != 0 ? "1" : "0") }
            if k == .char || k == .uchar { return String(decoding: [UInt8(truncatingIfNeeded: x)], as: UTF8.self) }
            if c.base == 16 {
                let s = String(UInt64(bitPattern: k.rango < 4 ? Int64(UInt32(truncatingIfNeeded: x)) : x), radix: 16)
                return c.mayusculas ? s.uppercased() : s
            }
            if c.base == 8 { return String(UInt64(bitPattern: k.rango < 4 ? Int64(UInt32(truncatingIfNeeded: x)) : x), radix: 8) }
            return try texto(v, linea)
        default:
            return try texto(v, linea)
        }
    }

    func aplicaManipulador(_ c: CCanal, _ nombre: String, _ linea: Int) throws {
        let partes = nombre.split(separator: ":").map(String.init)
        let n = partes.count > 1 ? partes[1] : ""
        let arg = partes.count > 2 ? Int(partes[2]) ?? 0 : 0
        switch n {
        case "endl":
            try escribeCanal(c, [10])
            if c.clase == .salida { vuelca() }
        case "ends": try escribeCanal(c, [0])
        case "flush": vuelca()
        case "fixed": c.fijo = true; c.cientifico = false
        case "scientific": c.cientifico = true; c.fijo = false
        case "setprecision": c.precision = arg
        case "setw": c.ancho = arg
        case "setfill": c.relleno = UInt8(truncatingIfNeeded: arg)
        case "left": c.izquierda = true
        case "right": c.izquierda = false
        case "boolalpha": c.boolalpha = true
        case "noboolalpha": c.boolalpha = false
        case "hex": c.base = 16
        case "dec": c.base = 10
        case "oct": c.base = 8
        case "showpoint": c.mostrarPunto = true
        case "uppercase": c.mayusculas = true
        case "nouppercase": c.mayusculas = false
        default: break
        }
    }

    func entradaFlujo(_ c: CCanal, _ l: CLugar, _ linea: Int) throws {
        guard let ent = entradaDe(c) else { c.fallo = true; return }
        if c.fallo { return }
        let actual = try lee(l)
        switch actual {
        case .n(_, let k):
            if k == .char || k == .uchar {
                ent.saltaBlancos()
                let b = ent.lee()
                if b < 0 { c.fallo = true; return }
                try escribe(l, .n(cAjusta(Int64(b), k), k))
                return
            }
            guard let v = ent.entero() else { c.fallo = true; return }
            try escribe(l, .n(cAjusta(v, k), k))
        case .d(_, let f):
            guard let d = ent.real() else { c.fallo = true; return }
            try escribe(l, .d(f ? Double(Float(d)) : d, f))
        case .p(let p):
            guard let w = ent.palabra() else { c.fallo = true; return }
            try escribeC(w, en: p, linea)
        default:
            guard let w = ent.palabra() else { c.fallo = true; return }
            try escribe(l, .s(w.map { UInt16($0) }))
        }
    }

    /// getline(cin, s) / getline(ss, s, ',')
    func getline(_ a: [CV], _ l: [CLugar?], _ linea: Int) throws -> CV {
        let cv = try arg(a, 0, "getline", linea)
        let c = try canalDe(cv, linea)
        guard l.count > 1, let destino = l[1] else { throw CFallo("getline necesita una variable string", linea) }
        var delim: UInt8 = 10
        if a.count > 2 { delim = UInt8(truncatingIfNeeded: try argEnt(a, 2, "getline", linea)) }
        guard let ent = entradaDe(c), ent.hay() else {
            c.fallo = true
            try escribe(destino, .s([]))
            return .canal(c)
        }
        var out: [UInt8] = []
        while ent.hay() {
            let b = ent.b[ent.i]
            ent.i += 1
            if b == delim { break }
            if b == 13 && delim == 10 { continue }
            out.append(b)
        }
        try escribe(destino, .s(out.map { UInt16($0) }))
        return .canal(c)
    }

    // MARK: temporales: string(5,'a'), vector<int>(n, 0), Punto(1,2)…

    func construyeTemporal(_ t: CTipo, _ vals: [CV], _ exprs: [CExpr], _ f: CMarco, _ linea: Int) throws -> CV {
        var a = vals
        for k in 0..<a.count { if case .ref(let r) = a[k] { a[k] = try lee(r.l) } }
        switch t {
        case .cad:
            return try cadenaTemporal(a, linea)
        case .clase(let n):
            guard let c = clases[n] else { throw CFallo("tipo desconocido '\(n)'", linea) }
            return .o(try nuevoObjeto(c, a, linea))
        case .num, .real:
            return convierte(a.first ?? .n(0, .int), t)
        case .cont(let n, let args):
            return try contenedorTemporal(n, args, a, linea)
        case .fn:
            return a.first ?? .vacio
        default:
            return a.first ?? .vacio
        }
    }

    func cadenaTemporal(_ a: [CV], _ linea: Int) throws -> CV {
        if a.isEmpty { return .s([]) }
        if a.count == 2, case .n(let n, _) = a[0], case .n(let ch, _) = a[1] {
            return .s([UInt16](repeating: UInt16(UInt8(truncatingIfNeeded: ch)), count: max(0, Int(n))))
        }
        if a.count == 2, case .it(let b, let i) = a[0], case .it(_, let j) = a[1] {
            var out: [UInt16] = []
            for k in i..<max(i, j) {
                if case .n(let x, _) = try leeIterador(b, k, linea) { out.append(UInt16(UInt8(truncatingIfNeeded: x))) }
            }
            return .s(out)
        }
        let u = try unidadesDe(a[0], linea)
        if a.count == 2 { return .s(Array(u.prefix(Int(try entero(a[1], linea))))) }
        if a.count == 3 {
            let pos = Int(try entero(a[1], linea))
            let n = Int(try entero(a[2], linea))
            return .s(Array(u.dropFirst(pos).prefix(n)))
        }
        return .s(u)
    }

    func contenedorTemporal(_ n: String, _ args: [CTipo], _ a: [CV], _ linea: Int) throws -> CV {
        switch n {
        case "greater", "less", "greater_equal", "less_equal":
            let mayor = n.hasPrefix("greater")
            return .f(CCierre(fn: nil, marco: nil, este: nil, nombre: n, nat: { m, x in
                guard x.count == 2 else { return m.verdad(false) }
                let c = try m.compara(x[0], x[1], linea)
                return m.verdad(mayor ? c > 0 : c < 0)
            }))
        case "pair":
            return par(a.count > 0 ? copia(a[0]) : .n(0, .int), a.count > 1 ? copia(a[1]) : .n(0, .int))
        case "stringstream", "istringstream", "ostringstream":
            let c = CCanal(.cadena)
            let s = a.isEmpty ? "" : try cadenaC(a[0], linea)
            c.buf = n == "istringstream" ? [] : Array(s.utf8)
            c.ent = CEntrada(s)
            return .canal(c)
        case "ifstream", "ofstream", "fstream":
            if a.isEmpty { return .canal(CCanal(n == "ifstream" ? .archivoEntrada : .archivoSalida)) }
            let nombre = try cadenaC(a[0], linea)
            var modo = n == "ifstream" ? "r" : "w"
            if a.count > 1, case .n(let flags, _) = a[1], flags & 1 != 0 { modo = "a" }
            let v = try abreArchivo(nombre, modo, linea)
            if case .canal = v { return v }
            let c = CCanal(n == "ifstream" ? .archivoEntrada : .archivoSalida)
            c.fallo = true
            c.abierto = false
            return .canal(c)
        default:
            break
        }
        guard case .l(let l) = try nuevoContenedor(n, args) else {
            let v = try nuevoContenedor(n, args)
            if case .m(let mp) = v, a.count == 2, case .it = a[0] {
                for x in try valoresRango(a[0], a[1], linea) { mp.pon(try clave(x, linea), x, x) }
            } else if case .m(let mp) = v, a.count == 1, case .m(let otro) = a[0] {
                return copia(.m(otro)).conTipo(mp)
            }
            return v
        }
        if a.isEmpty { return .l(l) }
        if a.count == 1, case .l(let otra) = a[0] { l.a = otra.a.map { copia($0) }; return .l(l) }
        if a.count == 1, case .f(let cmp) = a[0] { l.cmp = cmp; return .l(l) }
        if a.count == 1, case .o(let o) = a[0], n == "priority_queue" { l.cmp = cierreDe(.o(o)); return .l(l) }
        if a.count == 2, (a[0].esIterador || a[0].esPuntero) {
            l.a = try valoresRango(a[0], a[1], linea).map { copia($0) }
            if n == "priority_queue" { try reordenaCola(l, linea) }
            return .l(l)
        }
        let cuantos = Int(try entero(a[0], linea))
        if cuantos < 0 || cuantos > 50_000_000 { throw CFallo("tamaño no válido: \(cuantos)", linea) }
        let valor: CV = a.count > 1 ? convierte(a[1], l.elem ?? .auto) : try valorInicialT(l.elem ?? .num(.int))
        l.a.reserveCapacity(cuantos)
        for _ in 0..<cuantos { l.a.append(copia(valor)) }
        return .l(l)
    }

    func cierreDe(_ v: CV) -> CCierre {
        if case .f(let c) = v { return c }
        return CCierre(fn: nil, marco: nil, este: nil, nombre: "cmp", nat: { m, x in try m.invoca(v, x, 0) })
    }

    func comparadorDeTipo(_ t: CTipo, _ l: CLista) throws {
        switch t {
        case .cont(let n, _) where n.hasPrefix("greater"):
            l.max = false
        case .clase(let n):
            if let c = clases[n] {
                let o = try nuevoObjeto(c, [], 0)
                l.cmp = cierreDe(.o(o))
            }
        default:
            break
        }
    }

    // MARK: rangos de iteradores

    func rango(_ a: CV, _ b: CV, _ linea: Int) throws -> CRango {
        switch (a, b) {
        case (.it(let x, let i), .it(_, let j)):
            if let l = x.lista { return CRango(lista: l, mem: nil, cad: nil, lo: i, hi: j, reves: x.reves) }
            if let c = x.cad { return CRango(lista: nil, mem: nil, cad: c, lo: i, hi: j, reves: x.reves) }
            if let m = x.mem { return CRango(lista: nil, mem: m, cad: nil, lo: i, hi: j, reves: x.reves) }
            if let mp = x.mapa {
                let vals = try (i..<max(i, j)).map { try leeIterador(x, $0, linea) }
                _ = mp
                return CRango(lista: CLista("vector", vals), mem: nil, cad: nil, lo: 0, hi: vals.count, reves: false)
            }
        case (.p(let p), .p(let q)):
            guard let mem = p.mem else { throw CFallo("rango con puntero NULL", linea) }
            try prepara(mem, nil, linea)
            return CRango(lista: nil, mem: mem, cad: nil, lo: p.i, hi: q.i, reves: false)
        default:
            break
        }
        throw CFallo("se esperaban dos iteradores (v.begin(), v.end()) o dos punteros", linea)
    }

    func valoresDe(_ r: CRango) throws -> [CV] {
        var vals: [CV]
        if let l = r.lista {
            let n = l.a.count
            if r.reves {
                vals = (r.lo..<max(r.lo, r.hi)).map { l.a[n - 1 - $0] }
            } else {
                guard r.lo >= 0, r.hi <= n, r.lo <= r.hi else { throw CFallo("rango de iteradores no válido", 0) }
                vals = Array(l.a[r.lo..<r.hi])
            }
        } else if let m = r.mem {
            guard r.lo >= 0, r.hi <= m.a.count, r.lo <= r.hi else { throw CFallo("rango fuera del arreglo", 0) }
            vals = Array(m.a[r.lo..<r.hi])
        } else if let c = r.cad, case .s(let u) = try lee(c.l) {
            let n = u.count
            if r.reves { vals = (r.lo..<max(r.lo, r.hi)).map { caracter(u[n - 1 - $0]) } }
            else {
                guard r.lo >= 0, r.hi <= n, r.lo <= r.hi else { throw CFallo("rango fuera del string", 0) }
                vals = u[r.lo..<r.hi].map { caracter($0) }
            }
        } else {
            vals = []
        }
        return vals
    }

    func guarda(_ vals: [CV], en r: CRango) throws {
        if let l = r.lista {
            let n = l.a.count
            for (k, v) in vals.enumerated() {
                let idx = r.reves ? n - 1 - (r.lo + k) : r.lo + k
                if idx >= 0 && idx < n { l.a[idx] = v }
            }
        } else if let m = r.mem {
            for (k, v) in vals.enumerated() where r.lo + k < m.a.count { m.a[r.lo + k] = v }
        } else if let c = r.cad, case .s(var u) = try lee(c.l) {
            let n = u.count
            for (k, v) in vals.enumerated() {
                let idx = r.reves ? n - 1 - (r.lo + k) : r.lo + k
                if idx >= 0 && idx < n { u[idx] = UInt16(UInt8(truncatingIfNeeded: try entero(v, 0))) }
            }
            try escribe(c.l, .s(u))
        }
    }

    func iterador(_ r: CRango, _ pos: Int) -> CV {
        if let l = r.lista { return .it(CIterBase(lista: l, reves: r.reves), pos) }
        if let c = r.cad { return .it(CIterBase(cad: c, reves: r.reves), pos) }
        if let m = r.mem { return .p(CPtr(mem: m, i: pos)) }
        return .n(Int64(pos), .long)
    }

    func valoresRango(_ a: CV, _ b: CV, _ linea: Int) throws -> [CV] {
        try valoresDe(try rango(a, b, linea))
    }

    func menor(_ cmp: CV?, _ linea: Int) -> (CV, CV) throws -> Bool {
        if let c = cmp {
            return { x, y in self.aBool(try self.invoca(c, [x, y], linea)) }
        }
        return { x, y in try self.compara(x, y, linea) < 0 }
    }

    // MARK: algoritmos

    func nativaCpp(_ n: String, _ a: [CV], _ l: [CLugar?], _ linea: Int) throws -> CV? {
        switch n {
        case "max", "min":
            return try maxMin(n == "max", a, linea)
        case "swap", "iter_swap":
            guard l.count >= 2, let x = l[0], let y = l[1] else {
                if case .it(let b1, let i) = try arg(a, 0, n, linea), case .it(let b2, let j) = try arg(a, 1, n, linea) {
                    let lx = try lugarIterador(b1, i, linea)
                    let ly = try lugarIterador(b2, j, linea)
                    let t = try lee(lx)
                    try escribe(lx, try lee(ly))
                    try escribe(ly, t)
                    return .vacio
                }
                throw CFallo("swap necesita dos variables", linea)
            }
            let t = try lee(x)
            try escribe(x, try lee(y))
            try escribe(y, t)
            return .vacio
        case "getline":
            return try getline(a, l, linea)
        case "sort", "stable_sort", "partial_sort", "nth_element":
            let fin = n == "partial_sort" || n == "nth_element" ? 2 : 1
            let r = try rango(try arg(a, 0, n, linea), try arg(a, fin, n, linea), linea)
            var vals = try valoresDe(r)
            let cmpIdx = fin + 1
            try ordena(&vals, menor(a.count > cmpIdx ? try arg(a, cmpIdx, n, linea) : nil, linea))
            try guarda(vals, en: r)
            return .vacio
        case "reverse":
            let r = try rango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            try guarda(try valoresDe(r).reversed(), en: r)
            return .vacio
        case "fill":
            let r = try rango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            let v = try arg(a, 2, n, linea)
            try guarda([CV](repeating: v, count: max(0, r.hi - r.lo)).map { copia($0) }, en: r)
            return .vacio
        case "fill_n":
            let ini = try arg(a, 0, n, linea)
            let k = Int(try argEnt(a, 1, n, linea))
            let fin: CV
            switch ini {
            case .p(let p): fin = .p(CPtr(mem: p.mem, i: p.i + k))
            case .it(let b, let i): fin = .it(b, i + k)
            default: throw CFallo("fill_n necesita un puntero o iterador", linea)
            }
            let r = try rango(ini, fin, linea)
            try guarda([CV](repeating: try arg(a, 2, n, linea), count: k), en: r)
            return .vacio
        case "iota":
            let r = try rango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            var v = try arg(a, 2, n, linea)
            var vals: [CV] = []
            for _ in r.lo..<max(r.lo, r.hi) {
                vals.append(v)
                v = try binaria(.suma, v, .n(1, .int), linea)
            }
            try guarda(vals, en: r)
            return .vacio
        case "accumulate", "inner_product":
            return try acumula(n, a, linea)
        case "partial_sum":
            let r = try rango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            var vals = try valoresDe(r)
            for k in 1..<max(1, vals.count) { vals[k] = try binaria(.suma, vals[k - 1], vals[k], linea) }
            let dest = try rango(try arg(a, 2, n, linea), try arg(a, 2, n, linea), linea)
            try guarda(vals, en: dest)
            return .vacio
        default:
            return try nativaCpp2(n, a, l, linea)
        }
    }

    func maxMin(_ esMax: Bool, _ a: [CV], _ linea: Int) throws -> CV {
        var vals: [CV]
        var cmp: CV?
        if case .l(let lista)? = a.first, a.count <= 2 {
            vals = lista.a
            if a.count == 2 { cmp = try arg(a, 1, "max", linea) }
        } else {
            vals = [try arg(a, 0, "max", linea), try arg(a, 1, "max", linea)]
            if a.count > 2 { cmp = try arg(a, 2, "max", linea) }
        }
        guard var mejor = vals.first else { throw CFallo("max/min de una lista vacía", linea) }
        let m = menor(cmp, linea)
        for v in vals.dropFirst() {
            if esMax { if try m(mejor, v) { mejor = v } }
            else { if try m(v, mejor) { mejor = v } }
        }
        // max(int, long long) no compila en C++ real; aquí se da el tipo más ancho
        return mejor
    }

    func acumula(_ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        let vals = try valoresRango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
        if n == "inner_product" {
            let otros = try valoresRango(try arg(a, 2, n, linea), .vacio.siguienteDe(try arg(a, 2, n, linea), vals.count), linea)
            var acc = try arg(a, 3, n, linea)
            for k in 0..<min(vals.count, otros.count) {
                acc = try binaria(.suma, acc, try binaria(.mult, vals[k], otros[k], linea), linea)
            }
            return acc
        }
        var acc = try arg(a, 2, n, linea)
        let op: CV? = a.count > 3 ? try arg(a, 3, n, linea) : nil
        for v in vals {
            if let f = op { acc = try invoca(f, [acc, v], linea) }
            else {
                let suma = try binaria(.suma, acc, v, linea)
                // el resultado queda del tipo del valor inicial (accumulate(v, 0) → int)
                if case .n(_, let k) = acc { acc = convierte(suma, .num(k)) }
                else if case .d(_, let fl) = acc { acc = convierte(suma, .real(fl)) }
                else { acc = suma }
            }
        }
        return acc
    }

    func nativaCpp2(_ n: String, _ a: [CV], _ l: [CLugar?], _ linea: Int) throws -> CV? {
        switch n {
        case "count", "count_if":
            let vals = try valoresRango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            let objetivo = try arg(a, 2, n, linea)
            var k = 0
            for v in vals {
                if n == "count" { if try iguales(v, objetivo, linea) { k += 1 } }
                else if aBool(try invoca(objetivo, [v], linea)) { k += 1 }
            }
            return .n(Int64(k), .long)
        case "find", "find_if":
            let r = try rango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            let vals = try valoresDe(r)
            let objetivo = try arg(a, 2, n, linea)
            for (k, v) in vals.enumerated() {
                let ok = n == "find" ? try iguales(v, objetivo, linea) : aBool(try invoca(objetivo, [v], linea))
                if ok { return iterador(r, r.lo + k) }
            }
            return try arg(a, 1, n, linea)
        case "max_element", "min_element":
            let r = try rango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            let vals = try valoresDe(r)
            guard !vals.isEmpty else { return try arg(a, 1, n, linea) }
            let m = menor(a.count > 2 ? try arg(a, 2, n, linea) : nil, linea)
            var mejor = 0
            for k in 1..<vals.count {
                if n == "max_element" { if try m(vals[mejor], vals[k]) { mejor = k } }
                else { if try m(vals[k], vals[mejor]) { mejor = k } }
            }
            return iterador(r, r.lo + mejor)
        case "unique":
            let r = try rango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            let vals = try valoresDe(r)
            var out: [CV] = []
            for v in vals {
                if let u = out.last, try iguales(u, v, linea) { continue }
                out.append(v)
            }
            try guarda(out, en: r)
            return iterador(r, r.lo + out.count)
        case "remove", "remove_if":
            let r = try rango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            let vals = try valoresDe(r)
            let objetivo = try arg(a, 2, n, linea)
            var out: [CV] = []
            for v in vals {
                let quitar = n == "remove" ? try iguales(v, objetivo, linea) : aBool(try invoca(objetivo, [v], linea))
                if !quitar { out.append(v) }
            }
            try guarda(out, en: r)
            return iterador(r, r.lo + out.count)
        case "replace":
            let r = try rango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            let viejo = try arg(a, 2, n, linea)
            let nuevo = try arg(a, 3, n, linea)
            var vals = try valoresDe(r)
            for k in 0..<vals.count where try iguales(vals[k], viejo, linea) { vals[k] = nuevo }
            try guarda(vals, en: r)
            return .vacio
        case "rotate":
            let r = try rango(try arg(a, 0, n, linea), try arg(a, 2, n, linea), linea)
            let medio = try rango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            let vals = try valoresDe(r)
            let k = medio.hi - medio.lo
            try guarda(Array(vals[k...] + vals[..<k]), en: r)
            return iterador(r, r.lo + vals.count - k)
        case "lower_bound", "upper_bound", "binary_search":
            let r = try rango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            let vals = try valoresDe(r)
            let v = try arg(a, 2, n, linea)
            let m = menor(a.count > 3 ? try arg(a, 3, n, linea) : nil, linea)
            var lo = 0
            var hi = vals.count
            while lo < hi {
                let mid = (lo + hi) / 2
                let ir: Bool = n == "upper_bound" ? !(try m(v, vals[mid])) : try m(vals[mid], v)
                if ir { lo = mid + 1 } else { hi = mid }
            }
            if n == "binary_search" {
                let ok = lo < vals.count ? !(try m(v, vals[lo])) : false
                return verdad(ok)
            }
            return iterador(r, r.lo + lo)
        case "next_permutation", "prev_permutation":
            let r = try rango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            var vals = try valoresDe(r)
            let ok = try permuta(&vals, siguiente: n == "next_permutation", linea)
            try guarda(vals, en: r)
            return verdad(ok)
        case "is_sorted":
            let vals = try valoresRango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            for k in 1..<max(1, vals.count) where try compara(vals[k], vals[k - 1], linea) < 0 { return verdad(false) }
            return verdad(true)
        case "distance":
            let x = try arg(a, 0, n, linea)
            let y = try arg(a, 1, n, linea)
            switch (x, y) {
            case (.it(_, let i), .it(_, let j)): return .n(Int64(j - i), .long)
            case (.p(let p), .p(let q)): return .n(Int64(q.i - p.i), .long)
            default: throw CFallo("distance necesita dos iteradores", linea)
            }
        case "next", "prev":
            let x = try arg(a, 0, n, linea)
            let k = a.count > 1 ? Int(try argEnt(a, 1, n, linea)) : 1
            let d = n == "next" ? k : -k
            switch x {
            case .it(let b, let i): return .it(b, i + d)
            case .p(let p): return .p(CPtr(mem: p.mem, i: p.i + d))
            default: throw CFallo("\(n) necesita un iterador", linea)
            }
        case "advance":
            guard let lg = l.first ?? nil else { return .vacio }
            let k = Int(try argEnt(a, 1, n, linea))
            if case .it(let b, let i) = try lee(lg) { try escribe(lg, .it(b, i + k)) }
            return .vacio
        default:
            return try nativaCpp3(n, a, l, linea)
        }
    }

    func permuta(_ v: inout [CV], siguiente: Bool, _ linea: Int) throws -> Bool {
        guard v.count > 1 else { return false }
        func antes(_ x: CV, _ y: CV) throws -> Bool {
            let c = try compara(x, y, linea)
            return siguiente ? c < 0 : c > 0
        }
        var i = v.count - 2
        while i >= 0, !(try antes(v[i], v[i + 1])) { i -= 1 }
        if i < 0 { v.reverse(); return false }
        var j = v.count - 1
        while !(try antes(v[i], v[j])) { j -= 1 }
        v.swapAt(i, j)
        v[(i + 1)...].reverse()
        return true
    }

    func nativaCpp3(_ n: String, _ a: [CV], _ l: [CLugar?], _ linea: Int) throws -> CV? {
        switch n {
        case "transform":
            let r = try rango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            let vals = try valoresDe(r)
            var out: [CV] = []
            if a.count == 5 {
                let otros = try valoresDe(try rango(try arg(a, 2, n, linea), .vacio.siguienteDe(try arg(a, 2, n, linea), vals.count), linea))
                let f = try arg(a, 4, n, linea)
                for k in 0..<vals.count { out.append(try invoca(f, [vals[k], otros[k]], linea)) }
                let d = try arg(a, 3, n, linea)
                try guarda(out, en: try rango(d, .vacio.siguienteDe(d, out.count), linea))
                return .vacio
            }
            let f = try arg(a, 3, n, linea)
            for v in vals { out.append(try invoca(f, [v], linea)) }
            let d = try arg(a, 2, n, linea)
            // back_inserter no está: el destino tiene que tener sitio
            let dr = try rango(d, .vacio.siguienteDe(d, out.count), linea)
            if case .s = d.base { _ = dr }
            try guarda(out.map { convierteComo($0, vals.first) }, en: dr)
            return .vacio
        case "for_each":
            let r = try rango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            let f = try arg(a, 2, n, linea)
            for k in r.lo..<max(r.lo, r.hi) {
                let lg: CLugar
                switch iterador(r, k) {
                case .it(let b, let i): lg = try lugarIterador(b, i, linea)
                case .p(let p): lg = .celda(p.mem!, p.i)
                default: continue
                }
                _ = try invoca(f, [.ref(CRefCaja(lg))], linea)
            }
            return .vacio
        case "all_of", "any_of", "none_of":
            let vals = try valoresRango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            let f = try arg(a, 2, n, linea)
            var algun = false
            var todos = true
            for v in vals {
                if aBool(try invoca(f, [v], linea)) { algun = true } else { todos = false }
            }
            return verdad(n == "all_of" ? todos : (n == "any_of" ? algun : !algun))
        case "copy", "reverse_copy":
            var vals = try valoresRango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            if n == "reverse_copy" { vals.reverse() }
            let d = try arg(a, 2, n, linea)
            try guarda(vals.map { copia($0) }, en: try rango(d, .vacio.siguienteDe(d, vals.count), linea))
            return .vacio.siguienteDe(d, vals.count)
        case "equal":
            let vals = try valoresRango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            let d = try arg(a, 2, n, linea)
            let otros = try valoresRango(d, .vacio.siguienteDe(d, vals.count), linea)
            for k in 0..<vals.count where !(try iguales(vals[k], otros[k], linea)) { return verdad(false) }
            return verdad(true)
        case "shuffle", "random_shuffle":
            let r = try rango(try arg(a, 0, n, linea), try arg(a, 1, n, linea), linea)
            var vals = try valoresDe(r)
            for k in stride(from: vals.count - 1, to: 0, by: -1) {
                let j = Int(rand.siguiente() % Int64(k + 1))
                vals.swapAt(k, j)
            }
            try guarda(vals, en: r)
            return .vacio
        case "begin", "end":
            let c = try arg(a, 0, n, linea)
            let fin = n == "end"
            switch c {
            case .l(let lista): return .it(CIterBase(lista: lista), fin ? lista.a.count : 0)
            case .p(let p): return .p(CPtr(mem: p.mem, i: fin ? (p.mem?.a.count ?? 0) : p.i))
            case .m(let mp): return .it(CIterBase(mapa: mp), fin ? mp.claves.count : 0)
            default:
                if let lg = l.first ?? nil, case .s(let u) = c { return .it(CIterBase(cad: CRefCaja(lg)), fin ? u.count : 0) }
                throw CFallo("\(n)() no sirve con \(c.tipoNombre)", linea)
            }
        case "to_string":
            let v = try arg(a, 0, n, linea)
            if case .d(let d, _) = v { return cadena(String(format: "%f", d)) }
            return cadena(try texto(v, linea))
        case "stoi", "stol", "stoll", "stoul", "stoull":
            let s = try cadenaC(try arg(a, 0, n, linea), linea)
            let base = a.count > 2 ? Int(try argEnt(a, 2, n, linea)) : 10
            let (v, usados) = CMaq.prefijoEntero(s, base)
            if usados == 0 { throw excepcion("invalid_argument", "stoi: no es un número: \"\(s)\"", linea) }
            if n == "stoi" && (v > Int64(Int32.max) || v < Int64(Int32.min)) { throw excepcion("out_of_range", "stoi: número demasiado grande", linea) }
            return n == "stoi" ? .n(v, .int) : .n(v, n.contains("u") ? .ulong : .long)
        case "stod", "stof":
            let s = try cadenaC(try arg(a, 0, n, linea), linea)
            let (v, usados) = CMaq.prefijoReal(s)
            if usados == 0 { throw excepcion("invalid_argument", "stod: no es un número: \"\(s)\"", linea) }
            return .d(n == "stof" ? Double(Float(v)) : v, n == "stof")
        case "make_pair":
            return par(copia(try arg(a, 0, n, linea)), copia(try arg(a, 1, n, linea)))
        case "make_tuple", "tie":
            return tupla(try a.map { x -> CV in if case .ref(let r) = x { return copia(try lee(r.l)) }; return copia(x) })
        case "$get0", "$get1", "$get2", "$get3":
            let k = Int(String(n.last!))!
            guard case .o(let o) = try arg(a, 0, n, linea), k < o.c.a.count else { throw CFallo("get<\(k)> fuera de la tupla", linea) }
            return o.c.a[k]
        case "__gcd", "gcd", "lcm":
            var x = abs(try argEnt(a, 0, n, linea))
            var y = abs(try argEnt(a, 1, n, linea))
            let p = x
            let q = y
            while y != 0 { (x, y) = (y, x % y) }
            if n == "lcm" { return .n(x == 0 ? 0 : p / x * q, .long) }
            return .n(x, .long).conTipoDe(try arg(a, 0, n, linea))
        case "setw", "setprecision", "setfill":
            return manipulador(n, Int(try argEnt(a, 0, n, linea)))
        case "minmax":
            let x = try arg(a, 0, n, linea)
            let y = try arg(a, 1, n, linea)
            return try compara(y, x, linea) < 0 ? par(y, x) : par(x, y)
        case "clamp":
            let v = try arg(a, 0, n, linea)
            let lo = try arg(a, 1, n, linea)
            let hi = try arg(a, 2, n, linea)
            if try compara(v, lo, linea) < 0 { return lo }
            if try compara(hi, v, linea) < 0 { return hi }
            return v
        case "abs":
            let v = try arg(a, 0, n, linea)
            switch v {
            case .d(let d, let f): return .d(Swift.abs(d), f)
            case .n(let x, let k): let r = CNum.comun(k, .int); return .n(cAjusta(x < 0 ? 0 &- x : x, r), r)
            default: throw CFallo("abs necesita un número", linea)
            }
        case "chrono.milliseconds": return .d(try argReal(a, 0, n, linea) / 1000, false)
        case "chrono.seconds": return .d(try argReal(a, 0, n, linea), false)
        case "this_thread.sleep_for":
            vuelca()
            try duerme(try argReal(a, 0, n, linea))
            return .vacio
        case "ios.sync_with_stdio", "ios_base.sync_with_stdio":
            return .verdadero
        case "move":
            return try arg(a, 0, n, linea)
        case "exit", "quick_exit":
            throw CSalida(codigo: Int(try argEnt(a, 0, n, linea)))
        default:
            return nil
        }
    }

    func convierteComo(_ v: CV, _ modelo: CV?) -> CV {
        guard let m = modelo else { return v }
        if case .n(_, let k) = m { return convierte(v, .num(k)) }
        return v
    }
}

extension CV {
    var esIterador: Bool { if case .it = self { return true }; return false }
    var esPuntero: Bool { if case .p = self { return true }; return false }
    var base: CV { self }

    /// El iterador o puntero n posiciones después de x.
    func siguienteDe(_ x: CV, _ n: Int) -> CV {
        switch x {
        case .it(let b, let i): return .it(b, i + n)
        case .p(let p): return .p(CPtr(mem: p.mem, i: p.i + n))
        default: return x
        }
    }

    func conTipoDe(_ modelo: CV) -> CV {
        if case .n(let x, _) = self, case .n(_, let k) = modelo { return .n(cAjusta(x, CNum.comun(k, .int)), CNum.comun(k, .int)) }
        return self
    }

    func conTipo(_ mp: CMapa) -> CV { self }
}
