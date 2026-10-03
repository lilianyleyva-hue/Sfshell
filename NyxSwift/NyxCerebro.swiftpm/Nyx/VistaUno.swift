#if canImport(SwiftUI)
import SwiftUI

// VistaUno.swift — la pestaña de Nyx Uno: una sola mente con lo mejor de todos.

struct PantallaUno: View {
    @ObservedObject var nyx: NyxModelo
    @State private var texto = ""
    @State private var verFiabilidad = false

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("🌌 Nyx — una sola mente").font(.title2.bold())
            Text("Razona como el consejo lógico, asocia como el nuevo, usa las estrategias de las misiones y recuerda lo que le confirmas. Comprueba sus respuestas, piensa más cuando duda y te dice «no lo sé» cuando no lo sabe. Corrígela con «no, es …».")
                .font(.caption)
                .foregroundColor(.gray)
            ControlesUno(nyx: nyx, verFiabilidad: $verFiabilidad)
            Text(nyx.estadoUno).font(.caption).foregroundColor(.gray)
            Text(nyx.avisoUno).font(.caption)
            FiabilidadUno(nyx: nyx, ver: verFiabilidad)
            HStack {
                TextField("Pregúntale, enséñale o ponle un reto…", text: $texto)
                    .textFieldStyle(.roundedBorder)
                    .onSubmit { envia() }
                Button("Enviar") { envia() }
                    .buttonStyle(.borderedProminent)
            }
            HStack {
                Toggle("Traducir", isOn: $nyx.traducir)
                Button("👍") { nyx.opinaUno(true) }.buttonStyle(.bordered)
                Button("👎") { nyx.opinaUno(false) }.buttonStyle(.bordered)
            }
            ListaUno(nyx: nyx)
        }
        .padding()
        .onAppear { nyx.refrescaUno() }
    }

    func envia() {
        nyx.hablaUno(texto)
        texto = ""
    }
}

struct ControlesUno: View {
    @ObservedObject var nyx: NyxModelo
    @Binding var verFiabilidad: Bool

    var body: some View {
        HStack {
            Button("🎓 Entrenar") { nyx.entrenaUno(21) }
                .buttonStyle(.bordered)
            Button("💤 Consolidar") { nyx.consolidaUno() }
                .buttonStyle(.bordered)
            Button("❓ Curiosidad") { nyx.curiosidadUno() }
                .buttonStyle(.bordered)
            Button(verFiabilidad ? "Ocultar en qué se fía" : "¿De quién se fía?") { verFiabilidad.toggle() }
                .buttonStyle(.bordered)
        }
    }
}

struct FiabilidadUno: View {
    @ObservedObject var nyx: NyxModelo
    let ver: Bool

    var lineas: [String] { ver ? nyx.fiabilidadesUno() : [] }

    var body: some View {
        VStack(alignment: .leading, spacing: 1) {
            ForEach(lineas, id: \.self) { (l: String) in
                Text(l).font(.caption2).foregroundColor(.gray)
            }
        }
    }
}

struct ListaUno: View {
    @ObservedObject var nyx: NyxModelo

    var body: some View {
        List {
            ForEach(nyx.chatUno.reversed()) { (m: MensajeUno) in
                FilaUno(m: m, traducir: nyx.traducir)
            }
        }
        .listStyle(.plain)
    }
}

struct FilaUno: View {
    let m: MensajeUno
    let traducir: Bool

    var principal: String { m.deHumano ? "🙂 " + m.es : "🌌 " + m.resh }
    var traduccion: String { !m.deHumano && traducir ? "«" + m.es + "»" : "" }

    var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(principal).font(.body)
            Text(traduccion).font(.callout).foregroundColor(.blue)
            Text(m.detalle).font(.caption).foregroundColor(.gray)
            Text(m.pasos).font(.caption2).foregroundColor(.gray)
        }
    }
}
#endif
