import Foundation
#if (canImport(CryptoKit) && APPLE_COMPLETO)
import CryptoKit
#endif
#if (canImport(UIKit) && APPLE_COMPLETO)
import UIKit
#endif

// ============================================================
// MARK: - Simulacro: intérprete de la sintaxis con diagonales
// ============================================================
// usrroot/Direct/file/notas.txt
// Createfile=hola.py/Put/print("hola")/Launch/hola.py
//
// Cada verbo hace algo de verdad dentro del sandbox. Los que
// dependen de hardware o de root del sistema (USB, wifi, box64,
// QEMU, reboot del sistema) responden diciendo qué falta, en vez
// de fingir que funcionaron.

struct SimState {
    var root = false
    var capa = 1
    var target = ""        // último archivo o programa nombrado
    var valor = ""         // último valor con =
    var linea: Int? = nil  // Line(n)
}

extension Shell {

    static let simDirs = ["/var", "/var/sim", "/var/canal", "/etc/api"]

    /// Tope de anidamiento: Loop/, For/, emit y las macros pueden llamarse
    /// entre sí. Sin este tope, 'Detec/x/emit/x' colgaba la app.
    static let simNivelMax = 12

    /// Crea las carpetas de trabajo. Antes solo se llamaba desde 'sim',
    /// así que escribir 'Log/hola' sin 'sim' delante fallaba en silencio.
    static func simPrepara(_ ctx: Ctx) {
        let fm = FileManager.default
        for d in Shell.simDirs {
            if let u = try? ctx.env.resolve(d) {
                try? fm.createDirectory(at: u, withIntermediateDirectories: true)
            }
        }
    }

    /// Verbos que existen pero que iPadOS no permite ejecutar de verdad.
    static let simImposible: [String: String] = [
        "connectwifi": "cambiar de red wifi requiere permisos del sistema que ninguna app tiene",
        "connectserver": "puedes abrir conexiones con nc/curl, pero no montar un servidor de escucha",
        "connectclaud": "usa 'api <url>' o http.get dentro de js",
        "listen": "iOS no deja abrir puertos de escucha en segundo plano desde una app",
        "usb": "el acceso a dispositivos USB está cerrado fuera de la app Archivos",
        "usbport": "no hay API pública de puertos USB",
        "rebootsys": "una app no puede reiniciar iPadOS",
        "force": "no hay root en un iPad sin jailbreak",
        "translate": "box64 y wine traducen código máquina x86; iOS no ejecuta código sin firmar",
        "clone": "no se pueden duplicar hilos de proceso desde el sandbox",
        "extansionsys": "una app no puede convertirse en parte del sistema"
    ]

    /// Todos los verbos de Simulacro que se reconocen como cabeza de línea
    /// sin escribir 'sim' delante (usa Shell.execute). Si un verbo no está
    /// aquí, sigue funcionando escrito como 'sim Verbo/...'.
    static let simVerbos: Set<String> = [
        // básicos, archivos y capas
        "usr", "usrroot", "capa1", "capa2", "capa3", "capa4", "api",
        "createfile", "create", "file", "edit", "put", "dump", "line",
        "replace", "direct", "path", "remove", "launch", "install", "get",
        "import", "wed", "web", "sys", "tex", "s", "wipe", "reboot",
        "kill", "freeze", "thaw", "fuse", "container", "canal", "save",
        // eventos, mensajes y variables
        "detec", "detect", "emit", "wait", "message", "respondmessage",
        "var", "show", "random", "loop", "copy", "rename", "search",
        "list", "size", "log", "texcommando", "texcomando", "ccomand",
        "ccommand", "camara", "camera", "draw", "dibujar", "for",
        "color", "update",
        // lógicos y de control (encadenan tras otro verbo, pero también
        // se reconocen si abren la línea)
        "and", "is", "cual", "%", "porcentaje", "this", "in", "mod",
        "extension", "com", "interaction", "beta",
        // datos, red, portapapeles y utilidades — envuelven un comando
        // real de la terminal, pero además dejan el resultado en $VAR
        "hash", "b64", "unb64", "upper", "lower", "trim", "now", "uuid",
        "ip", "battery", "weather", "clip", "clipget", "post", "sort",
        "count", "matches", "diff", "calc", "exists", "prepend", "inc",
        "dec", "stop", "push", "pop", "vibrate",
        // las IAs propias (35_Huella.swift y 36_Nyx.swift)
        "huella", "nyx",
        // no funcionan de verdad en iPad, pero deben reconocerse para
        // explicar por qué en vez de dar 'comando no encontrado'
        "connectwifi", "connectserver", "connectclaud", "listen", "usb",
        "usbport", "rebootsys", "force", "translate", "clone", "extansionsys"
    ]

    static func simulacro() -> [String: Spec] {
        var c: [String: Spec] = [:]
        let fm = FileManager.default

        func prepara(_ ctx: Ctx) { Shell.simPrepara(ctx) }

        c["sim"] = Spec(help: "sim <cadena con diagonales> — ejecuta una línea de Simulacro") { ctx in
            prepara(ctx)
            return await Shell.runSim(ctx.args.joined(separator: " "), ctx)
        }

        c["simhelp"] = Spec(help: "simhelp — todos los verbos de Simulacro y qué hace cada uno") { _ in
            var out = "SIMULACRO — sintaxis: verbo/valor/verbo/valor\n\n"
            out += "FUNCIONAN DE VERDAD\n"
            let reales: [(String, String)] = [
                ("usr/ usrroot/", "marca el modo; en un iPad sin jailbreak no cambia permisos"),
                ("Capa1..4/ API=n", "fija la capa de trabajo"),
                ("Createfile=n/", "crea un archivo"),
                ("File/ n", "muestra un archivo"),
                ("Edit/ n", "abre el editor nano"),
                ("Put/ texto", "añade texto al archivo activo"),
                ("Dump/ texto", "vuelca texto sobrescribiendo el archivo"),
                ("Line(n)/ texto", "cambia una línea concreta (empieza en 1)"),
                ("REPLACE/ a>b", "reemplaza texto en el archivo activo"),
                ("Direct/ ruta", "va directo a un sitio o archivo"),
                ("Path/ n=ruta", "crea un atajo con nombre"),
                ("Remove/ n", "borra"),
                ("Launch/ n", "ejecuta un archivo o paquete"),
                ("Install/ n", "instala un paquete"),
                ("Get/file url", "descarga un archivo"),
                ("Import/ n", "trae un archivo desde la app Archivos"),
                ("Wed/ url", "abre una dirección"),
                ("Sys/", "estado del sistema"),
                ("Tex/ texto", "imprime texto"),
                ("S=n/", "espera n segundos"),
                ("Wipe/", "vacía la memoria del motor"),
                ("Reboot/", "reinicia el motor de ejecución"),
                ("Kill/ n", "quita un proceso de la lista"),
                ("Freeze/ n  Thaw/ n", "congela y despierta un proceso"),
                ("Fuse/ n", "empaqueta un archivo como .sys"),
                ("Container/ n", "crea un contenedor y trabaja dentro"),
                ("canal=n/ canal/", "crea y une canales de datos"),
                ("SAVE/=n", "guarda la línea entera como macro"),
                ("Detec/ evento", "registra una reacción a un evento"),
                ("emit/ evento", "dispara un evento"),
                ("For/lista/acciones", "repite por cada línea; el elemento va en $IT"),
                ("COLOR/ n", "guarda el color de la terminal"),
                ("Update=n/", "actualiza un paquete")
            ]
            let logicos: [(String, String)] = [
                ("And/", "encadena: la línea sigue"),
                ("CUAL/cond/", "dispara solo si se cumple la condición"),
                ("is/a/b  is=b", "si no coinciden, corta la línea (is=b: compara el activo)"),
                ("%/30/", "se cumple el 30% de las veces"),
                ("This/", "el objetivo activo, en $THIS"),
                ("in/=archivo", "fija sobre qué se trabaja"),
                ("Mod/dest/archivo", "inyecta un mod en el destino"),
                ("Extension/n", "registra una extensión entre capas"),
                ("com/a/b/c", "dibuja ramas en texto"),
                ("interaction/", "marca la salida como interactiva"),
                ("BetA/", "bandera de estado en $BETA"),
                ("wait=N/", "espera N segundos"),
                ("Message=texto/", "manda un mensaje"),
                ("respondmessage=t{c}", "responde a ese mensaje"),
                ("Camara/", "abre la cámara"),
                ("Draw/archivo.draw", "dibuja en el lienzo"),
                ("Var=n:valor", "guarda una variable"),
                ("Show/n", "enseña lo que vale"),
                ("Random=1:100", "número al azar, queda en $RANDOM"),
                ("Loop=3/acciones", "repite el resto de la línea, $LOOP"),
                ("Copy/a/b", "copia"),
                ("Rename/a/b", "renombra"),
                ("Search/texto/arch", "busca dentro de un archivo"),
                ("List/ruta", "lista el directorio"),
                ("Size/archivo", "cuánto ocupa"),
                ("Log/texto", "anota con fecha en /var/sim/log"),
                ("texcommando=cmd", "escribe un comando en la terminal"),
                ("ccomand=n{código}", "crea un comando tuyo, en varios idiomas")
            ]
            let utiles: [(String, String)] = [
                ("Hash/archivo", "sha256 del archivo, queda en $HASH"),
                ("B64/texto  Unb64/texto", "codifica o descodifica base64, en $B64/$UNB64"),
                ("Upper/texto  Lower/texto", "mayúsculas o minúsculas, en $UPPER/$LOWER"),
                ("Trim/texto", "quita espacios de los extremos, en $TRIM"),
                ("Now/", "fecha y hora en $NOW, epoch en $EPOCH"),
                ("Uuid/", "identificador único, en $UUID"),
                ("Ip/", "tu IP pública, en $IP"),
                ("Battery/", "estado de la batería"),
                ("Weather/ciudad", "el tiempo en esa ciudad"),
                ("Clip/ texto", "copia texto (o un archivo) al portapapeles"),
                ("Clipget/", "pega el portapapeles, en $CLIP"),
                ("Post/url/datos", "petición POST, respuesta en $HTTP"),
                ("Sort/archivo", "ordena las líneas del archivo, en el propio archivo"),
                ("Count/archivo", "líneas, palabras y bytes en $LINES/$WORDS/$BYTES"),
                ("Matches/texto/archivo", "cuántas veces aparece, en $MATCHES"),
                ("Diff/a/b", "compara dos archivos línea a línea"),
                ("Calc/expresión", "calculadora, resultado en $CALC"),
                ("Exists/archivo", "1 o 0 en $EXISTS, sin cortar la línea"),
                ("Prepend/ texto", "añade texto al principio del archivo activo"),
                ("Inc/n  Dec/n", "suma o resta 1 (o Inc=n:cantidad) a una variable"),
                ("Stop/", "corta la línea ahí mismo"),
                ("Push/ ruta  Pop/", "guarda el directorio actual y, con Pop/, vuelve a él"),
                ("Vibrate/", "hace vibrar el dispositivo")
            ]
            // padding(toLength:) recorta si el texto es más largo: se comían
            // nombres como "Upper/texto  Lower/texto". Rellenamos a mano.
            func col(_ s: String, _ n: Int = 24) -> String {
                s.count >= n ? s + " " : s + String(repeating: " ", count: n - s.count)
            }
            for (v, d) in reales + logicos + utiles {
                out += "  " + col(v) + " " + d + "\n"
            }
            out += "\nNO SE PUEDEN EJECUTAR EN UN IPAD SIN JAILBREAK\n"
            for (v, r) in Shell.simImposible.sorted(by: { $0.key < $1.key }) {
                out += "  " + col(v, 16) + " " + r + "\n"
            }
            out += "\nTodos estos verbos también se reconocen sin escribir 'sim' delante,\n"
            out += "con tal de que la línea lleve una diagonal: escribe 'Tex/hola' igual que 'sim Tex/hola'.\n"
            out += "\nArchivos .sim: una línea de Simulacro por renglón, se lanzan con 'run x.sim'.\n"
            out += "Las $VARIABLES se sustituyen en cualquier verbo. Usa \\/ para una diagonal literal.\n"
            return out
        }

        c["emit"] = Spec(help: "emit <evento> — dispara un evento registrado con Detec/") { ctx in
            prepara(ctx)
            guard let e = ctx.args.first else { throw ShErr("emit: falta el evento") }
            return await Shell.dispararEvento(e, ctx)
        }

        c["events"] = Spec(help: "events — reacciones registradas con Detec/") { ctx in
            prepara(ctx)
            let t = (try? ctx.input(["/var/sim/events"])) ?? ""
            return t.isEmpty ? "no hay eventos registrados\n" : t
        }

        c["procs"] = Spec(help: "procs — procesos de Simulacro y su estado") { ctx in
            prepara(ctx)
            let t = (try? ctx.input(["/var/sim/procs"])) ?? ""
            return t.isEmpty ? "no hay procesos\n" : t
        }

        c["canales"] = Spec(help: "canales — canales de datos creados con canal=") { ctx in
            prepara(ctx)
            guard let u = try? ctx.env.resolve("/var/canal"),
                  let items = try? fm.contentsOfDirectory(atPath: u.path), !items.isEmpty else {
                return "no hay canales\n"
            }
            return items.sorted().joined(separator: "\n") + "\n"
        }

        // run ampliado: .sim y .sys
        c["run"] = Spec(help: "run <archivo> — ejecuta cualquier tipo admitido, incluido .sim") { ctx in
            guard let p = ctx.args.first else { throw ShErr("run: falta el archivo") }
            let u = try ctx.env.resolve(p)
            guard ctx.env.exists(u), !ctx.env.isDir(u) else {
                let items = ((try? fm.contentsOfDirectory(atPath: ctx.env.cwd.path)) ?? []).sorted()
                throw ShErr("run: \(p): no existe en \(ctx.env.vpath(ctx.env.cwd))\n" +
                            "aquí hay: " + (items.isEmpty ? "(nada — prueba 'demo')" : items.joined(separator: "  ")))
            }
            let ext = u.pathExtension.lowercased()
            if ext == "sim" || ext == "sys" {
                prepara(ctx)
                var out = ""
                for l in try ctx.lines([p]) {
                    let t = l.trimmingCharacters(in: .whitespaces)
                    if t.isEmpty || t.hasPrefix("#") { continue }
                    out += await Shell.runSim(t, ctx)
                }
                return out
            }
            let args = Array(ctx.args.dropFirst()).joined(separator: " ")
            let extra = args.isEmpty ? "" : " " + args
            switch ext {
            case "swift": return await ctx.sh.execute("swift \(p)\(extra)")
            case "py": return await ctx.sh.execute("python \(p)\(extra)")
            case "js": return await ctx.sh.execute("js \(p)\(extra)")
            case "sh": return await ctx.sh.execute("sh \(p)")
            case "json": return await ctx.sh.execute("json pretty \(p)")
            case "xml": return await ctx.sh.execute("xml pretty \(p)")
            case "plist": return await ctx.sh.execute("plist show \(p)")
            case "csv": return await ctx.sh.execute("csv head \(p)")
            case "md", "txt", "": return try ctx.input([p])
            default: throw ShErr("run: no sé abrir '.\(ext)' — mira 'support'")
            }
        }

        return c
    }

    // ========================================================
    // MARK: intérprete
    // ========================================================

    static func dispararEvento(_ evento: String, _ ctx: Ctx, nivel: Int = 0) async -> String {
        guard nivel <= Shell.simNivelMax else {
            return "emit \(evento): demasiados eventos encadenados — cortado\n"
        }
        Shell.simPrepara(ctx)
        let texto = (try? ctx.input(["/var/sim/events"])) ?? ""
        let nombre = evento.trimmingCharacters(in: .whitespaces)
        var out = ""
        var alguno = false
        for l in texto.components(separatedBy: "\n") {
            // se parte solo en el primer "=>": la acción puede llevar más
            guard let r = l.range(of: "=>") else { continue }
            let clave = String(l[l.startIndex..<r.lowerBound]).trimmingCharacters(in: .whitespaces)
            guard clave == nombre else { continue }
            let accion = String(l[r.upperBound...]).trimmingCharacters(in: .whitespaces)
            guard !accion.isEmpty else { continue }
            alguno = true
            out += await runSim(accion, ctx, nivel: nivel + 1)
        }
        return alguno ? out : "evento '\(nombre)': nadie lo escucha\n"
    }

    /// Condiciones de CUAL/ : "$X is Y", "file algo", "30" (porcentaje)
    static func condicionSim(_ cruda: String, _ ctx: Ctx) -> Bool {
        var c = cruda.trimmingCharacters(in: .whitespaces)
        // de la variable más larga a la más corta: si no, con $L definida
        // "$LOOP" se convertía en "…OOP"
        for k in ctx.env.vars.keys.sorted(by: { $0.count > $1.count }) {
            c = c.replacingOccurrences(of: "$" + k, with: ctx.env.vars[k] ?? "")
        }
        if c.isEmpty { return false }
        if let p = Double(c) { return Double.random(in: 0..<100) < p }
        if c.lowercased().hasPrefix("file ") {
            let n = String(c.dropFirst(5)).trimmingCharacters(in: .whitespaces)
            guard let u = try? ctx.env.resolve(n) else { return false }
            return ctx.env.exists(u)
        }
        let p = c.components(separatedBy: " ").filter { !$0.isEmpty }
        guard p.count >= 3 else { return c.lowercased() != "false" && c != "0" }
        let a = p[0], op = p[1].lowercased(), b = p.dropFirst(2).joined(separator: " ")
        switch op {
        case "is", "=", "==": return a == b
        case "not", "!=": return a != b
        case ">": return (Double(a) ?? 0) > (Double(b) ?? 0)
        case "<": return (Double(a) ?? 0) < (Double(b) ?? 0)
        default: return false
        }
    }

    static func runSim(_ linea: String, _ ctx: Ctx,
                       nivel: Int = 0, heredado: SimState? = nil) async -> String {
        let fm = FileManager.default
        // Loop/, For/ y emit vuelven a entrar aquí: sin tope, una línea que
        // se llama a sí misma bloqueaba la app entera.
        guard nivel <= Shell.simNivelMax else {
            return "sim: más de \(Shell.simNivelMax) niveles anidados — línea detenida\n"
        }
        if nivel == 0 { Shell.simPrepara(ctx) }
        // el archivo activo y la capa se heredan dentro de Loop/ y For/
        var st = heredado ?? SimState()
        var out = ""
        let bruto = linea.trimmingCharacters(in: .whitespaces)
        // las URL llevan diagonales, y "\/" es una diagonal literal:
        // las protegemos antes de partir la línea
        let marca = "\u{2}"
        let marcaBarra = "\u{3}"
        let protegido = bruto
            .replacingOccurrences(of: "\\/", with: marcaBarra)
            .replacingOccurrences(of: "://", with: marca + marca)
        var toks = protegido.components(separatedBy: "/").map {
            $0.trimmingCharacters(in: .whitespaces)
                .replacingOccurrences(of: marca + marca, with: "://")
                .replacingOccurrences(of: marcaBarra, with: "/")
        }.filter { !$0.isEmpty }
        guard !toks.isEmpty else { return "" }

        /// Sustituye $VARIABLES. Antes solo funcionaba dentro de CUAL/,
        /// así que 'Tex/$NOW' escribía el texto tal cual.
        func ex(_ s: String) -> String {
            guard s.contains("$") else { return s }
            var r = s
            for k in ctx.env.vars.keys.sorted(by: { $0.count > $1.count }) {
                r = r.replacingOccurrences(of: "$" + k, with: ctx.env.vars[k] ?? "")
            }
            return r
        }
        /// Entre comillas simples para el parser: rutas o textos con espacios
        /// se partían en varios argumentos al pasarlos a grep/cp/mv/du.
        func q(_ s: String) -> String {
            s.contains(" ") || s.contains("'") || s.contains("\"") || s.contains("|") || s.contains(";") || s.contains("&")
                ? "'" + s.replacingOccurrences(of: "'", with: "") + "'" : s
        }
        func sig() -> String? { toks.isEmpty ? nil : ex(toks.removeFirst()) }
        func sigC() -> String? { toks.isEmpty ? nil : toks.removeFirst() }   // sin expandir
        func resto() -> String {
            let t = toks.joined(separator: "/"); toks.removeAll(); return ex(t)
        }
        func restoC() -> String {
            let t = toks.joined(separator: "/"); toks.removeAll(); return t
        }
        func escribe(_ path: String, _ texto: String) -> Bool {
            guard !path.isEmpty, let u = try? ctx.env.resolve(path) else { return false }
            // si la carpeta no existía, la escritura fallaba en silencio
            let dir = u.deletingLastPathComponent()
            if !fm.fileExists(atPath: dir.path) {
                try? fm.createDirectory(at: dir, withIntermediateDirectories: true)
            }
            return (try? texto.write(to: u, atomically: true, encoding: .utf8)) != nil
        }
        func lee(_ path: String) -> String {
            guard !path.isEmpty, let u = try? ctx.env.resolve(path),
                  !ctx.env.isDir(u), let d = fm.contents(atPath: u.path) else { return "" }
            return String(data: d, encoding: .utf8) ?? ""
        }
        func añade(_ path: String, _ l: String) {
            _ = escribe(path, lee(path) + l + "\n")
        }

        while !toks.isEmpty {
            let crudo = toks.removeFirst()
            var verbo = crudo
            var argRaw = ""
            if let i = crudo.firstIndex(of: "=") {
                verbo = String(crudo[crudo.startIndex..<i])
                argRaw = String(crudo[crudo.index(after: i)...])
            }
            // Line(3): el número va pegado al verbo. Antes 'line(3)' no
            // coincidía con ningún caso y la orden se perdía.
            var numeroEntre: Int? = nil
            if let a = verbo.firstIndex(of: "("), let b = verbo.lastIndex(of: ")"), a < b {
                numeroEntre = Int(verbo[verbo.index(after: a)..<b].trimmingCharacters(in: .whitespaces))
                verbo = String(verbo[verbo.startIndex..<a])
            }
            verbo = verbo.trimmingCharacters(in: .whitespaces)
            let arg = ex(argRaw)
            let v = verbo.lowercased()

            if let motivo = Shell.simImposible[v] {
                out += "\(verbo)/: no disponible — \(motivo)\n"
                continue
            }

            switch v {
            case "usr": st.root = false
            case "usrroot": st.root = true

            case "api", "capa1", "capa2", "capa3", "capa4":
                if v.hasPrefix("capa") { st.capa = Int(String(v.suffix(1))) ?? 1 }
                else { st.capa = Int(arg) ?? st.capa }
                ctx.env.vars["CAPA"] = String(st.capa)

            case "createfile", "create":
                let nombre = arg.isEmpty ? (sig() ?? "") : arg
                guard !nombre.isEmpty else { out += "Createfile=: falta el nombre\n"; break }
                // antes vaciaba sin avisar un archivo que ya existía
                if let u = try? ctx.env.resolve(nombre), fm.fileExists(atPath: u.path) {
                    st.target = nombre
                    out += "\(nombre) ya existe — lo dejo intacto y lo pongo como activo\n"
                    out += "(para vaciarlo: Dump/ ...)\n"
                    break
                }
                if escribe(nombre, "") { st.target = nombre; out += "creado \(nombre)\n" }
                else { out += "no se pudo crear \(nombre)\n" }

            case "file":
                let n = arg.isEmpty ? (sig() ?? st.target) : arg
                guard !n.isEmpty else { out += "File/: falta el archivo\n"; break }
                st.target = n
                let t = lee(n)
                out += t.isEmpty ? "\(n): vacío o no existe\n" : t + (t.hasSuffix("\n") ? "" : "\n")

            case "edit":
                let n = arg.isEmpty ? (sig() ?? st.target) : arg
                guard !n.isEmpty else { out += "Edit/: falta el archivo\n"; break }
                st.target = n
                if let u = try? ctx.env.resolve(n) {
                    if !fm.fileExists(atPath: u.path) {
                        let dir = u.deletingLastPathComponent()
                        try? fm.createDirectory(at: dir, withIntermediateDirectories: true)
                        fm.createFile(atPath: u.path, contents: Data())
                    }
                    ctx.sh.uiEdit?(u)
                    out += "abriendo \(n) en el editor…\n"
                } else { out += "\(n): ruta no válida\n" }

            case "put":
                let texto = resto()
                guard !st.target.isEmpty else {
                    out += "Put/: no hay archivo activo (usa Createfile= o File/ antes)\n"; break
                }
                añade(st.target, texto)
                out += "añadido a \(st.target)\n"

            case "dump":
                let texto = resto()
                guard !st.target.isEmpty else {
                    out += "Dump/: no hay archivo activo (usa Createfile= o File/ antes)\n"; break
                }
                _ = escribe(st.target, texto + "\n")
                out += "volcado en \(st.target)\n"

            case "replace":
                let expr = resto()
                // se parte en el primer '>': así 'a>b>c' cambia "a" por "b>c"
                guard let corte = expr.range(of: ">"), !st.target.isEmpty else {
                    out += "REPLACE/: usa a>b con un archivo activo\n"; break
                }
                let viejo = String(expr[expr.startIndex..<corte.lowerBound])
                let nuevo = String(expr[corte.upperBound...])
                guard !viejo.isEmpty else { out += "REPLACE/: falta el texto a cambiar\n"; break }
                let antes = lee(st.target)
                let veces = antes.components(separatedBy: viejo).count - 1
                _ = escribe(st.target, antes.replacingOccurrences(of: viejo, with: nuevo))
                out += veces == 0 ? "REPLACE/: '\(viejo)' no aparece en \(st.target)\n"
                                  : "\(veces) cambio(s) en \(st.target)\n"

            case "direct":
                if let d = sig() {
                    if d.lowercased() == "file" {
                        let n = sig() ?? ""
                        st.target = n
                        out += lee(n)
                    } else if d.lowercased().hasPrefix("capa") {
                        st.capa = Int(String(d.suffix(1))) ?? st.capa
                    } else if let u = try? ctx.env.resolve(d), ctx.env.isDir(u) {
                        ctx.env.cwd = u
                        out += ctx.env.vpath(u) + "\n"
                    } else {
                        st.target = d
                    }
                }

            case "path":
                // la ayuda decía 'Path/ n=ruta' pero solo se aceptaba ':'
                var def = arg.isEmpty ? (sig() ?? "") : arg
                if arg.isEmpty, !def.contains(":"), !def.contains("="), let mas = toks.first {
                    def += ":" + mas; _ = sig()
                }
                let corte = def.range(of: ":") ?? def.range(of: "=")
                guard let r = corte else { out += "Path/: usa nombre:ruta\n"; break }
                let nombre = String(def[def.startIndex..<r.lowerBound]).trimmingCharacters(in: .whitespaces)
                let destino = String(def[r.upperBound...]).trimmingCharacters(in: .whitespaces)
                guard !nombre.isEmpty, !destino.isEmpty else { out += "Path/: usa nombre:ruta\n"; break }
                ctx.env.aliases[nombre] = "cd \(destino)"
                out += "atajo \(nombre) → \(destino)\n"

            case "remove":
                let n = arg.isEmpty ? (sig() ?? st.target) : arg
                guard !n.isEmpty else { out += "Remove/: falta qué borrar\n"; break }
                if let u = try? ctx.env.resolve(n), fm.fileExists(atPath: u.path) {
                    do {
                        try fm.removeItem(at: u)
                        if st.target == n { st.target = "" }
                        out += "borrado \(n)\n"
                    } catch { out += "no se pudo borrar \(n)\n" }
                } else { out += "\(n): no existe\n" }

            case "launch":
                var n = arg.isEmpty ? (sig() ?? st.target) : arg
                if n.lowercased() == "python3" || n.lowercased() == "python" {
                    out += "python3: no está — el único intérprete embebido aquí es JavaScriptCore.\n"
                    out += "Los .py no se pueden ejecutar; usa .js o .swift.\n"
                    break
                }
                if n.lowercased() == "node" { n = sig() ?? "" }
                guard !n.isEmpty else { out += "Launch/: falta el programa\n"; break }
                st.target = n
                // no repetir la misma entrada cada vez que se lanza
                var procs = lee("/var/sim/procs").components(separatedBy: "\n").filter { !$0.isEmpty }
                procs.removeAll { $0.hasPrefix(n + "  ") }
                procs.append("\(n)  activo")
                _ = escribe("/var/sim/procs", procs.joined(separator: "\n") + "\n")
                out += await ctx.sh.execute("run \(n)")

            case "install":
                let n = arg.isEmpty ? (sig() ?? "") : arg
                if ["qemu", "box64", "wine", "proot"].contains(n.lowercased()) {
                    out += "\(n): no se puede instalar — traduce o ejecuta código máquina, y iOS\n"
                    out += "solo corre binarios firmados. Ni a-Shell ni iSH pueden hacerlo tampoco.\n"
                    break
                }
                guard !n.isEmpty else { out += "Install/: falta el paquete\n"; break }
                if toks.joined(separator: "/").contains("://") {
                    let url = resto()
                    out += await ctx.sh.execute("pkg install \(n) \(url)")
                } else {
                    out += await ctx.sh.execute("pkg install \(n)")
                }

            case "get":
                var tipo = arg.lowercased()
                if let t = toks.first {
                    let cabeza = t.components(separatedBy: " ").first?.lowercased() ?? ""
                    if ["file", "repository", "repo"].contains(cabeza) {
                        tipo = cabeza
                        // 'Get/file url' viene en un solo trozo: quitamos la palabra
                        let sinCabeza = String(t.dropFirst(cabeza.count)).trimmingCharacters(in: .whitespaces)
                        if sinCabeza.isEmpty { _ = sig() } else { toks[0] = sinCabeza }
                    }
                }
                // el resto de la línea es la dirección: la volvemos a unir
                let destino = resto()
                guard !destino.isEmpty else { out += "Get/: falta la dirección\n"; break }
                if tipo == "repository" || tipo == "repo" {
                    // no hay gestor de repositorios: 'repo' no existe como comando
                    out += "Get/repository: aquí no hay repositorios remotos.\n"
                    out += "Descargo el archivo directamente:\n"
                }
                out += await ctx.sh.execute("wget \(destino)")

            case "import":
                out += "Get/import: copia el archivo a Documentos/shell con la app Archivos\n"

            case "wed", "web":
                let u = arg.isEmpty ? resto() : arg
                guard !u.isEmpty else { out += "Wed/: falta la dirección\n"; break }
                out += await ctx.sh.execute("open \(u)")

            case "sys":
                out += await ctx.sh.execute("config")
                out += "capa activa: \(st.capa)   modo: \(st.root ? "usrroot" : "usr")\n"

            case "tex":
                let texto = resto()
                out += texto + "\n"

            case "s":
                let valor = arg.isEmpty ? (sig() ?? "1") : arg
                let n = max(0, min(10.0, Double(valor) ?? 1))
                try? await Task.sleep(nanoseconds: UInt64(n * 1_000_000_000))

            case "wipe":
                ctx.sh.js.reset()
                out += "memoria del motor vaciada\n"

            case "reboot":
                ctx.sh.js.reset()
                ctx.sh.mode = .shell
                out += "motor de ejecución reiniciado\n"

            case "kill":
                let n = arg.isEmpty ? (sig() ?? "") : arg
                guard !n.isEmpty else { out += "Kill/: falta el proceso\n"; break }
                let todos = lee("/var/sim/procs").components(separatedBy: "\n").filter { !$0.isEmpty }
                let quedan = todos.filter { !$0.hasPrefix(n + "  ") }
                guard quedan.count < todos.count else {
                    out += "\(n): no está en la lista de procesos\n"; break
                }
                _ = escribe("/var/sim/procs", quedan.isEmpty ? "" : quedan.joined(separator: "\n") + "\n")
                out += "\(n) fuera de la lista\n"

            case "freeze", "thaw":
                let n = arg.isEmpty ? (sig() ?? st.target) : arg
                guard !n.isEmpty else { out += "\(verbo)/: falta el proceso\n"; break }
                let estado = v == "freeze" ? "congelado" : "activo"
                var lineas = lee("/var/sim/procs").components(separatedBy: "\n").filter { !$0.isEmpty }
                lineas.removeAll { $0.hasPrefix(n + "  ") }
                lineas.append("\(n)  \(estado)")
                _ = escribe("/var/sim/procs", lineas.joined(separator: "\n") + "\n")
                out += "\(n): \(estado)\n"

            case "fuse":
                let origen = arg.isEmpty ? (sig() ?? st.target) : arg
                guard !origen.isEmpty else { out += "Fuse/: falta el archivo\n"; break }
                let destino = (origen as NSString).deletingPathExtension + ".sys"
                let cuerpo = lee(origen)
                guard !cuerpo.isEmpty else { out += "Fuse/: \(origen) está vacío o no existe\n"; break }
                _ = escribe(destino, "# .sys generado por Fuse/ desde \(origen)\n" + cuerpo)
                out += "creado \(destino)\n"

            case "container":
                let bruta = arg.isEmpty ? (sig() ?? "caja") : arg
                let n = bruta.replacingOccurrences(of: "/", with: "_")
                if let u = try? ctx.env.resolve("/var/\(n)"),
                   (try? fm.createDirectory(at: u, withIntermediateDirectories: true)) != nil {
                    ctx.env.cwd = u
                    ctx.env.vars["PWD"] = ctx.env.vpath(u)
                    out += "contenedor \(n) listo, estás dentro\n"
                } else {
                    out += "Container/: no se pudo crear \(n)\n"
                }

            case "canal":
                if !arg.isEmpty {
                    _ = escribe("/var/canal/\(arg)", "")
                    st.target = "/var/canal/\(arg)"
                    out += "canal \(arg) abierto\n"
                } else if let siguiente = toks.first, siguiente.lowercased().hasPrefix("canal=") {
                    // canal/ une el anterior con el siguiente
                    let otro = String(siguiente.dropFirst(6))
                    _ = sig()
                    guard !otro.isEmpty else { out += "canal/: falta el nombre del segundo canal\n"; break }
                    guard !st.target.isEmpty else {
                        out += "canal/: no hay canal anterior — abre uno con canal=nombre\n"; break
                    }
                    let unido = lee(st.target) + lee("/var/canal/\(otro)")
                    _ = escribe("/var/canal/\(otro)", unido)
                    out += "canales unidos en \(otro)\n"
                    st.target = "/var/canal/\(otro)"
                } else {
                    out += "canal/: usa canal=nombre para abrirlo, o canal/canal=otro para unir\n"
                }

            case "save":
                let nombre = (arg.isEmpty ? (sigC() ?? "") : argRaw).trimmingCharacters(in: .whitespaces)
                guard !nombre.isEmpty else { out += "SAVE/=: falta el nombre\n"; break }
                // se quitaba solo si estaba escrito "SAVE=" en mayúsculas:
                // con 'save=x' la macro se volvía a guardar a sí misma sin fin
                var cuerpoMacro = bruto
                for forma in ["/SAVE=\(nombre)", "/save=\(nombre)", "/Save=\(nombre)",
                              "SAVE=\(nombre)/", "save=\(nombre)/", "Save=\(nombre)/",
                              "/SAVE/\(nombre)", "/save/\(nombre)"] {
                    cuerpoMacro = cuerpoMacro.replacingOccurrences(of: forma, with: "")
                }
                cuerpoMacro = cuerpoMacro.trimmingCharacters(in: CharacterSet(charactersIn: " /"))
                guard !cuerpoMacro.isEmpty, !cuerpoMacro.lowercased().contains("save=" + nombre.lowercased()) else {
                    out += "SAVE/: la macro se quedaría vacía o se llamaría a sí misma\n"; break
                }
                ctx.env.aliases[nombre] = "sim " + cuerpoMacro
                out += "macro \(nombre) guardada — lánzala escribiendo \(nombre)\n"

            case "detec", "detect":
                let evento = (arg.isEmpty ? (sig() ?? "") : arg).trimmingCharacters(in: .whitespaces)
                // sin expandir: las variables se resuelven cuando salte el evento
                let accion = restoC()
                guard !evento.isEmpty else { out += "Detec/: falta el nombre del evento\n"; break }
                guard !accion.isEmpty else { out += "Detec/\(evento): falta qué hacer\n"; break }
                // un evento que se dispara a sí mismo era un cuelgue seguro
                guard !accion.lowercased().contains("emit/" + evento.lowercased()),
                      !accion.lowercased().contains("emit=" + evento.lowercased()) else {
                    out += "Detec/\(evento): esa reacción se dispararía a sí misma\n"; break
                }
                añade("/var/sim/events", "\(evento) => \(accion)")
                out += "reaccionaré a '\(evento)'\n"

            case "emit":
                let evento = arg.isEmpty ? (sig() ?? "") : arg
                guard !evento.isEmpty else { out += "emit/: falta el evento\n"; break }
                out += await dispararEvento(evento, ctx, nivel: nivel + 1)

            case "wait":
                let valor = arg.isEmpty ? (sig() ?? "") : arg
                if let segundos = Double(valor) {
                    // Wait=-5 hacía UInt64 negativo: cierre inmediato de la app
                    let seg = max(0, min(segundos, 30))
                    try? await Task.sleep(nanoseconds: UInt64(seg * 1_000_000_000))
                } else {
                    // Wait/evento: el resto de la línea espera a ese evento
                    let accion = restoC()
                    guard !valor.isEmpty else { out += "Wait/: usa Wait=segundos o Wait/evento/acciones\n"; break }
                    if accion.isEmpty {
                        out += "Wait/\(valor): falta qué hacer cuando llegue el evento\n"
                    } else {
                        añade("/var/sim/events", "\(valor) => \(accion)")
                        out += "esperando '\(valor)' — dispáralo con 'emit \(valor)'\n"
                    }
                }

            case "message":
                let texto = arg.isEmpty ? resto() : arg
                guard !texto.isEmpty else { out += "Message=: falta el texto\n"; break }
                out += await Shell.mandarMensaje(texto, ctx)

            case "respondmessage":
                // respondmessage=texto{comandos}
                var clave = argRaw
                var accion = restoC()
                if let abre = clave.firstIndex(of: "{") {
                    accion = String(clave[clave.index(after: abre)...]) + (accion.isEmpty ? "" : "/" + accion)
                    clave = String(clave[clave.startIndex..<abre])
                }
                accion = accion.replacingOccurrences(of: "}", with: "")
                clave = clave.trimmingCharacters(in: .whitespaces)
                guard !clave.isEmpty, !accion.isEmpty else {
                    out += "respondmessage=: usa respondmessage=texto{comando}\n"; break
                }
                out += await ctx.sh.execute("respond " + clave + " " + accion)

            case "var":
                // Var=nombre:valor
                let def = arg.isEmpty ? (sig() ?? "") : arg
                let p = def.components(separatedBy: ":")
                let nombreVar = p[0].trimmingCharacters(in: .whitespaces)
                guard p.count >= 2, !nombreVar.isEmpty else { out += "Var=: usa Var=nombre:valor\n"; break }
                let valor = p.dropFirst().joined(separator: ":")
                ctx.env.vars[nombreVar] = valor
                st.valor = valor

            case "show":
                // Show/nombre — enseña lo que vale una variable
                // sin expandir: si no, Show/$X buscaba la variable llamada como su valor
                var n = (arg.isEmpty ? (sigC() ?? "") : argRaw).trimmingCharacters(in: .whitespaces)
                if n.hasPrefix("$") { n = String(n.dropFirst()) }
                guard !n.isEmpty else { out += "Show/: falta el nombre\n"; break }
                out += n + " = " + (ctx.env.vars[n] ?? "(vacía)") + "\n"

            case "random":
                // Random=1:100 — deja el resultado en $RANDOM
                let rango = arg.isEmpty ? (sig() ?? "1:100") : arg
                let p = rango.components(separatedBy: ":")
                let lo = Int(p.first ?? "1") ?? 1
                let hi = p.count > 1 ? (Int(p[1]) ?? 100) : 100
                guard lo <= hi else { out += "Random=: el mínimo es mayor que el máximo\n"; break }
                let n = Int.random(in: lo...hi)
                ctx.env.vars["RANDOM"] = String(n)
                out += String(n) + "\n"

            case "loop":
                // Loop=3/acciones — repite el resto de la línea
                let veces = min(100, max(1, Int(arg.isEmpty ? (sig() ?? "1") : arg) ?? 1))
                let acciones = restoC()   // sin expandir: $LOOP cambia en cada vuelta
                guard !acciones.isEmpty else { out += "Loop=: falta qué repetir\n"; break }
                for k in 1...veces {
                    ctx.env.vars["LOOP"] = String(k)
                    // el archivo activo se hereda dentro del bucle
                    out += await runSim(acciones, ctx, nivel: nivel + 1, heredado: st)
                }

            case "copy":
                let origen = arg.isEmpty ? (sig() ?? "") : arg
                let destino = sig() ?? ""
                guard !origen.isEmpty, !destino.isEmpty else { out += "Copy/: usa Copy/origen/destino\n"; break }
                out += await ctx.sh.execute("cp -r " + q(origen) + " " + q(destino))
                out += "copiado " + origen + " → " + destino + "\n"

            case "rename":
                let viejo = arg.isEmpty ? (sig() ?? "") : arg
                let nuevo = sig() ?? ""
                guard !viejo.isEmpty, !nuevo.isEmpty else { out += "Rename/: usa Rename/viejo/nuevo\n"; break }
                out += await ctx.sh.execute("mv " + q(viejo) + " " + q(nuevo))
                if st.target == viejo { st.target = nuevo }
                out += "ahora se llama " + nuevo + "\n"

            case "search":
                let texto = arg.isEmpty ? (sig() ?? "") : arg
                let donde = resto()
                guard !texto.isEmpty else { out += "Search/: falta qué buscar\n"; break }
                let objetivo = donde.isEmpty ? st.target : donde
                // antes 'Search/hola mundo' buscaba "hola" en un archivo llamado "mundo"
                let r = await ctx.sh.execute("grep -n " + q(texto) + " " + (objetivo.isEmpty ? "*" : q(objetivo)))
                out += r.isEmpty ? "'\(texto)': sin coincidencias\n" : r

            case "list":
                var donde = arg
                if donde.isEmpty { donde = sig() ?? "." }
                out += await ctx.sh.execute("ls -l " + q(donde))

            case "size":
                let n = arg.isEmpty ? (sig() ?? st.target) : arg
                guard !n.isEmpty else { out += "Size/: falta el archivo\n"; break }
                out += await ctx.sh.execute("du " + q(n))

            case "log":
                let texto = arg.isEmpty ? resto() : arg
                guard !texto.isEmpty else { out += "Log/: falta el texto\n"; break }
                let df = DateFormatter()
                df.dateFormat = "yyyy-MM-dd HH:mm:ss"
                añade("/var/sim/log", "[" + df.string(from: Date()) + "] " + texto)
                out += "anotado en /var/sim/log\n"

            case "texcommando", "texcomando":
                // texcommando=comando/ — deja el comando escrito en la terminal
                let cmd = arg.isEmpty ? resto() : arg
                guard !cmd.isEmpty else { out += "texcommando=: falta el comando\n"; break }
                ctx.sh.uiType?(cmd)
                out += "escrito en la terminal: " + cmd + "\n"

            case "ccomand", "ccommand":
                // ccomand=nombre{código}
                var nombre = argRaw
                var cuerpo = restoC()   // el código puede llevar '$' propios
                if let abre = nombre.firstIndex(of: "{") {
                    let dentro = String(nombre[nombre.index(after: abre)...])
                    nombre = String(nombre[nombre.startIndex..<abre])
                    cuerpo = dentro + (cuerpo.isEmpty ? "" : "/" + cuerpo)
                }
                cuerpo = cuerpo.replacingOccurrences(of: "}", with: "")
                nombre = nombre.trimmingCharacters(in: .whitespaces)
                guard !nombre.isEmpty, !cuerpo.trimmingCharacters(in: .whitespaces).isEmpty else {
                    out += "ccomand=: usa ccomand=nombre{código}\n"; break
                }
                out += await ctx.sh.execute("ccomand " + nombre + " {" + cuerpo + "}")

            case "camara", "camera":
                out += await ctx.sh.execute("camara")

            case "draw", "dibujar":
                let archivo = arg.isEmpty ? (sig() ?? "") : arg
                guard !archivo.isEmpty else { out += "Draw/: falta el archivo .draw\n"; break }
                out += await ctx.sh.execute("draw " + archivo)

            case "for":
                let sobre = arg.isEmpty ? (sig() ?? "") : arg
                let accion = restoC()   // sin expandir: $IT cambia en cada vuelta
                guard !sobre.isEmpty else { out += "For/: falta la lista\n"; break }
                guard !accion.isEmpty else { out += "For/\(sobre): falta qué hacer\n"; break }
                let items = lee(sobre).components(separatedBy: "\n").filter { !$0.isEmpty }
                guard !items.isEmpty else { out += "For/\(sobre): nada sobre lo que repetir\n"; break }
                for it in items.prefix(50) {
                    // antes se cambiaba TODO '$' por el elemento, así que
                    // $NOW o $CAPA dentro del bucle quedaban destrozados
                    ctx.env.vars["IT"] = it
                    let paso = accion
                        .replacingOccurrences(of: "$IT", with: it)
                        .replacingOccurrences(of: "$$", with: it)
                    out += await runSim(paso, ctx, nivel: nivel + 1, heredado: st)
                }
                if items.count > 50 { out += "For/: solo las primeras 50 líneas de \(sobre)\n" }

            case "line":
                // Line(3)/texto  o  Line=3/texto
                let n = numeroEntre ?? Int(arg)
                st.linea = n
                let texto = resto()
                guard let idx = n, idx >= 1 else {
                    out += "Line(n)/: usa Line(3)/texto — el número empieza en 1\n"; break
                }
                guard !st.target.isEmpty else {
                    out += "Line(\(idx))/: no hay archivo activo (usa File/ o Createfile= antes)\n"; break
                }
                var ls = lee(st.target).components(separatedBy: "\n")
                while ls.count < idx { ls.append("") }
                ls[idx - 1] = texto
                _ = escribe(st.target, ls.joined(separator: "\n"))
                out += "línea \(idx) de \(st.target) cambiada\n"

            case "color":
                let n = arg.isEmpty ? (sig() ?? "1") : arg
                let nombres = ["1": "blanco", "2": "azul", "3": "rojo", "4": "verde", "5": "negro", "6": "violeta"]
                ctx.env.vars["COLOR"] = n
                out += "color \(nombres[n] ?? n) guardado en $COLOR\n"

            case "huella":
                // Huella/frase — pide algo a la IA que aprende viendo
                let texto = arg.isEmpty ? resto() : arg
                out += await ctx.sh.execute(texto.isEmpty ? "huella" : "huella " + q(texto))

            case "nyx":
                // Nyx/pregunta — las 18 mentes deliberan; el veredicto queda en $NYX
                let texto = arg.isEmpty ? resto() : arg
                out += await ctx.sh.execute(texto.isEmpty ? "nyx" : "nyx " + q(texto))

            case "update":
                let n = arg.isEmpty ? (sig() ?? "") : arg
                out += await ctx.sh.execute("pkg install \(n)")

            case "hash":
                let n = arg.isEmpty ? (sig() ?? st.target) : arg
                guard !n.isEmpty else { out += "Hash/: falta el archivo\n"; break }
                guard let u = try? ctx.env.resolve(n), ctx.env.exists(u), !ctx.env.isDir(u),
                      let datos = fm.contents(atPath: u.path) else {
                    out += "Hash/: \(n) no existe\n"; break
                }
                // se hashean los bytes reales, no el texto (los binarios daban hash falso)
                #if (canImport(CryptoKit) && APPLE_COMPLETO)
                let h = SHA256.hash(data: datos).map { String(format: "%02x", $0) }.joined()
                #else
                let h = Plataforma.fnv64(datos)
                #endif
                ctx.env.vars["HASH"] = h
                out += h + "\n"

            case "b64":
                let texto = arg.isEmpty ? resto() : arg
                guard !texto.isEmpty else { out += "B64/: falta el texto\n"; break }
                let cod = Data(texto.utf8).base64EncodedString()
                ctx.env.vars["B64"] = cod
                out += cod + "\n"

            case "unb64":
                let texto = arg.isEmpty ? resto() : arg
                guard !texto.isEmpty else { out += "Unb64/: falta el texto\n"; break }
                guard let d = Data(base64Encoded: texto), let s = String(data: d, encoding: .utf8) else {
                    out += "Unb64/: '\(texto)' no es base64 válido\n"; break
                }
                ctx.env.vars["UNB64"] = s
                out += s + "\n"

            case "upper":
                let texto = arg.isEmpty ? resto() : arg
                let r = texto.uppercased()
                ctx.env.vars["UPPER"] = r
                out += r + "\n"

            case "lower":
                let texto = arg.isEmpty ? resto() : arg
                let r = texto.lowercased()
                ctx.env.vars["LOWER"] = r
                out += r + "\n"

            case "trim":
                let texto = arg.isEmpty ? resto() : arg
                let r = texto.trimmingCharacters(in: .whitespacesAndNewlines)
                ctx.env.vars["TRIM"] = r
                out += r + "\n"

            case "now":
                let d = Date()
                let df = DateFormatter()
                df.dateFormat = "yyyy-MM-dd HH:mm:ss"
                let f = df.string(from: d)
                ctx.env.vars["NOW"] = f
                ctx.env.vars["EPOCH"] = String(Int(d.timeIntervalSince1970))
                out += f + "\n"

            case "uuid":
                let u = UUID().uuidString
                ctx.env.vars["UUID"] = u
                out += u + "\n"

            case "ip":
                let r = await ctx.sh.execute("ip")
                ctx.env.vars["IP"] = r.trimmingCharacters(in: .whitespacesAndNewlines)
                out += r

            case "battery":
                out += await ctx.sh.execute("battery")

            case "weather":
                let ciudad = arg.isEmpty ? resto() : arg
                out += await ctx.sh.execute(ciudad.isEmpty ? "weather" : "weather " + q(ciudad))

            case "clip":
                let objetivo = arg.isEmpty ? (toks.isEmpty ? st.target : resto()) : arg
                guard !objetivo.isEmpty else { out += "Clip/: no hay nada que copiar\n"; break }
                let delArchivo = lee(objetivo)
                let contenido = delArchivo.isEmpty ? objetivo : delArchivo
                // antes pasaba por la línea de comandos: los saltos de línea,
                // comillas y '|' del archivo rompían la copia
                #if (canImport(UIKit) && APPLE_COMPLETO)
                await MainActor.run { UIPasteboard.general.string = contenido }
                out += "copiados \(contenido.count) caracteres" + (delArchivo.isEmpty ? "" : " de \(objetivo)") + "\n"
                #else
                out += await ctx.sh.execute("clip " + q(contenido))
                #endif

            case "clipget":
                let r = await ctx.sh.execute("clipget")
                let limpio = r.trimmingCharacters(in: .whitespacesAndNewlines)
                ctx.env.vars["CLIP"] = limpio == "(portapapeles vacío)" ? "" : limpio
                out += r

            case "post":
                let url = arg.isEmpty ? (sig() ?? "") : arg
                let datos = resto()
                guard !url.isEmpty else { out += "Post/: falta la URL\n"; break }
                guard url.contains("://") else { out += "Post/: '\(url)' no parece una URL (falta http://)\n"; break }
                // las comillas dentro de los datos cerraban el argumento antes de tiempo
                let r = await ctx.sh.execute("curl -X POST -d '" + datos.replacingOccurrences(of: "'", with: "") + "' " + url)
                ctx.env.vars["HTTP"] = r.trimmingCharacters(in: .whitespacesAndNewlines)
                out += r

            case "sort":
                let n = arg.isEmpty ? (sig() ?? st.target) : arg
                guard !n.isEmpty else { out += "Sort/: falta el archivo\n"; break }
                guard let un = try? ctx.env.resolve(n), ctx.env.exists(un), !ctx.env.isDir(un) else {
                    out += "Sort/: \(n) no existe\n"; break
                }
                var lineas = lee(n).components(separatedBy: "\n")
                if lineas.last == "" { lineas.removeLast() }
                lineas.sort { $0.lowercased() < $1.lowercased() }
                _ = escribe(n, lineas.joined(separator: "\n") + "\n")
                out += "\(n) ordenado\n"

            case "count":
                let n = arg.isEmpty ? (sig() ?? st.target) : arg
                guard !n.isEmpty else { out += "Count/: falta el archivo\n"; break }
                let t = lee(n)
                let l = t.isEmpty ? 0 : t.components(separatedBy: "\n").count - (t.hasSuffix("\n") ? 1 : 0)
                let w = t.split(whereSeparator: { $0.isWhitespace }).count
                let b = t.utf8.count
                ctx.env.vars["LINES"] = String(l)
                ctx.env.vars["WORDS"] = String(w)
                ctx.env.vars["BYTES"] = String(b)
                out += "\(l) líneas, \(w) palabras, \(b) bytes\n"

            case "matches":
                let texto = arg.isEmpty ? (sig() ?? "") : arg
                let n = sig() ?? st.target
                guard !texto.isEmpty, !n.isEmpty else { out += "Matches/: usa Matches/texto/archivo\n"; break }
                guard let um = try? ctx.env.resolve(n), ctx.env.exists(um) else {
                    out += "Matches/: \(n) no existe\n"; break
                }
                let conteo = lee(n).components(separatedBy: texto).count - 1
                ctx.env.vars["MATCHES"] = String(conteo)
                out += "\(conteo)\n"

            case "diff":
                let a = arg.isEmpty ? (sig() ?? "") : arg
                let b = sig() ?? ""
                guard !a.isEmpty, !b.isEmpty else { out += "Diff/: usa Diff/a/b\n"; break }
                out += await ctx.sh.execute("diff " + q(a) + " " + q(b))

            case "calc":
                let expr = arg.isEmpty ? resto() : arg
                guard !expr.isEmpty else { out += "Calc/: falta la expresión\n"; break }
                // pasar por la línea de comandos rompía '*' (comodín) y '( )';
                // y 'expr' evalúa JavaScript entero: aquí solo se admiten cuentas
                let permitidos = CharacterSet(charactersIn: "0123456789+-*/%().,^ ")
                guard expr.unicodeScalars.allSatisfy({ permitidos.contains($0) }) else {
                    out += "Calc/: solo números y + - * / % ( ) ^\n"; break
                }
                let js = expr.replacingOccurrences(of: ",", with: ".")
                             .replacingOccurrences(of: "^", with: "**")
                do {
                    let r = try ctx.sh.js.eval("String(" + js + ")").trimmingCharacters(in: .whitespacesAndNewlines)
                    ctx.env.vars["CALC"] = r
                    out += r + "\n"
                } catch {
                    out += "Calc/: '\(expr)' no es una cuenta válida\n"
                }

            case "exists":
                let n = arg.isEmpty ? (sig() ?? st.target) : arg
                guard !n.isEmpty else { ctx.env.vars["EXISTS"] = "0"; out += "Exists/: falta el archivo\n"; break }
                let ok = (try? ctx.env.resolve(n)).map { ctx.env.exists($0) } ?? false
                ctx.env.vars["EXISTS"] = ok ? "1" : "0"
                out += "\(n): \(ok ? "existe" : "no existe")\n"

            case "prepend":
                let texto = resto()
                guard !st.target.isEmpty else { out += "Prepend/: no hay archivo activo\n"; break }
                let previo = lee(st.target)
                _ = escribe(st.target, texto + "\n" + previo)
                out += "añadido al principio de \(st.target)\n"

            case "inc", "dec":
                // el nombre sin expandir: Inc/$X sumaba a la variable equivocada
                let def = arg.isEmpty ? (sigC() ?? "") : argRaw
                let p = def.components(separatedBy: ":")
                var nombre = (p.first ?? "").trimmingCharacters(in: .whitespaces)
                if nombre.hasPrefix("$") { nombre = String(nombre.dropFirst()) }
                guard !nombre.isEmpty else { out += "\(verbo)/: falta el nombre de la variable\n"; break }
                let cantidad = p.count > 1 ? (Double(ex(p[1])) ?? 1) : 1
                let actual = Double(ctx.env.vars[nombre] ?? "0") ?? 0
                let nuevo = v == "inc" ? actual + cantidad : actual - cantidad
                let texto = (nuevo == nuevo.rounded() && abs(nuevo) < 1e15) ? String(Int(nuevo)) : String(nuevo)
                ctx.env.vars[nombre] = texto
                out += "\(nombre) = \(texto)\n"

            case "stop":
                out += "Stop/: línea detenida\n"
                toks.removeAll()

            case "push":
                let destino = arg.isEmpty ? (sig() ?? "") : arg
                let anterior = ctx.env.vpath(ctx.env.cwd)
                // antes guardaba en la pila aunque el destino no existiera
                if !destino.isEmpty {
                    guard let u = try? ctx.env.resolve(destino), ctx.env.isDir(u) else {
                        out += "Push/: \(destino) no es una carpeta\n"; break
                    }
                    ctx.env.vars["DIRSTACK"] = anterior + "|" + (ctx.env.vars["DIRSTACK"] ?? "")
                    ctx.env.cwd = u
                    ctx.env.vars["PWD"] = ctx.env.vpath(u)
                    out += "guardado \(anterior), ahora en \(ctx.env.vpath(u))\n"
                } else {
                    ctx.env.vars["DIRSTACK"] = anterior + "|" + (ctx.env.vars["DIRSTACK"] ?? "")
                    out += "guardado \(anterior)\n"
                }

            case "pop":
                let pila = (ctx.env.vars["DIRSTACK"] ?? "").components(separatedBy: "|").filter { !$0.isEmpty }
                guard let anterior = pila.first, let u = try? ctx.env.resolve(anterior) else {
                    out += "Pop/: no hay directorios guardados\n"; break
                }
                ctx.env.vars["DIRSTACK"] = pila.dropFirst().joined(separator: "|")
                guard ctx.env.isDir(u) else { out += "Pop/: \(anterior) ya no existe\n"; break }
                ctx.env.cwd = u
                ctx.env.vars["PWD"] = ctx.env.vpath(u)
                out += "de vuelta en \(ctx.env.vpath(u))\n"

            case "vibrate":
                out += await ctx.sh.execute("vibrate")

            case "and":
                // encadena: la línea sigue pase lo que pase
                continue

            case "this":
                // se sustituye por el objetivo activo
                ctx.env.vars["THIS"] = st.target
                if st.target.isEmpty { out += "This/: no hay objetivo activo\n" }
                continue

            case "is":
                // is/A/B  compara A con B;  is=B  compara el archivo activo con B.
                // Antes 'is/a/b' comparaba el activo con "a" y luego intentaba
                // ejecutar "b" como si fuera un verbo.
                let a: String, b: String
                if !arg.isEmpty { a = st.target; b = arg }
                else { a = sig() ?? ""; b = sig() ?? "" }
                if a != b {
                    out += "is/: '\(a)' no es '\(b)' — línea detenida\n"
                    toks.removeAll()
                }
                continue

            case "cual":
                // CUAL/condición/acciones — dispara solo si se cumple
                let cond = arg.isEmpty ? (sig() ?? "") : arg
                if !Shell.condicionSim(cond, ctx) {
                    toks.removeAll()
                }
                continue

            case "%", "porcentaje":
                // %/30 — se cumple ese porcentaje de las veces
                let texto = (arg.isEmpty ? (sig() ?? "50") : arg).replacingOccurrences(of: "%", with: "")
                let p = max(0, min(100, Double(texto) ?? 50))
                if Double.random(in: 0..<100) >= p {
                    toks.removeAll()
                }
                continue

            case "mod":
                // Mod/destino/archivo — inyecta el mod al final del destino
                let destino = arg.isEmpty ? (sig() ?? st.target) : arg
                let modFile = sig() ?? ""
                guard !destino.isEmpty, !modFile.isEmpty else {
                    out += "Mod/: usa Mod/destino/archivo_del_mod\n"; break
                }
                let cuerpo = lee(modFile)
                guard !cuerpo.isEmpty else { out += "Mod/: \(modFile) está vacío o no existe\n"; break }
                _ = escribe(destino, lee(destino) + "\n# --- mod: \(modFile) ---\n" + cuerpo)
                st.target = destino
                out += "mod \(modFile) inyectado en \(destino)\n"

            case "extension":
                // registra una extensión para que las capas se comuniquen
                let n = arg.isEmpty ? (sig() ?? "") : arg
                guard !n.isEmpty else { out += "Extension/: falta el nombre\n"; break }
                let objetivo = st.target.isEmpty ? "(sin objetivo)" : st.target
                añade("/var/sim/ext", n + " => " + objetivo + "  capa " + String(st.capa))
                out += "extensión \(n) registrada\n"

            case "in":
                // in/=archivo — fija sobre qué se trabaja
                let n = arg.isEmpty ? (sig() ?? "") : arg
                guard !n.isEmpty else { out += "in/: falta sobre qué trabajar (in/=archivo)\n"; break }
                st.target = n
                ctx.env.vars["IN"] = n
                continue

            case "com":
                // estructura visual en texto
                let ramas = toks.map { ex($0) }
                toks.removeAll()
                guard !ramas.isEmpty else { out += "com/: falta el contenido\n"; break }
                for (k, r) in ramas.enumerated() {
                    out += (k == ramas.count - 1 ? "`-- " : "|-- ") + r + "\n"
                }

            case "interaction":
                ctx.env.vars["INTERACTION"] = "1"
                out += "modo interactivo activado ($INTERACTION)\n"

            case "beta":
                ctx.env.vars["BETA"] = arg.isEmpty ? "1" : arg
                out += "bandera BetA = \(ctx.env.vars["BETA"] ?? "1")\n"

            default:
                // ¿es un comando normal de la terminal?
                if ctx.sh.commands[v] != nil {
                    // el '=valor' también es argumento (echo=hola)
                    var partes = arg.isEmpty ? [] : [arg]
                    partes += toks.map { ex($0) }
                    toks.removeAll()
                    out += await ctx.sh.execute(([v] + partes).joined(separator: " "))
                } else if !crudo.isEmpty {
                    // un nombre suelto pasa a ser el objetivo; si no parece
                    // archivo ni existe, se avisa en vez de ignorarlo en silencio
                    let nombre = ex(crudo)
                    let existe = (try? ctx.env.resolve(nombre)).map { ctx.env.exists($0) } ?? false
                    if existe || nombre.contains(".") || nombre.contains("://") {
                        st.target = nombre
                    } else {
                        st.target = nombre
                        out += "\(verbo)/: no es un verbo de Simulacro ni un archivo — lo tomo como objetivo (mira 'simhelp')\n"
                    }
                }
            }
        }
        return out
    }
}
