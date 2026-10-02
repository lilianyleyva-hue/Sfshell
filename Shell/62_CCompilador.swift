import Foundation

// ============================================================
// MARK: - C, C++ y Java: compilar y ejecutar
// ============================================================
// compila(): preprocesa, analiza y resuelve todo el programa, y da
// los errores con su línea (como gcc o javac). ejecuta(): corre main.

final class CCompilado: @unchecked Sendable {
    let prog: CPrograma
    let maq: CMaq
    let principal: CFuncion
    let clasePrincipal: CClase?
    let fuentes: [String: String]
    let archivo: String

    init(prog: CPrograma, maq: CMaq, principal: CFuncion, clasePrincipal: CClase?, fuentes: [String: String], archivo: String) {
        self.prog = prog
        self.maq = maq
        self.principal = principal
        self.clasePrincipal = clasePrincipal
        self.fuentes = fuentes
        self.archivo = archivo
    }
}

struct CResultado: Sendable {
    let codigo: Int
    let error: String?
}

enum CCompilador {

    static let excepcionesJava: [(String, String)] = [
        ("Exception", "Throwable"), ("Error", "Throwable"), ("RuntimeException", "Exception"),
        ("ArithmeticException", "RuntimeException"), ("IndexOutOfBoundsException", "RuntimeException"),
        ("ArrayIndexOutOfBoundsException", "IndexOutOfBoundsException"),
        ("StringIndexOutOfBoundsException", "IndexOutOfBoundsException"), ("NullPointerException", "RuntimeException"),
        ("IllegalArgumentException", "RuntimeException"), ("NumberFormatException", "IllegalArgumentException"),
        ("IllegalStateException", "RuntimeException"), ("ClassCastException", "RuntimeException"),
        ("NegativeArraySizeException", "RuntimeException"), ("UnsupportedOperationException", "RuntimeException"),
        ("NoSuchElementException", "RuntimeException"), ("InputMismatchException", "NoSuchElementException"),
        ("ConcurrentModificationException", "RuntimeException"), ("EmptyStackException", "RuntimeException"),
        ("ArrayStoreException", "RuntimeException"), ("PatternSyntaxException", "IllegalArgumentException"),
        ("IllegalFormatConversionException", "IllegalArgumentException"), ("UnknownFormatConversionException", "IllegalArgumentException"),
        ("InterruptedException", "Exception"), ("CloneNotSupportedException", "Exception"),
        ("IOException", "Exception"), ("FileNotFoundException", "IOException"), ("UncheckedIOException", "RuntimeException"),
        ("StackOverflowError", "Error"), ("OutOfMemoryError", "Error"), ("AssertionError", "Error")
    ]

    static var preludioJava: String {
        var s = """
        class Throwable {
            String message;
            Throwable cause;
            Throwable() {}
            Throwable(String message) { this.message = message; }
            Throwable(String message, Throwable cause) { this.message = message; this.cause = cause; }
            Throwable(Throwable cause) { this.cause = cause; this.message = cause == null ? null : cause.toString(); }
            String getMessage() { return message; }
            String getLocalizedMessage() { return message; }
            Throwable getCause() { return cause; }
            public String toString() {
                String n = this.getClass().getName();
                return message == null ? n : n + ": " + message;
            }
            void printStackTrace() { System.err.println(this.toString()); System.err.println("\\tat Main.main(Main.java)"); }
        }

        """
        for (n, p) in excepcionesJava {
            s += "class \(n) extends \(p) { \(n)() {} \(n)(String m) { super(m); } \(n)(String m, Throwable c) { super(m, c); } \(n)(Throwable c) { super(c); } }\n"
        }
        return s
    }

    static let preludioCpp = """
    class exception {
    public:
        exception() {}
        virtual const char* what() const { return "std::exception"; }
    };
    class runtime_error : public exception {
    public:
        string _m;
        runtime_error(string m) { _m = m; }
        virtual const char* what() const { return _m.c_str(); }
    };
    class logic_error : public exception {
    public:
        string _m;
        logic_error(string m) { _m = m; }
        virtual const char* what() const { return _m.c_str(); }
    };
    class out_of_range : public logic_error { public: out_of_range(string m) : logic_error(m) {} };
    class invalid_argument : public logic_error { public: invalid_argument(string m) : logic_error(m) {} };
    class length_error : public logic_error { public: length_error(string m) : logic_error(m) {} };
    class domain_error : public logic_error { public: domain_error(string m) : logic_error(m) {} };
    class overflow_error : public runtime_error { public: overflow_error(string m) : runtime_error(m) {} };
    class underflow_error : public runtime_error { public: underflow_error(string m) : runtime_error(m) {} };
    class range_error : public runtime_error { public: range_error(string m) : runtime_error(m) {} };
    class bad_alloc : public exception { public: bad_alloc() {} virtual const char* what() const { return "std::bad_alloc"; } };

    """

    static func dialectoDe(_ archivo: String) -> CDialecto? {
        switch (archivo as NSString).pathExtension.lowercased() {
        case "c", "h": return .c
        case "cpp", "cc", "cxx", "c++", "hpp", "hh", "ino": return .cpp
        case "java": return .java
        default: return nil
        }
    }

    /// Mensaje de error como el de gcc: archivo:línea: error: … con la línea marcada.
    static func mensaje(_ e: CFallo, _ archivo: String, _ fuente: String?) -> String {
        var s = "\(archivo):\(e.linea): error: \(e.msg)"
        if let f = fuente, e.linea > 0 {
            let lineas = f.components(separatedBy: "\n")
            if e.linea <= lineas.count {
                let l = lineas[e.linea - 1]
                s += "\n" + String(format: "%5d", e.linea) + " | " + l
            }
        }
        return s
    }

    static func compila(_ fuentes: [(String, String)], dialecto: CDialecto, claseMain: String? = nil, entrada: CEntrada) throws -> CCompilado {
        guard let (archivo, principalSrc) = fuentes.first else { throw ShErr("no hay nada que compilar") }
        var mapa: [String: String] = [:]
        for (n, s) in fuentes { mapa[n] = s; mapa[(n as NSString).lastPathComponent] = s }
        let prog = CPrograma()
        var archivoActual = archivo
        do {
            // la biblioteca escrita en el propio lenguaje (excepciones)
            if dialecto != .c {
                let pre = dialecto == .java ? preludioJava : preludioCpp
                let toks = try CPreproc(dialecto: dialecto, leer: { _ in nil }).procesa(pre, archivo: "<biblioteca>")
                let p = CParser(toks, dialecto: dialecto, prog: prog)
                try p.programa()
                for c in prog.ordenClases { c.nativa = true }
            }
            for (n, src) in fuentes {
                if dialecto != .java && n != archivo && (n.hasSuffix(".h") || n.hasSuffix(".hpp")) { continue }
                archivoActual = n
                let pre = CPreproc(dialecto: dialecto, leer: { mapa[$0] })
                let toks = try pre.procesa(src, archivo: n)
                prog.cabeceras.formUnion(pre.cabeceras)
                let p = CParser(toks, dialecto: dialecto, prog: prog)
                try p.programa()
            }
            archivoActual = archivo
            let m = CMaq(dialecto: dialecto, prog: prog, entrada: entrada)
            try resuelve(m, prog)
            let (fn, clase) = try buscaMain(prog, dialecto, claseMain)
            return CCompilado(prog: prog, maq: m, principal: fn, clasePrincipal: clase, fuentes: mapa, archivo: archivo)
        } catch let e as CFallo {
            throw ShErr(mensaje(e, archivoActual, mapa[archivoActual] ?? principalSrc))
        }
    }

    static func resuelve(_ m: CMaq, _ prog: CPrograma) throws {
        for c in prog.ordenClases { try m.preparaClase(c) }
        let r = CResol(m)
        for s in prog.globales { try s.res(r) }
        let nombres = prog.funciones.keys.sorted()
        for n in nombres {
            for fn in prog.funciones[n] ?? [] { try r.resuelveFuncion(fn) }
        }
        for c in prog.ordenClases where !c.nombre.hasPrefix("$Anonima") {
            for (_, lista) in c.metodos { for fn in lista { fn.clase = c; try r.resuelveFuncion(fn) } }
            for fn in c.ctors { fn.clase = c; try r.resuelveFuncion(fn) }
            if let d = c.dtor { d.clase = c; try r.resuelveFuncion(d) }
            if let fn = m.iniCampos[c.id] { try r.resuelveFuncion(fn) }
            if let fn = m.iniEstaticos[c.id] { try r.resuelveFuncion(fn) }
            try metodosEnum(m, c)
        }
    }

    /// values() y valueOf() de los enum de Java.
    static func metodosEnum(_ m: CMaq, _ c: CClase) throws {
        guard c.esEnum, m.dialecto == .java else { return }
        let values = CFuncion("values", params: [], ret: .arr(.clase(c.nombre), -1), linea: c.linea)
        values.estatico = true
        values.clase = c
        values.nat = { maq, _ in
            try maq.preparaEstaticos(c)
            let mem = CMem(c.enumValores.map { CV.o($0) })
            mem.elem = .clase(c.nombre)
            mem.esArreglo = true
            return .p(CPtr(mem: mem, i: 0))
        }
        let valueOf = CFuncion("valueOf", params: [CParam(nombre: "s", tipo: .cad, defecto: nil)], ret: .clase(c.nombre), linea: c.linea)
        valueOf.estatico = true
        valueOf.clase = c
        valueOf.nat = { maq, a in
            try maq.preparaEstaticos(c)
            let s = try maq.texto(a.first ?? .nulo, 0)
            for o in c.enumValores {
                if let i = c.indice["$nombre"], case .s(let u) = o.c.a[i], maq.texto(u) == s { return .o(o) }
            }
            throw maq.excepcion("IllegalArgumentException", "No enum constant \(c.nombre).\(s)", 0)
        }
        if c.metodos["values"] == nil { c.metodos["values"] = [values] }
        if c.metodos["valueOf"] == nil { c.metodos["valueOf"] = [valueOf] }
    }

    static func buscaMain(_ prog: CPrograma, _ d: CDialecto, _ claseMain: String?) throws -> (CFuncion, CClase?) {
        if d != .java {
            guard let fn = prog.funciones["main"]?.first(where: { $0.cuerpo != nil }) else {
                throw CFallo("falta la función main (int main() { … })", 0)
            }
            return (fn, nil)
        }
        if let n = claseMain {
            guard let c = prog.clases[n] else { throw CFallo("no encuentro la clase \(n)", 0) }
            guard let fn = c.metodos["main"]?.first(where: { $0.estatico }) else {
                throw CFallo("la clase \(n) no tiene public static void main(String[] args)", 0)
            }
            return (fn, c)
        }
        for c in prog.ordenClases where !c.nativa {
            if let fn = c.metodos["main"]?.first(where: { $0.estatico }) { return (fn, c) }
        }
        throw CFallo("no hay ninguna clase con public static void main(String[] args)", 0)
    }

    /// Ejecuta main y devuelve el código de salida (y el error, si lo hubo).
    static func ejecuta(_ c: CCompilado, argv: [String]) -> CResultado {
        let m = c.maq
        var codigo = 0
        var error: String?
        do {
            if m.dialecto == .java {
                if let cl = c.clasePrincipal { try m.preparaEstaticos(cl) }
                let args = CMem(argv.map { m.cadena($0) })
                args.elem = .cad
                args.esArreglo = true
                _ = try m.llama(c.principal, [.p(CPtr(mem: args, i: 0))], este: nil, padre: nil, linea: c.principal.linea)
            } else {
                let global = CMarco(0, padre: nil, este: nil)
                for s in c.prog.globales { _ = try s.exec(m, global) }
                var args: [CV] = []
                let todos = [m.nombrePrograma] + argv
                if c.principal.params.count >= 1 { args.append(.n(Int64(todos.count), .int)) }
                if c.principal.params.count >= 2 {
                    var celdas: [CV] = todos.map { m.nuevaCadenaC($0) }
                    celdas.append(.ptrNulo)
                    let mem = CMem(celdas)
                    mem.elem = .ptr(.num(.char))
                    args.append(.p(CPtr(mem: mem, i: 0)))
                }
                let r = try m.llama(c.principal, args, este: nil, padre: nil, linea: c.principal.linea)
                if case .n(let x, _) = r { codigo = Int(x & 0xFF) }
            }
        } catch let s as CSalida {
            codigo = s.codigo & 0xFF
        } catch let e as CLanzado {
            codigo = m.dialecto == .java ? 1 : 134
            error = mensajeExcepcion(m, e)
        } catch let e as CFallo {
            codigo = 1
            if e.msg == "interrumpido" { error = "^C" }
            else if m.dialecto == .java {
                error = "Exception in thread \"main\" java.lang.Error: \(e.msg)" + (e.linea > 0 ? "\n\tat \(c.clasePrincipal?.nombre ?? "Main").main(\(c.archivo):\(e.linea))" : "")
            } else {
                error = "error en tiempo de ejecución" + (e.linea > 0 ? " (línea \(e.linea))" : "") + ": \(e.msg)"
            }
        } catch let otro {
            codigo = 1
            error = "\(otro)"
        }
        try? m.muestraLienzo()
        m.cierraTodo()
        m.vuelca()
        return CResultado(codigo: codigo, error: error)
    }

    static func mensajeExcepcion(_ m: CMaq, _ e: CLanzado) -> String {
        if m.dialecto == .java {
            var desc = (try? m.texto(e.v, e.linea)) ?? "excepción"
            if case .o(let o) = e.v, o.clase.metodo("toString", 0) == nil || o.clase.nativa { desc = (try? m.textoObjeto(o, e.linea)) ?? desc }
            return "Exception in thread \"main\" \(desc)\n\tat Main.main(Main.java:\(e.linea))"
        }
        var tipo = e.v.tipoNombre
        var what = ""
        if case .o(let o) = e.v {
            tipo = (o.clase.nativa ? "std::" : "") + o.clase.nombre
            if let fn = o.clase.metodo("what", 0), let r = try? m.llama(fn, [], este: o, padre: nil, linea: e.linea) {
                what = (try? m.cadenaC(r, e.linea)) ?? ""
            }
        } else if case .p = e.v {
            tipo = "char const*"
        } else if case .s = e.v {
            tipo = "std::string"
        }
        var s = "terminate called after throwing an instance of '\(tipo)'"
        if !what.isEmpty { s += "\n  what():  \(what)" }
        return s + "\n(abortado, línea \(e.linea))"
    }
}
