#if canImport(AVFoundation) && canImport(UIKit) && canImport(SwiftUI) && canImport(CoreImage)
import Foundation
import UIKit
import SwiftUI
import AVFoundation
import CoreImage

// Camara.swift — las mentes ven el mundo por tu cámara.
// La enciendes tú; cada pocos segundos miran lo que hay delante (con Vision).

final class Camara: NSObject, ObservableObject, AVCaptureVideoDataOutputSampleBufferDelegate {
    @Published var activa = false
    @Published var trasera = true
    @Published var problema = ""
    let sesion = AVCaptureSession()
    /// Segundos entre una mirada y la siguiente.
    var cada: Double = 3
    /// Qué hacer con lo que ven (se llama en el hilo principal).
    var alVer: ((Percepcion) -> Void)?

    private let cola = DispatchQueue(label: "nyx.camara")
    private let contextoCI = CIContext()
    private let candado = NSLock()
    private var ultima = Date.distantPast
    private var mirando = false

    func enciende() {
        problema = ""
        AVCaptureDevice.requestAccess(for: .video) { (ok: Bool) in
            DispatchQueue.main.async {
                if ok {
                    self.configura()
                } else {
                    self.problema = "Sin permiso para la cámara (míralo en Ajustes)."
                }
            }
        }
    }

    func apaga() {
        let s = sesion
        cola.async { s.stopRunning() }
        activa = false
    }

    func cambia() {
        trasera.toggle()
        if activa { configura() }
    }

    private func configura() {
        sesion.beginConfiguration()
        for e in sesion.inputs { sesion.removeInput(e) }
        for s in sesion.outputs { sesion.removeOutput(s) }
        sesion.sessionPreset = .medium
        let lado: AVCaptureDevice.Position = trasera ? .back : .front
        guard let disp = AVCaptureDevice.default(.builtInWideAngleCamera, for: .video, position: lado),
              let entrada = try? AVCaptureDeviceInput(device: disp),
              sesion.canAddInput(entrada) else {
            sesion.commitConfiguration()
            problema = "No encontré la cámara."
            return
        }
        sesion.addInput(entrada)
        let salida = AVCaptureVideoDataOutput()
        salida.alwaysDiscardsLateVideoFrames = true
        salida.setSampleBufferDelegate(self, queue: cola)
        if sesion.canAddOutput(salida) { sesion.addOutput(salida) }
        sesion.commitConfiguration()
        let s = sesion
        cola.async { if !s.isRunning { s.startRunning() } }
        activa = true
    }

    // Llega cada fotograma (en la cola de la cámara). Solo se mira uno cada 'cada' segundos.
    func captureOutput(_ output: AVCaptureOutput, didOutput sampleBuffer: CMSampleBuffer, from connection: AVCaptureConnection) {
        candado.lock()
        let toca = !mirando && Date().timeIntervalSince(ultima) >= cada
        if toca {
            mirando = true
            ultima = Date()
        }
        candado.unlock()
        if !toca { return }
        guard let pb = CMSampleBufferGetImageBuffer(sampleBuffer) else {
            libera()
            return
        }
        let imagen = CIImage(cvPixelBuffer: pb)
        guard let cg = contextoCI.createCGImage(imagen, from: imagen.extent) else {
            libera()
            return
        }
        var p = Ojos.mira(cg: cg)
        p.origen = "la cámara"
        DispatchQueue.main.async {
            self.alVer?(p)
            self.libera()
        }
    }

    private func libera() {
        candado.lock()
        mirando = false
        candado.unlock()
    }
}

/// Lo que ve la cámara, en pantalla.
struct VistaPrevia: UIViewRepresentable {
    let sesion: AVCaptureSession

    func makeUIView(context: Context) -> VistaCapa {
        let v = VistaCapa()
        v.capa.session = sesion
        v.capa.videoGravity = .resizeAspectFill
        v.backgroundColor = .black
        return v
    }

    func updateUIView(_ uiView: VistaCapa, context: Context) {}
}

final class VistaCapa: UIView {
    override class var layerClass: AnyClass { AVCaptureVideoPreviewLayer.self }
    var capa: AVCaptureVideoPreviewLayer { layer as! AVCaptureVideoPreviewLayer }
}
#endif
