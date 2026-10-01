import SwiftUI
import UIKit

// ============================================================
// MARK: - Cámara y dibujo
// ============================================================
// camara            abre la cámara y guarda la foto en /fotos
// draw archivo.draw pinta lo que diga el archivo en un lienzo
//
// Un .draw lleva una orden por línea:
//   fondo negro
//   circulo 150 200 60 rojo
//   rect 40 40 120 80 azul
//   linea 0 0 300 400 verde
//   texto 30 350 hola blanco

extension Shell {

    static let plantillaDraw = """
    # lienzo: 300 de ancho por 400 de alto
    fondo negro
    circulo 150 120 60 amarillo
    rect 60 220 180 60 azul
    linea 20 320 280 320 verde
    texto 90 360 hola blanco
    """

    static func media() -> [String: Spec] {
        var c: [String: Spec] = [:]
        let fm = FileManager.default

        c["camara"] = Spec(help: "camara — abre la cámara y guarda la foto en /fotos") { ctx in
            if let u = try? ctx.env.resolve("/fotos") {
                try? fm.createDirectory(at: u, withIntermediateDirectories: true)
            }
            guard await MainActor.run(body: { UIImagePickerController.isSourceTypeAvailable(.camera) }) else {
                throw ShErr("camara: no hay cámara disponible.\n" +
                            "Si es la primera vez, activa el permiso de cámara en\n" +
                            "Swift Playgrounds → Configuración de la app → Capacidades.")
            }
            ctx.sh.uiCamera?()
            return "abriendo la cámara…\n"
        }

        c["fotos"] = Spec(help: "fotos — lista las fotos tomadas") { ctx in
            guard let u = try? ctx.env.resolve("/fotos"),
                  let items = try? fm.contentsOfDirectory(atPath: u.path), !items.isEmpty else {
                return "no hay fotos todavía\n"
            }
            return items.sorted().joined(separator: "\n") + "\n"
        }

        c["draw"] = Spec(help: "draw <archivo.draw> — dibuja en un lienzo lo que diga el archivo") { ctx in
            guard let p = ctx.args.first else {
                throw ShErr("draw: falta el archivo. Crea uno con: draw new mi.draw")
            }
            if p == "new" {
                let n = ctx.args.dropFirst().first ?? "mi.draw"
                let nombre = n.hasSuffix(".draw") ? n : n + ".draw"
                let u = try ctx.env.resolve(nombre)
                guard !ctx.env.exists(u) else { throw ShErr("draw: \(nombre) ya existe") }
                try Shell.plantillaDraw.write(to: u, atomically: true, encoding: .utf8)
                ctx.sh.uiEdit?(u)
                return "creado \(nombre) — dibújalo con: draw \(nombre)\n"
            }
            let u = try ctx.env.resolve(p)
            guard ctx.env.exists(u) else { throw ShErr("draw: \(p): no existe") }
            let texto = try ctx.input([p])
            ctx.sh.uiDraw?(texto, p)
            return "dibujando \(p)…\n"
        }

        c["drawhelp"] = Spec(help: "drawhelp — órdenes que entiende un archivo .draw") { _ in
            """
            ARCHIVOS .draw — una orden por línea

              fondo COLOR
              circulo X Y RADIO COLOR
              rect X Y ANCHO ALTO COLOR
              linea X1 Y1 X2 Y2 COLOR
              texto X Y PALABRA COLOR
              punto X Y COLOR

            Colores: negro blanco rojo verde azul amarillo naranja
                     morado rosa gris cian

            El lienzo va de 0 a 300 de ancho y de 0 a 400 de alto.
            Crea uno con 'draw new mi.draw' y píntalo con 'draw mi.draw'.

            """
        }

        return c
    }
}

// ============================================================
// MARK: - el lienzo
// ============================================================

struct OrdenDibujo {
    let tipo: String
    let numeros: [Double]
    let color: Color
    let texto: String
}

enum Lienzo {
    static func color(_ n: String) -> Color {
        switch n.lowercased() {
        case "negro": return .black
        case "blanco": return .white
        case "rojo": return .red
        case "verde": return .green
        case "azul": return .blue
        case "amarillo": return .yellow
        case "naranja": return .orange
        case "morado", "violeta": return .purple
        case "rosa": return .pink
        case "gris": return .gray
        case "cian": return .cyan
        default: return .white
        }
    }

    static func leer(_ texto: String) -> (fondo: Color, ordenes: [OrdenDibujo]) {
        var fondo = Color.black
        var ordenes: [OrdenDibujo] = []
        for linea in texto.components(separatedBy: "\n") {
            let t = linea.trimmingCharacters(in: .whitespaces)
            if t.isEmpty || t.hasPrefix("#") { continue }
            let p = t.split(separator: " ").map(String.init)
            guard let tipo = p.first?.lowercased() else { continue }
            if tipo == "fondo" {
                fondo = color(p.count > 1 ? p[1] : "negro")
                continue
            }
            let nums = p.dropFirst().compactMap { Double($0) }
            let ultimo = p.last ?? "blanco"
            var palabra = ""
            if tipo == "texto", p.count >= 4 { palabra = p[3] }
            ordenes.append(OrdenDibujo(tipo: tipo, numeros: nums, color: color(ultimo), texto: palabra))
        }
        return (fondo, ordenes)
    }
}

struct DrawView: View {
    let texto: String
    let nombre: String
    var cerrar: () -> Void

    var body: some View {
        let plano = Lienzo.leer(texto)
        return NavigationStack {
            ZStack {
                plano.fondo
                Canvas { ctx, _ in
                    for o in plano.ordenes {
                        let n = o.numeros
                        switch o.tipo {
                        case "circulo":
                            guard n.count >= 3 else { continue }
                            let r = CGRect(x: n[0] - n[2], y: n[1] - n[2], width: n[2] * 2, height: n[2] * 2)
                            ctx.fill(Path(ellipseIn: r), with: .color(o.color))
                        case "rect":
                            guard n.count >= 4 else { continue }
                            let r = CGRect(x: n[0], y: n[1], width: n[2], height: n[3])
                            ctx.fill(Path(r), with: .color(o.color))
                        case "linea":
                            guard n.count >= 4 else { continue }
                            var p = Path()
                            p.move(to: CGPoint(x: n[0], y: n[1]))
                            p.addLine(to: CGPoint(x: n[2], y: n[3]))
                            ctx.stroke(p, with: .color(o.color), lineWidth: 2)
                        case "punto":
                            guard n.count >= 2 else { continue }
                            let r = CGRect(x: n[0] - 2, y: n[1] - 2, width: 4, height: 4)
                            ctx.fill(Path(ellipseIn: r), with: .color(o.color))
                        case "texto":
                            guard n.count >= 2, !o.texto.isEmpty else { continue }
                            ctx.draw(Text(o.texto).foregroundColor(o.color),
                                     at: CGPoint(x: n[0], y: n[1]))
                        default:
                            continue
                        }
                    }
                }
                .frame(width: 300, height: 400)
            }
            .navigationTitle(nombre)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cerrar") { cerrar() }
                }
            }
        }
    }
}

// ============================================================
// MARK: - la cámara
// ============================================================

struct CamaraView: UIViewControllerRepresentable {
    var alTomar: (UIImage?) -> Void

    func makeCoordinator() -> Coordinador { Coordinador(alTomar: alTomar) }

    func makeUIViewController(context: Context) -> UIImagePickerController {
        let c = UIImagePickerController()
        c.sourceType = UIImagePickerController.isSourceTypeAvailable(.camera) ? .camera : .photoLibrary
        c.delegate = context.coordinator
        return c
    }

    func updateUIViewController(_ uiViewController: UIImagePickerController, context: Context) {}

    final class Coordinador: NSObject, UIImagePickerControllerDelegate, UINavigationControllerDelegate {
        let alTomar: (UIImage?) -> Void
        init(alTomar: @escaping (UIImage?) -> Void) { self.alTomar = alTomar }

        func imagePickerController(_ picker: UIImagePickerController,
                                   didFinishPickingMediaWithInfo info: [UIImagePickerController.InfoKey: Any]) {
            alTomar(info[.originalImage] as? UIImage)
        }

        func imagePickerControllerDidCancel(_ picker: UIImagePickerController) {
            alTomar(nil)
        }
    }
}
