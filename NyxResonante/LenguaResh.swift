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
