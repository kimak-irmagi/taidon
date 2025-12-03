# Contributing to Taidon

Thank you for your interest in contributing!  
This project is developed by an open community, including students, volunteers, and researchers.  
We keep the process simple to make onboarding easy.

---

## How to Contribute

### 1. Find an Issue

Check the issue tracker. Good starting points:

- `good first issue` — simple tasks, ideal for newcomers  
- `help wanted` — tasks that need attention  

If you want to propose your own idea, feel free to open a new issue.

---

### 2. Create a Branch

Please do not commit directly to `main`.

Use the following naming conventions:

- Features: `feature/<short-name>`
- Bug fixes: `fix/<short-name>`
- Documentation: `docs/<short-name>`

Example:

```bash
git checkout -b feature/query-snapshots
```

---

### 3. Development Model

Taidon is a multi-language monorepo.  
Each subproject (`frontend/*`, `backend/*`, `docs`, `research`) has its own setup, tooling, and README.

Before pushing changes:

- follow the tooling described in that module’s README  
- run its tests and linters  
- make sure you do not break other modules  

---

### 4. Pull Requests

When opening a PR:

- Describe what the change does  
- Include screenshots/logs if relevant  
- Link to related issues  
- Keep commits reasonably clean  

Review rules:

- Each PR must be approved by at least one reviewer  
- CI must pass  

Do not merge your own PR unless explicitly allowed.

---

## Community Guidelines

Please respect the Code of Conduct ([`CODE_OF_CONDUCT.md`](./CODE_OF_CONDUCT.md)).  
We aim to create a friendly, inclusive environment for contributors of all backgrounds.

If you need help, feel free to ask in Discussions or the project chat.
