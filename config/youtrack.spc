connection "youtrack" {
  plugin = "rafpe/youtrack"

  # Base URL of your YouTrack instance, without a trailing slash, e.g.
  # "https://example.youtrack.cloud" or "https://example.com/youtrack".
  # Can also be set with the YOUTRACK_URL environment variable.
  # base_url = "https://example.youtrack.cloud"

  # YouTrack permanent token. Create one in your YouTrack profile under
  # Account Security > Tokens. Can also be set with the YOUTRACK_TOKEN
  # environment variable. Keep permanent tokens out of configuration files
  # and source control.
  # token = "perm:cmFmcGU=.UGVybWFuZW50IHRva2Vu.abcdefghij1234567890"
}
