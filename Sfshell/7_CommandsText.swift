import Foundation
import CryptoKit

// ============================================================
// MARK: - Comandos: texto
// ============================================================

extension Shell {

    static func text() -> [String: Spec] {
        var c: [String: Spec] = [:]

        c["head"] = Spec(help: "head [-n N] [archivo...] — primeras líneas") { ctx in
            let o = opts(ctx.args, valued: ["n"])
            let n = Int(o.vals["n"] ?? "10") ?? 10
            return try ctx.lines(o.rest).prefix(n).joined(separator: "\n") + "\n"
        }

        c["tail"] = Spec(help: "tail [-n N] [archivo...] — últimas líneas") { ctx in
            let o = opts(ctx.args, valued: ["n"])
            let n = Int(o.vals["n"] ?? "10") ?? 10
            return try ctx.lines(o.rest).suffix(n).joined(separator: "\n") + "\n"
        }

        c["wc"] = Spec(help: "wc [-l -w -c] [archivo...] — cuenta líneas, palabras y bytes") { ctx in
            let o = opts(ctx.args)
            let t = try ctx.input(o.rest)
            let l = t.isEmpty ? 0 : t.components(separatedBy: "\n").count - (t.hasSuffix("\n") ? 1 : 0)
            let w = t.split(whereSeparator: { $0.isWhitespace }).count
            let b = t.utf8.count
            if o.flags.contains("l") { return "\(l)\n" }
            if o.flags.contains("w") { return "\(w)\n" }
            if o.flags.contains("c") { return "\(b)\n" }
            return "\(l) \(w) \(b)\n"
        }

        c["grep"] = Spec(help: "grep [-i] [-v] [-n] [-c] <patrón> [archivo...] — busca con expresiones regulares") { ctx in
            let o = opts(ctx.args)
            guard let pat = o.rest.first else { throw ShErr("grep: falta el patrón") }
            let re = try NSRegularExpression(pattern: pat, options: o.flags.contains("i") ? [.caseInsensitive] : [])
            let lines = try ctx.lines(Array(o.rest.dropFirst()))
            var out = ""
            var count = 0
            for (i, l) in lines.enumerated() {
                let hit = re.firstMatch(in: l, range: NSRange(l.startIndex..., in: l)) != nil
                if hit != o.flags.contains("v") {
                    count += 1
                    out += (o.flags.contains("n") ? "\(i + 1):" : "") + l + "\n"
                }
            }
            return o.flags.contains("c") ? "\(count)\n" : out
        }

        c["sed"] = Spec(help: "sed 's/patrón/reemplazo/[g][i]' [archivo...] — sustitución de texto") { ctx in
            guard let expr = ctx.args.first, expr.hasPrefix("s") else {
                throw ShErr("sed: solo se admite 's/patrón/reemplazo/'")
            }
            let chars = Array(expr)
            guard chars.count > 3 else { throw ShErr("sed: expresión incompleta") }
            let sep = chars[1]
            let parts = String(chars.dropFirst(2)).components(separatedBy: String(sep))
            guard parts.count >= 2 else { throw ShErr("sed: expresión incompleta") }
            let pat = parts[0], rep = parts[1]
            let flags = parts.count > 2 ? parts[2] : ""
            let re = try NSRegularExpression(pattern: pat, options: flags.contains("i") ? [.caseInsensitive] : [])
            let lines = try ctx.lines(Array(ctx.args.dropFirst()))
            var out = ""
            for l in lines {
                let range = NSRange(l.startIndex..., in: l)
                if flags.contains("g") {
                    out += re.stringByReplacingMatches(in: l, range: range, withTemplate: rep) + "\n"
                } else if let m = re.firstMatch(in: l, range: range) {
                    let ns = l as NSString
                    let replaced = re.replacementString(for: m, in: l, offset: 0, template: rep)
                    out += ns.replacingCharacters(in: m.range, with: replaced) + "\n"
                } else { out += l + "\n" }
            }
            return out
        }

        c["sort"] = Spec(help: "sort [-r] [-n] [-u] [archivo...] — ordena líneas") { ctx in
            let o = opts(ctx.args)
            var lines = try ctx.lines(o.rest)
            if o.flags.contains("n") {
                lines.sort { (Double($0.trimmingCharacters(in: .whitespaces)) ?? 0) < (Double($1.trimmingCharacters(in: .whitespaces)) ?? 0) }
            } else {
                lines.sort { $0.lowercased() < $1.lowercased() }
            }
            if o.flags.contains("u") {
                var seen = Set<String>()
                lines = lines.filter { seen.insert($0).inserted }
            }
            if o.flags.contains("r") { lines.reverse() }
            return lines.joined(separator: "\n") + "\n"
        }

        c["uniq"] = Spec(help: "uniq [-c] [archivo...] — quita líneas repetidas seguidas") { ctx in
            let o = opts(ctx.args)
            let lines = try ctx.lines(o.rest)
            var out = ""
            var prev: String? = nil
            var n = 0
            func emit() {
                if let p = prev { out += (o.flags.contains("c") ? "\(String(n).leftPad(5)) " : "") + p + "\n" }
            }
            for l in lines {
                if l == prev { n += 1 } else { emit(); prev = l; n = 1 }
            }
            emit()
            return out
        }

        c["cut"] = Spec(help: "cut -d <sep> -f <n> [archivo...] — extrae columnas") { ctx in
            let o = opts(ctx.args, valued: ["d", "f"])
            let d = o.vals["d"] ?? "\t"
            let fields = (o.vals["f"] ?? "1").split(separator: ",").compactMap { Int($0) }
            return try ctx.lines(o.rest).map { l -> String in
                let parts = l.components(separatedBy: d)
                return fields.compactMap { i in i >= 1 && i <= parts.count ? parts[i - 1] : nil }.joined(separator: d)
            }.joined(separator: "\n") + "\n"
        }

        c["tr"] = Spec(help: "tr [-d] <conjunto1> [conjunto2] — cambia o borra caracteres") { ctx in
            let o = opts(ctx.args)
            guard let a = o.rest.first else { throw ShErr("tr: faltan argumentos") }
            let skip = o.flags.contains("d") ? 1 : 2
            let t = try ctx.input(Array(o.rest.dropFirst(skip)))
            if o.flags.contains("d") { return String(t.filter { !a.contains($0) }) }
            guard let b = o.rest.dropFirst().first else { throw ShErr("tr: falta el segundo conjunto") }
            let from = Array(a), to = Array(b)
            return String(t.map { ch -> Character in
                if let i = from.firstIndex(of: ch) { return i < to.count ? to[i] : (to.last ?? ch) }
                return ch
            })
        }

        c["rev"] = Spec(help: "rev [archivo...] — invierte cada línea") { ctx in
            try ctx.lines(ctx.args).map { String($0.reversed()) }.joined(separator: "\n") + "\n"
        }

        c["nl"] = Spec(help: "nl [archivo...] — numera las líneas") { ctx in
            try ctx.lines(ctx.args).enumerated().map { "\(String($0.offset + 1).leftPad(5))  \($0.element)" }
                .joined(separator: "\n") + "\n"
        }

        c["tee"] = Spec(help: "tee <archivo> — escribe la entrada en un archivo y la deja pasar") { ctx in
            guard let p = ctx.args.first else { throw ShErr("tee: falta el archivo") }
            try ctx.stdin.write(to: try ctx.env.resolve(p), atomically: true, encoding: .utf8)
            return ctx.stdin
        }

        c["base64"] = Spec(help: "base64 [-d] [archivo...] — codifica o descodifica") { ctx in
            let o = opts(ctx.args)
            let t = try ctx.input(o.rest)
            if o.flags.contains("d") {
                let clean = t.trimmingCharacters(in: .whitespacesAndNewlines)
                guard let d = Data(base64Encoded: clean), let s = String(data: d, encoding: .utf8) else {
                    throw ShErr("base64: entrada no válida")
                }
                return s
            }
            return (t.data(using: .utf8) ?? Data()).base64EncodedString() + "\n"
        }

        c["sha256"] = Spec(help: "sha256 [archivo...] — huella criptográfica") { ctx in
            let t = try ctx.input(ctx.args)
            let h = SHA256.hash(data: Data(t.utf8))
            return h.map { String(format: "%02x", $0) }.joined() + "\n"
        }

        c["xxd"] = Spec(help: "xxd <archivo> — volcado hexadecimal") { ctx in
            let t = try ctx.input(ctx.args)
            let bytes = Array(t.utf8)
            var out = ""
            for off in stride(from: 0, to: bytes.count, by: 16) {
                let chunk = Array(bytes[off..<min(off + 16, bytes.count)])
                let hex = chunk.map { String(format: "%02x", $0) }.joined(separator: " ")
                let ascii = String(chunk.map { $0 >= 32 && $0 < 127 ? Character(UnicodeScalar($0)) : "." })
                out += String(format: "%08x  ", off) + hex.padding(toLength: 47, withPad: " ", startingAt: 0) + "  " + ascii + "\n"
            }
            return out
        }

        c["expr"] = Spec(help: "expr <expresión> — calculadora") { ctx in
            let e = ctx.args.joined(separator: " ")
            guard !e.isEmpty else { throw ShErr("expr: falta la expresión") }
            return try ctx.sh.js.eval("String(\(e))")
        }
        c["calc"] = c["expr"]

        c["seq"] = Spec(help: "seq [inicio] <fin> — genera números") { ctx in
            let nums = ctx.args.compactMap { Int($0) }
            guard let last = nums.last else { throw ShErr("seq: falta el número final") }
            let first = nums.count > 1 ? nums[0] : 1
            guard last >= first, last - first < 100_000 else { throw ShErr("seq: rango no válido") }
            return (first...last).map(String.init).joined(separator: "\n") + "\n"
        }

        return c
    }
}

// ============================================================
// MARK: - Comandos: datos estructurados
// ============================================================

extension Shell {

    static func data() -> [String: Spec] {
        var c: [String: Spec] = [:]

        c["json"] = Spec(help: "json <pretty|min|get RUTA|keys|check> [archivo] — trabaja con JSON") { ctx in
            guard let sub = ctx.args.first else { throw ShErr("json: uso: json pretty|min|get <ruta>|keys|check [archivo]") }
            switch sub {
            case "pretty":
                return try DataTools.jsonPretty(ctx.input(Array(ctx.args.dropFirst()))) + "\n"
            case "min":
                return try DataTools.jsonMin(ctx.input(Array(ctx.args.dropFirst()))) + "\n"
            case "check":
                _ = try DataTools.jsonMin(ctx.input(Array(ctx.args.dropFirst())))
                return "JSON válido\n"
            case "get":
                guard ctx.args.count >= 2 else { throw ShErr("json get: falta la ruta, por ejemplo: json get a.b[0]") }
                let text = try ctx.input(Array(ctx.args.dropFirst(2)))
                return DataTools.describe(try DataTools.jsonValue(text, path: ctx.args[1])) + "\n"
            case "keys":
                let text = try ctx.input(Array(ctx.args.dropFirst()))
                guard let d = text.data(using: .utf8),
                      let obj = try JSONSerialization.jsonObject(with: d) as? [String: Any] else {
                    throw ShErr("json keys: la raíz no es un objeto")
                }
                return obj.keys.sorted().joined(separator: "\n") + "\n"
            default:
                throw ShErr("json: subcomando desconocido '\(sub)'")
            }
        }
        c["jq"] = c["json"]

        c["xml"] = Spec(help: "xml <pretty|text|find TAG> [archivo] — trabaja con XML") { ctx in
            guard let sub = ctx.args.first else { throw ShErr("xml: uso: xml pretty|text|find <tag> [archivo]") }
            let skip = (sub == "find") ? 2 : 1
            let text = try ctx.input(Array(ctx.args.dropFirst(skip)))
            guard let d = text.data(using: .utf8) else { throw ShErr("xml: texto no válido") }
            let root = try XMLTreeBuilder().parse(d)
            switch sub {
            case "pretty": return root.pretty() + "\n"
            case "text": return root.allText() + "\n"
            case "find":
                guard ctx.args.count >= 2 else { throw ShErr("xml find: falta el nombre de la etiqueta") }
                var hits: [XMLNode] = []
                root.find(ctx.args[1], into: &hits)
                if hits.isEmpty { return "" }
                return hits.map { $0.pretty() }.joined(separator: "\n") + "\n"
            default: throw ShErr("xml: subcomando desconocido '\(sub)'")
            }
        }

        c["plist"] = Spec(help: "plist <show|tojson|fromjson> [archivo] — listas de propiedades") { ctx in
            guard let sub = ctx.args.first else { throw ShErr("plist: uso: plist show|tojson|fromjson [archivo]") }
            let rest = Array(ctx.args.dropFirst())
            if sub == "fromjson" { return try DataTools.jsonToPlist(ctx.input(rest)) + "\n" }
            var data: Data
            if let p = rest.first {
                let u = try ctx.env.resolve(p)
                guard let d = FileManager.default.contents(atPath: u.path) else { throw ShErr("plist: \(p): no se puede leer") }
                data = d
            } else { data = Data(ctx.stdin.utf8) }
            switch sub {
            case "show": return try DataTools.plistShow(data) + "\n"
            case "tojson": return try DataTools.plistToJSON(data) + "\n"
            default: throw ShErr("plist: subcomando desconocido '\(sub)'")
            }
        }

        return c
    }
}
