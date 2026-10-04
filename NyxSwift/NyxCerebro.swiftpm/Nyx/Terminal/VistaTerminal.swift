#if canImport(SwiftUI)
import Foundation
import SwiftUI

// VistaTerminal.swift — la pantalla de la terminal (estilo Termux):
// fondo negro, letra de ancho fijo, barra de teclas extra (↑ ↓ Tab ⏹ | > ~ / -),
// historial, autocompletar con Tab y el editor «nano».

@MainActor
final class TerminalModelo: ObservableObject {
    @Published var lineas: [LineaTerminal] = []
    @Published var texto = ""
    @Published var ocupada = false
    @Published var editando = false
    @Published var archivoEditado = ""
    @Published var contenidoEditor = ""
    @Published var avisoEditor = ""
    let consola = Consola()
    private var contador = 0
    private var posHistorial: Int? = nil

    init() {
        consola.alEscribir = { [weak self] (t: String, tipo: LineaTerminal.Tipo) in self?.agrega(t, tipo) }
        consola.alLimpiar = { [weak self] in self?.lineas = [] }
        consola.alEditar = { [weak self] (ruta: String) in self?.abreEditor(ruta) }
        agrega("NyxOS 2.0 · nyxsh — la terminal de Nyx (una sola mente)\nEscribe «help» para ver las órdenes. Tu casa es ~ (/home/nyx).\n\n", .aviso)
    }

    func conecta(_ cerebro: CerebroTerminal) {
        consola.cerebro = cerebro
    }

    var prompt: String { consola.prompt }

    /// Pone texto en la pantalla (sigue la última línea si no había terminado).
    func agrega(_ t: String, _ tipo: LineaTerminal.Tipo) {
        var partes = t.components(separatedBy: "\n")
        let terminaEnSalto = t.hasSuffix("\n")
        if terminaEnSalto { partes.removeLast() }
        for (k, p) in partes.enumerated() {
            let abierta = k == partes.count - 1 && !terminaEnSalto
            if k == 0, let u = lineas.last, u.abierta, u.tipo == tipo {
                lineas[lineas.count - 1].texto += p
                lineas[lineas.count - 1].abierta = abierta
                continue
            }
            contador += 1
            lineas.append(LineaTerminal(id: contador, texto: p, tipo: tipo, abierta: abierta))
        }
        if lineas.count > 3000 { lineas.removeFirst(lineas.count - 3000) }
    }

    func envia() {
        if ocupada { return }
        let l = texto
        texto = ""
        posHistorial = nil
        ocupada = true
        Task { @MainActor in
            await self.consola.ejecuta(l)
            self.ocupada = false
        }
    }

    func para() {
        consola.para()
        agrega("^C\n", .aviso)
    }

    func historial(_ arriba: Bool) {
        let h = consola.historial
        if h.isEmpty { return }
        var p = posHistorial ?? h.count
        p = arriba ? max(0, p - 1) : min(h.count, p + 1)
        posHistorial = p
        texto = p < h.count ? h[p] : ""
    }

    /// Tab: completa la orden o el nombre de archivo.
    func completa() {
        let palabras = texto.split(separator: " ", omittingEmptySubsequences: false).map(String.init)
        guard let ultima = palabras.last else { return }
        var opciones: [String] = []
        if palabras.count == 1 {
            opciones = (Array(Consola.ordenes) + Array(consola.alias.keys)).filter { $0.hasPrefix(ultima) }.sorted()
        } else {
            let carpeta = ultima.contains("/") ? String(ultima[..<ultima.lastIndex(of: "/")!]) : ""
            let inicio = ultima.contains("/") ? String(ultima[ultima.index(after: ultima.lastIndex(of: "/")!)...]) : ultima
            let base = carpeta.isEmpty ? "." : (carpeta.isEmpty ? "/" : carpeta)
            for n in consola.lista(base.isEmpty ? "/" : base) where n.hasPrefix(inicio) && (!n.hasPrefix(".") || inicio.hasPrefix(".")) {
                let completo = (carpeta.isEmpty ? "" : carpeta + "/") + n
                opciones.append(consola.esCarpeta(completo) ? completo + "/" : completo)
            }
        }
        if opciones.isEmpty { return }
        var comun = opciones[0]
        for o in opciones.dropFirst() {
            while !o.hasPrefix(comun) { comun.removeLast() }
        }
        let antes = palabras.dropLast().joined(separator: " ")
        if opciones.count == 1 {
            texto = (antes.isEmpty ? "" : antes + " ") + comun + (comun.hasSuffix("/") ? "" : " ")
        } else {
            if comun.count > ultima.count { texto = (antes.isEmpty ? "" : antes + " ") + comun }
            agrega(prompt + texto + "\n" + opciones.map { ($0 as NSString).lastPathComponent + ($0.hasSuffix("/") ? "/" : "") }.joined(separator: "  ") + "\n", .aviso)
        }
    }

    // MARK: editor (nano)

    func abreEditor(_ ruta: String) {
        archivoEditado = ruta
        contenidoEditor = consola.lee(ruta) ?? ""
        avisoEditor = ""
        editando = true
    }

    func guardaEditor() {
        if consola.escribeArchivo(archivoEditado, contenidoEditor) {
            avisoEditor = "guardado ✓"
        } else {
            avisoEditor = "no se pudo guardar"
        }
    }

    func ejecutaEditado() {
        guardaEditor()
        editando = false
        texto = "run " + (archivoEditado as NSString).lastPathComponent
        let carpeta = (archivoEditado as NSString).deletingLastPathComponent
        if carpeta != consola.cwd { texto = "run " + archivoEditado }
        envia()
    }
}

struct PantallaTerminal: View {
    @ObservedObject var nyx: NyxModelo
    @StateObject private var t = TerminalModelo()

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            SalidaTerminal(t: t)
            EntradaTerminal(t: t)
            TeclasExtra(t: t)
        }
        .background(Color.black)
        .onAppear { t.conecta(nyx) }
        .sheet(isPresented: $t.editando) { EditorTerminal(t: t) }
    }
}

struct SalidaTerminal: View {
    @ObservedObject var t: TerminalModelo

    var body: some View {
        ScrollViewReader { (proxy: ScrollViewProxy) in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 1) {
                    ForEach(t.lineas) { (l: LineaTerminal) in
                        FilaTerminal(l: l)
                    }
                }
                .padding(8)
            }
            .onChange(of: t.lineas.count) { (_: Int, _: Int) in
                if let u = t.lineas.last { proxy.scrollTo(u.id, anchor: .bottom) }
            }
        }
    }
}

struct FilaTerminal: View {
    let l: LineaTerminal

    var color: Color {
        switch l.tipo {
        case .normal: return .white
        case .error: return .red
        case .orden: return .green
        case .aviso: return .cyan
        }
    }

    var body: some View {
        Text(l.texto.isEmpty ? " " : l.texto)
            .font(.system(size: 14, design: .monospaced))
            .foregroundColor(color)
            .id(l.id)
    }
}

struct EntradaTerminal: View {
    @ObservedObject var t: TerminalModelo

    var body: some View {
        HStack(spacing: 4) {
            Text(t.ocupada ? "⏳ " : t.prompt)
                .font(.system(size: 14, design: .monospaced))
                .foregroundColor(.green)
            TextField("", text: $t.texto)
                .font(.system(size: 14, design: .monospaced))
                .foregroundColor(.white)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled(true)
                .onSubmit { t.envia() }
        }
        .padding(8)
    }
}

/// La barra de teclas extra de Termux.
struct TeclasExtra: View {
    @ObservedObject var t: TerminalModelo

    let simbolos = ["|", ">", "<", "~", "/", "-", "*", "$", "&", "'", "\"", ";"]

    var body: some View {
        VStack(spacing: 4) {
            HStack(spacing: 6) {
                TeclaTerminal(texto: "⏹ Ctrl-C") { t.para() }
                TeclaTerminal(texto: "↑") { t.historial(true) }
                TeclaTerminal(texto: "↓") { t.historial(false) }
                TeclaTerminal(texto: "Tab") { t.completa() }
                TeclaTerminal(texto: "clear") { t.lineas = [] }
                TeclaTerminal(texto: "⏎") { t.envia() }
            }
            HStack(spacing: 6) {
                ForEach(simbolos, id: \.self) { (s: String) in
                    TeclaTerminal(texto: s) { t.texto += s }
                }
            }
        }
        .padding(6)
    }
}

struct TeclaTerminal: View {
    let texto: String
    let accion: () -> Void

    var body: some View {
        Button(texto) { accion() }
            .font(.system(size: 15, design: .monospaced))
            .buttonStyle(.bordered)
    }
}

/// El editor (nano/edit): escribe tu programa y guárdalo o ejecútalo.
struct EditorTerminal: View {
    @ObservedObject var t: TerminalModelo

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("✏️ " + t.archivoEditado).font(.system(size: 15, design: .monospaced))
                Text(t.avisoEditor).font(.caption).foregroundColor(.green)
            }
            TextEditor(text: $t.contenidoEditor)
                .font(.system(size: 14, design: .monospaced))
                .autocorrectionDisabled(true)
                .textInputAutocapitalization(.never)
            HStack {
                Button("💾 Guardar") { t.guardaEditor() }
                    .buttonStyle(.borderedProminent)
                Button("▶️ Guardar y ejecutar") { t.ejecutaEditado() }
                    .buttonStyle(.bordered)
                Button("Salir") { t.editando = false }
                    .buttonStyle(.bordered)
            }
        }
        .padding()
    }
}
#endif
