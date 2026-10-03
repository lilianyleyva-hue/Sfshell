#if canImport(SwiftUI)
import SwiftUI

// VistaAsamblea.swift — el chat de los tres consejos (y tú), con misiones.

struct PantallaAsamblea: View {
    @ObservedObject var nyx: NyxModelo
    @State private var texto = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("🗣 Los tres consejos").font(.title2.bold())
            Text("El nuevo ✨, el antiguo 🏛 y el lógico ⚖️ hablan en Resh cuando tienen ganas, a su ritmo, y se callan cuando ya no tienen nada nuevo que decir. Escríbeles cuando quieras, o ponles una misión 🎯.")
                .font(.caption)
                .foregroundColor(.gray)
            ControlesAsamblea(nyx: nyx)
            Marcador(nyx: nyx)
            HStack {
                TextField("Escríbeles… (o «misión: 2, 4, 8, 16, ?»)", text: $texto)
                    .textFieldStyle(.roundedBorder)
                    .onSubmit { envia() }
                Button("Enviar") { envia() }
                    .buttonStyle(.borderedProminent)
            }
            Toggle("Traducir", isOn: $nyx.traducir)
            ListaAsamblea(nyx: nyx)
        }
        .padding()
    }

    func envia() {
        nyx.escribeAsamblea(texto)
        texto = ""
    }
}

struct ControlesAsamblea: View {
    @ObservedObject var nyx: NyxModelo

    var body: some View {
        HStack {
            Button(nyx.asambleaViva ? "⏹ Que se callen" : "▶️ Que hablen") { nyx.alternaAsamblea() }
                .buttonStyle(.borderedProminent)
            MenuMisiones(nyx: nyx)
            Text(nyx.estadoAsamblea).font(.caption).foregroundColor(.gray)
        }
    }
}

struct MenuMisiones: View {
    @ObservedObject var nyx: NyxModelo

    var body: some View {
        Menu("🎯 Misión") {
            Button("🎲 Al azar") { nyx.misionAsamblea(nil) }
            ForEach(TipoMision.allCases, id: \.rawValue) { (t: TipoMision) in
                Button(t.icono + " " + t.nombre) { nyx.misionAsamblea(t) }
            }
        }
        .buttonStyle(.bordered)
    }
}

struct Marcador: View {
    @ObservedObject var nyx: NyxModelo

    var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(nyx.marcador).font(.callout.bold())
            Text(nyx.resumenMisiones).font(.caption).foregroundColor(.gray)
        }
    }
}

struct ListaAsamblea: View {
    @ObservedObject var nyx: NyxModelo

    var body: some View {
        List {
            ForEach(nyx.lineasAsamblea.reversed()) { (l: LineaAsamblea) in
                FilaAsamblea(l: l, traducir: nyx.traducir)
            }
        }
        .listStyle(.plain)
    }
}

struct FilaAsamblea: View {
    let l: LineaAsamblea
    let traducir: Bool

    var color: Color {
        switch l.consejo {
        case 0: return .blue
        case 1: return .orange
        case 2: return .green
        case 3: return .red
        default: return .primary
        }
    }

    var traduccion: String { traducir && !l.es.isEmpty ? "«" + l.es + "»" : "" }

    var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            HStack {
                Text(l.icono).font(.caption.bold()).foregroundColor(color)
                Text(l.quien).font(.caption).foregroundColor(.gray)
            }
            Text(l.consejo == 3 ? l.es : l.resh).font(.body)
            Text(l.consejo == 3 ? "" : traduccion).font(.caption).foregroundColor(.gray)
        }
    }
}
#endif
