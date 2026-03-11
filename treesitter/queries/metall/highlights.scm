; >>> Keywords

[
  "fun"
  "struct"
  "shape"
  "use"
  "let"
  "if"
  "else"
  "for"
  "in"
  "return"
  "and"
  "or"
  "not"
] @keyword

"mut" @keyword

; break/continue are named nodes wrapping the keyword.
(break_expression) @keyword
(continue_expression) @keyword

; >>> Literals

(integer_literal) @number
(string_literal) @string
(rune_literal) @character
(boolean_literal) @constant.builtin
(void) @type

; >>> Comments

(line_comment) @comment
(block_comment) @comment

; >>> Types

(simple_type (type_identifier) @type)
(simple_type "void" @type)
(reference_type "&" @operator)

; >>> Generics

(type_parameter name: (type_identifier) @type.definition)
(type_parameter constraint: (type_identifier) @type)
(type_arguments (simple_type (type_identifier) @type))

; >>> Functions

(function_declaration "fun" @keyword.function)
(function_name (identifier) @function)
(function_name (type_identifier) @type)

; >>> Parameters

(parameter name: (identifier) @variable.parameter)
(parameter name: (allocator_identifier) @attribute)

; >>> Structs

(struct_declaration name: (type_identifier) @type.definition)
(struct_field name: (identifier) @property)
(struct_field name: (allocator_identifier) @attribute)
(struct_literal type: (type_identifier) @type)

; >>> Shapes

(shape_declaration name: (type_identifier) @type.definition)

(fun_signature "fun" @keyword.function)
(fun_signature name: (function_name (identifier) @function))
(fun_signature name: (function_name (type_identifier) @type))

; >>> Qualified names (Foo.bar)

(qualified_name (type_identifier) @type)
(qualified_name (identifier) @function)

; >>> Imports

(import_declaration "use" @keyword.import)
(import_declaration alias: (identifier) @variable)
(import_path (identifier) @module)

; >>> Path expressions (lib::foo, lib::Foo.bar)

(path_expression "::" @punctuation.delimiter)
(path_expression module: (identifier) @module)
(path_expression member: (type_identifier) @type)
(path_expression member: (identifier) @function)

; >>> Bindings

(let_binding name: (identifier) @variable)
(mut_binding name: (identifier) @variable)
(allocator_binding name: (allocator_identifier) @attribute)
(allocator_binding type: (type_identifier) @type)

; >>> For-in binding

(for_expression binding: (identifier) @variable)

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

["+" "-" "*" "/" "%" "==" "!=" "<" "<=" ">" ">=" "=" ".." "..=" "|" "^" "&" "<<" ">>" "~"] @operator

; >>> Punctuation

["(" ")" "{" "}" "[" "]" "<" ">"] @punctuation.bracket
["," "." "::"] @punctuation.delimiter
