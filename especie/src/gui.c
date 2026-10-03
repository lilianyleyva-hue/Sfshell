/* La app de la Especie Abla, dibujada en C.
 *
 * Cinco pantallas con una barra de pestañas abajo, como una app de iPad:
 *   Especie   la red de las 27 mentes con sus mensajes, y lo que piensan
 *   Misiones  el nivel, el problema que resuelven en equipo, sus logros
 *   Visión    la foto que miran (en colores reales), lo que ven, la cola de fotos
 *   Hablar    una charla con un ser o con toda la especie
 *   Terminal  los comandos de siempre
 *
 * Es una interfaz "de modo inmediato": cada cuadro se dibuja entero a partir
 * del estado de la especie, y los botones responden a los toques de ese cuadro. */
#include "gui.h"

#include "especie.h"
#include "misiones.h"
#include "vision.h"

/* ---------- colores ---------- */
#define C_FONDO 0x0B0D14
#define C_PANEL 0x141826
#define C_PANEL2 0x1C2133
#define C_BORDE 0x2A3045
#define C_TEXTO 0xE8EAF2
#define C_TENUE 0x8A90A6
#define C_ACENTO 0x7C9CFF
#define C_BIEN 0x5BD68A
#define C_MAL 0xFF6B6B
#define C_ORO 0xFFC44D
#define C_ABLA 0xF2F0FF
static const uint32_t C_TRIBU[3] = {0xFF6AD5, 0x4FD1FF, 0xB8FF6A};

/* ---------- escala: se diseña en puntos, se dibuja en píxeles ---------- */
static float S = 2;
static int pt(float v) { return (int)(v * S + 0.5f); }
static const Fuente* F(int puntos, int negrita) { return fuente(pt((float)puntos), negrita); }

enum { T_ESPECIE, T_MISIONES, T_VISION, T_HABLAR, T_TERMINAL, NTABS };
static int pestana = T_ESPECIE, elegido = -1;
static unsigned peticiones; /* 1 teclado, 2 fotos, 4 cámara */
static int camara_encendida = 0;
static char aviso[300];
static double aviso_hasta = 0; /* en ms del reloj de la página */
static double reloj = 0;

/* ---------- toques ---------- */
static struct {
  float x, y, x0, y0, yprev;
  int abajo, pulsado, soltado, arrastre;
  int p_pulsado, p_soltado;
  float rueda, p_rueda;
} P;

static int dentro(float px, float py, int x, int y, int w, int h) { return px >= x && py >= y && px < x + w && py < y + h; }
static int toque(int x, int y, int w, int h) {
  return P.soltado && !P.arrastre && dentro(P.x, P.y, x, y, w, h) && dentro(P.x0, P.y0, x, y, w, h);
}
static int apretado(int x, int y, int w, int h) {
  return P.abajo && !P.arrastre && dentro(P.x, P.y, x, y, w, h) && dentro(P.x0, P.y0, x, y, w, h);
}

typedef struct {
  float desp, contenido;
  int pegado_abajo;
} Desplazable;

/* arrastrar o usar la rueda dentro de una zona la desplaza */
static void desplazar(Desplazable* d, int x, int y, int w, int h) {
  float max = d->contenido - h > 0 ? d->contenido - h : 0;
  int antes_abajo = d->desp >= max - 2;
  if (P.abajo && P.arrastre && dentro(P.x0, P.y0, x, y, w, h)) d->desp -= P.y - P.yprev;
  if (P.rueda != 0 && dentro(P.x, P.y, x, y, w, h)) d->desp += P.rueda;
  if (d->pegado_abajo && antes_abajo && !(P.abajo && P.arrastre) && P.rueda == 0) d->desp = max;
  if (d->desp > max) d->desp = max;
  if (d->desp < 0) d->desp = 0;
}

/* ---------- piezas ---------- */
static void tarjeta(int x, int y, int w, int h) {
  lz_rrect((float)x, (float)y, (float)w, (float)h, (float)pt(16), C_PANEL, 255);
  lz_rrect_borde((float)x, (float)y, (float)w, (float)h, (float)pt(16), (float)pt(1), C_BORDE, 255);
}

static void titulo(int x, int y, const char* t) { lz_texto(F(17, 1), x, y, t, C_TEXTO, 255); }

static void texto_centrado(const Fuente* f, int x, int y, int w, const char* s, uint32_t c, int a) {
  lz_texto(f, x + (w - lz_ancho(f, s)) / 2, y, s, c, a);
}

static int boton(int x, int y, int w, int h, const char* t, int estilo) { /* 0 normal, 1 principal, 2 suave */
  int ap = apretado(x, y, w, h);
  uint32_t fondo = estilo == 1 ? C_ACENTO : estilo == 2 ? C_PANEL : C_PANEL2;
  lz_rrect((float)x, (float)y, (float)w, (float)h, (float)pt(10), fondo, ap ? 170 : 255);
  if (estilo != 1) lz_rrect_borde((float)x, (float)y, (float)w, (float)h, (float)pt(10), (float)pt(1), C_BORDE, 255);
  const Fuente* f = F(15, estilo == 1);
  texto_centrado(f, x, y + (h - lz_alto_linea(f)) / 2, w, t, estilo == 1 ? 0x0B0D14 : C_TEXTO, 255);
  return toque(x, y, w, h);
}

static void barra_progreso(int x, int y, int w, int h, float v, uint32_t c) {
  if (v < 0) v = 0;
  if (v > 1) v = 1;
  lz_rrect((float)x, (float)y, (float)w, (float)h, h / 2.0f, C_PANEL2, 255);
  if (v > 0) lz_rrect((float)x, (float)y, w * v < h ? (float)h : w * v, (float)h, h / 2.0f, c, 255);
}

static void miles(char* out, size_t n, unsigned long long v) {
  char tmp[32];
  snprintf(tmp, sizeof tmp, "%llu", v);
  size_t l = strlen(tmp), k = 0;
  for (size_t i = 0; i < l && k + 2 < n; i++) {
    out[k++] = tmp[i];
    if ((l - i - 1) % 3 == 0 && i + 1 < l) out[k++] = '.';
  }
  out[k] = 0;
}

/* ---------- campos de texto ---------- */
typedef struct {
  char txt[300];
  int enviado;
} Campo;
static Campo c_hablar, c_mision, c_terminal;
static Campo* foco = NULL;

static void campo(Campo* c, int x, int y, int w, int h, const char* indicacion) {
  if (toque(x, y, w, h)) {
    foco = c;
    peticiones |= 1;
  }
  int activo = foco == c;
  lz_rrect((float)x, (float)y, (float)w, (float)h, (float)pt(10), C_FONDO, 255);
  lz_rrect_borde((float)x, (float)y, (float)w, (float)h, (float)pt(10), (float)pt(activo ? 2 : 1), activo ? C_ACENTO : C_BORDE, 255);
  const Fuente* f = F(15, 0);
  int ty = y + (h - lz_alto_linea(f)) / 2;
  lz_recorte(x + pt(10), y, w - pt(20), h);
  if (c->txt[0]) {
    int ancho = lz_ancho(f, c->txt), sobra = ancho - (w - pt(30));
    int tx = x + pt(12) - (sobra > 0 ? sobra : 0);
    lz_texto(f, tx, ty, c->txt, C_TEXTO, 255);
    if (activo && ((int)(reloj / 500) % 2 == 0)) lz_rect(tx + ancho + pt(1), ty + pt(2), pt(2), lz_alto_linea(f) - pt(4), C_ACENTO, 255);
  } else {
    lz_texto(f, x + pt(12), ty, indicacion, C_TENUE, 255);
    if (activo && ((int)(reloj / 500) % 2 == 0)) lz_rect(x + pt(12), ty + pt(2), pt(2), lz_alto_linea(f) - pt(4), C_ACENTO, 255);
  }
  lz_sin_recorte();
}

/* ---------- mensajes que viajan y brillo de las mentes ---------- */
typedef struct {
  int de, para, c;
  float t;
} Particula;
#define MAX_PART 300
static Particula part[MAX_PART];
static int npart = 0;
static uint64_t visto_red = 0, visto_linea = 0;
static float brillo[NUM_SERES];

static void animar(float dt) {
  uint64_t min = E.nred > RED ? E.nred - RED : 0;
  if (visto_red < min) visto_red = min;
  if (E.nred - visto_red > 80) visto_red = E.nred - 80;
  for (; visto_red < E.nred; visto_red++) {
    if (npart == MAX_PART) memmove(part, part + 1, sizeof(Particula) * --npart);
    part[npart].de = E.red[visto_red % RED].de;
    part[npart].para = E.red[visto_red % RED].para;
    part[npart].c = E.red[visto_red % RED].c;
    part[npart++].t = 0;
  }
  for (; visto_linea < E.nlinea; visto_linea++) brillo[E.corriente[visto_linea % CORRIENTE].ser] = 1;
  int j = 0;
  for (int k = 0; k < npart; k++) {
    part[k].t += dt / 900.0f;
    if (part[k].t < 1) part[j++] = part[k];
    else if (brillo[part[k].para] < 0.5f) brillo[part[k].para] = 0.5f;
  }
  npart = j;
  for (int k = 0; k < NUM_SERES; k++) brillo[k] *= 1 - dt / 1500.0f;
}

/* raíz y seno sin libm */
static float raizf_aprox(float v) {
  if (v <= 0) return 0;
  float r = v > 1 ? v / 2 : 1;
  for (int i = 0; i < 12; i++) r = 0.5f * (r + v / r);
  return r;
}

/* seno sin libm, para colocar las mentes en círculo */
static float seno(float x) {
  const float PI = 3.14159265f;
  while (x > PI) x -= 2 * PI;
  while (x < -PI) x += 2 * PI;
  if (x > PI / 2) x = PI - x;
  if (x < -PI / 2) x = -PI - x;
  float x2 = x * x;
  return x * (1 - x2 / 6 * (1 - x2 / 20 * (1 - x2 / 42 * (1 - x2 / 72))));
}
static float coseno(float x) { return seno(x + 1.5707963f); }

/* ---------- pantalla: Especie ---------- */
static Desplazable d_pensamientos = {0, 0, 0}, d_ser = {0, 0, 0};

static void red(int x, int y, int w, int h) {
  tarjeta(x, y, w, h);
  titulo(x + pt(16), y + pt(14), "La especie");
  lz_texto(F(13, 0), x + pt(16), y + pt(38), "Toca una mente para conocerla", C_TENUE, 255);
  float cx = x + w / 2.0f, cy = y + h / 2.0f + pt(10);
  float R = (w < h ? w : h) * 0.36f;
  float px[NUM_SERES], py[NUM_SERES];
  for (int i = 0; i < NUM_SERES; i++) {
    int tribu = i / 9, k = i % 9;
    float ang = -1.5707963f + tribu * 2.0943951f + (k - 4) * 0.2f;
    float r = R * (k % 2 ? 0.8f : 1.0f);
    px[i] = cx + coseno(ang) * r;
    py[i] = cy + seno(ang) * r;
  }
  lz_recorte(x, y, w, h);
  /* mensajes en camino */
  for (int k = 0; k < npart; k++) {
    Particula* p = &part[k];
    float ax = px[p->de], ay = py[p->de], bx = px[p->para], by = py[p->para];
    lz_linea(ax, ay, bx, by, (float)pt(1), p->c ? C_ORO : C_ABLA, 18);
    float mx = ax + (bx - ax) * p->t, my = ay + (by - ay) * p->t;
    if (p->c) lz_rrect(mx - pt(3), my - pt(3), (float)pt(6), (float)pt(6), (float)pt(1), C_ORO, 255);
    else lz_circulo(mx, my, (float)pt(3), C_ABLA, 255);
  }
  /* las mentes */
  const Fuente* f = F(12, 0);
  for (int i = 0; i < NUM_SERES; i++) {
    Ser* s = &E.seres[i];
    float r = pt(7) + pt(1) * raizf_aprox((float)s->saber.n) * 0.9f;
    if (brillo[i] > 0.05f) lz_circulo(px[i], py[i], r * 2.4f, C_TRIBU[s->tribu], (int)(brillo[i] * 70));
    lz_circulo(px[i], py[i], r, C_TRIBU[s->tribu], 255);
    if (i == elegido) {
      lz_circulo(px[i], py[i], r + pt(5), 0xFFFFFF, 60);
      lz_circulo(px[i], py[i], r + pt(3), C_TRIBU[s->tribu], 255);
    }
    int ancho = lz_ancho(f, s->nombre);
    lz_texto(f, (int)px[i] - ancho / 2, (int)(py[i] + r + pt(3)), s->nombre, i == elegido ? 0xFFFFFF : 0xC9CDE0, 255);
    if (toque((int)(px[i] - r - pt(10)), (int)(py[i] - r - pt(10)), (int)(2 * r + pt(20)), (int)(2 * r + pt(20)))) {
      elegido = i;
      d_ser.desp = 0;
    }
  }
  /* leyenda */
  const char* nombres[3] = {"Lenguaje", "Datos", "Lógica"};
  int lx = x + pt(16), ly = y + h - pt(30);
  for (int t = 0; t < 3; t++) {
    lz_circulo((float)lx + pt(5), (float)ly + pt(9), (float)pt(5), C_TRIBU[t], 255);
    lx += pt(14) + lz_texto(F(13, 0), lx + pt(14), ly, nombres[t], C_TENUE, 255) + pt(14);
  }
  lz_circulo((float)lx + pt(5), (float)ly + pt(9), (float)pt(3), C_ABLA, 255);
  lx += pt(14) + lz_texto(F(13, 0), lx + pt(14), ly, "frase en Abla", C_TENUE, 255) + pt(14);
  lz_rrect((float)lx + pt(2), (float)ly + pt(6), (float)pt(6), (float)pt(6), (float)pt(1), C_ORO, 255);
  lz_texto(F(13, 0), lx + pt(14), ly, "data C", C_TENUE, 255);
  lz_sin_recorte();
}

static void pensamientos(int x, int y, int w, int h) {
  tarjeta(x, y, w, h);
  titulo(x + pt(16), y + pt(14), "Pensamientos");
  int zy = y + pt(48), zh = h - pt(56);
  desplazar(&d_pensamientos, x, zy, w, zh);
  lz_recorte(x + pt(8), zy, w - pt(16), zh);
  int cy = zy - (int)d_pensamientos.desp, alto = 0;
  const Fuente *fn = F(13, 1), *ft = F(14, 0);
  uint64_t desde = E.nlinea > 80 ? E.nlinea - 80 : 0;
  for (uint64_t n = E.nlinea; n > desde; n--) {
    Linea* l = &E.corriente[(n - 1) % CORRIENTE];
    const char* cuerpo = strstr(l->texto, "): ");
    cuerpo = cuerpo ? cuerpo + 3 : l->texto;
    Ser* s = &E.seres[l->ser];
    int ph = lz_parrafo(ft, 0, 0, w - pt(48), cuerpo, 0, 0, 6, 0);
    int bloque = pt(30) + ph + pt(10);
    if (cy + bloque > zy && cy < zy + zh) {
      lz_rrect((float)x + pt(14), (float)cy, (float)pt(3), (float)(bloque - pt(10)), (float)pt(1), C_TRIBU[s->tribu], 255);
      lz_texto(fn, x + pt(26), cy + pt(2), s->nombre, C_TRIBU[s->tribu], 255);
      const char* t = strchr(l->texto, '=');
      char ciclo[32];
      snprintf(ciclo, sizeof ciclo, "ciclo %.*s", t ? (int)strcspn(t + 1, "]") : 0, t ? t + 1 : "");
      lz_texto(F(12, 0), x + w - pt(20) - lz_ancho(F(12, 0), ciclo), cy + pt(4), ciclo, C_TENUE, 255);
      lz_parrafo(ft, x + pt(26), cy + pt(26), w - pt(48), cuerpo, C_TEXTO, 255, 6, 1);
    }
    cy += bloque;
    alto += bloque;
  }
  d_pensamientos.contenido = (float)alto;
  lz_sin_recorte();
}

static void ficha_ser(int x, int y, int w, int h) {
  Ser* s = &E.seres[elegido];
  tarjeta(x, y, w, h);
  if (boton(x + pt(12), y + pt(10), pt(140), pt(36), "‹ Pensamientos", 2)) {
    elegido = -1;
    return;
  }
  int zy = y + pt(56), zh = h - pt(64);
  desplazar(&d_ser, x, zy, w, zh);
  lz_recorte(x + pt(8), zy, w - pt(16), zh);
  int cy = zy - (int)d_ser.desp, ix = x + pt(18), iw = w - pt(36);
  char b[512];
  snprintf(b, sizeof b, "%d. %s", elegido + 1, s->nombre);
  lz_texto(F(26, 1), ix, cy, b, C_TRIBU[s->tribu], 255);
  cy += pt(38);
  static const char* TRIBUS[3] = {"lenguaje", "datos", "lógica"};
  snprintf(b, sizeof b, "«quien %s» · tribu %s", idioma_glosa_tmp(s->concepto), TRIBUS[s->tribu]);
  lz_texto(F(14, 0), ix, cy, b, C_TENUE, 255);
  cy += pt(30);
  /* cifras */
  int nuevas = 0;
  for (int p = BASE; p < idioma_tamano(); p++) nuevas += conoce_palabra(s, p);
  char v[6][64];
  const char* k[6] = {"hechos", "pensamientos", "contexto (tokens)", "largo plazo (recuerdos)", "palabras nuevas", "mensajes ↑ ↓"};
  miles(v[0], 64, s->saber.n);
  miles(v[1], 64, s->pensamientos);
  miles(v[2], 64, s->memoria.usados);
  miles(v[3], 64, s->memoria.total);
  snprintf(v[4], 64, "%d", nuevas);
  snprintf(v[5], 64, "%llu · %llu", (unsigned long long)s->enviados, (unsigned long long)s->recibidos);
  int cw = (iw - pt(16)) / 3;
  for (int i = 0; i < 6; i++) {
    int bx = ix + (i % 3) * (cw + pt(8)), by = cy + (i / 3) * pt(62);
    lz_rrect((float)bx, (float)by, (float)cw, (float)pt(54), (float)pt(10), C_PANEL2, 255);
    lz_texto(F(12, 0), bx + pt(10), by + pt(7), k[i], C_TENUE, 255);
    lz_texto_max(F(17, 1), bx + pt(10), by + pt(25), cw - pt(20), v[i], C_TEXTO, 255);
  }
  cy += pt(130);
  lz_texto(F(15, 1), ix, cy, "Ahora piensa", C_TEXTO, 255);
  cy += pt(26);
  cy += lz_parrafo(F(14, 0), ix, cy, iw, s->ultimo ? s->ultimo : "", C_TEXTO, 255, 0, 1) + pt(14);
  lz_texto(F(15, 1), ix, cy, "Su mente", C_TEXTO, 255);
  cy += pt(26);
  for (size_t i = s->memoria.n, n = 0; i > 0 && n < 8; i--, n++) {
    const Recuerdo* r = memoria_en(&s->memoria, i - 1);
    snprintf(b, sizeof b, "[%llu] %s", (unsigned long long)r->tick, r->texto);
    cy += lz_parrafo(F(13, 0), ix, cy, iw, b, C_TENUE, 255, 3, 1) + pt(6);
  }
  cy += pt(10);
  snprintf(b, sizeof b, "Su conocimiento, en C (/conocimiento/%s.c)", s->nombre);
  lz_texto(F(15, 1), ix, cy, b, C_TEXTO, 255);
  cy += pt(28);
  snprintf(b, sizeof b, "const struct hecho %s[] = {", s->nombre);
  lz_texto(F(13, 0), ix, cy, b, 0x7AA2FF, 255);
  cy += pt(22);
  for (size_t i = 0; i < s->saber.n && i < 40; i++) {
    const struct hecho* hh = &s->saber.h[i];
    snprintf(b, sizeof b, "  {%u, %u, %u, %.2ff, %u}, /* %s */", hh->s, hh->r, hh->o, (double)hh->confianza, (unsigned)hh->fuente,
             glosa_hecho(hh));
    lz_texto_max(F(13, 0), ix, cy, iw, b, 0x7AA2FF, 255);
    cy += pt(21);
  }
  d_ser.contenido = (float)(cy + (int)d_ser.desp - zy + pt(10));
  lz_sin_recorte();
}

static void pantalla_especie(int x, int y, int w, int h) {
  int ancho_red = w * 58 / 100;
  red(x, y, ancho_red, h);
  if (elegido >= 0) ficha_ser(x + ancho_red + pt(14), y, w - ancho_red - pt(14), h);
  else pensamientos(x + ancho_red + pt(14), y, w - ancho_red - pt(14), h);
}

/* ---------- pantalla: Misiones ---------- */
static void pantalla_misiones(int x, int y, int w, int h) {
  int ia = w * 58 / 100;
  tarjeta(x, y, ia, h);
  int ix = x + pt(20), iw = ia - pt(40), cy = y + pt(18);
  char b[400];
  snprintf(b, sizeof b, "Nivel %d", M.nivel);
  lz_texto(F(32, 1), ix, cy, b, C_TEXTO, 255);
  lz_texto(F(18, 1), ix + lz_ancho(F(32, 1), b) + pt(14), cy + pt(14), nivel_nombre(M.nivel), C_ACENTO, 255);
  cy += pt(54);
  barra_progreso(ix, cy, iw, pt(12), M.meta ? (float)M.aciertos / M.meta : 0, C_BIEN);
  int aw, ah;
  agudeza_de_nivel(M.nivel, &aw, &ah);
  snprintf(b, sizeof b, "%d de %d aciertos para subir · ahora ven a %d×%d píxeles", M.aciertos, M.meta, aw, ah);
  lz_texto(F(13, 0), ix, cy + pt(18), b, C_TENUE, 255);
  cy += pt(48);

  /* el problema */
  lz_rrect((float)ix, (float)cy, (float)iw, (float)pt(110), (float)pt(14), C_PANEL2, 255);
  lz_texto(F(13, 0), ix + pt(16), cy + pt(12), M.del_humano ? "Tu problema" : "Problema", C_TENUE, 255);
  lz_texto_max(F(32, 1), ix + pt(16), cy + pt(38), iw - pt(32), M.enunciado[0] ? M.enunciado : "…", C_TEXTO, 255);
  cy += pt(126);

  /* fases */
  int paso_w = iw / NFASES;
  for (int f = 0; f < NFASES; f++) {
    int fx = ix + f * paso_w + paso_w / 2, actual = f == M.fase, hecho = f < M.fase;
    if (f + 1 < NFASES) lz_linea((float)fx, (float)cy + pt(12), (float)fx + paso_w, (float)cy + pt(12), (float)pt(2), hecho ? C_ACENTO : C_BORDE, 255);
    lz_circulo((float)fx, (float)cy + pt(12), (float)pt(actual ? 11 : 8), actual ? C_ACENTO : hecho ? 0x4A5A9A : C_PANEL2, 255);
    texto_centrado(F(13, actual), fx - paso_w / 2, cy + pt(30), paso_w, fase_nombre(f), actual ? C_TEXTO : C_TENUE, 255);
  }
  cy += pt(60);
  cy += lz_parrafo(F(15, 0), ix, cy, iw, M.paso, C_TEXTO, 255, 2, 1) + pt(12);

  /* el equipo */
  lz_texto(F(15, 1), ix, cy, "El equipo", C_TEXTO, 255);
  cy += pt(28);
  snprintf(b, sizeof b, "Lee: %s", E.seres[M.lector].nombre);
  lz_texto(F(14, 0), ix, cy, b, C_TRIBU[0], 255);
  cy += pt(26);
  for (int i = 0; i < 3 && M.fase > FASE_CALCULAR; i++) {
    int correcta = M.propuesta[i] == M.respuesta;
    snprintf(b, sizeof b, "%s calcula %ld", E.seres[M.calc[i]].nombre, M.propuesta[i]);
    lz_texto(F(14, 0), ix, cy, b, C_TRIBU[1], 255);
    int bx = ix + pt(260);
    for (int v = 0; v < 2 && M.fase > FASE_VERIFICAR; v++) {
      const char* marca = M.aprobada[i][v] == 1 ? "✓" : "✗";
      lz_texto(F(15, 1), bx + v * pt(26), cy, marca, M.aprobada[i][v] == 1 ? C_BIEN : C_MAL, 255);
    }
    (void)correcta;
    cy += pt(24);
  }
  if (M.fase > FASE_VERIFICAR) {
    snprintf(b, sizeof b, "Comprueban: %s y %s", E.seres[M.verif[0]].nombre, E.seres[M.verif[1]].nombre);
    lz_texto(F(14, 0), ix, cy, b, C_TRIBU[2], 255);
    cy += pt(26);
  }

  /* ponles un problema */
  int fy = y + h - pt(64);
  lz_texto(F(13, 0), ix, fy - pt(24), "Ponles tú un problema:", C_TENUE, 255);
  campo(&c_mision, ix, fy, iw - pt(120), pt(46), "37*12   ·   144/12   ·   2^10");
  if (boton(ix + iw - pt(110), fy, pt(110), pt(46), "Enviar", 1) || c_mision.enviado) {
    c_mision.enviado = 0;
    if (c_mision.txt[0]) {
      Texto out;
      tx_iniciar(&out);
      misiones_problema_humano(c_mision.txt, &out);
      snprintf(aviso, sizeof aviso, "%s", out.p ? out.p : "");
      aviso_hasta = reloj + 4000;
      tx_liberar(&out);
      c_mision.txt[0] = 0;
    }
  }

  /* celebración */
  if (E.tick < M.celebrar_hasta) {
    int bw = iw, bh = pt(80), bx = ix, by = y + h - pt(200); /* encima del campo, sin tapar el problema */
    lz_rrect((float)bx, (float)by, (float)bw, (float)bh, (float)pt(16), C_ORO, 240);
    snprintf(b, sizeof b, "★ ¡Nivel %d! %s", M.nivel, nivel_nombre(M.nivel));
    texto_centrado(F(26, 1), bx, by + pt(12), bw, b, 0x1A1400, 255);
    texto_centrado(F(14, 0), bx, by + pt(48), bw, M.nlogros ? M.logros[M.nlogros - 1] : "", 0x1A1400, 255);
  }

  /* columna derecha: habilidad, historial, logros */
  int dx = x + ia + pt(14), dw = w - ia - pt(14);
  tarjeta(dx, y, dw, h);
  int jx = dx + pt(18), jw = dw - pt(36);
  cy = y + pt(16);
  titulo(jx, cy, "Lo que saben de matemáticas");
  cy += pt(34);
  for (int o = 0; o < NOPS; o++) {
    float m = 0;
    for (int s = 0; s < NUM_SERES; s++) m += M.habilidad[s][o];
    m /= NUM_SERES;
    lz_texto(F(13, 0), jx, cy, op_nombre(o), C_TENUE, 255);
    barra_progreso(jx + pt(120), cy + pt(5), jw - pt(170), pt(8), m, m > 0.8f ? C_BIEN : m > 0.5f ? C_ACENTO : C_ORO);
    snprintf(b, sizeof b, "%d%%", (int)(m * 100));
    lz_texto(F(13, 1), jx + jw - pt(40), cy, b, C_TEXTO, 255);
    cy += pt(24);
  }
  cy += pt(12);
  titulo(jx, cy, "Últimos problemas");
  snprintf(b, sizeof b, "%lu bien · %lu mal", M.resueltos, M.fallados);
  lz_texto(F(13, 0), jx + jw - lz_ancho(F(13, 0), b), cy + pt(3), b, C_TENUE, 255);
  cy += pt(32);
  for (int i = M.nhist - 1; i >= 0 && i >= M.nhist - 6; i--) {
    lz_texto_max(F(14, 0), jx, cy, jw, M.hist[i].texto, M.hist[i].bien ? C_BIEN : C_MAL, 255);
    cy += pt(24);
  }
  cy += pt(12);
  titulo(jx, cy, "Logros");
  cy += pt(32);
  lz_recorte(dx, cy, dw, y + h - cy - pt(10));
  for (int i = M.nlogros - 1; i >= 0; i--) {
    lz_texto(F(15, 1), jx, cy, "★", C_ORO, 255);
    cy += lz_parrafo(F(13, 0), jx + pt(24), cy + pt(1), jw - pt(24), M.logros[i], C_TEXTO, 255, 2, 1) + pt(8);
  }
  lz_sin_recorte();
}

/* ---------- pantalla: Visión ---------- */
typedef struct {
  char nombre[256];
  int w, h;
  uint8_t* rgb;
} Miniatura;
static Miniatura minis[12];
static int nminis = 0;
static unsigned long minis_de = (unsigned long)-1;

static int por_nombre(const void* a, const void* b) { return strcmp(((const Entrada*)a)->nombre, ((const Entrada*)b)->nombre); }

static void cargar_miniaturas(void) {
  if (minis_de == V.vistas) return;
  minis_de = V.vistas;
  for (int i = 0; i < nminis; i++) free(minis[i].rgb);
  nminis = 0;
  char r[RUTA_MAX];
  ruta_unir(r, RUTA_MAX, E.mundo, "fotos/vistas");
  Entrada* e;
  int n = arch_listar(r, &e), k = 0;
  Entrada* ppm = xmalloc(sizeof(Entrada) * (size_t)(n ? n : 1));
  for (int i = 0; i < n; i++)
    if (strstr(e[i].nombre, ".ppm")) ppm[k++] = e[i];
  qsort(ppm, (size_t)k, sizeof(Entrada), por_nombre);
  for (int i = k - 1; i >= 0 && nminis < 12; i--) {
    char ruta[RUTA_MAX];
    ruta_unir(ruta, RUTA_MAX, r, ppm[i].nombre);
    Miniatura* m = &minis[nminis];
    if (ppm_leer(ruta, &m->w, &m->h, &m->rgb)) {
      snprintf(m->nombre, sizeof m->nombre, "%s", ppm[i].nombre);
      nminis++;
    }
  }
  free(ppm);
  free(e);
}

static void imagen_encuadrada(const uint8_t* rgb, int iw, int ih, int x, int y, int w, int h, int suave) {
  lz_rrect((float)x, (float)y, (float)w, (float)h, (float)pt(10), 0x05060A, 255);
  if (!rgb) return;
  int dw = w, dh = ih * w / iw;
  if (dh > h) dh = h, dw = iw * h / ih;
  lz_imagen(rgb, iw, ih, x + (w - dw) / 2, y + (h - dh) / 2, dw, dh, suave);
}

static void pantalla_vision(int x, int y, int w, int h) {
  cargar_miniaturas();
  int ia = w * 64 / 100;
  tarjeta(x, y, ia, h);
  int ix = x + pt(18), iw = ia - pt(36), cy = y + pt(16);
  char b[700];
  int aw, ah;
  agudeza_de_nivel(M.nivel, &aw, &ah);
  titulo(ix, cy, V.activa ? "Lo que están mirando" : "La última foto que vieron");
  if (V.activa) {
    snprintf(b, sizeof b, "%s · paso: %s (%s)", vision_nombre_bonito(V.nombre), paso_vision_nombre(V.paso), E.seres[V.ser_paso].nombre);
    lz_texto_max(F(13, 0), ix, cy + pt(26), iw, b, C_TENUE, 255);
  }
  cy += pt(52);
  int ancho_img = (iw - pt(14)) / 2, alto_img = ancho_img * 3 / 4;
  lz_texto(F(13, 1), ix, cy, "La foto", C_TENUE, 255);
  snprintf(b, sizeof b, "Lo que ven (%d×%d)", V.vista ? V.aw : aw, V.vista ? V.ah : ah);
  lz_texto(F(13, 1), ix + ancho_img + pt(14), cy, b, C_TENUE, 255);
  cy += pt(22);
  if (V.rgb || V.vista) {
    imagen_encuadrada(V.rgb, V.w, V.h, ix, cy, ancho_img, alto_img, 1);
    imagen_encuadrada(V.vista, V.aw, V.ah, ix + ancho_img + pt(14), cy, ancho_img, alto_img, 0);
  } else {
    lz_rrect((float)ix, (float)cy, (float)iw, (float)alto_img, (float)pt(12), C_PANEL2, 255);
    texto_centrado(F(17, 1), ix, cy + alto_img / 2 - pt(24), iw, "Todavía no han visto ninguna foto", C_TEXTO, 255);
    texto_centrado(F(14, 0), ix, cy + alto_img / 2 + pt(6), iw, "Toca «Añadir fotos» y elige las que quieras, aunque sean cien.", C_TENUE, 255);
  }
  cy += alto_img + pt(16);
  /* los pasos */
  int paso_w = iw / V_PASOS;
  for (int p = 0; p < V_PASOS; p++) {
    int px = ix + p * paso_w + paso_w / 2, actual = V.activa && p == V.paso, hecho = V.activa && p < V.paso;
    if (p + 1 < V_PASOS) lz_linea((float)px, (float)cy + pt(9), (float)px + paso_w, (float)cy + pt(9), (float)pt(2), hecho ? C_ACENTO : C_BORDE, 255);
    lz_circulo((float)px, (float)cy + pt(9), (float)pt(actual ? 9 : 6), actual ? C_ACENTO : hecho ? 0x4A5A9A : C_PANEL2, 255);
    texto_centrado(F(12, actual), px - paso_w / 2, cy + pt(24), paso_w, paso_vision_nombre(p), actual ? C_TEXTO : C_TENUE, 255);
  }
  cy += pt(52);
  /* colores reales */
  if (V.npaleta) {
    int sw = (iw - pt(10) * (V.npaleta - 1)) / V.npaleta;
    for (int i = 0; i < V.npaleta; i++) {
      Tono* t = &V.paleta[i];
      int sx = ix + i * (sw + pt(10));
      lz_rrect((float)sx, (float)cy, (float)sw, (float)pt(40), (float)pt(8), (uint32_t)t->r << 16 | (uint32_t)t->g << 8 | t->b, 255);
      snprintf(b, sizeof b, "%s %d%%", t->nombre, (int)(t->frac * 100 + 0.5f));
      lz_texto_max(F(12, 0), sx, cy + pt(46), sw, b, C_TEXTO, 255);
    }
    cy += pt(72);
    const char* et[3] = {"brillo", "contraste", "bordes"};
    float vals[3] = {V.brillo, V.contraste, V.bordes * 3};
    int mw = (iw - pt(20)) / 3;
    for (int i = 0; i < 3; i++) {
      lz_texto(F(12, 0), ix + i * (mw + pt(10)), cy, et[i], C_TENUE, 255);
      barra_progreso(ix + i * (mw + pt(10)), cy + pt(20), mw, pt(8), vals[i], C_ACENTO);
    }
    cy += pt(40);
  }
  if (V.descripcion[0]) {
    cy += lz_parrafo(F(14, 0), ix, cy, iw, V.descripcion, C_TEXTO, 255, 4, 1) + pt(6);
    snprintf(b, sizeof b, "En Abla: %s", V.abla);
    lz_parrafo(F(14, 1), ix, cy, iw, b, C_TRIBU[0], 255, 2, 1);
  }

  /* columna derecha: la cola y las fotos vistas */
  int dx = x + ia + pt(14), dw = w - ia - pt(14);
  tarjeta(dx, y, dw, h);
  int jx = dx + pt(18), jw = dw - pt(36);
  cy = y + pt(16);
  titulo(jx, cy, "Fotos");
  cy += pt(36);
  char n1[32], n2[32];
  miles(n1, sizeof n1, (unsigned long long)V.en_cola);
  miles(n2, sizeof n2, V.vistas);
  const char* et2[2] = {"en cola", "vistas"};
  const char* vv[2] = {n1, n2};
  int cw = (jw - pt(10)) / 2;
  for (int i = 0; i < 2; i++) {
    lz_rrect((float)jx + i * (cw + pt(10)), (float)cy, (float)cw, (float)pt(64), (float)pt(10), C_PANEL2, 255);
    lz_texto(F(26, 1), jx + i * (cw + pt(10)) + pt(12), cy + pt(8), vv[i], C_TEXTO, 255);
    lz_texto(F(12, 0), jx + i * (cw + pt(10)) + pt(12), cy + pt(42), et2[i], C_TENUE, 255);
  }
  cy += pt(76);
  if (V.en_cola && V.ms_por_foto > 0) {
    double seg = V.en_cola * V.ms_por_foto / 1000.0;
    if (seg < 90) snprintf(b, sizeof b, "Terminarán en unos %.0f segundos", seg);
    else snprintf(b, sizeof b, "Terminarán en unos %.0f minutos", seg / 60.0);
  } else {
    snprintf(b, sizeof b, V.en_cola ? "Calculando cuánto tardarán…" : "No hay fotos esperando");
  }
  lz_texto(F(14, 0), jx, cy, b, C_TEXTO, 255);
  cy += pt(30);
  if (boton(jx, cy, jw, pt(48), "Añadir fotos", 1)) peticiones |= 2;
  cy += pt(58);
  if (boton(jx, cy, jw, pt(44), camara_encendida ? "Apagar la cámara" : "Mirar con la cámara", 0)) peticiones |= 4;
  cy += pt(58);
  titulo(jx, cy, "Ya vistas");
  cy += pt(32);
  int cols = 3, tw = (jw - pt(8) * (cols - 1)) / cols, th = tw * 3 / 4;
  lz_recorte(dx, cy, dw, y + h - cy - pt(10));
  for (int i = 0; i < nminis; i++) {
    int tx = jx + (i % cols) * (tw + pt(8)), ty = cy + (i / cols) * (th + pt(8));
    imagen_encuadrada(minis[i].rgb, minis[i].w, minis[i].h, tx, ty, tw, th, 1);
  }
  if (!nminis) lz_texto(F(13, 0), jx, cy, "Aquí aparecerán las fotos que vayan mirando.", C_TENUE, 255);
  lz_sin_recorte();
}

/* ---------- pantalla: Hablar ---------- */
#define MAX_CHARLA 80
static struct {
  char* texto;
  int mio;
} charla[MAX_CHARLA];
static int ncharla = 0, destino = 18; /* -1 = toda la especie */
static Desplazable d_charla = {0, 0, 1};

static void decir_en_charla(const char* t, int mio) {
  if (ncharla == MAX_CHARLA) {
    free(charla[0].texto);
    memmove(charla, charla + 1, sizeof charla[0] * (MAX_CHARLA - 1));
    ncharla--;
  }
  charla[ncharla].texto = xstrdup(t);
  charla[ncharla++].mio = mio;
}

static void hablar(const char* modo, const char* texto) {
  if (!texto[0]) return;
  char linea[400], para[16];
  if (destino < 0) snprintf(para, sizeof para, "todos");
  else snprintf(para, sizeof para, "%d", destino + 1);
  snprintf(linea, sizeof linea, "%s%s  →  %s", strcmp(modo, "decir") ? "[data C] " : "", texto,
           destino < 0 ? "toda la especie" : E.seres[destino].nombre);
  decir_en_charla(linea, 1);
  snprintf(linea, sizeof linea, "%s %s %s", modo, para, texto);
  const char* r = abla_ejecutar(linea);
  char limpio[1200];
  snprintf(limpio, sizeof limpio, "%s", r);
  size_t n = strlen(limpio);
  while (n && limpio[n - 1] == '\n') limpio[--n] = 0;
  decir_en_charla(limpio, 0);
}

static void pantalla_hablar(int x, int y, int w, int h) {
  tarjeta(x, y, w, h);
  int ix = x + pt(18), iw = w - pt(36), cy = y + pt(14);
  titulo(ix, cy + pt(8), "Hablar con");
  int sx = ix + pt(120);
  if (boton(sx, cy, pt(44), pt(40), "‹", 0)) destino = destino <= -1 ? NUM_SERES - 1 : destino - 1;
  char b[80];
  if (destino < 0) snprintf(b, sizeof b, "toda la especie");
  else snprintf(b, sizeof b, "%d. %s (%s)", destino + 1, E.seres[destino].nombre,
                E.seres[destino].tribu == 0 ? "lenguaje" : E.seres[destino].tribu == 1 ? "datos" : "lógica");
  lz_rrect((float)sx + pt(50), (float)cy, (float)pt(260), (float)pt(40), (float)pt(10), C_PANEL2, 255);
  texto_centrado(F(15, 1), sx + pt(50), cy + pt(9), pt(260), b, destino < 0 ? C_TEXTO : C_TRIBU[E.seres[destino].tribu], 255);
  if (boton(sx + pt(316), cy, pt(44), pt(40), "›", 0)) destino = destino >= NUM_SERES - 1 ? -1 : destino + 1;
  cy += pt(56);
  /* la charla */
  int zh = h - pt(56) - pt(150);
  desplazar(&d_charla, x, cy, w, zh);
  lz_recorte(x + pt(8), cy, w - pt(16), zh);
  int by = cy - (int)d_charla.desp, total = 0;
  const Fuente* f = F(15, 0);
  if (!ncharla) {
    lz_parrafo(F(15, 0), ix, cy + pt(10), iw,
               "Escríbeles con raíces en español o en Abla. Para enseñarles un hecho: sujeto relación objeto, con relación "
               "es, parte, causa, igual u opuesto. Añade «pregunta» al final para preguntar.",
               C_TENUE, 255, 0, 1);
  }
  for (int i = 0; i < ncharla; i++) {
    int maxw = iw * 3 / 4, ph = lz_parrafo(f, 0, 0, maxw - pt(24), charla[i].texto, 0, 0, 0, 0);
    int bw = maxw;
    if (ph <= lz_alto_linea(f)) {
      int a = lz_ancho(f, charla[i].texto) + pt(26);
      if (a < bw) bw = a;
    }
    int bx = charla[i].mio ? ix + iw - bw : ix, bh = ph + pt(18);
    lz_rrect((float)bx, (float)by, (float)bw, (float)bh, (float)pt(14), charla[i].mio ? 0x243058 : C_PANEL2, 255);
    lz_parrafo(f, bx + pt(12), by + pt(9), bw - pt(24), charla[i].texto, C_TEXTO, 255, 0, 1);
    by += bh + pt(10);
    total += bh + pt(10);
  }
  d_charla.contenido = (float)total;
  lz_sin_recorte();
  /* escribir */
  int fy = y + h - pt(140);
  campo(&c_hablar, ix, fy, iw - pt(260), pt(48), "sol causa luz pregunta");
  int decir = boton(ix + iw - pt(250), fy, pt(120), pt(48), "Decir", 1);
  int ensenar = boton(ix + iw - pt(122), fy, pt(122), pt(48), "Enseñar (C)", 0);
  if ((decir || c_hablar.enviado || ensenar) && c_hablar.txt[0]) {
    hablar(ensenar ? "enseñar" : "decir", c_hablar.txt);
    c_hablar.txt[0] = 0;
  }
  c_hablar.enviado = 0;
  /* ejemplos */
  static const char* EJ[][2] = {{"decir", "sol causa luz pregunta"}, {"decir", "agua parte mar pregunta"},
                                {"enseñar", "fuego causa caliente"}, {"enseñar", "luna parte cielo"},
                                {"decir", "hola mente"}};
  int ex = ix, ey = fy + pt(62);
  for (int i = 0; i < 5; i++) {
    char t[80];
    snprintf(t, sizeof t, "%s %s", EJ[i][0][0] == 'd' ? "›" : "•", EJ[i][1]);
    int ew = lz_ancho(F(15, 0), t) + pt(24);
    if (ex + ew > ix + iw) break;
    if (boton(ex, ey, ew, pt(40), t, 2)) hablar(EJ[i][0], EJ[i][1]);
    ex += ew + pt(8);
  }
}

/* ---------- pantalla: Terminal ---------- */
static Texto salida_terminal;
static Desplazable d_terminal = {0, 0, 1};
static char historial[30][300];
static int nhist = 0, poshist = 0;

static void pantalla_terminal(int x, int y, int w, int h) {
  tarjeta(x, y, w, h);
  int ix = x + pt(18), iw = w - pt(36), zy = y + pt(14), zh = h - pt(90);
  if (!salida_terminal.n) tx_add(&salida_terminal, "Especie Abla · cerebro en C\nEscribe ayuda para ver los comandos.\n\n");
  desplazar(&d_terminal, x, zy, w, zh);
  lz_recorte(x + pt(8), zy, w - pt(16), zh);
  int alto = lz_parrafo(F(14, 0), ix, zy - (int)d_terminal.desp, iw, salida_terminal.p, 0xC9CDE0, 255, 0, 1);
  d_terminal.contenido = (float)alto;
  lz_sin_recorte();
  int fy = y + h - pt(66);
  const char* pr = abla_prompt();
  int pw = lz_texto(F(14, 1), ix, fy + pt(13), pr, C_TRIBU[2], 255);
  campo(&c_terminal, ix + pw + pt(8), fy, iw - pw - pt(8), pt(48), "ayuda");
  if (c_terminal.enviado) {
    c_terminal.enviado = 0;
    if (!strcmp(c_terminal.txt, "clear")) tx_vaciar(&salida_terminal);
    else {
      tx_printf(&salida_terminal, "%s%s\n", abla_prompt(), c_terminal.txt);
      tx_add(&salida_terminal, abla_ejecutar(c_terminal.txt));
      if (salida_terminal.n > 40000) {
        size_t quitar = salida_terminal.n - 30000;
        memmove(salida_terminal.p, salida_terminal.p + quitar, salida_terminal.n - quitar + 1);
        salida_terminal.n -= quitar;
      }
    }
    if (c_terminal.txt[0]) {
      if (nhist == 30) memmove(historial, historial + 1, sizeof historial[0] * 29), nhist--;
      snprintf(historial[nhist++], sizeof historial[0], "%s", c_terminal.txt);
    }
    poshist = nhist;
    c_terminal.txt[0] = 0;
    d_terminal.desp = 1e9f;
  }
}

/* ---------- barras ---------- */
static void barra_superior(int w) {
  int h = pt(64);
  lz_rect(0, 0, w, h, C_FONDO, 255);
  lz_texto(F(22, 1), pt(20), pt(10), "Especie Abla", C_TEXTO, 255);
  lz_texto(F(12, 0), pt(20), pt(40), "27 mentes · cerebro en C · todo en este iPad", C_TENUE, 255);
  /* velocidad */
  int vx = w - pt(196);
  if (boton(vx, pt(12), pt(40), pt(40), "−", 0)) abla_fijar_ritmo(abla_ritmo() < 2000 ? abla_ritmo() * 3 / 2 : 3000);
  char b[64];
  snprintf(b, sizeof b, "%.2f s", abla_ritmo() / 1000.0);
  for (char* p = b; *p; p++)
    if (*p == '.') *p = ',';
  lz_rrect((float)vx + pt(46), (float)pt(12), (float)pt(100), (float)pt(40), (float)pt(10), C_PANEL, 255);
  texto_centrado(F(15, 1), vx + pt(46), pt(16), pt(100), b, C_ORO, 255);
  texto_centrado(F(10, 0), vx + pt(46), pt(36), pt(100), "por ciclo", C_TENUE, 255);
  if (boton(vx + pt(152), pt(12), pt(40), pt(40), "+", 0)) abla_fijar_ritmo(abla_ritmo() * 2 / 3 > 50 ? abla_ritmo() * 2 / 3 : 50);
  /* cifras */
  char v[4][48];
  const char* k[4] = {"ciclo", "palabras", "nivel", "fotos en cola"};
  miles(v[0], 48, E.tick);
  miles(v[1], 48, (unsigned long long)idioma_tamano());
  snprintf(v[2], 48, "%d", M.nivel);
  miles(v[3], 48, (unsigned long long)V.en_cola);
  int cx = vx - pt(16);
  for (int i = 3; i >= 0; i--) {
    int aw = lz_ancho(F(17, 1), v[i]), ak = lz_ancho(F(11, 0), k[i]), cw = (aw > ak ? aw : ak);
    cx -= cw;
    if (cx < pt(330)) break;
    lz_texto(F(17, 1), cx + cw - aw, pt(12), v[i], C_TEXTO, 255);
    lz_texto(F(11, 0), cx + cw - ak, pt(38), k[i], C_TENUE, 255);
    cx -= pt(24);
  }
}

static void dibujar_icono(int i, int cx, int cy, uint32_t c) {
  switch (i) {
    case T_ESPECIE:
      lz_linea((float)cx - pt(8), (float)cy + pt(6), (float)cx, (float)cy - pt(7), (float)pt(2), c, 255);
      lz_linea((float)cx, (float)cy - pt(7), (float)cx + pt(8), (float)cy + pt(6), (float)pt(2), c, 255);
      lz_linea((float)cx - pt(8), (float)cy + pt(6), (float)cx + pt(8), (float)cy + pt(6), (float)pt(2), c, 255);
      lz_circulo((float)cx, (float)cy - pt(7), (float)pt(4), c, 255);
      lz_circulo((float)cx - pt(8), (float)cy + pt(6), (float)pt(4), c, 255);
      lz_circulo((float)cx + pt(8), (float)cy + pt(6), (float)pt(4), c, 255);
      break;
    case T_MISIONES: texto_centrado(F(22, 1), cx - pt(20), cy - pt(14), pt(40), "★", c, 255); break;
    case T_VISION:
      lz_circulo((float)cx, (float)cy, (float)pt(10), c, 255);
      lz_circulo((float)cx, (float)cy, (float)pt(7), C_FONDO, 255);
      lz_circulo((float)cx, (float)cy, (float)pt(4), c, 255);
      break;
    case T_HABLAR:
      lz_rrect((float)cx - pt(11), (float)cy - pt(9), (float)pt(22), (float)pt(16), (float)pt(6), c, 255);
      lz_linea((float)cx - pt(5), (float)cy + pt(6), (float)cx - pt(8), (float)cy + pt(11), (float)pt(3), c, 255);
      break;
    default: texto_centrado(F(15, 1), cx - pt(20), cy - pt(10), pt(40), ">_", c, 255); break;
  }
}

static void barra_pestanas(int w, int h) {
  int bh = pt(72), y = h - bh;
  lz_rect(0, y, w, bh, 0x10131D, 255);
  lz_rect(0, y, w, pt(1), C_BORDE, 255);
  static const char* NOMBRES[NTABS] = {"Especie", "Misiones", "Visión", "Hablar", "Terminal"};
  int tw = w / NTABS;
  for (int i = 0; i < NTABS; i++) {
    int tx = i * tw;
    uint32_t c = i == pestana ? C_ACENTO : C_TENUE;
    dibujar_icono(i, tx + tw / 2, y + pt(26), c);
    texto_centrado(F(12, i == pestana), tx, y + pt(44), tw, NOMBRES[i], c, 255);
    if (toque(tx, y, tw, bh)) {
      pestana = i;
      foco = NULL;
    }
  }
}

/* ---------- lo que se exporta a la página ---------- */
EXPORTA("abla_gui_cuadro") uint32_t* abla_gui_cuadro(int w, int h, double escala, double ahora) {
  float dt = reloj > 0 ? (float)(ahora - reloj) : 16;
  if (dt > 500) dt = 500;
  reloj = ahora;
  S = (float)escala;
  P.pulsado = P.p_pulsado, P.soltado = P.p_soltado, P.rueda = P.p_rueda;
  P.p_pulsado = P.p_soltado = 0, P.p_rueda = 0;
  if (P.abajo && !P.arrastre && ((P.x - P.x0) * (P.x - P.x0) + (P.y - P.y0) * (P.y - P.y0)) > pt(10) * pt(10)) P.arrastre = 1;

  lz_tamano(w, h);
  lz_rect(0, 0, w, h, C_FONDO, 255);
  animar(dt);
  barra_superior(w);
  int m = pt(16), cy = pt(70), ch = h - pt(72) - cy - pt(12);
  switch (pestana) {
    case T_ESPECIE: pantalla_especie(m, cy, w - 2 * m, ch); break;
    case T_MISIONES: pantalla_misiones(m, cy, w - 2 * m, ch); break;
    case T_VISION: pantalla_vision(m, cy, w - 2 * m, ch); break;
    case T_HABLAR: pantalla_hablar(m, cy, w - 2 * m, ch); break;
    default: pantalla_terminal(m, cy, w - 2 * m, ch); break;
  }
  barra_pestanas(w, h);
  if (aviso[0] && reloj < aviso_hasta) {
    const Fuente* f = F(15, 0);
    int aw = lz_ancho(f, aviso) + pt(40);
    if (aw > w - pt(40)) aw = w - pt(40);
    int ax = (w - aw) / 2, ay = h - pt(72) - pt(70);
    lz_rrect((float)ax, (float)ay, (float)aw, (float)pt(48), (float)pt(24), 0x2A3150, 245);
    lz_texto_max(f, ax + pt(20), ay + pt(13), aw - pt(40), aviso, C_TEXTO, 255);
  }
  if (P.soltado) P.arrastre = 0;
  P.yprev = P.y;
  return L.px;
}

/* tipo: 0 apoyar el dedo, 1 moverlo, 2 levantarlo (coordenadas en píxeles del cuadro) */
EXPORTA("abla_gui_puntero") void abla_gui_puntero(int tipo, double x, double y) {
  P.x = (float)x, P.y = (float)y;
  if (tipo == 0) {
    P.abajo = 1, P.p_pulsado = 1, P.x0 = P.x, P.y0 = P.y, P.yprev = P.y, P.arrastre = 0;
  } else if (tipo == 2) {
    P.abajo = 0, P.p_soltado = 1;
  }
}

EXPORTA("abla_gui_rueda") void abla_gui_rueda(double dy) { P.p_rueda += (float)dy; }

/* Teclas como en una terminal: texto normal, \r intro, \x7f borrar, \x1b[A flechas, \x1b escape. */
EXPORTA("abla_gui_teclas") void abla_gui_teclas(const char* s) {
  if (!foco) foco = pestana == T_HABLAR ? &c_hablar : pestana == T_MISIONES ? &c_mision : pestana == T_TERMINAL ? &c_terminal : NULL;
  if (!foco) return;
  size_t n = strlen(foco->txt);
  while (*s) {
    unsigned char k = (unsigned char)*s;
    if (k == 27) {
      if (s[1] == '[' && s[2]) {
        if (foco == &c_terminal && (s[2] == 'A' || s[2] == 'B') && nhist) {
          poshist += s[2] == 'A' ? -1 : 1;
          if (poshist < 0) poshist = 0;
          if (poshist >= nhist) poshist = nhist, foco->txt[0] = 0;
          else snprintf(foco->txt, sizeof foco->txt, "%s", historial[poshist]);
          n = strlen(foco->txt);
        }
        s += 3;
        continue;
      }
      foco->txt[0] = 0, n = 0;
    } else if (k == '\r' || k == '\n') {
      foco->enviado = 1;
    } else if (k == 127 || k == 8) {
      while (n > 0) {
        unsigned char c = (unsigned char)foco->txt[--n];
        if ((c & 0xC0) != 0x80) break;
      }
      foco->txt[n] = 0;
    } else if (k >= 32 && n + 1 < sizeof foco->txt) {
      foco->txt[n++] = (char)k;
      foco->txt[n] = 0;
    }
    s++;
  }
}

/* Lo que la interfaz le pide a la página: 1 teclado, 2 elegir fotos, 4 encender/apagar la cámara. */
EXPORTA("abla_gui_peticiones") int abla_gui_peticiones(void) {
  int p = (int)peticiones;
  peticiones = 0;
  return p;
}

EXPORTA("abla_gui_aviso") void abla_gui_aviso(const char* s) {
  snprintf(aviso, sizeof aviso, "%s", s);
  aviso_hasta = reloj + 5000;
}

EXPORTA("abla_gui_camara") void abla_gui_camara(int encendida) { camara_encendida = encendida; }
