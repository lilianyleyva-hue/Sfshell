import Foundation
#if os(Linux) && canImport(FoundationNetworking)
import FoundationNetworking
#endif

// ============================================================
// MARK: - Navegador de texto (como lynx / w3m)
// ============================================================
// Abre páginas web y las muestra como texto, con los enlaces
// numerados. Lo tienes tú y lo tiene cada IA en su propia shell
// (cada una con su historial). También abre localhost (api-local).
//
//   web <dirección>        abre (wikipedia.org, https://…, localhost/x)
//   web <n>                sigue el enlace [n]
//   web buscar <texto>     busca en DuckDuckGo
//   web mas | menos        siguiente / anterior pantalla de texto
//   web atras | adelante | recarga | enlaces | historial
//   web guardar <archivo>  guarda la página como texto
//   web fuente             el HTML tal cual
// Alias: navegador, lynx, w3m, browser

final class Navegador: @unchecked Sendable {
    var historial: [String] = []
    var pos = -1
    var titulo = ""
    var url = ""
    var lineas: [String] = []
    var enlaces: [String] = []
    var html = ""
    var pantalla = 0
    static let porPantalla = 45

    static let ayuda = """
    web — navegador de texto
      web wikipedia.org            abre una página
      web 3                        sigue el enlace [3]
      web buscar gatos persas      busca en DuckDuckGo
      web mas / menos              baja / sube por la página
      web atras / adelante / recarga
      web enlaces                  lista de enlaces
      web historial · web guardar archivo.txt · web fuente
      web localhost/usuarios       también abre tu api-local

    """

    static let agente = "Mozilla/5.0 (iPad; CPU OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.6 Mobile/15E148 Safari/604.1 SwiftShell"

    // ---------- HTML → texto ----------

    struct Pagina {
        var titulo: String
        var lineas: [String]
        var enlaces: [String]
    }

    static let entidades: [String: String] = [
        "amp": "&", "lt": "<", "gt": ">", "quot": "\"", "apos": "'", "nbsp": " ", "copy": "©", "reg": "®",
        "mdash": "—", "ndash": "–", "hellip": "…", "laquo": "«", "raquo": "»", "iexcl": "¡", "iquest": "¿",
        "aacute": "á", "eacute": "é", "iacute": "í", "oacute": "ó", "uacute": "ú", "ntilde": "ñ", "uuml": "ü",
        "Aacute": "Á", "Eacute": "É", "Iacute": "Í", "Oacute": "Ó", "Uacute": "Ú", "Ntilde": "Ñ", "Uuml": "Ü",
        "ccedil": "ç", "deg": "°", "middot": "·", "bull": "•", "euro": "€", "pound": "£", "times": "×",
        "rsquo": "’", "lsquo": "‘", "rdquo": "”", "ldquo": "“", "trade": "™", "ordm": "º", "ordf": "ª",
    ]

    static func decodifica(_ s: String) -> String {
        guard s.contains("&") else { return s }
        var out = ""
        var i = s.startIndex
        while i < s.endIndex {
            if s[i] == "&", let fin = s[i...].prefix(12).firstIndex(of: ";") {
                let nombre = String(s[s.index(after: i) ..< fin])
                var rep: String? = entidades[nombre]
                if rep == nil, nombre.hasPrefix("#") {
                    let num = nombre.dropFirst()
                    let v = num.hasPrefix("x") || num.hasPrefix("X") ? UInt32(num.dropFirst(), radix: 16) : UInt32(num)
                    if let v, let u = Unicode.Scalar(v) { rep = String(Character(u)) }
                }
                if let r = rep { out += r; i = s.index(after: fin); continue }
            }
            out.append(s[i])
            i = s.index(after: i)
        }
        return out
    }

    /// Quita la redirección de DuckDuckGo (//duckduckgo.com/l/?uddg=…).
    static func directa(_ u: String) -> String {
        guard u.contains("duckduckgo.com/l/"), let c = URLComponents(string: u.hasPrefix("//") ? "https:" + u : u),
              let real = c.queryItems?.first(where: { $0.name == "uddg" })?.value else { return u }
        return real
    }

    static func render(_ html: String, base: URL?) -> Pagina {
        let c = Array(html)
        var i = 0
        var out = ""
        var titulo = ""
        var enlaces: [String] = []
        var enlaceAbierto: [Int?] = []
        var ignorar = 0              // dentro de script/style/…
        var enPre = 0
        var enTitulo = false
        let saltar: Set<String> = ["script", "style", "noscript", "svg", "template", "iframe", "object", "canvas"]
        let bloques: Set<String> = ["p", "div", "section", "article", "header", "footer", "main", "nav", "ul", "ol",
                                    "table", "tr", "form", "blockquote", "pre", "dl", "dt", "dd", "figure",
                                    "figcaption", "aside", "address", "details", "summary", "fieldset", "center"]

        func atributo(_ t: String, _ nombre: String) -> String? {
            let bajo = t.lowercased()
            guard let r = bajo.range(of: " " + nombre + "=") ?? bajo.range(of: "\n" + nombre + "=") else { return nil }
            var j = t.index(t.startIndex, offsetBy: bajo.distance(from: bajo.startIndex, to: r.upperBound))
            guard j < t.endIndex else { return nil }
            let q = t[j]
            if q == "\"" || q == "'" {
                j = t.index(after: j)
                guard let fin = t[j...].firstIndex(of: q) else { return nil }
                return decodifica(String(t[j ..< fin]))
            }
            let fin = t[j...].firstIndex(where: { $0 == " " || $0 == ">" || $0 == "/" }) ?? t.endIndex
            return decodifica(String(t[j ..< fin]))
        }
        func salto() { if !out.hasSuffix("\n") { out += "\n" } }

        while i < c.count {
            if c[i] == "<" {
                // comentario
                if i + 3 < c.count, c[i + 1] == "!", c[i + 2] == "-", c[i + 3] == "-" {
                    var j = i + 4
                    while j + 2 < c.count, !(c[j] == "-" && c[j + 1] == "-" && c[j + 2] == ">") { j += 1 }
                    i = j + 3; continue
                }
                var j = i + 1
                var q: Character? = nil
                while j < c.count {
                    if let x = q { if c[j] == x { q = nil } } else if c[j] == "\"" || c[j] == "'" { q = c[j] } else if c[j] == ">" { break }
                    j += 1
                }
                let etiqueta = String(c[(i + 1) ..< min(j, c.count)])
                i = j + 1
                let cierra = etiqueta.hasPrefix("/")
                let nombre = String(etiqueta.drop { $0 == "/" }.prefix { $0.isLetter || $0.isNumber }).lowercased()
                if saltar.contains(nombre) {
                    if !etiqueta.hasSuffix("/") { ignorar += cierra ? -1 : 1 }
                    ignorar = max(0, ignorar)
                    continue
                }
                if ignorar > 0 { continue }
                switch nombre {
                case "title": enTitulo = !cierra
                case "br": out += "\n"
                case "hr": salto(); out += "────────────────────\n"
                case "pre": enPre += cierra ? -1 : 1; salto()
                case "li": if !cierra { salto(); out += "  • " }
                case "h1", "h2", "h3", "h4", "h5", "h6":
                    salto()
                    if !cierra { out += "\n" + String(repeating: "#", count: Int(String(nombre.last!)) ?? 1) + " " } else { out += "\n" }
                case "td", "th": if !cierra { out += " | " }
                case "img":
                    if let alt = atributo(etiqueta, "alt"), !alt.trimmingCharacters(in: .whitespaces).isEmpty { out += "[imagen: \(alt)]" }
                case "input":
                    let tipo = atributo(etiqueta, "type")?.lowercased() ?? "text"
                    if tipo == "submit" || tipo == "button" { out += "[\(atributo(etiqueta, "value") ?? "botón")]" }
                    else if tipo != "hidden" { out += "[____]" }
                case "button": if !cierra { out += "[" } else { out += "]" }
                case "a":
                    if cierra {
                        if let n = enlaceAbierto.popLast(), let n { out += "[\(n)]" }
                    } else {
                        var n: Int? = nil
                        if let h = atributo(etiqueta, "href"), !h.hasPrefix("javascript:"), !h.hasPrefix("#"), !h.hasPrefix("mailto:") {
                            let absoluta = URL(string: h, relativeTo: base)?.absoluteString ?? h
                            enlaces.append(directa(absoluta))
                            n = enlaces.count
                        }
                        enlaceAbierto.append(n)
                    }
                default:
                    if bloques.contains(nombre) { salto() }
                }
                continue
            }
            // texto
            var j = i
            while j < c.count, c[j] != "<" { j += 1 }
            if ignorar == 0 {
                var t = decodifica(String(c[i ..< j]))
                if enTitulo {
                    titulo += t
                } else {
                    if enPre == 0 {
                        t = t.replacingOccurrences(of: "\n", with: " ").replacingOccurrences(of: "\t", with: " ")
                        while t.contains("  ") { t = t.replacingOccurrences(of: "  ", with: " ") }
                        if out.hasSuffix(" ") || out.hasSuffix("\n") || out.isEmpty { t = String(t.drop { $0 == " " }) }
                    }
                    out += t
                }
            }
            i = j
        }
        var lineas: [String] = []
        var vacias = 0
        for l in out.components(separatedBy: "\n") {
            let x = l.trimmingCharacters(in: .whitespaces)
            if x.isEmpty || x == "|" { vacias += 1; if vacias == 1, !lineas.isEmpty { lineas.append("") }; continue }
            vacias = 0
            lineas.append(x.hasPrefix("• ") ? "  " + x : x)
        }
        while lineas.last == "" { lineas.removeLast() }
        return Pagina(titulo: titulo.trimmingCharacters(in: .whitespacesAndNewlines), lineas: lineas, enlaces: enlaces)
    }

    // ---------- navegar ----------

    static func normaliza(_ s: String) -> String {
        var u = s.trimmingCharacters(in: .whitespaces)
        if u.hasPrefix("//") { u = "https:" + u }
        if APILocal.local(u) != nil, !u.contains("://") { return "http://" + u }
        if !u.contains("://") { u = "https://" + u }
        return u
    }

    /// Descarga y procesa una página (sin tocar el historial).
    static func descarga(_ direccion: String, quien: String) async throws -> (url: String, html: String, pagina: Pagina) {
        let u = normaliza(direccion)
        let (d, resp) = try await Shell.peticion(u, metodo: "GET", cabeceras: ["User-Agent: \(agente)", "Accept-Language: es,en;q=0.8"], cuerpo: nil, quien: quien)
        guard d.count < 8 << 20 else { throw ShErr("web: página demasiado grande (\(humanSize(d.count)))") }
        let final = resp.url?.absoluteString.hasPrefix("http://localhost") == true ? u : (resp.url?.absoluteString ?? u)
        let tipo = (resp.allHeaderFields["Content-Type"] as? String ?? "").lowercased()
        let html = String(data: d, encoding: .utf8) ?? String(data: d, encoding: .isoLatin1) ?? ""
        if resp.statusCode >= 400, html.isEmpty { throw ShErr("web: HTTP \(resp.statusCode)") }
        if !tipo.contains("html"), !html.lowercased().contains("<html"), !html.lowercased().contains("<body") {
            // texto plano, JSON…: se muestra tal cual
            let lineas = html.components(separatedBy: "\n")
            return (final, html, Pagina(titulo: (URL(string: final)?.lastPathComponent).map { $0.isEmpty ? final : $0 } ?? final, lineas: lineas, enlaces: []))
        }
        var p = render(html, base: URL(string: final))
        if resp.statusCode >= 400 { p.titulo = "HTTP \(resp.statusCode) — " + p.titulo }
        return (final, html, p)
    }

    func carga(_ direccion: String, quien: String, nuevo: Bool) async throws -> String {
        let (final, codigo, p) = try await Navegador.descarga(direccion, quien: quien)
        url = final; html = codigo; titulo = p.titulo.isEmpty ? final : p.titulo
        lineas = p.lineas; enlaces = p.enlaces; pantalla = 0
        if nuevo {
            if pos < historial.count - 1 { historial.removeSubrange((pos + 1)...) }
            historial.append(final)
            if historial.count > 100 { historial.removeFirst() }
            pos = historial.count - 1
        }
        return muestra()
    }

    func muestra() -> String {
        guard !url.isEmpty else { return "no hay ninguna página abierta — web <dirección>\n" }
        let total = max(1, (lineas.count + Navegador.porPantalla - 1) / Navegador.porPantalla)
        pantalla = min(max(0, pantalla), total - 1)
        let ini = pantalla * Navegador.porPantalla
        var out = "── \(titulo) ──\n\(url)\n\n"
        out += lineas[ini ..< min(lineas.count, ini + Navegador.porPantalla)].joined(separator: "\n") + "\n"
        var pie = "pantalla \(pantalla + 1)/\(total)"
        if pantalla + 1 < total { pie += " · web mas" }
        if !enlaces.isEmpty { pie += " · web <n> abre un enlace (\(enlaces.count))" }
        if pos > 0 { pie += " · web atras" }
        return out + "── \(pie) ──\n"
    }
}

extension Shell {

    static func web() -> [String: Spec] {
        var c: [String: Spec] = [:]

        c["web"] = Spec(help: "web <dirección|n|buscar texto|mas|atras|enlaces…> — navegador de texto (como lynx)") { ctx in
            try await Shell.ordenWeb(ctx)
        }
        for alias in ["navegador", "lynx", "w3m", "browser", "links"] { c[alias] = c["web"] }
        return c
    }

    /// Cuerpo de 'web' (en una función y no en un closure: compila mucho más rápido).
    static func ordenWeb(_ ctx: Ctx) async throws -> String {
            let nav = ctx.sh.navegador
            let a = ctx.args
            let quien = ctx.sh.quienPide
            guard let primero = a.first else { return nav.url.isEmpty ? Navegador.ayuda : nav.muestra() }
            switch primero.lowercased() {
            case "ayuda", "help", "-h":
                return Navegador.ayuda
            case "buscar", "busca", "search", "s":
                let q = a.dropFirst().joined(separator: " ")
                guard !q.isEmpty else { throw ShErr("web buscar: ¿qué busco?") }
                let cod = q.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? q
                return try await nav.carga("https://html.duckduckgo.com/html/?q=\(cod)", quien: quien, nuevo: true)
            case "mas", "más", "+", "abajo":
                nav.pantalla += 1; return nav.muestra()
            case "menos", "-", "arriba":
                nav.pantalla -= 1; return nav.muestra()
            case "atras", "atrás", "back":
                guard nav.pos > 0 else { throw ShErr("web: no hay página anterior") }
                nav.pos -= 1
                return try await nav.carga(nav.historial[nav.pos], quien: quien, nuevo: false)
            case "adelante", "forward":
                guard nav.pos + 1 < nav.historial.count else { throw ShErr("web: no hay página siguiente") }
                nav.pos += 1
                return try await nav.carga(nav.historial[nav.pos], quien: quien, nuevo: false)
            case "recarga", "reload":
                guard !nav.url.isEmpty else { throw ShErr("web: no hay página abierta") }
                return try await nav.carga(nav.url, quien: quien, nuevo: false)
            case "enlaces", "links":
                guard !nav.enlaces.isEmpty else { return "esta página no tiene enlaces\n" }
                return nav.enlaces.enumerated().map { "[\($0.offset + 1)] \($0.element)" }.joined(separator: "\n") + "\n"
            case "historial", "history":
                guard !nav.historial.isEmpty else { return "historial vacío\n" }
                return nav.historial.enumerated().map { ($0.offset == nav.pos ? "→ " : "  ") + $0.element }.joined(separator: "\n") + "\n"
            case "guardar", "save":
                guard let f = a.dropFirst().first else { throw ShErr("uso: web guardar <archivo.txt>") }
                guard !nav.url.isEmpty else { throw ShErr("web: no hay página abierta") }
                var texto: String = "\(nav.titulo)\n\(nav.url)\n\n"
                texto += nav.lineas.joined(separator: "\n")
                texto += "\n\nEnlaces:\n"
                for (i, e) in nav.enlaces.enumerated() { texto += "[\(i + 1)] \(e)\n" }
                try texto.write(to: try ctx.env.resolve(f), atomically: true, encoding: .utf8)
                return "guardada en \(f) (\(nav.lineas.count) líneas)\n"
            case "fuente", "source":
                guard !nav.html.isEmpty else { throw ShErr("web: no hay página abierta") }
                return nav.html + (nav.html.hasSuffix("\n") ? "" : "\n")
            default:
                if let n = Int(primero) {
                    guard n >= 1, n <= nav.enlaces.count else { throw ShErr("web: no hay enlace [\(n)] (hay \(nav.enlaces.count))") }
                    return try await nav.carga(nav.enlaces[n - 1], quien: quien, nuevo: true)
                }
                return try await nav.carga(a.joined(separator: "%20"), quien: quien, nuevo: true)
            }
    }
}
