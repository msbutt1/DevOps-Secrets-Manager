#!/bin/bash
# Example usage of the secrets CLI
# Make sure the API server is running at localhost:8080

set -e

SECRETS_BIN="${SECRETS_BIN:-./target/release/secrets}"

echo "DevOps Secrets Manager CLI - Example Usage"
echo "==========================================="
echo ""

# Check if binary exists
if [ ! -f "$SECRETS_BIN" ]; then
    echo "Error: Binary not found at $SECRETS_BIN"
    echo "Build it first with: cargo build --release"
    exit 1
fi

echo "Step 1: Login"
echo "-------------"
echo "Run: $SECRETS_BIN login"
echo "You'll be prompted for email and password"
echo ""

echo "Step 2: List Vaults"
echo "-------------------"
echo "Command: $SECRETS_BIN vault list"
echo ""

echo "Step 3: List Environments"
echo "------------------------"
echo "Command: $SECRETS_BIN env list --vault <vault-name>"
echo ""

echo "Step 4: Pull Secrets"
echo "-------------------"
echo "Command: $SECRETS_BIN pull --vault <vault> --env <env>"
echo "Or save to file: $SECRETS_BIN pull --vault <vault> --env <env> --out .env"
echo ""

echo "Step 5: Run Command with Secrets (KILLER FEATURE)"
echo "------------------------------------------------"
echo "Command: $SECRETS_BIN run --vault <vault> --env <env> -- <your-command>"
echo ""
echo "Examples:"
echo "  - $SECRETS_BIN run --vault api --env dev -- node server.js"
echo "  - $SECRETS_BIN run --vault app --env test -- npm test"
echo "  - $SECRETS_BIN run --vault backend --env prod -- python app.py"
echo ""

echo "Step 6: Set a Secret"
echo "-------------------"
echo "Command: $SECRETS_BIN set KEY=value --vault <vault> --env <env>"
echo ""
echo "With options:"
echo "  $SECRETS_BIN set API_KEY=abc123 --vault api --env dev \\"
echo "    --description 'Third-party API key' \\"
echo "    --rotation-days 90"
echo ""

echo "Step 7: View Audit Logs"
echo "----------------------"
echo "Command: $SECRETS_BIN audit"
echo "Or filter: $SECRETS_BIN audit --vault <vault-name>"
echo ""

echo "Step 8: Logout"
echo "-------------"
echo "Command: $SECRETS_BIN logout"
echo ""

echo "For help on any command, use --help:"
echo "  $SECRETS_BIN --help"
echo "  $SECRETS_BIN run --help"
echo ""
