// Tree-sitter grammar for the Metall programming language.
//
// Usage:
//   tree-sitter generate
//   tree-sitter test
//
// Compile the parser for Neovim:
//   cc -shared -fPIC -O2 -o parser/metall.so src/parser.c -I src

const PREC = {
  ASSIGN: 1,
  OR: 2,
  AND: 3,
  BIT_OR: 4,
  BIT_XOR: 5,
  BIT_AND: 6,
  COMPARE: 7,
  SHIFT: 8,
  ADD: 9,
  MUL: 10,
  UNARY: 11,
  POSTFIX: 12,
};

module.exports = grammar({
  name: "metall",

  extras: ($) => [/\s/, $.line_comment, $.block_comment],

  word: ($) => $.identifier,

  conflicts: ($) => [
    [$.qualified_name],
    [$.simple_type],
    [$.call_expression],
    [$.function_declaration],
  ],

  rules: {
    source_file: ($) => seq(repeat($.import_declaration), repeat($._declaration)),

    _declaration: ($) =>
      choice($.function_declaration, $.extern_function_declaration, $.export_declaration, $.struct_declaration, $.shape_declaration, $.union_declaration, $.let_binding, $.compile_if_declaration),

    // >>> Conditional compilation

    compile_if_declaration: ($) =>
      seq(
        "#", token.immediate("if"),
        field("condition", $._compile_condition),
        repeat(choice($.import_declaration, $._declaration)),
        "#", token.immediate("end"),
      ),

    compile_if_expression: ($) =>
      seq(
        "#", token.immediate("if"),
        field("condition", $._compile_condition),
        repeat($._expression),
        "#", token.immediate("end"),
      ),

    _compile_condition: ($) =>
      choice(
        $.compile_condition_flag,
        $.compile_condition_not,
        $.compile_condition_and,
        $.compile_condition_or,
      ),

    compile_condition_flag: ($) =>
      seq(field("category", $.identifier), ".", field("key", $.identifier)),

    compile_condition_not: ($) =>
      seq("not", field("operand", $._compile_condition)),

    compile_condition_and: ($) =>
      prec.left(PREC.AND, seq(
        field("left", $._compile_condition),
        "and",
        field("right", $._compile_condition),
      )),

    compile_condition_or: ($) =>
      prec.left(PREC.OR, seq(
        field("left", $._compile_condition),
        "or",
        field("right", $._compile_condition),
      )),

    // >>> Imports

    import_declaration: ($) =>
      seq(
        "use",
        optional(seq(field("alias", $.identifier), "=")),
        field("path", $.import_path),
      ),

    import_path: ($) =>
      seq($.identifier, repeat(seq(".", $.identifier))),

    // >>> Comments
    // Line comments: `-- ...`
    // Block comments: `--- ... ---`
    // The line comment regex requires a non-dash char after `--` to avoid
    // consuming `---` which starts a block comment.

    line_comment: (_) => token(seq("--", /[^-]/, /[^\n]*/)),

    block_comment: (_) =>
      token(seq("---", repeat(choice(/[^-]/, /-[^-]/, /--[^-]/)), "---")),

    // >>> Identifiers

    identifier: (_) => /[a-z][a-zA-Z0-9_]*/,

    type_identifier: (_) => /[A-Z][a-zA-Z0-9_]*/,

    allocator_identifier: (_) => /@[a-z][a-zA-Z0-9_]*/,

    // >>> Literals

    integer_literal: (_) => /[0-9]+/,

    string_literal: (_) => seq('"', /([^"\\]|\\.)*/, '"'),

    rune_literal: (_) => seq("'", /([^'\\]|\\.|\{[^}]*\})+/, "'"),

    boolean_literal: (_) => choice("true", "false"),

    void: (_) => "void",

    // >>> Generics
    // Type parameters on declarations: fun foo<T Showable>(...) ...
    // Type arguments on expressions/types: foo<Int>(...), Box<Int>
    //
    // The opening `<` uses token.immediate to require no whitespace before
    // it, mirroring the real parser's LtImmediate token. This disambiguates
    // `foo<Int>(x)` (type args) from `foo < Int` (comparison).

    type_parameters: ($) => seq(
      token.immediate("<"),
      $.type_parameter,
      repeat(seq(",", $.type_parameter)),
      ">",
    ),

    type_parameter: ($) => seq(
      optional(choice("sync", "unsync")),
      field("name", $.type_identifier),
      field("constraint", optional($.type_identifier)),
      optional(seq("=", field("default", $._type))),
    ),

    type_arguments: ($) => seq(
      token.immediate("<"),
      $._type,
      repeat(seq(",", $._type)),
      ">",
    ),

    // >>> Types

    _type: ($) =>
      choice($.simple_type, $.array_type, $.slice_type, $.reference_type),

    simple_type: ($) => choice(
      seq($.type_identifier, optional($.type_arguments)),
      "void",
      "never",
    ),

    array_type: ($) => seq("[", $.integer_literal, "]", $._type),

    slice_type: ($) => seq("[", "]", optional("mut"), $._type),

    reference_type: ($) => seq("&", optional("mut"), $._type),

    // >>> Function declaration

    function_declaration: ($) =>
      seq(
        optional("pub"),
        optional("unsync"),
        optional("unsafe"),
        "fun",
        field("name", $.function_name),
        optional(field("type_parameters", $.type_parameters)),
        "(", field("parameters", optional($.parameter_list)), ")",
        optional("noescape"),
        field("return_type", $._type),
        field("body", $.block),
      ),

    extern_function_declaration: ($) =>
      seq(
        "extern",
        optional(seq("(", field("link_name", $.string_literal), ")")),
        "fun",
        field("name", $.function_name),
        optional(field("type_parameters", $.type_parameters)),
        "(", field("parameters", optional($.parameter_list)), ")",
        optional("noescape"),
        field("return_type", $._type),
      ),

    // >>> Export declaration
    // `export <c_name> = <target>` exposes a Metall function under an
    // unmangled C symbol.

    export_declaration: ($) =>
      prec.right(seq(
        "export",
        field("name", $.identifier),
        "=",
        field("target", $._expression),
      )),

    function_name: ($) =>
      choice(
        $.identifier,
        seq($.type_identifier, ".", $.identifier),
      ),

    parameter_list: ($) => seq($.parameter, repeat(seq(",", $.parameter))),

    parameter: ($) =>
      seq(
        field("name", choice($.identifier, $.allocator_identifier)),
        optional(choice("sync", "noescape")),
        field("type", $._type),
        optional(seq("=", field("default", $._expression))),
      ),

    // >>> Struct declaration

    struct_declaration: ($) =>
      seq(
        optional("pub"),
        optional("nocopy"),
        optional(choice("sync", "unsync")),
        "struct",
        field("name", $.type_identifier),
        optional(field("type_parameters", $.type_parameters)),
        "{", repeat($.struct_field), "}",
      ),

    struct_field: ($) =>
      seq(
        optional("pub"),
        field("name", choice($.identifier, $.allocator_identifier)),
        field("type", $._type),
      ),

    // >>> Shape declaration

    shape_declaration: ($) =>
      seq(
        optional("pub"),
        "shape",
        field("name", $.type_identifier),
        "{",
        repeat($.struct_field),
        repeat($.fun_signature),
        "}",
      ),

    fun_signature: ($) =>
      seq(
        optional("pub"),
        "fun",
        field("name", $.function_name),
        optional(field("type_parameters", $.type_parameters)),
        "(", field("parameters", optional($.parameter_list)), ")",
        optional("noescape"),
        field("return_type", $._type),
      ),

    // >>> Union declaration

    union_declaration: ($) =>
      seq(
        optional("pub"),
        optional("nocopy"),
        optional(choice("sync", "unsync")),
        "union",
        field("name", $.type_identifier),
        optional(field("type_parameters", $.type_parameters)),
        "=",
        field("variants", $.union_variants),
      ),

    union_variants: ($) =>
      prec.left(seq($._type, repeat1(seq("|", $._type)))),

    // >>> Blocks

    block: ($) => seq("{", repeat($._expression), "}"),

    // >>> Expressions

    _expression: ($) =>
      choice(
        // Literals and atoms.
        $.integer_literal,
        $.string_literal,
        $.rune_literal,
        $.boolean_literal,
        $.void,
        $.identifier,
        $.allocator_identifier,
        $.grouped_expression,
        $.array_literal,
        $.empty_slice,
        $.block,

        // Declarations (can appear inside blocks).
        $.function_declaration,
        $.struct_declaration,
        $.shape_declaration,
        $.union_declaration,

        // Type-prefixed expressions.
        $.qualified_name,
        $.type_construction,

        // Bindings and assignment.
        $.let_binding,
        $.mut_binding,
        $.allocator_binding,
        $.assignment,

        // Operators.
        $.binary_expression,
        $.unary_expression,

        // Postfix.
        $.call_expression,
        $.field_access,
        $.index_expression,
        $.reference,
        $.dereference,

        // Function literal / closure.
        $.function_literal,

        // Conditional compilation.
        $.compile_if_expression,

        // Control flow.
        $.if_expression,
        $.when_expression,
        $.for_expression,
        $.match_expression,
        $.try_expression,
        $.return_expression,
        $.break_expression,
        $.continue_expression,
      ),

    // >>> Type-prefixed expressions

    // TypeName.methodName or TypeName.methodName<Args>
    qualified_name: ($) =>
      prec(PREC.POSTFIX, seq(
        $.type_identifier, ".", $.identifier,
        optional($.type_arguments),
      )),

    // TypeName(args...) or TypeName<Args>(args...)
    type_construction: ($) =>
      prec(PREC.POSTFIX, seq(
        field("type", $.type_identifier),
        optional(field("type_arguments", $.type_arguments)),
        "(", field("arguments", optional($.argument_list)), ")",
      )),

    // >>> Bindings

    let_binding: ($) =>
      prec.right(PREC.ASSIGN,
        seq("let", field("name", $.identifier), "=", field("value", $._expression))),

    mut_binding: ($) =>
      prec.right(PREC.ASSIGN,
        seq("mut", field("name", $.identifier), "=", field("value", $._expression))),

    allocator_binding: ($) =>
      prec.right(PREC.ASSIGN, seq(
        "let",
        field("name", $.allocator_identifier),
        "=",
        field("type", $.type_identifier),
        "(", field("arguments", optional($.argument_list)), ")",
      )),

    // >>> Assignment

    assignment: ($) =>
      prec.right(PREC.ASSIGN, seq(
        field("target", $._assignable),
        "=",
        field("value", $._expression),
      )),

    _assignable: ($) =>
      choice($.identifier, $.field_access, $.index_expression, $.dereference),

    // >>> Binary expressions

    binary_expression: ($) =>
      choice(
        prec.left(PREC.OR, seq($._expression, "or", $._expression)),
        prec.left(PREC.AND, seq($._expression, "and", $._expression)),
        prec.left(PREC.BIT_OR, seq($._expression, "|", $._expression)),
        prec.left(PREC.BIT_XOR, seq($._expression, "^", $._expression)),
        prec.left(PREC.BIT_AND, seq($._expression, "&", $._expression)),
        prec.left(PREC.COMPARE, seq($._expression, "==", $._expression)),
        prec.left(PREC.COMPARE, seq($._expression, "!=", $._expression)),
        prec.left(PREC.COMPARE, seq($._expression, "<", $._expression)),
        prec.left(PREC.COMPARE, seq($._expression, "<=", $._expression)),
        prec.left(PREC.COMPARE, seq($._expression, ">", $._expression)),
        prec.left(PREC.COMPARE, seq($._expression, ">=", $._expression)),
        prec.left(PREC.SHIFT, seq($._expression, "<<", $._expression)),
        prec.left(PREC.SHIFT, seq($._expression, ">>", $._expression)),
        prec.left(PREC.ADD, seq($._expression, "+", $._expression)),
        prec.left(PREC.ADD, seq($._expression, "-", $._expression)),
        prec.left(PREC.ADD, seq($._expression, "+%", $._expression)),
        prec.left(PREC.ADD, seq($._expression, "-%", $._expression)),
        prec.left(PREC.MUL, seq($._expression, "*", $._expression)),
        prec.left(PREC.MUL, seq($._expression, "/", $._expression)),
        prec.left(PREC.MUL, seq($._expression, "%", $._expression)),
        prec.left(PREC.MUL, seq($._expression, "*%", $._expression)),
      ),

    unary_expression: ($) =>
      choice(
        prec.right(PREC.UNARY, seq("not", $._expression)),
        prec.right(PREC.UNARY, seq("~", $._expression)),
      ),

    // >>> Postfix expressions

    call_expression: ($) =>
      prec(PREC.POSTFIX, seq(
        optional("unsafe"),
        field("callee", $._expression),
        optional(field("type_arguments", $.type_arguments)),
        "(", field("arguments", optional($.argument_list)), ")",
      )),

    argument_list: ($) =>
      seq($._expression, repeat(seq(",", $._expression))),

    field_access: ($) =>
      prec.left(PREC.POSTFIX, seq(
        field("object", $._expression),
        ".",
        field("field", choice($.identifier, $.type_identifier, $.allocator_identifier)),
      )),

    index_expression: ($) =>
      prec(PREC.POSTFIX, seq(
        field("object", $._expression),
        "[", field("index", $._expression), "]",
      )),

    reference: ($) => prec(PREC.UNARY, seq("&", optional("mut"), $._expression)),

    dereference: ($) =>
      prec.left(PREC.POSTFIX, seq($._expression, ".", "*")),

    // >>> Control flow

    if_expression: ($) =>
      prec.right(seq(
        "if",
        field("condition", $._expression),
        field("then", $.block),
        optional(seq("else", field("else", $.block))),
      )),

    when_expression: ($) =>
      seq(
        "when",
        "{",
        repeat1($.when_case),
        optional($.when_else),
        "}",
      ),

    when_case: ($) =>
      seq(
        "case",
        field("condition", $._expression),
        ":",
        repeat($._expression),
      ),

    when_else: ($) =>
      seq(
        "else",
        ":",
        repeat($._expression),
      ),

    for_expression: ($) =>
      choice(
        // Unconditional: `for { ... }` — higher precedence so `{` is not
        // parsed as a block expression in condition position.
        prec(1, seq("for", field("body", $.block))),
        seq("for", field("condition", $._expression), field("body", $.block)),
        seq(
          "for",
          field("binding", $.identifier),
          "in",
          field("range", $.range),
          field("body", $.block),
        ),
      ),

    range: ($) =>
      seq(
        field("lo", $._expression),
        choice("..", "..="),
        field("hi", $._expression),
      ),

    // >>> Match expression

    match_expression: ($) =>
      seq(
        "match",
        field("subject", $._expression),
        "{",
        repeat($.match_arm),
        optional($.match_else),
        "}",
      ),

    match_arm: ($) =>
      seq(
        "case",
        field("pattern", $._type),
        optional(field("binding", $.identifier)),
        optional(seq("if", field("guard", $._expression))),
        ":",
        repeat($._expression),
      ),

    match_else: ($) =>
      seq(
        "else",
        optional(field("binding", $.identifier)),
        ":",
        repeat($._expression),
      ),

    // >>> Try expression

    try_expression: ($) =>
      prec.right(seq(
        "try",
        field("expr", $._expression),
        optional(seq("is", field("type", $._type))),
        optional(seq(
          "else",
          optional(field("binding", $.identifier)),
          field("body", $.block),
        )),
      )),

    return_expression: ($) =>
      prec.right(PREC.ASSIGN, seq("return", $._expression)),

    break_expression: (_) => "break",

    continue_expression: (_) => "continue",

    // >>> Function literal / Closure

    function_literal: ($) =>
      seq(
        "fun",
        optional(field("captures", $.capture_list)),
        "(", field("parameters", optional($.function_literal_parameter_list)), ")",
        optional(seq(optional("noescape"), field("return_type", $._type))),
        field("body", $.block),
      ),

    function_literal_parameter_list: ($) =>
      seq($.function_literal_parameter, repeat(seq(",", $.function_literal_parameter))),

    function_literal_parameter: ($) =>
      seq(
        field("name", choice($.identifier, $.allocator_identifier)),
        optional(seq(optional(choice("sync", "noescape")), field("type", $._type))),
        optional(seq("=", field("default", $._expression))),
      ),

    capture_list: ($) => seq(
      token.immediate("["),
      optional(seq($.capture, repeat(seq(",", $.capture)))),
      "]",
    ),

    capture: ($) => choice(
      field("name", choice($.identifier, $.allocator_identifier)),  // by value
      seq("&", field("name", $.identifier)),                        // by ref
      seq("&", "mut", field("name", $.identifier)),                 // by mut ref
    ),

    // >>> Array literal

    array_literal: ($) => seq(
      "[",
      $._expression, repeat(seq(",", $._expression)), optional(","),
      "]",
    ),

    empty_slice: (_) => seq("[", "]"),

    // >>> Grouped expression

    grouped_expression: ($) => seq("(", $._expression, ")"),
  },
});
