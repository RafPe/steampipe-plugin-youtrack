connection "youtrack" {
  plugin = "youtrack"

  # Base URL of your YouTrack instance, without a trailing slash.
  base_url = "https://youtrack.example.com"

  # Keep permanent tokens out of configuration files and source control.
  token = env("YOUTRACK_TOKEN")
}
