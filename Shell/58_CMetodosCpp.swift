import Foundation

// ============================================================
// MARK: - C++: métodos de string, vector, map, set y streams
// ============================================================

extension CMaq {

    func metodoNativo(_ base: CV, _ lugar: CLugar?, _ n: String, _ a: [CV], _ la: [CLugar?], _ linea: Int, quiereLugar: Bool) throws -> CV {
        var args = a
        for k in 0..<args.count { if case .ref(let r) = args[k], !CMaq.metodosConLugar.contains(n) { args[k] = try lee(r.l) } }
        switch base {
        case .s(let u):
            if dialecto == .java { return try metodoCadenaJava(u, n, args, linea) }
            return try metodoCadenaCpp(u, lugar, n, args, la, linea, quiereLugar)
        case .l(let l):
            if dialecto == .java { return try metodoListaJava(l, n, args, linea) }
            return try metodoListaCpp(l, n, args, linea, quiereLugar)
        case .m(let mp):
            if dialecto == .java { return try metodoMapaJava(mp, n, args, linea) }
            return try metodoMapaCpp(mp, n, args, linea, quiereLugar)
        case .canal(let c):
            if dialecto == .java { return try metodoCanalJava(c, n, args, linea) }
            return try metodoCanalCpp(c, n, args, la, linea)
        case .it(let b, let i):
            return try metodoIterador(b, i, lugar, n, linea)
        case .n, .d:
            return try metodoNumero(base, n, args, linea)
        case .p(let p):
            return try metodoArreglo(p, n, args, linea)
        default:
            throw CFallo("\(base.tipoNombre) no tiene el método \(n)()", linea)
        }
    }

    func npos() -> CV { .n(-1, .ulong) }

    func tamanoCpp(_ n: Int) -> CV { .n(Int64(n), .ulong) }

    func escribeCadena(_ lugar: CLugar?, _ u: [UInt16], _ linea: Int) throws {
        guard let l = lugar else { throw CFallo("no se puede modificar un string temporal", linea) }
        try escribe(l, .s(u))
    }

    // MARK: std::string

    func metodoCadenaCpp(_ u: [UInt16], _ lugar: CLugar?, _ n: String, _ a: [CV], _ la: [CLugar?], _ linea: Int, _ quiereLugar: Bool) throws -> CV {
        switch n {
        case "size", "length": return tamanoCpp(u.count)
        case "empty": return verdad(u.isEmpty)
        case "c_str", "data": return nuevaCadenaC(texto(u))
        case "substr":
            let pos = a.isEmpty ? 0 : Int(try argEnt(a, 0, n, linea))
            if pos > u.count || pos < 0 { throw excepcion("out_of_range", "basic_string::substr: __pos (which is \(pos)) > this->size() (which is \(u.count))", linea) }
            var len = a.count > 1 ? try argEnt(a, 1, n, linea) : -1
            if len < 0 || len > Int64(u.count - pos) { len = Int64(u.count - pos) }
            return .s(Array(u[pos..<(pos + Int(len))]))
        case "find", "rfind":
            let aguja = try unidadesDe(try arg(a, 0, n, linea), linea)
            if n == "find" {
                let desde = a.count > 1 ? Int(try argEnt(a, 1, n, linea)) : 0
                guard let k = CMaq.busca(aguja, en: u, desde: desde) else { return npos() }
                return tamanoCpp(k)
            }
            var desde = a.count > 1 ? Int(try argEnt(a, 1, n, linea)) : u.count
            if desde < 0 { desde = u.count }
            var k = min(desde, u.count - aguja.count)
            while k >= 0 {
                if Array(u[k..<(k + aguja.count)]) == aguja { return tamanoCpp(k) }
                k -= 1
            }
            return npos()
        case "find_first_of", "find_last_of", "find_first_not_of", "find_last_not_of":
            let set = Set(try unidadesDe(try arg(a, 0, n, linea), linea))
            let dentro = !n.contains("not")
            let indices: [Int] = n.contains("first") ? Array(0..<u.count) : Array((0..<u.count).reversed())
            let desde = a.count > 1 ? Int(try argEnt(a, 1, n, linea)) : (n.contains("first") ? 0 : u.count)
            for k in indices where (n.contains("first") ? k >= desde : k <= desde) && set.contains(u[k]) == dentro {
                return tamanoCpp(k)
            }
            return npos()
        case "compare":
            let otro = try unidadesDe(try arg(a, a.count == 3 ? 2 : 0, n, linea), linea)
            var mio = u
            if a.count == 3 {
                let pos = Int(try argEnt(a, 0, n, linea))
                let len = Int(try argEnt(a, 1, n, linea))
                mio = Array(u.dropFirst(pos).prefix(len))
            }
            if mio == otro { return .n(0, .int) }
            return .n(mio.lexicographicallyPrecedes(otro) ? -1 : 1, .int)
        case "at", "front", "back":
            var k = 0
            if n == "at" { k = Int(try argEnt(a, 0, n, linea)) }
            if n == "back" { k = u.count - 1 }
            guard k >= 0 && k < u.count else {
                if n == "at" { throw excepcion("out_of_range", "basic_string::at: __n (which is \(k)) >= this->size() (which is \(u.count))", linea) }
                throw CFallo("\(n)() de un string vacío", linea)
            }
            if quiereLugar, let l = lugar { return .ref(CRefCaja(.car(l, k))) }
            return caracter(u[k])
        case "begin", "end", "rbegin", "rend", "cbegin", "cend":
            guard let l = lugar else {
                let mem = CMem(u.map { caracter($0) })
                return .it(CIterBase(mem: mem), n.contains("end") ? u.count : 0)
            }
            return .it(CIterBase(cad: CRefCaja(l), reves: n.hasPrefix("r")), n.hasSuffix("end") ? u.count : 0)
        default:
            return try metodoCadenaCppMuta(u, lugar, n, a, la, linea)
        }
    }

    func metodoCadenaCppMuta(_ u: [UInt16], _ lugar: CLugar?, _ n: String, _ a: [CV], _ la: [CLugar?], _ linea: Int) throws -> CV {
        var s = u
        switch n {
        case "push_back":
            s.append(UInt16(UInt8(truncatingIfNeeded: try argEnt(a, 0, n, linea))))
        case "pop_back":
            if s.isEmpty { throw CFallo("pop_back() de un string vacío", linea) }
            s.removeLast()
        case "append", "operator+=":
            if a.count == 2, case .n(let cuantos, _) = try arg(a, 0, n, linea) {
                let ch = UInt16(UInt8(truncatingIfNeeded: try argEnt(a, 1, n, linea)))
                s += [UInt16](repeating: ch, count: Int(cuantos))
            } else {
                var extra = try unidadesDe(try arg(a, 0, n, linea), linea)
                if a.count == 2 { extra = Array(extra.prefix(Int(try argEnt(a, 1, n, linea)))) }
                s += extra
            }
        case "insert":
            let p0 = try arg(a, 0, n, linea)
            var pos: Int
            if case .it(_, let i) = p0 { pos = i } else { pos = Int(try entero(p0, linea)) }
            guard pos >= 0 && pos <= s.count else { throw excepcion("out_of_range", "basic_string::insert", linea) }
            var extra: [UInt16]
            if a.count == 3 {
                let ch = UInt16(UInt8(truncatingIfNeeded: try argEnt(a, 2, n, linea)))
                extra = [UInt16](repeating: ch, count: Int(try argEnt(a, 1, n, linea)))
            } else {
                extra = try unidadesDe(try arg(a, 1, n, linea), linea)
            }
            s.insert(contentsOf: extra, at: pos)
        case "erase":
            if a.isEmpty { s = [] }
            else if case .it(_, let i) = try arg(a, 0, n, linea) {
                var j = i + 1
                if a.count > 1, case .it(_, let k) = try arg(a, 1, n, linea) { j = k }
                guard i >= 0 && j <= s.count && i <= j else { throw CFallo("erase fuera del string", linea) }
                s.removeSubrange(i..<j)
                try escribeCadena(lugar, s, linea)
                return .it(CIterBase(cad: lugar.map { CRefCaja($0) }), i)
            } else {
                let pos = Int(try argEnt(a, 0, n, linea))
                guard pos >= 0 && pos <= s.count else { throw excepcion("out_of_range", "basic_string::erase", linea) }
                var len = a.count > 1 ? Int(try argEnt(a, 1, n, linea)) : s.count - pos
                if len < 0 || len > s.count - pos { len = s.count - pos }
                s.removeSubrange(pos..<(pos + len))
            }
        case "clear":
            s = []
        case "replace":
            let pos = Int(try argEnt(a, 0, n, linea))
            var len = Int(try argEnt(a, 1, n, linea))
            if len > s.count - pos { len = s.count - pos }
            guard pos >= 0 && pos <= s.count else { throw excepcion("out_of_range", "basic_string::replace", linea) }
            let extra = try unidadesDe(try arg(a, 2, n, linea), linea)
            s.replaceSubrange(pos..<(pos + max(0, len)), with: extra)
        case "resize":
            let k = Int(try argEnt(a, 0, n, linea))
            let ch = a.count > 1 ? UInt16(UInt8(truncatingIfNeeded: try argEnt(a, 1, n, linea))) : 0
            if k < s.count { s.removeLast(s.count - k) } else { s += [UInt16](repeating: ch, count: k - s.count) }
        case "assign":
            if case .s = try cadenaTemporal(a, linea), case .s(let x) = try cadenaTemporal(a, linea) { s = x }
        case "swap":
            guard let otro = la.first ?? nil else { throw CFallo("swap necesita una variable", linea) }
            let v = try lee(otro)
            try escribe(otro, .s(s))
            if case .s(let x) = v { s = x }
        case "reserve", "shrink_to_fit":
            return .vacio
        case "capacity", "max_size":
            return tamanoCpp(max(15, s.count))
        default:
            throw CFallo("string no tiene el método \(n)()", linea)
        }
        try escribeCadena(lugar, s, linea)
        return .vacio
    }

    // MARK: vector, deque, list, stack, queue, priority_queue

    func elementoPara(_ l: CLista, _ v: CV) throws -> CV {
        guard let t = l.elem else { return copia(v) }
        if case .cad = t { return try aCadena(v, 0) }
        if case .auto = t { return copia(v) }
        return copia(convierte(v, t))
    }

    func metodoListaCpp(_ l: CLista, _ n: String, _ a: [CV], _ linea: Int, _ quiereLugar: Bool) throws -> CV {
        func ref(_ i: Int) -> CV { quiereLugar ? .ref(CRefCaja(.lista(l, i))) : l.a[i] }
        switch n {
        case "push_back", "push", "emplace_back", "emplace", "push_front", "emplace_front":
            var v: CV
            if n.hasPrefix("emplace") && a.count != 1 || (n.hasPrefix("emplace") && a.count == 1 && l.elem.map { if case .clase = $0 { return true }; return false } == true && !(a[0].esObjeto)) {
                v = try emplaza(l, a, linea)
            } else {
                v = try elementoPara(l, try arg(a, 0, n, linea))
            }
            if l.k == "priority_queue" { try empujaCola(l, v, linea); return .vacio }
            if n.hasSuffix("front") { l.a.insert(v, at: 0) } else { l.a.append(v) }
            return .vacio
        case "pop_back", "pop", "pop_front":
            if l.a.isEmpty { throw CFallo("\(n)() en un \(l.k) vacío (en C++ real esto es un fallo grave)", linea) }
            if l.k == "priority_queue" { _ = try sacaCola(l, linea); return .vacio }
            if n == "pop_front" || l.k == "queue" { l.a.removeFirst() } else { l.a.removeLast() }
            return .vacio
        case "top":
            if l.a.isEmpty { throw CFallo("top() de un \(l.k) vacío", linea) }
            return l.k == "priority_queue" ? l.a[0] : ref(l.a.count - 1)
        case "front":
            if l.a.isEmpty { throw CFallo("front() de un \(l.k) vacío", linea) }
            return ref(0)
        case "back":
            if l.a.isEmpty { throw CFallo("back() de un \(l.k) vacío", linea) }
            return ref(l.a.count - 1)
        case "at":
            let i = Int(try argEnt(a, 0, n, linea))
            guard i >= 0 && i < l.a.count else {
                throw excepcion("out_of_range", "vector::_M_range_check: __n (which is \(i)) >= this->size() (which is \(l.a.count))", linea)
            }
            return ref(i)
        case "size": return tamanoCpp(l.a.count)
        case "empty": return verdad(l.a.isEmpty)
        case "clear": l.a.removeAll(); return .vacio
        case "begin", "end", "rbegin", "rend", "cbegin", "cend", "crbegin", "crend":
            let reves = n.hasPrefix("r") || n.hasPrefix("cr")
            return .it(CIterBase(lista: l, reves: reves), n.hasSuffix("end") ? l.a.count : 0)
        case "resize":
            let k = Int(try argEnt(a, 0, n, linea))
            if k < 0 { throw CFallo("resize con tamaño negativo", linea) }
            if k < l.a.count { l.a.removeLast(l.a.count - k) }
            else {
                let v = a.count > 1 ? try elementoPara(l, try arg(a, 1, n, linea)) : try valorInicialT(l.elem ?? .num(.int))
                for _ in l.a.count..<k { l.a.append(copia(v)) }
            }
            return .vacio
        case "assign":
            if a.count == 2, a[0].esIterador || a[0].esPuntero {
                l.a = try valoresRango(a[0], a[1], linea).map { copia($0) }
            } else {
                let k = Int(try argEnt(a, 0, n, linea))
                let v = try elementoPara(l, try arg(a, 1, n, linea))
                l.a = [CV](repeating: v, count: k).map { copia($0) }
            }
            return .vacio
        case "reserve", "shrink_to_fit": return .vacio
        case "capacity", "max_size": return tamanoCpp(l.a.count)
        case "swap":
            guard case .l(let otra) = try arg(a, 0, n, linea) else { throw CFallo("swap necesita otro contenedor", linea) }
            let t = l.a
            l.a = otra.a
            otra.a = t
            return .vacio
        default:
            return try metodoListaCpp2(l, n, a, linea)
        }
    }

    func metodoListaCpp2(_ l: CLista, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        switch n {
        case "insert", "emplace":
            guard case .it(_, let pos) = try arg(a, 0, n, linea) else { throw CFallo("insert necesita un iterador (v.begin() + i)", linea) }
            guard pos >= 0 && pos <= l.a.count else { throw CFallo("insert fuera del vector", linea) }
            if a.count == 3, a[1].esIterador || a[1].esPuntero {
                l.a.insert(contentsOf: try valoresRango(a[1], a[2], linea).map { copia($0) }, at: pos)
            } else if a.count == 3 {
                let k = Int(try argEnt(a, 1, n, linea))
                let v = try elementoPara(l, try arg(a, 2, n, linea))
                l.a.insert(contentsOf: [CV](repeating: v, count: k), at: pos)
            } else if case .l(let ini) = try arg(a, 1, n, linea), l.elem.map({ if case .cont = $0 { return false }; return true }) ?? true {
                l.a.insert(contentsOf: ini.a, at: pos)
            } else {
                l.a.insert(try elementoPara(l, try arg(a, 1, n, linea)), at: pos)
            }
            return .it(CIterBase(lista: l), pos)
        case "erase":
            guard case .it(_, let i) = try arg(a, 0, n, linea) else { throw CFallo("erase necesita un iterador (v.begin() + i)", linea) }
            var j = i + 1
            if a.count > 1, case .it(_, let k) = try arg(a, 1, n, linea) { j = k }
            guard i >= 0 && j <= l.a.count && i <= j else { throw CFallo("erase fuera del vector", linea) }
            l.a.removeSubrange(i..<j)
            return .it(CIterBase(lista: l), i)
        case "sort":
            var vals = l.a
            try ordena(&vals, menor(a.first, linea))
            l.a = vals
            return .vacio
        case "reverse":
            l.a.reverse()
            return .vacio
        case "unique":
            var out: [CV] = []
            for v in l.a {
                if let u = out.last, try iguales(u, v, linea) { continue }
                out.append(v)
            }
            l.a = out
            return .vacio
        case "remove", "remove_if":
            let objetivo = try arg(a, 0, n, linea)
            var out: [CV] = []
            for v in l.a {
                let quitar = n == "remove" ? try iguales(v, objetivo, linea) : aBool(try invoca(objetivo, [v], linea))
                if !quitar { out.append(v) }
            }
            l.a = out
            return .vacio
        case "data":
            let mem = CMem(l.a)
            return .p(CPtr(mem: mem, i: 0))
        case "count":
            let objetivo = try arg(a, 0, n, linea)
            var k = 0
            for v in l.a where try iguales(v, objetivo, linea) { k += 1 }
            return tamanoCpp(k)
        default:
            throw CFallo("\(l.k) no tiene el método \(n)()", linea)
        }
    }

    func emplaza(_ l: CLista, _ a: [CV], _ linea: Int) throws -> CV {
        switch l.elem {
        case .clase(let cn)?:
            guard let c = clases[cn] else { break }
            return .o(try nuevoObjeto(c, a, linea))
        case .cont("pair", _)?:
            return par(a.count > 0 ? copia(a[0]) : .n(0, .int), a.count > 1 ? copia(a[1]) : .n(0, .int))
        case .cont("tuple", _)?:
            return tupla(a.map { copia($0) })
        case .cad?:
            return try cadenaTemporal(a, linea)
        case .cont(let cn, let args)?:
            return try contenedorTemporal(cn, args, a, linea)
        default:
            break
        }
        if a.count == 2 { return par(copia(a[0]), copia(a[1])) }
        return a.first.map { copia($0) } ?? .vacio
    }

    // MARK: cola de prioridad (montículo como el de C++ y Java)

    /// ¿x va antes (más cerca de la cima) que y?
    func antesEnCola(_ l: CLista, _ x: CV, _ y: CV, _ linea: Int) throws -> Bool {
        if let c = l.cmp {
            if dialecto == .java { return try entero(try llamaCierre(c, [x, y], linea), linea) < 0 }
            // comparador de C++: cmp(a, b) = "a tiene menos prioridad que b"
            return aBool(try llamaCierre(c, [y, x], linea))
        }
        let r = try compara(x, y, linea)
        return l.max ? r > 0 : r < 0
    }

    func empujaCola(_ l: CLista, _ v: CV, _ linea: Int) throws {
        var k = l.a.count
        l.a.append(v)
        while k > 0 {
            let p = (k - 1) >> 1
            if !(try antesEnCola(l, v, l.a[p], linea)) { break }
            l.a[k] = l.a[p]
            k = p
        }
        l.a[k] = v
    }

    func sacaCola(_ l: CLista, _ linea: Int) throws -> CV {
        let tope = l.a[0]
        let x = l.a.removeLast()
        if l.a.isEmpty { return tope }
        try hundeCola(l, 0, x, linea)
        return tope
    }

    func hundeCola(_ l: CLista, _ desde: Int, _ x: CV, _ linea: Int) throws {
        var k = desde
        let n = l.a.count
        let mitad = n >> 1
        while k < mitad {
            var hijo = 2 * k + 1
            var c = l.a[hijo]
            let der = hijo + 1
            if der < n, try antesEnCola(l, l.a[der], c, linea) {
                hijo = der
                c = l.a[der]
            }
            if !(try antesEnCola(l, c, x, linea)) { break }
            l.a[k] = c
            k = hijo
        }
        l.a[k] = x
    }

    func ordenados(_ l: CLista, _ linea: Int) throws -> [CV] {
        // heapify (como PriorityQueue(Collection) de Java)
        let copiaL = CLista(l.k, l.a)
        copiaL.cmp = l.cmp
        copiaL.max = l.max
        var i = (copiaL.a.count >> 1) - 1
        while i >= 0 {
            try hundeCola(copiaL, i, copiaL.a[i], linea)
            i -= 1
        }
        return copiaL.a
    }

    // MARK: map, set

    func metodoMapaCpp(_ mp: CMapa, _ n: String, _ a: [CV], _ linea: Int, _ quiereLugar: Bool) throws -> CV {
        switch n {
        case "insert", "emplace", "try_emplace":
            if a.count == 2 && (a[0].esIterador || a[0].esPuntero) && !mp.esSet || (a.count == 2 && mp.esSet && a[0].esIterador) {
                for v in try valoresRango(a[0], a[1], linea) { try inserta(mp, v, nil, linea) }
                return .vacio
            }
            if case .l(let lista)? = a.first, a.count == 1, !mp.esSet || lista.a.count != 2 {
                for v in lista.a { try inserta(mp, v, nil, linea) }
                return .vacio
            }
            let (idx, nuevo) = try inserta(mp, try arg(a, 0, n, linea), a.count > 1 ? try arg(a, 1, n, linea) : nil, linea)
            return par(.it(CIterBase(mapa: mp), idx), verdad(nuevo))
        case "erase":
            let x = try arg(a, 0, n, linea)
            if case .it(_, let i) = x {
                guard i >= 0 && i < mp.claves.count else { throw CFallo("erase con un iterador no válido", linea) }
                let k = mp.claves[i]
                _ = mp.quita(k)
                return .it(CIterBase(mapa: mp), i)
            }
            let k = try clave(x, linea)
            if mp.multi {
                let c = mp.cuentas[k] ?? 0
                mp.cuentas[k] = 1
                _ = mp.quita(k)
                return tamanoCpp(c)
            }
            return tamanoCpp(mp.quita(k) ? 1 : 0)
        case "find":
            let k = try clave(try arg(a, 0, n, linea), linea)
            guard mp.vals[k] != nil else { return .it(CIterBase(mapa: mp), mp.claves.count) }
            let idx = mp.ordenado ? mp.posicion(k) : (mp.claves.firstIndex(of: k) ?? mp.claves.count)
            return .it(CIterBase(mapa: mp), idx)
        case "count":
            let k = try clave(try arg(a, 0, n, linea), linea)
            if mp.multi { return tamanoCpp(mp.cuentas[k] ?? 0) }
            return tamanoCpp(mp.vals[k] != nil ? 1 : 0)
        case "contains":
            return verdad(mp.vals[try clave(try arg(a, 0, n, linea), linea)] != nil)
        case "size": return tamanoCpp(mp.count)
        case "empty": return verdad(mp.claves.isEmpty)
        case "clear":
            mp.claves = []
            mp.vals = [:]
            mp.cuentas = [:]
            return .vacio
        case "begin", "end", "rbegin", "rend", "cbegin", "cend":
            return .it(CIterBase(mapa: mp, reves: n.hasPrefix("r")), n.hasSuffix("end") ? mp.claves.count : 0)
        case "lower_bound", "upper_bound":
            let k = try clave(try arg(a, 0, n, linea), linea)
            var p = mp.posicion(k)
            if n == "upper_bound" && p < mp.claves.count && mp.claves[p] == k { p += 1 }
            return .it(CIterBase(mapa: mp), p)
        case "at":
            let x = try arg(a, 0, n, linea)
            let k = try clave(x, linea)
            guard let (kv, v) = mp.vals[k] else { throw excepcion("out_of_range", "map::at", linea) }
            return quiereLugar ? .ref(CRefCaja(.mapa(mp, kv))) : v
        case "swap":
            guard case .m(let otro) = try arg(a, 0, n, linea) else { return .vacio }
            let (c, v, cu) = (mp.claves, mp.vals, mp.cuentas)
            (mp.claves, mp.vals, mp.cuentas) = (otro.claves, otro.vals, otro.cuentas)
            (otro.claves, otro.vals, otro.cuentas) = (c, v, cu)
            return .vacio
        default:
            throw CFallo("\(mp.k) no tiene el método \(n)()", linea)
        }
    }

    @discardableResult
    func inserta(_ mp: CMapa, _ x: CV, _ y: CV?, _ linea: Int) throws -> (Int, Bool) {
        var kv: CV
        var vv: CV
        if mp.esSet {
            kv = copia(x)
            vv = kv
        } else if let v = y {
            kv = copia(x)
            vv = copia(v)
        } else if case .o(let o) = x, o.c.a.count >= 2 {
            kv = copia(o.c.a[0])
            vv = copia(o.c.a[1])
            if case .ref(let r) = vv { vv = try lee(r.l) }
        } else if case .l(let l) = x, l.a.count == 2 {
            kv = l.a[0]
            vv = l.a[1]
        } else {
            throw CFallo("insert en un map necesita un par {clave, valor}", linea)
        }
        if case .p = kv, dialecto == .cpp { kv = try aCadena(kv, linea) }
        let k = try clave(kv, linea)
        if mp.vals[k] != nil && !mp.multi {
            let idx = mp.ordenado ? mp.posicion(k) : (mp.claves.firstIndex(of: k) ?? 0)
            return (idx, false)
        }
        mp.pon(k, kv, vv)
        let idx = mp.ordenado ? mp.posicion(k) : (mp.claves.firstIndex(of: k) ?? 0)
        return (idx, true)
    }

    // MARK: streams de C++ (cin.get, ss.str, archivos…)

    func metodoCanalCpp(_ c: CCanal, _ n: String, _ a: [CV], _ la: [CLugar?], _ linea: Int) throws -> CV {
        switch n {
        case "get":
            guard let ent = entradaDe(c) else { return .n(-1, .int) }
            let b = ent.lee()
            if b < 0 { c.fallo = true }
            if let l = la.first ?? nil {
                if b >= 0 { try escribe(l, .n(Int64(Int8(truncatingIfNeeded: b)), .char)) }
                return .canal(c)
            }
            return .n(Int64(b), .int)
        case "peek":
            return .n(Int64(entradaDe(c)?.mira() ?? -1), .int)
        case "unget", "putback":
            entradaDe(c)?.devuelve()
            return .canal(c)
        case "ignore":
            guard let ent = entradaDe(c) else { return .canal(c) }
            let cuantos = a.isEmpty ? 1 : try argEnt(a, 0, n, linea)
            let delim = a.count > 1 ? Int(try argEnt(a, 1, n, linea)) : -1
            var k: Int64 = 0
            while k < cuantos {
                let b = ent.lee()
                if b < 0 || b == delim { break }
                k += 1
            }
            return .canal(c)
        case "getline":
            guard let ent = entradaDe(c) else { return .canal(c) }
            let p = try argPtr(a, 0, n, linea)
            let max = Int(try argEnt(a, 1, n, linea))
            guard let l = ent.linea() else { c.fallo = true; return .canal(c) }
            try escribeC(Array(l.prefix(max - 1)), en: p, linea)
            return .canal(c)
        case "eof":
            guard let ent = entradaDe(c) else { return verdad(true) }
            return verdad(c.fallo || ent.mira() < 0)
        case "fail", "bad": return verdad(c.fallo)
        case "good": return verdad(!c.fallo)
        case "clear":
            c.fallo = false
            return .vacio
        case "str":
            if let s = a.first {
                let txt = try cadenaC(s, linea)
                c.buf = Array(txt.utf8)
                c.ent = CEntrada(txt)
                c.fallo = false
                return .vacio
            }
            return .s(c.buf.map { UInt16($0) })
        case "precision":
            let viejo = c.precision
            if let p = a.first { c.precision = Int(try entero(p, linea)) }
            return .n(Int64(viejo), .long)
        case "width":
            if let w = a.first { c.ancho = Int(try entero(w, linea)) }
            return .n(0, .long)
        case "fill":
            if let f = a.first { c.relleno = UInt8(truncatingIfNeeded: try entero(f, linea)) }
            return .n(Int64(c.relleno), .char)
        case "setf", "unsetf", "tie", "sync_with_stdio", "exceptions", "imbue":
            if let f = a.first, case .f(let m) = f, m.nombre.hasPrefix("manip:") { try aplicaManipulador(c, m.nombre, linea) }
            return .vacio
        case "flush":
            vuelca()
            return .canal(c)
        case "put":
            try escribeCanal(c, [UInt8(truncatingIfNeeded: try argEnt(a, 0, n, linea))])
            return .canal(c)
        case "write":
            let bytes = try bytesDe(try arg(a, 0, n, linea), linea)
            let k = a.count > 1 ? Int(try argEnt(a, 1, n, linea)) : bytes.count
            try escribeCanal(c, Array(bytes.prefix(k)))
            return .canal(c)
        case "open":
            let nombre = try cadenaC(try arg(a, 0, n, linea), linea)
            if c.clase == .archivoEntrada {
                if let b = leerArchivo(nombre) {
                    c.ent = CEntrada(String(decoding: b, as: UTF8.self))
                    c.abierto = true
                    c.fallo = false
                } else { c.fallo = true; c.abierto = false }
            } else {
                c.nombre = nombre
                c.abierto = escribirArchivo(nombre, [], false)
                c.fallo = !c.abierto
                if c.abierto { abiertos.append(c) }
            }
            return .vacio
        case "is_open": return verdad(c.abierto && !c.fallo || (c.abierto && c.clase == .archivoSalida))
        case "close":
            try cierraCanal(c)
            return .vacio
        default:
            throw CFallo("el stream no tiene el método \(n)()", linea)
        }
    }

    // MARK: iteradores (Iterator de Java)

    func metodoIterador(_ b: CIterBase, _ i: Int, _ lugar: CLugar?, _ n: String, _ linea: Int) throws -> CV {
        let total = b.lista?.a.count ?? b.mapa?.claves.count ?? 0
        switch n {
        case "hasNext": return verdad(i < total)
        case "next":
            guard i < total else { throw excepcion("NoSuchElementException", "no hay más elementos", linea) }
            let v = try leeIterador(b, i, linea)
            if let l = lugar { try escribe(l, .it(b, i + 1)) }
            if let mp = b.mapa, mp.esSet == false, dialecto == .java { return entrada(mp, mp.claves[i]) }
            return v
        case "remove":
            guard i > 0 else { throw excepcion("IllegalStateException", "remove() antes de next()", linea) }
            if let l = b.lista { l.a.remove(at: i - 1) }
            if let mp = b.mapa { _ = mp.quita(mp.claves[i - 1]) }
            if let l = lugar { try escribe(l, .it(b, i - 1)) }
            return .vacio
        case "first", "second":
            if let mp = b.mapa, i < mp.claves.count, let (kv, vv) = mp.vals[mp.claves[i]] { return n == "first" ? kv : vv }
            throw CFallo("iterador no válido", linea)
        default:
            throw CFallo("un iterador no tiene el método \(n)()", linea)
        }
    }
}

extension CV {
    var esObjeto: Bool { if case .o = self { return true }; return false }
}
