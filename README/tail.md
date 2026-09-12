## Design

- **No I/O** — the caller loads and saves the spec.
- **No flatten/tidy/sort** — use separate libraries for those.
- **Own interaction types** — no dependency on a specific HTTP recording format.

The result of enrichment is deliberately raw: inline schemas, unsorted, unpolished.
Turning that into something pleasant to read or generate from is the job of the
modules below, applied in whatever order suits you.

## The openapi family

| Module | Purpose |
|---|---|
| [openapi](https://github.com/MarkRosemaker/openapi) | Parse, validate, and write OpenAPI 3.x specifications |
| [openapi-compare](https://github.com/MarkRosemaker/openapi-compare) | Compare specification objects — exact equality and shape equivalence |
| [openapi-edit](https://github.com/MarkRosemaker/openapi-edit) | Safe structural edits, such as renaming a schema and rewriting every `$ref` to it |
| [openapi-flatten](https://github.com/MarkRosemaker/openapi-flatten) | Promote inline definitions into named `components` entries |
| [openapi-compress](https://github.com/MarkRosemaker/openapi-compress) | Deduplicate and merge equivalent component schemas |
| [openapi-merge](https://github.com/MarkRosemaker/openapi-merge) | Merge schemas that were inferred independently from different samples |
| **openapi-enrich** (this module) | Infer specification content from observed HTTP traffic |
| [openapi-codegen](https://github.com/MarkRosemaker/openapi-codegen) | Generate Go types, clients, and servers from a specification |

A common sequence is to enrich from traffic, flatten the inline schemas into named
components, compress the duplicates that flattening produces, and then generate a
client.
