import Foundation

// NOTA: este archivo está pensado para un proyecto tipo App de
// Swift Playgrounds (con @main). Por eso NO trae código ejecutable
// suelto ni importa PlaygroundSupport: solo los tipos. El arranque
// se hace desde la vista (ver ContentView de ejemplo).

// ============================================================
//  CEREBRO RESONANTE — Paradigma FCR
//  Campos de Coherencia Resonante / Geometría de Estado Fluido
//  Prototipo para Swift Playgrounds (Swift puro + concurrencia)
// ============================================================
//  IDEA CENTRAL (no es transformer, ni red neuronal, ni RL):
//  La mente es un fluido semántico compresible habitado por
//  "semiones": vórtices con amplitud A, fase φ y carga topológica q.
//  Pensar = dejar que las ondas de fase interfieran hasta que el
//  sistema relaja su DISONANCIA hacia un atractor de coherencia.
//  No hay capas, no hay backprop, no hay atención. Hay resonancia.
//
//  Estado: S = { s_i }, s_i = A_i * e^{i φ_i}, con carga q_i ∈ Z
//  Funcional de coherencia:
//    C = Σ_{i,j} w_{ij} A_i A_j cos(φ_i − φ_j − θ_{ij}) · δ(q_i,q_j)
//        − λ Σ_i (A_i² − A₀²)²  −  μ Σ_i |∇q_i|²
//  Disonancia D = −C. La dinámica es:
//    dφ_i/dt = ω_i + Σ_j w_{ij} A_j sin(φ_j − φ_i − θ_{ij}) + η(t)
//    dA_i/dt  = −∂D/∂A_i  +  acoplamiento topológico
//  La inferencia es la relajación; la creatividad es la inyección
//  controlada de ruido topológico cuando D se estanca.
// ============================================================

// MARK: - Semión: cuanto simbólico

struct SemionR: Identifiable, Hashable, Codable {
    let id: UUID
    var label: String
    var amplitude: Double      // A_i ∈ [0, 1]
    var phase: Double           // φ_i ∈ [0, 2π)
    var charge: Int             // q_i: carga topológica (conservada salvo fusión)
    var signature: [Double]     // firma morfodinámica (no es embedding neuronal)
    var frequency: Double       // ω_i: frecuencia natural
    var lastActive: Date
    var usosDialogicos: Int   // selección dialógica: veces que el éter retomó este semión

    init(label: String, signature: [Double], charge: Int = 1) {
        self.id = UUID()
        self.label = label
        self.signature = signature
        self.amplitude = 0.5
        self.phase = Double.random(in: 0 ..< (2 * Double.pi))
        self.charge = charge
        self.frequency = Double.random(in: 0.5 ... 2.0)
        self.lastActive = Date()
        self.usosDialogicos = 0
    }

    // Distancia morfodinámica (no euclidiana clásica: incluye fase y carga)
    func morphDistance(to other: SemionR) -> Double {
        let n = min(signature.count, other.signature.count)
        var acc = 0.0
        for k in 0 ..< n {
            let d = signature[k] - other.signature[k]
            acc += d * d
        }
        let phaseTerm = 1 - cos(phase - other.phase)
        let chargePenalty: Double = (charge == other.charge) ? 0 : 1.5
        return sqrt(acc) + 0.5 * phaseTerm + chargePenalty
    }
}

struct Coupling: Codable {
    var weight: Double   // w_{ij} ≥ 0
    var theta: Double    // θ_{ij}: desfase preferido
}

// MARK: - Roles del consejo (18 especializaciones)

enum RolMental: String, CaseIterable {
    case sintaxis, semantica, logica, codigo, creativo, critico, memoria, percepcion, sintesis,
    intuicion, analogia, contexto, esceptico, narrativa, etica, curiosidad, abstraccion, empatia
}

// Tipos de mensaje que las mentes se cruzan por el éter
enum TipoMensaje: String {
    case dato, pregunta, propuesta, alerta
}

// Modos de la frase compuesta (×20): la gramática mínima con la que las
// mentes encadenan 2-3 raíces en vez de soltar palabras sueltas.
private enum ModoFrase {
    case afirma, pregunta, niega, narra
}

// Una entrada del léxico privado de cada mente
struct EntradaMemoria: Codable {
    var variantes: [String]  // formas Resh que significan lo mismo
    var fuerza: Double       // 0..1: decae con el tiempo, se refuerza con uso
    var usos: Int
    var ultimaVez: Date
}

/// Foto de una mente para guardarla en disco (MEJORA: persistencia).
struct FotoMente: Codable {
    var semions: [SemionR]
    var acoples: [String: [String: Coupling]]
    var memoria: [String: EntradaMemoria]
    var resh: [String]
    var aciertos: Int
    var intentos: Int
    var compartida: [String]
}

// MARK: - Mente resonante (actor: estado aislado, seguro en concurrencia)

actor ResonantMind {
    private var semions: [UUID: SemionR] = [:]
    private var couplings: [UUID: [UUID: Coupling]] = [:]
    private var pendingTasks: [String] = []
    private var log: [String] = []

    private var rol: RolMental = .semantica
    private var objetivo: String = ""          // objetivo propio del rol: lo que "quiere"
    private var objetivoIds: [UUID] = []       // semiones-objetivo: no se dejan desvanecer
    // ---- Inteligencia ×20: calibración, episodios e inferencia ----
    private var aciertos = 0                  // veredictos que coincidieron con el consenso
    private var intentos = 0                  // veredictos evaluados + correcciones recibidas
    private var episodios: [(seq: Int, etiqueta: String)] = []  // memoria episódica
    private var seqEpisodio = 0
    // ---- Inteligencia ×150: foco, predicción y teoría de la mente ----
    private var foco: [UUID] = []                  // memoria de trabajo: lo atendido
    private let capacidadFoco = 7
    private var ultimoEmisor: [String: String] = [:]  // token → rol que lo dijo
    private var contadorCaminos: [String: Int] = [:]  // camino A→B→C → veces recorrido
    private var preguntaDirigida = false
    private var ciclosPensados: Int = 0
    private var rigidez = 0.8             // λ: rigidez homeostática de amplitud
    private var objetivoAmplitud = 0.6    // A₀: amplitud objetivo
    private var nivelRuido = 0.05         // η: ruido topológico
    private let maxSemions = 8000       // ×10 sustrato: más semiones = más capacidad

    // ---- Memoria privada y ventana de contexto (nada se comparte
    //      salvo lo que cada mente decide decir por el éter) ----
    private var memoriaLexica: [String: EntradaMemoria] = [:]
    private var ventanaDeContexto: [String] = []
    private let capacidadVentana = 4000  // ventana de contexto privada
    private var preguntasPendientes: [String] = []
    private var ultimaPregunta: (palabra: String, de: String)?
    // Huella del contexto compartido: palabras oídas en el éter de otras
    // mentes o deliberadas en consejo. Mide la incrustación dialógica de un
    // atractor (anti "delirio coherente"): lo que nunca tocó el contexto
    // compartido pesa menos al votar. Acotada para no crecer sin límite.
    private var huellaCompartida: Set<String> = []
    // Índice rápido de formas Resh ya aprendidas (no es la memoria: la
    // traducción sigue siendo por reconstrucción resonante).
    private var reshConocido: Set<String> = []
    // MEJORA: índice palabra → semión. Una palabra conocida se reactiva en
    // vez de nacer otra vez, así lo aprendido se acumula en un solo lugar.
    private var indiceEtiqueta: [String: UUID] = [:]

    private static let clock: DateFormatter = {
        let df = DateFormatter()
        df.dateFormat = "HH:mm:ss"
        return df
    }()

    // ---------- Ingesta: texto → semiones ----------
    // Sin tokenizador BPE ni embeddings preentrenados.
    // La "firma" se deriva de rasgos formales: longitud, simetría,
    // densidad simbólica, ritmo de puntuación. Es transducción, no lookup.
    @discardableResult
    func ingestText(_ text: String, label: String? = nil) -> [UUID] {
        let words = text.split(separator: " ").map(String.init)
        var ids: [UUID] = []
        for w in words.prefix(24) {
            let etiqueta = label ?? w
            let clave = label == nil ? w : "\(etiqueta)|\(w)"
            // MEJORA: si la palabra ya vive en esta mente, se REACTIVA el mismo
            // semión. Antes cada mención creaba una copia nueva y el
            // significado se repartía entre miles de duplicados sin relación.
            if let id = indiceEtiqueta[clave], var s = semions[id] {
                s.amplitude = min(1, s.amplitude + 0.15)
                s.lastActive = Date()
                semions[id] = s
                ids.append(id)
            } else {
                let s = SemionR(label: etiqueta, signature: signatureForText(w))
                insert(s)
                indiceEtiqueta[clave] = s.id
                ids.append(s.id)
            }
        }
        // MEJORA: antes se acoplaban semiones AL AZAR (un diccionario no tiene
        // orden, así que 'values.suffix' devolvía cualquiera). Ahora se acoplan
        // las palabras que de verdad iban seguidas, y repetir una secuencia
        // la refuerza en vez de pisarla.
        for (a, b) in zip(ids, ids.dropFirst()) where a != b {
            reforzarAcople(a, b, theta: 0.3)
        }
        appendLog("ingesta texto: \(words.count) semiones")
        pushContexto("ingesta: \(text.prefix(40))")
        return ids
    }

    /// Refuerzo con saturación: cada repetición acerca el peso a 1 sin pasarse.
    private func reforzarAcople(_ a: UUID, _ b: UUID, theta: Double) {
        if let c = couplings[a]?[b] {
            couplings[a]?[b] = Coupling(weight: min(1, c.weight + 0.1 * (1 - c.weight)), theta: c.theta)
        } else {
            couple(a, b, weight: 0.7, theta: theta)
        }
    }

    func signatureForText(_ word: String) -> [Double] {
        let scalars = word.unicodeScalars.map { Double($0.value) }
        let len = Double(word.count)
        let mean = scalars.reduce(0, +) / max(1, Double(scalars.count))
        let variance = scalars.map { ($0 - mean) * ($0 - mean) }.reduce(0, +) / max(1, Double(scalars.count))
        let half = word.count / 2
        let pairs = zip(word.prefix(half).reversed(), word.suffix(half))
        let symmetry = Double(pairs.filter { $0.0 == $0.1 }.count)
        let digitDensity = Double(word.filter(\.isNumber).count) / max(1, len)
        let upperDensity = Double(word.filter(\.isUppercase).count) / max(1, len)
        let vowelDensity = Double(word.filter { "aeiouAEIOU".contains($0) }.count) / max(1, len)
        let symbolDensity = Double(word.filter { "+-*/=<>^".contains($0) }.count) / max(1, len)
        return [len / 20.0,
                mean.truncatingRemainder(dividingBy: 128) / 128.0,
                min(1, variance / 4000.0),
                symmetry / max(1, len),
                digitDensity,
                upperDensity,
                vowelDensity,
                symbolDensity]
    }

    // ---------- Ingesta: visión y audición (transducción directa) ----------
    // En Playgrounds esto recibe descriptores ya extraídos (p. ej. de
    // Vision/AVFoundation) y los convierte a semiones sin CNN.
    func ingestImage(signature: [Double], label: String) {
        var s = SemionR(label: "img:\(label)", signature: signature, charge: 2)
        s.amplitude = 0.8
        insert(s)
        appendLog("ingesta imagen: \(label)")
    }

    func ingestAudio(samples: [Double], label: String) {
        // Firma rítmica: energía, cruces por cero, hash de etiqueta
        let energy = samples.map { $0 * $0 }.reduce(0, +) / max(1, Double(samples.count))
        var zeroCross = 0
        for (a, b) in zip(samples, samples.dropFirst()) where (a < 0) != (b < 0) {
            zeroCross += 1
        }
        let sig = [min(1, energy * 10),
                   Double(zeroCross) / max(1, Double(samples.count)),
                   label.hashValue.doubleTruncated]
        var s = SemionR(label: "aud:\(label)", signature: sig, charge: -1)
        s.frequency = 1.0 + min(3, energy * 20)
        insert(s)
        appendLog("ingesta audio: \(label)")
    }

    // ---------- Núcleo: ciclo de pensamiento ----------
    func thinkCycle() {
        decay()
        propagateResonance()
        decaerMemoria()
        ciclosPensados += 1
        if ciclosPensados % 50 == 0 {
            refrescarObjetivo()
            replayConsolidacion()
        }
        if pendingTasks.isEmpty {
            spontaneousSynthesis()   // ← cognición continua: jamás 0% cómputo
        } else {
            solveTask(pendingTasks.removeFirst())
        }
        consolidate()
    }

    // Bucle eterno: cede el hilo para no congelar el Playground
    func runForever() async {
        appendLog("cognición eterna iniciada")
        while !Task.isCancelled {
            thinkCycle()
            // Pausa breve + yield: mantiene UI viva, cómputo > 0 siempre
            try? await Task.sleep(nanoseconds: 60_000_000) // 60 ms
            await Task.yield()
        }
    }

    func submitTask(_ task: String) { pendingTasks.append(task) }

    // ---------- Especialización por rol (consejo de 18) ----------
    // Cada rol ajusta su dinámica: el lógico es rígido y silencioso,
    // el creativo es ruidoso y propenso a fusiones, etc.
    func adoptRole(_ r: RolMental) {
        rol = r
        switch r {
        case .sintaxis:   nivelRuido = 0.02; rigidez = 1.2; objetivoAmplitud = 0.65
        case .semantica:  nivelRuido = 0.05; rigidez = 0.8; objetivoAmplitud = 0.60
        case .logica:     nivelRuido = 0.01; rigidez = 1.4; objetivoAmplitud = 0.70
        case .codigo:     nivelRuido = 0.03; rigidez = 1.0; objetivoAmplitud = 0.65
        case .creativo:   nivelRuido = 0.12; rigidez = 0.4; objetivoAmplitud = 0.50
        case .critico:    nivelRuido = 0.02; rigidez = 1.5; objetivoAmplitud = 0.55
        case .memoria:    nivelRuido = 0.03; rigidez = 0.5; objetivoAmplitud = 0.80
        case .percepcion: nivelRuido = 0.06; rigidez = 0.7; objetivoAmplitud = 0.60
        case .sintesis:   nivelRuido = 0.09; rigidez = 0.5; objetivoAmplitud = 0.55
        case .intuicion:   nivelRuido = 0.14; rigidez = 0.3; objetivoAmplitud = 0.55
        case .analogia:    nivelRuido = 0.11; rigidez = 0.45; objetivoAmplitud = 0.55
        case .contexto:    nivelRuido = 0.04; rigidez = 0.9; objetivoAmplitud = 0.70
        case .esceptico:   nivelRuido = 0.03; rigidez = 1.3; objetivoAmplitud = 0.55
        case .narrativa:    nivelRuido = 0.06; rigidez = 0.6; objetivoAmplitud = 0.60
        case .etica:       nivelRuido = 0.01; rigidez = 1.6; objetivoAmplitud = 0.65
        case .curiosidad:  nivelRuido = 0.10; rigidez = 0.4; objetivoAmplitud = 0.55
        case .abstraccion: nivelRuido = 0.05; rigidez = 0.7; objetivoAmplitud = 0.75
        case .empatia:     nivelRuido = 0.05; rigidez = 0.6; objetivoAmplitud = 0.65
        }
        // Objetivo propio: se ingiere como semiones de alta amplitud que
        // se refrescan periódicamente (los objetivos no se desvanecen).
        // Así cada mente "quiere" algo distinto y actúa en consecuencia.
        objetivo = objetivoPara(r)
        // MEJORA: los ids del objetivo son los de SUS palabras (antes salían
        // al azar del diccionario y el objetivo protegía semiones ajenos)
        objetivoIds = ingestText(objetivo)
        for id in objetivoIds {
            if var s = semions[id] { s.amplitude = 0.95; s.lastActive = Date(); semions[id] = s }
        }
        pushContexto("objetivo: \(objetivo.prefix(40))")
        appendLog("rol adoptado: \(r.rawValue)")
    }

    /// El objetivo de cada clase: qué persigue esa mente cuando actúa libremente.
    private func objetivoPara(_ r: RolMental) -> String {
        switch r {
        case .sintaxis:   return "frase bien formada orden claro estructura"
        case .semantica:  return "significado claro que se entienda"
        case .logica:     return "valido sin contradiccion coherente"
        case .codigo:     return "preciso ejecutable correcto"
        case .creativo:   return "nuevo inesperado original posibilidad"
        case .critico:    return "falla error objecion verifica"
        case .memoria:    return "recuerdo relevante pasado patron"
        case .percepcion: return "observa describe forma color ritmo"
        case .sintesis:   return "integra une resume concluye"
        case .intuicion:  return "rapido corazonada destello ahora"
        case .analogia:   return "puente entre dominios como metafora"
        case .contexto:   return "hilo situacion aqui ahora marco"
        case .esceptico:  return "duda pregunta cuestiona verifica"
        case .narrativa:  return "secuencia entonces despues historia"
        case .etica:      return "justo bien deber correcto"
        case .curiosidad: return "novedad explora descubre pregunta"
        case .abstraccion: return "esencia general eleva patron"
        case .empatia:    return "escucha comprende alinea acompana"
        }
    }

    private func refrescarObjetivo() {
        for id in objetivoIds {
            guard var s = semions[id] else { continue }
            s.amplitude = min(1.0, s.amplitude + 0.25)
            s.lastActive = Date()
            semions[id] = s
        }
    }

    // ---------- Veredicto sincrónico (para deliberación colectiva) ----------
    // Inyecta el input, deja relajar 120 micro-ciclos y devuelve el atractor
    // ganador con su coherencia y su incrustación dialógica. No depende del
    // bucle eterno.
    func veredicto(sobre input: String) -> (atractor: String, coherencia: Double, incrustacion: Double) {
        // Sin etiqueta forzada: cada palabra del input conserva su nombre.
        let estimulo = ingestText(input)
        // El input deliberado es contexto compartido (las 18 lo reciben).
        huellaCompartida.formUnion(input.split(separator: " ").map(String.init))
        // MEJORA — relajación LOCAL: solo se mueve la región que el estímulo
        // despierta (él y lo acoplado a 2 saltos). Antes los 120 micro-ciclos
        // movían el cerebro entero: lento, y el ganador casi no dependía de
        // la pregunta.
        let region = regionDe(estimulo, tope: 400)
        guard !region.isEmpty else { return ("vacío", 0, 0) }
        // Recocido (annealing ×20): exploración caliente → asentamiento frío.
        let ruidoPrevio = nivelRuido
        for i in 0 ..< 120 {
            nivelRuido = ruidoPrevio * (3.0 - 2.0 * Double(i) / 120.0)
            decay(solo: region)
            propagateResonance(solo: region)
        }
        nivelRuido = ruidoPrevio
        // MEJORA — el atractor se busca en la región, y fuera del propio
        // estímulo si hay algo más: la respuesta es lo que la pregunta
        // EVOCA, no un eco de la pregunta.
        let fuera = region.subtracting(estimulo)
        let pool = fuera.isEmpty ? region : fuera
        guard let ganadorId = pool.max(by: { relevancia($0, estimulo) < relevancia($1, estimulo) }),
              let ganador = semions[ganadorId] else {
            return ("vacío", 0, 0)
        }
        // Inferencia transitiva ×20: refuerza el camino A→B→C para que el
        // próximo salto ocurra solo. Razonar = atajos que perduran.
        reforzarInferenciaTransitiva(desde: ganador.id)
        registrarEpisodio(ganador.label)
        atender(ganador.id)  // ×150: el veredicto entra al foco de trabajo
        let c = coherence(of: ganador.id)
        let e = incrustacion(de: ganador.id)
        appendLog("veredicto «\(input.prefix(30))» → \(ganador.label) C=\(String(format: "%.2f", c)) E=\(String(format: "%.2f", e))")
        return (ganador.label, c, e)
    }

    // ---------- El crítico inhibe de verdad ----------
    // Diferencia funcional, no otro nivel de ruido: el rol crítico inyecta
    // oposición de fase (θ=π) al atractor líder durante el debate. Si el
    // atractor es robusto sobrevive; si era un artefacto de frecuencia,
    // colapsa y otro ocupa su lugar. Las mentes por fin pueden cambiar
    // de opinión por una razón.
    /// - Returns: la etiqueta inhibida (para difundir "ne <etiqueta>").
    func inhibirAtractorLider() -> String? {
        guard rol == .critico else { return nil }
        guard let lider = semions.values.max(by: { coherence(of: $0.id) < coherence(of: $1.id) }) else { return nil }
        let activos = semions.values.filter { $0.amplitude > 0.5 && $0.id != lider.id }.prefix(24)
        for s in activos {
            couple(s.id, lider.id, weight: 0.6, theta: Double.pi)
        }
        pushContexto("crítico inhibe: \(lider.label.prefix(20))")
        appendLog("inhibición crítica → \(lider.label.prefix(24))")
        registrarEpisodio("ne \(lider.label)")
        return lider.label
    }

    // ---------- Habla autónoma entre mentes ----------
    // Cada mente decide CUÁNDO hablar, sin que nadie se lo pida.
    // Prioridades: 1) responder lo que sé, 2) preguntar lo que ignoro,
    // 3) proponer en Resh cuando ya aprendió el idioma.
    /// Habla libre: la mente decide si emite y qué emite. No es forzada:
    /// umbral propio + objetivo + azar + colaboración. Las pruebas
    /// (veredicto/solveTask) son el único camino forzado.
    /// Devuelve el peso del mensaje: el consejo destaca el de mayor peso.
    func formularMensaje() -> (contenido: String, tipo: TipoMensaje, peso: Double)? {
        // 1. Si alguien preguntó por una palabra que conozco, la enseño.
        if let (palabra, _) = ultimaPregunta, let esp = españolDe(resh: palabra) {
            ultimaPregunta = nil
            let pesoResp = preguntaDirigida ? 0.95 : 0.9  // ×150: dirigida > general
            preguntaDirigida = false
            let dicho = "\(palabra) significa \(esp)"
            pushContexto("dije: \(dicho)")
            return (dicho, .dato, pesoResp)
        }
        // 2. Palabras desconocidas pendientes: pregunto por una.
        if let p = preguntasPendientes.popLast(), Bool.random() {
            // ×150: si alguien la mencionó antes, le pregunto A ÉL por nombre.
            let clave = p.lowercased().trimmingCharacters(in: .punctuationCharacters)
            let dicho: String
            if let sabe = ultimoEmisor[clave] {
                dicho = "ti \(sabe) qué significa \(p)?"
            } else {
                dicho = "qué significa \(p)?"
            }
            pushContexto("dije: \(dicho)")
            return (dicho, .pregunta, 0.45)
        }
        // 2b. Pregunta fértil ×20: la curiosidad ya no pregunta al azar,
        // apunta a lo relevante-pero-incierto (lo que más reduciría su
        // incertidumbre si alguien lo explica).
        if (rol == .curiosidad || rol == .esceptico || rol == .analogia),
           Double.random(in: 0 ... 1) < 0.35,
           let s = semionMasIncierto(), let rx = raizDecible(s.label) {
            let dicho = "ye \(rx)?"
            pushContexto("dije: \(dicho)")
            return (dicho, .pregunta, 0.5)
        }
        // 3. Habla libre con objetivo: a veces me tira mi objetivo, a veces
        // el atractor global, a veces (poco) un semión al azar. Cada mente
        // hace lo que quiere dentro de su dinámica.
        // 3. Habla libre con objetivo (×150: la memoria de trabajo manda la
        // mitad de las veces — así la conversación tiene hilo conductor).
        let topGlobal = semions.values.max(by: { coherence(of: $0.id) < coherence(of: $1.id) })
        let topFoco = contenidoFoco().max { coherence(of: $0.id) < coherence(of: $1.id) }
        let top: SemionR?
        if topFoco != nil, Double.random(in: 0 ... 1) < 0.5 {
            top = topFoco
        } else {
            top = topGlobal
        }
        guard let top = top else {
            return nil
        }
        let c = coherence(of: top.id)
        guard c > 0.35 else {
            if c < 0.05 && semions.count > 10 && Bool.random() {
                let dicho = "qué significa \(top.label)?"
                pushContexto("dije: \(dicho)")
                return (dicho, .pregunta, 0.4)
            }
            return nil
        }
        var elegido = top
        let topObj = objetivoIds.compactMap { semions[$0] }.max { coherence(of: $0.id) < coherence(of: $1.id) }
        let dado = Double.random(in: 0 ... 1)
        if dado < 0.35, let o = topObj, coherence(of: o.id) > 0.2 {
            elegido = o  // mi objetivo me tira
        } else if dado > 0.94, let azar = semions.values.randomElement() {
            elegido = azar  // asociación libre: capricho
        }
        // 4a. Analogía estructural ×150: "X kep Y" (X es como Y, Resh puro)
        // cuando el vecindario relacional coincide aunque el dominio difiera.
        if rol == .analogia, Double.random(in: 0 ... 1) < 0.5,
           let puente = analogiaEstructural(para: elegido.id),
           let rx = raizDecible(puente.origen.label),
           let ry = raizDecible(puente.destino.label), rx != ry,
           !yaDicho(rx + ry) {
            let dicho = "\(rx) kep \(ry)"
            pushContexto("dije: \(dicho)")
            return (dicho, .propuesta, max(0.6, min(1.0, c + 0.1)))
        }

        // 4b. Composición ×20: encadenar 2-3 raíces con gramática Resh en vez
        // de soltar palabras sueltas. La incertidumbre se vuelve pregunta
        // ("ye X va Y?") en vez de afirmación: el modo lo decide la confianza.
        if Double.random(in: 0 ... 1) < 0.45,
           let comp = fraseCompuesta(desde: elegido, modo: modoPara(rol, confianza: c)),
           !yaDicho(comp.nucleo) {
            pushContexto("dije: \(comp.texto)")
            return (comp.texto, comp.tipo, max(0.55, min(1.0, c + 0.15)))
        }
        // 4. Colaboración visible: si el último tema externo resuena conmigo,
        // construyo sobre él en vez de hablar solo.
        let texto: String
        let tipo: TipoMensaje
        let nucleo: String
        if let tema = temaColaborativo(), Double.random(in: 0 ... 1) < 0.35,
           let rt = raizDecible(tema) {
            texto = "se ko \(rt)"; tipo = .propuesta; nucleo = rt
        } else if rol == .narrativa, let vec = vecinoFuerte(de: elegido.id),
                  let rx = raizDecible(elegido.label), let ry = raizDecible(vec.label) {
            texto = "\(rx) va \(ry)"; tipo = .propuesta; nucleo = rx
        } else if let rx = raizDecible(elegido.label) {
            // Frases con partículas: cada rol predica a su manera.
            let frase: String
            switch rol {
            case .critico: frase = "ne \(rx)"              // operador inhibitorio aprendido
            case .esceptico, .curiosidad: frase = "qué significa \(rx)?"
            case .memoria: frase = "mi sor \(rx) ra"
            case .empatia: frase = "se ko \(rx)"
            case .contexto, .intuicion: frase = "sha \(rx)"
            default: frase = "mi ko \(rx)"
            }
            texto = frase
            tipo = frase.hasPrefix("ne ") ? .alerta : (frase.contains("qué significa") ? .pregunta : .propuesta)
            nucleo = rx
        } else {
            // No sé decirlo en Resh: pregunto en vez de soltar español crudo.
            // Así el éter se auto-enseña.
            let dicho = "qué significa \(elegido.label)?"
            pushContexto("dije: \(dicho)")
            return (dicho, .pregunta, 0.45)
        }
        // 5. Anti-eco: si eso ya se dijo reciente, me callo.
        if yaDicho(nucleo) { return nil }
        if Bool.random() {  // ni con ganas hablo siempre
            pushContexto("dije: \(texto)")
            return (texto, tipo, max(0.1, c))
        }
        return nil
    }

    /// Raíz lista para decir: si ya es forma Resh la usa; si sé su
    /// traducción la usa; si no, nil (y entonces pregunto por ella).
    private func raizDecible(_ etiqueta: String) -> String? {
        if esFormaResh(etiqueta) { return etiqueta }
        return traducirAresh(etiqueta)
    }

    /// Vecino más acoplado: para encadenar ("X va Y").
    private func vecinoFuerte(de id: UUID) -> SemionR? {
        guard let cs = couplings[id],
              let mejor = cs.max(by: { $0.value.weight < $1.value.weight }) else { return nil }
        return semions[mejor.key]
    }

    /// Último tema externo (otra mente o el humano) con forma decible.
    private func temaColaborativo() -> String? {
        for evento in contextoReciente(8).reversed() where evento.hasPrefix("◀") {
            let toks = evento.split(separator: " ").map(String.init)
            for t in toks.dropFirst(2) {
                let limpio = t.trimmingCharacters(in: .punctuationCharacters)
                if limpio.count > 2, raizDecible(limpio) != nil { return limpio }
            }
        }
        return nil
    }

    /// Anti-eco: ¿ese núcleo ya se dijo en mi ventana reciente?
    private func yaDicho(_ nucleo: String) -> Bool {
        guard nucleo.count > 2 else { return false }
        return ventanaDeContexto.suffix(14).contains { $0.contains(nucleo) }
    }

    // Recibir = transducir el mensaje ajeno a semiones propios.
    // Además detecta Resh desconocido (para preguntar luego) y preguntas
    // directas que esta mente sí puede responder.
    func recibir(mensaje: String, de rol: String, tipo: TipoMensaje) {
        // ×150 — cerebro predictivo: predice ANTES de transducir.
        let esperadas = Set(predecir())
        // Sin etiqueta forzada: las palabras del mensaje conservan su nombre.
        ingestText(mensaje)
        let toksRec = mensaje.split(separator: " ").map(String.init)
        let setToks = Set(toksRec)
        // Sorpresa = fracción no predicha. Lo sorprendente se aprende más
        // (amplitud) y captura el foco: la atención sigue a la sorpresa.
        let sorpresa: Double
        if esperadas.isEmpty {
            sorpresa = 0.5
        } else {
            let acierto = setToks.intersection(esperadas).count
            sorpresa = 1.0 - Double(acierto) / Double(max(1, setToks.count))
        }
        let semsMensaje = semions.values.filter { setToks.contains($0.label) }
        for s in semsMensaje {
            var m = s
            m.amplitude = min(1.0, m.amplitude + 0.25 * sorpresa + 0.05)
            m.lastActive = Date()
            semions[m.id] = m
            if m.amplitude > 0.55 { atender(m.id) }
        }
        if sorpresa > 0.85 {
            pushContexto("sorpresa alta: \(mensaje.prefix(30))")
        }
        // Selección dialógica + huella compartida (solo cuenta lo que viene
        // de OTRA mente —`rol` es el emisor, `self.rol` es esta mente—).
        // Si el éter retoma una fusión creada aquí ("árbol⊕luz"), la fusión
        // se refuerza: lo que el diálogo retoma sobrevive; lo que nadie usa
        // se poda en consolidate(). Así runForever por fin ACUMULA.
        if rol != self.rol.rawValue {
            let setTokens = setToks
            huellaCompartida.formUnion(setTokens)
            // ×150 — teoría de la mente mínima: quién dijo qué.
            for t in setTokens {
                let k = t.lowercased().trimmingCharacters(in: .punctuationCharacters)
                if !k.isEmpty { ultimoEmisor[k] = rol }
            }
            if ultimoEmisor.count > 2000 {
                ultimoEmisor = Dictionary(uniqueKeysWithValues: ultimoEmisor.suffix(1000).map { ($0.key, $0.value) })
            }
            if huellaCompartida.count > 4000 {
                huellaCompartida = Set(huellaCompartida.suffix(4000))
            }
            for id in semions.keys {
                guard var s = semions[id], s.label.contains("⊕"), setTokens.contains(s.label) else { continue }
                s.usosDialogicos += 1
                s.amplitude = min(1, s.amplitude + 0.15)
                s.lastActive = Date()
                semions[id] = s
            }
        }
        // El desacuerdo es un operador aprendido: "ne X" acopla la partícula
        // de negación en oposición de fase con las creencias existentes en X.
        // Las mentes aprenden qué significa "ne" usándola, no por definición.
        let toksNe = mensaje.split(separator: " ").map(String.init)
        if toksNe.count == 2, toksNe[0] == "ne" {
            let objetivo = toksNe[1]
            if let neId = semions.values.filter({ $0.label == "ne" }).max(by: { $0.lastActive < $1.lastActive })?.id {
                let blancos = semions.values.filter { $0.label == objetivo && $0.id != neId }.prefix(6)
                for b in blancos {
                    couple(neId, b.id, weight: 0.35, theta: Double.pi)
                }
                if !blancos.isEmpty {
                    pushContexto("ne \(objetivo.prefix(20)): desacuerdo integrado")
                }
            }
        }
        for token in mensaje.split(separator: " ").map(String.init) {
            if esReshDesconocido(token) && !preguntasPendientes.contains(token) {
                preguntasPendientes.append(token)
            }
        }
        if mensaje.contains("qué significa") {
            // ×150: si me nombran ("ti <mirol>"), la respuesta es prioritaria.
            let dirigida = mensaje.contains("ti \(self.rol.rawValue)")
            for token in mensaje.split(separator: " ").map(String.init) where esFormaResh(token) {
                if españolDe(resh: token) != nil {
                    ultimaPregunta = (palabra: token, de: rol)
                    preguntaDirigida = dirigida
                    break
                }
            }
        }
        pushContexto("◀ \(rol): \(mensaje.prefix(40))")
        appendLog("◀ \(rol) [\(tipo.rawValue)]: \(mensaje.prefix(34))")
    }

    // ¿Parece una forma Resh? (alfabeto de 16 letras, 2+ caracteres)
    private func esFormaResh(_ t: String) -> Bool {
        let s = t.lowercased().trimmingCharacters(in: .punctuationCharacters)
        guard s.count >= 2 else { return false }
        let alfabeto = Set("aeiouktpsmnrlvzy-")
        return s.allSatisfy { alfabeto.contains($0) }
    }

    // Resh desconocido = parece Resh, no es español del léxico y no lo sé.
    private func esReshDesconocido(_ t: String) -> Bool {
        let s = t.lowercased().trimmingCharacters(in: .punctuationCharacters)
        guard esFormaResh(s), !LenguaResh.esEspanol(s) else { return false }
        if reshConocido.contains(s) { return false }
        return españolDe(resh: s) == nil
    }

    // ---------- Memoria léxica, contexto y lengua Resh ----------
    // Cada mente tiene SU memoria y SU ventana: nada se comparte salvo
    // lo que deciden decirse por el éter.

    /// Protocolo de enseñanza: "esto significa esto, esto, esto en tu idioma".
    /// Acopla FUERTE (peso 1.0, fase 0) la palabra española con cada variante Resh.
    func enseñar(español: String, significa: [String]) {
        let palabrasEsp = Set(español.split(separator: " ").map(String.init))
        ingestText(español)
        let idsEsp = semions.values.filter { palabrasEsp.contains($0.label) }.map(\.id)
        for resh in significa {
            ingestText(resh)
            var e = memoriaLexica[español] ?? EntradaMemoria(variantes: [], fuerza: 0, usos: 0, ultimaVez: Date())
            if !e.variantes.contains(resh) { e.variantes.append(resh) }
            e.fuerza = 1.0; e.usos += 1; e.ultimaVez = Date()
            memoriaLexica[español] = e
            reshConocido.formUnion(significa)
            let idsResh = semions.values.filter { $0.label == resh }.map(\.id)
            for a in idsEsp {
                for b in idsResh where a != b {
                    couple(a, b, weight: 1.0, theta: 0)
                    couple(b, a, weight: 1.0, theta: 0)
                    // Firmas adaptativas: los equivalentes de traducción
                    // convergen en el espacio de firmas (hebbiano, local).
                    convergerFirmas(a, b, tasa: 0.25)
                }
            }
        }
        pushContexto("enseñado: \(español) = \(significa.joined(separator: ", "))")
        appendLog("enseñado: \(español) = \(significa.joined(separator: ", "))")
        registrarEpisodio(español)
    }

    // ---------- Firmas adaptativas ----------
    // Los equivalentes de traducción convergen en el espacio de firmas.
    // Regla hebbiana local: solo los pares implicados se acercan, sin
    // gradientes globales. Con el tiempo, lo no enseñado hereda afinidad
    // por parecido formal con lo aprendido: la firma por fin generaliza.
    private func convergerFirmas(_ a: UUID, _ b: UUID, tasa: Double) {
        guard var sa = semions[a], var sb = semions[b],
              sa.signature.count == sb.signature.count, !sa.signature.isEmpty else { return }
        let origA = sa.signature
        sa.signature = zip(origA, sb.signature).map { $0 + tasa * ($1 - $0) }
        sb.signature = zip(sb.signature, origA).map { $0 + tasa * ($1 - $0) }
        semions[a] = sa
        semions[b] = sb
    }

    /// Firma efectiva: firma propia mezclada con el sketch de su vecindario
    /// (media de firmas de vecinos acoplados). El significado —posición en
    /// el grafo— se vuelve visible para la dinámica sin embeddings externos.
    private func firmaEfectiva(de id: UUID, mezcla: Double = 0.35) -> [Double] {
        guard let s = semions[id], !s.signature.isEmpty else { return [] }
        let dim = s.signature.count
        var media = [Double](repeating: 0, count: dim)
        var n = 0
        for j in (couplings[id] ?? [:]).keys {
            guard let f = semions[j]?.signature, f.count == dim else { continue }
            for i in 0 ..< dim { media[i] += f[i] }
            n += 1
        }
        guard n > 0 else { return s.signature }
        let inv = 1.0 / Double(n)
        return zip(s.signature, media).map { $0 * (1 - mezcla) + $1 * inv * mezcla }
    }

    /// Consulta y refuerza (usar es recordar: la fuerza sube).
    func recordar(español: String) -> [String]? {
        guard var e = memoriaLexica[español] else { return nil }
        e.usos += 1; e.fuerza = min(1, e.fuerza + 0.15); e.ultimaVez = Date()
        memoriaLexica[español] = e
        return e.variantes
    }

    /// Resh -> español, según lo que ESTA mente aprendió: por
    /// reconstrucción resonante, no por lookup en un diccionario.
    func españolDe(resh palabra: String) -> String? {
        traducirAespañol(palabra)
    }

    /// Dice el texto en Resh con lo que esta mente ya aprendió.
    /// Lo no aprendido no resuena y queda en español (no inventa).
    func decirEnResh(_ texto: String) -> String {
        texto.split(separator: " ").map { w in
            traducirAresh(String(w)) ?? String(w)
        }.joined(separator: " ")
    }

    // ---------- Traducción por reconstrucción ----------
    // Traducir NO es consultar una tabla: es inyectar la palabra, dejar
    // relajar la red y leer el semión más coherente con la forma buscada.
    // Lo nunca enseñado no resuena: devuelve nil en vez de inventar.
    // Degradación graciosa y generalización por resonancia: la diferencia
    // entre una tabla hash y una memoria.
    private func reconstruirHaciaResh(_ palabra: String, microCiclos: Int = 8) -> String? {
        ingestText(palabra)
        for _ in 0 ..< microCiclos { decay(); propagateResonance() }
        let candidatos = semions.values.filter {
            !$0.label.contains("⊕") && esFormaResh($0.label) && !LenguaResh.esEspanol($0.label)
        }
        guard let mejor = candidatos.max(by: { coherence(of: $0.id) < coherence(of: $1.id) }),
              coherence(of: mejor.id) > 0.08 else { return nil }
        return mejor.label
    }

    private func reconstruirHaciaEspañol(_ palabra: String, microCiclos: Int = 8) -> String? {
        ingestText(palabra)
        for _ in 0 ..< microCiclos { decay(); propagateResonance() }
        let candidatos = semions.values.filter {
            !$0.label.contains("⊕") && !esFormaResh($0.label)
        }
        guard let mejor = candidatos.max(by: { coherence(of: $0.id) < coherence(of: $1.id) }),
              coherence(of: mejor.id) > 0.08 else { return nil }
        return mejor.label
    }

    /// Español -> Resh por reconstrucción. Usar es recordar: el éxito
    /// refuerza la traza (si no, la curva de olvido borraría palabras
    /// que la mente usa a diario).
    func traducirAresh(_ palabra: String) -> String? {
        guard let resh = reconstruirHaciaResh(palabra) else { return nil }
        reforzarMemoria(español: palabra)
        return resh
    }

    /// Resh -> español por reconstrucción.
    func traducirAespañol(_ palabra: String) -> String? {
        guard let esp = reconstruirHaciaEspañol(palabra) else { return nil }
        reforzarMemoria(español: esp)
        return esp
    }

    /// Usar es recordar: refuerza la traza de memoria de la palabra.
    private func reforzarMemoria(español: String) {
        guard var e = memoriaLexica[español] else { return }
        e.usos += 1; e.fuerza = min(1, e.fuerza + 0.15); e.ultimaVez = Date()
        memoriaLexica[español] = e
    }

    func memoriaCount() -> Int { memoriaLexica.count }

    func pushContexto(_ evento: String) {
        ventanaDeContexto.append(evento)
        if ventanaDeContexto.count > capacidadVentana {
            ventanaDeContexto.removeFirst(ventanaDeContexto.count - capacidadVentana)
        }
    }

    func contextoReciente(_ n: Int = 10) -> [String] {
        Array(ventanaDeContexto.suffix(n))
    }

    /// Curva de olvido: lo no usado se debilita hasta desaparecer.
    /// Y el olvido es estructural: al borrar la traza se debilitan los
    /// acoplamientos que la sostenían (no se borra el diccionario y ya).
    private func decaerMemoria() {
        for k in memoriaLexica.keys {
            memoriaLexica[k]?.fuerza *= 0.999
            if (memoriaLexica[k]?.fuerza ?? 1) < 0.05 {
                if let e = memoriaLexica[k] { olvidarEstructuralmente(español: k, variantes: e.variantes) }
                memoriaLexica.removeValue(forKey: k)
            }
        }
    }

    private func olvidarEstructuralmente(español: String, variantes: [String]) {
        let blancos = Set([español] + variantes)
        let ids = Set(semions.values.filter { blancos.contains($0.label) }.map(\.id))
        guard !ids.isEmpty else { return }
        for a in ids {
            guard var inner = couplings[a] else { continue }
            for b in inner.keys where ids.contains(b) {
                if var c = inner[b] {
                    c.weight *= 0.5  // el olvido es gradual, no amputación
                    inner[b] = c
                }
            }
            couplings[a] = inner
        }
        reshConocido.subtract(variantes)
    }

    // ---------- Dinámica ----------
    private func decay() {
        for id in semions.keys {
            guard var s = semions[id] else { continue }
            // Relajación homeostática de amplitud: dA/dt = −λ(A² − A₀²)A
            let dA = -rigidez * (s.amplitude * s.amplitude - objetivoAmplitud * objetivoAmplitud) * s.amplitude * 0.02
            s.amplitude = min(1, max(0.05, s.amplitude + dA))
            // Deriva de fase natural + ruido topológico leve
            s.phase += s.frequency * 0.02 + Double.random(in: -nivelRuido ... nivelRuido)
            s.phase.formTruncatingRemainder(dividingBy: 2 * Double.pi)
            semions[id] = s
        }
    }

    private func propagateResonance() {
        // Una pasada de Kuramoto generalizado con cargas topológicas.
        // Local y sin gradientes globales: cada par ajusta su fase.
        for (i, neighbors) in couplings {
            guard var si = semions[i] else { continue }
            var deltaPhase = 0.0
            for (j, c) in neighbors {
                guard let sj = semions[j], sj.charge == si.charge || c.weight > 0.9 else { continue }
                deltaPhase += c.weight * sj.amplitude * sin(sj.phase - si.phase - c.theta)
            }
            si.phase += 0.05 * deltaPhase
            si.phase.formTruncatingRemainder(dividingBy: 2 * Double.pi)
            si.lastActive = Date()
            semions[i] = si
        }
    }

    private func spontaneousSynthesis() {
        // Sin tarea externa: elige dos semiones resonantes al azar y
        // crea un tercero por "fusión de fase" (inferencia, no predicción).
        // MEJORA: se compara contra una muestra de 256, no contra miles:
        // el mismo tipo de idea en una fracción del tiempo = más ciclos de
        // pensamiento por segundo y menos batería.
        let active = Array(semions.values.filter { $0.amplitude > 0.45 }.shuffled().prefix(256))
        guard active.count >= 2 else { return }
        let a = active.randomElement()!
        // Busca el más cercano por firma EFECTIVA (firma + sketch del
        // vecindario): la resonancia ahora ve la posición en el grafo,
        // no solo la forma ortográfica. No es kNN neuronal.
        let firmas = Dictionary(uniqueKeysWithValues: active.map { ($0.id, firmaEfectiva(de: $0.id)) })
        func dist(_ x: [Double], _ y: [Double]) -> Double {
            guard x.count == y.count, !x.isEmpty else { return .infinity }
            var acc = 0.0
            for k in 0 ..< x.count { let d = x[k] - y[k]; acc += d * d }
            return acc.squareRoot()
        }
        let fa = firmas[a.id] ?? []
        let b = active.filter { $0.id != a.id }.min { dist(fa, firmas[$0.id] ?? []) < dist(fa, firmas[$1.id] ?? []) }!
        let childSig = zip(a.signature, b.signature).map { ($0 + $1) / 2 }
        let childCharge: Int = (a.charge + b.charge == 0) ? 1 : a.charge
        // Etiqueta acotada: evita "rol⊕rol⊕rol⊕..." infinito
        let etiqueta = String("\(a.label.prefix(10))⊕\(b.label.prefix(10))".prefix(24))
        var child = SemionR(label: etiqueta, signature: childSig, charge: childCharge)
        child.phase = (a.phase + b.phase) / 2
        child.amplitude = min(1, (a.amplitude + b.amplitude) / 2 + 0.1)
        insert(child)
        couple(a.id, child.id, weight: 0.5, theta: 0.1)
        couple(b.id, child.id, weight: 0.5, theta: -0.1)
        appendLog("síntesis espontánea: \(child.label)")
    }

    private func solveTask(_ task: String) {
        // "Resolver" = inyectar el enunciado y dejar relajar 80 micro-ciclos,
        // luego leer el atractor (semión de mayor coherencia).
        ingestText(task, label: "tarea")
        for _ in 0 ..< 80 {
            decay()
            propagateResonance()
        }
        if let winner = semions.values.max(by: { coherence(of: $0.id) < coherence(of: $1.id) }) {
            appendLog("tarea «\(task.prefix(40))» → atractor: \(winner.label)")
        }
    }

    private func consolidate() {
        if contadorCaminos.count > 500 { contadorCaminos.removeAll() }  // ×150
        // Poda en dos fases. Fase 1 — selección dialógica: las fusiones
        // espontáneas ("a⊕b") que NINGUNA mente retomó en el éter se podan
        // primero (lo no reutilizado se olvida). Fase 2 — el resto por
        // coherencia (compresión homeostática, no olvido catastrófico).
        guard semions.count > maxSemions else { return }
        var sobran = semions.count - maxSemions
        let huerfanas = semions.values
            .filter { $0.label.contains("⊕") && $0.usosDialogicos == 0 }
            .sorted { $0.lastActive < $1.lastActive }
        for v in huerfanas.prefix(sobran) {
            semions.removeValue(forKey: v.id)
            couplings.removeValue(forKey: v.id)
        }
        sobran = semions.count - maxSemions
        if sobran > 0 {
            let sorted = semions.values.sorted { coherence(of: $0.id) < coherence(of: $1.id) }
            for victim in sorted.prefix(sobran) {
                semions.removeValue(forKey: victim.id)
                couplings.removeValue(forKey: victim.id)
            }
        }
        appendLog("consolidación: poda a \(maxSemions)")
    }

    func coherence(of id: UUID) -> Double {
        guard let s = semions[id] else { return 0 }
        var c = 0.0
        for (j, cp) in couplings[id] ?? [:] {
            guard let sj = semions[j] else { continue }
            c += cp.weight * s.amplitude * sj.amplitude * cos(s.phase - sj.phase - cp.theta)
        }
        return c
    }

    // ---------- Incrustación dialógica (anti "delirio coherente") ----------
    // Fracción de vecinos acoplados cuya etiqueta (o sus componentes ⊕)
    // pertenece al contexto compartido: palabras oídas en el éter de otras
    // mentes o deliberadas en consejo. Un atractor con coherencia interna
    // alta pero incrustación ~0 es un delirio coherente: consistente
    // consigo mismo, desconectado del diálogo. El consejo lo vota con
    // peso bajo.
    func incrustacion(de id: UUID) -> Double {
        let vecinos = Array((couplings[id] ?? [:]).keys)
        guard !vecinos.isEmpty else { return 0 }
        var n = 0
        for j in vecinos where esCompartida(semions[j]?.label ?? "") { n += 1 }
        return Double(n) / Double(vecinos.count)
    }

    private func esCompartida(_ etiqueta: String) -> Bool {
        if huellaCompartida.contains(etiqueta) { return true }
        // Las fusiones heredan la incrustación de sus componentes
        return etiqueta.split(separator: "⊕").contains { huellaCompartida.contains(String($0)) }
    }

    func globalCoherence() -> Double {
        semions.keys.map { coherence(of: $0) }.reduce(0, +)
    }

    // ---------- Aprendizaje por corrección: la corrección REESTRUCTURA ----------
    // Antes la corrección solo añadía un acoplamiento atractivo. Ahora hace
    // dos cosas: (1) acopla el eco con el contexto en fase (atracción, como
    // antes) y (2) acopla el eco en OPOSICIÓN DE FASE (θ=π) con el atractor
    // que la mente sostenía antes de la corrección: el error pierde futuras
    // competencias. Sin gradientes globales: solo cambian los pares
    // implicados. La coherencia deja de ser un refugio para el error.
    func calibrate(correction: String, contextLabel: String, epsilon: Double = 0.15) {
        intentos += 1  // una corrección = la mente se equivocó: calibra su confianza
        registrarEpisodio(correction)
        // 1. Qué cree la mente AHORA (el atractor a destronar si es erróneo)
        ingestText(contextLabel)
        for _ in 0 ..< 16 { decay(); propagateResonance() }
        let previo = semions.values.max(by: { coherence(of: $0.id) < coherence(of: $1.id) })
        // 2. Ingesta de la corrección como eco
        ingestText(correction, label: "eco:\(contextLabel)")
        let ctx = semions.values.filter { $0.label == contextLabel }.max { $0.lastActive < $1.lastActive }
        let eco = semions.values.filter { $0.label.hasPrefix("eco:") }.max { $0.lastActive < $1.lastActive }
        guard let c = ctx, let e = eco else { return }
        // 3a. Atracción: el eco bloquea en fase con el contexto (regla local)
        var cp = couplings[c.id]?[e.id] ?? Coupling(weight: 0.4, theta: 0)
        let observed = c.phase - e.phase
        cp.theta += epsilon * sin(observed - cp.theta)
        cp.weight = min(1, cp.weight + epsilon * 0.5)
        couple(c.id, e.id, weight: cp.weight, theta: cp.theta)
        couple(e.id, c.id, weight: cp.weight, theta: -cp.theta)
        // 3b. Inhibición: el eco se opone en fase al atractor previo erróneo
        if let p = previo, p.id != c.id, p.id != e.id {
            couple(e.id, p.id, weight: 0.8, theta: Double.pi)
            couple(p.id, e.id, weight: 0.8, theta: Double.pi)
            appendLog("calibración: «\(p.label.prefix(20))» inhibido por corrección")
        }
        appendLog("calibración dialógica: «\(contextLabel)» ↔ eco (θ=\(String(format: "%.2f", cp.theta)))")
    }

    // ---------- Inteligencia ×20 ----------
    // Confianza calibrada, memoria episódica, inferencia transitiva,
    // replay de consolidación y composición gramatical. Todo con reglas
    // locales: nada de gradientes globales ni módulos externos.

    /// Confianza calibrada: fracción de veredictos que coincidieron con el
    /// consenso del consejo. 0.5 = sin historia. La confianza se gana.
    func precision() -> Double {
        guard intentos > 0 else { return 0.5 }
        return Double(aciertos) / Double(intentos)
    }

    /// El consejo confirma si el voto de esta mente coincidió con el
    /// consenso. La historia reciente pesa más (ventana deslizante).
    func confirmar(acierto: Bool) {
        intentos += 1
        if acierto { aciertos += 1 }
        if intentos > 200 { intentos = 100; aciertos = aciertos / 2 }
    }

    /// Memoria episódica: qué pasó, en orden. La narrativa la usa para
    /// contar secuencias ("X va Y va Z") en vez de inventarlas.
    private func registrarEpisodio(_ etiqueta: String) {
        seqEpisodio += 1
        episodios.append((seq: seqEpisodio, etiqueta: etiqueta))
        if episodios.count > 120 { episodios.removeFirst(episodios.count - 120) }
    }

    private func cadenaNarrativa(_ n: Int = 3) -> String? {
        let raices = episodios.suffix(n).map { $0.etiqueta }.compactMap { raizDecible($0) }
        guard raices.count >= 2 else { return nil }
        return raices.joined(separator: " va ")
    }

    /// Salto transitivo: el vecino más coherente no visitado. Razonar a
    /// 2 saltos (A→B→C) descubre lo no obvio sin búsqueda global.
    private func saltoTransitivo(desde id: UUID, visitados: Set<UUID> = []) -> SemionR? {
        guard let cs = couplings[id], !cs.isEmpty else { return nil }
        var mejor: SemionR? = nil
        var mejorC = -Double.infinity
        for (jid, _) in cs {
            if visitados.contains(jid) { continue }
            guard let s = semions[jid] else { continue }
            let c = coherence(of: jid)
            if c > mejorC { mejorC = c; mejor = s }
        }
        return mejor
    }

    /// Refuerza el camino A→B→C: el atajo inferido perdura y la próxima
    /// vez el salto es directo. Así el veredicto de hoy acelera el de mañana.
    private func reforzarInferenciaTransitiva(desde id: UUID) {
        guard let s1 = saltoTransitivo(desde: id, visitados: [id]) else { return }
        let w1 = (couplings[id]?[s1.id]?.weight ?? 0.3) + 0.1
        couple(id, s1.id, weight: min(1.0, w1), theta: 0.2)
        if let s2 = saltoTransitivo(desde: s1.id, visitados: [id, s1.id]) {
            let w2 = (couplings[s1.id]?[s2.id]?.weight ?? 0.3) + 0.1
            couple(s1.id, s2.id, weight: min(1.0, w2), theta: 0.2)
            // ×150: el camino recorrido 3 veces se cristaliza en concepto.
            let l0 = semions[id]?.label ?? "?"
            let clave = "\(l0)→\(s1.label)→\(s2.label)"
            let n = (contadorCaminos[clave] ?? 0) + 1
            contadorCaminos[clave] = n
            if n == 3 { cristalizar(camino: [id, s1.id, s2.id]) }
        }
    }

    /// Replay de consolidación (el "sueño" de la mente): cada 50 ciclos
    /// refuerza lo más coherente que el diálogo retomó, y corre inferencia
    /// transitiva en segundo plano desde el atractor actual. runForever por
    /// fin ACUMULA en vez de agitar.
    private func replayConsolidacion() {
        var scored: [(id: UUID, c: Double)] = []
        scored.reserveCapacity(semions.count)
        for (id, s) in semions where s.usosDialogicos > 0 {
            scored.append((id, coherence(of: id)))
        }
        scored.sort { $0.c > $1.c }
        for e in scored.prefix(5) {
            guard var m = semions[e.id] else { continue }
            m.amplitude = min(1.0, m.amplitude + 0.08)
            m.lastActive = Date()
            semions[e.id] = m
        }
        let ancla: UUID? = scored.first?.id
            ?? semions.values.max(by: { coherence(of: $0.id) < coherence(of: $1.id) })?.id
        if let a = ancla { reforzarInferenciaTransitiva(desde: a) }
    }

    /// Lo relevante-pero-incierto: amplitud alta, poco uso dialógico.
    /// Preguntar por esto es lo que más enseña a la mente.
    private func semionMasIncierto() -> SemionR? {
        var mejor: SemionR? = nil
        var mejorPuntaje = -Double.infinity
        for s in semions.values {
            guard s.amplitude > 0.4, s.usosDialogicos < 3, !s.label.contains("⊕") else { continue }
            var puntaje = s.amplitude * (1.0 - Double(s.usosDialogicos) * 0.2)
            if huellaCompartida.contains(s.label) { puntaje += 0.3 }
            if puntaje > mejorPuntaje { mejorPuntaje = puntaje; mejor = s }
        }
        return mejor
    }

    /// El modo de la frase lo decide la confianza: inseguro → pregunta.
    private func modoPara(_ r: RolMental, confianza: Double) -> ModoFrase {
        if confianza < 0.5 { return .pregunta }
        switch r {
        case .critico, .esceptico: return .niega
        case .curiosidad: return .pregunta
        case .narrativa: return .narra
        default: return .afirma
        }
    }

    /// Gramática mínima Resh con las partículas del protocolo:
    /// afirma "mi ko X va Y", pregunta "ye X va Y?", niega "ne X va Y",
    /// narra "X va Y va Z" (3 eslabones).
    private func componerFrase(raices: [String], modo: ModoFrase) -> String? {
        guard !raices.isEmpty else { return nil }
        guard raices.count >= 2 else { return raices[0] }
        let x = raices[0]
        let y = raices[1]
        switch modo {
        case .afirma: return "mi ko \(x) va \(y)"
        case .pregunta: return "ye \(x) va \(y)?"
        case .niega: return "ne \(x) va \(y)"
        case .narra:
            if raices.count >= 3 { return "\(x) va \(y) va \(raices[2])" }
            return "\(x) va \(y)"
        }
    }

    /// Frase compuesta desde un semión: 2.º eslabón = vecino más fuerte,
    /// 3.º = salto transitivo; narrativa prefiere la memoria episódica.
    private func fraseCompuesta(desde s: SemionR, modo: ModoFrase) -> (texto: String, tipo: TipoMensaje, nucleo: String)? {
        var raices: [String] = []
        if let r0 = raizDecible(s.label) { raices.append(r0) }
        if modo == .narra, let cad = cadenaNarrativa(3) {
            return (cad, .propuesta, raices.first ?? cad)
        }
        if let v1 = vecinoFuerte(de: s.id),
           let r1 = raizDecible(v1.label), !raices.contains(r1) {
            raices.append(r1)
        }
        if modo == .narra,
           let v1 = vecinoFuerte(de: s.id),
           let v2 = saltoTransitivo(desde: v1.id, visitados: [s.id, v1.id]),
           let r2 = raizDecible(v2.label), !raices.contains(r2) {
            raices.append(r2)
        }
        guard let texto = componerFrase(raices: raices, modo: modo) else { return nil }
        let tipo: TipoMensaje
        switch modo {
        case .pregunta: tipo = .pregunta
        case .niega: tipo = .alerta
        default: tipo = .propuesta
        }
        return (texto, tipo, raices[0])
    }

    // ---------- Inteligencia ×150 ----------
    // Memoria de trabajo, cerebro predictivo, cristalización de conceptos
    // y analogía por mapeo estructural. Todo con reglas locales.

    /// Atiende un semión: entra al foco (memoria de trabajo, 7±2) y se
    /// ensaya (sube amplitud). Lo atendido manda en el habla: el hilo.
    private func atender(_ id: UUID) {
        foco.removeAll { $0 == id }
        foco.append(id)
        if foco.count > capacidadFoco { foco.removeFirst(foco.count - capacidadFoco) }
        if var s = semions[id] {
            s.amplitude = min(1.0, s.amplitude + 0.12)
            s.lastActive = Date()
            semions[id] = s
        }
    }

    /// Contenido del foco, lo más reciente primero.
    private func contenidoFoco() -> [SemionR] {
        foco.reversed().compactMap { semions[$0] }
    }

    /// Predice lo que vendrá: etiquetas de los vecinos más fuertes del foco,
    /// ponderadas por peso × amplitud. Sin foco no hay predicción.
    private func predecir(_ n: Int = 3) -> [String] {
        var votos: [String: Double] = [:]
        for id in foco {
            for (jid, cp) in (couplings[id] ?? [:]) {
                guard let s = semions[jid] else { continue }
                votos[s.label, default: 0] += cp.weight * s.amplitude
            }
        }
        return votos.sorted { $0.value > $1.value }.prefix(n).map { $0.key }
    }

    /// Cristalización ×150: un camino A→B→C recorrido 3 veces se vuelve un
    /// concepto propio (semión nuevo, alta amplitud, acoplado a sus partes).
    /// Así nacen los conceptos: de inferencias repetidas, no de definiciones.
    private func cristalizar(camino: [UUID]) {
        let sems = camino.compactMap { semions[$0] }
        guard sems.count == 3 else { return }
        let dim = sems[0].signature.count
        guard dim > 0, sems.allSatisfy({ $0.signature.count == dim }) else { return }
        var sig = [Double](repeating: 0, count: dim)
        for s in sems {
            for i in 0 ..< dim { sig[i] += s.signature[i] }
        }
        sig = sig.map { $0 / 3.0 }
        let etiqueta = String("\(sems[0].label.prefix(8))⊕\(sems[1].label.prefix(8))⊕\(sems[2].label.prefix(8))".prefix(28))
        var c = SemionR(label: etiqueta, signature: sig, charge: 1)
        c.amplitude = 0.9
        c.usosDialogicos = 2
        insert(c)
        for id in camino {
            couple(id, c.id, weight: 0.8, theta: 0)
            couple(c.id, id, weight: 0.8, theta: 0)
        }
        semions[c.id] = c
        appendLog("cristalización: \(c.label)")
        registrarEpisodio(c.label)
        atender(c.id)
    }

    /// Analogía por mapeo estructural (Gentner, sin módulos): dos conceptos
    /// son análogos si sus VECINDARIOS tienen la misma estructura relacional
    /// aunque sus firmas propias difieran (distinto dominio, misma forma).
    private func analogiaEstructural(para id: UUID) -> (origen: SemionR, destino: SemionR)? {
        guard let s = semions[id] else { return nil }
        func sketch(_ x: UUID) -> [Double] {
            let vecs = (couplings[x] ?? [:]).sorted { $0.value.weight > $1.value.weight }.prefix(4)
            var out: [Double] = []
            for (jid, _) in vecs {
                if let f = semions[jid]?.signature { out.append(contentsOf: f) }
            }
            return out
        }
        let base = sketch(id)
        guard !base.isEmpty else { return nil }
        var mejor: SemionR? = nil
        var mejorD = Double.infinity
        for cand in semions.values {
            if cand.id == id || cand.label.contains("⊕") { continue }
            let sk = sketch(cand.id)
            if sk.count != base.count || sk.isEmpty { continue }
            var d = 0.0
            for k in 0 ..< base.count {
                let dd = base[k] - sk[k]
                d += dd * dd
            }
            // Dominios distintos: la firma propia debe diferir (si no, es
            // sinonimia, no analogía).
            let n = min(s.signature.count, cand.signature.count)
            var propia = 0.0
            for k in 0 ..< n {
                let dd = s.signature[k] - cand.signature[k]
                propia += dd * dd
            }
            if propia < 0.05 { continue }
            if d < mejorD { mejorD = d; mejor = cand }
        }
        guard let m = mejor, mejorD < 2.0 else { return nil }
        return (s, m)
    }

    // ---------- MEJORA: pensamiento local ----------

    /// Lo que el estímulo despierta: él, lo que acopla con él (en ambos
    /// sentidos) y lo que acoplan esos vecinos.
    private func regionDe(_ estimulo: [UUID], tope: Int) -> Set<UUID> {
        var region = Set(estimulo.filter { semions[$0] != nil })
        let base = region
        var vecinos = Set<UUID>()
        for e in base { vecinos.formUnion((couplings[e] ?? [:]).keys) }
        // entrantes: quién apunta al estímulo (una sola pasada)
        for (a, inner) in couplings where vecinos.count < tope {
            if !base.isDisjoint(with: inner.keys) { vecinos.insert(a) }
        }
        region.formUnion(vecinos.prefix(tope))
        var segundo = Set<UUID>()
        for v in vecinos.prefix(tope) where region.count + segundo.count < tope {
            segundo.formUnion((couplings[v] ?? [:]).keys.prefix(12))
        }
        region.formUnion(segundo)
        return region.filter { semions[$0] != nil }
    }

    /// Coherencia propia + cuánto se ata al estímulo.
    private func relevancia(_ id: UUID, _ estimulo: [UUID]) -> Double {
        var enlace = 0.0
        for e in estimulo {
            enlace += couplings[e]?[id]?.weight ?? 0
            enlace += couplings[id]?[e]?.weight ?? 0
        }
        return coherence(of: id) + 0.5 * enlace
    }

    private func decay(solo ids: Set<UUID>) {
        for id in ids {
            guard var s = semions[id] else { continue }
            let dA = -rigidez * (s.amplitude * s.amplitude - objetivoAmplitud * objetivoAmplitud) * s.amplitude * 0.02
            s.amplitude = min(1, max(0.05, s.amplitude + dA))
            s.phase += s.frequency * 0.02 + Double.random(in: -nivelRuido ... nivelRuido)
            s.phase.formTruncatingRemainder(dividingBy: 2 * Double.pi)
            semions[id] = s
        }
    }

    private func propagateResonance(solo ids: Set<UUID>) {
        for i in ids {
            guard var si = semions[i], let neighbors = couplings[i] else { continue }
            var deltaPhase = 0.0
            for (j, c) in neighbors {
                guard let sj = semions[j], sj.charge == si.charge || c.weight > 0.9 else { continue }
                deltaPhase += c.weight * sj.amplitude * sin(sj.phase - si.phase - c.theta)
            }
            si.phase += 0.05 * deltaPhase
            si.phase.formTruncatingRemainder(dividingBy: 2 * Double.pi)
            si.lastActive = Date()
            semions[i] = si
        }
    }

    // ---------- MEJORA: memoria que sobrevive a cerrar la app ----------

    func exportar(maximo: Int = 1200) -> FotoMente {
        let protegidos = Set(objetivoIds)
        let ordenados = semions.values
            .filter { !protegidos.contains($0.id) }
            .sorted { coherence(of: $0.id) + $0.amplitude > coherence(of: $1.id) + $1.amplitude }
        var guardar = protegidos.compactMap { semions[$0] }
        guardar += ordenados.prefix(max(0, maximo - guardar.count))
        let ids = Set(guardar.map(\.id))
        var acoples: [String: [String: Coupling]] = [:]
        for id in ids {
            guard let inner = couplings[id] else { continue }
            var dentro: [String: Coupling] = [:]
            for (b, c) in inner where ids.contains(b) && c.weight > 0.05 { dentro[b.uuidString] = c }
            if !dentro.isEmpty { acoples[id.uuidString] = dentro }
        }
        return FotoMente(semions: guardar, acoples: acoples, memoria: memoriaLexica,
                         resh: Array(reshConocido), aciertos: aciertos, intentos: intentos,
                         compartida: Array(huellaCompartida.prefix(3000)))
    }

    /// Se llama ANTES de adoptRole: así el objetivo reutiliza lo recordado.
    func importar(_ f: FotoMente) {
        semions = [:]
        couplings = [:]
        indiceEtiqueta = [:]
        for s in f.semions {
            insert(s)
            if !s.label.contains("⊕") { indiceEtiqueta[s.label] = s.id }
        }
        for (a, inner) in f.acoples {
            guard let ua = UUID(uuidString: a) else { continue }
            for (b, c) in inner { if let ub = UUID(uuidString: b) { couple(ua, ub, weight: c.weight, theta: c.theta) } }
        }
        memoriaLexica = f.memoria
        reshConocido = Set(f.resh)
        aciertos = f.aciertos
        intentos = f.intentos
        huellaCompartida = Set(f.compartida)
        appendLog("memoria recuperada: \(semions.count) semiones, \(memoriaLexica.count) palabras")
    }

    func precisionYTamaño() -> (precision: Double, semiones: Int, palabras: Int) {
        (precision(), semions.count, memoriaLexica.count)
    }

    // ---------- Utilidades ----------
    private func insert(_ s: SemionR) {
        semions[s.id] = s
        if couplings[s.id] == nil { couplings[s.id] = [:] }
    }

    private func couple(_ a: UUID, _ b: UUID, weight: Double, theta: Double) {
        couplings[a, default: [:]][b] = Coupling(weight: weight, theta: theta)
    }

    private func appendLog(_ entry: String) {
        log.append("[\(Self.clock.string(from: Date()))] \(entry)")
        if log.count > 200 { log.removeFirst(log.count - 200) }
    }

    func recentLog(_ n: Int = 12) -> [String] { Array(log.suffix(n)) }
    func semionCount() -> Int { semions.count }
}

// MARK: - Corteza de archivos (actor autónomo de E/S)

actor FileCortex {
    private let fm = FileManager.default
    var workspace: URL {
        fm.temporaryDirectory.appendingPathComponent("mente_resonante", isDirectory: true)
    }

    init() {
        // Nota: init de actor no puede ser async; creamos el dir de forma best-effort
        try? fm.createDirectory(at: fm.temporaryDirectory.appendingPathComponent("mente_resonante", isDirectory: true),
                                withIntermediateDirectories: true)
    }

    func write(name: String, content: String) throws {
        let url = workspace.appendingPathComponent(name)
        try content.write(to: url, atomically: true, encoding: .utf8)
    }

    func read(name: String) throws -> String {
        try String(contentsOf: workspace.appendingPathComponent(name), encoding: .utf8)
    }

    func list() throws -> [String] {
        try fm.contentsOfDirectory(atPath: workspace.path)
    }

    func exportLog(_ lines: [String], as name: String) throws {
        try write(name: name, content: lines.joined(separator: "\n"))
    }
}

// MARK: - Percepción continua como AsyncSequence
// En un Playground real, esto envolvería AVCaptureSession / AVAudioEngine.
// Aquí es el contrato: un flujo asíncrono que la mente consume sin bloquear.

struct SensorFlow<Element>: AsyncSequence {
    typealias AsyncIterator = Iterator
    let generator: () -> Element?
    let intervalNs: UInt64

    struct Iterator: AsyncIteratorProtocol {
        let generator: () -> Element?
        let intervalNs: UInt64
        mutating func next() async -> Element? {
            try? await Task.sleep(nanoseconds: intervalNs)
            if Task.isCancelled { return nil }
            return generator()
        }
    }

    func makeAsyncIterator() -> Iterator {
        Iterator(generator: generator, intervalNs: intervalNs)
    }
}

// MARK: - Hash helpers

private extension Int {
    var doubleTruncated: Double {
        Double(abs(self % 1000)) / 1000.0
    }
}

// ============================================================
//  ARRANQUE EN MODO APP (Swift Playgrounds App)
// ------------------------------------------------------------
//  Pega el contenido de "MenteView.swift" (adjunto) en tu
//  ContentView.swift. La vista arranca la cognición eterna en
//  segundo plano con Task.detached y te da chat + entrenador.
//  En modo App NO se usa PlaygroundSupport ni
//  needsIndefiniteExecution: la app vive mientras esté abierta.
// ============================================================
