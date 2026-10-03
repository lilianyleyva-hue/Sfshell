#if canImport(Vision) && canImport(UIKit)
import Foundation
import UIKit
import Vision

// Ojos.swift — las mentes ven fotos con Vision (viene con el iPad, sin internet):
// qué hay (objetos y escenas), texto escrito, caras y colores.

enum Ojos {
    /// Mira una foto y la convierte en una Percepcion.
    /// Es trabajo pesado: llamarla fuera del hilo principal.
    static func mira(_ imagen: UIImage) -> Percepcion {
        guard let cg = imagen.cgImage else { return Percepcion(sentido: .vista) }
        return mira(cg: cg)
    }

    /// Lo mismo con una imagen ya en CGImage (cámara, fotogramas de video).
    static func mira(cg: CGImage) -> Percepcion {
        var p = Percepcion(sentido: .vista)
        let clasifica = VNClassifyImageRequest()
        let lee = VNRecognizeTextRequest()
        lee.recognitionLevel = .accurate
        lee.recognitionLanguages = ["es-ES", "en-US"]
        lee.usesLanguageCorrection = true
        let caras = VNDetectFaceRectanglesRequest()
        let manejador = VNImageRequestHandler(cgImage: cg, options: [:])
        try? manejador.perform([clasifica, lee, caras])
        p.cosas = cosasDe(clasifica.results ?? [])
        p.texto = textoDe(lee.results ?? [])
        p.personas = caras.results?.count ?? 0
        let (cols, clara) = colores(cg)
        p.colores = cols
        p.rasgos = [clara ? "clara" : "oscura"]
        return p
    }

    private static func cosasDe(_ obs: [VNClassificationObservation]) -> [String] {
        var out: [String] = []
        for o in obs where o.confidence > 0.25 {
            let t = Etiquetas.traduce(o.identifier)
            if !out.contains(t) { out.append(t) }
            if out.count >= 6 { break }
        }
        return out
    }

    private static func textoDe(_ obs: [VNRecognizedTextObservation]) -> String {
        var lineas: [String] = []
        for o in obs {
            if let s = o.topCandidates(1).first?.string { lineas.append(s) }
            if lineas.count >= 4 { break }
        }
        return lineas.joined(separator: " ").lowercased()
    }

    /// Colores que más hay (y si la foto es clara u oscura).
    static func colores(_ cg: CGImage) -> ([String], Bool) {
        let w = 24
        let h = 24
        var px = [UInt8](repeating: 0, count: w * h * 4)
        let espacio = CGColorSpaceCreateDeviceRGB()
        let info: UInt32 = CGImageAlphaInfo.premultipliedLast.rawValue
        let dibujado: Bool = px.withUnsafeMutableBytes { (buf: UnsafeMutableRawBufferPointer) -> Bool in
            guard let ctx = CGContext(data: buf.baseAddress, width: w, height: h, bitsPerComponent: 8,
                                      bytesPerRow: w * 4, space: espacio, bitmapInfo: info) else { return false }
            ctx.draw(cg, in: CGRect(x: 0, y: 0, width: w, height: h))
            return true
        }
        if !dibujado { return ([], true) }
        var cuenta: [String: Int] = [:]
        var luz = 0.0
        for i in 0 ..< w * h {
            let r = Double(px[i * 4]) / 255
            let g = Double(px[i * 4 + 1]) / 255
            let b = Double(px[i * 4 + 2]) / 255
            luz += (r + g + b) / 3
            cuenta[Colores.nombre(r: r, g: g, b: b), default: 0] += 1
        }
        let minimo = w * h / 10
        let orden = cuenta.filter { $0.value >= minimo }.sorted { $0.value > $1.value }
        let nombres: [String] = orden.prefix(3).map { $0.key }
        return (nombres, luz / Double(w * h) > 0.5)
    }
}
#endif
