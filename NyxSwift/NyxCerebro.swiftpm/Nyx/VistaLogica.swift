#if canImport(SwiftUI)
import SwiftUI

// VistaLogica.swift — la pestaña del consejo lógico (18 agentes que razonan).

struct PantallaLogica: View {
    @ObservedObject var nyx: NyxModelo
    @State private var texto = ""

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 12) {
                Text("🧮 Consejo lógico").font(.title2.bold())
                Text("18 agentes que razonan con hechos: deducen, comprueban y te explican por qué. Contestan sí, no o no sé (no inventan).")
                    .font(.caption)
                    .foregroundColor(.gray)
                HStack {
                    TextField("Pregunta o enséñale: «el delfín es un mamífero», «¿tiene pelo el delfín?»", text: $texto)
                        .textFieldStyle(.roundedBorder)
                        .autocorrectionDisabled()
                        .onSubmit { envia() }
                    Button("Enviar") { envia() }
                        .buttonStyle(.borderedProminent)
                }
                RespuestaLogicaVista(nyx: nyx)
                HStack {
                    Button("Que razonen 20 pasos") { nyx.razonaLogica(20) }
                        .buttonStyle(.bordered)
                    Button("Leer lo que sabe el consejo nuevo") { nyx.logicaLeeConsejoNuevo() }
                        .buttonStyle(.bordered)
                }
                AgentesLogicos(nyx: nyx)
                Text("Lo que van haciendo").font(.headline)
                ForEach(nyx.lineasLogica.reversed().prefix(80)) { (l: LineaLogica) in
                    FilaLogica(l: l)
                }
            }
            .padding()
        }
    }

    func envia() {
        nyx.preguntaLogica(texto)
        texto = ""
    }
}

struct RespuestaLogicaVista: View {
    @ObservedObject var nyx: NyxModelo

    var resh: String { Resh.traduceTexto(nyx.veredictoLogico) }

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(nyx.veredictoLogico.uppercased()).font(.title3.bold())
            Text(nyx.traducir ? "resh: " + resh : "").font(.caption).foregroundColor(.gray)
            ForEach(nyx.explicacionLogica, id: \.self) { (p: String) in
                Text("· " + p).font(.callout)
            }
            ForEach(nyx.aportesLogicos) { (a: Aporte) in
                Text(a.agente + ": " + a.texto).font(.caption).foregroundColor(.gray)
            }
        }
    }
}

struct AgentesLogicos: View {
    @ObservedObject var nyx: NyxModelo

    var resumen: String {
        var partes: [String] = []
        for (i, n) in agentesLogicos.enumerated() {
            partes.append("\(n) \(nyx.logico.contadores[i])")
        }
        return partes.joined(separator: " · ")
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Los 18 agentes (y cuánto han trabajado)").font(.headline)
            Text(resumen).font(.caption)
            Text("\(nyx.logico.hechos.count) hechos · \(nyx.logico.contradicciones.count) contradicciones")
                .font(.caption)
                .foregroundColor(.gray)
        }
    }
}

struct FilaLogica: View {
    let l: LineaLogica

    var body: some View {
        HStack(alignment: .top) {
            Text(agentesLogicos[l.agente])
                .font(.caption.bold())
                .foregroundColor(colorDe(l.agente))
                .frame(width: 110, alignment: .leading)
            Text(l.texto).font(.callout)
        }
    }
}
#endif
