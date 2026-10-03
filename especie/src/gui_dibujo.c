/* Rasterizador sencillo para la interfaz: rectángulos con esquinas redondas,
 * círculos y líneas con bordes suaves (antialiasing), imágenes y texto.
 * Todo en C, sobre una imagen RGBA en memoria. */
#include "comun.h"
#include "gui.h"

Lienzo L;

void lz_tamano(int w, int h) {
  if (w != L.w || h != L.h || !L.px) {
    free(L.px);
    L.px = xmalloc(sizeof(uint32_t) * (size_t)w * (size_t)h);
    L.w = w, L.h = h;
  }
  lz_sin_recorte();
}

void lz_recorte(int x, int y, int w, int h) {
  L.cx0 = x < 0 ? 0 : x;
  L.cy0 = y < 0 ? 0 : y;
  L.cx1 = x + w > L.w ? L.w : x + w;
  L.cy1 = y + h > L.h ? L.h : y + h;
}
void lz_sin_recorte(void) { lz_recorte(0, 0, L.w, L.h); }

/* mezcla un color 0xRRGGBB con opacidad a (0-255) sobre el píxel */
static inline void mezclar(int x, int y, uint32_t rgb, int a) {
  if (x < L.cx0 || y < L.cy0 || x >= L.cx1 || y >= L.cy1 || a <= 0) return;
  uint32_t* d = &L.px[(size_t)y * (size_t)L.w + (size_t)x];
  uint32_t r = rgb >> 16 & 255, g = rgb >> 8 & 255, b = rgb & 255;
  if (a >= 255) {
    *d = 0xFF000000u | b << 16 | g << 8 | r;
    return;
  }
  uint32_t v = *d, dr = v & 255, dg = v >> 8 & 255, db = v >> 16 & 255;
  dr = (r * (uint32_t)a + dr * (uint32_t)(255 - a)) / 255;
  dg = (g * (uint32_t)a + dg * (uint32_t)(255 - a)) / 255;
  db = (b * (uint32_t)a + db * (uint32_t)(255 - a)) / 255;
  *d = 0xFF000000u | db << 16 | dg << 8 | dr;
}

void lz_rect(int x, int y, int w, int h, uint32_t rgb, int a) {
  int x0 = x < L.cx0 ? L.cx0 : x, y0 = y < L.cy0 ? L.cy0 : y;
  int x1 = x + w > L.cx1 ? L.cx1 : x + w, y1 = y + h > L.cy1 ? L.cy1 : y + h;
  if (a >= 255) {
    uint32_t c = 0xFF000000u | (rgb & 255) << 16 | (rgb >> 8 & 255) << 8 | (rgb >> 16 & 255);
    for (int yy = y0; yy < y1; yy++) {
      uint32_t* p = &L.px[(size_t)yy * (size_t)L.w];
      for (int xx = x0; xx < x1; xx++) p[xx] = c;
    }
    return;
  }
  for (int yy = y0; yy < y1; yy++)
    for (int xx = x0; xx < x1; xx++) mezclar(xx, yy, rgb, a);
}

static float raiz(float v) { /* raíz cuadrada sin libm (Newton) */
  if (v <= 0) return 0;
  float r = v > 1 ? v / 2 : 1;
  for (int i = 0; i < 12; i++) r = 0.5f * (r + v / r);
  return r;
}

static float cobertura(float d) { /* d: distancia firmada (negativa dentro) */
  float c = 0.5f - d;
  return c < 0 ? 0 : c > 1 ? 1 : c;
}

/* distancia firmada a un rectángulo de esquinas redondas */
static float dist_rrect(float px, float py, float x, float y, float w, float h, float r) {
  float cx = x + w / 2, cy = y + h / 2;
  float qx = (px > cx ? px - cx : cx - px) - (w / 2 - r), qy = (py > cy ? py - cy : cy - py) - (h / 2 - r);
  float ax = qx > 0 ? qx : 0, ay = qy > 0 ? qy : 0;
  float fuera = raiz(ax * ax + ay * ay), dentro = qx > qy ? qx : qy;
  return fuera + (dentro < 0 ? dentro : 0) - r;
}

void lz_rrect(float x, float y, float w, float h, float r, uint32_t rgb, int a) {
  if (r < 0.5f) {
    lz_rect((int)x, (int)y, (int)w, (int)h, rgb, a);
    return;
  }
  if (r > w / 2) r = w / 2;
  if (r > h / 2) r = h / 2;
  int ri = (int)r + 1;
  /* el centro es macizo: solo las esquinas necesitan suavizado */
  lz_rect((int)x + ri, (int)y, (int)w - 2 * ri, (int)h, rgb, a);
  lz_rect((int)x, (int)y + ri, ri, (int)h - 2 * ri, rgb, a);
  lz_rect((int)(x + w) - ri, (int)y + ri, ri, (int)h - 2 * ri, rgb, a);
  int xs[2] = {(int)x, (int)(x + w) - ri}, ys[2] = {(int)y, (int)(y + h) - ri};
  for (int k = 0; k < 4; k++)
    for (int yy = ys[k / 2]; yy < ys[k / 2] + ri; yy++)
      for (int xx = xs[k % 2]; xx < xs[k % 2] + ri; xx++) {
        float c = cobertura(dist_rrect(xx + 0.5f, yy + 0.5f, x, y, w, h, r));
        if (c > 0) mezclar(xx, yy, rgb, (int)(a * c));
      }
}

void lz_rrect_borde(float x, float y, float w, float h, float r, float g, uint32_t rgb, int a) {
  int x0 = (int)x - 1, x1 = (int)(x + w) + 1, y0 = (int)y - 1, y1 = (int)(y + h) + 1;
  int m = (int)(r + g) + 2, gi = (int)g + 2;
  for (int yy = y0; yy <= y1; yy++) {
    int recto = yy > y0 + m && yy < y1 - m; /* en los lados rectos, el interior se salta */
    for (int xx = x0; xx <= x1; xx++) {
      if (recto && xx > x0 + gi && xx < x1 - gi) {
        xx = x1 - gi - 1;
        continue;
      }
      float d = dist_rrect(xx + 0.5f, yy + 0.5f, x, y, w, h, r);
      float c = cobertura(d) - cobertura(d + g);
      if (c > 0) mezclar(xx, yy, rgb, (int)(a * c));
    }
  }
}

void lz_circulo(float cx, float cy, float r, uint32_t rgb, int a) {
  int x0 = (int)(cx - r - 1), x1 = (int)(cx + r + 1), y0 = (int)(cy - r - 1), y1 = (int)(cy + r + 1);
  for (int y = y0; y <= y1; y++)
    for (int x = x0; x <= x1; x++) {
      float dx = x + 0.5f - cx, dy = y + 0.5f - cy;
      float c = cobertura(raiz(dx * dx + dy * dy) - r);
      if (c > 0) mezclar(x, y, rgb, (int)(a * c));
    }
}

void lz_linea(float x0, float y0, float x1, float y1, float g, uint32_t rgb, int a) {
  float dx = x1 - x0, dy = y1 - y0, l2 = dx * dx + dy * dy;
  int bx0 = (int)((x0 < x1 ? x0 : x1) - g - 1), bx1 = (int)((x0 > x1 ? x0 : x1) + g + 1);
  int by0 = (int)((y0 < y1 ? y0 : y1) - g - 1), by1 = (int)((y0 > y1 ? y0 : y1) + g + 1);
  if (bx0 < L.cx0) bx0 = L.cx0;
  if (by0 < L.cy0) by0 = L.cy0;
  if (bx1 >= L.cx1) bx1 = L.cx1 - 1;
  if (by1 >= L.cy1) by1 = L.cy1 - 1;
  for (int y = by0; y <= by1; y++)
    for (int x = bx0; x <= bx1; x++) {
      float px = x + 0.5f - x0, py = y + 0.5f - y0;
      float t = l2 > 0 ? (px * dx + py * dy) / l2 : 0;
      t = t < 0 ? 0 : t > 1 ? 1 : t;
      float ex = px - t * dx, ey = py - t * dy;
      float c = cobertura(raiz(ex * ex + ey * ey) - g / 2);
      if (c > 0) mezclar(x, y, rgb, (int)(a * c));
    }
}

/* Dibuja una imagen RGB escalada: suave (bilineal) o con píxeles grandes (vecino). */
void lz_imagen(const uint8_t* rgb, int iw, int ih, int x, int y, int w, int h, int suave) {
  if (!rgb || iw <= 0 || ih <= 0 || w <= 0 || h <= 0) return;
  for (int yy = y < L.cy0 ? L.cy0 : y; yy < y + h && yy < L.cy1; yy++)
    for (int xx = x < L.cx0 ? L.cx0 : x; xx < x + w && xx < L.cx1; xx++) {
      uint32_t r, g, b;
      if (suave) {
        float fx = (xx - x + 0.5f) * iw / w - 0.5f, fy = (yy - y + 0.5f) * ih / h - 0.5f;
        if (fx < 0) fx = 0;
        if (fy < 0) fy = 0;
        int ix = (int)fx, iy = (int)fy;
        int ix1 = ix + 1 < iw ? ix + 1 : ix, iy1 = iy + 1 < ih ? iy + 1 : iy;
        float tx = fx - ix, ty = fy - iy;
        const uint8_t *a = rgb + ((size_t)iy * (size_t)iw + (size_t)ix) * 3, *bb = rgb + ((size_t)iy * (size_t)iw + (size_t)ix1) * 3;
        const uint8_t *c = rgb + ((size_t)iy1 * (size_t)iw + (size_t)ix) * 3, *d = rgb + ((size_t)iy1 * (size_t)iw + (size_t)ix1) * 3;
        float v[3];
        for (int k = 0; k < 3; k++) v[k] = (a[k] * (1 - tx) + bb[k] * tx) * (1 - ty) + (c[k] * (1 - tx) + d[k] * tx) * ty;
        r = (uint32_t)v[0], g = (uint32_t)v[1], b = (uint32_t)v[2];
      } else {
        int ix = (xx - x) * iw / w, iy = (yy - y) * ih / h;
        const uint8_t* p = rgb + ((size_t)iy * (size_t)iw + (size_t)ix) * 3;
        r = p[0], g = p[1], b = p[2];
      }
      L.px[(size_t)yy * (size_t)L.w + (size_t)xx] = 0xFF000000u | b << 16 | g << 8 | r;
    }
}

/* ---------- texto ---------- */
const Fuente* fuente(int px, int negrita) {
  const Fuente* mejor = &fuentes[0];
  int dmejor = 1 << 30;
  for (int i = 0; i < nfuentes; i++) {
    int d = (fuentes[i].px > px ? fuentes[i].px - px : px - fuentes[i].px) * 2 + (fuentes[i].negrita != negrita);
    if (d < dmejor) dmejor = d, mejor = &fuentes[i];
  }
  return mejor;
}

int lz_alto_linea(const Fuente* f) { return f->asc + f->desc + f->px / 6; }

static uint32_t siguiente_cp(const char** s) {
  const unsigned char* p = (const unsigned char*)*s;
  uint32_t c;
  int n;
  if (p[0] < 0x80) c = p[0], n = 1;
  else if (p[0] < 0xE0) c = (uint32_t)(p[0] & 0x1F) << 6 | (p[1] & 0x3F), n = 2;
  else if (p[0] < 0xF0) c = (uint32_t)(p[0] & 0x0F) << 12 | (uint32_t)(p[1] & 0x3F) << 6 | (p[2] & 0x3F), n = 3;
  else c = (uint32_t)(p[0] & 0x07) << 18 | (uint32_t)(p[1] & 0x3F) << 12 | (uint32_t)(p[2] & 0x3F) << 6 | (p[3] & 0x3F), n = 4;
  for (int i = 1; i < n; i++)
    if (!p[i]) n = i;
  *s += n;
  return c;
}

static const Glifo* glifo(const Fuente* f, uint32_t cp) {
  int a = 0, b = f->n - 1;
  while (a <= b) {
    int m = (a + b) / 2;
    if (f->g[m].cp == cp) return &f->g[m];
    if (f->g[m].cp < cp) a = m + 1;
    else b = m - 1;
  }
  return cp == '?' ? NULL : glifo(f, '?');
}

static int dibujar_glifo(const Fuente* f, const Glifo* g, int x, int y, uint32_t rgb, int a) {
  if (!g) return 0;
  const uint8_t* d = fuente_datos + g->off;
  for (int yy = 0; yy < g->h; yy++)
    for (int xx = 0; xx < g->w; xx++) {
      int i = yy * g->w + xx;
      int v = (d[i >> 1] >> ((i & 1) * 4)) & 15;
      if (v) mezclar(x + g->xoff + xx, y + g->yoff + yy, rgb, a * v / 15);
    }
  (void)f;
  return g->avance;
}

int lz_ancho(const Fuente* f, const char* s) {
  int w = 0;
  while (*s) {
    const Glifo* g = glifo(f, siguiente_cp(&s));
    if (g) w += g->avance;
  }
  return w;
}

int lz_texto(const Fuente* f, int x, int y, const char* s, uint32_t rgb, int a) {
  int x0 = x;
  while (*s) x += dibujar_glifo(f, glifo(f, siguiente_cp(&s)), x, y, rgb, a);
  return x - x0;
}

int lz_texto_max(const Fuente* f, int x, int y, int max, const char* s, uint32_t rgb, int a) {
  if (lz_ancho(f, s) <= max) return lz_texto(f, x, y, s, rgb, a);
  const Glifo* puntos = glifo(f, 0x2026);
  int limite = max - (puntos ? puntos->avance : 0), x0 = x;
  while (*s) {
    const Glifo* g = glifo(f, siguiente_cp(&s));
    if (!g || x - x0 + g->avance > limite) break;
    x += dibujar_glifo(f, g, x, y, rgb, a);
  }
  x += dibujar_glifo(f, puntos, x, y, rgb, a);
  return x - x0;
}

/* Párrafo partido en líneas a lo ancho. Si dibujar=0 solo mide. */
int lz_parrafo(const Fuente* f, int x, int y, int ancho, const char* s, uint32_t rgb, int a, int max_lineas, int dibujar) {
  int lineas = 0, alto = lz_alto_linea(f);
  while (*s && (max_lineas <= 0 || lineas < max_lineas)) {
    /* buscar hasta dónde cabe */
    const char *p = s, *corte = NULL, *fin = s;
    int w = 0;
    while (*p && *p != '\n') {
      const char* antes = p;
      const Glifo* g = glifo(f, siguiente_cp(&p));
      int gw = g ? g->avance : 0;
      if (w + gw > ancho && fin != s) break;
      if (*antes == ' ') corte = antes;
      w += gw;
      fin = p;
    }
    const char* hasta = (*fin && *fin != '\n' && corte && corte > s) ? corte : fin;
    if (dibujar) {
      int ultima = max_lineas > 0 && lineas == max_lineas - 1 && *hasta && *hasta != '\n';
      char buf[1024];
      size_t n = (size_t)(hasta - s) < sizeof buf - 1 ? (size_t)(hasta - s) : sizeof buf - 1;
      memcpy(buf, s, n);
      buf[n] = 0;
      if (ultima) lz_texto_max(f, x, y + lineas * alto, ancho, buf, rgb, a);
      else lz_texto(f, x, y + lineas * alto, buf, rgb, a);
    }
    lineas++;
    s = hasta;
    while (*s == ' ') s++;
    if (*s == '\n') s++;
  }
  return lineas * alto;
}
