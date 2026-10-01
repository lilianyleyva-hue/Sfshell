import Foundation

// ============================================================
// MARK: - Editor de líneas (nano / vi / edit) — solo texto
// ============================================================
// Sin ventana: el archivo se edita escribiendo en la misma terminal.
//   texto suelto     se añade al final
//   :p               muestra el archivo numerado
//   :3 texto         cambia la línea 3
//   :i 3 texto       inserta antes de la línea 3
//   :d 3  :d 3-5     borra líneas
//   :s/viejo/nuevo/  reemplaza en todo el archivo
//   :w   :q   :wq   guardar, salir sin guardar, guardar y salir
//   :r               guarda y ejecuta (como ^R en el nano de antes)

struct EdicionTexto {
    let url: URL
    let nombre: String
    var lineas: [String]
    var cambiado = false
}

extension Shell {

    static let ayudaEditor = """
    editor — escribe líneas para añadirlas al final
      :p  ver   :3 texto  cambiar   :i 3 texto  insertar   :d 3[-5]  borrar
      :s/viejo/nuevo/  reemplazar   :w  guardar   :q  salir   :wq  guardar y salir   :r  guardar y ejecutar

    """

    func abrirEditor(_ url: URL) {
        let texto = (try? String(contentsOf: url, encoding: .utf8)) ?? ""
        var lineas = texto.components(separatedBy: "\n")
        if lineas.last == "" { lineas.removeLast() }
        let e = EdicionTexto(url: url, nombre: url.lastPathComponent, lineas: lineas)
        editor = e
        avisoEditor = "editando \(env.vpath(url)) (\(lineas.count) líneas)\n" + Shell.ayudaEditor + Shell.numerar(e.lineas)
    }

    static func numerar(_ l: [String]) -> String {
        guard !l.isEmpty else { return "  (vacío)\n" }
        return l.enumerated().map { "\(String($0.offset + 1).leftPad(4))  \($0.element)" }.joined(separator: "\n") + "\n"
    }

    /// Una línea escrita mientras el editor está abierto.
    func lineaEditor(_ linea: String) async -> String {
        guard var e = editor else { return "" }
        defer { if editor != nil { editor = e } }
        let t = linea.trimmingCharacters(in: .whitespaces)

        func guardar() -> String {
            let cuerpo = e.lineas.isEmpty ? "" : e.lineas.joined(separator: "\n") + "\n"
            do {
                try cuerpo.write(to: e.url, atomically: true, encoding: .utf8)
                e.cambiado = false
                return "guardado \(env.vpath(e.url))\n"
            } catch {
                return Shell.errMark + errText(error) + "\n"
            }
        }
        func indice(_ s: Substring) -> Int? {
            guard let n = Int(s), n >= 1, n <= e.lineas.count else { return nil }
            return n - 1
        }

        guard t.hasPrefix(":") else {
            e.lineas.append(linea)
            e.cambiado = true
            return ""
        }
        let orden = t.dropFirst()
        switch orden {
        case "p": return Shell.numerar(e.lineas)
        case "h", "ayuda", "help": return Shell.ayudaEditor
        case "w": return guardar()
        case "q":
            editor = nil
            return e.cambiado ? "salí sin guardar (los cambios se perdieron)\n" : "cerrado\n"
        case "wq", "x":
            let r = guardar()
            editor = nil
            return r
        case "r":
            let r = guardar()
            editor = nil
            return r + (await execute("run " + env.vpath(e.url)))
        default: break
        }
        if orden.hasPrefix("s/") {
            let partes = orden.dropFirst(2).split(separator: "/", omittingEmptySubsequences: false)
            guard partes.count >= 2, !partes[0].isEmpty else { return Shell.errMark + "uso: :s/viejo/nuevo/\n" }
            var n = 0
            e.lineas = e.lineas.map { l in
                let c = l.components(separatedBy: String(partes[0])).count - 1
                n += c
                return l.replacingOccurrences(of: String(partes[0]), with: String(partes[1]))
            }
            if n > 0 { e.cambiado = true }
            return "\(n) cambios\n"
        }
        if orden.hasPrefix("d") {
            let rango = orden.dropFirst().trimmingCharacters(in: .whitespaces).split(separator: "-")
            guard let a = rango.first.flatMap({ indice($0) }) else { return Shell.errMark + "uso: :d 3  o  :d 3-5\n" }
            let b = rango.count > 1 ? (indice(rango[1]) ?? a) : a
            guard b >= a else { return Shell.errMark + "rango al revés\n" }
            e.lineas.removeSubrange(a...b)
            e.cambiado = true
            return "borradas \(b - a + 1) líneas\n"
        }
        if orden.hasPrefix("i ") {
            let resto = orden.dropFirst(2)
            let num = resto.prefix { $0.isNumber }
            guard let n = Int(num), n >= 1, n <= e.lineas.count + 1 else { return Shell.errMark + "uso: :i 3 texto\n" }
            e.lineas.insert(String(resto.dropFirst(num.count).drop { $0 == " " }), at: n - 1)
            e.cambiado = true
            return ""
        }
        let num = orden.prefix { $0.isNumber }
        if !num.isEmpty, let i = indice(num) {
            e.lineas[i] = String(orden.dropFirst(num.count).drop { $0 == " " })
            e.cambiado = true
            return ""
        }
        return Shell.errMark + "orden de editor desconocida (:h para ayuda)\n"
    }
}
