/* Cómo se le habla a la especie: los comandos de la terminal (que también usa
 * la interfaz web) y unas funciones que devuelven el estado en JSON para la UI. */
#include "especie.h"

static Texto salida;

static const char* AYUDA =
    "\nTerminal (estilo bash, dentro de mundo/):\n"
    "  ls [-l] [ruta]   cd [ruta]   pwd   cat f   head/tail [-n N] f   wc f   grep patrón f\n"
    "  mkdir d   touch f   rm [-r] f   mv a b   cp a b   tree [ruta]   echo texto [> | >> f]\n"
    "  clear   whoami   exit\n\n"
    "La especie (ser = número 1-27 o nombre):\n"
    "  seres                         los 27 y qué están pensando\n"
    "  ser <ser>                     estado: memoria, contexto, saber, mensajes\n"
    "  mente <ser> [n]               sus últimos n pensamientos (contexto)\n"
    "  recordar <ser> <texto>        busca en toda su memoria, también la de largo plazo\n"
    "  escuchar [segundos] [ser]     escucha a la especie pensar\n"
    "  terminal <ser>                lo que el ser ha hecho en su propia terminal\n"
    "  decir <ser|todos> <texto>     háblale en Abla o en español (raíces del diccionario)\n"
    "                                p. ej.  decir 3 sol causa luz pregunta\n"
    "  enseñar <ser|todos> <a> <rel> <b>   manda data C exacta; rel = es|parte|causa|igual|opuesto\n"
    "  alimentar <ser|todos> <archivo>     manda un archivo de líneas «a rel b» como data C\n"
    "  dic <palabra>                 traduce entre Abla y español\n"
    "  dic buscar <texto>            busca en el diccionario\n"
    "  traducir <frase>              traduce una frase de Abla\n"
    "  idioma                        estado del idioma (4000 innatas + creadas)\n"
    "  red                           mensajes entre los seres\n"
    "  guardar                       guarda ahora (también se guarda solo)\n";

static void unir_desde(int argc, char** argv, int desde, Texto* t) {
  for (int i = desde; i < argc; i++) tx_printf(t, "%s%s", i > desde ? " " : "", argv[i]);
  if (!t->p) tx_add(t, "");
}

static void cmd_seres(void) {
  for (int i = 0; i < NUM_SERES; i++) {
    Ser* s = &E.seres[i];
    char corto[220];
    snprintf(corto, sizeof corto, "%s", s->ultimo ? s->ultimo : "");
    if (strlen(corto) > 200) strcpy(corto + 197, "...");
    tx_printf(&salida, "%2d %-9s %-9s %4zu hechos  %s\n", i + 1, s->nombre, nombre_tribu(s->tribu), s->saber.n, corto);
  }
}

static void cmd_ser(Ser* s) {
  int nuevas = 0;
  for (int p = BASE; p < idioma_tamano(); p++) nuevas += conoce_palabra(s, p);
  Memoria* m = &s->memoria;
  tx_printf(&salida,
            "%s — «quien %s», tribu %s\n"
            "  pensamientos:     %llu (no se detiene)\n"
            "  contexto:         %llu / %llu tokens (%zu recuerdos presentes)\n"
            "  largo plazo:      %llu recuerdos, %lld KB en /memoria/%s.mem\n"
            "  conocimiento:     %zu hechos en /conocimiento/%s.c\n"
            "  idioma:           %d innatas + %d nuevas\n"
            "  mensajes:         %llu enviados, %llu recibidos, %d en el buzón\n"
            "  terminal:         /seres/%s\n"
            "  ahora piensa:     %s\n",
            s->nombre, idioma_glosa_tmp(s->concepto), nombre_tribu(s->tribu), (unsigned long long)s->pensamientos,
            (unsigned long long)m->usados, (unsigned long long)m->capacidad, m->n, (unsigned long long)m->total,
            memoria_bytes(m) / 1024, s->nombre, s->saber.n, s->nombre, BASE, nuevas, (unsigned long long)s->enviados,
            (unsigned long long)s->recibidos, s->bn, s->nombre, s->ultimo ? s->ultimo : "");
}

static void cmd_escuchar(int segundos, const char* filtro) {
  uint64_t visto = E.nlinea;
  int ciclos = segundos * 1000 / E.ritmo;
  if (ciclos < 1) ciclos = 1;
  if (ciclos > 400) ciclos = 400;
  for (int i = 0; i < ciclos; i++) abla_ciclo();
  uint64_t desde = E.nlinea > CORRIENTE ? E.nlinea - CORRIENTE : 0;
  if (visto > desde) desde = visto;
  for (uint64_t n = desde + 1; n <= E.nlinea; n++) {
    Linea* l = &E.corriente[(n - 1) % CORRIENTE];
    if (!filtro || strstr(l->texto, filtro)) tx_printf(&salida, "%s\n", l->texto);
  }
}

/* Mueve el último mensaje del humano al frente del buzón: te atiende primero. */
static void atender_primero(Ser* s) {
  for (int k = s->bn - 1; k > 0; k--) {
    int i = (s->bini + k) % BUZON;
    if (s->buzon[i].de != DE_HUMANO) continue;
    Mensaje m = s->buzon[i];
    for (int j = k; j > 0; j--) s->buzon[(s->bini + j) % BUZON] = s->buzon[(s->bini + j - 1) % BUZON];
    s->buzon[s->bini] = m;
    return;
  }
}

static int hecho_de_args(char** x, int n, struct hecho* h) {
  if (n != 3) return 0;
  int s = idioma_de_forma(x[0]), o = idioma_de_forma(x[2]), r = rel_de_nombre(x[1]);
  if (s < 0) s = idioma_de_espanol(x[0]);
  if (o < 0) o = idioma_de_espanol(x[2]);
  if (s < 0 || o < 0 || r < 0) return 0;
  h->s = (uint16_t)s, h->r = (uint8_t)r, h->o = (uint16_t)o, h->confianza = 1.0f, h->fuente = FUENTE_HUMANO;
  return 1;
}

static void cmd_hablar(int argc, char** argv) {
  const char* cmd = argv[0];
  Ser* para[NUM_SERES];
  int np = 0;
  if (argc > 1 && !strcmp(argv[1], "todos"))
    for (int i = 0; i < NUM_SERES; i++) para[np++] = &E.seres[i];
  else if (argc > 1 && buscar_ser(argv[1]))
    para[np++] = buscar_ser(argv[1]);
  if (!np) {
    tx_printf(&salida, "%s: ¿a quién? (1-27, nombre o todos)\n", cmd);
    return;
  }
  if (!strcmp(cmd, "decir")) {
    Texto texto, raras, abla, glosa;
    tx_iniciar(&texto), tx_iniciar(&raras), tx_iniciar(&abla), tx_iniciar(&glosa);
    unir_desde(argc, argv, 2, &texto);
    int f[6];
    int n = idioma_leer(texto.p, f, 6, &raras);
    if (raras.n) tx_printf(&salida, "(no son palabras de Abla ni raíces: %s)\n", raras.p);
    if (n) {
      idioma_frase(f, n, &abla);
      idioma_glosa_frase(f, n, &glosa);
      tx_printf(&salida, "En Abla: %s  («%s»)\n", abla.p, glosa.p);
      Mensaje m;
      memset(&m, 0, sizeof m);
      m.de = DE_HUMANO;
      m.n = n;
      memcpy(m.palabras, f, sizeof(int) * (size_t)n);
      for (int i = 0; i < np; i++) entregar(para[i]->id, &m);
    }
    tx_liberar(&texto), tx_liberar(&raras), tx_liberar(&abla), tx_liberar(&glosa);
    if (!n) return;
  } else {
    int malas = 0, enviados = 0;
    Texto datos;
    tx_iniciar(&datos);
    if (!strcmp(cmd, "alimentar")) {
      char v[RUTA_MAX], r[RUTA_MAX];
      if (argc < 3 || !shell_resolver(&E.yo, argv[2], v, sizeof v) || (shell_real(&E.yo, v, r, sizeof r), !arch_leer(r, &datos))) {
        tx_add(&salida, "alimentar: no se puede leer el archivo\n");
        tx_liberar(&datos);
        return;
      }
    } else {
      unir_desde(argc, argv, 2, &datos);
    }
    for (char* p = datos.p; p && *p;) {
      char* fin = strchr(p, '\n');
      if (fin) *fin = 0;
      char** x;
      int n = trocear(p, &x);
      if (n && x[0][0] != '#') {

        struct hecho h;
        if (hecho_de_args(x, n, &h)) {
          Mensaje m;
          memset(&m, 0, sizeof m);
          m.de = DE_HUMANO;
          m.cpp = 1;
          m.dato = h;
          for (int i = 0; i < np; i++) entregar(para[i]->id, &m);
          enviados++;
        } else {
          malas++;
        }
      }
      liberar_args(n, x);
      p = fin ? fin + 1 : NULL;
    }
    tx_liberar(&datos);
    tx_printf(&salida, "Enviada data C: %d hechos a %d ser(es)", enviados, np);
    if (malas) tx_printf(&salida, " (%d líneas no entendidas; formato: a es|parte|causa|igual|opuesto b)", malas);
    tx_add(&salida, "\n");
    if (!enviados) return;
  }
  if (np == 1) { /* respuesta inmediata */
    Ser* s = para[0];
    free(s->respuesta);
    s->respuesta = NULL;
    atender_primero(s);
    pensar(s);
    tx_printf(&salida, "%s: %s\n", s->nombre, s->respuesta ? s->respuesta : (s->ultimo ? s->ultimo : ""));
  }
}

static void cmd_dic(int argc, char** argv) {
  if (argc > 2 && !strcmp(argv[1], "buscar")) {
    Texto q;
    tx_iniciar(&q);
    unir_desde(argc, argv, 2, &q);
    minusculas(q.p);
    int n = 0;
    for (int i = 0; i < idioma_tamano() && n < 40; i++) {
      const char* g = idioma_glosa_tmp(i);
      if (strstr(g, q.p) || strstr(idioma_forma(i), q.p)) {
        tx_printf(&salida, "  %s\t%s\n", idioma_forma(i), g);
        n++;
      }
    }
    if (!n) tx_add(&salida, "(nada)\n");
    tx_liberar(&q);
  } else if (argc > 1) {
    int i = idioma_de_forma(argv[1]);
    if (i >= 0) tx_printf(&salida, "%s = %s\n", argv[1], idioma_glosa_tmp(i));
    else if ((i = idioma_de_espanol(argv[1])) >= 0) {
      tx_printf(&salida, "%s = %s   (aspectos: ", argv[1], idioma_forma(i));
      for (int k = 1; k < ASPECTOS; k++)
        tx_printf(&salida, "%s=%s%s", idioma_forma(i + k), idioma_aspecto(k), k + 1 < ASPECTOS ? ", " : ")\n");
    } else {
      tx_printf(&salida, "dic: «%s» no está en Abla (prueba: dic buscar %s)\n", argv[1], argv[1]);
    }
  } else {
    tx_add(&salida, "uso: dic <palabra> | dic buscar <texto>\n");
  }
}

EXPORTA("abla_ejecutar") const char* abla_ejecutar(const char* linea) {
  tx_vaciar(&salida);
  tx_add(&salida, "");
  char** argv;
  int argc = trocear(linea, &argv);
  if (!argc) {
    liberar_args(argc, argv);
    return salida.p;
  }
  const char* c = argv[0];
  Ser* s = argc > 1 ? buscar_ser(argv[1]) : NULL;

  if (!strcmp(c, "ayuda") || !strcmp(c, "help")) tx_add(&salida, AYUDA);
  else if (!strcmp(c, "whoami")) tx_add(&salida, "humano\n");
  else if (!strcmp(c, "seres")) cmd_seres();
  else if (!strcmp(c, "ser") || !strcmp(c, "mente") || !strcmp(c, "terminal") || !strcmp(c, "recordar")) {
    if (!s) tx_printf(&salida, "%s: ¿qué ser? (1-27 o nombre)\n", c);
    else if (!strcmp(c, "ser")) cmd_ser(s);
    else if (!strcmp(c, "mente")) {
      size_t n = argc > 2 ? strtoul(argv[2], NULL, 10) : 15;
      size_t total = s->memoria.n;
      for (size_t i = total > n ? total - n : 0; i < total; i++) {
        const Recuerdo* r = memoria_en(&s->memoria, i);
        tx_printf(&salida, "[t=%llu] %s\n", (unsigned long long)r->tick, r->texto);
      }
    } else if (!strcmp(c, "terminal")) {
      if (!s->hn) tx_add(&salida, "(todavía no ha usado su terminal)\n");
      for (int i = 0; i < s->hn; i++) tx_printf(&salida, "%s\n", s->historial[(s->hini + i) % HISTORIAL]);
    } else {
      Texto q;
      tx_iniciar(&q);
      unir_desde(argc, argv, 2, &q);
      size_t antes = salida.n;
      memoria_buscar(&s->memoria, q.p, 20, &salida);
      if (salida.n == antes) tx_add(&salida, "(no lo recuerda)\n");
      tx_liberar(&q);
    }
  } else if (!strcmp(c, "escuchar")) {
    int seg = 10;
    const char* filtro = NULL;
    for (int i = 1; i < argc; i++) {
      if (isdigit((unsigned char)argv[i][0])) seg = atoi(argv[i]);
      else filtro = buscar_ser(argv[i]) ? buscar_ser(argv[i])->nombre : argv[i];
    }
    cmd_escuchar(seg < 1 ? 1 : seg, filtro);
  } else if (!strcmp(c, "decir") || !strcmp(c, "enseñar") || !strcmp(c, "ensenar") || !strcmp(c, "alimentar")) {
    cmd_hablar(argc, argv);
  } else if (!strcmp(c, "dic")) {
    cmd_dic(argc, argv);
  } else if (!strcmp(c, "traducir")) {
    Texto t;
    tx_iniciar(&t);
    unir_desde(argc, argv, 1, &t);
    int f[64];
    int n = idioma_leer(t.p, f, 64, NULL);
    tx_liberar(&t);
    idioma_glosa_frase(f, n, &salida);
    tx_add(&salida, "\n");
  } else if (!strcmp(c, "idioma")) {
    tx_printf(&salida, "Abla: %d palabras = %d innatas (%d raíces × %d aspectos) + %d creadas por la especie\n"
                       "Diccionario completo: /abla/diccionario.txt\n",
              idioma_tamano(), BASE, RAICES, ASPECTOS, idioma_compuestas());
    int desde = idioma_tamano() - 8 > BASE ? idioma_tamano() - 8 : BASE;
    for (int i = desde; i < idioma_tamano(); i++)
      tx_printf(&salida, "  nueva: %s = %s\n", idioma_forma(i), idioma_glosa_tmp(i));
  } else if (!strcmp(c, "red")) {
    tx_printf(&salida, "Ciclo %llu · mensajes en Abla: %llu · data C: %llu\n", (unsigned long long)E.tick,
              (unsigned long long)E.mensajes_abla, (unsigned long long)E.mensajes_c);
  } else if (!strcmp(c, "guardar")) {
    abla_guardar();
    tx_add(&salida, "Guardado: conocimiento en /conocimiento/*.c, memoria en /memoria/\n");
  } else if (!shell_ejecutar(&E.yo, argc, argv, &salida)) {
    tx_printf(&salida, "%s: comando no encontrado (escribe ayuda)\n", c);
  }
  liberar_args(argc, argv);
  return salida.p;
}

EXPORTA("abla_prompt") const char* abla_prompt(void) {
  static char p[RUTA_MAX + 32];
  snprintf(p, sizeof p, "humano@abla:%s$ ", E.yo.cwd);
  return p;
}

/* ---------- API para la interfaz web (JSON) ---------- */

static Texto json;

EXPORTA("abla_estado_json") const char* abla_estado_json(void) {
  tx_vaciar(&json);
  tx_printf(&json, "{\"tick\":%llu,\"palabras\":%d,\"compuestas\":%d,\"abla\":%llu,\"c\":%llu,\"ritmo\":%d,\"seres\":[",
            (unsigned long long)E.tick, idioma_tamano(), idioma_compuestas(), (unsigned long long)E.mensajes_abla,
            (unsigned long long)E.mensajes_c, E.ritmo);
  for (int i = 0; i < NUM_SERES; i++) {
    Ser* s = &E.seres[i];
    int nuevas = 0;
    for (int p = BASE; p < idioma_tamano(); p++) nuevas += conoce_palabra(s, p);
    tx_printf(&json, "%s{\"id\":%d,\"nombre\":", i ? "," : "", i);
    tx_json(&json, s->nombre);
    tx_printf(&json, ",\"tribu\":%d,\"concepto\":", s->tribu);
    tx_json(&json, idioma_glosa_tmp(s->concepto));
    tx_printf(&json,
              ",\"hechos\":%zu,\"pensamientos\":%llu,\"enviados\":%llu,\"recibidos\":%llu,\"nuevas\":%d,"
              "\"contexto\":%llu,\"capacidad\":%llu,\"largo\":%llu,\"ultimo\":",
              s->saber.n, (unsigned long long)s->pensamientos, (unsigned long long)s->enviados,
              (unsigned long long)s->recibidos, nuevas, (unsigned long long)s->memoria.usados,
              (unsigned long long)s->memoria.capacidad, (unsigned long long)s->memoria.total);
    tx_json(&json, s->ultimo ? s->ultimo : "");
    tx_add(&json, "}");
  }
  tx_add(&json, "]}");
  return json.p;
}

EXPORTA("abla_corriente_json") const char* abla_corriente_json(double desde_d) {
  uint64_t desde = (uint64_t)desde_d;
  tx_vaciar(&json);
  tx_printf(&json, "{\"hasta\":%llu,\"lineas\":[", (unsigned long long)E.nlinea);
  uint64_t min = E.nlinea > CORRIENTE ? E.nlinea - CORRIENTE : 0;
  if (desde < min) desde = min;
  int primero = 1;
  for (uint64_t n = desde + 1; n <= E.nlinea; n++) {
    Linea* l = &E.corriente[(n - 1) % CORRIENTE];
    tx_printf(&json, "%s{\"n\":%llu,\"ser\":%d,\"texto\":", primero ? "" : ",", (unsigned long long)l->n, l->ser);
    tx_json(&json, l->texto);
    tx_add(&json, "}");
    primero = 0;
  }
  tx_add(&json, "]}");
  return json.p;
}

EXPORTA("abla_red_json") const char* abla_red_json(double desde_d) {
  uint64_t desde = (uint64_t)desde_d;
  tx_vaciar(&json);
  tx_printf(&json, "{\"hasta\":%llu,\"m\":[", (unsigned long long)E.nred);
  uint64_t min = E.nred > RED ? E.nred - RED : 0;
  if (desde < min) desde = min;
  for (uint64_t n = desde; n < E.nred; n++)
    tx_printf(&json, "%s[%d,%d,%d]", n > desde ? "," : "", E.red[n % RED].de, E.red[n % RED].para, E.red[n % RED].c);
  tx_add(&json, "]}");
  return json.p;
}

EXPORTA("abla_mente_json") const char* abla_mente_json(int id, int n) {
  tx_vaciar(&json);
  tx_add(&json, "[");
  if (id >= 0 && id < NUM_SERES) {
    Memoria* m = &E.seres[id].memoria;
    size_t total = m->n, desde = total > (size_t)n ? total - (size_t)n : 0;
    for (size_t i = desde; i < total; i++) {
      const Recuerdo* r = memoria_en(m, i);
      tx_printf(&json, "%s{\"t\":%llu,\"x\":", i > desde ? "," : "", (unsigned long long)r->tick);
      tx_json(&json, r->texto);
      tx_add(&json, "}");
    }
  }
  tx_add(&json, "]");
  return json.p;
}

#if defined(__wasm__)
EXPORTA("abla_buffer") char* abla_buffer(int n) { return xmalloc((size_t)n); }
EXPORTA("abla_liberar") void abla_liberar(char* p) { free(p); }
/* En el navegador el espacio es limitado: la memoria de largo plazo de cada
 * ser se guarda en tramos de 128 KB (se conservan los dos últimos). */
EXPORTA("abla_iniciar_web") int abla_iniciar_web(unsigned contexto) { return abla_iniciar("mundo", contexto, 128 * 1024); }
#endif
