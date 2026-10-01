import Foundation

// ============================================================
// MARK: - adivina: un juego con 48 reglas lógicas de verdad
// ============================================================
// No es una lista de preguntas al azar ni un guion fijo: hay 8
// animales posibles y 6 preguntas de sí/no. Cada respuesta se
// compara contra el rasgo esperado de los 8 animales a la vez —
// eso son 8 reglas por pregunta, 48 en total — y el que más
// aciertos acumula al final del todo es el que se adivina. Todo
// el cálculo pasa de verdad aquí mismo, sin nada oculto: al final
// se enseña la tabla completa de puntos, así que puedes comprobar
// que las 48 reglas se aplicaron.
//
// adivina        — empieza una partida nueva (o la reanuda)
// adivina si|no  — responde la pregunta actual
// adivina reset  — la cancela

/// Un animal candidato y sus 6 rasgos, en el mismo orden que
/// Shell.preguntasAdivina.
struct RasgoAnimal {
    let nombre: String
    let emoji: String
    let rasgos: [Bool]
}

/// Estado de una partida en marcha. Vive en Shell.adivinaEstado
/// mientras dura — es una clase para poder mutarla en sitio.
final class AdivinaState {
    var pregunta = 0
    var puntos: [String: Int]
    init(nombres: [String]) {
        puntos = Dictionary(uniqueKeysWithValues: nombres.map { ($0, 0) })
    }
}

extension Shell {

    static let preguntasAdivina = [
        "¿vuela?",
        "¿vive en el agua o nada muy bien?",
        "¿tiene plumas?",
        "¿tiene pelo o pelaje?",
        "¿pesa más de 500 kilos?",
        "¿es carnívoro?"
    ]

    // 8 animales × 6 rasgos = 48 reglas lógicas.
    static let animalesAdivina: [RasgoAnimal] = [
        RasgoAnimal(nombre: "perro",      emoji: "🐕", rasgos: [false, false, false, true,  false, true]),
        RasgoAnimal(nombre: "delfín",     emoji: "🐬", rasgos: [false, true,  false, false, false, true]),
        RasgoAnimal(nombre: "águila",     emoji: "🦅", rasgos: [true,  false, true,  false, false, true]),
        RasgoAnimal(nombre: "serpiente",  emoji: "🐍", rasgos: [false, false, false, false, false, true]),
        RasgoAnimal(nombre: "elefante",   emoji: "🐘", rasgos: [false, false, false, false, true,  false]),
        RasgoAnimal(nombre: "pingüino",   emoji: "🐧", rasgos: [false, true,  true,  false, false, true]),
        RasgoAnimal(nombre: "león",       emoji: "🦁", rasgos: [false, false, false, true,  true,  true]),
        RasgoAnimal(nombre: "murciélago", emoji: "🦇", rasgos: [true,  false, false, true,  false, false])
    ]

    private static let siValidos: Set<String> = ["si", "sí", "s", "y", "yes", "true", "1"]
    private static let noValidos: Set<String> = ["no", "n", "false", "0"]

    static func logicGame() -> [String: Spec] {
        var c: [String: Spec] = [:]

        c["adivina"] = Spec(help: "adivina [si|no|reset] — juego de adivinar un animal con 48 reglas lógicas (8 animales × 6 preguntas)") { ctx in
            let sh = ctx.sh
            let arg = ctx.args.first?.lowercased()

            if arg == "reset" {
                let había = sh.adivinaEstado != nil
                sh.adivinaEstado = nil
                return había ? "partida cancelada\n" : "no había ninguna partida en marcha\n"
            }

            // sin partida en marcha: empieza una (se ignora cualquier otro argumento)
            guard let estado = sh.adivinaEstado else {
                sh.adivinaEstado = AdivinaState(nombres: Shell.animalesAdivina.map { $0.nombre })
                return """
                🤔 estoy pensando en un animal — 8 posibles, 6 preguntas de sí o no.
                Cada respuesta se compara contra los 8 animales a la vez: son 48 reglas
                lógicas reales aplicándose ahí detrás, no una lista al azar.
                Responde con: adivina si   /   adivina no

                pregunta 1/\(Shell.preguntasAdivina.count): \(Shell.preguntasAdivina[0])

                """
            }

            let esSi = arg.map { Shell.siValidos.contains($0) } ?? false
            let esNo = arg.map { Shell.noValidos.contains($0) } ?? false
            guard esSi || esNo else {
                return "responde con: adivina si   /   adivina no\n\n" +
                       "pregunta \(estado.pregunta + 1)/\(Shell.preguntasAdivina.count): \(Shell.preguntasAdivina[estado.pregunta])\n"
            }

            // las 8 reglas de esta pregunta: una comparación por animal
            for animal in Shell.animalesAdivina where animal.rasgos[estado.pregunta] == esSi {
                estado.puntos[animal.nombre, default: 0] += 1
            }
            estado.pregunta += 1

            guard estado.pregunta < Shell.preguntasAdivina.count else {
                let ranking = estado.puntos.sorted {
                    $0.value != $1.value ? $0.value > $1.value : $0.key < $1.key
                }
                let mejor = ranking.first?.value ?? 0
                let ganadores = ranking.filter { $0.value == mejor }.map { $0.key }
                sh.adivinaEstado = nil

                var out = "se aplicaron las 48 reglas — esta es la tabla final:\n\n"
                for (nombre, puntos) in ranking {
                    let emoji = Shell.animalesAdivina.first(where: { $0.nombre == nombre })?.emoji ?? "•"
                    out += "  \(emoji) \(nombre.padding(toLength: 11, withPad: " ", startingAt: 0)) \(puntos)/6\n"
                }
                out += "\n"
                if ganadores.count == 1 {
                    out += "🎉 ¡es un \(ganadores[0])!\n"
                } else {
                    out += "🎉 empate entre \(ganadores.joined(separator: " y "))" +
                           " — con estas 6 respuestas las 48 reglas no alcanzan para desempatar.\n"
                }
                out += "\nescribe 'adivina' para jugar otra vez.\n"
                return out
            }

            return "pregunta \(estado.pregunta + 1)/\(Shell.preguntasAdivina.count): \(Shell.preguntasAdivina[estado.pregunta])\n"
        }
        c["guess"] = c["adivina"]

        return c
    }
}
