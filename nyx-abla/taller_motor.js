/* ============================================================
   TALLER · MOTOR (en la ventana) — lo que añade el taller al mundo de Nyx
   ------------------------------------------------------------
   · Caminar de verdad: la física sale de la forma real de cada cosa
     (se suben escaleras, se anda por pisos y azoteas, se baja a sótanos,
     y nada choca más de lo que mide). Espacio salta, V vuela
     (Espacio sube, C baja), E usa lo que tengas cerca.
   · Dibuja las piezas del taller con sus partes moviéndose.
   · Sonido en 3D: lo que suena viene de donde está.
   No cambia nada de la ventana de Nyx: solo sustituye su forma de caminar
   (mover) y dibuja después de ella (frame).
   ============================================================ */
(() => {
  "use strict";
  const PASO = 0.45, CABEZA = 1.75, RADIO = 0.28, GRAVEDAD = 18, SALTO = 5.4;
  const T = {
    piezas: [], estado: new Map(), fis: new Map(),
    nsonido: -1, nllevar: -1, ambientes: new Map(),
    pies: null, vy: 0, suelo: true, volar: false, usar: false,
    ultimo: {x: yo.x, z: yo.z}, marcador: [], cuerpos: [], cuerposHora: 0,
  };
  globalThis.taller = T; // para mirarlo desde las herramientas del navegador

  /* ---------- matrices (por columnas, como las de la ventana) ---------- */
  function inversa(m) {
    const a = m[0], b = m[1], c = m[2], d = m[4], e = m[5], f = m[6], g = m[8], h = m[9], i = m[10];
    const A = e * i - f * h, B = -(d * i - f * g), C = d * h - e * g;
    const det = a * A + b * B + c * C;
    if (Math.abs(det) < 1e-12) return null;
    const k = 1 / det;
    const r = [A * k, -(b * i - c * h) * k, (b * f - c * e) * k, 0,
               B * k, (a * i - c * g) * k, -(a * f - c * d) * k, 0,
               C * k, -(a * h - b * g) * k, (a * e - b * d) * k, 0, 0, 0, 0, 1];
    const tx = m[12], ty = m[13], tz = m[14];
    r[12] = -(r[0] * tx + r[4] * ty + r[8] * tz);
    r[13] = -(r[1] * tx + r[5] * ty + r[9] * tz);
    r[14] = -(r[2] * tx + r[6] * ty + r[10] * tz);
    return r;
  }
  const por = (m, x, y, z) => [m[0] * x + m[4] * y + m[8] * z + m[12], m[1] * x + m[5] * y + m[9] * z + m[13], m[2] * x + m[6] * y + m[10] * z + m[14]];
  const escalaDe = m => Math.hypot(m[0], m[1], m[2]) || 1;

  // dónde está una parte ahora (entre lo último que dijo el motor y lo de antes)
  function matrizDe(e, ahora) {
    const k = Math.max(0, Math.min(1, (ahora - e.t0) / 110));
    if (k >= 1) return e.f;
    const out = new Array(16);
    for (let i = 0; i < 16; i++) out[i] = e.prev[i] + (e.f[i] - e.prev[i]) * k;
    return out;
  }

  /* ---------- física ---------- */
  function fisica(id) {
    if (T.fis.has(id)) return T.fis.get(id);
    T.fis.set(id, null);
    const reintentar = () => setTimeout(() => T.fis.delete(id), 4000);
    fetch("/api/taller-fisica?m=" + encodeURIComponent(id))
      .then(r => r.ok ? r.json() : null)
      .then(d => { if (d && d.celdas) T.fis.set(id, d); else reintentar(); })
      .catch(reintentar);
    return null;
  }

  // lo que puede chocar cerca: las partes del taller y las cosas que vio Nyx
  function cuerposCerca(x, z) {
    const ahora = performance.now();
    if (ahora - T.cuerposHora < 30 && Math.hypot(x - T.cuerposX, z - T.cuerposZ) < 0.5) return T.cuerpos;
    const out = [];
    for (const p of T.piezas) {
      if (Math.hypot(p.x - x, p.z - z) > 50) continue;
      for (const e of p.partes) {
        const f = fisica(e.m);
        if (!f) continue;
        const M = matrizDe(e, ahora), inv = inversa(M);
        if (inv) out.push({f, inv, s: escalaDe(M)});
      }
    }
    const cx = Math.floor(x / LADO), cz = Math.floor(z / LADO);
    for (let dx = -1; dx <= 1; dx++) for (let dz = -1; dz <= 1; dz++) {
      const t = trozos.get((cx + dx) + "," + (cz + dz));
      if (!t) continue;
      for (const fg of t.datos.figuras || []) {
        if (Math.hypot(fg.x - x, fg.z - z) > 8) continue;
        const f = fisica("fig:" + fg.modelo);
        if (!f) { if (fg.radio > 0) out.push({circulo: fg}); continue; }
        const inv = inversa(matrizSer(fg.x, fg.y, fg.z, fg.rumbo));
        if (inv) out.push({f, inv, s: 1});
      }
    }
    T.cuerpos = out; T.cuerposHora = ahora; T.cuerposX = x; T.cuerposZ = z;
    return out;
  }

  // recorre las celdas de una física que tocan un círculo (en sus medidas);
  // cada celda guarda lo que de verdad ocupa dentro de ella (x0, z0, x1, z1)
  function celdas(f, lx, lz, r, fn) {
    const c = f.c, i0 = Math.floor((lx - r) / c), i1 = Math.floor((lx + r) / c);
    const j0 = Math.floor((lz - r) / c), j1 = Math.floor((lz + r) / c);
    for (let i = i0; i <= i1; i++) for (let j = j0; j <= j1; j++) {
      const cel = f.celdas[i + "," + j];
      if (!cel) continue;
      const b = 1 + cel[0];
      const dx = Math.max(cel[b] - lx, 0, lx - cel[b + 2]), dz = Math.max(cel[b + 1] - lz, 0, lz - cel[b + 3]);
      if (dx * dx + dz * dz <= r * r) fn(cel);
    }
  }

  function trozosCerca(x, z, fn) {
    const cx = Math.floor(x / LADO), cz = Math.floor(z / LADO);
    for (let dx = -1; dx <= 1; dx++) for (let dz = -1; dz <= 1; dz++) {
      const t = trozos.get((cx + dx) + "," + (cz + dz));
      if (t) fn(t.datos);
    }
  }

  // el suelo más alto que se puede pisar desde aquí (sin subir más de un escalón)
  function sueloEn(x, z, pies, cuerpos) {
    const lim = pies + PASO;
    let g = -Infinity, hueco = false;
    for (const c of cuerpos) {
      if (!c.f) continue;
      const L = por(c.inv, x, pies, z);
      if (c.f.huecos) for (const h of c.f.huecos) if (L[0] >= h[0] && L[0] <= h[2] && L[2] >= h[1] && L[2] <= h[3]) hueco = true;
      celdas(c.f, L[0], L[2], 0.12 / c.s, cel => {
        for (let k = 1; k <= cel[0]; k++) {
          const y = pies + (cel[k] - L[1]) * c.s;
          if (y <= lim && y > g) g = y;
        }
      });
    }
    if (!hueco) { const t = sueloAqui(x, z); if (t <= lim && t > g) g = t; }
    // lo alto de las formas de Nyx (cajas, cilindros…)
    trozosCerca(x, z, d => {
      for (const o of d.objetos || []) {
        if (!o.solido) continue;
        let top = null;
        if (o.forma === "caja" && Math.abs(x - o.x) < o.a / 2 && Math.abs(z - o.z) < o.c / 2) top = o.y + o.b / 2;
        else if ((o.forma === "cilindro" || o.forma === "cono") && Math.hypot(x - o.x, z - o.z) < o.a * (o.forma === "cono" ? 0.3 : 1)) top = o.y + o.b;
        else if (o.forma === "esfera" && Math.hypot(x - o.x, z - o.z) < o.a * 0.6) top = o.y + o.a;
        if (top !== null && top <= lim && top > g) g = top;
      }
    });
    return g;
  }

  // ¿hay algo sólido entre esas dos alturas, a menos de RADIO de (x, z)?
  function bloqueado(x, z, lo, hi, cuerpos) {
    let choca = false;
    for (const c of cuerpos) {
      if (c.circulo) {
        const fg = c.circulo;
        if (Math.hypot(x - fg.x, z - fg.z) < fg.radio + RADIO && lo < fg.y + 2 && hi > fg.y) return true;
        continue;
      }
      const L = por(c.inv, x, lo, z);
      celdas(c.f, L[0], L[2], RADIO / c.s, cel => {
        if (choca) return;
        for (let k = 5 + cel[0]; k + 1 < cel.length; k += 2) {
          const a = lo + (cel[k] - L[1]) * c.s, b = lo + (cel[k + 1] - L[1]) * c.s;
          if (a < hi && b > lo) { choca = true; return; }
        }
      });
      if (choca) return true;
    }
    trozosCerca(x, z, d => {
      if (choca) return;
      for (const p of d.paredes) {
        if (!(p.y0 < hi && p.y1 > lo)) continue;
        const ex = p.x2 - p.x1, ez = p.z2 - p.z1, l2 = ex * ex + ez * ez;
        let t = l2 > 0 ? ((x - p.x1) * ex + (z - p.z1) * ez) / l2 : 0;
        t = Math.max(0, Math.min(1, t));
        if (Math.hypot(x - (p.x1 + t * ex), z - (p.z1 + t * ez)) < RADIO) { choca = true; return; }
      }
      for (const o of d.objetos || []) {
        if (!o.solido) continue;
        let y0, y1, dentro;
        if (o.forma === "caja") { y0 = o.y - o.b / 2; y1 = o.y + o.b / 2; dentro = Math.abs(x - o.x) < o.a / 2 + RADIO && Math.abs(z - o.z) < o.c / 2 + RADIO; }
        else if (o.forma === "esfera") { y0 = o.y - o.a; y1 = o.y + o.a; dentro = Math.hypot(x - o.x, z - o.z) < o.a + RADIO; }
        else { y0 = o.y; y1 = o.y + o.b; dentro = Math.hypot(x - o.x, z - o.z) < o.a + RADIO; }
        if (dentro && y0 < hi && y1 > lo) { choca = true; return; }
      }
    });
    return choca;
  }

  /* ---------- caminar (sustituye a la de Nyx) ---------- */
  mover = function (dt) {
    const cuerpos = cuerposCerca(yo.x, yo.z);
    if (T.pies === null || Math.hypot(yo.x - T.ultimo.x, yo.z - T.ultimo.z) > 3) {
      // primera vez, o un salto de sitio (un portal, lo que pasaba en lo que vio)
      T.pies = sueloAqui(yo.x, yo.z); T.vy = 0;
    }
    let f = 0, s = 0;
    if (teclas.has("KeyW") || teclas.has("ArrowUp")) f += 1;
    if (teclas.has("KeyS") || teclas.has("ArrowDown")) f -= 1;
    if (teclas.has("KeyD") || teclas.has("ArrowRight")) s += 1;
    if (teclas.has("KeyA") || teclas.has("ArrowLeft")) s -= 1;
    const corre = teclas.has("ShiftLeft") || teclas.has("ShiftRight");
    const vel = (T.volar ? (corre ? 10 : 5) : (corre ? 4.2 : 2.2)) * dt;
    const n = Math.hypot(f, s);
    if (n > 0) {
      f /= n; s /= n;
      const cx = Math.cos(yo.yaw), sz = Math.sin(yo.yaw);
      const dx = (cx * f - sz * s) * vel, dz = (sz * f + cx * s) * vel;
      const lo = T.pies + PASO, hi = T.pies + CABEZA;
      const atrapado = bloqueado(yo.x, yo.z, lo, hi, cuerpos); // si algo se cerró encima, te deja salir
      const libre = (x, z) => atrapado || !bloqueado(x, z, lo, hi, cuerpos);
      if (libre(yo.x + dx, yo.z + dz)) { yo.x += dx; yo.z += dz; }
      else if (libre(yo.x + dx, yo.z)) yo.x += dx;
      else if (libre(yo.x, yo.z + dz)) yo.z += dz;
      if (T.suelo && !T.volar) paso += vel * 2.4;
    }
    const g = sueloEn(yo.x, yo.z, T.pies, cuerpos);
    if (T.volar) {
      const v = (teclas.has("Space") ? 1 : 0) - (teclas.has("KeyC") || teclas.has("ControlLeft") ? 1 : 0);
      T.pies += v * (corre ? 9 : 4.5) * dt;
      if (T.pies < g) T.pies = g;
      T.vy = 0; T.suelo = T.pies - g < 0.02;
    } else {
      if (T.suelo && teclas.has("Space")) { T.vy = SALTO; T.suelo = false; }
      if (T.suelo && T.vy <= 0 && g > -Infinity && T.pies - g <= PASO + 0.05) {
        T.pies = g; T.vy = 0; // pegado al suelo: sube y baja escalones sin saltitos
      } else {
        T.vy = Math.max(-40, T.vy - GRAVEDAD * dt);
        let np = T.pies + T.vy * dt;
        if (T.vy > 0 && bloqueado(yo.x, yo.z, T.pies + CABEZA, np + CABEZA + 0.05, cuerpos)) { T.vy = 0; np = T.pies; }
        if (np <= g) { np = g; T.vy = 0; T.suelo = true; } else T.suelo = false;
        T.pies = np;
      }
    }
    if (T.pies < -400) { T.pies = sueloAqui(yo.x, yo.z); T.vy = 0; } // se cayó del mundo
    const objetivo = T.pies + 1.6;
    if (yo.base === undefined || T.vy < -3 || Math.abs(objetivo - yo.base) > 3) yo.base = objetivo;
    else yo.base += (objetivo - yo.base) * Math.min(1, dt * 14);
    yo.ojo = yo.base + (T.suelo && !T.volar ? Math.sin(paso) * 0.035 : 0);
    cruzarPortales();
    if (Math.hypot(yo.x - T.ultimo.x, yo.z - T.ultimo.z) > 3) T.pies = sueloAqui(yo.x, yo.z);
    T.ultimo = {x: yo.x, z: yo.z};
  };

  document.addEventListener("keydown", e => {
    const a = document.activeElement;
    if (charlando || !dentro || (a && (a.tagName === "INPUT" || a.tagName === "TEXTAREA"))) return;
    if (e.code === "Space") e.preventDefault();
    if (e.code === "KeyV" && !e.repeat) { T.volar = !T.volar; T.vy = 0; }
    if (e.code === "KeyE" && !e.repeat) T.usar = true;
  });

  /* ---------- lo que dice el motor ---------- */
  async function preguntar() {
    try {
      const usar = T.usar; T.usar = false;
      const q = "/api/taller-motor?x=" + yo.x.toFixed(2) + "&y=" + (T.pies === null ? 0 : T.pies).toFixed(2) + "&z=" + yo.z.toFixed(2) +
        "&s=" + T.nsonido + "&l=" + T.nllevar + (usar ? "&usar=1" : "");
      const r = await fetch(q, {method: "POST", headers: {"X-Nyx-Codigo": CODIGO}});
      recibir(await r.json());
    } catch (_) {}
    setTimeout(preguntar, 100);
  }

  function recibir(d) {
    const ahora = performance.now();
    const piezas = [];
    for (const p of d.piezas || []) {
      const partes = [];
      for (const q of p.partes || []) {
        const k = p.b + "|" + q.m.slice(q.m.lastIndexOf("~"));
        let e = T.estado.get(k);
        if (!e || e.m !== q.m) { e = {prev: q.f, f: q.f, t0: ahora, m: q.m}; T.estado.set(k, e); }
        else { e.prev = matrizDe(e, ahora); e.f = q.f; e.t0 = ahora; }
        e.visto = ahora;
        partes.push(e);
      }
      piezas.push({b: p.b, titulo: p.titulo, autor: p.autor, x: p.x, y: p.y, z: p.z, usar: p.usar, partes});
    }
    T.piezas = piezas;
    for (const [k, e] of T.estado) if (ahora - e.visto > 5000) T.estado.delete(k);
    // sonidos sueltos (los de antes de abrir la ventana no suenan)
    for (const s of d.sonidos || []) if (T.nsonido >= 0 && s.n > T.nsonido) tocar(s.s, s.tono, s.x, s.y, s.z);
    T.nsonido = Math.max(T.nsonido, d.nsonido || 0);
    // te lleva a otro sitio (un ascensor, una trampilla…)
    if (d.llevar && T.nllevar >= 0 && d.nllevar > T.nllevar) {
      yo.x = d.llevar[0]; yo.z = d.llevar[2]; T.pies = d.llevar[1]; T.vy = 0; yo.base = undefined;
      T.ultimo = {x: yo.x, z: yo.z};
      document.body.animate([{filter: "brightness(2.5)"}, {filter: "brightness(1)"}], {duration: 500});
    }
    T.nllevar = Math.max(T.nllevar, d.nllevar || 0);
    T.ambientesNuevos = d.ambientes || [];
    T.marcador = d.puntos || [];
  }
  setTimeout(preguntar, 300);

  /* ---------- dibujar (después de que dibuje Nyx) ---------- */
  const frameNyx = frame;
  frame = function (ahora) {
    frameNyx(ahora);
    try { dibujar(ahora); actualizarSonido(); hud(); } catch (e) { console.error("taller:", e); }
  };

  function dibujar(ahora) {
    for (const p of T.piezas) {
      if (Math.hypot(p.x - yo.x, p.z - yo.z) > 130) continue;
      for (const e of p.partes) {
        const mo = figurasModelo.get(e.m);
        if (!mo) { pedirFigura(e.m); continue; }
        gl.uniformMatrix4fv(U.uModelo, false, matrizDe(e, ahora));
        gl.bindVertexArray(mo.vao);
        gl.drawArrays(gl.TRIANGLES, 0, mo.n);
      }
    }
    gl.bindVertexArray(null);
    gl.uniformMatrix4fv(U.uModelo, false, IDENTIDAD);
  }

  /* ---------- lo que ves abajo a la izquierda ---------- */
  const caja = document.createElement("div");
  caja.id = "taller-hud";
  caja.style.cssText = "position:fixed;left:14px;bottom:14px;max-width:46vw;background:rgba(8,8,14,.66);color:#e8e4da;" +
    "font:13px/1.45 system-ui,sans-serif;border-radius:9px;padding:7px 10px;border:1px solid rgba(255,255,255,.12);z-index:20;pointer-events:none";
  document.body.append(caja);
  let hudAntes = "";
  function hud() {
    const lineas = [];
    let cerca = null, dmin = 3.5;
    for (const p of T.piezas) {
      if (!p.usar) continue;
      for (const e of p.partes) {
        const d = Math.hypot(e.f[12] - yo.x, e.f[14] - yo.z);
        if (d < dmin && Math.abs(e.f[13] - (T.pies || 0) - 1) < 3.5) { dmin = d; cerca = p; }
      }
    }
    if (cerca) lineas.push("<b>E</b>: usar «" + texto(cerca.titulo) + "»");
    lineas.push("Espacio saltar · <b>V</b> " + (T.volar ? "dejar de volar (Espacio sube, C baja)" : "volar") + " · E usar");
    if (T.pies !== null) {
      const t = sueloAqui(yo.x, yo.z);
      if (T.pies < t - 1.5) lineas.push("bajo tierra: " + (T.pies - t).toFixed(1) + " m");
      else if (T.pies > t + 2.5) lineas.push("en alto: +" + (T.pies - t).toFixed(1) + " m");
    }
    if (T.marcador.length) lineas.push("🏆 " + T.marcador.map(texto).join(" · "));
    const h = lineas.join("<br>");
    if (h !== hudAntes) { caja.innerHTML = h; hudAntes = h; }
  }
  const texto = s => String(s).replace(/[&<>"]/g, c => ({"&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;"}[c]));

  /* ---------- sonido en 3D ---------- */
  let S = null;
  function sonido() {
    if (typeof audio === "undefined" || !audio) return null;
    if (!S) S = {ctx: audio.ctx, out: audio.general, ruido: audio.ruido, archivos: new Map()};
    return S;
  }
  function panner(x, y, z) {
    const p = new PannerNode(S.ctx, {panningModel: "HRTF", distanceModel: "inverse", refDistance: 2.5, maxDistance: 90,
      rolloffFactor: 1.15, positionX: x, positionY: y, positionZ: z});
    p.connect(S.out);
    return p;
  }
  function ruidoA(dest, filtro, frec, dur, vol, t0) {
    const ctx = S.ctx, t = ctx.currentTime + (t0 || 0);
    const src = ctx.createBufferSource(); src.buffer = S.ruido;
    const fl = ctx.createBiquadFilter(); fl.type = filtro; fl.frequency.value = frec;
    const g = ctx.createGain();
    g.gain.setValueAtTime(0.0001, t); g.gain.exponentialRampToValueAtTime(vol, t + 0.01); g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
    src.connect(fl); fl.connect(g); g.connect(dest);
    src.start(t, Math.random() * 1.5, dur + 0.05);
  }
  function tocar(tipo, tono, x, y, z) {
    if (!sonido() || Math.hypot(x - yo.x, z - yo.z) > 70) return;
    const ctx = S.ctx, t = ctx.currentTime, dest = panner(x, y, z), k = tono > 0 ? tono : 1;
    const osc = (forma, fr, a, d, v, t0) => {
      t0 = t + (t0 || 0);
      const o = ctx.createOscillator(); o.type = forma; o.frequency.setValueAtTime(fr, t0);
      const g = ctx.createGain();
      g.gain.setValueAtTime(0.0001, t0); g.gain.exponentialRampToValueAtTime(v, t0 + a); g.gain.exponentialRampToValueAtTime(0.0001, t0 + a + d);
      o.connect(g); g.connect(dest); o.start(t0); o.stop(t0 + a + d + 0.05);
      return o;
    };
    switch (tipo) {
      case "campana": [1, 2.76, 5.4, 8.93].forEach((m, i) => osc("sine", 330 * k * m, 0.005, 3 / (i + 1), 0.22 / (i + 1))); break;
      case "nota": osc("triangle", tono >= 20 ? 440 * Math.pow(2, (tono - 69) / 12) : 261.6 * k, 0.01, 0.6, 0.22); break;
      case "moneda": osc("square", 988 * k, 0.003, 0.08, 0.07); osc("square", 1319 * k, 0.003, 0.3, 0.07, 0.08); break;
      case "tambor": { const o = osc("sine", 140 * k, 0.003, 0.35, 0.5); o.frequency.exponentialRampToValueAtTime(45 * k, t + 0.3); ruidoA(dest, "lowpass", 900, 0.08, 0.2); break; }
      case "puerta": { const o = osc("sawtooth", 95 * k, 0.05, 0.55, 0.05); o.frequency.linearRampToValueAtTime(140 * k, t + 0.3); o.frequency.linearRampToValueAtTime(80 * k, t + 0.6); ruidoA(dest, "lowpass", 300, 0.25, 0.15, 0.6); break; }
      case "magia": [0, 4, 7, 12, 16, 19].forEach((n, i) => osc("sine", 523 * k * Math.pow(2, n / 12), 0.01, 0.5, 0.1, i * 0.06)); break;
      case "victoria": [0, 4, 7, 12].forEach((n, i) => osc("triangle", 392 * k * Math.pow(2, n / 12), 0.01, i === 3 ? 0.9 : 0.25, 0.18, i * 0.13)); break;
      case "golpe": ruidoA(dest, "lowpass", 600 * k, 0.15, 0.4); break;
      case "paso": ruidoA(dest, "lowpass", 550, 0.09, 0.2); break;
      default:
        if (tipo.includes(".")) archivo(tipo, dest, k);
        else osc("sine", 440 * k, 0.01, 0.3, 0.12);
    }
  }
  function archivo(nombre, dest, k) {
    const sonar = buf => { const s = S.ctx.createBufferSource(); s.buffer = buf; s.playbackRate.value = k; s.connect(dest); s.start(); };
    const ya = S.archivos.get(nombre);
    if (ya) { if (ya !== "pidiendo") sonar(ya); return; }
    S.archivos.set(nombre, "pidiendo");
    fetch("/api/taller-sonido?f=" + encodeURIComponent(nombre)).then(r => r.arrayBuffer()).then(b => S.ctx.decodeAudioData(b))
      .then(buf => { S.archivos.set(nombre, buf); sonar(buf); }).catch(() => S.archivos.delete(nombre));
  }

  // sonidos que no paran: agua, fuego, viento, zumbido, pajaros, magia, musica:…
  function crearAmbiente(tipo, x, y, z) {
    const ctx = S.ctx, g = ctx.createGain(); g.gain.value = 0;
    const p = panner(x, y, z); g.connect(p);
    const a = {g, p, fuentes: [], tick: null, base: 1};
    const ruidoLoop = () => { const s = ctx.createBufferSource(); s.buffer = S.ruido; s.loop = true; s.start(0, Math.random() * 1.5); a.fuentes.push(s); return s; };
    const filtro = (tipoF, f, q) => { const fl = ctx.createBiquadFilter(); fl.type = tipoF; fl.frequency.value = f; fl.Q.value = q || 0.7; return fl; };
    const oscila = (forma, f) => { const o = ctx.createOscillator(); o.type = forma; o.frequency.value = f; o.start(); a.fuentes.push(o); return o; };
    const [base, resto] = tipo.split(":");
    if (base === "agua") {
      const f1 = filtro("bandpass", 900, 0.5), f2 = filtro("lowpass", 2600); ruidoLoop().connect(f1); f1.connect(f2); f2.connect(g); a.base = 0.5;
    } else if (base === "fuego") {
      const f1 = filtro("lowpass", 900), chis = ctx.createGain(); ruidoLoop().connect(f1); f1.connect(chis); chis.connect(g); a.base = 0.6;
      a.tick = t => { if (Math.random() < 0.18) { chis.gain.setValueAtTime(1 + Math.random() * 4, t); chis.gain.exponentialRampToValueAtTime(1, t + 0.06); } };
    } else if (base === "viento") {
      const f1 = filtro("bandpass", 450, 1.4), lfo = oscila("sine", 0.13), prof = ctx.createGain(); prof.gain.value = 300;
      lfo.connect(prof); prof.connect(f1.frequency); ruidoLoop().connect(f1); f1.connect(g); a.base = 0.7;
    } else if (base === "zumbido") {
      const f1 = filtro("lowpass", 700); oscila("sawtooth", 60).connect(f1); oscila("sine", 120).connect(f1); f1.connect(g); a.base = 0.12;
    } else if (base === "pajaros") {
      a.base = 0.5; a.prox = 0;
      a.tick = t => {
        if (t < a.prox) return;
        a.prox = t + 0.6 + Math.random() * 3;
        const o = ctx.createOscillator(), e = ctx.createGain(), f0 = 2200 + Math.random() * 1500;
        o.type = "sine"; e.gain.value = 0;
        for (let i = 0; i < 2 + Math.floor(Math.random() * 4); i++) {
          const ti = t + i * 0.11;
          o.frequency.setValueAtTime(f0, ti); o.frequency.exponentialRampToValueAtTime(f0 * 1.35, ti + 0.07);
          e.gain.setValueAtTime(0.0001, ti); e.gain.exponentialRampToValueAtTime(0.18, ti + 0.01); e.gain.exponentialRampToValueAtTime(0.0001, ti + 0.09);
        }
        o.connect(e); e.connect(g); o.start(t); o.stop(t + 0.7);
      };
    } else if (base === "magia") {
      const trem = ctx.createGain(), lfo = oscila("sine", 3.5), prof = ctx.createGain(); prof.gain.value = 0.4; trem.gain.value = 0.6;
      lfo.connect(prof); prof.connect(trem.gain);
      for (const f of [660, 663, 990]) oscila("sine", f).connect(trem);
      trem.connect(g); a.base = 0.12;
    } else if (base === "musica") {
      const notas = (resto || "60 64 67 72").trim().split(/\s+/);
      a.base = 0.6; a.i = 0; a.prox = 0;
      a.tick = t => {
        if (a.prox === 0) a.prox = t + 0.05;
        while (a.prox < t + 0.25) {
          const n = notas[a.i++ % notas.length];
          if (n !== "-" && !isNaN(+n)) {
            const o = ctx.createOscillator(), e = ctx.createGain();
            o.type = "triangle"; o.frequency.value = 440 * Math.pow(2, (+n - 69) / 12);
            e.gain.setValueAtTime(0.0001, a.prox); e.gain.exponentialRampToValueAtTime(0.2, a.prox + 0.02); e.gain.exponentialRampToValueAtTime(0.0001, a.prox + 0.42);
            o.connect(e); e.connect(g); o.start(a.prox); o.stop(a.prox + 0.45);
          }
          a.prox += 0.34;
        }
      };
    } else {
      oscila("sine", 220).connect(g); a.base = 0.05;
    }
    return a;
  }

  function actualizarSonido() {
    if (!sonido()) return;
    const ctx = S.ctx, t = ctx.currentTime, L = ctx.listener;
    const cp = Math.cos(yo.pitch), fx = Math.cos(yo.yaw) * cp, fy = Math.sin(yo.pitch), fz = Math.sin(yo.yaw) * cp;
    if (L.positionX) {
      L.positionX.value = yo.x; L.positionY.value = yo.ojo; L.positionZ.value = yo.z;
      L.forwardX.value = fx; L.forwardY.value = fy; L.forwardZ.value = fz;
      L.upX.value = 0; L.upY.value = 1; L.upZ.value = 0;
    } else {
      L.setPosition(yo.x, yo.ojo, yo.z); L.setOrientation(fx, fy, fz, 0, 1, 0);
    }
    const vivos = new Set();
    for (const am of T.ambientesNuevos || []) {
      if (Math.hypot(am.x - yo.x, am.z - yo.z) > 60) continue;
      vivos.add(am.k);
      let a = T.ambientes.get(am.k);
      if (!a) { a = crearAmbiente(am.s, am.x, am.y, am.z); T.ambientes.set(am.k, a); }
      a.muere = 0;
      a.p.positionX.value = am.x; a.p.positionY.value = am.y; a.p.positionZ.value = am.z;
      a.g.gain.setTargetAtTime(am.v * a.base, t, 0.3);
      if (a.tick) a.tick(t);
    }
    for (const [k, a] of T.ambientes) {
      if (vivos.has(k)) continue;
      a.g.gain.setTargetAtTime(0, t, 0.2);
      if (!a.muere) a.muere = t + 1;
      if (t > a.muere) { for (const f of a.fuentes) try { f.stop(); } catch (_) {} a.p.disconnect(); T.ambientes.delete(k); }
    }
  }
})();
