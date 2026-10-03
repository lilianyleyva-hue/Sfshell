import Foundation

// ============================================================
// MARK: - Comandos extra: Linux y a-Shell
// ============================================================
// Este archivo se fusiona AL FINAL, así que lo que define aquí
// sustituye a la versión anterior del mismo comando.

extension Shell {

    static func extras() -> [String: Spec] {
        var c: [String: Spec] = [:]
        let fm = FileManager.default

        // --- rutas -------------------------------------------------

        c["basename"] = Spec(help: "basename <ruta> [sufijo] — deja solo el nombre del archivo") { ctx in
            guard let p = ctx.args.first else { throw ShErr("basename: falta la ruta") }
            var n = (p as NSString).lastPathComponent
            if let suf = ctx.args.dropFirst().first, n.hasSuffix(suf) { n.removeLast(suf.count) }
            return n + "\n"
        }

        c["dirname"] = Spec(help: "dirname <ruta> — deja solo el directorio") { ctx in
            guard let p = ctx.args.first else { throw ShErr("dirname: falta la ruta") }
            let d = (p as NSString).deletingLastPathComponent
            return (d.isEmpty ? "." : d) + "\n"
        }

        c["realpath"] = Spec(help: "realpath <ruta> — ruta absoluta dentro del área de trabajo") { ctx in
            guard let p = ctx.args.first else { throw ShErr("realpath: falta la ruta") }
            return ctx.env.vpath(try ctx.env.resolve(p)) + "\n"
        }

        c["mount"] = Spec(help: "mount — dónde está cada cosa de verdad en el iPad") { ctx in
            var out = "/            → \(ctx.env.root.path)\n"
            out += "actual       → \(ctx.env.cwd.path)\n"
            out += "\nLa terminal solo ve este directorio. Los archivos del proyecto de\n"
            out += "Swift Playgrounds (ContentView.swift y compañía) no están aquí:\n"
            out += "son código ya compilado, no archivos que la app pueda leer.\n"
            out += "Usa 'nano archivo.swift' para escribir el tuyo, o 'pick' para\n"
            out += "traer archivos desde la app Archivos.\n"
            return out
        }

        // --- pila de directorios -----------------------------------

        c["pushd"] = Spec(help: "pushd <dir> — entra en un directorio y guarda el actual") { ctx in
            guard let p = ctx.args.first else { throw ShErr("pushd: falta el directorio") }
            let u = try ctx.env.resolve(p)
            guard ctx.env.isDir(u) else { throw ShErr("pushd: \(p): no es un directorio") }
            var stack = (ctx.env.vars["DIRSTACK"] ?? "").split(separator: ":").map(String.init)
            stack.append(ctx.env.vpath(ctx.env.cwd))
            ctx.env.vars["DIRSTACK"] = stack.joined(separator: ":")
            ctx.env.cwd = u
            return ctx.env.vpath(u) + "\n"
        }

        c["popd"] = Spec(help: "popd — vuelve al directorio guardado") { ctx in
            var stack = (ctx.env.vars["DIRSTACK"] ?? "").split(separator: ":").map(String.init)
            guard let last = stack.popLast() else { throw ShErr("popd: la pila está vacía") }
            ctx.env.vars["DIRSTACK"] = stack.joined(separator: ":")
            ctx.env.cwd = try ctx.env.resolve(last)
            return last + "\n"
        }

        c["dirs"] = Spec(help: "dirs — muestra la pila de directorios") { ctx in
            let stack = (ctx.env.vars["DIRSTACK"] ?? "").split(separator: ":").map(String.init)
            return ([ctx.env.vpath(ctx.env.cwd)] + stack.reversed()).joined(separator: " ") + "\n"
        }

        // --- texto -------------------------------------------------

        c["tac"] = Spec(help: "tac [archivo...] — muestra las líneas al revés") { ctx in
            try ctx.lines(ctx.args).reversed().joined(separator: "\n") + "\n"
        }

        c["fold"] = Spec(help: "fold [-w N] [archivo...] — parte las líneas largas") { ctx in
            let o = opts(ctx.args, valued: ["w"])
            let w = max(1, Int(o.vals["w"] ?? "80") ?? 80)
            var out = ""
            for l in try ctx.lines(o.rest) {
                var rest = Substring(l)
                if rest.isEmpty { out += "\n"; continue }
                while !rest.isEmpty {
                    out += String(rest.prefix(w)) + "\n"
                    rest = rest.dropFirst(w)
                }
            }
            return out
        }

        c["strings"] = Spec(help: "strings [-n N] [archivo...] — saca los textos legibles") { ctx in
            let o = opts(ctx.args, valued: ["n"])
            let minLen = Int(o.vals["n"] ?? "4") ?? 4
            let t = try ctx.input(o.rest)
            var out = ""
            var cur = ""
            for ch in t {
                if ch.isLetter || ch.isNumber || ch.isPunctuation || ch == " " { cur.append(ch) }
                else { if cur.count >= minLen { out += cur + "\n" }; cur = "" }
            }
            if cur.count >= minLen { out += cur + "\n" }
            return out
        }

        c["printf"] = Spec(help: "printf <formato> [valores...] — imprime con formato (%s %d \\n)") { ctx in
            guard let f = ctx.args.first else { throw ShErr("printf: falta el formato") }
            var vals = Array(ctx.args.dropFirst())
            var out = ""
            var i = f.startIndex
            while i < f.endIndex {
                let ch = f[i]
                if ch == "\\", f.index(after: i) < f.endIndex {
                    i = f.index(after: i)
                    switch f[i] {
                    case "n": out += "\n"
                    case "t": out += "\t"
                    default: out.append(f[i])
                    }
                } else if ch == "%", f.index(after: i) < f.endIndex {
                    i = f.index(after: i)
                    let v = vals.isEmpty ? "" : vals.removeFirst()
                    switch f[i] {
                    case "s": out += v
                    case "d": out += String(Int(v) ?? 0)
                    case "%": out += "%"
                    default: out += v
                    }
                } else { out.append(ch) }
                i = f.index(after: i)
            }
            return out
        }

        c["diff"] = Spec(help: "diff <archivo1> <archivo2> — compara línea a línea") { ctx in
            guard ctx.args.count >= 2 else { throw ShErr("diff: hacen falta dos archivos") }
            let a = try ctx.lines([ctx.args[0]])
            let b = try ctx.lines([ctx.args[1]])
            var out = ""
            for i in 0..<max(a.count, b.count) {
                let x = i < a.count ? a[i] : nil
                let y = i < b.count ? b[i] : nil
                if x == y { continue }
                if let x = x { out += "\(i + 1)< \(x)\n" }
                if let y = y { out += "\(i + 1)> \(y)\n" }
            }
            return out.isEmpty ? "" : out
        }

        c["awk"] = Spec(help: "awk [-F sep] '{print $1, $2}' [archivo...] — columnas (subconjunto)") { ctx in
            let o = opts(ctx.args, valued: ["F"])
            guard let prog = o.rest.first else { throw ShErr("awk: falta el programa, por ejemplo '{print $1}'") }
            let sep = o.vals["F"]
            let body = prog.trimmingCharacters(in: CharacterSet(charactersIn: "{} "))
            guard body.hasPrefix("print") else { throw ShErr("awk: solo se admite '{print ...}'") }
            let items = String(body.dropFirst("print".count))
                .split(separator: ",")
                .map { $0.trimmingCharacters(in: .whitespaces) }
                .filter { !$0.isEmpty }
            let lines = try ctx.lines(Array(o.rest.dropFirst()))
            var out = ""
            for (n, line) in lines.enumerated() {
                let fields: [String] = sep == nil
                    ? line.split(whereSeparator: { $0.isWhitespace }).map(String.init)
                    : line.components(separatedBy: sep!)
                var parts: [String] = []
                for it in items.isEmpty ? ["$0"] : items {
                    if it == "$0" { parts.append(line) }
                    else if it == "NF" { parts.append(String(fields.count)) }
                    else if it == "NR" { parts.append(String(n + 1)) }
                    else if it.hasPrefix("$"), let k = Int(it.dropFirst()) {
                        parts.append(k >= 1 && k <= fields.count ? fields[k - 1] : "")
                    } else {
                        parts.append(it.trimmingCharacters(in: CharacterSet(charactersIn: "\"'")))
                    }
                }
                out += parts.joined(separator: " ") + "\n"
            }
            return out
        }

        c["xargs"] = Spec(help: "xargs <comando> — ejecuta el comando con lo que llega por la tubería") { ctx in
            guard !ctx.args.isEmpty else { throw ShErr("xargs: falta el comando") }
            let items = ctx.stdin.split(whereSeparator: { $0.isWhitespace }).map(String.init)
            let cmd = ctx.args.joined(separator: " ")
            return await ctx.sh.execute(cmd + " " + items.joined(separator: " "))
        }

        c["less"] = Spec(help: "less [archivo...] — muestra el contenido (igual que cat aquí)") { ctx in
            try ctx.input(ctx.args)
        }
        c["more"] = c["less"]

        // --- condiciones y control ---------------------------------

        c["test"] = Spec(help: "test <-e|-f|-d|-z|-n ruta> o <a = b> — falla si no se cumple") { ctx in
            let a = ctx.args
            func fail() throws -> Never { throw ShErr("") }
            if a.count == 2 {
                let u = try? ctx.env.resolve(a[1])
                switch a[0] {
                case "-e": if u == nil || !ctx.env.exists(u!) { try fail() }
                case "-f": if u == nil || !ctx.env.exists(u!) || ctx.env.isDir(u!) { try fail() }
                case "-d": if u == nil || !ctx.env.isDir(u!) { try fail() }
                case "-z": if !a[1].isEmpty { try fail() }
                case "-n": if a[1].isEmpty { try fail() }
                default: throw ShErr("test: operador desconocido '\(a[0])'")
                }
                return ""
            }
            if a.count == 3 {
                switch a[1] {
                case "=", "==": if a[0] != a[2] { try fail() }
                case "!=": if a[0] == a[2] { try fail() }
                case "-eq": if Int(a[0]) != Int(a[2]) { try fail() }
                case "-ne": if Int(a[0]) == Int(a[2]) { try fail() }
                case "-lt": if !((Int(a[0]) ?? 0) < (Int(a[2]) ?? 0)) { try fail() }
                case "-gt": if !((Int(a[0]) ?? 0) > (Int(a[2]) ?? 0)) { try fail() }
                default: throw ShErr("test: operador desconocido '\(a[1])'")
                }
                return ""
            }
            throw ShErr("test: número de argumentos incorrecto")
        }
        c["["] = c["test"]

        c["true"] = Spec(help: "true — no hace nada y sale bien") { _ in "" }
        c["false"] = Spec(help: "false — no hace nada y sale mal") { _ in throw ShErr("") }

        c["yes"] = Spec(help: "yes [texto] [-n N] — repite un texto N veces (100 por omisión)") { ctx in
            let o = opts(ctx.args, valued: ["n"])
            let n = min(10_000, Int(o.vals["n"] ?? "100") ?? 100)
            let t = o.rest.isEmpty ? "y" : o.rest.joined(separator: " ")
            return String(repeating: t + "\n", count: n)
        }

        // --- sistema -----------------------------------------------

        c["id"] = Spec(help: "id — usuario actual") { ctx in
            "uid=501(\(ctx.env.vars["USER"] ?? "mobile")) gid=501(mobile) sandbox=swiftshell\n"
        }

        c["hostname"] = Spec(help: "hostname — nombre del dispositivo") { _ in
            ProcessInfo.processInfo.hostName + "\n"
        }

        c["printenv"] = Spec(help: "printenv [nombre] — muestra una variable o todas") { ctx in
            if let n = ctx.args.first {
                guard let v = ctx.env.vars[n] else { throw ShErr("") }
                return v + "\n"
            }
            return ctx.env.vars.sorted { $0.key < $1.key }.map { "\($0.key)=\($0.value)" }.joined(separator: "\n") + "\n"
        }

        c["free"] = Spec(help: "free — memoria del dispositivo") { _ in
            let total = ProcessInfo.processInfo.physicalMemory
            return "memoria física: \(humanSize(Int(total)))\n"
        }

        c["mktemp"] = Spec(help: "mktemp — crea un archivo temporal vacío") { ctx in
            let name = "tmp.\(Int(Date().timeIntervalSince1970)).\(Int.random(in: 1000...9999))"
            let u = try ctx.env.resolve(name)
            fm.createFile(atPath: u.path, contents: Data())
            return ctx.env.vpath(u) + "\n"
        }

        c["cal"] = Spec(help: "cal [mes] [año] — calendario del mes") { ctx in
            var cal = Calendar(identifier: .gregorian)
            cal.firstWeekday = 2
            let now = Date()
            var comps = cal.dateComponents([.year, .month], from: now)
            if let m = ctx.args.first.flatMap({ Int($0) }), (1...12).contains(m) { comps.month = m }
            if let y = ctx.args.dropFirst().first.flatMap({ Int($0) }) { comps.year = y }
            comps.day = 1
            guard let first = cal.date(from: comps),
                  let range = cal.range(of: .day, in: .month, for: first) else {
                throw ShErr("cal: fecha no válida")
            }
            let df = DateFormatter()
            df.dateFormat = "MMMM yyyy"
            var out = df.string(from: first) + "\n"
            out += "lu ma mi ju vi sá do\n"
            let weekday = cal.component(.weekday, from: first)       // 1 = domingo
            let offset = (weekday + 5) % 7                            // lunes = 0
            var line = String(repeating: "   ", count: offset)
            for d in range {
                line += String(d).leftPad(2) + " "
                if (offset + d) % 7 == 0 { out += line.trimmingCharacters(in: .whitespaces) + "\n"; line = "" }
            }
            if !line.trimmingCharacters(in: .whitespaces).isEmpty { out += line + "\n" }
            return out
        }

        // --- ayuda para empezar ------------------------------------

        c["demo"] = Spec(help: "demo — crea archivos de ejemplo para probar la terminal") { ctx in
            let files: [String: String] = [
                "hola.swift": """
                let nombre = "mundo"
                for i in 1...3 {
                    print("hola \\(nombre) \\(i)")
                }
                """,
                "hola.js": """
                const suma = (a, b) => a + b;
                print("2 + 3 =", suma(2, 3));
                print("archivos aquí:", listDir(".").join(", "));
                """,
                "datos.json": """
                {"nombre": "SwiftShell", "version": 1, "comandos": ["ls", "cat", "cd"]}
                """,
                "notas.txt": "primera línea\nsegunda línea\ntercera línea\n"
            ]
            var out = ""
            for (name, body) in files.sorted(by: { $0.key < $1.key }) {
                try body.write(to: try ctx.env.resolve(name), atomically: true, encoding: .utf8)
                out += "creado \(name)\n"
            }
            out += "\nPruébalos así:\n"
            out += "  ls -l\n  cat notas.txt\n  swift hola.swift\n  js hola.js\n  json pretty datos.json\n"
            return out
        }

        c["pickFolder"] = Spec(help: "pickFolder — (sin interfaz) cómo traer una carpeta") { _ in
            throw ShErr("pickFolder: la terminal es solo texto. Copia la carpeta dentro de\n" +
                        "Documentos/shell con la app Archivos y aparecerá aquí.")
        }

        // --- swift y run, con mensaje claro si no está el archivo ---

        func missing(_ cmd: String, _ p: String, _ ctx: Ctx) -> ShErr {
            let items = ((try? fm.contentsOfDirectory(atPath: ctx.env.cwd.path)) ?? []).sorted()
            var m = "\(cmd): \(p): no existe en \(ctx.env.vpath(ctx.env.cwd))\n"
            m += "aquí hay: " + (items.isEmpty ? "(nada todavía — prueba 'demo')" : items.joined(separator: "  ")) + "\n"
            m += "La terminal no ve los archivos del proyecto de Swift Playgrounds:\n"
            m += "ContentView.swift ya está compilado dentro de la app y no se puede leer.\n"
            m += "Escribe el tuyo con 'nano \(p)' o cópialo a Documentos/shell desde la app Archivos."
            return ShErr(m)
        }

        c["swift"] = Spec(help: "swift [-e código] [archivo.swift] — ejecuta un subconjunto de Swift") { ctx in
            var src = ""
            if ctx.args.first == "-e" {
                src = ctx.args.dropFirst().joined(separator: " ")
            } else if let p = ctx.args.first {
                let u = try ctx.env.resolve(p)
                guard ctx.env.exists(u), !ctx.env.isDir(u) else { throw missing("swift", p, ctx) }
                src = try ctx.input([p])
            } else {
                src = ctx.stdin
            }
            guard !src.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
                throw ShErr("swift: no hay código que ejecutar")
            }
            return try ctx.sh.js.eval(SwiftJS.transpile(src))
        }

        c["run"] = Spec(help: "run <archivo> — ejecuta según la extensión (.js .swift .json .xml .plist)") { ctx in
            guard let p = ctx.args.first else { throw ShErr("run: falta el archivo") }
            let u = try ctx.env.resolve(p)
            guard ctx.env.exists(u), !ctx.env.isDir(u) else { throw missing("run", p, ctx) }
            let ext = u.pathExtension.lowercased()
            switch ext {
            case "js", "swift", "json", "xml", "plist":
                let cmd = (ext == "json" || ext == "xml") ? "\(ext) pretty" : (ext == "plist" ? "plist show" : ext)
                return await ctx.sh.execute("\(cmd) \(p)")
            default:
                throw ShErr("run: no sé ejecutar '.\(ext)' — prueba con .swift, .js, .json, .xml o .plist")
            }
        }

        return c
    }
}
