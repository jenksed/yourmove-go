<!-- graft:start -->
## Graft — optional repository navigation

Graft provides a generated source graph for faster code navigation.
Use it when installed and available.

Useful commands:

- `graft map` — repository orientation
- `graft ask "question" --source` — relevant source spans
- `graft callers symbol` — caller relationships
- `graft build` — refresh the graph after major source changes

Graft is not required. When unavailable, use
`docs/CODEBASE_GUIDE.md`, Go language tooling, `rg`,
and direct source inspection.

Do not treat generated graph data as more authoritative
than the current source code.
<!-- graft:end -->

## Application UI design decisions

Before changing a user-facing template or its styles, read `docs/design/DESIGN_SYSTEM.md` and `docs/design/RULES.md`. Start with R1 (the current decision) and R6 (existing artifacts before their generators); apply R7–R10 where control parity, prose width, duplicated copy, or overcrowded object lists are relevant. Prefer scoped composition changes over a new component library. Preserve the existing Go POST handlers, Wails redirect behavior, consent boundaries, and explicit submission confirmation.
