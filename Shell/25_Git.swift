import Foundation

// ============================================================
// MARK: - git: control de versiones local, de verdad
// ============================================================
// No hay red ni protocolo git real — eso necesitaría un servidor
// remoto al que conectarse, y aquí no hay ninguno — así que no hay
// 'push' ni 'pull' ni 'clone' de un remoto. Pero el historial, las
// ramas y los commits sí son reales: se guardan en .git-sim dentro
// del directorio, con la misma idea que los objetos de git, solo
// que en JSON en vez de en el formato binario de git.
//
// git init
// git add archivo.txt   (o: git add .)
// git commit -m "mensaje"
// git status
// git log
// git diff [archivo]
// git branch [nombre]
// git checkout <rama|commit>
// git show [commit]

struct GitCommit: Codable {
    let id: String
    let message: String
    let date: String
    let parent: String?
    let files: [String: String]   // ruta relativa -> contenido
}

struct GitRepo: Codable {
    var commits: [GitCommit] = []
    var branches: [String: String] = ["main": ""]   // rama -> id de commit (vacío = sin commits)
    var head: String = "main"
    var staged: [String: String] = [:]
}

extension Shell {

    static let gitDir = ".git-sim"

    static func gitLoad(_ ctx: Ctx) -> GitRepo? {
        guard let u = try? ctx.env.resolve("\(Shell.gitDir)/repo.json"),
              let d = FileManager.default.contents(atPath: u.path),
              let r = try? JSONDecoder().decode(GitRepo.self, from: d) else { return nil }
        return r
    }

    static func gitSave(_ repo: GitRepo, _ ctx: Ctx) throws {
        let u = try ctx.env.resolve("\(Shell.gitDir)/repo.json")
        let d = try JSONEncoder().encode(repo)
        try d.write(to: u)
    }

    /// Todos los archivos de texto del directorio actual (sin contar .git-sim ni ocultos).
    static func gitWorkingFiles(_ ctx: Ctx) -> [String: String] {
        let fm = FileManager.default
        guard let items = try? fm.contentsOfDirectory(atPath: ctx.env.cwd.path) else { return [:] }
        var out: [String: String] = [:]
        for name in items where name != Shell.gitDir && !name.hasPrefix(".") {
            let u = ctx.env.cwd.appendingPathComponent(name)
            var isDir: ObjCBool = false
            fm.fileExists(atPath: u.path, isDirectory: &isDir)
            guard !isDir.boolValue, let d = fm.contents(atPath: u.path), let s = String(data: d, encoding: .utf8) else { continue }
            out[name] = s
        }
        return out
    }

    static func git() -> [String: Spec] {
        var c: [String: Spec] = [:]
        let fm = FileManager.default

        func requireRepo(_ ctx: Ctx) throws -> GitRepo {
            guard let r = gitLoad(ctx) else { throw ShErr("git: esto no es un repositorio (usa 'git init')") }
            return r
        }

        func lastFiles(_ repo: GitRepo) -> [String: String] {
            guard let id = repo.branches[repo.head] else { return [:] }
            return repo.commits.first(where: { $0.id == id })?.files ?? [:]
        }

        c["git"] = Spec(help: "git <init|status|add|commit|log|diff|checkout|branch|show> — control de versiones local") { ctx in
            let sub = ctx.args.first ?? "status"
            let rest = Array(ctx.args.dropFirst())

            switch sub {
            case "init":
                guard gitLoad(ctx) == nil else { return "ya hay un repositorio aquí\n" }
                let dir = try ctx.env.resolve(Shell.gitDir)
                try fm.createDirectory(at: dir, withIntermediateDirectories: true)
                try gitSave(GitRepo(), ctx)
                return "repositorio creado en \(ctx.env.vpath(ctx.env.cwd))/\(Shell.gitDir)\n"

            case "status":
                let repo = try requireRepo(ctx)
                let working = Shell.gitWorkingFiles(ctx)
                let base = lastFiles(repo)
                var nuevo: [String] = [], modificado: [String] = [], borrado: [String] = []
                for (k, v) in working {
                    if let old = base[k] {
                        if old != v, repo.staged[k] == nil { modificado.append(k) }
                    } else if repo.staged[k] == nil {
                        nuevo.append(k)
                    }
                }
                for k in base.keys where working[k] == nil { borrado.append(k) }
                var out = "rama \(repo.head)\n"
                if !repo.staged.isEmpty {
                    out += "\npreparados para el commit:\n" + repo.staged.keys.sorted().map { "  \($0)" }.joined(separator: "\n") + "\n"
                }
                if !modificado.isEmpty {
                    out += "\nmodificados sin preparar:\n" + modificado.sorted().map { "  \($0)" }.joined(separator: "\n") + "\n"
                }
                if !nuevo.isEmpty {
                    out += "\nsin seguimiento:\n" + nuevo.sorted().map { "  \($0)" }.joined(separator: "\n") + "\n"
                }
                if !borrado.isEmpty {
                    out += "\nborrados desde el último commit:\n" + borrado.sorted().map { "  \($0)" }.joined(separator: "\n") + "\n"
                }
                if repo.staged.isEmpty, modificado.isEmpty, nuevo.isEmpty, borrado.isEmpty {
                    out += "\nnada que confirmar, el árbol está limpio\n"
                }
                return out

            case "add":
                var repo = try requireRepo(ctx)
                let working = Shell.gitWorkingFiles(ctx)
                guard let target = rest.first else { throw ShErr("git add: falta el archivo (o usa '.')") }
                if target == "." {
                    repo.staged = working
                } else {
                    guard let contenido = working[target] else { throw ShErr("git add: \(target): no existe") }
                    repo.staged[target] = contenido
                }
                try gitSave(repo, ctx)
                return ""

            case "commit":
                var repo = try requireRepo(ctx)
                guard !repo.staged.isEmpty else { throw ShErr("git commit: no hay nada preparado (usa 'git add')") }
                let o = opts(rest, valued: ["m"])
                guard let msg = o.vals["m"], !msg.isEmpty else { throw ShErr("git commit: usa -m \"mensaje\"") }
                let parent = repo.branches[repo.head]
                let parentId: String? = (parent?.isEmpty ?? true) ? nil : parent
                var files = lastFiles(repo)
                for (k, v) in repo.staged { files[k] = v }
                let df = DateFormatter(); df.dateFormat = "yyyy-MM-dd HH:mm:ss"
                let id = String(UUID().uuidString.prefix(7)).lowercased()
                let commit = GitCommit(id: id, message: msg, date: df.string(from: Date()), parent: parentId, files: files)
                repo.commits.append(commit)
                repo.branches[repo.head] = id
                repo.staged = [:]
                try gitSave(repo, ctx)
                return "[\(repo.head) \(id)] \(msg)\n"

            case "log":
                let repo = try requireRepo(ctx)
                guard let start = repo.branches[repo.head], !start.isEmpty else { return "todavía no hay commits\n" }
                var id: String? = start
                var out = ""
                var vistos = Set<String>()
                while let cid = id, !vistos.contains(cid), let commit = repo.commits.first(where: { $0.id == cid }) {
                    vistos.insert(cid)
                    out += "commit \(commit.id)\nfecha    \(commit.date)\n\n    \(commit.message)\n\n"
                    id = commit.parent
                }
                return out

            case "diff":
                let repo = try requireRepo(ctx)
                let working = Shell.gitWorkingFiles(ctx)
                let base = lastFiles(repo)
                let objetivo = rest.first
                var out = ""
                for (k, v) in working where objetivo == nil || objetivo == k {
                    guard let old = base[k], old != v else { continue }
                    out += "--- \(k) (último commit)\n+++ \(k) (actual)\n"
                    let a = old.components(separatedBy: "\n")
                    let b = v.components(separatedBy: "\n")
                    for i in 0..<max(a.count, b.count) {
                        let x = i < a.count ? a[i] : nil
                        let y = i < b.count ? b[i] : nil
                        if x == y { continue }
                        if let x = x { out += "-\(x)\n" }
                        if let y = y { out += "+\(y)\n" }
                    }
                }
                return out.isEmpty ? "sin diferencias\n" : out

            case "checkout":
                var repo = try requireRepo(ctx)
                guard let target = rest.first else { throw ShErr("git checkout: falta la rama o el commit") }
                var commit: GitCommit?
                if let id = repo.branches[target] {
                    repo.head = target
                    commit = repo.commits.first(where: { $0.id == id })
                } else {
                    commit = repo.commits.first(where: { $0.id == target })
                }
                guard let files = commit?.files else {
                    throw ShErr("git checkout: '\(target)' no es una rama ni un commit conocido")
                }
                for (k, v) in files {
                    if let u = try? ctx.env.resolve(k) {
                        try? v.write(to: u, atomically: true, encoding: .utf8)
                    }
                }
                try gitSave(repo, ctx)
                return "en \(target)\n"

            case "branch":
                var repo = try requireRepo(ctx)
                if let name = rest.first {
                    guard repo.branches[name] == nil else { throw ShErr("git branch: '\(name)' ya existe") }
                    repo.branches[name] = repo.branches[repo.head] ?? ""
                    try gitSave(repo, ctx)
                    return "rama \(name) creada desde \(repo.head)\n"
                }
                return repo.branches.keys.sorted().map { ($0 == repo.head ? "* " : "  ") + $0 }.joined(separator: "\n") + "\n"

            case "show":
                let repo = try requireRepo(ctx)
                let id = rest.first ?? repo.branches[repo.head]
                guard let cid = id, let commit = repo.commits.first(where: { $0.id == cid }) else {
                    throw ShErr("git show: no existe ese commit")
                }
                var out = "commit \(commit.id)\nfecha    \(commit.date)\n\n    \(commit.message)\n\narchivos:\n"
                out += commit.files.keys.sorted().map { "  \($0)" }.joined(separator: "\n") + "\n"
                return out

            default:
                throw ShErr("git: usa init, status, add, commit, log, diff, checkout, branch o show.\n" +
                            "Esto es un git local y de verdad dentro del sandbox: no hay 'push'/'pull'/'clone'\n" +
                            "de un remoto porque no hay ningún servidor git al que una app pueda conectarse así.")
            }
        }

        return c
    }
}
