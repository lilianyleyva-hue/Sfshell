#if canImport(SwiftUI)
import SwiftUI

// VistaAsamblea.swift — la pestaña donde los tres consejos hablan entre sí.

struct PantallaAsamblea: View {
    @ObservedObject var nyx: NyxModelo
    @State private var tema = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("🗣 Asamblea de los tres consejos").font(.title2.bold())
            Text("El nuevo ✨, el antiguo 🏛 y el lógico ⚖️ hablan por turnos en Resh. Cada uno contesta a lo que oyó: el lógico comprueba lo que dicen y enseña lo que deduce, y se preguntan las palabras que no entienden.")
                .font(.caption)
                .foregroundColor(.gray)
            ControlesAsamblea(nyx: nyx)
            HStack {
                TextField("Dales un tema (en español o en Resh)…", text: $tema)
                    .textFieldStyle(.roundedBorder)
                    .onSubmit { empieza() }
                Button("Empezar") { empieza() }
                    .buttonStyle(.bordered)
                    .disabled(nyx.puenteOcupado)
            }
            Toggle("Traducir", isOn: $nyx.traducir)
            ListaAsamblea(nyx: nyx)
        }
        .padding()
    }

    func empieza() {
        nyx.turnosAsamblea(6, tema: tema)
        tema = ""
    }
}

struct ControlesAsamblea: View {
    @ObservedObject var nyx: NyxModelo

    var body: some View {
        HStack {
            Button("1 turno") { nyx.turnosAsamblea(1) }
                .buttonStyle(.borderedProminent)
            Button("Una ronda (3)") { nyx.turnosAsamblea(3) }
                .buttonStyle(.bordered)
            Button("15 turnos") { nyx.turnosAsamblea(15) }
                .buttonStyle(.bordered)
            Text(nyx.puenteOcupado ? "hablando…" : "").font(.caption).foregroundColor(.gray)
        }
        .disabled(nyx.puenteOcupado)
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
            Text(l.resh).font(.body)
            Text(traduccion).font(.caption).foregroundColor(.gray)
        }
    }
}
#endif
