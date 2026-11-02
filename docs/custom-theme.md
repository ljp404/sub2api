# YUNDATA Theme

The custom theme follows the rendered homepage at
`https://oapi.yundata.ai/home`, inspected on 2026-11-02.
It preserves existing authentication, navigation and page layouts.

## Palette

- Primary button and brand title: `#1769f5` (`primary-600`).
- Hover: `#1456cc` (`primary-700`).
- Focus and accents: `#347df7` (`primary-500`).
- Dark mode accents: `#6097fb` (`primary-400`).
- Selected navigation surface: `#eff5ff` (`primary-50`).
- Light page canvas: `#f5f8fc` (`canvas`).

The primary scale is defined in `frontend/tailwind.config.js`. Global buttons,
focus states and navigation use it through `frontend/src/style.css`.
`AuthLayout.vue` and `AppLayout.vue` use a clean canvas without decorative
colored blobs. Auth brand names and logos continue to come from public settings.
The onboarding buttons, key-usage focus border, settings tab indicator and
unknown-platform fallback accent also follow this palette.

Success, warning, error, payment/provider branding and categorical chart colors
retain their meaning. The approved model plaza remains isolated in its shadow
root with its existing styling and API pricing behavior.

## Validation and Upgrades

Use pnpm 9 to build the frontend and lint changed files. Check login buttons,
hover and focus, active navigation, the theme toggle, desktop/mobile sizing and
the model plaza after merging an upstream release. Production preview or the
final image should be used for browser verification.

Keep these changes when following the upgrade process in `AGENTS.md`.
Publish a distinct image tag; preserve the previous model plaza image for
rollback. Theme changes do not require changes to database settings.
