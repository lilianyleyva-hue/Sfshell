import Foundation

// ============================================================
// MARK: - Java: clases y métodos estáticos de la biblioteca
// ============================================================

extension CMaq {

    static let clasesJava: Set<String> = [
        "Math", "System", "Integer", "Long", "Double", "Float", "Character", "Boolean", "String", "Arrays", "Collections",
        "List", "Map", "Set", "Objects", "Thread", "IntStream", "Collectors", "Stream", "Comparator", "Byte", "Short",
        "JOptionPane", "Optional", "ThreadLocalRandom", "StrictMath", "Graficos", "LongStream", "DoubleStream", "Pantalla"
    ]

    static let funcionesJava: Set<String> = [
        "Math.abs", "Math.max", "Math.min", "Math.pow", "Math.sqrt", "Math.cbrt", "Math.floor", "Math.ceil", "Math.round",
        "Math.random", "Math.sin", "Math.cos", "Math.tan", "Math.asin", "Math.acos", "Math.atan", "Math.atan2", "Math.log",
        "Math.log10", "Math.exp", "Math.hypot", "Math.signum", "Math.toRadians", "Math.toDegrees", "Math.floorDiv",
        "Math.floorMod", "Math.rint", "Math.addExact", "Math.multiplyExact", "Math.subtractExact", "Math.toIntExact",
        "Math.log1p", "Math.expm1", "Math.sinh", "Math.cosh", "Math.tanh", "Math.negateExact", "Math.clamp",
        "Integer.parseInt", "Integer.valueOf", "Integer.toString", "Integer.toBinaryString", "Integer.toHexString",
        "Integer.toOctalString", "Integer.max", "Integer.min", "Integer.sum", "Integer.compare", "Integer.bitCount",
        "Integer.signum", "Integer.reverse", "Integer.highestOneBit", "Integer.numberOfTrailingZeros", "Integer.parseUnsignedInt",
        "Long.parseLong", "Long.valueOf", "Long.toString", "Long.compare", "Long.max", "Long.min", "Long.sum", "Long.toBinaryString", "Long.bitCount",
        "Double.parseDouble", "Double.valueOf", "Double.compare", "Double.isNaN", "Double.toString", "Double.max",
        "Double.min", "Double.sum", "Double.isInfinite", "Float.parseFloat", "Float.valueOf", "Float.compare",
        "Character.isDigit", "Character.isLetter", "Character.isLetterOrDigit", "Character.isAlphabetic",
        "Character.isUpperCase", "Character.isLowerCase", "Character.isWhitespace", "Character.isSpaceChar",
        "Character.toUpperCase", "Character.toLowerCase", "Character.getNumericValue", "Character.toString",
        "Character.valueOf", "Character.compare", "Character.forDigit", "Character.digit",
        "Boolean.parseBoolean", "Boolean.toString", "Boolean.valueOf", "Boolean.compare",
        "String.valueOf", "String.format", "String.join", "String.copyValueOf",
        "Arrays.sort", "Arrays.toString", "Arrays.deepToString", "Arrays.fill", "Arrays.asList", "Arrays.copyOf",
        "Arrays.copyOfRange", "Arrays.equals", "Arrays.binarySearch", "Arrays.stream", "Arrays.deepEquals", "Arrays.hashCode",
        "Collections.sort", "Collections.reverse", "Collections.shuffle", "Collections.max", "Collections.min",
        "Collections.swap", "Collections.frequency", "Collections.unmodifiableList", "Collections.emptyList",
        "Collections.nCopies", "Collections.reverseOrder", "Collections.addAll", "Collections.unmodifiableMap",
        "Collections.unmodifiableSet", "Collections.emptyMap", "Collections.singletonList", "Collections.fill", "Collections.binarySearch",
        "List.of", "Set.of", "Map.of", "Map.entry", "List.copyOf",
        "Objects.equals", "Objects.hash", "Objects.requireNonNull", "Objects.isNull", "Objects.nonNull", "Objects.toString",
        "Objects.hashCode", "Objects.requireNonNullElse",
        "System.currentTimeMillis", "System.nanoTime", "System.exit", "System.arraycopy", "System.lineSeparator",
        "System.getProperty", "System.identityHashCode", "System.getenv",
        "Thread.sleep", "Comparator.comparing", "Comparator.comparingInt", "Comparator.comparingDouble",
        "Comparator.comparingLong", "Comparator.naturalOrder", "Comparator.reverseOrder",
        "IntStream.range", "IntStream.rangeClosed", "IntStream.of", "Stream.of", "LongStream.range", "LongStream.rangeClosed",
        "Collectors.toList", "Collectors.toSet", "Collectors.joining", "Collectors.counting", "Collectors.groupingBy",
        "Collectors.summingInt", "Collectors.averagingInt", "Collectors.toMap", "Collectors.averagingDouble",
        "Optional.of", "Optional.empty", "Optional.ofNullable", "ThreadLocalRandom.current",
        "JOptionPane.showMessageDialog", "JOptionPane.showInputDialog", "JOptionPane.showConfirmDialog",
        "JOptionPane.showOptionDialog"
    ]

    func valorJava(_ n: String, _ linea: Int) throws -> CV {
        switch n {
        case "System.out": return .canal(cout)
        case "System.err": return .canal(cerr)
        case "System.in": return .canal(cin)
        case "Math.PI", "StrictMath.PI": return .d(Double.pi, false)
        case "Math.E": return .d(M_E, false)
        case "Integer.MAX_VALUE": return .n(Int64(Int32.max), .int)
        case "Integer.MIN_VALUE": return .n(Int64(Int32.min), .int)
        case "Long.MAX_VALUE": return .n(Int64.max, .long)
        case "Long.MIN_VALUE": return .n(Int64.min, .long)
        case "Short.MAX_VALUE": return .n(32767, .short)
        case "Byte.MAX_VALUE": return .n(127, .char)
        case "Double.MAX_VALUE": return .d(Double.greatestFiniteMagnitude, false)
        case "Double.MIN_VALUE": return .d(Double.leastNonzeroMagnitude, false)
        case "Double.POSITIVE_INFINITY": return .d(Double.infinity, false)
        case "Double.NEGATIVE_INFINITY": return .d(-Double.infinity, false)
        case "Double.NaN": return .d(Double.nan, false)
        case "Float.MAX_VALUE": return .d(Double(Float.greatestFiniteMagnitude), true)
        case "Character.MAX_VALUE": return .n(65535, .jchar)
        case "Character.MIN_VALUE": return .n(0, .jchar)
        case "Boolean.TRUE": return .verdadero
        case "Boolean.FALSE": return .falso
        case "JOptionPane.YES_OPTION", "JOptionPane.OK_OPTION": return .n(0, .int)
        case "JOptionPane.NO_OPTION": return .n(1, .int)
        case "JOptionPane.CANCEL_OPTION": return .n(2, .int)
        case "JOptionPane.INFORMATION_MESSAGE", "JOptionPane.PLAIN_MESSAGE", "JOptionPane.WARNING_MESSAGE",
             "JOptionPane.ERROR_MESSAGE", "JOptionPane.QUESTION_MESSAGE", "JOptionPane.YES_NO_OPTION",
             "JOptionPane.YES_NO_CANCEL_OPTION", "JOptionPane.OK_CANCEL_OPTION":
            return .n(0, .int)
        default:
            if let c = CMaq.coloresGraficos[n.replacingOccurrences(of: "Graficos.", with: "")] { return .n(Int64(c), .int) }
            throw CFallo("no existe '\(n)'", linea)
        }
    }

    func nativaJava(_ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        var args = a
        for k in 0..<args.count { if case .ref(let r) = args[k] { args[k] = try lee(r.l) } }
        let partes = n.split(separator: ".", maxSplits: 1).map(String.init)
        let clase = partes.count > 1 ? partes[0] : ""
        let metodo = partes.count > 1 ? partes[1] : n
        switch clase {
        case "Math", "StrictMath": return try mathJava(metodo, args, linea)
        case "Integer", "Long", "Short", "Byte": return try enteroJava(clase, metodo, args, linea)
        case "Double", "Float": return try realJava(clase, metodo, args, linea)
        case "Character": return try caracterJava(metodo, args, linea)
        case "Boolean": return try booleanoJava(metodo, args, linea)
        case "String": return try cadenaEstatica(metodo, args, linea)
        case "Arrays": return try arraysJava(metodo, args, linea)
        case "Collections": return try coleccionesJava(metodo, args, linea)
        case "List", "Set", "Map": return try fabricaJava(clase, metodo, args, linea)
        case "Objects": return try objetosJava(metodo, args, linea)
        case "System", "Thread": return try sistemaJava(metodo, args, linea)
        case "Comparator": return try comparadorJava(metodo, args, linea)
        case "IntStream", "Stream", "LongStream": return try flujoEstatico(metodo, args, linea)
        case "Collectors": return colector(metodo, args)
        case "Optional": return opcional(metodo == "empty" || (metodo == "ofNullable" && (args.first?.esNulo ?? true)) ? nil : args.first)
        case "ThreadLocalRandom": return .l(CLista("Random", [.n(Int64(bitPattern: javaRandom), .long)]))
        case "JOptionPane": return try joptionPane(metodo, args, linea)
        default:
            if let r = try nativaGrafica(metodo, args, linea) { return r }
            throw CFallo("no existe el método '\(n)'", linea)
        }
    }

    // MARK: Math

    func mathJava(_ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        func d(_ k: Int) throws -> Double { try argReal(a, k, n, linea) }
        switch n {
        case "abs":
            let v = try arg(a, 0, n, linea)
            switch v {
            case .n(let x, let k): let r = CNum.comun(k, .int); return .n(cAjusta(x < 0 ? 0 &- x : x, r), r)
            case .d(let x, let f): return .d(Swift.abs(x), f)
            default: throw CFallo("Math.abs necesita un número", linea)
            }
        case "max", "min":
            let x = try arg(a, 0, n, linea)
            let y = try arg(a, 1, n, linea)
            if case .n = x, case .n = y {
                let c = try compara(x, y, linea)
                let r = (n == "max") == (c >= 0) ? x : y
                if case .n(let v, let k) = r, case .n(_, let k1) = x, case .n(_, let k2) = y { _ = k; return .n(v, CNum.comun(k1, k2)) }
                return r
            }
            let p = try real(x, linea)
            let q = try real(y, linea)
            if p.isNaN || q.isNaN { return .d(Double.nan, false) }
            return .d(n == "max" ? Swift.max(p, q) : Swift.min(p, q), false)
        case "pow": return .d(mPow(try d(0), try d(1)), false)
        case "sqrt": return .d(try d(0).squareRoot(), false)
        case "cbrt": return .d(mCbrt(try d(0)), false)
        case "floor": return .d(mFloor(try d(0)), false)
        case "ceil": return .d(mCeil(try d(0)), false)
        case "rint": return .d((try d(0)).rounded(.toNearestOrEven), false)
        case "round":
            let v = try arg(a, 0, n, linea)
            if case .d(let x, true) = v { return .n(cAjusta(Int64(mFloor(x + 0.5)), .int), .int) }
            let x = try real(v, linea)
            if x.isNaN { return .n(0, .long) }
            return .n(CV.truncaD(mFloor(x + 0.5)), .long)
        case "random": return .d(siguienteRealJava(&javaRandom), false)
        case "sin": return .d(mSin(try d(0)), false)
        case "cos": return .d(mCos(try d(0)), false)
        case "tan": return .d(mTan(try d(0)), false)
        case "asin": return .d(mAsin(try d(0)), false)
        case "acos": return .d(mAcos(try d(0)), false)
        case "atan": return .d(mAtan(try d(0)), false)
        case "atan2": return .d(mAtan2(try d(0), try d(1)), false)
        case "sinh": return .d(mSinh(try d(0)), false)
        case "cosh": return .d(mCosh(try d(0)), false)
        case "tanh": return .d(mTanh(try d(0)), false)
        case "log": return .d(logNatural(try d(0)), false)
        case "log10": return .d(mLog10(try d(0)), false)
        case "log1p": return .d(mLog1p(try d(0)), false)
        case "exp": return .d(mExp(try d(0)), false)
        case "expm1": return .d(mExpm1(try d(0)), false)
        case "hypot": return .d(mHypot(try d(0), try d(1)), false)
        case "signum": let x = try d(0); return .d(x > 0 ? 1 : (x < 0 ? -1 : x), false)
        case "toRadians": return .d(try d(0) * Double.pi / 180, false)
        case "toDegrees": return .d(try d(0) * 180 / Double.pi, false)
        case "floorDiv", "floorMod":
            let x = try argEnt(a, 0, n, linea)
            let y = try argEnt(a, 1, n, linea)
            if y == 0 { throw excepcion("ArithmeticException", "/ by zero", linea) }
            var q = x / y
            if (x % y != 0) && ((x < 0) != (y < 0)) { q -= 1 }
            let k: CNum = a.contains(where: { if case .n(_, .long) = $0 { return true }; return false }) ? .long : .int
            return .n(n == "floorDiv" ? q : x - q * y, k)
        case "addExact", "subtractExact", "multiplyExact":
            let x = try argEnt(a, 0, n, linea)
            let y = try argEnt(a, 1, n, linea)
            let largo = a.contains(where: { if case .n(_, .long) = $0 { return true }; return false })
            let r: (Int64, Bool) = n == "addExact" ? x.addingReportingOverflow(y) : (n == "subtractExact" ? x.subtractingReportingOverflow(y) : x.multipliedReportingOverflow(by: y))
            if r.1 || (!largo && (r.0 > Int64(Int32.max) || r.0 < Int64(Int32.min))) {
                throw excepcion("ArithmeticException", largo ? "long overflow" : "integer overflow", linea)
            }
            return .n(r.0, largo ? .long : .int)
        case "negateExact": return .n(0 &- (try argEnt(a, 0, n, linea)), .int)
        case "toIntExact":
            let x = try argEnt(a, 0, n, linea)
            if x > Int64(Int32.max) || x < Int64(Int32.min) { throw excepcion("ArithmeticException", "integer overflow", linea) }
            return .n(x, .int)
        case "clamp":
            let v = try arg(a, 0, n, linea)
            if try compara(v, try arg(a, 1, n, linea), linea) < 0 { return try arg(a, 1, n, linea) }
            if try compara(v, try arg(a, 2, n, linea), linea) > 0 { return try arg(a, 2, n, linea) }
            return v
        default:
            throw CFallo("Math.\(n) no existe", linea)
        }
    }

    // MARK: java.util.Random (mismo algoritmo: new Random(42) da lo mismo que en Java)

    func siguienteBits(_ s: inout UInt64, _ bits: Int) -> Int32 {
        s = (s &* 0x5DEECE66D &+ 0xB) & ((1 << 48) - 1)
        return Int32(truncatingIfNeeded: Int64(bitPattern: s >> UInt64(48 - bits)))
    }

    func siguienteRealJava(_ s: inout UInt64) -> Double {
        let a = Int64(siguienteBits(&s, 26))
        let b = Int64(siguienteBits(&s, 27))
        return Double((a << 27) + b) * 0x1.0p-53
    }

    func siguienteEnteroJava(_ s: inout UInt64, _ limite: Int32) -> Int32 {
        if limite & (limite &- 1) == 0 {
            return Int32(truncatingIfNeeded: (Int64(limite) &* Int64(siguienteBits(&s, 31))) >> 31)
        }
        var bits: Int32
        var val: Int32
        repeat {
            bits = siguienteBits(&s, 31)
            val = bits % limite
        } while bits &- val &+ (limite &- 1) < 0
        return val
    }

    static func semillaJava(_ x: Int64) -> UInt64 {
        (UInt64(bitPattern: x) ^ 0x5DEECE66D) & ((1 << 48) - 1)
    }

    // MARK: Integer, Long, Double, Character, Boolean

    func enteroJava(_ clase: String, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        let k: CNum = clase == "Long" ? .long : (clase == "Short" ? .short : (clase == "Byte" ? .char : .int))
        switch n {
        case "parseInt", "valueOf", "parseLong", "parseShort", "parseByte", "parseUnsignedInt":
            let v = try arg(a, 0, n, linea)
            if case .n(let x, _) = v { return .n(cAjusta(x, k), k) }
            let s = try texto(v, linea)
            let base = a.count > 1 ? Int(try argEnt(a, 1, n, linea)) : 10
            let limpio = s.hasPrefix("+") ? String(s.dropFirst()) : s
            guard let x = Int64(limpio, radix: base), cAjusta(x, k) == x else {
                throw excepcion("NumberFormatException", "For input string: \"\(s)\"" + (base != 10 ? " under radix \(base)" : ""), linea)
            }
            return .n(x, k)
        case "toString":
            let x = try argEnt(a, 0, n, linea)
            let base = a.count > 1 ? Int(try argEnt(a, 1, n, linea)) : 10
            return cadena(String(x, radix: base))
        case "toBinaryString", "toHexString", "toOctalString":
            let x = try argEnt(a, 0, n, linea)
            let base = n == "toBinaryString" ? 2 : (n == "toHexString" ? 16 : 8)
            let u: UInt64 = k == .long ? UInt64(bitPattern: x) : UInt64(UInt32(truncatingIfNeeded: x))
            return cadena(String(u, radix: base))
        case "max": return .n(Swift.max(try argEnt(a, 0, n, linea), try argEnt(a, 1, n, linea)), k)
        case "min": return .n(Swift.min(try argEnt(a, 0, n, linea), try argEnt(a, 1, n, linea)), k)
        case "sum": return .n(cAjusta(try argEnt(a, 0, n, linea) &+ (try argEnt(a, 1, n, linea)), k), k)
        case "compare":
            let x = try argEnt(a, 0, n, linea)
            let y = try argEnt(a, 1, n, linea)
            return .n(x < y ? -1 : (x == y ? 0 : 1), .int)
        case "bitCount":
            let x = try argEnt(a, 0, n, linea)
            return .n(Int64(k == .long ? x.nonzeroBitCount : UInt32(truncatingIfNeeded: x).nonzeroBitCount), .int)
        case "signum":
            let x = try argEnt(a, 0, n, linea)
            return .n(x > 0 ? 1 : (x < 0 ? -1 : 0), .int)
        case "reverse":
            var x = UInt32(truncatingIfNeeded: try argEnt(a, 0, n, linea))
            var r: UInt32 = 0
            for _ in 0..<32 { r = (r << 1) | (x & 1); x >>= 1 }
            return .n(Int64(Int32(bitPattern: r)), .int)
        case "highestOneBit":
            let x = UInt32(truncatingIfNeeded: try argEnt(a, 0, n, linea))
            return .n(x == 0 ? 0 : Int64(1 << (31 - x.leadingZeroBitCount)), .int)
        case "numberOfTrailingZeros":
            return .n(Int64(UInt32(truncatingIfNeeded: try argEnt(a, 0, n, linea)).trailingZeroBitCount), .int)
        default:
            throw CFallo("\(clase).\(n) no existe", linea)
        }
    }

    func realJava(_ clase: String, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        let f = clase == "Float"
        switch n {
        case "parseDouble", "valueOf", "parseFloat":
            let v = try arg(a, 0, n, linea)
            if case .n = v { return convierte(v, .real(f)) }
            if case .d = v { return convierte(v, .real(f)) }
            let s = try texto(v, linea).trimmingCharacters(in: .whitespaces)
            let limpio = s.hasSuffix("f") || s.hasSuffix("d") || s.hasSuffix("F") || s.hasSuffix("D") ? String(s.dropLast()) : s
            guard let x = Double(limpio) else {
                throw excepcion("NumberFormatException", s.isEmpty ? "empty String" : "For input string: \"\(s)\"", linea)
            }
            return .d(f ? Double(Float(x)) : x, f)
        case "compare":
            let x = try argReal(a, 0, n, linea)
            let y = try argReal(a, 1, n, linea)
            return .n(x < y ? -1 : (x == y ? 0 : 1), .int)
        case "isNaN": return verdad(try argReal(a, 0, n, linea).isNaN)
        case "isInfinite": return verdad(try argReal(a, 0, n, linea).isInfinite)
        case "toString": return cadena(CMaq.javaReal(try argReal(a, 0, n, linea), f))
        case "max": return .d(Swift.max(try argReal(a, 0, n, linea), try argReal(a, 1, n, linea)), f)
        case "min": return .d(Swift.min(try argReal(a, 0, n, linea), try argReal(a, 1, n, linea)), f)
        case "sum": return .d(try argReal(a, 0, n, linea) + (try argReal(a, 1, n, linea)), f)
        default: throw CFallo("\(clase).\(n) no existe", linea)
        }
    }

    func caracterJava(_ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        let x = try argEnt(a, 0, n, linea)
        let u = UnicodeScalar(UInt32(truncatingIfNeeded: Swift.max(0, x))) ?? " "
        let ch = Character(u)
        switch n {
        case "isDigit": return verdad(ch.isNumber && ch.isASCII || ("0"..."9").contains(ch))
        case "isLetter", "isAlphabetic": return verdad(ch.isLetter)
        case "isLetterOrDigit": return verdad(ch.isLetter || ch.isNumber)
        case "isUpperCase": return verdad(ch.isUppercase)
        case "isLowerCase": return verdad(ch.isLowercase)
        case "isWhitespace", "isSpaceChar": return verdad(ch.isWhitespace)
        case "toUpperCase":
            let s = String(ch).uppercased()
            return .n(Int64(s.utf16.count == 1 ? s.utf16.first! : UInt16(x)), .jchar)
        case "toLowerCase":
            let s = String(ch).lowercased()
            return .n(Int64(s.utf16.count == 1 ? s.utf16.first! : UInt16(x)), .jchar)
        case "getNumericValue":
            if let v = ch.wholeNumberValue { return .n(Int64(v), .int) }
            if ch.isLetter && ch.isASCII { return .n(Int64(ch.lowercased().unicodeScalars.first!.value) - 87, .int) }
            return .n(-1, .int)
        case "digit":
            let base = Int(try argEnt(a, 1, n, linea))
            if let v = Int(String(ch), radix: base) { return .n(Int64(v), .int) }
            return .n(-1, .int)
        case "forDigit":
            let base = Int(try argEnt(a, 1, n, linea))
            return .n(Int64(String(x, radix: base).utf16.first ?? 48), .jchar)
        case "toString", "valueOf":
            if n == "valueOf" { return .n(x, .jchar) }
            return .s([UInt16(truncatingIfNeeded: x)])
        case "compare":
            return .n(x - (try argEnt(a, 1, n, linea)), .int)
        default:
            throw CFallo("Character.\(n) no existe", linea)
        }
    }

    func booleanoJava(_ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        let v = try arg(a, 0, n, linea)
        switch n {
        case "parseBoolean", "valueOf":
            if case .n = v { return verdad(aBool(v)) }
            return verdad(try texto(v, linea).lowercased() == "true")
        case "toString": return cadena(aBool(v) ? "true" : "false")
        case "compare":
            let x = aBool(v)
            let y = aBool(try arg(a, 1, n, linea))
            return .n(x == y ? 0 : (x ? 1 : -1), .int)
        default: throw CFallo("Boolean.\(n) no existe", linea)
        }
    }

    // MARK: String estático y formato

    func cadenaEstatica(_ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        switch n {
        case "valueOf", "copyValueOf":
            let v = try arg(a, 0, n, linea)
            if case .p(let p) = v, let mem = p.mem, case .num(.jchar)? = mem.elem {
                var u: [UInt16] = []
                for c in mem.a[p.i...] { if case .n(let x, _) = c { u.append(UInt16(truncatingIfNeeded: x)) } }
                return .s(u)
            }
            return cadena(try texto(v, linea))
        case "format":
            let f = try texto(try arg(a, 0, n, linea), linea)
            return cadena(try formateaJava(f, Array(a.dropFirst()), linea))
        case "join":
            let sep = try texto(try arg(a, 0, n, linea), linea)
            var partes: [String] = []
            if a.count == 2, case .l(let l) = a[1] {
                for x in l.a { partes.append(try texto(x, linea)) }
            } else if a.count == 2, case .p(let p) = a[1], let mem = p.mem {
                for x in mem.a[p.i...] { partes.append(try texto(x, linea)) }
            } else if a.count == 2, case .m(let mp) = a[1] {
                for k in mp.claves { if let (kv, _) = mp.vals[k] { partes.append(try texto(kv, linea)) } }
            } else {
                for x in a.dropFirst() { partes.append(try texto(x, linea)) }
            }
            return cadena(partes.joined(separator: sep))
        default:
            throw CFallo("String.\(n) no existe", linea)
        }
    }

    /// String.format / printf de Java.
    func formateaJava(_ fmt: String, _ a: [CV], _ linea: Int) throws -> String {
        var out = ""
        let c = Array(fmt.unicodeScalars)
        var i = 0
        var k = 0
        while i < c.count {
            if c[i] != "%" { out.unicodeScalars.append(c[i]); i += 1; continue }
            i += 1
            guard i < c.count else { break }
            var spec = ""
            while i < c.count, "-+ 0,#(".unicodeScalars.contains(c[i]) || ("0"..."9").contains(c[i]) || c[i] == "." || c[i] == "$" {
                spec.unicodeScalars.append(c[i])
                i += 1
            }
            guard i < c.count else { break }
            let conv = c[i]
            i += 1
            if conv == "n" { out += "\n"; continue }
            if conv == "%" { out += "%"; continue }
            // %2$s: argumento por posición
            if let dolar = spec.firstIndex(of: "$"), let pos = Int(spec[..<dolar]) {
                k = pos - 1
                spec = String(spec[spec.index(after: dolar)...])
            }
            let v: CV = k < a.count ? a[k] : .nulo
            k += 1
            out += try unFormatoJava(conv, spec, v, linea)
        }
        return out
    }

    func unFormatoJava(_ conv: UnicodeScalar, _ spec0: String, _ v0: CV, _ linea: Int) throws -> String {
        var v = v0
        if case .ref(let r) = v { v = try lee(r.l) }
        let agrupa = spec0.contains(",")
        let spec = spec0.replacingOccurrences(of: ",", with: "").replacingOccurrences(of: "(", with: "")
        let izq = spec.contains("-")
        let ancho = CMaq.anchoFormato(spec)
        func pad(_ s: String) -> String {
            guard ancho > s.count else { return s }
            return izq ? s + String(repeating: " ", count: ancho - s.count) : String(repeating: " ", count: ancho - s.count) + s
        }
        switch conv {
        case "d":
            guard case .n(let x, _) = v else {
                throw excepcion("IllegalFormatConversionException", "d != \(v.tipoNombre)", linea)
            }
            var s = String(format: "%" + spec + "lld", x)
            if agrupa { s = CMaq.agrupaMiles(s) }
            return s
        case "f", "e", "E", "g":
            let x = try real(v, linea)
            var s = String(format: "%" + (spec.contains(".") ? spec : spec + ".6") + String(conv), CMaq.redondeoJava(x, spec))
            if agrupa { s = CMaq.agrupaMiles(s) }
            return s
        case "s", "S":
            var s = try texto(v, linea)
            if let p = spec.split(separator: ".").last, spec.contains("."), let n = Int(p) { s = String(s.prefix(n)) }
            if conv == "S" { s = s.uppercased() }
            return pad(s)
        case "c":
            if case .n(let x, _) = v { return pad(String(decoding: [UInt16(truncatingIfNeeded: x)], as: UTF16.self)) }
            return pad(try texto(v, linea))
        case "b", "B":
            return pad(aBool(v) && !v.esNulo ? "true" : "false")
        case "x", "X", "o":
            let x = try entero(v, linea)
            let u: UInt64 = { if case .n(_, .long) = v { return UInt64(bitPattern: x) }; return UInt64(UInt32(truncatingIfNeeded: x)) }()
            return String(format: "%" + spec + "ll" + String(conv), u)
        case "h":
            return pad(String(UInt(bitPattern: try texto(v, linea).hashValue) & 0xFFFFFFF, radix: 16))
        default:
            throw excepcion("UnknownFormatConversionException", "Conversion = '\(conv)'", linea)
        }
    }

    static func anchoFormato(_ spec: String) -> Int {
        var digitos = ""
        var empezo = false
        for ch in spec {
            if ch == "." { break }
            if ch.isNumber {
                if ch == "0" && !empezo { continue }
                empezo = true
                digitos.append(ch)
            }
        }
        return Int(digitos) ?? 0
    }

    /// Java redondea "hacia arriba" en los empates (2.5 → 3 con %.0f).
    static func redondeoJava(_ x: Double, _ spec: String) -> Double {
        guard let p = spec.split(separator: ".").last, spec.contains("."), let dec = Int(p), dec < 15 else { return x }
        let m = mPow(10.0, Double(dec))
        let y = x * m
        let r = (y >= 0 ? mFloor(y + 0.5) : mCeil(y - 0.5)) / m
        // solo si el binario quedó justo en el medio
        if Swift.abs(y - mTrunc(y)) == 0.5 || Swift.abs(Swift.abs(y - mTrunc(y)) - 0.5) < 1e-9 { return r }
        return x
    }

    static func agrupaMiles(_ s: String) -> String {
        var t = s
        var signo = ""
        while let f = t.first, !f.isNumber { signo.append(f); t.removeFirst() }
        var entera = t
        var resto = ""
        if let p = t.firstIndex(of: ".") {
            entera = String(t[..<p])
            resto = String(t[p...])
        }
        let digitos = entera.trimmingCharacters(in: .whitespaces)
        var out = ""
        for (k, ch) in digitos.reversed().enumerated() {
            if k > 0 && k % 3 == 0 { out.append(",") }
            out.append(ch)
        }
        return signo + String(out.reversed()) + resto
    }

    // MARK: Arrays

    func arraysJava(_ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        func arreglo(_ k: Int) throws -> CPtr {
            let v = try arg(a, k, n, linea)
            guard case .p(let p) = v, p.mem != nil else {
                if case .nulo = v { throw excepcion("NullPointerException", "el arreglo es null", linea) }
                throw CFallo("Arrays.\(n) necesita un arreglo", linea)
            }
            return p
        }
        switch n {
        case "sort":
            let p = try arreglo(0)
            let mem = p.mem!
            var lo = p.i
            var hi = mem.a.count
            var cmp: CV?
            if a.count >= 3, case .n = a[1] {
                lo = p.i + Int(try argEnt(a, 1, n, linea))
                hi = p.i + Int(try argEnt(a, 2, n, linea))
                if a.count > 3 { cmp = a[3] }
            } else if a.count == 2 { cmp = a[1] }
            var vals = Array(mem.a[lo..<hi])
            if let c = cmp {
                try ordena(&vals) { x, y in try self.entero(try self.invoca(c, [x, y], linea), linea) < 0 }
            } else {
                try ordena(&vals) { x, y in try self.compara(x, y, linea) < 0 }
            }
            for (k, v) in vals.enumerated() { mem.a[lo + k] = v }
            return .vacio
        case "toString":
            let v = try arg(a, 0, n, linea)
            guard case .p(let p) = v, let mem = p.mem else { return cadena("null") }
            var partes: [String] = []
            for x in mem.a[p.i...] { partes.append(try texto(x, linea)) }
            return cadena("[" + partes.joined(separator: ", ") + "]")
        case "deepToString":
            return cadena(try textoProfundo(try arg(a, 0, n, linea), linea))
        case "fill":
            let p = try arreglo(0)
            let mem = p.mem!
            if a.count == 4 {
                let lo = Int(try argEnt(a, 1, n, linea))
                let hi = Int(try argEnt(a, 2, n, linea))
                for k in lo..<hi { mem.a[p.i + k] = convierte(a[3], mem.elem ?? .auto) }
            } else {
                let v = convierte(try arg(a, 1, n, linea), mem.elem ?? .auto)
                for k in p.i..<mem.a.count { mem.a[k] = v }
            }
            return .vacio
        case "asList":
            if a.count == 1, case .p(let p) = a[0], let mem = p.mem { return .l(CLista("ArrayList", Array(mem.a[p.i...]))) }
            return .l(CLista("ArrayList", a))
        case "copyOf", "copyOfRange":
            let p = try arreglo(0)
            let mem = p.mem!
            var lo = 0
            var hi = Int(try argEnt(a, 1, n, linea))
            if n == "copyOfRange" { lo = hi; hi = Int(try argEnt(a, 2, n, linea)) }
            var celdas: [CV] = []
            let def = try valorCampo(mem.elem ?? .auto)
            for k in lo..<hi { celdas.append(p.i + k < mem.a.count ? mem.a[p.i + k] : def) }
            let x = CMem(celdas)
            x.elem = mem.elem
            x.esArreglo = true
            return .p(CPtr(mem: x, i: 0))
        case "equals", "deepEquals":
            let x = try arg(a, 0, n, linea)
            let y = try arg(a, 1, n, linea)
            guard case .p(let p) = x, case .p(let q) = y, let m1 = p.mem, let m2 = q.mem else { return verdad(x.esNulo && y.esNulo) }
            let v1 = Array(m1.a[p.i...])
            let v2 = Array(m2.a[q.i...])
            guard v1.count == v2.count else { return verdad(false) }
            for k in 0..<v1.count {
                if n == "deepEquals", case .p = v1[k] {
                    if !aBool(try arraysJava("deepEquals", [v1[k], v2[k]], linea)) { return verdad(false) }
                } else if !(try iguales(v1[k], v2[k], linea)) { return verdad(false) }
            }
            return verdad(true)
        case "binarySearch":
            let p = try arreglo(0)
            let vals = Array(p.mem!.a[p.i...])
            let clave = try arg(a, 1, n, linea)
            var lo = 0
            var hi = vals.count - 1
            while lo <= hi {
                let mid = (lo + hi) >> 1
                let c = try compara(vals[mid], clave, linea)
                if c < 0 { lo = mid + 1 } else if c > 0 { hi = mid - 1 } else { return .n(Int64(mid), .int) }
            }
            return .n(Int64(-(lo + 1)), .int)
        case "stream":
            let p = try arreglo(0)
            return .l(CLista("Stream", Array(p.mem!.a[p.i...])))
        case "hashCode":
            let p = try arreglo(0)
            var h: Int64 = 1
            for x in p.mem!.a[p.i...] { h = cAjusta(31 &* h &+ (try hashJava(x, linea)), .int) }
            return .n(h, .int)
        default:
            throw CFallo("Arrays.\(n) no existe", linea)
        }
    }

    func textoProfundo(_ v: CV, _ linea: Int) throws -> String {
        guard case .p(let p) = v, let mem = p.mem else { return try texto(v, linea) }
        var partes: [String] = []
        for x in mem.a[p.i...] { partes.append(try textoProfundo(x, linea)) }
        return "[" + partes.joined(separator: ", ") + "]"
    }

    func hashJava(_ v: CV, _ linea: Int) throws -> Int64 {
        switch v {
        case .n(let x, let k):
            if k == .long { return Int64(Int32(truncatingIfNeeded: x ^ (x >> 32))) }
            if k == .bool { return x != 0 ? 1231 : 1237 }
            return x
        case .d(let d, _):
            let b = Int64(bitPattern: d.bitPattern)
            return Int64(Int32(truncatingIfNeeded: b ^ (b >> 32)))
        case .s(let u):
            var h: Int64 = 0
            for c in u { h = cAjusta(31 &* h &+ Int64(c), .int) }
            return h
        case .o(let o):
            if let fn = o.clase.metodo("hashCode", 0) { return try entero(try llama(fn, [], este: o, padre: o.exterior, linea: linea), linea) }
            return Int64(hashObjeto(o))
        case .nulo: return 0
        default: return 0
        }
    }

    // MARK: Collections, List.of…

    func coleccionesJava(_ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        func lista(_ k: Int) throws -> CLista {
            guard case .l(let l) = try arg(a, k, n, linea) else { throw CFallo("Collections.\(n) necesita una lista", linea) }
            return l
        }
        switch n {
        case "sort":
            let l = try lista(0)
            var vals = l.a
            if a.count > 1, !a[1].esNulo {
                let c = a[1]
                try ordena(&vals) { x, y in try self.entero(try self.invoca(c, [x, y], linea), linea) < 0 }
            } else {
                try ordena(&vals) { x, y in try self.compara(x, y, linea) < 0 }
            }
            l.a = vals
            return .vacio
        case "reverse":
            try lista(0).a.reverse()
            return .vacio
        case "shuffle":
            let l = try lista(0)
            var s = javaRandom
            if a.count > 1, case .l(let r) = a[1], case .n(let x, _)? = r.a.first { s = UInt64(bitPattern: x) }
            for k in stride(from: l.a.count - 1, to: 0, by: -1) {
                let j = Int(siguienteEnteroJava(&s, Int32(k + 1)))
                l.a.swapAt(k, j)
            }
            javaRandom = s
            return .vacio
        case "max", "min":
            let vals = try valoresColeccion(try arg(a, 0, n, linea), linea)
            guard var mejor = vals.first else { throw excepcion("NoSuchElementException", "colección vacía", linea) }
            for v in vals.dropFirst() {
                let c: Int
                if a.count > 1 { c = Int(try entero(try invoca(a[1], [v, mejor], linea), linea)) }
                else { c = try compara(v, mejor, linea) }
                if (n == "max" && c > 0) || (n == "min" && c < 0) { mejor = v }
            }
            return mejor
        case "swap":
            let l = try lista(0)
            let i = Int(try argEnt(a, 1, n, linea))
            let j = Int(try argEnt(a, 2, n, linea))
            guard i >= 0, j >= 0, i < l.a.count, j < l.a.count else { throw excepcion("IndexOutOfBoundsException", "Index \(Swift.max(i, j)) out of bounds for length \(l.a.count)", linea) }
            l.a.swapAt(i, j)
            return .vacio
        case "frequency":
            let vals = try valoresColeccion(try arg(a, 0, n, linea), linea)
            let o = try arg(a, 1, n, linea)
            var k = 0
            for v in vals where try iguales(v, o, linea) { k += 1 }
            return .n(Int64(k), .int)
        case "unmodifiableList", "unmodifiableMap", "unmodifiableSet", "synchronizedList":
            return try arg(a, 0, n, linea)
        case "emptyList": return .l(CLista("ArrayList"))
        case "emptyMap": return try nuevoContenedor("HashMap", [])
        case "singletonList": return .l(CLista("ArrayList", [try arg(a, 0, n, linea)]))
        case "nCopies":
            let k = Int(try argEnt(a, 0, n, linea))
            return .l(CLista("ArrayList", [CV](repeating: try arg(a, 1, n, linea), count: Swift.max(0, k))))
        case "reverseOrder":
            if let c = a.first {
                return .f(CCierre(fn: nil, marco: nil, este: nil, nombre: "reverseOrder", nat: { m, x in
                    try m.invoca(c, [x[1], x[0]], linea)
                }))
            }
            return try comparadorJava("reverseOrder", [], linea)
        case "addAll":
            let l = try lista(0)
            l.a += a.dropFirst()
            return .verdadero
        case "fill":
            let l = try lista(0)
            let v = try arg(a, 1, n, linea)
            for k in 0..<l.a.count { l.a[k] = v }
            return .vacio
        case "binarySearch":
            let l = try lista(0)
            let clave = try arg(a, 1, n, linea)
            var lo = 0
            var hi = l.a.count - 1
            while lo <= hi {
                let mid = (lo + hi) >> 1
                let c = try compara(l.a[mid], clave, linea)
                if c < 0 { lo = mid + 1 } else if c > 0 { hi = mid - 1 } else { return .n(Int64(mid), .int) }
            }
            return .n(Int64(-(lo + 1)), .int)
        default:
            throw CFallo("Collections.\(n) no existe", linea)
        }
    }

    func valoresColeccion(_ v: CV, _ linea: Int) throws -> [CV] {
        switch v {
        case .l(let l): return l.a
        case .m(let mp): return mp.claves.compactMap { mp.vals[$0]?.0 }
        case .p(let p): return p.mem.map { Array($0.a[p.i...]) } ?? []
        case .nulo: throw excepcion("NullPointerException", "colección null", linea)
        default: throw CFallo("se esperaba una colección y llegó \(v.tipoNombre)", linea)
        }
    }

    func fabricaJava(_ clase: String, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        switch (clase, n) {
        case ("List", "of"), ("List", "copyOf"):
            if n == "copyOf" { return .l(CLista("ArrayList", try valoresColeccion(try arg(a, 0, n, linea), linea))) }
            if a.count == 1, case .p(let p) = a[0], let mem = p.mem { return .l(CLista("ArrayList", Array(mem.a[p.i...]))) }
            return .l(CLista("ArrayList", a))
        case ("Set", "of"):
            let mp = CMapa("HashSet", ordenado: false, esSet: true)
            for v in a { mp.pon(try clave(v, linea), v, v) }
            return .m(mp)
        case ("Map", "of"):
            let mp = CMapa("HashMap", ordenado: false, esSet: false)
            var k = 0
            while k + 1 < a.count { mp.pon(try clave(a[k], linea), a[k], a[k + 1]); k += 2 }
            return .m(mp)
        case ("Map", "entry"):
            return par(try arg(a, 0, n, linea), try arg(a, 1, n, linea))
        default:
            throw CFallo("\(clase).\(n) no existe", linea)
        }
    }

    func objetosJava(_ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        switch n {
        case "equals":
            let x = try arg(a, 0, n, linea)
            let y = try arg(a, 1, n, linea)
            if x.esNulo || y.esNulo { return verdad(x.esNulo && y.esNulo) }
            return verdad(try iguales(x, y, linea))
        case "hash":
            var h: Int64 = 1
            for x in a { h = cAjusta(31 &* h &+ (try hashJava(x, linea)), .int) }
            return .n(h, .int)
        case "hashCode": return .n(try hashJava(try arg(a, 0, n, linea), linea), .int)
        case "requireNonNull":
            let x = try arg(a, 0, n, linea)
            if x.esNulo { throw excepcion("NullPointerException", a.count > 1 ? try texto(a[1], linea) : "", linea) }
            return x
        case "requireNonNullElse":
            let x = try arg(a, 0, n, linea)
            return x.esNulo ? try arg(a, 1, n, linea) : x
        case "isNull": return verdad(try arg(a, 0, n, linea).esNulo)
        case "nonNull": return verdad(!(try arg(a, 0, n, linea).esNulo))
        case "toString": return cadena(try texto(try arg(a, 0, n, linea), linea))
        default: throw CFallo("Objects.\(n) no existe", linea)
        }
    }

    func sistemaJava(_ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        switch n {
        case "currentTimeMillis": return .n(Int64(Date().timeIntervalSince1970 * 1000), .long)
        case "nanoTime": return .n(Int64(Date().timeIntervalSince(inicio) * 1e9), .long)
        case "exit": throw CSalida(codigo: Int(try argEnt(a, 0, n, linea)))
        case "lineSeparator": return cadena("\n")
        case "getProperty": return cadena(try texto(try arg(a, 0, n, linea), linea) == "user.name" ? "mobile" : "")
        case "getenv": return .nulo
        case "identityHashCode":
            if case .o(let o) = try arg(a, 0, n, linea) { return .n(Int64(hashObjeto(o)), .int) }
            return .n(0, .int)
        case "arraycopy":
            guard case .p(let src) = try arg(a, 0, n, linea), case .p(let dst) = try arg(a, 2, n, linea),
                  let ms = src.mem, let md = dst.mem else { throw excepcion("NullPointerException", "arraycopy con null", linea) }
            let sp = Int(try argEnt(a, 1, n, linea))
            let dp = Int(try argEnt(a, 3, n, linea))
            let len = Int(try argEnt(a, 4, n, linea))
            guard sp >= 0, dp >= 0, len >= 0, src.i + sp + len <= ms.a.count, dst.i + dp + len <= md.a.count else {
                throw excepcion("ArrayIndexOutOfBoundsException", "arraycopy: last source index \(sp + len) out of bounds for length \(ms.a.count)", linea)
            }
            let trozo = Array(ms.a[(src.i + sp)..<(src.i + sp + len)])
            for k in 0..<len { md.a[dst.i + dp + k] = trozo[k] }
            return .vacio
        case "sleep":
            vuelca()
            try duerme(try argReal(a, 0, n, linea) / 1000)
            return .vacio
        default:
            throw CFallo("System.\(n) no existe", linea)
        }
    }

    // MARK: Comparator, streams y Optional

    func comparadorJava(_ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        switch n {
        case "comparing", "comparingInt", "comparingDouble", "comparingLong":
            let clave = try arg(a, 0, n, linea)
            let segundo: CV? = a.count > 1 ? a[1] : nil
            return .f(CCierre(fn: nil, marco: nil, este: nil, nombre: "comparing", nat: { m, x in
                let ka = try m.invoca(clave, [x[0]], linea)
                let kb = try m.invoca(clave, [x[1]], linea)
                if let s = segundo { return try m.invoca(s, [ka, kb], linea) }
                return .n(Int64(try m.compara(ka, kb, linea)), .int)
            }))
        case "naturalOrder":
            return .f(CCierre(fn: nil, marco: nil, este: nil, nombre: "naturalOrder", nat: { m, x in
                .n(Int64(try m.compara(x[0], x[1], linea)), .int)
            }))
        case "reverseOrder":
            return .f(CCierre(fn: nil, marco: nil, este: nil, nombre: "reverseOrder", nat: { m, x in
                .n(Int64(try m.compara(x[1], x[0], linea)), .int)
            }))
        default:
            throw CFallo("Comparator.\(n) no existe", linea)
        }
    }

    func flujoEstatico(_ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        switch n {
        case "range", "rangeClosed":
            let lo = try argEnt(a, 0, n, linea)
            var hi = try argEnt(a, 1, n, linea)
            if n == "rangeClosed" { hi += 1 }
            if hi - lo > 50_000_000 { throw CFallo("rango demasiado grande", linea) }
            var vals: [CV] = []
            if hi > lo { vals.reserveCapacity(Int(hi - lo)) }
            var x = lo
            while x < hi { vals.append(.n(x, .int)); x += 1 }
            return .l(CLista("Stream", vals))
        case "of":
            if a.count == 1, case .p(let p) = a[0], let mem = p.mem { return .l(CLista("Stream", Array(mem.a[p.i...]))) }
            return .l(CLista("Stream", a))
        default:
            throw CFallo("Stream.\(n) no existe", linea)
        }
    }

    func colector(_ n: String, _ a: [CV]) -> CV {
        let l = CLista("Collector:" + n, a)
        return .l(l)
    }

    func opcional(_ v: CV?) -> CV {
        .l(CLista("Optional", v.map { [$0] } ?? []))
    }

    // MARK: new de clases de la biblioteca

    func nuevoNativoJava(_ n: String, _ targs: [CTipo], _ a: [CV], _ linea: Int) throws -> CV {
        var args = a
        for k in 0..<args.count { if case .ref(let r) = args[k] { args[k] = try lee(r.l) } }
        switch n {
        case "ArrayList", "LinkedList", "Vector", "ArrayDeque", "Stack", "List", "Deque", "Queue":
            let l = CLista(n == "List" ? "ArrayList" : (n == "Deque" || n == "Queue" ? "ArrayDeque" : n))
            if let x = args.first {
                if case .n = x {} else { l.a = try valoresColeccion(x, linea) }
            }
            return .l(l)
        case "PriorityQueue":
            let l = CLista("PriorityQueue")
            for x in args {
                switch x {
                case .f(let c): l.cmp = c
                case .o: l.cmp = cierreDe(x)
                case .l, .m:
                    l.a = try valoresColeccion(x, linea)
                    l.a = try ordenados(l, linea)
                default: break
                }
            }
            return .l(l)
        case "HashMap", "TreeMap", "LinkedHashMap", "HashSet", "TreeSet", "LinkedHashSet", "Hashtable", "Map", "Set":
            let nombre = n == "Hashtable" ? "HashMap" : (n == "Map" ? "HashMap" : (n == "Set" ? "HashSet" : n))
            guard case .m(let mp) = try nuevoContenedor(nombre, targs) else { return .nulo }
            if let x = args.first {
                switch x {
                case .m(let otro):
                    for k in otro.claves { if let par = otro.vals[k] { mp.pon(k, par.0, par.1) } }
                case .l, .p:
                    for v in try valoresColeccion(x, linea) { mp.pon(try clave(v, linea), v, v) }
                case .f(let c):
                    mp.cmp = c
                default: break
                }
            }
            return .m(mp)
        case "StringBuilder", "StringBuffer":
            var u: [UInt16] = []
            if let x = args.first, !(x.esEntero) { u = unidades(try texto(x, linea)) }
            return .l(CLista("StringBuilder", [.s(u)]))
        case "Random":
            let s: UInt64 = args.first.map { CMaq.semillaJava((try? entero($0, linea)) ?? 0) } ?? javaRandom &+ 0x9E3779B97F4A7C15
            if args.isEmpty { javaRandom = javaRandom &* 6364136223846793005 &+ 1 }
            return .l(CLista("Random", [.n(Int64(bitPattern: s & ((1 << 48) - 1)), .long)]))
        case "Scanner":
            let c = CCanal(.entrada)
            c.nombre = "Scanner"
            if let x = args.first {
                switch x {
                case .canal(let otro): c.ent = otro.ent ?? entrada
                case .s(let u): c.ent = CEntrada(texto(u))
                case .l(let f) where f.k == "File":
                    let ruta = try texto(f.a[0], linea)
                    guard let b = leerArchivo(ruta) else { throw excepcion("FileNotFoundException", ruta + " (No such file or directory)", linea) }
                    c.ent = CEntrada(String(decoding: b, as: UTF8.self))
                default: c.ent = entrada
                }
            } else { c.ent = entrada }
            return .canal(c)
        case "BufferedReader", "InputStreamReader", "FileReader":
            let c = CCanal(.entrada)
            c.nombre = "BufferedReader"
            if let x = args.first {
                switch x {
                case .canal(let otro): c.ent = otro.ent ?? entrada
                case .s(let u):
                    let ruta = texto(u)
                    guard let b = leerArchivo(ruta) else { throw excepcion("FileNotFoundException", ruta + " (No such file or directory)", linea) }
                    c.ent = CEntrada(String(decoding: b, as: UTF8.self))
                case .l(let f) where f.k == "File":
                    let ruta = try texto(f.a[0], linea)
                    guard let b = leerArchivo(ruta) else { throw excepcion("FileNotFoundException", ruta + " (No such file or directory)", linea) }
                    c.ent = CEntrada(String(decoding: b, as: UTF8.self))
                default: c.ent = entrada
                }
            } else { c.ent = entrada }
            return .canal(c)
        case "FileWriter", "PrintWriter", "BufferedWriter", "FileOutputStream", "PrintStream":
            if let x = args.first, case .canal(let otro) = x { return .canal(otro) }
            guard let x = args.first else { return .canal(cout) }
            var ruta: String
            if case .l(let f) = x, f.k == "File" { ruta = try texto(f.a[0], linea) } else { ruta = try texto(x, linea) }
            let anexar = args.count > 1 && aBool(args[1])
            guard case .canal(let c) = try abreArchivo(ruta, anexar ? "a" : "w", linea) else {
                throw excepcion("IOException", "no se puede escribir \(ruta)", linea)
            }
            c.nombre = ruta
            return .canal(c)
        case "File":
            return .l(CLista("File", [cadena(try texto(try arg(args, 0, n, linea), linea))]))
        case "Object":
            return .o(CObj(clasePar(), CMem([.nulo, .nulo])))
        case "Thread":
            return .l(CLista("Thread", args))
        case "int", "Integer", "Long", "Double", "String":
            return args.first ?? .n(0, .int)
        case "AtomicInteger", "AtomicLong":
            return .l(CLista("AtomicInteger", [args.first ?? .n(0, .int)]))
        default:
            throw CFallo("no sé crear un \(n) (clase desconocida)", linea)
        }
    }

    func cadenaJavaDe(_ v: CV, _ a: [CV], _ linea: Int) throws -> CV {
        if case .p(let p) = v, let mem = p.mem {
            var u: [UInt16] = []
            var desde = p.i
            var hasta = mem.a.count
            if a.count == 3 {
                desde = p.i + Int(try entero(a[1], linea))
                hasta = desde + Int(try entero(a[2], linea))
            }
            for c in mem.a[desde..<hasta] { if case .n(let x, _) = c { u.append(UInt16(truncatingIfNeeded: x)) } }
            return .s(u)
        }
        if case .l(let l) = v, l.k == "StringBuilder" { return l.a[0] }
        return cadena(try texto(v, linea))
    }
}

extension CV {
    var esEntero: Bool { if case .n = self { return true }; return false }
}
