# Public model names and channel routing

The customer model catalogue merges channel aliases such as
`claude-opus-4-8/hjwl` and `claude-opus-4-8/sdl` into `claude-opus-4-8`.
Account mappings, administrator configuration and the model plaza keep their
channel aliases. Existing requests using a channel alias remain supported.

For a JSON or multipart generation request using the base model name, the
gateway selects an enabled account mapping in the caller's group with the
required endpoint capability. Only candidates admitted by the group model
allowlist participate. An explicit composite model route takes precedence.
The selected internal alias enters the existing protocol handler and billing
pipeline; reservations, settlement and usage records retain its pricing.
Gemini generation requests resolve the model from the URL.

Pricing comparison follows the configured billing model source and honors
group price overrides before channel prices. Rules are comparable only when
their billing modes, configured price dimensions, tiers, geometry, currency,
exchange rate and multipliers have the same structure. Storage IDs and model
names are ignored. A candidate is cheapest only if every configured charge is
no higher than every other candidate. Ties are randomized.

Different rule structures, missing configured prices and crossed price
advantages (for example cheaper input but dearer output) select a random
candidate. The gateway does not assume missing prices are zero or predict the
eventual generated token count.

The suffix convention is one slash followed by an alphanumeric channel code
of at most 32 characters. Known provider namespaces and multi-segment model
resource names are preserved.
