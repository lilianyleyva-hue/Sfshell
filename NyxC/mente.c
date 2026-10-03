/* mente.c — una mente: semiones, resonancia, veredicto y frases */
#include "nyx.h"

/* ------------------------------------------------------------------ */
/*  Mente                                                              */
/* ------------------------------------------------------------------ */




float m_precision(const Mente *m) {
    return m->intentos > 0 ? (float)m->aciertos / (float)m->intentos : 0.5f;
}

void m_ventana(Mente *m, const char *texto) {
    nyx_copia(m->ven[m->iven], texto, LVEN);
    m->iven = (m->iven + 1) % NVEN;
    if (m->nven < NVEN) m->nven++;
}

int m_busca(const Mente *m, const char *et) {
    unsigned h = nyx_fnv(et) & (HASHN - 1);
    while (m->hash[h] >= 0) {
        if (strcmp(m->s[m->hash[h]].et, et) == 0) return m->hash[h];
        h = (h + 1) & (HASHN - 1);
    }
    return -1;
}

void m_hash_pon(Mente *m, int i) {
    unsigned h = nyx_fnv(m->s[i].et) & (HASHN - 1);
    while (m->hash[h] >= 0) h = (h + 1) & (HASHN - 1);
    m->hash[h] = (short)i;
}

void m_hash_rehace(Mente *m) {
    int i;
    for (i = 0; i < HASHN; i++) m->hash[i] = -1;
    for (i = 0; i < m->n; i++) m_hash_pon(m, i);
}

int m_nuevo(Mente *m, const char *et, int fusion) {
    Semion *s;
    const char *r;
    if (m->n >= MAXSEM) return -1;
    s = &m->s[m->n];
    memset(s, 0, sizeof *s);
    nyx_copia(s->et, et, LARGO);
    nyx_firma(s->et, s->f);
    s->A = 0.6f;
    s->fase = nyx_rango(0, NYX_2PI);
    s->frec = nyx_rango(0.8f, 1.2f);
    s->fusion = (unsigned char)fusion;
    s->carga = (signed char)(fusion ? 1 : 0);
    r = fusion ? NULL : nyx_a_resh(s->et);
    if (r) s->sabe = (unsigned char)(nyx_es_particula(s->et) || nyx_f01() < NYX_ROLES[m->rol].sabeResh);
    m_hash_pon(m, m->n);
    return m->n++;
}

Acople *m_acople(Mente *m, int a, int b) {
    Semion *s = &m->s[a];
    int k;
    for (k = 0; k < s->nv; k++)
        if (s->v[k].j == b) return &s->v[k];
    return NULL;
}

/* Crea el acople a->b (si no cabe, reemplaza el más débil). */
Acople *m_acople_nuevo(Mente *m, int a, int b, float w, float th) {
    Semion *s = &m->s[a];
    Acople *c;
    if (a == b) return NULL;
    if (s->nv < MAXVEC) {
        c = &s->v[s->nv++];
    } else {
        int k, peor = 0;
        for (k = 1; k < MAXVEC; k++)
            if (s->v[k].w + s->v[k].sec < s->v[peor].w + s->v[peor].sec) peor = k;
        if (s->v[peor].w + s->v[peor].sec > w) return NULL;
        c = &s->v[peor];
    }
    c->j = (short)b; c->w = w; c->th = th; c->sec = 0;
    return c;
}

void m_fija(Mente *m, int a, int b, float w, float th) {
    Acople *c = m_acople(m, a, b);
    if (!c) c = m_acople_nuevo(m, a, b, w, th);
    if (c) { c->w = w; c->th = th; }
}

/* Refuerzo con saturación: repetir acerca a 1 sin pasarse. */
void m_refuerza(Mente *m, int a, int b, float dw, float th, float dsec) {
    Acople *c;
    if (a == b) return;
    c = m_acople(m, a, b);
    if (!c) { c = m_acople_nuevo(m, a, b, dw, th); if (!c) return; c->sec = dsec; return; }
    c->w += dw * (1 - c->w);
    c->sec += dsec * (1 - c->sec);
}

float m_coh(const Mente *m, int i) {
    const Semion *s = &m->s[i];
    float c = 0;
    int k;
    for (k = 0; k < s->nv; k++) {
        const Semion *o = &m->s[s->v[k].j];
        c += s->v[k].w * s->A * o->A * cosf(s->fase - o->fase - s->v[k].th);
    }
    return c;
}

void m_envuelve(float *f) {
    *f = fmodf(*f, NYX_2PI);
    if (*f < 0) *f += NYX_2PI;
}

void m_decae(Mente *m, int i, float ruido) {
    Semion *s = &m->s[i];
    float A = s->A;
    float dA = -m->rigidez * (A * A - m->A0 * m->A0) * A * 0.02f;
    s->A = nyx_lim(A + dA, 0.05f, 1.0f);
    s->fase += s->frec * 0.02f + nyx_rango(-ruido, ruido);
    m_envuelve(&s->fase);
}

void m_propaga(Mente *m, int i) {
    Semion *s = &m->s[i];
    float d = 0;
    int k;
    for (k = 0; k < s->nv; k++) {
        const Acople *c = &s->v[k];
        const Semion *o = &m->s[c->j];
        if (o->carga != s->carga && c->w <= 0.9f) continue;
        d += c->w * o->A * sinf(o->fase - s->fase - c->th);
    }
    s->fase += 0.05f * d;
    m_envuelve(&s->fase);
}

/* Lo que el estímulo despierta: él, sus vecinos y los vecinos de esos. */
int m_region(const Mente *m, const int *est, int ne, int *out, int tope) {
    static unsigned char marca[MAXSEM];
    int n = 0, i, k, ini, fin;
    for (i = 0; i < ne && n < tope; i++)
        if (est[i] >= 0 && !marca[est[i]]) { marca[est[i]] = 1; out[n++] = est[i]; }
    ini = 0; fin = n;
    for (i = ini; i < fin && n < tope; i++) {
        const Semion *s = &m->s[out[i]];
        for (k = 0; k < s->nv && n < tope; k++)
            if (!marca[s->v[k].j]) { marca[s->v[k].j] = 1; out[n++] = s->v[k].j; }
    }
    ini = fin; fin = n;
    for (i = ini; i < fin && n < tope; i++) {
        const Semion *s = &m->s[out[i]];
        for (k = 0; k < s->nv && k < 12 && n < tope; k++)
            if (!marca[s->v[k].j]) { marca[s->v[k].j] = 1; out[n++] = s->v[k].j; }
    }
    for (i = 0; i < n; i++) marca[out[i]] = 0;
    return n;
}

void m_atiende(Mente *m, int id) {
    int i, j = 0;
    for (i = 0; i < m->nfoco; i++) if (m->foco[i] != id) m->foco[j++] = m->foco[i];
    m->nfoco = j;
    if (m->nfoco == 7) { memmove(m->foco, m->foco + 1, 6 * sizeof m->foco[0]); m->nfoco = 6; }
    m->foco[m->nfoco++] = (short)id;
    m->s[id].A = nyx_min(1, m->s[id].A + 0.12f);
}

void m_episodio(Mente *m, int id) {
    if (m->nepis == 24) { memmove(m->epis, m->epis + 1, 23 * sizeof m->epis[0]); m->nepis = 23; }
    m->epis[m->nepis++] = (short)id;
}

int m_es_objetivo(const Mente *m, int id) {
    int k;
    for (k = 0; k < m->nobj; k++) if (m->obj[k] == id) return 1;
    return 0;
}

/* Poda homeostática: libera ~12% cuando la mente se llena. Primero caen las
 * ideas propias que nadie retomó; luego lo menos coherente y usado. */
void m_poda(Mente *m) {
    static float punt[MAXSEM];
    static short nuevo[MAXSEM];
    int quitar = m->n / 8, i, k, q;
    for (i = 0; i < m->n; i++) {
        const Semion *s = &m->s[i];
        punt[i] = s->A + 0.08f * (float)s->usos + 0.3f * (float)s->dialogo + 0.2f * m_coh(m, i)
                + 0.03f * (float)s->nv + (s->sabe ? 0.4f : 0.0f);
        if (s->fusion && s->dialogo == 0) punt[i] -= 5;
        if (m_es_objetivo(m, i)) punt[i] += 1000;
    }
    for (i = 0; i < m->n; i++) nuevo[i] = 0;
    for (q = 0; q < quitar; q++) {           /* marca los 'quitar' peores */
        int peor = -1;
        for (i = 0; i < m->n; i++)
            if (!nuevo[i] && (peor < 0 || punt[i] < punt[peor])) peor = i;
        if (peor < 0) break;
        nuevo[peor] = -1;
    }
    k = 0;
    for (i = 0; i < m->n; i++) {
        if (nuevo[i] == -1) continue;
        nuevo[i] = (short)k;
        if (k != i) m->s[k] = m->s[i];
        k++;
    }
    {
        int nn = k, a;
        for (a = 0; a < nn; a++) {
            Semion *s = &m->s[a];
            int j = 0;
            for (i = 0; i < s->nv; i++) {
                short d = nuevo[s->v[i].j];
                if (d >= 0) { s->v[j] = s->v[i]; s->v[j].j = d; j++; }
            }
            s->nv = (unsigned char)j;
        }
        /* índices guardados en otras partes */
        for (i = 0, k = 0; i < m->nobj; i++) if (nuevo[m->obj[i]] >= 0) m->obj[k++] = nuevo[m->obj[i]];
        m->nobj = k;
        for (i = 0, k = 0; i < m->nfoco; i++) if (nuevo[m->foco[i]] >= 0) m->foco[k++] = nuevo[m->foco[i]];
        m->nfoco = k;
        for (i = 0, k = 0; i < m->nrec; i++) if (nuevo[m->recientes[i]] >= 0) m->recientes[k++] = nuevo[m->recientes[i]];
        m->nrec = k;
        for (i = 0, k = 0; i < m->nepis; i++) if (nuevo[m->epis[i]] >= 0) m->epis[k++] = nuevo[m->epis[i]];
        m->nepis = k;
        for (i = 0, k = 0; i < m->ncam; i++) {
            Camino c = m->cam[i];
            if (nuevo[c.a] >= 0 && nuevo[c.b] >= 0 && nuevo[c.c] >= 0) {
                c.a = nuevo[c.a]; c.b = nuevo[c.b]; c.c = nuevo[c.c];
                m->cam[k++] = c;
            }
        }
        m->ncam = k;
        m->n = nn;
    }
    m_hash_rehace(m);
}

/* Ingesta: texto -> semiones, con asociación (ventana de 3) y secuencia. */

/* Ingesta con gramática plena (lo que escribe el humano o un libro). */
int m_ingesta(Mente *m, const char *texto, int *ids, int max, int dialogo) {
    return m_ingesta2(m, texto, ids, max, dialogo, 1.0f);
}

/* gramatica: cuánto se aprende el ORDEN de las palabras. Las frases de otras
 * mentes enseñan asociaciones, pero poca gramática: si no, se copiarían sus
 * frases a medio hacer y el idioma se degradaría. */
int m_ingesta2(Mente *m, const char *texto, int *ids, int max, int dialogo, float gramatica) {
    char toks[MAXEST][LARGO];
    int nt = nyx_tokens(texto, toks, MAXEST), k = 0, i, d;
    if (m->n > MAXSEM - MAXEST - 8) m_poda(m);
    for (i = 0; i < nt && k < max; i++) {
        int id = m_busca(m, toks[i]);
        if (id < 0) id = m_nuevo(m, toks[i], 0);
        if (id < 0) continue;
        m->s[id].A = nyx_min(1, m->s[id].A + 0.15f);
        if (m->s[id].usos < 65000) m->s[id].usos++;
        if (dialogo && m->s[id].dialogo < 65000) m->s[id].dialogo++;
        ids[k++] = id;
    }
    for (i = 0; i < k; i++)
        for (d = 1; d <= 3 && i + d < k; d++) {
            if (ids[i] == ids[i + d]) continue;
            m_refuerza(m, ids[i], ids[i + d], 0.35f / (float)d, 0.3f, d == 1 ? 0.4f * gramatica : 0.0f);
            m_refuerza(m, ids[i + d], ids[i], 0.2f / (float)d, -0.3f, 0.0f);
        }
    return k;
}

/* Predicción: lo que espera oír según su foco. */
int m_predice(const Mente *m, int *out, int max) {
    int cand[64], n = 0, i, k;
    float peso[64];
    for (i = 0; i < m->nfoco; i++) {
        const Semion *s = &m->s[m->foco[i]];
        for (k = 0; k < s->nv; k++) {
            int j = s->v[k].j, x;
            float p = s->v[k].w * m->s[j].A;
            for (x = 0; x < n; x++) if (cand[x] == j) break;
            if (x == n) { if (n == 64) continue; cand[n] = j; peso[n] = 0; n++; }
            peso[x] += p;
        }
    }
    for (i = 0; i < max && i < n; i++) {
        int mejor = i;
        for (k = i + 1; k < n; k++) if (peso[k] > peso[mejor]) mejor = k;
        { int t = cand[i]; float tp = peso[i]; cand[i] = cand[mejor]; peso[i] = peso[mejor]; cand[mejor] = t; peso[mejor] = tp; }
        out[i] = cand[i];
    }
    return i;
}

/* Recibir: transducir lo que otro dijo. Lo sorprendente se aprende más. */
int m_recibe(Mente *m, const char *texto, const char *quien, int *ids, int max) {
    int pred[3], np = m_predice(m, pred, 3), k, i, aciertos = 0;
    float sorpresa;
    char linea[LVEN];
    k = m_ingesta2(m, texto, ids, max, 1, strcmp(quien, "humano") == 0 ? 1.0f : 0.15f);
    for (i = 0; i < k; i++) {
        int x;
        for (x = 0; x < np; x++) if (pred[x] == ids[i]) aciertos++;
    }
    sorpresa = np == 0 ? 0.5f : 1.0f - (float)aciertos / (float)(k ? k : 1);
    for (i = 0; i < k; i++) {
        Semion *s = &m->s[ids[i]];
        s->A = nyx_min(1, s->A + 0.25f * sorpresa + 0.05f);
        if (s->A > 0.55f && !nyx_vacia(s->et)) m_atiende(m, ids[i]);
    }
    snprintf(linea, sizeof linea, "◀ %s: %s", quien, texto);
    m_ventana(m, linea);
    return k;
}

/* Elige el siguiente vecino más coherente no visitado (para razonar a saltos). */
int m_salto(const Mente *m, int id, const int *vis, int nvis) {
    const Semion *s = &m->s[id];
    int k, mejor = -1;
    float mc = -1e9f;
    for (k = 0; k < s->nv; k++) {
        int j = s->v[k].j, x, ya = 0;
        float c;
        if (m->s[j].fusion || nyx_vacia(m->s[j].et)) continue;
        for (x = 0; x < nvis; x++) if (vis[x] == j) ya = 1;
        if (ya) continue;
        c = m_coh(m, j);
        if (c > mc) { mc = c; mejor = j; }
    }
    return mejor;
}

/* Cristalización: un camino A->B->C recorrido 3 veces se vuelve concepto. */
int m_cristaliza(Mente *m, int a, int b, int c) {
    char et[LARGO * 3];
    char ea[10], eb[10], ec[10];
    int id, k;
    if (m->n >= MAXSEM - 1) return -1;
    nyx_copia(ea, m->s[a].et, sizeof ea);
    nyx_copia(eb, m->s[b].et, sizeof eb);
    nyx_copia(ec, m->s[c].et, sizeof ec);
    snprintf(et, sizeof et, "%s⊕%s⊕%s", ea, eb, ec);
    nyx_copia(et, et, LARGO);
    if (m_busca(m, et) >= 0) return -1;
    id = m_nuevo(m, et, 1);
    if (id < 0) return -1;
    m->s[id].A = 0.9f;
    m->s[id].dialogo = 2;
    for (k = 0; k < DIMF; k++) m->s[id].f[k] = (m->s[a].f[k] + m->s[b].f[k] + m->s[c].f[k]) / 3;
    m_fija(m, a, id, 0.8f, 0); m_fija(m, id, a, 0.8f, 0);
    m_fija(m, b, id, 0.8f, 0); m_fija(m, id, b, 0.8f, 0);
    m_fija(m, c, id, 0.8f, 0); m_fija(m, id, c, 0.8f, 0);
    m->cristales++;
    return id;
}

/* Inferencia transitiva: refuerza A->B->C; el atajo perdura. */
int m_transitiva(Mente *m, int id) {
    int vis[3], s1, s2, k;
    Acople *c;
    vis[0] = id;
    s1 = m_salto(m, id, vis, 1);
    if (s1 < 0) return -1;
    c = m_acople(m, id, s1);
    m_fija(m, id, s1, nyx_min(1, (c ? c->w : 0.3f) + 0.1f), 0.2f);
    vis[1] = s1;
    s2 = m_salto(m, s1, vis, 2);
    if (s2 < 0) return -1;
    c = m_acople(m, s1, s2);
    m_fija(m, s1, s2, nyx_min(1, (c ? c->w : 0.3f) + 0.1f), 0.2f);
    for (k = 0; k < m->ncam; k++) {
        Camino *p = &m->cam[k];
        if (p->a == id && p->b == s1 && p->c == s2) {
            if (++p->k == 3) return m_cristaliza(m, id, s1, s2);
            return -1;
        }
    }
    if (m->ncam == NCAM) { memmove(m->cam, m->cam + 1, (NCAM - 1) * sizeof m->cam[0]); m->ncam--; }
    m->cam[m->ncam].a = (short)id; m->cam[m->ncam].b = (short)s1; m->cam[m->ncam].c = (short)s2; m->cam[m->ncam].k = 1;
    m->ncam++;
    return -1;
}

float m_incrustacion(const Mente *m, int id) {
    const Semion *s = &m->s[id];
    int k, n = 0;
    if (s->nv == 0) return 0;
    for (k = 0; k < s->nv; k++) if (m->s[s->v[k].j].dialogo > 0) n++;
    return (float)n / (float)s->nv;
}

/* Veredicto: el estímulo despierta una región, se relaja con recocido
 * (caliente -> frío) y gana lo que la pregunta EVOCA (no su eco). */
int m_veredicto(Mente *m, const int *est0, int ne0, float *C, float *E, int *cristal) {
    static int reg[320];
    int est[MAXEST], ne = 0, nr, it, i, gan = -1;
    float mejor = -1e9f;
    if (cristal) *cristal = -1;
    /* lo que importa de la pregunta son sus palabras con contenido:
     * "qué", "es", "la" están en todas partes y llevarían a cualquier lado */
    for (i = 0; i < ne0 && ne < MAXEST; i++) if (!nyx_vacia(m->s[est0[i]].et)) est[ne++] = est0[i];
    if (ne == 0) for (i = 0; i < ne0 && ne < MAXEST; i++) est[ne++] = est0[i];
    nr = m_region(m, est, ne, reg, 300);
    if (nr == 0) { *C = 0; *E = 0; return -1; }
    for (it = 0; it < 60; it++) {
        float r = m->ruido * (3.0f - 2.0f * (float)it / 60.0f);
        for (i = 0; i < nr; i++) m_decae(m, reg[i], r);
        for (i = 0; i < nr; i++) m_propaga(m, reg[i]);
    }
    for (i = 0; i < nr; i++) {
        int id = reg[i], x, esEst = 0;
        float enlace = 0, rel;
        const Semion *s = &m->s[id];
        if (s->fusion || nyx_vacia(s->et)) continue;
        for (x = 0; x < ne; x++) {
            Acople *c;
            if (est[x] == id) esEst = 1;
            /* un acople inhibido (θ≈π) resta: así 'mal' cambia la respuesta */
            c = m_acople(m, est[x], id); if (c) enlace += c->w * cosf(c->th);
            c = m_acople(m, id, est[x]); if (c) enlace += c->w * cosf(c->th);
        }
        if (esEst && nr > ne) continue;
        /* manda lo que la pregunta evoca (enlace); la coherencia desempata */
        rel = enlace + 0.35f * tanhf(m_coh(m, id)) + 0.1f * (float)(m_es_objetivo(m, id));
        /* lo muy frecuente pesa menos: no gana lo que sale en todas partes */
        rel /= 1.0f + logf(1.0f + (float)s->usos) * 0.3f;
        {   /* fatiga: lo que acaba de ganar descansa un poco */
            int q;
            for (q = 0; q < m->nrec; q++) if (m->recientes[q] == id) { rel -= (rel > 0 ? rel : -rel) + 0.2f; break; }
        }
        if (rel > mejor) { mejor = rel; gan = id; }
    }
    if (gan < 0) {      /* solo estaba el propio estímulo */
        for (i = 0; i < ne; i++) if (!nyx_vacia(m->s[est[i]].et)) { gan = est[i]; break; }
        if (gan < 0) gan = est[0];
    }
    {
        int cr = m_transitiva(m, gan);
        if (cristal) *cristal = cr;
    }
    m_episodio(m, gan);
    m_atiende(m, gan);
    if (m->nrec == 6) { memmove(m->recientes, m->recientes + 1, 5 * sizeof m->recientes[0]); m->nrec = 5; }
    m->recientes[m->nrec++] = (short)gan;
    *C = m_coh(m, gan);
    *E = m_incrustacion(m, gan);
    return gan;
}

/* Frase: lo que va antes y después del centro según la memoria de
 * secuencia. Devuelve los semiones en orden. */
float m_en_contexto(const int *ctx, int nctx, int id) {
    int i;
    for (i = 0; i < nctx; i++) if (ctx[i] == id) return 0.6f;
    return 0;
}

/* ctx: las palabras de lo que se preguntó; la frase prefiere ir por ahí
 * (así "qué es el mar" lleva a "el mar es grande", no a otra frase con "es"). */
int m_frase(Mente *m, int centro, int *out, int max, const int *ctx, int nctx) {
    int pre[3], npre = 0, n = 0, i, k, actual, objetivoLargo;
    /* hasta 2 palabras que suelen ir ANTES */
    actual = centro;
    while (npre < 2) {
        int mejor = -1;
        float mp = 0.04f;
        for (i = 0; i < m->n; i++) {
            Acople *c;
            int ya = 0, x;
            if (i == actual || m->s[i].fusion) continue;
            c = m_acople(m, i, actual);
            if (!c || c->sec <= 0) continue;
            for (x = 0; x < npre; x++) if (pre[x] == i) ya = 1;
            if (ya || i == centro) continue;
            {
                float p = c->sec * (0.5f + m->s[i].A) + m_en_contexto(ctx, nctx, i) + nyx_rango(0, m->ruido * 2);
                if (p > mp) { mp = p; mejor = i; }
            }
        }
        if (mejor < 0) break;
        pre[npre++] = mejor;
        actual = mejor;
    }
    /* no empezar con "y", "en", "por"… (los artículos sí valen) */
    while (npre > 0 && nyx_conector(m->s[pre[npre - 1]].et)) npre--;
    for (i = npre - 1; i >= 0 && n < max; i--) out[n++] = pre[i];
    out[n++] = centro;
    objetivoLargo = 4 + nyx_ent(5);
    actual = centro;
    while (n < max && n < objetivoLargo) {
        const Semion *s = &m->s[actual];
        int mejor = -1;
        float mp = 0.02f;
        for (k = 0; k < s->nv; k++) {
            int j = s->v[k].j, x, ya = 0;
            float p;
            if (s->v[k].sec <= 0 || m->s[j].fusion) continue;
            for (x = 0; x < n; x++) if (out[x] == j) ya = 1;
            if (ya) continue;
            p = s->v[k].sec * (0.5f + m->s[j].A) + m_en_contexto(ctx, nctx, j) + nyx_rango(0, m->ruido * 3);
            if (p > mp) { mp = p; mejor = j; }
        }
        if (mejor < 0) break;
        out[n++] = mejor;
        actual = mejor;
    }
    /* una frase no termina en "y", "por", "un"…: se quita lo que cuelga */
    while (n > 2 && nyx_vacia(m->s[out[n - 1]].et) && out[n - 1] != centro) n--;
    if (n < 2) {      /* sin secuencia: al menos su asociación más fuerte */
        const Semion *s = &m->s[centro];
        int mejor = -1;
        float mw = 0;
        for (k = 0; k < s->nv; k++)
            if (!m->s[s->v[k].j].fusion && s->v[k].w > mw) { mw = s->v[k].w; mejor = s->v[k].j; }
        if (mejor >= 0 && n < max) out[n++] = mejor;
    }
    return n;
}

void m_texto(const Mente *m, const int *ids, int n, char *es, size_t ces, char *resh, size_t cre) {
    int i;
    size_t le = 0, lr = 0;
    es[0] = 0; resh[0] = 0;
    for (i = 0; i < n; i++) {
        const Semion *s = &m->s[ids[i]];
        const char *r = s->sabe ? nyx_a_resh(s->et) : NULL;
        le += (size_t)snprintf(es + le, le < ces ? ces - le : 0, "%s%s", i ? " " : "", s->et);
        lr += (size_t)snprintf(resh + lr, lr < cre ? cre - lr : 0, "%s%s", i ? " " : "", r ? r : s->et);
        if (le >= ces) le = ces - 1;
        if (lr >= cre) lr = cre - 1;
    }
}

/* Idea propia: fusiona dos ideas activas parecidas por su firma efectiva. */
int m_imagina(Mente *m, int *pa, int *pb) {
    int act[64], na = 0, i, k, a, b = -1, id;
    float fa[DIMF], mejor = 1e9f;
    char et[LARGO * 2], ea[12], eb[12];
    if (m->n > MAXSEM - 4) m_poda(m);
    for (k = 0; k < 400 && na < 64; k++) {
        int x = nyx_ent(m->n);
        if (m->n == 0) break;
        if (m->s[x].A > 0.45f && !m->s[x].fusion && !nyx_vacia(m->s[x].et)) {
            int y, ya = 0;
            for (y = 0; y < na; y++) if (act[y] == x) ya = 1;
            if (!ya) act[na++] = x;
        }
    }
    if (na < 2) return -1;
    a = act[nyx_ent(na)];
    for (k = 0; k < DIMF; k++) fa[k] = m->s[a].f[k];
    for (i = 0; i < na; i++) {
        float d = 0;
        if (act[i] == a) continue;
        for (k = 0; k < DIMF; k++) { float e = fa[k] - m->s[act[i]].f[k]; d += e * e; }
        /* además de la forma: si ya comparten vecinos, se acercan */
        {
            const Semion *sa = &m->s[a];
            int x;
            for (x = 0; x < sa->nv; x++) if (m_acople(m, act[i], sa->v[x].j)) d -= 0.05f;
        }
        d += nyx_rango(0, m->ruido);
        if (d < mejor) { mejor = d; b = act[i]; }
    }
    if (b < 0) return -1;
    nyx_copia(ea, m->s[a].et, sizeof ea);
    nyx_copia(eb, m->s[b].et, sizeof eb);
    snprintf(et, sizeof et, "%s⊕%s", ea, eb);
    nyx_copia(et, et, LARGO);
    id = m_busca(m, et);
    if (id < 0) id = m_nuevo(m, et, 1);
    if (id < 0) return -1;
    for (k = 0; k < DIMF; k++) m->s[id].f[k] = (m->s[a].f[k] + m->s[b].f[k]) / 2;
    m->s[id].fase = (m->s[a].fase + m->s[b].fase) / 2;
    m->s[id].A = nyx_min(1, (m->s[a].A + m->s[b].A) / 2 + 0.1f);
    m_fija(m, a, id, 0.5f, 0.1f);
    m_fija(m, b, id, 0.5f, -0.1f);
    m_fija(m, id, a, 0.5f, 0.1f);
    m_fija(m, id, b, 0.5f, -0.1f);
    m_refuerza(m, a, b, 0.2f, 0, 0);
    m->ideas++;
    *pa = a; *pb = b;
    return id;
}

/* Después de decir algo, esas ideas se cansan: el tema puede cambiar. */
void m_cansa(Mente *m, const int *ids, int n) {
    int i;
    for (i = 0; i < n; i++) m->s[ids[i]].A = nyx_max(0.1f, m->s[ids[i]].A * 0.7f);
}

/* Un ciclo de pensamiento local: relaja lo que está en el foco. */
void m_piensa(Mente *m) {
    int est[7], reg[200], nr, i;
    for (i = 0; i < m->nfoco; i++) est[i] = m->foco[i];
    nr = m_region(m, est, m->nfoco, reg, 160);
    for (i = 0; i < nr; i++) m_decae(m, reg[i], m->ruido);
    for (i = 0; i < nr; i++) m_propaga(m, reg[i]);
    for (i = 0; i < m->nobj; i++) m->s[m->obj[i]].A = nyx_min(1, m->s[m->obj[i]].A + 0.01f);
    m->ciclos++;
}

void m_init(Mente *m, int rol) {
    int ids[MAXEST], n, i;
    memset(m, 0, sizeof *m);
    m->rol = rol;
    m->ruido = NYX_ROLES[rol].ruido;
    m->rigidez = NYX_ROLES[rol].rigidez;
    m->A0 = NYX_ROLES[rol].A0;
    for (i = 0; i < HASHN; i++) m->hash[i] = -1;
    for (i = 0; i < NACC; i++) m->valor[i] = 0;
    m->ultima = -1;
    n = m_ingesta(m, NYX_ROLES[rol].objetivo, ids, MAXEST, 0);
    for (i = 0; i < n && m->nobj < 8; i++) {
        m->obj[m->nobj++] = (short)ids[i];
        m->s[ids[i]].A = 0.95f;
        if (nyx_a_resh(m->s[ids[i]].et)) m->s[ids[i]].sabe = 1;   /* su objetivo lo sabe decir */
    }
}
