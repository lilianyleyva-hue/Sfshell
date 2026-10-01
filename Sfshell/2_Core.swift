import Foundation

// ============================================================
// MARK: - Errores
// ============================================================

struct ShErr: Error, LocalizedError {
    let m: String
    init(_ m: String) { self.m = m }
    var errorDescription: String? { m }
}

func errText(_ e: Error) -> String {
    if let s = e as? ShErr { return s.m }
    return (e as NSError).localizedDescription
}

// ============================================================
// MARK: - Entorno y sistema de archivos virtual
// ============================================================

final class ShellEnv: @unchecked Sendable {
    let root: URL
    var cwd: URL
    var vars: [String: String] = [:]
    var aliases: [String: String] = [:]
    var history: [String] = []
    /// $1, $2… dentro de guiones y funciones
    var positional: [String] = []
    /// funciones definidas con  nombre() { … }
    var funcs: [String: [String]] = [:]

    init() {
        let docs = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0]
        let base = docs.appendingPathComponent("shell", isDirectory: true)
        try? FileManager.default.createDirectory(at: base, withIntermediateDirectories: true)
        root = base.standardizedFileURL
        cwd = root
        vars = ["HOME": "/", "USER": "mobile", "SHELL": "swiftsh", "PWD": "/", "TERM": "swiftshell"]
        aliases = ["ll": "ls -l", "la": "ls -a", "node": "js", "..": "cd ..", "dir": "ls"]
    }

    /// Ruta virtual (lo que ve el usuario) para una URL real.
    func vpath(_ url: URL) -> String {
        let r = root.standardizedFileURL.path
        var p = url.standardizedFileURL.path
        if p.hasPrefix(r) { p.removeFirst(r.count) }
        return p.isEmpty ? "/" : p
    }

    /// Convierte una ruta virtual en URL real, sin salir del sandbox.
    func resolve(_ path: String) throws -> URL {
        var p = path
        if p == "~" { p = "/" }
        if p.hasPrefix("~/") { p = String(p.dropFirst(1)) }
        let absolute = p.hasPrefix("/")
        var url = absolute ? root : cwd
        for comp in p.split(separator: "/") {
            if comp == "." { continue }
            if comp == ".." {
                if url.standardizedFileURL.path != root.path { url.deleteLastPathComponent() }
                continue
            }
            url.appendPathComponent(String(comp))
        }
        url = url.standardizedFileURL
        guard url.path == root.path || url.path.hasPrefix(root.path + "/") else {
            throw ShErr("permiso denegado: fuera del área de trabajo")
        }
        return url
    }

    func exists(_ u: URL) -> Bool { FileManager.default.fileExists(atPath: u.path) }

    func isDir(_ u: URL) -> Bool {
        var d: ObjCBool = false
        let e = FileManager.default.fileExists(atPath: u.path, isDirectory: &d)
        return e && d.boolValue
    }
}

// ============================================================
// MARK: - Análisis de la línea de comandos
// ============================================================

enum Tok {
    case word(String, Bool)   // texto, entrecomillado
    case op(String)           // | ; && || > >> < << <<< 2> 2>> 2>&1 >&2 &> &
}

struct Cmd {
    var argv: [String] = []
    var assigns: [(String, String)] = []   // VAR=valor delante del comando
    var inFile: String? = nil
    var here: String? = nil                // texto de <<<  o de un <<FIN ya leído
    var hereDelim: String? = nil           // <<FIN pendiente de leer
    var outFile: String? = nil
    var append = false
    var errFile: String? = nil
    var errAppend = false
    var errToOut = false                   // 2>&1
    var outToErr = false                   // >&2
    var negate = false                     // ! comando
}

enum Link { case seq, and, or }

struct Job {
    var link: Link = .seq
    var cmds: [Cmd] = []
    var background = false                 // la línea terminaba en &
}

enum Parser {

    static func unescape(_ c: Character) -> Character {
        switch c {
        case "n": return "\n"
        case "t": return "\t"
        case "0": return "\0"
        default: return c
        }
    }

    // --------------------------------------------------------
    // Variables:  $NOMBRE  ${NOMBRE}  ${NOMBRE:-valor}
    //             $?  $#  $@  $*  $$  $!  $0  $1…  $RANDOM
    // --------------------------------------------------------
    static func valorVar(_ name: String, _ env: ShellEnv) -> String {
        if name == "RANDOM", env.vars["RANDOM"] == nil { return String(Int.random(in: 0...32767)) }
        if let n = Int(name) {
            if n == 0 { return env.vars["0"] ?? "swiftsh" }
            return n <= env.positional.count ? env.positional[n - 1] : ""
        }
        switch name {
        case "?": return env.vars["?"] ?? "0"
        case "#": return String(env.positional.count)
        case "@", "*": return env.positional.joined(separator: " ")
        case "$": return env.vars["$"] ?? "1"
        case "!": return env.vars["!"] ?? ""
        default: return env.vars[name] ?? ""
        }
    }

    static func readVar(_ s: String, _ start: String.Index, _ env: ShellEnv) -> (String, String.Index) {
        var i = start
        guard i < s.endIndex else { return ("$", i) }

        // ${NOMBRE}, ${NOMBRE:-porDefecto}, ${NOMBRE:+siExiste}, ${#NOMBRE}
        if s[i] == "{" {
            i = s.index(after: i)
            var cuerpo = ""
            while i < s.endIndex, s[i] != "}" { cuerpo.append(s[i]); i = s.index(after: i) }
            if i < s.endIndex { i = s.index(after: i) }
            if cuerpo.hasPrefix("#"), cuerpo.count > 1 {
                return (String(valorVar(String(cuerpo.dropFirst()), env).count), i)
            }
            if let r = cuerpo.range(of: ":-") {
                let v = valorVar(String(cuerpo[cuerpo.startIndex..<r.lowerBound]), env)
                return (v.isEmpty ? String(cuerpo[r.upperBound...]) : v, i)
            }
            if let r = cuerpo.range(of: ":+") {
                let v = valorVar(String(cuerpo[cuerpo.startIndex..<r.lowerBound]), env)
                return (v.isEmpty ? "" : String(cuerpo[r.upperBound...]), i)
            }
            return (valorVar(cuerpo, env), i)
        }

        // variables especiales de un solo carácter
        let c = s[i]
        if "?#@*$!".contains(c) {
            i = s.index(after: i)
            return (valorVar(String(c), env), i)
        }

        // parámetros posicionales: $1, $2, $12…
        if c.isNumber {
            var num = ""
            while i < s.endIndex, s[i].isNumber { num.append(s[i]); i = s.index(after: i) }
            return (valorVar(num, env), i)
        }

        var name = ""
        while i < s.endIndex, s[i].isLetter || s[i].isNumber || s[i] == "_" {
            name.append(s[i]); i = s.index(after: i)
        }
        if name.isEmpty { return ("$", i) }
        return (valorVar(name, env), i)
    }

    // --------------------------------------------------------
    // Troceado
    // --------------------------------------------------------
    static func tokenize(_ s: String, _ env: ShellEnv) throws -> [Tok] {
        var toks: [Tok] = []
        var cur = ""
        var has = false
        var quoted = false
        var i = s.startIndex

        func flush() {
            if has { toks.append(.word(cur, quoted)); cur = ""; has = false; quoted = false }
        }

        while i < s.endIndex {
            let c = s[i]

            if c == " " || c == "\t" || c == "\n" { flush(); i = s.index(after: i); continue }

            if c == "#" && !has { break }

            if c == "'" {
                quoted = true; has = true
                i = s.index(after: i)
                while i < s.endIndex, s[i] != "'" { cur.append(s[i]); i = s.index(after: i) }
                guard i < s.endIndex else { throw ShErr("comilla ' sin cerrar") }
                i = s.index(after: i); continue
            }

            if c == "\"" {
                quoted = true; has = true
                i = s.index(after: i)
                while i < s.endIndex, s[i] != "\"" {
                    if s[i] == "\\" {
                        let n = s.index(after: i)
                        if n < s.endIndex { cur.append(unescape(s[n])); i = s.index(after: n); continue }
                    }
                    if s[i] == "$" {
                        let (v, ni) = readVar(s, s.index(after: i), env)
                        cur += v; i = ni; continue
                    }
                    cur.append(s[i]); i = s.index(after: i)
                }
                guard i < s.endIndex else { throw ShErr("comilla \" sin cerrar") }
                i = s.index(after: i); continue
            }

            if c == "\\" {
                let n = s.index(after: i)
                if n < s.endIndex {
                    cur.append(s[n]); has = true; quoted = true
                    i = s.index(after: n); continue
                }
            }

            if c == "$" {
                let (v, ni) = readVar(s, s.index(after: i), env)
                cur += v; has = true; i = ni; continue
            }

            if "|;&<>".contains(c) {

                // 2> 2>> 2>&1 1> 1>&2 — el descriptor está pegado delante
                if c == ">", !quoted, cur == "1" || cur == "2" {
                    let fd = cur
                    cur = ""; has = false; quoted = false
                    var j = s.index(after: i)
                    var op = fd == "2" ? "2>" : ">"
                    if j < s.endIndex, s[j] == ">" {
                        op = fd == "2" ? "2>>" : ">>"
                        j = s.index(after: j)
                    } else if j < s.endIndex, s[j] == "&" {
                        let k = s.index(after: j)
                        if k < s.endIndex, s[k] == "1" { op = fd == "2" ? "2>&1" : ">&1"; j = s.index(after: k) }
                        else if k < s.endIndex, s[k] == "2" { op = fd == "2" ? "2>&2" : ">&2"; j = s.index(after: k) }
                    }
                    toks.append(.op(op)); i = j; continue
                }

                flush()
                let n = s.index(after: i)

                if c == "&" {
                    if n < s.endIndex, s[n] == "&" { toks.append(.op("&&")); i = s.index(after: n); continue }
                    if n < s.endIndex, s[n] == ">" {
                        var j = s.index(after: n)
                        if j < s.endIndex, s[j] == ">" { j = s.index(after: j) }
                        toks.append(.op("&>")); i = j; continue
                    }
                    toks.append(.op("&")); i = n; continue      // segundo plano
                }

                if c == "|" {
                    if n < s.endIndex, s[n] == "|" { toks.append(.op("||")); i = s.index(after: n); continue }
                    toks.append(.op("|")); i = n; continue
                }

                if c == ">" {
                    if n < s.endIndex, s[n] == ">" { toks.append(.op(">>")); i = s.index(after: n); continue }
                    if n < s.endIndex, s[n] == "&" {
                        let k = s.index(after: n)
                        if k < s.endIndex, s[k] == "2" { toks.append(.op(">&2")); i = s.index(after: k); continue }
                        if k < s.endIndex, s[k] == "1" { toks.append(.op(">&1")); i = s.index(after: k); continue }
                    }
                    toks.append(.op(">")); i = n; continue
                }

                if c == "<" {
                    if n < s.endIndex, s[n] == "<" {
                        let k = s.index(after: n)
                        if k < s.endIndex, s[k] == "<" { toks.append(.op("<<<")); i = s.index(after: k); continue }
                        toks.append(.op("<<")); i = k; continue
                    }
                    toks.append(.op("<")); i = n; continue
                }

                toks.append(.op(String(c))); i = n; continue
            }

            cur.append(c); has = true; i = s.index(after: i)
        }
        flush()
        return toks
    }

    // --------------------------------------------------------
    // Llaves:  {a,b}  {1..5}  {a..e}  {1..9..2}
    // --------------------------------------------------------
    static func braces(_ w: String) -> [String] {
        let ch = Array(w)
        // primera llave abierta y su pareja
        var abre = -1, cierra = -1, prof = 0
        for (k, c) in ch.enumerated() {
            if c == "{" { if prof == 0 { abre = k }; prof += 1 }
            else if c == "}" {
                prof -= 1
                if prof == 0 { cierra = k; break }
                if prof < 0 { prof = 0 }
            }
        }
        guard abre >= 0, cierra > abre else { return [w] }
        let pre = String(ch[0..<abre])
        let post = String(ch[(cierra + 1)...])
        let dentro = String(ch[(abre + 1)..<cierra])

        var opciones: [String] = []

        // rango  {1..5}  {a..e}  {1..9..2}
        let partes = dentro.components(separatedBy: "..")
        if partes.count == 2 || partes.count == 3 {
            let paso = partes.count == 3 ? max(1, abs(Int(partes[2]) ?? 1)) : 1
            if let a = Int(partes[0]), let b = Int(partes[1]) {
                var v = a
                while (a <= b && v <= b) || (a > b && v >= b) {
                    opciones.append(String(v))
                    v += a <= b ? paso : -paso
                    if opciones.count > 1000 { break }
                }
            } else if partes[0].count == 1, partes[1].count == 1,
                      let a = partes[0].unicodeScalars.first?.value,
                      let b = partes[1].unicodeScalars.first?.value {
                var v = Int(a)
                let fin = Int(b)
                while (a <= b && v <= fin) || (a > b && v >= fin) {
                    if let u = Unicode.Scalar(UInt32(v)) { opciones.append(String(Character(u))) }
                    v += a <= b ? paso : -paso
                    if opciones.count > 1000 { break }
                }
            }
        }

        // lista  {a,b,c}  (respetando llaves anidadas)
        if opciones.isEmpty {
            var actual = ""
            var nivel = 0
            for c in dentro {
                if c == "{" { nivel += 1 }
                if c == "}" { nivel -= 1 }
                if c == ",", nivel == 0 { opciones.append(actual); actual = ""; continue }
                actual.append(c)
            }
            opciones.append(actual)
            if opciones.count < 2 { return [w] }   // {solo} se queda tal cual
        }

        var salida: [String] = []
        for o in opciones { salida += braces(pre + o + post) }
        return salida
    }

    // --------------------------------------------------------
    // Comodines:  *  ?  [abc]  [a-z]  [!abc]
    // --------------------------------------------------------
    static func match(_ p: [Character], _ n: [Character]) -> Bool {
        func go(_ pi: Int, _ ni: Int) -> Bool {
            if pi == p.count { return ni == n.count }
            let c = p[pi]
            if c == "*" {
                var k = ni
                while true {
                    if go(pi + 1, k) { return true }
                    if k == n.count { return false }
                    k += 1
                }
            }
            if ni == n.count { return false }
            if c == "?" { return go(pi + 1, ni + 1) }
            if c == "[" {
                var j = pi + 1
                var negado = false
                if j < p.count, p[j] == "!" || p[j] == "^" { negado = true; j += 1 }
                var sueltos: [Character] = []
                var rangos: [(Character, Character)] = []
                var cerrado = false
                var primero = true
                while j < p.count {
                    if p[j] == "]", !primero { cerrado = true; break }
                    if j + 2 < p.count, p[j + 1] == "-", p[j + 2] != "]" {
                        rangos.append((p[j], p[j + 2])); j += 3; primero = false; continue
                    }
                    sueltos.append(p[j]); j += 1; primero = false
                }
                guard cerrado else { return n[ni] == "[" && go(pi + 1, ni + 1) }
                let ch = n[ni]
                var dentro = sueltos.contains(ch) || rangos.contains { ch >= $0.0 && ch <= $0.1 }
                if negado { dentro = !dentro }
                return dentro && go(j + 1, ni + 1)
            }
            return c == n[ni] && go(pi + 1, ni + 1)
        }
        return go(0, 0)
    }

    static func esPatron(_ s: String) -> Bool {
        s.contains("*") || s.contains("?") || s.contains("[")
    }

    /// Comodines en cualquier tramo de la ruta (a*/b?.txt), sin tocar los ocultos
    /// salvo que el patrón empiece por punto, como en Linux.
    static func glob(_ word: String, _ env: ShellEnv) -> [String] {
        guard esPatron(word) else { return [word] }
        let absoluta = word.hasPrefix("/")
        let comps = word.split(separator: "/", omittingEmptySubsequences: true).map(String.init)
        guard !comps.isEmpty else { return [word] }

        func une(_ base: String, _ hoja: String) -> String {
            if base.isEmpty { return hoja }
            if base == "/" { return "/" + hoja }
            return base + "/" + hoja
        }

        var actuales: [String] = [absoluta ? "/" : ""]
        for comp in comps {
            var siguientes: [String] = []
            for base in actuales {
                if !esPatron(comp) { siguientes.append(une(base, comp)); continue }
                let dir = base.isEmpty ? "." : base
                guard let u = try? env.resolve(dir),
                      let items = try? FileManager.default.contentsOfDirectory(atPath: u.path) else { continue }
                let hits = items.filter { nombre in
                    if nombre.hasPrefix("."), !comp.hasPrefix(".") { return false }
                    return match(Array(comp), Array(nombre))
                }.sorted()
                for h in hits { siguientes.append(une(base, h)) }
            }
            actuales = siguientes
            if actuales.isEmpty { break }
        }
        return actuales.isEmpty ? [word] : actuales
    }

    /// ~ y ~/ apuntan a HOME, como en Linux.
    static func tilde(_ w: String, _ env: ShellEnv) -> String {
        let home = env.vars["HOME"] ?? "/"
        if w == "~" { return home }
        if w.hasPrefix("~/") { return home == "/" ? String(w.dropFirst(1)) : home + String(w.dropFirst(1)) }
        return w
    }

    /// Una palabra suelta → todas las palabras que genera (llaves + comodines).
    static func expandir(_ w: String, _ env: ShellEnv) -> [String] {
        var salida: [String] = []
        for b in braces(w) { salida += glob(tilde(b, env), env) }
        return salida.isEmpty ? [w] : salida
    }

    /// Texto → lista de palabras ya expandidas (lo usan 'for' y las funciones).
    static func palabras(_ s: String, _ env: ShellEnv) throws -> [String] {
        var salida: [String] = []
        for t in try tokenize(s, env) {
            if case .word(let w, let q) = t { salida += q ? [w] : expandir(w, env) }
        }
        return salida
    }

    static func esAsignacion(_ w: String) -> (String, String)? {
        guard let eq = w.firstIndex(of: "="), eq != w.startIndex else { return nil }
        let nombre = String(w[w.startIndex..<eq])
        guard let f = nombre.first, f.isLetter || f == "_" else { return nil }
        guard nombre.allSatisfy({ $0.isLetter || $0.isNumber || $0 == "_" }) else { return nil }
        return (nombre, String(w[w.index(after: eq)...]))
    }

    /// Delimitador de un <<FIN que aún no se ha leído (para pedir más líneas).
    static func heredocDelim(_ toks: [Tok]) -> String? {
        var esperando = false
        for t in toks {
            switch t {
            case .op(let o): esperando = (o == "<<")
            case .word(let w, _): if esperando { return w }
            }
        }
        return nil
    }

    // --------------------------------------------------------
    // Montaje de las órdenes
    // --------------------------------------------------------
    static func parse(_ toks: [Tok], _ env: ShellEnv) throws -> [Job] {
        var jobs: [Job] = []
        var job = Job()
        var cmd = Cmd()
        var pendiente: String? = nil

        func endCmd() {
            if !cmd.argv.isEmpty || cmd.outFile != nil || !cmd.assigns.isEmpty { job.cmds.append(cmd) }
            cmd = Cmd()
        }
        func endJob(_ next: Link, fondo: Bool = false) {
            endCmd()
            if !job.cmds.isEmpty { job.background = fondo; jobs.append(job) }
            job = Job(link: next, cmds: [])
        }

        for t in toks {
            switch t {
            case .word(let w, let q):
                if let r = pendiente {
                    switch r {
                    case "<": cmd.inFile = w
                    case "<<": cmd.hereDelim = w
                    case "<<<": cmd.here = w + "\n"
                    case "2>": cmd.errFile = w; cmd.errAppend = false
                    case "2>>": cmd.errFile = w; cmd.errAppend = true
                    case "&>": cmd.outFile = w; cmd.append = false; cmd.errToOut = true
                    default: cmd.outFile = w; cmd.append = (r == ">>")
                    }
                    pendiente = nil
                    continue
                }
                if cmd.argv.isEmpty, w == "!", !q { cmd.negate = true; continue }
                if cmd.argv.isEmpty, !q, let (n, v) = esAsignacion(w) {
                    cmd.assigns.append((n, v)); continue
                }
                cmd.argv.append(contentsOf: q ? [w] : expandir(w, env))

            case .op(let o):
                switch o {
                case "|": endCmd()
                case ";": endJob(.seq)
                case "&": endJob(.seq, fondo: true)
                case "&&": endJob(.and)
                case "||": endJob(.or)
                case "2>&1": cmd.errToOut = true
                case ">&2": cmd.outToErr = true
                case ">&1", "2>&2": break          // ya es lo normal
                case ">", ">>", "<", "<<", "<<<", "2>", "2>>", "&>": pendiente = o
                default: throw ShErr("operador no reconocido: \(o)")
                }
            }
        }
        if pendiente != nil { throw ShErr("falta el archivo después de la redirección") }
        endCmd()
        if !job.cmds.isEmpty { jobs.append(job) }
        return jobs
    }
}
