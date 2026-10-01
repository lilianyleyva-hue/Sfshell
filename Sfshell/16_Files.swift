import SwiftUI

// ============================================================
// MARK: - Explorador de archivos + plantillas por idioma
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

    static func filesUI() -> [String: Spec] {
        var c: [String: Spec] = [:]

        c["files"] = Spec(help: "files — abre el explorador de archivos") { ctx in
            ctx.sh.uiFiles?()
            return ""
        }
        c["filza"] = c["files"]

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

// ============================================================
// MARK: - Vista del explorador
// ============================================================

extension TerminalModel {
    /// Lanza un comando desde la interfaz, como si se hubiera escrito.
    func runFromUI(_ cmd: String) {
        lines.append(Line(kind: .cmd, text: prompt + cmd))
        busy = true
        Task {
            let out = await shell.execute(cmd)
            await MainActor.run {
                self.emit(out)
                self.busy = false
            }
        }
    }
}

/// Una fila del explorador: carpeta o archivo con su menú de acciones.
/// Sacada de FilesView.body — un ForEach con un if/else de Button/Menu (y el
/// Menu con siete botones más) directamente dentro es otro punto donde el
/// comprobador de tipos de SwiftUI puede tardar muchísimo o no terminar.
private struct FileRow: View {
    let url: URL
    let isDir: Bool
    let icono: String
    let peso: String
    let ejecutable: Bool
    let onAbrirCarpeta: () -> Void
    let onNano: () -> Void
    let onEjecutar: () -> Void
    let onVer: () -> Void
    let onDetalles: () -> Void
    let onExportar: () -> Void
    let onRenombrar: () -> Void
    let onBorrar: () -> Void

    var body: some View {
        if isDir {
            Button(action: onAbrirCarpeta) {
                Label(url.lastPathComponent, systemImage: "folder.fill")
            }
        } else {
            Menu {
                menu
            } label: {
                HStack {
                    Image(systemName: icono)
                    Text(url.lastPathComponent)
                    Spacer()
                    Text(peso).foregroundStyle(.secondary).font(.caption)
                }
            }
        }
    }

    @ViewBuilder private var menu: some View {
        Button("Abrir en nano", action: onNano)
        if ejecutable {
            Button("Ejecutar", action: onEjecutar)
        }
        Button("Ver", action: onVer)
        Button("Detalles", action: onDetalles)
        Button("Exportar", action: onExportar)
        Button("Renombrar", action: onRenombrar)
        Button("Borrar", role: .destructive, action: onBorrar)
    }
}

struct FilesView: View {
    @ObservedObject var model: TerminalModel
    @State private var dir: URL
    @State private var items: [URL] = []
    @State private var renaming: URL?
    @State private var nuevoNombre = ""
    @State private var error: String?
    /// Cuando FilesView vive dentro de una ventana del escritorio (no de un
    /// .sheet), no hay nada que 'dismiss()' — así que el botón Cerrar llama
    /// a esto en su lugar, si se lo dan. Si no, se comporta como siempre.
    var onCerrar: (() -> Void)?
    /// nano/run/cat/stat/save necesitan enseñar su resultado en la terminal
    /// de verdad — si estamos en una ventana del escritorio, esto además
    /// saca de ahí, no solo cierra la ventana.
    var onIrATerminal: (() -> Void)?

    init(model: TerminalModel, onCerrar: (() -> Void)? = nil, onIrATerminal: (() -> Void)? = nil) {
        self.model = model
        self.onCerrar = onCerrar
        self.onIrATerminal = onIrATerminal
        _dir = State(initialValue: model.shell.env.cwd)
    }

    private var fm: FileManager { .default }
    private var env: ShellEnv { model.shell.env }

    private static let ejecutables: Set<String> = ["swift", "js", "sh", "json", "xml", "plist", "csv"]

    var body: some View {
        NavigationStack {
            List {
                if dir.standardizedFileURL.path != env.root.path {
                    Button {
                        dir.deleteLastPathComponent()
                        cargar()
                    } label: {
                        Label("..", systemImage: "arrow.turn.left.up")
                    }
                }

                ForEach(items, id: \.path) { u in
                    FileRow(
                        url: u,
                        isDir: env.isDir(u),
                        icono: icono(u),
                        peso: peso(u),
                        ejecutable: Self.ejecutables.contains(u.pathExtension.lowercased()),
                        onAbrirCarpeta: { dir = u; cargar() },
                        onNano: { accion("nano \(ruta(u))") },
                        onEjecutar: { accion("run \(ruta(u))") },
                        onVer: { accion("cat \(ruta(u))") },
                        onDetalles: { accion("stat \(ruta(u))") },
                        onExportar: { accion("save \(ruta(u))") },
                        onRenombrar: { renaming = u; nuevoNombre = u.lastPathComponent },
                        onBorrar: { borrar(u) }
                    )
                }
                .onDelete { idx in idx.map { items[$0] }.forEach(borrar) }

                if items.isEmpty {
                    Text("carpeta vacía").foregroundStyle(.secondary)
                }
            }
            .navigationTitle(env.vpath(dir))
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cerrar") { cerrar() }
                }
                ToolbarItem(placement: .primaryAction) {
                    Menu {
                        Button("Trabajar aquí") { env.cwd = dir; cerrar() }
                        Button("Nuevo archivo") { nuevo() }
                        Button("Nueva carpeta") { nuevaCarpeta() }
                        Button("Importar de Archivos") { accion("pick"); cerrar() }
                        Button("Actualizar") { cargar() }
                    } label: {
                        Image(systemName: "ellipsis.circle")
                    }
                }
            }
            .alert("Renombrar", isPresented: Binding(get: { renaming != nil }, set: { if !$0 { renaming = nil } })) {
                TextField("nombre", text: $nuevoNombre)
                Button("Cancelar", role: .cancel) { renaming = nil }
                Button("Guardar") {
                    if let u = renaming {
                        try? fm.moveItem(at: u, to: u.deletingLastPathComponent().appendingPathComponent(nuevoNombre))
                        cargar()
                    }
                    renaming = nil
                }
            }
            .alert("Error", isPresented: Binding(get: { error != nil }, set: { if !$0 { error = nil } })) {
                Button("Vale", role: .cancel) { error = nil }
            } message: {
                Text(error ?? "")
            }
        }
        .onAppear { cargar() }
    }

    // --- acciones ---

    private func cerrar() {
        if let onCerrar { onCerrar() } else { model.showFiles = false }
    }

    private func ruta(_ u: URL) -> String { env.vpath(u) }

    private func cargar() {
        var list = (try? fm.contentsOfDirectory(at: dir, includingPropertiesForKeys: nil)) ?? []
        list.sort {
            let a = env.isDir($0), b = env.isDir($1)
            if a != b { return a }
            return $0.lastPathComponent.lowercased() < $1.lastPathComponent.lowercased()
        }
        items = list
    }

    private func accion(_ cmd: String) {
        env.cwd = dir
        model.runFromUI(cmd)
        if let onIrATerminal { onIrATerminal() } else { model.showFiles = false }
    }

    private func borrar(_ u: URL) {
        do { try fm.removeItem(at: u); cargar() }
        catch { self.error = error.localizedDescription }
    }

    private func nuevo() {
        var n = 1
        var nombre = "nuevo.swift"
        while fm.fileExists(atPath: dir.appendingPathComponent(nombre).path) {
            n += 1; nombre = "nuevo\(n).swift"
        }
        env.cwd = dir
        model.runFromUI("new \(nombre)")
        model.showFiles = false
    }

    private func nuevaCarpeta() {
        var n = 1
        var nombre = "carpeta"
        while fm.fileExists(atPath: dir.appendingPathComponent(nombre).path) {
            n += 1; nombre = "carpeta\(n)"
        }
        try? fm.createDirectory(at: dir.appendingPathComponent(nombre), withIntermediateDirectories: true)
        cargar()
    }

    private func peso(_ u: URL) -> String {
        let at = (try? fm.attributesOfItem(atPath: u.path)) ?? [:]
        return humanSize((at[.size] as? NSNumber)?.intValue ?? 0)
    }

    private func icono(_ u: URL) -> String {
        switch u.pathExtension.lowercased() {
        case "swift": return "swift"
        case "js": return "curlybraces"
        case "json", "xml", "plist": return "doc.badge.gearshape"
        case "sh": return "terminal"
        case "csv": return "tablecells"
        case "md", "txt": return "doc.text"
        case "deb": return "shippingbox"
        default: return "doc"
        }
    }
}
