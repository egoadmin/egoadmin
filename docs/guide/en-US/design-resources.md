# Design Assets

EgoAdmin frontend design assets live in `web/design`. After GitHub Pages deployment, they are mounted as a static preview site under `/design/`. The documentation site stays at the Pages root, while the design preview remains a dedicated entry for implementing Vue 3 + Element Plus screens.

::: tip Entry
After deployment, open **Design Assets** from the top navigation or visit `/design/`. The local source directory is `web/design`.
:::

## Asset Structure

| File | Purpose |
|------|---------|
| `web/design/index.html` | Design launcher and page index |
| `web/design/design-system.html` | Design system, visual tokens, component states and interaction rules |
| `web/design/DESIGN-MANIFEST.json` | Machine-readable design manifest for screens, assets, viewports and implementation policy |
| `web/design/DESIGN-HANDOFF.md` | Implementation handoff for developers |
| `web/design/DESIGN.md` | Design documentation |
| `web/design/critique.json` | Design review and critique data |

## Page Entries

| Page | Preview Path | Notes |
|------|--------------|-------|
| Design System | `/design/design-system.html` | Colors, type scale, spacing, radius, shadow and component states |
| Landing | `/design/landing.html` | Product landing / external page |
| Login | `/design/login.html` | Login form, brand area and error state reference |
| User Management | `/design/user.html` | User list, filters, table and actions |
| Role Management | `/design/role.html` | Role list and permission configuration entry |
| Organization | `/design/organization.html` | Department and organization hierarchy management |
| Online Users | `/design/online_user.html` | Online sessions and force logout operations |
| Operation Logs | `/design/operation_log.html` | Audit log filters and detail view |
| Profile | `/design/personal.html` | Profile and security settings |
| States | `/design/states.html` | Empty, loading, error and success states |

## Development Flow

When implementing or adjusting frontend pages, use the design assets in this order:

1. Open `/design/design-system.html` and extract colors, type scale, spacing, radius, shadows, motion and component states.
2. Open the target screen, such as `/design/user.html`, and confirm density, table columns, filters, button hierarchy and dialog behavior.
3. Match the existing Vue pages and components under `web/src`; do not introduce a visual system that conflicts with the current Element Plus baseline.
4. Verify the result on mobile, tablet and desktop widths. Avoid horizontal scrolling, overflowing button text and compressed table toolbars.
5. For user-facing changes, include request / response examples or screenshots in the PR.

## Screen to Frontend Mapping

| Design File | Suggested Implementation Area | Focus |
|-------------|-------------------------------|-------|
| `login.html` | `web/src/views/login` | Login form, error messages, loading and password encryption flow |
| `user.html` | `web/src/views/system/user` | Query form, paged table, create / edit / disable user |
| `role.html` | `web/src/views/system/role` | Role CRUD, menu permissions and button permissions |
| `organization.html` | `web/src/views/system/organization` | Organization tree, department editing and hierarchy states |
| `online_user.html` | `web/src/views/monitor/online-user` | Session list, online status and force logout |
| `operation_log.html` | `web/src/views/monitor/operation-log` | Audit log filters, detail dialog and export entry |
| `personal.html` | `web/src/views/account/personal` | Profile, security settings and password change |
| `states.html` | `web/src/components` | Empty, loading, error and success state components |

## Implementation Rules

::: warning Current-state only
Do not add old-vs-new layout comparisons or historical notes. The documentation and design assets describe the current project state only.
:::

Frontend implementation should follow these rules:

| Area | Requirement |
|------|-------------|
| Component Library | Prefer Vue 3 + Element Plus and avoid rebuilding basic primitives |
| States | Cover default, hover, focus, disabled, loading, empty, error and success |
| Layout | Keep the current admin-console density and avoid marketing-style card-heavy layouts |
| Responsive | Check at least 390px, 820px, 1366px and 1440px widths |
| Accessibility | Keep semantic forms, buttons and links; visible focus states are required |
| Data | Use real API fields and business naming instead of meaningless placeholders |

## Local Preview

The design assets are static HTML files. You can open them directly or serve them with any static file server:

```bash
# From the repository root
python3 -m http.server 4174 --directory web/design
```

Visit:

```text
http://localhost:4174/
http://localhost:4174/design-system.html
http://localhost:4174/user.html
```

## GitHub Pages Paths

The current Pages deployment layout is:

| Content | Published Path |
|---------|----------------|
| VitePress docs | `/` |
| Chinese docs | `/guide/...` |
| English docs | `/en-US/guide/...` |
| Design assets | `/design/...` |

The final Pages artifact is composed by `.github/workflows/pages.yml`, so documentation and design previews do not overwrite each other.
