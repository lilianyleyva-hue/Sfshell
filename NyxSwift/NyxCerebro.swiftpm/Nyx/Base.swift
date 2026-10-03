import Foundation

// Base.swift — azar, cómo es cada una de las 18, su infancia y las
// palabras vacías. (Es la versión Swift de base.c de NyxC.)

// MARK: - Azar (xorshift: igual en todas partes)

enum Azar {
    static var estado: UInt64 = 88172645463325252

    static func semilla(_ s: UInt64) {
        estado = s == 0 ? 88172645463325252 : s
    }

    static func u32() -> UInt32 {
        var x = estado
        x ^= x << 13
        x ^= x >> 7
        x ^= x << 17
        estado = x
        return UInt32(truncatingIfNeeded: x >> 32)
    }

    static func f01() -> Float {
        let v: UInt32 = u32() >> 8
        return Float(v) * (1.0 / 16777216.0)
    }

    static func rango(_ a: Float, _ b: Float) -> Float {
        return a + (b - a) * f01()
    }

    static func ent(_ n: Int) -> Int {
        if n <= 0 { return 0 }
        return Int(u32() % UInt32(n))
    }
}

func limita(_ x: Float, _ a: Float, _ b: Float) -> Float {
    if x < a { return a }
    if x > b { return b }
    return x
}

// MARK: - Acciones que una mente puede querer hacer

enum Accion: Int, CaseIterable {
    case hablar, preguntar, imaginar, sonar, recordar, escuchar

    var nombre: String {
        switch self {
        case .hablar: return "hablar"
        case .preguntar: return "preguntar"
        case .imaginar: return "imaginar"
        case .sonar: return "soñar"
        case .recordar: return "recordar"
        case .escuchar: return "escuchar"
        }
    }
}

let numAcciones = 6
let numRoles = 18

// MARK: - Las 18

struct RolInfo {
    let nombre: String
    let ruido: Float
    let rigidez: Float
    let A0: Float
    let sabeResh: Float
    let objetivo: String
    let gusto: [Float]
}

enum Roles {
    static let todos: [RolInfo] = [
        RolInfo(nombre: "sintaxis", ruido: 0.02, rigidez: 1.2, A0: 0.65, sabeResh: 0.45, objetivo: "frase bien formada orden claro estructura", gusto: [0.5, 0.2, 0.1, 0.4, 0.3, 0.3]),
        RolInfo(nombre: "semantica", ruido: 0.05, rigidez: 0.8, A0: 0.60, sabeResh: 0.40, objetivo: "significado claro que se entienda", gusto: [0.6, 0.3, 0.2, 0.3, 0.3, 0.3]),
        RolInfo(nombre: "logica", ruido: 0.01, rigidez: 1.4, A0: 0.70, sabeResh: 0.30, objetivo: "valido sin contradiccion coherente", gusto: [0.4, 0.3, 0.1, 0.6, 0.2, 0.3]),
        RolInfo(nombre: "codigo", ruido: 0.03, rigidez: 1.0, A0: 0.65, sabeResh: 0.30, objetivo: "preciso ejecutable correcto", gusto: [0.5, 0.2, 0.2, 0.5, 0.2, 0.2]),
        RolInfo(nombre: "creativo", ruido: 0.12, rigidez: 0.4, A0: 0.50, sabeResh: 0.25, objetivo: "nuevo inesperado original posibilidad", gusto: [0.4, 0.2, 0.9, 0.2, 0.2, 0.1]),
        RolInfo(nombre: "critico", ruido: 0.02, rigidez: 1.5, A0: 0.55, sabeResh: 0.25, objetivo: "falla error objecion verifica", gusto: [0.7, 0.4, 0.1, 0.2, 0.2, 0.3]),
        RolInfo(nombre: "memoria", ruido: 0.03, rigidez: 0.5, A0: 0.80, sabeResh: 0.55, objetivo: "recuerdo relevante pasado patron", gusto: [0.3, 0.2, 0.1, 0.5, 0.9, 0.3]),
        RolInfo(nombre: "percepcion", ruido: 0.06, rigidez: 0.7, A0: 0.60, sabeResh: 0.25, objetivo: "observa describe forma color ritmo", gusto: [0.4, 0.3, 0.3, 0.2, 0.2, 0.6]),
        RolInfo(nombre: "sintesis", ruido: 0.09, rigidez: 0.5, A0: 0.55, sabeResh: 0.30, objetivo: "integra une resume concluye", gusto: [0.6, 0.2, 0.5, 0.5, 0.3, 0.2]),
        RolInfo(nombre: "intuicion", ruido: 0.14, rigidez: 0.3, A0: 0.55, sabeResh: 0.20, objetivo: "rapido corazonada destello ahora", gusto: [0.3, 0.2, 0.7, 0.2, 0.2, 0.4]),
        RolInfo(nombre: "analogia", ruido: 0.11, rigidez: 0.45, A0: 0.55, sabeResh: 0.25, objetivo: "puente entre dominios como metafora", gusto: [0.5, 0.2, 0.7, 0.3, 0.2, 0.2]),
        RolInfo(nombre: "contexto", ruido: 0.04, rigidez: 0.9, A0: 0.70, sabeResh: 0.35, objetivo: "hilo situacion aqui ahora marco", gusto: [0.4, 0.3, 0.1, 0.3, 0.5, 0.5]),
        RolInfo(nombre: "esceptico", ruido: 0.03, rigidez: 1.3, A0: 0.55, sabeResh: 0.25, objetivo: "duda pregunta cuestiona verifica", gusto: [0.4, 0.8, 0.1, 0.3, 0.2, 0.3]),
        RolInfo(nombre: "narrativa", ruido: 0.06, rigidez: 0.6, A0: 0.60, sabeResh: 0.35, objetivo: "secuencia entonces despues historia", gusto: [0.6, 0.1, 0.3, 0.2, 0.8, 0.2]),
        RolInfo(nombre: "etica", ruido: 0.01, rigidez: 1.6, A0: 0.65, sabeResh: 0.35, objetivo: "justo bien deber correcto", gusto: [0.5, 0.3, 0.1, 0.5, 0.3, 0.4]),
        RolInfo(nombre: "curiosidad", ruido: 0.10, rigidez: 0.4, A0: 0.55, sabeResh: 0.20, objetivo: "novedad explora descubre pregunta", gusto: [0.3, 0.9, 0.5, 0.2, 0.2, 0.3]),
        RolInfo(nombre: "abstraccion", ruido: 0.05, rigidez: 0.7, A0: 0.75, sabeResh: 0.30, objetivo: "esencia general eleva patron", gusto: [0.4, 0.2, 0.6, 0.6, 0.2, 0.2]),
        RolInfo(nombre: "empatia", ruido: 0.05, rigidez: 0.6, A0: 0.65, sabeResh: 0.35, objetivo: "escucha comprende alinea acompana", gusto: [0.5, 0.3, 0.2, 0.2, 0.3, 0.8]),
    ]

    /// El número de un rol por su nombre (o por su comienzo: "crea" -> creativo).
    static func indice(_ nombre: String) -> Int? {
        let n = nombre.lowercased()
        for (i, r) in todos.enumerated() where r.nombre == n { return i }
        if n.count >= 3 {
            for (i, r) in todos.enumerated() where r.nombre.hasPrefix(n) { return i }
        }
        return nil
    }

    /// Lo que leen al nacer.
    static let infancia: [String] = [
        "la mente aprende cuando escucha y pregunta",
        "una idea nueva nace cuando dos ideas se unen",
        "la luz del sol hace crecer el árbol",
        "el agua corre por el río hasta el mar",
        "cada pregunta abre un camino nuevo",
        "el error enseña más que el acierto",
        "una historia tiene un principio y un final",
        "el recuerdo guarda lo que fue importante",
        "la duda ayuda a buscar la verdad",
        "un buen código es claro y correcto",
        "la palabra justa dice mucho con poco",
        "el amigo escucha y comprende",
        "la música tiene ritmo y forma",
        "el niño juega y descubre el mundo",
        "la noche trae silencio y estrellas",
        "pensar es unir lo que parece separado",
        "el tiempo pasa y la memoria queda",
        "un patrón se repite en muchas cosas",
        "la verdad se busca con preguntas",
        "el bien y el deber guían lo justo",
        "el color del cielo cambia con la luz",
        "la ciencia observa mide y explica",
        "un problema grande se divide en partes pequeñas",
        "la imaginación ve lo que todavía no existe",
        "el lenguaje une a las mentes",
        "aprender es cambiar con lo que se vive",
        "la casa protege del frío y de la lluvia",
        "el camino largo empieza con un paso",
        "la historia del mundo está llena de cambios",
        "el sueño ordena lo que pasó en el día",
        "una buena razón convence sin gritar",
        "la naturaleza tiene árboles ríos y montañas",
        "el número cuenta y la palabra explica",
        "escuchar al otro es una forma de respeto",
        "la curiosidad empuja a explorar",
        "lo simple es más fuerte que lo complicado",
        "el fuego da calor y luz",
        "cada mente piensa de forma distinta",
        "juntas las mentes ven más lejos",
        "la esencia de algo es lo que no cambia",
        "el sol sale por la mañana y se esconde por la tarde",
        "la luna cambia de forma cada noche",
        "el árbol crece despacio y da sombra en verano",
        "el río lleva agua de la montaña al mar",
        "la lluvia cae de las nubes y riega la tierra",
        "las estrellas brillan lejos en el cielo",
        "el que escucha aprende y el que aprende enseña",
        "la paciencia ayuda a resolver los problemas difíciles",
        "el mar es grande profundo y azul",
        "la curiosidad abre puertas que el miedo cierra",
    ]
}

// MARK: - Palabras vacías y conectores

enum Palabras {
    /// Sirven para la gramática, nunca para ganar un veredicto.
    static let vacias: Set<String> = [
        "el", "la", "los", "las", "un", "una", "unos", "unas", "de", "del", "al", "a", "y", "o", "u", "e",
        "que", "en", "por", "con", "para", "se", "su", "sus", "lo", "le", "les", "es", "son", "no", "mi",
        "tu", "me", "te", "nos", "ya", "más", "mas", "muy", "pero", "si", "sí", "como", "cuando", "hay",
        "este", "esta", "esto", "ese", "esa", "eso", "fue", "ser", "está", "están", "qué", "cómo", "yo",
        "tú", "él", "ella", "todo", "toda", "sin", "sobre", "entre", "hasta", "desde", "también", "ni",
    ]

    /// Una frase no empieza con estas.
    static let conectores: Set<String> = [
        "y", "o", "u", "e", "en", "por", "con", "de", "del", "que", "a", "al", "para", "ni", "pero",
        "sin", "como", "se", "es", "son",
    ]

    /// Verbos "ligeros": van con casi todo, así que dicen poco del tema.
    static let ligeras: Set<String> = [
        "hace", "hacer", "hacen", "tiene", "tienen", "tener", "da", "dan", "dar", "va", "van", "ir",
        "puede", "pueden", "poder", "sirve", "sirven", "viene", "vienen", "dice", "pasa", "queda",
        "háblame", "hablame", "dime", "cuéntame", "cuentame", "sabes", "sabe", "explica", "explícame",
    ]

    static func vacia(_ w: String) -> Bool { vacias.contains(w) }
    static func conector(_ w: String) -> Bool { conectores.contains(w) }

    /// Palabras en minúscula, sin puntuación (¿?¡!«»… separan).
    static func tokens(_ texto: String, max: Int = 24) -> [String] {
        var out: [String] = []
        var actual = ""
        for ch in texto.lowercased() {
            let esParte: Bool = ch.isLetter || ch.isNumber || ch == "-" || ch == "_"
            if esParte {
                actual.append(ch)
            } else if !actual.isEmpty {
                out.append(String(actual.prefix(24)))
                actual = ""
                if out.count >= max { return out }
            }
        }
        if !actual.isEmpty && out.count < max { out.append(String(actual.prefix(24))) }
        return out
    }

    /// Firma formal de una palabra (sin embeddings): forma, ritmo y símbolos.
    static func firma(_ w: String) -> [Float] {
        let bytes: [UInt8] = Array(w.utf8)
        let nb = max(1, bytes.count)
        let largo = max(1, w.count)
        var suma = 0.0
        var dig = 0, voc = 0, simb = 0, acen = 0
        for c in bytes {
            suma += Double(c)
            if c >= 48 && c <= 57 { dig += 1 }
            if c == 97 || c == 101 || c == 105 || c == 111 || c == 117 { voc += 1 }
            if c == 43 || c == 45 || c == 42 || c == 47 || c == 61 || c == 60 || c == 62 || c == 94 { simb += 1 }
            if c >= 128 { acen += 1 }
        }
        let media = suma / Double(nb)
        var varianza = 0.0
        for c in bytes {
            let d = Double(c) - media
            varianza += d * d
        }
        varianza /= Double(nb)
        var sim = 0
        if bytes.count >= 2 {
            for i in 0 ..< bytes.count / 2 where bytes[i] == bytes[bytes.count - 1 - i] { sim += 1 }
        }
        let L = Float(largo)
        var f: [Float] = []
        f.append(L / 20)
        f.append(Float(media.truncatingRemainder(dividingBy: 128) / 128))
        f.append(Float(min(1.0, varianza / 4000)))
        f.append(Float(sim) / L)
        f.append(Float(dig) / L)
        f.append(Float(acen) / Float(nb))
        f.append(Float(voc) / L)
        f.append(Float(simb) / L)
        return f
    }
}

// MARK: - Resh

enum Resh {
    /// español -> resh, leído de ReshDatos una sola vez.
    static let particulas: [String: String] = lee(ReshDatos.particulas)
    static let lexico: [String: String] = lee(ReshDatos.lexico)

    private static func lee(_ texto: String) -> [String: String] {
        var d: [String: String] = [:]
        for par in texto.split(separator: ";") {
            let partes = par.split(separator: "=")
            if partes.count != 2 { continue }
            let es = partes[0].trimmingCharacters(in: .whitespacesAndNewlines)
            let re = partes[1].trimmingCharacters(in: .whitespacesAndNewlines)
            if d[es] == nil { d[es] = re }
        }
        return d
    }

    /// español -> resh (nil si no está en el léxico). Primero las partículas.
    static func deEspanol(_ es: String) -> String? {
        return particulas[es] ?? lexico[es]
    }

    static func esParticula(_ es: String) -> Bool {
        return particulas[es] != nil
    }
}
