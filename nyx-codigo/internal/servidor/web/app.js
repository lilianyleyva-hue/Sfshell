/*
 * Nyx Código — la página (ES2017, sin dependencias, sin módulos externos).
 *
 * Reglas de seguridad: los datos del servidor NUNCA se insertan con innerHTML; solo con textContent
 * y createElement. Los enlaces solo se crean para direcciones http(s). La clave de la sesión llega en
 * el fragmento (#t=…), se guarda en sessionStorage y se borra de la barra de direcciones.
 */
(function () {
  'use strict';

  // ---------------------------------------------------------------------------------------------
  // Textos fijos de la interfaz (una sola tabla para poder revisarlos).
  // ---------------------------------------------------------------------------------------------
  var TEXTOS = {
    escribe: 'Escribe tu pregunta…',
    pensar: 'Pensar',
    pensarMas: 'Pensar más',
    parar: 'Parar',
    pegarCodigo: 'Pegar código',
    comoLoPienso: 'Cómo lo pienso',
    codigo: 'Código',
    pruebas: 'Pruebas',
    memoria: 'Memoria',
    estudiar: 'Estudiar',
    copiar: 'Copiar',
    copiado: 'Copiado',
    ejecutar: 'Ejecutar',
    probar: 'Probar',
    explicar: 'Explicar',
    arreglar: 'Arreglar',
    descargar: 'Descargar .go',
    estaBien: '¿Está bien?',
    si: 'Sí',
    no: 'No',
    noEsEso: 'No es eso',
    otraCosa: 'Otra cosa',
    gracias: 'Gracias, lo tendré en cuenta',
    sinConexion: 'Sin conexión con Nyx Código: ¿sigue abierto el programa?',
    pensando: 'Pensando…',

    irAEscribir: 'Ir a escribir la pregunta',
    conversacion: 'Conversación',
    modo: 'Modo',
    modoAuto: 'Automático',
    modoProgramar: 'Programar',
    modoArreglar: 'Arreglar',
    modoExplicar: 'Explicar',
    modoMates: 'Matemáticas',
    modoLogica: 'Lógica',
    modoPuzles: 'Puzles',
    modoBuscar: 'Buscar',
    codigoAdjunto: 'Código que acompaña a la pregunta',
    pistaAtajo: 'Ctrl+Enter envía la pregunta.',
    sinTokenTitulo: 'No puedo hablar con Nyx Código',
    sinToken: 'Abre Nyx Código desde su icono o escribe nyx-codigo en una terminal.',
    hola: '¡Hola! Pídeme una función, pega código para arreglar o hazme una pregunta.',
    ideas: 'Ideas:',
    tu: 'Tú',
    nyx: 'Nyx',
    trazaVacia: 'Aquí verás cómo pienso cada pregunta, paso a paso.',
    tuCodigo: 'Tu código Go',
    pistaEditor: 'Tab inserta una tabulación. Pulsa Esc y luego Tab para salir del editor.',
    entradaPrograma: 'Entrada (lo que escribirías)',
    explicacionLineas: 'Qué hace cada línea',
    pruebasVacio: 'Cuando pruebe una función, aquí verás todas las pruebas.',
    soloFallos: 'Solo fallos',
    descargarTest: 'Descargar _test.go',
    buscarMemoria: 'Buscar en mi memoria',
    funciones: 'Funciones',
    datos: 'Datos',
    palabras: 'Palabras',
    pendientes: 'Pendientes',
    estudiarExplica: 'Leo los ejemplos de gobyexample y la documentación de Go que tienes instalada, los compruebo y vuelvo a intentar las preguntas que no supe responder.',
    estudiarBoton: 'Estudiar Go (gobyexample y la documentación local)',
    progresoEstudio: 'Progreso',
    pendientesTitulo: 'Preguntas pendientes',
    usa: 'Este programa usa:',
    riesgoTexto: 'Podría leer, cambiar o borrar tus archivos. Lo ejecutaré en una carpeta temporal, sin red y con límites, pero aun así: ¿Ejecutar igualmente?',
    noPuedo: 'No puedo ejecutar esto: usa cgo/linkname',
    cancelar: 'Cancelar',
    queDeberiaDar: '¿Qué debería dar? (opcional)',
    enviar: 'Enviar',
    ejemploPlaceholder: 'Escribe un ejemplo: entrada -> salida',
    estaMal: 'está mal',
    parado: 'Lo he parado. Esto es lo mejor que tenía:',
    loHeParado: 'Lo he parado.',
    terminada: 'terminada',
    cancelada: 'cancelada',
    errorTarea: 'con error',
    enMarcha: 'en marcha',
    pasosMas: function (n) { return '… ' + n + (n === 1 ? ' paso más' : ' pasos más'); },
    verDetalles: 'ver detalles',
    respuestaLibre: 'O escribe tu respuesta',
    preguntaYaNo: 'Esa pregunta ya no espera respuesta.',
    sinRespuestaSegui: 'Seguí sin tu respuesta.',
    codigoGo: 'código Go',
    cambios: 'cambios',
    porQue: '¿Por qué?',
    fuentes: 'Fuentes',
    verPruebas: 'Ver todas las pruebas',
    noPudeEnviar: 'No pude enviar la pregunta',
    escribeAlgo: 'Escribe una pregunta o pega código.',
    pegaCodigoPrimero: 'Primero escribe o pega código en el editor.',
    ejecutarCodigo: 'Ejecutar este código',
    probarCodigo: 'Probar este código',
    explicarCodigo: 'Explicar este código',
    arreglarCodigo: 'Arreglar este código',
    conEntrada: 'con la entrada',
    pensarMasNota: 'Pensar más sobre lo anterior',
    olvidar: 'Olvidar',
    usar: 'Usar',
    reintentar: 'Reintentar',
    olvidarTitulo: function (x) { return '¿Olvidar «' + x + '»?'; },
    olvidarTexto: 'Lo borraré de mi memoria. No se puede deshacer.',
    olvidado: function (x) { return 'He olvidado «' + x + '».'; },
    verCodigo: 'Ver código',
    verCasos: function (n) { return 'Ver casos (' + n + ')'; },
    nadaFunciones: 'Todavía no he aprendido funciones.',
    nadaDatos: 'Todavía no sé datos.',
    nadaPalabras: 'Todavía no he aprendido palabras nuevas.',
    nadaPendientes: 'No tengo preguntas pendientes.',
    usos: function (n) { return n === 1 ? 'usada 1 vez' : 'usada ' + n + ' veces'; },
    origen: 'Origen',
    fuente: 'Fuente',
    confianza: 'confianza',
    intentos: function (n) { return n === 1 ? '1 intento' : n + ' intentos'; },
    estudioEmpieza: 'Empiezo a estudiar…',
    estudioTermina: 'He terminado de estudiar.',
    ultimoCodigo: 'Pasé el código al editor. Escribe la entrada que usaría el programa y pulsa Ejecutar.',
    entendiPrefijo: 'Así lo entendí: ',
    nadaQueMostrar: 'No tengo nada más que mostrar.',
    columnaOk: '✔/✘',
    deDonde: 'De dónde',
    resumenPruebas: function (ok, total) { return ok + ' de ' + total + ' pruebas pasan.'; },
    copiaManual: 'No pude copiar: selecciona el código y cópialo a mano.',
    internet: function (si) { return 'Internet: ' + (si ? 'sí' : 'no'); },
    caja: function (n) { return 'Caja segura: ' + (n || 'desconocida'); },
    sabe: function (f, h) { return 'Sé ' + f + (f === 1 ? ' función' : ' funciones') + ' · ' + h + (h === 1 ? ' dato' : ' datos'); },
    sinGo: 'Go no encontrado',
    sello: {
      tus_ejemplos: function (c) { return '✔ Cumple tus ' + c.total + ' ejemplos'; },
      entendido: function (c) { return '✔ ' + c.pasan + '/' + c.total + ' según lo que entendí'; },
      propiedades: function (c) { return '✔ Propiedades ' + c.pasan + '/' + c.total; },
      calculo: function () { return '✔ Comprobado por cálculo'; },
      sin_fallos: function (c) { return 'Sin errores en ' + c.total + ' pruebas'; },
      fuente: function () { return 'Según la fuente'; },
      sin_comprobar: function () { return 'Sin comprobar'; },
      fallo: function () { return '✘ No lo he conseguido'; }
    },
    nivelCorto: {
      tus_ejemplos: 'cumple tus ejemplos', entendido: 'según lo que entendí', propiedades: 'propiedades comprobadas',
      calculo: 'comprobado por cálculo', sin_fallos: 'sin errores', fuente: 'según una fuente',
      sin_comprobar: 'sin comprobar', fallo: 'no lo conseguí'
    },
    origenes: {
      sintesis: 'la construí yo', receta: 'de mis recetas', web: 'de la web', reparacion: 'al arreglar código',
      usuario: 'me la diste tú'
    },
    deDondeCaso: {
      usuario: 'tu ejemplo', contraejemplo: 'contraejemplo', borde: 'borde', azar: 'al azar',
      sonda: 'lo que entendí', receta: 'receta', web: 'web'
    },
    ideasLista: [
      ['suma de pares', 'haz una función que sume los números pares de una lista'],
      ['resuelve 2x+3=11', 'resuelve 2x+3=11'],
      ['8 reinas', 'resuelve el problema de las 8 reinas'],
      ['arregla mi código', null]
    ]
  };

  // ---------------------------------------------------------------------------------------------
  // Utilidades de DOM (sin innerHTML).
  // ---------------------------------------------------------------------------------------------
  function $(id) { return document.getElementById(id); }

  function el(etiqueta, attrs) {
    var n = document.createElement(etiqueta);
    if (attrs) {
      Object.keys(attrs).forEach(function (k) {
        var v = attrs[k];
        if (v === null || v === undefined || v === false) return;
        if (k === 'class') n.className = v;
        else if (k === 'text') n.textContent = v;
        else if (k.slice(0, 2) === 'on' && typeof v === 'function') n.addEventListener(k.slice(2), v);
        else n.setAttribute(k, v === true ? '' : String(v));
      });
    }
    for (var i = 2; i < arguments.length; i++) agregar(n, arguments[i]);
    return n;
  }

  function agregar(padre, hijo) {
    if (hijo === null || hijo === undefined || hijo === false) return;
    if (Array.isArray(hijo)) { hijo.forEach(function (h) { agregar(padre, h); }); return; }
    if (typeof hijo === 'string' || typeof hijo === 'number') { padre.appendChild(document.createTextNode(String(hijo))); return; }
    padre.appendChild(hijo);
  }

  function vaciar(n) { while (n.firstChild) n.removeChild(n.firstChild); }

  function boton(texto, alPulsar, clase, attrs) {
    var a = attrs || {};
    a.type = 'button';
    a.class = clase || 'secundario';
    var b = el('button', a, texto);
    if (alPulsar) b.addEventListener('click', alPulsar);
    return b;
  }

  function esURLSegura(u) { return typeof u === 'string' && /^https?:\/\//i.test(u); }

  function enlace(url, texto) {
    if (!esURLSegura(url)) return el('span', null, texto || url || '');
    return el('a', { href: url, rel: 'noopener noreferrer', target: '_blank' }, texto || url);
  }

  function dominio(url) {
    var m = /^https?:\/\/([^\/?#:]+)/i.exec(url || '');
    return m ? m[1].replace(/^www\./, '') : '';
  }

  function segundos(ms) {
    var s = (Number(ms) || 0) / 1000;
    return s.toFixed(1).replace('.', ',') + ' s';
  }

  function fecha(f) {
    if (!f) return '';
    var d = new Date(f);
    if (isNaN(d.getTime()) || d.getFullYear() < 2000) return '';
    return d.toLocaleDateString('es-ES', { day: 'numeric', month: 'numeric', year: 'numeric' });
  }

  function anunciar(texto) {
    var a = $('anuncios');
    a.textContent = '';
    setTimeout(function () { a.textContent = texto; }, 30);
  }

  function hexAleatorio(n) {
    var b = new Uint8Array(n);
    window.crypto.getRandomValues(b);
    return Array.prototype.map.call(b, function (x) { return ('0' + x.toString(16)).slice(-2); }).join('');
  }

  function guardar(clave, valor) { try { sessionStorage.setItem(clave, valor); } catch (e) { /* sin almacenamiento */ } }
  function leerGuardado(clave) { try { return sessionStorage.getItem(clave); } catch (e) { return null; } }

  // ---------------------------------------------------------------------------------------------
  // Estado.
  // ---------------------------------------------------------------------------------------------
  var token = null;
  var sesion = null;
  var tareaChat = null;       // contexto de la última tarea de la conversación
  var tareaEstudio = null;    // contexto de la tarea de estudio
  var preguntaAbierta = null; // { ctx, pregunta, contestar }
  var avisoActivo = false;

  // ---------------------------------------------------------------------------------------------
  // API.
  // ---------------------------------------------------------------------------------------------
  function api(metodo, ruta, cuerpo) {
    var opciones = { method: metodo, headers: { 'X-Nyx-Token': token }, cache: 'no-store', credentials: 'same-origin' };
    if (cuerpo !== undefined) {
      opciones.headers['Content-Type'] = 'application/json';
      opciones.body = JSON.stringify(cuerpo);
    }
    return fetch(ruta, opciones).then(function (r) {
      ocultarAvisoConexion();
      if (r.status === 403) mostrarSinToken();
      var tipo = r.headers.get('Content-Type') || '';
      if (r.status === 204 || tipo.indexOf('application/json') < 0) return { status: r.status, datos: null };
      return r.json().then(function (d) { return { status: r.status, datos: d }; }, function () { return { status: r.status, datos: null }; });
    }, function (e) {
      mostrarAvisoConexion();
      throw e;
    });
  }

  function mostrarAvisoConexion() {
    var a = $('aviso-conexion');
    a.textContent = TEXTOS.sinConexion;
    a.hidden = false;
    avisoActivo = true;
  }

  function ocultarAvisoConexion() {
    if (!avisoActivo) return;
    $('aviso-conexion').hidden = true;
    avisoActivo = false;
  }

  function mostrarSinToken() {
    $('sin-token').hidden = false;
    $('app').hidden = true;
  }

  // ---------------------------------------------------------------------------------------------
  // Estado de la aplicación (chips de la cabecera).
  // ---------------------------------------------------------------------------------------------
  function versionCorta(v) {
    var m = /go(\d+)\.(\d+)/.exec(v || '');
    return m ? 'Go ' + m[1] + '.' + m[2] : 'Go';
  }

  function chip(id, texto, bien) {
    var c = $(id);
    c.textContent = texto;
    c.classList.toggle('bien', bien === true);
    c.classList.toggle('mal', bien === false);
  }

  function actualizarEstado() {
    return api('GET', '/api/estado').then(function (r) {
      if (r.status !== 200 || !r.datos) return;
      var e = r.datos;
      var ar = e.arenero || {};
      chip('chip-go', ar.goOK ? versionCorta(ar.goVersion) + ' ✔' : TEXTOS.sinGo, !!ar.goOK);
      chip('chip-internet', TEXTOS.internet(!!e.internet), !!e.internet);
      chip('chip-caja', TEXTOS.caja(ar.nivel), ar.nivel === 'completa');
      chip('chip-memoria', TEXTOS.sabe(e.funciones || 0, e.hechos || 0));
      var avisos = $('avisos');
      vaciar(avisos);
      (e.avisos || []).forEach(function (t) { avisos.appendChild(el('li', null, t)); });
      avisos.hidden = !(e.avisos && e.avisos.length);
    }).catch(function () { /* aviso ya mostrado */ });
  }

  // ---------------------------------------------------------------------------------------------
  // Resaltado de Go: tokenizador con expresiones regulares; cada token es un span con textContent.
  // ---------------------------------------------------------------------------------------------
  var RE_GO = new RegExp([
    '(\\/\\/[^\\n]*|\\/\\*[\\s\\S]*?\\*\\/)',
    '("(?:[^"\\\\\\n]|\\\\.)*"|`[^`]*`|\'(?:[^\'\\\\\\n]|\\\\.)*\')',
    '(\\b(?:0[xXbBoO][0-9a-fA-F_]+|\\d[\\d_]*(?:\\.\\d*)?(?:[eE][+-]?\\d+)?)\\b)',
    '(\\b(?:break|case|chan|const|continue|default|defer|else|fallthrough|for|func|go|goto|if|import|interface|map|package|range|return|select|struct|switch|type|var)\\b)',
    '(\\b(?:bool|byte|rune|string|error|int|int8|int16|int32|int64|uint|uint8|uint16|uint32|uint64|uintptr|float32|float64|complex64|complex128|any|true|false|nil|iota|append|cap|close|copy|delete|len|make|new|panic|print|println|recover|min|max|clear)\\b)'
  ].join('|'), 'g');

  function resaltar(codigo, destino) {
    var ultimo = 0;
    var m;
    RE_GO.lastIndex = 0;
    while ((m = RE_GO.exec(codigo)) !== null) {
      if (m[0].length === 0) { RE_GO.lastIndex++; continue; }
      if (m.index > ultimo) destino.appendChild(document.createTextNode(codigo.slice(ultimo, m.index)));
      var clase = m[1] ? 'tk-com' : m[2] ? 'tk-cad' : m[3] ? 'tk-num' : m[4] ? 'tk-pal' : 'tk-tipo';
      destino.appendChild(el('span', { class: clase }, m[0]));
      ultimo = RE_GO.lastIndex;
    }
    if (ultimo < codigo.length) destino.appendChild(document.createTextNode(codigo.slice(ultimo)));
  }

  function nombreArchivo(codigo) {
    if (/^\s*package\s+main\b/m.test(codigo) && /func\s+main\s*\(/.test(codigo)) return 'programa.go';
    var m = /func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)/.exec(codigo);
    return m ? m[1].toLowerCase() + '.go' : 'codigo.go';
  }

  function descargarTexto(nombre, texto) {
    var url = URL.createObjectURL(new Blob([texto], { type: 'text/plain;charset=utf-8' }));
    var a = el('a', { href: url, download: nombre });
    document.body.appendChild(a);
    a.click();
    setTimeout(function () { URL.revokeObjectURL(url); a.remove(); }, 1000);
  }

  function copiar(texto, b) {
    function hecho() {
      var antes = b.textContent;
      b.textContent = TEXTOS.copiado;
      anunciar(TEXTOS.copiado);
      setTimeout(function () { b.textContent = antes; }, 1500);
    }
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(texto).then(hecho, function () { copiarViejo(texto) ? hecho() : anunciar(TEXTOS.copiaManual); });
    } else if (copiarViejo(texto)) {
      hecho();
    } else {
      anunciar(TEXTOS.copiaManual);
    }
  }

  function copiarViejo(texto) {
    var t = el('textarea', { class: 'visualmente-oculto', readonly: true });
    t.value = texto;
    document.body.appendChild(t);
    t.select();
    var ok = false;
    try { ok = document.execCommand('copy'); } catch (e) { ok = false; }
    t.remove();
    return ok;
  }

  function leeEntrada(codigo) {
    return /bufio\.|fmt\.Scan|os\.Stdin|os\.Args/.test(codigo);
  }

  function bloqueCodigo(codigo, conAcciones) {
    var pre = el('pre', { class: 'codigo', tabindex: '0' });
    var code = el('code');
    resaltar(codigo, code);
    pre.appendChild(code);
    var barra = el('div', { class: 'barra' }, el('span', { class: 'rotulo' }, TEXTOS.codigoGo));
    var bCopiar = boton(TEXTOS.copiar, function () { copiar(codigo, bCopiar); }, 'secundario pequeno');
    barra.appendChild(bCopiar);
    if (conAcciones) {
      barra.appendChild(boton(TEXTOS.ejecutar, function () {
        if (leeEntrada(codigo)) {
          ponerEnEditor(codigo);
          seleccionarPestana('tab-codigo');
          $('entrada-programa').focus();
          anunciar(TEXTOS.ultimoCodigo);
          return;
        }
        accionCodigo('ejecutar', codigo, '');
      }, 'secundario pequeno'));
      barra.appendChild(boton(TEXTOS.probar, function () { accionCodigo('probar', codigo, ''); }, 'secundario pequeno'));
      barra.appendChild(boton(TEXTOS.explicar, function () { ponerEnEditor(codigo); accionCodigo('explicar', codigo, ''); }, 'secundario pequeno'));
      barra.appendChild(boton(TEXTOS.descargar, function () { descargarTexto(nombreArchivo(codigo), codigo); }, 'secundario pequeno'));
    }
    return el('div', { class: 'bloque-codigo' }, barra, pre);
  }

  function bloqueDiff(diff) {
    var pre = el('pre', { class: 'codigo diff', tabindex: '0' });
    diff.split('\n').forEach(function (l) {
      var clase = 'linea';
      if (/^\+/.test(l) && !/^\+\+\+/.test(l)) clase = 'linea-mas';
      else if (/^-/.test(l) && !/^---/.test(l)) clase = 'linea-menos';
      else if (/^(@@|\+\+\+|---)/.test(l)) clase = 'linea-info';
      pre.appendChild(el('span', { class: clase }, l || ' '));
    });
    return el('div', { class: 'bloque-codigo' }, el('div', { class: 'barra' }, el('span', { class: 'rotulo' }, TEXTOS.cambios)), pre);
  }

  function tabla(t, filtro) {
    var tb = el('table', { class: 'tabla' });
    if (t.titulo) tb.appendChild(el('caption', null, t.titulo));
    var thead = el('thead');
    var trh = el('tr');
    (t.cabecera || []).forEach(function (h) { trh.appendChild(el('th', { scope: 'col' }, h)); });
    thead.appendChild(trh);
    tb.appendChild(thead);
    var tbody = el('tbody');
    (t.filas || []).forEach(function (f, i) {
      var marca = (t.marcas || [])[i] || '';
      if (filtro && !filtro(marca, f)) return;
      var tr = el('tr', { class: marca === 'bien' ? 'fila-bien' : marca === 'mal' ? 'fila-mal' : null });
      f.forEach(function (c) {
        var td = el('td', null, c);
        if (c === '✔') td.className = 'marca-bien';
        if (c === '✘') td.className = 'marca-mal';
        tr.appendChild(td);
      });
      tbody.appendChild(tr);
    });
    tb.appendChild(tbody);
    return el('div', { class: 'contenedor-tabla' }, tb);
  }

  function tablero(tb) {
    var t = el('table', { class: 'tablero', 'aria-label': 'Tablero' });
    var n = (tb.celdas || []).length;
    var caja = Math.round(Math.sqrt(n));
    (tb.celdas || []).forEach(function (fila, i) {
      var tr = el('tr', { class: (caja * caja === n && (i + 1) % caja === 0 && i + 1 < n) ? 'borde-abajo' : null });
      fila.forEach(function (v, j) {
        var fija = tb.fijas && tb.fijas[i] && tb.fijas[i][j];
        var clases = [];
        if (fija) clases.push('fija');
        if (caja * caja === fila.length && (j + 1) % caja === 0 && j + 1 < fila.length) clases.push('borde-derecho');
        tr.appendChild(el('td', { class: clases.join(' ') || null }, v ? String(v) : ''));
      });
      t.appendChild(tr);
    });
    return el('div', { class: 'contenedor-tabla' }, t);
  }

  function textoSello(c) {
    var t = (c.texto || '').trim();
    var f = TEXTOS.sello[c.nivel];
    if (!t) return f ? f(c) : (c.nivel || '');
    if (/^[✔✘]/.test(t)) return t;
    if (c.nivel === 'fallo') return '✘ ' + t;
    if (c.nivel === 'tus_ejemplos' || c.nivel === 'entendido' || c.nivel === 'propiedades' || c.nivel === 'calculo') return '✔ ' + t;
    return t;
  }

  function sellos(lista) {
    var d = el('div', { class: 'sellos' });
    lista.forEach(function (c) {
      var ayuda = [c.detalle, c.comando].filter(Boolean).join('\n');
      var texto = textoSello(c);
      d.appendChild(el('span', {
        class: 'sello nivel-' + (c.nivel || 'sin_comprobar'), tabindex: ayuda ? '0' : null, title: ayuda || null,
        'aria-label': ayuda ? texto + '. ' + ayuda : null
      }, texto));
    });
    return d;
  }

  // ---------------------------------------------------------------------------------------------
  // Conversación.
  // ---------------------------------------------------------------------------------------------
  function bajarMensajes() {
    var m = $('mensajes');
    m.scrollTop = m.scrollHeight;
  }

  function burbuja(quien, contenido, clase) {
    var cuerpo = el('div', { class: 'cuerpo' });
    agregar(cuerpo, contenido);
    var m = el('div', { class: 'mensaje de-' + quien + (clase ? ' ' + clase : '') },
      el('span', { class: 'autor' }, quien === 'tu' ? TEXTOS.tu : TEXTOS.nyx), cuerpo);
    $('mensajes').appendChild(m);
    bajarMensajes();
    return cuerpo;
  }

  function bienvenida() {
    var ideas = el('div', { class: 'ideas' }, el('span', null, TEXTOS.ideas));
    TEXTOS.ideasLista.forEach(function (par) {
      ideas.appendChild(boton(par[0], function () {
        if (par[1]) { enviarTexto(par[1]); return; }
        marcarModo('arreglar');
        abrirZonaCodigo(true);
        $('codigo-adjunto').focus();
      }, 'chip-boton'));
    });
    burbuja('nyx', [el('p', null, TEXTOS.hola), ideas]);
  }

  function marcarModo(valor) {
    var radios = document.querySelectorAll('input[name="modo"]');
    Array.prototype.forEach.call(radios, function (r) { r.checked = r.value === valor; });
  }

  function modoActual() {
    var r = document.querySelector('input[name="modo"]:checked');
    return r ? r.value : '';
  }

  function abrirZonaCodigo(abrir) {
    $('zona-codigo').hidden = !abrir;
    $('b-pegar').setAttribute('aria-expanded', abrir ? 'true' : 'false');
  }

  function ocupado(si, texto) {
    var e = $('estado-tarea');
    e.textContent = si ? (texto || TEXTOS.pensando) : '';
    e.classList.toggle('activa', !!si);
    $('b-parar').disabled = !si;
    $('b-mas').disabled = si || !tareaChat;
  }

  function enviarTexto(texto, codigo) {
    if (preguntaAbierta && !codigo) {
      // una pregunta pendiente: lo escrito es la respuesta
      preguntaAbierta.contestar(texto);
      return Promise.resolve();
    }
    texto = (texto || '').trim();
    codigo = codigo || '';
    if (!texto && !codigo.trim()) { anunciar(TEXTOS.escribeAlgo); return Promise.resolve(); }
    burbuja('tu', codigo ? [texto, codigo ? bloqueCodigo(codigo, false) : null] : texto);
    var peticion = { sesion: sesion, texto: texto, codigo: codigo, modo: modoActual() };
    ocupado(true);
    return api('POST', '/api/preguntar', peticion).then(function (r) {
      if (r.status !== 200 || !r.datos || !r.datos.tarea) {
        ocupado(false);
        burbuja('nyx', (r.datos && r.datos.error) || TEXTOS.noPudeEnviar, 'error');
        return;
      }
      tareaChat = seguirTarea(r.datos.tarea, { titulo: texto || TEXTOS.codigo, peticion: peticion, chat: true });
    }).catch(function () { ocupado(false); });
  }

  function accionCodigo(accion, codigo, entrada, confirmado) {
    if (!codigo || !codigo.trim()) { anunciar(TEXTOS.pegaCodigoPrimero); return; }
    var etiquetas = { ejecutar: TEXTOS.ejecutarCodigo, probar: TEXTOS.probarCodigo, explicar: TEXTOS.explicarCodigo, arreglar: TEXTOS.arreglarCodigo };
    var cuerpo = { sesion: sesion, codigo: codigo, texto: '', entrada: entrada || '', args: [], confirmado: !!confirmado };
    ocupado(true);
    api('POST', '/api/codigo/' + accion, cuerpo).then(function (r) {
      if (r.status === 409 && r.datos && r.datos.riesgos) {
        ocupado(false);
        dialogoRiesgos(r.datos.riesgos, function () { accionCodigo(accion, codigo, entrada, true); });
        return;
      }
      if (r.status !== 200 || !r.datos || !r.datos.tarea) {
        ocupado(false);
        burbuja('nyx', (r.datos && r.datos.error) || TEXTOS.noPudeEnviar, 'error');
        return;
      }
      burbuja('tu', [etiquetas[accion] || accion, entrada ? ' (' + TEXTOS.conEntrada + ' «' + recortar(entrada, 40) + '»)' : '']);
      tareaChat = seguirTarea(r.datos.tarea, { titulo: etiquetas[accion] || accion, peticion: cuerpo, accion: accion, chat: true });
    }).catch(function () { ocupado(false); });
  }

  // ---------------------------------------------------------------------------------------------
  // Tareas y flujo de eventos (SSE).
  // ---------------------------------------------------------------------------------------------
  function seguirTarea(id, opciones) {
    var ctx = {
      id: id, titulo: opciones.titulo, peticion: opciones.peticion || {}, accion: opciones.accion || '',
      chat: !!opciones.chat, estudio: !!opciones.estudio, respuesta: null, terminada: false, fallos: 0,
      nodos: {}, grupo: null, raiz: null
    };
    crearGrupoTraza(ctx);
    var url = '/api/tareas/' + encodeURIComponent(id) + '/eventos?t=' + encodeURIComponent(token);
    var es = new EventSource(url);
    ctx.es = es;
    function datos(e) { try { return JSON.parse(e.data); } catch (x) { return null; } }
    es.addEventListener('open', function () { ctx.fallos = 0; ocultarAvisoConexion(); });
    es.addEventListener('paso', function (e) { var p = datos(e); if (p) pintarPaso(ctx, p); });
    es.addEventListener('actualiza', function (e) { var p = datos(e); if (p) pintarPaso(ctx, p); });
    es.addEventListener('pregunta', function (e) { var q = datos(e); if (q) mostrarPregunta(ctx, q); });
    es.addEventListener('respuesta', function (e) { ctx.respuesta = datos(e); });
    es.addEventListener('fin', function (e) {
      es.close();
      terminarTarea(ctx, datos(e) || { estado: 'terminada' });
    });
    es.addEventListener('error', function () {
      if (ctx.terminada) return;
      if (es.readyState === EventSource.CLOSED) {
        // el servidor ya no tiene más que contar (204) o la tarea desapareció
        terminarTarea(ctx, { estado: ctx.respuesta ? 'terminada' : 'error' });
        return;
      }
      ctx.fallos++;
      if (ctx.fallos >= 2) {
        api('GET', '/api/estado').catch(function () { /* el aviso ya se muestra */ });
      }
    });
    return ctx;
  }

  function terminarTarea(ctx, fin) {
    if (ctx.terminada) return;
    ctx.terminada = true;
    if (ctx.es) ctx.es.close();
    marcarGrupo(ctx, fin);
    cerrarPreguntasDe(ctx);
    if (ctx.estudio) {
      terminarEstudio(ctx, fin);
    } else {
      if (ctx.respuesta) {
        pintarRespuesta(ctx, ctx.respuesta, fin);
      } else if (fin.estado === 'cancelada') {
        burbuja('nyx', TEXTOS.loHeParado);
      } else if (fin.estado === 'error') {
        burbuja('nyx', TEXTOS.sinConexion, 'error');
      }
      if (ctx === tareaChat) ocupado(false);
    }
    actualizarEstado();
    if (!$('panel-memoria').hidden) cargarMemoria();
  }

  // ---- árbol "Cómo lo pienso" ----
  var pegadoAbajo = true;

  function crearGrupoTraza(ctx) {
    $('traza-vacia').hidden = true;
    var arbol = $('arbol');
    // los grupos anteriores se pliegan
    Array.prototype.forEach.call(arbol.querySelectorAll('details.grupo-traza[open]'), function (d) { d.open = false; });
    var estado = el('span', { class: 'estado-grupo' }, TEXTOS.enMarcha);
    var resumen = el('summary', null, el('span', null, '«' + recortar(ctx.titulo, 90) + '»'), estado);
    var raiz = el('ul', { class: 'arbol-nivel' });
    var grupo = el('details', { class: 'grupo-traza', open: true }, resumen, raiz);
    arbol.appendChild(grupo);
    ctx.grupo = grupo;
    ctx.raiz = raiz;
    ctx.estadoGrupo = estado;
    if (!ctx.estudio) avisarPestana('tab-traza');
    desplazarArbol();
  }

  function recortar(s, n) {
    s = String(s || '').replace(/\s+/g, ' ').trim();
    return s.length > n ? s.slice(0, n - 1) + '…' : s;
  }

  function marcarGrupo(ctx, fin) {
    if (!ctx.estadoGrupo) return;
    var palabra = fin.estado === 'cancelada' ? TEXTOS.cancelada : fin.estado === 'error' ? TEXTOS.errorTarea : TEXTOS.terminada;
    ctx.estadoGrupo.textContent = palabra + (fin.ms ? ' · ' + segundos(fin.ms) : '');
  }

  function marca(estado) {
    return estado === 'bien' ? '✔' : estado === 'mal' ? '✘' : estado === 'info' ? '·' : '▸';
  }

  function desplazarArbol() {
    var p = $('panel-traza');
    if (pegadoAbajo) p.scrollTop = p.scrollHeight;
  }

  function pintarPaso(ctx, p) {
    var nodo = ctx.nodos[p.id];
    if (!nodo) {
      var li = el('li', { class: 'paso' });
      var hijos = el('ul', { class: 'arbol-nivel' });
      nodo = { li: li, hijos: hijos, abierto: false };
      ctx.nodos[p.id] = nodo;
      var padre = ctx.nodos[p.padre];
      var lista = padre ? padre.hijos : ctx.raiz;
      lista.appendChild(li);
      plegarHermanos(lista);
    }
    dibujarPaso(nodo, p);
    if (ctx.estudio) progresoEstudio(p);
    if (!ctx.estudio) avisarPestana('tab-traza');
    desplazarArbol();
  }

  function dibujarPaso(nodo, p) {
    var li = nodo.li;
    li.className = 'paso tipo-' + (p.tipo || 'nota') + ' estado-' + (p.estado || 'abierto');
    var tieneExtra = !!(p.detalle || p.codigo || p.tabla);
    var fila = el(tieneExtra ? 'button' : 'div', { class: 'fila', type: tieneExtra ? 'button' : null, 'aria-expanded': tieneExtra ? String(nodo.abierto) : null },
      el('span', { class: 'marca-paso', 'aria-hidden': 'true' }, marca(p.estado)),
      el('span', { class: 'visualmente-oculto' }, p.estado === 'bien' ? 'bien: ' : p.estado === 'mal' ? 'mal: ' : ''),
      el('span', { class: 'titulo' }, p.titulo || ''),
      p.progreso ? el('span', { class: 'progreso' }, p.progreso) : null,
      tieneExtra ? el('span', { class: 'mas-detalles' }, TEXTOS.verDetalles) : null,
      el('span', { class: 'tiempo' }, segundos(p.ms)));
    var extra = null;
    if (tieneExtra) {
      extra = el('div', { class: 'extra' });
      if (p.detalle) extra.appendChild(el('p', { class: 'detalle' }, p.detalle));
      if (p.codigo) extra.appendChild(bloqueCodigo(p.codigo, false));
      if (p.tabla) extra.appendChild(tabla(p.tabla));
      extra.hidden = !nodo.abierto;
      fila.addEventListener('click', function () {
        nodo.abierto = !nodo.abierto;
        extra.hidden = !nodo.abierto;
        fila.setAttribute('aria-expanded', String(nodo.abierto));
      });
    }
    // reemplazar fila y extra, conservar los hijos
    while (li.firstChild && li.firstChild !== nodo.hijos) li.removeChild(li.firstChild);
    if (!nodo.hijos.parentNode) li.appendChild(nodo.hijos);
    li.insertBefore(fila, nodo.hijos);
    if (extra) li.insertBefore(extra, nodo.hijos);
  }

  // más de 20 hermanos: se ocultan los más antiguos detrás de "… N pasos más"
  function plegarHermanos(lista) {
    if (lista.dataset.expandido === 'si') return;
    var pasos = Array.prototype.filter.call(lista.children, function (c) { return c.classList.contains('paso'); });
    var sobran = pasos.length - 20;
    var b = lista.querySelector(':scope > li.mas-pasos');
    if (sobran <= 0) return;
    pasos.forEach(function (p, i) { p.hidden = i < sobran; });
    if (!b) {
      var btn = boton('', function () {
        lista.dataset.expandido = 'si';
        Array.prototype.forEach.call(lista.children, function (c) { c.hidden = false; });
        b.remove();
      }, 'secundario pequeno');
      b = el('li', { class: 'mas-pasos' }, btn);
      lista.insertBefore(b, lista.firstChild);
    }
    b.firstChild.textContent = TEXTOS.pasosMas(sobran);
  }

  // ---- preguntas ----
  function mostrarPregunta(ctx, q) {
    var tarjeta = el('div', { class: 'tarjeta-pregunta', role: 'group', 'aria-label': q.texto || '' });
    tarjeta.appendChild(el('p', { class: 'texto-pregunta' }, q.texto || ''));
    var opciones = el('div', { class: 'opciones' });
    var controles = [];
    var hecha = false;
    function contestar(valor) {
      valor = String(valor || '').trim();
      if (hecha || !valor) return;
      hecha = true;
      controles.forEach(function (c) { c.disabled = true; });
      tarjeta.classList.add('contestada');
      if (preguntaAbierta && preguntaAbierta.tarjeta === tarjeta) preguntaAbierta = null;
      burbuja('tu', valor);
      api('POST', '/api/tareas/' + encodeURIComponent(ctx.id) + '/responder', { pregunta: q.id, respuesta: valor }).then(function (r) {
        if (r.status === 409 || r.status === 404) tarjeta.appendChild(el('p', { class: 'nota' }, TEXTOS.preguntaYaNo));
      }).catch(function () { /* aviso de conexión */ });
    }
    (q.opciones || []).forEach(function (o) {
      var b = boton(o, function () { contestar(o); }, o === q.porDefecto ? 'principal' : 'secundario');
      controles.push(b);
      opciones.appendChild(b);
    });
    tarjeta.appendChild(opciones);
    if (q.libre) {
      var campo = el('input', { type: 'text', class: 'campo', 'aria-label': TEXTOS.respuestaLibre, placeholder: TEXTOS.respuestaLibre });
      var enviar = boton(TEXTOS.enviar, function () { contestar(campo.value); }, 'principal');
      campo.addEventListener('keydown', function (e) { if (e.key === 'Enter') { e.preventDefault(); contestar(campo.value); } });
      controles.push(campo, enviar);
      tarjeta.appendChild(el('div', { class: 'libre' }, campo, enviar));
    }
    $('mensajes').appendChild(tarjeta);
    bajarMensajes();
    preguntaAbierta = { ctx: ctx, tarjeta: tarjeta, contestar: contestar, cerrar: function () {
      if (hecha) return;
      hecha = true;
      controles.forEach(function (c) { c.disabled = true; });
      tarjeta.classList.add('contestada');
      tarjeta.appendChild(el('p', { class: 'nota' }, TEXTOS.sinRespuestaSegui));
    } };
    var primero = controles[0];
    if (primero) primero.focus();
    anunciar(q.texto || '');
  }

  function cerrarPreguntasDe(ctx) {
    if (preguntaAbierta && preguntaAbierta.ctx === ctx) {
      preguntaAbierta.cerrar();
      preguntaAbierta = null;
    }
  }

  // ---- respuestas ----
  function tieneContenido(r) {
    return !!(r && (r.texto || (r.parrafos && r.parrafos.length) || r.codigo || r.diff || (r.tablas && r.tablas.length) || r.tablero || (r.plan && r.plan.length)));
  }

  function pintarRespuesta(ctx, r, fin) {
    var partes = [];
    if (fin.estado === 'cancelada' && tieneContenido(r) && r.texto !== TEXTOS.loHeParado) partes.push(el('p', { class: 'parado' }, TEXTOS.parado));
    if (r.texto) partes.push(el('p', { class: 'texto-principal' }, r.texto));
    (r.parrafos || []).forEach(function (t) { partes.push(el('p', null, t)); });
    if (r.entendi) {
      var linea = el('div', { class: 'entendi' }, el('span', null, r.entendi),
        boton(TEXTOS.noEsEso, function () { noEsEso(); }, 'secundario pequeno'));
      partes.push(linea);
    }
    if (r.codigo) partes.push(bloqueCodigo(r.codigo, true));
    if (r.diff) partes.push(bloqueDiff(r.diff));
    (r.tablas || []).forEach(function (t) { partes.push(tabla(t)); });
    if (r.tablero) partes.push(tablero(r.tablero));
    if (r.plan && r.plan.length) {
      var ol = el('ol', { class: 'plan' });
      r.plan.forEach(function (p) { ol.appendChild(el('li', null, p)); });
      partes.push(ol);
    }
    if (r.comprobaciones && r.comprobaciones.length) partes.push(sellos(r.comprobaciones));
    if (r.fuentes && r.fuentes.length) {
      var ul = el('ul', { class: 'fuentes', 'aria-label': TEXTOS.fuentes });
      r.fuentes.forEach(function (f) {
        ul.appendChild(el('li', null, enlace(f.url, f.titulo || f.url), ' ', el('span', { class: 'dominio' }, dominio(f.url)),
          f.nota ? el('span', { class: 'dominio' }, ' — ' + f.nota) : null));
      });
      partes.push(ul);
    }
    if (r.aprendido) partes.push(el('p', { class: 'aprendido' }, r.aprendido));
    if (r.justificacion && r.justificacion.length) {
      var lista = el('ol');
      r.justificacion.forEach(function (j) { lista.appendChild(el('li', null, j)); });
      partes.push(el('details', { class: 'justificacion' }, el('summary', null, TEXTOS.porQue), lista));
    }
    if (r.codigoTest || hayTablaPruebas(r)) {
      partes.push(el('p', null, boton(TEXTOS.verPruebas, function () { seleccionarPestana('tab-pruebas'); }, 'secundario pequeno')));
    }
    if (!tieneContenido(r) && !partes.length) partes.push(el('p', null, TEXTOS.nadaQueMostrar));
    if (!ctx.estudio && fin.estado !== 'cancelada' && fin.estado !== 'error' && tieneContenido(r)) partes.push(filaOpinion(ctx, r));
    if (r.sugerencias && r.sugerencias.length) {
      var sug = el('div', { class: 'sugerencias' });
      r.sugerencias.forEach(function (s) { sug.appendChild(boton(s, function () { enviarTexto(s); }, 'sugerencia')); });
      partes.push(sug);
    }
    burbuja('nyx', partes, fin.estado === 'error' ? 'error' : null);
    actualizarPruebas(r);
    if (ctx.accion === 'explicar' || r.metodo === 'explicar') actualizarExplicacion(r);
    if (r.codigo && !$('editor').value.trim()) ponerEnEditor(r.codigo);
    if (r.riesgos && r.riesgos.length) {
      var codigo = (ctx.peticion && ctx.peticion.codigo) || r.codigo || '';
      var entrada = (ctx.peticion && ctx.peticion.entrada) || '';
      if (codigo) dialogoRiesgos(r.riesgos, function () { accionCodigo('ejecutar', codigo, entrada, true); });
    }
  }

  function noEsEso() {
    enviarTexto(TEXTOS.estaMal);
    var e = $('entrada');
    e.placeholder = TEXTOS.ejemploPlaceholder;
    e.focus();
  }

  function filaOpinion(ctx, r) {
    var fila = el('div', { class: 'opinion' }, el('span', null, TEXTOS.estaBien));
    var bSi = boton(TEXTOS.si, function () { opinar(true, '', ''); }, 'secundario pequeno');
    var bNo = boton(TEXTOS.no, function () {
      bSi.disabled = true; bNo.disabled = true;
      var campo = el('input', { type: 'text', class: 'campo', placeholder: TEXTOS.queDeberiaDar, 'aria-label': TEXTOS.queDeberiaDar });
      var enviar = boton(TEXTOS.enviar, function () { opinar(false, '', campo.value.trim()); }, 'principal pequeno');
      campo.addEventListener('keydown', function (e) { if (e.key === 'Enter') { e.preventDefault(); enviar.click(); } });
      fila.appendChild(campo);
      fila.appendChild(enviar);
      campo.focus();
    }, 'secundario pequeno');
    fila.appendChild(bSi);
    fila.appendChild(bNo);
    function opinar(correcto, comentario, ejemplo) {
      api('POST', '/api/opinion', { sesion: sesion, tarea: r.tarea || ctx.id, correcto: correcto, comentario: comentario, ejemplo: ejemplo })
        .then(function () {
          vaciar(fila);
          fila.appendChild(el('span', { class: 'gracias' }, TEXTOS.gracias));
          anunciar(TEXTOS.gracias);
        }).catch(function () { /* aviso */ });
    }
    return fila;
  }

  // ---------------------------------------------------------------------------------------------
  // Diálogos.
  // ---------------------------------------------------------------------------------------------
  function abrirDialogo(d) {
    if (typeof d.showModal === 'function') { d.showModal(); return true; }
    return false;
  }

  function dialogoRiesgos(riesgos, alAceptar) {
    var d = $('dialogo-riesgos');
    var lista = $('dr-lista');
    vaciar(lista);
    var grave = false;
    riesgos.forEach(function (v) {
      if (v.grave) grave = true;
      lista.appendChild(el('li', { class: v.grave ? 'grave' : null }, v.que + (v.linea ? ' (línea ' + v.linea + ')' : '')));
    });
    $('dr-texto').hidden = grave;
    $('dr-grave').hidden = !grave;
    $('dr-ejecutar').hidden = grave;
    var bEj = $('dr-ejecutar');
    var bCa = $('dr-cancelar');
    function cerrar() { bEj.onclick = null; bCa.onclick = null; if (d.open) d.close(); }
    bCa.onclick = cerrar;
    bEj.onclick = function () { cerrar(); alAceptar(); };
    if (!abrirDialogo(d)) {
      var texto = TEXTOS.usa + '\n' + riesgos.map(function (v) { return '- ' + v.que; }).join('\n') + '\n\n' + (grave ? TEXTOS.noPuedo : TEXTOS.riesgoTexto);
      if (grave) { window.alert(texto); return; }
      if (window.confirm(texto)) alAceptar();
      return;
    }
    bCa.focus();
  }

  function confirmar(titulo, texto, aceptar, alAceptar) {
    var d = $('dialogo-confirmar');
    $('dc-titulo').textContent = titulo;
    $('dc-texto').textContent = texto;
    var bA = $('dc-aceptar');
    var bC = $('dc-cancelar');
    bA.textContent = aceptar;
    function cerrar() { bA.onclick = null; bC.onclick = null; if (d.open) d.close(); }
    bC.onclick = cerrar;
    bA.onclick = function () { cerrar(); alAceptar(); };
    if (!abrirDialogo(d)) {
      if (window.confirm(titulo + '\n' + texto)) alAceptar();
      return;
    }
    bC.focus();
  }

  // ---------------------------------------------------------------------------------------------
  // Pestañas.
  // ---------------------------------------------------------------------------------------------
  function pestanas(contenedor, alCambiar) {
    var tabs = Array.prototype.slice.call(contenedor.querySelectorAll('[role="tab"]'));
    function elegir(t, foco) {
      tabs.forEach(function (x) {
        var si = x === t;
        x.setAttribute('aria-selected', si ? 'true' : 'false');
        x.tabIndex = si ? 0 : -1;
        if (si) x.classList.remove('novedad');
      });
      if (foco) t.focus();
      alCambiar(t);
    }
    tabs.forEach(function (t, i) {
      t.addEventListener('click', function () { elegir(t, false); });
      t.addEventListener('keydown', function (e) {
        var j = -1;
        if (e.key === 'ArrowRight') j = (i + 1) % tabs.length;
        else if (e.key === 'ArrowLeft') j = (i - 1 + tabs.length) % tabs.length;
        else if (e.key === 'Home') j = 0;
        else if (e.key === 'End') j = tabs.length - 1;
        if (j >= 0) { e.preventDefault(); elegir(tabs[j], true); }
      });
    });
    return elegir;
  }

  var elegirPestana = null;

  function seleccionarPestana(id) {
    if (elegirPestana) elegirPestana($(id), false);
  }

  function avisarPestana(id) {
    var t = $(id);
    if (t.getAttribute('aria-selected') !== 'true') t.classList.add('novedad');
  }

  function mostrarPanel(tab) {
    ['tab-traza', 'tab-codigo', 'tab-pruebas', 'tab-memoria', 'tab-estudiar'].forEach(function (id) {
      var t = $(id);
      $(t.getAttribute('aria-controls')).hidden = t !== tab;
    });
    if (tab.id === 'tab-memoria') cargarMemoria();
    if (tab.id === 'tab-estudiar') cargarPendientes($('lista-pendientes'));
    if (tab.id === 'tab-codigo') pintarMedianil();
  }

  // ---------------------------------------------------------------------------------------------
  // Pestaña Código: editor con números de línea.
  // ---------------------------------------------------------------------------------------------
  var frasesPorLinea = {};
  var escPulsado = false;

  function ponerEnEditor(codigo) {
    var ed = $('editor');
    ed.value = codigo;
    frasesPorLinea = {};
    $('explicacion-lineas').hidden = true;
    pintarMedianil();
  }

  function pintarMedianil() {
    var ed = $('editor');
    var med = $('medianil');
    var n = ed.value.split('\n').length;
    vaciar(med);
    for (var i = 1; i <= n; i++) {
      var f = frasesPorLinea[i];
      med.appendChild(el('div', { class: f ? 'con-frase' : null, title: f || null, 'data-linea': String(i) }, String(i)));
    }
    med.scrollTop = ed.scrollTop;
  }

  function lineaDelCursor(ed) {
    return ed.value.slice(0, ed.selectionStart).split('\n').length;
  }

  function mostrarFrase(linea) {
    var f = frasesPorLinea[linea];
    $('frase-linea').textContent = f ? 'Línea ' + linea + ': ' + f : '';
    Array.prototype.forEach.call($('lista-frases').children, function (li) {
      li.classList.toggle('activa', li.dataset.linea === String(linea));
    });
  }

  function iniciarEditor() {
    var ed = $('editor');
    ed.addEventListener('input', pintarMedianil);
    ed.addEventListener('scroll', function () { $('medianil').scrollTop = ed.scrollTop; });
    ed.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') { escPulsado = true; return; }
      if (e.key === 'Tab' && !e.shiftKey && !e.ctrlKey && !e.altKey && !escPulsado) {
        e.preventDefault();
        var i = ed.selectionStart;
        var j = ed.selectionEnd;
        ed.value = ed.value.slice(0, i) + '\t' + ed.value.slice(j);
        ed.selectionStart = ed.selectionEnd = i + 1;
        pintarMedianil();
        return;
      }
      escPulsado = false;
    });
    ed.addEventListener('keyup', function () { mostrarFrase(lineaDelCursor(ed)); });
    ed.addEventListener('click', function () { mostrarFrase(lineaDelCursor(ed)); });
    $('medianil').addEventListener('mouseover', function (e) {
      var l = e.target && e.target.dataset ? e.target.dataset.linea : null;
      if (l) mostrarFrase(Number(l));
    });
    Array.prototype.forEach.call(document.querySelectorAll('[data-accion-codigo]'), function (b) {
      b.addEventListener('click', function () {
        accionCodigo(b.getAttribute('data-accion-codigo'), ed.value, $('entrada-programa').value);
      });
    });
    pintarMedianil();
  }

  function normal(s) {
    return String(s || '').toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '');
  }

  function actualizarExplicacion(r) {
    var t = (r.tablas || []).filter(function (x) { return x.cabecera && x.cabecera.length >= 2 && normal(x.cabecera[0]).indexOf('linea') === 0; })[0];
    if (!t) return;
    frasesPorLinea = {};
    var lista = $('lista-frases');
    vaciar(lista);
    (t.filas || []).forEach(function (f) {
      var n = parseInt(f[0], 10);
      if (!n) return;
      var frase = f.slice(1).join(' — ');
      frasesPorLinea[n] = frasesPorLinea[n] ? frasesPorLinea[n] + ' ' + frase : frase;
      lista.appendChild(el('li', { value: String(n), 'data-linea': String(n) }, frase));
    });
    $('explicacion-lineas').hidden = !lista.children.length;
    pintarMedianil();
  }

  // ---------------------------------------------------------------------------------------------
  // Pestaña Pruebas.
  // ---------------------------------------------------------------------------------------------
  var ultimaTablaPruebas = null;
  var urlTest = null;

  function esTablaPruebas(t) {
    var c = (t.cabecera || []).map(normal);
    return c.indexOf('entrada') >= 0 && (c.indexOf('obtenido') >= 0 || c.indexOf('esperado') >= 0);
  }

  function hayTablaPruebas(r) {
    return (r.tablas || []).some(esTablaPruebas);
  }

  function actualizarPruebas(r) {
    var t = (r.tablas || []).filter(esTablaPruebas)[0];
    if (!t && !r.codigoTest) return;
    $('pruebas-vacio').hidden = true;
    $('pruebas-contenido').hidden = false;
    if (t) {
      ultimaTablaPruebas = t;
      pintarTablaPruebas();
    }
    var a = $('descargar-test');
    if (r.codigoTest) {
      if (urlTest) URL.revokeObjectURL(urlTest);
      urlTest = URL.createObjectURL(new Blob([r.codigoTest], { type: 'text/plain;charset=utf-8' }));
      a.href = urlTest;
      var m = /^package\s+(\w+)/m.exec(r.codigoTest);
      a.setAttribute('download', (m ? m[1] : 'solucion') + '_test.go');
      a.hidden = false;
    }
    avisarPestana('tab-pruebas');
  }

  function pintarTablaPruebas() {
    var t = ultimaTablaPruebas;
    if (!t) return;
    var cab = (t.cabecera || []).slice();
    var conMarca = cab.map(normal).indexOf('✔/✘') < 0 && (t.marcas || []).some(Boolean);
    var tb = { titulo: t.titulo, cabecera: cab, filas: [], marcas: t.marcas };
    if (conMarca) tb.cabecera = cab.concat([TEXTOS.columnaOk]);
    (t.filas || []).forEach(function (f, i) {
      var m = (t.marcas || [])[i] || '';
      var fila = f.slice();
      if (conMarca) fila.push(m === 'bien' ? '✔' : m === 'mal' ? '✘' : '');
      tb.filas.push(fila);
    });
    var soloFallos = $('solo-fallos').checked;
    var cont = $('tabla-pruebas');
    vaciar(cont);
    cont.appendChild(tabla(tb, soloFallos ? function (m) { return m === 'mal'; } : null));
    var total = (t.filas || []).length;
    var ok = (t.marcas || []).filter(function (m) { return m === 'bien'; }).length;
    $('resumen-pruebas').textContent = (t.marcas && t.marcas.length) ? TEXTOS.resumenPruebas(ok, total) : '';
  }

  // ---------------------------------------------------------------------------------------------
  // Pestaña Memoria.
  // ---------------------------------------------------------------------------------------------
  var subMemoria = 'funciones';
  var temporizadorBusqueda = null;

  function cargarMemoria() {
    var q = $('buscar-memoria').value.trim();
    var lista = $('lista-memoria');
    if (subMemoria === 'funciones') {
      api('GET', '/api/memoria/funciones?q=' + encodeURIComponent(q)).then(function (r) { pintarFunciones(lista, r.datos || []); }).catch(function () {});
    } else if (subMemoria === 'datos') {
      api('GET', '/api/memoria/hechos?q=' + encodeURIComponent(q)).then(function (r) { pintarHechos(lista, r.datos || []); }).catch(function () {});
    } else if (subMemoria === 'palabras') {
      api('GET', '/api/memoria/palabras').then(function (r) { pintarPalabras(lista, r.datos || {}, q); }).catch(function () {});
    } else {
      cargarPendientes(lista);
    }
  }

  function vacioOLista(lista, items, textoVacio) {
    vaciar(lista);
    if (!items.length) { lista.appendChild(el('p', { class: 'vacio' }, textoVacio)); return false; }
    return true;
  }

  function olvidar(tipo, ruta, nombre) {
    confirmar(TEXTOS.olvidarTitulo(nombre), TEXTOS.olvidarTexto, TEXTOS.olvidar, function () {
      api('DELETE', ruta).then(function (r) {
        if (r.status === 204) anunciar(TEXTOS.olvidado(nombre));
        else if (r.datos && r.datos.error) anunciar(r.datos.error);
        cargarMemoria();
        actualizarEstado();
      }).catch(function () {});
    });
  }

  function pintarFunciones(lista, fs) {
    if (!vacioOLista(lista, fs, TEXTOS.nadaFunciones)) return;
    fs.forEach(function (f) {
      var meta = [
        TEXTOS.origen + ': ' + (TEXTOS.origenes[f.origen] || f.origen || '—'),
        fecha(f.creada),
        TEXTOS.usos(f.usos || 0),
        TEXTOS.nivelCorto[f.nivel] || f.nivel || ''
      ].filter(Boolean).join(' · ');
      var casos = f.casos || [];
      var tablaCasos = tabla({
        cabecera: ['Entrada', 'Esperado', TEXTOS.deDonde],
        filas: casos.map(function (c) { return [c.entrada, c.esperado || '—', TEXTOS.deDondeCaso[c.origen] || c.origen || '']; })
      });
      var tarjeta = el('article', { class: 'tarjeta' },
        el('h3', null, f.nombre),
        f.descripcion ? el('p', null, f.descripcion) : null,
        el('code', { class: 'firma' }, f.firmaGo || ''),
        el('p', { class: 'meta' }, meta),
        f.fuente ? el('p', { class: 'meta' }, TEXTOS.fuente + ': ', enlace(f.fuente, dominio(f.fuente) || f.fuente)) : null,
        f.codigo ? el('details', null, el('summary', null, TEXTOS.verCodigo), bloqueCodigo(f.codigo, true)) : null,
        casos.length ? el('details', null, el('summary', null, TEXTOS.verCasos(casos.length)), tablaCasos) : null,
        el('div', { class: 'botones' },
          boton(TEXTOS.usar, function () {
            var e = $('entrada');
            e.value = 'usa ' + f.nombre + ' para ';
            e.focus();
            e.selectionStart = e.selectionEnd = e.value.length;
          }, 'secundario pequeno'),
          boton(TEXTOS.olvidar, function () { olvidar('funcion', '/api/memoria/funciones/' + encodeURIComponent(f.nombre), f.nombre); }, 'secundario pequeno')));
      lista.appendChild(tarjeta);
    });
  }

  function pintarHechos(lista, hs) {
    if (!vacioOLista(lista, hs, TEXTOS.nadaDatos)) return;
    hs.forEach(function (h) {
      var texto = h.texto || [h.sujeto, h.negado ? 'no' : '', (h.relacion || '').replace(/_/g, ' '), h.objeto].filter(Boolean).join(' ');
      var fuente = h.fuente === 'usuario' ? 'tú' : h.fuente;
      lista.appendChild(el('article', { class: 'tarjeta' },
        el('p', null, texto),
        el('p', { class: 'meta' }, TEXTOS.fuente + ': ', esURLSegura(fuente) ? enlace(fuente, dominio(fuente)) : (fuente || '—'),
          h.fecha ? ' · ' + fecha(h.fecha) : '',
          typeof h.confianza === 'number' && h.confianza < 1 ? ' · ' + TEXTOS.confianza + ' ' + Math.round(h.confianza * 100) + ' %' : ''),
        el('div', { class: 'botones' },
          boton(TEXTOS.olvidar, function () { olvidar('hecho', '/api/memoria/hechos/' + encodeURIComponent(h.id), texto); }, 'secundario pequeno'))));
    });
  }

  function pintarPalabras(lista, mapa, q) {
    var claves = Object.keys(mapa).filter(function (k) { return !q || normal(k + ' ' + mapa[k]).indexOf(normal(q)) >= 0; }).sort();
    if (!vacioOLista(lista, claves, TEXTOS.nadaPalabras)) return;
    claves.forEach(function (k) {
      lista.appendChild(el('article', { class: 'tarjeta' },
        el('p', null, el('strong', null, k), ' → ', mapa[k]),
        el('div', { class: 'botones' },
          boton(TEXTOS.olvidar, function () { olvidar('palabra', '/api/memoria/palabras/' + encodeURIComponent(k), k); }, 'secundario pequeno'))));
    });
  }

  function propiedad(o, nombre) {
    if (!o) return undefined;
    if (o[nombre] !== undefined) return o[nombre];
    var minus = nombre.charAt(0).toLowerCase() + nombre.slice(1);
    return o[minus];
  }

  function cargarPendientes(lista) {
    api('GET', '/api/memoria/pendientes').then(function (r) {
      var ps = Array.isArray(r.datos) ? r.datos : [];
      if (!vacioOLista(lista, ps, TEXTOS.nadaPendientes)) return;
      ps.forEach(function (p) {
        var texto = propiedad(p, 'Texto') || '';
        var codigo = propiedad(p, 'Codigo') || '';
        var meta = [propiedad(p, 'Motivo'), TEXTOS.intentos(propiedad(p, 'Intentos') || 0), fecha(propiedad(p, 'Fecha'))].filter(Boolean).join(' · ');
        lista.appendChild(el('article', { class: 'tarjeta' },
          el('p', null, texto || '—'),
          el('p', { class: 'meta' }, meta),
          el('div', { class: 'botones' },
            boton(TEXTOS.reintentar, function () { enviarTexto(texto, codigo); }, 'secundario pequeno'))));
      });
    }).catch(function () {});
  }

  // ---------------------------------------------------------------------------------------------
  // Pestaña Estudiar.
  // ---------------------------------------------------------------------------------------------
  function estudiar() {
    var b = $('b-estudiar');
    b.disabled = true;
    $('estado-estudio').textContent = TEXTOS.estudioEmpieza;
    var pr = $('progreso-estudio');
    pr.removeAttribute('value');
    api('POST', '/api/estudiar', { sesion: sesion }).then(function (r) {
      if (r.status !== 200 || !r.datos || !r.datos.tarea) {
        b.disabled = false;
        $('estado-estudio').textContent = (r.datos && r.datos.error) || TEXTOS.noPudeEnviar;
        return;
      }
      tareaEstudio = seguirTarea(r.datos.tarea, { titulo: TEXTOS.estudiarBoton, estudio: true });
    }).catch(function () { b.disabled = false; });
  }

  function progresoEstudio(p) {
    var texto = p.progreso || p.titulo || '';
    var m = /(\d+)\s*(?:\/|de)\s*(\d+)/.exec(texto);
    var pr = $('progreso-estudio');
    if (m && Number(m[2]) > 0) {
      pr.max = Number(m[2]);
      pr.value = Math.min(Number(m[1]), Number(m[2]));
    }
    if (p.titulo) $('estado-estudio').textContent = p.titulo + (p.progreso ? ' — ' + p.progreso : '');
    avisarPestana('tab-estudiar');
  }

  function terminarEstudio(ctx, fin) {
    $('b-estudiar').disabled = false;
    var pr = $('progreso-estudio');
    if (fin.estado === 'terminada') { pr.max = 1; pr.value = 1; } else { pr.value = 0; }
    var r = ctx.respuesta;
    $('estado-estudio').textContent = (r && r.texto) || (fin.estado === 'terminada' ? TEXTOS.estudioTermina : TEXTOS.loHeParado);
    if (r) pintarRespuesta(ctx, r, fin);
    cargarPendientes($('lista-pendientes'));
    tareaEstudio = null;
  }

  // ---------------------------------------------------------------------------------------------
  // Arranque.
  // ---------------------------------------------------------------------------------------------
  function leerToken() {
    var m = /(?:^#|&)t=([0-9a-fA-F]{16,128})/.exec(location.hash || '');
    if (m) {
      guardar('nyx-token', m[1]);
      try { history.replaceState(null, '', location.pathname + location.search); } catch (e) { location.hash = ''; }
      return m[1];
    }
    return leerGuardado('nyx-token');
  }

  function aplicarTextos() {
    Array.prototype.forEach.call(document.querySelectorAll('[data-texto]'), function (n) {
      var t = TEXTOS[n.getAttribute('data-texto')];
      if (typeof t === 'string') n.textContent = t;
    });
    Array.prototype.forEach.call(document.querySelectorAll('[data-placeholder]'), function (n) {
      var t = TEXTOS[n.getAttribute('data-placeholder')];
      if (typeof t === 'string') n.placeholder = t;
    });
  }

  function iniciar() {
    aplicarTextos();
    token = leerToken();
    if (!token) { mostrarSinToken(); return; }
    sesion = leerGuardado('nyx-sesion');
    if (!sesion || !/^[0-9a-f]{32}$/.test(sesion)) {
      sesion = hexAleatorio(16);
      guardar('nyx-sesion', sesion);
    }

    elegirPestana = pestanas($('pestanas'), mostrarPanel);
    pestanas($('subpestanas'), function (t) {
      subMemoria = t.getAttribute('data-sub');
      $('lista-memoria').setAttribute('aria-labelledby', t.id);
      cargarMemoria();
    });

    var form = $('redactor');
    var entrada = $('entrada');
    form.addEventListener('submit', function (e) {
      e.preventDefault();
      var texto = entrada.value;
      var codigo = $('zona-codigo').hidden ? '' : $('codigo-adjunto').value;
      enviarTexto(texto, codigo).then(function () {
        entrada.value = '';
        entrada.placeholder = TEXTOS.escribe;
        if (codigo) { $('codigo-adjunto').value = ''; abrirZonaCodigo(false); }
      });
    });
    function ctrlEnter(e) {
      if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
        e.preventDefault();
        if (typeof form.requestSubmit === 'function') form.requestSubmit(); else $('b-pensar').click();
      }
    }
    entrada.addEventListener('keydown', ctrlEnter);
    $('codigo-adjunto').addEventListener('keydown', ctrlEnter);
    $('b-pegar').addEventListener('click', function () {
      var abrir = $('zona-codigo').hidden;
      abrirZonaCodigo(abrir);
      if (abrir) $('codigo-adjunto').focus();
    });
    $('b-parar').addEventListener('click', function () {
      if (!tareaChat || tareaChat.terminada) return;
      api('POST', '/api/tareas/' + encodeURIComponent(tareaChat.id) + '/cancelar').catch(function () {});
    });
    $('b-mas').addEventListener('click', function () {
      if (!tareaChat) return;
      var previa = tareaChat;
      burbuja('tu', TEXTOS.pensarMasNota);
      ocupado(true);
      api('POST', '/api/tareas/' + encodeURIComponent(previa.id) + '/pensar-mas').then(function (r) {
        if (r.status !== 200 || !r.datos || !r.datos.tarea) {
          ocupado(false);
          burbuja('nyx', (r.datos && r.datos.error) || TEXTOS.noPudeEnviar, 'error');
          return;
        }
        tareaChat = seguirTarea(r.datos.tarea, { titulo: TEXTOS.pensarMas + ': ' + previa.titulo, peticion: previa.peticion, accion: previa.accion, chat: true });
      }).catch(function () { ocupado(false); });
    });
    $('b-estudiar').addEventListener('click', estudiar);
    $('solo-fallos').addEventListener('change', pintarTablaPruebas);
    $('buscar-memoria').addEventListener('input', function () {
      clearTimeout(temporizadorBusqueda);
      temporizadorBusqueda = setTimeout(cargarMemoria, 250);
    });
    $('panel-traza').addEventListener('scroll', function () {
      var p = $('panel-traza');
      pegadoAbajo = p.scrollHeight - p.scrollTop - p.clientHeight < 40;
    });
    iniciarEditor();
    bienvenida();
    actualizarEstado();
    setInterval(actualizarEstado, 30000);
    entrada.focus();
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', iniciar);
  else iniciar();
})();
