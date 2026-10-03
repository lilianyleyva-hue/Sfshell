/* nyx_unico.c — Nyx completo en UN solo archivo (interfaz + cerebro + Resh).
 * En Code App: ponlo en la carpeta abierta y pulsa ▶. No necesita nada más.
 * (Se genera juntando nyx.c, cerebro.h y resh.h.)
 */
/* nyx.c — Interfaz de texto de las 18 IAs de Nyx (C99).
 *
 * Se ejecuta en la terminal de Code App (o en cualquier terminal):
 *   abre nyx.c y pulsa ▶   ·   o:  clang nyx.c -o nyx -lm && ./nyx
 * Necesita cerebro.h y resh.h en la misma carpeta.
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
/* cerebro.h — Cerebro resonante de las 18 IAs de Nyx, en C (C99).
 *
 * Es el mismo paradigma del CerebroResonante de Nyxshell (Swift), llevado a C
 * y mejorado:
 *   - Cada mente es un fluido de "semiones" (palabras/ideas) con amplitud A,
 *     fase y carga. Pensar = dejar que las fases se sincronicen (Kuramoto)
 *     hasta que gana un atractor de coherencia.
 *   - MEJORA: memoria de secuencia. Además de la asociación, cada acople
 *     guarda cuánto "A va seguido de B". Así las mentes responden con frases
 *     en español aprendidas de lo que leen y oyen, no con palabras sueltas.
 *   - MEJORA: las palabras vacías (el, la, de, que…) no ganan veredictos.
 *   - MEJORA: la lengua Resh se reparte: cada mente sabe una parte, pregunta
 *     lo que no sabe ("ye árbol?") y las demás le enseñan.
 *   - MEJORA: imaginación con aprendizaje por refuerzo. Cada mente imagina
 *     qué quiere hacer (hablar, preguntar, imaginar, soñar, recordar,
 *     escuchar), lo hace, mide cómo le fue y aprende qué le conviene.
 *   - MEJORA: tu opinión (bien/mal) reestructura lo aprendido: lo malo queda
 *     en oposición de fase y pierde futuros veredictos.
 *   - Persistencia en un archivo de texto.
 *
 * Solo usa la biblioteca estándar de C: compila con clang/gcc y en Code App.
 */
#ifndef NYX_CEREBRO_H
#define NYX_CEREBRO_H

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>
/* resh.h — léxico de la lengua Resh (generado del léxico Swift de Nyxshell).
 * Español -> Resh. No hace falta editarlo a mano. */
#ifndef NYX_RESH_H
#define NYX_RESH_H

typedef struct { const char *es; const char *resh; } ParResh;

/* partículas: la capa de coordinación (forma, significado) */
static const ParResh RESH_PARTICULAS[] = {
    {"marca de agente: quien hace la acción", "ka"},
    {"marca de paciente: quien recibe la acción", "to"},
    {"no / negación", "na"},
    {"marcador de pregunta", "ye"},
    {"pasado", "ra"},
    {"futuro", "li"},
    {"y (conjunción)", "va"},
    {"yo", "mi"},
    {"tú", "ti"},
    {"nosotros (el consejo)", "se"},
    {"acuerdo / alineamiento entre mentes", "ko"},
    {"desacuerdo", "ne"},
    {"esto", "sha"},
    {"eso", "ta"},
    {"para / hacia", "pa"},
    {"en / dentro de", "so"},
};
#define N_RESH_PARTICULAS ((int)(sizeof RESH_PARTICULAS / sizeof RESH_PARTICULAS[0]))

/* concepto -> partícula */
static const ParResh RESH_PROTOCOLO[] = {
    {"acuerdo", "ko"},
    {"desacuerdo", "ne"},
    {"yo", "mi"},
    {"tú", "ti"},
    {"nosotros", "se"},
    {"y", "va"},
    {"no", "na"},
    {"esto", "sha"},
    {"eso", "ta"},
    {"para", "pa"},
    {"en", "so"},
    {"agente", "ka"},
    {"paciente", "to"},
    {"pregunta", "ye"},
    {"pasado", "ra"},
    {"futuro", "li"},
};
#define N_RESH_PROTOCOLO ((int)(sizeof RESH_PROTOCOLO / sizeof RESH_PROTOCOLO[0]))

/* 3144 palabras */
static const ParResh RESH_LEXICO[] = {
    {"el", "kak"},
    {"la", "kat"},
    {"los", "kap"},
    {"las", "kas"},
    {"un", "kam"},
    {"una", "kan"},
    {"unos", "kar"},
    {"unas", "kal"},
    {"de", "kav"},
    {"que", "kaz"},
    {"a", "kay"},
    {"por", "kek"},
    {"con", "ket"},
    {"como", "kep"},
    {"se", "kes"},
    {"su", "kem"},
    {"sus", "ken"},
    {"al", "ker"},
    {"del", "kel"},
    {"lo", "kev"},
    {"le", "kez"},
    {"les", "key"},
    {"me", "kik"},
    {"te", "kit"},
    {"nos", "kip"},
    {"os", "kis"},
    {"este", "kim"},
    {"esta", "kin"},
    {"estos", "kir"},
    {"estas", "kil"},
    {"ese", "kiv"},
    {"esa", "kiz"},
    {"esos", "kiy"},
    {"esas", "kok"},
    {"aquel", "kot"},
    {"aquella", "kop"},
    {"aquellos", "kos"},
    {"aquellas", "kom"},
    {"aquello", "kon"},
    {"quien", "kor"},
    {"quienes", "kol"},
    {"cual", "kov"},
    {"cuales", "koz"},
    {"cuyo", "koy"},
    {"cuya", "kuk"},
    {"cuyos", "kut"},
    {"cuyas", "kup"},
    {"cuanto", "kus"},
    {"cuanta", "kum"},
    {"cuantos", "kun"},
    {"cuantas", "kur"},
    {"donde", "kul"},
    {"cuando", "kuv"},
    {"porque", "kuz"},
    {"pues", "kuy"},
    {"aunque", "tak"},
    {"mientras", "tat"},
    {"hasta", "tap"},
    {"desde", "tas"},
    {"entre", "tam"},
    {"contra", "tan"},
    {"sin", "tar"},
    {"sobre", "tal"},
    {"tras", "tav"},
    {"durante", "taz"},
    {"mediante", "tay"},
    {"según", "tek"},
    {"excepto", "tet"},
    {"hacia", "tep"},
    {"además", "tes"},
    {"también", "tem"},
    {"tampoco", "ten"},
    {"ni", "ter"},
    {"o", "tel"},
    {"u", "tev"},
    {"pero", "tez"},
    {"sino", "tey"},
    {"si", "tik"},
    {"cada", "tit"},
    {"todo", "tip"},
    {"toda", "tis"},
    {"todos", "tim"},
    {"todas", "tin"},
    {"otro", "tir"},
    {"otra", "til"},
    {"otros", "tiv"},
    {"otras", "tiz"},
    {"mismo", "tiy"},
    {"misma", "tok"},
    {"mismos", "tot"},
    {"mismas", "top"},
    {"tal", "tos"},
    {"tales", "tom"},
    {"qué", "ton"},
    {"quién", "tor"},
    {"cuál", "tol"},
    {"cuánto", "tov"},
    {"cuánta", "toz"},
    {"dónde", "toy"},
    {"cómo", "tuk"},
    {"ambos", "tut"},
    {"ambas", "tup"},
    {"demasiado", "tus"},
    {"demasiada", "tum"},
    {"demasiados", "tun"},
    {"demasiadas", "tur"},
    {"bastante", "tul"},
    {"bastantes", "tuv"},
    {"poco", "tuz"},
    {"poca", "tuy"},
    {"pocos", "pak"},
    {"pocas", "pat"},
    {"mucho", "pap"},
    {"mucha", "pas"},
    {"muchos", "pam"},
    {"muchas", "pan"},
    {"tanto", "par"},
    {"tanta", "pal"},
    {"tantos", "pav"},
    {"tantas", "paz"},
    {"varios", "pay"},
    {"varias", "pek"},
    {"algún", "pet"},
    {"alguna", "pep"},
    {"algunos", "pes"},
    {"algunas", "pem"},
    {"ningún", "pen"},
    {"ninguna", "per"},
    {"ninguno", "pel"},
    {"nadie", "pev"},
    {"nada", "pez"},
    {"alguien", "pey"},
    {"algo", "pik"},
    {"cualquiera", "pit"},
    {"mío", "pip"},
    {"mía", "pis"},
    {"míos", "pim"},
    {"mías", "pin"},
    {"tuyo", "pir"},
    {"tuya", "pil"},
    {"tuyos", "piv"},
    {"tuyas", "piz"},
    {"suyo", "piy"},
    {"suya", "pok"},
    {"suyos", "pot"},
    {"suyas", "pop"},
    {"nuestro", "pos"},
    {"nuestra", "pom"},
    {"nuestros", "pon"},
    {"nuestras", "por"},
    {"vuestro", "pol"},
    {"vuestra", "pov"},
    {"mi", "poz"},
    {"mis", "poy"},
    {"tu", "puk"},
    {"tus", "put"},
    {"ser", "pup"},
    {"estar", "pus"},
    {"tener", "pum"},
    {"hacer", "pun"},
    {"poder", "pur"},
    {"decir", "pul"},
    {"ir", "puv"},
    {"ver", "puz"},
    {"dar", "puy"},
    {"saber", "sak"},
    {"querer", "sat"},
    {"llegar", "sap"},
    {"pasar", "sas"},
    {"deber", "sam"},
    {"poner", "san"},
    {"parecer", "sar"},
    {"quedar", "sal"},
    {"creer", "sav"},
    {"hablar", "saz"},
    {"llevar", "say"},
    {"dejar", "sek"},
    {"seguir", "set"},
    {"encontrar", "sep"},
    {"llamar", "ses"},
    {"venir", "sem"},
    {"pensar", "sen"},
    {"salir", "ser"},
    {"volver", "sel"},
    {"tomar", "sev"},
    {"conocer", "sez"},
    {"vivir", "sey"},
    {"sentir", "sik"},
    {"tratar", "sit"},
    {"mirar", "sip"},
    {"contar", "sis"},
    {"empezar", "sim"},
    {"esperar", "sin"},
    {"buscar", "sir"},
    {"existir", "sil"},
    {"entrar", "siv"},
    {"trabajar", "siz"},
    {"escribir", "siy"},
    {"perder", "sok"},
    {"producir", "sot"},
    {"ocurrir", "sop"},
    {"entender", "sos"},
    {"pedir", "som"},
    {"recibir", "son"},
    {"recordar", "sor"},
    {"terminar", "sol"},
    {"permitir", "sov"},
    {"aparecer", "soz"},
    {"conseguir", "soy"},
    {"comenzar", "suk"},
    {"servir", "sut"},
    {"sacar", "sup"},
    {"necesitar", "sus"},
    {"mantener", "sum"},
    {"resultar", "sun"},
    {"leer", "sur"},
    {"caer", "sul"},
    {"cambiar", "suv"},
    {"presentar", "suz"},
    {"crear", "suy"},
    {"abrir", "mak"},
    {"considerar", "mat"},
    {"oír", "map"},
    {"acabar", "mas"},
    {"convertir", "mam"},
    {"ganar", "man"},
    {"formar", "mar"},
    {"traer", "mal"},
    {"aceptar", "mav"},
    {"realizar", "maz"},
    {"suponer", "may"},
    {"lograr", "mek"},
    {"explicar", "met"},
    {"preguntar", "mep"},
    {"tocar", "mes"},
    {"reconocer", "mem"},
    {"estudiar", "men"},
    {"alcanzar", "mer"},
    {"nacer", "mel"},
    {"andar", "mev"},
    {"aprender", "mez"},
    {"ayudar", "mey"},
    {"amar", "mik"},
    {"bailar", "mit"},
    {"bajar", "mip"},
    {"beber", "mis"},
    {"cantar", "mim"},
    {"cerrar", "min"},
    {"cocinar", "mir"},
    {"comer", "mil"},
    {"comprar", "miv"},
    {"comprender", "miz"},
    {"correr", "miy"},
    {"cortar", "mok"},
    {"crecer", "mot"},
    {"cruzar", "mop"},
    {"cuidar", "mos"},
    {"cumplir", "mom"},
    {"decidir", "mon"},
    {"defender", "mor"},
    {"descansar", "mol"},
    {"describir", "mov"},
    {"descubrir", "moz"},
    {"desear", "moy"},
    {"dibujar", "muk"},
    {"discutir", "mut"},
    {"disfrutar", "mup"},
    {"dormir", "mus"},
    {"elegir", "mum"},
    {"emplear", "mun"},
    {"empujar", "mur"},
    {"encender", "mul"},
    {"enseñar", "muv"},
    {"entrenar", "muz"},
    {"enviar", "muy"},
    {"equivocar", "nak"},
    {"escapar", "nat"},
    {"escuchar", "nap"},
    {"establecer", "nas"},
    {"evitar", "nam"},
    {"examinar", "nan"},
    {"exigir", "nar"},
    {"explorar", "nal"},
    {"expresar", "nav"},
    {"extender", "naz"},
    {"faltar", "nay"},
    {"firmar", "nek"},
    {"flotar", "net"},
    {"fracasar", "nep"},
    {"fumar", "nes"},
    {"funcionar", "nem"},
    {"gastar", "nen"},
    {"girar", "ner"},
    {"gobernar", "nel"},
    {"golpear", "nev"},
    {"gritar", "nez"},
    {"guardar", "ney"},
    {"gustar", "nik"},
    {"hallar", "nit"},
    {"hervir", "nip"},
    {"huir", "nis"},
    {"ignorar", "nim"},
    {"imaginar", "nin"},
    {"imitar", "nir"},
    {"importar", "nil"},
    {"incluir", "niv"},
    {"indicar", "niz"},
    {"influir", "niy"},
    {"informar", "nok"},
    {"insistir", "not"},
    {"instalar", "nop"},
    {"intentar", "nos"},
    {"interesar", "nom"},
    {"interrumpir", "non"},
    {"intervenir", "nor"},
    {"introducir", "nol"},
    {"intuir", "nov"},
    {"inventar", "noz"},
    {"invertir", "noy"},
    {"investigar", "nuk"},
    {"invitar", "nut"},
    {"jugar", "nup"},
    {"juntar", "nus"},
    {"juzgar", "num"},
    {"lanzar", "nun"},
    {"lavar", "nur"},
    {"levantar", "nul"},
    {"liberar", "nuv"},
    {"limitar", "nuz"},
    {"limpiar", "nuy"},
    {"llorar", "rak"},
    {"luchar", "rat"},
    {"mandar", "rap"},
    {"manejar", "ras"},
    {"marcar", "ram"},
    {"marchar", "ran"},
    {"matar", "rar"},
    {"medir", "ral"},
    {"mejorar", "rav"},
    {"mencionar", "raz"},
    {"mentir", "ray"},
    {"merecer", "rek"},
    {"meter", "ret"},
    {"mezclar", "rep"},
    {"morder", "res"},
    {"morir", "rem"},
    {"mostrar", "ren"},
    {"mover", "rer"},
    {"nadar", "rel"},
    {"narrar", "rev"},
    {"navegar", "rez"},
    {"negar", "rey"},
    {"negociar", "rik"},
    {"nombrar", "rit"},
    {"notar", "rip"},
    {"obedecer", "ris"},
    {"obligar", "rim"},
    {"observar", "rin"},
    {"obtener", "rir"},
    {"ocultar", "ril"},
    {"ocupar", "riv"},
    {"odiar", "riz"},
    {"ofrecer", "riy"},
    {"oler", "rok"},
    {"olvidar", "rot"},
    {"operar", "rop"},
    {"opinar", "ros"},
    {"optar", "rom"},
    {"ordenar", "ron"},
    {"organizar", "ror"},
    {"pagar", "rol"},
    {"parar", "rov"},
    {"participar", "roz"},
    {"partir", "roy"},
    {"pasear", "ruk"},
    {"pegar", "rut"},
    {"peinar", "rup"},
    {"pelear", "rus"},
    {"percibir", "rum"},
    {"perdonar", "run"},
    {"permanecer", "rur"},
    {"perseguir", "rul"},
    {"pertenecer", "ruv"},
    {"pesar", "ruz"},
    {"pescar", "ruy"},
    {"pintar", "lak"},
    {"pisar", "lat"},
    {"plantar", "lap"},
    {"portar", "las"},
    {"poseer", "lam"},
    {"practicar", "lan"},
    {"preceder", "lar"},
    {"predecir", "lal"},
    {"preferir", "lav"},
    {"premiar", "laz"},
    {"preocupar", "lay"},
    {"preparar", "lek"},
    {"presenciar", "let"},
    {"prestar", "lep"},
    {"pretender", "les"},
    {"probar", "lem"},
    {"proceder", "len"},
    {"procesar", "ler"},
    {"programar", "lel"},
    {"progresar", "lev"},
    {"prohibir", "lez"},
    {"prometer", "ley"},
    {"promover", "lik"},
    {"pronunciar", "lit"},
    {"proponer", "lip"},
    {"proteger", "lis"},
    {"provocar", "lim"},
    {"proyectar", "lin"},
    {"publicar", "lir"},
    {"quebrar", "lil"},
    {"quejar", "liv"},
    {"quemar", "liz"},
    {"quitar", "liy"},
    {"reaccionar", "lok"},
    {"rebotar", "lot"},
    {"reciclar", "lop"},
    {"reclamar", "los"},
    {"recoger", "lom"},
    {"recomendar", "lon"},
    {"recorrer", "lor"},
    {"reducir", "lol"},
    {"reemplazar", "lov"},
    {"reflejar", "loz"},
    {"reformar", "loy"},
    {"regalar", "luk"},
    {"regar", "lut"},
    {"regresar", "lup"},
    {"reír", "lus"},
    {"relajar", "lum"},
    {"relatar", "lun"},
    {"remar", "lur"},
    {"rendir", "lul"},
    {"renovar", "luv"},
    {"renunciar", "luz"},
    {"reparar", "luy"},
    {"repartir", "vak"},
    {"repasar", "vat"},
    {"repetir", "vap"},
    {"reportar", "vas"},
    {"representar", "vam"},
    {"reproducir", "van"},
    {"requerir", "var"},
    {"rescatar", "val"},
    {"reservar", "vav"},
    {"resolver", "vaz"},
    {"resonar", "vay"},
    {"respetar", "vek"},
    {"respirar", "vet"},
    {"responder", "vep"},
    {"restar", "ves"},
    {"restaurar", "vem"},
    {"resumir", "ven"},
    {"retener", "ver"},
    {"retirar", "vel"},
    {"reunir", "vev"},
    {"revelar", "vez"},
    {"revisar", "vey"},
    {"rezar", "vik"},
    {"robar", "vit"},
    {"rodar", "vip"},
    {"rogar", "vis"},
    {"romper", "vim"},
    {"saborear", "vin"},
    {"sacudir", "vir"},
    {"salar", "vil"},
    {"saltar", "viv"},
    {"saludar", "viz"},
    {"salvar", "viy"},
    {"sanar", "vok"},
    {"secar", "vot"},
    {"seducir", "vop"},
    {"sembrar", "vos"},
    {"sentar", "vom"},
    {"separar", "von"},
    {"significar", "vor"},
    {"silbar", "vol"},
    {"simular", "vov"},
    {"sincronizar", "voz"},
    {"sobrar", "voy"},
    {"sobrevivir", "vuk"},
    {"soñar", "vut"},
    {"soldar", "vup"},
    {"solicitar", "vus"},
    {"soltar", "vum"},
    {"solucionar", "vun"},
    {"sonar", "vur"},
    {"sonreír", "vul"},
    {"soportar", "vuv"},
    {"sorprender", "vuz"},
    {"sospechar", "vuy"},
    {"sostener", "zak"},
    {"subir", "zat"},
    {"suceder", "zap"},
    {"sudar", "zas"},
    {"sufrir", "zam"},
    {"sugerir", "zan"},
    {"sumar", "zar"},
    {"superar", "zal"},
    {"surgir", "zav"},
    {"suspirar", "zaz"},
    {"sustituir", "zay"},
    {"susurrar", "zek"},
    {"tapar", "zet"},
    {"tardar", "zep"},
    {"tejer", "zes"},
    {"temblar", "zem"},
    {"temer", "zen"},
    {"tender", "zer"},
    {"tentar", "zel"},
    {"tirar", "zev"},
    {"tolerar", "zez"},
    {"torcer", "zey"},
    {"toser", "zik"},
    {"traducir", "zit"},
    {"tragar", "zip"},
    {"traicionar", "zis"},
    {"transferir", "zim"},
    {"transformar", "zin"},
    {"transmitir", "zir"},
    {"transportar", "zil"},
    {"trazar", "ziv"},
    {"tropezar", "ziz"},
    {"ubicar", "ziy"},
    {"unir", "zok"},
    {"usar", "zot"},
    {"utilizar", "zop"},
    {"vaciar", "zos"},
    {"valer", "zom"},
    {"variar", "zon"},
    {"vencer", "zor"},
    {"vender", "zol"},
    {"verificar", "zov"},
    {"vestir", "zoz"},
    {"viajar", "zoy"},
    {"vibrar", "zuk"},
    {"vigilar", "zut"},
    {"vincular", "zup"},
    {"visitar", "zus"},
    {"volar", "zum"},
    {"volcar", "zun"},
    {"votar", "zur"},
    {"zumbar", "zul"},
    {"tiempo", "zuv"},
    {"año", "zuz"},
    {"día", "zuy"},
    {"noche", "yak"},
    {"mañana", "yat"},
    {"tarde", "yap"},
    {"semana", "yas"},
    {"mes", "yam"},
    {"hora", "yan"},
    {"minuto", "yar"},
    {"segundo", "yal"},
    {"momento", "yav"},
    {"rato", "yaz"},
    {"época", "yay"},
    {"siglo", "yek"},
    {"instante", "yet"},
    {"vez", "yep"},
    {"vida", "yes"},
    {"muerte", "yem"},
    {"mundo", "yen"},
    {"tierra", "yer"},
    {"país", "yel"},
    {"ciudad", "yev"},
    {"pueblo", "yez"},
    {"casa", "yey"},
    {"hogar", "yik"},
    {"edificio", "yit"},
    {"calle", "yip"},
    {"plaza", "yis"},
    {"parque", "yim"},
    {"puente", "yin"},
    {"puerta", "yir"},
    {"ventana", "yil"},
    {"pared", "yiv"},
    {"techo", "yiz"},
    {"suelo", "yiy"},
    {"cuarto", "yok"},
    {"cocina", "yot"},
    {"baño", "yop"},
    {"sala", "yos"},
    {"cama", "yom"},
    {"mesa", "yon"},
    {"silla", "yor"},
    {"luz", "yol"},
    {"sombra", "yov"},
    {"agua", "yoz"},
    {"fuego", "yoy"},
    {"aire", "yuk"},
    {"piedra", "yut"},
    {"arena", "yup"},
    {"mar", "yus"},
    {"río", "yum"},
    {"lago", "yun"},
    {"montaña", "yur"},
    {"valle", "yul"},
    {"bosque", "yuv"},
    {"árbol", "yuz"},
    {"flor", "yuy"},
    {"hoja", "kakk"},
    {"raíz", "kakt"},
    {"fruta", "kakp"},
    {"semilla", "kaks"},
    {"sol", "kakm"},
    {"luna", "kakn"},
    {"estrella", "kakr"},
    {"cielo", "kakl"},
    {"nube", "kakv"},
    {"lluvia", "kakz"},
    {"nieve", "kaky"},
    {"viento", "katk"},
    {"trueno", "katt"},
    {"rayo", "katp"},
    {"animal", "kats"},
    {"perro", "katm"},
    {"gato", "katn"},
    {"pájaro", "katr"},
    {"pez", "katl"},
    {"caballo", "katv"},
    {"vaca", "katz"},
    {"oveja", "katy"},
    {"cerdo", "kapk"},
    {"gallina", "kapt"},
    {"león", "kapp"},
    {"tigre", "kaps"},
    {"oso", "kapm"},
    {"lobo", "kapn"},
    {"zorro", "kapr"},
    {"conejo", "kapl"},
    {"ratón", "kapv"},
    {"serpiente", "kapz"},
    {"insecto", "kapy"},
    {"abeja", "kask"},
    {"mariposa", "kast"},
    {"araña", "kasp"},
    {"hombre", "kass"},
    {"mujer", "kasm"},
    {"niño", "kasn"},
    {"niña", "kasr"},
    {"bebé", "kasl"},
    {"joven", "kasv"},
    {"adulto", "kasz"},
    {"anciano", "kasy"},
    {"gente", "kamk"},
    {"persona", "kamt"},
    {"amigo", "kamp"},
    {"amiga", "kams"},
    {"enemigo", "kamm"},
    {"vecino", "kamn"},
    {"familia", "kamr"},
    {"padre", "kaml"},
    {"madre", "kamv"},
    {"hijo", "kamz"},
    {"hija", "kamy"},
    {"hermano", "kank"},
    {"hermana", "kant"},
    {"abuelo", "kanp"},
    {"abuela", "kans"},
    {"nieto", "kanm"},
    {"nieta", "kann"},
    {"tío", "kanr"},
    {"tía", "kanl"},
    {"primo", "kanv"},
    {"prima", "kanz"},
    {"esposo", "kany"},
    {"esposa", "kark"},
    {"novio", "kart"},
    {"novia", "karp"},
    {"cuerpo", "kars"},
    {"cabeza", "karm"},
    {"cara", "karn"},
    {"ojo", "karr"},
    {"nariz", "karl"},
    {"boca", "karv"},
    {"labio", "karz"},
    {"diente", "kary"},
    {"lengua", "kalk"},
    {"oreja", "kalt"},
    {"cuello", "kalp"},
    {"hombro", "kals"},
    {"brazo", "kalm"},
    {"mano", "kaln"},
    {"dedo", "kalr"},
    {"pierna", "kall"},
    {"pie", "kalv"},
    {"rodilla", "kalz"},
    {"espalda", "kaly"},
    {"pecho", "kavk"},
    {"corazón", "kavt"},
    {"sangre", "kavp"},
    {"hueso", "kavs"},
    {"piel", "kavm"},
    {"pelo", "kavn"},
    {"cerebro", "kavr"},
    {"mente", "kavl"},
    {"idea", "kavv"},
    {"pensamiento", "kavz"},
    {"sueño", "kavy"},
    {"recuerdo", "kazk"},
    {"memoria", "kazt"},
    {"olvido", "kazp"},
    {"amor", "kazs"},
    {"odio", "kazm"},
    {"miedo", "kazn"},
    {"alegría", "kazr"},
    {"tristeza", "kazl"},
    {"enojo", "kazv"},
    {"ira", "kazz"},
    {"calma", "kazy"},
    {"ansiedad", "kayk"},
    {"esperanza", "kayt"},
    {"fe", "kayp"},
    {"duda", "kays"},
    {"verdad", "kaym"},
    {"mentira", "kayn"},
    {"secreto", "kayr"},
    {"misterio", "kayl"},
    {"historia", "kayv"},
    {"cuento", "kayz"},
    {"leyenda", "kayy"},
    {"mito", "kekk"},
    {"nombre", "kekt"},
    {"palabra", "kekp"},
    {"frase", "keks"},
    {"idioma", "kekm"},
    {"letra", "kekn"},
    {"número", "kekr"},
    {"cuenta", "kekl"},
    {"pregunta", "kekv"},
    {"respuesta", "kekz"},
    {"tema", "keky"},
    {"asunto", "ketk"},
    {"problema", "kett"},
    {"solución", "ketp"},
    {"causa", "kets"},
    {"efecto", "ketm"},
    {"razón", "ketn"},
    {"motivo", "ketr"},
    {"prueba", "ketl"},
    {"ejemplo", "ketv"},
    {"muestra", "ketz"},
    {"señal", "kety"},
    {"signo", "kepk"},
    {"símbolo", "kept"},
    {"marca", "kepp"},
    {"huella", "keps"},
    {"pista", "kepm"},
    {"trabajo", "kepn"},
    {"empleo", "kepr"},
    {"oficio", "kepl"},
    {"tarea", "kepv"},
    {"esfuerzo", "kepz"},
    {"logro", "kepy"},
    {"éxito", "kesk"},
    {"fracaso", "kest"},
    {"premio", "kesp"},
    {"castigo", "kess"},
    {"regla", "kesm"},
    {"norma", "kesn"},
    {"ley", "kesr"},
    {"derecho", "kesl"},
    {"justicia", "kesv"},
    {"libertad", "kesz"},
    {"igualdad", "kesy"},
    {"paz", "kemk"},
    {"guerra", "kemt"},
    {"batalla", "kemp"},
    {"arma", "kems"},
    {"escudo", "kemm"},
    {"soldado", "kemn"},
    {"ejército", "kemr"},
    {"rey", "keml"},
    {"reina", "kemv"},
    {"príncipe", "kemz"},
    {"princesa", "kemy"},
    {"gobierno", "kenk"},
    {"estado", "kent"},
    {"nación", "kenp"},
    {"bandera", "kens"},
    {"frontera", "kenm"},
    {"capital", "kenn"},
    {"dinero", "kenr"},
    {"moneda", "kenl"},
    {"banco", "kenv"},
    {"precio", "kenz"},
    {"costo", "keny"},
    {"valor", "kerk"},
    {"riqueza", "kert"},
    {"pobreza", "kerp"},
    {"deuda", "kers"},
    {"impuesto", "kerm"},
    {"sueldo", "kern"},
    {"regalo", "kerr"},
    {"compra", "kerl"},
    {"venta", "kerv"},
    {"mercado", "kerz"},
    {"tienda", "kery"},
    {"producto", "kelk"},
    {"cliente", "kelt"},
    {"servicio", "kelp"},
    {"empresa", "kels"},
    {"negocio", "kelm"},
    {"jefe", "keln"},
    {"empleado", "kelr"},
    {"compañero", "kell"},
    {"reunión", "kelv"},
    {"cita", "kelz"},
    {"fiesta", "kely"},
    {"juego", "kevk"},
    {"deporte", "kevt"},
    {"música", "kevp"},
    {"canción", "kevs"},
    {"baile", "kevm"},
    {"arte", "kevn"},
    {"pintura", "kevr"},
    {"dibujo", "kevl"},
    {"foto", "kevv"},
    {"película", "kevz"},
    {"libro", "kevy"},
    {"revista", "kezk"},
    {"periódico", "kezt"},
    {"noticia", "kezp"},
    {"carta", "kezs"},
    {"mensaje", "kezm"},
    {"correo", "kezn"},
    {"teléfono", "kezr"},
    {"computadora", "kezl"},
    {"pantalla", "kezv"},
    {"teclado", "kezz"},
    {"internet", "kezy"},
    {"red", "keyk"},
    {"programa", "keyt"},
    {"archivo", "keyp"},
    {"dato", "keys"},
    {"sistema", "keym"},
    {"máquina", "keyn"},
    {"motor", "keyr"},
    {"herramienta", "keyl"},
    {"clavo", "keyv"},
    {"martillo", "keyz"},
    {"cuchillo", "keyy"},
    {"tijera", "kikk"},
    {"aguja", "kikt"},
    {"hilo", "kikp"},
    {"ropa", "kiks"},
    {"camisa", "kikm"},
    {"pantalón", "kikn"},
    {"zapato", "kikr"},
    {"sombrero", "kikl"},
    {"vestido", "kikv"},
    {"comida", "kikz"},
    {"pan", "kiky"},
    {"leche", "kitk"},
    {"queso", "kitt"},
    {"huevo", "kitp"},
    {"carne", "kits"},
    {"pescado", "kitm"},
    {"pollo", "kitn"},
    {"arroz", "kitr"},
    {"frijol", "kitl"},
    {"maíz", "kitv"},
    {"papa", "kitz"},
    {"tomate", "kity"},
    {"cebolla", "kipk"},
    {"ajo", "kipt"},
    {"sal", "kipp"},
    {"azúcar", "kips"},
    {"café", "kipm"},
    {"té", "kipn"},
    {"vino", "kipr"},
    {"cerveza", "kipl"},
    {"jugo", "kipv"},
    {"plato", "kipz"},
    {"vaso", "kipy"},
    {"taza", "kisk"},
    {"cuchara", "kist"},
    {"tenedor", "kisp"},
    {"escuela", "kiss"},
    {"clase", "kism"},
    {"maestro", "kisn"},
    {"alumno", "kisr"},
    {"lección", "kisl"},
    {"examen", "kisv"},
    {"nota", "kisz"},
    {"universidad", "kisy"},
    {"ciencia", "kimk"},
    {"matemática", "kimt"},
    {"física", "kimp"},
    {"química", "kims"},
    {"biología", "kimm"},
    {"geografía", "kimn"},
    {"literatura", "kimr"},
    {"filosofía", "kiml"},
    {"religión", "kimv"},
    {"dios", "kimz"},
    {"iglesia", "kimy"},
    {"templo", "kink"},
    {"oración", "kint"},
    {"infierno", "kinp"},
    {"alma", "kins"},
    {"espíritu", "kinm"},
    {"ángel", "kinn"},
    {"demonio", "kinr"},
    {"salud", "kinl"},
    {"enfermedad", "kinv"},
    {"dolor", "kinz"},
    {"fiebre", "kiny"},
    {"herida", "kirk"},
    {"médico", "kirt"},
    {"hospital", "kirp"},
    {"medicina", "kirs"},
    {"remedio", "kirm"},
    {"cura", "kirn"},
    {"ejercicio", "kirr"},
    {"viaje", "kirl"},
    {"camino", "kirv"},
    {"ruta", "kirz"},
    {"mapa", "kiry"},
    {"destino", "kilk"},
    {"origen", "kilt"},
    {"llave", "kilp"},
    {"candado", "kils"},
    {"espejo", "kilm"},
    {"reloj", "kiln"},
    {"calendario", "kilr"},
    {"estación", "kill"},
    {"primavera", "kilv"},
    {"verano", "kilz"},
    {"otoño", "kily"},
    {"invierno", "kivk"},
    {"clima", "kivt"},
    {"temperatura", "kivp"},
    {"grado", "kivs"},
    {"color", "kivm"},
    {"rojo", "kivn"},
    {"azul", "kivr"},
    {"verde", "kivl"},
    {"amarillo", "kivv"},
    {"negro", "kivz"},
    {"blanco", "kivy"},
    {"gris", "kizk"},
    {"rosa", "kizt"},
    {"morado", "kizp"},
    {"naranja", "kizs"},
    {"marrón", "kizm"},
    {"forma", "kizn"},
    {"tamaño", "kizr"},
    {"peso", "kizl"},
    {"altura", "kizv"},
    {"ancho", "kizz"},
    {"largo", "kizy"},
    {"distancia", "kiyk"},
    {"velocidad", "kiyt"},
    {"dirección", "kiyp"},
    {"norte", "kiys"},
    {"sur", "kiym"},
    {"oeste", "kiyn"},
    {"derecha", "kiyr"},
    {"izquierda", "kiyl"},
    {"arriba", "kiyv"},
    {"abajo", "kiyz"},
    {"dentro", "kiyy"},
    {"fuera", "kokk"},
    {"centro", "kokt"},
    {"borde", "kokp"},
    {"esquina", "koks"},
    {"línea", "kokm"},
    {"punto", "kokn"},
    {"círculo", "kokr"},
    {"cuadrado", "kokl"},
    {"triángulo", "kokv"},
    {"figura", "kokz"},
    {"imagen", "koky"},
    {"reflejo", "kotk"},
    {"sonido", "kott"},
    {"ruido", "kotp"},
    {"silencio", "kots"},
    {"voz", "kotm"},
    {"grito", "kotn"},
    {"susurro", "kotr"},
    {"eco", "kotl"},
    {"olor", "kotv"},
    {"sabor", "kotz"},
    {"tacto", "koty"},
    {"vista", "kopk"},
    {"oído", "kopt"},
    {"gusto", "kopp"},
    {"olfato", "kops"},
    {"sentido", "kopm"},
    {"sensación", "kopn"},
    {"emoción", "kopr"},
    {"sentimiento", "kopl"},
    {"pasión", "kopv"},
    {"deseo", "kopz"},
    {"necesidad", "kopy"},
    {"hambre", "kosk"},
    {"sed", "kost"},
    {"cansancio", "kosp"},
    {"energía", "koss"},
    {"fuerza", "kosm"},
    {"control", "kosn"},
    {"orden", "kosr"},
    {"caos", "kosl"},
    {"cambio", "kosv"},
    {"diferencia", "kosz"},
    {"similitud", "kosy"},
    {"realidad", "komk"},
    {"fantasía", "komt"},
    {"imaginación", "komp"},
    {"creatividad", "koms"},
    {"inteligencia", "komm"},
    {"sabiduría", "komn"},
    {"conocimiento", "komr"},
    {"ignorancia", "koml"},
    {"experiencia", "komv"},
    {"experimento", "komz"},
    {"teoría", "komy"},
    {"práctica", "konk"},
    {"método", "kont"},
    {"plan", "konp"},
    {"proyecto", "kons"},
    {"meta", "konm"},
    {"objetivo", "konn"},
    {"propósito", "konr"},
    {"fin", "konl"},
    {"inicio", "konv"},
    {"comienzo", "konz"},
    {"final", "kony"},
    {"medio", "kork"},
    {"parte", "kort"},
    {"bueno", "korp"},
    {"malo", "kors"},
    {"grande", "korm"},
    {"pequeño", "korn"},
    {"alto", "korr"},
    {"bajo", "korl"},
    {"corto", "korv"},
    {"nuevo", "korz"},
    {"viejo", "kory"},
    {"feliz", "kolk"},
    {"triste", "kolt"},
    {"enojado", "kolp"},
    {"tranquilo", "kols"},
    {"nervioso", "kolm"},
    {"cansado", "koln"},
    {"fuerte", "kolr"},
    {"débil", "koll"},
    {"rápido", "kolv"},
    {"lento", "kolz"},
    {"caliente", "koly"},
    {"frío", "kovk"},
    {"tibio", "kovt"},
    {"fresco", "kovp"},
    {"dulce", "kovs"},
    {"amargo", "kovm"},
    {"salado", "kovn"},
    {"ácido", "kovr"},
    {"suave", "kovl"},
    {"áspero", "kovv"},
    {"duro", "kovz"},
    {"blando", "kovy"},
    {"ligero", "kozk"},
    {"pesado", "kozt"},
    {"claro", "kozp"},
    {"oscuro", "kozs"},
    {"brillante", "kozm"},
    {"opaco", "kozn"},
    {"limpio", "kozr"},
    {"sucio", "kozl"},
    {"seco", "kozv"},
    {"mojado", "kozz"},
    {"lleno", "kozy"},
    {"vacío", "koyk"},
    {"abierto", "koyt"},
    {"cerrado", "koyp"},
    {"antiguo", "koys"},
    {"moderno", "koym"},
    {"rico", "koyn"},
    {"pobre", "koyr"},
    {"caro", "koyl"},
    {"barato", "koyv"},
    {"fácil", "koyz"},
    {"difícil", "koyy"},
    {"simple", "kukk"},
    {"complejo", "kukt"},
    {"posible", "kukp"},
    {"imposible", "kuks"},
    {"cierto", "kukm"},
    {"falso", "kukn"},
    {"verdadero", "kukr"},
    {"real", "kukl"},
    {"irreal", "kukv"},
    {"importante", "kukz"},
    {"trivial", "kuky"},
    {"necesario", "kutk"},
    {"innecesario", "kutt"},
    {"urgente", "kutp"},
    {"común", "kuts"},
    {"raro", "kutm"},
    {"extraño", "kutn"},
    {"normal", "kutr"},
    {"especial", "kutl"},
    {"general", "kutv"},
    {"particular", "kutz"},
    {"público", "kuty"},
    {"privado", "kupk"},
    {"libre", "kupt"},
    {"ocupado", "kupp"},
    {"próximo", "kups"},
    {"lejano", "kupm"},
    {"primero", "kupn"},
    {"último", "kupr"},
    {"siguiente", "kupl"},
    {"anterior", "kupv"},
    {"mejor", "kupz"},
    {"peor", "kupy"},
    {"mayor", "kusk"},
    {"menor", "kust"},
    {"superior", "kusp"},
    {"inferior", "kuss"},
    {"igual", "kusm"},
    {"distinto", "kusn"},
    {"similar", "kusr"},
    {"diferente", "kusl"},
    {"propio", "kusv"},
    {"ajeno", "kusz"},
    {"hermoso", "kusy"},
    {"feo", "kumk"},
    {"bonito", "kumt"},
    {"horrible", "kump"},
    {"increíble", "kums"},
    {"perfecto", "kumm"},
    {"imperfecto", "kumn"},
    {"correcto", "kumr"},
    {"incorrecto", "kuml"},
    {"justo", "kumv"},
    {"injusto", "kumz"},
    {"sabio", "kumy"},
    {"tonto", "kunk"},
    {"listo", "kunt"},
    {"valiente", "kunp"},
    {"cobarde", "kuns"},
    {"amable", "kunm"},
    {"cruel", "kunn"},
    {"generoso", "kunr"},
    {"egoísta", "kunl"},
    {"honesto", "kunv"},
    {"deshonesto", "kunz"},
    {"fiel", "kuny"},
    {"infiel", "kurk"},
    {"puntual", "kurt"},
    {"impuntual", "kurp"},
    {"curioso", "kurs"},
    {"serio", "kurm"},
    {"divertido", "kurn"},
    {"aburrido", "kurr"},
    {"interesante", "kurl"},
    {"gracioso", "kurv"},
    {"misterioso", "kurz"},
    {"evidente", "kury"},
    {"obvio", "kulk"},
    {"sagrado", "kult"},
    {"profano", "kulp"},
    {"divino", "kuls"},
    {"humano", "kulm"},
    {"natural", "kuln"},
    {"artificial", "kulr"},
    {"urbano", "kull"},
    {"rural", "kulv"},
    {"doméstico", "kulz"},
    {"salvaje", "kuly"},
    {"manual", "kuvk"},
    {"automático", "kuvt"},
    {"eléctrico", "kuvp"},
    {"digital", "kuvs"},
    {"sí", "kuvm"},
    {"ya", "kuvn"},
    {"aún", "kuvr"},
    {"todavía", "kuvl"},
    {"siempre", "kuvv"},
    {"nunca", "kuvz"},
    {"jamás", "kuvy"},
    {"pronto", "kuzk"},
    {"temprano", "kuzt"},
    {"ahora", "kuzp"},
    {"hoy", "kuzs"},
    {"ayer", "kuzm"},
    {"anoche", "kuzn"},
    {"luego", "kuzr"},
    {"después", "kuzl"},
    {"antes", "kuzv"},
    {"entonces", "kuzz"},
    {"aquí", "kuzy"},
    {"ahí", "kuyk"},
    {"allí", "kuyt"},
    {"acá", "kuyp"},
    {"allá", "kuys"},
    {"adentro", "kuym"},
    {"afuera", "kuyn"},
    {"adelante", "kuyr"},
    {"atrás", "kuyl"},
    {"alrededor", "kuyv"},
    {"encima", "kuyz"},
    {"debajo", "kuyy"},
    {"delante", "takk"},
    {"detrás", "takt"},
    {"junto", "takp"},
    {"apenas", "taks"},
    {"casi", "takm"},
    {"exactamente", "takn"},
    {"muy", "takr"},
    {"más", "takl"},
    {"menos", "takv"},
    {"tan", "takz"},
    {"bien", "taky"},
    {"mal", "tatk"},
    {"así", "tatt"},
    {"uno", "tatp"},
    {"dos", "tats"},
    {"tres", "tatm"},
    {"cuatro", "tatn"},
    {"cinco", "tatr"},
    {"seis", "tatl"},
    {"siete", "tatv"},
    {"ocho", "tatz"},
    {"nueve", "taty"},
    {"diez", "tapk"},
    {"once", "tapt"},
    {"doce", "tapp"},
    {"trece", "taps"},
    {"catorce", "tapm"},
    {"quince", "tapn"},
    {"dieciséis", "tapr"},
    {"veinte", "tapl"},
    {"treinta", "tapv"},
    {"cuarenta", "tapz"},
    {"cincuenta", "tapy"},
    {"sesenta", "task"},
    {"setenta", "tast"},
    {"ochenta", "tasp"},
    {"noventa", "tass"},
    {"cien", "tasm"},
    {"ciento", "tasn"},
    {"mil", "tasr"},
    {"millón", "tasl"},
    {"tercero", "tasv"},
    {"quinto", "tasz"},
    {"mitad", "tasy"},
    {"doble", "tamk"},
    {"par", "tamt"},
    {"impar", "tamp"},
    {"playa", "tams"},
    {"desierto", "tamm"},
    {"selva", "tamn"},
    {"isla", "tamr"},
    {"cueva", "taml"},
    {"volcán", "tamv"},
    {"cascada", "tamz"},
    {"jardín", "tamy"},
    {"huerto", "tank"},
    {"granja", "tant"},
    {"fábrica", "tanp"},
    {"oficina", "tans"},
    {"museo", "tanm"},
    {"teatro", "tann"},
    {"cine", "tanr"},
    {"estadio", "tanl"},
    {"gimnasio", "tanv"},
    {"restaurante", "tanz"},
    {"hotel", "tany"},
    {"aeropuerto", "tark"},
    {"puerto", "tart"},
    {"biblioteca", "tarp"},
    {"laboratorio", "tars"},
    {"carpeta", "tarm"},
    {"documento", "tarn"},
    {"video", "tarr"},
    {"audio", "tarl"},
    {"altavoz", "tarv"},
    {"micrófono", "tarz"},
    {"cámara", "tary"},
    {"batería", "talk"},
    {"cable", "talt"},
    {"enchufe", "talp"},
    {"interruptor", "tals"},
    {"doctor", "talm"},
    {"ingeniero", "taln"},
    {"artista", "talr"},
    {"músico", "tall"},
    {"escritor", "talv"},
    {"chef", "talz"},
    {"piloto", "taly"},
    {"juez", "tavk"},
    {"policía", "tavt"},
    {"bombero", "tavp"},
    {"carpintero", "tavs"},
    {"plomero", "tavm"},
    {"electricista", "tavn"},
    {"granjero", "tavr"},
    {"pescador", "tavl"},
    {"estudiante", "tavv"},
    {"profesor", "tavz"},
    {"paciente", "tavy"},
    {"turista", "tazk"},
    {"invitado", "tazt"},
    {"anfitrión", "tazp"},
    {"líder", "tazs"},
    {"seguidor", "tazm"},
    {"fan", "tazn"},
    {"héroe", "tazr"},
    {"villano", "tazl"},
    {"gigante", "tazv"},
    {"enano", "tazz"},
    {"fantasma", "tazy"},
    {"monstruo", "tayk"},
    {"dragón", "tayt"},
    {"unicornio", "tayp"},
    {"tesoro", "tays"},
    {"corona", "taym"},
    {"trono", "tayn"},
    {"castillo", "tayr"},
    {"espada", "tayl"},
    {"flecha", "tayv"},
    {"arco", "tayz"},
    {"tambor", "tayy"},
    {"guitarra", "tekk"},
    {"piano", "tekt"},
    {"violín", "tekp"},
    {"flauta", "teks"},
    {"trompeta", "tekm"},
    {"pincel", "tekn"},
    {"lienzo", "tekr"},
    {"escultura", "tekl"},
    {"poema", "tekv"},
    {"novela", "tekz"},
    {"ensayo", "teky"},
    {"discurso", "tetk"},
    {"debate", "tett"},
    {"diálogo", "tetp"},
    {"entrevista", "tets"},
    {"encuesta", "tetm"},
    {"censo", "tetn"},
    {"brújula", "tetr"},
    {"ancla", "tetl"},
    {"vela", "tetv"},
    {"remo", "tetz"},
    {"timón", "tety"},
    {"ala", "tepk"},
    {"pluma", "tept"},
    {"pico", "tepp"},
    {"garra", "teps"},
    {"cola", "tepm"},
    {"cuerno", "tepn"},
    {"escama", "tepr"},
    {"caparazón", "tepl"},
    {"nido", "tepv"},
    {"madriguera", "tepz"},
    {"colmena", "tepy"},
    {"telaraña", "tesk"},
    {"veneno", "test"},
    {"antídoto", "tesp"},
    {"perfume", "tess"},
    {"jabón", "tesm"},
    {"champú", "tesn"},
    {"toalla", "tesr"},
    {"cepillo", "tesl"},
    {"peine", "tesv"},
    {"almohada", "tesz"},
    {"manta", "tesy"},
    {"sábana", "temk"},
    {"cortina", "temt"},
    {"alfombra", "temp"},
    {"lámpara", "tems"},
    {"fósforo", "temm"},
    {"encendedor", "temn"},
    {"olla", "temr"},
    {"sartén", "teml"},
    {"horno", "temv"},
    {"refrigerador", "temz"},
    {"licuadora", "temy"},
    {"jarra", "tenk"},
    {"botella", "tent"},
    {"lata", "tenp"},
    {"caja", "tens"},
    {"bolsa", "tenm"},
    {"paquete", "tenn"},
    {"sello", "tenr"},
    {"boleto", "tenl"},
    {"entrada", "tenv"},
    {"salida", "tenz"},
    {"llegada", "teny"},
    {"partida", "terk"},
    {"demora", "tert"},
    {"prisa", "terp"},
    {"pausa", "ters"},
    {"turno", "term"},
    {"fila", "tern"},
    {"multitud", "terr"},
    {"audiencia", "terl"},
    {"aplauso", "terv"},
    {"silbido", "terz"},
    {"lágrima", "tery"},
    {"sonrisa", "telk"},
    {"abrazo", "telt"},
    {"beso", "telp"},
    {"caricia", "tels"},
    {"empujón", "telm"},
    {"caída", "teln"},
    {"salto", "telr"},
    {"carrera", "tell"},
    {"paseo", "telv"},
    {"excursión", "telz"},
    {"aventura", "tely"},
    {"riesgo", "tevk"},
    {"peligro", "tevt"},
    {"seguridad", "tevp"},
    {"rescate", "tevs"},
    {"ayuda", "tevm"},
    {"favor", "tevn"},
    {"consejo", "tevr"},
    {"sugerencia", "tevl"},
    {"advertencia", "tevv"},
    {"amenaza", "tevz"},
    {"promesa", "tevy"},
    {"juramento", "tezk"},
    {"apuesta", "tezt"},
    {"sorteo", "tezp"},
    {"concurso", "tezs"},
    {"torneo", "tezm"},
    {"campeonato", "tezn"},
    {"medalla", "tezr"},
    {"trofeo", "tezl"},
    {"récord", "tezv"},
    {"empate", "tezz"},
    {"victoria", "tezy"},
    {"derrota", "teyk"},
    {"paciencia", "teyt"},
    {"coraje", "teyp"},
    {"honor", "teys"},
    {"orgullo", "teym"},
    {"humildad", "teyn"},
    {"gratitud", "teyr"},
    {"respeto", "teyl"},
    {"confianza", "teyv"},
    {"lealtad", "teyz"},
    {"traición", "teyy"},
    {"venganza", "tikk"},
    {"perdón", "tikt"},
    {"suerte", "tikp"},
    {"azar", "tiks"},
    {"milagro", "tikm"},
    {"maravilla", "tikn"},
    {"horror", "tikr"},
    {"belleza", "tikl"},
    {"fealdad", "tikv"},
    {"juventud", "tikz"},
    {"vejez", "tiky"},
    {"infancia", "titk"},
    {"adolescencia", "titt"},
    {"madurez", "titp"},
    {"nacimiento", "tits"},
    {"boda", "titm"},
    {"divorcio", "titn"},
    {"funeral", "titr"},
    {"cumpleaños", "titl"},
    {"navidad", "titv"},
    {"robot", "titz"},
    {"algoritmo", "tity"},
    {"código", "tipk"},
    {"aplicación", "tipt"},
    {"sitio", "tipp"},
    {"web", "tips"},
    {"relámpago", "tipm"},
    {"arcoíris", "tipn"},
    {"niebla", "tipr"},
    {"rocío", "tipl"},
    {"escarcha", "tipv"},
    {"granizo", "tipz"},
    {"ola", "tipy"},
    {"marea", "tisk"},
    {"corriente", "tist"},
    {"remolino", "tisp"},
    {"tornado", "tiss"},
    {"huracán", "tism"},
    {"terremoto", "tisn"},
    {"erupción", "tisr"},
    {"géiser", "tisl"},
    {"glaciar", "tisv"},
    {"iceberg", "tisz"},
    {"polo", "tisy"},
    {"ecuador", "timk"},
    {"trópico", "timt"},
    {"meridiano", "timp"},
    {"latitud", "tims"},
    {"razonar", "timm"},
    {"inferir", "timn"},
    {"deducir", "timr"},
    {"inducir", "timl"},
    {"presumir", "timv"},
    {"dudar", "timz"},
    {"repensar", "timy"},
    {"reflexionar", "tink"},
    {"meditar", "tint"},
    {"contemplar", "tinp"},
    {"rememorar", "tins"},
    {"evocar", "tinm"},
    {"memorizar", "tinn"},
    {"fantasear", "tinr"},
    {"idear", "tinl"},
    {"concebir", "tinv"},
    {"planear", "tinz"},
    {"planificar", "tiny"},
    {"evaluar", "tirk"},
    {"analizar", "tirt"},
    {"sintetizar", "tirp"},
    {"comparar", "tirs"},
    {"contrastar", "tirm"},
    {"clasificar", "tirn"},
    {"definir", "tirr"},
    {"aclarar", "tirl"},
    {"argumentar", "tirv"},
    {"debatir", "tirz"},
    {"dialogar", "tiry"},
    {"conversar", "tilk"},
    {"interrogar", "tilt"},
    {"replicar", "tilp"},
    {"afirmar", "tils"},
    {"confirmar", "tilm"},
    {"refutar", "tiln"},
    {"admitir", "tilr"},
    {"confesar", "till"},
    {"jurar", "tilv"},
    {"persuadir", "tilz"},
    {"convencer", "tily"},
    {"aconsejar", "tivk"},
    {"advertir", "tivt"},
    {"instruir", "tivp"},
    {"educar", "tivs"},
    {"ensayar", "tivm"},
    {"fallar", "tivn"},
    {"errar", "tivr"},
    {"corregir", "tivl"},
    {"enmendar", "tivv"},
    {"empeorar", "tivz"},
    {"evolucionar", "tivy"},
    {"desarrollar", "tizk"},
    {"concentrar", "tizt"},
    {"distraer", "tizp"},
    {"atender", "tizs"},
    {"ilusión", "tizm"},
    {"atención", "tizn"},
    {"conciencia", "tizr"},
    {"inconsciencia", "tizl"},
    {"desesperación", "tizv"},
    {"curiosidad", "tizz"},
    {"sorpresa", "tizy"},
    {"aburrimiento", "tiyk"},
    {"vergüenza", "tiyt"},
    {"culpa", "tiyp"},
    {"envidia", "tiys"},
    {"celos", "tiym"},
    {"compasión", "tiyn"},
    {"empatía", "tiyr"},
    {"simpatía", "tiyl"},
    {"impaciencia", "tiyv"},
    {"valentía", "tiyz"},
    {"cobardía", "tiyy"},
    {"hipótesis", "tokk"},
    {"tesis", "tokt"},
    {"evidencia", "tokp"},
    {"relato", "toks"},
    {"narración", "tokm"},
    {"capítulo", "tokn"},
    {"verso", "tokr"},
    {"estrofa", "tokl"},
    {"término", "tokv"},
    {"vocabulario", "tokz"},
    {"gramática", "toky"},
    {"sintaxis", "totk"},
    {"semántica", "tott"},
    {"significado", "totp"},
    {"sinsentido", "tots"},
    {"metáfora", "totm"},
    {"analogía", "totn"},
    {"paradoja", "totr"},
    {"enigma", "totl"},
    {"revelación", "totv"},
    {"descubrimiento", "totz"},
    {"exploración", "toty"},
    {"excepción", "topk"},
    {"principio", "topt"},
    {"conjunto", "topp"},
    {"equipo", "tops"},
    {"consenso", "topm"},
    {"voto", "topn"},
    {"decisión", "topr"},
    {"opinión", "topl"},
    {"perspectiva", "topv"},
    {"enfoque", "topz"},
    {"aspecto", "topy"},
    {"detalle", "tosk"},
    {"resumen", "tost"},
    {"conclusión", "tosp"},
    {"introducción", "toss"},
    {"certeza", "tosm"},
    {"probabilidad", "tosn"},
    {"posibilidad", "tosr"},
    {"imposibilidad", "tosl"},
    {"obligación", "tosv"},
    {"permiso", "tosz"},
    {"prohibición", "tosy"},
    {"gesto", "tomk"},
    {"resonancia", "tomt"},
    {"vibración", "tomp"},
    {"onda", "toms"},
    {"pulso", "tomm"},
    {"ritmo", "tomn"},
    {"armonía", "tomr"},
    {"disonancia", "toml"},
    {"melodía", "tomv"},
    {"murmullo", "tomz"},
    {"estímulo", "tomy"},
    {"percepción", "tonk"},
    {"observador", "tont"},
    {"testigo", "tonp"},
    {"fondo", "tons"},
    {"límite", "tonm"},
    {"horizonte", "tonn"},
    {"cercanía", "tonr"},
    {"interior", "tonl"},
    {"exterior", "tonv"},
    {"superficie", "tonz"},
    {"profundidad", "tony"},
    {"anchura", "tork"},
    {"textura", "tort"},
    {"error", "torp"},
    {"acierto", "tors"},
    {"intento", "torm"},
    {"subconsciente", "torn"},
    {"vigilia", "torr"},
    {"introspección", "torl"},
    {"deducción", "torv"},
    {"inducción", "torz"},
    {"premisa", "tory"},
    {"silogismo", "tolk"},
    {"falacia", "tolt"},
    {"axioma", "tolp"},
    {"teorema", "tols"},
    {"corolario", "tolm"},
    {"lema", "toln"},
    {"aporía", "tolr"},
    {"dilema", "toll"},
    {"trilema", "tolv"},
    {"sinergia", "tolz"},
    {"emergencia", "toly"},
    {"atractor", "tovk"},
    {"cuenca", "tovt"},
    {"bifurcación", "tovp"},
    {"umbral", "tovs"},
    {"oscilar", "tovm"},
    {"pulsar", "tovn"},
    {"latir", "tovr"},
    {"fluir", "tovl"},
    {"emerger", "tovv"},
    {"converger", "tovz"},
    {"divergir", "tovy"},
    {"interferir", "tozk"},
    {"amplificar", "tozt"},
    {"atenuar", "tozp"},
    {"modular", "tozs"},
    {"filtrar", "tozm"},
    {"sintonizar", "tozn"},
    {"calibrar", "tozr"},
    {"deliberar", "tozl"},
    {"consensuar", "tozv"},
    {"disentir", "tozz"},
    {"secundar", "tozy"},
    {"matizar", "toyk"},
    {"puntualizar", "toyt"},
    {"recapitular", "toyp"},
    {"cuestionar", "toys"},
    {"indagar", "toym"},
    {"inspeccionar", "toyn"},
    {"escrutar", "toyr"},
    {"contestar", "toyl"},
    {"argüir", "toyv"},
    {"alegar", "toyz"},
    {"postular", "toyy"},
    {"conjeturar", "tukk"},
    {"demostrar", "tukt"},
    {"ilustrar", "tukp"},
    {"ejemplificar", "tuks"},
    {"asimilar", "tukm"},
    {"interiorizar", "tukn"},
    {"asociar", "tukr"},
    {"disociar", "tukl"},
    {"visualizar", "tukv"},
    {"presagiar", "tukz"},
    {"anticipar", "tuky"},
    {"prever", "tutk"},
    {"vacilar", "tutt"},
    {"titubear", "tutp"},
    {"osar", "tuts"},
    {"atreverse", "tutm"},
    {"arriesgar", "tutn"},
    {"anhelar", "tutr"},
    {"ansiar", "tutl"},
    {"confiar", "tutv"},
    {"desconfiar", "tutz"},
    {"captar", "tuty"},
    {"desconocer", "tupk"},
    {"confundir", "tupt"},
    {"acertar", "tupp"},
    {"triunfar", "tups"},
    {"iniciar", "tupm"},
    {"continuar", "tupn"},
    {"proseguir", "tupr"},
    {"detener", "tupl"},
    {"mutar", "tupv"},
    {"devenir", "tupz"},
    {"menguar", "tupy"},
    {"aumentar", "tusk"},
    {"disminuir", "tust"},
    {"dividir", "tusp"},
    {"deshacer", "tuss"},
    {"construir", "tusm"},
    {"destruir", "tusn"},
    {"callar", "tusr"},
    {"despertar", "tusl"},
    {"nervio", "tusv"},
    {"universo", "tusz"},
    {"cosmos", "tusy"},
    {"espacio", "tumk"},
    {"materia", "tumt"},
    {"contorno", "tump"},
    {"veces", "tums"},
    {"cerca", "tumm"},
    {"lejos", "tumn"},
    {"quizás", "tumr"},
    {"acaso", "tuml"},
    {"liso", "tumv"},
    {"rugoso", "tumz"},
    {"bello", "tumy"},
    {"probable", "tunk"},
    {"improbable", "tunt"},
    {"útil", "tunp"},
    {"inútil", "tuns"},
    {"preso", "tunm"},
    {"usado", "tunn"},
    {"roto", "tunr"},
    {"entero", "tunl"},
    {"único", "tunv"},
    {"múltiple", "tunz"},
    {"solo", "tuny"},
    {"acompañado", "turk"},
    {"separado", "turt"},
    {"presente", "turp"},
    {"ausente", "turs"},
    {"visible", "turm"},
    {"invisible", "turn"},
    {"despierto", "turr"},
    {"dormido", "turl"},
    {"vivo", "turv"},
    {"muerto", "turz"},
    {"contento", "tury"},
    {"alegre", "tulk"},
    {"calmado", "tult"},
    {"agitado", "tulp"},
    {"seguro", "tuls"},
    {"inseguro", "tulm"},
    {"incierto", "tuln"},
    {"confuso", "tulr"},
    {"oculto", "tull"},
    {"directo", "tulv"},
    {"indirecto", "tulz"},
    {"recto", "tuly"},
    {"curvo", "tuvk"},
    {"cuándo", "tuvt"},
    {"porqué", "tuvp"},
    {"cuántos", "tuvs"},
    {"cuáles", "tuvm"},
    {"cero", "tuvn"},
    {"diecisiete", "tuvr"},
    {"dieciocho", "tuvl"},
    {"diecinueve", "tuvv"},
    {"veintiuno", "tuvz"},
    {"veintidós", "tuvy"},
    {"veintitrés", "tuzk"},
    {"veinticuatro", "tuzt"},
    {"veinticinco", "tuzp"},
    {"veintiséis", "tuzs"},
    {"veintisiete", "tuzm"},
    {"veintiocho", "tuzn"},
    {"veintinueve", "tuzr"},
    {"doscientos", "tuzl"},
    {"trescientos", "tuzv"},
    {"cuatrocientos", "tuzz"},
    {"quinientos", "tuzy"},
    {"billón", "tuyk"},
    {"sexto", "tuyt"},
    {"séptimo", "tuyp"},
    {"octavo", "tuys"},
    {"noveno", "tuym"},
    {"décimo", "tuyn"},
    {"undécimo", "tuyr"},
    {"duodécimo", "tuyl"},
    {"penúltimo", "tuyv"},
    {"tercio", "tuyz"},
    {"triple", "tuyy"},
    {"cuádruple", "pakk"},
    {"multiplicar", "pakt"},
    {"suma", "pakp"},
    {"resta", "paks"},
    {"multiplicación", "pakm"},
    {"división", "pakn"},
    {"cociente", "pakr"},
    {"elevado", "pakl"},
    {"potencia", "pakv"},
    {"exponente", "pakz"},
    {"base", "paky"},
    {"cuadrada", "patk"},
    {"cúbica", "patt"},
    {"módulo", "patp"},
    {"resto", "pats"},
    {"factorial", "patm"},
    {"porcentaje", "patn"},
    {"dígito", "patr"},
    {"cifra", "patl"},
    {"cantidad", "patv"},
    {"variable", "patz"},
    {"constante", "paty"},
    {"incógnita", "papk"},
    {"ecuación", "papt"},
    {"fórmula", "papp"},
    {"función", "paps"},
    {"demostración", "papm"},
    {"resultado", "papn"},
    {"cálculo", "papr"},
    {"aritmética", "papl"},
    {"álgebra", "papv"},
    {"geometría", "papz"},
    {"trigonometría", "papy"},
    {"estadística", "pask"},
    {"infinito", "past"},
    {"finito", "pasp"},
    {"positivo", "pass"},
    {"negativo", "pasm"},
    {"decimal", "pasn"},
    {"fracción", "pasr"},
    {"numerador", "pasl"},
    {"denominador", "pasv"},
    {"compuesto", "pasz"},
    {"múltiplo", "pasy"},
    {"divisor", "pamk"},
    {"máximo", "pamt"},
    {"mínimo", "pamp"},
    {"promedio", "pams"},
    {"media", "pamm"},
    {"mediana", "pamn"},
    {"moda", "pamr"},
    {"rango", "paml"},
    {"subconjunto", "pamv"},
    {"unión", "pamz"},
    {"intersección", "pamy"},
    {"aproximado", "pank"},
    {"aproximadamente", "pant"},
    {"recta", "panp"},
    {"curva", "pans"},
    {"ángulo", "panm"},
    {"rectángulo", "pann"},
    {"circunferencia", "panr"},
    {"radio", "panl"},
    {"diámetro", "panv"},
    {"área", "panz"},
    {"perímetro", "pany"},
    {"volumen", "park"},
    {"esfera", "part"},
    {"cubo", "parp"},
    {"cilindro", "pars"},
    {"cono", "parm"},
    {"pirámide", "parn"},
    {"lado", "parr"},
    {"vértice", "parl"},
    {"arista", "parv"},
    {"dimensión", "parz"},
    {"plano", "pary"},
    {"medida", "palk"},
    {"masa", "palt"},
    {"unidad", "palp"},
    {"metro", "pals"},
    {"kilómetro", "palm"},
    {"kilo", "paln"},
    {"litro", "palr"},
    {"implica", "pall"},
    {"equivale", "palv"},
    {"proposición", "palz"},
    {"cómputo", "paly"},
    {"aproximación", "pavk"},
    {"estimación", "pavt"},
    {"redondeo", "pavp"},
    {"exacto", "pavs"},
    {"inexacto", "pavm"},
    {"serie", "pavn"},
    {"secuencia", "pavr"},
    {"patrón", "pavl"},
    {"sucesión", "pavv"},
    {"progresión", "pavz"},
    {"derivada", "pavy"},
    {"integral", "pazk"},
    {"matriz", "pazt"},
    {"vector", "pazp"},
    {"escalar", "pazs"},
    {"tensor", "pazm"},
    {"coordenada", "pazn"},
    {"eje", "pazr"},
    {"paralelo", "pazl"},
    {"perpendicular", "pazv"},
    {"tangente", "pazz"},
    {"secante", "pazy"},
    {"simetría", "payk"},
    {"simétrico", "payt"},
    {"asimetría", "payp"},
    {"proporción", "pays"},
    {"áurea", "paym"},
    {"escala", "payn"},
    {"binario", "payr"},
    {"hexadecimal", "payl"},
    {"octal", "payv"},
    {"bit", "payz"},
    {"byte", "payy"},
    {"sumando", "pekk"},
    {"minuendo", "pekt"},
    {"sustraendo", "pekp"},
    {"multiplicando", "peks"},
    {"multiplicador", "pekm"},
    {"dividendo", "pekn"},
    {"quebrado", "pekr"},
    {"mixto", "pekl"},
    {"irreducible", "pekv"},
    {"numeración", "pekz"},
    {"conteo", "peky"},
    {"enumerar", "petk"},
    {"absoluto", "pett"},
    {"numerable", "petp"},
    {"conjetura", "pets"},
    {"suficiente", "petm"},
    {"condición", "petn"},
    {"contradicción", "petr"},
    {"absurdo", "petl"},
    {"reducción", "petv"},
    {"abducción", "petz"},
    {"cuantificador", "pety"},
    {"universal", "pepk"},
    {"existencial", "pept"},
    {"pertenece", "pepp"},
    {"contiene", "peps"},
    {"incluido", "pepm"},
    {"disjunto", "pepn"},
    {"complemento", "pepr"},
    {"cartesiano", "pepl"},
    {"ordenado", "pepv"},
    {"inyectiva", "pepz"},
    {"sobreyectiva", "pepy"},
    {"biyectiva", "pesk"},
    {"monótona", "pest"},
    {"creciente", "pesp"},
    {"decreciente", "pess"},
    {"continua", "pesm"},
    {"discreta", "pesn"},
    {"lineal", "pesr"},
    {"cuadrática", "pesl"},
    {"exponencial", "pesv"},
    {"logarítmica", "pesz"},
    {"seno", "pesy"},
    {"coseno", "pemk"},
    {"pi", "pemt"},
    {"euler", "pemp"},
    {"radián", "pems"},
    {"isósceles", "pemm"},
    {"equilátero", "pemn"},
    {"escaleno", "pemr"},
    {"obtusángulo", "peml"},
    {"acutángulo", "pemv"},
    {"paralelogramo", "pemz"},
    {"trapecio", "pemy"},
    {"rombo", "penk"},
    {"pentágono", "pent"},
    {"hexágono", "penp"},
    {"polígono", "pens"},
    {"prisma", "penm"},
    {"tetraedro", "penn"},
    {"octaedro", "penr"},
    {"dodecaedro", "penl"},
    {"icosaedro", "penv"},
    {"poliedro", "penz"},
    {"hipotenusa", "peny"},
    {"cateto", "perk"},
    {"gravedad", "pert"},
    {"frecuencia", "perp"},
    {"amplitud", "pers"},
    {"fase", "perm"},
    {"período", "pern"},
    {"geométrica", "perr"},
    {"desviación", "perl"},
    {"varianza", "perv"},
    {"percentil", "perz"},
    {"cuartil", "pery"},
    {"población", "pelk"},
    {"aleatorio", "pelt"},
    {"determinista", "pelp"},
    {"estocástico", "pels"},
    {"equiprobable", "pelm"},
    {"independiente", "peln"},
    {"dependiente", "pelr"},
    {"condicional", "pell"},
    {"conjunta", "pelv"},
    {"marginal", "pelz"},
    {"covarianza", "pely"},
    {"correlación", "pevk"},
    {"distribución", "pevt"},
    {"binomial", "pevp"},
    {"poisson", "pevs"},
    {"uniforme", "pevm"},
    {"convergencia", "pevn"},
    {"divergencia", "pevr"},
    {"combinatoria", "pevl"},
    {"permutación", "pevv"},
    {"combinación", "pevz"},
    {"variación", "pevy"},
    {"coeficiente", "pezk"},
    {"polinomio", "pezt"},
    {"monomio", "pezp"},
    {"binomio", "pezs"},
    {"trinomio", "pezm"},
    {"discriminante", "pezn"},
    {"ecuaciones", "pezr"},
    {"columna", "pezl"},
    {"diagonal", "pezv"},
    {"determinante", "pezz"},
    {"inversa", "pezy"},
    {"transpuesta", "peyk"},
    {"magnitud", "peyt"},
    {"cruz", "peyp"},
    {"coordenadas", "peys"},
    {"cartesianas", "peym"},
    {"polares", "peyn"},
    {"cilíndricas", "peyr"},
    {"esféricas", "peyl"},
    {"dominio", "peyv"},
    {"codominio", "peyz"},
    {"preimagen", "peyy"},
    {"epiyectiva", "pikk"},
    {"composición", "pikt"},
    {"periódica", "pikp"},
    {"lateral", "piks"},
    {"asíntota", "pikm"},
    {"continuidad", "pikn"},
    {"discontinuidad", "pikr"},
    {"pendiente", "pikl"},
    {"antiderivada", "pikv"},
    {"fundamental", "pikz"},
    {"taylor", "piky"},
    {"fourier", "pitk"},
    {"diferencial", "pitt"},
    {"ordinaria", "pitp"},
    {"parcial", "pits"},
    {"inicial", "pitm"},
    {"racional", "pitn"},
    {"irracional", "pitr"},
    {"imaginario", "pitl"},
    {"imaginaria", "pitv"},
    {"conjugado", "pitz"},
    {"argumento", "pity"},
    {"binaria", "pipk"},
    {"significativo", "pipt"},
    {"significativa", "pipp"},
    {"notación", "pips"},
    {"científica", "pipm"},
    {"truncamiento", "pipn"},
    {"relativo", "pipr"},
    {"tolerancia", "pipl"},
    {"iteración", "pipv"},
    {"bisección", "pipz"},
    {"newton", "pipy"},
    {"raphson", "pisk"},
    {"interpolación", "pist"},
    {"extrapolación", "pisp"},
    {"ajuste", "piss"},
    {"regresión", "pism"},
    {"mínimos", "pisn"},
    {"cuadrados", "pisr"},
    {"criptografía", "pisl"},
    {"mcd", "pisv"},
    {"mcm", "pisz"},
    {"divisibilidad", "pisy"},
    {"congruencia", "pimk"},
    {"aritmético", "pimt"},
    {"fermat", "pimp"},
    {"euclides", "pims"},
    {"abundante", "pimm"},
    {"deficiente", "pimn"},
    {"capicúa", "pimr"},
    {"palíndromo", "piml"},
    {"numérico", "pimv"},
    {"mágico", "pimz"},
    {"pascal", "pimy"},
    {"fibonacci", "pink"},
    {"áureo", "pint"},
    {"total", "pinp"},
    {"dividido", "pins"},
    {"medición", "pinm"},
    {"profundo", "pinn"},
    {"mediano", "pinr"},
    {"insuficiente", "pinl"},
    {"completo", "pinv"},
    {"incompleto", "pinz"},
    {"preciso", "piny"},
    {"impreciso", "pirk"},
    {"duplicar", "pirt"},
    {"triplicar", "pirp"},
    {"ampliar", "pirs"},
    {"expandir", "pirm"},
    {"contraer", "pirn"},
    {"distribuir", "pirr"},
    {"agrupar", "pirl"},
    {"calcular", "pirv"},
    {"computar", "pirz"},
    {"estimar", "piry"},
    {"cronometrar", "pilk"},
    {"comprobar", "pilt"},
    {"contradecir", "pilp"},
    {"asumir", "pils"},
    {"denotar", "pilm"},
    {"sea", "piln"},
    {"existe", "pilr"},
    {"x", "pill"},
    {"n", "pilv"},
    {"qed", "pilz"},
    {"abstraer", "pily"},
    {"generalizar", "pivk"},
    {"particularizar", "pivt"},
    {"conceptualizar", "pivp"},
    {"categorizar", "pivs"},
    {"sistematizar", "pivm"},
    {"formalizar", "pivn"},
    {"modelar", "pivr"},
    {"emular", "pivl"},
    {"presuponer", "pivv"},
    {"hipotetizar", "pivz"},
    {"teorizar", "pivy"},
    {"especular", "pizk"},
    {"elucubrar", "pizt"},
    {"cavilar", "pizp"},
    {"rumiar", "pizs"},
    {"divagar", "pizm"},
    {"ponderar", "pizn"},
    {"sopesar", "pizr"},
    {"aquilatar", "pizl"},
    {"colegir", "pizv"},
    {"discernir", "pizz"},
    {"dilucidar", "pizy"},
    {"esclarecer", "piyk"},
    {"desentrañar", "piyt"},
    {"descifrar", "piyp"},
    {"decodificar", "piys"},
    {"reinterpretar", "piym"},
    {"aprehender", "piyn"},
    {"integrar", "piyr"},
    {"desintegrar", "piyl"},
    {"descomponer", "piyv"},
    {"recomponer", "piyz"},
    {"estructurar", "piyy"},
    {"desestructurar", "pokk"},
    {"reorganizar", "pokt"},
    {"jerarquizar", "pokp"},
    {"priorizar", "poks"},
    {"secuenciar", "pokm"},
    {"esquematizar", "pokn"},
    {"bosquejar", "pokr"},
    {"delinear", "pokl"},
    {"perfilar", "pokv"},
    {"caracterizar", "pokz"},
    {"tipificar", "poky"},
    {"prototipar", "potk"},
    {"documentar", "pott"},
    {"registrar", "potp"},
    {"archivar", "pots"},
    {"catalogar", "potm"},
    {"inventariar", "potn"},
    {"recopilar", "potr"},
    {"compilar", "potl"},
    {"parafrasear", "potv"},
    {"condensar", "potz"},
    {"compendiar", "poty"},
    {"glosar", "popk"},
    {"anotar", "popt"},
    {"apostillar", "popp"},
    {"subrayar", "pops"},
    {"enfatizar", "popm"},
    {"recalcar", "popn"},
    {"remarcar", "popr"},
    {"acentuar", "popl"},
    {"especificar", "popv"},
    {"detallar", "popz"},
    {"pormenorizar", "popy"},
    {"listar", "posk"},
    {"formular", "post"},
    {"plantear", "posp"},
    {"enunciar", "poss"},
    {"articular", "posm"},
    {"verbalizar", "posn"},
    {"exteriorizar", "posr"},
    {"manifestar", "posl"},
    {"declarar", "posv"},
    {"proclamar", "posz"},
    {"anunciar", "posy"},
    {"notificar", "pomk"},
    {"comunicar", "pomt"},
    {"difundir", "pomp"},
    {"divulgar", "poms"},
    {"propagar", "pomm"},
    {"develar", "pomn"},
    {"disertar", "pomr"},
    {"perorata", "poml"},
    {"aludir", "pomv"},
    {"referir", "pomz"},
    {"denominar", "pomy"},
    {"titular", "ponk"},
    {"vociferar", "pont"},
    {"sondear", "ponp"},
    {"tantear", "pons"},
    {"rastrear", "ponm"},
    {"acechar", "ponn"},
    {"atisbar", "ponr"},
    {"vislumbrar", "ponl"},
    {"entrever", "ponv"},
    {"columbrar", "ponz"},
    {"ojear", "pony"},
    {"fisgar", "pork"},
    {"curiosear", "port"},
    {"avistar", "porp"},
    {"divisar", "pors"},
    {"escudriñar", "porm"},
    {"palpar", "porn"},
    {"acariciar", "porr"},
    {"olfatear", "porl"},
    {"husmear", "porv"},
    {"degustar", "porz"},
    {"catar", "pory"},
    {"paladear", "polk"},
    {"aguzar", "polt"},
    {"enfocar", "polp"},
    {"desenfocar", "pols"},
    {"entumecer", "polm"},
    {"adormecer", "poln"},
    {"despabilar", "polr"},
    {"espabilar", "poll"},
    {"avivar", "polv"},
    {"reanimar", "polz"},
    {"estremecer", "poly"},
    {"erizar", "povk"},
    {"tiritar", "povt"},
    {"escalofriar", "povp"},
    {"jadear", "povs"},
    {"roncar", "povm"},
    {"bostezar", "povn"},
    {"estornudar", "povr"},
    {"carraspear", "povl"},
    {"cosquillear", "povv"},
    {"hormiguear", "povz"},
    {"picar", "povy"},
    {"escocer", "pozk"},
    {"arder", "pozt"},
    {"punzar", "pozp"},
    {"palpitar", "pozs"},
    {"hipar", "pozm"},
    {"eructar", "pozn"},
    {"regurgitar", "pozr"},
    {"transpirar", "pozl"},
    {"exhalar", "pozv"},
    {"inhalar", "pozz"},
    {"aspirar", "pozy"},
    {"soplar", "poyk"},
    {"resoplar", "poyt"},
    {"ventilar", "poyp"},
    {"orear", "poys"},
    {"enfriar", "poym"},
    {"calentar", "poyn"},
    {"templar", "poyr"},
    {"evaporar", "poyl"},
    {"destilar", "poyv"},
    {"fermentar", "poyz"},
    {"derretir", "poyy"},
    {"fundir", "pukk"},
    {"disolver", "pukt"},
    {"diluir", "pukp"},
    {"combinar", "puks"},
    {"fusionar", "pukm"},
    {"amalgamar", "pukn"},
    {"adherir", "pukr"},
    {"barnizar", "pukl"},
    {"teñir", "pukv"},
    {"desteñir", "pukz"},
    {"blanquear", "puky"},
    {"ennegrecer", "putk"},
    {"oscurecer", "putt"},
    {"transparentar", "putp"},
    {"opacar", "puts"},
    {"empañar", "putm"},
    {"desempañar", "putn"},
    {"asear", "putr"},
    {"higienizar", "putl"},
    {"desinfectar", "putv"},
    {"purificar", "putz"},
    {"enjugar", "puty"},
    {"airear", "pupk"},
    {"oxigenar", "pupt"},
    {"citar", "pupp"},
    {"desvelar", "pups"},
    {"recitar", "pupm"},
    {"declamar", "pupn"},
    {"exponer", "pupr"},
    {"redactar", "pupl"},
    {"transcribir", "pupv"},
    {"suscribir", "pupz"},
    {"rubricar", "pupy"},
    {"certificar", "pusk"},
    {"avalar", "pust"},
    {"respaldar", "pusp"},
    {"refrendar", "puss"},
    {"ratificar", "pusm"},
    {"corroborar", "pusn"},
    {"constatar", "pusr"},
    {"auditar", "pusl"},
    {"fiscalizar", "pusv"},
    {"supervisar", "pusz"},
    {"apercibir", "pusy"},
    {"prevenir", "pumk"},
    {"precaver", "pumt"},
    {"alertar", "pump"},
    {"alarmar", "pums"},
    {"tranquilizar", "pumm"},
    {"apaciguar", "pumn"},
    {"sosegar", "pumr"},
    {"serenar", "puml"},
    {"consolar", "pumv"},
    {"confortar", "pumz"},
    {"animar", "pumy"},
    {"alentar", "punk"},
    {"estimular", "punt"},
    {"motivar", "punp"},
    {"inspirar", "puns"},
    {"disuadir", "punm"},
    {"reconvencer", "punn"},
    {"insinuar", "punr"},
    {"brindar", "punl"},
    {"otorgar", "punv"},
    {"conceder", "punz"},
    {"conferir", "puny"},
    {"adjudicar", "purk"},
    {"asignar", "purt"},
    {"destinar", "purp"},
    {"dedicar", "purs"},
    {"consagrar", "purm"},
    {"devolver", "purn"},
    {"restituir", "purr"},
    {"reintegrar", "purl"},
    {"reponer", "purv"},
    {"suplir", "purz"},
    {"relevar", "pury"},
    {"anteceder", "pulk"},
    {"acompañar", "pult"},
    {"guiar", "pulp"},
    {"orientar", "puls"},
    {"desorientar", "pulm"},
    {"localizar", "puln"},
    {"situar", "pulr"},
    {"topar", "pull"},
    {"impactar", "pulv"},
    {"azotar", "pulz"},
    {"fustigar", "puly"},
    {"vapulear", "puvk"},
    {"zurrar", "puvt"},
    {"apedrear", "puvp"},
    {"fusilar", "puvs"},
    {"ejecutar", "puvm"},
    {"ajusticiar", "puvn"},
    {"condenar", "puvr"},
    {"absolver", "puvl"},
    {"exculpar", "puvv"},
    {"inculpar", "puvz"},
    {"acusar", "puvy"},
    {"denunciar", "puzk"},
    {"delatar", "puzt"},
    {"desmentir", "puzp"},
    {"rebatir", "puzs"},
    {"impugnar", "puzm"},
    {"recusar", "puzn"},
    {"apelar", "puzr"},
    {"anular", "puzl"},
    {"revocar", "puzv"},
    {"derogar", "puzz"},
    {"abolir", "puzy"},
    {"suprimir", "puyk"},
    {"eliminar", "puyt"},
    {"erradicar", "puyp"},
    {"extirpar", "puys"},
    {"exterminar", "puym"},
    {"aniquilar", "puyn"},
    {"demoler", "puyr"},
    {"arrasar", "puyl"},
    {"asolar", "puyv"},
    {"devastar", "puyz"},
    {"talar", "puyy"},
    {"repoblar", "sakk"},
    {"cultivar", "sakt"},
    {"labrar", "sakp"},
    {"arar", "saks"},
    {"podar", "sakm"},
    {"injertar", "sakn"},
    {"trasplantar", "sakr"},
    {"abonar", "sakl"},
    {"fertilizar", "sakv"},
    {"cosechar", "sakz"},
    {"recolectar", "saky"},
    {"vendimiar", "satk"},
    {"trillar", "satt"},
    {"moler", "satp"},
    {"hornear", "sats"},
    {"cocer", "satm"},
    {"freír", "satn"},
    {"asar", "satr"},
    {"tostar", "satl"},
    {"ahumar", "satv"},
    {"curar", "satz"},
    {"adobar", "saty"},
    {"marinar", "sapk"},
    {"aliñar", "sapt"},
    {"sazonar", "sapp"},
    {"condimentar", "saps"},
    {"endulzar", "sapm"},
    {"salpimentar", "sapn"},
    {"encantar", "sapr"},
    {"fascinar", "sapl"},
    {"cautivar", "sapv"},
    {"deleitar", "sapz"},
    {"regocijar", "sapy"},
    {"alborozar", "sask"},
    {"jubilar", "sast"},
    {"apenar", "sasp"},
    {"acongojar", "sass"},
    {"consternar", "sasm"},
    {"deprimir", "sasn"},
    {"abatir", "sasr"},
    {"desanimar", "sasl"},
    {"desalentar", "sasv"},
    {"desmoralizar", "sasz"},
    {"espantar", "sasy"},
    {"aterrorizar", "samk"},
    {"horrorizar", "samt"},
    {"sobrecoger", "samp"},
    {"intimidar", "sams"},
    {"acobardar", "samm"},
    {"encolerizar", "samn"},
    {"enfurecer", "samr"},
    {"irritar", "saml"},
    {"exasperar", "samv"},
    {"crispar", "samz"},
    {"aliviar", "samy"},
    {"ilusionar", "sank"},
    {"esperanzar", "sant"},
    {"angustiar", "sanp"},
    {"agobiar", "sans"},
    {"abrumar", "sanm"},
    {"afligir", "sann"},
    {"aquejar", "sanr"},
    {"atormentar", "sanl"},
    {"hastiar", "sanv"},
    {"fastidiar", "sanz"},
    {"importunar", "sany"},
    {"molestar", "sark"},
    {"incomodar", "sart"},
    {"repugnar", "sarp"},
    {"asquear", "sars"},
    {"avergonzar", "sarm"},
    {"ruborizar", "sarn"},
    {"sonrojar", "sarr"},
    {"enorgullecer", "sarl"},
    {"humillar", "sarv"},
    {"doblegar", "sarz"},
    {"someter", "sary"},
    {"claudicar", "salk"},
    {"desistir", "salt"},
    {"cejar", "salp"},
    {"obstinarse", "sals"},
    {"terquear", "salm"},
    {"porfiar", "saln"},
    {"persistir", "salr"},
    {"perseverar", "sall"},
    {"desvelarse", "salv"},
    {"adormilarse", "salz"},
    {"despejarse", "saly"},
    {"reverdecer", "savk"},
    {"florecer", "savt"},
    {"marchitar", "savp"},
    {"ajarse", "savs"},
    {"enorgullecerse", "savm"},
    {"avergonzarse", "savn"},
    {"arrepentirse", "savr"},
    {"lamentarse", "savl"},
    {"apiadarse", "savv"},
    {"compadecerse", "savz"},
    {"condolerse", "savy"},
    {"alegrarse", "sazk"},
    {"entristecerse", "sazt"},
    {"afligirse", "sazp"},
    {"angustiarse", "sazs"},
    {"desesperarse", "sazm"},
    {"enfurecerse", "sazn"},
    {"encolerizarse", "sazr"},
    {"calmarse", "sazl"},
    {"serenarse", "sazv"},
    {"sosegarse", "sazz"},
    {"apaciguarse", "sazy"},
    {"tranquilizarse", "sayk"},
    {"aliviarse", "sayt"},
    {"consolarse", "sayp"},
    {"animarse", "says"},
    {"alentarse", "saym"},
    {"ilusionarse", "sayn"},
    {"esperanzarse", "sayr"},
    {"desilusionarse", "sayl"},
    {"desencantarse", "sayv"},
    {"desengañarse", "sayz"},
    {"escarmentar", "sayy"},
    {"deleitarse", "sekk"},
    {"regocijarse", "sekt"},
    {"embelesarse", "sekp"},
    {"extasiarse", "seks"},
    {"arrobarse", "sekm"},
    {"pasmarse", "sekn"},
    {"asombrarse", "sekr"},
    {"espantarse", "sekl"},
    {"estremecerse", "sekv"},
    {"erizarse", "sekz"},
    {"escalofriarse", "seky"},
    {"acelerar", "setk"},
    {"desacelerar", "sett"},
    {"frenar", "setp"},
    {"precipitar", "sets"},
    {"apresurar", "setm"},
    {"retardar", "setn"},
    {"demorar", "setr"},
    {"posponer", "setl"},
    {"adelantar", "setv"},
    {"avanzar", "setz"},
    {"retroceder", "sety"},
    {"recular", "sepk"},
    {"retornar", "sept"},
    {"desviar", "sepp"},
    {"doblar", "seps"},
    {"plegar", "sepm"},
    {"desplegar", "sepn"},
    {"enrollar", "sepr"},
    {"desenrollar", "sepl"},
    {"estirar", "sepv"},
    {"encoger", "sepz"},
    {"dilatar", "sepy"},
    {"comprimir", "sesk"},
    {"inflar", "sest"},
    {"desinflar", "sesp"},
    {"llenar", "sess"},
    {"colmar", "sesm"},
    {"verter", "sesn"},
    {"derramar", "sesr"},
    {"salpicar", "sesl"},
    {"rociar", "sesv"},
    {"irrigar", "sesz"},
    {"escurrir", "sesy"},
    {"gotear", "semk"},
    {"chorrear", "semt"},
    {"manar", "semp"},
    {"brotar", "sems"},
    {"rezumar", "semm"},
    {"infiltrarse", "semn"},
    {"esparcir", "semr"},
    {"dispersar", "seml"},
    {"diseminar", "semv"},
    {"desparramar", "semz"},
    {"esculpir", "semy"},
    {"tallar", "senk"},
    {"cincelar", "sent"},
    {"forjar", "senp"},
    {"remachar", "sens"},
    {"atornillar", "senm"},
    {"enroscar", "senn"},
    {"desenroscar", "senr"},
    {"ajustar", "senl"},
    {"desajustar", "senv"},
    {"calzar", "senz"},
    {"encajar", "seny"},
    {"acoplar", "serk"},
    {"desacoplar", "sert"},
    {"engranar", "serp"},
    {"ensamblar", "sers"},
    {"desensamblar", "serm"},
    {"montar", "sern"},
    {"desmontar", "serr"},
    {"armar", "serl"},
    {"desarmar", "serv"},
    {"tensar", "serz"},
    {"destensar", "sery"},
    {"aflojar", "selk"},
    {"apretar", "selt"},
    {"extinguir", "selp"},
    {"abanicar", "sels"},
    {"exprimir", "selm"},
    {"estrujar", "seln"},
    {"prensar", "selr"},
    {"laminar", "sell"},
    {"moldear", "selv"},
    {"troquelar", "selz"},
    {"estampar", "sely"},
    {"grabar", "sevk"},
    {"chocar", "sevt"},
    {"colisionar", "sevp"},
    {"estrellarse", "sevs"},
    {"naufragar", "sevm"},
    {"zozobrar", "sevn"},
    {"encallar", "sevr"},
    {"hundirse", "sevl"},
    {"sumergirse", "sevv"},
    {"zambullirse", "sevz"},
    {"bucear", "sevy"},
    {"chapotear", "sezk"},
    {"surcar", "sezt"},
    {"merodear", "sezp"},
    {"vagar", "sezs"},
    {"deambular", "sezm"},
    {"peregrinar", "sezn"},
    {"emigrar", "sezr"},
    {"inmigrar", "sezl"},
    {"migrar", "sezv"},
    {"trasladarse", "sezz"},
    {"mudarse", "sezy"},
    {"desplazarse", "seyk"},
    {"contonearse", "seyt"},
    {"fluctuar", "seyp"},
    {"transmutar", "seys"},
    {"tornarse", "seym"},
    {"volverse", "seyn"},
    {"hacerse", "seyr"},
    {"quedarse", "seyl"},
    {"retomar", "seyv"},
    {"reemprender", "seyz"},
    {"reiniciar", "seyy"},
    {"recargar", "sikk"},
    {"descargar", "sikt"},
    {"sobrecargar", "sikp"},
    {"saturar", "siks"},
    {"desbordar", "sikm"},
    {"inundar", "sikn"},
    {"anegar", "sikr"},
    {"ahogar", "sikl"},
    {"asfixiar", "sikv"},
    {"sofocar", "sikz"},
    {"iluminar", "siky"},
    {"alumbrar", "sitk"},
    {"deslumbrar", "sitt"},
    {"cegar", "sitp"},
    {"encandilar", "sits"},
    {"ofuscar", "sitm"},
    {"obnubilar", "sitn"},
    {"aturdir", "sitr"},
    {"marear", "sitl"},
    {"extraviar", "sitv"},
    {"despistar", "sitz"},
    {"desconcertar", "sity"},
    {"descolocar", "sipk"},
    {"trastornar", "sipt"},
    {"perturbar", "sipp"},
    {"alterar", "sips"},
    {"inmutar", "sipm"},
    {"conmover", "sipn"},
    {"emocionar", "sipr"},
    {"enternecer", "sipl"},
    {"ablandar", "sipv"},
    {"colocar", "sipz"},
    {"emplazar", "sipy"},
    {"aposentar", "sisk"},
    {"yacer", "sist"},
    {"reposar", "sisp"},
    {"recostar", "siss"},
    {"tumbar", "sism"},
    {"erguir", "sisn"},
    {"inclinar", "sisr"},
    {"ladear", "sisl"},
    {"sesgar", "sisv"},
    {"alinear", "sisz"},
    {"centrar", "sisy"},
    {"descentrar", "simk"},
    {"enmarcar", "simt"},
    {"delimitar", "simp"},
    {"acotar", "sims"},
    {"demarcar", "simm"},
    {"cercar", "simn"},
    {"vallar", "simr"},
    {"rodear", "siml"},
    {"circundar", "simv"},
    {"envolver", "simz"},
    {"cubrir", "simy"},
    {"destapar", "sink"},
    {"obstruir", "sint"},
    {"desobstruir", "sinp"},
    {"bloquear", "sins"},
    {"desbloquear", "sinm"},
    {"atrancar", "sinn"},
    {"desatrancar", "sinr"},
    {"sellar", "sinl"},
    {"lacrar", "sinv"},
    {"precintar", "sinz"},
    {"desprecintar", "siny"},
    {"clausurar", "sirk"},
    {"inhabilitar", "sirt"},
    {"habilitar", "sirp"},
    {"atravesar", "sirs"},
    {"trasponer", "sirm"},
    {"vadear", "sirn"},
    {"sortear", "sirr"},
    {"esquivar", "sirl"},
    {"zigzaguear", "sirv"},
    {"serpentear", "sirz"},
    {"ondular", "siry"},
    {"balancear", "silk"},
    {"bambolear", "silt"},
    {"tambalear", "silp"},
    {"cabecear", "sils"},
    {"bascular", "silm"},
    {"pivotar", "siln"},
    {"orbitar", "silr"},
    {"circunvalar", "sill"},
    {"rondar", "silv"},
    {"remover", "silz"},
    {"batir", "sily"},
    {"agitar", "sivk"},
    {"zarandear", "sivt"},
    {"menear", "sivp"},
    {"blandir", "sivs"},
    {"esgrimir", "sivm"},
    {"enarbolar", "sivn"},
    {"izar", "sivr"},
    {"arriar", "sivl"},
    {"desfilar", "sivv"},
    {"empalmar", "sivz"},
    {"anudar", "sivy"},
    {"desanudar", "sizk"},
    {"atar", "sizt"},
    {"desatar", "sizp"},
    {"amarrar", "sizs"},
    {"desamarrar", "sizm"},
    {"sujetar", "sizn"},
    {"aferrar", "sizr"},
    {"asir", "sizl"},
    {"abandonar", "sizv"},
    {"desertar", "sizz"},
    {"abdicar", "sizy"},
    {"dimitir", "siyk"},
    {"jubilarse", "siyt"},
    {"retirarse", "siyp"},
    {"recluirse", "siys"},
    {"aislarse", "siym"},
    {"encerrarse", "siyn"},
    {"ocultarse", "siyr"},
    {"esconderse", "siyl"},
    {"camuflarse", "siyv"},
    {"mimetizarse", "siyz"},
    {"disfrazarse", "siyy"},
    {"enmascarar", "sokk"},
    {"desenmascarar", "sokt"},
    {"ostentar", "sokp"},
    {"lucir", "soks"},
    {"jactarse", "sokm"},
    {"vanagloriarse", "sokn"},
    {"alardear", "sokr"},
    {"fanfarronear", "sokl"},
    {"pavonearse", "sokv"},
    {"engreírse", "sokz"},
    {"envanecerse", "soky"},
    {"ufanarse", "sotk"},
    {"perdurar", "sott"},
    {"subsistir", "sotp"},
    {"pervivir", "sots"},
    {"durar", "sotm"},
    {"prolongar", "sotn"},
    {"prorrogar", "sotr"},
    {"acortar", "sotl"},
    {"pronosticar", "sotv"},
    {"vaticinar", "sotz"},
    {"augurar", "soty"},
    {"profetizar", "sopk"},
    {"madurar", "sopt"},
    {"rejuvenecer", "sopp"},
    {"caducar", "sops"},
    {"expirar", "sopm"},
    {"prescribir", "sopn"},
    {"debutar", "sopr"},
    {"estrenar", "sopl"},
    {"inaugurar", "sopv"},
    {"suspender", "sopz"},
    {"aplazar", "sopy"},
    {"diferir", "sosk"},
    {"procrastinar", "sost"},
    {"premura", "sosp"},
    {"prontitud", "soss"},
    {"puntualidad", "sosm"},
    {"tardanza", "sosn"},
    {"espera", "sosr"},
    {"antesala", "sosl"},
    {"preludio", "sosv"},
    {"epílogo", "sosz"},
    {"interludio", "sosy"},
    {"entreacto", "somk"},
    {"hiato", "somt"},
    {"lapso", "somp"},
    {"intervalo", "soms"},
    {"trecho", "somm"},
    {"tramo", "somn"},
    {"etapa", "somr"},
    {"trance", "soml"},
    {"transición", "somv"},
    {"mudanza", "somz"},
    {"traslado", "somy"},
    {"trasiego", "sonk"},
    {"vaivén", "sont"},
    {"alternancia", "sonp"},
    {"tanda", "sons"},
    {"racha", "sonm"},
    {"ronda", "sonn"},
    {"ocasión", "sonr"},
    {"oportunidad", "sonl"},
    {"coyuntura", "sonv"},
    {"inmenso", "sonz"},
    {"minúsculo", "sony"},
    {"colosal", "sork"},
    {"enorme", "sort"},
    {"ingente", "sorp"},
    {"exiguo", "sors"},
    {"escaso", "sorm"},
    {"copioso", "sorn"},
    {"profuso", "sorr"},
    {"exuberante", "sorl"},
    {"sobrio", "sorv"},
    {"austero", "sorz"},
    {"opulento", "sory"},
    {"suntuoso", "solk"},
    {"modesto", "solt"},
    {"humilde", "solp"},
    {"sencillo", "sols"},
    {"intrincado", "solm"},
    {"enrevesado", "soln"},
    {"sutil", "solr"},
    {"delicado", "soll"},
    {"frágil", "solv"},
    {"quebradizo", "solz"},
    {"robusto", "soly"},
    {"compacto", "sovk"},
    {"tenue", "sovt"},
    {"tupido", "sovp"},
    {"ralo", "sovs"},
    {"nítido", "sovm"},
    {"borroso", "sovn"},
    {"difuso", "sovr"},
    {"vago", "sovl"},
    {"certero", "sovv"},
    {"atinado", "sovz"},
    {"desacertado", "sovy"},
    {"errático", "sozk"},
    {"caprichoso", "sozt"},
    {"voluble", "sozp"},
    {"inconstante", "sozs"},
    {"firme", "sozm"},
    {"férreo", "sozn"},
    {"inquebrantable", "sozr"},
    {"indomable", "sozl"},
    {"manso", "sozv"},
    {"dócil", "sozz"},
    {"sumiso", "sozy"},
    {"rebelde", "soyk"},
    {"insumiso", "soyt"},
    {"terco", "soyp"},
    {"testarudo", "soys"},
    {"tozudo", "soym"},
    {"obstinado", "soyn"},
    {"contumaz", "soyr"},
    {"porfiado", "soyl"},
    {"empecinado", "soyv"},
    {"versátil", "soyz"},
    {"adaptable", "soyy"},
    {"flexible", "sukk"},
    {"rígido", "sukt"},
    {"tieso", "sukp"},
    {"laxo", "suks"},
    {"tenso", "sukm"},
    {"tirante", "sukn"},
    {"holgado", "sukr"},
    {"ceñido", "sukl"},
    {"suelto", "sukv"},
    {"cautivo", "sukz"},
    {"prisionero", "suky"},
    {"fugitivo", "sutk"},
    {"prófugo", "sutt"},
    {"errante", "sutp"},
    {"vagabundo", "suts"},
    {"nómada", "sutm"},
    {"sedentario", "sutn"},
    {"estable", "sutr"},
    {"inestable", "sutl"},
    {"precario", "sutv"},
    {"efímero", "sutz"},
    {"fugaz", "suty"},
    {"perenne", "supk"},
    {"eterno", "supt"},
    {"inmortal", "supp"},
    {"perecedero", "sups"},
    {"formidable", "supm"},
    {"prodigioso", "supn"},
    {"portentoso", "supr"},
    {"asombroso", "supl"},
    {"pasmoso", "supv"},
    {"estupendo", "supz"},
    {"magnífico", "supy"},
    {"espléndido", "susk"},
    {"soberbio", "sust"},
    {"exquisito", "susp"},
    {"refinado", "suss"},
    {"elegante", "susm"},
    {"solemne", "susn"},
    {"majestuoso", "susr"},
    {"imponente", "susl"},
    {"augusto", "susv"},
    {"venerable", "susz"},
    {"honorable", "susy"},
    {"digno", "sumk"},
    {"indigno", "sumt"},
    {"decoroso", "sump"},
    {"indecoroso", "sums"},
    {"púdico", "summ"},
    {"impúdico", "sumn"},
    {"casto", "sumr"},
    {"lascivo", "suml"},
    {"lúbrico", "sumv"},
    {"libidinoso", "sumz"},
    {"sensual", "sumy"},
    {"voluptuoso", "sunk"},
    {"carnal", "sunt"},
    {"platónico", "sunp"},
    {"idealista", "suns"},
    {"soñador", "sunm"},
    {"visionario", "sunn"},
    {"utópico", "sunr"},
    {"quimérico", "sunl"},
    {"ilusorio", "sunv"},
    {"engañoso", "sunz"},
    {"falaz", "suny"},
    {"artero", "surk"},
    {"taimado", "surt"},
    {"ladino", "surp"},
    {"astuto", "surs"},
    {"sagaz", "surm"},
    {"perspicaz", "surn"},
    {"agudo", "surr"},
    {"obtuso", "surl"},
    {"torpe", "surv"},
    {"hábil", "surz"},
    {"diestro", "sury"},
    {"siniestro", "sulk"},
    {"zurdo", "sult"},
    {"ambidiestro", "sulp"},
    {"mañoso", "suls"},
    {"apañado", "sulm"},
    {"desmañado", "suln"},
    {"patoso", "sulr"},
    {"inhábil", "sull"},
    {"servible", "sulv"},
    {"funcional", "sulz"},
    {"operativo", "suly"},
    {"viable", "suvk"},
    {"factible", "suvt"},
    {"realizable", "suvp"},
    {"irrealizable", "suvs"},
    {"sinnúmero", "suvm"},
    {"sinfín", "suvn"},
    {"montón", "suvr"},
    {"pila", "suvl"},
    {"puñado", "suvv"},
    {"pizca", "suvz"},
    {"pellizco", "suvy"},
    {"migaja", "suzk"},
    {"ápice", "suzt"},
    {"brizna", "suzp"},
    {"hebra", "suzs"},
    {"filamento", "suzm"},
    {"partícula", "suzn"},
    {"corpúsculo", "suzr"},
    {"gota", "suzl"},
    {"cúmulo", "suzv"},
    {"acervo", "suzz"},
    {"caudal", "suzy"},
    {"aluvión", "suyk"},
    {"avalancha", "suyt"},
    {"chubasco", "suyp"},
    {"aguacero", "suys"},
    {"diluvio", "suym"},
    {"torrente", "suyn"},
    {"raudales", "suyr"},
    {"infinidad", "suyl"},
    {"pluralidad", "suyv"},
    {"multiplicidad", "suyz"},
    {"diversidad", "suyy"},
    {"variedad", "makk"},
    {"surtido", "makt"},
    {"miscelánea", "makp"},
    {"popurrí", "maks"},
    {"batiburrillo", "makm"},
    {"revoltijo", "makn"},
    {"fárrago", "makr"},
    {"mezcolanza", "makl"},
    {"totum", "makv"},
    {"pisto", "makz"},
    {"ensalada", "maky"},
    {"cóctel", "matk"},
    {"combinado", "matt"},
    {"enlazar", "matp"},
    {"conectar", "mats"},
    {"desconectar", "matm"},
    {"relacionar", "matn"},
    {"alternar", "matr"},
    {"frecuentar", "matl"},
    {"cortejar", "matv"},
    {"agasajar", "matz"},
    {"homenajear", "maty"},
    {"condecorar", "mapk"},
    {"recompensar", "mapt"},
    {"remunerar", "mapp"},
    {"indemnizar", "maps"},
    {"compensar", "mapm"},
    {"resarcir", "mapn"},
    {"auxiliar", "mapr"},
    {"socorrer", "mapl"},
    {"amparar", "mapv"},
    {"abogar", "mapz"},
    {"interceder", "mapy"},
    {"mediar", "mask"},
    {"arbitrar", "mast"},
    {"conciliar", "masp"},
    {"reconciliar", "mass"},
    {"enemistar", "masm"},
    {"combatir", "masn"},
    {"lidiar", "masr"},
    {"rivalizar", "masl"},
    {"competir", "masv"},
    {"remedar", "masz"},
    {"parodiar", "masy"},
    {"caricaturizar", "mamk"},
    {"satirizar", "mamt"},
    {"ironizar", "mamp"},
    {"bromear", "mams"},
    {"mofarse", "mamm"},
    {"burlarse", "mamn"},
    {"escarnecer", "mamr"},
    {"vilipendiar", "maml"},
    {"difamar", "mamv"},
    {"calumniar", "mamz"},
    {"infamar", "mamy"},
    {"desacreditar", "mank"},
    {"desprestigiar", "mant"},
    {"deshonrar", "manp"},
    {"afrentar", "mans"},
    {"ultrajar", "manm"},
    {"menospreciar", "mann"},
    {"desdeñar", "manr"},
    {"ningunear", "manl"},
    {"orillar", "manv"},
    {"marginar", "manz"},
    {"excluir", "many"},
    {"acoger", "mark"},
    {"albergar", "mart"},
    {"hospedar", "marp"},
    {"asilar", "mars"},
    {"refugiar", "marm"},
    {"cobijar", "marn"},
    {"custodiar", "marr"},
    {"escoltar", "marl"},
    {"capitanear", "marv"},
    {"regir", "marz"},
    {"administrar", "mary"},
    {"gestionar", "malk"},
    {"coordinar", "malt"},
    {"sustentar", "malp"},
    {"ejercitar", "mals"},
    {"adiestrar", "malm"},
    {"capacitar", "maln"},
    {"alfabetizar", "malr"},
    {"desacertar", "mall"},
    {"malograr", "malv"},
    {"frustrar", "malz"},
    {"estropear", "maly"},
    {"rectificar", "mavk"},
    {"subsanar", "mavt"},
    {"remediar", "mavp"},
    {"componer", "mavs"},
    {"perfeccionar", "mavm"},
    {"pulir", "mavn"},
    {"afinar", "mavr"},
    {"optimizar", "mavl"},
    {"degradar", "mavv"},
    {"deteriorar", "mavz"},
    {"desgastar", "mavy"},
    {"agotar", "mazk"},
    {"actualizar", "mazt"},
    {"modernizar", "mazp"},
    {"innovar", "mazs"},
    {"revolucionar", "mazm"},
    {"reestructurar", "mazn"},
    {"reinventar", "mazr"},
    {"rediseñar", "mazl"},
    {"replantear", "mazv"},
    {"reconsiderar", "mazz"},
    {"reevaluar", "mazy"},
    {"reexaminar", "mayk"},
    {"releer", "mayt"},
    {"reescribir", "mayp"},
    {"rehacer", "mays"},
    {"reconstruir", "maym"},
    {"recuperar", "mayn"},
    {"preservar", "mayr"},
    {"conservar", "mayl"},
    {"salvaguardar", "mayv"},
};
#define N_RESH_LEXICO ((int)(sizeof RESH_LEXICO / sizeof RESH_LEXICO[0]))

#endif


#define NROLES 18
#define MAXSEM 1000        /* semiones por mente */
#define MAXVEC 16          /* acoples por semión */
#define DIMF 8             /* rasgos de la firma */
#define LARGO 28           /* bytes de una etiqueta (con el 0 final) */
#define HASHN 4096         /* tabla etiqueta -> semión (potencia de 2) */
#define NACC 6             /* cosas que una mente puede querer hacer */
#define NVEN 16            /* ventana de contexto (lo último que vivió) */
#define LVEN 112
#define MAXEST 24          /* palabras por estímulo */
#define NCAM 48            /* caminos A->B->C recordados */
#define MAXFRASE 12
#define NYX_PI 3.14159265f
#define NYX_2PI 6.28318531f

/* ------------------------------------------------------------------ */
/*  Azar (xorshift: igual en todas las plataformas)                    */
/* ------------------------------------------------------------------ */

static unsigned long long nyx_azar_estado = 88172645463325252ULL;

static void nyx_semilla(unsigned long long s) {
    nyx_azar_estado = s ? s : 88172645463325252ULL;
}

static unsigned nyx_u32(void) {
    unsigned long long x = nyx_azar_estado;
    x ^= x << 13;
    x ^= x >> 7;
    x ^= x << 17;
    nyx_azar_estado = x;
    return (unsigned)(x >> 32);
}

static float nyx_f01(void) { return (float)(nyx_u32() >> 8) * (1.0f / 16777216.0f); }
static float nyx_rango(float a, float b) { return a + (b - a) * nyx_f01(); }
static int nyx_ent(int n) { return n <= 0 ? 0 : (int)(nyx_u32() % (unsigned)n); }
static float nyx_min(float a, float b) { return a < b ? a : b; }
static float nyx_max(float a, float b) { return a > b ? a : b; }
static float nyx_lim(float x, float a, float b) { return x < a ? a : (x > b ? b : x); }

static unsigned nyx_fnv(const char *s) {
    unsigned h = 2166136261u;
    while (*s) { h ^= (unsigned char)*s++; h *= 16777619u; }
    return h;
}

static void nyx_copia(char *dst, const char *src, size_t cap) {
    size_t i = 0;
    if (cap == 0) return;
    while (src[i] && i + 1 < cap) { dst[i] = src[i]; i++; }
    /* no cortar un carácter UTF-8 por la mitad */
    while (i > 0 && ((unsigned char)dst[i - 1] & 0xC0) == 0x80) {
        size_t j = i - 1;
        while (j > 0 && ((unsigned char)dst[j] & 0xC0) == 0x80) j--;
        {
            unsigned char c = (unsigned char)dst[j];
            size_t largo = c >= 0xF0 ? 4 : c >= 0xE0 ? 3 : c >= 0xC0 ? 2 : 1;
            if (j + largo <= i) break;
            i = j;
        }
    }
    dst[i] = 0;
}

/* ------------------------------------------------------------------ */
/*  Roles: cómo es cada una de las 18                                  */
/* ------------------------------------------------------------------ */

enum { ACC_HABLAR, ACC_PREGUNTAR, ACC_IMAGINAR, ACC_SONAR, ACC_RECORDAR, ACC_ESCUCHAR };
static const char *NYX_ACC[NACC] = { "hablar", "preguntar", "imaginar", "soñar", "recordar", "escuchar" };

typedef struct {
    const char *nombre;
    float ruido, rigidez, A0;   /* η, λ, A₀ de la dinámica */
    float sabeResh;             /* qué parte del Resh sabe al nacer */
    const char *objetivo;       /* lo que persigue */
    float gusto[NACC];          /* gusto innato por cada acción */
    int color;                  /* color ANSI 256 para la UI */
} RolInfo;

static const RolInfo NYX_ROLES[NROLES] = {
    { "sintaxis",    0.02f, 1.2f,  0.65f, 0.45f, "frase bien formada orden claro estructura", {.5f,.2f,.1f,.4f,.3f,.3f},  39 },
    { "semantica",   0.05f, 0.8f,  0.60f, 0.40f, "significado claro que se entienda",          {.6f,.3f,.2f,.3f,.3f,.3f},  45 },
    { "logica",      0.01f, 1.4f,  0.70f, 0.30f, "valido sin contradiccion coherente",         {.4f,.3f,.1f,.6f,.2f,.3f},  33 },
    { "codigo",      0.03f, 1.0f,  0.65f, 0.30f, "preciso ejecutable correcto",                {.5f,.2f,.2f,.5f,.2f,.2f},  46 },
    { "creativo",    0.12f, 0.4f,  0.50f, 0.25f, "nuevo inesperado original posibilidad",      {.4f,.2f,.9f,.2f,.2f,.1f}, 213 },
    { "critico",     0.02f, 1.5f,  0.55f, 0.25f, "falla error objecion verifica",              {.7f,.4f,.1f,.2f,.2f,.3f}, 196 },
    { "memoria",     0.03f, 0.5f,  0.80f, 0.55f, "recuerdo relevante pasado patron",           {.3f,.2f,.1f,.5f,.9f,.3f}, 141 },
    { "percepcion",  0.06f, 0.7f,  0.60f, 0.25f, "observa describe forma color ritmo",         {.4f,.3f,.3f,.2f,.2f,.6f}, 214 },
    { "sintesis",    0.09f, 0.5f,  0.55f, 0.30f, "integra une resume concluye",                {.6f,.2f,.5f,.5f,.3f,.2f},  82 },
    { "intuicion",   0.14f, 0.3f,  0.55f, 0.20f, "rapido corazonada destello ahora",           {.3f,.2f,.7f,.2f,.2f,.4f}, 219 },
    { "analogia",    0.11f, 0.45f, 0.55f, 0.25f, "puente entre dominios como metafora",        {.5f,.2f,.7f,.3f,.2f,.2f}, 117 },
    { "contexto",    0.04f, 0.9f,  0.70f, 0.35f, "hilo situacion aqui ahora marco",            {.4f,.3f,.1f,.3f,.5f,.5f}, 186 },
    { "esceptico",   0.03f, 1.3f,  0.55f, 0.25f, "duda pregunta cuestiona verifica",           {.4f,.8f,.1f,.3f,.2f,.3f}, 167 },
    { "narrativa",   0.06f, 0.6f,  0.60f, 0.35f, "secuencia entonces despues historia",        {.6f,.1f,.3f,.2f,.8f,.2f}, 179 },
    { "etica",       0.01f, 1.6f,  0.65f, 0.35f, "justo bien deber correcto",                  {.5f,.3f,.1f,.5f,.3f,.4f},  77 },
    { "curiosidad",  0.10f, 0.4f,  0.55f, 0.20f, "novedad explora descubre pregunta",          {.3f,.9f,.5f,.2f,.2f,.3f}, 226 },
    { "abstraccion", 0.05f, 0.7f,  0.75f, 0.30f, "esencia general eleva patron",               {.4f,.2f,.6f,.6f,.2f,.2f}, 105 },
    { "empatia",     0.05f, 0.6f,  0.65f, 0.35f, "escucha comprende alinea acompana",          {.5f,.3f,.2f,.2f,.3f,.8f}, 210 },
};

static int nyx_rol_de(const char *nombre) {
    int r;
    for (r = 0; r < NROLES; r++)
        if (strcmp(NYX_ROLES[r].nombre, nombre) == 0) return r;
    /* también vale el comienzo: "crea" -> creativo */
    for (r = 0; r < NROLES; r++)
        if (strlen(nombre) >= 3 && strncmp(NYX_ROLES[r].nombre, nombre, strlen(nombre)) == 0) return r;
    return -1;
}

/* Lo que leen al nacer: la infancia de las 18 (si no hay memoria guardada). */
static const char *NYX_CORPUS[] = {
    "la mente aprende cuando escucha y pregunta",
    "una idea nueva nace cuando dos ideas se unen",
    "la luz del sol hace crecer el árbol",
    "el agua corre por el río hasta el mar",
    "cada pregunta abre un camino nuevo",
    "el error enseña más que el acierto",
    "una historia tiene un principio y un final",
    "el recuerdo guarda lo que fue importante",
    "la duda ayuda a buscar la verdad",
    "un buen código es claro y correcto",
    "la palabra justa dice mucho con poco",
    "el amigo escucha y comprende",
    "la música tiene ritmo y forma",
    "el niño juega y descubre el mundo",
    "la noche trae silencio y estrellas",
    "pensar es unir lo que parece separado",
    "el tiempo pasa y la memoria queda",
    "un patrón se repite en muchas cosas",
    "la verdad se busca con preguntas",
    "el bien y el deber guían lo justo",
    "el color del cielo cambia con la luz",
    "la ciencia observa mide y explica",
    "un problema grande se divide en partes pequeñas",
    "la imaginación ve lo que todavía no existe",
    "el lenguaje une a las mentes",
    "aprender es cambiar con lo que se vive",
    "la casa protege del frío y de la lluvia",
    "el camino largo empieza con un paso",
    "la historia del mundo está llena de cambios",
    "el sueño ordena lo que pasó en el día",
    "una buena razón convence sin gritar",
    "la naturaleza tiene árboles ríos y montañas",
    "el número cuenta y la palabra explica",
    "escuchar al otro es una forma de respeto",
    "la curiosidad empuja a explorar",
    "lo simple es más fuerte que lo complicado",
    "el fuego da calor y luz",
    "cada mente piensa de forma distinta",
    "juntas las mentes ven más lejos",
    "la esencia de algo es lo que no cambia",
};
#define NYX_NCORPUS ((int)(sizeof NYX_CORPUS / sizeof NYX_CORPUS[0]))

/* Palabras vacías: sirven para la gramática, nunca para ganar un veredicto. */
static const char *NYX_VACIAS[] = {
    "el", "la", "los", "las", "un", "una", "unos", "unas", "de", "del", "al", "a", "y", "o", "u", "e",
    "que", "en", "por", "con", "para", "se", "su", "sus", "lo", "le", "les", "es", "son", "no", "mi",
    "tu", "me", "te", "nos", "ya", "más", "mas", "muy", "pero", "si", "sí", "como", "cuando", "hay",
    "este", "esta", "esto", "ese", "esa", "eso", "fue", "ser", "está", "están", "qué", "cómo", "yo",
    "tú", "él", "ella", "todo", "toda", "sin", "sobre", "entre", "hasta", "desde", "también", "ni",
};

static int nyx_vacia(const char *w) {
    size_t i;
    for (i = 0; i < sizeof NYX_VACIAS / sizeof NYX_VACIAS[0]; i++)
        if (strcmp(NYX_VACIAS[i], w) == 0) return 1;
    return 0;
}

static int nyx_conector(const char *w) {
    static const char *c[] = { "y", "o", "u", "e", "en", "por", "con", "de", "del", "que", "a", "al", "para", "ni", "pero", "sin", "como", "se", "es", "son" };
    size_t i;
    for (i = 0; i < sizeof c / sizeof c[0]; i++) if (strcmp(c[i], w) == 0) return 1;
    return 0;
}

/* ------------------------------------------------------------------ */
/*  Diccionario Resh (tabla hash sobre resh.h)                         */
/* ------------------------------------------------------------------ */

#define NYX_DICN 8192
static short nyx_dic_es[NYX_DICN], nyx_dic_re[NYX_DICN];
static int nyx_dic_listo = 0;

static const ParResh *nyx_par(int k) {
    return k < N_RESH_PROTOCOLO ? &RESH_PROTOCOLO[k] : &RESH_LEXICO[k - N_RESH_PROTOCOLO];
}

static void nyx_dic_pon(short *t, int k, int porResh) {
    const char *clave = porResh ? nyx_par(k)->resh : nyx_par(k)->es;
    unsigned h = nyx_fnv(clave) & (NYX_DICN - 1);
    while (t[h] >= 0) {
        const ParResh *p = nyx_par(t[h]);
        if (strcmp(porResh ? p->resh : p->es, clave) == 0) return;  /* el primero manda */
        h = (h + 1) & (NYX_DICN - 1);
    }
    t[h] = (short)k;
}

static void nyx_dic_init(void) {
    int k;
    if (nyx_dic_listo) return;
    for (k = 0; k < NYX_DICN; k++) { nyx_dic_es[k] = -1; nyx_dic_re[k] = -1; }
    for (k = 0; k < N_RESH_PROTOCOLO + N_RESH_LEXICO; k++) {
        nyx_dic_pon(nyx_dic_es, k, 0);
        nyx_dic_pon(nyx_dic_re, k, 1);
    }
    nyx_dic_listo = 1;
}

static const ParResh *nyx_dic_busca(const short *t, const char *clave, int porResh) {
    unsigned h = nyx_fnv(clave) & (NYX_DICN - 1);
    while (t[h] >= 0) {
        const ParResh *p = nyx_par(t[h]);
        if (strcmp(porResh ? p->resh : p->es, clave) == 0) return p;
        h = (h + 1) & (NYX_DICN - 1);
    }
    return NULL;
}

/* español -> resh (NULL si no está en el léxico) */
static const char *nyx_a_resh(const char *es) {
    const ParResh *p = nyx_dic_busca(nyx_dic_es, es, 0);
    return p ? p->resh : NULL;
}

/* resh -> español */
static const char *nyx_a_es(const char *resh) {
    const ParResh *p = nyx_dic_busca(nyx_dic_re, resh, 1);
    return p ? p->es : NULL;
}

static int nyx_es_particula(const char *es) {
    int k;
    for (k = 0; k < N_RESH_PROTOCOLO; k++)
        if (strcmp(RESH_PROTOCOLO[k].es, es) == 0) return 1;
    return 0;
}

/* ------------------------------------------------------------------ */
/*  Texto: palabras en minúscula, sin puntuación                       */
/* ------------------------------------------------------------------ */

static int nyx_tokens(const char *t, char toks[][LARGO], int max) {
    int n = 0;
    char w[64];
    int lw = 0;
    const unsigned char *p = (const unsigned char *)t;
    for (;;) {
        unsigned char c = *p;
        int corta = 0;
        if (c == 0) corta = 1;
        else if (c < 0x80) {
            if ((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
                if (lw < 60) w[lw++] = (char)c;
            } else if (c >= 'A' && c <= 'Z') {
                if (lw < 60) w[lw++] = (char)(c + 32);
            } else corta = 1;
            p++;
        } else if (c == 0xE2 && p[1] && p[2]) {
            corta = 1;            /* ◀ ▶ ⊕ … — símbolos, no letras */
            p += 3;
        } else if (c == 0xC2 && (p[1] == 0xBF || p[1] == 0xA1 || p[1] == 0xAB || p[1] == 0xBB)) {
            corta = 1;            /* ¿ ¡ « » */
            p += 2;
        } else if (c == 0xC3 && p[1] >= 0x80 && p[1] <= 0x9E && p[1] != 0x97) {
            if (lw < 59) { w[lw++] = (char)c; w[lw++] = (char)(p[1] + 0x20); }  /* Á -> á */
            p += 2;
        } else {
            if (lw < 60) w[lw++] = (char)c;
            p++;
        }
        if (corta || lw >= 60) {
            if (lw > 0 && n < max) {
                w[lw] = 0;
                nyx_copia(toks[n++], w, LARGO);
            }
            lw = 0;
            if (c == 0) break;
        }
    }
    return n;
}

/* Firma formal de una palabra (sin embeddings): forma, ritmo y símbolos. */
static void nyx_firma(const char *w, float *f) {
    int len = 0, nb = 0, sim = 0, dig = 0, voc = 0, simb = 0, acen = 0, i;
    double suma = 0, var = 0, media;
    int L = (int)strlen(w);
    for (i = 0; i < L; i++) {
        unsigned char c = (unsigned char)w[i];
        if ((c & 0xC0) != 0x80) len++;
        suma += c; nb++;
        if (c >= '0' && c <= '9') dig++;
        if (strchr("aeiou", c) && c) voc++;
        if (strchr("+-*/=<>^", c) && c) simb++;
        if (c >= 0x80) acen++;
    }
    media = nb ? suma / nb : 0;
    for (i = 0; i < L; i++) { double d = (unsigned char)w[i] - media; var += d * d; }
    var = nb ? var / nb : 0;
    for (i = 0; i < L / 2; i++) if (w[i] == w[L - 1 - i]) sim++;
    if (len == 0) len = 1;
    f[0] = (float)len / 20.0f;
    f[1] = (float)fmod(media, 128.0) / 128.0f;
    f[2] = nyx_min(1.0f, (float)(var / 4000.0));
    f[3] = (float)sim / (float)len;
    f[4] = (float)dig / (float)len;
    f[5] = (float)acen / (float)(nb ? nb : 1);
    f[6] = (float)voc / (float)len;
    f[7] = (float)simb / (float)len;
}

/* ------------------------------------------------------------------ */
/*  Mente                                                              */
/* ------------------------------------------------------------------ */

typedef struct {
    short j;
    float w;      /* asociación (resonancia) */
    float th;     /* desfase preferido: 0 acuerdo, π oposición */
    float sec;    /* secuencia: cuánto "este va seguido de j" */
} Acople;

typedef struct {
    char et[LARGO];
    float f[DIMF];
    float A, fase, frec;
    signed char carga;
    unsigned char fusion;   /* idea propia (a⊕b) */
    unsigned char sabe;     /* sabe decirla en Resh */
    unsigned char nv;
    unsigned short usos, dialogo;
    Acople v[MAXVEC];
} Semion;

typedef struct { short a, b, c; unsigned char k; } Camino;

typedef struct {
    int rol;
    int n;
    Semion s[MAXSEM];
    short hash[HASHN];
    float ruido, rigidez, A0;
    short obj[8]; int nobj;
    short foco[7]; int nfoco;          /* memoria de trabajo (7±2) */
    short epis[24]; int nepis;         /* memoria episódica */
    Camino cam[NCAM]; int ncam;
    int aciertos, intentos;
    float valor[NACC];                 /* lo aprendido: cuánto le conviene cada acción */
    int veces[NACC];
    int ultima, racha;
    short recientes[6]; int nrec;      /* lo que ganó hace poco (fatiga) */
    char ven[NVEN][LVEN]; int iven, nven;
    long ciclos, dichos, ideas, cristales, reshAprendidas;
} Mente;

static float m_precision(const Mente *m) {
    return m->intentos > 0 ? (float)m->aciertos / (float)m->intentos : 0.5f;
}

static void m_ventana(Mente *m, const char *texto) {
    nyx_copia(m->ven[m->iven], texto, LVEN);
    m->iven = (m->iven + 1) % NVEN;
    if (m->nven < NVEN) m->nven++;
}

static int m_busca(const Mente *m, const char *et) {
    unsigned h = nyx_fnv(et) & (HASHN - 1);
    while (m->hash[h] >= 0) {
        if (strcmp(m->s[m->hash[h]].et, et) == 0) return m->hash[h];
        h = (h + 1) & (HASHN - 1);
    }
    return -1;
}

static void m_hash_pon(Mente *m, int i) {
    unsigned h = nyx_fnv(m->s[i].et) & (HASHN - 1);
    while (m->hash[h] >= 0) h = (h + 1) & (HASHN - 1);
    m->hash[h] = (short)i;
}

static void m_hash_rehace(Mente *m) {
    int i;
    for (i = 0; i < HASHN; i++) m->hash[i] = -1;
    for (i = 0; i < m->n; i++) m_hash_pon(m, i);
}

static int m_nuevo(Mente *m, const char *et, int fusion) {
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

static Acople *m_acople(Mente *m, int a, int b) {
    Semion *s = &m->s[a];
    int k;
    for (k = 0; k < s->nv; k++)
        if (s->v[k].j == b) return &s->v[k];
    return NULL;
}

/* Crea el acople a->b (si no cabe, reemplaza el más débil). */
static Acople *m_acople_nuevo(Mente *m, int a, int b, float w, float th) {
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

static void m_fija(Mente *m, int a, int b, float w, float th) {
    Acople *c = m_acople(m, a, b);
    if (!c) c = m_acople_nuevo(m, a, b, w, th);
    if (c) { c->w = w; c->th = th; }
}

/* Refuerzo con saturación: repetir acerca a 1 sin pasarse. */
static void m_refuerza(Mente *m, int a, int b, float dw, float th, float dsec) {
    Acople *c;
    if (a == b) return;
    c = m_acople(m, a, b);
    if (!c) { c = m_acople_nuevo(m, a, b, dw, th); if (!c) return; c->sec = dsec; return; }
    c->w += dw * (1 - c->w);
    c->sec += dsec * (1 - c->sec);
}

static float m_coh(const Mente *m, int i) {
    const Semion *s = &m->s[i];
    float c = 0;
    int k;
    for (k = 0; k < s->nv; k++) {
        const Semion *o = &m->s[s->v[k].j];
        c += s->v[k].w * s->A * o->A * cosf(s->fase - o->fase - s->v[k].th);
    }
    return c;
}

static void m_envuelve(float *f) {
    *f = fmodf(*f, NYX_2PI);
    if (*f < 0) *f += NYX_2PI;
}

static void m_decae(Mente *m, int i, float ruido) {
    Semion *s = &m->s[i];
    float A = s->A;
    float dA = -m->rigidez * (A * A - m->A0 * m->A0) * A * 0.02f;
    s->A = nyx_lim(A + dA, 0.05f, 1.0f);
    s->fase += s->frec * 0.02f + nyx_rango(-ruido, ruido);
    m_envuelve(&s->fase);
}

static void m_propaga(Mente *m, int i) {
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
static int m_region(const Mente *m, const int *est, int ne, int *out, int tope) {
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

static void m_atiende(Mente *m, int id) {
    int i, j = 0;
    for (i = 0; i < m->nfoco; i++) if (m->foco[i] != id) m->foco[j++] = m->foco[i];
    m->nfoco = j;
    if (m->nfoco == 7) { memmove(m->foco, m->foco + 1, 6 * sizeof m->foco[0]); m->nfoco = 6; }
    m->foco[m->nfoco++] = (short)id;
    m->s[id].A = nyx_min(1, m->s[id].A + 0.12f);
}

static void m_episodio(Mente *m, int id) {
    if (m->nepis == 24) { memmove(m->epis, m->epis + 1, 23 * sizeof m->epis[0]); m->nepis = 23; }
    m->epis[m->nepis++] = (short)id;
}

static int m_es_objetivo(const Mente *m, int id) {
    int k;
    for (k = 0; k < m->nobj; k++) if (m->obj[k] == id) return 1;
    return 0;
}

/* Poda homeostática: libera ~12% cuando la mente se llena. Primero caen las
 * ideas propias que nadie retomó; luego lo menos coherente y usado. */
static void m_poda(Mente *m) {
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
static int m_ingesta2(Mente *m, const char *texto, int *ids, int max, int dialogo, float gramatica);

/* Ingesta con gramática plena (lo que escribe el humano o un libro). */
static int m_ingesta(Mente *m, const char *texto, int *ids, int max, int dialogo) {
    return m_ingesta2(m, texto, ids, max, dialogo, 1.0f);
}

/* gramatica: cuánto se aprende el ORDEN de las palabras. Las frases de otras
 * mentes enseñan asociaciones, pero poca gramática: si no, se copiarían sus
 * frases a medio hacer y el idioma se degradaría. */
static int m_ingesta2(Mente *m, const char *texto, int *ids, int max, int dialogo, float gramatica) {
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
static int m_predice(const Mente *m, int *out, int max) {
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
static int m_recibe(Mente *m, const char *texto, const char *quien, int *ids, int max) {
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
static int m_salto(const Mente *m, int id, const int *vis, int nvis) {
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
static int m_cristaliza(Mente *m, int a, int b, int c) {
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
static int m_transitiva(Mente *m, int id) {
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

static float m_incrustacion(const Mente *m, int id) {
    const Semion *s = &m->s[id];
    int k, n = 0;
    if (s->nv == 0) return 0;
    for (k = 0; k < s->nv; k++) if (m->s[s->v[k].j].dialogo > 0) n++;
    return (float)n / (float)s->nv;
}

/* Veredicto: el estímulo despierta una región, se relaja con recocido
 * (caliente -> frío) y gana lo que la pregunta EVOCA (no su eco). */
static int m_veredicto(Mente *m, const int *est0, int ne0, float *C, float *E, int *cristal) {
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
static float m_en_contexto(const int *ctx, int nctx, int id) {
    int i;
    for (i = 0; i < nctx; i++) if (ctx[i] == id) return 0.6f;
    return 0;
}

/* ctx: las palabras de lo que se preguntó; la frase prefiere ir por ahí
 * (así "qué es el mar" lleva a "el mar es grande", no a otra frase con "es"). */
static int m_frase(Mente *m, int centro, int *out, int max, const int *ctx, int nctx) {
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

static void m_texto(const Mente *m, const int *ids, int n, char *es, size_t ces, char *resh, size_t cre) {
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
static int m_imagina(Mente *m, int *pa, int *pb) {
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
static void m_cansa(Mente *m, const int *ids, int n) {
    int i;
    for (i = 0; i < n; i++) m->s[ids[i]].A = nyx_max(0.1f, m->s[ids[i]].A * 0.7f);
}

/* Un ciclo de pensamiento local: relaja lo que está en el foco. */
static void m_piensa(Mente *m) {
    int est[7], reg[200], nr, i;
    for (i = 0; i < m->nfoco; i++) est[i] = m->foco[i];
    nr = m_region(m, est, m->nfoco, reg, 160);
    for (i = 0; i < nr; i++) m_decae(m, reg[i], m->ruido);
    for (i = 0; i < nr; i++) m_propaga(m, reg[i]);
    for (i = 0; i < m->nobj; i++) m->s[m->obj[i]].A = nyx_min(1, m->s[m->obj[i]].A + 0.01f);
    m->ciclos++;
}

static void m_init(Mente *m, int rol) {
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

/* ------------------------------------------------------------------ */
/*  Consejo: las 18 juntas                                             */
/* ------------------------------------------------------------------ */

enum { EV_PIENSA, EV_DICE, EV_APRENDE, EV_IDEA, EV_RESH, EV_SUENA, EV_NOTA };
typedef void (*NyxEvento)(void *ud, int rol, int tipo, const char *texto);

typedef struct {
    int rol;
    char atractor[LARGO];
    float C, E, peso;
    int acuerdo;
} NyxVoto;

typedef struct {
    char frase[420];
    char resh[420];
    char ganador[LARGO];
    int vocero;
    float acuerdo;
    NyxVoto votos[NROLES];
    int nvotos;
} NyxRespuesta;

typedef struct {
    Mente m[NROLES];
    unsigned long tick;
    int ultimo;
    NyxEvento ev;
    void *ud;
    /* para 'bien' y 'mal': qué se dijo y quién lo pensó */
    int fbValido, fbN;
    int fbMente[NROLES], fbGan[NROLES], fbNest[NROLES];
    int fbEst[NROLES][MAXEST];
    int fbFrase[MAXFRASE], fbNfrase, fbVocero;
    char ecos[10][200]; int iecos;     /* lo último que se dijo (anti-eco) */
} Consejo;

static int c_es_eco(const Consejo *c, const char *es) {
    int i;
    for (i = 0; i < 10; i++) if (c->ecos[i][0] && strcmp(c->ecos[i], es) == 0) return 1;
    return 0;
}

static void c_pon_eco(Consejo *c, const char *es) {
    nyx_copia(c->ecos[c->iecos], es, sizeof c->ecos[0]);
    c->iecos = (c->iecos + 1) % 10;
}

static void c_evento(Consejo *c, int rol, int tipo, const char *texto) {
    if (c->ev) c->ev(c->ud, rol, tipo, texto);
}

static void consejo_init(Consejo *c, unsigned long long semilla, int conInfancia) {
    int r, i, ids[MAXEST];
    nyx_dic_init();
    nyx_semilla(semilla);
    memset(c, 0, sizeof *c);
    for (r = 0; r < NROLES; r++) m_init(&c->m[r], r);
    if (conInfancia)
        for (i = 0; i < NYX_NCORPUS; i++)
            for (r = 0; r < NROLES; r++) m_ingesta(&c->m[r], NYX_CORPUS[i], ids, MAXEST, 0);
    c->ultimo = -1;
}

/* Todas leen un texto (enseñar). Devuelve cuántas palabras nuevas hubo. */
static int consejo_lee(Consejo *c, const char *texto) {
    int r, ids[MAXEST], antes = c->m[0].n;
    for (r = 0; r < NROLES; r++) m_ingesta(&c->m[r], texto, ids, MAXEST, 0);
    return c->m[0].n - antes > 0 ? c->m[0].n - antes : 0;
}

/* Las demás oyen lo que dijo 'emisor'. Si el emisor sabía decir una palabra
 * en Resh y la oyente no, puede aprenderla. Devuelve la resonancia media. */
static float c_difunde(Consejo *c, int emisor, const int *frase, int nf, int *aprendieron, char *palabraAprendida) {
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
static int c_ya_dicho(const Mente *m, const char *es) {
    int i;
    for (i = 0; i < m->nven; i++)
        if (strncmp(m->ven[i], "dije: ", 6) == 0 && strcmp(m->ven[i] + 6, es) == 0) return 1;
    return 0;
}

/* Tema de esta mente ahora: lo más activo de su foco (o de sus objetivos). */
static int c_tema(const Mente *m) {
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
static int c_duda(const Mente *m) {
    int i, mejor = -1;
    float mA = 0.3f;
    for (i = 0; i < m->n; i++) {
        const Semion *s = &m->s[i];
        if (s->sabe || s->fusion || nyx_vacia(s->et) || !nyx_a_resh(s->et)) continue;
        if (s->A + 0.05f * (float)(s->usos > 10 ? 10 : s->usos) > mA) { mA = s->A; mejor = i; }
    }
    return mejor;
}

static void c_aprende(Consejo *c, int rol, int acc, float recompensa) {
    Mente *m = &c->m[rol];
    float antes = m->valor[acc];
    char t[128];
    m->valor[acc] += 0.25f * (recompensa - m->valor[acc]);
    m->veces[acc]++;
    snprintf(t, sizeof t, "'%s' ahora vale %.2f (%+.2f)", NYX_ACC[acc], m->valor[acc], m->valor[acc] - antes);
    c_evento(c, rol, EV_APRENDE, t);
}

static float c_puntaje(const Mente *m, int a) {
    float p = NYX_ROLES[m->rol].gusto[a] + m->valor[a] + 0.3f / (1.0f + (float)m->veces[a]);
    if (a == m->ultima) p -= 0.15f * (float)m->racha;   /* aburrimiento */
    return p + nyx_rango(-m->ruido * 2, m->ruido * 2);
}

/* Habla una mente: forma una frase desde 'centro', la dice, las demás la oyen. */
static float c_habla(Consejo *c, int rol, int centro, const char *prefijo) {
    Mente *m = &c->m[rol];
    int frase[MAXFRASE], nf, aprend;
    char es[400], resh[400], linea[900], palabra[LARGO];
    float res;
    {
        int ctx[7], i;
        for (i = 0; i < m->nfoco; i++) ctx[i] = m->foco[i];
        nf = m_frase(m, centro, frase, MAXFRASE, ctx, m->nfoco);
    }
    m_texto(m, frase, nf, es, sizeof es, resh, sizeof resh);
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
static void consejo_paso(Consejo *c) {
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
static void consejo_delibera(Consejo *c, const char *pregunta, NyxRespuesta *out) {
    int r, i, j, gan[NROLES];
    float peso[NROLES], C[NROLES], E[NROLES], mejor = -1;
    char etiq[NROLES][LARGO];
    float suma[NROLES];
    int nEtq = 0, ganE = -1;
    memset(out, 0, sizeof *out);
    c->fbValido = 1; c->fbN = 0;
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
        c->fbNfrase = m_frase(v, gan[out->vocero], c->fbFrase, MAXFRASE, c->fbEst[out->vocero], c->fbNest[out->vocero]);
        c->fbVocero = out->vocero;
        m_texto(v, c->fbFrase, c->fbNfrase, es, sizeof es, resh, sizeof resh);
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
static void consejo_habla_con(Consejo *c, int rol, const char *texto, char *es, size_t ces, char *resh, size_t cre, float *C) {
    Mente *m = &c->m[rol];
    int ids[MAXEST], k = m_recibe(m, texto, "humano", ids, MAXEST), g, cr;
    float E;
    es[0] = 0; resh[0] = 0; *C = 0;
    c->fbValido = 0;
    if (k == 0) { snprintf(es, ces, "(no entendí)"); return; }
    {   /* si iba a repetir lo que se acaba de decir, piensa otra cosa */
        int intento;
        for (intento = 0; intento < 4; intento++) {
            g = m_veredicto(m, ids, k, C, &E, &cr);
            if (g < 0) { snprintf(es, ces, "(no entendí)"); return; }
            c->fbNfrase = m_frase(m, g, c->fbFrase, MAXFRASE, ids, k);
            m_texto(m, c->fbFrase, c->fbNfrase, es, ces, resh, cre);
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
static int consejo_opina(Consejo *c, int bueno) {
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
    if (bueno) {     /* la frase dicha queda como buena secuencia */
        Mente *v = &c->m[c->fbVocero];
        for (x = 0; x + 1 < c->fbNfrase; x++) m_refuerza(v, c->fbFrase[x], c->fbFrase[x + 1], 0.1f, 0.3f, 0.2f);
    }
    c->fbValido = 0;
    return 1;
}

/* ------------------------------------------------------------------ */
/*  Memoria en disco (texto)                                           */
/* ------------------------------------------------------------------ */

static int consejo_guarda(const Consejo *c, const char *ruta) {
    FILE *f = fopen(ruta, "w");
    int r, i, k;
    if (!f) return 0;
    fprintf(f, "NYX 1 %lu\n", c->tick);
    for (r = 0; r < NROLES; r++) {
        const Mente *m = &c->m[r];
        fprintf(f, "MENTE %d %d %d %d %ld %ld %ld %ld %ld\n", r, m->n, m->aciertos, m->intentos,
                m->ciclos, m->dichos, m->ideas, m->cristales, m->reshAprendidas);
        fprintf(f, "VAL");
        for (i = 0; i < NACC; i++) fprintf(f, " %.4f %d", m->valor[i], m->veces[i]);
        fprintf(f, "\nOBJ %d", m->nobj);
        for (i = 0; i < m->nobj; i++) fprintf(f, " %d", m->obj[i]);
        fprintf(f, "\n");
        for (i = 0; i < m->n; i++) {
            const Semion *s = &m->s[i];
            fprintf(f, "S %s %.4f %.4f %.4f %d %d %d %u %u", s->et, s->A, s->fase, s->frec,
                    s->carga, s->fusion, s->sabe, s->usos, s->dialogo);
            for (k = 0; k < DIMF; k++) fprintf(f, " %.4f", s->f[k]);
            fprintf(f, " %d", s->nv);
            for (k = 0; k < s->nv; k++) fprintf(f, " %d %.4f %.4f %.4f", s->v[k].j, s->v[k].w, s->v[k].th, s->v[k].sec);
            fprintf(f, "\n");
        }
    }
    fprintf(f, "FIN\n");
    return fclose(f) == 0;
}

static int consejo_carga2(Consejo *c, Consejo *tmp0, FILE *f);

static int consejo_carga(Consejo *c, const char *ruta) {
    /* se carga aparte (en memoria pedida al momento): si el archivo está
     * mal, no se pierde nada */
    FILE *f = fopen(ruta, "r");
    Consejo *tmp;
    int ok;
    if (!f) return 0;
    tmp = (Consejo *)malloc(sizeof *tmp);
    if (!tmp) { fclose(f); return 0; }
    *tmp = *c;
    ok = consejo_carga2(c, tmp, f);
    free(tmp);
    return ok;
}

static int consejo_carga2(Consejo *c, Consejo *tmp0, FILE *f) {
    int r, i, k, ver;
    char cab[8];
    if (fscanf(f, "%7s %d %lu", cab, &ver, &tmp0->tick) != 3 || strcmp(cab, "NYX") != 0) { fclose(f); return 0; }
    for (r = 0; r < NROLES; r++) {
        Mente *m = &tmp0->m[r];
        int rr, n, nobj;
        if (fscanf(f, " MENTE %d %d %d %d %ld %ld %ld %ld %ld", &rr, &n, &m->aciertos, &m->intentos,
                   &m->ciclos, &m->dichos, &m->ideas, &m->cristales, &m->reshAprendidas) != 9 || rr != r || n < 0 || n > MAXSEM) { fclose(f); return 0; }
        if (fscanf(f, " VAL") != 0) { fclose(f); return 0; }
        for (i = 0; i < NACC; i++) if (fscanf(f, " %f %d", &m->valor[i], &m->veces[i]) != 2) { fclose(f); return 0; }
        if (fscanf(f, " OBJ %d", &nobj) != 1 || nobj < 0 || nobj > 8) { fclose(f); return 0; }
        m->nobj = nobj;
        for (i = 0; i < nobj; i++) { int x; if (fscanf(f, " %d", &x) != 1 || x < 0 || x >= n) { fclose(f); return 0; } m->obj[i] = (short)x; }
        m->n = n;
        for (i = 0; i < n; i++) {
            Semion *s = &m->s[i];
            int carga, fusion, sabe, nv;
            unsigned usos, dialogo;
            memset(s, 0, sizeof *s);
            if (fscanf(f, " S %27s %f %f %f %d %d %d %u %u", s->et, &s->A, &s->fase, &s->frec,
                       &carga, &fusion, &sabe, &usos, &dialogo) != 9) { fclose(f); return 0; }
            s->carga = (signed char)carga; s->fusion = (unsigned char)fusion; s->sabe = (unsigned char)sabe;
            s->usos = (unsigned short)usos; s->dialogo = (unsigned short)dialogo;
            for (k = 0; k < DIMF; k++) if (fscanf(f, " %f", &s->f[k]) != 1) { fclose(f); return 0; }
            if (fscanf(f, " %d", &nv) != 1 || nv < 0 || nv > MAXVEC) { fclose(f); return 0; }
            s->nv = (unsigned char)nv;
            for (k = 0; k < nv; k++) {
                int j;
                if (fscanf(f, " %d %f %f %f", &j, &s->v[k].w, &s->v[k].th, &s->v[k].sec) != 4 || j < 0 || j >= n) { fclose(f); return 0; }
                s->v[k].j = (short)j;
            }
        }
        m->nfoco = 0; m->nepis = 0; m->ncam = 0; m->nven = 0; m->iven = 0; m->nrec = 0;
        m->ultima = -1; m->racha = 0;
        m_hash_rehace(m);
    }
    fclose(f);
    *c = *tmp0;
    return 1;
}

/* Cuántas palabras sabe decir en Resh una mente. */
static int m_cuenta_resh(const Mente *m) {
    int i, n = 0;
    for (i = 0; i < m->n; i++) if (m->s[i].sabe) n++;
    return n;
}

static int m_mejor_accion(const Mente *m) {
    int a, mejor = 0;
    for (a = 1; a < NACC; a++)
        if (NYX_ROLES[m->rol].gusto[a] + m->valor[a] > NYX_ROLES[m->rol].gusto[mejor] + m->valor[mejor]) mejor = a;
    return mejor;
}

#endif


#define RUTA_MEMORIA "nyx_memoria.txt"

/* las 18 mentes viven en memoria pedida al arrancar (no en el programa):
 * así el runtime de WebAssembly de Code App no tiene que reservarla de golpe */
static Consejo *nyxp;
#define nyx (*nyxp)
static int conColor = 1;
static int verAprende = 1;

/* ------------------------------------------------------------------ */
/*  Colores y dibujo                                                   */
/* ------------------------------------------------------------------ */

static void color(int c) { if (conColor) printf("\033[38;5;%dm", c); }
static void negrita(void) { if (conColor) printf("\033[1m"); }
static void tenue(void) { if (conColor) printf("\033[2m"); }
static void normal(void) { if (conColor) printf("\033[0m"); }

static void nombre_rol(int r, int ancho) {
    color(NYX_ROLES[r].color);
    printf("%-*s", ancho, NYX_ROLES[r].nombre);
    normal();
}

static void barra(float x, float max, int ancho, int c) {
    int i, llenas = max > 0 ? (int)(x / max * (float)ancho + 0.5f) : 0;
    if (llenas < 0) llenas = 0;
    if (llenas > ancho) llenas = ancho;
    color(c);
    for (i = 0; i < llenas; i++) printf("█");
    tenue();
    for (; i < ancho; i++) printf("░");
    normal();
}

static void linea_caja(const char *izq, const char *der, int ancho) {
    int i;
    color(99);
    printf("%s", izq);
    for (i = 0; i < ancho; i++) printf("─");
    printf("%s\n", der);
    normal();
}

static int ancho_utf8(const char *s) {
    int n = 0;
    for (; *s; s++) if (((unsigned char)*s & 0xC0) != 0x80) n++;
    return n;
}

static void fila_caja(const char *texto, int ancho, int c) {
    int pad = ancho - 2 - ancho_utf8(texto);
    color(99); printf("│ "); normal();
    color(c); printf("%s", texto); normal();
    while (pad-- > 0) printf(" ");
    color(99); printf(" │\n"); normal();
}

static void banner(void) {
    const int A = 58;
    printf("\n");
    linea_caja("╭", "╮", A);
    negrita();
    fila_caja("N Y X  ·  cerebro resonante de 18 mentes", A, 219);
    fila_caja("cada una piensa, imagina, aprende y habla Resh", A, 117);
    fila_caja("escribe una pregunta, o  ayuda", A, 250);
    linea_caja("╰", "╯", A);
}

/* ------------------------------------------------------------------ */
/*  Lo que pasa dentro (eventos del cerebro)                           */
/* ------------------------------------------------------------------ */

static const char *icono(int tipo) {
    switch (tipo) {
    case EV_PIENSA: return "💭";
    case EV_DICE: return "▶ ";
    case EV_APRENDE: return "📚";
    case EV_IDEA: return "✨";
    case EV_RESH: return "📖";
    case EV_SUENA: return "🌙";
    default: return "· ";
    }
}

static void al_evento(void *ud, int rol, int tipo, const char *texto) {
    (void)ud;
    if (tipo == EV_APRENDE && !verAprende) return;
    tenue(); printf("%5lu ", nyx.tick); normal();
    nombre_rol(rol, 11);
    printf(" %s ", icono(tipo));
    if (tipo == EV_DICE) { negrita(); printf("%s", texto); normal(); }
    else if (tipo == EV_APRENDE || tipo == EV_NOTA) { tenue(); printf("%s", texto); normal(); }
    else printf("%s", texto);
    printf("\n");
    fflush(stdout);
}

/* ------------------------------------------------------------------ */
/*  Pantallas                                                          */
/* ------------------------------------------------------------------ */

static void panel_mentes(void) {
    int r;
    printf("\n");
    tenue(); printf("  %-11s %-17s %6s %6s %5s  %s\n", "mente", "precisión", "ideas", "Resh", "dijo", "lo que más le gusta"); normal();
    for (r = 0; r < NROLES; r++) {
        const Mente *m = &nyx.m[r];
        float p = m_precision(m);
        printf("  ");
        nombre_rol(r, 11);
        printf(" ");
        barra(p, 1, 10, NYX_ROLES[r].color);
        printf(" %3d%% %6d %6d %5ld  %s\n", (int)(p * 100 + 0.5f), m->n, m_cuenta_resh(m), m->dichos, NYX_ACC[m_mejor_accion(m)]);
    }
    tenue(); printf("  detalle: mente <nombre>\n\n"); normal();
}

static void detalle_mente(int r) {
    const Mente *m = &nyx.m[r];
    int i, a, orden[24], n = 0;
    float vmax = 0.01f;
    printf("\n  ");
    negrita(); nombre_rol(r, 0); normal();
    tenue(); printf("  —  quiere: %s\n", NYX_ROLES[r].objetivo); normal();
    printf("  precisión %d%% · %d ideas · sabe %d palabras en Resh (aprendió %ld de las demás)\n",
           (int)(m_precision(m) * 100 + 0.5f), m->n, m_cuenta_resh(m), m->reshAprendidas);
    printf("  dijo %ld cosas · imaginó %ld ideas · cristalizó %ld conceptos · %ld ciclos de pensamiento\n\n",
           m->dichos, m->ideas, m->cristales, m->ciclos);
    tenue(); printf("  lo que le gusta hacer (gusto de su rol + lo que aprendió):\n"); normal();
    for (a = 0; a < NACC; a++) {
        float v = NYX_ROLES[r].gusto[a] + m->valor[a];
        if (v > vmax) vmax = v;
    }
    for (a = 0; a < NACC; a++) {
        float v = NYX_ROLES[r].gusto[a] + m->valor[a];
        printf("    %-9s %5.2f ", NYX_ACC[a], v);
        barra(v > 0 ? v : 0, vmax, 20, NYX_ROLES[r].color);
        tenue(); printf("  (%d veces)\n", m->veces[a]); normal();
    }
    /* lo más activo de su mente */
    for (i = 0; i < m->n; i++) {
        int k, pos;
        if (m->s[i].fusion && m->s[i].dialogo == 0) continue;
        if (nyx_vacia(m->s[i].et)) continue;
        if (n < 12) pos = n++;                                   /* todavía hay sitio */
        else if (m->s[i].A > m->s[orden[11]].A) pos = 11;        /* entra en lugar del último */
        else continue;
        orden[pos] = i;
        for (k = pos; k > 0 && m->s[orden[k]].A > m->s[orden[k - 1]].A; k--) {
            int t = orden[k]; orden[k] = orden[k - 1]; orden[k - 1] = t;
        }
    }
    tenue(); printf("\n  lo más activo en su mente:\n"); normal();
    for (i = 0; i < n; i++) {
        const Semion *s = &m->s[orden[i]];
        const char *rr = s->sabe ? nyx_a_resh(s->et) : NULL;
        printf("    %-24s A=%.2f  C=%+.2f", s->et, s->A, m_coh(m, orden[i]));
        if (rr) { tenue(); printf("  resh: %s", rr); normal(); }
        printf("\n");
    }
    if (m->nfoco > 0) {
        tenue(); printf("\n  en su foco ahora: "); normal();
        for (i = m->nfoco - 1; i >= 0; i--) printf("%s%s", m->s[m->foco[i]].et, i ? " · " : "\n");
    }
    if (m->nven > 0) {
        tenue(); printf("\n  lo último que vivió:\n"); normal();
        for (i = m->nven < 5 ? m->nven : 5; i > 0; i--) {
            int k = (m->iven + NVEN - i) % NVEN;
            printf("    %s\n", m->ven[k]);
        }
    }
    printf("\n");
}

static void muestra_respuesta(const NyxRespuesta *r) {
    int i;
    printf("\n  ");
    color(219); negrita(); printf("nyx: "); normal();
    negrita(); printf("%s\n", r->frase); normal();
    printf("  "); tenue(); printf("resh: %s\n", r->resh); normal();
    printf("  "); tenue();
    printf("ganó «%s» · acuerdo %d%% · habló ", r->ganador, (int)(r->acuerdo * 100 + 0.5f));
    normal();
    if (r->vocero >= 0) nombre_rol(r->vocero, 0);
    printf("\n\n");
    for (i = 0; i < r->nvotos && i < 6; i++) {
        const NyxVoto *v = &r->votos[i];
        printf("   ");
        nombre_rol(v->rol, 11);
        printf(" %s %-18s ", v->acuerdo ? "✓" : " ", v->atractor);
        barra(v->peso, r->votos[0].peso > 0 ? r->votos[0].peso : 1, 12, NYX_ROLES[v->rol].color);
        tenue(); printf("  C=%+.2f E=%.2f\n", v->C, v->E); normal();
    }
    tenue(); printf("   (¿te gustó?  bien / mal)\n\n"); normal();
}

static void ayuda(void) {
    printf("\n");
    negrita(); printf("  Hablar\n"); normal();
    printf("    <pregunta>             las 18 deliberan y responde la que más convence\n");
    printf("    @<mente> <texto>       hablas con una sola (ej: @creativo el mar)\n");
    printf("    bien · mal             tu opinión de la última respuesta (las cambia)\n");
    negrita(); printf("  Verlas vivir (en tiempo real)\n"); normal();
    printf("    vivo [n]               n pasos de vida libre: imaginan, hablan y aprenden\n");
    printf("    conversa <a> <b> [n]   dos mentes conversan n turnos\n");
    printf("    piensa [n]             todas piensan en silencio n ciclos\n");
    negrita(); printf("  Enseñarles\n"); normal();
    printf("    enseña <texto>         las 18 lo leen y lo recuerdan\n");
    printf("    lee <archivo.txt>      leen un archivo entero, línea por línea\n");
    printf("    resh <palabra>         la palabra en Resh (y quién la sabe)\n");
    negrita(); printf("  Mirar por dentro\n"); normal();
    printf("    mentes                 las 18: precisión, ideas, Resh y gustos\n");
    printf("    mente <nombre>         una por dentro: gustos aprendidos, foco, recuerdos\n");
    printf("    eventos si|no          ver o no cuándo aprenden ('ahora vale…')\n");
    negrita(); printf("  Otros\n"); normal();
    printf("    guarda · carga         memoria en %s (se guarda sola al salir)\n", RUTA_MEMORIA);
    printf("    olvida                 empezar de cero (otra infancia)\n");
    printf("    color si|no · salir\n\n");
    tenue(); printf("  mentes: "); normal();
    {
        int r;
        for (r = 0; r < NROLES; r++) { nombre_rol(r, 0); printf(r + 1 < NROLES ? " " : "\n\n"); }
    }
}

/* ------------------------------------------------------------------ */
/*  Órdenes                                                            */
/* ------------------------------------------------------------------ */

static void quita_saltos(char *s) {
    size_t n = strlen(s);
    while (n > 0 && (s[n - 1] == '\n' || s[n - 1] == '\r' || s[n - 1] == ' ' || s[n - 1] == '\t')) s[--n] = 0;
}

static const char *salta_espacios(const char *s) {
    while (*s == ' ' || *s == '\t') s++;
    return s;
}

static int empieza(const char *s, const char *orden, const char **resto) {
    size_t n = strlen(orden);
    if (strncmp(s, orden, n) != 0) return 0;
    if (s[n] != 0 && s[n] != ' ') return 0;
    *resto = salta_espacios(s + n);
    return 1;
}

static int numero(const char *s, int defecto, int max) {
    int n = atoi(s);
    if (n <= 0) n = defecto;
    return n > max ? max : n;
}

static void guarda(int avisa) {
    if (consejo_guarda(&nyx, RUTA_MEMORIA)) {
        if (avisa) { tenue(); printf("  memoria guardada en %s\n", RUTA_MEMORIA); normal(); }
    } else {
        color(203); printf("  no pude guardar la memoria (¿la carpeta no deja escribir?)\n"); normal();
    }
}

static void vivo(int n) {
    int i;
    tenue(); printf("  %d pasos de vida libre — cada línea es lo que hace una mente\n\n", n); normal();
    for (i = 0; i < n; i++) consejo_paso(&nyx);
    printf("\n");
}

/* Dos mentes conversan: cada una responde a lo último que dijo la otra. */
static void conversa(int a, int b, int n) {
    char ultimo[400], es[400], resh[400], linea[900];
    int i;
    const Mente *ma = &nyx.m[a];
    int tema = c_tema(ma);
    nyx_copia(ultimo, tema >= 0 ? ma->s[tema].et : NYX_ROLES[a].objetivo, sizeof ultimo);
    tenue(); printf("  "); normal();
    nombre_rol(a, 0); tenue(); printf(" y "); normal(); nombre_rol(b, 0);
    tenue(); printf(" conversan sobre «%s»\n\n", ultimo); normal();
    for (i = 0; i < n; i++) {
        int quien = i % 2 == 0 ? a : b, otro = quien == a ? b : a, aprend = 0;
        float C;
        char palabra[LARGO];
        nyx.tick++;
        consejo_habla_con(&nyx, quien, ultimo, es, sizeof es, resh, sizeof resh, &C);
        snprintf(linea, sizeof linea, "%s%s   «%s»", i == 0 ? "" : (C < 0.2f ? "ye " : "se ko "), resh, es);
        al_evento(NULL, quien, EV_DICE, linea);
        /* la otra (y las que escuchan) lo oyen */
        if (nyx.fbNfrase > 0) c_difunde(&nyx, quien, nyx.fbFrase, nyx.fbNfrase, &aprend, palabra);
        if (aprend > 0) {
            snprintf(linea, sizeof linea, "%d mente%s aprendi%s «%s» = %s", aprend, aprend > 1 ? "s" : "",
                     aprend > 1 ? "eron" : "ó", palabra, nyx_a_resh(palabra));
            al_evento(NULL, quien, EV_RESH, linea);
        }
        nyx_copia(ultimo, es, sizeof ultimo);
        (void)otro;
    }
    nyx.fbValido = 0;
    printf("\n");
}

static void lee_archivo(const char *ruta) {
    FILE *f = fopen(ruta, "r");
    char l[1024];
    int lineas = 0, nuevas = 0;
    if (!f) { color(203); printf("  no encuentro %s\n", ruta); normal(); return; }
    while (fgets(l, sizeof l, f)) {
        quita_saltos(l);
        if (!*salta_espacios(l)) continue;
        nuevas += consejo_lee(&nyx, l);
        lineas++;
    }
    fclose(f);
    tenue(); printf("  las 18 leyeron %d líneas (%d palabras nuevas)\n", lineas, nuevas); normal();
}

static void resh(const char *palabra) {
    char tok[1][LARGO];
    const char *r, *e;
    int n = nyx_tokens(palabra, tok, 1), q, saben = 0;
    if (n == 0) { printf("  uso: resh <palabra>\n"); return; }
    r = nyx_a_resh(tok[0]);
    e = nyx_a_es(tok[0]);
    if (r) printf("  %s → %s\n", tok[0], r);
    if (e) printf("  %s ← %s (en español)\n", tok[0], e);
    if (!r && !e) { printf("  «%s» no está en el léxico Resh\n", tok[0]); return; }
    for (q = 0; q < NROLES; q++) {
        int id = m_busca(&nyx.m[q], r ? tok[0] : e);
        if (id >= 0 && nyx.m[q].s[id].sabe) saben++;
    }
    tenue(); printf("  la saben decir %d de las 18\n", saben); normal();
}

int main(void) {
    char l[1024];
    int ordenes = 0;
    setvbuf(stdout, NULL, _IONBF, 0);   /* que se vea todo al momento */
    printf("despertando a las 18 mentes…\n");
    nyxp = (Consejo *)calloc(1, sizeof *nyxp);
    if (!nyxp) {
        printf("no hay memoria para las 18 mentes (%lu MB)\n", (unsigned long)(sizeof *nyxp >> 20));
        return 1;
    }
    nyx.ev = al_evento;
    consejo_init(&nyx, (unsigned long long)time(NULL) * 2654435761ULL, 0);
    nyx.ev = al_evento;
    if (consejo_carga(&nyx, RUTA_MEMORIA)) {
        nyx.ev = al_evento;
        banner();
        tenue(); printf("  recordaron todo lo de la última vez (%s)\n\n", RUTA_MEMORIA); normal();
    } else {
        consejo_init(&nyx, (unsigned long long)time(NULL) * 2654435761ULL, 1);
        nyx.ev = al_evento;
        banner();
        tenue(); printf("  nacieron y leyeron su infancia (%d frases)\n\n", NYX_NCORPUS); normal();
    }
    for (;;) {
        const char *s, *resto;
        color(219); negrita(); printf("nyx› "); normal();
        fflush(stdout);
        if (!fgets(l, sizeof l, stdin)) break;
        quita_saltos(l);
        s = salta_espacios(l);
        if (!*s) continue;
        ordenes++;
        if (strcmp(s, "salir") == 0 || strcmp(s, "exit") == 0 || strcmp(s, "q") == 0) break;
        else if (strcmp(s, "ayuda") == 0 || strcmp(s, "help") == 0 || strcmp(s, "?") == 0) ayuda();
        else if (strcmp(s, "mentes") == 0) panel_mentes();
        else if (empieza(s, "mente", &resto)) {
            int r = nyx_rol_de(resto);
            if (r < 0) printf("  ¿qué mente? (escribe: mentes)\n"); else detalle_mente(r);
        }
        else if (empieza(s, "vivo", &resto)) { vivo(numero(resto, 20, 2000)); guarda(0); }
        else if (empieza(s, "conversa", &resto)) {
            char a[32] = "", b[32] = "";
            int n = 6, r1, r2;
            if (sscanf(resto, "%31s %31s %d", a, b, &n) < 2) { printf("  uso: conversa <mente> <mente> [turnos]\n"); continue; }
            r1 = nyx_rol_de(a); r2 = nyx_rol_de(b);
            if (r1 < 0 || r2 < 0 || r1 == r2) { printf("  ¿qué mentes? (escribe: mentes)\n"); continue; }
            conversa(r1, r2, n < 1 ? 1 : (n > 200 ? 200 : n));
        }
        else if (empieza(s, "piensa", &resto)) {
            int n = numero(resto, 50, 5000), i, r;
            for (i = 0; i < n; i++) for (r = 0; r < NROLES; r++) m_piensa(&nyx.m[r]);
            tenue(); printf("  pensaron %d ciclos en silencio\n", n); normal();
        }
        else if (empieza(s, "enseña", &resto) || empieza(s, "ensena", &resto)) {
            if (!*resto) { printf("  uso: enseña <texto>\n"); continue; }
            tenue(); printf("  las 18 lo leyeron (%d palabras nuevas)\n", consejo_lee(&nyx, resto)); normal();
        }
        else if (empieza(s, "lee", &resto)) lee_archivo(resto);
        else if (empieza(s, "resh", &resto)) resh(resto);
        else if (strcmp(s, "bien") == 0 || strcmp(s, "mal") == 0) {
            int bueno = s[0] == 'b';
            if (consejo_opina(&nyx, bueno)) {
                tenue();
                printf(bueno ? "  lo reforzaron: esa idea y esa frase pesan más ahora\n"
                             : "  lo inhibieron: esa respuesta queda en oposición de fase y perderá la próxima vez\n");
                normal();
            } else printf("  no hay respuesta reciente que juzgar\n");
        }
        else if (empieza(s, "eventos", &resto)) { verAprende = strcmp(resto, "no") != 0; }
        else if (empieza(s, "color", &resto)) { conColor = strcmp(resto, "no") != 0; }
        else if (strcmp(s, "guarda") == 0) guarda(1);
        else if (strcmp(s, "carga") == 0) {
            if (consejo_carga(&nyx, RUTA_MEMORIA)) { nyx.ev = al_evento; printf("  memoria cargada\n"); }
            else printf("  no hay memoria guardada (o está dañada)\n");
        }
        else if (strcmp(s, "olvida") == 0) {
            consejo_init(&nyx, (unsigned long long)time(NULL) * 2654435761ULL, 1);
            nyx.ev = al_evento;
            printf("  empezaron de cero (otra infancia)\n");
        }
        else if (s[0] == '@') {
            char nom[32] = "", es[400], rs[400];
            float C;
            int r, k = 0;
            const char *p = s + 1;
            while (*p && *p != ' ' && k < 31) nom[k++] = *p++;
            nom[k] = 0;
            r = nyx_rol_de(nom);
            if (r < 0) { printf("  ¿qué mente? (escribe: mentes)\n"); continue; }
            p = salta_espacios(p);
            if (!*p) { printf("  uso: @%s <texto>\n", nom); continue; }
            consejo_habla_con(&nyx, r, p, es, sizeof es, rs, sizeof rs, &C);
            printf("\n  "); nombre_rol(r, 0); printf(": "); negrita(); printf("%s\n", es); normal();
            printf("  "); tenue(); printf("resh: %s%s   (C=%+.2f · ¿bien / mal?)\n\n", C < 0.2f ? "ye " : "mi ko ", rs, C); normal();
        }
        else {
            NyxRespuesta r;
            consejo_delibera(&nyx, s, &r);
            muestra_respuesta(&r);
        }
        if (ordenes % 15 == 0) guarda(0);
    }
    guarda(1);
    printf("  hasta luego\n");
    return 0;
}
