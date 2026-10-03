/* Misiones de matemáticas.
 *
 * Cada problema lo resuelve la especie en equipo, una fase por ciclo:
 *   1. LEER       un ser del lenguaje lee el problema y lo cuenta en Abla
 *   2. CALCULAR   tres seres de datos lo calculan cada uno por su cuenta
 *   3. VERIFICAR  dos seres de lógica comprueban cada resultado con la operación inversa
 *   4. DECIDIR    votan: gana la respuesta con más acuerdos y comprobaciones
 *   5. RESULTADO  el mundo dice si está bien; todos los que participaron aprenden
 *
 * Al principio se equivocan a menudo: cada ser tiene una habilidad por operación
 * que crece con la práctica (más si acierta, algo menos si se equivoca y lo ve).
 * Con suficientes aciertos pasan de nivel y consiguen premios, como ver con más
 * píxeles. La aritmética "verdadera" la hace el programa; las equivocaciones y el
 * aprendizaje de los seres son una simulación. */
#include "misiones.h"

Misiones M;

static const char* OPS[NOPS] = {"suma", "resta", "multiplicación", "división", "potencia", "ecuación", "primos", "sucesión"};
const char* op_nombre(int op) { return OPS[op]; }

static const char* FASES[NFASES] = {"leer", "calcular", "verificar", "decidir", "resultado"};
const char* fase_nombre(int f) { return FASES[f]; }

static const struct {
  const char* nombre;
  unsigned ops; /* operaciones posibles en este nivel (bits) */
} NIVELES[] = {
    {"Sumar con los dedos", 1u << OP_SUMA},
    {"Sumas grandes", 1u << OP_SUMA},
    {"Restar", 1u << OP_RESTA | 1u << OP_SUMA},
    {"Multiplicar", 1u << OP_MULT},
    {"Dividir", 1u << OP_DIV | 1u << OP_MULT},
    {"Potencias", 1u << OP_POT | 1u << OP_MULT},
    {"Ecuaciones", 1u << OP_ECUACION},
    {"Números primos", 1u << OP_PRIMO},
    {"Sucesiones", 1u << OP_SUCESION},
    {"Mezcla de todo", 0xFFu},
};
#define NNIVELES (int)(sizeof NIVELES / sizeof NIVELES[0])

const char* nivel_nombre(int nivel) { return nivel <= NNIVELES ? NIVELES[nivel - 1].nombre : "Maestría"; }

/* Cuántos píxeles ven según su nivel: el premio de cada nivel es ver mejor. */
void agudeza_de_nivel(int nivel, int* w, int* h) {
  static const int A[][2] = {{64, 48}, {96, 72}, {128, 96}, {160, 120}, {192, 144}, {256, 192}, {320, 240}};
  int i = nivel - 1;
  if (i < 0) i = 0;
  if (i > 6) i = 6;
  *w = A[i][0];
  *h = A[i][1];
}

static long rango(long a, long b) { return a + azar((int)(b - a + 1)); }

static int es_primo(long n) {
  if (n < 2) return 0;
  for (long d = 2; d * d <= n; d++)
    if (n % d == 0) return 0;
  return 1;
}

static void ruta(char* out, const char* rel) { ruta_unir(out, RUTA_MAX, E.mundo, rel); }

/* ---------- problemas ---------- */
static void nuevo_problema(void) {
  int nivel = M.nivel, escala = nivel > NNIVELES ? nivel - NNIVELES + 1 : 1;
  unsigned ops = NIVELES[(nivel <= NNIVELES ? nivel : NNIVELES) - 1].ops;
  int op;
  do op = azar(NOPS);
  while (!(ops >> op & 1u));
  long a = 0, b = 0, c = 0;
  M.del_humano = 0;
  if (M.humano_pendiente) { /* el humano les ha puesto un problema */
    op = M.humano_op, a = M.humano_a, b = M.humano_b;
    M.humano_pendiente = 0;
    M.del_humano = 1;
  }
  M.op = op;
  switch (op) {
    case OP_SUMA:
      if (!M.del_humano) {
        if (nivel == 1) a = rango(0, 10), b = rango(0, 10);
        else a = rango(10, 99 * escala), b = rango(10, 99 * escala);
      }
      M.respuesta = a + b;
      snprintf(M.enunciado, sizeof M.enunciado, "%ld + %ld", a, b);
      break;
    case OP_RESTA:
      if (!M.del_humano) a = rango(10, 100 * escala), b = rango(0, a);
      M.respuesta = a - b;
      snprintf(M.enunciado, sizeof M.enunciado, "%ld − %ld", a, b);
      break;
    case OP_MULT:
      if (!M.del_humano) a = rango(2, nivel >= 6 ? 25 * escala : 12), b = rango(2, 12);
      M.respuesta = a * b;
      snprintf(M.enunciado, sizeof M.enunciado, "%ld × %ld", a, b);
      break;
    case OP_DIV:
      if (!M.del_humano) b = rango(2, 12), a = b * rango(2, 12 * escala);
      if (!b) b = 1;
      M.respuesta = a / b;
      snprintf(M.enunciado, sizeof M.enunciado, "%ld ÷ %ld", a, b);
      break;
    case OP_POT:
      if (!M.del_humano) a = rango(2, 6), b = rango(2, a <= 3 ? 5 : 3);
      M.respuesta = 1;
      for (long i = 0; i < b && i < 40; i++) M.respuesta *= a;
      snprintf(M.enunciado, sizeof M.enunciado, "%ld elevado a %ld", a, b);
      break;
    case OP_ECUACION: { /* a·x + b = c */
      long x = rango(1, 12 * escala);
      a = rango(2, 9), b = rango(0, 20), c = a * x + b;
      M.respuesta = x;
      snprintf(M.enunciado, sizeof M.enunciado, "%ld·x + %ld = %ld   ¿x?", a, b, c);
      break;
    }
    case OP_PRIMO: { /* el siguiente primo después de n */
      a = rango(2, 60 * escala);
      long p = a + 1;
      while (!es_primo(p)) p++;
      M.respuesta = p;
      snprintf(M.enunciado, sizeof M.enunciado, "primer número primo mayor que %ld", a);
      break;
    }
    default: { /* sucesión aritmética o geométrica: el quinto término */
      long t0 = rango(1, 9), r = rango(2, 5);
      int geo = azar(2);
      long t[5];
      t[0] = t0;
      for (int i = 1; i < 5; i++) t[i] = geo ? t[i - 1] * r : t[i - 1] + r * escala;
      M.respuesta = t[4];
      snprintf(M.enunciado, sizeof M.enunciado, "%ld, %ld, %ld, %ld, ¿?", t[0], t[1], t[2], t[3]);
      break;
    }
  }
  M.fase = FASE_LEER;
  M.reintentos = 0;
}

/* ---------- equipo ---------- */
static int de_tribu(int tribu, const int* evitar, int n) {
  for (int intento = 0; intento < 50; intento++) {
    int id = tribu * 9 + azar(9), repetido = 0;
    for (int k = 0; k < n; k++) repetido |= evitar[k] == id;
    if (!repetido) return id;
  }
  return tribu * 9;
}

/* Un ser calcula: acierta según su habilidad; si no, se equivoca un poco. */
static long calcular(int ser) {
  float p = M.habilidad[ser][M.op];
  if (probabilidad(p)) return M.respuesta;
  static const long DESVIO[] = {1, -1, 2, -2, 10, -10};
  long r = M.respuesta + DESVIO[azar(6)];
  return r == M.respuesta ? r + 1 : r;
}

static void aprender(int ser, float cuanto) {
  float* h = &M.habilidad[ser][M.op];
  *h += (1.0f - *h) * cuanto;
  if (*h > 0.98f) *h = 0.98f;
}

static void historial(const char* texto, int bien) {
  if (M.nhist == MAX_HIST) {
    memmove(M.hist, M.hist + 1, sizeof M.hist[0] * (MAX_HIST - 1));
    M.nhist--;
  }
  snprintf(M.hist[M.nhist].texto, sizeof M.hist[0].texto, "%s", texto);
  M.hist[M.nhist++].bien = bien;
  char r[RUTA_MAX], linea[256];
  ruta(r, "misiones/registro.txt");
  snprintf(linea, sizeof linea, "%llu\tnivel %d\t%s\t%s\n", (unsigned long long)E.tick, M.nivel, bien ? "bien" : "mal", texto);
  arch_escribir(r, linea, strlen(linea), 1);
}

static void logro(const char* texto) {
  if (M.nlogros == MAX_LOGROS) {
    memmove(M.logros, M.logros + 1, sizeof M.logros[0] * (MAX_LOGROS - 1));
    M.nlogros--;
  }
  snprintf(M.logros[M.nlogros++], sizeof M.logros[0], "%s", texto);
}

static void subir_nivel(void) {
  int aw, ah, nw, nh;
  agudeza_de_nivel(M.nivel, &aw, &ah);
  M.nivel++;
  M.aciertos = 0;
  M.meta = 6 + M.nivel < 15 ? 6 + M.nivel : 15;
  agudeza_de_nivel(M.nivel, &nw, &nh);
  char t[256];
  if (nw != aw) snprintf(t, sizeof t, "Nivel %d: %s. Premio: ahora ven a %d×%d píxeles.", M.nivel, nivel_nombre(M.nivel), nw, nh);
  else snprintf(t, sizeof t, "Nivel %d: %s. Premio: más práctica, números más grandes.", M.nivel, nivel_nombre(M.nivel));
  logro(t);
  if (M.nivel == 4) { /* premio extra: palabras nuevas para las matemáticas */
    static const char* PARES[][2] = {{"suma", "número"}, {"resta", "número"}, {"función", "variable"}, {"número", "orden"}};
    for (int i = 0; i < 4; i++) {
      int a = idioma_de_espanol(PARES[i][0]), b = idioma_de_espanol(PARES[i][1]);
      int nueva = idioma_acunar(a, b);
      if (nueva >= 0)
        for (int k = 0; k < NUM_SERES; k++) E.seres[k].conoce[nueva / 8] |= (uint8_t)(1u << (nueva % 8));
    }
    logro("Premio extra: cuatro palabras nuevas de matemáticas en Abla.");
  }
  M.celebrar_hasta = E.tick + 12;
  for (int k = 0; k < NUM_SERES; k++)
    anotarf(&E.seres[k], k == 0, "¡Subimos al nivel %d (%s)! %s", M.nivel, nivel_nombre(M.nivel),
            nw != aw ? "Ahora vemos con más detalle." : "");
}

/* ---------- una fase por ciclo ---------- */
void misiones_paso(void) {
  if (!M.nivel) misiones_iniciar();
  switch (M.fase) {
    case FASE_LEER: {
      nuevo_problema();
      M.lector = de_tribu(0, NULL, 0);
      Ser* s = &E.seres[M.lector];
      int f[3] = {idioma_de_espanol("número"), idioma_de_espanol(M.op == OP_RESTA ? "resta" : "suma"), idioma_de_espanol("pregunta")};
      Texto abla;
      tx_iniciar(&abla);
      idioma_frase(f, 3, &abla);
      snprintf(M.paso, sizeof M.paso, "%s lee el problema%s: %s  («%s»)", s->nombre, M.del_humano ? " del humano" : "", M.enunciado,
               abla.p);
      anotarf(s, M.del_humano, "Misión (nivel %d, %s): %s. Se lo cuento a la tribu de los datos.", M.nivel, op_nombre(M.op),
              M.enunciado);
      tx_liberar(&abla);
      M.fase = FASE_CALCULAR;
      break;
    }
    case FASE_CALCULAR: {
      int usados[3];
      for (int i = 0; i < 3; i++) {
        M.calc[i] = de_tribu(1, usados, i);
        usados[i] = M.calc[i];
        M.propuesta[i] = calcular(M.calc[i]);
        M.aprobada[i][0] = M.aprobada[i][1] = -1;
        anotarf(&E.seres[M.calc[i]], 0, "Calculo %s y me da %ld.", M.enunciado, M.propuesta[i]);
      }
      snprintf(M.paso, sizeof M.paso, "Calculan: %s dice %ld, %s dice %ld, %s dice %ld", E.seres[M.calc[0]].nombre, M.propuesta[0],
               E.seres[M.calc[1]].nombre, M.propuesta[1], E.seres[M.calc[2]].nombre, M.propuesta[2]);
      M.fase = FASE_VERIFICAR;
      break;
    }
    case FASE_VERIFICAR: {
      M.verif[0] = de_tribu(2, NULL, 0);
      M.verif[1] = de_tribu(2, M.verif, 1);
      int aprobadas = 0;
      for (int v = 0; v < 2; v++) {
        float q = M.habilidad[M.verif[v]][M.op];
        for (int i = 0; i < 3; i++) {
          int correcta = M.propuesta[i] == M.respuesta;
          /* comprueba con la operación inversa: acierta al juzgar según su habilidad */
          M.aprobada[i][v] = probabilidad(q) ? correcta : !correcta;
          aprobadas += M.aprobada[i][v];
        }
        anotarf(&E.seres[M.verif[v]], 0, "Compruebo %s con la operación inversa: apruebo %s%s%s.", M.enunciado,
                M.aprobada[0][v] ? "la 1ª " : "", M.aprobada[1][v] ? "la 2ª " : "", M.aprobada[2][v] ? "la 3ª" : "");
      }
      snprintf(M.paso, sizeof M.paso, "%s y %s comprueban los resultados (%d aprobaciones)", E.seres[M.verif[0]].nombre,
               E.seres[M.verif[1]].nombre, aprobadas);
      M.fase = FASE_DECIDIR;
      break;
    }
    case FASE_DECIDIR: {
      int mejor = 0, puntos_mejor = -1, aprobada_mejor = 0;
      for (int i = 0; i < 3; i++) {
        int puntos = 0, aprob = 0;
        for (int j = 0; j < 3; j++) puntos += M.propuesta[j] == M.propuesta[i]; /* acuerdo entre calculadores */
        for (int v = 0; v < 2; v++) aprob += M.aprobada[i][v] == 1;
        puntos += 2 * aprob;
        if (puntos > puntos_mejor) mejor = i, puntos_mejor = puntos, aprobada_mejor = aprob;
      }
      if (!aprobada_mejor && M.reintentos < 2) { /* nadie convence: vuelven a calcular */
        M.reintentos++;
        snprintf(M.paso, sizeof M.paso, "Ninguna respuesta convence a la lógica. Vuelven a calcular (intento %d).", M.reintentos + 1);
        M.fase = FASE_CALCULAR;
        break;
      }
      M.decision = M.propuesta[mejor];
      snprintf(M.paso, sizeof M.paso, "Votan: la especie responde %ld", M.decision);
      M.fase = FASE_RESULTADO;
      break;
    }
    default: { /* RESULTADO */
      int bien = M.decision == M.respuesta;
      int participantes[6] = {M.calc[0], M.calc[1], M.calc[2], M.verif[0], M.verif[1], M.lector};
      for (int i = 0; i < 6; i++) aprender(participantes[i], bien ? 0.12f : 0.06f);
      char t[160];
      if (bien) {
        M.resueltos++;
        if (!M.del_humano) M.aciertos++;
        snprintf(t, sizeof t, "%s = %ld ✓", M.enunciado, M.decision);
        snprintf(M.paso, sizeof M.paso, "¡Bien! %s = %ld", M.enunciado, M.decision);
      } else {
        M.fallados++;
        snprintf(t, sizeof t, "%s: dijeron %ld, era %ld ✗", M.enunciado, M.decision, M.respuesta);
        snprintf(M.paso, sizeof M.paso, "Fallaron: dijeron %ld, era %ld. Aprenden del error.", M.decision, M.respuesta);
      }
      historial(t, bien);
      anotarf(&E.seres[M.lector], 1, bien ? "Misión cumplida: %s. Llevamos %d de %d en el nivel %d." : "Nos equivocamos: %s. (%d de %d en el nivel %d)",
              t, M.aciertos, M.meta, M.nivel);
      if (M.aciertos >= M.meta) subir_nivel();
      M.fase = FASE_LEER;
      break;
    }
  }
}

/* ---------- el humano les pone un problema ---------- */
int misiones_problema_humano(const char* expr, Texto* out) {
  long a, b;
  char op;
  if (sscanf(expr, " %ld %c %ld", &a, &op, &b) != 3) {
    tx_add(out, "uso: mision 37*12   (operaciones: + - * x / ^)\n");
    return 0;
  }
  int o = op == '+' ? OP_SUMA : op == '-' ? OP_RESTA : (op == '*' || op == 'x') ? OP_MULT : op == '/' ? OP_DIV : op == '^' ? OP_POT : -1;
  if (o < 0 || (o == OP_DIV && b == 0) || (o == OP_POT && (b < 0 || b > 30))) {
    tx_add(out, "Esa operación no la entienden (usa + - * x / ^, sin dividir entre 0).\n");
    return 0;
  }
  if (o == OP_DIV && a % b) {
    tx_add(out, "De momento solo saben divisiones exactas.\n");
    return 0;
  }
  M.humano_a = a, M.humano_b = b, M.humano_op = o, M.humano_pendiente = 1;
  if (M.fase != FASE_LEER) tx_add(out, "Lo resolverán en cuanto terminen el problema que tienen entre manos.\n");
  else tx_add(out, "Se ponen a resolverlo ahora mismo.\n");
  return 1;
}

/* ---------- guardar y describir ---------- */
void misiones_guardar(void) {
  char r[RUTA_MAX];
  Texto t;
  tx_iniciar(&t);
  tx_printf(&t, "nivel %d %d %lu %lu\n", M.nivel, M.aciertos, M.resueltos, M.fallados);
  for (int s = 0; s < NUM_SERES; s++) {
    tx_printf(&t, "habilidad %d", s);
    for (int o = 0; o < NOPS; o++) tx_printf(&t, " %.4f", (double)M.habilidad[s][o]);
    tx_add(&t, "\n");
  }
  for (int i = 0; i < M.nlogros; i++) tx_printf(&t, "logro %s\n", M.logros[i]);
  ruta(r, "misiones/estado.txt");
  arch_escribir(r, t.p, t.n, 0);
  tx_liberar(&t);
}

void misiones_iniciar(void) {
  memset(&M, 0, sizeof M);
  M.nivel = 1;
  M.meta = 7;
  /* habilidades de nacimiento: datos calcula mejor, lógica comprueba mejor */
  static const float BASE_OP[NOPS] = {0.70f, 0.60f, 0.45f, 0.40f, 0.30f, 0.25f, 0.30f, 0.30f};
  for (int s = 0; s < NUM_SERES; s++)
    for (int o = 0; o < NOPS; o++) {
      float b = BASE_OP[o] + (E.seres[s].tribu == DATOS ? 0.05f : E.seres[s].tribu == LOGICA ? 0.08f : -0.15f);
      M.habilidad[s][o] = b + (float)azar(100) / 1000.0f;
    }
  char r[RUTA_MAX];
  ruta(r, "misiones");
  arch_crear_directorios(r);
  ruta(r, "misiones/estado.txt");
  Texto t;
  tx_iniciar(&t);
  if (arch_leer(r, &t) && t.p) {
    for (char* p = t.p; p && *p;) {
      char* fin = strchr(p, '\n');
      if (fin) *fin = 0;
      int s, n;
      if (sscanf(p, "nivel %d %d %lu %lu", &M.nivel, &M.aciertos, &M.resueltos, &M.fallados) >= 2) {
        if (M.nivel < 1) M.nivel = 1;
      } else if (sscanf(p, "habilidad %d%n", &s, &n) == 1 && s >= 0 && s < NUM_SERES) {
        char* q = p + n;
        for (int o = 0; o < NOPS; o++) M.habilidad[s][o] = strtof(q, &q);
      } else if (!strncmp(p, "logro ", 6)) {
        logro(p + 6);
      }
      p = fin ? fin + 1 : NULL;
    }
    M.meta = 6 + M.nivel < 15 ? 6 + M.nivel : 15;
  }
  tx_liberar(&t);
  if (!M.nlogros) logro("Nivel 1: Sumar con los dedos. Ven a 64×48 píxeles.");
  snprintf(M.paso, sizeof M.paso, "Preparando la primera misión…");
}

void misiones_describir(Texto* t) {
  int w, h;
  agudeza_de_nivel(M.nivel, &w, &h);
  tx_printf(t, "Nivel %d: %s · %d de %d aciertos para subir · visión %d×%d\n", M.nivel, nivel_nombre(M.nivel), M.aciertos, M.meta, w, h);
  tx_printf(t, "Problema: %s   (fase: %s)\n%s\n", M.enunciado[0] ? M.enunciado : "—", fase_nombre(M.fase), M.paso);
  tx_printf(t, "Total: %lu bien, %lu mal\n", M.resueltos, M.fallados);
  for (int i = 0; i < M.nhist; i++) tx_printf(t, "  %s\n", M.hist[i].texto);
  tx_add(t, "Habilidad media de la especie: ");
  for (int o = 0; o < NOPS; o++) {
    float m = 0;
    for (int s = 0; s < NUM_SERES; s++) m += M.habilidad[s][o];
    tx_printf(t, "%s %d%%%s", op_nombre(o), (int)(m / NUM_SERES * 100), o + 1 < NOPS ? " · " : "\n");
  }
  tx_add(t, "Logros:\n");
  for (int i = 0; i < M.nlogros; i++) tx_printf(t, "  ★ %s\n", M.logros[i]);
}
