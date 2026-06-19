# Testing architecture

The edit engine's bugs lived at the **intersection of independent axes** —
operation × YAML form × container kind × value class × reference topology — not
in any single feature. So the suite is organised around **axes and invariants**,
not hand-picked scenarios. Discover with fuzz; lock with the deterministic
layers; every layer asserts the **same** invariants.

## The axes (what varies)

Defined once in `editharness_test.go`:

- **operation** — move, reparent, set-field (label/style/shape/id), add, add-edge, delete, duplicate
- **YAML form** — flow vs block, tight vs spaced, inline `parts:[…]` vs block, same-column vs indented
- **container** — root, plain/offset group, autosize, layout row/col/grid, boundary, nested
- **value class** — normal, reserved word (`null`/`true`), special chars (`<>&"{},`), control chars, NaN/Inf/huge, empty, dotted, `~N`
- **reference topology** — connector / place / annotation refs, with `.anchor` and `~N` suffixes

## The layers

| Layer | File | Role |
|---|---|---|
| Unit | `yamledit/*_test.go` | text-surgery helpers vs adversarial inputs |
| Property / acceptance | `acceptance_position_test.go`, `bugfix_regression_test.go`, `injection_xss_test.go` | rich semantic invariants on controlled inputs |
| Matrix | `editmatrix_test.go` | the **op × container** cross-product, explicit |
| Cumulative session | `editsequence_test.go` | seeded random op-chains; floor invariants after every step |
| Golden | `samples/**/expected.svg` | rendering fidelity, byte-stable |
| Fuzz | `editfuzz_test.go` (`FuzzEditSession`) | coverage-guided discovery, reuses the harness |

## The invariants (the oracle)

The **floor invariants** (`checkFloorInvariants`) hold for any op on any doc and
never false-fail, so they run over random sequences and fuzz:

- **F1** no panic
- **F2** a successful op's output always re-parses (no silent corruption)
- **F3** a refused op (err) leaves the source unchanged
- **F4** no op introduces a duplicate part id (only `duplicate` adds an id)

Richer semantic invariants — position preservation, idempotency, inverse,
locality, reference-integrity, no-injection, resolver≡renderer — live in the
property/matrix layers where the input is controlled.

## Fuzz → ratchet

`FuzzEditSession` mutates `(seed, steps)` over the generator. **Every crasher is
minimized into a deterministic test in `bugfix_regression_test.go`** — the
deterministic suite only grows, fuzz only discovers.

## Release gate

Run before every version (must be all-green):

```sh
scripts/release-regression.sh          # ~60s fuzz
FUZZTIME=5m scripts/release-regression.sh   # deeper pre-release
```

It runs build, vet, the full suite (unit + golden + matrix + cumulative
session), the WASM build, and a bounded coverage-guided fuzz of the edit engine.
