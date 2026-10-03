// La app de la Especie Abla. Todo lo que ves lo dibuja el cerebro en C (gui.c,
// compilado a WebAssembly) en una imagen; esta página solo:
//   - copia esa imagen a la pantalla,
//   - le pasa los toques y las teclas,
//   - y hace lo que el C no puede hacer solo: abrir el selector de fotos y la cámara.
(function () {
  "use strict";

  const lienzo = document.getElementById("app");
  const ctx = lienzo.getContext("2d");
  const teclado = document.getElementById("teclado");
  const selector = document.getElementById("fotos");
  const video = document.getElementById("camara");
  const ESCALA = 2; // la interfaz está diseñada para pantallas retina, como la del iPad
  let cerebro = null, ancho = 0, alto = 0;

  // ---------- tamaño ----------
  function ajustar() {
    const r = lienzo.getBoundingClientRect();
    ancho = Math.max(320, Math.round(r.width * ESCALA));
    alto = Math.max(320, Math.round(r.height * ESCALA));
    lienzo.width = ancho;
    lienzo.height = alto;
  }

  // ---------- dibujar: el C pinta, aquí se copia ----------
  function dibujar() {
    const ptr = cerebro.guiCuadro(ancho, alto, ESCALA, performance.now());
    const px = new Uint8ClampedArray(cerebro.x.memory.buffer, ptr, ancho * alto * 4);
    ctx.putImageData(new ImageData(px, ancho, alto), 0, 0);
  }

  // ---------- lo que pide la interfaz ----------
  function atenderPeticiones() {
    const p = cerebro.guiPeticiones();
    if (p & 1) teclado.focus({ preventScroll: true });
    if (p & 2) selector.click();
    if (p & 4) alternarCamara();
  }

  // ---------- toques ----------
  function punto(e) {
    const r = lienzo.getBoundingClientRect();
    return [(e.clientX - r.left) * (ancho / r.width), (e.clientY - r.top) * (alto / r.height)];
  }
  lienzo.addEventListener("pointerdown", (e) => {
    lienzo.setPointerCapture(e.pointerId);
    cerebro && cerebro.guiPuntero(0, ...punto(e));
  });
  lienzo.addEventListener("pointermove", (e) => cerebro && cerebro.guiPuntero(1, ...punto(e)));
  lienzo.addEventListener("pointerup", (e) => {
    if (!cerebro) return;
    cerebro.guiPuntero(2, ...punto(e));
    dibujar(); // el toque se procesa ya, para que abrir fotos o el teclado ocurra dentro del gesto
    atenderPeticiones();
  });
  lienzo.addEventListener("pointercancel", (e) => cerebro && cerebro.guiPuntero(2, -1, -1));
  lienzo.addEventListener("wheel", (e) => {
    e.preventDefault();
    cerebro && cerebro.guiRueda(e.deltaY * ESCALA);
  }, { passive: false });

  // ---------- teclas (también el teclado en pantalla del iPad) ----------
  const ESPECIALES = { Enter: "\r", Backspace: "\x7f", Escape: "\x1b", ArrowUp: "\x1b[A", ArrowDown: "\x1b[B" };
  function tecla(e) {
    if (!cerebro || e.isComposing) return;
    const s = ESPECIALES[e.key];
    if (s) {
      e.preventDefault();
      cerebro.guiTeclas(s);
      if (s === "\r") cerebro.guardar();
    }
  }
  teclado.addEventListener("keydown", tecla);
  teclado.addEventListener("input", () => {
    const t = teclado.value.replace(/\n/g, "\r");
    teclado.value = "";
    if (cerebro && t) cerebro.guiTeclas(t);
  });
  // con teclado físico, escribir sin tocar antes un campo también funciona
  document.addEventListener("keydown", (e) => {
    if (e.target === teclado || !cerebro) return;
    if (e.key.length === 1 && !e.metaKey && !e.ctrlKey) {
      cerebro.guiTeclas(e.key);
      e.preventDefault();
    } else tecla(e);
  });

  // ---------- fotos: el navegador las abre, el C las mira ----------
  // Cada foto se reduce a 320×240 como mucho y se guarda como PPM (los píxeles RGB tal cual)
  // en mundo/fotos/entrada/. Desde ahí, la especie las va mirando una a una.
  const MAX_W = 320, MAX_H = 240;
  const pintor = document.createElement("canvas");
  const pctx = pintor.getContext("2d", { willReadFrequently: true });

  function aPPM(fuente, w0, h0) {
    const k = Math.min(1, MAX_W / w0, MAX_H / h0);
    const w = Math.max(1, Math.round(w0 * k)), h = Math.max(1, Math.round(h0 * k));
    pintor.width = w;
    pintor.height = h;
    pctx.imageSmoothingQuality = "high";
    pctx.drawImage(fuente, 0, 0, w, h);
    const rgba = pctx.getImageData(0, 0, w, h).data;
    const cab = new TextEncoder().encode(`P6\n${w} ${h}\n255\n`);
    const ppm = new Uint8Array(cab.length + w * h * 3);
    ppm.set(cab);
    for (let i = 0, j = cab.length; i < w * h; i++) {
      ppm[j++] = rgba[i * 4];
      ppm[j++] = rgba[i * 4 + 1];
      ppm[j++] = rgba[i * 4 + 2];
    }
    return ppm;
  }

  function cargarImagen(archivo) {
    if (window.createImageBitmap) return createImageBitmap(archivo);
    return new Promise((ok, mal) => {
      const img = new Image();
      img.onload = () => ok(img);
      img.onerror = mal;
      img.src = URL.createObjectURL(archivo);
    });
  }

  let numeroFoto = 0;
  selector.addEventListener("change", async () => {
    const archivos = [...selector.files];
    selector.value = "";
    if (!archivos.length) return;
    cerebro.guiAviso(`Preparando ${archivos.length} foto${archivos.length > 1 ? "s" : ""}…`);
    let bien = 0;
    const lote = Date.now();
    for (const a of archivos) {
      try {
        const img = await cargarImagen(a);
        const ppm = aPPM(img, img.width, img.height);
        const nombre = (a.name || "foto").replace(/\.[^.]+$/, "").replace(/[^\w\-áéíóúñÁÉÍÓÚÑ]+/g, "_").slice(0, 40);
        cerebro.escribirArchivo(`mundo/fotos/entrada/${lote}-${String(++numeroFoto).padStart(4, "0")}-${nombre}.ppm`, ppm);
        bien++;
      } catch (e) {
        console.warn("no pude abrir", a.name, e);
      }
    }
    cerebro.fs.guardar();
    cerebro.guiAviso(bien === archivos.length
      ? `${bien} foto${bien > 1 ? "s" : ""} en la cola. Las mirarán una a una.`
      : `${bien} de ${archivos.length} fotos en la cola (alguna no se pudo abrir).`);
  });

  // ---------- cámara: una foto cada pocos segundos mientras esté encendida ----------
  let flujo = null, tomas = null;
  async function alternarCamara() {
    if (flujo) {
      flujo.getTracks().forEach((t) => t.stop());
      flujo = null;
      clearInterval(tomas);
      cerebro.guiCamara(false);
      cerebro.guiAviso("Cámara apagada.");
      return;
    }
    try {
      if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) throw new Error("sin cámara");
      flujo = await navigator.mediaDevices.getUserMedia({ video: { facingMode: "environment", width: { ideal: 640 } } });
      video.srcObject = flujo;
      await video.play();
      cerebro.guiCamara(true);
      cerebro.guiAviso("Cámara encendida: miran una imagen cada 4 segundos.");
      tomas = setInterval(() => {
        if (!video.videoWidth) return;
        const cola = (cerebro.fs.hijos("mundo/fotos/entrada") || []).length;
        if (cola > 3) return; // no amontonar: que terminen de mirar
        const ppm = aPPM(video, video.videoWidth, video.videoHeight);
        cerebro.escribirArchivo(`mundo/fotos/entrada/${Date.now()}-camara.ppm`, ppm);
      }, 4000);
    } catch (e) {
      flujo = null;
      cerebro.guiCamara(false);
      cerebro.guiAviso("La cámara no está disponible aquí. Usa «Añadir fotos».");
    }
  }

  // ---------- arranque y bucle que nunca para ----------
  async function iniciar() {
    try {
      cerebro = await Cerebro.despertar({});
    } catch (e) {
      document.getElementById("mensaje").textContent = "No pude despertar a la especie: " + e.message;
      return;
    }
    document.getElementById("mensaje").remove();
    let ritmo = 300;
    try { ritmo = Number(localStorage.getItem("abla-ritmo")) || 300; } catch (_) {}
    cerebro.fijarRitmo(ritmo);
    ajustar();
    addEventListener("resize", ajustar);

    // pensar: todos los ciclos que tocan según el ritmo (y ponerse al día al volver)
    let ultimo = performance.now();
    setInterval(() => {
      const r = cerebro.ritmo(), ahora = performance.now();
      let n = Math.floor((ahora - ultimo) / r);
      if (n > 0) {
        if (n > 200) { n = 200; ultimo = ahora - r; }
        for (let i = 0; i < n; i++) cerebro.ciclo();
        ultimo += n * r;
      }
    }, 25);

    // dibujar unas 15 veces por segundo
    let ultimoCuadro = 0;
    function cuadro(t) {
      if (!document.hidden && t - ultimoCuadro >= 66) {
        dibujar();
        atenderPeticiones();
        ultimoCuadro = t;
      }
      requestAnimationFrame(cuadro);
    }
    requestAnimationFrame(cuadro);

    setInterval(() => {
      cerebro.guardar();
      try { localStorage.setItem("abla-ritmo", String(cerebro.ritmo())); } catch (_) {}
    }, 10000);
    document.addEventListener("visibilitychange", () => { if (document.hidden) cerebro.guardar(); });
    addEventListener("pagehide", () => cerebro.guardar());
  }

  iniciar();
})();
