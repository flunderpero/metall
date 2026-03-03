; >>> Keywords

[
  "fun"
  "struct"
  "let"
  "if"
  "else"
  "for"
  "return"
  "new"
  "new_mut"
  "make"
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
(boolean_literal) @constant.builtin
(void) @constant.builtin

; >>> Comments

(line_comment) @comment
(block_comment) @comment

; >>> Types

(simple_type (type_identifier) @type)
(reference_type "&" @operator)

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

; >>> Qualified names (Foo.bar)

(qualified_name (type_identifier) @type)
(qualified_name (identifier) @function)

; >>> Bindings

(let_binding name: (identifier) @variable)
(mut_binding name: (identifier) @variable)
(allocator_binding name: (allocator_identifier) @attribute)
(allocator_binding type: (type_identifier) @type)

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

["+" "-" "*" "/" "%" "==" "!=" "="] @operator

; >>> Punctuation

["(" ")" "{" "}" "[" "]"] @punctuation.bracket
["," "."] @punctuation.delimiter
