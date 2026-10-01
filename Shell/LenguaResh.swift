import Foundation

// ============================================================
//  LENGUA RESH — idioma de resonancia para mentes FCR
//  Diseñada para ELLAS, no para humanos.
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

    // ---------- Léxico generado: 3144 entradas (español -> resh) ----------
    // Las primeras 1500 son la base; el resto es el suplemento curado
    // para las mentes (cognición, diálogo, aprendizaje, percepción,
    // matemáticas y 1000 palabras útiles a las IAs).
    // Dividido en tramos: un literal gigante puede hacer que el
    // compilador falle sin mensaje claro. La suma es el mismo arreglo,
    // en el mismo orden.
    // Los tramos viven en LenguaResh_Lexico1…4.swift. Se juntan con un
    // bucle y no con una cadena de '+': trece sumas seguidas de arrays
    // enormes son justo lo que hace que el compilador del iPad se rinda.
    static let pares: [(String, String)] = {
        let tramos: [[(String, String)]] = [
            tramo0, tramo1, tramo2, tramo3, tramo4, tramo5, tramo6, tramo7, tramo8, tramo9, tramo10, tramo11, tramo12
        ]
        var todo: [(String, String)] = []
        for t in tramos { todo.append(contentsOf: t) }
        return todo
    }()

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

    /// Generador determinista (mismo orden que el script): índice ->
    /// forma. Útil para palabras fuera del léxico de 3144.
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
