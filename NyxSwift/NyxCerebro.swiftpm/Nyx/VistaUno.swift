#if canImport(SwiftUI)
import Foundation
import SwiftUI
import PhotosUI
import UniformTypeIdentifiers

// VistaUno.swift — la pestaña de Nyx Uno: una sola mente con lo mejor de todos.

struct PantallaUno: View {
    @ObservedObject var nyx: NyxModelo
    @State private var texto = ""
    @State private var verFiabilidad = false
    @State private var verPensamiento = true

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("🌌 Nyx — una sola mente").font(.title2.bold())
            Text("Razona como el consejo lógico, asocia como el nuevo, usa las estrategias de las misiones y recuerda lo que le confirmas. Comprueba sus respuestas, piensa más cuando duda y te dice «no lo sé» cuando no lo sabe. Corrígela con «no, es …».")
                .font(.caption)
                .foregroundColor(.gray)
            ControlesUno(nyx: nyx, verFiabilidad: $verFiabilidad)
            AdjuntarUno(nyx: nyx)
            VStack(alignment: .leading, spacing: 2) {
                Text(nyx.estadoMemoria).font(.caption).foregroundColor(nyx.cargandoMemoria ? .orange : .gray)
                Text(nyx.estadoUno).font(.caption).foregroundColor(.gray)
                Text(nyx.avisoUno).font(.caption)
            }
            FiabilidadUno(nyx: nyx, ver: verFiabilidad)
            HStack {
                TextField("Pregúntale, enséñale, ponle un reto o escribe un tema y pulsa 🌐…", text: $texto)
                    .textFieldStyle(.roundedBorder)
                    .onSubmit { envia() }
                Button("Enviar") { envia() }
                    .buttonStyle(.borderedProminent)
                    .disabled(nyx.cargandoMemoria)
                Button("🌐") { busca() }
                    .buttonStyle(.bordered)
                    .disabled(nyx.buscandoInternet)
            }
            HStack {
                Toggle("Traducir", isOn: $nyx.traducir)
                Toggle("💭 Ver cómo piensa", isOn: $verPensamiento)
                Button("👍") { nyx.opinaUno(true) }.buttonStyle(.bordered)
                Button("👎") { nyx.opinaUno(false) }.buttonStyle(.bordered)
            }
            ListaUno(nyx: nyx, verPensamiento: verPensamiento)
        }
        .padding()
        .onAppear { nyx.refrescaUno() }
    }

    func envia() {
        nyx.hablaUno(texto)
        texto = ""
    }

    /// Lo que hay en la caja es el tema que busca en Wikipedia.
    func busca() {
        nyx.aprendeDeInternet(texto)
        texto = ""
    }
}

/// Mandarle cosas: fotos y videos (los que quieras), archivos, audios.
/// Los enlaces se pegan en la caja de texto.
struct AdjuntarUno: View {
    @ObservedObject var nyx: NyxModelo
    @State private var fotos: [PhotosPickerItem] = []
    @State private var eligiendo = false
    @State private var soloAudios = false

    var tipos: [UTType] { soloAudios ? [.audio] : [.item] }

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                PhotosPicker(selection: $fotos, maxSelectionCount: nil, matching: .any(of: [.images, .videos])) {
                    Text("🖼 Fotos y videos")
                }
                .buttonStyle(.bordered)
                Button("📄 Archivos") { soloAudios = false; eligiendo = true }
                    .buttonStyle(.bordered)
                Button("🎵 Audios") { soloAudios = true; eligiendo = true }
                    .buttonStyle(.bordered)
            }
            Text(nyx.adjuntando.isEmpty ? "Los enlaces (https://…) pégalos en la caja: los lee solos." : nyx.adjuntando)
                .font(.caption)
                .foregroundColor(.gray)
        }
        .onChange(of: fotos) { (_: [PhotosPickerItem], nuevas: [PhotosPickerItem]) in cargaFotos(nuevas) }
        .fileImporter(isPresented: $eligiendo, allowedContentTypes: tipos, allowsMultipleSelection: true) { (r: Result<[URL], Error>) in
            recibe(r)
        }
    }

    private func recibe(_ r: Result<[URL], Error>) {
        guard case .success(let urls) = r else { return }
        var copias: [(URL, String)] = []
        for u in urls {
            if let c = Lector.copiaDeArchivos(u) { copias.append((c, u.lastPathComponent)) }
        }
        nyx.adjunta(archivos: copias)
    }

    private func cargaFotos(_ items: [PhotosPickerItem]) {
        if items.isEmpty { return }
        let lista = items
        fotos = []
        Task {
            var copias: [(URL, String)] = []
            var n = 0
            for it in lista {
                n += 1
                if let peli = try? await it.loadTransferable(type: Pelicula.self) {
                    copias.append((peli.url, "video \(n)"))
                } else if let d = try? await it.loadTransferable(type: Data.self) {
                    let u = FileManager.default.temporaryDirectory.appendingPathComponent("nyx_foto_\(UUID().uuidString).jpg")
                    if (try? d.write(to: u)) != nil { copias.append((u, "foto \(n)")) }
                }
            }
            let listas = copias
            await MainActor.run { nyx.adjunta(archivos: listas) }
        }
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
            Button("🌈 Imaginar") { nyx.imaginaUno() }
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
    let verPensamiento: Bool

    var body: some View {
        List {
            ForEach(nyx.chatUno.reversed()) { (m: MensajeUno) in
                FilaUno(m: m, traducir: nyx.traducir, verPensamiento: verPensamiento)
            }
        }
        .listStyle(.plain)
    }
}

struct FilaUno: View {
    let m: MensajeUno
    let traducir: Bool
    let verPensamiento: Bool

    var pensado: String { verPensamiento && !m.pensamiento.isEmpty ? "💭 Pensando…\n" + m.pensamiento : "" }

    var principal: String { m.deHumano ? "🙂 " + m.es : "🌌 " + m.resh }
    var traduccion: String { !m.deHumano && traducir ? "«" + m.es + "»" : "" }

    var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(pensado).font(.caption).foregroundColor(.purple)
            Text(principal).font(.body)
            Text(traduccion).font(.callout).foregroundColor(.blue)
            Text(m.detalle).font(.caption).foregroundColor(.gray)
            Text(m.pasos).font(.caption2).foregroundColor(.gray)
        }
    }
}
#endif
