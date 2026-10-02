#if canImport(SwiftUI)
import SwiftUI

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
#endif
