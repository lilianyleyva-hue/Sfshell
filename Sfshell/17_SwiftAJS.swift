import Foundation

// ============================================================
// MARK: - Traductor de Swift a JavaScript: el código suelto
// ============================================================
// 3_JavaScript.swift llamaba a 'convertCode', pero esa función no
// existía en ningún archivo: por eso el proyecto no compilaba.
// Aquí está, hecha de nuevo.
//
// Recibe el código YA SIN cadenas ni comentarios (van guardados
// aparte como marcas) y traduce lo que queda: func, if, while,
// for, repeat, let/var, cierres, etiquetas de argumentos,
// diccionarios, rangos, nil, self, try, as…
// Lo que JavaScript no trae (count, append, isEmpty, uppercased…)
// lo añade 'preludio' al motor JS, así el código traducido se
// parece mucho al original.

extension SwiftJS {

    /// Marca interna: va pegada a la '{' de un cierre de varias líneas
    /// para que su '}' se cierre como '})'.
    static let marcaCierre: Character = "\u{E200}"

    static func rx(_ s: String, _ patron: String, _ plantilla: String) -> String {
        guard let re = try? NSRegularExpression(pattern: patron, options: [.anchorsMatchLines]) else { return s }
        return re.stringByReplacingMatches(in: s, range: NSRange(s.startIndex..., in: s), withTemplate: plantilla)
    }

    /// Grupos de la primera coincidencia ("" si un grupo no participó).
    static func grupos(_ s: String, _ patron: String) -> [String]? {
        guard let re = try? NSRegularExpression(pattern: patron),
              let m = re.firstMatch(in: s, range: NSRange(s.startIndex..., in: s)) else { return nil }
        return (0..<m.numberOfRanges).map { k in
            guard let r = Range(m.range(at: k), in: s) else { return "" }
            return String(s[r])
        }
    }

    // ---------- la función principal ----------

    static func convertCode(_ code: String) -> String {
        var s = code
        // bloques que cambian de nombre (el orden importa: 'do' de Swift
        // es 'try' en JS, y 'repeat' de Swift es 'do' en JS)
        s = rx(s, #"\bdo\s*\{"#, "try {")
        s = rx(s, #"\}\s*catch\s*\{"#, "} catch (error) {")
        s = rx(s, #"\brepeat\s*\{"#, "do {")

        // llamadas de Swift que en JS se llaman distinto o llevan otro orden
        s = rx(s, #"String\(\s*repeating:"#, "__repetir(")
        s = rx(s, #"Array\(\s*repeating:"#, "__arrayRep(")
        s = rx(s, #"\.split\(\s*separator:"#, ".partir(")
        s = rx(s, #"\.components\(\s*separatedBy:"#, ".split(")
        s = rx(s, #"\.first\(\s*where:"#, ".primero(")
        s = rx(s, #"\.last\(\s*where:"#, ".ultimo(")
        s = rx(s, #"\.firstIndex\(\s*where:"#, ".findIndex(")
        s = rx(s, #"\.contains\(\s*where:"#, ".some(")
        s = rx(s, #"\.append\(\s*contentsOf:"#, ".appendAll(")
        s = rx(s, #"\.reduce\("#, ".reduceSw(")
        s = rx(s, #",\s*([+*])\s*\)"#, ", __op(\"$1\"))")
        s = rx(s, #"\[(\w+)\s*:\s*(\w+)\]\(\)"#, "{}")
        s = rx(s, #"\[(\w+)\]\(\)"#, "[]")

        // línea a línea: func, if, while, for, let/var…
        s = s.components(separatedBy: "\n").map(convertirLinea).joined(separator: "\n")

        // cierres { $0 * 2 }  { x in … }
        s = convertirCierres(s)
        s = cerrarCierres(s)

        // etiquetas de argumentos y diccionarios
        s = quitarEtiquetas(s)

        // rangos sueltos: 1...6  0..<n
        let operando = #"(\w+(?:\.\w+)*|\([^()\n]*\)|\d+)"#
        s = rx(s, operando + #"\s*\.\.<\s*"# + operando, "__rango($1, $2, false)")
        s = rx(s, operando + #"\s*\.\.\.\s*"# + operando, "__rango($1, $2, true)")

        // palabras sueltas
        // .rojo (caso de enum abreviado) → "rojo": los enums se traducen a textos
        s = rx(s, #"([=(,:?]\s*|\breturn\s+|\bcase\s+)\.([a-z]\w*)\b(?!\s*\()"#, "$1\"$2\"")
        s = rx(s, #"\bnil\b"#, "null")
        // tupla.0 → tupla[0]
        s = rx(s, #"([A-Za-z_]\w*|\))\.(\d+)\b"#, "$1[$2]")
        s = rx(s, #"\bself\b"#, "this")
        s = rx(s, #"\s+as[!?]?\s+\[?\w+\]?[?!]?"#, "")
        s = rx(s, #"\btry[!?]?\s+(?!\{)"#, "")
        s = rx(s, #"\bthrows\s+"#, "")
        s = rx(s, #"\bfallthrough\b"#, "")
        // x! (desenvolver a la fuerza) → x   (sin tocar '!=')
        s = rx(s, #"([\w\)\]])!(?!=)"#, "$1")
        return s
    }

    // ---------- una línea ----------

    static func convertirLinea(_ l: String) -> String {
        let sangria = String(l.prefix { $0 == " " || $0 == "\t" })
        let t = String(l.dropFirst(sangria.count))

        // func nombre(_ a: Int, b: String = "x") -> Int {
        if let m = grupos(t, #"^(?:(?:private|fileprivate|public|internal|static|mutating|@discardableResult)\s+)*func\s+(\w+)\s*\((.*)\)\s*(?:throws\s*)?(?:->\s*[^{]+)?\{\s*$"#) {
            return sangria + "function \(m[1])(\(parametros(m[2]))) {"
        }
        // if let x = y {   /   } else if let x = y {
        if let m = grupos(t, #"^(\}\s*else\s+)?if\s+(?:let|var)\s+(\w+)\s*=\s*(.+?)\s*\{\s*$"#) {
            let pre = m[1].isEmpty ? "" : "} else "
            return sangria + pre + "if ((\(m[2]) = \(m[3])) != null) {"
        }
        // if let x {   (forma corta)
        if let m = grupos(t, #"^(\}\s*else\s+)?if\s+(?:let|var)\s+(\w+)\s*\{\s*$"#) {
            let pre = m[1].isEmpty ? "" : "} else "
            return sangria + pre + "if (\(m[2]) != null) {"
        }
        // if / else if / while con condición (el bloque puede seguir en la misma línea)
        if let m = grupos(t, #"^(\}\s*else\s+if|if|while)\s+(.+)$"#), let (cond, resto) = separarBloque(m[2]) {
            return sangria + "\(m[1]) (\(condicion(cond))) {" + resto
        }
        // } while cond      (final de un repeat)
        if let m = grupos(t, #"^\}\s*while\s+(.+?)\s*;?\s*$"#) {
            return sangria + "} while (\(condicion(m[1])))"
        }
        // for … in … {
        if let m = grupos(t, #"^for\s+(?:case\s+)?(.+?)\s+in\s+(.+)$"#), let (seq, resto) = separarBloque(m[2]) {
            return sangria + "for (\(cabeceraFor(m[1], seq))) {" + resto
        }
        // let / var con o sin tipo
        if let m = grupos(t, #"^(?:(?:private|fileprivate|public|internal|static|lazy|weak)\s+)*(?:let|var)\s+(\([\w\s,]+\)|\w+)\s*(?::\s*[^=]+?)?\s*(=.*)?$"#) {
            var nombre = m[1].trimmingCharacters(in: .whitespaces)
            if nombre.hasPrefix("(") { nombre = "[" + nombre.dropFirst().dropLast() + "]" }
            var valor = m[2]
            // una tupla (1, "dos") es un array en JS (si no, la coma la rompe)
            if let v = grupos(valor, #"^=\s*\((.*)\)\s*$"#), partirArriba(v[1], ",").count > 1 {
                valor = "= [" + v[1] + "]"
            }
            return sangria + "var \(nombre)" + (valor.isEmpty ? "" : " \(valor)")
        }
        // conteo[k, default: 0] += 1
        if let m = grupos(t, #"^([\w.]+)\[(.+?),\s*default:\s*(.+?)\]\s*([+\-*/])=\s*(.+)$"#) {
            return sangria + "\(m[1])[\(m[2])] = (\(m[1])[\(m[2])] ?? \(m[3])) \(m[4]) (\(m[5]))"
        }
        if let m = grupos(t, #"^(.*?)([\w.]+)\[(.+?),\s*default:\s*(.+?)\](.*)$"#) {
            return sangria + m[1] + "(\(m[2])[\(m[3])] ?? \(m[4]))" + m[5]
        }
        // _ = algo
        if t.hasPrefix("_ = ") { return sangria + String(t.dropFirst(4)) }
        return l
    }

    /// Parte "cond { resto" por la primera '{' que no esté dentro de
    /// paréntesis o corchetes (así 'if lista.some({ $0 > 1 }) {' funciona).
    static func separarBloque(_ s: String) -> (String, String)? {
        var nivel = 0
        var i = s.startIndex
        while i < s.endIndex {
            let ch = s[i]
            if ch == "(" || ch == "[" { nivel += 1 }
            if ch == ")" || ch == "]" { nivel -= 1 }
            if ch == "{" && nivel == 0 {
                let cabeza = s[s.startIndex..<i].trimmingCharacters(in: .whitespaces)
                guard !cabeza.isEmpty else { return nil }
                return (cabeza, String(s[s.index(after: i)...]))
            }
            i = s.index(after: i)
        }
        return nil
    }

    /// "_ a: Int, b c: String = x" → "a, c = x"
    static func parametros(_ p: String) -> String {
        guard !p.trimmingCharacters(in: .whitespaces).isEmpty else { return "" }
        return partirArriba(p, ",").map { param -> String in
            let partes = param.components(separatedBy: "=")
            let cabeza = partes[0].components(separatedBy: ":")[0]
            let nombre = cabeza.split(separator: " ").last.map(String.init) ?? cabeza
            let defecto = partes.count > 1 ? " = " + partes.dropFirst().joined(separator: "=").trimmingCharacters(in: .whitespaces) : ""
            return nombre.trimmingCharacters(in: .whitespaces) + defecto
        }.joined(separator: ", ")
    }

    /// Las comas de Swift en un if ("if a, b") son '&&' en JS.
    static func condicion(_ c: String) -> String {
        partirArriba(c, ",").map { parte -> String in
            let t = parte.trimmingCharacters(in: .whitespaces)
            if let m = grupos(t, #"^(?:let|var)\s+(\w+)\s*=\s*(.+)$"#) { return "(\(m[1]) = \(m[2])) != null" }
            if let m = grupos(t, #"^(?:let|var)\s+(\w+)$"#) { return "\(m[1]) != null" }
            return t
        }.joined(separator: " && ")
    }

    /// Parte por un separador solo fuera de paréntesis y corchetes.
    static func partirArriba(_ s: String, _ sep: Character) -> [String] {
        var r: [String] = []
        var actual = ""
        var nivel = 0
        for ch in s {
            if "([{".contains(ch) { nivel += 1 }
            if ")]}".contains(ch) { nivel -= 1 }
            if ch == sep && nivel == 0 { r.append(actual); actual = "" } else { actual.append(ch) }
        }
        r.append(actual)
        return r
    }

    static func cabeceraFor(_ v: String, _ seq: String) -> String {
        let s = seq.trimmingCharacters(in: .whitespaces)
        let variable = v.trimmingCharacters(in: .whitespaces)
        // 0..<n   1...n
        if !variable.hasPrefix("("), let m = grupos(s, #"^(.+?)\s*(\.\.<|\.\.\.)\s*(.+)$"#), !m[1].hasSuffix(")") || m[1].hasPrefix("(") {
            let op = m[2] == "..<" ? "<" : "<="
            return "var \(variable) = \(m[1]); \(variable) \(op) \(m[3]); \(variable)++"
        }
        // stride(from: a, to: b, by: c)   /   through:
        if let m = grupos(s, #"^stride\(\s*from:\s*(.+?),\s*(to|through):\s*(.+?),\s*by:\s*(.+)\)$"#) {
            let op = m[2] == "to" ? ("<", ">") : ("<=", ">=")
            return "var \(variable) = \(m[1]); (\(m[4])) > 0 ? \(variable) \(op.0) \(m[3]) : \(variable) \(op.1) \(m[3]); \(variable) += \(m[4])"
        }
        // (i, x) in lista.enumerated()   /   (clave, valor) in diccionario
        if variable.hasPrefix("(") {
            let dentro = variable.dropFirst().dropLast()
            return "var [\(dentro)] of __pares(\(s))"
        }
        return "var \(variable) of __iter(\(s))"
    }

    // ---------- cierres ----------

    static func convertirCierres(_ s: String) -> String {
        var r = s
        let lista = #"\(?(\w+(?:\s*,\s*\w+)*)\)?"#
        // f(a) { $0 + 1 }   → f(a, ($0, $1, $2) => ($0 + 1))
        r = rx(r, #"([\w\]\)])\(([^()\n]*)\)\s*\{\s*([^{}\n]*\$\d[^{}\n]*?)\s*\}"#, "$1($2, (\\$0, \\$1, \\$2) => ($3))")
        // { $0 * 2 }        → (($0, $1, $2) => ($0 * 2))
        r = rx(r, #"\{\s*([^{}\n]*\$\d[^{}\n]*?)\s*\}"#, "((\\$0, \\$1, \\$2) => ($1))")
        // f(a) { x in x + 1 }
        r = rx(r, #"([\w\]\)])\(([^()\n]*)\)\s*\{\s*"# + lista + #"\s+in\s+([^{}\n]+?)\s*\}"#, "$1($2, ($3) => ($4))")
        // { x, y in x + y }
        r = rx(r, #"\{\s*"# + lista + #"\s+in\s+([^{}\n]+?)\s*\}"#, "(($1) => ($2))")
        // cierres de varias líneas:  lista.forEach { x in
        r = rx(r, #"([\w\]\)])\(([^()\n]*)\)\s*\{\s*"# + lista + #"\s+in\s*$"#, "$1($2, ($3) => {\(marcaCierre)")
        r = rx(r, #"([\w\]\)])\s*\{\s*"# + lista + #"\s+in\s*$"#, "$1(($2) => {\(marcaCierre)")
        //                         lista.forEach {    (usa $0 dentro)
        r = rx(r, #"(\.\w+)\s*\{\s*$"#, "$1((\\$0, \\$1, \\$2) => {\(marcaCierre)")
        // un 'return' dentro de una flecha de una línea sobra
        r = rx(r, #"=> \(return\s+"#, "=> (")
        return r
    }

    /// Cierra con '})' las llaves de los cierres de varias líneas.
    static func cerrarCierres(_ s: String) -> String {
        var out = ""
        var pila: [Bool] = []
        var it = Array(s)
        var i = 0
        while i < it.count {
            let ch = it[i]
            if ch == "{" {
                if i + 1 < it.count, it[i + 1] == marcaCierre { pila.append(true); out.append("{"); i += 2; continue }
                pila.append(false)
            } else if ch == "}" {
                if pila.popLast() == true { out += "})"; i += 1; continue }
            } else if ch == marcaCierre {
                i += 1; continue
            }
            out.append(ch)
            i += 1
        }
        it.removeAll()
        return out
    }

    // ---------- etiquetas y diccionarios ----------

    /// Quita "etiqueta:" de las llamadas (f(a: 1, b: 2) → f(1, 2)) y
    /// convierte los diccionarios [k: v] en objetos {k: v}.
    static func quitarEtiquetas(_ s: String) -> String {
        let c = Array(s)
        var out = ""
        var pila: [Character] = []     // "(" "[" "{" o "D" (diccionario)
        var i = 0
        var claveAbierta = false       // dentro de la clave de un diccionario
        func ultimoSignificativo() -> Character? {
            out.last(where: { $0 != " " && $0 != "\t" && $0 != "\n" })
        }
        while i < c.count {
            let ch = c[i]
            switch ch {
            case "(":
                pila.append("("); out.append(ch); i += 1; continue
            case "{":
                pila.append("{"); out.append(ch); i += 1; continue
            case "[":
                // [:] → {}
                var j = i + 1
                while j < c.count, c[j] == " " { j += 1 }
                if j < c.count, c[j] == ":" {
                    var k = j + 1
                    while k < c.count, c[k] == " " { k += 1 }
                    if k < c.count, c[k] == "]" { out += "{}"; i = k + 1; continue }
                }
                // un '[' pegado a un nombre, ')' o ']' es un subíndice, no un diccionario
                let pegado = out.last.map { $0.isLetter || $0.isNumber || $0 == "_" || $0 == ")" || $0 == "]" || $0 == "`" } ?? false
                if !pegado && esDiccionario(c, desde: i) {
                    // las claves van entre [ ]: {[`ana`]: 30} es JS válido con cualquier expresión
                    pila.append("D"); out += "{["; claveAbierta = true
                } else { pila.append("["); out.append("[") }
                i += 1; continue
            case ")", "}":
                _ = pila.popLast(); out.append(ch); i += 1; continue
            case "]":
                if pila.popLast() == "D" {
                    // coma final ["a": 1,] → quitar la clave que se abrió de más
                    if claveAbierta, out.hasSuffix("[") {
                        out.removeLast()
                        while out.last == " " || out.last == "," { out.removeLast() }
                    }
                    claveAbierta = false
                    out.append("}")
                } else {
                    out.append("]")
                }
                i += 1; continue
            default:
                break
            }
            if pila.last == "D" {
                if ch == ":" && claveAbierta { out += "]:"; claveAbierta = false; i += 1; continue }
                if ch == "," && !claveAbierta { out += ", ["; claveAbierta = true; i += 1; continue }
            }
            // etiqueta: justo después de "(" o "," dentro de una llamada
            if pila.last == "(", ch.isLetter || ch == "_",
               let antes = ultimoSignificativo(), antes == "(" || antes == ",",
               out.last.map({ !$0.isLetter && !$0.isNumber && $0 != "_" }) ?? true {
                var j = i
                while j < c.count, c[j].isLetter || c[j].isNumber || c[j] == "_" { j += 1 }
                var k = j
                while k < c.count, c[k] == " " { k += 1 }
                if k < c.count, c[k] == ":", k + 1 >= c.count || c[k + 1] != ":" {
                    i = k + 1
                    while i < c.count, c[i] == " " { i += 1 }
                    continue
                }
            }
            out.append(ch)
            i += 1
        }
        return out
    }

    /// ¿Hay un ':' a nivel cero entre este '[' y su ']'?
    static func esDiccionario(_ c: [Character], desde: Int) -> Bool {
        var nivel = 0
        var i = desde + 1
        var ternario = 0
        while i < c.count {
            let ch = c[i]
            if "([{".contains(ch) { nivel += 1 }
            else if ")}".contains(ch) { nivel -= 1 }
            else if ch == "]" { if nivel == 0 { return false }; nivel -= 1 }
            else if ch == "?" && nivel == 0 { ternario += 1 }
            else if ch == ":" && nivel == 0 {
                if ternario > 0 { ternario -= 1 } else { return true }
            }
            i += 1
        }
        return false
    }

    // ---------- lo que JavaScript no trae ----------

    /// Se carga en el motor JS después del preludio de siempre.
    static let preludio = """
    (function(){
      function def(o, n, f){ Object.defineProperty(o, n, { value: f, writable: true, configurable: true, enumerable: false }); }
      function get(o, n, f){ Object.defineProperty(o, n, { get: f, set: function(v){ Object.defineProperty(this, n, { value: v, writable: true, configurable: true, enumerable: true }); }, configurable: true, enumerable: false }); }
      var A = Array.prototype, S = String.prototype, N = Number.prototype;
      [A, S].forEach(function(P){
        get(P, 'count', function(){ return this.length; });
        get(P, 'isEmpty', function(){ return this.length === 0; });
        get(P, 'first', function(){ return this.length ? this[0] : null; });
        get(P, 'last', function(){ return this.length ? this[this.length - 1] : null; });
        def(P, 'contains', function(x){ return typeof x === 'function' ? Array.from(this).some(x) : this.indexOf(x) >= 0; });
        def(P, 'hasPrefix', function(x){ return String(this).startsWith(x); });
        def(P, 'hasSuffix', function(x){ return String(this).endsWith(x); });
        def(P, 'reversed', function(){ return Array.from(this).reverse(); });
        def(P, 'sorted', function(f){ var a = Array.from(this); if (!f) return a.sort(function(x, y){ return x < y ? -1 : (x > y ? 1 : 0); });
          return a.sort(function(x, y){ var r = f(x, y); if (typeof r === 'number') return r; return r ? -1 : (f(y, x) ? 1 : 0); }); });
        def(P, 'enumerated', function(){ return Array.from(this).map(function(x, i){ return [i, x]; }); });
        def(P, 'dropFirst', function(n){ return Array.from(this).slice(n === undefined ? 1 : n); });
        def(P, 'dropLast', function(n){ var a = Array.from(this); return a.slice(0, a.length - (n === undefined ? 1 : n)); });
        def(P, 'prefix', function(n){ return Array.from(this).slice(0, n); });
        def(P, 'suffix', function(n){ var a = Array.from(this); return a.slice(Math.max(0, a.length - n)); });
        def(P, 'randomElement', function(){ return this.length ? this[Math.floor(Math.random() * this.length)] : null; });
        def(P, 'primero', function(f){ var a = Array.from(this); for (var i = 0; i < a.length; i++) if (f(a[i])) return a[i]; return null; });
        def(P, 'ultimo', function(f){ var a = Array.from(this); for (var i = a.length - 1; i >= 0; i--) if (f(a[i])) return a[i]; return null; });
        def(P, 'firstIndex', function(x){ var i = Array.from(this).indexOf(x); return i < 0 ? null : i; });
        def(P, 'reduceSw', function(ini, f){ return Array.from(this).reduce(function(acc, x){ return f(acc, x); }, ini); });
        def(P, 'max', function(){ return this.length ? Array.from(this).reduce(function(a, b){ return b > a ? b : a; }) : null; });
        def(P, 'min', function(){ return this.length ? Array.from(this).reduce(function(a, b){ return b < a ? b : a; }) : null; });
        def(P, 'shuffled', function(){ var a = Array.from(this); for (var i = a.length - 1; i > 0; i--) { var j = Math.floor(Math.random() * (i + 1)); var t = a[i]; a[i] = a[j]; a[j] = t; } return a; });
      });
      def(A, 'append', function(x){ this.push(x); });
      def(A, 'appendAll', function(xs){ for (var i = 0; i < xs.length; i++) this.push(xs[i]); });
      def(A, 'insert', function(x, i){ this.splice(i, 0, x); });
      def(A, 'remove', function(i){ return this.splice(i, 1)[0]; });
      def(A, 'removeLast', function(){ return this.pop(); });
      def(A, 'removeFirst', function(){ return this.shift(); });
      def(A, 'removeAll', function(){ this.length = 0; });
      def(A, 'joined', function(sep){ return this.join(sep === undefined ? '' : sep); });
      def(A, 'swapAt', function(i, j){ var t = this[i]; this[i] = this[j]; this[j] = t; });
      def(S, 'uppercased', function(){ return this.toUpperCase(); });
      def(S, 'lowercased', function(){ return this.toLowerCase(); });
      def(S, 'capitalized', function(){ return this.replace(/(^|\\s)\\S/g, function(c){ return c.toUpperCase(); }); });
      def(S, 'partir', function(sep){ return this.split(sep).filter(function(x){ return x.length > 0; }); });
      def(S, 'trimmed', function(){ return this.trim(); });
      def(S, 'replacingOccurrences', function(a, b){ return this.split(a).join(b); });
      get(S, 'description', function(){ return String(this); });
      get(N, 'description', function(){ return String(this); });
      def(N, 'isMultiple', function(n){ return this % n === 0; });
    })();
    function __repetir(s, n){ return String(s).repeat(n); }
    function __arrayRep(x, n){ var a = []; for (var i = 0; i < n; i++) a.push(x); return a; }
    function __op(o){ return o === '+' ? function(a, b){ return a + b; } : function(a, b){ return a * b; }; }
    function __rango(a, b, cerrado){ var r = []; for (var i = a; cerrado ? i <= b : i < b; i++) r.push(i); r.__desde = a; r.__hasta = b; r.__cerrado = cerrado; return r; }
    function __iter(x){ if (x === null || x === undefined) return []; if (typeof x === 'string' || Array.isArray(x)) return x; if (typeof x[Symbol.iterator] === 'function') return x; return Object.keys(x).map(function(k){ return [k, x[k]]; }); }
    function __pares(x){ if (Array.isArray(x)) return x; return __iter(x); }
    function Int(x){ return __int(x); }
    Int.random = function(r){ return r.__desde + Math.floor(Math.random() * (r.__hasta - r.__desde + (r.__cerrado ? 1 : 0))); };
    Int.max = 9007199254740991; Int.min = -9007199254740991;
    function Double(x){ return __dbl(x); }
    Double.random = function(r){ return r.__desde + Math.random() * (r.__hasta - r.__desde); };
    Double.pi = Math.PI;
    function Bool(x){ return !!x; }
    Bool.random = function(){ return Math.random() < 0.5; };
    function abs(x){ return Math.abs(x); }
    function sqrt(x){ return Math.sqrt(x); }
    function pow(a, b){ return Math.pow(a, b); }
    function floor(x){ return Math.floor(x); }
    function ceil(x){ return Math.ceil(x); }
    function round(x){ return Math.round(x); }
    function sin(x){ return Math.sin(x); }
    function cos(x){ return Math.cos(x); }
    function max(){ var a = arguments.length === 1 && Array.isArray(arguments[0]) ? arguments[0] : Array.from(arguments); return Math.max.apply(null, a); }
    function min(){ var a = arguments.length === 1 && Array.isArray(arguments[0]) ? arguments[0] : Array.from(arguments); return Math.min.apply(null, a); }
    function readLine(){ return null; }
    """
}
