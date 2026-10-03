import Foundation

// ============================================================
// MARK: - Comandos personalizados en varios idiomas (.mix)
// ============================================================
//
// ccomand=nombre{código}   crea un comando nuevo
// texcommando=comando/     escribe un comando en la terminal
//
// El código puede mezclar idiomas con etiquetas:
//
//   [js]   print("hola desde JavaScript");   [/js]
//   [swift] let x = 2 + 3
//           print("swift dice \(x)")          [/swift]
//   [sim]  Tex/y esto es Simulacro           [/sim]
//   [xml]  <raiz><a>1</a></raiz>             [/xml]
//
// Lo que va sin etiqueta se ejecuta como Simulacro o como
// comando normal de la terminal.

extension Shell {

    /// Trocea el código en bloques por idioma.
    static func trocearMix(_ src: String) -> [(lang: String, code: String)] {
        var bloques: [(String, String)] = []
        var actual = ""
        var lang = "sim"
        var i = src.startIndex

        func cerrar() {
            if !actual.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                bloques.append((lang, actual))
            }
            actual = ""
        }

        while i < src.endIndex {
            if src[i] == "[" {
                // ¿es una etiqueta de idioma?
                var etiqueta = ""
                var j = src.index(after: i)
                while j < src.endIndex, src[j] != "]", etiqueta.count < 12 {
                    etiqueta.append(src[j]); j = src.index(after: j)
                }
                if j < src.endIndex, src[j] == "]" {
                    let limpia = etiqueta.lowercased()
                    let conocidos = ["js", "javascript", "swift", "sim", "simulacro", "esc", "xml", "json", "java", "sh"]
                    if conocidos.contains(limpia) {
                        cerrar()
                        lang = limpia
                        i = src.index(after: j)
                        continue
                    }
                    if limpia.hasPrefix("/"), conocidos.contains(String(limpia.dropFirst())) {
                        cerrar()
                        lang = "sim"
                        i = src.index(after: j)
                        continue
                    }
                }
            }
            actual.append(src[i])
            i = src.index(after: i)
        }
        cerrar()
        return bloques
    }

    /// Ejecuta un comando mixto.
    static func runMix(_ src: String, args: [String], stdin: String, ctx: Ctx) async -> String {
        var out = ""
        for bloque in trocearMix(src) {
            let codigo = bloque.code.trimmingCharacters(in: .whitespacesAndNewlines)
            if codigo.isEmpty { continue }
            switch bloque.lang {
            case "js", "javascript":
                let prelude = "var ARGV = \(Shell.jsLiteral(args)); var STDIN = \(Shell.jsLiteral(stdin));"
                do { out += try ctx.sh.js.eval(prelude + "\n" + codigo) }
                catch { out += Shell.errMark + errText(error) + "\n" }

            case "swift":
                do {
                    let prelude = "var ARGV = \(Shell.jsLiteral(args)); var STDIN = \(Shell.jsLiteral(stdin));"
                    let js = try SwiftJS.transpile(codigo)
                    out += try ctx.sh.js.eval(prelude + "\n" + js)
                } catch {
                    out += Shell.errMark + errText(error) + "\n"
                }

            case "xml":
                if let d = codigo.data(using: .utf8), let raiz = try? XMLTreeBuilder().parse(d) {
                    out += raiz.pretty() + "\n"
                } else {
                    out += Shell.errMark + "xml: no se pudo analizar el bloque\n"
                }

            case "json":
                if let bonito = try? DataTools.jsonPretty(codigo) { out += bonito + "\n" }
                else { out += Shell.errMark + "json: el bloque no es válido\n" }

            case "java":
                out += Shell.errMark + "java: no hay máquina virtual de Java en iOS.\n" +
                       "Los bloques [java] no se ejecutan; usa [js] o [swift].\n"

            default:
                // Simulacro, .esc o comandos normales, línea a línea
                for linea in codigo.components(separatedBy: "\n") {
                    var t = linea.trimmingCharacters(in: .whitespaces)
                    if t.isEmpty || t.hasPrefix("#") { continue }
                    for (k, a) in args.enumerated() {
                        t = t.replacingOccurrences(of: "$\(k + 1)", with: a)
                    }
                    t = t.replacingOccurrences(of: "$ARGV", with: args.joined(separator: " "))
                    out += await ctx.sh.execute(t)
                }
            }
        }
        return out
    }

    static func mixCommands() -> [String: Spec] {
        var c: [String: Spec] = [:]
        let fm = FileManager.default

        c["ccomand"] = Spec(help: "ccomand <nombre> {código} — crea un comando propio en varios idiomas") { ctx in
            guard !ctx.args.isEmpty else {
                return """
                ccomand — crea un comando tuyo

                  ccomand saluda {[js] print("hola " + ARGV[0]); [/js]}

                También en Simulacro:
                  ccomand=saluda{Tex/hola $1}

                Mezcla idiomas con [js] [swift] [sim] [xml] [json].
                Se guarda en \(Shell.binDir) y se llama por su nombre.
                Míralos con 'mix list'.

                """
            }
            let todo = ctx.args.joined(separator: " ")
            var nombre = todo
            var cuerpo = ""
            if let abre = todo.firstIndex(of: "{") {
                nombre = String(todo[todo.startIndex..<abre]).trimmingCharacters(in: .whitespaces)
                var resto = String(todo[todo.index(after: abre)...])
                if resto.hasSuffix("}") { resto.removeLast() }
                cuerpo = resto
            }
            nombre = nombre.replacingOccurrences(of: "=", with: "").trimmingCharacters(in: .whitespaces)
            guard !nombre.isEmpty else { throw ShErr("ccomand: falta el nombre") }
            guard !cuerpo.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
                throw ShErr("ccomand: falta el código entre llaves { }")
            }
            let bin = try ctx.env.resolve(Shell.binDir)
            try fm.createDirectory(at: bin, withIntermediateDirectories: true)
            let destino = bin.appendingPathComponent("\(nombre).mix")
            try cuerpo.write(to: destino, atomically: true, encoding: .utf8)
            let idiomas = Set(Shell.trocearMix(cuerpo).map { $0.lang }).sorted().joined(separator: ", ")
            return "creado el comando '\(nombre)' (\(idiomas))\nya puedes escribir: \(nombre)\n"
        }

        c["mix"] = Spec(help: "mix <list|show|new|edit|remove> — comandos propios mixtos") { ctx in
            let bin = try ctx.env.resolve(Shell.binDir)
            try fm.createDirectory(at: bin, withIntermediateDirectories: true)
            let sub = ctx.args.first ?? "list"
            let rest = Array(ctx.args.dropFirst())

            switch sub {
            case "list":
                let items = ((try? fm.contentsOfDirectory(atPath: bin.path)) ?? [])
                    .filter { $0.hasSuffix(".mix") }.sorted()
                if items.isEmpty { return "no hay comandos mixtos. Crea uno con 'mix new saluda'.\n" }
                var out = ""
                for f in items {
                    let n = (f as NSString).deletingPathExtension
                    let cuerpo = (fm.contents(atPath: bin.appendingPathComponent(f).path))
                        .flatMap { String(data: $0, encoding: .utf8) } ?? ""
                    let idiomas = Set(Shell.trocearMix(cuerpo).map { $0.lang }).sorted().joined(separator: ", ")
                    out += "\(n)   [\(idiomas)]\n"
                }
                return out

            case "new":
                guard let n = rest.first else { throw ShErr("mix new: falta el nombre") }
                let destino = bin.appendingPathComponent("\(n).mix")
                guard !fm.fileExists(atPath: destino.path) else { throw ShErr("mix: '\(n)' ya existe") }
                let plantilla = """
                # comando '\(n)' — mezcla los idiomas que quieras
                # $1 $2 $3 son los argumentos

                [sim]
                Tex/empezando \(n)
                [/sim]

                [js]
                print("desde JavaScript, argumentos: " + ARGV.join(" "));
                [/js]

                [swift]
                let total = 2 + 3
                print("desde Swift: \\(total)")
                [/swift]
                """
                try plantilla.write(to: destino, atomically: true, encoding: .utf8)
                ctx.sh.uiEdit?(destino)
                return "creado \(n) — pruébalo escribiendo: \(n)\n"

            case "show":
                guard let n = rest.first else { throw ShErr("mix show: falta el nombre") }
                guard let d = fm.contents(atPath: bin.appendingPathComponent("\(n).mix").path) else {
                    throw ShErr("mix: '\(n)' no existe")
                }
                return (String(data: d, encoding: .utf8) ?? "") + "\n"

            case "edit":
                guard let n = rest.first else { throw ShErr("mix edit: falta el nombre") }
                let u = bin.appendingPathComponent("\(n).mix")
                guard fm.fileExists(atPath: u.path) else { throw ShErr("mix: '\(n)' no existe") }
                ctx.sh.uiEdit?(u)
                return ""

            case "remove":
                guard let n = rest.first else { throw ShErr("mix remove: falta el nombre") }
                let u = bin.appendingPathComponent("\(n).mix")
                guard fm.fileExists(atPath: u.path) else { throw ShErr("mix: '\(n)' no existe") }
                try fm.removeItem(at: u)
                return "borrado \(n)\n"

            default:
                throw ShErr("mix: usa list, show, new, edit o remove")
            }
        }

        c["texcommando"] = Spec(help: "texcommando <comando> — escribe un comando en la terminal") { ctx in
            let cmd = ctx.args.joined(separator: " ")
            guard !cmd.isEmpty else { throw ShErr("texcommando: falta el comando") }
            ctx.sh.uiType?(cmd)
            return "escrito en la terminal: \(cmd)\n"
        }

        return c
    }
}
