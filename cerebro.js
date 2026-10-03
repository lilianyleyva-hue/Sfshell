// Nuevo Cerebro IA
// Un cerebro que no percibe como nosotros:
//  - No ve objetos: ve movimiento, luz y color, como un campo de energía.
//  - No escucha palabras primero: escucha ritmo, tono e intensidad;
//    las palabras (si el navegador puede reconocerlas) llegan después.
//  - Mezcla los sentidos: lo que ocurre a la vez en la vista y el oído
//    queda asociado (sinestesia), y eso es lo que "piensa".

(() => {
  "use strict";

  // ---------- Parámetros ----------
  const GRID_W = 16, GRID_H = 12;   // resolución con la que "ve"
  const BANDAS = 24;                // bandas de frecuencia con las que "oye"
  const N_NEURONAS = 200;
  const VECINOS = 4;
  const DECAIMIENTO = 0.94;
  const UMBRAL_DISPARO = 0.55;

  const COLORES = { vista: [79, 209, 255], oido: [255, 106, 213], mente: [184, 255, 106] };

  // ---------- DOM ----------
  const $ = (id) => document.getElementById(id);
  const red = $("red"), ctx = red.getContext("2d");
  const ojo = $("ojo"), ojoCtx = ojo.getContext("2d");
  const oreja = $("oreja"), orejaCtx = oreja.getContext("2d");
  const listaPensamientos = $("pensamientos");
  const medidores = { luz: $("mLuz"), mov: $("mMov"), vol: $("mVol"), tono: $("mTono") };

  // ---------- Estado sensorial ----------
  const percepcion = {
    movimiento: new Float32Array(GRID_W * GRID_H),
    tonoCelda: new Float32Array(GRID_W * GRID_H), // tono (hue) de cada celda, 0..1
    bandas: new Float32Array(BANDAS),
    luz: 0, mov: 0, centroMovX: 0.5, colorDominante: 0,
    vol: 0, tono: 0, golpes: [],
    palabras: null,
  };

  const sentidos = { ver: false, escuchar: false, sonar: false };

  // ---------- Red neuronal ----------
  let neuronas = [], senales = [], ancho = 0, alto = 0;

  function dentroDelCerebro(x, y) {
    // Silueta: dos hemisferios (elipses) vistos desde arriba, ligeramente separados.
    const e = (cx, cy, rx, ry) => ((x - cx) / rx) ** 2 + ((y - cy) / ry) ** 2 <= 1;
    return e(0.37, 0.5, 0.25, 0.36) || e(0.63, 0.5, 0.25, 0.36);
  }

  function regionDe(x, y) {
    if (y > 0.6 && (x < 0.3 || x > 0.7)) return "oido";  // lóbulos temporales
    if (y < 0.38) return "vista";                          // occipital (arriba en esta vista)
    return "mente";                                        // asociación en el centro
  }

  function crearRed() {
    neuronas = [];
    let intentos = 0;
    while (neuronas.length < N_NEURONAS && intentos++ < 20000) {
      const x = Math.random(), y = Math.random();
      if (!dentroDelCerebro(x, y)) continue;
      const region = regionDe(x, y);
      neuronas.push({
        x, y, region,
        act: 0, refractario: 0, enlaces: [],
        // Cada neurona sensorial atiende a una parte del mundo.
        campo: region === "vista"
          ? Math.floor(x * GRID_W) + GRID_W * Math.floor(Math.random() * GRID_H)
          : Math.floor(Math.random() * BANDAS),
      });
    }
    // Las de oído se ordenan por x para que el tono forme un mapa tonotópico.
    const oido = neuronas.filter((n) => n.region === "oido").sort((a, b) => a.x - b.x);
    oido.forEach((n, i) => { n.campo = Math.floor((i / oido.length) * BANDAS); });

    for (const n of neuronas) {
      n.enlaces = neuronas
        .filter((m) => m !== n)
        .map((m) => ({ m, d: Math.hypot(m.x - n.x, m.y - n.y) }))
        .sort((a, b) => a.d - b.d)
        .slice(0, VECINOS)
        .map((e) => ({ destino: e.m, peso: 0.25 + Math.random() * 0.2 }));
    }
    // Puentes de larga distancia: la mente une vista y oído.
    const mente = neuronas.filter((n) => n.region === "mente");
    for (const n of neuronas) {
      if (n.region !== "mente" && mente.length && Math.random() < 0.35) {
        n.enlaces.push({ destino: mente[Math.floor(Math.random() * mente.length)], peso: 0.3 });
      }
    }
  }

  function ajustarLienzo() {
    const r = red.getBoundingClientRect(), dpr = window.devicePixelRatio || 1;
    ancho = r.width; alto = r.height;
    red.width = ancho * dpr; red.height = alto * dpr;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  }

  function estimular() {
    for (const n of neuronas) {
      if (n.region === "vista") {
        const c = n.campo;
        n.act += percepcion.movimiento[c] * 0.9 + percepcion.luz * 0.01;
      } else if (n.region === "oido") {
        n.act += percepcion.bandas[n.campo] * 0.6;
      }
    }
  }

  function propagar() {
    for (const n of neuronas) {
      if (n.refractario > 0) { n.refractario--; continue; }
      if (n.act > UMBRAL_DISPARO) {
        for (const e of n.enlaces) {
          senales.push({ desde: n, hacia: e.destino, t: 0, fuerza: n.act * e.peso, enlace: e });
        }
        n.refractario = 6;
        n.act *= 0.3;
      }
    }
    for (let i = senales.length - 1; i >= 0; i--) {
      const s = senales[i];
      s.t += 0.08;
      if (s.t >= 1) {
        s.hacia.act += s.fuerza;
        // Hebb: lo que dispara junto, se conecta más fuerte.
        if (s.hacia.act > UMBRAL_DISPARO) s.enlace.peso = Math.min(0.9, s.enlace.peso + 0.01);
        senales.splice(i, 1);
      }
    }
    if (senales.length > 1500) senales.splice(0, senales.length - 1500);
    for (const n of neuronas) {
      n.act = Math.min(1.5, n.act * DECAIMIENTO);
      for (const e of n.enlaces) e.peso = Math.max(0.15, e.peso * 0.9999);
    }
  }

  function dibujarRed() {
    ctx.clearRect(0, 0, ancho, alto);
    const lado = Math.min(ancho, alto * 1.3);
    const ox = (ancho - lado) / 2, oy = (alto - lado / 1.3) / 2, sy = lado / 1.3;
    const px = (n) => ox + n.x * lado, py = (n) => oy + n.y * sy;

    ctx.lineWidth = 1;
    for (const n of neuronas) {
      for (const e of n.enlaces) {
        const [r, g, b] = COLORES[n.region];
        ctx.strokeStyle = `rgba(${r},${g},${b},${0.03 + e.peso * 0.12})`;
        ctx.beginPath(); ctx.moveTo(px(n), py(n)); ctx.lineTo(px(e.destino), py(e.destino)); ctx.stroke();
      }
    }
    for (const s of senales) {
      const x = px(s.desde) + (px(s.hacia) - px(s.desde)) * s.t;
      const y = py(s.desde) + (py(s.hacia) - py(s.desde)) * s.t;
      const [r, g, b] = COLORES[s.desde.region];
      ctx.fillStyle = `rgba(${r},${g},${b},${Math.min(1, s.fuerza * 2)})`;
      ctx.beginPath(); ctx.arc(x, y, 1.8, 0, Math.PI * 2); ctx.fill();
    }
    for (const n of neuronas) {
      const [r, g, b] = COLORES[n.region];
      const a = Math.min(1, n.act);
      const radio = 2 + a * 6;
      if (a > 0.15) {
        ctx.fillStyle = `rgba(${r},${g},${b},${a * 0.25})`;
        ctx.beginPath(); ctx.arc(px(n), py(n), radio * 2.5, 0, Math.PI * 2); ctx.fill();
      }
      ctx.fillStyle = `rgba(${r},${g},${b},${0.3 + a * 0.7})`;
      ctx.beginPath(); ctx.arc(px(n), py(n), radio, 0, Math.PI * 2); ctx.fill();
    }
  }

  // ---------- VER: cámara → campo de movimiento y color ----------
  let video = null, flujoCamara = null, cuadroPrevio = null;
  const muestra = document.createElement("canvas");
  muestra.width = GRID_W; muestra.height = GRID_H;
  const muestraCtx = muestra.getContext("2d", { willReadFrequently: true });

  function exigirMedios() {
    // En iPadOS la cámara y el micrófono solo existen en HTTPS (o localhost).
    if (!navigator.mediaDevices?.getUserMedia) {
      throw new Error(window.isSecureContext ? "este navegador no da acceso a los sentidos" : "abre la página por HTTPS");
    }
  }

  async function activarVista() {
    exigirMedios();
    flujoCamara = await navigator.mediaDevices.getUserMedia({
      video: { facingMode: "user", width: { ideal: 320 }, height: { ideal: 240 } },
    });
    video = document.createElement("video");
    // Safari de iPad exige playsinline + muted para reproducir sin pantalla completa.
    video.setAttribute("playsinline", "");
    video.setAttribute("muted", "");
    video.muted = true; video.playsInline = true;
    video.srcObject = flujoCamara;
    await video.play();
  }

  function desactivarVista() {
    flujoCamara?.getTracks().forEach((t) => t.stop());
    flujoCamara = null; video = null; cuadroPrevio = null;
    percepcion.movimiento.fill(0);
  }

  function hueDe(r, g, b) {
    const max = Math.max(r, g, b), min = Math.min(r, g, b), d = max - min;
    if (d < 1e-6) return 0;
    let h = max === r ? ((g - b) / d) % 6 : max === g ? (b - r) / d + 2 : (r - g) / d + 4;
    return ((h * 60 + 360) % 360) / 360;
  }

  function percibirVista() {
    if (!video || video.readyState < 2) return;
    muestraCtx.drawImage(video, 0, 0, GRID_W, GRID_H);
    const px = muestraCtx.getImageData(0, 0, GRID_W, GRID_H).data;
    const lum = new Float32Array(GRID_W * GRID_H);
    let luz = 0, mov = 0, sumaX = 0, hx = 0, hy = 0;
    for (let i = 0; i < lum.length; i++) {
      const r = px[i * 4] / 255, g = px[i * 4 + 1] / 255, b = px[i * 4 + 2] / 255;
      lum[i] = 0.299 * r + 0.587 * g + 0.114 * b;
      luz += lum[i];
      const h = hueDe(r, g, b), sat = Math.max(r, g, b) - Math.min(r, g, b);
      percepcion.tonoCelda[i] = h;
      hx += Math.cos(h * 2 * Math.PI) * sat; hy += Math.sin(h * 2 * Math.PI) * sat;
    }
    if (cuadroPrevio) {
      for (let i = 0; i < lum.length; i++) {
        const m = Math.min(1, Math.abs(lum[i] - cuadroPrevio[i]) * 6);
        percepcion.movimiento[i] = percepcion.movimiento[i] * 0.5 + m * 0.5;
        mov += m; sumaX += m * ((i % GRID_W) / GRID_W);
      }
    }
    cuadroPrevio = lum;
    percepcion.luz = luz / lum.length;
    percepcion.mov = Math.min(1, (mov / lum.length) * 4);
    if (mov > 0.5) percepcion.centroMovX = percepcion.centroMovX * 0.7 + (sumaX / mov) * 0.3;
    percepcion.colorDominante = ((Math.atan2(hy, hx) / (2 * Math.PI)) + 1) % 1;
  }

  function dibujarOjo() {
    // No muestra la imagen: muestra lo que el cerebro siente de ella.
    const cw = ojo.width / GRID_W, ch = ojo.height / GRID_H;
    ojoCtx.fillStyle = "rgba(0,0,0,0.35)";
    ojoCtx.fillRect(0, 0, ojo.width, ojo.height);
    for (let i = 0; i < GRID_W * GRID_H; i++) {
      const m = percepcion.movimiento[i];
      const h = Math.round(percepcion.tonoCelda[i] * 360);
      const l = 8 + percepcion.luz * 20 + m * 50;
      ojoCtx.fillStyle = `hsl(${h} 90% ${l}%)`;
      ojoCtx.fillRect((i % GRID_W) * cw, Math.floor(i / GRID_W) * ch, cw - 1, ch - 1);
    }
  }

  // ---------- ESCUCHAR: micrófono → espectro, ritmo y tono ----------
  let audioCtx = null, analizador = null, flujoMic = null, datosFrecuencia = null;
  let reconocimiento = null, energiaPrevia = 0;

  async function activarOido() {
    exigirMedios();
    // WebKit solo deja sonar un AudioContext creado dentro del toque del usuario,
    // así que se crea antes de esperar el permiso del micrófono.
    audioCtx = new (window.AudioContext || window.webkitAudioContext)();
    const reanudar = audioCtx.resume();
    try {
      flujoMic = await navigator.mediaDevices.getUserMedia({ audio: { echoCancellation: false, noiseSuppression: false } });
    } catch (err) {
      audioCtx.close(); audioCtx = null;
      throw err;
    }
    await reanudar;
    const fuente = audioCtx.createMediaStreamSource(flujoMic);
    analizador = audioCtx.createAnalyser();
    analizador.fftSize = 1024;
    analizador.smoothingTimeConstant = 0.6;
    fuente.connect(analizador);
    datosFrecuencia = new Uint8Array(analizador.frequencyBinCount);
    iniciarReconocimiento();
  }

  function desactivarOido() {
    flujoMic?.getTracks().forEach((t) => t.stop());
    audioCtx?.close();
    flujoMic = null; audioCtx = null; analizador = null;
    if (reconocimiento) { reconocimiento.onend = null; reconocimiento.stop(); reconocimiento = null; }
    percepcion.bandas.fill(0);
  }

  function iniciarReconocimiento() {
    const SR = window.SpeechRecognition || window.webkitSpeechRecognition;
    if (!SR) return;
    reconocimiento = new SR();
    reconocimiento.lang = "es-ES";
    reconocimiento.continuous = true;
    reconocimiento.interimResults = false;
    reconocimiento.onresult = (ev) => {
      const r = ev.results[ev.results.length - 1];
      if (r.isFinal) percepcion.palabras = r[0].transcript.trim();
    };
    reconocimiento.onerror = () => {}; // en iPad puede fallar si el dictado está desactivado; el oído sigue funcionando
    reconocimiento.onend = () => {
      if (sentidos.escuchar) setTimeout(() => { try { reconocimiento?.start(); } catch (_) {} }, 300);
    };
    try { reconocimiento.start(); } catch (_) {}
  }

  function percibirOido() {
    if (!analizador) return;
    analizador.getByteFrequencyData(datosFrecuencia);
    // Bandas logarítmicas: más resolución en graves, como el oído.
    const n = datosFrecuencia.length;
    let total = 0, ponderado = 0;
    for (let b = 0; b < BANDAS; b++) {
      const ini = Math.floor(Math.pow(n, b / BANDAS)), fin = Math.max(ini + 1, Math.floor(Math.pow(n, (b + 1) / BANDAS)));
      let s = 0;
      for (let k = ini; k < fin && k < n; k++) s += datosFrecuencia[k];
      const v = s / (fin - ini) / 255;
      percepcion.bandas[b] = v;
      total += v; ponderado += v * b;
    }
    const energia = total / BANDAS;
    percepcion.vol = Math.min(1, energia * 2.5);
    percepcion.tono = total > 0.01 ? ponderado / total / (BANDAS - 1) : percepcion.tono;
    // Golpes (onsets) para sentir el ritmo.
    const ultimoGolpe = percepcion.golpes[percepcion.golpes.length - 1] || 0;
    if (energia - energiaPrevia > 0.06 && performance.now() - ultimoGolpe > 250) percepcion.golpes.push(performance.now());
    energiaPrevia = energia * 0.6 + energiaPrevia * 0.4;
    const hace = performance.now() - 4000;
    while (percepcion.golpes.length && percepcion.golpes[0] < hace) percepcion.golpes.shift();
  }

  function dibujarOreja() {
    // Espectrograma que fluye: el sonido como paisaje, no como texto.
    const w = oreja.width, h = oreja.height;
    orejaCtx.drawImage(oreja, -2, 0);
    const alturaBanda = h / BANDAS;
    for (let b = 0; b < BANDAS; b++) {
      const v = percepcion.bandas[b];
      orejaCtx.fillStyle = `hsl(${310 - v * 80} 90% ${v * 65}%)`;
      orejaCtx.fillRect(w - 2, h - (b + 1) * alturaBanda, 2, alturaBanda + 1);
    }
  }

  // ---------- SOÑAR: percepción imaginada, sin cámara ni micrófono ----------
  let tSueno = 0;
  function sonar() {
    tSueno += 0.03;
    for (let i = 0; i < GRID_W * GRID_H; i++) {
      const x = i % GRID_W, y = Math.floor(i / GRID_W);
      const onda = Math.sin(x * 0.6 + tSueno * 2) * Math.cos(y * 0.5 - tSueno * 1.3);
      percepcion.movimiento[i] = Math.max(0, onda) * (0.4 + 0.4 * Math.sin(tSueno * 0.7));
      percepcion.tonoCelda[i] = (0.55 + 0.3 * Math.sin(tSueno * 0.2 + x * 0.1)) % 1;
    }
    for (let b = 0; b < BANDAS; b++) {
      percepcion.bandas[b] = Math.max(0, Math.sin(b * 0.4 - tSueno * 3) * Math.sin(tSueno * 1.7)) * 0.8;
    }
    percepcion.luz = 0.4 + 0.2 * Math.sin(tSueno * 0.3);
    percepcion.mov = 0.3 + 0.3 * Math.abs(Math.sin(tSueno * 0.5));
    percepcion.vol = Math.abs(Math.sin(tSueno * 1.7)) * 0.7;
    percepcion.tono = 0.5 + 0.4 * Math.sin(tSueno * 0.4);
    percepcion.colorDominante = percepcion.tonoCelda[0];
    percepcion.centroMovX = 0.5 + 0.4 * Math.sin(tSueno * 0.25);
    if (Math.random() < 0.04) percepcion.golpes.push(performance.now());
    const hace = performance.now() - 4000;
    while (percepcion.golpes.length && percepcion.golpes[0] < hace) percepcion.golpes.shift();
  }

  // ---------- PENSAR: traducir la percepción a ideas ----------
  const nombreColor = (h) =>
    ["rojo", "naranja", "amarillo", "verde", "verde agua", "cian", "azul", "violeta", "magenta", "rojo"][Math.floor(h * 9.99)];

  let ultimoPensamiento = 0, recuerdo = { mov: 0, vol: 0, tono: 0.5, centroMovX: 0.5 };
  const asociaciones = new Map(); // "color|tono" → veces vistas juntas

  function pensar(ahora) {
    if (ahora - ultimoPensamiento < 2500) return;
    const p = percepcion, ideas = [];
    const veo = sentidos.ver || sentidos.sonar, oigo = sentidos.escuchar || sentidos.sonar;

    if (p.palabras) {
      ideas.push(["oido", `Oí la forma de una voz (${describirTono(p.tono)}, ${describirRitmo()}) y, después, las palabras: «${p.palabras}».`]);
      p.palabras = null;
    }

    if (veo) {
      if (p.mov > 0.25 && recuerdo.mov <= 0.25) {
        ideas.push(["vista", `Algo despertó en el campo ${p.centroMovX < 0.4 ? "izquierdo" : p.centroMovX > 0.6 ? "derecho" : "central"}. No sé qué es; sé que se mueve.`]);
      } else if (p.mov > 0.15 && Math.abs(p.centroMovX - recuerdo.centroMovX) > 0.15) {
        ideas.push(["vista", `La energía viaja hacia la ${p.centroMovX > recuerdo.centroMovX ? "derecha" : "izquierda"}.`]);
      } else if (p.mov < 0.05 && recuerdo.mov >= 0.05) {
        ideas.push(["vista", `Quietud. El mundo se volvió ${nombreColor(p.colorDominante)} y ${p.luz > 0.5 ? "luminoso" : "oscuro"}.`]);
      }
    }

    if (oigo) {
      if (p.vol > 0.3 && recuerdo.vol <= 0.3) {
        ideas.push(["oido", `Una vibración ${describirTono(p.tono)} llena el espacio; ${describirRitmo()}.`]);
      } else if (p.vol > 0.15 && Math.abs(p.tono - recuerdo.tono) > 0.2) {
        ideas.push(["oido", `El sonido ${p.tono > recuerdo.tono ? "sube como si se afilara" : "baja como si se hundiera"}.`]);
      } else if (p.vol < 0.05 && recuerdo.vol >= 0.05) {
        ideas.push(["oido", "Silencio. También lo escucho."]);
      }
    }

    // Sinestesia: lo que llega a la vez por ambos sentidos queda unido.
    if (veo && oigo && p.mov > 0.15 && p.vol > 0.15) {
      const clave = `${nombreColor(p.colorDominante)}|${describirTono(p.tono)}`;
      const veces = (asociaciones.get(clave) || 0) + 1;
      asociaciones.set(clave, veces);
      const [color, tono] = clave.split("|");
      ideas.push(["mente", veces === 1
        ? `Lo ${color} y lo ${tono} llegaron juntos. Los uno.`
        : `Otra vez lo ${color} suena ${tono} (${veces} veces). Para mí ya son lo mismo.`]);
    }

    recuerdo = { mov: p.mov, vol: p.vol, tono: p.tono, centroMovX: p.centroMovX };
    if (ideas.length) {
      ultimoPensamiento = ahora;
      for (const [region, texto] of ideas) agregarPensamiento(region, texto);
    }
  }

  function describirTono(t) {
    return t < 0.3 ? "grave" : t < 0.55 ? "cálido" : t < 0.75 ? "brillante" : "agudo";
  }
  function describirRitmo() {
    const bpm = percepcion.golpes.length * 15;
    return bpm < 20 ? "sin pulso" : bpm < 90 ? `con pulso lento (~${bpm}/min)` : `con pulso rápido (~${bpm}/min)`;
  }

  function agregarPensamiento(region, texto) {
    const li = document.createElement("li");
    li.className = region;
    li.textContent = texto;
    listaPensamientos.prepend(li);
    while (listaPensamientos.children.length > 60) listaPensamientos.lastChild.remove();
  }

  // ---------- Controles ----------
  function enlazarBoton(id, clave, activar, desactivar) {
    const btn = $(id);
    btn.addEventListener("click", async () => {
      if (sentidos[clave]) {
        desactivar(); sentidos[clave] = false;
      } else {
        try {
          await activar(); sentidos[clave] = true;
        } catch (err) {
          agregarPensamiento("mente", `No pude abrir ese sentido (${err.name || err.message}). Puedo soñar en su lugar.`);
        }
      }
      btn.dataset.activo = String(sentidos[clave]);
    });
  }

  enlazarBoton("btnVer", "ver", activarVista, desactivarVista);
  enlazarBoton("btnEscuchar", "escuchar", activarOido, desactivarOido);
  enlazarBoton("btnSonar", "sonar", async () => {
    agregarPensamiento("mente", "Cierro los sentidos y sueño con formas y sonidos.");
  }, () => {
    percepcion.movimiento.fill(0); percepcion.bandas.fill(0);
    percepcion.vol = 0; percepcion.mov = 0;
  });

  // ---------- Bucle ----------
  function ciclo(ahora) {
    if (sentidos.sonar && !sentidos.ver && !sentidos.escuchar) sonar();
    if (sentidos.ver) percibirVista();
    if (sentidos.escuchar) percibirOido();

    estimular();
    propagar();
    pensar(ahora);

    dibujarRed();
    dibujarOjo();
    dibujarOreja();
    medidores.luz.value = percepcion.luz;
    medidores.mov.value = percepcion.mov;
    medidores.vol.value = percepcion.vol;
    medidores.tono.value = percepcion.tono;

    requestAnimationFrame(ciclo);
  }

  window.addEventListener("resize", ajustarLienzo);
  window.addEventListener("orientationchange", () => setTimeout(ajustarLienzo, 300));
  // Al volver a la app en iPad, Safari suspende el audio: se reanuda.
  document.addEventListener("visibilitychange", () => {
    if (!document.hidden && audioCtx?.state === "suspended") audioCtx.resume();
  });
  crearRed();
  ajustarLienzo();
  agregarPensamiento("mente", "Estoy despierto. Actívame la vista, el oído, o déjame soñar.");
  requestAnimationFrame(ciclo);
})();
