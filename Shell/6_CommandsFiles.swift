import Foundation

// ============================================================
// MARK: - Comandos: archivos y sistema
// ============================================================

extension Shell {

    static func core() -> [String: Spec] {
        var c: [String: Spec] = [:]
        let fm = FileManager.default

        c["help"] = Spec(help: "help [comando] — lista los comandos o explica uno") { ctx in
            if let name = ctx.args.first {
                guard let s = ctx.sh.commands[name] else { throw ShErr("help: no existe '\(name)'") }
                var out = s.help + "\n"
                if name == "swift" { out += Shell.swiftHelp }
                return out
            }
            let names = ctx.sh.commands.keys.sorted()
            var out = "SwiftShell — \(names.count) comandos. 'help <nombre>' para detalles.\n\n"
            var line = ""
            for n in names {
                line += n.padding(toLength: max(10, n.count + 1), withPad: " ", startingAt: 0)
                if line.count >= 50 { out += line + "\n"; line = "" }
            }
            if !line.isEmpty { out += line + "\n" }
            out += "\nSintaxis: | pipes, > >> < redirección, ; && ||, $VAR, comodines * ?\n"
            out += "Idiomas: 'mode js|swift|json|xml|plist' para escribir código directamente,\n"
            out += "o pega el código sin más: la terminal detecta el idioma sola.\n"
            out += "En JavaScript tienes http.get/post/json, fetchJSON, readFile, writeFile y listDir.\n"
            return out
        }
        c["man"] = c["help"]

        c["pwd"] = Spec(help: "pwd — muestra el directorio actual") { ctx in
            ctx.env.vpath(ctx.env.cwd) + "\n"
        }

        c["cd"] = Spec(help: "cd [ruta] — cambia de directorio") { ctx in
            let target = ctx.args.first ?? "/"
            let u = try ctx.env.resolve(target)
            guard ctx.env.isDir(u) else { throw ShErr("cd: \(target): no es un directorio") }
            ctx.env.cwd = u
            return ""
        }

        c["ls"] = Spec(help: "ls [-l] [-a] [-r] [rutas] — lista archivos") { ctx in
            let o = opts(ctx.args)
            let paths = o.rest.isEmpty ? ["."] : o.rest
            var out = ""
            for p in paths {
                let u = try ctx.env.resolve(p)
                guard ctx.env.exists(u) else { throw ShErr("ls: \(p): no existe") }
                var items: [URL]
                if ctx.env.isDir(u) {
                    items = (try? fm.contentsOfDirectory(at: u, includingPropertiesForKeys: nil)) ?? []
                } else { items = [u] }
                if !o.flags.contains("a") { items = items.filter { !$0.lastPathComponent.hasPrefix(".") } }
                items.sort { $0.lastPathComponent.lowercased() < $1.lastPathComponent.lowercased() }
                if o.flags.contains("r") { items.reverse() }
                if paths.count > 1 { out += "\(p):\n" }
                for i in items {
                    let isDir = ctx.env.isDir(i)
                    let nm = i.lastPathComponent + (isDir ? "/" : "")
                    if o.flags.contains("l") {
                        let at = (try? fm.attributesOfItem(atPath: i.path)) ?? [:]
                        let size = (at[.size] as? NSNumber)?.intValue ?? 0
                        let date = (at[.modificationDate] as? Date) ?? Date()
                        let df = DateFormatter()
                        df.dateFormat = "yyyy-MM-dd HH:mm"
                        let sz = isDir ? "     -" : humanSize(size).leftPad(6)
                        out += "\(isDir ? "d" : "-") \(sz)  \(df.string(from: date))  \(nm)\n"
                    } else {
                        out += nm + "\n"
                    }
                }
                if paths.count > 1 { out += "\n" }
            }
            return out
        }

        c["mkdir"] = Spec(help: "mkdir [-p] <dir...> — crea directorios") { ctx in
            let o = opts(ctx.args)
            guard !o.rest.isEmpty else { throw ShErr("mkdir: falta el nombre") }
            for p in o.rest {
                let u = try ctx.env.resolve(p)
                try fm.createDirectory(at: u, withIntermediateDirectories: o.flags.contains("p"))
            }
            return ""
        }

        c["rmdir"] = Spec(help: "rmdir <dir...> — borra directorios vacíos") { ctx in
            for p in ctx.args {
                let u = try ctx.env.resolve(p)
                guard ctx.env.isDir(u) else { throw ShErr("rmdir: \(p): no es un directorio") }
                let items = (try? fm.contentsOfDirectory(atPath: u.path)) ?? []
                guard items.isEmpty else { throw ShErr("rmdir: \(p): el directorio no está vacío") }
                try fm.removeItem(at: u)
            }
            return ""
        }

        c["rm"] = Spec(help: "rm [-r] [-f] <archivo...> — borra archivos o carpetas") { ctx in
            let o = opts(ctx.args)
            guard !o.rest.isEmpty else { throw ShErr("rm: falta el nombre") }
            for p in o.rest {
                let u = try ctx.env.resolve(p)
                if !ctx.env.exists(u) {
                    if o.flags.contains("f") { continue }
                    throw ShErr("rm: \(p): no existe")
                }
                if ctx.env.isDir(u), !o.flags.contains("r") { throw ShErr("rm: \(p): es un directorio (usa -r)") }
                try fm.removeItem(at: u)
            }
            return ""
        }

        c["cp"] = Spec(help: "cp [-r] <origen...> <destino> — copia") { ctx in
            let o = opts(ctx.args)
            guard o.rest.count >= 2 else { throw ShErr("cp: uso: cp origen destino") }
            let dst = try ctx.env.resolve(o.rest.last!)
            for s in o.rest.dropLast() {
                let src = try ctx.env.resolve(s)
                guard ctx.env.exists(src) else { throw ShErr("cp: \(s): no existe") }
                if ctx.env.isDir(src), !o.flags.contains("r") { throw ShErr("cp: \(s): es un directorio (usa -r)") }
                let target = ctx.env.isDir(dst) ? dst.appendingPathComponent(src.lastPathComponent) : dst
                if ctx.env.exists(target) { try fm.removeItem(at: target) }
                try fm.copyItem(at: src, to: target)
            }
            return ""
        }

        c["mv"] = Spec(help: "mv <origen...> <destino> — mueve o renombra") { ctx in
            guard ctx.args.count >= 2 else { throw ShErr("mv: uso: mv origen destino") }
            let dst = try ctx.env.resolve(ctx.args.last!)
            for s in ctx.args.dropLast() {
                let src = try ctx.env.resolve(s)
                guard ctx.env.exists(src) else { throw ShErr("mv: \(s): no existe") }
                let target = ctx.env.isDir(dst) ? dst.appendingPathComponent(src.lastPathComponent) : dst
                if ctx.env.exists(target) { try fm.removeItem(at: target) }
                try fm.moveItem(at: src, to: target)
            }
            return ""
        }

        c["touch"] = Spec(help: "touch <archivo...> — crea o actualiza la fecha") { ctx in
            for p in ctx.args {
                let u = try ctx.env.resolve(p)
                if ctx.env.exists(u) {
                    try fm.setAttributes([.modificationDate: Date()], ofItemAtPath: u.path)
                } else {
                    fm.createFile(atPath: u.path, contents: Data())
                }
            }
            return ""
        }

        c["cat"] = Spec(help: "cat [-n] [archivo...] — muestra el contenido") { ctx in
            let o = opts(ctx.args)
            let t = try ctx.input(o.rest)
            guard o.flags.contains("n") else { return t }
            var out = ""
            for (i, l) in t.components(separatedBy: "\n").enumerated() {
                out += "\(String(i + 1).leftPad(5))  \(l)\n"
            }
            return out
        }

        c["tree"] = Spec(help: "tree [ruta] — muestra el árbol de directorios") { ctx in
            func walk(_ u: URL, _ prefix: String) -> String {
                var items = ((try? fm.contentsOfDirectory(at: u, includingPropertiesForKeys: nil)) ?? [])
                    .filter { !$0.lastPathComponent.hasPrefix(".") }
                items.sort { $0.lastPathComponent < $1.lastPathComponent }
                var out = ""
                for (i, item) in items.enumerated() {
                    let last = i == items.count - 1
                    out += prefix + (last ? "`-- " : "|-- ") + item.lastPathComponent + "\n"
                    if ctx.env.isDir(item) {
                        out += walk(item, prefix + (last ? "    " : "|   "))
                    }
                }
                return out
            }
            let u = try ctx.env.resolve(ctx.args.first ?? ".")
            return ctx.env.vpath(u) + "\n" + walk(u, "")
        }

        c["find"] = Spec(help: "find [ruta] [-name patrón] [-type f|d] — busca archivos") { ctx in
            let o = opts(ctx.args, valued: ["name", "type"])
            let start = try ctx.env.resolve(o.rest.first ?? ".")
            var out = ""
            let e = fm.enumerator(atPath: start.path)
            while let rel = e?.nextObject() as? String {
                let full = start.appendingPathComponent(rel)
                if let pat = o.vals["name"], !Parser.match(Array(pat), Array(full.lastPathComponent)) { continue }
                if let t = o.vals["type"] {
                    if t == "f" && ctx.env.isDir(full) { continue }
                    if t == "d" && !ctx.env.isDir(full) { continue }
                }
                out += ctx.env.vpath(full) + "\n"
            }
            return out
        }

        c["du"] = Spec(help: "du [ruta] — tamaño total en disco") { ctx in
            let u = try ctx.env.resolve(ctx.args.first ?? ".")
            var total = 0
            if ctx.env.isDir(u) {
                let e = fm.enumerator(atPath: u.path)
                while let rel = e?.nextObject() as? String {
                    let at = (try? fm.attributesOfItem(atPath: u.appendingPathComponent(rel).path)) ?? [:]
                    total += (at[.size] as? NSNumber)?.intValue ?? 0
                }
            } else {
                let at = (try? fm.attributesOfItem(atPath: u.path)) ?? [:]
                total = (at[.size] as? NSNumber)?.intValue ?? 0
            }
            return "\(humanSize(total))\t\(ctx.env.vpath(u))\n"
        }

        c["df"] = Spec(help: "df — espacio libre del dispositivo") { ctx in
            let v = try ctx.env.root.resourceValues(forKeys: [.volumeAvailableCapacityKey, .volumeTotalCapacityKey])
            let free = v.volumeAvailableCapacity ?? 0
            let total = v.volumeTotalCapacity ?? 0
            return "total \(humanSize(total))   libre \(humanSize(free))\n"
        }

        c["stat"] = Spec(help: "stat <archivo> — detalles del archivo") { ctx in
            guard let p = ctx.args.first else { throw ShErr("stat: falta el archivo") }
            let u = try ctx.env.resolve(p)
            guard ctx.env.exists(u) else { throw ShErr("stat: \(p): no existe") }
            let at = try fm.attributesOfItem(atPath: u.path)
            let df = DateFormatter(); df.dateFormat = "yyyy-MM-dd HH:mm:ss"
            var out = "ruta      \(ctx.env.vpath(u))\n"
            out += "tipo      \(ctx.env.isDir(u) ? "directorio" : "archivo")\n"
            out += "tamaño    \((at[.size] as? NSNumber)?.intValue ?? 0) bytes\n"
            if let d = at[.creationDate] as? Date { out += "creado    \(df.string(from: d))\n" }
            if let d = at[.modificationDate] as? Date { out += "modificado \(df.string(from: d))\n" }
            return out
        }

        c["file"] = Spec(help: "file <archivo> — adivina el tipo de archivo") { ctx in
            guard let p = ctx.args.first else { throw ShErr("file: falta el archivo") }
            let u = try ctx.env.resolve(p)
            guard ctx.env.exists(u) else { throw ShErr("file: \(p): no existe") }
            if ctx.env.isDir(u) { return "\(p): directorio\n" }
            let ext = u.pathExtension.lowercased()
            let names = ["swift": "código Swift", "js": "código JavaScript", "json": "datos JSON",
                         "xml": "documento XML", "plist": "lista de propiedades", "md": "texto Markdown",
                         "txt": "texto plano", "html": "documento HTML", "css": "hoja de estilos",
                         "png": "imagen PNG", "jpg": "imagen JPEG", "csv": "datos CSV"]
            if let n = names[ext] { return "\(p): \(n)\n" }
            let d = fm.contents(atPath: u.path) ?? Data()
            let isText = String(data: d.prefix(512), encoding: .utf8) != nil
            return "\(p): \(isText ? "texto" : "datos binarios")\n"
        }

        c["echo"] = Spec(help: "echo [-n] <texto...> — escribe texto") { ctx in
            let o = opts(ctx.args)
            return o.rest.joined(separator: " ") + (o.flags.contains("n") ? "" : "\n")
        }

        c["date"] = Spec(help: "date [+formato] — fecha y hora (formato tipo yyyy-MM-dd)") { ctx in
            let df = DateFormatter()
            if let a = ctx.args.first, a.hasPrefix("+") { df.dateFormat = String(a.dropFirst()) }
            else { df.dateFormat = "EEE dd MMM yyyy HH:mm:ss" }
            return df.string(from: Date()) + "\n"
        }

        c["sleep"] = Spec(help: "sleep <segundos> — espera") { ctx in
            let s = Double(ctx.args.first ?? "1") ?? 1
            try await Task.sleep(nanoseconds: UInt64(min(s, 30) * 1_000_000_000))
            return ""
        }

        c["env"] = Spec(help: "env — muestra las variables") { ctx in
            ctx.env.vars.sorted { $0.key < $1.key }.map { "\($0.key)=\($0.value)" }.joined(separator: "\n") + "\n"
        }
        c["set"] = c["env"]

        c["export"] = Spec(help: "export NOMBRE=valor — define una variable") { ctx in
            for a in ctx.args {
                guard let eq = a.firstIndex(of: "=") else { throw ShErr("export: uso: export NOMBRE=valor") }
                ctx.env.vars[String(a[a.startIndex..<eq])] = String(a[a.index(after: eq)...])
            }
            return ""
        }

        c["unset"] = Spec(help: "unset <nombre...> — borra variables") { ctx in
            for a in ctx.args { ctx.env.vars.removeValue(forKey: a) }
            return ""
        }

        c["alias"] = Spec(help: "alias [nombre=comando] — crea o lista atajos") { ctx in
            if ctx.args.isEmpty {
                return ctx.env.aliases.sorted { $0.key < $1.key }.map { "\($0.key)='\($0.value)'" }.joined(separator: "\n") + "\n"
            }
            for a in ctx.args {
                guard let eq = a.firstIndex(of: "=") else { throw ShErr("alias: uso: alias nombre='comando'") }
                ctx.env.aliases[String(a[a.startIndex..<eq])] = String(a[a.index(after: eq)...])
            }
            return ""
        }

        c["unalias"] = Spec(help: "unalias <nombre> — borra un atajo") { ctx in
            for a in ctx.args { ctx.env.aliases.removeValue(forKey: a) }
            return ""
        }

        c["which"] = Spec(help: "which <comando> — dice si un comando existe") { ctx in
            var out = ""
            for a in ctx.args {
                if ctx.sh.commands[a] != nil { out += "\(a): comando interno\n" }
                else if let al = ctx.env.aliases[a] { out += "\(a): atajo de '\(al)'\n" }
                else { out += "\(a): no encontrado\n" }
            }
            return out
        }

        c["history"] = Spec(help: "history [-c] — historial de comandos") { ctx in
            if ctx.args.contains("-c") { ctx.env.history.removeAll(); return "" }
            return ctx.env.history.enumerated().map { "\(String($0.offset + 1).leftPad(4))  \($0.element)" }
                .joined(separator: "\n") + "\n"
        }

        c["clear"] = Spec(help: "clear — limpia la pantalla") { ctx in
            ctx.sh.uiClear?()
            return ""
        }

        c["whoami"] = Spec(help: "whoami — usuario actual") { ctx in (ctx.env.vars["USER"] ?? "mobile") + "\n" }

        c["uname"] = Spec(help: "uname [-a] — información del sistema") { _ in
            "SwiftShell " + ProcessInfo.processInfo.operatingSystemVersionString + "\n"
        }

        c["uptime"] = Spec(help: "uptime — tiempo encendido") { _ in
            let up = Int(ProcessInfo.processInfo.systemUptime)
            return "encendido hace \(up / 3600)h \((up % 3600) / 60)m\n"
        }

        // Sin selector ni ventana de exportar: la carpeta de la shell ya es
        // Documentos/shell, visible desde la app Archivos.
        c["pick"] = Spec(help: "pick — cómo traer archivos (sin interfaz)") { _ in
            "pick: copia tus archivos dentro de Documentos/shell con la app Archivos;\n" +
            "aparecen aquí al instante (prueba 'ls').\n"
        }

        c["save"] = Spec(help: "save <archivo> — dónde encontrar un archivo desde la app Archivos") { ctx in
            guard let p = ctx.args.first else { throw ShErr("save: falta el archivo") }
            let u = try ctx.env.resolve(p)
            guard ctx.env.exists(u), !ctx.env.isDir(u) else { throw ShErr("save: \(p): no es un archivo") }
            return "\(p) ya está en Archivos: Documentos/shell\(ctx.env.vpath(u))\n"
        }

        c["nano"] = Spec(help: "nano <archivo> — editor de pantalla completa (crea el archivo si no existe)") { ctx in
            guard let p = ctx.args.first else { throw ShErr("nano: falta el archivo") }
            let u = try ctx.env.resolve(p)
            if !ctx.env.exists(u) { FileManager.default.createFile(atPath: u.path, contents: Data()) }
            guard !ctx.env.isDir(u) else { throw ShErr("nano: \(p): es un directorio") }
            guard ctx.sh.uiEdit != nil else {
                throw ShErr("nano: en la shell de una IA se escribe con echo, cat > o sed")
            }
            ctx.sh.uiEdit?(u)
            return ""
        }
        c["edit"] = c["nano"]
        c["vi"] = c["nano"]

        return c
    }
}
