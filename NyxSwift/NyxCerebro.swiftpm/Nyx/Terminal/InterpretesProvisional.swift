// Provisional: se sustituye por Python.swift y CFamilia.swift (intérpretes de Nyx).
import Foundation
enum Python {
    static func ejecuta(_ codigo: String, archivo: String, argumentos: [String], entrada: [String], escribe: @escaping (String) -> Void, cancelado: @escaping () -> Bool = { false }, limitePasos: Int = 50_000_000) -> Int { escribe("[python aún no instalado] \(archivo) entrada=\(entrada)\n"); return 0 }
}
enum LenguajeC { case c, cpp, java }
enum CFamilia {
    static func compila(_ codigo: String, lenguaje: LenguajeC, archivo: String) -> String? { codigo.contains("ERROR") ? "\(archivo):1:1: error: algo" : nil }
    static func ejecuta(_ codigo: String, lenguaje: LenguajeC, archivo: String, argumentos: [String], entrada: [String], escribe: @escaping (String) -> Void, cancelado: @escaping () -> Bool = { false }, limitePasos: Int = 50_000_000) -> Int { escribe("[\(lenguaje) stub] \(archivo) args=\(argumentos)\n"); return 0 }
}
