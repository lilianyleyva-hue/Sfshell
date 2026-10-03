#if canImport(AVFoundation) && canImport(Speech) && canImport(CoreTransferable) && canImport(UniformTypeIdentifiers) && canImport(UIKit)
import Foundation
import UIKit
import AVFoundation
import Speech
import CoreTransferable
import UniformTypeIdentifiers

// Video.swift — les mandas un video: miran varios momentos (con Vision) y
// escuchan lo que se dice (reconocimiento de voz de Apple).

/// Un video elegido en Fotos, copiado a un archivo temporal.
struct Pelicula: Transferable {
    let url: URL

    static var transferRepresentation: some TransferRepresentation {
        FileRepresentation(contentType: .movie) { (p: Pelicula) in
            SentTransferredFile(p.url)
        } importing: { (recibido: ReceivedTransferredFile) in
            let ext = recibido.file.pathExtension.isEmpty ? "mov" : recibido.file.pathExtension
            let destino = FileManager.default.temporaryDirectory.appendingPathComponent("nyx_video." + ext)
            try? FileManager.default.removeItem(at: destino)
            try FileManager.default.copyItem(at: recibido.file, to: destino)
            return Pelicula(url: destino)
        }
    }
}

enum Video {
    /// Mira 'n' momentos del video y escucha su audio.
    static func mira(_ url: URL, momentos n: Int = 5) async -> Percepcion {
        let asset = AVURLAsset(url: url)
        var p = Percepcion(sentido: .vista)
        p.origen = "el video"
        let duracion = (try? await asset.load(.duration)) ?? CMTime.zero
        let segundos = max(0.1, CMTimeGetSeconds(duracion))
        let generador = AVAssetImageGenerator(asset: asset)
        generador.appliesPreferredTrackTransform = true
        generador.maximumSize = CGSize(width: 640, height: 640)
        var conteo: [String: Int] = [:]
        var colores: [String: Int] = [:]
        for i in 0 ..< n {
            let t = CMTime(seconds: segundos * (Double(i) + 0.5) / Double(n), preferredTimescale: 600)
            guard let r = try? await generador.image(at: t) else { continue }
            let q = Ojos.mira(cg: r.image)
            for c in q.cosas {
                if conteo[c] == nil { p.cosas.append(c) }
                conteo[c, default: 0] += 1
            }
            for c in q.colores { colores[c, default: 0] += 1 }
            if let primera = q.cosas.first, p.secuencia.last != primera, p.secuencia.count < 4 {
                p.secuencia.append(primera)
            }
            p.personas = max(p.personas, q.personas)
            if p.texto.isEmpty { p.texto = q.texto }
        }
        p.cosas = Array(p.cosas.sorted { (conteo[$0] ?? 0) > (conteo[$1] ?? 0) }.prefix(6))
        p.colores = Array(colores.sorted { $0.value > $1.value }.prefix(3).map { $0.key })
        let oido = await escucha(url)
        if !oido.isEmpty { p.texto = oido.lowercased() }
        return p
    }

    /// Lo que se dice en el video (vacío si no hay voz o no hay permiso).
    static func escucha(_ url: URL) async -> String {
        let permitido: Bool = await withCheckedContinuation { (c: CheckedContinuation<Bool, Never>) in
            SFSpeechRecognizer.requestAuthorization { (e: SFSpeechRecognizerAuthorizationStatus) in
                c.resume(returning: e == .authorized)
            }
        }
        guard permitido, let rec = SFSpeechRecognizer(locale: Locale(identifier: "es-ES")), rec.isAvailable else { return "" }
        let pet = SFSpeechURLRecognitionRequest(url: url)
        pet.shouldReportPartialResults = false
        let unaVez = UnaVez()
        return await withCheckedContinuation { (c: CheckedContinuation<String, Never>) in
            let tarea = rec.recognitionTask(with: pet) { (r: SFSpeechRecognitionResult?, e: Error?) in
                if let r = r, r.isFinal {
                    if unaVez.primera() { c.resume(returning: r.bestTranscription.formattedString) }
                } else if e != nil {
                    if unaVez.primera() { c.resume(returning: "") }
                }
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + 25) {
                if unaVez.primera() {
                    tarea.cancel()
                    c.resume(returning: "")
                }
            }
        }
    }
}

/// Para responder una sola vez (aunque lleguen varias respuestas).
final class UnaVez {
    private let candado = NSLock()
    private var hecho = false

    func primera() -> Bool {
        candado.lock()
        defer { candado.unlock() }
        if hecho { return false }
        hecho = true
        return true
    }
}
#endif
