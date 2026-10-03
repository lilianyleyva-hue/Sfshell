#if canImport(SwiftUI)
import SwiftUI

// Pantallas.swift — la interfaz: Hablar · En vivo · Mentes · Transferencias · Enseñar.
// Cada vista es pequeña a propósito: así el compilador del iPad va rápido.

struct VistaPrincipal: View {
    @ObservedObject var nyx: NyxModelo

    var body: some View {
        TabView {
            PantallaHablar(nyx: nyx)
                .tabItem { Label("Hablar", systemImage: "bubble.left.and.bubble.right") }
            PantallaVivo(nyx: nyx)
                .tabItem { Label("En vivo", systemImage: "waveform") }
            PantallaMentes(nyx: nyx)
                .tabItem { Label("Mentes", systemImage: "brain") }
            PantallaTransferencias(nyx: nyx)
                .tabItem { Label("Transferencias", systemImage: "arrow.left.arrow.right") }
            PantallaEnsenar(nyx: nyx)
                .tabItem { Label("Enseñar", systemImage: "book") }
        }
        .preferredColorScheme(.dark)
    }
}

// MARK: - Hablar

struct PantallaHablar: View {
    @ObservedObject var nyx: NyxModelo
    @State private var texto = ""
    @State private var destino = -1

    var body: some View {
        VStack(spacing: 0) {
            Text(nyx.aviso).font(.caption).foregroundColor(.gray).padding(6)
            ListaChat(nyx: nyx)
            BarraOpinion(nyx: nyx)
            BarraEscribir(nyx: nyx, texto: $texto, destino: $destino)
        }
    }
}

struct ListaChat: View {
    @ObservedObject var nyx: NyxModelo

    var body: some View {
        ScrollViewReader { (proxy: ScrollViewProxy) in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 10) {
                    ForEach(nyx.chat) { (m: MensajeChat) in
                        Burbuja(m: m)
                    }
                    Color.clear.frame(height: 1).id("fin")
                }
                .padding()
            }
            .onChange(of: nyx.chat.count) { (_: Int) in proxy.scrollTo("fin") }
        }
    }
}

struct Burbuja: View {
    let m: MensajeChat

    var nombre: String {
        if m.deHumano { return "tú" }
        if m.rol >= 0 && m.rol < numRoles { return Roles.todos[m.rol].nombre }
        return "nyx"
    }

    /// Resh y detalles (vacío en tus mensajes).
    var extra: String {
        var partes: [String] = []
        if !m.resh.isEmpty { partes.append("resh: " + m.resh) }
        if !m.detalle.isEmpty { partes.append(m.detalle) }
        return partes.joined(separator: "\n")
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 3) {
            Text(nombre).font(.caption.bold()).foregroundColor(colorDe(m.rol))
            Text(m.texto).font(.body)
            Text(extra).font(.caption).foregroundColor(.gray)
        }
        .padding(10)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(m.deHumano ? Color.blue.opacity(0.25) : Color.white.opacity(0.07))
        .cornerRadius(12)
    }
}

struct BarraOpinion: View {
    @ObservedObject var nyx: NyxModelo

    var body: some View {
        HStack {
            Text("¿Te gustó la respuesta?").font(.caption).foregroundColor(.gray)
            Spacer()
            Button("👍 bien") { nyx.opina(true) }
            Button("👎 mal") { nyx.opina(false) }
        }
        .padding(.horizontal)
        .padding(.vertical, 4)
        .disabled(!nyx.puedeOpinar)
    }
}

struct BarraEscribir: View {
    @ObservedObject var nyx: NyxModelo
    @Binding var texto: String
    @Binding var destino: Int

    var titulo: String {
        destino < 0 ? "las 18" : Roles.todos[destino].nombre
    }

    var body: some View {
        HStack(spacing: 8) {
            MenuDestino(destino: $destino, titulo: titulo)
            TextField("Pregunta algo…", text: $texto)
                .textFieldStyle(.roundedBorder)
                .autocorrectionDisabled()
                .onSubmit { envia() }
            Button("Enviar") { envia() }
                .buttonStyle(.borderedProminent)
        }
        .padding()
    }

    func envia() {
        nyx.pregunta(texto, a: destino)
        texto = ""
    }
}

struct MenuDestino: View {
    @Binding var destino: Int
    let titulo: String

    var body: some View {
        Menu {
            Button("Las 18 (deliberan)") { destino = -1 }
            ForEach(0 ..< numRoles, id: \.self) { (r: Int) in
                Button(Roles.todos[r].nombre) { destino = r }
            }
        } label: {
            Label(titulo, systemImage: "person.2")
        }
    }
}

// MARK: - En vivo

struct PantallaVivo: View {
    @ObservedObject var nyx: NyxModelo
    @State private var a = 13
    @State private var b = 17

    var body: some View {
        VStack(spacing: 0) {
            ControlesVivo(nyx: nyx)
            ControlesConversa(nyx: nyx, a: $a, b: $b)
            ListaEventos(nyx: nyx)
        }
    }
}

struct ControlesVivo: View {
    @ObservedObject var nyx: NyxModelo

    var body: some View {
        HStack {
            Button(nyx.vivo ? "■ Parar" : "▶ Empezar") { nyx.alternaVivo() }
                .buttonStyle(.borderedProminent)
            Button("Un paso") { nyx.unPaso() }
                .buttonStyle(.bordered)
                .disabled(nyx.vivo)
            Spacer()
            Text("ritmo").font(.caption).foregroundColor(.gray)
            Slider(value: $nyx.pausa, in: 0.05 ... 2.0)
                .frame(maxWidth: 180)
        }
        .padding()
    }
}

struct ControlesConversa: View {
    @ObservedObject var nyx: NyxModelo
    @Binding var a: Int
    @Binding var b: Int

    var body: some View {
        HStack {
            Picker("", selection: $a) {
                ForEach(0 ..< numRoles, id: \.self) { (r: Int) in Text(Roles.todos[r].nombre).tag(r) }
            }
            Text("y").foregroundColor(.gray)
            Picker("", selection: $b) {
                ForEach(0 ..< numRoles, id: \.self) { (r: Int) in Text(Roles.todos[r].nombre).tag(r) }
            }
            Button("Que conversen") { nyx.conversa(a, b) }
                .buttonStyle(.bordered)
            Spacer()
        }
        .padding(.horizontal)
    }
}

struct ListaEventos: View {
    @ObservedObject var nyx: NyxModelo

    var body: some View {
        List {
            ForEach(nyx.eventos.reversed()) { (e: Evento) in
                FilaEvento(e: e)
            }
        }
        .listStyle(.plain)
    }
}

struct FilaEvento: View {
    let e: Evento

    var body: some View {
        HStack(alignment: .top, spacing: 6) {
            Text(Roles.todos[e.rol].nombre)
                .font(.caption.bold())
                .foregroundColor(colorDe(e.rol))
                .frame(width: 90, alignment: .leading)
            Text(e.tipo.icono)
            Text(e.texto)
                .font(.callout)
                .foregroundColor(e.tipo == .aprende || e.tipo == .nota ? .gray : .primary)
        }
    }
}

// MARK: - Mentes

struct PantallaMentes: View {
    @ObservedObject var nyx: NyxModelo

    var body: some View {
        NavigationStack {
            List {
                ForEach(nyx.filas()) { (f: FilaMente) in
                    NavigationLink(destination: DetalleMente(nyx: nyx, rol: f.id)) {
                        FilaMenteVista(f: f)
                    }
                }
            }
            .navigationTitle("Las 18 mentes")
        }
    }
}

struct FilaMenteVista: View {
    let f: FilaMente

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text(f.nombre).bold().foregroundColor(colorDe(f.id))
                Spacer()
                Text("le gusta: " + f.gusta).font(.caption).foregroundColor(.gray)
            }
            ProgressView(value: f.precision)
                .tint(colorDe(f.id))
            Text("precisión \(Int(f.precision * 100))% · \(f.ideas) ideas · \(f.resh) palabras en Resh · dijo \(f.dichos)")
                .font(.caption2)
                .foregroundColor(.gray)
        }
        .padding(.vertical, 4)
    }
}

struct DetalleMente: View {
    @ObservedObject var nyx: NyxModelo
    let rol: Int

    var body: some View {
        List {
            Section("Quiere") {
                Text(Roles.todos[rol].objetivo)
            }
            Section("Lo que le gusta hacer (lo aprendió)") {
                ForEach(nyx.gustos(rol)) { (g: Gusto) in
                    FilaGusto(nombre: g.nombre, valor: g.valor, veces: g.veces, color: colorDe(rol))
                }
            }
            Section("Lo más activo en su mente") {
                ForEach(nyx.activo(rol), id: \.self) { (t: String) in Text(t).font(.callout) }
            }
            Section("En su foco ahora") {
                Text(nyx.foco(rol)).font(.callout)
            }
            Section("Lo último que vivió") {
                ForEach(nyx.recuerdos(rol), id: \.self) { (t: String) in Text(t).font(.caption) }
            }
        }
        .navigationTitle(Roles.todos[rol].nombre)
    }
}

struct FilaGusto: View {
    let nombre: String
    let valor: Double
    let veces: Int
    let color: Color

    var body: some View {
        HStack {
            Text(nombre).frame(width: 90, alignment: .leading)
            ProgressView(value: min(1, max(0, valor / 2)))
                .tint(color)
            Text(fmt(Float(valor))).font(.caption).frame(width: 44)
            Text("\(veces)×").font(.caption).foregroundColor(.gray).frame(width: 40)
        }
    }
}

// MARK: - Transferencias (SwiftData)

struct PantallaTransferencias: View {
    @ObservedObject var nyx: NyxModelo

    var body: some View {
        NavigationStack {
            List {
                Section {
                    Text(nyx.conSwiftData
                         ? "Cada fila es un registro de SwiftData: algo que una mente le pasó a las demás. Las otras lo recogen cuando les toca."
                         : "SwiftData no está disponible: las transferencias van en memoria.")
                        .font(.caption)
                        .foregroundColor(.gray)
                }
                ForEach(nyx.transferencias) { (t: FilaTransferencia) in
                    FilaTransferenciaVista(t: t)
                }
            }
            .navigationTitle("Transferencias")
            .toolbar {
                Button("Actualizar") { nyx.refrescaTransferencias() }
            }
        }
    }
}

struct FilaTransferenciaVista: View {
    let t: FilaTransferencia

    var icono: String {
        return (TipoEnvio(rawValue: t.tipo) ?? .frase).icono
    }

    var destino: String {
        return t.para < 0 ? "todas" : Roles.todos[t.para].nombre
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 3) {
            HStack {
                Text(icono)
                Text(Roles.todos[t.de].nombre).bold().foregroundColor(colorDe(t.de))
                Text("→ " + destino).foregroundColor(.gray)
                Spacer()
                Text("recogida por \(t.recogieron)").font(.caption).foregroundColor(.gray)
            }
            Text(t.contenido).font(.callout)
        }
    }
}

// MARK: - Enseñar

struct PantallaEnsenar: View {
    @ObservedObject var nyx: NyxModelo
    @State private var texto = ""
    @State private var nota = ""
    @State private var confirmar = false

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Enséñales").font(.title2.bold())
            Text("Escribe o pega frases (una por línea). Las 18 las leen, las recuerdan enteras y aprenden cómo se asocian sus palabras.")
                .font(.callout)
                .foregroundColor(.gray)
            TextEditor(text: $texto)
                .frame(minHeight: 200)
                .border(Color.gray.opacity(0.4))
            HStack {
                Button("Enseñar a las 18") {
                    let n = nyx.ensena(texto)
                    nota = "Leyeron \(n) frases."
                    texto = ""
                }
                .buttonStyle(.borderedProminent)
                Button("Guardar ahora") {
                    nyx.guarda()
                    nota = nyx.conSwiftData ? "Guardado en SwiftData." : "Guardado."
                }
                .buttonStyle(.bordered)
                Spacer()
                Button("Olvidar todo") { confirmar = true }
                    .foregroundColor(.red)
            }
            Text(nota).font(.caption).foregroundColor(.gray)
            Spacer()
        }
        .padding()
        .alert("¿Empezar de cero?", isPresented: $confirmar) {
            Button("Olvidar todo", role: .destructive) {
                nyx.olvidaTodo()
                nota = "Empezaron de cero."
            }
            Button("Cancelar", role: .cancel) {}
        } message: {
            Text("Las 18 olvidarán todo lo aprendido.")
        }
    }
}
#endif
