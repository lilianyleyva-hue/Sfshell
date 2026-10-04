import Foundation

// Comandos.swift — qué orden hace qué, y la ayuda.

extension Consola {
    static let ordenes: Set<String> = [
        "help", "ayuda", "man", "ls", "cd", "pwd", "mkdir", "rmdir", "rm", "cp", "mv", "touch", "cat", "tree", "find",
        "du", "df", "stat", "file", "basename", "dirname", "realpath", "chmod", "ln", "echo", "printf", "head", "tail",
        "wc", "grep", "egrep", "sort", "uniq", "cut", "tr", "sed", "rev", "tee", "diff", "base64", "seq", "yes", "nl",
        "clear", "history", "alias", "unalias", "export", "env", "printenv", "set", "unset", "which", "type", "whoami",
        "hostname", "uname", "uptime", "date", "cal", "sleep", "expr", "calc", "bc", "true", "false", "test", "[",
        "curl", "wget", "sha256sum", "pkg", "apt", "neofetch", "ps", "kill", "exit", "time", "sh", "bash", "source", ".",
        "xargs", "nano", "edit", "nyx", "python", "pip", "pip3", "node", "js", "gcc", "cc", "clang", "g++", "c++",
        "clang++", "javac", "java", "run"]

    /// Ejecuta una orden ya expandida.
    func ejecutaComando(_ c: Contexto) async -> Int32 {
        let n = c.nombre
        if n.hasPrefix("./") || n.hasPrefix("/") || n.hasPrefix("~/") || (n.contains("/") && existe(n)) {
            return await cmdEjecutable(c)
        }
        if let r = ordenArchivos(c) { return r }
        if let r = ordenTexto(c) { return r }
        if let r = ordenSistema(c) { return r }
        if let r = await ordenAsincrona(c) { return r }
        c.error("nyxsh: \(n): orden no encontrada (escribe «help»)\n")
        return 127
    }

    private func ordenArchivos(_ c: Contexto) -> Int32? {
        switch c.nombre {
        case "ls": return cmdLs(c)
        case "cd": return cmdCd(c)
        case "pwd": return cmdPwd(c)
        case "mkdir": return cmdMkdir(c)
        case "rmdir": return cmdRmdir(c)
        case "rm": return cmdRm(c)
        case "cp": return cmdCopiaMueve(c, mover: false)
        case "mv": return cmdCopiaMueve(c, mover: true)
        case "touch": return cmdTouch(c)
        case "tree": return cmdTree(c)
        case "find": return cmdFind(c)
        case "du": return cmdDu(c)
        case "df": return cmdDf(c)
        case "stat": return cmdStat(c)
        case "file": return cmdFile(c)
        case "basename": return cmdBasename(c)
        case "dirname": return cmdDirname(c)
        case "realpath": return cmdRealpath(c)
        case "chmod": return 0
        case "ln": c.error("ln: en el disco de Nyx no hay enlaces; usa cp\n"); return 1
        default: return nil
        }
    }

    private func ordenTexto(_ c: Contexto) -> Int32? {
        switch c.nombre {
        case "cat": return cmdCat(c)
        case "nl":
            var c2 = c
            c2.args = ["-n"] + c.args
            return cmdCat(c2)
        case "echo": return cmdEcho(c)
        case "printf": return cmdPrintf(c)
        case "head": return cmdHeadTail(c, cola: false)
        case "tail": return cmdHeadTail(c, cola: true)
        case "wc": return cmdWc(c)
        case "grep", "egrep": return cmdGrep(c)
        case "sort": return cmdSort(c)
        case "uniq": return cmdUniq(c)
        case "cut": return cmdCut(c)
        case "tr": return cmdTr(c)
        case "sed": return cmdSed(c)
        case "rev": return cmdRev(c)
        case "tee": return cmdTee(c)
        case "diff": return cmdDiff(c)
        case "base64": return cmdBase64(c)
        case "seq": return cmdSeq(c)
        case "yes": return cmdYes(c)
        default: return nil
        }
    }

    private func ordenSistema(_ c: Contexto) -> Int32? {
        switch c.nombre {
        case "help", "ayuda": return cmdHelp(c)
        case "man": return cmdMan(c)
        case "clear":
            alLimpiar?()
            return 0
        case "history": return cmdHistory(c)
        case "alias": return cmdAlias(c)
        case "unalias":
            for a in c.args { alias[a] = nil }
            return 0
        case "export": return cmdExport(c)
        case "env", "printenv", "set": return cmdEnv(c)
        case "unset":
            for a in c.args { entorno[a] = nil }
            return 0
        case "which", "type": return cmdWhich(c)
        case "whoami": c.escribe("nyx\n"); return 0
        case "hostname": c.escribe("ipad\n"); return 0
        case "uname": return cmdUname(c)
        case "uptime": return cmdUptime(c)
        case "date": return cmdDate(c)
        case "cal": return cmdCal(c)
        case "expr", "calc", "bc": return cmdCalc(c)
        case "true": return 0
        case "false": return 1
        case "test", "[": return cmdTest(c)
        case "sha256sum": return cmdSha256(c)
        case "pkg", "apt": return cmdPkg(c)
        case "neofetch": return cmdNeofetch(c)
        case "ps": return cmdPs(c)
        case "kill": c.escribe("(no hay otros procesos: para un programa usa el botón ⏹)\n"); return 0
        case "exit": alLimpiar?(); return 0
        case "nano", "edit": return cmdNano(c)
        case "pip", "pip3": return cmdPip(c)
        case "gcc", "cc", "clang": return cmdCompilaC(c, lenguaje: .c)
        case "g++", "c++", "clang++": return cmdCompilaC(c, lenguaje: .cpp)
        case "javac": return cmdJavac(c)
        default: return nil
        }
    }

    private func ordenAsincrona(_ c: Contexto) async -> Int32? {
        switch c.nombre {
        case "sleep": return await cmdSleep(c)
        case "curl", "wget": return await cmdCurl(c)
        case "time": return await cmdTime(c)
        case "sh", "bash", "source", ".": return await cmdSh(c)
        case "xargs": return await cmdXargs(c)
        case "nyx": return await cmdNyx(c)
        case "python": return await cmdPython(c)
        case "node", "js": return await cmdNode(c)
        case "java": return await cmdJava(c)
        case "run": return await cmdRun(c)
        default: return nil
        }
    }

    // MARK: ayuda

    func cmdHelp(_ c: Contexto) -> Int32 {
        c.escribe("""
        nyxsh — la terminal de Nyx (estilo Termux). Tu casa es ~ (/home/nyx).

        📁 Archivos   ls [-la] · cd · pwd · mkdir [-p] · rm [-r] · cp [-r] · mv · touch · cat [-n]
                     tree · find [-name] [-type] · du [-h] · df · stat · file · basename · dirname
        📝 Texto      echo · printf · head/tail [-n] · wc [-lwc] · grep [-inrvc] · sort [-nru]
                     uniq [-c] · cut -d -f · tr · sed 's/a/b/g' · rev · tee · diff · base64 · seq · nl
        ✏️ Editor     nano archivo   (abre el editor de Nyx; también edit, vi, vim)
        🐍 Lenguajes  python archivo.py · python -c "print(2**10)"
                     node archivo.js · node -e "console.log([1,2,3].map(x=>x*2))"
                     gcc prog.c -o prog && ./prog · g++ prog.cpp && ./a.out
                     javac Main.java && java Main · java Main.java · run archivo
        🌐 Red        curl URL [-o archivo] [-I] · wget URL
        ⚙️ Sistema    clear · history · alias · export A=1 · env · which · date · cal · uptime
                     uname -a · sleep · calc 2^100 · sha256sum · time orden · sh guion.sh
                     xargs · neofetch · pkg list
        🌌 Nyx        nyx ¿qué es un motor? · nyx piensa … · nyx aprende … · nyx imagina …
                     nyx lee archivo · nyx guarda memoria.txt · nyx carga memoria.txt
        🔗 Shell      tuberías  a | b      redirigir  > >> < 2> 2>&1
                     encadenar  a && b   a || b   a ; b      variables $HOME $? $(orden)
                     comodines  *.py     comillas '…' "…"    ⏹ para el programa

        """)
        return 0
    }

    func cmdMan(_ c: Contexto) -> Int32 {
        guard let n = c.args.first else { return cmdHelp(c) }
        let ayudas: [String: String] = [
            "ls": "ls [-l] [-a] [-h] [-t] [-r] [camino…] — lista los archivos",
            "grep": "grep [-i] [-n] [-v] [-c] [-r] [-w] [-F] patrón [archivos…] — busca líneas (el patrón es una expresión regular)",
            "sed": "sed 's/viejo/nuevo/g' [archivo] · sed '/patrón/d' · sed -n '3p' · sed -i (cambia el archivo)",
            "find": "find [carpeta] [-name '*.py'] [-type f|d]",
            "python": "python archivo.py [args] · python -c código · echo 5 | python prog.py (input() lee la tubería)",
            "node": "node archivo.js · node -e código · node -p expresión (JavaScriptCore, el motor de Safari)",
            "gcc": "gcc prog.c [-o nombre] — comprueba el programa y crea el ejecutable (./a.out)",
            "java": "java Main (tras javac Main.java) · java Main.java",
            "nyx": "nyx pregunta · nyx piensa · nyx aprende · nyx imagina · nyx lee · nyx guarda · nyx carga · nyx estado",
            "curl": "curl URL [-o archivo] [-s] [-I] [-X MÉTODO] [-d datos] [-H 'Cabecera: valor']",
        ]
        c.escribe((ayudas[n] ?? "\(n): escribe «help» para ver todas las órdenes") + "\n")
        return 0
    }
}
