# Video Token Pricing

Channel model pricing has a separate opt-in `video_token` mode for the Seedance
Ark task endpoint. Existing token, image parameter, and per-second video rules
keep their existing behavior. Only standard balance groups are supported by the
new mode; subscription and simple mode are rejected before forwarding.

## Formula

```
billed_input_seconds = has_video ? max(input_seconds, output_seconds * 2/3) : 0
tokens = floor((billed_input_seconds + output_seconds) * width * height * 24 / 1024)
total_cost = tokens * price_per_million / 1000000
actual_cost = total_cost * effective_video_multiplier
```

The calculation uses decimal arithmetic, an exact rational 2/3 lower bound, and
one final floor for tokens. Money is rounded to the existing eight-place ledger.
For 1920x1080, 5 seconds and no input video: 243000 tokens, $1.8711 at $7.7/M.

## Configuration

Admin -> Channels -> OpenAI model pricing -> Video token pricing.
Alternatively, configure the model in Group Management > Model Pricing for an
OpenAI or Composite group. Composite video rules use the `openai` protocol and
the existing Ark reservation/settlement chain. Group model rules precede channel
rules; no separate Composite billing formula is introduced.
Load the standard or Fast tariff into separate model pricing cards. Rates remain
editable. Fast cards contain only `fast:*` tiers; the gateway selects them only
when that card has no ordinary resolution tier. It does not infer discounts from
model names or trust a client `service_tier` to lower the rate.

Expand the resolution/ratio size table and verify pixel sizes against the
supplier. A resolution such as 480p alone does not uniquely determine width.
Preset standard pixel sizes are editable, and are not an assertion of the
supplier's actual geometry. Add other supported ratios explicitly.

The create request must supply `duration`, `resolution`, and `ratio`, as in the
customer API documentation. Fixed output parameters can be calculated without
an upstream `usage` field. Trusted completion width/height/duration override
the fixed request estimate when present. The `frames` and `draft_task` options
are rejected in this new mode until their metering contracts are implemented.
Resolution matching is case-insensitive; Sub2API forwards the canonical
lowercase value (for example, `480p` or `720p`) to the upstream adapter.

For `duration=-1`, reserve 15 output seconds. For `ratio=adaptive`, reserve
the largest configured pixel area for the chosen tier. Actual adaptive output
duration and dimensions (or a configured actual ratio) must be returned by the
trusted task status response.

For input videos, reserve the configured `max_input_seconds` total. Completion
must include trusted aggregate `input_video_duration`, either at the top level
or under `usage`. A URL, user-supplied duration, or a missing field is not proof
of an input video's length. Output metadata is read from top-level
`width`, `height`, `duration`, `fps`, or their `video.*` counterparts.

## Task Lifecycle

Before forwarding create, reserve balance and persist a frozen rule, multiplier,
owner, account and output parameters in PostgreSQL. Redis contains only the
existing routing/ownership cache. Persist the upstream task ID before returning
it. Completion consumes the hold, refunds the difference or records debt.
Failed/cancelled/expired tasks release the hold. Retries are protected by both
row locks and the normal durable usage billing dedup key.

A background reconciler queries pending tasks even when the customer does not
poll; it resumes from PostgreSQL after restart. Upstream errors and missing
measurements do not silently become free completed generations. Missing
measurements remain `measurement_required` with a recorded reason and a held
balance. Correct the adapter's completion metadata to permit settlement.

Ambiguous create transport errors retain the hold as `uncertain`; an upstream
may already have accepted the task. A crash between create and task binding can
leave `reserved` records with no task ID. These require operator reconciliation
against upstream before release; they are never automatically refunded on a TTL.
Inspect `video_token_billing_jobs.state`, `last_error`, `task_id`, and logs prefixed
`service.video_token`. Do not release a hold merely because a GET returned 404.

The final formula, dimensions, measured durations, rate, tokens, rule hash and
hold ID are written to `video_token_billing_snapshots` in the same transaction
as money settlement. Usage detail displays this immutable audit snapshot.

## Upstream Actual Token Settlement

Successful task queries may return `usage.completion_tokens` or
`usage.video_tokens`. These fields mean the final billable video tokens for
the entire task, including any billable input video. They must not be reserved
tokens or output-only tokens that require adding another input amount.
If both are returned they must agree. Values must be positive JSON integers
up to 100000000000. Invalid, null, zero, conflicting or duplicate values retain
the hold in `measurement_required`; they never silently fall back.
`usage.total_tokens` is ignored because its video billing meaning is ambiguous.

Actual tokens override the formula count, while the frozen channel tier price
and effective multiplier still determine the charge. Supplier dollar amounts
and supplier multipliers never replace our selling price. No input-video
tokens are added to the actual count. The snapshot records `token_source` as
`upstream` or `formula`; old snapshots without this field mean formula.

Example response from the adapter to Sub2API, without a code/data envelope:

```json
{
  "id": "task-id",
  "model": "seedance-2.0",
  "status": "succeeded",
  "content": {"video_url": "https://example.test/video.mp4"},
  "width": 864,
  "height": 496,
  "duration": 5,
  "fps": 24,
  "input_video_duration": 0,
  "usage": {"completion_tokens": 50220}
}
```

Keep actual geometry and durations for audit and formula fallback. Existing
adaptive measurement requirements and input-video duration validation remain
in force even with actual tokens. Fixed requests without metadata retain
their prior configured-size fallback. Missing actual tokens should be omitted,
not represented as zero or null. Client-supplied usage is never used for final
settlement. Already settled tasks and historical usage are not recalculated.
