#if canImport(SwiftUI)
import Foundation
import SwiftUI
#if canImport(SwiftData)
import SwiftData
#endif

// NyxModelo.swift — une el cerebro con la pantalla y con SwiftData.

struct MensajeChat: Identifiable {
    let id: Int
    let deHumano: Bool
    let rol: Int          // quién habló (-1: tú)
    let texto: String
    let resh: String
    let detalle: String
}

struct FilaTransferencia: Identifiable {
    let id: Int
    let de: Int
    let para: Int
    let tipo: String
    let contenido: String
    let recogieron: Int
}

struct MensajeUno: Identifiable {
    let id: Int
    let deHumano: Bool
    let es: String
    let resh: String
    let detalle: String
    let pasos: String
    var pensamiento: String = ""     // cómo lo pensó, paso a paso
}

struct LineaLogica: Identifiable {
    let id: Int
    let agente: Int
    let texto: String
}

struct Aporte: Identifiable {
    let id: Int
    let agente: String
    let texto: String
}

struct Gusto: Identifiable {
    let id: Int
    let nombre: String
    let valor: Double
    let veces: Int
}

struct FilaMente: Identifiable {
    let id: Int
    let nombre: String
    let precision: Double
    let ideas: Int
    let resh: Int
    let dichos: Int
    let gusta: String
}

@MainActor
func colorDe(_ rol: Int) -> Color {
    if rol < 0 { return .white }
    return Color(hue: Double(rol) / 18.0, saturation: 0.55, brightness: 0.95)
}

final class NyxModelo: ObservableObject {
    @Published var chat: [MensajeChat] = []
    @Published var eventos: [Evento] = []
    @Published var vivo = false
    @Published var pausa: Double = 0.6
    @Published var transferencias: [FilaTransferencia] = []
    @Published var version = 0
    @Published var aviso = ""
    @Published var notasSentidos: [LoQueNoto] = []
    @Published var ultimaVista = ""
    @Published var ultimoOido = ""
    @Published var ultimaRespuesta = ""
    @Published var ultimaCamara = ""
    @Published var ultimoVideo = ""
    @Published var lineasVideo: [String] = []
    // el consejo lógico
    var logico = ConsejoLogico(primer: true)
    @Published var lineasLogica: [LineaLogica] = []
    @Published var veredictoLogico = ""
    @Published var explicacionLogica: [String] = []
    @Published var aportesLogicos: [Aporte] = []
    private var contadorLogica = 0
    // el consejo antiguo y el puente
    let puente = Puente()
    @Published var lineasPuente: [LineaPuente] = []
    @Published var puenteOcupado = false
    @Published var respuestaAntiguo = ""
    // Nyx Uno: una sola mente con lo mejor de todos
    var uno: NyxUno!
    /// Lo que le mandaste y aún no ha leído (sin límite: se leen uno tras otro).
    enum Adjunto {
        case archivo(URL, String)
        case enlace(URL)
    }
    private var colaAdjuntos: [Adjunto] = []
    private var ultimaGuardada = Date.distantPast
    private var guardadoPendiente = false
    @Published var adjuntando = ""
    @Published var cargandoMemoria = true
    @Published var estadoMemoria = "🧠 recordando…"
    private var guardando = false
    @Published var chatUno: [MensajeUno] = []
    @Published var estadoUno = ""
    @Published var avisoUno = ""
    private var contadorUno = 0
    // los tres consejos juntos
    let asamblea = Asamblea()
    @Published var lineasAsamblea: [LineaAsamblea] = []
    @Published var asambleaViva = false
    @Published var estadoAsamblea = ""
    @Published var marcador = "✨ 0   🏛 0   ⚖️ 0"
    @Published var resumenMisiones = ""
    private var tareaAsamblea: Task<Void, Never>?
    /// Mostrar la traducción al español debajo del Resh.
    @Published var traducir = true
    private var cosasCamara: Set<String> = []
    var conSwiftData = false
    var consejo: Consejo
    var puedeOpinar = false
    private var contador = 0
    private var tarea: Task<Void, Never>?
    #if canImport(SwiftData)
    private var contenedor: ModelContainer?
    private var contexto: ModelContext?
    private var buzonSD: BuzonSwiftData?
    #endif

    init() {
        #if canImport(SwiftData)
        if let cont = try? ModelContainer(for: SaberMente.self, Transferencia.self) {
            contenedor = cont
            contexto = ModelContext(cont)
            conSwiftData = true
        }
        #endif
        // arranca al instante con un cerebro recién nacido; su memoria se carga
        // en segundo plano (Swift Playgrounds cierra la app si tarda más de 5 s)
        consejo = Consejo(infancia: true)
        aviso = "recordando…"
        uno = NyxUno(consejo: consejo, logica: logico, saber: asamblea.saber)
        conecta()
        refrescaTransferencias()
        Task { @MainActor in await self.cargaMemoria() }
    }

    static var rutaLogico: URL {
        let docs = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first
            ?? URL(fileURLWithPath: NSTemporaryDirectory())
        return docs.appendingPathComponent("nyx_logica.txt")
    }

    /// Carga lo aprendido: lee los textos (rápido) y los convierte en cerebro en
    /// segundo plano. Mientras tanto no se guarda nada (para no pisar la memoria).
    @MainActor
    func cargaMemoria() async {
        cargandoMemoria = true
        estadoMemoria = "🧠 recordando…"
        var partes: (mentes: [String], frases: String)? = nil
        var txtLogico: String? = nil
        var txtSaber: String? = nil
        var txtUno: String? = nil
        #if canImport(SwiftData)
        if let ctx = contexto {
            partes = AlmacenSwiftData.textos(de: ctx)
            txtLogico = AlmacenSwiftData.cargaLogico(de: ctx)
            txtSaber = AlmacenSwiftData.cargaTexto(rol: 400, de: ctx)
            txtUno = AlmacenSwiftData.cargaTexto(rol: 500, de: ctx)
        }
        #endif
        let leidas = partes
        let tl = txtLogico
        let ts = txtSaber
        let caja = await Task.detached(priority: .userInitiated) { () -> CajaCarga in
            let k = CajaCarga()
            if let p = leidas, let c = Consejo.desdePartes(mentes: p.mentes, frases: p.frases) {
                k.consejo = c
                k.origen = "SwiftData"
            }
            if k.consejo == nil, let c = Consejo.carga() {
                k.consejo = c
                k.origen = "copia de seguridad"
            }
            var logica = tl
            if logica == nil || logica!.isEmpty { logica = try? String(contentsOf: NyxModelo.rutaLogico, encoding: .utf8) }
            if let t = logica, !t.isEmpty { k.logico = ConsejoLogico.desde(t) }
            if let t = ts, !t.isEmpty { k.saber = SaberMisiones.desde(t) }
            return k
        }.value
        aplica(caja, txtUno: txtUno)
        cargandoMemoria = false
        let ideas = consejo.mentes.map { $0.s.count }.reduce(0, +)
        estadoMemoria = caja.consejo != nil
            ? "🧠 recordó todo (\(caja.origen)): \(ideas) ideas, \(consejo.frases.frases.count) frases, \(logico.hechos.count) hechos"
            : "🧠 memoria nueva: todavía no había nada guardado"
        refrescaUno()
        refrescaTransferencias()
        version += 1
    }

    /// Pone en marcha el cerebro que se cargó.
    @MainActor
    private func aplica(_ caja: CajaCarga, txtUno: String?) {
        if let c = caja.consejo {
            consejo = c
            aviso = "recordaron todo (\(caja.origen))"
        } else {
            aviso = "nacieron y leyeron su infancia"
        }
        if let l = caja.logico { logico = l }
        if let sm = caja.saber { asamblea.saber = sm }
        uno = NyxUno(consejo: consejo, logica: logico, saber: asamblea.saber)
        if let t = txtUno, !t.isEmpty { uno.importa(t) }
        conecta()
    }

    // MARK: guardar y cargar con un archivo tuyo

    /// Todo lo que sabe, en un solo archivo de texto (para el botón 💾).
    @MainActor
    func preparaArchivo() async -> Data? {
        if ocupado() { return nil }
        avisoUno = "💾 preparando el archivo…"
        let foto = consejo.foto()
        let txtLogico = logico.exporta()
        let txtSaber = asamblea.saber.exporta()
        let txtUno = uno.exporta()
        let datos = await Task.detached(priority: .userInitiated) { () -> Data in
            let partes = ArchivoNyx.Partes(cerebro: foto.todo(), logica: txtLogico, misiones: txtSaber, uno: txtUno)
            return Data(ArchivoNyx.junta(partes).utf8)
        }.value
        let kb = datos.count / 1024
        avisoUno = "💾 archivo listo (\(kb) KB): elige dónde guardarlo"
        return datos
    }

    /// Carga un archivo guardado con 💾 (el botón 📂): recuerda todo lo que había.
    func cargaArchivo(_ url: URL) {
        if ocupado() { return }
        guard let copia = Lector.copiaDeArchivos(url) else {
            avisoUno = "📂 no pude abrir ese archivo"
            return
        }
        cargandoMemoria = true
        estadoMemoria = "📂 cargando «\(url.lastPathComponent)»…"
        Task { @MainActor in
            let caja = await Task.detached(priority: .userInitiated) { () -> CajaCarga in
                let k = CajaCarga()
                guard let texto = try? String(contentsOf: copia, encoding: .utf8), let p = ArchivoNyx.separa(texto) else { return k }
                k.consejo = Consejo.desde(p.cerebro)
                k.origen = "archivo"
                if !p.logica.isEmpty { k.logico = ConsejoLogico.desde(p.logica) }
                if !p.misiones.isEmpty { k.saber = SaberMisiones.desde(p.misiones) }
                k.uno = p.uno
                return k
            }.value
            try? FileManager.default.removeItem(at: copia)
            guard caja.consejo != nil else {
                self.cargandoMemoria = false
                self.estadoMemoria = "📂 ese archivo no es una memoria de Nyx (o está dañado): no cambié nada"
                return
            }
            self.aplica(caja, txtUno: caja.uno)
            self.cargandoMemoria = false
            let ideas = self.consejo.mentes.map { $0.s.count }.reduce(0, +)
            self.estadoMemoria = "📂 recordó todo del archivo: \(ideas) ideas, \(self.consejo.frases.frases.count) frases, \(self.logico.hechos.count) hechos"
            self.refrescaUno()
            self.version += 1
            self.guardaYa()                      // y queda también en la memoria de la app
        }
    }

    /// Mientras carga la memoria, no se aprende nada (se perdería al terminar de cargar).
    private func ocupado() -> Bool {
        if cargandoMemoria { avisoUno = "🧠 espera un momento: estoy recordando todo lo que sé…" }
        return cargandoMemoria
    }


    private func conecta() {
        consejo.alEvento = { [weak self] e in self?.agrega(e) }
        logico.alEvento = { [weak self] (a: Int, t: String) in self?.agregaLogica(a, t) }
        #if canImport(SwiftData)
        if let ctx = contexto {
            let b = BuzonSwiftData(contexto: ctx)
            b.alCambiar = { [weak self] in self?.refrescaTransferencias() }
            buzonSD = b
            consejo.buzon = b
        }
        #endif
    }

    private func agrega(_ e: Evento) {
        eventos.append(e)
        if eventos.count > 400 { eventos.removeFirst(eventos.count - 400) }
    }

    // MARK: hablar

    func pregunta(_ texto: String, a destino: Int) {
        if ocupado() { return }
        let t = texto.trimmingCharacters(in: .whitespacesAndNewlines)
        if t.isEmpty { return }
        contador += 1
        chat.append(MensajeChat(id: contador, deHumano: true, rol: -1, texto: t, resh: "", detalle: ""))
        contador += 1
        if destino < 0 {
            let r = consejo.delibera(t)
            let acuerdo = Int((r.acuerdo * 100).rounded())
            let quien = r.vocero >= 0 ? consejo.mentes[r.vocero].nombre : "nadie"
            let det = "ganó «\(r.ganador)» · acuerdo \(acuerdo)% · habló \(quien) · " + (r.recordada ? "lo recordó" : "frase propia")
            chat.append(MensajeChat(id: contador, deHumano: false, rol: r.vocero, texto: r.frase, resh: r.resh, detalle: det))
            ultimaRespuesta = "nyx: " + r.resh + (traducir ? "   «" + r.frase + "»" : "")
        } else {
            let r = consejo.hablaCon(destino, t)
            let det = "C=\(fmt(r.C)) · " + (r.recordada ? "lo recordó" : "frase propia")
            chat.append(MensajeChat(id: contador, deHumano: false, rol: destino, texto: r.es, resh: r.resh, detalle: det))
        }
        puedeOpinar = true
        version += 1
    }

    // MARK: sentidos

    /// Las 18 perciben una foto o un sonido.
    func percibe(_ p: Percepcion) {
        if ocupado() { return }
        notasSentidos = consejo.percibe(p)
        let d = p.descripcion
        if p.sentido == .oido {
            ultimoOido = d
        } else if p.origen == "la foto" {
            ultimaVista = d
        }
        version += 1
        guarda()
    }

    // MARK: consejo lógico

    private func agregaLogica(_ agente: Int, _ texto: String) {
        contadorLogica += 1
        lineasLogica.append(LineaLogica(id: contadorLogica, agente: agente, texto: texto))
        if lineasLogica.count > 300 { lineasLogica.removeFirst(lineasLogica.count - 300) }
    }

    /// Le preguntas (o le enseñas un hecho) al consejo lógico.
    func preguntaLogica(_ texto: String) {
        let t = texto.trimmingCharacters(in: .whitespacesAndNewlines)
        if t.isEmpty { return }
        let r = logico.procesa(t)
        veredictoLogico = r.veredicto
        explicacionLogica = r.explicacion
        var n = 0
        aportesLogicos = r.aportes.map { a -> Aporte in
            n += 1
            return Aporte(id: n, agente: a.agente, texto: a.texto)
        }
        version += 1
        guardaLogica()
    }

    /// Los 18 agentes lógicos razonan 'n' pasos por su cuenta.
    func razonaLogica(_ n: Int) {
        for _ in 0 ..< n { logico.razona() }
        version += 1
        guardaLogica()
    }

    /// El consejo lógico lee todo lo que sabe el consejo nuevo y saca hechos.
    func logicaLeeConsejoNuevo() {
        logico.encola(consejo.frases.frases.map { $0.texto })
        for _ in 0 ..< 18 * 12 { logico.razona() }
        version += 1
        guardaLogica()
    }

    private func guardaLogica() { guarda() }

    // MARK: consejo antiguo

    private func memoriaAntigua() -> Data? {
        #if canImport(SwiftData)
        if let ctx = contexto { return AlmacenSwiftData.cargaAntiguo(de: ctx) }
        #endif
        return nil
    }

    private func guardaAntiguo() async {
        if cargandoMemoria { return }
        guard let d = await puente.exporta() else { return }
        #if canImport(SwiftData)
        if let ctx = contexto { AlmacenSwiftData.guardaAntiguo(d, en: ctx) }
        #endif
    }

    /// Los dos consejos hablan 'n' turnos.
    func turnosPuente(_ n: Int) {
        if puenteOcupado { return }
        puenteOcupado = true
        Task { @MainActor in
            await self.puente.prepara(memoria: self.memoriaAntigua())
            for _ in 0 ..< n {
                let nuevas = await self.puente.turno(self.consejo)
                self.lineasPuente.append(contentsOf: nuevas)
                if self.lineasPuente.count > 300 { self.lineasPuente.removeFirst(self.lineasPuente.count - 300) }
                self.version += 1
            }
            self.puenteOcupado = false
            await self.guardaAntiguo()
            self.guarda()
        }
    }

    // MARK: Nyx Uno

    /// Le hablas a Nyx Uno: pregunta, enseñanza o corrección («no, es …»).
    func hablaUno(_ texto: String) {
        if ocupado() { return }
        let todo = texto.trimmingCharacters(in: .whitespacesAndNewlines)
        if todo.isEmpty { return }
        contadorUno += 1
        chatUno.append(MensajeUno(id: contadorUno, deHumano: true, es: todo, resh: "", detalle: "", pasos: ""))
        // los enlaces se ponen en la cola para leerlos
        let (urls, resto) = Lector.enlaces(en: todo)
        if !urls.isEmpty { adjunta(enlaces: urls) }
        let t = resto.trimmingCharacters(in: .whitespacesAndNewlines)
        if t.isEmpty { return }
        let p = uno.escucha(t)
        var detalle = ""
        var pasos: [String] = []
        if p.aprendio.isEmpty && !p.candidatos.isEmpty {
            let quien = p.elegido.map { $0.sistema.icono + " " + $0.sistema.nombre } ?? "nadie"
            detalle = "\(p.clase.nombre) · respondió \(quien) · confianza \(Int(p.confianza * 100))% · pensó \(p.rondas) ronda(s)"
            for c in p.candidatos {
                let nota = c.nota.isEmpty ? "" : " · " + c.nota
                pasos.append("\(c.sistema.icono) \(c.sistema.nombre): \(c.frase) (\(Int(c.puntos * 100)))\(nota)")
            }
            if let e = p.elegido { pasos.append(contentsOf: e.pasos.prefix(4).map { "   " + $0 }) }
        }
        contadorUno += 1
        var m = MensajeUno(id: contadorUno, deHumano: false, es: p.es, resh: p.resh, detalle: detalle, pasos: pasos.joined(separator: "\n"))
        m.pensamiento = p.pensamiento.joined(separator: "\n")
        chatUno.append(m)
        if chatUno.count > 200 { chatUno.removeFirst(chatUno.count - 200) }
        refrescaUno()
        guardaUno()
    }

    func opinaUno(_ bueno: Bool) {
        uno.opina(bueno)
        avisoUno = bueno ? "👍 anotado: se fía más de quien contestó" : "👎 anotado. Dile la correcta con «no, es …»"
        guardaUno()
    }

    /// Se entrena sola con retos (y las 18 mentes afinan sus estrategias).
    func entrenaUno(_ n: Int) {
        if ocupado() { return }
        let r = uno.entrena(n)
        avisoUno = "🎓 se entrenó con \(r.total) retos: acertó \(r.bien)"
        refrescaUno()
        guardaUno()
    }

    /// Consolida lo que sabe (como dormir).
    func consolidaUno() {
        if ocupado() { return }
        let r = uno.consolida()
        avisoUno = "💤 consolidó: \(r.hechos) hechos nuevos deducidos, \(r.pasadas) pasaron a la asociación"
        refrescaUno()
        guardaUno()
    }

    /// Lee en Wikipedia sobre un tema y lo aprende (con menos confianza que lo tuyo).
    @Published var buscandoInternet = false

    func aprendeDeInternet(_ tema: String) {
        if ocupado() { return }
        let t = tema.trimmingCharacters(in: .whitespacesAndNewlines)
        if t.isEmpty || buscandoInternet { avisoUno = "escribe un tema en la caja y pulsa 🌐"; return }
        buscandoInternet = true
        avisoUno = "🌐 buscando «\(t)» en Wikipedia…"
        Task { @MainActor in
            do {
                let l = try await Internet.aprendeSobre(t)
                var hechos: [String] = []
                for f in l.frases {
                    self.consejo.lee(f)
                    hechos += self.logico.lee(Internet.clausula(f), quien: "internet", confianza: 0.7)
                }
                self.contadorUno += 1
                let leido = l.frases.joined(separator: ". ")
                let detalle = "🌐 Wikipedia: «\(l.titulo)» · \(l.frases.count) frases leídas · \(hechos.count) hechos"
                let es = "leí sobre \(l.titulo): " + leido
                self.chatUno.append(MensajeUno(id: self.contadorUno, deHumano: false, es: es, resh: Resh.traduceTexto(es),
                                               detalle: detalle, pasos: hechos.prefix(8).joined(separator: "\n")))
                self.avisoUno = detalle
                self.refrescaUno()
                self.guardaUno()
            } catch {
                self.avisoUno = "🌐 no pude leer «\(t)» (sin internet o no está en Wikipedia)"
            }
            self.buscandoInternet = false
        }
    }

    // MARK: adjuntos (archivos, fotos, videos, audios, enlaces)

    /// Archivos ya copiados a un sitio propio (nombre para mostrar).
    func adjunta(archivos: [(URL, String)]) {
        if ocupado() { return }
        for (u, n) in archivos { colaAdjuntos.append(.archivo(u, n)) }
        sigueCola()
    }

    func adjunta(enlaces: [URL]) {
        if ocupado() { return }
        for u in enlaces { colaAdjuntos.append(.enlace(u)) }
        sigueCola()
    }

    private var leyendoCola: Bool { !adjuntando.isEmpty }

    private func sigueCola() {
        if leyendoCola || colaAdjuntos.isEmpty { return }
        adjuntando = "📎 empezando…"
        Task { @MainActor in
            var hechas = 0
            while !self.colaAdjuntos.isEmpty {
                let a = self.colaAdjuntos.removeFirst()
                hechas += 1
                let quedan = self.colaAdjuntos.count
                let aviso: (String) -> Void = { [weak self] (e: String) in
                    Task { @MainActor in self?.adjuntando = "📎 \(hechas) · \(e) · quedan \(quedan)" }
                }
                switch a {
                case .archivo(let u, let nombre):
                    self.adjuntando = "📎 leyendo «\(nombre)» · quedan \(quedan)"
                    let c = await Lector.archivo(u, nombre: nombre, progreso: aviso)
                    await self.aprendeAdjunto(c, fuente: "archivo", confianza: 0.8)
                    if u.path.hasPrefix(FileManager.default.temporaryDirectory.path) { try? FileManager.default.removeItem(at: u) }
                case .enlace(let u):
                    self.adjuntando = "🔗 \(u.host ?? "enlace") · quedan \(quedan)"
                    let c = await Lector.enlace(u, progreso: aviso)
                    await self.aprendeAdjunto(c, fuente: "internet", confianza: 0.7)
                }
            }
            self.adjuntando = ""
            self.avisoUno = "📎 leí \(hechas) cosa(s)"
            self.refrescaUno()
            self.guardaYa()
        }
    }

    /// Lo que salió de un adjunto lo aprenden su memoria y su lógica.
    private func aprendeAdjunto(_ c: Contenido, fuente: String, confianza: Float) async {
        var hechos: [String] = []
        if let p = c.percepcion {
            notasSentidos = consejo.percibe(p)
            ultimaVista = p.descripcion
        }
        if !c.escenas.isEmpty { consejo.recuerdaEscenas(c.escenas) }
        for (i, f) in c.frases.enumerated() {
            consejo.lee(f)
            hechos += logico.lee(Internet.clausula(f), quien: fuente, confianza: confianza)
            if i % 25 == 24 {
                adjuntando = "📖 «\(c.titulo)»: \(i + 1) de \(c.frases.count) frases"
                try? await Task.sleep(nanoseconds: 2_000_000)      // deja respirar a la pantalla
            }
        }
        var es = ""
        if let p = c.percepcion, c.frases.isEmpty || c.clase == .foto {
            es = "\(c.clase == .video ? "vi" : "miré") «\(c.titulo)»: " + p.descripcion
        } else if let p = c.percepcion {
            es = "leí «\(c.titulo)»: " + c.frases.prefix(2).joined(separator: ". ") + ". En la imagen: " + p.descripcion
        } else if !c.frases.isEmpty {
            es = "leí «\(c.titulo)»: " + c.frases.prefix(2).joined(separator: ". ")
        } else {
            es = "«\(c.titulo)»: " + (c.nota.isEmpty ? "no encontré nada que aprender" : c.nota)
        }
        var detalle = "\(c.clase.icono) \(c.clase.rawValue) · \(c.frases.count) frases · \(hechos.count) hechos"
        if !c.escenas.isEmpty { detalle += " · \(c.escenas.count) escenas" }
        if !c.nota.isEmpty && !c.frases.isEmpty { detalle += " · " + c.nota }
        let pasos = (hechos.prefix(6).map { "⚖️ " + $0 } + c.escenas.prefix(4).map { "🎬 " + $0 }).joined(separator: "\n")
        contadorUno += 1
        chatUno.append(MensajeUno(id: contadorUno, deHumano: false, es: es, resh: Resh.traduceTexto(es), detalle: detalle, pasos: pasos))
        if chatUno.count > 200 { chatUno.removeFirst(chatUno.count - 200) }
        version += 1
    }

    /// Nyx imagina sola: junta dos cosas que conoce y se pregunta cómo serían juntas.
    func imaginaUno() {
        if ocupado() { return }
        let p = uno.sueña()
        contadorUno += 1
        var m = MensajeUno(id: contadorUno, deHumano: false, es: p.es, resh: p.resh, detalle: "🌈 imaginación (no es un hecho)", pasos: "")
        m.pensamiento = p.pensamiento.joined(separator: "\n")
        chatUno.append(m)
        if chatUno.count > 200 { chatUno.removeFirst(chatUno.count - 200) }
        version += 1
    }

    /// Lo que Nyx quiere saber.
    func curiosidadUno() {
        guard let q = uno.curiosidad() else { avisoUno = "no se le ocurre nada que preguntar"; return }
        contadorUno += 1
        chatUno.append(MensajeUno(id: contadorUno, deHumano: false, es: "quiero saber: " + q, resh: Resh.traduceTexto(q) + "?", detalle: "curiosidad", pasos: ""))
    }

    func refrescaUno() {
        let pct = uno.entrenadas == 0 ? 0 : uno.entrenadasBien * 100 / uno.entrenadas
        estadoUno = "entrenó \(uno.entrenadas) retos (\(pct)% bien) · \(uno.recuerdos.count) recuerdos · \(logico.hechos.count) hechos · \(consejo.frases.frases.count) frases"
        version += 1
    }

    /// Cuánto se fía de cada forma de pensar en cada clase de pregunta.
    func fiabilidadesUno() -> [String] {
        return Clase.allCases.map { c -> String in
            let fs = Sistema.allCases.map { "\($0.icono)\(Int(uno.fiabilidad(c, $0) * 100))" }.joined(separator: " ")
            return c.nombre + ": " + fs
        }
    }

    private func guardaUno() { guarda() }

    // MARK: la asamblea de los tres consejos

    /// Que hablen (a su ritmo, hasta que ellos quieran) o que se callen.
    func alternaAsamblea() {
        if asambleaViva {
            asambleaViva = false
            estadoAsamblea = "les pediste silencio"
        } else {
            arrancaAsamblea()
        }
    }

    /// Escribes en el chat de los tres (y si estaban callados, te contestan).
    func escribeAsamblea(_ texto: String) {
        if ocupado() { return }
        asamblea.escribe(texto, logico: logico)
        refrescaAsamblea()
        if !asambleaViva { arrancaAsamblea() }
    }

    /// Les pones una misión (nil: al azar).
    func misionAsamblea(_ tipo: TipoMision?) {
        asamblea.proponMision(tipo)
        refrescaAsamblea()
        if !asambleaViva { arrancaAsamblea() }
    }

    private func arrancaAsamblea() {
        if asambleaViva { return }
        if puenteOcupado {
            estadoAsamblea = "el consejo antiguo está ocupado: espera un momento"
            return
        }
        asambleaViva = true
        puenteOcupado = true
        tareaAsamblea = Task { @MainActor in
            await self.puente.prepara(memoria: self.memoriaAntigua())
            var n = 0
            while self.asambleaViva {
                let r = await self.asamblea.siguiente(nuevo: self.consejo, puente: self.puente, logico: self.logico)
                self.refrescaAsamblea()
                n += 1
                if !r.sigue { break }
                if n % 30 == 0 { self.guardaMisiones() }
                try? await Task.sleep(nanoseconds: UInt64(r.pausa * 1_000_000_000))
            }
            self.asambleaViva = false
            self.puenteOcupado = false
            self.refrescaAsamblea()
            await self.guardaAntiguo()
            self.guardaLogica()
            self.guardaMisiones()
            self.guarda()
        }
    }

    private func refrescaAsamblea() {
        lineasAsamblea = asamblea.lineas
        estadoAsamblea = asamblea.estado
        let p = asamblea.saber.puntos
        marcador = "✨ \(p[0])   🏛 \(p[1])   ⚖️ \(p[2])   · \(asamblea.saber.jugadas) misiones"
        resumenMisiones = asamblea.saber.resumen()
        version += 1
    }

    private func guardaMisiones() { guarda() }

    /// Le preguntas al consejo antiguo (deliberan y votan, a su manera).
    func preguntaAntiguo(_ texto: String) {
        let t = texto.trimmingCharacters(in: .whitespacesAndNewlines)
        if t.isEmpty || puenteOcupado { return }
        puenteOcupado = true
        respuestaAntiguo = "pensando…"
        Task { @MainActor in
            await self.puente.prepara(memoria: self.memoriaAntigua())
            let r = await self.puente.preguntaAlViejo(t)
            self.respuestaAntiguo = "🏛 " + r.veredicto + "   (" + r.votos.joined(separator: " · ") + ")"
            self.puenteOcupado = false
        }
    }

    /// Un video visto entero: el resumen lo perciben; cada escena la recuerdan.
    func veVideo(_ resumen: Percepcion, escenas: [String], fotogramas: Int, duracion: Double) {
        percibe(resumen)
        consejo.recuerdaEscenas(escenas)
        lineasVideo = escenas
        ultimoVideo = "Lo vieron entero: \(fotogramas) fotogramas, \(escenas.count) escenas. " + resumen.descripcion
        version += 1
        guarda()
    }

    /// Lo que ve la cámara: solo lo perciben de verdad cuando cambia algo.
    func veCamara(_ p: Percepcion) {
        ultimaCamara = p.descripcion
        let cosas = Set(p.cosas)
        if cosas.isEmpty || cosas == cosasCamara { return }
        cosasCamara = cosas
        percibe(p)
    }

    /// Quita la traducción «…» de un texto si no se quiere ver.
    func soloResh(_ t: String) -> String {
        if traducir { return t }
        guard let r = t.range(of: "   «") else { return t }
        return String(t[..<r.lowerBound])
    }

    func opina(_ bueno: Bool) {
        guard puedeOpinar else { return }
        if consejo.opina(bueno: bueno) {
            contador += 1
            let t = bueno ? "👍 lo reforzaron (y lo compartieron con las demás)" : "👎 lo inhibieron: la próxima vez gana otra respuesta"
            chat.append(MensajeChat(id: contador, deHumano: false, rol: -2, texto: t, resh: "", detalle: ""))
        }
        puedeOpinar = false
        guarda()
    }

    // MARK: vida libre

    func alternaVivo() {
        vivo.toggle()
        if vivo { arranca() } else { tarea?.cancel(); guarda() }
    }

    private func arranca() {
        tarea?.cancel()
        tarea = Task { @MainActor [weak self] in
            while let s = self, s.vivo, !Task.isCancelled {
                s.unPaso()
                let ns = UInt64(max(0.05, s.pausa) * 1_000_000_000)
                try? await Task.sleep(nanoseconds: ns)
            }
        }
    }

    func unPaso() {
        consejo.paso()
        version += 1
        if consejo.tick % 30 == 0 { guarda() }
    }

    func conversa(_ a: Int, _ b: Int) {
        if a == b { return }
        consejo.conversa(a, b, turnos: 8)
        version += 1
    }

    // MARK: enseñar y memoria

    func ensena(_ texto: String) -> Int {
        if ocupado() { return 0 }
        var lineas = 0
        for l in texto.split(whereSeparator: { $0 == "\n" || $0 == "." }) {
            // puede venir en Resh: se traduce (los números y signos se quedan)
            let t = Resh.traduceConservando(l.trimmingCharacters(in: .whitespaces))
            if t.isEmpty { continue }
            consejo.lee(t)
            logico.lee(t, quien: "tú")
            lineas += 1
        }
        version += 1
        guardaYa()
        return lineas
    }

    /// Guardar todo el cerebro es pesado (miles de ideas): se guarda como mucho
    /// cada 20 s cuando aprendió algo, en segundo plano, y al salir de la app.
    func guarda() {
        if cargandoMemoria { return }
        let pasado = Date().timeIntervalSince(ultimaGuardada)
        if pasado >= 20 { guardaYa(); return }
        if guardadoPendiente { return }
        guardadoPendiente = true
        Task { @MainActor in
            try? await Task.sleep(nanoseconds: UInt64(max(1, 20 - pasado) * 1_000_000_000))
            if self.guardadoPendiente { self.guardaYa() }
        }
    }

    /// Guarda ya: saca una foto del cerebro (al instante), la pasa a texto en
    /// segundo plano, escribe una copia de seguridad en un archivo y la mete en SwiftData.
    func guardaYa() {
        if cargandoMemoria { return }          // nunca pisar la memoria con un cerebro a medio cargar
        if guardando { guardadoPendiente = true; return }
        guardando = true
        guardadoPendiente = false
        ultimaGuardada = Date()
        let foto = consejo.foto()
        let txtLogico = logico.exporta()
        let txtSaber = asamblea.saber.exporta()
        let txtUno = uno.exporta()
        Task { @MainActor in
            let hecho = await Task.detached(priority: .utility) { () -> TextosCerebro in
                let t = TextosCerebro(mentes: foto.textosMentes(), frases: foto.textoFrases())
                // copia de seguridad en archivos (por si SwiftData falla)
                let todo = "NYX 1 \(foto.tick)\n" + t.mentes.joined(separator: "\n") + "\n" + t.frases + "\nFIN\n"
                try? todo.write(to: Consejo.rutaMemoria, atomically: true, encoding: .utf8)
                try? txtLogico.write(to: NyxModelo.rutaLogico, atomically: true, encoding: .utf8)
                return t
            }.value
            #if canImport(SwiftData)
            if let ctx = self.contexto {
                AlmacenSwiftData.guardaTextos(mentes: hecho.mentes, frases: hecho.frases, en: ctx)
                AlmacenSwiftData.guardaLogico(txtLogico, en: ctx)
                AlmacenSwiftData.guardaTexto(txtSaber, rol: 400, en: ctx)
                AlmacenSwiftData.guardaTexto(txtUno, rol: 500, en: ctx)
                self.buzonSD?.limpia(dejando: 400)
            }
            #endif
            self.guardando = false
            let f = DateFormatter()
            f.dateFormat = "HH:mm:ss"
            self.estadoMemoria = "💾 guardado a las " + f.string(from: Date())
            if self.guardadoPendiente {
                self.guardadoPendiente = false
                self.guarda()
            }
        }
    }

    func olvidaTodo() {
        vivo = false
        tarea?.cancel()
        consejo = Consejo(infancia: true)
        logico = ConsejoLogico(primer: true)
        lineasLogica = []
        asambleaViva = false
        asamblea.olvida()
        lineasAsamblea = []
        uno = NyxUno(consejo: consejo, logica: logico, saber: asamblea.saber)
        chatUno = []
        #if canImport(SwiftData)
        if let ctx = contexto {
            try? ctx.delete(model: Transferencia.self)
            try? ctx.delete(model: SaberMente.self)
            try? ctx.save()
        }
        #endif
        conecta()
        chat = []
        eventos = []
        // también la copia de seguridad (si no, «olvidar» la resucitaría)
        try? FileManager.default.removeItem(at: Consejo.rutaMemoria)
        try? FileManager.default.removeItem(at: NyxModelo.rutaLogico)
        guardaYa()
        refrescaTransferencias()
        version += 1
    }

    func refrescaTransferencias() {
        #if canImport(SwiftData)
        if let b = buzonSD {
            var filas: [FilaTransferencia] = []
            var n = 0
            for t in b.recientes(150) {
                n += 1
                filas.append(FilaTransferencia(id: n, de: t.de, para: t.para, tipo: t.tipo,
                                               contenido: t.contenido, recogieron: t.cuantasRecogieron))
            }
            transferencias = filas
        }
        #endif
    }

    // MARK: datos para la pantalla

    func filas() -> [FilaMente] {
        var out: [FilaMente] = []
        for m in consejo.mentes {
            out.append(FilaMente(id: m.rol, nombre: m.nombre, precision: Double(m.precision), ideas: m.s.count,
                                 resh: m.cuentaResh, dichos: m.dichos, gusta: m.mejorAccion.nombre))
        }
        return out
    }

    /// Gusto total por cada acción (rol + lo aprendido).
    func gustos(_ rol: Int) -> [Gusto] {
        let m = consejo.mentes[rol]
        var out: [Gusto] = []
        for a in Accion.allCases {
            let v = Double(m.info.gusto[a.rawValue] + m.valor[a.rawValue])
            out.append(Gusto(id: a.rawValue, nombre: a.nombre, valor: v, veces: m.veces[a.rawValue]))
        }
        return out
    }

    /// Lo más activo en su mente.
    func activo(_ rol: Int) -> [String] {
        let m = consejo.mentes[rol]
        var ids: [Int] = []
        for i in 0 ..< m.s.count where !Palabras.vacia(m.s[i].et) && !(m.s[i].fusion && m.s[i].dialogo == 0) {
            ids.append(i)
        }
        ids.sort { m.s[$0].A > m.s[$1].A }
        return ids.prefix(10).map { i -> String in
            let x = m.s[i]
            let r = x.sabe ? (Resh.deEspanol(x.et).map { " · resh " + $0 } ?? "") : ""
            return "\(x.et)  A=\(fmt(x.A))\(r)"
        }
    }

    func foco(_ rol: Int) -> String {
        let m = consejo.mentes[rol]
        return m.foco.reversed().map { m.s[$0].et }.joined(separator: " · ")
    }

    func recuerdos(_ rol: Int) -> [String] {
        return Array(consejo.mentes[rol].ventana.suffix(6).reversed())
    }
}
#endif
