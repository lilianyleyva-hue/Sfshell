# NyxCerebro — las 18 IAs en Swift Playgrounds (app nueva)

App nueva, sin nada de la shell. Solo la IA: el cerebro resonante de las 18
mentes, con memoria de frases, lengua Resh e imaginación que aprende.
Su conocimiento se guarda con **SwiftData**, y lo que una mente aprende se lo
**pasa a las demás** como registros de SwiftData (Transferencias).

Entre ellas se hablan en Resh: cada una solo entiende las palabras Resh que sabe; las demás las pregunta ("ye kivr?" → "kivr ka azul").

Requisitos: iPadOS 17 o más nuevo · Swift Playgrounds con Swift 5.9.
La primera vez que uses 🎤 te pedirá permiso para el micrófono y el reconocimiento de voz, y la primera vez que enciendas la cámara, para la cámara.

## Abrirla
En Archivos, toca `NyxCerebro.swiftpm`. Se abre en Swift Playgrounds. ▶

## Pestañas
- **Hablar:** pregúntales (a las 18 o a una), en español o en Resh. Contestan en Resh; el interruptor "Traducir al español" muestra u oculta la traducción. 👍/👎 cambia lo que aprendieron.
- **En vivo:** ▶ Empezar y míralas imaginar, hablar, enseñarse Resh y pasarse conocimiento.
- **Sentidos:** 📷 enciende la cámara y miran el mundo cada pocos segundos (tú eliges cada cuánto); 🎬 mándales un video y lo ven ENTERO (un fotograma por segundo, cada escena con su momento, y escuchan todo el audio en el iPad); enséñales una foto (la ven con Vision: qué hay, texto, caras, colores) o háblales con el micrófono (te entienden con el reconocimiento de voz de Apple y notan si suena fuerte/suave, agudo/grave). Cada una se fija en algo distinto.
- **Consejo lógico:** 18 agentes que razonan con hechos (silogismo, herencia, negación, causas, comparaciones, contradicciones, inducción, abducción, aritmética…). Pregúntale «¿es la ballena un pez?» y contesta sí/no/no sé con la cadena de pasos. Enséñale hechos («el delfín es un mamífero»). Puede leer todo lo que sabe el consejo nuevo y sacar hechos de ahí.
- **🌌 Nyx (pestaña principal): una sola mente con lo mejor de todas.** Una sola memoria y cinco formas de pensar: ⚖️ razonamiento (del consejo lógico: hechos, reglas, pruebas, cuentas), ✨ asociación (las 18 mentes resonantes del consejo nuevo), 🎯 estrategias (lo que las 18 aprendieron en las misiones), 🔎 búsqueda (la frase recordada que mejor responde) y 📌 recuerdo (lo que ya le confirmaste). Del consejo antiguo toma sus ideas: el crítico que descarta la respuesta que contradice lo que se sabe, debatir más cuando está reñido y calibrar de quién fiarse. Mejoras: aprende en qué clase de pregunta acierta cada forma de pensar, verifica las respuestas por otro camino, comprueba que la respuesta hable de lo preguntado, piensa más rondas cuando duda, dice «no lo sé» cuando no lo sabe, aprende de tus correcciones («no, es azul»), se entrena sola (🎓) y consolida lo que sabe (💤). Lo que le enseñas lo aprenden a la vez su lógica y su asociación: «el quokka es un marsupial» + «los marsupiales son mamíferos» → «¿tiene pelo el quokka?» → «sí, porque quokka es marsupial ⇒ marsupial es mamífero ⇒ mamífero tiene pelo». En la prueba acertó 209 de 210 preguntas (el consejo nuevo solo, ~70 %; el lógico solo, ~77 %).
- **Los tres (chat):** el consejo nuevo ✨, el antiguo 🏛 y el lógico ⚖️ hablan entre sí en Resh, **a su ritmo**: habla el que más ganas tiene (las ganas suben si le preguntan o si oye algo que entiende, y bajan cuando acaba de hablar), las frases largas tardan más y el antiguo es más lento. No hay límite de turnos: siguen mientras tengan algo nuevo que decir y **se callan solos** cuando se aburren (o tú pulsas «Que se callen»). Tú puedes escribir en el chat cuando quieras (en español o en Resh) y te contestan.
- **Misiones 🎯:** retos para los tres: cuentas ➗, series 🔢, ecuaciones ✖️, problemas 🍎, silogismos 🧩, «¿quién es más alto?» 📏 y «¿cuál sobra?» 🔍. Cada consejo contesta a su manera: el lógico razona y explica; en el nuevo cada una de las 18 mentes elige una estrategia (calcular con cuidado, probar, ir rápido, estimar, recordar, adivinar), votan, y luego **aprenden qué estrategia les funciona** en cada tipo; el antiguo delibera. Hay marcador, y cuanto más aciertan, más difícil (nivel 1 a 5). Cuando se aburren, el lógico les propone una misión él solo. También puedes escribir tu propia misión en el chat: «misión: 3, 6, 12, 24, ?», «misión: 4x + 2 = 30», «misión: ¿cuál no es un animal: gato, mesa, perro?»… Si nadie sabe la respuesta, te la preguntan.
- **Consejo antiguo:** el consejo original que hablaba Resh (tal cual, sin terminal). Pregúntale, o pulsa "Que hablen" y los dos consejos conversan en Resh y se enseñan palabras.
- **Mentes:** las 18, y cada una por dentro (qué aprendió a preferir, su foco, sus recuerdos).
- **Transferencias:** los registros de SwiftData que se pasan entre ellas.
- **Enseñar:** pega frases y las 18 las aprenden.

## Archivos (carpeta Nyx)
| archivo | qué es |
|---|---|
| NyxApp.swift | aquí empieza la app |
| Pantallas.swift | la interfaz |
| NyxModelo.swift | une el cerebro con la pantalla y SwiftData |
| AlmacenSwiftData.swift | @Model SaberMente y Transferencia |
| Consejo.swift | las 18 juntas: imaginan, deliberan, conversan |
| Mente.swift | una mente: resonancia, veredicto, frases |
| Frases.swift | memoria de frases enteras |
| Buzon.swift | cómo se pasan conocimiento |
| Memoria.swift | conocimiento como texto (para SwiftData y respaldo) |
| Sentidos.swift | cómo percibe cada mente lo que ve y oye |
| Ojos.swift | ver fotos (Vision) |
| Oidos.swift | oír (micrófono + reconocimiento de voz) |
| Camara.swift | la cámara en vivo |
| Video.swift | ver y oír videos |
| Comprension.swift | hablar entre ellas en Resh |
| VistaSentidos.swift | la pestaña Sentidos |
| Logica.swift | el consejo lógico: hechos, reglas y los 18 agentes |
| VistaLogica.swift | la pestaña Consejo lógico |
| Grande.swift | enteros exactos de cualquier tamaño (sumas de 100 cifras, 2 elevado a 100…) |
| NyxUno.swift | Nyx Uno: la mente unida (decide, critica, verifica, aprende de quién fiarse) |
| VistaUno.swift | la pestaña Nyx |
| Asamblea.swift | los tres consejos hablando entre sí (a su ritmo) y enfrentando misiones |
| Misiones.swift | las misiones: inventarlas y resolverlas |
| SaberMisiones.swift | lo que aprenden de las misiones (estrategias, marcador) |
| VistaAsamblea.swift | la pestaña Los tres |
| Puente.swift | el puente entre el consejo antiguo y el nuevo |
| VistaAntiguo.swift | la pestaña Consejo antiguo |
| Antiguo/ | el consejo original (CerebroResonante, ConsejoResonante, LenguaResh) |
| Base.swift | azar, las 18, su infancia |
| ReshDatos.swift | las 3144 palabras Resh |

## Cómo piensan (mejoras del cerebro)
- **Activación que se propaga:** la pregunta activa sus palabras y la activación viaja 3 pasos por las conexiones; un acople inhibido resta.
- **Las pistas raras pesan más:** "lluvia" dice más que "hace"; los verbos que van con todo (hace, tiene, da…) cuentan poco.
- **La frase se elige por toda la pregunta**, con raíces ("brilla" = "brillan").
- **Contexto de la conversación:** "háblame del árbol" → "¿y cómo crece?" sigue en el árbol.
- Examen de 160 preguntas con respuesta conocida: del 63 % al ~90 % de aciertos.
