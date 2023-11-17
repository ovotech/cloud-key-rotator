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
        # if the count of workflows is 1, that's the current workflow we're running in
		elif [ "${COUNT}" -gt "0" ] ; then
            RUN_IDS=$(curl "${CURL_OPTS[@]}" -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" "${API_CALL_URL}?status=${STATUS}" | jq -r '.workflow_runs[] | .id' || true)
			for RUN_ID in $RUN_IDS ; do
                echo "Checking run (id: ${RUN_ID}) for in_progress e2e_tests"
                API_CALL_URL_JOBS=https://api.github.com/repos/ovotech/cloud-key-rotator/actions/runs/$RUN_ID/jobs
                JOB_NAME_STATII=$(curl "${CURL_OPTS[@]}" -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" "${API_CALL_URL_JOBS}" | jq -r '.jobs[] | "\(.name)\(.status)"' || true)
                for JOB_NAME_STATUS in $JOB_NAME_STATII; do
                    echo "Job name / status: ${JOB_NAME_STATUS}"
                    if [[ "${JOB_NAME_STATUS}" == "e2e_test"* ]] && [[ "${JOB_NAME_STATUS}" != *"completed" ]]; then
                        echo "Another e2e_test job is currently in progress, need to wait"
                        WAIT=true
                        break
                    fi
                done
            done
            echo "${COUNT} CI workflows with status ${STATUS}"
		fi
	done

	if [ "${WAIT}" != "true" ] ; then
		sleep $((MIN_WAIT+RANDOM % (MAX_WAIT-MIN_WAIT)))
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