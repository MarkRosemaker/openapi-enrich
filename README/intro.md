Plenty of APIs have no specification, or one that stopped matching reality some time
ago. What they do have is traffic. This module treats that traffic as the source of
truth: record some real calls, and get a document describing what the API actually
does.

That document is also the first step towards a client library. It feeds
[`openapi-flatten`](https://github.com/MarkRosemaker/openapi-flatten),
[`openapi-compress`](https://github.com/MarkRosemaker/openapi-compress) and
[`openapi-codegen`](https://github.com/MarkRosemaker/openapi-codegen), so an API
that was never properly documented gets both a specification and a Go library to
call it with. Where the library fails to decode a response, recording that call
and enriching again closes the gap.

Enrichment is incremental by design. Every additional interaction refines the
result rather than replacing it — a second observation of the same endpoint
contributes any fields the first one didn't include, and widens a type where the two
disagree. That merging is delegated to
[`openapi-merge`](https://github.com/MarkRosemaker/openapi-merge), which exists
precisely for the problem of reconciling schemas inferred from independent samples.
