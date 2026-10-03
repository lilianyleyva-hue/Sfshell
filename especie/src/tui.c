/* Interfaz de terminal a pantalla completa, en C puro (ANSI + termios).
 *
 *   ┌ Red ──────────────┐┌ Pensamientos / Ser / Salida ┐
 *   │  las 27 mentes y  ││  (Tab cambia de panel)      │
 *   │  sus mensajes     ││                             │
 *   └───────────────────┘└─────────────────────────────┘
 *   ┌ Seres ─────────────────────────────────────────────┐
 *   humano@abla:/$ _
 *
 * Mientras escribes, la especie sigue pensando: un solo bucle reparte el
 * tiempo entre el teclado, los ciclos de pensamiento y el dibujo.
 *
 * El dibujo y el teclado son los mismos en todas partes. Cambia solo dónde
 * se muestra:
 *   - en una terminal (Linux, macOS, Termux, iSH): ANSI + termios;
 *   - en WebAssembly (Code App, Safari): la página pide cada cuadro ya hecho
 *     (abla_tui_html) y le pasa las teclas (abla_tui_teclas). */
#ifndef _DEFAULT_SOURCE
#define _DEFAULT_SOURCE /* TIOCGWINSZ y struct winsize */
#endif
#include "especie.h"

#if defined(__wasm__) || defined(ABLA_SIN_TUI)
#define TERMINAL_CRUDA 0
#else
#define TERMINAL_CRUDA 1
#include <sys/ioctl.h>
#include <sys/select.h>
#include <termios.h>
#include <unistd.h>
#endif

/* ---------- colores ---------- */
enum { C_NORMAL, C_TENUE, C_LENGUAJE, C_DATOS, C_LOGICA, C_DATAC, C_BLANCO, C_ROJO, C_AZUL, NCOLORES };
static const int COLOR_TRIBU[3] = {C_LENGUAJE, C_DATOS, C_LOGICA};
#if TERMINAL_CRUDA
static const char* SGR[NCOLORES] = {"0", "90", "95", "96", "92", "93", "97", "91", "94"};
static int usar_color = 1;
#endif

/* ---------- pantalla: una rejilla de celdas que se vuelca de una vez ---------- */
typedef struct {
  char c[5];
  uint8_t color, negrita;
} Celda;

static Celda* pantalla;
static int W, H;

#if TERMINAL_CRUDA
static void tamano(int* w, int* h) {
  struct winsize ws;
  if (ioctl(STDOUT_FILENO, TIOCGWINSZ, &ws) == 0 && ws.ws_col > 0 && ws.ws_row > 0) {
    *w = ws.ws_col;
    *h = ws.ws_row;
  } else {
    *w = 80;
    *h = 24;
  }
}
#endif

static void limpiar_pantalla(void) {
  for (int i = 0; i < W * H; i++) {
    strcpy(pantalla[i].c, " ");
    pantalla[i].color = C_NORMAL;
    pantalla[i].negrita = 0;
  }
}

/* Largo de un carácter UTF-8 a partir de su primer byte. */
static int largo_utf8(unsigned char b) { return b < 0x80 ? 1 : b < 0xE0 ? 2 : b < 0xF0 ? 3 : 4; }

static int ancho_utf8(const char* s) {
  int n = 0;
  for (; *s; s += largo_utf8((unsigned char)*s)) n++;
  return n;
}

/* Escribe texto en (x, y) sin pasar de max columnas. Devuelve las columnas usadas. */
static int texto(int x, int y, int max, const char* s, int color, int negrita) {
  if (y < 0 || y >= H) return 0;
  int n = 0;
  while (*s && n < max && x + n < W) {
    int l = largo_utf8((unsigned char)*s);
    if (x + n >= 0) {
      Celda* c = &pantalla[y * W + x + n];
      memcpy(c->c, s, (size_t)l);
      c->c[l] = 0;
      c->color = (uint8_t)color;
      c->negrita = (uint8_t)negrita;
    }
    s += l;
    n++;
  }
  return n;
}

static void caja(int x, int y, int w, int h, const char* titulo, int activa) {
  int col = activa ? C_BLANCO : C_TENUE;
  for (int i = 1; i < w - 1; i++) {
    texto(x + i, y, 1, "─", col, 0);
    texto(x + i, y + h - 1, 1, "─", col, 0);
  }
  for (int j = 1; j < h - 1; j++) {
    texto(x, y + j, 1, "│", col, 0);
    texto(x + w - 1, y + j, 1, "│", col, 0);
  }
  texto(x, y, 1, "┌", col, 0);
  texto(x + w - 1, y, 1, "┐", col, 0);
  texto(x, y + h - 1, 1, "└", col, 0);
  texto(x + w - 1, y + h - 1, 1, "┘", col, 0);
  if (titulo) {
    char t[128];
    snprintf(t, sizeof t, " %s ", titulo);
    texto(x + 2, y, w - 4, t, activa ? C_BLANCO : C_TENUE, activa);
  }
}

#if TERMINAL_CRUDA
static void escribir_todo(const char* p, size_t n) {
  while (n) {
    ssize_t w = write(STDOUT_FILENO, p, n);
    if (w <= 0) return;
    p += w;
    n -= (size_t)w;
  }
}

/* Solo se reescriben las filas que cambiaron desde el cuadro anterior: en
 * terminales lentas (iSH) eso es mucho menos que escribir. El modo 2026 hace
 * que las terminales que lo entienden muestren solo cuadros completos. */
static char** previas;
static int previas_h = 0;

static void olvidar_cuadro(void) {
  for (int i = 0; i < previas_h; i++) free(previas[i]);
  free(previas);
  previas = NULL;
  previas_h = 0;
}

static void volcar(int cursor_x, int cursor_y) {
  if (previas_h != H) {
    olvidar_cuadro();
    previas = xcalloc((size_t)H, sizeof(char*));
    previas_h = H;
  }
  Texto t, fila;
  tx_iniciar(&t);
  tx_iniciar(&fila);
  tx_add(&t, "\033[?2026h\033[?25l");
  for (int y = 0; y < H; y++) {
    tx_vaciar(&fila);
    tx_add(&fila, "");
    int color = -1, negrita = -1;
    for (int x = 0; x < W; x++) {
      Celda* c = &pantalla[y * W + x];
      if (usar_color && (c->color != color || c->negrita != negrita)) {
        tx_printf(&fila, "\033[0;%s%sm", c->negrita ? "1;" : "", SGR[c->color]);
        color = c->color;
        negrita = c->negrita;
      }
      tx_add(&fila, c->c);
    }
    if (previas[y] && !strcmp(previas[y], fila.p)) continue;
    free(previas[y]);
    previas[y] = xstrdup(fila.p);
    tx_printf(&t, "\033[%d;1H", y + 1);
    tx_addn(&t, fila.p, fila.n);
  }
  tx_printf(&t, "\033[0m\033[%d;%dH\033[?25h\033[?2026l", cursor_y + 1, cursor_x + 1);
  escribir_todo(t.p, t.n);
  tx_liberar(&t);
  tx_liberar(&fila);
}

/* ---------- terminal cruda ---------- */
static struct termios original;
static int cruda = 0;

static void restaurar(void) {
  if (!cruda) return;
  const char* s = "\033[0m\033[?25h\033[?1049l";
  escribir_todo(s, strlen(s));
  tcsetattr(STDIN_FILENO, TCSAFLUSH, &original);
  cruda = 0;
}

static int activar_cruda(void) {
  if (tcgetattr(STDIN_FILENO, &original) != 0) return 0;
  struct termios r = original;
  r.c_iflag &= (tcflag_t) ~(BRKINT | ICRNL | INPCK | ISTRIP | IXON);
  r.c_oflag &= (tcflag_t) ~(OPOST);
  r.c_cflag |= CS8;
  r.c_lflag &= (tcflag_t) ~(ECHO | ICANON | IEXTEN | ISIG);
  r.c_cc[VMIN] = 0;
  r.c_cc[VTIME] = 0;
  if (tcsetattr(STDIN_FILENO, TCSAFLUSH, &r) != 0) return 0;
  cruda = 1;
  atexit(restaurar);
  const char* s = "\033[?1049h\033[2J";
  escribir_todo(s, strlen(s));
  return 1;
}

#endif /* TERMINAL_CRUDA */

/* Borra la terminal de verdad (en el navegador no hace falta). */
static void borrar_terminal(void) {
#if TERMINAL_CRUDA
  escribir_todo("\033[2J", 4);
  olvidar_cuadro();
#endif
}

/* ---------- estado de la interfaz ---------- */
enum { P_PENSAMIENTOS, P_SER, P_SALIDA, NPANELES };
static const char* NOMBRE_PANEL[NPANELES] = {"Pensamientos", "Ser", "Salida"};
static int panel = P_PENSAMIENTOS, elegido = 18;
static char entrada[1024];
static size_t lentrada = 0;

#define HIST 50
static char* historial[HIST];
static int nhist = 0, poshist = 0;

/* lo que devuelven los comandos */
#define SALIDA_MAX 600
static char* salida[SALIDA_MAX];
static uint8_t salida_color[SALIDA_MAX];
static int nsalida = 0, desplazar = 0;

static void anadir_salida(const char* linea, int color) {
  if (nsalida == SALIDA_MAX) {
    free(salida[0]);
    memmove(salida, salida + 1, sizeof(char*) * (SALIDA_MAX - 1));
    memmove(salida_color, salida_color + 1, SALIDA_MAX - 1);
    nsalida--;
  }
  salida[nsalida] = xstrdup(linea);
  salida_color[nsalida++] = (uint8_t)color;
}

/* mensajes que viajan por la red */
typedef struct {
  int de, para, c;
  float t;
} Particula;
#define MAX_PART 240
static Particula part[MAX_PART];
static int npart = 0;
static uint64_t visto_red = 0, visto_linea = 0;
static int actividad[NUM_SERES];

static void leer_novedades(void) {
  uint64_t min = E.nred > RED ? E.nred - RED : 0;
  if (visto_red < min) visto_red = min;
  if (E.nred - visto_red > 60) visto_red = E.nred - 60;
  for (; visto_red < E.nred; visto_red++) {
    if (npart == MAX_PART) memmove(part, part + 1, sizeof(Particula) * --npart);
    Particula* p = &part[npart++];
    p->de = E.red[visto_red % RED].de;
    p->para = E.red[visto_red % RED].para;
    p->c = E.red[visto_red % RED].c;
    p->t = 0;
  }
  for (; visto_linea < E.nlinea; visto_linea++) actividad[E.corriente[visto_linea % CORRIENTE].ser] = 6;
}

/* ---------- partir texto en líneas de un ancho dado ---------- */
typedef void (*Linea_fn)(const char* inicio, int bytes, void* ctx);

static void partir(const char* s, int ancho, Linea_fn fn, void* ctx) {
  if (ancho < 1) return;
  while (*s) {
    const char* p = s;
    const char* corte = NULL;
    int n = 0;
    while (*p && *p != '\n' && n < ancho) {
      if (*p == ' ') corte = p;
      p += largo_utf8((unsigned char)*p);
      n++;
    }
    if (!*p || *p == '\n' || !corte || corte == s) corte = p;
    fn(s, (int)(corte - s), ctx);
    s = corte;
    while (*s == ' ') s++;
    if (*s == '\n') s++;
  }
}

typedef struct {
  char** l;
  uint8_t* col;
  int n, cap, color;
} Lineas;

static void lineas_poner(const char* inicio, int bytes, void* ctx) {
  Lineas* L = ctx;
  if (L->n == L->cap) {
    L->cap = L->cap ? L->cap * 2 : 64;
    L->l = xrealloc(L->l, sizeof(char*) * (size_t)L->cap);
    L->col = xrealloc(L->col, (size_t)L->cap);
  }
  char* x = xmalloc((size_t)bytes + 1);
  memcpy(x, inicio, (size_t)bytes);
  x[bytes] = 0;
  L->l[L->n] = x;
  L->col[L->n++] = (uint8_t)L->color;
}

static void lineas_texto(Lineas* L, const char* s, int ancho, int color) {
  L->color = color;
  partir(s, ancho, lineas_poner, L);
}

static void lineas_liberar(Lineas* L) {
  for (int i = 0; i < L->n; i++) free(L->l[i]);
  free(L->l);
  free(L->col);
  memset(L, 0, sizeof *L);
}

/* ---------- seno y coseno sin libm (solo para colocar las mentes) ---------- */
static double sin_aprox(double x) {
  const double PI = 3.14159265358979;
  while (x > PI) x -= 2 * PI;
  while (x < -PI) x += 2 * PI;
  if (x > PI / 2) x = PI - x;
  if (x < -PI / 2) x = -PI - x;
  double x2 = x * x;
  return x * (1 - x2 / 6 * (1 - x2 / 20 * (1 - x2 / 42 * (1 - x2 / 72))));
}
static double cos_aprox(double x) { return sin_aprox(x + 3.14159265358979 / 2); }

/* ---------- paneles ---------- */
static void dibujar_red(int x0, int y0, int w, int h) {
  caja(x0, y0, w, h, "Red de la especie", 0);
  int iw = w - 2, ih = h - 2, ox = x0 + 1, oy = y0 + 1;
  if (iw < 12 || ih < 6) return;
  double cx = iw / 2.0, cy = ih / 2.0;
  double ry = (ih - 2) / 2.0 * 0.9, rx = ry * 2.1;
  if (rx > (iw - 14) / 2.0) rx = (iw - 14) / 2.0;
  int px[NUM_SERES], py[NUM_SERES];
  for (int i = 0; i < NUM_SERES; i++) {
    int tribu = i / 9, k = i % 9;
    double ang = -3.14159265 / 2 + tribu * (2 * 3.14159265 / 3) + (k - 4) * 0.2;
    double r = k % 2 ? 0.8 : 1.0;
    px[i] = (int)(cx + cos_aprox(ang) * rx * r + 0.5);
    py[i] = (int)(cy + sin_aprox(ang) * ry * r + 0.5);
  }
  /* 1) las 27 mentes */
  for (int i = 0; i < NUM_SERES; i++) {
    int sel = i == elegido, col = COLOR_TRIBU[E.seres[i].tribu];
    texto(ox + px[i], oy + py[i], 1, sel ? "◉" : actividad[i] ? "●" : "•", sel ? C_BLANCO : col, actividad[i] > 0 || sel);
  }
  /* 2) sus nombres, solo donde hay sitio: a la derecha, a la izquierda, o más corto */
  for (int i = 0; i < NUM_SERES; i++) {
    Ser* s = &E.seres[i];
    int sel = i == elegido, col = COLOR_TRIBU[s->tribu];
    for (int largo = 6; largo >= 3; largo--) {
      int puesto = 0;
      for (int intento = 0; intento < 2 && !puesto; intento++) {
        int derecha = (px[i] < cx) ? intento == 1 : intento == 0; /* primero hacia fuera */
        int x = derecha ? px[i] + 2 : px[i] - largo - 1;
        int libre = x >= 0 && x + largo <= iw;
        for (int k = -1; libre && k <= largo; k++) {
          int cx2 = ox + x + k;
          if (cx2 < ox || cx2 >= ox + iw) continue;
          if (strcmp(pantalla[(oy + py[i]) * W + cx2].c, " ")) libre = 0;
        }
        if (libre) {
          char etiqueta[16];
          snprintf(etiqueta, sizeof etiqueta, "%.*s", largo, s->nombre);
          texto(ox + x, oy + py[i], largo, etiqueta, sel ? C_BLANCO : col, sel);
          puesto = 1;
        }
      }
      if (puesto) break;
    }
  }
  /* 3) los mensajes en camino, solo en celdas vacías */
  for (int k = 0; k < npart; k++) {
    Particula* p = &part[k];
    int x = (int)(px[p->de] + (px[p->para] - px[p->de]) * p->t + 0.5);
    int y = (int)(py[p->de] + (py[p->para] - py[p->de]) * p->t + 0.5);
    if (x < 0 || x >= iw || y < 0 || y >= ih || strcmp(pantalla[(oy + y) * W + ox + x].c, " ")) continue;
    texto(ox + x, oy + y, 1, p->c ? "▪" : "·", p->c ? C_DATAC : C_BLANCO, 0);
  }
  texto(ox + 1, oy + ih - 1, iw - 2, "· frase en Abla   ▪ data C", C_TENUE, 0);
}

static void dibujar_pensamientos(int x0, int y0, int w, int h) {
  int iw = w - 4, ih = h - 2;
  Lineas L = {0};
  uint64_t desde = E.nlinea > 120 ? E.nlinea - 120 : 0;
  for (uint64_t n = desde; n < E.nlinea; n++) {
    Linea* l = &E.corriente[n % CORRIENTE];
    /* "[t=12] Nombre (tribu): texto" → "12 Nombre: texto" */
    const char* t = l->texto;
    const char* fin_t = strchr(t, ']');
    const char* dos = strstr(t, "): ");
    if (!fin_t || !dos) {
      lineas_texto(&L, t, iw, C_NORMAL);
      continue;
    }
    char cab[64];
    snprintf(cab, sizeof cab, "%.*s %s", (int)(fin_t - t - 3), t + 3, E.seres[l->ser].nombre);
    Texto x;
    tx_iniciar(&x);
    tx_printf(&x, "%s: %s", cab, dos + 3);
    int primera = L.n;
    lineas_texto(&L, x.p, iw, C_NORMAL);
    if (primera < L.n) L.col[primera] = (uint8_t)(100 + l->ser); /* marca: colorear la cabecera */
    tx_liberar(&x);
  }
  int empieza = L.n > ih ? L.n - ih : 0;
  for (int i = empieza; i < L.n; i++) {
    int y = y0 + 1 + i - empieza;
    if (L.col[i] >= 100) {
      int ser = L.col[i] - 100;
      char* dos_p = strstr(L.l[i], ": ");
      char* esp = strchr(L.l[i], ' ');
      if (dos_p && esp && esp < dos_p) {
        int n1 = texto(x0 + 2, y, iw, L.l[i], C_TENUE, 0);
        (void)n1;
        int an_t = (int)(esp - L.l[i]);
        char nombre[32];
        snprintf(nombre, sizeof nombre, "%.*s", (int)(dos_p - esp), esp);
        texto(x0 + 2 + an_t, y, iw - an_t, nombre, COLOR_TRIBU[ser / 9], 1);
        int an_n = ancho_utf8(nombre);
        texto(x0 + 2 + an_t + an_n, y, iw - an_t - an_n, dos_p, C_NORMAL, 0);
        continue;
      }
    }
    texto(x0 + 2, y, iw, L.l[i], L.col[i] >= 100 ? C_NORMAL : L.col[i], 0);
  }
  lineas_liberar(&L);
}

static void dibujar_ser(int x0, int y0, int w, int h) {
  int iw = w - 4, ih = h - 2;
  Ser* s = &E.seres[elegido];
  Lineas L = {0};
  char buf[512];
  snprintf(buf, sizeof buf, "%d. %s — «quien %s», tribu %s", elegido + 1, s->nombre, idioma_glosa_tmp(s->concepto),
           nombre_tribu(s->tribu));
  lineas_texto(&L, buf, iw, COLOR_TRIBU[s->tribu]);
  int nuevas = 0;
  for (int p = BASE; p < idioma_tamano(); p++) nuevas += conoce_palabra(s, p);
  snprintf(buf, sizeof buf,
           "%zu hechos · %llu pensamientos · contexto %llu/%llu tokens · largo plazo %llu recuerdos · %d palabras nuevas · "
           "%llu enviados, %llu recibidos",
           s->saber.n, (unsigned long long)s->pensamientos, (unsigned long long)s->memoria.usados,
           (unsigned long long)s->memoria.capacidad, (unsigned long long)s->memoria.total, nuevas,
           (unsigned long long)s->enviados, (unsigned long long)s->recibidos);
  lineas_texto(&L, buf, iw, C_TENUE);
  lineas_texto(&L, " ", iw, C_NORMAL);
  lineas_texto(&L, "Ahora piensa:", iw, C_BLANCO);
  lineas_texto(&L, s->ultimo ? s->ultimo : "", iw, C_NORMAL);
  lineas_texto(&L, " ", iw, C_NORMAL);
  lineas_texto(&L, "Su mente (lo último):", iw, C_BLANCO);
  size_t total = s->memoria.n;
  for (size_t i = total > 6 ? total - 6 : 0; i < total; i++) {
    const Recuerdo* r = memoria_en(&s->memoria, i);
    snprintf(buf, sizeof buf, "[%llu] %s", (unsigned long long)r->tick, r->texto);
    lineas_texto(&L, buf, iw, C_TENUE);
  }
  lineas_texto(&L, " ", iw, C_NORMAL);
  snprintf(buf, sizeof buf, "Su conocimiento, en C (/conocimiento/%s.c):", s->nombre);
  lineas_texto(&L, buf, iw, C_BLANCO);
  snprintf(buf, sizeof buf, "const struct hecho %s[] = {", s->nombre);
  lineas_texto(&L, buf, iw, C_AZUL);
  for (size_t i = 0; i < s->saber.n && L.n < ih + 40; i++) {
    const struct hecho* h = &s->saber.h[i];
    snprintf(buf, sizeof buf, "  {%u, %u, %u, %.2ff, %u}, /* %s */", h->s, h->r, h->o, (double)h->confianza, (unsigned)h->fuente,
             glosa_hecho(h));
    lineas_texto(&L, buf, iw, C_AZUL);
  }
  for (int i = 0; i < L.n && i < ih; i++) texto(x0 + 2, y0 + 1 + i, iw, L.l[i], L.col[i], i == 0);
  lineas_liberar(&L);
}

static void dibujar_salida(int x0, int y0, int w, int h) {
  int iw = w - 4, ih = h - 2;
  Lineas L = {0};
  for (int i = 0; i < nsalida; i++) lineas_texto(&L, salida[i][0] ? salida[i] : " ", iw, salida_color[i]);
  if (!L.n) lineas_texto(&L, "Escribe un comando abajo (prueba: ayuda, seres, decir 19 sol causa luz pregunta).", iw, C_TENUE);
  int max_desp = L.n > ih ? L.n - ih : 0;
  if (desplazar > max_desp) desplazar = max_desp;
  int empieza = L.n > ih ? L.n - ih - desplazar : 0;
  for (int i = 0; i < ih && empieza + i < L.n; i++) texto(x0 + 2, y0 + 1 + i, iw, L.l[empieza + i], L.col[empieza + i], 0);
  lineas_liberar(&L);
}

static void dibujar_seres(int x0, int y0, int w, int h) {
  caja(x0, y0, w, h, "Los 27  (← → para elegir)", 0);
  int ancho = 16, por_fila = (w - 2) / ancho;
  if (por_fila < 1) por_fila = 1;
  for (int i = 0; i < NUM_SERES; i++) {
    int fx = x0 + 1 + (i % por_fila) * ancho, fy = y0 + 1 + i / por_fila;
    if (fy >= y0 + h - 1) break;
    Ser* s = &E.seres[i];
    char b[40];
    snprintf(b, sizeof b, "%2d %-8s%4zu", i + 1, s->nombre, s->saber.n);
    texto(fx, fy, ancho - 1, b, i == elegido ? C_BLANCO : COLOR_TRIBU[s->tribu], i == elegido || actividad[i] > 0);
  }
}

static int cursor_x, cursor_y;

/* Compone el cuadro completo en `pantalla` para un tamaño dado. */
static void dibujar_en(int w, int h) {
  if (w != W || h != H || !pantalla) {
    borrar_terminal();
    free(pantalla);
    W = w, H = h;
    pantalla = xmalloc(sizeof(Celda) * (size_t)(W * H));
  }
  limpiar_pantalla();
  if (W < 50 || H < 16) {
    texto(0, 0, W, "Agranda la terminal (mínimo 50×16).", C_ROJO, 1);
    cursor_x = 0, cursor_y = 1;
    return;
  }
  /* cabecera */
  char cab[256];
  snprintf(cab, sizeof cab, " ESPECIE ABLA ");
  texto(0, 0, W, cab, C_BLANCO, 1);
  if (W >= 100)
    snprintf(cab, sizeof cab, "ciclo %llu · %d palabras · %llu frases Abla · %llu data C · 1 ciclo cada %.1f s",
             (unsigned long long)E.tick, idioma_tamano(), (unsigned long long)E.mensajes_abla,
             (unsigned long long)E.mensajes_c, E.ritmo / 1000.0);
  else
    snprintf(cab, sizeof cab, "ciclo %llu · %d palabras · %llu mensajes", (unsigned long long)E.tick, idioma_tamano(),
             (unsigned long long)(E.mensajes_abla + E.mensajes_c));
  texto(15, 0, W - 15, cab, C_TENUE, 0);

  int abajo = 2; /* entrada + ayuda */
  int alto_seres = 0;
  int por_fila = (W - 2) / 16;
  if (por_fila < 1) por_fila = 1;
  int filas_seres = (NUM_SERES + por_fila - 1) / por_fila;
  if (H >= 26 + filas_seres) alto_seres = filas_seres + 2;
  int alto = H - 1 - abajo - alto_seres;
  int con_red = W >= 96;
  int ancho_red = con_red ? W * 9 / 20 : 0;
  if (con_red) dibujar_red(0, 1, ancho_red, alto);
  int px = ancho_red, pw = W - ancho_red;

  char titulo[128] = "";
  for (int i = 0; i < NPANELES; i++) {
    char t[40];
    snprintf(t, sizeof t, i == panel ? "[%s]" : " %s ", NOMBRE_PANEL[i]);
    strcat(titulo, t);
  }
  caja(px, 1, pw, alto, NULL, 1);
  texto(px + 2, 1, pw - 4, titulo, C_BLANCO, 1);
  texto(px + pw - 13 > px ? px + pw - 13 : px, 1, 11, " Tab: panel ", C_TENUE, 0);
  if (panel == P_PENSAMIENTOS) dibujar_pensamientos(px, 1, pw, alto);
  else if (panel == P_SER) dibujar_ser(px, 1, pw, alto);
  else dibujar_salida(px, 1, pw, alto);

  if (alto_seres) dibujar_seres(0, 1 + alto, W, alto_seres);

  /* línea de órdenes */
  int yin = H - 2;
  const char* p = abla_prompt();
  int ap = texto(0, yin, W, p, C_LOGICA, 1);
  int libre = W - ap - 1;
  const char* e = entrada;
  int ancho_e = ancho_utf8(entrada);
  while (ancho_e > libre) {
    e += largo_utf8((unsigned char)*e);
    ancho_e--;
  }
  texto(ap, yin, libre, e, C_NORMAL, 0);
  texto(0, H - 1, W,
        W >= 100 ? "Tab panel · ←→ elegir ser · ↑↓ historial · RePág/AvPág desplazar · ritmo 500 · ayuda · salir"
                 : "Tab panel · ←→ ser · ↑↓ historial · ayuda · salir",
        C_TENUE, 0);
  cursor_x = ap + ancho_e, cursor_y = yin;
}

/* ---------- teclado ---------- */
static void ejecutar_linea(int* salir) {
  entrada[lentrada] = 0;
  char* c = entrada;
  while (*c == ' ') c++;
  if (*c) {
    if (nhist == HIST) {
      free(historial[0]);
      memmove(historial, historial + 1, sizeof(char*) * (HIST - 1));
      nhist--;
    }
    historial[nhist++] = xstrdup(c);
  }
  poshist = nhist;
  if (!strcmp(c, "salir") || !strcmp(c, "exit")) {
#if TERMINAL_CRUDA
    *salir = 1;
#else
    anadir_salida("En el navegador no hace falta salir: su memoria se guarda sola.", C_TENUE);
    panel = P_SALIDA;
#endif
  }
  else if (!strcmp(c, "clear")) {
    for (int i = 0; i < nsalida; i++) free(salida[i]);
    nsalida = 0;
  } else if (*c) {
    char eco[1200];
    snprintf(eco, sizeof eco, "%s%s", abla_prompt(), c);
    anadir_salida(eco, C_LOGICA);
    Texto out;
    tx_iniciar(&out);
    tx_add(&out, abla_ejecutar(c));
    for (char* p = out.p; p && *p;) {
      char* fin = strchr(p, '\n');
      if (fin) *fin = 0;
      anadir_salida(p, C_NORMAL);
      p = fin ? fin + 1 : NULL;
    }
    tx_liberar(&out);
    anadir_salida("", C_NORMAL);
    panel = P_SALIDA;
    desplazar = 0;
  }
  lentrada = 0;
  entrada[0] = 0;
}

static void poner_entrada(const char* s) {
  snprintf(entrada, sizeof entrada, "%s", s);
  lentrada = strlen(entrada);
}

static void tecla(const unsigned char* b, int n, int* i, int* salir) {
  unsigned char k = b[*i];
  if (k == 27) { /* secuencias de escape: flechas, RePág, AvPág */
    if (*i + 2 < n && b[*i + 1] == '[') {
      unsigned char x = b[*i + 2];
      *i += 2;
      if (x == 'A') { /* arriba: historial */
        if (poshist > 0) poner_entrada(historial[--poshist]);
      } else if (x == 'B') {
        if (poshist < nhist - 1) poner_entrada(historial[++poshist]);
        else {
          poshist = nhist;
          poner_entrada("");
        }
      } else if (x == 'C' || x == 'D') { /* derecha / izquierda: elegir ser */
        elegido = (elegido + (x == 'C' ? 1 : NUM_SERES - 1)) % NUM_SERES;
        panel = P_SER;
      } else if ((x == '5' || x == '6') && *i + 1 < n && b[*i + 1] == '~') {
        (*i)++;
        desplazar += x == '5' ? 10 : -10;
        if (desplazar < 0) desplazar = 0;
      }
    } else {
      poner_entrada(""); /* Esc: borra la línea */
    }
  } else if (k == 3 || (k == 4 && !lentrada)) { /* Ctrl+C, o Ctrl+D con la línea vacía */
    *salir = TERMINAL_CRUDA;
  } else if (k == '\t') {
    panel = (panel + 1) % NPANELES;
  } else if (k == '\r' || k == '\n') {
    ejecutar_linea(salir);
  } else if (k == 127 || k == 8) { /* borrar un carácter UTF-8 completo */
    while (lentrada > 0) {
      unsigned char c = (unsigned char)entrada[--lentrada];
      if ((c & 0xC0) != 0x80) break;
    }
    entrada[lentrada] = 0;
  } else if (k == 12) { /* Ctrl+L: redibujar todo */
    borrar_terminal();
  } else if (k >= 32 && lentrada + 1 < sizeof entrada) {
    entrada[lentrada++] = (char)k;
    entrada[lentrada] = 0;
  }
}

/* Avanza lo que se mueve: mensajes en camino y brillo de las mentes. */
static void animar(void) {
  leer_novedades();
  for (int k = 0; k < npart; k++) part[k].t += 0.18f;
  int j = 0;
  for (int k = 0; k < npart; k++)
    if (part[k].t < 1) part[j++] = part[k];
    else actividad[part[k].para] = actividad[part[k].para] > 2 ? actividad[part[k].para] : 2;
  npart = j;
  for (int k = 0; k < NUM_SERES; k++)
    if (actividad[k] > 0) actividad[k]--;
}

#if TERMINAL_CRUDA
static void dibujar(void) {
  int w, h;
  tamano(&w, &h);
  dibujar_en(w, h);
  volcar(cursor_x, cursor_y);
}

int tui_ejecutar(int color) {
  if (!isatty(STDIN_FILENO) || !isatty(STDOUT_FILENO) || !activar_cruda()) return 0;
  usar_color = color;
  visto_red = E.nred;
  visto_linea = E.nlinea > 30 ? E.nlinea - 30 : 0;
  double ultimo_ciclo = ahora_ms(), ultimo_dibujo = 0;
  int salir = 0, cambio = 1;
  while (!salir) {
    fd_set fds;
    FD_ZERO(&fds);
    FD_SET(STDIN_FILENO, &fds);
    struct timeval tv = {0, 40000};
    if (select(STDIN_FILENO + 1, &fds, NULL, NULL, &tv) > 0) {
      unsigned char b[256];
      ssize_t n = read(STDIN_FILENO, b, sizeof b);
      for (int i = 0; i < n && !salir; i++) tecla(b, (int)n, &i, &salir);
      cambio = 1;
    }
    /* pensar: todos los ciclos que tocan según el ritmo */
    double t = ahora_ms();
    int ciclos = (int)((t - ultimo_ciclo) / E.ritmo);
    if (ciclos > 0) {
      if (ciclos > 50) ciclos = 50, ultimo_ciclo = t - E.ritmo;
      for (int k = 0; k < ciclos; k++) abla_ciclo();
      ultimo_ciclo += ciclos * (double)E.ritmo;
      cambio = 1;
    }
    /* dibujar (unas 8 veces por segundo como mucho) */
    if (t - ultimo_dibujo >= 120 && (cambio || npart)) {
      animar();
      dibujar();
      ultimo_dibujo = t;
      cambio = 0;
    }
  }
  restaurar();
  return 1;
}
#else /* sin terminal cruda: la consola de líneas se encarga */
int tui_ejecutar(int color) {
  (void)color;
  return 0;
}
#endif

#if defined(__wasm__)
/* ---------- la misma interfaz, mostrada por una página web ---------- */
static Texto html;
static int tui_web_lista = 0;

static void tui_web_iniciar(void) {
  if (tui_web_lista) return;
  tui_web_lista = 1;
  visto_red = E.nred;
  visto_linea = E.nlinea > 30 ? E.nlinea - 30 : 0;
}

/* Teclas tal como llegarían de una terminal (flechas = ESC [ A…). */
EXPORTA("abla_tui_teclas") void abla_tui_teclas(const char* s) {
  tui_web_iniciar();
  int n = (int)strlen(s), salir = 0;
  for (int i = 0; i < n; i++) tecla((const unsigned char*)s, n, &i, &salir);
}

static void html_texto(const char* s) {
  for (; *s; s++) {
    if (*s == '<') tx_add(&html, "&lt;");
    else if (*s == '>') tx_add(&html, "&gt;");
    else if (*s == '&') tx_add(&html, "&amp;");
    else tx_addn(&html, s, 1);
  }
}

/* Devuelve el cuadro como HTML: filas de <span class="cN"> con el cursor marcado. */
EXPORTA("abla_tui_html") const char* abla_tui_html(int w, int h) {
  tui_web_iniciar();
  animar();
  dibujar_en(w, h);
  tx_vaciar(&html);
  tx_add(&html, "");
  for (int y = 0; y < H; y++) {
    int color = -1, negrita = -1, abierto = 0;
    for (int x = 0; x < W; x++) {
      Celda* c = &pantalla[y * W + x];
      int es_cursor = x == cursor_x && y == cursor_y;
      if (es_cursor || c->color != color || c->negrita != negrita) {
        if (abierto) tx_add(&html, "</span>");
        tx_printf(&html, "<span class=\"c%d%s%s\">", c->color, c->negrita ? " b" : "", es_cursor ? " cursor" : "");
        abierto = 1;
        color = es_cursor ? -1 : c->color;
        negrita = es_cursor ? -1 : c->negrita;
      }
      html_texto(c->c);
    }
    if (abierto) tx_add(&html, "</span>");
    if (y + 1 < H) tx_add(&html, "\n");
  }
  return html.p;
}
#endif
