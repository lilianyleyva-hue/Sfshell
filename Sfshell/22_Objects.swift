import Foundation

// ============================================================
// MARK: - Objetos .eop, cabeceras .dlf y mensajes
// ============================================================
//
// .eop  — un objeto: propiedades y métodos
// .dlf  — una cabecera: definiciones que otros archivos incluyen
//         (como los .h de C)
//
// Message=texto            manda un mensaje
// respondmessage=texto{…}  registra qué hacer cuando llegue

extension Shell {

    static let objDir = "/var/obj"
    static let msgDir = "/var/msg"

    static func objetos() -> [String: Spec] {
        var c: [String: Spec] = [:]
        let fm = FileManager.default

        func prepara(_ ctx: Ctx) {
            for d in [Shell.objDir, Shell.msgDir] {
                if let u = try? ctx.env.resolve(d) {
                    try? fm.createDirectory(at: u, withIntermediateDirectories: true)
                }
            }
        }

        let plantillaEop = """
        # Objeto .eop — propiedades y métodos
        objeto Criatura
          nombre = sin nombre
          energia = 10
          color = verde

          metodo mirar
            Tex/$self.nombre — energía $self.energia, color $self.color
          end

          metodo comer
            set nueva=$(expr $self.energia + 3)
            obj set $self.id energia $nueva
            Tex/$self.nombre come y sube a $nueva
          end

          metodo vive
            if $self.energia > 0
              Tex/$self.nombre sigue viva
            else
              Tex/$self.nombre se apagó
            end
          end
        end
        """

        let plantillaDlf = """
        # Cabecera .dlf — se incluye con: include comun.dlf
        # Lo que pongas aquí queda disponible en el archivo que la incluya.

        set VERSION=1.0
        set AUTOR=yo

        def saludar
          Tex/hola desde la cabecera, versión $VERSION
        end

        def separador
          Tex/------------------------
        end
        """

        // --- .eop -------------------------------------------

        c["eop"] = Spec(help: "eop <new|show|list> <archivo> — objetos .eop") { ctx in
            let sub = ctx.args.first ?? "list"
            let rest = Array(ctx.args.dropFirst())
            switch sub {
            case "new":
                guard let n = rest.first else { throw ShErr("eop new: falta el nombre") }
                let nombre = n.hasSuffix(".eop") ? n : n + ".eop"
                let u = try ctx.env.resolve(nombre)
                guard !ctx.env.exists(u) else { throw ShErr("eop: \(nombre) ya existe") }
                try plantillaEop.write(to: u, atomically: true, encoding: .utf8)
                ctx.sh.uiEdit?(u)
                return "creado \(nombre)\ncrea una instancia con: obj crear \(nombre) mi1\n"
            case "show":
                guard let n = rest.first else { throw ShErr("eop show: falta el archivo") }
                return try ctx.input([n])
            case "list":
                let items = ((try? fm.contentsOfDirectory(atPath: ctx.env.cwd.path)) ?? [])
                    .filter { $0.hasSuffix(".eop") }.sorted()
                return items.isEmpty ? "no hay objetos .eop aquí\n" : items.joined(separator: "\n") + "\n"
            default:
                throw ShErr("eop: usa new, show o list")
            }
        }

        c["obj"] = Spec(help: "obj <crear|set|get|call|list|borrar> — instancias de objetos") { ctx in
            prepara(ctx)
            let sub = ctx.args.first ?? "list"
            let rest = Array(ctx.args.dropFirst())
            let dir = try ctx.env.resolve(Shell.objDir)

            func cargar(_ id: String) throws -> [String: String] {
                let u = dir.appendingPathComponent("\(id).json")
                guard let d = fm.contents(atPath: u.path),
                      let j = try? JSONSerialization.jsonObject(with: d) as? [String: String] else {
                    throw ShErr("obj: no existe la instancia '\(id)'")
                }
                return j
            }
            func guardar(_ id: String, _ props: [String: String]) throws {
                let d = try JSONSerialization.data(withJSONObject: props, options: [.sortedKeys])
                try d.write(to: dir.appendingPathComponent("\(id).json"))
            }

            switch sub {
            case "crear":
                guard rest.count >= 2 else { throw ShErr("obj crear: usa obj crear archivo.eop nombre") }
                let texto = try ctx.input([rest[0]])
                var props: [String: String] = ["id": rest[1], "eop": rest[0]]
                var dentroMetodo = false
                for l in texto.components(separatedBy: "\n") {
                    let t = l.trimmingCharacters(in: .whitespaces)
                    if t.hasPrefix("metodo ") { dentroMetodo = true; continue }
                    if t == "end" { dentroMetodo = false; continue }
                    if dentroMetodo || t.isEmpty || t.hasPrefix("#") || t.hasPrefix("objeto ") { continue }
                    guard let eq = t.firstIndex(of: "=") else { continue }
                    let k = String(t[t.startIndex..<eq]).trimmingCharacters(in: .whitespaces)
                    let v = String(t[t.index(after: eq)...]).trimmingCharacters(in: .whitespaces)
                    if !k.isEmpty { props[k] = v }
                }
                try guardar(rest[1], props)
                let campos = props.keys.sorted().joined(separator: ", ")
                return "creado el objeto \(rest[1]) con: \(campos)\n"

            case "set":
                guard rest.count >= 3 else { throw ShErr("obj set: usa obj set nombre propiedad valor") }
                var props = try cargar(rest[0])
                props[rest[1]] = rest.dropFirst(2).joined(separator: " ")
                try guardar(rest[0], props)
                return ""

            case "get":
                guard rest.count >= 2 else { throw ShErr("obj get: usa obj get nombre propiedad") }
                let props = try cargar(rest[0])
                guard let v = props[rest[1]] else { throw ShErr("obj: '\(rest[0])' no tiene '\(rest[1])'") }
                return v + "\n"

            case "show":
                guard let id = rest.first else { throw ShErr("obj show: falta el nombre") }
                let props = try cargar(id)
                return props.sorted { $0.key < $1.key }.map { "\($0.key) = \($0.value)" }
                    .joined(separator: "\n") + "\n"

            case "list":
                let items = ((try? fm.contentsOfDirectory(atPath: dir.path)) ?? [])
                    .map { ($0 as NSString).deletingPathExtension }.sorted()
                return items.isEmpty ? "no hay objetos creados\n" : items.joined(separator: "\n") + "\n"

            case "borrar":
                guard let id = rest.first else { throw ShErr("obj borrar: falta el nombre") }
                try fm.removeItem(at: dir.appendingPathComponent("\(id).json"))
                return "borrado \(id)\n"

            case "call":
                guard rest.count >= 2 else { throw ShErr("obj call: usa obj call nombre metodo") }
                let props = try cargar(rest[0])
                guard let eop = props["eop"] else { throw ShErr("obj: la instancia no recuerda su .eop") }
                let texto = try ctx.input([eop])
                // sacamos el cuerpo del método
                var cuerpo: [String] = []
                var dentro = false
                var nivel = 0
                for l in texto.components(separatedBy: "\n") {
                    let t = l.trimmingCharacters(in: .whitespaces)
                    if !dentro {
                        if t == "metodo \(rest[1])" { dentro = true; nivel = 1 }
                        continue
                    }
                    if t.hasPrefix("if ") || t.hasPrefix("repeat ") || t.hasPrefix("while ") || t.hasPrefix("metodo ") {
                        nivel += 1
                    } else if t == "end" {
                        nivel -= 1
                        if nivel == 0 { break }
                    }
                    cuerpo.append(l)
                }
                guard !cuerpo.isEmpty else { throw ShErr("obj: '\(eop)' no tiene el método '\(rest[1])'") }
                // $self.propiedad
                for (k, v) in props { ctx.env.vars["self.\(k)"] = v }
                let tmp = "/var/obj/_metodo.esc"
                try cuerpo.joined(separator: "\n").write(to: try ctx.env.resolve(tmp), atomically: true, encoding: .utf8)
                let salida = try await Shell.runEsc(tmp, ctx)
                for k in props.keys { ctx.env.vars.removeValue(forKey: "self.\(k)") }
                return salida

            default:
                throw ShErr("obj: usa crear, set, get, show, call, list o borrar")
            }
        }

        // --- .dlf -------------------------------------------

        c["dlf"] = Spec(help: "dlf <new|show|list> — cabeceras .dlf, como los .h de C") { ctx in
            let sub = ctx.args.first ?? "list"
            let rest = Array(ctx.args.dropFirst())
            switch sub {
            case "new":
                guard let n = rest.first else { throw ShErr("dlf new: falta el nombre") }
                let nombre = n.hasSuffix(".dlf") ? n : n + ".dlf"
                let u = try ctx.env.resolve(nombre)
                guard !ctx.env.exists(u) else { throw ShErr("dlf: \(nombre) ya existe") }
                try plantillaDlf.write(to: u, atomically: true, encoding: .utf8)
                ctx.sh.uiEdit?(u)
                return "creada \(nombre)\ninclúyela con: include \(nombre)\n"
            case "show":
                guard let n = rest.first else { throw ShErr("dlf show: falta el archivo") }
                return try ctx.input([n])
            case "list":
                let items = ((try? fm.contentsOfDirectory(atPath: ctx.env.cwd.path)) ?? [])
                    .filter { $0.hasSuffix(".dlf") }.sorted()
                return items.isEmpty ? "no hay cabeceras .dlf aquí\n" : items.joined(separator: "\n") + "\n"
            default:
                throw ShErr("dlf: usa new, show o list")
            }
        }

        // --- mensajes ---------------------------------------

        c["message"] = Spec(help: "message <texto> — manda un mensaje y dispara a quien responda") { ctx in
            prepara(ctx)
            let texto = ctx.args.joined(separator: " ")
            guard !texto.isEmpty else { throw ShErr("message: falta el texto") }
            return await Shell.mandarMensaje(texto, ctx)
        }

        c["respond"] = Spec(help: "respond <texto> <comando> — qué hacer cuando llegue ese mensaje") { ctx in
            prepara(ctx)
            guard ctx.args.count >= 2 else { throw ShErr("respond: usa respond <texto> <comando>") }
            let texto = ctx.args[0]
            let accion = ctx.args.dropFirst().joined(separator: " ")
            let u = try ctx.env.resolve("/var/msg/responders")
            let viejo = (try? ctx.input(["/var/msg/responders"])) ?? ""
            try (viejo + "\(texto) => \(accion)\n").write(to: u, atomically: true, encoding: .utf8)
            return "responderé a '\(texto)'\n"
        }

        c["messages"] = Spec(help: "messages [-c] — mensajes recibidos") { ctx in
            prepara(ctx)
            if ctx.args.contains("-c") {
                let u = try ctx.env.resolve("/var/msg/buzon")
                try "".write(to: u, atomically: true, encoding: .utf8)
                return "buzón vacío\n"
            }
            let t = (try? ctx.input(["/var/msg/buzon"])) ?? ""
            return t.isEmpty ? "no hay mensajes\n" : t
        }

        c["responders"] = Spec(help: "responders — respuestas registradas") { ctx in
            prepara(ctx)
            let t = (try? ctx.input(["/var/msg/responders"])) ?? ""
            return t.isEmpty ? "nadie responde a nada todavía\n" : t
        }

        return c
    }

    /// Guarda el mensaje en el buzón y ejecuta lo que responda a él.
    static func mandarMensaje(_ texto: String, _ ctx: Ctx) async -> String {
        let df = DateFormatter()
        df.dateFormat = "HH:mm:ss"
        let linea = "[\(df.string(from: Date()))] \(texto)"
        if let u = try? ctx.env.resolve("/var/msg/buzon") {
            let viejo = (try? String(contentsOf: u, encoding: .utf8)) ?? ""
            try? (viejo + linea + "\n").write(to: u, atomically: true, encoding: .utf8)
        }
        var out = "mensaje: \(texto)\n"
        let reglas = (try? ctx.input(["/var/msg/responders"])) ?? ""
        for r in reglas.components(separatedBy: "\n") {
            let partes = r.components(separatedBy: "=>")
            guard partes.count == 2 else { continue }
            let clave = partes[0].trimmingCharacters(in: .whitespaces)
            guard !clave.isEmpty, texto.contains(clave) else { continue }
            out += await ctx.sh.execute(partes[1].trimmingCharacters(in: .whitespaces))
        }
        return out
    }
}
