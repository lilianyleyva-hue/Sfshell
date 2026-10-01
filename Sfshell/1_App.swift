import Foundation
#if canImport(SwiftUI)
import SwiftUI
#endif

// ============================================================
// MARK: - Punto de entrada: solo la shell, solo texto
// ============================================================
// No hay ventanas, botones ni escritorio gráfico: una pantalla de
// texto con la salida y una línea para escribir. Todo se hace con
// comandos (las 18 IAs también: 'ia ayuda').

#if canImport(SwiftUI)
/// Lo que ve la terminal: líneas de texto y la shell del humano.
@MainActor
final class Terminal: ObservableObject {
    enum Tipo { case orden, salida, error }
    struct Linea: Identifiable {
        let id = UUID()
        let tipo: Tipo
        let texto: String
    }

    @Published var lineas: [Linea] = []
    @Published var entrada = ""
    @Published var ocupada = false
    let shell = Shell()

    init() {
        escribe("SwiftShell — solo texto. 'help' comandos · 'ia ayuda' las 18 IAs · 'nano archivo' editor\n")
        shell.uiClear = { [weak self] in Task { @MainActor in self?.lineas.removeAll() } }
        shell.uiType = { [weak self] cmd in Task { @MainActor in self?.entrada = cmd } }
        Task {
            let out = await shell.runProfile()
            escribe(out)
        }
    }

    func escribe(_ out: String) {
        guard !out.isEmpty else { return }
        var t = out
        if t.hasSuffix("\n") { t.removeLast() }
        for l in t.components(separatedBy: "\n") {
            if l.hasPrefix(Shell.errMark) {
                lineas.append(Linea(tipo: .error, texto: String(l.dropFirst())))
            } else {
                lineas.append(Linea(tipo: .salida, texto: l))
            }
        }
        if lineas.count > 2000 { lineas.removeFirst(lineas.count - 2000) }
    }

    func enviar() {
        let linea = entrada
        entrada = ""
        lineas.append(Linea(tipo: .orden, texto: shell.prompt + linea))
        if shell.editor == nil, shell.dentroDe == nil, !linea.trimmingCharacters(in: .whitespaces).isEmpty {
            shell.env.history.append(linea)
            shell.saveHistory()
        }
        ocupada = true
        Task {
            // pasa por Huella para que pueda VER lo que haces cuando le enseñas
            let out = shell.editor == nil && shell.dentroDe == nil
                ? await HuellaIA.ejecuta(linea, shell)
                : await shell.execute(linea)
            escribe(out)
            ocupada = false
        }
    }
}

@main
struct MyApp: App {
    var body: some Scene {
        WindowGroup {
            ContentView()
        }
    }
}

struct ContentView: View {
    @StateObject private var t = Terminal()

    var body: some View {
        VStack(spacing: 0) {
            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 1) {
                        ForEach(t.lineas) { l in
                            Text(l.texto)
                                .foregroundColor(color(l.tipo))
                                .frame(maxWidth: .infinity, alignment: .leading)
                                .textSelection(.enabled)
                        }
                        Color.clear.frame(height: 1).id("fin")
                    }
                    .padding(8)
                }
                .onChange(of: t.lineas.count) { _ in proxy.scrollTo("fin") }
            }
            HStack(spacing: 0) {
                Text(t.shell.prompt).foregroundColor(.green)
                TextField("", text: $t.entrada)
                    .foregroundColor(.white)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .onSubmit { t.enviar() }
            }
            .padding(8)
        }
        .font(.system(size: 13, design: .monospaced))
        .background(Color.black.ignoresSafeArea())
    }

    private func color(_ tipo: Terminal.Tipo) -> Color {
        switch tipo {
        case .orden: return .gray
        case .salida: return .white
        case .error: return .red
        }
    }
}
#else
/// Sin SwiftUI (Linux/Debian): la misma shell en la terminal del sistema.
@main
struct ShellTexto {
    static func main() async {
        let sh = Shell()
        print("SwiftShell — solo texto. 'help' comandos · 'ia ayuda' las 18 IAs · 'exit' para salir")
        print(await sh.runProfile(), terminator: "")
        while true {
            print(sh.prompt, terminator: "")
            fflush(stdout)
            guard let linea = readLine() else { break }
            if sh.editor == nil, sh.dentroDe == nil, ["exit", "quit"].contains(linea.trimmingCharacters(in: .whitespaces)) { break }
            let out = sh.editor == nil && sh.dentroDe == nil
                ? await HuellaIA.ejecuta(linea, sh)
                : await sh.execute(linea)
            print(out.replacingOccurrences(of: Shell.errMark, with: ""), terminator: out.isEmpty || out.hasSuffix("\n") ? "" : "\n")
        }
        await NyxNucleo.uno.guardar()
    }
}
#endif
