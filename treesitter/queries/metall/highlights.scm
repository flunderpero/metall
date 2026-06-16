; >>> Keywords

[
  "fun"
  "struct"
  "shape"
  "union"
  "enum"
  "use"
  "let"
  "pub"
  "extern"
  "export"
  "nocopy"
  "noescape"
  "sync"
  "unsync"
  "unsafe"
  "if"
  "else"
  "for"
  "in"
  "match"
  "when"
  "case"
  "try"
  "is"
  "return"
  "defer"
  "and"
  "or"
  "not"
] @keyword

"mut" @keyword

; break/continue are named nodes wrapping the keyword.
(break_expression) @keyword
(continue_expression) @keyword

; `of`/`uninit` are contextual keywords: highlighted only inside array
; construction, plain identifiers (caught by the fallback below) elsewhere.
(array_construction "of" @keyword)
(array_construction "uninit" @keyword)

; >>> Literals

(integer_literal) @number
; String bodies, quotes, and literal fragments are @string. The modifiers+sigils
; (`f`/`b`/`m` and `#`) and the interpolation braces are punctuation, kept apart
; from the body. Interpolation expressions are left alone so their own (deeper)
; rules apply, making `{expr}` read as code.
(string_content) @string
(fstring_start) @string
(fstring_end) @string
(string_fragment) @string
(string_prefix) @punctuation.special
(fstring_prefix) @punctuation.special
(string_suffix) @punctuation.special
(interp_start) @punctuation.special
(interp_end) @punctuation.special
(rune_literal) @character
(boolean_literal) @constant.builtin
(void) @type

; >>> Comments

(line_comment) @comment
(block_comment) @comment

; >>> Types

(simple_type (type_identifier) @type)
(simple_type "never" @type)
(module_qualified_type (identifier) @module)
(reference_type "&" @operator)

; >>> Generics

(type_parameter name: (type_identifier) @type.definition)
(type_arguments (simple_type (type_identifier) @type))
(type_parameters ["<" ">"] @punctuation.bracket)
(type_arguments ["<" ">"] @punctuation.bracket)

; Comparison operators (binary_expression uses bare "<" / ">").
(binary_expression "<" @operator)
(binary_expression ">" @operator)

; >>> Functions

(function_declaration "fun" @keyword.function)
(extern_function_declaration "fun" @keyword.function)
(export_declaration name: (identifier) @function)
(function_name (identifier) @function)
(function_name (type_identifier) @type)

; >>> Parameters

(parameter name: (identifier) @variable.parameter)
(parameter name: (allocator_identifier) @attribute)
(function_literal_parameter name: (identifier) @variable.parameter)
(function_literal_parameter name: (allocator_identifier) @attribute)
(named_argument name: (identifier) @variable.parameter)

; >>> Structs

(struct_declaration name: (type_identifier) @type.definition)
(struct_field name: (identifier) @property)
(struct_field name: (allocator_identifier) @attribute)
(type_construction type: (type_identifier) @type)

; >>> Shapes

(shape_declaration name: (type_identifier) @type.definition)

(fun_signature "fun" @keyword.function)
(fun_signature name: (function_name (identifier) @function))
(fun_signature name: (function_name (type_identifier) @type))

; >>> Unions

(union_declaration name: (type_identifier) @type.definition)

; >>> Enums

(enum_declaration name: (type_identifier) @type.definition)
(enum_variant name: (identifier) @constant)
(enum_variant_pattern variant: (identifier) @constant)

; >>> Function types

(function_type "fun" @keyword.function)

; >>> Top-level macro invocation

(macro_invocation module: (identifier) @module)

; >>> Conditional compilation — placed after the generic identifier/keyword
; fallbacks so the `#if ... #end` overrides win (tree-sitter picks the
; last-declared matching capture). The entire directive (`#`, `if`/`end`,
; and the whole condition) reads as a comment so `#if ...` blocks visually
; recede like preprocessor noise.

; >>> Qualified names (Foo.bar)

(qualified_name (type_identifier) @type)
(qualified_name (identifier) @function)

; >>> Imports

(import_declaration "use" @keyword.import)
(import_declaration alias: (identifier) @variable)
(import_path (identifier) @module)

; >>> Module member access (lib.foo, lib.Foo.bar) — handled by field_access

; >>> Bindings

(let_binding name: (identifier) @variable)
(mut_binding name: (identifier) @variable)
(allocator_binding name: (allocator_identifier) @attribute)

; >>> For-in binding

(for_expression binding: (identifier) @variable)
(for_expression index: (identifier) @variable)

; >>> Match bindings

(match_arm binding: (identifier) @variable)
(match_arm pattern: (simple_type (type_identifier) @type))
(match_else binding: (identifier) @variable)

; >>> Try bindings

(try_expression binding: (identifier) @variable)

; >>> Assignment

(assignment target: (identifier) @variable)

; >>> Calls

(call_expression callee: (identifier) @function.call)
(call_expression callee: (field_access field: (identifier) @function.method))

; >>> Field access

(field_access field: (identifier) @property)
(field_access field: (allocator_identifier) @attribute)

; >>> References

(reference "&" @operator)
(reference (identifier) @variable)

; >>> Dereference

(dereference "*" @operator)

; >>> Fallback identifiers (lowest priority)

(identifier) @variable
(allocator_identifier) @attribute
(type_identifier) @type

; >>> Operators

["+" "-" "*" "/" "%" "+%" "-%" "*%" "==" "!=" "<=" ">=" "=" ".." "..=" "|" "^" "&" "<<" "~" "?" "!"] @operator

; >>> Punctuation

["(" ")" "{" "}" "[" "]"] @punctuation.bracket
["," "." ":"] @punctuation.delimiter

; >>> Conditional compilation overrides (last so they win)

(compile_if_declaration "#" @comment)
(compile_if_declaration "if" @comment)
(compile_if_declaration "end" @comment)
(compile_if_expression "#" @comment)
(compile_if_expression "if" @comment)
(compile_if_expression "end" @comment)
(compile_condition_flag (identifier) @comment)
(compile_condition_flag "." @comment)
(compile_condition_not "not" @comment)
(compile_condition_and "and" @comment)
(compile_condition_or "or" @comment)
