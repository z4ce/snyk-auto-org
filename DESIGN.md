# Snyk Auto Org CLI Design

## Overview
This tool will create a wrapper around the Snyk CLI that automatically sets the `SNYK_CFG_ORG` environment variable based on available organizations from the user's Snyk account. This simplifies the workflow by eliminating the need to manually specify the organization ID.

## Architecture

### Key Components
1. **Organization Retrieval**: Query and parse available Snyk organizations using the Snyk API
2. **Command Execution**: Run Snyk commands with the automatically set organization
3. **CLI Interface**: Handle user input and command-line arguments
4. **Caching System**: Store organization data in a local SQLite database

### Implementation Flow
1. User invokes the CLI with Snyk commands
2. Tool checks for cached organization data
   - If cache is valid, use cached data
   - If cache is invalid or missing, retrieve from Snyk API and update cache
3. Tool selects an appropriate organization (default: first one)
4. Tool sets the `SNYK_CFG_ORG` environment variable
5. Tool executes the original Snyk command with all arguments passed through

## Technical Details

### Organization Retrieval
- Make HTTP requests to the Snyk API endpoint for organizations (e.g., `https://api.snyk.io/v1/orgs`)
- Use the Snyk authentication token from user config (same one used by the CLI)
  - Token location: `~/.config/configstore/snyk.json` (default path, may vary by OS)
  - Format in config: `{"api": "your-api-token"}`
- Parse the JSON response to extract organization IDs
- Handle errors if the API request fails or no organizations are found
- API Response format:
  ```json
  {
    "orgs": [
      {
        "id": "org-id-1",
        "name": "Organization Name 1",
        "slug": "org-slug-1",
        "url": "https://app.snyk.io/org/org-slug-1"
      },
      {
        "id": "org-id-2",
        "name": "Organization Name 2",
        "slug": "org-slug-2",
        "url": "https://app.snyk.io/org/org-slug-2"
      }
    ]
  }
  ```

### Command Execution
- Copy the current environment variables
- Add or replace the `SNYK_CFG_ORG` environment variable
- Execute the Snyk command with all provided arguments
- Connect standard input/output/error to maintain the same user experience
- Handle special non-Snyk commands (like `--reset-cache`) before passing to Snyk

### CLI Interface
- Pass through all arguments to the Snyk CLI unchanged
- Provide minimal output about which organization is being used
- Return the same exit code as the underlying Snyk command
- Add special commands:
  - `--reset-cache`: Clear the organization cache and fetch fresh data
  - `--cache-ttl=<duration>`: Set the time-to-live for cached data (default: 24h)
  - `--org=<name or id>`: Explicitly specify which organization to use
  - `--list-orgs`: Display available organizations and exit
  - `--verbose`: Show additional information during execution

### Caching System
- Use an embedded SQLite database for storing organization data
- Database location: `~/.config/snyk-auto-org/cache.db` (create directory if it doesn't exist)
- Schema:
  ```sql
  CREATE TABLE organizations (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    url TEXT NOT NULL
  );
  
  CREATE TABLE metadata (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
  );
  ```
- Store:
  - Organization IDs and names
  - Timestamp of last update
  - User's authentication context (hashed to detect changes)
- Implement cache invalidation:
  - Based on configurable TTL (time-to-live)
  - When authentication changes
  - Manually via the `--reset-cache` flag

## Current Implementation Details

### Project Structure
```
/
├── cmd/
│   └── snyk-auto-org/
│       └── main.go           # Main entry point
├── internal/
│   ├── api/
│   │   ├── snyk.go           # Snyk API client
│   │   ├── suite_test.go     # API test suite
│   │   └── snyk_test.go      # API tests
│   ├── app/
│   │   ├── root.go           # Root command implementation
│   │   ├── suite_test.go     # App test suite
│   │   └── root_test.go      # App tests
│   ├── cache/
│   │   ├── sqlite.go         # Cache implementation
│   │   ├── suite_test.go     # Cache test suite
│   │   └── sqlite_test.go    # Cache tests
│   ├── config/
│   │   ├── config.go         # Configuration handling
│   │   ├── suite_test.go     # Config test suite
│   │   └── config_test.go    # Config tests
│   └── cmd/
│       ├── executor.go       # Command execution logic
│       ├── suite_test.go     # Command test suite
│       └── executor_test.go  # Command tests
├── go.mod                    # Go module definition
├── go.sum                    # Go module checksums
├── main.go                   # Application entry point
├── DESIGN.md                 # Project design documentation
└── README.md                 # Project documentation
```

### Dependencies
- Go standard library
  - `os/exec` for command execution
  - `encoding/json` for parsing organization data
  - `os` for environment variable handling
  - `net/http` for making API requests
  - `time` for TTL calculations
  - `path/filepath` for file path handling
- External dependencies
  - `github.com/mattn/go-sqlite3` for SQLite integration
  - `github.com/jmoiron/sqlx` for simplified database operations
  - `github.com/spf13/cobra` for CLI command structure
  - `github.com/spf13/viper` for configuration management
  - `github.com/onsi/ginkgo/v2` and `github.com/onsi/gomega` for testing

## Testing Approach

### Test Structure
- Using BDD-style tests with Ginkgo and Gomega
- Each package has a suite_test.go file for test initialization
- Tests follow a nested structure using Describe/Context/It blocks
- Test coverage includes unit tests, integration tests, and edge cases

### Test Categories

#### Unit Tests
- Testing individual components in isolation
- Mocking external dependencies
- Focus on specific functions and methods

#### Integration Tests
- Testing interaction between components
- Verifying correct data flow between layers
- Testing with external dependencies (where appropriate)

#### Edge Cases and Error Handling
- Testing with invalid inputs
- Testing error scenarios
- Testing boundary conditions

### Test Techniques

#### Mock API Server
- Using `httptest.Server` to mock the Snyk API
- Verifying correct request formatting
- Simulating various API responses and errors

#### Temporary Files and Directories
- Creating isolated test environments
- Managing HOME directory redirection for config/cache testing
- Cleaning up after tests

#### Environment Variables
- Testing with various environment configurations
- Simulating different user environments
- Ensuring proper cleanup after tests

#### Output Capture
- Capturing and testing command output
- Verifying proper output formatting

## Future Enhancements

### Code Improvements
1. **Dependency Injection**:
   - Refactor to allow easier mocking of dependencies
   - Make external services like file system and HTTP client injectable
   - Improve testability of components like token retrieval

2. **Time-based Testing**:
   - Add a clock interface for better time-based testing
   - Make time-dependent functions more testable
   - Use deterministic time sources in tests

3. **Error Handling**:
   - Improve error messages and contexts
   - Add structured error types
   - Implement better user-facing error reporting

### Feature Enhancements
1. **Organization Selection**:
   - Add interactive organization selection with arrow keys
   - Implement organization search/filtering
   - Add organization aliases for easier selection

2. **Performance Improvements**:
   - Add a timeout for API requests
   - Implement concurrent API requests if multiple endpoints are needed
   - Optimize startup time for faster execution

3. **User Experience**:
   - Add color output for better readability
   - Implement rich terminal UI for interactive features
   - Add completion suggestions for commands and organization names

4. **Advanced Caching**:
   - Sync cache across multiple machines
   - Add cache compression for large datasets
   - Implement smarter invalidation strategies

5. **Security Improvements**:
   - Implement secure token storage
   - Add support for API key rotation
   - Enhance error messages to avoid leaking sensitive information

## API Reference
- Snyk API base URL: `https://api.snyk.io/api/v1`
- Authentication: Bearer token in Authorization header
- Relevant endpoints:
  - `GET /orgs` - List organizations
  - `GET /user` - Get current user info (for auth verification)

## Configuration
Default configuration saved at `~/.config/snyk-auto-org/config.json`:
```json
{
  "cache_ttl": "24h",
  "default_org": "",
  "verbose": false
}
```

## Troubleshooting and Debugging

### Verbose Mode
- Use `--verbose` flag to enable detailed logging
- Shows API requests, caching operations, and command execution details
- Useful for troubleshooting issues with organization selection or command execution

### Cache Reset
- If you encounter issues with organizations not appearing correctly, use `--reset-cache`
- This forces a fresh retrieval from the API
- Especially useful after organization changes or token updates

### Common Issues

1. **Authentication Failures**:
   - Check that Snyk CLI is properly authenticated
   - Verify token in `~/.config/configstore/snyk.json`
   - Try running `snyk auth` to refresh your token

2. **Organization Not Found**:
   - Use `--list-orgs` to see available organizations
   - Reset cache to get fresh data from API
   - Check if you have the correct permissions in Snyk

3. **Command Execution Issues**:
   - Ensure Snyk CLI is properly installed and in PATH
   - Try running the command directly with Snyk CLI to verify it works
   - Check for environment variables that might interfere with execution 