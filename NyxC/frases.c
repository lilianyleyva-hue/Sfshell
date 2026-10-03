/* frases.c — memoria de frases: lo que leyeron, entero.
 *
 * Las mentes recuerdan palabras y cómo se asocian (mente.c), pero una frase
 * armada palabra por palabra a veces mezcla dos frases distintas. Aquí se
 * guardan las frases completas que leyeron. Para responder, la mente que
 * habla busca las frases que contienen su idea ganadora y elige la que mejor
 * encaja con la pregunta y con lo que tiene activo. Tu opinión (bien/mal)
 * también cuenta: una frase que no te gustó pierde la próxima vez.
 */
#include "nyx.h"

static int fr_tokens(const char *t, char toks[][LARGO], int max) {
    return nyx_tokens(t, toks, max);
}

/* Guarda una frase (si no la tenía). Devuelve su índice o -1. */
int frases_pon(Consejo *c, const char *texto) {
    char toks[MAXEST][LARGO];
    char limpio[LFRASE];
    int n = fr_tokens(texto, toks, MAXEST), i, k, peor;
    size_t l = 0;
    if (n < 3) return -1;               /* "hola" no es una frase que recordar */
    limpio[0] = 0;
    for (i = 0; i < n; i++) {           /* se guarda ya normalizada */
        int esc = snprintf(limpio + l, sizeof limpio - l, "%s%s", i ? " " : "", toks[i]);
        if (esc < 0 || (size_t)esc >= sizeof limpio - l) break;
        l += (size_t)esc;
    }
    for (k = 0; k < c->nfr; k++)
        if (strcmp(c->fr[k].t, limpio) == 0) { if (c->fr[k].usos < 65000) c->fr[k].usos++; return k; }
    if (c->nfr < MAXFR) k = c->nfr++;
    else {                               /* llena: se olvida la menos valorada */
        peor = 0;
        for (k = 1; k < MAXFR; k++)
            if (c->fr[k].puntos + 0.1f * c->fr[k].usos < c->fr[peor].puntos + 0.1f * c->fr[peor].usos) peor = k;
        k = peor;
    }
    nyx_copia(c->fr[k].t, limpio, LFRASE);
    c->fr[k].puntos = 0;
    c->fr[k].usos = 1;
    return k;
}

/* Elige la frase que esta mente diría sobre 'centro'.
 * ctx: lo que se preguntó (ids de la mente). Devuelve índice o -1. */
int frases_elige(const Consejo *c, const Mente *m, int centro, const int *ctx, int nctx, float *puntaje) {
    char toks[MAXEST][LARGO];
    int k, mejor = -1;
    float mp = -1e9f;
    const char *et;
    if (centro < 0) return -1;
    et = m->s[centro].et;
    for (k = 0; k < c->nfr; k++) {
        int n = fr_tokens(c->fr[k].t, toks, MAXEST), i, x, tiene = 0, enCtx = 0, conocidas = 0;
        float act = 0, p;
        for (i = 0; i < n; i++) {
            int id;
            if (strcmp(toks[i], et) == 0) tiene = 1;
            if (nyx_vacia(toks[i])) continue;
            for (x = 0; x < nctx; x++)
                if (ctx[x] != centro && strcmp(m->s[ctx[x]].et, toks[i]) == 0) { enCtx++; break; }
            id = m_busca(m, toks[i]);
            if (id >= 0) { act += m->s[id].A; conocidas++; }
        }
        if (!tiene) continue;
        p = 1.0f * (float)enCtx + 0.5f * (conocidas ? act / (float)conocidas : 0) + c->fr[k].puntos
            + nyx_rango(0, m->ruido + 0.05f);
        if (c_es_eco(c, c->fr[k].t)) p -= 2.0f;      /* no repetir lo que se acaba de decir */
        if (p > mp) { mp = p; mejor = k; }
    }
    if (puntaje) *puntaje = mp;
    return mejor;
}

/* La frase en Resh, con lo que esta mente sabe decir. */
void frases_resh(const Mente *m, const char *es, char *resh, size_t cap) {
    char toks[MAXEST][LARGO];
    int n = fr_tokens(es, toks, MAXEST), i;
    size_t l = 0;
    resh[0] = 0;
    for (i = 0; i < n; i++) {
        int id = m_busca(m, toks[i]);
        const char *r = id >= 0 && m->s[id].sabe ? nyx_a_resh(toks[i]) : NULL;
        int esc = snprintf(resh + l, cap > l ? cap - l : 0, "%s%s", i ? " " : "", r ? r : toks[i]);
        if (esc < 0 || (size_t)esc >= cap - l) break;
        l += (size_t)esc;
    }
}

/* Tu opinión sobre una frase recordada. */
void frases_opina(Consejo *c, int k, int bueno) {
    if (k < 0 || k >= c->nfr) return;
    c->fr[k].puntos += bueno ? 0.5f : -1.0f;
    if (c->fr[k].puntos < -5) c->fr[k].puntos = -5;
    if (c->fr[k].puntos > 5) c->fr[k].puntos = 5;
}
