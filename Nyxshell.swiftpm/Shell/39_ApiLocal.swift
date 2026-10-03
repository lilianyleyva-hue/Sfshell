import Foundation
#if os(Linux) && canImport(FoundationNetworking)
import FoundationNetworking
#endif

// ============================================================
// MARK: - Emulador de API local (como json-server, sin internet)
// ============================================================
// Una API REST de mentira que responde DENTRO de la app. La usan tú
// y las 18 IAs (todas ven la misma). Cualquier petición a
//     http://localhost/…   http://127.0.0.1/…   http://api.local/…
// (con o sin puerto: localhost:3000 sirve igual) la contesta el
// emulador en vez de la red. Funciona con curl, wget, api y http.
//
// Datos: /srv/api/db.json — cada clave es una colección (lista de
// objetos con "id") o un objeto suelto:
//   GET    /usuarios                 lista (filtros: ?nombre=Ana  ?q=texto
//                                     ?_sort=edad&_order=desc  ?_limit=5&_page=2)
//   GET    /usuarios/1               uno
//   POST   /usuarios                 crea (id automático; la colección se crea sola)
//   PUT    /usuarios/1               reemplaza      PATCH  /usuarios/1  modifica
//   DELETE /usuarios/1               borra
//   GET    /db                       todo           GET /  índice
//
// Rutas propias: /srv/api/rutas.json
//   api-local ruta GET /hola/:nombre '{"saludo":"hola :nombre"}'
//   api-local ruta GET /archivos --ejecuta ls /Escritorio
// Una ruta con --ejecuta corre el comando en la shell de quien la
// creó (tu shell, o la de la IA), con $NOMBRE, $BODY y $QUERY_x.
//
// Simulación: api-local retraso 300 · api-local fallo 10%

final class APILocal: @unchecked Sendable {
    static let uno = APILocal()

    struct Ruta: Codable {
        var metodo: String          // GET, POST… o * para cualquiera
        var ruta: String            // /hola/:nombre
        var responde: String?       // texto o JSON, con :param y {{body}}
        var ejecuta: String?        // comando de la shell
        var estado: Int
        var dueño: String           // "humano" o el rol de la IA
    }

    struct Peticion {
        let hora: String
        let quien: String
        let metodo: String
        let ruta: String
        let estado: Int
        let ms: Int
    }

    private let l = NSLock()
    private var registro: [Peticion] = []
    private var anidadas = 0
    private(set) var retrasoMs = 0
    private(set) var probFallo = 0.0
    private(set) var encendido = true

    static let hosts: Set<String> = ["localhost", "127.0.0.1", "0.0.0.0", "api.local", "local"]

    var dir: URL { ShellEnv.raizHumano.appendingPathComponent("srv/api", isDirectory: true) }
    var archivoDB: URL { dir.appendingPathComponent("db.json") }
    var archivoRutas: URL { dir.appendingPathComponent("rutas.json") }

    // ---------- direcciones ----------

    /// ¿Esta URL es para el emulador? Devuelve la ruta y la consulta.
    static func local(_ texto: String) -> (ruta: String, consulta: [(String, String)])? {
        var s = texto.trimmingCharacters(in: .whitespaces)
        for esquema in ["http://", "https://", "local://"] where s.lowercased().hasPrefix(esquema) {
            s = String(s.dropFirst(esquema.count))
        }
        if s.hasPrefix(":") { s = "localhost" + s }            // :3000/usuarios (como httpie)
        if s.hasPrefix("/") { s = "localhost" + s }            // /usuarios
        let finHost = s.firstIndex(where: { $0 == "/" || $0 == "?" }) ?? s.endIndex
        let hostPuerto = String(s[s.startIndex..<finHost])
        let host = hostPuerto.split(separator: ":").first.map { String($0).lowercased() } ?? ""
        guard hosts.contains(host) else { return nil }
        var resto = String(s[finHost...])
        var consulta: [(String, String)] = []
        if let q = resto.firstIndex(of: "?") {
            let qs = resto[resto.index(after: q)...]
            resto = String(resto[resto.startIndex..<q])
            for par in qs.split(separator: "&") {
                let kv = par.split(separator: "=", maxSplits: 1).map {
                    String($0).removingPercentEncoding ?? String($0)
                }
                if let k = kv.first, !k.isEmpty { consulta.append((k, kv.count > 1 ? kv[1] : "")) }
            }
        }
        if resto.isEmpty { resto = "/" }
        return (resto.removingPercentEncoding ?? resto, consulta)
    }

    // ---------- datos ----------

    func leerDB() -> [String: Any] {
        guard let d = try? Data(contentsOf: archivoDB),
              let o = try? JSONSerialization.jsonObject(with: d) as? [String: Any] else { return [:] }
        return o
    }

    func guardarDB(_ db: [String: Any]) {
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        if let d = try? JSONSerialization.data(withJSONObject: db, options: [.prettyPrinted, .sortedKeys]) {
            try? d.write(to: archivoDB)
        }
    }

    func leerRutas() -> [Ruta] {
        guard let d = try? Data(contentsOf: archivoRutas) else { return [] }
        return (try? JSONDecoder().decode([Ruta].self, from: d)) ?? []
    }

    func guardarRutas(_ r: [Ruta]) {
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let enc = JSONEncoder()
        enc.outputFormatting = [.prettyPrinted]
        if let d = try? enc.encode(r) { try? d.write(to: archivoRutas) }
    }

    func configurar(retraso: Int? = nil, fallo: Double? = nil, encendido e: Bool? = nil) {
        l.conCandado {
            if let r = retraso { retrasoMs = max(0, min(30_000, r)) }
            if let f = fallo { probFallo = max(0, min(1, f)) }
            if let e { encendido = e }
        }
    }

    func ultimas(_ n: Int) -> [Peticion] { l.conCandado { Array(registro.suffix(n)) } }
    var totalPeticiones: Int { l.conCandado { registro.count } }

    // ---------- atender ----------

    /// Contesta una petición si es para el emulador; nil si es para la red.
    func atender(url: String, metodo: String, cabeceras: [String], cuerpo: String?, quien: String) async throws -> (Data, HTTPURLResponse)? {
        guard let (ruta, consulta) = APILocal.local(url) else { return nil }
        let (on, retraso, fallo) = l.conCandado { (encendido, retrasoMs, probFallo) }
        guard on else { throw ShErr("conexión rechazada: el emulador está apagado ('api-local enciende')") }
        let inicio = Date()
        let profundidad = l.conCandado { () -> Int in anidadas += 1; return anidadas }
        defer { l.conCandado { anidadas -= 1 } }
        if retraso > 0 { try? await Task.sleep(nanoseconds: UInt64(retraso) * 1_000_000) }

        var estado = 200
        var cuerpoResp: Any = [String: Any]()
        var texto: String? = nil
        let m = metodo.uppercased()
        if profundidad > 6 {
            estado = 508; cuerpoResp = ["error": "demasiadas peticiones anidadas (¿una ruta se llama a sí misma?)"]
        } else if fallo > 0, Double.random(in: 0 ..< 1) < fallo {
            estado = 500; cuerpoResp = ["error": "fallo simulado", "probabilidad": fallo]
        } else if let r = await rutaPropia(m, ruta, consulta, cuerpo) {
            estado = r.estado; texto = r.texto
        } else {
            (estado, cuerpoResp) = rest(m, ruta, consulta, cuerpo)
        }

        let datos: Data
        var tipo = "application/json; charset=utf-8"
        if let t = texto {
            datos = Data(t.utf8)
            let limpio = t.trimmingCharacters(in: .whitespacesAndNewlines)
            if !(limpio.hasPrefix("{") || limpio.hasPrefix("[")) { tipo = "text/plain; charset=utf-8" }
        } else {
            datos = (try? JSONSerialization.data(withJSONObject: cuerpoResp, options: [.prettyPrinted, .sortedKeys, .fragmentsAllowed])) ?? Data("{}".utf8)
        }
        let ms = Int(Date().timeIntervalSince(inicio) * 1000)
        l.conCandado {
            registro.append(Peticion(hora: SistemaIAs.hora(), quien: quien, metodo: m, ruta: ruta, estado: estado, ms: ms))
            if registro.count > 300 { registro.removeFirst(registro.count - 300) }
        }
        let u = URL(string: "http://localhost\(ruta.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? "/")")
            ?? URL(fileURLWithPath: "/")
        let resp = HTTPURLResponse(url: u, statusCode: estado, httpVersion: "HTTP/1.1",
                                   headerFields: ["Content-Type": tipo, "X-Emulador": "SwiftShell api-local",
                                                  "Content-Length": String(datos.count)])!
        return (datos, resp)
    }

    /// Cuerpo de la petición → objeto: JSON, o formulario a=1&b=2.
    static func objeto(_ cuerpo: String?) -> [String: Any]? {
        guard let c = cuerpo?.trimmingCharacters(in: .whitespacesAndNewlines), !c.isEmpty else { return nil }
        if let d = c.data(using: .utf8), let o = try? JSONSerialization.jsonObject(with: d) as? [String: Any] { return o }
        var o: [String: Any] = [:]
        for par in c.split(separator: "&") {
            let kv = par.split(separator: "=", maxSplits: 1).map { String($0).removingPercentEncoding ?? String($0) }
            guard let k = kv.first, !k.isEmpty else { continue }
            let v = kv.count > 1 ? kv[1] : ""
            switch v {
            case "true": o[k] = true
            case "false": o[k] = false
            case "null": o[k] = NSNull()
            default: o[k] = Int(v).map { $0 as Any } ?? Double(v).map { $0 as Any } ?? v
            }
        }
        return o.isEmpty ? nil : o
    }

    static func texto(_ v: Any?) -> String {
        switch v {
        case nil: return ""
        case let s as String: return s
        case let n as NSNumber: return n.stringValue
        default: return "\(v!)"
        }
    }

    // ---------- REST automático ----------

    private func rest(_ m: String, _ ruta: String, _ consulta: [(String, String)], _ cuerpo: String?) -> (Int, Any) {
        let partes = ruta.split(separator: "/").map(String.init)
        return l.conCandado { () -> (Int, Any) in
            var db = leerDB()
            if partes.isEmpty {
                guard m == "GET" else { return (405, ["error": "método no permitido en /"]) }
                var indice: [String: Any] = [:]
                for (k, v) in db { indice["/" + k] = (v as? [Any]).map { "\($0.count) elementos" } ?? "objeto" }
                return (200, ["emulador": "SwiftShell api-local", "rutas": indice,
                              "ayuda": "GET /coleccion · GET /coleccion/id · POST · PUT · PATCH · DELETE · GET /db"])
            }
            if partes == ["db"], m == "GET" { return (200, db) }
            let col = partes[0]
            let id = partes.count > 1 ? partes[1] : nil
            guard partes.count <= 2 else { return (404, ["error": "ruta no encontrada: \(ruta)"]) }

            // objeto suelto (no lista)
            if let obj = db[col] as? [String: Any] {
                guard id == nil else { return (404, ["error": "\(col) es un objeto, no una colección"]) }
                switch m {
                case "GET": return (200, obj)
                case "PUT":
                    guard let nuevo = APILocal.objeto(cuerpo) else { return (400, ["error": "falta el cuerpo JSON"]) }
                    db[col] = nuevo; guardarDB(db); return (200, nuevo)
                case "PATCH":
                    guard let cambios = APILocal.objeto(cuerpo) else { return (400, ["error": "falta el cuerpo JSON"]) }
                    let unido = obj.merging(cambios) { _, b in b }
                    db[col] = unido; guardarDB(db); return (200, unido)
                default: return (405, ["error": "\(m) no se puede usar en un objeto suelto"])
                }
            }

            var lista = db[col] as? [[String: Any]]
            if lista == nil, db[col] != nil { return (500, ["error": "\(col) no es una lista de objetos"]) }
            func indice(_ id: String) -> Int? {
                lista?.firstIndex { APILocal.texto($0["id"]) == id }
            }

            switch (m, id) {
            case ("GET", nil):
                guard var l = lista else { return (404, ["error": "no existe la colección '\(col)'"]) }
                var orden: String? = nil, desc = false, limite: Int? = nil, pagina = 1
                for (k, v) in consulta {
                    switch k {
                    case "_sort": orden = v
                    case "_order": desc = v.lowercased() == "desc"
                    case "_limit": limite = Int(v)
                    case "_page": pagina = max(1, Int(v) ?? 1)
                    case "q":
                        let t = v.lowercased()
                        l = l.filter { $0.values.contains { APILocal.texto($0).lowercased().contains(t) } }
                    default:
                        if k.hasSuffix("_gte"), let n = Double(v) {
                            let c = String(k.dropLast(4)); l = l.filter { (Double(APILocal.texto($0[c])) ?? -.infinity) >= n }
                        } else if k.hasSuffix("_lte"), let n = Double(v) {
                            let c = String(k.dropLast(4)); l = l.filter { (Double(APILocal.texto($0[c])) ?? .infinity) <= n }
                        } else if k.hasSuffix("_like") {
                            let c = String(k.dropLast(5)); l = l.filter { APILocal.texto($0[c]).lowercased().contains(v.lowercased()) }
                        } else if !k.hasPrefix("_") {
                            l = l.filter { APILocal.texto($0[k]) == v }
                        }
                    }
                }
                if let o = orden {
                    l.sort { a, b in
                        let x = APILocal.texto(a[o]), y = APILocal.texto(b[o])
                        let menor: Bool
                        if let nx = Double(x), let ny = Double(y) { menor = nx < ny } else { menor = x < y }
                        return desc ? !menor && x != y : menor
                    }
                }
                if let n = limite, n > 0 {
                    let ini = (pagina - 1) * n
                    l = ini < l.count ? Array(l[ini ..< min(l.count, ini + n)]) : []
                }
                return (200, l)

            case ("GET", let id?):
                guard let i = indice(id), let l = lista else { return (404, ["error": "\(col)/\(id) no existe"]) }
                return (200, l[i])

            case ("POST", nil):
                guard var nuevo = APILocal.objeto(cuerpo) else { return (400, ["error": "falta el cuerpo (JSON o a=1&b=2)"]) }
                var l = lista ?? []
                if nuevo["id"] == nil {
                    let maximo = l.compactMap { Int(APILocal.texto($0["id"])) }.max() ?? 0
                    nuevo["id"] = maximo + 1
                } else if l.contains(where: { APILocal.texto($0["id"]) == APILocal.texto(nuevo["id"]) }) {
                    return (409, ["error": "ya existe \(col)/\(APILocal.texto(nuevo["id"]))"])
                }
                l.append(nuevo); db[col] = l; guardarDB(db)
                return (201, nuevo)

            case ("PUT", let id?), ("PATCH", let id?):
                guard var l = lista, let i = indice(id) else { return (404, ["error": "\(col)/\(id) no existe"]) }
                guard let datos = APILocal.objeto(cuerpo) else { return (400, ["error": "falta el cuerpo JSON"]) }
                var nuevo = m == "PUT" ? datos : l[i].merging(datos) { _, b in b }
                nuevo["id"] = l[i]["id"]
                l[i] = nuevo; db[col] = l; guardarDB(db)
                return (200, nuevo)

            case ("DELETE", let id?):
                guard var l = lista, let i = indice(id) else { return (404, ["error": "\(col)/\(id) no existe"]) }
                l.remove(at: i); db[col] = l; guardarDB(db)
                return (200, ["borrado": "\(col)/\(id)"])

            default:
                return (405, ["error": "\(m) no se puede usar en \(ruta)"])
            }
        }
    }

    // ---------- rutas propias ----------

    private func rutaPropia(_ m: String, _ ruta: String, _ consulta: [(String, String)], _ cuerpo: String?) async -> (estado: Int, texto: String)? {
        let rutas = l.conCandado { leerRutas() }
        let partes = ruta.split(separator: "/").map(String.init)
        for r in rutas where r.metodo == "*" || r.metodo.uppercased() == m {
            let patron = r.ruta.split(separator: "/").map(String.init)
            guard patron.count == partes.count else { continue }
            var params: [String: String] = [:]
            var coincide = true
            for (p, x) in zip(patron, partes) {
                if p.hasPrefix(":") { params[String(p.dropFirst())] = x } else if p != x { coincide = false; break }
            }
            guard coincide else { continue }
            if let cmd = r.ejecuta {
                let sh = shellDe(r.dueño)
                for (k, v) in params { sh.env.vars[k.uppercased()] = v }
                for (k, v) in consulta { sh.env.vars["QUERY_" + k.uppercased()] = v }
                sh.env.vars["BODY"] = cuerpo ?? ""
                sh.env.vars["METHOD"] = m
                let salida = await sh.execute(cmd)
                let fallo = (sh.env.vars["?"] ?? "0") != "0"
                return (fallo && r.estado == 200 ? 500 : r.estado, salida.replacingOccurrences(of: Shell.errMark, with: ""))
            }
            var t = r.responde ?? ""
            for (k, v) in params { t = t.replacingOccurrences(of: ":" + k, with: v) }
            for (k, v) in consulta { t = t.replacingOccurrences(of: "{{" + k + "}}", with: v) }
            t = t.replacingOccurrences(of: "{{body}}", with: cuerpo ?? "")
            t = t.replacingOccurrences(of: "{{metodo}}", with: m)
            return (r.estado, t)
        }
        return nil
    }

    /// Shell nueva para correr una ruta: la del humano o la de la IA dueña
    /// (misma carpeta, sin tocar su sesión: así no se bloquea si la IA se
    /// llama a sí misma).
    private func shellDe(_ dueño: String) -> Shell {
        if let r = RolMental(rawValue: dueño) {
            let raiz = SistemaIAs.uno.espacio(r).shell.env.root
            return Shell(raiz: raiz, usuario: r.rawValue, rolIA: r)
        }
        return Shell(raiz: nil, usuario: "api")
    }

    static func semilla() -> [String: Any] { [
        "usuarios": [
            ["id": 1, "nombre": "Ana", "edad": 31, "ciudad": "Bogotá"],
            ["id": 2, "nombre": "Luis", "edad": 27, "ciudad": "Madrid"],
            ["id": 3, "nombre": "Sam", "edad": 22, "ciudad": "Lima"],
        ],
        "tareas": [
            ["id": 1, "titulo": "probar el emulador", "hecha": false, "usuarioId": 1],
            ["id": 2, "titulo": "enseñar una palabra a Nyx", "hecha": true, "usuarioId": 2],
        ],
        "memorias": [[String: Any]](),
        "config": ["nombre": "mi api", "version": 1],
    ] }
}

extension Shell {

    /// Quién hace la petición (para el registro del emulador).
    var quienPide: String { rolIA?.rawValue ?? "humano" }

    static func apiLocal() -> [String: Spec] {
        var c: [String: Spec] = [:]

        c["api-local"] = Spec(help: "api-local — emulador de API REST en http://localhost (tú y las IAs) · api-local ayuda") { ctx in
            try await Shell.ordenApiLocal(ctx)
        }
        c["json-server"] = c["api-local"]
        c["mockapi"] = c["api-local"]
        c["http"] = Spec(help: "http [MÉTODO] <url> [campo=texto campo:=json Cabecera:valor] — petición HTTP con JSON bonito") { ctx in
            try await Shell.ordenHttp(ctx)
        }
        c["https"] = c["http"]
        return c
    }

    /// Cuerpo de 'api-local' (función y no closure: compila más rápido).
    static func ordenApiLocal(_ ctx: Ctx) async throws -> String {
            let api = APILocal.uno
            let a = ctx.args
            let sub = a.first?.lowercased() ?? "estado"
            let resto = Array(a.dropFirst())

            func json(_ v: Any) -> String {
                guard let d = try? JSONSerialization.data(withJSONObject: v, options: [.prettyPrinted, .sortedKeys, .fragmentsAllowed]),
                      let s = String(data: d, encoding: .utf8) else { return "\(v)" }
                return s + "\n"
            }

            switch sub {
            case "estado":
                let db = api.leerDB()
                var out = "api-local · \(api.encendido ? "encendido" : "APAGADO") · http://localhost (o localhost:3000, api.local)\n"
                if db.isEmpty {
                    out += "sin datos todavía — 'api-local ejemplo' crea usuarios, tareas y config\n"
                } else {
                    for k in db.keys.sorted() {
                        let v = db[k]
                        out += "  /\(k.padding(toLength: 14, withPad: " ", startingAt: 0)) "
                        out += (v as? [Any]).map { "\($0.count) elementos" } ?? "objeto"
                        out += "\n"
                    }
                }
                let rutas = api.leerRutas()
                out += "rutas propias: \(rutas.count) · peticiones: \(api.totalPeticiones)"
                if api.retrasoMs > 0 { out += " · retraso \(api.retrasoMs) ms" }
                if api.probFallo > 0 { out += " · fallos \(Int(api.probFallo * 100))%" }
                return out + "\nprueba: http localhost/usuarios · api-local ayuda\n"

            case "ayuda", "help", "-h":
                return """
                api-local — una API REST de mentira dentro de la app (sin internet)
                  api-local                     estado: colecciones, rutas, peticiones
                  api-local ejemplo             datos de ejemplo (usuarios, tareas, config)
                  api-local crea <col> [json…]  crea una colección (y le mete objetos)
                  api-local datos [col]         muestra los datos
                  api-local borra <col>         borra una colección
                  api-local ruta GET /hola/:n '{"hola":":n"}' [--estado 201]
                  api-local ruta GET /ls --ejecuta ls /     (corre en TU shell o en la de la IA)
                  api-local rutas · api-local quita-ruta <n>
                  api-local retraso <ms>        simula una red lenta
                  api-local fallo <10%|0.1>     simula errores 500
                  api-local log [n]             últimas peticiones (quién, qué, estado)
                  api-local importa|exporta <archivo.json>
                  api-local reinicia            borra todos los datos
                  api-local apaga|enciende
                usarla:
                  http localhost/usuarios                    lista
                  http localhost/usuarios?ciudad=Lima        filtra (q= _sort= _order= _limit= _page=)
                  http POST localhost/usuarios nombre=Eva edad:=40
                  http PATCH localhost/usuarios/1 edad:=32
                  http DELETE localhost/usuarios/3
                  curl -s localhost:3000/db                  (curl, wget y api también sirven)
                las IAs usan la misma API desde sus shells: ia memoria http localhost/memorias

                """

            case "ejemplo", "semilla", "seed":
                var db = api.leerDB()
                for (k, v) in APILocal.semilla() where db[k] == nil { db[k] = v }
                api.guardarDB(db)
                return "listo: /usuarios /tareas /memorias /config — prueba: http localhost/usuarios\n"

            case "crea":
                guard let col = resto.first else { throw ShErr("uso: api-local crea <coleccion> ['{\"campo\":1}' …]") }
                var db = api.leerDB()
                var lista = db[col] as? [[String: Any]] ?? []
                var n = 0
                for t in resto.dropFirst() {
                    guard var o = APILocal.objeto(t) else { throw ShErr("api-local: no es JSON ni a=1&b=2: \(t)") }
                    if o["id"] == nil { o["id"] = (lista.compactMap { Int(APILocal.texto($0["id"])) }.max() ?? 0) + 1 }
                    lista.append(o); n += 1
                }
                db[col] = lista
                api.guardarDB(db)
                return "/\(col) lista (\(lista.count) elementos\(n > 0 ? ", \(n) nuevos" : ""))\n"

            case "datos", "db":
                let db = api.leerDB()
                if let col = resto.first {
                    guard let v = db[col] else { throw ShErr("api-local: no existe '\(col)'") }
                    return json(v)
                }
                return json(db)

            case "borra":
                guard let col = resto.first else { throw ShErr("uso: api-local borra <coleccion>") }
                var db = api.leerDB()
                guard db.removeValue(forKey: col) != nil else { throw ShErr("api-local: no existe '\(col)'") }
                api.guardarDB(db)
                return "borrada /\(col)\n"

            case "ruta":
                guard resto.count >= 3 else {
                    throw ShErr("uso: api-local ruta <GET|POST|…|*> /ruta/:param <respuesta> [--estado N]  ·  o  --ejecuta <comando>")
                }
                let metodo = resto[0].uppercased()
                var ruta = resto[1]
                if !ruta.hasPrefix("/") { ruta = "/" + ruta }
                var cuerpo = Array(resto.dropFirst(2))
                var estado = 200
                if let i = cuerpo.firstIndex(of: "--estado"), i + 1 < cuerpo.count, let n = Int(cuerpo[i + 1]) {
                    estado = n; cuerpo.removeSubrange(i ... i + 1)
                }
                var nueva = APILocal.Ruta(metodo: metodo, ruta: ruta, responde: nil, ejecuta: nil, estado: estado, dueño: ctx.sh.quienPide)
                if cuerpo.first == "--ejecuta" {
                    let cmd = cuerpo.dropFirst().joined(separator: " ")
                    guard !cmd.isEmpty else { throw ShErr("api-local ruta: falta el comando después de --ejecuta") }
                    nueva.ejecuta = cmd
                } else {
                    nueva.responde = cuerpo.joined(separator: " ")
                }
                var rutas = api.leerRutas()
                rutas.removeAll { $0.metodo == metodo && $0.ruta == ruta }
                rutas.append(nueva)
                api.guardarRutas(rutas)
                return "ruta \(metodo) \(ruta) → " + (nueva.ejecuta.map { "ejecuta '\($0)' en la shell de \(nueva.dueño)" } ?? "responde \(estado)") + "\n"

            case "rutas":
                let rutas = api.leerRutas()
                guard !rutas.isEmpty else { return "sin rutas propias — api-local ruta GET /hola '{\"hola\":1}'\n" }
                return rutas.enumerated().map { i, r in
                    "\(i + 1). \(r.metodo) \(r.ruta) [\(r.estado)] (\(r.dueño)) → " + (r.ejecuta.map { "ejecuta: \($0)" } ?? String((r.responde ?? "").prefix(50)))
                }.joined(separator: "\n") + "\n"

            case "quita-ruta", "borra-ruta":
                guard let n = Int(resto.first ?? ""), n >= 1 else { throw ShErr("uso: api-local quita-ruta <número> (mira 'api-local rutas')") }
                var rutas = api.leerRutas()
                guard n <= rutas.count else { throw ShErr("api-local: no hay ruta \(n)") }
                let r = rutas.remove(at: n - 1)
                api.guardarRutas(rutas)
                return "quitada \(r.metodo) \(r.ruta)\n"

            case "retraso":
                guard let ms = Int(resto.first ?? "") else { throw ShErr("uso: api-local retraso <milisegundos>") }
                api.configurar(retraso: ms)
                return ms > 0 ? "cada respuesta tardará \(api.retrasoMs) ms\n" : "sin retraso\n"

            case "fallo", "fallos":
                guard var t = resto.first else { throw ShErr("uso: api-local fallo <10%|0.1|0>") }
                let porciento = t.hasSuffix("%")
                if porciento { t.removeLast() }
                guard var p = Double(t) else { throw ShErr("api-local fallo: número no válido") }
                if porciento || p > 1 { p /= 100 }
                api.configurar(fallo: p)
                return p > 0 ? "\(Int(api.probFallo * 100))% de las peticiones darán 500\n" : "sin fallos simulados\n"

            case "log":
                let n = Int(resto.first ?? "20") ?? 20
                let ps = api.ultimas(n)
                guard !ps.isEmpty else { return "nadie ha llamado todavía\n" }
                return ps.map {
                    "\($0.hora) \($0.quien.padding(toLength: 11, withPad: " ", startingAt: 0)) \($0.metodo.padding(toLength: 6, withPad: " ", startingAt: 0)) \($0.ruta)  → \($0.estado) (\($0.ms) ms)"
                }.joined(separator: "\n") + "\n"

            case "importa":
                guard let p = resto.first else { throw ShErr("uso: api-local importa <archivo.json>") }
                let t = try ctx.input([p])
                guard let d = t.data(using: .utf8), let o = try? JSONSerialization.jsonObject(with: d) as? [String: Any] else {
                    throw ShErr("api-local: \(p) no es un objeto JSON {\"coleccion\": [...]}")
                }
                api.guardarDB(o)
                return "importadas \(o.count) colecciones de \(p)\n"

            case "exporta":
                guard let p = resto.first else { throw ShErr("uso: api-local exporta <archivo.json>") }
                try json(api.leerDB()).write(to: try ctx.env.resolve(p), atomically: true, encoding: .utf8)
                return "exportado a \(p)\n"

            case "reinicia":
                api.guardarDB([:])
                return "datos borrados (las rutas propias siguen; 'api-local rutas')\n"

            case "apaga":
                api.configurar(encendido: false)
                return "emulador apagado: localhost no responde\n"

            case "enciende":
                api.configurar(encendido: true)
                return "emulador encendido en http://localhost\n"

            default:
                throw ShErr("api-local: no conozco '\(sub)' — mira 'api-local ayuda'")
            }
    }

    /// http: peticiones cómodas, al estilo HTTPie.
    static func ordenHttp(_ ctx: Ctx) async throws -> String {
            var a = ctx.args
            let verbos: Set<String> = ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"]
            var metodo: String? = nil
            var soloCuerpo = false
            if let i = a.firstIndex(of: "-b") { soloCuerpo = true; a.remove(at: i) }
            if let f = a.first, verbos.contains(f.uppercased()) { metodo = f.uppercased(); a.removeFirst() }
            guard !a.isEmpty else { throw ShErr("uso: http [GET|POST|PUT|PATCH|DELETE] <url> [campo=valor campo:=json Cabecera:valor]") }
            let url = a.removeFirst()
            var cuerpo: [String: Any] = [:]
            var cabeceras: [String] = []
            for item in a {
                if let r = item.range(of: ":=") {
                    let k = String(item[item.startIndex ..< r.lowerBound]), v = String(item[r.upperBound...])
                    cuerpo[k] = (try? JSONSerialization.jsonObject(with: Data(v.utf8), options: [.fragmentsAllowed])) ?? v
                } else if let r = item.range(of: "=") {
                    cuerpo[String(item[item.startIndex ..< r.lowerBound])] = String(item[r.upperBound...])
                } else if let r = item.range(of: ":") {
                    cabeceras.append(String(item[item.startIndex ..< r.lowerBound]) + ": " + String(item[r.upperBound...]))
                }
            }
            var texto: String? = nil
            if !ctx.stdin.isEmpty, cuerpo.isEmpty { texto = ctx.stdin }
            if !cuerpo.isEmpty, let d = try? JSONSerialization.data(withJSONObject: cuerpo) { texto = String(data: d, encoding: .utf8) }
            let m = metodo ?? (texto == nil ? "GET" : "POST")
            let (datos, resp) = try await Shell.peticion(url, metodo: m, cabeceras: cabeceras + ["Content-Type: application/json"],
                                                         cuerpo: texto, quien: ctx.sh.quienPide)
            var out = ""
            if !soloCuerpo {
                let nombre = HTTPURLResponse.localizedString(forStatusCode: resp.statusCode)
                out += "HTTP \(resp.statusCode) \(nombre)\n"
            }
            if let o = try? JSONSerialization.jsonObject(with: datos, options: [.fragmentsAllowed]),
               let d = try? JSONSerialization.data(withJSONObject: o, options: [.prettyPrinted, .sortedKeys, .fragmentsAllowed]),
               let s = String(data: d, encoding: .utf8) {
                out += s + "\n"
            } else {
                let s = String(data: datos, encoding: .utf8) ?? "(\(datos.count) bytes)"
                out += s + (s.hasSuffix("\n") ? "" : "\n")
            }
            ctx.env.vars["HTTP_STATUS"] = String(resp.statusCode)
            if resp.statusCode >= 400 { throw ShErr(out.trimmingCharacters(in: .newlines)) }
            return out
    }

    /// Una petición: la contesta el emulador si es para localhost; si no, la red.
    static func peticion(_ url: String, metodo: String, cabeceras: [String], cuerpo: String?, quien: String) async throws -> (Data, HTTPURLResponse) {
        if let r = try await APILocal.uno.atender(url: url, metodo: metodo, cabeceras: cabeceras, cuerpo: cuerpo, quien: quien) {
            return r
        }
        var s = url
        if !s.contains("://") { s = "https://" + s }
        guard let u = URL(string: s) else { throw ShErr("URL no válida: \(url)") }
        var req = URLRequest(url: u)
        req.httpMethod = metodo
        req.timeoutInterval = 30
        for h in cabeceras {
            guard let i = h.firstIndex(of: ":") else { continue }
            req.setValue(String(h[h.index(after: i)...]).trimmingCharacters(in: .whitespaces),
                         forHTTPHeaderField: String(h[h.startIndex ..< i]))
        }
        if let b = cuerpo { req.httpBody = Data(b.utf8) }
        let (d, r) = try await URLSession.shared.data(for: req)
        guard let http = r as? HTTPURLResponse else { throw ShErr("respuesta no HTTP") }
        return (d, http)
    }
}
