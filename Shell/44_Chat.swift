import Foundation

// ============================================================
// MARK: - Sala de chat: tú y las 18 IAs, con archivos
// ============================================================
// Un chat de equipo compartido. Todos ven todo; con @rol le hablas a
// una IA concreta y te contesta. Los archivos adjuntos quedan en la
// sala y cualquiera (tú o una IA) los baja a su propio espacio.
//
//   chat                          últimos mensajes
//   chat <mensaje>                escribe (con @critico le hablas a esa IA)
//   chat -a <archivo> [mensaje]   adjunta un archivo
//   chat archivos                 adjuntos de la sala
//   chat baja <n|nombre> [dest]   copia un adjunto a tu espacio
//   chat de <rol> · chat busca <texto> · chat leer [n]
//   chat limpia                   (solo el humano) vacía la sala
// Guardado en /srv/chat (sala.jsonl y adjuntos/).

final class SalaChat: @unchecked Sendable {
    static let una = SalaChat()

    struct Mensaje: Codable {
        let id: Int
        let hora: String
        let de: String          // "humano" o el rol
        let para: String?       // nil = todos
        let texto: String
        let adjunto: String?
    }

    private let l = NSLock()
    private var cache: [Mensaje]?

    var dir: URL { ShellEnv.raizHumano.appendingPathComponent("srv/chat", isDirectory: true) }
    var adjuntos: URL { dir.appendingPathComponent("adjuntos", isDirectory: true) }
    private var archivo: URL { dir.appendingPathComponent("sala.jsonl") }

    func mensajes() -> [Mensaje] {
        l.conCandado {
            if let c = cache { return c }
            let t = (try? String(contentsOf: archivo, encoding: .utf8)) ?? ""
            let ms = t.split(separator: "\n").compactMap { try? JSONDecoder().decode(Mensaje.self, from: Data($0.utf8)) }
            cache = ms
            return ms
        }
    }

    @discardableResult
    func publica(de: String, para: String?, texto: String, adjunto: String? = nil) -> Mensaje {
        _ = mensajes()
        return l.conCandado {
            var ms = cache ?? []
            let m = Mensaje(id: (ms.last?.id ?? 0) + 1, hora: SistemaIAs.hora(), de: de, para: para, texto: texto, adjunto: adjunto)
            ms.append(m)
            if ms.count > 500 { ms.removeFirst(ms.count - 500) }
            cache = ms
            try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
            let lineas = ms.compactMap { try? JSONEncoder().encode($0) }.compactMap { String(data: $0, encoding: .utf8) }
            try? (lineas.joined(separator: "\n") + "\n").write(to: archivo, atomically: true, encoding: .utf8)
            return m
        }
    }

    func limpia() {
        l.conCandado {
            cache = []
            try? FileManager.default.removeItem(at: archivo)
        }
    }

    static func nombre(_ quien: String) -> String { quien == "humano" ? "tú" : quien }

    static func formato(_ m: Mensaje) -> String {
        var s = "#\(m.id) \(m.hora) \(nombre(m.de))"
        if let p = m.para { s += " → @\(nombre(p))" }
        s += ": \(m.texto)"
        if let a = m.adjunto { s += "  📎 \(a)" }
        // si habla en Resh, la glosa debajo
        let g = Herramientas.glosa(m.texto)
        if m.de != "humano", g != m.texto { s += "\n      (\(g))" }
        return s
    }
}

extension Shell {

    static func chat() -> [String: Spec] {
        var c: [String: Spec] = [:]
        let fm = FileManager.default

        /// Una IA contesta en la sala (su mente oye el mensaje y habla).
        func responde(_ r: RolMental, a quien: String, texto: String) async -> SalaChat.Mensaje? {
            guard let m = await NyxNucleo.uno.consejo.mente(r) else { return nil }
            await m.recibir(mensaje: texto, de: quien, tipo: .pregunta)
            var dicho = await m.formularMensaje()?.contenido
            if dicho == nil {
                await m.recibir(mensaje: texto, de: quien, tipo: .pregunta)
                dicho = await m.formularMensaje()?.contenido
            }
            guard let d = dicho else { return nil }
            return SalaChat.una.publica(de: r.rawValue, para: quien, texto: d)
        }

        c["chat"] = Spec(help: "chat [mensaje | -a archivo | archivos | baja n | de rol | busca texto] — sala con las IAs") { ctx in
            let sala = SalaChat.una
            let quien = ctx.sh.quienPide
            var a = ctx.args
            _ = await NyxNucleo.uno.preparar(memoria: nil)

            if a.isEmpty || a == ["leer"] || (a.first == "leer" && a.count == 2) {
                let n = a.count == 2 ? (Int(a[1]) ?? 20) : 20
                let ms = sala.mensajes().suffix(n)
                return ms.isEmpty ? "la sala está vacía — chat hola a todos · chat @critico ¿qué opinas?\n"
                                  : ms.map(SalaChat.formato).joined(separator: "\n") + "\n"
            }

            switch a[0] {
            case "ayuda", "-h", "help":
                return """
                chat — sala compartida: tú y las 18 IAs
                  chat                          últimos mensajes  ·  chat leer 50
                  chat hola a todos             escribe (contesta alguna IA)
                  chat @critico ¿qué opinas?    le hablas a una IA y te contesta
                  chat -a notas.txt mira esto   adjunta un archivo
                  chat archivos                 adjuntos de la sala
                  chat baja 3 [destino]         copia el adjunto del mensaje #3 a tu espacio
                  chat de narrativa · chat busca gato
                las IAs también lo usan desde su shell (ia empatia chat se ko)

                """

            case "archivos":
                let ms = sala.mensajes().filter { $0.adjunto != nil }
                guard !ms.isEmpty else { return "no hay adjuntos — chat -a <archivo> [mensaje]\n" }
                return ms.map { m -> String in
                    let u = sala.adjuntos.appendingPathComponent("\(m.id)-\(m.adjunto!)")
                    let t = (try? fm.attributesOfItem(atPath: u.path)[.size] as? Int) ?? 0
                    return "#\(m.id) \(m.adjunto!) (\(humanSize(t ?? 0))) de \(SalaChat.nombre(m.de))"
                }.joined(separator: "\n") + "\nbájalo con: chat baja <número>\n"

            case "baja", "descarga", "bajar":
                guard a.count >= 2 else { throw ShErr("uso: chat baja <n|nombre> [destino]") }
                let clave = a[1].trimmingCharacters(in: CharacterSet(charactersIn: "#"))
                guard let m = sala.mensajes().last(where: { $0.adjunto != nil && (String($0.id) == clave || $0.adjunto == clave) }) else {
                    throw ShErr("chat: no hay adjunto '\(a[1])' (mira 'chat archivos')")
                }
                let origen = sala.adjuntos.appendingPathComponent("\(m.id)-\(m.adjunto!)")
                let destino = try ctx.env.resolve(a.count > 2 ? a[2] : (ctx.sh.rolIA != nil ? "/buzon/\(m.adjunto!)" : m.adjunto!))
                try fm.createDirectory(at: destino.deletingLastPathComponent(), withIntermediateDirectories: true)
                if fm.fileExists(atPath: destino.path) { try fm.removeItem(at: destino) }
                try fm.copyItem(at: origen, to: destino)
                return "bajado \(m.adjunto!) → \(ctx.env.vpath(destino))\n"

            case "de":
                guard a.count >= 2 else { throw ShErr("uso: chat de <rol|humano>") }
                let r = a[1] == "tú" || a[1] == "yo" ? "humano" : a[1]
                let ms = sala.mensajes().filter { $0.de == r || $0.para == r }.suffix(30)
                return ms.isEmpty ? "nada de \(a[1])\n" : ms.map(SalaChat.formato).joined(separator: "\n") + "\n"

            case "busca":
                let t = a.dropFirst().joined(separator: " ").lowercased()
                guard !t.isEmpty else { throw ShErr("uso: chat busca <texto>") }
                let ms = sala.mensajes().filter { $0.texto.lowercased().contains(t) || ($0.adjunto ?? "").lowercased().contains(t) }
                return ms.isEmpty ? "sin resultados\n" : ms.suffix(30).map(SalaChat.formato).joined(separator: "\n") + "\n"

            case "limpia":
                guard ctx.sh.rolIA == nil else { throw ShErr("chat limpia: solo el humano") }
                sala.limpia()
                return "sala vacía (los adjuntos siguen en /srv/chat/adjuntos)\n"

            default:
                break
            }

            // ---- escribir (con adjunto opcional) ----
            var adjunto: URL? = nil
            if a.first == "-a" {
                guard a.count >= 2 else { throw ShErr("uso: chat -a <archivo> [mensaje]") }
                let u = try ctx.env.resolve(a[1])
                guard ctx.env.exists(u), !ctx.env.isDir(u) else { throw ShErr("chat: \(a[1]) no es un archivo") }
                let t = (try? fm.attributesOfItem(atPath: u.path)[.size] as? Int) ?? 0
                guard (t ?? 0) <= 5 << 20 else { throw ShErr("chat: el adjunto pasa de 5 MB") }
                adjunto = u
                a.removeFirst(2)
            }
            // @mención: a quién va (se quita del texto; se ve en la flecha)
            var para: RolMental? = nil
            var paraHumano = false
            if let men = a.first(where: { $0.hasPrefix("@") }) {
                let n = men.dropFirst().trimmingCharacters(in: .punctuationCharacters).lowercased()
                para = RolMental(rawValue: n)
                paraHumano = ["humano", "tú", "tu", "yo"].contains(n)
                if para == nil, !paraHumano, n != "todos" {
                    throw ShErr("chat: no hay ninguna IA llamada '\(n)'")
                }
            }
            var texto = a.filter { !$0.hasPrefix("@") }.joined(separator: " ")
            if texto.isEmpty, let u = adjunto { texto = "te paso \(u.lastPathComponent)" }
            guard !texto.isEmpty else { throw ShErr("chat: ¿qué escribo?") }
            let m = sala.publica(de: quien, para: para?.rawValue ?? (paraHumano ? "humano" : nil), texto: texto, adjunto: adjunto?.lastPathComponent)
            if let u = adjunto {
                try fm.createDirectory(at: sala.adjuntos, withIntermediateDirectories: true)
                try fm.copyItem(at: u, to: sala.adjuntos.appendingPathComponent("\(m.id)-\(u.lastPathComponent)"))
            }
            var out = SalaChat.formato(m) + "\n"

            // las mentes lo oyen; contesta la mencionada, o (si escribe el
            // humano a todos) alguna que tenga algo que decir
            let limpio = texto
            if let r = para, r != ctx.sh.rolIA {
                if let resp = await responde(r, a: quien, texto: limpio) { out += SalaChat.formato(resp) + "\n" }
                else { out += "  (\(r.rawValue) lo leyó pero no dijo nada)\n" }
            } else if quien == "humano" {
                var contestaron = 0
                for r in RolMental.allCases.shuffled() where contestaron < 2 {
                    if let resp = await responde(r, a: quien, texto: limpio) { out += SalaChat.formato(resp) + "\n"; contestaron += 1 }
                }
            } else if let yo = ctx.sh.rolIA {
                // una IA a todos: las demás lo oyen (sin contestar en cadena)
                await NyxNucleo.uno.consejo.difundir(de: yo, mensaje: limpio)
            }
            return out
        }
        c["sala"] = c["chat"]
        return c
    }
}
