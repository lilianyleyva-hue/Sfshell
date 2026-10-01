import Foundation

// ============================================================
// MARK: - Huella: la IA propia de Sfshell
// ============================================================
// No usa internet, ni tokens, ni redes neuronales, ni textos de
// entrenamiento. Huella solo sabe lo que ha VISTO pasar en esta
// terminal. Para ella, entender una frase es saber qué cambio
// produce en el mundo.
//
// CÓMO LEE — parte tu frase en piezas. Las piezas que también
//   aparecen dentro de las órdenes son HUECOS (nombres, valores);
//   las demás son ANCLAS (la forma fija de lo que pides).
//
// CÓMO VE — mira el PAISAJE de la terminal (archivos, carpetas,
//   variables y lugar) antes y después de actuar. La diferencia
//   son ACONTECIMIENTOS: creado, cambiado, borrado, variable…
//
// CÓMO PIENSA — cuando le pides algo busca la intención cuyas
//   anclas encajan con tu frase, rellena los huecos con tus
//   palabras y propone un plan. Después de hacerlo MIRA si pasó lo
//   que esperaba: si sí, gana confianza; si no, la pierde.
//
// CÓMO SUEÑA — 'huella sueña' junta las intenciones que hacen lo
//   mismo (así aprende otras formas de pedir algo) y olvida las
//   que siempre fallan.
//
// huella <frase>         pide algo
// huella si | no         acepta o rechaza el plan propuesto
// huella listo           termina de enseñarle
// huella deja            cancela la enseñanza
// huella mira            lo que ve ahora
// huella sabe            lo que ha aprendido
// huella sueña           ordena su memoria
// huella olvida <n|todo> borra lo aprendido
// Entiende pedidos compuestos: "crea notas con hola y luego bórralo"
// si sabe hacer cada parte. Lo que ve también se lo cuenta a Nyx.
// Simulacro: Huella/frase
// Memoria: /var/huella/memoria.json (se puede leer con cat)

// ---------- lo que guarda ----------

/// Un trozo de la forma de una frase.
enum HuellaTrozo: Codable, Hashable {
    case ancla(String)   // pieza fija: "crea", "archivo", "con"
    case hueco(Int)      // pieza que cambia: un nombre, un valor
}

/// Un cambio que Huella vio en el mundo.
struct HuellaHecho: Codable, Hashable {
    let tipo: String     // creado, cambiado, borrado, carpeta, sincarpeta, variable, lugar
    let que: String
    var texto: String { "\(tipo) \(que)" }
}

/// Algo que Huella sabe hacer.
struct HuellaIntencion: Codable {
    var formas: [[HuellaTrozo]]            // maneras de pedirlo
    var plan: [String]               // órdenes, con ⟨0⟩ ⟨1⟩… en los huecos
    var espera: [HuellaHecho]     // lo que debe pasar, también con huecos
    var ejemplo: String              // la frase con que se enseñó
    var aciertos = 0
    var fallos = 0
    /// Empieza en 50 % y se mueve con cada acierto o fallo.
    var confianza: Double { Double(aciertos + 1) / Double(aciertos + fallos + 2) }
}

struct HuellaMemoria: Codable {
    var intenciones: [HuellaIntencion] = []
}

struct HuellaPropuesta {
    let indices: [Int]      // una intención, o varias encadenadas con "y"
    let frase: String
    let ordenes: [String]
    let espera: [HuellaHecho]
}

/// Lo que Huella ve en un momento dado.
struct HuellaPaisaje {
    struct Marca: Equatable { let tam: Int; let fecha: Double }
    var archivos: [String: Marca] = [:]
    var carpetas: Set<String> = []
    var vars: [String: String] = [:]
    var lugar = ""
}

final class HuellaMente: @unchecked Sendable {
    static let una = HuellaMente()
    var memoria = HuellaMemoria()
    var cargada = false
    var observando: (frase: String, ordenes: [String], vistos: [HuellaHecho])?
    var propuesta: HuellaPropuesta?
}

// ============================================================
// MARK: - El funcionamiento
// ============================================================

enum HuellaIA {
    static let ruta = "/var/huella/memoria.json"
    static let umbral = 0.55          // nota mínima para atreverse a proponer
    static let maxVistos = 6000       // tope de archivos que mira de una vez
    /// Variables que cambian solas y no cuentan como acontecimiento.
    static let varsRuido: Set<String> = ["?", "PWD", "OLDPWD", "IT", "_", "RANDOM"]

    // ---------- memoria ----------

    static func carga(_ env: ShellEnv) {
        let m = HuellaMente.una
        guard !m.cargada else { return }
        m.cargada = true
        if let u = try? env.resolve(ruta),
           let d = FileManager.default.contents(atPath: u.path),
           let mem = try? JSONDecoder().decode(HuellaMemoria.self, from: d) {
            m.memoria = mem
        }
    }

    static func guarda(_ env: ShellEnv) {
        guard let u = try? env.resolve(ruta) else { return }
        try? FileManager.default.createDirectory(at: u.deletingLastPathComponent(),
                                                 withIntermediateDirectories: true)
        let enc = JSONEncoder()
        enc.outputFormatting = [.prettyPrinted, .sortedKeys]
        if let d = try? enc.encode(HuellaMente.una.memoria) { try? d.write(to: u) }
    }

    // ---------- cómo ve ----------

    /// Mira el paisaje entero del sandbox. Es síncrona a propósito:
    /// el enumerador de archivos no se puede recorrer en código async.
    static func mira(_ env: ShellEnv) -> HuellaPaisaje {
        var p = HuellaPaisaje()
        p.lugar = env.vpath(env.cwd)
        for (k, v) in env.vars where !varsRuido.contains(k) { p.vars[k] = v }

        let claves: [URLResourceKey] = [.isDirectoryKey, .fileSizeKey, .contentModificationDateKey]
        guard let e = FileManager.default.enumerator(at: env.root, includingPropertiesForKeys: claves,
                                                     options: [.skipsHiddenFiles]) else { return p }
        var n = 0
        while let u = e.nextObject() as? URL {
            n += 1
            if n > maxVistos { break }
            let v = env.vpath(u)
            if v.hasPrefix("/var/huella") { continue }   // no se mira a sí misma
            let r = try? u.resourceValues(forKeys: Set(claves))
            if r?.isDirectory == true {
                p.carpetas.insert(v)
            } else {
                p.archivos[v] = HuellaPaisaje.Marca(tam: r?.fileSize ?? 0,
                                              fecha: r?.contentModificationDate?.timeIntervalSince1970 ?? 0)
            }
        }
        return p
    }

    /// Lo que cambió entre dos paisajes.
    static func cambios(_ a: HuellaPaisaje, _ b: HuellaPaisaje) -> [HuellaHecho] {
        var r: [HuellaHecho] = []
        for (k, v) in b.archivos {
            if let o = a.archivos[k] {
                if o != v { r.append(HuellaHecho(tipo: "cambiado", que: k)) }
            } else {
                r.append(HuellaHecho(tipo: "creado", que: k))
            }
        }
        for k in a.archivos.keys where b.archivos[k] == nil {
            r.append(HuellaHecho(tipo: "borrado", que: k))
        }
        for k in b.carpetas.subtracting(a.carpetas) { r.append(HuellaHecho(tipo: "carpeta", que: k)) }
        for k in a.carpetas.subtracting(b.carpetas) { r.append(HuellaHecho(tipo: "sincarpeta", que: k)) }
        for (k, v) in b.vars where a.vars[k] != v {
            r.append(HuellaHecho(tipo: "variable", que: "\(k)=\(v)"))
        }
        if a.lugar != b.lugar { r.append(HuellaHecho(tipo: "lugar", que: b.lugar)) }
        return r.sorted { $0.texto < $1.texto }
    }

    // ---------- cómo lee ----------

    static let puntuacion = CharacterSet(charactersIn: "¿?¡!.,;:\"'«»()")

    /// MEJORA: lee la forma de la palabra, no su ropa. "Creá", "crea" y
    /// "CREA" son la misma ancla; "archivos" y "archivo" también.
    static func norma(_ w: String) -> String {
        var r = w.lowercased().folding(options: .diacriticInsensitive, locale: nil)
        if r.count > 3, r.hasSuffix("s") { r.removeLast() }
        return r
    }

    static func piezas(_ frase: String) -> [String] {
        frase.split(whereSeparator: { $0.isWhitespace })
            .map { String($0).trimmingCharacters(in: puntuacion) }
            .filter { !$0.isEmpty }
    }

    /// Un borde es limpio si no hay letra ni número pegado:
    /// así "nota" no se confunde con un trozo de "notable".
    static func bordeLimpio(_ c: Character?) -> Bool {
        guard let c else { return true }
        return !(c.isLetter || c.isNumber)
    }

    static func reemplazaLimpio(_ texto: String, _ buscar: String, _ por: String) -> String {
        guard !buscar.isEmpty else { return texto }
        var res = ""
        var i = texto.startIndex
        while i < texto.endIndex {
            guard let r = texto.range(of: buscar, range: i..<texto.endIndex) else {
                res += texto[i...]
                break
            }
            let antes: Character? = r.lowerBound == texto.startIndex ? nil : texto[texto.index(before: r.lowerBound)]
            let despues: Character? = r.upperBound == texto.endIndex ? nil : texto[r.upperBound]
            res += texto[i..<r.lowerBound]
            res += (bordeLimpio(antes) && bordeLimpio(despues)) ? por : String(texto[r])
            i = r.upperBound
        }
        return res
    }

    static func contieneLimpio(_ texto: String, _ s: String) -> Bool {
        reemplazaLimpio(texto, s, "\u{1}") != texto
    }

    static func temporal(_ k: Int) -> String {
        String(Character(UnicodeScalar(UInt32(0xE000 + k))!))   // carácter privado: nunca choca con tu texto
    }

    // ---------- cómo aprende ----------

    /// Convierte una demostración en una intención.
    static func aprende(_ frase: String, _ ordenes: [String], _ vistos: [HuellaHecho]) -> (HuellaIntencion, [String]) {
        let p = piezas(frase)
        let todo = ordenes.joined(separator: "\n")
        var forma: [HuellaTrozo] = []
        var valores: [String] = []

        // 1. leer: el trozo más largo de la frase que aparece en las órdenes es un hueco
        var i = 0
        while i < p.count {
            var hallado: (fin: Int, valor: String)?
            var j = p.count
            while j > i {
                let v = p[i..<j].joined(separator: " ")
                if v.count >= 2, contieneLimpio(todo, v) { hallado = (j, v); break }
                j -= 1
            }
            if let h = hallado {
                if let k = valores.firstIndex(of: h.valor) {
                    forma.append(.hueco(k))
                } else {
                    valores.append(h.valor)
                    forma.append(.hueco(valores.count - 1))
                }
                i = h.fin
            } else {
                forma.append(.ancla(p[i].lowercased()))
                i += 1
            }
        }

        // 2. marcar los huecos en el plan y en lo visto (los valores largos primero)
        let porLargo = valores.indices.sorted { valores[$0].count > valores[$1].count }
        func marca(_ s: String) -> String {
            var r = s
            for k in porLargo { r = reemplazaLimpio(r, valores[k], temporal(k)) }
            return r
        }
        let planT = ordenes.map(marca)
        let esperaT = vistos.map { HuellaHecho(tipo: $0.tipo, que: marca($0.que)) }

        // 3. numerar los huecos según aparecen en el plan: así dos frases con
        //    distinto orden de palabras producen el mismo plan y se pueden juntar
        var nuevo: [Int: Int] = [:]
        for ch in planT.joined() {
            for k in valores.indices where nuevo[k] == nil && String(ch) == temporal(k) {
                nuevo[k] = nuevo.count
            }
        }
        func final(_ s: String) -> String {
            var r = s
            for k in valores.indices { r = r.replacingOccurrences(of: temporal(k), with: "⟨\(nuevo[k] ?? k)⟩") }
            return r
        }
        let formaF = forma.map { t -> HuellaTrozo in
            if case .hueco(let k) = t { return .hueco(nuevo[k] ?? k) }
            return t
        }
        var valoresF = Array(repeating: "", count: valores.count)
        for k in valores.indices { valoresF[nuevo[k] ?? k] = valores[k] }

        let it = HuellaIntencion(formas: [formaF],
                           plan: planT.map(final),
                           espera: esperaT.map { HuellaHecho(tipo: $0.tipo, que: final($0.que)) },
                           ejemplo: frase)
        return (it, valoresF)
    }

    // ---------- cómo entiende ----------

    /// Encaja una frase nueva en una forma conocida. Devuelve los
    /// valores de los huecos y una nota de 0 a 1.
    static func encaja(_ forma: [HuellaTrozo], _ b: [String]) -> (valores: [Int: String], nota: Double)? {
        let anclas: [(fi: Int, w: String)] = forma.enumerated().compactMap { i, t in
            if case .ancla(let w) = t { return (i, norma(w)) }
            return nil
        }
        let bl = b.map(norma)
        let n = anclas.count, m = bl.count

        // la secuencia más larga de anclas que aparecen en orden en la frase nueva
        var dp = Array(repeating: Array(repeating: 0, count: m + 1), count: n + 1)
        if n > 0 && m > 0 {
            for i in stride(from: n - 1, through: 0, by: -1) {
                for j in stride(from: m - 1, through: 0, by: -1) {
                    dp[i][j] = anclas[i].w == bl[j] ? dp[i + 1][j + 1] + 1 : max(dp[i + 1][j], dp[i][j + 1])
                }
            }
        }
        var pares: [(f: Int, b: Int)] = [(-1, -1)]
        var i = 0, j = 0
        while i < n && j < m {
            if anclas[i].w == bl[j] { pares.append((anclas[i].fi, j)); i += 1; j += 1 }
            else if dp[i + 1][j] >= dp[i][j + 1] { i += 1 }
            else { j += 1 }
        }
        let encajadas = pares.count - 1
        pares.append((forma.count, b.count))

        // lo que queda entre dos anclas encajadas va a los huecos de ese tramo
        var valores: [Int: String] = [:]
        var sobrantes = 0
        for t in 0..<(pares.count - 1) {
            let (fa, ba) = pares[t], (fb, bb) = pares[t + 1]
            let huecos: [Int] = (fa + 1 < fb ? Array(forma[(fa + 1)..<fb]) : []).compactMap {
                if case .hueco(let k) = $0 { return k }
                return nil
            }
            let palabras: [String] = ba + 1 < bb ? Array(b[(ba + 1)..<bb]) : []
            if huecos.isEmpty { sobrantes += palabras.count; continue }
            guard palabras.count >= huecos.count else { return nil }   // falta algún valor
            for (x, k) in huecos.enumerated() {
                let v = x == huecos.count - 1
                    ? palabras[x...].joined(separator: " ")
                    : palabras[x]
                if valores[k] == nil { valores[k] = v }
            }
        }
        let parte = n == 0 ? 0.4 : Double(encajadas) / Double(n)
        return (valores, max(0, parte - 0.07 * Double(sobrantes)))
    }

    static func entiende(_ frase: String) -> (indice: Int, valores: [Int: String], nota: Double)? {
        let p = piezas(frase)
        var mejor: (indice: Int, valores: [Int: String], nota: Double)?
        for (i, it) in HuellaMente.una.memoria.intenciones.enumerated() {
            for forma in it.formas {
                guard let l = encaja(forma, p) else { continue }
                let total = l.nota * 0.85 + it.confianza * 0.15
                if total > (mejor?.nota ?? 0) { mejor = (i, l.valores, total) }
            }
        }
        guard let m = mejor, m.nota >= umbral else { return nil }
        return m
    }

    /// MEJORA: pedidos compuestos. Si no entiende la frase entera pero sí
    /// cada parte separada por "y" / "luego" / "después", encadena los planes.
    static func entiendeCompuesto(_ frase: String) -> [(indice: Int, valores: [Int: String], nota: Double)]? {
        var partes = [frase]
        for sep in [" y luego ", " y después ", " luego ", " después ", " y "] {
            partes = partes.flatMap { $0.components(separatedBy: sep) }
        }
        partes = partes.map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
        guard partes.count > 1 else { return nil }
        var r: [(indice: Int, valores: [Int: String], nota: Double)] = []
        for p in partes {
            guard let e = entiende(p) else { return nil }
            r.append(e)
        }
        return r
    }

    static func rellena(_ s: String, _ valores: [Int: String]) -> String {
        var r = s
        for (k, v) in valores { r = r.replacingOccurrences(of: "⟨\(k)⟩", with: v) }
        return r
    }

    static func dibuja(_ forma: [HuellaTrozo], _ valores: [String]) -> String {
        forma.map { t in
            switch t {
            case .ancla(let w): return w
            case .hueco(let k): return "[" + (k < valores.count ? valores[k] : "?") + "]"
            }
        }.joined(separator: " ")
    }

    static func porcentaje(_ x: Double) -> String { "\(Int((x * 100).rounded()))%" }

    // ---------- los ojos en la terminal ----------

    /// Toda orden que escribes pasa por aquí. Si Huella está aprendiendo,
    /// mira el paisaje antes y después y guarda lo que pasó.
    static func ejecuta(_ linea: String, _ sh: Shell) async -> String {
        let mente = HuellaMente.una
        let t = linea.trimmingCharacters(in: .whitespaces)
        let esSuya = t.lowercased().hasPrefix("huella")
        let libre = sh.mode == .shell && sh.blockBuffer.isEmpty && sh.heredocPend == nil
        guard mente.observando != nil, !esSuya, !t.isEmpty, libre else {
            return await sh.execute(linea)
        }
        let antes = mira(sh.env)
        let out = await sh.execute(linea)
        let despues = mira(sh.env)
        // solo guarda órdenes completas, no bloques a medio escribir
        if sh.mode == .shell && sh.blockBuffer.isEmpty && sh.heredocPend == nil {
            mente.observando?.ordenes.append(t)
            for a in cambios(antes, despues) where mente.observando?.vistos.contains(a) == false {
                mente.observando?.vistos.append(a)
            }
        }
        return out
    }

    static let ayuda = """
    Huella — la IA propia de Sfshell. Aprende viendo lo que haces.
      huella <frase>       pídele algo
      huella si | no       acepta o rechaza lo que propone
      huella listo         termina de enseñarle
      huella deja          cancela la enseñanza
      huella mira          lo que ve ahora
      huella sabe          lo que ha aprendido
      huella sueña         ordena su memoria
      huella olvida <n|todo>
    Si no sabe algo, te pedirá que se lo muestres: escribe las órdenes
    normales y termina con 'huella listo'.

    """
}

// ============================================================
// MARK: - Comando
// ============================================================

extension Shell {

    static func huella() -> [String: Spec] {
        var c: [String: Spec] = [:]

        c["huella"] = Spec(help: "huella <frase> — la IA propia: aprende viendo lo que haces · huella si|no|listo|mira|sabe|sueña") { ctx in
            let env = ctx.env
            let mente = HuellaMente.una
            HuellaIA.carga(env)
            let a = ctx.args
            let sola = a.count == 1 ? a[0].lowercased() : ""

            if a.isEmpty {
                var s = HuellaIA.ayuda
                if let o = mente.observando { s += "Ahora estoy aprendiendo «\(o.frase)» (\(o.ordenes.count) órdenes vistas).\n" }
                return s
            }

            switch sola {
            case "si", "sí":
                guard let pr = mente.propuesta,
                      pr.indices.allSatisfy({ $0 < mente.memoria.intenciones.count }) else {
                    return "huella: no tengo nada propuesto\n"
                }
                mente.propuesta = nil
                var out = ""
                let antes = HuellaIA.mira(env)
                for o in pr.ordenes {
                    out += "  $ \(o)\n"
                    out += await ctx.sh.execute(o)
                }
                let vistos = HuellaIA.cambios(antes, HuellaIA.mira(env))
                let falta = pr.espera.filter { !vistos.contains($0) }
                if falta.isEmpty {
                    for i in pr.indices { mente.memoria.intenciones[i].aciertos += 1 }
                    // MEJORA: Nyx también se entera de lo que pasó en el mundo
                    NyxNucleo.uno.percibir(vistos.map(\.texto))
                    out += pr.espera.isEmpty
                        ? "huella: hecho (esta intención no cambia nada que yo pueda ver)\n"
                        : "huella: ✓ vi lo que esperaba: " + pr.espera.map(\.texto).joined(separator: ", ") + "\n"
                } else {
                    // pierden confianza las partes que esperaban ver algo
                    for i in pr.indices where !mente.memoria.intenciones[i].espera.isEmpty {
                        mente.memoria.intenciones[i].fallos += 1
                    }
                    out += "huella: ✗ esperaba ver " + falta.map(\.texto).joined(separator: ", ") + "\n"
                    out += "        y vi " + (vistos.isEmpty ? "nada" : vistos.map(\.texto).joined(separator: ", ")) + "\n"
                    out += "        pierdo confianza en esto; si quieres, enséñamelo otra vez\n"
                }
                HuellaIA.guarda(env)
                return out

            case "no":
                guard let pr = mente.propuesta else { return "huella: no tengo nada propuesto\n" }
                mente.propuesta = nil
                for i in pr.indices where i < mente.memoria.intenciones.count {
                    mente.memoria.intenciones[i].fallos += 1
                }
                HuellaIA.guarda(env)
                mente.observando = (pr.frase, [], [])
                return "huella: vale. Muéstrame cómo se hace «\(pr.frase)»: escribe las órdenes y termina con 'huella listo'\n"

            case "listo":
                guard let o = mente.observando else { return "huella: no estaba aprendiendo nada\n" }
                mente.observando = nil
                guard !o.ordenes.isEmpty else { return "huella: no vi ninguna orden; no aprendí nada\n" }
                let (nueva, valores) = HuellaIA.aprende(o.frase, o.ordenes, o.vistos)
                var out = ""
                if let k = mente.memoria.intenciones.firstIndex(where: { $0.plan == nueva.plan && $0.espera == nueva.espera }) {
                    if !mente.memoria.intenciones[k].formas.contains(nueva.formas[0]) {
                        mente.memoria.intenciones[k].formas.append(nueva.formas[0])
                    }
                    out += "huella: eso ya lo sabía hacer; aprendí otra forma de pedirlo\n"
                } else {
                    mente.memoria.intenciones.append(nueva)
                    out += "huella: aprendí «\(o.frase)»\n"
                }
                out += "  la leo así: " + HuellaIA.dibuja(nueva.formas[0], valores) + "\n"
                out += "  plan: " + nueva.plan.joined(separator: " ; ") + "\n"
                out += nueva.espera.isEmpty
                    ? "  no vi cambios en el mundo: solo podré comprobar que se ejecuta\n"
                    : "  espero ver: " + nueva.espera.map(\.texto).joined(separator: ", ") + "\n"
                HuellaIA.guarda(env)
                NyxNucleo.uno.percibir(o.vistos.map(\.texto))
                return out

            case "deja":
                guard mente.observando != nil else { return "huella: no estaba aprendiendo nada\n" }
                mente.observando = nil
                return "huella: olvido esa enseñanza\n"

            case "mira":
                let p = HuellaIA.mira(env)
                var out = "huella: veo \(p.archivos.count) archivos y \(p.carpetas.count) carpetas; estoy en \(p.lugar); hay \(p.vars.count) variables\n"
                let recientes = p.archivos.sorted { $0.value.fecha > $1.value.fecha }.prefix(5)
                if !recientes.isEmpty {
                    out += "  lo último que cambió:\n"
                    for (k, _) in recientes { out += "    \(k)\n" }
                }
                if let o = mente.observando { out += "  estoy aprendiendo «\(o.frase)» (\(o.ordenes.count) órdenes vistas)\n" }
                return out

            case "sabe":
                let its = mente.memoria.intenciones
                guard !its.isEmpty else { return "huella: todavía no sé hacer nada. Pídeme algo y enséñamelo.\n" }
                var out = ""
                for (i, it) in its.enumerated() {
                    out += "\(i + 1). «\(it.ejemplo)» — confianza \(HuellaIA.porcentaje(it.confianza))"
                    out += it.formas.count > 1 ? " — \(it.formas.count) formas de pedirlo\n" : "\n"
                    out += "   plan: " + it.plan.joined(separator: " ; ") + "\n"
                }
                return out

            case "sueña", "suena":
                let antes = mente.memoria.intenciones.count
                // 1. olvida lo que casi siempre falla
                mente.memoria.intenciones.removeAll { $0.fallos >= 3 && $0.confianza < 0.35 }
                let olvidadas = antes - mente.memoria.intenciones.count
                // 2. junta lo que hace lo mismo
                var juntas: [HuellaIntencion] = []
                var unidas = 0
                for it in mente.memoria.intenciones {
                    if let k = juntas.firstIndex(where: { $0.plan == it.plan && $0.espera == it.espera }) {
                        for f in it.formas where !juntas[k].formas.contains(f) { juntas[k].formas.append(f) }
                        juntas[k].aciertos += it.aciertos
                        juntas[k].fallos += it.fallos
                        unidas += 1
                    } else {
                        juntas.append(it)
                    }
                }
                mente.memoria.intenciones = juntas
                mente.propuesta = nil
                HuellaIA.guarda(env)
                return "huella: soñé. Olvidé \(olvidadas), junté \(unidas); ahora sé \(juntas.count) cosas\n"

            default:
                break
            }

            if a.first?.lowercased() == "olvida" {
                guard a.count > 1 else { return "uso: huella olvida <número|todo>\n" }
                if a[1] == "todo" {
                    mente.memoria.intenciones.removeAll()
                } else if let n = Int(a[1]), n >= 1, n <= mente.memoria.intenciones.count {
                    mente.memoria.intenciones.remove(at: n - 1)
                } else {
                    throw ShErr("huella olvida: no existe '\(a[1])' (mira 'huella sabe')")
                }
                mente.propuesta = nil
                HuellaIA.guarda(env)
                return "huella: olvidado\n"
            }

            // ---------- una frase: entenderla ----------
            let frase = a.joined(separator: " ")
            if let o = mente.observando {
                return "huella: estoy aprendiendo «\(o.frase)». Termina con 'huella listo' o cancela con 'huella deja'\n"
            }
            var partes: [(indice: Int, valores: [Int: String], nota: Double)] = []
            if let e = HuellaIA.entiende(frase) {
                partes = [e]
            } else if let compuesto = HuellaIA.entiendeCompuesto(frase) {
                partes = compuesto
            } else {
                mente.observando = (frase, [], [])
                return """
                huella: todavía no sé hacer «\(frase)».
                  muéstramelo: escribe las órdenes normales y termina con 'huella listo'

                """
            }

            var ordenes: [String] = []
            var espera: [HuellaHecho] = []
            var out = partes.count > 1
                ? "huella: lo parto en \(partes.count) cosas que sé hacer\n"
                : ""
            for e in partes {
                let it = mente.memoria.intenciones[e.indice]
                ordenes += it.plan.map { HuellaIA.rellena($0, e.valores) }
                espera += it.espera.map { HuellaHecho(tipo: $0.tipo, que: HuellaIA.rellena($0.que, e.valores)) }
                out += "huella: lo entiendo como «\(it.ejemplo)» (seguridad \(HuellaIA.porcentaje(e.nota)))\n"
                let huecos = e.valores.sorted { $0.key < $1.key }.map(\.value)
                if !huecos.isEmpty { out += "  huecos: " + huecos.joined(separator: " · ") + "\n" }
            }
            mente.propuesta = HuellaPropuesta(indices: partes.map(\.indice), frase: frase, ordenes: ordenes, espera: espera)

            out += "  haría:\n"
            for (i, o) in ordenes.enumerated() { out += "    \(i + 1)) \(o)\n" }
            if !espera.isEmpty { out += "  y espero ver: " + espera.map(\.texto).joined(separator: ", ") + "\n" }
            out += "  → 'huella si' para hacerlo · 'huella no' para enseñarme otra cosa\n"
            return out
        }

        return c
    }
}
