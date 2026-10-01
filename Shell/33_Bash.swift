import Foundation

// ============================================================
// MARK: - Estructuras de control al estilo de bash
// ============================================================
// if / elif / else / fi
// for NOMBRE in lista; do … done
// while COND; do … done      until COND; do … done
// case VALOR in patrón) … ;; esac
// nombre() { … }             break / continue / return
//
// Todo se apoya en Shell.execute: aquí solo se decide qué líneas
// se ejecutan, en qué orden y cuántas veces.

enum Bash {

    enum Flujo { case normal, romper, seguir, retorno }

    static let aperturas: Set<String> = ["if", "for", "while", "until", "case"]
    static let cierres: Set<String> = ["fi", "done", "esac"]

    /// Puntero sobre la lista de sentencias (evita pasar índices por todos lados).
    final class Cursor {
        let st: [String]
        var i = 0
        init(_ st: [String]) { self.st = st }
    }

    // --------------------------------------------------------
    // Trocear las líneas en sentencias
    // --------------------------------------------------------

    static func primera(_ s: String) -> String {
        String(s.split(separator: " ").first ?? "")
    }

    static func resto(_ s: String) -> String {
        let p = s.split(separator: " ", maxSplits: 1, omittingEmptySubsequences: true)
        return p.count > 1 ? String(p[1]).trimmingCharacters(in: .whitespaces) : ""
    }

    static func nombreValido(_ n: String) -> Bool {
        guard let f = n.first, f.isLetter || f == "_" else { return false }
        return n.allSatisfy { $0.isLetter || $0.isNumber || $0 == "_" }
    }

    /// ¿Es la cabecera de una función?  nombre() {   ·   function nombre {
    static func esFuncion(_ s: String) -> String? {
        var t = s.trimmingCharacters(in: .whitespaces)
        if t.hasSuffix("{") { t = String(t.dropLast()).trimmingCharacters(in: .whitespaces) }
        // 'function nombre() {' se deja para JavaScript: aquí solo  nombre() {
        if t.hasPrefix("function ") { return nil }
        guard t.hasSuffix(")") else { return nil }
        let sinCierre = String(t.dropLast()).trimmingCharacters(in: .whitespaces)
        guard sinCierre.hasSuffix("(") else { return nil }
        let nom = String(sinCierre.dropLast()).trimmingCharacters(in: .whitespaces)
        return nombreValido(nom) ? nom : nil
    }

    /// Corta por ';' (respetando comillas) y separa las palabras clave pegadas.
    static func partir(_ lineas: [String]) -> [String] {
        var bruto: [String] = []
        func guarda(_ s: String) {
            let t = s.trimmingCharacters(in: .whitespaces)
            if !t.isEmpty { bruto.append(t) }
        }

        for l in lineas {
            var cur = ""
            var comilla: Character? = nil
            var i = l.startIndex
            while i < l.endIndex {
                let ch = l[i]
                if let q = comilla {
                    cur.append(ch)
                    if ch == q { comilla = nil }
                    i = l.index(after: i); continue
                }
                if ch == "'" || ch == "\"" { comilla = ch; cur.append(ch); i = l.index(after: i); continue }
                if ch == "#", cur.trimmingCharacters(in: .whitespaces).isEmpty { break }   // comentario
                if ch == ";" {
                    let n = l.index(after: i)
                    if n < l.endIndex, l[n] == ";" {          // ;; de case
                        guarda(cur); cur = ""
                        bruto.append(";;")
                        i = l.index(after: n); continue
                    }
                    guarda(cur); cur = ""
                    i = n; continue
                }
                cur.append(ch)
                i = l.index(after: i)
            }
            guarda(cur)
        }

        var salida: [String] = []
        for s in bruto {
            var t = s

            // "saluda() { echo hola" → "saluda() {" + "echo hola"
            if let r = t.range(of: "{"),
               esFuncion(String(t[t.startIndex..<r.lowerBound]) + "{") != nil {
                salida.append(String(t[t.startIndex..<r.lowerBound]).trimmingCharacters(in: .whitespaces) + " {")
                t = String(t[r.upperBound...]).trimmingCharacters(in: .whitespaces)
            }

            // "then echo hola" → "then" + "echo hola"
            var seguir = true
            while seguir {
                seguir = false
                for k in ["then", "do", "else"] where t != k && t.hasPrefix(k + " ") {
                    salida.append(k)
                    t = String(t.dropFirst(k.count + 1)).trimmingCharacters(in: .whitespaces)
                    seguir = true
                    break
                }
            }
            if !t.isEmpty { salida.append(t) }
        }
        return salida
    }

    /// ¿La línea abre una estructura de control?
    static func empieza(_ linea: String) -> Bool {
        guard let p = partir([linea]).first else { return false }
        let w = primera(p)
        if aperturas.contains(w) {
            // 'if (x > 1) {' o 'for (let i…)' es JavaScript, no shell
            return !resto(p).hasPrefix("(")
        }
        return esFuncion(p) != nil
    }

    /// ¿Falta cerrar algún bloque? (para seguir pidiendo líneas)
    static func abierto(_ lineas: [String]) -> Bool {
        var prof = 0
        for s in partir(lineas) {
            let w = primera(s)
            if aperturas.contains(w) || esFuncion(s) != nil || w == "{" { prof += 1 }
            if cierres.contains(w) || w == "}" { prof -= 1 }
        }
        return prof > 0
    }

    /// Recoge las sentencias hasta una de 'fin', sin consumirla, contando anidamiento.
    static func recoger(_ c: Cursor, _ fin: Set<String>) -> [String] {
        var out: [String] = []
        var prof = 0
        while c.i < c.st.count {
            let s = c.st[c.i]
            let w = primera(s)
            if prof == 0, fin.contains(w) { break }
            if aperturas.contains(w) || esFuncion(s) != nil || w == "{" { prof += 1 }
            if cierres.contains(w) || w == "}" { prof -= 1 }
            out.append(s)
            c.i += 1
        }
        return out
    }

    // --------------------------------------------------------
    // Ejecución
    // --------------------------------------------------------

    static func correr(_ lineas: [String], _ sh: Shell) async -> String {
        let (texto, _) = await ejecutar(partir(lineas), sh)
        return texto
    }

    static func ejecutar(_ st: [String], _ sh: Shell) async -> (String, Flujo) {
        let c = Cursor(st)
        var out = ""
        var pasos = 0

        while c.i < st.count {
            pasos += 1
            if pasos > 50_000 {
                out += Shell.errMark + "bash: demasiadas sentencias seguidas, lo paro\n"
                break
            }
            let s = st[c.i]
            let w = primera(s)

            if cierres.contains(w) || w == ";;" || w == "}" || w == "{" || w == "then" || w == "do" {
                c.i += 1; continue
            }

            switch w {
            case "if":
                let (t, f) = await hazIf(c, sh)
                out += t
                if case .normal = f {} else { return (out, f) }

            case "while", "until":
                let (t, f) = await hazWhile(c, sh, invertido: w == "until")
                out += t
                if case .retorno = f { return (out, f) }

            case "for":
                let (t, f) = await hazFor(c, sh)
                out += t
                if case .retorno = f { return (out, f) }

            case "case":
                let (t, f) = await hazCase(c, sh)
                out += t
                if case .normal = f {} else { return (out, f) }

            case "break":
                c.i += 1
                return (out, .romper)

            case "continue":
                c.i += 1
                return (out, .seguir)

            case "return":
                let p = s.split(separator: " ")
                sh.env.vars["?"] = p.count > 1 ? String(p[1]) : "0"
                c.i += 1
                return (out, .retorno)

            default:
                if let nombre = esFuncion(s) {
                    c.i += 1
                    if c.i < st.count, primera(st[c.i]) == "{" { c.i += 1 }
                    let cuerpo = recoger(c, ["}"])
                    if c.i < st.count { c.i += 1 }
                    sh.env.funcs[nombre] = cuerpo
                    sh.env.vars["?"] = "0"
                    continue
                }
                out += await sh.execute(s)
                c.i += 1
            }
        }
        return (out, .normal)
    }

    static func cierto(_ sh: Shell) -> Bool { (sh.env.vars["?"] ?? "1") == "0" }

    static func hazIf(_ c: Cursor, _ sh: Shell) async -> (String, Flujo) {
        var ramas: [(String, [String])] = []
        var otra: [String]? = nil
        var cond = resto(c.st[c.i])

        while true {
            c.i += 1
            if c.i < c.st.count, primera(c.st[c.i]) == "then" { c.i += 1 }
            let cuerpo = recoger(c, ["elif", "else", "fi"])
            ramas.append((cond, cuerpo))
            guard c.i < c.st.count else { break }
            let k = primera(c.st[c.i])
            if k == "elif" { cond = resto(c.st[c.i]); continue }
            if k == "else" { c.i += 1; otra = recoger(c, ["fi"]) }
            break
        }
        if c.i < c.st.count, primera(c.st[c.i]) == "fi" { c.i += 1 }

        var out = ""
        for (cnd, cuerpo) in ramas {
            out += await sh.execute(cnd)
            if cierto(sh) {
                let (t, f) = await ejecutar(cuerpo, sh)
                return (out + t, f)
            }
        }
        if let e = otra {
            let (t, f) = await ejecutar(e, sh)
            return (out + t, f)
        }
        sh.env.vars["?"] = "0"
        return (out, .normal)
    }

    static func hazWhile(_ c: Cursor, _ sh: Shell, invertido: Bool) async -> (String, Flujo) {
        let cond = resto(c.st[c.i])
        c.i += 1
        if c.i < c.st.count, primera(c.st[c.i]) == "do" { c.i += 1 }
        let cuerpo = recoger(c, ["done"])
        if c.i < c.st.count { c.i += 1 }

        var out = ""
        var vueltas = 0
        while true {
            vueltas += 1
            if vueltas > 10_000 {
                out += Shell.errMark + "\(invertido ? "until" : "while"): 10000 vueltas, lo paro\n"
                break
            }
            out += await sh.execute(cond)
            var sigue = cierto(sh)
            if invertido { sigue = !sigue }
            if !sigue { break }
            let (t, f) = await ejecutar(cuerpo, sh)
            out += t
            if case .romper = f { break }
            if case .retorno = f { return (out, f) }
        }
        return (out, .normal)
    }

    static func hazFor(_ c: Cursor, _ sh: Shell) async -> (String, Flujo) {
        let cabecera = resto(c.st[c.i])
        c.i += 1
        if c.i < c.st.count, primera(c.st[c.i]) == "do" { c.i += 1 }
        let cuerpo = recoger(c, ["done"])
        if c.i < c.st.count { c.i += 1 }

        var out = ""
        let trozos = cabecera.components(separatedBy: " in ")
        let nombre = (trozos.first ?? "").trimmingCharacters(in: .whitespaces)
        guard nombreValido(nombre) else {
            return (Shell.errMark + "for: falta el nombre de la variable\n", .normal)
        }
        var listaTexto = trozos.count > 1 ? trozos.dropFirst().joined(separator: " in ") : ""
        if listaTexto.isEmpty { listaTexto = sh.env.positional.joined(separator: " ") }   // for x; do
        if listaTexto.contains("$(") {
            listaTexto = (try? await sh.substitute(listaTexto)) ?? listaTexto
        }
        let valores = (try? Parser.palabras(listaTexto, sh.env)) ?? []

        for v in valores.prefix(10_000) {
            sh.env.vars[nombre] = v
            let (t, f) = await ejecutar(cuerpo, sh)
            out += t
            if case .romper = f { break }
            if case .retorno = f { return (out, f) }
        }
        return (out, .normal)
    }

    static func hazCase(_ c: Cursor, _ sh: Shell) async -> (String, Flujo) {
        var cab = resto(c.st[c.i])
        c.i += 1
        if cab.hasSuffix(" in") { cab = String(cab.dropLast(3)).trimmingCharacters(in: .whitespaces) }
        if c.i < c.st.count, primera(c.st[c.i]) == "in" { c.i += 1 }
        let cuerpo = recoger(c, ["esac"])
        if c.i < c.st.count { c.i += 1 }

        let valor = ((try? Parser.palabras(cab, sh.env)) ?? []).first ?? ""
        var k = 0
        while k < cuerpo.count {
            let s = cuerpo[k]
            guard let cierre = s.firstIndex(of: ")") else { k += 1; continue }
            var patrones = String(s[s.startIndex..<cierre]).trimmingCharacters(in: .whitespaces)
            if patrones.hasPrefix("(") { patrones = String(patrones.dropFirst()).trimmingCharacters(in: .whitespaces) }
            let primeraSent = String(s[s.index(after: cierre)...]).trimmingCharacters(in: .whitespaces)
            var sentencias: [String] = primeraSent.isEmpty ? [] : [primeraSent]
            k += 1
            while k < cuerpo.count, cuerpo[k] != ";;" { sentencias.append(cuerpo[k]); k += 1 }
            if k < cuerpo.count { k += 1 }      // salta el ;;

            let coincide = patrones.components(separatedBy: "|").contains { p in
                let pat = p.trimmingCharacters(in: .whitespaces)
                return pat == "*" || Parser.match(Array(pat), Array(valor))
            }
            if coincide {
                let (t, f) = await ejecutar(sentencias, sh)
                return (t, f)
            }
        }
        sh.env.vars["?"] = "0"
        return ("", .normal)
    }

    // --------------------------------------------------------
    // test / [ … ]
    // --------------------------------------------------------

    static func evalTest(_ a: [String], _ ctx: Ctx) throws -> Bool {
        if a.isEmpty { return false }

        if let i = a.firstIndex(of: "-o") {
            return try evalTest(Array(a[..<i]), ctx) || evalTest(Array(a[(i + 1)...]), ctx)
        }
        if let i = a.firstIndex(of: "-a") {
            return try evalTest(Array(a[..<i]), ctx) && evalTest(Array(a[(i + 1)...]), ctx)
        }
        if a.count == 1 { return !a[0].isEmpty }

        if a.count == 2 {
            let u = try? ctx.env.resolve(a[1])
            let existe = u.map { ctx.env.exists($0) } ?? false
            let esDir = u.map { ctx.env.isDir($0) } ?? false
            switch a[0] {
            case "-e": return existe
            case "-f": return existe && !esDir
            case "-d": return esDir
            case "-s":
                guard let u, existe, let d = FileManager.default.contents(atPath: u.path) else { return false }
                return !d.isEmpty
            case "-r", "-w": return existe
            case "-x":
                let ejecutables = [".js", ".sh", ".swift", ".sim", ".mix", ".esc"]
                return existe && ejecutables.contains { a[1].hasSuffix($0) }
            case "-z": return a[1].isEmpty
            case "-n": return !a[1].isEmpty
            default: throw ShErr("test: operador desconocido '\(a[0])'")
            }
        }

        if a.count == 3 {
            let x = a[0], op = a[1], y = a[2]
            func num(_ s: String) throws -> Double {
                guard let d = Double(s) else { throw ShErr("test: '\(s)' no es un número") }
                return d
            }
            switch op {
            case "=", "==": return x == y
            case "!=": return x != y
            case "<": return x < y
            case ">": return x > y
            case "-eq": return try num(x) == num(y)
            case "-ne": return try num(x) != num(y)
            case "-lt": return try num(x) < num(y)
            case "-le": return try num(x) <= num(y)
            case "-gt": return try num(x) > num(y)
            case "-ge": return try num(x) >= num(y)
            case "-nt", "-ot":
                let fa = try? FileManager.default.attributesOfItem(atPath: ctx.env.resolve(x).path)
                let fb = try? FileManager.default.attributesOfItem(atPath: ctx.env.resolve(y).path)
                let da = (fa?[.modificationDate] as? Date) ?? .distantPast
                let db = (fb?[.modificationDate] as? Date) ?? .distantPast
                return op == "-nt" ? da > db : da < db
            default: throw ShErr("test: operador desconocido '\(op)'")
            }
        }

        throw ShErr("test: número de argumentos incorrecto")
    }
}

// ============================================================
// MARK: - Comandos que acompañan a las estructuras de control
// ============================================================

extension Shell {

    static func bash() -> [String: Spec] {
        var c: [String: Spec] = [:]

        c["test"] = Spec(help: "test <expr> — cierto o falso: -e -f -d -s -z -n, = != -eq -lt -ge…") { ctx in
            var a = ctx.args
            if ctx.name == "[" {
                guard a.last == "]" else { throw ShErr("[: falta el ']' final") }
                a.removeLast()
            }
            var negado = false
            while a.first == "!" { negado.toggle(); a.removeFirst() }
            var ok = try Bash.evalTest(a, ctx)
            if negado { ok = !ok }
            if !ok { throw ShErr("") }      // sin texto: solo marca el código de salida
            return ""
        }
        c["["] = c["test"]

        c["sh"] = Spec(help: "sh -c '<órdenes>' | sh <guion> [args…] — con if, for, while y funciones") { ctx in
            if ctx.args.first == "-c" {
                return await Bash.correr([ctx.args.dropFirst().joined(separator: " ")], ctx.sh)
            }
            guard let p = ctx.args.first else { throw ShErr("sh: falta el guion o -c") }
            let lineas = try ctx.lines([p])
            let guardados = ctx.env.positional
            let g0 = ctx.env.vars["0"]
            ctx.env.positional = Array(ctx.args.dropFirst())
            ctx.env.vars["0"] = p
            let out = await Bash.correr(lineas, ctx.sh)
            ctx.env.positional = guardados
            ctx.env.vars["0"] = g0
            return out
        }
        c["bash"] = c["sh"]

        c["source"] = Spec(help: "source <guion> [args…] — lo ejecuta en este mismo shell") { ctx in
            guard let p = ctx.args.first else { throw ShErr("source: falta el guion") }
            let lineas = try ctx.lines([p])
            if ctx.args.count > 1 {
                let guardados = ctx.env.positional
                ctx.env.positional = Array(ctx.args.dropFirst())
                let out = await Bash.correr(lineas, ctx.sh)
                ctx.env.positional = guardados
                return out
            }
            return await Bash.correr(lineas, ctx.sh)
        }
        c["."] = c["source"]

        c["shift"] = Spec(help: "shift [n] — descarta los primeros argumentos ($1, $2…)") { ctx in
            let n = Int(ctx.args.first ?? "1") ?? 1
            guard n >= 0, n <= ctx.env.positional.count else { throw ShErr("shift: no hay tantos argumentos") }
            ctx.env.positional.removeFirst(n)
            return ""
        }

        c["read"] = Spec(help: "read NOMBRE… — reparte la primera línea de la entrada en variables") { ctx in
            let linea = ctx.stdin.components(separatedBy: "\n").first ?? ""
            let nombres = ctx.args.filter { !$0.hasPrefix("-") }
            guard !nombres.isEmpty else { ctx.env.vars["REPLY"] = linea; return "" }
            let trozos = linea.split(separator: " ", omittingEmptySubsequences: true).map(String.init)
            for (k, n) in nombres.enumerated() {
                if k == nombres.count - 1, trozos.count > nombres.count {
                    ctx.env.vars[n] = trozos[k...].joined(separator: " ")
                } else {
                    ctx.env.vars[n] = k < trozos.count ? trozos[k] : ""
                }
            }
            return ""
        }

        c["functions"] = Spec(help: "functions — enseña las funciones definidas") { ctx in
            let f = ctx.env.funcs
            guard !f.isEmpty else { return "no hay funciones definidas\n" }
            return f.keys.sorted().map { k in
                "\(k)() {\n" + (f[k] ?? []).map { "    " + $0 }.joined(separator: "\n") + "\n}"
            }.joined(separator: "\n") + "\n"
        }

        c["unfunction"] = Spec(help: "unfunction <nombre…> — borra funciones") { ctx in
            guard !ctx.args.isEmpty else { throw ShErr("unfunction: falta el nombre") }
            for a in ctx.args { ctx.env.funcs.removeValue(forKey: a) }
            return ""
        }

        c["type"] = Spec(help: "type <nombre> — dice si es alias, función, comando o paquete") { ctx in
            guard let n = ctx.args.first else { throw ShErr("type: falta el nombre") }
            if let a = ctx.env.aliases[n] { return "\(n) es un alias de '\(a)'\n" }
            if ctx.env.funcs[n] != nil { return "\(n) es una función\n" }
            if ctx.sh.commands[n] != nil { return "\(n) es un comando de la terminal\n" }
            if ctx.sh.installedSpec(n) != nil { return "\(n) es un paquete de \(Shell.binDir)\n" }
            if Shell.simVerbos.contains(n.lowercased()) { return "\(n) es un verbo de Simulacro\n" }
            throw ShErr("\(n): no encontrado")
        }

        c["jobs"] = Spec(help: "jobs — trabajos lanzados con '&'") { ctx in
            let js = ctx.sh.bgJobs
            guard !js.isEmpty else { return "no hay trabajos en segundo plano\n" }
            return js.map { "[\($0.id)]  \($0.linea)" }.joined(separator: "\n") +
                   "\nusa 'wait' para recoger su salida\n"
        }

        c["wait"] = Spec(help: "wait [n] — espera a un trabajo de segundo plano y enseña su salida") { ctx in
            let objetivo = ctx.args.first.flatMap { Int($0.replacingOccurrences(of: "%", with: "")) }
            var out = ""
            var quedan: [Shell.Trabajo] = []
            for t in ctx.sh.bgJobs {
                if let o = objetivo, o != t.id { quedan.append(t); continue }
                let r = await t.task.value
                out += "[\(t.id)] \(t.linea)\n" + r
            }
            ctx.sh.bgJobs = quedan
            return out.isEmpty ? "no hay nada que esperar\n" : out
        }
        c["fg"] = c["wait"]

        return c
    }
}
