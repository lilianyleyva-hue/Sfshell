#if canImport(AVFoundation) && canImport(Speech) && canImport(CoreTransferable) && canImport(UniformTypeIdentifiers) && canImport(UIKit)
import Foundation
import UIKit
import AVFoundation
import Speech
import CoreTransferable
import UniformTypeIdentifiers

// Video.swift — les mandas un video y lo ven ENTERO.
//
// Miran un fotograma por segundo de principio a fin. Para no tardar, solo
// analizan a fondo (con Vision) cuando cambia la escena; si la imagen sigue
// igual, siguen en la misma escena. Escuchan todo el audio en el propio iPad
// (sin el límite de 1 minuto del reconocimiento por internet). Cada escena
// queda en su memoria con su momento: "al 1:20 hay perro playa · se oye …".

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

/// Un trozo del video donde la imagen se mantiene parecida.
struct Escena {
    var inicio: Double
    var fin: Double
    var cosas: [String]
    var texto: String
    var personas: Int
    var colores: [String]
    var dicho: String = ""          // lo que se oye durante la escena

    var momento: String { Video.reloj(inicio) }

    /// Cómo la recuerdan.
    var recuerdo: String {
        var partes: [String] = ["al \(momento) en el video"]
        if !cosas.isEmpty { partes.append("hay " + cosas.prefix(4).joined(separator: " ")) }
        if personas > 0 { partes.append(personas == 1 ? "una persona" : "\(personas) personas") }
        if !texto.isEmpty { partes.append("dice " + texto) }
        if !dicho.isEmpty { partes.append("se oye " + dicho) }
        return partes.joined(separator: " · ")
    }
}

struct VideoVisto {
    var percepcion: Percepcion
    var escenas: [Escena]
    var duracion: Double
    var fotogramas: Int
}

enum Video {
    static func reloj(_ s: Double) -> String {
        let t = Int(s.rounded())
        return "\(t / 60):" + String(format: "%02d", t % 60)
    }

    /// Ve el video entero. 'progreso' recibe de 0 a 1 (en el hilo principal).
    static func mira(_ url: URL, progreso: @escaping (Double, String) -> Void) async -> VideoVisto {
        let asset = AVURLAsset(url: url)
        let duracion = max(0.5, CMTimeGetSeconds((try? await asset.load(.duration)) ?? CMTime.zero))
        // un fotograma por segundo; en videos muy largos, como mucho 900
        let paso = max(1.0, duracion / 900)
        let generador = AVAssetImageGenerator(asset: asset)
        generador.appliesPreferredTrackTransform = true
        generador.maximumSize = CGSize(width: 480, height: 480)
        generador.requestedTimeToleranceBefore = CMTime(seconds: paso / 2, preferredTimescale: 600)
        generador.requestedTimeToleranceAfter = CMTime(seconds: paso / 2, preferredTimescale: 600)
        var escenas: [Escena] = []
        var huellaAnterior: [Float] = []
        var t = 0.0
        var vistos = 0
        while t < duracion {
            let momento = CMTime(seconds: t, preferredTimescale: 600)
            if let r = try? await generador.image(at: momento) {
                vistos += 1
                let huella = huellaDe(r.image)
                if escenas.isEmpty || diferencia(huella, huellaAnterior) > 0.12 {
                    let q = Ojos.mira(cg: r.image)       // escena nueva: se mira a fondo
                    escenas.append(Escena(inicio: t, fin: t + paso, cosas: q.cosas, texto: q.texto,
                                          personas: q.personas, colores: q.colores))
                    huellaAnterior = huella
                } else {
                    escenas[escenas.count - 1].fin = t + paso
                }
            }
            t += paso
            let hecho = min(1, t / duracion) * 0.7
            let texto = "mirando \(reloj(min(t, duracion))) de \(reloj(duracion)) · \(escenas.count) escenas"
            await MainActor.run { progreso(hecho, texto) }
        }
        await MainActor.run { progreso(0.7, "escuchando el audio…") }
        let segmentos = await escucha(url)
        for s in segmentos {
            if let k = escenas.lastIndex(where: { $0.inicio <= s.momento }) {
                escenas[k].dicho += (escenas[k].dicho.isEmpty ? "" : " ") + s.texto
            }
        }
        await MainActor.run { progreso(1, "listo") }
        return VideoVisto(percepcion: resumen(escenas), escenas: escenas, duracion: duracion, fotogramas: vistos)
    }

    /// Todo el video en una percepción: lo que más sale, el orden y lo dicho.
    static func resumen(_ escenas: [Escena]) -> Percepcion {
        var p = Percepcion(sentido: .vista)
        p.origen = "el video"
        var tiempo: [String: Double] = [:]
        var colores: [String: Double] = [:]
        for e in escenas {
            let dur = e.fin - e.inicio
            for c in e.cosas { tiempo[c, default: 0] += dur }
            for c in e.colores { colores[c, default: 0] += dur }
            p.personas = max(p.personas, e.personas)
            if let primera = e.cosas.first, p.secuencia.last != primera, p.secuencia.count < 8 {
                p.secuencia.append(primera)
            }
        }
        p.cosas = Array(tiempo.sorted { $0.value > $1.value }.prefix(6).map { $0.key })
        p.colores = Array(colores.sorted { $0.value > $1.value }.prefix(3).map { $0.key })
        let dicho = escenas.map { $0.dicho }.filter { !$0.isEmpty }.joined(separator: " ")
        p.texto = String(dicho.prefix(200)).lowercased()
        return p
    }

    /// Huella pequeña de la imagen (8×8 grises) para ver si cambió la escena.
    static func huellaDe(_ cg: CGImage) -> [Float] {
        let lado = 8
        var px = [UInt8](repeating: 0, count: lado * lado)
        let espacio = CGColorSpaceCreateDeviceGray()
        let dibujado: Bool = px.withUnsafeMutableBytes { (buf: UnsafeMutableRawBufferPointer) -> Bool in
            guard let ctx = CGContext(data: buf.baseAddress, width: lado, height: lado, bitsPerComponent: 8,
                                      bytesPerRow: lado, space: espacio, bitmapInfo: CGImageAlphaInfo.none.rawValue) else { return false }
            ctx.draw(cg, in: CGRect(x: 0, y: 0, width: lado, height: lado))
            return true
        }
        if !dibujado { return [] }
        return px.map { Float($0) / 255 }
    }

    static func diferencia(_ a: [Float], _ b: [Float]) -> Float {
        if a.count != b.count || a.isEmpty { return 1 }
        var d: Float = 0
        for i in 0 ..< a.count { d += abs(a[i] - b[i]) }
        return d / Float(a.count)
    }

    struct Segmento {
        let momento: Double
        let texto: String
    }

    /// Todo lo que se dice en el video, con su momento.
    static func escucha(_ url: URL) async -> [Segmento] {
        let permitido: Bool = await withCheckedContinuation { (c: CheckedContinuation<Bool, Never>) in
            SFSpeechRecognizer.requestAuthorization { (e: SFSpeechRecognizerAuthorizationStatus) in
                c.resume(returning: e == .authorized)
            }
        }
        guard permitido, let rec = SFSpeechRecognizer(locale: Locale(identifier: "es-ES")), rec.isAvailable else { return [] }
        let pet = SFSpeechURLRecognitionRequest(url: url)
        pet.shouldReportPartialResults = false
        // en el propio iPad: sin internet y sin el límite de un minuto
        if rec.supportsOnDeviceRecognition { pet.requiresOnDeviceRecognition = true }
        let unaVez = UnaVez()
        return await withCheckedContinuation { (c: CheckedContinuation<[Segmento], Never>) in
            let tarea = rec.recognitionTask(with: pet) { (r: SFSpeechRecognitionResult?, e: Error?) in
                if let r = r, r.isFinal {
                    if unaVez.primera() { c.resume(returning: frasesDe(r.bestTranscription)) }
                } else if e != nil {
                    if unaVez.primera() { c.resume(returning: []) }
                }
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + 600) {
                if unaVez.primera() {
                    tarea.cancel()
                    c.resume(returning: [])
                }
            }
        }
    }

    /// Junta las palabras reconocidas en trozos de ~10 segundos.
    static func frasesDe(_ t: SFTranscription) -> [Segmento] {
        var out: [Segmento] = []
        var actual: [String] = []
        var inicio = 0.0
        for s in t.segments {
            if actual.isEmpty { inicio = s.timestamp }
            actual.append(s.substring)
            if s.timestamp - inicio > 10 {
                out.append(Segmento(momento: inicio, texto: actual.joined(separator: " ")))
                actual = []
            }
        }
        if !actual.isEmpty { out.append(Segmento(momento: inicio, texto: actual.joined(separator: " "))) }
        return out
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
