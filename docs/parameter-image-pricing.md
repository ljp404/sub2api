# Parameter Image Pricing

Custom opt-in mode: `parameter`. Configure it in Channel Management > Model
Pricing > OpenAI > Billing Mode. The first version supports API-key accounts
on `/v1/images/generations` and `/v1/images/edits`, including JSON, multipart
and streaming responses. It does not configure video billing.

Group Management > Model Pricing also supports this mode for OpenAI and
Composite groups. Composite media entries are stored with the `openai` protocol;
other protocol entries retain their platform. Explicit group model rules take
priority over channel rules and use the same validation and calculator.

## Calculation

`base_price * product(parameter factors) * actual image count * effective rate`

Base prices and factors are decimal strings. The calculator rounds the final
standard and customer amounts to 8 decimal places, matching the existing
ledger. Token counts, reasoning effort, peak-hour token rates and old image
tier prices are not additional charges in this mode. The effective rate
retains the existing user/group rate and independent image-rate policy.

An explicit channel parameter rule takes precedence over legacy group image
prices. Other billing modes keep their original resolution and calculation.
The selected rule and effective rate are frozen before upstream generation.

## Parameters and Defaults

Rules use version 1 and unit `image`, with up to 16 enumerated factors.
Any permitted short top-level scalar request field can be a factor.
`quality`, `variant`, `resolution` and `aspect_ratio` are examples, not a
vendor-specific branch in the calculator.

Keys and enum values use lowercase. Values are matched case-insensitively.
Prompts, models, credentials and image payloads cannot be pricing factors.
Missing factors require an explicitly configured default, except `variant`,
which falls back to `flare` when no rule default is configured. Missing,
empty, whitespace-only or JSON null variants use that default. Explicit
nonempty variants are validated, never silently replaced. Unknown values,
duplicate or nonscalar factor fields fail before upstream generation.
Defaults must match the upstream's behavior. A model-name suffix is not
automatically interpreted as a variant.

When the selected parameter rule contains `resolution` or `aspect_ratio`,
the public size input is required: `size=auto` or `size=WIDTHxHEIGHT`.
Each side must be an integer from 1 to 65536; case and surrounding whitespace
are normalized. Bare `1k`/`2k`/`4k` are not valid public sizes in this mode.

- `auto` resolves to `resolution=2k` and `aspect_ratio=auto`. The configured
  factors must contain those options; no fallback to another tier/default.
- For pixel sizes, match the longest side to the nearest configured integer
  `Nk` tier (`N * 1024` pixels). Equal distances choose the larger tier.
- Match width/height to configured numeric ratios, excluding `auto` and
  candidates with a different orientation (landscape, portrait or square).
  Minimize `abs(log(request_ratio / candidate_ratio))` using exact fraction
  comparisons; equal distances choose the smaller ratio, then lexical order
  for equivalent ratio names. No candidate means an error.
- A factor not present in the selected rule is neither inferred nor forwarded.
  The current factor schema defines independent options, not a matrix of
  allowed resolution/ratio combinations; administrators must configure a
  compatible combination of options for the upstream model.

For example, `1536x1024` maps to `2k` + `3:2`, `1920x1080` to `2k` + `16:9`,
and `3840x2160` to `4k` + `16:9` when those options are configured. These are
nearest supported specifications, not a promise of exact output pixels.
Client `resolution`, `aspect_ratio` and `image_size` cannot override mapping.
Forwarded JSON/multipart drops the original `size` and aliases, injecting the
same frozen dimensions used for billing. Uploads and unrelated fields remain
unchanged. Usage retains the original `image_input_size` and the matched factors.
Rules without size factors and all legacy billing modes keep their behavior.

For saved rules that contain a variant factor, the resolved default is written to the
forwarded JSON or multipart request so the adapter receives the same variant
used for billing. Legacy modes do not inject this field. An explicit rule
default takes precedence over the built-in fallback.

The supplier preset contains the provided $0.052706 base price and quality,
resolution and aspect-ratio factors. Loading it changes the unsaved form;
save the channel only after confirming its required/default parameters.

Configure `gpt-image-2.5-flare` and `gpt-image-2.5-sunburst` as separate model
pricing cards. Each card owns its base price and factors; neither requires
`variant`. The upstream selects the version from the forwarded model name.
Without a variant factor, Sub2API neither defaults nor injects variant.
When splitting an existing rule, multiply the base price by the corresponding
old variant factor before removing that factor, preserving the price.
Existing rules and historical billing snapshots are not automatically rewritten.

## Settlement and Audit

Parameter-priced Images requests require a standard balance group. After
validating parameters, the gateway reserves the matched request cost for `n`
images in PostgreSQL: available `balance` decreases and `frozen_balance`
increases in one transaction. Insufficient balance or reservation failure stops
the request before any upstream generation. This mode does not additionally
reserve a maximum tariff in Redis. Legacy billing modes remain unchanged.

Each attempt has a durable `image_parameter_billing_holds` row and frozen price.
Success consumes the hold, refunds any unused portion for partial output,
and updates quota counters and pricing audit with the normal billing dedup
in one transaction. The held amount is not deducted a second time. An explicit
upstream rejection (4xx except timeout) or failure before sending releases the
hold. A transport timeout, malformed response or ambiguous server failure
retains the hold as `uncertain`; it must be reconciled against the supplier,
not automatically refunded or sent again. Interrupted processes also retain
their durable holds; there is no expiry that silently refunds billable work.

Settlement commands and usage metadata are saved before settlement. A leased
background retry recovers `settlement_required` rows after temporary database
failures or restarts. A crash before the outcome is persisted leaves an active
hold for supplier reconciliation. The synchronous Images protocol has no
standard task-query endpoint, so uncertain outcomes cannot be polled like video.
If an upstream returns more successful images than requested, final settlement
records their actual cost, including a possible outstanding balance, matching
the video policy rather than discarding an incurred charge.

Only completed image outputs count, using the existing response counter.
Requested `n` is not used as a fallback for this mode. Partial successful
batches settle their actual outputs. Preview calls the same calculator
without generation or balance changes.

`parameter_billing_snapshots` stores the rule, content hash, normalized matched
values, quantity, effective rate and amounts in the same transaction as
balance/quota settlement and the existing deduplication claim.
Retries cannot create another charge or audit row for the same request.
Usage detail APIs expose this snapshot, and the cost tooltip displays it.
Historical usage is never recalculated.

Account statistics retain their separate existing policy. They do not imply
actual supplier invoices. Parameter rules can be configured in OpenAI/Composite
group model cards, but not independent account-statistics cards.

## Upgrades

Keep the additive migration and the standalone calculator/adapter/editor
when merging upstream tags. If upstream introduces another migration numbered
249, reconcile its name with the migration runner before deployment; never
rename or edit an already-applied migration on a running installation.
Validate rule persistence, decimal calculation, image output counts, frozen
rules, atomic audit/dedup and old modes when upgrading.
