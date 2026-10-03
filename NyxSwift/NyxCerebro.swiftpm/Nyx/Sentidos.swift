import Foundation

// Sentidos.swift — lo que las mentes ven y oyen, y cómo cada una lo percibe.
//
// Los ojos (Ojos.swift, con Vision) y los oídos (Oidos.swift, con Speech)
// convierten una foto o un sonido en una Percepcion: palabras en español.
// Aquí cada mente se fija en cosas distintas según quién es:
//   percepcion → colores y formas · curiosidad → lo que no conocía
//   memoria → lo que reconoce · empatia → las personas
//   narrativa, logica, codigo → el texto y los números · creativo → mezcla ideas

enum Sentido: String {
    case vista, oido
}

struct Percepcion {
    var sentido: Sentido
    var cosas: [String] = []        // lo que hay (objetos, escenas): "perro", "cielo"
    var colores: [String] = []      // "azul", "verde"
    var texto: String = ""          // texto leído en la foto o palabras oídas
    var personas: Int = 0           // caras en la foto
    var rasgos: [String] = []       // "clara", "oscura", "fuerte", "suave", "agudo", "grave"
    var origen = "la foto"          // "la foto", "la cámara", "el video"
    var secuencia: [String] = []    // en un video: lo que aparece primero, luego…

    /// Una frase que describe todo.
    var descripcion: String {
        if sentido == .oido {
            var o = texto.isEmpty ? "oí un sonido" : "oí: " + texto
            if !rasgos.isEmpty { o += " · " + (texto.isEmpty ? "" : "voz ") + rasgos.joined(separator: " ") }
            return o
        }
        var partes: [String] = []
        let que = "en " + origen + " hay"
        if !cosas.isEmpty { partes.append("\(que) " + cosas.prefix(5).joined(separator: " ")) }
        if !colores.isEmpty { partes.append("colores " + colores.joined(separator: " ")) }
        if personas > 0 { partes.append(personas == 1 ? "una persona" : "\(personas) personas") }
        if secuencia.count >= 2 { partes.append("primero " + secuencia.joined(separator: " luego ")) }
        if !rasgos.isEmpty { partes.append(rasgos.joined(separator: " ")) }
        if !texto.isEmpty { partes.append((origen == "el video" ? "se oye " : "dice ") + texto) }
        return partes.joined(separator: " · ")
    }
}

/// Lo que una mente notó.
struct LoQueNoto: Identifiable {
    let id: Int        // el rol
    let palabras: [String]
    let dijo: String
}

// MARK: - Colores

enum Colores {
    /// Nombre en español de un color (r, g, b de 0 a 1).
    static func nombre(r: Double, g: Double, b: Double) -> String {
        let mx = max(r, g, b)
        let mn = min(r, g, b)
        let luz = (mx + mn) / 2
        let sat = mx - mn
        if sat < 0.12 {
            if luz > 0.8 { return "blanco" }
            if luz < 0.2 { return "negro" }
            return "gris"
        }
        var h: Double
        if mx == r { h = (g - b) / sat } else if mx == g { h = 2 + (b - r) / sat } else { h = 4 + (r - g) / sat }
        h *= 60
        if h < 0 { h += 360 }
        if h < 15 || h >= 340 { return luz < 0.35 ? "marrón" : "rojo" }
        if h < 40 { return luz < 0.4 ? "marrón" : "naranja" }
        if h < 70 { return "amarillo" }
        if h < 160 { return "verde" }
        if h < 200 { return "celeste" }
        if h < 255 { return "azul" }
        if h < 290 { return "morado" }
        return "rosa"
    }
}

// MARK: - Traducción de lo que reconoce Vision (viene en inglés)

enum Etiquetas {
    static let es: [String: String] = [
        "dog": "perro", "cat": "gato", "bird": "pájaro", "horse": "caballo", "fish": "pez", "animal": "animal",
        "insect": "insecto", "butterfly": "mariposa", "cow": "vaca", "sheep": "oveja", "rabbit": "conejo",
        "people": "gente", "person": "persona", "adult": "adulto", "child": "niño", "baby": "bebé", "face": "cara",
        "hand": "mano", "selfie": "selfie", "crowd": "multitud", "portrait": "retrato",
        "sky": "cielo", "cloud": "nube", "sun": "sol", "moon": "luna", "star": "estrella", "night_sky": "cielo",
        "sunset_sunrise": "atardecer", "sunset": "atardecer", "rainbow": "arcoíris", "rain": "lluvia", "snow": "nieve",
        "tree": "árbol", "plant": "planta", "flower": "flor", "grass": "hierba", "leaf": "hoja", "forest": "bosque",
        "garden": "jardín", "mountain": "montaña", "hill": "colina", "rock": "roca", "sand": "arena",
        "beach": "playa", "sea": "mar", "ocean": "mar", "water": "agua", "water_body": "agua", "lake": "lago",
        "river": "río", "waterfall": "cascada", "wave": "ola", "shore": "orilla", "land": "tierra",
        "outdoor": "afuera", "indoor": "adentro", "nature": "naturaleza", "landscape": "paisaje",
        "building": "edificio", "house": "casa", "home": "casa", "room": "cuarto", "kitchen": "cocina",
        "structure": "estructura", "city": "ciudad", "street": "calle", "road": "camino", "bridge": "puente",
        "window": "ventana", "door": "puerta", "wall": "pared", "floor": "suelo", "ceiling": "techo",
        "furniture": "mueble", "chair": "silla", "table": "mesa", "bed": "cama", "sofa": "sofá", "lamp": "lámpara",
        "car": "coche", "vehicle": "vehículo", "bicycle": "bicicleta", "motorcycle": "moto", "bus": "autobús",
        "train": "tren", "airplane": "avión", "boat": "barco", "ship": "barco", "wheel": "rueda",
        "food": "comida", "fruit": "fruta", "apple": "manzana", "banana": "plátano", "bread": "pan",
        "drink": "bebida", "coffee": "café", "cake": "pastel", "pizza": "pizza", "vegetable": "verdura",
        "book": "libro", "paper": "papel", "document": "documento", "text": "texto", "screenshot": "captura",
        "computer": "computadora", "screen": "pantalla", "phone": "teléfono", "keyboard": "teclado",
        "toy": "juguete", "ball": "pelota", "game": "juego", "sport": "deporte", "music": "música",
        "musical_instrument": "instrumento", "guitar": "guitarra", "piano": "piano", "art": "arte",
        "painting": "pintura", "drawing": "dibujo", "light": "luz", "fire": "fuego", "shadow": "sombra",
        "clothing": "ropa", "shirt": "camisa", "shoe": "zapato", "hat": "sombrero", "glasses": "gafas",
        "machine": "máquina", "tool": "herramienta", "box": "caja", "bottle": "botella", "cup": "taza",
    ]

    static func traduce(_ id: String) -> String {
        if let t = es[id] { return t }
        let partes = id.split(separator: "_").map(String.init)
        if let ultima = partes.last, let t = es[ultima] { return t }
        return id.replacingOccurrences(of: "_", with: " ")
    }
}

// MARK: - Las mentes perciben

extension Consejo {
    /// Recuerdan un video escena por escena (con su momento), para poder
    /// hablar de cualquier parte: "¿qué había al principio?", "¿qué se oye?".
    func recuerdaEscenas(_ recuerdos: [String]) {
        for r in recuerdos {
            for m in mentes { m.ingesta(r, dialogo: false) }
            frases.pon(r)
        }
    }

    /// Las 18 perciben lo mismo, pero cada una se fija en cosas distintas.
    @discardableResult
    func percibe(_ p: Percepcion) -> [LoQueNoto] {
        let desc = p.descripcion
        if desc.isEmpty { return [] }
        frases.pon(desc)                      // lo que vieron/oyeron queda como recuerdo
        var notas: [LoQueNoto] = []
        var vieron = 0
        for m in mentes {
            let suyas = loQueNota(m, p)
            if suyas.isEmpty { continue }
            let ids = m.recibe(suyas.joined(separator: " "), de: p.sentido == .vista ? "ojos" : "oídos")
            for id in ids where !Palabras.vacia(m.s[id].et) { m.atiende(id) }
            let dijo = frasePercepcion(m, suyas, p.sentido)
            notas.append(LoQueNoto(id: m.rol, palabras: suyas, dijo: dijo))
            vieron |= 1 << m.rol
            evento(m.rol, .percibe, dijo)
        }
        // queda registrado en el buzón (SwiftData); las que ya lo percibieron
        // no lo vuelven a recibir, las que no, lo recogen cuando les toque
        comparte(7, .percepcion, desc, yaLoSaben: vieron)
        return notas
    }

    /// En qué se fija cada mente.
    private func loQueNota(_ m: Mente, _ p: Percepcion) -> [String] {
        let textoPal = Palabras.tokens(p.texto, max: 12).filter { !Palabras.vacia($0) }
        let nuevas = p.cosas.filter { m.busca($0) == nil }
        let conocidas = p.cosas.filter { m.busca($0) != nil }
        var out: [String]
        let primeras: [String] = Array(p.cosas.prefix(2))
        let unColor: [String] = Array(p.colores.prefix(1))
        let unRasgo: [String] = Array(p.rasgos.prefix(1))
        switch m.nombre {
        case "percepcion":
            out = p.colores
            out.append(contentsOf: p.cosas.prefix(3))
            out.append(contentsOf: p.rasgos)
        case "curiosidad": out = nuevas.isEmpty ? Array(p.cosas.suffix(2)) : nuevas
        case "memoria": out = conocidas.isEmpty ? primeras : conocidas
        case "empatia":
            out = p.personas > 0 ? ["persona", "cara"] : unRasgo
            out.append(contentsOf: p.cosas.prefix(1))
        case "narrativa", "logica", "codigo", "sintaxis", "semantica":
            out = textoPal.isEmpty ? primeras : Array(textoPal.prefix(6))
        case "creativo", "intuicion", "analogia":
            out = Array(p.cosas.shuffled().prefix(2))
            out.append(contentsOf: unColor)
        case "critico", "esceptico":
            out = Array(p.cosas.suffix(1))
            out.append(contentsOf: unRasgo)
        default:
            out = primeras
            out.append(contentsOf: unColor)
        }
        var vistas = Set<String>()
        return out.filter { !$0.isEmpty && vistas.insert($0).inserted }
    }

    /// Lo que dice sobre lo que notó (en su estilo).
    private func frasePercepcion(_ m: Mente, _ suyas: [String], _ s: Sentido) -> String {
        let lista = suyas.joined(separator: ", ")
        let verbo = s == .vista ? "veo" : "oigo"
        switch m.nombre {
        case "curiosidad": return "¿qué es \(suyas.first ?? "eso")? nunca había \(s == .vista ? "visto" : "oído") \(lista)"
        case "memoria": return "reconozco \(lista)"
        case "empatia": return "me fijo en \(lista)"
        case "critico", "esceptico": return "¿seguro que es \(lista)?"
        case "creativo", "intuicion", "analogia": return "\(lista)… me hace imaginar algo nuevo"
        default: return "\(verbo) \(lista)"
        }
    }
}
