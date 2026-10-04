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
- **Núcleo de Nyx 2** (la misma arquitectura, mejorada): la memoria de frases pasó de 1 500 a 40 000 frases y tiene un índice (como el de un libro), así que buscar ya no recorre todo; la lógica también tiene índice por sujeto y por relación (deducir es 10 veces más rápido), y una palabra entre comillas («motores») se guarda tal cual, sin pasarla a singular. Medido: con las 7000 palabras leídas, «¿qué es X?» pasó de 25 % a 95 % de aciertos, y cada pregunta de ~1 100 ms a ~100 ms.
- **💾 Guardar en archivo / 📂 Cargar archivo** (pestaña Nyx): «💾» mete todo lo que sabe (ideas, frases, hechos, misiones y de quién se fía) en un archivo de texto y te pregunta dónde guardarlo (por ejemplo en Archivos → En mi iPad o iCloud Drive). «📂» abre ese archivo y lo recuerda todo. Así no depende de que la app guarde sola: antes de salir pulsa 💾, y al volver pulsa 📂.
- **Nyx 2 piensa en voz alta (modelo de pensamiento):** antes de contestar razona paso a paso y lo puedes ver (interruptor «💭 Ver cómo piensa»): 🧩 entiende la pregunta, 📖 recuerda lo que sabe de cada palabra, 💡 cada forma de pensar propone, comprueba, 🤔 si duda se pregunta lo mismo de otra forma o deduce más, y ✅ concluye con su confianza. Ve cuándo dos partes dicen lo mismo con otras palabras.
- **🌈 Imaginación:** «imagina un pez que vuela», «¿qué pasaría si llueve?», o el botón «🌈 Imaginar» para que imagine sola. Mezcla lo que sabe de verdad de cada cosa (el pez tiene escamas; para volar toma algo del pájaro: plumas) y avisa cuando en la realidad no es posible. Lo imaginado se dice como imaginado y no se guarda como hecho.
- **Memoria 9 veces mayor:** cada una de las 18 mentes guarda hasta 8000 ideas (antes 900). La memoria se carga en segundo plano al abrir la app («🧠 recordando…»: mientras tanto no aprende nada, para no pisarla) y se guarda en segundo plano cada 20 s si aprendió algo, al leer archivos y al salir de la app, con una copia de seguridad en un archivo. Arriba en la pestaña Nyx se ve si recordó todo y cuándo guardó por última vez.
- **🌌 Nyx (pestaña principal): una sola mente con lo mejor de todas.** Una sola memoria y cinco formas de pensar: ⚖️ razonamiento (del consejo lógico: hechos, reglas, pruebas, cuentas), ✨ asociación (las 18 mentes resonantes del consejo nuevo), 🎯 estrategias (lo que las 18 aprendieron en las misiones), 🔎 búsqueda (la frase recordada que mejor responde) y 📌 recuerdo (lo que ya le confirmaste). Del consejo antiguo toma sus ideas: el crítico que descarta la respuesta que contradice lo que se sabe, debatir más cuando está reñido y calibrar de quién fiarse. Mejoras: aprende en qué clase de pregunta acierta cada forma de pensar, verifica las respuestas por otro camino, comprueba que la respuesta hable de lo preguntado, piensa más rondas cuando duda, dice «no lo sé» cuando no lo sabe, aprende de tus correcciones («no, es azul»), se entrena sola (🎓) y consolida lo que sabe (💤). Lo que le enseñas lo aprenden a la vez su lógica y su asociación: «el quokka es un marsupial» + «los marsupiales son mamíferos» → «¿tiene pelo el quokka?» → «sí, porque quokka es marsupial ⇒ marsupial es mamífero ⇒ mamífero tiene pelo». En la prueba acertó 209 de 210 preguntas (el consejo nuevo solo, ~70 %; el lógico solo, ~77 %).
- **📎 Mandarle cosas (sin límite):** en la pestaña Nyx, «🖼 Fotos y videos» (elige las que quieras), «📄 Archivos» (texto, PDF, páginas web, fotos, videos, audios…) y «🎵 Audios». Los enlaces (https://…) se pegan en la caja y los descarga y lee solos (páginas, fotos, PDF, audio o video). Todo va a una cola y lo lee uno tras otro: mira las fotos, ve los videos enteros, pasa los audios a texto en el propio iPad, y lo que lee lo aprenden su memoria y su lógica. Un libro entero (el Quijote, 13 000 frases) tarda unos segundos; su memoria de frases guarda las 1500 más valiosas, y los hechos de la lógica no tienen tope.
- **🌐 Aprender de internet:** en la pestaña Nyx, escribe un tema («delfín») y pulsa 🌐. Lee el resumen de Wikipedia en español, se queda con las frases útiles y las aprende con su memoria y su lógica (con menos confianza que lo que le enseñas tú). La pestaña Enseñar ahora acepta texto en Resh (ver Textos/Resh_palabras_para_comunicar.md).
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
| Adjuntos.swift | mandarle fotos, videos, audios, archivos y enlaces (sin límite: van en cola) |
| Internet.swift | aprender de Wikipedia en español (botón 🌐 en la pestaña Nyx) |
| Imaginacion.swift | la imaginación de Nyx (combinar lo que sabe, «¿qué pasaría si…?») |
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
