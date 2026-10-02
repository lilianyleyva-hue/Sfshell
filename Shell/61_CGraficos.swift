import Foundation

// ============================================================
// MARK: - Gráficos e interfaz en texto (graphics.h, ui.h, JOptionPane)
// ============================================================
// La shell es solo texto, así que los programas dibujan en un lienzo
// que se muestra con caracteres de bloque (▀ ▄ █), y sus ventanas,
// botones y campos se dibujan con marcos y se contestan escribiendo.

final class CLienzo {
    let cols: Int
    let filas: Int
    var ancho: Int
    var alto: Int
    var puntos: [Bool]
    var textos: [(Int, Int, String)] = []
    var sucio = false
    var color = 15
    var cx = 0
    var cy = 0

    init(ancho: Int, alto: Int, cols: Int = 72, filas: Int = 24) {
        self.ancho = max(1, ancho)
        self.alto = max(1, alto)
        self.cols = cols
        self.filas = filas
        puntos = [Bool](repeating: false, count: cols * filas * 2)
    }

    var w: Int { cols }
    var h: Int { filas * 2 }

    func limpia() {
        puntos = [Bool](repeating: false, count: w * h)
        textos = []
        sucio = true
    }

    func aPunto(_ x: Double, _ y: Double) -> (Int, Int) {
        (Int((x * Double(w) / Double(ancho)).rounded(.down)), Int((y * Double(h) / Double(alto)).rounded(.down)))
    }

    func pon(_ x: Int, _ y: Int, _ on: Bool = true) {
        guard x >= 0 && y >= 0 && x < w && y < h else { return }
        puntos[y * w + x] = on
        sucio = true
    }

    func pixel(_ x: Double, _ y: Double) {
        let (px, py) = aPunto(x, y)
        pon(px, py, color != 0)
    }

    func linea(_ x1: Double, _ y1: Double, _ x2: Double, _ y2: Double) {
        let (a, b) = aPunto(x1, y1)
        let (c, d) = aPunto(x2, y2)
        var x = a
        var y = b
        let dx = abs(c - a)
        let dy = -abs(d - b)
        let sx = a < c ? 1 : -1
        let sy = b < d ? 1 : -1
        var err = dx + dy
        var pasos = 0
        while pasos < 10000 {
            pon(x, y, color != 0)
            if x == c && y == d { break }
            let e2 = 2 * err
            if e2 >= dy { err += dy; x += sx }
            if e2 <= dx { err += dx; y += sy }
            pasos += 1
        }
    }

    func elipse(_ x: Double, _ y: Double, _ rx: Double, _ ry: Double, desde: Double = 0, hasta: Double = 360, relleno: Bool = false) {
        if relleno {
            let (x0, y0) = aPunto(x - rx, y - ry)
            let (x1, y1) = aPunto(x + rx, y + ry)
            let (ccx, ccy) = aPunto(x, y)
            let a = max(0.5, Double(x1 - x0) / 2)
            let b = max(0.5, Double(y1 - y0) / 2)
            for py in y0...max(y0, y1) {
                for px in x0...max(x0, x1) {
                    let u = (Double(px - ccx)) / a
                    let v = (Double(py - ccy)) / b
                    if u * u + v * v <= 1.0 { pon(px, py, color != 0) }
                }
            }
            return
        }
        let n = max(24, Int((rx + ry) / 2))
        var ant: (Double, Double)?
        for k in 0...n {
            let ang = (desde + (hasta - desde) * Double(k) / Double(n)) * Double.pi / 180
            let p = (x + rx * cos(ang), y - ry * sin(ang))
            if let q = ant { linea(q.0, q.1, p.0, p.1) }
            ant = p
        }
    }

    func caja(_ l: Double, _ t: Double, _ r: Double, _ b: Double, relleno: Bool) {
        if !relleno {
            linea(l, t, r, t)
            linea(r, t, r, b)
            linea(r, b, l, b)
            linea(l, b, l, t)
            return
        }
        let (x0, y0) = aPunto(min(l, r), min(t, b))
        let (x1, y1) = aPunto(max(l, r), max(t, b))
        for py in y0...max(y0, y1) { for px in x0...max(x0, x1) { pon(px, py, color != 0) } }
    }

    func rellena(_ x: Double, _ y: Double) {
        let (sx, sy) = aPunto(x, y)
        guard sx >= 0 && sy >= 0 && sx < w && sy < h, !puntos[sy * w + sx] else { return }
        var pila = [(sx, sy)]
        var cuenta = 0
        while let (px, py) = pila.popLast(), cuenta < w * h {
            guard px >= 0 && py >= 0 && px < w && py < h, !puntos[py * w + px] else { continue }
            puntos[py * w + px] = true
            cuenta += 1
            pila.append((px + 1, py))
            pila.append((px - 1, py))
            pila.append((px, py + 1))
            pila.append((px, py - 1))
        }
        sucio = true
    }

    func texto(_ x: Double, _ y: Double, _ s: String) {
        let col = Int(x * Double(cols) / Double(ancho))
        let fila = Int(y * Double(filas) / Double(alto))
        textos.append((col, fila, s))
        sucio = true
    }

    func dibuja() -> String {
        var lineas: [[Character]] = []
        for f in 0..<filas {
            var l: [Character] = []
            for c in 0..<cols {
                let arriba = puntos[(2 * f) * w + c]
                let abajo = puntos[(2 * f + 1) * w + c]
                l.append(arriba && abajo ? "█" : (arriba ? "▀" : (abajo ? "▄" : " ")))
            }
            lineas.append(l)
        }
        for (c, f, s) in textos where f >= 0 && f < filas {
            for (k, ch) in s.enumerated() where c + k >= 0 && c + k < cols { lineas[f][c + k] = ch }
        }
        let borde = String(repeating: "─", count: cols)
        var out = "┌" + borde + "┐\n"
        for l in lineas { out += "│" + String(l) + "│\n" }
        out += "└" + borde + "┘\n"
        sucio = false
        return out
    }
}

/// Ventana de texto de ui.h: título, líneas, campos y botones.
final class CVentana {
    var titulo = ""
    var lineas: [String] = []
    var campos: [String] = []
    var botones: [String] = []
    var valores: [String] = []
}

extension CMaq {

    static let funcionesGraficas: Set<String> = [
        "initgraph", "initwindow", "closegraph", "cleardevice", "putpixel", "getpixel", "line", "lineto", "moveto",
        "rectangle", "bar", "bar3d", "circle", "ellipse", "fillellipse", "arc", "pieslice", "outtextxy", "outtext",
        "setcolor", "getcolor", "setbkcolor", "setfillstyle", "setlinestyle", "settextstyle", "settextjustify",
        "getmaxx", "getmaxy", "delay", "floodfill", "drawpoly", "fillpoly", "graphresult", "grapherrormsg",
        "setviewport", "textheight", "textwidth", "mostrar", "linea", "circulo", "rectangulo", "punto", "texto",
        "limpia", "lienzo", "espera", "relleno",
        "ui_ventana", "ui_texto", "ui_campo", "ui_boton", "ui_mostrar", "ui_valor", "ui_limpia", "ui_mensaje",
        "ui_pregunta", "ui_confirma", "ui_menu"
    ]

    static let coloresGraficos: [String: Int] = [
        "BLACK": 0, "BLUE": 1, "GREEN": 2, "CYAN": 3, "RED": 4, "MAGENTA": 5, "BROWN": 6, "LIGHTGRAY": 7,
        "DARKGRAY": 8, "LIGHTBLUE": 9, "LIGHTGREEN": 10, "LIGHTCYAN": 11, "LIGHTRED": 12, "LIGHTMAGENTA": 13,
        "YELLOW": 14, "WHITE": 15, "DETECT": 0, "SOLID_FILL": 1, "EMPTY_FILL": 0, "grOk": 0,
        "DEFAULT_FONT": 0, "HORIZ_DIR": 0, "SOLID_LINE": 0
    ]

    func lienzoActivo(_ linea: Int) -> CLienzo {
        if let l = lienzo { return l }
        let l = CLienzo(ancho: 640, alto: 480)
        lienzo = l
        return l
    }

    func muestraLienzo() throws {
        guard let l = lienzo, l.sucio else { return }
        try emite(l.dibuja())
        vuelca()
    }

    func nativaGrafica(_ n: String, _ a: [CV], _ linea: Int) throws -> CV? {
        guard CMaq.funcionesGraficas.contains(n) else { return nil }
        if n.hasPrefix("ui_") { return try nativaUI(n, a, linea) }
        func d(_ k: Int) throws -> Double { try argReal(a, k, n, linea) }
        switch n {
        case "initgraph", "lienzo":
            if n == "lienzo" && a.count >= 2 {
                lienzo = CLienzo(ancho: Int(try d(0)), alto: Int(try d(1)), cols: a.count > 2 ? Int(try d(2)) : 72, filas: a.count > 3 ? Int(try d(3)) : 24)
            } else {
                lienzo = CLienzo(ancho: 640, alto: 480)
            }
            return .vacio
        case "initwindow":
            lienzo = CLienzo(ancho: Int(try d(0)), alto: Int(try d(1)))
            return .n(0, .int)
        case "closegraph", "mostrar":
            try muestraLienzo()
            if n == "closegraph" { lienzo = nil }
            return .vacio
        case "cleardevice", "limpia":
            lienzoActivo(linea).limpia()
            return .vacio
        case "graphresult": return .n(0, .int)
        case "grapherrormsg": return nuevaCadenaC("sin errores")
        case "getmaxx": return .n(Int64(lienzoActivo(linea).ancho - 1), .int)
        case "getmaxy": return .n(Int64(lienzoActivo(linea).alto - 1), .int)
        case "setcolor":
            lienzoActivo(linea).color = Int(try d(0))
            return .vacio
        case "getcolor": return .n(Int64(lienzoActivo(linea).color), .int)
        case "putpixel", "punto":
            let l = lienzoActivo(linea)
            let previo = l.color
            if a.count > 2 { l.color = Int(try d(2)) }
            l.pixel(try d(0), try d(1))
            l.color = previo
            return .vacio
        case "getpixel":
            let l = lienzoActivo(linea)
            let (px, py) = l.aPunto(try d(0), try d(1))
            guard px >= 0 && py >= 0 && px < l.w && py < l.h else { return .n(0, .int) }
            return .n(l.puntos[py * l.w + px] ? 15 : 0, .int)
        case "line", "linea":
            lienzoActivo(linea).linea(try d(0), try d(1), try d(2), try d(3))
            return .vacio
        case "moveto":
            let l = lienzoActivo(linea)
            l.cx = Int(try d(0))
            l.cy = Int(try d(1))
            return .vacio
        case "lineto":
            let l = lienzoActivo(linea)
            let x = try d(0)
            let y = try d(1)
            l.linea(Double(l.cx), Double(l.cy), x, y)
            l.cx = Int(x)
            l.cy = Int(y)
            return .vacio
        case "rectangle", "rectangulo":
            lienzoActivo(linea).caja(try d(0), try d(1), try d(2), try d(3), relleno: false)
            return .vacio
        case "bar", "bar3d":
            lienzoActivo(linea).caja(try d(0), try d(1), try d(2), try d(3), relleno: true)
            return .vacio
        case "circle", "circulo":
            let r = try d(2)
            lienzoActivo(linea).elipse(try d(0), try d(1), r, r)
            return .vacio
        case "ellipse":
            lienzoActivo(linea).elipse(try d(0), try d(1), try d(4), try d(5), desde: try d(2), hasta: try d(3))
            return .vacio
        case "fillellipse":
            lienzoActivo(linea).elipse(try d(0), try d(1), try d(2), try d(3), relleno: true)
            return .vacio
        case "arc":
            let r = try d(4)
            lienzoActivo(linea).elipse(try d(0), try d(1), r, r, desde: try d(2), hasta: try d(3))
            return .vacio
        case "pieslice":
            let l = lienzoActivo(linea)
            let x = try d(0)
            let y = try d(1)
            let ini = try d(2)
            let fin = try d(3)
            let r = try d(4)
            l.elipse(x, y, r, r, desde: ini, hasta: fin)
            l.linea(x, y, x + r * cos(ini * Double.pi / 180), y - r * sin(ini * Double.pi / 180))
            l.linea(x, y, x + r * cos(fin * Double.pi / 180), y - r * sin(fin * Double.pi / 180))
            return .vacio
        case "outtextxy", "texto":
            lienzoActivo(linea).texto(try d(0), try d(1), try cadenaC(try arg(a, 2, n, linea), linea))
            return .vacio
        case "outtext":
            let l = lienzoActivo(linea)
            l.texto(Double(l.cx), Double(l.cy), try cadenaC(try arg(a, 0, n, linea), linea))
            return .vacio
        case "floodfill", "relleno":
            lienzoActivo(linea).rellena(try d(0), try d(1))
            return .vacio
        case "drawpoly", "fillpoly":
            let k = Int(try d(0))
            guard case .p(let p) = try arg(a, 1, n, linea), let mem = p.mem else { return .vacio }
            var pts: [(Double, Double)] = []
            var j = p.i
            while pts.count < k && j + 1 < mem.a.count {
                pts.append((try real(mem.a[j], linea), try real(mem.a[j + 1], linea)))
                j += 2
            }
            let l = lienzoActivo(linea)
            for q in 0..<max(0, pts.count - 1) { l.linea(pts[q].0, pts[q].1, pts[q + 1].0, pts[q + 1].1) }
            if n == "fillpoly", let f = pts.first, let u = pts.last { l.linea(u.0, u.1, f.0, f.1) }
            return .vacio
        case "delay", "espera":
            try muestraLienzo()
            vuelca()
            try duerme(try d(0) / 1000)
            return .vacio
        case "textheight": return .n(8, .int)
        case "textwidth": return .n(Int64(8 * (try cadenaC(try arg(a, 0, n, linea), linea)).count), .int)
        default:
            return .vacio
        }
    }

    // MARK: ui.h

    func textoArg(_ a: [CV], _ k: Int, _ n: String, _ linea: Int) throws -> String {
        let v = try arg(a, k, n, linea)
        if case .p = v { return try cadenaC(v, linea) }
        return try texto(v, linea)
    }

    func nativaUI(_ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        switch n {
        case "ui_ventana":
            let v = CVentana()
            v.titulo = a.isEmpty ? "" : try textoArg(a, 0, n, linea)
            ventana = v
            return .vacio
        case "ui_texto":
            ventanaActiva().lineas.append(try textoArg(a, 0, n, linea))
            return .vacio
        case "ui_campo":
            ventanaActiva().campos.append(try textoArg(a, 0, n, linea))
            return .n(Int64(ventanaActiva().campos.count), .int)
        case "ui_boton":
            ventanaActiva().botones.append(try textoArg(a, 0, n, linea))
            return .n(Int64(ventanaActiva().botones.count), .int)
        case "ui_limpia":
            ventana = CVentana()
            return .vacio
        case "ui_mostrar":
            return .n(Int64(try muestraVentana(ventanaActiva())), .int)
        case "ui_valor":
            let i = Int(try argEnt(a, 0, n, linea)) - 1
            let vals = ventana?.valores ?? []
            let s = i >= 0 && i < vals.count ? vals[i] : ""
            return dialecto == .cpp ? cadena(s) : nuevaCadenaC(s)
        case "ui_mensaje":
            let v = CVentana()
            v.titulo = a.count > 1 ? try textoArg(a, 1, n, linea) : "Mensaje"
            v.lineas = try textoArg(a, 0, n, linea).components(separatedBy: "\n")
            _ = try muestraVentana(v)
            return .vacio
        case "ui_pregunta":
            let v = CVentana()
            v.titulo = "Pregunta"
            v.lineas = [try textoArg(a, 0, n, linea)]
            v.campos = ["›"]
            _ = try muestraVentana(v)
            let s = v.valores.first ?? ""
            return dialecto == .cpp ? cadena(s) : nuevaCadenaC(s)
        case "ui_confirma":
            let v = CVentana()
            v.titulo = "Confirmar"
            v.lineas = [try textoArg(a, 0, n, linea)]
            v.botones = ["Sí", "No"]
            return .n(try muestraVentana(v) == 1 ? 1 : 0, .int)
        case "ui_menu":
            let v = CVentana()
            v.titulo = try textoArg(a, 0, n, linea)
            for k in 1..<a.count { v.botones.append(try textoArg(a, k, n, linea)) }
            return .n(Int64(try muestraVentana(v)), .int)
        default:
            return .vacio
        }
    }

    func ventanaActiva() -> CVentana {
        if let v = ventana { return v }
        let v = CVentana()
        ventana = v
        return v
    }

    /// Dibuja la ventana con un marco y pide los campos y el botón.
    func muestraVentana(_ v: CVentana) throws -> Int {
        var cuerpo: [String] = v.lineas
        for c in v.campos { cuerpo.append(c + " ________") }
        if !v.botones.isEmpty {
            cuerpo.append("")
            cuerpo.append(v.botones.enumerated().map { "[\($0.offset + 1)] \($0.element)" }.joined(separator: "   "))
        }
        let anchoTexto = max(v.titulo.count + 4, cuerpo.map { $0.count }.max() ?? 0, 20)
        var out = "╭─" + (v.titulo.isEmpty ? "" : " \(v.titulo) ") + String(repeating: "─", count: max(0, anchoTexto - (v.titulo.isEmpty ? 0 : v.titulo.count + 2))) + "─╮\n"
        for l in cuerpo { out += "│ " + l + String(repeating: " ", count: max(0, anchoTexto - l.count)) + " │\n" }
        out += "╰" + String(repeating: "─", count: anchoTexto + 2) + "╯\n"
        try emite(out)
        v.valores = []
        for c in v.campos {
            try emite(c + " ")
            vuelca()
            guard let l = entrada.linea() else { break }
            v.valores.append(String(decoding: l, as: UTF8.self))
        }
        guard !v.botones.isEmpty else { return 0 }
        while true {
            try emite("elige (1-\(v.botones.count)): ")
            vuelca()
            guard let l = entrada.linea() else { return 0 }
            let t = String(decoding: l, as: UTF8.self).trimmingCharacters(in: .whitespaces)
            if let k = Int(t), k >= 1 && k <= v.botones.count { return k }
            if let k = v.botones.firstIndex(where: { $0.lowercased() == t.lowercased() }) { return k + 1 }
        }
    }

    // MARK: JOptionPane (Java)

    func joptionPane(_ n: String, _ a: [CV], _ linea: Int) throws -> CV {
        func txt(_ k: Int) throws -> String { k < a.count ? try texto(a[k], linea) : "" }
        let v = CVentana()
        switch n {
        case "showMessageDialog":
            v.titulo = a.count > 2 ? try txt(2) : "Mensaje"
            v.lineas = try txt(1).components(separatedBy: "\n")
            v.lineas.append("")
            v.lineas.append("[ Aceptar ]")
            _ = try muestraVentana(v)
            return .vacio
        case "showInputDialog":
            let conPadre = a.count >= 2 || (a.first?.esNulo ?? false)
            v.titulo = a.count > 2 ? try txt(2) : "Entrada"
            v.lineas = try txt(conPadre ? 1 : 0).components(separatedBy: "\n")
            v.campos = ["›"]
            _ = try muestraVentana(v)
            guard let s = v.valores.first else { return .nulo }
            return cadena(s)
        case "showConfirmDialog":
            v.titulo = a.count > 2 ? try txt(2) : "Selecciona una opción"
            v.lineas = try txt(1).components(separatedBy: "\n")
            v.botones = ["Sí", "No", "Cancelar"]
            let k = try muestraVentana(v)
            return .n(Int64(k == 0 ? 2 : k - 1), .int)
        case "showOptionDialog":
            v.titulo = try txt(2)
            v.lineas = try txt(1).components(separatedBy: "\n")
            if a.count > 6, case .p(let p) = a[6], let mem = p.mem {
                for x in mem.a[p.i...] { v.botones.append(try texto(x, linea)) }
            } else {
                v.botones = ["Sí", "No"]
            }
            return .n(Int64(try muestraVentana(v) - 1), .int)
        default:
            throw CFallo("JOptionPane.\(n) no existe", linea)
        }
    }
}
