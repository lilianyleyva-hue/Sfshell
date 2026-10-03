// Mini-WASI para el navegador: el cerebro en C (abla.wasm) usa fopen, mkdir,
// opendir… como en un disco de verdad. Aquí esas llamadas van a un sistema de
// archivos en memoria que se guarda en el navegador (IndexedDB).
//
// Script clásico (sin módulos) para que funcione también abriendo el archivo
// directamente, sin servidor, en Code App o Safari.
(function (global) {
  "use strict";

  const E = { EXITO: 0, BADF: 8, EXISTE: 20, INVAL: 28, ESDIR: 31, NOENT: 44, NOSYS: 52, NODIR: 54, NOVACIO: 55 };
  const TIPO = { CARACTER: 2, DIR: 3, ARCHIVO: 4 };
  const utf8 = new TextEncoder();
  const deUtf8 = new TextDecoder();

  // ---------- Sistema de archivos en memoria ----------
  class SistemaArchivos {
    constructor() {
      this.nodos = new Map([["", { dir: true, mtime: Date.now() }]]);
      this.sucios = new Set();
    }

    static normalizar(base, rel) {
      const partes = [];
      for (const p of (rel.startsWith("/") ? rel : base + "/" + rel).split("/")) {
        if (!p || p === ".") continue;
        if (p === "..") partes.pop();
        else partes.push(p);
      }
      return partes.join("/");
    }
    static padre(ruta) {
      const i = ruta.lastIndexOf("/");
      return i < 0 ? "" : ruta.slice(0, i);
    }

    obtener(ruta) { return this.nodos.get(ruta); }
    marcar(ruta) { this.sucios.add(ruta); }
    poner(ruta, nodo) { this.nodos.set(ruta, nodo); this.marcar(ruta); }
    quitar(ruta) { this.nodos.delete(ruta); this.marcar(ruta); }
    hijos(ruta) {
      const pre = ruta ? ruta + "/" : "";
      const r = [];
      for (const k of this.nodos.keys())
        if (k !== ruta && k.startsWith(pre) && !k.slice(pre.length).includes("/")) r.push(k.slice(pre.length));
      return r.sort();
    }
    leerTexto(ruta) {
      const n = this.nodos.get(SistemaArchivos.normalizar("", ruta));
      return n && !n.dir ? deUtf8.decode(n.datos.subarray(0, n.tam)) : null;
    }
  }

  // ---------- Persistencia en IndexedDB ----------
  const BD = "especie-abla", ALMACEN = "archivos";
  function abrirBD() {
    return new Promise((ok, mal) => {
      if (!global.indexedDB) return mal(new Error("sin IndexedDB"));
      const p = indexedDB.open(BD, 1);
      p.onupgradeneeded = () => p.result.createObjectStore(ALMACEN);
      p.onsuccess = () => ok(p.result);
      p.onerror = () => mal(p.error);
    });
  }

  SistemaArchivos.cargar = async function () {
    const fs = new SistemaArchivos();
    try {
      fs.bd = await abrirBD();
      await new Promise((ok, mal) => {
        const tx = fs.bd.transaction(ALMACEN, "readonly");
        const cur = tx.objectStore(ALMACEN).openCursor();
        cur.onsuccess = () => {
          const c = cur.result;
          if (!c) return ok();
          const v = c.value;
          fs.nodos.set(c.key, v.dir ? { dir: true, mtime: v.mtime } : { dir: false, datos: v.datos, tam: v.datos.length, mtime: v.mtime });
          c.continue();
        };
        cur.onerror = () => mal(cur.error);
      });
      fs.persistente = true;
    } catch (e) {
      fs.persistente = false; // modo privado o sin permiso: solo en memoria
    }
    fs.sucios.clear();
    return fs;
  };

  SistemaArchivos.prototype.guardar = function () {
    if (!this.bd || !this.sucios.size) return Promise.resolve(0);
    const rutas = [...this.sucios];
    this.sucios.clear();
    return new Promise((ok) => {
      const tx = this.bd.transaction(ALMACEN, "readwrite");
      const al = tx.objectStore(ALMACEN);
      for (const r of rutas) {
        const n = this.nodos.get(r);
        if (!n) al.delete(r);
        else if (n.dir) al.put({ dir: true, mtime: n.mtime }, r);
        else al.put({ dir: false, datos: n.datos.slice(0, n.tam), mtime: n.mtime }, r);
      }
      tx.oncomplete = () => ok(rutas.length);
      tx.onerror = () => { rutas.forEach((r) => this.sucios.add(r)); ok(0); };
    });
  };

  SistemaArchivos.prototype.borrarTodo = function () {
    this.nodos = new Map([["", { dir: true, mtime: Date.now() }]]);
    this.sucios.clear();
    const bd = this.bd;
    this.bd = null; // nada más se guarda: ni el autoguardado ni el de al cerrar
    if (!bd) return Promise.resolve();
    return new Promise((ok) => {
      const tx = bd.transaction(ALMACEN, "readwrite");
      tx.objectStore(ALMACEN).clear();
      tx.oncomplete = ok;
      tx.onerror = ok;
    });
  };

  // ---------- WASI preview1 (lo que usa abla.wasm) ----------
  function crearWasi(fs, alEscribir) {
    let memoria = null;
    const dv = () => new DataView(memoria.buffer);
    const bytes = () => new Uint8Array(memoria.buffer);
    const texto = (p, n) => deUtf8.decode(bytes().subarray(p, p + n));
    const fds = new Map([
      [0, { tipo: "entrada" }],
      [1, { tipo: "salida" }],
      [2, { tipo: "salida" }],
      [3, { tipo: "dir", ruta: "", preabierto: "." }],
    ]);
    let siguienteFd = 4;

    const resolver = (fd, p, n) => {
      const d = fds.get(fd);
      if (!d || d.tipo !== "dir") return null;
      return SistemaArchivos.normalizar(d.ruta, texto(p, n));
    };
    const crecer = (nodo, tam) => {
      if (tam <= nodo.datos.length) return;
      const nuevo = new Uint8Array(Math.max(tam, nodo.datos.length * 2, 64));
      nuevo.set(nodo.datos.subarray(0, nodo.tam));
      nodo.datos = nuevo;
    };
    const escribirStat = (p, nodo, ruta) => {
      const v = dv();
      for (let i = 0; i < 64; i++) v.setUint8(p + i, 0);
      let ino = 0;
      for (const c of ruta) ino = (ino * 31 + c.charCodeAt(0)) >>> 0;
      v.setBigUint64(p + 8, BigInt(ino), true);
      v.setUint8(p + 16, nodo.dir ? TIPO.DIR : TIPO.ARCHIVO);
      v.setBigUint64(p + 24, 1n, true);
      v.setBigUint64(p + 32, BigInt(nodo.dir ? 0 : nodo.tam), true);
      const t = BigInt(nodo.mtime || 0) * 1000000n;
      v.setBigUint64(p + 40, t, true);
      v.setBigUint64(p + 48, t, true);
      v.setBigUint64(p + 56, t, true);
    };

    const imports = {
      args_get: () => E.EXITO,
      args_sizes_get: (pn, pt) => { dv().setUint32(pn, 0, true); dv().setUint32(pt, 0, true); return E.EXITO; },
      environ_get: () => E.EXITO,
      environ_sizes_get: (pn, pt) => { dv().setUint32(pn, 0, true); dv().setUint32(pt, 0, true); return E.EXITO; },
      random_get: (p, n) => { crypto.getRandomValues(bytes().subarray(p, p + n)); return E.EXITO; },
      proc_exit: (c) => { throw new Error("abla terminó con código " + c); },

      clock_time_get: (id, _prec, p) => {
        const ns = id === 0 ? BigInt(Date.now()) * 1000000n : BigInt(Math.round(performance.now() * 1e6));
        dv().setBigUint64(p, ns, true);
        return E.EXITO;
      },

      fd_prestat_get: (fd, p) => {
        const d = fds.get(fd);
        if (!d || !d.preabierto) return E.BADF;
        dv().setUint8(p, 0);
        dv().setUint32(p + 4, utf8.encode(d.preabierto).length, true);
        return E.EXITO;
      },
      fd_prestat_dir_name: (fd, p, n) => {
        const d = fds.get(fd);
        if (!d || !d.preabierto) return E.BADF;
        bytes().set(utf8.encode(d.preabierto).subarray(0, n), p);
        return E.EXITO;
      },
      fd_fdstat_get: (fd, p) => {
        const d = fds.get(fd);
        if (!d) return E.BADF;
        const v = dv();
        v.setUint8(p, d.tipo === "dir" ? TIPO.DIR : d.tipo === "archivo" ? TIPO.ARCHIVO : TIPO.CARACTER);
        v.setUint16(p + 2, d.anexar ? 1 : 0, true);
        v.setBigUint64(p + 8, 0xffffffffffffffffn, true);
        v.setBigUint64(p + 16, 0xffffffffffffffffn, true);
        return E.EXITO;
      },
      fd_fdstat_set_flags: () => E.EXITO,
      fd_close: (fd) => (fds.delete(fd) ? E.EXITO : E.BADF),

      fd_write: (fd, iovs, n, pEscritos) => {
        const d = fds.get(fd);
        if (!d) return E.BADF;
        const v = dv();
        let total = 0;
        const trozos = [];
        for (let i = 0; i < n; i++) {
          const ptr = v.getUint32(iovs + i * 8, true), len = v.getUint32(iovs + i * 8 + 4, true);
          trozos.push(bytes().slice(ptr, ptr + len));
          total += len;
        }
        if (d.tipo === "salida") {
          if (alEscribir) alEscribir(trozos.map((t) => deUtf8.decode(t)).join(""));
        } else if (d.tipo === "archivo") {
          const nodo = fs.obtener(d.ruta);
          if (!nodo) return E.BADF;
          if (d.anexar) d.pos = nodo.tam;
          crecer(nodo, d.pos + total);
          for (const t of trozos) { nodo.datos.set(t, d.pos); d.pos += t.length; }
          nodo.tam = Math.max(nodo.tam, d.pos);
          nodo.mtime = Date.now();
          fs.marcar(d.ruta);
        } else return E.BADF;
        dv().setUint32(pEscritos, total, true);
        return E.EXITO;
      },

      fd_read: (fd, iovs, n, pLeidos) => {
        const d = fds.get(fd);
        if (!d) return E.BADF;
        let total = 0;
        if (d.tipo === "archivo") {
          const nodo = fs.obtener(d.ruta);
          const v = dv();
          for (let i = 0; i < n; i++) {
            const ptr = v.getUint32(iovs + i * 8, true), len = v.getUint32(iovs + i * 8 + 4, true);
            const k = Math.max(0, Math.min(len, nodo.tam - d.pos));
            bytes().set(nodo.datos.subarray(d.pos, d.pos + k), ptr);
            d.pos += k;
            total += k;
            if (k < len) break;
          }
        }
        dv().setUint32(pLeidos, total, true);
        return E.EXITO;
      },

      fd_seek: (fd, desplazamiento, desde, pNueva) => {
        const d = fds.get(fd);
        if (!d || d.tipo !== "archivo") return E.BADF;
        const nodo = fs.obtener(d.ruta);
        const base = desde === 0 ? 0 : desde === 1 ? d.pos : nodo.tam;
        const nueva = base + Number(desplazamiento);
        if (nueva < 0) return E.INVAL;
        d.pos = nueva;
        dv().setBigUint64(pNueva, BigInt(nueva), true);
        return E.EXITO;
      },

      fd_readdir: (fd, buf, lenBuf, cookie, pUsado) => {
        const d = fds.get(fd);
        if (!d || d.tipo !== "dir") return E.BADF;
        const nombres = fs.hijos(d.ruta);
        let usado = 0;
        for (let i = Number(cookie); i < nombres.length; i++) {
          const nombre = utf8.encode(nombres[i]);
          const nodo = fs.obtener(d.ruta ? d.ruta + "/" + nombres[i] : nombres[i]);
          const ent = new Uint8Array(24 + nombre.length);
          const ev = new DataView(ent.buffer);
          ev.setBigUint64(0, BigInt(i + 1), true);
          ev.setBigUint64(8, BigInt(i + 1), true);
          ev.setUint32(16, nombre.length, true);
          ev.setUint8(20, nodo && nodo.dir ? TIPO.DIR : TIPO.ARCHIVO);
          ent.set(nombre, 24);
          const cabe = Math.min(ent.length, lenBuf - usado);
          bytes().set(ent.subarray(0, cabe), buf + usado);
          usado += cabe;
          if (usado >= lenBuf) break;
        }
        dv().setUint32(pUsado, usado, true);
        return E.EXITO;
      },

      path_filestat_get: (fd, _flags, p, n, pStat) => {
        const ruta = resolver(fd, p, n);
        if (ruta === null) return E.BADF;
        const nodo = fs.obtener(ruta);
        if (!nodo) return E.NOENT;
        escribirStat(pStat, nodo, ruta);
        return E.EXITO;
      },
      fd_filestat_get: (fd, pStat) => {
        const d = fds.get(fd);
        if (!d) return E.BADF;
        const nodo = d.tipo === "salida" || d.tipo === "entrada" ? { dir: false, tam: 0 } : fs.obtener(d.ruta);
        escribirStat(pStat, nodo, d.ruta || "");
        return E.EXITO;
      },

      path_create_directory: (fd, p, n) => {
        const ruta = resolver(fd, p, n);
        if (ruta === null) return E.BADF;
        if (fs.obtener(ruta)) return E.EXISTE;
        const padre = fs.obtener(SistemaArchivos.padre(ruta));
        if (!padre) return E.NOENT;
        if (!padre.dir) return E.NODIR;
        fs.poner(ruta, { dir: true, mtime: Date.now() });
        return E.EXITO;
      },

      path_open: (fd, _dirflags, p, n, oflags, _rb, _ri, fdflags, pFd) => {
        const ruta = resolver(fd, p, n);
        if (ruta === null) return E.BADF;
        let nodo = fs.obtener(ruta);
        const CREAR = 1, DIRECTORIO = 2, EXCLUSIVO = 4, TRUNCAR = 8;
        if (nodo && (oflags & EXCLUSIVO) && (oflags & CREAR)) return E.EXISTE;
        if (!nodo) {
          if (!(oflags & CREAR)) return E.NOENT;
          const padre = fs.obtener(SistemaArchivos.padre(ruta));
          if (!padre) return E.NOENT;
          if (!padre.dir) return E.NODIR;
          nodo = { dir: false, datos: new Uint8Array(64), tam: 0, mtime: Date.now() };
          fs.poner(ruta, nodo);
        }
        if ((oflags & DIRECTORIO) && !nodo.dir) return E.NODIR;
        if (nodo.dir) {
          fds.set(siguienteFd, { tipo: "dir", ruta });
        } else {
          if (oflags & TRUNCAR) { nodo.tam = 0; nodo.mtime = Date.now(); fs.marcar(ruta); }
          fds.set(siguienteFd, { tipo: "archivo", ruta, pos: 0, anexar: !!(fdflags & 1) });
        }
        dv().setUint32(pFd, siguienteFd++, true);
        return E.EXITO;
      },

      path_unlink_file: (fd, p, n) => {
        const ruta = resolver(fd, p, n);
        if (ruta === null) return E.BADF;
        const nodo = fs.obtener(ruta);
        if (!nodo) return E.NOENT;
        if (nodo.dir) return E.ESDIR;
        fs.quitar(ruta);
        return E.EXITO;
      },
      path_remove_directory: (fd, p, n) => {
        const ruta = resolver(fd, p, n);
        if (ruta === null) return E.BADF;
        const nodo = fs.obtener(ruta);
        if (!nodo) return E.NOENT;
        if (!nodo.dir) return E.NODIR;
        if (fs.hijos(ruta).length) return E.NOVACIO;
        fs.quitar(ruta);
        return E.EXITO;
      },
      path_rename: (fd, p, n, fd2, p2, n2) => {
        const a = resolver(fd, p, n), b = resolver(fd2, p2, n2);
        if (a === null || b === null) return E.BADF;
        const nodo = fs.obtener(a);
        if (!nodo) return E.NOENT;
        if (!fs.obtener(SistemaArchivos.padre(b))) return E.NOENT;
        const mover = [...fs.nodos.keys()].filter((k) => k === a || k.startsWith(a + "/"));
        for (const k of mover) {
          const v = fs.obtener(k);
          fs.quitar(k);
          fs.poner(b + k.slice(a.length), v);
        }
        return E.EXITO;
      },
    };

    // Cualquier otra llamada WASI: «no implementada», en vez de romper al cargar.
    const proxy = new Proxy(imports, { get: (t, k) => (k in t ? t[k] : () => E.NOSYS) });
    return { imports: proxy, usarMemoria: (m) => { memoria = m; } };
  }

  global.AblaWasi = { SistemaArchivos, crearWasi };
})(typeof window !== "undefined" ? window : globalThis);
