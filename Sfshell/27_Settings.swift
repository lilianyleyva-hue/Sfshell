import SwiftUI

// ============================================================
// MARK: - Ajustes de la interfaz
// ============================================================
// Se guardan con @AppStorage — UserDefaults de verdad — así que
// sobreviven a cerrar la app. Esto es aparte del comando 'defaults'
// de la terminal (que guarda claves tuyas dentro del shell): estos
// ajustes son de cómo SE VE y se comporta la app, no del sandbox.

enum AppTheme: String, CaseIterable, Identifiable {
    case oscuro, claro, matrix
    var id: String { rawValue }
    var etiqueta: String {
        switch self {
        case .oscuro: return "Oscuro"
        case .claro: return "Claro"
        case .matrix: return "Matrix"
        }
    }
    var icono: String {
        switch self {
        case .oscuro: return "moon.fill"
        case .claro: return "sun.max.fill"
        case .matrix: return "terminal.fill"
        }
    }
}

enum AccentColorName: String, CaseIterable, Identifiable {
    case verde, azul, naranja, rosa, cian, blanco, morado, ubuntu
    var id: String { rawValue }
    var etiqueta: String { self == .ubuntu ? "Ubuntu" : rawValue.capitalized }
    var color: Color {
        switch self {
        case .verde: return .green
        case .azul: return .blue
        case .naranja: return .orange
        case .rosa: return .pink
        case .cian: return .cyan
        case .blanco: return .white
        case .morado: return .purple
        case .ubuntu: return Color(red: 0.914, green: 0.325, blue: 0.125)   // #E95420, el naranja de Ubuntu
        }
    }
}

struct Paleta {
    let fondo: Color
    let texto: Color
    let comando: Color
    let error: Color
    let nota: Color
}

/// Une un tema con un color de acento en una paleta concreta.
func paleta(tema: AppTheme, acento: AccentColorName) -> Paleta {
    switch tema {
    case .oscuro:
        return Paleta(fondo: Color.black.opacity(0.92), texto: .white,
                      comando: acento.color, error: .red, nota: .cyan)
    case .claro:
        return Paleta(fondo: Color(white: 0.97), texto: .black,
                      comando: acento.color, error: Color(red: 0.75, green: 0.1, blue: 0.1), nota: .blue)
    case .matrix:
        return Paleta(fondo: .black, texto: .green,
                      comando: .green, error: Color(red: 1, green: 0.35, blue: 0.35),
                      nota: Color.green.opacity(0.6))
    }
}

// ============================================================
// MARK: - Pantalla de ajustes
// ============================================================

struct SettingsView: View {
    @ObservedObject var model: TerminalModel
    @Environment(\.dismiss) private var dismiss
    /// Cuando esta vista vive dentro de una ventana del escritorio (no de
    /// un .sheet), 'dismiss()' no tiene nada que cerrar — el botón Listo
    /// llama a esto en su lugar, si se lo dan.
    var onCerrar: (() -> Void)? = nil

    @AppStorage("ui.fontSize") private var fontSize: Double = 13
    @AppStorage("ui.theme") private var temaRaw: String = AppTheme.oscuro.rawValue
    @AppStorage("ui.accent") private var acentoRaw: String = AccentColorName.verde.rawValue
    @AppStorage("ui.roundedFont") private var letraRedondeada: Bool = false
    @AppStorage("ui.haptics") private var haptics: Bool = true
    @AppStorage("ui.historyLimit") private var historyLimit: Double = 500
    @AppStorage("ui.startDesktop") private var startDesktop: Bool = true

    @State private var confirmarVaciado = false

    private var tema: AppTheme { AppTheme(rawValue: temaRaw) ?? .oscuro }
    private var acento: AccentColorName { AccentColorName(rawValue: acentoRaw) ?? .verde }
    private var pal: Paleta { paleta(tema: tema, acento: acento) }

    var body: some View {
        NavigationStack {
            Form {
                Section("Vista previa") {
                    VStack(alignment: .leading, spacing: 4) {
                        Text("\(model.shell.env.vpath(model.shell.env.cwd)) $ run demo.swift")
                            .foregroundStyle(pal.comando)
                        Text("hola mundo 1")
                            .foregroundStyle(pal.texto)
                        Text("swift: falta el archivo — mira 'support'")
                            .foregroundStyle(pal.error)
                        Text("[nota] los comandos van así")
                            .foregroundStyle(pal.nota)
                    }
                    .font(.system(size: fontSize, design: letraRedondeada ? .rounded : .monospaced))
                    .padding(10)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(pal.fondo)
                    .clipShape(RoundedRectangle(cornerRadius: 8))
                }
                .listRowInsets(EdgeInsets())
                .padding(4)

                Section("Apariencia") {
                    Picker("Tema", selection: $temaRaw) {
                        ForEach(AppTheme.allCases) { t in
                            Label(t.etiqueta, systemImage: t.icono).tag(t.rawValue)
                        }
                    }
                    Picker("Color de acento", selection: $acentoRaw) {
                        ForEach(AccentColorName.allCases) { a in
                            HStack {
                                Circle().fill(a.color).frame(width: 14, height: 14)
                                Text(a.etiqueta)
                            }
                            .tag(a.rawValue)
                        }
                    }
                    Toggle("Letra redondeada (en vez de monoespaciada)", isOn: $letraRedondeada)
                    VStack(alignment: .leading) {
                        HStack {
                            Text("Tamaño de letra")
                            Spacer()
                            Text("\(Int(fontSize))")
                                .foregroundStyle(.secondary)
                                .monospacedDigit()
                        }
                        Slider(value: $fontSize, in: 10...22, step: 1)
                    }
                }

                Section("Comportamiento") {
                    Toggle("Empezar en el escritorio", isOn: $startDesktop)
                    Toggle("Vibrar cuando un comando falla", isOn: $haptics)
                    VStack(alignment: .leading) {
                        HStack {
                            Text("Líneas de historial en pantalla")
                            Spacer()
                            Text("\(Int(historyLimit))")
                                .foregroundStyle(.secondary)
                                .monospacedDigit()
                        }
                        Slider(value: $historyLimit, in: 100...3000, step: 100)
                    }
                    Button("Vaciar historial de comandos", role: .destructive) {
                        confirmarVaciado = true
                    }
                    .confirmationDialog("¿Vaciar todo el historial?", isPresented: $confirmarVaciado, titleVisibility: .visible) {
                        Button("Vaciar", role: .destructive) {
                            model.shell.env.history.removeAll()
                            model.shell.saveHistory()
                        }
                        Button("Cancelar", role: .cancel) {}
                    }
                }

                Section("Sistema") {
                    LabeledContent("Comandos disponibles", value: "\(model.shell.commands.count)")
                    LabeledContent("Directorio actual", value: model.shell.env.vpath(model.shell.env.cwd))
                    LabeledContent("Modo del intérprete", value: model.shell.mode.rawValue)
                    LabeledContent("Historial guardado", value: "\(model.shell.env.history.count) líneas")
                }

                Section {
                    Button("Restablecer estos ajustes") {
                        temaRaw = AppTheme.oscuro.rawValue
                        acentoRaw = AccentColorName.verde.rawValue
                        letraRedondeada = false
                        fontSize = 13
                        haptics = true
                        historyLimit = 500
                        startDesktop = true
                    }
                } footer: {
                    Text("Esto solo cambia cómo se ve la app. El comando 'defaults' guarda claves\ntuyas por separado, dentro del propio shell.")
                }
            }
            .navigationTitle("Ajustes")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Listo") { onCerrar?() ?? dismiss() }
                }
            }
        }
    }
}
