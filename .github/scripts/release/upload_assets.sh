#!/usr/bin/env bash

set -o nounset
set -o errexit
set -E
set -o pipefail

uploadFile() {
  filePath=${1}
  ghAsset=${2}

  echo "Uploading ${filePath} as ${ghAsset}"
  response=$(curl -s -o output.txt -w "%{http_code}" \
                  --request POST --data-binary @"$filePath" \
                  -H "Authorization: token $GITHUB_TOKEN" \
                  -H "Content-Type: text/yaml" \
                   "$ghAsset")
  if [[ "$response" != "201" ]]; then
    echo "Unable to upload the asset ($filePath): "
    echo "HTTP Status: $response"
    cat output.txt
    exit 1
  else
    echo "$filePath uploaded"
  fi
}

echo "PULL_BASE_REF= ${PULL_BASE_REF}"

IMG=${IMG} make publish-manifests
echo "Generated template-operator.yaml:"

cat template-operator.yaml

echo "Fetching releases"
CURL_RESPONSE=$(curl -w "%{http_code}" -sL \
                -H "Accept: application/vnd.github+json" \
                -H "Authorization: Bearer $GITHUB_TOKEN"\
                https://api.github.com/repos/kyma-project/template-operator/releases)
JSON_RESPONSE=$(sed '$ d' <<< "${CURL_RESPONSE}")
HTTP_CODE=$(tail -n1 <<< "${CURL_RESPONSE}")
if [[ "${HTTP_CODE}" != "200" ]]; then
  echo "${CURL_RESPONSE}"
  exit 1
fi

echo "Finding release id for: ${PULL_BASE_REF}"
RELEASE_ID=$(jq <<< "${JSON_RESPONSE}" --arg tag "${PULL_BASE_REF}" '.[] | select(.tag_name == $ARGS.named.tag) | .id')

echo "Got '${RELEASE_ID}' release id"
if [ -z "${RELEASE_ID}" ]
then
  echo "No release with tag = ${PULL_BASE_REF}"
  exit 1
fi

echo "Adding assets to Github release"
UPLOAD_URL="https://uploads.github.com/repos/kyma-project/template-operator/releases/${RELEASE_ID}/assets"

echo "$UPLOAD_URL"
uploadFile "template-operator.yaml" "${UPLOAD_URL}?name=template-operator.yaml"
uploadFile "config/samples/default-sample-cr.yaml" "${UPLOAD_URL}?name=default-sample-cr.yaml"
