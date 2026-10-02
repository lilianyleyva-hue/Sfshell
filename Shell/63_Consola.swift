import Foundation

// ============================================================
// MARK: - Consola: programas interactivos
// ============================================================
// Cuando un programa (C, C++, Java, Python…) corre, su salida va
// apareciendo en pantalla mientras trabaja, y si pide datos (scanf,
// cin, Scanner, input) la siguiente línea que escribas es para él.
// ^C lo detiene.

final class Consola: @unchecked Sendable {
    private let lock = NSLock()
    private let senal = DispatchSemaphore(value: 0)
    private var lineas: [String] = []
    private var _cancelado = false
    private var _activa = false
    private var _esperando = false

    /// Muestra texto en la terminal (desde cualquier hilo).
    let escribir: @Sendable (String) -> Void
    /// En la versión de terminal (Linux) se lee directo del teclado.
    let lector: (@Sendable () -> String?)?

    init(escribir: @escaping @Sendable (String) -> Void, lector: (@Sendable () -> String?)? = nil) {
        self.escribir = escribir
        self.lector = lector
    }

    var activa: Bool { lock.conCandado { _activa } }
    var esperando: Bool { lock.conCandado { _esperando } }
    var cancelado: Bool { lock.conCandado { _cancelado } }

    func empieza() {
        lock.conCandado {
            _activa = true
            _cancelado = false
            lineas = []
        }
    }

    func termina() {
        lock.conCandado {
            _activa = false
            _esperando = false
        }
    }

    /// Lo que se escribe mientras corre un programa es su entrada.
    func entrega(_ linea: String) {
        lock.conCandado { lineas.append(linea) }
        senal.signal()
    }

    /// La siguiente línea escrita, si hay alguna (sin esperar).
    func tomaSiHay() -> String? {
        lock.conCandado { lineas.isEmpty ? nil : lineas.removeFirst() }
    }

    func cancela() {
        lock.conCandado { _cancelado = true }
        senal.signal()
    }

    /// Espera la siguiente línea; nil si se canceló o se cerró la entrada.
    func leeLinea() -> String? {
        if let l = lector {
            if cancelado { return nil }
            return l()
        }
        lock.conCandado { _esperando = true }
        defer { lock.conCandado { _esperando = false } }
        while true {
            let (linea, cancel): (String?, Bool) = lock.conCandado {
                if _cancelado { return (nil, true) }
                if !lineas.isEmpty { return (lineas.removeFirst(), false) }
                return (nil, false)
            }
            if cancel { return nil }
            if let l = linea { return l }
            _ = senal.wait(timeout: .now() + 0.25)
        }
    }
}

extension Shell {
    /// Ejecuta en un hilo con pila grande (los intérpretes son recursivos).
    static func enHiloGrande<T: Sendable>(_ trabajo: @escaping @Sendable () -> T) async -> T {
        await withCheckedContinuation { (c: CheckedContinuation<T, Never>) in
            let t = Thread {
                c.resume(returning: trabajo())
            }
            t.stackSize = 512 << 20
            t.start()
        }
    }
}
