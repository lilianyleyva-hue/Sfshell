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
- **Sentidos:** 📷 enciende la cámara y miran el mundo cada pocos segundos (tú eliges cada cuánto); 🎬 mándales un video (miran 5 momentos y escuchan lo que se dice); enséñales una foto (la ven con Vision: qué hay, texto, caras, colores) o háblales con el micrófono (te entienden con el reconocimiento de voz de Apple y notan si suena fuerte/suave, agudo/grave). Cada una se fija en algo distinto.
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
| Base.swift | azar, las 18, su infancia |
| ReshDatos.swift | las 3144 palabras Resh |
