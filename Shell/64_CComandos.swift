import Foundation

// ============================================================
// MARK: - Comandos: gcc, g++, clang, javac, java, ./programa
// ============================================================
// gcc hola.c -o hola   → revisa el programa entero y crea 'hola'
// ./hola               → lo ejecuta (lo corre el intérprete)
// javac Main.java      → crea Main.class
// java Main            → lo ejecuta
// También: tcc -run hola.c, java Hola.java, run hola.cpp…

/// Lo que se guarda en un "ejecutable": el lenguaje y los fuentes.
struct CEjecutable {
    let dialecto: CDialecto
    let claseMain: String?
    let fuentes: [(String, String)]

    static let marca = "#!swiftshell "

    func serializa() -> String {
        var s = CEjecutable.marca + dialecto.rawValue + (claseMain.map { " " + $0 } ?? "") + "\n"
        for (n, src) in fuentes {
            let lineas = src.components(separatedBy: "\n")
            s += "#archivo \(lineas.count) \(n)\n" + lineas.joined(separator: "\n") + "\n"
        }
        return s
    }

    static func lee(_ texto: String) -> CEjecutable? {
        guard texto.hasPrefix(marca) else { return nil }
        var lineas = texto.components(separatedBy: "\n")
        let cab = lineas.removeFirst().dropFirst(marca.count).split(separator: " ").map(String.init)
        guard let d = cab.first.flatMap({ CDialecto(rawValue: $0) }) else { return nil }
        var fuentes: [(String, String)] = []
        var k = 0
        while k < lineas.count {
            let l = lineas[k]
            guard l.hasPrefix("#archivo ") else { k += 1; continue }
            let partes = l.split(separator: " ", maxSplits: 2).map(String.init)
            guard partes.count == 3, let n = Int(partes[1]) else { k += 1; continue }
            let fin = min(lineas.count, k + 1 + n)
            fuentes.append((partes[2], lineas[(k + 1)..<fin].joined(separator: "\n")))
            k = fin
        }
        return CEjecutable(dialecto: d, claseMain: cab.count > 1 ? cab[1] : nil, fuentes: fuentes)
    }
}

extension Shell {

    static func compiladores() -> [String: Spec] {
        var c: [String: Spec] = [:]
        for n in ["gcc", "cc", "clang", "tcc"] {
            c[n] = Spec(help: "\(n) archivo.c [-o salida] — compila C (revisa todo y crea un ejecutable; ./salida lo corre)") { ctx in
                try await ordenCompila(ctx, .c)
            }
        }
        for n in ["g++", "c++", "clang++"] {
            c[n] = Spec(help: "\(n) archivo.cpp [-o salida] — compila C++ (revisa todo y crea un ejecutable)") { ctx in
                try await ordenCompila(ctx, .cpp)
            }
        }
        c["javac"] = Spec(help: "javac Archivo.java — compila Java y crea Clase.class (java Clase lo corre)") { ctx in
            try await ordenJavac(ctx)
        }
        c["java"] = Spec(help: "java Clase [args] · java Archivo.java — ejecuta un programa de Java") { ctx in
            try await ordenJava(ctx)
        }
        c["lenguajes"] = Spec(help: "lenguajes — qué se puede programar aquí y cómo") { _ in Shell.ayudaLenguajes }
        return c
    }

    /// 'run' sabe además ejecutar programas compilados y fuentes .c/.cpp/.java
    static func envuelveRun(_ viejo: Spec?) -> Spec {
        Spec(help: viejo?.help ?? "run <archivo> — ejecuta cualquier tipo admitido") { ctx in
            guard let p = ctx.args.first else {
                if let v = viejo { return try await v.run(ctx) }
                throw ShErr("run: falta el archivo")
            }
            let u = try ctx.env.resolve(p)
            if let d = CCompilador.dialectoDe(p), !(ctx.env.isDir(u)) {
                let src = try ctx.input([p])
                return try await ejecutaPrograma(ctx, CEjecutable(dialecto: d, claseMain: nil, fuentes: [(p, src)]), Array(ctx.args.dropFirst()))
            }
            if let datos = FileManager.default.contents(atPath: u.path), datos.starts(with: Array(CEjecutable.marca.utf8)),
               let ej = CEjecutable.lee(String(decoding: datos, as: UTF8.self)) {
                return try await ejecutaPrograma(ctx, ej, Array(ctx.args.dropFirst()), nombre: p)
            }
            if (p as NSString).pathExtension.lowercased() == "out" || (p as NSString).pathExtension.isEmpty,
               ctx.env.exists(u), let datos = FileManager.default.contents(atPath: u.path), datos.starts(with: [0x7F, 0x45, 0x4C, 0x46]) {
                throw ShErr("\(p): es un ejecutable de Linux (ELF): iOS no permite ejecutar código nativo de otro sistema")
            }
            if let v = viejo { return try await v.run(ctx) }
            throw ShErr("run: no sé ejecutar \(p)")
        }
    }

    static func leeFuente(_ ctx: Ctx, _ p: String) throws -> String {
        let u = try ctx.env.resolve(p)
        guard ctx.env.exists(u) else { throw ShErr("\(ctx.name): \(p): no existe") }
        guard let d = FileManager.default.contents(atPath: u.path) else { throw ShErr("\(ctx.name): \(p): no se puede leer") }
        return String(decoding: d, as: UTF8.self)
    }

    /// Los #include "x.h" del programa, para guardarlos con él.
    static func cabecerasLocales(_ ctx: Ctx, _ src: String, _ vistos: inout Set<String>) -> [(String, String)] {
        var out: [(String, String)] = []
        for l in src.components(separatedBy: "\n") {
            let t = l.trimmingCharacters(in: .whitespaces)
            guard t.hasPrefix("#"), t.dropFirst().trimmingCharacters(in: .whitespaces).hasPrefix("include") else { continue }
            guard let i = t.firstIndex(of: "\""), let f = t[t.index(after: i)...].firstIndex(of: "\"") else { continue }
            let n = String(t[t.index(after: i)..<f])
            if vistos.contains(n) || CPreproc.cabecerasConocidas.contains(n) { continue }
            vistos.insert(n)
            guard let s = try? leeFuente(ctx, n) else { continue }
            out.append((n, s))
            out += cabecerasLocales(ctx, s, &vistos)
        }
        return out
    }

    static func ordenCompila(_ ctx: Ctx, _ d: CDialecto) async throws -> String {
        let o = opts(ctx.args, valued: ["o", "std", "I", "L", "l", "W", "O", "D", "x"])
        if o.flags.contains("version") || o.flags.contains("v") {
            return "\(ctx.name) (SwiftShell) — \(d == .c ? "C" : "C++") interpretado, sin código nativo\n"
        }
        var archivos = o.rest.filter { !$0.hasPrefix("-") }
        if o.flags.contains("run"), let k = ctx.args.firstIndex(of: "-run"), k + 1 < ctx.args.count {
            archivos = [ctx.args[k + 1]]
        }
        guard !archivos.isEmpty else { throw ShErr("\(ctx.name): falta el archivo (ej: \(ctx.name) hola.\(d == .c ? "c" : "cpp") -o hola)") }
        var fuentes: [(String, String)] = []
        var vistos: Set<String> = []
        for a in archivos {
            let src = try leeFuente(ctx, a)
            if let ej = CEjecutable.lee(src) { fuentes += ej.fuentes; continue }
            fuentes.append((a, src))
        }
        for (n, s) in fuentes { fuentes += cabecerasLocales(ctx, s, &vistos).filter { x in !fuentes.contains { $0.0 == x.0 } }; _ = n }
        let ej = CEjecutable(dialecto: d, claseMain: nil, fuentes: fuentes)
        // -run (tcc): compilar y ejecutar en un paso
        if o.flags.contains("run") {
            let k = ctx.args.firstIndex(of: "-run") ?? 0
            let resto = Array(ctx.args.dropFirst(k + 2))
            return try await ejecutaPrograma(ctx, ej, resto)
        }
        if o.flags.contains("E") {
            var out = ""
            for (n, s) in fuentes where !n.hasSuffix(".h") {
                let mapa = Dictionary(fuentes, uniquingKeysWith: { a, _ in a })
                let toks = try CPreproc(dialecto: d, leer: { mapa[$0] }).procesa(s, archivo: n)
                out += toks.filter { $0.k != .fin }.map { $0.k == .cad ? "\"\($0.t)\"" : ($0.k == .car ? "'\\\($0.v)'" : $0.t) }.joined(separator: " ") + "\n"
            }
            return out
        }
        _ = try CCompilador.compila(ej.fuentes, dialecto: d, entrada: CEntrada(""))
        let salida = o.vals["o"] ?? (o.flags.contains("c") ? ((archivos[0] as NSString).deletingPathExtension + ".o") : "a.out")
        let u = try ctx.env.resolve(salida)
        try ej.serializa().write(to: u, atomically: true, encoding: .utf8)
        return ""
    }

    static func ordenJavac(_ ctx: Ctx) async throws -> String {
        let o = opts(ctx.args, valued: ["d", "cp", "classpath", "encoding", "source", "target", "release"])
        if o.flags.contains("version") { return "javac 21 (SwiftShell, interpretado)\n" }
        let archivos = o.rest.filter { $0.hasSuffix(".java") }
        guard !archivos.isEmpty else { throw ShErr("javac: falta el archivo .java (ej: javac Main.java)") }
        var fuentes: [(String, String)] = []
        for a in archivos { fuentes.append((a, try leeFuente(ctx, a))) }
        let c = try CCompilador.compila(fuentes, dialecto: .java, entrada: CEntrada(""))
        let dir = o.vals["d"] ?? "."
        var creadas: [String] = []
        for cl in c.prog.ordenClases where !cl.nativa && c.prog.exterior[cl.nombre] == nil && !cl.nombre.hasPrefix("$") {
            let ej = CEjecutable(dialecto: .java, claseMain: cl.nombre, fuentes: fuentes)
            let u = try ctx.env.resolve(dir + "/" + cl.nombre + ".class")
            try? FileManager.default.createDirectory(at: u.deletingLastPathComponent(), withIntermediateDirectories: true)
            try ej.serializa().write(to: u, atomically: true, encoding: .utf8)
            creadas.append(cl.nombre)
        }
        _ = creadas
        return ""
    }

    static func ordenJava(_ ctx: Ctx) async throws -> String {
        var args = ctx.args
        var cp = "."
        while let f = args.first, f.hasPrefix("-") {
            args.removeFirst()
            if (f == "-cp" || f == "-classpath"), let d = args.first { cp = d; args.removeFirst() }
            if f == "-version" || f == "--version" { return "java 21 (SwiftShell, interpretado)\n" }
        }
        guard let primero = args.first else { throw ShErr("java: falta la clase (ej: java Main) o el archivo (java Main.java)") }
        let resto = Array(args.dropFirst())
        if primero.hasSuffix(".java") {
            let src = try leeFuente(ctx, primero)
            return try await ejecutaPrograma(ctx, CEjecutable(dialecto: .java, claseMain: nil, fuentes: [(primero, src)]), resto)
        }
        let nombre = primero.hasSuffix(".class") ? String(primero.dropLast(6)) : primero
        let ruta = cp + "/" + nombre + ".class"
        guard let texto = try? leeFuente(ctx, ruta), let ej = CEjecutable.lee(texto) else {
            throw ShErr("Error: no se encontró la clase \(nombre) (¿compilaste con javac \(nombre).java?)")
        }
        return try await ejecutaPrograma(ctx, CEjecutable(dialecto: .java, claseMain: nombre, fuentes: ej.fuentes), resto)
    }

    // MARK: ejecutar

    static func ejecutaPrograma(_ ctx: Ctx, _ ej: CEjecutable, _ argv: [String], nombre: String? = nil) async throws -> String {
        let consola = ctx.consola
        let pide: (() -> String?)? = ctx.tecladoVivo ? consola.map { c in { c.leeLinea().map { $0 + "\n" } } } : nil
        let entrada = CEntrada(ctx.stdin, pide: pide)
        let comp = try CCompilador.compila(ej.fuentes, dialecto: ej.dialecto, claseMain: ej.claseMain, entrada: entrada)
        let m = comp.maq
        m.nombrePrograma = nombre.map { "./" + $0 } ?? "./a.out"
        let env = ctx.env
        m.leerArchivo = { p in
            guard let u = try? env.resolve(p) else { return nil }
            return FileManager.default.contents(atPath: u.path).map { Array($0) }
        }
        m.escribirArchivo = { p, bytes, anexar in
            guard let u = try? env.resolve(p) else { return false }
            if anexar, let h = try? FileHandle(forWritingTo: u) {
                h.seekToEndOfFile()
                h.write(Data(bytes))
                h.closeFile()
                return true
            }
            return FileManager.default.createFile(atPath: u.path, contents: Data(bytes))
        }
        if let c = consola {
            m.volcar = { s in c.escribir(s) }
            m.cancelado = { c.cancelado }
            c.empieza()
        }
        if ctx.sh.rolIA != nil { m.limite = Date().addingTimeInterval(20) }
        else if consola == nil { m.limite = Date().addingTimeInterval(120) }
        let r = await enHiloGrande { CCompilador.ejecuta(comp, argv: argv) }
        consola?.termina()
        var out = m.textoSalida()
        if let e = r.error {
            if !out.isEmpty && !out.hasSuffix("\n") { out += "\n" }
            for l in e.components(separatedBy: "\n") { out += Shell.errMark + l + "\n" }
        } else if r.codigo != 0 {
            if !out.isEmpty && !out.hasSuffix("\n") { out += "\n" }
            out += Shell.errMark + "(el programa terminó con código \(r.codigo))\n"
        }
        return out
    }

    static let ayudaLenguajes = """
    Lenguajes de esta shell (todo dentro de la app, sin internet):

      C        gcc hola.c -o hola  →  ./hola       (también cc, clang, tcc -run hola.c)
      C++      g++ hola.cpp -o hola  →  ./hola     (también c++, clang++)
      Java     javac Main.java  →  java Main       (o directo: java Main.java)
      Python   python3 hola.py
      Wasm     wasm hola.wasm · wasm hola.wat · wat2wasm hola.wat
      Swift/JS swift hola.swift · js hola.js (modo seguro: limitados)
      run      run hola.c / run Main.java / run hola.py — compila y ejecuta

    Los programas pueden pedir datos (scanf, cin, Scanner, input): escribe la
    respuesta en la línea de abajo. Para pararlos: ^C.

    Interfaz (en texto):
      C/C++  #include <graphics.h>: initgraph, line, circle, rectangle, outtextxy…
             #include "ui.h": ui_ventana, ui_texto, ui_campo, ui_boton, ui_mostrar
      Java   JOptionPane.showMessageDialog / showInputDialog / showConfirmDialog
             Graficos.line(…), Graficos.circle(…), Graficos.mostrar()

    No es código nativo: iOS no deja que una app cree y ejecute código máquina
    nuevo, así que gcc/g++/javac revisan el programa completo (errores con su
    línea) y un intérprete lo ejecuta, con las reglas de cada lenguaje.
    """
}
