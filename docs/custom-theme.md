# YUNDATA Theme

The custom theme follows the rendered homepage at
`https://oapi.yundata.ai/home`, inspected on 2026-11-02.
It preserves existing authentication and navigation behavior.

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

## Authentication Presentation

`frontend/src/components/layout/AuthLayout.vue` and its scoped
`auth-layout.css` provide the shared login, registration and recovery shell.
Desktop uses an unframed model-network presentation alongside the account form.
Mobile hides the diagram and keeps the form in a single scrollable column.
Desktop windows below 950px height use tighter page spacing. Below 820px,
form controls and field spacing also become more compact so the standard
registration form and footer fit in common laptop viewports. Longer optional
forms keep natural scrolling instead of clipping controls.
The footer contains only centered copyright text; omit the decorative English
slogan.
Account forms start directly with their page heading; omit the lock icon and
account-center caption above the heading.
Model brand icons reuse the existing `ModelIcon` component. The network is an
illustration of the gateway concept, not live availability or usage data.

Site name and logo still use public settings. Presentation copy lives in the
English and Chinese `auth.presentation` locale entries. The old technical site
subtitle is not displayed in this shell. Keep other branding settings intact.
Password visibility on login has translated accessible labels.

Preserve OAuth, captcha, agreements, registration switches and authentication
handlers when updating the presentation. Avoid adding fixed form heights; some
deployments have long registration forms or multiple authentication providers.
Dark mode and reduced-motion preferences are supported.

## Validation and Upgrades

Use pnpm 9 to build the frontend and lint changed files. Check login buttons,
hover and focus, active navigation, the theme toggle, desktop/mobile sizing and
the model plaza after merging an upstream release. Production preview or the
final image should be used for browser verification.

Keep these changes when following the upgrade process in `AGENTS.md`.
Publish a distinct image tag; preserve the previous model plaza image for
rollback. Theme changes do not require changes to database settings.
