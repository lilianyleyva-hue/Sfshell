import Foundation
#if canImport(FoundationNetworking)
import FoundationNetworking
#endif

// Internet.swift — Nyx aprende de Wikipedia en español.
//
// Le das un tema ("delfín"), descarga el resumen de Wikipedia (sin clave),
// lo parte en frases, se queda solo con las útiles (ni muy cortas ni muy
// largas, sin paréntesis) y las lee con su memoria y con su lógica. Lo que
// viene de internet se guarda con menos confianza que lo que le enseñas tú.

enum Internet {
    struct Leido {
        let titulo: String
        let frases: [String]
    }

    enum Fallo: Error {
        case sinRed, noEncontrado, raro
    }

    private static func pide(_ url: URL) async throws -> (Data, Int) {
        var r = URLRequest(url: url)
        r.setValue("NyxCerebro/1.0 (app de Swift Playgrounds)", forHTTPHeaderField: "User-Agent")
        r.timeoutInterval = 20
        return try await withCheckedThrowingContinuation { (c: CheckedContinuation<(Data, Int), Error>) in
            let tarea = URLSession.shared.dataTask(with: r) { datos, respuesta, error in
                if let d = datos, let h = respuesta as? HTTPURLResponse {
                    c.resume(returning: (d, h.statusCode))
                } else {
                    c.resume(throwing: error ?? Fallo.sinRed)
                }
            }
            tarea.resume()
        }
    }

    private static func codifica(_ s: String) -> String {
        return s.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? s
    }

    /// El resumen de Wikipedia sobre un tema (si no está tal cual, lo busca).
    static func aprendeSobre(_ tema: String) async throws -> Leido {
        let limpio = tema.trimmingCharacters(in: .whitespacesAndNewlines)
        if limpio.isEmpty { throw Fallo.noEncontrado }
        var titulo = limpio.prefix(1).uppercased() + limpio.dropFirst()
        var leido = try? await resumen(titulo)
        if leido == nil {
            // búsqueda por título
            let q = limpio.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? limpio
            guard let u = URL(string: "https://es.wikipedia.org/w/rest.php/v1/search/title?q=\(q)&limit=1") else { throw Fallo.raro }
            let (d, codigo) = try await pide(u)
            guard codigo == 200,
                  let j = try? JSONSerialization.jsonObject(with: d) as? [String: Any],
                  let paginas = j["pages"] as? [[String: Any]],
                  let t = paginas.first?["title"] as? String else { throw Fallo.noEncontrado }
            titulo = t
            leido = try await resumen(titulo)
        }
        guard let l = leido else { throw Fallo.noEncontrado }
        return l
    }

    private static func resumen(_ titulo: String) async throws -> Leido {
        let ruta = codifica(titulo.replacingOccurrences(of: " ", with: "_"))
        guard let u = URL(string: "https://es.wikipedia.org/api/rest_v1/page/summary/\(ruta)") else { throw Fallo.raro }
        let (d, codigo) = try await pide(u)
        guard codigo == 200,
              let j = try? JSONSerialization.jsonObject(with: d) as? [String: Any],
              let texto = j["extract"] as? String, !texto.isEmpty else { throw Fallo.noEncontrado }
        let t = (j["title"] as? String) ?? titulo
        return Leido(titulo: t, frases: frasesUtiles(texto))
    }

    /// Solo las frases que sirven: sin paréntesis, ni muy cortas ni muy largas.
    static func frasesUtiles(_ texto: String, maximo: Int = 10) -> [String] {
        var sin = ""
        var nivel = 0
        for ch in texto {
            if ch == "(" || ch == "[" { nivel += 1; continue }
            if ch == ")" || ch == "]" { nivel = max(0, nivel - 1); continue }
            if nivel == 0 { sin.append(ch) }
        }
        var out: [String] = []
        for trozo in sin.replacingOccurrences(of: "\n", with: " ").components(separatedBy: ". ") {
            var f = trozo.trimmingCharacters(in: .whitespacesAndNewlines)
            while f.contains("  ") { f = f.replacingOccurrences(of: "  ", with: " ") }
            f = f.replacingOccurrences(of: " ,", with: ",")
            if f.hasSuffix(".") { f.removeLast() }
            let n = Palabras.tokens(f, max: 80).count
            if n < 4 || n > 30 { continue }
            out.append(f)
            if out.count >= maximo { break }
        }
        return out
    }

    /// La primera parte de una frase (antes de la coma o de "que"), para la lógica:
    /// "El delfín es un cetáceo odontoceto, que vive…" → "El delfín es un cetáceo odontoceto"
    static func clausula(_ f: String) -> String {
        var c = f
        for corte in [",", ";", ":", " que ", " cuyo ", " cuya ", " el cual ", " la cual "] {
            if let r = c.range(of: corte) { c = String(c[..<r.lowerBound]) }
        }
        return c.trimmingCharacters(in: .whitespaces)
    }
}
