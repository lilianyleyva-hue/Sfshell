import Foundation

// ============================================================
// MARK: - apk al estilo Alpine
// ============================================================
// La lista no es de mentira: cada entrada es una pieza que la
// terminal usa de verdad. Las versiones y licencias son las de
// los componentes reales de iOS y de Swift.

struct Paquete {
    let nombre: String
    let version: String
    let origen: String
    let licencia: String
    let desc: String
    var instalado: Bool = true

    func linea(_ arch: String) -> String {
        "\(nombre)-\(version) \(arch) {\(origen)} (\(licencia))" + (instalado ? " [installed]" : "")
    }
}

extension Shell {

    static var arch: String {
        #if arch(arm64)
        return "aarch64"
        #else
        return "x86_64"
        #endif
    }

    static let baseSystem: [Paquete] = [
        Paquete(nombre: "swiftshell", version: "1.0.0-r0", origen: "swiftshell", licencia: "MIT",
                desc: "la terminal: intérprete, tuberías, redirección e historial"),
        Paquete(nombre: "swift-runtime", version: "6.0-r0", origen: "swift", licencia: "Apache-2.0 WITH LLVM-exception",
                desc: "biblioteca de ejecución del lenguaje Swift"),
        Paquete(nombre: "foundation", version: "6.0-r0", origen: "swift-corelibs", licencia: "Apache-2.0",
                desc: "cadenas, fechas, archivos, JSON y plist"),
        Paquete(nombre: "javascriptcore", version: "19.0-r0", origen: "webkit", licencia: "LGPL-2.1 AND BSD-2-Clause",
                desc: "motor de JavaScript: comandos js y swift"),
        Paquete(nombre: "swiftjs-transpiler", version: "1.2.0-r0", origen: "swiftshell", licencia: "MIT",
                desc: "traductor del subconjunto de Swift a JavaScript"),
        Paquete(nombre: "busybox-swift", version: "1.0.0-r0", origen: "swiftshell", licencia: "MIT",
                desc: "los comandos de tipo Unix escritos en Swift"),
        Paquete(nombre: "urlsession", version: "6.0-r0", origen: "apple-networking", licencia: "Apple-SDK",
                desc: "peticiones HTTP: curl, wget, api"),
        Paquete(nombre: "network-framework", version: "1.0-r0", origen: "apple-networking", licencia: "Apple-SDK",
                desc: "conexiones TCP: nc, telnet, whois"),
        Paquete(nombre: "cryptokit", version: "1.0-r0", origen: "apple-security", licencia: "Apple-SDK",
                desc: "huellas md5sum, sha1sum y sha256sum"),
        Paquete(nombre: "libcompression", version: "1.0-r0", origen: "apple-compression", licencia: "Apple-SDK",
                desc: "descompresión gzip para los .deb"),
        Paquete(nombre: "zlib", version: "1.2.12-r0", origen: "zlib", licencia: "Zlib",
                desc: "deflate, por debajo de libcompression"),
        Paquete(nombre: "ca-certificates-bundle", version: "2024.03-r0", origen: "apple-security", licencia: "MPL-2.0 AND MIT",
                desc: "certificados raíz del sistema para HTTPS"),
        Paquete(nombre: "apk-tools-swift", version: "1.0.0-r0", origen: "swiftshell", licencia: "MIT",
                desc: "este gestor de paquetes"),
        Paquete(nombre: "deb-tools", version: "1.0.0-r0", origen: "swiftshell", licencia: "MIT",
                desc: "lectura y creación de .deb: ar, tar y gzip"),
        Paquete(nombre: "swiftui", version: "6.0-r0", origen: "apple-ui", licencia: "Apple-SDK",
                desc: "la interfaz de la terminal, nano y el explorador"),
        Paquete(nombre: "xmlparser", version: "6.0-r0", origen: "swift-corelibs", licencia: "Apache-2.0",
                desc: "análisis de XML"),
        Paquete(nombre: "swiftshell-baselayout", version: "1.0.0-r0", origen: "swiftshell", licencia: "MIT",
                desc: "estructura de directorios: /etc, /usr/bin, /backup")
    ]

    static func apk() -> [String: Spec] {
        var c: [String: Spec] = [:]
        let fm = FileManager.default

        func locales(_ ctx: Ctx) -> [Paquete] {
            guard let bin = try? ctx.env.resolve(Shell.binDir),
                  let items = try? fm.contentsOfDirectory(atPath: bin.path) else { return [] }
            return items.sorted().map { f in
                let nombre = (f as NSString).deletingPathExtension
                let lenguaje = (f as NSString).pathExtension.lowercased() == "swift" ? "swift" : "js"
                return Paquete(nombre: nombre, version: "1.0-r0", origen: "local-\(lenguaje)",
                               licencia: "none", desc: "paquete instalado por ti")
            }
        }

        c["apk"] = Spec(help: "apk <list|info|search|add|del|update|stats|policy> — gestor de paquetes") { ctx in
            let sub = ctx.args.first ?? "list"
            let rest = Array(ctx.args.dropFirst())
            let todos = Shell.baseSystem + locales(ctx)

            switch sub {
            case "list", "-L", "--installed":
                let filtro = rest.first(where: { !$0.hasPrefix("-") })?.lowercased()
                let vistos = todos.filter { filtro == nil || $0.nombre.lowercased().contains(filtro!) }
                return vistos.map { $0.linea(Shell.arch) }.joined(separator: "\n") + "\n"

            case "info":
                guard let n = rest.first(where: { !$0.hasPrefix("-") }) else {
                    return todos.map { $0.nombre }.joined(separator: "\n") + "\n"
                }
                guard let p = todos.first(where: { $0.nombre == n }) else {
                    throw ShErr("apk: no hay ningún paquete llamado '\(n)'")
                }
                var out = "\(p.nombre)-\(p.version) descripción:\n\(p.desc)\n\n"
                out += "\(p.nombre)-\(p.version) origen:\n\(p.origen)\n\n"
                out += "\(p.nombre)-\(p.version) licencia:\n\(p.licencia)\n\n"
                out += "\(p.nombre)-\(p.version) arquitectura:\n\(Shell.arch)\n"
                return out

            case "search":
                let t = rest.first?.lowercased() ?? ""
                let base = todos.filter { $0.nombre.lowercased().contains(t) || $0.desc.lowercased().contains(t) }
                var out = base.map { "\($0.nombre)-\($0.version)  \($0.desc)" }.joined(separator: "\n")
                if !out.isEmpty { out += "\n" }
                out += "\nDisponibles para instalar (catálogo de scripts):\n"
                out += await ctx.sh.execute("pkg search")
                return out

            case "add":
                guard let n = rest.first else { throw ShErr("apk add: falta el nombre") }
                if Shell.baseSystem.contains(where: { $0.nombre == n }) {
                    return "\(n) ya viene con el sistema, no hace falta instalarlo\n"
                }
                return await ctx.sh.execute("pkg install \(rest.joined(separator: " "))")

            case "del":
                guard let n = rest.first else { throw ShErr("apk del: falta el nombre") }
                if Shell.baseSystem.contains(where: { $0.nombre == n }) {
                    throw ShErr("apk: '\(n)' es parte del sistema y no se puede quitar")
                }
                return await ctx.sh.execute("pkg remove \(n)")

            case "update":
                let repos = (try? ctx.input(["/etc/repos"]))?
                    .components(separatedBy: "\n").filter { !$0.isEmpty } ?? []
                if repos.isEmpty {
                    return "fetch: no hay repositorios configurados\n" +
                           "OK: \(todos.count) paquetes distintos disponibles\n" +
                           "añade uno con: repo add <url>\n"
                }
                return await ctx.sh.execute("repo update")

            case "upgrade":
                return "OK: \(Shell.baseSystem.count) paquetes del sistema, ninguno por actualizar.\n" +
                       "El sistema se actualiza recompilando la app en Swift Playgrounds.\n"

            case "stats":
                let mios = locales(ctx)
                var out = "instalados:            \(todos.count)\n"
                out += "  del sistema:         \(Shell.baseSystem.count)\n"
                out += "  tuyos:               \(mios.count)\n"
                out += "comandos disponibles:  \(ctx.sh.commands.count)\n"
                out += "arquitectura:          \(Shell.arch)\n"
                out += "directorio de paquetes: \(Shell.binDir)\n"
                return out

            case "policy":
                guard let n = rest.first else { throw ShErr("apk policy: falta el nombre") }
                guard let p = todos.first(where: { $0.nombre == n }) else {
                    throw ShErr("apk: no hay ningún paquete llamado '\(n)'")
                }
                return "\(p.nombre) policy:\n  \(p.version):\n    lib/apk/db/installed\n    origen: \(p.origen)\n"

            case "version":
                return "apk-tools-swift 1.0.0, compilado para \(Shell.arch)\n"

            default:
                throw ShErr("apk: subcomando desconocido '\(sub)'\n" +
                            "usa: list, info, search, add, del, update, upgrade, stats, policy, version")
            }
        }

        return c
    }
}
