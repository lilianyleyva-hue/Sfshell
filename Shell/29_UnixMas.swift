import Foundation
#if (canImport(Compression) && APPLE_COMPLETO)
import Compression
#endif

// ============================================================
// MARK: - Lo que faltaba del catálogo clásico de Unix/Linux
// ============================================================
// Cierra los huecos que quedaban frente a un bash real: unos son
// comandos de verdad (tar, gzip/gunzip reutilizan el mismo Tar/Gzip
// que ya usa 'deb'; ifconfig lee las interfaces de red reales del
// dispositivo con getifaddrs; rsync sincroniza de verdad comparando
// contenido, aunque solo entre rutas del propio sandbox). Los que
// de verdad no se pueden hacer en el sandbox de una app de iOS
// (procesos ajenos, un demonio en segundo plano, sockets en crudo)
// lo dicen claro, con el motivo, en vez de fingir.

extension Gzip {
    /// Comprime con DEFLATE real (Compression.framework) y arma a mano
    /// la cabecera y el pie de un .gz de verdad (CRC32 + tamaño).
    static func compress(_ data: Data) -> Data? {
        let bytes = [UInt8](data)
        let payload: Data
        if bytes.isEmpty {
            payload = Data()
        } else {
            #if !(canImport(Compression) && APPLE_COMPLETO)
            return nil   // sin Compression.framework (Linux)
            #else
            let capacity = max(bytes.count + 512, 256)
            let dst = UnsafeMutablePointer<UInt8>.allocate(capacity: capacity)
            defer { dst.deallocate() }
            let n = bytes.withUnsafeBufferPointer { src -> Int in
                guard let base = src.baseAddress else { return 0 }
                return compression_encode_buffer(dst, capacity, base, bytes.count, nil, COMPRESSION_ZLIB)
            }
            guard n > 0 else { return nil }
            payload = Data(bytes: dst, count: n)
            #endif
        }
        var out = Data([0x1f, 0x8b, 0x08, 0x00, 0, 0, 0, 0, 0x00, 0xff])   // cabecera gzip mínima
        out.append(payload)
        let crc = Shell.crc32(data)
        withUnsafeBytes(of: crc.littleEndian) { out.append(contentsOf: $0) }
        let isize = UInt32(truncatingIfNeeded: data.count)
        withUnsafeBytes(of: isize.littleEndian) { out.append(contentsOf: $0) }
        return out
    }
}

extension Shell {

    /// CRC32 de toda la vida (tabla IEEE 802.3), a mano porque Foundation no trae una.
    static func crc32(_ data: Data) -> UInt32 {
        var crc: UInt32 = 0xFFFFFFFF
        for byte in data {
            crc ^= UInt32(byte)
            for _ in 0..<8 {
                crc = (crc & 1 != 0) ? (crc >> 1) ^ 0xEDB88320 : crc >> 1
            }
        }
        return crc ^ 0xFFFFFFFF
    }

    static func unixMas() -> [String: Spec] {
        var c: [String: Spec] = [:]
        let fm = FileManager.default

        // --- tar: empaqueta/extrae, formato ustar real (mismo que usa 'deb') ---

        c["tar"] = Spec(help: "tar -c|-x|-t -f archivo.tar [-C destino] [archivos...] — formato ustar real") { ctx in
            let o = opts(ctx.args, valued: ["f", "C"])
            guard let archivo = o.vals["f"] else { throw ShErr("tar: usa -f archivo.tar (junto con -c, -x o -t)") }

            if o.flags.contains("c") {
                guard !o.rest.isEmpty else { throw ShErr("tar -c: falta al menos un archivo") }
                var miembros: [(path: String, body: Data)] = []
                for p in o.rest {
                    let u = try ctx.env.resolve(p)
                    guard !ctx.env.isDir(u), let d = fm.contents(atPath: u.path) else {
                        throw ShErr("tar: \(p): no se puede leer (¿es una carpeta?)")
                    }
                    miembros.append((p, d))
                }
                let data = Tar.build(miembros)
                try data.write(to: try ctx.env.resolve(archivo))
                return "creado \(archivo) (\(humanSize(data.count)))\n"
            }

            let u = try ctx.env.resolve(archivo)
            guard let d = fm.contents(atPath: u.path) else { throw ShErr("tar: \(archivo): no existe") }
            let miembros = Tar.parse(d)

            if o.flags.contains("t") {
                return miembros.map { "\(humanSize($0.body.count).leftPad(8))  \($0.path)" }.joined(separator: "\n") + "\n"
            }
            if o.flags.contains("x") {
                let destino = try ctx.env.resolve(o.vals["C"] ?? ".")
                var n = 0
                for f in miembros {
                    guard !f.path.isEmpty, !f.path.hasSuffix("/") else { continue }
                    let target = destino.appendingPathComponent(f.path)
                    try fm.createDirectory(at: target.deletingLastPathComponent(), withIntermediateDirectories: true)
                    try f.body.write(to: target)
                    n += 1
                }
                return "extraídos \(n) archivos en \(ctx.env.vpath(destino))\n"
            }
            throw ShErr("tar: usa -c (crear), -x (extraer) o -t (listar) junto con -f archivo.tar")
        }

        // --- gzip / gunzip: DEFLATE real vía Compression.framework ---

        c["gzip"] = Spec(help: "gzip [-k] <archivo> — comprime a .gz de verdad (DEFLATE, no un truco de texto)") { ctx in
            let o = opts(ctx.args, valued: [])
            guard let p = o.rest.first else { throw ShErr("gzip: falta el archivo") }
            let u = try ctx.env.resolve(p)
            guard let d = fm.contents(atPath: u.path) else { throw ShErr("gzip: \(p): no se puede leer") }
            guard let comprimido = Gzip.compress(d) else { throw ShErr("gzip: no se pudo comprimir") }
            let destino = u.appendingPathExtension("gz")
            try comprimido.write(to: destino)
            if !o.flags.contains("k") { try? fm.removeItem(at: u) }
            return "\(p).gz — \(humanSize(d.count)) → \(humanSize(comprimido.count))\n"
        }

        c["gunzip"] = Spec(help: "gunzip <archivo.gz> — descomprime un .gz real") { ctx in
            guard let p = ctx.args.first else { throw ShErr("gunzip: falta el archivo") }
            let u = try ctx.env.resolve(p)
            guard let d = fm.contents(atPath: u.path) else { throw ShErr("gunzip: \(p): no se puede leer") }
            guard let plano = Gzip.inflate(d) else { throw ShErr("gunzip: \(p): no es un .gz válido") }
            let destino = u.pathExtension.lowercased() == "gz" ? u.deletingPathExtension() : u.appendingPathExtension("out")
            try plano.write(to: destino)
            return "\(destino.lastPathComponent) — \(humanSize(plano.count))\n"
        }

        // --- ifconfig: interfaces de red REALES del dispositivo (getifaddrs) ---

        c["ifconfig"] = Spec(help: "ifconfig — interfaces de red reales del dispositivo (getifaddrs, no inventado)") { _ in
            var out = ""
            var ptr: UnsafeMutablePointer<ifaddrs>?
            guard getifaddrs(&ptr) == 0, let primero = ptr else {
                throw ShErr("ifconfig: no se pudo leer la lista de interfaces")
            }
            defer { freeifaddrs(ptr) }

            var direcciones: [String: [String]] = [:]
            var orden: [String] = []
            var nodo: UnsafeMutablePointer<ifaddrs>? = primero
            while let n = nodo {
                let iface = n.pointee
                let nombre = String(cString: iface.ifa_name)
                if let sa = iface.ifa_addr, sa.pointee.sa_family == sa_family_t(AF_INET) || sa.pointee.sa_family == sa_family_t(AF_INET6) {
                    var host = [CChar](repeating: 0, count: Int(NI_MAXHOST))
                    #if os(Linux)
                    let largo = socklen_t(sa.pointee.sa_family == sa_family_t(AF_INET) ? MemoryLayout<sockaddr_in>.size : MemoryLayout<sockaddr_in6>.size)
                    #else
                    let largo = socklen_t(sa.pointee.sa_len)
                    #endif
                    getnameinfo(sa, largo, &host, socklen_t(host.count), nil, 0, NI_NUMERICHOST)
                    let addr = String(cString: host)
                    let etiqueta = (sa.pointee.sa_family == sa_family_t(AF_INET) ? "inet  " : "inet6 ") + addr
                    if direcciones[nombre] == nil { orden.append(nombre) }
                    direcciones[nombre, default: []].append(etiqueta)
                }
                nodo = iface.ifa_next
            }
            for nombre in orden {
                out += "\(nombre):\n"
                for linea in direcciones[nombre] ?? [] { out += "    \(linea)\n" }
            }
            return out.isEmpty ? "sin interfaces activas\n" : out
        }

        // --- rsync: sincronización real, local, dentro del sandbox ---
        // No hay 'rsync remoto' posible (haría falta un demonio rsync o SSH
        // al otro lado, y ninguno existe en un iPad), pero comparar y copiar
        // solo lo que cambió entre dos rutas de aquí dentro sí es real.

        c["rsync"] = Spec(help: "rsync <origen> <destino> — sincroniza copiando solo lo que cambió (local, dentro del sandbox)") { ctx in
            guard ctx.args.count >= 2 else { throw ShErr("rsync: usa rsync origen destino") }
            let origenURL = try ctx.env.resolve(ctx.args[0])
            let destinoURL = try ctx.env.resolve(ctx.args[1])
            guard fm.fileExists(atPath: origenURL.path) else { throw ShErr("rsync: \(ctx.args[0]): no existe") }

            func sincroniza(_ src: URL, _ dst: URL) throws -> Bool {
                let nuevo = fm.contents(atPath: src.path)
                let viejo = fm.contents(atPath: dst.path)
                guard nuevo != viejo else { return false }
                try fm.createDirectory(at: dst.deletingLastPathComponent(), withIntermediateDirectories: true)
                if fm.fileExists(atPath: dst.path) { try fm.removeItem(at: dst) }
                try fm.copyItem(at: src, to: dst)
                return true
            }

            var copiados = 0, revisados = 0
            if ctx.env.isDir(origenURL) {
                // ojo: 'for...in' sobre un FileManager.DirectoryEnumerator llama a
                // makeIterator(), y eso está prohibido en una función async bajo
                // Swift 6 (modo de concurrencia estricta) — con nextObject() a mano
                // se evita esa API y funciona exactamente igual.
                if let en = fm.enumerator(at: origenURL, includingPropertiesForKeys: nil) {
                    while let obj = en.nextObject() {
                        guard let f = obj as? URL, !ctx.env.isDir(f) else { continue }
                        let rel = f.path.replacingOccurrences(of: origenURL.path + "/", with: "")
                        revisados += 1
                        if try sincroniza(f, destinoURL.appendingPathComponent(rel)) { copiados += 1 }
                    }
                }
            } else {
                revisados = 1
                if try sincroniza(origenURL, destinoURL) { copiados = 1 }
            }
            return "\(copiados)/\(revisados) archivos actualizados en \(ctx.env.vpath(destinoURL))\n"
        }

        // --- lo que de verdad no se puede, con el motivo ---

        c["killall"] = Spec(help: "killall <nombre> — no disponible") { _ in
            throw ShErr("killall: igual que 'kill', esta terminal no lanza procesos aparte — todo\n" +
                        "corre en el mismo hilo de la app. No hay nada por nombre que matar.")
        }

        c["jobs"] = Spec(help: "jobs — no hay trabajos en segundo plano") { _ in
            "no hay trabajos en segundo plano: cada comando termina antes de devolverte\n" +
            "el prompt — no existe un '&' que deje algo corriendo aparte.\n"
        }

        c["crontab"] = Spec(help: "crontab — no disponible") { _ in
            throw ShErr("crontab: para ejecutar algo periódicamente hace falta un demonio despierto\n" +
                        "en segundo plano, y iOS suspende una app normal en cuanto sales de ella — no\n" +
                        "hay cron posible aquí. Lo más parecido: 'watch' repite un comando mientras\n" +
                        "la terminal siga abierta y en primer plano.")
        }

        c["traceroute"] = Spec(help: "traceroute <host> — no disponible") { _ in
            throw ShErr("traceroute: necesita mandar paquetes ICMP con TTL creciente y leer cada\n" +
                        "salto, y eso exige sockets en crudo — el sandbox de una app de iOS no deja\n" +
                        "abrir uno sin privilegios especiales. Usa 'ping' (mide latencia real por\n" +
                        "HTTPS) o 'nslookup'/'whois' para lo demás.")
        }

        c["netstat"] = Spec(help: "netstat — no disponible") { _ in
            throw ShErr("netstat: lista las conexiones abiertas de todo el sistema, y una app en su\n" +
                        "sandbox no tiene ninguna API pública para ver ni siquiera las suyas propias\n" +
                        "a ese nivel. No hay tabla de sockets que enseñar aquí. Mira 'ifconfig' para\n" +
                        "las interfaces de red reales del dispositivo.")
        }

        c["chown"] = Spec(help: "chown — no disponible") { _ in
            throw ShErr("chown: en el sandbox de una app solo existe un dueño posible — la app\n" +
                        "misma — así que no hay a quién cambiarle un archivo. 'chmod' sí funciona\n" +
                        "de verdad porque los permisos de lectura/escritura sí se guardan.")
        }
        c["chgrp"] = c["chown"]

        return c
    }
}
