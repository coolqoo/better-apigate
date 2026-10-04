#!/bin/bash
# prepare-release.sh - Prepares a new release by creating and pushing a version tag
#
# Usage: ./scripts/prepare-release.sh [major|minor|patch]
#   major: 1.0.0 -> 2.0.0 (breaking changes)
#   minor: 1.0.0 -> 1.1.0 (new features)
#   patch: 1.0.0 -> 1.0.1 (bug fixes, default)

set -euo pipefail

# Get bump type from argument, default to patch
BUMP_TYPE=${1:-patch}

if [[ ! "$BUMP_TYPE" =~ ^(major|minor|patch)$ ]]; then
    echo "Usage: $0 [major|minor|patch]"
    exit 1
fi

# Fetch latest tags
echo "Fetching latest tags..."
git fetch --tags

# Get latest version tag
LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
echo "Current version: $LATEST_TAG"

# Parse version numbers
VERSION=${LATEST_TAG#v}
IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION"

# Bump version
case $BUMP_TYPE in
    major)
        MAJOR=$((MAJOR + 1))
        MINOR=0
        PATCH=0
        ;;
    minor)
        MINOR=$((MINOR + 1))
        PATCH=0
        ;;
    patch)
        PATCH=$((PATCH + 1))
        ;;
esac

NEW_VERSION="v${MAJOR}.${MINOR}.${PATCH}"
echo "New version: $NEW_VERSION"

# Show recent commits since last tag
echo ""
echo "Changes since $LATEST_TAG:"
echo "----------------------------------------"
if [[ "$LATEST_TAG" == v0.0.0 ]]; then git log --oneline -20; else git log --oneline "$LATEST_TAG"..HEAD; fi
echo "----------------------------------------"
echo ""

# Verification applies to this exact commit, not to a previous release.
echo "Set APIGATE_RELEASE_VERIFIED_SHA to $(git rev-parse HEAD) in repository variables only after completing release verification."

# Confirm
read -p "Create and push tag $NEW_VERSION? [y/N] " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted."
    exit 0
fi

# Create and push tag
echo "Creating tag $NEW_VERSION..."
git tag -a "$NEW_VERSION" -m "Release $NEW_VERSION"

echo "Pushing tag to origin..."
git push origin "$NEW_VERSION"

echo ""
echo "Release $NEW_VERSION created!"
echo "GitHub Actions will now:"
echo "  1. Build the frontend and verify generated contracts"
echo "  2. Require release verification for the tagged commit"
echo "  3. Build binaries and create the GitHub release"
echo "  4. Build and push Docker image"
echo ""
echo "Monitor progress: https://github.com/coolqoo/better-apigate/actions"
