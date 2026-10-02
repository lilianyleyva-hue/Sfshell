import Foundation

// ============================================================
// MARK: - WAT: texto de WebAssembly → binario
// ============================================================
// Ensamblador del formato de texto (.wat). Entiende:
//   type, import, func, table, memory, global, export, start, elem, data
//   nombres con $ (funciones, locales, parámetros, globales, tipos,
//   etiquetas…), exportaciones e importaciones en línea,
//   instrucciones planas (i32.add) y plegadas ((i32.add (local.get 0) …)),
//   block/loop/if con etiquetas, números decimales/hex con _ , floats
//   (hex, inf, nan, nan:0x…), offset= align=, textos con \n \t \hh \u{…},
//   comentarios ;; … y (; … ;).
// Los errores dicen la línea: "línea 12: instrucción desconocida 'i32.ad'".

struct WatError: Error {
    let m: String
}

struct WatNodo {
    enum Clase { case lista, atomo, cadena }
    let clase: Clase
    let texto: String
    let bytes: [UInt8]
    let hijos: [WatNodo]
    let linea: Int

    /// Primera palabra de una lista: (func …) → "func".
    var cabeza: String? {
        guard clase == .lista, let p = hijos.first, p.clase == .atomo else { return nil }
        return p.texto
    }

    var esId: Bool { clase == .atomo && texto.hasPrefix("$") }

    /// Índice numérico o $nombre (para inmediatos opcionales).
    var esIndice: Bool {
        guard clase == .atomo, let c = texto.unicodeScalars.first else { return false }
        return c == "$" || (c >= "0" && c <= "9")
    }
}

/// Inmediatos de cada instrucción.
enum WatImm {
    case nada, local, global, funcion, etiqueta, tablaBr, memoria(UInt32)
    case i32, i64, f32, f64, callInd, memIdx, memIdx2, memInit, datoIdx
    case tabla, tabla2, tablaInit, elemIdx, refNull, select
}

struct WatOp {
    let bytes: [UInt8]
    let imm: WatImm
}

/// Una func/table/memory/global ya con su cabecera leída.
struct WatDef {
    let nodo: WatNodo
    var nombre: String?
    var exports: [String]
    var impModulo: String?
    var impCampo: String
    var resto: Int
}

final class WatEnsamblador {
    private var tipos: [WasmFuncTipo] = []
    private var nTipo: [String: Int] = [:]
    private var nFunc: [String: Int] = [:]
    private var nTabla: [String: Int] = [:]
    private var nMem: [String: Int] = [:]
    private var nGlobal: [String: Int] = [:]
    private var nElem: [String: Int] = [:]
    private var nDato: [String: Int] = [:]
    private var locales: [String: Int] = [:]
    private var etiquetas: [String?] = []
    private var usaDataCount = false

    /// Ensambla un módulo en texto. Lanza WatError con la línea.
    static func ensambla(_ texto: String) throws -> [UInt8] {
        let e = WatEnsamblador()
        return try e.modulo(try WatEnsamblador.analiza(texto))
    }

    private func err(_ n: WatNodo, _ m: String) -> WatError {
        WatError(m: "línea \(n.linea): \(m)")
    }

    // ============================================================
    // MARK: Lectura (tokens → árbol)
    // ============================================================

    static func analiza(_ texto: String) throws -> [WatNodo] {
        let b: [UInt8] = Array(texto.utf8)
        var pila: [([WatNodo], Int)] = [([], 1)]
        var i = 0
        var linea = 1
        while i < b.count {
            let c = b[i]
            switch c {
            case 0x0A:
                linea += 1
                i += 1
            case 0x20, 0x09, 0x0D:
                i += 1
            case 0x28:
                if i + 1 < b.count && b[i + 1] == 0x3B {
                    i = try comentarioBloque(b, i, &linea)
                } else {
                    pila.append(([], linea))
                    i += 1
                }
            case 0x29:
                guard pila.count > 1 else { throw WatError(m: "línea \(linea): sobra un ')'") }
                let (hs, l) = pila.removeLast()
                pila[pila.count - 1].0.append(WatNodo(clase: .lista, texto: "", bytes: [], hijos: hs, linea: l))
                i += 1
            case 0x3B:
                guard i + 1 < b.count && b[i + 1] == 0x3B else { throw WatError(m: "línea \(linea): ';' suelto") }
                while i < b.count && b[i] != 0x0A { i += 1 }
            case 0x22:
                let l = linea
                let (bytes, sig) = try cadena(b, i + 1, &linea)
                pila[pila.count - 1].0.append(WatNodo(clase: .cadena, texto: "", bytes: bytes, hijos: [], linea: l))
                i = sig
            default:
                var j = i
                while j < b.count {
                    let d = b[j]
                    if d == 0x20 || d == 0x09 || d == 0x0A || d == 0x0D || d == 0x28 || d == 0x29 || d == 0x22 || d == 0x3B { break }
                    j += 1
                }
                let t = String(decoding: b[i ..< j], as: UTF8.self)
                pila[pila.count - 1].0.append(WatNodo(clase: .atomo, texto: t, bytes: [], hijos: [], linea: linea))
                i = j
            }
        }
        guard pila.count == 1 else {
            throw WatError(m: "línea \(pila[pila.count - 1].1): falta cerrar este paréntesis con ')'")
        }
        return pila[0].0
    }

    /// (; … ;) con anidamiento. Devuelve la posición después del cierre.
    private static func comentarioBloque(_ b: [UInt8], _ desde: Int, _ linea: inout Int) throws -> Int {
        let inicio = linea
        var i = desde + 2
        var nivel = 1
        while i < b.count {
            if b[i] == 0x0A { linea += 1 }
            if b[i] == 0x28 && i + 1 < b.count && b[i + 1] == 0x3B {
                nivel += 1
                i += 2
                continue
            }
            if b[i] == 0x3B && i + 1 < b.count && b[i + 1] == 0x29 {
                nivel -= 1
                i += 2
                if nivel == 0 { return i }
                continue
            }
            i += 1
        }
        throw WatError(m: "línea \(inicio): comentario (; sin cerrar")
    }

    private static func hex(_ c: UInt8) -> Int? {
        switch c {
        case 0x30 ... 0x39: return Int(c) - 0x30
        case 0x41 ... 0x46: return Int(c) - 0x41 + 10
        case 0x61 ... 0x66: return Int(c) - 0x61 + 10
        default: return nil
        }
    }

    /// Texto entre comillas con escapes. 'desde' es justo después de la comilla.
    private static func cadena(_ b: [UInt8], _ desde: Int, _ linea: inout Int) throws -> ([UInt8], Int) {
        var r: [UInt8] = []
        var i = desde
        let malo = WatError(m: "línea \(linea): escape no válido en el texto")
        while i < b.count {
            let c = b[i]
            if c == 0x22 { return (r, i + 1) }
            if c == 0x0A { linea += 1 }
            if c != 0x5C {
                r.append(c)
                i += 1
                continue
            }
            guard i + 1 < b.count else { throw malo }
            let e = b[i + 1]
            i += 2
            switch e {
            case 0x6E: r.append(0x0A)
            case 0x74: r.append(0x09)
            case 0x72: r.append(0x0D)
            case 0x22, 0x27, 0x5C: r.append(e)
            case 0x75:
                guard i < b.count, b[i] == 0x7B else { throw malo }
                var v = 0
                i += 1
                while i < b.count, b[i] != 0x7D {
                    guard let h = hex(b[i]), v < 0x110000 else { throw malo }
                    v = v * 16 + h
                    i += 1
                }
                i += 1
                guard let u = Unicode.Scalar(v) else { throw malo }
                r.append(contentsOf: Array(String(Character(u)).utf8))
            default:
                guard let h1 = hex(e), i < b.count, let h2 = hex(b[i]) else { throw malo }
                r.append(UInt8(h1 * 16 + h2))
                i += 1
            }
        }
        throw WatError(m: "línea \(linea): texto sin cerrar (falta \")")
    }

    // ============================================================
    // MARK: Codificación binaria
    // ============================================================

    static func une(_ partes: [UInt8]...) -> [UInt8] {
        var r: [UInt8] = []
        for x in partes { r.append(contentsOf: x) }
        return r
    }

    static func uleb64(_ v: UInt64) -> [UInt8] {
        var v = v
        var r: [UInt8] = []
        repeat {
            var x = UInt8(v & 0x7F)
            v >>= 7
            if v != 0 { x |= 0x80 }
            r.append(x)
        } while v != 0
        return r
    }

    static func uleb(_ v: Int) -> [UInt8] { uleb64(UInt64(v)) }

    static func sleb(_ v: Int64) -> [UInt8] {
        var v = v
        var r: [UInt8] = []
        while true {
            let x = UInt8(truncatingIfNeeded: v & 0x7F)
            v >>= 7
            let fin = (v == 0 && x & 0x40 == 0) || (v == -1 && x & 0x40 != 0)
            if fin {
                r.append(x)
                return r
            }
            r.append(x | 0x80)
        }
    }

    static func nombreBin(_ s: String) -> [UInt8] {
        let b: [UInt8] = Array(s.utf8)
        return une(uleb(b.count), b)
    }

    static func seccion(_ id: UInt8, _ items: [[UInt8]]) -> [UInt8] {
        if items.isEmpty { return [] }
        var cuerpo: [UInt8] = uleb(items.count)
        for it in items { cuerpo.append(contentsOf: it) }
        return une([id], uleb(cuerpo.count), cuerpo)
    }

    static func le(_ v: UInt64, _ n: Int) -> [UInt8] {
        var r: [UInt8] = []
        for k in 0 ..< n { r.append(UInt8(truncatingIfNeeded: v >> UInt64(8 * k))) }
        return r
    }

    // ============================================================
    // MARK: Números
    // ============================================================

    private func entero(_ n: WatNodo, _ bits: Int) throws -> UInt64 {
        var t = n.texto.replacingOccurrences(of: "_", with: "")
        var neg = false
        if t.hasPrefix("-") {
            neg = true
            t.removeFirst()
        } else if t.hasPrefix("+") {
            t.removeFirst()
        }
        let esHex = t.hasPrefix("0x") || t.hasPrefix("0X")
        let mag: UInt64? = esHex ? UInt64(t.dropFirst(2), radix: 16) : UInt64(t, radix: 10)
        guard let m = mag else { throw err(n, "número entero no válido: '\(n.texto)'") }
        let maxU: UInt64 = bits == 64 ? UInt64.max : (UInt64(1) << UInt64(bits)) - 1
        let maxNeg: UInt64 = UInt64(1) << UInt64(bits - 1)
        if neg {
            guard m <= maxNeg else { throw err(n, "número fuera de rango para i\(bits): \(n.texto)") }
            return (0 &- m) & maxU
        }
        guard m <= maxU else { throw err(n, "número fuera de rango para i\(bits): \(n.texto)") }
        return m
    }

    private func flotante(_ n: WatNodo, _ es64: Bool) throws -> UInt64 {
        var t = n.texto.replacingOccurrences(of: "_", with: "")
        var neg = false
        if t.hasPrefix("-") {
            neg = true
            t.removeFirst()
        } else if t.hasPrefix("+") {
            t.removeFirst()
        }
        let exp: UInt64 = es64 ? 0x7FF0_0000_0000_0000 : 0x7F80_0000
        let signo: UInt64 = es64 ? 0x8000_0000_0000_0000 : 0x8000_0000
        var bits: UInt64 = 0
        let malo = err(n, "número decimal no válido: '\(n.texto)'")
        if t == "inf" {
            bits = exp
        } else if t == "nan" {
            bits = es64 ? 0x7FF8_0000_0000_0000 : 0x7FC0_0000
        } else if t.hasPrefix("nan:0x") {
            let tope: UInt64 = es64 ? (UInt64(1) << 52) : (UInt64(1) << 23)
            guard let p = UInt64(t.dropFirst(6), radix: 16), p != 0, p < tope else { throw malo }
            bits = exp | p
        } else {
            guard let c = t.unicodeScalars.first, c >= "0" && c <= "9" else { throw malo }
            if es64 {
                guard let d = Double(t) else { throw malo }
                bits = d.bitPattern
            } else {
                guard let f = Float(t) else { throw malo }
                bits = UInt64(f.bitPattern)
            }
        }
        if neg { bits |= signo }
        return bits
    }

    private func indiceNumerico(_ n: WatNodo) throws -> Int {
        guard n.clase == .atomo else { throw err(n, "se esperaba un índice") }
        let t = n.texto.replacingOccurrences(of: "_", with: "")
        let v: UInt32? = t.hasPrefix("0x") ? UInt32(t.dropFirst(2), radix: 16) : UInt32(t)
        guard let x = v else { throw err(n, "se esperaba un número o un $nombre y hay '\(n.texto)'") }
        return Int(x)
    }

    private func indice(_ n: WatNodo, _ mapa: [String: Int], _ que: String) throws -> Int {
        if n.esId {
            guard let i = mapa[n.texto] else { throw err(n, "no existe \(que) \(n.texto)") }
            return i
        }
        return try indiceNumerico(n)
    }

    private func valTipo(_ n: WatNodo) throws -> UInt8 {
        switch n.texto {
        case "i32": return WasmValor.i32
        case "i64": return WasmValor.i64
        case "f32": return WasmValor.f32
        case "f64": return WasmValor.f64
        case "funcref", "anyfunc", "func": return WasmValor.funcref
        case "externref", "extern": return WasmValor.externref
        default: throw err(n, "tipo desconocido '\(n.texto)' (se espera i32, i64, f32, f64…)")
        }
    }

    private func cadenaDe(_ n: WatNodo, _ k: Int) throws -> String {
        guard k < n.hijos.count, n.hijos[k].clase == .cadena else { throw err(n, "falta un texto entre comillas") }
        return String(decoding: n.hijos[k].bytes, as: UTF8.self)
    }

    // ============================================================
    // MARK: Tipos
    // ============================================================

    private func tipoExplicito(_ c: WatNodo) throws {
        var i = 1
        let hs = c.hijos
        var nombre: String? = nil
        if i < hs.count, hs[i].esId {
            nombre = hs[i].texto
            i += 1
        }
        guard i < hs.count, hs[i].cabeza == "func" else { throw err(c, "(type …) necesita (func …)") }
        var ps: [UInt8] = []
        var rs: [UInt8] = []
        for h in hs[i].hijos.dropFirst() {
            guard let cab = h.cabeza, cab == "param" || cab == "result" else { throw err(h, "se esperaba (param …) o (result …)") }
            for x in h.hijos.dropFirst() where !x.esId {
                if cab == "param" { ps.append(try valTipo(x)) } else { rs.append(try valTipo(x)) }
            }
        }
        if let n = nombre {
            guard nTipo[n] == nil else { throw err(c, "el tipo \(n) ya existe") }
            nTipo[n] = tipos.count
        }
        tipos.append(WasmFuncTipo(params: ps, results: rs))
    }

    private func buscaTipo(_ t: WasmFuncTipo) -> Int {
        if let k = tipos.firstIndex(of: t) { return k }
        tipos.append(t)
        return tipos.count - 1
    }

    /// (type $t)? (param …)* (result …)*  → índice de tipo, nombres de params, siguiente posición.
    private func usoTipo(_ hs: [WatNodo], _ desde: Int) throws -> (Int, [String?], Int) {
        var i = desde
        var explicito: Int? = nil
        if i < hs.count, hs[i].cabeza == "type" {
            guard hs[i].hijos.count >= 2 else { throw err(hs[i], "(type) sin índice") }
            explicito = try indice(hs[i].hijos[1], nTipo, "el tipo")
            i += 1
        }
        var ps: [UInt8] = []
        var nombres: [String?] = []
        var rs: [UInt8] = []
        while i < hs.count, hs[i].cabeza == "param" {
            let h = hs[i].hijos
            if h.count == 3, h[1].esId {
                ps.append(try valTipo(h[2]))
                nombres.append(h[1].texto)
            } else {
                for x in h.dropFirst() {
                    ps.append(try valTipo(x))
                    nombres.append(nil)
                }
            }
            i += 1
        }
        while i < hs.count, hs[i].cabeza == "result" {
            for x in hs[i].hijos.dropFirst() { rs.append(try valTipo(x)) }
            i += 1
        }
        if let e = explicito {
            guard e < tipos.count else { throw err(hs[desde], "el tipo \(e) no existe") }
            if ps.isEmpty && rs.isEmpty {
                nombres = [String?](repeating: nil, count: tipos[e].params.count)
            }
            return (e, nombres, i)
        }
        return (buscaTipo(WasmFuncTipo(params: ps, results: rs)), nombres, i)
    }

    /// Tipo de bloque: vacío (0x40), un resultado, o índice de tipo.
    private func tipoBloque(_ hs: [WatNodo], _ desde: Int) throws -> ([UInt8], Int) {
        var i = desde
        var explicito: Int? = nil
        if i < hs.count, hs[i].cabeza == "type" {
            guard hs[i].hijos.count >= 2 else { throw err(hs[i], "(type) sin índice") }
            explicito = try indice(hs[i].hijos[1], nTipo, "el tipo")
            i += 1
        }
        var ps: [UInt8] = []
        var rs: [UInt8] = []
        while i < hs.count, hs[i].cabeza == "param" {
            for x in hs[i].hijos.dropFirst() where !x.esId { ps.append(try valTipo(x)) }
            i += 1
        }
        while i < hs.count, hs[i].cabeza == "result" {
            for x in hs[i].hijos.dropFirst() { rs.append(try valTipo(x)) }
            i += 1
        }
        if let e = explicito { return (WatEnsamblador.sleb(Int64(e)), i) }
        if ps.isEmpty && rs.isEmpty { return ([0x40], i) }
        if ps.isEmpty && rs.count == 1 { return ([rs[0]], i) }
        let k = buscaTipo(WasmFuncTipo(params: ps, results: rs))
        return (WatEnsamblador.sleb(Int64(k)), i)
    }

    // ============================================================
    // MARK: Tabla de instrucciones
    // ============================================================

    static let ops: [String: WatOp] = construyeOps()

    static let nombresNumericos: [String] = [
        "i32.eqz", "i32.eq", "i32.ne", "i32.lt_s", "i32.lt_u", "i32.gt_s", "i32.gt_u", "i32.le_s", "i32.le_u", "i32.ge_s", "i32.ge_u",
        "i64.eqz", "i64.eq", "i64.ne", "i64.lt_s", "i64.lt_u", "i64.gt_s", "i64.gt_u", "i64.le_s", "i64.le_u", "i64.ge_s", "i64.ge_u",
        "f32.eq", "f32.ne", "f32.lt", "f32.gt", "f32.le", "f32.ge",
        "f64.eq", "f64.ne", "f64.lt", "f64.gt", "f64.le", "f64.ge",
        "i32.clz", "i32.ctz", "i32.popcnt", "i32.add", "i32.sub", "i32.mul", "i32.div_s", "i32.div_u",
        "i32.rem_s", "i32.rem_u", "i32.and", "i32.or", "i32.xor", "i32.shl", "i32.shr_s", "i32.shr_u", "i32.rotl", "i32.rotr",
        "i64.clz", "i64.ctz", "i64.popcnt", "i64.add", "i64.sub", "i64.mul", "i64.div_s", "i64.div_u",
        "i64.rem_s", "i64.rem_u", "i64.and", "i64.or", "i64.xor", "i64.shl", "i64.shr_s", "i64.shr_u", "i64.rotl", "i64.rotr",
        "f32.abs", "f32.neg", "f32.ceil", "f32.floor", "f32.trunc", "f32.nearest", "f32.sqrt",
        "f32.add", "f32.sub", "f32.mul", "f32.div", "f32.min", "f32.max", "f32.copysign",
        "f64.abs", "f64.neg", "f64.ceil", "f64.floor", "f64.trunc", "f64.nearest", "f64.sqrt",
        "f64.add", "f64.sub", "f64.mul", "f64.div", "f64.min", "f64.max", "f64.copysign",
        "i32.wrap_i64", "i32.trunc_f32_s", "i32.trunc_f32_u", "i32.trunc_f64_s", "i32.trunc_f64_u",
        "i64.extend_i32_s", "i64.extend_i32_u", "i64.trunc_f32_s", "i64.trunc_f32_u", "i64.trunc_f64_s", "i64.trunc_f64_u",
        "f32.convert_i32_s", "f32.convert_i32_u", "f32.convert_i64_s", "f32.convert_i64_u", "f32.demote_f64",
        "f64.convert_i32_s", "f64.convert_i32_u", "f64.convert_i64_s", "f64.convert_i64_u", "f64.promote_f32",
        "i32.reinterpret_f32", "i64.reinterpret_f64", "f32.reinterpret_i32", "f64.reinterpret_i64",
        "i32.extend8_s", "i32.extend16_s", "i64.extend8_s", "i64.extend16_s", "i64.extend32_s"
    ]

    /// (nombre, alineación natural en log2) de 0x28 a 0x3E.
    static let nombresMemoria: [(String, UInt32)] = [
        ("i32.load", 2), ("i64.load", 3), ("f32.load", 2), ("f64.load", 3),
        ("i32.load8_s", 0), ("i32.load8_u", 0), ("i32.load16_s", 1), ("i32.load16_u", 1),
        ("i64.load8_s", 0), ("i64.load8_u", 0), ("i64.load16_s", 1), ("i64.load16_u", 1),
        ("i64.load32_s", 2), ("i64.load32_u", 2),
        ("i32.store", 2), ("i64.store", 3), ("f32.store", 2), ("f64.store", 3),
        ("i32.store8", 0), ("i32.store16", 1), ("i64.store8", 0), ("i64.store16", 1), ("i64.store32", 2)
    ]

    static let nombresFC: [String] = [
        "i32.trunc_sat_f32_s", "i32.trunc_sat_f32_u", "i32.trunc_sat_f64_s", "i32.trunc_sat_f64_u",
        "i64.trunc_sat_f32_s", "i64.trunc_sat_f32_u", "i64.trunc_sat_f64_s", "i64.trunc_sat_f64_u"
    ]

    private static func construyeOps() -> [String: WatOp] {
        var d: [String: WatOp] = [:]
        let simples: [(String, UInt8)] = [("unreachable", 0x00), ("nop", 0x01), ("return", 0x0F),
                                          ("drop", 0x1A), ("ref.is_null", 0xD1)]
        for (n, b) in simples { d[n] = WatOp(bytes: [b], imm: .nada) }
        let conImm: [(String, UInt8, WatImm)] = [
            ("br", 0x0C, .etiqueta), ("br_if", 0x0D, .etiqueta), ("br_table", 0x0E, .tablaBr),
            ("call", 0x10, .funcion), ("call_indirect", 0x11, .callInd), ("select", 0x1B, .select),
            ("local.get", 0x20, .local), ("local.set", 0x21, .local), ("local.tee", 0x22, .local),
            ("global.get", 0x23, .global), ("global.set", 0x24, .global),
            ("get_local", 0x20, .local), ("set_local", 0x21, .local), ("tee_local", 0x22, .local),
            ("get_global", 0x23, .global), ("set_global", 0x24, .global),
            ("table.get", 0x25, .tabla), ("table.set", 0x26, .tabla),
            ("memory.size", 0x3F, .memIdx), ("memory.grow", 0x40, .memIdx),
            ("current_memory", 0x3F, .memIdx), ("grow_memory", 0x40, .memIdx),
            ("i32.const", 0x41, .i32), ("i64.const", 0x42, .i64), ("f32.const", 0x43, .f32), ("f64.const", 0x44, .f64),
            ("ref.null", 0xD0, .refNull), ("ref.func", 0xD2, .funcion)
        ]
        for (n, b, imm) in conImm { d[n] = WatOp(bytes: [b], imm: imm) }
        for (k, (n, al)) in nombresMemoria.enumerated() {
            d[n] = WatOp(bytes: [UInt8(0x28 + k)], imm: .memoria(al))
        }
        for (k, n) in nombresNumericos.enumerated() {
            d[n] = WatOp(bytes: [UInt8(0x45 + k)], imm: .nada)
        }
        for (k, n) in nombresFC.enumerated() {
            d[n] = WatOp(bytes: [0xFC, UInt8(k)], imm: .nada)
        }
        let fc: [(String, UInt8, WatImm)] = [
            ("memory.init", 8, .memInit), ("data.drop", 9, .datoIdx), ("memory.copy", 10, .memIdx2),
            ("memory.fill", 11, .memIdx), ("table.init", 12, .tablaInit), ("elem.drop", 13, .elemIdx),
            ("table.copy", 14, .tabla2), ("table.grow", 15, .tabla), ("table.size", 16, .tabla), ("table.fill", 17, .tabla)
        ]
        for (n, b, imm) in fc { d[n] = WatOp(bytes: [0xFC, b], imm: imm) }
        // nombres viejos
        let viejos: [(String, String)] = [
            ("i32.wrap/i64", "i32.wrap_i64"), ("i64.extend_s/i32", "i64.extend_i32_s"), ("i64.extend_u/i32", "i64.extend_i32_u"),
            ("f32.demote/f64", "f32.demote_f64"), ("f64.promote/f32", "f64.promote_f32"),
            ("i32.reinterpret/f32", "i32.reinterpret_f32"), ("f32.reinterpret/i32", "f32.reinterpret_i32"),
            ("i64.reinterpret/f64", "i64.reinterpret_f64"), ("f64.reinterpret/i64", "f64.reinterpret_i64")
        ]
        for (v, n) in viejos { d[v] = d[n] }
        return d
    }

    // ============================================================
    // MARK: Instrucciones
    // ============================================================

    private func instrs(_ hs: [WatNodo], _ desde: Int, _ out: inout [UInt8]) throws {
        var i = desde
        while i < hs.count {
            let n = hs[i]
            switch n.clase {
            case .lista:
                try plegada(n, &out)
                i += 1
            case .atomo:
                i = try plana(hs, i, &out)
            case .cadena:
                throw err(n, "texto suelto donde se esperaba una instrucción")
            }
        }
    }

    private func plana(_ hs: [WatNodo], _ i: Int, _ out: inout [UInt8]) throws -> Int {
        let n = hs[i]
        switch n.texto {
        case "block", "loop", "if":
            var j = i + 1
            var etiqueta: String? = nil
            if j < hs.count, hs[j].esId {
                etiqueta = hs[j].texto
                j += 1
            }
            let (bt, sig) = try tipoBloque(hs, j)
            out.append(n.texto == "block" ? 0x02 : (n.texto == "loop" ? 0x03 : 0x04))
            out.append(contentsOf: bt)
            etiquetas.append(etiqueta)
            return sig
        case "else", "end":
            if n.texto == "end" {
                guard !etiquetas.isEmpty else { throw err(n, "'end' sin bloque abierto") }
                etiquetas.removeLast()
            } else {
                guard !etiquetas.isEmpty else { throw err(n, "'else' fuera de un 'if'") }
            }
            out.append(n.texto == "end" ? 0x0B : 0x05)
            if i + 1 < hs.count, hs[i + 1].esId { return i + 2 }
            return i + 1
        default:
            let (b, sig) = try codificaOp(hs, i)
            out.append(contentsOf: b)
            return sig
        }
    }

    private func plegada(_ n: WatNodo, _ out: inout [UInt8]) throws {
        let hs = n.hijos
        guard let cab = n.cabeza else { throw err(n, "se esperaba una instrucción entre paréntesis") }
        switch cab {
        case "block", "loop":
            var j = 1
            var etiqueta: String? = nil
            if j < hs.count, hs[j].esId {
                etiqueta = hs[j].texto
                j += 1
            }
            let (bt, sig) = try tipoBloque(hs, j)
            out.append(cab == "block" ? 0x02 : 0x03)
            out.append(contentsOf: bt)
            etiquetas.append(etiqueta)
            try instrs(hs, sig, &out)
            out.append(0x0B)
            etiquetas.removeLast()
        case "if":
            try plegadaIf(n, &out)
        case "then", "else":
            throw err(n, "(\(cab) …) solo va dentro de (if …)")
        default:
            let (b, sig) = try codificaOp(hs, 0)
            for k in sig ..< hs.count {
                guard hs[k].clase == .lista else {
                    throw err(hs[k], "en forma plegada los operandos van entre paréntesis ('\(hs[k].texto)')")
                }
                try plegada(hs[k], &out)
            }
            out.append(contentsOf: b)
        }
    }

    private func plegadaIf(_ n: WatNodo, _ out: inout [UInt8]) throws {
        let hs = n.hijos
        var j = 1
        var etiqueta: String? = nil
        if j < hs.count, hs[j].esId {
            etiqueta = hs[j].texto
            j += 1
        }
        let (bt, sig) = try tipoBloque(hs, j)
        j = sig
        while j < hs.count, hs[j].cabeza != "then" {
            guard hs[j].clase == .lista, hs[j].cabeza != "else" else { throw err(hs[j], "a este (if …) le falta (then …)") }
            try plegada(hs[j], &out)
            j += 1
        }
        guard j < hs.count else { throw err(n, "a este (if …) le falta (then …)") }
        out.append(0x04)
        out.append(contentsOf: bt)
        etiquetas.append(etiqueta)
        try instrs(hs[j].hijos, 1, &out)
        j += 1
        if j < hs.count, hs[j].cabeza == "else" {
            out.append(0x05)
            try instrs(hs[j].hijos, 1, &out)
            j += 1
        }
        guard j == hs.count else { throw err(hs[j], "sobra algo después de (else …)") }
        out.append(0x0B)
        etiquetas.removeLast()
    }

    private func etiqueta(_ n: WatNodo) throws -> Int {
        if n.esId {
            var k = etiquetas.count - 1
            while k >= 0 {
                if etiquetas[k] == n.texto { return etiquetas.count - 1 - k }
                k -= 1
            }
            throw err(n, "no existe la etiqueta \(n.texto)")
        }
        return try indiceNumerico(n)
    }

    private func atomo(_ hs: [WatNodo], _ j: inout Int, _ ref: WatNodo, _ que: String) throws -> WatNodo {
        guard j < hs.count, hs[j].clase == .atomo else { throw err(ref, "a '\(ref.texto)' le falta \(que)") }
        j += 1
        return hs[j - 1]
    }

    /// Instrucción + inmediatos. Devuelve los bytes y la posición siguiente.
    private func codificaOp(_ hs: [WatNodo], _ i: Int) throws -> ([UInt8], Int) {
        let n = hs[i]
        guard n.clase == .atomo, let op = WatEnsamblador.ops[n.texto] else {
            throw err(n, "instrucción desconocida '\(n.texto)'")
        }
        var b = op.bytes
        var j = i + 1
        try inmediatos(op.imm, hs, &j, &b, n)
        return (b, j)
    }

    private func inmediatos(_ imm: WatImm, _ hs: [WatNodo], _ j: inout Int, _ b: inout [UInt8], _ n: WatNodo) throws {
        switch imm {
        case .nada:
            break
        case .local:
            b += WatEnsamblador.uleb(try indice(try atomo(hs, &j, n, "la variable"), locales, "la variable local"))
        case .global:
            b += WatEnsamblador.uleb(try indice(try atomo(hs, &j, n, "la global"), nGlobal, "la global"))
        case .funcion:
            b += WatEnsamblador.uleb(try indice(try atomo(hs, &j, n, "la función"), nFunc, "la función"))
        case .etiqueta:
            b += WatEnsamblador.uleb(try etiqueta(try atomo(hs, &j, n, "la etiqueta")))
        case .tablaBr:
            var ls: [Int] = []
            while j < hs.count, hs[j].esIndice {
                ls.append(try etiqueta(hs[j]))
                j += 1
            }
            guard let def = ls.popLast() else { throw err(n, "br_table necesita al menos una etiqueta") }
            b += WatEnsamblador.uleb(ls.count)
            for l in ls { b += WatEnsamblador.uleb(l) }
            b += WatEnsamblador.uleb(def)
        case .memoria(let al):
            try memarg(al, hs, &j, &b)
        case .i32:
            let v = try entero(try atomo(hs, &j, n, "el número"), 32)
            b += WatEnsamblador.sleb(Int64(Int32(bitPattern: UInt32(v))))
        case .i64:
            let v = try entero(try atomo(hs, &j, n, "el número"), 64)
            b += WatEnsamblador.sleb(Int64(bitPattern: v))
        case .f32:
            b += WatEnsamblador.le(try flotante(try atomo(hs, &j, n, "el número"), false), 4)
        case .f64:
            b += WatEnsamblador.le(try flotante(try atomo(hs, &j, n, "el número"), true), 8)
        default:
            try inmediatos2(imm, hs, &j, &b, n)
        }
    }

    private func inmediatos2(_ imm: WatImm, _ hs: [WatNodo], _ j: inout Int, _ b: inout [UInt8], _ n: WatNodo) throws {
        switch imm {
        case .callInd:
            var tabla = 0
            if j < hs.count, hs[j].esIndice {
                tabla = try indice(hs[j], nTabla, "la tabla")
                j += 1
            }
            let (t, _, sig) = try usoTipo(hs, j)
            j = sig
            b += WatEnsamblador.une(WatEnsamblador.uleb(t), WatEnsamblador.uleb(tabla))
        case .memIdx, .memIdx2:
            var k = 0
            while j < hs.count, hs[j].esIndice, k < 2 {
                j += 1
                k += 1
            }
            b += imm.esDoble ? [0, 0] : [0]
        case .memInit:
            var idx = try atomo(hs, &j, n, "el segmento de datos")
            if j < hs.count, hs[j].esIndice { idx = hs[j]; j += 1 }
            b += WatEnsamblador.uleb(try indice(idx, nDato, "el segmento de datos"))
            b.append(0)
            usaDataCount = true
        case .datoIdx:
            b += WatEnsamblador.uleb(try indice(try atomo(hs, &j, n, "el segmento de datos"), nDato, "el segmento de datos"))
            usaDataCount = true
        case .elemIdx:
            b += WatEnsamblador.uleb(try indice(try atomo(hs, &j, n, "el segmento de elementos"), nElem, "el segmento"))
        case .tabla:
            var t = 0
            if j < hs.count, hs[j].esIndice {
                t = try indice(hs[j], nTabla, "la tabla")
                j += 1
            }
            b += WatEnsamblador.uleb(t)
        case .tabla2:
            var ts: [Int] = []
            while j < hs.count, hs[j].esIndice, ts.count < 2 {
                ts.append(try indice(hs[j], nTabla, "la tabla"))
                j += 1
            }
            let x = ts.count > 0 ? ts[0] : 0
            let y = ts.count > 1 ? ts[1] : 0
            b += WatEnsamblador.une(WatEnsamblador.uleb(x), WatEnsamblador.uleb(y))
        case .tablaInit:
            var a: [WatNodo] = []
            while j < hs.count, hs[j].esIndice, a.count < 2 {
                a.append(hs[j])
                j += 1
            }
            guard let ultimo = a.last else { throw err(n, "table.init necesita el segmento") }
            let t = a.count == 2 ? try indice(a[0], nTabla, "la tabla") : 0
            b += WatEnsamblador.uleb(try indice(ultimo, nElem, "el segmento"))
            b += WatEnsamblador.uleb(t)
        case .refNull:
            b.append(try valTipo(try atomo(hs, &j, n, "el tipo (func o extern)")))
        case .select:
            var ts: [UInt8] = []
            while j < hs.count, hs[j].cabeza == "result" {
                for x in hs[j].hijos.dropFirst() { ts.append(try valTipo(x)) }
                j += 1
            }
            if !ts.isEmpty { b = WatEnsamblador.une([0x1C], WatEnsamblador.uleb(ts.count), ts) }
        default:
            break
        }
    }

    private func memarg(_ natural: UInt32, _ hs: [WatNodo], _ j: inout Int, _ b: inout [UInt8]) throws {
        var offset: UInt64 = 0
        var al = natural
        while j < hs.count, hs[j].clase == .atomo {
            let t = hs[j].texto
            if t.hasPrefix("offset=") {
                let v = WatNodo(clase: .atomo, texto: String(t.dropFirst(7)), bytes: [], hijos: [], linea: hs[j].linea)
                offset = try entero(v, 32)
            } else if t.hasPrefix("align=") {
                let v = WatNodo(clase: .atomo, texto: String(t.dropFirst(6)), bytes: [], hijos: [], linea: hs[j].linea)
                let a = try entero(v, 32)
                guard a > 0, a & (a - 1) == 0 else { throw err(hs[j], "align debe ser potencia de 2") }
                al = UInt32(a.trailingZeroBitCount)
            } else if hs[j].esId || t == "0" {
                // índice de memoria (solo hay una)
            } else {
                break
            }
            j += 1
        }
        b += WatEnsamblador.une(WatEnsamblador.uleb(Int(al)), WatEnsamblador.uleb64(offset))
    }

    /// Expresión constante (offsets, globales): instrucciones + end.
    private func expresion(_ hs: [WatNodo], _ desde: Int) throws -> [UInt8] {
        locales = [:]
        etiquetas = []
        var out: [UInt8] = []
        try instrs(hs, desde, &out)
        out.append(0x0B)
        return out
    }

    // ============================================================
    // MARK: Módulo
    // ============================================================

    private func cabecera(_ n: WatNodo) throws -> WatDef {
        let hs = n.hijos
        var d = WatDef(nodo: n, nombre: nil, exports: [], impModulo: nil, impCampo: "", resto: 1)
        var i = 1
        if i < hs.count, hs[i].esId {
            d.nombre = hs[i].texto
            i += 1
        }
        while i < hs.count, hs[i].cabeza == "export" {
            d.exports.append(try cadenaDe(hs[i], 1))
            i += 1
        }
        if i < hs.count, hs[i].cabeza == "import" {
            d.impModulo = try cadenaDe(hs[i], 1)
            d.impCampo = try cadenaDe(hs[i], 2)
            i += 1
        }
        d.resto = i
        return d
    }

    private func nombra(_ defs: [WatDef], _ mapa: inout [String: Int], _ que: String) throws {
        for (k, d) in defs.enumerated() {
            guard let n = d.nombre else { continue }
            guard mapa[n] == nil else { throw err(d.nodo, "\(que) \(n) está repetida") }
            mapa[n] = k
        }
    }

    private func limites(_ hs: [WatNodo], _ desde: Int) throws -> ([UInt8], Int) {
        var i = desde
        var ns: [UInt64] = []
        while i < hs.count, hs[i].clase == .atomo, let c = hs[i].texto.unicodeScalars.first, c >= "0" && c <= "9" {
            ns.append(try entero(hs[i], 32))
            i += 1
        }
        guard let mn = ns.first else {
            throw err(hs.isEmpty ? WatNodo(clase: .atomo, texto: "", bytes: [], hijos: [], linea: 0) : hs[0],
                      "faltan los límites (tamaño mínimo)")
        }
        if ns.count >= 2 { return (WatEnsamblador.une([0x01], WatEnsamblador.uleb64(mn), WatEnsamblador.uleb64(ns[1])), i) }
        return (WatEnsamblador.une([0x00], WatEnsamblador.uleb64(mn)), i)
    }

    private func tipoGlobal(_ n: WatNodo) throws -> [UInt8] {
        if n.cabeza == "mut" {
            guard n.hijos.count == 2 else { throw err(n, "(mut …) necesita un tipo") }
            let vt: UInt8 = try valTipo(n.hijos[1])
            return [vt, 1]
        }
        let vt: UInt8 = try valTipo(n)
        return [vt, 0]
    }

    private struct Partes {
        var fImp: [WatDef] = []
        var fDef: [WatDef] = []
        var tImp: [WatDef] = []
        var tDef: [WatDef] = []
        var mImp: [WatDef] = []
        var mDef: [WatDef] = []
        var gImp: [WatDef] = []
        var gDef: [WatDef] = []
        var exports: [WatNodo] = []
        /// orden de aparición de TODAS las exportaciones: (nodo suelto) o (clase, importada, posición, nombre)
        var orden: [(WatNodo?, UInt8, Bool, Int, String)] = []
        var elems: [WatNodo] = []      // también las tablas con (elem …) dentro, en su orden
        var datos: [WatNodo] = []      // y las memorias con (data …) dentro
        var elemTabla: [Int] = []      // -1 = (elem …) suelto; si no, qué tabla propia
        var datoMem: [Int] = []
        var inicio: WatNodo? = nil
    }

    private func reparte(_ campos: [WatNodo]) throws -> Partes {
        var p = Partes()
        for c in campos {
            guard let cab = c.cabeza else { throw err(c, "se esperaba un campo del módulo entre paréntesis") }
            var d: WatDef
            var clase = cab
            switch cab {
            case "type":
                try tipoExplicito(c)
                continue
            case "export":
                p.exports.append(c)
                p.orden.append((c, 0, false, 0, ""))
                continue
            case "start": p.inicio = c; continue
            case "elem":
                p.elems.append(c)
                p.elemTabla.append(-1)
                continue
            case "data":
                p.datos.append(c)
                p.datoMem.append(-1)
                continue
            case "import":
                guard c.hijos.count >= 4, let k = c.hijos[3].cabeza else { throw err(c, "(import \"m\" \"n\" (func …)) incompleto") }
                d = try cabecera(c.hijos[3])
                d.impModulo = try cadenaDe(c, 1)
                d.impCampo = try cadenaDe(c, 2)
                clase = k
            case "func", "table", "memory", "global":
                d = try cabecera(c)
            default:
                throw err(c, "campo desconocido '\(cab)'")
            }
            let imp = d.impModulo != nil
            let pos: [String: (UInt8, Int)] = ["func": (0, imp ? p.fImp.count : p.fDef.count),
                                               "table": (1, imp ? p.tImp.count : p.tDef.count),
                                               "memory": (2, imp ? p.mImp.count : p.mDef.count),
                                               "global": (3, imp ? p.gImp.count : p.gDef.count)]
            if let (cl, k) = pos[clase] {
                for e in d.exports { p.orden.append((nil, cl, imp, k, e)) }
            }
            switch clase {
            case "func": if imp { p.fImp.append(d) } else { p.fDef.append(d) }
            case "table":
                if imp { p.tImp.append(d); break }
                let hs = d.nodo.hijos
                if d.resto + 1 < hs.count, hs[d.resto + 1].cabeza == "elem" {
                    p.elems.append(d.nodo)
                    p.elemTabla.append(p.tDef.count)
                }
                p.tDef.append(d)
            case "memory":
                if imp { p.mImp.append(d); break }
                let hs = d.nodo.hijos
                if d.resto < hs.count, hs[d.resto].cabeza == "data" {
                    p.datos.append(d.nodo)
                    p.datoMem.append(p.mDef.count)
                }
                p.mDef.append(d)
            case "global": if imp { p.gImp.append(d) } else { p.gDef.append(d) }
            default: throw err(c, "no se puede importar '\(clase)'")
            }
        }
        return p
    }

    private func modulo(_ raiz: [WatNodo]) throws -> [UInt8] {
        var campos = raiz
        if raiz.count == 1, raiz[0].cabeza == "module" {
            var hs = raiz[0].hijos
            hs.removeFirst()
            if let p = hs.first, p.esId { hs.removeFirst() }
            if let p = hs.first, p.clase == .atomo { throw err(p, "módulos '\(p.texto)' (binary/quote) no soportados") }
            campos = hs
        }
        let p = try reparte(campos)
        let funcs = p.fImp + p.fDef
        try nombra(funcs, &nFunc, "la función")
        try nombra(p.tImp + p.tDef, &nTabla, "la tabla")
        try nombra(p.mImp + p.mDef, &nMem, "la memoria")
        try nombra(p.gImp + p.gDef, &nGlobal, "la global")
        for (k, e) in p.elems.enumerated() where p.elemTabla[k] < 0 && e.hijos.count > 1 && e.hijos[1].esId {
            nElem[e.hijos[1].texto] = k
        }
        for (k, d) in p.datos.enumerated() where p.datoMem[k] < 0 && d.hijos.count > 1 && d.hijos[1].esId {
            nDato[d.hijos[1].texto] = k
        }

        // tipos de todas las funciones
        var fTipo: [Int] = []
        var fParams: [[String?]] = []
        var fResto: [Int] = []
        for f in funcs {
            let (t, ns, sig) = try usoTipo(f.nodo.hijos, f.resto)
            fTipo.append(t)
            fParams.append(ns)
            fResto.append(sig)
        }

        var elemsExtra: [Int: [UInt8]] = [:]
        var datosExtra: [Int: [UInt8]] = [:]
        let imports = try seccionImports(p, fTipo)
        var tablas: [[UInt8]] = []
        for (k, t) in p.tDef.enumerated() {
            tablas.append(try tabla(t, p.tImp.count + k, k, &elemsExtra))
        }
        var memorias: [[UInt8]] = []
        for (k, m) in p.mDef.enumerated() {
            memorias.append(try memoria(m, p.mImp.count + k, k, &datosExtra))
        }
        var globales: [[UInt8]] = []
        for g in p.gDef {
            let hs = g.nodo.hijos
            guard g.resto < hs.count else { throw err(g.nodo, "a la global le falta el tipo") }
            let tg: [UInt8] = try tipoGlobal(hs[g.resto])
            globales.append(WatEnsamblador.une(tg, try expresion(hs, g.resto + 1)))
        }
        let exports = try seccionExports(p)
        var cuerpos: [[UInt8]] = []
        for (k, f) in p.fDef.enumerated() {
            let i = p.fImp.count + k
            cuerpos.append(try cuerpo(f, fParams[i], fResto[i], fTipo[i]))
        }
        var elems: [[UInt8]] = []
        for (k, e) in p.elems.enumerated() {
            if p.elemTabla[k] >= 0 { elems.append(elemsExtra[p.elemTabla[k]] ?? []) } else { elems.append(try elemento(e)) }
        }
        var datos: [[UInt8]] = []
        for (k, d) in p.datos.enumerated() {
            if p.datoMem[k] >= 0 { datos.append(datosExtra[p.datoMem[k]] ?? []) } else { datos.append(try dato(d)) }
        }

        var out: [UInt8] = [0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00]
        var tiposBin: [[UInt8]] = []
        for t in tipos {
            tiposBin.append(WatEnsamblador.une([0x60], WatEnsamblador.uleb(t.params.count), t.params, WatEnsamblador.uleb(t.results.count), t.results))
        }
        out += WatEnsamblador.seccion(1, tiposBin)
        out += WatEnsamblador.seccion(2, imports)
        out += WatEnsamblador.seccion(3, p.fDef.indices.map { WatEnsamblador.uleb(fTipo[p.fImp.count + $0]) })
        out += WatEnsamblador.seccion(4, tablas)
        out += WatEnsamblador.seccion(5, memorias)
        out += WatEnsamblador.seccion(6, globales)
        out += WatEnsamblador.seccion(7, exports)
        if let s = p.inicio {
            guard s.hijos.count == 2 else { throw err(s, "(start $función)") }
            let f = WatEnsamblador.uleb(try indice(s.hijos[1], nFunc, "la función"))
            out += WatEnsamblador.une([8], WatEnsamblador.uleb(f.count), f)
        }
        out += WatEnsamblador.seccion(9, elems)
        if usaDataCount {
            let n = WatEnsamblador.uleb(datos.count)
            out += WatEnsamblador.une([12], WatEnsamblador.uleb(n.count), n)
        }
        out += WatEnsamblador.seccion(10, cuerpos)
        out += WatEnsamblador.seccion(11, datos)
        return out
    }

    private func seccionImports(_ p: Partes, _ fTipo: [Int]) throws -> [[UInt8]] {
        var r: [[UInt8]] = []
        func cab(_ d: WatDef) -> [UInt8] {
            WatEnsamblador.une(WatEnsamblador.nombreBin(d.impModulo ?? ""), WatEnsamblador.nombreBin(d.impCampo))
        }
        for (k, d) in p.fImp.enumerated() {
            r.append(WatEnsamblador.une(cab(d), [0x00], WatEnsamblador.uleb(fTipo[k])))
        }
        for d in p.tImp {
            let hs = d.nodo.hijos
            let (lim, sig) = try limites(hs, d.resto)
            guard sig < hs.count else { throw err(d.nodo, "a la tabla le falta el tipo (funcref)") }
            let vt: UInt8 = try valTipo(hs[sig])
            r.append(WatEnsamblador.une(cab(d), [0x01, vt], lim))
        }
        for d in p.mImp {
            let (lim, _) = try limites(d.nodo.hijos, d.resto)
            r.append(WatEnsamblador.une(cab(d), [0x02], lim))
        }
        for d in p.gImp {
            let hs = d.nodo.hijos
            guard d.resto < hs.count else { throw err(d.nodo, "a la global le falta el tipo") }
            let tg: [UInt8] = try tipoGlobal(hs[d.resto])
            r.append(WatEnsamblador.une(cab(d), [0x03], tg))
        }
        return r
    }

    private func seccionExports(_ p: Partes) throws -> [[UInt8]] {
        var r: [[UInt8]] = []
        let importadas: [Int] = [p.fImp.count, p.tImp.count, p.mImp.count, p.gImp.count]
        for (nodo, cl, imp, pos, nom) in p.orden {
            guard let e = nodo else {
                let i = imp ? pos : importadas[Int(cl)] + pos
                r.append(WatEnsamblador.une(WatEnsamblador.nombreBin(nom), [cl], WatEnsamblador.uleb(i)))
                continue
            }
            let nombre = try cadenaDe(e, 1)
            guard e.hijos.count == 3, let k = e.hijos[2].cabeza, e.hijos[2].hijos.count == 2 else {
                throw err(e, "(export \"nombre\" (func $f)) mal escrito")
            }
            let x = e.hijos[2].hijos[1]
            var clase: UInt8 = 0
            var idx = 0
            switch k {
            case "func": idx = try indice(x, nFunc, "la función")
            case "table": clase = 1; idx = try indice(x, nTabla, "la tabla")
            case "memory": clase = 2; idx = try indice(x, nMem, "la memoria")
            case "global": clase = 3; idx = try indice(x, nGlobal, "la global")
            default: throw err(e, "no se puede exportar '\(k)'")
            }
            r.append(WatEnsamblador.une(WatEnsamblador.nombreBin(nombre), [clase], WatEnsamblador.uleb(idx)))
        }
        return r
    }

    private func tabla(_ d: WatDef, _ pos: Int, _ k: Int, _ elems: inout [Int: [UInt8]]) throws -> [UInt8] {
        let hs = d.nodo.hijos
        if d.resto + 1 < hs.count, hs[d.resto + 1].cabeza == "elem" {
            let tipo = try valTipo(hs[d.resto])
            var fs: [UInt8] = []
            let items = Array(hs[d.resto + 1].hijos.dropFirst())
            for x in items { fs += WatEnsamblador.uleb(try indice(x, nFunc, "la función")) }
            let off: [UInt8] = [0x41, 0x00, 0x0B]
            let n: [UInt8] = WatEnsamblador.uleb(items.count)
            if pos == 0 {
                elems[k] = WatEnsamblador.une([0x00], off, n, fs)
            } else {
                elems[k] = WatEnsamblador.une([0x02], WatEnsamblador.uleb(pos), off, [0x00], n, fs)
            }
            return WatEnsamblador.une([tipo, 0x01], n, n)
        }
        let (lim, sig) = try limites(hs, d.resto)
        guard sig < hs.count else { throw err(d.nodo, "a la tabla le falta el tipo (funcref)") }
        let vt: UInt8 = try valTipo(hs[sig])
        return WatEnsamblador.une([vt], lim)
    }

    private func memoria(_ d: WatDef, _ pos: Int, _ k: Int, _ datos: inout [Int: [UInt8]]) throws -> [UInt8] {
        let hs = d.nodo.hijos
        if d.resto < hs.count, hs[d.resto].cabeza == "data" {
            var bytes: [UInt8] = []
            for x in hs[d.resto].hijos.dropFirst() { bytes += x.bytes }
            let pags = (bytes.count + 65535) / 65536
            let off: [UInt8] = [0x41, 0x00, 0x0B]
            let n: [UInt8] = WatEnsamblador.uleb(bytes.count)
            if pos == 0 {
                datos[k] = WatEnsamblador.une([0x00], off, n, bytes)
            } else {
                datos[k] = WatEnsamblador.une([0x02], WatEnsamblador.uleb(pos), off, n, bytes)
            }
            let pg: [UInt8] = WatEnsamblador.uleb(pags)
            return WatEnsamblador.une([0x01], pg, pg)
        }
        return try limites(hs, d.resto).0
    }

    private func cuerpo(_ f: WatDef, _ params: [String?], _ desde: Int, _ tipo: Int) throws -> [UInt8] {
        let hs = f.nodo.hijos
        locales = [:]
        etiquetas = []
        for (k, p) in params.enumerated() {
            if let n = p { locales[n] = k }
        }
        var tiposLoc: [UInt8] = []
        var i = desde
        while i < hs.count, hs[i].cabeza == "local" {
            let h = hs[i].hijos
            if h.count == 3, h[1].esId {
                guard locales[h[1].texto] == nil else { throw err(hs[i], "la variable \(h[1].texto) está repetida") }
                locales[h[1].texto] = params.count + tiposLoc.count
                tiposLoc.append(try valTipo(h[2]))
            } else {
                for x in h.dropFirst() { tiposLoc.append(try valTipo(x)) }
            }
            i += 1
        }
        var codigo: [UInt8] = []
        var grupos: [[UInt8]] = []
        var k = 0
        while k < tiposLoc.count {
            var n = 1
            while k + n < tiposLoc.count, tiposLoc[k + n] == tiposLoc[k] { n += 1 }
            grupos.append(WatEnsamblador.une(WatEnsamblador.uleb(n), [tiposLoc[k]]))
            k += n
        }
        codigo += WatEnsamblador.uleb(grupos.count)
        for g in grupos { codigo += g }
        try instrs(hs, i, &codigo)
        guard etiquetas.isEmpty else { throw err(f.nodo, "falta un 'end' en esta función") }
        codigo.append(0x0B)
        return WatEnsamblador.une(WatEnsamblador.uleb(codigo.count), codigo)
    }

    /// (offset …) o una instrucción plegada → expresión constante.
    private func offset(_ n: WatNodo) throws -> [UInt8] {
        if n.cabeza == "offset" { return try expresion(n.hijos, 1) }
        return try expresion([n], 0)
    }

    private func elemento(_ e: WatNodo) throws -> [UInt8] {
        let hs = e.hijos
        var i = 1
        if i < hs.count, hs[i].esId { i += 1 }
        var modo = 1             // 0 activo, 1 pasivo, 2 declarativo
        var tabla = 0
        var off: [UInt8] = []
        if i < hs.count, hs[i].clase == .atomo, hs[i].texto == "declare" {
            modo = 2
            i += 1
        }
        if i < hs.count, hs[i].cabeza == "table" {
            guard hs[i].hijos.count == 2 else { throw err(hs[i], "(table $t)") }
            tabla = try indice(hs[i].hijos[1], nTabla, "la tabla")
            i += 1
        } else if i < hs.count, hs[i].esIndice, i + 1 < hs.count, hs[i + 1].clase == .lista {
            tabla = try indice(hs[i], nTabla, "la tabla")
            i += 1
        }
        if modo != 2, i < hs.count, let c = hs[i].cabeza, c != "item", c != "ref.func", c != "ref.null" {
            off = try offset(hs[i])
            modo = 0
            i += 1
        }
        var tipo = WasmValor.funcref
        if i < hs.count, hs[i].clase == .atomo, ["func", "funcref", "externref"].contains(hs[i].texto) {
            if hs[i].texto == "externref" { tipo = WasmValor.externref }
            i += 1
        }
        var funcs: [Int] = []
        var exprs: [[UInt8]] = []
        var soloFuncs = true
        for x in hs[i...] {
            if x.clase == .atomo {
                let f = try indice(x, nFunc, "la función")
                funcs.append(f)
                exprs.append(WatEnsamblador.une([0xD2], WatEnsamblador.uleb(f), [0x0B]))
            } else {
                soloFuncs = false
                exprs.append(x.cabeza == "item" ? try expresion(x.hijos, 1) : try expresion([x], 0))
            }
        }
        return codificaElem(modo, tabla, off, tipo, soloFuncs, funcs, exprs)
    }

    private func codificaElem(_ modo: Int, _ tabla: Int, _ off: [UInt8], _ tipo: UInt8, _ soloFuncs: Bool,
                              _ funcs: [Int], _ exprs: [[UInt8]]) -> [UInt8] {
        var items: [UInt8] = WatEnsamblador.uleb(soloFuncs ? funcs.count : exprs.count)
        if soloFuncs {
            for f in funcs { items += WatEnsamblador.uleb(f) }
        } else {
            for x in exprs { items += x }
        }
        let tipoExtra: [UInt8] = soloFuncs ? [0x00] : [tipo]
        var flags: UInt8 = soloFuncs ? 0 : 4
        switch modo {
        case 0:
            if tabla == 0 { return WatEnsamblador.une([flags], off, items) }
            flags |= 2
            return WatEnsamblador.une([flags], WatEnsamblador.uleb(tabla), off, tipoExtra, items)
        case 1:
            return WatEnsamblador.une([flags | 1], tipoExtra, items)
        default:
            return WatEnsamblador.une([flags | 3], tipoExtra, items)
        }
    }

    private func dato(_ d: WatNodo) throws -> [UInt8] {
        let hs = d.hijos
        var i = 1
        if i < hs.count, hs[i].esId { i += 1 }
        var mem = 0
        if i < hs.count, hs[i].cabeza == "memory" {
            guard hs[i].hijos.count == 2 else { throw err(hs[i], "(memory $m)") }
            mem = try indice(hs[i].hijos[1], nMem, "la memoria")
            i += 1
        } else if i < hs.count, hs[i].esIndice {
            mem = try indice(hs[i], nMem, "la memoria")
            i += 1
        }
        var off: [UInt8]? = nil
        if i < hs.count, hs[i].clase == .lista {
            off = try offset(hs[i])
            i += 1
        }
        var bytes: [UInt8] = []
        for x in hs[i...] {
            guard x.clase == .cadena else { throw err(x, "en (data …) solo van textos entre comillas") }
            bytes += x.bytes
        }
        let cuerpo: [UInt8] = WatEnsamblador.une(WatEnsamblador.uleb(bytes.count), bytes)
        guard let o = off else { return WatEnsamblador.une([0x01], cuerpo) }
        if mem == 0 { return WatEnsamblador.une([0x00], o, cuerpo) }
        return WatEnsamblador.une([0x02], WatEnsamblador.uleb(mem), o, cuerpo)
    }
}

extension WatImm {
    var esDoble: Bool {
        if case .memIdx2 = self { return true }
        return false
    }
}
