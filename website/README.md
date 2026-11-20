# Yard docs site

Docusaurus site for Yard — same Zyvor docs-site family as Netra.

```bash
cd website
npm install
npm start          # http://localhost:3000/yard/
npm run build
npm run serve
```

Published at **https://zyvorai.github.io/yard/** via GitHub Actions
(`.github/workflows/pages.yml`). Do not use `npm run deploy`
(gh-pages branch); Pages source is GitHub Actions.

Docs cover quickstart, architecture, connectors, deploy, security, API,
and [console features](docs/guides/console.md) (bulk IO, map clustering,
severity runbooks). Keep them aligned with `docs/ROADMAP.md` when shipping.
