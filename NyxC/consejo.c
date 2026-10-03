/* consejo.c — las 18 juntas: imaginación, deliberación y opinión */
#include "nyx.h"

/* ------------------------------------------------------------------ */
/*  Consejo: las 18 juntas                                             */
/* ------------------------------------------------------------------ */





int c_es_eco(const Consejo *c, const char *es) {
    int i;
    for (i = 0; i < 10; i++) if (c->ecos[i][0] && strcmp(c->ecos[i], es) == 0) return 1;
    return 0;
}

void c_pon_eco(Consejo *c, const char *es) {
    nyx_copia(c->ecos[c->iecos], es, sizeof c->ecos[0]);
    c->iecos = (c->iecos + 1) % 10;
}

void c_evento(Consejo *c, int rol, int tipo, const char *texto) {
    if (c->ev) c->ev(c->ud, rol, tipo, texto);
}

void consejo_init(Consejo *c, unsigned long long semilla, int conInfancia) {
    int r, i, ids[MAXEST];
    nyx_dic_init();
    nyx_semilla(semilla);
    memset(c, 0, sizeof *c);
    for (r = 0; r < NROLES; r++) m_init(&c->m[r], r);
    if (conInfancia)
        for (i = 0; i < NYX_NCORPUS; i++)
        {
            for (r = 0; r < NROLES; r++) m_ingesta(&c->m[r], NYX_CORPUS[i], ids, MAXEST, 0);
            frases_pon(c, NYX_CORPUS[i]);
        }
    c->ultimo = -1;
    c->fbFr = -1;
}

/* Todas leen un texto (enseñar). Devuelve cuántas palabras nuevas hubo. */
int consejo_lee(Consejo *c, const char *texto) {
    int r, ids[MAXEST], antes = c->m[0].n;
    for (r = 0; r < NROLES; r++) m_ingesta(&c->m[r], texto, ids, MAXEST, 0);
    frases_pon(c, texto);
    return c->m[0].n - antes > 0 ? c->m[0].n - antes : 0;
}

/* Las demás oyen lo que dijo 'emisor'. Si el emisor sabía decir una palabra
 * en Resh y la oyente no, puede aprenderla. Devuelve la resonancia media. */
float c_difunde(Consejo *c, int emisor, const int *frase, int nf, int *aprendieron, char *palabraAprendida) {
    Mente *e = &c->m[emisor];
    char es[400], resh[400];
    float suma = 0;
    int r, i, oyentes = 0;
    m_texto(e, frase, nf, es, sizeof es, resh, sizeof resh);
    *aprendieron = 0;
    palabraAprendida[0] = 0;
    for (r = 0; r < NROLES; r++) {
        Mente *o;
        int ids[MAXEST], k;
        float res = 0;
        if (r == emisor) continue;
        o = &c->m[r];
        k = m_recibe(o, es, NYX_ROLES[emisor].nombre, ids, MAXEST);
        for (i = 0; i < k; i++) {
            res += m_coh(o, ids[i]);
            if (i < nf && e->s[frase[i]].sabe && !o->s[ids[i]].sabe && nyx_a_resh(o->s[ids[i]].et)
                && nyx_f01() < 0.35f) {
                o->s[ids[i]].sabe = 1;
                o->reshAprendidas++;
                /* se aprenden todas, pero solo se anuncian las que dicen algo */
                if (!nyx_vacia(o->s[ids[i]].et)) {
                    (*aprendieron)++;
                    nyx_copia(palabraAprendida, o->s[ids[i]].et, LARGO);
                }
            }
        }
        suma += k ? res / (float)k : 0;
        oyentes++;
    }
    return oyentes ? suma / (float)oyentes : 0;
}

/* ¿Ya lo dijo hace poco? (anti-eco) */
int c_ya_dicho(const Mente *m, const char *es) {
    int i;
    for (i = 0; i < m->nven; i++)
        if (strncmp(m->ven[i], "dije: ", 6) == 0 && strcmp(m->ven[i] + 6, es) == 0) return 1;
    return 0;
}

/* Tema de esta mente ahora: lo más activo de su foco (o de sus objetivos). */
int c_tema(const Mente *m) {
    int i, mejor = -1;
    float mA = -1;
    for (i = 0; i < m->nfoco; i++) {
        const Semion *s = &m->s[m->foco[i]];
        if (s->fusion || nyx_vacia(s->et)) continue;
        if (s->A + nyx_rango(0, 0.3f) > mA) { mA = s->A; mejor = m->foco[i]; }
    }
    if (mejor < 0 && m->nobj > 0) mejor = m->obj[nyx_ent(m->nobj)];
    return mejor;
}

/* Palabra que quiere aprender a decir en Resh (o -1). */
int c_duda(const Mente *m) {
    int i, mejor = -1;
    float mA = 0.3f;
    for (i = 0; i < m->n; i++) {
        const Semion *s = &m->s[i];
        if (s->sabe || s->fusion || nyx_vacia(s->et) || !nyx_a_resh(s->et)) continue;
        if (s->A + 0.05f * (float)(s->usos > 10 ? 10 : s->usos) > mA) { mA = s->A; mejor = i; }
    }
    return mejor;
}

void c_aprende(Consejo *c, int rol, int acc, float recompensa) {
    Mente *m = &c->m[rol];
    float antes = m->valor[acc];
    char t[128];
    m->valor[acc] += 0.25f * (recompensa - m->valor[acc]);
    m->veces[acc]++;
    snprintf(t, sizeof t, "'%s' ahora vale %.2f (%+.2f)", NYX_ACC[acc], m->valor[acc], m->valor[acc] - antes);
    c_evento(c, rol, EV_APRENDE, t);
}

float c_puntaje(const Mente *m, int a) {
    float p = NYX_ROLES[m->rol].gusto[a] + m->valor[a] + 0.3f / (1.0f + (float)m->veces[a]);
    if (a == m->ultima) p -= 0.15f * (float)m->racha;   /* aburrimiento */
    return p + nyx_rango(-m->ruido * 2, m->ruido * 2);
}

/* Lo que una mente dirá sobre 'centro': una frase que recuerda (si encaja
 * con lo que se habla) o una que arma ella. No crea ni poda semiones: los
 * índices que tiene quien llama siguen valiendo. Devuelve 1 si la recordó. */
static int c_compone(Consejo *c, Mente *m, int centro, const int *ctx, int nctx, float probRecuerdo,
                     int *out, int *nout, char *es, size_t ces, char *resh, size_t cre, int *idx) {
    float p = 0;
    int k = nyx_f01() < probRecuerdo ? frases_elige(c, m, centro, ctx, nctx, &p) : -1;
    *idx = -1;
    if (k >= 0 && p > -0.5f) {
        char toks[MAXEST][LARGO];
        int n = nyx_tokens(c->fr[k].t, toks, MAXEST), i;
        nyx_copia(es, c->fr[k].t, ces);
        frases_resh(m, es, resh, cre);
        *nout = 0;
        for (i = 0; i < n && *nout < MAXFRASE; i++) {
            int id = m_busca(m, toks[i]);
            if (id >= 0) out[(*nout)++] = id;
        }
        if (c->fr[k].usos < 65000) c->fr[k].usos++;
        *idx = k;
        return 1;
    }
    *nout = m_frase(m, centro, out, MAXFRASE, ctx, nctx);
    m_texto(m, out, *nout, es, ces, resh, cre);
    return 0;
}

/* Las que más recuerdan frases enteras (y las que más inventan). */
static float c_prob_recuerdo(int rol) {
    switch (rol) {
    case 6: return 0.8f;                 /* memoria */
    case 13: return 0.6f;                /* narrativa */
    case 4: case 9: return 0.15f;        /* creativo, intuicion: prefieren inventar */
    default: return 0.35f;
    }
}

/* Habla una mente: forma una frase desde 'centro', la dice, las demás la oyen. */
float c_habla(Consejo *c, int rol, int centro, const char *prefijo) {
    Mente *m = &c->m[rol];
    int frase[MAXFRASE], nf, aprend;
    char es[400], resh[400], linea[900], palabra[LARGO];
    float res;
    {
        int ctx[7], i, idx;
        for (i = 0; i < m->nfoco; i++) ctx[i] = m->foco[i];
        c_compone(c, m, centro, ctx, m->nfoco, c_prob_recuerdo(rol), frase, &nf, es, sizeof es, resh, sizeof resh, &idx);
    }
    if (c_ya_dicho(m, es) || c_es_eco(c, es)) return -0.6f;
    c_pon_eco(c, es);
    snprintf(linea, sizeof linea, "%s%s   «%s»", prefijo, resh, es);
    c_evento(c, rol, EV_DICE, linea);
    snprintf(linea, sizeof linea, "dije: %s", es);
    m_ventana(m, linea);
    m->dichos++;
    res = c_difunde(c, rol, frase, nf, &aprend, palabra);
    m_cansa(m, frase, nf);
    if (aprend > 0) {
        snprintf(linea, sizeof linea, "%d mente%s aprendi%s a decir «%s» = %s",
                 aprend, aprend > 1 ? "s" : "", aprend > 1 ? "eron" : "ó", palabra, nyx_a_resh(palabra));
        c_evento(c, rol, EV_RESH, linea);
    }
    return 1.5f * tanhf(res);
}

/* Un paso de vida libre: una mente imagina qué quiere hacer, lo hace,
 * mide cómo le fue y aprende. */
void consejo_paso(Consejo *c) {
    int rol, a, tema, duda, mejorA = -1, segunda = -1, tercera = -1, posible[NACC], i;
    float punt[NACC], rec = 0;
    Mente *m;
    char t[400];
    c->tick++;
    do { rol = nyx_ent(NROLES); } while (rol == c->ultimo && NROLES > 1);
    c->ultimo = rol;
    m = &c->m[rol];
    for (i = 0; i < 3; i++) m_piensa(m);
    tema = c_tema(m);
    duda = c_duda(m);
    for (a = 0; a < NACC; a++) {
        posible[a] = 1;
        punt[a] = c_puntaje(m, a);
    }
    if (tema < 0) posible[ACC_HABLAR] = 0;
    if (m->nepis < 2) posible[ACC_RECORDAR] = 0;
    for (a = 0; a < NACC; a++) {
        if (!posible[a]) continue;
        if (mejorA < 0 || punt[a] > punt[mejorA]) { tercera = segunda; segunda = mejorA; mejorA = a; }
        else if (segunda < 0 || punt[a] > punt[segunda]) { tercera = segunda; segunda = a; }
        else if (tercera < 0 || punt[a] > punt[tercera]) tercera = a;
    }
    {   /* exploración: a veces prueba otra cosa */
        int total = 0;
        float explora;
        for (a = 0; a < NACC; a++) total += m->veces[a];
        explora = nyx_max(0.08f, 0.35f - 0.01f * (float)total);
        if (nyx_f01() < explora) {
            int x;
            do { x = nyx_ent(NACC); } while (!posible[x]);
            if (x != mejorA) { tercera = segunda; segunda = mejorA; mejorA = x; }
        }
    }
    a = mejorA;
    if (a == m->ultima) m->racha++; else { m->ultima = a; m->racha = 1; }
    {
        const char *tt = tema >= 0 ? m->s[tema].et : "algo";
        char alt[96];
        alt[0] = 0;
        if (segunda >= 0 && tercera >= 0)
            snprintf(alt, sizeof alt, " · también pensó: %s %.2f, %s %.2f", NYX_ACC[segunda], punt[segunda], NYX_ACC[tercera], punt[tercera]);
        else if (segunda >= 0)
            snprintf(alt, sizeof alt, " · también pensó: %s %.2f", NYX_ACC[segunda], punt[segunda]);
        switch (a) {
        case ACC_HABLAR:    snprintf(t, sizeof t, "quiero hablar de «%s»%s", tt, alt); break;
        case ACC_PREGUNTAR: snprintf(t, sizeof t, "quiero preguntar%s%s%s", duda >= 0 ? " cómo se dice «" : " algo", duda >= 0 ? m->s[duda].et : "", duda >= 0 ? "»" : ""); { size_t l = strlen(t); snprintf(t + l, sizeof t - l, "%s", alt); } break;
        case ACC_IMAGINAR:  snprintf(t, sizeof t, "quiero imaginar algo nuevo%s", alt); break;
        case ACC_SONAR:     snprintf(t, sizeof t, "quiero soñar y ordenar lo que viví%s", alt); break;
        case ACC_RECORDAR:  snprintf(t, sizeof t, "quiero contar lo que recuerdo%s", alt); break;
        default:            snprintf(t, sizeof t, "quiero escuchar%s", alt); break;
        }
        c_evento(c, rol, EV_PIENSA, t);
    }
    switch (a) {
    case ACC_HABLAR: {
        int modo = nyx_ent(10);
        const char *pre = "mi ko ";
        if (m_coh(m, tema) < 0.2f) pre = "ye ";
        else if ((rol == 5 || rol == 12) && modo < 5) pre = "ne ";
        rec = c_habla(c, rol, tema, pre);
        break;
    }
    case ACC_PREGUNTAR: {
        if (duda >= 0) {
            int r, quien = -1;
            float mejorP = -1;
            snprintf(t, sizeof t, "ye %s?   «¿cómo se dice %s?»", m->s[duda].et, m->s[duda].et);
            c_evento(c, rol, EV_DICE, t);
            for (r = 0; r < NROLES; r++) {
                int id;
                if (r == rol) continue;
                id = m_busca(&c->m[r], m->s[duda].et);
                if (id >= 0 && c->m[r].s[id].sabe && m_precision(&c->m[r]) + nyx_rango(0, 0.3f) > mejorP) {
                    mejorP = m_precision(&c->m[r]);
                    quien = r;
                }
            }
            if (quien >= 0) {
                const char *rr = nyx_a_resh(m->s[duda].et);
                int r2, extra = 0;
                snprintf(t, sizeof t, "%s ka %s   «%s se dice %s»", m->s[duda].et, rr, m->s[duda].et, rr);
                c_evento(c, quien, EV_DICE, t);
                m->s[duda].sabe = 1;
                m->reshAprendidas++;
                for (r2 = 0; r2 < NROLES; r2++) {
                    int id;
                    if (r2 == rol || r2 == quien) continue;
                    id = m_busca(&c->m[r2], m->s[duda].et);
                    if (id >= 0 && !c->m[r2].s[id].sabe && nyx_f01() < 0.5f) {
                        c->m[r2].s[id].sabe = 1; c->m[r2].reshAprendidas++; extra++;
                    }
                }
                snprintf(t, sizeof t, "aprendió «%s» = %s%s", m->s[duda].et, rr, extra ? " (y otras que oyeron también)" : "");
                c_evento(c, rol, EV_RESH, t);
                rec = 1.5f;
            } else {
                c_evento(c, rol, EV_NOTA, "nadie supo contestarle");
                rec = -0.3f;
            }
        } else if (tema >= 0) {
            /* sin dudas de idioma: pregunta por una idea y otra le responde */
            int r = nyx_ent(NROLES), ids[MAXEST], k, cr;
            float C, E;
            if (r == rol) r = (r + 1) % NROLES;
            snprintf(t, sizeof t, "ye %s?   «¿qué piensas de %s, %s?»", m->s[tema].et, m->s[tema].et, NYX_ROLES[r].nombre);
            c_evento(c, rol, EV_DICE, t);
            k = m_recibe(&c->m[r], m->s[tema].et, NYX_ROLES[rol].nombre, ids, MAXEST);
            if (k > 0) {
                int g = m_veredicto(&c->m[r], ids, k, &C, &E, &cr);
                if (g >= 0) rec = c_habla(c, r, g, "");
                rec = rec > 0 ? 1.0f : 0.1f;
            }
        } else rec = -0.2f;
        break;
    }
    case ACC_IMAGINAR: {
        int pa, pb, id = m_imagina(m, &pa, &pb), x;
        if (id >= 0) {
            float C;
            for (x = 0; x < 10; x++) { m_decae(m, id, m->ruido); m_propaga(m, id); m_propaga(m, pa); m_propaga(m, pb); }
            C = m_coh(m, id);
            snprintf(t, sizeof t, "imaginó «%s» (de %s y %s) · coherencia %.2f", m->s[id].et, m->s[pa].et, m->s[pb].et, C);
            c_evento(c, rol, EV_IDEA, t);
            m_atiende(m, id);
            rec = nyx_lim(C * 2.0f, -1, 1.5f);
        } else rec = -0.3f;
        break;
    }
    case ACC_SONAR: {
        int i2, ids[8], n = 0, cr = -1;
        float antes = 0, despues = 0;
        for (i2 = 0; i2 < m->nfoco; i2++) antes += m_coh(m, m->foco[i2]);
        for (i2 = 0; i2 < 12; i2++) m_piensa(m);
        /* repasa lo más hablado y razona a saltos desde ahí */
        for (i2 = 0; i2 < m->n && n < 8; i2++) {
            int x = nyx_ent(m->n);
            if (m->s[x].dialogo > 0 && !m->s[x].fusion && !nyx_vacia(m->s[x].et)) ids[n++] = x;
        }
        for (i2 = 0; i2 < n; i2++) {
            m->s[ids[i2]].A = nyx_min(1, m->s[ids[i2]].A + 0.08f);
            if (cr < 0) cr = m_transitiva(m, ids[i2]);
        }
        for (i2 = 0; i2 < m->nfoco; i2++) despues += m_coh(m, m->foco[i2]);
        if (cr >= 0) {
            snprintf(t, sizeof t, "soñando cristalizó un concepto: «%s»", m->s[cr].et);
            c_evento(c, rol, EV_IDEA, t);
        } else {
            snprintf(t, sizeof t, "soñó con %d recuerdos · coherencia %+.2f", n, despues - antes);
            c_evento(c, rol, EV_SUENA, t);
        }
        rec = nyx_lim((despues - antes) * 0.5f + (cr >= 0 ? 1.0f : 0.1f), -1, 1.5f);
        break;
    }
    case ACC_RECORDAR: {
        int ids[MAXFRASE], n = 0, i2;
        for (i2 = m->nepis - 1; i2 >= 0 && n < 3; i2--) {
            int x, ya = 0;
            for (x = 0; x < n; x++) if (ids[x] == m->epis[i2]) ya = 1;
            if (!ya && !m->s[m->epis[i2]].fusion) ids[n++] = m->epis[i2];
        }
        if (n >= 2) {
            char es[400], resh[400], linea[900];
            int aprend;
            char palabra[LARGO];
            float res;
            /* en orden: lo más viejo primero */
            for (i2 = 0; i2 < n / 2; i2++) { int tmp = ids[i2]; ids[i2] = ids[n - 1 - i2]; ids[n - 1 - i2] = tmp; }
            m_texto(m, ids, n, es, sizeof es, resh, sizeof resh);
            snprintf(linea, sizeof linea, "mi sor ra: %s   «recuerdo: %s»", resh, es);
            c_evento(c, rol, EV_DICE, linea);
            res = c_difunde(c, rol, ids, n, &aprend, palabra);
            rec = 1.2f * tanhf(res);
        } else rec = -0.2f;
        break;
    }
    default: {
        int r = nyx_ent(NROLES);
        Mente *o = &c->m[r];
        if (o->nven > 0 && r != rol) {
            const char *ult = o->ven[(o->iven + NVEN - 1) % NVEN];
            const char *dos = strstr(ult, ": ");
            int ids[MAXEST];
            m_recibe(m, dos ? dos + 2 : ult, NYX_ROLES[r].nombre, ids, MAXEST);
            snprintf(t, sizeof t, "escuchó a %s", NYX_ROLES[r].nombre);
            c_evento(c, rol, EV_NOTA, t);
        }
        rec = 0.2f;
        break;
    }
    }
    c_aprende(c, rol, a, rec);
}

/* Pregunta del humano a las 18: deliberación con votos. */
void consejo_delibera(Consejo *c, const char *pregunta, NyxRespuesta *out) {
    int r, i, j, gan[NROLES];
    float peso[NROLES], C[NROLES], E[NROLES], mejor = -1;
    char etiq[NROLES][LARGO];
    float suma[NROLES];
    int nEtq = 0, ganE = -1;
    memset(out, 0, sizeof *out);
    c->fbValido = 1; c->fbN = 0; c->fbFr = -1;
    for (r = 0; r < NROLES; r++) {
        Mente *m = &c->m[r];
        int ids[MAXEST], k = m_recibe(m, pregunta, "humano", ids, MAXEST), cr;
        gan[r] = k > 0 ? m_veredicto(m, ids, k, &C[r], &E[r], &cr) : -1;
        c->fbNest[r] = k;
        for (i = 0; i < k; i++) c->fbEst[r][i] = ids[i];
        c->fbGan[r] = gan[r];
        if (gan[r] < 0) { peso[r] = 0; continue; }
        peso[r] = (0.3f + m_precision(m)) * (0.3f + nyx_max(0, C[r])) * (0.5f + E[r]);
        if (cr >= 0) {
            char t[96];
            snprintf(t, sizeof t, "cristalizó un concepto: «%s»", m->s[cr].et);
            c_evento(c, r, EV_IDEA, t);
        }
    }
    for (r = 0; r < NROLES; r++) {         /* votos por etiqueta */
        if (gan[r] < 0) continue;
        for (j = 0; j < nEtq; j++) if (strcmp(etiq[j], c->m[r].s[gan[r]].et) == 0) break;
        if (j == nEtq) { nyx_copia(etiq[nEtq], c->m[r].s[gan[r]].et, LARGO); suma[nEtq] = 0; nEtq++; }
        suma[j] += peso[r];
    }
    for (j = 0; j < nEtq; j++) if (ganE < 0 || suma[j] > suma[ganE]) ganE = j;
    if (ganE < 0) { snprintf(out->frase, sizeof out->frase, "(silencio: no entendimos nada)"); c->fbValido = 0; return; }
    nyx_copia(out->ganador, etiq[ganE], LARGO);
    out->vocero = -1;
    {
        int acuerdo = 0;
        for (r = 0; r < NROLES; r++) {
            int ok = gan[r] >= 0 && strcmp(c->m[r].s[gan[r]].et, etiq[ganE]) == 0;
            NyxVoto *v = &out->votos[out->nvotos++];
            v->rol = r; v->C = gan[r] >= 0 ? C[r] : 0; v->E = gan[r] >= 0 ? E[r] : 0; v->peso = peso[r]; v->acuerdo = ok;
            nyx_copia(v->atractor, gan[r] >= 0 ? c->m[r].s[gan[r]].et : "-", LARGO);
            if (gan[r] >= 0) {
                c->m[r].intentos++;
                if (ok) c->m[r].aciertos++;
                if (c->m[r].intentos > 200) { c->m[r].intentos = 100; c->m[r].aciertos /= 2; }
            }
            if (ok) {
                acuerdo++;
                c->fbMente[c->fbN++] = r;
                if (peso[r] > mejor) { mejor = peso[r]; out->vocero = r; }
            }
        }
        out->acuerdo = (float)acuerdo / (float)NROLES;
    }
    /* ordena los votos por peso */
    for (i = 0; i < out->nvotos; i++)
        for (j = i + 1; j < out->nvotos; j++)
            if (out->votos[j].peso > out->votos[i].peso) { NyxVoto t = out->votos[i]; out->votos[i] = out->votos[j]; out->votos[j] = t; }
    {
        Mente *v = &c->m[out->vocero];
        int aprend;
        char palabra[LARGO], es[400], resh[400];
        out->recordada = c_compone(c, v, gan[out->vocero], c->fbEst[out->vocero], c->fbNest[out->vocero], 0.9f,
                                   c->fbFrase, &c->fbNfrase, es, sizeof es, resh, sizeof resh, &c->fbFr);
        c->fbRecordada = out->recordada;
        c->fbVocero = out->vocero;
        c_pon_eco(c, es);
        if (out->acuerdo < 0.25f) {
            snprintf(out->frase, sizeof out->frase, "¿%s?", es);
            snprintf(out->resh, sizeof out->resh, "ye %s?", resh);
        } else {
            snprintf(out->frase, sizeof out->frase, "%s", es);
            snprintf(out->resh, sizeof out->resh, "se ko %s", resh);
        }
        c_difunde(c, out->vocero, c->fbFrase, c->fbNfrase, &aprend, palabra);
    }
}

/* Hablar con una sola mente. */
void consejo_habla_con(Consejo *c, int rol, const char *texto, char *es, size_t ces, char *resh, size_t cre, float *C) {
    Mente *m = &c->m[rol];
    int ids[MAXEST], k = m_recibe(m, texto, "humano", ids, MAXEST), g, cr;
    float E;
    es[0] = 0; resh[0] = 0; *C = 0;
    c->fbValido = 0; c->fbFr = -1;
    if (k == 0) { snprintf(es, ces, "(no entendí)"); return; }
    {   /* si iba a repetir lo que se acaba de decir, piensa otra cosa */
        int intento;
        for (intento = 0; intento < 4; intento++) {
            g = m_veredicto(m, ids, k, C, &E, &cr);
            if (g < 0) { snprintf(es, ces, "(no entendí)"); return; }
            c->fbRecordada = c_compone(c, m, g, ids, k, nyx_max(0.8f, c_prob_recuerdo(rol)), c->fbFrase, &c->fbNfrase, es, ces, resh, cre, &c->fbFr);
            if (!c_es_eco(c, es)) break;
        }
    }
    c_pon_eco(c, es);
    c->fbValido = 1; c->fbN = 1; c->fbMente[0] = rol; c->fbGan[rol] = g; c->fbNest[rol] = k;
    memcpy(c->fbEst[rol], ids, (size_t)k * sizeof ids[0]);
    c->fbVocero = rol;
    m_cansa(m, c->fbFrase, c->fbNfrase);
    if (cr >= 0) {
        char t[96];
        snprintf(t, sizeof t, "cristalizó un concepto: «%s»", m->s[cr].et);
        c_evento(c, rol, EV_IDEA, t);
    }
}

/* Tu opinión sobre la última respuesta reestructura lo aprendido. */
int consejo_opina(Consejo *c, int bueno) {
    int i, k, x;
    if (!c->fbValido) return 0;
    for (i = 0; i < c->fbN; i++) {
        int r = c->fbMente[i], g = c->fbGan[r];
        Mente *m = &c->m[r];
        if (g < 0) continue;
        for (k = 0; k < c->fbNest[r]; k++) {
            int e = c->fbEst[r][k];
            if (e == g || nyx_vacia(m->s[e].et)) continue;
            if (bueno) { m_refuerza(m, e, g, 0.3f, 0, 0); m_refuerza(m, g, e, 0.2f, 0, 0); }
            else { m_fija(m, e, g, 0.7f, NYX_PI); m_fija(m, g, e, 0.7f, NYX_PI); }
        }
        if (bueno) { m->aciertos++; m->intentos++; m->valor[ACC_HABLAR] += 0.05f; }
        else { m->intentos++; m->valor[ACC_HABLAR] -= 0.05f; }
    }
    frases_opina(c, c->fbFr, bueno);     /* la frase recordada también se juzga */
    if (bueno) {     /* la frase dicha queda como buena secuencia */
        Mente *v = &c->m[c->fbVocero];
        for (x = 0; x + 1 < c->fbNfrase; x++) m_refuerza(v, c->fbFrase[x], c->fbFrase[x + 1], 0.1f, 0.3f, 0.2f);
    }
    c->fbValido = 0;
    return 1;
}
