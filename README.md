# Snyk Auto Org

**THIS IS A VERY EXPERIMENTAL PROOF OF CONCEPT. USE AT OWN RISK TALK TO ME FIRST**
Snyk Auto Org is a command-line tool that wraps the [Snyk CLI](https://docs.snyk.io/snyk-cli) and automatically sets the `SNYK_CFG_ORG` environment variable based on available organizations from your Snyk account. It intelligently selects the appropriate organization based on your Git repository or specified criteria.

To use this, navigate in VS Code or your IDE of choice to the Snyk plugin settings and find the CLI Path. Modify the snyk cli path to the binary release of this app. Make sure you have the version of snyk cli that you want the IDE to use in your system path.


## Features

- **Intelligent Organization Selection**:
  - Automatically detects Git repository URL and finds the matching organization
  - Caches organization and target data for faster subsequent runs
  - Falls back to default organization if configured
  - Runs without organization setting if no match is found
- **Flexible Configuration**:
  - Manual organization selection by name, ID, or slug
  - Configurable cache duration
  - Verbose logging for troubleshooting
- **Performance Optimized**:
  - SQLite-based caching system
  - Efficient API pagination handling
  - Smart cache invalidation
- **Full Snyk CLI Integration**:
  - Passes through all Snyk CLI commands and arguments
  - Preserves Snyk CLI's exit codes and output

## Installation
Prerequisites:
* Have the Snyk CLI installed and in your global PATH
* Have authenticated with `snyk auth`
* Do not have an `CFG_ORG` environment variable set in your environment
* Do not have an Snyk Organization set in your Snyk IDE
* Do not have `snyk config org` set (if so, unset it)

1. Download the release in github
2. Unzip
2.1 On MacOS clear quarantine `xattr -d com.apple.quarantine snyk-auto-org`
3. Move the binary to a directory in your PATH
4. Set your IDE to use the Snyk CLI binary you just installed or use the snyk-auto-org binary in the command line

## Usage

```bash
# Basic usage - automatically detects Git repository and organization
snyk-auto-org test

# List available organizations
snyk-auto-org --list-orgs

# Use a specific organization (by name, ID, or slug)
snyk-auto-org --org="My Organization" test
snyk-auto-org --org="org-id-123" test

# Reset the organization cache
snyk-auto-org --reset-cache

# Show verbose output
snyk-auto-org --verbose test

# Specify a Git URL explicitly
snyk-auto-org --git-url="https://github.com/username/repo" test

# Disable automatic Git detection
snyk-auto-org --auto-detect-git=false test

# Set custom cache TTL
snyk-auto-org --cache-ttl="12h" test
```

## How It Works

1. **Organization Selection Process**:
   - If `--org` flag is provided, uses the specified organization (by name, ID, or slug)
   - Otherwise:
     1. Checks for Git remote URL (if `--auto-detect-git=true` or `--git-url` provided)
     2. Searches cached targets for matching repository URL
     3. If not in cache or cache expired, queries Snyk API for organizations and targets
     4. If matching target found, uses its organization
     5. If no match but default organization configured, uses that
     6. If no organization determined, runs without setting one

2. **Caching System**:
   - Uses SQLite database at `~/.config/snyk-auto-org/cache.db`
   - Caches organizations, targets, and their relationships
   - Default TTL: 24 hours (configurable)
   - Manual cache reset available via `--reset-cache`

## Configuration

Configuration file: `~/.config/snyk-auto-org/config.json`

```json
{
  "cache_ttl": "24h",
  "default_org": "",
  "verbose": false
}
```

### Configuration Options

- `cache_ttl`: Duration to cache organization and target data (default: "24h")
- `default_org`: Default organization to use when no match found (optional)
- `verbose`: Enable detailed logging by default (default: false)

## Requirements

- Go 1.21 or higher
- Snyk CLI installed and authenticated
- Git (for repository URL auto-detection)
- SQLite (for caching)

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
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...
```

### Project Structure

```
/
├── cmd/snyk-auto-org/    # Main entry point
├── internal/
│   ├── api/             # Snyk API client
│   ├── app/             # Core application logic
│   ├── cache/           # SQLite caching system
│   ├── config/          # Configuration handling
│   └── cmd/             # Command execution
└── docs/                # Additional documentation
```

## Troubleshooting

### Common Issues

1. **Organization Not Found**
   - Use `--list-orgs` to verify available organizations
   - Check if Git remote URL matches any Snyk targets
   - Try resetting cache with `--reset-cache`

2. **Authentication Issues**
   - Ensure Snyk CLI is authenticated (`snyk auth`)
   - Verify token in `~/.config/configstore/snyk.json`
   - Check token permissions in Snyk settings

3. **Cache Problems**
   - Reset cache: `snyk-auto-org --reset-cache`
   - Verify cache file permissions
   - Check available disk space

For more detailed design information and technical details, see [DESIGN.md](DESIGN.md).
