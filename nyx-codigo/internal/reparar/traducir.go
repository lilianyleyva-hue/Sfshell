package reparar

import (
	"regexp"
	"strconv"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// plantilla turns one compiler message into simple Spanish.
type plantilla struct {
	re *regexp.Regexp
	fn func(m []string, msg string) string
}

var plantillas []plantilla

func p(expr string, fn func(m []string, msg string) string) {
	plantillas = append(plantillas, plantilla{re: regexp.MustCompile(expr), fn: fn})
}

// fija returns a template that ignores the match.
func fija(s string) func([]string, string) string { return func([]string, string) string { return s } }

func codigo(s string) string { return "`" + s + "`" }

// contexto translates the "in …" part of "cannot use" messages.
func contexto(s string) string {
	switch {
	case s == "assignment":
		return "en una asignación"
	case s == "return statement":
		return "en el return"
	case s == "variable declaration":
		return "al crear la variable"
	case strings.HasPrefix(s, "argument to "):
		return "al llamar a " + codigo(strings.TrimPrefix(s, "argument to "))
	case strings.HasPrefix(s, "struct literal"):
		return "al rellenar la estructura"
	case strings.HasPrefix(s, "array or slice literal"), strings.HasPrefix(s, "slice literal"):
		return "dentro de la lista"
	case strings.HasPrefix(s, "map literal"):
		return "dentro del mapa"
	case strings.HasPrefix(s, "send"):
		return "al enviar por el canal"
	}
	return "(" + s + ")"
}

// quiere extracts "want (int, error)" from a multi-line message.
func quiere(msg string) string {
	if i := strings.Index(msg, "want ("); i >= 0 {
		s := msg[i+5:]
		if j := strings.IndexByte(s, '\n'); j >= 0 {
			s = s[:j]
		}
		return strings.TrimSpace(s)
	}
	return ""
}

func encontrado(s string) string {
	s = strings.Trim(s, "'")
	switch s {
	case "EOF":
		return "el final del archivo"
	case "newline":
		return "el final de la línea"
	}
	return codigo(s)
}

func init() {
	// unused things
	p(`^declared and not used: (\w+)$`, func(m []string, _ string) string {
		return "Creaste la variable " + codigo(m[1]) + " pero nunca la usas."
	})
	p(`^(\w+) declared (?:and|but) not used$`, func(m []string, _ string) string {
		return "Creaste la variable " + codigo(m[1]) + " pero nunca la usas."
	})
	p(`^"([^"]+)" imported(?: as (\w+))? and not used$`, func(m []string, _ string) string {
		return "Importas " + codigo(m[1]) + " pero no lo usas."
	})
	p(`^label (\w+) defined and not used$`, func(m []string, _ string) string {
		return "La etiqueta " + codigo(m[1]) + " está definida pero no se usa."
	})
	// undefined names
	p(`^undefined: (\w+)\.(\w+)$`, func(m []string, _ string) string {
		return codigo(m[1]+"."+m[2]) + " no existe: " + codigo(m[1]) + " no tiene nada llamado " + codigo(m[2]) + "."
	})
	p(`^name (\w+) not exported by package (\w+)$`, func(m []string, _ string) string {
		return codigo(m[2]+"."+m[1]) + " no se puede usar: en Go solo se usan desde fuera los nombres que empiezan por mayúscula."
	})
	p(`^undefined: (\w+)$`, func(m []string, _ string) string {
		if _, ok := paquetesStd[m[1]]; ok {
			return "Usas " + codigo(m[1]) + " pero no lo importaste."
		}
		return "No existe nada llamado " + codigo(m[1]) + ": ¿lo escribiste bien o te faltó crearlo?"
	})
	p(`^(.+)\.(\w+) undefined \(type (.+) has no field or method (\w+), but does have (?:field|method) (\w+)\)$`, func(m []string, _ string) string {
		return "El tipo " + codigo(m[3]) + " no tiene " + codigo(m[4]) + ", pero sí tiene " + codigo(m[5]) + ": fíjate en las mayúsculas."
	})
	p(`^(.+)\.(\w+) undefined \(type (.+) has no field or method (\w+)\)$`, func(m []string, _ string) string {
		return "El tipo " + codigo(m[3]) + " no tiene nada llamado " + codigo(m[4]) + "."
	})
	p(`^(\w+) redeclared in this block`, func(m []string, _ string) string {
		return codigo(m[1]) + " ya estaba declarado antes en este mismo bloque."
	})
	p(`^no new variables on left side of :=$`, fija("Todas las variables a la izquierda de `:=` ya existían: usa `=` para cambiarlas."))
	p(`^non-name (.+) on left side of :=$`, func(m []string, _ string) string {
		return "Con `:=` solo se crean variables nuevas; para cambiar " + codigo(m[1]) + " usa `=`."
	})
	// returns
	p(`^missing return$`, fija("A la función le falta un `return` al final."))
	p(`^not enough return values`, func(_ []string, msg string) string {
		if q := quiere(msg); q != "" {
			return "A este `return` le faltan valores: la función devuelve " + codigo(q) + "."
		}
		return "A este `return` le faltan valores."
	})
	p(`^too many return values`, func(_ []string, msg string) string {
		if q := quiere(msg); q != "" && q != "()" {
			return "Este `return` devuelve más valores de los que la función promete: " + codigo(q) + "."
		}
		return "Este `return` devuelve valores, pero la función no devuelve nada."
	})
	// types
	p(`^cannot use nil as (.+?) value in (.+)$`, func(m []string, _ string) string {
		return "`nil` no vale como " + codigo(m[1]) + " " + contexto(m[2]) + ": usa su valor vacío (0, \"\" o false)."
	})
	p(`^cannot use (.+?) \(untyped (\w+) constant(?: [^)]*)?\) as (.+?) value in (.+?)(?: \(truncated\))?$`, func(m []string, _ string) string {
		return "El valor " + codigo(m[1]) + " no vale como " + codigo(m[3]) + " " + contexto(m[4]) + "."
	})
	p(`^cannot use (.+?) \((?:variable|value|constant) of (?:\w+ )?type (.+?)\) as (.+?) value in (.+?)(?:: .*)?$`, func(m []string, _ string) string {
		return codigo(m[1]) + " es de tipo " + codigo(m[2]) + ", pero aquí hace falta un " + codigo(m[3]) + " " + contexto(m[4]) + "."
	})
	p(`^assignment mismatch: (\d+) variables? but (.+) returns (\d+) values?$`, func(m []string, _ string) string {
		return codigo(m[2]) + " devuelve " + m[3] + " valores, pero recoges " + m[1] + "."
	})
	p(`^assignment mismatch: (\d+) variables? but (\d+) values?$`, func(m []string, _ string) string {
		return "A la izquierda hay " + m[1] + " variables y a la derecha " + m[2] + " valores: tiene que haber los mismos."
	})
	p(`^non-boolean condition in (\w+) statement$`, func(m []string, _ string) string {
		return "La condición del " + codigo(m[1]) + " tiene que ser verdadero o falso (por ejemplo `x != 0`)."
	})
	p(`^invalid operation: (.+) \(mismatched types (rune|byte|int32|uint8) and untyped string\)$`, func(m []string, _ string) string {
		return "En " + codigo(m[1]) + " comparas un carácter con un texto: un solo carácter se escribe entre comillas simples, como `'a'`."
	})
	p(`^invalid operation: (.+) \(mismatched types (.+) and (.+)\)$`, func(m []string, _ string) string {
		return "No se puede operar " + codigo(m[2]) + " con " + codigo(m[3]) + " directamente en " + codigo(m[1]) + ": convierte uno de los dos (por ejemplo `float64(x)`)."
	})
	p(`^invalid operation: operator (\S+) not defined on (.+?) \((.+)\)$`, func(m []string, _ string) string {
		return "El operador " + codigo(m[1]) + " no se puede usar con " + codigo(m[2]) + "."
	})
	p(`^invalid operation: division by zero$`, fija("Estás dividiendo entre cero."))
	p(`^cannot assign to struct field (.+) in map$`, func(m []string, _ string) string {
		return "No se puede cambiar " + codigo(m[1]) + " directamente: cópialo, cámbialo y guárdalo otra vez en el mapa."
	})
	p(`^cannot assign to (.+?) \(neither addressable nor a map index expression\)$`, func(m []string, _ string) string {
		return "No se puede cambiar " + codigo(m[1]) + ": si es un texto (string), en Go los textos no se pueden modificar letra a letra."
	})
	p(`^(.+) \((?:value|variable) of type (.+)\) is not used$`, func(m []string, _ string) string {
		return "Calculas " + codigo(m[1]) + " pero no guardas ni usas el resultado."
	})
	p(`^too many arguments in call to (.+)$`, func(m []string, _ string) string {
		return "Le pasas demasiados valores a " + codigo(m[1]) + "."
	})
	p(`^not enough arguments in call to (.+)$`, func(m []string, _ string) string {
		return "Le faltan valores a la llamada a " + codigo(m[1]) + "."
	})
	p(`^invalid argument: index (.+?) \(.+\) must be integer$`, func(m []string, _ string) string {
		return "La posición " + codigo(m[1]) + " tiene que ser un número entero."
	})
	p(`^invalid argument: index (.+?) out of bounds \[0:(\d+)\]$`, func(m []string, _ string) string {
		return "La posición " + codigo(m[1]) + " no existe: la lista solo tiene " + m[2] + " elementos."
	})
	p(`^cannot range over (.+?) \((.+)\)$`, func(m []string, _ string) string {
		return "No se puede recorrer " + codigo(m[1]) + " con `range`."
	})
	p(`^first argument to append must be a slice`, fija("El primer valor de `append` tiene que ser una lista."))
	p(`^(?:invalid operation: )?cannot call non-function (.+?)(?: \(.*\))?$`, func(m []string, _ string) string {
		return codigo(m[1]) + " no es una función: no se puede llamar con paréntesis."
	})
	p(`^(.+?) is not a type$`, func(m []string, _ string) string { return codigo(m[1]) + " no es un tipo." })
	p(`^could not import (\S+)`, func(m []string, _ string) string {
		return "No encuentro el paquete " + codigo(m[1]) + "."
	})
	p(`^package (\S+) is not in std`, func(m []string, _ string) string {
		return "No encuentro el paquete " + codigo(m[1]) + ": no es de la biblioteca estándar."
	})
	p(`^cannot use _ as value$`, fija("`_` no se puede leer: solo sirve para descartar valores."))
	p(`^break is not in a loop, switch, or select$`, fija("`break` solo se puede usar dentro de un bucle o un switch."))
	p(`^function main is undeclared in the main package$`, fija("El paquete `main` necesita una función `main`."))
	p(`^missing function body$`, fija("La función no tiene cuerpo: faltan las llaves `{ … }`."))
	// syntax (go/parser and cmd/compile)
	p(`^expected declaration, found (.+)$`, func(m []string, _ string) string {
		return "Fuera de una función solo puede haber declaraciones (func, var, const, type, import); encontré " + encontrado(m[1]) + "."
	})
	p(`^syntax error: non-declaration statement outside function body$`, fija("Fuera de una función solo puede haber declaraciones (func, var, const, type, import)."))
	p(`^(?:expected statement, found 'else'|syntax error: else must be followed by if or statement block|syntax error: unexpected else.*)$`,
		fija("`else` tiene que ir en la misma línea que la `}` que cierra el if."))
	p(`^(?:unexpected semicolon or newline before \{|syntax error: unexpected semicolon or newline before \{)$`,
		fija("La `{` tiene que estar en la misma línea que la función, el if o el for."))
	p(`^(?:missing ',' before newline in composite literal|syntax error: unexpected newline in composite literal; possibly missing comma or \})$`,
		fija("Falta una coma `,` al final de la línea dentro de la lista."))
	p(`^(?:missing ',' (?:before newline )?in argument list|syntax error: unexpected newline in argument list; possibly missing comma or \))$`,
		fija("Falta una coma `,` o un paréntesis `)` en la llamada."))
	p(`^(?:string literal not terminated|newline in string|raw string literal not terminated)$`, fija("Un texto no tiene las comillas de cierre."))
	p(`^(?:illegal rune literal|more than one character in rune literal)$`, fija("Entre comillas simples solo va un carácter; para un texto usa comillas dobles."))
	p(`^illegal character U\+0023 '#'$`, fija("El carácter `#` no vale en Go: los comentarios empiezan con `//`."))
	p(`^expected '(.+)', found (.+)$`, func(m []string, _ string) string {
		return "Esperaba " + codigo(m[1]) + " y encontré " + encontrado(m[2]) + "."
	})
	p(`^syntax error: unexpected (.+?), expected (.+)$`, func(m []string, _ string) string {
		return "Error de escritura: encontré " + encontrado(m[1]) + " cuando esperaba " + m[2] + "."
	})
	p(`^syntax error: unexpected (.+)$`, func(m []string, _ string) string {
		return "Error de escritura: no esperaba " + encontrado(m[1]) + " aquí."
	})
	// vet
	p(`^(?:fmt\.)?(\w+) format %(\w) has arg (.+) of wrong type (.+)$`, func(m []string, _ string) string {
		return "En el " + codigo(m[1]) + ", " + codigo("%"+m[2]) + " no sirve para " + codigo(m[3]) + ", que es de tipo " + codigo(m[4]) + "."
	})
	p(`^(?:fmt\.)?(\w+) call has possible (?:Printf )?formatting directive (%\w)`, func(m []string, _ string) string {
		return codigo(m[1]) + " no entiende " + codigo(m[2]) + ": para eso usa " + codigo("fmt.Printf") + "."
	})
	p(`^unreachable code$`, fija("Este código nunca se ejecuta."))
	p(`^self-assignment of (.+) to (.+)$`, func(m []string, _ string) string {
		return "Asignas " + codigo(m[1]) + " a sí misma: no hace nada."
	})
}

// Traducir explains a compiler (or vet) error in simple Spanish, with its line. Messages without a
// template become "El compilador dice: <msg> (línea N)".
func Traducir(e nucleo.ErrorGo) string {
	msg := strings.TrimSpace(e.Msg)
	primera := primeraLinea(msg)
	linea := ""
	if e.Linea > 0 {
		linea = " (línea " + strconv.Itoa(e.Linea) + ")"
	}
	for _, pl := range plantillas {
		if m := pl.re.FindStringSubmatch(primera); m != nil {
			return pl.fn(m, msg) + linea
		}
	}
	return "El compilador dice: " + primera + linea
}

var (
	reIndice    = regexp.MustCompile(`index out of range \[(-?\d+)\] with length (\d+)`)
	reCorte     = regexp.MustCompile(`slice bounds out of range \[([^\]]*)\](?: with (?:length|capacity) (\d+))?`)
	reAtoi      = regexp.MustCompile(`strconv\.(?:Atoi|ParseInt|ParseFloat): parsing "(.*)": (invalid syntax|value out of range)`)
	reInterfaz  = regexp.MustCompile(`interface conversion: (.+) is (.+), not (.+)`)
	reMakeslice = regexp.MustCompile(`makeslice: (len|cap) out of range`)
)

// TraducirPanico explains a runtime panic in simple Spanish.
func TraducirPanico(msg string) string {
	m := strings.TrimSpace(msg)
	switch {
	case reIndice.MatchString(m):
		x := reIndice.FindStringSubmatch(m)
		i, _ := strconv.Atoi(x[1])
		n, _ := strconv.Atoi(x[2])
		switch {
		case i < 0:
			return "Intentaste leer la posición " + x[1] + ", que no existe: las posiciones empiezan en 0."
		case n == 0:
			return "Intentaste leer la posición " + x[1] + " de una lista vacía."
		case n == 1:
			return "Intentaste leer la posición " + x[1] + " de una lista que solo tiene 1 elemento (la posición 0)."
		}
		return "Intentaste leer la posición " + x[1] + " de una lista que solo tiene " + x[2] + " elementos (van de 0 a " + strconv.Itoa(n-1) + ")."
	case reCorte.MatchString(m):
		x := reCorte.FindStringSubmatch(m)
		if x[2] != "" {
			return "Intentaste cortar la lista con [" + x[1] + "], pero solo tiene " + x[2] + " elementos."
		}
		return "Intentaste cortar la lista con [" + x[1] + "]: el principio no puede ir después del final."
	case strings.Contains(m, "integer divide by zero"):
		return "Dividiste un número entre cero."
	case strings.Contains(m, "nil pointer dereference") || strings.Contains(m, "invalid memory address"):
		return "Usaste un puntero que vale nil (no apunta a nada)."
	case strings.Contains(m, "assignment to entry in nil map"):
		return "Intentaste guardar algo en un mapa que no se creó: hace falta `make(map[…]…)`."
	case strings.Contains(m, "stack overflow") || strings.Contains(m, "stack exceeds") || strings.Contains(m, "Hondo"):
		return "La función se llama a sí misma sin parar (recursión sin fin) y se acabó la memoria."
	case strings.Contains(m, "SinComb") || strings.Contains(m, "sin combustible"):
		return "La función tardó demasiado: puede que un bucle no termine nunca."
	case strings.Contains(m, "all goroutines are asleep") || strings.Contains(m, "deadlock"):
		return "El programa se quedó esperando para siempre (bloqueo)."
	case strings.Contains(m, "out of memory") || strings.Contains(m, "cannot allocate memory"):
		return "El programa intentó usar demasiada memoria."
	case reMakeslice.MatchString(m):
		return "Intentaste crear una lista de tamaño negativo o demasiado grande."
	case reAtoi.MatchString(m):
		x := reAtoi.FindStringSubmatch(m)
		if x[2] == "value out of range" {
			return "El número " + codigo(x[1]) + " es demasiado grande."
		}
		return "El texto " + codigo(x[1]) + " no es un número."
	case reInterfaz.MatchString(m):
		x := reInterfaz.FindStringSubmatch(m)
		return "Una conversión de tipo falló: el valor es " + codigo(x[2]) + " y no " + codigo(x[3]) + "."
	case strings.Contains(m, "close of closed channel") || strings.Contains(m, "send on closed channel"):
		return "Se usó un canal que ya estaba cerrado."
	case strings.Contains(m, "close of nil channel"):
		return "Se intentó cerrar un canal que no se creó."
	}
	if m == "" {
		return "El programa se detuvo con un error."
	}
	return "El programa se detuvo con este error: " + m
}
