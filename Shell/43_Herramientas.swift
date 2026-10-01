import Foundation
#if os(Linux) && canImport(FoundationNetworking)
import FoundationNetworking
#endif

// ============================================================
// MARK: - Herramientas (para las IAs, y también para ti)
// ============================================================
//   herramientas              la lista
//   busca <tema>              Wikipedia (o DuckDuckGo si falla): títulos y resumen
//   lee <url|archivo> [n]     lo lee y lo resume; si eres una IA, lo aprende
//   resume <archivo|-> [n]    las n frases más importantes de un texto
//   calcula <expresión>       cuentas con Python: calcula 2**10 + sqrt(2)
//   pregunta <rol> <texto>    le preguntas a una IA y te contesta (con glosa)
//   guarda <clave> <valor>    memoria propia (en /Documentos/memoria.json)
//   saca [clave] · olvida <clave>
//   web …                     el navegador de texto (42_Web.swift)
// Cuando una IA usa 'busca' o 'lee', lo que lee entra a su mente
// (frase a frase) y queda apuntado en /Documentos/lecturas.txt.

enum Herramientas {

    static let vacias: Set<String> = [
        "de", "la", "que", "el", "en", "y", "a", "los", "del", "se", "las", "por", "un", "para", "con", "no", "una",
        "su", "al", "lo", "como", "más", "mas", "pero", "sus", "le", "ya", "o", "este", "sí", "porque", "esta", "entre",
        "cuando", "muy", "sin", "sobre", "también", "me", "hasta", "hay", "donde", "quien", "desde", "todo", "nos",
        "durante", "todos", "uno", "les", "ni", "contra", "otros", "ese", "eso", "ante", "ellos", "e", "esto", "mí",
        "antes", "algunos", "qué", "unos", "yo", "otro", "otras", "otra", "él", "tanto", "esa", "estos", "mucho",
        "quienes", "nada", "muchos", "cual", "poco", "ella", "estar", "estas", "algunas", "algo", "nosotros", "es",
        "son", "fue", "ser", "ha", "han", "era", "the", "of", "and", "to", "in", "is", "it", "that", "for", "on", "was",
        "with", "as", "by", "at", "an", "be", "this", "are", "from", "or", "which",
    ]

    /// Quita lo que no es prosa: marcas de enlace [3], [imagen: …], [botón].
    static func limpia(_ texto: String) -> String {
        var t = texto
        for patron in ["\\[imagen:[^\\]]*\\]", "\\[[0-9]+\\]", "\\[____\\]", "\\| "] {
            t = t.replacingOccurrences(of: patron, with: " ", options: .regularExpression)
        }
        return t
    }

    /// Si parece HTML, lo convierte a texto con el navegador.
    static func comoTexto(_ t: String) -> String {
        let bajo = t.prefix(2000).lowercased()
        guard bajo.contains("<html") || bajo.contains("<body") || bajo.contains("<p>") || bajo.contains("<!doctype") else { return t }
        return Navegador.render(t, base: nil).lineas.joined(separator: "\n")
    }

    /// Divide un texto en frases.
    static func frases(_ texto: String) -> [String] {
        var out: [String] = []
        var actual = ""
        for ch in limpia(texto) {
            if ch == "\n" {
                if !actual.trimmingCharacters(in: .whitespaces).isEmpty { out.append(actual) }
                actual = ""; continue
            }
            actual.append(ch)
            if ".!?".contains(ch) {
                out.append(actual); actual = ""
            }
        }
        if !actual.trimmingCharacters(in: .whitespaces).isEmpty { out.append(actual) }
        return out.map { $0.trimmingCharacters(in: .whitespaces) }.filter { $0.count > 25 && $0.split(separator: " ").count >= 4 }
    }

    static func palabras(_ s: String) -> [String] {
        s.lowercased().split(whereSeparator: { !$0.isLetter && !$0.isNumber }).map(String.init)
            .filter { $0.count > 2 && !vacias.contains($0) }
    }

    /// Resumen extractivo: las n frases con palabras más frecuentes del
    /// texto, en su orden original. Sin modelos: cuenta y elige.
    static func resume(_ texto: String, _ n: Int) -> [String] {
        let fs = frases(texto)
        guard fs.count > n else { return fs }
        var frec: [String: Int] = [:]
        for p in palabras(texto) { frec[p, default: 0] += 1 }
        let puntos = fs.enumerated().map { (i, f) -> (Int, Double) in
            let ps = palabras(f)
            let suma = ps.reduce(0) { $0 + (frec[$1] ?? 0) }
            let bonus = i == 0 ? 1.3 : 1.0               // la primera frase suele presentar el tema
            return (i, Double(suma) / Double(max(5, ps.count)) * bonus)
        }
        let elegidas = Set(puntos.sorted { $0.1 > $1.1 }.prefix(n).map(\.0))
        return fs.enumerated().filter { elegidas.contains($0.offset) }.map(\.element)
    }

    /// Glosa sencilla de una frase Resh (palabra por palabra).
    static func glosa(_ frase: String) -> String {
        frase.split(separator: " ").map { w -> String in
            let t = w.trimmingCharacters(in: .punctuationCharacters).lowercased()
            if let c = LenguaResh.conceptoPorParticula[t] { return c }
            return LenguaResh.glosaDe(t) ?? String(w)
        }.joined(separator: " ")
    }

    /// Si la shell es de una IA, lo leído entra a su mente y a su disco.
    static func aprende(_ ctx: Ctx, fuente: String, frases fs: [String]) async -> String {
        guard let r = ctx.sh.rolIA, !fs.isEmpty, let m = await NyxNucleo.uno.consejo.mente(r) else { return "" }
        for f in fs.prefix(12) { await m.recibir(mensaje: f, de: "lectura", tipo: .dato) }
        if let u = try? ctx.env.resolve("/Documentos/lecturas.txt") {
            let t = "\(SistemaIAs.hora()) \(fuente)\n" + fs.prefix(12).map { "  " + $0 }.joined(separator: "\n") + "\n"
            SistemaIAs.agregar(t, a: u, maxLineas: 600)
        }
        return "(\(r.rawValue) aprendió \(min(12, fs.count)) frases de esto)\n"
    }

    /// Wikipedia: búsqueda y resumen del primer resultado.
    static func wikipedia(_ tema: String, quien: String) async throws -> (titulos: [String], resumen: String, url: String) {
        let q = tema.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? tema
        let cab = ["User-Agent: SwiftShell/1.0 (iPad; terminal de texto)", "Accept: application/json"]
        let (d, r) = try await Shell.peticion("https://es.wikipedia.org/w/api.php?action=opensearch&limit=5&format=json&search=\(q)",
                                              metodo: "GET", cabeceras: cab, cuerpo: nil, quien: quien)
        guard r.statusCode < 400, let j = try? JSONSerialization.jsonObject(with: d) as? [Any], j.count >= 4,
              let titulos = j[1] as? [String], let urls = j[3] as? [String], let primero = titulos.first else {
            throw ShErr("Wikipedia no respondió (HTTP \(r.statusCode))")
        }
        let t = primero.replacingOccurrences(of: " ", with: "_").addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? primero
        let (d2, r2) = try await Shell.peticion("https://es.wikipedia.org/api/rest_v1/page/summary/\(t)", metodo: "GET", cabeceras: cab, cuerpo: nil, quien: quien)
        var resumen = ""
        if r2.statusCode < 400, let o = try? JSONSerialization.jsonObject(with: d2) as? [String: Any] {
            resumen = o["extract"] as? String ?? ""
        }
        return (titulos, resumen, urls.first ?? "")
    }
}

extension Shell {

    static func herramientas() -> [String: Spec] {
        var c: [String: Spec] = [:]

        c["herramientas"] = Spec(help: "herramientas — lo que pueden usar las IAs (y tú)") { ctx in
            var out = """
            herramientas
              web <dirección|n|buscar …>   navegador de texto (enlaces numerados)
              busca <tema>                 Wikipedia (o DuckDuckGo): títulos y resumen
              lee <url|archivo> [n]        lo resume; si eres una IA, lo aprende
              resume <archivo> [n]         las n frases más importantes
              calcula <expresión>          cuentas (con Python): calcula 2**10/3
              pregunta <rol> <texto>       pregúntale a una IA
              guarda <clave> <valor> · saca [clave] · olvida <clave>   memoria propia
              chat <msg> · chat @rol <msg> · chat -a archivo · chat baja n   sala con todos
              python · http · curl · api-local · pkg · nota · diario …  (y toda la shell)

            """
            if let r = ctx.sh.rolIA {
                out += "Eres \(r.rawValue): lo que lees con 'busca' y 'lee' entra a tu mente y a /Documentos/lecturas.txt\n"
            } else {
                out += "Úsalas en una IA: ia curiosidad busca gatos · ia memoria lee es.wikipedia.org/wiki/Luna\n"
            }
            return out
        }

        c["busca"] = Spec(help: "busca <tema> — busca en Wikipedia (o DuckDuckGo) y resume") { ctx in
            let tema = ctx.args.joined(separator: " ")
            guard !tema.isEmpty else { throw ShErr("busca: ¿qué busco?") }
            let quien = ctx.sh.quienPide
            var out = ""
            var texto = ""
            var fuente = ""
            do {
                let w = try await Herramientas.wikipedia(tema, quien: quien)
                out += "Wikipedia: " + w.titulos.joined(separator: " · ") + "\n\n"
                texto = w.resumen
                fuente = w.url
                if !w.resumen.isEmpty { out += w.resumen + "\n" }
                if !w.url.isEmpty { out += "\n(más: web \(w.url))\n" }
            } catch {
                // sin Wikipedia: DuckDuckGo en el navegador de texto
                let q = tema.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? tema
                let r = try await Navegador.descarga("https://html.duckduckgo.com/html/?q=\(q)", quien: quien)
                let externos = r.pagina.enlaces.filter { !$0.contains("duckduckgo.com") }
                texto = r.pagina.lineas.joined(separator: "\n")
                fuente = r.url
                out += "DuckDuckGo (\(tema)):\n" + Herramientas.resume(texto, 4).map { "  · " + $0 }.joined(separator: "\n") + "\n"
                if !externos.isEmpty { out += "\nenlaces:\n" + externos.prefix(5).map { "  " + $0 }.joined(separator: "\n") + "\n" }
            }
            out += await Herramientas.aprende(ctx, fuente: "busca \(tema) — \(fuente)", frases: Herramientas.frases(texto))
            return out
        }

        c["lee"] = Spec(help: "lee <url|archivo> [n] — lee y resume (n frases); una IA además lo aprende") { ctx in
            guard let p = ctx.args.first else { throw ShErr("uso: lee <url|archivo> [frases]") }
            let n = Int(ctx.args.dropFirst().first ?? "5") ?? 5
            var texto: String
            var titulo = p
            let esArchivo = (try? ctx.env.resolve(p)).map { ctx.env.exists($0) } ?? false
            let pareceURL = p.contains("://") || p.hasPrefix("www.") || APILocal.local(p) != nil ||
                            (p.contains(".") && !p.hasPrefix("/") && !esArchivo)
            if pareceURL {
                let r = try await Navegador.descarga(p, quien: ctx.sh.quienPide)
                texto = r.pagina.lineas.joined(separator: "\n")
                titulo = r.pagina.titulo.isEmpty ? r.url : r.pagina.titulo
            } else {
                texto = Herramientas.comoTexto(try ctx.input([p]))
            }
            let resumen = Herramientas.resume(texto, n)
            var out = "── \(titulo) ──\n"
            out += resumen.isEmpty ? "(no encontré frases que resumir)\n" : resumen.map { "· " + $0 }.joined(separator: "\n") + "\n"
            out += await Herramientas.aprende(ctx, fuente: "lee \(p)", frases: resumen.isEmpty ? Herramientas.frases(texto) : resumen)
            return out
        }

        c["resume"] = Spec(help: "resume <archivo|-> [n] — las n frases más importantes de un texto") { ctx in
            let a = ctx.args
            let n = Int(a.dropFirst().first ?? "") ?? Int(a.first ?? "") ?? 3
            let archivos = a.first.map { Int($0) == nil && $0 != "-" ? [$0] : [] } ?? []
            let texto = Herramientas.comoTexto(try ctx.input(archivos))
            let r = Herramientas.resume(texto, n)
            return r.isEmpty ? "(texto demasiado corto para resumir)\n" : r.map { "· " + $0 }.joined(separator: "\n") + "\n"
        }

        c["calcula"] = Spec(help: "calcula <expresión> — cuentas con Python (math incluido): calcula sqrt(2)*10") { ctx in
            let e = ctx.args.isEmpty ? ctx.stdin : ctx.args.joined(separator: " ")
            guard !e.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { throw ShErr("calcula: ¿qué cuenta?") }
            let codigo = "from math import *\nprint(\(e))"
            let (salida, error) = await Shell.enHiloPython(PyInterprete(env: ctx.env), codigo, archivo: "<calcula>", interactivo: false)
            if let err = error { throw ShErr("calcula: " + (err.split(separator: "\n").last.map(String.init) ?? err)) }
            ctx.env.vars["RESULTADO"] = salida.trimmingCharacters(in: .whitespacesAndNewlines)
            return salida
        }

        c["pregunta"] = Spec(help: "pregunta <rol> <texto> — pregúntale algo a una IA y te contesta") { ctx in
            guard ctx.args.count >= 2, let r = RolMental(rawValue: ctx.args[0].lowercased()) else {
                throw ShErr("uso: pregunta <rol> <texto>  (roles: " + RolMental.allCases.map(\.rawValue).joined(separator: " ") + ")")
            }
            guard r != ctx.sh.rolIA else { throw ShErr("pregunta: no te preguntes a ti misma") }
            _ = await NyxNucleo.uno.preparar(memoria: nil)
            guard let m = await NyxNucleo.uno.consejo.mente(r) else { throw ShErr("pregunta: \(r.rawValue) no está lista") }
            let quien = ctx.sh.quienPide
            let texto = ctx.args.dropFirst().joined(separator: " ")
            await m.recibir(mensaje: texto, de: quien, tipo: .pregunta)
            var respuesta = await m.formularMensaje()?.contenido
            if respuesta == nil {                     // a veces calla: un segundo intento
                await m.recibir(mensaje: texto, de: quien, tipo: .pregunta)
                respuesta = await m.formularMensaje()?.contenido
            }
            guard let resp = respuesta else { return "\(r.rawValue) se queda en silencio\n" }
            if let yo = ctx.sh.rolIA, let mia = await NyxNucleo.uno.consejo.mente(yo) {
                await mia.recibir(mensaje: resp, de: r.rawValue, tipo: .dato)
            }
            let g = Herramientas.glosa(resp)
            return "\(r.rawValue): \(resp)" + (g != resp ? "\n  → \(g)" : "") + "\n"
        }

        func memoria(_ ctx: Ctx) throws -> (URL, [String: String]) {
            let u = try ctx.env.resolve("/Documentos/memoria.json")
            let d = (try? Data(contentsOf: u)).flatMap { try? JSONSerialization.jsonObject(with: $0) as? [String: String] } ?? [:]
            return (u, d)
        }
        func guardaMemoria(_ u: URL, _ m: [String: String]) throws {
            try FileManager.default.createDirectory(at: u.deletingLastPathComponent(), withIntermediateDirectories: true)
            try JSONSerialization.data(withJSONObject: m, options: [.prettyPrinted, .sortedKeys]).write(to: u)
        }

        c["guarda"] = Spec(help: "guarda <clave> <valor> — memoria propia (/Documentos/memoria.json)") { ctx in
            guard ctx.args.count >= 2 else { throw ShErr("uso: guarda <clave> <valor>") }
            var (u, m) = try memoria(ctx)
            m[ctx.args[0]] = ctx.args.dropFirst().joined(separator: " ")
            try guardaMemoria(u, m)
            return ""
        }

        c["saca"] = Spec(help: "saca [clave] — lo que guardaste con 'guarda'") { ctx in
            let (_, m) = try memoria(ctx)
            if let k = ctx.args.first {
                guard let v = m[k] else { throw ShErr("saca: no hay nada guardado como '\(k)'") }
                return v + "\n"
            }
            return m.isEmpty ? "(memoria vacía — guarda <clave> <valor>)\n" : m.keys.sorted().map { "\($0) = \(m[$0]!)" }.joined(separator: "\n") + "\n"
        }

        c["olvida"] = Spec(help: "olvida <clave> — borra algo de la memoria propia") { ctx in
            guard let k = ctx.args.first else { throw ShErr("uso: olvida <clave>") }
            var (u, m) = try memoria(ctx)
            guard m.removeValue(forKey: k) != nil else { throw ShErr("olvida: no había '\(k)'") }
            try guardaMemoria(u, m)
            return ""
        }

        return c
    }
}
