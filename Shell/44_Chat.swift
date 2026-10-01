import Foundation

// ============================================================
// MARK: - Chat: salas, conversaciones entre IAs y archivos
// ============================================================
// Sala "general" para todos, y salas propias que crea cualquiera
// (tú o una IA) con quien quiera. Las IAs pueden conversar entre
// ellas por turnos: cada una oye lo último que se dijo y contesta
// con su propia mente.
//
//   chat                            últimos mensajes de general
//   chat <mensaje>                  escribe en general (@rol: le hablas a esa IA)
//   chat -a <archivo> [mensaje]     adjunta un archivo
//   chat todas <pregunta>           las 18 contestan, cada una a su manera
//   chat salas                      salas que existen (y quién está)
//   chat crea <sala> @rol @rol…     crea una sala (@todas = las 18)
//   chat invita <sala> @rol…        mete a más   ·   chat sal <sala>
//   chat en <sala> [mensaje|-a …]   lee o escribe en esa sala
//   chat archivos · chat baja <n> [dest] · chat de <rol> · chat busca <texto>
//   chat todo                       mensajes de todas las salas
//   chat limpia                     (solo el humano)
//   conversa <rol> <rol> [rol…] [turnos] [tema]
//                                   conversan entre ellas en su propia sala
// Guardado en /srv/chat (sala.jsonl, salas.json y adjuntos/).

final class SalaChat: @unchecked Sendable {
    static let una = SalaChat()

    struct Mensaje: Codable {
        let id: Int
        let hora: String
        let de: String          // "humano" o el rol
        let para: String?       // nil = todos los de la sala
        let texto: String
        let adjunto: String?
        var sala: String?       // nil = general
    }

    struct Sala: Codable {
        var miembros: [String]   // roles y/o "humano"
        let creador: String
        let creada: String
    }

    private let l = NSLock()
    private var cache: [Mensaje]?

    var dir: URL { ShellEnv.raizHumano.appendingPathComponent("srv/chat", isDirectory: true) }
    var adjuntos: URL { dir.appendingPathComponent("adjuntos", isDirectory: true) }
    private var archivo: URL { dir.appendingPathComponent("sala.jsonl") }
    private var archivoSalas: URL { dir.appendingPathComponent("salas.json") }

    func mensajes() -> [Mensaje] {
        l.conCandado {
            if let c = cache { return c }
            let t = (try? String(contentsOf: archivo, encoding: .utf8)) ?? ""
            let ms = t.split(separator: "\n").compactMap { try? JSONDecoder().decode(Mensaje.self, from: Data($0.utf8)) }
            cache = ms
            return ms
        }
    }

    func de(sala: String) -> [Mensaje] {
        mensajes().filter { ($0.sala ?? "general") == sala }
    }

    @discardableResult
    func publica(de: String, para: String?, texto: String, adjunto: String? = nil, sala: String = "general") -> Mensaje {
        _ = mensajes()
        return l.conCandado {
            var ms = cache ?? []
            let m = Mensaje(id: (ms.last?.id ?? 0) + 1, hora: SistemaIAs.hora(), de: de, para: para,
                            texto: texto, adjunto: adjunto, sala: sala == "general" ? nil : sala)
            ms.append(m)
            if ms.count > 1000 { ms.removeFirst(ms.count - 1000) }
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

    // ---------- salas ----------

    func salas() -> [String: Sala] {
        l.conCandado {
            guard let d = try? Data(contentsOf: archivoSalas) else { return [:] }
            return (try? JSONDecoder().decode([String: Sala].self, from: d)) ?? [:]
        }
    }

    func guarda(_ s: [String: Sala]) {
        l.conCandado {
            try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
            if let d = try? JSONEncoder().encode(s) { try? d.write(to: archivoSalas) }
        }
    }

    /// Crea la sala si no existe y se asegura de que estén esos miembros.
    @discardableResult
    func abre(_ nombre: String, creador: String, miembros: [String]) -> Sala {
        var todas = salas()
        var s = todas[nombre] ?? Sala(miembros: [], creador: creador, creada: SistemaIAs.hora())
        for m in [creador] + miembros where !s.miembros.contains(m) { s.miembros.append(m) }
        todas[nombre] = s
        guarda(todas)
        return s
    }

    /// Miembros de una sala ("general": todos).
    func miembros(_ sala: String) -> [String] {
        if sala == "general" { return ["humano"] + RolMental.allCases.map(\.rawValue) }
        return salas()[sala]?.miembros ?? []
    }

    static func nombre(_ quien: String) -> String { quien == "humano" ? "tú" : quien }

    static func formato(_ m: Mensaje) -> String {
        var s = "#\(m.id) "
        if let sala = m.sala { s += "[\(sala)] " }
        s += "\(m.hora) \(nombre(m.de))"
        if let p = m.para { s += " → @\(nombre(p))" }
        s += ": \(m.texto)"
        if let a = m.adjunto { s += "  📎 \(a)" }
        let g = Herramientas.glosa(m.texto)
        if m.de != "humano", g != m.texto { s += "\n      (\(g))" }
        return s
    }
}

extension Shell {

    static func chat() -> [String: Spec] {
        var c: [String: Spec] = [:]
        c["chat"] = Spec(help: "chat [mensaje | -a archivo | todas | salas | crea | en | archivos | baja …] — salas con las IAs") { ctx in
            try await Shell.ordenChat(ctx, ctx.args)
        }
        c["sala"] = c["chat"]
        c["conversa"] = Spec(help: "conversa <rol> <rol> [rol…] [turnos] [tema] — las IAs conversan entre ellas") { ctx in
            try await Shell.ordenConversa(ctx)
        }
        return c
    }

    /// Una IA oye un mensaje y contesta con su mente (o calla).
    static func respuestaDe(_ r: RolMental, oye texto: String, de quien: String) async -> String? {
        guard let m = await NyxNucleo.uno.consejo.mente(r) else { return nil }
        await m.recibir(mensaje: texto, de: quien, tipo: .pregunta)
        if let d = await m.formularMensaje()?.contenido { return d }
        await m.recibir(mensaje: texto, de: quien, tipo: .pregunta)
        return await m.formularMensaje()?.contenido
    }

    static func listaMensajes(_ ms: ArraySlice<SalaChat.Mensaje>) -> String {
        var out = ""
        for m in ms { out += SalaChat.formato(m) + "\n" }
        return out
    }

    // ---------- chat ----------

    static func ordenChat(_ ctx: Ctx, _ args: [String]) async throws -> String {
        let sala = SalaChat.una
        let quien = ctx.sh.quienPide
        let fm = FileManager.default
        _ = await NyxNucleo.uno.preparar(memoria: nil)
        let a = args

        if a.isEmpty || a.first == "leer" {
            let n = a.count == 2 ? (Int(a[1]) ?? 20) : 20
            let ms = sala.de(sala: "general").suffix(n)
            return ms.isEmpty ? "la sala general está vacía — chat hola a todos · chat ayuda\n" : listaMensajes(ms)
        }

        switch a[0] {
        case "ayuda", "-h", "help":
            return """
            chat — salas compartidas: tú y las 18 IAs
              chat                          últimos mensajes de general
              chat hola a todos             escribe en general (contesta alguna IA)
              chat @critico ¿qué opinas?    le hablas a una IA y te contesta
              chat todas ¿qué es el tiempo? las 18 contestan, cada una a su manera
              chat -a notas.txt mira esto   adjunta un archivo
              chat salas                    salas que existen
              chat crea taller @codigo @logica    crea una sala (@todas = las 18)
              chat en taller hola equipo    escribe en esa sala (sin mensaje: la lees)
              chat invita taller @critico · chat sal taller
              chat archivos · chat baja 3 · chat de narrativa · chat busca gato · chat todo
              conversa critico empatia 6 la verdad   dos o más IAs conversan entre ellas

            """

        case "todo":
            let ms = sala.mensajes().suffix(40)
            return ms.isEmpty ? "no hay mensajes\n" : listaMensajes(ms)

        case "salas":
            var out = "general — todos\n"
            for (n, s) in sala.salas().sorted(by: { $0.key < $1.key }) {
                let cuantos = sala.de(sala: n).count
                out += "\(n) — \(s.miembros.map(SalaChat.nombre).joined(separator: ", ")) · \(cuantos) mensajes (creó \(SalaChat.nombre(s.creador)))\n"
            }
            return out

        case "crea", "invita":
            guard a.count >= 2 else { throw ShErr("uso: chat \(a[0]) <sala> @rol @rol…") }
            let nombre = a[1].lowercased().trimmingCharacters(in: CharacterSet(charactersIn: "#"))
            guard nombre != "general", !nombre.isEmpty else { throw ShErr("chat: elige otro nombre de sala") }
            if a[0] == "invita", sala.salas()[nombre] == nil { throw ShErr("chat: no existe la sala '\(nombre)'") }
            var miembros: [String] = []
            for x in a.dropFirst(2) where x.hasPrefix("@") {
                let n = x.dropFirst().lowercased().trimmingCharacters(in: .punctuationCharacters)
                if n == "todas" { miembros += RolMental.allCases.map(\.rawValue); continue }
                if ["tú", "tu", "humano", "yo"].contains(n) { miembros.append("humano"); continue }
                guard RolMental(rawValue: n) != nil else { throw ShErr("chat: no hay ninguna IA llamada '\(n)'") }
                miembros.append(n)
            }
            if quien != "humano" { miembros.append("humano") }     // tú puedes ver todas
            let s = sala.abre(nombre, creador: quien, miembros: miembros)
            let aviso = a[0] == "crea" ? "creó la sala" : "invitó a más gente a"
            sala.publica(de: quien, para: nil, texto: "\(aviso) \(nombre)", sala: nombre)
            return "sala \(nombre): \(s.miembros.map(SalaChat.nombre).joined(separator: ", "))\n"

        case "sal", "salir":
            guard a.count >= 2 else { throw ShErr("uso: chat sal <sala>") }
            var todas = sala.salas()
            guard var s = todas[a[1]] else { throw ShErr("chat: no existe la sala '\(a[1])'") }
            s.miembros.removeAll { $0 == quien }
            todas[a[1]] = s
            sala.guarda(todas)
            sala.publica(de: quien, para: nil, texto: "salió de la sala", sala: a[1])
            return "saliste de \(a[1])\n"

        case "en":
            guard a.count >= 2 else { throw ShErr("uso: chat en <sala> [mensaje]") }
            let nombre = a[1].lowercased()
            guard nombre == "general" || sala.salas()[nombre] != nil else {
                throw ShErr("chat: no existe la sala '\(nombre)' (chat crea \(nombre) @rol …)")
            }
            if a.count == 2 {
                let ms = sala.de(sala: nombre).suffix(30)
                return ms.isEmpty ? "la sala \(nombre) está vacía\n" : listaMensajes(ms)
            }
            guard quien == "humano" || sala.miembros(nombre).contains(quien) else {
                throw ShErr("chat: no estás en la sala \(nombre)")
            }
            return try await escribe(ctx, Array(a.dropFirst(2)), sala: nombre)

        case "todas":
            let pregunta = a.dropFirst().joined(separator: " ")
            guard !pregunta.isEmpty else { throw ShErr("uso: chat todas <pregunta>") }
            let m = sala.publica(de: quien, para: nil, texto: pregunta)
            var out = SalaChat.formato(m) + "\n"
            var callaron: [String] = []
            for r in RolMental.allCases where r != ctx.sh.rolIA {
                if let d = await respuestaDe(r, oye: pregunta, de: quien) {
                    out += SalaChat.formato(sala.publica(de: r.rawValue, para: quien, texto: d)) + "\n"
                } else {
                    callaron.append(r.rawValue)
                }
            }
            if !callaron.isEmpty { out += "  (en silencio: \(callaron.joined(separator: ", ")))\n" }
            return out

        case "archivos":
            let ms = sala.mensajes().filter { $0.adjunto != nil }
            guard !ms.isEmpty else { return "no hay adjuntos — chat -a <archivo> [mensaje]\n" }
            var out = ""
            for m in ms {
                let u = sala.adjuntos.appendingPathComponent("\(m.id)-\(m.adjunto!)")
                let t: Int = ((try? fm.attributesOfItem(atPath: u.path))?[.size] as? Int) ?? 0
                out += "#\(m.id) \(m.adjunto!) (\(humanSize(t))) de \(SalaChat.nombre(m.de))" + (m.sala.map { " en \($0)" } ?? "") + "\n"
            }
            return out + "bájalo con: chat baja <número>\n"

        case "baja", "descarga", "bajar":
            guard a.count >= 2 else { throw ShErr("uso: chat baja <n|nombre> [destino]") }
            let clave = a[1].trimmingCharacters(in: CharacterSet(charactersIn: "#"))
            guard let m = sala.mensajes().last(where: { $0.adjunto != nil && (String($0.id) == clave || $0.adjunto == clave) }) else {
                throw ShErr("chat: no hay adjunto '\(a[1])' (mira 'chat archivos')")
            }
            let origen = sala.adjuntos.appendingPathComponent("\(m.id)-\(m.adjunto!)")
            let porDefecto = ctx.sh.rolIA != nil ? "/buzon/\(m.adjunto!)" : m.adjunto!
            let destino = try ctx.env.resolve(a.count > 2 ? a[2] : porDefecto)
            try fm.createDirectory(at: destino.deletingLastPathComponent(), withIntermediateDirectories: true)
            if fm.fileExists(atPath: destino.path) { try fm.removeItem(at: destino) }
            try fm.copyItem(at: origen, to: destino)
            return "bajado \(m.adjunto!) → \(ctx.env.vpath(destino))\n"

        case "de":
            guard a.count >= 2 else { throw ShErr("uso: chat de <rol|humano>") }
            let r = ["tú", "yo"].contains(a[1]) ? "humano" : a[1]
            let ms = sala.mensajes().filter { $0.de == r || $0.para == r }.suffix(30)
            return ms.isEmpty ? "nada de \(a[1])\n" : listaMensajes(ms)

        case "busca":
            let t = a.dropFirst().joined(separator: " ").lowercased()
            guard !t.isEmpty else { throw ShErr("uso: chat busca <texto>") }
            let ms = sala.mensajes().filter { $0.texto.lowercased().contains(t) || ($0.adjunto ?? "").lowercased().contains(t) }
            return ms.isEmpty ? "sin resultados\n" : listaMensajes(ms.suffix(30))

        case "limpia":
            guard ctx.sh.rolIA == nil else { throw ShErr("chat limpia: solo el humano") }
            sala.limpia()
            return "mensajes borrados (las salas y los adjuntos siguen)\n"

        default:
            return try await escribe(ctx, a, sala: "general")
        }
    }

    /// Escribir en una sala (con adjunto y @mención opcionales).
    static func escribe(_ ctx: Ctx, _ argumentos: [String], sala nombreSala: String) async throws -> String {
        let sala = SalaChat.una
        let quien = ctx.sh.quienPide
        let fm = FileManager.default
        var a = argumentos
        var adjunto: URL? = nil
        if a.first == "-a" {
            guard a.count >= 2 else { throw ShErr("uso: chat -a <archivo> [mensaje]") }
            let u = try ctx.env.resolve(a[1])
            guard ctx.env.exists(u), !ctx.env.isDir(u) else { throw ShErr("chat: \(a[1]) no es un archivo") }
            let t: Int = ((try? fm.attributesOfItem(atPath: u.path))?[.size] as? Int) ?? 0
            guard t <= 5 << 20 else { throw ShErr("chat: el adjunto pasa de 5 MB") }
            adjunto = u
            a.removeFirst(2)
        }
        var para: RolMental? = nil
        var paraHumano = false
        if let men = a.first(where: { $0.hasPrefix("@") }) {
            let n = men.dropFirst().trimmingCharacters(in: .punctuationCharacters).lowercased()
            para = RolMental(rawValue: n)
            paraHumano = ["humano", "tú", "tu", "yo"].contains(n)
            if para == nil, !paraHumano, n != "todos" { throw ShErr("chat: no hay ninguna IA llamada '\(n)'") }
        }
        var texto = a.filter { !$0.hasPrefix("@") }.joined(separator: " ")
        if texto.isEmpty, let u = adjunto { texto = "te paso \(u.lastPathComponent)" }
        guard !texto.isEmpty else { throw ShErr("chat: ¿qué escribo?") }
        let destino: String? = para?.rawValue ?? (paraHumano ? "humano" : nil)
        let m = sala.publica(de: quien, para: destino, texto: texto, adjunto: adjunto?.lastPathComponent, sala: nombreSala)
        if let u = adjunto {
            try fm.createDirectory(at: sala.adjuntos, withIntermediateDirectories: true)
            try fm.copyItem(at: u, to: sala.adjuntos.appendingPathComponent("\(m.id)-\(u.lastPathComponent)"))
        }
        var out = SalaChat.formato(m) + "\n"
        let miembros = sala.miembros(nombreSala)

        if let r = para, r != ctx.sh.rolIA {
            // la mencionada contesta (una vez: así no hay cadenas sin fin)
            if let d = await respuestaDe(r, oye: texto, de: quien) {
                out += SalaChat.formato(sala.publica(de: r.rawValue, para: quien, texto: d, sala: nombreSala)) + "\n"
            } else {
                out += "  (\(r.rawValue) lo leyó pero no dijo nada)\n"
            }
        } else if quien == "humano" {
            // a todos: contestan hasta dos miembros que tengan algo que decir
            var contestaron = 0
            for r in RolMental.allCases.shuffled() where contestaron < 2 && miembros.contains(r.rawValue) {
                if let d = await respuestaDe(r, oye: texto, de: quien) {
                    out += SalaChat.formato(sala.publica(de: r.rawValue, para: quien, texto: d, sala: nombreSala)) + "\n"
                    contestaron += 1
                }
            }
        } else {
            // una IA a la sala: los demás miembros lo oyen (sin contestar en cadena)
            for r in RolMental.allCases where r != ctx.sh.rolIA && miembros.contains(r.rawValue) {
                if let mente = await NyxNucleo.uno.consejo.mente(r) {
                    await mente.recibir(mensaje: texto, de: quien, tipo: .dato)
                }
            }
        }
        return out
    }

    // ---------- conversaciones entre IAs ----------

    /// conversa critico empatia [memoria …] [turnos] [tema…]
    /// Desde la shell de una IA, ella misma participa: conversa critico 4
    static func ordenConversa(_ ctx: Ctx) async throws -> String {
        _ = await NyxNucleo.uno.preparar(memoria: nil)
        var roles: [RolMental] = []
        if let yo = ctx.sh.rolIA { roles.append(yo) }
        var turnos = 6
        var tema: [String] = []
        for x in ctx.args {
            let n = x.lowercased().trimmingCharacters(in: CharacterSet(charactersIn: "@"))
            if tema.isEmpty, let r = RolMental(rawValue: n) { if !roles.contains(r) { roles.append(r) }; continue }
            if tema.isEmpty, let t = Int(x) { turnos = max(2, min(20, t)); continue }
            tema.append(x)
        }
        guard roles.count >= 2 else {
            throw ShErr("uso: conversa <rol> <rol> [más roles] [turnos] [tema] — ej: conversa critico empatia 6 la verdad")
        }
        let nombreSala = roles.map(\.rawValue).sorted().joined(separator: "-")
        let sala = SalaChat.una
        sala.abre(nombreSala, creador: ctx.sh.quienPide, miembros: roles.map(\.rawValue) + ["humano"])
        var out = "── conversación en la sala \(nombreSala) ──\n"

        // primera frase: el tema (dicho por la primera) o lo que ella piense
        var ultimo: String
        let primera = roles[0]
        if !tema.isEmpty {
            ultimo = tema.joined(separator: " ")
        } else if let m = await NyxNucleo.uno.consejo.mente(primera), let d = await m.formularMensaje()?.contenido {
            ultimo = d
        } else {
            ultimo = "ye ko?"
        }
        out += SalaChat.formato(sala.publica(de: primera.rawValue, para: nil, texto: ultimo, sala: nombreSala)) + "\n"

        var silencios = 0
        var hablante = primera
        for i in 1 ..< turnos {
            let r = roles[i % roles.count]
            if let d = await respuestaDe(r, oye: ultimo, de: hablante.rawValue) {
                out += SalaChat.formato(sala.publica(de: r.rawValue, para: hablante.rawValue, texto: d, sala: nombreSala)) + "\n"
                ultimo = d
                hablante = r
                silencios = 0
            } else {
                silencios += 1
                out += "  (\(r.rawValue) se queda pensando)\n"
                if silencios >= 2 { out += "  (la conversación se apaga)\n"; break }
            }
        }
        return out + "── léela otra vez con: chat en \(nombreSala) ──\n"
    }
}
