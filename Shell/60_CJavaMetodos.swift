import Foundation

// ============================================================
// MARK: - Java: métodos de String, colecciones, Scanner…
// ============================================================

extension CMaq {

    func entJ(_ v: Int) -> CV { .n(Int64(v), .int) }

    // MARK: String

    func metodoCadenaJava(_ u: [UInt16], _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        func s(_ k: Int) throws -> [UInt16] { unidades(try texto(try arg(a, k, n, linea), linea)) }
        func fuera(_ i: Int) -> Error {
            excepcion("StringIndexOutOfBoundsException", "index \(i), length \(u.count)", linea)
        }
        switch n {
        case "length": return entJ(u.count)
        case "isEmpty": return verdad(u.isEmpty)
        case "isBlank": return verdad(texto(u).trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
        case "charAt":
            let i = Int(try argEnt(a, 0, n, linea))
            guard i >= 0 && i < u.count else { throw excepcion("StringIndexOutOfBoundsException", "Index \(i) out of bounds for length \(u.count)", linea) }
            return .n(Int64(u[i]), .jchar)
        case "codePointAt":
            let i = Int(try argEnt(a, 0, n, linea))
            guard i >= 0 && i < u.count else { throw fuera(i) }
            return entJ(Int(u[i]))
        case "substring", "subSequence":
            let i = Int(try argEnt(a, 0, n, linea))
            let j = a.count > 1 ? Int(try argEnt(a, 1, n, linea)) : u.count
            guard i >= 0 && j <= u.count && i <= j else {
                throw excepcion("StringIndexOutOfBoundsException", "begin \(i), end \(j), length \(u.count)", linea)
            }
            return .s(Array(u[i..<j]))
        case "indexOf", "lastIndexOf":
            let x = try arg(a, 0, n, linea)
            let aguja: [UInt16] = { if case .n(let c, _) = x { return [UInt16(truncatingIfNeeded: c)] }; return [] }()
            let busca = aguja.isEmpty ? try s(0) : aguja
            if n == "indexOf" {
                let desde = a.count > 1 ? Swift.max(0, Int(try argEnt(a, 1, n, linea))) : 0
                return entJ(CMaq.busca(busca, en: u, desde: desde) ?? -1)
            }
            var k = Swift.min(a.count > 1 ? Int(try argEnt(a, 1, n, linea)) : u.count, u.count - busca.count)
            while k >= 0 {
                if Array(u[k..<(k + busca.count)]) == busca { return entJ(k) }
                k -= 1
            }
            return entJ(-1)
        case "equals":
            let x = try arg(a, 0, n, linea)
            if case .s(let w) = x { return verdad(w == u) }
            return verdad(false)
        case "equalsIgnoreCase":
            let x = try arg(a, 0, n, linea)
            guard case .s(let w) = x else { return verdad(false) }
            return verdad(texto(w).lowercased() == texto(u).lowercased())
        case "compareTo":
            return entJ(try compara(.s(u), .s(try s(0)), linea))
        case "compareToIgnoreCase":
            return entJ(try compara(cadena(texto(u).lowercased()), cadena(texto(try s(0)).lowercased()), linea))
        case "contains": return verdad(CMaq.busca(try s(0), en: u, desde: 0) != nil)
        case "startsWith":
            let p = try s(0)
            let desde = a.count > 1 ? Int(try argEnt(a, 1, n, linea)) : 0
            return verdad(desde >= 0 && desde + p.count <= u.count && Array(u[desde..<(desde + p.count)]) == p)
        case "endsWith":
            let p = try s(0)
            return verdad(p.count <= u.count && Array(u.suffix(p.count)) == p)
        case "toUpperCase": return cadena(texto(u).uppercased())
        case "toLowerCase": return cadena(texto(u).lowercased())
        case "trim":
            var x = u[...]
            while let f = x.first, f <= 32 { x = x.dropFirst() }
            while let l = x.last, l <= 32 { x = x.dropLast() }
            return .s(Array(x))
        case "strip": return cadena(texto(u).trimmingCharacters(in: .whitespacesAndNewlines))
        case "stripLeading": return cadena(String(texto(u).drop { $0.isWhitespace }))
        case "stripTrailing": return cadena(String(String(texto(u).reversed()).drop { $0.isWhitespace }.reversed()))
        case "concat": return .s(u + (try s(0)))
        case "repeat":
            let k = Int(try argEnt(a, 0, n, linea))
            if k < 0 { throw excepcion("IllegalArgumentException", "count is negative: \(k)", linea) }
            return .s(Array([[UInt16]](repeating: u, count: k).joined()))
        case "replace":
            let x = try arg(a, 0, n, linea)
            if case .n(let c1, _) = x {
                let c2 = UInt16(truncatingIfNeeded: try argEnt(a, 1, n, linea))
                return .s(u.map { $0 == UInt16(truncatingIfNeeded: c1) ? c2 : $0 })
            }
            return cadena(texto(u).replacingOccurrences(of: texto(try s(0)), with: texto(try s(1))))
        case "replaceAll", "replaceFirst":
            return cadena(try reemplazaRegex(texto(u), texto(try s(0)), texto(try s(1)), todo: n == "replaceAll", linea))
        case "matches":
            return verdad(try coincideRegex(texto(u), texto(try s(0)), linea))
        case "split":
            return try divide(texto(u), texto(try s(0)), a.count > 1 ? Int(try argEnt(a, 1, n, linea)) : 0, linea)
        case "toCharArray":
            let mem = CMem(u.map { CV.n(Int64($0), .jchar) })
            mem.elem = .num(.jchar)
            mem.esArreglo = true
            return .p(CPtr(mem: mem, i: 0))
        case "chars":
            return .l(CLista("Stream", u.map { CV.n(Int64($0), .int) }))
        case "hashCode": return .n(try hashJava(.s(u), linea), .int)
        case "toString", "intern", "getName", "getSimpleName": return .s(u)
        case "lines":
            return .l(CLista("Stream", texto(u).components(separatedBy: "\n").map { cadena($0) }))
        case "format", "formatted":
            return cadena(try formateaJava(texto(u), a, linea))
        case "getBytes":
            let mem = CMem(Array(texto(u).utf8).map { CV.n(Int64(Int8(bitPattern: $0)), .char) })
            mem.elem = .num(.char)
            return .p(CPtr(mem: mem, i: 0))
        default:
            throw CFallo("String no tiene el método \(n)()", linea)
        }
    }

    func regex(_ patron: String, _ linea: Int) throws -> NSRegularExpression {
        do { return try NSRegularExpression(pattern: patron) }
        catch { throw excepcion("PatternSyntaxException", "expresión regular no válida: \(patron)", linea) }
    }

    func reemplazaRegex(_ s: String, _ patron: String, _ por: String, todo: Bool, _ linea: Int) throws -> String {
        let re = try regex(patron, linea)
        let rango = NSRange(s.startIndex..., in: s)
        let plantilla = por.replacingOccurrences(of: "\\$", with: "\u{1}").replacingOccurrences(of: "$", with: "$").replacingOccurrences(of: "\u{1}", with: "\\$")
        if todo { return re.stringByReplacingMatches(in: s, range: rango, withTemplate: plantilla) }
        guard let m = re.firstMatch(in: s, range: rango), let r = Range(m.range, in: s) else { return s }
        let trozo = re.replacementString(for: m, in: s, offset: 0, template: plantilla)
        return s.replacingCharacters(in: r, with: trozo)
    }

    func coincideRegex(_ s: String, _ patron: String, _ linea: Int) throws -> Bool {
        let re = try regex("^(?:" + patron + ")$", linea)
        return re.firstMatch(in: s, range: NSRange(s.startIndex..., in: s)) != nil
    }

    func divide(_ s: String, _ patron: String, _ limite: Int, _ linea: Int) throws -> CV {
        var partes: [String] = []
        let especiales = Set("\\[](){}.*+?^$|")
        if patron.count == 1, let c = patron.first, !especiales.contains(c) {
            partes = s.components(separatedBy: patron)
        } else {
            let re = try regex(patron, linea)
            var ini = s.startIndex
            for m in re.matches(in: s, range: NSRange(s.startIndex..., in: s)) {
                guard let r = Range(m.range, in: s) else { continue }
                if r.isEmpty && r.lowerBound == s.startIndex { continue }
                if limite > 0 && partes.count == limite - 1 { break }
                partes.append(String(s[ini..<r.lowerBound]))
                ini = r.upperBound
            }
            partes.append(String(s[ini...]))
            if partes.first == "" && !s.isEmpty, let primero = re.firstMatch(in: s, range: NSRange(s.startIndex..., in: s)), primero.range.location == 0, primero.range.length > 0 {
                partes.removeFirst()
            }
        }
        if limite > 0 && partes.count > limite {
            let resto = partes[(limite - 1)...].joined(separator: patron)
            partes = Array(partes.prefix(limite - 1)) + [resto]
        }
        if limite == 0 { while partes.count > 1 && partes.last == "" { partes.removeLast() } }
        if s.isEmpty { partes = [""] }
        let mem = CMem(partes.map { cadena($0) })
        mem.elem = .cad
        mem.esArreglo = true
        return .p(CPtr(mem: mem, i: 0))
    }

    // MARK: listas, StringBuilder, Random, Optional, Stream…

    func indiceJava(_ a: [CV], _ k: Int, _ n: String, _ total: Int, _ linea: Int, incluyeFin: Bool = false) throws -> Int {
        let i = Int(try argEnt(a, k, n, linea))
        if i < 0 || i > total || (i == total && !incluyeFin) {
            throw excepcion("IndexOutOfBoundsException", "Index \(i) out of bounds for length \(total)", linea)
        }
        return i
    }

    func metodoListaJava(_ l: CLista, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        switch l.k {
        case "StringBuilder": return try metodoConstructor(l, n, a, linea)
        case "Random": return try metodoRandom(l, n, a, linea)
        case "Optional": return try metodoOpcional(l, n, a, linea)
        case "Stream": return try metodoStream(l, n, a, linea)
        case "File": return try metodoArchivo(l, n, a, linea)
        case "AtomicInteger": return try metodoAtomico(l, n, a, linea)
        case "Thread":
            if n == "start" || n == "run", let f = l.a.first { _ = try invoca(f, [], linea) }
            return .vacio
        case "PriorityQueue": return try metodoColaJava(l, n, a, linea)
        default: break
        }
        switch n {
        case "add", "addLast", "offer", "offerLast", "push", "addElement", "addFirst", "offerFirst":
            if a.count == 2 {
                let i = try indiceJava(a, 0, n, l.a.count, linea, incluyeFin: true)
                l.a.insert(a[1], at: i)
                return .vacio
            }
            let v = try arg(a, 0, n, linea)
            if n == "addFirst" || n == "offerFirst" || (n == "push" && l.k != "Stack") { l.a.insert(v, at: 0) }
            else { l.a.append(v) }
            return n == "push" ? v : verdad(true)
        case "get", "elementAt":
            let i = try indiceJava(a, 0, n, l.a.count, linea)
            return l.a[i]
        case "set":
            let i = try indiceJava(a, 0, n, l.a.count, linea)
            let viejo = l.a[i]
            l.a[i] = try arg(a, 1, n, linea)
            return viejo
        case "remove":
            if a.isEmpty {
                guard !l.a.isEmpty else { throw excepcion("NoSuchElementException", "lista vacía", linea) }
                return l.a.removeFirst()
            }
            let x = try arg(a, 0, n, linea)
            if case .n(_, let k) = x, k == .int || k == .short || k == .char, l.k != "ArrayDeque" {
                let i = try indiceJava(a, 0, n, l.a.count, linea)
                return l.a.remove(at: i)
            }
            for (k, v) in l.a.enumerated() where try iguales(v, x, linea) {
                l.a.remove(at: k)
                return verdad(true)
            }
            return verdad(false)
        case "size": return entJ(l.a.count)
        case "isEmpty", "empty": return verdad(l.a.isEmpty)
        case "clear", "removeAllElements": l.a.removeAll(); return .vacio
        case "contains":
            let x = try arg(a, 0, n, linea)
            for v in l.a where try iguales(v, x, linea) { return verdad(true) }
            return verdad(false)
        case "indexOf", "lastIndexOf":
            let x = try arg(a, 0, n, linea)
            let indices = n == "indexOf" ? Array(0..<l.a.count) : Array((0..<l.a.count).reversed())
            for k in indices where try iguales(l.a[k], x, linea) { return entJ(k) }
            return entJ(-1)
        case "pop", "removeFirst", "poll", "pollFirst", "removeLast", "pollLast":
            if l.a.isEmpty {
                if n.hasPrefix("poll") { return .nulo }
                throw excepcion(l.k == "Stack" ? "EmptyStackException" : "NoSuchElementException", "", linea)
            }
            if n == "removeLast" || n == "pollLast" || (n == "pop" && l.k == "Stack") { return l.a.removeLast() }
            return l.a.removeFirst()
        case "peek", "peekFirst", "element", "getFirst", "firstElement", "peekLast", "getLast", "lastElement":
            if l.a.isEmpty {
                if n.hasPrefix("peek") { return .nulo }
                throw excepcion(l.k == "Stack" ? "EmptyStackException" : "NoSuchElementException", "", linea)
            }
            if n == "peekLast" || n == "getLast" || n == "lastElement" || (n == "peek" && l.k == "Stack") { return l.a[l.a.count - 1] }
            return l.a[0]
        case "search":
            let x = try arg(a, 0, n, linea)
            for k in (0..<l.a.count).reversed() where try iguales(l.a[k], x, linea) { return entJ(l.a.count - k) }
            return entJ(-1)
        default:
            return try metodoListaJava2(l, n, a, linea)
        }
    }

    func metodoListaJava2(_ l: CLista, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        switch n {
        case "addAll":
            if a.count == 2 {
                let i = try indiceJava(a, 0, n, l.a.count, linea, incluyeFin: true)
                l.a.insert(contentsOf: try valoresColeccion(a[1], linea), at: i)
            } else {
                l.a += try valoresColeccion(try arg(a, 0, n, linea), linea)
            }
            return verdad(true)
        case "removeAll", "retainAll":
            let otros = try valoresColeccion(try arg(a, 0, n, linea), linea)
            var out: [CV] = []
            for v in l.a {
                var esta = false
                for o in otros where try iguales(v, o, linea) { esta = true; break }
                if esta == (n == "retainAll") { out.append(v) }
            }
            let cambio = out.count != l.a.count
            l.a = out
            return verdad(cambio)
        case "containsAll":
            for o in try valoresColeccion(try arg(a, 0, n, linea), linea) {
                var esta = false
                for v in l.a where try iguales(v, o, linea) { esta = true; break }
                if !esta { return verdad(false) }
            }
            return verdad(true)
        case "removeIf":
            let f = try arg(a, 0, n, linea)
            var out: [CV] = []
            for v in l.a where !aBool(try invoca(f, [v], linea)) { out.append(v) }
            let cambio = out.count != l.a.count
            l.a = out
            return verdad(cambio)
        case "replaceAll":
            let f = try arg(a, 0, n, linea)
            for k in 0..<l.a.count { l.a[k] = try invoca(f, [l.a[k]], linea) }
            return .vacio
        case "forEach":
            let f = try arg(a, 0, n, linea)
            for v in l.a { _ = try invoca(f, [v], linea) }
            return .vacio
        case "sort":
            var vals = l.a
            if let c = a.first, !c.esNulo {
                try ordena(&vals) { x, y in try self.entero(try self.invoca(c, [x, y], linea), linea) < 0 }
            } else {
                try ordena(&vals) { x, y in try self.compara(x, y, linea) < 0 }
            }
            l.a = vals
            return .vacio
        case "subList":
            let i = try indiceJava(a, 0, n, l.a.count, linea, incluyeFin: true)
            let j = try indiceJava(a, 1, n, l.a.count, linea, incluyeFin: true)
            guard i <= j else { throw excepcion("IllegalArgumentException", "fromIndex(\(i)) > toIndex(\(j))", linea) }
            return .l(CLista("ArrayList", Array(l.a[i..<j])))
        case "toArray":
            let mem = CMem(l.a)
            mem.esArreglo = true
            return .p(CPtr(mem: mem, i: 0))
        case "stream":
            return .l(CLista("Stream", l.a))
        case "iterator", "listIterator":
            return .it(CIterBase(lista: l), 0)
        case "descendingIterator":
            return .it(CIterBase(lista: CLista(l.k, l.a.reversed())), 0)
        case "equals":
            return verdad(try iguales(.l(l), try arg(a, 0, n, linea), linea))
        case "hashCode":
            var h: Int64 = 1
            for v in l.a { h = cAjusta(31 &* h &+ (try hashJava(v, linea)), .int) }
            return .n(h, .int)
        case "toString":
            return cadena(try texto(.l(l), linea))
        case "reversed":
            return .l(CLista(l.k, l.a.reversed()))
        case "getClass":
            return cadena(l.k)
        case "ensureCapacity", "trimToSize":
            return .vacio
        default:
            throw CFallo("\(l.k) no tiene el método \(n)()", linea)
        }
    }

    func metodoColaJava(_ l: CLista, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        switch n {
        case "add", "offer":
            try empujaCola(l, try arg(a, 0, n, linea), linea)
            return verdad(true)
        case "poll", "remove" where a.isEmpty:
            if l.a.isEmpty {
                if n == "poll" { return .nulo }
                throw excepcion("NoSuchElementException", "", linea)
            }
            return try sacaCola(l, linea)
        case "peek", "element":
            if l.a.isEmpty {
                if n == "peek" { return .nulo }
                throw excepcion("NoSuchElementException", "", linea)
            }
            return l.a[0]
        case "remove":
            let x = try arg(a, 0, n, linea)
            guard let k = try l.a.firstIndex(where: { try iguales($0, x, linea) }) else { return verdad(false) }
            let ultimo = l.a.removeLast()
            if k < l.a.count {
                try hundeCola(l, k, ultimo, linea)
                if case .o = l.a[k] {} else {
                    var j = k
                    let v = l.a[j]
                    while j > 0 {
                        let p = (j - 1) >> 1
                        if !(try antesEnCola(l, v, l.a[p], linea)) { break }
                        l.a[j] = l.a[p]
                        j = p
                    }
                    l.a[j] = v
                }
            }
            return verdad(true)
        case "addAll":
            for v in try valoresColeccion(try arg(a, 0, n, linea), linea) { try empujaCola(l, v, linea) }
            return verdad(true)
        default:
            let copia = CLista("ArrayList", l.a)
            return try metodoListaJava(copia, n, a, linea)
        }
    }

    func metodoConstructor(_ l: CLista, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        guard case .s(var u) = l.a[0] else { return .vacio }
        l.a[0] = .vacio
        defer { l.a[0] = .s(u) }
        func fuera(_ i: Int) -> Error { excepcion("StringIndexOutOfBoundsException", "index \(i),length \(u.count)", linea) }
        switch n {
        case "append":
            let v = try arg(a, 0, n, linea)
            if case .p(let p) = v, let mem = p.mem, case .num(.jchar)? = mem.elem {
                for c in mem.a[p.i...] { if case .n(let x, _) = c { u.append(UInt16(truncatingIfNeeded: x)) } }
            } else {
                u += unidades(try texto(v, linea))
            }
            return .l(l)
        case "toString": return .s(u)
        case "length": return entJ(u.count)
        case "isEmpty": return verdad(u.isEmpty)
        case "charAt":
            let i = Int(try argEnt(a, 0, n, linea))
            guard i >= 0 && i < u.count else { throw fuera(i) }
            return .n(Int64(u[i]), .jchar)
        case "setCharAt":
            let i = Int(try argEnt(a, 0, n, linea))
            guard i >= 0 && i < u.count else { throw fuera(i) }
            u[i] = UInt16(truncatingIfNeeded: try argEnt(a, 1, n, linea))
            return .vacio
        case "insert":
            let i = Int(try argEnt(a, 0, n, linea))
            guard i >= 0 && i <= u.count else { throw fuera(i) }
            u.insert(contentsOf: unidades(try texto(try arg(a, 1, n, linea), linea)), at: i)
            return .l(l)
        case "reverse":
            u.reverse()
            return .l(l)
        case "deleteCharAt":
            let i = Int(try argEnt(a, 0, n, linea))
            guard i >= 0 && i < u.count else { throw fuera(i) }
            u.remove(at: i)
            return .l(l)
        case "delete", "replace":
            let i = Int(try argEnt(a, 0, n, linea))
            let j = Swift.min(Int(try argEnt(a, 1, n, linea)), u.count)
            guard i >= 0 && i <= j else { throw fuera(i) }
            u.removeSubrange(i..<j)
            if n == "replace" { u.insert(contentsOf: unidades(try texto(try arg(a, 2, n, linea), linea)), at: i) }
            return .l(l)
        case "setLength":
            let k = Int(try argEnt(a, 0, n, linea))
            if k < u.count { u.removeLast(u.count - k) } else { u += [UInt16](repeating: 0, count: k - u.count) }
            return .vacio
        case "indexOf", "lastIndexOf", "substring", "contains", "equals", "chars", "compareTo":
            if n == "equals" { return verdad(false) }
            return try metodoCadenaJava(u, n, a, linea)
        case "capacity": return entJ(Swift.max(16, u.count))
        default:
            throw CFallo("StringBuilder no tiene el método \(n)()", linea)
        }
    }

    func metodoRandom(_ l: CLista, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        guard case .n(let raw, _) = l.a[0] else { return .vacio }
        var s = UInt64(bitPattern: raw)
        defer { l.a[0] = .n(Int64(bitPattern: s), .long) }
        switch n {
        case "nextInt":
            if a.isEmpty { return .n(Int64(siguienteBits(&s, 32)), .int) }
            var origen: Int64 = 0
            var limite = try argEnt(a, 0, n, linea)
            if a.count > 1 { origen = limite; limite = try argEnt(a, 1, n, linea) - origen }
            if limite <= 0 { throw excepcion("IllegalArgumentException", "bound must be positive", linea) }
            return .n(origen + Int64(siguienteEnteroJava(&s, Int32(truncatingIfNeeded: limite))), .int)
        case "nextDouble": return .d(siguienteRealJava(&s), false)
        case "nextFloat":
            let bits: Int32 = siguienteBits(&s, 24)
            let f: Float = Float(bits) / Float(16_777_216)
            return .d(Double(f), true)
        case "nextBoolean": return verdad(siguienteBits(&s, 1) != 0)
        case "nextLong":
            let hi: Int64 = Int64(siguienteBits(&s, 32))
            let lo: Int64 = Int64(siguienteBits(&s, 32))
            let r: Int64 = (hi << 32) &+ lo
            return .n(r, .long)
        case "nextGaussian":
            return .d(gaussiana(&s), false)
        case "setSeed":
            s = CMaq.semillaJava(try argEnt(a, 0, n, linea))
            return .vacio
        default:
            throw CFallo("Random no tiene el método \(n)()", linea)
        }
    }

    func gaussiana(_ s: inout UInt64) -> Double {
        var v1: Double = 0
        var v2: Double = 0
        var r: Double = 0
        repeat {
            v1 = 2 * siguienteRealJava(&s) - 1
            v2 = 2 * siguienteRealJava(&s) - 1
            r = v1 * v1 + v2 * v2
        } while r >= 1 || r == 0
        let factor: Double = (-2 * logNatural(r) / r).squareRoot()
        return v1 * factor
    }

    func metodoOpcional(_ l: CLista, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        let v = l.a.first
        switch n {
        case "isPresent": return verdad(v != nil)
        case "isEmpty": return verdad(v == nil)
        case "get", "getAsInt", "getAsDouble", "getAsLong", "orElseThrow":
            guard let x = v else { throw excepcion("NoSuchElementException", "No value present", linea) }
            return x
        case "orElse":
            if let x = v { return x }
            return try arg(a, 0, n, linea)
        case "orElseGet":
            if let x = v { return x }
            return try invoca(try arg(a, 0, n, linea), [], linea)
        case "ifPresent":
            if let x = v { _ = try invoca(try arg(a, 0, n, linea), [x], linea) }
            return .vacio
        case "map":
            guard let x = v else { return .l(l) }
            return opcional(try invoca(try arg(a, 0, n, linea), [x], linea))
        case "toString":
            return cadena(v == nil ? "Optional.empty" : "Optional[\(try texto(v!, linea))]")
        default:
            throw CFallo("Optional no tiene el método \(n)()", linea)
        }
    }

    func metodoArchivo(_ l: CLista, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        let ruta = try texto(l.a[0], linea)
        switch n {
        case "exists", "isFile", "canRead": return verdad(leerArchivo(ruta) != nil)
        case "getName": return cadena((ruta as NSString).lastPathComponent)
        case "getPath", "getAbsolutePath", "toString": return cadena(ruta)
        case "length": return .n(Int64(leerArchivo(ruta)?.count ?? 0), .long)
        case "createNewFile":
            if leerArchivo(ruta) != nil { return verdad(false) }
            return verdad(escribirArchivo(ruta, [], false))
        case "delete": return verdad(false)
        case "isDirectory": return verdad(false)
        default: throw CFallo("File no tiene el método \(n)()", linea)
        }
    }

    func metodoAtomico(_ l: CLista, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        let v = try entero(l.a[0], linea)
        switch n {
        case "get", "intValue": return .n(v, .int)
        case "set": l.a[0] = try arg(a, 0, n, linea); return .vacio
        case "incrementAndGet": l.a[0] = .n(v + 1, .int); return l.a[0]
        case "getAndIncrement": l.a[0] = .n(v + 1, .int); return .n(v, .int)
        case "decrementAndGet": l.a[0] = .n(v - 1, .int); return l.a[0]
        case "addAndGet": l.a[0] = .n(v + (try argEnt(a, 0, n, linea)), .int); return l.a[0]
        default: throw CFallo("AtomicInteger no tiene el método \(n)()", linea)
        }
    }

    // MARK: streams

    func metodoStream(_ l: CLista, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        func f() throws -> CV { try arg(a, 0, n, linea) }
        switch n {
        case "filter":
            let p = try f()
            var out: [CV] = []
            for v in l.a where aBool(try invoca(p, [v], linea)) { out.append(v) }
            return .l(CLista("Stream", out))
        case "map", "mapToInt", "mapToObj", "mapToDouble", "mapToLong", "asDoubleStream":
            if n == "asDoubleStream" { return .l(CLista("Stream", l.a.map { convierte($0, .real(false)) })) }
            let fn = try f()
            var out: [CV] = []
            for v in l.a { out.append(try invoca(fn, [v], linea)) }
            if n == "mapToDouble" { out = out.map { convierte($0, .real(false)) } }
            return .l(CLista("Stream", out))
        case "boxed", "parallel", "sequential", "stream":
            return .l(l)
        case "sorted":
            var vals = l.a
            if let c = a.first {
                try ordena(&vals) { x, y in try self.entero(try self.invoca(c, [x, y], linea), linea) < 0 }
            } else {
                try ordena(&vals) { x, y in try self.compara(x, y, linea) < 0 }
            }
            return .l(CLista("Stream", vals))
        case "distinct":
            var out: [CV] = []
            var vistos: Set<CKey> = []
            for v in l.a where vistos.insert(try clave(v, linea)).inserted { out.append(v) }
            return .l(CLista("Stream", out))
        case "limit": return .l(CLista("Stream", Array(l.a.prefix(Int(try argEnt(a, 0, n, linea))))))
        case "skip": return .l(CLista("Stream", Array(l.a.dropFirst(Int(try argEnt(a, 0, n, linea))))))
        case "forEach", "forEachOrdered":
            let fn = try f()
            for v in l.a { _ = try invoca(fn, [v], linea) }
            return .vacio
        case "peek":
            let fn = try f()
            for v in l.a { _ = try invoca(fn, [v], linea) }
            return .l(l)
        case "count": return .n(Int64(l.a.count), .long)
        case "sum":
            var acc: CV = .n(0, .int)
            for v in l.a { acc = try binaria(.suma, acc, v, linea) }
            return acc
        case "average":
            if l.a.isEmpty { return opcional(nil) }
            var s = 0.0
            for v in l.a { s += try real(v, linea) }
            return opcional(.d(s / Double(l.a.count), false))
        case "max", "min":
            guard var mejor = l.a.first else { return opcional(nil) }
            for v in l.a.dropFirst() {
                let c = a.isEmpty ? try compara(v, mejor, linea) : Int(try entero(try invoca(a[0], [v, mejor], linea), linea))
                if (n == "max" && c > 0) || (n == "min" && c < 0) { mejor = v }
            }
            return opcional(mejor)
        case "reduce":
            if a.count >= 2 {
                var acc = a[0]
                for v in l.a { acc = try invoca(a[1], [acc, v], linea) }
                return acc
            }
            guard var acc = l.a.first else { return opcional(nil) }
            for v in l.a.dropFirst() { acc = try invoca(try f(), [acc, v], linea) }
            return opcional(acc)
        case "anyMatch", "allMatch", "noneMatch":
            let p = try f()
            var algun = false
            var todos = true
            for v in l.a { if aBool(try invoca(p, [v], linea)) { algun = true } else { todos = false } }
            return verdad(n == "anyMatch" ? algun : (n == "allMatch" ? todos : !algun))
        case "findFirst", "findAny": return opcional(l.a.first)
        case "toList": return .l(CLista("ArrayList", l.a))
        case "toArray":
            let mem = CMem(l.a)
            mem.esArreglo = true
            return .p(CPtr(mem: mem, i: 0))
        case "collect":
            return try recoge(l.a, try f(), linea)
        case "iterator":
            return .it(CIterBase(lista: l), 0)
        default:
            throw CFallo("Stream no tiene el método \(n)()", linea)
        }
    }

    func recoge(_ vals: [CV], _ c: CV, _ linea: Int) throws -> CV {
        guard case .l(let col) = c, col.k.hasPrefix("Collector:") else { throw CFallo("collect necesita un Collectors.…", linea) }
        let tipo = String(col.k.dropFirst("Collector:".count))
        switch tipo {
        case "toList": return .l(CLista("ArrayList", vals))
        case "toSet":
            let mp = CMapa("HashSet", ordenado: false, esSet: true)
            for v in vals { mp.pon(try clave(v, linea), v, v) }
            return .m(mp)
        case "joining":
            let sep = col.a.isEmpty ? "" : try texto(col.a[0], linea)
            let pre = col.a.count > 1 ? try texto(col.a[1], linea) : ""
            let suf = col.a.count > 2 ? try texto(col.a[2], linea) : ""
            return cadena(pre + (try vals.map { try texto($0, linea) }).joined(separator: sep) + suf)
        case "counting": return .n(Int64(vals.count), .long)
        case "summingInt":
            var s: Int64 = 0
            for v in vals { s += try entero(try invoca(col.a[0], [v], linea), linea) }
            return .n(s, .int)
        case "averagingInt", "averagingDouble":
            var s = 0.0
            for v in vals { s += try real(try invoca(col.a[0], [v], linea), linea) }
            return .d(vals.isEmpty ? 0 : s / Double(vals.count), false)
        case "groupingBy":
            let mp = CMapa("HashMap", ordenado: false, esSet: false)
            var grupos: [CKey: [CV]] = [:]
            var claves: [(CKey, CV)] = []
            for v in vals {
                let k = try invoca(col.a[0], [v], linea)
                let ck = try clave(k, linea)
                if grupos[ck] == nil { claves.append((ck, k)) }
                grupos[ck, default: []].append(v)
            }
            for (ck, k) in claves {
                let g = grupos[ck] ?? []
                mp.pon(ck, k, col.a.count > 1 ? try recoge(g, col.a[1], linea) : .l(CLista("ArrayList", g)))
            }
            return .m(mp)
        case "toMap":
            let mp = CMapa("HashMap", ordenado: false, esSet: false)
            for v in vals {
                let k = try invoca(col.a[0], [v], linea)
                mp.pon(try clave(k, linea), k, try invoca(col.a[1], [v], linea))
            }
            return .m(mp)
        default:
            throw CFallo("Collectors.\(tipo) no está disponible", linea)
        }
    }

    // MARK: Map y Set

    func metodoMapaJava(_ mp: CMapa, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        func k0() throws -> (CKey, CV) {
            let v = try arg(a, 0, n, linea)
            return (try clave(v, linea), v)
        }
        switch n {
        case "put":
            let (k, kv) = try k0()
            let viejo = mp.vals[k]?.1 ?? .nulo
            mp.pon(k, kv, try arg(a, 1, n, linea))
            return viejo
        case "get":
            return mp.vals[try k0().0]?.1 ?? .nulo
        case "getOrDefault":
            if let x = mp.vals[try k0().0]?.1 { return x }
            return try arg(a, 1, n, linea)
        case "containsKey", "contains":
            return verdad(mp.vals[try k0().0] != nil)
        case "containsValue":
            let x = try arg(a, 0, n, linea)
            for k in mp.claves { if let (_, v) = mp.vals[k], try iguales(v, x, linea) { return verdad(true) } }
            return verdad(false)
        case "remove":
            let (k, _) = try k0()
            let viejo = mp.vals[k]?.1
            let estaba = mp.quita(k)
            if mp.esSet { return verdad(estaba) }
            return viejo ?? .nulo
        case "add":
            let (k, kv) = try k0()
            if mp.vals[k] != nil { return verdad(false) }
            mp.pon(k, kv, kv)
            return verdad(true)
        case "size": return entJ(mp.claves.count)
        case "isEmpty": return verdad(mp.claves.isEmpty)
        case "clear":
            mp.claves = []
            mp.vals = [:]
            return .vacio
        case "keySet":
            let s = CMapa(mp.ordenado ? "TreeSet" : "LinkedHashSet", ordenado: mp.ordenado, esSet: true)
            for k in mp.claves { if let (kv, _) = mp.vals[k] { s.pon(k, kv, kv) } }
            return .m(s)
        case "values":
            return .l(CLista("ArrayList", mp.claves.compactMap { mp.vals[$0]?.1 }))
        case "entrySet":
            return .l(CLista("ArrayList", mp.claves.map { entrada(mp, $0) }))
        case "putIfAbsent":
            let (k, kv) = try k0()
            if let v = mp.vals[k]?.1, !v.esNulo { return v }
            mp.pon(k, kv, try arg(a, 1, n, linea))
            return .nulo
        case "merge":
            let (k, kv) = try k0()
            let nuevo = try arg(a, 1, n, linea)
            if let viejo = mp.vals[k]?.1, !viejo.esNulo {
                let r = try invoca(try arg(a, 2, n, linea), [viejo, nuevo], linea)
                if r.esNulo { _ = mp.quita(k) } else { mp.pon(k, kv, r) }
                return r
            }
            mp.pon(k, kv, nuevo)
            return nuevo
        case "computeIfAbsent":
            let (k, kv) = try k0()
            if let v = mp.vals[k]?.1, !v.esNulo { return v }
            let r = try invoca(try arg(a, 1, n, linea), [kv], linea)
            if !r.esNulo { mp.pon(k, kv, r) }
            return r
        case "computeIfPresent", "compute":
            let (k, kv) = try k0()
            let viejo = mp.vals[k]?.1 ?? .nulo
            if n == "computeIfPresent" && viejo.esNulo { return .nulo }
            let r = try invoca(try arg(a, 1, n, linea), [kv, viejo], linea)
            if r.esNulo { _ = mp.quita(k) } else { mp.pon(k, kv, r) }
            return r
        case "forEach":
            let f = try arg(a, 0, n, linea)
            for k in mp.claves {
                guard let (kv, v) = mp.vals[k] else { continue }
                _ = try invoca(f, mp.esSet ? [kv] : [kv, v], linea)
            }
            return .vacio
        default:
            return try metodoMapaJava2(mp, n, a, linea)
        }
    }

    func metodoMapaJava2(_ mp: CMapa, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        func vacio() -> Error { excepcion("NoSuchElementException", "", linea) }
        switch n {
        case "firstKey", "first", "lastKey", "last":
            guard let k = n.hasPrefix("first") ? mp.claves.first : mp.claves.last, let (kv, _) = mp.vals[k] else { throw vacio() }
            return kv
        case "firstEntry", "lastEntry", "pollFirstEntry", "pollLastEntry":
            guard let k = n.contains("First") || n == "firstEntry" ? mp.claves.first : mp.claves.last else { return .nulo }
            let e = entrada(mp, k)
            if n.hasPrefix("poll"), case .o(let o) = e {
                if case .ref(let r) = o.c.a[1] { o.c.a[1] = try lee(r.l) }
                _ = mp.quita(k)
            }
            return e
        case "pollFirst", "pollLast":
            guard let k = n == "pollFirst" ? mp.claves.first : mp.claves.last, let (kv, _) = mp.vals[k] else { return .nulo }
            _ = mp.quita(k)
            return kv
        case "floorKey", "floor", "lowerKey", "lower", "ceilingKey", "ceiling", "higherKey", "higher",
             "floorEntry", "ceilingEntry", "lowerEntry", "higherEntry":
            let k = try clave(try arg(a, 0, n, linea), linea)
            let p = mp.posicion(k)
            let exacto = p < mp.claves.count && mp.claves[p] == k
            var idx: Int
            if n.hasPrefix("floor") { idx = exacto ? p : p - 1 }
            else if n.hasPrefix("lower") { idx = p - 1 }
            else if n.hasPrefix("ceiling") { idx = p }
            else { idx = exacto ? p + 1 : p }
            guard idx >= 0 && idx < mp.claves.count, let (kv, _) = mp.vals[mp.claves[idx]] else { return .nulo }
            return n.hasSuffix("Entry") ? entrada(mp, mp.claves[idx]) : kv
        case "headMap", "tailMap", "headSet", "tailSet", "subMap", "subSet":
            let x = try clave(try arg(a, 0, n, linea), linea)
            let nuevo = CMapa(mp.k, ordenado: mp.ordenado, esSet: mp.esSet)
            var hasta: CKey?
            if n.hasPrefix("sub") { hasta = try clave(try arg(a, 1, n, linea), linea) }
            let incluye = a.count > 1 && !n.hasPrefix("sub") ? aBool(a[1]) : n.hasPrefix("tail")
            for k in mp.claves {
                let ok: Bool
                if n.hasPrefix("head") { ok = k < x || (incluye && k == x) }
                else if n.hasPrefix("tail") { ok = x < k || (incluye && k == x) }
                else { ok = !(k < x) && (hasta.map { k < $0 } ?? true) }
                if ok, let par = mp.vals[k] { nuevo.pon(k, par.0, par.1) }
            }
            return .m(nuevo)
        case "descendingKeySet", "descendingSet", "descendingMap":
            let nuevo = CMapa("LinkedHash" + (mp.esSet ? "Set" : "Map"), ordenado: false, esSet: mp.esSet)
            for k in mp.claves.reversed() { if let par = mp.vals[k] { nuevo.pon(k, par.0, par.1) } }
            return .m(nuevo)
        case "addAll", "putAll":
            let x = try arg(a, 0, n, linea)
            var cambio = false
            if case .m(let otro) = x, !otro.esSet {
                for k in otro.claves { if let par = otro.vals[k] { mp.pon(k, par.0, par.1) } }
                return .vacio
            }
            for v in try valoresColeccion(x, linea) {
                let k = try clave(v, linea)
                if mp.vals[k] == nil { cambio = true }
                mp.pon(k, v, v)
            }
            return verdad(cambio)
        case "removeAll", "retainAll":
            var otros = Set<CKey>()
            for v in try valoresColeccion(try arg(a, 0, n, linea), linea) { otros.insert(try clave(v, linea)) }
            var cambio = false
            for k in mp.claves where otros.contains(k) != (n == "retainAll") { _ = mp.quita(k); cambio = true }
            return verdad(cambio)
        case "containsAll":
            for v in try valoresColeccion(try arg(a, 0, n, linea), linea) where mp.vals[try clave(v, linea)] == nil {
                return verdad(false)
            }
            return verdad(true)
        case "removeIf":
            let f = try arg(a, 0, n, linea)
            var cambio = false
            for k in mp.claves {
                guard let (kv, _) = mp.vals[k] else { continue }
                if aBool(try invoca(f, [kv], linea)) { _ = mp.quita(k); cambio = true }
            }
            return verdad(cambio)
        case "iterator":
            return .it(CIterBase(mapa: mp), 0)
        case "stream":
            return .l(CLista("Stream", mp.claves.compactMap { mp.vals[$0]?.0 }))
        case "toArray":
            let mem = CMem(mp.claves.compactMap { mp.vals[$0]?.0 })
            mem.esArreglo = true
            return .p(CPtr(mem: mem, i: 0))
        case "equals":
            return verdad(try iguales(.m(mp), try arg(a, 0, n, linea), linea))
        case "toString":
            return cadena(try texto(.m(mp), linea))
        case "hashCode":
            var h: Int64 = 0
            for k in mp.claves { if let (kv, _) = mp.vals[k] { h = cAjusta(h &+ (try hashJava(kv, linea)), .int) } }
            return .n(h, .int)
        default:
            throw CFallo("\(mp.k) no tiene el método \(n)()", linea)
        }
    }

    // MARK: System.out, Scanner, BufferedReader, PrintWriter

    func metodoCanalJava(_ c: CCanal, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        switch n {
        case "println", "print":
            var s = ""
            if let v = a.first {
                if case .p(let p) = v, let mem = p.mem, case .num(.jchar)? = mem.elem {
                    var u: [UInt16] = []
                    for x in mem.a[p.i...] { if case .n(let k, _) = x { u.append(UInt16(truncatingIfNeeded: k)) } }
                    s = texto(u)
                } else {
                    s = try texto(v, linea)
                }
            }
            if n == "println" { s += "\n" }
            try escribeCanal(c, Array(s.utf8))
            return .vacio
        case "printf", "format":
            let f = try texto(try arg(a, 0, n, linea), linea)
            try escribeCanal(c, Array(try formateaJava(f, Array(a.dropFirst()), linea).utf8))
            return .canal(c)
        case "write", "append":
            let v = try arg(a, 0, n, linea)
            if case .n(let x, let k) = v, k != .jchar { try escribeCanal(c, Array(String(decoding: [UInt16(truncatingIfNeeded: x)], as: UTF16.self).utf8)) }
            else { try escribeCanal(c, Array(try texto(v, linea).utf8)) }
            return n == "append" ? .canal(c) : .vacio
        case "newLine":
            try escribeCanal(c, [10])
            return .vacio
        case "flush":
            if c.clase == .salida { vuelca() }
            return .vacio
        case "close":
            try cierraCanal(c)
            return .vacio
        case "checkError":
            return verdad(false)
        default:
            return try metodoLectorJava(c, n, a, linea)
        }
    }

    func metodoLectorJava(_ c: CCanal, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        guard let ent = c.ent ?? entradaDe(c) else { throw CFallo("no hay entrada", linea) }
        if c.ent === entrada { vuelca() }
        func token() throws -> String {
            guard let w = ent.palabra() else { throw excepcion("NoSuchElementException", "", linea) }
            return String(decoding: w, as: UTF8.self)
        }
        func mira() -> String? {
            let guardado = ent.i
            defer { ent.i = guardado }
            return ent.palabra().map { String(decoding: $0, as: UTF8.self) }
        }
        switch n {
        case "nextInt", "nextLong", "nextShort", "nextByte":
            let t = try token()
            guard let x = Int64(t), n != "nextInt" || (x >= Int64(Int32.min) && x <= Int64(Int32.max)) else {
                throw excepcion("InputMismatchException", "For input string: \"\(t)\"", linea)
            }
            return .n(x, n == "nextLong" ? .long : .int)
        case "nextDouble", "nextFloat":
            let t = try token()
            guard let d = Double(t.replacingOccurrences(of: ",", with: ".")) else { throw excepcion("InputMismatchException", "For input string: \"\(t)\"", linea) }
            return .d(n == "nextFloat" ? Double(Float(d)) : d, n == "nextFloat")
        case "nextBoolean":
            let t = try token().lowercased()
            guard t == "true" || t == "false" else { throw excepcion("InputMismatchException", t, linea) }
            return verdad(t == "true")
        case "next":
            return cadena(try token())
        case "nextLine", "readLine":
            guard let l = ent.linea() else {
                if n == "readLine" { return .nulo }
                throw excepcion("NoSuchElementException", "No line found", linea)
            }
            return cadena(String(decoding: l, as: UTF8.self))
        case "read":
            return .n(Int64(ent.lee()), .int)
        case "hasNext": return verdad(mira() != nil)
        case "hasNextInt", "hasNextLong":
            guard let t = mira() else { return verdad(false) }
            return verdad(Int64(t) != nil)
        case "hasNextDouble":
            guard let t = mira() else { return verdad(false) }
            return verdad(Double(t) != nil)
        case "hasNextLine": return verdad(ent.hay())
        case "ready": return verdad(ent.hay())
        case "close", "useDelimiter", "useLocale": return .vacio
        case "lines":
            var out: [CV] = []
            while let l = ent.linea() { out.append(cadena(String(decoding: l, as: UTF8.self))) }
            return .l(CLista("Stream", out))
        default:
            throw CFallo("\(c.nombre.isEmpty ? "el stream" : c.nombre) no tiene el método \(n)()", linea)
        }
    }

    // MARK: números (Integer x; x.equals(y)…), arreglos y objetos

    func metodoNumero(_ v: CV, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        switch n {
        case "intValue": return convierte(v, .num(.int))
        case "longValue": return convierte(v, .num(.long))
        case "doubleValue": return convierte(v, .real(false))
        case "floatValue": return convierte(v, .real(true))
        case "shortValue": return convierte(v, .num(.short))
        case "byteValue": return convierte(v, .num(.char))
        case "charValue", "booleanValue": return v
        case "equals":
            let x = try arg(a, 0, n, linea)
            if case .n(_, let k1) = v, case .n(_, let k2) = x, (k1 == .long) != (k2 == .long) { return verdad(false) }
            if case .d = v, case .n = x { return verdad(false) }
            return verdad(try iguales(v, x, linea))
        case "compareTo", "compare": return entJ(try compara(v, try arg(a, 0, n, linea), linea))
        case "hashCode": return .n(try hashJava(v, linea), .int)
        case "toString": return cadena(try texto(v, linea))
        case "isNaN": return verdad((try real(v, linea)).isNaN)
        case "isInfinite": return verdad((try real(v, linea)).isInfinite)
        default:
            throw CFallo("\(v.tipoNombre) no tiene el método \(n)()", linea)
        }
    }

    func metodoArreglo(_ p: CPtr, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        guard let mem = p.mem else { throw excepcion("NullPointerException", "arreglo null", linea) }
        switch n {
        case "clone":
            let x = CMem(Array(mem.a[p.i...]))
            x.elem = mem.elem
            x.esArreglo = true
            return .p(CPtr(mem: x, i: 0))
        case "equals":
            if case .p(let q) = try arg(a, 0, n, linea) { return verdad(q.mem === mem && q.i == p.i) }
            return verdad(false)
        case "hashCode": return entJ(Int(truncatingIfNeeded: UInt(bitPattern: ObjectIdentifier(mem).hashValue) & 0x7FFFFFFF))
        case "getClass": return cadena("[" + CMaq.firmaJava(mem.elem ?? .auto))
        case "toString": return cadena(try texto(.p(p), linea))
        default:
            throw CFallo("un arreglo no tiene el método \(n)() (¿querías .length?)", linea)
        }
    }

    func metodoObjetoNativo(_ o: CObj, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        switch n {
        case "equals":
            if case .o(let x) = try arg(a, 0, n, linea) {
                if x === o { return verdad(true) }
                if o.clase.nombre == "pair" { return verdad(try iguales(.o(o), .o(x), linea)) }
                return verdad(false)
            }
            return verdad(false)
        case "hashCode":
            if o.clase.nombre == "pair" { return .n(try hashJava(o.c.a[0], linea) ^ (try hashJava(o.c.a[1], linea)), .int) }
            return entJ(hashObjeto(o))
        case "toString": return cadena(try textoObjeto(o, linea))
        case "getClass": return cadena(nombreCompleto(o.clase))
        case "name":
            if let i = o.clase.indice["$nombre"] { return o.c.a[i] }
        case "ordinal":
            if let i = o.clase.indice["$ordinal"] { return o.c.a[i] }
        case "compareTo":
            return entJ(try compara(.o(o), try arg(a, 0, n, linea), linea))
        case "getKey", "getFirst":
            if o.clase.nombre == "pair" { return o.c.a[0] }
        case "getValue", "getSecond":
            if o.clase.nombre == "pair" {
                if case .ref(let r) = o.c.a[1] { return try lee(r.l) }
                return o.c.a[1]
            }
        case "setValue":
            if o.clase.nombre == "pair" {
                let v = try arg(a, 0, n, linea)
                let viejo = o.c.a[1]
                if case .ref(let r) = viejo { let ant = try lee(r.l); try escribe(r.l, v); return ant }
                o.c.a[1] = v
                return viejo
            }
        default:
            break
        }
        throw CFallo("\(o.clase.nombre) no tiene el método \(n)()", linea)
    }

    func nombreCompleto(_ c: CClase) -> String {
        let javaLang: Set<String> = ["Exception", "RuntimeException", "ArithmeticException", "ArrayIndexOutOfBoundsException",
                                      "IndexOutOfBoundsException", "StringIndexOutOfBoundsException", "NullPointerException",
                                      "NumberFormatException", "IllegalArgumentException", "IllegalStateException",
                                      "ClassCastException", "NegativeArraySizeException", "UnsupportedOperationException",
                                      "Throwable", "Error", "StackOverflowError", "OutOfMemoryError", "InterruptedException",
                                      "CloneNotSupportedException", "AssertionError"]
        let javaUtil: Set<String> = ["InputMismatchException", "NoSuchElementException", "ConcurrentModificationException", "EmptyStackException"]
        let javaIO: Set<String> = ["IOException", "FileNotFoundException", "UncheckedIOException"]
        if javaLang.contains(c.nombre) { return "java.lang." + c.nombre }
        if javaUtil.contains(c.nombre) { return "java.util." + c.nombre }
        if javaIO.contains(c.nombre) { return "java.io." + c.nombre }
        return c.nombre
    }
}
