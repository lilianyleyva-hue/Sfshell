package reparar

import "testing"

// Golden Spanish translations: 46 compiler/vet messages and 15 runtime panics.

var doradoTraducir = []struct{ msg, es string }{
	{"declared and not used: x", "Creaste la variable `x` pero nunca la usas. (línea 3)"},
	{"x declared and not used", "Creaste la variable `x` pero nunca la usas. (línea 3)"},
	{"\"os\" imported and not used", "Importas `os` pero no lo usas. (línea 3)"},
	{"\"math/rand\" imported as rnd and not used", "Importas `math/rand` pero no lo usas. (línea 3)"},
	{"label fuera defined and not used", "La etiqueta `fuera` está definida pero no se usa. (línea 3)"},
	{"undefined: strings", "Usas `strings` pero no lo importaste. (línea 3)"},
	{"undefined: total", "No existe nada llamado `total`: ¿lo escribiste bien o te faltó crearlo? (línea 3)"},
	{"undefined: strings.Fieldz", "`strings.Fieldz` no existe: `strings` no tiene nada llamado `Fieldz`. (línea 3)"},
	{"name toUpper not exported by package strings", "`strings.toUpper` no se puede usar: en Go solo se usan desde fuera los nombres que empiezan por mayúscula. (línea 3)"},
	{"p.nombre undefined (type Persona has no field or method nombre, but does have field Nombre)", "El tipo `Persona` no tiene `nombre`, pero sí tiene `Nombre`: fíjate en las mayúsculas. (línea 3)"},
	{"xs.length undefined (type []int has no field or method length)", "El tipo `[]int` no tiene nada llamado `length`. (línea 3)"},
	{"x redeclared in this block", "`x` ya estaba declarado antes en este mismo bloque. (línea 3)"},
	{"no new variables on left side of :=", "Todas las variables a la izquierda de `:=` ya existían: usa `=` para cambiarlas. (línea 3)"},
	{"non-name p.X on left side of :=", "Con `:=` solo se crean variables nuevas; para cambiar `p.X` usa `=`. (línea 3)"},
	{"missing return", "A la función le falta un `return` al final. (línea 3)"},
	{"not enough return values\n\thave (error)\n\twant (int, error)", "A este `return` le faltan valores: la función devuelve `(int, error)`. (línea 3)"},
	{"too many return values\n\thave (int)\n\twant ()", "Este `return` devuelve valores, pero la función no devuelve nada. (línea 3)"},
	{"cannot use nil as int value in return statement", "`nil` no vale como `int` en el return: usa su valor vacío (0, \"\" o false). (línea 3)"},
	{"cannot use \"hola\" (untyped string constant) as int value in assignment", "El valor `\"hola\"` no vale como `int` en una asignación. (línea 3)"},
	{"cannot use suma / len(xs) (value of type int) as float64 value in assignment", "`suma / len(xs)` es de tipo `int`, pero aquí hace falta un `float64` en una asignación. (línea 3)"},
	{"cannot use x (variable of type int) as string value in argument to strings.ToUpper", "`x` es de tipo `int`, pero aquí hace falta un `string` al llamar a `strings.ToUpper`. (línea 3)"},
	{"assignment mismatch: 1 variable but strconv.Atoi returns 2 values", "`strconv.Atoi` devuelve 2 valores, pero recoges 1. (línea 3)"},
	{"assignment mismatch: 2 variables but 1 value", "A la izquierda hay 2 variables y a la derecha 1 valor: tiene que haber los mismos. (línea 3)"},
	{"non-boolean condition in if statement", "La condición del `if` tiene que ser verdadero o falso (por ejemplo `x != 0`). (línea 3)"},
	{"invalid operation: r == \"a\" (mismatched types rune and untyped string)", "En `r == \"a\"` comparas un carácter con un texto: un solo carácter se escribe entre comillas simples, como `'a'`. (línea 3)"},
	{"invalid operation: a + b (mismatched types int and float64)", "No se puede operar `int` con `float64` directamente en `a + b`: convierte uno de los dos (por ejemplo `float64(x)`). (línea 3)"},
	{"invalid operation: operator ! not defined on n (variable of type int)", "El operador `!` no se puede usar con `n`. (línea 3)"},
	{"invalid operation: division by zero", "Estás dividiendo entre cero. (línea 3)"},
	{"cannot assign to struct field m[quien].Saldo in map", "No se puede cambiar `m[quien].Saldo` directamente: cópialo, cámbialo y guárdalo otra vez en el mapa. (línea 3)"},
	{"cannot assign to s[0] (neither addressable nor a map index expression)", "No se puede cambiar `s[0]`: si es un texto (string), en Go los textos no se pueden modificar letra a letra. (línea 3)"},
	{"x + 1 (value of type int) is not used", "Calculas `x + 1` pero no guardas ni usas el resultado. (línea 3)"},
	{"too many arguments in call to f\n\thave (int, int)\n\twant (int)", "Le pasas demasiados valores a `f`. (línea 3)"},
	{"not enough arguments in call to f\n\thave ()\n\twant (int)", "Le faltan valores a la llamada a `f`. (línea 3)"},
	{"cannot range over n (variable of type int)", "No se puede recorrer `n` con `range`. (línea 3)"},
	{"first argument to append must be a slice; have x (variable of type int)", "El primer valor de `append` tiene que ser una lista. (línea 3)"},
	{"invalid operation: cannot call non-function x (variable of type int)", "`x` no es una función: no se puede llamar con paréntesis. (línea 3)"},
	{"function main is undeclared in the main package", "El paquete `main` necesita una función `main`. (línea 3)"},
	{"missing function body", "La función no tiene cuerpo: faltan las llaves `{ … }`. (línea 3)"},
	{"expected declaration, found x", "Fuera de una función solo puede haber declaraciones (func, var, const, type, import); encontré `x`. (línea 3)"},
	{"expected ';', found 'else'", "Esperaba `;` y encontré `else`. (línea 3)"},
	{"string literal not terminated", "Un texto no tiene las comillas de cierre. (línea 3)"},
	{"illegal character U+0023 '#'", "El carácter `#` no vale en Go: los comentarios empiezan con `//`. (línea 3)"},
	{"syntax error: unexpected newline, expected comma or )", "Error de escritura: encontré el final de la línea cuando esperaba una coma `,` o `)`. (línea 3)"},
	{"fmt.Printf format %d has arg s of wrong type string", "En el `Printf`, `%d` no sirve para `s`, que es de tipo `string`. (línea 3)"},
	{"unreachable code", "Este código nunca se ejecuta. (línea 3)"},
	{"algo rarísimo que no conozco", "El compilador dice: algo rarísimo que no conozco (línea 3)"},
}

var doradoPanico = []struct{ msg, es string }{
	{"runtime error: index out of range [3] with length 3", "Intentaste leer la posición 3 de una lista que solo tiene 3 elementos (van de 0 a 2)."},
	{"runtime error: index out of range [0] with length 0", "Intentaste leer la posición 0 de una lista vacía."},
	{"runtime error: index out of range [5] with length 1", "Intentaste leer la posición 5 de una lista que solo tiene 1 elemento (la posición 0)."},
	{"runtime error: index out of range [-1]", "Intentaste leer la posición -1, que no existe: las posiciones empiezan en 0."},
	{"runtime error: slice bounds out of range [:5] with capacity 3", "Intentaste cortar la lista con [:5], pero solo tiene 3 elementos."},
	{"runtime error: slice bounds out of range [3:2]", "Intentaste cortar la lista con [3:2]: el principio no puede ir después del final."},
	{"runtime error: integer divide by zero", "Dividiste un número entre cero."},
	{"runtime error: invalid memory address or nil pointer dereference", "Usaste un puntero que vale nil (no apunta a nada)."},
	{"assignment to entry in nil map", "Intentaste guardar algo en un mapa que no se creó: hace falta `make(map[…]…)`."},
	{"runtime: goroutine stack exceeds 1000000000-byte limit", "La función se llama a sí misma sin parar (recursión sin fin) y se acabó la memoria."},
	{"all goroutines are asleep - deadlock!", "El programa se quedó esperando para siempre (bloqueo)."},
	{"strconv.Atoi: parsing \"abc\": invalid syntax", "El texto `abc` no es un número."},
	{"interface conversion: interface {} is string, not int", "Una conversión de tipo falló: el valor es `string` y no `int`."},
	{"runtime error: makeslice: len out of range", "Intentaste crear una lista de tamaño negativo o demasiado grande."},
	{"algo", "El programa se detuvo con este error: algo"},
}

func TestTraducirDorado(t *testing.T) {
	if len(doradoTraducir) < 40 {
		t.Fatalf("solo hay %d mensajes dorados", len(doradoTraducir))
	}
	for _, d := range doradoTraducir {
		if got := Traducir(errorEn(3, d.msg)); got != d.es {
			t.Errorf("Traducir(%q)\n  = %q\nquiero %q", d.msg, got, d.es)
		}
	}
}

func TestTraducirPanicoDorado(t *testing.T) {
	for _, d := range doradoPanico {
		if got := TraducirPanico(d.msg); got != d.es {
			t.Errorf("TraducirPanico(%q)\n  = %q\nquiero %q", d.msg, got, d.es)
		}
	}
}
