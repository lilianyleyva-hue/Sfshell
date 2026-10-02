import Foundation

// ============================================================
// MARK: - C, C++ y Java: resolución de nombres
// ============================================================
// Antes de ejecutar se recorre todo el programa: cada variable local
// recibe una posición en el marco de su función, cada nombre se liga
// a lo que es (local, global, campo, función, clase…) y se detectan
// los errores típicos de compilación ("'x' no está declarado").

final class CContexto {
    var ambitos: [[String: (Int, CTipo)]] = [[:]]
    var siguiente = 0
    var maximo = 0
    let clase: CClase?
    let estatico: Bool
    let fn: CFuncion?
    init(clase: CClase?, estatico: Bool, fn: CFuncion?) {
        self.clase = clase
        self.estatico = estatico
        self.fn = fn
    }
}

final class CResol {
    let m: CMaq
    var ctx: [CContexto] = []
    var avisos: [String] = []
    /// por cada bloque abierto: las variables con destructor (C++)
    var destructores: [[Int]] = []

    init(_ m: CMaq) { self.m = m }

    var claseActual: CClase? { ctx.last?.clase }
    var esEstatico: Bool { ctx.last?.estatico ?? true }

    // MARK: ámbitos

    func abre() {
        guard let c = ctx.last else { return }
        c.ambitos.append([:])
    }

    func cierra() {
        guard let c = ctx.last, c.ambitos.count > 1 else { return }
        c.ambitos.removeLast()
    }

    func declara(_ n: String, _ t: CTipo, _ linea: Int) throws -> Int {
        guard let c = ctx.last else {
            return try m.declaraGlobal(n, t, linea)
        }
        if c.ambitos[c.ambitos.count - 1][n] != nil && m.dialecto != .java {
            throw CFallo("'\(n)' ya está declarada en este bloque", linea)
        }
        if m.dialecto == .java {
            for a in c.ambitos where a[n] != nil {
                throw CFallo("la variable '\(n)' ya está definida", linea)
            }
        }
        let s = c.siguiente
        c.siguiente += 1
        c.maximo = max(c.maximo, c.siguiente)
        c.ambitos[c.ambitos.count - 1][n] = (s, t)
        return s
    }

    func buscaLocal(_ n: String) -> (Int, Int, CTipo)? {
        var prof = 0
        for c in ctx.reversed() {
            for a in c.ambitos.reversed() {
                if let (s, t) = a[n] { return (prof, s, t) }
            }
            prof += 1
            // una función normal no ve las variables de quien la llama
            if c.fn?.nombre != "lambda" && c.fn?.clase?.nombre.hasPrefix("$Anonima") != true { break }
        }
        return nil
    }

    // MARK: funciones

    func resuelveFuncion(_ fn: CFuncion) throws {
        if fn.resuelta { return }
        fn.resuelta = true
        let c = CContexto(clase: fn.clase, estatico: fn.estatico || fn.clase == nil, fn: fn)
        ctx.append(c)
        let destructoresPrevios = destructores
        destructores = []
        defer { ctx.removeLast(); destructores = destructoresPrevios }
        for p in fn.params {
            _ = try declara(p.nombre, p.tipo, fn.linea)
            try p.defecto?.res(self)
        }
        for (_, args, _) in fn.inits {
            for a in args { try a.res(self) }
        }
        try fn.cuerpo?.res(self)
        fn.nSlots = c.maximo
    }

    func resuelveLambda(_ fn: CFuncion) throws {
        let c = CContexto(clase: claseActual, estatico: esEstatico, fn: fn)
        ctx.append(c)
        defer { ctx.removeLast() }
        for p in fn.params { _ = try declara(p.nombre, p.tipo, fn.linea) }
        try fn.cuerpo?.res(self)
        fn.nSlots = c.maximo
        fn.resuelta = true
    }

    func resuelveClaseAnonima(_ c: CClase) throws {
        try m.preparaClase(c)
        // sus métodos ven las variables locales de donde se crea (como una lambda)
        for (_, lista) in c.metodos {
            for fn in lista where !fn.resuelta {
                let cx = CContexto(clase: c, estatico: false, fn: fn)
                ctx.append(cx)
                for p in fn.params { _ = try declara(p.nombre, p.tipo, fn.linea) }
                try fn.cuerpo?.res(self)
                fn.nSlots = cx.maximo
                fn.resuelta = true
                ctx.removeLast()
            }
        }
        if let ini = m.iniCampos[c.id], !ini.resuelta {
            let cx = CContexto(clase: c, estatico: false, fn: ini)
            ctx.append(cx)
            try ini.cuerpo?.res(self)
            ini.nSlots = cx.maximo
            ini.resuelta = true
            ctx.removeLast()
        }
    }

    // MARK: nombres

    func resuelveNombre(_ e: CNombre) throws {
        let n = e.nombre
        if let (p, s, t) = buscaLocal(n) {
            e.modo = s < -1 ? .global(-(s + 2)) : .local(p, s)
            e.tipo = t
            return
        }
        if let c = claseActual {
            if let i = c.indice[n] {
                if esEstatico && m.dialecto == .java {
                    throw CFallo("no se puede usar el campo '\(n)' desde un método static (no hay objeto)", e.linea)
                }
                e.modo = .campo(i)
                e.tipo = c.tipos[i]
                return
            }
            var k: CClase? = c
            while let x = k {
                if let (cc, i) = x.campoEstatico(n) {
                    e.modo = .estatico(cc, i)
                    e.tipo = cc.estTipos[i]
                    return
                }
                k = m.claseExterior(x)
            }
            if c.tieneMetodo(n) { e.modo = .metodo(n); return }
        }
        if let i = m.globalIndice[n] {
            e.modo = .global(i)
            e.tipo = m.globalTipos[i]
            return
        }
        if let v = m.prog.enumConst[n] {
            e.modo = .constante(v)
            e.tipo = .num(.int)
            return
        }
        if m.prog.funciones[n] != nil { e.modo = .funcion(n); e.tipo = .fn; return }
        if let c = m.clases[n] { e.modo = .clase(c); return }
        if m.esNativoValor(n) { e.modo = .nativo(n); return }
        if m.esNativoFuncion(n) { e.modo = .funcion(n); return }
        throw CFallo(m.dialecto == .java ? "no encuentro el símbolo '\(n)'" : "'\(n)' no está declarado (¿falta declararlo o un #include?)", e.linea)
    }

    func resuelveMiembro(_ e: CMiembro) throws {
        if let nb = e.a as? CNombre, buscaLocal(nb.nombre) == nil, claseActual?.indice[nb.nombre] == nil,
           m.globalIndice[nb.nombre] == nil {
            let cn = nb.nombre
            // Clase.estatico / Clase::estatico / Enum::valor
            if let c = m.clases[cn] {
                try m.preparaClase(c)
                if let (cc, i) = c.campoEstatico(e.nombre) {
                    e.estatico = (cc, i)
                    e.tipo = cc.estTipos[i]
                    return
                }
                if c.tieneMetodo(e.nombre) { e.claseEst = cn; return }
                if m.dialecto == .java { throw CFallo("\(cn) no tiene el miembro '\(e.nombre)'", e.linea) }
            }
            if let amb = m.prog.enumAmbito[cn], let v = amb[e.nombre] {
                e.nativo = "$const"
                e.estaticoConst = v
                e.tipo = .num(.int)
                return
            }
            if m.esClaseNativa(cn) {
                e.nativo = cn + "." + e.nombre
                e.tipo = m.tipoNativo(e.nativo!)
                return
            }
        }
        try e.a.res(self)
        e.tipo = tipoCampo(e.a.tipo, e.nombre, flecha: e.flecha)
    }

    func tipoCampo(_ base: CTipo?, _ n: String, flecha: Bool) -> CTipo? {
        guard var t = base?.sinRef else { return nil }
        if flecha, let e = t.elemento { t = e }
        switch t {
        case .clase(let cn):
            if let c = m.clases[cn] {
                if let i = c.indice[n] { return c.tipos[i] }
                if let (cc, i) = c.campoEstatico(n) { return cc.estTipos[i] }
            }
            if cn == "pair" { return .auto }
            return nil
        case .arr where n == "length":
            return .num(.int)
        case .cont(let cn, let args) where n == "first" || n == "second":
            if cn == "pair" && args.count == 2 { return n == "first" ? args[0] : args[1] }
            return nil
        default:
            return nil
        }
    }

    // MARK: llamadas

    func resuelveLlamada(_ e: CLlamada) throws {
        for a in e.args { try a.res(self) }
        if let nb = e.f as? CNombre {
            try resuelveLlamadaNombre(e, nb)
            return
        }
        if let mb = e.f as? CMiembro {
            try resuelveLlamadaMiembro(e, mb)
            return
        }
        try e.f.res(self)
        e.destino = .dinamico
    }

    func resuelveLlamadaNombre(_ e: CLlamada, _ nb: CNombre) throws {
        let n = nb.nombre
        switch n {
        case "$construye":
            e.destino = .nativo("$construye")
            e.tipo = e.plantilla.first
            if case .clase(let cn)? = e.tipo, let c = m.clases[cn] { try m.preparaClase(c) }
            return
        case "$this": e.destino = .esteCtor; e.claseCtx = claseActual; return
        case "$super": e.destino = .superCtor; e.claseCtx = claseActual; return
        default: break
        }
        if buscaLocal(n) != nil || (claseActual?.indice[n] != nil && !(claseActual?.tieneMetodo(n) ?? false)) || m.globalIndice[n] != nil {
            try nb.res(self)
            e.destino = .dinamico
            return
        }
        if let c = claseActual {
            var k: CClase? = c
            while let x = k {
                if x.tieneMetodo(n) {
                    let fn = x.metodo(n, e.args.count)
                    e.destino = (x === c && !esEstatico) ? .propio(n) : .estatico(x, n)
                    if let fn = fn {
                        e.tipo = fn.ret
                        if fn.estatico && x !== c { e.destino = .estatico(x, n) }
                    }
                    if esEstatico, let fn = fn, !fn.estatico, m.dialecto == .java, x === c {
                        throw CFallo("no se puede llamar al método '\(n)' desde un método static (falta un objeto o 'static')", e.linea)
                    }
                    return
                }
                k = m.claseExterior(x)
            }
        }
        if let fs = m.prog.funciones[n] {
            e.destino = .global(fs)
            if let f = fs.first(where: { $0.params.count == e.args.count }) ?? fs.first { e.tipo = f.ret }
            return
        }
        if let c = m.clases[n] {
            e.destino = .ctor(c)
            e.tipo = .clase(c.nombre)
            return
        }
        if m.esNativoFuncion(n) {
            e.destino = .nativo(n)
            e.tipo = m.tipoNativo(n)
            return
        }
        if m.esNativoValor(n) {
            try nb.res(self)
            e.destino = .dinamico
            return
        }
        throw CFallo(m.dialecto == .java ? "no encuentro el método '\(n)'" : "la función '\(n)' no está declarada (¿falta un #include o escribirla?)", e.linea)
    }

    func resuelveLlamadaMiembro(_ e: CLlamada, _ mb: CMiembro) throws {
        if let nb = mb.a as? CNombre {
            if nb.nombre == "$super" {
                guard let p = claseActual?.padre else { throw CFallo("super.\(mb.nombre)(): la clase no tiene padre", e.linea) }
                e.destino = .estatico(p, mb.nombre)
                e.tipo = p.metodo(mb.nombre, e.args.count)?.ret
                return
            }
            if buscaLocal(nb.nombre) == nil && claseActual?.indice[nb.nombre] == nil && m.globalIndice[nb.nombre] == nil {
                if let c = m.clases[nb.nombre], c.tieneMetodo(mb.nombre) {
                    e.destino = .estatico(c, mb.nombre)
                    e.tipo = c.metodo(mb.nombre, e.args.count)?.ret
                    return
                }
                if m.esClaseNativa(nb.nombre), m.clases[nb.nombre] == nil {
                    let nn = nb.nombre + "." + mb.nombre
                    e.destino = .nativo(nn)
                    e.tipo = m.tipoNativo(nn)
                    return
                }
            }
        }
        try mb.res(self)
        e.destino = .dinamico
        e.tipo = tipoMetodo(mb.a.tipo, mb.nombre, e.args.count, flecha: mb.flecha)
    }

    func tipoMetodo(_ base: CTipo?, _ n: String, _ nargs: Int, flecha: Bool) -> CTipo? {
        guard var t = base?.sinRef else { return nil }
        if flecha, let e = t.elemento { t = e }
        switch t {
        case .clase(let cn):
            return m.clases[cn]?.metodo(n, nargs)?.ret
        case .cad:
            switch n {
            case "length", "size", "indexOf", "lastIndexOf", "compareTo", "compareToIgnoreCase", "find", "rfind", "compare": return m.dialecto == .java ? .num(.int) : .num(.ulong)
            case "charAt": return .num(.jchar)
            case "substring", "substr", "toUpperCase", "toLowerCase", "trim", "replace", "strip", "repeat": return .cad
            case "equals", "isEmpty", "empty", "contains", "startsWith", "endsWith", "equalsIgnoreCase": return .num(.bool)
            case "at", "front", "back": return .num(.char)
            default: return nil
            }
        case .cont(let cn, let args):
            switch n {
            case "size", "length": return m.dialecto == .java ? .num(.int) : .num(.ulong)
            case "empty", "isEmpty", "contains", "containsKey": return .num(.bool)
            case "get", "at", "front", "back", "top", "peek", "poll", "pop", "remove", "getFirst", "getLast", "pollFirst", "pollLast", "removeFirst", "removeLast", "peekFirst", "peekLast", "first", "last":
                if cn.contains("map") || cn.contains("Map") { return args.count > 1 ? args[1] : nil }
                return args.first
            default: return nil
            }
        default:
            return nil
        }
    }

    // MARK: declaraciones

    func resuelveDecl(_ d: CDeclVar) throws {
        if let l = d.ini as? CListaIni { l.tipo = d.tipo.sinRef }
        let esLambda = d.ini is CLambda
        if esLambda && !d.global { d.slot = try declara(d.nombre, d.tipo, d.linea) }
        try d.ini?.res(self)
        if let args = d.ctorArgs { for a in args { try a.res(self) } }
        for x in d.dims { try x?.res(self) }
        if case .auto = d.tipo, let t = d.ini?.tipo { d.tipo = t.sinRef }
        if case .ref(.auto) = d.tipo, let t = d.ini?.tipo { d.tipo = .ref(t.sinRef) }
        if !d.enlaces.isEmpty {
            d.slotsEnlace = []
            for n in d.enlaces { d.slotsEnlace.append(try declara(n, .auto, d.linea)) }
            return
        }
        if d.estatico && !ctx.isEmpty {
            // static local: vive en la memoria global, con un nombre propio
            d.slot = try m.declaraGlobal("$\(d.nombre)#\(d.linea)#\(m.globalTipos.count)", d.tipo, d.linea)
            d.global = true
            guard let c = ctx.last else { return }
            c.ambitos[c.ambitos.count - 1][d.nombre] = (-(d.slot + 2), d.tipo)
            return
        }
        if d.global || ctx.isEmpty {
            d.global = true
            if let i = m.globalIndice[d.nombre] {
                d.slot = i
                m.globalTipos[i] = d.tipo
            } else {
                d.slot = try m.declaraGlobal(d.nombre, d.tipo, d.linea)
            }
            return
        }
        if !esLambda { d.slot = try declara(d.nombre, d.tipo, d.linea) }
        if m.dialecto == .cpp, case .clase(let cn) = d.tipo, let c = m.clases[cn], m.tieneDestructor(c), !destructores.isEmpty {
            destructores[destructores.count - 1].append(d.slot)
        }
    }

    func resuelveCaso(_ e: CExpr, _ tipoSwitch: CTipo?) throws {
        if let nb = e as? CNombre, buscaLocal(nb.nombre) == nil {
            // case ROJO: de un enum de Java
            var candidatas: [CClase] = []
            if case .clase(let cn)? = tipoSwitch?.sinRef, let c = m.clases[cn], c.esEnum { candidatas = [c] }
            else { candidatas = m.prog.ordenClases.filter { $0.esEnum } }
            for c in candidatas {
                try m.preparaClase(c)
                if let i = c.estIndice[nb.nombre] {
                    nb.modo = .estatico(c, i)
                    return
                }
            }
        }
        try e.res(self)
    }

    // MARK: tipos estáticos

    func tipoBinario(_ op: COp, _ a: CTipo?, _ b: CTipo?) -> CTipo? {
        if op.esComparacion { return m.tipoVerdad }
        guard let x = a?.sinRef, let y = b?.sinRef else { return nil }
        switch (x, y) {
        case (.num(let p), .num(let q)):
            if op == .shl || op == .shr || op == .ushr { return .num(CNum.comun(p, .int)) }
            return .num(CNum.comun(p, q))
        case (.real(let f), .num), (.num, .real(let f)): return .real(f)
        case (.real(let f), .real(let g)): return .real(f && g)
        case (.ptr, .num), (.arr, .num):
            if op == .suma || op == .resta { return x.elemento.map { .ptr($0) } }
            return nil
        case (.num, .ptr), (.num, .arr):
            return op == .suma ? y.elemento.map { .ptr($0) } : nil
        case (.ptr, .ptr):
            return op == .resta ? .num(.long) : nil
        case (.cad, _), (_, .cad):
            return op == .suma ? .cad : nil
        default:
            return nil
        }
    }

    func tipoConversion(_ t: CTipo?) -> CTipo? {
        guard let x = t?.sinRef else { return nil }
        switch x {
        case .num, .real, .ptr: return x
        case .cad: return m.dialecto == .cpp ? .cad : nil
        default: return nil
        }
    }

    func tipoElemento(_ t: CTipo?) -> CTipo? {
        guard let x = t?.sinRef else { return nil }
        switch x {
        case .arr(let e, _), .ptr(let e): return e
        case .cad: return .num(m.dialecto == .java ? .jchar : .char)
        case .cont(let n, let a):
            if n.contains("map") || n.contains("Map") { return a.count == 2 ? .cont("pair", a) : nil }
            return a.first
        default: return nil
        }
    }
}

