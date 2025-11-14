#!/bin/bash
# Stamps the build. The values end up in bazel-out/stable-status.txt.
# The file `.bazelrc` has the necessary build stamp flags.
# See //bin/pcc:BUILD.bazel for how these values are used.

# Exit immediately if a command exits with a non-zero status.
set -eu

# Use STABLE_ for values that, when changed, should trigger a rebuild of stamped targets.
# The full Git SHA
echo "STABLE_GIT_COMMIT $(git rev-parse HEAD)"

# The short Git SHA
echo "STABLE_GIT_SHORT_COMMIT $(git rev-parse --short HEAD)"

# The branch name
echo "STABLE_GIT_BRANCH $(git rev-parse --abbrev-ref HEAD)"

# The version (tag, or branch/commit if no tag)
if [ -n "${VERSION-}" ]; then
  echo "STABLE_VERSION ${VERSION}"
else
  echo "STABLE_VERSION $(git describe --always --tags --dirty)"
fi

