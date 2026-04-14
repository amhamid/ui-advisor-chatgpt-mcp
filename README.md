# ui-advisor-chatgpt-mcp

An MCP server that connects Claude Code to OpenAI's API for UI/UX design work. It provides two capabilities:

- **Design Review** — Send a screenshot to GPT vision for specific, actionable UI/UX feedback
- **Image Generation** — Generate UI mockups and design assets (logos, icons) via GPT Image

## Setup

### Prerequisites

- Go 1.22+
- An [OpenAI API key](https://platform.openai.com/api-keys)

### Install

```bash
git clone https://github.com/your-username/ui-advisor-chatgpt-mcp.git
cd ui-advisor-chatgpt-mcp

# Copy the example config
cp config.yaml.example config.yaml
```

Edit `config.yaml` and set your API key (or use an environment variable):

```yaml
openai_api_key: "${OPENAI_API_KEY}"  # reads from env
# or
openai_api_key: "sk-..."             # hardcoded (not recommended)
```

### Add to Claude Code

Using `go run` (no build step):

```bash
claude mcp add ui-advisor -- go run ~/path/to/ui-advisor-chatgpt-mcp/main.go
```

Or compile first for faster startup:

```bash
go build -o ui-advisor-chatgpt-mcp .
claude mcp add ui-advisor -- ~/path/to/ui-advisor-chatgpt-mcp/ui-advisor-chatgpt-mcp
```

## Tools

### `design_review`

Review a UI screenshot with GPT vision. Returns feedback structured as what works, issues found, and suggested changes with concrete values (padding, hex colors, font weights, corner radii).

| Parameter    | Type    | Required | Description                                                    |
|-------------|---------|----------|----------------------------------------------------------------|
| `image_path` | string  | yes      | Path to the screenshot                                         |
| `context`    | string  | no       | What this screen is, what app it's for                         |
| `focus`      | string  | no       | Aspect to review: `spacing`, `typography`, `color`, `hierarchy`, `overall` |
| `force`      | boolean | no       | Bypass budget/daily limit checks                               |

### `generate_mockup`

Generate a UI mockup image. When `quality` is `"low"`, uses the cheaper model for quick iterations.

| Parameter              | Type    | Required | Description                                              |
|------------------------|---------|----------|----------------------------------------------------------|
| `prompt`               | string  | yes      | Description of the UI to generate                        |
| `reference_image_path` | string  | no       | Existing screenshot to use as reference for redesign     |
| `size`                 | string  | no       | `1024x1024`, `1536x1024`, or `1024x1536`                 |
| `quality`              | string  | no       | `low` (cheap model), `medium`, or `high`                 |
| `filename`             | string  | no       | Custom filename (without extension)                      |
| `force`                | boolean | no       | Bypass budget/daily limit checks                         |

### `generate_asset`

Generate a design asset (logo, icon) at high quality with transparent background support.

| Parameter    | Type    | Required | Description                                    |
|-------------|---------|----------|------------------------------------------------|
| `prompt`     | string  | yes      | Description of the asset                       |
| `background` | string  | no       | `transparent` (default) or `opaque`            |
| `size`       | string  | no       | Default `1024x1024`                            |
| `filename`   | string  | yes      | Filename without extension, e.g. `"app-logo"`  |
| `force`      | boolean | no       | Bypass budget/daily limit checks               |

### `get_usage`

Returns current spending: monthly total, remaining budget, images generated today, daily limit remaining, and a breakdown of today's calls. No arguments.

## Configuration

All options in `config.yaml`:

```yaml
openai_api_key: "${OPENAI_API_KEY}"  # read from env if prefixed with $
review_model: "gpt-5.4-mini"         # for vision-based design review
image_model: "gpt-image-1"           # for high quality mockups
image_model_cheap: "gpt-image-1-mini" # for quick iterations
max_budget_usd: 10.00                # monthly budget, resets on the 1st
daily_limit_images: 30                # resets at midnight
default_image_quality: "medium"      # low, medium, high
default_image_size: "1024x1024"      # 1024x1024, 1536x1024, 1024x1536
asset_quality: "high"                # for logos/icons
save_path: "./outputs"               # where generated images are saved
```

You can update `config.yaml` at any time without rebuilding — it's read at server startup.

## Budget & Limits

The server tracks spending in a local `usage.json` file:

- **Monthly budget** (`max_budget_usd`) resets on the 1st of each month
- **Daily image limit** (`daily_limit_images`) resets at midnight
- When a limit is hit, the tool returns an error suggesting to retry with `force: true` to override
- Use `get_usage` to check current spending at any time

## Development

```bash
# Run tests
go test ./...

# Build
go build -o ui-advisor-chatgpt-mcp .
```

## License

MIT
