# Custom Model Plaza

The customization branch `custom/model-plaza` starts from upstream `v0.2.19`.
The existing `/model-plaza` route displays the approved custom design. Logged-in
users opening `/model-plaza?embedded=1` retain the application navigation.

## Files

- `frontend/src/views/ModelPlazaView.vue`: existing route and layout selection.
- `frontend/src/components/modelPlaza/CustomModelPlazaContent.vue`: Vue lifecycle,
  authenticated API loading, request cancellation, and style isolation.
- `frontend/src/components/modelPlaza/custom/`: template, styles, and behavior
  ported from the standalone design.

The original upstream plaza components remain available for comparison during
future upgrades. No additional public endpoint or separate web service is needed.
The custom page uses the existing `getModelPlaza` API client, including its login
token and response handling. Prices and groups come from the server.

## Billing Display

- User-specific rates override default group rates.
- Token prices can be shown per million or per thousand tokens.
- Interval prices use the existing `resolveIntervalPrices` helper, including
  the cache-write 1h fallback.
- Image and video prices respect their independent rate settings.
- Per-request prices are not compared with per-token official catalog prices.
- Cards disclose the selected display group and standard-time price basis.
- Details expose context tiers, reasoning effort rates, peak windows and model
  time-pricing schedules.
- Missing reference prices remain unavailable.
- Markdown price descriptions are sanitized before display.
- Model content labels are display metadata; they do not claim API capabilities.

## Validation

Run `pnpm install --frozen-lockfile` with pnpm 9, then `pnpm run build` from
`frontend/`. Lint the changed files with the project's ESLint configuration.

Browser regression checks should cover anonymous and authenticated routes,
search, filters, group and user-specific rates, both context tiers, tiny prices,
copying, reference visibility, table mode, failure/retry, and mobile overflow.
Also check image/video independent rates and sanitize an HTML-bearing Markdown
description using an intercepted browser response without changing server data.

## Upgrades and Images

Create an `upgrade/<tag>` branch from the customization branch, fetch the
selected upstream tag, and merge it. Resolve conflicts individually and retain
both upstream fixes and these customizations. Deepen shallow history before a
merge when necessary. Validate before merging back.

Use a unique image tag in
`registry.cn-guangzhou.aliyuncs.com/quickpay-sit/sub2api`. Keep the base release,
source revision, target architecture and registry digest in release records.
Do not add registry credentials to source, build arguments or image layers.
