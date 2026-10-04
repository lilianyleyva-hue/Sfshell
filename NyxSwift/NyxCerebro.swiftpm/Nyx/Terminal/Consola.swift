import Foundation

// Consola.swift — la terminal de Nyx: una shell nueva (estilo Termux) con su
// propio disco, tuberías, redirecciones, variables y lenguajes de programación.
//
//  · Disco: carpetas y archivos de verdad dentro de la app (Documentos/NyxTerminal).
//    «/» es la raíz de ese disco y tu casa es /home/nyx (~).
//  · Línea de órdenes: comillas '…' "…", \, $VAR ${VAR} $? $(orden), ~, comodines * ?,
//    tuberías |, redirecciones > >> < 2> 2>&1 &>, y && || ; para encadenar.
//  · Órdenes: las de un Linux (ls, cd, cat, grep, sed, find…), los lenguajes
//    (python, node, gcc, g++, javac, java) y «nyx» para hablar con Nyx.

/// Lo que la terminal necesita de Nyx (solo Nyx: una sola mente).
@MainActor
protocol CerebroTerminal: AnyObject {
    func terminalPregunta(_ texto: String) -> (respuesta: String, pensamiento: [String])
    func terminalImagina(_ texto: String) -> String
    func terminalAprende(_ texto: String) -> String
    func terminalLee(_ url: URL, nombre: String)
    func terminalEstado() -> String
    func terminalGuardaMemoria() async -> Data?
    func terminalCargaMemoria(_ url: URL)
}

/// Una línea de la pantalla.
struct LineaTerminal: Identifiable {
    enum Tipo { case normal, error, orden, aviso }
    let id: Int
    var texto: String
    let tipo: Tipo
    var abierta = false          // la última línea aún no terminó (sin "\n")
}

@MainActor
final class Consola {
    // MARK: estado

    let raiz: URL
    var cwd = "/home/nyx"
    var entorno: [String: String] = [:]
    var alias: [String: String] = [:]
    var historial: [String] = []
    var ultimoCodigo: Int32 = 0
    weak var cerebro: CerebroTerminal?
    /// La pantalla: cada trozo de salida que llega.
    var alEscribir: ((String, LineaTerminal.Tipo) -> Void)?
    var alLimpiar: (() -> Void)?
    /// Abrir el editor de texto con un archivo (nano, edit…).
    var alEditar: ((String) -> Void)?
    /// Para parar lo que se está ejecutando (botón ⏹ / Ctrl-C).
    private let parada = BanderaParada()
    private(set) var ocupada = false
    let inicio = Date()

    init(raiz: URL? = nil) {
        let docs = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first
            ?? URL(fileURLWithPath: NSTemporaryDirectory())
        self.raiz = raiz ?? docs.appendingPathComponent("NyxTerminal")
        for d in ["home/nyx", "tmp"] {
            try? FileManager.default.createDirectory(at: self.raiz.appendingPathComponent(d), withIntermediateDirectories: true)
        }
        entorno = ["HOME": "/home/nyx", "USER": "nyx", "SHELL": "/bin/nyxsh", "PATH": "/bin:/usr/bin",
                   "TERM": "xterm-256color", "LANG": "es_ES.UTF-8", "PWD": "/home/nyx", "PS1": "nyx@ipad:\\w$ "]
        alias = ["ll": "ls -l", "la": "ls -a", "l": "ls", "vi": "nano", "vim": "nano", "python3": "python",
                 "cls": "clear", "dir": "ls", "py": "python"]
    }

    /// El texto del prompt: nyx@ipad:~$
    var prompt: String {
        var donde = cwd
        if donde == "/home/nyx" { donde = "~" } else if donde.hasPrefix("/home/nyx/") { donde = "~" + donde.dropFirst(9) }
        return "nyx@ipad:\(donde)$ "
    }

    func para() { parada.activa() }

    func escribe(_ t: String) { alEscribir?(t, .normal) }
    func error(_ t: String) { alEscribir?(t, .error) }

    // MARK: rutas del disco

    /// Ruta virtual absoluta y normalizada ("~/a/../b" → "/home/nyx/b").
    func absoluta(_ ruta: String) -> String {
        var r = ruta.isEmpty ? cwd : ruta
        if r == "~" { r = "/home/nyx" } else if r.hasPrefix("~/") { r = "/home/nyx/" + r.dropFirst(2) }
        if !r.hasPrefix("/") { r = cwd + "/" + r }
        var partes: [String] = []
        for p in r.split(separator: "/") {
            if p == "." || p.isEmpty { continue }
            if p == ".." { if !partes.isEmpty { partes.removeLast() }; continue }
            partes.append(String(p))
        }
        return "/" + partes.joined(separator: "/")
    }

    /// El archivo de verdad (nunca sale de la raíz del disco de la terminal).
    func url(_ ruta: String) -> URL {
        let a = absoluta(ruta)
        var u = raiz
        for p in a.split(separator: "/") { u = u.appendingPathComponent(String(p)) }
        return u
    }

    func existe(_ ruta: String) -> Bool { FileManager.default.fileExists(atPath: url(ruta).path) }

    func esCarpeta(_ ruta: String) -> Bool {
        var d: ObjCBool = false
        return FileManager.default.fileExists(atPath: url(ruta).path, isDirectory: &d) && d.boolValue
    }

    func lee(_ ruta: String) -> String? {
        guard let d = try? Data(contentsOf: url(ruta)) else { return nil }
        return String(data: d, encoding: .utf8) ?? String(data: d, encoding: .isoLatin1)
    }

    @discardableResult
    func escribeArchivo(_ ruta: String, _ texto: String, añadir: Bool = false) -> Bool {
        let u = url(ruta)
        if añadir, let h = try? FileHandle(forWritingTo: u) {
            h.seekToEndOfFile()
            h.write(Data(texto.utf8))
            h.closeFile()
            return true
        }
        return (try? Data(texto.utf8).write(to: u)) != nil
    }

    func lista(_ ruta: String) -> [String] {
        let todo = (try? FileManager.default.contentsOfDirectory(atPath: url(ruta).path)) ?? []
        return todo.sorted { $0.lowercased() < $1.lowercased() }
    }

    // MARK: ejecutar una línea

    /// Ejecuta lo que escribiste (con historial, alias, tuberías…).
    func ejecuta(_ linea: String) async {
        let t = linea.trimmingCharacters(in: .whitespaces)
        alEscribir?(prompt + linea + "\n", .orden)
        if t.isEmpty { return }
        if historial.last != t { historial.append(t) }
        if historial.count > 500 { historial.removeFirst(historial.count - 500) }
        ocupada = true
        parada.reinicia()
        let codigo = await ejecutaTexto(t, entrada: nil, captura: nil)
        ultimoCodigo = codigo
        ocupada = false
    }

    /// Ejecuta un texto (también lo usan los scripts .sh y $(…)).
    /// Si `captura` no es nil, la salida estándar se guarda ahí en vez de mostrarse.
    func ejecutaTexto(_ texto: String, entrada: String?, captura: Caja<String>?) async -> Int32 {
        let ordenes: [LineaShell.Enlazada]
        do {
            ordenes = try LineaShell.analiza(texto)
        } catch let e as LineaShell.Fallo {
            error("nyxsh: \(e.mensaje)\n")
            return 2
        } catch {
            return 2
        }
        var codigo: Int32 = 0
        for o in ordenes {
            if parada.activada { return 130 }
            if o.enlace == .y && codigo != 0 { continue }
            if o.enlace == .o && codigo == 0 { continue }
            codigo = await ejecutaTuberia(o.tuberia, entrada: entrada, captura: captura)
            ultimoCodigo = codigo
        }
        return codigo
    }

    /// cmd1 | cmd2 | cmd3: la salida de cada una es la entrada de la siguiente.
    private func ejecutaTuberia(_ t: [LineaShell.Orden], entrada: String?, captura: Caja<String>?) async -> Int32 {
        var tuberia: String? = entrada
        var codigo: Int32 = 0
        for (k, orden) in t.enumerated() {
            let ultima = k == t.count - 1
            let salida = Caja("")
            let destino: Caja<String>? = ultima ? captura : salida
            codigo = await ejecutaOrden(orden, entrada: tuberia, captura: destino)
            tuberia = salida.valor
            if parada.activada { return 130 }
        }
        return codigo
    }

    /// Una orden con sus redirecciones.
    private func ejecutaOrden(_ o: LineaShell.Orden, entrada: String?, captura: Caja<String>?) async -> Int32 {
        // expandir variables, ~, $(…) y comodines
        var palabras: [String] = []
        for p in o.palabras {
            let e = await expande(p)
            palabras.append(contentsOf: e)
        }
        // asignaciones sueltas: A=1
        if !palabras.isEmpty, palabras.allSatisfy({ esAsignacion($0) }) {
            for a in palabras { asigna(a) }
            return 0
        }
        guard !palabras.isEmpty else { return 0 }
        // alias
        if let al = alias[palabras[0]] {
            let resto = palabras.dropFirst().map { citado($0) }.joined(separator: " ")
            return await ejecutaTexto(al + (resto.isEmpty ? "" : " " + resto), entrada: entrada, captura: captura)
        }
        // entrada < archivo
        var stdin = entrada
        if let f = o.entrada {
            let ruta = (await expande(f)).first ?? ""
            guard let t = lee(ruta) else {
                error("nyxsh: \(ruta): no existe el archivo\n")
                return 1
            }
            stdin = t
        }
        // salida > o >> archivo
        let salida = Caja("")
        let errores = Caja("")
        let redirigeSalida = o.salida != nil
        let redirigeError = o.errores != nil || o.errorASalida
        var ctx = Contexto(consola: self, args: Array(palabras.dropFirst()), nombre: palabras[0], entrada: stdin ?? "",
                           hayEntrada: stdin != nil,
                           escribe: { [weak self] (t: String) in
                               if redirigeSalida || captura != nil { salida.valor += t } else { self?.escribe(t) }
                           },
                           error: { [weak self] (t: String) in
                               if o.errorASalida { if redirigeSalida || captura != nil { salida.valor += t } else { self?.escribe(t) } }
                               else if redirigeError { errores.valor += t } else { self?.error(t) }
                           },
                           parada: parada)
        ctx.aPantalla = !(redirigeSalida || captura != nil)
        let codigo = await ejecutaComando(ctx)
        if let s = o.salida {
            let ruta = (await expande(s.archivo)).first ?? ""
            if !escribeArchivo(ruta, salida.valor, añadir: s.añadir) && !(s.añadir && escribeArchivo(ruta, salida.valor)) {
                error("nyxsh: \(ruta): no se puede escribir\n")
            }
        } else if let c = captura {
            c.valor += salida.valor
        }
        if let e = o.errores {
            let ruta = (await expande(e.archivo)).first ?? ""
            escribeArchivo(ruta, errores.valor, añadir: e.añadir)
        }
        return codigo
    }

    // MARK: expansiones

    private func esAsignacion(_ p: String) -> Bool {
        guard let i = p.firstIndex(of: "="), i != p.startIndex else { return false }
        return p[..<i].allSatisfy { $0.isLetter || $0.isNumber || $0 == "_" } && !(p.first?.isNumber ?? true)
    }

    func asigna(_ a: String) {
        guard let i = a.firstIndex(of: "=") else { return }
        entorno[String(a[..<i])] = String(a[a.index(after: i)...])
    }

    /// Pone comillas si hace falta (para volver a analizar un alias).
    private func citado(_ s: String) -> String {
        if s.isEmpty { return "''" }
        if s.contains(where: { " \t|&;<>()$'\"\\*?".contains($0) }) { return "'" + s.replacingOccurrences(of: "'", with: "'\\''") + "'" }
        return s
    }

    /// Una palabra → una o varias (los comodines pueden dar varias).
    func expande(_ p: LineaShell.Palabra) async -> [String] {
        var texto = ""
        var comodin = false
        for trozo in p.trozos {
            switch trozo {
            case .literal(let t, let protegido):
                texto += t
                if !protegido && (t.contains("*") || t.contains("?")) { comodin = true }
            case .variable(let n):
                texto += valor(de: n)
            case .orden(let o) where o.hasPrefix("(") && o.hasSuffix(")"):
                texto += cuenta(String(o.dropFirst().dropLast()))
            case .orden(let o):
                let c = Caja("")
                _ = await ejecutaTexto(o, entrada: nil, captura: c)
                var r = c.valor
                while r.hasSuffix("\n") { r.removeLast() }
                texto += r
            case .casa:
                texto += "/home/nyx"
            }
        }
        if comodin {
            let encontrados = globo(texto)
            if !encontrados.isEmpty { return encontrados }
        }
        return [texto]
    }

    /// $(( … )): cuentas con enteros; las variables valen su número (o 0).
    func cuenta(_ e: String) -> String {
        var t = ""
        var nombre = ""
        func suelta() {
            if !nombre.isEmpty { t += Int(valor(de: nombre)).map(String.init) ?? "0"; nombre = "" }
        }
        for ch in e {
            if ch.isLetter || ch == "_" || (!nombre.isEmpty && ch.isNumber) { nombre.append(ch); continue }
            suelta()
            if ch != "$" { t.append(ch) }
        }
        suelta()
        t = t.replacingOccurrences(of: "**", with: "^")
        if let g = Aritmetica.calculaExacto(t) { return g.texto }
        if let v = Aritmetica.calcula(t), v.isFinite, abs(v) < 9e18 { return String(Int(v)) }
        error("nyxsh: $((\(e))): no entiendo la cuenta\n")
        return ""
    }

    func expande(_ s: String) async -> [String] {
        guard let p = try? LineaShell.palabras(s).first else { return [s] }
        return await expande(p)
    }

    func valor(de n: String) -> String {
        switch n {
        case "?": return String(ultimoCodigo)
        case "PWD": return cwd
        case "RANDOM": return String(Int.random(in: 0 ... 32767))
        case "$": return "4242"
        case "0": return "nyxsh"
        default: return entorno[n] ?? ""
        }
    }

    /// *.py, a?.txt, carpeta/*.c
    private func globo(_ patron: String) -> [String] {
        let abs = patron.hasPrefix("/") || patron.hasPrefix("~")
        let partes = patron.split(separator: "/").map(String.init)
        guard let ultima = partes.last else { return [] }
        let carpeta = partes.dropLast().joined(separator: "/")
        let base = carpeta.isEmpty ? (abs ? "/" : ".") : (patron.hasPrefix("/") ? "/" + carpeta : carpeta)
        guard let re = Consola.regexComodin(ultima) else { return [] }
        var out: [String] = []
        for f in lista(base) where !f.hasPrefix(".") || ultima.hasPrefix(".") {
            if re.firstMatch(in: f, range: NSRange(f.startIndex..., in: f)) != nil {
                out.append(carpeta.isEmpty ? (abs ? "/" + f : f) : base + "/" + f)
            }
        }
        return out.sorted()
    }

    static func regexComodin(_ p: String) -> NSRegularExpression? {
        var r = "^"
        for ch in p {
            switch ch {
            case "*": r += ".*"
            case "?": r += "."
            default: r += NSRegularExpression.escapedPattern(for: String(ch))
            }
        }
        return try? NSRegularExpression(pattern: r + "$")
    }
}

/// Una caja para pasar texto por referencia.
final class Caja<T> {
    var valor: T
    init(_ v: T) { valor = v }
}

/// Para parar un programa desde la pantalla (también desde otros hilos).
final class BanderaParada: @unchecked Sendable {
    private let candado = NSLock()
    private var valor = false
    var activada: Bool {
        candado.lock()
        defer { candado.unlock() }
        return valor
    }
    func activa() { candado.lock(); valor = true; candado.unlock() }
    func reinicia() { candado.lock(); valor = false; candado.unlock() }
}

/// Lo que recibe cada orden.
@MainActor
struct Contexto {
    let consola: Consola
    var args: [String]
    let nombre: String
    let entrada: String
    let hayEntrada: Bool
    let escribe: (String) -> Void
    let error: (String) -> Void
    let parada: BanderaParada
    /// false si la salida va a una tubería o a un archivo (ls pone uno por línea).
    var aPantalla = true

    /// Las opciones (-l, -n 5…) y el resto de argumentos.
    func opciones(_ conValor: Set<Character> = []) -> (op: [Character: String], resto: [String]) {
        var op: [Character: String] = [:]
        var resto: [String] = []
        var i = 0
        var fin = false
        while i < args.count {
            let a = args[i]
            if !fin && a == "--" { fin = true; i += 1; continue }
            if !fin && a.hasPrefix("-") && a.count > 1 && !(Double(a) != nil) {
                var cs = Array(a.dropFirst())
                while !cs.isEmpty {
                    let c = cs.removeFirst()
                    if conValor.contains(c) {
                        if !cs.isEmpty { op[c] = String(cs); cs = [] } else if i + 1 < args.count { i += 1; op[c] = args[i] } else { op[c] = "" }
                    } else {
                        op[c] = ""
                    }
                }
            } else {
                resto.append(a)
            }
            i += 1
        }
        return (op, resto)
    }

    /// El texto de los archivos dados, o la entrada si no hay archivos.
    func textos(_ archivos: [String]) -> [(nombre: String, texto: String)]? {
        if archivos.isEmpty { return [("-", entrada)] }
        var out: [(String, String)] = []
        for f in archivos {
            if f == "-" { out.append(("-", entrada)); continue }
            if consola.esCarpeta(f) { error("\(nombre): \(f): es una carpeta\n"); return nil }
            guard let t = consola.lee(f) else { error("\(nombre): \(f): no existe el archivo\n"); return nil }
            out.append((f, t))
        }
        return out
    }
}
