#!/bin/bash
set -uo pipefail

POLL_INTERVAL=30
MAX_RETRIES=30

# https://docs.github.com/en/rest/actions/workflow-runs#list-workflow-runs-for-a-repository
# status can be: completed, action_required, cancelled, failure, neutral, skipped, stale, success, timed_out, in_progress, queued, requested, waiting
BLOCKING_STATUS="action_required in_progress queued requested waiting"

# GitHub API Worfklows Run URL
API_CALL_URL=https://api.github.com/repos/ovotech/cloud-key-rotator/actions/workflows/ci.yml/runs

# Set options for curl
CURL_OPTS=(--max-time 60 --retry 5 --retry-connrefused -s)

# sleep for random amount of time to prevent workflows being triggered
# concurrently
sleep $((MIN_WAIT+RANDOM % (MAX_WAIT-MIN_WAIT)))

# While CI workflows are running, wait
i=0
while true ; do
	WAIT=false
	for STATUS in $BLOCKING_STATUS ; do	
		COUNT=$(curl "${CURL_OPTS[@]}" -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" "${API_CALL_URL}?status=${STATUS}" | jq -r '.total_count' || true)
		if [ "${COUNT}" == "null" ] ; then
			echo "failed to get CI workflows with status ${STATUS}"
			WAIT=true
		elif [ "${COUNT}" != "0" ] ; then
			echo "${COUNT} CI workflows with status ${STATUS}"
			WAIT=true
		fi
	done

	if [ "${WAIT}" != "true" ] ; then
		break
	fi

	((i+=1))
	if [ "$i" -gt "${MAX_RETRIES}" ] ; then
		echo "Aborting..."
		exit 1
	fi
	echo "Retrying after ${POLL_INTERVAL} sec ($i of ${MAX_RETRIES})"
	sleep "${POLL_INTERVAL}"
done