# Feature plans

One folder per feature, created by the `/feature <topic>` skill (`.claude/skills/feature/`) before implementation starts:

```
docs/features/<slug>/
├── README.md            # investigation, reference legend, Step 0 (prereqs + tooling), steps, done-when
├── ref-NN-<what>.png    # original-client reference screenshots (kRO/iRO first; roBrowser / korangar fallback)
├── grf-<name>.png       # UI bitmaps extracted from the GRF, when the feature touches UI chrome
└── current-<what>.png   # our client's state at planning time, if it renders anything related
```

The README is the source of truth; the GitHub issue (label `feature`) is generated from it with image links pinned to the commit SHA. Review happens on the issue; `/feature revise <issue#>` applies Boris's comments back to the README and the issue. Each feature lands on `feature/<slug>` in a single PR whose commits follow the plan's steps.

Related: `qa/use-cases/` (acceptance scenarios written at planning time), `docs/adr/` (only when a step contains an architectural decision), `docs/sessions/` (written at implementation time).
