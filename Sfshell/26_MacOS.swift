import Foundation
import UIKit
import AVFoundation

// ============================================================
// MARK: - Sabor macOS (y un par de herramientas Linux más)
// ============================================================
// Los que dependen de AppKit, del Finder o de privilegios que no
// existen en el sandbox de una app (screencapture real, osascript,
// launchctl, proot) dicen claramente por qué no funcionan, en vez
// de fingir que sí. Los demás hacen lo mismo que en una Mac de verdad.

extension Shell {

    static func macos() -> [String: Spec] {
        var c: [String: Spec] = [:]

        c["pbcopy"] = Spec(help: "pbcopy — copia la entrada al portapapeles (como en macOS)") { ctx in
            await ctx.sh.execute("clip " + ctx.stdin)
        }
        c["pbpaste"] = Spec(help: "pbpaste — pega el portapapeles (como en macOS)") { ctx in
            await ctx.sh.execute("clipget")
        }

        c["say"] = Spec(help: "say <texto> — lo lee en voz alta con el sintetizador de voz del sistema") { ctx in
            let texto = ctx.args.isEmpty ? ctx.stdin : ctx.args.joined(separator: " ")
            guard !texto.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
                throw ShErr("say: falta el texto")
            }
            await MainActor.run {
                let u = AVSpeechUtterance(string: texto)
                u.voice = AVSpeechSynthesisVoice(language: Locale.current.identifier) ?? AVSpeechSynthesisVoice(language: "es-ES")
                ctx.sh.speech.speak(u)
            }
            return "🔊 \(texto)\n"
        }

        c["caffeinate"] = Spec(help: "caffeinate [on|off] — evita que la pantalla se apague, como en macOS") { ctx in
            let on = ctx.args.first?.lowercased() != "off"
            await MainActor.run { UIApplication.shared.isIdleTimerDisabled = on }
            return on ? "pantalla despierta mientras dure la sesión ('caffeinate off' para soltarla)\n"
                      : "ya puede apagarse la pantalla otra vez\n"
        }

        c["sw_vers"] = Spec(help: "sw_vers — versión del sistema, al estilo macOS") { _ in
            await MainActor.run {
                let d = UIDevice.current
                return "ProductName:\t\(d.systemName)\nProductVersion:\t\(d.systemVersion)\nModelo:\t\t\(d.model)\n"
            }
        }

        c["system_profiler"] = Spec(help: "system_profiler — lo mismo que 'device', con el nombre de macOS") { ctx in
            await ctx.sh.execute("device")
        }

        c["osascript"] = Spec(help: "osascript — no disponible") { _ in
            throw ShErr("osascript: AppleScript solo existe en macOS. No hay equivalente en iPadOS/iOS.")
        }

        c["launchctl"] = Spec(help: "launchctl — no disponible") { _ in
            throw ShErr("launchctl: administra procesos del sistema en macOS. Una app en su sandbox no\n" +
                        "puede administrar ni lanzar procesos ajenos en iOS.")
        }

        c["defaults"] = Spec(help: "defaults <read|write|delete> <clave> [valor] — preferencias, al estilo macOS") { ctx in
            let sub = ctx.args.first ?? "read"
            let rest = Array(ctx.args.dropFirst())
            let prefijo = "DEFAULT_"
            switch sub {
            case "write":
                guard rest.count >= 2 else { throw ShErr("defaults write: usa defaults write clave valor") }
                ctx.env.vars[prefijo + rest[0]] = rest.dropFirst().joined(separator: " ")
                return ""
            case "read":
                guard let k = rest.first else {
                    let todas = ctx.env.vars.filter { $0.key.hasPrefix(prefijo) }
                        .map { "\($0.key.dropFirst(prefijo.count)) = \($0.value)" }.sorted()
                    return todas.isEmpty ? "no hay preferencias guardadas\n" : todas.joined(separator: "\n") + "\n"
                }
                guard let v = ctx.env.vars[prefijo + k] else { throw ShErr("defaults: no existe '\(k)'") }
                return v + "\n"
            case "delete":
                guard let k = rest.first else { throw ShErr("defaults delete: falta la clave") }
                ctx.env.vars.removeValue(forKey: prefijo + k)
                return ""
            default:
                throw ShErr("defaults: usa read, write o delete")
            }
        }

        c["top"] = Spec(help: "top — procesos y memoria, de un vistazo") { ctx in
            await ctx.sh.execute("ps") + "\n" + (await ctx.sh.execute("free"))
        }

        c["gpu"] = Spec(help: "gpu — qué hay de verdad para gráficos en este sandbox") { _ in
            """
            No hay OpenGL ni Vulkan en iOS/iPadOS: Apple los retiró a favor de Metal, y Metal
            solo se puede usar compilando código nativo dentro de la app (MTLDevice, shaders
            .metal) — un intérprete que corre dentro de una app ya compilada no puede abrir
            su propio contexto de GPU independiente ni cargar shaders nuevos sobre la marcha.

            Lo que sí es real aquí: 'draw' (mira 'drawhelp') pinta de verdad con SwiftUI
            Canvas, que por debajo usa Core Graphics/Core Animation — no es OpenGL ni Vulkan,
            pero es una superficie de dibujo real del sistema, no una simulación de texto.

            """
        }
        c["opengl"] = c["gpu"]
        c["vulkan"] = c["gpu"]

        c["proot"] = Spec(help: "proot — no disponible") { _ in
            throw ShErr("proot: simula chroot/mount interceptando llamadas al sistema con ptrace.\n" +
                        "iOS bloquea ptrace y los espacios de nombres de usuario incluso dentro del\n" +
                        "sandbox de una app normal — es la misma razón por la que no hay box64 ni QEMU.\n" +
                        "Lo más parecido que sí tienes: el sistema de archivos virtual de esta terminal\n" +
                        "(mira 'mount') y 'container' dentro de Simulacro, que aísla una carpeta, no procesos.")
        }

        return c
    }
}
