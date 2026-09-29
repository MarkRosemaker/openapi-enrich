# Golden files in testdata

`TestEnrich_TestData` runs every folder in `testdata/`. It loads
`openapi.json` if the folder has one, otherwise it starts from `NewDocument`.
It then enriches the document with `interactions.json` three times over and
compares each result with `golden.json`. A folder without `golden.json`
fails, and a changed result fails at the first differing byte.

`make ready` rewrites every `golden.json`, through `go generate` and
`tools/generate.go`: it enriches each folder's document once with its
interactions and writes the result. It also masks and rewrites
`interactions.json`, and sends any recorded request that has no response
yet, which needs the network. A new folder needs only `interactions.json`
and, optionally, `openapi.json`. Review the diff of `golden.json` as part of
the change, because it is the behaviour under test.
