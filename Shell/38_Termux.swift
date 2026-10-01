import Foundation
#if os(Linux) && canImport(FoundationNetworking)
import FoundationNetworking
#endif

// ============================================================
// MARK: - Termux para iOS
// ============================================================
// Lo que hace que esto se sienta como Termux:
//   pkg / apt       update · upgrade · install · uninstall · search ·
//                   list-all · list-installed · show  (repositorio propio)
//   paquetes        neofetch cowsay fortune sl cmatrix todo htop clima
//                   ipinfo … solo existen cuando los instalas
//   sistema         almacenamiento · abrir-url · abrir · notificar ·
//                   notificaciones · hablar · wake-lock/unlock · recargar ·
//                   repo · ayuda-sistema (y toast, battery, clip, clipget,
//                   device, vibrate de antes) — sin 'termux' en los nombres
//   ~/.bashrc       se ejecuta al abrir (además de /etc/profile.sh)
//   PS1             prompt a tu gusto: PS1='\u@\h:\w \$ '
//   /etc/motd       el mensaje de bienvenida (edítalo con nano /etc/motd)
// Igual que Termux, cada IA tiene su propio $PREFIX: lo que instala una
// shell (/usr/bin) es solo de esa shell.

/// Inmutable: se puede compartir entre hilos (la lista es estática).
struct PaqueteTermux: @unchecked Sendable {
    let descripcion: String
    let version: String
    /// nil = no se puede en iOS (el texto explica por qué y qué usar)
    let spec: Spec?
    let nota: String
}

enum RepoTermux {

    static func paquete(_ n: String) -> PaqueteTermux? { todos[n.lowercased()] }

    static let fortunas = [
        "El que madruga, compila antes.",
        "Más vale un commit pequeño que cien en el tintero.",
        "Funciona en mi iPad.",
        "Lee el error: casi siempre dice la verdad.",
        "No hay bug pequeño si es en producción.",
        "Un 'ls' a tiempo ahorra cien 'cd'.",
        "Quien no guarda con :wq, escribe dos veces.",
        "La IA que escucha aprende; la que habla, enseña.",
        "Primero que funcione, luego que sea bonito, luego que sea rápido.",
        "Todo programa tiene al menos un bug más.",
        "Las 18 mentes no se ponen de acuerdo, y está bien.",
        "Si duele, hazlo más seguido: compila, prueba, repite.",
        "No es magia, es un bucle while.",
        "Dime qué comandos usas y te diré qué sistema extrañas.",
        "Un byte en la mano vale más que un gigabyte en la nube.",
        "Camarón que se duerme, se lo lleva el garbage collector.",
        "En casa de herrero, script de bash.",
        "No dejes para mañana lo que puedes automatizar hoy.",
    ]

    static let todos: [String: PaqueteTermux] = {
        var r: [String: PaqueteTermux] = [:]

        r["neofetch"] = PaqueteTermux(descripcion: "información del sistema con logo", version: "7.1.0", spec: Spec(help: "neofetch — información del sistema") { ctx in
            let d = await Plataforma.dispositivo()
            let up = Int(ProcessInfo.processInfo.systemUptime)
            let mem = ProcessInfo.processInfo.physicalMemory
            let t = SistemaIAs.tamaño(ctx.env.root)
            let bin = (try? ctx.env.resolve(Shell.binDir)).flatMap { try? FileManager.default.contentsOfDirectory(atPath: $0.path) } ?? []
            let usuario = ctx.env.vars["USER"] ?? "mobile"
            let logo = [
                "   ____  _____ ",
                "  / ___||  ___|",
                "  \\___ \\| |_   ",
                "   ___) |  _|  ",
                "  |____/|_|    ",
                "  SwiftShell   ",
                "               ",
                "               ",
                "               ",
                "               ",
            ]
            let info = [
                "\(usuario)@\(d.modelo.lowercased())",
                String(repeating: "-", count: usuario.count + d.modelo.count + 1),
                "SO: \(d.sistema) \(d.version)",
                "Host: \(d.modelo)",
                "Tiempo encendido: \(up / 3600)h \((up % 3600) / 60)m",
                "Paquetes: \(bin.count) (pkg) · \(ctx.sh.commands.count) integrados",
                "Shell: swiftsh" + (ctx.sh.rolIA.map { " (IA \($0.rawValue))" } ?? ""),
                "Terminal: solo texto",
                "Memoria: \(humanSize(Int(min(mem, UInt64(Int.max)))))",
                "Disco (/): \(t.archivos) archivos, \(humanSize(t.bytes))",
            ]
            return zip(logo, info).map { "\($0)  \($1)" }.joined(separator: "\n") + "\n"
        }, nota: "")

        r["cowsay"] = PaqueteTermux(descripcion: "una vaca que dice lo que quieras", version: "3.04", spec: Spec(help: "cowsay <texto> — una vaca que habla") { ctx in
            let t = (ctx.args.isEmpty ? ctx.stdin : ctx.args.joined(separator: " ")).trimmingCharacters(in: .whitespacesAndNewlines)
            let texto = t.isEmpty ? "muu" : t
            var lineas: [String] = []
            var actual = ""
            for p in texto.split(separator: " ") {
                if actual.count + p.count + 1 > 38, !actual.isEmpty { lineas.append(actual); actual = "" }
                actual += (actual.isEmpty ? "" : " ") + p
            }
            if !actual.isEmpty { lineas.append(actual) }
            let ancho = lineas.map(\.count).max() ?? 0
            var out = " " + String(repeating: "_", count: ancho + 2) + "\n"
            for (i, l) in lineas.enumerated() {
                let (a, b): (String, String) = lineas.count == 1 ? ("<", ">") : (i == 0 ? ("/", "\\") : (i == lineas.count - 1 ? ("\\", "/") : ("|", "|")))
                out += "\(a) \(l.padding(toLength: ancho, withPad: " ", startingAt: 0)) \(b)\n"
            }
            out += " " + String(repeating: "-", count: ancho + 2) + "\n"
            out += "        \\   ^__^\n         \\  (oo)\\_______\n            (__)\\       )\\/\\\n                ||----w |\n                ||     ||\n"
            return out
        }, nota: "")

        r["fortune"] = PaqueteTermux(descripcion: "una frase al azar", version: "1.99", spec: Spec(help: "fortune — una frase al azar") { _ in
            (fortunas.randomElement() ?? "") + "\n"
        }, nota: "")

        r["sl"] = PaqueteTermux(descripcion: "pasa un tren (por escribir mal 'ls')", version: "5.02", spec: Spec(help: "sl — un tren de vapor") { _ in
            """
                  ====        ________                ___________
              _D _|  |_______/        \\__I_I_____===__|_________|
               |(_)---  |   H\\________/ |   |        =|___ ___|
               /     |  |   H  |  |     |   |         ||_| |_||
              |      |  |   H  |__--------------------| [___] |
              | ________|___H__/__|_____/[][]~\\_______|       |
              |/ |   |-----------I_____I [][] []  D   |=======|_
            __/ =| o |=-~~\\  /~~\\  /~~\\  /~~\\ ____Y___________|__
             |/-=|___|=    ||    ||    ||    |_____/~\\___/
              \\_/      \\O=====O=====O=====O_/      \\_/

            """
        }, nota: "")

        r["cmatrix"] = PaqueteTermux(descripcion: "lluvia de caracteres estilo Matrix", version: "2.0", spec: Spec(help: "cmatrix [líneas] — lluvia de caracteres") { ctx in
            let n = min(60, max(1, Int(ctx.args.first ?? "12") ?? 12))
            let simbolos = Array("ｱｲｳｴｵｶｷｸｹｺｻｼｽｾｿﾀﾁﾂﾃﾄ0123456789")
            return (0 ..< n).map { _ in
                String((0 ..< 48).map { _ in Int.random(in: 0 ..< 4) == 0 ? " " : simbolos.randomElement()! })
            }.joined(separator: "\n") + "\n"
        }, nota: "")

        r["todo"] = PaqueteTermux(descripcion: "lista de tareas en ~/.todo", version: "1.0", spec: Spec(help: "todo [add <texto> | done <n> | rm <n> | clear]") { ctx in
            let u = try ctx.env.resolve("/.todo")
            var tareas = ((try? String(contentsOf: u, encoding: .utf8)) ?? "").split(separator: "\n").map(String.init)
            let a = ctx.args
            switch a.first ?? "" {
            case "add", "+":
                let t = a.dropFirst().joined(separator: " ")
                guard !t.isEmpty else { throw ShErr("todo add: ¿qué tarea?") }
                tareas.append("[ ] " + t)
            case "done", "x":
                guard let n = Int(a.dropFirst().first ?? ""), n >= 1, n <= tareas.count else { throw ShErr("todo done <número>") }
                tareas[n - 1] = "[x]" + tareas[n - 1].dropFirst(3)
            case "rm", "-":
                guard let n = Int(a.dropFirst().first ?? ""), n >= 1, n <= tareas.count else { throw ShErr("todo rm <número>") }
                tareas.remove(at: n - 1)
            case "clear":
                tareas.removeAll { $0.hasPrefix("[x]") }
            case "": break
            default:
                tareas.append("[ ] " + a.joined(separator: " "))
            }
            try (tareas.joined(separator: "\n") + (tareas.isEmpty ? "" : "\n")).write(to: u, atomically: true, encoding: .utf8)
            if tareas.isEmpty { return "sin tareas — todo add comprar pan\n" }
            return tareas.enumerated().map { "\(String($0.offset + 1).leftPad(3)). \($0.element)" }.joined(separator: "\n") + "\n"
        }, nota: "")

        r["htop"] = PaqueteTermux(descripcion: "monitor de procesos y memoria", version: "3.3.0", spec: Spec(help: "htop — procesos, memoria y las IAs") { ctx in
            let p = ProcessInfo.processInfo
            var out = "  CPU: \(p.activeProcessorCount) núcleos   Mem: \(humanSize(Int(min(p.physicalMemory, UInt64(Int.max)))))   "
            out += "Encendido: \(Int(p.systemUptime) / 60) min\n"
            out += "  IAs: 18 " + (SistemaIAs.uno.estanLibres ? "(actuando solas)" : "(quietas)") + "   api-local: \(APILocal.uno.totalPeticiones) peticiones\n\n"
            return out + (await ctx.sh.execute("top"))
        }, nota: "")

        r["clima"] = PaqueteTermux(descripcion: "el clima de una ciudad (wttr.in)", version: "1.0", spec: Spec(help: "clima [ciudad] — el clima desde wttr.in") { ctx in
            let ciudad = ctx.args.joined(separator: "+")
            let (d, _) = try await Shell.peticion("https://wttr.in/\(ciudad)?format=3&lang=es", metodo: "GET", cabeceras: ["User-Agent: curl"], cuerpo: nil, quien: ctx.sh.quienPide)
            return (String(data: d, encoding: .utf8) ?? "") + "\n"
        }, nota: "")

        r["ipinfo"] = PaqueteTermux(descripcion: "tu IP pública y su ubicación", version: "1.0", spec: Spec(help: "ipinfo — tu IP pública") { ctx in
            await ctx.sh.execute("http -b https://ipinfo.io/json")
        }, nota: "")

        // Los que en Termux se instalan pero aquí ya vienen dentro.
        for (n, cual) in [("curl", "curl"), ("wget", "wget"), ("git", "git"), ("nano", "nano (editor de líneas)"),
                          ("vim", "vi (editor de líneas)"), ("jq", "jq"), ("tree", "tree"), ("bc", "bc"),
                          ("httpie", "http"), ("json-server", "api-local"), ("termux-api", "los comandos del sistema (ayuda-sistema)"),
                          ("coreutils", "ls, cp, mv, cat, sort…"), ("bash", "sh / bash")] {
            r[n] = PaqueteTermux(descripcion: "ya incluido: \(cual)", version: "integrado", spec: nil, nota: "ya viene en SwiftShell: usa \(cual)")
        }
        // Los que iOS no deja: se explica en vez de fingir.
        for (n, cual) in [("python", "python (subconjunto: python archivo.py, python -c, python)"),
                          ("python3", "python")] {
            r[n] = PaqueteTermux(descripcion: "ya incluido: \(cual)", version: "integrado", spec: nil, nota: "ya viene en SwiftShell: usa \(cual)")
        }
        for (n, por) in [("nodejs", "no hay Node en iOS. 'js' es JavaScript de verdad (JavaScriptCore)."),
                         ("clang", "iOS no deja compilar y ejecutar binarios. Usa swift (traducido a JS)."),
                         ("openssh", "no se puede abrir un servidor SSH en el sandbox de iOS. nc/telnet sirven para TCP."),
                         ("ruby", "sin intérpretes externos en iOS. Usa js o los guiones .sh."),
                         ("php", "sin intérpretes externos en iOS. Para servir datos: api-local."),
                         ("proot", "iOS no permite proot ni chroot.")] {
            r[n] = PaqueteTermux(descripcion: "no disponible en iOS", version: "—", spec: nil, nota: por)
        }
        return r
    }()
}

extension Shell {

    // ---------- PS1, motd y .bashrc ----------

    /// Expande un PS1 de bash: \u \h \w \W \$ \t \d \n (sin colores).
    static func expandirPS1(_ ps1: String, _ env: ShellEnv) -> String {
        var s = ps1
        // quita colores: \[ \] y \e[…m / \033[…m
        s = s.replacingOccurrences(of: "\\[", with: "").replacingOccurrences(of: "\\]", with: "")
        for marca in ["\\e[", "\\033[", "\u{1B}["] {
            while let r = s.range(of: marca), let fin = s[r.upperBound...].firstIndex(of: "m") {
                s.removeSubrange(r.lowerBound ... fin)
            }
        }
        let home = env.vars["HOME"] ?? "/"
        let cwd = env.vpath(env.cwd)
        let w = cwd == home ? "~" : (home != "/" && cwd.hasPrefix(home + "/") ? "~" + cwd.dropFirst(home.count) : cwd)
        let f = DateFormatter()
        f.dateFormat = "HH:mm:ss"
        let hora = f.string(from: Date())
        f.dateFormat = "EEE d MMM"
        let fecha = f.string(from: Date())
        let pares: [(String, String)] = [
            ("\\u", env.vars["USER"] ?? "mobile"), ("\\h", "ipad"), ("\\H", "ipad"),
            ("\\w", w), ("\\W", cwd == home ? "~" : (cwd as NSString).lastPathComponent),
            ("\\$", "$"), ("\\t", hora), ("\\d", fecha), ("\\n", "\n"), ("\\s", "swiftsh"),
        ]
        for (k, v) in pares { s = s.replacingOccurrences(of: k, with: v) }
        return s
    }

    static let motdInicial = """
    Bienvenido a SwiftShell — un Termux para iOS, solo texto.

    Paquetes:   pkg search <texto>   pkg install <nombre>   pkg list-all
    Las IAs:    ia ayuda             ias                    ia libres
    API local:  api-local ejemplo    http localhost/usuarios
    Lenguajes:  python   js   swift   (y ipa / unzip para leer apps y zips)
    Ayuda:      help                 ayuda-sistema

    Edita este mensaje con:  nano /etc/motd
    Tu ~/.bashrc se ejecuta al abrir (alias, PS1, export…).

    """

    static let bashrcInicial = """
    # ~/.bashrc — se ejecuta cada vez que abres la terminal.
    # Ejemplos (quita el # para activarlos):
    # PS1='\\u@\\h:\\w \\$ '
    # alias l='ls -l'
    # export EDITOR=nano
    # neofetch

    """

    /// El mensaje de bienvenida (se crea la primera vez).
    func motd() -> String {
        guard rolIA == nil, let u = try? env.resolve("/etc/motd") else { return "" }
        let fm = FileManager.default
        if !fm.fileExists(atPath: u.path) {
            try? fm.createDirectory(at: u.deletingLastPathComponent(), withIntermediateDirectories: true)
            try? Shell.motdInicial.write(to: u, atomically: true, encoding: .utf8)
            if let b = try? env.resolve("/.bashrc"), !fm.fileExists(atPath: b.path) {
                try? Shell.bashrcInicial.write(to: b, atomically: true, encoding: .utf8)
            }
        }
        return (try? String(contentsOf: u, encoding: .utf8)) ?? ""
    }

    // ---------- comandos ----------

    static func termux() -> [String: Spec] {
        var c: [String: Spec] = [:]
        let fm = FileManager.default
        let pkgViejo = Shell.packages()["pkg"]

        func bin(_ ctx: Ctx) throws -> URL {
            let u = try ctx.env.resolve(Shell.binDir)
            try fm.createDirectory(at: u, withIntermediateDirectories: true)
            return u
        }
        func instalados(_ ctx: Ctx) -> [String] {
            guard let b = try? bin(ctx) else { return [] }
            return ((try? fm.contentsOfDirectory(atPath: b.path)) ?? []).sorted()
        }

        c["pkg"] = Spec(help: "pkg <update|upgrade|install|uninstall|search|list-all|list-installed|show> — paquetes, como en Termux") { ctx in
            let a = ctx.args.filter { $0 != "-y" && $0 != "--yes" }
            let sub = a.first?.lowercased() ?? "help"
            let nombres = Array(a.dropFirst())

            switch sub {
            case "update", "upgrade", "up":
                var out = "Obteniendo: repositorio swiftshell estable [\(RepoTermux.todos.count) paquetes]\n"
                out += "Leyendo listas de paquetes... Hecho\n"
                let n = instalados(ctx).count
                out += sub == "update" ? "Todos los paquetes están al día.\n" : "\(n) instalados, 0 para actualizar.\n"
                return out

            case "install", "i", "add":
                guard !nombres.isEmpty else { throw ShErr("uso: pkg install <paquete> [otro…]") }
                // pkg install nombre https://… → el instalador de antes (desde la red)
                if nombres.count == 2, nombres[1].contains("/") || nombres[1].contains("."), RepoTermux.paquete(nombres[0]) == nil {
                    guard let viejo = pkgViejo else { throw ShErr("pkg: no puedo instalar desde URL") }
                    return try await viejo.run(Ctx(name: "pkg", args: ["install"] + nombres, stdin: "", sh: ctx.sh))
                }
                var out = ""
                let b = try bin(ctx)
                for n in nombres {
                    if let p = RepoTermux.paquete(n) {
                        guard p.spec != nil else {
                            out += (p.version == "integrado" ? "\(n): " : "E: \(n): ") + p.nota + "\n"
                            continue
                        }
                        try "\(n) \(p.version)\n".write(to: b.appendingPathComponent("\(n).pkg"), atomically: true, encoding: .utf8)
                        out += "Instalando \(n) (\(p.version))…\nConfigurando \(n)… listo. Escribe: \(n)\n"
                    } else if let viejo = pkgViejo {
                        do {
                            out += try await viejo.run(Ctx(name: "pkg", args: ["install", n], stdin: "", sh: ctx.sh))
                        } catch {
                            out += "E: no se encuentra el paquete \(n) (prueba: pkg search \(n))\n"
                        }
                    }
                }
                return out

            case "uninstall", "remove", "rm", "purge":
                guard !nombres.isEmpty else { throw ShErr("uso: pkg uninstall <paquete>") }
                var out = ""
                let b = try bin(ctx)
                for n in nombres {
                    let archivos = instalados(ctx).filter { ($0 as NSString).deletingPathExtension == n }
                    if archivos.isEmpty { out += "E: \(n) no está instalado\n"; continue }
                    for f in archivos { try fm.removeItem(at: b.appendingPathComponent(f)) }
                    out += "Desinstalando \(n)… listo\n"
                }
                return out

            case "reinstall":
                guard let pkg = ctx.sh.commands["pkg"] else { throw ShErr("pkg no disponible") }
                return try await pkg.run(Ctx(name: "pkg", args: ["install"] + nombres, stdin: "", sh: ctx.sh))

            case "search", "s":
                let t = nombres.joined(separator: " ").lowercased()
                var out = ""
                for (n, p) in RepoTermux.todos.sorted(by: { $0.key < $1.key })
                where t.isEmpty || n.contains(t) || p.descripcion.lowercased().contains(t) {
                    out += "\(n)/estable \(p.version)\n  \(p.descripcion)\n"
                }
                if let viejo = pkgViejo, let extra = try? await viejo.run(Ctx(name: "pkg", args: ["search", t], stdin: "", sh: ctx.sh)) {
                    let guiones = extra.split(separator: "\n").filter { $0.hasPrefix("  ") }
                    if !guiones.isEmpty { out += "\nguiones (JavaScript):\n" + guiones.joined(separator: "\n") + "\n" }
                }
                return out.isEmpty ? "sin resultados para '\(t)'\n" : out

            case "list-all", "la":
                return RepoTermux.todos.sorted(by: { $0.key < $1.key })
                    .map { "\($0.key.padding(toLength: 13, withPad: " ", startingAt: 0)) \($0.value.version.padding(toLength: 10, withPad: " ", startingAt: 0)) \($0.value.descripcion)" }
                    .joined(separator: "\n") + "\n"

            case "list-installed", "li", "list":
                let l = instalados(ctx)
                guard !l.isEmpty else { return "ninguno todavía — pkg install neofetch\n" }
                return l.map { f -> String in
                    let n = (f as NSString).deletingPathExtension
                    return "\(n)/" + (f.hasSuffix(".pkg") ? "estable \(RepoTermux.paquete(n)?.version ?? "") [instalado]" : "local (\((f as NSString).pathExtension))")
                }.joined(separator: "\n") + "\n"

            case "show", "info":
                guard let n = nombres.first else { throw ShErr("uso: pkg show <paquete>") }
                guard let p = RepoTermux.paquete(n) else {
                    if let viejo = pkgViejo { return try await viejo.run(Ctx(name: "pkg", args: ["info", n], stdin: "", sh: ctx.sh)) }
                    throw ShErr("E: no existe \(n)")
                }
                let inst = instalados(ctx).contains("\(n).pkg")
                return "Package: \(n)\nVersion: \(p.version)\nEstado: \(inst ? "instalado" : (p.spec == nil ? p.nota : "no instalado"))\nDescription: \(p.descripcion)\n"

            case "files":
                guard let n = nombres.first else { throw ShErr("uso: pkg files <paquete>") }
                return instalados(ctx).filter { ($0 as NSString).deletingPathExtension == n }.map { "\(Shell.binDir)/\($0)" }.joined(separator: "\n") + "\n"

            case "clean", "autoclean":
                return "nada que limpiar\n"

            case "new":
                guard let viejo = pkgViejo else { throw ShErr("pkg new: no disponible") }
                return try await viejo.run(Ctx(name: "pkg", args: ctx.args, stdin: ctx.stdin, sh: ctx.sh))

            default:
                return """
                pkg — gestor de paquetes (como en Termux)
                  pkg update | upgrade         actualiza la lista
                  pkg install <p> [p2…]        instala (pkg install neofetch cowsay)
                  pkg uninstall <p>            desinstala
                  pkg search <texto>           busca
                  pkg list-all                 todo el repositorio
                  pkg list-installed           lo instalado
                  pkg show <p>                 detalles
                  pkg new <nombre> [swift]     crea tu propio paquete (JS o Swift)
                Tus guiones .sh en /usr/bin también se llaman por su nombre.

                """
            }
        }

        c["apt"] = Spec(help: "apt <install|remove|search|list|show|update|upgrade> — como pkg") { ctx in
            var a = ctx.args
            if a.first == "list" { a = a.contains("--installed") ? ["list-installed"] : ["list-all"] }
            guard let pkg = ctx.sh.commands["pkg"] else { throw ShErr("apt: pkg no disponible") }
            return try await pkg.run(Ctx(name: "apt", args: a, stdin: ctx.stdin, sh: ctx.sh))
        }
        c["apt-get"] = c["apt"]

        // ---------- termux-* ----------

        c["ayuda-sistema"] = Spec(help: "ayuda-sistema — comandos del sistema (estilo Termux, sin 'termux' en el nombre)") { _ in
            """
            almacenamiento            carpetas /storage (visibles en la app Archivos)
            abrir-url <url>           abre una dirección
            abrir <archivo|url>       abre o ejecuta un archivo
            toast <texto>             un aviso
            notificar -t T -c texto   notificación · notificaciones  (las últimas)
            hablar <texto>            lo dice en voz alta
            wake-lock / wake-unlock   evita que la pantalla se apague / lo suelta
            recargar                  vuelve a leer ~/.bashrc y /etc/profile.sh
            repo                      el repositorio de paquetes
            battery · clip · clipget · device · vibrate

            """
        }

        c["almacenamiento"] = Spec(help: "almacenamiento — crea /storage") { ctx in
            for d in ["/storage/shared", "/storage/downloads", "/storage/documents"] {
                try fm.createDirectory(at: try ctx.env.resolve(d), withIntermediateDirectories: true)
            }
            return "listo: /storage/shared, /storage/downloads, /storage/documents\n" +
                   "En iOS todo tu espacio ya se ve desde la app Archivos (En mi iPad → Playgrounds → Documentos/shell).\n"
        }

        c["abrir-url"] = Spec(help: "abrir-url <url> — abre una dirección") { ctx in
            guard let u = ctx.args.first else { throw ShErr("uso: abrir-url <url>") }
            return await ctx.sh.execute("open \(u)")
        }

        c["abrir"] = Spec(help: "abrir <archivo|url> — abre un archivo o dirección") { ctx in
            guard let p = ctx.args.first else { throw ShErr("uso: abrir <archivo>") }
            if p.contains("://") { return await ctx.sh.execute("open \(p)") }
            return await ctx.sh.execute("run \(p)")
        }

        c["notificar"] = Spec(help: "notificar -t <título> -c <texto> — guarda y muestra una notificación") { ctx in
            let o = opts(ctx.args, valued: ["t", "c", "title", "content"])
            let titulo = o.vals["t"] ?? o.vals["title"] ?? "SwiftShell"
            let texto = o.vals["c"] ?? o.vals["content"] ?? (o.rest.isEmpty ? ctx.stdin : o.rest.joined(separator: " "))
            let u = try ctx.env.resolve("/var/log/notificaciones")
            try fm.createDirectory(at: u.deletingLastPathComponent(), withIntermediateDirectories: true)
            SistemaIAs.agregar("\(SistemaIAs.hora())\t\(titulo)\t\(texto)\n", a: u, maxLineas: 200)
            return "🔔 \(titulo): \(texto)\n"
        }

        c["notificaciones"] = Spec(help: "notificaciones — las últimas notificaciones") { ctx in
            let u = try ctx.env.resolve("/var/log/notificaciones")
            let t = (try? String(contentsOf: u, encoding: .utf8)) ?? ""
            return t.isEmpty ? "sin notificaciones\n" : t.replacingOccurrences(of: "\t", with: "  ")
        }

        c["hablar"] = Spec(help: "hablar <texto> — lo dice en voz alta") { ctx in
            await ctx.sh.execute("say " + (ctx.args.isEmpty ? ctx.stdin : ctx.args.joined(separator: " ")))
        }

        c["wake-lock"] = Spec(help: "wake-lock — la pantalla no se apaga") { ctx in
            await ctx.sh.execute("caffeinate on")
        }
        c["wake-unlock"] = Spec(help: "wake-unlock — la pantalla puede apagarse") { ctx in
            await ctx.sh.execute("caffeinate off")
        }

        c["recargar"] = Spec(help: "recargar — vuelve a leer ~/.bashrc") { ctx in
            let r = await ctx.sh.runProfile()
            return r.isEmpty ? "ajustes recargados\n" : r
        }

        c["repo"] = Spec(help: "repo — elige el repositorio") { _ in
            "repositorio: swiftshell estable (integrado, funciona sin internet)\n"
        }

        for n in ["sms", "sms-lista", "llamada", "foto",
                  "ubicacion", "sensores", "contactos", "grabar"] {
            c[n] = Spec(help: "\(n) — no disponible en iOS") { _ in
                throw ShErr("\(n): iOS no deja que una app de Playgrounds use eso desde la terminal")
            }
        }

        return c
    }
}
