#!/usr/bin/env bash
#
# Blocks staged changes that contain credential-shaped strings.
#
# GitHub push protection covers well-known vendor formats, but its free tier
# does not scan generic or non-provider patterns. Google AI Studio keys in the
# "AQ.Ab..." format are one such gap — exactly the credential this project
# uses. This hook catches those before a commit is created.
#
# Install:
#     ln -sf ../../scripts/check-secrets.sh .git/hooks/pre-commit
#
# Run manually against the whole tree:
#     ./scripts/check-secrets.sh --all
#
# Bypass for a single commit (use deliberately):
#     git commit --no-verify

set -uo pipefail

# Pattern list. Each entry is "label<TAB>extended-regex".
PATTERNS=$(
  cat <<'EOF'
Google AI Studio key (AQ. format)	AQ\.[A-Za-z0-9_-]{30,}
Google API key (AIza format)	AIza[0-9A-Za-z_-]{35}
Google OAuth token	ya29\.[0-9A-Za-z_-]{20,}
Cloudflare API token	cfat_[A-Za-z0-9_-]{20,}
GitHub token	gh[pousr]_[A-Za-z0-9]{36}
OpenAI / Anthropic style key	sk-[A-Za-z0-9_-]{20,}
AWS access key id	AKIA[0-9A-Z]{16}
Slack token	xox[baprs]-[A-Za-z0-9-]{10,}
Private key block	-----BEGIN [A-Z ]*PRIVATE KEY-----
Service account json	"private_key"[[:space:]]*:
EOF
)

# Placeholders that are meant to be committed.
ALLOW='your-api-key-here|generate-with-openssl|example\.com|EXAMPLE|placeholder|<your|xxxxx'

if [ "${1:-}" = "--all" ]; then
  FILES=$(git ls-files)
  MODE="tracked files"
else
  FILES=$(git diff --cached --name-only --diff-filter=ACM)
  MODE="staged changes"
fi

[ -z "$FILES" ] && exit 0

FOUND=0
while IFS=$'\t' read -r LABEL REGEX; do
  [ -z "${REGEX:-}" ] && continue
  while IFS= read -r FILE; do
    [ -f "$FILE" ] || continue
    # Skip this script — it necessarily contains the patterns themselves.
    case "$FILE" in scripts/check-secrets.sh) continue ;; esac

    # -e is required: patterns beginning with "-" (private key blocks) would
    # otherwise be parsed as grep options.
    HITS=$(grep -nEI -e "$REGEX" "$FILE" 2>/dev/null | grep -vE "$ALLOW")
    if [ -n "$HITS" ]; then
      if [ "$FOUND" -eq 0 ]; then
        echo ""
        echo "BLOCKED: credential-shaped strings found in $MODE"
        echo ""
      fi
      FOUND=1
      echo "  [$LABEL]"
      echo "$HITS" | sed "s|^|    $FILE:|"
    fi
  done <<< "$FILES"
done <<< "$PATTERNS"

if [ "$FOUND" -eq 1 ]; then
  cat <<'EOF'

Nothing was committed. Move the value into .env (gitignored) and read it with
os.Getenv, or add a placeholder to the allowlist in scripts/check-secrets.sh.

If this is a false positive, bypass with:
    git commit --no-verify

EOF
  exit 1
fi

exit 0
