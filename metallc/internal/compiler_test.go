package internal

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/base"
)

func TestCompile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		src        string
		wantOutput string
	}{
		{"happy path", `fun main() void { print_str("hello") }`, "hello\n"},
		{"int constant", `fun main() void { print_int(123) }`, "123\n"},

		{"str variable", `fun main() void { let a = "hello" print_str(a) }`, "hello\n"},
		{"int variable", `fun main() void { let a = 123 print_int(a) }`, "123\n"},
		{"bool variable", `fun main() void { let a = true print_bool(a) }`, "true\n"},
		{"mut variable same scope", `fun main() void { mut a = 123 print_int(a) a = 456 print_int(a) }`, "123\n456\n"},

		{"int function", `fun get() Int { 123 } fun main() void { print_int(get()) }`, "123\n"},
		{"str function", `fun get() Str { "hello" } fun main() void { print_str(get()) }`, "hello\n"},
		{"bool function", `fun get() Bool { true } fun main() void { print_bool(get()) }`, "true\n"},
		{"fun int param", `fun foo(a Int) Int { a } fun main() void { print_int(foo(123)) }`, "123\n"},
		{"fun str param", `fun foo(a Str) Str { a } fun main() void { let s = foo("hello") print_str(s) }`, "hello\n"},
		{"fun bool param", `fun foo(a Bool) Bool { a } fun main() void { print_bool(foo(true)) }`, "true\n"},

		{"block expr", `fun main() void { let s = { "hello" } print_str(s) }`, "hello\n"},
		{"var block expr is void", `fun main() void { print_str("hello") let a = 123 }`, "hello\n"},
		{"assign block expr is void", `fun main() void { print_str("hello") mut a = 123 a = 321 }`, "hello\n"},

		{"if expr", `fun main() void { let a = if true { 123 } else { 321 } print_int(a) }`, "123\n"},
		{"if expr else", `fun main() void { let a = if false { 123 } else { 321 } print_int(a) }`, "321\n"},
		{
			"if expr var",
			`fun main() void { mut a = 1 if true { a = 123 } else { a = 321 } print_int(a) }`,
			"123\n",
		},
		{"nested if exper", `
			fun main() void {
				let a = if true {
					if false { 1 } else { 123 }
				} else {
					2
				}
				print_int(a)
			}
			`, "123\n"},

		{
			"ref/deref",
			`fun main() void { mut a = 123 mut b = &mut a print_int(*b) *b = 321 print_int(a) }`,
			"123\n321\n",
		},
		{"nested ref/deref", `
			fun main() void { 
				mut a = 123 
				mut b = &mut a
				mut c = &mut b
				print_int(*b)
				*b = 321 
				print_int(a)
				**c = 111
				print_int(a)
			}`, "123\n321\n111\n"},
		{"assign through mut ref parameter", `
			fun foo(a &mut Int) void { 
				print_int(*a)
				*a = 321 
			}
			fun main() void { 
				mut a = 123 
				foo(&mut a)
				print_int(a)
			}
			`, "123\n321\n"},

		{"struct", `
			struct Planet {
				mut name Str
				mut diameter Int
			}

			fun main() void {
				mut earth = Planet("Earth", 12500)
				print_str(earth.name)
				print_int(earth.diameter)

				earth.name = "Mother"
				earth.diameter = 12742
				print_str(earth.name)
				print_int(earth.diameter)
			}
			`, "Earth\n12500\nMother\n12742\n"},

		{"struct as value parameter", `
			struct Planet {
				name Str
			}

			fun print_planet(p Planet) void {
				print_str(p.name)
			}

			fun main() void {
				let earth = Planet("Earth")
				print_planet(earth)
			}
			`, "Earth\n"},

		{"struct as ref parameter, struct ref auto-derefence, mut ref coercion", `
			struct Planet {
				mut name Str
			}

			fun print_planet(p &Planet) void {
				print_str(p.name)
			}

			fun update(p &mut Planet, new_name Str) void {
				p.name = new_name
			}

			fun main() void {
				mut earth = Planet("Earth")
				print_planet(&earth)

				update(&mut earth, "Mother")
				print_planet(&earth)
			}
			`, "Earth\nMother\n"},

		{"struct as value return", `
			struct Planet {
				name Str
			}

			fun make_earth() Planet {
				Planet("Earth")
			}

			fun main() void {
				let earth = make_earth()
				print_str(earth.name)
			}
			`, "Earth\n"},

		{"nested struct", `
			struct Planet {
				mut name Str
			}

			struct SolarSystem {
				earth Planet
				mut mars Planet
			}

			fun main() void {
				mut s = SolarSystem(Planet("Earth"), Planet("Mars"))
				print_str(s.earth.name)
				print_str(s.mars.name)
				s.mars.name = "God of War"
				print_str(s.mars.name)
			}
			`, "Earth\nMars\nGod of War\n"},

		{"struct copy on assignment", `
			struct Planet {
				mut name Str
			}

			fun main() void {
				mut a = Planet("Earth")
				mut b = a
				b.name = "Mars"
				print_str(a.name)
				print_str(b.name)
			}
			`, "Earth\nMars\n"},

		{"assign struct to nested struct field copies value", `
			struct Inner {
				mut name Str
			}

			struct Outer {
				mut inner Inner
			}

			fun main() void {
				mut a = Outer(Inner("Earth"))
				mut replacement = Inner("Mars")
				a.inner = replacement
				replacement.name = "Venus"
				print_str(a.inner.name)
				print_str(replacement.name)
			}
			`, "Mars\nVenus\n"},

		{"struct with ref field", `
			struct Wrapper {
				value Int
				ptr &Int
			}

			fun main() void {
				mut x = 42
				let w = Wrapper(1, &x)
				print_int(w.value)
				print_int(*w.ptr)
				x = 99
				print_int(*w.ptr)
			}
			`, "1\n42\n99\n"},

		{"struct ref aliases", `
			struct Planet {
				mut name Str
			}

			fun main() void {
				mut a = Planet("Earth")
				let b = &a
				let c = b
				a.name = "Mars"
				print_str(c.name)
			}
			`, "Mars\n"},

		{"struct from if else", `
			struct Planet {
				mut name Str
			}

			fun main() void {
				let p = if true { Planet("Earth") } else { Planet("Mars") }
				print_str(p.name)
				mut q = if false { Planet("Earth") } else { Planet("Mars") }
				print_str(q.name)
				q.name = "Venus"
				print_str(q.name)
			}
			`, "Earth\nMars\nVenus\n"},

		{"struct reassign from if else", `
			struct Planet {
				name Str
			}

			fun main() void {
				mut p = Planet("Earth")
				print_str(p.name)
				p = if true { Planet("Mars") } else { Planet("Venus") }
				print_str(p.name)
			}
			`, "Earth\nMars\n"},

		{"struct block expr as arg", `
			struct Planet {
				name Str
			}

			fun print_planet(p Planet) void {
				print_str(p.name)
			}

			fun main() void {
				print_planet({ Planet("Earth") })
			}
			`, "Earth\n"},

		{"forward declare", `
			fun main() void {
				print_int(foo())
			}

			fun foo() Int {
				123
			}

			`, "123\n"},

		{"allocator", `
			struct Planet {
				name Str
			}

			fun make_saturn(@a Arena) &Planet {
				let p = @a Planet("Saturn")
				&p
			}

			fun main() void {
				alloc @a = Arena()
				let earth = @a Planet("Earth")
				let mars = @a Planet("Mars")
				{
					alloc @b = Arena()
					let venus = @b Planet("Venus")
					print_str(venus.name)
				}
				print_str(mars.name)
				print_str(earth.name)
				let saturn = make_saturn(@a)
				print_str(saturn.name)
			}
			`, "Venus\nMars\nEarth\nSaturn\n"},

		{"int array", `
			fun main() void {
				let number = [1, 2, 3]
				print_int(number[2])
				print_int(number[1])
				print_int(number[0])
			}
			`, "3\n2\n1\n"},

		{"struct array", `
			struct Planet {
				name Str
			}

			fun main() void {
				let planets = [
					Planet("Earth"),
					Planet("Mars"),
					Planet("Venus"),
				]
				print_str(planets[2].name)
				print_str(planets[1].name)
				print_str(planets[0].name)
			}
			`, "Venus\nMars\nEarth\n"},
		{"nested array", `
			fun main() void {
				let nested = [
					[1, 2],
					[3, 4],
					[5, 6],
				]
				let first = nested[0]
				print_int(first[1])
				let second = nested[1]
				print_int(second[0])
				let third = nested[2]
				print_int(third[1])
			}
			`, "2\n3\n6\n"},
		{"array in struct", `
			struct Numbers {
				values [3]Int
			}

			fun main() void {
				let n = Numbers([1, 2, 3])
				print_int(n.values[1])
			}
			`, "2\n"},
		{"array with refs", `
			struct Planet {
			 	name Str
			}

			fun main() void {
				let earth = Planet("Earth")
				let mars = Planet("Mars")
				let planets = [earth, mars]
				print_str(planets[1].name)
				print_str(planets[0].name)

				let one = 1
				let two = 2
				let nums = [&one, &two]
				print_int(*nums[1])
				print_int(*nums[0])
			}
			`, "Mars\nEarth\n2\n1\n"},
		{"assign to array index", `
			fun main() void {
				mut a = [1, 2, 3]
				print_int(a[1])
				a[1] = 4
				print_int(a[1])
			}
			`, "2\n4\n"},
		{"assign struct to array index", `
			struct Planet { name Str }

			fun main() void {
				mut planets = [Planet("Earth"), Planet("Mars")]
				print_str(planets[0].name)
				planets[0] = Planet("Venus")
				print_str(planets[0].name)
			}
			`, "Earth\nVenus\n"},
		{"assign ref struct to array index", `
			struct Planet { name Str }

			fun main() void {
				let earth = Planet("Earth")
				let mars = Planet("Mars")
				let venus = Planet("Venus")
				mut planets = [&earth, &mars]
				print_str(planets[0].name)
				planets[0] = &venus
				print_str(planets[0].name)
			}
			`, "Earth\nVenus\n"},
		{"array alloc", `
			fun main() void {
				alloc @a = Arena()
				mut numbers = @a [5]Int()
				numbers[1] = 1
				numbers[2] = 2

				print_int(numbers[0])
				print_int(numbers[1])
				print_int(numbers[2])
			}
			`, "0\n1\n2\n"},
	}

	assert := base.NewAssert(t)
	hasOnly := false
	for _, tt := range tests {
		if strings.HasPrefix(tt.name, "!"+"only") {
			hasOnly = true
			break
		}
	}
	_ = os.RemoveAll(".build")
	if err := os.MkdirAll(".build", 0o700); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		if hasOnly && !strings.HasPrefix(tt.name, "!"+"only") {
			continue
		}
		name := strings.TrimSpace(strings.ReplaceAll(tt.name, "!"+"only", ""))
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			source := base.NewSource("test.met", []rune(tt.src))
			reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
			outputPath := "./.build/" + reg.ReplaceAllString(name, "_")
			opts := CompileOpts{
				Listener:         nil,
				Output:           outputPath,
				KeepIR:           true,
				LLVMPasses:       "verify," + DefaultLLVMPasses,
				AddressSanitizer: true,
			}
			exitCode, output, err := CompileAndRun(t.Context(), source, opts)
			assert.NoError(err)
			assert.Equal(0, exitCode)
			assert.Equal(tt.wantOutput, output)
		})
	}
}
