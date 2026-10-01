import Foundation
#if (canImport(AVFoundation) && APPLE_COMPLETO)
import AVFoundation
#endif

// MARK: - Contexto de comando
// ============================================================

struct Ctx {
    let name: String
    let args: [String]
    let stdin: String
    let sh: Shell
    var env: ShellEnv { sh.env }

    func input(_ paths: [String]) throws -> String {
        if paths.isEmpty { return stdin }
        var s = ""
        for p in paths {
            let u = try env.resolve(p)
            guard env.exists(u) else { throw ShErr("\(name): \(p): no existe") }
            guard let d = FileManager.default.contents(atPath: u.path) else { throw ShErr("\(name): \(p): no se puede leer") }
            s += String(data: d, encoding: .utf8) ?? ""
        }
        return s
    }

    func lines(_ paths: [String]) throws -> [String] {
        let t = try input(paths)
        var l = t.components(separatedBy: "\n")
        if l.last == "" { l.removeLast() }
        return l
    }
}

struct Spec {
    let help: String
    let run: (Ctx) async throws -> String
}

func opts(_ args: [String], valued: Set<String> = []) -> (flags: Set<String>, vals: [String: String], rest: [String]) {
    var flags = Set<String>()
    var vals = [String: String]()
    var rest = [String]()
    var i = 0
    while i < args.count {
        let a = args[i]
        if a == "--" { rest.append(contentsOf: args[(i + 1)...]); break }
        if a.hasPrefix("--"), a.count > 2 {
            let name = String(a.dropFirst(2))
            if valued.contains(name), i + 1 < args.count { vals[name] = args[i + 1]; i += 2; continue }
            flags.insert(name); i += 1; continue
        }
        if a.hasPrefix("-"), a.count > 1, !a.dropFirst().allSatisfy({ $0.isNumber }) {
            let chars = Array(a.dropFirst())
            var j = 0
            while j < chars.count {
                let k = String(chars[j])
                if valued.contains(k) {
                    let rem = String(chars[(j + 1)...])
                    if !rem.isEmpty { vals[k] = rem }
                    else if i + 1 < args.count { vals[k] = args[i + 1]; i += 1 }
                    j = chars.count
                } else { flags.insert(k); j += 1 }
            }
            i += 1; continue
        }
        rest.append(a); i += 1
    }
    return (flags, vals, rest)
}

func humanSize(_ n: Int) -> String {
    let units = ["B", "K", "M", "G", "T"]
    var v = Double(n), i = 0
    while v >= 1024, i < units.count - 1 { v /= 1024; i += 1 }
    return i == 0 ? "\(n)B" : String(format: "%.1f%@", v, units[i])
}

// ============================================================
// MARK: - El shell
// ============================================================

final class Shell: @unchecked Sendable {
    let env: ShellEnv
    /// Si esta shell es la de una IA, cuál. nil = la del humano.
    let rolIA: RolMental?
    let js = JSRuntime()
    #if (canImport(AVFoundation) && APPLE_COMPLETO)
    let speech = AVSpeechSynthesizer()   // para 'say': si se crea una nueva cada vez, se corta a medias
    #endif
    var commands: [String: Spec] = [:]
    var adivinaEstado: AdivinaState?     // partida en marcha del juego 'adivina' (48 reglas)

    // Ganchos de la terminal de texto (solo texto: no hay ventanas)
    var uiClear: (@Sendable () -> Void)?
    var uiType: (@Sendable (String) -> Void)?
    /// Abre el editor de líneas (34_Editor.swift). Las shells de las IAs
    /// no lo tienen: ellas escriben con echo, cat > y sed.
    var uiEdit: (@Sendable (URL) -> Void)?

    /// Editor de texto abierto (nano/vi/edit): las líneas van a él.
    var editor: EdicionTexto?
    /// Lo que el editor quiere mostrar al abrirse.
    var avisoEditor: String?
    /// 'ia entra <rol>': la terminal habla con la shell de esa IA.
    var dentroDe: Shell?

    /// Idioma activo del intérprete de la terminal.
    enum Lang: String, CaseIterable {
        case shell, js, swift, json, xml, plist
    }
    var mode: Lang = .shell
    var buffer: [String] = []

    /// Bloque if/for/while/case a medio escribir (varias líneas)
    var blockBuffer: [String] = []
    /// <<FIN a medio leer: la orden, el delimitador y las líneas leídas
    var heredocPend: (linea: String, fin: String, cuerpo: [String])?
    /// Texto ya leído de un <<FIN, a la espera de entrar por la entrada estándar
    var hereText: String?

    /// Trabajos lanzados con '&'
    struct Trabajo { let id: Int; let linea: String; let task: Task<String, Never> }
    var bgJobs: [Trabajo] = []
    var bgNext = 1

    init(raiz: URL? = nil, usuario: String = "mobile", rolIA: RolMental? = nil) {
        env = ShellEnv(raiz: raiz, usuario: usuario)
        self.rolIA = rolIA
        // El registro se arma con un bucle y no con una cadena de
        // '.merging(...)': esa cadena era tan larga que el compilador se
        // rendía ("unable to type-check this expression in reasonable
        // time") y Playgrounds solo decía que la compilación falló.
        var cmds = Shell.core()
        // estos NO pisan lo que ya existe
        let base: [[String: Spec]] = [Shell.text(), Shell.data(), Shell.net(), Shell.lang()]
        for m in base { cmds.merge(m) { a, _ in a } }
        // el resto sí pisa: el último en registrarse gana.
        // Al final, las IAs propias: Huella (aprende viendo) y Nyx (18 mentes)
        let modulos: [[String: Spec]] = [
            Shell.extras(), Shell.ish(), Shell.more(), Shell.packages(), Shell.deb(),
            Shell.shellPlus(), Shell.plantillasCmd(), Shell.apk(), Shell.simulacro(), Shell.apis(),
            Shell.esc(), Shell.objetos(), Shell.mixCommands(), Shell.git(),
            Shell.macos(), Shell.logicGame(), Shell.unixMas(), Shell.bash(),
            Shell.huella(), Shell.nyx(), Shell.ias(), Shell.comandosIA(),
            Shell.apiLocal(), Shell.termux()
        ]
        for m in modulos { cmds.merge(m) { _, b in b } }
        commands = cmds
        js.envRef = env
        loadHistory()
        // El editor de texto es parte de la shell (no de una vista). Solo
        // la del humano lo usa: una IA no teclea línea a línea.
        if rolIA == nil {
            uiEdit = { [weak self] url in self?.abrirEditor(url) }
        }
    }

    var prompt: String {
        if let d = dentroDe { return d.prompt }
        if let e = editor { return "\(e.nombre) (:q sale)·\(e.lineas.count + 1)> " }
        if let r = rolIA, mode == .shell, heredocPend == nil, blockBuffer.isEmpty {
            return "\(r.rawValue)@ia:\(env.vpath(env.cwd)) $ "
        }
        // PS1 como en bash/Termux: PS1='\u@\h:\w \$ '
        if mode == .shell, heredocPend == nil, blockBuffer.isEmpty, let ps1 = env.vars["PS1"], !ps1.isEmpty {
            return Shell.expandirPS1(ps1, env)
        }
        if heredocPend != nil || !blockBuffer.isEmpty { return "> " }
        if mode == .shell { return "\(env.vpath(env.cwd)) $ " }
        return buffer.isEmpty ? "\(mode.rawValue)> " : "\(mode.rawValue)… "
    }

    /// ¿Están cerradas todas las llaves, corchetes y paréntesis?
    static func balanced(_ s: String) -> Bool {
        var depth = 0
        var inStr: Character? = nil
        var prev: Character = " "
        for ch in s {
            if let q = inStr {
                if ch == q && prev != "\\" { inStr = nil }
            } else if ch == "\"" || ch == "'" || ch == "`" {
                inStr = ch
            } else if "{[(".contains(ch) { depth += 1 }
            else if "}])".contains(ch) { depth -= 1 }
            prev = ch
        }
        return depth <= 0
    }

    /// Adivina en qué idioma está escrito un fragmento suelto.
    static func detect(_ code: String) -> Lang? {
        let t = code.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !t.isEmpty else { return nil }
        if t.hasPrefix("{") || t.hasPrefix("[") {
            if let d = t.data(using: .utf8),
               (try? JSONSerialization.jsonObject(with: d, options: [.fragmentsAllowed])) != nil { return .json }
        }
        if t.hasPrefix("<?xml") || t.hasPrefix("<") { return .xml }
        let swiftMarks = ["func ", "print(", " in ", "-> ", "var ", "let ", "struct ", "if let "]
        let jsMarks = ["console.", "function ", "=>", "const ", "document.", "http.", "JSON.", "Math.",
                       "readFile(", "writeFile(", "listDir(", "fetchJSON(", "fetchText("]
        let isJS = jsMarks.contains { t.contains($0) }
        let isSwift = swiftMarks.contains { t.contains($0) }
        if isJS { return .js }
        if isSwift { return .swift }
        // expresión aritmética suelta: 2+2, (3*4)/2
        if t.rangeOfCharacter(from: CharacterSet(charactersIn: "+*/=()")) != nil,
           t.rangeOfCharacter(from: .decimalDigits) != nil { return .js }
        return nil
    }

    /// Ejecuta un fragmento en el idioma indicado.
    func runCode(_ code: String, lang: Lang) async -> String {
        do {
            switch lang {
            case .shell: return await execute(code)
            case .js: return try js.eval(code)
            case .swift: return try js.eval(SwiftJS.transpile(code))
            case .json: return try DataTools.jsonPretty(code) + "\n"
            case .xml:
                guard let d = code.data(using: .utf8) else { throw ShErr("xml: texto no válido") }
                return try XMLTreeBuilder().parse(d).pretty() + "\n"
            case .plist:
                return try DataTools.plistShow(Data(code.utf8)) + "\n"
            }
        } catch {
            return errText(error) + "\n"
        }
    }

    func execute(_ line: String) async -> String {
        var trimmed = line.trimmingCharacters(in: .whitespaces)

        // --- dentro de la shell de una IA ('ia entra <rol>') ---
        if let d = dentroDe {
            if ["salir", "exit", "logout"].contains(trimmed) {
                dentroDe = nil
                return "de vuelta a tu shell\n"
            }
            return await SistemaIAs.uno.ejecutar(line, en: d, por: "humano")
        }

        // --- editor de texto abierto ---
        if editor != nil { return await lineaEditor(line) }

        // --- 'ia <rol> …': la línea entera (con >, |, $VAR…) es para SU shell ---
        if rolIA == nil, trimmed.hasPrefix("ia "), let r = await SistemaIAs.uno.lineaCruda(trimmed, self) {
            return r
        }

        // --- dentro de un modo de idioma: todo lo escrito es código ---
        if mode != .shell {
            if ["exit", ".exit", "quit", "shell", ":q"].contains(trimmed) {
                mode = .shell; buffer = []
                return "de vuelta al shell\n"
            }
            if trimmed == ".cancel" { buffer = []; return "" }
            buffer.append(line)
            let joined = buffer.joined(separator: "\n")
            if !Shell.balanced(joined) { return "" }   // sigue leyendo líneas
            buffer = []
            return await runCode(joined, lang: mode)
        }

        // --- seguimos leyendo el cuerpo de un <<FIN ---
        if var h = heredocPend {
            if trimmed == h.fin {
                heredocPend = nil
                hereText = h.cuerpo.isEmpty ? "" : h.cuerpo.joined(separator: "\n") + "\n"
                let r = await execute(h.linea)
                hereText = nil
                return r
            }
            h.cuerpo.append(line)
            heredocPend = h
            return ""
        }

        // --- seguimos leyendo un bloque if/for/while/case/función ---
        if !blockBuffer.isEmpty {
            blockBuffer.append(line)
            if Bash.abierto(blockBuffer) { return "" }
            let lineas = blockBuffer
            blockBuffer = []
            return await Bash.correr(lineas, self)
        }

        guard !trimmed.isEmpty else { return "" }

        // --- if / for / while / until / case / función ---
        if Bash.empieza(trimmed) {
            if Bash.abierto([trimmed]) { blockBuffer = [line]; return "" }
            return await Bash.correr([trimmed], self)
        }

        // --- !! y !n repiten órdenes del historial ---
        if let h = Shell.historyExpand(trimmed, env.history) { trimmed = h }

        // --- $(comando) se sustituye por su salida ---
        do { trimmed = try await substitute(trimmed) }
        catch { return Shell.errMark + errText(error) + "\n" }

        // --- ./archivo.js ejecuta el archivo ---
        if trimmed.hasPrefix("./") {
            return await execute("run " + String(trimmed.dropFirst(2)))
        }

        // --- sintaxis de Simulacro: verbo/valor/verbo, sin escribir 'sim' delante ---
        // la cabeza se limpia de '=valor' (API=1) y de '(n)' (Line(3)) para
        // comparar solo el verbo contra Shell.simVerbos
        //
        // Antes se exigía que la cabeza NO fuera un comando real, así que
        // 'Sort/notas.txt', 'Now/', 'Clip/hola', 'Calc/2*3'… daban "comando no
        // encontrado". Ahora decide la diagonal pegada a la primera palabra:
        //   sort notas.txt   → comando normal (primera palabra sin '/')
        //   Sort/notas.txt   → Simulacro
        //   cat /etc/hosts   → comando normal ('/' está en otra palabra)
        let primera = String(trimmed.split(separator: " ").first ?? "")
        let cabeza = String(primera.split(separator: "/").first ?? "")
            .components(separatedBy: "=").first?
            .components(separatedBy: "(").first?
            .lowercased() ?? ""
        if primera.contains("/"), !primera.hasPrefix("/"), Shell.simVerbos.contains(cabeza) {
            return await Shell.runSim(trimmed, Ctx(name: "sim", args: [], stdin: "", sh: self))
        }

        // --- código suelto en cualquier idioma, sin ser un comando ---
        let first = String(trimmed.split(separator: " ").first ?? "")
        if commands[first] == nil, env.aliases[first] == nil,
           !first.contains("="), !first.hasPrefix("$"),
           let lang = Shell.detect(trimmed) {
            if !Shell.balanced(trimmed) {
                mode = lang
                buffer = [line]
                return "[\(lang.rawValue)] bloque abierto — sigue escribiendo, '.cancel' para descartar\n"
            }
            let r = await runCode(trimmed, lang: lang)
            return r.isEmpty ? "" : "[\(lang.rawValue)] " + r
        }

        var out = ""
        do {
            let toks = try Parser.tokenize(trimmed, env)
            let jobs = try Parser.parse(toks, env)
            // ¿falta el cuerpo de un <<FIN? se pide en las líneas siguientes
            if let fin = Parser.heredocDelim(toks), hereText == nil {
                heredocPend = (trimmed, fin, [])
                return ""
            }
            var ok = true
            for job in jobs {
                if job.link == .and && !ok { continue }
                if job.link == .or && ok { continue }
                if job.background {
                    let n = lanzarEnFondo(job.cmds)
                    out += "[\(n)] en segundo plano\n"
                    ok = true
                    env.vars["?"] = "0"
                    continue
                }
                let r = await runPipeline(job.cmds)
                out += r.out + r.err
                ok = (r.status == 0)
                env.vars["?"] = String(r.status)
            }
            if jobs.isEmpty { env.vars["?"] = "0" }
        } catch {
            out += Shell.errMark + errText(error) + "\n"
            env.vars["?"] = "1"
        }
        env.vars["PWD"] = env.vpath(env.cwd)
        return out
    }

    /// Escribe una redirección, respetando /dev/null.
    private func escribeArchivo(_ texto: String, _ archivo: String, _ append: Bool) throws {
        if archivo == "/dev/null" { return }
        let u = try env.resolve(archivo)
        if append, let d = FileManager.default.contents(atPath: u.path) {
            let unido = (String(data: d, encoding: .utf8) ?? "") + texto
            try unido.write(to: u, atomically: true, encoding: .utf8)
        } else {
            try texto.write(to: u, atomically: true, encoding: .utf8)
        }
    }

    /// Ejecuta una tubería y devuelve salida, errores y código de salida.
    func runPipeline(_ cmds: [Cmd]) async -> (out: String, err: String, status: Int) {
        var data = ""
        var errAll = ""
        var status = 0

        for cmd in cmds {
            var argv = cmd.argv

            // VAR=valor sin comando detrás: se queda en el entorno
            if argv.isEmpty {
                for (k, v) in cmd.assigns { env.vars[k] = v }
                status = 0
                continue
            }

            // alias (una vuelta)
            if let a = env.aliases[argv[0]] {
                let expanded = ((try? Parser.tokenize(a, env)) ?? []).compactMap { t -> String? in
                    if case .word(let w, _) = t { return w }
                    return nil
                }
                argv = expanded + Array(argv.dropFirst())
            }
            guard !argv.isEmpty else { continue }
            let name = argv[0]

            // VAR=valor delante del comando: solo mientras dura el comando
            var previas: [(String, String?)] = []
            for (k, v) in cmd.assigns { previas.append((k, env.vars[k])); env.vars[k] = v }
            func restaura() {
                for (k, v) in previas {
                    if let v { env.vars[k] = v } else { env.vars.removeValue(forKey: k) }
                }
            }

            // entrada: <<<texto, <<FIN, < archivo, o lo que traiga la tubería
            var stdin = data
            if let h = cmd.here { stdin = h }
            else if cmd.hereDelim != nil { stdin = hereText ?? "" }
            else if let f = cmd.inFile {
                if f == "/dev/null" { stdin = "" }
                else if let u = try? env.resolve(f), let d = FileManager.default.contents(atPath: u.path) {
                    stdin = String(data: d, encoding: .utf8) ?? ""
                } else {
                    errAll += Shell.errMark + "\(f): no se puede leer\n"
                    status = 1
                    restaura()
                    continue
                }
            }

            var salida = ""
            var fallo: String? = nil

            if let cuerpo = env.funcs[name] {
                // función definida por el usuario: sus argumentos son $1, $2…
                let guardados = env.positional
                let guardado0 = env.vars["0"]
                env.positional = Array(argv.dropFirst())
                env.vars["0"] = name
                salida = await Bash.correr(cuerpo, self)
                env.positional = guardados
                env.vars["0"] = guardado0
                if (env.vars["?"] ?? "0") != "0" { fallo = "" }
            } else if let spec = commands[name] ?? installedSpec(name) {
                let ctx = Ctx(name: name, args: Array(argv.dropFirst()), stdin: stdin, sh: self)
                do { salida = try await spec.run(ctx) }
                catch { fallo = errText(error) }
                if let a = avisoEditor { salida += a; avisoEditor = nil }
            } else {
                fallo = "\(name): comando no encontrado (escribe 'help' o 'pkg search')"
            }

            restaura()

            if cmd.negate {
                let fueBien = (fallo == nil)
                status = fueBien ? 1 : 0
                fallo = nil
            } else {
                status = (fallo == nil) ? 0 : 1
            }

            // los errores van a 2>, se mezclan con 2>&1, o se enseñan
            if let m = fallo, !m.isEmpty {
                if cmd.errToOut { salida += m + "\n" }
                else if let f = cmd.errFile { try? escribeArchivo(m + "\n", f, cmd.errAppend) }
                else { errAll += Shell.errMark + m + "\n" }
            } else if fallo != nil, let f = cmd.errFile {
                try? escribeArchivo("", f, cmd.errAppend)
            }

            if cmd.outToErr { errAll += salida; salida = "" }

            if let f = cmd.outFile {
                do { try escribeArchivo(salida, f, cmd.append) }
                catch { errAll += Shell.errMark + errText(error) + "\n"; status = 1 }
                data = ""
            } else {
                data = salida
            }
        }
        return (data, errAll, status)
    }

    /// Lanza una tubería con '&' y devuelve su número de trabajo.
    func lanzarEnFondo(_ cmds: [Cmd]) -> Int {
        let id = bgNext
        bgNext += 1
        let linea = cmds.map { $0.argv.joined(separator: " ") }.joined(separator: " | ")
        let t = Task<String, Never> { [weak self] in
            guard let self else { return "" }
            let r = await self.runPipeline(cmds)
            return r.out + r.err
        }
        bgJobs.append(Trabajo(id: id, linea: linea, task: t))
        env.vars["!"] = String(id)
        return id
    }

    /// Autocompletado para la tecla Tab.
    func complete(_ line: String) -> String {
        let parts = line.split(separator: " ", omittingEmptySubsequences: false).map(String.init)
        guard let last = parts.last else { return line }
        var candidates: [String] = []
        if parts.count == 1 {
            var names = Set(commands.keys)
            if let bin = try? env.resolve(Shell.binDir),
               let items = try? FileManager.default.contentsOfDirectory(atPath: bin.path) {
                for i in items { names.insert((i as NSString).deletingPathExtension) }
            }
            candidates = names.filter { $0.hasPrefix(last) }.sorted()
        } else {
            let slash = last.lastIndex(of: "/")
            let dir = slash.map { String(last[last.startIndex..<$0]) } ?? ""
            let stem = slash.map { String(last[last.index(after: $0)...]) } ?? last
            if let u = try? env.resolve(dir.isEmpty ? "." : dir),
               let items = try? FileManager.default.contentsOfDirectory(atPath: u.path) {
                candidates = items.filter { $0.hasPrefix(stem) }.sorted().map { dir.isEmpty ? $0 : dir + "/" + $0 }
            }
        }
        guard let first = candidates.first else { return line }
        var common = first
        for c in candidates.dropFirst() {
            common = String(zip(common, c).prefix { $0.0 == $0.1 }.map { $0.0 })
        }
        var newParts = parts
        newParts[newParts.count - 1] = common
        var result = newParts.joined(separator: " ")
        if candidates.count == 1 {
            let isDir = (try? env.resolve(common)).map { env.isDir($0) } ?? false
            result += isDir ? "/" : " "
        }
        return result
    }
}
