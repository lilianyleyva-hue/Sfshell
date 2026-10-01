import Foundation
#if canImport(FoundationNetworking)
import FoundationNetworking
#endif
#if canImport(CryptoKit)
import CryptoKit
#endif

// ============================================================
// MARK: - Comandos al estilo iSH / BusyBox
// ============================================================

extension Shell {

    static func ish() -> [String: Spec] {
        var c: [String: Spec] = [:]
        let fm = FileManager.default

        // --- guiones y control ------------------------------------

        c["sh"] = Spec(help: "sh -c '<comando>' | sh <guion> — ejecuta comandos del shell") { ctx in
            if ctx.args.first == "-c" {
                return await ctx.sh.execute(ctx.args.dropFirst().joined(separator: " "))
            }
            guard let p = ctx.args.first else { throw ShErr("sh: falta el guion o -c") }
            var out = ""
            for line in try ctx.lines([p]) {
                let t = line.trimmingCharacters(in: .whitespaces)
                if t.isEmpty || t.hasPrefix("#") { continue }
                out += await ctx.sh.execute(t)
            }
            return out
        }

        c["source"] = Spec(help: "source <guion> — ejecuta un guion en el shell actual") { ctx in
            guard let p = ctx.args.first else { throw ShErr("source: falta el guion") }
            var out = ""
            for line in try ctx.lines([p]) {
                let t = line.trimmingCharacters(in: .whitespaces)
                if t.isEmpty || t.hasPrefix("#") { continue }
                out += await ctx.sh.execute(t)
            }
            return out
        }
        c["."] = c["source"]

        c["time"] = Spec(help: "time <comando> — mide cuánto tarda") { ctx in
            guard !ctx.args.isEmpty else { throw ShErr("time: falta el comando") }
            let start = Date()
            let out = await ctx.sh.execute(ctx.args.joined(separator: " "))
            let ms = Date().timeIntervalSince(start) * 1000
            return out + String(format: "\nreal  %.1f ms\n", ms)
        }

        c["exit"] = Spec(help: "exit — aquí no cierra nada; usa el botón de inicio") { _ in
            "esta terminal vive dentro de la app: no hay proceso que cerrar.\n"
        }

        c["reset"] = Spec(help: "reset — limpia la pantalla y reinicia el motor JavaScript") { ctx in
            ctx.sh.js.reset()
            ctx.sh.mode = .shell
            ctx.sh.buffer = []
            ctx.sh.uiClear?()
            return ""
        }

        // --- inventario del sistema --------------------------------

        c["busybox"] = Spec(help: "busybox — lista todos los comandos integrados") { ctx in
            let names = ctx.sh.commands.keys.sorted()
            return "SwiftShell multi-call binary\n\nComandos (\(names.count)):\n" +
                   names.joined(separator: ", ") + "\n"
        }

        c["apk"] = Spec(help: "apk — gestor de paquetes (aquí no hay)") { _ in
            "apk: no hay paquetes que instalar.\n" +
            "Esta terminal está escrita en Swift y todos sus comandos vienen dentro.\n" +
            "Usa 'busybox' para ver la lista completa.\n"
        }

        c["arch"] = Spec(help: "arch — arquitectura del procesador") { _ in
            #if arch(arm64)
            return "aarch64\n"
            #else
            return "x86_64\n"
            #endif
        }

        c["nproc"] = Spec(help: "nproc — número de núcleos") { _ in
            "\(ProcessInfo.processInfo.activeProcessorCount)\n"
        }

        c["ps"] = Spec(help: "ps — procesos (aquí solo existe el shell)") { ctx in
            let up = Int(ProcessInfo.processInfo.systemUptime)
            var out = "  PID  TIEMPO  COMANDO\n"
            out += "    1  \(String(up / 60).leftPad(5))m  swiftshell\n"
            out += "    2      0m  \(ctx.sh.mode == .shell ? "sh" : ctx.sh.mode.rawValue)\n"
            return out
        }

        c["kill"] = Spec(help: "kill <pid> — no hay procesos que matar") { _ in
            "kill: no hay procesos reales; usa 'reset' para reiniciar el intérprete.\n"
        }

        c["tty"] = Spec(help: "tty — nombre del terminal") { _ in "/dev/swiftshell\n" }

        c["groups"] = Spec(help: "groups — grupos del usuario") { _ in "mobile staff\n" }

        c["logname"] = Spec(help: "logname — nombre de acceso") { ctx in
            (ctx.env.vars["USER"] ?? "mobile") + "\n"
        }

        c["sync"] = Spec(help: "sync — vuelca los cambios a disco") { _ in "" }

        // --- permisos y enlaces ------------------------------------

        c["chmod"] = Spec(help: "chmod <modo octal> <archivo...> — cambia los permisos") { ctx in
            guard ctx.args.count >= 2, let mode = Int(ctx.args[0], radix: 8) else {
                throw ShErr("chmod: uso: chmod 644 archivo")
            }
            for p in ctx.args.dropFirst() {
                let u = try ctx.env.resolve(p)
                guard ctx.env.exists(u) else { throw ShErr("chmod: \(p): no existe") }
                try fm.setAttributes([.posixPermissions: NSNumber(value: mode)], ofItemAtPath: u.path)
            }
            return ""
        }

        c["ln"] = Spec(help: "ln [-s] <destino> <nombre> — crea un enlace") { ctx in
            let o = opts(ctx.args)
            guard o.rest.count >= 2 else { throw ShErr("ln: uso: ln [-s] destino nombre") }
            let target = try ctx.env.resolve(o.rest[0])
            let link = try ctx.env.resolve(o.rest[1])
            guard ctx.env.exists(target) else { throw ShErr("ln: \(o.rest[0]): no existe") }
            if ctx.env.exists(link) { throw ShErr("ln: \(o.rest[1]): ya existe") }
            if o.flags.contains("s") {
                try fm.createSymbolicLink(at: link, withDestinationURL: target)
            } else {
                try fm.linkItem(at: target, to: link)
            }
            return ""
        }

        c["readlink"] = Spec(help: "readlink <enlace> — a dónde apunta un enlace") { ctx in
            guard let p = ctx.args.first else { throw ShErr("readlink: falta el enlace") }
            let u = try ctx.env.resolve(p)
            let dest = try fm.destinationOfSymbolicLink(atPath: u.path)
            return dest + "\n"
        }

        // --- huellas -----------------------------------------------

        func hashCmd(_ name: String, _ fn: @escaping (Data) -> String) -> Spec {
            Spec(help: "\(name) [archivo...] — huella \(name.replacingOccurrences(of: "sum", with: ""))") { ctx in
                if ctx.args.isEmpty { return fn(Data(ctx.stdin.utf8)) + "  -\n" }
                var out = ""
                for p in ctx.args {
                    let u = try ctx.env.resolve(p)
                    guard let d = fm.contents(atPath: u.path) else { throw ShErr("\(name): \(p): no se puede leer") }
                    out += fn(d) + "  \(p)\n"
                }
                return out
            }
        }
        #if canImport(CryptoKit)
        c["md5sum"] = hashCmd("md5sum") { Insecure.MD5.hash(data: $0).map { String(format: "%02x", $0) }.joined() }
        c["sha1sum"] = hashCmd("sha1sum") { Insecure.SHA1.hash(data: $0).map { String(format: "%02x", $0) }.joined() }
        c["sha256sum"] = hashCmd("sha256sum") { SHA256.hash(data: $0).map { String(format: "%02x", $0) }.joined() }
        #else
        c["fnvsum"] = hashCmd("fnvsum") { Plataforma.fnv64($0) }
        #endif

        // --- texto y archivos --------------------------------------

        c["split"] = Spec(help: "split [-l N] <archivo> — parte un archivo en trozos xaa, xab…") { ctx in
            let o = opts(ctx.args, valued: ["l"])
            guard let p = o.rest.first else { throw ShErr("split: falta el archivo") }
            let n = max(1, Int(o.vals["l"] ?? "1000") ?? 1000)
            let lines = try ctx.lines([p])
            var out = ""
            var idx = 0
            var i = 0
            let letters = Array("abcdefghijklmnopqrstuvwxyz")
            while i < lines.count {
                let chunk = lines[i..<min(i + n, lines.count)].joined(separator: "\n") + "\n"
                let name = "x" + String(letters[idx / 26]) + String(letters[idx % 26])
                try chunk.write(to: try ctx.env.resolve(name), atomically: true, encoding: .utf8)
                out += "\(name)\n"
                i += n; idx += 1
                if idx >= 26 * 26 { break }
            }
            return out
        }

        c["paste"] = Spec(help: "paste [-d sep] <archivo1> <archivo2> — une líneas en columnas") { ctx in
            let o = opts(ctx.args, valued: ["d"])
            guard o.rest.count >= 2 else { throw ShErr("paste: hacen falta dos archivos") }
            let d = o.vals["d"] ?? "\t"
            let a = try ctx.lines([o.rest[0]])
            let b = try ctx.lines([o.rest[1]])
            var out = ""
            for i in 0..<max(a.count, b.count) {
                out += (i < a.count ? a[i] : "") + d + (i < b.count ? b[i] : "") + "\n"
            }
            return out
        }

        c["comm"] = Spec(help: "comm <archivo1> <archivo2> — qué líneas comparten y cuáles no") { ctx in
            guard ctx.args.count >= 2 else { throw ShErr("comm: hacen falta dos archivos") }
            let a = Set(try ctx.lines([ctx.args[0]]))
            let b = Set(try ctx.lines([ctx.args[1]]))
            var out = "solo en \(ctx.args[0]):\n"
            for l in a.subtracting(b).sorted() { out += "  \(l)\n" }
            out += "solo en \(ctx.args[1]):\n"
            for l in b.subtracting(a).sorted() { out += "  \(l)\n" }
            out += "en los dos:\n"
            for l in a.intersection(b).sorted() { out += "  \(l)\n" }
            return out
        }

        c["expand"] = Spec(help: "expand [-t N] [archivo...] — tabuladores a espacios") { ctx in
            let o = opts(ctx.args, valued: ["t"])
            let n = max(1, Int(o.vals["t"] ?? "8") ?? 8)
            let t = try ctx.input(o.rest)
            return t.replacingOccurrences(of: "\t", with: String(repeating: " ", count: n))
        }

        c["unexpand"] = Spec(help: "unexpand [-t N] [archivo...] — espacios a tabuladores") { ctx in
            let o = opts(ctx.args, valued: ["t"])
            let n = max(1, Int(o.vals["t"] ?? "8") ?? 8)
            let t = try ctx.input(o.rest)
            return t.replacingOccurrences(of: String(repeating: " ", count: n), with: "\t")
        }

        c["od"] = Spec(help: "od [archivo...] — volcado en octal") { ctx in
            let bytes = Array(try ctx.input(ctx.args).utf8)
            var out = ""
            for off in stride(from: 0, to: bytes.count, by: 16) {
                let chunk = bytes[off..<min(off + 16, bytes.count)]
                out += String(format: "%07o ", off) + chunk.map { String(format: "%03o", $0) }.joined(separator: " ") + "\n"
            }
            out += String(format: "%07o\n", bytes.count)
            return out
        }

        c["truncate"] = Spec(help: "truncate -s <bytes> <archivo> — recorta o rellena un archivo") { ctx in
            let o = opts(ctx.args, valued: ["s"])
            guard let p = o.rest.first, let size = Int(o.vals["s"] ?? "") else {
                throw ShErr("truncate: uso: truncate -s 100 archivo")
            }
            let u = try ctx.env.resolve(p)
            var d = fm.contents(atPath: u.path) ?? Data()
            if d.count > size { d = d.prefix(size) }
            else { d.append(Data(repeating: 0, count: size - d.count)) }
            try d.write(to: u)
            return ""
        }

        c["shuf"] = Spec(help: "shuf [-n N] [archivo...] — baraja las líneas") { ctx in
            let o = opts(ctx.args, valued: ["n"])
            var lines = try ctx.lines(o.rest)
            lines.shuffle()
            if let n = Int(o.vals["n"] ?? "") { lines = Array(lines.prefix(n)) }
            return lines.joined(separator: "\n") + "\n"
        }

        c["factor"] = Spec(help: "factor <número...> — descompone en factores primos") { ctx in
            var out = ""
            for a in ctx.args {
                guard var n = Int(a), n > 0 else { throw ShErr("factor: '\(a)' no es un número positivo") }
                var fs: [Int] = []
                var d = 2
                while d * d <= n {
                    while n % d == 0 { fs.append(d); n /= d }
                    d += 1
                }
                if n > 1 { fs.append(n) }
                out += "\(a): " + fs.map(String.init).joined(separator: " ") + "\n"
            }
            return out
        }

        c["bc"] = Spec(help: "bc <expresión> — calculadora") { ctx in
            let e = ctx.args.joined(separator: " ")
            let src = e.isEmpty ? ctx.stdin : e
            guard !src.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
                throw ShErr("bc: falta la expresión")
            }
            return try ctx.sh.js.eval("String(\(src))")
        }

        // --- red ---------------------------------------------------

        c["nslookup"] = Spec(help: "nslookup <dominio> — resuelve un nombre a direcciones IP") { ctx in
            guard let host = ctx.args.first else { throw ShErr("nslookup: falta el dominio") }
            var hints = addrinfo()
            hints.ai_family = AF_UNSPEC
            #if os(Linux)
            hints.ai_socktype = Int32(SOCK_STREAM.rawValue)
            #else
            hints.ai_socktype = SOCK_STREAM
            #endif
            var info: UnsafeMutablePointer<addrinfo>?
            let status = getaddrinfo(host, nil, &hints, &info)
            guard status == 0, let start = info else {
                throw ShErr("nslookup: no se pudo resolver '\(host)'")
            }
            defer { freeaddrinfo(info) }
            var out = "Nombre: \(host)\n"
            var node: UnsafeMutablePointer<addrinfo>? = start
            var buf = [CChar](repeating: 0, count: Int(NI_MAXHOST))
            while let n = node {
                if getnameinfo(n.pointee.ai_addr, n.pointee.ai_addrlen,
                               &buf, socklen_t(buf.count), nil, 0, NI_NUMERICHOST) == 0 {
                    let ip = String(cString: buf)
                    if !out.contains(ip) { out += "Dirección: \(ip)\n" }
                }
                node = n.pointee.ai_next
            }
            return out
        }
        c["host"] = c["nslookup"]
        c["dig"] = c["nslookup"]

        c["ping"] = Spec(help: "ping <host> — mide la latencia por HTTPS (no hay ICMP en iOS)") { ctx in
            guard let host = ctx.args.first else { throw ShErr("ping: falta el host") }
            var s = host
            if !s.contains("://") { s = "https://" + s }
            guard let url = URL(string: s) else { throw ShErr("ping: dirección no válida") }
            var out = "PING \(host) (por HTTPS, no ICMP)\n"
            var times: [Double] = []
            for i in 1...3 {
                var req = URLRequest(url: url)
                req.httpMethod = "HEAD"
                req.timeoutInterval = 10
                let start = Date()
                do {
                    let (_, resp) = try await URLSession.shared.data(for: req)
                    let ms = Date().timeIntervalSince(start) * 1000
                    times.append(ms)
                    let code = (resp as? HTTPURLResponse)?.statusCode ?? 0
                    out += String(format: "respuesta %d: HTTP %d  tiempo=%.0f ms\n", i, code, ms)
                } catch {
                    out += "respuesta \(i): sin respuesta\n"
                }
            }
            if !times.isEmpty {
                let avg = times.reduce(0, +) / Double(times.count)
                out += String(format: "media %.0f ms sobre %d respuestas\n", avg, times.count)
            }
            return out
        }

        return c
    }
}
