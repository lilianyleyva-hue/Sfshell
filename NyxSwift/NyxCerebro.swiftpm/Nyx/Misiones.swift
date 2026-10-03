import Foundation

// Misiones.swift — retos para los tres consejos: cuentas, series, ecuaciones,
// problemas, silogismos, quién es más alto y cuál sobra.
//
// Cada consejo lo enfrenta a su manera:
//  ⚖️ el lógico lo resuelve con sus reglas (y explica los pasos);
//  ✨ en el nuevo, CADA una de las 18 mentes elige una estrategia (calcular con
//     cuidado, probar, ir rápido, estimar, recordar, adivinar), da su respuesta
//     y votan. Después aprenden qué estrategia les funciona en cada tipo de misión;
//  🏛 el antiguo delibera como siempre (y recuerda la solución para la próxima).
// Cuantas más aciertan, más difíciles se ponen (nivel 1 a 5).

enum TipoMision: Int, CaseIterable {
    case cuenta, serie, ecuacion, problema, silogismo, orden, intruso

    var nombre: String {
        switch self {
        case .cuenta: return "cuenta"
        case .serie: return "serie"
        case .ecuacion: return "ecuación"
        case .problema: return "problema"
        case .silogismo: return "silogismo"
        case .orden: return "¿quién es más?"
        case .intruso: return "¿cuál sobra?"
        }
    }

    var icono: String {
        switch self {
        case .cuenta: return "➗"
        case .serie: return "🔢"
        case .ecuacion: return "✖️"
        case .problema: return "🍎"
        case .silogismo: return "🧩"
        case .orden: return "📏"
        case .intruso: return "🔍"
        }
    }
}

struct Mision {
    let tipo: TipoMision
    let enunciado: String
    var respuesta: String?          // nil: la propusiste tú y no se sabe
    var nivel = 1
    var datos: [String] = []        // las frases del acertijo
    var consulta: Hecho? = nil      // lo que pregunta un silogismo
    var opciones: [String] = []     // los nombres o la lista de cosas
    var categoria = ""              // ¿cuál no es un ...?
    var numeros: [Double] = []      // la serie
    var lados: (String, String) = ("", "")   // la ecuación
}

enum Estrategia: Int, CaseIterable {
    case calcula, prueba, rapido, estima, recuerda, adivina

    var nombre: String {
        switch self {
        case .calcula: return "calcular con cuidado"
        case .prueba: return "probar"
        case .rapido: return "ir rápido"
        case .estima: return "estimar"
        case .recuerda: return "recordar"
        case .adivina: return "adivinar"
        }
    }
}

// MARK: - comparar respuestas

enum Respuestas {
    static func norma(_ s: String) -> String {
        var t = s.lowercased()
        for c in ["¿", "?", ".", ",", "¡", "!", "«", "»"] { t = t.replacingOccurrences(of: c, with: "") }
        t = t.trimmingCharacters(in: .whitespaces)
        for p in ["la respuesta es ", "respuesta ", "es ", "creo que "] where t.hasPrefix(p) {
            t = String(t.dropFirst(p.count))
        }
        if t == "si" { t = "sí" }
        return t.trimmingCharacters(in: .whitespaces)
    }

    static func igual(_ a: String, _ b: String) -> Bool {
        let x = norma(a)
        let y = norma(b)
        if let p = Double(x), let q = Double(y) { return abs(p - q) < 1e-6 * max(1, abs(q)) }
        return x == y || ConsejoLogico.singular(x) == ConsejoLogico.singular(y)
    }

    static func numero(_ v: Double?) -> String? {
        guard let x = v, x.isFinite else { return nil }
        return Aritmetica.bonito(x)
    }
}

// MARK: - inventar misiones

enum Misiones {
    private static func r(_ a: Int, _ b: Int) -> Int { a + Azar.ent(b - a + 1) }
    private static func uno<T>(_ xs: [T]) -> T { xs[Azar.ent(xs.count)] }

    static func nueva(_ tipo: TipoMision, nivel: Int) -> Mision {
        let n = max(1, min(5, nivel))
        switch tipo {
        case .cuenta: return cuenta(n)
        case .serie: return serie(n)
        case .ecuacion: return ecuacion(n)
        case .problema: return problema(n)
        case .silogismo: return silogismo(n)
        case .orden: return orden(n)
        case .intruso: return intruso(n)
        }
    }

    private static func cuenta(_ n: Int) -> Mision {
        let e: String
        switch n {
        case 1: e = "\(r(2, 30)) más \(r(2, 30))"
        case 2: e = "\(r(2, 9)) por \(r(2, 9)) más \(r(1, 20))"
        case 3: e = "\(r(3, 12)) por \(r(3, 12)) menos \(r(2, 6)) por \(r(2, 6))"
        case 4: e = "\(r(2, 9)) por (\(r(2, 15)) más \(r(2, 15))) menos \(r(1, 30))"
        default: e = "(\(r(10, 40)) más \(r(2, 20))) por (\(r(8, 15)) menos \(r(1, 7))) más \(r(2, 9)) por \(r(2, 9))"
        }
        let v = Aritmetica.calcula(e)
        return Mision(tipo: .cuenta, enunciado: "¿cuánto es \(e)?", respuesta: Respuestas.numero(v), nivel: n)
    }

    private static func serie(_ n: Int) -> Mision {
        var t: [Double] = []
        let clase = Azar.ent(n)          // nivel 1: solo sumar; nivel 5: de todo
        switch clase {
        case 0:
            let a = r(1, 20), d = r(2, 3 + 3 * n)
            for i in 0 ..< 6 { t.append(Double(a + i * d)) }
        case 1:
            let a = r(1, 4), q = r(2, 3)
            var x = a
            for _ in 0 ..< 6 { t.append(Double(x)); x *= q }
        case 2:
            let k = r(1, 6)
            for i in 0 ..< 6 { t.append(Double((k + i) * (k + i))) }
        case 3:
            var x = r(1, 10), d = r(1, 3)
            for _ in 0 ..< 6 { t.append(Double(x)); x += d; d += 1 }
        default:
            var a = r(1, 3), b = r(2, 5)
            for _ in 0 ..< 6 { t.append(Double(a)); (a, b) = (b, a + b) }
        }
        let vistos = Array(t.prefix(5))
        let texto = vistos.map { Aritmetica.bonito($0) }.joined(separator: ", ")
        var m = Mision(tipo: .serie, enunciado: "¿qué número sigue? \(texto), ?", respuesta: Aritmetica.bonito(t[5]), nivel: n)
        m.numeros = vistos
        return m
    }

    private static func ecuacion(_ n: Int) -> Mision {
        let x = r(-2 + n, 6 + 3 * n)
        let a = r(2, 3 + n), b = r(1, 9 + 3 * n), c = r(2, 4)
        var izq = "", der = ""
        switch n {
        case 1: izq = "x + \(b)"; der = "\(x + b)"
        case 2: izq = "\(a)x + \(b)"; der = "\(a * x + b)"
        case 3: izq = "\(a)x - \(b)"; der = "\(a * x - b)"
        case 4: izq = "\(a)(x + \(b))"; der = "\(a * (x + b))"
        default:
            let k = a + c
            izq = "\(k)x + \(b)"; der = "\(a)x + \(c * x + b)"
        }
        var m = Mision(tipo: .ecuacion, enunciado: "encuentra x: \(izq) = \(der)", respuesta: "\(x)", nivel: n)
        m.lados = (izq, der)
        return m
    }

    private static let nombres = ["Ana", "Beto", "Carla", "Dani", "Eva", "Fede", "Gala", "Hugo"]
    private static let objetos = ["caramelos", "lápices", "cromos", "libros", "globos", "botones"]

    private static func problema(_ n: Int) -> Mision {
        let quien = uno(nombres), cosa = uno(objetos)
        let a = r(3, 9 + 3 * n), b = r(2, 6 + n), c = r(1, 5)
        var e = "", v = 0
        switch Azar.ent(min(5, n + 1)) {
        case 0:
            e = "\(quien) tiene \(a) \(cosa). Compra \(b) y regala \(c). ¿Cuántos \(cosa) tiene ahora?"; v = a + b - c
        case 1:
            e = "Hay \(b) cajas con \(a) \(cosa) cada una. ¿Cuántos \(cosa) hay en total?"; v = a * b
        case 2:
            e = "\(quien) tiene \(a * b) \(cosa) y los reparte entre \(b) amigos. ¿Cuántos \(cosa) le tocan a cada uno?"; v = a
        case 3:
            e = "\(quien) compra \(b) paquetes de \(a) \(cosa) y regala \(c). ¿Cuántos \(cosa) le quedan?"; v = a * b - c
        default:
            e = "\(quien) tiene \(a) \(cosa), gana \(b), pierde \(c) y luego consigue el doble de lo que tiene. ¿Cuántos \(cosa) tiene?"; v = (a + b - c) * 2
        }
        return Mision(tipo: .problema, enunciado: e, respuesta: "\(v)", nivel: n)
    }

    private static let inventadas = ["zorg", "blip", "krano", "tumo", "pliro", "fendo", "garu", "misko", "vadri", "sulo"]

    private static func silogismo(_ n: Int) -> Mision {
        let w = inventadas.shuffled()
        let quien = uno(["Mo", "Lu", "Ki", "Ta", "Ru"])
        let eslabones = 1 + min(n, 4)
        var datos: [String] = []
        for i in 0 ..< eslabones { datos.append("Todos los \(w[i])s son \(w[i + 1])s") }
        datos.append("\(quien) es un \(w[0])")
        if n >= 3 { datos.append("Todos los \(w[8])s son \(w[9])s") }     // una pista que no sirve
        let niega = Azar.f01() < 0.4
        var objetivo = w[1 + Azar.ent(eslabones)]
        if niega {
            datos.append("Ningún \(objetivo) es \(w[7])")
            objetivo = w[7]
        }
        let orden = datos.shuffled()
        var m = Mision(tipo: .silogismo, enunciado: orden.joined(separator: ". ") + ". ¿Es \(quien) un \(objetivo)?",
                       respuesta: niega ? "no" : "sí", nivel: n)
        m.datos = orden
        m.consulta = Hecho(a: quien.lowercased(), rel: .es, b: objetivo)
        return m
    }

    private static func orden(_ n: Int) -> Mision {
        let gente = Array(nombres.shuffled().prefix(2 + min(n, 4)))     // del más alto al más bajo
        var datos: [String] = []
        for i in 0 ..< gente.count - 1 { datos.append("\(gente[i]) es más alto que \(gente[i + 1])") }
        datos.shuffle()
        let i = Azar.ent(gente.count - 1)
        let j = gente.count - 1 - Azar.ent(max(1, gente.count - 1 - i))
        let par = Azar.f01() < 0.5 ? [gente[i], gente[j]] : [gente[j], gente[i]]
        var m = Mision(tipo: .orden, enunciado: datos.joined(separator: ". ") + ". ¿Quién es más alto, \(par[0]) o \(par[1])?",
                       respuesta: gente[i].lowercased(), nivel: n)
        m.datos = datos
        m.opciones = par.map { $0.lowercased() }
        return m
    }

    private static let grupos: [(String, [String], [String])] = [
        ("animal", ["perro", "gato", "ballena", "tiburón", "águila", "pingüino"], ["árbol", "piedra", "mesa"]),
        ("mamífero", ["perro", "gato", "ballena"], ["tiburón", "águila", "pingüino"]),
        ("ser vivo", ["perro", "árbol", "águila", "gato", "ballena"], ["piedra", "mesa", "silla"]),
    ]

    private static func intruso(_ n: Int) -> Mision {
        let g = uno(grupos)
        let raro = uno(g.2)
        let lista = (Array(g.1.shuffled().prefix(3)) + [raro]).shuffled()
        var m = Mision(tipo: .intruso, enunciado: "¿cuál no es un \(g.0): \(lista.joined(separator: ", "))?", respuesta: raro, nivel: n)
        m.opciones = lista
        m.categoria = g.0
        return m
    }

    /// Una misión que escribes tú ("misión: 2, 4, 8, 16, ?").
    static func propia(_ texto: String) -> Mision {
        let t = texto.trimmingCharacters(in: .whitespaces)
        let bajo = t.lowercased()
        if bajo.contains("="), bajo.contains("x") {
            let partes = bajo.replacingOccurrences(of: "encuentra x:", with: "").components(separatedBy: "=")
            var m = Mision(tipo: .ecuacion, enunciado: t, respuesta: nil)
            m.lados = (partes[0].trimmingCharacters(in: .whitespaces), partes[1...].joined().trimmingCharacters(in: .whitespaces))
            return m
        }
        let nums = Resuelve.numerosDe(bajo)
        if nums.count >= 4, bajo.filter({ $0 == "," }).count >= 3 {
            var m = Mision(tipo: .serie, enunciado: t, respuesta: nil)
            m.numeros = nums
            return m
        }
        if Aritmetica.calcula(bajo) != nil { return Mision(tipo: .cuenta, enunciado: t, respuesta: nil) }
        let frases = t.components(separatedBy: ".").map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
        if bajo.contains("cuál no es"), let dos = bajo.range(of: ":") {
            var m = Mision(tipo: .intruso, enunciado: t, respuesta: nil)
            let antes = bajo[..<dos.lowerBound].replacingOccurrences(of: "¿", with: "")
            m.categoria = ConsejoLogico.frase(Palabras.tokens(antes.replacingOccurrences(of: "cuál no es", with: "")))
            m.opciones = bajo[dos.upperBound...].replacingOccurrences(of: "?", with: "")
                .components(separatedBy: ",").map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
            return m
        }
        if bajo.contains("quién es más") || bajo.contains("quien es más") {
            var m = Mision(tipo: .orden, enunciado: t, respuesta: nil)
            m.datos = frases.filter { !$0.contains("?") }
            if let q = frases.last, let coma = q.range(of: ",") {
                m.opciones = q[coma.upperBound...].replacingOccurrences(of: "?", with: "").lowercased()
                    .components(separatedBy: " o ").map { $0.trimmingCharacters(in: .whitespaces) }
            }
            return m
        }
        if bajo.contains("todos") || bajo.contains("ningún") || bajo.contains("ninguno") {
            var m = Mision(tipo: .silogismo, enunciado: t, respuesta: nil)
            m.datos = frases.filter { !$0.contains("?") }
            return m
        }
        return Mision(tipo: .problema, enunciado: t, respuesta: nil)
    }
}

// MARK: - resolver (el lógico, y las estrategias de las mentes nuevas)

extension Aritmetica {
    /// Evalúa una expresión con símbolos ("3*(2)+5").
    static func evalua(_ e: String) -> Double? {
        let limpio = Array(e.filter { $0 != " " })
        if limpio.isEmpty { return nil }
        var p = Parser(limpio)
        guard let v = p.suma(), p.i == p.c.count else { return nil }
        return v
    }

    /// Sin respetar prioridades: de izquierda a derecha (el error típico).
    static func izquierdaDerecha(_ t: String) -> Double? {
        let e = expresion(t).replacingOccurrences(of: "(", with: "").replacingOccurrences(of: ")", with: "")
        var total: Double? = nil
        var op: Character = "+"
        var num = ""
        func aplica() {
            guard let v = Double(num) else { return }
            let a = total ?? 0
            switch op {
            case "-": total = a - v
            case "*": total = a * v
            case "/": total = v == 0 ? a : a / v
            default: total = a + v
            }
            num = ""
        }
        for ch in e where ch != " " {
            if ch.isNumber || ch == "." { num.append(ch) } else { aplica(); op = ch }
        }
        aplica()
        return total
    }
}

enum Resuelve {
    static func numerosDe(_ t: String) -> [Double] {
        var out: [Double] = []
        var s = ""
        for ch in t + " " {
            if ch.isNumber || (ch == "-" && s.isEmpty) || (ch == "." && !s.isEmpty) {
                s.append(ch)
            } else {
                if let v = Double(s) { out.append(v) }
                s = ""
            }
        }
        return out
    }

    // series

    static func serie(_ n: [Double]) -> (Double, String)? {
        guard n.count >= 3 else { return nil }
        var d: [Double] = []
        for i in 1 ..< n.count { d.append(n[i] - n[i - 1]) }
        if Set(d).count == 1 { return (n.last! + d[0], "siempre suma \(Aritmetica.bonito(d[0]))") }
        if !n.contains(0) {
            var q: [Double] = []
            for i in 1 ..< n.count { q.append(n[i] / n[i - 1]) }
            if Set(q).count == 1 { return (n.last! * q[0], "siempre multiplica por \(Aritmetica.bonito(q[0]))") }
        }
        var fib = true
        for i in 2 ..< n.count where n[i] != n[i - 1] + n[i - 2] { fib = false }
        if fib { return (n[n.count - 1] + n[n.count - 2], "cada uno es la suma de los dos anteriores") }
        var dd: [Double] = []
        for i in 1 ..< d.count { dd.append(d[i] - d[i - 1]) }
        if Set(dd).count == 1 {
            let sig = d.last! + dd[0]
            return (n.last! + sig, "lo que suma crece de \(Aritmetica.bonito(dd[0])) en \(Aritmetica.bonito(dd[0])): ahora suma \(Aritmetica.bonito(sig))")
        }
        return nil
    }

    // ecuaciones

    static func conX(_ lado: String, _ x: Double) -> Double? {
        var e = ""
        var antes: Character = " "
        for ch in lado.lowercased() where ch != " " {
            if ch == "x" {
                if antes.isNumber || antes == ")" { e.append("*") }
                e += "(\(x))"
            } else {
                if ch == "(" && (antes.isNumber || antes == "x") { e.append("*") }
                e.append(ch)
            }
            antes = ch
        }
        return Aritmetica.evalua(e)
    }

    static func ecuacion(_ l: (String, String)) -> (Double, String)? {
        guard let a0 = conX(l.0, 0), let b0 = conX(l.1, 0), let a1 = conX(l.0, 1), let b1 = conX(l.1, 1) else { return nil }
        let f0 = a0 - b0
        let pendiente = (a1 - b1) - f0
        if pendiente == 0 { return nil }
        let x = -f0 / pendiente
        return (x, "paso todo a un lado: cada x vale \(Aritmetica.bonito(pendiente)) y sin x queda \(Aritmetica.bonito(f0)), así que x = \(Aritmetica.bonito(x))")
    }

    static func pruebaEcuacion(_ l: (String, String)) -> Double? {
        for x in -60 ... 150 {
            guard let a = conX(l.0, Double(x)), let b = conX(l.1, Double(x)) else { return nil }
            if abs(a - b) < 1e-9 { return Double(x) }
        }
        return nil
    }

    // problemas con palabras

    static let mas: Set<String> = ["compra", "compran", "gana", "ganan", "recibe", "encuentra", "consigue", "añade", "suma", "más", "junta"]
    static let menos: Set<String> = ["regala", "pierde", "come", "gasta", "vende", "da", "quita", "menos", "rompe"]
    static let grupos: Set<String> = ["cajas", "paquetes", "bolsas", "grupos", "filas", "equipos"]

    static func problema(_ t: String) -> (Double, String)? {
        var total: Double? = nil
        var op = "+"
        var pasos: [String] = []
        var vioGrupo = false
        for w in Palabras.tokens(t.lowercased(), max: 80) {
            if let v = Double(w) {
                guard let x = total else { total = v; pasos.append(Aritmetica.bonito(v)); continue }
                var r = x
                switch op {
                case "-": r = x - v
                case "*": r = x * v
                case "/": r = v == 0 ? x : x / v
                default: r = x + v
                }
                pasos.append("\(op == "*" ? "×" : (op == "/" ? "÷" : op)) \(Aritmetica.bonito(v))")
                total = r
                op = "+"
                vioGrupo = false
                continue
            }
            if total == nil { continue }
            if grupos.contains(w) { vioGrupo = true }
            if vioGrupo && (w == "con" || w == "de") { op = "*" }
            if w == "entre" { op = "/" }
            if mas.contains(w) { op = "+" }
            if menos.contains(w) { op = "-" }
            if let x = total, w == "doble" || w == "triple" || w == "mitad" {
                total = w == "doble" ? x * 2 : (w == "triple" ? x * 3 : x / 2)
                pasos.append(w == "doble" ? "× 2" : (w == "triple" ? "× 3" : "÷ 2"))
            }
        }
        guard let v = total, pasos.count >= 2 else { return nil }
        return (v, pasos.joined(separator: " ") + " = " + Aritmetica.bonito(v))
    }

    // acertijos de lógica

    static func silogismo(_ m: Mision, hasta: Int = 7) -> (String, String)? {
        let l = ConsejoLogico(primer: false)
        for d in m.datos { l.lee(d, quien: "misión") }
        guard let q = m.consulta ?? preguntaDe(m.enunciado, l) else { return nil }
        if hasta < 7 {
            if l.camino(q.a, .es, q.b, hasta: hasta) != nil { return ("sí", "lo encontré en pocos pasos") }
            return ("no", "no lo encontré en pocos pasos")
        }
        let r = l.prueba(q)
        if r.veredicto != "sí" && r.veredicto != "no" { return nil }
        return (r.veredicto, r.explicacion.joined(separator: " ⇒ "))
    }

    private static func preguntaDe(_ enunciado: String, _ l: ConsejoLogico) -> Hecho? {
        guard let q = enunciado.components(separatedBy: ".").last(where: { $0.contains("?") }) else { return nil }
        var toks = Palabras.tokens(q.lowercased(), max: 30)
        if let v = toks.first, ["es", "son", "tiene", "puede"].contains(v), toks.count >= 3 {
            // "¿es mo un krano?" → "mo es un krano"
            toks = [toks[1], v] + Array(toks.dropFirst(2))
        }
        return l.hechoDe(toks)
    }

    static func orden(_ m: Mision) -> (String, String)? {
        guard m.opciones.count == 2 else { return nil }
        let l = ConsejoLogico(primer: false)
        for d in m.datos { l.lee(d, quien: "misión") }
        let a = m.opciones[0], b = m.opciones[1]
        if let c = l.camino(a, .mayor, b) { return (a, c.joined(separator: " > ")) }
        if let c = l.camino(b, .mayor, a) { return (b, c.joined(separator: " > ")) }
        return nil
    }

    static func intruso(_ m: Mision, _ l: ConsejoLogico) -> (String, String)? {
        let cat = ConsejoLogico.singular(m.categoria)
        let fuera = m.opciones.filter { l.demuestraEs(ConsejoLogico.singular($0), cat) == nil }
        guard fuera.count == 1 else { return nil }
        let x = fuera[0]
        let es = l.ancestros(ConsejoLogico.singular(x))
        return (x, es.isEmpty ? "de \(x) no sé que sea \(cat); de los demás, sí" : "\(x) es \(es.joined(separator: ", ")), no \(cat)")
    }

    /// Lo que contesta el consejo lógico (y por qué).
    static func logico(_ m: Mision, _ l: ConsejoLogico) -> (String, String)? {
        switch m.tipo {
        case .cuenta:
            guard let v = Aritmetica.calcula(m.enunciado) else { return nil }
            return (Aritmetica.bonito(v), Aritmetica.expresion(m.enunciado) + " = " + Aritmetica.bonito(v) + " (primero × y ÷, luego + y −)")
        case .serie:
            return serie(m.numeros).map { (Aritmetica.bonito($0.0), $0.1) }
        case .ecuacion:
            return ecuacion(m.lados).map { (Aritmetica.bonito($0.0), $0.1) }
        case .problema:
            return problema(m.enunciado).map { (Aritmetica.bonito($0.0), $0.1) }
        case .silogismo:
            return silogismo(m)
        case .orden:
            return orden(m)
        case .intruso:
            return intruso(m, l)
        }
    }
}
