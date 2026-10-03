// Interfaz de la Especie Abla. El pensamiento ocurre en C (abla.wasm);
// aquí solo se dibuja, se escucha y se le habla.
(function () {
  "use strict";

  const $ = (id) => document.getElementById(id);
  const TRIBUS = ["lenguaje", "datos", "lógica"];
  const COLOR = ["#ff6ad5", "#4fd1ff", "#b8ff6a"];
  const COLOR_ABLA = "#f2f0ff", COLOR_C = "#ffc44d";
  const reducirMovimiento = matchMedia("(prefers-reduced-motion: reduce)").matches;

  let cerebro = null, estado = null, elegido = null;
  let vistaCorriente = 0, vistaRed = 0;
  const actividad = new Array(27).fill(0); // brillo al pensar algo público
  const ultimos = [];
  const particulas = [];

  // ---------- arranque ----------
  async function iniciar() {
    try {
      cerebro = await Cerebro.despertar({ alEscribir: (t) => console.log("[abla]", t) });
    } catch (e) {
      $("cargando").textContent = "No pude despertar a la especie: " + e.message;
      console.error(e);
      return;
    }
    $("cargando").hidden = true;
    $("almacen").textContent = cerebro.fs.persistente
      ? "Su memoria se guarda en este navegador."
      : "Este navegador no deja guardar: olvidarán todo al cerrar.";
    $("ritmo").value = cerebro.ritmo();
    estado = cerebro.estado();
    construirTarjetas();
    construirSelects();
    construirEjemplos();
    escribirTerminal("Especie Abla · cerebro en C (WebAssembly)\nEscribe ayuda para ver los comandos.\n\n");
    actualizar(true);
    pensarSinParar();
    requestAnimationFrame(dibujar);
    // Cada 10 s el cerebro escribe su saber (.c) y su memoria, y se guardan en el navegador.
    // No se espera al cierre de la página: Safari no siempre deja guardar en ese momento.
    setInterval(() => cerebro.guardar(), 10000);
    const guardarYa = () => cerebro.guardar();
    document.addEventListener("visibilitychange", () => { if (document.hidden) guardarYa(); });
    addEventListener("pagehide", guardarYa);
  }

  // ---------- el bucle que nunca para ----------
  // Mientras la página está abierta piensan sin pausa. Si el navegador la duerme
  // (otra pestaña, pantalla apagada), al volver se ponen al día.
  function pensarSinParar() {
    let ultimo = performance.now();
    setInterval(() => {
      const ritmo = cerebro.ritmo();
      const ahora = performance.now();
      let n = Math.floor((ahora - ultimo) / ritmo);
      if (n < 1) return;
      if (n > 200) { n = 200; ultimo = ahora - ritmo; }
      for (let i = 0; i < n; i++) cerebro.ciclo();
      ultimo += n * ritmo;
      actualizar(false);
    }, 50);
  }

  let ultimoRefresco = 0;
  function actualizar(forzar) {
    leerRed();
    leerCorriente();
    const ahora = performance.now();
    if (!forzar && ahora - ultimoRefresco < 450) return;
    ultimoRefresco = ahora;
    estado = cerebro.estado();
    $("cCiclo").textContent = estado.tick.toLocaleString("es");
    $("cPalabras").textContent = estado.palabras.toLocaleString("es");
    $("cAbla").textContent = estado.abla.toLocaleString("es");
    $("cC").textContent = estado.c.toLocaleString("es");
    for (const s of estado.seres) {
      const t = tarjetas[s.id];
      if (ultimos[s.id] !== s.ultimo) {
        ultimos[s.id] = s.ultimo;
        t.p.textContent = s.ultimo;
        t.el.classList.add("activa");
        setTimeout(() => t.el.classList.remove("activa"), 400);
      }
      t.small.textContent = `${TRIBUS[s.tribu]} · ${s.hechos} hechos`;
    }
    if (elegido !== null && !$("p-ser").hidden) mostrarSer(false);
  }

  // ---------- red ----------
  const lienzo = $("lienzo"), ctx = lienzo.getContext("2d");
  let ancho = 0, alto = 0, posiciones = [];

  function ajustar() {
    const r = lienzo.getBoundingClientRect(), dpr = devicePixelRatio || 1;
    ancho = r.width; alto = r.height;
    lienzo.width = ancho * dpr; lienzo.height = alto * dpr;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    // tres tribus repartidas en un círculo, cada una en su arco
    const cx = ancho / 2, cy = alto / 2, R = Math.min(ancho, alto) * 0.38;
    posiciones = [];
    for (let i = 0; i < 27; i++) {
      const tribu = Math.floor(i / 9), k = i % 9;
      const ang = -Math.PI / 2 + tribu * (2 * Math.PI / 3) + (k - 4) * 0.2;
      const r = R * (k % 2 ? 0.82 : 1);
      posiciones.push({ x: cx + Math.cos(ang) * r, y: cy + Math.sin(ang) * r });
    }
  }
  addEventListener("resize", ajustar);
  ajustar();

  function leerRed() {
    const r = cerebro.red(vistaRed);
    vistaRed = r.hasta;
    for (const [de, para, c] of r.m.slice(-60)) {
      if (particulas.length > 220) particulas.shift();
      particulas.push({ de, para, c, t: 0, v: 0.015 + Math.random() * 0.02 });
    }
  }

  function dibujar() {
    ctx.clearRect(0, 0, ancho, alto);
    if (!estado) return requestAnimationFrame(dibujar);
    // lazos tenues dentro de cada tribu
    ctx.lineWidth = 1;
    for (let i = 0; i < 27; i++)
      for (let j = i + 1; j < 27; j++) {
        if (Math.floor(i / 9) !== Math.floor(j / 9)) continue;
        ctx.strokeStyle = COLOR[Math.floor(i / 9)] + "14";
        ctx.beginPath(); ctx.moveTo(posiciones[i].x, posiciones[i].y); ctx.lineTo(posiciones[j].x, posiciones[j].y); ctx.stroke();
      }
    // mensajes viajando
    for (let k = particulas.length - 1; k >= 0; k--) {
      const p = particulas[k];
      p.t += reducirMovimiento ? 1 : p.v;
      if (p.t >= 1) { actividad[p.para] = Math.min(1, actividad[p.para] + 0.25); particulas.splice(k, 1); continue; }
      const a = posiciones[p.de], b = posiciones[p.para];
      const x = a.x + (b.x - a.x) * p.t, y = a.y + (b.y - a.y) * p.t;
      ctx.strokeStyle = (p.c ? COLOR_C : COLOR_ABLA) + "22";
      ctx.beginPath(); ctx.moveTo(a.x, a.y); ctx.lineTo(b.x, b.y); ctx.stroke();
      ctx.fillStyle = p.c ? COLOR_C : COLOR_ABLA;
      if (p.c) ctx.fillRect(x - 2.5, y - 2.5, 5, 5);
      else { ctx.beginPath(); ctx.arc(x, y, 2.6, 0, 7); ctx.fill(); }
    }
    // las 27 mentes
    ctx.textAlign = "center";
    ctx.font = "11px system-ui, sans-serif";
    for (const s of estado.seres) {
      const p = posiciones[s.id], col = COLOR[s.tribu];
      const radio = 6 + Math.sqrt(s.hechos) * 0.9;
      actividad[s.id] *= 0.96;
      const brillo = actividad[s.id];
      if (brillo > 0.05) {
        ctx.fillStyle = col + Math.round(brillo * 90).toString(16).padStart(2, "0");
        ctx.beginPath(); ctx.arc(p.x, p.y, radio * 2.2, 0, 7); ctx.fill();
      }
      ctx.fillStyle = col;
      ctx.beginPath(); ctx.arc(p.x, p.y, radio, 0, 7); ctx.fill();
      if (elegido === s.id) {
        ctx.strokeStyle = "#fff"; ctx.lineWidth = 2;
        ctx.beginPath(); ctx.arc(p.x, p.y, radio + 4, 0, 7); ctx.stroke(); ctx.lineWidth = 1;
      }
      ctx.fillStyle = "#c9cde0";
      ctx.fillText(s.nombre, p.x, p.y + radio + 13);
    }
    requestAnimationFrame(dibujar);
  }

  lienzo.addEventListener("click", (ev) => {
    const r = lienzo.getBoundingClientRect(), x = ev.clientX - r.left, y = ev.clientY - r.top;
    let mejor = -1, d = 30 * 30;
    posiciones.forEach((p, i) => { const dd = (p.x - x) ** 2 + (p.y - y) ** 2; if (dd < d) { d = dd; mejor = i; } });
    if (mejor >= 0) elegir(mejor);
  });

  // ---------- tarjetas ----------
  const tarjetas = [];
  function construirTarjetas() {
    const cont = $("tarjetas");
    for (const s of estado.seres) {
      const el = document.createElement("button");
      el.className = "tarjeta t" + s.tribu;
      el.innerHTML = "<b></b><small></small><p></p>";
      el.querySelector("b").textContent = `${s.id + 1}. ${s.nombre}`;
      el.title = `${s.nombre}: quien ${s.concepto}`;
      el.addEventListener("click", () => elegir(s.id));
      cont.appendChild(el);
      tarjetas[s.id] = { el, small: el.querySelector("small"), p: el.querySelector("p") };
    }
  }

  function elegir(id) {
    elegido = id;
    tarjetas.forEach((t, i) => t.el.classList.toggle("elegida", i === id));
    $("destino").value = String(id + 1);
    abrirPestana("ser");
    mostrarSer(true);
  }

  // ---------- pestañas ----------
  document.querySelectorAll(".pestanas button").forEach((b) => b.addEventListener("click", () => abrirPestana(b.dataset.p)));
  function abrirPestana(p) {
    document.querySelectorAll(".pestanas button").forEach((b) => b.setAttribute("aria-selected", String(b.dataset.p === p)));
    document.querySelectorAll(".hoja").forEach((h) => (h.hidden = h.id !== "p-" + p));
    if (p === "ser" && elegido !== null) mostrarSer(true);
    if (p === "terminal") $("comando").focus({ preventScroll: true });
    if (p === "diccionario" && !$("dicInfo").textContent) mostrarIdioma();
  }

  // ---------- pensamientos ----------
  function leerCorriente() {
    const r = cerebro.corriente(vistaCorriente);
    vistaCorriente = r.hasta;
    const filtro = $("filtro").value;
    const lista = $("corriente");
    for (const l of r.lineas) {
      actividad[l.ser] = 1;
      if (filtro !== "" && String(l.ser) !== filtro) continue;
      lista.prepend(lineaCorriente(l));
    }
    while (lista.children.length > 300) lista.lastChild.remove();
  }
  function lineaCorriente(l) {
    const li = document.createElement("li");
    li.className = "t" + Math.floor(l.ser / 9);
    const m = /^\[t=(\d+)\] (\S+) \([^)]*\): (.*)$/.exec(l.texto);
    if (m) {
      li.innerHTML = "<small></small> <b></b> <span></span>";
      li.querySelector("small").textContent = "ciclo " + m[1];
      li.querySelector("b").textContent = m[2];
      li.querySelector("span").textContent = m[3];
    } else li.textContent = l.texto;
    return li;
  }
  $("filtro").addEventListener("change", () => {
    $("corriente").replaceChildren();
    const r = cerebro.corriente(Math.max(0, vistaCorriente - 200));
    const f = $("filtro").value;
    for (const l of r.lineas) if (f === "" || String(l.ser) === f) $("corriente").prepend(lineaCorriente(l));
  });

  // ---------- ser ----------
  function mostrarSer(completo) {
    if (elegido === null) return;
    const s = estado.seres[elegido];
    $("serVacio").hidden = true;
    $("serDatos").hidden = false;
    $("serNombre").textContent = `${s.id + 1}. ${s.nombre}`;
    $("serNombre").style.color = COLOR[s.tribu];
    $("serSub").textContent = `«quien ${s.concepto}» · tribu ${TRIBUS[s.tribu]}`;
    const cifras = [
      ["hechos que sabe", s.hechos], ["pensamientos", s.pensamientos],
      ["contexto", `${s.contexto.toLocaleString("es")} / ${s.capacidad.toLocaleString("es")} tokens`],
      ["largo plazo", `${s.largo} recuerdos`], ["palabras nuevas", s.nuevas],
      ["mensajes", `${s.enviados} ↑ · ${s.recibidos} ↓`],
    ];
    $("serCifras").replaceChildren(...cifras.map(([k, v]) => {
      const d = document.createElement("div");
      d.innerHTML = "<dt></dt><dd></dd>";
      d.firstChild.textContent = k; d.lastChild.textContent = v;
      return d;
    }));
    $("serUltimo").textContent = s.ultimo;
    $("serMente").replaceChildren(...cerebro.mente(s.id, 12).reverse().map((r) => {
      const li = document.createElement("li");
      li.textContent = `[${r.t}] ${r.x}`;
      return li;
    }));
    if (completo) {
      const c = cerebro.fs.leerTexto(`mundo/conocimiento/${s.nombre}.c`) || "(todavía no se ha guardado)";
      const lineas = c.split("\n");
      $("serC").textContent = lineas.slice(0, 80).join("\n") + (lineas.length > 80 ? `\n… (${lineas.length - 80} líneas más)` : "");
      $("serTerminal").textContent = cerebro.ejecutar("terminal " + (s.id + 1)).split("\n").slice(-12).join("\n");
    }
  }
  $("bajarC").addEventListener("click", () => {
    if (elegido === null) return;
    const s = estado.seres[elegido];
    cerebro.ejecutar("guardar");
    const texto = cerebro.fs.leerTexto(`mundo/conocimiento/${s.nombre}.c`) || "";
    const a = document.createElement("a");
    a.href = URL.createObjectURL(new Blob([texto], { type: "text/x-c" }));
    a.download = s.nombre + ".c";
    a.click();
    setTimeout(() => URL.revokeObjectURL(a.href), 1000);
  });

  // ---------- hablar ----------
  function construirSelects() {
    const filtro = $("filtro"), destino = $("destino");
    destino.append(new Option("toda la especie", "todos"));
    for (const s of estado.seres) {
      filtro.append(new Option(`${s.id + 1}. ${s.nombre} (${TRIBUS[s.tribu]})`, String(s.id)));
      destino.append(new Option(`${s.id + 1}. ${s.nombre} (${TRIBUS[s.tribu]})`, String(s.id + 1)));
    }
    destino.value = "19";
  }
  function construirEjemplos() {
    const ejemplos = [
      ["decir", "sol causa luz pregunta"], ["decir", "agua parte mar pregunta"], ["enseñar", "fuego causa caliente"],
      ["enseñar", "luna parte cielo"], ["enseñar", "río es agua"], ["decir", "hola mente"],
    ];
    for (const [modo, texto] of ejemplos) {
      const b = document.createElement("button");
      b.type = "button";
      b.textContent = (modo === "decir" ? "💬 " : "◆ ") + texto;
      b.addEventListener("click", () => hablar(modo, texto));
      $("ejemplos").append(b);
    }
  }
  function burbuja(texto, quien) {
    const d = document.createElement("div");
    d.className = "burbuja " + quien;
    d.textContent = texto;
    $("charla").append(d);
    d.scrollIntoView({ block: "end", behavior: reducirMovimiento ? "auto" : "smooth" });
  }
  function hablar(modo, texto) {
    texto = texto.trim();
    if (!texto) return;
    const a = $("destino").value;
    burbuja(`${modo === "decir" ? "" : "[data C] "}${texto}  →  ${a === "todos" ? "toda la especie" : $("destino").selectedOptions[0].text}`, "yo");
    burbuja(cerebro.ejecutar(`${modo} ${a} ${texto}`).trim(), "ellos");
    actualizar(true);
    cerebro.guardar();
  }
  $("formHablar").addEventListener("submit", (ev) => {
    ev.preventDefault();
    const modo = ev.submitter && ev.submitter.dataset.modo ? ev.submitter.dataset.modo : "decir";
    hablar(modo, $("mensaje").value);
    $("mensaje").value = "";
  });

  // ---------- diccionario ----------
  function mostrarIdioma() {
    const r = cerebro.ejecutar("idioma").trim().split("\n");
    $("dicInfo").textContent = r[0];
    const nuevas = r.filter((l) => l.includes("nueva:")).map((l) => l.replace(/^\s*nueva:\s*/, "").split(" = "));
    pintarDic(nuevas.map(([f, g]) => [f, g + "  (creada por la especie)"]));
  }
  function pintarDic(filas) {
    $("dicResultados").replaceChildren(...filas.map(([a, b]) => {
      const tr = document.createElement("tr");
      tr.innerHTML = "<td></td><td></td>";
      tr.firstChild.textContent = a; tr.lastChild.textContent = b;
      return tr;
    }));
  }
  $("formDic").addEventListener("submit", (ev) => {
    ev.preventDefault();
    const q = $("busqueda").value.trim();
    if (!q) return mostrarIdioma();
    const exacto = cerebro.ejecutar("dic " + q.split(/\s+/)[0]).trim();
    const filas = [];
    if (!exacto.startsWith("dic:")) {
      const [cabeza, resto] = exacto.split("   (aspectos: ");
      const [a, b] = cabeza.split(" = ");
      filas.push([a, b]);
      if (resto) for (const par of resto.replace(/\)$/, "").split(", ")) { const [f, g] = par.split("="); filas.push([f, `${b} · ${g}`]); }
    }
    for (const l of cerebro.ejecutar("dic buscar " + q).split("\n")) {
      const [f, g] = l.trim().split("\t");
      if (g && !filas.some((x) => x[0] === f)) filas.push([f, g]);
    }
    pintarDic(filas.length ? filas : [["—", "nada encontrado"]]);
  });

  // ---------- terminal ----------
  const historial = [];
  let posHistorial = 0;
  function escribirTerminal(t) {
    const s = $("salida");
    s.textContent += t;
    if (s.textContent.length > 60000) s.textContent = s.textContent.slice(-50000);
    s.scrollTop = s.scrollHeight;
  }
  $("formTerminal").addEventListener("submit", (ev) => {
    ev.preventDefault();
    const linea = $("comando").value;
    $("comando").value = "";
    if (linea.trim()) { historial.push(linea); posHistorial = historial.length; }
    if (linea.trim() === "clear") { $("salida").textContent = ""; return; }
    escribirTerminal(cerebro.prompt() + linea + "\n" + cerebro.ejecutar(linea));
    $("prompt").textContent = cerebro.prompt().trim();
    actualizar(true);
    cerebro.guardar();
  });
  $("comando").addEventListener("keydown", (ev) => {
    if (ev.key === "ArrowUp" && posHistorial > 0) { $("comando").value = historial[--posHistorial]; ev.preventDefault(); }
    if (ev.key === "ArrowDown") { posHistorial = Math.min(historial.length, posHistorial + 1); $("comando").value = historial[posHistorial] || ""; ev.preventDefault(); }
  });

  // ---------- velocidad y reinicio ----------
  function textoRitmo(ms) { return `1 ciclo / ${(ms / 1000).toLocaleString("es", { maximumFractionDigits: 1 })} s`; }
  $("ritmo").addEventListener("input", () => {
    const ms = Number($("ritmo").value);
    if (cerebro) cerebro.fijarRitmo(ms);
    $("ritmoTexto").textContent = textoRitmo(ms);
  });
  $("reiniciar").addEventListener("click", async () => {
    if (!confirm("Esto borra la memoria, el conocimiento y las palabras creadas de los 27. ¿Empezar de cero?")) return;
    await cerebro.fs.borrarTodo();
    location.reload();
  });

  iniciar();
})();
