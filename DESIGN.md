# Design System: codexpert Authenticated Dashboard

## 1. Visual Theme & Atmosphere

The authenticated console is a quiet, high-density operations dashboard. It uses a warm off-white canvas, faint technical grid lines, and white translucent surfaces to make financial and usage data easy to scan. The visual posture is precise and product-led: mostly near-black text, restrained gray chrome, one coral accent, and shallow glass depth rather than decorative illustration.

- Overall feeling: calm, technical, premium utility.
- Visual density: compact navigation and metric cards with generous enough gaps for scanning.
- Brand posture: confident and minimal, with black primary actions and coral state emphasis.
- Signature motifs: 4.6rem grid, subtle -45deg line texture, rounded glass cards, coral active marker.

### Key Characteristics

- Warm neutral background `#fbfaf8` instead of pure white.
- 224px fixed sidebar with an active row ring and small coral indicator.
- 20px cards with translucent white fill, inset highlight, and soft deep shadow.
- Header stays sticky and translucent while the workspace scrolls underneath.

## 2. Color Palette & Roles

| Role | Semantic Name | Value | Usage |
| --- | --- | --- | --- |
| Primary action | Ink black | `#050505` | Primary buttons, selected navigation, logo field |
| Accent | Coral | `#ff705a` | Active marker, cost emphasis, focus border, hover card ring |
| Surface | Elevated white | `rgba(255,255,255,0.9)` | Cards, dropdowns, dialogs |
| Background | Warm cloud | `#fbfaf8` | Authenticated app canvas and sidebar |
| Text | Ink | `#11100e` | Headings and key values |
| Body text | Quiet black | `rgba(5,5,5,0.62)` | Navigation and supporting copy |
| Border | Hairline ink | `rgba(5,5,5,0.08)` | Card, header, table, and divider borders |

### Primary

- Ink black is the strongest interactive color and replaces a colored primary CTA.
- Coral is reserved for meaningful state, cost, focus, and active navigation signals.

### Interactive

- Hover surfaces use `rgba(5,5,5,0.043)`.
- Focus uses a coral border with a translucent coral ring.
- Active sidebar rows use `rgba(5,5,5,0.06)` and an inset 1px ring.

### Neutral Scale

- Page: `#fbfaf8`.
- Surface: `rgba(255,255,255,0.9)`.
- Primary ink: `#11100e` / `#050505`.
- Quiet text: `rgba(5,5,5,0.52)` to `rgba(5,5,5,0.62)`.
- Hairlines: `rgba(5,5,5,0.055)` to `rgba(5,5,5,0.12)`.

### Surface & Overlay

- Main page surface: `#fbfaf8`.
- Elevated card and control surface: `rgba(255,255,255,0.9)`.
- Sticky header overlay: `rgba(251,250,248,0.88)` with 18px blur.
- Background texture: two 1px grid gradients, a -45deg repeating line, and a low-opacity coral radial wash.

### Theme Modes

#### Light Mode

- Background: `#fbfaf8`.
- Surface: translucent white.
- Text: near-black ink.
- Accent: `#ff705a`.
- Notes: This is the observed dashboard mode and the reference implementation.

#### Dark Mode

- Background: `#0f172a`.
- Surface: `rgba(30,41,59,0.9)`.
- Text: slate-100 and white.
- Accent: keep coral for state and focus.
- Notes: Preserve the same radius, spacing, and shadow language while replacing white glass with dark slate glass.

### Shadows & Depth

- Card shadow: `0 1rem 2.4rem rgba(17,24,39,0.06)` plus a 1px white inset highlight.
- Hover shadow: `0 1.15rem 2.7rem rgba(17,24,39,0.08)`.
- Depth comes from blur, inset highlights, and soft shadows; avoid heavy borders.

## 3. Typography Rules

### Font Family

- Primary: `system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "PingFang SC", "Microsoft YaHei", sans-serif`.
- Monospace: system monospace for code or technical values only.
- OpenType Features: normal system rendering; use tight negative tracking only for headings.

### Hierarchy

| Role | Font | Size | Weight | Line Height | Letter Spacing | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| Page title | System sans | 18px | 600 | 28px | -0.035em | Header title |
| Section heading | System sans | 16-18px | 600 | 24-28px | -0.025em | Card and module headings |
| Metric value | System sans | 28-32px | 700 | 32-36px | normal | Numbers should dominate labels |
| Body | System sans | 14-16px | 400 | 20-24px | normal | Dashboard copy |
| Label / eyebrow | System sans | 12-14px | 500-700 | 16-20px | 0.02em | Table headers and stat labels |
| Caption / meta | System sans | 12px | 400 | 16px | normal | Hints, dates, totals |

### Principles

- Keep labels quiet and values high-contrast.
- Use sentence-case Chinese UI copy and short operational labels.
- Let numeric columns align and breathe; do not use oversized marketing typography inside the console.

## 4. Component Stylings

### Buttons and Links

- Primary CTA: black fill, white text, hairline black border, compact 40px-ish height.
- Secondary CTA: white translucent fill, ink text, `rgba(5,5,5,0.12)` border.
- Text links: gray by default, near-black on hover.
- Hover and active feel: crisp color change, no bounce; coral appears for focus or important state.

### Cards and Containers

- Surface style: translucent white with 14px backdrop blur.
- Radius: 20px for cards; 12-18px for smaller panels and controls.
- Border: `rgba(5,5,5,0.08)` hairline.
- Shadow or elevation: soft 16px/38px shadow and 1px inset white highlight.
- Internal spacing: 16px compact cards, 20-24px section headers and larger modules.

### Inputs and Interactive Controls

- Inputs and selects use white translucent fill with a 0.12 ink border.
- Focus switches the border and ring to coral.
- Selectors and date controls are compact, rounded, and grouped into a single filter bar.

### Navigation

- Structure: fixed vertical sidebar plus sticky utility header.
- Background treatment: sidebar repeats the same grid/diagonal texture as the canvas.
- Link style: 12px radius, 10px vertical padding, gray ink; active row gets inset ring and coral dot.
- Sticky or scroll behavior: header remains pinned; sidebar remains fixed and independently scrollable.

### Image Treatment

- Keep logo imagery contained in a small rounded square; avoid large decorative imagery in the dashboard.
- Preserve real product marks where available and do not blur or darken them.

### Distinctive Components

- Metric grid: short cards with colored icon tiles and large values.
- Reward progress rail: horizontal milestone track with green completion states.
- Recharge lottery panel: framed floating card with compact reward chips and a single wide action.
- Model usage table: dense, readable table inside a rounded surface.

## 5. Layout Principles

### Spacing System

- Base unit: 4px, with 8px and 16px as dominant rhythm.
- Repeated spacing values: 12px control gaps, 16px card padding, 24px section gaps, 32px desktop page padding.

### Grid & Container

- Grid logic: responsive CSS grids; five compact metrics at wide desktop, four token/performance metrics below, then two-column charts and a 3-column recent/quick-actions area.
- Max content width: fluid within the main area; desktop content uses 32px page padding.
- Section spacing: 24px vertical rhythm between dashboard modules.

### Whitespace Philosophy

- Whitespace is functional: enough separation to scan metrics, never oversized hero gaps.
- Align labels and values to a consistent card baseline.
- Let long model names truncate rather than force layout expansion.

### Border Radius Scale

- Micro: 8px for controls and icon tiles.
- Standard: 12px for nav rows, tables, and compact panels.
- Large: 20px for cards and dialogs.
- Pill: 999px for progress markers and badges only.

## 6. Depth & Elevation

| Level | Treatment | Use |
| --- | --- | --- |
| Flat | Warm canvas and 1px texture | App background |
| Ring | Hairline ink border or inset ring | Active rows, controls, tables |
| Card | 14px blur + soft 16/38 shadow | Metrics, charts, recent usage |
| Focus | Coral border and translucent coral ring | Inputs and keyboard focus |

### Depth Principles

- Surface hierarchy: canvas -> sticky glass header/sidebar -> white glass cards -> floating dropdowns.
- Shadow language: broad and pale, never black or dramatic.
- Blur, glass, or overlay behavior: use backdrop blur on sticky and elevated surfaces.
- Use depth for grouping and hierarchy; keep the grid texture visually subordinate.

## 7. Do's and Don'ts

### Do

- Use near-black primary actions and coral only for meaningful emphasis.
- Keep dashboard cards rounded, translucent, and lightly elevated.
- Preserve dense but readable tables and compact metric labels.

### Don't

- Do not reintroduce the previous beige/green hand-painted palette.
- Do not use large marketing hero treatments inside authenticated screens.
- Do not stack cards inside cards without a clear module boundary.

## 8. Responsive Behavior

### Breakpoints

| Name | Width | Key Changes |
| --- | --- | --- |
| Mobile | <640px | Sidebar becomes an overlay drawer; grids collapse to one column; header utility labels hide. |
| Tablet | 640-1023px | Sidebar remains drawer-capable; metric grids use two columns; controls wrap. |
| Desktop | >=1024px | Fixed 224px sidebar, sticky header, multi-column metric and chart grids. |

### Touch Targets

- Keep icon buttons at least 36-40px square.
- Keep nav rows around 40px high with 12px horizontal gaps.

### Collapsing Strategy

- Desktop behavior: fixed sidebar at 224px, collapsible to 72px icon rail.
- Tablet behavior: preserve the same shell but allow the drawer to close after navigation.
- Mobile behavior: hide sidebar by default and use the header menu toggle.
- Breakpoint-driven component changes: card grids collapse before typography changes; labels hide before controls disappear.
- Touch target and spacing adjustments: retain 8-16px gaps and avoid shrinking primary actions below 40px.

## 9. Agent Prompt Guide

### Quick Color Reference

- Primary CTA: `#050505`.
- Background: `#fbfaf8`.
- Heading text: `#11100e`.
- Body text: `rgba(5,5,5,0.62)`.
- Border or ring: `rgba(5,5,5,0.08)`.
- Accent: `#ff705a`.

### Quick Summary

Build a restrained authenticated AI API console on a warm off-white grid canvas. Use a 224px textured sidebar, sticky translucent header, 20px white glass cards, compact system typography, black primary actions, and coral only for active/focus/cost emphasis. Keep density high enough for repeated operational scanning and let responsive grids collapse cleanly.

### Example Component Prompts

- Hero: Create a compact dashboard header with an 18px semibold title, muted one-line description, and a sticky off-white glass utility bar.
- Card: Create a 20px rounded translucent white metric card with 16px padding, 14px blur, an inset white highlight, and a broad pale shadow.
- Navigation: Create a fixed 224px sidebar with a faint 1px grid and -45deg line texture; active rows use a black translucent fill, inset ring, and 6px coral marker.
- Button or badge: Create a 40px black primary button with white text; use coral for focus rings and important status badges.

### Ready-to-Use Prompt

Implement the authenticated page in the codexpert console language: `#fbfaf8` grid background, 224px fixed sidebar, sticky translucent header, 20px white glass cards, system sans typography, `#050505` primary actions, and `#ff705a` state accents. Preserve compact metric grids, readable tables, 24px section rhythm, and mobile drawer behavior.

### Iteration Guide

1. Verify canvas, sidebar, header, and card surfaces before tuning individual widgets.
2. Check the 1024px desktop layout first, then collapse grids at tablet and mobile widths.
3. Keep accent usage sparse; if every element is colorful, return emphasis to ink black and neutral gray.

## Optional Appendix: Interaction Patterns

- Scroll behavior: header stays sticky; sidebar stays fixed; content scrolls beneath the shell.
- Hover behavior: links gain a faint ink wash; cards gain a coral-tinted border and slightly broader shadow.
- Click behavior: sidebar collapse toggles to a 72px icon rail; utility controls open compact dropdowns or panels.
- Animation tone: short, restrained transitions around 200-300ms.

## Optional Appendix: Content & Messaging Patterns

- Headline pattern: direct page names such as “仪表盘”, followed by one short orientation sentence.
- CTA language: operational verbs such as “刷新”, “查看全部”, “立即抽奖”, and “创建 API 密钥”.
- Trust signal pattern: show actual balance, request, token, and cost values rather than promotional claims.
- Voice and tone: concise, factual, bilingual-friendly product UI copy.

## Optional Appendix: Observed Pages

- `https://codexpert.top/dashboard`: authenticated user dashboard observed after user login; contributed the shell, metric cards, reward rail, lottery panel, charts, usage table, and quick-action layout.
