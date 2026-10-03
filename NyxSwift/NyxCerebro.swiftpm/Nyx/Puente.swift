import Foundation

// Puente.swift — el consejo antiguo y el nuevo hablan entre ellos.
//
// El consejo antiguo es el original (CerebroResonante + ConsejoResonante, la
// última versión que hablaba Resh, carpeta Antiguo/), tal cual, sin terminal.
// En cada turno del puente:
//   1. el consejo antiguo dice algo (su mente con más peso, en Resh);
//   2. las mentes nuevas lo oyen: cada una entiende solo el Resh que sabe;
//      la que más entendió contesta, y las demás nuevas oyen la respuesta;
//   3. las 18 mentes antiguas oyen la respuesta del consejo nuevo.
// Si el antiguo pregunta "qué significa X?" y una mente nueva lo sabe, se lo
// enseña a las 18 antiguas; y al revés, las nuevas aprenden del antiguo.

struct LineaPuente: Identifiable {
    let id: Int
    let antiguo: Bool       // true: habló el consejo antiguo
    let quien: String
    let resh: String
    let es: String
}

final class Puente {
    let viejo = ConsejoResonante()
    private(set) var listo = false
    private var contador = 0
    private var ultimoNuevo = ""

    /// Despierta al consejo antiguo (y le devuelve su memoria si la hay).
    @MainActor
    func prepara(memoria: Data?) async {
        if listo { return }
        await viejo.especializar()
        if let d = memoria, await viejo.importar(d) > 0 {
            listo = true
            return
        }
        await infancia()
        listo = true
    }

    /// La misma infancia que el consejo nuevo: leen las frases y se les
    /// enseña el Resh de sus palabras (así aprendía el consejo original).
    @MainActor
    private func infancia() async {
        var ensenadas = Set<String>()
        for frase in Roles.infancia {
            await viejo.percibir(frase)
            for w in Palabras.tokens(frase) where !Palabras.vacia(w) && !ensenadas.contains(w) {
                guard let r = Resh.deEspanol(w) else { continue }
                ensenadas.insert(w)
                await viejo.enseñarATodos(español: w, significa: [r])
            }
        }
    }

    @MainActor
    func exporta() async -> Data? {
        return await viejo.exportar()
    }

    private func linea(_ antiguo: Bool, _ quien: String, _ resh: String, _ es: String) -> LineaPuente {
        contador += 1
        return LineaPuente(id: contador, antiguo: antiguo, quien: quien, resh: resh, es: es)
    }

    /// Un turno de conversación entre los dos consejos.
    @MainActor
    func turno(_ nuevo: Consejo) async -> [LineaPuente] {
        if !listo { await prepara(memoria: nil) }
        var out: [LineaPuente] = []
        // 1. habla el consejo antiguo
        let emitidos = await viejo.rondaDeDialogo()
        var dicho: (rol: RolMental, texto: String, peso: Double)? = nil
        for e in emitidos where dicho == nil || e.peso > dicho!.peso { dicho = e }
        guard let d = dicho else {
            // el antiguo calla: le hablamos con lo último del nuevo para despertarlo
            if !ultimoNuevo.isEmpty { await oyeElViejo(ultimoNuevo) }
            return out
        }
        out.append(linea(true, d.rol.rawValue, d.texto, Resh.traduceHumano(d.texto)))
        // si pregunta por una palabra Resh, una mente nueva que la sepa le enseña
        if let leccion = ensenaAlViejo(d.texto, nuevo) {
            await viejo.enseñarATodos(español: leccion.es, significa: [leccion.resh])
            out.append(linea(false, leccion.quien, "\(leccion.resh) ka \(leccion.es)", "\(leccion.resh) significa \(leccion.es)"))
            return out
        }
        // 2. el consejo nuevo lo oye y contesta
        guard let r = nuevo.respondeAExterno(d.texto, autor: "antiguo·" + d.rol.rawValue) else {
            return out
        }
        out.append(linea(false, nuevo.mentes[r.rol].nombre, r.resh, r.es))
        ultimoNuevo = r.resh
        // 3. las 18 antiguas oyen la respuesta
        await oyeElViejo(r.resh)
        return out
    }

    @MainActor
    private func oyeElViejo(_ texto: String) async {
        for rol in RolMental.allCases {
            await viejo.mente(rol)?.recibir(mensaje: texto, de: "consejo-nuevo", tipo: .dato)
        }
    }

    /// "qué significa kash?" / "ye kash?": si alguna mente nueva lo sabe, enseña.
    private func ensenaAlViejo(_ texto: String, _ nuevo: Consejo) -> (quien: String, resh: String, es: String)? {
        let pregunta = texto.contains("significa") || texto.hasPrefix("ye ") || texto.hasSuffix("?")
        if !pregunta { return nil }
        for t in Palabras.tokens(texto) where Resh.esForma(t) && !Resh.formasParticula.contains(t) {
            guard let es = Resh.aEspanol(t) else { continue }
            for m in nuevo.mentes {
                if let id = m.busca(es), m.s[id].sabe {
                    nuevo.comparte(m.rol, .palabra, "\(es)=\(t)")
                    return (m.nombre, t, es)
                }
            }
        }
        return nil
    }

    /// Le preguntas directamente al consejo antiguo (vota y responde).
    @MainActor
    func preguntaAlViejo(_ texto: String) async -> (veredicto: String, votos: [String]) {
        if !listo { await prepara(memoria: nil) }
        let r = await viejo.deliberar(sobre: texto)
        let votos = r.votos.prefix(6).map { "\($0.rol.rawValue): \($0.atractor)" }
        return (r.veredicto, Array(votos))
    }
}

extension Consejo {
    /// Algo que dijo el otro consejo (en Resh): cada mente entiende lo que
    /// sabe; contesta la que más entendió y las demás oyen la respuesta.
    func respondeAExterno(_ resh: String, autor: String) -> (rol: Int, resh: String, es: String)? {
        var mejor = -1
        var mejorN = 0
        var entendido = ""
        for m in mentes {
            let (e, _) = m.comprende(resh)
            let n = Palabras.tokens(e).filter { !Palabras.vacia($0) }.count
            if n > mejorN || (n == mejorN && n > 0 && Azar.f01() < 0.3) {
                mejor = m.rol
                mejorN = n
                entendido = e
            }
        }
        if mejor < 0 {
            // nadie entendió nada: una pregunta por la primera palabra desconocida
            let m = mentes[Azar.ent(numRoles)]
            guard let duda = m.reshOido.last else { return nil }
            evento(m.rol, .dice, "ye \(duda)?   «¿qué significa \(duda)?» (al consejo antiguo)")
            return (m.rol, "ye \(duda)?", "¿qué significa \(duda)?")
        }
        let r = hablaCon(mejor, entendido)
        if r.es.hasPrefix("(") { return nil }
        evento(mejor, .dice, "→ antiguo: \(r.resh)   «\(r.es)»")
        return (mejor, r.resh, r.es)
    }
}
