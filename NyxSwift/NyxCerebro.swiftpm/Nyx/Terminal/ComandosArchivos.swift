import Foundation

// ComandosArchivos.swift — órdenes para carpetas y archivos:
// ls cd pwd mkdir rmdir rm cp mv touch cat tree find du df stat file
// basename dirname realpath ln(no) chmod(sin efecto)

extension Consola {
    func cmdPwd(_ c: Contexto) -> Int32 {
        c.escribe(cwd + "\n")
        return 0
    }

    func cmdCd(_ c: Contexto) -> Int32 {
        var destino = c.args.first ?? "~"
        if destino == "-" { destino = entorno["OLDPWD"] ?? cwd }
        let a = absoluta(destino)
        guard esCarpeta(a) else {
            c.error("cd: \(destino): \(existe(a) ? "no es una carpeta" : "no existe la carpeta")\n")
            return 1
        }
        entorno["OLDPWD"] = cwd
        cwd = a
        entorno["PWD"] = a
        return 0
    }

    func cmdLs(_ c: Contexto) -> Int32 {
        let (op, resto) = c.opciones()
        let largo = op["l"] != nil
        let todos = op["a"] != nil || op["A"] != nil
        let unoPorLinea = op["1"] != nil || largo || !c.aPantalla
        let humano = op["h"] != nil
        let rutas = resto.isEmpty ? ["."] : resto
        var codigo: Int32 = 0
        for (k, r) in rutas.enumerated() {
            guard existe(r) else {
                c.error("ls: no se puede acceder a '\(r)': no existe\n")
                codigo = 2
                continue
            }
            if !esCarpeta(r) {
                c.escribe((largo ? lineaLarga(r, nombre: r, humano: humano) : r) + "\n")
                continue
            }
            if rutas.count > 1 { c.escribe((k > 0 ? "\n" : "") + r + ":\n") }
            var nombres = lista(r).filter { todos || !$0.hasPrefix(".") }
            if op["t"] != nil { nombres.sort { fecha(r + "/" + $0) > fecha(r + "/" + $1) } }
            if op["r"] != nil { nombres.reverse() }
            if largo {
                c.escribe("total \(nombres.count)\n")
                for n in nombres { c.escribe(lineaLarga(r + "/" + n, nombre: n, humano: humano) + "\n") }
            } else if unoPorLinea {
                for n in nombres { c.escribe((c.aPantalla ? conBarra(r, n) : n) + "\n") }
            } else if !nombres.isEmpty {
                c.escribe(nombres.map { conBarra(r, $0) }.joined(separator: "  ") + "\n")
            }
        }
        return codigo
    }

    private func conBarra(_ carpeta: String, _ n: String) -> String {
        return esCarpeta(carpeta + "/" + n) ? n + "/" : n
    }

    private func atributos(_ r: String) -> [FileAttributeKey: Any] {
        return (try? FileManager.default.attributesOfItem(atPath: url(r).path)) ?? [:]
    }

    func tamaño(_ r: String) -> Int {
        return (atributos(r)[.size] as? NSNumber)?.intValue ?? 0
    }

    func fecha(_ r: String) -> Date {
        return (atributos(r)[.modificationDate] as? Date) ?? Date.distantPast
    }

    static func humano(_ n: Int) -> String {
        let u = ["B", "K", "M", "G"]
        var v = Double(n)
        var i = 0
        while v >= 1024 && i < u.count - 1 { v /= 1024; i += 1 }
        return i == 0 ? "\(n)" : String(format: v < 10 ? "%.1f%@" : "%.0f%@", v, u[i])
    }

    private func lineaLarga(_ r: String, nombre: String, humano h: Bool) -> String {
        let carpeta = esCarpeta(r)
        let permisos = carpeta ? "drwxr-xr-x" : (nombre.hasSuffix(".sh") || nombre.hasSuffix(".out") ? "-rwxr-xr-x" : "-rw-r--r--")
        let t = tamaño(r)
        let tam = h ? Consola.humano(t) : String(t)
        let f = DateFormatter()
        f.dateFormat = "MMM d HH:mm"
        f.locale = Locale(identifier: "es_ES")
        let relleno = String(repeating: " ", count: max(0, 8 - tam.count))
        return "\(permisos) 1 nyx nyx \(relleno)\(tam) \(f.string(from: fecha(r))) \(carpeta ? nombre + "/" : nombre)"
    }

    func cmdMkdir(_ c: Contexto) -> Int32 {
        let (op, resto) = c.opciones()
        if resto.isEmpty { c.error("mkdir: falta el nombre de la carpeta\n"); return 1 }
        var codigo: Int32 = 0
        for r in resto {
            if existe(r) && op["p"] == nil { c.error("mkdir: no se puede crear '\(r)': ya existe\n"); codigo = 1; continue }
            do {
                try FileManager.default.createDirectory(at: url(r), withIntermediateDirectories: op["p"] != nil || true)
            } catch {
                c.error("mkdir: no se puede crear '\(r)'\n")
                codigo = 1
            }
        }
        return codigo
    }

    func cmdRmdir(_ c: Contexto) -> Int32 {
        var codigo: Int32 = 0
        for r in c.args {
            if !esCarpeta(r) { c.error("rmdir: '\(r)': no es una carpeta\n"); codigo = 1; continue }
            if !lista(r).isEmpty { c.error("rmdir: '\(r)': la carpeta no está vacía\n"); codigo = 1; continue }
            try? FileManager.default.removeItem(at: url(r))
        }
        return codigo
    }

    func cmdRm(_ c: Contexto) -> Int32 {
        let (op, resto) = c.opciones()
        let recursivo = op["r"] != nil || op["R"] != nil
        let forzar = op["f"] != nil
        if resto.isEmpty && !forzar { c.error("rm: falta el archivo\n"); return 1 }
        var codigo: Int32 = 0
        for r in resto {
            let a = absoluta(r)
            if a == "/" || a == "/home" || a == "/home/nyx" { c.error("rm: no voy a borrar '\(r)' (es una carpeta principal)\n"); codigo = 1; continue }
            if !existe(r) { if !forzar { c.error("rm: '\(r)': no existe\n"); codigo = 1 }; continue }
            if esCarpeta(r) && !recursivo { c.error("rm: '\(r)': es una carpeta (usa rm -r)\n"); codigo = 1; continue }
            try? FileManager.default.removeItem(at: url(r))
        }
        return codigo
    }

    func cmdTouch(_ c: Contexto) -> Int32 {
        if c.args.isEmpty { c.error("touch: falta el archivo\n"); return 1 }
        for r in c.args {
            if existe(r) {
                try? FileManager.default.setAttributes([.modificationDate: Date()], ofItemAtPath: url(r).path)
            } else if !escribeArchivo(r, "") {
                c.error("touch: no se puede crear '\(r)'\n")
                return 1
            }
        }
        return 0
    }

    /// cp y mv: origen(es) → destino (si el destino es una carpeta, dentro).
    func cmdCopiaMueve(_ c: Contexto, mover: Bool) -> Int32 {
        let (op, resto) = c.opciones()
        let n = mover ? "mv" : "cp"
        guard resto.count >= 2 else { c.error("\(n): faltan el origen y el destino\n"); return 1 }
        let destino = resto.last!
        let origenes = resto.dropLast()
        if origenes.count > 1 && !esCarpeta(destino) { c.error("\(n): '\(destino)' no es una carpeta\n"); return 1 }
        for o in origenes {
            guard existe(o) else { c.error("\(n): '\(o)': no existe\n"); return 1 }
            if esCarpeta(o) && !mover && op["r"] == nil && op["R"] == nil { c.error("cp: '\(o)' es una carpeta (usa cp -r)\n"); return 1 }
            let nombre = absoluta(o).split(separator: "/").last.map(String.init) ?? o
            let final = esCarpeta(destino) ? destino + "/" + nombre : destino
            if absoluta(final) == absoluta(o) { continue }
            try? FileManager.default.removeItem(at: url(final))
            do {
                if mover { try FileManager.default.moveItem(at: url(o), to: url(final)) }
                else { try FileManager.default.copyItem(at: url(o), to: url(final)) }
            } catch {
                c.error("\(n): no se pudo con '\(o)'\n")
                return 1
            }
        }
        return 0
    }

    func cmdTree(_ c: Contexto) -> Int32 {
        let r = c.opciones().resto.first ?? "."
        guard esCarpeta(r) else { c.error("tree: '\(r)': no es una carpeta\n"); return 1 }
        c.escribe(r + "\n")
        var carpetas = 0
        var archivos = 0
        func rama(_ ruta: String, _ sangria: String) {
            let hijos = lista(ruta).filter { !$0.hasPrefix(".") }
            for (k, h) in hijos.enumerated() {
                let ultimo = k == hijos.count - 1
                let p = ruta + "/" + h
                c.escribe(sangria + (ultimo ? "└── " : "├── ") + h + "\n")
                if esCarpeta(p) { carpetas += 1; rama(p, sangria + (ultimo ? "    " : "│   ")) } else { archivos += 1 }
            }
        }
        rama(r, "")
        c.escribe("\n\(carpetas) carpetas, \(archivos) archivos\n")
        return 0
    }

    /// Todos los caminos bajo una carpeta.
    func recorre(_ r: String) -> [String] {
        var out: [String] = []
        var cola = [r]
        while !cola.isEmpty {
            let x = cola.removeFirst()
            for h in lista(x) {
                let p = (x == "/" ? "" : x) + "/" + h
                let rel = x == "." ? "./" + h : p
                out.append(rel)
                if esCarpeta(rel) { cola.append(rel) }
            }
        }
        return out
    }

    func cmdFind(_ c: Contexto) -> Int32 {
        var raizBusqueda = "."
        var nombre: NSRegularExpression? = nil
        var tipo: String? = nil
        var i = 0
        let a = c.args
        if let p = a.first, !p.hasPrefix("-") { raizBusqueda = p; i = 1 }
        while i < a.count {
            if a[i] == "-name" || a[i] == "-iname", i + 1 < a.count {
                nombre = Consola.regexComodin(a[i + 1])
                i += 2
            } else if a[i] == "-type", i + 1 < a.count {
                tipo = a[i + 1]
                i += 2
            } else {
                i += 1
            }
        }
        guard existe(raizBusqueda) else { c.error("find: '\(raizBusqueda)': no existe\n"); return 1 }
        for p in [raizBusqueda] + recorre(raizBusqueda) {
            let n = p.split(separator: "/").last.map(String.init) ?? p
            if let re = nombre, re.firstMatch(in: n, range: NSRange(n.startIndex..., in: n)) == nil { continue }
            if tipo == "f" && esCarpeta(p) { continue }
            if tipo == "d" && !esCarpeta(p) { continue }
            c.escribe(p + "\n")
        }
        return 0
    }

    func cmdDu(_ c: Contexto) -> Int32 {
        let (op, resto) = c.opciones()
        let h = op["h"] != nil
        for r in resto.isEmpty ? ["."] : resto {
            var total = esCarpeta(r) ? 0 : tamaño(r)
            if esCarpeta(r) { for p in recorre(r) where !esCarpeta(p) { total += tamaño(p) } }
            c.escribe((h ? Consola.humano(total) : String((total + 1023) / 1024)) + "\t" + r + "\n")
        }
        return 0
    }

    func cmdDf(_ c: Contexto) -> Int32 {
        let a = (try? FileManager.default.attributesOfFileSystem(forPath: raiz.path)) ?? [:]
        let total = (a[.systemSize] as? NSNumber)?.intValue ?? 0
        let libre = (a[.systemFreeSize] as? NSNumber)?.intValue ?? 0
        let usado = total - libre
        let pct = total > 0 ? usado * 100 / total : 0
        c.escribe("S.ficheros   Tamaño  Usado  Disp  Uso%  Montado en\n")
        c.escribe("nyxdisco     \(Consola.humano(total))   \(Consola.humano(usado))  \(Consola.humano(libre))  \(pct)%   /\n")
        return 0
    }

    func cmdStat(_ c: Contexto) -> Int32 {
        for r in c.args {
            guard existe(r) else { c.error("stat: '\(r)': no existe\n"); return 1 }
            c.escribe("  Archivo: \(r)\n  Tamaño: \(tamaño(r))\tTipo: \(esCarpeta(r) ? "carpeta" : "archivo normal")\nModificado: \(fecha(r))\n")
        }
        return 0
    }

    func cmdFile(_ c: Contexto) -> Int32 {
        for r in c.args {
            guard existe(r) else { c.error("file: '\(r)': no existe\n"); return 1 }
            c.escribe("\(r): \(tipoDeArchivo(r))\n")
        }
        return 0
    }

    func tipoDeArchivo(_ r: String) -> String {
        if esCarpeta(r) { return "directory" }
        if let t = lee(r), t.hasPrefix(ProgramaCompilado.marca) { return "programa compilado por nyx (\(t.split(separator: "\n").first.map { String($0.dropFirst(ProgramaCompilado.marca.count)) } ?? ""))" }
        let ext = (r as NSString).pathExtension.lowercased()
        let tipos: [String: String] = ["py": "Python script, UTF-8 text", "js": "JavaScript source, UTF-8 text",
                                       "c": "C source, UTF-8 text", "cpp": "C++ source, UTF-8 text", "java": "Java source, UTF-8 text",
                                       "sh": "shell script, UTF-8 text", "txt": "UTF-8 text", "md": "Markdown text",
                                       "json": "JSON data", "html": "HTML document", "png": "PNG image", "jpg": "JPEG image"]
        if let t = tipos[ext] { return t }
        if tamaño(r) == 0 { return "empty" }
        return lee(r) != nil ? "UTF-8 text" : "data"
    }

    func cmdBasename(_ c: Contexto) -> Int32 {
        guard let r = c.args.first else { c.error("basename: falta el camino\n"); return 1 }
        var b = r.split(separator: "/").last.map(String.init) ?? r
        if c.args.count > 1, b.hasSuffix(c.args[1]) { b = String(b.dropLast(c.args[1].count)) }
        c.escribe(b + "\n")
        return 0
    }

    func cmdDirname(_ c: Contexto) -> Int32 {
        guard let r = c.args.first else { c.error("dirname: falta el camino\n"); return 1 }
        let partes = r.split(separator: "/")
        let d = partes.dropLast().joined(separator: "/")
        c.escribe((d.isEmpty ? (r.hasPrefix("/") ? "/" : ".") : (r.hasPrefix("/") ? "/" + d : d)) + "\n")
        return 0
    }

    func cmdRealpath(_ c: Contexto) -> Int32 {
        for r in c.args.isEmpty ? ["."] : c.args { c.escribe(absoluta(r) + "\n") }
        return 0
    }
}
