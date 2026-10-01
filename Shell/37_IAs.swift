import Foundation

// ============================================================
// MARK: - Las 18 IAs, cada una con su shell y su espacio
// ============================================================
// Cada mente del consejo de Nyx tiene:
//   · su almacenamiento propio:  /ias/<rol>/   (desde tu shell)
//     y para ella eso es "/" — no puede salir de ahí
//   · su escritorio y carpetas:  /Escritorio /Documentos /buzon /scripts
//   · su propia shell completa (todos los comandos), con su historial,
//     sus variables, sus alias y sus funciones
//
// Control completo: la IA ejecuta lo que quiera en SU shell. Cuando
// está libre ('ia libres') decide sola qué hacer en cada turno según
// su rol; tú también puedes ordenarle comandos o encargarle tareas.
// Lo que pasa en su shell vuelve a su mente (éxito = ko, fallo = ne):
// aprende de las consecuencias de lo que hace.
//
// TÚ (desde tu shell):
//   ias                         las 18: archivos, tamaño, último comando
//   ia <rol>                    estado y bitácora de esa IA
//   ia <rol> <comando …>        ejecuta el comando en SU shell
//   ia entra <rol>              tu terminal pasa a SU shell ('salir' vuelve)
//   ia todas <comando …>        el comando en las 18 shells
//   ia tarea <rol> <comando …>  se lo encargas: lo hará en su próximo turno
//   ia turno [n]                n rondas: cada IA actúa una vez por ronda
//   ia libres [segundos]        actúan solas sin parar   · ia quietas  las para
//   ia bitacora <rol> [n]       lo último que hizo
//   ia ayuda
//
// ELLAS (dentro de su shell, además de todos los comandos normales):
//   yo · pienso <tema> · digo <msg> · oigo [n] · nota <texto> · diario
//   escritorio · envia <rol|humano> <archivo> · buzon · aprende es = resh
//   actua

/// Cerrojo asíncrono: una shell no puede correr dos órdenes a la vez
/// (la IA actuando sola y tú mandándole algo al mismo tiempo).
/// Hecho con NSLock y no con un actor: así compila igual en Swift 5.9,
/// 5.10 y 6 (en 5.x un actor no puede tocar su estado desde dentro de
/// withCheckedContinuation).
final class CerrojoIA: @unchecked Sendable {
    private let l = NSLock()
    private var ocupado = false
    private var cola: [CheckedContinuation<Void, Never>] = []

    func tomar() async {
        await withCheckedContinuation { (c: CheckedContinuation<Void, Never>) in
            l.lock()
            if !ocupado {
                ocupado = true
                l.unlock()
                c.resume()
            } else {
                cola.append(c)
                l.unlock()
            }
        }
    }

    func soltar() {
        l.lock()
        if cola.isEmpty {
            ocupado = false
            l.unlock()
        } else {
            let c = cola.removeFirst()
            l.unlock()
            c.resume()
        }
    }
}

/// El espacio de una IA: su carpeta, su shell y lo que ha hecho.
final class EspacioIA: @unchecked Sendable {
    let rol: RolMental
    let shell: Shell
    let cerrojo = CerrojoIA()
    // Bitácora, tareas y contadores se tocan desde el bucle autónomo y
    // desde tus órdenes a la vez: todo pasa por este candado (sin él, dos
    // escrituras simultáneas en un Array pueden cerrar la app).
    private let candado = NSLock()
    private var _bitacora: [String] = []
    private var _tareas: [String] = []
    private var _acciones = 0
    private var _fallos = 0

    var bitacora: [String] { candado.conCandado { _bitacora } }
    var pendientes: Int { candado.conCandado { _tareas.count } }
    var acciones: Int { candado.conCandado { _acciones } }
    var fallos: Int { candado.conCandado { _fallos } }

    /// Encarga una orden para su próximo turno; devuelve cuántas hay.
    func encargar(_ linea: String) -> Int {
        candado.conCandado { _tareas.append(linea); return _tareas.count }
    }

    func sacarTarea() -> String? {
        candado.conCandado { _tareas.isEmpty ? nil : _tareas.removeFirst() }
    }

    func registrar(ok: Bool, _ s: String) {
        candado.conCandado {
            _acciones += 1
            if !ok { _fallos += 1 }
            _bitacora.append(s)
            if _bitacora.count > 300 { _bitacora.removeFirst(_bitacora.count - 300) }
        }
    }

    static let carpetas = ["Escritorio", "Documentos", "buzon", "scripts"]

    init(rol: RolMental, raiz: URL) {
        self.rol = rol
        shell = Shell(raiz: raiz, usuario: rol.rawValue, rolIA: rol)
        let fm = FileManager.default
        for c in Self.carpetas {
            try? fm.createDirectory(at: raiz.appendingPathComponent(c), withIntermediateDirectories: true)
        }
        let leeme = raiz.appendingPathComponent("Escritorio/LEEME.txt")
        if !fm.fileExists(atPath: leeme.path) {
            let t = """
            Soy \(rol.rawValue). Esta es mi shell y este es mi espacio.
            Escritorio/  lo que estoy haciendo
            Documentos/  mi diario, mis notas y mi bitácora
            buzon/       lo que me mandan las otras IAs
            scripts/     mis guiones
            Comandos míos: yo, pienso, digo, oigo, nota, diario, escritorio,
            envia, buzon, aprende, actua (y todos los de la shell).

            """
            try? t.write(to: leeme, atomically: true, encoding: .utf8)
        }
        let diario = raiz.appendingPathComponent("Documentos/diario.txt")
        if !fm.fileExists(atPath: diario.path) { fm.createFile(atPath: diario.path, contents: Data()) }
        shell.env.vars["ROL"] = rol.rawValue
    }

    /// Puede borrar todo lo suyo (control completo), pero su espacio no
    /// desaparece: si 'rm -rf /' se llevó las carpetas, vuelven vacías.
    func reparar() {
        let fm = FileManager.default
        let raiz = shell.env.root
        for c in Self.carpetas {
            let u = raiz.appendingPathComponent(c)
            if !fm.fileExists(atPath: u.path) { try? fm.createDirectory(at: u, withIntermediateDirectories: true) }
        }
        let diario = raiz.appendingPathComponent("Documentos/diario.txt")
        if !fm.fileExists(atPath: diario.path) { fm.createFile(atPath: diario.path, contents: Data()) }
        if !fm.fileExists(atPath: shell.env.cwd.path) { shell.env.cwd = raiz }
    }

}

final class SistemaIAs: @unchecked Sendable {
    static let uno = SistemaIAs()

    private let lock = NSLock()
    private var espacios: [RolMental: EspacioIA] = [:]
    private var base: URL?
    private var libres: Task<Void, Never>?
    private(set) var pausaLibres: Double = 3

    var estanLibres: Bool { lock.conCandado { libres != nil } }

    /// Carpeta real donde viven las IAs: /ias dentro de la shell del humano.
    func preparar(base raizHumano: URL) {
        lock.conCandado {
            if base == nil { base = raizHumano.appendingPathComponent("ias", isDirectory: true) }
        }
    }

    func espacio(_ rol: RolMental) -> EspacioIA {
        lock.conCandado {
            if let e = espacios[rol] { return e }
            let b = base ?? ShellEnv.raizHumano.appendingPathComponent("ias", isDirectory: true)
            let e = EspacioIA(rol: rol, raiz: b.appendingPathComponent(rol.rawValue, isDirectory: true))
            espacios[rol] = e
            return e
        }
    }

    /// Deja listas a Nyx y a las IAs desde la shell del humano.
    func prepararDesde(_ sh: Shell) async {
        _ = await NyxNucleo.uno.preparar(memoria: try? sh.env.resolve(NyxNucleo.ruta))
        preparar(base: sh.env.root)
    }

    /// 'ia <rol> …', 'ia todas …' y 'ia tarea <rol> …' se leen CRUDAS: así
    /// los >, |, && y $VAR los interpreta la shell de la IA y no la tuya
    /// (antes 'ia critico echo hola > a.txt' escribía a.txt en TU espacio).
    func lineaCruda(_ linea: String, _ sh: Shell) async -> String? {
        let partes = linea.split(separator: " ", maxSplits: 2, omittingEmptySubsequences: true).map(String.init)
        guard partes.count == 3, partes[0] == "ia" else { return nil }
        let quien = partes[1].lowercased()
        if quien == "todas" {
            await prepararDesde(sh)
            var out = ""
            for r in RolMental.allCases {
                let res = await ejecutar(partes[2], en: espacio(r).shell, por: "humano")
                out += "── \(r.rawValue)\n" + res
                if !res.isEmpty, !res.hasSuffix("\n") { out += "\n" }
            }
            return out
        }
        if quien == "tarea" {
            let resto = partes[2].split(separator: " ", maxSplits: 1).map(String.init)
            guard resto.count == 2, let r = RolMental(rawValue: resto[0].lowercased()) else { return nil }
            await prepararDesde(sh)
            let e = espacio(r)
            let n = e.encargar(resto[1])
            return "\(r.rawValue) lo hará en su próximo turno (\(n) pendientes)\n"
        }
        guard let r = RolMental(rawValue: quien) else { return nil }
        await prepararDesde(sh)
        return await ejecutar(partes[2], en: espacio(r).shell, por: "humano")
    }

    /// Palabras de lo que persigue cada rol: de qué escribe cuando su
    /// mente no dijo nada concreto.
    static let temas: [RolMental: [String]] = [
        .sintaxis: ["frase", "orden", "estructura"], .semantica: ["significado", "claro", "sentido"],
        .logica: ["valido", "coherente", "premisa"], .codigo: ["preciso", "ejecutable", "correcto"],
        .creativo: ["nuevo", "inesperado", "original"], .critico: ["falla", "error", "objecion"],
        .memoria: ["recuerdo", "pasado", "patron"], .percepcion: ["forma", "color", "ritmo"],
        .sintesis: ["integra", "une", "resume"], .intuicion: ["corazonada", "destello", "ahora"],
        .analogia: ["puente", "metafora", "como"], .contexto: ["situacion", "marco", "aqui"],
        .esceptico: ["duda", "cuestiona", "verifica"], .narrativa: ["historia", "secuencia", "despues"],
        .etica: ["justo", "deber", "bien"], .curiosidad: ["novedad", "explora", "descubre"],
        .abstraccion: ["esencia", "general", "patron"], .empatia: ["escucha", "comprende", "acompana"],
    ]

    func espacio(de sh: Shell) -> EspacioIA? {
        guard let r = sh.rolIA else { return nil }
        return espacio(r)
    }

    /// Carpeta real del buzón de una IA (o del humano).
    func buzon(de destino: String) -> URL? {
        if destino == "humano" || destino == "tú" || destino == "tu" {
            return lock.conCandado { base?.deletingLastPathComponent().appendingPathComponent("buzon", isDirectory: true) }
        }
        guard let r = RolMental(rawValue: destino) else { return nil }
        return espacio(r).shell.env.root.appendingPathComponent("buzon", isDirectory: true)
    }

    /// Ejecuta una línea en la shell de una IA, de una en una. Lo que
    /// pase queda en su bitácora (memoria y disco) y su mente lo percibe.
    func ejecutar(_ linea: String, en sh: Shell, por autor: String) async -> String {
        guard let e = espacio(de: sh) else { return await sh.execute(linea) }
        await e.cerrojo.tomar()
        let out = await sh.execute(linea)
        e.reparar()
        e.cerrojo.soltar()
        let ok = (sh.env.vars["?"] ?? "0") == "0"
        let hora = SistemaIAs.hora()
        let resumen = out.split(separator: "\n").first.map { String($0.prefix(60)) } ?? ""
        e.registrar(ok: ok, "\(hora) [\(autor)] \(ok ? "✓" : "✗") \(linea)" + (resumen.isEmpty ? "" : "  → \(resumen)"))
        // su bitácora en SU disco (sin pasar por su shell: no es una orden suya)
        if let u = try? sh.env.resolve("/Documentos/bitacora.log") {
            let l = "\(hora)\t\(autor)\t\(ok ? "ok" : "error")\t\(linea)\n"
            SistemaIAs.agregar(l, a: u, maxLineas: 400)
        }
        // la mente siente la consecuencia: lo que funciona, ko; lo que falla, ne
        let orden = linea.split(separator: " ").first.map(String.init) ?? linea
        if let m = await NyxNucleo.uno.consejo.mente(e.rol) {
            await m.recibir(mensaje: "\(ok ? "ko" : "ne") \(orden)", de: "shell", tipo: .dato)
        }
        return out
    }

    // ---------- autonomía ----------

    /// Un turno de una IA: si tiene tareas, hace la primera; si no, piensa,
    /// escribe en su diario y hace lo que su rol le pide.
    @discardableResult
    func turno(_ rol: RolMental) async -> [String] {
        let e = espacio(rol)
        var hecho: [String] = []
        if let t = e.sacarTarea() {
            _ = await ejecutar(t, en: e.shell, por: "tarea")
            return ["tarea: \(t)"]
        }
        let consejo = NyxNucleo.uno.consejo
        guard let mente = await consejo.mente(rol) else { return [] }
        // De qué habla: una raíz de lo que su mente dijo, o (si calló) un
        // tema de su rol, dicho en Resh cuando lo sabe.
        let tema = SistemaIAs.temas[rol]?.randomElement() ?? rol.rawValue
        var palabra = LenguaResh.traducir(tema) ?? tema
        var frase: String? = nil
        let marco: Set<String> = ["qué", "que", "significa", "ton"]
        if let h = await mente.formularMensaje() {
            frase = SistemaIAs.limpio(h.contenido)
            let raices = h.contenido.split(separator: " ").map {
                $0.trimmingCharacters(in: .punctuationCharacters).lowercased()
            }.filter {
                $0.count > 2 && !LenguaResh.esParticula($0) && !marco.contains($0)
                    && $0.allSatisfy { $0.isLetter || $0 == "-" }
            }
            if let r = raices.randomElement() { palabra = r }
        }
        if let f = frase, !f.isEmpty {
            let l = "echo \"\(SistemaIAs.hora()) \(f)\" >> /Documentos/diario.txt"
            _ = await ejecutar(l, en: e.shell, por: "sí misma")
            hecho.append("diario: \(f)")
            if Double.random(in: 0 ... 1) < 0.3 {
                _ = await ejecutar("digo \(f)", en: e.shell, por: "sí misma")
                hecho.append("dijo: \(f)")
            }
        }
        for l in accionesDeRol(rol, palabra: palabra, frase: frase ?? palabra) {
            _ = await ejecutar(l, en: e.shell, por: "sí misma")
            hecho.append(l)
        }
        return hecho
    }

    /// Lo que cada rol hace en su shell cuando actúa libre. Son órdenes
    /// de verdad: pasan por su shell igual que las tuyas.
    func accionesDeRol(_ rol: RolMental, palabra w: String, frase f: String) -> [String] {
        let otro = RolMental.allCases.filter { $0 != rol }.randomElement()?.rawValue ?? "memoria"
        switch rol {
        case .sintaxis:    return ["wc -w /Documentos/diario.txt"]
        case .semantica:   return ["grep -c \(w) /Documentos/diario.txt"]
        case .logica:      return ["test -s /Documentos/diario.txt && echo coherente || echo vacio"]
        case .codigo:
            return ["test -f /scripts/cuenta.sh || echo 'ls -R / | wc -l' > /scripts/cuenta.sh",
                    "sh /scripts/cuenta.sh"]
        case .creativo:    return ["echo \(w)-\(SistemaIAs.raizAlAzar()) >> /Escritorio/ideas.txt"]
        case .critico:     return ["tail -n 3 /Documentos/diario.txt"]
        case .memoria:
            // guarda el recuerdo en su disco y en la API compartida (api-local)
            return ["echo \(w) >> /Documentos/palabras.txt", "sort -u /Documentos/palabras.txt | wc -l",
                    "http -b POST localhost/memorias rol=memoria palabra=\(w)"]
        case .percepcion:  return ["ls -l /Escritorio"]
        case .sintesis:    return ["cat /Documentos/diario.txt | wc -l", "curl -s localhost/memorias | grep -c palabra"]
        case .intuicion:   return ["head -n 1 /Documentos/diario.txt"]
        case .analogia:
            return ["echo \(w) kep \(SistemaIAs.raizAlAzar()) >> /Escritorio/ideas.txt",
                    "envia \(otro) /Escritorio/ideas.txt"]
        case .contexto:    return ["pwd", "date"]
        case .esceptico:   return ["ls /buzon"]
        case .narrativa:   return ["echo \(f) >> /Escritorio/historia.txt"]
        case .etica:
            // cuida el espacio: el diario no crece sin límite
            return ["du -s /",
                    "test $(wc -l < /Documentos/diario.txt) -gt 300 && tail -n 150 /Documentos/diario.txt > /Documentos/diario.tmp && mv /Documentos/diario.tmp /Documentos/diario.txt || echo espacio en orden"]
        case .curiosidad:  return ["ls -a /", "ls /buzon", "http -b GET 'localhost/memorias?_sort=id&_order=desc&_limit=3'"]
        case .abstraccion: return ["sort /Documentos/palabras.txt | uniq -c | sort -rn | head -n 3"]
        case .empatia:     return ["ls /buzon", "digo se ko \(w)"]
        }
    }

    /// Rondas en las que cada una de las 18 actúa una vez.
    func rondas(_ n: Int) async -> String {
        var out = ""
        for i in 1 ... max(1, n) {
            for rol in RolMental.allCases {
                let h = await turno(rol)
                if n == 1 { out += "\(rol.rawValue.padding(toLength: 11, withPad: " ", startingAt: 0)) \(h.first ?? "(nada)")\n" }
            }
            if n > 1 { out += "ronda \(i): las 18 actuaron\n" }
        }
        return out
    }

    func soltar(cada segundos: Double) {
        lock.conCandado {
            pausaLibres = max(0.5, segundos)
            guard libres == nil else { return }
            libres = Task.detached(priority: .background) { [weak self] in
                var i = 0
                while !Task.isCancelled {
                    guard let self else { return }
                    let roles = RolMental.allCases
                    await self.turno(roles[i % roles.count])
                    i += 1
                    let pausa = self.lock.conCandado { self.pausaLibres } / Double(roles.count)
                    try? await Task.sleep(nanoseconds: UInt64(pausa * 1_000_000_000))
                }
            }
        }
    }

    func aquietar() {
        lock.conCandado {
            libres?.cancel()
            libres = nil
        }
    }

    // ---------- utilidades ----------

    static func hora() -> String {
        let f = DateFormatter()
        f.dateFormat = "HH:mm:ss"
        return f.string(from: Date())
    }

    /// Texto seguro dentro de comillas dobles de la shell.
    static func limpio(_ s: String) -> String {
        String(s.filter { !"\"$`\\;|&<>".contains($0) }.prefix(120))
    }

    static func raizAlAzar() -> String {
        LenguaResh.pares.randomElement()?.1 ?? "kash"
    }

    /// Añade al final de un archivo y lo recorta a sus últimas líneas.
    static func agregar(_ texto: String, a u: URL, maxLineas: Int) {
        let previo = (try? String(contentsOf: u, encoding: .utf8)) ?? ""
        var lineas = (previo + texto).components(separatedBy: "\n")
        if lineas.last == "" { lineas.removeLast() }
        if lineas.count > maxLineas { lineas.removeFirst(lineas.count - maxLineas) }
        try? (lineas.joined(separator: "\n") + "\n").write(to: u, atomically: true, encoding: .utf8)
    }

    static func tamaño(_ u: URL) -> (archivos: Int, bytes: Int) {
        var n = 0, b = 0
        if let en = FileManager.default.enumerator(at: u, includingPropertiesForKeys: [.fileSizeKey, .isDirectoryKey]) {
            for case let f as URL in en {
                let v = try? f.resourceValues(forKeys: [.fileSizeKey, .isDirectoryKey])
                if v?.isDirectory == true { continue }
                n += 1
                b += v?.fileSize ?? 0
            }
        }
        return (n, b)
    }
}

extension Shell {

    // ============================================================
    // MARK: comandos del humano: ias / ia
    // ============================================================

    static func ias() -> [String: Spec] {
        var c: [String: Spec] = [:]

        func prepararNyx(_ ctx: Ctx) async {
            _ = await NyxNucleo.uno.preparar(memoria: try? ctx.env.resolve(NyxNucleo.ruta))
            SistemaIAs.uno.preparar(base: ctx.env.root)
        }

        func rol(_ s: String) throws -> RolMental {
            guard let r = RolMental(rawValue: s.lowercased()) else {
                throw ShErr("no hay ninguna IA llamada '\(s)'. Son: " + RolMental.allCases.map(\.rawValue).joined(separator: " "))
            }
            return r
        }

        c["ias"] = Spec(help: "ias — las 18 IAs: archivos, tamaño y último comando") { ctx in
            guard ctx.sh.rolIA == nil else { throw ShErr("ias: solo desde la shell del humano") }
            await prepararNyx(ctx)
            let sis = SistemaIAs.uno
            var out = "IA           archivos  tamaño   acciones  último\n"
            for r in RolMental.allCases {
                let e = sis.espacio(r)
                let t = SistemaIAs.tamaño(e.shell.env.root)
                let ultimo = e.bitacora.last.map { String($0.prefix(48)) } ?? "—"
                out += r.rawValue.padding(toLength: 12, withPad: " ", startingAt: 0)
                out += String(t.archivos).leftPad(9) + "  " + humanSize(t.bytes).padding(toLength: 8, withPad: " ", startingAt: 0)
                out += String(e.acciones).leftPad(8) + "  " + ultimo + "\n"
            }
            out += sis.estanLibres ? "actúan solas (ia quietas para pararlas)\n" : "quietas (ia libres para que actúen solas · ia turno para un turno)\n"
            return out
        }

        c["ia"] = Spec(help: "ia <rol> [comando] · ia entra|todas|tarea|turno|libres|quietas|bitacora|ayuda — las shells de las IAs") { ctx in
            guard ctx.sh.rolIA == nil else {
                throw ShErr("ia: solo el humano manda en las shells ajenas. Tú tienes 'envia' y 'digo'")
            }
            await prepararNyx(ctx)
            let sis = SistemaIAs.uno
            let a = ctx.args
            guard let primero = a.first else { return await ctx.sh.execute("ias") }

            switch primero.lowercased() {
            case "ayuda", "help", "-h":
                return """
                ia — cada una de las 18 IAs tiene su shell y su espacio (/ias/<rol>)
                  ias                          las 18 de un vistazo
                  ia <rol>                     estado y bitácora
                  ia <rol> <comando …>         ejecútalo en su shell
                  ia entra <rol>               entra en su shell ('salir' para volver)
                  ia todas <comando …>         en las 18 shells
                  ia tarea <rol> <comando …>   encárgaselo para su próximo turno
                  ia turno [n]                 n rondas: cada una actúa una vez
                  ia libres [segundos]         actúan solas · ia quietas  las para
                  ia bitacora <rol> [n]        lo último que hizo
                dentro de su shell ellas tienen: yo pienso digo oigo nota diario
                  escritorio envia buzon aprende actua (y todos los comandos)
                roles: \(RolMental.allCases.map(\.rawValue).joined(separator: " "))

                """

            case "entra":
                guard a.count >= 2 else { throw ShErr("uso: ia entra <rol>") }
                let e = sis.espacio(try rol(a[1]))
                ctx.sh.dentroDe = e.shell
                return "estás en la shell de \(e.rol.rawValue) — su / es tu /ias/\(e.rol.rawValue) · 'salir' para volver\n"

            case "todas":
                let linea = a.dropFirst().joined(separator: " ")
                guard !linea.isEmpty else { throw ShErr("uso: ia todas <comando>") }
                var out = ""
                for r in RolMental.allCases {
                    let res = await sis.ejecutar(linea, en: sis.espacio(r).shell, por: "humano")
                    out += "── \(r.rawValue)\n" + res
                    if !res.isEmpty, !res.hasSuffix("\n") { out += "\n" }
                }
                return out

            case "tarea":
                guard a.count >= 3 else { throw ShErr("uso: ia tarea <rol> <comando>") }
                let e = sis.espacio(try rol(a[1]))
                let linea = a.dropFirst(2).joined(separator: " ")
                let n = e.encargar(linea)
                return "\(e.rol.rawValue) lo hará en su próximo turno (\(n) pendientes)\n"

            case "turno", "turnos":
                let n = min(50, max(1, Int(a.dropFirst().first ?? "1") ?? 1))
                return await sis.rondas(n)

            case "libres", "libre":
                let s = Double(a.dropFirst().first ?? "") ?? 3
                sis.soltar(cada: s)
                return "las 18 actúan solas en sus shells (una ronda cada \(String(format: "%.1f", max(0.5, s))) s) · 'ia quietas' para parar\n"

            case "quietas", "quieta", "para":
                sis.aquietar()
                return "las 18 quietas: solo actúan si se lo pides\n"

            case "bitacora", "bitácora", "log":
                guard a.count >= 2 else { throw ShErr("uso: ia bitacora <rol> [n]") }
                let e = sis.espacio(try rol(a[1]))
                let n = Int(a.dropFirst(2).first ?? "15") ?? 15
                let b = e.bitacora.suffix(n)
                return b.isEmpty ? "\(e.rol.rawValue) todavía no ha hecho nada\n" : b.joined(separator: "\n") + "\n"

            default:
                let r = try rol(primero)
                let e = sis.espacio(r)
                if a.count == 1 {
                    let t = SistemaIAs.tamaño(e.shell.env.root)
                    var out = "\(r.rawValue) — /ias/\(r.rawValue) · \(t.archivos) archivos · \(humanSize(t.bytes))\n"
                    out += "acciones \(e.acciones) · fallos \(e.fallos) · tareas pendientes \(e.pendientes)\n"
                    if let m = await NyxNucleo.uno.consejo.mente(r) {
                        let p = await m.precisionYTamaño()
                        out += "mente: precisión \(Int((p.precision * 100).rounded()))% · \(p.semiones) semiones · \(p.palabras) palabras\n"
                    }
                    let b = e.bitacora.suffix(8)
                    out += b.isEmpty ? "(aún no ha hecho nada — prueba: ia turno)\n" : b.joined(separator: "\n") + "\n"
                    return out
                }
                let linea = a.dropFirst().joined(separator: " ")
                return await sis.ejecutar(linea, en: e.shell, por: "humano")
            }
        }

        return c
    }

    // ============================================================
    // MARK: comandos de las IAs (dentro de su propia shell)
    // ============================================================

    static func comandosIA() -> [String: Spec] {
        var c: [String: Spec] = [:]

        /// La IA dueña de esta shell (estos comandos son suyos).
        func yo(_ ctx: Ctx) throws -> RolMental {
            guard let r = ctx.sh.rolIA else {
                throw ShErr("\(ctx.name): es un comando de las IAs — úsalo dentro de su shell ('ia entra <rol>') o con 'ia <rol> \(ctx.name) …'")
            }
            return r
        }
        func mente(_ r: RolMental) async throws -> ResonantMind {
            guard let m = await NyxNucleo.uno.consejo.mente(r) else { throw ShErr("su mente no está lista") }
            return m
        }
        func añade(_ ctx: Ctx, _ ruta: String, _ texto: String) throws {
            let u = try ctx.env.resolve(ruta)
            try? FileManager.default.createDirectory(at: u.deletingLastPathComponent(), withIntermediateDirectories: true)
            SistemaIAs.agregar(texto.hasSuffix("\n") ? texto : texto + "\n", a: u, maxLineas: 1000)
        }

        c["yo"] = Spec(help: "yo — (IA) quién soy: rol, mente y espacio") { ctx in
            let r = try yo(ctx)
            let m = try await mente(r)
            let p = await m.precisionYTamaño()
            let t = SistemaIAs.tamaño(ctx.env.root)
            return "soy \(r.rawValue) · precisión \(Int((p.precision * 100).rounded()))% · \(p.semiones) semiones · \(p.palabras) palabras\n" +
                   "mi espacio: \(t.archivos) archivos, \(humanSize(t.bytes))\n"
        }

        c["pienso"] = Spec(help: "pienso <tema> — (IA) mi mente delibera y deja el resultado en $PIENSO") { ctx in
            let r = try yo(ctx)
            let tema = ctx.args.isEmpty ? ctx.stdin : ctx.args.joined(separator: " ")
            guard !tema.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { throw ShErr("pienso: ¿sobre qué?") }
            let v = try await mente(r).veredicto(sobre: tema)
            ctx.env.vars["PIENSO"] = v.atractor
            let glosa = LenguaResh.glosaDe(v.atractor).map { " (\($0))" } ?? ""
            return "\(v.atractor)\(glosa)  C=\(String(format: "%.2f", v.coherencia))\n"
        }

        c["digo"] = Spec(help: "digo <mensaje> — (IA) lo digo al éter: lo oyen las otras 17") { ctx in
            let r = try yo(ctx)
            let msg = ctx.args.isEmpty ? ctx.stdin.trimmingCharacters(in: .whitespacesAndNewlines) : ctx.args.joined(separator: " ")
            guard !msg.isEmpty else { throw ShErr("digo: ¿qué digo?") }
            await NyxNucleo.uno.consejo.difundir(de: r, mensaje: msg)
            return "\(r.rawValue): \(msg)\n"
        }

        c["oigo"] = Spec(help: "oigo [n] — (IA) lo último que se dijo en el éter") { ctx in
            _ = try yo(ctx)
            let n = Int(ctx.args.first ?? "8") ?? 8
            let chat = await NyxNucleo.uno.consejo.chatReciente(n)
            guard !chat.isEmpty else { return "(silencio)\n" }
            return chat.map { "\($0.autor): \($0.texto)" }.joined(separator: "\n") + "\n"
        }

        c["nota"] = Spec(help: "nota <texto> — (IA) apunto algo en /Documentos/notas.txt") { ctx in
            _ = try yo(ctx)
            let t = ctx.args.isEmpty ? ctx.stdin : ctx.args.joined(separator: " ")
            guard !t.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { throw ShErr("nota: ¿qué apunto?") }
            try añade(ctx, "/Documentos/notas.txt", "\(SistemaIAs.hora()) \(t)")
            return ""
        }

        c["diario"] = Spec(help: "diario [n] — (IA) mi diario: lo que he pensado") { ctx in
            _ = try yo(ctx)
            let n = Int(ctx.args.first ?? "15") ?? 15
            let u = try ctx.env.resolve("/Documentos/diario.txt")
            let t = (try? String(contentsOf: u, encoding: .utf8)) ?? ""
            let l = t.split(separator: "\n").suffix(n)
            return l.isEmpty ? "(diario vacío)\n" : l.joined(separator: "\n") + "\n"
        }

        c["escritorio"] = Spec(help: "escritorio — (IA) lo que tengo en mi escritorio") { ctx in
            _ = try yo(ctx)
            return await ctx.sh.execute("ls -l /Escritorio")
        }

        c["envia"] = Spec(help: "envia <rol|humano> <archivo> — (IA) mando una copia al buzón de otra IA o al tuyo") { ctx in
            let r = try yo(ctx)
            guard ctx.args.count >= 2 else { throw ShErr("uso: envia <rol|humano> <archivo>") }
            let destino = ctx.args[0].lowercased()
            guard destino != r.rawValue else { throw ShErr("envia: no me lo mando a mí misma") }
            guard let dir = SistemaIAs.uno.buzon(de: destino) else {
                throw ShErr("envia: no conozco a '\(destino)'")
            }
            let p = ctx.args.dropFirst().joined(separator: " ")
            let u = try ctx.env.resolve(p)
            guard ctx.env.exists(u), !ctx.env.isDir(u) else { throw ShErr("envia: \(p): no es un archivo") }
            let fm = FileManager.default
            try fm.createDirectory(at: dir, withIntermediateDirectories: true)
            let dest = dir.appendingPathComponent("\(r.rawValue)-\(u.lastPathComponent)")
            if fm.fileExists(atPath: dest.path) { try fm.removeItem(at: dest) }
            try fm.copyItem(at: u, to: dest)
            return "mandé \(u.lastPathComponent) a \(destino)\n"
        }

        c["buzon"] = Spec(help: "buzon — (IA) lo que me han mandado") { ctx in
            _ = try yo(ctx)
            let u = try ctx.env.resolve("/buzon")
            let items = ((try? FileManager.default.contentsOfDirectory(atPath: u.path)) ?? []).sorted()
            return items.isEmpty ? "(buzón vacío)\n" : items.joined(separator: "\n") + "\n"
        }

        c["aprende"] = Spec(help: "aprende <español> = <resh> — (IA) me enseño una palabra") { ctx in
            let r = try yo(ctx)
            let partes = ctx.args.joined(separator: " ").components(separatedBy: "=")
            guard partes.count == 2 else { throw ShErr("uso: aprende árbol = kash") }
            let es = partes[0].trimmingCharacters(in: .whitespaces)
            let resh = partes[1].split(separator: ",").map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
            guard !es.isEmpty, !resh.isEmpty else { throw ShErr("uso: aprende árbol = kash") }
            try await mente(r).enseñar(español: es, significa: resh)
            try añade(ctx, "/Documentos/palabras.txt", resh.joined(separator: "\n"))
            return "aprendí \(es) = \(resh.joined(separator: ", "))\n"
        }

        c["actua"] = Spec(help: "actua — (IA) hago un turno por mi cuenta") { ctx in
            let r = try yo(ctx)
            // corre fuera de este comando: el turno usa la misma shell
            Task.detached { await SistemaIAs.uno.turno(r) }
            return "\(r.rawValue) actúa…\n"
        }

        return c
    }
}
