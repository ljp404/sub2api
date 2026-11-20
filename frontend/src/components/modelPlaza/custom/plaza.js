import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { formatScaled, resolveIntervalPrices } from '@/utils/pricing'
import { referencePricingSegments } from './referencePricing'
import geminiIcon from './gemini.svg?raw'
import qwenIcon from './qwen.svg?raw'

export function mountPlaza(root, fetchPlaza, authenticated) {
  const lifecycle = new AbortController()
  let disposed = false
  const svgAsset = (path, color) =>
    `data:image/svg+xml,${encodeURIComponent(
      `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="${color}"><path d="${path}"/></svg>`,
    )}`
  const ASSETS = {
    gemini: `data:image/svg+xml,${encodeURIComponent(geminiIcon)}`,
    google: `data:image/svg+xml,${encodeURIComponent(geminiIcon)}`,
    qwen: `data:image/svg+xml,${encodeURIComponent(qwenIcon)}`,
    doubao: `data:image/svg+xml,${encodeURIComponent(
      '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">' +
        '<path fill="#1E37FC" d="M5.31 15.756c.172-3.75 1.883-5.999 2.549-6.739-3.26 2.058-5.425 5.658-6.358 8.308v1.12C1.501 21.513 4.226 24 7.59 24a6.59 6.59 0 002.2-.375c.353-.12.7-.248 1.039-.378.913-.899 1.65-1.91 2.243-2.992-4.877 2.431-7.974.072-7.763-4.5l.002.001z"/>' +
        '<path fill="#37E1BE" d="M22.57 10.283c-1.212-.901-4.109-2.404-7.397-2.8.295 3.792.093 8.766-2.1 12.773a12.782 12.782 0 01-2.244 2.992c3.764-1.448 6.746-3.457 8.596-5.219 2.82-2.683 3.353-5.178 3.361-6.66a2.737 2.737 0 00-.216-1.084v-.002z"/>' +
        '<path fill="#A569FF" d="M14.303 1.867C12.955.7 11.248 0 9.39 0 7.532 0 5.883.677 4.545 1.807 2.791 3.29 1.627 5.557 1.5 8.125v9.201c.932-2.65 3.097-6.25 6.357-8.307.5-.318 1.025-.595 1.569-.829 1.883-.801 3.878-.932 5.746-.706-.222-2.83-.718-5.002-.87-5.617h.001z"/>' +
        '<path fill="#1E37FC" d="M17.305 4.961a199.47 199.47 0 01-1.08-1.094c-.202-.213-.398-.419-.586-.622l-1.333-1.378c.151.615.648 2.786.869 5.617 3.288.395 6.185 1.898 7.396 2.8-1.306-1.275-3.475-3.487-5.266-5.323z"/>' +
        '</svg>',
    )}`,
    anthropic: `data:image/svg+xml,${encodeURIComponent(
      '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="#D97757"><path d="m3.127 10.604 3.135-1.76.053-.153-.053-.085H6.11l-.525-.032-1.791-.048-1.554-.065-1.505-.08-.38-.081L0 7.832l.036-.234.32-.214.455.04 1.009.069 1.513.105 1.097.064 1.626.17h.259l.036-.105-.089-.065-.068-.064-1.566-1.062-1.695-1.121-.887-.646-.48-.327-.243-.306-.104-.67.435-.48.585.04.15.04.593.456 1.267.981 1.654 1.218.242.202.097-.068.012-.049-.109-.181-.9-1.626-.96-1.655-.428-.686-.113-.411a2 2 0 0 1-.068-.484l.496-.674L4.446 0l.662.089.279.242.411.94.666 1.48 1.033 2.014.302.597.162.553.06.17h.105v-.097l.085-1.134.157-1.392.154-1.792.052-.504.25-.605.497-.327.387.186.319.456-.045.294-.19 1.23-.37 1.93-.243 1.29h.142l.161-.16.654-.868 1.097-1.372.484-.545.565-.601.363-.287h.686l.505.751-.226.775-.707.895-.585.759-.839 1.13-.524.904.048.072.125-.012 1.897-.403 1.024-.186 1.223-.21.553.258.06.263-.218.536-1.307.323-1.533.307-2.284.54-.028.02.032.04 1.029.098.44.024h1.077l2.005.15.525.346.315.424-.053.323-.807.411-3.631-.863-.872-.218h-.12v.073l.726.71 1.331 1.202 1.667 1.55.084.383-.214.302-.226-.032-1.464-1.101-.565-.497-1.28-1.077h-.084v.113l.295.432 1.557 2.34.08.718-.112.234-.404.141-.444-.08-.911-1.28-.94-1.44-.759-1.291-.093.053-.448 4.821-.21.246-.484.186-.403-.307-.214-.496.214-.98.258-1.28.21-1.016.19-1.263.112-.42-.008-.028-.092.012-.953 1.307-1.448 1.957-1.146 1.227-.274.109-.477-.247.045-.44.266-.39 1.586-2.018.956-1.25.617-.723-.004-.105h-.036l-4.212 2.736-.75.096-.324-.302.04-.496.154-.162 1.267-.871z"/></svg>',
    )}`,
    kimi: svgAsset(
      'M21.765.351C22.998.351 24 1.353 24 2.586S22.998 4.82 21.765 4.82h-1.974c-.15 0-.26-.12-.26-.26V2.586A2.237 2.237 0 0 1 21.765.35M9.41 13.388l8.447-8.377c.16-.16.07-.471-.14-.471h-4.55s-.1.02-.14.06l-9.099 9.029c-.14.14-.35.02-.35-.21V4.81c0-.15-.1-.27-.221-.27H.22c-.12 0-.22.12-.22.27v18.57c0 .15.1.27.22.27h3.137c.12 0 .22-.12.22-.27v-3.79c0-.08.03-.16.08-.21l2.826-2.796c.07-.07.16-.08.241-.03l7.546 5.551a8.9 8.9 0 0 0 4.018 1.493c.12.01.23-.11.23-.27V19.76c0-.14-.08-.25-.19-.26a5.8 5.8 0 0 1-2.355-.942l-6.533-4.73c-.14-.09-.15-.32-.03-.441',
      '#111827',
    ),
    deepseek: svgAsset(
      'M23.748 4.651c-.254-.124-.364.113-.512.233-.051.04-.094.09-.137.137-.372.397-.806.657-1.373.626-.829-.046-1.537.214-2.163.848-.133-.782-.575-1.248-1.247-1.548-.352-.155-.708-.311-.955-.65-.172-.24-.219-.509-.305-.774-.055-.16-.11-.323-.293-.35-.2-.031-.278.136-.356.276-.313.572-.434 1.202-.422 1.84.027 1.436.633 2.58 1.838 3.393.137.094.172.187.129.323-.082.28-.18.553-.266.833-.055.179-.137.218-.328.14a5.5 5.5 0 0 1-1.737-1.179c-.857-.828-1.631-1.743-2.597-2.46a12 12 0 0 0-.689-.47c-.985-.957.13-1.743.387-1.836.27-.098.094-.433-.778-.428-.872.003-1.67.295-2.687.685a3 3 0 0 1-.465.136 9.6 9.6 0 0 0-2.883-.101c-1.885.21-3.39 1.1-4.497 2.622C.082 8.776-.231 10.854.152 13.02c.403 2.284 1.568 4.175 3.36 5.653 1.857 1.533 3.997 2.284 6.438 2.14 1.482-.085 3.132-.284 4.994-1.86.47.234.962.328 1.78.398.629.058 1.235-.031 1.705-.129.735-.155.684-.836.418-.961-2.155-1.004-1.682-.595-2.112-.926 1.095-1.295 2.768-3.598 3.284-6.733.05-.346.115-.834.108-1.114-.004-.171.035-.238.23-.257a4.2 4.2 0 0 0 1.545-.475c1.397-.763 1.96-2.016 2.093-3.517.02-.23-.004-.467-.247-.588M11.58 18.168c-2.088-1.642-3.101-2.183-3.52-2.16-.39.024-.32.472-.234.763.09.288.207.487.371.74.114.167.192.416-.113.603-.673.416-1.842-.14-1.897-.168-1.361-.801-2.5-1.86-3.301-3.306-.775-1.393-1.225-2.888-1.299-4.482-.02-.385.094-.522.477-.592a4.7 4.7 0 0 1 1.53-.038c2.131.311 3.946 1.264 5.467 2.774.868.86 1.525 1.887 2.202 2.89.72 1.066 1.494 2.082 2.48 2.915.348.291.626.513.892.677-.802.09-2.14.109-3.055-.615zm1.001-6.44a.306.306 0 0 1 .415-.287.3.3 0 0 1 .113.074.3.3 0 0 1 .086.214c0 .17-.136.307-.308.307a.303.303 0 0 1-.306-.307m3.11 1.596c-.2.081-.4.151-.591.16a1.25 1.25 0 0 1-.798-.254c-.274-.23-.47-.358-.551-.758a1.7 1.7 0 0 1 .015-.588c.07-.327-.007-.537-.238-.727-.188-.156-.426-.199-.689-.199a.6.6 0 0 1-.254-.078.253.253 0 0 1-.114-.358 1 1 0 0 1 .192-.21c.356-.202.767-.136 1.146.016.352.144.618.408 1.001.782.392.451.462.576.685.915.176.264.336.536.446.848.066.194-.02.353-.25.45',
      '#4D6BFE',
    ),
    minimax: svgAsset(
      'M16.278 2c1.156 0 2.093.927 2.093 2.07v12.501a.74.74 0 0 0 .744.709.74.74 0 0 0 .743-.709V9.099a2.06 2.06 0 0 1 2.071-2.049A2.06 2.06 0 0 1 24 9.1v6.561a.649.649 0 0 1-.652.645.649.649 0 0 1-.653-.645V9.1a.762.762 0 0 0-.766-.758.762.762 0 0 0-.766.758v7.472a2.037 2.037 0 0 1-2.048 2.026 2.037 2.037 0 0 1-2.048-2.026v-12.5a.785.785 0 0 0-.788-.753.785.785 0 0 0-.789.752l-.001 15.904A2.037 2.037 0 0 1 13.441 22a2.037 2.037 0 0 1-2.048-2.026V18.04c0-.356.292-.645.652-.645.36 0 .652.289.652.645v1.934c0 .263.142.506.372.638.23.131.514.131.744 0a.734.734 0 0 0 .372-.638V4.07c0-1.143.937-2.07 2.093-2.07zm-5.674 0c1.156 0 2.093.927 2.093 2.07v11.523a.648.648 0 0 1-.652.645.648.648 0 0 1-.652-.645V4.07a.785.785 0 0 0-.789-.78.785.785 0 0 0-.789.78v14.013a2.06 2.06 0 0 1-2.07 2.048 2.06 2.06 0 0 1-2.071-2.048V9.1a.762.762 0 0 0-.766-.758.762.762 0 0 0-.766.758v3.8a2.06 2.06 0 0 1-2.071 2.049A2.06 2.06 0 0 1 0 12.9v-1.378c0-.357.292-.646.652-.646.36 0 .653.29.653.646V12.9c0 .418.343.757.766.757s.766-.339.766-.757V9.099a2.06 2.06 0 0 1 2.07-2.048 2.06 2.06 0 0 1 2.071 2.048v8.984c0 .419.343.758.767.758.423 0 .766-.339.766-.758V4.07c0-1.143.937-2.07 2.093-2.07z',
      '#F44763',
    ),
    zhipu: svgAsset(
      'M11.991 23.503a.24.24 0 0 0-.244.248a.24.24 0 0 0 .244.249a.24.24 0 0 0 .245-.249a.24.24 0 0 0-.22-.247zM9.671 5.365a1.697 1.697 0 0 1 1.099 2.132l-.071.172l-.016.04l-.018.054c-.07.16-.104.32-.104.498c-.035.71.47 1.279 1.186 1.314h.366c1.309.053 2.338 1.173 2.286 2.523c-.052 1.332-1.152 2.38-2.478 2.327h-.174c-.715.018-1.274.64-1.239 1.368c0 .124.018.23.053.337c.209.373.54.658.96.8c.75.23 1.517-.125 1.9-.782l.018-.035c.402-.64 1.17-.96 1.92-.711c.854.284 1.378 1.226 1.099 2.167a1.66 1.66 0 0 1-2.077 1.102a1.7 1.7 0 0 1-.907-.711l-.017-.035c-.2-.323-.463-.58-.851-.711l-.056-.018a1.646 1.646 0 0 0-1.954.746a1.66 1.66 0 0 1-1.065.764a1.677 1.677 0 0 1-1.989-1.279c-.209-.906.332-1.83 1.257-2.043a1.5 1.5 0 0 1 .296-.035h.018c.68-.071 1.151-.622 1.116-1.333a1.3 1.3 0 0 0-.227-.693a2.5 2.5 0 0 1-.366-1.403a2.4 2.4 0 0 1 .366-1.208c.14-.195.21-.444.227-.693c.018-.71-.506-1.261-1.186-1.332l-.07-.018a1.4 1.4 0 0 1-.299-.07l-.05-.019a1.7 1.7 0 0 1-1.047-2.114a1.68 1.68 0 0 1 2.094-1.101m-5.575 10.11c.26-.264.639-.367.994-.27s.633.379.728.74c.095.362-.007.748-.267 1.013c-.402.41-1.053.41-1.455 0a1.06 1.06 0 0 1 0-1.482zm14.845-.294c.359-.09.738.024.992.297c.254.274.344.665.237 1.025s-.396.634-.756.718c-.551.128-1.1-.22-1.23-.781a1.05 1.05 0 0 1 .757-1.26m-.064-4.39c.314.32.49.753.49 1.206s-.176.886-.49 1.206c-.315.32-.74.5-1.185.5c-.444 0-.87-.18-1.184-.5a1.727 1.727 0 0 1 0-2.412a1.654 1.654 0 0 1 2.369 0m-11.243.163c.364.484.447 1.128.218 1.691a1.665 1.665 0 0 1-2.188.923c-.855-.36-1.26-1.358-.907-2.228a1.68 1.68 0 0 1 1.33-1.038a1.66 1.66 0 0 1 1.547.652m11.545-4.221c.368 0 .708.2.892.524s.184.724 0 1.048a1.03 1.03 0 0 1-.892.524a1.04 1.04 0 0 1-1.03-1.048a1.04 1.04 0 0 1 1.03-1.048m-14.358 0c.368 0 .707.2.891.524s.184.724 0 1.048a1.03 1.03 0 0 1-.891.524a1.04 1.04 0 0 1-1.03-1.048c0-.579.461-1.048 1.03-1.048m10.031-1.475c.925 0 1.675.764 1.675 1.706s-.75 1.705-1.675 1.705s-1.674-.763-1.674-1.705s.75-1.706 1.674-1.706m-2.626-.684c.362-.082.653-.356.761-.718a1.06 1.06 0 0 0-.238-1.028a1.02 1.02 0 0 0-.996-.294c-.547.14-.881.7-.752 1.257c.13.558.675.907 1.225.783m0 16.876c.359-.087.644-.36.75-.72a1.06 1.06 0 0 0-.237-1.019a1.02 1.02 0 0 0-.985-.301a1.04 1.04 0 0 0-.762.717c-.108.361-.017.754.239 1.028c.245.263.606.377.953.305zM17.19 3.5a.63.63 0 0 0 .628-.64a.63.63 0 0 0-.628-.64a.63.63 0 0 0-.628.64c0 .355.28.64.628.64m-10.38 0c.368 0 .708.2.892.524s.184.724 0 1.048a1.03 1.03 0 0 1-.892.524a1.04 1.04 0 0 1-1.03-1.048c0-.579.461-1.048 1.03-1.048m-5.182 7.852a.63.63 0 0 0-.628.64c0 .354.28.639.628.639a.63.63 0 0 0 .627-.606l.001-.034a.62.62 0 0 0-.628-.64zm5.182 9.13a.63.63 0 0 0-.628.64c0 .355.279.64.628.64a.63.63 0 0 0 .628-.64c0-.355-.28-.64-.628-.64m10.38.018a.63.63 0 0 0-.628.64c0 .355.28.64.628.64a.63.63 0 0 0 .628-.64a.63.63 0 0 0-.628-.64m5.182-9.148a.63.63 0 0 0-.628.64c0 .354.279.639.628.639a.63.63 0 0 0 .628-.64c0-.355-.28-.64-.628-.64z',
      '#3859FF',
    ),
    search:
      'data:image/svg+xml;base64,PCEtLSBAbGljZW5zZSBsdWNpZGUtc3RhdGljIHYwLjQ2OC4wIC0gSVNDIC0tPgo8c3ZnCiAgY2xhc3M9Imx1Y2lkZSBsdWNpZGUtc2VhcmNoIgogIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIKICB3aWR0aD0iMjQiCiAgaGVpZ2h0PSIyNCIKICB2aWV3Qm94PSIwIDAgMjQgMjQiCiAgZmlsbD0ibm9uZSIKICBzdHJva2U9ImN1cnJlbnRDb2xvciIKICBzdHJva2Utd2lkdGg9IjIiCiAgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIgogIHN0cm9rZS1saW5lam9pbj0icm91bmQiCj4KICA8Y2lyY2xlIGN4PSIxMSIgY3k9IjExIiByPSI4IiAvPgogIDxwYXRoIGQ9Im0yMSAyMS00LjMtNC4zIiAvPgo8L3N2Zz4K',
    x: 'data:image/svg+xml;base64,PCEtLSBAbGljZW5zZSBsdWNpZGUtc3RhdGljIHYwLjQ2OC4wIC0gSVNDIC0tPgo8c3ZnCiAgY2xhc3M9Imx1Y2lkZSBsdWNpZGUteCIKICB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciCiAgd2lkdGg9IjI0IgogIGhlaWdodD0iMjQiCiAgdmlld0JveD0iMCAwIDI0IDI0IgogIGZpbGw9Im5vbmUiCiAgc3Ryb2tlPSJjdXJyZW50Q29sb3IiCiAgc3Ryb2tlLXdpZHRoPSIyIgogIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIKICBzdHJva2UtbGluZWpvaW49InJvdW5kIgo+CiAgPHBhdGggZD0iTTE4IDYgNiAxOCIgLz4KICA8cGF0aCBkPSJtNiA2IDEyIDEyIiAvPgo8L3N2Zz4K',
    'sliders-horizontal':
      'data:image/svg+xml;base64,PCEtLSBAbGljZW5zZSBsdWNpZGUtc3RhdGljIHYwLjQ2OC4wIC0gSVNDIC0tPgo8c3ZnCiAgY2xhc3M9Imx1Y2lkZSBsdWNpZGUtc2xpZGVycy1ob3Jpem9udGFsIgogIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIKICB3aWR0aD0iMjQiCiAgaGVpZ2h0PSIyNCIKICB2aWV3Qm94PSIwIDAgMjQgMjQiCiAgZmlsbD0ibm9uZSIKICBzdHJva2U9ImN1cnJlbnRDb2xvciIKICBzdHJva2Utd2lkdGg9IjIiCiAgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIgogIHN0cm9rZS1saW5lam9pbj0icm91bmQiCj4KICA8bGluZSB4MT0iMjEiIHgyPSIxNCIgeTE9IjQiIHkyPSI0IiAvPgogIDxsaW5lIHgxPSIxMCIgeDI9IjMiIHkxPSI0IiB5Mj0iNCIgLz4KICA8bGluZSB4MT0iMjEiIHgyPSIxMiIgeTE9IjEyIiB5Mj0iMTIiIC8+CiAgPGxpbmUgeDE9IjgiIHgyPSIzIiB5MT0iMTIiIHkyPSIxMiIgLz4KICA8bGluZSB4MT0iMjEiIHgyPSIxNiIgeTE9IjIwIiB5Mj0iMjAiIC8+CiAgPGxpbmUgeDE9IjEyIiB4Mj0iMyIgeTE9IjIwIiB5Mj0iMjAiIC8+CiAgPGxpbmUgeDE9IjE0IiB4Mj0iMTQiIHkxPSIyIiB5Mj0iNiIgLz4KICA8bGluZSB4MT0iOCIgeDI9IjgiIHkxPSIxMCIgeTI9IjE0IiAvPgogIDxsaW5lIHgxPSIxNiIgeDI9IjE2IiB5MT0iMTgiIHkyPSIyMiIgLz4KPC9zdmc+Cg==',
    copy: 'data:image/svg+xml;base64,PCEtLSBAbGljZW5zZSBsdWNpZGUtc3RhdGljIHYwLjQ2OC4wIC0gSVNDIC0tPgo8c3ZnCiAgY2xhc3M9Imx1Y2lkZSBsdWNpZGUtY29weSIKICB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciCiAgd2lkdGg9IjI0IgogIGhlaWdodD0iMjQiCiAgdmlld0JveD0iMCAwIDI0IDI0IgogIGZpbGw9Im5vbmUiCiAgc3Ryb2tlPSJjdXJyZW50Q29sb3IiCiAgc3Ryb2tlLXdpZHRoPSIyIgogIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIKICBzdHJva2UtbGluZWpvaW49InJvdW5kIgo+CiAgPHJlY3Qgd2lkdGg9IjE0IiBoZWlnaHQ9IjE0IiB4PSI4IiB5PSI4IiByeD0iMiIgcnk9IjIiIC8+CiAgPHBhdGggZD0iTTQgMTZjLTEuMSAwLTItLjktMi0yVjRjMC0xLjEuOS0yIDItMmgxMGMxLjEgMCAyIC45IDIgMiIgLz4KPC9zdmc+Cg==',
    'layout-grid':
      'data:image/svg+xml;base64,PCEtLSBAbGljZW5zZSBsdWNpZGUtc3RhdGljIHYwLjQ2OC4wIC0gSVNDIC0tPgo8c3ZnCiAgY2xhc3M9Imx1Y2lkZSBsdWNpZGUtbGF5b3V0LWdyaWQiCiAgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIgogIHdpZHRoPSIyNCIKICBoZWlnaHQ9IjI0IgogIHZpZXdCb3g9IjAgMCAyNCAyNCIKICBmaWxsPSJub25lIgogIHN0cm9rZT0iY3VycmVudENvbG9yIgogIHN0cm9rZS13aWR0aD0iMiIKICBzdHJva2UtbGluZWNhcD0icm91bmQiCiAgc3Ryb2tlLWxpbmVqb2luPSJyb3VuZCIKPgogIDxyZWN0IHdpZHRoPSI3IiBoZWlnaHQ9IjciIHg9IjMiIHk9IjMiIHJ4PSIxIiAvPgogIDxyZWN0IHdpZHRoPSI3IiBoZWlnaHQ9IjciIHg9IjE0IiB5PSIzIiByeD0iMSIgLz4KICA8cmVjdCB3aWR0aD0iNyIgaGVpZ2h0PSI3IiB4PSIxNCIgeT0iMTQiIHJ4PSIxIiAvPgogIDxyZWN0IHdpZHRoPSI3IiBoZWlnaHQ9IjciIHg9IjMiIHk9IjE0IiByeD0iMSIgLz4KPC9zdmc+Cg==',
    'table-2':
      'data:image/svg+xml;base64,PCEtLSBAbGljZW5zZSBsdWNpZGUtc3RhdGljIHYwLjQ2OC4wIC0gSVNDIC0tPgo8c3ZnCiAgY2xhc3M9Imx1Y2lkZSBsdWNpZGUtdGFibGUtMiIKICB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciCiAgd2lkdGg9IjI0IgogIGhlaWdodD0iMjQiCiAgdmlld0JveD0iMCAwIDI0IDI0IgogIGZpbGw9Im5vbmUiCiAgc3Ryb2tlPSJjdXJyZW50Q29sb3IiCiAgc3Ryb2tlLXdpZHRoPSIyIgogIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIKICBzdHJva2UtbGluZWpvaW49InJvdW5kIgo+CiAgPHBhdGggZD0iTTkgM0g1YTIgMiAwIDAgMC0yIDJ2NG02LTZoMTBhMiAyIDAgMCAxIDIgMnY0TTkgM3YxOG0wIDBoMTBhMiAyIDAgMCAwIDItMlY5TTkgMjFINWEyIDIgMCAwIDEtMi0yVjltMCAwaDE4IiAvPgo8L3N2Zz4K',
    'refresh-cw':
      'data:image/svg+xml;base64,PCEtLSBAbGljZW5zZSBsdWNpZGUtc3RhdGljIHYwLjQ2OC4wIC0gSVNDIC0tPgo8c3ZnCiAgY2xhc3M9Imx1Y2lkZSBsdWNpZGUtcmVmcmVzaC1jdyIKICB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciCiAgd2lkdGg9IjI0IgogIGhlaWdodD0iMjQiCiAgdmlld0JveD0iMCAwIDI0IDI0IgogIGZpbGw9Im5vbmUiCiAgc3Ryb2tlPSJjdXJyZW50Q29sb3IiCiAgc3Ryb2tlLXdpZHRoPSIyIgogIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIKICBzdHJva2UtbGluZWpvaW49InJvdW5kIgo+CiAgPHBhdGggZD0iTTMgMTJhOSA5IDAgMCAxIDktOSA5Ljc1IDkuNzUgMCAwIDEgNi43NCAyLjc0TDIxIDgiIC8+CiAgPHBhdGggZD0iTTIxIDN2NWgtNSIgLz4KICA8cGF0aCBkPSJNMjEgMTJhOSA5IDAgMCAxLTkgOSA5Ljc1IDkuNzUgMCAwIDEtNi43NC0yLjc0TDMgMTYiIC8+CiAgPHBhdGggZD0iTTggMTZIM3Y1IiAvPgo8L3N2Zz4K',
    'chevron-right':
      'data:image/svg+xml;base64,PCEtLSBAbGljZW5zZSBsdWNpZGUtc3RhdGljIHYwLjQ2OC4wIC0gSVNDIC0tPgo8c3ZnCiAgY2xhc3M9Imx1Y2lkZSBsdWNpZGUtY2hldnJvbi1yaWdodCIKICB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciCiAgd2lkdGg9IjI0IgogIGhlaWdodD0iMjQiCiAgdmlld0JveD0iMCAwIDI0IDI0IgogIGZpbGw9Im5vbmUiCiAgc3Ryb2tlPSJjdXJyZW50Q29sb3IiCiAgc3Ryb2tlLXdpZHRoPSIyIgogIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIKICBzdHJva2UtbGluZWpvaW49InJvdW5kIgo+CiAgPHBhdGggZD0ibTkgMTggNi02LTYtNiIgLz4KPC9zdmc+Cg==',
    unplug:
      'data:image/svg+xml;base64,PCEtLSBAbGljZW5zZSBsdWNpZGUtc3RhdGljIHYwLjQ2OC4wIC0gSVNDIC0tPgo8c3ZnCiAgY2xhc3M9Imx1Y2lkZSBsdWNpZGUtdW5wbHVnIgogIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIKICB3aWR0aD0iMjQiCiAgaGVpZ2h0PSIyNCIKICB2aWV3Qm94PSIwIDAgMjQgMjQiCiAgZmlsbD0ibm9uZSIKICBzdHJva2U9ImN1cnJlbnRDb2xvciIKICBzdHJva2Utd2lkdGg9IjIiCiAgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIgogIHN0cm9rZS1saW5lam9pbj0icm91bmQiCj4KICA8cGF0aCBkPSJtMTkgNSAzLTMiIC8+CiAgPHBhdGggZD0ibTIgMjIgMy0zIiAvPgogIDxwYXRoIGQ9Ik02LjMgMjAuM2EyLjQgMi40IDAgMCAwIDMuNCAwTDEyIDE4bC02LTYtMi4zIDIuM2EyLjQgMi40IDAgMCAwIDAgMy40WiIgLz4KICA8cGF0aCBkPSJNNy41IDEzLjUgMTAgMTEiIC8+CiAgPHBhdGggZD0iTTEwLjUgMTYuNSAxMyAxNCIgLz4KICA8cGF0aCBkPSJtMTIgNiA2IDYgMi4zLTIuM2EyLjQgMi40IDAgMCAwIDAtMy40bC0yLjYtMi42YTIuNCAyLjQgMCAwIDAtMy40IDBaIiAvPgo8L3N2Zz4K',
    openai:
      'data:image/svg+xml;base64,PHN2ZyBmaWxsPSJjdXJyZW50Q29sb3IiIGZpbGwtcnVsZT0iZXZlbm9kZCIgaGVpZ2h0PSIxZW0iIHN0eWxlPSJmbGV4Om5vbmU7bGluZS1oZWlnaHQ6MSIgdmlld0JveD0iMCAwIDI0IDI0IiB3aWR0aD0iMWVtIiB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciPjx0aXRsZT5PcGVuQUk8L3RpdGxlPjxwYXRoIGQ9Ik05LjIwNSA4LjY1OHYtMi4yNmMwLS4xOS4wNzItLjMzMy4yMzgtLjQyOGw0LjU0My0yLjYxNmMuNjE5LS4zNTcgMS4zNTYtLjUyMyAyLjExNy0uNTIzIDIuODU0IDAgNC42NjIgMi4yMTIgNC42NjIgNC41NjYgMCAuMTY3IDAgLjM1Ny0uMDI0LjU0N2wtNC43MS0yLjc1OWEuNzk3Ljc5NyAwIDAwLS44NTYgMGwtNS45NyAzLjQ3M3ptMTAuNjA5IDguOFYxMi4wNmMwLS4zMzMtLjE0My0uNTctLjQyOS0uNzM3bC01Ljk3LTMuNDczIDEuOTUtMS4xMThhLjQzMy40MzMgMCAwMS40NzYgMGw0LjU0MyAyLjYxN2MxLjMwOS43NiAyLjE4OSAyLjM3OCAyLjE4OSAzLjk0OCAwIDEuODA4LTEuMDcgMy40NzMtMi43NiA0LjE2M3pNNy44MDIgMTIuNzAzbC0xLjk1LTEuMTQyYy0uMTY3LS4wOTUtLjIzOS0uMjM4LS4yMzktLjQyOFY1Ljg5OWMwLTIuNTQ1IDEuOTUtNC40NzIgNC41OTEtNC40NzIgMSAwIDEuOTI3LjMzMyAyLjcxMi45MjhMOC4yMyA1LjA2N2MtLjI4NS4xNjYtLjQyOC40MDQtLjQyOC43Mzd2Ni44OTh6TTEyIDE1LjEyOGwtMi43OTUtMS41N3YtMy4zM0wxMiA4LjY1OGwyLjc5NSAxLjU3djMuMzNMMTIgMTUuMTI4em0xLjc5NiA3LjIzYy0xIDAtMS45MjctLjMzMi0yLjcxMi0uOTI3bDQuNjg2LTIuNzEyYy4yODUtLjE2Ni40MjgtLjQwNC40MjgtLjczN3YtNi44OThsMS45NzQgMS4xNDJjLjE2Ny4wOTUuMjM4LjIzOC4yMzguNDI4djUuMjMzYzAgMi41NDUtMS45NzQgNC40NzItNC42MTQgNC40NzJ6bS01LjYzNy01LjMwM2wtNC41NDQtMi42MTdjLTEuMzA4LS43NjEtMi4xODgtMi4zNzgtMi4xODgtMy45NDhBNC40ODIgNC40ODIgMCAwMTQuMjEgNi4zMjd2NS40MjNjMCAuMzMzLjE0My41NzEuNDI4LjczOGw1Ljk0NyAzLjQ0OS0xLjk1IDEuMTE4YS40MzIuNDMyIDAgMDEtLjQ3NiAwem0tLjI2MiAzLjljLTIuNjg4IDAtNC42NjItMi4wMjEtNC42NjItNC41MTkgMC0uMTkuMDI0LS4zOC4wNDctLjU3bDQuNjg2IDIuNzFjLjI4Ni4xNjcuNTcxLjE2Ny44NTYgMGw1Ljk3LTMuNDQ4djIuMjZjMCAuMTktLjA3LjMzMy0uMjM3LjQyOGwtNC41NDMgMi42MTZjLS42MTkuMzU3LTEuMzU2LjUyMy0yLjExNy41MjN6bTUuODk5IDIuODNhNS45NDcgNS45NDcgMCAwMDUuODI3LTQuNzU2QzIyLjI4NyAxOC4zMzkgMjQgMTUuODQgMjQgMTMuMjk2YzAtMS42NjUtLjcxMy0zLjI4Mi0xLjk5OC00LjQ0OC4xMTktLjUuMTktLjk5OS4xOS0xLjQ5OCAwLTMuNDAxLTIuNzU5LTUuOTQ3LTUuOTQ2LTUuOTQ3LS42NDIgMC0xLjI2LjA5NS0xLjg4LjMxQTUuOTYyIDUuOTYyIDAgMDAxMC4yMDUgMGE1Ljk0NyA1Ljk0NyAwIDAwLTUuODI3IDQuNzU3QzEuNzEzIDUuNDQ3IDAgNy45NDUgMCAxMC40OWMwIDEuNjY2LjcxMyAzLjI4MyAxLjk5OCA0LjQ0OC0uMTE5LjUtLjE5IDEtLjE5IDEuNDk5IDAgMy40MDEgMi43NTkgNS45NDYgNS45NDYgNS45NDYuNjQyIDAgMS4yNi0uMDk1IDEuODgtLjMwOWE1Ljk2IDUuOTYgMCAwMDQuMTYyIDEuNzEzeiI+PC9wYXRoPjwvc3ZnPg==',
  }
  const $ = (id) => root.getElementById(id)
  const escapeHTML = (value) =>
    String(value ?? '').replace(
      /[&<>"']/g,
      (c) =>
        ({
          '&': '&amp;',
          '<': '&lt;',
          '>': '&gt;',
          '"': '&quot;',
          "'": '&#39;',
        })[c],
    )
  const icon = (name) => `<img src="${ASSETS[name] || ''}" alt="">`
  root
    .querySelectorAll('[data-icon]')
    .forEach((el) => (el.src = ASSETS[el.dataset.icon]))
  $('anonymous-hint').hidden = authenticated
  const state = {
    groups: [],
    models: [],
    provider: '',
    billing: '',
    supplier: '',
    query: '',
    unit: 1000000,
    compare: true,
    view: 'grid',
    sort: 'default',
    selected: new Set(),
    visible: [],
    loading: false,
    error: '',
  }
  const providerNames = {
    openai: 'OpenAI',
    anthropic: 'Anthropic',
    gemini: 'Google',
    google: 'Google',
    kimi: 'Kimi',
    moonshot: 'Kimi',
    deepseek: 'DeepSeek',
    minimax: 'MiniMax',
    zhipu: 'Zhipu GLM',
    qwen: '通义千问',
    alibaba: '通义千问',
    dashscope: '通义千问',
    doubao: '字节跳动',
  }
  const PROVIDER_ICONS = new Set([
    'openai',
    'anthropic',
    'gemini',
    'google',
    'kimi',
    'moonshot',
    'deepseek',
    'minimax',
    'zhipu',
    'qwen',
    'doubao',
  ])
  function providerForModel(name, platform) {
    const model = String(name || '').toLowerCase()
    // Model identity takes precedence over the account's upstream protocol.
    const rules = [
      ['doubao', /seedance|doubao/],
      ['gemini', /gemini|gemma|learnlm|imagen-|veo-/],
      ['anthropic', /claude/],
      ['qwen', /qwen|qwq/],
      ['minimax', /minimax|abab/],
      ['deepseek', /deepseek/],
      ['kimi', /kimi|moonshot/],
      ['zhipu', /glm|cogview|cogvideo/],
      ['openai', /(?:^|\/)(?:gpt|o[134](?:-|$))|chatgpt|dall-e|whisper|tts-1|text-embedding-3|text-moderation/],
    ]
    return rules.find(([, pattern]) => pattern.test(model))?.[0] || platform || 'unknown'
  }
  const billingNames = {
    token: '按量计费',
    request: '按次计费',
    image: '按张计费',
    video: '视频计费',
    parameter: '图片（参数）',
    video_token: '视频（Token）',
  }
  const rows = [
    ['input_price', '输入'],
    ['output_price', '输出'],
    ['cache_read_price', '缓存读取'],
    ['cache_write_price', '缓存写入 / 5m'],
    ['cache_write_1h_price', '缓存写入 / 1h'],
    ['image_input_price', '图片输入'],
    ['image_output_price', '图片输出'],
    ['per_request_price', '单次请求'],
  ]
  const mode = (model) => model.pricing?.billing_mode || 'token'
  const baseMultiplier = (group) =>
    group.user_rate_multiplier ?? group.rate_multiplier ?? 1
  function multiplier(variant, key) {
    const g = variant.group,
      m = mode(variant.model)
    if (
      key === 'per_request_price' &&
      (m === 'image' || m === 'parameter') &&
      g.image_rate_independent
    )
      return g.image_rate_multiplier ?? 1
    if (
      key === 'per_request_price' &&
      (m === 'video' || m === 'video_token') &&
      g.video_rate_independent
    )
      return Math.max(0, g.video_rate_multiplier ?? 1)
    return baseMultiplier(g)
  }
  function basePricing(model) {
    const base = model.pricing
    const first = [...(base?.intervals || [])].sort(
      (a, b) => a.min_tokens - b.min_tokens,
    )[0]
    return base && first
      ? {
          ...base,
          ...resolveIntervalPrices(first, base),
          per_request_price: first.per_request_price ?? base.per_request_price,
        }
      : base
  }
  function price(variant, key, pricing = basePricing(variant.model)) {
    const value = pricing?.[key]
    return typeof value === 'number' && Number.isFinite(value)
      ? value * multiplier(variant, key)
      : null
  }
  function money(value, key = 'input_price') {
    if (value == null || !Number.isFinite(value)) return '—'
    return formatScaled(value, key === 'per_request_price' ? 1 : state.unit)
  }
  function discount(actual, reference) {
    if (actual == null || reference == null || reference <= 0) return '—'
    const ratio = actual / reference
    if (Math.abs(ratio - 1) < 1e-6)
      return '<span class="discount full">原价</span>'
    return `<span class="discount ${ratio > 1 ? 'premium' : ''}">${Number((ratio * 10).toFixed(2))}折</span>`
  }
  function brand(provider) {
    const name = providerNames[provider] || provider || '?'
    return `<span class="brand">${PROVIDER_ICONS.has(provider) ? `<img src="${ASSETS[provider]}" alt="${escapeHTML(name)}">` : `<span class="brand-fallback">${escapeHTML(name.slice(0, 2))}</span>`}</span>`
  }
  function selectedVariants(item) {
    return item.variants.filter(
      (v) =>
        (!state.billing || mode(v.model) === state.billing),
    )
  }
  function representative(item) {
    return selectedVariants(item)[0]
  }
  function filtered() {
    return state.models
      .filter(
        (item) =>
          (!state.provider || item.provider === state.provider) &&
          (!state.supplier || item.supplier === state.supplier) &&
          item.name.toLowerCase().includes(state.query.toLowerCase().trim()) &&
          selectedVariants(item).some(
            (v) => !state.billing || mode(v.model) === state.billing,
          ),
      )
      .sort((a, b) => {
        if (state.sort === 'name')
          return a.displayName.localeCompare(b.displayName) || a.supplier.localeCompare(b.supplier)
        if (state.sort.startsWith('price-')) {
          const av = representative(a),
            bv = representative(b)
          const x =
              mode(av.model) === 'token' ? price(av, 'input_price') : null,
            y = mode(bv.model) === 'token' ? price(bv, 'input_price') : null
          if (x == null) return y == null ? 0 : 1
          if (y == null) return -1
          return state.sort === 'price-asc' ? x - y : y - x
        }
        return a.order - b.order
      })
  }
  function chip(value, label, active, type, count, logo = '') {
    const image = PROVIDER_ICONS.has(logo) ? ASSETS[logo] : ''
    return `<button class="chip ${active === value ? 'active' : ''}" data-filter="${type}" data-value="${escapeHTML(value)}" aria-pressed="${active === value}">${image ? `<img src="${image}" alt="">` : ''}<span>${escapeHTML(label)}</span>${count == null ? '' : `<span class="count">${count}</span>`}</button>`
  }
  function renderFilters() {
    const providers = [...new Set(state.models.map((m) => m.provider))]
    $('providers').innerHTML =
      chip('', '全部模型', state.provider, 'provider') +
      providers
        .map((p) =>
          chip(
            p,
            providerNames[p] || p,
            state.provider,
            'provider',
            state.models.filter((m) => m.provider === p).length,
            PROVIDER_ICONS.has(p) ? p : '',
          ),
        )
        .join('')
    const modes = [
      ...new Set(
        state.models.flatMap((m) => m.variants.map((v) => mode(v.model))),
      ),
    ]
    $('billing').innerHTML =
      chip('', '全部类型', state.billing, 'billing') +
      modes
        .map((m) => chip(m, billingNames[m] || m, state.billing, 'billing'))
        .join('')
    const suppliers = [...new Set(state.models.map((m) => m.supplier))]
    $('suppliers').innerHTML =
      chip('', '全部渠道', state.supplier, 'supplier', suppliers.length) +
      suppliers.map((supplier) =>
        chip(supplier, supplier, state.supplier, 'supplier',
          state.models.filter((m) => m.supplier === supplier).length),
      ).join('')
  }
  function priceTable(
    variant,
    pricing = basePricing(variant.model),
    reference = variant.model.official_pricing,
    detailed = false,
  ) {
    if (mode(variant.model) === 'video' && pricing?.video_task_pricing) {
      const rule = pricing.video_task_pricing
      const isCNY = rule.currency === 'CNY'
      const rate = multiplier(variant, 'per_request_price')
      const formatter = new Intl.NumberFormat(isCNY ? 'zh-CN' : 'en-US', {
        style: 'currency',
        currency: isCNY ? 'CNY' : 'USD',
        minimumFractionDigits: 0,
        maximumSignificantDigits: 10,
      })
      const displayPrice = (value) => Number.isFinite(Number(value) * rate)
        ? formatter.format(Number(value) * rate)
        : '—'
      const charges = [
        ['480P / 秒', rule.price_480p_per_second],
        ['768P / 秒', rule.price_768p_per_second],
        ['2K / 秒（含基础费）', Number(rule.price_768p_per_second) + Number(rule.upgrade_2k_per_second)],
        ['提示词优化 / 次', rule.prompt_polish_price_per_call],
      ]
      return `<div class="price-box"><table class="prices"><thead><tr><th>价格项</th><th>平台价 / ${isCNY ? '人民币' : 'USD'}</th></tr></thead><tbody>${charges.map(([label, value]) => `<tr><td>${escapeHTML(label)}</td><td class="actual">${displayPrice(value)}</td></tr>`).join('')}</tbody></table></div>${detailed ? `<p class="detail-note">2K 单价 = 768P 基础价 + 2K 升级附加费。费用按每秒价格 × 视频秒数，加已启用的提示词优化费计算。以上价格已计入所选分组的视频倍率。</p><p class="detail-note">提示词优化默认${rule.prompt_polish_default_enabled ? '开启' : '关闭'}，仅开启时计费。${isCNY ? `以上为人民币展示价格，结算按配置汇率 ${escapeHTML(rule.cny_per_usd)} CNY/USD 换算为美元。` : ''}</p>` : ''}`
    }
    if (mode(variant.model) === 'video_token') {
      const rule = pricing?.video_token_pricing
      if (!rule) return '<div class="detail-note">暂无视频 Token 规则</div>'
      const rate = multiplier(variant, 'per_request_price')
      const manualPerRequest = variant.model.reference_pricing?.pricing?.per_request_price
      return `<div class="price-box"><table class="prices"><thead><tr><th>档位</th><th>无视频输入 / M</th><th>含视频输入 / M</th></tr></thead><tbody>${Object.entries(rule.tiers).map(([name, tier]) => `<tr><td>${escapeHTML(name)}</td><td class="actual">${formatScaled(Number(tier.without_video) * rate, 1)}</td><td class="actual">${formatScaled(Number(tier.with_video) * rate, 1)}</td></tr>`).join('')}</tbody></table></div>${detailed ? `<p class="detail-note">Token = floor((计费输入秒数 + 输出秒数) × 宽 × 高 × 24 / 1024)。含视频输入至少计输出时长的 2/3；按生成结果结算。</p>${manualPerRequest == null ? '' : `<p class="detail-note">人工维护参考价：${money(manualPerRequest, 'per_request_price')}/次（不同计费单位，不参与折扣比较）</p>`}` : ''}`
    }
    if (mode(variant.model) === 'parameter') {
      const rule = pricing?.parameter_pricing
      if (!rule) return '<div class="detail-note">暂无参数计费规则</div>'
      const rate = variant.group.image_rate_independent
        ? (variant.group.image_rate_multiplier ?? 1)
        : baseMultiplier(variant.group)
      const base = Number(rule.base_price) * rate
      const manualPerRequest = variant.model.reference_pricing?.pricing?.per_request_price
      return `<div class="price-box"><table class="prices"><tbody><tr><td>基础价 / 张</td><td class="actual">${formatScaled(base, 1)}</td></tr></tbody></table></div><p class="detail-note">实际费用 = 基础价 × 参数倍率 × 实际图片张数</p>${detailed ? `${rule.factors.map(f => `<p class="detail-note"><strong>${escapeHTML(f.key)}</strong>: ${Object.entries(f.values).map(([k,v]) => `${escapeHTML(k)} ×${escapeHTML(v)}`).join(' / ')}${f.default ? `；缺省 ${escapeHTML(f.default)}` : '；必传'}</p>`).join('')}${manualPerRequest == null ? '' : `<p class="detail-note">人工维护参考价：${money(manualPerRequest, 'per_request_price')}/张（不同计费单位，不参与折扣比较）</p>`}` : ''}`
    }
    const hasToken = mode(variant.model) === 'token'
    const manualReference = variant.model.reference_pricing?.pricing
    reference = { ...(reference || {}), ...(manualReference || {}) }
    // Catalog prices are per token; only an explicitly entered manual request
    // price has a comparable unit for image/video billing.
    if (!hasToken) {
      reference = manualReference?.per_request_price == null
        ? null
        : { per_request_price: manualReference.per_request_price }
    }
    let displayRows = hasToken
      ? detailed
        ? rows.filter(
            ([key]) =>
              key !== 'per_request_price' &&
              (pricing?.[key] != null || reference?.[key] != null) &&
              (!['image_input_price', 'image_output_price'].includes(key) ||
                Number(pricing?.[key]) > 0 || Number(reference?.[key]) > 0),
          )
        : rows.slice(0, 3)
      : rows.filter(([key]) => key === 'per_request_price')
    if (!displayRows.length) return '<div class="detail-note">暂无价格</div>'
    const suffix = hasToken
      ? `/${state.unit === 1000000 ? 'M' : 'K'}`
      : mode(variant.model) === 'image'
        ? '/张'
        : '/次'
    return `<div class="price-box"><table class="prices ${detailed ? 'detail-prices' : ''}"><thead><tr><th scope="col">价格项</th><th scope="col">平台价${suffix}</th>${state.compare ? `<th scope="col">参考价${suffix}</th><th scope="col">折扣</th>` : ''}</tr></thead><tbody>${displayRows
      .map(([key, label]) => {
        const actual = price(variant, key, pricing),
          ref = reference?.[key]
        return `<tr data-price="${key}"><td>${escapeHTML(label)}</td><td class="actual">${money(actual, key)}</td>${state.compare ? `<td class="reference ${actual != null && ref != null && actual < ref ? 'discounted' : ''}">${money(ref, key)}</td><td>${discount(actual, ref)}</td>` : ''}</tr>`
      })
      .join('')}</tbody></table></div>`
  }
  function card(item, index) {
    const v = representative(item),
      selected = state.selected.has(item.key)
    const intervals =
      v.group.long_context_pricing_enabled === false
        ? []
        : v.model.pricing?.intervals || []
    return `<article class="model-card ${selected ? 'selected' : ''}" data-model="${escapeHTML(item.name)}">
    <div class="card-head">${brand(item.provider)}<div class="card-identity"><h2>${escapeHTML(item.displayName)}</h2><div class="badges"><span class="badge group-badge" title="渠道：${escapeHTML(item.supplier)}">${escapeHTML(item.supplier)}</span>${intervals.length > 1 ? '<span class="badge tier-badge">阶梯定价</span>' : '<span class="badge tier-badge">标准定价</span>'}</div></div><div class="card-actions"><button class="icon-button quiet" data-copy="${index}" title="复制模型名称" aria-label="复制 ${escapeHTML(item.name)}">${icon('copy')}</button><input class="model-check" type="checkbox" data-select="${index}" ${selected ? 'checked' : ''} aria-label="选择 ${escapeHTML(item.name)}"></div></div>
    ${priceTable(v)}
    <div class="base-note">展示分组：${escapeHTML(v.group.name)} · ${intervals.length > 1 ? '首档价格' : '标准价格'}</div>
    <div class="card-footer"><div class="badges"><span class="tag">${escapeHTML(billingNames[mode(v.model)] || mode(v.model))}</span></div><button class="details-button" data-detail="${index}">详情 ${icon('chevron-right')}</button></div></article>`
  }
  function listTable(items) {
    const unit = state.unit === 1000000 ? 'M' : 'K'
    return `<div class="table-wrap"><table class="model-table"><thead><tr><th></th><th>模型</th><th>渠道</th><th>输入 /${unit}</th><th>输出 /${unit}</th><th>缓存读取 /${unit}</th><th>按次 / 媒体</th><th>操作</th></tr></thead><tbody>${items
      .map((item, i) => {
        const v = representative(item)
        const billing = mode(v.model)
        const base = v.model.pricing?.parameter_pricing?.base_price
        const mediaPrice = billing === 'video' && v.model.pricing?.video_task_pricing
          ? '按秒计费 · 详情'
          : billing === 'video_token'
          ? '视频 Token / 百万'
          : billing === 'parameter'
          ? `基础价 ${money(base == null ? null : Number(base) * multiplier(v, 'per_request_price'), 'per_request_price')}/张`
          : billing === 'token' ? '—'
            : `${money(price(v, 'per_request_price'), 'per_request_price')}/${billing === 'image' ? '张' : '次'}`
        return `<tr><td><input type="checkbox" class="model-check" data-select="${i}" ${state.selected.has(item.key) ? 'checked' : ''} aria-label="选择 ${escapeHTML(item.name)}"></td><td class="name">${escapeHTML(item.displayName)}</td><td>${escapeHTML(item.supplier)}</td>${['input_price', 'output_price', 'cache_read_price'].map((key) => `<td class="money">${billing === 'token' ? money(price(v, key), key) : '—'}</td>`).join('')}<td class="money">${mediaPrice}</td><td><button class="details-button" data-detail="${i}">详情 ${icon('chevron-right')}</button></td></tr>`
      })
      .join('')}</tbody></table></div>`
  }
  function updateSelection() {
    const visible = state.visible,
      checked = visible.filter((m) => state.selected.has(m.key)).length
    $('select-all').checked = visible.length > 0 && checked === visible.length
    $('select-all').indeterminate = checked > 0 && checked < visible.length
    $('select-all').disabled = !visible.length
    $('copy-selected').disabled = state.selected.size === 0
    $('copy-selected').querySelector('span').textContent = state.selected.size
      ? `复制 (${state.selected.size})`
      : '复制'
  }
  function render() {
    state.visible = state.loading || state.error ? [] : filtered()
    $('results').className = state.view === 'grid' ? 'grid' : ''
    $('results').setAttribute('aria-busy', String(state.loading))
    $('model-total').textContent = `共 ${state.models.length} 个模型`
    $('page-title').textContent = [
      state.provider && (providerNames[state.provider] || state.provider),
      state.supplier,
    ].filter(Boolean).join(' · ') || '全部模型'
    $('result-count').innerHTML = state.loading
      ? '正在获取模型…'
      : state.error
        ? '数据加载失败'
        : `显示 <strong>${state.visible.length}</strong> / ${state.models.length} 个模型`
    $('clear-search').hidden = !state.query
    if (state.loading)
      $('results').innerHTML = Array(6)
        .fill('<div class="skeleton"></div>')
        .join('')
    else if (state.error)
      $('results').innerHTML =
        `<div class="state">${icon('unplug')}<h2>暂时无法获取模型</h2><p>${escapeHTML(state.error)}</p><button data-action="retry">重新加载</button></div>`
    else if (!state.visible.length)
      $('results').innerHTML =
        `<div class="state">${icon('search')}<h2>没有找到匹配的模型</h2><p>试试其他关键词或调整筛选条件。</p><button data-action="reset">重置筛选</button></div>`
    else
      $('results').innerHTML =
        state.view === 'grid'
          ? state.visible.map(card).join('')
          : listTable(state.visible)
    ;['grid', 'table'].forEach((view) => {
      const b = $(view + '-view')
      b.classList.toggle('active', state.view === view)
      b.setAttribute('aria-pressed', String(state.view === view))
    })
    updateSelection()
  }
  let requestId = 0
  async function load() {
    const id = ++requestId
    state.loading = true
    state.error = ''
    render()
    try {
      const payload = await fetchPlaza()
      if (!Array.isArray(payload?.groups))
        throw new Error('接口数据格式不正确。')
      if (disposed || id !== requestId) return
      $('description').innerHTML = DOMPurify.sanitize(
        marked.parse(payload.description || ''),
      )
      $('description').hidden = !payload.description?.trim()
      const groups = payload.groups
        .filter((g) => Array.isArray(g.models))
        .sort((a, b) => baseMultiplier(a) - baseMultiplier(b) || a.id - b.id)
      const map = new Map()
      groups.forEach((group) =>
        group.models.forEach((model) => {
          if (typeof model.name !== 'string') return
          const platform = model.platform || group.platform || 'unknown',
            key = JSON.stringify([platform, model.name])
          const separator = model.name.lastIndexOf('/')
          const hasSupplier = separator > 0 && model.name.slice(separator + 1).trim()
          const displayName = hasSupplier ? model.name.slice(0, separator) : model.name
          const supplier = hasSupplier ? model.name.slice(separator + 1).trim() : '未标注'
          if (!map.has(key))
            map.set(key, {
              key,
              name: model.name,
              displayName,
              supplier,
              platform,
              provider: providerForModel(displayName, platform),
              order: map.size,
              variants: [],
            })
          map.get(key).variants.push({ model, group })
        }),
      )
      state.groups = groups
      state.models = [...map.values()]
      state.selected = new Set(
        [...state.selected].filter((key) => map.has(key)),
      )
      if (!state.models.some((m) => m.supplier === state.supplier))
        state.supplier = ''
      if (!state.models.some((m) => m.provider === state.provider))
        state.provider = ''
      if (
        !state.models.some((m) =>
          m.variants.some((v) => mode(v.model) === state.billing),
        )
      )
        state.billing = ''
      renderFilters()
    } catch (error) {
      if (disposed || id !== requestId) return
      state.error =
        error.status === 404
          ? '模型广场未开启，请检查后台模型广场设置。'
          : error.code === 'ERR_CANCELED' || error.name === 'TimeoutError'
            ? '接口请求超时，请稍后重试。'
            : '无法获取模型数据，请检查服务连接后重试。'
      $('description').hidden = true
    } finally {
      if (!disposed && id === requestId) {
        state.loading = false
        render()
      }
    }
  }
  function resolveTier(tier, base) {
    return {
      ...base,
      ...resolveIntervalPrices(tier, base),
      per_request_price: tier.per_request_price ?? base.per_request_price,
    }
  }
  function tierLabel(tier) {
    return (
      tier.tier_label ||
      (tier.max_tokens == null
        ? `> ${tier.min_tokens.toLocaleString()} tokens`
        : `≤ ${tier.max_tokens.toLocaleString()} tokens`)
    )
  }
  let detailItem = null
  function renderDetails() {
    if (!detailItem) return
    const item = detailItem,
      v = representative(item),
      m = v.model,
      g = v.group
    $('detail-title').textContent = item.displayName
    const displayCurrency = mode(m) === 'video' && m.pricing?.video_task_pricing?.currency === 'CNY'
      ? '人民币（CNY）'
      : 'USD'
    $('detail-subtitle').textContent =
      `${providerNames[item.provider] || item.provider} · 渠道：${item.supplier} · ${displayCurrency} · ${billingNames[mode(m)] || mode(m)}`
    const platformTiers = m.pricing?.intervals || [],
      officialTiers = m.official_pricing?.intervals || []
    const tiers =
      g.long_context_pricing_enabled === false
        ? []
        : [...platformTiers].sort((a, b) => a.min_tokens - b.min_tokens)
    let html = `<div class="detail-select"><span class="badge group-badge">分组倍率 ×${escapeHTML(baseMultiplier(g))}</span></div>`
    if (g.description)
      html += `<p class="detail-note">${escapeHTML(g.description)}</p>`
    html += `<h3 style="font-size:13px;margin-bottom:10px">${tiers.length > 1 ? '阶梯价格' : '标准价格'}</h3>`
    if (tiers.length <= 1)
      html += priceTable(v, basePricing(m), m.official_pricing, true)
    const efforts = Object.entries(
      m.pricing?.reasoning_effort_multipliers || {},
    ).filter(
      ([, factor]) =>
        typeof factor === 'number' && Number.isFinite(factor) && factor > 0,
    )
    if (efforts.length)
      html += `<p class="detail-note">推理强度倍率：${efforts.map(([effort, factor]) => `${escapeHTML(effort)} ×${escapeHTML(factor)}`).join(' / ')}。</p>`
    if (mode(m) !== 'token' && mode(m) !== 'parameter' && mode(m) !== 'video_token' && !m.pricing?.video_task_pricing)
      html +=
        '<p class="detail-note">当前按次或媒体计费，官方目录的每 token 参考价与此单位不同，不作折扣比较。</p>'
    if (tiers.length > 1) {
      html += `<p class="detail-note">${m.long_context_basis === 'marginal' ? '按区间分段计费，只有超出阈值的部分按对应档位单价计价。' : '整单按请求所在档位的单价计价。'} 以下价格已计入当前分组倍率。</p>`
      html += tiers
        .map((tier) => {
          const segments = referencePricingSegments(tier, m.official_pricing)
          const tables = (state.compare ? segments : segments.slice(0, 1))
            .map((segment) => {
              const range = segments.length > 1 && state.compare
                ? `<p class="detail-note">参考价适用范围：${segment.min_tokens.toLocaleString()} &lt; tokens${segment.max_tokens == null ? '' : ` ≤ ${segment.max_tokens.toLocaleString()}`}</p>`
                : ''
              return `${range}${priceTable(v, resolveTier(tier, m.pricing), segment.reference, true)}`
            }).join('')
          return `<section class="tier-section"><h3>${escapeHTML(tierLabel(tier))}</h3>${tables}</section>`
        })
        .join('')
    } else if (
      g.long_context_pricing_enabled === false &&
      officialTiers.length > 1
    ) {
      html +=
        '<p class="detail-note">此分组未启用平台长上下文阶梯计费。参考价仍可能随上下文档位变化。</p>'
      if (state.compare && mode(m) === 'token')
        html += `<table class="schedule"><thead><tr><th>参考价档位</th><th>输入</th><th>输出</th></tr></thead><tbody>${officialTiers
          .map((t) => {
            const resolved = resolveTier(t, m.official_pricing)
            return `<tr><td>${escapeHTML(tierLabel(t))}</td><td>${money(resolved.input_price)}</td><td>${money(resolved.output_price)}</td></tr>`
          })
          .join('')}</tbody></table>`
    }
    if (g.peak_rate_enabled)
      html += `<p class="detail-note">分组高峰时段：${escapeHTML(g.peak_start)}–${escapeHTML(g.peak_end)}，高峰倍率 ×${escapeHTML(g.peak_rate_multiplier)}。上表展示标准时段价格。</p>`
    const schedule = m.time_pricing
    if (schedule?.periods?.length)
      html += `<section class="tier-section"><h3>分时价格</h3><p class="detail-note">${escapeHTML(schedule.timezone)}${schedule.weekdays_only ? ' · 仅工作日' : ''}，以下时段在标准分组倍率上叠加时段倍率。</p><table class="schedule"><thead><tr><th>时段</th><th>时段倍率</th><th>输入价格</th><th>输出价格</th></tr></thead><tbody>${schedule.periods
        .map((p) => {
          const factor =
            Math.round(baseMultiplier(g) * p.multiplier * 1000) / 1000
          return `<tr><td>${escapeHTML(p.start_time)}–${escapeHTML(p.end_time)}</td><td>×${escapeHTML(p.multiplier)}</td><td>${money(m.pricing?.input_price == null ? null : m.pricing.input_price * factor)}</td><td>${money(m.pricing?.output_price == null ? null : m.pricing.output_price * factor)}</td></tr>`
        })
        .join('')}</tbody></table></section>`
    if (m.reference_pricing) {
      const sourceLink = m.reference_pricing.source_url
        ? ` · <a href="${escapeHTML(m.reference_pricing.source_url)}" target="_blank" rel="noopener noreferrer">来源</a>`
        : ''
      html += `<p class="detail-note">人工维护参考价：${escapeHTML(m.reference_pricing.source_label)}${sourceLink}${m.reference_pricing.notes ? ` · ${escapeHTML(m.reference_pricing.notes)}` : ''}。该价格仅供比较，不参与计费。</p>`
    }
    html +=
      '<p class="detail-note">参考价来自定价目录或人工维护；“—”表示未提供。最终费用以实际调用账单为准。</p>'
    $('detail-body').innerHTML = html
  }
  function openDetails(index) {
    detailItem = state.visible[index]
    renderDetails()
    $('details').showModal()
  }
  let toastTimer
  function toast(message) {
    $('toast').textContent = message
    $('toast').hidden = false
    clearTimeout(toastTimer)
    toastTimer = setTimeout(() => ($('toast').hidden = true), 2400)
  }
  async function copyText(text) {
    try {
      if (navigator.clipboard && window.isSecureContext)
        await navigator.clipboard.writeText(text)
      else {
        const area = document.createElement('textarea')
        area.value = text
        area.style.cssText = 'position:fixed;opacity:0'
        document.body.append(area)
        area.select()
        const ok = document.execCommand('copy')
        area.remove()
        if (!ok) throw new Error()
      }
      toast('已复制模型名称')
    } catch {
      toast('复制失败，请手动复制模型名称')
    }
  }
  function reset() {
    Object.assign(state, {
      provider: '',
      billing: '',
      supplier: '',
      query: '',
      sort: 'default',
    })
    $('search').value = ''
    $('sort').value = 'default'
    renderFilters()
    render()
  }
  function setFiltersOpen(open) {
    $('sidebar').classList.toggle('open', open)
    $('backdrop').classList.toggle('open', open)
  }
  root.addEventListener(
    'click',
    (event) => {
      const b = event.target.closest('button')
      if (!b) return
      if (b.dataset.filter) {
        state[b.dataset.filter] = b.dataset.value
        renderFilters()
        render()
      }
      if (b.dataset.copy != null)
        copyText(state.visible[Number(b.dataset.copy)].name)
      if (b.dataset.detail != null) openDetails(Number(b.dataset.detail))
      if (b.dataset.action === 'reset') reset()
      if (b.dataset.action === 'retry') load()
      if (b.dataset.action === 'close-detail') $('details').close()
      if (b.dataset.action === 'open-filters') setFiltersOpen(true)
      if (b.dataset.action === 'close-filters') setFiltersOpen(false)
    },
    { signal: lifecycle.signal },
  )
  root.addEventListener(
    'change',
    (event) => {
      const el = event.target
      if (el.dataset.select != null) {
        const item = state.visible[Number(el.dataset.select)]
        if (el.checked) state.selected.add(item.key)
        else state.selected.delete(item.key)
        el.closest('.model-card')?.classList.toggle('selected', el.checked)
        updateSelection()
      }
    },
    { signal: lifecycle.signal },
  )
  $('search').addEventListener('input', (event) => {
    state.query = event.target.value
    render()
  })
  $('clear-search').onclick = () => {
    state.query = ''
    $('search').value = ''
    render()
    $('search').focus()
  }
  $('reset').onclick = reset
  $('backdrop').onclick = () => setFiltersOpen(false)
  $('unit').onchange = (event) => {
    state.unit = Number(event.target.value)
    render()
    if ($('details').open) renderDetails()
  }
  $('sort').onchange = (event) => {
    state.sort = event.target.value
    render()
  }
  $('compare').onchange = (event) => {
    state.compare = event.target.checked
    render()
    if ($('details').open) renderDetails()
  }
  $('grid-view').onclick = () => {
    state.view = 'grid'
    render()
  }
  $('table-view').onclick = () => {
    state.view = 'table'
    render()
  }
  $('select-all').onchange = (event) => {
    state.visible.forEach((item) => {
      if (event.target.checked) state.selected.add(item.key)
      else state.selected.delete(item.key)
    })
    render()
  }
  $('copy-selected').onclick = () =>
    copyText(
      [
        ...new Set(
          state.models
            .filter((m) => state.selected.has(m.key))
            .map((m) => m.name),
        ),
      ].join('\n'),
    )
  $('details').addEventListener('click', (event) => {
    if (event.target === $('details')) {
      const r = $('details').getBoundingClientRect()
      if (
        event.clientX < r.left ||
        event.clientX > r.right ||
        event.clientY < r.top ||
        event.clientY > r.bottom
      )
        $('details').close()
    }
  })
  document.addEventListener(
    'keydown',
    (event) => {
      if (event.key === 'Escape') setFiltersOpen(false)
    },
    { signal: lifecycle.signal },
  )
  renderFilters()
  load()
  return () => {
    disposed = true
    requestId++
    lifecycle.abort()
    clearTimeout(toastTimer)
    $('details').close()
    root.replaceChildren()
  }
}
