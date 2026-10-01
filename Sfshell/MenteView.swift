import SwiftUI

// ============================================================
//  MenteView — la ventana de Nyx dentro de Sfshell. Se abre con
//  'nyx ver' en la terminal o desde el escritorio. Usa el MISMO
//  consejo que el comando 'nyx' (NyxNucleo), no uno aparte.
//  UI para iPad con menú lateral:
//   · Consejo — tablero de las 18 mentes y diálogo en vivo
//   · Chat del consejo — las 18 mentes + tú, habla libre
//   · Enseñar palabras — "esto significa esto en tu idioma"
//   · Léxico — explora las 1500 palabras y enséñalas con un toque
// ============================================================

// ---------- Destinos del menú ----------
enum Destino: Hashable {
    case consejo, hablar, ensenar, lexico
}

// ---------- Identidad visual por rol ----------
extension RolMental {
    var color: Color {
        switch self {
        case .sintaxis: return .blue
        case .semantica: return .purple
        case .logica: return .indigo
        case .codigo: return .green
        case .creativo: return .orange
        case .critico: return .red
        case .memoria: return .teal
        case .percepcion: return .pink
        case .sintesis: return .yellow
        case .intuicion: return .cyan
        case .analogia: return .mint
        case .contexto: return .brown
        case .esceptico: return .gray
        case .narrativa: return .indigo
        case .etica: return .red
        case .curiosidad: return .yellow
        case .abstraccion: return .purple
        case .empatia: return .pink
        }
    }

    var icono: String {
        switch self {
        case .sintaxis: return "textformat.abc"
        case .semantica: return "brain.head.profile"
        case .logica: return "function"
        case .codigo: return "chevron.left.forwardslash.chevron.right"
        case .creativo: return "paintbrush.pointed"
        case .critico: return "exclamationmark.triangle"
        case .memoria: return "archivebox"
        case .percepcion: return "eye"
        case .sintesis: return "circle.hexagongrid"
        case .intuicion: return "sparkles"
        case .analogia: return "link"
        case .contexto: return "map.fill"
        case .esceptico: return "questionmark.circle.fill"
        case .narrativa: return "book.fill"
        case .etica: return "checkmark.shield.fill"
        case .curiosidad: return "magnifyingglass"
        case .abstraccion: return "triangle.fill"
        case .empatia: return "heart.fill"
        }
    }
}

// ============================================================
//  CONTENT VIEW — menú lateral (iPad)
// ============================================================
struct NyxVista: View {
    /// Dónde guardar la memoria (la ruta real de /var/nyx/memoria.json)
    var memoria: URL? = nil
    @StateObject private var modelo = ModeloMente()
    @State private var destino: Destino? = .consejo

    init(memoria: URL? = nil) {
        self.memoria = memoria
    }

    var body: some View {
        NavigationSplitView {
            List(selection: $destino) {
                Label("Consejo", systemImage: "person.3.fill")
                    .tag(Destino.consejo)
                Label("Chat del consejo", systemImage: "bubble.left.and.bubble.right.fill")
                    .tag(Destino.hablar)
                Label("Enseñar palabras", systemImage: "graduationcap.fill")
                    .tag(Destino.ensenar)
                Label("Léxico", systemImage: "book.fill")
                    .tag(Destino.lexico)
            }
            .navigationTitle("Nyx")
        } detail: {
            switch destino ?? .consejo {
            case .consejo: VistaConsejo(modelo: modelo)
            case .hablar: VistaHablar(modelo: modelo)
            case .ensenar: VistaEnsenar(modelo: modelo)
            case .lexico: VistaLexico(modelo: modelo)
            }
        }
        .task {
            await modelo.arrancar(memoria: memoria)
        }
        .preferredColorScheme(.dark) // estética Grok: siempre oscuro
    }
}

// ============================================================
//  CONSEJO — tablero principal
// ============================================================
struct VistaConsejo: View {
    @ObservedObject var modelo: ModeloMente
    @Environment(\.horizontalSizeClass) private var sizeClass
    private let columnas = [
        GridItem(.flexible()), GridItem(.flexible()), GridItem(.flexible()),
        GridItem(.flexible()), GridItem(.flexible()), GridItem(.flexible()),
    ]

    var body: some View {
        ScrollView {
            if sizeClass == .regular {
                // iPad: dos columnas
                HStack(alignment: .top, spacing: 16) {
                    VStack(spacing: 14) {
                        encabezado
                        tarjetaVeredicto
                        mentesGrid
                    }
                    .frame(maxWidth: .infinity)
                    dialogoEnVivo
                        .frame(width: 360)
                }
                .padding()
            } else {
                VStack(spacing: 14) {
                    encabezado
                    tarjetaVeredicto
                    mentesGrid
                    dialogoEnVivo
                }
                .padding()
            }
        }
        .navigationTitle("Consejo × 18")
        .safeAreaInset(edge: .bottom) { barraInput }
    }

    private var encabezado: some View {
        HStack(spacing: 10) {
            ZStack {
                Circle()
                    .fill(Color.green.opacity(0.35))
                    .frame(width: 14, height: 14)
                    .scaleEffect(modelo.enMarcha ? 1.8 : 1.0)
                    .opacity(modelo.enMarcha ? 0 : 0.35)
                    .animation(
                        .easeOut(duration: 1.4).repeatForever(autoreverses: false),
                        value: modelo.enMarcha
                    )
                Circle()
                    .fill(Color.green)
                    .frame(width: 10, height: 10)
            }
            VStack(alignment: .leading, spacing: 2) {
                Text(modelo.enMarcha ? "18 mentes en diálogo" : "despertando…")
                    .font(.headline)
                Text("memoria privada total: \(modelo.memoriaTotal) palabras")
                    .font(.caption2)
                    .foregroundColor(.secondary)
            }
            Spacer()
        }
    }

    private var tarjetaVeredicto: some View {
        VStack(spacing: 6) {
            Text("VEREDICTO DEL CONSEJO")
                .font(.caption2)
                .foregroundColor(.secondary)
            if modelo.deliberando {
                ProgressView().padding(.vertical, 4)
            } else {
                Text(modelo.veredicto)
                    .font(.title2)
                    .bold()
                    .multilineTextAlignment(.center)
            }
        }
        .frame(maxWidth: .infinity)
        .padding()
        .background(
            LinearGradient(
                colors: [.purple.opacity(0.18), .blue.opacity(0.10)],
                startPoint: .topLeading,
                endPoint: .bottomTrailing
            )
        )
        .cornerRadius(16)
    }

    private var mentesGrid: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("VOTOS POR MENTE")
                .font(.caption2).bold().foregroundColor(.secondary)
            if modelo.votos.isEmpty {
                Text("Mándales un input para verlas votar…")
                    .font(.caption).foregroundColor(.secondary)
                    .frame(maxWidth: .infinity).padding()
                    .background(Color(.secondarySystemBackground).opacity(0.6))
                    .cornerRadius(12)
            } else {
                LazyVGrid(columns: columnas, spacing: 10) {
                    ForEach(modelo.votos, id: \.rol) { v in
                        TarjetaMente(
                            voto: v,
                            maxCoherencia: modelo.votos.map(\.coherencia).max() ?? 1
                        )
                    }
                }
            }
        }
    }

    private var dialogoEnVivo: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("DIÁLOGO EN VIVO")
                    .font(.caption2).bold().foregroundColor(.secondary)
                Spacer()
                Image(systemName: "waveform")
                    .font(.caption).foregroundColor(.secondary)
            }
            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(spacing: 6) {
                        ForEach(modelo.log.indices, id: \.self) { i in
                            LineaDialogo(texto: modelo.log[i]).id(i)
                        }
                    }
                    .padding(2)
                }
                .frame(minHeight: 220)
                .onChange(of: modelo.log.count) { _, _ in
                    if let ultimo = modelo.log.indices.last {
                        withAnimation { proxy.scrollTo(ultimo, anchor: .bottom) }
                    }
                }
            }
        }
        .padding(10)
        .background(Color(.secondarySystemBackground).opacity(0.6))
        .cornerRadius(12)
    }

    private var barraInput: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("PRUEBA — deliberación forzada")
                .font(.caption2).bold().foregroundColor(.secondary)
            HStack(spacing: 8) {
                TextField("Input para las 18 mentes…", text: $modelo.entrada)
                    .textFieldStyle(.roundedBorder)
                    .disabled(modelo.deliberando)
                Button {
                    modelo.deliberar()
                } label: {
                    Image(systemName: "arrow.up.circle.fill").font(.title2)
                }
                .disabled(modelo.entrada.isEmpty || modelo.deliberando)
            }
        }
        .padding(.horizontal)
        .padding(.vertical, 8)
        .background(.thinMaterial)
    }
}

// ---------- Tarjeta de una mente ----------
struct TarjetaMente: View {
    let voto: VotoMental
    let maxCoherencia: Double

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 6) {
                Image(systemName: voto.rol.icono)
                    .foregroundColor(voto.rol.color).font(.caption)
                Text(voto.rol.rawValue.capitalized)
                    .font(.caption).bold().lineLimit(1)
                Spacer()
            }
            Text(voto.atractor).font(.callout).lineLimit(1)
            GeometryReader { geo in
                RoundedRectangle(cornerRadius: 3)
                    .fill(voto.rol.color.opacity(0.85))
                    .frame(width: geo.size.width * fraccion)
            }
            .frame(height: 6)
            Text(String(format: "%.2f", voto.coherencia))
                .font(.caption2).foregroundColor(.secondary)
        }
        .padding(10)
        .background(Color(.secondarySystemBackground))
        .cornerRadius(12)
    }

    private var fraccion: Double {
        guard maxCoherencia > 0 else { return 0 }
        return min(1, max(0, voto.coherencia / maxCoherencia))
    }
}

// ---------- Línea del diálogo ----------
struct LineaDialogo: View {
    let texto: String

    var body: some View {
        HStack(alignment: .top, spacing: 8) {
            Circle().fill(color).frame(width: 8, height: 8).padding(.top, 4)
            Text(texto).font(.caption).lineLimit(2)
            Spacer()
        }
        .padding(.horizontal, 8).padding(.vertical, 6)
        .background(color.opacity(0.08))
        .cornerRadius(8)
    }

    private var color: Color {
        if texto.contains("[propuesta]") { return .orange }
        if texto.contains("[pregunta]") { return .blue }
        if texto.contains("[dato]") { return .green }
        if texto.contains("veredicto") { return .purple }
        if texto.contains("enseñado") { return .teal }
        if texto.contains("humano") { return .pink }
        return .gray
    }
}

// ============================================================
//  HABLAR EN RESH — chat directo en su idioma
// ============================================================

struct EntradaLexico: Hashable {
    let es: String
    let resh: String
}

// ============================================================
//  CHAT DEL CONSEJO — estilo Grok: negro, texto plano, input abajo
// ============================================================
struct VistaHablar: View {
    @ObservedObject var modelo: ModeloMente

    var body: some View {
        ZStack {
            Color.black.ignoresSafeArea()
            VStack(spacing: 0) {
                ScrollViewReader { proxy in
                    ScrollView {
                        LazyVStack(spacing: 18) {
                            if modelo.chat.isEmpty {
                                saludoInicial
                            }
                            ForEach(modelo.chat) { m in
                                FilaChat(mensaje: m).id(m.id)
                            }
                            if modelo.enviando {
                                FilaPensando().id("pensando")
                            }
                        }
                        .padding(.horizontal)
                        .padding(.vertical, 12)
                    }
                    .onChange(of: modelo.chat.count) { _, _ in desplazar(proxy) }
                    .onChange(of: modelo.enviando) { _, _ in desplazar(proxy) }
                }

                // Partículas rápidas: tócalas para armar frases
                ScrollView(.horizontal, showsIndicators: false) {
                    HStack(spacing: 8) {
                        ForEach(LenguaResh.particulas, id: \.forma) { p in
                            Button(p.forma) {
                                modelo.mensajeChat += (modelo.mensajeChat.isEmpty ? "" : " ") + p.forma
                            }
                            .font(.caption)
                            .padding(.horizontal, 10).padding(.vertical, 6)
                            .background(Color(white: 0.14))
                            .foregroundColor(.gray)
                            .cornerRadius(14)
                        }
                    }
                    .padding(.horizontal)
                }
                .padding(.vertical, 6)

                HStack(spacing: 8) {
                    TextField("Pregúntale al consejo…", text: $modelo.mensajeChat)
                        .padding(12)
                        .background(Color(white: 0.12))
                        .cornerRadius(22)
                        .foregroundColor(.white)
                        .autocorrectionDisabled()
                        .textInputAutocapitalization(.never)
                        .disabled(modelo.enviando)
                    Button {
                        modelo.enviarAlChat()
                    } label: {
                        Image(systemName: "arrow.up")
                            .font(.body.bold())
                            .foregroundColor(.black)
                            .frame(width: 38, height: 38)
                            .background(botonActivo ? Color.white : Color.gray.opacity(0.35))
                            .clipShape(Circle())
                    }
                    .disabled(!botonActivo)
                }
                .padding()
                .background(Color.black)
            }
        }
        .toolbar {
            ToolbarItem(placement: .principal) {
                VStack(spacing: 0) {
                    HStack(spacing: 4) {
                        Text("Nyx").bold().foregroundColor(.white)
                        Image(systemName: "chevron.down")
                            .font(.caption2).foregroundColor(.secondary)
                    }
                    Text("consejo de 18 mentes")
                        .font(.caption2).foregroundColor(.secondary)
                }
            }
            ToolbarItem(placement: .topBarTrailing) {
                Button {
                    modelo.limpiarChat()
                } label: {
                    Image(systemName: "square.and.pencil").foregroundColor(.white)
                }
            }
        }
    }

    private var botonActivo: Bool {
        !modelo.mensajeChat.trimmingCharacters(in: .whitespaces).isEmpty && !modelo.enviando
    }

    private func desplazar(_ proxy: ScrollViewProxy) {
        if let u = modelo.chat.last {
            withAnimation { proxy.scrollTo(u.id, anchor: .bottom) }
        } else if modelo.enviando {
            withAnimation { proxy.scrollTo("pensando", anchor: .bottom) }
        }
    }

    private var saludoInicial: some View {
        VStack(spacing: 10) {
            Spacer().frame(height: 60)
            Image(systemName: "sparkles")
                .font(.system(size: 44)).foregroundColor(.white)
            Text("Hola, soy Nyx")
                .font(.title2).bold().foregroundColor(.white)
            Text("18 mentes resonando en un solo chat")
                .font(.callout).foregroundColor(.secondary)
            Spacer().frame(height: 40)
        }
        .frame(maxWidth: .infinity)
    }
}

// Una fila del chat: el humano a la derecha en burbuja gris,
// las mentes como texto plano con avatar (como Grok).
struct FilaChat: View {
    let mensaje: MensajeChat

    var body: some View {
        if mensaje.esHumano {
            HStack {
                Spacer()
                Text(mensaje.texto)
                    .font(.body).foregroundColor(.white)
                    .padding(.horizontal, 14).padding(.vertical, 10)
                    .background(Color(white: 0.16))
                    .cornerRadius(18)
            }
        } else {
            HStack(alignment: .top, spacing: 10) {
                AvatarRol(autor: mensaje.autor)
                VStack(alignment: .leading, spacing: 4) {
                    if !mensaje.autor.isEmpty {
                        Text(mensaje.autor)
                            .font(.caption).bold()
                            .foregroundColor(mensaje.destacada ? .yellow : .secondary)
                    }
                    Text(mensaje.texto)
                        .font(.body).foregroundColor(.white)
                    if mensaje.destacada {
                        Text("respuesta del consejo · mayor peso")
                            .font(.caption2).foregroundColor(.yellow)
                    }
                }
                Spacer()
            }
            .padding(.leading, 2)
            .overlay(alignment: .leading) {
                if mensaje.destacada {
                    Rectangle().fill(Color.yellow).frame(width: 2)
                }
            }
        }
    }
}

// Avatar circular con el icono y color del rol.
struct AvatarRol: View {
    let autor: String

    var body: some View {
        let rol = RolMental(rawValue: autor)
        ZStack {
            Circle()
                .fill(rol?.color ?? Color(white: 0.2))
                .frame(width: 30, height: 30)
            Image(systemName: rol?.icono ?? "sparkles")
                .font(.caption)
                .foregroundColor(.white)
        }
    }
}

// "El consejo está pensando…": tres puntos pulsando.
struct FilaPensando: View {
    var body: some View {
        HStack(alignment: .top, spacing: 10) {
            AvatarRol(autor: "")
            HStack(spacing: 5) {
                ForEach(0 ..< 3) { i in
                    PuntoPulsante(retraso: Double(i) * 0.25)
                }
            }
            .padding(.top, 10)
            Spacer()
        }
    }
}

struct PuntoPulsante: View {
    let retraso: Double
    @State private var encendido = false

    var body: some View {
        Circle()
            .frame(width: 7, height: 7)
            .foregroundColor(.gray)
            .opacity(encendido ? 1 : 0.25)
            .onAppear {
                withAnimation(
                    .easeInOut(duration: 0.5).repeatForever(autoreverses: true).delay(retraso)
                ) {
                    encendido = true
                }
            }
    }
}

// ============================================================
//  ENSEÑAR — "esto significa esto en tu idioma"
// ============================================================
struct VistaEnsenar: View {
    @ObservedObject var modelo: ModeloMente

    var body: some View {
        ScrollView {
            VStack(spacing: 14) {
                Text("Enséñales palabra por palabra. Cada mente la guarda en su memoria privada; lo que no saben lo preguntan entre ellas.")
                    .font(.caption)
                    .foregroundColor(.secondary)

                TextField("palabra en español", text: $modelo.palabraEspañol)
                    .textFieldStyle(.roundedBorder)
                    .autocorrectionDisabled()
                TextField("significa en Resh (separa con comas)", text: $modelo.significados)
                    .textFieldStyle(.roundedBorder)
                    .autocorrectionDisabled()
                    .textInputAutocapitalization(.never)

                if !modelo.palabraEspañol.isEmpty {
                    Text("sugerencia del léxico: \(LenguaResh.traducir(modelo.palabraEspañol) ?? "—")")
                        .font(.callout)
                        .foregroundColor(.secondary)
                }

                Picker("Enseñar a", selection: $modelo.destinoEnsenanza) {
                    Text("Las 18 mentes").tag("todas")
                    ForEach(RolMental.allCases, id: \.self) { rol in
                        Text("Solo \(rol.rawValue.capitalized)").tag(rol.rawValue)
                    }
                }
                .pickerStyle(.menu)

                Button {
                    modelo.enseñar()
                } label: {
                    Label("Enseñar", systemImage: "graduationcap")
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                .disabled(modelo.palabraEspañol.isEmpty || modelo.significados.isEmpty)

                Text("\"esto significa esto, esto… en tu idioma\" — si le enseñas a una sola mente, las demás le preguntarán a ella cuando la necesiten.")
                    .font(.caption2)
                    .foregroundColor(.secondary)
                    .italic()
            }
            .padding()
        }
        .navigationTitle("Enseñar palabras")
    }
}

// ============================================================
//  LÉXICO — explora y enseña con un toque
// ============================================================
struct VistaLexico: View {
    @ObservedObject var modelo: ModeloMente
    @State private var busqueda = ""

    private var entradas: [EntradaLexico] {
        let todas = LenguaResh.pares.map { EntradaLexico(es: $0.0, resh: $0.1) }
        let q = busqueda.lowercased().trimmingCharacters(in: .whitespaces)
        guard !q.isEmpty else { return Array(todas.prefix(120)) }
        return todas.filter { $0.es.contains(q) || $0.resh.contains(q) }
    }

    var body: some View {
        List {
            Section("Partículas de protocolo") {
                ForEach(LenguaResh.particulas, id: \.forma) { p in
                    HStack {
                        Text(p.forma)
                            .bold()
                            .foregroundColor(.orange)
                            .frame(width: 44, alignment: .leading)
                        Text(p.significado)
                            .font(.caption)
                            .foregroundColor(.secondary)
                        Spacer()
                        Button("Enseñar") {
                            let concepto = LenguaResh.conceptoPorParticula[p.forma] ?? p.forma
                            modelo.ensenarRapido(español: concepto, resh: p.forma)
                        }
                        .font(.caption)
                        .buttonStyle(.bordered)
                    }
                }
            }
            Section("Léxico (\(busqueda.isEmpty ? "primeras 120" : "resultados"))") {
                ForEach(entradas, id: \.es) { e in
                    HStack {
                        Text(e.es)
                            .frame(maxWidth: .infinity, alignment: .leading)
                        Text(e.resh)
                            .bold()
                            .foregroundColor(.purple)
                        Button("Enseñar") {
                            modelo.ensenarRapido(español: e.es, resh: e.resh)
                        }
                        .font(.caption)
                        .buttonStyle(.bordered)
                    }
                    .font(.callout)
                }
            }
        }
        .searchable(text: $busqueda, prompt: "buscar palabra…")
        .navigationTitle("Léxico Resh")
    }
}

// ============================================================
//  MODELO
// ============================================================
@MainActor
final class ModeloMente: ObservableObject {
    private var consejo: ConsejoResonante { NyxNucleo.uno.consejo }

    @Published var veredicto = "esperando input…"
    @Published var votos: [VotoMental] = []
    @Published var log: [String] = []
    @Published var entrada = ""
    @Published var deliberando = false
    @Published var palabraEspañol = ""
    @Published var significados = ""
    @Published var destinoEnsenanza = "todas"
    @Published var memoriaTotal = 0
    @Published private(set) var enMarcha = false

    // Chat en Resh
    @Published var chat: [MensajeChat] = []
    @Published var mensajeChat = ""
    @Published var enviando = false

    func arrancar(memoria: URL?) async {
        guard !enMarcha else { return }

        _ = await NyxNucleo.uno.preparar(memoria: memoria)
        await NyxNucleo.uno.despertar()
        log = await consejo.recentLog()
        enMarcha = true
        Task {
            while !Task.isCancelled {
                try? await Task.sleep(nanoseconds: 2_000_000_000)
                log = await consejo.recentLog()
                chat = await consejo.chatReciente()
                memoriaTotal = await consejo.memoriaTotal()
            }
        }
    }

    func deliberar() {
        let input = entrada
        guard !input.isEmpty else { return }
        entrada = ""
        deliberando = true
        Task {
            let resultado = await consejo.deliberar(sobre: input)
            veredicto = resultado.veredicto
            votos = resultado.votos
            log = await consejo.recentLog()
            deliberando = false
        }
    }

    /// Enseña una palabra: a las 18 o a una sola mente.
    func enseñar() {
        let esp = palabraEspañol.trimmingCharacters(in: .whitespaces)
        let sigs = significados.split(separator: ",").map {
            $0.trimmingCharacters(in: .whitespaces)
        }.filter { !$0.isEmpty }
        guard !esp.isEmpty, !sigs.isEmpty else { return }
        let destino = destinoEnsenanza
        palabraEspañol = ""
        significados = ""
        Task {
            if destino == "todas" {
                await consejo.enseñarATodos(español: esp, significa: sigs)
            } else if let rol = RolMental(rawValue: destino) {
                await consejo.enseñarA(rol: rol, español: esp, significa: sigs)
            }
            log = await consejo.recentLog()
            memoriaTotal = await consejo.memoriaTotal()
        }
    }

    /// Enseñanza rápida desde el léxico (una palabra, una forma).
    func ensenarRapido(español: String, resh: String) {
        Task {
            await consejo.enseñarATodos(español: español, significa: [resh])
            log = await consejo.recentLog()
            memoriaTotal = await consejo.memoriaTotal()
        }
    }

    /// Nuevo chat: limpia el historial del consejo.
    func limpiarChat() {
        Task {
            await consejo.limpiarChat()
            chat = await consejo.chatReciente()
        }
    }

    /// Chat libre del consejo: tu mensaje es uno más del éter.
    /// Las mentes responden libremente; se destaca la de mayor peso.
    func enviarAlChat() {
        let m = mensajeChat.trimmingCharacters(in: .whitespaces)
        guard !m.isEmpty else { return }
        mensajeChat = ""
        enviando = true
        chat.append(MensajeChat(texto: m, esHumano: true, autor: "tú"))
        Task {
            await consejo.conversar(m)
            chat = await consejo.chatReciente()
            log = await consejo.recentLog()
            memoriaTotal = await consejo.memoriaTotal()
            enviando = false
        }
    }
}
