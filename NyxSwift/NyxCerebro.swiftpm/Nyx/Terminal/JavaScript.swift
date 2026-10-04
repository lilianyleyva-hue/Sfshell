import Foundation
#if canImport(JavaScriptCore)
import JavaScriptCore
#endif

// JavaScript.swift — JavaScript de verdad: el motor de Safari (JavaScriptCore),
// que viene con el iPad. Se le añade console.log/error, print, prompt (lee la
// entrada), process.argv/process.exit y un require() mínimo (readline, fs).

enum JavaScriptNyx {
    #if canImport(JavaScriptCore)
    /// Lo que hace falta para que console.log, prompt… funcionen como en Node.
    static let preludio = """
    (function(){
      function inspecciona(v, prof) {
        if (typeof v === 'string') return prof > 0 ? "'" + v + "'" : v;
        if (v === null) return 'null';
        if (v === undefined) return 'undefined';
        if (typeof v === 'function') return '[Function: ' + (v.name || 'anónima') + ']';
        if (typeof v !== 'object') return String(v);
        if (prof > 3) return Array.isArray(v) ? '[Array]' : '[Object]';
        if (v instanceof Error) return v.stack ? (v.name + ': ' + v.message) : String(v);
        if (Array.isArray(v)) {
          if (v.length === 0) return '[]';
          return '[ ' + v.map(function(x){ return inspecciona(x, prof + 1); }).join(', ') + ' ]';
        }
        if (v instanceof Map) return 'Map(' + v.size + ') { ' + Array.from(v).map(function(p){ return inspecciona(p[0], prof+1) + ' => ' + inspecciona(p[1], prof+1); }).join(', ') + ' }';
        if (v instanceof Set) return 'Set(' + v.size + ') { ' + Array.from(v).map(function(x){ return inspecciona(x, prof+1); }).join(', ') + ' }';
        var ks = Object.keys(v);
        if (ks.length === 0) return '{}';
        return '{ ' + ks.map(function(k){ return k + ': ' + inspecciona(v[k], prof + 1); }).join(', ') + ' }';
      }
      function junta(args) { return Array.prototype.map.call(args, function(a){ return inspecciona(a, 0); }).join(' '); }
      var c = {};
      c.log = function(){ __escribe(junta(arguments) + '\\n'); };
      c.info = c.log; c.debug = c.log;
      c.error = function(){ __escribe(junta(arguments) + '\\n'); };
      c.warn = c.error;
      c.table = function(t){ __escribe(inspecciona(t, 0) + '\\n'); };
      this.console = c;
      this.print = c.log;
      this.prompt = function(t){ if (t !== undefined) __escribe(String(t)); return __lee(); };
      this.process = { argv: __argv, env: {}, exit: function(n){ throw { __salida: (n === undefined ? 0 : n) }; },
                       stdout: { write: function(s){ __escribe(String(s)); return true; } } };
      this.require = function(n){
        if (n === 'readline') return { createInterface: function(){ return {
            question: function(q, f){ __escribe(String(q)); f(__lee()); },
            on: function(ev, f){ if (ev === 'line') { var l; while ((l = __lee2()) !== null) f(l); } return this; },
            close: function(){} }; } };
        if (n === 'fs') return { readFileSync: function(){ return __todo(); } };
        throw new Error("Cannot find module '" + n + "' (en Nyx solo hay readline y fs)");
      };
      this.setTimeout = function(f){ f(); return 0; };
    })();
    """
    #endif

    static func ejecuta(_ codigo: String, archivo: String, argumentos: [String], entrada: [String],
                        escribe: @escaping (String) -> Void) -> Int {
        #if canImport(JavaScriptCore)
        guard let ctx = JSContext() else { escribe("No se pudo arrancar JavaScript\n"); return 1 }
        var fallo: JSValue? = nil
        ctx.exceptionHandler = { _, ex in fallo = ex }
        var cola = entrada
        let escribir: @convention(block) (String) -> Void = { t in escribe(t) }
        let leer: @convention(block) () -> String = { cola.isEmpty ? "" : cola.removeFirst() }
        let leer2: @convention(block) () -> Any = {
            if cola.isEmpty { return NSNull() }
            return cola.removeFirst()
        }
        let todo: @convention(block) () -> String = { entrada.joined(separator: "\n") }
        ctx.setObject(escribir, forKeyedSubscript: "__escribe" as NSString)
        ctx.setObject(leer, forKeyedSubscript: "__lee" as NSString)
        ctx.setObject(leer2, forKeyedSubscript: "__lee2" as NSString)
        ctx.setObject(todo, forKeyedSubscript: "__todo" as NSString)
        ctx.setObject(["node", archivo] + argumentos, forKeyedSubscript: "__argv" as NSString)
        ctx.evaluateScript(preludio)
        ctx.evaluateScript(codigo, withSourceURL: URL(fileURLWithPath: archivo))
        if let ex = fallo {
            if let s = ex.objectForKeyedSubscript("__salida"), s.isNumber { return Int(s.toInt32()) }
            let linea = ex.objectForKeyedSubscript("line")?.toInt32() ?? 0
            let nombre = ex.objectForKeyedSubscript("name")?.toString() ?? "Error"
            let mensaje = ex.objectForKeyedSubscript("message")?.toString() ?? ex.toString() ?? ""
            escribe("\(archivo):\(linea)\n\(nombre == "undefined" ? "Error" : nombre): \(mensaje == "undefined" ? (ex.toString() ?? "") : mensaje)\n")
            return 1
        }
        return 0
        #else
        escribe("JavaScript no está disponible en este equipo (en el iPad sí: usa JavaScriptCore).\n")
        return 1
        #endif
    }
}
