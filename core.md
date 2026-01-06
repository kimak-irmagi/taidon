# Taidon - Core Thesis

## I. Problem: Database Statefulness

1. Relational databases not only answer queries but also change their state, which complicates learning, testing, experimentation, and CI/CD.
2. Irreversible changes create problems:
   - **Learning**: a query mistake can "kill" a training DB; parallel learning requires many separate copies.
   - **Testing**: hard to roll back to arbitrary states; hard to reproduce change chains.
   - **Experiments and benchmarks**: data prep is expensive; identical conditions are hard to guarantee.
   - **Product delivery**: migrations, CI/CD, and integration tests suffer without a reproducible environment.
3. Existing DBMSs poorly support mass parallel sandboxes: snapshots are heavy, isolation is expensive.

## II. Core idea: Smart versioning of query chains

4. Taidon treats a **SQL query chain as a deterministic description of database state**.
5. MVP assumption: non-determinism can be ignored; if needed, the user can mark true non-deterministic points.
6. The user works with a **hierarchically organized SQL project**, interpreted as a tree of states.

## III. Architectural model: Deterministic SQL Replay Engine

7. The chain is split into _head_ (last query) and _tail_ (all previous queries).
8. Tail is hashed; the system checks whether a ready snapshot exists in cache.
9. If a snapshot is found, it is restored instantly and head is executed.
10. If not, tail is restored recursively, a new snapshot is created, and saved under the full-chain hash.
11. The system naturally supports **branching histories**: branches with a shared prefix share earlier states.

## IV. Snapshots: Leveraging existing container mechanisms

12. Taidon does not build its own storage engine; it uses existing technologies: copy-on-write FS, snapshotting, delta layers.
13. The main requirement: **minimal environment startup time**.
14. A hybrid storage strategy is possible: delta layers plus periodic materialization.
15. The delta approach will be refined through research.

## V. Intelligent cache policy

16. Cache considers:
    - storage cost,
    - rebuild cost (time, resources),
    - access frequency.
17. The system prioritizes heavy init scripts, allowing their results to be reused many times.
18. Future optimization may include delta compaction.

## VI. Platform UX

19. MVP hides internal mechanics; the user simply **runs SQL scripts**.
20. Versions and snapshots are hidden by default and available via API / Pro mode.
21. The user does not work with containers directly.
22. The project structure is managed via `init/`, `seed/`, `test/`, `bench/`, etc.

## VII. SQL compatibility and MVP limits

23. The MVP engine is stock PostgreSQL.
24. SQL is full, industrial-grade, no emulation.
25. One user = one sandbox.
26. External extensions are limited to built-ins (future options possible).
27. Schema is managed only through SQL chains; the initial state is an empty or prebuilt DB.

## VIII. Target audience

28. Primary: developers of SQL-heavy backends.
29. Secondary: education platforms, analysts, CI/CD.

## IX. Core product values

30. **Instant setup** - work in seconds without deployments.
31. **Reproducibility** - exact state restoration at any point.
32. **Scalable learning** - hundreds of parallel sandboxes without data duplication.
33. **Fearless usage** - experiment without risk.
34. **Interactivity** - precise error messages and SQL analysis.

## X. Key differences from alternatives

35. Not cloning DBs but **computing state from a deterministic query chain**.
36. Not rebuilding data but **smart caching of init procedures**.
37. Not isolated sandboxes per user but **optimal storage of versioned states**.
38. Not a SQL playground but a **replicable platform for projects with branching histories**.
39. Not SQL emulation but **real PostgreSQL managed by an orchestration engine**.
