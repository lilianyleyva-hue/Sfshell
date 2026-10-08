package main

// ============================================================
//  TALLER · GODOT — su mundo, con el motor de Godot
// ------------------------------------------------------------
//  «godot» en el taller escribe un proyecto de Godot 4 en
//  ~/.local/share/nyx-mundo/taller/godot/ y lo conecta en vivo con el
//  taller: Godot pide al taller lo que hay (las piezas, sus partes
//  moviéndose, sus materiales y texturas) y lo dibuja con su luz, sus
//  sombras y su física (colisiones exactas con la forma de cada cosa).
//  Nyx y Abla siguen construyendo en el taller; Godot lo enseña.
//
//  Usa el modo «Compatibilidad» (OpenGL): va en cualquier gráfica,
//  también en las integradas de Intel.
// ============================================================

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// EscribirGodot: el proyecto, con dónde está el taller y su código.
func (o *tallerObra) EscribirGodot(url, codigo string) (string, error) {
	dir := filepath.Join(o.dir, "godot")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	archivos := map[string]string{
		"project.godot": tallerGodotProyecto,
		"mundo.tscn":    tallerGodotEscena,
		"mundo.gd":      tallerGodotScript,
		"conexion.cfg":  fmt.Sprintf("[taller]\nurl=%q\ncodigo=%q\n", url, codigo),
	}
	for n, c := range archivos {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(c), 0o644); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// tallerAbrirGodot: si Godot está instalado, lo abre con el proyecto.
func tallerAbrirGodot(dir string) (string, bool) {
	for _, g := range []string{"godot4", "godot", "godot-4", "Godot"} {
		if ruta, err := exec.LookPath(g); err == nil {
			cmd := exec.Command(ruta, "--path", dir)
			if cmd.Start() == nil {
				return ruta, true
			}
		}
	}
	if _, err := exec.LookPath("flatpak"); err == nil {
		cmd := exec.Command("flatpak", "run", "org.godotengine.Godot", "--path", dir)
		if out, err := exec.Command("flatpak", "info", "org.godotengine.Godot").CombinedOutput(); err == nil && len(out) > 0 && cmd.Start() == nil {
			return "flatpak org.godotengine.Godot", true
		}
	}
	return "", false
}

const tallerGodotProyecto = `; El mundo de Nyx y Abla (lo escribe el taller).
config_version=5

[application]
config/name="Mundo de Nyx y Abla"
run/main_scene="res://mundo.tscn"
config/features=PackedStringArray("4.3")

[display]
window/size/viewport_width=1280
window/size/viewport_height=720

[rendering]
renderer/rendering_method="gl_compatibility"
renderer/rendering_method.mobile="gl_compatibility"
`

const tallerGodotEscena = `[gd_scene load_steps=2 format=3]

[ext_resource type="Script" path="res://mundo.gd" id="1"]

[node name="Mundo" type="Node3D"]
script = ExtResource("1")
`

const tallerGodotScript = `extends Node3D
# El mundo de Nyx y Abla, con el motor de Godot.
# Lo construyen ellas en el taller (nyx taller); aquí se dibuja, se choca
# y se camina. WASD andar · Mayús correr · Espacio saltar · V volar
# (Espacio sube, C baja) · E usar · Esc soltar el ratón.

var url := "http://127.0.0.1:8471"
var codigo := ""
var instancias := {}   # "pieza~parte" -> {"cuerpo": AnimatableBody3D, "modelo": String, "objetivo": Transform3D}
var mallas := {}       # modelo -> {"mesh": ArrayMesh, "forma": ConcavePolygonShape3D}
var pidiendo := {}     # modelo -> true
var texturas := {}     # nombre -> ImageTexture
var materiales := {}   # nombre -> StandardMaterial3D (para ponerles la textura cuando llegue)
var nsonido := -1
var nllevar := -1
var usar := false
var volar := true
var cayendo := 0.0
var ocupado := false
var marcador := []
var piezas_cerca := []
var ias := {}          # las 36 IAs del mundo 3D: nombre -> {"nodo", "letrero", "objetivo", "rumbo"}
const TRIBUS := [Color(0.95, 0.68, 0.22), Color(0.3, 0.78, 0.95), Color(0.9, 0.35, 0.8), Color(0.4, 0.88, 0.4)]
const NOMBRE_TRIBU := ["constructora", "exploradora", "artista", "jugadora"]
var jugador: CharacterBody3D
var camara: Camera3D
var hud: Label
var charla: Label
var http_motor: HTTPRequest
var http_charla: HTTPRequest

func _ready() -> void:
	_leer_conexion()
	_entorno()
	_crear_jugador()
	_crear_hud()
	http_motor = HTTPRequest.new()
	add_child(http_motor)
	http_motor.request_completed.connect(_al_motor)
	http_charla = HTTPRequest.new()
	add_child(http_charla)
	http_charla.request_completed.connect(_al_charla)
	var t := Timer.new()
	t.wait_time = 0.12
	t.autostart = true
	add_child(t)
	t.timeout.connect(_preguntar)
	var t2 := Timer.new()
	t2.wait_time = 2.0
	t2.autostart = true
	add_child(t2)
	t2.timeout.connect(_pedir_charla)
	var t3 := Timer.new()
	t3.wait_time = 5.0
	t3.autostart = true
	add_child(t3)
	t3.timeout.connect(func(): print("taller: %d piezas a la vista, %d modelos, %d texturas, %d IAs" % [instancias.size(), mallas.size(), texturas.size(), ias.size()]))
	Input.mouse_mode = Input.MOUSE_MODE_CAPTURED

func _leer_conexion() -> void:
	var cfg := ConfigFile.new()
	if cfg.load("res://conexion.cfg") == OK:
		url = str(cfg.get_value("taller", "url", url)).trim_suffix("/")
		codigo = str(cfg.get_value("taller", "codigo", ""))

func _entorno() -> void:
	var env := Environment.new()
	env.background_mode = Environment.BG_COLOR
	env.background_color = Color(0.05, 0.06, 0.1)
	env.ambient_light_source = Environment.AMBIENT_SOURCE_COLOR
	env.ambient_light_color = Color(0.62, 0.64, 0.72)
	env.ambient_light_energy = 0.7
	env.fog_enabled = true
	env.fog_light_color = Color(0.05, 0.06, 0.1)
	env.fog_density = 0.008
	env.glow_enabled = true
	var we := WorldEnvironment.new()
	we.environment = env
	add_child(we)
	var sol := DirectionalLight3D.new()
	sol.rotation_degrees = Vector3(-55, 35, 0)
	sol.light_energy = 0.9
	sol.shadow_enabled = true
	add_child(sol)

func _crear_jugador() -> void:
	jugador = CharacterBody3D.new()
	jugador.position = Vector3(8, 1.5, 8)
	var forma := CollisionShape3D.new()
	var capsula := CapsuleShape3D.new()
	capsula.radius = 0.28
	capsula.height = 1.8
	forma.shape = capsula
	forma.position = Vector3(0, 0.9, 0)
	jugador.add_child(forma)
	camara = Camera3D.new()
	camara.position = Vector3(0, 1.6, 0)
	camara.far = 400.0
	camara.current = true
	jugador.add_child(camara)
	jugador.floor_snap_length = 0.3
	add_child(jugador)

func _crear_hud() -> void:
	var capa := CanvasLayer.new()
	add_child(capa)
	hud = Label.new()
	hud.position = Vector2(14, 10)
	hud.add_theme_color_override("font_shadow_color", Color.BLACK)
	capa.add_child(hud)
	charla = Label.new()
	charla.position = Vector2(14, 420)
	charla.size = Vector2(760, 280)
	charla.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	charla.add_theme_color_override("font_shadow_color", Color.BLACK)
	capa.add_child(charla)

# ---------- caminar, volar, usar ----------

func _unhandled_input(ev: InputEvent) -> void:
	if ev is InputEventMouseMotion and Input.mouse_mode == Input.MOUSE_MODE_CAPTURED:
		jugador.rotate_y(-ev.relative.x * 0.0022)
		camara.rotation.x = clamp(camara.rotation.x - ev.relative.y * 0.0022, -1.4, 1.4)
	elif ev is InputEventMouseButton and ev.pressed:
		Input.mouse_mode = Input.MOUSE_MODE_CAPTURED
	elif ev is InputEventKey and ev.pressed and not ev.echo:
		match ev.keycode:
			KEY_ESCAPE:
				Input.mouse_mode = Input.MOUSE_MODE_VISIBLE
			KEY_V:
				volar = not volar
			KEY_E:
				usar = true

func _physics_process(dt: float) -> void:
	var dir := Vector3.ZERO
	var b := jugador.global_transform.basis
	if Input.is_key_pressed(KEY_W): dir -= b.z
	if Input.is_key_pressed(KEY_S): dir += b.z
	if Input.is_key_pressed(KEY_A): dir -= b.x
	if Input.is_key_pressed(KEY_D): dir += b.x
	dir.y = 0
	dir = dir.normalized()
	var corre := Input.is_key_pressed(KEY_SHIFT)
	var vel := (4.2 if corre else 2.2) * (2.3 if volar else 1.0)
	var v := jugador.velocity
	v.x = dir.x * vel
	v.z = dir.z * vel
	if volar:
		v.y = 0.0
		if Input.is_key_pressed(KEY_SPACE): v.y = 9.0 if corre else 4.5
		if Input.is_key_pressed(KEY_C): v.y = -(9.0 if corre else 4.5)
		cayendo = 0.0
	elif jugador.is_on_floor():
		v.y = 5.4 if Input.is_key_pressed(KEY_SPACE) else 0.0
		cayendo = 0.0
	else:
		v.y -= 18.0 * dt
		cayendo += dt
		if cayendo > 2.5: # en el vacío no hay fondo: a volar
			volar = true
	jugador.velocity = v
	_subir_escalon(dir * vel * dt)
	jugador.move_and_slide()
	# las piezas que se mueven (ascensores, aspas…) van a donde dice el taller
	for k in instancias:
		var i: Dictionary = instancias[k]
		var c: AnimatableBody3D = i["cuerpo"]
		c.global_transform = c.global_transform.interpolate_with(i["objetivo"], min(1.0, dt * 12.0))
	# las IAs van suaves hacia donde dice el taller
	for n in ias:
		var a: Dictionary = ias[n]
		var nodo: Node3D = a["nodo"]
		nodo.global_position = nodo.global_position.lerp(a["objetivo"], min(1.0, dt * 6.0))
		nodo.rotation.y = lerp_angle(nodo.rotation.y, -float(a["rumbo"]), min(1.0, dt * 6.0))

# un escalón (hasta 45 cm) se sube andando
func _subir_escalon(mov: Vector3) -> void:
	if volar or mov.length() < 0.0001 or not jugador.is_on_floor():
		return
	var t := jugador.global_transform
	if not jugador.test_move(t, mov):
		return
	var arriba := t.translated(Vector3(0, 0.45, 0))
	if jugador.test_move(t, Vector3(0, 0.45, 0)) or jugador.test_move(arriba, mov):
		return
	jugador.global_position.y += 0.45
	jugador.apply_floor_snap()

# ---------- lo que dice el taller ----------

func _preguntar() -> void:
	if ocupado:
		return
	ocupado = true
	var p := jugador.global_position
	var q := "%s/api/taller-motor?x=%.2f&y=%.2f&z=%.2f&s=%d&l=%d" % [url, p.x, p.y, p.z, nsonido, nllevar]
	if usar:
		q += "&usar=1"
		usar = false
	var err := http_motor.request(q, ["X-Nyx-Codigo: " + codigo], HTTPClient.METHOD_POST, "")
	if err != OK:
		ocupado = false

func _al_motor(resultado: int, cod: int, _cab: PackedStringArray, cuerpo: PackedByteArray) -> void:
	ocupado = false
	if resultado != HTTPRequest.RESULT_SUCCESS or cod != 200:
		hud.text = "Esperando al taller en %s… (abre «nyx taller» y escribe «godot»)" % url
		return
	var d = JSON.parse_string(cuerpo.get_string_from_utf8())
	if typeof(d) != TYPE_DICTIONARY:
		return
	var vistas := {}
	piezas_cerca = _lista(d, "piezas")
	for p in piezas_cerca:
		for q in _lista(p, "partes"):
			var m: String = q["m"]
			var k: String = str(p["b"]) + m.substr(m.rfind("~"))
			vistas[k] = true
			var f: Array = q["f"]
			var tr := Transform3D(Basis(Vector3(f[0], f[1], f[2]), Vector3(f[4], f[5], f[6]), Vector3(f[8], f[9], f[10])), Vector3(f[12], f[13], f[14]))
			if instancias.has(k) and instancias[k]["modelo"] == m:
				instancias[k]["objetivo"] = tr
				continue
			if not mallas.has(m):
				_pedir_malla(m)
				continue
			if instancias.has(k):
				instancias[k]["cuerpo"].queue_free()
			instancias[k] = {"cuerpo": _crear_instancia(m, tr), "modelo": m, "objetivo": tr}
	for k in instancias.keys():
		if not vistas.has(k):
			instancias[k]["cuerpo"].queue_free()
			instancias.erase(k)
	var nl := int(d.get("nllevar", 0))
	if d.has("llevar") and nllevar >= 0 and nl > nllevar:
		var l: Array = d["llevar"]
		jugador.global_position = Vector3(l[0], l[1], l[2])
		jugador.velocity = Vector3.ZERO
	nllevar = maxi(nllevar, nl)
	nsonido = maxi(nsonido, int(d.get("nsonido", 0)))
	marcador = _lista(d, "puntos")
	_actualizar_ias(_lista(d, "ias"))
	_actualizar_hud()

# ---------- las 36 IAs del mundo 3D ----------

func _actualizar_ias(lista: Array) -> void:
	var vistas := {}
	for a in lista:
		var n := str(a.get("n", ""))
		vistas[n] = true
		if not ias.has(n):
			ias[n] = _crear_ia(n, int(a.get("t", 0)), Vector3(a["x"], a["y"], a["z"]))
		var e: Dictionary = ias[n]
		e["objetivo"] = Vector3(a["x"], a["y"], a["z"])
		e["rumbo"] = float(a.get("r", 0.0))
		var dice := str(a.get("d", ""))
		var letrero: Label3D = e["letrero"]
		letrero.text = n if dice == "" else "%s\n«%s»" % [n, dice]
	for n in ias.keys():
		if not vistas.has(n):
			ias[n]["nodo"].queue_free()
			ias.erase(n)

func _crear_ia(n: String, t: int, pos: Vector3) -> Dictionary:
	var nodo := Node3D.new()
	add_child(nodo)
	nodo.global_position = pos
	var mat := StandardMaterial3D.new()
	mat.albedo_color = TRIBUS[t % 4]
	var cuerpo := MeshInstance3D.new()
	var capsula := CapsuleMesh.new()
	capsula.radius = 0.25
	capsula.height = 1.3
	cuerpo.mesh = capsula
	cuerpo.material_override = mat
	cuerpo.position = Vector3(0, 0.65, 0)
	nodo.add_child(cuerpo)
	var cabeza := MeshInstance3D.new()
	var esfera := SphereMesh.new()
	esfera.radius = 0.2
	esfera.height = 0.4
	cabeza.mesh = esfera
	var claro := StandardMaterial3D.new()
	claro.albedo_color = TRIBUS[t % 4].lightened(0.4)
	cabeza.material_override = claro
	cabeza.position = Vector3(0, 1.5, 0)
	nodo.add_child(cabeza)
	var ojo_mat := StandardMaterial3D.new()
	ojo_mat.emission_enabled = true
	ojo_mat.emission = Color(1, 1, 1)
	for z in [-0.08, 0.08]:
		var ojo := MeshInstance3D.new()
		var bola := SphereMesh.new()
		bola.radius = 0.04
		bola.height = 0.08
		ojo.mesh = bola
		ojo.material_override = ojo_mat
		ojo.position = Vector3(0.18, 1.53, z)
		nodo.add_child(ojo)
	var letrero := Label3D.new()
	letrero.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	letrero.position = Vector3(0, 2.1, 0)
	letrero.font_size = 28
	letrero.pixel_size = 0.008
	letrero.modulate = TRIBUS[t % 4].lightened(0.5)
	letrero.text = n
	letrero.width = 420
	letrero.autowrap_mode = TextServer.AUTOWRAP_WORD
	nodo.add_child(letrero)
	return {"nodo": nodo, "letrero": letrero, "objetivo": pos, "rumbo": 0.0}

# una lista del taller (si viene vacía, llega como null)
func _lista(d: Dictionary, k: String) -> Array:
	var v = d.get(k)
	return v if v is Array else []

func _crear_instancia(m: String, tr: Transform3D) -> AnimatableBody3D:
	var c := AnimatableBody3D.new()
	c.sync_to_physics = false
	var mi := MeshInstance3D.new()
	mi.mesh = mallas[m]["mesh"]
	c.add_child(mi)
	if mallas[m]["forma"] != null:
		var col := CollisionShape3D.new()
		col.shape = mallas[m]["forma"]
		c.add_child(col)
	add_child(c)
	c.global_transform = tr
	return c

func _pedir_malla(m: String) -> void:
	if pidiendo.has(m) or pidiendo.size() > 6:
		return
	pidiendo[m] = true
	var h := HTTPRequest.new()
	add_child(h)
	h.request_completed.connect(func(r, cod, _c, cuerpo):
		pidiendo.erase(m)
		h.queue_free()
		if r == HTTPRequest.RESULT_SUCCESS and cod == 200:
			var d = JSON.parse_string(cuerpo.get_string_from_utf8())
			if typeof(d) == TYPE_DICTIONARY:
				mallas[m] = _construir_malla(d)
	)
	h.request("%s/api/figura?id=%s" % [url, m.uri_encode()])

# la malla de un modelo: un trozo por material (con su textura), otro para
# lo que brilla, y la forma exacta para chocar
func _construir_malla(d: Dictionary) -> Dictionary:
	var pos: Array = d["pos"]
	var nor: Array = d["nor"]
	var col: Array = d["col"]
	var emi: Array = d["emi"]
	var tm: Array = _lista(d, "tm")
	var mats: Array = _lista(d, "mats")
	var sol: Array = _lista(d, "sol")
	var grupos := {}
	var caras := PackedVector3Array()
	var nt := pos.size() / 9
	for t in nt:
		var k := -1
		if t < tm.size():
			k = int(tm[t])
		if k < 0 and float(emi[t * 3]) > 3.5:
			k = -2
		if not grupos.has(k):
			grupos[k] = [PackedVector3Array(), PackedVector3Array(), PackedColorArray(), PackedVector2Array()]
		var g: Array = grupos[k]
		for v in range(t * 3, t * 3 + 3):
			var p := Vector3(pos[v * 3], pos[v * 3 + 1], pos[v * 3 + 2])
			var n := Vector3(nor[v * 3], nor[v * 3 + 1], nor[v * 3 + 2])
			g[0].append(p)
			g[1].append(n)
			g[2].append(Color(col[v * 3], col[v * 3 + 1], col[v * 3 + 2]))
			g[3].append(_uv(p, n))
			if t >= sol.size() or int(sol[t]) == 1:
				caras.append(p)
	var mesh := ArrayMesh.new()
	for k in grupos:
		var g: Array = grupos[k]
		var arr := []
		arr.resize(Mesh.ARRAY_MAX)
		arr[Mesh.ARRAY_VERTEX] = g[0]
		arr[Mesh.ARRAY_NORMAL] = g[1]
		arr[Mesh.ARRAY_COLOR] = g[2]
		arr[Mesh.ARRAY_TEX_UV] = g[3]
		mesh.add_surface_from_arrays(Mesh.PRIMITIVE_TRIANGLES, arr)
		var mat := StandardMaterial3D.new()
		mat.cull_mode = BaseMaterial3D.CULL_DISABLED
		if k == -2:
			mat.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
			mat.vertex_color_use_as_albedo = true
		elif k == -1:
			mat.vertex_color_use_as_albedo = true
			mat.roughness = 0.85
		else:
			mat.albedo_texture = _textura(str(mats[k]), mat)
			mat.roughness = 0.8
		mesh.surface_set_material(mesh.get_surface_count() - 1, mat)
	var forma = null
	if caras.size() >= 3:
		forma = ConcavePolygonShape3D.new()
		forma.backface_collision = true
		forma.set_faces(caras)
	return {"mesh": mesh, "forma": forma}

func _uv(p: Vector3, n: Vector3) -> Vector2:
	var a := n.abs()
	if a.y >= a.x and a.y >= a.z:
		return Vector2(p.x, p.z) / 2.0
	if a.x >= a.z:
		return Vector2(p.z, p.y) / 2.0
	return Vector2(p.x, p.y) / 2.0

func _textura(nombre: String, mat: StandardMaterial3D) -> Texture2D:
	if texturas.has(nombre):
		return texturas[nombre]
	if not materiales.has(nombre):
		materiales[nombre] = []
		var h := HTTPRequest.new()
		add_child(h)
		h.request_completed.connect(func(r, cod, _c, cuerpo):
			h.queue_free()
			if r == HTTPRequest.RESULT_SUCCESS and cod == 200:
				var img := Image.new()
				if img.load_png_from_buffer(cuerpo) == OK:
					img.generate_mipmaps()
					var tex := ImageTexture.create_from_image(img)
					texturas[nombre] = tex
					for m in materiales[nombre]:
						m.albedo_texture = tex
		)
		h.request("%s/api/taller-textura?n=%s" % [url, nombre.uri_encode()])
	materiales[nombre].append(mat)
	return null

# ---------- lo que se dicen ----------

func _pedir_charla() -> void:
	if http_charla.get_http_client_status() == HTTPClient.STATUS_DISCONNECTED:
		http_charla.request(url + "/api/taller")

func _al_charla(resultado: int, cod: int, _cab: PackedStringArray, cuerpo: PackedByteArray) -> void:
	if resultado != HTTPRequest.RESULT_SUCCESS or cod != 200:
		return
	var d = JSON.parse_string(cuerpo.get_string_from_utf8())
	if typeof(d) != TYPE_DICTIONARY:
		return
	var lineas := []
	var c: Array = _lista(d, "charla")
	for f in c.slice(max(0, c.size() - 7)):
		var t := str(f.get("texto", ""))
		if t.length() > 160:
			t = t.substr(0, 160) + "…"
		lineas.append("%s: %s" % [f.get("quien", ""), t])
	charla.text = "\n".join(lineas)

func _actualizar_hud() -> void:
	var p := jugador.global_position
	var l := ["Nyx y Abla · altura %.1f m%s" % [p.y, " · volando" if volar else ""]]
	var cerca := ""
	for pz in piezas_cerca:
		if pz.get("usar", false) and Vector2(pz["x"] - p.x, pz["z"] - p.z).length() < 4.0:
			cerca = str(pz.get("titulo", ""))
	if cerca != "":
		l.append("E: usar «%s»" % cerca)
	l.append("WASD andar · Espacio saltar · V volar (Espacio/C) · E usar · Esc ratón")
	if marcador.size() > 0:
		l.append("Puntos: " + " · ".join(PackedStringArray(marcador)))
	if ias.size() > 0:
		l.append("%d IAs a la vista" % ias.size())
	hud.text = "\n".join(l)
`
