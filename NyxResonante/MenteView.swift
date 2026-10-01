// MenteView.swift — v16 (2026-10-01): ojo a 30 fps.
import SwiftUI
import AVFoundation
import CoreGraphics

// ============================================================
//  MenteView — pega este contenido en tu ContentView.swift
//  UI para iPad con menú lateral:
//   · Consejo — tablero de las 18 mentes y diálogo en vivo
//   · Chat del consejo — las 18 mentes + tú, habla libre
//   · Enseñar palabras — "esto significa esto en tu idioma"
//   · Léxico — explora las 1500 palabras y enséñalas con un toque
// ============================================================

// ---------- Destinos del menú ----------
enum Destino: Hashable {
    case consejo, hablar, ensenar, lexico, ojo
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
struct ContentView: View {
    @StateObject private var modelo = ModeloMente()
    @State private var destino: Destino? = .consejo

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
                Label("Ojo", systemImage: "eye.fill")
                    .tag(Destino.ojo)
            }
            .navigationTitle("Nyx")
        } detail: {
            switch destino ?? .consejo {
            case .consejo: VistaConsejo(modelo: modelo)
            case .hablar: VistaHablar(modelo: modelo)
            case .ensenar: VistaEnsenar(modelo: modelo)
            case .lexico: VistaLexico(modelo: modelo)
            case .ojo: VistaOjo(modelo: modelo)
            }
        }
        .task {
            await modelo.arrancar()
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
struct MensajeChat: Identifiable {
    let id = UUID()
    let texto: String
    let esHumano: Bool
    let glosa: String?
    let comprension: Double?
    let autor: String      // "tú" o el rol de la mente ("critico", …)
    let destacada: Bool    // respuesta de mayor peso

    init(texto: String, esHumano: Bool, glosa: String? = nil, comprension: Double? = nil,
         autor: String = "", destacada: Bool = false) {
        self.texto = texto
        self.esHumano = esHumano
        self.glosa = glosa
        self.comprension = comprension
        self.autor = autor
        self.destacada = destacada
    }
}

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
    private let consejo = ConsejoResonante()

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

    func arrancar() async {
        guard !enMarcha else { return }

        await consejo.especializar()
        await consejo.arrancarCognicionEterna()
        Task.detached(priority: .background) { [consejo] in
            await consejo.runForever()
        }
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

    /// Ojo: muestra un cuadro en vivo a las mentes objetivo.
    /// Devuelve lo que cada una "vio".
    func mostrarOjo(cuadro: CuadroVisual, a objetivo: ObjetivoOjo) async -> [(rol: RolMental, vio: String)] {
        await consejo.mostrar(cuadro: cuadro, a: objetivo)
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

// ============================================================
//  OJO — visión resonante en vivo
// ============================================================
// Como un humano viendo un live en YouTube: un flujo de cuadros entra
// en tiempo real y las mentes lo resuenan. Tres fuentes:
//  - Demo en vivo: plasma fluido sintético (siempre funciona).
//  - Foto por URL: trae una imagen de la red cada 3 s.
//  - Cámara: intenta abrir la cámara del iPad (si Playgrounds la deja).
// El núcleo no clasifica nada: ver() resuena el cuadro con los semiones
// activos de cada mente. Sin CNN, sin etiquetas.

// ---------- Conversión imagen <-> CuadroVisual ----------
// El humano ve el live a resolución completa (foto); las mentes reciben
// el mapa 64×64 (cuadro). Toda fuente produce un FotogramaOjo con ambas.

/// Lo que viaja por el ojo: foto full-res para ti, mapa 64×64 para ellas.
struct FotogramaOjo {
    let foto: UIImage        // lo que TÚ ves (el live)
    let cuadro: CuadroVisual  // lo que ELLAS ven (el mapa)
}

/// Demo en vivo a 240×135: plasma fluido con domain warp (el espacio se
/// deforma a sí mismo: se ve líquido, orgánico, no mecánico). Paleta
/// cálida sobre fondo casi negro: brasa, ámbar, oro. Nada de azul.
/// Se genera a baja resolución y la vista la escala por GPU: a 30 fps
/// el costo queda en ~6% CPU en vez de ~25%.
func demoImagen(t: Double) -> UIImage? {
    let w = 240, h = 135
    var px = [UInt8](repeating: 0, count: w * h * 4)
    // Paleta cálida: negro cálido -> brasa -> ámbar -> oro pálido
    let stops: [(Double, Double, Double, Double)] = [
        (0.00, 14, 9, 12),
        (0.35, 110, 38, 22),
        (0.62, 215, 110, 40),
        (0.82, 245, 180, 100),
        (1.00, 255, 238, 200),
    ]
    func paleta(_ m: Double) -> (Double, Double, Double) {
        let x = min(1, max(0, m))
        var k = 0
        while k < stops.count - 2, x > stops[k + 1].0 { k += 1 }
        let (x0, r0, g0, b0) = stops[k]
        let (x1, r1, g1, b1) = stops[k + 1]
        let f = (x - x0) / max(0.0001, x1 - x0)
        return (r0 + (r1 - r0) * f, g0 + (g1 - g0) * f, b0 + (b1 - b0) * f)
    }
    var i = 0
    for y in 0 ..< h {
        let ny = Double(y) / Double(h) * 4.0
        for x in 0 ..< w {
            let nx = Double(x) / Double(w) * 4.0
            // q deforma el dominio: el flujo se arrastra a sí mismo
            let q = sin(nx * 2.0 + t) + sin(ny * 2.0 - t * 1.2)
            var v = sin(nx * 1.5 + q + t * 0.8) + sin(ny * 1.8 + q * 1.3 - t * 0.6)
            v = v / 4.0 + 0.5
            // detalle fino arrastrado por el flujo
            let d = sin(nx * 3.4 - t * 1.9 + v * 4.0) / 2.0 + 0.5
            let m = v * 0.7 + d * 0.3
            let (r, g, b) = paleta(m)
            px[i] = UInt8(r); px[i + 1] = UInt8(g); px[i + 2] = UInt8(b); px[i + 3] = 255
            i += 4
        }
    }
    let cs = CGColorSpaceCreateDeviceRGB()
    let cg: CGImage? = px.withUnsafeMutableBytes { buf in
        guard let ctx = CGContext(data: buf.baseAddress, width: w, height: h,
                                 bitsPerComponent: 8, bytesPerRow: w * 4, space: cs,
                                 bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue) else { return nil }
        return ctx.makeImage()
    }
    guard let imagen = cg else { return nil }
    return UIImage(cgImage: imagen)
}

/// UIImage -> mapa 64×64 en gris (lo que las mentes ven) + vector de flujo.
func cuadroDesdeImagen(_ img: UIImage, anterior: CuadroVisual?, fuente: String) -> CuadroVisual? {
    let l = CuadroVisual.lado
    guard let cg = img.cgImage else { return nil }
    let cs = CGColorSpaceCreateDeviceGray()
    var pix = [UInt8](repeating: 0, count: l * l)
    let ok: Bool = pix.withUnsafeMutableBytes { buf in
        guard let ctx = CGContext(data: buf.baseAddress, width: l, height: l,
                                 bitsPerComponent: 8, bytesPerRow: l, space: cs,
                                 bitmapInfo: CGImageAlphaInfo.none.rawValue) else { return false }
        ctx.interpolationQuality = .low
        ctx.draw(cg, in: CGRect(x: 0, y: 0, width: l, height: l))
        return true
    }
    guard ok else { return nil }
    let p = pix.map { Float($0) / 255 }
    let mov = anterior.map { CuadroVisual.movimiento(desde: $0, hasta: p) } ?? 0
    // Flujo: centroide actual − centroide anterior (fracción de cuadro).
    func centroide(_ q: [Float]) -> (x: Float, y: Float) {
        var sx: Float = 0, sy: Float = 0, m: Float = 0
        for y in 0 ..< l {
            for x in 0 ..< l {
                let v = q[y * l + x]
                sx += Float(x) * v; sy += Float(y) * v; m += v
            }
        }
        guard m > 0.001 else { return (0.5, 0.5) }
        return (sx / m / Float(l), sy / m / Float(l))
    }
    var c = CuadroVisual(pixeles: p, movimiento: mov, fuente: fuente, fecha: Date())
    if let a = anterior {
        // Flujo normalizado por dt: fracción de cuadro por segundo
        // (independiente de los fps a los que corra la fuente).
        let dt = max(0.01, Float(Date().timeIntervalSince(a.fecha)))
        let c0 = centroide(a.pixeles), c1 = centroide(p)
        c.flujoX = (c1.x - c0.x) / dt
        c.flujoY = (c1.y - c0.y) / dt
    }
    return c
}

/// CVPixelBuffer (cámara) -> UIImage a resolución completa (copia segura).
func imagenDesdePixelBuffer(_ pb: CVPixelBuffer) -> UIImage? {
    CVPixelBufferLockBaseAddress(pb, .readOnly)
    defer { CVPixelBufferUnlockBaseAddress(pb, .readOnly) }
    let w = CVPixelBufferGetWidth(pb), h = CVPixelBufferGetHeight(pb)
    let bpr = CVPixelBufferGetBytesPerRow(pb)
    guard w > 0, h > 0, let base = CVPixelBufferGetBaseAddress(pb) else { return nil }
    var copia = [UInt8](repeating: 0, count: h * bpr)
    copia.withUnsafeMutableBytes { dst in
        dst.baseAddress!.copyMemory(from: base, byteCount: h * bpr)
    }
    let cs = CGColorSpaceCreateDeviceRGB()
    let cg: CGImage? = copia.withUnsafeMutableBytes { buf in
        guard let ctx = CGContext(data: buf.baseAddress, width: w, height: h,
                                 bitsPerComponent: 8, bytesPerRow: bpr, space: cs,
                                 bitmapInfo: CGBitmapInfo(rawValue: CGImageAlphaInfo.premultipliedFirst.rawValue).rawValue) else { return nil }
        return ctx.makeImage()
    }
    guard let imagen = cg else { return nil }
    return UIImage(cgImage: imagen)
}

// ---------- Cámara (intento defensivo) ----------
// Playgrounds puede no dar acceso a la cámara: si falla, el estado lo dice
// y se usan las otras fuentes. Nada se rompe.

/// Flecha de 8 direcciones para el vector de flujo (y hacia abajo = pantalla).
/// El flujo va en fracción de cuadro por segundo: quieto bajo 0.02.
func flechaFlujo(x: Float, y: Float) -> String {
    let mag = (x * x + y * y).squareRoot()
    guard mag > 0.02 else { return "·" }
    let dirs = ["→", "↘", "↓", "↙", "←", "↖", "↑", "↗"]
    let a = atan2(Double(y), Double(x))
    let norm = (a + 2 * Double.pi).truncatingRemainder(dividingBy: 2 * Double.pi)
    let idx = Int((norm / (Double.pi / 4) + 0.5).truncatingRemainder(dividingBy: 8))
    return dirs[idx]
}

final class CapturaOjo: NSObject, ObservableObject {
    @Published var estado = "apagada"
    var alFotograma: ((FotogramaOjo) -> Void)?
    var alEstado: ((String) -> Void)?
    private var sesion: AVCaptureSession?
    private let cola = DispatchQueue(label: "ojo.camara")
    private var anterior: CuadroVisual?
    private var ultimo = Date.distantPast

    private func informar(_ s: String) {
        DispatchQueue.main.async { [weak self] in
            self?.estado = s
            self?.alEstado?(s)
        }
    }

    func iniciar() {
        detener()
        let s = AVCaptureSession()
        s.sessionPreset = .low
        do {
            guard let disp = AVCaptureDevice.default(.builtInWideAngleCamera, for: .video, position: .back) else {
                informar("sin cámara en este dispositivo")
                return
            }
            let ent = try AVCaptureDeviceInput(device: disp)
            guard s.canAddInput(ent) else { informar("no pude abrir la cámara"); return }
            s.addInput(ent)
            let out = AVCaptureVideoDataOutput()
            out.videoSettings = [kCVPixelBufferPixelFormatTypeKey as String: kCVPixelFormatType_32BGRA]
            out.setSampleBufferDelegate(self, queue: cola)
            guard s.canAddOutput(out) else { informar("no pude abrir la cámara"); return }
            s.addOutput(out)
        } catch {
            informar("no pude abrir la cámara")
            return
        }
        sesion = s
        switch AVCaptureDevice.authorizationStatus(for: .video) {
        case .authorized:
            s.startRunning()
            informar("en vivo")
        case .notDetermined:
            informar("pidiendo permiso…")
            AVCaptureDevice.requestAccess(for: .video) { [weak self, weak s] ok in
                if ok { s?.startRunning(); self?.informar("en vivo") }
                else { self?.informar("sin permiso de cámara") }
            }
        default:
            informar("sin permiso de cámara")
        }
    }

    func detener() {
        sesion?.stopRunning()
        sesion = nil
        informar("apagada")
    }
}

extension CapturaOjo: AVCaptureVideoDataOutputSampleBufferDelegate {
    func captureOutput(_ output: AVCaptureOutput, didOutput sampleBuffer: CMSampleBuffer, from connection: AVCaptureConnection) {
        guard Date().timeIntervalSince(ultimo) > 0.033,
              let pb = CMSampleBufferGetImageBuffer(sampleBuffer) else { return }
        ultimo = Date()
        guard let foto = imagenDesdePixelBuffer(pb),
              let cuadro = cuadroDesdeImagen(foto, anterior: anterior, fuente: "camara") else { return }
        anterior = cuadro
        let f = FotogramaOjo(foto: foto, cuadro: cuadro)
        DispatchQueue.main.async { [weak self] in self?.alFotograma?(f) }
    }
}

// ---------- Coordinador del ojo ----------

@MainActor
final class OjoResonante: ObservableObject {
    enum Fuente: String, CaseIterable, Hashable {
        case demo = "Demo en vivo"
        case url = "Foto por URL"
        case camara = "Cámara"
    }

    @Published var fuente: Fuente = .demo
    @Published var todasMiran = true
    @Published var rolObjetivo: RolMental = .percepcion
    @Published var transmitiendo = false
    @Published var vistaPrevia: UIImage?
    @Published var lecturas: [(rol: RolMental, vio: String)] = []
    @Published var estado = "El ojo está apagado."
    @Published var estadoCamara = "apagada"
    @Published var flujoX: Float = 0
    @Published var flujoY: Float = 0
    @Published var movVisto: Float = 0
    @Published var urlTexto = ""

    private var tarea: Task<Void, Never>?
    private var bandeja: FotogramaOjo?
    private var camara: CapturaOjo?
    private var ultimoPollURL = Date.distantPast

    var objetivo: ObjetivoOjo { todasMiran ? .todas : .una(rolObjetivo) }

    func conmutar(modelo: ModeloMente) {
        transmitiendo ? detener() : iniciar(modelo: modelo)
    }

    private func iniciar(modelo: ModeloMente) {
        transmitiendo = true
        estado = "Transmitiendo en vivo…"
        if fuente == .camara { arrancarCamara() }
        var t = 0.0
        var anterior: CuadroVisual?
        tarea = Task { [weak self] in
            guard let self else { return }
            while !Task.isCancelled {
                var nuevo: FotogramaOjo?
                switch self.fuente {
                case .demo:
                    t += 1.0 / 30.0
                    if let foto = demoImagen(t: t),
                       let cuadro = cuadroDesdeImagen(foto, anterior: anterior, fuente: "demo") {
                        nuevo = FotogramaOjo(foto: foto, cuadro: cuadro)
                    }
                case .url:
                    if Date().timeIntervalSince(self.ultimoPollURL) > 3 {
                        self.ultimoPollURL = Date()
                        nuevo = await self.descargar(anterior: anterior)
                    }
                case .camara:
                    nuevo = self.bandeja
                    self.bandeja = nil
                }
                if let f = nuevo {
                    anterior = f.cuadro
                    self.vistaPrevia = f.foto
                    self.flujoX = f.cuadro.flujoX
                    self.flujoY = f.cuadro.flujoY
                    self.movVisto = f.cuadro.movimiento
                    // Las mentes ven a 30 fps: ver() es barato (~0.2 ms)
                    // y el log solo anota cambios de escena.
                    self.lecturas = await modelo.mostrarOjo(cuadro: f.cuadro, a: self.objetivo)
                }
                try? await Task.sleep(nanoseconds: 33_000_000)  // 30 fps
            }
        }
    }

    func detener() {
        tarea?.cancel()
        tarea = nil
        camara?.detener()
        camara = nil
        transmitiendo = false
        estado = "El ojo está apagado."
    }

    private func arrancarCamara() {
        let c = CapturaOjo()
        c.alFotograma = { [weak self] f in
            Task { @MainActor [weak self] in self?.bandeja = f }
        }
        c.alEstado = { [weak self] s in
            Task { @MainActor [weak self] in self?.estadoCamara = s }
        }
        camara = c
        c.iniciar()
    }

    private func descargar(anterior: CuadroVisual?) async -> FotogramaOjo? {
        let txt = urlTexto.trimmingCharacters(in: .whitespacesAndNewlines)
        guard let url = URL(string: txt), !txt.isEmpty else {
            estado = "Pon una URL de imagen válida."
            return nil
        }
        do {
            let (data, _) = try await URLSession.shared.data(from: url)
            guard let foto = UIImage(data: data),
                  let cuadro = cuadroDesdeImagen(foto, anterior: anterior, fuente: "url") else {
                estado = "Esa URL no devolvió una imagen."
                return nil
            }
            estado = "Foto recibida — transmitiendo."
            return FotogramaOjo(foto: foto, cuadro: cuadro)
        } catch {
            estado = "Error de red al traer la foto."
            return nil
        }
    }
}

// ---------- Vista del ojo ----------

struct VistaOjo: View {
    @ObservedObject var modelo: ModeloMente
    @StateObject private var ojo = OjoResonante()

    var body: some View {
        ScrollView {
            VStack(spacing: 16) {
                ZStack {
                    RoundedRectangle(cornerRadius: 16).fill(Color(white: 0.08))
                    if let img = ojo.vistaPrevia {
                        Image(uiImage: img)
                            .resizable()
                            .scaledToFit()
                            .clipShape(RoundedRectangle(cornerRadius: 16))
                    } else {
                        Image(systemName: "eye.slash.fill")
                            .font(.largeTitle)
                            .foregroundColor(.gray)
                    }
                }
                .frame(height: 270)
                .overlay(alignment: .bottomLeading) {
                    Text(ojo.estado)
                        .font(.caption)
                        .padding(8)
                        .background(.ultraThinMaterial)
                        .clipShape(RoundedRectangle(cornerRadius: 8))
                        .padding(8)
                }

                if ojo.transmitiendo {
                    HStack(spacing: 8) {
                        Text("flujo")
                        Text(flechaFlujo(x: ojo.flujoX, y: ojo.flujoY))
                            .font(.title3)
                        Text(String(format: "mov %.2f", ojo.movVisto))
                    }
                    .font(.caption)
                    .foregroundColor(.gray)
                }

                Picker("Fuente", selection: $ojo.fuente) {
                    ForEach(OjoResonante.Fuente.allCases, id: \.self) { f in
                        Text(f.rawValue).tag(f)
                    }
                }
                .pickerStyle(.segmented)
                .disabled(ojo.transmitiendo)

                if ojo.fuente == .url {
                    TextField("https://…/foto.jpg", text: $ojo.urlTexto)
                        .textFieldStyle(.roundedBorder)
                        .autocorrectionDisabled()
                        .textInputAutocapitalization(.never)
                        .disabled(ojo.transmitiendo)
                }
                if ojo.fuente == .camara {
                    Text("cámara: \(ojo.estadoCamara)")
                        .font(.caption)
                        .foregroundColor(.secondary)
                    Text("Si Playgrounds no abre la cámara, usa Demo en vivo o Foto por URL.")
                        .font(.caption2)
                        .foregroundColor(.secondary)
                }

                Picker("¿Quién mira?", selection: $ojo.todasMiran) {
                    Text("Las 18").tag(true)
                    Text("Una sola").tag(false)
                }
                .pickerStyle(.segmented)

                if !ojo.todasMiran {
                    Picker("Mente", selection: $ojo.rolObjetivo) {
                        ForEach(RolMental.allCases, id: \.self) { r in
                            Label(r.rawValue, systemImage: r.icono).tag(r)
                        }
                    }
                }

                Button(ojo.transmitiendo ? "Dejar de transmitir" : "Transmitir en vivo") {
                    ojo.conmutar(modelo: modelo)
                }
                .buttonStyle(.borderedProminent)
                .tint(ojo.transmitiendo ? .red : .green)

                VStack(alignment: .leading, spacing: 8) {
                    Text("Qué ven").font(.headline)
                    if ojo.lecturas.isEmpty {
                        Text("Todavía nadie ha visto nada.")
                            .font(.caption)
                            .foregroundColor(.secondary)
                    }
                    ForEach(ojo.lecturas.indices, id: \.self) { i in
                        let l = ojo.lecturas[i]
                        HStack {
                            Image(systemName: l.rol.icono)
                                .foregroundColor(l.rol.color)
                            Text(l.rol.rawValue)
                                .font(.subheadline)
                            Spacer()
                            Text("veo: \(l.vio)")
                                .font(.subheadline)
                                .foregroundColor(.secondary)
                        }
                    }
                }
                .frame(maxWidth: .infinity, alignment: .leading)
            }
            .padding()
        }
        .navigationTitle("Ojo")
    }
}
