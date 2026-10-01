import Foundation
#if (canImport(Compression) && APPLE_COMPLETO)
import Compression
#endif

// ============================================================
// MARK: - Deb / dpkg
// ============================================================
// Este archivo contenía por error una copia antigua de 5_Shell.swift
// (struct Ctx, class Shell, execute… repetidos), lo que impedía compilar:
// "invalid redeclaration". Además 5_Shell.swift llama a Shell.deb(), que
// ya no existía en ningún archivo.
//
// Esto es un reemplazo que compila y responde con honestidad.
// Si tienes tu versión original de los comandos .deb, pégala aquí
// dentro de 'deb()' y borra este bloque.

extension Shell {

    static func deb() -> [String: Spec] {
        var c: [String: Spec] = [:]
        let fm = FileManager.default

        let motivo = "iPadOS solo ejecuta binarios firmados: los programas de un .deb " +
                     "(código máquina de Linux) no pueden correr aquí.\n" +
                     "Usa 'pkg install' para paquetes de esta terminal (js, swift, sim).\n"

        c["deb"] = Spec(help: "deb <archivo.deb> — explica qué se puede hacer con un .deb") { ctx in
            guard let p = ctx.args.first else {
                return "deb: uso: deb <archivo.deb>\n" + motivo
            }
            let u = try ctx.env.resolve(p)
            guard ctx.env.exists(u) else { throw ShErr("deb: \(p): no existe") }
            // un .deb es un archivo 'ar': comprobamos la firma de verdad
            let cabecera = (fm.contents(atPath: u.path) ?? Data()).prefix(8)
            let esAr = String(data: cabecera, encoding: .ascii) == "!<arch>\n"
            return (esAr ? "\(p): paquete .deb válido (formato ar)\n"
                         : "\(p): no parece un .deb (falta la firma !<arch>)\n") + motivo
        }

        c["dpkg"] = Spec(help: "dpkg — no disponible en iPadOS; explica por qué") { _ in
            "dpkg: no disponible.\n" + motivo
        }

        return c
    }
}

// ============================================================
// MARK: - Tar y Gzip
// ============================================================
// 29_UnixMas.swift (tar, gzip, gunzip) usa Tar y Gzip, que vivían en la
// versión vieja de este archivo. Al sustituirlo se perdieron y el proyecto
// dejó de compilar. Aquí están otra vez.

enum Tar {
    /// Empaqueta en formato ustar (bloques de 512 bytes).
    static func build(_ miembros: [(path: String, body: Data)]) -> Data {
        var out = Data()
        for m in miembros {
            var h = [UInt8](repeating: 0, count: 512)
            func pon(_ s: String, _ at: Int, _ len: Int) {
                let b = Array(s.utf8.prefix(len))
                for (i, x) in b.enumerated() { h[at + i] = x }
            }
            func octal(_ v: Int, _ at: Int, _ len: Int) {
                pon(String(String(v, radix: 8).suffix(len - 1)).leftPadZeros(len - 1), at, len - 1)
            }
            pon(m.path, 0, 100)
            octal(0o644, 100, 8)
            octal(0, 108, 8)
            octal(0, 116, 8)
            octal(m.body.count, 124, 12)
            octal(Int(Date().timeIntervalSince1970), 136, 12)
            for i in 148..<156 { h[i] = 0x20 }          // el checksum se calcula con espacios
            h[156] = UInt8(ascii: "0")
            pon("ustar", 257, 6)
            pon("00", 263, 2)
            let suma = h.reduce(0) { $0 + Int($1) }
            pon(String(String(suma, radix: 8)).leftPadZeros(6), 148, 6)
            h[154] = 0; h[155] = 0x20
            out.append(contentsOf: h)
            out.append(m.body)
            let resto = m.body.count % 512
            if resto != 0 { out.append(Data(count: 512 - resto)) }
        }
        out.append(Data(count: 1024))                   // dos bloques vacíos = fin
        return out
    }

    /// Lee un .tar (ustar o el antiguo v7). Solo archivos normales.
    static func parse(_ d: Data) -> [(path: String, body: Data)] {
        let b = [UInt8](d)
        var r: [(path: String, body: Data)] = []
        var i = 0
        func texto(_ at: Int, _ len: Int) -> String {
            let trozo = b[at..<min(at + len, b.count)].prefix { $0 != 0 }
            return String(decoding: trozo, as: UTF8.self)
        }
        while i + 512 <= b.count {
            if b[i..<(i + 512)].allSatisfy({ $0 == 0 }) { break }
            var nombre = texto(i, 100)
            let prefijo = texto(i + 345, 155)
            if texto(i + 257, 5) == "ustar", !prefijo.isEmpty { nombre = prefijo + "/" + nombre }
            let tamTexto = texto(i + 124, 12).trimmingCharacters(in: .whitespaces)
            let tam = Int(tamTexto, radix: 8) ?? 0
            let tipo = b[i + 156]
            let inicio = i + 512
            let fin = min(inicio + tam, b.count)
            if tipo == 0 || tipo == UInt8(ascii: "0") {
                r.append((nombre, Data(b[inicio..<fin])))
            }
            i = inicio + ((tam + 511) / 512) * 512
        }
        return r
    }
}

enum Gzip {
    /// Descomprime un .gz (o un zlib/deflate suelto) con Compression.framework.
    static func inflate(_ d: Data) -> Data? {
        let b = [UInt8](d)
        var cuerpo: [UInt8]
        var esperado = 0
        if b.count >= 18, b[0] == 0x1f, b[1] == 0x8b {
            let flags = b[3]
            var p = 10
            if flags & 0x04 != 0, p + 2 <= b.count {                                            // FEXTRA
                let extra = Int(b[p]) + (Int(b[p + 1]) << 8)
                p += 2 + extra
            }
            if flags & 0x08 != 0 { while p < b.count && b[p] != 0 { p += 1 }; p += 1 }        // FNAME
            if flags & 0x10 != 0 { while p < b.count && b[p] != 0 { p += 1 }; p += 1 }        // FCOMMENT
            if flags & 0x02 != 0 { p += 2 }                                                     // FHCRC
            guard p < b.count - 8 else { return nil }
            cuerpo = Array(b[p..<(b.count - 8)])
            let n = b.count
            let b0 = Int(b[n - 4]), b1 = Int(b[n - 3]) << 8
            let b2 = Int(b[n - 2]) << 16, b3 = Int(b[n - 1]) << 24
            esperado = b0 | b1 | b2 | b3
        } else if b.count > 2, b[0] & 0x0f == 8, ((Int(b[0]) << 8) + Int(b[1])) % 31 == 0 {
            cuerpo = Array(b[2...])                    // cabecera zlib
        } else {
            cuerpo = b                                 // deflate sin cabecera
        }
        guard !cuerpo.isEmpty else { return Data() }
        #if !(canImport(Compression) && APPLE_COMPLETO)
        // sin Compression.framework: el descompresor en Swift puro (41_Ipa.swift)
        return (try? Inflar.deflate(cuerpo)).map { Data($0) }
        #else
        var capacidad = esperado > 0 ? esperado : max(cuerpo.count * 8, 4096)
        for _ in 0..<6 {
            let dst = UnsafeMutablePointer<UInt8>.allocate(capacity: capacidad)
            defer { dst.deallocate() }
            let n = cuerpo.withUnsafeBufferPointer { src -> Int in
                guard let base = src.baseAddress else { return 0 }
                return compression_decode_buffer(dst, capacidad, base, cuerpo.count, nil, COMPRESSION_ZLIB)
            }
            if n > 0 && n < capacidad { return Data(bytes: dst, count: n) }
            if n > 0 && esperado > 0 && n == esperado { return Data(bytes: dst, count: n) }
            if n == 0 && esperado > 0 { return nil }
            capacidad *= 4                             // no cupo: más espacio
        }
        return nil
        #endif
    }
}

private extension String {
    func leftPadZeros(_ n: Int) -> String {
        count >= n ? self : String(repeating: "0", count: n - count) + self
    }
}
