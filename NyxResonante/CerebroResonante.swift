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

struct Semion: Identifiable, Hashable {
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
    func morphDistance(to other: Semion) -> Double {
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

struct Coupling {
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
struct EntradaMemoria {
    var variantes: [String]  // formas Resh que significan lo mismo
    var fuerza: Double       // 0..1: decae con el tiempo, se refuerza con uso
    var usos: Int
    var ultimaVez: Date
}

// ---------- Cuadro visual: lo que el ojo ve ----------
// Un fotograma retinotópico (64×64 luminancia): más fino que la visión
// periférica, menos que una foto. Lo produce cualquier fuente
// (demo sintética, URL, cámara) y lo consumen las mentes con ver(_:).
// La vista previa que ve el humano va a resolución completa (ver
// FotogramaOjo): lo que tú ves es el live, lo que ellas ven es el mapa.
struct CuadroVisual {
    static let lado = 64
    let pixeles: [Float]   // lado×lado, 0..1
    let movimiento: Float  // 0..1: cambio vs el cuadro anterior
    /// Vector de flujo (formato video): hacia dónde se mueve lo que se
    /// mueve, en fracción de cuadro por segundo. Lo calcula quien produce
    /// el cuadro ((centroide actual − centroide anterior) / dt).
    var flujoX: Float = 0
    var flujoY: Float = 0
    let fuente: String     // "demo", "url", "camara"
    let fecha: Date

    /// Energía de cambio entre dos cuadros (para fuentes reales).
    static func movimiento(desde a: CuadroVisual, hasta p: [Float]) -> Float {
        // Tamaños distintos (cambio de fuente) no deben crashear.
        let n = min(a.pixeles.count, p.count)
        guard n > 0 else { return 0 }
        var acc: Float = 0
        for i in 0 ..< n { acc += abs(a.pixeles[i] - p[i]) }
        return min(1, acc / Float(n) * 3)
    }

    /// Demo en vivo: mancha brillante que deriva sobre ruido suave.
    /// Siempre funciona (no necesita cámara ni red): el live de prueba.
    static func demo(t: Double, anterior: CuadroVisual?) -> CuadroVisual {
        let l = lado
        var p = [Float](repeating: 0, count: l * l)
        let cx = Float(l) / 2 + 8 * Float(sin(t * 0.7))
        let cy = Float(l) / 2 + 8 * Float(cos(t * 0.45))
        for y in 0 ..< l {
            for x in 0 ..< l {
                let dx = Float(x) - cx, dy = Float(y) - cy
                let d = (dx * dx + dy * dy).squareRoot()
                let blob = max(0, 1 - d / 7)
                p[y * l + x] = min(1, blob * blob + Float.random(in: 0 ... 0.12))
            }
        }
        let mov = anterior.map { movimiento(desde: $0, hasta: p) } ?? 0.5
        return CuadroVisual(pixeles: p, movimiento: mov, fuente: "demo", fecha: Date())
    }
}

// MARK: - Mente resonante (actor: estado aislado, seguro en concurrencia)

actor ResonantMind {
    private var semions: [UUID: Semion] = [:]
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
    // Pregunta por una relación ("ye X va Y?") que esta mente puede contestar.
    private var preguntaRelacion: (x: String, y: String, de: String)?
    // ---- Inteligencia ×1500: arousal, chunking y metacognición de modos ----
    private var arousal = 0.5                      // activación: la sorpresa la sube
    private var coFoco: [String: Int] = [:]        // tríos que co-ocurren en el foco
    private var ultimoModo: ModoFrase = .afirma
    private var usoModo: [ModoFrase: Int] = [:]
    private var exitoModo: [ModoFrase: Int] = [:]
    private var raicesEmitidas: [String] = []      // raíces dichas, esperando retoma
    private var ciclosPensados: Int = 0
    private var rigidez = 0.8             // λ: rigidez homeostática de amplitud
    private var objetivoAmplitud = 0.6    // A₀: amplitud objetivo
    private var nivelRuido = 0.05         // η: ruido topológico
    private let maxSemions = 1200       // iPad: 1200 por mente (×18 = 21.600 totales)

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
    // Desde el nacer incluye el idioma COMPLETO (base innata): ver
    // conocerIdiomaCompleto(). Lo vivido se distingue por memoriaLexica.
    private var reshConocido: Set<String> = []
    /// El idioma es innato: conozco las 5844 formas + partículas desde el
    /// nacer. La red de semiones (cap 1200) es mi experiencia vivida.
    private var idiomaInnata = false

    private static let clock: DateFormatter = {
        let df = DateFormatter()
        df.dateFormat = "HH:mm:ss"
        return df
    }()

    // ---------- Procesamiento de datos: índice y tokenización ----------
    // Índice etiqueta → semión: buscar una palabra es O(1) en vez de
    // recorrer los 1200 semiones. También evita duplicados: la misma
    // palabra oída 50 veces es UN semión que se refuerza, no 50 copias
    // que llenan la red y expulsan lo aprendido.
    private var indiceEtiqueta: [String: UUID] = [:]
    // Huella compartida en orden de llegada (FIFO real: un Set no tiene
    // orden, así que podar con suffix() borraba palabras al azar).
    private var ordenHuella: [String] = []
    private static let maxHuella = 4000
    // Cuántas palabras de un texto entran por ingesta (antes 24 fijas,
    // y el resto se perdía). Si hay más, se eligen las más salientes.
    private static let maxTokensIngesta = 48

    /// Tokenizador: separa por cualquier espacio o salto de línea, pasa a
    /// minúsculas y quita la puntuación pegada ("hola," = "hola",
    /// "kash?" = "kash"). Los compuestos Resh ("luz-calor") y las
    /// fusiones ("a⊕b") se conservan intactos.
    static func tokenizar(_ texto: String) -> [String] {
        texto.split(whereSeparator: { $0.isWhitespace || $0.isNewline }).compactMap { w in
            let t = String(w).lowercased()
                .trimmingCharacters(in: CharacterSet.punctuationCharacters.subtracting(CharacterSet(charactersIn: "-")))
                .trimmingCharacters(in: CharacterSet(charactersIn: "-"))
            return t.isEmpty ? nil : t
        }
    }

    /// Semión vivo con esa etiqueta (O(1)).
    private func semionCon(etiqueta: String) -> Semion? {
        guard let id = indiceEtiqueta[etiqueta] else { return nil }
        return semions[id]
    }

    /// El más coherente de un conjunto, calculando cada coherencia UNA vez
    /// (max(by:) con coherence() en el comparador la calculaba 2 veces
    /// por comparación).
    private func masCoherente<S: Sequence>(_ candidatos: S) -> Semion? where S.Element == Semion {
        var mejor: Semion? = nil
        var mejorC = -Double.infinity
        for s in candidatos {
            let c = coherence(of: s.id)
            if c > mejorC { mejorC = c; mejor = s }
        }
        return mejor
    }

    /// Añade palabras a la huella compartida con poda FIFO real.
    private func registrarCompartido<S: Sequence>(_ tokens: S) where S.Element == String {
        for t in tokens where !t.isEmpty && !huellaCompartida.contains(t) {
            huellaCompartida.insert(t)
            ordenHuella.append(t)
        }
        if ordenHuella.count > Self.maxHuella {
            let sobran = ordenHuella.count - Self.maxHuella
            for t in ordenHuella.prefix(sobran) { huellaCompartida.remove(t) }
            ordenHuella.removeFirst(sobran)
        }
    }

    /// Acople hebbiano: si el par ya estaba acoplado, el uso lo refuerza
    /// (y conserva su desfase aprendido) en vez de pisarlo. Antes una
    /// frase cualquiera reescribía a 0.7 un acople enseñado a 1.0 o una
    /// oposición (θ=π) aprendida del mundo o del crítico.
    /// `tope`: hasta dónde puede crecer por repetición. La co-ocurrencia
    /// en frases se queda por debajo del umbral de equivalencia (0.85):
    /// "qué" y "significa" van juntas mil veces y no son sinónimos.
    private func reforzarAcople(_ a: UUID, _ b: UUID, peso: Double, theta: Double, tope: Double = 1.0) {
        guard a != b else { return }
        if var c = couplings[a]?[b] {
            c.weight = max(c.weight, min(tope, max(c.weight, peso) + 0.03))
            couplings[a]?[b] = c
        } else {
            couple(a, b, weight: peso, theta: theta)
        }
    }

    // ---------- Ingesta: texto → semiones ----------
    // Sin tokenizador BPE ni embeddings preentrenados.
    // La "firma" se deriva de rasgos formales: longitud, simetría,
    // densidad simbólica, ritmo de puntuación. Es transducción, no lookup.
    /// - Returns: los ids de los semiones de la frase, en orden.
    @discardableResult
    func ingestText(_ text: String, label: String? = nil) -> [UUID] {
        var words = Self.tokenizar(text)
        guard !words.isEmpty else { return [] }
        // Textos largos: en vez de cortar a ciegas, quedarse con las
        // palabras más salientes (frecuentes en el texto, con contenido,
        // no partículas) conservando su orden original.
        if words.count > Self.maxTokensIngesta {
            var frec: [String: Int] = [:]
            for w in words { frec[w, default: 0] += 1 }
            func saliencia(_ w: String) -> Double {
                let contenido = w.count > 3 ? 1.0 : (w.count > 2 ? 0.6 : 0.2)
                let particula = LenguaResh.esParticula(w) ? 0.3 : 1.0
                return Double(frec[w] ?? 1).squareRoot() * contenido * particula
            }
            let elegidos = Set(words.indices
                .sorted { saliencia(words[$0]) > saliencia(words[$1]) || (saliencia(words[$0]) == saliencia(words[$1]) && $0 < $1) }
                .prefix(Self.maxTokensIngesta))
            words = words.indices.filter { elegidos.contains($0) }.map { words[$0] }
        }
        var ids: [UUID] = []
        ids.reserveCapacity(words.count)
        let ahora = Date()
        for w in words {
            let etiqueta = label ?? w
            if let id = indiceEtiqueta[etiqueta], var s = semions[id] {
                // Ya existe: oír de nuevo = reforzar, no duplicar.
                s.amplitude = min(1.0, s.amplitude + 0.1)
                s.lastActive = ahora
                semions[id] = s
                if ids.last != id { ids.append(id) }
            } else {
                let s = Semion(label: etiqueta, signature: signatureForText(w))
                insert(s)
                ids.append(s.id)
            }
        }
        // Acoplar secuencialmente (sintaxis como resonancia, no como posición).
        // Antes se acoplaban semions.values.suffix(n): un diccionario no tiene
        // orden, así que se unían semiones AL AZAR. Ahora es la frase real:
        // vecino directo fuerte, vuelta débil, y salto de 2 (contexto).
        for k in 0 ..< ids.count {
            if k + 1 < ids.count {
                reforzarAcople(ids[k], ids[k + 1], peso: 0.7, theta: 0.3, tope: 0.8)
                reforzarAcople(ids[k + 1], ids[k], peso: 0.35, theta: -0.3, tope: 0.8)
            }
            if k + 2 < ids.count {
                reforzarAcople(ids[k], ids[k + 2], peso: 0.3, theta: 0.6, tope: 0.8)
            }
        }
        appendLog("ingesta texto: \(ids.count) semiones")
        pushContexto("ingesta: \(text.prefix(40))")
        return ids
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
        return Self.sanear([min(1, len / 20.0),
                mean.truncatingRemainder(dividingBy: 128) / 128.0,
                min(1, variance / 4000.0),
                symmetry / max(1, len),
                digitDensity,
                upperDensity,
                vowelDensity,
                symbolDensity])
    }

    /// Higiene de datos: toda firma queda con 8 dims finitas en 0...1.
    /// Un NaN (p. ej. de un cuadro vacío) contaminaba cada distancia y
    /// cada fusión que lo tocara.
    static let dimFirma = 8
    static func sanear(_ f: [Double]) -> [Double] {
        var out = f.prefix(dimFirma).map { $0.isFinite ? min(1, max(0, $0)) : 0 }
        while out.count < dimFirma { out.append(0) }
        return out
    }

    /// Hash estable (FNV-1a). `hashValue` de Swift cambia en cada arranque,
    /// así que la misma etiqueta daba firmas distintas entre sesiones.
    static func hashEstable(_ s: String) -> Double {
        var h: UInt64 = 0xcbf29ce484222325
        for b in s.utf8 { h ^= UInt64(b); h = h &* 0x100000001b3 }
        return Double(h % 1000) / 1000.0
    }

    // ---------- Ingesta: visión y audición (transducción directa) ----------
    // En Playgrounds esto recibe descriptores ya extraídos (p. ej. de
    // Vision/AVFoundation) y los convierte a semiones sin CNN.
    func ingestImage(signature: [Double], label: String) {
        var s = Semion(label: "img:\(label)", signature: Self.sanear(signature), charge: 2)
        s.amplitude = 0.8
        insert(s)
        appendLog("ingesta imagen: \(label)")
    }

    func ingestAudio(samples: [Double], label: String) {
        // Firma rítmica de 8 dims (mismo espacio que texto y visión, antes
        // eran 3 y no se podía comparar bien con lo demás): energía, cruces
        // por cero, pico, factor de cresta, brillo, ataque, rango dinámico
        // y hash estable de la etiqueta.
        let limpias = samples.filter(\.isFinite)
        let n = Double(max(1, limpias.count))
        let energy = limpias.reduce(0) { $0 + $1 * $1 } / n
        var zeroCross = 0
        var brillo = 0.0
        for (a, b) in zip(limpias, limpias.dropFirst()) {
            if (a < 0) != (b < 0) { zeroCross += 1 }
            brillo += abs(b - a)
        }
        let absolutas = limpias.map { abs($0) }
        let pico = absolutas.max() ?? 0
        let rms = energy.squareRoot()
        let cresta = rms > 0 ? pico / rms : 0
        let ataque = Double(absolutas.firstIndex(of: pico) ?? 0) / n
        let suelo = absolutas.min() ?? 0
        let sig = Self.sanear([min(1, energy * 10),
                               Double(zeroCross) / n,
                               min(1, pico),
                               min(1, cresta / 10),
                               min(1, brillo / n),
                               ataque,
                               min(1, pico - suelo),
                               Self.hashEstable(label)])
        var s = Semion(label: "aud:\(label)", signature: sig, charge: -1)
        s.frequency = 1.0 + min(3, energy * 20)
        insert(s)
        appendLog("ingesta audio: \(label)")
    }

    // ---------- Núcleo: ciclo de pensamiento ----------
    func thinkCycle() {
        decay()
        propagateResonance()
        decaerMemoria()
        arousal = max(0.2, arousal * 0.999)  // ×1500: la calma vuelve sola
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
            try? await Task.sleep(nanoseconds: 150_000_000) // 150 ms — iPad
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
        // Los ids salen de la ingesta misma (antes: semions.values.suffix,
        // que en un diccionario son semiones al azar, no el objetivo).
        objetivoIds = ingestText(objetivo)
        for id in objetivoIds {
            if var s = semions[id] { s.amplitude = 0.95; s.lastActive = Date(); semions[id] = s }
        }
        pushContexto("objetivo: \(objetivo.prefix(40))")
        appendLog("rol adoptado: \(r.rawValue)")
        // Idioma innato: cada mente nace conociendo el idioma completo.
        conocerIdiomaCompleto()
        // Vocabulario del rol ×150: cada agente nace sabiendo 150 palabras
        // hechas para él (acopladas fuerte, como si se las hubieran enseñado).
        // Viven en el léxico común, así que las demás mentes también pueden
        // usarlas: las aprenden por el éter cuando el especialista las dice.
        if let idx = RolMental.allCases.firstIndex(of: r) {
            var n = 0
            for esp in LenguaResh.espanolParaRol(indiceRol: idx) {
                if let resh = LenguaResh.traducir(esp) {
                    enseñar(español: esp, significa: [resh], silencioso: true)
                    n += 1
                }
            }
            appendLog("vocabulario del rol: \(n) palabras")
        }
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
        let idsInput = ingestText(input)
        // El input deliberado es contexto compartido (las 18 lo reciben).
        registrarCompartido(Self.tokenizar(input))
        // Relevancia: el veredicto es SOBRE el input. Antes ganaba el semión
        // más coherente de toda la red aunque no tuviera nada que ver con la
        // pregunta. Ahora lo que el input toca (y sus vecinos) pesa más.
        var relevancia: [UUID: Double] = [:]
        for id in idsInput {
            relevancia[id] = 1.0
            for j in (couplings[id] ?? [:]).keys where relevancia[j] == nil { relevancia[j] = 0.6 }
        }
        func puntaje(_ s: Semion) -> Double {
            let c = coherence(of: s.id)
            let r = relevancia[s.id] ?? 0
            return c >= 0 ? c * (1 + 0.5 * r) : c
        }
        func ganadorActual() -> Semion? {
            var mejor: Semion? = nil
            var mejorP = -Double.infinity
            for s in semions.values {
                let p = puntaje(s)
                if p > mejorP { mejorP = p; mejor = s }
            }
            return mejor
        }
        // Recocido (annealing ×20): exploración caliente → asentamiento frío.
        // El ruido decae a lo largo de los 120 micro-ciclos: primero salta
        // entre atractores candidatos, al final se asienta en el mejor.
        let ruidoPrevio = nivelRuido
        // Salida temprana: si el ganador no cambia en 30 microciclos, ya
        // convergió; no hace falta quemar los 120 (en iPad × 18 mentes,
        // una deliberación completa sin corte puede tardar segundos).
        var ganadorPrevio: String? = nil
        var rachaEstable = 0
        for i in 0 ..< 120 {
            // ×1500: el arousal modula la exploración (neutro en 0.5).
            nivelRuido = ruidoPrevio * (3.0 - 2.0 * Double(i) / 120.0) * (0.7 + 0.6 * arousal)
            decay()
            propagateResonance(completa: true)
            if i % 10 == 9 {
                let g = ganadorActual()?.label
                if g == ganadorPrevio { rachaEstable += 1 } else { ganadorPrevio = g; rachaEstable = 0 }
                if rachaEstable >= 2 { break }
            }
        }
        nivelRuido = ruidoPrevio
        guard let ganador = ganadorActual() else {
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

    /// Razonamiento contrafáctico ×1500: ¿qué decidiría SIN este atractor?
    /// Suprime el semión, re-delibera 120 microciclos y lo restaura intacto.
    /// Pensar en "qué pasaría si" es la prueba de robustez del veredicto.
    func veredictoContrafactual(sobre input: String, sin etiqueta: String) -> (atractor: String, coherencia: Double) {
        var guardadas: [UUID: Double] = [:]
        for v in semions.values where v.label == etiqueta {
            guardadas[v.id] = v.amplitude
            var m = v
            m.amplitude = 0.001
            semions[v.id] = m
        }
        let r = veredicto(sobre: input)
        for (id, amp) in guardadas {
            if var m = semions[id] { m.amplitude = amp; semions[id] = m }
        }
        return (r.atractor, r.coherencia)
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
        guard let lider = masCoherente(semions.values) else { return nil }
        let activos = semions.values.filter { $0.amplitude > 0.5 && $0.id != lider.id }.prefix(24)
        for s in activos {
            couple(s.id, lider.id, weight: 0.6, theta: Double.pi)
        }
        pushContexto("crítico inhibe: \(lider.label.prefix(20))")
        appendLog("inhibición crítica → \(lider.label.prefix(24))")
        registrarEpisodio("ne \(lider.label)")
        return lider.label
    }

    // ---------- Primer grounding: el Mundo ----------
    // Hasta aquí todo era símbolos apuntando a símbolos. Un SucesoMundo
    // es una CONSECUENCIA: algo que pasó fuera (un sensor, el resultado
    // de una acción, "sí, eso era" / "no, mal" del humano) con valor -1..1.
    // Toca LOCALMENTE los acoplamientos de los semiones que nombran el
    // hecho (hebbiano con signo, sin gradientes globales) y el hecho entra
    // a huellaCompartida: lo que tuvo consecuencias pesa más al votar.
    // Así la "verdad" deja de ser solo coherencia interna.
    func absorberSuceso(_ suceso: SucesoMundo) {
        let ids = Set(ingestText(suceso.hecho))
        registrarCompartido(Self.tokenizar(suceso.hecho))
        guard !ids.isEmpty else { return }
        for a in ids {
            guard var inner = couplings[a] else { continue }
            for b in inner.keys {
                if var c = inner[b] {
                    c.weight = min(1.0, max(0.05, c.weight + 0.15 * suceso.valor))
                    if suceso.valor < -0.5 { c.theta = Double.pi }       // lo malo se opone
                    else if suceso.valor > 0.5, c.theta == Double.pi { c.theta = 0 }  // lo bueno reconcilia
                    inner[b] = c
                }
            }
            couplings[a] = inner
        }
        pushContexto("mundo: \(suceso.hecho.prefix(40)) → \(String(format: "%+.1f", suceso.valor))")
    }

    // ---------- Ojo: visión resonante en vivo ----------
    // Ver no es clasificar: el cuadro se convierte a firma visual (mismo
    // espacio de 8 dims que las firmas de texto) y RESUENA con los semiones
    // activos. Ver = reconocer por resonancia + enlazar lo visto.
    // Sin CNN, sin etiquetas: pura dinámica local. Como un humano viendo
    // un live: el cuadro entra en tiempo real y cada mente lo resuena
    // a su manera (dos mentes pueden "ver" cosas distintas del mismo cuadro).
    //
    // El buffer visual es UN solo semión "veo" por mente que se actualiza
    // en cada cuadro (no uno por cuadro: insertar al cap expulsaría y
    // erosionaría la red con cada fotograma del live).
    private var veoId: UUID?
    private var veoVisto: String?
    func ver(_ cuadro: CuadroVisual) -> String {
        let firma = firmaVisual(de: cuadro)
        let activos = Array(semions.values.filter { $0.amplitude > 0.4 }.prefix(400))
        func dist(_ x: [Double], _ y: [Double]) -> Double {
            guard x.count == y.count, !x.isEmpty else { return .infinity }
            var acc = 0.0
            for k in 0 ..< x.count { let d = x[k] - y[k]; acc += d * d }
            return acc.squareRoot()
        }
        let firmas = Dictionary(uniqueKeysWithValues: activos.map { ($0.id, firmaEfectiva(de: $0.id)) })
        let parecido = activos.min {
            dist(firma, firmas[$0.id] ?? []) < dist(firma, firmas[$1.id] ?? [])
        }
        let nombre = parecido.flatMap { raizDecible($0.label) } ?? "nuevo"
        let amp = min(1, 0.45 + Double(cuadro.movimiento) * 0.5)
        if let id = veoId, var s = semions[id] {
            s.signature = firma
            s.amplitude = amp
            s.phase = Double.random(in: 0 ..< 2 * Double.pi)
            s.lastActive = Date()
            if let p = parecido { s.charge = p.charge }
            semions[id] = s
        } else {
            var veo = Semion(label: "veo", signature: firma, charge: parecido?.charge ?? 2)
            veo.amplitude = amp
            veo.phase = Double.random(in: 0 ..< 2 * Double.pi)
            insert(veo)
            veoId = veo.id
        }
        // Re-enlazo solo cuando cambia lo reconocido (escena nueva).
        // A 30 fps el log/contexto solo anotan el cambio, no cada cuadro.
        let cambioEscena = veoVisto != nombre
        if cambioEscena {
            if let p = parecido, let vid = veoId {
                couple(vid, p.id, weight: 0.9, theta: 0.05)
                couple(p.id, vid, weight: 0.9, theta: -0.05)
            }
            veoVisto = nombre
            pushContexto("veo: \(nombre.prefix(24))")
            appendLog("veo [\(cuadro.fuente)]: \(nombre.prefix(24)) mov=\(String(format: "%.2f", cuadro.movimiento))")
        }
        if cuadro.movimiento > 0.55 {
            arousal = min(1, arousal + 0.25)   // el movimiento sobresalta
        }
        return nombre
    }

    /// Firma visual de 8 dims (mismo espacio que signatureForText):
    /// brillo medio, varianza, 4 cuadrantes, movimiento, dirección del flujo.
    private func firmaVisual(de c: CuadroVisual) -> [Double] {
        let p = c.pixeles
        let l = CuadroVisual.lado
        // Cuadro vacío o de otro tamaño: firma neutra en vez de NaN o crash.
        guard p.count == l * l else { return Self.sanear([0, 0, 0, 0, 0, 0, Double(c.movimiento), 0.5]) }
        let n = Double(p.count)
        let media = Double(p.reduce(0, +)) / n
        let varianza = Double(p.map { ($0 - Float(media)) * ($0 - Float(media)) }.reduce(0, +)) / n
        var cuad = [Double](repeating: 0, count: 4)
        for y in 0 ..< l {
            for x in 0 ..< l {
                let v = Double(p[y * l + x])
                cuad[(y < l / 2 ? 0 : 2) + (x < l / 2 ? 0 : 1)] += v
            }
        }
        let porCuad = n / 4
        // Octavo dim = dirección del flujo (formato video): hacia dónde se
        // mueve la escena, 0...1 como ángulo. El flujo va en fracción de
        // cuadro por segundo; si la velocidad es baja, 0.5 (neutral).
        let vx = Double(c.flujoX), vy = Double(c.flujoY)
        let vel = (vx * vx + vy * vy).squareRoot()
        let dir = vel > 0.03 ? atan2(vy, vx) / (2 * Double.pi) + 0.5 : 0.5
        return Self.sanear([media,
                min(1, varianza * 4),
                cuad[0] / porCuad, cuad[1] / porCuad,
                cuad[2] / porCuad, cuad[3] / porCuad,
                Double(c.movimiento),
                dir])
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
            // ×1500 — enseñanza proactiva: además del significado, regalo una
            // palabra vecina para que el éter siga aprendiendo solo.
            var dicho = "\(palabra) significa \(esp)"
            if let s = semionCon(etiqueta: palabra),
               let vec = vecinoFuerte(de: s.id),
               let r = raizDecible(vec.label), r != palabra, !LenguaResh.esParticula(r) {
                dicho += " va \(r)"
            }
            pushContexto("dije: \(dicho)")
            return (dicho, .dato, pesoResp)
        }
        // 1b. Me preguntaron por una relación ("ye X va Y?"): contesto con lo
        // que mi red sabe — ko si van juntas, ne si se oponen. Si no lo sé,
        // me callo (no invento).
        if let (x, y, _) = preguntaRelacion {
            preguntaRelacion = nil
            if let sx = semionCon(etiqueta: x), let sy = semionCon(etiqueta: y),
               let c = couplings[sx.id]?[sy.id] ?? couplings[sy.id]?[sx.id] {
                var dicho: String? = nil
                if c.weight >= 0.5 && cos(c.theta) > 0.5 { dicho = "ko \(x) va \(y)" }
                else if c.weight >= 0.3 && cos(c.theta) < -0.5 { dicho = "ne \(x) va \(y)" }
                if let d = dicho, !yaDicho(d) {
                    pushContexto("dije: \(d)")
                    return (d, d.hasPrefix("ne") ? .alerta : .dato, 0.85)
                }
            }
        }
        // 2. Palabras desconocidas pendientes: pregunto por una.
        // (Primero el azar y luego sacar: antes se sacaba y, si el azar
        // decía que no, la pregunta se perdía para siempre.)
        if Bool.random(), let p = preguntasPendientes.popLast() {
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
        let topGlobal = masCoherente(semions.values)
        let topFoco = masCoherente(contenidoFoco())
        let top: Semion?
        if topFoco != nil, Double.random(in: 0 ... 1) < 0.5 {
            top = topFoco
        } else {
            top = topGlobal
        }
        guard let top = top else {
            return nil
        }
        let c = coherence(of: top.id)
        guard c > 0.12 else {
            if c < 0.05 && semions.count > 10 && Bool.random() && esPreguntable(top.label) {
                let dicho = "qué significa \(top.label)?"
                pushContexto("dije: \(dicho)")
                return (dicho, .pregunta, 0.4)
            }
            return nil
        }
        var elegido = top
        let topObj = masCoherente(objetivoIds.compactMap { semions[$0] })
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
        let modo = modoPara(rol, confianza: c)
        if Double.random(in: 0 ... 1) < 0.45,
           let comp = fraseCompuesta(desde: elegido, modo: modo),
           !yaDicho(comp.nucleo) {
            // ×1500: registro modo y raíces — si el éter las retoma, el modo
            // suma éxito y se calibra solo.
            ultimoModo = modo
            usoModo[modo, default: 0] += 1
            let rcs = comp.texto.split(separator: " ").map {
                $0.trimmingCharacters(in: .punctuationCharacters)
            }.filter { $0.count > 2 }
            raicesEmitidas.append(contentsOf: rcs)
            if raicesEmitidas.count > 200 { raicesEmitidas.removeFirst(raicesEmitidas.count - 200) }
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
            case .critico: frase = "ne \(rx)"
            case .esceptico, .curiosidad: frase = "ye \(rx)?"
            case .memoria:
                frase = "mi sor \(cadenaNarrativa(2) ?? rx) ra"
            case .empatia: frase = "se ko \(rx)"
            case .contexto, .intuicion: frase = "sha \(rx)"
            case .narrativa:
                // (El vecino se dice en Resh: antes salía su etiqueta cruda,
                // a veces en español o una fusión "a⊕b".)
                if let cad = cadenaNarrativa(3) {
                    frase = cad
                } else if let ry = vecinoDecible(de: elegido.id), ry != rx {
                    frase = "\(rx) va \(ry)"
                } else {
                    frase = "\(rx) ra"
                }
            case .sintesis, .logica:
                if let ry = vecinoDecible(de: elegido.id), ry != rx {
                    frase = "mi ko \(rx) va \(ry)"
                } else {
                    frase = "mi ko \(rx)"
                }
            case .analogia:
                if let puente = analogiaEstructural(para: elegido.id),
                   let ry = raizDecible(puente.destino.label) {
                    frase = "\(rx) kep \(ry)"
                } else {
                    frase = "mi ko \(rx)"
                }
            default: frase = "mi ko \(rx)"
            }
            texto = frase
            tipo = frase.hasPrefix("ne ") ? .alerta : ((frase.contains("qué significa") || frase.hasPrefix("ye ")) ? .pregunta : .propuesta)
            nucleo = rx
        } else {
            // No sé decirlo en Resh: pregunto en vez de soltar español crudo.
            // Así el éter se auto-enseña.
            // (Nunca por fusiones "a⊕b" ni por el marco de la pregunta.)
            guard esPreguntable(elegido.label) else { return nil }
            let dicho = "qué significa \(elegido.label)?"
            pushContexto("dije: \(dicho)")
            return (dicho, .pregunta, 0.45)
        }
        // 5. Anti-eco: si eso ya se dijo reciente, me callo.
        if yaDicho(nucleo) { return nil }
        if Double.random(in: 0 ... 1) < 0.85 {  // iPad: 85% habla
            pushContexto("dije: \(texto)")
            return (texto, tipo, max(0.1, c))
        }
        return nil
    }

    /// Raíz lista para decir: si ya es forma Resh la usa; si sé su
    /// traducción la usa; si no, nil (y entonces pregunto por ella).
    /// Las partículas son gramática, no contenido: "mi ko va" o "ye ko?"
    /// no dicen nada, así que nunca se usan como raíz.
    /// Tampoco el marco de las preguntas ("qué", "significa"): son cómo
    /// se pregunta, no de qué se habla.
    private static let marcoPregunta: Set<String> = ["qué", "que", "significa", "ton"]
    private func raizDecible(_ etiqueta: String) -> String? {
        guard !Self.marcoPregunta.contains(etiqueta) else { return nil }
        let r = esFormaResh(etiqueta) ? etiqueta : traducirAresh(etiqueta)
        guard let raiz = r, !LenguaResh.esParticula(raiz), raiz != "kep" else { return nil }
        return raiz
    }

    /// Una etiqueta por la que tiene sentido preguntar "qué significa".
    private func esPreguntable(_ l: String) -> Bool {
        esEtiquetaLexica(l) && !Self.marcoPregunta.contains(l) && l.count > 1
    }

    /// El vecino más acoplado que se pueda decir en Resh.
    private func vecinoDecible(de id: UUID) -> String? {
        guard let cs = couplings[id] else { return nil }
        for (j, _) in cs.sorted(by: { $0.value.weight > $1.value.weight }).prefix(5) {
            if let l = semions[j]?.label, !l.contains("⊕"), let r = raizDecible(l) { return r }
        }
        return nil
    }

    /// Vecino más acoplado: para encadenar ("X va Y").
    private func vecinoFuerte(de id: UUID) -> Semion? {
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
        // Tokens normalizados (sin "?" ni "," pegados): antes "kash?" y
        // "kash" eran palabras distintas y las preguntas no se entendían.
        let idsMensaje = ingestText(mensaje)
        let toksRec = Self.tokenizar(mensaje)
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
        let semsMensaje = idsMensaje.compactMap { semions[$0] }
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
        // ×1500 — arousal: la sorpresa activa, lo predecible calma.
        arousal = min(1.0, max(0.1, arousal + 0.3 * (sorpresa - 0.5)))
        // Selección dialógica + huella compartida (solo cuenta lo que viene
        // de OTRA mente —`rol` es el emisor, `self.rol` es esta mente—).
        // Si el éter retoma una fusión creada aquí ("árbol⊕luz"), la fusión
        // se refuerza: lo que el diálogo retoma sobrevive; lo que nadie usa
        // se poda en consolidate(). Así runForever por fin ACUMULA.
        if rol != self.rol.rawValue {
            let setTokens = setToks
            registrarCompartido(toksRec)
            // ×150 — teoría de la mente mínima: quién dijo qué.
            for k in setTokens { ultimoEmisor[k] = rol }
            if ultimoEmisor.count > 2000 {
                ultimoEmisor = Dictionary(uniqueKeysWithValues: ultimoEmisor.suffix(1000).map { ($0.key, $0.value) })
            }
            // ×1500: si el éter retoma raíces que dije, mi último modo suma
            // éxito (así cada mente calibra su propio estilo de hablar).
            let retomadas = setTokens.intersection(raicesEmitidas)
            if !retomadas.isEmpty {
                exitoModo[ultimoModo, default: 0] += 1
                raicesEmitidas.removeAll { retomadas.contains($0) }
            }
            // Índice O(1) en vez de recorrer toda la red buscando fusiones.
            for t in setTokens where t.contains("⊕") {
                guard let id = indiceEtiqueta[t], var s = semions[id] else { continue }
                s.usosDialogicos += 1
                s.amplitude = min(1, s.amplitude + 0.15)
                s.lastActive = Date()
                semions[id] = s
            }
        }
        // El desacuerdo es un operador aprendido: "ne X" acopla la partícula
        // de negación en oposición de fase con las creencias existentes en X.
        // Las mentes aprenden qué significa "ne" usándola, no por definición.
        // (Ahora también entiende "ne X va Y": niega cada raíz nombrada.)
        if toksRec.count >= 2, toksRec[0] == "ne" {
            let objetivos = Set(toksRec.dropFirst().filter { !LenguaResh.esParticula($0) })
            if let neId = indiceEtiqueta["ne"] {
                let blancos = objetivos.compactMap { indiceEtiqueta[$0] }.filter { $0 != neId }.compactMap { semions[$0] }.prefix(6)
                let objetivo = objetivos.sorted().joined(separator: " ")
                for b in blancos {
                    couple(neId, b.id, weight: 0.35, theta: Double.pi)
                }
                if !blancos.isEmpty {
                    pushContexto("ne \(objetivo.prefix(20)): desacuerdo integrado")
                }
            }
        }
        for token in toksRec {
            if esReshDesconocido(token) && !preguntasPendientes.contains(token) {
                preguntasPendientes.append(token)
            }
        }
        // Cota: si nadie responde, las preguntas no se acumulan sin fin.
        if preguntasPendientes.count > 32 { preguntasPendientes.removeFirst(preguntasPendientes.count - 32) }
        // Comprensión del Resh: leer la frase, no solo transducirla.
        let preguntaPorRelacion = comprender(mensaje, de: rol)
        if !preguntaPorRelacion && (mensaje.contains("qué significa") || toksRec.first == "ye") {
            // ×150: si me nombran ("ti <mirol>"), la respuesta es prioritaria.
            let dirigida = mensaje.contains("ti \(self.rol.rawValue)")
            // Se busca la palabra PREGUNTADA: se saltan las partículas
            // ("ye", "ti", "va"...) y los nombres de rol. Antes la primera
            // forma Resh era "ye" y la respuesta era "ye significa
            // marcador de pregunta" en vez de responder lo preguntado.
            let roles = Set(RolMental.allCases.map(\.rawValue))
            for token in toksRec where esFormaResh(token) && !LenguaResh.esParticula(token) && !roles.contains(token) {
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

    // ---------- Comprensión del Resh ----------
    // Antes la mente solo transducía: las palabras de un mensaje quedaban
    // acopladas en fila, y "kash significa árbol" dicho por otra mente no
    // enseñaba nada (el acople en fila no llega a equivalencia). Ahora la
    // frase se analiza con la gramática del idioma y cada forma hace algo:
    //   define  -> aprende la palabra (el éter por fin se auto-enseña)
    //   ko      -> X y Y se acercan en fase (acuerdo)
    //   kep     -> X y Y quedan ligados (analogía)
    //   ye X va Y? -> pregunta que esta mente puede contestar
    //   ra / narración -> entra a la memoria episódica
    //   español -> se enlaza con su raíz Resh (entiende al humano)
    /// - Returns: true si era una pregunta por una relación X–Y.
    private func comprender(_ mensaje: String, de emisor: String) -> Bool {
        let a = LenguaResh.analizar(mensaje)
        let deOtra = emisor != rol.rawValue
        if deOtra, let d = a.definicion { aprenderDelEter(resh: d.resh, español: d.espanol) }
        if deOtra { enlazarEspañol(a.raices) }
        let raices = a.raices.filter { esFormaResh($0) && !LenguaResh.esParticula($0) }
        let ids = raices.compactMap { indiceEtiqueta[$0] }
        if a.tiempo == .pasado || a.modo == .narra {
            for r in raices.prefix(3) { registrarEpisodio(r) }
        }
        guard ids.count >= 2 else { return false }
        switch a.modo {
        case .acuerdo:
            reforzarAcople(ids[0], ids[1], peso: 0.6, theta: 0, tope: 0.8)
            reforzarAcople(ids[1], ids[0], peso: 0.6, theta: 0, tope: 0.8)
        case .analogia:
            reforzarAcople(ids[0], ids[1], peso: 0.5, theta: 0, tope: 0.8)
            reforzarAcople(ids[1], ids[0], peso: 0.5, theta: 0, tope: 0.8)
            pushContexto("entendí: \(raices[0]) kep \(raices[1])")
        case .pregunta where deOtra && mensaje.hasPrefix("ye "):
            preguntaRelacion = (raices[0], raices[1], emisor)
            return true
        default:
            break
        }
        return false
    }

    /// Semión de una etiqueta, creándolo en silencio si no existe.
    private func asegurarSemion(_ etiqueta: String) -> UUID {
        if let id = indiceEtiqueta[etiqueta], semions[id] != nil { return id }
        let s = Semion(label: etiqueta, signature: signatureForText(etiqueta))
        insert(s)
        return s.id
    }

    /// "X significa Y" oído en el éter: se aprende como equivalencia,
    /// algo más débil que lo que enseña el humano (fuerza 0.6, no 1.0).
    /// Lo innato no se reescribe: si el léxico ya dice otra cosa, gana.
    private func aprenderDelEter(resh: String, español: String) {
        guard resh != español, español != "significa",
              !LenguaResh.esParticula(resh), !LenguaResh.esParticula(español),
              esFormaResh(resh) || LenguaResh.esFormaValida(resh) else { return }
        if let innata = LenguaResh.glosaDe(resh), innata != español { return }
        if memoriaLexica[español]?.variantes.contains(resh) == true { return }
        let a = asegurarSemion(español)
        let b = asegurarSemion(resh)
        reforzarAcople(a, b, peso: 0.9, theta: 0)
        reforzarAcople(b, a, peso: 0.9, theta: 0)
        convergerFirmas(a, b, tasa: 0.15)
        var e = memoriaLexica[español] ?? EntradaMemoria(variantes: [], fuerza: 0, usos: 0, ultimaVez: Date())
        e.variantes.append(resh)
        e.fuerza = max(e.fuerza, 0.6); e.usos += 1; e.ultimaVez = Date()
        memoriaLexica[español] = e
        reshConocido.insert(resh)
        preguntasPendientes.removeAll { $0 == resh }
        pushContexto("aprendí del éter: \(resh) = \(español)")
        appendLog("aprendí del éter: \(resh) = \(español)")
    }

    /// Palabras en español (del humano, o "qué significa" de otra mente) se
    /// enlazan con su raíz Resh y la raíz entra al foco: así la mente
    /// responde SOBRE lo que se le dijo, y en su idioma.
    private func enlazarEspañol(_ tokens: [String]) {
        var n = 0
        for t in tokens where n < 6 {
            // (> 3 letras: "el", "la", "de" no son tema de conversación.)
            guard t.count > 3, LenguaResh.esEspanol(t), !reshConocido.contains(t),
                  t != "significa", t != "qué", t != "que" else { continue }
            guard let resh = memoriaLexica[t]?.variantes.first ?? LenguaResh.traducir(t),
                  !LenguaResh.esParticula(resh) else { continue }
            let a = asegurarSemion(t)
            let b = asegurarSemion(resh)
            reforzarAcople(a, b, peso: 0.9, theta: 0)
            reforzarAcople(b, a, peso: 0.9, theta: 0)
            atender(b)
            n += 1
        }
    }

    // ¿Es una forma Resh? Lo que esta mente ya conoce (innato o enseñado,
    // aunque lleve 'h' como "kash" o "sha"), o lo que respeta la fonotáctica
    // del idioma (CVC, CVCC, partículas, compuestos) y no es español.
    // Antes bastaba con usar letras del alfabeto: "alinea", "mapea" o "sol"
    // pasaban por Resh y se decían como raíces o se preguntaban.
    private func esFormaResh(_ t: String) -> Bool {
        let s = t.lowercased().trimmingCharacters(in: .punctuationCharacters)
        guard s.count >= 2 else { return false }
        if reshConocido.contains(s) { return true }
        return LenguaResh.esFormaValida(s) && !LenguaResh.esEspanol(s)
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
    /// Con silencioso=true no deja rastro en log/contexto/episodios (para
    /// cargas masivas como el vocabulario inicial del rol).
    func enseñar(español: String, significa: [String], silencioso: Bool = false) {
        let idsEsp = ingestText(español)
        for resh in significa {
            let idsResh = ingestText(resh)
            var e = memoriaLexica[español] ?? EntradaMemoria(variantes: [], fuerza: 0, usos: 0, ultimaVez: Date())
            if !e.variantes.contains(resh) { e.variantes.append(resh) }
            e.fuerza = 1.0; e.usos += 1; e.ultimaVez = Date()
            memoriaLexica[español] = e
            reshConocido.formUnion(significa)
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
        if !silencioso {
            pushContexto("enseñado: \(español) = \(significa.joined(separator: ", "))")
            appendLog("enseñado: \(español) = \(significa.joined(separator: ", "))")
            registrarEpisodio(español)
        }
    }

    /// Idioma innato: desde el nacer, la mente conoce el léxico COMPLETO
    /// (las 5844 formas + las 16 partículas). El léxico vive en LenguaResh
    /// (estático, compartido, costo cero por mente); reshConocido lo refleja
    /// para que nada del idioma dispare "qué significa X?".
    /// La red de semiones (cap 1200) no podría contener 5844 palabras como
    /// semiones: el idioma es base innata, la red es experiencia vivida.
    /// Lo usado o enseñado se refuerza en memoriaLexica y RESUENA (vía
    /// principal); lo demás se resuelve por la base innata, sin inventar.
    /// Enseñar una palabra ahora = dominarla (fuerza 1.0 + acople fuerte),
    /// no descubrirla.
    func conocerIdiomaCompleto() {
        guard !idiomaInnata else { return }
        idiomaInnata = true
        reshConocido.formUnion(LenguaResh.pares.map { $0.1 })
        reshConocido.formUnion(LenguaResh.particulaPorForma.keys)
        appendLog("idioma innato: \(reshConocido.count) formas")
    }

    /// Siembra memoria vivida para una palabra innata usada por primera vez:
    /// usar es empezar a recordar (la fuerza sube con el uso, como siempre).
    private func sembrarMemoria(español: String, variantes: [String]) {
        var e = memoriaLexica[español] ?? EntradaMemoria(variantes: [], fuerza: 0, usos: 0, ultimaVez: Date())
        for v in variantes where !e.variantes.contains(v) { e.variantes.append(v) }
        e.usos += 1; e.fuerza = min(1, e.fuerza + 0.15); e.ultimaVez = Date()
        memoriaLexica[español] = e
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
    /// Lo vivido resuena; lo innato (el idioma completo desde el nacer)
    /// se resuelve por la base; lo demás queda en español (no inventa).
    /// La puntuación se conserva ("¿árbol?" -> "yuz?"): antes "árbol?"
    /// no se reconocía y quedaba en español.
    func decirEnResh(_ texto: String) -> String {
        texto.split(separator: " ").map { w in
            let palabra = String(w)
            let nucleo = palabra.trimmingCharacters(in: .punctuationCharacters)
            guard !nucleo.isEmpty, let resh = traducirAresh(nucleo.lowercased()) else { return palabra }
            let fin = palabra.hasSuffix("?") ? "?" : ""
            return resh + fin
        }.joined(separator: " ")
    }

    /// Glosa para el humano: Resh -> español con lo que ESTA mente sabe
    /// (léxico innato + lo enseñado). No toca la red: es solo lectura.
    func glosar(_ frase: String) -> (texto: String, comprension: Double) {
        LenguaResh.glosar(frase) { [memoriaLexica] t in
            memoriaLexica.first(where: { $0.value.variantes.contains(t) })?.key
        }
    }

    // ---------- Traducción por reconstrucción ----------
    // Traducir NO es consultar una tabla: es excitar la palabra y leer qué
    // resuena con ella en SU vecindario (acople × amplitud × alineación de
    // fase). Lo nunca enseñado no resuena: devuelve nil en vez de inventar.
    // Degradación graciosa y generalización por resonancia: la diferencia
    // entre una tabla hash y una memoria.
    //
    // Antes se inyectaba la palabra (un semión nuevo por consulta, y se
    // consulta muchas veces por mensaje), se relajaba la red ENTERA 8
    // ciclos y se devolvía el semión más coherente de toda la red, que
    // casi nunca tenía relación con la palabra: traducciones al azar,
    // red contaminada y mucho cómputo. Ahora la lectura es local.
    private func resonanciaLocal(_ palabra: String, acepta: (String) -> Bool) -> String? {
        let clave = palabra.lowercased().trimmingCharacters(in: .punctuationCharacters)
        guard let origen = semionCon(etiqueta: clave) else { return nil }
        var puntajes: [UUID: Double] = [:]
        func alineacion(_ a: Semion, _ b: Semion, _ c: Coupling) -> Double {
            (1 + cos(b.phase - a.phase - c.theta)) / 2
        }
        // Solo cuentan los acoples de EQUIVALENCIA (enseñados, o reforzados
        // por mucho uso). Un vecino casual de frase ("significa" en "qué
        // significa X") no es una traducción de X.
        let umbralEquivalencia = 0.85
        for (j, c1) in couplings[origen.id] ?? [:] where c1.weight >= umbralEquivalencia {
            guard let sj = semions[j] else { continue }
            // La alineación modula pero no anula: una equivalencia enseñada
            // se lee aunque las fases aún no se hayan asentado.
            let p1 = c1.weight * sj.amplitude * (0.5 + 0.5 * alineacion(origen, sj, c1))
            if acepta(sj.label) { puntajes[j, default: 0] += p1 }
            // Segundo salto, atenuado: generaliza por la red (sinónimos,
            // variantes enseñadas a una palabra vecina).
            for (k, c2) in couplings[j] ?? [:] where k != origen.id && c2.weight >= umbralEquivalencia {
                guard let sk = semions[k], acepta(sk.label) else { continue }
                puntajes[k, default: 0] += 0.4 * c1.weight * c2.weight * sk.amplitude * (0.5 + 0.5 * alineacion(sj, sk, c2))
            }
        }
        guard let mejor = puntajes.max(by: { $0.value < $1.value }),
              mejor.value > 0.08, let s = semions[mejor.key] else { return nil }
        return s.label
    }

    private func esEtiquetaLexica(_ l: String) -> Bool {
        !l.contains("⊕") && !l.contains(":") && l != "veo"
    }

    private func reconstruirHaciaResh(_ palabra: String) -> String? {
        resonanciaLocal(palabra) {
            esEtiquetaLexica($0) && esFormaResh($0) && !LenguaResh.esEspanol($0)
        }
    }

    private func reconstruirHaciaEspañol(_ palabra: String) -> String? {
        // Español = lo que el léxico reconoce como español, o lo que no
        // tiene forma Resh ("luz" o "sol" usan letras Resh y son español).
        resonanciaLocal(palabra) {
            esEtiquetaLexica($0) && (LenguaResh.esEspanol($0) || !esFormaResh($0))
        }
    }

    /// Español -> Resh por reconstrucción. Usar es recordar: el éxito
    /// refuerza la traza (si no, la curva de olvido borraría palabras
    /// que la mente usa a diario).
    /// Si la resonancia no encuentra nada, cae a la base innata (el léxico
    /// estático): conozco el idioma desde el nacer, no hace falta inventar.
    func traducirAresh(_ palabra: String) -> String? {
        if let resh = reconstruirHaciaResh(palabra) {
            reforzarMemoria(español: palabra)
            return resh
        }
        // Lo vivido antes que lo innato: si la resonancia aún no se asentó,
        // la memoria privada de esta mente ya sabe la palabra enseñada.
        if let resh = memoriaLexica[palabra]?.variantes.first {
            reforzarMemoria(español: palabra)
            return resh
        }
        guard idiomaInnata, let resh = LenguaResh.traducir(palabra) else { return nil }
        sembrarMemoria(español: palabra, variantes: [resh])
        return resh
    }

    /// Resh -> español por reconstrucción (con caída a la base innata).
    func traducirAespañol(_ palabra: String) -> String? {
        if let esp = reconstruirHaciaEspañol(palabra) {
            reforzarMemoria(español: esp)
            return esp
        }
        if let esp = memoriaLexica.first(where: { $0.value.variantes.contains(palabra) })?.key {
            reforzarMemoria(español: esp)
            return esp
        }
        guard idiomaInnata, let esp = LenguaResh.glosaDe(palabra) else { return nil }
        sembrarMemoria(español: esp, variantes: [palabra])
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
    /// Repaso espaciado: cuanto más se usó una palabra, más lento se
    /// olvida (como en la memoria humana). Antes todo decaía igual
    /// (×0.999 por ciclo): una palabra enseñada se borraba en ~7 minutos
    /// aunque se hubiera usado cien veces.
    private func decaerMemoria() {
        var olvidadas: [(String, [String])] = []
        for (k, e) in memoriaLexica {
            let tasa = 0.001 / (1 + log1p(Double(e.usos)))
            let f = e.fuerza * (1 - tasa)
            if f < 0.05 {
                olvidadas.append((k, e.variantes))
            } else {
                memoriaLexica[k]?.fuerza = f
            }
        }
        for (k, v) in olvidadas {
            olvidarEstructuralmente(español: k, variantes: v)
            memoriaLexica.removeValue(forKey: k)
        }
    }

    private func olvidarEstructuralmente(español: String, variantes: [String]) {
        let blancos = Set(Self.tokenizar(español) + variantes.map { $0.lowercased() })
        let ids = Set(blancos.compactMap { indiceEtiqueta[$0] })
        // Lo innato no se des-conoce: olvidar lo vivido no borra el idioma.
        reshConocido.subtract(variantes.filter { LenguaResh.glosaDe($0) == nil })
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

    // iPad: la resonancia se procesa por rebanadas rotativas en el ciclo
    // caliente. Con 8000 semiones × 18 mentes cada 60 ms, una pasada completa
    // por tick saturaría un núcleo y iPadOS mataría la app (watchdog).
    // Por rebanadas el costo por tick queda acotado y la red entera se barre
    // en pocos ticks (la actualización asíncrona también converge en Kuramoto).
    // En deliberación se usa completa=true: ahí sí importa el asentamiento total.
    private var rebanadaResonancia = 0
    private let maxRebanada = 1200
    private func propagateResonance(completa: Bool = false) {
        // Una pasada de Kuramoto generalizado con cargas topológicas.
        // Local y sin gradientes globales: cada par ajusta su fase.
        let claves = Array(couplings.keys)
        guard !claves.isEmpty else { return }
        var idx = completa ? 0 : (rebanadaResonancia % claves.count)
        let inicio = idx
        var procesados = 0
        repeat {
            let i = claves[idx]
            if var si = semions[i] {
                var deltaPhase = 0.0
                for (j, c) in couplings[i] ?? [:] {
                    guard let sj = semions[j], sj.charge == si.charge || c.weight > 0.9 else { continue }
                    deltaPhase += c.weight * sj.amplitude * sin(sj.phase - si.phase - c.theta)
                }
                si.phase += 0.05 * deltaPhase
                si.phase.formTruncatingRemainder(dividingBy: 2 * Double.pi)
                // (Ya no se marca lastActive aquí: la propagación toca a
                // TODOS, así que "activo" dejaba de distinguir lo recién
                // oído de lo viejo, y la poda por antigüedad era ciega.)
                semions[i] = si
            }
            procesados += 1
            idx += 1
            if idx >= claves.count { idx = 0 }
        } while (completa ? idx != inicio : procesados < maxRebanada && idx != inicio)
        if !completa { rebanadaResonancia = idx }
    }

    private func spontaneousSynthesis(comoHipotesis: Bool = false) {
        // Sin tarea externa: elige dos semiones resonantes al azar y
        // crea un tercero por "fusión de fase" (inferencia, no predicción).
        // ×1500 — imaginación: como hipótesis nace débil (0.35); solo
        // sobrevive si el diálogo la retoma, o la poda se la lleva.
        // El pool de candidatas se acota a 400: con 8000 semiones, el
        // min() cuadrático de firmas en cada ciclo caliente quemaría CPU.
        let active = Array(semions.values.filter { $0.amplitude > 0.45 }.shuffled().prefix(400))
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
        // Si esa fusión ya existe, se refuerza en vez de clonarla (antes
        // la misma idea se repetía hasta llenar la red).
        if let existente = semionCon(etiqueta: etiqueta) {
            var e = existente
            e.amplitude = min(1, e.amplitude + 0.05)
            e.lastActive = Date()
            semions[e.id] = e
            reforzarAcople(a.id, e.id, peso: 0.5, theta: 0.1)
            reforzarAcople(b.id, e.id, peso: 0.5, theta: -0.1)
            return
        }
        var child = Semion(label: etiqueta, signature: childSig, charge: childCharge)
        child.phase = (a.phase + b.phase) / 2
        child.amplitude = comoHipotesis ? 0.35 : min(1, (a.amplitude + b.amplitude) / 2 + 0.1)
        insert(child)
        couple(a.id, child.id, weight: 0.5, theta: 0.1)
        couple(b.id, child.id, weight: 0.5, theta: -0.1)
        appendLog(comoHipotesis ? "hipótesis imaginada: \(child.label)" : "síntesis espontánea: \(child.label)")
    }

    private func solveTask(_ task: String) {
        // "Resolver" = inyectar el enunciado y dejar relajar 80 micro-ciclos,
        // luego leer el atractor (semión de mayor coherencia).
        ingestText(task, label: "tarea")
        for _ in 0 ..< 80 {
            decay()
            propagateResonance(completa: true)
        }
        if let winner = masCoherente(semions.values) {
            appendLog("tarea «\(task.prefix(40))» → atractor: \(winner.label)")
        }
    }

    private func consolidate() {
        if contadorCaminos.count > 500 { contadorCaminos.removeAll() }  // ×150
        if coFoco.count > 500 { coFoco.removeAll() }  // ×1500
        // Poda en dos fases. Fase 1 — selección dialógica: las fusiones
        // espontáneas ("a⊕b") que NINGUNA mente retomó en el éter se podan
        // primero (lo no reutilizado se olvida). Fase 2 — el resto por
        // coherencia (compresión homeostática, no olvido catastrófico).
        guard semions.count > maxSemions else { return }
        var sobran = semions.count - maxSemions
        let huerfanas = semions.values
            .filter { $0.label.contains("⊕") && $0.usosDialogicos == 0 }
            .sorted { $0.lastActive < $1.lastActive }
        quitar(Set(huerfanas.prefix(sobran).map(\.id)))
        sobran = semions.count - maxSemions
        if sobran > 0 {
            let sorted = semions.values.map { ($0.id, coherence(of: $0.id)) }.sorted { $0.1 < $1.1 }
            quitar(Set(sorted.prefix(sobran).map(\.0)))
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
        let idsCtx = ingestText(contextLabel)
        for _ in 0 ..< 16 { decay(); propagateResonance() }
        let previo = masCoherente(semions.values)
        // 2. Ingesta de la corrección como eco
        let idsEco = ingestText(correction, label: "eco:\(contextLabel)")
        // Los ids salen de la ingesta: antes se buscaba la etiqueta exacta
        // del contexto (fallaba con mayúsculas o varias palabras) y el eco
        // "más reciente" de CUALQUIER contexto.
        guard let cid = idsCtx.last, let eid = idsEco.last,
              let c = semions[cid], let e = semions[eid] else { return }
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
    /// Con suavizado de Laplace: con poca historia no salta a 0 o a 1
    /// por un solo voto (1 acierto de 1 ya no es "precisión perfecta").
    func precision() -> Double {
        Double(aciertos + 1) / Double(intentos + 2)
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
    private func saltoTransitivo(desde id: UUID, visitados: Set<UUID> = []) -> Semion? {
        guard let cs = couplings[id], !cs.isEmpty else { return nil }
        var mejor: Semion? = nil
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
        let ancla: UUID? = scored.first?.id ?? masCoherente(semions.values)?.id
        if let a = ancla { reforzarInferenciaTransitiva(desde: a) }
        // ×1500 — imaginación generativa: el replay también inventa (1 de
        // cada 2). La hipótesis nace débil; el éter decide si vive.
        if Bool.random() { spontaneousSynthesis(comoHipotesis: true) }
    }

    /// Lo relevante-pero-incierto: amplitud alta, poco uso dialógico.
    /// Preguntar por esto es lo que más enseña a la mente.
    private func semionMasIncierto() -> Semion? {
        var mejor: Semion? = nil
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
    /// ×1500: además el modo se calibra solo — el que falla se retira —
    /// y el arousal alto empuja a preguntar (exploración).
    private func modoPara(_ r: RolMental, confianza: Double) -> ModoFrase {
        var m: ModoFrase
        if confianza < 0.5 {
            m = .pregunta
        } else {
            switch r {
            case .critico, .esceptico: m = .niega
            case .curiosidad: m = .pregunta
            case .narrativa: m = .narra
            default: m = .afirma
            }
        }
        let n = usoModo[m] ?? 0
        if n >= 5, Double(exitoModo[m] ?? 0) / Double(n) < 0.4 {
            m = .afirma  // el modo perdedor se retira a lo seguro
        }
        if arousal > 0.75, m == .afirma, Double.random(in: 0 ... 1) < 0.3 {
            m = .pregunta
        }
        return m
    }

    /// Gramática mínima Resh con las partículas del protocolo:
    /// afirma "mi ko X va Y", pregunta "ye X va Y?", niega "ne X va Y",
    /// narra "X va Y va Z" (3 eslabones).
    private func componerFrase(raices: [String], modo: ModoFrase) -> String? {
        guard !raices.isEmpty else { return nil }
        // Una sola raíz también lleva gramática (antes salía la raíz suelta).
        guard raices.count >= 2 else {
            let x = raices[0]
            switch modo {
            case .afirma: return "mi ko \(x)"
            case .pregunta: return "ye \(x)?"
            case .niega: return "ne \(x)"
            case .narra: return "\(x) ra"
            }
        }
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
    private func fraseCompuesta(desde s: Semion, modo: ModoFrase) -> (texto: String, tipo: TipoMensaje, nucleo: String)? {
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
        // ×1500 — chunking: el trío que co-ocurre 4 veces en el foco se
        // vuelve un concepto (reusa cristalizar: la memoria de trabajo
        // alimenta a la de largo plazo).
        if foco.count >= 3 {
            let ids3 = Array(foco.suffix(3))
            let labs = ids3.compactMap { semions[$0]?.label }.sorted()
            if labs.count == 3 {
                let clave = labs.joined(separator: "+")
                let n = (coFoco[clave] ?? 0) + 1
                coFoco[clave] = n
                if n == 4 { cristalizar(camino: ids3) }
            }
        }
    }

    /// Contenido del foco, lo más reciente primero.
    private func contenidoFoco() -> [Semion] {
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
        var c = Semion(label: etiqueta, signature: sig, charge: 1)
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
    private func analogiaEstructural(para id: UUID) -> (origen: Semion, destino: Semion)? {
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
        var mejor: Semion? = nil
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

    // ---------- Utilidades ----------
    private func insert(_ s: Semion) {
        // El cap se cumple AQUÍ, no solo en consolidate(): la red nunca
        // crece sin cota entre podas (la síntesis espontánea inserta en
        // cada ciclo caliente). Al expulsar, el olvido es estructural:
        // los vecinos debilitan su acoplamiento con la traza que se va.
        if semions[s.id] == nil, semions.count >= maxSemions {
            expulsarMasDebiles()
        }
        semions[s.id] = s
        indiceEtiqueta[s.label] = s.id
        if couplings[s.id] == nil { couplings[s.id] = [:] }
    }

    /// Expulsa, EN LOTE, a los semiones más incoherentes que no sean
    /// objetivo ni conocimiento enseñado (amplitud alta). Antes se expulsaba
    /// de a uno, recorriendo toda la red en CADA inserción con la red llena
    /// (la mayor parte del tiempo): ahora el costo se reparte entre ~24.
    private func expulsarMasDebiles() {
        let lote = max(1, maxSemions / 50)
        var candidatos: [(id: UUID, c: Double)] = []
        candidatos.reserveCapacity(semions.count)
        let protegidos = Set(objetivoIds)
        // Lo recién oído (último segundo) aún no tuvo tiempo de acoplarse:
        // su coherencia es ~0 y sería la primera víctima. Se protege salvo
        // que no haya otra cosa que expulsar.
        let reciente = Date().addingTimeInterval(-1)
        for s in semions.values where s.amplitude < 0.85 && !protegidos.contains(s.id) && s.lastActive < reciente {
            candidatos.append((s.id, coherence(of: s.id)))
        }
        if candidatos.isEmpty {
            for s in semions.values where s.amplitude < 0.85 && !protegidos.contains(s.id) {
                candidatos.append((s.id, coherence(of: s.id)))
            }
        }
        candidatos.sort { $0.c < $1.c }
        quitar(Set(candidatos.prefix(lote).map(\.id)))
    }

    /// Borra semiones de verdad: del mapa, del índice y de los acoples de
    /// sus vecinos. Antes los vecinos conservaban acoples hacia semiones
    /// ya borrados ("fantasmas") que se acumulaban sin límite.
    private func quitar(_ ids: Set<UUID>) {
        guard !ids.isEmpty else { return }
        for id in ids {
            if let s = semions.removeValue(forKey: id), indiceEtiqueta[s.label] == id {
                indiceEtiqueta.removeValue(forKey: s.label)
            }
            couplings.removeValue(forKey: id)
        }
        for a in Array(couplings.keys) {
            guard let inner = couplings[a], inner.keys.contains(where: ids.contains) else { continue }
            couplings[a] = inner.filter { !ids.contains($0.key) }
        }
        foco.removeAll(where: ids.contains)
    }

    private func couple(_ a: UUID, _ b: UUID, weight: Double, theta: Double) {
        // Solo entre semiones vivos: si uno fue expulsado en medio de una
        // operación, no se crea un acople huérfano.
        guard semions[a] != nil, semions[b] != nil else { return }
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

// ============================================================
//  ARRANQUE EN MODO APP (Swift Playgrounds App)
// ------------------------------------------------------------
//  Pega el contenido de "MenteView.swift" (adjunto) en tu
//  ContentView.swift. La vista arranca la cognición eterna en
//  segundo plano con Task.detached y te da chat + entrenador.
//  En modo App NO se usa PlaygroundSupport ni
//  needsIndefiniteExecution: la app vive mientras esté abierta.
// ============================================================
