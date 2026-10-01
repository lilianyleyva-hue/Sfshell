import SwiftUI
import UIKit

// ============================================================
// MARK: - Escritorio estilo Ubuntu: dock lateral, panel superior,
//         Actividades y ventanas de verdad
// ============================================================
// Las apps son ventanas reales dentro de un ZStack — se arrastran,
// se minimizan, se maximizan y se cierran una por una, con varias
// abiertas a la vez. El dock vive a la izquierda (como el Launcher
// de Ubuntu), hay un panel arriba con reloj y batería reales, y el
// botón "Aplicaciones" abre una vista de Actividades al estilo
// GNOME/Ubuntu: no solo las 4 apps con ventana, sino un buscador que
// lanza CUALQUIER comando real del shell. La terminal sigue siendo
// la pantalla base: el escritorio es una capa encima.

/// Qué apps pueden abrirse como ventana flotante en el escritorio.
enum AppVentana: String, CaseIterable, Identifiable {
    case archivos, ajustes, navegador, monitor, nyx
    var id: String { rawValue }

    var titulo: String {
        switch self {
        case .archivos: return "Archivos"
        case .ajustes: return "Ajustes"
        case .navegador: return "Navegador"
        case .monitor: return "Monitor"
        case .nyx: return "Nyx"
        }
    }
    var icono: String {
        switch self {
        case .archivos: return "folder.fill"
        case .ajustes: return "gearshape.fill"
        case .navegador: return "globe"
        case .monitor: return "chart.bar.fill"
        case .nyx: return "sparkles"
        }
    }
    var color: Color {
        switch self {
        case .archivos: return .blue
        case .ajustes: return .gray
        case .navegador: return .teal
        case .monitor: return .orange
        case .nyx: return .purple
        }
    }
}

struct VentanaAbierta: Identifiable {
    let id = UUID()
    let app: AppVentana
    let posInicial: CGPoint
    var minimizada = false
}

/// Qué ventanas hay abiertas, en qué orden y cuáles están minimizadas.
/// El orden del array ES el orden de apilado: la última se dibuja encima.
@MainActor
final class DesktopManager: ObservableObject {
    @Published var ventanas: [VentanaAbierta] = []
    private var siguientePos = CGPoint(x: 220, y: 260)

    func abre(_ app: AppVentana) {
        if let i = ventanas.firstIndex(where: { $0.app == app }) {
            var v = ventanas.remove(at: i)
            v.minimizada = false
            ventanas.append(v)
            return
        }
        ventanas.append(VentanaAbierta(app: app, posInicial: siguientePos))
        siguientePos.x += 26
        siguientePos.y += 26
        if siguientePos.x > 380 { siguientePos = CGPoint(x: 220, y: 260) }
    }

    func cierra(_ id: UUID) {
        ventanas.removeAll { $0.id == id }
    }

    func alFrente(_ id: UUID) {
        guard let i = ventanas.firstIndex(where: { $0.id == id }), !ventanas[i].minimizada else { return }
        let v = ventanas.remove(at: i)
        ventanas.append(v)
    }

    func minimiza(_ id: UUID) {
        guard let i = ventanas.firstIndex(where: { $0.id == id }) else { return }
        ventanas[i].minimizada = true
    }

    /// Restaura una ventana minimizada y la trae al frente (esto es lo que
    /// hace tocar su icono en el dock, tanto si está minimizada como si no).
    func alternar(_ app: AppVentana) {
        if let i = ventanas.firstIndex(where: { $0.app == app }) {
            var v = ventanas.remove(at: i)
            v.minimizada = false
            ventanas.append(v)
        } else {
            abre(app)
        }
    }

    func estaAbierta(_ app: AppVentana) -> Bool {
        ventanas.contains { $0.app == app }
    }
}

/// Una ventana flotante: barra de título arrastrable + minimizar/maximizar/
/// cerrar + el contenido que le pasen. El contenido no sabe nada de que está
/// "en una ventana" — es la misma vista que se usaría en un .sheet.
private struct FloatingWindow<Content: View>: View {
    let titulo: String
    let icono: String
    var tamañoBase = CGSize(width: 340, height: 440)
    /// Tamaño disponible del escritorio — al maximizar, la ventana ocupa
    /// esto entero (menos un margen mínimo), como maximizar una ventana
    /// de verdad en Ubuntu, no solo "un poco más grande".
    var pantalla = CGSize(width: 400, height: 700)
    let onClose: () -> Void
    let onFront: () -> Void
    let onMinimize: () -> Void
    let contenido: () -> Content

    @State private var pos: CGPoint
    @State private var maximizada = false
    @State private var posAntesDeMaximizar: CGPoint?
    @GestureState private var arrastre: CGSize = .zero

    init(titulo: String, icono: String, posInicial: CGPoint, pantalla: CGSize,
         onClose: @escaping () -> Void, onFront: @escaping () -> Void, onMinimize: @escaping () -> Void,
         @ViewBuilder contenido: @escaping () -> Content) {
        self.titulo = titulo
        self.icono = icono
        self.pantalla = pantalla
        self.onClose = onClose
        self.onFront = onFront
        self.onMinimize = onMinimize
        self.contenido = contenido
        _pos = State(initialValue: posInicial)
    }

    private var tamaño: CGSize {
        guard maximizada else { return tamañoBase }
        return CGSize(width: max(tamañoBase.width, pantalla.width - 16),
                      height: max(tamañoBase.height, pantalla.height - 16))
    }

    var body: some View {
        VStack(spacing: 0) {
            barra
            contenido()
                .frame(width: tamaño.width, height: tamaño.height - 34)
                .clipped()
        }
        .frame(width: tamaño.width)
        .background(.regularMaterial)
        .clipShape(RoundedRectangle(cornerRadius: maximizada ? 0 : 14))
        .shadow(color: .black.opacity(0.35), radius: 14, y: 6)
        .position(x: pos.x + arrastre.width, y: pos.y + arrastre.height)
        .gesture(arrastreVentana)
        .onTapGesture(perform: onFront)
        .animation(.easeInOut(duration: 0.18), value: maximizada)
    }

    private var arrastreVentana: some Gesture {
        DragGesture()
            .updating($arrastre) { valor, estado, _ in estado = valor.translation }
            .onChanged { _ in onFront() }
            .onEnded { valor in
                pos.x += valor.translation.width
                pos.y += valor.translation.height
            }
    }

    /// Maximizar guarda dónde estaba la ventana para poder devolverla ahí
    /// al restaurar — igual que en un escritorio de verdad.
    private func alternarMaximizar() {
        if maximizada {
            if let p = posAntesDeMaximizar { pos = p }
            maximizada = false
        } else {
            posAntesDeMaximizar = pos
            pos = CGPoint(x: pantalla.width / 2, y: pantalla.height / 2)
            maximizada = true
        }
    }

    @ViewBuilder private var barra: some View {
        HStack(spacing: 10) {
            Image(systemName: icono).font(.caption2)
            Text(titulo).font(.caption).bold()
            Spacer()
            Button(action: alternarMaximizar) {
                Image(systemName: maximizada ? "arrow.down.right.and.arrow.up.left" : "arrow.up.left.and.arrow.down.right")
            }
            Button(action: onMinimize) {
                Image(systemName: "minus.circle.fill")
            }
            Button(action: onClose) {
                Image(systemName: "xmark.circle.fill")
            }
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 8)
        .background(Color.black.opacity(0.28))
    }
}

/// Icono del dock lateral (estilo Launcher de Ubuntu): un punto a la
/// izquierda marca qué apps tienen una ventana abierta ahora mismo.
private struct DockIcon: View {
    let nombre: String
    let icono: String
    let color: Color
    var activo = false
    let onTap: () -> Void

    var body: some View {
        Button(action: onTap) {
            HStack(spacing: 0) {
                Circle()
                    .fill(activo ? Color.white : Color.clear)
                    .frame(width: 4, height: 4)
                    .padding(.trailing, 3)
                ZStack {
                    RoundedRectangle(cornerRadius: 12)
                        .fill(color)
                        .frame(width: 46, height: 46)
                    Image(systemName: icono)
                        .font(.system(size: 19))
                        .foregroundStyle(.white)
                }
            }
        }
        .buttonStyle(.plain)
        .accessibilityLabel(nombre)
    }
}

struct DesktopView: View {
    @ObservedObject var model: TerminalModel
    @Environment(\.dismiss) private var dismiss
    @StateObject private var manager = DesktopManager()
    @State private var mostrarInfo = false
    @State private var mostrarActividades = false
    @State private var busquedaApp = ""
    @State private var nivelBateria: Float = -1

    @AppStorage("ui.theme") private var temaRaw: String = AppTheme.oscuro.rawValue
    @AppStorage("ui.accent") private var acentoRaw: String = AccentColorName.verde.rawValue

    private var tema: AppTheme { AppTheme(rawValue: temaRaw) ?? .oscuro }
    private var acento: AccentColorName { AccentColorName(rawValue: acentoRaw) ?? .verde }
    private var pal: Paleta { paleta(tema: tema, acento: acento) }

    var body: some View {
        GeometryReader { geo in
            ZStack {
                pal.fondo.ignoresSafeArea()
                if tema != .claro {
                    LinearGradient(colors: [Color(red: 0.145, green: 0.078, blue: 0.157),
                                             Color(red: 0.35, green: 0.14, blue: 0.11)],
                                    startPoint: .topLeading, endPoint: .bottomTrailing)
                        .opacity(0.5)
                        .ignoresSafeArea()
                        .allowsHitTesting(false)
                }
                HStack(spacing: 0) {
                    dockLateral
                    VStack(spacing: 0) {
                        topBar
                        Spacer()
                    }
                }
                ventanasAbiertas(pantalla: geo.size)
                actividades
            }
        }
        .preferredColorScheme(tema == .claro ? .light : .dark)
        .onAppear {
            UIDevice.current.isBatteryMonitoringEnabled = true
            nivelBateria = UIDevice.current.batteryLevel
        }
        .alert("SwiftShell", isPresented: $mostrarInfo) {
            Button("Vale", role: .cancel) {}
        } message: {
            Text("\(model.shell.commands.count) comandos instalados · \(model.shell.env.history.count) líneas en el historial")
        }
    }

    @ViewBuilder private func ventanasAbiertas(pantalla: CGSize) -> some View {
        ForEach(manager.ventanas.filter { !$0.minimizada }) { v in
            FloatingWindow(titulo: v.app.titulo, icono: v.app.icono, posInicial: v.posInicial, pantalla: pantalla,
                           onClose: { manager.cierra(v.id) },
                           onFront: { manager.alFrente(v.id) },
                           onMinimize: { manager.minimiza(v.id) }) {
                contenido(v)
            }
        }
    }

    @ViewBuilder private func contenido(_ v: VentanaAbierta) -> some View {
        switch v.app {
        case .archivos:
            FilesView(model: model,
                      onCerrar: { manager.cierra(v.id) },
                      onIrATerminal: { manager.cierra(v.id); dismiss() })
        case .ajustes:
            SettingsView(model: model, onCerrar: { manager.cierra(v.id) })
        case .navegador:
            BrowserView(onCerrar: { manager.cierra(v.id) })
        case .monitor:
            MonitorView(onCerrar: { manager.cierra(v.id) })
        case .nyx:
            NyxVista(memoria: model.memoriaNyx)
        }
    }

    // --- panel superior: reloj y batería de verdad, como el de Ubuntu ---

    @ViewBuilder private var topBar: some View {
        HStack {
            Button { mostrarActividades.toggle() } label: {
                Text("Actividades").font(.footnote.bold())
            }
            Spacer()
            TimelineView(.periodic(from: .now, by: 30)) { context in
                Text(horaTexto(context.date)).font(.footnote.bold())
            }
            Spacer()
            HStack(spacing: 12) {
                if nivelBateria >= 0 {
                    Label("\(Int(nivelBateria * 100))%", systemImage: iconoBateria)
                }
                Image(systemName: "wifi")
                Button { mostrarInfo = true } label: { Image(systemName: "info.circle") }
            }
            .font(.caption)
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 8)
        .background(Color.black.opacity(0.55))
        .foregroundStyle(.white)
    }

    private var iconoBateria: String {
        switch nivelBateria {
        case ..<0.15: return "battery.0"
        case ..<0.4: return "battery.25"
        case ..<0.65: return "battery.50"
        case ..<0.9: return "battery.75"
        default: return "battery.100"
        }
    }

    private func horaTexto(_ d: Date) -> String {
        let f = DateFormatter()
        f.dateFormat = "HH:mm · EEE d MMM"
        f.locale = Locale(identifier: "es_ES")
        return f.string(from: d)
    }

    // --- dock lateral: como el Launcher de Ubuntu, con "Mostrar aplicaciones" abajo ---

    @ViewBuilder private var dockLateral: some View {
        VStack(spacing: 14) {
            DockIcon(nombre: "Terminal", icono: "terminal.fill", color: .green, activo: true) { dismiss() }
            ForEach(AppVentana.allCases) { app in
                DockIcon(nombre: app.titulo, icono: app.icono, color: app.color,
                         activo: manager.estaAbierta(app)) { manager.alternar(app) }
            }
            DockIcon(nombre: "Adivina", icono: "questionmark.circle.fill", color: .purple) {
                dismiss(); model.runFromUI("adivina")
            }
            DockIcon(nombre: "git", icono: "arrow.triangle.branch", color: .indigo) {
                dismiss(); model.runFromUI("git status")
            }
            Spacer()
            DockIcon(nombre: "Aplicaciones", icono: "square.grid.3x3.fill", color: .gray.opacity(0.7),
                     activo: mostrarActividades) { mostrarActividades.toggle() }
        }
        .padding(.vertical, 18)
        .padding(.horizontal, 10)
        .frame(width: 72)
        .background(.ultraThinMaterial)
        .zIndex(1)
    }

    // --- Actividades: como el Activities Overview de GNOME/Ubuntu, pero con ---
    // --- un buscador que lanza CUALQUIER comando real del shell, no solo apps ---

    @ViewBuilder private var actividades: some View {
        if mostrarActividades {
            ZStack {
                Color.black.opacity(0.78).ignoresSafeArea()
                    .onTapGesture { mostrarActividades = false }
                VStack(spacing: 16) {
                    HStack {
                        Image(systemName: "magnifyingglass").foregroundStyle(.white.opacity(0.7))
                        TextField("Buscar aplicaciones y comandos", text: $busquedaApp)
                            .textInputAutocapitalization(.never)
                            .autocorrectionDisabled()
                            .foregroundStyle(.white)
                    }
                    .padding(10)
                    .background(Color.white.opacity(0.12))
                    .clipShape(RoundedRectangle(cornerRadius: 10))
                    .padding(.horizontal, 60)
                    .padding(.top, 36)

                    ScrollView {
                        LazyVGrid(columns: [GridItem(.adaptive(minimum: 84))], spacing: 22) {
                            ForEach(appsFiltradas) { app in
                                LauncherTile(nombre: app.titulo, icono: app.icono, color: app.color) {
                                    manager.alternar(app); mostrarActividades = false
                                }
                            }
                            ForEach(comandosFiltrados, id: \.self) { nombre in
                                LauncherTile(nombre: nombre, icono: "chevron.right.circle.fill", color: .gray.opacity(0.6)) {
                                    mostrarActividades = false
                                    dismiss()
                                    model.runFromUI(nombre)
                                }
                            }
                        }
                        .padding(28)
                    }
                }
            }
            .transition(.opacity)
            .zIndex(2)
        }
    }

    private var appsFiltradas: [AppVentana] {
        guard !busquedaApp.isEmpty else { return AppVentana.allCases }
        let q = busquedaApp.lowercased()
        return AppVentana.allCases.filter { $0.titulo.lowercased().contains(q) }
    }

    private var comandosFiltrados: [String] {
        let todos = model.shell.commands.keys.sorted()
        guard !busquedaApp.isEmpty else { return Array(todos.prefix(48)) }
        let q = busquedaApp.lowercased()
        return todos.filter { $0.contains(q) }
    }
}

/// Un cuadro del grid de Actividades: icono grande + nombre debajo.
private struct LauncherTile: View {
    let nombre: String
    let icono: String
    let color: Color
    let onTap: () -> Void

    var body: some View {
        Button(action: onTap) {
            VStack(spacing: 6) {
                ZStack {
                    RoundedRectangle(cornerRadius: 16).fill(color).frame(width: 60, height: 60)
                    Image(systemName: icono).font(.title2).foregroundStyle(.white)
                }
                Text(nombre).font(.caption2).foregroundStyle(.white).lineLimit(1)
            }
        }
        .buttonStyle(.plain)
    }
}
