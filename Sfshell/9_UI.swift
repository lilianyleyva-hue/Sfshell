import SwiftUI
import UIKit
import UniformTypeIdentifiers

// ============================================================
// MARK: - Interfaz
// ============================================================

struct Line: Identifiable {
    enum Kind { case cmd, out, err, note }
    let id = UUID()
    let kind: Kind
    let text: String
}

struct ExportDoc: FileDocument {
    static var readableContentTypes: [UTType] { [.data] }
    var data: Data
    init(data: Data) { self.data = data }
    init(configuration: ReadConfiguration) throws {
        data = configuration.file.regularFileContents ?? Data()
    }
    func fileWrapper(configuration: WriteConfiguration) throws -> FileWrapper {
        FileWrapper(regularFileWithContents: data)
    }
}

struct DrawSession: Identifiable {
    let id = UUID()
    let texto: String
    let nombre: String
}

struct EditSession: Identifiable {
    let id = UUID()
    let url: URL
    var text: String
}

@MainActor
final class TerminalModel: ObservableObject {
    @Published var lines: [Line] = []
    @Published var input = ""
    @Published var busy = false
    @Published var showImporter = false
    @Published var pickFolders = false
    @Published var showFiles = false
    @Published var showCamera = false
    @Published var drawing: DrawSession?
    @Published var showExporter = false
    @Published var exportDoc = ExportDoc(data: Data())
    @Published var exportName = "archivo"
    @Published var editSession: EditSession?
    @Published var showSettings = false
    @Published var showDesktop = false
    @Published var showBrowser = false
    @Published var showNyx = false
    /// Ruta real de /var/nyx/memoria.json (calculada aquí y no dentro de
    /// la vista, para no cargar al compilador)
    var memoriaNyx: URL? { try? shell.env.resolve(NyxNucleo.ruta) }
    @Published var browserURL: String?
    private var historyIndex = 0
    private var currentTask: Task<Void, Never>?

    let shell = Shell()

    init() {
        // valores por omisión de los ajustes, para que @AppStorage y
        // UserDefaults.standard tengan algo sensato la primera vez.
        UserDefaults.standard.register(defaults: [
            "ui.fontSize": 13.0,
            "ui.theme": AppTheme.oscuro.rawValue,
            "ui.accent": AccentColorName.verde.rawValue,
            "ui.roundedFont": false,
            "ui.haptics": true,
            "ui.historyLimit": 500.0,
            "ui.startDesktop": true
        ])
        showDesktop = UserDefaults.standard.bool(forKey: "ui.startDesktop")

        lines = [
            Line(kind: .note, text: "SwiftShell — todo en Swift, dentro de Swift Playgrounds."),
            Line(kind: .note, text: "help: lista de comandos · nano <archivo>: editor · mode js|swift|json|xml: escribe código directo"),
            Line(kind: .note, text: "api <url>: llama a una API · run <archivo> o ./archivo: ejecuta · pick: importa archivos · ⚙️ ajustes")
        ]
        shell.uiClear = { [weak self] in
            Task { @MainActor in self?.lines.removeAll() }
        }
        shell.uiPick = { [weak self] in
            Task { @MainActor in
                self?.pickFolders = false
                self?.showImporter = true
            }
        }
        shell.uiPickFolder = { [weak self] in
            Task { @MainActor in
                self?.pickFolders = true
                self?.showImporter = true
            }
        }
        shell.uiSave = { [weak self] url in
            Task { @MainActor in
                guard let self else { return }
                self.exportDoc = ExportDoc(data: (try? Data(contentsOf: url)) ?? Data())
                self.exportName = url.lastPathComponent
                self.showExporter = true
            }
        }
        historyIndex = shell.env.history.count
        Task { [weak self] in
            guard let self else { return }
            let out = await self.shell.runProfile()
            await MainActor.run { self.emit(out) }
        }
        shell.uiFiles = { [weak self] in
            Task { @MainActor in self?.showFiles = true }
        }
        shell.uiType = { [weak self] cmd in
            Task { @MainActor in self?.input = cmd }
        }
        shell.uiCamera = { [weak self] in
            Task { @MainActor in self?.showCamera = true }
        }
        shell.uiDraw = { [weak self] texto, nombre in
            Task { @MainActor in self?.drawing = DrawSession(texto: texto, nombre: nombre) }
        }
        shell.uiEdit = { [weak self] url in
            Task { @MainActor in
                let text = (try? String(contentsOf: url, encoding: .utf8)) ?? ""
                self?.editSession = EditSession(url: url, text: text)
            }
        }
        shell.uiNyx = { [weak self] in
            Task { @MainActor in self?.showNyx = true }
        }
        shell.uiBrowser = { [weak self] url in
            Task { @MainActor in
                self?.browserURL = url
                self?.showBrowser = true
            }
        }
    }

    var prompt: String { shell.prompt }

    /// Reparte la salida en bloques normales y de error.
    func emit(_ out: String) {
        guard !out.isEmpty else { return }
        var normal = ""
        for raw in out.components(separatedBy: "\n") {
            if raw.hasPrefix(Shell.errMark) {
                if !normal.isEmpty { lines.append(Line(kind: .out, text: normal)); normal = "" }
                lines.append(Line(kind: .err, text: String(raw.dropFirst())))
            } else {
                normal += (normal.isEmpty ? "" : "\n") + raw
            }
        }
        let t = normal.hasSuffix("\n") ? String(normal.dropLast()) : normal
        if !t.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            lines.append(Line(kind: .out, text: t))
        }
        // recorta el historial visible en pantalla al límite de los ajustes,
        // para que una sesión larga no vaya arrastrando cada vez más memoria
        let limite = max(100, Int(UserDefaults.standard.double(forKey: "ui.historyLimit")))
        if lines.count > limite * 2 {
            lines.removeFirst(lines.count - limite * 2)
        }
    }

    func guardarFoto(_ imagen: UIImage?) {
        showCamera = false
        guard let img = imagen, let datos = img.jpegData(compressionQuality: 0.8) else {
            lines.append(Line(kind: .out, text: "cámara cancelada"))
            return
        }
        let nombre = "foto_\(Int(Date().timeIntervalSince1970)).jpg"
        guard let dir = try? shell.env.resolve("/fotos") else { return }
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let destino = dir.appendingPathComponent(nombre)
        do {
            try datos.write(to: destino)
            lines.append(Line(kind: .out, text: "guardada /fotos/\(nombre) — \(humanSize(datos.count))"))
        } catch {
            lines.append(Line(kind: .err, text: errText(error)))
        }
    }

    func cancel() {
        currentTask?.cancel()
        currentTask = nil
        busy = false
        lines.append(Line(kind: .err, text: "^C detenido"))
    }

    /// Vibra (si está activado en ajustes) cuando un comando falla.
    private func avisaSiFalló() {
        guard UserDefaults.standard.bool(forKey: "ui.haptics") else { return }
        guard shell.env.vars["?"] == "1" else { return }
        UINotificationFeedbackGenerator().notificationOccurred(.error)
    }

    func submit() {
        let line = input
        input = ""
        lines.append(Line(kind: .cmd, text: prompt + line))
        guard !line.trimmingCharacters(in: .whitespaces).isEmpty else { return }
        shell.env.history.append(line)
        historyIndex = shell.env.history.count
        shell.saveHistory()
        busy = true
        currentTask = Task {
            // pasa por Huella para que pueda VER lo que haces cuando le enseñas
            let out = await HuellaIA.ejecuta(line, shell)
            if Task.isCancelled { return }
            await MainActor.run {
                self.emit(out)
                self.avisaSiFalló()
                self.busy = false
                self.currentTask = nil
            }
        }
    }

    func historyBack() {
        guard !shell.env.history.isEmpty else { return }
        historyIndex = max(0, historyIndex - 1)
        input = shell.env.history[historyIndex]
    }

    func historyForward() {
        guard !shell.env.history.isEmpty else { return }
        historyIndex = min(shell.env.history.count, historyIndex + 1)
        input = historyIndex < shell.env.history.count ? shell.env.history[historyIndex] : ""
    }

    func completeInput() { input = shell.complete(input) }

    func handlePick(_ result: Result<[URL], Error>) {
        if pickFolders { importFolder(result) } else { importFiles(result) }
        pickFolders = false
    }

    func importFiles(_ result: Result<[URL], Error>) {
        switch result {
        case .success(let urls):
            var report: [String] = []
            for u in urls {
                let access = u.startAccessingSecurityScopedResource()
                defer { if access { u.stopAccessingSecurityScopedResource() } }
                let dest = shell.env.cwd.appendingPathComponent(u.lastPathComponent)
                do {
                    if FileManager.default.fileExists(atPath: dest.path) {
                        try FileManager.default.removeItem(at: dest)
                    }
                    try FileManager.default.copyItem(at: u, to: dest)
                    report.append("importado \(u.lastPathComponent)")
                } catch {
                    report.append("no se pudo importar \(u.lastPathComponent): \(errText(error))")
                }
            }
            lines.append(Line(kind: .out, text: report.joined(separator: "\n")))
        case .failure(let e):
            lines.append(Line(kind: .err, text: errText(e)))
        }
    }

    func importFolder(_ result: Result<[URL], Error>) {
        switch result {
        case .success(let urls):
            guard let src = urls.first else { return }
            let access = src.startAccessingSecurityScopedResource()
            defer { if access { src.stopAccessingSecurityScopedResource() } }
            let fm = FileManager.default
            let dest = shell.env.cwd.appendingPathComponent(src.lastPathComponent)
            var copied = 0
            var failed = 0
            do {
                try fm.createDirectory(at: dest, withIntermediateDirectories: true)
                let items = try fm.contentsOfDirectory(at: src, includingPropertiesForKeys: nil)
                for item in items where !item.hasDirectoryPath {
                    let target = dest.appendingPathComponent(item.lastPathComponent)
                    if fm.fileExists(atPath: target.path) { try? fm.removeItem(at: target) }
                    do { try fm.copyItem(at: item, to: target); copied += 1 }
                    catch { failed += 1 }
                }
            } catch {
                lines.append(Line(kind: .err, text: errText(error)))
                return
            }
            shell.env.cwd = dest
            var msg = "importados \(copied) archivos en \(shell.env.vpath(dest))"
            if failed > 0 { msg += " (\(failed) no se pudieron copiar)" }
            lines.append(Line(kind: .out, text: msg))
        case .failure(let e):
            lines.append(Line(kind: .err, text: errText(e)))
        }
    }

    @discardableResult
    func saveEdit(close: Bool = true) -> Bool {
        guard let s = editSession else { return false }
        do {
            try s.text.write(to: s.url, atomically: true, encoding: .utf8)
            lines.append(Line(kind: .out, text: "guardado \(shell.env.vpath(s.url))"))
        } catch {
            lines.append(Line(kind: .err, text: errText(error)))
            return false
        }
        if close { editSession = nil }
        return true
    }

    /// ^R en nano: guarda y ejecuta el archivo según su extensión.
    func runEdited() {
        guard let s = editSession else { return }
        guard saveEdit(close: true) else { return }
        let path = shell.env.vpath(s.url)
        lines.append(Line(kind: .cmd, text: prompt + "run " + path))
        busy = true
        Task {
            let out = await shell.execute("run " + path)
            await MainActor.run {
                self.emit(out)
                self.busy = false
            }
        }
    }
}

/// Una línea de salida en la terminal, con su menú contextual de copiar/reutilizar.
/// Sacada de ContentView.body porque un ForEach con un contextMenu condicional
/// dentro, inline en una vista ya grande, es justo el patrón que hace que el
/// comprobador de tipos de SwiftUI se atasque (o falle sin decir por qué).
private struct LineRow: View {
    let line: Line
    let font: Font
    let color: Color
    let promptPrefix: String
    let onReuse: (String) -> Void

    var body: some View {
        Text(line.text)
            .font(font)
            .foregroundStyle(color)
            .frame(maxWidth: .infinity, alignment: .leading)
            .textSelection(.enabled)
            .contextMenu { menu }
    }

    @ViewBuilder private var menu: some View {
        Button {
            UIPasteboard.general.string = line.text
        } label: {
            Label("Copiar", systemImage: "doc.on.doc")
        }
        if line.kind == .cmd {
            Button {
                var cmd = line.text
                if cmd.hasPrefix(promptPrefix) { cmd.removeFirst(promptPrefix.count) }
                onReuse(cmd)
            } label: {
                Label("Reutilizar", systemImage: "arrow.uturn.up")
            }
        }
    }
}

struct ContentView: View {
    @StateObject private var m = TerminalModel()
    @FocusState private var focused: Bool

    @AppStorage("ui.fontSize") private var fontSize: Double = 13
    @AppStorage("ui.theme") private var temaRaw: String = AppTheme.oscuro.rawValue
    @AppStorage("ui.accent") private var acentoRaw: String = AccentColorName.verde.rawValue
    @AppStorage("ui.roundedFont") private var letraRedondeada: Bool = false

    private var tema: AppTheme { AppTheme(rawValue: temaRaw) ?? .oscuro }
    private var acento: AccentColorName { AccentColorName(rawValue: acentoRaw) ?? .verde }
    private var pal: Paleta { paleta(tema: tema, acento: acento) }

    private var mono: Font {
        .system(size: fontSize, design: letraRedondeada ? .rounded : .monospaced)
    }

    // barra de estado: dónde estás, y un acceso rápido a ajustes
    @ViewBuilder private var statusBar: some View {
        HStack(spacing: 6) {
            Image(systemName: "chevron.right").font(.caption2)
            Text(m.shell.env.vpath(m.shell.env.cwd))
                .font(.system(size: 11, design: .monospaced))
                .lineLimit(1)
                .truncationMode(.head)
            Spacer()
            if m.busy {
                ProgressView().scaleEffect(0.5)
            }
            Button {
                m.showDesktop = true
            } label: {
                Image(systemName: "square.grid.2x2.fill")
            }
            .font(.caption)
            Button {
                m.showSettings = true
            } label: {
                Image(systemName: "gearshape.fill")
            }
            .font(.caption)
        }
        .foregroundStyle(pal.texto.opacity(0.7))
        .padding(.horizontal, 10)
        .padding(.vertical, 4)
        .background(pal.fondo.opacity(0.6))
    }

    @ViewBuilder private var promptRow: some View {
        HStack(spacing: 4) {
            Text(m.prompt).font(mono).foregroundStyle(pal.comando)
            TextField("", text: $m.input)
                .font(mono)
                .foregroundStyle(pal.texto)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .focused($focused)
                .onSubmit { m.submit(); focused = true }
            if !m.input.isEmpty {
                Button {
                    m.input = ""
                } label: {
                    Image(systemName: "xmark.circle.fill")
                }
                .foregroundStyle(pal.texto.opacity(0.4))
            }
            if m.busy { ProgressView().scaleEffect(0.6) }
        }
        .id("prompt")
    }

    @ToolbarContentBuilder private var keyboardToolbar: some ToolbarContent {
        ToolbarItemGroup(placement: .keyboard) {
            if m.busy { Button("detener") { m.cancel() } }
            Button("archivos") { m.showFiles = true }
            Button("tab") { m.completeInput() }
            Button("↑") { m.historyBack() }
            Button("↓") { m.historyForward() }
            ForEach(["/", "-", "|", "~", ">", "$"], id: \.self) { s in
                Button(s) { m.input += s }
            }
            Spacer()
            Button {
                m.showSettings = true
            } label: {
                Image(systemName: "gearshape")
            }
            Button("ocultar") { focused = false }
        }
    }

    var body: some View {
        VStack(spacing: 0) {
            statusBar

            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 2) {
                        ForEach(m.lines) { l in
                            LineRow(line: l, font: mono, color: color(l.kind), promptPrefix: m.prompt) { cmd in
                                m.input = cmd
                                focused = true
                            }
                        }
                        promptRow
                    }
                    .padding(10)
                }
                .onChange(of: m.lines.count) {
                    withAnimation { proxy.scrollTo("prompt", anchor: .bottom) }
                }
                .onChange(of: m.input) {
                    withAnimation { proxy.scrollTo("prompt", anchor: .bottom) }
                }
            }
        }
        .background(pal.fondo)
        .foregroundStyle(pal.texto)
        .preferredColorScheme(tema == .claro ? .light : .dark)
        .onAppear { focused = true }
        .toolbar { keyboardToolbar }
        .fileImporter(isPresented: $m.showImporter,
                      allowedContentTypes: m.pickFolders ? [.folder] : [.item],
                      allowsMultipleSelection: !m.pickFolders) { m.handlePick($0) }
        .background(
            Color.clear
                .frame(width: 1, height: 1)
                .fileExporter(isPresented: $m.showExporter,
                              document: m.exportDoc,
                              contentType: .data,
                              defaultFilename: m.exportName) { _ in }
        )
        .sheet(item: $m.editSession) { _ in
            NanoView(model: m)
        }
        .sheet(isPresented: $m.showFiles) {
            FilesView(model: m)
        }
        .sheet(isPresented: $m.showSettings) {
            SettingsView(model: m)
        }
        .fullScreenCover(isPresented: $m.showDesktop) {
            DesktopView(model: m)
        }
        .sheet(isPresented: $m.showNyx) {
            NyxVista(memoria: m.memoriaNyx)
        }
        .sheet(isPresented: $m.showBrowser) {
            BrowserView(urlInicial: m.browserURL)
        }
        .sheet(item: $m.drawing) { d in
            DrawView(texto: d.texto, nombre: d.nombre) { m.drawing = nil }
        }
        .fullScreenCover(isPresented: $m.showCamera) {
            CamaraView { imagen in
                m.guardarFoto(imagen)
            }
            .ignoresSafeArea()
        }
    }

    private func color(_ k: Line.Kind) -> Color {
        switch k {
        case .cmd: return pal.comando
        case .out: return pal.texto
        case .err: return pal.error
        case .note: return pal.nota
        }
    }
}

// ============================================================
// MARK: - nano: editor de pantalla completa
// ============================================================

struct NanoView: View {
    @ObservedObject var model: TerminalModel
    @FocusState private var focused: Bool
    @AppStorage("ui.fontSize") private var fontSize: Double = 13
    @AppStorage("ui.roundedFont") private var letraRedondeada: Bool = false

    private var text: Binding<String> {
        Binding(get: { model.editSession?.text ?? "" },
                set: { model.editSession?.text = $0 })
    }

    private var name: String { model.editSession?.url.lastPathComponent ?? "" }

    private var stats: String {
        let t = text.wrappedValue
        let lines = t.isEmpty ? 0 : t.components(separatedBy: "\n").count
        return "\(lines) líneas · \(t.count) caracteres"
    }

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Text("nano").bold()
                Text(name)
                Spacer()
                Text(stats).foregroundStyle(.secondary)
            }
            .font(.system(size: 12, design: .monospaced))
            .padding(.horizontal, 10)
            .padding(.vertical, 6)
            .background(Color.gray.opacity(0.25))

            TextEditor(text: text)
                .font(.system(size: fontSize + 1, design: letraRedondeada ? .rounded : .monospaced))
                .autocorrectionDisabled()
                .textInputAutocapitalization(.never)
                .focused($focused)
                .padding(.horizontal, 4)

            HStack(spacing: 16) {
                Button("^O Guardar") { model.saveEdit(close: false) }
                Button("^R Ejecutar") { model.runEdited() }
                Spacer()
                Button("^X Salir") {
                    model.saveEdit(close: true)
                }
                Button("Descartar") { model.editSession = nil }
                    .foregroundStyle(.red)
            }
            .font(.system(size: 12, design: .monospaced))
            .padding(.horizontal, 10)
            .padding(.vertical, 8)
            .background(Color.gray.opacity(0.25))
        }
        .onAppear { focused = true }
        .toolbar {
            ToolbarItemGroup(placement: .keyboard) {
                ForEach(["\t", "{", "}", "(", ")", "[", "]", "\"", "<", ">", "/"], id: \.self) { s in
                    Button(s == "\t" ? "tab" : s) { model.editSession?.text += s }
                }
                Spacer()
                Button("ocultar") { focused = false }
            }
        }
    }
}
