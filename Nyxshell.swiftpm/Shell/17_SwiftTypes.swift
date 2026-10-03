import Foundation

// ============================================================
// MARK: - Traductor de Swift: struct, class, enum, guard, switch
// ============================================================

extension SwiftJS {

    /// Todo lo que hay que arreglar antes de traducir el resto.
    static func preprocess(_ src: String) -> String {
        var s = src
        s = convertGuards(s)
        s = convertExtensions(s)
        s = convertEnums(s)
        s = convertSwitches(s)
        s = convertTypes(s)
        return s
    }

    // --- utilidad: encuentra la llave que cierra ---------------

    /// Dado el índice de una '{', devuelve el índice de su '}'.
    static func matchBrace(_ s: String, from open: String.Index) -> String.Index? {
        var depth = 0
        var i = open
        var inStr: Character? = nil
        var prev: Character = " "
        while i < s.endIndex {
            let ch = s[i]
            if let q = inStr {
                if ch == q && prev != "\\" { inStr = nil }
            } else if ch == "\"" || ch == "'" {
                inStr = ch
            } else if ch == "{" {
                depth += 1
            } else if ch == "}" {
                depth -= 1
                if depth == 0 { return i }
            }
            prev = ch
            i = s.index(after: i)
        }
        return nil
    }

    static func firstMatch(_ s: String, _ pattern: String, desde: Int) -> (range: Range<String.Index>, groups: [String])? {
        guard let re = try? NSRegularExpression(pattern: pattern) else { return nil }
        let ns = s as NSString
        guard desde <= ns.length,
              let m = re.firstMatch(in: s, range: NSRange(location: desde, length: ns.length - desde)),
              let r = Range(m.range, in: s) else { return nil }
        var groups: [String] = []
        for k in 0..<m.numberOfRanges {
            groups.append(m.range(at: k).location == NSNotFound ? "" : ns.substring(with: m.range(at: k)))
        }
        return (r, groups)
    }

    static func firstMatch(_ s: String, _ pattern: String) -> (range: Range<String.Index>, groups: [String])? {
        guard let re = try? NSRegularExpression(pattern: pattern) else { return nil }
        let ns = s as NSString
        guard let m = re.firstMatch(in: s, range: NSRange(location: 0, length: ns.length)),
              let r = Range(m.range, in: s) else { return nil }
        var groups: [String] = []
        for k in 0..<m.numberOfRanges {
            groups.append(m.range(at: k).location == NSNotFound ? "" : ns.substring(with: m.range(at: k)))
        }
        return (r, groups)
    }

    // --- guard cond else { ... } -------------------------------

    static func convertGuards(_ src: String) -> String {
        var s = src
        var vueltas = 0
        while let hit = firstMatch(s, #"\bguard\s+(.+?)\s+else\s*\{"#), vueltas < 200 {
            vueltas += 1
            let cond = hit.groups[1]
                .replacingOccurrences(of: "let ", with: "")
                .replacingOccurrences(of: "var ", with: "")
            s.replaceSubrange(hit.range, with: "if (!(\(cond))) {")
        }
        return s
    }

    // --- enum Color { case rojo, verde } -----------------------

    static func convertEnums(_ src: String) -> String {
        var s = src
        var vueltas = 0
        while let hit = firstMatch(s, #"\benum\s+(\w+)[^\{]*\{"#), vueltas < 100 {
            vueltas += 1
            let name = hit.groups[1]
            guard let close = matchBrace(s, from: s.index(before: hit.range.upperBound)) else { break }
            let body = String(s[hit.range.upperBound..<close])
            var casos: [String] = []
            for line in body.components(separatedBy: "\n") {
                let t = line.trimmingCharacters(in: .whitespaces)
                guard t.hasPrefix("case ") else { continue }
                for c in t.dropFirst(5).components(separatedBy: ",") {
                    let n = c.trimmingCharacters(in: .whitespaces)
                        .components(separatedBy: "=").first?
                        .trimmingCharacters(in: .whitespaces) ?? ""
                    if !n.isEmpty, !n.contains("(") { casos.append(n) }
                }
            }
            let js = "const \(name) = { " + casos.map { "\($0): \"\($0)\"" }.joined(separator: ", ") + " };"
            s.replaceSubrange(hit.range.lowerBound...close, with: js)
        }
        return s
    }

    // --- switch x { case a: ... default: ... } -----------------

    static func convertSwitches(_ src: String) -> String {
        var s = src
        var vueltas = 0
        while let hit = firstMatch(s, #"\bswitch\s+(.+?)\s*\{"#), vueltas < 100 {
            vueltas += 1
            let expr = hit.groups[1].trimmingCharacters(in: .whitespaces)
            guard let close = matchBrace(s, from: s.index(before: hit.range.upperBound)) else { break }
            let body = String(s[hit.range.upperBound..<close])

            // partimos el cuerpo en ramas
            var ramas: [(patrones: [String], cuerpo: String)] = []
            var actual: [String] = []
            var acumulado = ""
            func cerrar() {
                if !actual.isEmpty || !acumulado.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                    ramas.append((actual, acumulado))
                }
                acumulado = ""
            }
            for line in body.components(separatedBy: "\n") {
                let t = line.trimmingCharacters(in: .whitespaces)
                if t.hasPrefix("case ") {
                    cerrar()
                    let sinCase = String(t.dropFirst(5))
                    let corte = sinCase.lastIndex(of: ":") ?? sinCase.endIndex
                    actual = String(sinCase[sinCase.startIndex..<corte])
                        .components(separatedBy: ",")
                        .map { $0.trimmingCharacters(in: .whitespaces) }
                    if corte < sinCase.endIndex {
                        acumulado = String(sinCase[sinCase.index(after: corte)...]) + "\n"
                    }
                } else if t.hasPrefix("default") {
                    cerrar()
                    actual = []
                    if let i = t.firstIndex(of: ":") {
                        acumulado = String(t[t.index(after: i)...]) + "\n"
                    }
                } else {
                    acumulado += line + "\n"
                }
            }
            cerrar()

            // lo convertimos en if / else if / else
            var js = ""
            var primero = true
            for r in ramas {
                if r.patrones.isEmpty {
                    js += js.isEmpty ? "{\n\(r.cuerpo)}" : " else {\n\(r.cuerpo)}"
                } else {
                    let cond = r.patrones.map { p -> String in
                        if p.contains("...") || p.contains("..<") {
                            let sep = p.contains("...") ? "..." : "..<"
                            let lados = p.components(separatedBy: sep)
                            let op = sep == "..." ? "<=" : "<"
                            return "((\(expr)) >= \(lados[0]) && (\(expr)) \(op) \(lados.count > 1 ? lados[1] : lados[0]))"
                        }
                        if p == "_" { return "true" }
                        return "(\(expr)) === \(p)"
                    }.joined(separator: " || ")
                    js += (primero ? "if (" : " else if (") + cond + ") {\n\(r.cuerpo)}"
                    primero = false
                }
            }
            s.replaceSubrange(hit.range.lowerBound...close, with: js)
        }
        return s
    }

    // --- extension Nombre { func ... } -------------------------

    /// Los métodos de una extension se cuelgan del prototipo de la clase.
    static func convertExtensions(_ src: String) -> String {
        var s = src
        var vueltas = 0
        while let hit = firstMatch(s, #"\bextension\s+(\w+)\s*(:[^\{]*)?\{"#), vueltas < 100 {
            vueltas += 1
            let name = hit.groups[1]
            guard let close = matchBrace(s, from: s.index(before: hit.range.upperBound)) else { break }
            let body = String(s[hit.range.upperBound..<close])
            var js = ""
            var i = body.startIndex
            while i < body.endIndex {
                let resto = String(body[i...])
                if let m = firstMatch(resto, #"^\s*(mutating\s+|static\s+)?func\s+(\w+)\s*\(([^)]*)\)\s*(->[^\{]*)?\{"#) {
                    let nombre = m.groups[2]
                    let estatico = m.groups[1].contains("static")
                    let params = m.groups[3].split(separator: ",").map { p -> String in
                        let head = p.split(separator: ":").first.map(String.init) ?? String(p)
                        return head.split(separator: " ").map(String.init).last ?? head
                    }.joined(separator: ", ")
                    let abre = body.index(i, offsetBy: resto.distance(from: resto.startIndex, to: m.range.upperBound) - 1)
                    guard let cierra = matchBrace(body, from: abre) else { break }
                    let cuerpo = String(body[body.index(after: abre)..<cierra])
                    let destino = estatico ? name : "\(name).prototype"
                    js += "\(destino).\(nombre) = function(\(params)) {\(cuerpo)};\n"
                    i = body.index(after: cierra)
                    continue
                }
                i = body.index(after: i)
            }
            s.replaceSubrange(hit.range.lowerBound...close, with: js)
        }
        return s
    }

    // --- struct y class ----------------------------------------

    static func convertTypes(_ src: String) -> String {
        var s = src
        var tipos: [String] = []
        var vueltas = 0
        // Se busca SIEMPRE después de la última clase generada: antes se
        // volvía a encontrar la 'class' recién escrita, se reprocesaba y
        // perdía sus propiedades y métodos (salía una clase vacía).
        var desde = 0

        while vueltas < 100, let hit = firstMatch(s, #"\b(struct|class)\s+(\w+)\s*(:[^\{]*)?\{"#, desde: desde) {
            vueltas += 1
            let name = hit.groups[2]
            tipos.append(name)
            guard let close = matchBrace(s, from: s.index(before: hit.range.upperBound)) else { break }
            let body = String(s[hit.range.upperBound..<close])

            var props: [(String, String?)] = []     // nombre, valor por omisión
            var metodos: [(estatico: Bool, nombre: String, params: String, cuerpo: String)] = []
            var i = body.startIndex

            while i < body.endIndex {
                // ¿empieza aquí un func?
                let resto = String(body[i...])
                if let m = firstMatch(resto, #"^\s*(?:(?:private|fileprivate|public|internal|@discardableResult)\s+)*(mutating\s+|static\s+)?func\s+(\w+)\s*\(([^)]*)\)\s*(->[^\{]*)?\{"#) {
                    let params = parametros(m.groups[3])
                    let abre = body.index(i, offsetBy: resto.distance(from: resto.startIndex, to: m.range.upperBound) - 1)
                    guard let cierra = matchBrace(body, from: abre) else { break }
                    let cuerpo = String(body[body.index(after: abre)..<cierra])
                    metodos.append((m.groups[1].hasPrefix("static"), m.groups[2], params, cuerpo))
                    i = body.index(after: cierra)
                    continue
                }
                // ¿una propiedad?
                if let m = firstMatch(resto, #"^\s*(?:(?:private|fileprivate|public|internal)\s+)*(var|let)\s+(\w+)\s*(:\s*[^=\n]+)?(=\s*([^\n]+))?\n"#) {
                    let nombre = m.groups[2]
                    let valor = m.groups[5].trimmingCharacters(in: .whitespaces)
                    props.append((nombre, valor.isEmpty ? nil : valor))
                    i = body.index(i, offsetBy: resto.distance(from: resto.startIndex, to: m.range.upperBound))
                    continue
                }
                i = body.index(after: i)
            }

            var ctor = "  constructor(o) {\n    o = o || {};\n"
            for (n, def) in props {
                let fallback = def ?? "undefined"
                ctor += "    this.\(n) = (o.\(n) !== undefined) ? o.\(n) : \(fallback);\n"
            }
            ctor += "  }\n"

            // dentro de un método, 'x' (una propiedad) es 'this.x' en JS
            var js = "class \(name) {\n" + ctor
            for m in metodos {
                var cuerpo = m.cuerpo
                let locales = Set(m.params.components(separatedBy: ",").map {
                    $0.components(separatedBy: "=")[0].trimmingCharacters(in: .whitespaces)
                })
                for (n, _) in props where !locales.contains(n) && !m.estatico {
                    cuerpo = rx(cuerpo, "(?<![.\\w$])\(n)\\b(?!\\s*:)", "this.\(n)")
                }
                js += "  " + (m.estatico ? "static " : "") + "\(m.nombre)(\(m.params)) {\(cuerpo)}\n"
            }
            js += "}"
            let inicio = s.distance(from: s.startIndex, to: hit.range.lowerBound)
            s.replaceSubrange(hit.range.lowerBound...close, with: js)
            desde = (String(s.prefix(inicio)) as NSString).length + (js as NSString).length
        }

        // Punto(x: 1, y: 2)  →  new Punto({x: 1, y: 2})
        for t in Set(tipos) {
            guard let re = try? NSRegularExpression(pattern: "(?<![\\w.])\(t)\\s*\\(([^()]*)\\)") else { continue }
            let ns = s as NSString
            var out = ns
            for m in re.matches(in: s, range: NSRange(location: 0, length: ns.length)).reversed() {
                let args = out.substring(with: m.range(at: 1)).trimmingCharacters(in: .whitespaces)
                let objeto = args.isEmpty ? "{}" : "{\(args)}"
                out = out.replacingCharacters(in: m.range, with: "new \(t)(\(objeto))") as NSString
            }
            s = out as String
        }
        return s
    }
}
