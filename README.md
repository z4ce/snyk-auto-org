# Snyk Auto Org

Snyk Auto Org is a command-line tool that wraps the [Snyk CLI](https://docs.snyk.io/snyk-cli) and automatically sets the `SNYK_CFG_ORG` environment variable based on available organizations from your Snyk account.

## Features

- Automatically detects Git repository URL and finds the matching organization
- Runs Snyk CLI commands without organization setting if no match is found
- Caches organization data for faster subsequent runs
- Allows manual selection of organizations
- Passes through all Snyk CLI commands and arguments

## Installation

```bash
# Clone the repository
git clone https://github.com/z4ce/snyk-auto-org.git
cd snyk-auto-org

# Build the tool
go build -o snyk-auto-org ./cmd/snyk-auto-org

# Install locally (optional)
go install ./cmd/snyk-auto-org
```

## Usage

```bash
# Run a Snyk command with auto organization selection
# (will auto-detect Git remote URL if in a Git repository)
# If no organization is found for the Git URL, the command runs as normal
snyk-auto-org test

# List available organizations
snyk-auto-org --list-orgs

# Use a specific organization
snyk-auto-org --org="My Organization" test

# Reset the organization cache
snyk-auto-org --reset-cache test

# Show verbose output
snyk-auto-org --verbose test

# Specify a Git URL explicitly to find the right organization
snyk-auto-org --git-url="https://github.com/username/repo" test

# Disable automatic Git detection
snyk-auto-org --auto-detect-git=false test
```

## How It Works

1. If `--org` flag is provided, the specified organization is used.
2. Otherwise, the tool:
   - Automatically detects the Git remote URL (unless disabled with `--auto-detect-git=false`)
   - Searches for an organization with a matching target
   - If found, sets `SNYK_CFG_ORG` to that organization's ID
   - If not found, runs Snyk without setting an organization
3. If no Git URL is found but a default organization is set in config, that organization is used.
4. If no organization can be determined, the command runs without setting one.

## Configuration

Configuration is stored in `~/.config/snyk-auto-org/config.json`:

```json
{
  "cache_ttl": "24h",
  "default_org": "",
  "verbose": false
}
```

## Requirements

- Go 1.21 or higher
- Snyk CLI installed and configured
- Git (for auto-detection of repository URLs)

## Development

### Building from Source

```bash
# Clone the repository
git clone https://github.com/z4ce/snyk-auto-org.git
cd snyk-auto-org

# Install dependencies
go mod download

# Build
go build -o snyk-auto-org ./cmd/snyk-auto-org
```

### Running Tests

```bash
go test ./...
```

## License

[MIT License](LICENSE) 