# The golden files

`TestEnrich_Golden` enriches `testdata/openapi.json` with
`testdata/interactions.json`, three times over, and must get
`testdata/golden.json` each time. Nothing regenerates them: a feature is a
case in them, edited in by hand.

- A new behaviour is a recording in `interactions.json`, next to the ones of
  its kind, and, where it needs the specification to know something first, a
  part of `openapi.json` whose `description` says what it is there for.
- `golden.json` changes with it, by hand, so the diff is read before it is
  accepted. Write the expected result; the failure names the first line that
  differs.
- The recordings are made up, not recorded: there is nothing to mask, and no
  account behind them.
