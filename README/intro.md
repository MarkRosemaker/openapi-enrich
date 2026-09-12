Plenty of APIs have no specification, or one that stopped matching reality some time
ago. What they do have is traffic. This module treats that traffic as the source of
truth: record some real calls, and get a document describing what the API actually
does.

Enrichment is incremental by design. Every additional interaction refines the
result rather than replacing it — a second observation of the same endpoint
contributes any fields the first one didn't include, and widens a type where the two
disagree. That merging is delegated to
[`openapi-merge`](https://github.com/MarkRosemaker/openapi-merge), which exists
precisely for the problem of reconciling schemas inferred from independent samples.
