import Foundation

// ============================================================
// MARK: - .esc — programar con comandos de Simulacro
// ============================================================
// Un .esc es un guion donde cada línea es un comando de Simulacro
// o de la terminal, más unas cuantas estructuras para poder
// programar de verdad: variables, condiciones, repeticiones y
// bloques con nombre.
//
//   # comentario
//   set nombre=Angelo
//   Tex/hola $nombre
//
//   if $nombre is Angelo
//     Tex/eres tú
//   else
//     Tex/no te conozco
//   end
//
//   repeat 3
//     Tex/vuelta
//   end
//
//   def saludar
//     Tex/hola desde el bloque
//   end
//   call saludar

extension Shell {

    static let escLimite = 5000   // tope de líneas ejecutadas, para no colgar la app

    static func esc() -> [String: Spec] {
        var c: [String: Spec] = [:]
        let fm = FileManager.default

        let plantillaEsc = """
        # Mi primer programa en .esc
        # Cada línea es un comando de Simulacro o de la terminal.

        set nombre=mundo
        Tex/hola $nombre

        set veces=3
        repeat $veces
          Tex/vuelta
        end

        if $nombre is mundo
          Tex/todo en orden
        else
          Tex/alguien cambió el nombre
        end

        def despedida
          Tex/hasta luego
        end
        call despedida
        """

        c["esc"] = Spec(help: "esc <new|run|list|show> <archivo> — programas en lenguaje Simulacro") { ctx in
            let sub = ctx.args.first ?? "list"
            let rest = Array(ctx.args.dropFirst())

            switch sub {
            case "new":
                guard let n = rest.first else { throw ShErr("esc new: falta el nombre") }
                let nombre = n.hasSuffix(".esc") ? n : n + ".esc"
                let u = try ctx.env.resolve(nombre)
                guard !ctx.env.exists(u) else { throw ShErr("esc: \(nombre) ya existe") }
                try plantillaEsc.write(to: u, atomically: true, encoding: .utf8)
                ctx.sh.uiEdit?(u)
                return "creado \(nombre) — se abre el editor\nlánzalo con: esc run \(nombre)\n"

            case "run":
                guard let n = rest.first else { throw ShErr("esc run: falta el archivo") }
                return try await Shell.runEsc(n, ctx)

            case "show":
                guard let n = rest.first else { throw ShErr("esc show: falta el archivo") }
                return try ctx.input([n])

            case "list":
                let items = ((try? fm.contentsOfDirectory(atPath: ctx.env.cwd.path)) ?? [])
                    .filter { $0.hasSuffix(".esc") }.sorted()
                if items.isEmpty { return "no hay programas .esc aquí. Crea uno con: esc new mio\n" }
                return items.joined(separator: "\n") + "\n"

            case "help":
                return Shell.escAyuda

            default:
                throw ShErr("esc: usa new, run, show, list o help")
            }
        }

        c["eschelp"] = Spec(help: "eschelp — cómo se escribe un .esc") { _ in Shell.escAyuda }

        // run con .esc añadido
        c["run"] = Spec(help: "run <archivo> — ejecuta cualquier tipo admitido, incluidos .sim y .esc") { ctx in
            guard let p = ctx.args.first else { throw ShErr("run: falta el archivo") }
            let u = try ctx.env.resolve(p)
            guard ctx.env.exists(u), !ctx.env.isDir(u) else {
                let items = ((try? fm.contentsOfDirectory(atPath: ctx.env.cwd.path)) ?? []).sorted()
                throw ShErr("run: \(p): no existe en \(ctx.env.vpath(ctx.env.cwd))\n" +
                            "aquí hay: " + (items.isEmpty ? "(nada — prueba 'demo')" : items.joined(separator: "  ")))
            }
            let ext = u.pathExtension.lowercased()
            if ext == "esc" { return try await Shell.runEsc(p, ctx) }
            if ext == "sim" || ext == "sys" {
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

    static let escAyuda = """
    ARCHIVOS .esc — programar con Simulacro

    Cada línea suelta es un comando: vale Simulacro (Tex/hola) o
    cualquier comando de la terminal (ls -l). Encima tienes esto:

      # comentario                    no hace nada
      set nombre=valor                guarda una variable
      set n=$(comando)                guarda la salida de un comando
                                      ej: set x=$(expr 2 + 3)
      $nombre                         la usa en cualquier línea

      if CONDICION                    condición
        ...
      else
        ...
      end

      repeat N                        repite N veces ($i es la vuelta)
        ...
      end

      while CONDICION                 repite mientras se cumpla
        ...
      end

      def nombre                      bloque con nombre
        ...
      end
      call nombre                     lo ejecuta

      stop                            termina el programa
      wait N   /  wait=N              espera N segundos
      include cabecera.dlf            trae definiciones de otro archivo
      message texto                   manda un mensaje
      obj call nombre metodo          llama a un método de un objeto

    CONDICIONES
      $x is valor          iguales
      $x not valor         distintos
      $x > 5   $x < 5      comparación numérica
      file archivo         el archivo existe
      % 30                 se cumple el 30% de las veces

    Se lanza con 'esc run mio.esc', con 'run mio.esc' o con ./mio.esc
    """

    // --- el intérprete -------------------------------------

    static func runEsc(_ path: String, _ ctx: Ctx) async throws -> String {
        let texto = try ctx.input([path])
        var lineas = texto.components(separatedBy: "\n")
        if lineas.last == "" { lineas.removeLast() }

        // include archivo.dlf — mete aquí las líneas de la cabecera
        var expandidas: [String] = []
        for l in lineas {
            let t = l.trimmingCharacters(in: .whitespaces)
            if t.lowercased().hasPrefix("include ") {
                let nombre = String(t.dropFirst(8)).trimmingCharacters(in: .whitespaces)
                if let cabecera = try? ctx.input([nombre]) {
                    expandidas.append(contentsOf: cabecera.components(separatedBy: "\n"))
                } else {
                    expandidas.append("Tex/no encontré la cabecera " + nombre)
                }
                continue
            }
            expandidas.append(l)
        }
        lineas = expandidas
        var bloques: [String: (Int, Int)] = [:]
        var salida = ""
        var ejecutadas = 0
        var parar = false

        // busca el 'end' que cierra el bloque abierto en 'desde'
        func cierre(_ desde: Int) -> Int {
            var nivel = 0
            var i = desde
            while i < lineas.count {
                let t = lineas[i].trimmingCharacters(in: .whitespaces).lowercased()
                if t.hasPrefix("if ") || t.hasPrefix("repeat ") || t.hasPrefix("while ") || t.hasPrefix("def ") {
                    nivel += 1
                } else if t == "end" {
                    nivel -= 1
                    if nivel == 0 { return i }
                }
                i += 1
            }
            return lineas.count - 1
        }

        // el 'else' de este 'if', si lo hay
        func alternativa(_ desde: Int, _ hasta: Int) -> Int? {
            var nivel = 0
            var i = desde
            while i < hasta {
                let t = lineas[i].trimmingCharacters(in: .whitespaces).lowercased()
                if t.hasPrefix("if ") || t.hasPrefix("repeat ") || t.hasPrefix("while ") || t.hasPrefix("def ") { nivel += 1 }
                else if t == "end" { nivel -= 1 }
                else if t == "else" && nivel == 1 { return i }
                i += 1
            }
            return nil
        }

        func expande(_ s: String) -> String {
            var r = s
            for (k, v) in ctx.env.vars {
                r = r.replacingOccurrences(of: "$\(k)", with: v)
            }
            return r
        }

        func condicion(_ crudo: String) -> Bool {
            let c = expande(crudo).trimmingCharacters(in: .whitespaces)
            if c.hasPrefix("%") {
                let p = Double(c.dropFirst().trimmingCharacters(in: .whitespaces)) ?? 50
                return Double.random(in: 0..<100) < p
            }
            if c.lowercased().hasPrefix("file ") {
                let n = String(c.dropFirst(5)).trimmingCharacters(in: .whitespaces)
                guard let u = try? ctx.env.resolve(n) else { return false }
                return ctx.env.exists(u)
            }
            let partes = c.components(separatedBy: " ").filter { !$0.isEmpty }
            guard partes.count >= 3 else { return !c.isEmpty && c != "0" && c.lowercased() != "false" }
            let a = partes[0]
            let op = partes[1].lowercased()
            let b = partes.dropFirst(2).joined(separator: " ")
            switch op {
            case "is", "==", "=": return a == b
            case "not", "!=": return a != b
            case ">": return (Double(a) ?? 0) > (Double(b) ?? 0)
            case "<": return (Double(a) ?? 0) < (Double(b) ?? 0)
            case ">=": return (Double(a) ?? 0) >= (Double(b) ?? 0)
            case "<=": return (Double(a) ?? 0) <= (Double(b) ?? 0)
            default: return false
            }
        }

        // ejecuta un rango de líneas
        func corre(_ desde: Int, _ hasta: Int) async {
            var i = desde
            while i < hasta && !parar {
                ejecutadas += 1
                if ejecutadas > Shell.escLimite {
                    salida += Shell.errMark + "esc: pasadas \(Shell.escLimite) líneas, lo paro por seguridad\n"
                    parar = true
                    return
                }
                let bruta = lineas[i].trimmingCharacters(in: .whitespaces)
                let baja = bruta.lowercased()

                if bruta.isEmpty || bruta.hasPrefix("#") { i += 1; continue }

                if baja == "stop" { parar = true; return }
                if baja == "end" || baja == "else" { i += 1; continue }

                if baja.hasPrefix("set ") {
                    let resto = String(bruta.dropFirst(4))
                    if let eq = resto.firstIndex(of: "=") {
                        let k = String(resto[resto.startIndex..<eq]).trimmingCharacters(in: .whitespaces)
                        var v = expande(String(resto[resto.index(after: eq)...])).trimmingCharacters(in: .whitespaces)
                        // set x=$(comando) guarda la salida del comando
                        if v.contains("$(") {
                            v = ((try? await ctx.sh.substitute(v)) ?? v).trimmingCharacters(in: .whitespaces)
                        }
                        ctx.env.vars[k] = v
                    } else {
                        salida += Shell.errMark + "esc línea \(i + 1): 'set' necesita nombre=valor\n"
                    }
                    i += 1; continue
                }

                if baja.hasPrefix("wait ") || baja.hasPrefix("wait=") {
                    let crudo = String(bruta.dropFirst(5))
                    let s = min(10.0, Double(expande(crudo).trimmingCharacters(in: .whitespaces)) ?? 1)
                    try? await Task.sleep(nanoseconds: UInt64(s * 1_000_000_000))
                    i += 1; continue
                }

                if baja.hasPrefix("def ") {
                    let nombre = String(bruta.dropFirst(4)).trimmingCharacters(in: .whitespaces)
                    let fin = cierre(i)
                    bloques[nombre] = (i + 1, fin)
                    i = fin + 1; continue
                }

                if baja.hasPrefix("call ") {
                    let nombre = String(bruta.dropFirst(5)).trimmingCharacters(in: .whitespaces)
                    if let b = bloques[nombre] { await corre(b.0, b.1) }
                    else { salida += Shell.errMark + "esc: no hay ningún bloque llamado '\(nombre)'\n" }
                    i += 1; continue
                }

                if baja.hasPrefix("if ") {
                    let fin = cierre(i)
                    let otro = alternativa(i, fin)
                    if condicion(String(bruta.dropFirst(3))) {
                        await corre(i + 1, otro ?? fin)
                    } else if let o = otro {
                        await corre(o + 1, fin)
                    }
                    i = fin + 1; continue
                }

                if baja.hasPrefix("repeat ") {
                    let fin = cierre(i)
                    let n = min(1000, Int(expande(String(bruta.dropFirst(7))).trimmingCharacters(in: .whitespaces)) ?? 0)
                    if n > 0 {
                        for vuelta in 1...n {
                            if parar { break }
                            ctx.env.vars["i"] = String(vuelta)
                            await corre(i + 1, fin)
                        }
                    }
                    i = fin + 1; continue
                }

                if baja.hasPrefix("while ") {
                    let fin = cierre(i)
                    var vueltas = 0
                    while condicion(String(bruta.dropFirst(6))) && !parar {
                        vueltas += 1
                        if vueltas > 1000 {
                            salida += Shell.errMark + "esc línea \(i + 1): el 'while' no termina, lo corto\n"
                            break
                        }
                        await corre(i + 1, fin)
                    }
                    i = fin + 1; continue
                }

                // línea normal: Simulacro o comando de la terminal
                salida += await ctx.sh.execute(expande(bruta))
                i += 1
            }
        }

        await corre(0, lineas.count)
        return salida
    }
}
