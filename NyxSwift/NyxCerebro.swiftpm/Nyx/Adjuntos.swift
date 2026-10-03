import Foundation
#if canImport(FoundationNetworking)
import FoundationNetworking
#endif
#if canImport(UIKit)
import UIKit
#endif
#if canImport(PDFKit)
import PDFKit
#endif

// Adjuntos.swift — le mandas a Nyx archivos, fotos, videos, audios y enlaces,
// todos los que quieras: se ponen en cola y los lee uno detrás de otro.
//
//  · foto       → la mira (qué hay, texto escrito, caras, colores)
//  · video      → lo ve entero y escucha su audio
//  · audio      → lo escucha y lo pasa a texto (en el propio iPad)
//  · PDF        → lee su texto
//  · texto      → .txt .md .csv .json .html .swift … lo lee
//  · enlace     → lo descarga: si es una página, lee su texto; si es una foto,
//                 un video, un audio o un PDF, lo trata como tal
//
// Todo lo que lee lo aprenden su memoria (frases) y su lógica (hechos).

enum ClaseArchivo: String {
    case foto, video, audio, pdf, web, texto, desconocido

    var icono: String {
        switch self {
        case .foto: return "🖼"
        case .video: return "🎬"
        case .audio: return "🎵"
        case .pdf: return "📕"
        case .web: return "🔗"
        case .texto: return "📄"
        case .desconocido: return "📦"
        }
    }

    static func de(extension e: String) -> ClaseArchivo {
        let x = e.lowercased()
        if ["jpg", "jpeg", "png", "heic", "heif", "gif", "bmp", "tif", "tiff", "webp"].contains(x) { return .foto }
        if ["mov", "mp4", "m4v", "avi", "mkv", "3gp"].contains(x) { return .video }
        if ["m4a", "mp3", "wav", "aac", "aif", "aiff", "caf", "flac", "ogg", "opus"].contains(x) { return .audio }
        if x == "pdf" { return .pdf }
        if ["html", "htm", "xhtml"].contains(x) { return .web }
        if ["txt", "md", "markdown", "csv", "tsv", "json", "xml", "swift", "py", "js", "c", "h", "cpp", "java",
            "rtf", "log", "yaml", "yml", "ini", "srt", "vtt", "tex"].contains(x) { return .texto }
        return .desconocido
    }

    /// Por el tipo que dice el servidor ("text/html; charset=utf-8").
    static func de(mime m: String) -> ClaseArchivo {
        let x = m.lowercased()
        if x.hasPrefix("image/") { return .foto }
        if x.hasPrefix("video/") { return .video }
        if x.hasPrefix("audio/") { return .audio }
        if x.contains("pdf") { return .pdf }
        if x.contains("html") { return .web }
        if x.hasPrefix("text/") || x.contains("json") || x.contains("xml") { return .texto }
        return .desconocido
    }

    var extensionTipica: String {
        switch self {
        case .foto: return "jpg"
        case .video: return "mp4"
        case .audio: return "m4a"
        case .pdf: return "pdf"
        case .web: return "html"
        case .texto: return "txt"
        case .desconocido: return "bin"
        }
    }
}

/// Lo que sacó de un adjunto.
struct Contenido {
    var titulo: String
    var clase: ClaseArchivo
    var frases: [String] = []
    var percepcion: Percepcion? = nil
    var escenas: [String] = []
    var nota = ""
}

enum Lector {
    // MARK: texto

    /// Parte un texto en frases que se pueden aprender (de 3 a 40 palabras).
    static func frases(de original: String) -> [String] {
        // saltos de Windows ("\r\n") y de Mac antiguo ("\r") → "\n"
        var texto = original.replacingOccurrences(of: "\r\n", with: "\n").replacingOccurrences(of: "\r", with: "\n")
        // libros con las líneas cortadas a mitad de frase: una línea suelta es un espacio,
        // y solo la línea en blanco separa
        let lineas = texto.components(separatedBy: "\n")
        let largas = lineas.filter { $0.count >= 50 }
        let cortadas = largas.filter { l in
            guard let u = l.trimmingCharacters(in: .whitespaces).last else { return false }
            return !".!?:;»\"".contains(u)
        }
        if largas.count >= 5 && cortadas.count * 2 > largas.count {
            texto = texto.replacingOccurrences(of: "\n\n", with: "\u{1}")
                .replacingOccurrences(of: "\n", with: " ")
                .replacingOccurrences(of: "\u{1}", with: "\n")
        }
        var out: [String] = []
        var actual = ""
        func cierra() {
            let f = actual.trimmingCharacters(in: .whitespacesAndNewlines)
            actual = ""
            let n = Palabras.tokens(f, max: 60).count
            if n >= 3 && n <= 40 { out.append(f) }
        }
        let cs = Array(texto)
        for (i, ch) in cs.enumerated() {
            if ch == "\n" || ch == "!" || ch == "?" || ch == ";" || ch == "•" {
                cierra()
                continue
            }
            if ch == "." {
                // "3.5" no corta la frase
                let entreCifras = i > 0 && i + 1 < cs.count && cs[i - 1].isNumber && cs[i + 1].isNumber
                if !entreCifras { cierra(); continue }
            }
            actual.append(ch)
        }
        cierra()
        return out
    }

    static func textoPlano(_ d: Data) -> String? {
        if let s = String(data: d, encoding: .utf8) { return s }
        if let s = String(data: d, encoding: .isoLatin1) { return s }
        return String(data: d, encoding: .windowsCP1252)
    }

    /// El texto de una página web: sin scripts, estilos ni etiquetas.
    static func textoDeHTML(_ html: String) -> (titulo: String, texto: String) {
        var s = html
        var titulo = ""
        if let a = s.range(of: "<title", options: .caseInsensitive),
           let b = s.range(of: ">", range: a.upperBound ..< s.endIndex),
           let c = s.range(of: "</title>", options: .caseInsensitive, range: b.upperBound ..< s.endIndex) {
            titulo = String(s[b.upperBound ..< c.lowerBound]).trimmingCharacters(in: .whitespacesAndNewlines)
        }
        for etiqueta in ["head", "script", "style", "noscript", "svg", "nav", "footer", "header", "form"] {
            s = quitaBloques(s, etiqueta)
        }
        // los párrafos y títulos acaban en salto de línea
        for fin in ["</p>", "</h1>", "</h2>", "</h3>", "</h4>", "</li>", "<br>", "<br/>", "<br />", "</div>", "</tr>"] {
            s = s.replacingOccurrences(of: fin, with: "\n", options: .caseInsensitive)
        }
        var limpio = ""
        var dentro = false
        for ch in s {
            if ch == "<" { dentro = true; continue }
            if ch == ">" { dentro = false; limpio.append(" "); continue }
            if !dentro { limpio.append(ch) }
        }
        for (a, b) in [("&nbsp;", " "), ("&amp;", "&"), ("&quot;", "\""), ("&#39;", "'"), ("&apos;", "'"),
                       ("&lt;", "<"), ("&gt;", ">"), ("&aacute;", "á"), ("&eacute;", "é"), ("&iacute;", "í"),
                       ("&oacute;", "ó"), ("&uacute;", "ú"), ("&ntilde;", "ñ"), ("&iquest;", "¿"), ("&iexcl;", "¡")] {
            limpio = limpio.replacingOccurrences(of: a, with: b)
            titulo = titulo.replacingOccurrences(of: a, with: b)
        }
        // sin los corchetes de las notas: [1], [2]…
        var sinNotas = ""
        var nivel = 0
        for ch in limpio {
            if ch == "[" { nivel += 1; continue }
            if ch == "]" { nivel = max(0, nivel - 1); continue }
            if nivel == 0 { sinNotas.append(ch) }
        }
        let lineas = sinNotas.components(separatedBy: "\n").map {
            $0.split(separator: " ", omittingEmptySubsequences: true).joined(separator: " ")
        }
        return (titulo, lineas.filter { !$0.isEmpty }.joined(separator: "\n"))
    }

    private static func quitaBloques(_ s: String, _ etiqueta: String) -> String {
        var out = s
        while let a = out.range(of: "<" + etiqueta, options: .caseInsensitive),
              let b = out.range(of: "</" + etiqueta + ">", options: .caseInsensitive, range: a.upperBound ..< out.endIndex) {
            out.removeSubrange(a.lowerBound ..< b.upperBound)
        }
        return out
    }

    static func textoDePDF(_ url: URL) -> String? {
        #if canImport(PDFKit)
        return PDFDocument(url: url)?.string
        #else
        return nil
        #endif
    }

    /// Los enlaces que hay en un texto, y lo que queda sin ellos.
    static func enlaces(en texto: String) -> (urls: [URL], resto: String) {
        var urls: [URL] = []
        var resto: [String] = []
        for trozo in texto.split(whereSeparator: { $0 == " " || $0 == "\n" || $0 == "\t" }) {
            var w = String(trozo)
            while let u = w.last, ",;)»\"'".contains(u) { w.removeLast() }
            let bajo = w.lowercased()
            if bajo.hasPrefix("http://") || bajo.hasPrefix("https://") || bajo.hasPrefix("www.") {
                if let u = URL(string: bajo.hasPrefix("www.") ? "https://" + w : w) { urls.append(u); continue }
            }
            resto.append(String(trozo))
        }
        return (urls, resto.joined(separator: " "))
    }

    // MARK: archivos

    /// Lee un archivo del iPad (ya copiado a un sitio donde se puede leer).
    static func archivo(_ url: URL, nombre: String, progreso: @escaping (String) -> Void) async -> Contenido {
        let clase = ClaseArchivo.de(extension: url.pathExtension)
        var c = Contenido(titulo: nombre, clase: clase)
        switch clase {
        case .foto:
            c.percepcion = mira(url)
            if c.percepcion == nil { c.nota = "no pude abrir la foto" }
        case .video:
            #if canImport(AVFoundation) && canImport(Speech) && canImport(CoreTransferable) && canImport(UniformTypeIdentifiers) && canImport(UIKit)
            let visto = await Video.mira(url) { (a: Double, e: String) in progreso("\(Int(a * 100))% · \(e)") }
            c.percepcion = visto.percepcion
            c.escenas = visto.escenas.map { $0.recuerdo }
            c.frases = visto.escenas.compactMap { $0.dicho.isEmpty ? nil : $0.dicho }
            #else
            c.nota = "aquí no puedo ver videos"
            #endif
        case .audio:
            progreso("escuchando…")
            c.frases = await escucha(url)
            if c.frases.isEmpty { c.nota = "no entendí palabras en el audio" }
        case .pdf:
            if let t = textoDePDF(url) { c.frases = frases(de: t) } else { c.nota = "no pude leer el PDF" }
        case .web:
            if let d = try? Data(contentsOf: url), let h = textoPlano(d) {
                let w = textoDeHTML(h)
                if !w.titulo.isEmpty { c.titulo = w.titulo }
                c.frases = frases(de: w.texto)
            }
        case .texto, .desconocido:
            if let d = try? Data(contentsOf: url), let t = textoPlano(d), !t.contains("\u{0}") {
                c.clase = .texto
                c.frases = frases(de: t)
            } else {
                c.nota = "no sé leer este tipo de archivo (.\(url.pathExtension))"
            }
        }
        return c
    }

    private static func mira(_ url: URL) -> Percepcion? {
        #if canImport(Vision) && canImport(UIKit)
        guard let img = UIImage(contentsOfFile: url.path) else { return nil }
        var p = Ojos.mira(img)
        p.origen = "la foto"
        return p
        #else
        return nil
        #endif
    }

    private static func escucha(_ url: URL) async -> [String] {
        #if canImport(AVFoundation) && canImport(Speech) && canImport(CoreTransferable) && canImport(UniformTypeIdentifiers) && canImport(UIKit)
        let trozos = await Video.escucha(url)
        return trozos.map { $0.texto }.filter { Palabras.tokens($0).count >= 2 }
        #else
        return []
        #endif
    }

    // MARK: enlaces

    /// Descarga un enlace y lo lee según lo que sea.
    static func enlace(_ url: URL, progreso: @escaping (String) -> Void) async -> Contenido {
        progreso("descargando…")
        guard let bajado = try? await descarga(url) else {
            return Contenido(titulo: url.absoluteString, clase: .web, nota: "no pude descargar el enlace")
        }
        let (local, mime) = bajado
        defer { try? FileManager.default.removeItem(at: local) }
        var clase = ClaseArchivo.de(mime: mime)
        if clase == .desconocido { clase = ClaseArchivo.de(extension: url.pathExtension) }
        var c = await archivo(local, nombre: url.host ?? url.absoluteString, progreso: progreso)
        c.clase = clase == .desconocido ? c.clase : clase
        if c.titulo == local.lastPathComponent { c.titulo = url.absoluteString }
        return c
    }

    /// Descarga a un archivo temporal con la extensión que le toca.
    static func descarga(_ url: URL) async throws -> (URL, String) {
        var r = URLRequest(url: url)
        r.setValue("Mozilla/5.0 (iPad) NyxCerebro/1.0", forHTTPHeaderField: "User-Agent")
        r.timeoutInterval = 60
        return try await withCheckedThrowingContinuation { (c: CheckedContinuation<(URL, String), Error>) in
            let tarea = URLSession.shared.downloadTask(with: r) { lugar, respuesta, error in
                guard let l = lugar, let h = respuesta as? HTTPURLResponse, (200 ..< 300).contains(h.statusCode) else {
                    c.resume(throwing: error ?? Internet.Fallo.sinRed)
                    return
                }
                let mime = h.value(forHTTPHeaderField: "Content-Type") ?? ""
                var clase = ClaseArchivo.de(mime: mime)
                if clase == .desconocido { clase = ClaseArchivo.de(extension: url.pathExtension) }
                let ext = clase == .desconocido ? (url.pathExtension.isEmpty ? "bin" : url.pathExtension) : clase.extensionTipica
                let destino = FileManager.default.temporaryDirectory.appendingPathComponent("nyx_web_\(UUID().uuidString).\(ext)")
                do {
                    try FileManager.default.moveItem(at: l, to: destino)
                    c.resume(returning: (destino, mime))
                } catch {
                    c.resume(throwing: error)
                }
            }
            tarea.resume()
        }
    }

    /// Copia un archivo que eliges en Archivos a un sitio propio (para poder leerlo luego).
    static func copiaDeArchivos(_ url: URL) -> URL? {
        #if os(iOS) || os(macOS)
        let permiso = url.startAccessingSecurityScopedResource()
        defer { if permiso { url.stopAccessingSecurityScopedResource() } }
        #endif
        let destino = FileManager.default.temporaryDirectory.appendingPathComponent("nyx_\(UUID().uuidString)_" + url.lastPathComponent)
        do {
            try FileManager.default.copyItem(at: url, to: destino)
            return destino
        } catch {
            return nil
        }
    }
}
