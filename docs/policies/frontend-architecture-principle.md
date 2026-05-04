# Frontend architecture principle

## Tech stack

- Language: [TypeScript](https://typescriptlang.org)
- Framework: [React](https://react.dev)
- Router: [React router](https://reactrouter.com)
- Build tool: [Vite](https://vite.dev)
- Test framework: [Vitest](https://vitest.dev)
- UI library: [Chakra UI](https://chakra-ui.com)
- Style: [Emotion](https://emotion.sh)
- Virtual scroll: [TanStack Virtual](https://www.npmjs.com/package/@tanstack/react-virtual)
- State management: [Zustand](https://github.com/pmndrz/zustand)
- HTTP: [Axios](https://github.com/axios/axios)
- Data schema: [Zod](https://zod.dev/)

## Core principles

- Keep route/page components responsible for composition and orchestration; keep reusable components focused on rendering and interaction.
- Prefer unidirectional data flow: API clients fetch and validate data, stores hold shared domain state, pages compose state and UI, and components receive explicit props.
- Keep raw transport payloads at the API boundary. UI, stores, hooks, and components should consume frontend domain models rather than backend DTOs.
- Prefer local component state for local UI concerns. Use URL state for shareable navigation state, and Zustand only for state shared across routes, sessions, or distant component trees.
- Keep shared UI components pure. They must not fetch data, mutate global state, or know about route-specific behavior.
- Make loading, empty, error, and permission states explicit for user-facing flows.
- Prefer simple, typed code over clever abstractions. Add abstractions only when they reduce repeated behavior or clarify ownership.
- Follow existing repository patterns unless intentionally refactoring a boundary.

## Directory rules

### Expected Layout

```plaintext
.
└── web
    ├── app
    │   ├── pages
    │   │   └── <page>
    │   │       ├── components
    │   │       │   └── <component>.tsx
    │   │       └── <page>-page.tsx
    │   ├── components
    │   │   └── <component>
    │   ├── stores
    │   ├── contexts
    │   ├── hooks
    │   ├── api
    │   ├── models
    │   ├── utils
    │   ├── routes.ts
    │   └── root.tsx
    ├── public
    ├── tests
    ├── package-lock.json
    └── package.json
```

### Responsibilities

- `app/pages`
  - Page components used in routing (e.g. `/rooms/:roomId` renders `Room` page component).
  - Pages are the route-level composition layer. They may compose API calls, stores, hooks, contexts, and UI components.
  - Page-specific components must live under `app/pages/<page>/components`.
  - If page logic grows complex, extract it into page-local hooks or reusable `app/hooks` only when it is shared.
- `app/components`
  - (1) custom UI components not provided by Chakra UI, and/or
  - (2) higher-level, complex UI components **composed from Chakra UI primitives**.
  - Allowed imports: `app/components`, `app/models` for type-only imports, `app/utils` only.
  - Must not import `app/api`, `app/stores`, route modules, or page-specific modules.
- `app/utils`: Reusable, non-business-logic utilities (e.g. `isNull`).
  - Allowed imports: `app/utils` only.
- `app/api`: API client layer. Use Zod to parse/validate API responses and map them into **domain models** to avoid coupling UI/business logic to raw API response shapes.
  - Allowed imports: `app/api`, `app/models`, `app/utils` only.
- `app/models`: Frontend domain types and pure domain helpers shared across API, stores, hooks, and pages.
  - Allowed imports: `app/models`, `app/utils` only.
  - Must not import API clients, stores, React components, route modules, or framework-specific code.
- `app/stores`: Global state management (Zustand). Stores should primarily hold **domain models** for page components to consume.
  - Allowed imports: `app/stores`, `app/api`, `app/models`, `app/utils` only.
- `app/contexts`: Global contexts.
  - Use for cross-cutting providers and values that naturally belong in React context.
  - Allowed imports: `app/contexts`, `app/hooks`, `app/stores`, `app/models`, `app/utils` only.
  - Must not import page-specific modules or API clients directly unless the context is explicitly an application-level data provider.
- `app/hooks`: Custom React hooks.
  - Use for reusable UI/data orchestration that is not itself a component.
  - Allowed imports: `app/hooks`, `app/api`, `app/stores`, `app/contexts`, `app/models`, `app/utils` only.
  - Must not import components or page-specific modules.

---

## Import boundaries

- `utils` is the lowest layer: it must not depend on anything else.
- `models` may depend only on `utils` and other model modules.
- `components` must stay pure: no direct `api`/`stores`/route imports.
- `api` must not import `stores` or UI layers.
- `stores` may depend on `api` (and `utils`) but not on UI layers.
- `hooks` may orchestrate `api`, `stores`, `contexts`, and `utils`, but must not render UI or import components.
- `contexts` must not depend on page-specific modules.
- `pages` may import lower-level layers, but shared lower-level layers must not import pages.
- Avoid circular dependencies and barrel exports that hide cross-layer imports.

---

## API and data contracts

- All HTTP calls must go through `app/api`; do not call Axios directly from pages, hooks, components, stores, or contexts.
- Validate external data with Zod at the API boundary before it reaches UI or global state.
- Keep DTO schemas, response parsing, and DTO-to-domain mapping in `app/api`.
- Expose typed functions from API modules that return domain models or typed command results, not raw Axios responses.
- Normalize API errors into a consistent frontend error shape before exposing them to stores, hooks, or pages.
- Do not let components depend on backend field naming. Backend DTO field names must be mapped before rendering.

---

## State management rules

- Use local React state for ephemeral UI state such as open/closed controls, selected tabs, and unsaved input.
- Use URL/search params for state that should be shareable, restorable, or browser-navigation aware.
- Use Zustand only when multiple unrelated components or routes need the same state.
- Do not duplicate derived state in stores when it can be computed from source state.
- Keep stores focused on state and actions. Move complex business decisions into domain helpers or API/service orchestration hooks.
- Persist store state only when there is a product requirement and the persistence boundary is explicit.

---

## UI and interaction rules

- Prefer Chakra UI primitives and theme tokens over ad-hoc styling.
- Shared components should be accessible by default: use semantic elements, keyboard support, visible focus states, and meaningful labels.
- User-facing flows must handle loading, empty, error, and permission-denied states intentionally.
- Do not add a new shared component until at least two call sites need it or the component represents a clear design-system primitive.
- Page-specific UI should stay page-local until reuse is real.
- Avoid hard-coded layout values when Chakra tokens or responsive props express the intent clearly.

---

## Coding style

Our code style follows the Airbnb style guides. For anything ambiguous, **ESLint/Prettier output is the source of truth** (do not fight the linter/formatter).

### 1) Source of truth
- JavaScript/TypeScript: Airbnb JavaScript Style Guide
- React/JSX: Airbnb React/JSX Style Guide
- This repo uses an Airbnb-based ESLint configuration (and TypeScript/React rules). If the written rules conflict, follow ESLint.

### 2) Practical rules
- TypeScript:
  - Prefer strong typing; avoid `any` unless absolutely necessary.
  - If types are unclear, add explicit types or generics rather than leaving it implicit.
- React:
  - Use **Function Components + Hooks** (do not use class components).
  - Use **camelCase** for prop names.
  - Omit the prop value when it is explicitly `true` (e.g. `<Foo hidden />`).
- Imports/exports:
  - Avoid circular dependencies and arbitrary cross-layer imports (follow the repo's folder boundaries).
- Dependencies:
  - Do not add a new runtime dependency when the existing stack or small local code is sufficient.
  - Any new dependency must include a rationale, expected usage scope, and trade-off.
- Formatting:
  - Do not hand-tune indentation/line breaks; let Prettier (or the repo formatter) handle formatting.

---

## Testing rules

- Unit tests must be **co-located** with the file they test (same folder).
  - Example: `app/pages/rooms/rooms-page.tsx` → `app/pages/rooms/rooms-page.spec.tsx`
- Test file naming must use `.spec.ts(x)`, so Vitest can discover them by default.
- Test behavior and user-visible outcomes rather than implementation details.
- API modules should test Zod parsing, DTO-to-domain mapping, and normalized error behavior.
- Stores should test state transitions and actions without rendering React components.
- Hooks and page components should cover loading, success, empty, and error paths when those states affect the user.
- Mock at external boundaries such as API modules or network calls. Avoid over-mocking internal helper functions.
- Bug fixes should include a failing test or a documented reason why the behavior cannot be tested directly.

## Verification rules

- Prefer repository-provided package scripts when available.
- Frontend changes should run lint, type checking, unit tests, and build checks when available.
- If no repository command exists, use the closest stack-default command and report any missing checks.
- Do not mark frontend work complete until the relevant checks have either passed or been explicitly reported as skipped with reason and risk.
