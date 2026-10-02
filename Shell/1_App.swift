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
    /// La última línea quedó a medias (un programa escribió sin '\n').
    var abierta = false

    init() {
        escribe(shell.motd())
        #if !APPLE_COMPLETO
        // Modo seguro: solo Foundation + esta pantalla. js/swift, say, sha256,
        // gzip y nc usan sus versiones de texto. Para activarlos, define
        // APPLE_COMPLETO (ver CAMBIOS.md).
        escribe("(modo seguro: sin motor JavaScript, voz, SHA256 ni nc — python sí funciona)\n")
        #endif
        // (Se saca 'self' a una constante antes del Task: Swift 5.10 no deja
        // usar la variable débil capturada dentro de código concurrente.)
        shell.uiClear = { [weak self] in
            guard let t = self else { return }
            Task { @MainActor in t.lineas.removeAll() }
        }
        shell.uiType = { [weak self] cmd in
            guard let t = self else { return }
            Task { @MainActor in t.entrada = cmd }
        }
        // salida en vivo de los programas (scanf, cin, Scanner…)
        let yo = self
        shell.consola = Consola(escribir: { texto in
            Task { @MainActor in yo.escribeVivo(texto) }
        })
        Task {
            let out = await shell.runProfile()
            escribe(out)
        }
    }

    /// Texto que llega mientras el programa corre: puede ser media línea.
    func escribeVivo(_ texto: String) {
        guard !texto.isEmpty else { return }
        var partes = texto.components(separatedBy: "\n")
        let terminaEnSalto = texto.hasSuffix("\n")
        if terminaEnSalto { partes.removeLast() }
        for (k, p) in partes.enumerated() {
            if k == 0 && abierta, let ultima = lineas.last {
                lineas[lineas.count - 1] = Linea(tipo: ultima.tipo, texto: ultima.texto + p)
            } else {
                lineas.append(Linea(tipo: .salida, texto: p))
            }
        }
        abierta = !terminaEnSalto
        if lineas.count > 2000 { lineas.removeFirst(lineas.count - 2000) }
    }

    /// ^C: detiene el programa que esté corriendo.
    func interrumpe() {
        shell.consola?.cancela()
    }

    /// ¿La línea que se escribe es para un programa que corre?
    var programaActivo: Bool { shell.consola?.activa ?? false }

    func escribe(_ out: String) {
        abierta = false
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
        if let c = shell.consola, c.activa {
            // entrada para el programa en marcha (se ve junto a su pregunta)
            escribeVivo(linea + "\n")
            c.entrega(linea)
            return
        }
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
            SalidaView(t: t)
            EntradaView(t: t)
        }
        .font(.system(size: 13, design: .monospaced))
        .background(Color.black.ignoresSafeArea())
    }
}

struct LineaView: View {
    let texto: String
    let color: Color

    var body: some View {
        Text(texto)
            .foregroundColor(color)
            .frame(maxWidth: .infinity, alignment: .leading)
            .textSelection(.enabled)
    }
}

struct SalidaView: View {
    @ObservedObject var t: Terminal

    var body: some View {
        ScrollViewReader { (proxy: ScrollViewProxy) in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 1) {
                    ForEach(t.lineas) { (l: Terminal.Linea) in
                        LineaView(texto: l.texto, color: SalidaView.color(l.tipo))
                    }
                    Color.clear.frame(height: 1).id("fin")
                }
                .padding(8)
            }
            .onChange(of: t.lineas.count) { (_: Int) in proxy.scrollTo("fin") }
        }
    }

    static func color(_ tipo: Terminal.Tipo) -> Color {
        switch tipo {
        case .orden: return .gray
        case .salida: return .white
        case .error: return .red
        }
    }
}

struct BotonCancelar: View {
    @ObservedObject var t: Terminal

    var color: Color {
        if t.ocupada { return .red }
        return .clear
    }

    var body: some View {
        Button("^C") { t.interrumpe() }
            .foregroundColor(color)
            .disabled(!t.ocupada)
            .keyboardShortcut("c", modifiers: .control)
    }
}

struct EntradaView: View {
    @ObservedObject var t: Terminal

    var prompt: String {
        if t.ocupada && t.programaActivo { return "› " }
        return t.shell.prompt
    }

    var body: some View {
        HStack(spacing: 0) {
            Text(prompt).foregroundColor(.green)
            TextField("", text: $t.entrada)
                .foregroundColor(.white)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .onSubmit { t.enviar() }
            BotonCancelar(t: t)
        }
        .padding(8)
    }
}
#else
/// Sin SwiftUI (Linux/Debian): la misma shell en la terminal del sistema.
@main
struct ShellTexto {
    static func main() async {
        let sh = Shell()
        sh.consola = Consola(escribir: { texto in
            print(texto, terminator: "")
            fflush(nil)
        }, lector: { readLine() })
        print(sh.motd(), terminator: "")
        print(await sh.runProfile(), terminator: "")
        while true {
            print(sh.prompt, terminator: "")
            fflush(nil)
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
