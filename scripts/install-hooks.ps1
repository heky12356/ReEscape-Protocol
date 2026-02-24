$ErrorActionPreference = "Stop"

git config core.hooksPath .githooks
Write-Output "Git hooks path set to .githooks"
