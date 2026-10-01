import Foundation
#if os(Linux) && canImport(FoundationNetworking)
import FoundationNetworking
#endif
#if canImport(Network)
import Network
#endif
#if canImport(UIKit)
import UIKit
#endif

// ============================================================
// MARK: - Comandos al estilo Blink y Termux, más algunos extras
// ============================================================

final class Once: @unchecked Sendable { var done = false }

#if canImport(Network)
enum TCP {
    /// Abre una conexión TCP, manda un texto y devuelve lo que llegue.
    static func request(host: String, port: UInt16, send: String, timeout: Double = 15) async throws -> String {
        try await withCheckedThrowingContinuation { cont in
            guard let p = NWEndpoint.Port(rawValue: port) else {
                cont.resume(throwing: ShErr("puerto no válido: \(port)")); return
            }
            let conn = NWConnection(host: NWEndpoint.Host(host), port: p, using: .tcp)
            let box = OutBox()
            let once = Once()

            @Sendable func finish(_ result: Result<String, Error>) {
                if once.done { return }
                once.done = true
                conn.cancel()
                switch result {
                case .success(let s): cont.resume(returning: s)
                case .failure(let e): cont.resume(throwing: e)
                }
            }

            @Sendable func receive() {
                conn.receive(minimumIncompleteLength: 1, maximumLength: 65536) { data, _, isComplete, error in
                    if let d = data, !d.isEmpty {
                        box.s += String(data: d, encoding: .utf8) ?? ""
                    }
                    if isComplete || error != nil { finish(.success(box.s)) } else { receive() }
                }
            }

            conn.stateUpdateHandler = { state in
                switch state {
                case .ready:
                    if !send.isEmpty {
                        conn.send(content: Data(send.utf8), completion: .contentProcessed { _ in })
                    }
                    receive()
                case .failed(let e):
                    finish(.failure(ShErr("no se pudo conectar: \(e.localizedDescription)")))
                case .cancelled:
                    finish(.success(box.s))
                default:
                    break
                }
            }
            conn.start(queue: .global())
            DispatchQueue.global().asyncAfter(deadline: .now() + timeout) {
                finish(.success(box.s))
            }
        }
    }
}
#else
enum TCP {
    static func request(host: String, port: UInt16, send: String, timeout: Double = 15) async throws -> String {
        throw ShErr("nc: sin conexiones TCP directas en este sistema")
    }
}
#endif

extension Shell {

    static func more() -> [String: Spec] {
        var c: [String: Spec] = [:]

        // ========== Blink: red y terminal ==========

        c["nc"] = Spec(help: "nc <host> <puerto> — abre una conexión TCP y manda lo que llegue por la tubería") { ctx in
            guard ctx.args.count >= 2, let port = UInt16(ctx.args[1]) else {
                throw ShErr("nc: uso: nc host puerto")
            }
            return try await TCP.request(host: ctx.args[0], port: port, send: ctx.stdin)
        }
        c["telnet"] = c["nc"]

        c["whois"] = Spec(help: "whois <dominio> — consulta el registro del dominio") { ctx in
            guard let d = ctx.args.first else { throw ShErr("whois: falta el dominio") }
            var out = try await TCP.request(host: "whois.iana.org", port: 43, send: d + "\r\n")
            // si el registro apunta a otro servidor, preguntamos allí también
            for line in out.components(separatedBy: "\n") where line.lowercased().hasPrefix("refer:") {
                let server = line.dropFirst(6).trimmingCharacters(in: .whitespaces)
                if !server.isEmpty {
                    let second = try? await TCP.request(host: server, port: 43, send: d + "\r\n")
                    if let s = second, !s.isEmpty { out += "\n--- \(server) ---\n" + s }
                }
                break
            }
            return out
        }

        c["ip"] = Spec(help: "ip — tu dirección IP pública") { _ in
            guard let url = URL(string: "https://api.ipify.org") else { throw ShErr("ip: error interno") }
            let (d, _) = try await URLSession.shared.data(from: url)
            return (String(data: d, encoding: .utf8) ?? "") + "\n"
        }
        c["myip"] = c["ip"]

        c["weather"] = Spec(help: "weather [ciudad] — el tiempo desde wttr.in") { ctx in
            let city = ctx.args.joined(separator: "+")
            let s = "https://wttr.in/\(city)?format=3&lang=es"
            guard let url = URL(string: s) else { throw ShErr("weather: ciudad no válida") }
            let (d, _) = try await URLSession.shared.data(from: url)
            return (String(data: d, encoding: .utf8) ?? "") + "\n"
        }

        c["open"] = Spec(help: "open <url> — abre una dirección en Safari") { ctx in
            guard let s = ctx.args.first else { throw ShErr("open: falta la dirección") }
            let full = s.contains("://") ? s : "https://" + s
            guard let url = URL(string: full) else { throw ShErr("open: dirección no válida") }
            #if canImport(UIKit)
            await MainActor.run { UIApplication.shared.open(url) }
            #endif
            return "abriendo \(full)\n"
        }
        c["link"] = c["open"]

        c["config"] = Spec(help: "config — resumen de cómo está montada la terminal") { ctx in
            var out = "shell      swiftshell (Swift puro)\n"
            out += "comandos   \(ctx.sh.commands.count)\n"
            out += "modo       \(ctx.sh.mode.rawValue)\n"
            out += "raíz       \(ctx.env.vpath(ctx.env.root))  →  \(ctx.env.root.path)\n"
            out += "actual     \(ctx.env.vpath(ctx.env.cwd))\n"
            out += "motores    JavaScriptCore (js) · traductor a JS (swift)\n"
            out += "red        URLSession (curl, wget, api) · TCP (nc, whois)\n"
            return out
        }

        c["ssh"] = Spec(help: "ssh — no disponible") { _ in
            throw ShErr("ssh: iOS no permite abrir sesiones SSH desde una app así.\n" +
                        "Para conexiones tienes 'nc', 'curl', 'api' y 'whois'.")
        }
        c["scp"] = c["ssh"]
        c["sftp"] = c["ssh"]

        // ========== Termux ==========

        c["pkg"] = Spec(help: "pkg — gestor de paquetes (aquí no hay)") { _ in
            "pkg: no hay repositorios. Todos los comandos vienen dentro de la app; usa 'busybox' para verlos.\n"
        }

        c["battery"] = Spec(help: "battery — estado de la batería") { _ in
            #if canImport(UIKit)
            return await MainActor.run {
                let d = UIDevice.current
                d.isBatteryMonitoringEnabled = true
                let pct = d.batteryLevel < 0 ? "desconocido" : "\(Int(d.batteryLevel * 100))%"
                let estado: String
                switch d.batteryState {
                case .charging: estado = "cargando"
                case .full: estado = "llena"
                case .unplugged: estado = "desenchufada"
                default: estado = "desconocido"
                }
                return "nivel   \(pct)\nestado  \(estado)\n"
            }
            #else
            return "battery: no disponible en este sistema\n"
            #endif
        }
        c["termux-battery-status"] = c["battery"]

        c["clip"] = Spec(help: "clip — copia la entrada al portapapeles") { ctx in
            let t = ctx.stdin.isEmpty ? ctx.args.joined(separator: " ") : ctx.stdin
            guard !t.isEmpty else { throw ShErr("clip: no hay nada que copiar") }
            await Plataforma.copiar(t)
            return "copiados \(t.count) caracteres\n"
        }
        c["termux-clipboard-set"] = c["clip"]

        c["clipget"] = Spec(help: "clipget — pega el contenido del portapapeles") { _ in
            let t = await Plataforma.pegar()
            return t.isEmpty ? "(portapapeles vacío)\n" : t + "\n"
        }
        c["termux-clipboard-get"] = c["clipget"]

        c["vibrate"] = Spec(help: "vibrate — hace vibrar el dispositivo") { _ in
            #if canImport(UIKit)
            await MainActor.run {
                UIImpactFeedbackGenerator(style: .medium).impactOccurred()
            }
            #endif
            return ""
        }
        c["termux-vibrate"] = c["vibrate"]

        c["device"] = Spec(help: "device — información del iPad") { _ in
            let d = await Plataforma.dispositivo()
            var out = "nombre    \(d.nombre)\n"
            out += "modelo    \(d.modelo)\n"
            out += "sistema   \(d.sistema) \(d.version)\n"
            out += "pantalla  \(d.pantalla)\n"
            out += "núcleos   \(ProcessInfo.processInfo.activeProcessorCount)\n"
            return out
        }
        c["termux-info"] = c["device"]

        c["toast"] = Spec(help: "toast <texto> — muestra un aviso en la terminal") { ctx in
            let t = ctx.args.joined(separator: " ")
            guard !t.isEmpty else { throw ShErr("toast: falta el texto") }
            return "┌\(String(repeating: "─", count: t.count + 2))┐\n│ \(t) │\n└\(String(repeating: "─", count: t.count + 2))┘\n"
        }

        // ========== Extras ==========

        c["csv"] = Spec(help: "csv <tojson|cols|head> [archivo] — trabaja con archivos CSV") { ctx in
            guard let sub = ctx.args.first else { throw ShErr("csv: uso: csv tojson|cols|head [archivo]") }
            let lines = try ctx.lines(Array(ctx.args.dropFirst()))
            guard let header = lines.first else { throw ShErr("csv: el archivo está vacío") }
            let cols = header.components(separatedBy: ",").map { $0.trimmingCharacters(in: .whitespaces) }
            switch sub {
            case "cols":
                return cols.enumerated().map { "\($0.offset + 1)  \($0.element)" }.joined(separator: "\n") + "\n"
            case "head":
                return lines.prefix(6).joined(separator: "\n") + "\n"
            case "tojson":
                var rows: [[String: String]] = []
                for l in lines.dropFirst() where !l.trimmingCharacters(in: .whitespaces).isEmpty {
                    let vals = l.components(separatedBy: ",")
                    var row: [String: String] = [:]
                    for (i, k) in cols.enumerated() {
                        row[k] = i < vals.count ? vals[i].trimmingCharacters(in: .whitespaces) : ""
                    }
                    rows.append(row)
                }
                let d = try JSONSerialization.data(withJSONObject: rows, options: [.prettyPrinted, .sortedKeys])
                return (String(data: d, encoding: .utf8) ?? "") + "\n"
            default:
                throw ShErr("csv: subcomando desconocido '\(sub)'")
            }
        }

        c["urlencode"] = Spec(help: "urlencode <texto> — codifica para una URL") { ctx in
            let t = ctx.args.isEmpty ? ctx.stdin : ctx.args.joined(separator: " ")
            let allowed = CharacterSet.alphanumerics.union(CharacterSet(charactersIn: "-._~"))
            return (t.trimmingCharacters(in: .newlines).addingPercentEncoding(withAllowedCharacters: allowed) ?? "") + "\n"
        }

        c["urldecode"] = Spec(help: "urldecode <texto> — descodifica una URL") { ctx in
            let t = ctx.args.isEmpty ? ctx.stdin : ctx.args.joined(separator: " ")
            return (t.trimmingCharacters(in: .newlines).removingPercentEncoding ?? "") + "\n"
        }

        c["uuid"] = Spec(help: "uuid [n] — genera identificadores únicos") { ctx in
            let n = min(100, Int(ctx.args.first ?? "1") ?? 1)
            return (0..<n).map { _ in UUID().uuidString }.joined(separator: "\n") + "\n"
        }

        c["now"] = Spec(help: "now — fecha, hora y marca de tiempo") { _ in
            let d = Date()
            let df = DateFormatter()
            df.dateFormat = "yyyy-MM-dd HH:mm:ss"
            return "\(df.string(from: d))\nepoch \(Int(d.timeIntervalSince1970))\n"
        }

        c["rand"] = Spec(help: "rand [min] [max] [-n N] — números al azar") { ctx in
            let o = opts(ctx.args, valued: ["n"])
            let nums = o.rest.compactMap { Int($0) }
            let lo = nums.count > 1 ? nums[0] : 1
            let hi = nums.count > 1 ? nums[1] : (nums.first ?? 100)
            guard lo <= hi else { throw ShErr("rand: el mínimo es mayor que el máximo") }
            let count = min(1000, Int(o.vals["n"] ?? "1") ?? 1)
            return (0..<count).map { _ in String(Int.random(in: lo...hi)) }.joined(separator: "\n") + "\n"
        }

        c["freq"] = Spec(help: "freq [archivo...] — cuenta cuántas veces aparece cada palabra") { ctx in
            let t = try ctx.input(ctx.args)
            var counts: [String: Int] = [:]
            for w in t.lowercased().split(whereSeparator: { !$0.isLetter && !$0.isNumber }) {
                counts[String(w), default: 0] += 1
            }
            return counts.sorted { $0.value == $1.value ? $0.key < $1.key : $0.value > $1.value }
                .prefix(40)
                .map { "\(String($0.value).leftPad(6))  \($0.key)" }
                .joined(separator: "\n") + "\n"
        }

        c["note"] = Spec(help: "note <texto> — apunta una línea en notas.md; sin texto, las muestra") { ctx in
            let u = try ctx.env.resolve("/notas.md")
            if ctx.args.isEmpty {
                guard let d = FileManager.default.contents(atPath: u.path) else { return "(no hay notas)\n" }
                return String(data: d, encoding: .utf8) ?? ""
            }
            let df = DateFormatter()
            df.dateFormat = "yyyy-MM-dd HH:mm"
            let line = "- [\(df.string(from: Date()))] \(ctx.args.joined(separator: " "))\n"
            let old = (FileManager.default.contents(atPath: u.path)).flatMap { String(data: $0, encoding: .utf8) } ?? ""
            try (old + line).write(to: u, atomically: true, encoding: .utf8)
            return "apuntado\n"
        }

        c["repeat"] = Spec(help: "repeat <n> <comando> — repite un comando n veces") { ctx in
            guard let n = Int(ctx.args.first ?? ""), n > 0, n <= 100, ctx.args.count > 1 else {
                throw ShErr("repeat: uso: repeat 5 'echo hola'  (máximo 100)")
            }
            let cmd = ctx.args.dropFirst().joined(separator: " ")
            var out = ""
            for _ in 0..<n { out += await ctx.sh.execute(cmd) }
            return out
        }

        c["watch"] = Spec(help: "watch [-n seg] [-c veces] <comando> — repite un comando cada cierto tiempo") { ctx in
            let o = opts(ctx.args, valued: ["n", "c"])
            let wait = min(10.0, Double(o.vals["n"] ?? "1") ?? 1)
            let times = min(20, Int(o.vals["c"] ?? "5") ?? 5)
            guard !o.rest.isEmpty else { throw ShErr("watch: falta el comando") }
            let cmd = o.rest.joined(separator: " ")
            var out = ""
            for i in 1...times {
                out += "--- \(i)/\(times) ---\n" + (await ctx.sh.execute(cmd))
                if i < times { try await Task.sleep(nanoseconds: UInt64(wait * 1_000_000_000)) }
            }
            return out
        }

        return c
    }
}
