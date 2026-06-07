# Conventions

These rules apply to all code in this project. They are also portable to other
projects — copy this file and add `"instructions": ["conventions.md"]` to that
project's `opencode.json`.

## Concept-driven modules

- Group code by **concept** (domain area), not by file size alone.
- Concepts live at `src/lib/concepts/<concept>/` (SvelteKit alias
  `$lib/concepts/<concept>/`).
- Avoid naming a concept after the app/product itself. App-name concepts tend
  to become dumping grounds. Prefer names that describe the actual boundary
  being modeled, such as `video`, `payment`, `rendering`, `http`, etc.
- Nested concepts are allowed when a parent domain naturally contains focused
  sub-concepts, e.g. `src/lib/concepts/video/segments/` and
  `src/lib/concepts/video/transcription/`. A nested concept still follows the
  same per-concept layout and exposes its own `index.ts` public surface.
- Concepts are 90% server-side. When a type or util is needed in client code,
  place it inside the concept directory and import via `$lib/concepts/...`
  (never `$lib/server/concepts/...` — that prevents client imports).
- A concept can have cross-concept dependencies (e.g. `trimia` using `http`
  utils). Other concepts then `import * as http from '$lib/concepts/http'`.
- When an action or workflow needs to coordinate multiple concepts, do not
  bolt methods onto an existing service. Promote a new wrapper concept that
  imports the other concepts and exposes its own `service.ts` with the
  composite logic. Composition beats cross-cutting services.

  Example: an `order-fulfillment` concept depends on `order`, `payment`, and
  `inventory`; its `service.ts` orchestrates the trio. Each underlying
  concept's service stays focused on its own rules.

## Per-concept layout

A concept is a directory. Each file has a single, well-defined role. Always
include `index.ts` as the public surface; include the other files only when
the concept actually needs them.

| File           | Role                                                                                  |
| -------------- | ------------------------------------------------------------------------------------- |
| `types.ts`     | Plain TypeScript types. Shape-only, no runtime.                                       |
| `schema.ts`    | Runtime-validated shapes (zod). Types are derived from the schema.                    |
| `repo.ts`      | Data access — HTTP calls, DB queries, external services.                              |
| `service.ts`   | Business rules and logic. Orchestrates `repo.ts` and other concepts.                  |
| `utils.ts`     | Concept-local helpers. Rare — prefer a shared concept (see below).                    |
| `builder.ts`   | Builder pattern for complex object construction.                                      |
| `factory.ts`   | Factory pattern for choosing an implementation or composing a family of objects.      |
| `presenter.ts` | Transforms domain shapes into view/DTO shapes (UI formatting, date formatting, etc.). |
| `index.ts`     | Barrel re-exporting the public surface.                                               |

Pattern files (`builder.ts`, `factory.ts`, `presenter.ts`) are not common.
Reach for them only when the pattern is clearly justified by the code's
shape, not by default.

### `types.ts` vs `schema.ts`

- **`types.ts`** — use when the shape is a contract with a trusted system
  (typed API client, generated DB types, internal DTOs). No runtime cost.
- **`schema.ts`** — use when the shape crosses an untrusted boundary (form
  input, env vars, webhook payloads, anything from the network or user).
  Derive the type from the schema with `z.infer`:

  ```ts
  // schema.ts
  import { z } from 'zod';

  export const Job = z.object({
  	jobId: z.string(),
  	mediaId: z.string(),
  	status: z.string()
  	/* ... */
  });

  export type Job = z.infer<typeof Job>;
  ```

  A concept picks **one** per shape: either `types.ts` or `schema.ts`, not
  both. A concept can hold both for different shapes (e.g. `Job` is a plain
  type, `JobFormInput` is a zod schema), but the same shape only lives in
  one place.

### `service.ts`

- Home for **business rules**: validation, computed fields, side-effect
  orchestration, multi-step workflows.
- A service may call `repo.ts` from its own concept and from sibling
  concepts, but it should not contain raw HTTP or DB code — that belongs in
  `repo.ts`.
- Pure functions are preferred; classes are fine when state is required.

### Cross-concept actions — wrapper concept

When a workflow spans multiple concepts, **do not** add methods to a
service for the other concepts. Create a new concept that depends on them
and put the orchestration in its own `service.ts`:

```ts
// concepts/order-fulfillment/service.ts
import * as order from '$lib/concepts/order';
import * as payment from '$lib/concepts/payment';
import * as inventory from '$lib/concepts/inventory';

export async function fulfill(orderId: string, fetcher: typeof fetch) {
	const draft = await order.service.getDraft(orderId, fetcher);
	const stock = await inventory.service.reserve(draft.items, fetcher);
	const charge = await payment.service.charge(draft.total, fetcher);
	return order.service.markFulfilled(draft.id, charge.id, fetcher);
}
```

This keeps every concept's service focused on its own rules. The wrapper
concept owns the workflow.

## Symbol naming — no redundant prefix

The concept directory already provides the namespace, so symbols inside the
concept drop the concept's name as a prefix:

- `$lib/concepts/trimia/` exports `Job`, `Segment`, `fetchJob`, `sourceUrl` —
  **not** `TrimiaJob`, `fetchTrimiaJob`, `trimiaSourceUrl`.
- When discussing a symbol in prose (PRs, comments, reviews), refer to it
  with the dir prefix restored: `TrimiaJob`, `fetchTrimiaJob`, etc.

## Consumer imports — Go-style namespace

External consumers (routes, components, other concepts that take a
dependency on the whole concept) import the **whole barrel as a namespace**:

```ts
import * as trimia from '$lib/concepts/trimia';

const job: trimia.Job = await trimia.fetchJob(id, fetch);
const url: string = trimia.sourceUrl(job.mediaId);
```

This avoids name collisions (e.g. having another `Job` type elsewhere) and
reads like a Go package import (`trimia.Job`).

Client-side `.svelte` files that only need types use a type-only namespace
import:

```ts
import type * as trimia from '$lib/concepts/trimia';

let localSegments = $state<trimia.Segment[]>([]);
```

Do not create auxiliary local type aliases when the imported type name is
already available through the namespace. Prefer `core.Media` directly over
`type TrimiaMedia = core.Media`, and prefer `Media` directly over
`type TrimiaMedia = Media` when using named type imports.

Internal usage within the same concept (e.g. `service.ts` importing from
`./types`) uses regular named imports — that is fine.

If a concept barrel exports server-only modules such as `repo.ts`, browser code
must not import that barrel as a value because it can pull private server code
into the client graph. Client code may import a specific client-safe module
instead, e.g. `$lib/concepts/core/service`, or use a type-only namespace import
when it only needs types.

## Where utilities live

- **`utils.ts` inside a business concept is a smell.** Utilities should not
  be tied to a single business domain. Move them to a shared concept named
  after the utility's actual scope: `utils`, `date`, `id`, `format`,
  `http`, etc.
- Shared concepts are themselves first-class concepts: they have an
  `index.ts` barrel and use the same namespace-import convention
  (`import * as http from '$lib/concepts/http'`).
- A business concept may still have a `utils.ts` for one-off helpers that
  are genuinely tied to that concept, but prefer extracting to a shared
  concept if the helper is reusable.

## When to split a file or create a new concept

- **Split a file in a concept** when it grows beyond ~80 lines or mixes
  multiple concerns. Common splits: `types.ts` vs `repo.ts` vs
  `service.ts`; or per-resource (`job.ts`, `segment.ts`, `media.ts`)
  when those resources are non-trivial.
- **Create a new concept** when:
  - A new domain area emerges (e.g. `payment`, `auth`, `billing`).
  - A workflow coordinates multiple existing concepts (wrapper concept,
    see above).
  - A set of utilities is reused across concepts — extract them to a
    shared concept named for their scope.

Always keep `index.ts` as the public surface of a concept.

## Component-level concepts

- `src/lib/components/ui/` is reserved for shadcn-svelte managed primitives.
  Do not add app-specific components there, and avoid editing generated UI
  primitives unless the change is intentionally updating the shadcn layer.
- App-specific components live under `src/lib/components/<concept>/`, named
  after the UI/product area they represent, e.g. `$lib/components/studio/`.
- Component concepts are presentational only: Svelte components, UI-only
  types, formatters, CSS class helpers, DOM/UX calculations, and small view
  transformations.
- Business rules, domain mutations, data access, and cross-concept workflows
  stay in `src/lib/concepts/...`, usually in `service.ts` or `repo.ts`.
- Component concepts may have nested component folders. Prefer colocating
  component-specific `types.ts` and `helpers.ts` beside the component that uses
  them over creating a central helper/presenter file for the whole component
  concept.
- `presenter.ts` is allowed but not the default. Use it only when a concept
  clearly needs a reusable transformation from domain data into a view model.
  For local UI logic, prefer a nearby `helpers.ts`.
