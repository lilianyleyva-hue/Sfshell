import Foundation

// ComandosTexto.swift — órdenes para texto:
// cat echo printf head tail wc grep sort uniq cut tr sed rev tee diff base64 seq yes nl

extension Consola {
    static func lineas(_ t: String) -> [String] {
        var ls = t.components(separatedBy: "\n")
        if ls.last == "" { ls.removeLast() }
        return ls
    }

    func cmdCat(_ c: Contexto) -> Int32 {
        let (op, resto) = c.opciones()
        guard let ts = c.textos(resto) else { return 1 }
        var n = 0
        for (_, t) in ts {
            if op["n"] != nil {
                for l in Consola.lineas(t) {
                    n += 1
                    c.escribe(String(repeating: " ", count: max(0, 6 - String(n).count)) + "\(n)\t\(l)\n")
                }
            } else {
                c.escribe(t)
            }
        }
        return 0
    }

    func cmdEcho(_ c: Contexto) -> Int32 {
        var a = c.args
        var nueva = true
        var escapes = false
        while let p = a.first, p == "-n" || p == "-e" || p == "-ne" || p == "-en" {
            if p.contains("n") { nueva = false }
            if p.contains("e") { escapes = true }
            a.removeFirst()
        }
        var t = a.joined(separator: " ")
        if escapes { t = Consola.escapes(t) }
        c.escribe(t + (nueva ? "\n" : ""))
        return 0
    }

    static func escapes(_ t: String) -> String {
        return t.replacingOccurrences(of: "\\n", with: "\n").replacingOccurrences(of: "\\t", with: "\t")
            .replacingOccurrences(of: "\\\\", with: "\\")
    }

    /// printf "%s tiene %d años\n" Ana 20
    func cmdPrintf(_ c: Contexto) -> Int32 {
        guard let formato = c.args.first else { c.error("printf: falta el formato\n"); return 1 }
        var valores = Array(c.args.dropFirst())
        let f = Consola.escapes(formato)
        repeat {
            var out = ""
            var i = f.startIndex
            while i < f.endIndex {
                let ch = f[i]
                if ch == "%", f.index(after: i) < f.endIndex {
                    let n = f[f.index(after: i)]
                    i = f.index(i, offsetBy: 2)
                    if n == "%" { out.append("%"); continue }
                    let v = valores.isEmpty ? "" : valores.removeFirst()
                    switch n {
                    case "d", "i": out += String(Int(v) ?? Int(Double(v) ?? 0))
                    case "f": out += String(format: "%f", Double(v) ?? 0)
                    case "x": out += String(Int(v) ?? 0, radix: 16)
                    default: out += v
                    }
                    continue
                }
                out.append(ch)
                i = f.index(after: i)
            }
            c.escribe(out)
        } while !valores.isEmpty && f.contains("%")
        return 0
    }

    func cmdHeadTail(_ c: Contexto, cola: Bool) -> Int32 {
        var args = c.args
        // head -5 = head -n 5
        if let p = args.first, p.count > 1, p.hasPrefix("-"), Int(p.dropFirst()) != nil { args[0] = "-n"; args.insert(String(p.dropFirst()), at: 1) }
        var c2 = c
        c2.args = args
        let (op, resto) = c2.opciones(["n"])
        let n = Int(op["n"] ?? "10") ?? 10
        guard let ts = c.textos(resto) else { return 1 }
        for (nombre, t) in ts {
            if ts.count > 1 { c.escribe("==> \(nombre) <==\n") }
            let ls = Consola.lineas(t)
            let parte = cola ? ls.suffix(n) : ls.prefix(n)
            for l in parte { c.escribe(l + "\n") }
        }
        return 0
    }

    func cmdWc(_ c: Contexto) -> Int32 {
        let (op, resto) = c.opciones()
        let todas = op.isEmpty
        guard let ts = c.textos(resto) else { return 1 }
        var tl = 0, tw = 0, tc = 0
        for (nombre, t) in ts {
            let l = t.filter { $0 == "\n" }.count
            let w = t.split(whereSeparator: { $0 == " " || $0 == "\n" || $0 == "\t" }).count
            let ch = t.utf8.count
            tl += l; tw += w; tc += ch
            c.escribe(fila(l, w, ch, op, todas) + (nombre == "-" ? "" : " " + nombre) + "\n")
        }
        if ts.count > 1 { c.escribe(fila(tl, tw, tc, op, todas) + " total\n") }
        return 0
    }

    private func fila(_ l: Int, _ w: Int, _ ch: Int, _ op: [Character: String], _ todas: Bool) -> String {
        var p: [String] = []
        if todas || op["l"] != nil { p.append(String(l)) }
        if todas || op["w"] != nil { p.append(String(w)) }
        if todas || op["c"] != nil || op["m"] != nil { p.append(String(ch)) }
        return p.map { String(repeating: " ", count: max(0, 7 - $0.count)) + $0 }.joined(separator: " ")
    }

    func cmdGrep(_ c: Contexto) -> Int32 {
        let (op, resto) = c.opciones(["e"])
        var r = resto
        guard let patron = op["e"] ?? (r.isEmpty ? nil : r.removeFirst()) else { c.error("grep: falta el patrón\n"); return 2 }
        var opciones: NSRegularExpression.Options = []
        if op["i"] != nil { opciones.insert(.caseInsensitive) }
        let p = op["F"] != nil ? NSRegularExpression.escapedPattern(for: patron) : (op["w"] != nil ? "\\b" + patron + "\\b" : patron)
        guard let re = try? NSRegularExpression(pattern: p, options: opciones) else { c.error("grep: patrón no válido\n"); return 2 }
        var archivos = r
        if op["r"] != nil || op["R"] != nil {
            archivos = (archivos.isEmpty ? ["."] : archivos).flatMap { esCarpeta($0) ? recorre($0).filter { !esCarpeta($0) } : [$0] }
        }
        guard let ts = c.textos(archivos) else { return 2 }
        var total = 0
        for (nombre, t) in ts {
            var cuenta = 0
            for (k, l) in Consola.lineas(t).enumerated() {
                let si = re.firstMatch(in: l, range: NSRange(l.startIndex..., in: l)) != nil
                if si == (op["v"] == nil) {
                    cuenta += 1
                    if op["c"] != nil || op["l"] != nil || op["q"] != nil { continue }
                    var pre = ts.count > 1 ? nombre + ":" : ""
                    if op["n"] != nil { pre += "\(k + 1):" }
                    c.escribe(pre + l + "\n")
                }
            }
            if op["c"] != nil { c.escribe((ts.count > 1 ? nombre + ":" : "") + "\(cuenta)\n") }
            if op["l"] != nil && cuenta > 0 { c.escribe(nombre + "\n") }
            total += cuenta
        }
        return total > 0 ? 0 : 1
    }

    func cmdSort(_ c: Contexto) -> Int32 {
        let (op, resto) = c.opciones()
        guard let ts = c.textos(resto) else { return 1 }
        var ls = ts.flatMap { Consola.lineas($0.texto) }
        if op["n"] != nil {
            ls.sort { (Double($0.trimmingCharacters(in: .whitespaces)) ?? 0) < (Double($1.trimmingCharacters(in: .whitespaces)) ?? 0) }
        } else if op["f"] != nil {
            ls.sort { $0.lowercased() < $1.lowercased() }
        } else {
            ls.sort()
        }
        if op["r"] != nil { ls.reverse() }
        if op["u"] != nil {
            var vistos = Set<String>()
            ls = ls.filter { vistos.insert($0).inserted }
        }
        for l in ls { c.escribe(l + "\n") }
        return 0
    }

    func cmdUniq(_ c: Contexto) -> Int32 {
        let (op, resto) = c.opciones()
        guard let ts = c.textos(resto) else { return 1 }
        var anterior: String? = nil
        var n = 0
        func suelta() {
            guard let a = anterior else { return }
            if op["d"] != nil && n < 2 { return }
            if op["u"] != nil && n > 1 { return }
            c.escribe((op["c"] != nil ? String(repeating: " ", count: max(0, 7 - String(n).count)) + "\(n) " : "") + a + "\n")
        }
        for l in ts.flatMap({ Consola.lineas($0.texto) }) {
            if l == anterior { n += 1; continue }
            suelta()
            anterior = l
            n = 1
        }
        suelta()
        return 0
    }

    func cmdCut(_ c: Contexto) -> Int32 {
        let (op, resto) = c.opciones(["d", "f", "c"])
        guard let ts = c.textos(resto) else { return 1 }
        let sep = op["d"].flatMap { $0.first } ?? "\t"
        let campos = rango(op["f"] ?? op["c"] ?? "1")
        for l in ts.flatMap({ Consola.lineas($0.texto) }) {
            if op["c"] != nil {
                let cs = Array(l)
                c.escribe(String(campos.compactMap { $0 - 1 < cs.count && $0 >= 1 ? cs[$0 - 1] : nil }) + "\n")
            } else {
                let partes = l.split(separator: sep, omittingEmptySubsequences: false).map(String.init)
                c.escribe(campos.compactMap { $0 - 1 < partes.count && $0 >= 1 ? partes[$0 - 1] : nil }.joined(separator: String(sep)) + "\n")
            }
        }
        return 0
    }

    /// "1,3" "2-4" "3-"
    private func rango(_ s: String) -> [Int] {
        var out: [Int] = []
        for p in s.split(separator: ",") {
            let ab = p.split(separator: "-", omittingEmptySubsequences: false)
            if ab.count == 2 {
                let a = Int(ab[0]) ?? 1
                let b = Int(ab[1]) ?? 1000
                if a <= b { out.append(contentsOf: a ... b) }
            } else if let n = Int(p) {
                out.append(n)
            }
        }
        return out
    }

    func cmdTr(_ c: Contexto) -> Int32 {
        let (op, resto) = c.opciones()
        func conjunto(_ s: String) -> [Character] {
            if s == "[:upper:]" || s == "A-Z" { return Array("ABCDEFGHIJKLMNOPQRSTUVWXYZ") }
            if s == "[:lower:]" || s == "a-z" { return Array("abcdefghijklmnopqrstuvwxyz") }
            if s == "[:digit:]" || s == "0-9" { return Array("0123456789") }
            return Array(Consola.escapes(s))
        }
        guard let a = resto.first else { c.error("tr: faltan los caracteres\n"); return 1 }
        let de = conjunto(a)
        if op["d"] != nil {
            c.escribe(String(c.entrada.filter { !de.contains($0) }))
            return 0
        }
        guard resto.count > 1 else { c.error("tr: falta el segundo conjunto\n"); return 1 }
        let a2 = conjunto(resto[1])
        var mapa: [Character: Character] = [:]
        for (k, ch) in de.enumerated() { mapa[ch] = a2.isEmpty ? ch : a2[min(k, a2.count - 1)] }
        c.escribe(String(c.entrada.map { mapa[$0] ?? $0 }))
        return 0
    }

    /// sed 's/viejo/nuevo/g' (y también /patrón/d y -n 'Np')
    func cmdSed(_ c: Contexto) -> Int32 {
        let (op, resto) = c.opciones(["e"])
        var r = resto
        guard let guion = op["e"] ?? (r.isEmpty ? nil : r.removeFirst()) else { c.error("sed: falta la orden\n"); return 1 }
        guard let ts = c.textos(op["i"] != nil ? r : r) else { return 1 }
        for (nombre, t) in ts {
            guard let res = aplicaSed(guion, t, silencioso: op["n"] != nil) else { c.error("sed: no entiendo «\(guion)»\n"); return 1 }
            if op["i"] != nil && nombre != "-" { escribeArchivo(nombre, res) } else { c.escribe(res) }
        }
        return 0
    }

    private func aplicaSed(_ g: String, _ t: String, silencioso: Bool) -> String? {
        var out = ""
        let ls = Consola.lineas(t)
        if g.hasPrefix("s"), g.count > 3 {
            let sep = g[g.index(after: g.startIndex)]
            let partes = g.dropFirst(2).split(separator: sep, omittingEmptySubsequences: false).map(String.init)
            guard partes.count >= 2, let re = try? NSRegularExpression(pattern: partes[0], options: partes.count > 2 && partes[2].contains("i") ? [.caseInsensitive] : []) else { return nil }
            let global = partes.count > 2 && partes[2].contains("g")
            let nuevo = partes[1].replacingOccurrences(of: "&", with: "$0").replacingOccurrences(of: "\\1", with: "$1").replacingOccurrences(of: "\\2", with: "$2")
            for l in ls {
                let ns = NSMutableString(string: l)
                if global {
                    re.replaceMatches(in: ns, range: NSRange(location: 0, length: ns.length), withTemplate: nuevo)
                } else if let m = re.firstMatch(in: l, range: NSRange(l.startIndex..., in: l)) {
                    ns.replaceCharacters(in: m.range, with: re.replacementString(for: m, in: l, offset: 0, template: nuevo))
                }
                out += (ns as String) + "\n"
            }
            return out
        }
        if g.hasSuffix("d"), g.hasPrefix("/"), let re = try? NSRegularExpression(pattern: String(g.dropFirst().dropLast(2))) {
            for l in ls where re.firstMatch(in: l, range: NSRange(l.startIndex..., in: l)) == nil { out += l + "\n" }
            return out
        }
        if g.hasSuffix("p"), let n = Int(g.dropLast()) {
            for (k, l) in ls.enumerated() {
                if !silencioso { out += l + "\n" }
                if k + 1 == n { out += l + "\n" }
            }
            return out
        }
        if g.hasSuffix("d"), let n = Int(g.dropLast()) {
            for (k, l) in ls.enumerated() where k + 1 != n { out += l + "\n" }
            return out
        }
        return nil
    }

    func cmdRev(_ c: Contexto) -> Int32 {
        guard let ts = c.textos(c.args) else { return 1 }
        for l in ts.flatMap({ Consola.lineas($0.texto) }) { c.escribe(String(l.reversed()) + "\n") }
        return 0
    }

    func cmdTee(_ c: Contexto) -> Int32 {
        let (op, resto) = c.opciones()
        for f in resto { escribeArchivo(f, c.entrada, añadir: op["a"] != nil) }
        c.escribe(c.entrada)
        return 0
    }

    func cmdDiff(_ c: Contexto) -> Int32 {
        guard c.args.count == 2, let a = lee(c.args[0]), let b = lee(c.args[1]) else { c.error("diff: hacen falta dos archivos\n"); return 2 }
        let la = Consola.lineas(a), lb = Consola.lineas(b)
        if la == lb { return 0 }
        // diferencia por subsecuencia común más larga
        let n = la.count, m = lb.count
        var t = Array(repeating: Array(repeating: 0, count: m + 1), count: n + 1)
        if n * m <= 4_000_000 {
            for i in stride(from: n - 1, through: 0, by: -1) {
                for j in stride(from: m - 1, through: 0, by: -1) {
                    t[i][j] = la[i] == lb[j] ? t[i + 1][j + 1] + 1 : max(t[i + 1][j], t[i][j + 1])
                }
            }
        }
        var i = 0, j = 0
        while i < n || j < m {
            if i < n && j < m && la[i] == lb[j] { i += 1; j += 1 }
            else if j < m && (i == n || t[i][j + 1] >= t[i + 1][j]) { c.escribe("> \(lb[j])\n"); j += 1 }
            else { c.escribe("< \(la[i])\n"); i += 1 }
        }
        return 1
    }

    func cmdBase64(_ c: Contexto) -> Int32 {
        let (op, resto) = c.opciones()
        guard let ts = c.textos(resto) else { return 1 }
        let t = ts.map { $0.texto }.joined()
        if op["d"] != nil {
            guard let d = Data(base64Encoded: t.trimmingCharacters(in: .whitespacesAndNewlines)), let s = String(data: d, encoding: .utf8) else { c.error("base64: entrada no válida\n"); return 1 }
            c.escribe(s)
        } else {
            c.escribe(Data(t.utf8).base64EncodedString() + "\n")
        }
        return 0
    }

    func cmdSeq(_ c: Contexto) -> Int32 {
        let n = c.args.compactMap { Double($0) }
        var a = 1.0, paso = 1.0, b = 1.0
        switch n.count {
        case 1: b = n[0]
        case 2: a = n[0]; b = n[1]
        case 3: a = n[0]; paso = n[1]; b = n[2]
        default: c.error("seq: uso: seq [inicio [paso]] fin\n"); return 1
        }
        if paso == 0 { return 1 }
        var x = a
        var k = 0
        while (paso > 0 ? x <= b + 1e-9 : x >= b - 1e-9) && k < 1_000_000 {
            c.escribe((x == x.rounded() ? String(Int(x)) : String(x)) + "\n")
            x += paso
            k += 1
        }
        return 0
    }

    func cmdYes(_ c: Contexto) -> Int32 {
        let t = c.args.isEmpty ? "y" : c.args.joined(separator: " ")
        for _ in 0 ..< 1000 { c.escribe(t + "\n") }
        return 0
    }
}
