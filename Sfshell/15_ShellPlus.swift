import Foundation

// ============================================================
// MARK: - Mejoras del shell
// ============================================================
// 1. $(comando) se sustituye por su salida
// 2. !! y !n repiten órdenes anteriores
// 3. $? guarda si el último comando fue bien o mal
// 4. el historial sobrevive al cierre de la app
// 5. /etc/profile.sh se ejecuta al arrancar

extension Shell {

    /// Marca invisible con la que la interfaz sabe que una línea es un error.
    static let errMark = "\u{1}"

    static let historyFile = "/etc/history"
    static let profileFile = "/etc/profile.sh"

    // --- sustitución de comandos -------------------------------

    /// Reemplaza cada $(...) —y cada `...`— por la salida del comando de dentro.
    /// Lo que va entre comillas simples se deja tal cual, como en Linux.
    func substitute(_ line: String) async throws -> String {
        guard line.contains("$(") || line.contains("`") else { return line }
        var out = ""
        var i = line.startIndex
        var vueltas = 0

        func limpia(_ r: String) -> String {
            r.replacingOccurrences(of: Shell.errMark, with: "")
                .trimmingCharacters(in: .whitespacesAndNewlines)
                .replacingOccurrences(of: "\n", with: " ")
        }

        while i < line.endIndex {
            let c = line[i]

            // dentro de 'comillas simples' no se sustituye nada
            if c == "'" {
                out.append(c)
                i = line.index(after: i)
                while i < line.endIndex, line[i] != "'" { out.append(line[i]); i = line.index(after: i) }
                if i < line.endIndex { out.append("'"); i = line.index(after: i) }
                continue
            }

            // \$( y \` escapados
            if c == "\\", line.index(after: i) < line.endIndex {
                let n = line.index(after: i)
                if line[n] == "$" || line[n] == "`" {
                    out.append(line[n]); i = line.index(after: n); continue
                }
            }

            if c == "$", line.index(after: i) < line.endIndex, line[line.index(after: i)] == "(" {
                vueltas += 1
                if vueltas > 20 { throw ShErr("sustitución: demasiados $( ) anidados") }
                var depth = 1
                var inner = ""
                var j = line.index(i, offsetBy: 2)
                while j < line.endIndex {
                    if line[j] == "(" { depth += 1 }
                    if line[j] == ")" { depth -= 1; if depth == 0 { break } }
                    inner.append(line[j])
                    j = line.index(after: j)
                }
                guard depth == 0 else { throw ShErr("sustitución: falta cerrar $(") }
                out += limpia(await execute(inner))
                i = line.index(after: j)
                continue
            }

            // `comando` a la manera antigua
            if c == "`" {
                vueltas += 1
                if vueltas > 20 { throw ShErr("sustitución: demasiados ` ` anidados") }
                var inner = ""
                var j = line.index(after: i)
                while j < line.endIndex, line[j] != "`" { inner.append(line[j]); j = line.index(after: j) }
                guard j < line.endIndex else { throw ShErr("sustitución: falta cerrar `") }
                out += limpia(await execute(inner))
                i = line.index(after: j)
                continue
            }

            out.append(c)
            i = line.index(after: i)
        }
        return out
    }

    // --- historial ---------------------------------------------

    /// !! repite la última orden, !3 la número 3, !texto la última que empiece así.
    static func historyExpand(_ line: String, _ history: [String]) -> String? {
        guard line.hasPrefix("!"), line.count > 1 else { return nil }
        let arg = String(line.dropFirst())
        if arg == "!" { return history.last }
        if let n = Int(arg), n >= 1, n <= history.count { return history[n - 1] }
        return history.last { $0.hasPrefix(arg) }
    }

    func loadHistory() {
        guard let u = try? env.resolve(Shell.historyFile),
              let d = FileManager.default.contents(atPath: u.path),
              let s = String(data: d, encoding: .utf8) else { return }
        env.history = s.components(separatedBy: "\n").filter { !$0.isEmpty }.suffix(500)
    }

    func saveHistory() {
        guard let etc = try? env.resolve("/etc"), let u = try? env.resolve(Shell.historyFile) else { return }
        try? FileManager.default.createDirectory(at: etc, withIntermediateDirectories: true)
        try? env.history.suffix(500).joined(separator: "\n").write(to: u, atomically: true, encoding: .utf8)
    }

    /// Se ejecuta al abrir la terminal: alias y variables que quieras fijos.
    func runProfile() async -> String {
        guard let u = try? env.resolve(Shell.profileFile),
              FileManager.default.fileExists(atPath: u.path) else { return "" }
        return await execute("sh \(Shell.profileFile)")
    }

    // --- comandos que acompañan a estas mejoras ------------------

    static func shellPlus() -> [String: Spec] {
        var c: [String: Spec] = [:]

        c["profile"] = Spec(help: "profile [edit] — guion que se ejecuta al abrir la terminal") { ctx in
            let etc = try ctx.env.resolve("/etc")
            try FileManager.default.createDirectory(at: etc, withIntermediateDirectories: true)
            let u = try ctx.env.resolve(Shell.profileFile)
            if !FileManager.default.fileExists(atPath: u.path) {
                let plantilla = """
                # Se ejecuta cada vez que abres la terminal.
                # Una orden por línea; las que empiezan por # se ignoran.
                alias l='ls -l'
                export EDITOR=nano
                """
                try plantilla.write(to: u, atomically: true, encoding: .utf8)
            }
            if ctx.args.first == "edit" || ctx.args.isEmpty {
                ctx.sh.uiEdit?(u)
                return ""
            }
            return try ctx.input([Shell.profileFile])
        }

        c["status"] = Spec(help: "status — cómo acabó el último comando") { ctx in
            let s = ctx.env.vars["?"] ?? "0"
            return s == "0" ? "0 (bien)\n" : "\(s) (falló)\n"
        }

        c["hgrep"] = Spec(help: "hgrep <texto> — busca en el historial") { ctx in
            guard let t = ctx.args.first?.lowercased() else { throw ShErr("hgrep: falta el texto") }
            let hits = ctx.env.history.enumerated().filter { $0.element.lowercased().contains(t) }
            if hits.isEmpty { return "sin resultados\n" }
            return hits.map { "\(String($0.offset + 1).leftPad(4))  \($0.element)" }.joined(separator: "\n") +
                   "\n\nrepite una con !número\n"
        }

        return c
    }
}
