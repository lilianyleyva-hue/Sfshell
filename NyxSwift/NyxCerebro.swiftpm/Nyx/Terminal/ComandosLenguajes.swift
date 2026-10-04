import Foundation

// ComandosLenguajes.swift — los lenguajes de la terminal:
//   python archivo.py · python -c "print(1+1)"
//   node archivo.js   · js -e "console.log(2**10)"
//   gcc prog.c -o prog && ./prog      (también cc, clang)
//   g++ prog.cpp -o prog && ./prog    (también c++, clang++)
//   javac Main.java && java Main      (o directamente: java Main.java)
//   run archivo                        (elige el lenguaje por la extensión)
// En el iPad no hay compiladores: «compilar» es comprobar el programa entero
// (con errores como los de gcc/javac) y el «ejecutable» guarda el código, que
// luego ejecuta el intérprete de Nyx.

/// Un «ejecutable» hecho por gcc/g++/javac en la terminal de Nyx.
enum ProgramaCompilado {
    static let marca = "#!nyx-programa "

    static func crea(lenguaje: String, archivo: String, codigo: String) -> String {
        return marca + lenguaje + "\n" + archivo + "\n" + codigo
    }

    static func lee(_ t: String) -> (lenguaje: String, archivo: String, codigo: String)? {
        guard t.hasPrefix(marca) else { return nil }
        let partes = t.split(separator: "\n", maxSplits: 2, omittingEmptySubsequences: false)
        guard partes.count == 3 else { return nil }
        return (String(partes[0].dropFirst(marca.count)), String(partes[1]), String(partes[2]))
    }
}

/// El código de salida de un trabajo en segundo plano (cuando termina).
final class ResultadoTrabajo: @unchecked Sendable {
    private let candado = NSLock()
    private var r: Int? = nil
    var valor: Int? {
        candado.lock()
        defer { candado.unlock() }
        return r
    }
    func termina(_ v: Int) { candado.lock(); r = v; candado.unlock() }
}

/// La salida de un programa que corre en segundo plano, mandada a la pantalla
/// a trozos (sin inundar el hilo principal) y en orden.
final class TuboSalida: @unchecked Sendable {
    private let candado = NSLock()
    private var pendiente = ""
    private var programado = false
    private let destino: (String) -> Void

    init(_ destino: @escaping (String) -> Void) { self.destino = destino }

    func escribe(_ t: String) {
        candado.lock()
        pendiente += t
        let yaProgramado = programado
        programado = true
        candado.unlock()
        if !yaProgramado {
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.04) { self.vacia() }
        }
    }

    func vacia() {
        candado.lock()
        let t = pendiente
        pendiente = ""
        programado = false
        candado.unlock()
        if !t.isEmpty { destino(t) }
    }
}

extension Consola {
    /// Ejecuta un trabajo pesado (un intérprete) fuera del hilo principal.
    /// Si pulsas ⏹ y el programa no para (p. ej. un bucle infinito en JavaScript),
    /// la terminal queda libre igualmente.
    func enSegundoPlano(_ c: Contexto, _ trabajo: @escaping @Sendable (_ escribe: @escaping (String) -> Void, _ parada: BanderaParada) -> Int) async -> Int32 {
        let tubo = TuboSalida(c.escribe)
        let parada = c.parada
        let resultado = ResultadoTrabajo()
        Task.detached(priority: .userInitiated) {
            let r = trabajo({ tubo.escribe($0) }, parada)
            resultado.termina(r)
        }
        var esperandoTrasParar = 0
        while true {
            if let r = resultado.valor {
                tubo.vacia()
                return Int32(truncatingIfNeeded: r)
            }
            if parada.activada {
                esperandoTrasParar += 1
                if esperandoTrasParar > 20 {           // 1 s después de ⏹ y sigue: se abandona
                    tubo.vacia()
                    c.error("\n(el programa no paraba: lo dejo de lado)\n")
                    return 130
                }
            }
            try? await Task.sleep(nanoseconds: 50_000_000)
        }
    }

    private func lineasEntrada(_ c: Contexto) -> [String] {
        return c.hayEntrada ? Consola.lineas(c.entrada) : []
    }

    // MARK: python

    func cmdPython(_ c: Contexto) async -> Int32 {
        var a = c.args
        if a.first == "--version" || a.first == "-V" { c.escribe("Python 3 (intérprete de Nyx)\n"); return 0 }
        var codigo: String
        var archivo: String
        if a.first == "-c", a.count > 1 {
            codigo = a[1]
            archivo = "<string>"
            a.removeFirst(2)
        } else if let f = a.first {
            guard let t = lee(f) else { c.error("python: no se puede abrir '\(f)': no existe el archivo\n"); return 2 }
            codigo = t
            archivo = f
            a.removeFirst()
        } else if c.hayEntrada {
            codigo = c.entrada
            archivo = "<stdin>"
        } else {
            c.escribe("Python 3 de Nyx. Escribe un programa con «nano hola.py» y ejecútalo con «python hola.py»,\no prueba: python -c \"print('hola')\"\n")
            return 0
        }
        let args = a
        let entrada = archivo == "<stdin>" ? [] : lineasEntrada(c)
        let src = codigo
        let nombre = archivo
        return await enSegundoPlano(c) { escribe, parada in
            Python.ejecuta(src, archivo: nombre, argumentos: args, entrada: entrada, escribe: escribe,
                           cancelado: { parada.activada })
        }
    }

    func cmdPip(_ c: Contexto) -> Int32 {
        c.escribe("En el iPad no se pueden instalar paquetes de Python. Vienen incluidos: math, random, time y sys.\n")
        return c.args.first == "install" ? 1 : 0
    }

    // MARK: javascript

    func cmdNode(_ c: Contexto) async -> Int32 {
        var a = c.args
        if a.first == "--version" || a.first == "-v" { c.escribe("JavaScript (JavaScriptCore, el motor de Safari)\n"); return 0 }
        var codigo: String
        var archivo: String
        if (a.first == "-e" || a.first == "-p"), a.count > 1 {
            codigo = a.first == "-p" ? "console.log(\(a[1]))" : a[1]
            archivo = "[eval]"
            a.removeFirst(2)
        } else if let f = a.first {
            guard let t = lee(f) else { c.error("node: no se encuentra '\(f)'\n"); return 1 }
            codigo = t
            archivo = f
            a.removeFirst()
        } else if c.hayEntrada {
            codigo = c.entrada
            archivo = "[stdin]"
        } else {
            c.escribe("JavaScript de Nyx (JavaScriptCore). Escribe un programa con «nano hola.js» y ejecútalo con «node hola.js»,\no prueba: node -e \"console.log('hola')\"\n")
            return 0
        }
        let args = a
        let entrada = archivo == "[stdin]" ? [] : lineasEntrada(c)
        let src = codigo
        let nombre = archivo
        return await enSegundoPlano(c) { escribe, parada in
            JavaScriptNyx.ejecuta(src, archivo: nombre, argumentos: args, entrada: entrada, escribe: escribe)
        }
    }

    // MARK: C, C++ y Java

    /// gcc / g++: comprueba el programa y crea el «ejecutable» (a.out o -o nombre).
    func cmdCompilaC(_ c: Contexto, lenguaje: LenguajeC) -> Int32 {
        let (op, resto) = c.opciones(["o", "O", "l", "I", "W", "s"])
        let fuentes = resto.filter { !$0.hasPrefix("-") }
        guard let f = fuentes.first else { c.error("\(c.nombre): error fatal: no hay archivos de entrada\n"); return 1 }
        guard let codigo = lee(f) else { c.error("\(c.nombre): error: \(f): no existe el archivo\n"); return 1 }
        var leng = lenguaje
        let ext = (f as NSString).pathExtension.lowercased()
        if ["cpp", "cc", "cxx", "c++", "hpp"].contains(ext) { leng = .cpp } else if ext == "c" && lenguaje != .cpp { leng = .c }
        if let errores = CFamilia.compila(codigo, lenguaje: leng, archivo: f) {
            c.error(errores.hasSuffix("\n") ? errores : errores + "\n")
            return 1
        }
        let salida = op["o"] ?? "a.out"
        escribeArchivo(salida, ProgramaCompilado.crea(lenguaje: leng == .cpp ? "c++" : "c", archivo: f, codigo: codigo))
        return 0
    }

    /// javac Main.java → Main.class (por cada clase con main).
    func cmdJavac(_ c: Contexto) -> Int32 {
        let fuentes = c.args.filter { $0.hasSuffix(".java") }
        guard !fuentes.isEmpty else { c.error("javac: no hay archivos .java\n"); return 2 }
        for f in fuentes {
            guard let codigo = lee(f) else { c.error("javac: archivo no encontrado: \(f)\n"); return 2 }
            if let errores = CFamilia.compila(codigo, lenguaje: .java, archivo: f) {
                c.error(errores.hasSuffix("\n") ? errores : errores + "\n")
                return 1
            }
            let carpeta = (absoluta(f) as NSString).deletingLastPathComponent
            for clase in Consola.clasesJava(codigo) {
                escribeArchivo(carpeta + "/" + clase + ".class", ProgramaCompilado.crea(lenguaje: "java", archivo: f, codigo: codigo))
            }
        }
        return 0
    }

    static func clasesJava(_ codigo: String) -> [String] {
        guard let re = try? NSRegularExpression(pattern: "\\bclass\\s+([A-Za-z_][A-Za-z0-9_]*)") else { return [] }
        let ns = codigo as NSString
        return re.matches(in: codigo, range: NSRange(location: 0, length: ns.length)).map { ns.substring(with: $0.range(at: 1)) }
    }

    /// java Main (usa Main.class) · java Main.java (sin compilar antes)
    func cmdJava(_ c: Contexto) async -> Int32 {
        var a = c.args
        if a.first == "-version" || a.first == "--version" { c.escribe("openjdk de Nyx (intérprete de Java)\n"); return 0 }
        if a.first == "-cp" || a.first == "-classpath" { a.removeFirst(min(2, a.count)) }
        guard let f = a.first else { c.error("uso: java Clase  |  java Archivo.java\n"); return 1 }
        var codigo: String
        var archivo = f
        if f.hasSuffix(".java") {
            guard let t = lee(f) else { c.error("error: no se encuentra el archivo \(f)\n"); return 1 }
            if let errores = CFamilia.compila(t, lenguaje: .java, archivo: f) { c.error(errores + "\n"); return 1 }
            codigo = t
        } else {
            guard let t = lee(f + ".class"), let p = ProgramaCompilado.lee(t) else {
                c.error("Error: no se ha encontrado la clase principal \(f) (¿hiciste «javac \(f).java»?)\n")
                return 1
            }
            codigo = p.codigo
            archivo = p.archivo
        }
        a.removeFirst()
        return await corre(c, lenguaje: "java", archivo: archivo, codigo: codigo, args: a)
    }

    /// Ejecuta un programa hecho por gcc/g++/javac (./a.out).
    func corre(_ c: Contexto, lenguaje: String, archivo: String, codigo: String, args: [String]) async -> Int32 {
        let leng: LenguajeC = lenguaje == "java" ? .java : (lenguaje == "c++" ? .cpp : .c)
        let entrada = lineasEntrada(c)
        return await enSegundoPlano(c) { escribe, parada in
            CFamilia.ejecuta(codigo, lenguaje: leng, archivo: archivo, argumentos: args, entrada: entrada,
                             escribe: escribe, cancelado: { parada.activada })
        }
    }

    /// ./programa, ./script.sh, ./hola.py…
    func cmdEjecutable(_ c: Contexto) async -> Int32 {
        let f = c.nombre
        guard existe(f), !esCarpeta(f), let t = lee(f) else {
            c.error("nyxsh: \(f): \(esCarpeta(f) ? "es una carpeta" : "no existe el archivo")\n")
            return 127
        }
        if let p = ProgramaCompilado.lee(t) {
            return await corre(c, lenguaje: p.lenguaje, archivo: p.archivo, codigo: p.codigo, args: c.args)
        }
        var c2 = c
        c2.args = [f] + c.args
        return await porExtension(c2, archivo: f, texto: t)
    }

    /// run archivo: elige el lenguaje por la extensión (o la primera línea #!).
    func cmdRun(_ c: Contexto) async -> Int32 {
        guard let f = c.args.first else { c.error("run: falta el archivo (run hola.py)\n"); return 1 }
        guard let t = lee(f) else { c.error("run: \(f): no existe\n"); return 1 }
        return await porExtension(c, archivo: f, texto: t)
    }

    private func porExtension(_ c: Contexto, archivo f: String, texto t: String) async -> Int32 {
        let ext = (f as NSString).pathExtension.lowercased()
        let primera = t.split(separator: "\n").first.map(String.init) ?? ""
        var c2 = c
        if ext == "py" || primera.contains("python") { return await cmdPython(c) }
        if ext == "js" || primera.contains("node") { return await cmdNode(c) }
        if ext == "sh" || primera.hasPrefix("#!") { return await cmdSh(c) }
        if ext == "java" { return await cmdJava(c) }
        if ["c", "cpp", "cc", "cxx"].contains(ext) {
            let leng: LenguajeC = ext == "c" ? .c : .cpp
            if let errores = CFamilia.compila(t, lenguaje: leng, archivo: f) { c.error(errores + "\n"); return 1 }
            c2.args = Array(c.args.dropFirst())
            return await corre(c2, lenguaje: leng == .c ? "c" : "c++", archivo: f, codigo: t, args: c2.args)
        }
        c.error("nyxsh: \(f): no sé ejecutar este tipo de archivo (.\(ext))\n")
        return 126
    }
}
