// Junta la app entera en un solo archivo HTML (web/especie-abla.html):
// la página, el puente, el sistema de archivos y el cerebro en C (WebAssembly en base64).
// Así se puede abrir sin depender de que el visor encuentre los demás archivos.
import { readFileSync, writeFileSync } from "node:fs";

const web = new URL("../web/", import.meta.url);
const leer = (n) => readFileSync(new URL(n, web), "utf8");
let html = leer("index.html");
for (const js of ["wasi.js", "abla.js", "abla-wasm.js", "app.js"]) {
  const etiqueta = `<script src="${js}"></script>`;
  if (!html.includes(etiqueta)) throw new Error("no encuentro " + etiqueta);
  // "</script" dentro del código cerraría la etiqueta antes de tiempo
  const codigo = leer(js).replace(/<\/script/gi, "<\\/script");
  html = html.replace(etiqueta, () => `<script>\n${codigo}\n</script>`);
}
writeFileSync(new URL("especie-abla.html", web), html);
console.log(`especie-abla.html: ${(html.length / 1024).toFixed(0)} KB, todo en un archivo`);
