// Mete abla.wasm dentro de abla-wasm.js (en base64) para que la página
// funcione también abierta como archivo, sin servidor (Code App, Safari).
import { readFileSync, writeFileSync } from "node:fs";

const dir = new URL(".", import.meta.url);
const wasm = readFileSync(new URL("abla.wasm", dir));
writeFileSync(
  new URL("abla-wasm.js", dir),
  "// Generado por empaquetar.mjs a partir de abla.wasm (el cerebro en C). No editar.\n" +
    `window.ABLA_WASM_B64 = "${wasm.toString("base64")}";\n`,
);
console.log(`abla-wasm.js: ${wasm.length} bytes de WebAssembly empaquetados`);
