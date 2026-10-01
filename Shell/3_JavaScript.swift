import Foundation
#if os(Linux) && canImport(FoundationNetworking)
import FoundationNetworking
#endif
#if canImport(JavaScriptCore) && APPLE_COMPLETO
import JavaScriptCore
#endif

// ============================================================
// MARK: - Motor JavaScript (JavaScriptCore)
// ============================================================

final class OutBox: @unchecked Sendable { var s = "" }

#if canImport(JavaScriptCore) && APPLE_COMPLETO
final class JSRuntime: @unchecked Sendable {
    private(set) var ctx: JSContext
    private let box = OutBox()
    private var lastError: String?
    /// Se conecta al crear el shell: da a los scripts acceso a los archivos.
    weak var envRef: ShellEnv?

    init() {
        ctx = JSContext()
        configure()
    }

    private func configure() {
        let box = self.box
        let p: @convention(block) (JSValue?) -> Void = { v in
            box.s += (v?.toString() ?? "undefined") + "\n"
        }
        ctx.setObject(p, forKeyedSubscript: "__out" as NSString)

        // --- puente HTTP síncrono (API) ---
        let http: @convention(block) (String, String, String, String) -> String = { method, urlStr, headersJSON, body in
            var s = urlStr
            if !s.contains("://") { s = "https://" + s }
            guard let url = URL(string: s) else { return "error: URL no válida" }
            var req = URLRequest(url: url)
            req.httpMethod = method.isEmpty ? "GET" : method
            req.timeoutInterval = 30
            if let d = headersJSON.data(using: .utf8),
               let h = (try? JSONSerialization.jsonObject(with: d)) as? [String: String] {
                for (k, v) in h { req.setValue(v, forHTTPHeaderField: k) }
            }
            if !body.isEmpty {
                req.httpBody = Data(body.utf8)
                if req.value(forHTTPHeaderField: "Content-Type") == nil {
                    req.setValue("application/json", forHTTPHeaderField: "Content-Type")
                }
            }
            let sem = DispatchSemaphore(value: 0)
            let result = OutBox()   // caja compartida: Swift 6 no deja mutar una 'var' capturada
            URLSession.shared.dataTask(with: req) { data, _, err in
                if let err = err { result.s = "error: " + err.localizedDescription }
                else { result.s = String(data: data ?? Data(), encoding: .utf8) ?? "" }
                sem.signal()
            }.resume()
            _ = sem.wait(timeout: .now() + 35)
            return result.s
        }
        ctx.setObject(http, forKeyedSubscript: "__http" as NSString)

        // --- puente de archivos ---
        let readFile: @convention(block) (String) -> String = { [weak self] path in
            guard let env = self?.envRef, let u = try? env.resolve(path),
                  let d = FileManager.default.contents(atPath: u.path) else { return "" }
            return String(data: d, encoding: .utf8) ?? ""
        }
        let writeFile: @convention(block) (String, String) -> Bool = { [weak self] path, text in
            guard let env = self?.envRef, let u = try? env.resolve(path) else { return false }
            return (try? text.write(to: u, atomically: true, encoding: .utf8)) != nil
        }
        let listDir: @convention(block) (String) -> [String] = { [weak self] path in
            guard let env = self?.envRef, let u = try? env.resolve(path.isEmpty ? "." : path) else { return [] }
            return ((try? FileManager.default.contentsOfDirectory(atPath: u.path)) ?? []).sorted()
        }
        ctx.setObject(readFile, forKeyedSubscript: "readFile" as NSString)
        ctx.setObject(writeFile, forKeyedSubscript: "writeFile" as NSString)
        ctx.setObject(listDir, forKeyedSubscript: "listDir" as NSString)

        ctx.exceptionHandler = { [weak self] _, e in
            self?.lastError = e?.toString() ?? "error de JavaScript"
        }

        ctx.evaluateScript("""
        var __j = function(a){ return Array.prototype.slice.call(a).map(function(x){
            if (typeof x === 'object' && x !== null) { try { return JSON.stringify(x); } catch(e) { return String(x); } }
            return String(x); }).join(' '); };
        var console = { log: function(){ __out(__j(arguments)); },
                        error: function(){ __out('error: ' + __j(arguments)); },
                        warn: function(){ __out('aviso: ' + __j(arguments)); },
                        info: function(){ __out(__j(arguments)); } };
        function print(){ __out(__j(arguments)); }
        function __str(x){ return String(x); }
        function __int(x){ var n = Number(x); return isNaN(n) ? 0 : Math.trunc(n); }
        function __dbl(x){ var n = Number(x); return isNaN(n) ? 0 : n; }
        var http = {
          request: function(m, u, h, b){
            var body = (b === undefined || b === null) ? '' : (typeof b === 'string' ? b : JSON.stringify(b));
            return __http(m, u, JSON.stringify(h || {}), body);
          },
          get: function(u, h){ return http.request('GET', u, h, ''); },
          post: function(u, b, h){ return http.request('POST', u, h, b); },
          put: function(u, b, h){ return http.request('PUT', u, h, b); },
          del: function(u, h){ return http.request('DELETE', u, h, ''); },
          json: function(u, h){ try { return JSON.parse(http.get(u, h)); } catch(e) { return null; } }
        };
        function fetchText(u){ return http.get(u); }
        function fetchJSON(u){ return http.json(u); }
        """)
        // lo que el código traducido de Swift espera encontrar (count, append…)
        ctx.evaluateScript(SwiftJS.preludio)
    }

    func reset() {
        ctx = JSContext()
        configure()
    }

    func eval(_ code: String) throws -> String {
        box.s = ""
        lastError = nil
        let value = ctx.evaluateScript(code)
        if let e = lastError { throw ShErr(e) }
        var out = box.s
        if out.isEmpty, let v = value, !v.isUndefined, !v.isNull {
            out = v.toString() + "\n"
        }
        return out
    }
}
#else
/// Sin JavaScriptCore (Linux/Debian): el resto de la shell funciona igual,
/// solo 'js' y 'swift' avisan de que no hay motor.
final class JSRuntime: @unchecked Sendable {
    weak var envRef: ShellEnv?
    func reset() {}
    func eval(_ code: String) throws -> String {
        throw ShErr("js: no hay motor JavaScript en este sistema")
    }
}
#endif

// ============================================================
// MARK: - Traductor de un subconjunto de Swift a JavaScript
// ============================================================

enum SwiftJS {

    /// Palabras que el traductor no puede convertir, con su motivo.
    static let unsupported: [(String, String)] = [
        ("protocol", "los protocolos no existen en JavaScript"),
        ("actor", "la concurrencia de Swift no tiene equivalente aquí"),
        ("@State", "SwiftUI"),
        ("@Binding", "SwiftUI"),
        ("@Published", "SwiftUI"),
        ("@StateObject", "SwiftUI"),
        ("@ObservedObject", "SwiftUI"),
        ("@EnvironmentObject", "SwiftUI"),
        ("@main", "SwiftUI"),
        ("some View", "SwiftUI"),
        ("some Scene", "SwiftUI"),
        ("import SwiftUI", "SwiftUI"),
        ("import SceneKit", "SwiftUI"),
        ("import UIKit", "SwiftUI")
    ]

    /// Revisa el código y devuelve TODO lo que no se puede traducir, de una vez.
    static func revisar(_ src: String) -> [(String, String)] {
        // quitamos comentarios para no dar falsas alarmas
        var limpio = ""
        var i = src.startIndex
        while i < src.endIndex {
            if src[i] == "/", src.index(after: i) < src.endIndex, src[src.index(after: i)] == "/" {
                while i < src.endIndex, src[i] != "\n" { i = src.index(after: i) }
                continue
            }
            limpio.append(src[i])
            i = src.index(after: i)
        }
        return unsupported.filter { limpio.contains($0.0) }
    }

    static func transpile(_ rawSrc: String) throws -> String {
        let problemas = revisar(rawSrc)
        if !problemas.isEmpty {
            let esSwiftUI = problemas.contains { $0.1 == "SwiftUI" }
            var m = ""
            if esSwiftUI {
                m += "swift: esto es una app de SwiftUI, no un guion.\n"
                m += "Describe una interfaz que solo puede construir el compilador de\n"
                m += "Playgrounds al pulsar el play. Ningún intérprete dentro de una app\n"
                m += "de iOS puede crear vistas nuevas: eso lo decide la compilación.\n\n"
            } else {
                m += "swift: hay cosas que el traductor no admite.\n\n"
            }
            m += "encontrado: " + problemas.map { $0.0 }.joined(separator: ", ") + "\n\n"
            if esSwiftUI {
                m += "Lo que sí puedes ejecutar: la lógica sin vistas — struct, class,\n"
                m += "enum, extension, func, if, for, while, switch, guard, arrays y\n"
                m += "cadenas. Saca esas partes a su propio archivo y pruébalas aquí."
            } else {
                m += "Mira 'help swift' para la lista completa."
            }
            throw ShErr(m)
        }
        let src = preprocess(rawSrc)
        // Las cadenas y los comentarios se apartan como marcas y el resto del
        // código se traduce DE UNA VEZ: así 'if x == "a" {' se ve entero y no
        // partido en trozos alrededor de la cadena.
        var guardados: [String] = []
        var code = ""
        func guarda(_ t: String) {
            code += "\u{E000}\(guardados.count)\u{E001}"
            guardados.append(t)
        }
        var i = src.startIndex

        while i < src.endIndex {
            let c = src[i]

            if c == "/", src.index(after: i) < src.endIndex, src[src.index(after: i)] == "/" {
                var comentario = ""
                while i < src.endIndex, src[i] != "\n" { comentario.append(src[i]); i = src.index(after: i) }
                guarda(comentario)
                continue
            }

            if c == "\"" {
                var lit = ""
                i = src.index(after: i)
                while i < src.endIndex, src[i] != "\"" {
                    if src[i] == "\\" {
                        let n = src.index(after: i)
                        if n < src.endIndex, src[n] == "(" {
                            var depth = 1
                            var expr = ""
                            var j = src.index(after: n)
                            while j < src.endIndex {
                                if src[j] == "(" { depth += 1 }
                                if src[j] == ")" { depth -= 1; if depth == 0 { break } }
                                expr.append(src[j]); j = src.index(after: j)
                            }
                            lit += "${" + convertCode(expr) + "}"
                            i = j < src.endIndex ? src.index(after: j) : j
                            continue
                        }
                        if n < src.endIndex { lit.append("\\"); lit.append(src[n]); i = src.index(after: n); continue }
                    }
                    if src[i] == "`" { lit += "\\`" }
                    else if src[i] == "$" { lit += "\\$" }
                    else { lit.append(src[i]) }
                    i = src.index(after: i)
                }
                if i < src.endIndex { i = src.index(after: i) }
                guarda("`" + lit + "`")
                continue
            }

            code.append(c)
            i = src.index(after: i)
        }
        var out = convertCode(code)
        for (k, t) in guardados.enumerated().reversed() {
            out = out.replacingOccurrences(of: "\u{E000}\(k)\u{E001}", with: t)
        }
        return out
    }
}
