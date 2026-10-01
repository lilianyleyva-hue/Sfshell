import SwiftUI
import WebKit

// ============================================================
// MARK: - Navegador: WebKit real, con pestañas, historial y marcadores
// ============================================================
// No es un lector de HTML hecho a mano — es WKWebView, el motor
// WebKit de verdad que trae iOS (el mismo que usa Safari), a través
// de la API pública de Apple (UIViewRepresentable). JavaScript, CSS,
// cookies: todo real. Encima se le añaden pestañas, una barra de
// progreso de verdad (KVO sobre estimatedProgress), una página de
// error cuando falla la carga, e historial/marcadores que persisten
// de verdad en UserDefaults — nada de esto se pierde al cerrar la app.

enum WebAction { case atras, adelante, recargar }

/// El puente SwiftUI ↔ WKWebView.
struct WebViewRepresentable: UIViewRepresentable {
    let url: URL
    @Binding var accion: WebAction?
    @Binding var cargando: Bool
    @Binding var progreso: Double
    @Binding var puedeAtras: Bool
    @Binding var puedeAdelante: Bool
    @Binding var titulo: String
    @Binding var error: String?
    var onCargado: ((URL, String) -> Void)? = nil

    func makeCoordinator() -> Coordinador { Coordinador(self) }

    func makeUIView(context: Context) -> WKWebView {
        let web = WKWebView()
        web.navigationDelegate = context.coordinator
        web.allowsBackForwardNavigationGestures = true
        context.coordinator.observador = web.observe(\.estimatedProgress, options: [.new]) { webView, _ in
            DispatchQueue.main.async { self.progreso = webView.estimatedProgress }
        }
        context.coordinator.urlPedida = url
        web.load(URLRequest(url: url))
        return web
    }

    func updateUIView(_ web: WKWebView, context: Context) {
        // OJO: esto se llama en cada refresco de SwiftUI, y mientras carga
        // una página el progreso cambia varias veces por segundo — así que
        // NUNCA se compara contra web.url (que cambia solo con la redirección
        // real, ej. http→https o "sin www"→"con www") porque eso volvía a
        // pedir load() en cada refresco y la página no paraba de recargarse.
        // Se compara contra la última URL que NOSOTROS pedimos cargar.
        if context.coordinator.urlPedida != url {
            context.coordinator.urlPedida = url
            web.load(URLRequest(url: url))
        }
        switch accion {
        case .atras: web.goBack()
        case .adelante: web.goForward()
        case .recargar: web.reload()
        case nil: break
        }
        if accion != nil {
            DispatchQueue.main.async { accion = nil }
        }
    }

    final class Coordinador: NSObject, WKNavigationDelegate {
        var padre: WebViewRepresentable
        var urlPedida: URL?
        var observador: NSKeyValueObservation?
        init(_ padre: WebViewRepresentable) { self.padre = padre }

        func webView(_ webView: WKWebView, didStartProvisionalNavigation nav: WKNavigation!) {
            padre.cargando = true
            padre.error = nil
        }
        func webView(_ webView: WKWebView, didFinish nav: WKNavigation!) {
            padre.cargando = false
            padre.progreso = 1
            padre.puedeAtras = webView.canGoBack
            padre.puedeAdelante = webView.canGoForward
            let t = webView.title ?? ""
            padre.titulo = t
            if let u = webView.url { padre.onCargado?(u, t) }
        }
        func webView(_ webView: WKWebView, didFail nav: WKNavigation!, withError error: Error) {
            padre.cargando = false
            if (error as NSError).code != NSURLErrorCancelled {
                padre.error = error.localizedDescription
            }
        }
        func webView(_ webView: WKWebView, didFailProvisionalNavigation nav: WKNavigation!, withError error: Error) {
            padre.cargando = false
            if (error as NSError).code != NSURLErrorCancelled {
                padre.error = error.localizedDescription
            }
        }
    }
}

// ============================================================
// MARK: - Historial y marcadores (persisten de verdad, en UserDefaults)
// ============================================================

private struct EntradaHistorial: Codable, Identifiable {
    let id: UUID
    let titulo: String
    let url: String
    let fecha: Date
}

private struct Marcador: Codable, Identifiable {
    let id: UUID
    let titulo: String
    let url: String
}

private enum Persistencia {
    static func cargarHistorial() -> [EntradaHistorial] {
        guard let d = UserDefaults.standard.data(forKey: "browser.historial"),
              let arr = try? JSONDecoder().decode([EntradaHistorial].self, from: d) else { return [] }
        return arr
    }
    static func agregarHistorial(_ titulo: String, _ url: String) {
        var arr = cargarHistorial()
        arr.insert(EntradaHistorial(id: UUID(), titulo: titulo.isEmpty ? url : titulo, url: url, fecha: Date()), at: 0)
        if arr.count > 200 { arr.removeLast(arr.count - 200) }
        if let d = try? JSONEncoder().encode(arr) { UserDefaults.standard.set(d, forKey: "browser.historial") }
    }
    static func vaciarHistorial() {
        UserDefaults.standard.removeObject(forKey: "browser.historial")
    }

    static func cargarMarcadores() -> [Marcador] {
        guard let d = UserDefaults.standard.data(forKey: "browser.marcadores"),
              let arr = try? JSONDecoder().decode([Marcador].self, from: d) else { return [] }
        return arr
    }
    static func guardarMarcadores(_ m: [Marcador]) {
        if let d = try? JSONEncoder().encode(m) { UserDefaults.standard.set(d, forKey: "browser.marcadores") }
    }
}

// ============================================================
// MARK: - Barra de herramientas
// ============================================================

private struct BrowserToolbar: View {
    @Binding var urlTexto: String
    let cargando: Bool
    let progreso: Double
    let puedeAtras: Bool
    let puedeAdelante: Bool
    let esMarcador: Bool
    let onAtras: () -> Void
    let onAdelante: () -> Void
    let onRecargar: () -> Void
    let onIr: () -> Void
    let onMarcador: () -> Void

    var body: some View {
        VStack(spacing: 0) {
            VStack(spacing: 6) {
                HStack(spacing: 18) {
                    Button(action: onAtras) { Image(systemName: "chevron.left") }
                        .disabled(!puedeAtras)
                    Button(action: onAdelante) { Image(systemName: "chevron.right") }
                        .disabled(!puedeAdelante)
                    Button(action: onRecargar) { Image(systemName: "arrow.clockwise") }
                    Spacer()
                }
                HStack {
                    Image(systemName: "lock.fill").font(.caption2).foregroundStyle(.secondary)
                    TextField("buscar o escribir una URL", text: $urlTexto)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .font(.system(size: 13, design: .monospaced))
                        .onSubmit(onIr)
                    Button(action: onMarcador) {
                        Image(systemName: esMarcador ? "star.fill" : "star")
                            .foregroundStyle(esMarcador ? .yellow : .secondary)
                    }
                    Button("Ir", action: onIr)
                }
            }
            .padding(10)
            .background(Color.gray.opacity(0.15))
            if cargando {
                ProgressView(value: progreso).progressViewStyle(.linear).tint(.blue)
            }
        }
    }
}

// ============================================================
// MARK: - Tira de pestañas
// ============================================================

private struct TabMeta: Identifiable {
    let id = UUID()
    var urlTexto: String
    var titulo: String = ""
}

private struct TabStrip: View {
    let pestañas: [TabMeta]
    let activa: UUID
    let onSelect: (UUID) -> Void
    let onClose: (UUID) -> Void
    let onNew: () -> Void

    var body: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 6) {
                ForEach(pestañas) { t in
                    HStack(spacing: 4) {
                        Text(t.titulo.isEmpty ? t.urlTexto : t.titulo)
                            .font(.caption2)
                            .lineLimit(1)
                            .frame(maxWidth: 110)
                        if pestañas.count > 1 {
                            Button(action: { onClose(t.id) }) {
                                Image(systemName: "xmark").font(.system(size: 9))
                            }
                        }
                    }
                    .padding(.horizontal, 8)
                    .padding(.vertical, 5)
                    .background(t.id == activa ? Color.blue.opacity(0.25) : Color.gray.opacity(0.15))
                    .clipShape(RoundedRectangle(cornerRadius: 7))
                    .onTapGesture { onSelect(t.id) }
                }
                Button(action: onNew) { Image(systemName: "plus") }
                    .padding(.horizontal, 6)
            }
            .padding(.horizontal, 8)
            .padding(.vertical, 6)
        }
        .background(Color.gray.opacity(0.08))
    }
}

// ============================================================
// MARK: - El navegador
// ============================================================

struct BrowserView: View {
    @Environment(\.dismiss) private var dismiss
    var onCerrar: (() -> Void)? = nil

    @State private var pestañas: [TabMeta]
    @State private var pestañaActiva: UUID

    @State private var urlTexto: String
    @State private var urlActual: URL?
    @State private var accion: WebAction?
    @State private var cargando = false
    @State private var progreso: Double = 0
    @State private var puedeAtras = false
    @State private var puedeAdelante = false
    @State private var titulo = ""
    @State private var error: String?

    @State private var mostrarHistorial = false
    @State private var mostrarMarcadores = false
    @State private var marcadores: [Marcador] = Persistencia.cargarMarcadores()
    @State private var historialCache: [EntradaHistorial] = []

    init(urlInicial: String? = nil, onCerrar: (() -> Void)? = nil) {
        self.onCerrar = onCerrar
        let arranque = urlInicial ?? "https://www.wikipedia.org"
        let t = TabMeta(urlTexto: arranque)
        _pestañas = State(initialValue: [t])
        _pestañaActiva = State(initialValue: t.id)
        _urlTexto = State(initialValue: arranque)
        _urlActual = State(initialValue: BrowserView.normaliza(arranque))
    }

    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                TabStrip(pestañas: pestañas, activa: pestañaActiva, onSelect: cambiaPestaña, onClose: cierraPestaña, onNew: nuevaPestaña)
                toolbar
                pagina
            }
            .navigationTitle(titulo.isEmpty ? "Navegador" : titulo)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cerrar") { onCerrar?() ?? dismiss() }
                }
                ToolbarItem(placement: .primaryAction) {
                    Menu {
                        Button("Historial") { mostrarHistorial = true }
                        Button("Marcadores") { mostrarMarcadores = true }
                        Button("Nueva pestaña") { nuevaPestaña() }
                    } label: {
                        Image(systemName: "ellipsis.circle")
                    }
                }
            }
            .sheet(isPresented: $mostrarHistorial) { historialSheet }
            .sheet(isPresented: $mostrarMarcadores) { marcadoresSheet }
        }
    }

    @ViewBuilder private var toolbar: some View {
        BrowserToolbar(urlTexto: $urlTexto, cargando: cargando, progreso: progreso,
                       puedeAtras: puedeAtras, puedeAdelante: puedeAdelante, esMarcador: esMarcadorActual,
                       onAtras: { accion = .atras }, onAdelante: { accion = .adelante },
                       onRecargar: { accion = .recargar }, onIr: navega, onMarcador: alternaMarcador)
    }

    @ViewBuilder private var pagina: some View {
        ZStack {
            if let urlActual {
                WebViewRepresentable(url: urlActual, accion: $accion, cargando: $cargando, progreso: $progreso,
                                     puedeAtras: $puedeAtras, puedeAdelante: $puedeAdelante, titulo: $titulo,
                                     error: $error) { u, t in
                    Persistencia.agregarHistorial(t, u.absoluteString)
                    if let i = pestañas.firstIndex(where: { $0.id == pestañaActiva }) {
                        pestañas[i].titulo = t
                    }
                }
            } else {
                Spacer()
                Text("escribe una dirección arriba").foregroundStyle(.secondary)
                Spacer()
            }
            if let error {
                errorView(error)
            }
        }
    }

    @ViewBuilder private func errorView(_ mensaje: String) -> some View {
        VStack(spacing: 12) {
            Image(systemName: "wifi.exclamationmark").font(.largeTitle).foregroundStyle(.secondary)
            Text("no se pudo cargar la página").font(.headline)
            Text(mensaje).font(.caption).foregroundStyle(.secondary).multilineTextAlignment(.center).padding(.horizontal, 30)
            Button("Reintentar") { accion = .recargar }
        }
        .padding(24)
        .background(.regularMaterial)
        .clipShape(RoundedRectangle(cornerRadius: 12))
        .padding(30)
    }

    private var esMarcadorActual: Bool {
        guard let u = urlActual?.absoluteString else { return false }
        return marcadores.contains { $0.url == u }
    }

    // --- historial ---

    @ViewBuilder private var historialSheet: some View {
        NavigationStack {
            List {
                ForEach(historialCache) { e in
                    Button {
                        urlTexto = e.url
                        navega()
                        mostrarHistorial = false
                    } label: {
                        VStack(alignment: .leading) {
                            Text(e.titulo).lineLimit(1)
                            Text(e.url).font(.caption2).foregroundStyle(.secondary).lineLimit(1)
                        }
                    }
                }
            }
            .navigationTitle("Historial")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("Cerrar") { mostrarHistorial = false } }
                ToolbarItem(placement: .primaryAction) {
                    Button("Vaciar", role: .destructive) {
                        Persistencia.vaciarHistorial()
                        historialCache = []
                    }
                }
            }
            .onAppear { historialCache = Persistencia.cargarHistorial() }
        }
    }

    // --- marcadores ---

    @ViewBuilder private var marcadoresSheet: some View {
        NavigationStack {
            List {
                ForEach(marcadores) { m in
                    Button {
                        urlTexto = m.url
                        navega()
                        mostrarMarcadores = false
                    } label: {
                        VStack(alignment: .leading) {
                            Text(m.titulo).lineLimit(1)
                            Text(m.url).font(.caption2).foregroundStyle(.secondary).lineLimit(1)
                        }
                    }
                }
                .onDelete { idx in
                    marcadores.remove(atOffsets: idx)
                    Persistencia.guardarMarcadores(marcadores)
                }
            }
            .navigationTitle("Marcadores")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("Cerrar") { mostrarMarcadores = false } }
            }
        }
    }

    private func alternaMarcador() {
        guard let u = urlActual?.absoluteString else { return }
        if let i = marcadores.firstIndex(where: { $0.url == u }) {
            marcadores.remove(at: i)
        } else {
            marcadores.append(Marcador(id: UUID(), titulo: titulo.isEmpty ? u : titulo, url: u))
        }
        Persistencia.guardarMarcadores(marcadores)
    }

    // --- pestañas ---

    private func nuevaPestaña() {
        guardaPestañaActual()
        let t = TabMeta(urlTexto: "https://www.wikipedia.org")
        pestañas.append(t)
        pestañaActiva = t.id
        cargaPestaña(t)
    }

    private func cambiaPestaña(_ id: UUID) {
        guard id != pestañaActiva else { return }
        guardaPestañaActual()
        pestañaActiva = id
        if let t = pestañas.first(where: { $0.id == id }) { cargaPestaña(t) }
    }

    private func cierraPestaña(_ id: UUID) {
        guard pestañas.count > 1 else { return }
        let eraActiva = id == pestañaActiva
        pestañas.removeAll { $0.id == id }
        if eraActiva, let primera = pestañas.first {
            pestañaActiva = primera.id
            cargaPestaña(primera)
        }
    }

    private func guardaPestañaActual() {
        guard let i = pestañas.firstIndex(where: { $0.id == pestañaActiva }) else { return }
        pestañas[i].urlTexto = urlTexto
        pestañas[i].titulo = titulo
    }

    private func cargaPestaña(_ t: TabMeta) {
        urlTexto = t.urlTexto
        titulo = t.titulo
        error = nil
        cargando = false
        progreso = 0
        puedeAtras = false
        puedeAdelante = false
        urlActual = BrowserView.normaliza(t.urlTexto)
    }

    private func navega() {
        urlActual = BrowserView.normaliza(urlTexto)
    }

    /// Si parece una dirección, la completa con https://. Si no, busca en DuckDuckGo.
    static func normaliza(_ texto: String) -> URL? {
        var s = texto.trimmingCharacters(in: .whitespaces)
        guard !s.isEmpty else { return nil }
        if s.contains("://") { return URL(string: s) }
        if s.contains("."), !s.contains(" ") {
            s = "https://" + s
            return URL(string: s)
        }
        let q = s.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? s
        return URL(string: "https://duckduckgo.com/?q=\(q)")
    }
}

extension Shell {
    static func browserCmd() -> [String: Spec] {
        var c: [String: Spec] = [:]
        c["browser"] = Spec(help: "browser [url] — abre el navegador de verdad (WebKit, el motor de Safari)") { ctx in
            ctx.sh.uiBrowser?(ctx.args.first)
            return ""
        }
        c["web"] = c["browser"]
        return c
    }
}
