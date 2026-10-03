// Puente entre la interfaz y el cerebro en C (abla.wasm).
// El cerebro vive en WebAssembly; aquí solo se le pasan textos y se leen sus respuestas.
(function (global) {
  "use strict";
  const utf8 = new TextEncoder();
  const deUtf8 = new TextDecoder();

  function base64ABytes(b64) {
    const bin = atob(b64);
    const out = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
    return out;
  }

  class Cerebro {
    static async despertar({ wasm, fs, contexto = 1000000, alEscribir } = {}) {
      const { crearWasi, SistemaArchivos } = global.AblaWasi;
      fs = fs || (await SistemaArchivos.cargar());
      const wasi = crearWasi(fs, alEscribir || ((t) => console.log("[abla]", t)));
      const bytes = wasm || base64ABytes(global.ABLA_WASM_B64);
      const { instance } = await WebAssembly.instantiate(bytes, { wasi_snapshot_preview1: wasi.imports });
      wasi.usarMemoria(instance.exports.memory);
      instance.exports._initialize();
      const c = new Cerebro(instance.exports, fs);
      c.x.abla_iniciar_web(contexto);
      return c;
    }

    constructor(exports, fs) {
      this.x = exports;
      this.fs = fs;
    }

    cadena(ptr) {
      const mem = new Uint8Array(this.x.memory.buffer);
      let fin = ptr;
      while (mem[fin]) fin++;
      return deUtf8.decode(mem.subarray(ptr, fin));
    }

    conCadena(texto, fn) {
      const b = utf8.encode(texto);
      const p = this.x.abla_buffer(b.length + 1);
      const mem = new Uint8Array(this.x.memory.buffer);
      mem.set(b, p);
      mem[p + b.length] = 0;
      try {
        return fn(p);
      } finally {
        this.x.abla_liberar(p);
      }
    }

    ciclo() { this.x.abla_ciclo(); }
    ejecutar(linea) { return this.conCadena(linea, (p) => this.cadena(this.x.abla_ejecutar(p))); }
    prompt() { return this.cadena(this.x.abla_prompt()); }
    estado() { return JSON.parse(this.cadena(this.x.abla_estado_json())); }
    corriente(desde) { return JSON.parse(this.cadena(this.x.abla_corriente_json(desde))); }
    red(desde) { return JSON.parse(this.cadena(this.x.abla_red_json(desde))); }
    mente(id, n) { return JSON.parse(this.cadena(this.x.abla_mente_json(id, n))); }
    guardar() { this.x.abla_guardar(); return this.fs.guardar(); }
    ritmo() { return this.x.abla_ritmo(); }
    fijarRitmo(ms) { this.x.abla_fijar_ritmo(ms); }
  }

  global.Cerebro = Cerebro;
})(typeof window !== "undefined" ? window : globalThis);
