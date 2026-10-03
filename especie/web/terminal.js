// La interfaz de terminal de la Especie Abla, dentro de una página.
// Todo lo que se ve lo dibuja el cerebro en C (tui.c, compilado a WebAssembly):
// esta página solo le pasa las teclas, le dice el tamaño y muestra cada cuadro.
(function () {
  "use strict";

  const pantalla = document.getElementById("pantalla");
  const teclado = document.getElementById("teclado");
  let cerebro = null, columnas = 100, filas = 30;
  const RITMOS = [50, 100, 150, 200, 300, 500, 800, 1200, 2000];

  // ---------- tamaño: cuántos caracteres caben ----------
  function medir() {
    const ancho = pantalla.clientWidth - 16, alto = pantalla.clientHeight - 12;
    // letra lo más grande posible que deje al menos 100 columnas (para ver la red), entre 10 y 15 px
    let fs = Math.max(10, Math.min(15, Math.floor(ancho / 100 / 0.6)));
    pantalla.style.fontSize = fs + "px";
    const prueba = document.createElement("span");
    prueba.textContent = "M".repeat(50);
    pantalla.textContent = "";
    pantalla.append(prueba);
    const r = prueba.getBoundingClientRect();
    prueba.remove();
    const anchoLetra = r.width / 50, altoLinea = r.height;
    columnas = Math.max(40, Math.floor(ancho / anchoLetra));
    filas = Math.max(14, Math.floor(alto / altoLinea));
  }

  // ---------- dibujar ----------
  function dibujar() {
    if (!cerebro) return;
    pantalla.innerHTML = cerebro.tuiHtml(columnas, filas);
  }

  // ---------- teclas: se traducen a lo que mandaría una terminal ----------
  const ESPECIALES = {
    Enter: "\r", Backspace: "\x7f", Tab: "\t", Escape: "\x1b",
    ArrowUp: "\x1b[A", ArrowDown: "\x1b[B", ArrowRight: "\x1b[C", ArrowLeft: "\x1b[D",
    PageUp: "\x1b[5~", PageDown: "\x1b[6~",
  };
  function enviar(s) {
    if (!cerebro || !s) return;
    cerebro.tuiTeclas(s);
    if (s.includes("\r")) cerebro.guardar();
    dibujar();
  }
  teclado.addEventListener("keydown", (e) => {
    if (e.isComposing) return;
    let s = ESPECIALES[e.key];
    if (e.ctrlKey && e.key.length === 1) {
      const k = e.key.toLowerCase();
      if (k === "l") s = "\x0c";
      else if (k === "c") s = "\x1b"; // en el navegador, Ctrl+C borra la línea
    }
    if (s) {
      e.preventDefault();
      enviar(s);
    }
  });
  // el texto normal (también acentos, ñ y el teclado en pantalla) llega por «input»
  teclado.addEventListener("input", () => {
    const t = teclado.value.replace(/\n/g, "\r");
    teclado.value = "";
    enviar(t);
  });

  const enfocar = () => teclado.focus({ preventScroll: true });
  pantalla.addEventListener("pointerup", enfocar);
  document.getElementById("abrirTeclado").addEventListener("click", enfocar);
  document.querySelectorAll(".barra button[data-t]").forEach((b) =>
    b.addEventListener("click", () => {
      enviar(b.dataset.t);
      enfocar();
    }));

  // ---------- velocidad de pensamiento ----------
  function mostrarRitmo() {
    const ms = cerebro.ritmo();
    document.getElementById("ritmo").textContent = (ms / 1000).toLocaleString("es", { maximumFractionDigits: 2 }) + " s";
  }
  function cambiarRitmo(paso) {
    const i = RITMOS.findIndex((r) => r >= cerebro.ritmo());
    const j = Math.max(0, Math.min(RITMOS.length - 1, (i < 0 ? RITMOS.length - 1 : i) + paso));
    cerebro.fijarRitmo(RITMOS[j]);
    try { localStorage.setItem("abla-ritmo", String(RITMOS[j])); } catch (_) {}
    mostrarRitmo();
    dibujar();
  }
  document.getElementById("rapido").addEventListener("click", () => cambiarRitmo(-1));
  document.getElementById("lento").addEventListener("click", () => cambiarRitmo(+1));

  // ---------- arranque y bucle que nunca para ----------
  async function iniciar() {
    try {
      cerebro = await Cerebro.despertar({});
    } catch (e) {
      pantalla.textContent = "No pude despertar a la especie: " + e.message;
      return;
    }
    let ritmo = 300;
    try { ritmo = Number(localStorage.getItem("abla-ritmo")) || 300; } catch (_) {}
    cerebro.fijarRitmo(ritmo);
    mostrarRitmo();
    medir();
    dibujar();
    enfocar();

    let ultimo = performance.now(), ultimoCuadro = 0;
    setInterval(() => {
      const r = cerebro.ritmo(), ahora = performance.now();
      let n = Math.floor((ahora - ultimo) / r);
      if (n > 0) {
        if (n > 200) { n = 200; ultimo = ahora - r; } // al volver de segundo plano se ponen al día
        for (let i = 0; i < n; i++) cerebro.ciclo();
        ultimo += n * r;
      }
      if (ahora - ultimoCuadro >= 100) { // unos 10 cuadros por segundo (los mensajes se mueven)
        dibujar();
        ultimoCuadro = ahora;
      }
    }, 30);

    setInterval(() => cerebro.guardar(), 10000);
    document.addEventListener("visibilitychange", () => { if (document.hidden) cerebro.guardar(); });
    addEventListener("pagehide", () => cerebro.guardar());
    addEventListener("resize", () => { medir(); dibujar(); });
  }

  iniciar();
})();
