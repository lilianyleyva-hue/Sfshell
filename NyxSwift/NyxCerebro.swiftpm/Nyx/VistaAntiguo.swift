#if canImport(SwiftUI)
import SwiftUI

// VistaAntiguo.swift — la pestaña del consejo antiguo y del puente entre consejos.

struct PantallaAntiguo: View {
    @ObservedObject var nyx: NyxModelo
    @State private var pregunta = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("🏛 Consejo antiguo").font(.title2.bold())
            Text("El consejo original que hablaba Resh, tal como era, sin terminal. Conversa con el consejo nuevo: cada lado entiende solo el Resh que sabe, y se enseñan palabras.")
                .font(.caption)
                .foregroundColor(.gray)
            ControlesPuente(nyx: nyx)
            HStack {
                TextField("Pregúntale al consejo antiguo…", text: $pregunta)
                    .textFieldStyle(.roundedBorder)
                    .onSubmit { envia() }
                Button("Preguntar") { envia() }
                    .buttonStyle(.bordered)
                    .disabled(nyx.puenteOcupado)
            }
            Text(nyx.respuestaAntiguo).font(.callout)
            ListaPuente(nyx: nyx)
        }
        .padding()
    }

    func envia() {
        nyx.preguntaAntiguo(pregunta)
        pregunta = ""
    }
}

struct ControlesPuente: View {
    @ObservedObject var nyx: NyxModelo

    var body: some View {
        HStack {
            Button("Que hablen 1 turno") { nyx.turnosPuente(1) }
                .buttonStyle(.borderedProminent)
            Button("10 turnos") { nyx.turnosPuente(10) }
                .buttonStyle(.bordered)
            Text(nyx.puenteOcupado ? "hablando…" : "").font(.caption).foregroundColor(.gray)
        }
        .disabled(nyx.puenteOcupado)
    }
}

struct ListaPuente: View {
    @ObservedObject var nyx: NyxModelo

    var body: some View {
        List {
            ForEach(nyx.lineasPuente.reversed()) { (l: LineaPuente) in
                FilaPuente(l: l, traducir: nyx.traducir)
            }
        }
        .listStyle(.plain)
    }
}

struct FilaPuente: View {
    let l: LineaPuente
    let traducir: Bool

    var lado: String { l.antiguo ? "🏛 antiguo" : "✨ nuevo" }
    var traduccion: String { traducir && !l.es.isEmpty ? "«" + l.es + "»" : "" }

    var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            HStack {
                Text(lado).font(.caption.bold()).foregroundColor(l.antiguo ? .orange : .blue)
                Text(l.quien).font(.caption).foregroundColor(.gray)
            }
            Text(l.resh).font(.body)
            Text(traduccion).font(.caption).foregroundColor(.gray)
        }
    }
}
#endif
