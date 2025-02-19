# Fuzz testing

Agentbaker exposes a large input surface to its public API.

Namely, `NodeBootstrappingConfiguration` is an enormous struct.

It has heavy duplication from organic evolution and little defense against malicious input.

This makes it a prime candidate for fuzz testing.

## Fuzz targets

We currently have one fuzz target:

- `api` fuzzes `baker.GetNodeBootstrapping` to test custom data generation.

It generates random data as input and attempts to decode it as JSON into `NodeBootstrappingConfiguration`.

If decode fails, we assume it is invalid and deprioritize the fuzz input.

If encode succeeds, we assume it is a valid input to Agentbaker.

We then run the fuzz target function (`GetNodeBootstrapping`) on the input and check for panics.

If it exits successfully, we assume the fuzz input is valid, and return 0 or 1 to tell the fuzzer so it may add it to
the test corpus.

## Continuous integration

We currently have 3 fuzzing pipelines.

- 'batch' mode runs continuously to a set time limit. It finds as many crashes as possible and adds them to the corpus.
- an official build job which runs no real tests, but is used by the fuzzer to track introduced regressions by comparing
  old fuzzer builds.
- a pruning and coverage generation job, which both updates the corpus and generates coverage reports to github pages.

The corpus is stored in a personal github repository (it can be easily moved), and will be generated from scratch if
empty.

The current corpus is https://github.com/alexeldeib/agentbaker-corpus.

Coverage reports are on the gh-pages branch, published
to https://alexeldeib.github.io/agentbaker-corpus/coverage/latest/report/index.html#file0

The pipline definitions are defined in the following locations:

- [batch](../.github/workflows/cflite_batch.yaml)
- [build](../.github/workflows/cflite_build.yaml)
- [prune/coverage](../.github/workflows/cflite_prune.yaml)
