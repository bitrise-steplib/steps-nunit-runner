#!/bin/bash

THIS_SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

ruby "${THIS_SCRIPT_DIR}/step.rb" \
	-s "${xamarin_solution}" \
	-c "${xamarin_configuration}" \
	-l "${xamarin_platform}" \
	-o "${nunit_options}"
