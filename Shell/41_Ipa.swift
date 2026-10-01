import Foundation

// ============================================================
// MARK: - .ipa, .zip y DEFLATE en Swift puro
// ============================================================
// Un .ipa es un zip con Payload/<App>.app dentro. Aquí se LEE: nombre,
// identificador, versión, iOS mínimo, permisos que pide, archivos…
// No se instala ni se ejecuta: iOS solo deja que el sistema instale
// apps firmadas por Apple.
//
//   ipa <app.ipa>                     resumen
//   ipa permisos <app.ipa>            qué pide (cámara, fotos, ubicación…)
//   ipa info <app.ipa>                Info.plist completo
//   ipa lista <app.ipa> [texto]       archivos de dentro
//   ipa extrae <app.ipa> <ruta> [destino]
//   unzip [-l] <archivo.zip> [-d carpeta]
//
// El descompresor (RFC 1951) está escrito aquí mismo: no necesita el
// framework Compression, así que funciona también en modo seguro
// (y arregla gunzip y los .deb sin él).

enum Inflar {

    struct Fallo: Error { let m: String }

    private struct Lector {
        let d: [UInt8]
        var pos: Int
        var buf = 0
        var cuenta = 0

        mutating func bits(_ n: Int) throws -> Int {
            var v = buf
            while cuenta < n {
                guard pos < d.count else { throw Fallo(m: "datos DEFLATE incompletos") }
                v |= Int(d[pos]) << cuenta
                pos += 1
                cuenta += 8
            }
            buf = v >> n
            cuenta -= n
            return v & ((1 << n) - 1)
        }
    }

    /// Tabla de Huffman canónica: cuántos códigos hay de cada largo y
    /// los símbolos en orden.
    private struct Huffman {
        var cuenta = [Int](repeating: 0, count: 16)
        var simbolo: [Int]

        init(_ largos: [Int]) throws {
            simbolo = [Int](repeating: 0, count: largos.count)
            for l in largos { cuenta[l] += 1 }
            // desplazamientos de cada largo (como puff.c)
            var offs = [Int](repeating: 0, count: 16)
            offs[1] = 0
            for l in 1 ..< 15 { offs[l + 1] = offs[l] + cuenta[l] }
            for (s, l) in largos.enumerated() where l != 0 {
                simbolo[offs[l]] = s
                offs[l] += 1
            }
        }

        func decodifica(_ r: inout Lector) throws -> Int {
            var codigo = 0, primero = 0, indice = 0
            for l in 1 ..< 16 {
                codigo |= try r.bits(1)
                let c = cuenta[l]
                if codigo - c < primero { return simbolo[indice + (codigo - primero)] }
                indice += c
                primero += c
                primero <<= 1
                codigo <<= 1
            }
            throw Fallo(m: "código de Huffman no válido")
        }
    }

    private static let baseLargo = [3, 4, 5, 6, 7, 8, 9, 10, 11, 13, 15, 17, 19, 23, 27, 31, 35, 43, 51, 59, 67, 83, 99, 115, 131, 163, 195, 227, 258]
    private static let extraLargo = [0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 0]
    private static let baseDist = [1, 2, 3, 4, 5, 7, 9, 13, 17, 25, 33, 49, 65, 97, 129, 193, 257, 385, 513, 769, 1025, 1537, 2049, 3073, 4097, 6145, 8193, 12289, 16385, 24577]
    private static let extraDist = [0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 12, 12, 13, 13]

    /// Descomprime DEFLATE puro (sin cabecera zlib ni gzip).
    static func deflate(_ datos: [UInt8], maximo: Int = 256 << 20) throws -> [UInt8] {
        var r = Lector(d: datos, pos: 0)
        var out: [UInt8] = []
        out.reserveCapacity(datos.count * 3)
        var ultimo = 0
        repeat {
            ultimo = try r.bits(1)
            let tipo = try r.bits(2)
            switch tipo {
            case 0:                                   // sin comprimir
                r.buf = 0; r.cuenta = 0
                guard r.pos + 4 <= r.d.count else { throw Fallo(m: "bloque sin comprimir incompleto") }
                let len = Int(r.d[r.pos]) | Int(r.d[r.pos + 1]) << 8
                r.pos += 4
                guard r.pos + len <= r.d.count else { throw Fallo(m: "bloque sin comprimir incompleto") }
                out.append(contentsOf: r.d[r.pos ..< r.pos + len])
                r.pos += len
            case 1, 2:
                let (lit, dist): (Huffman, Huffman)
                if tipo == 1 {
                    var l = [Int](repeating: 8, count: 288)
                    for i in 144 ..< 256 { l[i] = 9 }
                    for i in 256 ..< 280 { l[i] = 7 }
                    lit = try Huffman(l)
                    dist = try Huffman([Int](repeating: 5, count: 30))
                } else {
                    let nlen = try r.bits(5) + 257
                    let ndist = try r.bits(5) + 1
                    let ncode = try r.bits(4) + 4
                    let orden = [16, 17, 18, 0, 8, 7, 9, 6, 10, 5, 11, 4, 12, 3, 13, 2, 14, 1, 15]
                    var lc = [Int](repeating: 0, count: 19)
                    for i in 0 ..< ncode { lc[orden[i]] = try r.bits(3) }
                    let hc = try Huffman(lc)
                    var largos: [Int] = []
                    while largos.count < nlen + ndist {
                        let s = try hc.decodifica(&r)
                        if s < 16 { largos.append(s) }
                        else if s == 16 {
                            guard let prev = largos.last else { throw Fallo(m: "repetición sin código previo") }
                            largos += [Int](repeating: prev, count: 3 + (try r.bits(2)))
                        } else if s == 17 { largos += [Int](repeating: 0, count: 3 + (try r.bits(3))) }
                        else { largos += [Int](repeating: 0, count: 11 + (try r.bits(7))) }
                    }
                    guard largos.count == nlen + ndist else { throw Fallo(m: "largos de código no válidos") }
                    lit = try Huffman(Array(largos[0 ..< nlen]))
                    dist = try Huffman(Array(largos[nlen...]))
                }
                while true {
                    let s = try lit.decodifica(&r)
                    if s < 256 { out.append(UInt8(s)); continue }
                    if s == 256 { break }
                    let i = s - 257
                    guard i < baseLargo.count else { throw Fallo(m: "largo no válido") }
                    let largo = baseLargo[i] + (try r.bits(extraLargo[i]))
                    let ds = try dist.decodifica(&r)
                    guard ds < baseDist.count else { throw Fallo(m: "distancia no válida") }
                    let d = baseDist[ds] + (try r.bits(extraDist[ds]))
                    guard d <= out.count else { throw Fallo(m: "distancia hacia atrás demasiado grande") }
                    let ini = out.count - d
                    for k in 0 ..< largo { out.append(out[ini + k]) }
                    if out.count > maximo { throw Fallo(m: "archivo demasiado grande") }
                }
            default:
                throw Fallo(m: "tipo de bloque no válido")
            }
        } while ultimo == 0
        return out
    }

    /// DEFLATE "sin comprimir" (bloques tipo 0): válido para cualquier
    /// descompresor; sirve cuando no hay Compression.framework.
    static func guardado(_ datos: [UInt8]) -> [UInt8] {
        var out: [UInt8] = []
        var i = 0
        repeat {
            let n = min(65535, datos.count - i)
            let ultimo: UInt8 = i + n >= datos.count ? 1 : 0
            out.append(ultimo)
            out += [UInt8(n & 0xff), UInt8(n >> 8), UInt8(~n & 0xff), UInt8((~n >> 8) & 0xff)]
            out += datos[i ..< i + n]
            i += n
        } while i < datos.count
        return out
    }
}

/// Lector de zip (y de .ipa, .apk, .jar…): directorio central + DEFLATE.
struct Zip {
    struct Entrada {
        let nombre: String
        let metodo: Int
        let comprimido: Int
        let tamaño: Int
        let offsetLocal: Int
    }

    let datos: [UInt8]
    let entradas: [Entrada]

    init(_ d: Data) throws {
        let datos = [UInt8](d)
        func u16(_ i: Int) -> Int { Int(datos[i]) | Int(datos[i + 1]) << 8 }
        func u32(_ i: Int) -> Int { u16(i) | u16(i + 2) << 16 }
        guard datos.count >= 22 else { throw Inflar.Fallo(m: "no es un zip (demasiado corto)") }
        // fin del directorio central: firma 50 4b 05 06, buscando desde el final
        var eocd = -1
        var i = datos.count - 22
        while i >= max(0, datos.count - 65557) {
            if datos[i] == 0x50, datos[i + 1] == 0x4b, datos[i + 2] == 5, datos[i + 3] == 6 { eocd = i; break }
            i -= 1
        }
        guard eocd >= 0 else { throw Inflar.Fallo(m: "no es un zip (falta el directorio central)") }
        let total = u16(eocd + 10)
        var p = u32(eocd + 16)
        var es: [Entrada] = []
        for _ in 0 ..< total {
            guard p + 46 <= datos.count, u32(p) == 0x02014b50 else { throw Inflar.Fallo(m: "directorio central dañado") }
            let n = u16(p + 28), extra = u16(p + 30), coment = u16(p + 32)
            guard p + 46 + n <= datos.count else { throw Inflar.Fallo(m: "nombre de archivo dañado") }
            let nombre = String(decoding: datos[p + 46 ..< p + 46 + n], as: UTF8.self)
            es.append(Entrada(nombre: nombre, metodo: u16(p + 10), comprimido: u32(p + 20), tamaño: u32(p + 24), offsetLocal: u32(p + 42)))
            p += 46 + n + extra + coment
        }
        self.datos = datos
        entradas = es
    }

    func lee(_ e: Entrada) throws -> Data {
        let p = e.offsetLocal
        guard p + 30 <= datos.count else { throw Inflar.Fallo(m: "entrada fuera del archivo") }
        let n = Int(datos[p + 26]) | Int(datos[p + 27]) << 8
        let extra = Int(datos[p + 28]) | Int(datos[p + 29]) << 8
        let ini = p + 30 + n + extra
        guard ini + e.comprimido <= datos.count else { throw Inflar.Fallo(m: "\(e.nombre): datos incompletos") }
        let trozo = Array(datos[ini ..< ini + e.comprimido])
        switch e.metodo {
        case 0: return Data(trozo)
        case 8: return Data(try Inflar.deflate(trozo, maximo: max(e.tamaño, 1) + 1024))
        default: throw Inflar.Fallo(m: "\(e.nombre): método de compresión \(e.metodo) no soportado")
        }
    }
}

extension Shell {

    static func ipa() -> [String: Spec] {
        var c: [String: Spec] = [:]

        func abre(_ ctx: Ctx, _ p: String) throws -> Zip {
            let u = try ctx.env.resolve(p)
            guard let d = FileManager.default.contents(atPath: u.path) else { throw ShErr("\(p): no existe") }
            do { return try Zip(d) } catch let f as Inflar.Fallo { throw ShErr("\(p): \(f.m)") }
        }

        func infoPlist(_ z: Zip) throws -> (app: String, plist: [String: Any]) {
            guard let e = z.entradas.first(where: {
                let partes = $0.nombre.split(separator: "/")
                return partes.count == 3 && partes[0] == "Payload" && partes[1].hasSuffix(".app") && partes[2] == "Info.plist"
            }) else { throw ShErr("ipa: no encuentro Payload/<App>.app/Info.plist (¿es un .ipa?)") }
            let d = try z.lee(e)
            guard let p = try? PropertyListSerialization.propertyList(from: d, format: nil) as? [String: Any] else {
                throw ShErr("ipa: el Info.plist no se puede leer")
            }
            return (String(e.nombre.split(separator: "/")[1]), p)
        }

        func texto(_ v: Any?) -> String {
            switch v {
            case nil: return "—"
            case let s as String: return s
            case let a as [Any]: return a.map { texto($0) }.joined(separator: ", ")
            case let n as NSNumber: return n.stringValue
            default: return "\(v!)"
            }
        }

        c["ipa"] = Spec(help: "ipa [permisos|info|lista|extrae] <app.ipa> — lee una app de iOS (no la instala)") { ctx in
            let a = ctx.args
            let subs = ["permisos", "info", "lista", "extrae", "resumen"]
            let sub = a.first.flatMap { subs.contains($0) ? $0 : nil } ?? "resumen"
            let resto = sub == "resumen" && a.first != "resumen" ? a : Array(a.dropFirst())
            guard let archivo = resto.first else {
                return """
                ipa — lee archivos .ipa (apps de iOS). No los instala: iOS no lo permite.
                  ipa <app.ipa>                      resumen
                  ipa permisos <app.ipa>             qué pide la app
                  ipa info <app.ipa>                 Info.plist completo
                  ipa lista <app.ipa> [texto]        archivos de dentro
                  ipa extrae <app.ipa> <ruta> [dest] saca un archivo

                """
            }
            let z = try abre(ctx, archivo)

            switch sub {
            case "lista":
                let filtro = resto.dropFirst().first?.lowercased() ?? ""
                let es = z.entradas.filter { filtro.isEmpty || $0.nombre.lowercased().contains(filtro) }
                return es.map { "\(humanSize($0.tamaño).leftPad(8))  \($0.nombre)" }.joined(separator: "\n") + "\n\(es.count) archivos\n"

            case "extrae":
                guard resto.count >= 2 else { throw ShErr("uso: ipa extrae <app.ipa> <ruta dentro> [destino]") }
                let ruta = resto[1]
                guard let e = z.entradas.first(where: { $0.nombre == ruta || $0.nombre.hasSuffix("/" + ruta) }) else {
                    throw ShErr("ipa: no hay '\(ruta)' dentro (mira 'ipa lista \(archivo)')")
                }
                let destino = resto.count > 2 ? resto[2] : (e.nombre as NSString).lastPathComponent
                try z.lee(e).write(to: try ctx.env.resolve(destino))
                return "extraído \(e.nombre) → \(destino)\n"

            case "info":
                let (_, p) = try infoPlist(z)
                return p.keys.sorted().map { "\($0) = \(texto(p[$0]))" }.joined(separator: "\n") + "\n"

            case "permisos":
                let (app, p) = try infoPlist(z)
                let permisos = p.keys.filter { $0.hasSuffix("UsageDescription") }.sorted()
                if permisos.isEmpty { return "\(app) no pide permisos especiales\n" }
                return permisos.map {
                    let corto = $0.replacingOccurrences(of: "NS", with: "").replacingOccurrences(of: "UsageDescription", with: "")
                    return "\(corto): \(texto(p[$0]))"
                }.joined(separator: "\n") + "\n"

            default:
                let (app, p) = try infoPlist(z)
                let total = z.entradas.reduce(0) { $0 + $1.tamaño }
                let frameworks = Set(z.entradas.compactMap { e -> String? in
                    guard let r = e.nombre.range(of: ".app/Frameworks/") else { return nil }
                    return e.nombre[r.upperBound...].split(separator: "/").first.map(String.init)
                }).sorted()
                let familia = (p["UIDeviceFamily"] as? [Any])?.compactMap { texto($0) }.map { $0 == "1" ? "iPhone" : ($0 == "2" ? "iPad" : $0) } ?? []
                let permisos = p.keys.filter { $0.hasSuffix("UsageDescription") }.count
                var out = "\(texto(p["CFBundleDisplayName"] ?? p["CFBundleName"]))  (\(app))\n"
                out += "identificador   \(texto(p["CFBundleIdentifier"]))\n"
                out += "versión         \(texto(p["CFBundleShortVersionString"])) (\(texto(p["CFBundleVersion"])))\n"
                out += "iOS mínimo      \(texto(p["MinimumOSVersion"]))\n"
                out += "dispositivos    \(familia.isEmpty ? "—" : familia.joined(separator: ", "))\n"
                out += "ejecutable      \(texto(p["CFBundleExecutable"]))\n"
                out += "permisos        \(permisos)\(permisos > 0 ? " (ipa permisos \(archivo))" : "")\n"
                out += "frameworks      \(frameworks.isEmpty ? "ninguno" : frameworks.joined(separator: ", "))\n"
                out += "archivos        \(z.entradas.count) · \(humanSize(total)) sin comprimir\n"
                return out
            }
        }

        c["unzip"] = Spec(help: "unzip [-l] <archivo.zip> [-d carpeta] — lista o descomprime un zip") { ctx in
            let o = opts(ctx.args, valued: ["d"])
            guard let p = o.rest.first else { throw ShErr("uso: unzip [-l] <archivo.zip> [-d carpeta]") }
            let z = try abre(ctx, p)
            if o.flags.contains("l") {
                return z.entradas.map { "\(String($0.tamaño).leftPad(10))  \($0.nombre)" }.joined(separator: "\n") +
                       "\n\(z.entradas.count) archivos\n"
            }
            let base = try ctx.env.resolve(o.vals["d"] ?? ".")
            let fm = FileManager.default
            var n = 0
            for e in z.entradas {
                let destino = base.appendingPathComponent(e.nombre).standardizedFileURL
                // no se sale de tu espacio aunque el zip traiga rutas con ../
                guard destino.path.hasPrefix(ctx.env.root.path) else { continue }
                if e.nombre.hasSuffix("/") {
                    try fm.createDirectory(at: destino, withIntermediateDirectories: true)
                    continue
                }
                try fm.createDirectory(at: destino.deletingLastPathComponent(), withIntermediateDirectories: true)
                try z.lee(e).write(to: destino)
                n += 1
            }
            return "descomprimidos \(n) archivos en \(ctx.env.vpath(base))\n"
        }

        return c
    }
}
