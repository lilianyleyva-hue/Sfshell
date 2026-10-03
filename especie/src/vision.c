/* Visión de la especie.
 *
 * Las fotos llegan a mundo/fotos/entrada/ como imágenes PPM (el formato de
 * imagen más simple: los píxeles RGB tal cual). La especie las mira de una en
 * una, un paso por ciclo, repartiéndose el trabajo:
 *
 *   abrir     (datos)     carga la foto
 *   mirar     (datos)     la reduce a su agudeza de visión (sube con el nivel de las misiones)
 *   colores   (datos)     agrupa los píxeles por color con sus valores RGB reales
 *   bordes    (lógica)    mide los bordes (filtro de Sobel): horizontales, verticales
 *   regiones  (lógica)    qué color domina arriba y abajo
 *   describir (lenguaje)  lo cuenta en español y en Abla
 *   compartir (todos)     lo recuerdan, y lo que aparece junto se asocia
 *
 * Al terminar, la foto se guarda en pequeño en mundo/fotos/vistas/ con su
 * descripción, y se borra de la entrada. */
#include "vision.h"

#include "misiones.h"

Vision V;

static const char* PASOS[V_PASOS] = {"abrir", "mirar", "colores", "bordes", "regiones", "describir", "compartir"};
const char* paso_vision_nombre(int p) { return PASOS[p]; }

static void ruta(char* out, const char* rel) { ruta_unir(out, RUTA_MAX, E.mundo, rel); }

/* ---------- PPM (P6) ---------- */
static int ppm_numero(const char** p, const char* fin) {
  while (*p < fin) {
    if (**p == '#') {
      while (*p < fin && **p != '\n') (*p)++;
    } else if (isspace((unsigned char)**p)) {
      (*p)++;
    } else {
      break;
    }
  }
  int n = 0, digitos = 0;
  while (*p < fin && isdigit((unsigned char)**p)) n = n * 10 + (*(*p)++ - '0'), digitos++;
  return digitos ? n : -1;
}

int ppm_leer(const char* r, int* w, int* h, uint8_t** rgb) {
  Texto t;
  tx_iniciar(&t);
  if (!arch_leer(r, &t) || t.n < 11 || t.p[0] != 'P' || t.p[1] != '6') {
    tx_liberar(&t);
    return 0;
  }
  const char *p = t.p + 2, *fin = t.p + t.n;
  int ancho = ppm_numero(&p, fin), alto = ppm_numero(&p, fin), max = ppm_numero(&p, fin);
  p++; /* un solo espacio antes de los píxeles */
  if (ancho <= 0 || alto <= 0 || ancho > 4096 || alto > 4096 || max <= 0 || max > 255 ||
      fin - p < (long)ancho * alto * 3) {
    tx_liberar(&t);
    return 0;
  }
  *w = ancho, *h = alto;
  *rgb = xmalloc((size_t)ancho * (size_t)alto * 3);
  memcpy(*rgb, p, (size_t)ancho * (size_t)alto * 3);
  tx_liberar(&t);
  return 1;
}

int ppm_escribir(const char* r, int w, int h, const uint8_t* rgb) {
  char cab[32];
  int n = snprintf(cab, sizeof cab, "P6\n%d %d\n255\n", w, h);
  FILE* f = fopen(r, "wb");
  if (!f) return 0;
  int ok = fwrite(cab, 1, (size_t)n, f) == (size_t)n && fwrite(rgb, 1, (size_t)w * (size_t)h * 3, f) == (size_t)w * (size_t)h * 3;
  return fclose(f) == 0 && ok;
}

/* Reducir con media de cada bloque (los colores siguen siendo los reales). */
static uint8_t* reducir(const uint8_t* src, int w, int h, int nw, int nh) {
  uint8_t* d = xmalloc((size_t)nw * (size_t)nh * 3);
  for (int y = 0; y < nh; y++)
    for (int x = 0; x < nw; x++) {
      int x0 = x * w / nw, x1 = (x + 1) * w / nw, y0 = y * h / nh, y1 = (y + 1) * h / nh;
      if (x1 <= x0) x1 = x0 + 1;
      if (y1 <= y0) y1 = y0 + 1;
      unsigned long s[3] = {0, 0, 0}, n = 0;
      for (int yy = y0; yy < y1 && yy < h; yy++)
        for (int xx = x0; xx < x1 && xx < w; xx++, n++)
          for (int c = 0; c < 3; c++) s[c] += src[((size_t)yy * (size_t)w + (size_t)xx) * 3 + (size_t)c];
      for (int c = 0; c < 3; c++) d[((size_t)y * (size_t)nw + (size_t)x) * 3 + (size_t)c] = (uint8_t)(n ? s[c] / n : 0);
    }
  return d;
}

/* ---------- colores: nombre de un color real ---------- */
static const char* NOMBRES_COLOR[] = {"negro", "blanco", "gris", "rojo", "naranja", "marrón", "amarillo",
                                      "verde", "turquesa", "azul", "violeta", "rosa"};
enum { NEGRO, BLANCO, GRIS, ROJO, NARANJA, MARRON, AMARILLO, VERDE, TURQUESA, AZUL, VIOLETA, ROSA, NCOLOR };

static int nombre_color(int r, int g, int b) {
  float R = r / 255.0f, G = g / 255.0f, B = b / 255.0f;
  float mx = R > G ? (R > B ? R : B) : (G > B ? G : B), mn = R < G ? (R < B ? R : B) : (G < B ? G : B);
  float l = (mx + mn) / 2, d = mx - mn;
  float s = d < 1e-6f ? 0 : d / (1 - (2 * l - 1 < 0 ? -(2 * l - 1) : 2 * l - 1));
  if (l < 0.12f || (l < 0.2f && s < 0.5f)) return NEGRO; /* casi negro, aunque tenga un tinte */
  if (s < 0.15f || d < 0.06f) return l > 0.82f ? BLANCO : l < 0.22f ? NEGRO : GRIS;
  if (l > 0.92f) return BLANCO;
  float hue;
  if (mx == R) hue = 60 * ((G - B) / d);
  else if (mx == G) hue = 60 * ((B - R) / d + 2);
  else hue = 60 * ((R - G) / d + 4);
  if (hue < 0) hue += 360;
  if ((hue >= 10 && hue < 48) && l < 0.42f) return MARRON;
  if (hue < 12 || hue >= 345) return ROJO;
  if (hue < 40) return NARANJA;
  if (hue < 68) return AMARILLO;
  if (hue < 160) return VERDE;
  if (hue < 195) return TURQUESA;
  if (hue < 255) return AZUL;
  if (hue < 290) return VIOLETA;
  return ROSA;
}

/* Qué concepto de Abla les evoca cada color (arriba o abajo de la foto). */
static const char* concepto_color(int c, int arriba) {
  switch (c) {
    case AZUL: return arriba ? "cielo" : "agua";
    case TURQUESA: return "mar";
    case VERDE: return "árbol";
    case AMARILLO: return "sol";
    case NARANJA:
    case ROJO: return "fuego";
    case MARRON: return "tierra";
    case GRIS: return "piedra";
    case BLANCO: return "luz";
    case NEGRO: return "sombra";
    case ROSA: return "alegría";
    default: return "sueño";
  }
}

static void sumar_concepto(const char* palabra, float peso) {
  int id = idioma_de_espanol(palabra);
  if (id < 0) return;
  for (int i = 0; i < V.nconceptos; i++)
    if (V.conceptos[i] == id) {
      V.pesos[i] += peso;
      return;
    }
  if (V.nconceptos < 6) {
    V.conceptos[V.nconceptos] = id;
    V.pesos[V.nconceptos++] = peso;
  }
}

/* El nombre de la foto sin el número de cola que le pone la app ni la extensión. */
static const char* bonito(const char* n) {
  static char b[256];
  const char* p = n;
  while (*p && (isdigit((unsigned char)*p) || *p == '-')) p++; /* "1791011343645-0003-" */
  if (!*p || *p == '.') p = n;
  snprintf(b, sizeof b, "%s", p);
  char* e = strstr(b, ".ppm");
  if (e) *e = 0;
  return b;
}
const char* vision_nombre_bonito(const char* n) { return bonito(n); }

/* ---------- los pasos ---------- */
static int dominante(int y0, int y1) {
  int cuenta[NCOLOR] = {0}, mejor = 0;
  for (int y = y0; y < y1; y++)
    for (int x = 0; x < V.aw; x++) {
      const uint8_t* p = V.vista + ((size_t)y * (size_t)V.aw + (size_t)x) * 3;
      cuenta[nombre_color(p[0], p[1], p[2])]++;
    }
  for (int c = 1; c < NCOLOR; c++)
    if (cuenta[c] > cuenta[mejor]) mejor = c;
  return mejor;
}

static int siguiente_en_cola(char* nombre, size_t n) {
  char r[RUTA_MAX];
  ruta(r, "fotos/entrada");
  Entrada* e;
  int k = arch_listar(r, &e), elegido = -1;
  V.en_cola = 0;
  for (int i = 0; i < k; i++)
    if (!e[i].directorio && strstr(e[i].nombre, ".ppm")) {
      V.en_cola++;
      if (elegido < 0) elegido = i;
    }
  if (elegido >= 0) snprintf(nombre, n, "%s", e[elegido].nombre);
  free(e);
  return elegido >= 0;
}

static void terminar_foto(int guardar) {
  char r[RUTA_MAX], base[260];
  snprintf(base, sizeof base, "%s", V.nombre);
  char* punto = strstr(base, ".ppm");
  if (punto) *punto = 0;
  if (guardar) {
    char rel[300];
    snprintf(rel, sizeof rel, "fotos/vistas/%s.ppm", base);
    ruta(r, rel);
    int tw = V.w > 160 ? 160 : V.w, th = V.w > 160 ? V.h * 160 / V.w : V.h;
    if (th < 1) th = 1;
    uint8_t* mini = reducir(V.rgb, V.w, V.h, tw, th);
    ppm_escribir(r, tw, th, mini);
    free(mini);
    snprintf(rel, sizeof rel, "fotos/vistas/%s.txt", base);
    ruta(r, rel);
    Texto t;
    tx_iniciar(&t);
    tx_printf(&t, "%s\nEn Abla: %s\n", V.descripcion, V.abla);
    arch_escribir(r, t.p, t.n, 0);
    tx_liberar(&t);
  }
  char rel[300];
  snprintf(rel, sizeof rel, "fotos/entrada/%s", V.nombre);
  ruta(r, rel);
  remove(r);
  V.activa = 0;
}

void vision_paso(void) {
  if (!V.activa) {
    char nombre[256];
    if (!siguiente_en_cola(nombre, sizeof nombre)) return;
    snprintf(V.nombre, sizeof V.nombre, "%s", nombre);
    V.activa = 1;
    V.paso = V_ABRIR;
    V.inicio_ms = ahora_ms();
  }
  static const int TRIBU_PASO[V_PASOS] = {DATOS, DATOS, DATOS, LOGICA, LOGICA, LENGUAJE, LENGUAJE};
  V.ser_paso = TRIBU_PASO[V.paso] * 9 + azar(9);
  Ser* s = &E.seres[V.ser_paso];
  switch (V.paso) {
    case V_ABRIR: {
      char r[RUTA_MAX], rel[300];
      snprintf(rel, sizeof rel, "fotos/entrada/%s", V.nombre);
      ruta(r, rel);
      free(V.rgb);
      V.rgb = NULL;
      if (!ppm_leer(r, &V.w, &V.h, &V.rgb)) {
        anotarf(s, 1, "No puedo abrir la foto «%s»: no es una imagen PPM válida. La aparto.", bonito(V.nombre));
        terminar_foto(0);
        return;
      }
      anotarf(s, 0, "Abro la foto «%s» (%d×%d píxeles).", bonito(V.nombre), V.w, V.h);
      break;
    }
    case V_MIRAR: {
      int aw, ah;
      agudeza_de_nivel(M.nivel ? M.nivel : 1, &aw, &ah);
      if (aw > V.w) aw = V.w;
      ah = aw * V.h / V.w;
      if (ah < 1) ah = 1;
      free(V.vista);
      V.vista = reducir(V.rgb, V.w, V.h, aw, ah);
      V.aw = aw, V.ah = ah;
      anotarf(s, 0, "Miro «%s» con mi agudeza de %d×%d píxeles.", bonito(V.nombre), aw, ah);
      break;
    }
    case V_COLORES: {
      unsigned long cuenta[NCOLOR] = {0}, suma[NCOLOR][3];
      memset(suma, 0, sizeof suma);
      double brillo = 0, brillo2 = 0;
      int n = V.aw * V.ah;
      for (int i = 0; i < n; i++) {
        const uint8_t* p = V.vista + (size_t)i * 3;
        int c = nombre_color(p[0], p[1], p[2]);
        cuenta[c]++;
        for (int k = 0; k < 3; k++) suma[c][k] += p[k];
        double l = (0.299 * p[0] + 0.587 * p[1] + 0.114 * p[2]) / 255.0;
        brillo += l, brillo2 += l * l;
      }
      V.brillo = (float)(brillo / n);
      double var = brillo2 / n - (brillo / n) * (brillo / n);
      V.contraste = (float)(var > 0 ? var * 4 : 0);
      if (V.contraste > 1) V.contraste = 1;
      V.npaleta = 0;
      for (int k = 0; k < 6; k++) {
        int mejor = -1;
        for (int c = 0; c < NCOLOR; c++) {
          int ya = 0;
          for (int j = 0; j < V.npaleta; j++) ya |= !strcmp(V.paleta[j].nombre, NOMBRES_COLOR[c]);
          if (!ya && cuenta[c] && (mejor < 0 || cuenta[c] > cuenta[mejor])) mejor = c;
        }
        if (mejor < 0) break;
        Tono* t = &V.paleta[V.npaleta++];
        snprintf(t->nombre, sizeof t->nombre, "%s", NOMBRES_COLOR[mejor]);
        t->r = (uint8_t)(suma[mejor][0] / cuenta[mejor]);
        t->g = (uint8_t)(suma[mejor][1] / cuenta[mejor]);
        t->b = (uint8_t)(suma[mejor][2] / cuenta[mejor]);
        t->frac = (float)cuenta[mejor] / (float)n;
      }
      anotarf(s, 0, "Colores de «%s»: %s %d%%, %s %d%%. Brillo %d%%.", bonito(V.nombre), V.paleta[0].nombre, (int)(V.paleta[0].frac * 100),
              V.npaleta > 1 ? V.paleta[1].nombre : "-", V.npaleta > 1 ? (int)(V.paleta[1].frac * 100) : 0, (int)(V.brillo * 100));
      break;
    }
    case V_BORDES: { /* Sobel sobre la luminancia de lo que ven */
      double total = 0, hor = 0, ver = 0;
      int n = 0;
      for (int y = 1; y + 1 < V.ah; y++)
        for (int x = 1; x + 1 < V.aw; x++) {
#define L(xx, yy) (0.299 * V.vista[((size_t)(yy) * (size_t)V.aw + (size_t)(xx)) * 3] + 0.587 * V.vista[((size_t)(yy) * (size_t)V.aw + (size_t)(xx)) * 3 + 1] + 0.114 * V.vista[((size_t)(yy) * (size_t)V.aw + (size_t)(xx)) * 3 + 2])
          double gx = L(x + 1, y - 1) + 2 * L(x + 1, y) + L(x + 1, y + 1) - L(x - 1, y - 1) - 2 * L(x - 1, y) - L(x - 1, y + 1);
          double gy = L(x - 1, y + 1) + 2 * L(x, y + 1) + L(x + 1, y + 1) - L(x - 1, y - 1) - 2 * L(x, y - 1) - L(x + 1, y - 1);
#undef L
          double m = (gx < 0 ? -gx : gx) + (gy < 0 ? -gy : gy);
          if (m > 120) {
            total++;
            if ((gy < 0 ? -gy : gy) > (gx < 0 ? -gx : gx)) hor++;
            else ver++;
          }
          n++;
        }
      V.bordes = n ? (float)(total / n) : 0;
      V.bordes_h = total ? (float)(hor / total) : 0;
      V.bordes_v = total ? (float)(ver / total) : 0;
      anotarf(s, 0, "Bordes en «%s»: %d%% de la imagen (%s).", bonito(V.nombre), (int)(V.bordes * 100),
              V.bordes_h > 0.6f ? "sobre todo horizontales" : V.bordes_v > 0.6f ? "sobre todo verticales" : "en todas direcciones");
      break;
    }
    case V_REGIONES: {
      int tercio = V.ah / 3 > 0 ? V.ah / 3 : 1;
      int a = dominante(0, tercio), b = dominante(V.ah - tercio, V.ah);
      snprintf(V.arriba, sizeof V.arriba, "%s", NOMBRES_COLOR[a]);
      snprintf(V.abajo, sizeof V.abajo, "%s", NOMBRES_COLOR[b]);
      V.nconceptos = 0;
      sumar_concepto(concepto_color(a, 1), 1.0f);
      sumar_concepto(concepto_color(b, 0), 1.0f);
      for (int i = 0; i < V.npaleta && i < 3; i++)
        for (int c = 0; c < NCOLOR; c++)
          if (V.paleta[i].frac >= 0.05f && !strcmp(V.paleta[i].nombre, NOMBRES_COLOR[c]))
            sumar_concepto(concepto_color(c, 1), V.paleta[i].frac); /* lo que apenas se ve no cuenta */
      sumar_concepto(V.brillo > 0.6f ? "día" : V.brillo < 0.25f ? "noche" : "luz", 0.5f);
      if (V.bordes > 0.18f) sumar_concepto("patrón", V.bordes);
      anotarf(s, 0, "En «%s», arriba domina el %s y abajo el %s.", bonito(V.nombre), V.arriba, V.abajo);
      break;
    }
    case V_DESCRIBIR: {
      Texto d, a;
      tx_iniciar(&d);
      tx_iniciar(&a);
      tx_printf(&d, "Veo «%s»: arriba %s, abajo %s; sobre todo ", bonito(V.nombre), V.arriba, V.abajo);
      for (int i = 0; i < V.npaleta && i < 3; i++)
        tx_printf(&d, "%s%s (%d%%)", i ? (i + 1 < V.npaleta && i < 2 ? ", " : " y ") : "", V.paleta[i].nombre, (int)(V.paleta[i].frac * 100));
      tx_printf(&d, ". %s, %s. Me recuerda a: ", V.brillo > 0.6f ? "Hay mucha luz" : V.brillo < 0.25f ? "Está oscuro" : "Luz media",
                V.bordes > 0.18f ? "con muchos detalles" : "con formas tranquilas");
      int ids[6];
      for (int i = 0; i < V.nconceptos; i++) {
        tx_printf(&d, "%s%s", i ? ", " : "", idioma_glosa_tmp(V.conceptos[i]));
        ids[i] = V.conceptos[i];
      }
      tx_add(&d, ".");
      idioma_frase(ids, V.nconceptos, &a);
      snprintf(V.descripcion, sizeof V.descripcion, "%s", d.p);
      snprintf(V.abla, sizeof V.abla, "%s", a.p ? a.p : "");
      anotarf(s, 1, "%s En Abla: «%s».", V.descripcion, V.abla);
      tx_liberar(&d);
      tx_liberar(&a);
      break;
    }
    default: { /* COMPARTIR: lo recuerdan y asocian lo que aparece junto */
      for (int k = 0; k < NUM_SERES; k++)
        if (k != V.ser_paso) anotarf(&E.seres[k], 0, "Recuerdo la foto «%s»: %s", bonito(V.nombre), V.abla);
      for (int i = 0; i < V.nconceptos; i++)
        for (int j = 0; j < V.nconceptos; j++)
          if (i != j)
            for (int k = 0; k < 9; k++) notar_par(&E.seres[k], V.conceptos[i], V.conceptos[j]); /* la tribu del lenguaje */
      /* si arriba y abajo tienen el mismo color, la lógica deduce que se parecen */
      int dom = V.npaleta ? nombre_color(V.paleta[0].r, V.paleta[0].g, V.paleta[0].b) : NEGRO;
      int a = idioma_de_espanol(concepto_color(dom, 1)), b = idioma_de_espanol(concepto_color(dom, 0));
      if (V.npaleta && !strcmp(V.arriba, V.abajo) && a >= 0 && b >= 0 && a != b) {
        Ser* l = &E.seres[18 + azar(9)];
        struct hecho h = {(uint16_t)a, IGUAL, (uint16_t)b, 0.6f, (uint32_t)l->id};
        if (saber_agregar(&l->saber, h)) anotarf(l, 1, "Viendo «%s» deduzco que %s.", bonito(V.nombre), glosa_hecho(&h));
      }
      double ms = ahora_ms() - V.inicio_ms;
      V.ms_por_foto = V.vistas ? V.ms_por_foto * 0.8 + ms * 0.2 : ms;
      V.vistas++;
      terminar_foto(1);
      return;
    }
  }
  V.paso++;
}

void vision_iniciar(void) {
  memset(&V, 0, sizeof V);
  char r[RUTA_MAX];
  ruta(r, "fotos/entrada");
  arch_crear_directorios(r);
  ruta(r, "fotos/vistas");
  arch_crear_directorios(r);
  Entrada* e;
  int n = arch_listar(r, &e);
  for (int i = 0; i < n; i++)
    if (strstr(e[i].nombre, ".ppm")) V.vistas++;
  free(e);
  char nombre[256];
  siguiente_en_cola(nombre, sizeof nombre);
}

void vision_describir(Texto* t) {
  int aw, ah;
  agudeza_de_nivel(M.nivel ? M.nivel : 1, &aw, &ah);
  tx_printf(t, "Visión: %d×%d píxeles (sube con el nivel de las misiones) · %lu fotos vistas · %d en cola", aw, ah, V.vistas, V.en_cola);
  if (V.en_cola && V.ms_por_foto > 0) {
    double seg = V.en_cola * V.ms_por_foto / 1000.0;
    if (seg < 90) tx_printf(t, " · faltan unos %.0f s", seg);
    else tx_printf(t, " · faltan unos %.1f min", seg / 60.0);
  }
  tx_add(t, "\n");
  if (V.activa) tx_printf(t, "Ahora: «%s», paso %s (%s)\n", bonito(V.nombre), paso_vision_nombre(V.paso), E.seres[V.ser_paso].nombre);
  if (V.descripcion[0]) tx_printf(t, "Última: %s\nEn Abla: %s\n", V.descripcion, V.abla);
  tx_add(t, "Para mandarles fotos: ponlas como .ppm en /fotos/entrada (en la app, botón «Añadir fotos»).\n");
}
