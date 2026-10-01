import Foundation

// ============================================================
// MARK: - Plantillas por idioma (comando 'new')
// ============================================================

extension Shell {

    static let plantillas: [String: (String, String)] = [
        "swift": ("Swift (subconjunto)", """
        // Swift interpretado: funciones, bucles, arrays y cadenas.
        // No admite struct, class, enum ni SwiftUI.

        func saluda(nombre: String) -> String {
            return "hola, \\(nombre)"
        }

        let gente = ["Ana", "Luis", "Sam"]
        for p in gente {
            print(saluda(nombre: p))
        }
        """),

        "js": ("JavaScript completo", """
        // JavaScript de verdad, con acceso a archivos y red.
        // ARGV son los argumentos, STDIN lo que llega por la tubería.

        const archivos = listDir(".");
        print("hay " + archivos.length + " archivos aquí");

        // const datos = http.json("https://api.github.com/users/apple");
        // print(datos.name);
        """),

        "sh": ("guion de la terminal", """
        # Un comando por línea. Se ejecuta con: sh archivo.sh
        echo "empezando"
        ls -l
        date
        echo "listo"
        """),

        "json": ("datos JSON", """
        {
          "nombre": "ejemplo",
          "version": 1,
          "etiquetas": ["a", "b"]
        }
        """),

        "xml": ("documento XML", """
        <?xml version="1.0" encoding="UTF-8"?>
        <raiz>
            <elemento id="1">primero</elemento>
            <elemento id="2">segundo</elemento>
        </raiz>
        """),

        "plist": ("lista de propiedades", """
        <?xml version="1.0" encoding="UTF-8"?>
        <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
        <plist version="1.0">
        <dict>
            <key>nombre</key>
            <string>ejemplo</string>
            <key>activo</key>
            <true/>
        </dict>
        </plist>
        """),

        "csv": ("tabla CSV", """
        nombre,edad,ciudad
        Ana,31,Bogotá
        Luis,27,Madrid
        """),

        "md": ("texto Markdown", """
        # Título

        Un párrafo de ejemplo.

        - primer punto
        - segundo punto
        """),

        "esc": ("programa en lenguaje Simulacro", """
        # programa .esc
        set nombre=mundo
        Tex/hola $nombre
        repeat 3
          Tex/vuelta $i
        end
        """),

        "sim": ("línea de Simulacro por renglón", """
        # una línea de Simulacro por renglón
        Tex/empezando
        Sys/
        """),

        "txt": ("texto plano", "escribe aquí\n")
    ]

    static func plantillasCmd() -> [String: Spec] {
        var c: [String: Spec] = [:]

        c["new"] = Spec(help: "new <archivo.ext> — crea un archivo con plantilla y lo abre en el editor") { ctx in
            guard let p = ctx.args.first else {
                let langs = Shell.plantillas.sorted { $0.key < $1.key }
                    .map { "  .\($0.key.padding(toLength: 6, withPad: " ", startingAt: 0)) \($0.value.0)" }
                    .joined(separator: "\n")
                return "uso: new archivo.ext\n\nplantillas disponibles:\n\(langs)\n"
            }
            let u = try ctx.env.resolve(p)
            guard !ctx.env.exists(u) else { throw ShErr("new: \(p) ya existe — usa 'nano \(p)'") }
            let ext = u.pathExtension.lowercased()
            let cuerpo = Shell.plantillas[ext]?.1 ?? "\n"
            try cuerpo.write(to: u, atomically: true, encoding: .utf8)
            ctx.sh.uiEdit?(u)
            let como = ["swift", "js", "sh"].contains(ext) ? "ejecútalo con: run \(p)  (o ^R en el editor)\n" : ""
            return "creado \(p)\n" + como
        }

        return c
    }
}

