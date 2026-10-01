import Foundation

// ============================================================
//  CONSEJO RESONANTE — 18 mentes, un veredicto
//  Cada mente es un ResonantMind especializado en un rol.
//  Ante un input, las 18 deliberan en paralelo (TaskGroup) y el
//  consejo emite UN veredicto por consenso ponderado por coherencia.
//  El consenso se reinyecta a las 18 como calibración suave.
// ============================================================

struct VotoMental {
    let rol: RolMental
    let atractor: String
    let coherencia: Double
    let incrustacion: Double  // E: fracción dialógica (anti delirio coherente)
    let precision: Double     // ×20: confianza calibrada de la mente (historial)
}

struct ConsejoVeredicto {
    let veredicto: String
    let votos: [VotoMental]
}

// ---------- Mundo: la puerta del grounding ----------
// Un suceso con consecuencias. El Consejo lo reparte a las 18 mentes:
// cada una refuerza o debilita lo que el hecho nombra, y el hecho entra
// a su huella compartida. Es la primera forma en que la realidad toca
// al sistema (antes: solo símbolos apuntando a símbolos).
struct SucesoMundo {
    let hecho: String   // "el usuario dijo que sí", "el sensor marcó 34°", ...
    let valor: Double    // -1 (malo/falso) .. 1 (bueno/verdadero)
    let fecha: Date
}

// ---------- Ojo: a quién le llega lo que se ve ----------
enum ObjetivoOjo: Hashable {
    case todas            // las 18 mentes ven el live
    case una(RolMental)   // solo una mente ve
}


actor ConsejoResonante {
    private var mentes: [(rol: RolMental, mente: ResonantMind)] = []
    private var logConsejo: [String] = []
    private var mundo: [SucesoMundo] = []

    init() {
        for rol in RolMental.allCases {
            mentes.append((rol, ResonantMind()))
        }
    }

    // Especializa cada mente en su rol y le da semilla de dominio
    func especializar() async {
        for (rol, mente) in mentes {
            await mente.adoptRole(rol)
            // Sin etiqueta forzada: cada palabra conserva su propio nombre
            // como etiqueta, así los veredictos muestran palabras reales.
            await mente.ingestText(semilla(para: rol))
        }
        appendLog("consejo de \(mentes.count) mentes especializado")
    }

    private func semilla(para rol: RolMental) -> String {
        switch rol {
        case .sintaxis:   return "sintaxis gramatica estructura frase orden sujeto predicado"
        case .semantica:  return "significado concepto idea sentido contexto interpretacion"
        case .logica:     return "si entonces por lo tanto premisa conclusion valido invalido"
        case .codigo:     return "func var let return if else for while struct class actor"
        case .creativo:   return "imagina inventa combina metafora analogia posibilidad"
        case .critico:    return "pero sin embargo objecion falla contraejemplo verifica"
        case .memoria:    return "recuerda antes previamente historia patron conocido"
        case .percepcion: return "veo escucho forma color sonido ritmo patron visual"
        case .sintesis:   return "en resumen por tanto integra une fusiona concluye"
        case .intuicion:   return "corazonada instante fulgor destello presiento arrebato"
        case .analogia:    return "como igual que metafora puente entre dominios mapea"
        case .contexto:    return "situacion aqui ahora entorno circunstancia marco"
        case .esceptico:   return "duda pregunta cuestiona y si acaso tal vez"
        case .narrativa:   return "entonces despues antes historia secuencia relata"
        case .etica:       return "deber bien mal justo correcto valor principio"
        case .curiosidad:  return "que por que como novedad explora descubre"
        case .abstraccion: return "esencia patron general comprime eleva resume"
        case .empatia:     return "siente con otro comprende acompana sintoniza"
        }
    }

    // Las 18 mentes piensan sin parar, cada una en su propio Task
    func arrancarCognicionEterna() {
        for (_, mente) in mentes {
            Task.detached(priority: .background) { [mente] in
                await mente.runForever()
            }
        }
        appendLog("cognición eterna × 18 iniciada")
    }

    // Bucle eterno del consejo: cada 0.4 s las mentes hablan entre sí
    // aunque no haya input del usuario. El diálogo nunca se detiene.
    // El chat visible se publica moderado (legible); el éter corre libre.
    func runForever() async {
        while !Task.isCancelled {
            try? await Task.sleep(nanoseconds: 400_000_000) // ×10 diálogo
            let emitidos = await rondaDeDialogo()
            if let mejor = mejorEmision(de: emitidos) {
                let haceSuficiente = Date().timeIntervalSince(ultimoChat) >= 0.8
                let yaPublicado = publicadosRecientes.contains(mejor.texto)
                if haceSuficiente && !yaPublicado {
                    let g = await glosa(de: mejor.rol, texto: mejor.texto)
                    publicar(autor: mejor.rol.rawValue, texto: mejor.texto, glosa: g.texto, comprension: g.comprension)
                    ultimoChat = Date()
                    publicadosRecientes.append(mejor.texto)
                    publicadosRecientes = Array(publicadosRecientes.suffix(10))
                }
            }
            await Task.yield()
        }
    }

    /// Traducción al español de lo que dijo una mente, con SU léxico (lo
    /// que le enseñaron incluido), para que el humano entienda el Resh.
    private func glosa(de rol: RolMental, texto: String) async -> (texto: String?, comprension: Double?) {
        guard let mente = mentes.first(where: { $0.rol == rol })?.mente else { return (nil, nil) }
        let g = await mente.glosar(texto)
        let igual = g.texto.trimmingCharacters(in: .punctuationCharacters) == texto.lowercased().trimmingCharacters(in: .punctuationCharacters)
        return (igual ? nil : g.texto, g.comprension)
    }

    private func mejorEmision(de emitidos: [(rol: RolMental, texto: String, peso: Double)]) -> (rol: RolMental, texto: String, peso: Double)? {
        var mejor: (rol: RolMental, texto: String, peso: Double)? = nil
        for e in emitidos {
            if mejor == nil || e.peso > mejor!.peso { mejor = e }
        }
        return mejor
    }

    // Una ronda de diálogo: cada mente decide libremente si emite algo;
    // lo emitido se reparte a las otras 17, que lo transducen.
    // Devuelve las emisiones con su peso (el consejo destaca el mayor peso).
    func rondaDeDialogo() async -> [(rol: RolMental, texto: String, peso: Double)] {
        var emitidos: [(rol: RolMental, contenido: String, tipo: TipoMensaje, peso: Double)] = []
        for (rol, mente) in mentes {
            if let (contenido, tipo, peso) = await mente.formularMensaje() {
                emitidos.append((rol, contenido, tipo, peso))
            }
        }
        guard !emitidos.isEmpty else { return [] }
        for (emisor, contenido, tipo, _) in emitidos {
            let vista = String(contenido.prefix(44))
            appendLog("\(emisor.rawValue) [\(tipo.rawValue)]: \(vista)")
            for (receptor, mente) in mentes where receptor != emisor {
                await mente.recibir(mensaje: contenido, de: emisor.rawValue, tipo: tipo)
            }
        }
        var salida: [(rol: RolMental, texto: String, peso: Double)] = []
        for e in emitidos {
            salida.append((rol: e.rol, texto: e.contenido, peso: e.peso))
        }
        return salida
    }

    // Votación paralela de las 18 mentes (helper)
    private func votar(sobre input: String) async -> [VotoMental] {
        await withTaskGroup(of: VotoMental.self,
                            returning: [VotoMental].self) { grupo in
            for (rol, mente) in mentes {
                grupo.addTask {
                    let (atractor, coherencia, incrustacion) = await mente.veredicto(sobre: input)
                    let p = await mente.precision()
                    return VotoMental(rol: rol, atractor: atractor, coherencia: coherencia, incrustacion: incrustacion, precision: p)
                }
            }
            var out: [VotoMental] = []
            for await v in grupo { out.append(v) }
            return out
        }
    }

    // ×150 — descomposición: parte el input en pasos ("y", ",").
    private func pasosDe(_ input: String) -> [String] {
        var pasos: [String] = []
        for chunk in input.components(separatedBy: " y ") {
            for c2 in chunk.components(separatedBy: ",") {
                let t = c2.trimmingCharacters(in: .whitespacesAndNewlines)
                if !t.isEmpty { pasos.append(t) }
            }
        }
        if pasos.count > 3 { pasos = Array(pasos.prefix(3)) }
        return pasos.isEmpty ? [input] : pasos
    }

    // Urna ponderada coherencia × incrustación × precisión (una sola
    // implementación para votar y para medir el margen del consenso).
    private func urnaDe(_ votos: [VotoMental]) -> [String: Double] {
        var urna: [String: Double] = [:]
        for v in votos {
            let peso = max(0, v.coherencia) * (0.2 + 0.8 * v.incrustacion) * (0.5 + 0.5 * v.precision)
            urna[v.atractor, default: 0] += peso + 0.01
        }
        return urna
    }

    // Margen del consenso: diferencia entre los 2 primeros. Si está reñido,
    // el consejo debate una ronda extra (cómputo adaptativo, no fijo).
    private func margenDe(_ votos: [VotoMental]) -> Double {
        let top = urnaDe(votos).values.sorted(by: >)
        guard top.count >= 2 else { return 1.0 }
        return top[0] - top[1]
    }

    // Deliberación colectiva: voto → debate (2 rondas) → re-voto → consenso
    func deliberar(sobre input: String) async -> ConsejoVeredicto {
        appendLog("deliberación: «\(input.prefix(50))»")

        // Fase 0 ×150: descomposición en pasos. Cada paso vota rápido y su
        // veredicto parcial alimenta al siguiente: cadena de pensamiento.
        let pasos = pasosDe(input)
        var contextoPasos = ""
        if pasos.count > 1 {
            for (i, p) in pasos.enumerated() {
                let pregunta = (i == 0 || contextoPasos.isEmpty) ? p : "\(contextoPasos) \(p)"
                let vs = await votar(sobre: pregunta)
                if let top = vs.max(by: { $0.coherencia < $1.coherencia }) {
                    contextoPasos = top.atractor
                    appendLog("paso \(i + 1)/\(pasos.count): \(top.atractor.prefix(30))")
                }
            }
        }
        let inputEnriquecido = pasos.count > 1 ? "\(input) \(contextoPasos)" : input

        // Fase 1: veredictos individuales en paralelo
        let votosIni = await votar(sobre: inputEnriquecido)

        // ×150 — cómputo adaptativo: si el consenso inicial está reñido, una
        // ronda extra de debate en vez de decidir a prisa.
        let rondasDebate = margenDe(votosIni) < 0.08 ? 3 : 2

        // Fase 2: debate — las mentes se hablan entre sí antes de decidir.
        // El rol crítico inhibe funcionalmente al atractor líder (θ=π) y
        // difunde el desacuerdo: las demás reciben "ne <etiqueta>" y lo
        // integran como operador inhibitorio. Disenso real, no ruido.
        // Lo robusto sobrevive; el artefacto de frecuencia colapsa.
        for ronda in 1 ... rondasDebate {
            _ = await rondaDeDialogo()
            let difusion = await difundirInhibicionCritica()
            if difusion.isEmpty {
                appendLog("debate ronda \(ronda)/\(rondasDebate)")
            } else {
                appendLog("debate ronda \(ronda)/\(rondasDebate): crítico difunde «ne \(difusion)»")
            }
        }

        // Fase 3: re-votación tras escucharse unas a otras
        let votos = await votar(sobre: inputEnriquecido)

        // Fase 4: consenso ponderado por coherencia × incrustación dialógica
        // × precisión calibrada. Un atractor muy coherente pero sin
        // incrustación (delirio coherente) pesa poco; y una mente con
        // historial de errores pesa menos aunque grite fuerte.
        let urna = urnaDe(votos)
        let veredicto = urna.max { $0.value < $1.value }?.key ?? "sin consenso"
        appendLog("veredicto del consejo: \(veredicto)")

        // Fase 4.5 ×1500: contrafáctico — ¿el consenso sobrevive sin su atractor?
        // La mente ganadora re-delibera con su atractor suprimido: si otro toma
        // el mando con fuerza, el veredicto es frágil; si se mantiene, robusto.
        if let rolGanador = votos.first(where: { $0.atractor == veredicto })?.rol,
           let menteG = mentes.first(where: { $0.rol == rolGanador })?.mente {
            let cf = await menteG.veredictoContrafactual(sobre: inputEnriquecido, sin: veredicto)
            if cf.atractor != veredicto, cf.coherencia > 0.5 {
                appendLog("contrafáctico: sin «\(veredicto)» caería en «\(cf.atractor)» — veredicto frágil")
            } else {
                appendLog("contrafáctico: «\(veredicto)» sobrevive — veredicto robusto")
            }
        }

        // Fase 5: el consenso se reinyecta como calibración suave en las 18,
        // y cada mente calibra su confianza según si su voto coincidió (×20).
        for (rol, mente) in mentes {
            await mente.ingestText(veredicto)
            let miVoto = votos.first(where: { $0.rol == rol })?.atractor
            await mente.confirmar(acierto: miVoto == veredicto)
        }

        return ConsejoVeredicto(veredicto: veredicto,
                                votos: votos.sorted { $0.coherencia > $1.coherencia })
    }

    // El crítico inhibe su atractor líder y difunde "ne <etiqueta>".
    // Devuelve la etiqueta difundida (vacía si no hubo inhibición).
    private func difundirInhibicionCritica() async -> String {
        var etiqueta = ""
        for (rol, mente) in mentes where rol == .critico {
            if let blanco = await mente.inhibirAtractorLider() {
                etiqueta = String(blanco.prefix(24))
            }
        }
        guard !etiqueta.isEmpty else { return "" }
        for (rol, mente) in mentes where rol != .critico {
            await mente.recibir(mensaje: "ne \(etiqueta)", de: RolMental.critico.rawValue, tipo: .alerta)
        }
        return etiqueta
    }

    func recentLog(_ n: Int = 20) -> [String] { Array(logConsejo.suffix(n)) }

    // ---------- Chat unificado del consejo ----------
    // Las 18 mentes + el humano en un solo chat. El mensaje humano es uno
    // más del éter: las mentes responden libremente (no forzadas) según su
    // objetivo y su dinámica. La respuesta destacada es la emisión de mayor
    // peso. Las pruebas (deliberar) siguen siendo el camino forzado.
    private var chat: [MensajeChat] = []
    private var ultimoChat = Date.distantPast
    private var publicadosRecientes: [String] = []

    func chatReciente(_ n: Int = 120) -> [MensajeChat] { Array(chat.suffix(n)) }

    func limpiarChat() { chat.removeAll() }

    private func publicar(autor: String, texto: String, esHumano: Bool = false, destacada: Bool = false,
                          glosa: String? = nil, comprension: Double? = nil) {
        chat.append(MensajeChat(texto: texto, esHumano: esHumano, glosa: glosa, comprension: comprension,
                                autor: autor, destacada: destacada))
        if chat.count > 150 { chat.removeFirst(chat.count - 150) }
    }

    /// Chat libre con el consejo: tu mensaje entra al éter como uno más.
    func conversar(_ mensaje: String) async {
        publicar(autor: "tú", texto: mensaje, esHumano: true)
        appendLog("humano: \(mensaje.prefix(50))")
        for (_, mente) in mentes {
            await mente.recibir(mensaje: mensaje, de: "humano", tipo: .dato)
        }
        // Ráfaga de diálogo: las mentes hablan entre sí y con el humano.
        var rafaga: [(rol: RolMental, texto: String, peso: Double)] = []
        for _ in 1 ... 3 {
            let ronda = await rondaDeDialogo()
            var ordenada = ronda
            ordenada.sort { $0.peso > $1.peso }
            for e in ordenada.prefix(2) { rafaga.append(e) }
        }
        // La respuesta es la emisión de mayor peso (por índice, sin closures).
        var mejorIndice = -1
        var mejorPeso = -Double.infinity
        for (i, e) in rafaga.enumerated() {
            if e.peso > mejorPeso {
                mejorPeso = e.peso
                mejorIndice = i
            }
        }
        for (i, e) in rafaga.enumerated() {
            let g = await glosa(de: e.rol, texto: e.texto)
            publicar(autor: e.rol.rawValue, texto: e.texto, destacada: i == mejorIndice,
                     glosa: g.texto, comprension: g.comprension)
        }
        if rafaga.isEmpty {
            publicar(autor: "consejo", texto: "…")
        }
    }

    // ---------- Enseñanza del idioma ----------
    // "Esto significa esto, esto, esto en tu idioma": el humano enseña
    // palabra por palabra; cada mente la guarda en SU memoria privada.
    func enseñarATodos(español: String, significa: [String]) async {
        for (_, mente) in mentes {
            await mente.enseñar(español: español, significa: significa)
        }
        appendLog("enseñado a las 18: \(español) = \(significa.joined(separator: ", "))")
    }

    func enseñarA(rol: RolMental, español: String, significa: [String]) async {
        if let mente = mentes.first(where: { $0.rol == rol })?.mente {
            await mente.enseñar(español: español, significa: significa)
            appendLog("enseñado a \(rol.rawValue): \(español)")
        }
    }

    /// Puerta de grounding: registra una consecuencia del mundo real y la
    /// reparte a las 18 mentes. El valor refuerza (+) o debilita (−) los
    /// acoplamientos que nombran el hecho, y el hecho entra a la huella
    /// compartida: lo que tuvo consecuencias pesa más al votar.
    /// Úsalo desde sensores, atajos, o cuando el humano confirme/corrija
    /// ("sí, eso era" → +1 / "no, estaba mal" → −1).
    func registrarSuceso(hecho: String, valor: Double) async {
        let s = SucesoMundo(hecho: hecho, valor: max(-1, min(1, valor)), fecha: Date())
        mundo.append(s)
        if mundo.count > 500 { mundo.removeFirst(mundo.count - 500) }
        for (_, mente) in mentes { await mente.absorberSuceso(s) }
        appendLog("mundo: \(hecho.prefix(40)) → \(String(format: "%+.1f", s.valor))")
    }

    /// Historial de consecuencias (para depurar el grounding).
    func sucesosMundo() -> [SucesoMundo] { mundo }

    // ---------- Ojo: visión en vivo ----------
    /// Muestra un cuadro a las mentes objetivo y devuelve lo que cada una
    /// "vio" (la etiqueta con la que resonó). Como un humano viendo un live:
    /// el cuadro entra en tiempo real y cada mente lo resuena a su manera.
    func mostrar(cuadro: CuadroVisual, a objetivo: ObjetivoOjo) async -> [(rol: RolMental, vio: String)] {
        var r: [(rol: RolMental, vio: String)] = []
        for (rol, mente) in mentes {
            switch objetivo {
            case .todas: break
            case .una(let target): guard rol == target else { continue }
            }
            let vio = await mente.ver(cuadro)
            r.append((rol, vio))
        }
        if r.count > 1 {
            appendLog("ojo [\(cuadro.fuente)] → \(r.count) mentes")
        }
        return r
    }

    /// Suma de palabras aprendidas por las 18 memorias privadas.
    func memoriaTotal() async -> Int {
        var t = 0
        for (_, mente) in mentes { t += await mente.memoriaCount() }
        return t
    }

    /// Ventana de contexto privada de una mente (para inspeccionar qué vive).
    func contextoDe(rol: RolMental, n: Int = 8) async -> [String] {
        guard let mente = mentes.first(where: { $0.rol == rol })?.mente else { return [] }
        return await mente.contextoReciente(n)
    }
    func contarMentes() -> Int { mentes.count }

    private func appendLog(_ e: String) {
        logConsejo.append(e)
        if logConsejo.count > 100 { logConsejo.removeFirst(logConsejo.count - 100) }
    }
}
