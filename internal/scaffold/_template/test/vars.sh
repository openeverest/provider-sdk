#!/bin/bash

# Environment variables sourced by the `make test-integration*` targets before
# running chainsaw. Use it to pin the operator/engine versions your tests run
# against so they are reproducible locally and in CI.
#
# Reference values in chainsaw test files via ($values) bindings or plain
# environment substitution in `script:` steps.

export PROVIDER_ROOT_PATH=${PROVIDER_ROOT_PATH:-${PWD}}
echo "PROVIDER_ROOT_PATH=${PROVIDER_ROOT_PATH}"

# TODO: pin the versions of your operator and database engine, e.g.:
#
# export MY_OPERATOR_VERSION=${MY_OPERATOR_VERSION:-"1.2.3"}
# echo "MY_OPERATOR_VERSION=${MY_OPERATOR_VERSION}"
#
# export MY_DB_ENGINE_VERSION=${MY_DB_ENGINE_VERSION:-"8.0.0"}
# echo "MY_DB_ENGINE_VERSION=${MY_DB_ENGINE_VERSION}"
