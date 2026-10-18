# llm-pricing-sync

Refreshes the Azure entries of the gateway's LLM pricing file
(`gateway/configs/llm-pricing/model_prices.json` and its Helm copy) from the
[Azure Retail Prices API](https://learn.microsoft.com/en-us/rest/api/cost-management/retail-prices/azure-retail-prices),
so that the `azure-llm-cost` policy prices Azure OpenAI and Azure AI Foundry
calls at Azure's published list prices.

## Usage

From `gateway/`:

```bash
make sync-llm-pricing    # rewrite both pricing files
make check-llm-pricing   # exit 1 if either file is behind the API
```

Or directly, to replay saved API responses instead of calling the API:

```bash
go run . -file <model_prices.json> [-file ...] [-meters <saved.json> ...] [-check]
```

The tool prints every key it added (`+`), updated (`~`) or removed (`-`), and
any entry it skipped.

## Keys it writes

| Key | Price |
|---|---|
| `azure/<model>`, `azure_ai/<model>` | Global Standard |
| `azure/<zone>/<model>`, `azure_ai/<zone>/<model>` | Data Zone Standard, `zone` one of `us`, `eu`, `apac` |
| `azure/regional/<model>`, `azure_ai/regional/<model>` | Regional Standard in **eastus2** |

`azure/` holds Azure OpenAI models and `azure_ai/` Foundry's own models, as
`azure-llm-cost` reads them. Global and Regional prices are read from eastus2;
Data Zone prices from eastus2 (`us`), swedencentral (`eu`) and japaneast
(`apac`). Regional prices differ by Azure region, so operators elsewhere
override the pricing file.

Per entry, it writes input, output, cached-input and cache-write rates, each
with `_priority`, `_flex`, `_above_272k_tokens` and combined variants where
Azure has a meter. Every other field of an existing entry is kept, and entries
it does not manage are copied byte for byte.

It also removes keys no policy reads: `azure/global/*`, `azure_ai/global/*`,
`azure_ai/model_router` (model router is billed as the models it routes to),
and `azure/global-standard/*` once the base key holds the Global price.

## How meters are matched

Azure names meters in per-family shorthand, e.g. `5.6 sol ShortCo Cd Wr PP DZ`,
`56sol ShCo Inp Fl Gl` or `Llama 3.3 70B Inp glbl`. Each model in
[`models.go`](models.go) lists its product and SKU stems; the words after a
stem must all be known attribute tokens ([`classify.go`](classify.go)), which
is how batch, fine-tuning, realtime, audio and image meters are kept out.

Safety rules:

- A SKU matching two models, or two meters giving one field different
  prices, fails the run.
- An entry whose meters lack an input or an output price is skipped, so a
  partial meter set never erases a complete entry.
- Long-context meters are used only for models in `longContextAt272k`, since
  `azure-llm-cost` applies them above 272k input tokens. Grok's start lower
  and are left out.

## Adding a model

Add one line to `models` in `models.go` with the meter's `productName`, the SKU
stem(s) and the key(s) operators map deployments to. Find them with:

```bash
curl -s "https://prices.azure.com/api/retail/prices?api-version=2023-01-01-preview&\$filter=serviceName%20eq%20'Foundry%20Models'%20and%20armRegionName%20eq%20'eastus2'" \
  | jq -r '.Items[] | [.productName, .skuName, .unitPrice, .unitOfMeasure] | @tsv'
```

## Not covered

- Claude in Foundry is billed in Claude Consumption Units through Azure
  Marketplace and has no Retail Prices API meter; its `azure_ai/claude-*`
  entries are maintained by hand.
- Batch, Provisioned Throughput, tool calls and storage are not per-token
  request prices and are not written.
- Prices are USD list prices; invoices apply agreement discounts.
