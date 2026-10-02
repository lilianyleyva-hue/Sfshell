import Foundation

// ============================================================
// MARK: - WebAssembly: intérprete en Swift puro
// ============================================================
// Lee módulos .wasm (formato binario MVP + extras comunes), los
// "precompila" a un arreglo plano de instrucciones con los saltos ya
// resueltos y los corre con una pila de UInt64 (los floats van como
// patrones de bits). Trae WASI preview1 (lo básico: imprimir, leer,
// argumentos, reloj, archivos dentro del área de trabajo) y un módulo
// "env" sencillo para programas sin libc.
//
//   wasm programa.wasm [args]        corre _start (o main)
//   wasm programa.wat  [args]        ensambla el texto y lo corre
//   wasm --invoke fib x.wasm 20      llama a una función exportada
//   wasm info x.wasm                 importaciones, exportaciones…
//   wat2wasm x.wat [-o x.wasm]       texto → binario (ver 51_Wat.swift)
//
// Cómo va rápido: cada instrucción sabe en qué altura de la pila
// trabaja (se calcula al precompilar), así que no hay puntero de pila
// que mover, 'drop' no cuesta nada y un salto es solo cambiar el pc.
// Las llamadas entre funciones wasm no usan recursión de Swift: hay una
// pila de marcos propia.

struct WasmTrap: Error {
    let m: String
    init(_ m: String) { self.m = m }
}

/// proc_exit(n): no es un error, es la salida del programa.
struct WasmSalida: Error {
    let codigo: Int32
}

enum WasmValor {
    static let i32: UInt8 = 0x7F
    static let i64: UInt8 = 0x7E
    static let f32: UInt8 = 0x7D
    static let f64: UInt8 = 0x7C
    static let funcref: UInt8 = 0x70
    static let externref: UInt8 = 0x6F

    static func nombre(_ t: UInt8) -> String {
        switch t {
        case 0x7F: return "i32"
        case 0x7E: return "i64"
        case 0x7D: return "f32"
        case 0x7C: return "f64"
        case 0x7B: return "v128"
        case 0x70: return "funcref"
        case 0x6F: return "externref"
        default: return "?"
        }
    }

    /// Letra para comparar firmas: i I f F.
    static func letra(_ t: UInt8) -> Character {
        switch t {
        case 0x7F: return "i"
        case 0x7E: return "I"
        case 0x7D: return "f"
        case 0x7C: return "F"
        default: return "r"
        }
    }
}

struct WasmFuncTipo: Equatable {
    var params: [UInt8]
    var results: [UInt8]

    var texto: String {
        let p: String = params.map { WasmValor.nombre($0) }.joined(separator: ", ")
        let r: String = results.map { WasmValor.nombre($0) }.joined(separator: ", ")
        if results.isEmpty { return "(" + p + ")" }
        return "(" + p + ") -> " + (results.count == 1 ? r : "(" + r + ")")
    }

    var letrasParams: String { String(params.map { WasmValor.letra($0) }) }
}

struct WasmLimites {
    var min: UInt64
    var max: UInt64?
}

/// Una instrucción de una expresión constante (offsets, valores iniciales).
struct WasmOpConst {
    let op: UInt8
    let v: UInt64
}

struct WasmImport {
    let modulo: String
    let campo: String
    let clase: UInt8          // 0 función, 1 tabla, 2 memoria, 3 global
    var tipo: Int = 0         // función: índice de tipo
    var lim = WasmLimites(min: 0, max: nil)
    var valTipo: UInt8 = 0    // tabla / global
    var mutable = false

    var nombre: String { modulo + "." + campo }
}

struct WasmExport {
    let nombre: String
    let clase: UInt8
    let indice: Int
}

struct WasmGlobalDef {
    let tipo: UInt8
    let mutable: Bool
    let inicio: [WasmOpConst]
}

struct WasmElem {
    var modo: Int             // 0 activo, 1 pasivo, 2 declarativo
    var tabla: Int
    var offset: [WasmOpConst]
    var items: [[WasmOpConst]]
}

struct WasmDato {
    var activo: Bool
    var memoria: Int
    var offset: [WasmOpConst]
    var bytes: [UInt8]
}

struct WasmCodigo {
    var locales: [UInt8]
    var inicio: Int
    var fin: Int
}

// ============================================================
// MARK: - Lector binario
// ============================================================

struct WasmLector {
    let b: [UInt8]
    var p: Int
    var fin: Int

    init(_ b: [UInt8], _ p: Int = 0, _ fin: Int? = nil) {
        self.b = b
        self.p = p
        self.fin = fin ?? b.count
    }

    var alFinal: Bool { p >= fin }

    static func malo(_ m: String) -> WasmTrap { WasmTrap("módulo no válido: " + m) }

    mutating func byte() throws -> UInt8 {
        guard p < fin else { throw WasmLector.malo("se acabaron los bytes antes de tiempo") }
        let v = b[p]
        p += 1
        return v
    }

    mutating func u32() throws -> UInt32 {
        var r: UInt64 = 0
        var s: UInt64 = 0
        while true {
            let x = try byte()
            r |= UInt64(x & 0x7F) << s
            if x & 0x80 == 0 { break }
            s += 7
            if s > 35 { throw WasmLector.malo("número LEB128 demasiado largo") }
        }
        guard r <= UInt64(UInt32.max) else { throw WasmLector.malo("número u32 fuera de rango") }
        return UInt32(r)
    }

    mutating func entero() throws -> Int { Int(try u32()) }

    mutating func sleb() throws -> Int64 {
        var r: Int64 = 0
        var s: UInt64 = 0
        var x: UInt8 = 0
        repeat {
            x = try byte()
            if s < 64 { r |= Int64(x & 0x7F) << s }
            s += 7
            if s > 70 { throw WasmLector.malo("número LEB128 demasiado largo") }
        } while x & 0x80 != 0
        if s < 64 && (x & 0x40) != 0 { r |= Int64(-1) << s }
        return r
    }

    mutating func s32() throws -> Int32 { Int32(truncatingIfNeeded: try sleb()) }

    mutating func fijo32() throws -> UInt32 {
        guard p + 4 <= fin else { throw WasmLector.malo("se acabaron los bytes antes de tiempo") }
        var v: UInt32 = 0
        for i in 0 ..< 4 { v |= UInt32(b[p + i]) << UInt32(8 * i) }
        p += 4
        return v
    }

    mutating func fijo64() throws -> UInt64 {
        let lo = UInt64(try fijo32())
        let hi = UInt64(try fijo32())
        return lo | (hi << 32)
    }

    mutating func nombre() throws -> String {
        let n = try entero()
        guard p + n <= fin else { throw WasmLector.malo("nombre cortado") }
        let s = String(decoding: b[p ..< p + n], as: UTF8.self)
        p += n
        return s
    }

    mutating func bytes(_ n: Int) throws -> [UInt8] {
        guard n >= 0, p + n <= fin else { throw WasmLector.malo("datos cortados") }
        let r = Array(b[p ..< p + n])
        p += n
        return r
    }

    mutating func limites() throws -> WasmLimites {
        let f = try byte()
        guard f & 0x04 == 0 else { throw WasmTrap("memoria de 64 bits (memory64): no soportada") }
        let mn = UInt64(try u32())
        let mx: UInt64? = (f & 1) != 0 ? UInt64(try u32()) : nil
        return WasmLimites(min: mn, max: mx)
    }

    /// Expresión constante terminada en 'end' (0x0B).
    mutating func exprConst() throws -> [WasmOpConst] {
        var ops: [WasmOpConst] = []
        while true {
            let op = try byte()
            switch op {
            case 0x0B: return ops
            case 0x41: ops.append(WasmOpConst(op: op, v: UInt64(UInt32(bitPattern: try s32()))))
            case 0x42: ops.append(WasmOpConst(op: op, v: UInt64(bitPattern: try sleb())))
            case 0x43: ops.append(WasmOpConst(op: op, v: UInt64(try fijo32())))
            case 0x44: ops.append(WasmOpConst(op: op, v: try fijo64()))
            case 0x23, 0xD2: ops.append(WasmOpConst(op: op, v: UInt64(try u32())))
            case 0xD0: _ = try byte(); ops.append(WasmOpConst(op: op, v: 0))
            case 0x6A, 0x6B, 0x6C, 0x7C, 0x7D, 0x7E: ops.append(WasmOpConst(op: op, v: 0))
            default: throw WasmLector.malo("instrucción 0x" + String(op, radix: 16, uppercase: true) + " en una expresión constante")
            }
        }
    }
}

// ============================================================
// MARK: - Módulo (decodificado)
// ============================================================

final class WasmModulo {
    let bytes: [UInt8]
    var tipos: [WasmFuncTipo] = []
    var importaciones: [WasmImport] = []
    var funciones: [Int] = []
    var tablas: [(tipo: UInt8, lim: WasmLimites)] = []
    var memorias: [WasmLimites] = []
    var globales: [WasmGlobalDef] = []
    var exportaciones: [WasmExport] = []
    var inicio: Int? = nil
    var elementos: [WasmElem] = []
    var codigos: [WasmCodigo] = []
    var datos: [WasmDato] = []
    var personalizadas: [String] = []

    var nImpFunc = 0
    var nImpTabla = 0
    var nImpMem = 0
    var nImpGlobal = 0

    /// Índice de tipo de cada función (importadas primero).
    var tipoDeFuncion: [Int] = []

    init(_ b: [UInt8]) throws {
        bytes = b
        guard b.count >= 8, b[0] == 0, b[1] == 0x61, b[2] == 0x73, b[3] == 0x6D else {
            throw WasmTrap("no es un módulo WebAssembly (falta la cabecera \\0asm)")
        }
        guard b[4] == 1, b[5] == 0, b[6] == 0, b[7] == 0 else {
            throw WasmTrap("versión de WebAssembly no soportada (solo la 1; los 'components' no)")
        }
        var r = WasmLector(b, 8)
        while !r.alFinal {
            let id = try r.byte()
            let tam = try r.entero()
            let fin = r.p + tam
            guard fin <= b.count else { throw WasmLector.malo("sección \(id) cortada") }
            var s = WasmLector(b, r.p, fin)
            try seccion(id, &s)
            r.p = fin
        }
        guard codigos.count == funciones.count else {
            throw WasmLector.malo("hay \(funciones.count) funciones pero \(codigos.count) cuerpos")
        }
        tipoDeFuncion = importaciones.filter { $0.clase == 0 }.map { $0.tipo } + funciones
        for t in tipoDeFuncion where t >= tipos.count {
            throw WasmLector.malo("índice de tipo \(t) fuera de rango")
        }
    }

    private func seccion(_ id: UInt8, _ s: inout WasmLector) throws {
        switch id {
        case 0:
            if let n = try? s.nombre() { personalizadas.append(n) }
        case 1: try leeTipos(&s)
        case 2: try leeImportaciones(&s)
        case 3:
            let n = try s.entero()
            for _ in 0 ..< n { funciones.append(try s.entero()) }
        case 4:
            let n = try s.entero()
            for _ in 0 ..< n {
                let t = try s.byte()
                tablas.append((tipo: t, lim: try s.limites()))
            }
        case 5:
            let n = try s.entero()
            for _ in 0 ..< n { memorias.append(try s.limites()) }
        case 6:
            let n = try s.entero()
            for _ in 0 ..< n {
                let t = try s.byte()
                let mu = try s.byte() == 1
                globales.append(WasmGlobalDef(tipo: t, mutable: mu, inicio: try s.exprConst()))
            }
        case 7:
            let n = try s.entero()
            for _ in 0 ..< n {
                let nom = try s.nombre()
                let c = try s.byte()
                exportaciones.append(WasmExport(nombre: nom, clase: c, indice: try s.entero()))
            }
        case 8: inicio = try s.entero()
        case 9: try leeElementos(&s)
        case 10: try leeCodigo(&s)
        case 11: try leeDatos(&s)
        case 12: _ = try s.u32()
        case 13: throw WasmTrap("el módulo usa excepciones (sección tag): no soportado")
        default: throw WasmLector.malo("sección desconocida \(id)")
        }
    }

    private func leeTipos(_ s: inout WasmLector) throws {
        let n = try s.entero()
        for _ in 0 ..< n {
            let f = try s.byte()
            guard f == 0x60 else { throw WasmLector.malo("tipo que no es de función (0x\(String(f, radix: 16)))") }
            let np = try s.entero()
            var ps: [UInt8] = []
            for _ in 0 ..< np { ps.append(try s.byte()) }
            let nr = try s.entero()
            var rs: [UInt8] = []
            for _ in 0 ..< nr { rs.append(try s.byte()) }
            tipos.append(WasmFuncTipo(params: ps, results: rs))
        }
    }

    private func leeImportaciones(_ s: inout WasmLector) throws {
        let n = try s.entero()
        for _ in 0 ..< n {
            let mo = try s.nombre()
            let ca = try s.nombre()
            let c = try s.byte()
            var imp = WasmImport(modulo: mo, campo: ca, clase: c)
            switch c {
            case 0:
                imp.tipo = try s.entero()
                nImpFunc += 1
            case 1:
                imp.valTipo = try s.byte()
                imp.lim = try s.limites()
                nImpTabla += 1
            case 2:
                imp.lim = try s.limites()
                nImpMem += 1
            case 3:
                imp.valTipo = try s.byte()
                imp.mutable = try s.byte() == 1
                nImpGlobal += 1
            default:
                throw WasmLector.malo("importación de clase \(c) no soportada (\(mo).\(ca))")
            }
            importaciones.append(imp)
        }
    }

    private func leeElementos(_ s: inout WasmLector) throws {
        let n = try s.entero()
        for _ in 0 ..< n {
            let f = try s.u32()
            guard f <= 7 else { throw WasmLector.malo("segmento de elementos con banderas \(f)") }
            var e = WasmElem(modo: 0, tabla: 0, offset: [], items: [])
            if f & 1 == 0 {
                if f & 2 != 0 { e.tabla = try s.entero() }
                e.offset = try s.exprConst()
            } else {
                e.modo = (f & 2) != 0 ? 2 : 1
            }
            let conExpr = (f & 4) != 0
            if f & 3 != 0 { _ = try s.byte() }   // elemkind o reftype
            let k = try s.entero()
            for _ in 0 ..< k {
                if conExpr {
                    e.items.append(try s.exprConst())
                } else {
                    e.items.append([WasmOpConst(op: 0xD2, v: UInt64(try s.u32()))])
                }
            }
            elementos.append(e)
        }
    }

    private func leeCodigo(_ s: inout WasmLector) throws {
        let n = try s.entero()
        for _ in 0 ..< n {
            let tam = try s.entero()
            let fin = s.p + tam
            guard fin <= s.fin else { throw WasmLector.malo("cuerpo de función cortado") }
            let grupos = try s.entero()
            var locs: [UInt8] = []
            for _ in 0 ..< grupos {
                let k = try s.entero()
                let t = try s.byte()
                guard locs.count + k <= 50_000 else { throw WasmTrap("función con demasiadas variables locales") }
                locs.append(contentsOf: [UInt8](repeating: t, count: k))
            }
            codigos.append(WasmCodigo(locales: locs, inicio: s.p, fin: fin))
            s.p = fin
        }
    }

    private func leeDatos(_ s: inout WasmLector) throws {
        let n = try s.entero()
        for _ in 0 ..< n {
            let f = try s.u32()
            var d = WasmDato(activo: true, memoria: 0, offset: [], bytes: [])
            switch f {
            case 0: d.offset = try s.exprConst()
            case 1: d.activo = false
            case 2:
                d.memoria = try s.entero()
                d.offset = try s.exprConst()
            default: throw WasmLector.malo("segmento de datos con banderas \(f)")
            }
            d.bytes = try s.bytes(try s.entero())
            datos.append(d)
        }
    }

    /// Exportación de función con ese nombre.
    func exportFuncion(_ nombre: String) -> Int? {
        exportaciones.first { $0.clase == 0 && $0.nombre == nombre }?.indice
    }

    func tipoFuncion(_ f: Int) -> WasmFuncTipo { tipos[tipoDeFuncion[f]] }
}

// ============================================================
// MARK: - Precompilado a instrucciones planas
// ============================================================
// Códigos internos (además de los de wasm, que se usan tal cual):
//   0x100+n  instrucciones 0xFC n (trunc_sat, memory.copy, tablas…)
//   0x200    salta si el tope es 0 (el 'if')
//   0x201    br con copia de resultados (usa la tabla 'saltos')
//   0x202    br_if con copia
//   0x203    br hacia atrás (vuelta de loop: revisa si hay que parar)
//   0x204    br_if hacia atrás
//   0x206    vuelve de la función sin copiar (destino de los br a la
//            función entera; siempre está en el índice 0)

/// Instrucción ya decodificada. 'a' es la altura de la pila (relativa al
/// marco) ANTES de ejecutarla; 'b' el inmediato (constante, índice, destino…).
struct WasmIns {
    var op: UInt32
    var a: UInt32
    var b: UInt64
}

/// Salto que tiene que mover resultados: a dónde, a qué altura y cuántos.
struct WasmSalto {
    var destino: Int32
    var base: Int32
    var aridad: Int32
}

struct WasmFn {
    var entrada: Int
    var np: Int
    var nl: Int
    var nr: Int
    var maxAltura: Int
    var canon: Int
    var host: Int
}

struct WasmCtrl {
    var tipo: Int          // 0 block, 1 loop, 2 if, 3 función
    var base: Int
    var np: Int
    var nr: Int
    var inicio: Int
    var parches: [Int] = []
    var parchesSalto: [Int] = []
    var ifIns: Int = -1
    var muerto = false     // después de br/return/unreachable (pila polimórfica)
}

final class WasmCompilador {
    let m: WasmModulo
    var code: [WasmIns] = [WasmIns(op: 0x206, a: 0, b: 0)]
    var saltos: [WasmSalto] = []
    var canon: [Int] = []
    private var ctrl: [WasmCtrl] = []
    private var cur = 0
    private var maxA = 0
    private var nl = 0
    private var nGlobales = 0
    private var pilaMal = false
    private var r: WasmLector

    init(_ m: WasmModulo) {
        self.m = m
        r = WasmLector(m.bytes)
        nGlobales = m.nImpGlobal + m.globales.count
        for (i, t) in m.tipos.enumerated() {
            canon.append(m.tipos.firstIndex(of: t) ?? i)
        }
    }

    func compila() throws -> [WasmFn] {
        var fns: [WasmFn] = []
        for f in 0 ..< m.tipoDeFuncion.count {
            let t = m.tipoFuncion(f)
            if f < m.nImpFunc {
                let np = t.params.count
                let nr = t.results.count
                fns.append(WasmFn(entrada: 0, np: np, nl: np, nr: nr, maxAltura: max(np, nr) + 1,
                                  canon: canon[m.tipoDeFuncion[f]], host: 9999))
            } else {
                fns.append(try funcion(f, t))
            }
        }
        return fns
    }

    private func funcion(_ f: Int, _ t: WasmFuncTipo) throws -> WasmFn {
        let c = m.codigos[f - m.nImpFunc]
        let np = t.params.count
        nl = np + c.locales.count
        r = WasmLector(m.bytes, c.inicio, c.fin)
        let entrada = code.count
        cur = nl
        maxA = nl
        ctrl = [WasmCtrl(tipo: 3, base: nl, np: 0, nr: t.results.count, inicio: 0)]
        pilaMal = false
        do {
            while true {
                let op = try r.byte()
                if try instruccion(op) { break }
            }
            if pilaMal { throw WasmLector.malo("faltan operandos en la pila") }
        } catch let e as WasmTrap {
            throw WasmTrap(e.m + " (función \(f))")
        }
        let alto = max(maxA, nl + t.results.count) + 2
        return WasmFn(entrada: entrada, np: np, nl: nl, nr: t.results.count, maxAltura: alto,
                      canon: canon[m.tipoDeFuncion[f]], host: -1)
    }

    // ---------- pila estática ----------

    private func emite(_ op: UInt32, _ b: UInt64 = 0) {
        code.append(WasmIns(op: op, a: UInt32(cur), b: b))
    }

    private func push(_ n: Int = 1) {
        cur += n
        if cur > maxA { maxA = cur }
    }

    /// Saca n valores. En código vivo nunca puede bajar de la base del
    /// bloque (si no, una instrucción leería fuera de su marco).
    private func pop(_ n: Int = 1) {
        let i = ctrl.count - 1
        let b = ctrl[i].base
        if cur - n < b {
            if !ctrl[i].muerto { pilaMal = true }
            cur = b
        } else {
            cur -= n
        }
    }

    /// Hacen falta n valores encima de la base del bloque actual.
    private func necesita(_ n: Int, _ tope: Int) {
        let i = ctrl.count - 1
        if !ctrl[i].muerto && tope - n < ctrl[i].base { pilaMal = true }
    }

    /// Después de br/return/unreachable: lo que sigue no se ejecuta.
    private func inalcanzable() {
        let i = ctrl.count - 1
        cur = ctrl[i].base
        ctrl[i].muerto = true
    }

    private func tipoBloque() throws -> (Int, Int) {
        let v = try r.sleb()
        if v == -64 { return (0, 0) }
        if v < 0 { return (0, 1) }
        let i = Int(v)
        guard i < m.tipos.count else { throw WasmLector.malo("tipo de bloque \(i) fuera de rango") }
        return (m.tipos[i].params.count, m.tipos[i].results.count)
    }

    private func abre(_ tipo: Int, _ np: Int, _ nr: Int) {
        necesita(np, cur)
        let base = max(cur - np, ctrl[ctrl.count - 1].base)
        var c = WasmCtrl(tipo: tipo, base: base, np: np, nr: nr, inicio: code.count)
        c.muerto = ctrl[ctrl.count - 1].muerto
        ctrl.append(c)
    }

    private func sino() {
        let i = ctrl.count - 1
        code.append(WasmIns(op: 0x0C, a: UInt32(cur), b: 0))
        ctrl[i].parches.append(code.count - 1)
        if ctrl[i].ifIns >= 0 {
            code[ctrl[i].ifIns].b = UInt64(code.count)
            ctrl[i].ifIns = -1
        }
        cur = ctrl[i].base + ctrl[i].np
        ctrl[i].muerto = ctrl.count >= 2 ? ctrl[ctrl.count - 2].muerto : false
    }

    /// 'end'. Devuelve true cuando termina la función.
    private func fin() -> Bool {
        necesita(ctrl[ctrl.count - 1].nr, cur)
        let c = ctrl.removeLast()
        if c.tipo == 3 {
            code.append(WasmIns(op: 0x0F, a: UInt32(cur), b: UInt64(c.nr)))
            return true
        }
        let d = code.count
        for i in c.parches { code[i].b = UInt64(d) }
        for s in c.parchesSalto { saltos[s].destino = Int32(d) }
        if c.ifIns >= 0 { code[c.ifIns].b = UInt64(d) }
        cur = c.base + c.nr
        if cur > maxA { maxA = cur }
        return false
    }

    private func nuevoSalto(_ idx: Int, _ base: Int, _ k: Int) -> Int {
        let c = ctrl[idx]
        let destino = c.tipo == 1 ? c.inicio : 0
        saltos.append(WasmSalto(destino: Int32(destino), base: Int32(base), aridad: Int32(k)))
        if c.tipo == 0 || c.tipo == 2 { ctrl[idx].parchesSalto.append(saltos.count - 1) }
        return saltos.count - 1
    }

    private func rama(_ d: Int, _ condicional: Bool) throws {
        guard d < ctrl.count else { throw WasmLector.malo("etiqueta de salto \(d) fuera de rango") }
        let idx = ctrl.count - 1 - d
        let c = ctrl[idx]
        let k = c.tipo == 1 ? c.np : c.nr
        let base = c.tipo == 3 ? 0 : c.base
        let tope = condicional ? cur - 1 : cur
        necesita(k + (condicional ? 1 : 0), cur)
        let atras = c.tipo == 1
        if k == 0 || base + k == tope {
            let op: UInt32 = condicional ? (atras ? 0x204 : 0x0D) : (atras ? 0x203 : 0x0C)
            let destino = atras ? c.inicio : 0
            code.append(WasmIns(op: op, a: UInt32(cur), b: UInt64(destino)))
            if c.tipo == 0 || c.tipo == 2 { ctrl[idx].parches.append(code.count - 1) }
        } else {
            let s = nuevoSalto(idx, base, k)
            code.append(WasmIns(op: condicional ? 0x202 : 0x201, a: UInt32(cur), b: UInt64(s)))
        }
    }

    private func tablaRamas() throws {
        let n = try r.entero()
        guard n < 1_000_000 else { throw WasmLector.malo("br_table demasiado grande") }
        var ds: [Int] = []
        for _ in 0 ... n { ds.append(try r.entero()) }
        let inicio = saltos.count
        for d in ds {
            guard d < ctrl.count else { throw WasmLector.malo("etiqueta de br_table fuera de rango") }
            let idx = ctrl.count - 1 - d
            let c = ctrl[idx]
            let k = c.tipo == 1 ? c.np : c.nr
            necesita(k + 1, cur)
            _ = nuevoSalto(idx, c.tipo == 3 ? 0 : c.base, k)
        }
        let b: UInt64 = (UInt64(inicio) << 32) | UInt64(ds.count)
        code.append(WasmIns(op: 0x0E, a: UInt32(cur), b: b))
    }

    private static func esBinario(_ op: UInt8) -> Bool {
        switch op {
        case 0x46 ... 0x4F, 0x51 ... 0x5A, 0x5B ... 0x66, 0x6A ... 0x78, 0x7C ... 0x8A, 0x92 ... 0x98, 0xA0 ... 0xA6:
            return true
        default:
            return false
        }
    }

    // ---------- instrucciones ----------

    private func instruccion(_ op: UInt8) throws -> Bool {
        switch op {
        case 0x00:
            emite(0x00)
            inalcanzable()
        case 0x01:
            break
        case 0x02, 0x03:
            let (np, nr) = try tipoBloque()
            abre(op == 0x02 ? 0 : 1, np, nr)
        case 0x04:
            let (np, nr) = try tipoBloque()
            emite(0x200)
            pop()
            abre(2, np, nr)
            ctrl[ctrl.count - 1].ifIns = code.count - 1
        case 0x05:
            sino()
        case 0x0B:
            return fin()
        case 0x0C:
            try rama(try r.entero(), false)
            inalcanzable()
        case 0x0D:
            try rama(try r.entero(), true)
            pop()
        case 0x0E:
            try tablaRamas()
            inalcanzable()
        case 0x0F:
            necesita(ctrl[0].nr, cur)
            emite(0x0F, UInt64(ctrl[0].nr))
            inalcanzable()
        case 0x10, 0x11:
            try llamada(op)
        case 0x1A:
            pop()
        case 0x1B:
            emite(0x1B)
            pop(3)
            push()
        case 0x1C:
            let n = try r.entero()
            for _ in 0 ..< n { _ = try r.byte() }
            emite(0x1B)
            pop(3)
            push()
        case 0x20 ... 0x26:
            try variable(op)
        case 0x28 ... 0x3E:
            let al = try r.u32()
            if al & 0x40 != 0 { _ = try r.u32() }
            let off = try r.u32()
            emite(UInt32(op), UInt64(off))
            if op >= 0x36 {
                pop(2)
            } else {
                pop()
                push()
            }
        case 0x3F:
            _ = try r.byte()
            emite(0x3F)
            push()
        case 0x40:
            _ = try r.byte()
            emite(0x40)
            pop()
            push()
        default:
            return try instruccion2(op)
        }
        return false
    }

    private func instruccion2(_ op: UInt8) throws -> Bool {
        switch op {
        case 0x41:
            emite(0x41, UInt64(UInt32(bitPattern: try r.s32())))
            push()
        case 0x42:
            emite(0x41, UInt64(bitPattern: try r.sleb()))
            push()
        case 0x43:
            emite(0x41, UInt64(try r.fijo32()))
            push()
        case 0x44:
            emite(0x41, try r.fijo64())
            push()
        case 0x45 ... 0xC4:
            // wrap y reinterpret no tocan los bits: no generan nada
            if op == 0xA7 || (op >= 0xBC && op <= 0xBF) { break }
            emite(UInt32(op))
            pop(WasmCompilador.esBinario(op) ? 2 : 1)
            push()
        case 0xD0:
            _ = try r.byte()
            emite(0x41, 0)
            push()
        case 0xD1:
            emite(0xD1)
            pop()
            push()
        case 0xD2:
            let f = try r.entero()
            emite(0x41, UInt64(f) + 1)
            push()
        case 0xFC:
            try prefijoFC()
        case 0xFD:
            throw WasmTrap("el módulo usa SIMD (0xFD): no soportado")
        default:
            throw WasmLector.malo("instrucción desconocida 0x" + String(op, radix: 16, uppercase: true))
        }
        return false
    }

    private func llamada(_ op: UInt8) throws {
        if op == 0x10 {
            let f = try r.entero()
            guard f < m.tipoDeFuncion.count else { throw WasmLector.malo("call a la función \(f), que no existe") }
            let t = m.tipoFuncion(f)
            emite(0x10, UInt64(f))
            pop(t.params.count)
            push(t.results.count)
        } else {
            let ti = try r.entero()
            let tab = try r.entero()
            guard ti < m.tipos.count else { throw WasmLector.malo("call_indirect con tipo \(ti) fuera de rango") }
            let t = m.tipos[ti]
            emite(0x11, UInt64(canon[ti]) | (UInt64(tab) << 32))
            pop(1 + t.params.count)
            push(t.results.count)
        }
    }

    private func variable(_ op: UInt8) throws {
        let i = try r.entero()
        switch op {
        case 0x20, 0x21, 0x22:
            guard i < nl else { throw WasmLector.malo("variable local \(i) no existe") }
        case 0x23, 0x24:
            guard i < nGlobales else { throw WasmLector.malo("global \(i) no existe") }
        default:
            break
        }
        emite(UInt32(op), UInt64(i))
        switch op {
        case 0x20, 0x23: push()
        case 0x21, 0x24: pop()
        case 0x26: pop(2)
        default:
            pop()
            push()
        }
    }

    private func prefijoFC() throws {
        let sub = try r.u32()
        switch sub {
        case 0 ... 7:
            emite(0x100 + sub)
            pop()
            push()
        case 8:
            let d = try r.entero()
            _ = try r.byte()
            emite(0x108, UInt64(d))
            pop(3)
        case 9, 13:
            emite(0x100 + sub, UInt64(try r.entero()))
        case 10:
            _ = try r.byte()
            _ = try r.byte()
            emite(0x10A)
            pop(3)
        case 11:
            _ = try r.byte()
            emite(0x10B)
            pop(3)
        case 12, 14:
            let x = UInt64(try r.entero())
            let y = UInt64(try r.entero())
            emite(0x100 + sub, x | (y << 32))
            pop(3)
        case 15:
            emite(0x10F, UInt64(try r.entero()))
            pop(2)
            push()
        case 16:
            emite(0x110, UInt64(try r.entero()))
            push()
        case 17:
            emite(0x111, UInt64(try r.entero()))
            pop(3)
        default:
            throw WasmTrap("instrucción 0xFC \(sub) no soportada")
        }
    }
}

// ============================================================
// MARK: - Operaciones numéricas
// ============================================================
// 't' apunta justo encima del tope: t[-1] es el último valor.
// Los i32 se leen SIEMPRE truncando (los 32 bits de arriba pueden
// traer basura y da igual). Los floats van como patrones de bits.

enum WasmNum {
    static let divCero = WasmTrap("trap: división entre cero")
    static let desborde = WasmTrap("trap: desbordamiento entero")
    static let conversion = WasmTrap("trap: conversión a entero no válida (NaN)")

    @inline(__always) static func u32(_ v: UInt64) -> UInt32 { UInt32(truncatingIfNeeded: v) }
    @inline(__always) static func s32(_ v: UInt64) -> Int32 { Int32(truncatingIfNeeded: v) }
    @inline(__always) static func s64(_ v: UInt64) -> Int64 { Int64(bitPattern: v) }
    @inline(__always) static func f32(_ v: UInt64) -> Float { Float(bitPattern: UInt32(truncatingIfNeeded: v)) }
    @inline(__always) static func f64(_ v: UInt64) -> Double { Double(bitPattern: v) }
    @inline(__always) static func b(_ c: Bool) -> UInt64 { c ? 1 : 0 }
    /// Dirección (i32 sin signo) como Int.
    @inline(__always) static func dir(_ v: UInt64) -> Int { Int(UInt32(truncatingIfNeeded: v)) }
    @inline(__always) static func bits(_ f: Float) -> UInt64 { UInt64(f.bitPattern) }

    static func ejecuta(_ op: UInt32, _ t: UnsafeMutablePointer<UInt64>) throws {
        switch op {
        case 0x45 ... 0x50: cmpEnteros(op, t)
        case 0x51 ... 0x5A: cmpI64(op, t)
        case 0x5B ... 0x66: cmpFlotantes(op, t)
        case 0x67 ... 0x78: try i32(op, t)
        case 0x79 ... 0x8A: try i64(op, t)
        case 0x8B ... 0x98: opF32(op, t)
        case 0x99 ... 0xA6: opF64(op, t)
        case 0xA7 ... 0xB1: try aEntero(op, t)
        case 0xB2 ... 0xC4: aFlotante(op, t)
        case 0x100 ... 0x107: saturado(op, t)
        case 0xD1: t[-1] = b(t[-1] == 0)
        default: throw WasmTrap("instrucción interna \(op) desconocida")
        }
    }

    static func cmpEnteros(_ op: UInt32, _ t: UnsafeMutablePointer<UInt64>) {
        if op == 0x45 { t[-1] = b(u32(t[-1]) == 0); return }
        if op == 0x50 { t[-1] = b(t[-1] == 0); return }
        let x = u32(t[-2])
        let y = u32(t[-1])
        let sx = Int32(bitPattern: x)
        let sy = Int32(bitPattern: y)
        var r = false
        switch op {
        case 0x46: r = x == y
        case 0x47: r = x != y
        case 0x48: r = sx < sy
        case 0x49: r = x < y
        case 0x4A: r = sx > sy
        case 0x4B: r = x > y
        case 0x4C: r = sx <= sy
        case 0x4D: r = x <= y
        case 0x4E: r = sx >= sy
        default: r = x >= y
        }
        t[-2] = b(r)
    }

    static func cmpI64(_ op: UInt32, _ t: UnsafeMutablePointer<UInt64>) {
        let x = t[-2]
        let y = t[-1]
        let sx = Int64(bitPattern: x)
        let sy = Int64(bitPattern: y)
        var r = false
        switch op {
        case 0x51: r = x == y
        case 0x52: r = x != y
        case 0x53: r = sx < sy
        case 0x54: r = x < y
        case 0x55: r = sx > sy
        case 0x56: r = x > y
        case 0x57: r = sx <= sy
        case 0x58: r = x <= y
        case 0x59: r = sx >= sy
        default: r = x >= y
        }
        t[-2] = b(r)
    }

    static func cmpFlotantes(_ op: UInt32, _ t: UnsafeMutablePointer<UInt64>) {
        var r = false
        if op <= 0x60 {
            let x = f32(t[-2])
            let y = f32(t[-1])
            switch op {
            case 0x5B: r = x == y
            case 0x5C: r = x != y
            case 0x5D: r = x < y
            case 0x5E: r = x > y
            case 0x5F: r = x <= y
            default: r = x >= y
            }
        } else {
            let x = f64(t[-2])
            let y = f64(t[-1])
            switch op {
            case 0x61: r = x == y
            case 0x62: r = x != y
            case 0x63: r = x < y
            case 0x64: r = x > y
            case 0x65: r = x <= y
            default: r = x >= y
            }
        }
        t[-2] = b(r)
    }

    static func i32(_ op: UInt32, _ t: UnsafeMutablePointer<UInt64>) throws {
        let y = u32(t[-1])
        switch op {
        case 0x67: t[-1] = UInt64(y.leadingZeroBitCount); return
        case 0x68: t[-1] = UInt64(y.trailingZeroBitCount); return
        case 0x69: t[-1] = UInt64(y.nonzeroBitCount); return
        default: break
        }
        let x = u32(t[-2])
        let k = y & 31
        var r: UInt32 = 0
        switch op {
        case 0x6A: r = x &+ y
        case 0x6B: r = x &- y
        case 0x6C: r = x &* y
        case 0x6D:
            let a = Int32(bitPattern: x)
            let d = Int32(bitPattern: y)
            if d == 0 { throw divCero }
            if a == Int32.min && d == -1 { throw desborde }
            r = UInt32(bitPattern: a / d)
        case 0x6E:
            if y == 0 { throw divCero }
            r = x / y
        case 0x6F:
            let a = Int32(bitPattern: x)
            let d = Int32(bitPattern: y)
            if d == 0 { throw divCero }
            r = d == -1 ? 0 : UInt32(bitPattern: a % d)
        case 0x70:
            if y == 0 { throw divCero }
            r = x % y
        case 0x71: r = x & y
        case 0x72: r = x | y
        case 0x73: r = x ^ y
        case 0x74: r = x << k
        case 0x75: r = UInt32(bitPattern: Int32(bitPattern: x) >> Int32(k))
        case 0x76: r = x >> k
        case 0x77: r = (x << k) | (x >> ((32 - k) & 31))
        default: r = (x >> k) | (x << ((32 - k) & 31))
        }
        t[-2] = UInt64(r)
    }

    static func i64(_ op: UInt32, _ t: UnsafeMutablePointer<UInt64>) throws {
        let y = t[-1]
        switch op {
        case 0x79: t[-1] = UInt64(y.leadingZeroBitCount); return
        case 0x7A: t[-1] = UInt64(y.trailingZeroBitCount); return
        case 0x7B: t[-1] = UInt64(y.nonzeroBitCount); return
        default: break
        }
        let x = t[-2]
        let k = y & 63
        var r: UInt64 = 0
        switch op {
        case 0x7C: r = x &+ y
        case 0x7D: r = x &- y
        case 0x7E: r = x &* y
        case 0x7F:
            let a = Int64(bitPattern: x)
            let d = Int64(bitPattern: y)
            if d == 0 { throw divCero }
            if a == Int64.min && d == -1 { throw desborde }
            r = UInt64(bitPattern: a / d)
        case 0x80:
            if y == 0 { throw divCero }
            r = x / y
        case 0x81:
            let a = Int64(bitPattern: x)
            let d = Int64(bitPattern: y)
            if d == 0 { throw divCero }
            r = d == -1 ? 0 : UInt64(bitPattern: a % d)
        case 0x82:
            if y == 0 { throw divCero }
            r = x % y
        case 0x83: r = x & y
        case 0x84: r = x | y
        case 0x85: r = x ^ y
        case 0x86: r = x << k
        case 0x87: r = UInt64(bitPattern: Int64(bitPattern: x) >> Int64(k))
        case 0x88: r = x >> k
        case 0x89: r = (x << k) | (x >> ((64 - k) & 63))
        default: r = (x >> k) | (x << ((64 - k) & 63))
        }
        t[-2] = r
    }

    // ---------- floats ----------

    static func minimo<T: BinaryFloatingPoint>(_ a: T, _ b: T) -> T {
        if a.isNaN || b.isNaN { return T.nan }
        if a == 0 && b == 0 { return a.sign == .minus ? a : b }
        return a < b ? a : b
    }

    static func maximo<T: BinaryFloatingPoint>(_ a: T, _ b: T) -> T {
        if a.isNaN || b.isNaN { return T.nan }
        if a == 0 && b == 0 { return a.sign == .minus ? b : a }
        return a > b ? a : b
    }

    static func opF32(_ op: UInt32, _ t: UnsafeMutablePointer<UInt64>) {
        let yb = u32(t[-1])
        let y = Float(bitPattern: yb)
        var r: UInt32 = 0
        if op <= 0x91 {
            switch op {
            case 0x8B: r = yb & 0x7FFF_FFFF
            case 0x8C: r = yb ^ 0x8000_0000
            case 0x8D: r = y.rounded(.up).bitPattern
            case 0x8E: r = y.rounded(.down).bitPattern
            case 0x8F: r = y.rounded(.towardZero).bitPattern
            case 0x90: r = y.rounded(.toNearestOrEven).bitPattern
            default: r = y.squareRoot().bitPattern
            }
            t[-1] = UInt64(r)
            return
        }
        let xb = u32(t[-2])
        let x = Float(bitPattern: xb)
        switch op {
        case 0x92: r = (x + y).bitPattern
        case 0x93: r = (x - y).bitPattern
        case 0x94: r = (x * y).bitPattern
        case 0x95: r = (x / y).bitPattern
        case 0x96: r = minimo(x, y).bitPattern
        case 0x97: r = maximo(x, y).bitPattern
        default: r = (xb & 0x7FFF_FFFF) | (yb & 0x8000_0000)
        }
        t[-2] = UInt64(r)
    }

    static func opF64(_ op: UInt32, _ t: UnsafeMutablePointer<UInt64>) {
        let yb = t[-1]
        let y = Double(bitPattern: yb)
        if op <= 0x9F {
            var r: UInt64 = 0
            switch op {
            case 0x99: r = yb & 0x7FFF_FFFF_FFFF_FFFF
            case 0x9A: r = yb ^ 0x8000_0000_0000_0000
            case 0x9B: r = y.rounded(.up).bitPattern
            case 0x9C: r = y.rounded(.down).bitPattern
            case 0x9D: r = y.rounded(.towardZero).bitPattern
            case 0x9E: r = y.rounded(.toNearestOrEven).bitPattern
            default: r = y.squareRoot().bitPattern
            }
            t[-1] = r
            return
        }
        let xb = t[-2]
        let x = Double(bitPattern: xb)
        var r: UInt64 = 0
        switch op {
        case 0xA0: r = (x + y).bitPattern
        case 0xA1: r = (x - y).bitPattern
        case 0xA2: r = (x * y).bitPattern
        case 0xA3: r = (x / y).bitPattern
        case 0xA4: r = minimo(x, y).bitPattern
        case 0xA5: r = maximo(x, y).bitPattern
        default: r = (xb & 0x7FFF_FFFF_FFFF_FFFF) | (yb & 0x8000_0000_0000_0000)
        }
        t[-2] = r
    }

    // ---------- conversiones ----------

    /// Trunca hacia cero y comprueba que quepa en [lo, hi).
    static func trunca(_ x: Double, _ lo: Double, _ hi: Double) throws -> Double {
        if x.isNaN { throw conversion }
        let v = x.rounded(.towardZero)
        if !(v >= lo && v < hi) { throw desborde }
        return v
    }

    static let dos31: Double = 2147483648.0
    static let dos32: Double = 4294967296.0
    static let dos63: Double = 9223372036854775808.0
    static let dos64: Double = 18446744073709551616.0

    static func aEntero(_ op: UInt32, _ t: UnsafeMutablePointer<UInt64>) throws {
        let v = t[-1]
        var r: UInt64 = 0
        switch op {
        case 0xA7: r = UInt64(u32(v))
        case 0xA8: r = UInt64(UInt32(bitPattern: Int32(try trunca(Double(f32(v)), -dos31, dos31))))
        case 0xA9: r = UInt64(UInt32(try trunca(Double(f32(v)), 0, dos32)))
        case 0xAA: r = UInt64(UInt32(bitPattern: Int32(try trunca(f64(v), -dos31, dos31))))
        case 0xAB: r = UInt64(UInt32(try trunca(f64(v), 0, dos32)))
        case 0xAC: r = UInt64(bitPattern: Int64(s32(v)))
        case 0xAD: r = UInt64(u32(v))
        case 0xAE: r = UInt64(bitPattern: Int64(try trunca(Double(f32(v)), -dos63, dos63)))
        case 0xAF: r = UInt64(try trunca(Double(f32(v)), 0, dos64))
        case 0xB0: r = UInt64(bitPattern: Int64(try trunca(f64(v), -dos63, dos63)))
        default: r = UInt64(try trunca(f64(v), 0, dos64))
        }
        t[-1] = r
    }

    static func aFlotante(_ op: UInt32, _ t: UnsafeMutablePointer<UInt64>) {
        let v = t[-1]
        var r: UInt64 = v
        switch op {
        case 0xB2: r = bits(Float(s32(v)))
        case 0xB3: r = bits(Float(u32(v)))
        case 0xB4: r = bits(Float(s64(v)))
        case 0xB5: r = bits(Float(v))
        case 0xB6: r = bits(Float(f64(v)))
        case 0xB7: r = Double(s32(v)).bitPattern
        case 0xB8: r = Double(u32(v)).bitPattern
        case 0xB9: r = Double(s64(v)).bitPattern
        case 0xBA: r = Double(v).bitPattern
        case 0xBB: r = Double(f32(v)).bitPattern
        case 0xC0: r = UInt64(UInt32(bitPattern: Int32(Int8(truncatingIfNeeded: v))))
        case 0xC1: r = UInt64(UInt32(bitPattern: Int32(Int16(truncatingIfNeeded: v))))
        case 0xC2: r = UInt64(bitPattern: Int64(Int8(truncatingIfNeeded: v)))
        case 0xC3: r = UInt64(bitPattern: Int64(Int16(truncatingIfNeeded: v)))
        case 0xC4: r = UInt64(bitPattern: Int64(Int32(truncatingIfNeeded: v)))
        default: break
        }
        t[-1] = r
    }

    /// Conversiones que no atrapan: NaN → 0 y se satura en los extremos.
    static func saturado(_ op: UInt32, _ t: UnsafeMutablePointer<UInt64>) {
        let v = t[-1]
        let x: Double = (op & 2) == 0 ? Double(f32(v)) : f64(v)
        let sinSigno = (op & 1) == 1
        let es64 = op >= 0x104
        var r: UInt64 = 0
        if x.isNaN {
            r = 0
        } else if es64 {
            if sinSigno {
                r = x <= 0 ? 0 : (x >= dos64 ? UInt64.max : UInt64(x))
            } else {
                let s: Int64 = x < -dos63 ? Int64.min : (x >= dos63 ? Int64.max : Int64(x))
                r = UInt64(bitPattern: s)
            }
        } else {
            if sinSigno {
                r = x <= 0 ? 0 : (x >= dos32 ? UInt64(UInt32.max) : UInt64(UInt32(x)))
            } else {
                let s: Int32 = x < -dos31 ? Int32.min : (x >= dos31 ? Int32.max : Int32(x))
                r = UInt64(UInt32(bitPattern: s))
            }
        }
        t[-1] = r
    }
}

// ============================================================
// MARK: - Máquina: instancia y bucle principal
// ============================================================

struct WasmMarco {
    var pc: Int
    var fp: Int
}

final class WasmFd {
    var tipo: Int            // 2 terminal, 3 carpeta, 4 archivo
    var vpath: String
    var url: URL?
    var datos: [UInt8] = []
    var pos = 0
    var escribe = false
    var agrega = false
    var sucio = false

    init(tipo: Int, vpath: String, url: URL?) {
        self.tipo = tipo
        self.vpath = vpath
        self.url = url
    }

    /// Guarda en disco lo escrito (al cerrar y al terminar).
    func guarda() {
        guard sucio, let u = url else { return }
        try? Data(datos).write(to: u)
        sucio = false
    }
}

final class WasmMaquina: @unchecked Sendable {
    let modulo: WasmModulo

    /// Trozos de stdout / stderr (texto ya en UTF-8 completo).
    var salida: (String) -> Void = { _ in }
    var errores: (String) -> Void = { _ in }
    /// Siguiente línea de stdin CON su "\n", o nil si se acabó.
    var entrada: () -> String? = { nil }
    /// Se consulta cada tanto; si da true el programa para con "interrumpido".
    var cancelado: () -> Bool = { false }
    var argumentos: [String] = ["programa"]
    var variables: [String] = []
    /// Para path_open: la carpeta virtual de la shell. Sin ella no hay archivos.
    var entorno: ShellEnv? = nil

    let codigo: UnsafeMutablePointer<WasmIns>
    let nCodigo: Int
    let saltos: UnsafeMutablePointer<WasmSalto>
    let nSaltos: Int
    let fns: UnsafeMutablePointer<WasmFn>
    let nFns: Int
    let pilaCap = 1 << 20
    let pila: UnsafeMutablePointer<UInt64>
    let profMax = 100_000
    let marcos: UnsafeMutablePointer<WasmMarco>
    let globales: UnsafeMutablePointer<UInt64>
    let nGlobales: Int
    var mem: UnsafeMutableRawPointer
    var memTam = 0
    var memMax = 0
    var tablas: [[UInt64]] = []
    var tablasMax: [Int] = []
    var datosVivos: [[UInt8]] = []
    var elemVivos: [[UInt64]] = []
    var nombresImport: [String] = []
    var notaImport: [String] = []
    /// 1 GiB como mucho (en un iPad ya es muchísimo).
    static let paginasMax = 16384

    // estado de WASI
    var fds: [Int: WasmFd] = [:]
    var siguienteFd = 5
    var pendiente1: [UInt8] = []
    var pendiente2: [UInt8] = []
    var bufEntrada: [UInt8] = []
    var finEntrada = false

    init(_ m: WasmModulo) throws {
        modulo = m
        let comp = WasmCompilador(m)
        var fs = try comp.compila()
        var k = 0
        for imp in m.importaciones where imp.clase == 0 {
            let (h, nota) = WasmMaquina.buscaHost(imp, m.tipos[imp.tipo])
            fs[k].host = h
            nombresImport.append(imp.nombre)
            notaImport.append(nota)
            k += 1
        }
        nCodigo = comp.code.count
        codigo = UnsafeMutablePointer<WasmIns>.allocate(capacity: nCodigo)
        codigo.initialize(from: comp.code, count: nCodigo)
        nSaltos = comp.saltos.count
        saltos = UnsafeMutablePointer<WasmSalto>.allocate(capacity: max(nSaltos, 1))
        saltos.initialize(from: comp.saltos, count: nSaltos)
        nFns = fs.count
        fns = UnsafeMutablePointer<WasmFn>.allocate(capacity: max(nFns, 1))
        fns.initialize(from: fs, count: nFns)
        pila = UnsafeMutablePointer<UInt64>.allocate(capacity: pilaCap)
        pila.initialize(repeating: 0, count: pilaCap)
        marcos = UnsafeMutablePointer<WasmMarco>.allocate(capacity: profMax)
        marcos.initialize(repeating: WasmMarco(pc: 0, fp: 0), count: profMax)
        nGlobales = m.nImpGlobal + m.globales.count
        globales = UnsafeMutablePointer<UInt64>.allocate(capacity: max(nGlobales, 1))
        globales.initialize(repeating: 0, count: max(nGlobales, 1))
        mem = calloc(8, 1)!
    }

    deinit {
        codigo.deallocate()
        saltos.deallocate()
        fns.deallocate()
        pila.deallocate()
        marcos.deallocate()
        globales.deallocate()
        free(mem)
    }

    static func fuera(_ ea: Int) -> WasmTrap {
        WasmTrap("trap: acceso a memoria fuera de rango (dirección \(ea))")
    }

    static let pilaLlena = WasmTrap("trap: desbordamiento de pila (demasiadas llamadas anidadas)")

    // ---------- instanciar ----------

    /// Memoria, tablas, globales, segmentos y función de inicio.
    func instancia() throws {
        let m = modulo
        preparaArchivos()
        let limMem: WasmLimites? = m.importaciones.first(where: { $0.clase == 2 })?.lim ?? m.memorias.first
        if let lim = limMem {
            let pags = Int(lim.min)
            guard pags <= WasmMaquina.paginasMax else {
                throw WasmTrap("el módulo pide \(pags) páginas de memoria (máximo \(WasmMaquina.paginasMax))")
            }
            memMax = min(Int(lim.max ?? 65536), WasmMaquina.paginasMax)
            if pags > 0 {
                guard let p = calloc(pags * 65536, 1) else { throw WasmTrap("no hay memoria para el módulo") }
                free(mem)
                mem = p
                memTam = pags * 65536
            }
        }
        for imp in m.importaciones where imp.clase == 1 {
            tablas.append([UInt64](repeating: 0, count: Int(min(imp.lim.min, 10_000_000))))
            tablasMax.append(Int(min(imp.lim.max ?? 10_000_000, 10_000_000)))
        }
        for t in m.tablas {
            tablas.append([UInt64](repeating: 0, count: Int(min(t.lim.min, 10_000_000))))
            tablasMax.append(Int(min(t.lim.max ?? 10_000_000, 10_000_000)))
        }
        var gi = m.nImpGlobal
        for g in m.globales {
            globales[gi] = evalua(g.inicio)
            gi += 1
        }
        try segmentos()
        if let s = m.inicio {
            guard s < nFns else { throw WasmLector.malo("función de inicio \(s) no existe") }
            _ = try invoca(s, [])
        }
    }

    private func segmentos() throws {
        let m = modulo
        elemVivos = m.elementos.map { e in e.items.map { evalua($0) } }
        for (i, e) in m.elementos.enumerated() {
            if e.modo == 0 {
                let off = Int(UInt32(truncatingIfNeeded: evalua(e.offset)))
                let items = elemVivos[i]
                guard e.tabla < tablas.count, off + items.count <= tablas[e.tabla].count else {
                    throw WasmTrap("trap: segmento de elementos fuera de la tabla")
                }
                for (j, v) in items.enumerated() { tablas[e.tabla][off + j] = v }
            }
            if e.modo != 1 { elemVivos[i] = [] }
        }
        datosVivos = m.datos.map { $0.bytes }
        for (i, d) in m.datos.enumerated() where d.activo {
            let off = Int(UInt32(truncatingIfNeeded: evalua(d.offset)))
            guard off + d.bytes.count <= memTam else {
                throw WasmTrap("trap: segmento de datos fuera de la memoria (dirección \(off))")
            }
            if !d.bytes.isEmpty {
                d.bytes.withUnsafeBytes { src in
                    if let base = src.baseAddress { (mem + off).copyMemory(from: base, byteCount: d.bytes.count) }
                }
            }
            datosVivos[i] = []
        }
    }

    /// Evalúa una expresión constante.
    func evalua(_ ops: [WasmOpConst]) -> UInt64 {
        var st: [UInt64] = []
        for o in ops {
            switch o.op {
            case 0x23:
                let g = Int(o.v)
                st.append(g < nGlobales ? globales[g] : 0)
            case 0xD0: st.append(0)
            case 0xD2: st.append(o.v + 1)
            case 0x6A, 0x6B, 0x6C, 0x7C, 0x7D, 0x7E:
                let y = st.popLast() ?? 0
                let x = st.popLast() ?? 0
                switch o.op {
                case 0x6A, 0x7C: st.append(x &+ y)
                case 0x6B, 0x7D: st.append(x &- y)
                default: st.append(x &* y)
                }
            default: st.append(o.v)
            }
        }
        return st.last ?? 0
    }

    /// Cambia el tamaño de la memoria. Devuelve las páginas viejas o -1.
    func crece(_ n: Int) -> UInt32 {
        let viejas = memTam / 65536
        if n == 0 { return UInt32(viejas) }
        let nuevas = viejas + n
        guard nuevas <= memMax, modulo.memorias.count + modulo.nImpMem > 0 else { return UInt32.max }
        guard let p = realloc(mem, nuevas * 65536) else { return UInt32.max }
        memset(p + memTam, 0, n * 65536)
        mem = p
        memTam = nuevas * 65536
        return UInt32(viejas)
    }

    // ---------- llamar desde fuera ----------

    /// Llama a la función f con esos argumentos y devuelve los resultados.
    func invoca(_ f: Int, _ args: [UInt64]) throws -> [UInt64] {
        let fn = fns[f]
        for (i, a) in args.enumerated() where i < fn.np { pila[i] = a }
        if fn.host >= 0 {
            try llamarHost(fn.host, f, pila)
        } else {
            var i = fn.np
            while i < fn.nl {
                pila[i] = 0
                i += 1
            }
            try ejecuta(fn.entrada, pila)
        }
        var r: [UInt64] = []
        for i in 0 ..< fn.nr { r.append(pila[i]) }
        return r
    }

    func revisaCancelado() throws {
        if cancelado() { throw WasmTrap("interrumpido") }
    }

    /// Cuenta saltos hacia atrás y llamadas; cada tanto mira si hay que parar.
    @inline(__always)
    func paso(_ presupuesto: inout Int) throws {
        presupuesto -= 1
        if presupuesto <= 0 {
            presupuesto = 20_000
            try revisaCancelado()
        }
    }

    // ---------- el bucle ----------

    func ejecuta(_ pc0: Int, _ fp0: UnsafeMutablePointer<UInt64>) throws {
        let code = codigo
        let pila = self.pila
        let fns = self.fns
        let glob = globales
        let marcos = self.marcos
        let saltos = self.saltos
        let limite = pilaCap
        let profMax = self.profMax
        var mem = self.mem
        var memTam = self.memTam
        var pc = pc0
        var fp = fp0
        var prof = 0
        var presupuesto = 20_000
        while true {
            let ins = code[pc]
            pc += 1
            let t = fp + Int(ins.a)
            let ib = Int(truncatingIfNeeded: ins.b)
            switch ins.op {
            case 0x20:
                t.pointee = fp[ib]
            case 0x21, 0x22:
                fp[ib] = t[-1]
            case 0x41:
                t.pointee = ins.b
            case 0x23:
                t.pointee = glob[ib]
            case 0x24:
                glob[ib] = t[-1]
            case 0x0C:
                pc = ib
            case 0x0D:
                if WasmRapido.cierto(t) { pc = ib }
            case 0x200:
                if !WasmRapido.cierto(t) { pc = ib }
            case 0x203:
                pc = ib
                try paso(&presupuesto)
            case 0x204:
                if WasmRapido.cierto(t) {
                    pc = ib
                    try paso(&presupuesto)
                }
            case 0x201, 0x202:
                if ins.op == 0x202 && !WasmRapido.cierto(t) { break }
                let s = saltos[ib]
                let k = Int(s.aridad)
                WasmRapido.copia(fp + Int(s.base), ins.op == 0x202 ? t - 1 - k : t - k, k)
                pc = Int(s.destino)
                try paso(&presupuesto)
            case 0x0E:
                let s = saltos[WasmRapido.entradaTabla(ins.b, t)]
                let k = Int(s.aridad)
                WasmRapido.copia(fp + Int(s.base), t - 1 - k, k)
                pc = Int(s.destino)
                try paso(&presupuesto)
            case 0x0F, 0x206:
                if ins.op == 0x0F { WasmRapido.copia(fp, t - ib, ib) }
                if prof == 0 { return }
                prof -= 1
                pc = marcos[prof].pc
                fp = pila + marcos[prof].fp
            case 0x10, 0x11:
                let directa = ins.op == 0x10
                let fi = directa ? ib : try indirecta(ins.b, t[-1])
                let f = fns[fi]
                let nfp = (directa ? t : t - 1) - f.np
                if f.host >= 0 {
                    try llamarHost(f.host, fi, nfp)
                    mem = self.mem
                    memTam = self.memTam
                    break
                }
                if prof >= profMax || (nfp - pila) + f.maxAltura >= limite { throw WasmMaquina.pilaLlena }
                marcos[prof] = WasmMarco(pc: pc, fp: fp - pila)
                prof += 1
                fp = nfp
                WasmRapido.ceros(fp + f.np, f.nl - f.np)
                pc = f.entrada
                try paso(&presupuesto)
            case 0x1B:
                WasmRapido.elige(t)
            case 0x00:
                throw WasmTrap("trap: unreachable (se llegó a código inalcanzable)")
            case 0x28, 0x2A, 0x35:
                try WasmRapido.mem28_2A_35(t, ib, mem, memTam)
            case 0x29, 0x2B:
                try WasmRapido.mem29_2B(t, ib, mem, memTam)
            case 0x2D, 0x31:
                try WasmRapido.mem2D_31(t, ib, mem, memTam)
            case 0x36, 0x38, 0x3E:
                try WasmRapido.mem36_38_3E(t, ib, mem, memTam)
            case 0x37, 0x39:
                try WasmRapido.mem37_39(t, ib, mem, memTam)
            case 0x3A, 0x3C:
                try WasmRapido.mem3A_3C(t, ib, mem, memTam)
            case 0x2C, 0x2E, 0x2F, 0x30, 0x32, 0x33, 0x34, 0x3B, 0x3D:
                try WasmMaquina.memoriaRara(ins.op, t, ib, mem, memTam)
            case 0x3F:
                t.pointee = UInt64(memTam / 65536)
            case 0x40:
                t[-1] = UInt64(crece(WasmNum.dir(t[-1])))
                mem = self.mem
                memTam = self.memTam
            case 0x45:
                WasmRapido.op45(t)
            case 0x46:
                WasmRapido.op46(t)
            case 0x47:
                WasmRapido.op47(t)
            case 0x48:
                WasmRapido.op48(t)
            case 0x49:
                WasmRapido.op49(t)
            case 0x4A:
                WasmRapido.op4A(t)
            case 0x4C:
                WasmRapido.op4C(t)
            case 0x4E:
                WasmRapido.op4E(t)
            case 0x6A, 0x7C:
                WasmRapido.op6A_7C(t)
            case 0x6B, 0x7D:
                WasmRapido.op6B_7D(t)
            case 0x6C, 0x7E:
                WasmRapido.op6C_7E(t)
            case 0x71, 0x83:
                WasmRapido.op71_83(t)
            case 0x72, 0x84:
                WasmRapido.op72_84(t)
            case 0x73, 0x85:
                WasmRapido.op73_85(t)
            case 0x74:
                WasmRapido.op74(t)
            case 0x76:
                WasmRapido.op76(t)
            case 0x50:
                WasmRapido.op50(t)
            case 0x51:
                WasmRapido.op51(t)
            case 0x52:
                WasmRapido.op52(t)
            case 0x53:
                WasmRapido.op53(t)
            case 0x54:
                WasmRapido.op54(t)
            case 0x86:
                WasmRapido.op86(t)
            case 0x88:
                WasmRapido.op88(t)
            case 0xAC:
                WasmRapido.opAC(t)
            case 0xAD:
                WasmRapido.opAD(t)
            case 0xA0:
                WasmRapido.opA0(t)
            case 0xA1:
                WasmRapido.opA1(t)
            case 0xA2:
                WasmRapido.opA2(t)
            case 0xA3:
                WasmRapido.opA3(t)
            default:
                if ins.op <= 0x107 {
                    try WasmNum.ejecuta(ins.op, t)
                } else {
                    try extendida(ins, t)
                    mem = self.mem
                    memTam = self.memTam
                }
            }
        }
    }

    /// Cargas y guardados menos frecuentes (16 bits, con signo, f32…).
    static func memoriaRara(_ op: UInt32, _ t: UnsafeMutablePointer<UInt64>, _ off: Int,
                            _ mem: UnsafeMutableRawPointer, _ memTam: Int) throws {
        let esGuardar = op >= 0x36
        let ea = Int(UInt32(truncatingIfNeeded: esGuardar ? t[-2] : t[-1])) &+ off
        var n = 2
        switch op {
        case 0x2C, 0x30: n = 1
        case 0x34: n = 4
        default: break
        }
        if ea &+ n > memTam { throw fuera(ea) }
        switch op {
        case 0x2C: t[-1] = UInt64(UInt32(bitPattern: Int32(mem.load(fromByteOffset: ea, as: Int8.self))))
        case 0x2E: t[-1] = UInt64(UInt32(bitPattern: Int32(mem.loadUnaligned(fromByteOffset: ea, as: Int16.self))))
        case 0x2F: t[-1] = UInt64(mem.loadUnaligned(fromByteOffset: ea, as: UInt16.self))
        case 0x30: t[-1] = UInt64(bitPattern: Int64(mem.load(fromByteOffset: ea, as: Int8.self)))
        case 0x32: t[-1] = UInt64(bitPattern: Int64(mem.loadUnaligned(fromByteOffset: ea, as: Int16.self)))
        case 0x33: t[-1] = UInt64(mem.loadUnaligned(fromByteOffset: ea, as: UInt16.self))
        case 0x34: t[-1] = UInt64(bitPattern: Int64(mem.loadUnaligned(fromByteOffset: ea, as: Int32.self)))
        default:
            mem.storeBytes(of: UInt16(truncatingIfNeeded: t[-1]), toByteOffset: ea, as: UInt16.self)
        }
    }

    /// call_indirect: busca la función en la tabla y comprueba la firma.
    func indirecta(_ b: UInt64, _ v: UInt64) throws -> Int {
        let canon = Int(b & 0xFFFF_FFFF)
        let tab = Int(b >> 32)
        let i = Int(UInt32(truncatingIfNeeded: v))
        guard tab < tablas.count else { throw WasmTrap("trap: la tabla \(tab) no existe") }
        guard i < tablas[tab].count else { throw WasmTrap("trap: índice \(i) fuera de la tabla (call_indirect)") }
        let r = tablas[tab][i]
        guard r != 0 else { throw WasmTrap("trap: elemento \(i) de la tabla vacío (call_indirect)") }
        let fi = Int(r - 1)
        guard fi < nFns else { throw WasmTrap("trap: referencia de función no válida") }
        guard fns[fi].canon == canon else { throw WasmTrap("trap: firma incorrecta en call_indirect (elemento \(i))") }
        return fi
    }

    // ---------- memoria masiva y tablas ----------

    func extendida(_ ins: WasmIns, _ t: UnsafeMutablePointer<UInt64>) throws {
        switch ins.op {
        case 0x108, 0x10A, 0x10B:
            try masiva(ins, t)
        case 0x109:
            let i = Int(ins.b)
            if i < datosVivos.count { datosVivos[i] = [] }
        case 0x10D:
            let i = Int(ins.b)
            if i < elemVivos.count { elemVivos[i] = [] }
        default:
            try opTabla(ins, t)
        }
    }

    private func masiva(_ ins: WasmIns, _ t: UnsafeMutablePointer<UInt64>) throws {
        let d = Int(UInt32(truncatingIfNeeded: t[-3]))
        let s = Int(UInt32(truncatingIfNeeded: t[-2]))
        let n = Int(UInt32(truncatingIfNeeded: t[-1]))
        switch ins.op {
        case 0x108:
            let i = Int(ins.b)
            guard i < datosVivos.count else { throw WasmTrap("trap: segmento de datos \(i) no existe") }
            let seg = datosVivos[i]
            guard s + n <= seg.count, d + n <= memTam else { throw WasmTrap("trap: memory.init fuera de rango") }
            if n > 0 {
                seg.withUnsafeBytes { src in
                    if let base = src.baseAddress { (mem + d).copyMemory(from: base + s, byteCount: n) }
                }
            }
        case 0x10A:
            guard s + n <= memTam, d + n <= memTam else { throw WasmMaquina.fuera(max(s, d) + n) }
            if n > 0 { memmove(mem + d, mem + s, n) }
        default:
            guard d + n <= memTam else { throw WasmMaquina.fuera(d + n) }
            if n > 0 { memset(mem + d, Int32(UInt8(truncatingIfNeeded: t[-2])), n) }
        }
    }

    private func opTabla(_ ins: WasmIns, _ t: UnsafeMutablePointer<UInt64>) throws {
        let tab = Int(ins.b & 0xFFFF_FFFF)
        let fuera = WasmTrap("trap: acceso a tabla fuera de rango")
        switch ins.op {
        case 0x25:
            let i = Int(UInt32(truncatingIfNeeded: t[-1]))
            guard tab < tablas.count, i < tablas[tab].count else { throw fuera }
            t[-1] = tablas[tab][i]
        case 0x26:
            let i = Int(UInt32(truncatingIfNeeded: t[-2]))
            guard tab < tablas.count, i < tablas[tab].count else { throw fuera }
            tablas[tab][i] = t[-1]
        case 0x10C:
            let tb = Int(ins.b >> 32)
            let d = Int(UInt32(truncatingIfNeeded: t[-3]))
            let s = Int(UInt32(truncatingIfNeeded: t[-2]))
            let n = Int(UInt32(truncatingIfNeeded: t[-1]))
            guard tab < elemVivos.count, tb < tablas.count, s + n <= elemVivos[tab].count,
                  d + n <= tablas[tb].count else { throw fuera }
            for j in 0 ..< n { tablas[tb][d + j] = elemVivos[tab][s + j] }
        case 0x10E:
            let ts = Int(ins.b >> 32)
            let d = Int(UInt32(truncatingIfNeeded: t[-3]))
            let s = Int(UInt32(truncatingIfNeeded: t[-2]))
            let n = Int(UInt32(truncatingIfNeeded: t[-1]))
            guard tab < tablas.count, ts < tablas.count, s + n <= tablas[ts].count,
                  d + n <= tablas[tab].count else { throw fuera }
            let copia = Array(tablas[ts][s ..< s + n])
            for j in 0 ..< n { tablas[tab][d + j] = copia[j] }
        case 0x10F:
            guard tab < tablas.count else { throw fuera }
            let n = Int(UInt32(truncatingIfNeeded: t[-1]))
            let viejo = tablas[tab].count
            if viejo + n > tablasMax[tab] {
                t[-2] = UInt64(UInt32.max)
            } else {
                tablas[tab].append(contentsOf: [UInt64](repeating: t[-2], count: n))
                t[-2] = UInt64(viejo)
            }
        case 0x110:
            guard tab < tablas.count else { throw fuera }
            t.pointee = UInt64(tablas[tab].count)
        case 0x111:
            let d = Int(UInt32(truncatingIfNeeded: t[-3]))
            let n = Int(UInt32(truncatingIfNeeded: t[-1]))
            guard tab < tablas.count, d + n <= tablas[tab].count else { throw fuera }
            for j in 0 ..< n { tablas[tab][d + j] = t[-2] }
        default:
            throw WasmTrap("instrucción interna \(ins.op) desconocida")
        }
    }
}

/// Cuerpos de las instrucciones más usadas, en funciones aparte que el
/// optimizador mete dentro del bucle (@inline(__always)): así 'ejecuta'
/// queda con un switch de llamadas simples que el compilador revisa rápido.
enum WasmRapido {
    /// ¿El i32 del tope es distinto de cero?
    @inline(__always) static func cierto(_ t: UnsafeMutablePointer<UInt64>) -> Bool {
        UInt32(truncatingIfNeeded: t[-1]) != 0
    }

    /// select
    @inline(__always) static func elige(_ t: UnsafeMutablePointer<UInt64>) {
        if UInt32(truncatingIfNeeded: t[-1]) == 0 { t[-3] = t[-2] }
    }

    /// Mueve k valores hacia abajo (dst <= src, así que hacia delante sirve).
    @inline(__always) static func copia(_ dst: UnsafeMutablePointer<UInt64>, _ src: UnsafeMutablePointer<UInt64>, _ k: Int) {
        var i = 0
        while i < k {
            dst[i] = src[i]
            i += 1
        }
    }

    @inline(__always) static func ceros(_ p: UnsafeMutablePointer<UInt64>, _ n: Int) {
        var i = 0
        while i < n {
            p[i] = 0
            i += 1
        }
    }

    /// br_table: qué entrada de la tabla de saltos toca (la última es la de por defecto).
    @inline(__always) static func entradaTabla(_ b: UInt64, _ t: UnsafeMutablePointer<UInt64>) -> Int {
        let n = Int(b & 0xFFFF_FFFF)
        var i = WasmNum.dir(t[-1])
        if i >= n - 1 { i = n - 1 }
        return Int(b >> 32) + i
    }

    @inline(__always)
    static func mem28_2A_35(_ t: UnsafeMutablePointer<UInt64>, _ ib: Int, _ mem: UnsafeMutableRawPointer, _ memTam: Int) throws {
        let ea = WasmNum.dir(t[-1]) &+ ib
        if ea &+ 4 > memTam { throw WasmMaquina.fuera(ea) }
        t[-1] = UInt64(mem.loadUnaligned(fromByteOffset: ea, as: UInt32.self))
    }

    @inline(__always)
    static func mem29_2B(_ t: UnsafeMutablePointer<UInt64>, _ ib: Int, _ mem: UnsafeMutableRawPointer, _ memTam: Int) throws {
        let ea = WasmNum.dir(t[-1]) &+ ib
        if ea &+ 8 > memTam { throw WasmMaquina.fuera(ea) }
        t[-1] = mem.loadUnaligned(fromByteOffset: ea, as: UInt64.self)
    }

    @inline(__always)
    static func mem2D_31(_ t: UnsafeMutablePointer<UInt64>, _ ib: Int, _ mem: UnsafeMutableRawPointer, _ memTam: Int) throws {
        let ea = WasmNum.dir(t[-1]) &+ ib
        if ea &+ 1 > memTam { throw WasmMaquina.fuera(ea) }
        t[-1] = UInt64(mem.load(fromByteOffset: ea, as: UInt8.self))
    }

    @inline(__always)
    static func mem36_38_3E(_ t: UnsafeMutablePointer<UInt64>, _ ib: Int, _ mem: UnsafeMutableRawPointer, _ memTam: Int) throws {
        let ea = WasmNum.dir(t[-2]) &+ ib
        if ea &+ 4 > memTam { throw WasmMaquina.fuera(ea) }
        mem.storeBytes(of: WasmNum.u32(t[-1]), toByteOffset: ea, as: UInt32.self)
    }

    @inline(__always)
    static func mem37_39(_ t: UnsafeMutablePointer<UInt64>, _ ib: Int, _ mem: UnsafeMutableRawPointer, _ memTam: Int) throws {
        let ea = WasmNum.dir(t[-2]) &+ ib
        if ea &+ 8 > memTam { throw WasmMaquina.fuera(ea) }
        mem.storeBytes(of: t[-1], toByteOffset: ea, as: UInt64.self)
    }

    @inline(__always)
    static func mem3A_3C(_ t: UnsafeMutablePointer<UInt64>, _ ib: Int, _ mem: UnsafeMutableRawPointer, _ memTam: Int) throws {
        let ea = WasmNum.dir(t[-2]) &+ ib
        if ea &+ 1 > memTam { throw WasmMaquina.fuera(ea) }
        mem.storeBytes(of: UInt8(truncatingIfNeeded: t[-1]), toByteOffset: ea, as: UInt8.self)
    }

    @inline(__always) static func op45(_ t: UnsafeMutablePointer<UInt64>) {
        t[-1] = WasmNum.b(WasmNum.u32(t[-1]) == 0)
    }

    @inline(__always) static func op46(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = WasmNum.b(WasmNum.u32(t[-2]) == WasmNum.u32(t[-1]))
    }

    @inline(__always) static func op47(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = WasmNum.b(WasmNum.u32(t[-2]) != WasmNum.u32(t[-1]))
    }

    @inline(__always) static func op48(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = WasmNum.b(WasmNum.s32(t[-2]) < WasmNum.s32(t[-1]))
    }

    @inline(__always) static func op49(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = WasmNum.b(WasmNum.u32(t[-2]) < WasmNum.u32(t[-1]))
    }

    @inline(__always) static func op4A(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = WasmNum.b(WasmNum.s32(t[-2]) > WasmNum.s32(t[-1]))
    }

    @inline(__always) static func op4C(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = WasmNum.b(WasmNum.s32(t[-2]) <= WasmNum.s32(t[-1]))
    }

    @inline(__always) static func op4E(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = WasmNum.b(WasmNum.s32(t[-2]) >= WasmNum.s32(t[-1]))
    }

    @inline(__always) static func op6A_7C(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = t[-2] &+ t[-1]
    }

    @inline(__always) static func op6B_7D(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = t[-2] &- t[-1]
    }

    @inline(__always) static func op6C_7E(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = t[-2] &* t[-1]
    }

    @inline(__always) static func op71_83(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = t[-2] & t[-1]
    }

    @inline(__always) static func op72_84(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = t[-2] | t[-1]
    }

    @inline(__always) static func op73_85(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = t[-2] ^ t[-1]
    }

    @inline(__always) static func op74(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = UInt64(WasmNum.u32(t[-2]) << (WasmNum.u32(t[-1]) & 31))
    }

    @inline(__always) static func op76(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = UInt64(WasmNum.u32(t[-2]) >> (WasmNum.u32(t[-1]) & 31))
    }

    @inline(__always) static func op50(_ t: UnsafeMutablePointer<UInt64>) {
        t[-1] = WasmNum.b(t[-1] == 0)
    }

    @inline(__always) static func op51(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = WasmNum.b(t[-2] == t[-1])
    }

    @inline(__always) static func op52(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = WasmNum.b(t[-2] != t[-1])
    }

    @inline(__always) static func op53(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = WasmNum.b(WasmNum.s64(t[-2]) < WasmNum.s64(t[-1]))
    }

    @inline(__always) static func op54(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = WasmNum.b(t[-2] < t[-1])
    }

    @inline(__always) static func op86(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = t[-2] << (t[-1] & 63)
    }

    @inline(__always) static func op88(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = t[-2] >> (t[-1] & 63)
    }

    @inline(__always) static func opAC(_ t: UnsafeMutablePointer<UInt64>) {
        t[-1] = UInt64(bitPattern: Int64(WasmNum.s32(t[-1])))
    }

    @inline(__always) static func opAD(_ t: UnsafeMutablePointer<UInt64>) {
        t[-1] = UInt64(WasmNum.u32(t[-1]))
    }

    @inline(__always) static func opA0(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = (WasmNum.f64(t[-2]) + WasmNum.f64(t[-1])).bitPattern
    }

    @inline(__always) static func opA1(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = (WasmNum.f64(t[-2]) - WasmNum.f64(t[-1])).bitPattern
    }

    @inline(__always) static func opA2(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = (WasmNum.f64(t[-2]) * WasmNum.f64(t[-1])).bitPattern
    }

    @inline(__always) static func opA3(_ t: UnsafeMutablePointer<UInt64>) {
        t[-2] = (WasmNum.f64(t[-2]) / WasmNum.f64(t[-1])).bitPattern
    }
}

// ============================================================
// MARK: - Funciones del anfitrión: WASI preview1 y "env"
// ============================================================
// Descriptores: 0 stdin, 1 stdout, 2 stderr, 3 = "/" (la raíz de la
// shell, preabierta), 4 = "." (la carpeta actual). Los archivos se leen
// enteros al abrirlos y se guardan al cerrarlos (o al terminar).
// Cualquier importación que no esté aquí deja instanciar el módulo y
// solo atrapa si de verdad se llama.

extension WasmMaquina {
    /// (módulo, nombre, letras de los parámetros: i=i32 I=i64 f=f32 F=f64)
    static let hosts: [(String, String, String)] = [
        ("wasi_snapshot_preview1", "fd_write", "iiii"),            // 0
        ("wasi_snapshot_preview1", "fd_read", "iiii"),             // 1
        ("wasi_snapshot_preview1", "args_sizes_get", "ii"),        // 2
        ("wasi_snapshot_preview1", "args_get", "ii"),              // 3
        ("wasi_snapshot_preview1", "environ_sizes_get", "ii"),     // 4
        ("wasi_snapshot_preview1", "environ_get", "ii"),           // 5
        ("wasi_snapshot_preview1", "proc_exit", "i"),              // 6
        ("wasi_snapshot_preview1", "clock_time_get", "iIi"),       // 7
        ("wasi_snapshot_preview1", "clock_res_get", "ii"),         // 8
        ("wasi_snapshot_preview1", "random_get", "ii"),            // 9
        ("wasi_snapshot_preview1", "fd_close", "i"),               // 10
        ("wasi_snapshot_preview1", "fd_seek", "iIii"),             // 11
        ("wasi_snapshot_preview1", "fd_fdstat_get", "ii"),         // 12
        ("wasi_snapshot_preview1", "fd_prestat_get", "ii"),        // 13
        ("wasi_snapshot_preview1", "fd_prestat_dir_name", "iii"),  // 14
        ("wasi_snapshot_preview1", "path_open", "iiiiiIIii"),      // 15
        ("wasi_snapshot_preview1", "sched_yield", ""),             // 16
        ("wasi_snapshot_preview1", "poll_oneoff", "iiii"),         // 17
        ("wasi_snapshot_preview1", "fd_filestat_get", "ii"),       // 18
        ("wasi_snapshot_preview1", "path_filestat_get", "iiiii"),  // 19
        ("wasi_snapshot_preview1", "fd_tell", "ii"),               // 20
        ("wasi_snapshot_preview1", "path_create_directory", "iii"), // 21
        ("wasi_snapshot_preview1", "path_unlink_file", "iii"),     // 22
        ("wasi_snapshot_preview1", "path_remove_directory", "iii"), // 23
        ("wasi_snapshot_preview1", "fd_fdstat_set_flags", "ii"),   // 24
        ("wasi_snapshot_preview1", "fd_sync", "i"),                // 25
        ("wasi_snapshot_preview1", "fd_datasync", "i"),            // 26
        ("env", "print_i32", "i"),                                 // 27
        ("env", "print_i64", "I"),                                 // 28
        ("env", "print_f64", "F"),                                 // 29
        ("env", "print_f32", "f"),                                 // 30
        ("env", "putchar", "i"),                                   // 31
        ("env", "print", "ii"),                                    // 32
        ("env", "puts", "i"),                                      // 33
        ("env", "abort", "")                                       // 34 (cualquier firma)
    ]

    /// Qué función del anfitrión atiende una importación (9999 = ninguna).
    static func buscaHost(_ imp: WasmImport, _ tipo: WasmFuncTipo) -> (Int, String) {
        let mo = imp.modulo == "wasi_unstable" ? "wasi_snapshot_preview1" : imp.modulo
        for (i, h) in hosts.enumerated() where h.0 == mo && h.1 == imp.campo {
            if i == 34 || h.2 == tipo.letrasParams { return (i, "") }
            return (9999, "su firma \(tipo.texto) no es la esperada")
        }
        return (9999, "")
    }

    /// Descriptores preabiertos (se llama al instanciar).
    func preparaArchivos() {
        fds[0] = WasmFd(tipo: 2, vpath: "<stdin>", url: nil)
        fds[1] = WasmFd(tipo: 2, vpath: "<stdout>", url: nil)
        fds[2] = WasmFd(tipo: 2, vpath: "<stderr>", url: nil)
        guard let e = entorno else { return }
        fds[3] = WasmFd(tipo: 3, vpath: "/", url: e.root)
        fds[4] = WasmFd(tipo: 3, vpath: e.vpath(e.cwd), url: e.cwd)
    }

    /// Vacía lo pendiente (UTF-8 a medias) y guarda los archivos abiertos.
    func termina() {
        if !pendiente1.isEmpty { salida(String(decoding: pendiente1, as: UTF8.self)) }
        if !pendiente2.isEmpty { errores(String(decoding: pendiente2, as: UTF8.self)) }
        pendiente1 = []
        pendiente2 = []
        for (_, f) in fds { f.guarda() }
    }

    // ---------- memoria del invitado ----------

    @inline(__always) func ptr(_ v: UInt64) -> Int { Int(UInt32(truncatingIfNeeded: v)) }

    func rango(_ d: Int, _ n: Int) throws -> Int {
        guard d >= 0, n >= 0, d + n <= memTam else { throw WasmMaquina.fuera(d) }
        return d
    }

    func lee32(_ d: Int) throws -> Int {
        let p = try rango(d, 4)
        return Int(mem.loadUnaligned(fromByteOffset: p, as: UInt32.self))
    }

    func pon32(_ d: Int, _ v: UInt32) throws {
        let p = try rango(d, 4)
        mem.storeBytes(of: v, toByteOffset: p, as: UInt32.self)
    }

    func pon64(_ d: Int, _ v: UInt64) throws {
        let p = try rango(d, 8)
        mem.storeBytes(of: v, toByteOffset: p, as: UInt64.self)
    }

    func pon16(_ d: Int, _ v: UInt16) throws {
        let p = try rango(d, 2)
        mem.storeBytes(of: v, toByteOffset: p, as: UInt16.self)
    }

    func pon8(_ d: Int, _ v: UInt8) throws {
        let p = try rango(d, 1)
        mem.storeBytes(of: v, toByteOffset: p, as: UInt8.self)
    }

    func leeBytes(_ d: Int, _ n: Int) throws -> [UInt8] {
        let p = try rango(d, n)
        if n == 0 { return [] }
        return Array(UnsafeRawBufferPointer(start: mem + p, count: n))
    }

    func ponBytes(_ d: Int, _ b: ArraySlice<UInt8>) throws {
        let p = try rango(d, b.count)
        var i = p
        for x in b {
            mem.storeBytes(of: x, toByteOffset: i, as: UInt8.self)
            i += 1
        }
    }

    func leeTexto(_ d: Int, _ n: Int) throws -> String {
        String(decoding: try leeBytes(d, n), as: UTF8.self)
    }

    // ---------- salida ----------

    /// Dónde cortar para no partir un carácter UTF-8 a la mitad.
    static func corteUTF8(_ b: [UInt8]) -> Int {
        var i = b.count - 1
        var n = 0
        while i >= 0 && n < 4 {
            let c = b[i]
            if c & 0xC0 == 0x80 {
                i -= 1
                n += 1
                continue
            }
            if c >= 0xC0 {
                let largo = c >= 0xF0 ? 4 : (c >= 0xE0 ? 3 : 2)
                if b.count - i < largo { return i }
            }
            return b.count
        }
        return b.count
    }

    func emite(_ fd: Int, _ bytes: [UInt8]) {
        let buf = (fd == 2 ? pendiente2 : pendiente1) + bytes
        let corte = WasmMaquina.corteUTF8(buf)
        let txt = String(decoding: buf[0 ..< corte], as: UTF8.self)
        let resto = Array(buf[corte...])
        if fd == 2 { pendiente2 = resto } else { pendiente1 = resto }
        if txt.isEmpty { return }
        if fd == 2 { errores(txt) } else { salida(txt) }
    }

    func emiteTexto(_ s: String) { emite(1, Array(s.utf8)) }

    // ---------- despacho ----------

    func llamarHost(_ h: Int, _ fi: Int, _ a: UnsafeMutablePointer<UInt64>) throws {
        var r: UInt32 = 0
        switch h {
        case 0: r = try fdEscribe(a)
        case 1: r = try fdLee(a)
        case 2: r = try tamanos(argumentos, a)
        case 3: r = try copiaLista(argumentos, a)
        case 4: r = try tamanos(variables, a)
        case 5: r = try copiaLista(variables, a)
        case 6: throw WasmSalida(codigo: Int32(truncatingIfNeeded: a[0]))
        case 7: r = try reloj(a)
        case 8: try pon64(ptr(a[1]), 1000)
        case 9: r = try aleatorio(a)
        case 10: r = cierra(Int(UInt32(truncatingIfNeeded: a[0])))
        case 11: r = try busca(a)
        case 12: r = try fdstat(a)
        case 13: r = try prestat(a)
        case 14: r = try prestatNombre(a)
        case 15: r = try abre(a)
        case 16: r = 0
        case 17: r = try sondea(a)
        case 18: r = try filestatFd(a)
        case 19: r = try filestatRuta(a)
        case 20: r = try dice(a)
        case 21, 22, 23: r = try opRuta(h, a)
        case 24: r = 0
        case 25, 26: fds[Int(UInt32(truncatingIfNeeded: a[0]))]?.guarda()
        default:
            try hostEnv(h, fi, a)
            return
        }
        a[0] = UInt64(r)
    }

    private func hostEnv(_ h: Int, _ fi: Int, _ a: UnsafeMutablePointer<UInt64>) throws {
        switch h {
        case 27: emiteTexto("\(Int32(truncatingIfNeeded: a[0]))\n")
        case 28: emiteTexto("\(Int64(bitPattern: a[0]))\n")
        case 29: emiteTexto("\(Double(bitPattern: a[0]))\n")
        case 30: emiteTexto("\(Float(bitPattern: UInt32(truncatingIfNeeded: a[0])))\n")
        case 31: emite(1, [UInt8(truncatingIfNeeded: a[0])])
        case 32: emite(1, try leeBytes(ptr(a[0]), ptr(a[1])))
        case 33:
            var p = ptr(a[0])
            var b: [UInt8] = []
            while true {
                let d = try rango(p, 1)
                let c = mem.load(fromByteOffset: d, as: UInt8.self)
                if c == 0 { break }
                b.append(c)
                p += 1
            }
            b.append(10)
            emite(1, b)
            a[0] = 0
        case 34:
            let t = fi < modulo.tipoDeFuncion.count ? modulo.tipoFuncion(fi) : WasmFuncTipo(params: [], results: [])
            if t.params.count == 4 {
                throw WasmTrap("trap: abort (línea \(Int32(truncatingIfNeeded: a[2])), columna \(Int32(truncatingIfNeeded: a[3])))")
            }
            throw WasmTrap("trap: abort (el programa llamó a abort)")
        default:
            let nombre = fi < nombresImport.count ? nombresImport[fi] : "?"
            let nota = fi < notaImport.count ? notaImport[fi] : ""
            throw WasmTrap("trap: función importada no disponible: \(nombre)" + (nota.isEmpty ? "" : " (\(nota))"))
        }
    }

    // ---------- WASI: E/S ----------

    private func iovecs(_ iovs: Int, _ n: Int) throws -> [(Int, Int)] {
        var r: [(Int, Int)] = []
        for i in 0 ..< n {
            r.append((try lee32(iovs + 8 * i), try lee32(iovs + 8 * i + 4)))
        }
        return r
    }

    private func fdEscribe(_ a: UnsafeMutablePointer<UInt64>) throws -> UInt32 {
        let fd = Int(UInt32(truncatingIfNeeded: a[0]))
        var todo: [UInt8] = []
        for (base, largo) in try iovecs(ptr(a[1]), ptr(a[2])) {
            todo.append(contentsOf: try leeBytes(base, largo))
        }
        if fd == 1 || fd == 2 {
            emite(fd, todo)
        } else {
            guard let f = fds[fd], f.tipo == 4, f.escribe else { return 8 }
            if f.agrega { f.pos = f.datos.count }
            let fin = f.pos + todo.count
            if fin > f.datos.count { f.datos.append(contentsOf: [UInt8](repeating: 0, count: fin - f.datos.count)) }
            f.datos.replaceSubrange(f.pos ..< fin, with: todo)
            f.pos = fin
            f.sucio = true
        }
        try pon32(ptr(a[3]), UInt32(todo.count))
        return 0
    }

    private func fdLee(_ a: UnsafeMutablePointer<UInt64>) throws -> UInt32 {
        let fd = Int(UInt32(truncatingIfNeeded: a[0]))
        let vecs = try iovecs(ptr(a[1]), ptr(a[2]))
        var fuente: [UInt8] = []
        var desde = 0
        if fd == 0 {
            if bufEntrada.isEmpty && !finEntrada {
                if let l = entrada() { bufEntrada = Array(l.utf8) } else { finEntrada = true }
            }
            fuente = bufEntrada
        } else {
            guard let f = fds[fd], f.tipo == 4 else { return 8 }
            fuente = f.datos
            desde = f.pos
        }
        var total = 0
        for (base, largo) in vecs {
            let k = min(largo, fuente.count - desde - total)
            if k <= 0 { break }
            try ponBytes(base, fuente[(desde + total) ..< (desde + total + k)])
            total += k
        }
        if fd == 0 {
            bufEntrada.removeFirst(total)
        } else {
            fds[fd]?.pos += total
        }
        try pon32(ptr(a[3]), UInt32(total))
        return 0
    }

    private func cierra(_ fd: Int) -> UInt32 {
        guard let f = fds[fd] else { return 8 }
        f.guarda()
        if fd > 2 { fds[fd] = nil }
        return 0
    }

    private func busca(_ a: UnsafeMutablePointer<UInt64>) throws -> UInt32 {
        let fd = Int(UInt32(truncatingIfNeeded: a[0]))
        guard let f = fds[fd], f.tipo == 4 else { return fd <= 2 ? 70 : 8 }
        let off = Int64(bitPattern: a[1])
        var base: Int64 = 0
        switch UInt32(truncatingIfNeeded: a[2]) {
        case 0: base = 0
        case 1: base = Int64(f.pos)
        case 2: base = Int64(f.datos.count)
        default: return 28
        }
        let nueva = base + off
        guard nueva >= 0 else { return 28 }
        f.pos = Int(nueva)
        try pon64(ptr(a[3]), UInt64(nueva))
        return 0
    }

    private func dice(_ a: UnsafeMutablePointer<UInt64>) throws -> UInt32 {
        let fd = Int(UInt32(truncatingIfNeeded: a[0]))
        guard let f = fds[fd], f.tipo == 4 else { return fd <= 2 ? 70 : 8 }
        try pon64(ptr(a[1]), UInt64(f.pos))
        return 0
    }

    private func fdstat(_ a: UnsafeMutablePointer<UInt64>) throws -> UInt32 {
        let fd = Int(UInt32(truncatingIfNeeded: a[0]))
        guard let f = fds[fd] else { return 8 }
        let p = try rango(ptr(a[1]), 24)
        try pon8(p, UInt8(f.tipo))
        try pon8(p + 1, 0)
        try pon16(p + 2, f.agrega ? 1 : 0)
        try pon32(p + 4, 0)
        try pon64(p + 8, UInt64.max)
        try pon64(p + 16, UInt64.max)
        return 0
    }

    private func prestat(_ a: UnsafeMutablePointer<UInt64>) throws -> UInt32 {
        let fd = Int(UInt32(truncatingIfNeeded: a[0]))
        guard fd == 3 || fd == 4, entorno != nil, fds[fd] != nil else { return 8 }
        let nombre = fd == 3 ? "/" : "."
        let p = try rango(ptr(a[1]), 8)
        try pon32(p, 0)
        try pon32(p + 4, UInt32(nombre.utf8.count))
        return 0
    }

    private func prestatNombre(_ a: UnsafeMutablePointer<UInt64>) throws -> UInt32 {
        let fd = Int(UInt32(truncatingIfNeeded: a[0]))
        guard fd == 3 || fd == 4, entorno != nil, fds[fd] != nil else { return 8 }
        let nombre: [UInt8] = Array((fd == 3 ? "/" : ".").utf8)
        let n = min(ptr(a[2]), nombre.count)
        try ponBytes(ptr(a[1]), nombre[0 ..< n])
        return 0
    }

    // ---------- WASI: rutas ----------

    private func resuelve(_ dir: WasmFd, _ ruta: String, _ e: ShellEnv) -> URL? {
        let base = dir.vpath.hasSuffix("/") ? dir.vpath : dir.vpath + "/"
        let v = ruta.hasPrefix("/") ? ruta : base + ruta
        return try? e.resolve(v)
    }

    private func registra(_ f: WasmFd, _ salidaPtr: UInt64) throws -> UInt32 {
        let n = siguienteFd
        siguienteFd += 1
        fds[n] = f
        try pon32(ptr(salidaPtr), UInt32(n))
        return 0
    }

    private func abre(_ a: UnsafeMutablePointer<UInt64>) throws -> UInt32 {
        guard let e = entorno else { return 76 }
        guard let dir = fds[Int(UInt32(truncatingIfNeeded: a[0]))], dir.tipo == 3 else { return 8 }
        let ruta = try leeTexto(ptr(a[2]), ptr(a[3]))
        guard let u = resuelve(dir, ruta, e) else { return 76 }
        let of = UInt32(truncatingIfNeeded: a[4])
        let ff = UInt32(truncatingIfNeeded: a[7])
        let existe = e.exists(u)
        let esDir = e.isDir(u)
        if of & 4 != 0 && of & 1 != 0 && existe { return 20 }
        if of & 2 != 0 && !esDir { return existe ? 54 : 44 }
        if esDir {
            return try registra(WasmFd(tipo: 3, vpath: e.vpath(u), url: u), a[8])
        }
        if !existe && of & 1 == 0 { return 44 }
        let f = WasmFd(tipo: 4, vpath: e.vpath(u), url: u)
        f.escribe = a[5] & (1 << 6) != 0 || of & 9 != 0 || ff & 1 != 0
        f.agrega = ff & 1 != 0
        if !existe || of & 8 != 0 {
            guard e.isDir(u.deletingLastPathComponent()) else { return 44 }
            do { try Data().write(to: u) } catch { return 2 }
        } else {
            f.datos = [UInt8](FileManager.default.contents(atPath: u.path) ?? Data())
        }
        return try registra(f, a[8])
    }

    private func opRuta(_ h: Int, _ a: UnsafeMutablePointer<UInt64>) throws -> UInt32 {
        guard let e = entorno else { return 76 }
        guard let dir = fds[Int(UInt32(truncatingIfNeeded: a[0]))], dir.tipo == 3 else { return 8 }
        let ruta = try leeTexto(ptr(a[1]), ptr(a[2]))
        guard let u = resuelve(dir, ruta, e) else { return 76 }
        let fm = FileManager.default
        switch h {
        case 21:
            if e.exists(u) { return 20 }
            do { try fm.createDirectory(at: u, withIntermediateDirectories: false) } catch { return 44 }
        case 22:
            guard e.exists(u) else { return 44 }
            if e.isDir(u) { return 31 }
            do { try fm.removeItem(at: u) } catch { return 2 }
        default:
            guard e.exists(u) else { return 44 }
            guard e.isDir(u) else { return 54 }
            if let c = try? fm.contentsOfDirectory(atPath: u.path), !c.isEmpty { return 55 }
            do { try fm.removeItem(at: u) } catch { return 2 }
        }
        return 0
    }

    private func escribeStat(_ d: Int, _ tipo: UInt8, _ tam: UInt64, _ fecha: UInt64) throws {
        let p = try rango(d, 64)
        try ponBytes(p, [UInt8](repeating: 0, count: 64)[...])
        try pon8(p + 16, tipo)
        try pon64(p + 24, 1)
        try pon64(p + 32, tam)
        try pon64(p + 40, fecha)
        try pon64(p + 48, fecha)
        try pon64(p + 56, fecha)
    }

    private func filestatFd(_ a: UnsafeMutablePointer<UInt64>) throws -> UInt32 {
        let fd = Int(UInt32(truncatingIfNeeded: a[0]))
        guard let f = fds[fd] else { return 8 }
        try escribeStat(ptr(a[1]), UInt8(f.tipo), UInt64(f.datos.count), 0)
        return 0
    }

    private func filestatRuta(_ a: UnsafeMutablePointer<UInt64>) throws -> UInt32 {
        guard let e = entorno else { return 76 }
        guard let dir = fds[Int(UInt32(truncatingIfNeeded: a[0]))], dir.tipo == 3 else { return 8 }
        let ruta = try leeTexto(ptr(a[2]), ptr(a[3]))
        guard let u = resuelve(dir, ruta, e) else { return 76 }
        guard e.exists(u) else { return 44 }
        let at = (try? FileManager.default.attributesOfItem(atPath: u.path)) ?? [:]
        let tam = (at[.size] as? NSNumber)?.uint64Value ?? 0
        let fecha = (at[.modificationDate] as? Date)?.timeIntervalSince1970 ?? 0
        try escribeStat(ptr(a[4]), e.isDir(u) ? 3 : 4, tam, UInt64(max(0, fecha) * 1e9))
        return 0
    }

    // ---------- WASI: varios ----------

    private func tamanos(_ lista: [String], _ a: UnsafeMutablePointer<UInt64>) throws -> UInt32 {
        var total = 0
        for s in lista { total += s.utf8.count + 1 }
        try pon32(ptr(a[0]), UInt32(lista.count))
        try pon32(ptr(a[1]), UInt32(total))
        return 0
    }

    private func copiaLista(_ lista: [String], _ a: UnsafeMutablePointer<UInt64>) throws -> UInt32 {
        let tabla = ptr(a[0])
        var p = ptr(a[1])
        for (i, s) in lista.enumerated() {
            try pon32(tabla + 4 * i, UInt32(p))
            let b: [UInt8] = Array(s.utf8) + [0]
            try ponBytes(p, b[...])
            p += b.count
        }
        return 0
    }

    func ahoraNs(_ reloj: UInt32) -> UInt64 {
        if reloj == 0 { return UInt64(max(0, Date().timeIntervalSince1970) * 1e9) }
        return UInt64(ProcessInfo.processInfo.systemUptime * 1e9)
    }

    private func reloj(_ a: UnsafeMutablePointer<UInt64>) throws -> UInt32 {
        let id = UInt32(truncatingIfNeeded: a[0])
        guard id <= 3 else { return 28 }
        try pon64(ptr(a[2]), ahoraNs(id))
        return 0
    }

    private func aleatorio(_ a: UnsafeMutablePointer<UInt64>) throws -> UInt32 {
        let n = ptr(a[1])
        let p = try rango(ptr(a[0]), n)
        var g = SystemRandomNumberGenerator()
        for i in 0 ..< n {
            mem.storeBytes(of: UInt8.random(in: 0 ... 255, using: &g), toByteOffset: p + i, as: UInt8.self)
        }
        return 0
    }

    /// poll_oneoff mínimo: los relojes duermen, los descriptores están siempre listos.
    private func sondea(_ a: UnsafeMutablePointer<UInt64>) throws -> UInt32 {
        let n = ptr(a[2])
        var eventos: [(UInt64, UInt8)] = []
        var relojes: [(UInt64, UInt64)] = []
        for i in 0 ..< n {
            let s = try rango(ptr(a[0]) + 48 * i, 48)
            let ud = mem.loadUnaligned(fromByteOffset: s, as: UInt64.self)
            let tag = mem.load(fromByteOffset: s + 8, as: UInt8.self)
            if tag == 0 {
                var espera = mem.loadUnaligned(fromByteOffset: s + 24, as: UInt64.self)
                let banderas = mem.loadUnaligned(fromByteOffset: s + 40, as: UInt16.self)
                if banderas & 1 != 0 {
                    let ahora = ahoraNs(mem.loadUnaligned(fromByteOffset: s + 16, as: UInt32.self))
                    espera = espera > ahora ? espera - ahora : 0
                }
                relojes.append((ud, espera))
            } else {
                eventos.append((ud, tag))
            }
        }
        if eventos.isEmpty, let minimo = relojes.map({ $0.1 }).min() {
            var falta = Double(minimo) / 1e9
            while falta > 0 {
                try revisaCancelado()
                let trozo = min(falta, 0.05)
                Thread.sleep(forTimeInterval: trozo)
                falta -= trozo
            }
            for r in relojes where r.1 == minimo { eventos.append((r.0, 0)) }
        }
        let salidaP = ptr(a[1])
        for (k, ev) in eventos.enumerated() {
            let p = try rango(salidaP + 32 * k, 32)
            try ponBytes(p, [UInt8](repeating: 0, count: 32)[...])
            try pon64(p, ev.0)
            try pon8(p + 10, ev.1)
        }
        try pon32(ptr(a[3]), UInt32(eventos.count))
        return 0
    }
}

// ============================================================
// MARK: - Comandos: wasm, wat2wasm (y alias wasmer/wasmtime/wasm3)
// ============================================================

/// Junta stdout y stderr en orden (stderr marcado como error).
final class WasmTexto: @unchecked Sendable {
    private let l = NSLock()
    private var partes: [String] = []
    private var inicioLinea = true
    private var bytes = 0
    private var cancelada = false
    private(set) var porTiempo = false
    private(set) var demasiado = false
    static let limite = 32 << 20
    /// Salida en vivo a la terminal (si la hay).
    var vivo: Consola?

    func escribe(_ s: String, error: Bool) {
        if let c = vivo, !error {
            let seguir: Bool = l.conCandado {
                bytes += s.utf8.count
                if bytes > WasmTexto.limite { demasiado = true; return false }
                return true
            }
            if seguir { c.escribir(s) }
            return
        }
        l.conCandado {
            if bytes > WasmTexto.limite {
                demasiado = true
                return
            }
            bytes += s.utf8.count
            if error {
                var r = ""
                for ch in s.unicodeScalars {
                    if inicioLinea { r += Shell.errMark }
                    r.unicodeScalars.append(ch)
                    inicioLinea = ch == "\n"
                }
                partes.append(r)
            } else {
                partes.append(s)
                if let u = s.unicodeScalars.last { inicioLinea = u == "\n" }
            }
        }
    }

    var texto: String { l.conCandado { partes.joined() } }

    func cancela() { l.conCandado { cancelada = true } }

    /// Lo consulta la máquina cada tanto.
    func debeParar(_ fin: Date?) -> Bool {
        l.conCandado {
            if let f = fin, Date() > f { porTiempo = true }
            if vivo?.cancelado == true { cancelada = true }
            return cancelada || porTiempo || demasiado
        }
    }
}

/// stdin del programa: las líneas que llegaron por la tubería.
final class WasmEntrada: @unchecked Sendable {
    private let l = NSLock()
    private var lineas: [String] = []
    private var i = 0
    /// Consola de la terminal: si no llegó nada por tubería, se pide al teclado.
    let consola: Consola?

    init(_ texto: String, consola: Consola? = nil) {
        var partes = texto.components(separatedBy: "\n")
        let ultima = partes.removeLast()
        lineas = partes.map { $0 + "\n" }
        if !ultima.isEmpty { lineas.append(ultima) }
        self.consola = consola
    }

    func siguiente() -> String? {
        let l1: String? = l.conCandado {
            guard i < lineas.count else { return nil }
            i += 1
            return lineas[i - 1]
        }
        if let x = l1 { return x }
        return consola?.leeLinea().map { $0 + "\n" }
    }
}

struct WasmConfig: Sendable {
    let args: [String]
    let vars: [String]
    let invocar: String?
    let invArgs: [String]
    let env: ShellEnv
    let entrada: WasmEntrada
    let fin: Date?
}

struct WasmFin: Sendable {
    let error: String?
    let codigo: Int32
}

extension Shell {
    static func wasm() -> [String: Spec] {
        var c: [String: Spec] = [:]
        c["wasm"] = Spec(help: "wasm archivo.wasm|.wat [args] — corre WebAssembly (WASI); wasm ayuda") { ctx in
            try await Shell.wasmComando(ctx)
        }
        for alias in ["wasmer", "wasmtime", "wasm3"] { c[alias] = c["wasm"] }
        c["wat2wasm"] = Spec(help: "wat2wasm archivo.wat [-o salida.wasm] — texto WebAssembly → binario") { ctx in
            try Shell.wat2wasmComando(ctx)
        }
        return c
    }

    static let wasmAyuda = """
    wasm — WebAssembly en Swift puro (MVP + WASI preview1)
      wasm programa.wasm [args…]        corre _start (o main)
      wasm programa.wat [args…]         ensambla el texto y lo corre
      wasm --invoke f x.wasm [args…]    llama a la función exportada f e imprime lo que devuelve
      wasm info x.wasm                  importaciones, exportaciones, memoria…
      wasm ejemplo                      crea hola.wat y fib.wat para probar
      wat2wasm x.wat [-o x.wasm]        texto → binario
    Opciones: --tiempo N (segundos como mucho; 60 por defecto, 0 = sin límite), --env CLAVE=valor
    stdin llega por tubería: echo hola | wasm eco.wasm
    Archivos (WASI): "/" es la raíz de la shell. Para programas sin libc
    hay un módulo "env": print_i32, print_i64, print_f32, print_f64,
    putchar, print(ptr, largo), puts(ptr).
    No hay: hilos, SIMD, excepciones, red, componentes.
    Alias: wasmer, wasmtime, wasm3.

    """

    /// Cuerpo de 'wasm' (en una función: compila mucho más rápido que un closure).
    static func wasmComando(_ ctx: Ctx) async throws -> String {
        var a = ctx.args
        if a.first == "run" { a.removeFirst() }
        guard let primero = a.first else { return wasmAyuda }
        switch primero {
        case "ayuda", "help", "--help", "-h": return wasmAyuda
        case "ejemplo", "ejemplos": return try wasmEjemplo(ctx)
        case "--version", "-V", "version": return "wasm (SwiftShell) 1.0 — WebAssembly MVP + WASI preview1\n"
        case "info":
            guard a.count >= 2 else { throw ShErr("uso: wasm info archivo.wasm") }
            return try wasmInfo(ctx, a[1])
        default: break
        }
        var invocar: String? = nil
        // sin límite un bucle infinito dejaría la terminal colgada; con la consola hay ^C
        var tiempo: Double? = ctx.sh.rolIA != nil ? 20 : (ctx.consola != nil ? nil : 60)
        var extra: [String] = []
        var i = 0
        bucle: while i < a.count, a[i].hasPrefix("-") {
            let o = a[i]
            let valor: String? = i + 1 < a.count ? a[i + 1] : nil
            switch o {
            case "--invoke", "-i":
                guard let v = valor else { throw ShErr("wasm: --invoke necesita el nombre de la función") }
                invocar = v
            case "--tiempo", "--timeout":
                guard let v = valor, let n = Double(v), n >= 0 else { throw ShErr("wasm: --tiempo necesita segundos") }
                tiempo = n > 0 ? n : nil
            case "--env":
                guard let v = valor, v.contains("=") else { throw ShErr("wasm: --env necesita CLAVE=valor") }
                extra.append(v)
            case "--dir", "--mapdir":
                break
            case "--":
                i += 1
                break bucle
            default:
                throw ShErr("wasm: opción desconocida \(o) (mira: wasm ayuda)")
            }
            i += 2
        }
        guard i < a.count else { throw ShErr("wasm: falta el archivo .wasm o .wat") }
        let archivo = a[i]
        var resto = Array(a[(i + 1)...])
        if invocar == nil, resto.first == "--invoke", resto.count >= 2 {
            invocar = resto[1]
            resto = Array(resto.dropFirst(2))
        }
        let bytes = try wasmCarga(ctx, archivo)
        let env = ctx.env
        let nombre = (archivo as NSString).lastPathComponent
        var vars: [String] = ["HOME=/", "PWD=" + env.vpath(env.cwd), "USER=" + (env.vars["USER"] ?? "mobile"), "TERM=swiftshell"]
        vars.append(contentsOf: extra)
        let conf = WasmConfig(args: [nombre] + resto, vars: vars, invocar: invocar, invArgs: resto, env: env,
                              entrada: WasmEntrada(ctx.stdin, consola: ctx.tecladoVivo ? ctx.consola : nil),
                              fin: tiempo.map { Date().addingTimeInterval($0) })
        let caja = WasmTexto()
        caja.vivo = ctx.consola
        ctx.consola?.empieza()
        defer { ctx.consola?.termina() }
        let r = await withTaskCancellationHandler {
            await Shell.enHiloWasm { Shell.wasmCorre(bytes, conf, caja) }
        } onCancel: {
            caja.cancela()
        }
        return wasmCierra(caja, r, tiempo)
    }

    /// Salida final: lo que imprimió el programa y, si falló, la razón.
    static func wasmCierra(_ caja: WasmTexto, _ r: WasmFin, _ tiempo: Double?) -> String {
        var out = caja.texto
        var nota: String? = nil
        if caja.demasiado {
            nota = "wasm: el programa escribió demasiado (más de 32 MB), lo paré"
        } else if var e = r.error {
            if e == "interrumpido", caja.porTiempo, let s = tiempo {
                e = "interrumpido: pasó el límite de \(Int(s)) s (usa --tiempo N para cambiarlo)"
            }
            nota = e
        } else if r.codigo != 0 {
            nota = "(salió con código \(r.codigo))"
        }
        if let n = nota {
            if !out.isEmpty && !out.hasSuffix("\n") { out += "\n" }
            out += Shell.errMark + n + "\n"
        }
        return out
    }

    /// Hilo con pila grande, como Python (ver enHiloPython).
    static func enHiloWasm(_ trabajo: @escaping @Sendable () -> WasmFin) async -> WasmFin {
        await withCheckedContinuation { (c: CheckedContinuation<WasmFin, Never>) in
            let t = Thread {
                c.resume(returning: trabajo())
            }
            t.stackSize = 64 << 20
            t.start()
        }
    }

    /// Lee un .wasm o ensambla un .wat.
    static func wasmCarga(_ ctx: Ctx, _ p: String) throws -> [UInt8] {
        let u = try ctx.env.resolve(p)
        guard ctx.env.exists(u), !ctx.env.isDir(u) else { throw ShErr("\(ctx.name): \(p): no existe") }
        guard let d = FileManager.default.contents(atPath: u.path) else { throw ShErr("\(ctx.name): \(p): no se puede leer") }
        let b = [UInt8](d)
        let binario = b.count >= 4 && b[0] == 0 && b[1] == 0x61 && b[2] == 0x73 && b[3] == 0x6D
        if binario { return b }
        let low = p.lowercased()
        let pareceTexto = low.hasSuffix(".wat") || low.hasSuffix(".wast") || b.first == 0x28 || b.first == 0x3B
        guard pareceTexto else { throw ShErr("\(ctx.name): \(p): no es WebAssembly (ni binario ni texto .wat)") }
        do {
            return try WatEnsamblador.ensambla(String(decoding: b, as: UTF8.self))
        } catch let e as WatError {
            throw ShErr("\(p): \(e.m)")
        }
    }

    /// Lo que corre en el hilo: decodifica, instancia y llama a la entrada.
    static func wasmCorre(_ bytes: [UInt8], _ conf: WasmConfig, _ caja: WasmTexto) -> WasmFin {
        do {
            let maq = try WasmMaquina(WasmModulo(bytes))
            maq.salida = { caja.escribe($0, error: false) }
            maq.errores = { caja.escribe($0, error: true) }
            let entrada = conf.entrada
            maq.entrada = { entrada.siguiente() }
            let fin = conf.fin
            maq.cancelado = { caja.debeParar(fin) }
            maq.argumentos = conf.args
            maq.variables = conf.vars
            maq.entorno = conf.env
            defer { maq.termina() }
            do {
                try maq.instancia()
                if let f = conf.invocar {
                    caja.escribe(try wasmInvoca(maq, f, conf.invArgs), error: false)
                    return WasmFin(error: nil, codigo: 0)
                }
                return try wasmEntrada(maq, caja)
            } catch let s as WasmSalida {
                return WasmFin(error: nil, codigo: s.codigo)
            }
        } catch let t as WasmTrap {
            return WasmFin(error: t.m, codigo: 1)
        } catch {
            return WasmFin(error: "wasm: \(error)", codigo: 1)
        }
    }

    /// _start, o main, o explica qué exporta el módulo.
    static func wasmEntrada(_ maq: WasmMaquina, _ caja: WasmTexto) throws -> WasmFin {
        let m = maq.modulo
        if let f = m.exportFuncion("_start") {
            _ = try maq.invoca(f, [])
            return WasmFin(error: nil, codigo: 0)
        }
        if let f = m.exportFuncion("main") ?? m.exportFuncion("__main_argc_argv") {
            let t = m.tipoFuncion(f)
            let args = [UInt64](repeating: 0, count: t.params.count)
            let r = try maq.invoca(f, args)
            if t.results.first == WasmValor.i32, let v = r.first {
                return WasmFin(error: nil, codigo: Int32(truncatingIfNeeded: v))
            }
            return WasmFin(error: nil, codigo: 0)
        }
        if m.inicio != nil { return WasmFin(error: nil, codigo: 0) }
        var s = "el módulo no exporta _start ni main.\n"
        let fs = m.exportaciones.filter { $0.clase == 0 }
        if fs.isEmpty {
            s += "No exporta ninguna función.\n"
        } else {
            s += "Funciones exportadas:\n"
            for e in fs { s += "  " + e.nombre + " " + m.tipoFuncion(e.indice).texto + "\n" }
            s += "Prueba: wasm --invoke \(fs[0].nombre) archivo.wasm [args…]\n"
        }
        caja.escribe(s, error: false)
        return WasmFin(error: nil, codigo: 0)
    }

    static func wasmInvoca(_ maq: WasmMaquina, _ nombre: String, _ args: [String]) throws -> String {
        let m = maq.modulo
        guard let f = m.exportFuncion(nombre) else {
            let hay = m.exportaciones.filter { $0.clase == 0 }.map { $0.nombre }.joined(separator: ", ")
            throw WasmTrap("no hay ninguna función exportada '\(nombre)'" + (hay.isEmpty ? "" : " (hay: \(hay))"))
        }
        let t = m.tipoFuncion(f)
        guard args.count == t.params.count else {
            throw WasmTrap("\(nombre) \(t.texto) espera \(t.params.count) argumento(s) y le diste \(args.count)")
        }
        var vals: [UInt64] = []
        for (tipo, s) in zip(t.params, args) { vals.append(try wasmValor(s, tipo)) }
        let r = try maq.invoca(f, vals)
        var out = ""
        for (tipo, v) in zip(t.results, r) { out += wasmTexto(v, tipo) + "\n" }
        return out
    }

    /// Texto → valor según el tipo del parámetro.
    static func wasmValor(_ s: String, _ tipo: UInt8) throws -> UInt64 {
        let malo = WasmTrap("'\(s)' no es un \(WasmValor.nombre(tipo)) válido")
        let hex: UInt64? = s.lowercased().hasPrefix("0x") ? UInt64(s.dropFirst(2), radix: 16) : nil
        switch tipo {
        case WasmValor.i32:
            if let h = hex { return h & 0xFFFF_FFFF }
            if let v = Int32(s) { return UInt64(UInt32(bitPattern: v)) }
            if let v = UInt32(s) { return UInt64(v) }
        case WasmValor.i64:
            if let h = hex { return h }
            if let v = Int64(s) { return UInt64(bitPattern: v) }
            if let v = UInt64(s) { return v }
        case WasmValor.f32:
            if let v = Float(s) { return UInt64(v.bitPattern) }
        case WasmValor.f64:
            if let v = Double(s) { return v.bitPattern }
        default:
            if let v = UInt64(s) { return v }
        }
        throw malo
    }

    static func wasmTexto(_ v: UInt64, _ tipo: UInt8) -> String {
        switch tipo {
        case WasmValor.i32: return String(Int32(truncatingIfNeeded: v))
        case WasmValor.i64: return String(Int64(bitPattern: v))
        case WasmValor.f32: return "\(Float(bitPattern: UInt32(truncatingIfNeeded: v)))"
        case WasmValor.f64: return "\(Double(bitPattern: v))"
        default: return v == 0 ? "null" : "ref \(v - 1)"
        }
    }

    // ---------- wasm info ----------

    static func wasmInfo(_ ctx: Ctx, _ p: String) throws -> String {
        let b = try wasmCarga(ctx, p)
        let m: WasmModulo
        do { m = try WasmModulo(b) } catch let t as WasmTrap { throw ShErr("wasm info: \(t.m)") }
        var s = "\(p): \(b.count) bytes\n"
        s += "funciones: \(m.funciones.count) propias + \(m.nImpFunc) importadas · tipos: \(m.tipos.count)\n"
        s += "memoria: " + wasmTextoMemoria(m) + "\n"
        if !m.tablas.isEmpty || m.nImpTabla > 0 {
            let tams = m.tablas.map { String($0.lim.min) }.joined(separator: ", ")
            s += "tablas: \(m.tablas.count + m.nImpTabla)" + (tams.isEmpty ? "" : " (tamaño \(tams))") + "\n"
        }
        s += "globales: \(m.globales.count + m.nImpGlobal) · segmentos: \(m.datos.count) de datos, \(m.elementos.count) de elementos\n"
        if let i = m.inicio { s += "función de inicio: \(i)\n" }
        if !m.importaciones.isEmpty {
            s += "importaciones:\n"
            for imp in m.importaciones { s += "  " + wasmTextoImport(m, imp) + "\n" }
        }
        if !m.exportaciones.isEmpty {
            s += "exportaciones:\n"
            for e in m.exportaciones { s += "  " + wasmTextoExport(m, e) + "\n" }
        }
        if !m.personalizadas.isEmpty {
            s += "secciones personalizadas: " + m.personalizadas.joined(separator: ", ") + "\n"
        }
        return s
    }

    static func wasmTextoMemoria(_ m: WasmModulo) -> String {
        let imp = m.importaciones.first { $0.clase == 2 }
        guard let lim = imp?.lim ?? m.memorias.first else { return "ninguna" }
        var s = "\(lim.min) página(s) de 64 KiB"
        if let mx = lim.max { s += ", máximo \(mx)" }
        if let i = imp { s += " (importada de \(i.nombre))" }
        return s
    }

    static func wasmTextoImport(_ m: WasmModulo, _ imp: WasmImport) -> String {
        switch imp.clase {
        case 0:
            let t = imp.tipo < m.tipos.count ? m.tipos[imp.tipo] : WasmFuncTipo(params: [], results: [])
            let (h, _) = WasmMaquina.buscaHost(imp, t)
            let marca = h == 9999 ? "  [no disponible: atrapa si se llama]" : ""
            return imp.nombre + " función " + t.texto + marca
        case 1: return imp.nombre + " tabla"
        case 2: return imp.nombre + " memoria (\(imp.lim.min) páginas)"
        default: return imp.nombre + " global " + WasmValor.nombre(imp.valTipo) + (imp.mutable ? " mutable (vale 0)" : " (vale 0)")
        }
    }

    static func wasmTextoExport(_ m: WasmModulo, _ e: WasmExport) -> String {
        switch e.clase {
        case 0:
            let t = e.indice < m.tipoDeFuncion.count ? m.tipoFuncion(e.indice).texto : "?"
            return e.nombre + " función " + t
        case 1: return e.nombre + " tabla"
        case 2: return e.nombre + " memoria"
        default: return e.nombre + " global"
        }
    }

    // ---------- wasm ejemplo ----------

    static let wasmHola = """
    ;; hola.wat — imprime con WASI (fd_write a stdout)
    ;; córrelo con:  wasm hola.wat
    (module
      (import "wasi_snapshot_preview1" "fd_write"
        (func $fd_write (param i32 i32 i32 i32) (result i32)))
      (memory (export "memory") 1)
      ;; el texto vive en la dirección 16
      (data (i32.const 16) "Hola desde WebAssembly\\n")
      (func $main (export "_start")
        ;; iovec en la dirección 0: puntero y largo del texto
        (i32.store (i32.const 0) (i32.const 16))
        (i32.store (i32.const 4) (i32.const 23))
        ;; fd 1 = stdout, 1 iovec, bytes escritos en la dirección 8
        (drop (call $fd_write (i32.const 1) (i32.const 0) (i32.const 1) (i32.const 8)))))

    """

    static let wasmFib = """
    ;; fib.wat — Fibonacci recursivo y con bucle (i64)
    ;; córrelo con:  wasm --invoke fib fib.wat 20
    ;;          o:  wasm --invoke fib_rapido fib.wat 90
    (module
      (func $fib (export "fib") (param $n i32) (result i32)
        (if (result i32) (i32.lt_s (local.get $n) (i32.const 2))
          (then (local.get $n))
          (else
            (i32.add
              (call $fib (i32.sub (local.get $n) (i32.const 1)))
              (call $fib (i32.sub (local.get $n) (i32.const 2)))))))

      (func (export "fib_rapido") (param $n i32) (result i64)
        (local $a i64) (local $b i64) (local $t i64)
        (local.set $b (i64.const 1))
        (block $fin
          (loop $otra
            (br_if $fin (i32.eqz (local.get $n)))
            (local.set $t (i64.add (local.get $a) (local.get $b)))
            (local.set $a (local.get $b))
            (local.set $b (local.get $t))
            (local.set $n (i32.sub (local.get $n) (i32.const 1)))
            (br $otra)))
        (local.get $a)))

    """

    static func wasmEjemplo(_ ctx: Ctx) throws -> String {
        var s = ""
        for (nombre, texto) in [("hola.wat", wasmHola), ("fib.wat", wasmFib)] {
            let u = ctx.env.cwd.appendingPathComponent(nombre)
            if ctx.env.exists(u) {
                s += "\(nombre): ya existía, no lo toco\n"
                continue
            }
            do { try texto.write(to: u, atomically: true, encoding: .utf8) } catch {
                throw ShErr("wasm ejemplo: no pude escribir \(nombre)")
            }
            s += "escrito \(nombre)\n"
        }
        s += """
        Pruébalos:
          wasm hola.wat                       → Hola desde WebAssembly
          wasm --invoke fib fib.wat 20        → 6765
          wasm --invoke fib_rapido fib.wat 90 → 2880067194370816120
          wat2wasm hola.wat                   → hola.wasm (binario)
          wasm info hola.wasm

        """
        return s
    }

    // ---------- wat2wasm ----------

    static func wat2wasmComando(_ ctx: Ctx) throws -> String {
        let o = opts(ctx.args, valued: ["o", "output"])
        guard let entrada = o.rest.first else {
            return "uso: wat2wasm archivo.wat [-o salida.wasm]\n"
        }
        let u = try ctx.env.resolve(entrada)
        guard ctx.env.exists(u), let d = FileManager.default.contents(atPath: u.path) else {
            throw ShErr("wat2wasm: \(entrada): no existe")
        }
        let bin: [UInt8]
        do {
            bin = try WatEnsamblador.ensambla(String(decoding: d, as: UTF8.self))
        } catch let e as WatError {
            throw ShErr("\(entrada): \(e.m)")
        }
        var destino = o.vals["o"] ?? o.vals["output"] ?? ""
        if destino.isEmpty {
            let base = entrada.lowercased().hasSuffix(".wat") ? String(entrada.dropLast(4)) : entrada
            destino = base + ".wasm"
        }
        let du = try ctx.env.resolve(destino)
        do { try Data(bin).write(to: du) } catch { throw ShErr("wat2wasm: no pude escribir \(destino)") }
        return "wat2wasm: escrito \(ctx.env.vpath(du)) (\(bin.count) bytes)\n"
    }
}
