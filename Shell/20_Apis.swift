import Foundation

// ============================================================
// MARK: - APIs: las del sistema y las tuyas
// ============================================================
// API=1 usuario · API=2 red · API=3 dispositivo · API=4 root
//
// Una API tuya es un archivo JSON en /etc/api con métodos que
// se traducen a comandos de la terminal. Los argumentos entran
// como $1, $2, $3.

struct ApiSistema {
    let nombre: String
    let capa: Int
    let desc: String
    let metodos: [String]
    let disponible: Bool
    let nota: String
}

extension Shell {

    static let apisSistema: [ApiSistema] = [
        ApiSistema(nombre: "files", capa: 1, desc: "archivos del entorno",
                   metodos: ["ls", "cat", "new", "nano", "rm", "cp", "mv", "find"],
                   disponible: true, nota: ""),
        ApiSistema(nombre: "text", capa: 1, desc: "proceso de texto",
                   metodos: ["grep", "sed", "awk", "sort", "cut", "wc"],
                   disponible: true, nota: ""),
        ApiSistema(nombre: "data", capa: 1, desc: "JSON, XML, plist y CSV",
                   metodos: ["json", "xml", "plist", "csv"],
                   disponible: true, nota: ""),
        ApiSistema(nombre: "exec", capa: 2, desc: "ejecución de código",
                   metodos: ["js", "swift", "run", "sim", "sh"],
                   disponible: true, nota: ""),
        ApiSistema(nombre: "net", capa: 2, desc: "red",
                   metodos: ["curl", "wget", "api", "nc", "whois", "nslookup", "ping", "ip"],
                   disponible: true, nota: ""),
        ApiSistema(nombre: "huella", capa: 1, desc: "Huella, la IA propia: aprende viendo",
                   metodos: ["huella"],
                   disponible: true, nota: "no usa internet: solo sabe lo que ha visto en esta terminal"),
        ApiSistema(nombre: "nyx", capa: 2, desc: "Nyx, las 18 mentes resonantes",
                   metodos: ["nyx"],
                   disponible: true, nota: "piensan en el iPad; 'nyx despierta' gasta batería"),
        ApiSistema(nombre: "canal", capa: 2, desc: "canales de datos entre procesos",
                   metodos: ["canal", "canales", "emit", "events"],
                   disponible: true, nota: ""),
        ApiSistema(nombre: "device", capa: 3, desc: "el iPad",
                   metodos: ["device", "battery", "free", "nproc", "arch", "uptime"],
                   disponible: true, nota: ""),
        ApiSistema(nombre: "clipboard", capa: 3, desc: "portapapeles del sistema",
                   metodos: ["clip", "clipget"],
                   disponible: true, nota: ""),
        ApiSistema(nombre: "haptics", capa: 3, desc: "vibración",
                   metodos: ["vibrate"],
                   disponible: true, nota: ""),
        ApiSistema(nombre: "documents", capa: 3, desc: "app Archivos",
                   metodos: ["pick", "pickFolder", "save", "files"],
                   disponible: true, nota: "el usuario elige qué se comparte, no la app"),
        ApiSistema(nombre: "packages", capa: 4, desc: "paquetes y .deb",
                   metodos: ["pkg", "apk", "deb", "dpkg", "repo"],
                   disponible: true, nota: ""),
        ApiSistema(nombre: "procs", capa: 4, desc: "procesos de Simulacro",
                   metodos: ["procs", "kill", "freeze", "thaw", "wipe"],
                   disponible: true, nota: "son entradas en una lista, no procesos del sistema"),
        ApiSistema(nombre: "camera", capa: 3, desc: "cámara",
                   metodos: [], disponible: false,
                   nota: "necesita permisos declarados al compilar la app"),
        ApiSistema(nombre: "location", capa: 3, desc: "ubicación",
                   metodos: [], disponible: false,
                   nota: "igual que la cámara: hay que pedir el permiso en la app"),
        ApiSistema(nombre: "kernel", capa: 4, desc: "núcleo del sistema",
                   metodos: [], disponible: false,
                   nota: "cerrado en iPadOS; es justo lo que abre un jailbreak"),
        ApiSistema(nombre: "hardware", capa: 4, desc: "USB, wifi, puertos",
                   metodos: [], disponible: false,
                   nota: "sin API pública para apps")
    ]

    static func apis() -> [String: Spec] {
        var c: [String: Spec] = [:]
        let fm = FileManager.default

        func dir(_ ctx: Ctx) throws -> URL {
            let u = try ctx.env.resolve("/etc/api")
            try fm.createDirectory(at: u, withIntermediateDirectories: true)
            return u
        }

        c["sysapi"] = Spec(help: "sysapi <list|get|capa N> — APIs del sistema por capas") { ctx in
            let sub = ctx.args.first ?? "list"
            switch sub {
            case "list":
                var out = ""
                for capa in 1...4 {
                    let grupo = Shell.apisSistema.filter { $0.capa == capa }
                    guard !grupo.isEmpty else { continue }
                    let etiqueta = ["", "usuario", "ejecución y red", "dispositivo", "root del entorno"][capa]
                    out += "\nCAPA \(capa) — \(etiqueta)\n"
                    for a in grupo {
                        let marca = a.disponible ? "·" : "✕"
                        out += "  \(marca) \(a.nombre.padding(toLength: 12, withPad: " ", startingAt: 0)) \(a.desc)\n"
                    }
                }
                out += "\n'sysapi get <nombre>' para ver sus métodos.\n"
                return out

            case "capa":
                guard let n = ctx.args.dropFirst().first.flatMap({ Int($0) }), (1...4).contains(n) else {
                    throw ShErr("sysapi capa: pon un número del 1 al 4")
                }
                let grupo = Shell.apisSistema.filter { $0.capa == n }
                return grupo.map { "\($0.nombre): \($0.metodos.joined(separator: " "))" }.joined(separator: "\n") + "\n"

            case "get":
                guard let n = ctx.args.dropFirst().first,
                      let a = Shell.apisSistema.first(where: { $0.nombre == n }) else {
                    throw ShErr("sysapi get: no existe esa API. Mira 'sysapi list'.")
                }
                var out = "\(a.nombre) — capa \(a.capa)\n\(a.desc)\n\n"
                if a.disponible {
                    out += "métodos: \(a.metodos.joined(separator: ", "))\n"
                    out += "llámalos como comandos, o desde una API tuya.\n"
                } else {
                    out += "no disponible: \(a.nota)\n"
                }
                if a.disponible && !a.nota.isEmpty { out += "nota: \(a.nota)\n" }
                return out

            default:
                throw ShErr("sysapi: usa list, get <nombre> o capa <n>")
            }
        }

        c["apidef"] = Spec(help: "apidef <new|list|show|call|remove> — tus propias APIs") { ctx in
            let d = try dir(ctx)
            let sub = ctx.args.first ?? "list"
            let rest = Array(ctx.args.dropFirst())

            switch sub {
            case "list":
                let items = ((try? fm.contentsOfDirectory(atPath: d.path)) ?? []).sorted()
                if items.isEmpty { return "no hay APIs tuyas. Crea una con: apidef new <nombre>\n" }
                var out = ""
                for f in items {
                    let n = (f as NSString).deletingPathExtension
                    var linea = n
                    if let dat = fm.contents(atPath: d.appendingPathComponent(f).path),
                       let j = try? JSONSerialization.jsonObject(with: dat) as? [String: Any] {
                        let capa = (j["capa"] as? Int) ?? 1
                        let ms = (j["metodos"] as? [String: String])?.keys.sorted() ?? []
                        linea += "  capa \(capa)  [\(ms.joined(separator: ", "))]"
                    }
                    out += linea + "\n"
                }
                return out

            case "new":
                guard let n = rest.first else { throw ShErr("apidef new: falta el nombre") }
                let u = d.appendingPathComponent("\(n).json")
                guard !fm.fileExists(atPath: u.path) else { throw ShErr("apidef: '\(n)' ya existe") }
                let plantilla: [String: Any] = [
                    "nombre": n,
                    "capa": 1,
                    "desc": "API creada por mí",
                    "metodos": [
                        "saludo": "echo hola $1",
                        "leer": "cat $1",
                        "buscar": "grep $1 $2"
                    ]
                ]
                let dat = try JSONSerialization.data(withJSONObject: plantilla, options: [.prettyPrinted, .sortedKeys])
                try dat.write(to: u)
                ctx.sh.uiEdit?(u)
                return "creada /etc/api/\(n).json — se abre el editor\n" +
                       "llámala con: apidef call \(n) saludo mundo\n"

            case "show":
                guard let n = rest.first else { throw ShErr("apidef show: falta el nombre") }
                let u = d.appendingPathComponent("\(n).json")
                guard let dat = fm.contents(atPath: u.path) else { throw ShErr("apidef: '\(n)' no existe") }
                return (String(data: dat, encoding: .utf8) ?? "") + "\n"

            case "remove":
                guard let n = rest.first else { throw ShErr("apidef remove: falta el nombre") }
                let u = d.appendingPathComponent("\(n).json")
                guard fm.fileExists(atPath: u.path) else { throw ShErr("apidef: '\(n)' no existe") }
                try fm.removeItem(at: u)
                return "borrada \(n)\n"

            case "call":
                guard rest.count >= 2 else { throw ShErr("apidef call: usa apidef call <api> <método> [args]") }
                let n = rest[0], metodo = rest[1]
                let args = Array(rest.dropFirst(2))
                let u = d.appendingPathComponent("\(n).json")
                guard let dat = fm.contents(atPath: u.path),
                      let j = try? JSONSerialization.jsonObject(with: dat) as? [String: Any] else {
                    throw ShErr("apidef: '\(n)' no existe o su JSON está mal")
                }
                let capa = (j["capa"] as? Int) ?? 1
                guard let metodos = j["metodos"] as? [String: String], let plantilla = metodos[metodo] else {
                    let hay = (j["metodos"] as? [String: String])?.keys.sorted().joined(separator: ", ") ?? ""
                    throw ShErr("apidef: '\(n)' no tiene el método '\(metodo)'. Tiene: \(hay)")
                }
                if capa >= 4 && ctx.env.vars["ROOT"] != "1" {
                    throw ShErr("apidef: '\(n)' es de capa 4; activa el modo root con: export ROOT=1")
                }
                var cmd = plantilla
                for (i, a) in args.enumerated() {
                    cmd = cmd.replacingOccurrences(of: "$\(i + 1)", with: a)
                }
                // los huecos que sobren se quedan vacíos
                for i in (args.count + 1)...9 {
                    cmd = cmd.replacingOccurrences(of: "$\(i)", with: "")
                }
                return await ctx.sh.execute(cmd.trimmingCharacters(in: .whitespaces))

            default:
                throw ShErr("apidef: usa new, list, show, call o remove")
            }
        }

        c["API"] = Spec(help: "API — atajo de sysapi list") { ctx in
            await ctx.sh.execute("sysapi " + (ctx.args.isEmpty ? "list" : ctx.args.joined(separator: " ")))
        }

        return c
    }
}
