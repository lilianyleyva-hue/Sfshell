import Foundation

// ============================================================
// MARK: - Paquetes: instalar scripts y usarlos como comandos
// ============================================================
// Un "paquete" es un archivo .js o .swift guardado en /usr/bin.
// Cualquier archivo que esté ahí se puede llamar por su nombre,
// igual que un comando integrado.

extension Shell {

    static let binDir = "/usr/bin"

    /// Convierte un valor de Swift en un literal de JavaScript.
    static func jsLiteral(_ v: Any) -> String {
        guard let d = try? JSONSerialization.data(withJSONObject: v, options: [.fragmentsAllowed]),
              let s = String(data: d, encoding: .utf8) else { return "null" }
        return s
    }

    /// Busca un script instalado y lo envuelve como si fuera un comando.
    func installedSpec(_ name: String) -> Spec? {
        guard !name.contains("/") else { return nil }
        let candidates = ["\(Shell.binDir)/\(name)", "\(Shell.binDir)/\(name).js",
                          "\(Shell.binDir)/\(name).swift", "\(Shell.binDir)/\(name).mix"]
        for path in candidates {
            guard let u = try? env.resolve(path),
                  FileManager.default.fileExists(atPath: u.path),
                  !env.isDir(u),
                  let d = FileManager.default.contents(atPath: u.path),
                  let src = String(data: d, encoding: .utf8) else { continue }
            let ext = u.pathExtension.lowercased()
            return Spec(help: "\(name) — paquete instalado en \(path)") { ctx in
                if ext == "mix" {
                    return await Shell.runMix(src, args: ctx.args, stdin: ctx.stdin, ctx: ctx)
                }
                let prelude = "var ARGV = \(Shell.jsLiteral(ctx.args)); var STDIN = \(Shell.jsLiteral(ctx.stdin));"
                let body = ext == "swift" ? try SwiftJS.transpile(src) : src
                return try ctx.sh.js.eval(prelude + "\n" + body)
            }
        }
        return nil
    }

    static func packages() -> [String: Spec] {
        var c: [String: Spec] = [:]
        let fm = FileManager.default

        // Paquetes que vienen dentro y se instalan sin conexión.
        let catalog: [String: (String, String)] = [
            "saluda": ("saluda a quien le digas", """
            var quien = ARGV.length ? ARGV.join(" ") : "mundo";
            print("hola, " + quien + " 👋");
            """),

            "dado": ("tira dados: dado 2d6", """
            var spec = ARGV[0] || "1d6";
            var p = spec.split("d");
            var n = parseInt(p[0]) || 1, caras = parseInt(p[1]) || 6;
            var total = 0, tiradas = [];
            for (var i = 0; i < n; i++) {
                var t = 1 + Math.floor(Math.random() * caras);
                tiradas.push(t); total += t;
            }
            print(tiradas.join(" + ") + " = " + total);
            """),

            "json2csv": ("convierte un JSON de objetos en CSV", """
            var texto = ARGV.length ? readFile(ARGV[0]) : STDIN;
            var filas = JSON.parse(texto);
            if (!Array.isArray(filas) || !filas.length) { print("hace falta una lista de objetos"); }
            else {
                var cols = Object.keys(filas[0]);
                print(cols.join(","));
                for (var i = 0; i < filas.length; i++) {
                    print(cols.map(function(k){ return String(filas[i][k] === undefined ? "" : filas[i][k]); }).join(","));
                }
            }
            """),

            "tabla": ("alinea en columnas un texto separado por comas", """
            var texto = ARGV.length ? readFile(ARGV[0]) : STDIN;
            var filas = texto.split("\\n").filter(function(l){ return l.trim().length; })
                             .map(function(l){ return l.split(","); });
            var anchos = [];
            filas.forEach(function(f){ f.forEach(function(v, i){
                anchos[i] = Math.max(anchos[i] || 0, v.trim().length); }); });
            filas.forEach(function(f){
                print(f.map(function(v, i){
                    var s = v.trim();
                    return s + Array(anchos[i] - s.length + 3).join(" ");
                }).join(""));
            });
            """),

            "github": ("muestra los datos públicos de un usuario de GitHub", """
            var user = ARGV[0];
            if (!user) { print("uso: github <usuario>"); }
            else {
                var d = http.json("https://api.github.com/users/" + user);
                if (!d || d.message) { print("no encontrado: " + user); }
                else {
                    print(d.name || d.login);
                    print("repos     " + d.public_repos);
                    print("seguidores " + d.followers);
                    if (d.bio) print("bio       " + d.bio);
                }
            }
            """),

            "backup": ("copia todos los archivos del directorio a /backup", """
            var archivos = listDir(".");
            var n = 0;
            for (var i = 0; i < archivos.length; i++) {
                var f = archivos[i];
                if (f.indexOf(".") === 0) continue;
                var t = readFile(f);
                if (t && writeFile("/backup/" + f, t)) n++;
            }
            print("copiados " + n + " archivos en /backup");
            """),

            "fib": ("serie de Fibonacci, escrito en Swift", """
            func fib(n: Int) -> Int {
                if n < 2 { return n }
                var a = 0
                var b = 1
                for _ in 2...n {
                    let t = a + b
                    a = b
                    b = t
                }
                return b
            }
            let hasta = ARGV.length > 0 ? Int(ARGV[0]) : 15
            for i in 0...hasta {
                print("fib(\\(i)) = \\(fib(n: i))")
            }
            """)
        ]

        func ensureBin(_ ctx: Ctx) throws -> URL {
            let u = try ctx.env.resolve(Shell.binDir)
            try fm.createDirectory(at: u, withIntermediateDirectories: true)
            _ = try? fm.createDirectory(at: ctx.env.resolve("/backup"), withIntermediateDirectories: true)
            return u
        }

        c["pkg"] = Spec(help: "pkg <list|search|install|remove|info|new> — instala y gestiona paquetes") { ctx in
            let bin = try ensureBin(ctx)
            let sub = ctx.args.first ?? "list"
            let rest = Array(ctx.args.dropFirst())

            func installed() -> [String] {
                ((try? fm.contentsOfDirectory(atPath: bin.path)) ?? []).sorted()
            }

            switch sub {
            case "list":
                let items = installed()
                if items.isEmpty { return "no hay paquetes instalados. Prueba 'pkg search'.\n" }
                return "instalados en \(Shell.binDir):\n" +
                       items.map { "  \(($0 as NSString).deletingPathExtension)   (\($0))" }.joined(separator: "\n") + "\n"

            case "search":
                let term = rest.first?.lowercased() ?? ""
                var out = "paquetes disponibles:\n"
                for (name, info) in catalog.sorted(by: { $0.key < $1.key })
                where term.isEmpty || name.contains(term) || info.0.lowercased().contains(term) {
                    out += "  \(name.padding(toLength: 12, withPad: " ", startingAt: 0)) \(info.0)\n"
                }
                out += "\ninstala con: pkg install <nombre>\n"
                out += "o desde la red: pkg install <nombre> <url del script>\n"
                return out

            case "info":
                guard let n = rest.first else { throw ShErr("pkg info: falta el nombre") }
                guard let info = catalog[n] else { throw ShErr("pkg: '\(n)' no está en el catálogo") }
                let lang = info.1.contains("func ") && info.1.contains("print(") && info.1.contains("let ") ? "swift" : "js"
                return "\(n) — \(info.0)\nidioma: \(lang)\nlíneas: \(info.1.components(separatedBy: "\n").count)\n"

            case "install":
                guard let n = rest.first else { throw ShErr("pkg install: falta el nombre") }
                // desde una URL
                if let urlStr = rest.dropFirst().first {
                    var s = urlStr
                    if !s.contains("://") { s = "https://" + s }
                    guard let url = URL(string: s) else { throw ShErr("pkg: URL no válida") }
                    let (d, resp) = try await URLSession.shared.data(from: url)
                    if let http = resp as? HTTPURLResponse, http.statusCode >= 400 {
                        throw ShErr("pkg: HTTP \(http.statusCode) al descargar")
                    }
                    guard let src = String(data: d, encoding: .utf8) else { throw ShErr("pkg: el archivo no es texto") }
                    let ext = url.pathExtension.lowercased() == "swift" ? "swift" : "js"
                    let dest = bin.appendingPathComponent("\(n).\(ext)")
                    try src.write(to: dest, atomically: true, encoding: .utf8)
                    return "instalado \(n) (\(humanSize(d.count))) desde \(url.host ?? s)\nya puedes escribir: \(n)\n"
                }
                // desde el catálogo interno
                guard let info = catalog[n] else {
                    throw ShErr("pkg: no conozco '\(n)'. Mira 'pkg search' o da una URL.")
                }
                let isSwift = n == "fib"
                let dest = bin.appendingPathComponent("\(n).\(isSwift ? "swift" : "js")")
                try info.1.write(to: dest, atomically: true, encoding: .utf8)
                return "instalado \(n) — \(info.0)\nya puedes escribir: \(n)\n"

            case "remove", "uninstall":
                guard let n = rest.first else { throw ShErr("pkg remove: falta el nombre") }
                var borrados = 0
                for f in installed() where (f as NSString).deletingPathExtension == n {
                    try fm.removeItem(at: bin.appendingPathComponent(f))
                    borrados += 1
                }
                guard borrados > 0 else { throw ShErr("pkg: '\(n)' no está instalado") }
                return "desinstalado \(n)\n"

            case "new":
                guard let n = rest.first else { throw ShErr("pkg new: falta el nombre") }
                let ext = rest.dropFirst().first == "swift" ? "swift" : "js"
                let dest = bin.appendingPathComponent("\(n).\(ext)")
                guard !fm.fileExists(atPath: dest.path) else { throw ShErr("pkg: '\(n)' ya existe") }
                let plantilla = ext == "swift"
                    ? "// \(n) — paquete propio\nlet quien = ARGV.length > 0 ? ARGV[0] : \"mundo\"\nprint(\"hola \\(quien)\")\n"
                    : "// \(n) — paquete propio\n// ARGV son los argumentos, STDIN lo que llega por la tubería\nprint(\"hola desde \(n)\", ARGV.join(\" \"));\n"
                try plantilla.write(to: dest, atomically: true, encoding: .utf8)
                ctx.sh.uiEdit?(dest)
                return "creado \(Shell.binDir)/\(n).\(ext) — se abre el editor\n"

            default:
                throw ShErr("pkg: subcomando desconocido '\(sub)'. Usa list, search, install, remove, info o new.")
            }
        }
        c["apk"] = c["pkg"]

        c["support"] = Spec(help: "support — qué tipos de archivo sabe ejecutar la terminal") { _ in
            var out = "Tipos que 'run' y './archivo' saben abrir:\n\n"
            out += "  .swift   subconjunto de Swift traducido a JavaScript\n"
            out += "  .js      JavaScript completo (JavaScriptCore)\n"
            out += "  .sh      guion de comandos de esta terminal\n"
            out += "  .json    se valida y se muestra formateado\n"
            out += "  .xml     se analiza y se muestra con sangría\n"
            out += "  .plist   lista de propiedades, en texto legible\n"
            out += "  .csv     cabecera y primeras filas\n"
            out += "  .md .txt se muestran tal cual\n\n"
            out += "Los paquetes de \(Shell.binDir) se llaman por su nombre, sin extensión.\n"
            return out
        }

        c["run"] = Spec(help: "run <archivo> — ejecuta cualquier tipo admitido (mira 'support')") { ctx in
            guard let p = ctx.args.first else { throw ShErr("run: falta el archivo") }
            let u = try ctx.env.resolve(p)
            guard ctx.env.exists(u), !ctx.env.isDir(u) else {
                let items = ((try? fm.contentsOfDirectory(atPath: ctx.env.cwd.path)) ?? []).sorted()
                throw ShErr("run: \(p): no existe en \(ctx.env.vpath(ctx.env.cwd))\n" +
                            "aquí hay: " + (items.isEmpty ? "(nada — prueba 'demo')" : items.joined(separator: "  ")))
            }
            let args = Array(ctx.args.dropFirst()).joined(separator: " ")
            let extra = args.isEmpty ? "" : " " + args
            switch u.pathExtension.lowercased() {
            case "swift": return await ctx.sh.execute("swift \(p)\(extra)")
            case "js": return await ctx.sh.execute("js \(p)\(extra)")
            case "sh": return await ctx.sh.execute("sh \(p)")
            case "json": return await ctx.sh.execute("json pretty \(p)")
            case "xml": return await ctx.sh.execute("xml pretty \(p)")
            case "plist": return await ctx.sh.execute("plist show \(p)")
            case "csv": return await ctx.sh.execute("csv head \(p)")
            case "md", "txt", "": return try ctx.input([p])
            default: throw ShErr("run: no sé abrir '.\(u.pathExtension)' — mira 'support'")
            }
        }

        return c
    }
}
