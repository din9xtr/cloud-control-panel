# Web interface design

Guidance for agents changing the look, layout or motion of `web/`. The interface is a dark,
quiet product UI with a console identity: the `cloud_` wordmark, shell prompts, `~/paths` and
monospace figures sit next to clean sans-serif text, hairline-bordered cards and one
saturated blue for actions. Every screen should feel like part of one product.

The design system is original to this project. Do not copy CSS, layouts, assets, icons or
fonts from other products or websites, and do not reproduce another site's characteristic
look. Take ideas only at the level of general UI practice.

## Principles

- **Console first.** The wordmark, prompts (`admin@cloud:~$ whoami`), paths, page numbers
  (`02 / Links`), table headers, sizes and dates are monospace. Headlines, body text and
  buttons are sans-serif. This contrast is the signature of the interface.
- **Dark canvas, bordered cards.** Depth comes from slightly lighter surfaces with a 1 px
  hairline border. Shadows are reserved for floating layers (menus, toasts, dialogs).
- **One action color.** Blue marks primary actions, links, focus, selection and progress.
  Green means done, red destructive, amber warning. Category colors only tint file icons
  and the storage chart.
- **Content first.** Pages open with a headline and one line of context. Chrome stays
  compact: a sticky top bar, a page toolbar, and actions as small buttons.
- **Motion explains structure.** Panels drop or slide from their origin, lists enter in a
  short stagger from the left. Nothing bounces or loops except spinners and the caret.

## Tokens

All tokens live in `web/src/styles/index.css` under `@theme`. Use the Tailwind utilities they
generate (`bg-surface`, `text-fg-subtle`, `rounded-card`, `ease-glide`) instead of raw
values. Add a token before adding a second hard-coded color or radius.

| Role | Token | Value |
| --- | --- | --- |
| Page background | `canvas` | `#07090c` |
| Inputs, menus on phones, insets | `sunken` | `#0d1015` |
| Cards, dialogs, list containers | `surface` | `#12161c` |
| Floating menus, hover rows | `raised` | `#1a1f27` |
| Chips, meter tracks, menu hover | `elevated` | `#232a33` |
| Hairlines | `line`, `line-strong` | `#1e242c`, `#2b333e` |
| Text | `fg`, `fg-secondary`, `fg-muted`, `fg-subtle`, `fg-faint` | `#eef1f5` → `#4e5763` |
| Primary action | `accent` (+ `-hover`, `-active`), `on-accent` | `#3d7eff`, white text |
| Links, folders, progress | `link` | `#6aa5ff` |
| Status | `success`, `warning`, `danger` | `#3fcf8e`, `#f5a524`, `#ff5c5c` |
| Categories | `orange`, `purple`, `pink`, `teal` | images, video, audio, archives |
| Translucent bars | `bar` with the `glass` utility | `rgb(7 9 12 / 0.78)` + 16 px blur |
| Radii | `control`, `card`, `sheet` | 10 px, 14 px, 20 px |

The favicon, app icons and the `c_` mark are part of the brand and are not regenerated for
visual changes. Keep `theme-color` and the manifest colors equal to `canvas`.

## Typography

Inter (bundled, SIL Open Font License) is the sans face, with the platform UI font as the
fallback. Fira Code (bundled, OFL) is the monospace face. Never bundle or reference fonts
whose license does not allow redistribution.

| Utility | Size / line height | Use |
| --- | --- | --- |
| `type-display` | 46 / 1.06, 32 on phones | rare, standalone moments |
| `type-page` | 38 / 1.1, 28 on phones | the page `<h1>` |
| `type-section` | 24 / 1.25, 21 on phones | section titles, stat values |
| `type-title` | 18 / 1.33 | dialog and card titles |
| `type-body` | 16 / 1.55 | rows, paragraphs, inputs |
| `type-small` | 14 / 1.45 | toolbar, buttons, table cells, secondary lines |
| `type-micro` | 12 / 1.4 | hints, badges, footer |
| `label-caps` | 11 mono, uppercase, tracked | eyebrows and table headers |

Headlines use weight 650 with negative tracking. A muted second half of a headline
(`88 MB <muted>of 460 GB used</muted>`) is the preferred way to show a value with context.

## Layout

- **Top bar** (`layout/AppShell.tsx`): sticky, 56 px (52 px on phones), translucent with a
  bottom hairline. Wordmark, section tabs in 14 px with a blue underline for the active one,
  account button with the user's initial. On phones the tabs move into the full-screen menu.
- **Page toolbar** (`layout/PageBar.tsx`): 48 px bar under the top bar, sticky from `md`.
  Breadcrumbs or a numbered title on the left (`03 Storage`), ghost buttons with icons and one
  primary button on the right. On phones the ghost buttons collapse into a `⋯` sheet.
- **Content width**: `Page` gives 1000 px (default), 720 px (`narrow`, settings) or 1120 px
  (`wide`, storage and files), with 24 px gutters and 16 px on phones.
- **Lists** sit in a bordered `surface` card; rows are at least 56 px tall with inset
  hairlines. On phones lists go edge to edge.
- **Cards** use the `card` utility (surface, hairline, 14 px radius) and 28 px padding
  (20 px on phones).
- **Footer**: a single hairline row with the wordmark, section links and the tagline.

## Components

- `Button`: 10 px radius. `primary` for the one main action, `secondary` for neutral actions
  (raised with a border), `ghost` in toolbars, `danger` (outlined red) for destructive
  actions. Sizes `sm` 32 px, `md` 36 px, `lg` 44 px.
- `Field`: 52 px input with a floating label that turns blue on focus; always pass `label`,
  never a placeholder. Focus shows a blue border and a soft blue ring.
- `Dialog`: 20 px radius, header and footer separated by hairlines, footer on a sunken band.
  Becomes a bottom sheet with a grabber on phones.
- `Menu`: raised dropdown with a border on desktop (portalled, fixed position, flips up near
  the bottom edge), bottom sheet on phones.
- `Badge`: 6 px radius chip for states such as "This device" or remaining link time.
- `EmptyState`, `Loading`, `ErrorState`, `PageHeader` in `ui/Feedback.tsx` cover the standard
  page states.
- Checkboxes use the `check` utility: rounded square, blue when checked, animated tick.
- Icons come from Lucide (ISC license). Do not add icons from other icon sets without
  checking their license.

## Motion

Easing tokens: `ease-swift` = `cubic-bezier(0.33, 0, 0.2, 1)` for color and small state
changes, `ease-glide` = `cubic-bezier(0.16, 1, 0.3, 1)` for things that move or grow.

| Pattern | Timing | Where |
| --- | --- | --- |
| Color and hover | 180–200 ms `ease-swift` | links, buttons, rows |
| Active tab underline | scale from left, 280 ms `ease-glide` | top bar |
| Dropdown | `animate-drop`: 6 px drop and fade, 220 ms | account menu, row menus |
| Menu items | `animate-enter`: 10 px slide from the left, 280 ms, 60 ms + 30 ms × index | all menus |
| Phone menu | clip-path reveal from the top, 280 ms; items 140 ms + 30 ms × index | global menu |
| Menu icon | two bars of different length turn into a cross | phone top bar |
| Page reveal | `animate-reveal`: 16 px rise and fade, 560 ms | headers, first blocks |
| Dialog | `animate-rise` on desktop, `animate-sheet` on phones | `ui/Dialog.tsx` |
| Bars and meters | `animate-grow` from the left, 800 ms | storage |

Use `stagger(index, base)` from `lib/motion.ts` for delays. When closing, drop the delay with
`!delay-0`. Global CSS shortens all motion under `prefers-reduced-motion`; do not add motion
that is required to understand the UI.

## Writing

Sentence case everywhere. Labels are verbs ("Upload", "Create link", "Move to Trash").
Console fragments stay lowercase (`store. share. stay in control.`). Helper text is one short
sentence. Never show internal paths, tokens or raw server errors beyond what `messageOf`
returns.

## Checklist for UI changes

1. Use tokens and type utilities; no new hex values, radii or font sizes without a reason.
2. Check desktop (≥ 1024 px) and phone (375 px) widths, including menus and sheets.
3. Keep CSP rules: no inline `<style>` or third-party assets. Dynamic sizes go through
   React `style` props, as the meters do.
4. Keep accessible names stable; tests query by role and name.
5. Use invented, neutral names in screenshots and fixtures, and refresh README screenshots
   when a screen changes noticeably.
6. Run `pnpm test && pnpm lint && pnpm typecheck && pnpm build` in `web/`.
