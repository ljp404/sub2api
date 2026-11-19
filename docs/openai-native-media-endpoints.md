# Native Media Endpoints on OpenAI API Key Accounts

An aggregator account can serve multiple upstream protocols without creating
separate platform accounts. In account creation, editing, or bulk editing,
select the required endpoint capabilities:

| Capability | Upstream Protocol |
| --- | --- |
| Seedance (Ark) | Existing Ark video task endpoints |
| Gemini Images | Gemini native `generateContent` and `streamGenerateContent`, for image output |
| MiniMax Video | MiniMax native video creation and task query |

The new capabilities are opt-in. Accounts without them retain their existing
eligibility. They require an OpenAI API Key account with a configured Base URL.
The same Base URL and API key are used for each selected protocol.

With Base URL `https://aggregator.example/v1`, native requests use:

| Method | Upstream Path | Authentication |
| --- | --- | --- |
| POST | `/v1beta/models/{mapped_model}:generateContent` | `x-goog-api-key` |
| POST | `/v1beta/models/{mapped_model}:streamGenerateContent` | `x-goog-api-key` |
| POST | `/v2/video_generation` | Bearer API key |
| GET | `/v2/query/video_generation/{task_id}` | Bearer API key |

Gemini preserves its native body and response format. MiniMax preserves its
existing native request and response handling. Account model mappings still
apply. MiniMax images continue to use the existing OpenAI Images flow.

Use an OpenAI or composite API key group containing the account. Gemini image
requests also require the group's image-generation permission. Composite
groups honor explicit routes and account model ownership; when only the model
name identifies a native platform and no native account is available, they may
select an OpenAI account with the required capability.

Video queries remain bound to the API key, user, group, and account that created
the task. Existing billing, reservation, settlement, and reconciliation flows
are reused; enabling a protocol does not choose a billing mode.
