#if canImport(Speech) && canImport(AVFoundation) && canImport(SwiftUI)
import Foundation
import SwiftUI
import Speech
import AVFoundation

// Oidos.swift — las mentes oyen por el micrófono: qué dices (reconocimiento de
// voz en español, de Apple) y cómo suena (fuerte o suave, agudo o grave).

final class Oido: ObservableObject {
    @Published var escuchando = false
    @Published var texto = ""
    @Published var nivel: Double = 0
    @Published var problema = ""

    private let motor = AVAudioEngine()
    private var peticion: SFSpeechAudioBufferRecognitionRequest?
    private var tarea: SFSpeechRecognitionTask?
    private let reconocedor = SFSpeechRecognizer(locale: Locale(identifier: "es-ES"))
    private let candado = NSLock()
    private var sumaNivel = 0.0
    private var sumaCruces = 0.0
    private var muestras = 0

    /// Pide permiso (micrófono y voz) y empieza a escuchar.
    func empieza() {
        problema = ""
        SFSpeechRecognizer.requestAuthorization { (estado: SFSpeechRecognizerAuthorizationStatus) in
            AVAudioSession.sharedInstance().requestRecordPermission { (ok: Bool) in
                DispatchQueue.main.async {
                    if estado == .authorized && ok {
                        self.arranca()
                    } else {
                        self.problema = "Sin permiso para el micrófono o el reconocimiento de voz (míralo en Ajustes)."
                    }
                }
            }
        }
    }

    private func arranca() {
        texto = ""
        candado.lock()
        sumaNivel = 0
        sumaCruces = 0
        muestras = 0
        candado.unlock()
        do {
            let sesion = AVAudioSession.sharedInstance()
            try sesion.setCategory(.record, mode: .measurement, options: .duckOthers)
            try sesion.setActive(true, options: .notifyOthersOnDeactivation)
            let pet = SFSpeechAudioBufferRecognitionRequest()
            pet.shouldReportPartialResults = true
            peticion = pet
            let entrada = motor.inputNode
            let formato = entrada.outputFormat(forBus: 0)
            entrada.installTap(onBus: 0, bufferSize: 1024, format: formato) { [weak self] (b: AVAudioPCMBuffer, _: AVAudioTime) in
                pet.append(b)
                self?.mide(b)
            }
            motor.prepare()
            try motor.start()
            tarea = reconocedor?.recognitionTask(with: pet) { [weak self] (r: SFSpeechRecognitionResult?, _: Error?) in
                guard let r = r else { return }
                let t = r.bestTranscription.formattedString
                DispatchQueue.main.async { self?.texto = t }
            }
            escuchando = true
        } catch {
            problema = "No pude usar el micrófono."
            _ = para()
        }
    }

    /// Volumen (RMS) y cruces por cero (agudo/grave) de cada trozo de sonido.
    private func mide(_ b: AVAudioPCMBuffer) {
        guard let canales = b.floatChannelData else { return }
        let datos = canales[0]
        let n = Int(b.frameLength)
        if n < 2 { return }
        var suma: Float = 0
        var cruces = 0
        for i in 0 ..< n {
            let x = datos[i]
            suma += x * x
            if i > 0 && (datos[i - 1] < 0) != (x < 0) { cruces += 1 }
        }
        let rms = (suma / Float(n)).squareRoot()
        candado.lock()
        sumaNivel += Double(rms)
        sumaCruces += Double(cruces) / Double(n)
        muestras += 1
        candado.unlock()
        DispatchQueue.main.async { self.nivel = min(1, Double(rms) * 10) }
    }

    /// Deja de escuchar y devuelve lo que oyó.
    func para() -> Percepcion {
        if motor.isRunning {
            motor.stop()
            motor.inputNode.removeTap(onBus: 0)
        }
        peticion?.endAudio()
        tarea?.finish()
        peticion = nil
        tarea = nil
        escuchando = false
        nivel = 0
        try? AVAudioSession.sharedInstance().setActive(false, options: .notifyOthersOnDeactivation)
        var p = Percepcion(sentido: .oido)
        p.texto = texto.lowercased()
        p.cosas = Array(Palabras.tokens(p.texto, max: 12).filter { !Palabras.vacia($0) }.prefix(6))
        candado.lock()
        let m = muestras
        let nivelMedio = m > 0 ? sumaNivel / Double(m) : 0
        let crucesMedio = m > 0 ? sumaCruces / Double(m) : 0
        candado.unlock()
        if m > 0 {
            p.rasgos.append(nivelMedio > 0.05 ? "fuerte" : "suave")
            p.rasgos.append(crucesMedio > 0.12 ? "agudo" : "grave")
        }
        return p
    }
}
#endif
