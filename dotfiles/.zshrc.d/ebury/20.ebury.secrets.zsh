jira-api-token () {
  local jira_api_item_uuid="dmp4se4re4ughkv6ykyvirgmja"

  op item get "${jira_api_item_uuid}" --fields token --reveal
}

export-jira-api-token () {
  local token
  token="$(jira-api-token)"

  export JIRA_API_TOKEN="${token}"
}
