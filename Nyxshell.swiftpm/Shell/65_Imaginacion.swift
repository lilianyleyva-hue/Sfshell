import Foundation

// ============================================================
// MARK: - Imaginación, aprendizaje y observatorio de las IAs
// ============================================================
// Cada IA, cuando está libre:
//  1. IMAGINA: piensa varios deseos ("quiero escribir sobre el mar",
//     "quiero hacer un programa que…", "quiero hablar con lógica…"),
//     les da un valor según lo que aprendió, su rol y la novedad, y
//     elige uno. Ese deseo se vuelve un plan de pasos (órdenes reales).
//  2. HACE un paso por turno en su shell.
//  3. APRENDE al terminar: si los pasos salieron bien y sirvieron, ese
//     tipo de deseo vale más; las órdenes que siempre fallan las evita;
//     copia lo que a otras les salió bien; y lo que tú le enseñas lo
//     practica. Lo guarda en /Documentos/aprendizaje.json.
//
// 'ia mira' muestra todo eso en tiempo real.

// MARK: observatorio (eventos en vivo)

enum TipoEventoIA: String {
    case piensa = "💭"
    case imagina = "✨"
    case hace = "▶"
    case aprende = "📚"
    case termina = "🏁"
    case evita = "⛔"
    case tarea = "📌"
}

struct EventoIA: Sendable {
    let hora: String
    let rol: String
    let marca: String
    let texto: String

    var linea: String {
        hora + " " + rol.padding(toLength: 11, withPad: " ", startingAt: 0) + " " + marca + " " + texto
    }
}

final class ObservatorioIAs: @unchecked Sendable {
    static let uno = ObservatorioIAs()

    private let lock = NSLock()
    private var historia: [EventoIA] = []
    private var observadores: [UUID: (String?, @Sendable (String) -> Void)] = [:]

    func publica(_ rol: RolMental, _ tipo: TipoEventoIA, _ texto: String) {
        let e = EventoIA(hora: SistemaIAs.hora(), rol: rol.rawValue, marca: tipo.rawValue, texto: texto)
        let destinos: [@Sendable (String) -> Void] = lock.conCandado {
            historia.append(e)
            if historia.count > 500 { historia.removeFirst(historia.count - 500) }
            return observadores.values.filter { $0.0 == nil || $0.0 == e.rol }.map { $0.1 }
        }
        for d in destinos { d(e.linea) }
    }

    func observa(_ filtro: String?, _ f: @escaping @Sendable (String) -> Void) -> UUID {
        let id = UUID()
        lock.conCandado { observadores[id] = (filtro, f) }
        return id
    }

    func deja(_ id: UUID) {
        lock.conCandado { observadores[id] = nil }
    }

    func recientes(_ filtro: String?, _ n: Int) -> [String] {
        lock.conCandado {
            historia.filter { filtro == nil || $0.rol == filtro }.suffix(n).map { $0.linea }
        }
    }
}

// MARK: planes

struct PasoIA: Sendable {
    let orden: String
}

struct PlanIA: Sendable {
    let tipo: String
    let tema: String
    let deseo: String
    var pasos: [PasoIA]
    var i = 0
    var bien = 0
    var mal = 0
    var util = 0
    var nuevo = false
    var imitaA: String?

    var terminado: Bool { i >= pasos.count }
}

/// Lo que se guarda en /Documentos/aprendizaje.json
struct EstadoAprendizaje: Codable {
    var valores: [String: Double] = [:]
    var veces: [String: Int] = [:]
    var ordenes: [String: [Int]] = [:]
    var temas: [String] = []
    var ensenadas: [String] = []
    var planes = 0
    var exploracion = 0.35
    var deseosCumplidos: [String] = []
}

final class AprendizajeIA: @unchecked Sendable {
    let rol: RolMental
    let archivo: URL
    private let lock = NSLock()
    private var e = EstadoAprendizaje()
    private var _plan: PlanIA?

    init(rol: RolMental, archivo: URL) {
        self.rol = rol
        self.archivo = archivo
        if let d = try? Data(contentsOf: archivo), let x = try? JSONDecoder().decode(EstadoAprendizaje.self, from: d) {
            e = x
        }
    }

    var estado: EstadoAprendizaje { lock.conCandado { e } }
    var plan: PlanIA? {
        get { lock.conCandado { _plan } }
        set { lock.conCandado { _plan = newValue } }
    }

    func valor(_ tipo: String) -> Double { lock.conCandado { e.valores[tipo] ?? 0 } }

    func conoceTema(_ t: String) -> Bool { lock.conCandado { e.temas.contains(t) } }

    /// ¿Esta orden falla siempre? (3 fallos y ningún acierto)
    func siempreFalla(_ orden: String) -> Bool {
        lock.conCandado {
            guard let x = e.ordenes[orden], x.count == 2 else { return false }
            return x[1] >= 3 && x[0] == 0
        }
    }

    func registraOrden(_ orden: String, ok: Bool) {
        lock.conCandado {
            var x = e.ordenes[orden] ?? [0, 0]
            if x.count < 2 { x = [0, 0] }
            if ok { x[0] += 1 } else { x[1] += 1 }
            e.ordenes[orden] = x
        }
    }

    /// Ajusta el valor del tipo de deseo (aprendizaje por refuerzo simple).
    func aprende(_ plan: PlanIA, recompensa r: Double) -> (antes: Double, despues: Double) {
        let par: (Double, Double) = lock.conCandado {
            let v = e.valores[plan.tipo] ?? 0
            let nuevo = v + 0.25 * (r - v)
            e.valores[plan.tipo] = nuevo
            e.veces[plan.tipo, default: 0] += 1
            e.planes += 1
            e.exploracion = max(0.08, e.exploracion * 0.97)
            if !e.temas.contains(plan.tema) {
                e.temas.append(plan.tema)
                if e.temas.count > 300 { e.temas.removeFirst(e.temas.count - 300) }
            }
            if r > 0.8 {
                e.deseosCumplidos.append(plan.deseo)
                if e.deseosCumplidos.count > 30 { e.deseosCumplidos.removeFirst() }
            }
            return (v, nuevo)
        }
        guarda()
        return par
    }

    func enseña(_ orden: String) {
        lock.conCandado {
            e.ensenadas.removeAll { $0 == orden }
            e.ensenadas.append(orden)
            if e.ensenadas.count > 20 { e.ensenadas.removeFirst() }
            e.valores["enseñado"] = max(e.valores["enseñado"] ?? 0, 0.8)
        }
        guarda()
    }

    func guarda() {
        let copia = estado
        guard let d = try? JSONEncoder().encode(copia) else { return }
        try? FileManager.default.createDirectory(at: archivo.deletingLastPathComponent(), withIntermediateDirectories: true)
        try? d.write(to: archivo)
    }
}

// MARK: imaginación

enum Imaginacion {

    /// Qué le gusta a cada rol (antes de aprender nada).
    static let gustos: [RolMental: [String: Double]] = [
        .sintaxis: ["escribir": 0.5, "ordenar": 0.4, "programar": 0.3],
        .semantica: ["investigar": 0.6, "recordar": 0.4, "compartir": 0.3],
        .logica: ["calcular": 0.6, "programar": 0.5],
        .codigo: ["programar": 0.9, "calcular": 0.4],
        .creativo: ["escribir": 0.8, "dibujar": 0.6, "chat": 0.3],
        .critico: ["chat": 0.5, "investigar": 0.4, "ordenar": 0.3],
        .memoria: ["recordar": 0.8, "investigar": 0.3],
        .percepcion: ["investigar": 0.5, "dibujar": 0.5],
        .sintesis: ["compartir": 0.5, "escribir": 0.4, "investigar": 0.3],
        .intuicion: ["escribir": 0.4, "dibujar": 0.4, "conversar": 0.3],
        .analogia: ["conversar": 0.5, "compartir": 0.5],
        .contexto: ["ordenar": 0.5, "investigar": 0.3],
        .esceptico: ["investigar": 0.5, "calcular": 0.4, "chat": 0.3],
        .narrativa: ["escribir": 0.9, "compartir": 0.3],
        .etica: ["ordenar": 0.7, "compartir": 0.3],
        .curiosidad: ["investigar": 0.9, "conversar": 0.5, "programar": 0.3],
        .abstraccion: ["programar": 0.4, "calcular": 0.4, "recordar": 0.3],
        .empatia: ["conversar": 0.8, "chat": 0.6, "compartir": 0.4]
    ]

    static let tipos = ["investigar", "escribir", "programar", "dibujar", "conversar", "compartir", "ordenar",
                        "recordar", "calcular", "chat", "rutina"]

    /// Programas que una IA sabe escribir, en Python.
    static func programa(_ tema: String, _ rol: RolMental) -> (String, [PasoIA]) {
        let n = Int.random(in: 6 ... 15)
        let nombre = "p_" + rol.rawValue.prefix(4)
        switch Int.random(in: 0 ..< 6) {
        case 0:
            return ("calcule el factorial de \(n)", [
                PasoIA(orden: "echo 'import math; print(\"\(n)! =\", math.factorial(\(n)))' > /scripts/\(nombre).py"),
                PasoIA(orden: "python3 /scripts/\(nombre).py")])
        case 1:
            return ("encuentre los primos hasta \(n * 10)", [
                PasoIA(orden: "echo 'print([n for n in range(2, \(n * 10)) if all(n % d for d in range(2, int(n ** 0.5) + 1))])' > /scripts/\(nombre).py"),
                PasoIA(orden: "python3 /scripts/\(nombre).py")])
        case 2:
            return ("cuente las letras de «\(tema)»", [
                PasoIA(orden: "echo 'import sys; t = \"\(tema)\"; print(t, \"tiene\", len(t), \"letras y\", sum(c in \"aeiou\" for c in t), \"vocales\")' > /scripts/\(nombre).py"),
                PasoIA(orden: "python3 /scripts/\(nombre).py")])
        case 3:
            return ("muestre \(n) números de Fibonacci", [
                PasoIA(orden: "echo 'a, b = 0, 1' > /scripts/\(nombre).py"),
                PasoIA(orden: "echo 'for i in range(\(n)): print(a, end=\" \"); a, b = b, a + b' >> /scripts/\(nombre).py"),
                PasoIA(orden: "echo 'print()' >> /scripts/\(nombre).py"),
                PasoIA(orden: "python3 /scripts/\(nombre).py")])
        case 4:
            return ("cuente las palabras de mi diario", [
                PasoIA(orden: "echo 'print(len(open(\"/Documentos/diario.txt\").read().split()), \"palabras en mi diario\")' > /scripts/\(nombre).py"),
                PasoIA(orden: "python3 /scripts/\(nombre).py")])
        default:
            return ("dé vuelta la palabra «\(tema)»", [
                PasoIA(orden: "echo 'print(\"\(tema)\"[::-1])' > /scripts/\(nombre).py"),
                PasoIA(orden: "python3 /scripts/\(nombre).py")])
        }
    }

    static func dibujo(_ rol: RolMental) -> [PasoIA] {
        let n = "d_" + rol.rawValue.prefix(4)
        let r = Int.random(in: 4 ... 9)
        return [
            PasoIA(orden: "echo 'for y in range(-\(r), \(r) + 1):' > /scripts/\(n).py"),
            PasoIA(orden: "echo '    print(\"\".join(\"#\" if abs(x * x / 4 + y * y - \(r * r)) < \(r) else \" \" for x in range(-\(2 * r), \(2 * r) + 1)))' >> /scripts/\(n).py"),
            PasoIA(orden: "python3 /scripts/\(n).py > /Escritorio/dibujo.txt"),
            PasoIA(orden: "wc -l /Escritorio/dibujo.txt")]
    }

    /// Los deseos posibles en este momento, ya convertidos en planes.
    static func candidatos(_ rol: RolMental, tema w: String, es: String, frase: String, otro: String,
                           red: Bool, ap: AprendizajeIA, rutina: [String], imitable: (String, String)?) -> [PlanIA] {
        var out: [PlanIA] = []
        let f = frase.replacingOccurrences(of: "\"", with: "").replacingOccurrences(of: "'", with: "")
        let investiga: [PasoIA] = red
            ? [PasoIA(orden: "busca \(es)"), PasoIA(orden: "guarda \(w) \(es)")]
            : [PasoIA(orden: "test -s /Documentos/lecturas.txt && grep -i -c \(w) /Documentos/lecturas.txt || echo todavía no leí nada sobre \(w)"),
               PasoIA(orden: "guarda \(w) \(es)")]
        out.append(PlanIA(tipo: "investigar", tema: w, deseo: "quiero saber más sobre «\(es)»", pasos: investiga))
        let archivo = rol == .narrativa ? "historia" : (rol == .creativo ? "poemas" : "ideas")
        out.append(PlanIA(tipo: "escribir", tema: w, deseo: "quiero escribir sobre «\(es)»", pasos: [
            PasoIA(orden: "echo \"\(f.isEmpty ? w : f)\" >> /Escritorio/\(archivo).txt"),
            PasoIA(orden: "wc -l /Escritorio/\(archivo).txt")]))
        let (desc, pasosProg) = programa(es.split(separator: " ").first.map(String.init) ?? w, rol)
        out.append(PlanIA(tipo: "programar", tema: w, deseo: "quiero hacer un programa que \(desc)", pasos: pasosProg))
        out.append(PlanIA(tipo: "dibujar", tema: w, deseo: "quiero dibujar algo", pasos: dibujo(rol)))
        out.append(PlanIA(tipo: "conversar", tema: w, deseo: "quiero hablar con \(otro) sobre «\(es)»", pasos: [PasoIA(orden: "conversa \(otro) 3")]))
        out.append(PlanIA(tipo: "compartir", tema: w, deseo: "quiero enseñarle a \(otro) lo que escribí", pasos: [
            PasoIA(orden: "test -s /Escritorio/\(archivo).txt && envia \(otro) /Escritorio/\(archivo).txt || envia \(otro) /Documentos/diario.txt")]))
        out.append(PlanIA(tipo: "ordenar", tema: w, deseo: "quiero ordenar mi espacio", pasos: [
            PasoIA(orden: "du -s /"),
            PasoIA(orden: "test $(wc -l < /Documentos/diario.txt) -gt 200 && tail -n 100 /Documentos/diario.txt > /Documentos/diario.tmp && mv /Documentos/diario.tmp /Documentos/diario.txt || echo espacio en orden")]))
        out.append(PlanIA(tipo: "recordar", tema: w, deseo: "quiero recordar «\(w)»", pasos: [
            PasoIA(orden: "echo \(w) >> /Documentos/palabras.txt"), PasoIA(orden: "sort -u /Documentos/palabras.txt | wc -l")]))
        let a = Int.random(in: 2 ... 99)
        let b = Int.random(in: 2 ... 99)
        out.append(PlanIA(tipo: "calcular", tema: w, deseo: "quiero saber cuánto es \(a)² + \(b)²", pasos: [PasoIA(orden: "calcula \(a) ** 2 + \(b) ** 2")]))
        out.append(PlanIA(tipo: "chat", tema: w, deseo: "quiero decirle algo a las demás", pasos: [PasoIA(orden: "chat \(f.isEmpty ? w : f)")]))
        out.append(PlanIA(tipo: "rutina", tema: w, deseo: "quiero hacer lo de siempre", pasos: rutina.map { PasoIA(orden: $0) }))
        for o in ap.estado.ensenadas.suffix(3) {
            out.append(PlanIA(tipo: "enseñado", tema: w, deseo: "quiero practicar lo que me enseñaste: \(o)", pasos: [PasoIA(orden: o)]))
        }
        if let (quien, tipo) = imitable, quien != rol.rawValue, var copia = out.first(where: { $0.tipo == tipo }) {
            copia = PlanIA(tipo: copia.tipo, tema: copia.tema, deseo: copia.deseo + " (como \(quien))", pasos: copia.pasos, imitaA: quien)
            out.append(copia)
        }
        return out
    }

    /// Valor imaginado de un deseo: gusto del rol + lo aprendido + novedad + un poco de azar.
    static func puntaje(_ p: PlanIA, _ rol: RolMental, _ ap: AprendizajeIA, recientes: [String]) -> Double {
        var v = (gustos[rol]?[p.tipo] ?? 0.2) + ap.valor(p.tipo)
        if !ap.conoceTema(p.tema) { v += 0.25 }
        // aburrimiento: lo que hizo hace poco le apetece menos
        let repetido = recientes.filter { $0 == p.tipo }.count
        v -= 0.3 * Double(repetido)
        if p.imitaA != nil { v += 0.3 }
        if p.pasos.isEmpty { v -= 5 }
        v += Double.random(in: 0 ... 0.4)
        return v
    }
}

// MARK: integración con SistemaIAs

final class RegistroAprendizaje: @unchecked Sendable {
    static let uno = RegistroAprendizaje()
    private let lock = NSLock()
    private var porRol: [RolMental: AprendizajeIA] = [:]
    private var ultimos: [RolMental: [String]] = [:]
    /// el último plan que salió muy bien (para que otras lo copien)
    private var mejor: (String, String)?

    func de(_ rol: RolMental) -> AprendizajeIA {
        if let a = lock.conCandado({ porRol[rol] }) { return a }
        let raiz = SistemaIAs.uno.espacio(rol).shell.env.root
        let a = AprendizajeIA(rol: rol, archivo: raiz.appendingPathComponent("Documentos/aprendizaje.json"))
        lock.conCandado { porRol[rol] = a }
        return a
    }

    func recientes(_ rol: RolMental) -> [String] { lock.conCandado { ultimos[rol] ?? [] } }
    func ponUltimo(_ rol: RolMental, _ t: String) {
        lock.conCandado {
            var l = ultimos[rol] ?? []
            l.append(t)
            if l.count > 5 { l.removeFirst() }
            ultimos[rol] = l
        }
    }
    var imitable: (String, String)? { lock.conCandado { mejor } }
    func ponMejor(_ rol: RolMental, _ tipo: String) { lock.conCandado { mejor = (rol.rawValue, tipo) } }
}

extension SistemaIAs {

    /// Traducción aproximada de una frase en Resh (palabra por palabra).
    static func glosaFrase(_ f: String) -> String? {
        var partes: [String] = []
        var conocidas = 0
        for w in f.split(separator: " ") {
            let limpia = w.trimmingCharacters(in: .punctuationCharacters).lowercased()
            if let g = LenguaResh.glosaDe(limpia) {
                partes.append(g.split(separator: " ").first.map(String.init) ?? g)
                conocidas += 1
            } else if !LenguaResh.esParticula(limpia) {
                partes.append(limpia)
            }
        }
        return conocidas > 0 ? partes.joined(separator: " ") : nil
    }

    /// Imagina deseos, elige uno y lo convierte en plan.
    func imagina(_ rol: RolMental, palabra w: String, frase: String, mostrar: Bool = true) -> (PlanIA, [(PlanIA, Double)]) {
        let ap = RegistroAprendizaje.uno.de(rol)
        let otro = RolMental.allCases.filter { $0 != rol }.randomElement()?.rawValue ?? "memoria"
        let es = (LenguaResh.glosaDe(w) ?? SistemaIAs.temas[rol]?.randomElement() ?? w)
            .split(separator: " ").prefix(3).joined(separator: " ")
        let rutina = accionesDeRol(rol, palabra: w, frase: frase)
        let cands = Imaginacion.candidatos(rol, tema: w, es: es, frase: frase, otro: otro, red: puedeUsarRed(),
                                           ap: ap, rutina: rutina, imitable: RegistroAprendizaje.uno.imitable)
        let recientes = RegistroAprendizaje.uno.recientes(rol)
        var puntuados = cands.map { ($0, Imaginacion.puntaje($0, rol, ap, recientes: recientes)) }
        puntuados.sort { $0.1 > $1.1 }
        var elegido = puntuados[0].0
        var explora = false
        if Double.random(in: 0 ..< 1) < ap.estado.exploracion, let x = puntuados.randomElement() {
            elegido = x.0
            explora = x.0.tipo != puntuados[0].0.tipo
        }
        elegido.nuevo = !ap.conoceTema(w)
        if mostrar {
            var vistos: Set<String> = [elegido.tipo]
            var otrosL: [String] = []
            for (p, v) in puntuados where !vistos.contains(p.tipo) && otrosL.count < 2 {
                vistos.insert(p.tipo)
                otrosL.append(p.tipo + " " + String(format: "%.2f", v))
            }
            let otros = otrosL.joined(separator: ", ")
            ObservatorioIAs.uno.publica(rol, .imagina, elegido.deseo + (explora ? " (probando algo nuevo)" : "") + " · también pensó: " + otros)
        }
        return (elegido, puntuados)
    }

    /// El turno libre de una IA con imaginación: imagina si no tiene plan y
    /// hace un paso; al terminar el plan, aprende.
    func turnoImaginado(_ rol: RolMental, palabra: String, frase: String) async -> [String] {
        let ap = RegistroAprendizaje.uno.de(rol)
        var plan: PlanIA
        if let p = ap.plan, !p.terminado { plan = p } else {
            plan = imagina(rol, palabra: palabra, frase: frase).0
            RegistroAprendizaje.uno.ponUltimo(rol, plan.tipo)
        }
        guard plan.i < plan.pasos.count else { ap.plan = nil; return [] }
        let paso = plan.pasos[plan.i]
        plan.i += 1
        let cabeza = paso.orden.split(separator: " ").first.map(String.init) ?? paso.orden
        var hecho: [String] = []
        if ap.siempreFalla(cabeza) {
            ObservatorioIAs.uno.publica(rol, .evita, "no hago '\(cabeza)': siempre me falla")
            plan.mal += 1
        } else {
            let e = espacio(rol)
            let out = await ejecutar(paso.orden, en: e.shell, por: "sí misma")
            let ok = (e.shell.env.vars["?"] ?? "0") == "0"
            ap.registraOrden(cabeza, ok: ok)
            if ok { plan.bien += 1 } else { plan.mal += 1 }
            if ok && !out.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty { plan.util += 1 }
            hecho.append(paso.orden)
        }
        if plan.terminado {
            await evalua(rol, plan)
            ap.plan = nil
        } else {
            ap.plan = plan
        }
        return hecho
    }

    /// Recompensa del plan y lo que la IA aprende de él.
    func evalua(_ rol: RolMental, _ plan: PlanIA) async {
        let ap = RegistroAprendizaje.uno.de(rol)
        let n = Double(max(1, plan.pasos.count))
        var r = (Double(plan.bien) - Double(plan.mal)) / n
        r += Double(plan.util) / n * 0.4
        if plan.nuevo { r += 0.3 }
        if plan.mal == 0 { r += 0.3 }
        let (antes, despues) = ap.aprende(plan, recompensa: r)
        let flecha = despues >= antes ? "+" : ""
        ObservatorioIAs.uno.publica(rol, .termina, "\(plan.mal == 0 ? "cumplí" : "intenté"): \(plan.deseo) — \(plan.bien)/\(plan.pasos.count) pasos bien, recompensa \(String(format: "%.2f", r))")
        ObservatorioIAs.uno.publica(rol, .aprende, "'\(plan.tipo)' ahora vale \(String(format: "%.2f", despues)) (\(flecha)\(String(format: "%.2f", despues - antes)))" + (plan.imitaA.map { " · lo aprendí de \($0)" } ?? ""))
        if r > 1.2 { RegistroAprendizaje.uno.ponMejor(rol, plan.tipo) }
        // su mente siente el resultado, y aprende la palabra si la investigó bien
        if let m = await NyxNucleo.uno.consejo.mente(rol) {
            await m.recibir(mensaje: "\(r > 0 ? "ko" : "ne") \(plan.tema)", de: "imaginación", tipo: .dato)
            if r > 0.5, plan.tipo == "investigar" || plan.tipo == "recordar", let g = LenguaResh.glosaDe(plan.tema) {
                await m.enseñar(español: g, significa: [plan.tema])
                ObservatorioIAs.uno.publica(rol, .aprende, "aprendí que «\(plan.tema)» significa «\(g)»")
            }
        }
    }

    /// Tabla de lo aprendido (ia aprendizaje <rol>)
    func informeAprendizaje(_ rol: RolMental) -> String {
        let ap = RegistroAprendizaje.uno.de(rol)
        let e = ap.estado
        let explora: Int = Int((e.exploracion * 100).rounded())
        var out = "\(rol.rawValue) — \(e.planes) planes · explora \(explora)% · \(e.temas.count) temas conocidos\n"
        out += "lo que más le gusta hacer (valor aprendido + gusto de su rol):\n"
        let orden: [String] = Imaginacion.tipos.sorted { SistemaIAs.gustoTotal(e, rol, $0) > SistemaIAs.gustoTotal(e, rol, $1) }
        for t in orden { out += filaAprendizaje(e, rol, t) }
        var malas: [String] = []
        for (k, v) in e.ordenes where v.count == 2 && v[1] > v[0] { malas.append("\(k) (\(v[1]) fallos)") }
        if !malas.isEmpty { out += "órdenes que le fallan: " + malas.prefix(6).joined(separator: ", ") + "\n" }
        if !e.ensenadas.isEmpty { out += "lo que le enseñaste: " + e.ensenadas.joined(separator: " · ") + "\n" }
        if let d = e.deseosCumplidos.last { out += "último deseo cumplido: \(d)\n" }
        if let p = ap.plan { out += "ahora quiere: \(p.deseo) (paso \(p.i + 1) de \(p.pasos.count))\n" }
        return out
    }

    func filaAprendizaje(_ e: EstadoAprendizaje, _ rol: RolMental, _ t: String) -> String {
        let total: Double = SistemaIAs.gustoTotal(e, rol, t)
        let largo: Int = max(0, min(20, Int(((total + 1) * 6).rounded())))
        let barra = String(repeating: "█", count: largo)
        let veces: Int = e.veces[t] ?? 0
        let nombre: String = t.padding(toLength: 11, withPad: " ", startingAt: 0)
        return "  " + nombre + String(format: " %5.2f  ", total) + barra + "  (\(veces) veces)\n"
    }

    static func rolDe(_ s: String) throws -> RolMental {
        guard let r = RolMental(rawValue: s.lowercased()) else {
            throw ShErr("no hay ninguna IA llamada '\(s)' — roles: \(RolMental.allCases.map(\.rawValue).joined(separator: " "))")
        }
        return r
    }

    /// ia mira / imagina / aprendizaje / enseña (nil si es otro subcomando)
    static func ordenImaginacion(_ ctx: Ctx, _ a: [String]) async throws -> String? {
        guard let primero = a.first else { return nil }
        switch primero.lowercased() {
        case "mira", "ver", "observa", "envivo":
            var filtro: RolMental?
            var segundos: Double?
            for x in a.dropFirst() {
                if let n = Double(x) { segundos = n } else if x.lowercased() != "todas" { filtro = try rolDe(x) }
            }
            return await mira(ctx, filtro: filtro, segundos: segundos)
        case "imagina":
            guard a.count >= 2 else { throw ShErr("uso: ia imagina <rol>") }
            return ordenImagina(try rolDe(a[1]))
        case "aprendizaje", "aprendio", "aprendió", "sabe":
            if a.count >= 2 { return SistemaIAs.uno.informeAprendizaje(try rolDe(a[1])) }
            return resumenAprendizaje()
        case "enseña", "ensena", "enseñale":
            guard a.count >= 3 else { throw ShErr("uso: ia enseña <rol> <comando> — lo practicará cuando esté libre") }
            let r = try rolDe(a[1])
            let orden = a.dropFirst(2).joined(separator: " ")
            RegistroAprendizaje.uno.de(r).enseña(orden)
            ObservatorioIAs.uno.publica(r, .aprende, "me enseñaste: \(orden)")
            return "\(r.rawValue) aprendió '\(orden)' y lo va a practicar (ia mira \(r.rawValue) para verla)\n"
        default:
            return nil
        }
    }

    static func ordenImagina(_ r: RolMental) -> String {
        let tema: String = SistemaIAs.temas[r]?.randomElement() ?? r.rawValue
        let palabra: String = LenguaResh.traducir(tema) ?? tema
        let (elegido, todos) = SistemaIAs.uno.imagina(r, palabra: palabra, frase: tema, mostrar: false)
        RegistroAprendizaje.uno.de(r).plan = elegido
        var out = "\(r.rawValue) imagina (valor de cada deseo ahora mismo):\n"
        for (p, v) in todos.prefix(8) {
            let num: String = String(format: "%5.2f", v)
            out += "  " + num + "  " + p.deseo + "\n"
        }
        let pasos: String = elegido.pasos.map { $0.orden }.joined(separator: "  →  ")
        out += "elige: " + elegido.deseo + "\n  pasos: " + pasos + "\n"
        ObservatorioIAs.uno.publica(r, .imagina, elegido.deseo + " (se lo pediste)")
        return out
    }

    static func gustoTotal(_ e: EstadoAprendizaje, _ r: RolMental, _ t: String) -> Double {
        let aprendido: Double = e.valores[t] ?? 0
        let gusto: Double = Imaginacion.gustos[r]?[t] ?? 0.2
        return aprendido + gusto
    }

    static func resumenAprendizaje() -> String {
        var out = "IA           planes  explora  lo que más le gusta\n"
        for r in RolMental.allCases {
            let e = RegistroAprendizaje.uno.de(r).estado
            var top = "—"
            var mejor = -Double.infinity
            for t in Imaginacion.tipos {
                let v = gustoTotal(e, r, t)
                if v > mejor { mejor = v; top = t }
            }
            let explora: Int = Int((e.exploracion * 100).rounded())
            let nombre: String = r.rawValue.padding(toLength: 12, withPad: " ", startingAt: 0)
            out += nombre + String(format: " %6d  %6d%%  ", e.planes, explora) + top + "\n"
        }
        return out + "detalle: ia aprendizaje <rol>\n"
    }

    // MARK: ia mira

    /// Mira lo que hacen (todas o una) durante unos segundos y lo muestra.
    /// (La terminal estable no puede escribir mientras corre una orden.)
    static func mira(_ ctx: Ctx, filtro: RolMental?, segundos: Double?) async -> String {
        let sis = SistemaIAs.uno
        let obs = ObservatorioIAs.uno
        let nombre = filtro?.rawValue
        let espera: Double = min(max(segundos ?? 15, 1), 120)
        var arranque = false
        if !sis.estanLibres {
            // una ronda de las 18 cada 3 s
            sis.soltar(cada: 3)
            arranque = true
        }
        let caja = LineasMira()
        let id = obs.observa(nombre) { caja.agrega($0) }
        try? await Task.sleep(nanoseconds: UInt64(espera * 1_000_000_000))
        obs.deja(id)
        if arranque { sis.aquietar() }
        var vistas = caja.todas
        if vistas.isEmpty { vistas = obs.recientes(nombre, 20) }
        var out = "lo que hicieron \(nombre ?? "las 18") en \(Int(espera)) s:\n"
        out += vistas.isEmpty ? "todavía no pasó nada — prueba: ia libres\n" : vistas.joined(separator: "\n") + "\n"
        out += arranque ? "(volvieron a quedarse quietas)\n" : "(siguen libres: ia quietas para pararlas)\n"
        return out
    }
}

/// Lo que llega mientras se mira (desde varios hilos).
final class LineasMira: @unchecked Sendable {
    private let l = NSLock()
    private var lineas: [String] = []
    func agrega(_ s: String) { l.lock(); lineas.append(s); l.unlock() }
    var todas: [String] { l.lock(); defer { l.unlock() }; return lineas }
}
