#! /bin/bash
# Prints the test logs.

readonly _logs="bazel-testlogs/nvim_testing/nvim_testing_test/test.outputs/outputs.zip"

unzip -c "${BUILD_WORKSPACE_DIRECTORY}/${_logs}" | less
