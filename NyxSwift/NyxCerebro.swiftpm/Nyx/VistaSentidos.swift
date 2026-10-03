#if canImport(SwiftUI) && canImport(PhotosUI) && canImport(UIKit)
import Foundation
import UIKit
import SwiftUI
import PhotosUI

// VistaSentidos.swift — la pestaña Sentidos: les enseñas una foto o les hablas,
// y ves qué notó cada una de las 18.

struct PantallaSentidos: View {
    @ObservedObject var nyx: NyxModelo
    @StateObject private var oido = Oido()
    @StateObject private var camara = Camara()
    @State private var item: PhotosPickerItem? = nil
    @State private var foto: UIImage? = nil
    @State private var mirando = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 18) {
                SeccionCamara(nyx: nyx, camara: camara)
                SeccionVer(nyx: nyx, item: $item, foto: foto, mirando: mirando)
                SeccionVideo(nyx: nyx)
                SeccionOir(nyx: nyx, oido: oido)
                SeccionNotas(nyx: nyx)
            }
            .padding()
        }
        .onChange(of: item) { (nuevo: PhotosPickerItem?) in carga(nuevo) }
        .onAppear { conectaCamara() }
    }

    private func conectaCamara() {
        let n = nyx
        camara.alVer = { (p: Percepcion) in n.veCamara(p) }
    }

    private func carga(_ nuevo: PhotosPickerItem?) {
        guard let it = nuevo else { return }
        mirando = true
        Task {
            let datos = try? await it.loadTransferable(type: Data.self)
            guard let d = datos, let img = UIImage(data: d) else {
                await MainActor.run { mirando = false }
                return
            }
            let p = await Task.detached { Ojos.mira(img) }.value
            await MainActor.run {
                foto = img
                mirando = false
                nyx.percibe(p)
            }
        }
    }
}

struct SeccionVer: View {
    @ObservedObject var nyx: NyxModelo
    @Binding var item: PhotosPickerItem?
    let foto: UIImage?
    let mirando: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("👁 Ver").font(.title2.bold())
            PhotosPicker(selection: $item, matching: .images) {
                Label("Enséñales una foto", systemImage: "photo")
            }
            .buttonStyle(.borderedProminent)
            Image(uiImage: foto ?? UIImage())
                .resizable()
                .scaledToFit()
                .frame(maxHeight: 260)
                .cornerRadius(10)
            Text(mirando ? "mirando…" : nyx.ultimaVista).font(.callout).foregroundColor(.gray)
        }
    }
}

struct SeccionCamara: View {
    @ObservedObject var nyx: NyxModelo
    @ObservedObject var camara: Camara
    @State private var cada: Double = 3

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("📷 Cámara en vivo").font(.title2.bold())
            HStack {
                Button(camara.activa ? "■ Apagar cámara" : "📷 Encender cámara") { alterna() }
                    .buttonStyle(.borderedProminent)
                Button("↺ Cambiar de cámara") { camara.cambia() }
                    .buttonStyle(.bordered)
                Spacer()
                Text("miran cada \(Int(cada)) s").font(.caption).foregroundColor(.gray)
                Slider(value: $cada, in: 1 ... 10)
                    .frame(maxWidth: 140)
            }
            VistaPrevia(sesion: camara.sesion)
                .frame(height: 280)
                .cornerRadius(10)
            Text(camara.problema).font(.caption).foregroundColor(.red)
            Text(nyx.ultimaCamara).font(.callout).foregroundColor(.gray)
        }
        .onChange(of: cada) { (v: Double) in camara.cada = v }
    }

    func alterna() {
        if camara.activa {
            camara.apaga()
        } else {
            camara.enciende()
        }
    }
}

struct SeccionVideo: View {
    @ObservedObject var nyx: NyxModelo
    @State private var item: PhotosPickerItem? = nil
    @State private var viendo = false
    @State private var avance: Double = 0
    @State private var estado = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("🎬 Videos").font(.title2.bold())
            PhotosPicker(selection: $item, matching: .videos) {
                Label("Mándales un video", systemImage: "film")
            }
            .buttonStyle(.borderedProminent)
            ProgressView(value: avance)
            Text(viendo ? estado : nyx.ultimoVideo)
                .font(.callout)
                .foregroundColor(.gray)
            ForEach(nyx.lineasVideo, id: \.self) { (l: String) in
                Text(l).font(.caption)
            }
        }
        .onChange(of: item) { (nuevo: PhotosPickerItem?) in carga(nuevo) }
    }

    private func carga(_ nuevo: PhotosPickerItem?) {
        guard let it = nuevo else { return }
        viendo = true
        Task {
            let peli = try? await it.loadTransferable(type: Pelicula.self)
            guard let p = peli else {
                await MainActor.run {
                    viendo = false
                    nyx.ultimoVideo = "No pude abrir ese video."
                }
                return
            }
            let visto = await Video.mira(p.url) { (a: Double, e: String) in
                avance = a
                estado = e
            }
            try? FileManager.default.removeItem(at: p.url)
            await MainActor.run {
                viendo = false
                nyx.veVideo(visto.percepcion, escenas: visto.escenas.map { $0.recuerdo },
                            fotogramas: visto.fotogramas, duracion: visto.duracion)
            }
        }
    }
}

struct SeccionOir: View {
    @ObservedObject var nyx: NyxModelo
    @ObservedObject var oido: Oido

    var escuchado: String {
        oido.texto.isEmpty ? "(pulsa 🎤 y habla)" : "«" + oido.texto + "»"
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("👂 Oír").font(.title2.bold())
            HStack {
                Button(oido.escuchando ? "■ Parar" : "🎤 Escuchar") { alterna() }
                    .buttonStyle(.borderedProminent)
                ProgressView(value: oido.nivel)
                    .frame(maxWidth: 160)
                Button("Pregúntales lo que dije") { nyx.pregunta(oido.texto, a: -1) }
                    .buttonStyle(.bordered)
                    .disabled(oido.texto.isEmpty || oido.escuchando)
            }
            Text(escuchado)
            Text(oido.problema).font(.caption).foregroundColor(.red)
            Text(nyx.ultimoOido).font(.callout).foregroundColor(.gray)
            Text(nyx.ultimaRespuesta).font(.body)
        }
    }

    func alterna() {
        if oido.escuchando {
            nyx.percibe(oido.para())
        } else {
            oido.empieza()
        }
    }
}

struct SeccionNotas: View {
    @ObservedObject var nyx: NyxModelo

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Lo que notó cada una").font(.title2.bold())
            ForEach(nyx.notasSentidos) { (n: LoQueNoto) in
                FilaNota(n: n)
            }
        }
    }
}

struct FilaNota: View {
    let n: LoQueNoto

    var body: some View {
        HStack(alignment: .top) {
            Text(Roles.todos[n.id].nombre)
                .font(.caption.bold())
                .foregroundColor(colorDe(n.id))
                .frame(width: 90, alignment: .leading)
            Text(n.dijo).font(.callout)
        }
    }
}
#endif
