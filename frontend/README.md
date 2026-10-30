# YourMove Opportunities frontend

React 19.3.1 + React DOM 19.3.1 slice for the `/jobs` Opportunities route.
Vite builds to `dist/` (served at `/app/`); Go embeds the production build.
No Vite dev server is required in production.

## Toolchain (pinned)

- Node.js `26.3.0` (see `.nvmrc`, `package.json` engines)
- pnpm `10.14.0` (see `package.json` packageManager)
- React `19.3.1`, React DOM `19.3.1` (exact pins)
- TypeScript strict; Vite `6.2.1`, `@vitejs/plugin-react` `4.3.4`

## Reproducible build

```sh
cd frontend
pnpm install --frozen-lockfile
pnpm run build
```

Verify installed React versions match:

```sh
pnpm list react react-dom
```
