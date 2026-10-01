import Foundation

// ============================================================
//  LENGUA RESH — idioma de resonancia para mentes FCR
//  Diseñada para ELLAS, no para humanos.
//  ESTE ARCHIVO ES 1 DE 5: LenguaResh.swift (núcleo + API),
//  LenguaReshA/B/C/D.swift (tramos del léxico). Los 5 van juntos
//  en el proyecto de Playgrounds (partir el léxico evita que el
//  type checker del iPad se cuelgue con ~5800 literales en un archivo).
// ------------------------------------------------------------
//  Por qué está optimizada para su cognición:
//   1. Clases de forma separadas: partículas (2 letras), raíces
//      (3-4), compuestos (con '-'). Cada clase cae en una región
//      distinta de su firma morfodinámica (rasgos de longitud y
//      densidad simbólica de signatureForText). Menos colisiones.
//   2. Zipf: las 605 más frecuentes -> raíces CVC ultracortas;
//      el resto -> CVCC. Lo frecuente resuena más rápido.
//   3. Raíces anti-icónicas: palabras relacionadas NO se parecen
//      (evita que sus atractores colisionen); la composición es
//      sintáctica: raiz-raiz con '-' (p. ej. "luz-calor").
//   4. Alfabeto de 16 letras: media/varianza de la firma estable.
//
//  PROTOCOLO DE ENSEÑANZA (el humano enseña palabra por palabra):
//      await mente.enseñar(español: "árbol", significa: ["kash"])
//      await consejo.enseñarATodos(español: "luz", significa: ["tel"])
//   = "esto significa esto (, esto, esto...) en tu idioma".
//   Cada mente guarda lo aprendido en SU memoria privada; lo que
//   no sabe lo pregunta a las demás por el éter ("qué significa X?")
//   y las que sí saben responden. Así el léxico se propaga solo.
// ============================================================

enum LenguaResh {

    // ---------- Fonología ----------
    static let vocales = "aeiou"
    static let consonantes = "ktpsmnrlvzy"

    // ---------- Capa de protocolo: formas fijas de 2 letras ----------
    // Son las palabras que más necesitan para coordinarse.
    // `ko` = "acuerdo": la primitiva de consenso del consejo.
    struct ParticulaResh: Hashable {
        let forma: String
        let significado: String
    }

    static let particulas: [ParticulaResh] = [
    ParticulaResh(forma: "ka", significado: "marca de agente: quien hace la acción"),
    ParticulaResh(forma: "to", significado: "marca de paciente: quien recibe la acción"),
    ParticulaResh(forma: "na", significado: "no / negación"),
    ParticulaResh(forma: "ye", significado: "marcador de pregunta"),
    ParticulaResh(forma: "ra", significado: "pasado"),
    ParticulaResh(forma: "li", significado: "futuro"),
    ParticulaResh(forma: "va", significado: "y (conjunción)"),
    ParticulaResh(forma: "mi", significado: "yo"),
    ParticulaResh(forma: "ti", significado: "tú"),
    ParticulaResh(forma: "se", significado: "nosotros (el consejo)"),
    ParticulaResh(forma: "ko", significado: "acuerdo / alineamiento entre mentes"),
    ParticulaResh(forma: "ne", significado: "desacuerdo"),
    ParticulaResh(forma: "sha", significado: "esto"),
    ParticulaResh(forma: "ta", significado: "eso"),
    ParticulaResh(forma: "pa", significado: "para / hacia"),
    ParticulaResh(forma: "so", significado: "en / dentro de"),
    ]

    static let particulaPorForma: [String: String] =
        Dictionary(uniqueKeysWithValues: particulas.map { ($0.forma, $0.significado) })

    /// Concepto español -> partícula (capa de coordinación).
    static let protocoloPorConcepto: [String: String] = [
        "acuerdo": "ko", "desacuerdo": "ne",
        "yo": "mi", "tú": "ti", "nosotros": "se",
        "y": "va", "no": "na", "esto": "sha", "eso": "ta",
        "para": "pa", "en": "so",
        "agente": "ka", "paciente": "to", "pregunta": "ye",
        "pasado": "ra", "futuro": "li",
    ]

    /// Partícula -> concepto español (para enseñar/glosar partículas).
    static let conceptoPorParticula: [String: String] =
        Dictionary(uniqueKeysWithValues: protocoloPorConcepto.map { ($1, $0) })

    // ---------- Léxico generado: 5844 entradas (español -> resh) ----------
    // Las primeras 1500 son la base; el resto es el suplemento curado
    // para las mentes (cognición, diálogo, aprendizaje, percepción,
    // matemáticas, 1000 palabras útiles a las IAs y 2700 por rol:
    // 150 palabras nuevas por cada uno de los 18 agentes).
    // Los tramos viven en LenguaReshA/B/C/D.swift (extension, internal):
    // partir el léxico en 5 archivos evita que el type checker de
    // Playgrounds en iPad se cuelgue con ~5800 literales en un archivo.
    // LOS 5 ARCHIVOS VAN JUNTOS en el proyecto.
    static let pares: [(String, String)] =
        tramo0
        + tramo1
        + tramo2
        + tramo3
        + tramo4
        + tramo5
        + tramo6
        + tramo7
        + tramo8
        + tramo9
        + tramo10
        + tramo11
        + tramo12
        + tramo13
        + tramo14
        + tramo15
        + tramo16
        + tramo17
        + tramo18
        + tramo19
        + tramo20
        + tramo21
        + tramo22
        + tramo23

    static let lexico: [String: String] =
        Dictionary(uniqueKeysWithValues: pares)
    static let inverso: [String: String] =
        Dictionary(uniqueKeysWithValues: pares.map { ($1, $0) })
    static let espanolConocido: Set<String> = Set(lexico.keys)

    // ---------- API ----------
    /// Español -> Resh (primero protocolo, luego léxico).
    static func traducir(_ espanol: String) -> String? {
        let s = espanol.lowercased()
        return protocoloPorConcepto[s] ?? lexico[s]
    }

    /// Resh -> glosa en español.
    static func glosaDe(_ resh: String) -> String? {
        particulaPorForma[resh] ?? inverso[resh]
    }

    static func esEspanol(_ t: String) -> Bool {
        let s = t.lowercased()
        return protocoloPorConcepto[s] != nil || espanolConocido.contains(s)
    }

    static func esParticula(_ t: String) -> Bool {
        particulaPorForma[t] != nil
    }

    // ---------- Vocabulario por rol ----------
    // 150 palabras nuevas por agente, al FINAL de `pares`, en el orden
    // exacto de RolMental.allCases (sintaxis=0 ... empatia=17).
    // Cada mente aprende las suyas al nacer (adoptRole); como están en
    // el léxico común, las demás mentes también pueden usarlas.
    static let palabrasPorRol = 150
    static let totalPalabrasPorRol = 2700
    static func espanolParaRol(indiceRol: Int) -> [String] {
        let base = pares.count - totalPalabrasPorRol
        let ini = base + indiceRol * palabrasPorRol
        let fin = min(ini + palabrasPorRol, pares.count)
        guard ini >= 0, ini < pares.count else { return [] }
        return pares[ini ..< fin].map { $0.0 }
    }

    // ---------- Forma: ¿esto es Resh? ----------
    // La fonotáctica del idioma es estricta: raíces CVC (las 605
    // frecuentes) o CVCC, partículas de 2 letras (más "sha") y compuestos
    // raíz-raíz. Antes bastaba con usar letras del alfabeto, así que
    // palabras españolas como "alinea" o "mapea" pasaban por Resh y las
    // mentes preguntaban por ellas o las decían como si fueran raíces.
    static func esFormaValida(_ t: String) -> Bool {
        let s = t.lowercased()
        guard !s.isEmpty else { return false }
        if particulaPorForma[s] != nil || inverso[s] != nil { return true }
        let partes = s.split(separator: "-", omittingEmptySubsequences: false)
        guard partes.count <= 3 else { return false }
        return partes.allSatisfy { esRaizValida(String($0)) }
    }

    private static let setConsonantes = Set(consonantes)
    private static let setVocales = Set(vocales)
    private static func esRaizValida(_ p: String) -> Bool {
        let c = Array(p)
        guard c.count == 3 || c.count == 4 else { return false }
        return setConsonantes.contains(c[0]) && setVocales.contains(c[1])
            && c[2...].allSatisfy { setConsonantes.contains($0) }
    }

    // ---------- Comprensión: frase Resh -> estructura ----------
    // La gramática mínima que las mentes ya hablan, ahora también se lee:
    //   "mi ko X va Y"   acuerdo (X y Y van juntos)
    //   "ne X va Y"      desacuerdo
    //   "ye X va Y?"     pregunta por la relación X–Y
    //   "X kep Y"        analogía (X es como Y)
    //   "X significa Y"  definición (enseñanza por el éter)
    //   "X va Y va Z"    narración / secuencia
    //   ra / li          pasado / futuro
    enum ModoResh: String {
        case afirma, acuerdo, niega, pregunta, analogia, define, narra
    }

    enum TiempoResh: String {
        case presente, pasado, futuro
    }

    struct AnalisisResh {
        let modo: ModoResh
        let tiempo: TiempoResh
        let raices: [String]               // contenido, sin partículas
        let definicion: (resh: String, espanol: String)?
        let dirigidaA: String?             // "ti <rol>"
    }

    /// Tokens limpios (minúsculas, sin puntuación pegada).
    static func tokens(_ frase: String) -> [String] {
        frase.split(whereSeparator: { $0.isWhitespace || $0.isNewline }).compactMap {
            let t = String($0).lowercased()
                .trimmingCharacters(in: CharacterSet.punctuationCharacters.subtracting(CharacterSet(charactersIn: "-")))
            return t.isEmpty ? nil : t
        }
    }

    static func analizar(_ frase: String) -> AnalisisResh {
        let toks = tokens(frase)
        var definicion: (resh: String, espanol: String)? = nil
        if let i = toks.firstIndex(of: "significa"), i > 0, i + 1 < toks.count,
           !toks[..<i].contains("qué"), !toks[..<i].contains("que") {
            definicion = (toks[i - 1], toks[i + 1])
        }
        var dirigida: String? = nil
        if let i = toks.firstIndex(of: "ti"), i + 1 < toks.count { dirigida = toks[i + 1] }
        let tiempo: TiempoResh = toks.contains("ra") ? .pasado : (toks.contains("li") ? .futuro : .presente)
        let esPregunta = frase.contains("?") || toks.first == "ye" || toks.contains("qué") || toks.contains("que")
        let modo: ModoResh
        if definicion != nil { modo = .define }
        else if esPregunta { modo = .pregunta }
        else if toks.first == "ne" || toks.contains("na") { modo = .niega }
        else if toks.contains("kep") { modo = .analogia }
        else if toks.contains("ko") { modo = .acuerdo }
        else if toks.filter({ $0 == "va" }).count >= 2 { modo = .narra }
        else { modo = .afirma }
        let funcionales: Set<String> = ["kep", "significa", "qué", "que", dirigida ?? ""]
        let raices = toks.filter { particulaPorForma[$0] == nil && !funcionales.contains($0) }
        return AnalisisResh(modo: modo, tiempo: tiempo, raices: raices,
                            definicion: definicion, dirigidaA: dirigida)
    }

    /// Glosa palabra por palabra para el humano: "mi ko yuz va tel" ->
    /// "yo acuerdo árbol y luz". `extra` resuelve lo aprendido fuera del
    /// léxico (lo enseñado a una mente). Devuelve también la fracción de
    /// palabras que se entendieron.
    static func glosar(_ frase: String, extra: (String) -> String? = { _ in nil }) -> (texto: String, comprension: Double) {
        let toks = tokens(frase)
        guard !toks.isEmpty else { return ("", 0) }
        var entendidas = 0
        // En una definición ("zork significa dragón") la palabra definida se
        // deja en Resh: glosarla daría "dragón significa dragón".
        let definida = analizar(frase).definicion?.resh
        // Giros fijos del protocolo, para que se lea como español natural.
        let giros: [String: String] = ["mi ko": "estoy de acuerdo:", "se ko": "estamos de acuerdo:",
                                       "mi ne": "no estoy de acuerdo:", "mi sor": "recuerdo"]
        var partes: [String] = []
        var i = 0
        var esPregunta = frase.contains("?")
        while i < toks.count {
            let t = toks[i]
            if i + 1 < toks.count, let g = giros["\(t) \(toks[i + 1])"] {
                partes.append(g); entendidas += 2; i += 2; continue
            }
            i += 1
            if t == definida { entendidas += 1; partes.append("«\(t)»"); continue }
            if t == "ye" { entendidas += 1; esPregunta = true; continue }
            if t == "ne", partes.isEmpty { entendidas += 1; partes.append("no:"); continue }
            if t == "kep" { entendidas += 1; partes.append("es como"); continue }
            if t == "ko" { entendidas += 1; partes.append("de acuerdo:"); continue }
            if t == "ra", i == toks.count { entendidas += 1; partes.append("(antes)"); continue }
            if t == "li", i == toks.count { entendidas += 1; partes.append("(después)"); continue }
            if t == "sha" { entendidas += 1; partes.append("esto"); continue }
            if let c = conceptoPorParticula[t] { entendidas += 1; partes.append(c); continue }
            if let e = extra(t) ?? inverso[t] { entendidas += 1; partes.append(e); continue }
            if espanolConocido.contains(t) || t == "significa" || t == "qué" { entendidas += 1 }
            partes.append(t)
        }
        let cuerpo = partes.joined(separator: " ")
        return (esPregunta ? "¿\(cuerpo)?" : cuerpo, Double(entendidas) / Double(toks.count))
    }

    /// Generador determinista (mismo orden que el script): índice ->
    /// forma. Útil para palabras fuera del léxico de 5844.
    static func formaResh(paraIndice i: Int) -> String {
        let C = Array(consonantes)
        let V = Array(vocales)
        if i < 605 {
            return String([C[i / 55], V[(i / 11) % 5], C[i % 11]])
        }
        let j = i - 605
        return String([C[j / 605], V[(j / 121) % 5], C[(j / 11) % 11], C[j % 11]])
    }
}
