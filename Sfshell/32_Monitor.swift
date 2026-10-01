import SwiftUI
import UIKit

// ============================================================
// MARK: - Monitor del sistema: datos reales del dispositivo
// ============================================================
// Como el "Monitor del sistema" de Ubuntu, pero honesto con lo que
// una app normal de iOS puede consultar de verdad: batería,
// almacenamiento, RAM total y tiempo encendido salen de
// UIDevice/FileManager/ProcessInfo, APIs públicas reales. Lo que iOS
// no deja ver a una app normal (uso de CPU en vivo por proceso, RAM
// exacta en uso, tráfico de red) se dice claramente que no está
// disponible, en vez de inventar un número que quede bonito.

struct MonitorView: View {
    @Environment(\.dismiss) private var dismiss
    var onCerrar: (() -> Void)? = nil

    @State private var bateria: Float = -1
    @State private var estadoCarga: UIDevice.BatteryState = .unknown
    @State private var libreGB: Double = 0
    @State private var totalGB: Double = 0
    @State private var memoriaGB: Double = 0

    private let timer = Timer.publish(every: 5, on: .main, in: .common).autoconnect()

    var body: some View {
        NavigationStack {
            Form {
                Section("Batería") {
                    if bateria < 0 {
                        Text("no disponible en este dispositivo o simulador")
                            .foregroundStyle(.secondary)
                    } else {
                        LabeledContent("Nivel", value: "\(Int(bateria * 100))%")
                        LabeledContent("Estado", value: textoEstado)
                    }
                }
                Section("Almacenamiento") {
                    LabeledContent("Libre", value: String(format: "%.1f GB", libreGB))
                    LabeledContent("Total", value: String(format: "%.1f GB", totalGB))
                    ProgressView(value: totalGB > 0 ? max(0, min(1, (totalGB - libreGB) / totalGB)) : 0)
                }
                Section("Sistema") {
                    LabeledContent("RAM del dispositivo", value: String(format: "%.1f GB", memoriaGB))
                    LabeledContent("Encendido desde hace", value: uptimeTexto)
                    LabeledContent("Núcleos de CPU", value: "\(ProcessInfo.processInfo.activeProcessorCount)")
                    LabeledContent("iOS", value: UIDevice.current.systemVersion)
                    LabeledContent("Modelo", value: UIDevice.current.model)
                }
                Section {
                    Text("iOS no deja a una app normal leer el uso de CPU en vivo, la RAM exacta en uso ni el tráfico de red — por eso no aparecen aquí. Mejor decirlo que inventar un número.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
            .navigationTitle("Monitor")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cerrar") { onCerrar?() ?? dismiss() }
                }
            }
            .onAppear(perform: actualiza)
            .onReceive(timer) { _ in actualiza() }
        }
    }

    private var textoEstado: String {
        switch estadoCarga {
        case .charging: return "cargando"
        case .full: return "completa"
        case .unplugged: return "con batería"
        default: return "desconocido"
        }
    }

    private var uptimeTexto: String {
        let s = Int(ProcessInfo.processInfo.systemUptime)
        let h = s / 3600, m = (s % 3600) / 60
        return "\(h)h \(m)m"
    }

    private func actualiza() {
        UIDevice.current.isBatteryMonitoringEnabled = true
        bateria = UIDevice.current.batteryLevel
        estadoCarga = UIDevice.current.batteryState
        memoriaGB = Double(ProcessInfo.processInfo.physicalMemory) / 1_073_741_824
        if let attrs = try? FileManager.default.attributesOfFileSystem(forPath: NSHomeDirectory()) {
            let libre = (attrs[.systemFreeSize] as? NSNumber)?.doubleValue ?? 0
            let total = (attrs[.systemSize] as? NSNumber)?.doubleValue ?? 0
            libreGB = libre / 1_073_741_824
            totalGB = total / 1_073_741_824
        }
    }
}
