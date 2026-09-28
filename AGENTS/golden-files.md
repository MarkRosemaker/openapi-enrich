# Golden files in testdata

`TestEnrich_TestData` runs every folder in `testdata/`. It loads
`openapi.json` if the folder has one, otherwise it starts from `NewDocument`.
It then enriches the document with `interactions.json` three times over and
compares each result with `golden.json`. A folder without `golden.json`
fails, and a changed result fails at the first differing byte.

There is no flag or make target that writes a golden file. To create or
update one, temporarily make the test write `gotDoc` to the golden path
instead of comparing it. Create an empty `golden.json` first if the folder
has none, because the test reads the file before it enriches anything. Run
the test once, revert the change, and run it again: the second run must
pass, which shows that enriching is stable across repeated passes. Review
the diff of `golden.json` as part of the change, because it is the
behaviour under test.
