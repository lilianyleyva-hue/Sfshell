package lengua

import "strings"

// Word tables used by Lema and by the frame and intent readers.

func conjunto(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

// funcionales are left as they are by Lema: articles, prepositions, pronouns, conjunctions, adverbs,
// forms of "ser"/"estar"/"haber", quantifiers, and words that end in -s but are not plurals.
var funcionales = conjunto(`
a al ante bajo con contra de del desde en entre hacia hasta para por segun sin sobre tras
el la lo los las un una unos unas y e o u ni que quien quienes cual cuales cuanto cuanta cuantos cuantas
como donde cuando si no mas menos muy tan tanto ya aun tambien tampoco siempre nunca jamas todavia solo
bien mal asi aqui alli ahi hoy ayer ahora luego despues antes entonces pues porque aunque pero sino mientras
me te se nos os le les mi mis tu tus su sus nuestro nuestra nuestros nuestras yo ella ellos ellas usted ustedes
nosotros vosotros esto eso aquello este esta estos estas ese esa esos esas aquel aquella aquellos aquellas
es son era eran sea sean fue fueron ser sera serian seria estan estar estoy estamos hay habia ha han he
otro otra otros otras cada algo nada alguien nadie mismo misma mismos mismas todos todas algunos algunas
ningun ninguno ninguna ningunos ningunas cualquier cualquiera varios varias muchos muchas pocos pocas
dos tres seis lunes martes miercoles jueves viernes virus crisis analisis tesis bus atlas paraguas gratis
ademas casi incluso dentro fuera cerca lejos encima debajo delante detras tal tales sí vs etc
`)

// verbosConocidos are infinitives that Lema can return for conjugated forms.
var verbosConocidos = conjunto(`
sumar restar multiplicar dividir contar filtrar transformar convertir ordenar invertir devolver calcular
obtener sacar hallar encontrar buscar decir mostrar imprimir escribir leer pedir preguntar hacer crear
generar construir programar necesitar querer poder saber tener dar regalar prestar comprar ganar recibir
perder gastar comer vender romper llover mojar estar haber ir venir salir entrar meter poner quitar
eliminar borrar agregar añadir insertar reemplazar sustituir cambiar repetir unir juntar concatenar
separar partir comprobar verificar revisar probar ejecutar correr lanzar explicar entender arreglar
reparar corregir fallar compilar funcionar recordar aprender olvidar significar resolver factorizar
descomponer elegir escoger colocar sentar empezar comenzar terminar acabar contener incluir aparecer
quedar seleccionar recorrer elevar redondear comparar intercambiar aplanar transponer acumular capitalizar
recortar tomar coger usar servir llamar pasar retornar mover cruzar llenar vaciar verter llegar viajar
tardar costar pagar ahorrar cobrar repartir compartir volar nadar ladrar vivir morir jugar mentir
trabajar estudiar mirar ver oir abrir cerrar valer medir pesar durar faltar sobrar doblar triplicar
duplicar aumentar disminuir rebajar descontar subir bajar crecer llevar traer enviar mandar deber gustar
preferir pensar creer parecer conocer nacer cumplir pintar colorear despejar simplificar evaluar dibujar
contestar responder indicar determinar averiguar adivinar intentar seguir dejar volver mantener detener
caer oler pensar soler sentir dormir encender apagar esperar pagar recoger conseguir encontrar repasar
expresar calcular comentar documentar optimizar acelerar mejorar validar analizar listar enumerar
definir declarar inicializar asignar incrementar decrementar iterar recorrer leer guardar cargar copiar
descargar subir instalar importar exportar formatear depurar traducir decidir resultar equivaler
sustraer adicionar duplicar pedir rellenar completar marcar tachar sonar aprobar suspender
`)

// irregulares maps forms that the suffix rules cannot undo to their infinitive.
var irregulares = map[string]string{
	"haz": "hacer", "haga": "hacer", "hagas": "hacer", "hagan": "hacer", "hizo": "hacer", "hice": "hacer",
	"hecho": "hacer", "hago": "hacer", "haria": "hacer", "hara": "hacer", "hagamos": "hacer",
	"di": "decir", "diga": "decir", "digas": "decir", "digan": "decir", "dice": "decir", "dicen": "decir",
	"dijo": "decir", "dicho": "decir", "digo": "decir", "dime": "decir", "dira": "decir", "diria": "decir",
	"pon": "poner", "ponga": "poner", "pongas": "poner", "pongan": "poner", "puso": "poner", "puesto": "poner",
	"pongo": "poner", "ponle": "poner", "pondra": "poner",
	"ten": "tener", "tenga": "tener", "tengas": "tener", "tengan": "tener", "tuvo": "tener", "tengo": "tener",
	"tendra": "tener", "tendria": "tener", "tenia": "tener", "tenian": "tener",
	"sal": "salir", "salga": "salir", "salgan": "salir", "salgo": "salir", "saldra": "salir",
	"ven": "venir", "venga": "venir", "vengan": "venir", "vino": "venir", "vengo": "venir",
	"va": "ir", "van": "ir", "vaya": "ir", "vayan": "ir", "voy": "ir", "vamos": "ir", "iba": "ir", "iban": "ir",
	"da": "dar", "dan": "dar", "doy": "dar", "dio": "dar", "dieron": "dar", "dame": "dar", "dale": "dar", "des": "dar",
	"sepa": "saber", "sepan": "saber", "supo": "saber", "sabes": "saber", "sabe": "saber", "sabia": "saber",
	"pueda": "poder", "puedan": "poder", "pudo": "poder", "puedo": "poder", "puedes": "poder", "podria": "poder",
	"quiero": "querer", "quieres": "querer", "quiere": "querer", "quieren": "querer", "quiso": "querer",
	"querria": "querer", "quisiera": "querer",
	"veo": "ver", "ves": "ver", "ve": "ver", "vea": "ver", "vean": "ver", "vio": "ver", "visto": "ver",
	"oye": "oir", "oiga": "oir", "oigo": "oir", "oyen": "oir",
	"juega": "jugar", "juegan": "jugar", "juegue": "jugar", "jueguen": "jugar", "juego": "jugar",
	"huele": "oler", "huelen": "oler",
	"traiga": "traer", "traigan": "traer", "traigo": "traer", "trajo": "traer",
	"conozca": "conocer", "conozco": "conocer", "parezca": "parecer", "parezco": "parecer",
	"nazca": "nacer", "nazco": "nacer", "produzca": "producir", "traduzca": "traducir", "traduce": "traducir",
	"caiga": "caer", "caigo": "caer", "cayo": "caer",
	"lea": "leer", "lean": "leer", "leyo": "leer", "leido": "leer", "leyendo": "leer",
	"pida": "pedir", "pidan": "pedir", "pide": "pedir", "piden": "pedir", "pidio": "pedir", "pido": "pedir",
	"sigue": "seguir", "siga": "seguir", "sigan": "seguir", "sigo": "seguir", "siguen": "seguir",
	"elige": "elegir", "elija": "elegir", "elijan": "elegir", "elijo": "elegir",
	"escoja": "escoger", "escojo": "escoger", "coja": "coger", "cojo": "coger",
	"devuelva": "devolver", "devuelvan": "devolver", "devuelve": "devolver", "devuelven": "devolver",
	"devuelvas": "devolver", "devuelto": "devolver",
	"vuelve": "volver", "vuelva": "volver", "vuelto": "volver",
	"muestre": "mostrar", "muestren": "mostrar", "muestra": "mostrar", "muestran": "mostrar", "muestrame": "mostrar",
	"cuente": "contar", "cuenten": "contar", "cuenta": "contar", "cuentan": "contar", "cuento": "contar",
	"encuentre": "encontrar", "encuentra": "encontrar", "encuentran": "encontrar", "encuentren": "encontrar",
	"prueba": "probar", "pruebe": "probar", "prueben": "probar", "pruebala": "probar", "pruebalo": "probar",
	"cuesta": "costar", "cuestan": "costar", "costo": "costar",
	"invierta": "invertir", "inviertan": "invertir", "invierte": "invertir", "invierten": "invertir", "invirtio": "invertir",
	"llueve": "llover", "llovio": "llover", "lloviendo": "llover", "llueva": "llover",
	"piensa": "pensar", "pienso": "pensar", "empieza": "empezar", "empiezan": "empezar", "empiece": "empezar",
	"empiecen": "empezar", "comienza": "comenzar", "comienzan": "comenzar", "comience": "comenzar", "comiencen": "comenzar",
	"cierra": "cerrar", "repite": "repetir", "repita": "repetir", "repitan": "repetir", "repiten": "repetir",
	"mide": "medir", "miden": "medir", "mida": "medir", "sirve": "servir", "sirven": "servir", "sirva": "servir",
	"muere": "morir", "murio": "morir", "muerto": "morir", "duerme": "dormir", "durmio": "dormir",
	"contiene": "contener", "contienen": "contener", "contenga": "contener", "contengan": "contener",
	"obtiene": "obtener", "obtenga": "obtener", "obtengo": "obtener", "mantiene": "mantener", "detiene": "detener",
	"construye": "construir", "construya": "construir", "incluye": "incluir", "incluya": "incluir", "incluyan": "incluir",
	"resuelve": "resolver", "resuelva": "resolver", "resuelto": "resolver", "resuelvo": "resolver",
	"descompon": "descomponer", "descomponga": "descomponer",
	"escrito": "escribir", "abierto": "abrir", "roto": "romper", "rompio": "romper",
	"consiga": "conseguir", "consigue": "conseguir", "consiguio": "conseguir",
	"pierde": "perder", "pierden": "perder", "pierda": "perder", "perdio": "perder",
	"quiebra": "quebrar", "siente": "sentir", "sienta": "sentir", "prefiere": "preferir",
	"corrige": "corregir", "corrija": "corregir", "corrijan": "corregir", "corrigelo": "corregir",
	"explicalo": "explicar", "explicamelo": "explicar", "explicame": "explicar",
	"hazlo": "hacer", "hazla": "hacer", "hazme": "hacer",
	"ejecutalo": "ejecutar", "ejecutala": "ejecutar", "correlo": "correr",
	"arreglalo": "arreglar", "arreglame": "arreglar",
	"suelta": "soltar", "suena": "sonar",
}

// comunes are frequent words that are neither in the lexicon nor verbs; Lema keeps them and
// PalabrasDesconocidas does not report them.
var comunes = conjunto(`
forma manera ejemplo favor codigo caso cosa persona gente dia año mes semana minuto kilometro metro
kilo gramo litro euro dolar precio coste parte resultado respuesta pregunta problema ecuacion sistema
incognita formula grupo equipo amigo hermano padre madre hijo hija niño niña hombre mujer perro gato
caramelo canica manzana libro lapiz moneda cara cruz dado carta bola caja bolsa tren coche velocidad
distancia vez veces solucion verdad mentira caballero bribon reina sudoku puzle puzzle jarra lobo cabra
col disco torre laberinto color hola adios gracias mundo ia inteligencia lenguaje modelo go golang
mitad siguiente anterior consecutivo cierto dicho hora horas cuantas ultima primera
`)
