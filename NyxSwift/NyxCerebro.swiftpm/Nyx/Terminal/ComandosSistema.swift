import Foundation
#if canImport(FoundationNetworking)
import FoundationNetworking
#endif

// ComandosSistema.swift — órdenes del sistema, de la red y de Nyx:
// clear history alias unalias export env unset which type whoami hostname uname
// uptime date cal sleep expr calc bc true false test [ curl wget sha256sum
// pkg apt neofetch ps exit time nano edit sh source xargs man nyx

extension Consola {
    func cmdHistory(_ c: Contexto) -> Int32 {
        if c.args.first == "-c" { historial.removeAll(); return 0 }
        for (k, h) in historial.enumerated() { c.escribe(String(repeating: " ", count: max(0, 5 - String(k + 1).count)) + "\(k + 1)  \(h)\n") }
        return 0
    }

    func borraHistorial() { historial.removeAll() }

    func cmdAlias(_ c: Contexto) -> Int32 {
        if c.args.isEmpty {
            for (k, v) in alias.sorted(by: { $0.key < $1.key }) { c.escribe("alias \(k)='\(v)'\n") }
            return 0
        }
        for a in c.args {
            if let i = a.firstIndex(of: "=") { alias[String(a[..<i])] = String(a[a.index(after: i)...]) }
            else if let v = alias[a] { c.escribe("alias \(a)='\(v)'\n") }
            else { c.error("alias: \(a): no existe\n"); return 1 }
        }
        return 0
    }

    func cmdExport(_ c: Contexto) -> Int32 {
        if c.args.isEmpty { return cmdEnv(c) }
        for a in c.args where a.contains("=") { asigna(a) }
        return 0
    }

    func cmdEnv(_ c: Contexto) -> Int32 {
        for (k, v) in entorno.sorted(by: { $0.key < $1.key }) { c.escribe("\(k)=\(v)\n") }
        return 0
    }

    func cmdUname(_ c: Contexto) -> Int32 {
        let (op, _) = c.opciones()
        c.escribe(op["a"] != nil ? "NyxOS 2.0 ipad nyxsh arm64 (iPadOS, dentro de Swift Playgrounds)\n" : "NyxOS\n")
        return 0
    }

    func cmdUptime(_ c: Contexto) -> Int32 {
        let s = Int(Date().timeIntervalSince(inicio))
        c.escribe("encendida desde hace \(s / 3600) h \(s % 3600 / 60) min \(s % 60) s\n")
        return 0
    }

    func cmdDate(_ c: Contexto) -> Int32 {
        let f = DateFormatter()
        f.locale = Locale(identifier: "es_ES")
        if let fmt = c.args.first, fmt.hasPrefix("+") {
            var p = String(fmt.dropFirst())
            for (a, b) in [("%Y", "yyyy"), ("%m", "MM"), ("%d", "dd"), ("%H", "HH"), ("%M", "mm"), ("%S", "ss"), ("%A", "EEEE"), ("%B", "MMMM")] {
                p = p.replacingOccurrences(of: a, with: b)
            }
            f.dateFormat = p
        } else {
            f.dateFormat = "EEE d MMM yyyy, HH:mm:ss zzz"
        }
        c.escribe(f.string(from: Date()) + "\n")
        return 0
    }

    func cmdCal(_ c: Contexto) -> Int32 {
        let cal = Calendar(identifier: .gregorian)
        let hoy = Date()
        let comp = cal.dateComponents([.year, .month, .day], from: hoy)
        guard let primero = cal.date(from: DateComponents(year: comp.year, month: comp.month, day: 1)),
              let dias = cal.range(of: .day, in: .month, for: primero) else { return 1 }
        let f = DateFormatter()
        f.locale = Locale(identifier: "es_ES")
        f.dateFormat = "MMMM yyyy"
        let titulo = f.string(from: hoy)
        c.escribe(String(repeating: " ", count: max(0, (20 - titulo.count) / 2)) + titulo + "\n")
        c.escribe("lu ma mi ju vi sá do\n")
        let semana = (cal.component(.weekday, from: primero) + 5) % 7      // lunes = 0
        var linea = String(repeating: "   ", count: semana)
        var col = semana
        for d in dias {
            let s = d < 10 ? " \(d)" : "\(d)"
            linea += (d == comp.day ? s : s) + " "
            col += 1
            if col == 7 { c.escribe(linea + "\n"); linea = ""; col = 0 }
        }
        if !linea.isEmpty { c.escribe(linea + "\n") }
        return 0
    }

    func cmdSleep(_ c: Contexto) async -> Int32 {
        let s = min(60, Double(c.args.first ?? "1") ?? 1)
        var hecho = 0.0
        while hecho < s && !c.parada.activada {
            try? await Task.sleep(nanoseconds: 100_000_000)
            hecho += 0.1
        }
        return c.parada.activada ? 130 : 0
    }

    /// expr 2 + 3 · calc 2^10 * (3+4) · bc (lee la entrada)
    func cmdCalc(_ c: Contexto) -> Int32 {
        var t = c.args.joined(separator: " ")
        if t.isEmpty { t = c.entrada }
        var codigo: Int32 = 0
        for linea in t.components(separatedBy: "\n") where !linea.trimmingCharacters(in: .whitespaces).isEmpty {
            let e = linea.replacingOccurrences(of: "\\*", with: "*")
            if let r = Aritmetica.resultado(e) {
                c.escribe(r + "\n")
            } else {
                c.error("\(c.nombre): no entiendo «\(linea)»\n")
                codigo = 1
            }
        }
        return codigo
    }

    /// test / [ ]: -f -d -e -z -n, = != y -eq -ne -lt -le -gt -ge
    func cmdTest(_ c: Contexto) -> Int32 {
        var a = c.args
        if c.nombre == "[" { guard a.last == "]" else { c.error("[: falta ]\n"); return 2 }; a.removeLast() }
        var niega = false
        if a.first == "!" { niega = true; a.removeFirst() }
        var r = false
        if a.count == 2 {
            switch a[0] {
            case "-f": r = existe(a[1]) && !esCarpeta(a[1])
            case "-d": r = esCarpeta(a[1])
            case "-e": r = existe(a[1])
            case "-z": r = a[1].isEmpty
            case "-n": r = !a[1].isEmpty
            default: r = false
            }
        } else if a.count == 3 {
            let x = Int(a[0]), y = Int(a[2])
            switch a[1] {
            case "=", "==": r = a[0] == a[2]
            case "!=": r = a[0] != a[2]
            case "-eq": r = x != nil && x == y
            case "-ne": r = x != y
            case "-lt": r = (x ?? 0) < (y ?? 0)
            case "-le": r = (x ?? 0) <= (y ?? 0)
            case "-gt": r = (x ?? 0) > (y ?? 0)
            case "-ge": r = (x ?? 0) >= (y ?? 0)
            default: r = false
            }
        } else if a.count == 1 {
            r = !a[0].isEmpty
        }
        return (r != niega) ? 0 : 1
    }

    func cmdWhich(_ c: Contexto) -> Int32 {
        var codigo: Int32 = 0
        for n in c.args {
            if let a = alias[n] { c.escribe("\(n): alias de «\(a)»\n") }
            else if Consola.ordenes.contains(n) { c.escribe("/bin/\(n)\n") }
            else { c.error("\(n): no encontrado\n"); codigo = 1 }
        }
        return codigo
    }

    // MARK: red

    /// curl [-o archivo] [-s] [-I] URL   ·   wget [-O archivo] URL
    func cmdCurl(_ c: Contexto) async -> Int32 {
        let (op, resto) = c.opciones(["o", "O", "X", "d", "H"])
        guard var dir = resto.first else { c.error("\(c.nombre): falta la dirección\n"); return 1 }
        if !dir.contains("://") { dir = "https://" + dir }
        guard let u = URL(string: dir) else { c.error("\(c.nombre): dirección no válida\n"); return 1 }
        var peticion = URLRequest(url: u)
        peticion.timeoutInterval = 30
        peticion.setValue(Lector.navegador, forHTTPHeaderField: "User-Agent")
        if let m = op["X"] { peticion.httpMethod = m }
        if let d = op["d"] { peticion.httpMethod = peticion.httpMethod == "GET" ? "POST" : peticion.httpMethod; peticion.httpBody = Data(d.utf8) }
        if let h = op["H"], let i = h.firstIndex(of: ":") {
            peticion.setValue(String(h[h.index(after: i)...]).trimmingCharacters(in: .whitespaces), forHTTPHeaderField: String(h[..<i]))
        }
        let respuesta: (Data, URLResponse)
        do {
            respuesta = try await withCheckedThrowingContinuation { (k: CheckedContinuation<(Data, URLResponse), Error>) in
                URLSession.shared.dataTask(with: peticion) { d, r, e in
                    if let d = d, let r = r { k.resume(returning: (d, r)) } else { k.resume(throwing: e ?? Internet.Fallo.sinRed) }
                }.resume()
            }
        } catch {
            c.error("\(c.nombre): no se pudo conectar con \(u.host ?? dir): \(error.localizedDescription)\n")
            return 6
        }
        let (datos, r) = respuesta
        let http = r as? HTTPURLResponse
        if op["I"] != nil, let h = http {
            c.escribe("HTTP/1.1 \(h.statusCode)\n")
            for (k, v) in h.allHeaderFields { c.escribe("\(k): \(v)\n") }
            return 0
        }
        let wget = c.nombre == "wget"
        let destino = op["o"] ?? op["O"] ?? (wget ? (u.lastPathComponent.isEmpty || u.lastPathComponent == "/" ? "index.html" : u.lastPathComponent) : nil)
        if let f = destino {
            do { try datos.write(to: url(f)) } catch { c.error("\(c.nombre): no se puede escribir \(f)\n"); return 23 }
            if op["s"] == nil { c.error("guardado «\(f)» (\(Consola.humano(datos.count)))\n") }
        } else {
            c.escribe(String(data: datos, encoding: .utf8) ?? String(data: datos, encoding: .isoLatin1) ?? "(datos binarios: \(datos.count) bytes; usa -o archivo)\n")
        }
        if let h = http, h.statusCode >= 400 { return 22 }
        return 0
    }

    func cmdSha256(_ c: Contexto) -> Int32 {
        if c.args.isEmpty {
            c.escribe(SHA256Nyx.hex(Data(c.entrada.utf8)) + "  -\n")
            return 0
        }
        for f in c.args {
            guard let d = try? Data(contentsOf: url(f)) else { c.error("sha256sum: \(f): no existe\n"); return 1 }
            c.escribe(SHA256Nyx.hex(d) + "  \(f)\n")
        }
        return 0
    }

    // MARK: paquetes y presentación

    func cmdPkg(_ c: Contexto) -> Int32 {
        let paquetes = ["python 3 (intérprete de Nyx)", "nodejs (JavaScriptCore)", "clang/gcc (C, intérprete de Nyx)",
                        "g++ (C++, intérprete de Nyx)", "openjdk (Java, intérprete de Nyx)", "coreutils", "grep", "sed",
                        "curl", "wget", "nano (editor de Nyx)", "nyx"]
        switch c.args.first ?? "list" {
        case "install", "add":
            for p in c.args.dropFirst() { c.escribe("\(p) ya viene con Nyx (no hace falta instalarlo). En el iPad no se pueden instalar programas nuevos.\n") }
        case "list", "list-installed":
            for p in paquetes { c.escribe(p + " [instalado]\n") }
        case "update", "upgrade":
            c.escribe("Todo está al día.\n")
        default:
            c.escribe("uso: \(c.nombre) list | install <paquete>\n")
        }
        return 0
    }

    func cmdNeofetch(_ c: Contexto) -> Int32 {
        let s = Int(Date().timeIntervalSince(inicio))
        let estado = cerebro?.terminalEstado() ?? "—"
        let arte = ["   ✦     ✧   ", "  ╭───────╮  ", "  │ N Y X │  ", "  ╰───────╯  ", "   ✧     ✦   "]
        let datos = ["nyx@ipad", "────────", "SO: NyxOS 2.0 (iPadOS)", "Shell: nyxsh", "Terminal: Nyx",
                     "Lenguajes: python, js, c, c++, java", "Encendida: \(s / 60) min", "Mente: \(estado)"]
        for k in 0 ..< max(arte.count, datos.count) {
            c.escribe((k < arte.count ? arte[k] : String(repeating: " ", count: 13)) + "  " + (k < datos.count ? datos[k] : "") + "\n")
        }
        return 0
    }

    func cmdPs(_ c: Contexto) -> Int32 {
        c.escribe("  PID TTY          TIME CMD\n 4242 pts/0    00:00:00 nyxsh\n 4243 pts/0    00:00:00 nyx (una sola mente)\n")
        return 0
    }

    /// time orden…: cuánto tarda.
    func cmdTime(_ c: Contexto) async -> Int32 {
        let t0 = Date()
        let orden = c.args.map { $0.contains(" ") ? "'\($0)'" : $0 }.joined(separator: " ")
        let codigo = await ejecutaTexto(orden, entrada: c.hayEntrada ? c.entrada : nil, captura: nil)
        c.error(String(format: "\nreal\t%.3fs\n", Date().timeIntervalSince(t0)))
        return codigo
    }

    /// sh guion.sh / source guion.sh / ./guion.sh
    func cmdSh(_ c: Contexto) async -> Int32 {
        if c.args.first == "-c", c.args.count > 1 {
            return await ejecutaTexto(c.args[1], entrada: c.entrada, captura: nil)
        }
        guard let f = c.args.first else { return await ejecutaTexto(c.entrada, entrada: nil, captura: nil) }
        guard let t = lee(f) else { c.error("\(c.nombre): \(f): no existe\n"); return 127 }
        let guardados = c.args.dropFirst().enumerated().map { ("\($0.offset + 1)", $0.element) }
        for (k, v) in guardados { entorno[k] = v }
        var codigo: Int32 = 0
        for linea in t.components(separatedBy: "\n") {
            let l = linea.trimmingCharacters(in: .whitespaces)
            if l.isEmpty || l.hasPrefix("#") { continue }
            if c.parada.activada { return 130 }
            if l == "exit" { break }
            codigo = await ejecutaTexto(l, entrada: nil, captura: nil)
            ultimoCodigo = codigo
        }
        return codigo
    }

    /// xargs orden: cada palabra de la entrada como argumento.
    func cmdXargs(_ c: Contexto) async -> Int32 {
        let palabras = c.entrada.split(whereSeparator: { $0 == " " || $0 == "\n" || $0 == "\t" }).map { "'" + $0 + "'" }
        let orden = (c.args.isEmpty ? ["echo"] : c.args).joined(separator: " ")
        return await ejecutaTexto(orden + " " + palabras.joined(separator: " "), entrada: nil, captura: nil)
    }

    func cmdNano(_ c: Contexto) -> Int32 {
        guard let f = c.args.first(where: { !$0.hasPrefix("-") }) else { c.error("\(c.nombre): falta el archivo (nano archivo.py)\n"); return 1 }
        if esCarpeta(f) { c.error("\(c.nombre): '\(f)' es una carpeta\n"); return 1 }
        if !existe(f) { escribeArchivo(f, "") }
        alEditar?(absoluta(f))
        return 0
    }

    // MARK: nyx

    /// nyx <pregunta> · nyx piensa … · nyx aprende … · nyx imagina … · nyx lee archivo
    /// nyx guarda [archivo] · nyx carga archivo · nyx estado
    func cmdNyx(_ c: Contexto) async -> Int32 {
        guard let cerebro = cerebro else { c.error("nyx: la mente no está conectada\n"); return 1 }
        var a = c.args
        if a.isEmpty && c.hayEntrada { a = [c.entrada] }
        guard let primera = a.first else {
            c.escribe("""
            nyx — habla con Nyx desde la terminal:
              nyx ¿qué es un motor?        pregúntale (o cualquier frase)
              nyx piensa ¿tiene pelo el gato?   y enseña cómo lo pensó
              nyx aprende el delfín es un mamífero
              nyx imagina un pez que vuela
              nyx lee apuntes.txt           que lea un archivo de la terminal
              nyx guarda [memoria.txt]      guarda todo lo que sabe en un archivo
              nyx carga memoria.txt         lo recuerda todo
              nyx estado
              cat notas.txt | nyx aprende   también por tubería

            """)
            return 0
        }
        let resto = a.dropFirst().joined(separator: " ")
        switch primera {
        case "piensa":
            let r = cerebro.terminalPregunta(resto)
            for p in r.pensamiento { c.escribe("  💭 " + p + "\n") }
            c.escribe("🌌 " + r.respuesta + "\n")
        case "aprende", "enseña":
            let t = resto.isEmpty ? c.entrada : resto
            var n = 0
            for l in Lector.frases(de: t) { _ = cerebro.terminalAprende(l); n += 1 }
            if n == 0 && !t.isEmpty { c.escribe(cerebro.terminalAprende(t) + "\n") } else { c.escribe("🌌 aprendí \(n) frase(s)\n") }
        case "imagina":
            c.escribe(cerebro.terminalImagina("imagina " + resto) + "\n")
        case "lee":
            guard let f = a.dropFirst().first, existe(f) else { c.error("nyx lee: falta un archivo que exista\n"); return 1 }
            let tmp = FileManager.default.temporaryDirectory.appendingPathComponent("nyx_term_" + UUID().uuidString + "_" + (f as NSString).lastPathComponent)
            try? FileManager.default.copyItem(at: url(f), to: tmp)
            cerebro.terminalLee(tmp, nombre: f)
            c.escribe("🌌 lo estoy leyendo (mira el progreso en la pestaña Nyx)\n")
        case "guarda":
            let f = a.dropFirst().first ?? "nyx_memoria.txt"
            guard let d = await cerebro.terminalGuardaMemoria() else { c.error("nyx: ahora no puedo guardar (¿estoy recordando?)\n"); return 1 }
            do { try d.write(to: url(f)) } catch { c.error("nyx: no puedo escribir \(f)\n"); return 1 }
            c.escribe("💾 guardé todo lo que sé en \(f) (\(Consola.humano(d.count)))\n")
        case "carga":
            guard let f = a.dropFirst().first, existe(f) else { c.error("nyx carga: falta el archivo de memoria\n"); return 1 }
            cerebro.terminalCargaMemoria(url(f))
            c.escribe("📂 recordando lo que hay en \(f)…\n")
        case "estado":
            c.escribe("🌌 " + cerebro.terminalEstado() + "\n")
        default:
            let r = cerebro.terminalPregunta(a.joined(separator: " "))
            c.escribe("🌌 " + r.respuesta + "\n")
        }
        return 0
    }
}

/// SHA-256 escrito a mano (para sha256sum; no depende de CryptoKit).
enum SHA256Nyx {
    private static let k: [UInt32] = [
        0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
        0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
        0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
        0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
        0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
        0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
        0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
        0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2]

    private static func rot(_ x: UInt32, _ n: UInt32) -> UInt32 { (x >> n) | (x << (32 - n)) }

    static func hex(_ datos: Data) -> String {
        var h: [UInt32] = [0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a, 0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19]
        var m = [UInt8](datos)
        let largo = UInt64(m.count) * 8
        m.append(0x80)
        while m.count % 64 != 56 { m.append(0) }
        for i in stride(from: 56, through: 0, by: -8) { m.append(UInt8((largo >> UInt64(i)) & 0xff)) }
        for bloque in stride(from: 0, to: m.count, by: 64) { comprime(m, bloque, &h) }
        return h.map { String(format: "%08x", $0) }.joined()
    }

    private static func palabras(_ m: [UInt8], _ bloque: Int) -> [UInt32] {
        var w = [UInt32](repeating: 0, count: 64)
        for t in 0 ..< 16 {
            let b = bloque + t * 4
            let x0 = UInt32(m[b]) << 24
            let x1 = UInt32(m[b + 1]) << 16
            let x2 = UInt32(m[b + 2]) << 8
            w[t] = x0 | x1 | x2 | UInt32(m[b + 3])
        }
        for t in 16 ..< 64 {
            let s0: UInt32 = rot(w[t - 15], 7) ^ rot(w[t - 15], 18) ^ (w[t - 15] >> 3)
            let s1: UInt32 = rot(w[t - 2], 17) ^ rot(w[t - 2], 19) ^ (w[t - 2] >> 10)
            w[t] = w[t - 16] &+ s0 &+ w[t - 7] &+ s1
        }
        return w
    }

    private static func comprime(_ m: [UInt8], _ bloque: Int, _ h: inout [UInt32]) {
        let w = palabras(m, bloque)
        var v = h
        for t in 0 ..< 64 {
            let e = v[4]
            let a = v[0]
            let s1: UInt32 = rot(e, 6) ^ rot(e, 11) ^ rot(e, 25)
            let ch: UInt32 = (e & v[5]) ^ (~e & v[6])
            let t1: UInt32 = v[7] &+ s1 &+ ch &+ k[t] &+ w[t]
            let s0: UInt32 = rot(a, 2) ^ rot(a, 13) ^ rot(a, 22)
            let may: UInt32 = (a & v[1]) ^ (a & v[2]) ^ (v[1] & v[2])
            let t2: UInt32 = s0 &+ may
            v = [t1 &+ t2, a, v[1], v[2], v[3] &+ t1, e, v[5], v[6]]
        }
        for i in 0 ..< 8 { h[i] = h[i] &+ v[i] }
    }

}
