import Foundation
#if os(Linux) && canImport(FoundationNetworking)
import FoundationNetworking
#endif

// ============================================================
// MARK: - Comandos: red
// ============================================================

extension Shell {

    static func net() -> [String: Spec] {
        var c: [String: Spec] = [:]

        func fetch(_ urlStr: String, method: String, headers: [String], body: String?) async throws -> (Data, HTTPURLResponse) {
            var s = urlStr
            if !s.contains("://") { s = "https://" + s }
            guard let url = URL(string: s) else { throw ShErr("URL no válida: \(urlStr)") }
            var req = URLRequest(url: url)
            req.httpMethod = method
            req.timeoutInterval = 30
            for h in headers {
                guard let i = h.firstIndex(of: ":") else { continue }
                req.setValue(String(h[h.index(after: i)...]).trimmingCharacters(in: .whitespaces),
                             forHTTPHeaderField: String(h[h.startIndex..<i]))
            }
            if let b = body {
                req.httpBody = Data(b.utf8)
                if req.value(forHTTPHeaderField: "Content-Type") == nil {
                    req.setValue("application/x-www-form-urlencoded", forHTTPHeaderField: "Content-Type")
                }
            }
            let (d, r) = try await URLSession.shared.data(for: req)
            guard let http = r as? HTTPURLResponse else { throw ShErr("respuesta no HTTP") }
            return (d, http)
        }

        c["curl"] = Spec(help: "curl [-I] [-s] [-X MÉTODO] [-H 'K: V'] [-d datos] [-o archivo] <url> — petición HTTP") { ctx in
            var headers: [String] = []
            var method = "GET"
            var body: String? = nil
            var outFile: String? = nil
            var url: String? = nil
            var headOnly = false
            var showStatus = true
            var i = 0
            while i < ctx.args.count {
                let a = ctx.args[i]
                switch a {
                case "-H": if i + 1 < ctx.args.count { headers.append(ctx.args[i + 1]); i += 1 }
                case "-X": if i + 1 < ctx.args.count { method = ctx.args[i + 1].uppercased(); i += 1 }
                case "-d": if i + 1 < ctx.args.count { body = ctx.args[i + 1]; method = method == "GET" ? "POST" : method; i += 1 }
                case "-o": if i + 1 < ctx.args.count { outFile = ctx.args[i + 1]; i += 1 }
                case "-I": headOnly = true; method = "HEAD"
                case "-s": showStatus = false
                default: if !a.hasPrefix("-") { url = a }
                }
                i += 1
            }
            guard let u = url else { throw ShErr("curl: falta la URL") }
            let (data, resp) = try await fetch(u, method: method, headers: headers, body: body)
            if headOnly {
                var out = "HTTP \(resp.statusCode)\n"
                for (k, v) in resp.allHeaderFields {
                    out += "\(k): \(v)\n"
                }
                return out
            }
            if let f = outFile {
                try data.write(to: try ctx.env.resolve(f))
                return "guardado \(humanSize(data.count)) en \(f)\n"
            }
            let text = String(data: data, encoding: .utf8) ?? "(\(data.count) bytes binarios)"
            return (showStatus && resp.statusCode >= 400 ? "HTTP \(resp.statusCode)\n" : "") + text + "\n"
        }

        c["wget"] = Spec(help: "wget [-O archivo] <url> — descarga un archivo") { ctx in
            let o = opts(ctx.args, valued: ["O"])
            guard let u = o.rest.first else { throw ShErr("wget: falta la URL") }
            let (data, resp) = try await fetch(u, method: "GET", headers: [], body: nil)
            guard resp.statusCode < 400 else { throw ShErr("wget: HTTP \(resp.statusCode)") }
            var name = o.vals["O"] ?? URL(string: u.contains("://") ? u : "https://" + u)?.lastPathComponent ?? "descarga"
            if name.isEmpty { name = "index.html" }
            try data.write(to: try ctx.env.resolve(name))
            return "\(name): \(humanSize(data.count)) descargados\n"
        }

        c["api"] = Spec(help: "api [MÉTODO] <url> [-H 'K: V'] [-d datos] [-q ruta] [-o archivo] — llama a una API y formatea el JSON") { ctx in
            var headers: [String] = []
            var method = "GET"
            var body: String? = nil
            var query: String? = nil
            var outFile: String? = nil
            var url: String? = nil
            var i = 0
            let verbs = ["GET", "POST", "PUT", "DELETE", "PATCH", "HEAD"]
            while i < ctx.args.count {
                let a = ctx.args[i]
                switch a {
                case "-H": if i + 1 < ctx.args.count { headers.append(ctx.args[i + 1]); i += 1 }
                case "-d":
                    if i + 1 < ctx.args.count {
                        body = ctx.args[i + 1]
                        if method == "GET" { method = "POST" }
                        i += 1
                    }
                case "-q": if i + 1 < ctx.args.count { query = ctx.args[i + 1]; i += 1 }
                case "-o": if i + 1 < ctx.args.count { outFile = ctx.args[i + 1]; i += 1 }
                default:
                    if verbs.contains(a.uppercased()), url == nil { method = a.uppercased() }
                    else if !a.hasPrefix("-") { url = a }
                }
                i += 1
            }
            guard let u = url else { throw ShErr("api: falta la URL") }
            if body != nil, !headers.contains(where: { $0.lowercased().hasPrefix("content-type") }) {
                headers.append("Content-Type: application/json")
            }
            let (data, resp) = try await fetch(u, method: method, headers: headers, body: body)
            let raw = String(data: data, encoding: .utf8) ?? ""
            var text = raw
            if let pretty = try? DataTools.jsonPretty(raw) { text = pretty }
            if let q = query {
                text = DataTools.describe(try DataTools.jsonValue(raw, path: q))
            }
            if let f = outFile {
                try text.write(to: try ctx.env.resolve(f), atomically: true, encoding: .utf8)
                return "HTTP \(resp.statusCode) — guardado en \(f)\n"
            }
            return "HTTP \(resp.statusCode)\n" + text + "\n"
        }

        return c
    }
}

// ============================================================
// MARK: - Comandos: intérpretes
// ============================================================

extension Shell {

    static let swiftHelp = """

    El comando 'swift' traduce un subconjunto de Swift a JavaScript y lo ejecuta
    en el motor JavaScriptCore del sistema. iOS no permite compilar Swift real
    dentro de una app, así que esto es un intérprete de subconjunto, no swiftc.

    Sí funciona: let, var, func con retorno, if/else, while, for x in a..<b,
    for x in a...b, for x in array, arrays, cadenas con \\(interpolación),
    print, operadores, .count, .isEmpty, .append, .uppercased(), .lowercased(),
    .joined(separator:), Int.random(in:), Array(repeating:count:).

    También: struct y class (con constructor por nombre de campo),
    enum sencillos, guard y switch.

    No funciona: protocol, extension, import, opcionales, closures,
    genéricos, diccionarios literales, ni nada de SwiftUI.
    Ojo: la división de enteros da decimales, como en JavaScript.
    """

    static func lang() -> [String: Spec] {
        var c: [String: Spec] = [:]

        c["js"] = Spec(help: "js [-e código] [archivo] — ejecuta JavaScript (motor real)") { ctx in
            if ctx.args.first == "--reset" { ctx.sh.js.reset(); return "motor JavaScript reiniciado\n" }
            var code = ""
            if ctx.args.first == "-e" {
                code = ctx.args.dropFirst().joined(separator: " ")
            } else if let p = ctx.args.first {
                code = try ctx.input([p])
            } else {
                code = ctx.stdin
            }
            guard !code.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
                throw ShErr("js: no hay código que ejecutar")
            }
            return try ctx.sh.js.eval(code)
        }

        c["swift"] = Spec(help: "swift [-e código] [archivo.swift] — ejecuta un subconjunto de Swift") { ctx in
            var src = ""
            if ctx.args.first == "-e" { src = ctx.args.dropFirst().joined(separator: " ") }
            else if let p = ctx.args.first { src = try ctx.input([p]) }
            else { src = ctx.stdin }
            guard !src.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
                throw ShErr("swift: no hay código que ejecutar")
            }
            let jsCode = try SwiftJS.transpile(src)
            return try ctx.sh.js.eval(jsCode)
        }

        c["mode"] = Spec(help: "mode [shell|js|swift|json|xml|plist] — escribe directamente en ese idioma") { ctx in
            guard let name = ctx.args.first else {
                return "modo actual: \(ctx.sh.mode.rawValue)\ndisponibles: " +
                       Shell.Lang.allCases.map { $0.rawValue }.joined(separator: " ") + "\n"
            }
            guard let l = Shell.Lang(rawValue: name) else { throw ShErr("mode: idioma desconocido '\(name)'") }
            ctx.sh.mode = l
            ctx.sh.buffer = []
            if l == .shell { return "de vuelta al shell\n" }
            return "modo \(l.rawValue): escribe código directamente. 'exit' para volver al shell.\n"
        }
        c["repl"] = c["mode"]

        c["transpile"] = Spec(help: "transpile <archivo.swift> — muestra el JavaScript generado") { ctx in
            let src = try ctx.input(ctx.args)
            return try SwiftJS.transpile(src) + "\n"
        }

        c["run"] = Spec(help: "run <archivo> — ejecuta según la extensión (.js .swift .json .xml .plist)") { ctx in
            guard let p = ctx.args.first else { throw ShErr("run: falta el archivo") }
            let u = try ctx.env.resolve(p)
            guard ctx.env.exists(u) else { throw ShErr("run: \(p): no existe") }
            let ext = u.pathExtension.lowercased()
            let rest = Array(ctx.args.dropFirst())
            switch ext {
            case "js": return try await ctx.sh.commands["js"]!.run(Ctx(name: "js", args: [p] + rest, stdin: "", sh: ctx.sh))
            case "swift": return try await ctx.sh.commands["swift"]!.run(Ctx(name: "swift", args: [p] + rest, stdin: "", sh: ctx.sh))
            case "json": return try await ctx.sh.commands["json"]!.run(Ctx(name: "json", args: ["pretty", p], stdin: "", sh: ctx.sh))
            case "xml": return try await ctx.sh.commands["xml"]!.run(Ctx(name: "xml", args: ["pretty", p], stdin: "", sh: ctx.sh))
            case "plist": return try await ctx.sh.commands["plist"]!.run(Ctx(name: "plist", args: ["show", p], stdin: "", sh: ctx.sh))
            default: throw ShErr("run: no sé ejecutar '.\(ext)'")
            }
        }

        return c
    }
}

extension String {
    func leftPad(_ n: Int) -> String {
        count >= n ? self : String(repeating: " ", count: n - count) + self
    }
}
