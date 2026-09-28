# Roadmap

Work not yet done. An entry is deleted once it is.

## Infer a schema's type from its enum or const

JSON Schema 2020-12 makes `type` optional, and real specifications rely on
that: Notion's constrain 135 schemas with `enum` or `const` alone, e.g.
`{"enum": ["error"]}` or `{"const": 401}`. The openapi library accepts
these as they are, since they are valid. But every consumer after it,
codegen above all, then has to work out the type itself.

Enriching a specification is this repository's job, and it runs before
codegen. So it should fill in the type once, for everyone downstream:

- A schema without `type` whose `enum` and `const` values are all of one
  JSON kind gets that type: `string`, `integer` (whole numbers only),
  `number`, `boolean`, `array` or `object`.
- A `null` among other values of one kind makes that type nullable
  (`Nullable`, written as `[X, "null"]`), rather than blocking inference.
- Values of several other kinds leave the schema as it is.
- A schema without `enum` or `const` is left alone. With no constraints
  at all, it deliberately accepts any value.

Every one of Notion's 135 cases holds values of a single kind (87 string,
48 integer), so all of them would get a type.
