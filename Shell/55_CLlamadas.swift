import Foundation

// ============================================================
// MARK: - C, C++ y Java: llamadas, objetos y declaraciones
// ============================================================

/// Inicializa una constante de un enum de Java (Color.ROJO).
final class CEnumConst: CSent {
    let c: CClase
    let nombre: String
    let ordinal: Int
    let args: [CExpr]
    init(_ c: CClase, _ nombre: String, _ ordinal: Int, _ args: [CExpr], _ linea: Int) {
        self.c = c
        self.nombre = nombre
        self.ordinal = ordinal
        self.args = args
        super.init(linea)
    }
    override func exec(_ m: CMaq, _ f: CMarco) throws -> CSalto {
        var vals: [CV] = []
        for a in args { vals.append(try a.ev(m, f)) }
        let o = try m.creaObjeto(c, linea)
        if let i = c.indice["$nombre"] { o.c.a[i] = m.cadena(nombre) }
        if let i = c.indice["$ordinal"] { o.c.a[i] = .n(Int64(ordinal), .int) }
        try m.construye(o, c, vals, linea)
        if let i = c.estIndice[nombre] { c.estaticos.a[i] = .o(o) }
        c.enumValores.append(o)
        return .nada
    }
    override func res(_ r: CResol) throws {
        for a in args { try a.res(r) }
    }
}

extension CMaq {

    // MARK: funciones

    func ligaParams(_ fn: CFuncion, _ args: [CV], _ f: CMarco, _ linea: Int) throws {
        let np = fn.params.count
        var k = 0
        while k < np {
            let p = fn.params[k]
            var v: CV
            if fn.variadica && dialecto == .java && k == np - 1 {
                // String... args: lo que sobra se junta en un arreglo
                if args.count == np, case .p = args[k] { v = args[k] }
                else {
                    let resto = k < args.count ? Array(args[k...]) : []
                    let mem = CMem(resto)
                    mem.esArreglo = true
                    mem.elem = p.tipo.elemento
                    v = .p(CPtr(mem: mem, i: 0))
                }
                f.s.a[k] = v
                k += 1
                continue
            }
            if k < args.count { v = args[k] }
            else if let d = p.defecto { v = try d.ev(self, f) }
            else { throw CFallo("faltan argumentos al llamar a '\(fn.nombre)' (pide \(fn.minArgs), recibe \(args.count))", linea) }
            if p.tipo.esRef {
                if case .ref = v {} else { v = copia(convierte(v, p.tipo)) }
            } else {
                if case .ref(let r) = v { v = try lee(r.l) }
                if case .auto = p.tipo {} else { v = convierte(v, p.tipo) }
                v = try copiaConstruida(v, linea)
            }
            f.s.a[k] = v
            k += 1
        }
    }

    func llama(_ fn: CFuncion, _ args: [CV], este: CObj?, padre: CMarco?, linea: Int) throws -> CV {
        if let n = fn.nat { return try n(self, args) }
        guard let cuerpo = fn.cuerpo else {
            if fn.abstracto { throw CFallo("se llamó al método abstracto '\(fn.nombre)' sin implementar", linea) }
            throw CFallo("'\(fn.nombre)' está declarada pero no tiene cuerpo", linea)
        }
        profundidad += 1
        defer { profundidad -= 1 }
        if profundidad > maxProfundidad {
            if dialecto == .java { throw excepcion("StackOverflowError", "demasiadas llamadas anidadas", linea) }
            throw CFallo("desbordamiento de pila: demasiadas llamadas anidadas (¿recursión sin fin?)", linea)
        }
        let f = CMarco(max(fn.nSlots, fn.params.count), padre: padre, este: este)
        f.devuelveRef = fn.ret.esRef
        try ligaParams(fn, args, f, linea)
        let r = try cuerpo.exec(self, f)
        if dialecto == .cpp { try destruyeParametros(fn, f, linea) }
        if case .vuelve(let v) = r {
            if fn.ret.esRef, case .ref = v { return v }
            var x = v
            if case .ref(let rr) = x { x = try lee(rr.l) }
            switch fn.ret {
            case .vacio, .auto, .clase, .cont, .ref: return x
            default: return convierte(x, fn.ret)
            }
        }
        return .vacio
    }

    func llamaCierre(_ c: CCierre, _ args: [CV], _ linea: Int) throws -> CV {
        if let n = c.nat { return try n(self, args) }
        guard let fn = c.fn else { throw CFallo("función vacía", linea) }
        return try llama(fn, args, este: c.este, padre: c.marco, linea: linea)
    }

    /// Llama a cualquier cosa que se pueda llamar: lambda, puntero a función u objeto con operator().
    func invoca(_ v: CV, _ args: [CV], _ linea: Int) throws -> CV {
        switch v {
        case .f(let c): return try llamaCierre(c, args, linea)
        case .o(let o):
            if let fn = o.clase.metodo("operator()", args.count) { return try llama(fn, args, este: o, padre: o.exterior, linea: linea) }
            // interfaz funcional de Java implementada con una clase
            for (_, lista) in o.clase.metodos {
                if let fn = lista.first(where: { $0.params.count == args.count && !$0.estatico && $0.nombre != "toString" && $0.nombre != "equals" }) {
                    return try llama(fn, args, este: o, padre: o.exterior, linea: linea)
                }
            }
            throw CFallo("\(o.clase.nombre) no se puede llamar como función", linea)
        case .p(let p):
            if let mem = p.mem, p.i < mem.a.count { return try invoca(mem.a[p.i], args, linea) }
            throw CFallo("llamada a un puntero a función NULL", linea)
        case .ref(let r): return try invoca(try lee(r.l), args, linea)
        case .nulo: throw excepcion("NullPointerException", "se llamó a una función null", linea)
        default: throw CFallo("\(v.tipoNombre) no es una función", linea)
        }
    }

    func evaluaArgs(_ exprs: [CExpr], _ params: [CParam]?, _ f: CMarco) throws -> [CV] {
        var r: [CV] = []
        r.reserveCapacity(exprs.count)
        for (k, e) in exprs.enumerated() {
            if let ps = params, k < ps.count, ps[k].tipo.esRef, e.asignable {
                r.append(.ref(CRefCaja(try e.lugar(self, f))))
            } else {
                r.append(try e.ev(self, f))
            }
        }
        return r
    }

    /// Valores y, si se puede, lugares de los argumentos (para elegir sobrecarga).
    func evaluaArgsCompletos(_ exprs: [CExpr], _ f: CMarco) throws -> ([CV], [CLugar?]) {
        var vals: [CV] = []
        var lugares: [CLugar?] = []
        for e in exprs {
            if e.asignable, !(e is CLlamada) {
                let l = try e.lugar(self, f)
                lugares.append(l)
                vals.append(try lee(l))
            } else {
                lugares.append(nil)
                vals.append(try e.ev(self, f))
            }
        }
        return (vals, lugares)
    }

    func elige(_ fs: [CFuncion], _ n: Int, _ vals: [CV]?, _ nombre: String, _ linea: Int) throws -> CFuncion {
        let cand = fs.filter { n >= $0.minArgs && (n <= $0.params.count || $0.variadica) }
        if cand.count == 1 { return cand[0] }
        if cand.isEmpty {
            if let f = fs.first(where: { $0.variadica }) { return f }
            throw CFallo("no hay ninguna versión de '\(nombre)' con \(n) argumentos", linea)
        }
        guard let vs = vals else { return cand[0] }
        var mejor = cand[0]
        var puntos = Int.min
        for f in cand {
            var p = 0
            for (k, v) in vs.enumerated() where k < f.params.count { p += puntaje(f.params[k].tipo, v) }
            if f.params.count == n { p += 1 }
            if p > puntos { puntos = p; mejor = f }
        }
        return mejor
    }

    func puntaje(_ t: CTipo, _ v: CV) -> Int {
        var x = v
        if case .ref(let r) = x { x = (try? lee(r.l)) ?? .vacio }
        switch (t.sinRef, x) {
        case (.num(let k), .n(_, let q)): return k == q ? 10 : ((k == .char || k == .jchar) != (q == .char || q == .jchar) ? 3 : 6)
        case (.real, .d): return 10
        case (.real, .n): return 4
        case (.num, .d): return 1
        case (.cad, .s): return 10
        case (.cad, .p): return 7
        case (.ptr(.num(.char)), .p): return 9
        case (.ptr, .p): return 8
        case (.clase(let n), .o(let o)): return o.clase.nombre == n ? 10 : (o.clase.esSubclase(de: n) ? 7 : -50)
        case (.cont(let n, _), .l(let l)): return CMaq.familia(l.k, n) || n == l.k ? 10 : 2
        case (.cont, .m): return 8
        case (.arr, .p): return 9
        case (.auto, _): return 2
        case (.fn, .f): return 9
        case (.clase, .nulo), (.cad, .nulo), (.arr, .nulo): return 5
        default: return -20
        }
    }

    func valorFuncion(_ n: String, _ linea: Int) throws -> CV {
        if let fs = prog.funciones[n], let fn = fs.first { return .f(CCierre(fn: fn, marco: nil, este: nil)) }
        if esNativoFuncion(n) {
            return .f(CCierre(fn: nil, marco: nil, este: nil, nombre: n, nat: { m, a in
                try m.nativa(n, a, a.map { _ in nil }, 0)
            }))
        }
        throw CFallo("'\(n)' no es una función", linea)
    }

    // MARK: constructores de copia y destructores (C++)

    func constructorDeCopia(_ c: CClase) -> CFuncion? {
        c.ctors.first { fn in
            guard fn.params.count == 1, fn.cuerpo != nil, case .clase(let n) = fn.params[0].tipo.sinRef else { return false }
            return n == c.nombre
        }
    }

    /// Copia un objeto llamando a su constructor de copia si lo escribió el usuario.
    func copiaConstruida(_ v: CV, _ linea: Int) throws -> CV {
        guard dialecto == .cpp, case .o(let o) = v, constructorDeCopia(o.clase) != nil else { return copia(v) }
        return .o(try nuevoObjeto(o.clase, [v], linea))
    }

    func tieneDestructor(_ c: CClase) -> Bool {
        var k: CClase? = c
        while let x = k {
            if x.dtor != nil { return true }
            for n in x.campos where !n.estatico {
                if case .clase(let cn) = n.tipo, let cc = clases[cn], cc !== x, tieneDestructor(cc) { return true }
            }
            k = x.padre ?? x.nombrePadre.flatMap { clases[$0] }
        }
        return false
    }

    /// ~Derivada() y luego ~Base(); después, los campos que son objetos.
    func destruye(_ o: CObj, _ linea: Int) throws {
        var k: CClase? = o.clase
        while let x = k {
            if let d = x.dtor { _ = try llama(d, [], este: o, padre: o.exterior, linea: linea) }
            for campo in x.campos.reversed() where !campo.estatico {
                guard case .clase = campo.tipo, let i = x.indice[campo.nombre], i < o.c.a.count, case .o(let sub) = o.c.a[i] else { continue }
                try destruye(sub, linea)
            }
            k = x.padre
        }
    }

    func destruyeLocales(_ slots: [Int], _ f: CMarco, _ linea: Int) throws {
        for s in slots.reversed() {
            guard s >= 0 && s < f.s.a.count, case .o(let o) = f.s.a[s] else { continue }
            f.s.a[s] = .vacio
            try destruye(o, linea)
        }
    }

    func destruyeParametros(_ fn: CFuncion, _ f: CMarco, _ linea: Int) throws {
        for (k, p) in fn.params.enumerated() {
            guard case .clase(let cn) = p.tipo, let c = clases[cn], tieneDestructor(c), k < f.s.a.count, case .o(let o) = f.s.a[k] else { continue }
            f.s.a[k] = .vacio
            try destruye(o, linea)
        }
    }

    // MARK: el despachador de llamadas

    func llamada(_ e: CLlamada, _ marco: CMarco, quiereLugar: Bool = false) throws -> CV {
        let linea = e.linea
        switch e.destino {
        case .global(let fs):
            if fs.count == 1, let fn = fs.first {
                var args = try evaluaArgs(e.args, fn.params, marco)
                if let t = e.plantilla.first { args = try conPlantilla(fn, args, t, linea) }
                return try llama(fn, args, este: nil, padre: nil, linea: linea)
            }
            let (vals, lugares) = try evaluaArgsCompletos(e.args, marco)
            let fn = try elige(fs, vals.count, vals, fs[0].nombre, linea)
            return try llama(fn, pasaRefs(fn, vals, lugares), este: nil, padre: nil, linea: linea)
        case .propio(let n):
            if let o = marco.este {
                return try llamaMetodo(o, n, e.args, marco, linea)
            }
            throw CFallo("'\(n)' necesita un objeto", linea)
        case .estatico(let c, let n):
            let lista = metodosDe(c, n)
            guard !lista.isEmpty else { throw CFallo("\(c.nombre) no tiene el método '\(n)'", linea) }
            let (vals, lugares) = try evaluaArgsCompletos(e.args, marco)
            let fn = try elige(lista, vals.count, vals, n, linea)
            let args = pasaRefs(fn, vals, lugares)
            if !fn.estatico, let o = marco.este {
                return try llama(fn, args, este: o, padre: o.exterior, linea: linea)
            }
            try preparaEstaticos(c)
            return try llama(fn, args, este: nil, padre: nil, linea: linea)
        case .nativo(let n):
            if n == "$construye" {
                let vals = try e.args.map { try $0.ev(self, marco) }
                return try construyeTemporal(e.plantilla.first ?? .auto, vals, e.args, marco, linea)
            }
            if CMaq.nativasConLugar.contains(n) {
                let (vals, lugares) = try evaluaArgsCompletos(e.args, marco)
                return try nativa(n, vals, lugares, linea)
            }
            var vals: [CV] = []
            vals.reserveCapacity(e.args.count)
            for a in e.args { vals.append(try a.ev(self, marco)) }
            return try nativa(n, vals, [], linea)
        case .ctor(let c):
            let vals = try e.args.map { try $0.ev(self, marco) }
            return .o(try nuevoObjeto(c, vals, linea))
        case .esteCtor, .superCtor:
            guard let o = marco.este, let c = e.claseCtx else { throw CFallo("this()/super() fuera de un constructor", linea) }
            let objetivo: CClase
            if case .esteCtor = e.destino { objetivo = c } else {
                guard let p = c.padre else { return .vacio }
                objetivo = p
            }
            let vals = try e.args.map { try $0.ev(self, marco) }
            try construye(o, objetivo, vals, linea)
            return .vacio
        case .superMetodo:
            throw CFallo("super sin resolver", linea)
        case .dinamico:
            if let mb = e.f as? CMiembro { return try llamaMiembro(e, mb, marco, quiereLugar: quiereLugar) }
            let fv = try e.f.ev(self, marco)
            var params: [CParam]?
            if case .f(let c) = fv { params = c.fn?.params }
            let args = try evaluaArgs(e.args, params, marco)
            return try invoca(fv, args, linea)
        }
    }

    /// mayor<string>("a", "b"): los argumentos de tipo T se convierten a string
    func conPlantilla(_ fn: CFuncion, _ args: [CV], _ t: CTipo, _ linea: Int) throws -> [CV] {
        var r = args
        for k in 0..<min(fn.params.count, r.count) {
            guard case .auto = fn.params[k].tipo.sinRef else { continue }
            if case .cad = t { r[k] = try aCadena(r[k], linea) } else { r[k] = convierte(r[k], t) }
        }
        return r
    }

    func pasaRefs(_ fn: CFuncion, _ vals: [CV], _ lugares: [CLugar?]) -> [CV] {
        var r = vals
        for k in 0..<min(fn.params.count, vals.count) where fn.params[k].tipo.esRef {
            if k < lugares.count, let l = lugares[k] { r[k] = .ref(CRefCaja(l)) }
        }
        return r
    }

    func metodosDe(_ c: CClase, _ n: String) -> [CFuncion] {
        var k: CClase? = c
        var lista: [CFuncion] = []
        while let x = k {
            for f in x.metodos[n] ?? [] where !lista.contains(where: { $0.params.count == f.params.count && !f.abstracto }) {
                if !f.abstracto || f.cuerpo != nil { lista.append(f) }
            }
            k = x.padre
        }
        if lista.isEmpty {
            for i in interfacesDe(c) { lista += (i.metodos[n] ?? []).filter { $0.cuerpo != nil } }
        }
        return lista
    }

    func interfacesDe(_ c: CClase) -> [CClase] {
        var out: [CClase] = []
        var k: CClase? = c
        while let x = k {
            for n in x.interfaces { if let i = clases[n] { out.append(i); out += interfacesDe(i) } }
            k = x.padre
        }
        return out
    }

    /// Método de un objeto (despacho virtual: gana la clase real del objeto).
    func llamaMetodo(_ o: CObj, _ n: String, _ exprs: [CExpr], _ marco: CMarco, _ linea: Int) throws -> CV {
        let lista = metodosDe(o.clase, n)
        if lista.count == 1, let fn = lista.first {
            let args = try evaluaArgs(exprs, fn.params, marco)
            return try llama(fn, args, este: fn.estatico ? nil : o, padre: o.exterior, linea: linea)
        }
        if lista.isEmpty {
            let vals = try exprs.map { try $0.ev(self, marco) }
            return try metodoObjetoNativo(o, n, vals, linea)
        }
        let (vals, lugares) = try evaluaArgsCompletos(exprs, marco)
        let fn = try elige(lista, vals.count, vals, n, linea)
        return try llama(fn, pasaRefs(fn, vals, lugares), este: fn.estatico ? nil : o, padre: o.exterior, linea: linea)
    }

    func llamaMiembro(_ e: CLlamada, _ mb: CMiembro, _ marco: CMarco, quiereLugar: Bool) throws -> CV {
        let linea = e.linea
        if let (c, i) = mb.estatico {
            try preparaEstaticos(c)
            let fv = c.estaticos.a[i]
            return try llamaSobre(fv, nil, mb.nombre, e, marco, quiereLugar: quiereLugar)
        }
        if let n = mb.nativo {
            let fv = try valorNativo(n, linea)
            return try llamaSobre(fv, nil, mb.nombre, e, marco, quiereLugar: quiereLugar)
        }
        var base: CV
        var lugar: CLugar?
        if mb.flecha {
            base = try mb.a.ev(self, marco)
            if case .p(let p) = base {
                guard let mem = p.mem else { throw CFallo("p->\(mb.nombre)() con p = NULL", linea) }
                try prepara(mem, mb.a.tipo?.sinRef.elemento, linea)
                guard p.i >= 0 && p.i < mem.a.count else { throw CFallo("p->\(mb.nombre)() fuera de la memoria reservada", linea) }
                lugar = .celda(mem, p.i)
                base = mem.a[p.i]
            } else if case .it(let b, let i) = base {
                lugar = try lugarIterador(b, i, linea)
                base = try lee(lugar!)
            }
        } else if mb.a.asignable {
            let l = try mb.a.lugar(self, marco)
            lugar = l
            base = try lee(l)
        } else {
            base = try mb.a.ev(self, marco)
        }
        return try llamaSobre(base, lugar, mb.nombre, e, marco, quiereLugar: quiereLugar)
    }

    func llamaSobre(_ base: CV, _ lugar: CLugar?, _ n: String, _ e: CLlamada, _ marco: CMarco, quiereLugar: Bool) throws -> CV {
        let linea = e.linea
        switch base {
        case .o(let o):
            if o.clase.tieneMetodo(n) || !interfacesDe(o.clase).filter({ $0.metodos[n] != nil }).isEmpty {
                return try llamaMetodo(o, n, e.args, marco, linea)
            }
            let vals = try e.args.map { try $0.ev(self, marco) }
            return try metodoObjetoNativo(o, n, vals, linea)
        case .f(let c):
            let vals = try e.args.map { try $0.ev(self, marco) }
            return try metodoCierre(c, n, vals, linea)
        case .nulo:
            throw excepcion("NullPointerException", "Cannot invoke \"\(n)()\" because value is null", linea)
        default:
            if CMaq.metodosConLugar.contains(n) {
                let (vals, lugares) = try evaluaArgsCompletos(e.args, marco)
                return try metodoNativo(base, lugar, n, vals, lugares, linea, quiereLugar: quiereLugar)
            }
            var vals: [CV] = []
            vals.reserveCapacity(e.args.count)
            for a in e.args { vals.append(try a.ev(self, marco)) }
            return try metodoNativo(base, lugar, n, vals, [], linea, quiereLugar: quiereLugar)
        }
    }

    /// Métodos de lambdas: compare, apply, test, run… y los de Comparator.
    func metodoCierre(_ c: CCierre, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        switch n {
        case "reversed":
            return .f(CCierre(fn: nil, marco: nil, este: nil, nombre: "reversed", nat: { m, x in
                let r = try m.llamaCierre(c, x.reversed(), linea)
                return r
            }))
        case "thenComparing", "thenComparingInt", "thenComparingDouble":
            guard let otro = a.first else { return .f(c) }
            return .f(CCierre(fn: nil, marco: nil, este: nil, nombre: n, nat: { m, x in
                let r = try m.entero(try m.llamaCierre(c, x, linea), linea)
                if r != 0 { return .n(r, .int) }
                if case .f(let o2) = otro, (o2.fn?.params.count ?? 2) == 1 || o2.nombre.hasPrefix("clave") {
                    let ka = try m.invoca(otro, [x[0]], linea)
                    let kb = try m.invoca(otro, [x[1]], linea)
                    return .n(Int64(try m.compara(ka, kb, linea)), .int)
                }
                return try m.invoca(otro, x, linea)
            }))
        case "andThen":
            guard let otro = a.first else { return .f(c) }
            return .f(CCierre(fn: nil, marco: nil, este: nil, nombre: n, nat: { m, x in
                let r = try m.llamaCierre(c, x, linea)
                if case .vacio = r { return try m.invoca(otro, x, linea) }
                return try m.invoca(otro, [r], linea)
            }))
        case "compose":
            guard let otro = a.first else { return .f(c) }
            return .f(CCierre(fn: nil, marco: nil, este: nil, nombre: n, nat: { m, x in
                try m.llamaCierre(c, [try m.invoca(otro, x, linea)], linea)
            }))
        case "negate":
            return .f(CCierre(fn: nil, marco: nil, este: nil, nombre: n, nat: { m, x in
                m.verdad(!m.aBool(try m.llamaCierre(c, x, linea)))
            }))
        case "and", "or":
            guard let otro = a.first else { return .f(c) }
            let esY = n == "and"
            return .f(CCierre(fn: nil, marco: nil, este: nil, nombre: n, nat: { m, x in
                let p = m.aBool(try m.llamaCierre(c, x, linea))
                if esY && !p { return m.verdad(false) }
                if !esY && p { return m.verdad(true) }
                return m.verdad(m.aBool(try m.invoca(otro, x, linea)))
            }))
        case "equals": return verdad(false)
        default:
            return try llamaCierre(c, a, linea)
        }
    }

    // MARK: operadores sobrecargados (C++)

    func llamaOperador(_ nombre: String, _ v: CV, _ args: [CV], _ linea: Int) throws -> CV? {
        guard case .o(let o) = v else { return nil }
        if let fn = o.clase.metodo(nombre, args.count) {
            return try llama(fn, args, este: o, padre: o.exterior, linea: linea)
        }
        if let fs = prog.funciones[nombre], let fn = fs.first(where: { $0.params.count == args.count + 1 }) {
            return try llama(fn, [v] + args, este: nil, padre: nil, linea: linea)
        }
        return nil
    }

    func llamaOperadorBinario(_ op: COp, _ x: CV, _ y: CV, _ linea: Int) throws -> CV? {
        guard dialecto == .cpp else { return nil }
        let nombre = "operator" + op.texto
        if case .o(let o) = x, let fn = o.clase.metodo(nombre, 1) {
            return try llama(fn, [y], este: o, padre: o.exterior, linea: linea)
        }
        if let fs = prog.funciones[nombre] {
            let cand = fs.filter { $0.params.count == 2 }
            if !cand.isEmpty {
                let fn = try elige(cand, 2, [x, y], nombre, linea)
                return try llama(fn, [x, y], este: nil, padre: nil, linea: linea)
            }
        }
        // != a partir de ==, > a partir de <…
        switch op {
        case .distinto:
            if let r = try llamaOperadorBinario(.igual, x, y, linea) { return verdad(!aBool(r)) }
        case .mayor:
            if let r = try llamaOperadorBinario(.menor, y, x, linea) { return r }
        case .menorIg:
            if let r = try llamaOperadorBinario(.menor, y, x, linea) { return verdad(!aBool(r)) }
        case .mayorIg:
            if let r = try llamaOperadorBinario(.menor, x, y, linea) { return verdad(!aBool(r)) }
        default:
            break
        }
        return nil
    }

    // MARK: clases

    func preparaClase(_ c: CClase) throws {
        if c.preparada { return }
        c.preparada = true
        if let pn = c.nombrePadre {
            guard let p = clases[pn] else {
                if dialecto == .java && pn == "Object" { c.nombrePadre = nil; return try preparaClaseCampos(c) }
                throw CFallo("la clase '\(c.nombre)' hereda de '\(pn)', que no existe", c.linea)
            }
            try preparaClase(p)
            c.padre = p
        }
        try preparaClaseCampos(c)
    }

    func preparaClaseCampos(_ c: CClase) throws {
        if let p = c.padre {
            c.indice = p.indice
            c.tipos = p.tipos
        }
        for f in c.campos where !f.estatico {
            c.indice[f.nombre] = c.tipos.count
            c.tipos.append(f.tipo)
        }
        if c.esEnum {
            c.indice["$nombre"] = c.tipos.count
            c.tipos.append(.cad)
            c.indice["$ordinal"] = c.tipos.count
            c.tipos.append(.num(.int))
        }
        var sentEst: [CSent] = []
        for (k, ea) in c.enumArgs.enumerated() {
            c.estIndice[ea.0] = c.estTipos.count
            c.estTipos.append(.clase(c.nombre))
            c.estaticos.a.append(.nulo)
            sentEst.append(CEnumConst(c, ea.0, k, ea.1, ea.2))
        }
        for f in c.campos where f.estatico {
            if c.estIndice[f.nombre] == nil {
                c.estIndice[f.nombre] = c.estTipos.count
                c.estTipos.append(f.tipo)
                c.estaticos.a.append(.vacio)
            }
        }
        // inicializadores de campos y bloques { } de Java
        var sent: [CSent] = []
        for f in c.campos where !f.estatico {
            guard let ini = f.ini else { continue }
            if let l = ini as? CListaIni { l.tipo = f.tipo }
            sent.append(CExprSent(CAsig(nil, CMiembro(CEste(f.linea), f.nombre, flecha: false, f.linea), ini, f.linea), f.linea))
        }
        sent += c.iniInstancia
        if !sent.isEmpty {
            let fn = CFuncion("$campos", params: [], ret: .vacio, linea: c.linea)
            fn.clase = c
            fn.cuerpo = CBloque(sent, c.linea, nuevoAmbito: false)
            iniCampos[c.id] = fn
        }
        for f in c.campos where f.estatico {
            guard let ini = f.ini else { continue }
            if let l = ini as? CListaIni { l.tipo = f.tipo }
            let dest = CMiembro(CNombre(c.nombre, f.linea), f.nombre, flecha: false, f.linea)
            sentEst.append(CExprSent(CAsig(nil, dest, ini, f.linea), f.linea))
        }
        sentEst += c.iniEstatico
        if !sentEst.isEmpty {
            let fn = CFuncion("$estaticos", params: [], ret: .vacio, linea: c.linea)
            fn.clase = c
            fn.estatico = true
            fn.cuerpo = CBloque(sentEst, c.linea, nuevoAmbito: false)
            iniEstaticos[c.id] = fn
        }
    }

    func preparaEstaticos(_ c: CClase) throws {
        if c.listo { return }
        c.listo = true
        if let p = c.padre { try preparaEstaticos(p) }
        for (k, t) in c.estTipos.enumerated() {
            if case .vacio = c.estaticos.a[k] { c.estaticos.a[k] = try valorCampo(t) }
        }
        if let fn = iniEstaticos[c.id] {
            _ = try llama(fn, [], este: nil, padre: nil, linea: c.linea)
        }
    }

    func valorCampo(_ t: CTipo) throws -> CV {
        if dialecto == .java {
            switch t {
            case .num(let k): return .n(0, k)
            case .real(let f): return .d(0, f)
            default: return .nulo
            }
        }
        return try valorInicialT(t)
    }

    func creaObjeto(_ c: CClase, _ linea: Int) throws -> CObj {
        try preparaClase(c)
        try preparaEstaticos(c)
        var celdas: [CV] = []
        celdas.reserveCapacity(c.tipos.count)
        for t in c.tipos { celdas.append(try valorCampo(t)) }
        return CObj(c, CMem(celdas))
    }

    func nuevoObjeto(_ c: CClase, _ args: [CV], _ linea: Int) throws -> CObj {
        if dialecto == .java && (c.esInterfaz || c.esAbstracta) {
            throw CFallo("\(c.nombre) es abstracta: no se pueden crear objetos de ella", linea)
        }
        let o = try creaObjeto(c, linea)
        try construye(o, c, args, linea)
        return o
    }

    func eligeCtor(_ c: CClase, _ args: [CV], _ linea: Int) throws -> CFuncion? {
        let lista = c.ctors.filter { $0.cuerpo != nil || $0.nat != nil }
        if lista.isEmpty {
            if !c.ctors.isEmpty { throw CFallo("el constructor de \(c.nombre) está declarado pero no escrito", linea) }
            return nil
        }
        let cand = lista.filter { args.count >= $0.minArgs && (args.count <= $0.params.count || $0.variadica) }
        if cand.isEmpty {
            if args.isEmpty && dialecto == .cpp { return nil }
            if args.count == 1, case .o(let o) = args[0], o.clase === c { return nil }
            throw CFallo("\(c.nombre) no tiene un constructor con \(args.count) argumento\(args.count == 1 ? "" : "s")", linea)
        }
        if args.count == 1, case .o(let o) = args[0], o.clase === c, dialecto != .java,
           !cand.contains(where: { if case .clase(let n) = $0.params[0].tipo.sinRef { return n == c.nombre }; return false }) {
            return nil
        }
        return try elige(cand, args.count, args, c.nombre, linea)
    }

    func construye(_ o: CObj, _ c: CClase, _ args: [CV], _ linea: Int) throws {
        profundidad += 1
        defer { profundidad -= 1 }
        if profundidad > maxProfundidad { throw CFallo("desbordamiento de pila al construir objetos", linea) }
        guard let fn = try eligeCtor(c, args, linea) else {
            // sin constructor: copia (C++), agregado (Punto{1,2}) o vacío
            if args.count == 1, case .o(let otro) = args[0], otro.clase === c, dialecto != .java {
                for k in 0..<min(o.c.a.count, otro.c.a.count) { o.c.a[k] = copiaCampo(otro.c.a[k]) }
                return
            }
            if let p = c.padre { try construye(o, p, [], linea) }
            try ejecutaIniCampos(o, c)
            if !args.isEmpty {
                guard dialecto != .java else { throw CFallo("\(c.nombre) no tiene un constructor con \(args.count) argumentos", linea) }
                let propios = c.campos.filter { !$0.estatico }
                for (k, v) in args.enumerated() where k < propios.count {
                    if let i = c.indice[propios[k].nombre] { o.c.a[i] = copia(convierte(v, propios[k].tipo)) }
                }
            }
            return
        }
        let f = CMarco(max(fn.nSlots, fn.params.count), padre: o.exterior, este: o)
        try ligaParams(fn, args, f, linea)
        guard let cuerpo = fn.cuerpo else { return }
        if dialecto == .java {
            try construyeJava(o, c, fn, cuerpo, f, linea)
            return
        }
        var baseHecha = false
        for (n, exprs, ln) in fn.inits where n == c.padre?.nombre || c.interfaces.contains(n) {
            guard let p = clases[n] else { continue }
            let vals = try exprsCtor(exprs, f)
            try construye(o, p, vals, ln)
            if n == c.padre?.nombre { baseHecha = true }
        }
        if !baseHecha, let p = c.padre { try construye(o, p, [], linea) }
        try ejecutaIniCampos(o, c)
        for (n, exprs, ln) in fn.inits {
            guard let i = c.indice[n] else {
                if n == c.padre?.nombre || c.interfaces.contains(n) || n == c.nombre { continue }
                throw CFallo("'\(n)' no es un campo de \(c.nombre)", ln)
            }
            o.c.a[i] = try valorInicioCampo(c.tipos[i], exprs, f, ln)
        }
        _ = try cuerpo.exec(self, f)
    }

    func exprsCtor(_ exprs: [CExpr], _ f: CMarco) throws -> [CV] {
        if exprs.count == 1, let l = exprs[0] as? CListaIni, l.tipo == nil {
            return try l.elems.map { try $0.ev(self, f) }
        }
        return try exprs.map { try $0.ev(self, f) }
    }

    func valorInicioCampo(_ t: CTipo, _ exprs: [CExpr], _ f: CMarco, _ linea: Int) throws -> CV {
        if t.esRef, let e = exprs.first, e.asignable { return .ref(CRefCaja(try e.lugar(self, f))) }
        if exprs.count == 1, let l = exprs[0] as? CListaIni {
            l.tipo = t.sinRef
            return try construyeLista(l, t.sinRef, f)
        }
        let vals = try exprs.map { try $0.ev(self, f) }
        switch t.sinRef {
        case .clase(let cn):
            if vals.count == 1, case .o(let x) = vals[0], x.clase.esSubclase(de: cn) { return copia(vals[0]) }
            if let c = clases[cn] { return .o(try nuevoObjeto(c, vals, linea)) }
        case .cont, .cad:
            if vals.count != 1 { return try construyeTemporal(t.sinRef, vals, exprs, f, linea) }
            if case .cad = t.sinRef { return try aCadena(vals[0], linea) }
            if case .n = vals[0] { return try construyeTemporal(t.sinRef, vals, exprs, f, linea) }
        default:
            break
        }
        guard let v = vals.first else { return try valorInicialT(t.sinRef) }
        return copia(convierte(v, t.sinRef))
    }

    func construyeJava(_ o: CObj, _ c: CClase, _ fn: CFuncion, _ cuerpo: CSent, _ f: CMarco, _ linea: Int) throws {
        var resto: [CSent] = []
        var primera: CLlamada?
        if let b = cuerpo as? CBloque {
            resto = b.s
            if let es = resto.first as? CExprSent, let ll = es.e as? CLlamada {
                switch ll.destino {
                case .esteCtor, .superCtor:
                    primera = ll
                    resto.removeFirst()
                default: break
                }
            }
        } else {
            resto = [cuerpo]
        }
        if let ll = primera {
            _ = try ll.ev(self, f)
            if case .superCtor = ll.destino { try ejecutaIniCampos(o, c) }
        } else {
            if let p = c.padre { try construye(o, p, [], linea) }
            try ejecutaIniCampos(o, c)
        }
        for s in resto {
            let r = try s.exec(self, f)
            if case .vuelve = r { break }
        }
    }

    func ejecutaIniCampos(_ o: CObj, _ c: CClase) throws {
        guard let fn = iniCampos[c.id] else { return }
        _ = try llama(fn, [], este: o, padre: o.exterior, linea: c.linea)
    }

    // MARK: declaraciones

    func valorDeclarado(_ d: CDeclVar, _ f: CMarco) throws -> CV {
        let t = d.tipo
        if !d.enlaces.isEmpty { return try d.ini?.ev(self, f) ?? .vacio }
        switch t {
        case .ref(let x):
            guard let ini = d.ini else { throw CFallo("la referencia '\(d.nombre)' tiene que inicializarse", d.linea) }
            if ini.asignable { return .ref(CRefCaja(try ini.lugar(self, f))) }
            if let ll = ini as? CLlamada {
                let v = try llamada(ll, f, quiereLugar: true)
                if case .ref = v { return v }
                return convierte(v, x)
            }
            return convierte(try ini.ev(self, f), x)
        case .arr(let e, _):
            if dialecto == .java { return try d.ini?.ev(self, f) ?? .nulo }
            return try arregloDeclarado(d, e, f)
        case .clase(let n) where dialecto != .java:
            guard let c = clases[n] else { return try d.ini?.ev(self, f) ?? .n(0, .int) }
            if let args = d.ctorArgs {
                let vals = try args.map { try $0.ev(self, f) }
                return .o(try nuevoObjeto(c, vals, d.linea))
            }
            if let ini = d.ini {
                var v = try ini.ev(self, f)
                if case .ref(let r) = v { v = try lee(r.l) }
                if case .o(let o) = v, o.clase.esSubclase(de: n) {
                    // Punto b = a; llama al constructor de copia (si no es un temporal)
                    if ini.asignable { return try copiaConstruida(v, d.linea) }
                    return v
                }
                return .o(try nuevoObjeto(c, [v], d.linea))
            }
            return .o(try nuevoObjeto(c, [], d.linea))
        case .cont where dialecto == .cpp, .cad where dialecto == .cpp:
            if let args = d.ctorArgs {
                let vals = try args.map { try $0.ev(self, f) }
                return try construyeTemporal(t, vals, args, f, d.linea)
            }
            if let ini = d.ini {
                var v = try ini.ev(self, f)
                if case .ref(let r) = v { v = try lee(r.l) }
                if case .cad = t { return try aCadena(v, d.linea) }
                if case .n = v, case .cont = t { return try construyeTemporal(t, [v], [ini], f, d.linea) }
                return copia(v)
            }
            return try valorInicialT(t)
        default:
            if let args = d.ctorArgs, let a = args.first {
                return convierte(try a.ev(self, f), t)
            }
            if let ini = d.ini {
                var v = try ini.ev(self, f)
                if case .ref(let r) = v { v = try lee(r.l) }
                if case .auto = t { return copia(v) }
                return copia(convierte(v, t))
            }
            if dialecto == .java, case .auto = t { return .nulo }
            return try valorInicialT(t)
        }
    }

    func arregloDeclarado(_ d: CDeclVar, _ e: CTipo, _ f: CMarco) throws -> CV {
        var tamanos: [Int] = []
        var tt = d.tipo
        var k = 0
        while case .arr(let sub, let n) = tt {
            var tam = n
            if k < d.dims.count, let ex = d.dims[k] { tam = Int(try entero(try ex.ev(self, f), d.linea)) }
            if tam < 0 && k > 0 { tam = 0 }
            tamanos.append(tam)
            tt = sub
            k += 1
        }
        if let l = d.ini as? CListaIni {
            l.tipo = d.tipo
            return try construyeLista(l, d.tipo, f)
        }
        if let ini = d.ini {
            let v = try ini.ev(self, f)
            if case .p(let p) = v, tamanos.count == 1, case .num(.char) = tt, let mem = p.mem {
                // char s[] = "hola";
                let bytes = try bytesC(p, d.linea)
                let n = max(tamanos[0], bytes.count + 1)
                var celdas: [CV] = bytes.map { CV.n(Int64(Int8(bitPattern: $0)), .char) }
                while celdas.count < n { celdas.append(.n(0, .char)) }
                let x = CMem(celdas)
                x.elem = .num(.char)
                x.esArreglo = true
                _ = mem
                return .p(CPtr(mem: x, i: 0))
            }
            return v
        }
        if let n = tamanos.first, n < 0 {
            throw CFallo("al arreglo '\(d.nombre)' le falta el tamaño", d.linea)
        }
        return try arregloDims(tamanos, tt)
    }

    func arregloDims(_ tamanos: [Int], _ base: CTipo) throws -> CV {
        guard let n = tamanos.first else { return try valorInicialT(base) }
        if n > 50_000_000 { throw CFallo("arreglo demasiado grande (\(n) elementos)", 0) }
        if tamanos.count == 1 { return try nuevoArreglo(base, n) }
        var celdas: [CV] = []
        celdas.reserveCapacity(n)
        let resto = Array(tamanos.dropFirst())
        for _ in 0..<n { celdas.append(try arregloDims(resto, base)) }
        let mem = CMem(celdas)
        mem.esArreglo = true
        var sub = base
        for t in resto.reversed() { sub = .arr(sub, t) }
        mem.elem = sub
        return .p(CPtr(mem: mem, i: 0))
    }

    /// auto [a, b] = par;
    func enlaza(_ d: CDeclVar, _ v: CV, _ f: CMarco) throws {
        var x = v
        if case .ref(let r) = x { x = try lee(r.l) }
        var partes: [CV] = []
        switch x {
        case .o(let o): partes = o.c.a
        case .l(let l): partes = l.a
        case .p(let p): if let mem = p.mem { partes = Array(mem.a[p.i...]) }
        default: throw CFallo("no se puede descomponer \(x.tipoNombre)", d.linea)
        }
        for (k, s) in d.slotsEnlace.enumerated() {
            guard k < partes.count else { break }
            var pv = partes[k]
            if case .ref(let r) = pv { pv = try lee(r.l) }
            f.s.a[s] = copia(pv)
        }
    }

    // MARK: listas {…}

    func construyeLista(_ l: CListaIni, _ tipo: CTipo?, _ f: CMarco) throws -> CV {
        let t = tipo?.sinRef ?? .auto
        switch t {
        case .arr(let e, _), .ptr(let e):
            var n2 = -1
            if case .arr(_, let nn) = t { n2 = nn }
            var celdas: [CV] = []
            for x in l.elems {
                if let sub = x as? CListaIni { celdas.append(try construyeLista(sub, e, f)); continue }
                var v = try x.ev(self, f)
                if case .ref(let r) = v { v = try lee(r.l) }
                if case .arr = e, case .p(let p) = v, dialecto != .java, let mem = p.mem, case .num(.char)? = mem.elem {
                    v = try arregloDesdeCadena(p, e)
                }
                celdas.append(copia(convierte(v, e)))
            }
            if dialecto != .java {
                while celdas.count < n2 { celdas.append(try valorInicialT(e)) }
            }
            let mem = CMem(celdas)
            mem.elem = e
            mem.esArreglo = true
            return .p(CPtr(mem: mem, i: 0))
        case .clase(let cn):
            guard let c = clases[cn] else { return try listaGenerica(l, f) }
            try preparaClase(c)
            if dialecto == .java { throw CFallo("en Java se crea con new \(cn)(…)", l.linea) }
            let tieneCtor = c.ctors.contains { $0.cuerpo != nil }
            if tieneCtor {
                let vals = try l.elems.map { try $0.ev(self, f) }
                return .o(try nuevoObjeto(c, vals, l.linea))
            }
            let o = try creaObjeto(c, l.linea)
            if let p = c.padre { try construye(o, p, [], l.linea) }
            try ejecutaIniCampos(o, c)
            let propios = c.campos.filter { !$0.estatico }
            var pos = 0
            for (k, x) in l.elems.enumerated() {
                if let nombre = l.nombres[k], let j = propios.firstIndex(where: { $0.nombre == nombre }) { pos = j }
                guard pos < propios.count, let i = c.indice[propios[pos].nombre] else { break }
                let ft = propios[pos].tipo
                if let sub = x as? CListaIni { o.c.a[i] = try construyeLista(sub, ft, f) }
                else {
                    var v = try x.ev(self, f)
                    if case .ref(let r) = v { v = try lee(r.l) }
                    if case .arr(let ae, _) = ft, case .p(let p) = v, case .num(.char) = ae { v = try arregloDesdeCadena(p, ft) }
                    else if case .cad = ft { v = try aCadena(v, l.linea) }
                    o.c.a[i] = copia(convierte(v, ft))
                }
                pos += 1
            }
            return .o(o)
        case .cont(let n, let args):
            return try listaContenedor(l, n, args, f)
        case .cad:
            if let x = l.elems.first { return try aCadena(try x.ev(self, f), l.linea) }
            return .s([])
        case .num, .real:
            if let x = l.elems.first { return convierte(try x.ev(self, f), t) }
            return try valorInicialT(t)
        default:
            return try listaGenerica(l, f)
        }
    }

    func arregloDesdeCadena(_ p: CPtr, _ t: CTipo) throws -> CV {
        var n = -1
        if case .arr(_, let k) = t { n = k }
        let bytes = try bytesC(p, 0)
        var celdas: [CV] = bytes.map { CV.n(Int64(Int8(bitPattern: $0)), .char) }
        while celdas.count < max(n, bytes.count + 1) { celdas.append(.n(0, .char)) }
        let mem = CMem(celdas)
        mem.elem = .num(.char)
        mem.esArreglo = true
        return .p(CPtr(mem: mem, i: 0))
    }

    func listaGenerica(_ l: CListaIni, _ f: CMarco) throws -> CV {
        var vals: [CV] = []
        for x in l.elems {
            if let sub = x as? CListaIni { vals.append(try listaGenerica(sub, f)); continue }
            vals.append(copia(try x.ev(self, f)))
        }
        if dialecto == .cpp && vals.count == 2 && l.elems.allSatisfy({ !($0 is CListaIni) }) {
            // {a, b} sin tipo: lo más probable es un par
            if case .n = vals[0] {} else { return par(vals[0], vals[1]) }
        }
        return .l(CLista("vector", vals))
    }

    func listaContenedor(_ l: CListaIni, _ n: String, _ args: [CTipo], _ f: CMarco) throws -> CV {
        let te = args.first ?? .auto
        switch n {
        case "pair":
            let a = try valorElemento(l.elems.first, args.first, f)
            let b = try valorElemento(l.elems.count > 1 ? l.elems[1] : nil, args.count > 1 ? args[1] : nil, f)
            return par(a, b)
        case "tuple":
            var vals: [CV] = []
            for (k, x) in l.elems.enumerated() { vals.append(try valorElemento(x, k < args.count ? args[k] : nil, f)) }
            return tupla(vals)
        case "map", "unordered_map", "multimap":
            guard case .m(let mp) = try nuevoContenedor(n, args) else { return .vacio }
            for x in l.elems {
                guard let sub = x as? CListaIni, sub.elems.count == 2 else { continue }
                let k = try valorElemento(sub.elems[0], args.first, f)
                let v = try valorElemento(sub.elems[1], args.count > 1 ? args[1] : nil, f)
                mp.pon(try clave(k, l.linea), k, v)
            }
            return .m(mp)
        case "set", "unordered_set", "multiset":
            guard case .m(let mp) = try nuevoContenedor(n, args) else { return .vacio }
            for x in l.elems {
                let k = try valorElemento(x, te, f)
                mp.pon(try clave(k, l.linea), k, k)
            }
            return .m(mp)
        default:
            guard case .l(let lista) = try nuevoContenedor(n, args) else { return .vacio }
            for x in l.elems { lista.a.append(try valorElemento(x, te, f)) }
            if n == "priority_queue" { try reordenaCola(lista, l.linea) }
            return .l(lista)
        }
    }

    func valorElemento(_ x: CExpr?, _ t: CTipo?, _ f: CMarco) throws -> CV {
        guard let x = x else { return try valorInicialT(t ?? .num(.int)) }
        if let sub = x as? CListaIni { return try construyeLista(sub, t, f) }
        var v = try x.ev(self, f)
        if case .ref(let r) = v { v = try lee(r.l) }
        if let tt = t {
            if case .cad = tt.sinRef { return try aCadena(v, x.linea) }
            if case .auto = tt {} else { v = convierte(v, tt) }
        }
        return copia(v)
    }

    func tupla(_ vals: [CV]) -> CV {
        let c: CClase
        if let x = prog.clases["$tupla"] { c = x } else {
            c = prog.clase("$tupla", 0)
            c.preparada = true
            c.listo = true
            c.nativa = true
        }
        return .o(CObj(c, CMem(vals)))
    }

    // MARK: new

    func nuevo(_ e: CNuevo, _ f: CMarco) throws -> CV {
        if dialecto == .java { return try nuevoJava(e, f) }
        if !e.dims.isEmpty || e.ini != nil {
            var n = 0
            if let d0 = e.dims.first { n = Int(try entero(try d0.ev(self, f), e.linea)) }
            if n < 0 { throw CFallo("new con tamaño negativo (\(n))", e.linea) }
            if let ini = e.ini {
                ini.tipo = .arr(e.t, max(n, ini.elems.count))
                return try construyeLista(ini, ini.tipo, f)
            }
            var resto: [Int] = []
            for d in e.dims.dropFirst() { resto.append(Int(try entero(try d.ev(self, f), e.linea))) }
            var base = e.t
            for t in resto.reversed() { base = .arr(base, t) }
            return try nuevoArreglo(base, n)
        }
        let vals = try e.args.map { try $0.ev(self, f) }
        let v: CV
        switch e.t {
        case .clase(let n):
            guard let c = clases[n] else { throw CFallo("tipo desconocido '\(n)'", e.linea) }
            v = .o(try nuevoObjeto(c, vals, e.linea))
        case .cont, .cad:
            v = try construyeTemporal(e.t, vals, e.args, f, e.linea)
        default:
            if let x = vals.first { v = convierte(x, e.t) } else { v = try valorInicialT(e.t) }
        }
        let mem = CMem([v])
        mem.elem = e.t
        return .p(CPtr(mem: mem, i: 0))
    }

    func nuevoJava(_ e: CNuevo, _ f: CMarco) throws -> CV {
        if let ini = e.ini { return try construyeLista(ini, ini.tipo, f) }
        if !e.dims.isEmpty {
            var tamanos: [Int] = []
            for d in e.dims {
                let n = Int(try entero(try d.ev(self, f), e.linea))
                if n < 0 { throw excepcion("NegativeArraySizeException", String(n), e.linea) }
                tamanos.append(n)
            }
            return try arregloJava(tamanos, e.t, dimsTotales: e.dims.count)
        }
        let vals = try e.args.map { try $0.ev(self, f) }
        switch e.t {
        case .clase(let n):
            guard let c = e.anonima ?? clases[n] else { return try nuevoNativoJava(n, [], vals, e.linea) }
            if e.anonima != nil {
                let o = try creaObjeto(c, e.linea)
                o.exterior = f
                try construye(o, c, vals, e.linea)
                return .o(o)
            }
            return .o(try nuevoObjeto(c, vals, e.linea))
        case .cont(let n, let args):
            return try nuevoNativoJava(n, args, vals, e.linea)
        case .cad:
            if let v = vals.first { return try cadenaJavaDe(v, vals, e.linea) }
            return .s([])
        default:
            return try nuevoNativoJava(e.t.nombre, [], vals, e.linea)
        }
    }

    func arregloJava(_ tamanos: [Int], _ base: CTipo, dimsTotales: Int) throws -> CV {
        guard let n = tamanos.first else { return .nulo }
        if n > 50_000_000 { throw excepcion("OutOfMemoryError", "arreglo demasiado grande", 0) }
        if tamanos.count == 1 {
            var celdas: [CV] = []
            celdas.reserveCapacity(n)
            let v = try valorCampo(base)
            for _ in 0..<n { celdas.append(v) }
            let mem = CMem(celdas)
            mem.elem = base
            mem.esArreglo = true
            return .p(CPtr(mem: mem, i: 0))
        }
        var celdas: [CV] = []
        let resto = Array(tamanos.dropFirst())
        for _ in 0..<n { celdas.append(try arregloJava(resto, base, dimsTotales: dimsTotales)) }
        let mem = CMem(celdas)
        var sub = base
        for _ in resto { sub = .arr(sub, -1) }
        mem.elem = sub
        mem.esArreglo = true
        return .p(CPtr(mem: mem, i: 0))
    }

    // MARK: for-each

    func elementos(_ v: CV, _ base: CLugar?, _ linea: Int) throws -> [CLugar] {
        var x = v
        if case .ref(let r) = x { x = try lee(r.l) }
        switch x {
        case .p(let p):
            guard let mem = p.mem else { throw excepcion("NullPointerException", "for sobre un arreglo null", linea) }
            return (p.i..<mem.a.count).map { CLugar.celda(mem, $0) }
        case .l(let l):
            if l.k == "priority_queue" || l.k == "PriorityQueue" {
                let copiaL = try ordenados(l, linea)
                let mem = CMem(copiaL)
                return (0..<copiaL.count).map { CLugar.celda(mem, $0) }
            }
            if l.k == "stack" || l.k == "Stack" && dialecto != .java {
                return (0..<l.a.count).reversed().map { CLugar.lista(l, $0) }
            }
            return (0..<l.a.count).map { CLugar.lista(l, $0) }
        case .m(let mp):
            var out: [CLugar] = []
            for k in mp.claves {
                guard let (kv, _) = mp.vals[k] else { continue }
                let repeticiones = mp.multi ? (mp.cuentas[k] ?? 1) : 1
                for _ in 0..<repeticiones {
                    if mp.esSet { out.append(.celda(CMem([kv]), 0)); continue }
                    let parV = dialecto == .java ? entrada(mp, k) : par(kv, .ref(CRefCaja(.mapa(mp, kv))))
                    out.append(.celda(CMem([parV]), 0))
                }
            }
            return out
        case .s(let u):
            if let b = base { return (0..<u.count).map { CLugar.car(b, $0) } }
            let mem = CMem(u.map { caracter($0) })
            return (0..<u.count).map { CLugar.celda(mem, $0) }
        case .nulo:
            throw excepcion("NullPointerException", "for sobre una colección null", linea)
        case .o(let o) where o.clase.tieneMetodo("iterator") || o.clase.tieneMetodo("begin"):
            return try elementosIterables(o, linea)
        default:
            throw CFallo("no se puede recorrer \(x.tipoNombre) con for (… : …)", linea)
        }
    }

    /// for (x : objeto) con iterator() de Java (o begin()/end() de C++)
    func elementosIterables(_ o: CObj, _ linea: Int) throws -> [CLugar] {
        let nombre = o.clase.tieneMetodo("iterator") ? "iterator" : "begin"
        let it = try llamaMetodoValor(.o(o), nombre, [], linea)
        var vals: [CV] = []
        switch it {
        case .it(let b, let i):
            if let l = b.lista { vals = Array(l.a[max(0, i)...]) }
            else if let mp = b.mapa { vals = mp.claves.compactMap { mp.vals[$0]?.0 } }
        case .o:
            var guardia = 0
            while aBool(try llamaMetodoValor(it, "hasNext", [], linea)) && guardia < 10_000_000 {
                vals.append(try llamaMetodoValor(it, "next", [], linea))
                guardia += 1
            }
        default:
            break
        }
        let mem = CMem(vals)
        return (0..<vals.count).map { CLugar.celda(mem, $0) }
    }

    /// Map.Entry de Java: getKey(), getValue(), setValue()
    func entrada(_ mp: CMapa, _ k: CKey) -> CV {
        guard let (kv, _) = mp.vals[k] else { return .nulo }
        return par(kv, .ref(CRefCaja(.mapa(mp, kv))))
    }

    func asignaElemento(_ pc: CParaCada, _ l: CLugar, _ f: CMarco) throws {
        if pc.slots.count == 1 {
            if pc.esRef { f.s.a[pc.slots[0]] = .ref(CRefCaja(l)); return }
            var v = try lee(l)
            if case .auto = pc.tipoVar {} else { v = convierte(v, pc.tipoVar) }
            f.s.a[pc.slots[0]] = copia(v)
            return
        }
        let v = try lee(l)
        guard case .o(let o) = v else { throw CFallo("no se puede descomponer \(v.tipoNombre)", pc.linea) }
        for (k, s) in pc.slots.enumerated() where k < o.c.a.count {
            let campo = o.c.a[k]
            if pc.esRef {
                if case .ref = campo { f.s.a[s] = campo } else { f.s.a[s] = .ref(CRefCaja(.celda(o.c, k))) }
            } else {
                var x = campo
                if case .ref(let r) = x { x = try lee(r.l) }
                f.s.a[s] = copia(x)
            }
        }
    }

    // MARK: miembros que no son campos

    func miembro(_ base: CV, _ n: String, _ flecha: Bool, _ nodo: CMiembro, _ linea: Int) throws -> CV {
        var b = base
        if case .ref(let r) = b { b = try lee(r.l) }
        switch b {
        case .p(let p):
            if flecha || dialecto != .java {
                if let mem = p.mem, n != "length" {
                    try prepara(mem, nodo.a.tipo?.sinRef.elemento, linea)
                    guard p.i >= 0 && p.i < mem.a.count else { throw CFallo("p->\(n) fuera de la memoria reservada", linea) }
                    var x = mem.a[p.i]
                    if case .o = x {} else { x = try objetoEnCelda(mem, p.i, nodo.a.tipo?.sinRef.elemento, n, linea) }
                    guard case .o(let o) = x, let i = nodo.indiceCampo(o) else {
                        throw CFallo("'\(n)' no es un campo", linea)
                    }
                    var v = o.c.a[i]
                    if case .ref(let r) = v { v = try lee(r.l) }
                    return v
                }
                if p.mem == nil { throw CFallo("p->\(n) con p = NULL", linea) }
            }
            if n == "length", let mem = p.mem { return .n(Int64(mem.a.count - p.i), .int) }
        case .it(let ib, let i):
            if let mp = ib.mapa {
                let idx = ib.reves ? mp.claves.count - 1 - i : i
                guard idx >= 0 && idx < mp.claves.count, let (kv, vv) = mp.vals[mp.claves[idx]] else {
                    throw CFallo("iterador fuera del mapa (¿comparaste con end()?)", linea)
                }
                if n == "first" { return kv }
                if n == "second" { return vv }
            }
            let x = try leeIterador(ib, i, linea)
            if case .o(let o) = x, let k = nodo.indiceCampo(o) { return o.c.a[k] }
        case .nulo:
            throw excepcion("NullPointerException", "Cannot read field \"\(n)\" because value is null", linea)
        case .o(let o):
            throw CFallo("\(o.clase.nombre) no tiene el campo '\(n)'", linea)
        default:
            break
        }
        throw CFallo("'\(n)' no existe en \(b.tipoNombre)", linea)
    }

    // MARK: iteradores

    func leeIterador(_ b: CIterBase, _ i: Int, _ linea: Int) throws -> CV {
        if let mp = b.mapa {
            let idx = b.reves ? mp.claves.count - 1 - i : i
            guard idx >= 0 && idx < mp.claves.count, let (kv, _) = mp.vals[mp.claves[idx]] else {
                throw CFallo("iterador fuera del contenedor (¿comparaste con end()?)", linea)
            }
            if mp.esSet { return kv }
            return par(kv, .ref(CRefCaja(.mapa(mp, kv))))
        }
        return try lee(try lugarIterador(b, i, linea))
    }

    func lugarIterador(_ b: CIterBase, _ i: Int, _ linea: Int) throws -> CLugar {
        if let l = b.lista {
            let idx = b.reves ? l.a.count - 1 - i : i
            guard idx >= 0 && idx < l.a.count else { throw CFallo("iterador fuera del contenedor (¿comparaste con end()?)", linea) }
            return .lista(l, idx)
        }
        if let c = b.cad {
            let v = try lee(c.l)
            guard case .s(let u) = v else { throw CFallo("iterador de string no válido", linea) }
            let idx = b.reves ? u.count - 1 - i : i
            guard idx >= 0 && idx < u.count else { throw CFallo("iterador fuera del string", linea) }
            return .car(c.l, idx)
        }
        if let mem = b.mem {
            guard i >= 0 && i < mem.a.count else { throw CFallo("iterador fuera del arreglo", linea) }
            return .celda(mem, i)
        }
        if let mp = b.mapa {
            let idx = b.reves ? mp.claves.count - 1 - i : i
            guard idx >= 0 && idx < mp.claves.count, let (kv, _) = mp.vals[mp.claves[idx]] else {
                throw CFallo("iterador fuera del mapa", linea)
            }
            return .celda(CMem([try leeIterador(b, i, linea)]), 0).conClave(kv, mp)
        }
        throw CFallo("iterador no válido", linea)
    }

    // MARK: referencias a métodos (Java)

    func refMetodo(_ e: CRefMetodo, _ f: CMarco) throws -> CV {
        let n = e.nombre
        let linea = e.linea
        if let b = e.base {
            let v = try b.ev(self, f)
            return .f(CCierre(fn: nil, marco: nil, este: nil, nombre: n, nat: { m, a in
                try m.llamaMetodoValor(v, n, a, linea)
            }))
        }
        guard let cn = e.clase else { throw CFallo("referencia a método sin clase", linea) }
        if n == "new" {
            return .f(CCierre(fn: nil, marco: nil, este: nil, nombre: "new", nat: { m, a in
                if let c = m.clases[cn] { return .o(try m.nuevoObjeto(c, a, linea)) }
                return try m.nuevoNativoJava(cn, [], a, linea)
            }))
        }
        if let c = clases[cn] {
            let lista = metodosDe(c, n)
            return .f(CCierre(fn: nil, marco: nil, este: nil, nombre: "clave:" + n, nat: { m, a in
                if let fn = lista.first(where: { $0.estatico && $0.params.count == a.count }) {
                    return try m.llama(fn, a, este: nil, padre: nil, linea: linea)
                }
                guard let primero = a.first else { throw CFallo("\(cn)::\(n) necesita un objeto", linea) }
                return try m.llamaMetodoValor(primero, n, Array(a.dropFirst()), linea)
            }))
        }
        let nn = cn + "." + n
        let estatico = esNativoFuncion(nn)
        return .f(CCierre(fn: nil, marco: nil, este: nil, nombre: "clave:" + n, nat: { m, a in
            if estatico && !(cn == "String" && ["length", "toUpperCase", "toLowerCase", "trim", "isEmpty", "compareTo", "compareToIgnoreCase", "strip", "chars"].contains(n)) {
                return try m.nativa(nn, a, [], linea)
            }
            guard let primero = a.first else { return try m.nativa(nn, a, [], linea) }
            return try m.llamaMetodoValor(primero, n, Array(a.dropFirst()), linea)
        }))
    }

    /// obj.metodo(args) con el objeto ya evaluado.
    func llamaMetodoValor(_ v: CV, _ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        switch v {
        case .o(let o):
            let lista = metodosDe(o.clase, n)
            if !lista.isEmpty {
                let fn = try elige(lista, a.count, a, n, linea)
                return try llama(fn, a, este: fn.estatico ? nil : o, padre: o.exterior, linea: linea)
            }
            return try metodoObjetoNativo(o, n, a, linea)
        case .f(let c):
            return try metodoCierre(c, n, a, linea)
        case .nulo:
            throw excepcion("NullPointerException", "Cannot invoke \"\(n)()\" because value is null", linea)
        default:
            return try metodoNativo(v, nil, n, a, [], linea, quiereLugar: false)
        }
    }

    func valorDeRama(_ s: CSent, _ f: CMarco, _ linea: Int) throws -> CV {
        let r = try s.exec(self, f)
        if case .vuelve(let v) = r { return v }
        return .vacio
    }

    func reordenaCola(_ l: CLista, _ linea: Int) throws {
        l.a = try ordenados(l, linea)
    }
}

extension CLugar {
    func conClave(_ k: CV, _ mp: CMapa) -> CLugar { self }
}
